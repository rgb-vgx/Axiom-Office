"""Spec: writer.checkTables bo qua "empty-cells" cho BANG DAN TRANG (khong can LibreOffice).

Log 11/10/2026 (review 8 skill): van-ban-hanh-chinh dung bang KHONG KE KHUNG lam bo cuc (phan dau, phan
cuoi), trong do o trong la CO Y - khoang trong ky, o cho dien. the-thuc-van-ban lai bao "sua empty-cells
(dien hoac ghi —)", va hai skill nay von duoc thiet ke de dung chung. Model lam dung huong dan se nhet "—"
vao cho ky, tuc la pha dung the thuc van ban.

Nen: formatTable {borders:false} danh DAU bang (writer.LAYOUT_TABLE_NAME), checkTables doc tem do roi bo qua
empty-cells. Khong doan theo net ke duoc, vi bang DU LIEU chua dinh dang cung khong co net ke nao - do la
chinh truong hop hai live test dang kiem (chen bang tran roi doi checkTables bat empty-cells).

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
# test_calc.py chay truoc (discover xep theo abc) va cam mot module "uno" RONG. Neu o day chi dat stub khi
# "uno" CHUA co thi stub yeu do thang, va _set_borders (writer.py:336) do voi "uno" khong co createUnoStruct
# -> do o `python3 -m unittest discover -s tests/lo` (lenh CI) trong khi chay rieng file nay van xanh. Nen
# bo sung vao module dang co, dung bo qua khi no da ton tai.
if not hasattr(sys.modules["uno"], "createUnoStruct"):
    sys.modules["uno"].createUnoStruct = lambda _name: types.SimpleNamespace()
sys.path.insert(0, PYTHONPATH)

from axiom import commands  # noqa: E402

commands.load_all()
from axiom import checks, writer  # noqa: E402


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


# ---------------------------------------------------------------- bang dan trang
#
# FakeTable mo phong dung phan ma writer/checks dung: getRows/getColumns/getCellByPosition, TableBorder2
# (cho _set_borders) va Name. Dat ten giong LibreOffice that: ten da co chu bang khac thi nem loi
# (unotbl.cxx:2971 - do tren LO that 11/10/2026), ten dang co san thi dat lai cung nem loi.

class FakeTable:
    def __init__(self, values, name="Table1", taken=()):
        self.values = values
        self.taken = set(taken)
        self.name_sets = 0
        self.TableBorder2 = types.SimpleNamespace()
        self._name = name

    @property
    def Name(self):
        return self._name

    @Name.setter
    def Name(self, value):
        self.name_sets += 1
        if value in self.taken or value == self._name:
            raise RuntimeError("name already in use")
        self._name = value

    def getRows(self):
        return types.SimpleNamespace(getCount=lambda: len(self.values))

    def getColumns(self):
        return types.SimpleNamespace(getCount=lambda: len(self.values[0]))

    def getCellByPosition(self, c, r):
        return types.SimpleNamespace(getString=lambda: self.values[r][c])


def run_check_tables(tables):
    env = types.SimpleNamespace(document=types.SimpleNamespace())
    original = writer.tables_in_order
    writer.tables_in_order = lambda _doc: tables
    try:
        return checks.writer_check_tables(env, {})
    finally:
        writer.tables_in_order = original


def issue_types(result, index=None):
    return {i["type"] for i in result["issues"] if index is None or i.get("table") == index}


# Bang phan cuoi van ban hanh chinh: khoang trong ky nam trong dong CO chu ("- Nhu tren;").
LAYOUT_VALUES = [["Nơi nhận:", "TM. ỦY BAN NHÂN DÂN"], ["- Như trên;", ""], ["- Lưu: VT", "Nguyễn Văn A"]]
# Bang QA cua live test: chu lot vao o tieu de + o trong cuoi bang.
DATA_VALUES = [["Lịch họp áp dụng cho toàn bộ phòng ban - thời gian có thể điều chỉnh.Thứ", "Giờ", "Nội dung"],
               ["Hai", "8h", ""]]


class LayoutTableTests(unittest.TestCase):
    def test_borderless_data_table_is_still_reported(self):
        """Bang du lieu chua dinh dang KHONG co net ke nao -> van phai bat empty-cells (live test doi vay)."""
        result = run_check_tables([FakeTable(DATA_VALUES)])
        self.assertFalse(result["tables"][0]["layout"])
        self.assertIn("empty-cells", issue_types(result, 1))
        self.assertIn("long-header-cell", issue_types(result, 1))

    def test_marked_layout_table_is_exempt_from_empty_cells(self):
        result = run_check_tables([FakeTable(LAYOUT_VALUES, name="AxiomLayout")])
        self.assertTrue(result["tables"][0]["layout"], "tem phai duoc nhan ra")
        self.assertNotIn("empty-cells", issue_types(result, 1))
        self.assertEqual(result["issueCount"], 0, result["issues"])
        # O trong van hien trong report de agent thay, chi la khong thanh issue.
        self.assertEqual(result["tables"][0]["emptyCells"], 1)

    def test_long_header_cell_still_reported_on_layout_table(self):
        """Chu lot vao o tieu de la loi that voi moi bang - van-ban-hanh-chinh van doi soat no.

        O dai o day la chu that, khong phai khoang trang: _cell_text cat khoang trang hai dau truoc khi do.
        """
        table = FakeTable([["Lịch họp áp dụng cho toàn bộ phòng ban - thời gian có thể điều chỉnh.Thứ", "Giờ", "Nội dung"],
                           ["- Như trên;", "x", ""]], name="AxiomLayout")
        result = run_check_tables([table])
        self.assertIn("long-header-cell", issue_types(result, 1))
        self.assertNotIn("empty-cells", issue_types(result, 1), "bang dan trang van duoc mien empty-cells")

    # Muc 4.7 docs/skills-review.md: nguong "3 x trung vi(do dai o tieu de)" voi bang 2 cot lay trung vi cua
    # CHINH HAI O do, tuc la o dai hon -> o dai nhat luon co len == typical nen khong bao gio vuot 3 x typical.
    # Hai bang cua van-ban-hanh-chinh deu 2 cot, nen loi hua "khong duoc co long-header-cell" cua no khong the
    # sai. Nay moi o so voi trung vi cac o tieu de CON LAI (leave-one-out).

    def test_two_column_table_with_paragraph_in_header_is_reported(self):
        """Bang 2 cot: doan van go lot vao o tieu de phai bat duoc (truoc 4.7: khong bao gio bat)."""
        table = FakeTable([["Nơi nhận:", "Lịch họp áp dụng cho toàn bộ phòng ban - thời gian có thể điều chỉnh."],
                           ["- Như trên;", "Hai"]], name="AxiomLayout")
        result = run_check_tables([table])
        self.assertIn("long-header-cell", issue_types(result, 1), result["issues"])

    def test_two_column_table_with_legal_header_is_not_reported(self):
        """...nhung KHONG duoc bao oan bang 2 cot that: quoc hieu 34 ky tu la chu hop le, dai nhat bang."""
        table = FakeTable([["CỤC THUẾ TỈNH X", "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM"],
                           ["Số: 12/CT-VP", "Độc lập - Tự do - Hạnh phúc"]], name="AxiomLayout")
        result = run_check_tables([table])
        self.assertNotIn("long-header-cell", issue_types(result, 1), result["issues"])

    def test_all_long_headers_are_not_reported(self):
        """Ti-le con giu tac dung: bang ma MOI tieu de deu dai thi mot o dai hon chut khong phai loi."""
        table = FakeTable([["Tình hình thực hiện kế hoạch quý III năm 2026 toàn tỉnh", "Kết quả ước tính đạt được",
                            "Nguyên nhân và bài học kinh nghiệm rút ra", "x"]])
        result = run_check_tables([table])
        self.assertNotIn("long-header-cell", issue_types(result, 1), result["issues"])

    def test_single_column_table_is_left_alone(self):
        """Bang 1 cot khong co nhom de so -> khong bao (khong co ti le nao noi o do la BAT THUONG)."""
        table = FakeTable([["Lịch họp áp dụng cho toàn bộ phòng ban - thời gian có thể điều chỉnh."]])
        result = run_check_tables([table])
        self.assertNotIn("long-header-cell", issue_types(result, 1), result["issues"])

    def test_mark_is_per_table(self):
        result = run_check_tables([FakeTable(LAYOUT_VALUES, name="AxiomLayout"), FakeTable(DATA_VALUES, name="Table2")])
        self.assertTrue(result["tables"][0]["layout"])
        self.assertFalse(result["tables"][1]["layout"])
        self.assertNotIn("empty-cells", issue_types(result, 1))
        self.assertIn("empty-cells", issue_types(result, 2))

    def test_borders_false_marks_the_table_and_drops_the_borders(self):
        """Duong that: formatTable {borders:false} -> _set_borders -> dat tem ma checkTables doc."""
        table = FakeTable(LAYOUT_VALUES, name="Table2")
        writer._set_borders(table, False)
        self.assertEqual(table.TableBorder2.TopLine.LineWidth, 0, "bo khung that su")
        self.assertTrue(writer.is_layout_table(table), table.Name)
        result = run_check_tables([table])
        self.assertNotIn("empty-cells", issue_types(result, 1))

    def test_borders_true_removes_the_mark_again(self):
        table = FakeTable(LAYOUT_VALUES, name="Table1")
        writer._set_borders(table, False)
        self.assertTrue(writer.is_layout_table(table))
        writer._set_borders(table, True)
        self.assertFalse(writer.is_layout_table(table), table.Name)
        result = run_check_tables([table])
        self.assertIn("empty-cells", issue_types(result, 1))

    def test_rename_avoids_a_taken_name(self):
        table = FakeTable(LAYOUT_VALUES, name="Table1", taken=[writer.LAYOUT_TABLE_NAME])
        self.assertTrue(writer._mark_layout(table, True))
        self.assertNotEqual(table.Name, writer.LAYOUT_TABLE_NAME, "trung ten thi phai lui ve ten khac")
        self.assertTrue(writer.is_layout_table(table), table.Name)

    def test_rename_gives_up_without_raising(self):
        """Het cach dat ten thi formatTable khong duoc do: chi la mat tem, phan con lai van chay."""
        taken = [writer.LAYOUT_TABLE_NAME] + ["%s-%d" % (writer.LAYOUT_TABLE_NAME, n) for n in range(1, 60)]
        table = FakeTable(LAYOUT_VALUES, name="Table1", taken=taken)
        self.assertFalse(writer._mark_layout(table, True))
        self.assertEqual(table.Name, "Table1")

    def test_marking_again_does_not_touch_the_name(self):
        """Goi lai formatTable {borders:false} lan hai khong duoc dat ten lai (LO nem loi khi dat lai dung
        ten dang co - do tren LO that 11/10/2026)."""
        table = FakeTable(LAYOUT_VALUES, name="AxiomLayout")
        table.name_sets = 0
        self.assertTrue(writer._mark_layout(table, True))
        self.assertEqual(table.name_sets, 0, "khong duoc dat ten lai khi da co tem")

    def test_removing_the_mark_does_not_leave_a_marker_name(self):
        table = FakeTable(LAYOUT_VALUES, name="AxiomLayout")
        self.assertTrue(writer._mark_layout(table, False))
        self.assertFalse(writer.is_layout_table(table))
        self.assertFalse(table.Name.startswith(writer.LAYOUT_TABLE_NAME), table.Name)


if __name__ == "__main__":
    unittest.main(verbosity=2)
