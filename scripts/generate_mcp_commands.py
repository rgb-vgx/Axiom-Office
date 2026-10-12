"""Sinh danh sach lenh bridge tu registry lenh cua extension LibreOffice.

Nguon duy nhat: registry Python `axiom.commands` (cac module general/writer/calc/impress/checks
tu dang ky qua `command()`; trung ten bi bao loi).

Ra ba ban sinh, KHONG sua tay (ca ba deu sinh tu registry Python):
  - `catalog/live-commands.json`  - dung nguyen dau ra cua `commands.catalog()`.
  - `core-go/internal/mcpserver/livecommands_gen.go` - Agent Core ban Go nhung san (chi lay
    name/kind/params name+hint tu registry).
  - `src/AxiomOffice.WPS/commands.js` - mang CATALOG cua add-in WPS (vung giua hai moc
    BEGIN/END SINH TU DONG; HANDLERS + MUTATING trong file do van viet tay).

`tests/lo/test_extension.py` so lai file JSON voi registry de khong bi lech.

    python scripts/generate_mcp_commands.py [--check]
"""
from __future__ import annotations

import argparse
import json
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath"))

TARGET = os.path.join(ROOT, "catalog", "live-commands.json")
GO = os.path.join(ROOT, "core-go", "internal", "mcpserver", "livecommands_gen.go")
JS = os.path.join(ROOT, "src", "AxiomOffice.WPS", "commands.js")

JS_BEGIN = "// --- BEGIN SINH TU DONG (scripts/generate_mcp_commands.py) - KHONG SUA TAY ---"
JS_END = "// --- END SINH TU DONG ---"

HEADER_GO = "// SINH TU DONG tu registry Python (axiom.commands) bang scripts/generate_mcp_commands.py - KHONG sua tay.\n"


def build() -> list:
    import types

    # Registry import cac module lenh -> can `uno`; thay bang module gia nhu tests/lo (chi doc khai bao).
    if "uno" not in sys.modules:
        uno = types.ModuleType("uno")
        uno.createUnoStruct = lambda name: types.SimpleNamespace()
        uno.Enum = lambda kind, name: name
        uno.invoke = lambda obj, name, args: getattr(obj, name)(*args)
        uno.Any = lambda kind, value: value
        uno.fileUrlToSystemPath = lambda url: url
        uno.systemPathToFileUrl = lambda path: path
        uno.getConstantByName = lambda name: 0
        sys.modules["uno"] = uno
    if "unohelper" not in sys.modules:
        sys.modules["unohelper"] = types.ModuleType("unohelper")

    from axiom import commands

    commands.load_all()
    return commands.catalog()


def render(items: list) -> str:
    return json.dumps(items, ensure_ascii=False, indent=2) + "\n"


def go_string(value) -> str:
    if value is None:
        return '""'
    return '"' + str(value).replace("\\", "\\\\").replace('"', '\\"') + '"'


def render_go(items: list) -> str:
    lines = [HEADER_GO, "package mcpserver\n\n"]
    lines.append("type liveCommandParam struct {\n\tName string\n\tHint string\n}\n\n")
    lines.append("type liveCommand struct {\n\tName   string\n\tKind   string\n\tParams []liveCommandParam\n}\n\n")
    lines.append("// liveCommands la danh sach lenh bridge (sinh tu registry Python).\n")
    lines.append("var liveCommands = []liveCommand{\n")
    for item in items:
        params = ", ".join("{Name: %s, Hint: %s}" % (go_string(p["name"]), go_string(p["hint"]))
                           for p in item["params"])
        lines.append("\t{Name: %s, Kind: %s, Params: []liveCommandParam{%s}},\n" % (
            go_string(item["name"]), go_string(item["kind"]), params))
    lines.append("}\n")
    return "".join(lines)


def read_text(path: str) -> str:
    if not os.path.exists(path):
        return ""
    with open(path, encoding="utf-8", newline="") as handle:
        return handle.read().replace("\r\n", "\n")


