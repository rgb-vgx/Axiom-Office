"""Cham mot lan chay brief slide bang du lieu doc tu tai lieu - KHONG dung LLM (docs/plans/17 muc 5).

Dau vao cua mot lan chay (thu muc do run.py ghi):
    brief.json   brief da chay (mot muc cua briefs.json)
    run.json     so do cua luot: rounds, toolCalls, failures, billable, seconds, outcome
    check.json   ket qua wpp.checkLayout (moi slide)
    slides.json  ket qua wpp.listSlides {detail: true}

    python tests/bench/slides/score.py <thu_muc_ket_qua> [--csv out.csv]

In bang Markdown: moi brief mot dong, gop qua cac lan chay.

Chi so cung (hard) quyet dinh dat/khong; chi so mem (soft) chi de so giua cac lan chay, khong ket luan dat.
"""
from __future__ import annotations

import csv
import html
import json
import os
import re
import statistics
import sys
import unicodedata
import zipfile
from collections import Counter
from xml.etree import ElementTree

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

# Luoi cua skill trinh-bay-chuyen-nghiep muc 1 va _design/tokens.json (slide.margin = 40): tieu de top 30,
# dong nguon day o H - 12. Lech qua TOLERANCE pt moi tinh.
MARGIN = 40.0
TOP_SAFE = 20.0
BOTTOM_SAFE = 12.0
TOLERANCE = 2.0
DENSE_CHARS = 600


def normalize(text: str) -> str:
    """So khop chu khong phan biet hoa thuong, khoang trang, dang Unicode (NFC) va dau cach khong ngat."""
    text = unicodedata.normalize("NFC", str(text or "")).replace(" ", " ").casefold()
    return re.sub(r"\s+", " ", text).strip()


def slide_texts(slides: dict) -> list[str]:
    return ["\n".join(entry.get("shapeTexts") or []) for entry in (slides or {}).get("slides") or []]


NUMBER = re.compile(r"\d+(?:[.,]\d+)*")


def numbers(text: str) -> set[str]:
    """Cac con so trong chu, bo dau phan cach hang nghin/thap phan de "1.240" khop "1240" va "98,5" khop "98.5"."""
    return {re.sub(r"[.,]", "", n) for n in NUMBER.findall(text or "")}


def unsourced_numbers(prompt: str, texts: list[str]) -> list[str]:
    """So tren slide khong co trong de (bat noi dung tu bia, kieu s13 lan 2 ngay 25/10/2026). Chi so MEM: so tinh ra
    tu de (ty le, tong) cung bi dem; bo so 1 chu so (danh so muc, "3 ly do")."""
    source = numbers(prompt)
    found = numbers("\n".join(texts)) - source
    return sorted(n for n in found if len(n) > 1)


CHART_NS = {"c": "http://schemas.openxmlformats.org/drawingml/2006/chart",
            "a": "http://schemas.openxmlformats.org/drawingml/2006/main"}


def chart_value(value: str, code: str) -> str:
    """Gia tri nhu nhan hien thi: chuoi hang trong ngoac kep la don vi, % ngoai ngoac thi nhan 100 (khop
    FormatChartValue cua bridge)."""
    try:
        number = float(value)
    except ValueError:
        return value
    suffix, quoted, percent = "", False, False
    for ch in code or "":
        if ch == '"':
            quoted = not quoted
        elif quoted:
            suffix += ch
        elif ch == "%":
            percent = True
    if percent:
        number, suffix = number * 100, suffix + "%"
    return ("%.2f" % round(number, 2)).rstrip("0").rstrip(".") + suffix


def chart_text(xml: bytes) -> str:
    """Chu cua mot bieu do goc: tieu de, roi moi chuoi "ten: danh muc gia tri+don vi". So lieu nam o
    ppt/charts/chartN.xml, khong co trong slide (docs/plans/18-s0 muc 9.3)."""
    root = ElementTree.fromstring(xml)
    lines = [" ".join(t.text or "" for t in root.iterfind(".//c:title//a:t", CHART_NS)).strip()]
    for series in root.iterfind(".//c:ser", CHART_NS):
        name = " ".join(v.text or "" for v in series.iterfind("c:tx//c:v", CHART_NS))
        categories = [v.text or "" for v in series.iterfind("c:cat//c:pt/c:v", CHART_NS)]
        cache = series.find("c:val//c:numCache", CHART_NS)
        code = cache.findtext("c:formatCode", "", CHART_NS) if cache is not None else ""
        label = series.find("c:dLbls/c:numFmt", CHART_NS)
        if label is not None and label.get("sourceLinked") == "0":
            code = label.get("formatCode", code)
        values = [chart_value(v.text or "", code) for v in series.iterfind("c:val//c:pt/c:v", CHART_NS)]
        lines.append(name + ": " + ", ".join("%s %s" % pair for pair in zip(categories, values)))
    return "\n".join(line for line in lines if line)


