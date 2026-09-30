"""Client Agent Core (LibreOffice_arch.md muc 9, 10): ban Python cua CoreClient.cs.

Pane trong LibreOffice noi chuyen voi Agent Core qua HTTP/SSE: POST /v1/runs, GET /v1/runs/{id}/events,
POST /v1/runs/{id}/cancel|confirm, GET /v1/memory|/v1/skills. Core doc session registry cua bridge
(theo `office.port`) nen agent sua dung tai lieu dang mo.

Khong import uno: module thuan Python de test don vi duoc (xem tests/lo/test_chat.py).
"""
from __future__ import annotations

import json
import os
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

from . import config

START_TIMEOUT_SECONDS = 25
RUN_START_TIMEOUT = 30        # POST /v1/runs
CALL_TIMEOUT = 30             # cac lenh ngan (memory, skills, cancel, confirm)
RUN_POLL_SECONDS = 1.0


class CoreError(Exception):
    """Core khong chay, tra loi loi, hoac du lieu tra ve khong doc duoc."""


def core_json_path() -> str:
    return os.path.join(config.data_dir(), "core.json")


def read_core_json() -> dict:
    try:
        with open(core_json_path(), encoding="utf-8") as handle:
            return json.load(handle) or {}
    except (OSError, ValueError):
        return {}


def base_url(port: int) -> str:
    return "http://127.0.0.1:%d" % port


def health(base: str, timeout: float = 3.0):
    """Tra ve dict `result` cua /health, hoac None neu Core khong tra loi."""
    try:
        with urllib.request.urlopen(base + "/health", timeout=timeout) as response:
            payload = json.loads(response.read().decode("utf-8"))
        return payload.get("result") if payload.get("ok") else None
    except (OSError, ValueError, urllib.error.URLError):
        return None


