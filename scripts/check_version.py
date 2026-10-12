"""Kiem tra so phien ban san pham khong lech nhau.

VERSION o goc repo la nguon duy nhat; 4 noi duoi day chi la ban sao duoc canh:
  - src/AxiomOffice.LibreOffice/python/pythonpath/axiom/__init__.py (VERSION = "x")
  - src/AxiomOffice.LibreOffice/description.xml (<version value="x"/>)
  - src/AxiomOffice.WPS/main.js (var VERSION = "x";)
  - core-go/cmd/axiom-core/main.go (var version = "x")

CO Y khong canh src/AxiomOffice/Properties/AssemblyInfo.cs (1.0.0.0): nhanh C# dong bang -
Windows bao so cua DLL, day la sai lech da biet, khong phai loi.

    python3 scripts/check_version.py [--root DIR]
"""
from __future__ import annotations

import argparse
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

CHECKS = [
    (
        os.path.join("src", "AxiomOffice.LibreOffice", "python", "pythonpath", "axiom", "__init__.py"),
        r'VERSION\s*=\s*"([^"]+)"',
    ),
    (
        os.path.join("src", "AxiomOffice.LibreOffice", "description.xml"),
        r'<version\s+value="([^"]+)"\s*/>',
    ),
    (
        os.path.join("src", "AxiomOffice.WPS", "main.js"),
        r'var\s+VERSION\s*=\s*"([^"]+)"\s*;',
    ),
    (
        os.path.join("core-go", "cmd", "axiom-core", "main.go"),
        r'var\s+version\s*=\s*"([^"]+)"',
    ),
]


def read_version(root: str) -> str:
    path = os.path.join(root, "VERSION")
    try:
        with open(path, encoding="utf-8") as handle:
            value = handle.read().strip()
    except OSError:
        print("LECH: khong doc duoc VERSION trong %s" % path, file=sys.stderr)
        raise SystemExit(1)
    if not value:
        print("LECH: khong doc duoc VERSION trong %s (file rong)" % path, file=sys.stderr)
        raise SystemExit(1)
    return value


def read_copy(root: str, rel: str, pattern: str) -> str | None:
    path = os.path.join(root, rel)
    try:
        with open(path, encoding="utf-8") as handle:
            text = handle.read()
    except OSError:
        return None
    match = re.search(pattern, text)
    if not match:
        return None
    return match.group(1)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=ROOT)
    args = parser.parse_args()
    root = os.path.abspath(args.root)

    expected = read_version(root)

    bad = []
    for rel, pattern in CHECKS:
        got = read_copy(root, rel, pattern)
        if got is None:
            print("LECH: %s: khong doc duoc gia tri, mong doi \"%s\"" % (rel, expected), file=sys.stderr)
            bad.append(rel)
        elif got != expected:
            print("LECH: %s: doc duoc \"%s\", mong doi \"%s\"" % (rel, got, expected), file=sys.stderr)
            bad.append(rel)
    if bad:
        return 1
    print("khop: VERSION=%s (%d cho)" % (expected, len(CHECKS)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
