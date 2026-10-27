"""Tu soat phan trang va viet nhieu khoi (docs/plans/21-v0 P2) tren Office THAT: goi thang qua bridge, luu tep, cham XML.

    python tests/live/test_office_layout.py [word|excel]

Can Microsoft Office x64 da cai add-in. Khong chay trong CI. Tu mo va dong Word/Excel.
Word: writer.writeContent (tieu de, doan can deu, danh sach that, bang kem chu thich, ngat trang - dung thu tu, danh sach
khong lan sang doan sau), writer.checkLayout (trang trang do hai lan ngat trang, doan trong lien tiep), writer.deleteParagraphs
(ca doan cuoi bai - giu style doan truoc), insertPageBreak {before}, bang nho giu tren mot trang.
Excel: et.checkRange khong truyen range/sheet thay o loi o sheet khac; truyen sheet thi chi soat sheet do.
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
from test_writer_format import style_alignments  # noqa: E402

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

RESULTS = []
W = check_docx.W


def check(ok: bool, name: str, detail: str = "") -> None:
    RESULTS.append(ok)
    print(("PASS " if ok else "FAIL ") + name)
    if not ok and detail:
        print("     " + detail[:500])


def body_paragraphs(path: str) -> list:
    """[(style, text, props)] cua cac doan ngoai bang, theo thu tu; bang = ("TABLE", chu, props cua o dau)."""
    package = zipfile.ZipFile(path)
    root = ET.fromstring(package.read("word/document.xml"))
    aligns = style_alignments(ET.fromstring(package.read("word/styles.xml")))
    items = []
    for element in root.find(W + "body"):
        if element.tag == W + "tbl":
            keep = element.findall(".//" + W + "keepNext")
            items.append(("TABLE", check_docx.para_text(element), {"keepNext": len(keep)}))
        elif element.tag == W + "p":
            ppr = element.find(W + "pPr")
            props = {}
            style = ""
            if ppr is not None:
                if ppr.find(W + "pStyle") is not None:
                    style = check_docx.attr(ppr.find(W + "pStyle"), "val")
                props["list"] = ppr.find(W + "numPr") is not None
                jc = ppr.find(W + "jc")
                props["jc"] = check_docx.attr(jc, "val") if jc is not None else aligns.get(style, aligns[""])
                props["pageBreakBefore"] = ppr.find(W + "pageBreakBefore") is not None
            items.append((style, check_docx.para_text(element).strip(), props))
    return items


def issues_of(layout: dict, kind: str) -> list:
    return [i for i in (layout or {}).get("issues", []) if i.get("type") == kind]


def run_word(out: str) -> None:
    port, pid = live.launch("word", False)
    try:
        bridge = live.Bridge(port, out, "word")
        bridge.cmd("writer.newDocument", record=False)
        written = bridge.cmd("writer.writeContent", {"blocks": [
            {"heading": "1. Mở đầu", "level": 1},
            {"text": "Đoạn thân bài thứ nhất.\nĐoạn thân bài thứ hai."},
            {"bullets": ["Ý một", "Ý hai", "Ý ba"]},
            {"text": "Đoạn sau danh sách."},
            {"heading": "1.1. Số liệu", "level": 2},
            {"numbers": ["Bước một", "Bước hai"]},
            {"table": [["Năm", "Chỉ tiêu"], ["2025", "550"], ["2026", "600"]], "caption": "Chỉ tiêu tuyển sinh"},
            {"text": "Nhận xét dưới bảng."},
            {"pageBreak": True},
            {"heading": "2. Kết luận", "level": 1},
            {"text": "Hết."},
        ]}, record=False) or {}
        check(written.get("blocks") == 11 and written.get("table") == 1 and written.get("heading") == 3,
              "writeContent viet 11 khoi trong mot lenh", str(written))
        layout = bridge.cmd("writer.checkLayout", record=False) or {}
        check(layout.get("pages") == 2 and layout.get("issueCount") == 0, "checkLayout: 2 trang, khong loi", str(layout))
        levels = [(h.get("level"), h.get("page")) for h in layout.get("headings", [])]
        check(levels == [(1, 1), (2, 1), (1, 2)], "checkLayout: trang cua tung tieu de", str(levels))
        longer = bridge.cmd("writer.checkLayout", {"maxPages": 1}, record=False) or {}
        check(len(issues_of(longer, "too-long")) == 1, "checkLayout {maxPages:1} bao too-long", str(longer.get("issues")))
        refused = bridge.cmd("writer.writeContent", {"blocks": [{"text": "x"}, {"table": [["=A1*2"]]}]}, expect_ok=False, record=False)
        check("formulas" in str(refused), "writeContent tu choi o cong thuc TRUOC khi ghi", str(refused))
        added = bridge.cmd("writer.addTableRows", {"table": 1, "rows": [["2027", "650"]]}, record=False) or {}
        bridge.cmd("writer.addTableRows", {"table": 1, "before": 2, "rows": [["2024", "500"]]}, record=False)
        check(added.get("rows") == 4, "addTableRows noi cuoi bang", str(added))
        path = os.path.join(out, "content.docx")
        bridge.cmd("writer.saveAs", {"path": path}, record=False)
        bridge.cmd("writer.closeAll", record=False)

        # Trang trang: hai lan ngat trang lien nhau; doan trong lien tiep o cuoi bai sau mot tieu de.
        bridge.cmd("writer.newDocument", record=False)
        bridge.cmd("writer.writeContent", {"blocks": [
            {"text": "Trang một."}, {"pageBreak": True}, {"pageBreak": True}, {"text": "Trang ba."},
            {"heading": "Phụ lục", "level": 2}]}, record=False)
        bridge.cmd("writer.insertStyledText", {"text": "\n\n", "alignment": "left"}, record=False)
        before = bridge.cmd("writer.checkLayout", record=False) or {}
        blank = issues_of(before, "blank-page")
        empties = issues_of(before, "empty-paragraphs")
        check(before.get("pages") == 3 and len(blank) == 1 and blank[0].get("page") == 2, "checkLayout thay trang trang 2", str(before))
        check(len(empties) == 1, "checkLayout thay doan trong lien tiep cuoi bai", str(before.get("issues")))
        targets = sorted(set((blank[0].get("paragraphs") if blank else []) + (empties[0].get("paragraphs") if empties else [])))
        deleted = bridge.cmd("writer.deleteParagraphs", {"paragraphs": targets}, record=False) or {}
        after = bridge.cmd("writer.checkLayout", record=False) or {}
        check(after.get("pages") == 2 and not issues_of(after, "blank-page") and not issues_of(after, "empty-paragraphs"),
              "sau deleteParagraphs: 2 trang, het trang trang va doan trong", "deleted %s, after %s" % (deleted, after))
        outline = bridge.cmd("writer.outline", record=False) or {}
        heading = [i for i in outline.get("items", []) if i.get("t") == "Phụ lục"]
        check(heading and "2" in str(heading[0].get("style", "")), "doan cuoi (tieu de) giu style sau khi xoa doan trong cuoi bai", str(outline.get("items")))
        if heading:
            bridge.cmd("writer.insertPageBreak", {"before": heading[0]["n"]}, record=False)
        moved = bridge.cmd("writer.checkLayout", record=False) or {}
        check(moved.get("pages") == 3, "insertPageBreak {before} dua tieu de sang trang moi", str(moved.get("headings")))
        bridge.cmd("writer.insertToc", {"before": 1}, record=False)
        toc = [i for i in (bridge.cmd("writer.outline", record=False) or {}).get("items", []) if str(i.get("style", "")).upper().startswith("TOC")]
        if toc:
            error = bridge.cmd("writer.deleteParagraphs", {"paragraphs": [toc[0]["n"]]}, expect_ok=False, record=False)
            check("table of contents" in str(error), "deleteParagraphs tu choi doan cua muc luc", str(error))
        else:
            check(False, "muc luc co doan de thu", "")
        path2 = os.path.join(out, "blank.docx")
        bridge.cmd("writer.saveAs", {"path": path2}, record=False)
    finally:
        live.close_app(pid)

    items = body_paragraphs(path)
    texts = [text for _, text, _ in items if text]
    expected = ["1. Mở đầu", "Đoạn thân bài thứ nhất.", "Đoạn thân bài thứ hai.", "Ý một", "Ý hai", "Ý ba", "Đoạn sau danh sách.",
                "1.1. Số liệu", "Bước một", "Bước hai", "Bảng 1. Chỉ tiêu tuyển sinh", "NămChỉ tiêu2024500202555020266002027650",
                "Nhận xét dưới bảng.", "2. Kết luận", "Hết."]
    check(texts == expected, "writeContent dung thu tu", repr(texts))
    by_text = {text: (style, props) for style, text, props in items}
    lists = [t for t in ["Ý một", "Ý hai", "Ý ba", "Bước một", "Bước hai"] if by_text.get(t, ("", {}))[1].get("list")]
    check(len(lists) == 5, "5 muc la danh sach that (numPr)", str(lists))
    check(not by_text.get("Đoạn sau danh sách.", ("", {}))[1].get("list"), "doan sau danh sach khong bi keo vao danh sach",
          str(by_text.get("Đoạn sau danh sách.")))
    check(by_text.get("Đoạn thân bài thứ nhất.", ("", {}))[1].get("jc") == "both", "doan than bai can deu",
          str(by_text.get("Đoạn thân bài thứ nhất.")))
    check(by_text.get("1.1. Số liệu", ("",))[0] == "Heading2", "tieu de cap 2", str(by_text.get("1.1. Số liệu")))
    table = [props for style, _, props in items if style == "TABLE"]
    check(table and table[0]["keepNext"] >= 2, "bang nho giu tren mot trang (keepNext)", str(table))
    report = check_docx.check_package(path)
    check(report["captions"] == 1, "chu thich SEQ cho bang", str(report["captions"]))
    blank_items = body_paragraphs(path2)
    check(any(props.get("pageBreakBefore") for style, text, props in blank_items if text == "Phụ lục"),
          "insertPageBreak {before} = ngat trang truoc doan", repr(blank_items[-3:]))


def run_excel(out: str) -> None:
    port, pid = live.launch("excel", False)
    try:
        bridge = live.Bridge(port, out, "excel")
        bridge.cmd("et.newWorkbook", record=False)
        bridge.cmd("et.writeRange", {"sheet": "Sheet1", "range": "A1", "values": [["Mục", "Số"], ["A", 1], ["B", 2]]}, record=False)
        bridge.cmd("et.addSheet", {"name": "Tinh"}, record=False)
        bridge.cmd("et.writeRange", {"sheet": "Tinh", "range": "A1", "values": [["Tỷ lệ"], ["=1/0"], ["=Sheet1!B2*2"], ["=XYZ()"]]}, record=False)
        bridge.cmd("et.activateSheet", {"sheet": "Sheet1"}, record=False)
        whole = bridge.cmd("et.checkRange", record=False) or {}
        errors = [i for i in whole.get("issues", []) if i.get("type") == "error-cells"]
        cells = errors[0].get("cells", []) if errors else []
        check(whole.get("sheet") == "Sheet1" and len(cells) == 2 and all(c.startswith("Tinh!") for c in cells)
              and any("#DIV/0!" in c for c in cells) and any("#NAME?" in c for c in cells),
              "checkRange khong range/sheet: thay o loi o sheet khac", str(whole.get("issues")))
        one = bridge.cmd("et.checkRange", {"sheet": "Sheet1"}, record=False) or {}
        check(not [i for i in one.get("issues", []) if i.get("type") == "error-cells"], "checkRange {sheet}: chi soat sheet do",
              str(one.get("issues")))
    finally:
        live.close_app(pid)


def main() -> int:
    which = sys.argv[1:] or ["word", "excel"]
    out = tempfile.mkdtemp(prefix="axiom-layout-")
    if "word" in which:
        run_word(out)
    if "excel" in which:
        run_excel(out)
    print("\n%d passed, %d failed" % (sum(RESULTS), len(RESULTS) - sum(RESULTS)))
    return 0 if all(RESULTS) else 1


if __name__ == "__main__":
    sys.exit(main())
