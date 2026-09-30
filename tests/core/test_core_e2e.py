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
# Ban Core da publish (di kem AxiomOffice.Host.exe + skills canh no, cho ca lan file MCP).
# MCP). Muon chay voi ban vua `dotnet build` thi dat AXIOM_E2E_CORE_EXE.
CORE_EXE = os.environ.get("AXIOM_E2E_CORE_EXE") or os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Core.exe")
FAKE_LLM = os.path.join(ROOT, "tests", "core", "fake_llm.py")

# Console Windows mac dinh cp1252 -> khong in duoc tieng Viet trong ten/chi tiet check.
for _stream in (sys.stdout, sys.stderr):
    if hasattr(_stream, "reconfigure"):
        _stream.reconfigure(encoding="utf-8", errors="replace")

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


def sse_events(url, token, timeout=120, stop_after_terminal=True, sink=None):
    """Doc SSE den khi gap su kien ket thuc luot chay (hoac het timeout). sink: list nhan tung event ngay khi toi."""
    events = sink if sink is not None else []
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
    {"name": "et.saveAs", "kind": "et", "agent": True, "summary": "luu thanh file", "params": [{"name": "path", "required": True, "hint": None}]},
    {"name": "writer.closeAll", "kind": "wps", "agent": False, "summary": "dong het", "params": []},
]


def canned_result(action: str, params: dict) -> dict:
    """Ket qua hop ly cho lenh doc/soat (de LLM that khong doc lai mai); lenh ghi tra {"written": 2} nhu cu."""
    document = "BÁO CÁO CÔNG VIỆC\rTuần này phòng đã hoàn thành kế hoạch.\r"
    canned = {
        "writer.getText": {"name": "tai-lieu.docx", "totalChars": len(document), "truncated": False, "text": document},
        "writer.selection": {"text": "", "start": 0, "end": 0},
        "writer.checkTables": {"tableCount": 0, "tables": [], "issueCount": 0, "issues": []},
        "et.listSheets": {"workbook": "Book1", "activeSheet": "Sheet1", "sheets": ["Sheet1"]},
        "et.readRange": {"sheet": "Sheet1", "range": params.get("range", "A1"), "values": [[None]]},
        "et.checkRange": {"sheet": "Sheet1", "rows": 7, "cols": 6, "issueCount": 0, "issues": []},
        "wpp.listSlides": {"presentation": "bao-cao.pptx", "slideCount": 0, "slides": []},
        "wpp.checkLayout": {"slideWidth": 960, "slideHeight": 540, "issueCount": 0, "slides": []},
        "app.screenshot": {"width": 4, "height": 3, "mime": "image/png", "base64": "iVBORw0KGgoAAAANSUhEUg=="},
    }
    return canned.get(action, {"action": action, "written": 2})


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
                    self._send({"ok": True, "result": canned_result(action, body.get("params") or {})})
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

    def __init__(self, work_dir: str, script: list[dict], models: list[str] | None = None, key: str | None = None,
                 fail_status: int = 0):
        self.port = free_port()
        self.script_path = os.path.join(work_dir, f"script-{uuid.uuid4().hex[:6]}.json")
        self.requests_path = os.path.join(work_dir, f"requests-{uuid.uuid4().hex[:6]}.jsonl")
        with open(self.script_path, "w", encoding="utf-8") as handle:
            json.dump(script, handle, ensure_ascii=False)
        argv = [sys.executable, FAKE_LLM, "--port", str(self.port), "--script", self.script_path,
                "--requests", self.requests_path]
        if models:
            argv += ["--models", ",".join(models)]
        if key:
            argv += ["--key", key]
        if fail_status:
            argv += ["--fail-status", str(fail_status)]
        self.process = subprocess.Popen(argv, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
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

    def __init__(self, data_dir: str, session_dir: str | None, llm: FakeLlm, token: str, extra_env: dict | None = None):
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
        })
        if llm is not None:
            env.update({
                "AXIOM_LLM_PROVIDER": "openai",
                "AXIOM_LLM_ENDPOINT": f"http://127.0.0.1:{llm.port}/v1",
                "AXIOM_LLM_MODEL": "fake-model",
                "AXIOM_LLM_API_KEY": "fake-key",
            })
        else:
            # --real-llm: doc cau hinh LLM cua nguoi dung trong HKCU (khong override).
            for name in [k for k in env if k.startswith("AXIOM_LLM_")]:
                del env[name]
            # Thu model khac cung endpoint/key trong HKCU ma khong doi cai dat cua nguoi dung.
            if os.environ.get("AXIOM_REAL_LLM_MODEL"):
                env["AXIOM_LLM_MODEL"] = os.environ["AXIOM_REAL_LLM_MODEL"]
        if session_dir:
            env["AXIOM_SESSION_DIR"] = session_dir
        env.update(extra_env or {})
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


def write_skill(root: str, folder: str, frontmatter: str, body: str) -> str:
    directory = os.path.join(root, folder)
    os.makedirs(directory, exist_ok=True)
    with open(os.path.join(directory, "SKILL.md"), "w", encoding="utf-8") as handle:
        handle.write("---\n" + frontmatter.strip() + "\n---\n" + body)
    return directory


