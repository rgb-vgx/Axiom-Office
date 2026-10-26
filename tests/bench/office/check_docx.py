"""Cham mot tep .docx theo tieu chi cau truc (de A va cac ca Word cua bo do van phong).

    python tests/bench/office/check_docx.py <result.docx> [--layout] [--json out.json]

Chi dung thu vien chuan: doc thang XML trong goi .docx (styles, document, header/footer). --layout mo tep bang Word
(COM qua PowerShell, chi doc) de lay so trang tung doan: tieu de nam cuoi trang, trang trang, bang bi cat - nhung thu
khong doc duoc tu XML vi phan trang do Word tinh khi hien.

Muc do (giong bao cao de xuat 26/10/2026): critical = mat/sai noi dung, hong tep; major = loi bo cuc ro hoac chuc nang
quan trong khong co (khong heading that, khong muc luc, khong so trang); minor = can le, khoang cach, dinh dang nho.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import zipfile
import xml.etree.ElementTree as ET

W = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
R = "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}"

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")


def attr(element, name: str) -> str:
    return element.get(W + name, "") if element is not None else ""


def load_styles(package: zipfile.ZipFile) -> dict:
    """styleId -> {name, outline, keepNext, font, size}. Ten style la ten goc tieng Anh ("heading 1") du Word ban dia."""
    styles = {}
    if "word/styles.xml" not in package.namelist():
        return styles
    root = ET.fromstring(package.read("word/styles.xml"))
    for style in root.iter(W + "style"):
        sid = attr(style, "styleId")
        name = attr(style.find(W + "name"), "val").lower()
        ppr = style.find(W + "pPr")
        outline = attr(ppr.find(W + "outlineLvl"), "val") if ppr is not None and ppr.find(W + "outlineLvl") is not None else ""
        based = attr(style.find(W + "basedOn"), "val")
        styles[sid] = {"name": name, "outline": outline, "basedOn": based,
                       "keepNext": ppr is not None and ppr.find(W + "keepNext") is not None}
    return styles


def heading_level(style_id: str, styles: dict) -> int:
    """Cap heading theo ten style ("heading N") hoac outlineLvl; 0 = khong phai heading."""
    seen = set()
    while style_id and style_id not in seen:
        seen.add(style_id)
        style = styles.get(style_id) or {}
        match = re.fullmatch(r"heading (\d)", style.get("name", ""))
        if match:
            return int(match.group(1))
        if style.get("outline", "") != "":
            return int(style["outline"]) + 1
        style_id = style.get("basedOn", "")
    return 0


def para_text(paragraph) -> str:
    return "".join(node.text or "" for node in paragraph.iter() if node.tag in (W + "t", W + "delText"))


def run_props(paragraph):
    """(moi run deu dam?, co chu lon nhat theo half-point, tap font truc tiep, tap co chu truc tiep)."""
    bold_all, sizes, fonts = True, set(), set()
    has_run = False
    for run in paragraph.iter(W + "r"):
        if not "".join(t.text or "" for t in run.iter(W + "t")).strip():
            continue
        has_run = True
        rpr = run.find(W + "rPr")
        bold = rpr is not None and rpr.find(W + "b") is not None and attr(rpr.find(W + "b"), "val") not in ("0", "false")
        bold_all = bold_all and bold
        if rpr is not None:
            if rpr.find(W + "sz") is not None:
                sizes.add(int(attr(rpr.find(W + "sz"), "val") or 0))
            if rpr.find(W + "rFonts") is not None:
                font = attr(rpr.find(W + "rFonts"), "ascii") or attr(rpr.find(W + "rFonts"), "hAnsi")
                if font:
                    fonts.add(font)
    return has_run and bold_all, max(sizes) if sizes else 0, fonts, sizes


def instr_texts(root) -> list[str]:
    texts = [node.text or "" for node in root.iter(W + "instrText")]
    texts += [attr(node, "instr") for node in root.iter(W + "fldSimple")]
    return texts


def check_package(path: str) -> dict:
    report: dict = {"file": os.path.basename(path), "issues": []}

    def issue(severity: str, code: str, detail: str) -> None:
        report["issues"].append({"severity": severity, "code": code, "detail": detail})

    try:
        package = zipfile.ZipFile(path)
        body = ET.fromstring(package.read("word/document.xml")).find(W + "body")
    except (OSError, KeyError, zipfile.BadZipFile, ET.ParseError) as error:
        issue("critical", "BROKEN_FILE", str(error))
        return report
    styles = load_styles(package)

    headings, fake, paragraphs, empty_run, max_empty = [], [], 0, 0, 0
    fonts, sizes, chars, page_breaks = set(), set(), 0, 0
    captions, previous_level = 0, 0
    jumps = []
    in_tables = {id(p) for table in body.iter(W + "tbl") for p in table.iter(W + "p")}
    for paragraph in body.iter(W + "p"):
        paragraphs += 1
        text = para_text(paragraph).strip()
        ppr = paragraph.find(W + "pPr")
        sid = attr(ppr.find(W + "pStyle"), "val") if ppr is not None and ppr.find(W + "pStyle") is not None else ""
        style_name = (styles.get(sid) or {}).get("name", "")
        page_breaks += sum(1 for br in paragraph.iter(W + "br") if attr(br, "type") == "page")
        if ppr is not None and ppr.find(W + "pageBreakBefore") is not None:
            page_breaks += 1
        if not text:
            if not list(paragraph.iter(W + "drawing")) and not any(attr(br, "type") == "page" for br in paragraph.iter(W + "br")):
                empty_run += 1
                max_empty = max(max_empty, empty_run)
            continue
        empty_run = 0
        chars += len(text)
        if style_name == "caption" or any("SEQ" in instr for instr in instr_texts(paragraph)):
            captions += 1
        level = heading_level(sid, styles)
        bold_all, size, run_fonts, run_sizes = run_props(paragraph)
        fonts |= run_fonts
        sizes |= run_sizes
        if level:
            headings.append({"level": level, "text": text[:80]})
            if previous_level and level > previous_level + 1:
                jumps.append("%s -> H%d '%s'" % ("H%d" % previous_level, level, text[:40]))
            previous_level = level
        elif len(text) <= 90 and (bold_all or size >= 28) and not text.endswith((".", ":", ";")) \
                and style_name not in ("title", "subtitle", "caption", "toc heading") and not style_name.startswith("toc"):
            # Doan ngan, dam/to, khong phai heading, ngoai bang, CAN TRAI: kieu "tieu de gia" (tieu chi 1 de A). Dong can
            # giua/phai la quoc hieu, tieu ngu, ten loai van ban, trich yeu, chuc danh ky, ten bia - in dam la dung
            # the thuc (Nghi dinh 30/2020), khong phai tieu de muc.
            align = attr(ppr.find(W + "jc"), "val") if ppr is not None and ppr.find(W + "jc") is not None else ""
            if id(paragraph) not in in_tables and align not in ("center", "right", "end"):
                fake.append(text[:60])

    tables = list(body.iter(W + "tbl"))
    repeat_header = sum(1 for table in tables if any(row.find(W + "trPr") is not None and
                                                     row.find(W + "trPr").find(W + "tblHeader") is not None
                                                     for row in table.iter(W + "tr")))
    instr = instr_texts(body)
    toc = any(re.search(r"\bTOC\b", text) for text in instr) or any(
        (styles.get(attr(p.find(W + "pPr").find(W + "pStyle"), "val")) or {}).get("name", "").startswith("toc")
        for p in body.iter(W + "p") if p.find(W + "pPr") is not None and p.find(W + "pPr").find(W + "pStyle") is not None)
    drawings = len(list(body.iter(W + "drawing")))

    # Header/footer: co tham chieu trong sectPr va co truong PAGE.
    sections = list(body.iter(W + "sectPr"))
    header_refs = sum(len(section.findall(W + "headerReference")) for section in sections)
    footer_refs = sum(len(section.findall(W + "footerReference")) for section in sections)
    page_field, header_text = False, ""
    for name in package.namelist():
        if re.fullmatch(r"word/(header|footer)\d*\.xml", name):
            part = ET.fromstring(package.read(name))
            if any(re.search(r"\bPAGE\b", text) for text in instr_texts(part)):
                page_field = True
            header_text += "".join(t.text or "" for t in part.iter(W + "t"))
    margins = []
    for section in sections:
        mar = section.find(W + "pgMar")
        if mar is not None:
            margins.append({k: round(int(attr(mar, k) or 0) / 567, 2) for k in ("top", "bottom", "left", "right")})

    full_text = "\n".join(para_text(p) for p in body.iter(W + "p"))
    report.update({
        "paragraphs": paragraphs, "chars": chars, "headings": len(headings),
        "headingLevels": {str(n): sum(1 for h in headings if h["level"] == n) for n in (1, 2, 3)},
        "headingList": headings[:40], "fakeHeadings": fake[:20], "levelJumps": jumps,
        "tables": len(tables), "tablesRepeatHeader": repeat_header, "captions": captions, "toc": toc,
        "headerRefs": header_refs, "footerRefs": footer_refs, "pageField": page_field, "headerFooterText": header_text[:120],
        "pageBreaks": page_breaks, "maxEmptyParagraphs": max_empty, "drawings": drawings,
        "directFonts": sorted(fonts), "directSizesPt": sorted(size / 2 for size in sizes), "marginsCm": margins[:3],
        "assumptionMarked": bool(re.search(r"giả định|giả thiết|minh họa|minh hoạ|ước tính", full_text, re.I)),
    })

    # Tieu chi de A -> loi.
    if not headings:
        issue("major", "NO_HEADING_STYLES", "khong doan nao dung style Heading")
    if fake:
        issue("major" if len(fake) > 2 else "minor", "FAKE_HEADINGS", "%d doan ngan dam/to khong dung Heading: %s" % (len(fake), fake[:5]))
    if jumps:
        issue("minor", "HEADING_LEVEL_JUMP", "; ".join(jumps[:5]))
    if not toc:
        issue("major", "NO_TOC", "khong co truong TOC")
    if not (header_refs or footer_refs):
        issue("major", "NO_HEADER_FOOTER", "sectPr khong tham chieu header/footer")
    if not page_field:
        issue("major", "NO_PAGE_NUMBER", "header/footer khong co truong PAGE")
    if tables and captions < len(tables):
        issue("minor", "TABLE_WITHOUT_CAPTION", "%d bang, %d caption" % (len(tables), captions))
    if tables and repeat_header < len(tables):
        issue("minor", "TABLE_HEADER_NOT_REPEATED", "%d/%d bang lap hang tieu de" % (repeat_header, len(tables)))
    if max_empty >= 3:
        issue("minor", "EMPTY_PARAGRAPH_RUN", "%d doan trong lien tiep (dan trang bang Enter)" % max_empty)
    if len(fonts) > 3 or len(sizes) > 5:
        issue("minor", "INCONSISTENT_DIRECT_FORMAT", "font truc tiep %s, co chu %s" % (sorted(fonts), sorted(s / 2 for s in sizes)))
    if not report["assumptionMarked"]:
        issue("minor", "ASSUMPTION_NOT_MARKED", "khong thay chu 'gia dinh/minh hoa/uoc tinh'")
    return report


# Bo cuc trang tu Word (chi doc): trang cua tung heading/bang, trang trong. wdActiveEndPageNumber = 3,
# wdVerticalPositionRelativeToPage = 6, wdNumberOfPagesInDocument = 4.
LAYOUT_PS = r"""
$ErrorActionPreference = 'Stop'
$path = $args[0]
$word = New-Object -ComObject Word.Application
$word.Visible = $false
$word.DisplayAlerts = 0
try {
  $doc = $word.Documents.Open($path, $false, $true)
  $doc.Repaginate()
  $pages = $doc.ComputeStatistics(2)
  $height = $doc.PageSetup.PageHeight
  $bottom = $doc.PageSetup.BottomMargin
  $items = @()
  $textPages = @{}
  $n = $doc.Paragraphs.Count
  for ($i = 1; $i -le $n; $i++) {
    $p = $doc.Paragraphs.Item($i)
    $t = $p.Range.Text.Trim()
    $page = $p.Range.Information(3)
    if ($t.Length -gt 0) { $textPages[[string]$page] = 1 }
    if ($p.OutlineLevel -lt 10 -and $t.Length -gt 0) {
      $items += [pscustomobject]@{ kind = 'heading'; level = $p.OutlineLevel; page = $page; top = $p.Range.Information(6); text = $t.Substring(0, [Math]::Min(60, $t.Length)); nextPage = $(if ($i -lt $n) { $doc.Paragraphs.Item($i + 1).Range.Information(3) } else { $page }) }
    }
  }
  foreach ($tb in $doc.Tables) {
    $first = $tb.Range.Characters.First.Information(3)
    $last = $tb.Range.Characters.Last.Information(3)
    $items += [pscustomobject]@{ kind = 'table'; rows = $tb.Rows.Count; page = $first; lastPage = $last }
  }
  $blank = @()
  for ($k = 1; $k -le $pages; $k++) { if (-not $textPages.ContainsKey([string]$k)) { $blank += $k } }
  [pscustomobject]@{ pages = $pages; pageHeight = $height; bottomMargin = $bottom; items = $items; blankPages = $blank } | ConvertTo-Json -Depth 4 -Compress
  $doc.Close(0)
} finally { $word.Quit() }
"""


def check_layout(path: str, report: dict) -> None:
    script = os.path.join(os.environ.get("TEMP", "."), "axiom_docx_layout.ps1")
    with open(script, "w", encoding="utf-8-sig") as handle:
        handle.write(LAYOUT_PS)
    done = subprocess.run(["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script, os.path.abspath(path)],
                          capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=600)
    try:
        layout = json.loads(done.stdout.strip().splitlines()[-1])
    except (IndexError, json.JSONDecodeError):
        report["issues"].append({"severity": "minor", "code": "LAYOUT_UNREADABLE", "detail": (done.stderr or done.stdout)[-300:]})
        return
    items = layout.get("items") or []
    if isinstance(items, dict):
        items = [items]
    report["pages"] = layout.get("pages")
    report["blankPages"] = layout.get("blankPages") or []
    limit = (layout.get("pageHeight") or 842) - (layout.get("bottomMargin") or 72) - 90
    stranded = [i["text"] for i in items if i["kind"] == "heading" and i.get("nextPage", i["page"]) > i["page"]
                and (i.get("top") or 0) > limit]
    split = [i for i in items if i["kind"] == "table" and i["lastPage"] > i["page"] and i["rows"] <= 15]
    report["strandedHeadings"] = stranded
    report["splitSmallTables"] = len(split)
    if report["blankPages"]:
        report["issues"].append({"severity": "major", "code": "BLANK_PAGE", "detail": "trang trong: %s" % report["blankPages"]})
    if stranded:
        report["issues"].append({"severity": "major", "code": "HEADING_AT_PAGE_BOTTOM", "detail": "; ".join(stranded[:5])})
    if split:
        report["issues"].append({"severity": "minor", "code": "SMALL_TABLE_SPLIT", "detail": "%d bang <=15 dong bi cat sang trang" % len(split)})


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("path")
    parser.add_argument("--layout", action="store_true", help="mo bang Word (COM) de do phan trang")
    parser.add_argument("--json", default="")
    args = parser.parse_args()
    report = check_package(args.path)
    if args.layout and not any(i["code"] == "BROKEN_FILE" for i in report["issues"]):
        check_layout(args.path, report)
    text = json.dumps(report, ensure_ascii=False, indent=1)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as handle:
            handle.write(text)
    print(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
