"""Spec T1 (docs/wps-bridge.md muc 2): Agent Core lam bridge cho JS add-in WPS - test hop den.

Build va chay Core THAT (go build, hoac AXIOM_E2E_CORE_EXE), dong vai add-in bang HTTP (poll/result), kiem:
phuc vu file add-in, xac thuc theo Origin cua chinh Core, bridge WPS tren cong rieng (health/commands/session/cmd)
va file session. Khong can WPS.

    python3 tests/wps/test_wps_bridge_e2e.py
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
import urllib.error
import urllib.request
import uuid

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
RESULTS: list = []


def check(ok, name, detail=""):
    RESULTS.append((bool(ok), name))
    print(("PASS " if ok else "FAIL ") + name + ("" if ok else "\n     " + str(detail)[:600]))


def free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def request(url, method="GET", body=None, headers=None, timeout=30, raw_body=None):
    """(status, headers, text). Khong nem loi HTTP; loi ket noi -> (0, {}, loi)."""
    data = raw_body if raw_body is not None else (None if body is None else json.dumps(body).encode("utf-8"))
    req = urllib.request.Request(url, data=data, method=method)
    if body is not None and raw_body is None:
        req.add_header("Content-Type", "application/json")
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:
            return response.status, dict(response.headers), response.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as exc:
        return exc.code, dict(exc.headers), exc.read().decode("utf-8", "replace")
    except Exception as exc:  # noqa: BLE001
        return 0, {}, str(exc)


def as_json(text):
    try:
        return json.loads(text)
    except ValueError:
        return {}


def core_exe(work: str) -> str:
    exe = os.environ.get("AXIOM_E2E_CORE_EXE")
    if exe:
        return exe
    exe = os.path.join(work, "axiom-core")
    built = subprocess.run(["go", "-C", os.path.join(ROOT, "core-go"), "build", "-o", exe, "./cmd/axiom-core"],
                           capture_output=True, text=True, timeout=600)
    if built.returncode != 0:
        raise SystemExit("go build loi:\n" + built.stderr[-2000:])
    return exe


class Core:
    def __init__(self, exe: str, work: str, token: str, bridge_port: int, addin_dir: str, session_dir: str,
                 extra_env: dict | None = None):
        self.data_dir = os.path.join(work, "core")
        os.makedirs(self.data_dir, exist_ok=True)
        env = {k: v for k, v in os.environ.items() if not k.startswith("AXIOM_")}
        env.update({
            "AXIOM_CORE_DATA_DIR": self.data_dir,
            "AXIOM_CORE_PORT": "0",
            "AXIOM_CORE_SINGLE_INSTANCE": "0",
            "AXIOM_CORE_MUTEX_NAME": "Local\\AxiomOffice.Core.WpsE2E." + uuid.uuid4().hex[:8],
            "AXIOM_TOKEN": token,
            "AXIOM_SESSION_DIR": session_dir,
            "AXIOM_MEMORY_ENABLED": "0",
            "AXIOM_WPS_BRIDGE_PORT": str(bridge_port),
            "AXIOM_WPS_ADDIN_DIR": addin_dir,
            "AXIOM_WPS_POLL_SECONDS": "2",
            "AXIOM_WPS_DISCONNECT_SECONDS": "4",
        })
        env.update(extra_env or {})
        self.log_path = os.path.join(work, "core.out")
        self.log = open(self.log_path, "w", encoding="utf-8")
        self.process = subprocess.Popen([exe], env=env, cwd=self.data_dir, stdout=self.log, stderr=subprocess.STDOUT)
        self.port = 0
        deadline = time.time() + 40
        while time.time() < deadline:
            if self.process.poll() is not None:
                raise SystemExit("Core thoat som:\n" + open(self.log_path, encoding="utf-8").read()[-3000:])
            try:
                with open(os.path.join(self.data_dir, "core.json"), encoding="utf-8") as handle:
                    self.port = json.load(handle)["port"]
                if request("http://127.0.0.1:%d/health" % self.port, timeout=2)[0] == 200:
                    return
            except Exception:  # noqa: BLE001
                pass
            time.sleep(0.2)
        raise SystemExit("Core khong san sang:\n" + open(self.log_path, encoding="utf-8").read()[-3000:])

    @property
    def base(self) -> str:
        return "http://127.0.0.1:%d" % self.port

    @property
    def origin(self) -> str:
        return "http://127.0.0.1:%d" % self.port

    def stop(self) -> None:
        request(self.base + "/v1/admin/shutdown", "POST", {}, {"X-Auth-Token": TOKEN}, timeout=5)
        try:
            self.process.wait(timeout=15)
        except subprocess.TimeoutExpired:
            self.process.kill()
        self.log.close()


TOKEN = "t-" + uuid.uuid4().hex
CATALOG = [
    {"name": "writer.appendText", "kind": "wps", "agent": True, "summary": "Noi vao cuoi tai lieu",
     "params": [{"name": "text", "required": True, "hint": None}]},
    {"name": "writer.getText", "kind": "wps", "agent": True, "summary": "Doc text",
     "params": [{"name": "maxChars", "required": False, "hint": None}]},
]
DOCUMENT = {"name": "bao-cao.docx", "fullName": "/home/u/bao-cao.docx"}


def poll(core: Core, headers=None, document=None, timeout=15):
    body = {"client": "test-addin", "app": "wps", "version": "0.1.0",
            "document": document or DOCUMENT, "commands": CATALOG}
    return request(core.base + "/wps/v1/poll", "POST", body,
                   headers if headers is not None else {"Origin": core.origin}, timeout=timeout)


def session_file(session_dir: str, core: Core) -> str:
    return os.path.join(session_dir, "%d-wps.json" % core.process.pid)


def wait_for(predicate, timeout=10.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if predicate():
            return True
        time.sleep(0.2)
    return predicate()


def main() -> int:
    work = tempfile.mkdtemp(prefix="axiom-wps-bridge-")
    addin_dir = os.path.join(work, "wps-addin")
    os.makedirs(addin_dir)
    with open(os.path.join(addin_dir, "index.html"), "w", encoding="utf-8") as handle:
        handle.write('<!DOCTYPE html><html><body><script src="main.js"></script></body></html>\n')
    with open(os.path.join(addin_dir, "ribbon.xml"), "w", encoding="utf-8") as handle:
        handle.write('<customUI xmlns="http://schemas.microsoft.com/office/2006/01/customui"/>\n')
    with open(os.path.join(work, "secret.txt"), "w", encoding="utf-8") as handle:
        handle.write("khong duoc lo")
    session_dir = os.path.join(work, "sessions")
    os.makedirs(session_dir)
    bridge_port = free_port()
    bridge = "http://127.0.0.1:%d" % bridge_port
    auth = {"X-Auth-Token": TOKEN}

    core = Core(core_exe(work), work, TOKEN, bridge_port, addin_dir, session_dir)
    try:
        # ---- 2.1 phuc vu file add-in
        status, headers, text = request(core.base + "/wps/addin/index.html")
        check(status == 200 and "main.js" in text and "text/html" in headers.get("Content-Type", ""),
              "GET /wps/addin/index.html khong token -> 200 text/html", (status, headers.get("Content-Type"), text[:200]))
        check(headers.get("Cache-Control") == "no-store",
              "GET /wps/addin/index.html -> Cache-Control no-store", headers.get("Cache-Control"))
        status, headers, _ = request(core.base + "/wps/addin/ribbon.xml")
        check(status == 200 and "xml" in headers.get("Content-Type", ""), "ribbon.xml -> 200 Content-Type xml",
              (status, headers.get("Content-Type")))
        status, _, _ = request(core.base + "/wps/addin/khong-co.js")
        check(status == 404, "file add-in khong ton tai -> 404", status)
        for path in ("/wps/addin/../secret.txt", "/wps/addin/%2e%2e/secret.txt", "/wps/addin/..%2fsecret.txt"):
            status, _, text = request(core.base + path)
            check(status in (400, 403, 404) and "khong duoc lo" not in text, "chan thoat khoi thu muc add-in: " + path,
                  (status, text[:200]))
        status, _, _ = request(core.base + "/wps/addin/index.html", headers={"Origin": "http://evil.example"})
        check(status == 403, "file add-in voi Origin la -> 403", status)
        status, _, _ = request(core.base + "/wps/addin/index.html", "POST", {})
        check(status in (404, 405), "file add-in chi GET", status)

        # ---- 2.3 bridge truoc khi add-in ket noi
        status, _, text = request(bridge + "/health")
        health = as_json(text).get("result") or {}
        check(status == 200 and health.get("app") == "wps" and health.get("family") == "wps-linux"
              and health.get("pid") == core.process.pid and health.get("port") == bridge_port
              and health.get("connected") is False,
              "bridge /health khong token: app=wps family=wps-linux pid=Core connected=false", text)
        status, _, text = request(bridge + "/cmd", "POST", {"action": "writer.getText", "params": {}}, auth)
        reply = as_json(text)
        check(status == 200 and reply.get("ok") is False and "not connected" in str(reply.get("error")),
              "bridge /cmd khi chua co add-in -> ok:false 'not connected'", text)
        check(not os.path.exists(session_file(session_dir, core)), "chua ket noi thi khong co file session",
              os.listdir(session_dir))

        # ---- 2.2 xac thuc kenh add-in
        status, _, text = poll(core, headers={"Origin": "http://evil.example"})
        check(status == 403, "poll voi Origin la -> 403", (status, text))
        status, _, text = poll(core, headers={})
        check(status == 401, "poll khong Origin, khong token -> 401", (status, text))
        status, _, text = request(core.base + "/wps/v1/poll", "POST", raw_body=b"{}",
                                  headers={"Origin": core.origin, "Content-Type": "text/plain"})
        check(status == 415, "poll text/plain -> 415", (status, text))
        status, _, text = request(core.base + "/v1/skills", headers={"Origin": core.origin})
        check(status in (401, 403), "Origin cua Core KHONG mo khoa API khac ngoai /wps/v1 (/v1/skills)", (status, text))

        started = time.time()
        status, _, text = poll(core)
        waited = time.time() - started
        reply = as_json(text)
        check(status == 200 and reply.get("ok") is True and (reply.get("result") or {}).get("jobs") == [],
              "poll voi Origin cua Core -> 200, het gio tra jobs rong", text)
        check(1.0 <= waited <= 6.0, "poll giu request ~AXIOM_WPS_POLL_SECONDS (2s) khi khong co lenh", round(waited, 2))
        status, _, text = poll(core, headers=auth)
        check(status == 200 and as_json(text).get("ok") is True, "poll khong Origin nhung co token -> 200", text)

        # ---- da ket noi: session + commands + session file
        status, _, text = request(bridge + "/health")
        check((as_json(text).get("result") or {}).get("connected") is True, "sau poll: /health connected=true", text)
        status, _, text = request(bridge + "/commands", headers=auth)
        commands = (as_json(text).get("result") or {}).get("commands") or []
        check(status == 200 and [c.get("name") for c in commands] == ["writer.appendText", "writer.getText"]
              and commands[0].get("agent") is True and commands[0]["params"][0].get("required") is True,
              "bridge /commands = catalog add-in gui trong poll", text)
        status, _, text = request(bridge + "/session", headers=auth)
        result = as_json(text).get("result") or {}
        check(status == 200 and result.get("app") == "wps" and (result.get("document") or {}).get("name") == "bao-cao.docx",
              "bridge /session co tai lieu add-in bao", text)
        path = session_file(session_dir, core)
        check(wait_for(lambda: os.path.exists(path), 5), "file session <pid Core>-wps.json xuat hien", os.listdir(session_dir))
        try:
            with open(path, encoding="utf-8") as handle:
                data = json.load(handle)
        except Exception as exc:  # noqa: BLE001
            data = {"error": str(exc)}
        check(data.get("pid") == core.process.pid and data.get("app") == "wps" and data.get("family") == "wps-linux"
              and data.get("port") == bridge_port and data.get("document") == "bao-cao.docx"
              and data.get("documentPath") == "/home/u/bao-cao.docx" and data.get("lastSeenEpoch"),
              "file session dung truong (pid, app, family, port, document, documentPath, lastSeenEpoch)", data)

        # ---- guard cua bridge WPS (giong bridge LibreOffice)
        status, _, _ = request(bridge + "/cmd", "POST", {"action": "writer.getText"}, {**auth, "Origin": core.origin})
        check(status == 403, "bridge /cmd co Origin (ke ca origin Core) -> 403", status)
        status, _, _ = request(bridge + "/cmd", "POST", {"action": "writer.getText"})
        check(status == 401, "bridge /cmd khong token -> 401", status)
        status, _, _ = request(bridge + "/cmd", "POST", raw_body=b'{"action":"x"}',
                               headers={**auth, "Content-Type": "text/plain"})
        check(status == 415, "bridge /cmd text/plain -> 415", status)

        # ---- vong lenh: /cmd -> poll nhan job -> result -> /cmd tra ket qua add-in
        def roundtrip(action, params, answer):
            box = {}

            def caller():
                box["reply"] = request(bridge + "/cmd", "POST", {"action": action, "params": params}, auth, timeout=30)

            thread = threading.Thread(target=caller)
            thread.start()
            job = None
            deadline = time.time() + 10
            while job is None and time.time() < deadline:
                status, _, text = poll(core)
                jobs = (as_json(text).get("result") or {}).get("jobs") or []
                if jobs:
                    job = jobs[0]
            if job is None:
                thread.join(35)
                return None, box.get("reply")
            posted = request(core.base + "/wps/v1/result", "POST", dict(answer, id=job.get("id")),
                             {"Origin": core.origin})
            thread.join(35)
            return job, box.get("reply"), posted

        outcome = roundtrip("writer.appendText", {"text": "Xin chào"}, {"ok": True, "result": {"appended": 8}})
        job, reply = outcome[0], outcome[1]
        check(job is not None and job.get("action") == "writer.appendText" and job.get("params") == {"text": "Xin chào"}
              and job.get("id"), "poll nhan job dung action/params/id", job)
        posted = outcome[2] if len(outcome) > 2 else None
        check(posted is not None and posted[0] == 200 and as_json(posted[2]).get("ok") is True,
              "POST /wps/v1/result -> ok", posted)
        body = as_json(reply[2]) if reply else {}
        check(reply and reply[0] == 200 and body == {"ok": True, "result": {"appended": 8}},
              "bridge /cmd tra nguyen ket qua add-in", reply)

        outcome = roundtrip("writer.getText", {}, {"ok": False, "error": "TypeError: x is undefined"})
        body = as_json(outcome[1][2]) if outcome[1] else {}
        check(body.get("ok") is False and body.get("error") == "TypeError: x is undefined",
              "loi cua add-in duoc tra nguyen ok:false + error", outcome[1])

        status, _, text = request(core.base + "/wps/v1/result", "POST", {"id": "khong-co", "ok": True, "result": {}},
                                  {"Origin": core.origin})
        check(status == 200 and as_json(text).get("ok") is False, "result voi id la -> ok:false", (status, text))

        # ---- mat ket noi: sau AXIOM_WPS_DISCONNECT_SECONDS khong poll -> connected=false, xoa file session
        check(wait_for(lambda: not os.path.exists(path), 12), "ngung poll -> file session bi xoa", os.listdir(session_dir))
        status, _, text = request(bridge + "/health")
        check((as_json(text).get("result") or {}).get("connected") is False, "ngung poll -> /health connected=false", text)
        status, _, text = request(bridge + "/cmd", "POST", {"action": "writer.getText", "params": {}}, auth)
        check("not connected" in str(as_json(text).get("error")), "ngung poll -> /cmd lai bao not connected", text)

        # ---- Core dung -> khong de lai file session
        poll(core)
        wait_for(lambda: os.path.exists(path), 5)
    finally:
        core.stop()
    check(not os.path.exists(session_file(session_dir, core)), "Core dung -> xoa file session WPS", os.listdir(session_dir))
    shutil.rmtree(work, ignore_errors=True)

    failed = [name for ok, name in RESULTS if not ok]
    print("\n%d passed, %d failed" % (len(RESULTS) - len(failed), len(failed)))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
