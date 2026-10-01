"""Dong bo template OOXML nhung san cho MCP server ban Go.

Nguon duy nhat: `src/AxiomOffice.Host/Mcp/Templates/` (ban Windows net48 nhung thang vao
AxiomOffice.Host.exe; ban .NET 10 cu cung nhung tu day).

`go:embed` cua Go khong doc duoc file ngoai module `core-go/`, nen ban Go giu mot BAN SAO trong
`core-go/internal/templates/`. Script nay sinh ban sao do, va `--check` (CI chay) bao loi neu hai
ben lech nhau - de template khong am tham khac nhau giua Windows va Linux.

    python scripts/generate_templates.py [--check]
"""
from __future__ import annotations

import argparse
import os
import shutil
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
SOURCE_DIR = os.path.join(ROOT, "src", "AxiomOffice.Host", "Mcp", "Templates")
TARGET_DIR = os.path.join(ROOT, "core-go", "internal", "templates")

TEMPLATES = ("default.docx", "default.pptx")


def read(path: str) -> bytes:
    with open(path, "rb") as handle:
        return handle.read()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem tra ban sao co khop nguon khong")
    args = parser.parse_args()

    missing = [name for name in TEMPLATES if not os.path.exists(os.path.join(SOURCE_DIR, name))]
    if missing:
        print("thieu template nguon: %s" % ", ".join(missing), file=sys.stderr)
        return 1

    if args.check:
        for name in TEMPLATES:
            source = os.path.join(SOURCE_DIR, name)
            target = os.path.join(TARGET_DIR, name)
            if not os.path.exists(target):
                print("LECH: thieu ban sao %s - chay lai scripts/generate_templates.py" % target, file=sys.stderr)
                return 1
            if read(source) != read(target):
                print("LECH: %s khac %s - chay lai scripts/generate_templates.py" % (target, source), file=sys.stderr)
                return 1
        print("khop: %s" % TARGET_DIR)
        return 0

    os.makedirs(TARGET_DIR, exist_ok=True)
    changed = []
    for name in TEMPLATES:
        source = os.path.join(SOURCE_DIR, name)
        target = os.path.join(TARGET_DIR, name)
        if os.path.exists(target) and read(source) == read(target):
            continue
        shutil.copyfile(source, target)
        changed.append(name)
    if changed:
        print("da cap nhat %s: %s" % (TARGET_DIR, ", ".join(changed)))
    else:
        print("khong doi: %s" % TARGET_DIR)
    return 0


if __name__ == "__main__":
    sys.exit(main())
