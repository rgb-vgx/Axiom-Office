"""Test e2e cua Agent Core (New_arch.md muc 11): Core that + LLM gia + bridge gia.

    python tests/core/test_core_e2e.py            # khong can Office (bridge gia, session gia)
    python tests/core/test_core_e2e.py --office   # bridge that: tu mo Word/Excel/PowerPoint

Khong can Office cho che do mac dinh: bridge gia tra loi /health, /commands, /cmd va ghi lai lenh
nhan duoc; LLM gia (fake_llm.py) tra loi theo kich ban. Kiem tra: thu tu su kien SSE, hoi thoai,
audit, allowlist, huy, tran token, loi office, va (che do --office) tai lieu that su doi.

Chi dung thu vien chuan.
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
CORE_EXE = os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Core.exe")
FAKE_LLM = os.path.join(ROOT, "tests", "core", "fake_llm.py")

RESULTS: list[tuple[bool, str, str]] = []


def check(ok, name, detail=""):
    RESULTS.append((bool(ok), name, str(detail)))
    print(("PASS " if ok else "FAIL ") + name + ("" if ok or not detail else "\n     " + str(detail)[:700]))


def free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def dotnet_root() -> str:
    candidates = [
        os.environ.get("DOTNET_ROOT"),
        os.path.join(os.environ.get("LOCALAPPDATA", ""), "Microsoft", "dotnet"),
        r"C:\Program Files\dotnet",
    ]
    for candidate in candidates:
        if candidate and os.path.isdir(candidate):
            return candidate
    raise SystemExit("Khong tim thay .NET runtime root (dat DOTNET_ROOT)")


def http_json(url, method="GET", body=None, token=None, timeout=30):
    data = None if body is None else json.dumps(body, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(url, data=data, method=method)
    request.add_header("Content-Type", "application/json; charset=utf-8")
    if token:
        request.add_header("X-Auth-Token", token)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.status, json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as ex:
        return ex.code, json.loads(ex.read().decode("utf-8") or "null")


def sse_events(url, token, timeout=120, stop_after_terminal=True):
    """Doc SSE den khi gap su kien ket thuc luot chay (hoac het timeout)."""
    events = []
    request = urllib.request.Request(url)
    request.add_header("X-Auth-Token", token)
    request.add_header("Accept", "text/event-stream")
    deadline = time.time() + timeout
    with urllib.request.urlopen(request, timeout=timeout) as response:
        event_type = None
        while time.time() < deadline:
            line = response.readline()
            if not line:
                break
            text = line.decode("utf-8").rstrip("\n")
            if text.startswith("event:"):
                event_type = text[6:].strip()
                continue
            if not text.startswith("data:"):
                continue
            payload = json.loads(text[5:].strip())
            if event_type == "ping":
                continue
            events.append((event_type, payload))
            if stop_after_terminal and event_type in ("run.completed", "run.failed", "run.cancelled", "run.timedout", "run.stopped"):
                break
    return events


COMMANDS = [
    {"name": "et.writeRange", "kind": "et", "agent": True, "summary": "ghi vung",
     "params": [{"name": "range", "required": True, "hint": "top-left cell e.g. 'A1'"}, {"name": "values", "required": True, "hint": "2D array of rows"}]},
    {"name": "et.save", "kind": "et", "agent": False, "summary": "luu", "params": []},
    {"name": "writer.closeAll", "kind": "wps", "agent": False, "summary": "dong het", "params": []},
]


class FakeBridge:
    """Bridge gia: /health, /commands, /cmd (ghi lai lenh nhan duoc)."""

    def __init__(self, pid: int, app: str = "et"):
        self.pid = pid
        self.app = app
        self.commands: list[dict] = []
        self.port = free_port()
        outer = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *args):
                pass

            def _send(self, payload, status=200):
                body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def do_GET(self):
                if self.path == "/health":
                    self._send({"ok": True, "result": {"app": outer.app, "pid": outer.pid, "port": outer.port, "version": "1.0.0"}})
                elif self.path == "/commands":
                    self._send({"ok": True, "result": {"version": "1.0.0", "commands": COMMANDS}})
                else:
                    self._send({"ok": False, "error": "not found"}, 404)

            def do_POST(self):
                length = int(self.headers.get("Content-Length", "0"))
                body = json.loads(self.rfile.read(length).decode("utf-8") or "{}") if length else {}
                if self.path == "/cmd":
                    outer.commands.append(body)
                    action = body.get("action", "")
                    self._send({"ok": True, "result": {"action": action, "written": 2}})
                else:
                    self._send({"ok": False, "error": "not found"}, 404)

        self.server = ThreadingHTTPServer(("127.0.0.1", self.port), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def stop(self):
        self.server.shutdown()
        self.server.server_close()

    def actions(self) -> list[str]:
        return [item.get("action") for item in self.commands]


class FakeLlm:
    """Chay fake_llm.py trong tien trinh rieng voi kich ban cho truoc."""

    def __init__(self, work_dir: str, script: list[dict]):
        self.port = free_port()
        self.script_path = os.path.join(work_dir, f"script-{uuid.uuid4().hex[:6]}.json")
        self.requests_path = os.path.join(work_dir, f"requests-{uuid.uuid4().hex[:6]}.jsonl")
        with open(self.script_path, "w", encoding="utf-8") as handle:
            json.dump(script, handle, ensure_ascii=False)
        self.process = subprocess.Popen(
            [sys.executable, FAKE_LLM, "--port", str(self.port), "--script", self.script_path, "--requests", self.requests_path],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        deadline = time.time() + 15
        while time.time() < deadline:
            try:
                status, _ = http_json(f"http://127.0.0.1:{self.port}/__state", timeout=1)
                if status == 200:
                    return
            except Exception:
                time.sleep(0.2)
        raise SystemExit("fake LLM khong khoi dong duoc")

    def stop(self):
        self.process.terminate()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill()

    def requests(self) -> list[dict]:
        if not os.path.exists(self.requests_path):
            return []
        with open(self.requests_path, encoding="utf-8") as handle:
            return [json.loads(line) for line in handle if line.strip()]


class Core:
    """AxiomOffice.Core.exe that voi cau hinh AXIOM_* tro vao bridge gia va LLM gia."""

    def __init__(self, data_dir: str, session_dir: str | None, llm: FakeLlm, token: str):
        self.data_dir = data_dir
        self.session_dir = session_dir
        self.token = token
        env = os.environ.copy()
        env.update({
            "DOTNET_ROOT": dotnet_root(),
            "AXIOM_CORE_DATA_DIR": data_dir,
            "AXIOM_CORE_PORT": "0",
            "AXIOM_CORE_SINGLE_INSTANCE": "0",
            "AXIOM_CORE_MUTEX_NAME": "Local\\AxiomOffice.Core.E2E." + uuid.uuid4().hex[:8],
            "AXIOM_TOKEN": token,
            "AXIOM_LLM_PROVIDER": "openai",
            "AXIOM_LLM_ENDPOINT": f"http://127.0.0.1:{llm.port}/v1",
            "AXIOM_LLM_MODEL": "fake-model",
            "AXIOM_LLM_API_KEY": "fake-key",
        })
        if session_dir:
            env["AXIOM_SESSION_DIR"] = session_dir
        self.log_path = os.path.join(data_dir, "core.out")
        self.log = open(self.log_path, "w", encoding="utf-8")
        self.process = subprocess.Popen([CORE_EXE], env=env, cwd=data_dir, stdout=self.log, stderr=subprocess.STDOUT)

        deadline = time.time() + 40
        self.info = None
        self.port = 0
        while time.time() < deadline:
            if self.process.poll() is not None:
                raise SystemExit(f"Core thoat som (exit {self.process.returncode}):\n{open(self.log_path, encoding='utf-8').read()}")
            path = os.path.join(data_dir, "core.json")
            if os.path.exists(path):
                try:
                    with open(path, encoding="utf-8") as handle:
                        self.info = json.load(handle)
                    self.port = self.info["port"]
                    status, _ = http_json(f"http://127.0.0.1:{self.port}/health", timeout=2)
                    if status == 200:
                        return
                except Exception:
                    pass
            time.sleep(0.2)
        raise SystemExit("Core khong san sang")

    @property
    def base(self) -> str:
        return f"http://127.0.0.1:{self.port}"

    def stop(self):
        try:
            http_json(self.base + "/v1/admin/shutdown", method="POST", token=self.token, timeout=5)
        except Exception:
            pass
        try:
            self.process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.process.kill()
        self.log.close()

    def run(self, prompt, office, conversation_id=None, document=None, options=None, token_missing=False):
        body = {"prompt": prompt, "office": office}
        if conversation_id:
            body["conversationId"] = conversation_id
        if document:
            body["document"] = document
        if options:
            body["options"] = options
        return http_json(self.base + "/v1/runs", method="POST", body=body, token=None if token_missing else self.token)

    def events(self, run_id, timeout=120):
        return sse_events(f"{self.base}/v1/runs/{run_id}/events", self.token, timeout=timeout)

    def get_run(self, run_id):
        return http_json(f"{self.base}/v1/runs/{run_id}", token=self.token)[1]["result"]

    def conversation(self, conversation_id):
        return http_json(f"{self.base}/v1/conversations/{conversation_id}", token=self.token)[1]["result"]


def write_session(session_dir: str, pid: int, port: int, app: str, document: str) -> None:
    os.makedirs(session_dir, exist_ok=True)
    payload = {
        "pid": pid,
        "app": app,
        "family": "office",
        "port": port,
        "host": "EXCEL.EXE" if app == "et" else "WINWORD.EXE",
        "version": "1.0.0",
        "lastSeenEpoch": time.time(),
        "document": os.path.basename(document),
        "documentPath": document,
    }
    with open(os.path.join(session_dir, f"{pid}.json"), "w", encoding="utf-8") as handle:
        json.dump(payload, handle)


def test_fake_bridge(core: Core, bridge: FakeBridge, llm: FakeLlm, document_path: str) -> None:
    office = {"port": bridge.port, "pid": bridge.pid, "app": "et", "family": "office"}
    document = {"name": os.path.basename(document_path), "fullName": document_path}

    # 1. Mot luot day du: goi tool roi tra loi.
    llm_script = [
        {"tool": "office_action", "arguments": {"action": "et.writeRange", "params": {"range": "A1", "values": [["a", 1]]}}},
        {"text": "Da ghi xong bang diem"},
    ]
    status, reply = core.run("ghi bang diem", office, document=document)
    check(status == 200 and reply["ok"] is True, "POST /v1/runs tao luot chay", reply)
    if not reply.get("ok"):
        return

    run_id = reply["result"]["runId"]
    conversation_id = reply["result"]["conversationId"]
    check(bool(run_id) and bool(conversation_id), "tra ve runId + conversationId", reply["result"])

    events = core.events(run_id)
    types = [item[0] for item in events]
    check(types == ["run.started", "tool.started", "tool.finished", "run.completed"], "thu tu su kien SSE", types)
    check(events[1][1]["data"]["action"] == "et.writeRange", "tool.started co action", events[1][1]["data"])
    check(events[2][1]["data"]["ok"] is True and events[2][1]["data"]["resultPreview"], "tool.finished co ket qua",
          events[2][1]["data"])

    check(bridge.actions() == ["et.writeRange"], "bridge nhan dung lenh", bridge.actions())
    check(bridge.commands[0].get("params", {}).get("range") == "A1", "tham so nguyen van xuong bridge", bridge.commands[0])

    run = core.get_run(run_id)
    check(run["status"] == "completed" and run["reply"] == "Da ghi xong bang diem", "GET /v1/runs/{id} co ket qua", run)
    check(run["rounds"] == 2 and run["toolCalls"] == 1, "dem vong va tool call", run)

    conversation = core.conversation(conversation_id)
    roles = [message["role"] for message in conversation["messages"]]
    check(roles == ["user", "tool_summary", "assistant"], "hoi thoai luu user + tom tat tool + assistant", roles)
    check(conversation["documentKey"] == document_path.lower(), "hoi thoai gan dung tai lieu", conversation["documentKey"])
    check(conversation["messages"][1]["content"].startswith("et.writeRange"), "tom tat tool co ten lenh",
          conversation["messages"][1]["content"])

    # 2. Luot tiep theo cua cung hoi thoai: prompt phai mang ngu canh luot truoc.
    before = len(llm.requests())
    status, reply2 = core.run("them cot trung binh", office, conversation_id=conversation_id, document=document)
    check(status == 200 and reply2["result"]["conversationId"] == conversation_id, "luot tiep dung lai hoi thoai", reply2["result"])
    core.events(reply2["result"]["runId"])
    sent = llm.requests()[before:]
    prompt_text = json.dumps([item["body"] for item in sent], ensure_ascii=False)
    check("ghi bang diem" in prompt_text and "Da ghi xong bang diem" in prompt_text,
          "prompt luot 2 co ngu canh luot 1", prompt_text[:300])

    # 3. Tim hoi thoai theo tai lieu (pane dung khi mo lai tai lieu).
    status, listing = http_json(core.base + "/v1/conversations?limit=1&documentKey=" + urllib.parse.quote(document_path.lower()),
                               token=core.token)
    check(status == 200 and listing["result"]["conversations"][0]["id"] == conversation_id,
          "GET /v1/conversations?documentKey tra hoi thoai cua tai lieu", listing["result"])


def test_guards(core: Core, bridge: FakeBridge, document_path: str) -> None:
    office = {"port": bridge.port, "pid": bridge.pid, "app": "et", "family": "office"}
    document = {"name": os.path.basename(document_path), "fullName": document_path}

    # Thieu token -> 401.
    status, body = core.run("x", office, token_missing=True)
    check(status == 401 and body["ok"] is False, "thieu token -> 401", body)

    # Port khong co session -> run.failed kind=office.
    status, reply = core.run("x", {"port": 47911, "pid": 1, "app": "et", "family": "office"})
    run_id = reply["result"]["runId"]
    events = core.events(run_id)
    check(events[-1][0] == "run.failed" and events[-1][1]["data"]["kind"] == "office", "port la -> run.failed kind=office",
          events[-1][1]["data"])

    # Lenh ngoai danh sach: KHONG xuong bridge, tool.finished bao loi, luot chay van xong.
    before = len(bridge.commands)
    status, reply = core.run("x", office, document=document)
    run_id = reply["result"]["runId"]
    events = core.events(run_id)
    finished = [item for item in events if item[0] == "tool.finished"]
    check(len(bridge.commands) == before, "allowlist chan lenh -> bridge khong nhan lenh nao", bridge.actions()[before:])
    check(len(finished) == 1 and finished[0][1]["data"]["ok"] is False
          and "not an available action" in (finished[0][1]["data"]["error"] or ""),
          "tool.finished bao lenh khong duoc phep", finished[0][1]["data"] if finished else events)
    check(events[-1][0] == "run.completed", "luot chay van ket thuc binh thuong sau khi bi tu choi", [e[0] for e in events])


def live_bridge_cmd(port: int, action: str, params=None, timeout: int = 90):
    """Goi thang bridge that trong app (nhu test live) de chuan bi va doc lai tai lieu."""
    request = urllib.request.Request(
        f"http://127.0.0.1:{port}/cmd",
        data=json.dumps({"action": action, "params": params or {}}, ensure_ascii=False).encode("utf-8"),
        method="POST",
    )
    request.add_header("Content-Type", "application/json; charset=utf-8")
    token = subprocess.run(
        ["powershell", "-NoProfile", "-Command",
         "(Get-ItemProperty -Path 'HKCU:\\Software\\AxiomOffice' -Name Token -ErrorAction SilentlyContinue).Token"],
        capture_output=True, text=True).stdout.strip()
    if token:
        request.add_header("X-Auth-Token", token)
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


def test_office(core: Core, port: int, pid: int) -> None:
    """Che do --office: Core chay lenh that len Excel dang mo va tai lieu that su doi."""
    live_bridge_cmd(port, "et.newWorkbook")
    office = {"port": port, "pid": pid, "app": "et", "family": "office"}

    status, reply = core.run(
        "Ghi bang diem vao A1:B3 cua sheet dang mo",
        office,
        document={"name": "Book1", "fullName": ""},
    )
    check(status == 200 and reply["ok"] is True, "tao luot chay tren Excel that", reply)
    if not reply.get("ok"):
        return

    events = core.events(reply["result"]["runId"])
    types = [item[0] for item in events]
    check(types[-1] == "run.completed", "luot chay tren Excel that xong",
          events[-1][1].get("data") if events else types)

    values = live_bridge_cmd(port, "et.readRange", {"range": "A1:B3"})
    check(values["ok"] is True, "doc lai vung vua ghi", values)
    rows = values["result"]["values"] if values.get("ok") else []
    check(rows and rows[0][:2] == ["Ten", "Diem"], "tai lieu that su doi (A1:B1)", rows)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--office", action="store_true", help="dung Office that thay vi bridge gia")
    parser.add_argument("--keep", action="store_true", help="giu thu muc tam de xem log")
    args = parser.parse_args()

    work = os.path.join(tempfile.gettempdir(), "axiom-core-e2e-" + uuid.uuid4().hex[:8])
    os.makedirs(work, exist_ok=True)
    data_dir = os.path.join(work, "core")
    session_dir = os.path.join(work, "sessions")
    os.makedirs(data_dir, exist_ok=True)
    document_path = os.path.join(work, "bao-cao.xlsx")
    token = "e2e-token-" + uuid.uuid4().hex[:8]

    print("work dir:", work)
    bridge = None
    launched = None
    llm = None
    core = None
    try:
        office_port = 0
        office_pid = 0
        if args.office:
            sys.path.insert(0, os.path.join(ROOT, "tests", "live"))
            import test_live_commands as live  # noqa: E402

            office_port, office_pid = live.launch("excel", use_wps=False)
            launched = office_pid
            bridge = None
            # Bridge that dung token trong HKCU: Core phai dung cung token do.
            token = live.token()
        else:
            bridge = FakeBridge(pid=os.getpid())
            office_port, office_pid = bridge.port, bridge.pid
            write_session(session_dir, os.getpid(), bridge.port, "et", document_path)

        llm = FakeLlm(work, [
            {"tool": "office_action", "arguments": {"action": "et.writeRange", "params": {"range": "A1", "values": [["Ten", "Diem"], ["An", 9]]}}},
            {"text": "Da ghi xong bang diem"},
            {"tool": "office_action", "arguments": {"action": "et.writeRange", "params": {"range": "A3", "values": [["Binh", 8]]}}},
            {"text": "Da them dong"},
            {"tool": "office_action", "arguments": {"action": "writer.closeAll"}},
            {"text": "Lenh do khong duoc phep"},
        ])
        # Che do --office dung session registry that cua add-in (khong tro vao thu muc tam).
        core = Core(data_dir, None if args.office else session_dir, llm, token)

        status, health = http_json(core.base + "/health")
        check(status == 200 and health["result"]["memory"] == "on", "GET /health bao memory on", health["result"])
        check(health["result"]["version"] and health["result"]["protocol"] == 1, "version + protocol trong /health",
              health["result"])

        if args.office:
            test_office(core, office_port, office_pid)
        else:
            test_fake_bridge(core, bridge, llm, document_path)
            test_guards(core, bridge, document_path)
    finally:
        if core is not None:
            core.stop()
        if llm is not None:
            llm.stop()
        if bridge is not None:
            bridge.stop()
        if launched:
            # /T /F: dong ca tien trinh con, khong de lai Excel mo (test tu mo thi test tu dong).
            subprocess.run(["taskkill", "/PID", str(launched), "/T", "/F"], capture_output=True)
        if not args.keep:
            shutil.rmtree(work, ignore_errors=True)

    failed = [item for item in RESULTS if not item[0]]
    print("\n%d passed, %d failed" % (len(RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
