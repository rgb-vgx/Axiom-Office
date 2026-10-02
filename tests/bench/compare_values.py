"""Đối chiếu GIÁ TRỊ (không phải cấu trúc) giữa file của Axiom và file của Claude Code.

Bốn báo cáo trước chỉ so cấu trúc: tên sheet, số ô công thức, số chart. Đề chấm bằng "kết quả phải
đúng về dữ liệu, công thức, logic nghiệp vụ", nên bài này đọc **giá trị đã lưu** trong file.

So từng ô giữa hai file là VÔ NGHĨA khi mô hình có yếu tố ngẫu nhiên (Monte Carlo khác seed thì ra số
khác, và hai bên còn thiết kế mô hình khác nhau). Cái so được — và là cái quyết định file có dùng được
không — là **tính tự nhất quán của từng bên**:

  - có ô nào là LỖI không (#REF!, #DIV/0!, #VALUE!, #N/A, Err:xxx)
  - ô nào mang công thức, ô nào là số gõ cứng (ghi cứng kết quả tính được là lỗi của đề)
  - sheet kiểm tra (Checks/Audit/Validation) tự báo bao nhiêu PASS/FAIL
  - ô công thức có trỏ ra ngoài vùng dữ liệu thật không (bảng con trỏ vào ô trống)

    python tests/bench/compare_values.py <file A> <file B>

.ods được đổi sang .xlsx bằng LibreOffice trước khi đọc (cùng một đường đọc cho cả hai bên).
"""
from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

ERROR_TEXT = re.compile(r"^(#[A-Z0-9/]+!?|Err:\d+)")
CHECK_SHEETS = ("check", "audit", "valid", "kiểm", "kiem", "qc")
CELL_LIMIT = 400000        # tran quet moi sheet: file 10.000 dong ai cung doc duoc, file loi thi khong treo


def to_xlsx(path: str, workdir: str) -> str:
    """Tra ve duong dan .xlsx. File .xlsx thi tra ve chinh no, .ods thi nho LibreOffice doi."""
    if path.lower().endswith(".xlsx"):
        return path
    soffice = shutil.which("soffice") or r"C:\Program Files\LibreOffice\program\soffice.exe"
    if not os.path.exists(soffice):
        raise SystemExit("khong thay LibreOffice de doi .ods sang .xlsx")
    subprocess.run([soffice, "-env:UserInstallation=file:///" + workdir.replace(os.sep, "/") + "/lo",
                    "--headless", "--norestore", "--convert-to", "xlsx", "--outdir", workdir, path],
                   capture_output=True, timeout=900)
    out = os.path.join(workdir, os.path.splitext(os.path.basename(path))[0] + ".xlsx")
    if not os.path.exists(out):
        raise SystemExit("LibreOffice khong doi duoc %s sang .xlsx" % path)
    return out


def scan(path: str) -> dict:
    import openpyxl

    formulas = openpyxl.load_workbook(path, data_only=False)
    values = openpyxl.load_workbook(path, data_only=True)
    report = {"path": path, "sheets": [], "errors": [], "hardcoded": [], "checks": {}}
    for name in formulas.sheetnames:
        sheet_f = formulas[name]
        sheet_v = values[name]
        dimension = sheet_f.calculate_dimension()
        formula_cells = 0
        number_cells = 0
        text_cells = 0
        empty_formula = 0
        seen = 0
        for row_f, row_v in zip(sheet_f.iter_rows(), sheet_v.iter_rows()):
            for cell_f, cell_v in zip(row_f, row_v):
                seen += 1
                if seen > CELL_LIMIT:
                    break
                value = cell_v.value
                # Phan loai theo o CONG THUC truoc, roi moi den gia tri da luu: file do thu vien sinh ra
                # (openpyxl/xlsxwriter) co cong thuc nhung KHONG co ket qua luu san, nen neu bo qua o
                # None truoc thi ca mot bang cong thuc bi dem thanh 0 (da mac dung loi nay).
                if cell_f.data_type == "f":
                    formula_cells += 1
                    if value is None:
                        empty_formula += 1
                    elif isinstance(value, str) and ERROR_TEXT.match(value.strip()):
                        report["errors"].append({"sheet": name, "cell": cell_f.coordinate, "value": value[:40]})
                    continue
                if value is None:
                    continue
                if isinstance(value, str) and ERROR_TEXT.match(value.strip()):
                    report["errors"].append({"sheet": name, "cell": cell_f.coordinate, "value": value[:40]})
                if isinstance(value, (int, float)):
                    number_cells += 1
                elif isinstance(value, str):
                    text_cells += 1
            if seen > CELL_LIMIT:
                break

        # Sheet kiem tra: dem PASS/FAIL theo GIA TRI da luu, khong phai theo y dinh.
        if any(key in name.lower() for key in CHECK_SHEETS):
            passed = failed = 0
            for row in sheet_v.iter_rows():
                for cell in row:
                    text = str(cell.value).strip().upper() if cell.value is not None else ""
                    if text == "PASS":
                        passed += 1
                    elif text == "FAIL":
                        failed += 1
            report["checks"][name] = {"pass": passed, "fail": failed}

        report["sheets"].append({"name": name, "dimension": dimension, "formulas": formula_cells,
                                 "numbers": number_cells, "texts": text_cells, "noCache": empty_formula})
    formulas.close()
    values.close()
    return report


def show(report: dict, label: str) -> None:
    print("=== %s: %s" % (label, os.path.basename(report["path"])))
    print("%-22s %14s %10s %10s %8s %10s" % ("sheet", "vung dung", "cong thuc", "so", "chu", "chua tinh"))
    for item in report["sheets"]:
        print("%-22s %14s %10d %10d %8d %10d"
              % (item["name"], item["dimension"], item["formulas"], item["numbers"], item["texts"],
                 item["noCache"]))
    total_formulas = sum(item["formulas"] for item in report["sheets"])
    total_no_cache = sum(item["noCache"] for item in report["sheets"])
    print("tong cong thuc: %d | o loi: %d | cong thuc CHUA co ket qua luu: %d"
          % (total_formulas, len(report["errors"]), total_no_cache))
    for problem in report["errors"][:10]:
        print("   LOI %s!%s = %s" % (problem["sheet"], problem["cell"], problem["value"]))
    for name, counts in report["checks"].items():
        print("   sheet kiem tra '%s': PASS=%d FAIL=%d" % (name, counts["pass"], counts["fail"]))
    print()


def main() -> int:
    if len(sys.argv) < 3:
        print(__doc__)
        return 2
    workdir = tempfile.mkdtemp(prefix="axiom-compare-")
    first = scan(to_xlsx(sys.argv[1], workdir))
    second = scan(to_xlsx(sys.argv[2], workdir))
    show(first, "A")
    show(second, "B")
    print("GHI CHU: so tung o giua hai ben chi co nghia khi mo hinh tat dinh; voi mo hinh ngau nhien thi")
    print("cai so duoc la o loi / cong thuc / sheet kiem tra cua TUNG ben (in o tren).")
    shutil.rmtree(workdir, ignore_errors=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
