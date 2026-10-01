"""Giao mot de benchmark cho Axiom Core qua dung API ma pane dung, va ghi lai toan bo SSE.

    python tests/bench/run_prompt.py <core_port> <bridge_port> <bridge_pid> <prompt_file> <log_file> [token]

Axiom lam viec tren tai lieu DANG MO, nen phai mo LibreOffice that truoc (khong mo phong bridge):
    "C:\\Program Files\\LibreOffice\\program\\soffice.exe" --calc
roi lay pid + port tu GET /health cua bridge (mac dinh port 47852 cho Calc).

Man hinh in ra tung tool call va tung vong suy luan (neu bat LlmShowReasoning) de theo doi luc chay;
<log_file> giu nguyen van SSE de doi chieu sau nay.
"""
from __future__ import annotations

import json
import sys
import time
import urllib.request

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

TERMINAL = ("run.completed", "run.failed", "run.cancelled", "run.timedout", "run.stopped")


def post(url: str, body: dict, token: str) -> dict:
    request = urllib.request.Request(url, data=json.dumps(body, ensure_ascii=False).encode("utf-8"),
                                     method="POST")
    request.add_header("Content-Type", "application/json")
    if token:
        request.add_header("X-Auth-Token", token)
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.loads(response.read().decode("utf-8"))


def main() -> int:
    if len(sys.argv) < 6:
        print(__doc__)
        return 2
    core_port, bridge_port, pid, prompt_file, log_path = sys.argv[1:6]
    token = sys.argv[6] if len(sys.argv) > 6 else ""
    with open(prompt_file, encoding="utf-8") as handle:
        prompt = handle.read()

    base = "http://127.0.0.1:%s" % core_port
    created = post(base + "/v1/runs", {
        "prompt": prompt,
        "office": {"port": int(bridge_port), "pid": int(pid), "app": "et", "family": "libreoffice"},
        "document": {"name": "Untitled 1"},
        "options": {"maxRounds": 100, "maxTokens": 400000},
    }, token)
    if not created.get("ok"):
        print("KHONG TAO DUOC LUOT CHAY:", created)
        return 1
    run_id = created["result"]["runId"]
    print("run:", run_id, "| conversation:", created["result"]["conversationId"], flush=True)

    request = urllib.request.Request(base + "/v1/runs/" + run_id + "/events")
    if token:
        request.add_header("X-Auth-Token", token)
    started = time.time()
    tools = 0
    with open(log_path, "w", encoding="utf-8") as log:
        log.write("run %s\n" % run_id)
        with urllib.request.urlopen(request, timeout=60 * 60) as response:
            event_type = None
            for raw in response:
                line = raw.decode("utf-8").rstrip("\r\n")
                log.write(line + "\n")
                if line.startswith("event:"):
                    event_type = line[6:].strip()
                    continue
                if not line.startswith("data:"):
                    continue
                try:
                    data = (json.loads(line[5:].strip()).get("data") or {})
                except json.JSONDecodeError:
                    continue
                elapsed = time.time() - started
                if event_type == "tool.finished":
                    tools += 1
                    print("[%6.0fs] %3d %s %s %s" % (
                        elapsed, tools, "ok " if data.get("ok") else "LOI",
                        data.get("action") or data.get("tool"), (data.get("error") or "")[:90]), flush=True)
                elif event_type == "run.reasoning":
                    print("[%6.0fs] ~~ vong %s: %s" % (
                        elapsed, data.get("round"), (data.get("text") or "")[:180].replace("\n", " ")), flush=True)
                elif event_type == "run.started":
                    print("[%6.0fs] bat dau (model %s)" % (elapsed, data.get("model")), flush=True)
                elif event_type in TERMINAL:
                    print("[%6.0fs] KET THUC: %s" % (elapsed, event_type), flush=True)
                    if event_type == "run.completed":
                        print("reply:", (data.get("reply") or "")[:600], flush=True)
                    else:
                        print("error:", data.get("error"), flush=True)
                    print("rounds=%s toolCalls=%s in=%s out=%s cached=%s verified=%s" % (
                        data.get("rounds"), tools, data.get("inputTokens"), data.get("outputTokens"),
                        data.get("cachedTokens"), data.get("verified")), flush=True)
                    log.flush()
                    break
    return 0


if __name__ == "__main__":
    sys.exit(main())
