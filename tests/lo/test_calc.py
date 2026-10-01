"""Unit test cho axiom.calc - chay KHONG can LibreOffice (sheet/o deu la do gia trong file nay).

Giu dung phan de sai nhat: dau phan cach tham so cong thuc. Mot so ban LibreOffice doi ';' va khong
con muc Tools > Options > Calc > Formula > Separators de doi, nen cong thuc kieu en-US do agent sinh
ra bi Err:508 - trong khi Excel qua COM luon nhan ','. Truoc day et.writeRange ghi xong la tra ve
"written": n, khong ai biet cong thuc hong.

    python tests\\lo\\test_calc.py
"""
from __future__ import annotations

import os
import sys
import types
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
if "uno" not in sys.modules:      # axiom.calc chi dung uno o trong ham (createUnoStruct), khong luc import
    sys.modules["uno"] = types.ModuleType("uno")
sys.path.insert(0, PYTHONPATH)

from axiom import commands  # noqa: E402

# Nap truoc theo dung thu tu chuan (general, writer, calc, impress, checks): file nay chay truoc
# test_extension.py (xep theo abc) nen neu import thang axiom.calc thi lenh cua calc duoc dang ky
# som, lam lech thu tu registry ma test_mcp_live_commands_khop_registry so voi live-commands.json.
commands.load_all()

from axiom import calc  # noqa: E402


# ---------------------------------------------------------------- do gia

class FakeCell:
    """O gia mo phong LibreOffice: dau phan cach sai -> 508; cong thuc trong `broken` -> value_error."""

    def __init__(self, separator=",", value_error=0, broken=(), unparsable=()):
        self.separator, self.value_error = separator, value_error
        self.broken, self.unparsable = set(broken), set(unparsable)
        self.formula, self.writes, self._error = "", [], 0

    def setFormula(self, formula: str) -> None:
        self.writes.append(formula)
        self.formula = formula
        if not formula.startswith("="):
            self._error = 0
        elif formula in self.unparsable:
            self._error = 508                      # hong vi ly do khac (thieu ngoac...), doi dau vo ich
        elif calc._with_separator(formula, self.separator) is not None:
            self._error = 508                      # con dau phan cach khac loai -> khong doc duoc
        elif formula in self.broken:
            self._error = self.value_error         # doc duoc nhung tinh ra loi (#DIV/0!...)
        else:
            self._error = 0

    def setValue(self, value) -> None:
        self.writes.append(value)
        self.formula, self._error = str(value), 0

    def setString(self, text: str) -> None:
        self.writes.append(text)
        self.formula, self._error = text, 0

    def getError(self) -> int:
        return self._error

    def getString(self) -> str:
        if self._error == 508:
            return "Err:508"
        return "#DIV/0!" if self._error else self.formula


class FakeRange:
    def __init__(self, sheet, col: int, row: int):
        self.sheet, self.col, self.row = sheet, col, row
        self.RangeAddress = types.SimpleNamespace(StartColumn=col, StartRow=row)

    def getCellByPosition(self, c: int, r: int) -> FakeCell:
        return self.sheet.cell(self.col + c, self.row + r)


class FakeSheet:
    """Sheet gia: luoi o tao theo nhu cau, du de chay calc.write_range."""

    def __init__(self, separator: str = ",", value_error: int = 0, broken=(), unparsable=()):
        self.name, self.cells = "Sheet1", {}
        self.separator, self.value_error = separator, value_error
        self.broken, self.unparsable = tuple(broken), tuple(unparsable)

    def getName(self) -> str:
        return self.name

    def cell(self, col: int, row: int) -> FakeCell:
        if (col, row) not in self.cells:
            self.cells[(col, row)] = FakeCell(self.separator, self.value_error, self.broken, self.unparsable)
        return self.cells[(col, row)]

    def getCellRangeByName(self, address: str) -> FakeRange:
        letters = "".join(ch for ch in address if ch.isalpha())
        digits = "".join(ch for ch in address if ch.isdigit())
        col = 0
        for ch in letters.upper():
            col = col * 26 + (ord(ch) - ord("A") + 1)
        return FakeRange(self, col - 1, int(digits) - 1)

    def getCellRangeByPosition(self, col: int, row: int, end_col: int, end_row: int) -> FakeRange:
        return FakeRange(self, col, row)


class FakeDoc:
    def __init__(self, sheet):
        self.sheet = sheet

    def getSheets(self):
        return self                              # du cho _ensure_sheet: getElementNames + getByName

    def getElementNames(self):
        return ("Sheet1",)

    def getByName(self, name):
        return self.sheet

    def getCurrentController(self):
        return types.SimpleNamespace(getActiveSheet=lambda: self.sheet)


def write(sheet, values, address: str = "F2"):
    env = types.SimpleNamespace(document=FakeDoc(sheet))
    return calc.write_range(env, {"range": address, "values": values})


# ---------------------------------------------------------------- doi dau phan cach

