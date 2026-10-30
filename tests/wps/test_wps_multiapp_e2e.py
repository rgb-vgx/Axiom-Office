"""Writer va Spreadsheets mo CUNG LUC tren WPS Linux: moi app mot phien bridge (docs/architecture/wps-bridge.md muc 2.4).

Tai hien dung loi o docs/audits/wps-linux-live-test-2026-10-28-run2.md muc 4.5: hai tien trinh WPS cung poll Core, bridge
chi co MOT phien voi app = app poll gan nhat, nen agent chay tu pane Writer lai doc bang tinh. Sau sua:
  - moi ban add-in gui clientId; client dau giu cong 47861 (cau hinh cu), client sau co cong rieng + file session rieng;
  - pane gui {client, app} -> luot chay, /exec, /status di dung client;
  - /health cua moi cong giu dung app cua no du hai app poll xen ke.

Core THAT, LLM gia (tests/core/fake_llm.py), hai add-in gia. Khong can WPS.

    python3 tests/wps/test_wps_multiapp_e2e.py
"""
from __future__ import annotations

import glob
import json
import os
import shutil
import sys
import tempfile
import threading
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import test_wps_bridge_e2e as base  # noqa: E402
from test_wps_pane_e2e import FakeLlm, payload, sse  # noqa: E402

check = base.check

WRITER = {"clientId": "cwriter1", "app": "wps",
          "document": {"name": "bao-cao.docx", "fullName": "/home/u/bao-cao.docx"},
          "commands": [{"name": "writer.getText", "kind": "wps", "agent": True, "summary": "Đọc text",
                        "params": [{"name": "maxChars", "required": False, "hint": None}]}]}
SHEET = {"clientId": "csheet1", "app": "et",
         "document": {"name": "ban-hang.xlsx", "fullName": "/home/u/ban-hang.xlsx"},
         "commands": [{"name": "et.readRange", "kind": "et", "agent": True, "summary": "Đọc vùng",
                       "params": [{"name": "range", "required": True, "hint": None},
                                  {"name": "sheet", "required": False, "hint": None}]}]}


class AppAddin(threading.Thread):
    """Mot ban add-in gia cua MOT app WPS: poll voi clientId cua no, ghi lai lenh nhan duoc."""

    def __init__(self, core, identity):
        super().__init__(daemon=True)
        self.core, self.identity, self.actions, self.port = core, identity, [], None
        self.stop_flag = threading.Event()

    def run(self):
        while not self.stop_flag.is_set():
            body = dict(self.identity, client="wps-jsaddon", version="0.1.0")
            status, _, text = base.request(self.core.base + "/wps/v1/poll", "POST", body,
                                           {"Origin": self.core.origin}, timeout=15)
            if status != 200:
                time.sleep(0.3)
                continue
            result = base.as_json(text).get("result") or {}
            self.port = result.get("port") or self.port
            for job in result.get("jobs") or []:
                self.actions.append(job.get("action"))
                if job.get("action") == "writer.getText":
                    answer = {"ok": True, "result": {"text": "Báo cáo tuần", "totalChars": 12, "truncated": False}}
                elif job.get("action") == "et.readRange":
                    answer = {"ok": True, "result": {"values": [["Doanh thu", 120]]}}
                else:
                    answer = {"ok": False, "error": "unknown action: " + str(job.get("action"))}
                base.request(self.core.base + "/wps/v1/result", "POST", dict(answer, id=job.get("id")),
                             {"Origin": self.core.origin})


def run_from_pane(core, client, app, prompt):
    """Gui luot chay nhu pane cua mot app (body mang client + app) va doc SSE toi het."""
    own = {"Origin": core.origin}
    status, _, text = base.request(core.base + "/wps/v1/runs", "POST",
                                   {"prompt": prompt, "client": client, "app": app}, own)
    run = base.as_json(text).get("result") or {}
    if status != 200 or not run.get("runId"):
        return status, text, []
    events = sse(core.base + "/wps/v1/runs/%s/events" % run["runId"], own)
    return status, text, events


