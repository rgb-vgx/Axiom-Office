"""Cham mot tep .xlsx theo tieu chi cau truc va tinh dung (de B va cac ca Excel cua bo do van phong).

    python tests/bench/office/check_xlsx.py <result.xlsx> [--sheets Dashboard,Project_Data,...] [--recalc] [--json out.json]

Chi dung thu vien chuan: doc XML trong goi .xlsx (sheet, style, table, chart, validation, dinh dang co dieu kien).
--recalc mo tep bang Excel (COM qua PowerShell, khong luu): doi mot o nhap lieu roi tinh lai, xem o tong hop co doi
theo khong - doc gia tri khong phan biet duoc cong thuc song voi so go cung (tests/bench/README.md, "dau vao con song").

Muc do: critical = sai/mat du lieu, o loi cong thuc, so tong hop go cung; major = thieu chuc nang chinh (table/loc,
freeze, validation, CF, chart); minor = dinh dang so/ngay khong nhat quan, thieu ghi chu gia dinh.
"""
from __future__ import annotations

import argparse
import json
import os
import posixpath
import re
import subprocess
import sys
import zipfile
import xml.etree.ElementTree as ET

S = "{http://schemas.openxmlformats.org/spreadsheetml/2006/main}"
REL = "{http://schemas.openxmlformats.org/package/2006/relationships}"
RID = "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}id"
C = "{http://schemas.openxmlformats.org/drawingml/2006/chart}"

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

# numFmtId dung san cua Excel: phan tram 9/10, ngay 14-17/22, so co phan cach 3/4/37-40.
BUILTIN_PERCENT = {9, 10}
BUILTIN_DATE = {14, 15, 16, 17, 22}
BUILTIN_GROUPED = {3, 4, 37, 38, 39, 40}


def rels(package: zipfile.ZipFile, part: str) -> dict:
    folder, name = posixpath.split(part)
    path = posixpath.join(folder, "_rels", name + ".rels")
    if path not in package.namelist():
        return {}
    root = ET.fromstring(package.read(path))
    return {rel.get("Id"): posixpath.normpath(posixpath.join(folder, rel.get("Target"))) for rel in root.iter(REL + "Relationship")}


def format_kind(code: str, fmt_id: int) -> str:
    lowered = code.lower()
    if fmt_id in BUILTIN_PERCENT or "%" in code:
        return "percent"
    if fmt_id in BUILTIN_DATE or (re.search(r"[dmy]", re.sub(r'"[^"]*"|\[[^\]]*\]', "", lowered)) and "0" not in lowered):
        return "date"
    if fmt_id in BUILTIN_GROUPED or "#,##" in code:
        return "grouped"
    if fmt_id == 0:
        return "general"
    return "number"


def load_styles(package: zipfile.ZipFile) -> tuple[list, list]:
    """(kieu so cua tung xf, mau nen cua tung xf)."""
    root = ET.fromstring(package.read("xl/styles.xml"))
    codes = {int(f.get("numFmtId")): f.get("formatCode", "") for f in root.iter(S + "numFmt")}
    fills = []
    fills_node = root.find(S + "fills")
    for fill in (fills_node if fills_node is not None else []):
        fg = fill.find(S + "patternFill/" + S + "fgColor")
        fills.append((fg.get("rgb") or ("theme%s" % fg.get("theme"))) if fg is not None else "")
    kinds, backgrounds = [], []
    xfs = root.find(S + "cellXfs")
    for xf in (xfs if xfs is not None else []):
        fmt_id = int(xf.get("numFmtId", "0"))
        kinds.append(format_kind(codes.get(fmt_id, ""), fmt_id))
        fill_id = int(xf.get("fillId", "0"))
        backgrounds.append(fills[fill_id] if fill_id < len(fills) else "")
    return kinds, backgrounds


def column_of(ref: str) -> str:
    return re.match(r"[A-Z]+", ref).group(0)


