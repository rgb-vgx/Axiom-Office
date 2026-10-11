"""Spec T4a (pane Ask AI trong WPS, docs/wps-bridge.md muc 7): luot chay agent qua kenh add-in /wps/v1/runs.

Pane la trang do Core phuc vu (cung origin voi Core) nen khong co token: no goi
    POST /wps/v1/runs {prompt, conversationId?}     -> {runId, conversationId, status}
    GET  /wps/v1/runs/{id}                           -> trang thai (nhu GET /v1/runs/{id})
    GET  /wps/v1/runs/{id}/events                    -> SSE (nhu /v1/runs/{id}/events)
    POST /wps/v1/runs/{id}/cancel | /confirm         -> nhu /v1/runs/...
voi cung xac thuc kenh add-in (Origin = origin cua Core, hoac token khi khong co Origin). Core tu gan luot chay vao
bridge WPS (office = bridge WPS, tai lieu = lan poll gan nhat cua add-in). Add-in chua ket noi -> HTTP 409.
/v1/runs giu nguyen guard cu (moi Origin -> 403).

Build Core that, LLM gia (tests/core/fake_llm.py), add-in gia poll + tra ket qua lenh. Khong can WPS.

    python3 tests/wps/test_wps_pane_e2e.py
"""
from __future__ import annotations

import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import test_wps_bridge_e2e as base  # noqa: E402

ROOT = base.ROOT
check = base.check
FAKE_LLM = os.path.join(ROOT, "tests", "core", "fake_llm.py")
CATALOG = [
    {"name": "writer.getText", "kind": "wps", "agent": True, "summary": "Đọc toàn bộ text",
     "params": [{"name": "maxChars", "required": False, "hint": None}]},
    {"name": "writer.appendText", "kind": "wps", "agent": True, "summary": "Nối vào cuối tài liệu",
     "params": [{"name": "text", "required": True, "hint": None}]},
    {"name": "writer.save", "kind": "wps", "agent": True, "summary": "Lưu", "params": []},
]
DOCUMENT = {"name": "bao-cao-tuan.docx", "fullName": "/home/u/bao-cao-tuan.docx"}


class FakeAddin(threading.Thread):
    """Add-in gia: poll lien tuc, chay job (ghi lai action) va gui ket qua."""

    def __init__(self, core):
        super().__init__(daemon=True)
        self.core, self.actions, self.stop_flag = core, [], threading.Event()

    def run(self):
        while not self.stop_flag.is_set():
            status, _, text = base.request(self.core.base + "/wps/v1/poll", "POST",
                                           {"client": "pane-test", "app": "wps", "version": "0.1.0",
                                            "document": DOCUMENT, "commands": CATALOG},
                                           {"Origin": self.core.origin}, timeout=15)
            if status != 200:
                time.sleep(0.3)
                continue
            for job in (base.as_json(text).get("result") or {}).get("jobs") or []:
                self.actions.append((job.get("action"), job.get("params")))
                if job.get("action") == "writer.getText":
                    answer = {"ok": True, "result": {"name": DOCUMENT["name"], "fullName": DOCUMENT["fullName"],
                                                     "totalChars": 12, "truncated": False, "text": "Bao cao tuan"}}
                else:
                    answer = {"ok": True, "result": {"done": True}}
                base.request(self.core.base + "/wps/v1/result", "POST", dict(answer, id=job.get("id")),
                             {"Origin": self.core.origin})