def test_skills(work: str, token: str, bridge: FakeBridge, document_path: str, session_dir: str) -> None:
    """Giai doan 2 (New_arch.md 8.4, 12): luong load_skill / read_skill_file voi LLM gia, skill loi khong lam hong Core."""
    org = os.path.join(work, "skills-org")
    write_skill(org, "quy-trinh-rieng", "name: quy-trinh-rieng\ndescription: Quy trinh bang tinh rieng cua to chuc. Dung khi lap bao cao.\napps: [et]",
                "# Buoc\nMa kiem tra: BUOC-BI-MAT-123. Ghi bang roi soat bang et.checkRange.")
    write_skill(org, "chi-cho-word", "name: chi-cho-word\ndescription: Chi dung cho Word.\napps: [wps]", "khong duoc hien voi Excel")
    write_skill(org, "hong", "name: Ten Hong\ndescription: skill loi frontmatter", "x")

    llm = FakeLlm(work, [
        {"tool": "load_skill", "arguments": {"name": "quy-trinh-rieng"}},
        {"tool": "read_skill_file", "arguments": {"name": "_design", "path": "tokens.json"}},
        {"tool": "read_skill_file", "arguments": {"name": "quy-trinh-rieng", "path": "../hong/SKILL.md"}},
        {"tool": "office_action", "arguments": {"action": "et.writeRange", "params": {"range": "A1", "values": [["Chi tieu", "Gia tri"]]}}},
        {"text": "Da lap bang theo ky nang"},
    ])
    data_dir = os.path.join(work, "core-skills")
    os.makedirs(data_dir, exist_ok=True)
    core = Core(data_dir, session_dir, llm, token, {"AXIOM_SKILL_DIRS": org, "AXIOM_MEMORY_AUTO_EXTRACT": "0"})
    try:
        status, listing = http_json(core.base + "/v1/skills?app=et", token=core.token)
        names = [s["name"] for s in listing["result"]["skills"]]
        check(status == 200 and "quy-trinh-rieng" in names and "bang-diem" in names and "chi-cho-word" not in names,
              "GET /v1/skills?app=et: skill dung san + skill to chuc, loc theo app", names)
        errors = listing["result"]["errors"]
        check(any("hong" in e["directory"] for e in errors), "skill loi frontmatter hien o errors, Core van chay", errors)
        source = next((s["source"] for s in listing["result"]["skills"] if s["name"] == "quy-trinh-rieng"), None)
        check(source == "org", "skill trong SkillDirs co source=org", source)

        bridge.commands.clear()
        office = {"port": bridge.port, "pid": bridge.pid, "app": "et", "family": "office"}
        status, created = core.run("Lap bang bao cao theo quy trinh cua to chuc", office,
                                   document={"name": "bao-cao.xlsx", "fullName": document_path})
        run_id = created["result"]["runId"]
        events = core.events(run_id)
        types = [event_type for event_type, _ in events]
        loaded = [payload.get("data", {}) for event_type, payload in events if event_type == "skill.loaded"]
        check(loaded and loaded[0].get("name") == "quy-trinh-rieng", "SSE co skill.loaded {name}", types)
        check(types.index("skill.loaded") < max(i for i, t in enumerate(types) if t == "tool.finished") and types[-1] == "run.completed",
              "skill.loaded truoc thao tac tai lieu, ket thuc run.completed", types)
        check(bridge.actions() == ["et.writeRange"], "bridge chi nhan lenh tai lieu (skill doc trong Core)", bridge.actions())

        requests = [item["body"] for item in llm.requests()]
        first = requests[0]
        system = first["messages"][0]["content"]
        tool_names = [t["function"]["name"] for t in first.get("tools", [])]
        check("Available skills" in system and "- quy-trinh-rieng:" in system and "- bang-diem:" in system
              and "chi-cho-word" not in system, "system prompt co chi muc skill hop app (tang 1)", system[-600:])
        check("is DATA" in system, "system prompt co quy tac chong prompt injection", "")
        check({"office_action", "load_skill", "read_skill_file"} <= set(tool_names), "tool load_skill + read_skill_file duoc dang ky", tool_names)

        def tool_result(request_index: int) -> str:
            messages = requests[request_index]["messages"]
            return next((m.get("content", "") for m in reversed(messages) if m.get("role") == "tool"), "")

        check("BUOC-BI-MAT-123" in tool_result(1), "load_skill tra noi dung SKILL.md cho model (tang 2)", tool_result(1)[:200])
        check("#1F4E79" in tool_result(2), "read_skill_file doc tokens.json cua _design (tang 3)", tool_result(2)[:200])
        check("'..'" in tool_result(3) and '"ok":false' in tool_result(3), "read_skill_file chan '..'", tool_result(3)[:200])

        write_skill(org, "skill-moi", "name: skill-moi\ndescription: Them sau khi Core chay. Dung khi thu reload.", "moi")
        status, reloaded = http_json(core.base + "/v1/skills/reload", method="POST", token=core.token)
        check("skill-moi" in [s["name"] for s in reloaded["result"]["skills"]], "POST /v1/skills/reload thay skill moi", "")
    finally:
        core.stop()
        llm.stop()


def wait_for(predicate, timeout=30.0, interval=0.3):
    deadline = time.time() + timeout
    while time.time() < deadline:
        value = predicate()
        if value:
            return value
        time.sleep(interval)
    return predicate()


