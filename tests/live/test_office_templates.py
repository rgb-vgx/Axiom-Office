"""Mau van ban, dashboard, dinh dang co dieu kien nang cao (docs/plans/21-v0 P3) tren Office THAT.

    python tests/live/test_office_templates.py [word|excel]

Can Microsoft Office x64 da cai add-in va Go (chay test Go -tags live cua core-go). Khong chay trong CI.
Word: writer.insertLayoutTable (bang khong vien, dinh dang tung dong); tool doc_from_brief dung to trinh dung the thuc
NĐ 30 (phan dau, ten loai, trich yeu, Kinh gui, noi dung, chu thich bang, noi nhan, chu ky).
Excel: et.setConditionalFormat kieu truc quan (colorScale, dataBar, iconSet, top, duplicate), et.formatRange
columnWidth; tool dashboard_from_data (o KPI la cong thuc tro sheet Data, 2 bieu do doc thang Data).
"""
from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import zipfile
import xml.etree.ElementTree as ET

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(os.path.dirname(HERE), "bench", "office"))
import test_live_commands as live  # noqa: E402
import check_docx  # noqa: E402
import check_xlsx  # noqa: E402

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

RESULTS = []
W = check_docx.W
S = "{http://schemas.openxmlformats.org/spreadsheetml/2006/main}"


def check(ok: bool, name: str, detail: str = "") -> None:
    RESULTS.append(ok)
    print(("PASS " if ok else "FAIL ") + name)
    if not ok and detail:
        print("     " + detail[:600])


def go_live(test: str, app: str, port: int, out: str) -> str:
    env = dict(os.environ, AXIOM_LIVE_PORT=str(port), AXIOM_LIVE_TOKEN=live.token(), AXIOM_LIVE_APP=app, AXIOM_LIVE_OUT=out)
    done = subprocess.run(["go", "test", "-tags", "live", "-count=1", "-run", test, "-v", "./internal/tools/"],
                          cwd=os.path.join(ROOT, "core-go"), env=env, capture_output=True, text=True, encoding="utf-8",
                          errors="replace", timeout=600)
    output = done.stdout + done.stderr
    check(done.returncode == 0 and os.path.exists(out), "go test " + test, output[-1500:])
    return output


