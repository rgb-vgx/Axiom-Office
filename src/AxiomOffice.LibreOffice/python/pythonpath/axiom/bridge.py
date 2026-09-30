"""Bridge HTTP cua LibreOffice (LibreOffice_arch.md muc 5, 6): ba listener (Writer/Calc/Impress) tren
127.0.0.1, cung giao thuc voi add-in Office/WPS: /health (khong token), /commands, /cmd, /session.
Bao ve giong HttpBridge.cs: chan Origin (403), sai token (401), body khong phai JSON (415)."""
from __future__ import annotations

import json
import os
import threading
import time
import traceback
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from . import VERSION, commands, config, documents, log, sessions, values
from .gate import BusyError, UnoGate

_STATE: dict = {}
_START_LOCK = threading.Lock()
PORT_ATTEMPTS = 5          # ban thi thu +10, +20...


class Env:
    """Moi truong mot lenh: ctx UNO, loai app, tai lieu dich (tim khi can)."""

    def __init__(self, ctx, kind: str, gate: UnoGate):
        self.ctx = ctx
        self.kind = kind
        self.gate = gate
        self._document = None

    @property
    def document(self):
        if self._document is None:
            self._document = documents.active(self.ctx, self.kind)
        return self._document


def execute(ctx, gate: UnoGate, kind: str, action: str, params: dict) -> dict:
    command = commands.REGISTRY.get(action)
    if command is None:
        return {"ok": False, "error": "unknown action: " + str(action)}
    if command.kind is not None and command.kind != kind:
        return {"ok": False, "error": "'%s' is not available on the %s port (use the port of app kind '%s')" % (action, kind, command.kind)}

    def run():
        env = Env(ctx, kind, gate)
        if command.undo:
            with documents.undo_step(env.document, action):
                return command.handler(env, params or {})
        return command.handler(env, params or {})

    try:
        result = gate.run(run) if command.gated else run()
        return {"ok": True, "result": result}
    except (values.ParamError, ValueError) as exc:
        return {"ok": False, "error": "ArgumentException: " + str(exc)}
    except documents.NoDocumentError as exc:
        return {"ok": False, "error": "InvalidOperationException: " + str(exc)}
    except BusyError as exc:
        return {"ok": False, "error": "Busy: " + str(exc)}
    except Exception as exc:  # noqa: BLE001 - loi UNO tra ve cho model tu xu ly
        log.error("Action failed: %s => %s\n%s" % (action, exc, traceback.format_exc(limit=4)))
        name = type(exc).__name__
        message = getattr(exc, "Message", None) or str(exc)
        return {"ok": False, "error": "%s: %s" % (name, message)}


def _handler_class(ctx, gate: UnoGate, kind: str, port_ref: list):
    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"
        server_version = "AxiomOfficeLibreOffice/" + VERSION

        def log_message(self, *args):
            pass

        def _send(self, status: int, payload) -> None:
            body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def _guard(self) -> bool:
            if self.headers.get("Origin"):
                self._send(403, {"ok": False, "error": "forbidden: requests with an Origin header are rejected"})
                return False
            if self.path.split("?")[0].rstrip("/") == "/health":
                return True
            token = _STATE.get("token", "")
            if token and self.headers.get("X-Auth-Token") != token:
                self._send(401, {"ok": False, "error": "unauthorized"})
                return False
            return True

        def do_GET(self):  # noqa: N802
            if not self._guard():
                return
            path = self.path.split("?")[0].rstrip("/") or "/"
            if path == "/health":
                self._send(200, {"ok": True, "result": {
                    "app": kind, "family": "libreoffice", "pid": os.getpid(), "port": port_ref[0],
                    "version": VERSION, "stuck": gate.stuck,
                    "log": os.path.join(config.data_dir(), "bridge.log")}})
            elif path == "/commands":
                self._send(200, {"ok": True, "result": {"version": VERSION, "commands": commands.catalog()}})
            elif path == "/session":
                info = gate.run_quiet(lambda: documents.describe(documents.active(ctx, kind, required=False)), 5.0, {})
                self._send(200, {"ok": True, "result": {"app": kind, "family": "libreoffice", "port": port_ref[0],
                                                        "pid": os.getpid(), "document": info}})
            else:
                self._send(404, {"ok": False, "error": "not found: " + path})

        def do_POST(self):  # noqa: N802
            if not self._guard():
                return
            path = self.path.split("?")[0].rstrip("/")
            if path != "/cmd":
                self._send(404, {"ok": False, "error": "not found: " + path})
                return
            if "application/json" not in (self.headers.get("Content-Type") or "").lower():
                self._send(415, {"ok": False, "error": "Content-Type must be application/json"})
                return
            length = int(self.headers.get("Content-Length") or 0)
            try:
                body = json.loads(self.rfile.read(length).decode("utf-8") or "{}")
            except ValueError:
                self._send(400, {"ok": False, "error": "invalid JSON body"})
                return
            if not isinstance(body, dict) or "action" not in body:
                self._send(400, {"ok": False, "error": "missing 'action' field"})
                return
            params = body.get("params")
            self._send(200, execute(ctx, gate, kind, str(body["action"]), params if isinstance(params, dict) else {}))

    return Handler


