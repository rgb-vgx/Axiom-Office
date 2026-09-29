"""LLM gia (OpenAI-compatible) tra loi theo kich ban - dung cho test e2e cua Agent Core.

    python tests/core/fake_llm.py --port 47877 --script kichban.json --requests reqs.jsonl

Kich ban la JSON: danh sach cac buoc, tra lan luot; het thi lap lai buoc cuoi.

    [{"tool": "office_action", "arguments": {"action": "et.writeRange", "params": {"range": "A1"}}},
     {"text": "Xong roi"},
     {"delay": 5, "text": "Cham"}]

Moi request duoc ghi them mot dong JSON vao --requests de test kiem tra prompt (ngu canh hoi thoai cu).
Khong can thu vien ngoai.
"""
from __future__ import annotations

import argparse
import json
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

SCRIPT: list[dict] = []
REQUESTS_PATH: str | None = None
_LOCK = threading.Lock()
_INDEX = 0
_CALLS = 0


def next_step() -> dict:
    global _INDEX, _CALLS
    with _LOCK:
        step = SCRIPT[min(_INDEX, len(SCRIPT) - 1)] if SCRIPT else {"text": "ok"}
        _INDEX += 1
        _CALLS += 1
    return step


def build_response(step: dict) -> dict:
    if "raw" in step:
        return step["raw"]
    if "tool" in step:
        return {
            "id": "chatcmpl-fake",
            "object": "chat.completion",
            "model": "fake",
            "choices": [
                {
                    "index": 0,
                    "finish_reason": "tool_calls",
                    "message": {
                        "role": "assistant",
                        "content": step.get("text"),
                        "tool_calls": [
                            {
                                "id": "call_" + str(_CALLS),
                                "type": "function",
                                "function": {
                                    "name": step["tool"],
                                    "arguments": json.dumps(step.get("arguments", {}), ensure_ascii=False),
                                },
                            }
                        ],
                    },
                }
            ],
            "usage": {"prompt_tokens": 100, "completion_tokens": 20},
        }
    return {
        "id": "chatcmpl-fake",
        "object": "chat.completion",
        "model": "fake",
        "choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": step.get("text", "")}}],
        "usage": {"prompt_tokens": 50, "completion_tokens": 10},
    }


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *args) -> None:  # tat log ra stderr
        pass

    def do_GET(self) -> None:
        if self.path == "/__state":
            body = json.dumps({"calls": _CALLS}).encode()
        else:
            body = b'{"ok":false,"error":"not found"}'
        self.send_response(200 if self.path == "/__state" else 404)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self) -> None:
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length).decode("utf-8") if length else "{}"
        if REQUESTS_PATH:
            with open(REQUESTS_PATH, "a", encoding="utf-8") as handle:
                handle.write(json.dumps({"path": self.path, "body": json.loads(raw)}, ensure_ascii=False) + "\n")

        step = next_step()
        delay = float(step.get("delay", 0))
        if delay > 0:
            time.sleep(delay)

        payload = json.dumps(build_response(step), ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)


def main() -> int:
    global SCRIPT, REQUESTS_PATH
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--script", required=True)
    parser.add_argument("--requests")
    args = parser.parse_args()

    with open(args.script, encoding="utf-8") as handle:
        SCRIPT = json.load(handle)
    REQUESTS_PATH = args.requests
    if REQUESTS_PATH:
        open(REQUESTS_PATH, "w", encoding="utf-8").close()

    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(f"fake llm on 127.0.0.1:{args.port} ({len(SCRIPT)} buoc kich ban)", flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
