"""Spec: et.checkRange (axiom.checks.range_report) phan loai dung va nhan dien dong tieu de - khong can LibreOffice.

Log 11/10/2026 (bang "CHI PHI VAN CHUYEN" tren Dashboard, A48:D53) bao 5 canh bao gia lam agent mat vong:
1. Ket qua cong thuc so duoc dinh dang (0.0%, #,##0, ngay) bi dem la "texts" vi phan loai theo CHUOI HIEN THI
   ("66.3%" khong parse duoc). Phai phan loai theo gia tri that: o VALUE la so; o FORMULA theo kieu ket qua
   (FormulaResultType2: 1 = so, 2 = chuoi, 4 = loi).
2. Dong dau la dong TIEU DE (chi o dau co chu, cac o con lai trong) ma dong sau la header that -> bao
   "empty-header" cho B..D va "mixed-types" vi dong header bi dem nhu du lieu. Phai: nhan dong do la tieu de
   (tra ve "title"), lay dong ke tiep lam header.
3. "numberFormat" lay o dong ngay sau dong dau (= dong header) nen luon "General"; phai lay o dong du lieu dau.
Canh bao THAT (header trong that, tron so va chu that, so luu dang chu) van phai con.

    python3 tests/lo/test_range_report.py
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

FORMATS = {0: "General", 2: "#,##0", 3: "0.0%", 4: "DD/MM/YYYY"}


def col_row(address):
    letters = "".join(ch for ch in address if ch.isalpha())
    digits = "".join(ch for ch in address if ch.isdigit())
    col = 0
    for ch in letters.upper():
        col = col * 26 + (ord(ch) - ord("A") + 1)
    return col - 1, int(digits) - 1


class Cell:
    """kind: EMPTY / TEXT / VALUE / FORMULA. shown = chuoi hien thi (getString), value = so that."""

    def __init__(self, kind="EMPTY", shown="", value=0.0, formula="", result_type=1, fmt=0, error=0):
        self.kind, self.shown, self.value, self.formula = kind, shown, value, formula
        self.FormulaResultType2, self.fmt, self.error = result_type, fmt, error

    def getType(self):
        return self.kind

    def getString(self):
        return self.shown

    def getValue(self):
        return self.value

    def getFormula(self):
        if self.kind == "FORMULA":
            return self.formula
        return self.shown if self.kind == "TEXT" else ("" if self.kind == "EMPTY" else repr(self.value))

    def getError(self):
        return self.error

    def getPropertyValue(self, name):
        if name == "NumberFormat":
            return self.fmt
        if name == "FormulaResultType2":
            return self.FormulaResultType2
        raise KeyError(name)


def text(s):
    return Cell("TEXT", s)


def number(v, shown=None, fmt=0):
    return Cell("VALUE", shown if shown is not None else ("%g" % v), v, fmt=fmt)


def formula(v, shown, fmt=0, f="=1"):
    return Cell("FORMULA", shown, v, formula=f, result_type=1, fmt=fmt)


class Bounds(types.SimpleNamespace):
    pass


class Cursor:
    def __init__(self, sheet, bounds):
        self.sheet, self.bounds = sheet, bounds

    def gotoStartOfUsedArea(self, expand):
        pass

    def gotoEndOfUsedArea(self, expand):
        pass

    def collapseToCurrentRegion(self):
        pass

    def getRangeAddress(self):
        return self.bounds


class Sheet:
    def __init__(self, name, cells):
        self.name = name
        self.cells = {col_row(a): c for a, c in cells.items()}

    def getName(self):
        return self.name

    def getCellByPosition(self, c, r):
        return self.cells.get((c, r), Cell())

    def used(self):
        cols = [c for c, _ in self.cells] or [0]
        rows = [r for _, r in self.cells] or [0]
        return Bounds(StartColumn=min(cols), EndColumn=max(cols), StartRow=min(rows), EndRow=max(rows))

    def createCursor(self):
        return Cursor(self, self.used())

    def createCursorByRange(self, cell):
        return Cursor(self, self.used())

    def getCellRangeByName(self, address):
        parts = address.split(":")
        c1, r1 = col_row(parts[0])
        c2, r2 = col_row(parts[-1])
        bounds = Bounds(StartColumn=c1, EndColumn=c2, StartRow=r1, EndRow=r2)
        return types.SimpleNamespace(getRangeAddress=lambda: bounds)


class Doc:
    def __init__(self, sheet):
        self.sheet = sheet

    def getSheets(self):
        sheet = self.sheet
        return types.SimpleNamespace(getElementNames=lambda: (sheet.name,), getByName=lambda name: sheet)

    def getCurrentController(self):
        return types.SimpleNamespace(getActiveSheet=lambda: self.sheet)

    def getNumberFormats(self):
        return types.SimpleNamespace(getByKey=lambda key: types.SimpleNamespace(FormatString=FORMATS.get(key, "General")))


def report(cells, rng, name="Dashboard"):
    sheet = Sheet(name, cells)
    env = types.SimpleNamespace(document=Doc(sheet))
    return checks.range_report(env, {"range": rng, "sheet": name})


def kinds(result):
    return sorted({i["type"] for i in result["issues"]})


TITLE = "CHI PHÍ VẬN CHUYỂN THEO PHƯƠNG THỨC - NỬA ĐẦU NĂM 2026"


def shipping_table():
    cells = {"A48": text(TITLE),
             "A49": text("Phương thức vận chuyển"), "B49": text("Chi phí vận chuyển (VND)"),
             "C49": text("Tỷ lệ %"), "D49": text("Số chuyến")}
    rows = [("Air", 303014392, 0.6629, 22), ("Sea", 73603902, 0.1610, 16),
            ("Express", 80488881, 0.1761, 17), ("Tổng cộng", 457107175, 1.0, 55)]
    for index, (label, cost, share, trips) in enumerate(rows):
        r = 50 + index
        cells["A%d" % r] = text(label)
        cells["B%d" % r] = formula(cost, "{:,}".format(cost), fmt=2)
        cells["C%d" % r] = formula(share, "%.1f%%" % (share * 100), fmt=3)
        cells["D%d" % r] = formula(trips, str(trips))
    return cells


class RangeReportTests(unittest.TestCase):
    def test_title_row_and_formatted_formula_results(self):
        result = report(shipping_table(), "A48:D53")
        self.assertEqual(kinds(result), [], result["issues"])
        self.assertEqual(result.get("title"), TITLE, "dong dau chi co o A la tieu de")
        columns = {c["column"]: c for c in result["columns"]}
        self.assertEqual(columns["B"]["header"], "Chi phí vận chuyển (VND)")
        for letter in "BCD":
            self.assertEqual((columns[letter]["numbers"], columns[letter]["texts"]), (4, 0),
                             "cot %s: ket qua cong thuc la so (du hien thi '66.3%%' / '303,014,392')" % letter)
        self.assertEqual(columns["C"]["numberFormat"], "0.0%", "numberFormat lay tu dong du lieu dau")
        self.assertEqual(columns["B"]["numberFormat"], "#,##0")

    def test_date_values_count_as_numbers(self):
        cells = {"A1": text("Ngày ĐK"), "B1": text("Trị giá")}
        for index, (serial, shown) in enumerate([(46107, "26/03/2026"), (46111, "30/03/2026")]):
            cells["A%d" % (index + 2)] = number(serial, shown, fmt=4)
            cells["B%d" % (index + 2)] = number(100 + index)
        result = report(cells, "A1:B3", "ADT")
        self.assertEqual(kinds(result), [], result["issues"])
        self.assertEqual(result["columns"][0]["numbers"], 2)

    def test_plain_header_table_unchanged(self):
        cells = {"A1": text("Tên"), "B1": text("Điểm"), "A2": text("An"), "B2": number(8), "A3": text("Bình"), "B3": number(7)}
        result = report(cells, "A1:B3", "Sheet1")
        self.assertIsNone(result.get("title"))
        self.assertEqual([c["header"] for c in result["columns"]], ["Tên", "Điểm"])
        self.assertEqual(kinds(result), [])

    def test_real_problems_still_reported(self):
        cells = {"A1": text("Tên"), "B1": text(""), "C1": text("Ghi chú"),
                 "A2": text("An"), "B2": number(8), "C2": number(5),
                 "A3": text("Bình"), "B3": number(7), "C3": text("vắng"),
                 "A4": text("Chi"), "B4": number(6), "C4": text("đi muộn")}
        result = report(cells, "A1:C4", "Sheet1")
        self.assertIn("empty-header", kinds(result), "cot B co du lieu ma header trong")
        self.assertIn("mixed-types", kinds(result), "cot C tron so va chu that")
        cells = {"A1": text("Trị giá"), "A2": text("15.553,8"), "A3": text("21,294"), "A4": number(10)}
        self.assertIn("numbers-as-text", kinds(report(cells, "A1:A4", "ADT")))


if __name__ == "__main__":
    unittest.main(verbosity=2)
