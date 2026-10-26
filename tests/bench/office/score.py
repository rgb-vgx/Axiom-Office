"""Cham ket qua bo do Word/Excel (run.py) theo briefs.json + check_docx/check_xlsx.

    python tests/bench/office/score.py tests/bench/office/results/<label> [--no-app] [--json out.json]

Moi lan chay: dat (pass) khi khong co loi critical, du moi tieu chi `expect`, du mustContain/mustKeep, mat mustRemove.
Loi cua check_* (major/minor) van dem va bao, nhung chi `expect` cua brief quyet dinh dat/khong - de nho (sua mot muc)
khong bi danh truot vi thieu muc luc. --no-app bo phan can mo Word/Excel (do phan trang, tinh lai).
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import zipfile
import xml.etree.ElementTree as ET

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import check_docx  # noqa: E402
import check_xlsx  # noqa: E402

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")


def normalize(text: str) -> str:
    return re.sub(r"\s+", " ", (text or "").replace(" ", " ")).strip().lower()


def docx_text(path: str) -> str:
    root = ET.fromstring(zipfile.ZipFile(path).read("word/document.xml"))
    return "\n".join(check_docx.para_text(p) for p in root.iter(check_docx.W + "p"))


def docx_table_rows(path: str) -> int:
    root = ET.fromstring(zipfile.ZipFile(path).read("word/document.xml"))
    return max((len(table.findall(check_docx.W + "tr")) for table in root.iter(check_docx.W + "tbl")), default=0)


def xlsx_cells(path: str) -> dict:
    """{(sheet, ref): (value, formula)} cho moi sheet; value da doi chuoi chia se."""
    package = zipfile.ZipFile(path)
    workbook = ET.fromstring(package.read("xl/workbook.xml"))
    targets = check_xlsx.rels(package, "xl/workbook.xml")
    shared = []
    if "xl/sharedStrings.xml" in package.namelist():
        shared = ["".join(t.text or "" for t in item.iter(check_xlsx.S + "t"))
                  for item in ET.fromstring(package.read("xl/sharedStrings.xml")).iter(check_xlsx.S + "si")]
    cells = {}
    for index, sheet in enumerate(workbook.iter(check_xlsx.S + "sheet")):
        root = ET.fromstring(package.read(targets[sheet.get(check_xlsx.RID)]))
        for cell in root.iter(check_xlsx.S + "c"):
            value_node, formula = cell.find(check_xlsx.S + "v"), cell.find(check_xlsx.S + "f")
            value = value_node.text if value_node is not None else "".join(t.text or "" for t in cell.iter(check_xlsx.S + "t"))
            if cell.get("t") == "s" and value not in (None, ""):
                value = shared[int(value)]
            key = (sheet.get("name"), cell.get("r"))
            cells[key] = (value, formula.text if formula is not None else None)
            cells[(index, cell.get("r"))] = cells[key]
    return cells


def same_value(actual, expected) -> bool:
    if isinstance(expected, (int, float)):
        try:
            return abs(float(actual) - float(expected)) < 1e-6
        except (TypeError, ValueError):
            return False
    return normalize(str(actual)) == normalize(str(expected))


def score_word(path: str, brief: dict, use_app: bool) -> tuple[dict, list]:
    expect = brief.get("expect") or {}
    report = check_docx.check_package(path)
    needs_layout = expect.get("noBlankPage") or expect.get("pagesBetween")
    if use_app and needs_layout:
        check_docx.check_layout(path, report)
    text = docx_text(path)
    failed = []
    if expect.get("toc") and not report.get("toc"):
        failed.append("toc")
    if expect.get("pageNumber") and not report.get("pageField"):
        failed.append("pageNumber")
    if expect.get("headerText") and normalize(expect["headerText"]) not in normalize(report.get("headerFooterText", "")):
        failed.append("headerText")
    for level, minimum in (expect.get("minHeadings") or {}).items():
        if report.get("headingLevels", {}).get(level, 0) < minimum:
            failed.append("H%s>=%d (co %d)" % (level, minimum, report.get("headingLevels", {}).get(level, 0)))
    if "maxFakeHeadings" in expect and len(report.get("fakeHeadings") or []) > expect["maxFakeHeadings"]:
        failed.append("fakeHeadings<=%d (co %d)" % (expect["maxFakeHeadings"], len(report["fakeHeadings"])))
    if "maxEmptyRun" in expect and report.get("maxEmptyParagraphs", 0) > expect["maxEmptyRun"]:
        failed.append("emptyRun")
    if expect.get("minTables") and report.get("tables", 0) < expect["minTables"]:
        failed.append("tables>=%d" % expect["minTables"])
    if expect.get("captions") and report.get("captions", 0) < expect["captions"]:
        failed.append("captions>=%d (co %d)" % (expect["captions"], report.get("captions", 0)))
    if expect.get("repeatHeader") and not report.get("tablesRepeatHeader"):
        failed.append("repeatHeader")
    if expect.get("minTableRows") and docx_table_rows(path) < expect["minTableRows"]:
        failed.append("tableRows>=%d (co %d)" % (expect["minTableRows"], docx_table_rows(path)))
    if expect.get("headingOrder"):
        titles = [normalize(h["text"]) for h in report.get("headingList") or []]
        positions = [next((i for i, t in enumerate(titles) if normalize(want) in t), -1) for want in expect["headingOrder"]]
        if -1 in positions or positions != sorted(positions):
            failed.append("headingOrder")
    if use_app and expect.get("noBlankPage") and report.get("blankPages"):
        failed.append("blankPages %s" % report["blankPages"])
    if use_app and expect.get("pagesBetween") and report.get("pages") is not None:
        low, high = expect["pagesBetween"]
        if not low <= report["pages"] <= high:
            failed.append("pages %d ngoai %d-%d" % (report["pages"], low, high))
    if expect.get("noInventedQ4"):
        # So de cho + so suy ra (tong 3 quy, chenh lech giua cac quy): khong phai so bia cho quy 4.
        given = [12.5, 13.1, 14.0]
        derived = {round(sum(given), 1)} | {round(abs(a - b), 1) for a in given for b in given}
        allowed = {"12,5", "13,1", "14,0"} | {("%.1f" % value).replace(".", ",") for value in derived}
        invented = [n for n in re.findall(r"\d+,\d+(?=\s*tỷ)", text) if n not in allowed]
        if invented:
            failed.append("so tu bia %s" % invented[:4])
    failed += content_checks(brief, normalize(text))
    return report, failed


def content_checks(brief: dict, text: str) -> list:
    failed = []
    for phrase in brief.get("mustContain") or []:
        # Mot muc la danh sach = chap nhan mot trong cac cach viet ("125 triệu" / "125.000.000").
        options = phrase if isinstance(phrase, list) else [phrase]
        if not any(normalize(option) in text for option in options):
            failed.append("thieu '%s'" % options[0])
    for phrase in brief.get("mustKeep") or []:
        if normalize(phrase) not in text:
            failed.append("mat chu cu '%s'" % phrase[:40])
    for phrase in brief.get("mustRemove") or []:
        if normalize(phrase) in text:
            failed.append("con '%s'" % phrase)
    return failed


def score_excel(path: str, brief: dict, use_app: bool) -> tuple[dict, list]:
    expect = brief.get("expect") or {}
    required = expect.get("sheets") or []
    report = check_xlsx.check_package(path, required)
    if use_app and expect.get("recalc") and all(s in report.get("sheetNames", []) for s in expect["recalc"].values()):
        check_xlsx.check_recalc(path, report, expect["recalc"]["source"], expect["recalc"]["target"])
    sheets = report.get("sheets") or {}
    cells = xlsx_cells(path) if not any(i["code"] == "BROKEN_FILE" for i in report["issues"]) else {}
    failed = []
    if any(i["code"] == "MISSING_SHEETS" for i in report["issues"]):
        failed.append("sheets %s" % required)
    if expect.get("minFormulas") and sum(s["formulas"] for s in sheets.values()) < expect["minFormulas"]:
        failed.append("formulas>=%d (co %d)" % (expect["minFormulas"], sum(s["formulas"] for s in sheets.values())))
    if expect.get("noErrors") and any(s["errors"] for s in sheets.values()):
        failed.append("o loi %s" % [e for s in sheets.values() for e in s["errors"]][:4])
    kinds = {kind for s in sheets.values() for column in s["formatKinds"].values() for kind in column}
    if expect.get("groupedNumbers") and "grouped" not in kinds:
        failed.append("groupedNumbers")
    if expect.get("percent") and "percent" not in kinds:
        failed.append("percent")
    if expect.get("dates") and "date" not in kinds:
        failed.append("dates")
    if expect.get("frozen") and not any(s["frozen"] for s in sheets.values()):
        failed.append("frozen")
    if expect.get("autoFilter") and not any(s["autoFilter"] or s["tables"] for s in sheets.values()):
        failed.append("autoFilter")
    if expect.get("listValidation") and not any(v["type"] == "list" for s in sheets.values() for v in s["validations"]):
        failed.append("listValidation")
    rules = [rule for s in sheets.values() for rule in s["conditional"]]
    if expect.get("conditional") and not rules:
        failed.append("conditional")
    if expect.get("cfReferences") and not any(expect["cfReferences"].replace("$", "") in rule["formula"].replace("$", "") for rule in rules):
        failed.append("cf doc %s (rule: %s)" % (expect["cfReferences"], [r["formula"] for r in rules][:3]))
    if expect.get("chartOn") and not (sheets.get(expect["chartOn"]) or {}).get("charts"):
        failed.append("chart tren %s" % expect["chartOn"])
    if expect.get("chartOn") and (sheets.get(expect["chartOn"]) or {}).get("charts", 0) > 1:
        failed.append("%d bieu do tren %s (thua)" % (sheets[expect["chartOn"]]["charts"], expect["chartOn"]))
    if expect.get("chartSource"):
        # Chi bieu do cua sheet de neu (chartOn): bieu do lac o sheet khac khong tinh (x09 lan dau, 26/10/2026).
        holder = [sheets.get(expect["chartOn"]) or {}] if expect.get("chartOn") else list(sheets.values())
        refs = [ref for s in holder for ref in s.get("chartRefs") or []]
        if not any(expect["chartSource"] in ref for ref in refs):
            failed.append("chart lay tu %s (refs %s)" % (expect["chartSource"], refs[:3]))
    if expect.get("recalc") and use_app:
        recalc = report.get("recalc") or {}
        if not recalc.get("moved"):
            failed.append("tong hop khong doi khi doi %s" % recalc.get("changedInput"))
    for ref, needle in (expect.get("formulaContains") or {}).items():
        formula = (cells.get((0, ref)) or (None, None))[1] or ""
        if needle.replace("$", "") not in formula.replace("$", ""):
            failed.append("%s=%s (can %s)" % (ref, formula or "?", needle))
    if "parameterCell" in expect and not any(formula is None and same_value(value, expect["parameterCell"])
                                             for (key, _), (value, formula) in cells.items() if isinstance(key, str)):
        failed.append("o tham so %s" % expect["parameterCell"])
    for ref, value in (brief.get("mustKeepCells") or {}).items():
        from run import resolve  # noqa: E402 - chi can khi brief co du lieu sinh
        actual = (cells.get((0, ref)) or (None, None))[0]
        if not same_value(actual, resolve(value)):
            failed.append("o %s doi: %s" % (ref, actual))
    text = normalize(" ".join(str(v) for (key, _), (v, _f) in cells.items() if isinstance(key, str) and v))
    failed += content_checks(brief, text)
    return report, failed


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("folder")
    parser.add_argument("--no-app", action="store_true")
    parser.add_argument("--json", default="")
    args = parser.parse_args()
    with open(os.path.join(HERE, "briefs.json"), encoding="utf-8") as handle:
        current = {brief["id"]: brief for brief in json.load(handle)["briefs"]}
    rows = []
    for brief_id in sorted(os.listdir(args.folder)):
        brief_dir = os.path.join(args.folder, brief_id)
        if not os.path.isdir(brief_dir):
            continue
        for n in sorted(os.listdir(brief_dir)):
            folder = os.path.join(brief_dir, n)
            if not os.path.isdir(folder):
                continue
            if not os.path.exists(os.path.join(folder, "run.json")):
                # Thu muc co ma khong co run.json: bo do loi giua chung (vd luu that bai) - van tinh la khong dat.
                rows.append({"brief": brief_id, "n": n, "pass": False, "outcome": "runner-error", "failed": ["khong co run.json"],
                             "severity": {"critical": 0, "major": 0, "minor": 0}, "codes": [], "rounds": None,
                             "billable": None, "seconds": None, "toolFailures": None, "confirmations": 0})
                continue
            brief = json.load(open(os.path.join(folder, "brief.json"), encoding="utf-8"))
            # Cham theo briefs.json hien tai (tieu chi co the da sua sau lan chay); brief.json chi la ban luc chay.
            brief = current.get(brief["id"], brief)
            run = json.load(open(os.path.join(folder, "run.json"), encoding="utf-8"))
            path = os.path.join(folder, "result.docx" if brief["app"] == "word" else "result.xlsx")
            if not os.path.exists(path):
                report, failed = {"issues": [{"severity": "critical", "code": "NO_FILE", "detail": ""}]}, ["khong co tep"]
            elif brief["app"] == "word":
                report, failed = score_word(path, brief, not args.no_app)
            else:
                report, failed = score_excel(path, brief, not args.no_app)
            with open(os.path.join(folder, "check.json"), "w", encoding="utf-8") as handle:
                json.dump(report, handle, ensure_ascii=False, indent=1)
            # Mat chu co san (mustKeep / o giu nguyen) la mat du lieu nguoi dung: critical.
            lost = [item for item in failed if item.startswith(("mat chu cu", "o "))]
            if lost:
                report["issues"].append({"severity": "critical", "code": "CONTENT_LOST", "detail": "; ".join(lost)[:300]})
            severity = {s: sum(1 for i in report["issues"] if i["severity"] == s) for s in ("critical", "major", "minor")}
            ok = run.get("outcome") == "run.completed" and not severity["critical"] and not failed
            rows.append({"brief": brief_id, "n": n, "pass": ok, "outcome": run.get("outcome"), "failed": failed,
                         "severity": severity, "codes": sorted({i["code"] for i in report["issues"]}),
                         "rounds": run.get("rounds"), "billable": run.get("billable"), "seconds": run.get("seconds"),
                         "toolFailures": run.get("failures"), "confirmations": len(run.get("confirmations") or [])})
    print("| brief | lan | dat | ket thuc | crit/maj/min | khong dat | vong | billable | giay | lenh loi | xac nhan |")
    print("|---|---|---|---|---|---|---|---|---|---|---|")
    for row in rows:
        print("| %s | %s | %s | %s | %d/%d/%d | %s | %s | %s | %s | %s | %s |" % (
            row["brief"], row["n"], "x" if row["pass"] else "", (row["outcome"] or "").replace("run.", ""),
            row["severity"]["critical"], row["severity"]["major"], row["severity"]["minor"], "; ".join(row["failed"])[:160],
            row["rounds"], row["billable"], row["seconds"], row["toolFailures"], row["confirmations"]))
    passed = sum(1 for row in rows if row["pass"])
    print("\nDat %d/%d" % (passed, len(rows)))
    if args.json:
        with open(args.json, "w", encoding="utf-8") as handle:
            json.dump(rows, handle, ensure_ascii=False, indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
