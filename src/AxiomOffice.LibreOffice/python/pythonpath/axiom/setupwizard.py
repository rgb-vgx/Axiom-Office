"""Wizard thiet lap trong LibreOffice - tuong ung SetupWizardForm.cs cua ban Windows.

Cung buoc, cung cau chu (lay tu `setup_catalog.py`, sinh tu catalog/setup.json) va cung goi Agent Core
(GET /v1/setup, POST /v1/llm/test, GET /v1/llm/models). Phan quyet dinh nam o `setup.py` (thuan Python,
co test); o day chi dung giao dien: awt cho o nhap/danh sach, `widgets.py` + `theme.py` cho hop bo goc,
nut, icon (giong pane Ask AI).

Mo tu: menu Axiom Office -> Thiet lap..., link "Thiet lap" o dau pane, hoac khi pane mo lan dau ma chua
co cau hinh.
"""
from __future__ import annotations

import json
import os
import shutil
import stat
import threading
import time
from urllib.parse import urlencode

import uno

from . import bridge, config, core, documents, log, setup, setup_catalog, theme
from .awt import Dialog, ItemListener, message_box
from .widgets import RoundBox, Surface, TextListener, Widget, button, link, set_icon, set_prop

WIDTH, HEIGHT = 640, 524
PAD = 16
CONTENT_Y = 114
CONTENT_H = 340
ROW_H = 44
BUTTON_W, BUTTON_H = 150, 30
CURRENT: "Wizard | None" = None   # wizard dang mo; mo lai thi dung lai de khong mat buoc dang lam

def open_setup(target) -> "Wizard | None":
    """Mo wizard giua vung tai lieu (giong dialog Cai dat). `target` can co .ctx va .frame."""
    ctx = getattr(target, "ctx", None)
    if ctx is None:
        return None
    frame = getattr(target, "frame", None) or documents.desktop(ctx).getCurrentFrame()
    global CURRENT
    if CURRENT is not None and CURRENT.is_open():
        CURRENT.window.to_front()   # co the dang nam sau cua so LibreOffice
        return CURRENT           # dang mo: giu nguyen trang thai (mo lai khong lam mat buoc dang lam)
    wizard = Wizard(ctx, frame)
    wizard.show()
    wizard.start()
    CURRENT = wizard
    return wizard

class Box:
    """Vung noi dung cua mot buoc: nhieu Widget (control roi + hop bo goc) co toa do rieng.

    `widgets.Widget` khong long duoc vao nhau (Widget.parts chi chua control that), nen gom o day roi
    dat/an ca nhom cung luc.
    """

    def __init__(self, surface: Surface):
        self._surface = surface
        self._items: list = []      # [Widget, dx, dy]
        self._visible = True

    def widget(self, dx: int = 0, dy: int = 0) -> Widget:
        widget = Widget(self._surface)
        self._items.append([widget, dx, dy])
        return widget

    def add(self, widget: Widget, dx: int, dy: int) -> Widget:
        self._items.append([widget, dx, dy])
        return widget

    def place(self, x: int, y: int) -> None:
        for widget, dx, dy in self._items:
            widget.place(x + dx, y + dy)

    def set_visible(self, visible: bool) -> None:
        """Ap moi lan (khong bo qua khi gia tri khong doi): control tao luc cua so chua hien co the khong giu
        trang thai an, nen phai dat lai moi khi doi buoc."""
        self._visible = visible
        for widget, _dx, _dy in self._items:
            widget.visible = visible
            for part in widget.parts:
                try:
                    part[0].setVisible(visible)
                except Exception:  # noqa: BLE001 - control da bi huy
                    pass