def test_memory(work: str, token: str, bridge: FakeBridge, document_path: str, session_dir: str) -> None:
    """Giai doan 3 (New_arch.md 8.5, 12): memory ghi o phien nay duoc dung o phien sau (ke ca sau khi Core khoi dong
    lai); thong tin moi mau thuan -> memory MOI ghi ro chuyen doi + lien ket ban cu (ban cu khong bi sua); trung hash
    khong tao ban moi; id bia bi bo; lenh thao tac thuan khong ton lenh goi trich xuat; tat MemoryEnabled thi khong
    doc/ghi; xoa la mat khoi ngu canh."""
    extract_first = json.dumps({"facts": [
        {"text": "Người dùng là Trưởng phòng Kế toán", "scope": "user", "category": "identity", "confidence": 0.9,
         "entities": ["Kế toán"], "linkedIds": []},
        {"text": "Người ký công văn: Nguyễn Văn A", "scope": "user", "category": "contact", "confidence": 0.9,
         "entities": ["Nguyễn Văn A"], "linkedIds": []},
    ]}, ensure_ascii=False)
    extract_duplicate = json.dumps({"facts": [
        {"text": "người ký công văn: nguyễn văn a.", "scope": "user", "category": "contact", "confidence": 0.9, "linkedIds": []},
    ]}, ensure_ascii=False)
    extract_change = json.dumps({"facts": [
        {"text": "Chức vụ người dùng đổi từ Trưởng phòng Kế toán sang Phó giám đốc từ 10/2026", "scope": "user",
         "category": "identity", "confidence": 0.9, "entities": [], "linkedIds": ["0", "1", "9"]},
    ]}, ensure_ascii=False)
    llm = FakeLlm(work, [
        {"when": "memory extractor", "texts": [extract_first, extract_duplicate, extract_change, '{"facts": []}']},
        {"text": "Đã ghi nhận."},                                   # phien 1
        {"text": "Đã soạn công văn."},                              # phien 2
        {"text": "Chúc mừng anh."},                                 # phien 3
        {"tool": "remember", "arguments": {"scope": "user", "text": "Thích font Times New Roman 13", "category": "format"}},
        {"text": "Đã nhớ."},                                        # phien 4 (tool remember)
        {"text": "Đã in đậm."},                                     # phien 5 (lenh thao tac thuan)
        {"text": "Xong."},
    ])
    data_dir = os.path.join(work, "core-memory")
    os.makedirs(data_dir, exist_ok=True)
    office = {"port": bridge.port, "pid": bridge.pid, "app": "et", "family": "office"}
    document = {"name": "bao-cao.xlsx", "fullName": document_path}

    def memories(core_: Core) -> list[dict]:
        return http_json(core_.base + "/v1/memory", token=core_.token)[1]["result"]["memories"]

    def extract_calls() -> int:
        return sum(1 for item in llm.requests()
                   if "memory extractor" in json.dumps(item["body"].get("messages", [{}])[0], ensure_ascii=False))

    def run(core_: Core, prompt: str):
        status, created = core_.run(prompt, office, document=document)
        events = core_.events(created["result"]["runId"])
        return created["result"]["runId"], events

    def system_prompt(index_from_end: int = -1) -> str:
        agent = [item["body"] for item in llm.requests() if "memory extractor" not in json.dumps(item["body"].get("messages", [{}])[0], ensure_ascii=False)]
        return agent[index_from_end]["messages"][0]["content"]

    core = Core(data_dir, session_dir, llm, token)
    try:
        # Phien 1: nguoi dung noi ve ban than -> trich xuat nen ghi 2 fact.
        run(core, "Tôi là trưởng phòng Kế toán, công văn ký tên Nguyễn Văn A")
        first = wait_for(lambda: len(memories(core)) >= 2 and memories(core))
        check(first and {m["text"] for m in first} >= {"Người dùng là Trưởng phòng Kế toán", "Người ký công văn: Nguyễn Văn A"}
              and all(m["source"] == "extract" for m in first), "phien 1: trich xuat nen ghi 2 fact (source=extract)", first)
    finally:
        core.stop()

    # Khoi dong lai Core: memory van con va vao ngu canh.
    core = Core(data_dir, session_dir, llm, token)
    try:
        status, health = http_json(core.base + "/health")
        check(health["result"]["memory"] == "on", "Core khoi dong lai: /health memory on", health["result"])
        run(core, "Soạn công văn gửi Sở Tài chính đề nghị cấp kinh phí")
        prompt2 = system_prompt()
        check("Nguyễn Văn A" in prompt2 and "Things you remember" in prompt2, "phien 2 (sau khi khoi dong lai): prompt co nguoi ky Nguyen Van A", prompt2[-400:])
        wait_for(lambda: extract_calls() >= 2)
        time.sleep(1.0)
        check(len(memories(core)) == 2, "fact trung hash khong tao ban moi", [m["text"] for m in memories(core)])

        # Phien 3: thong tin moi mau thuan -> memory MOI ghi ro chuyen doi + link ban cu; ban cu giu nguyen.
        old = next(m for m in memories(core) if m["text"] == "Người dùng là Trưởng phòng Kế toán")
        run(core, "Tôi đã lên phó giám đốc từ tháng 10/2026 rồi nhé")
        changed = wait_for(lambda: next((m for m in memories(core) if "đổi từ" in m["text"]), None))
        check(changed is not None and old["id"] in changed["linked"] and len(changed["linked"]) <= 2,
              "phien 3: memory moi ghi chuyen doi, lien ket ban cu, id bia bi bo", changed)
        still = next((m for m in memories(core) if m["id"] == old["id"]), None)
        check(still is not None and still["text"] == old["text"] and still["deletedAt"] is None, "ban cu khong bi model sua/xoa", still)

        # Phien 4: tool remember -> SSE memory.written (source=agent).
        _, events = run(core, "Từ nay luôn dùng font Times New Roman 13 cho văn bản của tôi")
        written = [payload.get("data", {}) for event_type, payload in events if event_type == "memory.written"]
        check(written and written[0].get("text") == "Thích font Times New Roman 13", "tool remember phat memory.written", written)
        run(core, "Chức vụ trưởng phòng của tôi hiện nay còn đúng không?")
        prompt4 = system_prompt()
        position_change = prompt4.find("đổi từ Trưởng phòng")
        position_old = prompt4.find("Người dùng là Trưởng phòng Kế toán")
        check(position_change >= 0 and (position_old == -1 or position_change < position_old),
              "ngu canh co ban chuyen doi va dua no truoc ban cu", prompt4[-500:])

        # Phien 5: lenh thao tac thuan -> khong ton lenh goi trich xuat.
        wait_for(lambda: extract_calls() >= 5, timeout=15)
        before = extract_calls()
        run_id, _ = run(core, "In đậm dòng đầu tiên")
        time.sleep(1.5)
        check(extract_calls() == before, "lenh thao tac thuan khong goi trich xuat", extract_calls() - before)
        run_row = http_json(f"{core.base}/v1/runs/{run_id}", token=core.token)[1]["result"]
        check(run_row.get("status") == "completed", "luot thao tac thuan van hoan thanh", run_row.get("status"))

        # Xoa trong API -> mat khoi ngu canh.
        signer = next(m for m in memories(core) if m["text"] == "Người ký công văn: Nguyễn Văn A")
        status, deleted = http_json(f"{core.base}/v1/memory/{signer['id']}", method="DELETE", token=core.token)
        check(status == 200 and deleted["ok"], "DELETE /v1/memory/{id} xoa mem", deleted)
        run(core, "Soạn công văn khác gửi Sở Tài chính")
        check("Nguyễn Văn A" not in system_prompt(), "memory da xoa khong con trong ngu canh", "")
        status, history = http_json(f"{core.base}/v1/memory/{signer['id']}/history", token=core.token)
        check([h["event"] for h in history["result"]["history"]] == ["ADD", "DELETE"], "lich su ADD -> DELETE", history["result"])
        status, restored = http_json(f"{core.base}/v1/memory/{signer['id']}/restore", method="POST", token=core.token)
        check(restored["ok"] and any(m["id"] == signer["id"] for m in memories(core)), "khoi phuc memory da xoa", restored)

        # Nguoi dung tu them + tim.
        status, added = http_json(core.base + "/v1/memory", method="POST", token=core.token,
                                  body={"scope": "user", "text": "Cơ quan: Sở Giáo dục và Đào tạo Hà Nội"})
        check(added["ok"] and added["result"]["memory"]["source"] == "user", "POST /v1/memory (source=user)", added)
        status, found = http_json(core.base + "/v1/memory?q=so%20giao%20duc", token=core.token)
        check(found["result"]["memories"] and "Sở Giáo dục" in found["result"]["memories"][0]["text"], "GET /v1/memory?q= tim khong dau", found["result"])
    finally:
        core.stop()

    # Tat MemoryEnabled: khong doc, khong ghi.
    core = Core(data_dir, session_dir, llm, token, {"AXIOM_MEMORY_ENABLED": "0"})
    try:
        status, health = http_json(core.base + "/health")
        check(health["result"]["memory"] == "off", "MemoryEnabled=0: /health memory off", health["result"])
        before = extract_calls()
        run(core, "Tôi là kế toán trưởng của công ty ABC")
        time.sleep(1.5)
        body = [item["body"] for item in llm.requests()][-1]
        tools = [t["function"]["name"] for t in body.get("tools", [])]
        check("Things you remember" not in body["messages"][0]["content"] and "remember" not in tools and extract_calls() == before,
              "MemoryEnabled=0: khong dua memory vao prompt, khong tool remember, khong trich xuat", tools)
    finally:
        core.stop()
        llm.stop()