def check_package(path: str, required: list[str]) -> dict:
    report: dict = {"file": os.path.basename(path), "issues": [], "sheets": {}}

    def issue(severity: str, code: str, detail: str) -> None:
        report["issues"].append({"severity": severity, "code": code, "detail": detail})

    try:
        package = zipfile.ZipFile(path)
        workbook = ET.fromstring(package.read("xl/workbook.xml"))
    except (OSError, KeyError, zipfile.BadZipFile, ET.ParseError) as error:
        issue("critical", "BROKEN_FILE", str(error))
        return report
    targets = rels(package, "xl/workbook.xml")
    shared = []
    if "xl/sharedStrings.xml" in package.namelist():
        for item in ET.fromstring(package.read("xl/sharedStrings.xml")).iter(S + "si"):
            shared.append("".join(t.text or "" for t in item.iter(S + "t")))
    kinds, backgrounds = load_styles(package)
    defined = [d.text or "" for d in workbook.iter(S + "definedName")]
    all_text = []

    names = []
    for sheet in workbook.iter(S + "sheet"):
        name = sheet.get("name")
        names.append(name)
        part = targets.get(sheet.get(RID))
        root = ET.fromstring(package.read(part))
        info = {"formulas": 0, "constants": 0, "text": 0, "errors": [], "frozen": False, "autoFilter": False,
                "tables": 0, "validations": [], "conditional": [], "charts": 0, "chartRefs": [], "inputFills": set(),
                "formulaFills": set(), "formatKinds": {}, "formulaSamples": []}
        for cell in root.iter(S + "c"):
            ref, kind_attr = cell.get("r"), cell.get("t")
            formula, value = cell.find(S + "f"), cell.find(S + "v")
            style = int(cell.get("s", "0"))
            if kind_attr == "e" and value is not None:
                info["errors"].append("%s=%s" % (ref, value.text))
            if formula is not None:
                info["formulas"] += 1
                if formula.text and len(info["formulaSamples"]) < 6:
                    info["formulaSamples"].append("%s: =%s" % (ref, formula.text[:80]))
                if style < len(backgrounds):
                    info["formulaFills"].add(backgrounds[style])
            elif value is not None:
                if kind_attr in ("s", "inlineStr", "str"):
                    info["text"] += 1
                    if kind_attr == "s" and value.text and int(value.text) < len(shared):
                        all_text.append(shared[int(value.text)])
                else:
                    info["constants"] += 1
                    if style < len(backgrounds):
                        info["inputFills"].add(backgrounds[style])
            if value is not None and kind_attr not in ("s", "inlineStr", "str", "e") and style < len(kinds):
                column = column_of(ref)
                info["formatKinds"].setdefault(column, set()).add(kinds[style])
            for t in cell.iter(S + "t"):
                all_text.append(t.text or "")
        info["frozen"] = any(pane.get("state") in ("frozen", "frozenSplit") for pane in root.iter(S + "pane"))
        info["autoFilter"] = root.find(S + "autoFilter") is not None
        for validation in root.iter(S + "dataValidation"):
            formula = validation.find(S + "formula1")
            info["validations"].append({"type": validation.get("type"), "sqref": validation.get("sqref"),
                                        "formula": formula.text if formula is not None else ""})
        for block in root.iter(S + "conditionalFormatting"):
            for rule in block.iter(S + "cfRule"):
                info["conditional"].append({"sqref": block.get("sqref"), "type": rule.get("type"),
                                            "formula": "; ".join(f.text or "" for f in rule.iter(S + "formula"))})
        sheet_rels = rels(package, part)
        for table in root.iter(S + "tablePart"):
            if sheet_rels.get(table.get(RID)):
                info["tables"] += 1
        for drawing in root.iter(S + "drawing"):
            drawing_part = sheet_rels.get(drawing.get(RID))
            for chart_part in rels(package, drawing_part).values() if drawing_part else []:
                if "/charts/chart" in chart_part and chart_part in package.namelist():
                    info["charts"] += 1
                    chart = ET.fromstring(package.read(chart_part))
                    info["chartRefs"] += [f.text or "" for f in chart.iter(C + "f")][:12]
        info["inputFills"] = sorted(info["inputFills"])
        info["formulaFills"] = sorted(info["formulaFills"])
        info["formatKinds"] = {column: sorted(kinds_) for column, kinds_ in sorted(info["formatKinds"].items())}
        report["sheets"][name] = info

    report["sheetNames"] = names
    report["definedNames"] = defined[:10]
    missing = [name for name in required if name not in names]
    if missing:
        issue("critical", "MISSING_SHEETS", "thieu sheet: %s" % missing)
    for name, info in report["sheets"].items():
        if info["errors"]:
            issue("critical", "FORMULA_ERRORS", "%s: %s" % (name, info["errors"][:8]))
        mixed = {column: kinds_ for column, kinds_ in info["formatKinds"].items()
                 if len(set(kinds_) - {"general"}) > 1}
        if mixed:
            issue("minor", "MIXED_NUMBER_FORMAT", "%s: %s" % (name, mixed))
    dashboard = report["sheets"].get("Dashboard")
    if dashboard is not None:
        if dashboard["formulas"] == 0:
            issue("critical", "DASHBOARD_HARDCODED", "Dashboard khong co cong thuc nao (%d so go cung)" % dashboard["constants"])
        elif dashboard["constants"] > dashboard["formulas"]:
            issue("major", "DASHBOARD_MOSTLY_CONSTANTS", "Dashboard %d so go cung / %d cong thuc" % (dashboard["constants"], dashboard["formulas"]))
        if dashboard["charts"] == 0:
            issue("major", "NO_CHART", "Dashboard khong co bieu do")
    project = report["sheets"].get("Project_Data")
    if project is not None:
        if not any("Monthly_Budget" in sample for sample in project["formulaSamples"]) and project["formulas"] == 0:
            issue("critical", "ACTUAL_COST_NOT_LINKED", "Project_Data khong co cong thuc (chi phi thuc te phai tong tu Monthly_Budget)")
        if not (project["tables"] or project["autoFilter"]):
            issue("major", "NO_TABLE_OR_FILTER", "Project_Data khong co Excel Table hay bo loc")
        if not project["frozen"]:
            issue("major", "NO_FREEZE_PANES", "Project_Data khong co freeze panes")
        lists = [v for v in project["validations"] if v["type"] == "list"]
        if not lists:
            issue("major", "NO_STATUS_VALIDATION", "Project_Data khong co data validation dang list")
        elif not any("Config" in (v["formula"] or "") or (v["formula"] or "").lstrip("=") in [d for d in defined] for v in lists):
            issue("minor", "VALIDATION_NOT_FROM_CONFIG", "validation khong tham chieu Config: %s" % lists[:2])
    conditional = [rule for info in report["sheets"].values() for rule in info["conditional"]]
    if not conditional:
        issue("major", "NO_CONDITIONAL_FORMAT", "khong co dinh dang co dieu kien")
    elif not any("Config" in rule["formula"] or "TODAY" in rule["formula"].upper() for rule in conditional):
        issue("minor", "CF_NOT_DATA_DRIVEN", "dinh dang co dieu kien khong doc nguong Config/ngay: %s" % conditional[:3])
    inputs = set().union(*[set(info["inputFills"]) for info in report["sheets"].values()]) if report["sheets"] else set()
    formulas = set().union(*[set(info["formulaFills"]) for info in report["sheets"].values()]) if report["sheets"] else set()
    report["inputVsFormulaFillsDiffer"] = bool(inputs - formulas) or bool(formulas - inputs)
    if not report["inputVsFormulaFillsDiffer"]:
        issue("minor", "INPUT_NOT_DISTINGUISHED", "o nhap lieu va o cong thuc cung mau nen")
    report["assumptionMarked"] = bool(re.search(r"giả định|giả thiết|minh họa|minh hoạ|dữ liệu mẫu", " ".join(all_text), re.I))
    if not report["assumptionMarked"]:
        issue("minor", "ASSUMPTION_NOT_MARKED", "khong thay ghi chu 'gia dinh/du lieu mau'")
    return report


