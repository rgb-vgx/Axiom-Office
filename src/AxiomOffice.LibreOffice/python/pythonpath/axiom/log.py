"""Log chung voi add-in: %LOCALAPPDATA%\\AxiomOffice\\bridge.log (Windows) / ~/.local/share/axiom-office/bridge.log."""
from __future__ import annotations

import datetime
import os
import threading

from . import config

_LOCK = threading.Lock()
MAX_BYTES = 10 * 1024 * 1024


def _path() -> str:
    return os.path.join(config.data_dir(), "bridge.log")


def _write(level: str, message: str) -> None:
    line = "%s [%s] [pid %d] lo: %s\n" % (
        datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S.%f")[:-3], level, os.getpid(), message)
    try:
        with _LOCK:
            path = _path()
            os.makedirs(os.path.dirname(path), exist_ok=True)
            if os.path.exists(path) and os.path.getsize(path) > MAX_BYTES:
                os.replace(path, path + ".1")
            with open(path, "a", encoding="utf-8") as handle:
                handle.write(line)
    except OSError:
        pass


def info(message: str) -> None:
    _write("INFO", message)


def error(message: str) -> None:
    _write("ERROR", message)
