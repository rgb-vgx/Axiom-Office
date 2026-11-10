"""Thống kê paired cho benchmark: McNemar, median giảm theo cặp, bootstrap CI.

Chỉ dùng thư viện chuẩn. Nguyên tắc (theo docs/research/gpt-advice.md):
- Success là nhị phân; so paired bằng McNemar (exact khi n nhỏ).
- Token/time lệch mạnh: báo median của tỷ lệ theo cặp, không lấy trung bình cộng.
- Bootstrap resample theo CẶP, không resample từng tool call.
- Cost per successful task lấy tổng billable / số task thành công, task thất bại vẫn vào tử số.
"""
from __future__ import annotations

import math
import random
import statistics
from dataclasses import dataclass

try:
    from .records import NUMERIC_METRICS, Pair, RunRecord
except ImportError:  # chạy trực tiếp từ thư mục campaign
    from records import NUMERIC_METRICS, Pair, RunRecord


def mcnemar_exact(pairs: list[Pair], success_level: str = "S3") -> dict:
    """Đếm 4 ô paired và trả p-value exact hai phía.

    b = control thành công, treatment thất bại (treatment làm hỏng)
    c = control thất bại, treatment thành công (treatment cứu được)
    """
    both_pass = both_fail = control_only = treatment_only = 0
    for pair in pairs:
        control = pair.control.successful(success_level)
        treatment = pair.treatment.successful(success_level)
        if control and treatment:
            both_pass += 1
        elif not control and not treatment:
            both_fail += 1
        elif control:
            control_only += 1
        else:
            treatment_only += 1
    discordant = control_only + treatment_only
    if discordant == 0:
        return {
            "both_pass": both_pass, "both_fail": both_fail,
            "control_only": control_only, "treatment_only": treatment_only,
            "discordant": 0, "p_value": 1.0,
        }
    tail = sum(math.comb(discordant, k) for k in range(0, min(control_only, treatment_only) + 1))
    p_value = min(1.0, 2.0 * tail / (2 ** discordant))
    return {
        "both_pass": both_pass, "both_fail": both_fail,
        "control_only": control_only, "treatment_only": treatment_only,
        "discordant": discordant, "p_value": p_value,
    }


def paired_ratios(pairs: list[Pair], metric: str, success_level: str = "S3",
                  require_success: bool = True) -> list[float]:
    """Tỷ lệ giảm của treatment so với control cho từng cặp: 1 - t/c.

    Mặc định chỉ lấy cặp cả hai đều thành công: so lượng token để làm CÙNG một việc đã xong.
    Bỏ cặp có giá trị control <= 0 để tránh chia cho 0.
    """
    ratios: list[float] = []
    for pair in pairs:
        if require_success and not (pair.control.successful(success_level) and pair.treatment.successful(success_level)):
            continue
        control = getattr(pair.control, metric, 0)
        treatment = getattr(pair.treatment, metric, 0)
        if control and control > 0:
            ratios.append(1.0 - (float(treatment) / float(control)))
    return ratios


def median_reduction(pairs: list[Pair], metric: str, success_level: str = "S3",
                     require_success: bool = True) -> dict:
    ratios = paired_ratios(pairs, metric, success_level, require_success)
    if not ratios:
        return {"n": 0, "median": None}
    return {"n": len(ratios), "median": statistics.median(ratios)}


def bootstrap_median_ci(values: list[float], iterations: int = 5000, seed: int = 20261111,
                        confidence: float = 0.95) -> tuple[float, float] | None:
    if not values:
        return None
    rng = random.Random(seed)
    medians = []
    size = len(values)
    for _ in range(iterations):
        sample = [values[rng.randrange(size)] for _ in range(size)]
        medians.append(statistics.median(sample))
    medians.sort()
    alpha = (1.0 - confidence) / 2.0
    low = medians[int(alpha * (len(medians) - 1))]
    high = medians[int((1.0 - alpha) * (len(medians) - 1))]
    return low, high


