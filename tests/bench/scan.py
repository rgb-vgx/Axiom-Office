"""Quet moi cong thuc trong file .ods co chua mot chuoi nao do.

    python scan.py <file.ods> <chuoi> [<chuoi> ...]
"""
import re
import sys
import zipfile

sys.stdout.reconfigure(encoding="utf-8")


def main() -> int:
    path = sys.argv[1]
    needles = sys.argv[2:]
    xml = zipfile.ZipFile(path).read("content.xml").decode("utf-8")
    formulas = re.findall(r'table:formula="([^"]*)"', xml)
    print("tong so cong thuc trong file:", len(formulas))
    for needle in needles:
        hits = [f for f in formulas if needle in f]
        print("%-12s -> %d cong thuc" % (needle, len(hits)))
        for hit in hits[:4]:
            print("    ", hit[:170])
    return 0


if __name__ == "__main__":
    sys.exit(main())
