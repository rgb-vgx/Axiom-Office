"""Spec: loi HTTP cua Agent Core phai co thong diep doc duoc, va ai.ask phai HUY luot khi het gio.

Hien tai `core.call` doc body loi bang json.loads: Core tra HTML/plain text (vd 502 cua proxy) thi nguoi
dung nhan JSONDecodeError, con Core tat thi URLError tho. `ai.ask` cho 330 s roi bo mac luot chay tren Core.

    python3 tests/lo/test_general_core.py
"""
from __future__ import annotations

import json
import os
import socket
import sys
import threading
import types
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
if "uno" not in sys.modules:
    sys.modules["uno"] = types.ModuleType("uno")
try:                                    # may CI khong co python3-uno
    import unohelper  # noqa: F401
except Exception:  # noqa: BLE001
    _helper = types.ModuleType("unohelper")
    _helper.Base = type("Base", (), {})
    sys.modules["unohelper"] = _helper
if "com" not in sys.modules:
    _com = types.ModuleType("com")
    _sun = types.ModuleType("com.sun")
    _star = types.ModuleType("com.sun.star")
    _awt = types.ModuleType("com.sun.star.awt")
    _awt.XCallback = type("XCallback", (), {})
    _com.sun = _sun
    _sun.star = _star
    _star.awt = _awt
    sys.modules.update({"com": _com, "com.sun": _sun, "com.sun.star": _star, "com.sun.star.awt": _awt})
sys.path.insert(0, PYTHONPATH)

from axiom import core  # noqa: E402


class CoreHandler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def _send(self, status, body, kind="application/json"):
        data = body if isinstance(body, bytes) else body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", kind)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/bad":
            self._send(500, "<html>proxy loi</html>", "text/html")
        elif self.path in ("/v1/runs/r_2", "/v1/runs/r_3"):
            self._send(200, json.dumps({"ok": True, "result": {"status": "completed", "reply": "xong"}}))
        else:
            self.server.hits.append(("GET", self.path))
            self._send(200, json.dumps({"ok": True, "result": {"status": "running"}}))

    def do_POST(self):
        length = int(self.headers.get("Content-Length") or 0)
        if length:
            self.rfile.read(length)
        self.server.hits.append(("POST", self.path))
        self._send(200, json.dumps({"ok": True, "result": {"cancelled": True}}))


class GeneralCoreTest(unittest.TestCase):
    def setUp(self):
        self.server = ThreadingHTTPServer(("127.0.0.1", 0), CoreHandler)
        self.server.hits = []
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.base = "http://127.0.0.1:%d" % self.server.server_address[1]

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()

    def test_body_loi_khong_phai_json_van_co_thong_diep(self):
        with self.assertRaises(core.CoreError) as caught:
            core.call(self.base, "GET", "/bad")
        message = str(caught.exception)
        self.assertIn("500", message)
        self.assertIn("proxy loi", message)

    def test_core_tat_bao_loi_ro_rang(self):
        sock = socket.socket()
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
        sock.close()
        with self.assertRaises(core.CoreError) as caught:
            core.call("http://127.0.0.1:%d" % port, "GET", "/v1/x", timeout=1)
        self.assertIn("Agent Core", str(caught.exception))

    def test_wait_run_het_gio_thi_huy_luot(self):
        timed_out, run = core.wait_for_run(self.base, "r_1", poll=0.01, timeout=0.2)
        self.assertTrue(timed_out)
        self.assertEqual(run.get("status"), "running")
        self.assertIn(("POST", "/v1/runs/r_1/cancel"), self.server.hits)

    def test_wait_run_xong_thi_khong_huy(self):
        timed_out, run = core.wait_for_run(self.base, "r_2", poll=0.01, timeout=1)
        self.assertFalse(timed_out)
        self.assertEqual(run.get("status"), "completed")
        self.assertNotIn(("POST", "/v1/runs/r_2/cancel"), self.server.hits)


if __name__ == "__main__":
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    unittest.main()