class Wizard:
    """Nam buoc: chao mung -> kiem tra may -> ket noi AI -> tinh nang -> hoan tat."""

    def __init__(self, ctx, frame):
        self.ctx = ctx
        self.frame = frame
        self.gate = bridge.gate()
        self.kind, _port = _pane_target(frame)
        self.state = setup.SetupState(self.kind)
        self.busy = False
        self._token = 0
        self._writing = False     # dang ghi o nhap tu code -> TextListener bo qua
        self.closed = False       # da goi close() (bam X co the bao nhieu lan)
        # Dialog that (co X window rieng): cua so con VCL trong vung tai lieu khong bam duoc (xem awt.Dialog).
        self.window = Dialog(ctx, WIDTH, HEIGHT, "Thiết lập Axiom Office")
        self.window.on_close(self.close)      # bam X tren thanh tieu de
        self.surface = Surface(ctx, self.window.container)
        self.boxes: dict = {}
        self.check_rows: list = []
        self.feature_boxes: dict = {}
        self.provider_chips: dict = {}
        self._build()

    def start(self) -> None:
        """Goi sau khi cua so da hien (show()): dat lai vi tri/an hien roi doc trang thai Core."""
        self._show_step()
        self._show_step()
        self.refresh_async()

    # ---------------------------------------------------------------- dung giao dien

    def _build(self) -> None:
        s = self.surface
        self.title = self._chrome(s, PAD, 14, WIDTH - 2 * PAD, 22, TextColor=theme.TEXT_PRIMARY,
                                  font=(theme.FONT_SEMIBOLD, theme.SIZE_EMPTY_TITLE))
        self.step_line = self._chrome(s, PAD, 40, WIDTH - 2 * PAD, 16, TextColor=theme.ACCENT_FG,
                                      font=(theme.FONT_SEMIBOLD, theme.SIZE_CAPTION))
        self.bar = Widget(s)
        self.bar_control = self.bar.add("FixedText", 0, 0, 0, 3, Label="", BackgroundColor=theme.ACCENT, Border=0)
        self._chrome(s, PAD, 60, WIDTH - 2 * PAD, 3, BackgroundColor=theme.DIVIDER, NoLabel=True)
        self.subtitle = self._chrome(s, PAD, 70, WIDTH - 2 * PAD, 30, MultiLine=True, TextColor=theme.TEXT_SECONDARY,
                                     NoLabel=True, font=(theme.FONT, theme.SIZE_SMALL))
        self._chrome(s, PAD, 106, WIDTH - 2 * PAD, 1, BackgroundColor=theme.DIVIDER, NoLabel=True)
        self._chrome(s, PAD, 462, WIDTH - 2 * PAD, 1, BackgroundColor=theme.DIVIDER, NoLabel=True)

        status_w = WIDTH - 2 * PAD - 2 * BUTTON_W - 24
        self.status = self._chrome(s, PAD, 470, status_w, 30, MultiLine=True, TextColor=theme.TEXT_MUTED,
                                   NoLabel=True, font=(theme.FONT, theme.SIZE_CAPTION))
        self.back_button = button(s, "← Quay lại", 110, BUTTON_H, self._back, primary=False)
        self.next_button = button(s, "Tiếp tục →", BUTTON_W, BUTTON_H, self._next)
        self.back_button.place(PAD, 470)
        self.next_button.place(WIDTH - PAD - BUTTON_W, 470)
        self.later_link = link(s, "Để sau", self._later, color=theme.TEXT_MUTED)
        self.later_link.place(WIDTH - PAD - BUTTON_W - 24 - self.later_link.width, 478)

        self._build_welcome()
        self._build_checks()
        self._build_connect()
        self._build_features()
        self._build_done()

    def _chrome(self, surface: Surface, x: int, y: int, width: int, height: int, **props):
        """Control khung cua wizard. Model awt bo qua PositionX/Y/Width/Height nen phai dat bang setPosSize."""
        props.setdefault("Label", "")
        props.setdefault("BackgroundColor", theme.PANE_BG)
        props.setdefault("Border", 0)
        control = surface.create("FixedText", **props)
        control.setPosSize(x, y, width, height, 15)
        return control

    def _box(self, step_id: str) -> Box:
        box = Box(self.surface)
        self.boxes[step_id] = box
        return box

    def _build_welcome(self) -> None:
        box = self._box("welcome")
        lines = (
            "Axiom Office đọc và sửa trực tiếp tài liệu bạn đang mở khi bạn yêu cầu.",
            "Để làm được vậy, nó cần nói chuyện với một máy chủ AI (LLM).",
            "Ba bước: kiểm tra máy → nối vào máy chủ AI → chọn tính năng.",
        )
        y = 0
        for text in lines:
            box.widget().add("FixedText", 0, y, WIDTH - 2 * PAD, 22, Label="•   " + text, Border=0, NoLabel=True,
                             BackgroundColor=theme.PANE_BG, TextColor=theme.TEXT_PRIMARY,
                             font=(theme.FONT, theme.SIZE_BODY))
            y += 28
        box.widget().add("FixedText", 0, y + 8, WIDTH - 2 * PAD, 60,
                         Label="Không biết điền gì? Chọn \"Máy chủ của công ty\" nếu cơ quan bạn có máy chủ AI "
                               "riêng (hỏi quản trị viên địa chỉ), hoặc dùng tài khoản OpenAI / Anthropic của bạn.",
                         MultiLine=True, Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG,
                         TextColor=theme.TEXT_MUTED, font=(theme.FONT, theme.SIZE_CAPTION))

    def _build_checks(self) -> None:
        box = self._box("checks")
        for index, meta in enumerate(setup_catalog.CHECKS):
            y = index * ROW_H
            holder = box.widget(0, y)
            icon = holder.add("ImageControl", 2, 3, 12, 12, Border=0, ScaleImage=False, BackgroundColor=theme.PANE_BG,
                              Tabstop=False)
            set_icon(icon, "dot", 12, theme.PANE_BG)
            label = holder.add("FixedText", 24, 0, WIDTH - 2 * PAD - 150, 18, Label=meta["label"], Border=0,
                               NoLabel=True, BackgroundColor=theme.PANE_BG, TextColor=theme.TEXT_PRIMARY,
                               font=(theme.FONT_SEMIBOLD, theme.SIZE_SMALL))
            detail = holder.add("FixedText", 24, 18, WIDTH - 2 * PAD - 150, 22, Label=meta["help"], Border=0,
                                NoLabel=True, BackgroundColor=theme.PANE_BG, TextColor=theme.TEXT_MUTED,
                                font=(theme.FONT, theme.SIZE_CAPTION))
            fix = link(self.surface, meta["fixLabel"] or "Sửa", lambda check_id=meta["id"]: self._fix(check_id),
                       color=theme.ACCENT_FG)
            fix.set_visible(False)
            box.add(fix, WIDTH - 2 * PAD - 120, y + 2)
            self.check_rows.append({"id": meta["id"], "meta": meta, "icon": icon, "detail": detail, "fix": fix})

    def _build_connect(self) -> None:
        box = self._box("connect")
        x = 0
        for preset in setup_catalog.PROVIDERS[:3]:
            chip = button(self.surface, preset["label"], 192, 34, lambda pid=preset["id"]: self._choose(pid), primary=False)
            box.add(chip, x, 0)
            self.provider_chips[preset["id"]] = chip
            x += 198
        self.provider_note = box.widget(0, 40).add("FixedText", 0, 0, WIDTH - 2 * PAD, 32, Label="", MultiLine=True,
                                                   Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG,
                                                   TextColor=theme.TEXT_MUTED, font=(theme.FONT, theme.SIZE_CAPTION))
        self.endpoint_edit = self._field(box, 74, "Địa chỉ máy chủ AI")
        self.key_edit = self._field(box, 110, "Khoá API", password=True)
        self.key_link = link(self.surface, "Lấy khoá ở đâu?", self._open_key_url, color=theme.ACCENT_FG)
        box.add(self.key_link, 150, 138)
        self.model_edit = self._field(box, 158, "Model")
        for edit in (self.endpoint_edit, self.key_edit, self.model_edit):
            edit.addTextListener(TextListener(self._on_field_changed))   # bat/tat nut chinh ngay khi go
        self.models_button = button(self.surface, "Tải danh sách model", 170, 26, self._load_models, primary=False)
        box.add(self.models_button, 150, 184)
        self.models_hint = box.widget(332, 184).add("FixedText", 0, 0, WIDTH - 2 * PAD - 332, 28, Label="", MultiLine=True,
                                                    Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG,
                                                    TextColor=theme.TEXT_MUTED, font=(theme.FONT, theme.SIZE_CAPTION))
        list_holder = box.widget(150, 214)
        self.model_list = list_holder.add("ListBox", 0, 0, WIDTH - 2 * PAD - 150, 80, StringItemList=(), LineCount=4)
        self.model_list.setVisible(False)
        self.model_list.addItemListener(ItemListener(self._on_model_picked))   # chon model khac -> thu lai
        test_holder = box.widget(0, 300)
        self.test_icon = test_holder.add("ImageControl", 2, 3, 12, 12, Border=0, ScaleImage=False,
                                         BackgroundColor=theme.PANE_BG, Tabstop=False)
        set_icon(self.test_icon, "dot", 12, theme.PANE_BG)
        self.test_line = test_holder.add("FixedText", 22, 0, WIDTH - 2 * PAD - 22, 42, Label="", MultiLine=True,
                                         Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG,
                                         TextColor=theme.TEXT_SECONDARY, font=(theme.FONT, theme.SIZE_SMALL))

    def _build_features(self) -> None:
        box = self._box("features")
        y = 0
        for feature in setup_catalog.FEATURES:
            holder = box.widget(0, y)
            check = holder.add("CheckBox", 0, 0, WIDTH - 2 * PAD, 20, Label=feature["label"], Border=0,
                               font=(theme.FONT_SEMIBOLD, theme.SIZE_SMALL))
            holder.add("FixedText", 24, 20, WIDTH - 2 * PAD - 24, 34, Label=feature["description"], MultiLine=True,
                       Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG, TextColor=theme.TEXT_MUTED,
                       font=(theme.FONT, theme.SIZE_CAPTION))
            self.feature_boxes[feature["key"]] = check
            y += 78
        advanced = link(self.surface, "Tuỳ chọn nâng cao (cổng Agent Core, hạn giờ, đường dẫn Core)…",
                        self._open_advanced, color=theme.ACCENT_FG)
        box.add(advanced, 0, y + 6)

    def _build_done(self) -> None:
        box = self._box("done")
        head = box.widget()
        self.done_icon = head.add("ImageControl", 2, 3, 14, 14, Border=0, ScaleImage=False,
                                  BackgroundColor=theme.PANE_BG, Tabstop=False)
        set_icon(self.done_icon, "ok", 14, theme.PANE_BG)
        self.done_line = head.add("FixedText", 26, 0, WIDTH - 2 * PAD - 26, 20, Label="", Border=0, NoLabel=True,
                                  BackgroundColor=theme.PANE_BG, TextColor=theme.TEXT_PRIMARY,
                                  font=(theme.FONT_SEMIBOLD, theme.SIZE_SMALL))
        self.summary = box.widget(0, 30).add("FixedText", 0, 0, WIDTH - 2 * PAD, 44, Label="", MultiLine=True,
                                             Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG,
                                             TextColor=theme.TEXT_SECONDARY, font=(theme.FONT, theme.SIZE_SMALL))
        self.try_button = button(self.surface, "Thử ngay", 150, 30, self._try_now, primary=False)
        box.add(self.try_button, 0, 84)
        self.try_line = box.widget(0, 124).add("FixedText", 0, 0, WIDTH - 2 * PAD, 44, Label="", MultiLine=True,
                                               Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG,
                                               TextColor=theme.TEXT_MUTED, font=(theme.FONT, theme.SIZE_CAPTION))
        self.mcp_line = box.widget(0, 176).add("FixedText", 0, 0, WIDTH - 2 * PAD, 60, Label="", MultiLine=True,
                                               Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG,
                                               TextColor=theme.TEXT_MUTED, font=(theme.FONT, theme.SIZE_CAPTION))
        self.done_hint = box.widget(0, 244).add("FixedText", 0, 0, WIDTH - 2 * PAD, 44, Label="", MultiLine=True,
                                                Border=0, NoLabel=True, BackgroundColor=theme.PANE_BG,
                                                TextColor=theme.TEXT_SECONDARY, font=(theme.FONT, theme.SIZE_SMALL))

    def _field(self, box: Box, y: int, caption: str, password: bool = False):
        """O nhap 1 dong co hop bo goc + nhan ben trai; tra ve control Edit."""
        box.widget(0, y).add("FixedText", 0, 4, 144, 20, Label=caption, Border=0, NoLabel=True,
                             BackgroundColor=theme.PANE_BG, TextColor=theme.TEXT_SECONDARY,
                             font=(theme.FONT, theme.SIZE_SMALL))
        inner = box.widget(150, y)
        width, radius = WIDTH - 2 * PAD - 150, theme.RADIUS_CONTROL
        frame_box = RoundBox(inner, width, 26, radius, theme.PANE_BG, theme.INPUT_BORDER)
        frame_box.add_corners()
        edit = inner.add("Edit", radius // 2, 3, width - radius, 20, Border=0, BackgroundColor=theme.PANE_BG,
                         TextColor=theme.TEXT_PRIMARY, font=(theme.FONT, theme.SIZE_BODY))
        if password:
            set_prop(edit, "EchoChar", 0x2022)
        frame_box.add_background()
        return edit

    # ---------------------------------------------------------------- hien / an

    def show(self) -> None:
        rect = self.frame.ComponentWindow.getPosSize()
        x = max(10, int(rect.X) + max(0, (int(rect.Width) - WIDTH) // 2))
        y = max(10, int(rect.Y) + max(0, (int(rect.Height) - HEIGHT) // 2))
        self.window.place(x, y, WIDTH, HEIGHT)
        self.window.set_title("Thiết lập Axiom Office")

    def is_open(self) -> bool:
        """Wizard con song khong (nguoi dung co the da bam X cua so -> dispose)."""
        if getattr(self, "closed", False):
            return False
        return self.window.is_alive()

    def close(self) -> None:
        global CURRENT

        if self.closed:
            return               # close() -> dispose -> TopWindowListener goi lai
        self.closed = True
        if CURRENT is self:
            CURRENT = None
        try:
            self.window.close()
        except Exception as exc:  # noqa: BLE001
            log.error("setup wizard close failed: %s" % exc)

    def _show_step(self) -> None:
        self._token += 1
        state = self.state
        set_prop(self.title, "Label", state.step_title)
        set_prop(self.subtitle, "Label", state.step_subtitle)
        set_prop(self.step_line, "Label", state.progress)
        filled = int((WIDTH - 2 * PAD) * (state.step + 1) / len(setup.STEP_IDS))
        self.bar.place(PAD, 60)
        self.bar.resize_part(self.bar_control, w=filled)
        for step_id, box in self.boxes.items():
            box.place(PAD, CONTENT_Y)
            box.set_visible(step_id == state.step_id)
        self.back_button.set_visible(not state.is_first)
        self._set_next()
        self._refresh_step()

    def _set_next(self) -> None:
        label = self.state.next_label
        if label != getattr(self.next_button, "text", None):
            self.next_button.text = label
            set_prop(self.next_button.label, "Label", label)
        enabled = True
        if self.state.step_id == "connect":
            enabled = bool(self.state.endpoint.strip() and self.state.model.strip()) and not self.busy
        self.next_button.set_enabled(enabled)

    def _set_chip(self, chip, chosen: bool) -> None:
        fill = theme.ACTION_BG if chosen else theme.PANE_BG
        chip.box.recolor(fill, None if chosen else theme.CHIP_BORDER)
        set_prop(chip.label, "BackgroundColor", fill)
        set_prop(chip.label, "TextColor", theme.ON_ACTION if chosen else theme.CHIP_FG)

    def _refresh_step(self) -> None:
        state = self.state
        if state.step_id == "connect":
            self._paint_connect()
        elif state.step_id == "checks":
            self._paint_checks()
        elif state.step_id == "features":
            for key, check in self.feature_boxes.items():
                set_prop(check, "State", 1 if state.features.get(key) else 0)
        elif state.step_id == "done":
            ready = bool(state.endpoint.strip() and state.model.strip() and not state.core_error)
            set_icon(self.done_icon, "ok" if ready else "error", 14, theme.PANE_BG)
            set_prop(self.done_line, "Label", "Thiết lập xong" if ready else "Còn thiếu cấu hình AI")
            note = ("Agent Core: " + state.core_error) if state.core_error else \
                "Agent Core: đang chạy (cổng %s)" % state.core_info.get("port", "?")
            set_prop(self.summary, "Label", state.summary() + "\n" + note)
            set_prop(self.mcp_line, "Label", self._mcp_line())
            set_prop(self.done_hint, "Label", state.replace_document_hint()
                     + "\nLần sau muốn đổi: menu Axiom Office → Thiết lập…")
        self._refresh_status()

    def _sync_fields(self) -> None:
        """Dien o nhap theo trang thai (mo lai wizard tren may da thiet lap thi thay gia tri cu).

        Chi ghi khi khac gia tri dang co: nguoi dung dang go thi khong bi ghi de.
        """
        for control, value in ((self.endpoint_edit, self.state.endpoint), (self.model_edit, self.state.model)):
            try:
                if str(control.getModel().getPropertyValue("Text") or "") != value:
                    self._set_text(control, value)
            except Exception:  # noqa: BLE001
                pass

    def _paint_connect(self) -> None:
        state = self.state
        self._sync_fields()
        for provider_id, chip in self.provider_chips.items():
            self._set_chip(chip, provider_id == state.provider_id)
        note = setup.provider(state.provider_id)["description"]
        if not state.needs_key:
            note += "  Khoá truy cập: chỉ điền nếu máy chủ yêu cầu."
        set_prop(self.provider_note, "Label", note)
        self.key_link.set_visible(bool(state.key_url))
        self.models_button.set_enabled(not self.busy)
        has_list = bool(state.models)
        self.model_list.setVisible(has_list)
        self.models_hint.setVisible(not has_list)      # control awt: setVisible (khong phai Widget.set_visible)
        set_prop(self.models_hint, "Label", state.models_status)
        if has_list:
            self._fill_model_list()

        set_icon(self.test_icon, state.test_kind() or "dot", 12, theme.PANE_BG)
        set_prop(self.test_line, "Label", state.test_line())
        set_prop(self.status, "Label", "Đang xử lý…" if self.busy else state.models_status)

    def _refresh_status(self) -> None:
        if self.busy:
            set_prop(self.status, "Label", "Đang xử lý…")
        elif self.state.step_id != "connect":
            set_prop(self.status, "Label", self.state.core_error or self._status_message())

    def _status_message(self) -> str:
        pending = [row["meta"]["label"] for row in self.check_rows if row.get("ok") is False]
        if not pending:
            return "Mọi thứ đều ổn."
        return "Cần chú ý: " + "; ".join(pending)

    def _mcp_line(self) -> str:
        path = os.path.join(config.data_dir(), "mcp", "axiom-office-mcp")
        if os.path.exists(path):
            return "Dùng từ Claude Code/Desktop (tuỳ chọn):\n%s all" % path
        return "MCP cho Claude Code/Desktop: chưa cài (chạy lại trình cài đặt nếu cần)."

    # ---------------------------------------------------------------- chay nen

    def _async(self, work, done) -> None:
        """Chay work() o thread nen, tra ket qua ve main thread (awt chi chay tren main thread)."""
        token = self._token

        def runner():
            try:
                result = work()
            except Exception as exc:  # noqa: BLE001
                log.error("setup wizard task failed: %s" % exc)
                result = {"__error__": str(exc)}
            self.gate.run_quiet(lambda: self._async_done(token, done, result), 20.0, None)

        threading.Thread(target=runner, name="axiom-setup", daemon=True).start()

    def _async_done(self, token, done, result) -> None:
        if token != self._token:      # nguoi dung da doi buoc -> bo ket qua cu
            return
        self.busy = False
        done(result)
        self._show_step()      # ve lai buoc hien tai (nhan nut, dong kiem tra, o nhap)

    # ---------------------------------------------------------------- kiem tra may

    def refresh_async(self) -> None:
        self.busy = True
        for row in self.check_rows:
            set_icon(row["icon"], "running", 12, theme.PANE_BG)
            set_prop(row["detail"], "Label", "Đang kiểm tra…")
        self.next_button.set_enabled(False)
        self._async(self._collect, self._apply_collect)

    def _collect(self) -> dict:
        checks: dict = {}
        payload = None
        error = ""
        try:
            base, _port = core.ensure()
        except core.CoreError as exc:
            error = str(exc)
            checks["core"] = (False, "Chưa chạy — bấm \"Khởi động Core\".")
        else:
            checks["core"] = (True, "Đang chạy ở %s." % base)
            try:
                answer = core.call(base, "GET", "/v1/setup")      # nem CoreError khi loi
                payload = answer if isinstance(answer, dict) else {}
            except core.CoreError as exc:
                error = str(exc)

        port = next((p for k, p, _ in bridge._STATE.get("servers", []) if k == self.kind), 0)  # noqa: SLF001
        ok = bool(port) and bool(core.health("http://127.0.0.1:%d" % port))
        checks["bridge"] = (ok, ("Cổng %d trả lời được." % port) if ok else "Cổng %d chưa trả lời." % port)

        current = (payload or {}).get("current") or {}
        endpoint = current.get("endpoint") or config.value("LlmEndpoint", "")
        model = current.get("model") or config.value("LlmModel", "")
        checks["config"] = (bool(endpoint and model),
                            "Đã có: %s" % model if endpoint and model else "Chưa có địa chỉ máy chủ AI và model.")
        token = config.token()
        checks["token"] = (bool(token), "Đã có khoá bảo vệ." if token else "Chưa có khoá bảo vệ.")
        checks["configFile"] = self._config_file_check()
        checks["mcp"] = self._mcp_check()
        checks["app"] = (True, "Axiom Office đã nạp trong LibreOffice.")
        return {"checks": checks, "payload": payload, "error": error}

    def _config_file_check(self) -> tuple:
        path = config.config_file()
        if not os.path.exists(path):
            return True, "Chưa có tệp cấu hình (tạo khi lưu lần đầu)."
        try:
            mode = stat.S_IMODE(os.stat(path).st_mode)
        except OSError as exc:
            return False, "Không đọc được tệp cấu hình: %s" % exc
        if mode != 0o600:
            return False, "Tệp cấu hình đang cho người khác đọc (quyền %o)." % mode
        return True, "Tệp cấu hình riêng tư (0600)."

    def _mcp_check(self) -> tuple:
        path = os.path.join(config.data_dir(), "mcp", "axiom-office-mcp")
        if os.path.exists(path):
            return True, "Có sẵn cho Claude Code/Desktop."
        return True, "Chưa cài MCP (không bắt buộc)."

    def _apply_collect(self, result: dict) -> None:
        error = result.get("__error__") or result.get("error") or ""
        if error:
            self.state.apply_core_error(error)
        else:
            self.state.apply_core_error("")
        if result.get("payload"):
            self.state.apply_core_payload(result["payload"])
        for row in self.check_rows:
            ok, detail = (result.get("checks") or {}).get(row["id"], (None, ""))
            row["ok"] = ok
            row["fixable"] = bool(row["meta"]["fixable"]) and ok is not True
            set_icon(row["icon"], "dot" if ok is None else ("ok" if ok else "error"), 12, theme.PANE_BG)
            set_prop(row["detail"], "Label", detail or row["meta"]["help"])
        self._paint_checks()
        if self.state.step_id == "welcome" and self.state.configured:
            self.state.go("checks")          # da thiet lap roi thi vao thang man kiem tra / sua loi

    # ---------------------------------------------------------------- sua loi

    def _fill_model_list(self) -> None:
        """Do danh sach model vao ListBox. Ghi log khi that bai (set_prop im lang bo qua loi)."""
        try:
            # pyuno can kieu tuong minh cho sequence<string> va uno.Any chi dung duoc qua uno.invoke.
            uno.invoke(self.model_list.getModel(), "setPropertyValue",
                       ("StringItemList", uno.Any("[]string", tuple(self.state.models))))
            log.info("setup: model list -> %d muc" % len(self.state.models))
        except Exception as exc:  # noqa: BLE001
            log.error("setup: khong do duoc danh sach model (%s) - dung o nhap tay" % exc)

    def _paint_checks(self) -> None:
        """An/hien link "Sua" theo tung dong. Phai goi lai sau khi Box.set_visible(True) hien lai ca nhom."""
        for row in self.check_rows:
            row["fix"].set_visible(bool(row.get("fixable")))

    def _fix(self, check_id: str) -> None:
        if self.busy:
            return
        if check_id == "config":
            self.state.go("connect")
            self._show_step()
            return
        self.busy = True
        self.next_button.set_enabled(False)
        set_prop(self.status, "Label", "Đang sửa…")
        self._async(lambda: self._apply_fix(check_id), lambda result: self._after_fix(result))

    def _apply_fix(self, check_id: str) -> dict:
        if check_id in ("core", "bridge"):
            base, _port = core.ensure()
            return {"message": "Agent Core đang chạy ở %s." % base}
        if check_id == "token":
            token = config.token()
            if not token:
                return {"message": "Chưa tạo được khoá bảo vệ — chạy lại trình cài đặt."}
            self._restart_core()
            return {"message": "Đã tạo khoá bảo vệ và khởi động lại Agent Core."}
        if check_id == "configFile":
            return self._repair_config_file()
        return {"message": "Việc này cần làm thủ công — xem hướng dẫn ở dòng tương ứng."}

    def _after_fix(self, result: dict) -> None:
        message = result.get("message") or result.get("__error__") or ""
        if message:
            set_prop(self.status, "Label", message)
        self.refresh_async()

    def _restart_core(self) -> None:
        try:
            base, _port = core.ensure()
            core.call(base, "POST", "/v1/admin/shutdown", timeout=5)
        except Exception as exc:  # noqa: BLE001 - Core chua chay thi chi can khoi dong lai
            log.info("setup: bo qua shutdown Core: %s" % exc)
        try:
            core.ensure()
        except core.CoreError as exc:
            log.error("setup: khong khoi dong lai duoc Core: %s" % exc)

    def _repair_config_file(self) -> dict:
        path = config.config_file()
        try:
            os.makedirs(os.path.dirname(path), exist_ok=True)
            if os.path.exists(path):
                backup = path + ".bak"
                if not os.path.exists(backup):
                    shutil.copy2(path, backup)
                try:
                    with open(path, encoding="utf-8") as handle:
                        json.load(handle)
                except (OSError, ValueError):
                    with open(path, "w", encoding="utf-8") as handle:
                        json.dump({"Token": config.token()}, handle, indent=2)
            os.chmod(path, 0o600)
            os.chmod(os.path.dirname(path), 0o700)
        except OSError as exc:
            return {"message": "Không sửa được tệp cấu hình: %s" % exc}
        return {"message": "Đã sửa quyền tệp cấu hình (bản cũ giữ ở config.json.bak)."}

    # ---------------------------------------------------------------- hanh dong buoc 3

    def _read_fields(self) -> bool:
        """Doc ba o nhap vao trang thai; tra ve True neu gia tri doi (ket qua thu cu het hieu luc).

        Di qua `SetupState.update_fields` chu KHONG gan thang: do la cho duy nhat biet "gia tri vua doi thi
        ket qua thu cu khong con dung". Danh sach model (ListBox) la nguon chinh khi no co du lieu.
        """
        state = self.state
        endpoint = str(self.endpoint_edit.getModel().getPropertyValue("Text") or "").strip()
        model = str(self.model_edit.getModel().getPropertyValue("Text") or "").strip()
        api_key = str(self.key_edit.getModel().getPropertyValue("Text") or "").strip()
        selected = self._selected_model()
        if selected:
            model = selected
            self._set_text(self.model_edit, selected)
        return state.update_fields(endpoint, model, api_key)

    def _set_text(self, edit, value: str) -> None:
        """Ghi o nhap tu code ma khong kich _on_field_changed (ghi tung o se doc lech o con lai)."""
        self._writing = True
        try:
            edit.getModel().setPropertyValue("Text", value)
        finally:
            self._writing = False

    def _on_field_changed(self) -> None:
        """Nguoi dung go vao o dia chi/khoa/model: cap nhat trang thai nut chinh, bo ket qua thu cu."""
        if self._writing or self.state.step_id != "connect":
            return
        try:
            texts = [str(edit.getModel().getPropertyValue("Text") or "")
                     for edit in (self.endpoint_edit, self.model_edit, self.key_edit)]
        except Exception:  # noqa: BLE001 - control da bi huy
            return
        if texts[1].strip() != self._selected_model():
            self._drop_list_selection()   # tu go ten model -> chu trong o thang the danh sach
        self._note_change(self.state.update_fields(*texts))

    def _on_model_picked(self) -> None:
        """Chon model trong danh sach: nhu go tay - ket qua thu cu (do model cu) het hieu luc."""
        if self._writing or self.state.step_id != "connect":
            return
        state = self.state
        self._note_change(state.update_fields(state.endpoint, self._selected_model() or state.model,
                                               state.api_key))

    def _drop_list_selection(self) -> None:
        """Bo muc dang chon trong danh sach model (neu co)."""
        try:
            index = int(self.model_list.getSelectedItemPos())
            if index >= 0:
                self.model_list.selectItemPos(index, False)
        except Exception as exc:  # noqa: BLE001 - chua co peer / ban khac khong ho tro
            log.info("setup: khong bo chon duoc muc model (%s)" % exc)

    def _note_change(self, changed: bool) -> None:
        if changed:
            set_icon(self.test_icon, "dot", 12, theme.PANE_BG)
            set_prop(self.test_line, "Label", "")
        self._set_next()

    def _selected_model(self) -> str:
        """Model dang chon trong ListBox ("" khi danh sach trong hoac chua chon muc nao).

        Phai doc tu CONTROL (`XListBox.getSelectedItemPos`), KHONG phai tu model: thuoc tinh
        `SelectedItemPos` cua model tra ve None du nguoi dung da chon (LibreOffice 26.8), nen doc model thi
        khong bao gio thay duoc lua chon - chon model trong danh sach se vo hieu.
        """
        if not self.state.models:
            return ""
        try:
            index = int(self.model_list.getSelectedItemPos())
        except Exception:  # noqa: BLE001 - control chua co peer hoac da bi huy
            return ""
        return self.state.models[index] if 0 <= index < len(self.state.models) else ""

    def _choose(self, provider_id: str) -> None:
        self._read_fields()
        self.state.choose_provider(provider_id)
        self._set_text(self.endpoint_edit, self.state.endpoint)
        self._set_text(self.model_edit, self.state.model)
        self._refresh_step()

    def _open_key_url(self) -> None:
        url = self.state.key_url
        if not url:
            return
        try:
            documents.desktop(self.ctx).loadComponentFromURL(url, "_blank", 0, ())
        except Exception as exc:  # noqa: BLE001
            log.error("setup: khong mo duoc %s: %s" % (url, exc))
            message_box(self.ctx, self.window.window(), "Lấy khoá API",
                        "Mở địa chỉ này trong trình duyệt để lấy khoá:\n%s" % url, 1)

    def _load_models(self) -> None:
        if self.busy:
            return
        self._read_fields()      # lay gia tri nguoi dung vua go trong o (chua can roi o)
        if not self.state.endpoint:
            set_prop(self.status, "Label", "Điền địa chỉ máy chủ AI trước.")
            return
        self.busy = True
        self.state.loading_models = True
        self.next_button.set_enabled(False)
        set_prop(self.models_hint, "Label", "Đang lấy danh sách model…")
        set_prop(self.status, "Label", "Đang lấy danh sách model…")
        query = urlencode(self.state.models_query())
        self._async(lambda: core.call(core.ensure()[0], "GET", "/v1/llm/models?" + query), self._after_models)

    def _after_models(self, payload) -> None:
        if isinstance(payload, dict) and payload.get("__error__"):
            self.state.apply_models_error(payload["__error__"])
        else:
            models, error = setup.payload_to_models(payload)
            if error:
                self.state.apply_models_error(error)
            else:
                self.state.apply_models(models)
                self._set_text(self.model_edit, self.state.model)

    def _test_connection(self) -> None:
        if self.busy:
            return
        self._read_fields()
        error = self.state.validation_error()
        if error:
            self.state.apply_test({"ok": False, "message": error, "hint": ""})
            self._refresh_step()
            return
        self.busy = True
        self.state.testing = True
        set_icon(self.test_icon, "running", 12, theme.PANE_BG)
        set_prop(self.test_line, "Label", "Đang kiểm tra kết nối…")
        self.next_button.set_enabled(False)
        body = self.state.test_payload()
        self._async(lambda: core.call(core.ensure()[0], "POST", "/v1/llm/test", body), self._after_test)

    def _after_test(self, payload) -> None:
        if isinstance(payload, dict) and payload.get("__error__"):
            self.state.apply_test({"ok": False, "kind": "internal", "message": payload["__error__"], "hint": ""})
        else:
            self.state.apply_test(setup.payload_to_test_result(payload))

    # ---------------------------------------------------------------- hanh dong buoc 4/5

    def _read_feature_boxes(self) -> None:
        for key, check in self.feature_boxes.items():
            try:
                self.state.features[key] = bool(check.getModel().getPropertyValue("State"))
            except Exception:  # noqa: BLE001
                pass

    def _open_advanced(self) -> None:
        from . import dialogs

        dialogs.open_settings(_SettingsTarget(self.ctx, self.frame), floating=True)   # cua so roi: noi len tren wizard

    def _try_now(self) -> None:
        if self.busy:
            return
        self.busy = True
        self.next_button.set_enabled(False)
        set_prop(self.try_line, "Label", "Đang gửi một yêu cầu thử tới AI…")
        self._async(self._run_sample, self._after_try)

    def _run_sample(self) -> dict:
        base, _port = core.ensure()
        port = next((p for k, p, _ in bridge._STATE.get("servers", []) if k == self.kind), 0)  # noqa: SLF001
        if not port:
            return {"message": "Chưa thấy cầu nối trong ứng dụng — mở lại tài liệu rồi thử lại."}
        run = core.start_run(base, "Viết một câu chào ngắn vào tài liệu", port, self.kind)
        run_id = (run or {}).get("runId") if isinstance(run, dict) else None
        if not run_id:
            return {"message": "Agent Core không nhận được yêu cầu thử."}
        deadline = time.time() + 120
        while time.time() < deadline:
            time.sleep(1.0)
            status = core.get_run(base, run_id) or {}
            if status.get("status") != "running":
                ok = status.get("status") == "completed"
                return {"ok": ok, "reply": status.get("reply") or "",
                        "message": "" if ok else (status.get("error") or "Lượt thử không hoàn tất.")}
        return {"message": "Quá 120 giây chưa xong — thử lại sau."}

    def _after_try(self, result: dict) -> None:
        if result.get("ok"):
            set_prop(self.try_line, "Label", "Thành công! AI đã trả lời: %s" % (result.get("reply") or "xong"))
        else:
            set_prop(self.try_line, "Label", "Chưa chạy được: %s"
                     % (result.get("message") or result.get("__error__") or "lỗi không rõ"))

    # ---------------------------------------------------------------- nut duoi

    def _next(self) -> None:
        state = self.state
        if state.step_id == "connect":
            self._read_fields()
            if not (state.test and state.test.get("ok")):
                self._test_connection()     # bam lan dau = kiem tra; chi di tiep khi ket noi tot
                return
            self._save_config()
        elif state.step_id == "features":
            self._read_feature_boxes()
            self._save_config()
        elif state.step_id == "done":
            self._read_feature_boxes()
            self._save_config()
            state.done = True
            self.close()
            return
        state.next()
        self._show_step()

    def _back(self) -> None:
        if self.state.step_id == "connect":
            self._read_fields()
        if self.state.back():
            self._show_step()

    def _later(self) -> None:
        self.close()

    def _save_config(self) -> None:
        state = self.state
        self._read_fields()
        try:
            config.set_value("LlmProvider", state.codec)
            config.set_value("LlmEndpoint", state.endpoint)
            config.set_value("LlmModel", state.model)
            if state.api_key:
                config.set_value("LlmApiKey", config.protect_secret(state.api_key, "LlmApiKey"))
                state.api_key = ""
                state.has_stored_key = True
                self._set_text(self.key_edit, "")
            for key, enabled in state.features.items():
                config.set_value(key, 1 if enabled else 0)
        except Exception as exc:  # noqa: BLE001
            log.error("setup: khong luu duoc cau hinh: %s" % exc)
            message_box(self.ctx, self.window.window(), "Không lưu được",
                        "Không ghi được cấu hình: %s" % exc, 3)
            return
        set_prop(self.status, "Label", "Đã lưu. Cấu hình mới dùng được ngay, không cần khởi động lại.")

def drive(wizard: "Wizard", params: dict) -> dict:
    """Dieu khien wizard tu ben ngoai (test live, hoac quan tri vien cau hinh san qua bridge) roi tra trang thai.

    Cac thao tac nang (nap danh sach model, thu ket noi) chi DUOC KHOI DONG roi tra ve ngay: loi goi bridge
    chay tren main thread, ma ket qua tra ve main thread qua UnoGate - cho ngay trong cung loi goi se tu treo.
    Test goi lai `ui.setup` voi params rong de doc ket qua (xem tests/live/test_live_libreoffice.py).
    """
    state = wizard.state
    if params.get("step"):
        state.go(str(params["step"]))
        wizard._show_step()
    if params.get("provider"):
        wizard._choose(str(params["provider"]))
    if params.get("endpoint") is not None or params.get("model") is not None or params.get("apiKey") is not None:
        # Di qua update_fields (khong gan thang): doi gia tri thi ket qua thu cu phai het hieu luc, giong
        # duong nguoi dung go tay.
        state.update_fields(
            str(params["endpoint"]) if params.get("endpoint") is not None else state.endpoint,
            str(params["model"]) if params.get("model") is not None else state.model,
            str(params["apiKey"]) if params.get("apiKey") is not None else state.api_key)
        if params.get("endpoint") is not None or params.get("model") is not None:
            state.touched = True
    if params.get("step") or params.get("provider") or params.get("endpoint") is not None or params.get("model") is not None:
        wizard._sync_fields()      # _sync_fields chi lo o dia chi + model; o khoa API khong bao gio dien san
    if params.get("model") is not None:
        wizard._on_field_changed()  # nhu go tay vao o Model: bo muc dang chon trong danh sach
    if params.get("models"):
        wizard._load_models()
    if params.get("modelIndex") is not None:
        # Chon model trong ListBox (nhu nguoi dung bam): dat muc dang chon roi goi thang handler cua listener,
        # vi gan muc dang chon bang code khong ban itemStateChanged.
        uno.invoke(wizard.model_list, "selectItemPos", (int(params["modelIndex"]), True))
        wizard._on_model_picked()
    if params.get("advanced"):
        wizard._open_advanced()
    if params.get("test"):
        wizard._test_connection()
    if params.get("features"):
        for key, value in dict(params["features"]).items():
            state.features[key] = bool(value)
        wizard._save_config()
    if params.get("save"):
        wizard._save_config()
    if params.get("close"):
        wizard.close()
        return {"closed": True}
    return status(wizard)

def status(wizard: "Wizard") -> dict:
    """Trang thai hien tai cua wizard (test doc lai sau khi thao tac nang chay xong)."""
    state = wizard.state
    return {
        "step": state.step_id,
        "busy": wizard.busy,
        "loadingModels": state.loading_models,
        "models": list(state.models),
        "modelsStatus": state.models_status,
        "testKind": state.test_kind(),
        "testLine": state.test_line(),
        "nextLabel": state.next_label,
        "endpoint": state.endpoint,
        "model": state.model,
        "coreError": state.core_error,
        "features": dict(state.features),
        "selectedPos": _selected_pos(wizard),
    }

def _selected_pos(wizard: "Wizard"):
    """Muc dang chon trong ListBox model (-1: chua chon muc nao; "khong doc duoc": control da bi huy).

    Doc tu control - cung nguon voi `Wizard._selected_model` (model khong phan anh lua chon cua nguoi dung).
    """
    try:
        return wizard.model_list.getSelectedItemPos()
    except Exception:  # noqa: BLE001 - control da bi huy
        return "khong doc duoc"

class _SettingsTarget:
    """Doi tuong gia de mo dialog Cai dat nang cao tu wizard (giong dispatch._SettingsTarget)."""

    def __init__(self, ctx, frame):
        self.ctx = ctx
        self.frame = frame

def _pane_target(frame) -> tuple:
    try:
        from .panel import pane_target

        return pane_target(frame)
    except Exception as exc:  # noqa: BLE001
        log.info("setup: khong xac dinh duoc app dang mo: %s" % exc)
        return "wps", 0
