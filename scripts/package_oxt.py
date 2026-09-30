"""Dong goi extension LibreOffice (src/AxiomOffice.LibreOffice) thanh .oxt - chay duoc tren Windows lan Linux.

    python3 scripts/package_oxt.py [--out dist] [--print-path]

Giu dung cau truc goc cua extension (description.xml, META-INF/manifest.xml, *.xcu, python/...), bo __pycache__
va *.pyc. Ten file: AxiomOffice-LibreOffice-<VERSION>.oxt (VERSION doc tu python/pythonpath/axiom/__init__.py).
Chi dung thu vien chuan.
"""
from __future__ import annotations

import argparse
import os
import re
import sys
import zipfile

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
SOURCE = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice")


def version() -> str:
    init = os.path.join(SOURCE, "python", "pythonpath", "axiom", "__init__.py")
    with open(init, encoding="utf-8") as handle:
        match = re.search(r'VERSION\s*=\s*"([^"]+)"', handle.read())
    if not match:
        raise SystemExit("khong doc duoc VERSION trong " + init)
    return match.group(1)


def files():
    for folder, dirs, names in os.walk(SOURCE):
        dirs[:] = sorted(d for d in dirs if d != "__pycache__")
        for name in sorted(names):
            if name.endswith(".pyc"):
                continue
            path = os.path.join(folder, name)
            yield path, os.path.relpath(path, SOURCE).replace(os.sep, "/")


def build(out_dir: str) -> str:
    os.makedirs(out_dir, exist_ok=True)
    target = os.path.join(out_dir, "AxiomOffice-LibreOffice-%s.oxt" % version())
    temp = target + ".tmp"
    count = 0
    with zipfile.ZipFile(temp, "w", zipfile.ZIP_DEFLATED) as archive:
        for path, name in files():
            archive.write(path, name)
            count += 1
    os.replace(temp, target)
    print("Packaged %s (%d files)" % (target, count), file=sys.stderr)
    return target


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", default=os.path.join(ROOT, "dist"))
    parser.add_argument("--print-path", action="store_true", help="in duong dan .oxt ra stdout (cho script cai)")
    args = parser.parse_args()
    target = build(args.out)
    if args.print_path:
        print(target)
    return 0


if __name__ == "__main__":
    sys.exit(main())
