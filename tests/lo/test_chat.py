"""Unit test cho logic pane (axiom.chat, axiom.core) - chay KHONG can LibreOffice lan Agent Core.

Phan UI (awt) khong test o day; test nay giu dung phan de sai nhat: dem thao tac de "Hoan tac luot nay",
the xac nhan, ghi nho vua ghi, dong trang thai, va doc SSE cua Core (bang server gia).

    python tests\\lo\\test_chat.py
"""
from __future__ import annotations

import json
import os
import sys
import threading
import types
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
if "uno" not in sys.modules:      # axiom.core chi can stdlib; axiom.chat thuan Python
    sys.modules["uno"] = types.ModuleType("uno")
sys.path.insert(0, PYTHONPATH)

from axiom import chat, core  # noqa: E402


class EditCountTests(unittest.TestCase):
    def test_read_only_actions_are_not_edits(self):
        for action in ("writer.getText", "writer.checkTables", "writer.save", "et.readRange", "et.saveAs",
                       "wpp.listSlides", "wpp.checkLayout", "wpp.exportPdf"):
            self.assertFalse(chat.is_edit(action), action)

    def test_mutating_actions_are_edits(self):
        for action in ("writer.appendText", "writer.insertTable", "writer.formatTable", "et.writeRange",
                       "et.formatSelection", "wpp.addSlide", "wpp.deleteSlide", "wpp.setNotes"):
            self.assertTrue(chat.is_edit(action), action)

    def test_unknown_tools_are_not_edits(self):
        for action in (None, "", "load_skill", "memory.list", "app.screenshot", "ui.askpane"):
            self.assertFalse(chat.is_edit(action), action)


class SessionTests(unittest.TestCase):
    def _session(self):
        self.changes = []
        return chat.ChatSession("wps", on_change=lambda: self.changes.append(1))

    def test_turn_records_reply_and_summary(self):
        session = self._session()
        session.begin("Thêm bảng điểm")
        session.handle_event({"type": "tool.finished", "data": {"action": "writer.insertTable", "ok": True}})
        session.handle_event({"type": "tool.finished", "data": {"action": "writer.getText", "ok": True}})
        session.finish({"status": "completed", "reply": "Đã thêm bảng.", "seconds": 12.5, "rounds": 3})
        self.assertFalse(session.running)
        self.assertEqual(session.edit_count(), 1)       # getText khong tinh
        self.assertIn("✓ writer.insertTable", session.lines)
        self.assertIn("AI: Đã thêm bảng.", session.lines)
        self.assertEqual(session.summary(), "Xong trong 12s · 1 thao tác")
        self.assertTrue(self.changes)                    # moi thay doi deu bao cho UI

    def test_only_successful_edits_count(self):
        session = self._session()
        session.begin("x")
        session.handle_event({"type": "tool.finished", "data": {"action": "writer.appendText", "ok": False, "error": "boom"}})
        self.assertEqual(session.edit_count(), 0)
        self.assertIn("✗ writer.appendText: boom", session.lines)

    def test_skill_memory_and_confirm_events(self):
        session = self._session()
        session.begin("Soạn công văn")
        session.handle_event({"type": "skill.loaded", "data": {"name": "van-ban-hanh-chinh"}})
        session.handle_event({"type": "memory.written", "data": {"id": "m1", "text": "Người ký: Nguyễn Văn A"}})
        session.handle_event({"type": "confirm.required", "data": {"confirmationId": "cf_1", "action": "writer.saveAs",
                                                                   "reason": "lưu đè"}})
        self.assertIn("✓ Dùng kỹ năng: van-ban-hanh-chinh", session.lines)
        self.assertEqual(session.notes, [{"id": "m1", "text": "Người ký: Nguyễn Văn A"}])
        self.assertEqual(session.pending["confirmationId"], "cf_1")
        session.handle_event({"type": "confirm.resolved", "data": {"confirmationId": "cf_1", "approved": True}})
        self.assertIsNone(session.pending)

    def test_failures_and_cancel(self):
        session = self._session()
        session.begin("x")
        session.finish({"status": "cancelled"})
        self.assertIn("(đã dừng)", session.lines)
        session.begin("y")
        session.finish({"status": "failed", "error": "Agent Core tra ve loi"})
        self.assertEqual(session.error, "Agent Core tra ve loi")
        self.assertIn("Lỗi: Agent Core tra ve loi", session.lines)
        self.assertIn("Lỗi", session.summary())

    def test_fail_and_reset(self):
        session = self._session()
        session.begin("x")
        session.fail("khong ket noi duoc Agent Core")
        self.assertFalse(session.running)
        self.assertEqual(session.summary(), "Lỗi · nhấn Cài đặt để kiểm tra Agent Core")
        session.reset()
        self.assertEqual(session.lines, [])
        self.assertEqual(session.edit_count(), 0)
        self.assertEqual(session.status, "Sẵn sàng")

    def test_clear_edits_after_undo(self):
        session = self._session()
        session.begin("x")
        session.handle_event({"type": "tool.finished", "data": {"action": "writer.appendText", "ok": True}})
        session.clear_edits()
        self.assertEqual(session.edit_count(), 0)

    def test_tick_updates_status_only(self):
        session = self._session()
        session.begin("x")
        session.tick({"rounds": 4, "seconds": 8.2})
        self.assertIn("4 vòng", session.status)


class CoreSseTests(unittest.TestCase):
    """Doc SSE that: server gia gui dung dinh dang cua Agent Core (event:/data: + ping)."""

    @classmethod
    def setUpClass(cls):
        outer = cls

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *args):
                pass

            def do_GET(self):  # noqa: N802
                if self.path.startswith("/v1/runs/r_test/events"):
                    self.send_response(200)
                    self.send_header("Content-Type", "text/event-stream; charset=utf-8")
                    self.send_header("Cache-Control", "no-cache")
                    body = (
                        "event: run.started\n"
                        'data: {"seq":1,"type":"run.started","data":{"model":"test-model"}}\n\n'
                        "event: ping\n"
                        'data: {"time":"now"}\n\n'
                        "event: tool.finished\n"
                        'data: {"seq":2,"type":"tool.finished","data":{"action":"writer.appendText","ok":true}}\n\n'
                    ).encode("utf-8")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                else:
                    self.send_response(404)
                    self.send_header("Content-Length", "0")
                    self.end_headers()

        cls.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        cls.port = cls.server.server_address[1]
        threading.Thread(target=cls.server.serve_forever, daemon=True).start()
        outer.base = "http://127.0.0.1:%d" % cls.port

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()

    def test_stream_events_skips_ping_and_parses_data(self):
        seen = []
        core.stream_events(self.base, "r_test", seen.append)
        types_seen = [item["type"] for item in seen]
        self.assertEqual(types_seen, ["run.started", "tool.finished"])
        self.assertEqual(seen[0]["data"]["model"], "test-model")
        self.assertTrue(seen[1]["data"]["ok"])

    def test_stream_events_stops_on_stop_event(self):
        stop = threading.Event()
        stop.set()
        seen = []
        core.stream_events(self.base, "r_test", seen.append, stop)
        self.assertEqual(seen, [])


if __name__ == "__main__":
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    unittest.main(verbosity=2)