# Doi mot o nhap lieu (so, khong cong thuc, khong phai ngay - ngay cung la so > 1000) o sheet nguon roi tinh lai: o cong thuc cua Dashboard phai doi theo.
RECALC_PS = r"""
$ErrorActionPreference = 'Stop'
$path = $args[0]; $source = $args[1]; $target = $args[2]
$excel = New-Object -ComObject Excel.Application
$excel.Visible = $false; $excel.DisplayAlerts = $false
try {
  $wb = $excel.Workbooks.Open($path, 0, $true)
  $src = $wb.Worksheets.Item($source); $dst = $wb.Worksheets.Item($target)
  $cell = $null
  foreach ($c in $src.UsedRange.Cells) {
    if (-not $c.HasFormula -and ($c.Value2 -is [double]) -and $c.Value2 -gt 1000 -and -not ([string]$c.NumberFormat -match '[dmy]')) { $cell = $c; break }
  }
  $before = @{}
  foreach ($c in $dst.UsedRange.Cells) { if ($c.HasFormula) { $before[$c.Address(0,0)] = [string]$c.Value2 } }
  if ($cell -eq $null) { [pscustomobject]@{ changedInput = ''; formulas = $before.Count; moved = 0 } | ConvertTo-Json -Compress; return }
  $address = $cell.Address(0,0)
  $cell.Value2 = $cell.Value2 + 1000000
  $excel.CalculateFull()
  $moved = 0
  foreach ($c in $dst.UsedRange.Cells) { if ($c.HasFormula -and $before[$c.Address(0,0)] -ne [string]$c.Value2) { $moved++ } }
  [pscustomobject]@{ changedInput = "$source!$address"; formulas = $before.Count; moved = $moved } | ConvertTo-Json -Compress
  $wb.Close($false)
} finally { $excel.Quit() }
"""


