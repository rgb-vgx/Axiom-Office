"""Session registry: moi (process, loai app) mot file {pid}-{kind}.json trong thu muc sessions chung voi
add-in (Windows) de Agent Core / MCP tim thay bridge (LibreOffice_arch.md muc 5.2). Heartbeat 25s."""
from __future__ import annotations

import datetime
import json
import os
import threading
import time

from . import VERSION, config, documents, log

HEARTBEAT_SECONDS = 25
_STOP = threading.Event()
_FILES: list = []


def _path(kind: str) -> str:
    return os.path.join(config.sessions_dir(), "%d-%s.json" % (os.getpid(), kind))


def _write(kind: str, port: int, info: dict, started: str) -> None:
    now = datetime.datetime.now(datetime.timezone.utc)
    payload = {
        "pid": os.getpid(),
        "app": kind,
        "family": "libreoffice",
        "port": port,
        "host": "soffice",
        "version": VERSION,
        "started": started,
        "lastSeen": now.strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z",
        "lastSeenEpoch": time.time(),
        "document": (info or {}).get("name"),
        "documentPath": (info or {}).get("fullName"),
    }
    path = _path(kind)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    temp = path + ".tmp"
    with open(temp, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, ensure_ascii=False)
    os.replace(temp, path)


STALE_SECONDS = 600  # bridge that bi tat dot ngot (kill) de lai file; Core chi coi la cu sau 90s


def _alive(pid: int) -> bool:
    """Tien trinh con song? (Linux: kill 0; Windows: OpenProcess). Khong chac chan thi coi nhu con song."""
    if pid <= 0:
        return False
    if config.IS_WINDOWS:
        import ctypes

        kernel32 = ctypes.windll.kernel32
        handle = kernel32.OpenProcess(0x1000, False, pid)   # PROCESS_QUERY_LIMITED_INFORMATION
        if not handle:
            return kernel32.GetLastError() == 5             # 5 = ACCESS_DENIED: co that, chi la khong mo duoc
        kernel32.CloseHandle(handle)
        return True
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except OSError:
        return True                                         # EPERM: cua tien trinh khac -> con song
    return True


def _stale(path: str, name: str, now: float) -> bool:
    """File cua lan chay truoc: pid da chet (soffice bi kill, dang xuat) hoac heartbeat qua cu."""
    try:
        pid = int(name.split("-")[0])
    except ValueError:
        pid = 0                       # ten la khac: chi con luat heartbeat (khong xoa file la)
    if pid and not _alive(pid):
        return True
    try:
        with open(path, encoding="utf-8") as handle:
            import json

            seen = float(json.load(handle).get("lastSeenEpoch") or 0)
    except (OSError, ValueError, TypeError):
        seen = 0
    return now - seen > STALE_SECONDS


def sweep() -> None:
    """Xoa file {pid}-{kind}.json cua lan chay truoc (pid da chet, hoac heartbeat cu hon 10 phut)."""
    import glob

    now = time.time()
    for path in glob.glob(os.path.join(config.sessions_dir(), "*-*.json")):
        name = os.path.basename(path)
        kind = os.path.splitext(name)[0].split("-")[-1]
        if kind not in config.KINDS or not path.endswith(".json") or ".tmp" in name:
            continue
        if _stale(path, name, now):
            try:
                os.remove(path)
                log.info("Removed stale session %s" % name)
            except OSError:
                pass


def start(ctx, gate, listeners) -> None:
    started = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"
    _FILES.extend(_path(kind) for kind, _ in listeners)
    sweep()

    def beat():
        while True:
            for kind, port in listeners:
                info = gate.run_quiet(lambda k=kind: documents.describe(documents.active(ctx, k, required=False)), 3.0, {})
                try:
                    _write(kind, port, info, started)
                except OSError as exc:
                    log.error("session write failed: %s" % exc)
            if _STOP.wait(HEARTBEAT_SECONDS):
                return

    # Ghi ngay mot lan (khong cho tai lieu - main thread co the chua san sang luc OnStartApp).
    for kind, port in listeners:
        try:
            _write(kind, port, {}, started)
            log.info("Session registered: %s" % _path(kind))
        except OSError as exc:
            log.error("session write failed: %s" % exc)
    threading.Thread(target=beat, name="axiom-sessions", daemon=True).start()


def stop() -> None:
    _STOP.set()
    for path in _FILES:
        try:
            os.remove(path)
            log.info("Session unregistered: %s" % path)
        except OSError:
            pass