class SeparatorTests(unittest.TestCase):
    def test_doi_phay_sang_cham_phay(self):
        self.assertEqual(calc._with_separator("=ROUND(AVERAGE(C2:E2),1)", ";"), "=ROUND(AVERAGE(C2:E2);1)")

    def test_doi_cham_phay_sang_phay(self):
        self.assertEqual(calc._with_separator("=SUM(A1;B1)", ","), "=SUM(A1,B1)")

    def test_khong_co_gi_doi_thi_tra_ve_None(self):
        self.assertIsNone(calc._with_separator("=SUM(A1,B1)", ","))
        self.assertIsNone(calc._with_separator("=B2*2", ";"))

    def test_dau_phay_trong_chuoi_duoc_giu_nguyen(self):
        self.assertEqual(calc._with_separator('=IF(A1>0,"Trung bình, khá","Yếu")', ";"),
                         '=IF(A1>0;"Trung bình, khá";"Yếu")')

    def test_ngoac_kep_doi_de_thoat_ky_tu(self):
        # "" la dau " duoc thoat, khong lam lech trang thai trong/ngoai chuoi
        self.assertEqual(calc._with_separator('=IF(A1,"nói ""vâng, thưa""","khác")', ";"),
                         '=IF(A1;"nói ""vâng, thưa""";"khác")')


# ---------------------------------------------------------------- ghi cong thuc

class WriteFormulaTests(unittest.TestCase):
    def test_o_dung_dau_phay_thi_ghi_mot_lan(self):
        cell = FakeCell(",")
        self.assertEqual(calc._write_formula(cell, "=SUM(A1,B1)", {}), (0, "=SUM(A1,B1)"))
        self.assertEqual(cell.writes, ["=SUM(A1,B1)"])

    def test_o_doi_cham_phay_thi_tu_doi_va_nho_lai(self):
        cell, state = FakeCell(";"), {}
        self.assertEqual(calc._write_formula(cell, "=SUM(A1,B1)", state), (0, "=SUM(A1;B1)"))
        self.assertEqual(cell.writes, ["=SUM(A1,B1)", "=SUM(A1;B1)"])
        self.assertEqual(state["separator"], ";")
        # O sau trong cung lan ghi: da biet dau phan cach nen chi ghi mot lan
        other = FakeCell(";")
        self.assertEqual(calc._write_formula(other, "=ROUND(1,5)", state), (0, "=ROUND(1;5)"))
        self.assertEqual(other.writes, ["=ROUND(1;5)"])

    def test_loi_luc_tinh_thi_khong_doi_dau(self):
        # 532 = #DIV/0!: dau phan cach da dung, doi sang kieu kia chi lam cong thuc hong them
        cell = FakeCell(";", 532, broken=("=SUM(A1;B1)",))
        self.assertEqual(calc._write_formula(cell, "=SUM(A1;B1)", {}), (532, "=SUM(A1;B1)"))
        self.assertEqual(cell.writes, ["=SUM(A1;B1)"])

    def test_doi_dau_khong_giup_thi_tra_lai_ban_agent_viet(self):
        # 508 nhung khong phai vi dau phan cach (thieu ngoac): ca hai kieu van 508
        cell = FakeCell(";", unparsable=("=SUM(A1,B1", "=SUM(A1;B1"))
        self.assertEqual(calc._write_formula(cell, "=SUM(A1,B1", {}), (508, "=SUM(A1,B1"))
        self.assertEqual(cell.writes, ["=SUM(A1,B1", "=SUM(A1;B1", "=SUM(A1,B1"])   # da tra lai ban dau


# ---------------------------------------------------------------- ket qua write_range

class WriteRangeTests(unittest.TestCase):
    def test_bao_cong_thuc_con_loi_trong_ket_qua(self):
        sheet = FakeSheet(separator=";", value_error=532, broken=("=ROUND(AVERAGE(C2:C9);1)",))
        result = write(sheet, [["Điểm", "TB"], [8.5, "=ROUND(AVERAGE(C2:C9),1)"]])
        self.assertEqual(result["written"], 4)
        self.assertEqual(result["sheet"], "Sheet1")
        self.assertEqual(result["formulaErrors"],
                         [{"cell": "G3", "formula": "=ROUND(AVERAGE(C2:C9);1)", "error": 532, "text": "#DIV/0!"}])

    def test_khong_co_loi_thi_khong_co_truong_formulaErrors(self):
        sheet = FakeSheet(separator=";")
        result = write(sheet, [["Điểm", "Gấp đôi"], [8.5, "=ROUND(F2*2,1)"]])
        self.assertNotIn("formulaErrors", result)
        self.assertEqual(sheet.cell(6, 2).formula, "=ROUND(F2*2;1)")

    def test_so_va_chuoi_thuong_van_ghi_nhu_cu(self):
        sheet = FakeSheet(separator=";")
        write(sheet, [["Tên", None], ["An", 9.5]])
        self.assertEqual(sheet.cell(5, 1).formula, "Tên")     # chuoi thuong -> setString
        self.assertEqual(sheet.cell(6, 1).formula, "")        # None -> xoa o
        self.assertEqual(sheet.cell(6, 2).formula, "9.5")     # so -> setValue


# ---------------------------------------------------------------- quan ly sheet