def start(ctx) -> None:
    """Goi tu job OnStartApp (co the nhieu lan): chi bat mot lan moi process."""
    with _START_LOCK:
        if _STATE.get("started"):
            return
        if not config.enabled():
            log.info("bridge disabled (Enabled=0)")
            return
        commands.load_all()
        gate = UnoGate(ctx)
        _STATE.update(started=True, token=config.token(), gate=gate, servers=[], ctx=ctx)
        base = config.base_port()
        for index, kind in enumerate(config.KINDS):
            port_ref = [0]
            server = None
            for attempt in range(PORT_ATTEMPTS):
                port = base + index + attempt * 10
                try:
                    server = ThreadingHTTPServer(("127.0.0.1", port), _handler_class(ctx, gate, kind, port_ref))
                    port_ref[0] = port
                    break
                except OSError:
                    continue
            if server is None:
                log.error("no free port for %s (tried %d..)" % (kind, base + index))
                continue
            server.daemon_threads = True
            threading.Thread(target=server.serve_forever, name="axiom-bridge-" + kind, daemon=True).start()
            _STATE["servers"].append((kind, port_ref[0], server))
            log.info("HttpBridge listening on http://127.0.0.1:%d/ (kind=%s, libreoffice)" % (port_ref[0], kind))
        sessions.start(ctx, gate, [(k, p) for k, p, _ in _STATE["servers"]])
        _watch_termination(ctx)


def stop() -> None:
    sessions.stop()
    for kind, port, server in _STATE.get("servers", []):
        try:
            server.shutdown()
            server.server_close()
        except Exception:  # noqa: BLE001
            pass
    _STATE["servers"] = []
    log.info("HttpBridge stopped (libreoffice)")


def _watch_termination(ctx) -> None:
    import unohelper
    from com.sun.star.frame import XTerminateListener

    class Listener(unohelper.Base, XTerminateListener):
        def queryTermination(self, event):  # noqa: N802
            pass

        def notifyTermination(self, event):  # noqa: N802
            stop()

        def disposing(self, event):
            pass

    try:
        documents.desktop(ctx).addTerminateListener(Listener())
    except Exception as exc:  # noqa: BLE001
        log.error("cannot watch termination: %s" % exc)


def gate() -> UnoGate:
    """UnoGate dang dung (pane can de day cap nhat giao dien ve main thread)."""
    with _START_LOCK:
        self_gate = _STATE.get("gate")
        if self_gate is None:
            self_gate = UnoGate(_STATE["ctx"])
            _STATE["gate"] = self_gate
        return self_gate


def pane_target(kind: str | None = None) -> tuple[str, int]:
    """(kind, port) cho pane: theo tai lieu dang mo, khong thi kind dau tien dang co bridge."""
    servers = _STATE.get("servers") or []
    if kind:
        for server_kind, port, _ in servers:
            if server_kind == kind:
                return kind, port
    if servers:
        server_kind, port, _ = servers[0]
        return server_kind, port
    return kind or config.KINDS[0], config.base_port()


def run_on_main(ctx, uno_gate, kind: str, action: str, params: dict) -> dict:
    """Chay mot lenh NGAY tren thread dang goi (pane da o main thread: qua gate se tu treo)."""
    command = commands.REGISTRY.get(action)
    if command is None:
        return {"ok": False, "error": "unknown action: " + str(action)}
    try:
        env = Env(ctx, kind, uno_gate)
        if command.undo:
            with documents.undo_step(env.document, action):
                return {"ok": True, "result": command.handler(env, params or {})}
        return {"ok": True, "result": command.handler(env, params or {})}
    except (values.ParamError, ValueError) as exc:
        return {"ok": False, "error": "ArgumentException: " + str(exc)}
    except Exception as exc:  # noqa: BLE001
        log.error("Action failed (pane): %s => %s" % (action, exc))
        return {"ok": False, "error": "%s: %s" % (type(exc).__name__, exc)}


def uptime_started() -> float:
    return _STATE.setdefault("t0", time.time())
