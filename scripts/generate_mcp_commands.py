"""Sinh danh sach lenh bridge tu registry lenh cua extension LibreOffice.

Nguon duy nhat: `catalog/live-commands.json` (thay cho `src/AxiomOffice.Mcp/live-commands.json` cu).
Ban Windows lay danh sach lenh tu CommandCatalog cua add-in (AxiomOffice.Host.exe); ban chay doc lap
(MCP server .NET 10 cu, va MCP server Go) khong co add-in nen nhung san file nay - dung cho mo ta cac
tool `word_command`/`ppt_command`/`wps_live_command`.

Ra hai dich, KHONG sua tay:
  - `catalog/live-commands.json`  - nguon cho moi ban (nhu `catalog/setup.json`).
  - `core-go/internal/mcpserver/livecommands_gen.go` - Agent Core ban Go nhung san.

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

HEADER_GO = "// SINH TU DONG tu catalog/live-commands.json bang scripts/generate_mcp_commands.py - KHONG sua tay.\n"


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
    return [{"name": item["name"], "kind": item["kind"],
             "params": [{"name": p["name"], "hint": p["hint"]} for p in item["params"]]}
            for item in commands.catalog()]


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
    lines.append("// liveCommands la danh sach lenh bridge (sinh tu catalog/live-commands.json).\n")
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


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem tra cac file co khop registry khong")
    args = parser.parse_args()

    items = build()
    json_text = render(items)
    go_text = render_go(items)

    if args.check:
        stale = []
        if read_text(TARGET) != json_text:
            stale.append(TARGET)
        if read_text(GO) != go_text:
            stale.append(GO)
        if stale:
            print("LECH: chay lai scripts/generate_mcp_commands.py (%s)" % ", ".join(stale), file=sys.stderr)
            return 1
        print("khop: %s" % ", ".join([TARGET, GO]))
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
    if changed:
        print("da ghi %s (%d lenh)" % (", ".join(changed), len(items)))
    else:
        print("khong doi (%d lenh)" % len(items))
    return 0


if __name__ == "__main__":
    sys.exit(main())
