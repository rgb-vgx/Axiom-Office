"""Lenh cau truc Word/Excel (docs/plans/21-v0 P1) tren Office THAT: goi thang qua bridge, luu tep, cham bang XML.

    python tests/live/test_office_structure.py [word|excel]

Can Microsoft Office x64 da cai add-in. Khong chay trong CI (CI khong co Office). Tu mo va dong Word/Excel.
Word: writer.setStyle (doan co san), writer.insertToc (chen + cap nhat), writer.setHeaderFooter, writer.insertCaption
(cuoi bai va truoc bang co san), insertTable lap hang tieu de, va thu tu viet noi tiep appendText -> heading ->
insertTable (bang/tieu de khong roi vao vi tri con tro cu). Excel: et.createTable, et.freezePanes, et.setValidation
(danh sach o sheet khac, so, danh sach go thang), et.addChart lay vung o sheet khac.
"""
from __future__ import annotations

import os
import sys
import tempfile
import zipfile
import xml.etree.ElementTree as ET

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(os.path.dirname(HERE), "bench", "office"))
import test_live_commands as live  # noqa: E402
import check_docx  # noqa: E402
import check_xlsx  # noqa: E402

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

RESULTS = []


def check(ok: bool, name: str, detail: str = "") -> None:
    RESULTS.append(ok)
    print(("PASS " if ok else "FAIL ") + name)
    if not ok and detail:
        print("     " + detail[:400])


def body_items(path: str) -> list:
    """[(style, text)] theo thu tu trong than bai; bang = ("TABLE", chu cua bang)."""
    W = check_docx.W
    root = ET.fromstring(zipfile.ZipFile(path).read("word/document.xml"))
    items = []
    for element in root.find(W + "body"):
        if element.tag == W + "tbl":
            items.append(("TABLE", check_docx.para_text(element)))
        elif element.tag == W + "p":
            ppr = element.find(W + "pPr")
            style = check_docx.attr(ppr.find(W + "pStyle"), "val") if ppr is not None and ppr.find(W + "pStyle") is not None else ""
            text = check_docx.para_text(element).strip()
            if text:
                items.append((style, text))
    return items


def run_word(out: str) -> None:
    port, pid = live.launch("word", False)
    try:
        bridge = live.Bridge(port, out, "word")
        bridge.cmd("writer.newDocument", record=False)
        bridge.cmd("writer.appendText", {"text": "BÁO CÁO THỬ\nMở đầu\nĐoạn mở đầu.\nKết quả\nĐoạn kết quả."})
        bridge.cmd("writer.setStyle", {"style": "Title", "paragraphs": [1]})
        styled = bridge.cmd("writer.setStyle", {"style": "Heading 1", "paragraphs": [2, 4]}) or {}
        check(styled.get("paragraphs") == 2, "setStyle gan Heading 1 cho 2 doan co san", str(styled))
        bridge.cmd("writer.insertToc", {"before": 2})
        bridge.cmd("writer.setHeaderFooter", {"header": "Trường THPT Nguyễn Du", "pageNumber": True})
        bridge.cmd("writer.appendText", {"text": "Bảng dưới đây tóm tắt kết quả."})
        caption = bridge.cmd("writer.insertCaption", {"text": "Kết quả tuyển sinh"}) or {}
        check(caption.get("caption") == "Bảng 1. Kết quả tuyển sinh", "insertCaption danh so Bang 1", str(caption))
        bridge.cmd("writer.insertTable", {"values": [["Năm", "Chỉ tiêu"], ["2025", "550"], ["2026", "600"]]})
        bridge.cmd("writer.appendText", {"text": "Đoạn sau bảng."})
        bridge.cmd("writer.insertTable", {"values": [["A", "B"], ["1", "2"]]})
        tables = [item for item in (bridge.cmd("writer.outline") or {}).get("items", []) if item.get("kind") == "table"]
        bridge.cmd("writer.insertCaption", {"text": "Bảng có sẵn", "before": tables[-1]["n"]})
        bridge.cmd("writer.heading", {"text": "Phụ lục", "level": 1})
        bridge.cmd("writer.appendText", {"text": "Hết."})
        updated = bridge.cmd("writer.insertToc", {}) or {}
        check(updated.get("updated") == 1, "insertToc lan hai cap nhat muc luc co san", str(updated))
        path = os.path.join(out, "structure.docx")
        bridge.cmd("writer.saveAs", {"path": path}, record=False)
    finally:
        live.close_app(pid)

    report = check_docx.check_package(path)
    check(report["toc"], "co truong TOC")
    check(report["pageField"] and "Nguyễn Du" in report["headerFooterText"], "dau trang + so trang", report["headerFooterText"])
    check(report["headingLevels"]["1"] == 3, "3 Heading 1 (2 gan + 1 viet moi)", str(report["headingLevels"]))
    check(report["captions"] == 2 and report["tablesRepeatHeader"] == 2, "2 chu thich SEQ, 2 bang lap tieu de",
          "captions %s repeat %s" % (report["captions"], report["tablesRepeatHeader"]))
    items = [item for item in body_items(path) if not item[0].startswith("TOC")]
    order = [text for _, text in items]
    expected = ["BÁO CÁO THỬ", "Mở đầu", "Đoạn mở đầu.", "Kết quả", "Đoạn kết quả.", "Bảng dưới đây tóm tắt kết quả.",
                "Bảng 1. Kết quả tuyển sinh", "NămChỉ tiêu20255502026600", "Đoạn sau bảng.", "Bảng 2. Bảng có sẵn",
                "AB12", "Phụ lục", "Hết."]
    check(order == expected, "thu tu viet noi tiep dung (bang, chu thich, tieu de khong roi vao vi tri cu)", repr(order))
    check(("Heading1", "Phụ lục") in items, "heading sau appendText la doan rieng", repr(items[-3:]))


