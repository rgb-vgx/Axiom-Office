"""Logic hoi thoai cua pane (LibreOffice_arch.md muc 10): thuan Python, khong import uno/awt.

`ChatSession` giu transcript, trang thai luot chay, the xac nhan dang cho, ghi nho vua ghi va dem so thao
tac sua tai lieu de "Hoan tac luot nay". `panel.py` chi ve ra awt; `tests/lo/test_chat.py` test truc tiep
voi mot backend gia (khong can LibreOffice lan Agent Core).
"""
from __future__ import annotations

# Lenh chi doc/luu: khong tinh la thao tac sua tai lieu (giong AskAiPane.IsWordEdit cua ban Windows).
READ_ONLY_ACTIONS = {
    "writer.getText", "writer.selection", "writer.checkTables", "writer.save", "writer.saveAs", "writer.exportPdf",
    "writer.undo", "et.readRange", "et.listSheets", "et.checkRange", "et.save", "et.saveAs", "et.exportPdf",
    "et.undo", "wps.listSlides", "wpp.listSlides", "wpp.checkLayout", "wpp.save", "wpp.saveAs", "wpp.exportPdf",
}
TERMINAL_STATUS = ("completed", "cancelled", "timedout", "stopped", "failed")


def is_edit(action: str | None) -> bool:
    """Lenh sua tai lieu (moi lenh nhu vay = mot buoc Undo trong undo context cua bridge)."""
    if not action or action in READ_ONLY_ACTIONS:
        return False
    return action.split(".", 1)[0] in ("writer", "et", "wpp")


class ChatSession:
    def __init__(self, kind: str, on_change=None):
        self.kind = kind
        self.on_change = on_change
        self.lines: list[str] = []
        self.notes: list[dict] = []          # ghi nho vua ghi trong luot: {id, text}
        self.pending: dict | None = None     # the xac nhan dang cho
        self.conversation_id: str | None = None
        self.run_id: str | None = None
        self.running = False
        self.status = ""
        self.seconds = 0.0
        self.rounds = 0
        self._edits = 0
        self._tools: list[str] = []
        self.error: str | None = None

    # ---------------------------------------------------------------- luot chay

    def begin(self, prompt: str, run_id: str | None = None) -> None:
        self.running = True
        self.error = None
        self.pending = None
        self.notes = []
        self._edits = 0
        self._tools = []
        self.run_id = run_id
        self.lines.append("Bạn: " + prompt.strip())
        self.status = "Đang chạy…"
        self._changed()

    def finish(self, run: dict) -> None:
        """Ket thuc luot: run la ket qua GET /v1/runs/{id} (hoac dict tuong tu)."""
        self.running = False
        self.pending = None
        status = str(run.get("status") or "")
        self.seconds = float(run.get("seconds") or 0)
        self.rounds = int(run.get("rounds") or 0)
        if status == "completed" and run.get("reply"):
            self.lines.append("AI: " + str(run["reply"]).strip())
        elif status == "cancelled":
            self.lines.append("(đã dừng)")
        elif status == "timedout":
            self.lines.append("(hết thời gian chờ)")
        elif status == "stopped":
            self.lines.append("(đã dừng theo yêu cầu)")
        elif run.get("error"):
            self.error = str(run["error"])
            self.lines.append("Lỗi: " + self.error)
        self.status = self.summary()
        self._changed()

    def fail(self, message: str) -> None:
        self.running = False
        self.pending = None
        self.error = message
        self.lines.append("Lỗi: " + message)
        self.status = "Lỗi"
        self._changed()

    def cancel_requested(self) -> None:
        self.status = "Đang dừng…"
        self._changed()

    # ---------------------------------------------------------------- su kien SSE

    def handle_event(self, event: dict) -> None:
        kind = str(event.get("type") or "")
        data = event.get("data") if isinstance(event.get("data"), dict) else {}
        if kind == "run.started":
            self.conversation_id = data.get("conversationId") or self.conversation_id
            model = data.get("model")
            if model:
                self.status = "Đang chạy · %s" % model
        elif kind == "tool.finished":
            action = data.get("action") or data.get("tool") or "?"
            if data.get("ok"):
                self.lines.append("✓ " + str(action))
                if is_edit(str(action)):
                    self._edits += 1
                self._tools.append(str(action))
            else:
                self.lines.append("✗ %s: %s" % (action, data.get("error") or "lỗi"))
        elif kind == "skill.loaded":
            self.lines.append("✓ Dùng kỹ năng: " + str(data.get("name") or "?"))
        elif kind == "memory.written":
            text = str(data.get("text") or "")
            self.notes.append({"id": data.get("id"), "text": text})
            self.lines.append("✓ Đã ghi nhớ: " + text)
        elif kind == "confirm.required":
            self.pending = data
            self.lines.append("⚠ Cần xác nhận: %s — %s" % (data.get("action"), data.get("reason") or ""))
        elif kind == "confirm.resolved":
            self.pending = None
            self.lines.append("• %s: %s" % ("Đồng ý" if data.get("approved") else "Từ chối", data.get("confirmationId") or ""))
        elif kind in ("run.cancelled", "run.stopped", "run.timedout", "run.failed"):
            if kind == "run.failed":
                self.error = str(data.get("error") or "lỗi")
        self._changed()

    def tick(self, run: dict) -> None:
        """Cap nhat tu vong hoi trang thai (khi khong dung duoc SSE)."""
        rounds = run.get("rounds")
        seconds = run.get("seconds")
        bits = []
        if rounds:
            bits.append("%s vòng" % rounds)
        if seconds:
            bits.append("%ss" % seconds)
        if bits:
            self.status = "Đang chạy · " + " · ".join(bits)
            self._changed()

    # ---------------------------------------------------------------- ket qua cho UI

    def transcript(self) -> str:
        return "\n".join(self.lines)

    def summary(self) -> str:
        if self.running:
            return self.status or "Đang chạy…"
        if self.error:
            return "Lỗi · nhấn Cài đặt để kiểm tra Agent Core"
        seconds = int(self.seconds)
        bits = []
        if seconds:
            bits.append("Xong trong %ds" % seconds)
        if self._edits:
            bits.append("%d thao tác" % self._edits)
        return " · ".join(bits) if bits else "Sẵn sàng"

    def edit_count(self) -> int:
        return self._edits

    def clear_edits(self) -> None:
        """Da hoan tac luot nay: khong dem lai so thao tac do nua."""
        self._edits = 0

    def reset(self) -> None:
        self.lines = []
        self.notes = []
        self.pending = None
        self.conversation_id = None
        self.run_id = None
        self.error = None
        self.seconds = 0.0
        self.rounds = 0
        self._edits = 0
        self._tools = []
        self.status = "Sẵn sàng"
        self._changed()

    def note(self, text: str) -> None:
        """Dong thong bao them tay (vd ket qua hoan tac)."""
        self.lines.append(text)
        self._changed()

    def _changed(self) -> None:
        if self.on_change:
            self.on_change()