class FakeLlm:
    def __init__(self, work, script):
        self.port = base.free_port()
        self.requests_path = os.path.join(work, "llm-requests.jsonl")
        path = os.path.join(work, "llm-script.json")
        with open(path, "w", encoding="utf-8") as handle:
            json.dump(script, handle, ensure_ascii=False)
        self.process = subprocess.Popen([sys.executable, FAKE_LLM, "--port", str(self.port), "--script", path,
                                         "--requests", self.requests_path],
                                        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        deadline = time.time() + 15
        while time.time() < deadline:
            if base.request("http://127.0.0.1:%d/__state" % self.port, timeout=1)[0] == 200:
                return
            time.sleep(0.2)
        raise SystemExit("fake LLM khong len")

    def requests(self):
        if not os.path.exists(self.requests_path):
            return []
        with open(self.requests_path, encoding="utf-8") as handle:
            return [json.loads(line) for line in handle if line.strip()]

    def stop(self):
        self.process.terminate()


def sse(url, headers, timeout=40, until=("run.completed", "run.failed", "run.cancelled", "run.stopped",
                                           "run.timedout"), on_event=None):
    """Doc SSE toi su kien ket thuc; tra [(event, data)]."""
    events = []
    request = urllib.request.Request(url, headers=dict(headers, Accept="text/event-stream"))
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            name = None
            for raw in response:
                line = raw.decode("utf-8", "replace").rstrip("\r\n")
                if line.startswith("event:"):
                    name = line[6:].strip()
                elif line.startswith("data:"):
                    data = base.as_json(line[5:].strip())
                    events.append((name, data))
                    if on_event:
                        on_event(name, data)
                    if name in until:
                        break
    except Exception as exc:  # noqa: BLE001
        events.append(("__error__", {"error": str(exc)}))
    return events


def payload(data):
    return data.get("data", data) if isinstance(data, dict) else {}


def main() -> int:
    work = tempfile.mkdtemp(prefix="axiom-wps-pane-")
    addin_dir = os.path.join(work, "wps-addin")
    os.makedirs(addin_dir)
    with open(os.path.join(addin_dir, "index.html"), "w", encoding="utf-8") as handle:
        handle.write("<!DOCTYPE html><html><body></body></html>\n")
    session_dir = os.path.join(work, "sessions")
    os.makedirs(session_dir)
    llm = FakeLlm(work, [
        # Luot 1: sua tai lieu roi tra loi.
        {"tool": "office_action", "arguments": {"action": "writer.appendText", "params": {"text": "Xin chào"}}},
        {"text": "Đã thêm dòng chào"},
        # Luot 2: cham - de huy.
        {"delay": 8, "text": "Cham qua"},
        # Luot 3: tu goi luu (nguoi dung khong nho luu) -> the xac nhan.
        {"tool": "office_action", "arguments": {"action": "writer.save", "params": {}}},
        {"text": "Không lưu theo ý bạn"},
        # Luot 4: hoi thoai tiep.
        {"text": "Tiếp tục cuộc trò chuyện"},
    ])
    core = base.Core(base.core_exe(work), work, base.TOKEN, base.free_port(), addin_dir, session_dir, {
        "AXIOM_LLM_PROVIDER": "openai", "AXIOM_LLM_ENDPOINT": "http://127.0.0.1:%d/v1" % llm.port,
        "AXIOM_LLM_MODEL": "fake-model", "AXIOM_LLM_API_KEY": "fake-key", "AXIOM_VERIFY_WORK": "0",
        "AXIOM_MEMORY_ENABLED": "0", "AXIOM_CONFIRM_TIMEOUT": "20",
    })
    own = {"Origin": core.origin}
    addin = None
    try:
        # ---- chua co add-in
        status, _, text = base.request(core.base + "/wps/v1/runs", "POST", {"prompt": "chao"}, own)
        check(status == 409 and "not connected" in text, "pane: add-in chua ket noi -> 409 'not connected'", (status, text))

        addin = FakeAddin(core)
        addin.start()
        base.wait_for(lambda: (base.as_json(base.request("http://127.0.0.1:%d/health" % core.port)[2])), 2)
        time.sleep(1.0)

        # ---- xac thuc
        status, _, _ = base.request(core.base + "/wps/v1/runs", "POST", {"prompt": "x"}, {"Origin": "http://evil.example"})
        check(status == 403, "pane: Origin la -> 403", status)
        status, _, _ = base.request(core.base + "/wps/v1/runs", "POST", {"prompt": "x"}, {})
        check(status == 401, "pane: khong Origin, khong token -> 401", status)
        status, _, _ = base.request(core.base + "/v1/runs", "POST", {"prompt": "x", "office": {"port": 1}}, own)
        check(status == 403, "/v1/runs voi Origin cua Core van 403 (guard cu)", status)

        # ---- luot 1
        status, _, text = base.request(core.base + "/wps/v1/runs", "POST", {"prompt": "Thêm một dòng chào"}, own)
        reply = base.as_json(text).get("result") or {}
        check(status == 200 and reply.get("runId") and reply.get("conversationId"), "pane: POST /wps/v1/runs -> runId",
              (status, text))
        run_id, conversation = reply.get("runId"), reply.get("conversationId")
        events = sse(core.base + "/wps/v1/runs/%s/events" % run_id, own)
        names = [e for e, _ in events]
        started = [payload(d) for e, d in events if e == "tool.started"]
        done = [payload(d) for e, d in events if e == "run.completed"]
        check(any(s.get("action") == "writer.appendText" for s in started), "pane SSE: tool.started writer.appendText", names)
        check(done and done[0].get("reply") == "Đã thêm dòng chào", "pane SSE: run.completed co reply", events[-2:])
        check(("writer.appendText", {"text": "Xin chào"}) in addin.actions, "add-in nhan lenh cua luot pane", addin.actions)
        status, _, text = base.request(core.base + "/wps/v1/runs/%s" % run_id, headers=own)
        check(status == 200 and (base.as_json(text).get("result") or {}).get("status") == "completed",
              "pane: GET /wps/v1/runs/{id} -> completed", text[:300])
        first = llm.requests()[0] if llm.requests() else {}
        check("bao-cao-tuan.docx" in json.dumps(first, ensure_ascii=False),
              "prompt co ten tai lieu tu lan poll cua add-in", "")

        # ---- trinh duyet KHONG gui Origin cho GET cung origin (EventSource cua pane trong WPS, thu 11/10/2026: 401).
        # Pane la trang cua Core nen trinh duyet gui "Sec-Fetch-Site: same-origin" (header trang khac khong gia duoc).
        # Chi GET duoc noi long; POST van can Origin (trinh duyet luon gui Origin cho POST).
        same = {"Sec-Fetch-Site": "same-origin"}
        events = sse(core.base + "/wps/v1/runs/%s/events" % run_id, same)
        check(any(e == "run.completed" for e, _ in events), "pane SSE khong Origin + Sec-Fetch-Site: same-origin -> doc duoc",
              [e for e, _ in events][:5])
        status, _, _ = base.request(core.base + "/wps/v1/runs/%s" % run_id, headers=same)
        check(status == 200, "GET /wps/v1/runs/{id} voi Sec-Fetch-Site: same-origin -> 200", status)
        status, _, _ = base.request(core.base + "/wps/v1/runs/%s" % run_id, headers={"Sec-Fetch-Site": "cross-site"})
        check(status in (401, 403), "GET voi Sec-Fetch-Site: cross-site (khong token) -> bi chan", status)
        status, _, _ = base.request(core.base + "/wps/v1/runs", "POST", {"prompt": "x"}, same)
        check(status in (401, 403), "POST khong Origin chi co Sec-Fetch-Site -> van bi chan", status)
        status, _, _ = base.request(core.base + "/v1/skills", headers=same)
        check(status in (401, 403), "Sec-Fetch-Site khong mo khoa /v1/* (guard cu)", status)

        # ---- luot 2: huy
        status, _, text = base.request(core.base + "/wps/v1/runs", "POST",
                                       {"prompt": "Làm việc lâu", "conversationId": conversation}, own)
        run2 = (base.as_json(text).get("result") or {}).get("runId")
        time.sleep(1.0)
        status, _, text = base.request(core.base + "/wps/v1/runs/%s/cancel" % run2, "POST", {}, own)
        check(status == 200 and (base.as_json(text).get("result") or {}).get("cancelled") is True,
              "pane: POST /wps/v1/runs/{id}/cancel", (status, text))
        events = sse(core.base + "/wps/v1/runs/%s/events" % run2, own, timeout=20)
        check(any(e == "run.cancelled" for e, _ in events), "pane SSE: run.cancelled", [e for e, _ in events])

        # ---- luot 3: the xac nhan
        status, _, text = base.request(core.base + "/wps/v1/runs", "POST",
                                       {"prompt": "Sửa lỗi chính tả", "conversationId": conversation}, own)
        run3 = (base.as_json(text).get("result") or {}).get("runId")
        answered = {}

        def on_event(name, data):
            if name == "confirm.required" and not answered:
                info = payload(data)
                answered["id"] = info.get("confirmationId")
                answered["reply"] = base.request(core.base + "/wps/v1/runs/%s/confirm" % run3, "POST",
                                                 {"confirmationId": info.get("confirmationId"), "approved": False}, own)

        events = sse(core.base + "/wps/v1/runs/%s/events" % run3, own, timeout=40, on_event=on_event)
        check(answered.get("id") and answered["reply"][0] == 200, "pane: confirm.required -> POST /wps/v1/runs/{id}/confirm",
              answered)
        check(any(e == "run.completed" for e, _ in events), "luot 3 ket thuc binh thuong sau khi tu choi", [e for e, _ in events])
        check(not any(a == "writer.save" for a, _ in addin.actions), "tu choi -> add-in khong nhan writer.save", addin.actions)

        # ---- luot 4: cung hoi thoai
        status, _, text = base.request(core.base + "/wps/v1/runs", "POST",
                                       {"prompt": "Tiếp", "conversationId": conversation}, own)
        reply = base.as_json(text).get("result") or {}
        check(status == 200 and reply.get("conversationId") == conversation, "pane: conversationId giu nguyen", reply)
        sse(core.base + "/wps/v1/runs/%s/events" % reply.get("runId"), own)
    finally:
        if addin is not None:
            addin.stop_flag.set()
        core.stop()
        llm.stop()
        shutil.rmtree(work, ignore_errors=True)

    failed = [name for ok, name in base.RESULTS if not ok]
    print("\n%d passed, %d failed" % (len(base.RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
