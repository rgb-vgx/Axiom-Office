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
# Nguoi dung boi den "Bao cao tuan" (12 ky tu, ca tai lieu) truoc khi hoi.
SELECTION_TEXT = "Bao cao tuan"


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
                                                     "totalChars": 12, "truncated": False, "text": SELECTION_TEXT}}
                elif job.get("action") == "writer.selection":
                    answer = {"ok": True, "result": {"text": SELECTION_TEXT, "start": 0, "end": 12}}
                elif job.get("action") == "writer.lockSelection":
                    answer = {"ok": True, "result": {"chars": 12, "start": 0, "end": 12, "docEnd": 12,
                                                     "tables": 0, "tablesKnown": True}}
                elif job.get("action") == "writer.unlockSelection":
                    answer = {"ok": True, "result": {"locked": False}}
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
        # Cau hinh ghi tu pane di vao $XDG_CONFIG_HOME/axiom-office/config.json - de rieng, khong dung
        # vao cau hinh that cua may chay test.
        "XDG_CONFIG_HOME": os.path.join(work, "xdg"),
    })
    config_path = os.path.join(work, "xdg", "axiom-office", "config.json")
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

        # ---- kenh quan tri cho pane/ribbon (docs/ribbon-ui.md muc 2): cung quy tac xac thuc voi /wps/v1/runs,
        # nhung mo duoc cac duong /v1/* ma pane khong co token khong goi duoc (Thiet lap, Bo nho AI, Hoi thoai
        # truoc, Nhat ky hoat dong).
        status, _, text = base.request(core.base + "/wps/v1/setup", headers=own)
        check(status == 200 and "visualQaEnabled" in text, "pane: GET /wps/v1/setup (Thiet lap) -> 200", (status, text[:200]))
        status, _, _ = base.request(core.base + "/wps/v1/setup", headers={"Origin": "http://evil.example"})
        check(status == 403, "pane: /wps/v1/setup Origin la -> 403", status)
        status, _, _ = base.request(core.base + "/wps/v1/setup")
        check(status == 401, "pane: /wps/v1/setup khong Origin, khong token -> 401", status)
        status, _, text = base.request(core.base + "/wps/v1/conversations", headers=own)
        check(status == 200, "pane: GET /wps/v1/conversations (Hoi thoai truoc) -> 200", (status, text[:200]))
        status, _, _ = base.request(core.base + "/v1/conversations", headers=own)
        check(status in (401, 403), "/v1/conversations voi Origin cua Core van bi chan (guard cu)", status)
        status, _, _ = base.request(core.base + "/wps/v1/memory", headers=own)
        check(status not in (401, 403), "pane: /wps/v1/memory di qua kenh WPS (503 khi memory tat)", status)

        # ---- ghi cau hinh tu pane: nut toggle "Nhin tai lieu" (docs/ribbon-ui.md 5.1 - truoc day khong co
        # duong that dung sau). Ghi vao config.json dung chung voi extension LibreOffice, roi doc lai.
        status, _, text = base.request(core.base + "/wps/v1/config", "POST", {"Token": "khac"}, own)
        check(status == 400, "pane: ghi khoa bi cam (Token) -> 400", (status, text[:200]))
        status, _, text = base.request(core.base + "/wps/v1/config", "POST", {"VisualQaEnabled": True}, own)
        reply = base.as_json(text).get("result") or {}
        check(status == 200 and reply.get("saved") == ["VisualQaEnabled"], "pane: POST /wps/v1/config -> saved",
              (status, text[:300]))
        check(reply.get("current", {}).get("visualQaEnabled") is True, "pane: config tra gia tri song vua doi", reply)
        status, _, text = base.request(core.base + "/wps/v1/setup", headers=own)
        check(base.as_json(text).get("result", {}).get("current", {}).get("visualQaEnabled") is True,
              "pane: toggle doc lai duoc trang thai THAT (khong suy dien tu viec ghi)", text[:300])
        check(os.path.exists(config_path) and (os.stat(config_path).st_mode & 0o777) == 0o600,
              "pane: config.json ghi 0600 (co the chua khoa API)", config_path)
        with open(config_path, encoding="utf-8") as handle:
            check(json.load(handle).get("VisualQaEnabled") is True, "pane: config.json giu kieu JSON that", "")

        # ---- lenh truc tiep (nut "Soat bang"/"Hoan tac luot nay"): di thang ra add-in, khong qua agent.
        status, _, text = base.request(core.base + "/wps/v1/exec", "POST",
                                       {"action": "writer.getText", "params": {"maxChars": 100}}, own)
        check(status == 200 and (base.as_json(text).get("result") or {}).get("text") == SELECTION_TEXT,
              "pane: POST /wps/v1/exec -> ket qua tu add-in", (status, text[:300]))
        check(("writer.getText", {"maxChars": 100}) in addin.actions, "exec: add-in nhan lenh truc tiep",
              addin.actions)
        before = len(addin.actions)
        status, _, _ = base.request(core.base + "/wps/v1/exec", "POST", {"action": "writer.save"},
                                    {"Origin": "http://evil.example"})
        check(status == 403, "exec: Origin la -> 403 (khong chay lenh nao len tai lieu)", status)
        time.sleep(0.5)
        check(len(addin.actions) == before, "exec: lenh tu Origin la khong toi add-in", addin.actions)
        status, _, _ = base.request(core.base + "/wps/v1/exec", "POST", {}, own)
        check(status == 400, "exec: thieu action -> 400", status)

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

        # ---- khoa vung boi den: Core hoi add-in vung chon SONG (khong tin pane) va mo khoa khi luot xong.
        # SSE dong ngay khi co run.completed, con finish() (cho mo khoa) chay sau do - nen phai CHO.
        base.wait_for(lambda: "writer.unlockSelection" in [a for a, _ in addin.actions], 10)
        names = [a for a, _ in addin.actions]
        check("writer.lockSelection" in names and "writer.unlockSelection" in names,
              "khoa vung: luot chay khoa roi mo khoa vung boi den", addin.actions)
        check(names.index("writer.lockSelection") < names.index("writer.appendText"),
              "khoa vung: khoa TRUOC khi agent chay lenh dau tien", names)
        check(names.index("writer.unlockSelection") > names.index("writer.appendText"),
              "khoa vung: mo khoa SAU khi agent chay xong", names)
        check("ONLY it may change" in json.dumps(first, ensure_ascii=False),
              "khoa vung: prompt co luat chi sua trong vung boi den", "")

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
        # Lenh truc tiep giua luot chay: add-in chay tuan tu nen du lieu khong hong, nhung agent mat dung
        # trang thai tai lieu no dang theo doi -> tu choi cho xong luot.
        status, _, text = base.request(core.base + "/wps/v1/exec", "POST", {"action": "writer.save"}, own)
        check(status == 409 and "in progress" in text, "exec: dang co luot chay -> 409", (status, text[:200]))
        status, _, text = base.request(core.base + "/wps/v1/runs/%s/cancel" % run2, "POST", {}, own)
        check(status == 200 and (base.as_json(text).get("result") or {}).get("cancelled") is True,
              "pane: POST /wps/v1/runs/{id}/cancel", (status, text))
        events = sse(core.base + "/wps/v1/runs/%s/events" % run2, own, timeout=20)
        check(any(e == "run.cancelled" for e, _ in events), "pane SSE: run.cancelled", [e for e, _ in events])
        # Huy giua chung cung phai mo khoa (finish la moc duy nhat moi duong di deu qua): khoa con treo
        # lai thi luot sau bi chan oan.
        base.wait_for(lambda: [a for a, _ in addin.actions].count("writer.unlockSelection") >= 2, 5)
        check([a for a, _ in addin.actions].count("writer.unlockSelection") >= 2,
              "khoa vung: huy giua chung van mo khoa", addin.actions)

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
