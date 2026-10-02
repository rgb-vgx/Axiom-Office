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


def save_document(bridge_port: str, token: str, path: str) -> str:
    """Luu tai lieu dang mo ra file. Phai lam TRUOC khi dong LibreOffice, khong thi ket qua bay mat:
    do tai lieu chua luu la "Untitled 1" nam trong bo nho - dong app la xong."""
    body = {"action": "et.saveAs", "params": {"path": path}}
    request = urllib.request.Request("http://127.0.0.1:%s/cmd" % bridge_port,
                                     data=json.dumps(body, ensure_ascii=False).encode("utf-8"), method="POST")
    request.add_header("Content-Type", "application/json")
    if token:
        request.add_header("X-Auth-Token", token)
    try:
        with urllib.request.urlopen(request, timeout=300) as response:
            payload = json.loads(response.read().decode("utf-8"))
        return "da luu %s" % path if payload.get("ok") else "luu that bai: %s" % payload.get("error")
    except Exception as exc:  # noqa: BLE001
        return "luu that bai: %s: %s" % (type(exc).__name__, exc)


def main() -> int:
    if len(sys.argv) < 6:
        print(__doc__)
        return 2
    core_port, bridge_port, pid, prompt_file, log_path = sys.argv[1:6]
    token = sys.argv[6] if len(sys.argv) > 6 else ""
    # Tuy chon: <file_luu> <max_tokens>.
    #   <file_luu>  - luu tai lieu ra day sau khi luot chay ket thuc.
    #   <max_tokens> - ngan sach token PHAI TRA cho ca luot (mac dinh 400000; Core chan tren o 1.000.000).
    save_path = sys.argv[7] if len(sys.argv) > 7 else ""
    max_tokens = int(sys.argv[8]) if len(sys.argv) > 8 else 400000
    # <max_rounds>: tran so vong. Mac dinh 300; Core chan tren o 1000.
    max_rounds = int(sys.argv[9]) if len(sys.argv) > 9 else 300
    with open(prompt_file, encoding="utf-8") as handle:
        prompt = handle.read()

    base = "http://127.0.0.1:%s" % core_port
    created = post(base + "/v1/runs", {
        "prompt": prompt,
        "office": {"port": int(bridge_port), "pid": int(pid), "app": "et", "family": "libreoffice"},
        "document": {"name": "Untitled 1"},
        # Ngan sach token ca luot, tinh theo token PHAI TRA THAT (cache khong tinh). De 400k cho de so
        # giua cac lan chay: cung mot con so, doi cach tinh thi luot chay di duoc bao xa.
        # maxRounds 100 da la nut that that su o Test 1 (dung sau 315 tool call), nen nang len de bai
        # khong bi cat vi so vong khi dang con tien.
        "options": {"maxRounds": max_rounds, "maxTokens": max_tokens},
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
    if save_path:
        # Luu NGAY sau khi luot chay xong va TRUOC khi ai do dong LibreOffice: tai lieu chua luu chi nam
        # trong bo nho, dong app la mat sach cong da lam.
        print(save_document(bridge_port, token, save_path), flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