def js_string(value) -> str:
    # Escape cua JSON la tap con cua string literal JS (voi dau nhay doi), nen dung nguyen
    # json.dumps de khong tu noi chuoi bang tay lam hong JS (vd hint cua et.writeRange chua \",
    # hint cua et.importCsv chua \).
    return json.dumps(value, ensure_ascii=False)


def render_js_catalog(items: list) -> str:
    # Sinh phan than mang CATALOG (ke ca dong `var CATALOG = [` va `];`), theo dung thu tu
    # cua registry. Dinh dang giu nguyen loi trinh bay dang co trong commands.js (ES5:
    # chi object/array literal).
    lines = ["  var CATALOG = ["]
    for item in items:
        kind = "null" if item["kind"] is None else js_string(item["kind"])
        agent = "true" if item["agent"] else "false"
        lines.append("    { name: %s, kind: %s, agent: %s," % (js_string(item["name"]), kind, agent))
        lines.append("      summary: %s," % js_string(item["summary"]))
        params = item["params"]
        if len(params) == 0:
            lines.append("      params: [] },")
        elif len(params) == 1:
            hint = "null" if params[0]["hint"] is None else js_string(params[0]["hint"])
            required = "true" if params[0]["required"] else "false"
            lines.append("      params: [{ name: %s, required: %s, hint: %s }] }," % (
                js_string(params[0]["name"]), required, hint))
        else:
            lines.append("      params: [")
            for index, param in enumerate(params):
                hint = "null" if param["hint"] is None else js_string(param["hint"])
                required = "true" if param["required"] else "false"
                comma = "," if index < len(params) - 1 else ""
                lines.append("        { name: %s, required: %s, hint: %s }%s" % (
                    js_string(param["name"]), required, hint, comma))
            lines.append("      ] },")
    lines.append("  ];")
    return "\n".join(lines) + "\n"


def splice_js(text: str, block: str) -> str:
    if text.count(JS_BEGIN) != 1:
        raise ValueError("khong tim thay moc BEGIN dung 1 lan trong %s (tim thay %d lan)" % (
            JS, text.count(JS_BEGIN)))
    if text.count(JS_END) != 1:
        raise ValueError("khong tim thay moc END dung 1 lan trong %s (tim thay %d lan)" % (
            JS, text.count(JS_END)))
    pre, rest = text.split(JS_BEGIN, 1)
    if JS_END not in rest:
        raise ValueError("moc END nam truoc moc BEGIN trong %s" % JS)
    _, post = rest.split(JS_END, 1)
    # Moc nam trong than IIFE (thut 2 khoang trang nhu `var CATALOG`): giu indent de ghep idempotent.
    return pre + JS_BEGIN + "\n" + block + "  " + JS_END + post


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem tra cac file co khop registry khong")
    args = parser.parse_args()

    items = build()
    json_text = render(items)
    go_text = render_go(items)
    js_block = render_js_catalog(items)
    try:
        js_text = splice_js(read_text(JS), js_block)
    except ValueError as err:
        if args.check:
            print("LECH: %s" % err, file=sys.stderr)
            return 1
        raise

    if args.check:
        stale = []
        if read_text(TARGET) != json_text:
            stale.append(TARGET)
        if read_text(GO) != go_text:
            stale.append(GO)
        if read_text(JS) != js_text:
            stale.append(JS)
        if stale:
            print("LECH: chay lai scripts/generate_mcp_commands.py (%s)" % ", ".join(stale), file=sys.stderr)
            return 1
        print("khop: %s" % ", ".join([TARGET, GO, JS]))
        return 0

    changed = []
    if read_text(TARGET) != json_text:
        os.makedirs(os.path.dirname(TARGET), exist_ok=True)
        with open(TARGET, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(json_text)
        changed.append(TARGET)
    if read_text(GO) != go_text:
        os.makedirs(os.path.dirname(GO), exist_ok=True)
        with open(GO, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(go_text)
        changed.append(GO)
    if read_text(JS) != js_text:
        with open(JS, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(js_text)
        changed.append(JS)
    if changed:
        print("da ghi %s (%d lenh)" % (", ".join(changed), len(items)))
    else:
        print("khong doi (%d lenh)" % len(items))
    return 0


if __name__ == "__main__":
    sys.exit(main())