def test_shutdown(work: str, token: str, bridge: FakeBridge, document_path: str, session_dir: str) -> None:
    """Core dung khi dang chay luot -> agent dung sua tai lieu (khong de tai lieu bi sua do).

    Ban .NET huy cac luot trong ApplicationStopped; ban Go huy truoc khi dong HTTP server nen pane con
    nhan duoc run.cancelled. Ca hai deu phai: KHONG gui them lenh xuong bridge sau khi Core dung.
    """
    slow = {"tool": "office_action",
            "arguments": {"action": "et.writeRange", "params": {"range": "A1", "values": [["a", 1]]}},
            "delay": 0.4}
    llm = FakeLlm(work, [slow] * 100)
    data_dir = os.path.join(work, "core-shutdown")
    os.makedirs(data_dir, exist_ok=True)
    core = Core(data_dir, session_dir, llm, token, {"AXIOM_MEMORY_AUTO_EXTRACT": "0"})
    office = {"port": bridge.port, "pid": bridge.pid, "app": "et", "family": "office"}
    try:
        bridge.commands.clear()
        status, created = core.run("ghi tung o mot", office, document={"name": "bao-cao.xlsx", "fullName": document_path})
        check(status == 200 and created["result"]["runId"], "Core dung giua luot: tao duoc luot chay", created)

        # Cho agent that su dang lam viec (co lenh xuong bridge).
        deadline = time.time() + 30
        while not bridge.commands and time.time() < deadline:
            time.sleep(0.1)
        check(bool(bridge.commands), "Core dung giua luot: agent da bat dau sua tai lieu", bridge.actions()[:3])

        # Dung Core ngay giua luot.
        core.stop()
        time.sleep(0.5)
        stopped_at = len(bridge.commands)
        time.sleep(2.0)
        check(len(bridge.commands) == stopped_at,
              "Core dung -> agent KHONG sua tai lieu nua", (stopped_at, len(bridge.commands)))
        check(core.process.poll() is not None, "Core dung -> tien trinh thoat", core.process.poll())
    finally:
        try:
            core.stop()
        except Exception:
            pass
        llm.stop()


