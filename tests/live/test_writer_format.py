"""Hoi quy dinh dang Word (24/10/2026): moi khoi la mot doan, khong dinh bold/mau/can le.

Loi truoc day (do tren file that): `writer.insertStyledText` chi TypeText nen 4 dong header + than bai
dinh vao MOT doan; `appendText` ke thua bold + can giua cua khoi truoc; `writer.restyle` thay doan
centered+bold nen to mau accent ca la don. Test nay goi thang bridge tren Word that, luu .docx roi doc
lai bang zipfile/XML (chi thu vien chuan) de kiem cau truc doan + can le + dinh dang.

    python tests/live/test_writer_format.py

Can Microsoft Office (Word) x64, da cai add-in. Khong chay trong CI vi CI khong co Office.
"""
from __future__ import annotations

import os
import sys
import tempfile
import zipfile
import xml.etree.ElementTree as ET

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import test_live_commands as live  # noqa: E402

W = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
FAILED: list[str] = []
CHECKS: list[str] = []


def check(ok: bool, name: str, detail: str = "") -> None:
    print(("PASS " if ok else "FAIL ") + name + ("" if ok or not detail else "\n     " + detail))
    CHECKS.append(name)
    if not ok:
        FAILED.append(name)


def style_alignments(styles) -> dict:
    """styleId -> can le cua style (theo basedOn); Word khong ghi jc vao doan khi doan can giong style cua no."""
    own, parent, default = {}, {}, "Normal"
    for style in styles.findall(W + "style"):
        sid = style.get(W + "styleId")
        if style.get(W + "type") == "paragraph" and style.get(W + "default") == "1":
            default = sid
        jc = style.find(W + "pPr/" + W + "jc")
        if jc is not None:
            own[sid] = jc.get(W + "val")
        based = style.find(W + "basedOn")
        if based is not None:
            parent[sid] = based.get(W + "val")

    def resolve(sid, depth=0):
        if sid in own or depth > 10 or sid not in parent:
            return own.get(sid, "left")
        return resolve(parent[sid], depth + 1)
    return {sid: resolve(sid) for sid in set(own) | set(parent) | {default}} | {"": resolve(default)}


def parse(path: str) -> list[dict]:
    with zipfile.ZipFile(path) as archive:
        root = ET.fromstring(archive.read("word/document.xml"))
        aligns = style_alignments(ET.fromstring(archive.read("word/styles.xml")))
    paragraphs = []
    for paragraph in root.findall(".//" + W + "body/" + W + "p"):
        ppr = paragraph.find(W + "pPr")
        style = ppr.find(W + "pStyle") if ppr is not None else None
        align = aligns.get(style.get(W + "val") if style is not None else "", aligns[""])
        if ppr is not None and ppr.find(W + "jc") is not None:
            align = ppr.find(W + "jc").get(W + "val")
        text = "".join(t.text or "" for t in paragraph.findall(".//" + W + "t"))
        bold = False
        colored = False
        for run in paragraph.findall(".//" + W + "r"):
            rpr = run.find(W + "rPr")
            if rpr is None:
                continue
            if rpr.find(W + "b") is not None:
                bold = True
            color = rpr.find(W + "color")
            if color is not None and color.get(W + "val") not in (None, "auto", "000000"):
                colored = True
        paragraphs.append({"text": text, "align": align, "bold": bold, "colored": colored})
    return paragraphs


