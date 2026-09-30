"""Unit test cho logic pane (axiom.chat, axiom.core) - chay KHONG can LibreOffice lan Agent Core.

Phan UI (awt) khong test o day; test nay giu dung phan de sai nhat: dem thao tac de "Hoan tac luot nay",
the xac nhan, ghi nho vua ghi, dong trang thai, va doc SSE cua Core (bang server gia).

    python tests\\lo\\test_chat.py
"""
from __future__ import annotations

import json
import os
import shutil
import sys
import tempfile
import threading
import time
import types
import unittest
from unittest import mock
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
if "uno" not in sys.modules:      # axiom.core chi can stdlib; axiom.chat thuan Python
    sys.modules["uno"] = types.ModuleType("uno")
sys.path.insert(0, PYTHONPATH)

from axiom import chat, config, core, sessions, theme  # noqa: E402


class ItemTests(unittest.TestCase):
    """Muc co cau truc ma pane ve thanh bong bong / dong thao tac / the (chatview.py)."""

    def kinds(self, session):
        return [item["kind"] for item in session.items]

    def test_turn_items(self):
        session = chat.ChatSession("wps")
        session.begin("Chèn bảng")
        session.handle_event({"type": "skill.loaded", "data": {"name": "the-thuc-van-ban"}})
        session.handle_event({"type": "tool.finished", "data": {"tool": "load_skill", "ok": True}})
        session.handle_event({"type": "tool.finished", "data": {"action": "writer.insertTable", "ok": True}})
        session.handle_event({"type": "tool.finished", "data": {"action": "writer.heading", "ok": False, "error": "boom"}})
        self.assertTrue(session.thinking)
        session.finish({"status": "completed", "reply": "Xong."})
        # tool.finished cua load_skill khong lap lai dong "Dung ky nang".
        self.assertEqual(self.kinds(session), ["user", "skill", "tool", "tool", "ai"])
        self.assertEqual(session.items[3]["state"], "error")
        self.assertEqual(session.items[3]["error"], "boom")
        self.assertFalse(session.thinking)

    def test_confirm_card_lifecycle(self):
        session = chat.ChatSession("wpp")
        session.begin("Xoá slide")
        session.handle_event({"type": "confirm.required", "data": {"confirmationId": "cf_1", "action": "wpp.deleteSlide",
                                                                   "reason": "xoá", "paramsPreview": "{\"slide\":3}"}})
        card = session.items[-1]
        self.assertEqual((card["kind"], card["state"], card["rev"]), ("confirm", "pending", 0))
        self.assertFalse(session.thinking)          # dang cho nguoi dung: khong hien "..."
        session.handle_event({"type": "confirm.resolved", "data": {"confirmationId": "cf_1", "approved": False, "by": "timeout"}})
        self.assertEqual(card["state"], "timeout")
        self.assertEqual(card["rev"], 1)            # rev tang -> pane ve lai dung the nay

    def test_pending_confirm_closed_when_run_ends(self):
        session = chat.ChatSession("wps")
        session.begin("x")
        session.handle_event({"type": "confirm.required", "data": {"confirmationId": "cf_2", "action": "writer.saveAs"}})
        session.finish({"status": "cancelled"})
        self.assertEqual(session.items[1]["state"], "cancelled")
        self.assertEqual((session.items[-1]["kind"], session.items[-1]["text"]), ("info", "Đã dừng."))

    def test_memory_and_error_items(self):
        session = chat.ChatSession("wps")
        session.begin("x")
        session.handle_event({"type": "memory.written", "data": {"id": "m1", "text": "Người ký: A"}})
        memory = session.items[-1]
        session.touch(memory, state="deleted")
        self.assertEqual((memory["state"], memory["rev"]), ("deleted", 1))
        session.fail("khong ket noi duoc Agent Core (http://127.0.0.1:47840)")
        error = session.items[-1]
        self.assertEqual(error["kind"], "error")
        self.assertEqual(error["title"], "Không kết nối được Agent Core")

    def test_reset_clears_items(self):
        session = chat.ChatSession("wps")
        session.begin("x")
        session.note("Đã hoàn tác 1 thao tác.")
        session.reset()
        self.assertEqual(session.items, [])