def deck_texts(path: str) -> list[str]:
    """Chu tung slide doc thang tu .pptx (moi <a:t>, ca o bang, va so lieu bieu do goc). wpp.listSlides chi doc
    shape co TextFrame nen BO SOT chu trong bang (luot 25/10/2026: bang so lieu dung du ma bi cham thieu so)."""
    texts = []
    with zipfile.ZipFile(path) as package:
        members = set(package.namelist())
        names = [n for n in members if re.fullmatch(r"ppt/slides/slide\d+\.xml", n)]
        for name in sorted(names, key=lambda n: int(re.search(r"(\d+)\.xml$", n).group(1))):
            xml = package.read(name).decode("utf-8")
            parts = [html.unescape(t) for t in re.findall(r"<a:t>([^<]*)</a:t>", xml)]
            rels = name.replace("ppt/slides/", "ppt/slides/_rels/") + ".rels"
            if rels in members:
                for target in re.findall(r'Target="\.\./charts/(chart\d+\.xml)"', package.read(rels).decode("utf-8")):
                    if "ppt/charts/" + target in members:
                        parts.append(chart_text(package.read("ppt/charts/" + target)))
            texts.append("\n".join(parts))
    return texts


def is_background(shape: dict, width: float, height: float) -> bool:
    return shape.get("width", 0) * shape.get("height", 0) >= 0.9 * width * height


def title_of(shapes: list[dict]) -> dict | None:
    """Hop chu cao nhat (top nho nhat); bang nhau thi hop rong hon - du cho slide dung theo luoi."""
    texts = [s for s in shapes if s.get("chars", 0) > 0]
    if not texts:
        return None
    return min(texts, key=lambda s: (round(s.get("top", 0)), -s.get("width", 0)))


def title_drift(check: dict) -> float:
    """Do lech lon nhat (pt) cua vi tri tieu de so voi trung vi cac slide - tieu de nhay cho giua cac slide.
    Bo slide chi mot hop chu (trich dan) va slide co hop chu cao nhat nam duoi 1/4 slide (bia, chia muc: tieu de dat
    giua la chu y - luot 25/10/2026 do nham 154 pt vi so tieu de bia o top 184 voi tieu de noi dung o top 30)."""
    height = float(check.get("slideHeight") or 540)
    titles = []
    for entry in check.get("slides") or []:
        shapes = entry.get("shapes") or []
        if sum(1 for s in shapes if s.get("chars", 0) > 0) < 2:
            continue
        title = title_of(shapes)
        if title and title.get("top", 0) < height / 4:
            titles.append((title.get("left", 0), title.get("top", 0)))
    if len(titles) < 2:
        return 0.0
    left = statistics.median(t[0] for t in titles)
    top = statistics.median(t[1] for t in titles)
    return round(max(abs(t[0] - left) + abs(t[1] - top) for t in titles), 1)


def margin_violations(check: dict, margin: float = MARGIN) -> list[str]:
    """Shape ra ngoai vung an toan cua luoi (khong tinh shape nen phu ca slide)."""
    width, height = float(check.get("slideWidth") or 960), float(check.get("slideHeight") or 540)
    found = []
    for entry in check.get("slides") or []:
        for shape in entry.get("shapes") or []:
            if is_background(shape, width, height):
                continue
            left, top = shape.get("left", 0), shape.get("top", 0)
            right, bottom = left + shape.get("width", 0), top + shape.get("height", 0)
            if (left < margin - TOLERANCE or right > width - margin + TOLERANCE
                    or top < TOP_SAFE - TOLERANCE or bottom > height - BOTTOM_SAFE + TOLERANCE):
                found.append("%s:%s" % (entry.get("index"), shape.get("name")))
    return found


