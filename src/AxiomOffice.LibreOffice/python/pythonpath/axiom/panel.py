"""Pane Ask AI cua LibreOffice (LibreOffice_arch.md muc 10, 14.3).

Giao dien giong pane cua MS Office/WPS (AskAiPane.cs + PaneControls.cs): header co cham accent, tieu de,
phu de va link "Tro chuyen moi / Ghi nho / Cai dat"; vung chat voi bong bong bo goc, dong thao tac co icon,
the xac nhan, the ghi nho, the loi va man hinh trong co goi y; o soan bo goc co placeholder + nut Gui
indigo; footer trang thai voi link "Dung" / "Hoan tac luot nay". Mau/co chu/khoang cach lay tu theme.py.

Hai cho hien thi, cung mot `PaneUI` ve tren mot UnoControlContainer:

* **Sidebar deck** "Axiom Office" - `Sidebar.xcu` + `Factory.xcu`; factory tra ve `PanelElement` boc
  `AskAiPanel`.
* **Pane neo** (`AskAiDock`) ben phai cua so tai lieu - khi ban LibreOffice khong co sidebar dung duoc.

Luong chay: Gui -> thread nen goi Agent Core (`core.py`) -> SSE do `chat.ChatSession` cap nhat -> main
thread ve lai qua `UnoGate` (awt chi chay tren main thread; da o main thread thi goi thang).
"""
from __future__ import annotations

import os
import threading

import uno
import unohelper
from com.sun.star.awt import XWindowListener
from com.sun.star.frame import XStatusListener
from com.sun.star.ui import XSidebarPanel, XToolPanel, XUIElement, XUIElementFactory

from . import chat, config, core, documents, log, theme
from .awt import peer_of, toolkit
from .chatview import ChatView
from .widgets import (FocusListener, KeyListener, RoundBox, Surface, TextListener, Widget, button, dot, link,
                      make_clickable, set_prop, text_width, _valign_middle)

DECK_ID = "AxiomOfficeDeck"
DECK_TITLE = "Axiom Office"
PANEL_ID = "AskAiPanel"
FACTORY_NAME = "AxiomOfficePanelFactory"
FACTORY_IMPL = "org.axiomoffice.bridge.PanelFactory"
PANEL_URL = "private:resource/toolpanel/%s/%s" % (FACTORY_NAME, PANEL_ID)

DOCK_WIDTH = 380
# kind (ma port) -> tien to ten lenh: wps dung writer.* (giong ban Windows)
PREFIX = {"wps": "writer", "et": "et", "wpp": "wpp"}
DOCKS: dict = {}   # kind -> AskAiDock dang mo (moi kind mot pane; dong = an, mo lai = hien)
SIDEBAR_PANES: list = []   # PaneUI dang song trong sidebar (de test/log; sidebar tu quan ly vong doi)

SHIFT = 1          # com.sun.star.awt.KeyModifier.SHIFT


class WindowListener(unohelper.Base, XWindowListener):
    """Doi kich thuoc cua so (sidebar keo rong/hep, cua so tai lieu doi co) -> xep lai pane."""

    def __init__(self, handler):
        self._handler = handler

    def windowResized(self, event):  # noqa: N802
        self._handler(event.Width, event.Height)

    def windowMoved(self, event):  # noqa: N802
        pass

    def windowShown(self, event):  # noqa: N802
        self._handler(event.Width, event.Height)

    def windowHidden(self, event):  # noqa: N802
        pass

    def disposing(self, event):
        pass


def new_container(ctx, parent_peer, background: int = theme.PANE_BG):
    """UnoControlContainer nen trang, da co peer (ve duoc ngay)."""
    sm = ctx.ServiceManager
    container = sm.createInstanceWithContext("com.sun.star.awt.UnoControlContainer", ctx)
    model = sm.createInstanceWithContext("com.sun.star.awt.UnoControlContainerModel", ctx)
    model.setPropertyValue("BackgroundColor", background)
    container.setModel(model)
    container.createPeer(toolkit(ctx), parent_peer)
    return container