def main() -> int:
    out = os.path.join(tempfile.gettempdir(), "axiom-writer-format.docx")
    try:
        os.remove(out)
    except OSError:
        pass

    port, pid = live.launch("word", False)
    bridge = live.Bridge(port, os.path.join(tempfile.gettempdir(), "axiom-format-live"), "word")
    try:
        bridge.cmd("writer.newDocument")
        bridge.cmd("writer.insertStyledText", {"text": "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM",
                                               "alignment": "center", "bold": True, "size": 13})
        bridge.cmd("writer.insertStyledText", {"text": "Độc lập - Tự do - Hạnh phúc",
                                               "alignment": "center", "bold": True, "size": 13})
        bridge.cmd("writer.insertStyledText", {"text": "[Địa danh], ngày ... tháng ... năm ...",
                                               "alignment": "center", "italic": True, "size": 13})
        bridge.cmd("writer.insertStyledText", {"text": "ĐƠN XIN VIỆC",
                                               "alignment": "center", "bold": True, "size": 16})
        bridge.cmd("writer.insertStyledText", {"text": "Tôi tên là: ...\nNgày sinh: ...\nĐịa chỉ: ...",
                                               "alignment": "justify", "size": 13})
        # Khong truyen bold/alignment: phai tu reset, khong ke thua khoi truoc.
        bridge.cmd("writer.appendText", {"text": "1. Trình độ học vấn:\n- Tốt nghiệp đại học\n- Chuyên ngành kế toán"})
        bridge.cmd("writer.appendText", {"text": "Người làm đơn\n(Ký, ghi rõ họ tên)", "alignment": "right"})
        # Noi sau mot doan can PHAI: chu ky van right, chi doan moi la left (Range khong bat dau tren
        # paragraph mark cua doan cu).
        bridge.cmd("writer.appendText", {"text": "Ghi chú: hồ sơ gửi kèm.", "alignment": "left"})
        # Caller gui kem '\n' dau chuoi: khong duoc tao them doan rong.
        bridge.cmd("writer.appendText", {"text": "\nGhi chú 2", "alignment": "left"})
        bridge.cmd("writer.saveAs", {"path": out})
    finally:
        live.close_app(pid)

    paragraphs = parse(out)
    texts = [p["text"] for p in paragraphs]
    check(len(paragraphs) == 14, "14 doan (4 header + 3 than + 3 muc + 2 chu ky + 2 ghi chu)", str(len(paragraphs)))
    check(texts[:4] == ["CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM", "Độc lập - Tự do - Hạnh phúc",
                        "[Địa danh], ngày ... tháng ... năm ...", "ĐƠN XIN VIỆC"],
          "header tach thanh 4 doan rieng", repr(texts[:5]))
    check(all(p["align"] == "center" for p in paragraphs[:4]), "4 dong header can giua",
          str([p["align"] for p in paragraphs[:4]]))
    check(all(p["align"] == "both" for p in paragraphs[4:7]), "than bai can deu",
          str([p["align"] for p in paragraphs[4:7]]))
    check(all(p["align"] == "left" for p in paragraphs[7:10]), "muc 1 + gach dau dong can trai",
          str([p["align"] for p in paragraphs[7:10]]))
    check(all(p["align"] == "right" for p in paragraphs[10:12]), "chu ky can phai",
          str([p["align"] for p in paragraphs[10:12]]))
    check(all(p["align"] == "left" for p in paragraphs[12:14]), "ghi chu sau chu ky can trai (khong keo doan cu)",
          str([p["align"] for p in paragraphs[10:14]]))
    check(not any(p["text"].strip() == "" for p in paragraphs), "khong co doan rong", repr(texts))
    check(not any(p["colored"] for p in paragraphs), "khong co mau chu ngoai automatic",
          str([(i + 1, p["text"][:20]) for i, p in enumerate(paragraphs) if p["colored"]]))
    check(not any(p["bold"] for p in paragraphs[7:]), "muc appendText khong bi bold ke thua",
          str([(i + 1, p["text"][:20]) for i, p in enumerate(paragraphs[7:], 7) if p["bold"]]))

    check_restyle()

    print("\n%d passed, %d failed" % (len(CHECKS) - len(FAILED), len(FAILED)))
    return 1 if FAILED else 0


def colored_runs(path: str) -> list[str]:
    """Moi run co mau chu khac den/automatic, ca trong bang."""
    with zipfile.ZipFile(path) as archive:
        root = ET.fromstring(archive.read("word/document.xml"))
    found = []
    for run in root.findall(".//" + W + "r"):
        color = run.find(W + "rPr/" + W + "color")
        if color is not None and color.get(W + "val") not in (None, "auto", "000000"):
            found.append(color.get(W + "val") + ":" + "".join(t.text or "" for t in run.findall(W + "t"))[:20])
    return found


def first_table_borders(path: str) -> list[str]:
    """Net ke THAT cua bang dau tien: o muc bang (tblBorders) va tung o (tcBorders), bo qua none/nil.
    Word ghi bang tat vien thanh KHONG co tblBorders, nen danh sach rong = khong vien."""
    with zipfile.ZipFile(path) as archive:
        root = ET.fromstring(archive.read("word/document.xml"))
    table = root.find(".//" + W + "tbl")
    if table is None:
        return ["<khong co bang>"]
    lines = []
    for group in table.findall(".//" + W + "tblBorders") + table.findall(".//" + W + "tcBorders"):
        lines.extend(side.get(W + "val") for side in group if side.get(W + "val") not in ("none", "nil"))
    if table.find(W + "tblPr/" + W + "tblStyle") is not None:
        lines.append("style=" + table.find(W + "tblPr/" + W + "tblStyle").get(W + "val"))
    return lines


