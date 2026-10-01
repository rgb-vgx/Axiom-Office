"""LLM gia (OpenAI-compatible) tra loi theo kich ban - dung cho test e2e cua Agent Core.

    python tests/core/fake_llm.py --port 47877 --script kichban.json --requests reqs.jsonl

Kich ban la JSON: danh sach cac buoc, tra lan luot; het thi lap lai buoc cuoi.

    [{"tool": "office_action", "arguments": {"action": "et.writeRange", "params": {"range": "A1"}}},
     {"text": "Xong roi"},
     {"delay": 5, "text": "Cham"}]

Buoc co "when" (chuoi con cua system prompt) KHONG theo thu tu: request nao co system prompt chua chuoi do
thi tra buoc do (vd bo trich xuat memory chay nen xen giua cac luot agent). "texts" = danh sach cau tra loi
tra lan luot, het thi lap lai cau cuoi:

    {"when": "memory extractor", "texts": ["{\"facts\": []}"]}

Moi request duoc ghi them mot dong JSON vao --requests de test kiem tra prompt (ngu canh hoi thoai cu).

Cho wizard thiet lap (GET /v1/setup, POST /v1/llm/test, GET /v1/llm/models):
    --key <khoa>        bat buoc Authorization: Bearer <khoa>; sai/thieu -> 401 (nhanh "sai khoa API")
    --models a,b,c      GET /v1/models tra dung danh sach nay
    --fail-status 503   moi POST tra ma loi nay (nhanh "may chu loi")
    model "no-such-model" -> 404 (nhanh "model khong ton tai")
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
REQUIRED_KEY: str = ""      # --key: bat buoc Authorization: Bearer <key> (de test nhanh sai khoa)
MODELS: list[str] = ["fake-model", "fake-model-2"]
FAIL_STATUS: int = 0        # --fail-status 503: moi POST tra ma loi nay (de test nhanh may chu loi)
UNKNOWN_MODEL: str = "no-such-model"   # POST voi model nay -> 404 (de test nhanh model khong ton tai)
_LOCK = threading.Lock()
_INDEX = 0
_CALLS = 0


_ROUTED_INDEX: dict[int, int] = {}

# Cau trong loi nhac tu kiem chung cua Core (model.VerifyWorkNudge, core-go/internal/model/client.go).
# test_fake_bridge kiem tra chinh chuoi nay trong request, nen doi ben Go ma quen doi day thi se do ngay.
VERIFY_MARK = "check the work you just did"


def sequential() -> list[dict]:
    return [step for step in SCRIPT if "when" not in step and "on_verify" not in step]


def routed_step(body: dict) -> dict | None:
    """Buoc "when" khop system prompt cua request (khong tieu thu thu tu kich ban chinh)."""
    global _CALLS
    system = body.get("system") if isinstance(body.get("system"), str) else ""
    for message in body.get("messages", []):
        if message.get("role") == "system" and isinstance(message.get("content"), str):
            system += message["content"]
    for position, step in enumerate(SCRIPT):
        if "when" in step and step["when"] in system:
            with _LOCK:
                index = _ROUTED_INDEX.get(position, 0)
                _ROUTED_INDEX[position] = index + 1
                _CALLS += 1
            texts = step.get("texts") or [step.get("text", "")]
            return {"text": texts[min(index, len(texts) - 1)]}
    return None


def next_step() -> dict:
    global _INDEX, _CALLS
    steps = sequential()
    with _LOCK:
        step = steps[min(_INDEX, len(steps) - 1)] if steps else {"text": "ok"}
        _INDEX += 1
        _CALLS += 1
    return step


def is_verification_round(body: dict) -> bool:
    """True khi request HIEN TAI la luot tu kiem chung.

    Core chen loi nhac kiem chung thanh mot tin user, va tin do nam lai trong lich su den het luot - nen
    phai xet dung tin CUOI CUNG, khong phai ca body.
    """
    messages = body.get("messages") or []
    if not messages:
        return False
    content = messages[-1].get("content")
    if isinstance(content, str):
        return VERIFY_MARK in content
    if isinstance(content, list):
        return any(isinstance(block, dict) and VERIFY_MARK in str(block.get("text", "")) for block in content)
    return False


def verification_step(body: dict) -> dict | None:
    """Buoc rieng cho luot tu kiem chung, khi kich ban khai bao `"on_verify": true`.

    Chi dung MOT lan. Bai test muon buoc doc lai phat hien loi that thi khai bao buoc nay (thuong la mot
    tool call sua tiep); khong khai bao thi `verification_reply` tra loi khang dinh nhu binh thuong.
    """
    if not is_verification_round(body):
        return None
    for position, step in enumerate(SCRIPT):
        if not step.get("on_verify"):
            continue
        with _LOCK:
            index = _ROUTED_INDEX.get(position, 0)
            _ROUTED_INDEX[position] = index + 1
        if index == 0:
            return dict(step)
    return None


def verification_reply(body: dict) -> dict | None:
    """Luot TU KIEM CHUNG: Core chen them mot tin user sau khi agent da sua tai lieu (VerifyWorkEnabled).

    Fake tra lai dung cau tra loi nhap truoc do - giong mot model doc lai roi khang dinh lai la dung. Luot
    nay KHONG tieu thu buoc nao cua kich ban, nen moi kich ban cu van chay y nguyen khi tinh nang bat mac
    dinh. Muon kiem chung bat ra loi thi viet mot bai rieng dung buoc {"raw": ...}.
    """
    if not is_verification_round(body):
        return None
    for message in reversed(body.get("messages", [])):
        if message.get("role") != "assistant":
            continue
        content = message.get("content")
        if isinstance(content, str) and content.strip():
            return {"text": content}
        if isinstance(content, list):
            text = "".join(block.get("text", "") for block in content if isinstance(block, dict) and block.get("type") == "text")
            if text.strip():
                return {"text": text}
    return {"text": "Da kiem tra lai."}


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

    def _authorized(self) -> bool:
        """Khong co --key thi khong kiem tra (mac dinh)."""
        if not REQUIRED_KEY:
            return True
        return self.headers.get("Authorization", "") == "Bearer " + REQUIRED_KEY

    def _json(self, payload: dict, status: int = 200) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        if self.path == "/__state":
            self._json({"calls": _CALLS})
            return
        if self.path.endswith("/models"):
            if not self._authorized():
                self._json({"error": {"message": "invalid api key"}}, 401)
                return
            self._json({"object": "list", "data": [{"id": name, "object": "model"} for name in MODELS]})
            return
        self._json({"ok": False, "error": "not found"}, 404)

    def do_POST(self) -> None:
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length).decode("utf-8") if length else "{}"
        if REQUESTS_PATH:
            with open(REQUESTS_PATH, "a", encoding="utf-8") as handle:
                handle.write(json.dumps({"path": self.path, "body": json.loads(raw)}, ensure_ascii=False) + "\n")

        if not self._authorized():
            self._json({"error": {"message": "invalid api key"}}, 401)
            return
        if FAIL_STATUS:
            self._json({"error": {"message": "server error"}}, FAIL_STATUS)
            return

        parsed = json.loads(raw)
        if parsed.get("model") == UNKNOWN_MODEL:
            self._json({"error": {"message": "model `%s` does not exist" % UNKNOWN_MODEL}}, 404)
            return

        step = verification_step(parsed) or verification_reply(parsed) or routed_step(parsed) or next_step()
        delay = float(step.get("delay", 0))
        if delay > 0:
            time.sleep(delay)

        self._json(build_response(step))


def main() -> int:
    global SCRIPT, REQUESTS_PATH, REQUIRED_KEY, MODELS, FAIL_STATUS
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--script", required=True)
    parser.add_argument("--requests")
    parser.add_argument("--key", help="bat buoc Authorization: Bearer <key> (test nhanh sai khoa)")
    parser.add_argument("--models", help="danh sach model cho GET /v1/models, phan cach dau phay")
    parser.add_argument("--fail-status", type=int, default=0, help="moi POST tra ma loi nay (test may chu loi)")
    args = parser.parse_args()
    REQUIRED_KEY = args.key or ""
    MODELS = [name.strip() for name in (args.models or "fake-model,fake-model-2").split(",") if name.strip()]
    FAIL_STATUS = args.fail_status

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
