from __future__ import annotations

import ctypes
import glob
import json
import os
import time
import urllib.request

APP_PORTS = {
    "wps": 47821,
    "et": 47822,
    "wpp": 47823,
    "word": 47831,
    "excel": 47832,
    "ppt": 47833,
    "powerpoint": 47833,
}
DEFAULT_TIMEOUT = 120


def _read_token() -> str:
    try:
        import winreg
    except ImportError:
        return ""
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\WpsAiBridge") as key:
            value, _ = winreg.QueryValueEx(key, "Token")
            return str(value or "")
    except OSError:
        return ""


def base_url(app: str = "wps", port: int | None = None) -> str:
    resolved = port or APP_PORTS.get(app)
    if not resolved:
        raise ValueError(f"unknown app '{app}' (expected wps/et/wpp or word/excel/ppt)")
    return f"http://127.0.0.1:{resolved}"


def health(app: str = "wps", port: int | None = None, timeout: float = 10) -> dict:
    url = base_url(app, port) + "/health"
    with urllib.request.urlopen(url, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


def command(app: str, action: str, params: dict | None = None,
            port: int | None = None, timeout: int = DEFAULT_TIMEOUT) -> dict:
    url = base_url(app, port) + "/cmd"
    payload = json.dumps({"action": action, "params": params or {}}, ensure_ascii=False).encode("utf-8")
    headers = {"Content-Type": "application/json; charset=utf-8"}
    token = _read_token()
    if token:
        headers["X-Auth-Token"] = token
    request = urllib.request.Request(url, data=payload, method="POST", headers=headers)
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


# ---------------------------------------------------------------------------
# Session registry: mỗi bridge đang sống ghi %LOCALAPPDATA%\WpsAiBridge\sessions\{pid}.json
# (heartbeat 25s). sessions() đọc thư mục đó để phát hiện mọi instance Office + WPS.
# ---------------------------------------------------------------------------

SESSIONS_DIR = os.path.join(os.environ.get("LOCALAPPDATA", ""), "WpsAiBridge", "sessions")
STALE_SECONDS = 90


def _pid_alive(pid: int) -> bool:
    if not pid:
        return False
    if os.name != "nt":
        try:
            os.kill(pid, 0)
            return True
        except OSError:
            return False
    kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel32.OpenProcess.restype = ctypes.c_void_p
    handle = kernel32.OpenProcess(0x1000, False, int(pid))  # PROCESS_QUERY_LIMITED_INFORMATION
    if not handle:
        return ctypes.get_last_error() == 5  # ERROR_ACCESS_DENIED: process tồn tại nhưng không mở được
    try:
        code = ctypes.c_ulong()
        if not kernel32.GetExitCodeProcess(ctypes.c_void_p(handle), ctypes.byref(code)):
            return False
        return code.value == 259  # STILL_ACTIVE
    finally:
        kernel32.CloseHandle(ctypes.c_void_p(handle))


def sessions(prune: bool = True, timeout: float = 1.5, directory: str | None = None) -> list[dict]:
    """List live bridge sessions from the registry directory.

    Mỗi entry: pid, app (wps|et|wpp), family (office|wps), port, host, document, started,
    lastSeen, healthy (/health trả đúng pid), ageSeconds, file.
    Prune file khi process đã chết, hoặc heartbeat quá STALE_SECONDS mà /health cũng không trả lời.
    Bridge đang bận một lệnh dài (/health chưa trả kịp) nhưng heartbeat còn mới thì vẫn giữ,
    với healthy=False.
    """
    directory = directory or SESSIONS_DIR
    found = []
    for path in sorted(glob.glob(os.path.join(directory, "*.json"))):
        try:
            with open(path, encoding="utf-8-sig") as handle:
                data = json.load(handle)
        except (OSError, ValueError):
            continue  # đang được ghi lại; lần sau đọc
        pid = int(data.get("pid") or 0)
        port = data.get("port")
        try:
            last_seen = float(data.get("lastSeenEpoch") or os.path.getmtime(path))
        except (OSError, TypeError, ValueError):
            last_seen = 0.0
        age = max(0.0, time.time() - last_seen)
        alive = _pid_alive(pid)
        healthy = False
        if alive and port:
            try:
                reply = health(port=int(port), timeout=timeout)
                result = reply.get("result") or {}
                healthy = bool(reply.get("ok")) and int(result.get("pid") or 0) == pid
            except Exception:
                healthy = False
        if not alive or (age > STALE_SECONDS and not healthy):
            if prune:
                try:
                    os.remove(path)
                except OSError:
                    pass
            continue
        data["healthy"] = healthy
        data["ageSeconds"] = round(age, 1)
        data["file"] = path
        found.append(data)
    found.sort(key=lambda item: (item.get("family") or "", int(item.get("port") or 0)))
    return found