def check_restyle() -> None:
    """writer.restyle: van ban hanh chinh giu chu den, giu khoang trong ky, khong ke khung bang dan trang;
    van ban thuong van to mau nhan nhu cu (do 24/10/2026: quoc hieu bi to #2E5C8A)."""
    admin_before = os.path.join(tempfile.gettempdir(), "axiom-restyle-admin-before.docx")
    admin_after = os.path.join(tempfile.gettempdir(), "axiom-restyle-admin-after.docx")
    design_after = os.path.join(tempfile.gettempdir(), "axiom-restyle-design-after.docx")
    port, pid = live.launch("word", False)
    bridge = live.Bridge(port, os.path.join(tempfile.gettempdir(), "axiom-format-live"), "word")
    try:
        bridge.cmd("writer.newDocument")
        # Phan dau dang bang 2 cot khong vien (skill van-ban-hanh-chinh): o trai trong la co y.
        bridge.cmd("writer.insertTable", {"values": [["", "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM"],
                                                     ["", "Độc lập - Tự do - Hạnh phúc"]]})
        bridge.cmd("writer.formatTable", {"borders": False, "font": "Times New Roman", "size": 13,
                                          "alignment": "center"})
        report = bridge.cmd("writer.checkTables") or {}
        tables = report.get("tables") or [{}]
        check(tables[0].get("layout") is True, "checkTables: bang borders:false la layout", str(tables[:1]))
        check(not any(i.get("type") == "empty-cells" for i in report.get("issues") or []),
              "checkTables: bang dan trang khong bao empty-cells", str(report.get("issues")))
        bridge.cmd("writer.insertStyledText", {"text": "ĐƠN XIN VIỆC", "alignment": "center", "bold": True, "size": 14})
        bridge.cmd("writer.insertStyledText", {"text": "Tôi tên là: ...", "alignment": "justify"})
        bridge.cmd("writer.insertStyledText", {"text": "Người làm đơn\n\n\n\n[Họ và tên]", "alignment": "right"})
        bridge.cmd("writer.saveAs", {"path": admin_before})
        result = bridge.cmd("writer.restyle") or {}
        check(result.get("mode") == "administrative", "restyle nhan ra van ban hanh chinh (co quoc hieu)", str(result))
        check(result.get("layoutTables") == 1 and result.get("tables") == 0, "restyle bo qua bang dan trang", str(result))
        bridge.cmd("writer.saveAs", {"path": admin_after})

        bridge.cmd("writer.newDocument")
        bridge.cmd("writer.insertStyledText", {"text": "I. MỞ ĐẦU", "alignment": "left", "bold": True})
        bridge.cmd("writer.insertStyledText", {"text": "Nội dung báo cáo.", "alignment": "justify"})
        design = bridge.cmd("writer.restyle") or {}
        check(design.get("mode") == "design", "van ban thuong: restyle che do thiet ke", str(design))
        bridge.cmd("writer.saveAs", {"path": design_after})
    finally:
        live.close_app(pid)

    check(not colored_runs(admin_after), "van ban hanh chinh sau restyle: chu den, khong mau nhan",
          str(colored_runs(admin_after)))
    empty_before = sum(1 for p in parse(admin_before) if not p["text"].strip())
    empty_after = sum(1 for p in parse(admin_after) if not p["text"].strip())
    check(empty_after >= 3 and empty_after == empty_before, "giu khoang trong ky (dong trong giua chuc danh va ho ten)",
          "truoc %d, sau %d" % (empty_before, empty_after))
    sides = first_table_borders(admin_after)
    check(not sides, "bang dan trang van khong vien sau restyle", str(sides))
    check(any(c.upper().startswith("2E5C8A") for c in colored_runs(design_after)),
          "van ban thuong: tieu de muc van to mau nhan #2E5C8A", str(colored_runs(design_after)))


if __name__ == "__main__":
    sys.exit(main())
