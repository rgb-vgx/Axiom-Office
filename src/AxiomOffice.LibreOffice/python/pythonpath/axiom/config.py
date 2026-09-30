"""Cau hinh va duong dan (LibreOffice_arch.md muc 4, 9).

Windows: dung chung HKCU\\Software\\AxiomOffice va %LOCALAPPDATA%\\AxiomOffice voi add-in (cung token,
cung thu muc sessions de Agent Core tim thay bridge). Linux: ~/.config/axiom-office/config.json (0600)
va thu muc XDG.
"""
from __future__ import annotations

import json
import os
import secrets
import sys

IS_WINDOWS = sys.platform.startswith("win")
DEFAULT_PORT = 47851          # Writer; Calc +1, Impress +2
KINDS = ("wps", "et", "wpp")  # ten loai app dung chung voi Office/WPS


def data_dir() -> str:
    if IS_WINDOWS:
        return os.path.join(os.environ.get("LOCALAPPDATA", os.path.expanduser("~")), "AxiomOffice")
    return os.path.join(os.environ.get("XDG_DATA_HOME", os.path.expanduser("~/.local/share")), "axiom-office")


def runtime_dir() -> str:
    if IS_WINDOWS:
        return data_dir()
    base = os.environ.get("XDG_RUNTIME_DIR") or os.path.expanduser("~/.cache")
    return os.path.join(base, "axiom-office")


def sessions_dir() -> str:
    return os.path.join(runtime_dir(), "sessions")


def config_file() -> str:
    base = os.environ.get("XDG_CONFIG_HOME", os.path.expanduser("~/.config"))
    return os.path.join(base, "axiom-office", "config.json")


def _registry_value(name: str):
    try:
        import winreg

        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\AxiomOffice") as key:
            return winreg.QueryValueEx(key, name)[0]
    except OSError:
        return None


def _json_config() -> dict:
    try:
        with open(config_file(), encoding="utf-8") as handle:
            return json.load(handle)
    except (OSError, ValueError):
        return {}


def value(name: str, default=None):
    """Bien moi truong AXIOM_<NAME> (test) -> HKCU (Windows) / config.json (Linux) -> mac dinh."""
    env = os.environ.get("AXIOM_" + name.upper())
    if env is not None:
        return env
    found = _registry_value(name) if IS_WINDOWS else _json_config().get(name)
    return default if found is None else found


def token() -> str:
    found = value("Token")
    if found:
        return str(found)
    if IS_WINDOWS:
        return ""  # install.ps1 sinh token; thieu thi bridge chay khong token nhu add-in
    # Linux: sinh token lan dau, luu config.json quyen 0600.
    generated = secrets.token_hex(16)
    path = config_file()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    data = _json_config()
    data["Token"] = generated
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(data, handle, indent=2)
    os.chmod(path, 0o600)
    return generated


def base_port() -> int:
    try:
        return int(value("PortLibreOffice", DEFAULT_PORT))
    except (TypeError, ValueError):
        return DEFAULT_PORT


def enabled() -> bool:
    return str(value("Enabled", 1)) not in ("0", "false", "False")