class PaneUI:
    """Giao dien + luot chay cua pane tren mot container (sidebar hoac pane neo)."""

    def __init__(self, ctx, gate, kind: str, port: int, container, width: int, height: int, frame=None,
                 on_close=None, left_border: bool = False, in_sidebar: bool = False):
        self.ctx = ctx
        self.gate = gate
        self.kind = kind
        self.port = port
        self.frame = frame
        self.on_close = on_close
        self.left_border = left_border
        self.in_sidebar = in_sidebar      # deck cua sidebar da co tieu de "Axiom Office": header ghi ten app
        self.container = container
        self.surface = Surface(ctx, container)
        self.session = chat.ChatSession(kind, on_change=self._on_change)
        self.session.status = "Sẵn sàng"
        self.setup_offered = False
        self._stop = threading.Event()
        self._busy = False
        self._clear_prompt = False
        self._focused = False
        self._width, self._height = width, height
        self._build()
        self.resize(width, height)
        self.offer_setup()

    def offer_setup(self) -> None:
        """Chua co cau hinh AI -> nhac o footer va tu mo wizard thiet lap MOT lan (khong lam phien).

        Wizard mo qua gate sau khi pane dung xong (cua so con can cua so tai lieu da san sang).
        """
        if self.setup_offered:
            return
        if config.value("LlmEndpoint", "") and config.value("LlmModel", ""):
            return
        self.setup_offered = True
        self.session.status = "Chưa thiết lập AI — bấm Thiết lập ở trên"
        self._on_change()

        def later() -> None:
            import time

            time.sleep(1.5)
            self.gate.run_quiet(self._open_setup, 20.0, None)

        threading.Thread(target=later, name="axiom-setup-offer", daemon=True).start()

    # ---------------------------------------------------------------- dung giao dien

    def _build(self) -> None:
        s = self.surface
        # Header ------------------------------------------------------------
        self.header_links = []
        link_specs = [("Trò chuyện mới", self._new_chat), ("Ghi nhớ", self._open_memory),
                      ("Thiết lập", self._open_setup)]
        if self.on_close is not None:
            link_specs.append(("✕", self._close))
        for text, handler in link_specs:
            color = theme.TEXT_MUTED if text == "✕" else theme.ACCENT_FG
            item = link(s, text, handler, color=color, hover=theme.TEXT_PRIMARY if text == "✕" else theme.ACTION_HOVER,
                        size=10.0 if text == "✕" else theme.SIZE_CAPTION)
            self.header_links.append(item)
        self.accent_dot = dot(s, "dot", 8)
        self.title = Widget(s)
        title = theme.APP_NAMES.get(self.kind, self.kind) if self.in_sidebar else "Axiom Office"
        self.title_label = self.title.add("FixedText", 0, 0, 120, 20, Label=title, TextColor=theme.TEXT_PRIMARY,
                                          BackgroundColor=theme.PANE_BG, Border=0, NoLabel=True,
                                          font=(theme.FONT_SEMIBOLD, theme.SIZE_TITLE))
        self.subtitle = Widget(s)
        self.subtitle_label = self.subtitle.add("FixedText", 0, 0, 200, 16, Label=self._subtitle(),
                                                TextColor=theme.TEXT_MUTED, BackgroundColor=theme.PANE_BG, Border=0,
                                                NoLabel=True, font=(theme.FONT, theme.SIZE_CAPTION))
        self.header_divider = Widget(s)
        self.header_divider.add("FixedText", 0, 0, 10, 1, Label="", BackgroundColor=theme.DIVIDER, Border=0)
        self.left_line = None
        if self.left_border:
            self.left_line = Widget(s)
            self.left_line.add("FixedText", 0, 0, 1, 10, Label="", BackgroundColor=theme.DIVIDER, Border=0)

        # Footer ------------------------------------------------------------
        self.busy_dot = dot(s, "dot", 6)
        self.status = Widget(s)
        self.status_label = self.status.add("FixedText", 0, 0, 200, theme.FOOTER_H, Label="Sẵn sàng",
                                            TextColor=theme.TEXT_MUTED, BackgroundColor=theme.PANE_BG, Border=0,
                                            VerticalAlign=_valign_middle(), NoLabel=True,
                                            font=(theme.FONT, theme.SIZE_CAPTION))
        self.stop_link = link(s, "Dừng", self._stop_run)
        self.undo_link = link(s, "Hoàn tác lượt này", self._undo_turn)

        # O soan (bo goc, vien; chu + nut them TRUOC nen de nam tren) ---------
        self.composer = Widget(s)
        self.composer_box = RoundBox(self.composer, 300, theme.COMPOSER_H - 12, theme.RADIUS_COMPOSER, theme.PANE_BG,
                                     theme.INPUT_BORDER)
        self.composer_box.add_corners()
        self.placeholder = self.composer.add("FixedText", 0, 0, 10, 20, Label=theme.PROMPT_PLACEHOLDER,
                                             TextColor=theme.TEXT_MUTED, BackgroundColor=theme.PANE_BG, Border=0,
                                             NoLabel=True, font=(theme.FONT, theme.SIZE_BODY))
        self.prompt = self.composer.add("Edit", 0, 0, 10, 20, MultiLine=True, Border=0, BackgroundColor=theme.PANE_BG,
                                        TextColor=theme.TEXT_PRIMARY, VScroll=False, HScroll=False, AutoVScroll=True,
                                        font=(theme.FONT, theme.SIZE_BODY))
        self.hint = self.composer.add("FixedText", 0, 0, 10, 16, Label=theme.PROMPT_HINT, TextColor=theme.TEXT_MUTED,
                                      BackgroundColor=theme.PANE_BG, Border=0, NoLabel=True,
                                      font=(theme.FONT, theme.SIZE_CAPTION))
        self.send = button(s, "Gửi", theme.SEND_W, theme.SEND_H, self._send)
        self.composer_box.add_background()

        # Vung chat ------------------------------------------------------------
        self.chat = ChatView(self.ctx, s, {"suggestion": self._use_suggestion, "confirm": self._answer_confirm,
                                           "forget": self._forget, "settings": self._open_settings})

        self._return_key = uno.getConstantByName("com.sun.star.awt.Key.RETURN")
        self._page_up = uno.getConstantByName("com.sun.star.awt.Key.PAGEUP")
        self._page_down = uno.getConstantByName("com.sun.star.awt.Key.PAGEDOWN")
        self.prompt.addKeyListener(KeyListener(self._on_key, self._on_key_up))
        self.prompt.addTextListener(TextListener(self._on_prompt_changed))
        self.prompt.addFocusListener(FocusListener(self._on_focus))
        make_clickable(s, [self.placeholder], lambda: self.prompt.setFocus())
        self._refresh_send()

    def _subtitle(self) -> str:
        model = str(config.value("LlmModel", "") or "")
        app = theme.APP_NAMES.get(self.kind, self.kind)
        if self.in_sidebar:
            return model or "Agent Core"
        return "%s · %s" % (app, model) if model else app

    # ---------------------------------------------------------------- layout

    def resize(self, width: int, height: int) -> None:
        inset = theme.SIDEBAR_BOTTOM_INSET if self.in_sidebar else 0
        self._width, self._height = max(260, width), max(320, height) - inset
        w, h = self._width, self._height
        pad = theme.PAD_X
        # header
        self.accent_dot.place(pad, 20)
        self.title.place(28, 7)
        title_w = text_width(self.title_label) + 4
        self.title.resize_part(self.title_label, w=title_w)
        right = w - pad
        for item in reversed(self.header_links):
            right -= item.width
            item.place(right, 8)
            right -= 10
        self.title.resize_part(self.title_label, w=min(title_w, max(40, right - 28)))
        self.subtitle.resize_part(self.subtitle_label, w=w - 28 - pad)
        self.subtitle.place(28, 26)
        self.header_divider.resize_part(self.header_divider.parts[0][0], w=w)
        self.header_divider.place(0, theme.HEADER_H - 1)
        if self.left_line is not None:
            self.left_line.resize_part(self.left_line.parts[0][0], h=h)
            self.left_line.place(0, 0)
        # footer
        footer_y = h - theme.FOOTER_H
        self.busy_dot.place(pad, footer_y + (theme.FOOTER_H - 6) // 2)
        self._layout_footer()
        # composer
        box_h = theme.COMPOSER_H - 12
        box_w = w - 2 * pad
        box_y = footer_y - box_h - 4
        self.composer_box.resize(box_w, box_h)
        inner = 12
        self.composer.resize_part(self.prompt, dx=inner, dy=8, w=box_w - 2 * inner, h=box_h - 8 - theme.SEND_H - 12)
        self.composer.resize_part(self.placeholder, dx=inner + 3, dy=10, w=box_w - 2 * inner - 6, h=18)
        self.composer.resize_part(self.hint, dx=inner, dy=box_h - 8 - theme.SEND_H + 8,
                                  w=box_w - 2 * inner - theme.SEND_W - 8, h=16)
        set_prop(self.hint, "Label", theme.PROMPT_HINT if box_w >= 340 else theme.PROMPT_HINT_SHORT)
        self.composer.place(pad, box_y)
        self.send.place(pad + box_w - 8 - theme.SEND_W, box_y + box_h - 8 - theme.SEND_H)
        # chat
        top = theme.HEADER_H
        self.chat.set_rect(1 if self.left_line else 0, top, w - (1 if self.left_line else 0), box_y - 8 - top,
                           self.session)

    def _layout_footer(self) -> None:
        w, h = self._width, self._height
        pad = theme.PAD_X
        footer_y = h - theme.FOOTER_H
        right = w - pad
        for item, visible in ((self.undo_link, self._show_undo()), (self.stop_link, self.session.running)):
            item.set_visible(visible)
            if visible:
                right -= item.width
                item.place(right, footer_y + (theme.FOOTER_H - 18) // 2)
                right -= 12
        left = pad + (12 if self.session.running else 0)
        self.busy_dot.set_visible(self.session.running)
        self.status.resize_part(self.status_label, w=max(40, right - left))
        self.status.place(left, footer_y)

    def _show_undo(self) -> bool:
        return not self.session.running and self.session.edit_count() > 0

    # ---------------------------------------------------------------- o soan

    def _prompt_text(self) -> str:
        try:
            return str(self.prompt.getText() or "")
        except Exception:  # noqa: BLE001
            return ""

    def _on_key(self, event) -> None:
        if event.KeyCode == self._return_key and not (event.Modifiers & SHIFT):
            # O Edit da chen xuong dong TRUOC khi listener nhan phim (thu tren 26.8): bo dung ky tu do tai con tro,
            # neu khong tin nhan co "\n" o giua khi nguoi dung bam Enter luc con tro chua o cuoi.
            self._strip_enter_newline()
            self._clear_prompt = True
            self._send()
        elif event.KeyCode in (self._page_up, self._page_down):
            # Con lan chuot khong toi duoc khung chat tu ve (awt khong co su kien wheel): cuon bang PgUp/PgDn.
            step = max(60, self.chat.rect[3] - 60)
            self.chat.scroll_by(-step if event.KeyCode == self._page_up else step)

    def _strip_enter_newline(self) -> None:
        text = self._prompt_text()
        try:
            position = int(self.prompt.getSelection().Min)
        except Exception:  # noqa: BLE001
            position = len(text)
        for separator in ("\r\n", "\n", "\r"):
            start = position - len(separator)
            if start >= 0 and text[start:position] == separator:
                self.prompt.setText(text[:start] + text[position:])
                return

    def _on_key_up(self, event) -> None:
        if self._clear_prompt:
            # Enter da chen xuong dong vao o soan sau khi gui: xoa not.
            self._clear_prompt = False
            try:
                self.prompt.setText("")
            except Exception:  # noqa: BLE001
                pass
            self._on_prompt_changed()

    def _on_prompt_changed(self) -> None:
        empty = not self._prompt_text().strip()
        try:
            self.placeholder.setVisible(empty)
        except Exception:  # noqa: BLE001
            pass
        self._refresh_send()

    def _on_focus(self, focused: bool) -> None:
        self._focused = focused
        self.composer_box.recolor(theme.PANE_BG, theme.FOCUS if focused else theme.INPUT_BORDER)

    def _refresh_send(self) -> None:
        self.send.set_enabled(not self.session.running and bool(self._prompt_text().strip()))

    def _use_suggestion(self, text: str) -> None:
        if self.session.running:
            return
        try:
            self.prompt.setText(text)
            self.prompt.setFocus()
        except Exception:  # noqa: BLE001
            pass
        self._on_prompt_changed()

    # ---------------------------------------------------------------- luot chay

    def _send(self) -> None:
        if self._busy or self.session.running:
            return
        prompt = self._prompt_text().strip()
        if not prompt:
            return
        try:
            self.prompt.setText("")
        except Exception:  # noqa: BLE001
            pass
        self._busy = True
        self._stop.clear()
        self.session.begin(prompt)
        self._on_prompt_changed()
        threading.Thread(target=self._worker, args=(prompt,), name="axiom-pane-run", daemon=True).start()

    def _worker(self, prompt: str) -> None:
        try:
            base, _ = core.ensure()
            info = self.gate.run_quiet(lambda: documents.describe(documents.active(self.ctx, self.kind, required=False)),
                                      3.0, {}) or {}
            started = core.start_run(base, prompt, self.port, self.kind,
                                     document={"name": info.get("name"), "fullName": info.get("fullName")},
                                     conversation_id=self.session.conversation_id, interactive=True, pid=os.getpid())
            run_id = started.get("runId")
            self.session.run_id = run_id
            self.session.conversation_id = started.get("conversationId") or self.session.conversation_id
            core.stream_events(base, run_id, self._on_event, self._stop)
            final = core.get_run(base, run_id)
            self._ui(lambda: self.session.finish(final))
        except core.CoreError as exc:
            self._ui(lambda: self.session.fail(str(exc)))
        except Exception as exc:  # noqa: BLE001
            log.error("pane run failed: %s" % exc)
            self._ui(lambda: self.session.fail(str(exc)))
        finally:
            self._busy = False
            self._ui(self._render)

    def _on_event(self, event: dict) -> None:
        self._ui(lambda: self.session.handle_event(event))

    def _stop_run(self) -> None:
        if not self.session.running:
            return
        self._stop.set()
        self.session.cancel_requested()
        run_id = self.session.run_id
        try:
            base, _ = core.ensure()
        except core.CoreError:
            return
        if run_id:
            threading.Thread(target=lambda: core.cancel_run(base, run_id), daemon=True).start()

    def _answer_confirm(self, item: dict, approved: bool) -> None:
        run_id, confirmation_id = self.session.run_id, item.get("id")
        self.session.pending = None
        self.session.set_confirm_state(confirmation_id, "approved" if approved else "rejected")
        if run_id and confirmation_id:
            try:
                base, _ = core.ensure()
            except core.CoreError:
                return
            threading.Thread(target=lambda: core.confirm_run(base, run_id, confirmation_id, approved), daemon=True).start()

    def _forget(self, item: dict) -> None:
        memory_id = item.get("id")
        if not memory_id:
            return
        self.session.touch(item, state="deleting")

        def work():
            ok = False
            try:
                base, _ = core.ensure()
                ok = core.delete_memory(base, memory_id)
            except core.CoreError:
                ok = False
            self._ui(lambda: self.session.touch(item, state="deleted" if ok else "failed"))

        threading.Thread(target=work, daemon=True).start()

    def _undo_turn(self) -> None:
        count = self.session.edit_count()
        if not count or self.session.running:
            return
        from . import bridge

        action = PREFIX.get(self.kind, self.kind) + ".undo"
        result = bridge.run_on_main(self.ctx, self.gate, self.kind, action, {"count": count})
        if result.get("ok"):
            outcome = result.get("result") or {}
            undone = outcome.get("undone", 0)
            self.session.clear_edits()
            if outcome.get("stoppedAt") is not None and undone < count:
                # Undo chi go buoc "AI: ..." (documents.undo): nguoi dung da sua sau luot AI thi dung lai.
                self.session.note("Đã hoàn tác %d/%d thao tác của AI rồi dừng lại vì bạn đã sửa tài liệu sau lượt "
                                  "này. Dùng Ctrl+Z nếu muốn hoàn tác thêm." % (undone, count))
            else:
                self.session.note("Đã hoàn tác %d thao tác của lượt vừa rồi." % undone)
        else:
            self.session.note("Không hoàn tác được: %s" % result.get("error"))

    def _new_chat(self) -> None:
        if self.session.running:
            return
        self.session.reset()

    def _open_setup(self) -> None:
        """Mo wizard thiet lap (nguoi dung khong chuyen); "Tuy chon nang cao" trong do mo dialog cu."""
        from . import setupwizard

        setupwizard.open_setup(self)

    def _open_settings(self) -> None:
        from . import dialogs

        dialogs.open_settings(self)

    def _open_memory(self) -> None:
        from . import dialogs

        dialogs.open_memory(self)

    def _close(self) -> None:
        if self.on_close is not None:
            self.on_close()

    # ---------------------------------------------------------------- ve lai

    def _ui(self, fn) -> None:
        """Chay fn tren main thread. Dang o main thread thi goi thang (qua gate se tu treo)."""
        if self.gate.on_main_thread():
            fn()
            return
        self.gate.run_quiet(fn, 5.0, None)

    def _on_change(self) -> None:
        self._ui(self._render)

    def _render(self) -> None:
        try:
            set_prop(self.status_label, "Label", self.session.summary())
            self.chat.sync(self.session)
            self._layout_footer()
            self._refresh_send()
        except Exception as exc:  # noqa: BLE001 - loi ve khong duoc lam sap LibreOffice
            log.error("pane render failed: %s" % exc)


class AskAiPanel(unohelper.Base, XToolPanel, XSidebarPanel):
    """Panel cua sidebar: PaneUI tren container con cua cua so sidebar cap.

    XSidebarPanel bao cho sidebar chieu cao mong muon: Maximum = -1 (khong gioi han) de panel chiem het deck
    - thieu interface nay sidebar coi panel cao 0 va khong hien gi.
    """

    def __init__(self, ctx, gate, parent_window, frame, kind: str, port: int, parent_peer):
        self.ctx = ctx
        self.frame = frame
        rect = parent_window.getPosSize()
        width, height = max(300, int(rect.Width)), max(400, int(rect.Height))
        log.info("sidebar panel: %s port %d, %dx%d" % (kind, port, width, height))
        self.container = new_container(ctx, parent_peer)
        self.container.setPosSize(0, 0, width, height, 15)
        self.container.setVisible(True)
        self.pane = PaneUI(ctx, gate, kind, port, self.container, width, height, frame=frame, in_sidebar=True)
        SIDEBAR_PANES.append(self.pane)
        self.container.addWindowListener(WindowListener(self.pane.resize))

    def getWindow(self):  # noqa: N802 - ten UNO
        return self.container

    def createAccessible(self, parent):  # noqa: N802
        return None

    def getHeightForWidth(self, width):  # noqa: N802 - XSidebarPanel
        size = uno.createUnoStruct("com.sun.star.ui.LayoutSize")
        size.Minimum, size.Maximum, size.Preferred = 360, -1, 600
        return size

    def getMinimalWidth(self):  # noqa: N802
        return 300


class AskAiDock:
    """Pane neo ben phai vung tai lieu (khi khong co sidebar), nhu task pane cua Office.

    Vung tai lieu (frame.ComponentWindow) duoc co hep lai de nhuong cho pane - tai lieu tu xep dong, co
    thanh cuon rieng, khong bi pane de len. Moi lan LibreOffice xep lai cua so (doi co, hien/an thanh cong
    cu) no dat vung tai lieu ve full rong -> listener co lai. Dong pane = an + tra lai be rong.
    Khong huy/tao lai container: tao lai container thu hai o cung cho tren ban 26.8 co luc khong ve.
    """

    def __init__(self, ctx, gate, frame, kind: str, port: int):
        self.ctx = ctx
        self.frame = frame
        self.kind = kind
        self.component = frame.ComponentWindow
        self._applied_width = None     # be rong vung tai lieu do chinh pane dat (de phan biet voi LibreOffice)
        self._shown = False
        rect = self.component.getPosSize()
        self.container = new_container(ctx, peer_of(frame.ContainerWindow))
        self.container.setPosSize(int(rect.X + rect.Width - DOCK_WIDTH), int(rect.Y), DOCK_WIDTH, int(rect.Height), 15)
        self.pane = PaneUI(ctx, gate, kind, port, self.container, DOCK_WIDTH, int(rect.Height), frame=frame,
                           on_close=self.hide, left_border=True)
        self.component.addWindowListener(WindowListener(self._on_component_resize))
        self.show()

    def show(self) -> None:
        self._shown = True
        rect = self.component.getPosSize()
        full = int(rect.Width) if self._applied_width is None else int(rect.Width) + (
            DOCK_WIDTH if int(rect.Width) == self._applied_width else 0)
        self._dock(int(rect.X), int(rect.Y), full, int(rect.Height))
        self.container.setVisible(True)
        try:
            self.pane.prompt.setFocus()
        except Exception:  # noqa: BLE001
            pass

    def hide(self) -> None:
        self._shown = False
        self.container.setVisible(False)
        rect = self.component.getPosSize()
        if self._applied_width is not None and int(rect.Width) == self._applied_width:
            self._applied_width = None
            self.component.setPosSize(int(rect.X), int(rect.Y), int(rect.Width) + DOCK_WIDTH, int(rect.Height), 15)
        self._applied_width = None

    @property
    def visible(self) -> bool:
        return self._shown

    def _dock(self, x: int, y: int, full_width: int, height: int) -> None:
        doc_width = max(200, full_width - DOCK_WIDTH)
        self._applied_width = doc_width
        self.component.setPosSize(x, y, doc_width, height, 15)
        self.container.setPosSize(x + doc_width, y, full_width - doc_width, height, 15)
        self.pane.resize(full_width - doc_width, height)

    def _on_component_resize(self, width: int, height: int) -> None:
        if not self._shown:
            return
        if width == self._applied_width:
            # Chi doi chieu cao (hoac la lan dat cua chinh pane): giu be rong, doi chieu cao pane.
            rect = self.container.getPosSize()
            if int(rect.Height) != height:
                comp = self.component.getPosSize()
                self.container.setPosSize(int(comp.X) + width, int(comp.Y), DOCK_WIDTH, height, 15)
                self.pane.resize(DOCK_WIDTH, height)
            return
        # LibreOffice vua xep lai vung tai lieu ve full rong: co lai.
        comp = self.component.getPosSize()
        self._dock(int(comp.X), int(comp.Y), width, height)


class PanelElement(unohelper.Base, XUIElement):
    def __init__(self, ctx, url: str, frame, panel):
        self.ctx = ctx
        self._url = url
        self._frame = frame
        self._panel = panel

    @property
    def Frame(self):  # noqa: N802
        return self._frame

    @property
    def ResourceURL(self):  # noqa: N802
        return self._url

    @property
    def Type(self):  # noqa: N802
        return uno.getConstantByName("com.sun.star.ui.UIElementType.TOOLPANEL")

    def getRealInterface(self):  # noqa: N802
        return self._panel

    def disposing(self, event):
        pass


class PanelFactory(unohelper.Base, XUIElementFactory):
    """Factory cho panel cua sidebar: Sidebar.xcu tro ImplementationURL vao FACTORY_NAME/PANEL_ID."""

    def __init__(self, ctx):
        self.ctx = ctx

    def createUIElement(self, resource_url, args):  # noqa: N802 - ten UNO
        if not str(resource_url).startswith("private:resource/toolpanel/" + FACTORY_NAME):
            raise _no_such_element(resource_url)
        from . import bridge

        # Sidebar truyen tham so dang PropertyValue co ten (ParentWindow, Frame, Sidebar...), khong theo vi tri.
        named = {}
        for arg in args or ():
            name = getattr(arg, "Name", None)
            if name:
                named[name] = arg.Value
        parent_window = named.get("ParentWindow") or (args[0] if args and not named else None)
        frame = named.get("Frame")
        if parent_window is None:
            raise _no_such_element("missing ParentWindow")
        kind, port = pane_target(frame)
        panel = AskAiPanel(self.ctx, bridge.gate(), parent_window, frame, kind, port, peer_of(parent_window))
        return PanelElement(self.ctx, str(resource_url), frame, panel)


def _no_such_element(resource_url):
    from com.sun.star.container import NoSuchElementException

    return NoSuchElementException("unknown sidebar panel: " + str(resource_url))


def pane_target(frame):
    """(kind, port) cua pane: theo tai lieu dang mo trong cua so, khong thi kind dau tien."""
    from . import bridge

    kind = None
    try:
        model = frame.getController().getModel()
        kind = next((k for k in config.KINDS if documents._is(model, k)), None)  # noqa: SLF001
    except Exception:  # noqa: BLE001
        kind = None
    return bridge.pane_target(kind)


# ---------------------------------------------------------------- mo pane

class _StatusListener(unohelper.Base, XStatusListener):
    def __init__(self):
        self.enabled = False
        self.checked = False

    def statusChanged(self, event):  # noqa: N802
        try:
            self.enabled = bool(event.IsEnabled)
            self.checked = event.State is True
        except BaseException:  # noqa: BLE001 - khong duoc nem loi ra C++
            pass

    def disposing(self, event):
        pass


def command_state(ctx, frame, command: str) -> tuple[bool, bool]:
    """(enabled, checked) cua mot lenh .uno:... - vd .uno:Sidebar checked = sidebar dang hien.

    XDispatch.addStatusListener(listener, url) goi statusChanged ngay (dong bo) voi trang thai hien tai.
    """
    listener = _StatusListener()
    try:
        transformer = ctx.ServiceManager.createInstanceWithContext("com.sun.star.util.URLTransformer", ctx)
        url = uno.createUnoStruct("com.sun.star.util.URL")
        url.Complete = command
        parsed = transformer.parseStrict(url)
        parsed_url = parsed[1] if isinstance(parsed, tuple) and len(parsed) > 1 else parsed
        provider = frame.queryInterface(uno.getTypeByName("com.sun.star.frame.XDispatchProvider"))
        item = provider.queryDispatch(parsed_url, "", 0)
        if item is None:
            return False, False
        item.addStatusListener(listener, parsed_url)
        item.removeStatusListener(listener, parsed_url)
    except Exception as exc:  # noqa: BLE001
        log.info("command_state %s: %s" % (command, exc))
    return listener.enabled, listener.checked


def sidebar_visible(ctx, frame) -> bool:
    """Sidebar cua cua so dang hien? (XSidebarProvider khong co o ban 26.8 nen doc trang thai .uno:Sidebar.)"""
    enabled, checked = command_state(ctx, frame, ".uno:Sidebar")
    return enabled and checked


def dispatch(ctx, frame, command: str) -> bool:
    """Chay mot lenh .uno:... trong cua so.

    Khong dung DispatchHelper.executeDispatch: pyuno loi "Type 17 is not supported!" khi truyen struct URL
    qua helper; di thang queryDispatch/dispatch cua XDispatchProvider thi chay dung.
    """
    sm = ctx.ServiceManager
    transformer = sm.createInstanceWithContext("com.sun.star.util.URLTransformer", ctx)
    url = uno.createUnoStruct("com.sun.star.util.URL")
    url.Complete = command
    parsed = transformer.parseStrict(url)
    parsed_url = parsed[1] if isinstance(parsed, tuple) and len(parsed) > 1 else parsed
    provider = frame.queryInterface(uno.getTypeByName("com.sun.star.frame.XDispatchProvider"))
    item = provider.queryDispatch(parsed_url, "", 0)
    if item is None:
        log.info("dispatch %s: khong co dich" % command)
        return False
    item.dispatch(parsed_url, ())
    return True


def _same(a, b) -> bool:
    try:
        return a == b
    except Exception:  # noqa: BLE001
        return False


def show_pane(ctx, frame) -> dict:
    """Lenh ui.askpane: mo deck trong sidebar neu ban LibreOffice nay co sidebar, khong thi pane neo."""
    from . import bridge

    kind, port = pane_target(frame)
    if sidebar_visible(ctx, frame):
        # Sidebar cua LibreOffice dang hien (Calc/Impress mac dinh): mo deck Axiom Office trong do.
        dispatch(ctx, frame, ".uno:SidebarDeck." + DECK_ID)
        return {"taskPane": True, "host": "sidebar", "kind": kind}

    dock = DOCKS.get(kind)
    if dock is not None and _same(dock.frame, frame):
        dock.show()
        return {"taskPane": True, "host": "dock", "kind": kind}
    DOCKS[kind] = AskAiDock(ctx, bridge.gate(), frame, kind, port)
    return {"taskPane": True, "host": "dock", "kind": kind}
