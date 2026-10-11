"""Spec: et.checkRange khong bao "empty-reference" gia (axiom.checks._empty_references) - khong can LibreOffice.

Log 11/10/2026 (dashboard tren sheet ADT that): cong thuc
    =SUMIFS(ADT.$S$2:$S$65;ADT.$F$2:$F$65;"Duong khong";ADT.$J$2:$J$65;"01B1*")
bi bao "Dashboard!B50 -> Dashboard!B1 (trong)" vi regex doc chu "B1" BEN TRONG chuoi "01B1*". Cung mot loai
loi: ten ham co so (LOG10, ATAN2, ...) bi doc thanh o LOG10 / TAN2. Agent mat them vong de "chung minh"
canh bao la gia. Phai: bo qua noi dung trong chuoi "...", va chi nhan tham chieu dung o ranh gioi tu
(khong dinh lien chu/so phia truoc, khong phai ten ham ngay truoc dau "(").
Nhung tham chieu that toi o trong (cung sheet hay sheet khac) van phai duoc bao.

    python3 tests/lo/test_checks.py
"""
from __future__ import annotations

import os
import sys
import types
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
if "uno" not in sys.modules:
    sys.modules["uno"] = types.ModuleType("uno")
sys.path.insert(0, PYTHONPATH)

from axiom import commands  # noqa: E402

commands.load_all()
from axiom import checks  # noqa: E402


def col_row(address: str):
    letters = "".join(ch for ch in address if ch.isalpha())
    digits = "".join(ch for ch in address if ch.isdigit())
    col = 0
    for ch in letters.upper():
        col = col * 26 + (ord(ch) - ord("A") + 1)
    return col - 1, int(digits) - 1


class Cell:
    def __init__(self, content=None):
        self.content = content

    def getFormula(self):
        return "" if self.content is None else str(self.content)

    def getType(self):
        if self.content is None:
            return "EMPTY"
        if isinstance(self.content, str) and self.content.startswith("="):
            return "FORMULA"
        return "VALUE" if isinstance(self.content, (int, float)) else "TEXT"


class Sheet:
    def __init__(self, name, cells=None):
        self.name = name
        self.cells = {col_row(a): Cell(v) for a, v in (cells or {}).items()}

    def getName(self):
        return self.name

    def getCellByPosition(self, col, row):
        return self.cells.get((col, row), Cell())


class Sheets:
    def __init__(self, *sheets):
        self.by_name = {s.name: s for s in sheets}

    def getByName(self, name):
        if name not in self.by_name:
            raise KeyError(name)
        return self.by_name[name]


def scan(formula, extra_cells=None):
    """Dat `formula` vao Dashboard!B50 roi quet dung o do; ADT co du lieu o cot A..Z dong 2..65."""
    adt = Sheet("ADT", {"%s%d" % (chr(ord("A") + c), r): 1 for c in range(26) for r in range(2, 66)})
    cells = {"B50": formula, "A2": 5, "A3": 7}
    cells.update(extra_cells or {})
    dashboard = Sheet("Dashboard", cells)
    doc = types.SimpleNamespace(getSheets=lambda: Sheets(adt, dashboard))
    col, row = col_row("B50")
    bounds = types.SimpleNamespace(StartColumn=col, EndColumn=col, StartRow=row, EndRow=row)
    return checks._empty_references(doc, dashboard, bounds)


class EmptyReferenceTests(unittest.TestCase):
    def test_text_inside_string_is_not_a_reference(self):
        formula = '=SUMIFS(ADT.$S$2:$S$65;ADT.$F$2:$F$65;"Đường không";ADT.$J$2:$J$65;"01B1*")'
        self.assertEqual(scan(formula), [], "chu 'B1' trong chuoi \"01B1*\" khong phai tham chieu")

    def test_string_with_cell_like_text_and_escaped_quotes(self):
        self.assertEqual(scan('=IF(A2="X12";1;0)'), [])
        self.assertEqual(scan('=IF(A2="say ""Q7"" now";1;0)'), [], "chuoi co dau nhay kep lap \"\"")

    def test_function_names_with_digits_are_not_references(self):
        self.assertEqual(scan("=LOG10(A2)"), [])
        self.assertEqual(scan("=ATAN2(A2;A3)"), [])

    def test_real_empty_reference_same_sheet_still_reported(self):
        found = scan("=B16*2")
        self.assertEqual(len(found), 1, found)
        self.assertIn("Dashboard!B16", found[0])

    def test_real_empty_reference_other_sheet_still_reported(self):
        found = scan("=ADT.$Z$99+A2")
        self.assertEqual(len(found), 1, found)
        self.assertIn("ADT!Z99", found[0])

    def test_reference_after_string_still_checked(self):
        found = scan('=IF(A2="01B1*";B16;0)')
        self.assertEqual(len(found), 1, found)
        self.assertIn("Dashboard!B16", found[0])


if __name__ == "__main__":
    unittest.main(verbosity=2)
