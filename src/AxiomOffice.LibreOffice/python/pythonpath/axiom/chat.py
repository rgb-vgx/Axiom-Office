"""Logic hoi thoai cua pane (LibreOffice_arch.md muc 10): thuan Python, khong import uno/awt.

`ChatSession` giu transcript, trang thai luot chay, the xac nhan dang cho, ghi nho vua ghi va dem so thao
tac sua tai lieu de "Hoan tac luot nay". `panel.py` chi ve ra awt; `tests/lo/test_chat.py` test truc tiep
voi mot backend gia (khong can LibreOffice lan Agent Core).

Hai dang ket qua: `lines` (chuoi, cho log/test) va `items` - danh sach muc co cau truc de pane ve bong bong,
dong tool, the xac nhan... Moi muc la dict co `kind` va `rev`; muc bi sua (vd the xac nhan da tra loi) thi
`rev` tang de giao dien ve lai dung muc do.
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
        self.items: list[dict] = []
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
        self._add("user", text=prompt.strip())
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
            self._add("ai", text=str(run["reply"]).strip())
        elif status == "cancelled":
            self.lines.append("(đã dừng)")
            self._add("info", text="Đã dừng.")
        elif status == "timedout":
            self.lines.append("(hết thời gian chờ)")
            self._add("error", title="Hết thời gian chờ", message="Lượt chạy quá lâu nên đã dừng. Thử chia yêu cầu nhỏ hơn.")
        elif status == "stopped":
            self.lines.append("(đã dừng theo yêu cầu)")
            self._add("info", text="Đã dừng theo yêu cầu.")
        elif run.get("error"):
            self.error = str(run["error"])
            self.lines.append("Lỗi: " + self.error)
            self._add("error", title="Không hoàn thành được yêu cầu", message=self.error)
        self._resolve_pending("cancelled")
        self.status = self.summary()
        self._changed()

    def fail(self, message: str) -> None:
        self.running = False
        self.pending = None
        self.error = message
        self.lines.append("Lỗi: " + message)
        self._add("error", title="Không kết nối được Agent Core" if "Agent Core" in message else "Có lỗi xảy ra",
                  message=message)
        self._resolve_pending("cancelled")
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
            if data.get("tool") in ("load_skill", "remember") and data.get("ok"):
                # Da co dong rieng tu skill.loaded / memory.written: khong lap lai.
                self._changed()
                return
            if data.get("ok"):
                self.lines.append("✓ " + str(action))
                if is_edit(str(action)):
                    self._edits += 1
                self._tools.append(str(action))
                self._add("tool", action=str(action), state="ok")
            else:
                self.lines.append("✗ %s: %s" % (action, data.get("error") or "lỗi"))
                self._add("tool", action=str(action), state="error", error=str(data.get("error") or "lỗi"))
        elif kind == "skill.loaded":
            self.lines.append("✓ Dùng kỹ năng: " + str(data.get("name") or "?"))
            self._add("skill", name=str(data.get("name") or "?"))
        elif kind == "memory.written":
            text = str(data.get("text") or "")
            self.notes.append({"id": data.get("id"), "text": text})
            self.lines.append("✓ Đã ghi nhớ: " + text)
            self._add("memory", id=data.get("id"), text=text, state="saved")
        elif kind == "confirm.required":
            self.pending = data
            self.lines.append("⚠ Cần xác nhận: %s — %s" % (data.get("action"), data.get("reason") or ""))
            self._add("confirm", id=data.get("confirmationId"), action=data.get("action"), reason=data.get("reason") or "",
                      preview=data.get("paramsPreview") or "", state="pending")
        elif kind == "confirm.resolved":
            self.pending = None
            self.lines.append("• %s: %s" % ("Đồng ý" if data.get("approved") else "Từ chối", data.get("confirmationId") or ""))
            by = str(data.get("by") or "user")
            state = "approved" if data.get("approved") else (
                "timeout" if by == "timeout" else "cancelled" if by == "cancelled" else "rejected")
            self.set_confirm_state(data.get("confirmationId"), state)
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

    # ---------------------------------------------------------------- muc co cau truc

    def _add(self, kind: str, **fields) -> dict:
        item = dict(fields, kind=kind, rev=0)
        self.items.append(item)
        return item

    def touch(self, item: dict, **fields) -> None:
        """Sua mot muc da co (giao dien ve lai muc do vi rev tang)."""
        item.update(fields)
        item["rev"] = item.get("rev", 0) + 1
        self._changed()

    def set_confirm_state(self, confirmation_id, state: str) -> None:
        for item in reversed(self.items):
            if item["kind"] == "confirm" and item.get("id") == confirmation_id:
                if item.get("state") != state:
                    self.touch(item, state=state)
                return

    def _resolve_pending(self, state: str) -> None:
        for item in self.items:
            if item["kind"] == "confirm" and item.get("state") == "pending":
                item.update(state=state, rev=item.get("rev", 0) + 1)

    @property
    def thinking(self) -> bool:
        """Dang chay va chua co tra loi/the xac nhan cho: pane hien bong bong 'dang suy nghi'."""
        return self.running and not (self.items and self.items[-1]["kind"] == "confirm"
                                     and self.items[-1].get("state") == "pending")

    def reset(self) -> None:
        self.lines = []
        self.items = []
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
        self._add("info", text=text)
        self._changed()

    def _changed(self) -> None:
        if self.on_change:
            self.on_change()
