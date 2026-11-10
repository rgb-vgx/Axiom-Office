"""Sinh báo cáo Markdown từ runs.jsonl (paired control/treatment).

    python tests/bench/campaign/report.py runs.jsonl --out report.md
    python tests/bench/campaign/report.py runs.jsonl --success-level S2 --bootstrap 3000

Mỗi dòng runs.jsonl là một lần chạy; xem records.py để biết schema. Báo cáo cố ý không
tự quyết định thay người đọc: chỉ đưa số paired, CI và một gợi ý KEEP/REJECT/HOLD.
"""
from __future__ import annotations

import argparse
import datetime
import sys
from pathlib import Path

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

try:
    from .records import NUMERIC_METRICS, load_jsonl, pair_records
    from .stats import decision_hint, group_by_variant, median_reduction, variant_summary
except ImportError:  # chạy trực tiếp: python tests/bench/campaign/report.py
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    from records import NUMERIC_METRICS, load_jsonl, pair_records
    from stats import decision_hint, group_by_variant, median_reduction, variant_summary


def percent(value: float | None, digits: int = 1) -> str:
    if value is None:
        return "—"
    return f"{value * 100:.{digits}f}%"


def number(value: float | None, digits: int = 0) -> str:
    if value is None:
        return "—"
    return f"{value:,.{digits}f}"


def delta(current: float | None, base: float | None, digits: int = 1) -> str:
    if current is None or base is None:
        return "—"
    value = (current - base) * 100
    return f"{value:+.{digits}f} pp"


def metric_name(metric: str) -> str:
    return {
        "billable": "Billable token",
        "input_tokens": "Input token",
        "cached_tokens": "Cached token",
        "output_tokens": "Output token",
        "tool_calls": "Tool call",
        "invalid_tool_calls": "Tool call sai",
        "duplicate_reads": "Đọc trùng",
        "rounds": "Số vòng",
        "wall_seconds": "Wall time",
        "read_result_calls": "read_result",
        "unnecessary_read_result": "read_result thừa",
    }.get(metric, metric)