def runs_of(paragraph) -> list:
    """[(text, bold, italic, size)] tung run co chu."""
    found = []
    for run in paragraph.iter(W + "r"):
        text = "".join(t.text or "" for t in run.iter(W + "t"))
        if not text:
            continue
        rpr = run.find(W + "rPr")
        bold = rpr is not None and rpr.find(W + "b") is not None and check_docx.attr(rpr.find(W + "b"), "val") not in ("0", "false")
        italic = rpr is not None and rpr.find(W + "i") is not None and check_docx.attr(rpr.find(W + "i"), "val") not in ("0", "false")
        size = rpr.find(W + "sz") if rpr is not None else None
        found.append((text, bold, italic, int(check_docx.attr(size, "val")) // 2 if size is not None else 0))
    return found


def run_word(out: str) -> None:
    port, pid = live.launch("word", False)
    path = os.path.join(out, "to-trinh.docx")
    try:
        bridge = live.Bridge(port, out, "word")
        bridge.cmd("writer.newDocument", record=False)
        table = bridge.cmd("writer.insertLayoutTable", {"rows": [[
            {"lines": [{"t": "trường a", "upper": True, "bold": True}, "Số: 1/TB"]},
            {"lines": [{"t": "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM", "bold": True, "size": 12}, {"t": "Hà Nội, ngày 01", "italic": True}]}]],
            "widths": [40, 60]}, record=False) or {}
        check(table.get("layout") is True and table.get("cols") == 2, "insertLayoutTable", str(table))
        checked = bridge.cmd("writer.checkTables", record=False) or {}
        check(any(t.get("layout") for t in checked.get("tables", [])), "bang dan trang duoc danh dau layout (khong bao o trong)",
              str(checked))
        bad = bridge.cmd("writer.insertLayoutTable", {"rows": [["a", "b"]], "widths": [100]}, expect_ok=False, record=False)
        check("widths" in str(bad), "widths sai so cot bi tu choi", str(bad))
        bridge.cmd("writer.closeAll", record=False)
        bridge.cmd("writer.newDocument", record=False)
        output = go_live("TestLiveP3DocFromBrief", "wps", port, path)
        check('"placeholders"' in output and "signer.name" in output, "doc_from_brief bao cho con thieu (ten nguoi ky)", output[-800:])
    finally:
        live.close_app(pid)
    if not os.path.exists(path):
        return
    root = ET.fromstring(zipfile.ZipFile(path).read("word/document.xml"))
    body = root.find(W + "body")
    tables = body.findall(W + "tbl")
    check(len(tables) == 3, "3 bang: phan dau, du toan, phan ky", str(len(tables)))
    header = [r for p in tables[0].iter(W + "p") for r in runs_of(p)]
    by_text = {text: (bold, italic, size) for text, bold, italic, size in header}
    check(by_text.get("SỞ GIÁO DỤC VÀ ĐÀO TẠO HÀ NỘI", (True,))[0] is False and by_text.get("TRƯỜNG THPT NGUYỄN DU", (False,))[0],
          "co quan chu quan thuong, co quan ban hanh IN HOA dam", str(header))
    check(by_text.get("CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM", (False,))[0] and by_text.get("Độc lập - Tự do - Hạnh phúc", (False,))[0],
          "quoc hieu, tieu ngu dam", str(header))
    check(by_text.get("Hà Nội, ngày 05 tháng 02 năm 2026", (False, False))[1] and "Số: 12/TTr-THPT" in by_text,
          "ngay thang nghieng, dung NĐ30 (05, 02), so ky hieu", str(header))
    borders = tables[0].find(W + "tblPr/" + W + "tblBorders")
    check(borders is None or all(check_docx.attr(b, "val") in ("nil", "none") for b in borders), "phan dau khong vien")
    paragraphs = [check_docx.para_text(p).strip() for p in body.findall(W + "p")]
    for part in ["TỜ TRÌNH", "Về việc mua 5 máy tính xách tay cho phòng Kế toán", "Kính gửi: Giám đốc Sở.", "1. Lý do",
                 "Máy cũ đã dùng 7 năm, thường xuyên hỏng.", "Bảng 1. Dự toán kinh phí", "Kính đề nghị Giám đốc Sở xem xét, phê duyệt."]:
        check(part in paragraphs, "co doan: " + part, repr(paragraphs[:20]))
    order = [paragraphs.index(p) for p in ["TỜ TRÌNH", "Kính gửi: Giám đốc Sở.", "1. Lý do", "Kính đề nghị Giám đốc Sở xem xét, phê duyệt."] if p in paragraphs]
    check(order == sorted(order) and len(order) == 4, "thu tu: ten loai -> kinh gui -> noi dung", str(order))
    sign = [r for p in tables[2].iter(W + "p") for r in runs_of(p)]
    texts = [text for text, *_ in sign]
    check("Nơi nhận:" in texts and "- Như trên;" in texts and "- Lưu: VT, KT." in texts and "HIỆU TRƯỞNG" in texts and "…" in texts,
          "phan ky: noi nhan + chuc vu + cho ten con trong", str(texts))
    report = check_docx.check_package(path)
    check(report["captions"] == 1 and report["headingLevels"]["1"] == 1, "chu thich SEQ + Heading 1", str(report["headingLevels"]))


def run_excel(out: str) -> None:
    port, pid = live.launch("excel", False)
    path = os.path.join(out, "dashboard.xlsx")
    try:
        bridge = live.Bridge(port, out, "excel")
        bridge.cmd("et.newWorkbook", record=False)
        bridge.cmd("et.renameSheet", {"sheet": "Sheet1", "name": "Data"}, record=False)
        bridge.cmd("et.writeRange", {"sheet": "Data", "range": "A1", "values": [
            ["Tháng", "Doanh thu", "Chi phí", "Tỷ suất"], ["T1", 120000000, 80000000, "=1-C2/B2"], ["T2", 150000000, 90000000, "=1-C3/B3"],
            ["T3", 130000000, 85000000, "=1-C4/B4"], ["T4", 170000000, 95000000, "=1-C5/B5"], ["T5", 160000000, 99000000, "=1-C6/B6"],
            ["T6", 180000000, 120000000, "=1-C7/B7"]]}, record=False)
        rules = [
            ("B2:B7", [{"type": "colorScale"}, {"type": "top", "rank": 2, "bold": True, "fillColor": "#C6EFCE"}]),
            ("C2:C7", [{"type": "dataBar", "color": "#C00000"}]),
            ("D2:D7", [{"type": "iconSet", "icons": "3traffic"}]),
            ("A2:A7", [{"type": "duplicate", "fillColor": "#FFC7CE"}]),
        ]
        for address, rule in rules:
            bridge.cmd("et.setConditionalFormat", {"sheet": "Data", "range": address, "rules": rule}, record=False)
        bad = bridge.cmd("et.setConditionalFormat", {"sheet": "Data", "range": "B2:B7", "rules": [{"type": "top", "rank": 3}]},
                         expect_ok=False, record=False)
        check("needs a style" in str(bad), "top khong co mau bi tu choi", str(bad))
        # Rule tren ca hang chong len B/C: khong duoc xoa thang mau / thanh du lieu cua B, C (chi thay rule dung vung).
        bridge.cmd("et.setConditionalFormat", {"sheet": "Data", "range": "A2:D7", "rules": [
            {"operator": "formula", "formula1": "RANK($B2,$B$2:$B$7)<=2", "bold": True}]}, record=False)
        bridge.cmd("et.setConditionalFormat", {"sheet": "Data", "range": "C2:C7", "rules": [
            {"type": "dataBar", "color": "#C00000"}]}, record=False)
        scale = bridge.cmd("et.setConditionalFormat", {"sheet": "Data", "range": "E2:E3", "rules": [
            {"type": "colorScale", "minColor": "#FF0000", "maxColor": "#00B050"}]}, record=False) or {}
        check(scale.get("rules") == [{"type": "colorScale"}], "colorScale nhan minColor/maxColor", str(scale))
        listed = bridge.cmd("et.listConditionalFormats", {"sheet": "Data"}, record=False) or {}
        rules = [(f.get("range"), r.get("operator")) for f in listed.get("formats", []) for r in f.get("rules", [])]
        kinds = sorted({kind for _, kind in rules})
        check(kinds == ["colorScale", "dataBar", "formula", "iconSet", "top10", "uniqueValues"], "listConditionalFormats doc lai dung kieu", str(kinds))
        check(sum(1 for r, k in rules if k == "dataBar") == 1 and ("B2:B7", "colorScale") in rules,
              "goi lai dung vung thi thay rule cu, vung chong len giu nguyen", str(rules))
        bridge.cmd("et.addSheet", {"name": "Tam"}, record=False)
        chart = bridge.cmd("et.addChart", {"sheet": "Tam", "range": "Data!A1:A7,Data!C1:C7", "type": "line"}, record=False) or {}
        check(chart.get("typeApplied") is True, "addChart vung roi moi phan co ten sheet", str(chart))
        # Nhieu vung, hai sheet, MOT loi goi; muc sai (mau / sheet) thi khong vung nao bi doi.
        many = bridge.cmd("et.formatRanges", {"sheet": "Data", "formats": [
            {"range": "A1:D1", "bold": True, "fillColor": "#DDEBF7", "horizontal": "center"},
            {"range": "B2:C7", "numFmt": "#,##0", "fillColor": "#FFF2CC"},
            {"range": "D2:D7", "numFmt": "0%", "fillColor": "#E2EFDA"},
            {"sheet": "Tam", "range": "A1", "bold": True, "columnWidth": 20}]}, record=False) or {}
        check(many.get("ranges") == 4 and many.get("sheets") == ["Data", "Tam"], "formatRanges 4 vung 2 sheet mot lan", str(many))
        bad = bridge.cmd("et.formatRanges", {"sheet": "Data", "formats": [
            {"range": "D2:D7", "fillColor": "#FF0000"}, {"range": "B2", "fillColor": "do"}]}, expect_ok=False, record=False)
        check("not a #RRGGBB" in str(bad), "formatRanges tu choi mau sai", str(bad))
        bad = bridge.cmd("et.formatRanges", {"formats": [{"sheet": "Khong", "range": "A1", "bold": True}]},
                         expect_ok=False, record=False)
        check("no sheet named" in str(bad) and "Data" in str(bad), "formatRanges sheet sai ke ten sheet co that", str(bad))
        bad = bridge.cmd("et.formatRanges", {"formats": [{"range": "A1"}]}, expect_ok=False, record=False)
        check("nothing to apply" in str(bad), "formatRanges muc khong co thuoc tinh", str(bad))
        deleted = bridge.cmd("et.deleteSheet", {"sheet": "Tam"}, record=False) or {}
        check(deleted.get("sheets") == ["Data"], "deleteSheet", str(deleted))
        last = bridge.cmd("et.deleteSheet", {"sheet": "Data"}, expect_ok=False, record=False)
        check("only sheet" in str(last), "khong xoa duoc sheet cuoi", str(last))
        output = go_live("TestLiveP3Dashboard", "et", port, path)
        check("=SUM(Data!$B$2:$B$7)" in output, "KPI la cong thuc tro Data", output[-800:])
    finally:
        live.close_app(pid)
    if not os.path.exists(path):
        return
    package = zipfile.ZipFile(path)
    report = check_xlsx.check_package(path, ["Data", "Dashboard"])
    data = report["sheets"]["Data"]
    check(data["inputFills"] == ["FFFFF2CC"] and data["formulaFills"] == ["FFE2EFDA"],
          "formatRanges: o nhap / o cong thuc khac mau, lan goi sai khong doi gi", str((data["inputFills"], data["formulaFills"])))
    check(data["formatKinds"].get("B") == ["grouped"] and data["formatKinds"].get("D") == ["percent"],
          "formatRanges: dinh dang so tung vung", str(data["formatKinds"]))
    dash = report["sheets"]["Dashboard"]
    check(dash["charts"] == 2 and all(ref.startswith("Data!") for ref in dash["chartRefs"]), "2 bieu do doc thang Data", str(dash["chartRefs"]))
    names = [n for n in package.namelist() if n.startswith("xl/worksheets/sheet")]
    cf_types = set()
    for name in names:
        root = ET.fromstring(package.read(name))
        cf_types |= {rule.get("type") for rule in root.iter(S + "cfRule")}
    check({"colorScale", "dataBar", "iconSet", "top10", "duplicateValues"} <= cf_types, "xlsx co cfRule truc quan", str(cf_types))
    check("#,##0 \\\"đ\\\"" in output or "đ" in package.read("xl/styles.xml").decode("utf-8"), "dinh dang tien vnd cho KPI")


def main() -> int:
    which = sys.argv[1:] or ["word", "excel"]
    out = tempfile.mkdtemp(prefix="axiom-templates-")
    if "word" in which:
        run_word(out)
    if "excel" in which:
        run_excel(out)
    print("\n%d passed, %d failed" % (sum(RESULTS), len(RESULTS) - sum(RESULTS)))
    return 0 if all(RESULTS) else 1


if __name__ == "__main__":
    sys.exit(main())
