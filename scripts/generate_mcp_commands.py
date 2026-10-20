"""Sinh danh sach lenh bridge tu registry lenh cua extension LibreOffice.

Nguon duy nhat: registry Python `axiom.commands` (cac module general/writer/calc/impress/checks
tu dang ky qua `command()`; trung ten bi bao loi).

Ra ba ban sinh, KHONG sua tay (ca ba deu sinh tu registry Python):
  - `catalog/live-commands.json`  - dung nguyen dau ra cua `commands.catalog()`.
  - `core-go/internal/mcpserver/livecommands_gen.go` - Agent Core ban Go nhung san (chi lay
    name/kind/params name+hint tu registry).
  - `src/AxiomOffice.WPS/commands.js` - mang CATALOG cua add-in WPS (vung giua hai moc
    BEGIN/END SINH TU DONG; HANDLERS trong file do van viet tay).
  - Danh sach lenh SUA (co `undo`: moi lenh = dung mot buoc Undo) cho nut "Hoan tac luot nay" cua ba pane:
    `pane-core.js` (WPS), `axiom/chat.py` (LibreOffice), `Ai/AskAiPane.cs` (Windows, chi writer.*) - cung cap
    moc BEGIN/END nhu commands.js.

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
PANE_JS = os.path.join(ROOT, "src", "AxiomOffice.WPS", "pane-core.js")
CHAT_PY = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath", "axiom", "chat.py")
PANE_CS = os.path.join(ROOT, "src", "AxiomOffice", "Ai", "AskAiPane.cs")

JS_BEGIN = "// --- BEGIN SINH TU DONG (scripts/generate_mcp_commands.py) - KHONG SUA TAY ---"
JS_END = "// --- END SINH TU DONG ---"
PY_BEGIN = "# --- BEGIN SINH TU DONG (scripts/generate_mcp_commands.py) - KHONG SUA TAY ---"
PY_END = "# --- END SINH TU DONG ---"

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
        undo = "true" if item["undo"] else "false"
        lines.append("    { name: %s, kind: %s, agent: %s, undo: %s," % (js_string(item["name"]), kind, agent, undo))
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


def splice(path: str, text: str, block: str, begin: str = JS_BEGIN, end: str = JS_END, indent: str = "  ") -> str:
    if text.count(begin) != 1:
        raise ValueError("khong tim thay moc BEGIN dung 1 lan trong %s (tim thay %d lan)" % (path, text.count(begin)))
    if text.count(end) != 1:
        raise ValueError("khong tim thay moc END dung 1 lan trong %s (tim thay %d lan)" % (path, text.count(end)))
    pre, rest = text.split(begin, 1)
    if end not in rest:
        raise ValueError("moc END nam truoc moc BEGIN trong %s" % path)
    _, post = rest.split(end, 1)
    # Giu indent cua moc de ghep idempotent.
    return pre + begin + "\n" + block + indent + end + post


def edit_actions(items: list, prefix: str = "") -> list:
    return sorted(item["name"] for item in items if item["undo"] and item["name"].startswith(prefix))


def render_pane_js(items: list) -> str:
    lines = ["  // Lenh SUA tai lieu: co `undo` trong registry = dung mot buoc Undo; pane dem de \"Hoan tac luot nay\".",
             "  var EDIT_ACTIONS = {"]
    names = edit_actions(items)
    for index, name in enumerate(names):
        lines.append("    %s: true%s" % (js_string(name), "," if index < len(names) - 1 else ""))
    lines.append("  };")
    return "\n".join(lines) + "\n"


def render_chat_py(items: list) -> str:
    lines = ["# Lenh SUA tai lieu: co `undo` trong registry = dung mot buoc Undo (documents.undo_step).",
             "EDIT_ACTIONS = frozenset({"]
    lines += ["    %s," % json.dumps(name) for name in edit_actions(items)]
    lines.append("})")
    return "\n".join(lines) + "\n"


def render_pane_cs(items: list) -> str:
    lines = ["        // Lenh writer.* co `undo` trong registry = dung mot buoc Undo (UndoRecordScope).",
             "        private static readonly HashSet<string> WordEditActions = new HashSet<string>(StringComparer.Ordinal)",
             "        {"]
    lines += ["            %s," % json.dumps(name) for name in edit_actions(items, "writer.")]
    lines.append("        };")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem tra cac file co khop registry khong")
    args = parser.parse_args()

    items = build()
    outputs = [(TARGET, render(items)), (GO, render_go(items))]
    try:
        outputs.append((JS, splice(JS, read_text(JS), render_js_catalog(items))))
        outputs.append((PANE_JS, splice(PANE_JS, read_text(PANE_JS), render_pane_js(items))))
        outputs.append((CHAT_PY, splice(CHAT_PY, read_text(CHAT_PY), render_chat_py(items), PY_BEGIN, PY_END, "")))
        outputs.append((PANE_CS, splice(PANE_CS, read_text(PANE_CS), render_pane_cs(items), indent="        ")))
    except ValueError as err:
        if args.check:
            print("LECH: %s" % err, file=sys.stderr)
            return 1
        raise

    stale = [path for path, text in outputs if read_text(path) != text]
    if args.check:
        if stale:
            print("LECH: chay lai scripts/generate_mcp_commands.py (%s)" % ", ".join(stale), file=sys.stderr)
            return 1
        print("khop: %s" % ", ".join(path for path, _ in outputs))
        return 0

    for path, text in outputs:
        if path in stale:
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w", encoding="utf-8", newline="\n") as handle:
                handle.write(text)
    if stale:
        print("da ghi %s (%d lenh)" % (", ".join(stale), len(items)))
    else:
        print("khong doi (%d lenh)" % len(items))
    return 0


if __name__ == "__main__":
    sys.exit(main())