def score_run(brief: dict, check: dict, slides: dict, run: dict | None = None, texts: list[str] | None = None) -> dict:
    """texts: chu tung slide doc tu .pptx (deck_texts); thieu thi lay tu wpp.listSlides."""
    check, slides, run = check or {}, slides or {}, run or {}
    count = int(slides.get("slideCount") or len(slides.get("slides") or []))
    low, high = (brief.get("slides") or [1, 99])[:2]
    issues: Counter[str] = Counter()
    clean = 0
    max_chars = 0
    for entry in check.get("slides") or []:
        found = entry.get("issues") or []
        clean += 0 if found else 1
        for issue in found:
            issues[str(issue.get("type") or "?")] += 1
        max_chars = max(max_chars, sum(s.get("chars", 0) for s in entry.get("shapes") or []))
    deck = normalize("\n".join(texts if texts is not None else slide_texts(slides)))
    missing = [phrase for phrase in brief.get("mustContain") or [] if normalize(phrase) not in deck]
    # mustRemove: chu cai san trong bai (brief.seed) phai BI MAT sau khi sua (ket luan 04-)
    leftover = [phrase for phrase in brief.get("mustRemove") or [] if normalize(phrase) in deck]
    # readGap: cum co tren slide (.pptx) ma wpp.listSlides - lenh doc agent dung de tu soat - khong thay (19-h0 H3).
    listed = normalize("\n".join(slide_texts(slides)))
    read_gap = [phrase for phrase in brief.get("mustContain") or []
                if normalize(phrase) in deck and normalize(phrase) not in listed] if texts is not None else []
    checked = len(check.get("slides") or [])
    outcome = run.get("outcome") or "unknown"
    result = {
        "id": brief.get("id"),
        "outcome": outcome,
        "slides": count,
        "slidesOk": low <= count <= high,
        "issues": sum(issues.values()),
        "issuesByType": dict(sorted(issues.items())),
        "cleanSlideRatio": round(clean / checked, 3) if checked else 0.0,
        "missing": missing,
        "leftover": leftover,
        "readGap": read_gap,
        "titleDrift": title_drift(check),
        "marginViolations": len(margin_violations(check)),
        "maxChars": max_chars,
        "unsourced": unsourced_numbers(brief.get("prompt", ""), texts if texts is not None else slide_texts(slides)),
        "rounds": run.get("rounds"),
        "toolCalls": run.get("toolCalls"),
        "failures": run.get("failures"),
        "billable": run.get("billable"),
        "seconds": run.get("seconds"),
    }
    # Hop le: .pptx da luu dung la bai cua luot. Lenh khong chon bai (wpp.addSlide, wpp.saveAs) lam tren bai ACTIVE;
    # luot 25/10/2026 s09 lan 1: bai moi khong thanh active, slide ghi noi vao bai cua luot truoc va file luu la bai do
    # (2 slide) trong khi listSlides doc bai dich (1 slide) - phai loai, khong cham (docs/plans/19-h0 H1).
    result["valid"] = texts is None or len(texts) == count
    # Dat (hard): luot xong, so slide trong khoang, checkLayout sach, khong mat so lieu nao cua brief, va LOI CAI
    # SAN (mustRemove) khong con tren slide - day la thu do hum kiem tra doc lai sau khi sua cua luot that.
    result["pass"] = (result["valid"] and outcome == "run.completed" and result["slidesOk"] and result["issues"] == 0
                      and not missing and not leftover and count > 0)
    return result


def load(path: str) -> dict:
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def score_dir(run_dir: str) -> dict | None:
    def part(name):
        file = os.path.join(run_dir, name)
        return load(file) if os.path.exists(file) else {}
    brief = part("brief.json")
    if not brief:
        return None
    deck = os.path.join(run_dir, "deck.pptx")
    texts = deck_texts(deck) if os.path.exists(deck) else None
    return score_run(brief, part("check.json"), part("slides.json"), part("run.json"), texts)


def collect(root: str) -> list[dict]:
    """Moi thu muc con co brief.json la mot lan chay (<root>/<brief>/<lan>/)."""
    results = []
    for folder, _dirs, files in os.walk(root):
        if "brief.json" in files:
            scored = score_dir(folder)
            if scored:
                scored["dir"] = os.path.relpath(folder, root)
                results.append(scored)
    return sorted(results, key=lambda r: (str(r["id"]), r["dir"]))


def mean(values) -> float | str:
    numbers = [v for v in values if isinstance(v, (int, float))]
    return round(statistics.mean(numbers), 1) if numbers else "-"


def summary(results: list[dict]) -> list[dict]:
    by_brief: dict[str, list[dict]] = {}
    for result in results:
        by_brief.setdefault(str(result["id"]), []).append(result)
    rows = []
    for brief, runs in sorted(by_brief.items()):
        invalid = sum(1 for r in runs if not r["valid"])
        runs = [r for r in runs if r["valid"]]
        rows.append({
            "brief": brief,
            "runs": len(runs),
            "invalid": invalid,
            "pass": "%d/%d" % (sum(1 for r in runs if r["pass"]), len(runs)),
            "issues": mean(r["issues"] for r in runs),
            "missing": mean(len(r["missing"]) for r in runs),
            "leftover": mean(len(r.get("leftover") or []) for r in runs),
            "titleDrift": mean(r["titleDrift"] for r in runs),
            "margin": mean(r["marginViolations"] for r in runs),
            "unsourced": mean(len(r["unsourced"]) for r in runs),
            "readGap": mean(len(r["readGap"]) for r in runs),
            "rounds": mean(r["rounds"] for r in runs),
            "billable": mean(r["billable"] for r in runs),
        })
    return rows


def markdown(rows: list[dict]) -> str:
    if not rows:
        return "(không có lần chạy nào)"
    keys = list(rows[0].keys())
    lines = ["| " + " | ".join(keys) + " |", "|" + "---|" * len(keys)]
    lines += ["| " + " | ".join(str(row[k]) for k in keys) + " |" for row in rows]
    return "\n".join(lines)


def main(argv: list[str]) -> int:
    if not argv:
        print(__doc__)
        return 2
    results = collect(argv[0])
    rows = summary(results)
    print(markdown(rows))
    if "--csv" in argv:
        out = argv[argv.index("--csv") + 1]
        with open(out, "w", encoding="utf-8", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=list(results[0].keys()) if results else ["id"])
            writer.writeheader()
            for result in results:
                writer.writerow({k: json.dumps(v, ensure_ascii=False) if isinstance(v, (list, dict)) else v
                                 for k, v in result.items()})
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