def main() -> int:
    work = tempfile.mkdtemp(prefix="axiom-wps-multi-")
    addin_dir = os.path.join(work, "wps-addin")
    os.makedirs(addin_dir)
    session_dir = os.path.join(work, "sessions")
    os.makedirs(session_dir)
    llm = FakeLlm(work, [
        # Luot tu pane Writer: model doc tai lieu.
        {"tool": "office_action", "arguments": {"action": "writer.getText", "params": {}}},
        {"text": "Tài liệu nói về báo cáo tuần"},
        # Luot tu pane Spreadsheets: model doc bang.
        {"tool": "office_action", "arguments": {"action": "et.readRange", "params": {"range": "A1:B1"}}},
        {"text": "Doanh thu 120"},
    ])
    bridge_port = base.free_port()
    core = base.Core(base.core_exe(work), work, base.TOKEN, bridge_port, addin_dir, session_dir, {
        "AXIOM_LLM_PROVIDER": "openai", "AXIOM_LLM_ENDPOINT": "http://127.0.0.1:%d/v1" % llm.port,
        "AXIOM_LLM_MODEL": "fake-model", "AXIOM_LLM_API_KEY": "fake-key", "AXIOM_VERIFY_WORK": "0",
        "AXIOM_MEMORY_ENABLED": "0", "XDG_CONFIG_HOME": os.path.join(work, "xdg"),
    })
    own = {"Origin": core.origin}
    writer, sheet = AppAddin(core, WRITER), AppAddin(core, SHEET)
    try:
        writer.start()
        base.wait_for(lambda: writer.port, 10)
        sheet.start()
        base.wait_for(lambda: sheet.port, 10)
        check(writer.port == bridge_port, "client dau tien (Writer) giu cong chinh", (writer.port, bridge_port))
        check(sheet.port and sheet.port != writer.port, "Spreadsheets co cong bridge rieng", (sheet.port, writer.port))

        # /health cua moi cong giu dung app trong luc hai add-in poll xen ke (truoc day dao app moi lan poll).
        apps = set()
        for _ in range(6):
            apps.add(("w", base.as_json(base.request("http://127.0.0.1:%d/health" % writer.port)[2])
                      .get("result", {}).get("app")))
            apps.add(("e", base.as_json(base.request("http://127.0.0.1:%d/health" % sheet.port)[2])
                      .get("result", {}).get("app")))
            time.sleep(0.3)
        check(apps == {("w", "wps"), ("e", "et")}, "moi cong luon bao dung app cua no", sorted(apps))

        # Hai file session: agent/MCP/office_sessions tim thay ca hai app nhu hai bridge rieng.
        base.wait_for(lambda: len(glob.glob(os.path.join(session_dir, "*-wps*.json"))) == 2, 5)
        sessions = {}
        for path in glob.glob(os.path.join(session_dir, "*-wps*.json")):
            with open(path, encoding="utf-8") as handle:
                data = json.load(handle)
            sessions[data.get("app")] = data.get("port")
        check(sessions == {"wps": writer.port, "et": sheet.port}, "file session rieng cho tung app", sessions)

        # Luot chay tu pane Writer: agent chi lam tren Writer.
        status, text, events = run_from_pane(core, WRITER["clientId"], "wps", "Tài liệu này nói gì?")
        names = [name for name, _ in events]
        check(status == 200 and "run.completed" in names, "pane Writer: luot chay xong", (status, text[:200], names))
        started = next((payload(data) for name, data in events if name == "run.started"), {})
        check(started.get("app") == "wps" and started.get("port") == writer.port,
              "pane Writer: luot chay gan vao phien Writer", started)
        # Core tu goi writer.selection/lockSelection truoc moi luot Writer (khoa vung boi den) - van la add-in Writer.
        check("writer.getText" in writer.actions and sheet.actions == [],
              "pane Writer: lenh chi toi add-in Writer, Spreadsheets khong nhan gi", (writer.actions, sheet.actions))
        writer_before = list(writer.actions)

        # Luot chay tu pane Spreadsheets: dung phien ET.
        status, text, events = run_from_pane(core, SHEET["clientId"], "et", "Doanh thu bao nhiêu?")
        started = next((payload(data) for name, data in events if name == "run.started"), {})
        check(started.get("app") == "et" and started.get("port") == sheet.port,
              "pane Spreadsheets: luot chay gan vao phien ET", started)
        check(sheet.actions == ["et.readRange"] and writer.actions == writer_before,
              "pane Spreadsheets: lenh chi toi add-in ET", (writer.actions, sheet.actions))

        # Tool office_action cua luot ET chi liet ke lenh et.* (catalog cua dung client).
        # `enum` cua tham so action = danh sach lenh model duoc goi (mo ta tool co "writer.getText" lam vi du cu phap
        # co dinh, nen khong so chuoi trong mo ta).
        requests = llm.requests()
        tools = (requests[-1].get("body") or {}).get("tools") or [] if requests else []
        office = next((t["function"] for t in tools if (t.get("function") or {}).get("name") == "office_action"), {})
        allowed = (((office.get("parameters") or {}).get("properties") or {}).get("action") or {}).get("enum")
        check(allowed == ["et.readRange"], "luot ET: model chi duoc goi lenh cua Spreadsheets", allowed)

        # /exec va /status theo client.
        status, _, text = base.request(core.base + "/wps/v1/exec", "POST",
                                       {"action": "et.readRange", "params": {"range": "A1"},
                                        "client": SHEET["clientId"], "app": "et"}, own)
        check(status == 200 and base.as_json(text).get("ok") is True and sheet.actions[-1] == "et.readRange",
              "exec mang client ET -> chay tren ET", (status, text[:200], sheet.actions))
        status, _, text = base.request(core.base + "/wps/v1/status?app=et&client=" + SHEET["clientId"], headers=own)
        addin = (base.as_json(text).get("result") or {}).get("addin") or {}
        check(addin.get("app") == "et" and (addin.get("document") or {}).get("name") == "ban-hang.xlsx",
              "status mang client ET -> trang thai cua ET", addin)
        check(len(addin.get("clients") or []) == 2, "status liet ke ca hai app WPS", addin.get("clients"))
    finally:
        writer.stop_flag.set()
        sheet.stop_flag.set()
        core.stop()
        llm.stop()
        shutil.rmtree(work, ignore_errors=True)
    failed = [name for ok, name in base.RESULTS if not ok]
    print("\n%d passed, %d failed" % (len(base.RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