def test_confirm(work: str, token: str, bridge: FakeBridge, document_path: str, session_dir: str) -> None:
    """Giai doan 4 (New_arch.md 8.6, 12): policy xac nhan - dong y / tu choi / het gio; audit day du."""
    llm = FakeLlm(work, [
        {"tool": "office_action", "arguments": {"action": "et.saveAs", "params": {"path": os.path.join(work, "dong-y.xlsx")}}},
        {"text": "Da luu (dong y)"},
        {"tool": "office_action", "arguments": {"action": "et.saveAs", "params": {"path": os.path.join(work, "tu-choi.xlsx")}}},
        {"text": "Nguoi dung tu choi luu"},
        {"tool": "office_action", "arguments": {"action": "et.saveAs", "params": {"path": os.path.join(work, "het-gio.xlsx")}}},
        {"text": "Het gio xac nhan"},
    ])
    data_dir = os.path.join(work, "core-confirm")
    os.makedirs(data_dir, exist_ok=True)
    core = Core(data_dir, session_dir, llm, token, {"AXIOM_CONFIRM_TIMEOUT": "8", "AXIOM_MEMORY_AUTO_EXTRACT": "0"})
    office = {"port": bridge.port, "pid": bridge.pid, "app": "et", "family": "office"}
    try:
        def run_with_answer(prompt: str, answer):
            bridge.commands.clear()
            status, created = core.run(prompt, office, document={"name": "bao-cao.xlsx", "fullName": document_path})
            run_id = created["result"]["runId"]
            collected: list = []
            url = f"{core.base}/v1/runs/{run_id}/events"
            reader = threading.Thread(target=lambda: sse_events(url, core.token, timeout=60, sink=collected), daemon=True)
            reader.start()
            confirmation = None
            deadline = time.time() + 20
            while answer is not None and confirmation is None and time.time() < deadline:
                confirmation = next((p.get("data", {}).get("confirmationId") for t, p in list(collected) if t == "confirm.required"), None)
                if confirmation is None:
                    time.sleep(0.1)
            if answer is not None and confirmation:
                http_json(f"{core.base}/v1/runs/{run_id}/confirm", method="POST", token=core.token,
                          body={"confirmationId": confirmation, "approved": answer})
            reader.join(timeout=60)
            return run_id, collected

        run_id, events = run_with_answer("Tao bang doanh thu", True)
        required = [p.get("data", {}) for t, p in events if t == "confirm.required"]
        resolved = [p.get("data", {}) for t, p in events if t == "confirm.resolved"]
        check(required and required[0].get("action") == "et.saveAs" and "lưu" in required[0].get("reason", ""),
              "confirm.required khi model tu luu ma nguoi dung khong yeu cau", required)
        check(resolved and resolved[0].get("approved") is True and bridge.actions() == ["et.saveAs"],
              "dong y -> lenh xuong bridge", (resolved, bridge.actions()))

        run_id, events = run_with_answer("Tao bang chi phi", False)
        resolved = [p.get("data", {}) for t, p in events if t == "confirm.resolved"]
        declined = [p.get("data", {}) for t, p in events if t == "tool.finished"]
        check(resolved and resolved[0].get("approved") is False and bridge.actions() == []
              and declined and "user declined" in (declined[0].get("error") or ""),
              "tu choi -> bridge khong nhan lenh, model nhan 'user declined'", (resolved, declined))
        check(events[-1][0] == "run.completed", "run van ket thuc binh thuong sau khi bi tu choi", events[-1][0])

        started = time.time()
        run_id, events = run_with_answer("Tao bang nhan su", None)
        resolved = [p.get("data", {}) for t, p in events if t == "confirm.resolved"]
        check(resolved and resolved[0].get("by") == "timeout" and resolved[0].get("approved") is False
              and bridge.actions() == [] and time.time() - started < 30,
              "het gio (AXIOM_CONFIRM_TIMEOUT=8s) = tu choi", resolved)

        status, audit = http_json(f"{core.base}/v1/audit?runId={run_id}", token=core.token)
        calls = audit["result"]["calls"]
        check(calls and calls[0]["action"] == "et.saveAs" and calls[0]["ok"] is False and "het-gio.xlsx" in (calls[0]["params"] or ""),
              "GET /v1/audit ghi tool call (action, params, ok, error)", calls)
        status, bad = http_json(f"{core.base}/v1/runs/{run_id}/confirm", method="POST", token=core.token,
                                body={"confirmationId": "cf_khong_co", "approved": True})
        check(status == 404, "confirm id khong ton tai -> 404", bad)
    finally:
        core.stop()
        llm.stop()


def test_mcp(work: str, token: str, bridge: FakeBridge, document_path: str, session_dir: str) -> None:
    """Giai doan 4 (New_arch.md 8.7, 12): MCP server mau (Python stdlib) duoc goi qua agent; server ngoai can xac
    nhan, "trusted" thi khong; lan file office (Host.exe) doc/ghi file; ghi de file da co thi hoi; server loi
    khong hong run; audit ghi du tham so."""
    sample = os.path.join(ROOT, "tests", "core", "mcp_sample_server.py")
    data_dir = os.path.join(work, "core-mcp")
    os.makedirs(data_dir, exist_ok=True)
    with open(os.path.join(data_dir, "mcp.json"), "w", encoding="utf-8") as handle:
        json.dump({"mcpServers": {
            "mau": {"command": sys.executable, "args": [sample]},
            "mau-tin": {"command": sys.executable, "args": [sample], "trusted": True},
            "hong": {"command": os.path.join(work, "khong-ton-tai.exe")},
            "tat": {"command": sys.executable, "args": [sample], "disabled": True},
        }}, handle)
    new_docx = os.path.join(work, "ghi-chu-mcp.docx")
    llm = FakeLlm(work, [
        {"tool": "mcp__mau__add", "arguments": {"a": 2, "b": 3}},
        {"tool": "mcp__mau-tin__shout", "arguments": {"text": "xin chao"}},
        {"tool": "mcp__office__doc_create", "arguments": {"path": new_docx, "paragraphs": ["Ghi chu tu lan file MCP"]}},
        {"tool": "mcp__office__doc_get_text", "arguments": {"path": new_docx}},
        {"tool": "mcp__office__doc_create", "arguments": {"path": new_docx, "paragraphs": ["ghi de"], "overwrite": True}},
        {"text": "Da dung cong cu MCP"},
    ])
    core = Core(data_dir, session_dir, llm, token, {"AXIOM_MEMORY_AUTO_EXTRACT": "0", "AXIOM_CONFIRM_TIMEOUT": "20"})
    office = {"port": bridge.port, "pid": bridge.pid, "app": "et", "family": "office"}
    try:
        status, listing = http_json(core.base + "/v1/mcp", token=core.token)
        servers = {s["name"]: s for s in listing["result"]["servers"]}
        check(set(servers) == {"office", "mau", "mau-tin", "hong"}, "GET /v1/mcp: office built-in + mcp.json (bo server disabled)", sorted(servers))
        check(servers["office"]["builtIn"] and servers["office"]["trusted"] and "mcp__office__doc_get_text" in servers["office"]["tools"]
              and not any("word_command" in t or "word_save" in t for t in servers["office"]["tools"]),
              "lan file office: co tool file, KHONG co tool live di vong allowlist", servers["office"]["tools"])
        check(servers["hong"]["error"] and not servers["hong"]["tools"], "server loi: bao loi, an tool", servers["hong"])
        check(servers["mau"]["tools"] == ["mcp__mau__add", "mcp__mau__shout", "mcp__mau__fail"], "tool ten mcp__<server>__<tool>", servers["mau"]["tools"])

        status, created = core.run("Dung cong cu MCP", office, document={"name": "bao-cao.xlsx", "fullName": document_path})
        run_id = created["result"]["runId"]
        collected: list = []
        url = f"{core.base}/v1/runs/{run_id}/events"
        reader = threading.Thread(target=lambda: sse_events(url, core.token, timeout=90, sink=collected), daemon=True)
        reader.start()
        answered: dict = {}
        deadline = time.time() + 60
        while reader.is_alive() and time.time() < deadline:
            for event_type, payload in list(collected):
                data = payload.get("data", {})
                if event_type == "confirm.required" and data.get("confirmationId") not in answered:
                    approve = data.get("action") == "mcp__mau__add"   # dong y tool ngoai, tu choi ghi de file
                    answered[data["confirmationId"]] = (data.get("action"), approve)
                    http_json(f"{core.base}/v1/runs/{run_id}/confirm", method="POST", token=core.token,
                              body={"confirmationId": data["confirmationId"], "approved": approve})
            time.sleep(0.1)
        reader.join(timeout=5)

        asked = sorted(action for action, _ in answered.values())
        check(asked == ["mcp__mau__add", "mcp__office__doc_create"],
              "xac nhan: tool ngoai + ghi de file; trusted va tao file moi thi khong hoi", asked)
        bodies = [item["body"] for item in llm.requests()]
        tools = [t["function"]["name"] for t in bodies[0].get("tools", [])]
        check({"mcp__mau__add", "mcp__mau-tin__shout", "mcp__office__doc_get_text"} <= set(tools), "tool MCP duoc dang ky cho model", tools)

        def result_of(index: int) -> str:
            return next((m.get("content", "") for m in reversed(bodies[index]["messages"]) if m.get("role") == "tool"), "")

        check('"result":"5"' in result_of(1), "mcp__mau__add (sau khi dong y) tra 5", result_of(1))
        check("XIN CHAO" in result_of(2), "mcp__mau-tin__shout (trusted) chay khong can hoi", result_of(2))
        check(os.path.exists(new_docx) and "Ghi chu tu lan file MCP" in result_of(4), "lan file office: tao docx roi doc lai", result_of(4)[:200])
        check("user declined" in result_of(5), "ghi de file da co bi tu choi -> user declined", result_of(5)[:200])
        check(collected and collected[-1][0] == "run.completed", "run MCP ket thuc binh thuong", collected[-1][0] if collected else None)

        status, audit = http_json(f"{core.base}/v1/audit?runId={run_id}", token=core.token)
        calls = list(reversed(audit["result"]["calls"]))
        check([c["tool"] for c in calls] == ["mcp__mau__add", "mcp__mau-tin__shout", "mcp__office__doc_create", "mcp__office__doc_get_text", "mcp__office__doc_create"]
              and '"a":2' in (calls[0]["params"] or "") and calls[4]["ok"] is False,
              "audit day du cho tool MCP (ten, tham so, ok)", [(c["tool"], c["ok"]) for c in calls])
    finally:
        core.stop()
        llm.stop()


