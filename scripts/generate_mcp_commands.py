"""Sinh `src/AxiomOffice.Mcp/live-commands.json` từ registry lệnh của extension LibreOffice.

Ban Windows lấy danh sách lệnh từ CommandCatalog của add-in (AxiomOffice.Host.exe); bản
`axiom-office-mcp` (net10.0) không có add-in nên nhúng sẵn file này - dùng cho mô tả tool
`word_command`/`excel_command`/`ppt_command`/`wps_live_command`.

File sinh ra chỉ gồm tên lệnh + tham số (không phải nguồn duy nhất của giao thức: bridge vẫn là
nguồn thật). `tests/lo/test_extension.py` so lại file này với registry để không bị lệch.

    python scripts/generate_mcp_commands.py [--check]
"""
from __future__ import annotations

import argparse
import json
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath"))

TARGET = os.path.join(ROOT, "src", "AxiomOffice.Mcp", "live-commands.json")


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


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem tra file co khop registry khong")
    args = parser.parse_args()
    text = render(build())
    current = ""
    if os.path.exists(TARGET):
        with open(TARGET, encoding="utf-8", newline="") as handle:
            current = handle.read().replace("\r\n", "\n")
    if args.check:
        if current == text:
            print("khop: %s" % TARGET)
            return 0
        print("LECH: chay lai scripts/generate_mcp_commands.py", file=sys.stderr)
        return 1
    if current == text:
        print("khong doi: %s" % TARGET)
        return 0
    os.makedirs(os.path.dirname(TARGET), exist_ok=True)
    with open(TARGET, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(text)
    print("da ghi %s (%d lenh)" % (TARGET, len(build())))
    return 0


if __name__ == "__main__":
    sys.exit(main())
