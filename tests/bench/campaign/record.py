"""Ghi một lần chạy benchmark vào runs.jsonl.

    python tests/bench/campaign/record.py --out runs.jsonl --pair p001 --task W03 \\
      --variant B0 --role control --success S3 --billable 12000 --tool-calls 8

Chạy control (B0) và treatment (B1..B6) với cùng pair/task; report.py sẽ ghép cặp.
Có thể truyền nguyên một JSON bằng --json để script khác gọi.
"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

try:
    from .records import RunRecord, append_jsonl
except ImportError:  # chạy trực tiếp
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    from records import RunRecord, append_jsonl


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, help="file JSONL để ghi thêm")
    parser.add_argument("--json", help="một object JSON đầy đủ (bỏ qua các cờ bên dưới)")
    parser.add_argument("--run-id")
    parser.add_argument("--pair", dest="pair_id")
    parser.add_argument("--task", dest="task_id")
    parser.add_argument("--variant", default="B0")
    parser.add_argument("--role", default="control", choices=["control", "treatment"])
    parser.add_argument("--success", default="S3", choices=["S0", "S1", "S2", "S3"])
    parser.add_argument("--failure", default="")
    parser.add_argument("--app", default="")
    parser.add_argument("--model", default="")
    parser.add_argument("--commit", default="")
    parser.add_argument("--fixture-hash", default="")
    parser.add_argument("--billable", type=int, default=0)
    parser.add_argument("--input-tokens", type=int, default=0)
    parser.add_argument("--cached-tokens", type=int, default=0)
    parser.add_argument("--output-tokens", type=int, default=0)
    parser.add_argument("--tool-calls", type=int, default=0)
    parser.add_argument("--invalid-tool-calls", type=int, default=0)
    parser.add_argument("--duplicate-reads", type=int, default=0)
    parser.add_argument("--rounds", type=int, default=0)
    parser.add_argument("--wall", type=float, default=0.0)
    parser.add_argument("--context-overflow", action="store_true")
    parser.add_argument("--read-result-calls", type=int, default=0)
    parser.add_argument("--unnecessary-read-result", type=int, default=0)
    parser.add_argument("--notes", default="")
    args = parser.parse_args(argv)

    if args.json:
        payload = json.loads(args.json)
    else:
        missing = [name for name in ("run_id", "pair_id", "task_id") if not getattr(args, name)]
        if missing:
            parser.error("thiếu " + ", ".join("--" + name.replace("_", "-") for name in missing))
        payload = {
            "run_id": args.run_id, "pair_id": args.pair_id, "task_id": args.task_id,
            "variant": args.variant, "role": args.role, "success": args.success,
            "failure": args.failure, "app": args.app, "model": args.model, "commit": args.commit,
            "fixture_hash": args.fixture_hash, "billable": args.billable,
            "input_tokens": args.input_tokens, "cached_tokens": args.cached_tokens,
            "output_tokens": args.output_tokens, "tool_calls": args.tool_calls,
            "invalid_tool_calls": args.invalid_tool_calls, "duplicate_reads": args.duplicate_reads,
            "rounds": args.rounds, "wall_seconds": args.wall, "context_overflow": args.context_overflow,
            "read_result_calls": args.read_result_calls,
            "unnecessary_read_result": args.unnecessary_read_result, "notes": args.notes,
        }
    record = RunRecord.from_dict(payload)
    append_jsonl(args.out, record)
    print(f"đã ghi {record.run_id} ({record.role}, {record.variant}) vào {args.out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
