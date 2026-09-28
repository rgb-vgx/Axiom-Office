from __future__ import annotations

import json
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


def base_url(app: str = "ppt", port: int | None = None) -> str:
    resolved = port or APP_PORTS.get(app)
    if not resolved:
        raise ValueError(f"unknown app '{app}' (expected ppt/powerpoint or wpp)")
    return f"http://127.0.0.1:{resolved}"


def health(app: str = "ppt", port: int | None = None) -> dict:
    url = base_url(app, port) + "/health"
    with urllib.request.urlopen(url, timeout=10) as response:
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