def run_excel(out: str) -> None:
    port, pid = live.launch("excel", False)
    try:
        bridge = live.Bridge(port, out, "excel")
        bridge.cmd("et.newWorkbook", record=False)
        bridge.cmd("et.renameSheet", {"sheet": "Sheet1", "name": "Data"})
        bridge.cmd("et.writeRange", {"sheet": "Data", "range": "A1", "values": [
            ["Tháng", "Kế hoạch", "Thực hiện", "Trạng thái"], ["T1", 100, 92, ""], ["T2", 110, 115, ""], ["T3", 120, 118, ""]]})
        bridge.cmd("et.addSheet", {"name": "Config"})
        bridge.cmd("et.writeRange", {"sheet": "Config", "range": "A1", "values": [["Trạng thái"], ["Đạt"], ["Chưa đạt"]]})
        bridge.cmd("et.addSheet", {"name": "Dashboard"})
        table = bridge.cmd("et.createTable", {"sheet": "Data", "range": "A1:D4", "name": "tblData"}) or {}
        check(table.get("table") == "tblData", "createTable", str(table))
        # Hang tieu de co o trong -> tu choi (Excel se dat "Column1" va o do khong xoa trang duoc).
        blank = bridge.cmd("et.createTable", {"sheet": "Config", "range": "A1:B3"}, expect_ok=False, record=False)
        check("B1 is empty" in str(blank), "createTable tu choi tieu de trong", str(blank))
        # Sheet cau hinh: hang 2 trong, o 0.9 da de 0% -> khong bao no-number-format (lay dinh dang o so le that).
        bridge.cmd("et.writeRange", {"sheet": "Config", "range": "D1", "values": [["Ngưỡng"], [None], [0.9]]}, record=False)
        bridge.cmd("et.formatRange", {"sheet": "Config", "range": "D3", "numFmt": "0%"}, record=False)
        qa = bridge.cmd("et.checkRange", {"sheet": "Config", "range": "D1:D3"}, record=False) or {}
        check("no-number-format" not in str(qa), "checkRange doc dinh dang cua o so le, khong phai hang 2", str(qa.get("issues")))
        bridge.cmd("et.formatRange", {"sheet": "Config", "range": "D3", "numFmt": "General"}, record=False)
        qa = bridge.cmd("et.checkRange", {"sheet": "Config", "range": "D1:D3"}, record=False) or {}
        check("no-number-format" in str(qa), "checkRange van bat so le de General", str(qa.get("issues")))
        bridge.cmd("et.freezePanes", {"sheet": "Data", "rows": 1})
        bridge.cmd("et.setValidation", {"sheet": "Data", "range": "D2:D4", "source": "Config!$A$2:$A$3"})
        bridge.cmd("et.setValidation", {"sheet": "Data", "range": "B2:B4", "type": "whole", "min": 0, "max": 1000})
        bridge.cmd("et.setValidation", {"sheet": "Config", "range": "B2", "list": ["Có", "Không"]})
        bridge.cmd("et.addChart", {"sheet": "Dashboard", "range": "Data!A1:C4", "type": "column"})
        path = os.path.join(out, "structure.xlsx")
        bridge.cmd("et.saveAs", {"path": path}, record=False)
    finally:
        live.close_app(pid)

    sheets = check_xlsx.check_package(path, ["Data", "Config", "Dashboard"])["sheets"]
    data = sheets["Data"]
    check(data["tables"] == 1 and data["frozen"], "Data: 1 Table + co dinh hang tieu de", str(data))
    kinds = sorted((v["type"], v["sqref"]) for v in data["validations"])
    check(kinds == [("list", "D2:D4"), ("whole", "B2:B4")], "Data: validation danh sach (sheet khac) + so nguyen", str(kinds))
    check(any(v["type"] == "list" for v in sheets["Config"]["validations"]), "Config: danh sach go thang")
    refs = sheets["Dashboard"]["chartRefs"]
    check(sheets["Dashboard"]["charts"] == 1 and all(ref.startswith("Data!") for ref in refs), "bieu do Dashboard doc thang Data!", str(refs))


def main() -> int:
    which = sys.argv[1:] or ["word", "excel"]
    out = tempfile.mkdtemp(prefix="axiom-structure-")
    if "word" in which:
        run_word(out)
    if "excel" in which:
        run_excel(out)
    print("\n%d passed, %d failed" % (sum(RESULTS), len(RESULTS) - sum(RESULTS)))
    return 0 if all(RESULTS) else 1


if __name__ == "__main__":
    sys.exit(main())