def check_recalc(path: str, report: dict, source: str, target: str) -> None:
    script = os.path.join(os.environ.get("TEMP", "."), "axiom_xlsx_recalc.ps1")
    with open(script, "w", encoding="utf-8-sig") as handle:
        handle.write(RECALC_PS)
    done = subprocess.run(["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script,
                           os.path.abspath(path), source, target],
                          capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=600)
    try:
        result = json.loads(done.stdout.strip().splitlines()[-1])
    except (IndexError, json.JSONDecodeError):
        report["issues"].append({"severity": "minor", "code": "RECALC_UNREADABLE", "detail": (done.stderr or done.stdout)[-300:]})
        return
    report["recalc"] = result
    if not result.get("changedInput"):
        report["issues"].append({"severity": "major", "code": "NO_INPUT_CELL", "detail": "%s khong co o so nhap lieu" % source})
    elif result.get("moved", 0) == 0:
        report["issues"].append({"severity": "critical", "code": "DASHBOARD_NOT_LIVE",
                                 "detail": "doi %s, %s khong o nao doi (%d cong thuc)" % (result["changedInput"], target, result.get("formulas", 0))})


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("path")
    parser.add_argument("--sheets", default="Dashboard,Project_Data,Monthly_Budget,Config")
    parser.add_argument("--recalc", action="store_true", help="doi o nhap lieu bang Excel (COM) va xem tong hop doi theo")
    parser.add_argument("--recalc-source", default="Monthly_Budget")
    parser.add_argument("--recalc-target", default="Dashboard")
    parser.add_argument("--json", default="")
    args = parser.parse_args()
    report = check_package(args.path, [s for s in args.sheets.split(",") if s])
    if args.recalc and args.recalc_source in report.get("sheetNames", []) and args.recalc_target in report.get("sheetNames", []):
        check_recalc(args.path, report, args.recalc_source, args.recalc_target)
    text = json.dumps(report, ensure_ascii=False, indent=1)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as handle:
            handle.write(text)
    print(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