def overall_table(groups: dict, summaries: dict) -> list[str]:
    lines = [
        "| Nhánh | Cặp | Task | Control | Treatment | Δ success | McNemar p | Cost/control | Cost/treatment | Giảm billable (95% CI) | Gợi ý |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    for variant in sorted(groups):
        summary = summaries[variant]
        control = summary["control_success"]
        treatment = summary["treatment_success"]
        ci = summary["billable_ci"]
        ci_text = "—"
        if ci is not None:
            ci_text = f"{percent(ci[0])} … {percent(ci[1])}"
        mcnemar = summary["mcnemar"]
        lines.append(
            f"| {variant} | {summary['pairs']} | {summary['tasks']} | {percent(control)} | {percent(treatment)} | "
            f"{delta(treatment, control)} | {mcnemar['p_value']:.3f} | {number(summary['control_cost'])} | "
            f"{number(summary['treatment_cost'])} | {percent(summary['reduction']['billable']['median'])} "
            f"({ci_text}) | {decision_hint(summary)} |"
        )
    return lines


def metric_table(pairs: list, success_level: str) -> list[str]:
    lines = [
        "| Metric | Cặp dùng được | Median giảm |",
        "|---|---:|---:|",
    ]
    for metric in NUMERIC_METRICS:
        result = median_reduction(pairs, metric, success_level)
        if result["n"] == 0:
            continue
        lines.append(f"| {metric_name(metric)} | {result['n']} | {percent(result['median'])} |")
    return lines


def task_table(pairs: list, success_level: str) -> list[str]:
    lines = [
        "| Task | Nhánh | Control | Treatment | Billable Δ | Tool call Δ | Vòng Δ | Lỗi treatment |",
        "|---|---|---|---|---:|---:|---:|---|",
    ]
    for pair in pairs:
        control, treatment = pair.control, pair.treatment
        failure = treatment.failure or "—"
        lines.append(
            f"| {pair.task_id} | {pair.variant} | {control.success} | {treatment.success} | "
            f"{percent(_ratio_delta(control.billable, treatment.billable))} | "
            f"{percent(_ratio_delta(control.tool_calls, treatment.tool_calls))} | "
            f"{percent(_ratio_delta(control.rounds, treatment.rounds))} | {failure} |"
        )
    return lines


def _ratio_delta(control: float, treatment: float) -> float | None:
    if not control:
        return None
    return 1.0 - float(treatment) / float(control)


def render(path: str, success_level: str, bootstrap: int) -> str:
    records = load_jsonl(path)
    pairs = pair_records(records)
    groups = group_by_variant(pairs)
    summaries = {variant: variant_summary(items, success_level, bootstrap) for variant, items in groups.items()}
    lines = [
        f"# Báo cáo benchmark paired — {Path(path).name}",
        "",
        f"- Sinh lúc: {datetime.datetime.now().strftime('%Y-%m-%d %H:%M')}",
        f"- Đơn vị: {len(pairs)} cặp ({len(records)} lần chạy), success level **{success_level}**",
        f"- Bootstrap: {bootstrap} lần, resample theo cặp",
        "",
        "## Tổng hợp theo nhánh",
        "",
        *overall_table(groups, summaries),
        "",
        "## Chi tiết từng nhánh",
    ]
    for variant in sorted(groups):
        summary = summaries[variant]
        items = groups[variant]
        lines += [
            "",
            f"### {variant}",
            "",
            f"- Control success: **{percent(summary['control_success'])}** · "
            f"Treatment: **{percent(summary['treatment_success'])}** · "
            f"McNemar p = **{summary['mcnemar']['p_value']:.3f}** "
            f"(both pass {summary['mcnemar']['both_pass']}, both fail {summary['mcnemar']['both_fail']}, "
            f"control-only {summary['mcnemar']['control_only']}, treatment-only {summary['mcnemar']['treatment_only']})",
            f"- Cost/success: control **{number(summary['control_cost'])}** · treatment "
            f"**{number(summary['treatment_cost'])}** billable token",
            f"- Lỗi treatment: {summary['treatment_failures'] or '—'}",
            "",
            *metric_table(items, success_level),
            "",
            "### Theo task",
            "",
            *task_table(items, success_level),
        ]
    lines += [
        "",
        "## Cách đọc",
        "",
        "- `Δ success` là treatment − control, đơn vị điểm phần trăm. McNemar exact chỉ dùng các cặp bất đồng.",
        "- Giảm billable/tool call/vòng tính theo **median của tỷ lệ từng cặp**, chỉ trên cặp cả hai thành công.",
        "- CI bootstrap resample theo cặp; CI quá rộng thì kết luận là HOLD, không phải KEEP.",
        "- Gợi ý KEEP/REJECT/HOLD chỉ xét success + billable; phải xem thêm invalid tool call, đọc trùng, "
        "recovery và lỗi treatment trước khi quyết định.",
        "- Số n nhỏ: 3 cặp chỉ đủ phát hiện regression lớn, không đủ để tuyên bố cải tiến thắng.",
        "",
    ]
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("runs", help="file JSONL các lần chạy")
    parser.add_argument("--out", help="ghi Markdown ra file (mặc định in stdout)")
    parser.add_argument("--success-level", default="S3", choices=["S0", "S1", "S2", "S3"],
                        help="mức tối thiểu tính là thành công (mặc định S3)")
    parser.add_argument("--bootstrap", type=int, default=5000, help="số lần bootstrap (mặc định 5000)")
    args = parser.parse_args(argv)
    text = render(args.runs, args.success_level, args.bootstrap)
    if args.out:
        Path(args.out).write_text(text, encoding="utf-8")
        print(f"đã ghi {args.out}")
    else:
        print(text)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