def call(base: str, method: str, path: str, body=None, timeout: float = CALL_TIMEOUT, stream: bool = False):
    """Goi API Core; tra ve `result` (dict) hoac doi tuong response khi stream=True."""
    data = None if body is None else json.dumps(body, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(base + path, data=data, method=method)
    request.add_header("Content-Type", "application/json")
    token = config.token()
    if token:
        request.add_header("X-Auth-Token", token)
    try:
        response = urllib.request.urlopen(request, timeout=timeout)
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", "replace")
        try:
            raise CoreError(json.loads(detail).get("error") or detail[:200]) from None
        except ValueError:
            raise CoreError("HTTP %d: %s" % (exc.code, detail[:200])) from None
    except (OSError, urllib.error.URLError) as exc:
        raise CoreError("khong ket noi duoc Agent Core (%s): %s" % (base, exc)) from None
    if stream:
        return response
    with response:
        payload = json.loads(response.read().decode("utf-8"))
    if not payload.get("ok"):
        raise CoreError(payload.get("error") or "Agent Core tra ve loi khong ro")
    return payload.get("result")


def ensure(port: int = 0) -> tuple[str, int]:
    """Core dang song -> (base_url, port). Chua chay thi khoi dong bang `CoreExe` (neu co) roi cho."""
    candidates = []
    info = read_core_json()
    if info.get("port"):
        candidates.append(int(info["port"]))
    if port:
        candidates.insert(0, int(port))
    for candidate in candidates:
        base = base_url(candidate)
        if health(base):
            return base, candidate

    exe = config.value("CoreExe")
    if exe and os.path.isfile(str(exe)):
        flags = getattr(subprocess, "CREATE_NO_WINDOW", 0)
        subprocess.Popen([str(exe)], cwd=os.path.dirname(str(exe)), creationflags=flags)
        deadline = time.time() + START_TIMEOUT_SECONDS
        while time.time() < deadline:
            time.sleep(0.5)
            info = read_core_json()
            if info.get("port"):
                base = base_url(int(info["port"]))
                if health(base):
                    return base, int(info["port"])
        raise CoreError("Agent Core khong san sang sau %ds" % START_TIMEOUT_SECONDS)
    raise CoreError("Agent Core chua chay. Chay AxiomOffice.Core.exe (hoac dat CoreExe trong cau hinh) roi thu lai.")


def start_run(base: str, prompt: str, port: int, kind: str, document=None, conversation_id: str | None = None,
              interactive: bool = True, pid: int | None = None, max_seconds: int = 300) -> dict:
    body = {
        "prompt": prompt,
        "office": {"port": port, "pid": pid or os.getpid(), "app": kind, "family": "libreoffice"},
        "document": document or {},
        "options": {"maxSeconds": max_seconds, "maxTokens": 200000, "interactive": interactive},
    }
    if conversation_id:
        body["conversationId"] = conversation_id
    return call(base, "POST", "/v1/runs", body, timeout=RUN_START_TIMEOUT)


def get_run(base: str, run_id: str) -> dict:
    return call(base, "GET", "/v1/runs/" + urllib.parse.quote(run_id), timeout=CALL_TIMEOUT)


def cancel_run(base: str, run_id: str) -> bool:
    try:
        call(base, "POST", "/v1/runs/%s/cancel" % urllib.parse.quote(run_id), {}, timeout=CALL_TIMEOUT)
        return True
    except CoreError:
        return False


def confirm_run(base: str, run_id: str, confirmation_id: str, approved: bool) -> bool:
    try:
        call(base, "POST", "/v1/runs/%s/confirm" % urllib.parse.quote(run_id),
             {"confirmationId": confirmation_id, "approved": approved}, timeout=CALL_TIMEOUT)
        return True
    except CoreError:
        return False


TERMINAL = ("completed", "cancelled", "timedout", "stopped", "failed")


def stream_events(base: str, run_id: str, on_event, stop=None, after: int = 0, timeout: float = 3600):
    """Doc SSE cua luot chay, goi on_event(dict) cho tung su kien. Tra ve khi luot chay ket thuc."""
    path = "/v1/runs/%s/events" % urllib.parse.quote(run_id)
    if after:
        path += "?after=%d" % after
    response = call(base, "GET", path, timeout=timeout, stream=True)
    event_type, data_lines = None, []
    with response:
        for raw in response:
            if stop is not None and stop.is_set():
                return
            line = raw.decode("utf-8", "replace").rstrip("\r\n")
            if line.startswith("event:"):
                event_type = line[6:].strip()
            elif line.startswith("data:"):
                data_lines.append(line[5:].strip())
            elif not line:
                if event_type and event_type != "ping":
                    payload = None
                    text = "\n".join(data_lines)
                    if text:
                        try:
                            payload = json.loads(text)
                        except ValueError:
                            payload = {"type": event_type, "raw": text}
                    on_event(payload if isinstance(payload, dict) else {"type": event_type})
                event_type, data_lines = None, []
                continue


def wait_for_run(base: str, run_id: str, stop=None, on_tick=None, poll: float = RUN_POLL_SECONDS) -> dict:
    """Khong dung SSE: cho bang cach hoi trang thai (du phong khi SSE bi chan)."""
    while True:
        if stop is not None and stop.is_set():
            cancel_run(base, run_id)
        run = get_run(base, run_id)
        if on_tick:
            on_tick(run)
        if str(run.get("status")) in TERMINAL:
            return run
        time.sleep(poll)


# ---------------------------------------------------------------- memory, skills

def memories(base: str, query: str = "", scope: str | None = None, limit: int = 50) -> list:
    params = {"limit": str(limit)}
    if query:
        params["q"] = query
    if scope:
        params["scope"] = scope
    result = call(base, "GET", "/v1/memory?" + urllib.parse.urlencode(params))
    return result.get("memories") or []


def delete_memory(base: str, memory_id: str, reason: str = "pane") -> bool:
    try:
        call(base, "DELETE", "/v1/memory/" + urllib.parse.quote(memory_id) + "?reason=" + urllib.parse.quote(reason))
        return True
    except CoreError:
        return False


def skills(base: str, kind: str | None = None) -> list:
    path = "/v1/skills" + ("?app=" + urllib.parse.quote(kind) if kind else "")
    return call(base, "GET", path).get("skills") or []