def bootstrap_reduction_ci(pairs: list[Pair], metric: str, success_level: str = "S3",
                           iterations: int = 5000, seed: int = 20261111) -> tuple[float, float] | None:
    """CI của median reduction, resample theo cặp (đúng đơn vị độc lập)."""
    eligible = [pair for pair in pairs
                if pair.control.successful(success_level) and pair.treatment.successful(success_level)
                and getattr(pair.control, metric, 0) > 0]
    if not eligible:
        return None
    rng = random.Random(seed)
    medians = []
    for _ in range(iterations):
        sample = [eligible[rng.randrange(len(eligible))] for _ in range(len(eligible))]
        ratios = []
        for pair in sample:
            control = float(getattr(pair.control, metric, 0))
            treatment = float(getattr(pair.treatment, metric, 0))
            if control > 0:
                ratios.append(1.0 - treatment / control)
        if ratios:
            medians.append(statistics.median(ratios))
    if not medians:
        return None
    medians.sort()
    low = medians[int(0.025 * (len(medians) - 1))]
    high = medians[int(0.975 * (len(medians) - 1))]
    return low, high


def success_rate(records: list[RunRecord], success_level: str = "S3") -> float | None:
    if not records:
        return None
    return sum(1 for record in records if record.successful(success_level)) / len(records)


def cost_per_success(records: list[RunRecord], success_level: str = "S3") -> float | None:
    """Tổng billable / số task thành công; task thất bại vẫn nằm ở tử số."""
    successes = sum(1 for record in records if record.successful(success_level))
    if successes == 0:
        return None
    return sum(record.billable for record in records) / successes


def median_metric(records: list[RunRecord], metric: str) -> float | None:
    values = [float(getattr(record, metric, 0)) for record in records if getattr(record, metric, 0) is not None]
    return statistics.median(values) if values else None


def failure_counts(records: list[RunRecord]) -> dict[str, int]:
    counts: dict[str, int] = {}
    for record in records:
        key = record.failure or "none"
        counts[key] = counts.get(key, 0) + 1
    return dict(sorted(counts.items(), key=lambda item: (-item[1], item[0])))


def group_by_variant(pairs: list[Pair]) -> dict[str, list[Pair]]:
    groups: dict[str, list[Pair]] = {}
    for pair in pairs:
        groups.setdefault(pair.variant, []).append(pair)
    return dict(sorted(groups.items()))


def variant_summary(pairs: list[Pair], success_level: str = "S3", bootstrap: int = 5000) -> dict:
    controls = [pair.control for pair in pairs]
    treatments = [pair.treatment for pair in pairs]
    reduction = {}
    for metric in NUMERIC_METRICS:
        reduction[metric] = median_reduction(pairs, metric, success_level)
    billable_ci = bootstrap_reduction_ci(pairs, "billable", success_level, iterations=bootstrap)
    return {
        "pairs": len(pairs),
        "tasks": len({pair.task_id for pair in pairs}),
        "control_success": success_rate(controls, success_level),
        "treatment_success": success_rate(treatments, success_level),
        "mcnemar": mcnemar_exact(pairs, success_level),
        "control_cost": cost_per_success(controls, success_level),
        "treatment_cost": cost_per_success(treatments, success_level),
        "reduction": reduction,
        "billable_ci": billable_ci,
        "treatment_failures": failure_counts(treatments),
    }


def decision_hint(summary: dict, min_billable_reduction: float = 0.15,
                  max_success_drop: float = 0.05) -> str:
    """Gợi ý KEEP / REJECT / HOLD từ dữ liệu — không thay thế đánh giá của người đọc.

    Chỉ dùng cho metric billable và success; các metric khác (invalid param, duplicate reads,
    recovery) phải xem bảng chi tiết trước khi quyết định.
    """
    control = summary.get("control_success")
    treatment = summary.get("treatment_success")
    billable = summary.get("reduction", {}).get("billable", {}).get("median")
    if control is None or treatment is None or billable is None:
        return "HOLD (thiếu dữ liệu)"
    if summary.get("pairs", 0) < 3:
        return "HOLD (<3 cặp: chỉ đủ screening, chưa đủ kết luận)"
    ci = summary.get("billable_ci")
    if ci is None or (ci[1] - ci[0]) > 0.5:
        return "HOLD (CI quá rộng, cần thêm cặp)"
    drop = control - treatment
    if drop > 0.10:
        return "REJECT (success giảm > 10 điểm phần trăm)"
    if billable >= min_billable_reduction and drop <= max_success_drop:
        return "KEEP (giảm billable, success không giảm quá ngưỡng)"
    if billable < 0:
        return "REJECT (billable tăng)"
    return "HOLD (cần thêm mẫu hoặc xem metric phụ)"