def test_visual(work: str, token: str, bridge: FakeBridge, document_path: str, session_dir: str) -> None:
    """Giai doan 4 (New_arch.md 8.4.6): QA thi giac chi khi bat VisualQaEnabled; anh toi model dang image_url."""
    office = {"port": bridge.port, "pid": bridge.pid, "app": "et", "family": "office"}
    for enabled in (False, True):
        # Moi Core mot LLM gia rieng: kich ban tuan tu, Core truoc khong duoc tieu buoc cua Core sau.
        llm = FakeLlm(work, [
            {"tool": "look_at_document", "arguments": {"reason": "soat bo cuc"}},
            {"text": "Bo cuc on"},
        ])
        data_dir = os.path.join(work, "core-visual-" + ("on" if enabled else "off"))
        os.makedirs(data_dir, exist_ok=True)
        extra = {"AXIOM_MEMORY_AUTO_EXTRACT": "0", "AXIOM_VISUAL_QA": "1" if enabled else "0"}
        core = Core(data_dir, session_dir, llm, token, extra)
        try:
            before = len(llm.requests())
            bridge.commands.clear()
            status, created = core.run("Lam dep bang", office, document={"name": "bao-cao.xlsx", "fullName": document_path})
            events = core.events(created["result"]["runId"])
            bodies = [item["body"] for item in llm.requests()[before:]]
            tools = [t["function"]["name"] for t in bodies[0].get("tools", [])]
            if not enabled:
                check("look_at_document" not in tools and "app.screenshot" not in bridge.actions(),
                      "QA thi giac tat mac dinh: khong co tool look_at_document", tools)
                continue
            check("look_at_document" in tools and bridge.actions() == ["app.screenshot"], "VisualQaEnabled=1: tool look_at_document goi app.screenshot", bridge.actions())
            last = bodies[1]["messages"][-1]
            image = last["content"][1]["image_url"]["url"] if isinstance(last.get("content"), list) else ""
            check(last["role"] == "user" and image.startswith("data:image/png;base64,iVBOR"), "anh chup toi model dang image_url", str(last)[:200])
            status, audit = http_json(f"{core.base}/v1/audit?runId={created['result']['runId']}", token=core.token)
            check(all("iVBOR" not in (c["params"] or "") for c in audit["result"]["calls"]), "audit khong luu base64 anh", "")
        finally:
            core.stop()
            llm.stop()


REAL_LLM_CASES = [
    # (app, prompt, skill phai nap, skill thiet ke KHONG duoc nap)
    ("wpp", "Làm bộ slide báo cáo tháng 9/2026 của phòng Kinh doanh: doanh thu 12 tỷ, tăng 8% so với tháng 8.", "bao-cao-thang", None),
    ("et", "Lập bảng điểm lớp 10A gồm 5 học sinh với ba môn Toán, Văn, Anh và cột điểm trung bình.", "bang-diem", None),
    ("wps", "Soạn công văn của Phòng Hành chính đề nghị các phòng ban nộp báo cáo quý III trước ngày 15/10.", "van-ban-hanh-chinh", None),
    ("wps", "In đậm dòng đầu tiên của văn bản.", None, "thiet-ke-van-phong"),
]


def real_commands() -> list[dict]:
    """Danh sach lenh that (Host.exe commands --json) de LLM that thay dung mo ta tool nhu khi chay voi Office."""
    host = os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Host.exe")
    raw = subprocess.run([host, "commands", "--json"], capture_output=True, timeout=60).stdout
    data = json.loads(raw.decode("utf-8-sig", errors="replace"))
    return data if isinstance(data, list) else data.get("commands", data)