class FakeNamedSheet:
    def __init__(self, name: str):
        self.name = name

    def getName(self) -> str:
        return self.name

    def setName(self, name: str) -> None:
        self.name = name


class FakeBook:
    """So gia du de chay et.addSheet/et.renameSheet.

    Giu danh sach theo THU TU cac doi tuong sheet (khong phai danh sach ten), de setName() doi ten that
    giong LibreOffice: getElementNames() doc ten hien tai cua tung sheet.
    """

    def __init__(self, names=("Sheet1",)):
        self.order = [FakeNamedSheet(name) for name in names]
        self.active = names[0]
        self.title = "Untitled 1"

    def names(self):
        return [sheet.getName() for sheet in self.order]

    def getTitle(self) -> str:
        return self.title

    def getSheets(self):
        return self

    def getElementNames(self):
        return tuple(self.names())

    def getByName(self, name):
        for sheet in self.order:
            if sheet.getName() == name:
                return sheet
        raise KeyError(name)

    def insertNewByName(self, name: str, position: int) -> None:
        self.order.insert(position, FakeNamedSheet(name))

    def getCurrentController(self):
        book = self

        def active():
            return book.getByName(book.active)

        def set_active(sheet):
            book.active = sheet.getName()

        return types.SimpleNamespace(getActiveSheet=active, setActiveSheet=set_active)


def book_env(book):
    return types.SimpleNamespace(document=book)


class SheetTests(unittest.TestCase):
    """et.addSheet/et.renameSheet - khong co hai lenh nay thi agent khong dung duoc so nhieu sheet tren
    tai lieu dang mo (log 02/10/2026: model thu ~30 ten lenh tu nghi ra roi bo cuoc)."""

    def test_them_sheet_dat_ten_va_chuyen_sang_active(self):
        book = FakeBook()
        result = calc.add_sheet(book_env(book), {"name": "Raw_Data"})
        self.assertEqual(result["sheet"], "Raw_Data")
        self.assertEqual(result["sheets"], ["Sheet1", "Raw_Data"])
        self.assertEqual(book.active, "Raw_Data", "sheet moi phai duoc chuyen sang de nguoi dung nhin thay")

    def test_bo_trong_ten_thi_dat_ten_kieu_excel_va_tranh_trung(self):
        book = FakeBook(("Sheet1", "Sheet2", "Raw_Data"))
        self.assertEqual(calc.add_sheet(book_env(book), {})["sheet"], "Sheet3")

    def test_trung_ten_thi_bao_loi_kem_danh_sach(self):
        book = FakeBook(("Sheet1",))
        with self.assertRaises(calc.values.ParamError) as caught:
            calc.add_sheet(book_env(book), {"name": "Sheet1"})
        self.assertIn("already exists", str(caught.exception))
        self.assertIn("Sheet1", str(caught.exception))

    def test_index_chen_dung_vi_tri_va_kep_trong_khoang(self):
        book = FakeBook(("A", "B"))
        calc.add_sheet(book_env(book), {"name": "Giua", "index": 1})
        self.assertEqual(book.names(), ["A", "Giua", "B"])
        calc.add_sheet(book_env(book), {"name": "Cuoi", "index": 99})
        self.assertEqual(book.names(), ["A", "Giua", "B", "Cuoi"], "index qua lon thi them vao cuoi")

    def test_doi_ten_sheet(self):
        book = FakeBook(("Sheet1", "Raw"))
        result = calc.rename_sheet(book_env(book), {"sheet": "Sheet1", "name": "Dashboard"})
        self.assertEqual(result["sheets"], ["Dashboard", "Raw"])
        self.assertEqual(book.getByName("Dashboard").getName(), "Dashboard")

    def test_doi_ten_sheet_khong_ton_tai(self):
        book = FakeBook(("Sheet1",))
        with self.assertRaises(calc.values.ParamError) as caught:
            calc.rename_sheet(book_env(book), {"sheet": "KhongCo", "name": "X"})
        self.assertIn("no sheet named 'KhongCo'", str(caught.exception))

    def test_doi_ten_thanh_ten_da_co_thi_tu_choi(self):
        book = FakeBook(("A", "B"))
        with self.assertRaises(calc.values.ParamError) as caught:
            calc.rename_sheet(book_env(book), {"sheet": "A", "name": "B"})
        self.assertIn("already exists", str(caught.exception))
        self.assertEqual(book.names(), ["A", "B"], "tu choi thi khong duoc doi gi")
        # Doi ten thanh chinh no thi vo hai (model hay goi lai cho chac).
        self.assertEqual(calc.rename_sheet(book_env(book), {"sheet": "A", "name": "A"})["sheet"], "A")

    def test_thieu_tham_so_bat_buoc(self):
        book = FakeBook(("A",))
        with self.assertRaises(calc.values.ParamError):
            calc.rename_sheet(book_env(book), {"sheet": "A"})
        with self.assertRaises(calc.values.ParamError):
            calc.rename_sheet(book_env(book), {"name": "B"})


if __name__ == "__main__":
    unittest.main(verbosity=2)
