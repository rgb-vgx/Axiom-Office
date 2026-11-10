"""Bản ghi một lần chạy benchmark + ghép cặp control/treatment.

Mỗi dòng trong `runs.jsonl` là một lần thực thi MỘT task trên MỘT trạng thái tài liệu.
Cặp (pair) gồm đúng hai bản ghi cùng `pair_id`: một `control` (B0) và một `treatment` (B1..B6).

Thiết kế theo khuyến nghị trong docs/research/gpt-advice.md: đơn vị benchmark là một lần
thực thi task, không phải một vòng agent hay một tool call; mọi bản ghi phải có manifest
(commit, model, fixture, variant) để không so nhầm hai điều kiện khác nhau.
"""
from __future__ import annotations

import json
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Iterable

SUCCESS_LEVELS = ("S0", "S1", "S2", "S3")
ROLES = ("control", "treatment")

# Các metric số được tổng hợp mặc định. Tên khớp với cột trong báo cáo.
NUMERIC_METRICS = (
    "billable",
    "input_tokens",
    "cached_tokens",
    "output_tokens",
    "tool_calls",
    "invalid_tool_calls",
    "duplicate_reads",
    "rounds",
    "wall_seconds",
    "read_result_calls",
    "unnecessary_read_result",
)


@dataclass
class RunRecord:
    run_id: str
    pair_id: str
    task_id: str
    variant: str
    role: str
    app: str = ""
    model: str = ""
    commit: str = ""
    fixture_hash: str = ""
    started_at: str = ""
    success: str = "S0"
    failure: str = ""
    billable: int = 0
    input_tokens: int = 0
    cached_tokens: int = 0
    output_tokens: int = 0
    tool_calls: int = 0
    invalid_tool_calls: int = 0
    duplicate_reads: int = 0
    rounds: int = 0
    wall_seconds: float = 0.0
    context_overflow: bool = False
    read_result_calls: int = 0
    unnecessary_read_result: int = 0
    notes: str = ""
    extra: dict = field(default_factory=dict)

    def __post_init__(self) -> None:
        self.success = str(self.success).upper()
        self.role = str(self.role).lower()
        if self.success not in SUCCESS_LEVELS:
            raise ValueError(f"{self.run_id}: success phải là {SUCCESS_LEVELS}, được {self.success!r}")
        if self.role not in ROLES:
            raise ValueError(f"{self.run_id}: role phải là {ROLES}, được {self.role!r}")

    @classmethod
    def from_dict(cls, data: dict) -> "RunRecord":
        known = {name for name in cls.__dataclass_fields__ if name != "extra"}
        payload = {key: value for key, value in data.items() if key in known}
        payload["extra"] = {key: value for key, value in data.items() if key not in known}
        return cls(**payload)

    def to_dict(self) -> dict:
        data = asdict(self)
        extra = data.pop("extra", {})
        return {**data, **extra}

    def successful(self, level: str = "S3") -> bool:
        """Primary success chỉ tính mức yêu cầu trở lên (mặc định S3)."""
        return SUCCESS_LEVELS.index(self.success) >= SUCCESS_LEVELS.index(str(level).upper())


@dataclass
class Pair:
    pair_id: str
    task_id: str
    control: RunRecord
    treatment: RunRecord

    @property
    def variant(self) -> str:
        return self.treatment.variant


def load_jsonl(path: str | Path) -> list[RunRecord]:
    records: list[RunRecord] = []
    with open(path, encoding="utf-8") as handle:
        for number, line in enumerate(handle, 1):
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            try:
                records.append(RunRecord.from_dict(json.loads(line)))
            except (json.JSONDecodeError, ValueError) as exc:
                raise ValueError(f"{path}:{number}: {exc}") from exc
    return records


def write_jsonl(path: str | Path, records: Iterable[RunRecord]) -> None:
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        for record in records:
            handle.write(json.dumps(record.to_dict(), ensure_ascii=False) + "\n")


def append_jsonl(path: str | Path, record: RunRecord) -> None:
    """Ghi nối một bản ghi (record.py dùng để nhiều lần chạy cùng gom vào một file)."""
    with open(path, "a", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps(record.to_dict(), ensure_ascii=False) + "\n")


def pair_records(records: Iterable[RunRecord]) -> list[Pair]:
    """Ghép theo pair_id; mỗi cặp phải có đúng một control và một treatment.

    Không tự đoán cặp theo task_id để tránh ghép nhầm hai lần chạy khác điều kiện.
    """
    groups: dict[str, list[RunRecord]] = {}
    for record in records:
        groups.setdefault(record.pair_id, []).append(record)
    pairs: list[Pair] = []
    for pair_id, group in groups.items():
        controls = [item for item in group if item.role == "control"]
        treatments = [item for item in group if item.role == "treatment"]
        if len(controls) != 1 or len(treatments) != 1:
            raise ValueError(
                f"pair {pair_id}: cần đúng 1 control + 1 treatment, được "
                f"{len(controls)} control / {len(treatments)} treatment"
            )
        control, treatment = controls[0], treatments[0]
        if control.task_id != treatment.task_id:
            raise ValueError(f"pair {pair_id}: task khác nhau ({control.task_id} vs {treatment.task_id})")
        if control.variant != "B0":
            raise ValueError(f"pair {pair_id}: control phải là B0, được {control.variant}")
        pairs.append(Pair(pair_id, control.task_id, control, treatment))
    pairs.sort(key=lambda item: (item.task_id, item.pair_id))
    return pairs