def test_real_llm(work: str, token: str) -> None:
    """LLM that (cau hinh HKCU, chi doc): model tu chon dung skill cho 3 yeu cau mau va khong nap skill thiet ke cho sua nho.
    Bridge gia (khong can Office) tra ok cho moi lenh. Bao cao so vong va thoi gian (New_arch.md muc 11, 12)."""
    global COMMANDS
    COMMANDS = real_commands()
    session_dir = os.path.join(work, "sessions-real")
    report = []
    only = [a.strip() for a in os.environ.get("AXIOM_REAL_LLM_APPS", "").split(",") if a.strip()]
    for app, prompt, expected, forbidden in REAL_LLM_CASES:
        if only and app not in only:
            continue
        bridge = FakeBridge(pid=os.getpid(), app=app)
        for name in os.listdir(session_dir) if os.path.isdir(session_dir) else []:
            os.remove(os.path.join(session_dir, name))
        document = os.path.join(work, {"wpp": "bao-cao.pptx", "et": "bang-diem.xlsx", "wps": "cong-van.docx"}[app])
        write_session(session_dir, os.getpid(), bridge.port, app, document)
        data_dir = os.path.join(work, "core-real-" + app + "-" + uuid.uuid4().hex[:4])
        os.makedirs(data_dir, exist_ok=True)
        core = Core(data_dir, session_dir, None, token)
        try:
            office = {"port": bridge.port, "pid": bridge.pid, "app": app, "family": "office"}
            started = time.time()
            status, created = core.run(prompt, office, document={"name": os.path.basename(document), "fullName": document},
                                       options={"maxSeconds": 240, "maxTokens": 150000})
            events = core.events(created["result"]["runId"], timeout=300)
            seconds = time.time() - started
            loaded = [payload.get("data", {}).get("name") for event_type, payload in events if event_type == "skill.loaded"]
            final = events[-1][0] if events else "?"
            rounds = next((payload.get("data", {}).get("rounds") for event_type, payload in events if event_type == "run.completed"), None)
            error = next((payload.get("data", {}).get("error") for event_type, payload in events if event_type == "run.failed"), None)
            report.append((app, prompt[:48], loaded, final + (" (" + str(error)[:120] + ")" if error else ""), rounds, seconds, len(bridge.commands)))
            if expected:
                check(expected in loaded, "LLM that [%s] nap dung skill '%s'" % (app, expected), loaded)
            if forbidden:
                check(forbidden not in loaded and not any(n in loaded for n in ("the-thuc-van-ban", "van-ban-hanh-chinh")),
                      "LLM that [%s] sua nho KHONG nap skill thiet ke" % app, loaded)
        finally:
            core.stop()
            bridge.stop()
    print("\nBao cao LLM that (model trong HKCU):")
    for app, prompt, loaded, final, rounds, seconds, commands in report:
        print("  %-4s %-50s skills=%-45s %s vong=%s %.1fs lenh=%d" % (app, prompt, ",".join(loaded) or "-", final, rounds, seconds, commands))


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


def test_setup(core: Core, llm: FakeLlm, work: str) -> None:
    """Wizard thiet lap: /v1/setup + /v1/llm/test + /v1/llm/models (Api/SetupEndpoints.cs).

    Day la duong ma SetupWizardForm (Windows) va setupwizard.py (Linux) goi; cau chu tieng Viet tra ve tu day.
    """
    token = core.token
    # LLM gia rieng tra loi text: kich ban chung cua `llm` bat dau bang tool call nen --only setup se thieu text.
    chat = FakeLlm(work, [{"text": "OK"}])
    try:
        _test_setup(core, token, f"http://127.0.0.1:{chat.port}/v1", work)
    finally:
        chat.stop()


