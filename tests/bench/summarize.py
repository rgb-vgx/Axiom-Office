"""Doc transcript .jsonl cua mot phien Claude Code va tom tat CACH no lam bai.

    python tests/bench/summarize.py <duong_dan.jsonl> [--tail N] [--full]

Dung cho benchmark LibreOffice Calc (tests/bench/README.md): can biet no goi tool gi, dung script hay
go tung o, co doc lai kiem chung khong, va cho nao phai quay lai sua - khong chi ket qua cuoi cung.
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from collections import Counter

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")


def tool_calls(entry: dict) -> list[tuple[str, dict]]:
    message = entry.get("message")
    if not isinstance(message, dict):
        return []
    content = message.get("content")
    if not isinstance(content, list):
        return []
    calls = []
    for block in content:
        if isinstance(block, dict) and block.get("type") == "tool_use":
            calls.append((str(block.get("name") or "?"), block.get("input") or {}))
    return calls


def text_of(entry: dict) -> str:
    message = entry.get("message")
    if not isinstance(message, dict):
        return ""
    content = message.get("content")
    if isinstance(content, str):
        return content
    if not isinstance(content, list):
        return ""
    return "\n".join(block.get("text", "") for block in content
                     if isinstance(block, dict) and block.get("type") == "text")


def one_line(value, limit: int = 150) -> str:
    flat = " ".join(str(value or "").split())
    return flat[: limit - 1] + "…" if len(flat) > limit else flat


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("transcript")
    parser.add_argument("--tail", type=int, default=12, help="so dong cuoi cua tung nhom tool")
    parser.add_argument("--full", action="store_true", help="in tat ca chu khong cat theo --tail")
    args = parser.parse_args()

    entries = []
    with open(args.transcript, encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if line:
                try:
                    entries.append(json.loads(line))
                except json.JSONDecodeError:
                    continue

    histogram = Counter()
    by_tool: dict[str, list[str]] = {}
    usage = Counter()
    assistant_messages = 0
    files_written: Counter = Counter()
    failed_results: list[str] = []

    for entry in entries:
        kind = entry.get("type")
        message = entry.get("message") if isinstance(entry.get("message"), dict) else {}
        if kind == "assistant":
            assistant_messages += 1
            for name, payload in tool_calls(entry):
                histogram[name] += 1
                if name == "Bash":
                    by_tool.setdefault("Bash", []).append(one_line(payload.get("command")))
                elif name in ("Write", "Edit", "NotebookEdit"):
                    path = str(payload.get("file_path") or payload.get("notebook_path") or "?")
                    files_written[os.path.basename(path)] += 1
                    by_tool.setdefault(name, []).append(one_line(path, 90))
                else:
                    by_tool.setdefault(name, []).append(one_line(json.dumps(payload, ensure_ascii=False)))
            if isinstance(message.get("usage"), dict):
                for key, value in message["usage"].items():
                    if isinstance(value, int):
                        usage[key] += value
        if kind == "user":
            content = message.get("content")
            if isinstance(content, list):
                for block in content:
                    if isinstance(block, dict) and block.get("type") == "tool_result" and block.get("is_error"):
                        failed_results.append(one_line(block.get("content"), 200))

    total_calls = sum(histogram.values())
    print("=" * 72)
    print("transcript:", args.transcript)
    print("entries: %d | luot assistant: %d | tool call: %d" % (len(entries), assistant_messages, total_calls))
    print("tool:", ", ".join("%s=%d" % item for item in histogram.most_common()))
    if usage:
        print("token:", ", ".join("%s=%d" % item for item in usage.most_common()))
    if failed_results:
        print("tool tra loi LOI: %d lan" % len(failed_results))
    if files_written:
        print("file ghi/sua:", ", ".join("%s×%d" % item for item in files_written.most_common()))

    for name in sorted(by_tool):
        lines = by_tool[name]
        print("-" * 72)
        print("%s (%d)" % (name, len(lines)))
        shown = lines if args.full else lines[-args.tail:]
        if not args.full and len(lines) > len(shown):
            print("  … %d dong dau bo qua (dung --full de xem het)" % (len(lines) - len(shown)))
        for line in shown:
            print("  ", line)

    final = ""
    for entry in reversed(entries):
        if entry.get("type") == "assistant":
            candidate = text_of(entry)
            if candidate.strip():
                final = candidate
                break
    print("=" * 72)
    print("tra loi cuoi cung:")
    print(final[:4000] if final else "(khong co)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