class ThemeTests(unittest.TestCase):
    def test_labels(self):
        self.assertEqual(theme.label_for("writer.insertTable"), "Chèn bảng")
        self.assertEqual(theme.label_for("et.saveAs"), "Lưu tệp")
        self.assertEqual(theme.label_for("wpp.exportPdf"), "Xuất PDF")
        self.assertEqual(theme.label_for("load_skill"), "Dùng kỹ năng")
        self.assertEqual(theme.label_for("writer.somethingNew"), "Thao tác trên tài liệu")
        self.assertEqual(theme.label_for(None), "Thao tác trên tài liệu")

    def test_empty_state_per_app(self):
        for kind, noun in (("wps", "văn bản"), ("et", "bảng tính"), ("wpp", "bản trình chiếu")):
            title, description, suggestions = theme.empty_state(kind)
            self.assertIn(noun, title)
            self.assertEqual(len(suggestions), 3)

    def test_plain_reply_strips_markdown_marks(self):
        self.assertEqual(theme.plain_reply("**Xong** `writer.heading`"), "Xong writer.heading")

    def test_error_summary_bo_ten_kieu_loi_va_cat_bot(self):
        # Ten kieu loi (tu dai khong co khoang trang) lam dong thao tac khong xuong dong duoc -> bo di.
        self.assertEqual(theme.error_summary("ArgumentException: 'rows' and 'cols' are required"), "'rows' and 'cols' are required")
        self.assertEqual(theme.error_summary("com.sun.star.lang.DisposedException: Document closed"), "Document closed")
        self.assertEqual(theme.error_summary("RuntimeError: boom"), "boom")
        self.assertEqual(theme.error_summary("binh thuong"), "binh thuong")
        self.assertEqual(theme.error_summary(""), "lỗi")
        self.assertEqual(theme.error_summary(None), "lỗi")
        long_error = "x" * 300
        self.assertEqual(theme.error_summary(long_error), "x" * 157 + "…")

    def _png_size(self, data: bytes):
        self.assertEqual(data[:8], b"\x89PNG\r\n\x1a\n")
        import struct

        return struct.unpack(">II", data[16:24])

    def _pixels(self, data: bytes, width: int):
        import struct
        import zlib

        pos, idat = 8, b""
        while pos < len(data):
            length = struct.unpack(">I", data[pos:pos + 4])[0]
            if data[pos + 4:pos + 8] == b"IDAT":
                idat += data[pos + 8:pos + 8 + length]
            pos += 12 + length
        raw = zlib.decompress(idat)
        stride = 1 + width * 3
        rows = [raw[i * stride + 1:(i + 1) * stride] for i in range(len(raw) // stride)]
        return [[tuple(row[x * 3:x * 3 + 3]) for x in range(width)] for row in rows]

    def test_corner_png_geometry(self):
        """Goc 'tl': diem ngoai cung tron = mau nen pane, goc trong = mau to."""
        data = theme.corner_png(10, "tl", theme.USER_BG, theme.PANE_BG)
        self.assertEqual(self._png_size(data), (10, 10))
        pixels = self._pixels(data, 10)
        self.assertEqual(pixels[0][0], theme.rgb(theme.PANE_BG))
        self.assertEqual(pixels[9][9], theme.rgb(theme.USER_BG))
        br = self._pixels(theme.corner_png(10, "br", theme.USER_BG, theme.PANE_BG), 10)
        self.assertEqual(br[9][9], theme.rgb(theme.PANE_BG))
        self.assertEqual(br[0][0], theme.rgb(theme.USER_BG))

    def test_corner_png_border_ring(self):
        pixels = self._pixels(theme.corner_png(10, "tl", theme.AI_BG, theme.PANE_BG, theme.AI_BORDER), 10)
        # Diem nam tren cung tron (ban kinh ~9.5) mang mau vien, khong phai mau to.
        self.assertNotEqual(pixels[9][0], theme.rgb(theme.AI_BG))
        self.assertEqual(pixels[9][9], theme.rgb(theme.AI_BG))

    def test_icons(self):
        for kind in ("ok", "error", "running", "dot"):
            data = theme.icon_png(kind, 12)
            self.assertEqual(self._png_size(data), (12, 12))
        center = self._pixels(theme.icon_png("dot", 12), 12)[6][6]
        self.assertEqual(center, theme.rgb(theme.ACCENT))


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


class CoreLaunchTests(unittest.TestCase):
    """Khoi dong Agent Core (axiom.core): CoreExe -> vi tri cai mac dinh cua scripts/linux/install.sh."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="axiom-core-")
        self.addCleanup(shutil.rmtree, self.tmp, True)
        # config.IS_WINDOWS la bien module: gia lap Linux tren may Windows va nguoc lai.
        mock.patch.object(core.config, "IS_WINDOWS", False).start()
        mock.patch.object(core.config, "data_dir", lambda: self.tmp).start()
        mock.patch.object(core.config, "value", lambda name, default=None: None).start()
        self.addCleanup(mock.patch.stopall)

    def _installed(self, executable: bool) -> str:
        path = os.path.join(self.tmp, "core", "AxiomOffice.Core")
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as handle:
            handle.write("#!/bin/sh\n")
        os.chmod(path, 0o755 if executable else 0o644)
        return path

    def test_core_exe_uutien_coreexe_roi_vi_tri_cai(self):
        installed = self._installed(executable=True)
        self.assertEqual(core.core_exe(), installed)
        # CoreExe tro sai duong dan -> van dung vi tri cai.
        mock.patch.object(core.config, "value", lambda name, default=None: "/khong/co/AxiomOffice.Core").start()
        self.assertEqual(core.core_exe(), installed)

    def test_core_exe_rong_khi_thieu_hoac_khong_chay_duoc(self):
        self.assertEqual(core.core_exe(), "")
        if os.name != "nt":                       # Windows: os.access(X_OK) chi kiem tra file ton tai
            self._installed(executable=False)     # co file nhung thieu quyen chay
            self.assertEqual(core.core_exe(), "")

    def test_launch_tach_session_tren_linux(self):
        calls = []
        with mock.patch.object(core.subprocess, "Popen", lambda argv, **kw: calls.append((argv, kw))):
            core.launch("/opt/axiom/AxiomOffice.Core")
        argv, options = calls[0]
        self.assertEqual(argv, ["/opt/axiom/AxiomOffice.Core"])
        self.assertEqual(options["cwd"], "/opt/axiom")
        self.assertIs(options["start_new_session"], True)   # khong chet theo LibreOffice/SIGHUP
        self.assertNotIn("creationflags", options)

    def test_launch_an_console_tren_windows(self):
        core.config.IS_WINDOWS = True
        calls = []
        with mock.patch.object(core.subprocess, "Popen", lambda argv, **kw: calls.append((argv, kw))):
            core.launch(r"C:\Axiom\AxiomOffice.Core.exe")
        options = calls[0][1]
        self.assertIn("creationflags", options)
        self.assertNotIn("start_new_session", options)


class SessionSweepTests(unittest.TestCase):
    """Don file session cua lan chay truoc (axiom.sessions.sweep): pid da chet, hoac heartbeat qua cu.

    File cua add-in Windows ({pid}.json, khong co "-kind") va file tam khong bi dong toi.
    """

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="axiom-sessions-")
        self.addCleanup(shutil.rmtree, self.tmp, True)
        mock.patch.object(config, "sessions_dir", lambda: self.tmp).start()
        self.addCleanup(mock.patch.stopall)

    def _write(self, name: str, seen: float | None = None, pid: int = 0) -> str:
        path = os.path.join(self.tmp, name)
        with open(path, "w", encoding="utf-8") as handle:
            json.dump({"pid": pid or int(name.split("-")[0]), "lastSeenEpoch": time.time() if seen is None else seen},
                      handle)
        return path

    def _left(self) -> list:
        return sorted(os.listdir(self.tmp))

    def test_xoa_file_cua_tien_trinh_da_chet(self):
        dead = self._write("999999-wps.json")     # heartbeat vua ghi nhung pid da chet
        alive = self._write("%d-wps.json" % os.getpid(), pid=os.getpid())
        with mock.patch.object(sessions, "_alive", lambda pid: pid == os.getpid()):
            sessions.sweep()
        self.assertFalse(os.path.exists(dead))
        self.assertTrue(os.path.exists(alive))

    def test_alive_theo_tien_trinh_that(self):
        self.assertTrue(sessions._alive(os.getpid()))
        self.assertFalse(sessions._alive(0) or sessions._alive(-1))

    def test_xoa_file_heartbeat_qua_cu(self):
        with mock.patch.object(sessions, "_alive", lambda pid: True):
            old = self._write("1234-et.json", seen=time.time() - sessions.STALE_SECONDS - 60)
            fresh = self._write("1235-et.json")
            sessions.sweep()
        self.assertFalse(os.path.exists(old))
        self.assertTrue(os.path.exists(fresh))

    def test_bo_qua_file_khong_phai_session(self):
        # Add-in ghi {pid}.json; them file tam .tmp dang ghi do -> khong xoa.
        addin = self._write("4321.json", pid=999999)
        temp = self._write("999999-wps.json.tmp", pid=999999)
        other = self._write("999999-slides.json", pid=999999)      # kind la khong thuoc KINDS
        sessions.sweep()
        self.assertEqual(self._left(), sorted([os.path.basename(addin), os.path.basename(temp),
                                               os.path.basename(other)]))

    def test_ten_la_chi_xet_heartbeat(self):
        weird = self._write("abc-wps.json", seen=time.time() - sessions.STALE_SECONDS - 60, pid=1)
        sessions.sweep()
        self.assertFalse(os.path.exists(weird))      # ten la + cu -> van don
        kept = os.path.join(self.tmp, "xyz-et.json")
        with open(kept, "w", encoding="utf-8") as handle:
            json.dump({"lastSeenEpoch": time.time()}, handle)
        sessions.sweep()
        self.assertTrue(os.path.exists(kept))        # ten la nhung con moi -> giu


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