def _test_setup(core: Core, token: str, endpoint: str, work: str) -> None:

    status, payload = http_json(core.base + "/v1/setup", token=token)
    check(status == 200 and payload.get("ok") is True, "GET /v1/setup tra ok", json.dumps(payload, ensure_ascii=False)[:200])
    if not payload.get("ok"):
        return
    result = payload["result"]
    check([p["id"] for p in result["presets"]] == ["company", "openai", "anthropic", "gemini"],
          "/v1/setup co du preset nha cung cap", str([p["id"] for p in result["presets"]]))
    check([s["id"] for s in result["steps"]] == ["welcome", "checks", "connect", "features", "done"],
          "/v1/setup co du buoc cua wizard", str([s["id"] for s in result["steps"]]))
    check(all(p["label"] and p["description"] for p in result["presets"]), "moi preset co nhan + mo ta")
    check(result["current"]["model"] == "fake-model" and result["current"]["configured"] is True,
          "/v1/setup bao dung cau hinh dang chay", json.dumps(result["current"], ensure_ascii=False))
    check(result["current"]["hasKey"] is True, "/v1/setup noi da co khoa API (khong tra khoa)", json.dumps(result["current"]))
    check("fake-key" not in json.dumps(result), "/v1/setup KHONG tra khoa API")
    check(result["core"]["skills"] >= 1 and result["core"]["port"] == core.port, "/v1/setup bao trang thai Core", json.dumps(result["core"]))

    # Danh sach model: nguoi dung chon thay vi tu go ten.
    status, models = http_json(f"{core.base}/v1/llm/models?provider=openai&endpoint={endpoint}&apiKey=fake-key", token=token)
    check(status == 200 and models["result"].get("models") == ["fake-model", "fake-model-2"],
          "GET /v1/llm/models tra danh sach model", json.dumps(models, ensure_ascii=False)[:200])

    # Thu ket noi thanh cong (gia tri chua luu, y het luc bam nut trong wizard).
    status, ok = http_json(core.base + "/v1/llm/test", method="POST", token=token, body={
        "providerId": "company", "endpoint": endpoint, "model": "fake-model", "apiKey": "fake-key"})
    check(status == 200 and ok["result"].get("reply"), "POST /v1/llm/test bao ket noi tot", json.dumps(ok, ensure_ascii=False)[:200])
    check(ok["result"].get("seconds") is not None and ok["result"].get("model") == "fake-model",
          "ket qua thu co so giay + ten model", json.dumps(ok["result"], ensure_ascii=False))

    # Model khong ton tai -> goi y chon lai tu danh sach.
    status, bad = http_json(core.base + "/v1/llm/test", method="POST", token=token, body={
        "providerId": "company", "endpoint": endpoint, "model": "no-such-model", "apiKey": "fake-key"})
    check(bad["result"].get("kind") == "model", "model sai -> kind=model", json.dumps(bad, ensure_ascii=False)[:200])
    check("danh sách model" in bad["result"].get("hint", ""), "goi y bam Tai danh sach model", str(bad["result"].get("hint"))[:200])

    # Dia chi sai -> khong ket noi duoc (khong phai loi khoa).
    status, dead = http_json(core.base + "/v1/llm/test", method="POST", token=token, body={
        "providerId": "company", "endpoint": f"http://127.0.0.1:{free_port()}/v1", "model": "fake-model", "apiKey": "x"})
    check(dead["result"].get("kind") in ("network", "dns", "timeout"),
          "dia chi sai -> kind mang/dns/timeout", json.dumps(dead, ensure_ascii=False)[:200])

    # Thieu dia chi -> loi cau hinh (khong goi mang).
    status, missing = http_json(core.base + "/v1/llm/test", method="POST", token=token, body={"model": "fake-model"})
    check(missing["result"].get("kind") == "config", "thieu dia chi -> kind=config", json.dumps(missing, ensure_ascii=False)[:200])

    # Sai khoa API: LLM gia doi khoa khac -> 401 -> cau "khoa khong dung".
    picky = FakeLlm(work, [{"text": "OK"}], key="sk-dung", models=["fake-model"])
    try:
        status, wrong = http_json(core.base + "/v1/llm/test", method="POST", token=token, body={
            "providerId": "company", "endpoint": f"http://127.0.0.1:{picky.port}/v1", "model": "fake-model",
            "apiKey": "sk-sai"})
        check(wrong["result"].get("kind") == "auth", "sai khoa -> kind=auth", json.dumps(wrong, ensure_ascii=False)[:200])
        status, models = http_json(
            f"{core.base}/v1/llm/models?provider=openai&endpoint=http://127.0.0.1:{picky.port}/v1&apiKey=sk-sai", token=token)
        check(models["result"].get("kind") == "auth", "sai khoa khi lay danh sach model -> kind=auth",
              json.dumps(models, ensure_ascii=False)[:200])
    finally:
        picky.stop()

    # Khong co token -> 401 nhu moi endpoint khac cua Core (khoa API khong duoc lo ra ngoai).
    status, _ = http_json(core.base + "/v1/llm/test", method="POST", body={"endpoint": endpoint, "model": "fake-model"})
    check(status == 401, "POST /v1/llm/test can token", str(status))


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


# Cac phan chay duoc rieng (--only); /health luon chay truoc.
SECTIONS = ["fake_bridge", "guards", "skills", "memory", "confirm", "mcp", "visual", "setup", "shutdown"]

# Phan phu thuoc: `guards` dung chung kich ban LLM voi `fake_bridge` (kich ban tuan tu) nen chay mot minh
# se lech buoc -> --only tu keo theo phan can truoc.
REQUIRES = {"guards": ["fake_bridge"]}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--office", action="store_true", help="dung Office that thay vi bridge gia")
    parser.add_argument("--keep", action="store_true", help="giu thu muc tam de xem log")
    parser.add_argument("--real-llm", action="store_true",
                        help="goi LLM that trong HKCU (ton token): model tu chon dung skill; khong chay mac dinh")
    parser.add_argument("--only", default="",
                        help="chi chay cac phan (cach nhau dau phay): " + ",".join(SECTIONS)
                        + " - dung khi port Core sang Go tung giai doan")
    args = parser.parse_args()
    only = [name.strip() for name in args.only.split(",") if name.strip()]
    unknown = [name for name in only if name not in SECTIONS]
    if unknown:
        parser.error("phan khong co: " + ", ".join(unknown))
    for name in list(only):
        for needed in REQUIRES.get(name, []):
            if needed not in only:
                only.append(needed)
                print("--only %s: chay them %s (phan phu thuoc)" % (name, needed))
    only = [name for name in SECTIONS if name in only]

    def wanted(section: str) -> bool:
        return not only or section in only

    if args.real_llm:
        work = os.path.join(tempfile.gettempdir(), "axiom-core-real-" + uuid.uuid4().hex[:8])
        os.makedirs(work, exist_ok=True)
        try:
            test_real_llm(work, "e2e-token-" + uuid.uuid4().hex[:8])
        finally:
            if not args.keep:
                shutil.rmtree(work, ignore_errors=True)
        failed = [item for item in RESULTS if not item[0]]
        print("\n%d passed, %d failed" % (len(RESULTS) - len(failed), len(failed)))
        return 1 if failed else 0

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
        core = Core(data_dir, None if args.office else session_dir, llm, token, {"AXIOM_MEMORY_AUTO_EXTRACT": "0"})

        status, health = http_json(core.base + "/health")
        check(status == 200 and health["result"]["memory"] == "on", "GET /health bao memory on", health["result"])
        check(health["result"]["version"] and health["result"]["protocol"] == 1, "version + protocol trong /health",
              health["result"])

        if args.office:
            test_office(core, office_port, office_pid)
        else:
            if wanted("fake_bridge"):
                test_fake_bridge(core, bridge, llm, document_path)
            if wanted("guards"):
                test_guards(core, bridge, document_path)
            if wanted("skills"):
                test_skills(work, token, bridge, document_path, session_dir)
            if wanted("memory"):
                test_memory(work, token, bridge, document_path, session_dir)
            if wanted("confirm"):
                test_confirm(work, token, bridge, document_path, session_dir)
            if wanted("mcp"):
                test_mcp(work, token, bridge, document_path, session_dir)
            if wanted("visual"):
                test_visual(work, token, bridge, document_path, session_dir)
            if wanted("setup"):
                test_setup(core, llm, work)
            if wanted("shutdown"):
                test_shutdown(work, token, bridge, document_path, session_dir)
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
