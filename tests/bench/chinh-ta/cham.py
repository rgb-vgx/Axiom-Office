"""Cham ket qua soat chinh ta theo bo de cases.json.

    python3 tests/bench/chinh-ta/cham.py tests/bench/chinh-ta/cases.json outputs.json [--json report.json]

outputs.json: {"<id>": "<van ban sau khi soat>"}. Dac ta: test_cham.py.
"""
from __future__ import annotations

import argparse
import difflib
import json
import re
import sys


def tokenize(text):
    return re.findall(r"\s+|\S+", text)


def expected_text(case):
    text = case["text"]
    for fix in case["fixes"]:
        wrong = fix["wrong"]
        count = text.count(wrong)
        if count != 1:
            raise ValueError(f"{case['id']}: '{wrong}' xuat hien {count} lan, can dung 1")
    for fix in case["fixes"]:
        text = text.replace(fix["wrong"], fix["right"])
    return text


def _offsets(tokens):
    out = [0]
    for token in tokens:
        out.append(out[-1] + len(token))
    return out


def _hunks(a, offsets, b):
    # Moi opcode khac "equal" la mot hunk tren khoang token cua a.
    sm = difflib.SequenceMatcher(None, a, b, autojunk=False)
    hunks = []
    for tag, i1, i2, j1, j2 in sm.get_opcodes():
        if tag == "equal":
            continue
        hunks.append({
            "i1": i1,
            "i2": i2,
            "start": offsets[i1],
            "end": offsets[i2],
            "repl": "".join(b[j1:j2]),
        })
    return hunks


def _belongs(hunk, span):
    start, end = span
    if hunk["start"] == hunk["end"]:
        # Chen thuan: chi tinh khi nam han trong khoang loi.
        return start < hunk["start"] < end
    return hunk["start"] < end and hunk["end"] > start


def _render(tokens, lo, hi, hunks):
    by_start = {h["i1"]: h for h in hunks if lo <= h["i1"] < hi}
    parts = []
    i = lo
    while i < hi:
        hunk = by_start.get(i)
        if hunk is None:
            parts.append(tokens[i])
            i += 1
        else:
            parts.append(hunk["repl"])
            i = max(hunk["i2"], i + 1)
    return "".join(parts)


def score_case(case, output):
    text = case["text"]
    a = tokenize(text)
    offsets = _offsets(a)
    expected_hunks = _hunks(a, offsets, tokenize(expected_text(case)))
    output_hunks = _hunks(a, offsets, tokenize(output))

    correct = wrong = missed = false = 0
    wrong_fixes = []
    missed_fixes = []
    by_kind = {}
    matched = set()

    for fix in case["fixes"]:
        kind = fix["kind"]
        row = by_kind.setdefault(kind, {"expected": 0, "correct": 0})
        row["expected"] += 1

        start = text.index(fix["wrong"])
        span = (start, start + len(fix["wrong"]))
        exp = [h for h in expected_hunks if _belongs(h, span)]
        out = [h for h in output_hunks if _belongs(h, span)]
        matched.update(id(h) for h in out)

        if not out:
            missed += 1
            missed_fixes.append(fix)
            continue

        both = exp + out
        lo = min(h["i1"] for h in both)
        hi = max(h["i2"] for h in both)
        if hi == lo:
            hi = min(lo + 1, len(a))
        if _render(a, lo, hi, exp) == _render(a, lo, hi, out):
            correct += 1
            row["correct"] += 1
        else:
            wrong += 1
            got = "".join(h["repl"] for h in sorted(out, key=lambda h: h["i1"]))
            wrong_fixes.append(dict(fix, got=got))

    false_edits = []
    for hunk in output_hunks:
        if id(hunk) in matched:
            continue
        false += 1
        false_edits.append({
            "before": "".join(a[hunk["i1"]:hunk["i2"]]),
            "after": hunk["repl"],
        })

    return {
        "id": case["id"],
        "correct": correct,
        "wrong": wrong,
        "missed": missed,
        "false": false,
        "wrong_fixes": wrong_fixes,
        "missed_fixes": missed_fixes,
        "false_edits": false_edits,
        "by_kind": by_kind,
    }


def score(cases, outputs):
    ids = [case["id"] for case in cases]
    known = set(ids)
    for key in outputs:
        if key not in known:
            raise ValueError(f"output khong co ca: {key}")

    results = []
    missing = []
    totals = {"expected": 0, "correct": 0, "wrong": 0, "missed": 0, "false": 0}
    for case in cases:
        if case["id"] not in outputs:
            missing.append(case["id"])
            continue
        result = score_case(case, outputs[case["id"]])
        results.append(result)
        totals["expected"] += len(case["fixes"])
        for key in ("correct", "wrong", "missed", "false"):
            totals[key] += result[key]

    recall = totals["correct"] / totals["expected"] if totals["expected"] else None
    denom = totals["correct"] + totals["wrong"] + totals["false"]
    precision = totals["correct"] / denom if denom else None
    return {
        "totals": totals,
        "cases": results,
        "missing_outputs": missing,
        "recall": recall,
        "precision": precision,
    }


def _fmt(value):
    return "n/a" if value is None else f"{value:.3f}"


def _kind_totals(results):
    agg = {}
    for result in results:
        for kind, row in result["by_kind"].items():
            total = agg.setdefault(kind, {"expected": 0, "correct": 0})
            total["expected"] += row["expected"]
            total["correct"] += row["correct"]
    return agg


def main(argv=None):
    parser = argparse.ArgumentParser(description="Cham ket qua soat chinh ta.")
    parser.add_argument("cases", help="tep cases.json")
    parser.add_argument("outputs", help="tep outputs.json")
    parser.add_argument("--json", dest="json_report", help="ghi bao cao day du ra tep JSON")
    args = parser.parse_args(argv)

    with open(args.cases, encoding="utf-8") as f:
        data = json.load(f)
    with open(args.outputs, encoding="utf-8") as f:
        outputs = json.load(f)

    cases = data["cases"]
    report = score(cases, outputs)
    totals = report["totals"]

    print(f"Cham {len(report['cases'])}/{len(cases)} ca ({len(report['missing_outputs'])} ca thieu output).")
    print(f"  expected={totals['expected']} correct={totals['correct']} "
          f"wrong={totals['wrong']} missed={totals['missed']} false={totals['false']}")
    print(f"  recall={_fmt(report['recall'])} precision={_fmt(report['precision'])}")

    print("  recall theo loai:")
    for kind, row in sorted(_kind_totals(report["cases"]).items()):
        rate = row["correct"] / row["expected"] if row["expected"] else None
        print(f"    {kind}: {row['correct']}/{row['expected']} = {_fmt(rate)}")

    for result in report["cases"]:
        cid = result["id"]
        for fix in result["wrong_fixes"]:
            print(f"  {cid}: wrong {fix['wrong']} -> {fix['right']} (got {fix['got']!r})")
        for fix in result["missed_fixes"]:
            print(f"  {cid}: missed {fix['wrong']} -> {fix['right']}")
        for edit in result["false_edits"]:
            print(f"  {cid}: false {edit['before']!r} -> {edit['after']!r}")

    if args.json_report:
        with open(args.json_report, "w", encoding="utf-8") as f:
            json.dump(report, f, ensure_ascii=False, indent=2)

    return 0


if __name__ == "__main__":
    sys.exit(main())
