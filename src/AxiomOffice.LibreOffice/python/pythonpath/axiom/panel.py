"""Pane Ask AI cua LibreOffice (LibreOffice_arch.md muc 10).

Hai cho hien thi, cung mot giao dien (`PaneUI`):

* **Sidebar deck** "Axiom Office" - `Sidebar.xcu` + `Factory.xcu` dang ky; factory trong file nay tra ve
  `PanelElement` boc `AskAiPanel`. Sidebar goi factory khi nguoi dung mo deck.
* **Cua so noi** (`AskAiWindow`) - dung khi sidebar khong kha dung (mot so ban LibreOffice khong tao phan
  tu sidebar: layout khong co `private:resource/uielement/sidebar`). `ui.askpane` mo deck neu co, khong
  thi mo cua so.

Luong chay: nut Gui -> thread nen goi Agent Core (`core.py`) -> SSE do `chat.ChatSession` cap nhat ->
main thread ve lai awt qua `UnoGate` (moi thao tac awt phai o main thread, va KHONG duoc goi lai gate khi
da o main thread - se tu treo).
"""
from __future__ import annotations

import os
import threading

import uno
import unohelper
from com.sun.star.awt import XKeyListener, XWindowListener
from com.sun.star.ui import XToolPanel, XUIElement, XUIElementFactory

from . import chat, core, documents, log
from .awt import BOLD, Container, Dialog, bind_actions, peer_of, toolkit

DECK_ID = "AxiomOfficeDeck"
DECK_TITLE = "Axiom Office"
PANEL_ID = "AskAiPanel"
FACTORY_NAME = "AxiomOfficePanelFactory"
FACTORY_IMPL = "org.axiomoffice.bridge.PanelFactory"
PANEL_URL = "private:resource/toolpanel/%s/%s" % (FACTORY_NAME, PANEL_ID)

MARGIN = 6
GAP = 4
BUTTONS = ("newchat", "undo", "settings", "memory")
BUTTON_LABELS = {"newchat": "Trò chuyện mới", "undo": "Hoàn tác lượt này", "settings": "Cài đặt", "memory": "Ghi nhớ",
                 "send": "Gửi", "stop": "Dừng", "confirm_yes": "Đồng ý", "confirm_no": "Từ chối", "close": "Đóng"}
_ALL_BUTTONS = ("send", "stop", "newchat", "undo", "settings", "memory", "confirm_yes", "confirm_no", "close")

DOCK_WIDTH = 380
# kind (ma port) -> tien to ten lenh: wps dung writer.* (giong ban Windows)
PREFIX = {"wps": "writer", "et": "et", "wpp": "wpp"}
DOCKS: dict = {}   # kind -> AskAiDock dang mo (moi kind mot pane, khong mo trung)


class KeyListener(unohelper.Base, XKeyListener):
    def __init__(self, handler):
        self._handler = handler

    def keyPressed(self, event):  # noqa: N802
        self._handler(event)

    def keyReleased(self, event):  # noqa: N802
        pass

    def disposing(self, event):
        pass


class WindowListener(unohelper.Base, XWindowListener):
    """Doi kich thuoc cua so (sidebar keo rong/hep) -> ve lai layout."""

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


class PaneUI:
    """Giao dien + luot chay cua pane, khong phu thuoc host la container hay dialog."""

    def __init__(self, ctx, gate, kind: str, port: int, host, width: int = 300, height: int = 620, frame=None):
        self.ctx = ctx
        self.gate = gate
        self.kind = kind
        self.port = port
        self.frame = frame
        self.host = host
        self.session = chat.ChatSession(kind, on_change=self._on_change)
        self.session.status = "Sẵn sàng"
        self._stop = threading.Event()
        self._busy = False
        self._width, self._height = width, height
        self._build()
        self._layout(width, height)

    # ---------------------------------------------------------------- dung giao dien

    def _build(self) -> None:
        host = self.host
        host.add("FixedText", "header", PositionX=MARGIN, PositionY=MARGIN, Width=280, Height=16, CharWeight=BOLD,
                 Label=self._header())
        host.add("Edit", "transcript", PositionX=MARGIN, PositionY=28, Width=288, Height=300, MultiLine=True,
                 ReadOnly=True, AutoVScroll=True, VScroll=True)
        host.add("FixedText", "confirm", PositionX=MARGIN, PositionY=330, Width=288, Height=28, MultiLine=True,
                 CharWeight=BOLD, Visible=False)
        host.add("Edit", "input", PositionX=MARGIN, PositionY=380, Width=200, Height=24)
        host.add("Button", "send", PositionX=212, PositionY=380, Width=40, Height=24, Label=BUTTON_LABELS["send"])
        host.add("Button", "stop", PositionX=256, PositionY=380, Width=44, Height=24, Label=BUTTON_LABELS["stop"],
                 Enabled=False)
        host.add("Button", "confirm_yes", PositionX=MARGIN, PositionY=352, Width=70, Height=22,
                 Label=BUTTON_LABELS["confirm_yes"], Visible=False)
        host.add("Button", "confirm_no", PositionX=MARGIN + 76, PositionY=352, Width=70, Height=22,
                 Label=BUTTON_LABELS["confirm_no"], Visible=False)
        host.add("Button", "close", PositionX=330, PositionY=MARGIN, Width=44, Height=18, Label=BUTTON_LABELS["close"])
        for name in BUTTONS:
            host.add("Button", name, PositionX=MARGIN, PositionY=420, Width=60, Height=24, Label=BUTTON_LABELS[name])
        host.add("FixedText", "status", PositionX=MARGIN, PositionY=450, Width=288, Height=16, Label="Sẵn sàng")

        self._return_key = uno.getConstantByName("com.sun.star.awt.Key.RETURN")
        host.control("input").addKeyListener(KeyListener(self._on_key))
        bind_actions(host, _ALL_BUTTONS, self._on_action)

    def _header(self) -> str:
        return "Axiom Office · %s" % documents.NAMES.get(self.kind, self.kind)

    # ---------------------------------------------------------------- layout

    def _layout(self, width: int, height: int) -> None:
        self._width, self._height = max(240, width), max(300, height)
        w, h = self._width, self._height
        inner = w - 2 * MARGIN
        h_status, h_buttons, h_input = 16, 24, 24
        h_confirm = 34 if self.session.pending else 0

        y_status = h - MARGIN - h_status
        y_buttons = y_status - GAP - h_buttons
        y_input = y_buttons - GAP - h_input
        y_confirm = y_input - GAP - h_confirm if h_confirm else y_input
        top = MARGIN + 16 + GAP
        transcript_h = max(80, (y_confirm if h_confirm else y_input) - GAP - top)

        ui = self.host
        ui.set("header", PositionX=MARGIN, PositionY=MARGIN, Width=inner, Height=16)
        ui.set("transcript", PositionX=MARGIN, PositionY=top, Width=inner, Height=transcript_h)
        send_w, stop_w = 40, 44
        input_w = max(60, inner - send_w - stop_w - 2 * GAP)
        ui.set("input", PositionX=MARGIN, PositionY=y_input, Width=input_w, Height=h_input)
        ui.set("send", PositionX=MARGIN + input_w + GAP, PositionY=y_input, Width=send_w, Height=h_input)
        ui.set("stop", PositionX=MARGIN + input_w + GAP + send_w + GAP, PositionY=y_input, Width=stop_w, Height=h_input)
        if h_confirm:
            ui.set("confirm", PositionX=MARGIN, PositionY=y_confirm, Width=inner, Height=h_confirm - 24)
            ui.set("confirm_yes", PositionX=MARGIN, PositionY=y_confirm + h_confirm - 22, Width=70, Height=22)
            ui.set("confirm_no", PositionX=MARGIN + 76, PositionY=y_confirm + h_confirm - 22, Width=70, Height=22)
        share = max(52, inner // len(BUTTONS))
        for index, name in enumerate(BUTTONS):
            ui.set(name, PositionX=MARGIN + index * share, PositionY=y_buttons, Width=share - GAP, Height=h_buttons)
        ui.set("status", PositionX=MARGIN, PositionY=y_status, Width=inner, Height=h_status)

    def close(self) -> None:
        """Nut Dong: panel trong sidebar thi an (khong pha), pane neo thi go han."""
        if hasattr(self.host, "dispose"):
            try:
                self.host.dispose()
            except Exception:  # noqa: BLE001
                pass

    def on_resize(self, width: int, height: int) -> None:
        if width > 0 and height > 0 and (width, height) != (self._width, self._height):
            self._layout(width, height)

    # ---------------------------------------------------------------- awt -> logic

    def _on_key(self, event) -> None:
        if event.KeyCode == self._return_key:
            self._send()

    def _on_action(self, command: str, event) -> None:
        if command == "send":
            self._send()
        elif command == "stop":
            self._stop_run()
        elif command == "newchat":
            self.session.reset()
            self._render()
        elif command == "undo":
            self._undo_turn()
        elif command == "confirm_yes":
            self._answer_confirm(True)
        elif command == "confirm_no":
            self._answer_confirm(False)
        elif command == "settings":
            from . import dialogs

            dialogs.open_settings(self)
        elif command == "memory":
            from . import dialogs

            dialogs.open_memory(self)
        elif command == "close":
            self.close()

    # ---------------------------------------------------------------- luot chay

    def _send(self) -> None:
        if self._busy or self.session.running:
            return
        prompt = str(self.host.value("input", "Text") or "").strip()
        if not prompt:
            return
        self.host.set("input", Text="")
        self._busy = True
        self._stop.clear()
        self.session.begin(prompt)
        self._render()
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
            self._ui(lambda: (self.session.finish(final), self._render()))
        except core.CoreError as exc:
            self._ui(lambda: (self.session.fail(str(exc)), self._render()))
        except Exception as exc:  # noqa: BLE001
            log.error("pane run failed: %s" % exc)
            self._ui(lambda: (self.session.fail(str(exc)), self._render()))
        finally:
            self._busy = False

    def _on_event(self, event: dict) -> None:
        self.session.handle_event(event)
        self._ui(self._render)

    def _stop_run(self) -> None:
        if not self.session.running:
            return
        self._stop.set()
        self.session.cancel_requested()
        self._render()
        run_id = self.session.run_id
        try:
            base, _ = core.ensure()
        except core.CoreError:
            return
        if run_id:
            threading.Thread(target=lambda: core.cancel_run(base, run_id), daemon=True).start()

    def _answer_confirm(self, approved: bool) -> None:
        pending = self.session.pending or {}
        run_id, confirmation_id = self.session.run_id, pending.get("confirmationId")
        self.session.pending = None
        self.session.note("• Bạn đã %s: %s" % ("đồng ý" if approved else "từ chối", pending.get("action") or ""))
        self._render()
        if run_id and confirmation_id:
            try:
                base, _ = core.ensure()
            except core.CoreError:
                return
            threading.Thread(target=lambda: core.confirm_run(base, run_id, confirmation_id, approved), daemon=True).start()

    def _undo_turn(self) -> None:
        count = self.session.edit_count()
        if not count:
            self.session.note("(lượt này chưa có thao tác nào để hoàn tác)")
            self._render()
            return
        from . import bridge

        action = PREFIX.get(self.kind, self.kind) + ".undo"
        result = bridge.run_on_main(self.ctx, self.gate, self.kind, action, {"count": count})
        if result.get("ok"):
            undone = (result.get("result") or {}).get("undone", 0)
            self.session.note("⏪ Đã hoàn tác %d bước" % undone)
            self.session.clear_edits()
        else:
            self.session.note("Không hoàn tác được: %s" % result.get("error"))
        self._render()

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
        host = self.host
        host.set("transcript", Text=self.session.transcript())
        host.set("status", Label=self.session.summary())
        pending = self.session.pending
        host.show("confirm", bool(pending))
        host.show("confirm_yes", bool(pending))
        host.show("confirm_no", bool(pending))
        if pending:
            host.set("confirm", Label="Cần xác nhận: %s\n%s" % (pending.get("action"), pending.get("reason") or ""))
        host.enable("stop", bool(self.session.running))
        host.enable("send", not self.session.running)
        host.enable("input", not self.session.running)
        self._layout(self._width, self._height)


class AskAiPanel(unohelper.Base, XToolPanel):
    """Panel cua sidebar: boc PaneUI trong container cua so con do sidebar cap."""

    def __init__(self, ctx, gate, parent_window, frame, kind: str, port: int, parent_peer):
        self.ctx = ctx
        self.frame = frame
        self.ui = Container(ctx, toolkit(ctx), 300, 620)
        self.pane = PaneUI(ctx, gate, kind, port, self.ui, frame=frame)
        self.ui.create_peer(parent_peer)
        self.ui.set_visible(True)
        self.ui.window().addWindowListener(WindowListener(self.pane.on_resize))

    def getWindow(self):  # noqa: N802 - ten UNO
        return self.ui.window()

    def createAccessible(self, parent):  # noqa: N802
        return None


class AskAiDock:
    """Pane neo vao cua so tai lieu (cua so con goc phai vung soan thao).

    Dung khi ban LibreOffice nay khong co sidebar. Cua so con luon nam TRONG cua so tai lieu nen khong bi
    che khuat (khac dialog roi: tren ban nay dialog khong hien len duoc - xem ghi chu trong LibreOffice_arch.md).
    """

    def __init__(self, ctx, gate, frame, kind: str, port: int):
        self.ctx = ctx
        self.frame = frame
        self.kind = kind
        self.parent = frame.ContainerWindow
        rect = self.parent.getPosSize()
        height = self._height(rect)
        self.ui = Container(ctx, toolkit(ctx), DOCK_WIDTH, height)
        self.pane = PaneUI(ctx, gate, kind, port, self.ui, DOCK_WIDTH, height, frame=frame)
        self.ui.create_peer(peer_of(self.parent))
        self.parent.addWindowListener(WindowListener(self._on_parent_resize))
        self.place()

    def _height(self, rect) -> int:
        return max(320, min(int(rect.Height) - 60, 900))

    def place(self) -> None:
        rect = self.parent.getPosSize()
        height = self._height(rect)
        self.ui.resize(max(DOCK_WIDTH, int(rect.Width) - DOCK_WIDTH - 34), 44, DOCK_WIDTH, height)
        self.ui.set_visible(True)

    def _on_parent_resize(self, width: int, height: int) -> None:
        self.place()
        self.pane.on_resize(DOCK_WIDTH, self._height(self.parent.getPosSize()))

    def close(self) -> None:
        self.ui.dispose()


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

        parent_window = args[0]
        frame = args[1] if len(args) > 1 else None
        kind, port = pane_target(frame)
        panel = AskAiPanel(self.ctx, bridge.gate(), parent_window, frame, kind, port, peer_of(parent_window))
        return PanelElement(self.ctx, str(resource_url), frame, panel)


def _no_such_element(resource_url):
    from com.sun.star.container import NoSuchElementException

    return NoSuchElementException("unknown sidebar panel: " + str(resource_url))


def pane_target(frame):
    """(kind, port) cua pane: theo tai lieu dang mo trong cua so, khong thi kind dau tien."""
    from . import bridge, config

    kind = None
    try:
        model = frame.getController().getModel()
        kind = next((k for k in config.KINDS if documents._is(model, k)), None)  # noqa: SLF001
    except Exception:  # noqa: BLE001
        kind = None
    return bridge.pane_target(kind)


# ---------------------------------------------------------------- mo pane

def current_sidebar(ctx, frame):
    """Doi tuong sidebar cua cua so (None khi ban LibreOffice nay khong tao sidebar)."""
    try:
        controller = frame.getController()
        provider = controller.queryInterface(uno.getTypeByName("com.sun.star.ui.XSidebarProvider"))
        return provider.getSidebar() if provider is not None else None
    except Exception:  # noqa: BLE001
        return None


def sidebar_available(ctx, frame) -> bool:
    return current_sidebar(ctx, frame) is not None


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


def show_pane(ctx, frame) -> dict:
    """Lenh ui.askpane: mo deck trong sidebar neu ban LibreOffice nay co sidebar, khong thi mo cua so noi."""
    from . import bridge

    kind, port = pane_target(frame)
    if sidebar_available(ctx, frame):
        dispatch(ctx, frame, ".uno:SidebarDeck." + DECK_ID)
        return {"taskPane": True, "host": "sidebar", "kind": kind}

    dock = DOCKS.get(kind)
    if dock is not None:
        dock.place()
        return {"taskPane": True, "host": "dock", "kind": kind}
    DOCKS[kind] = AskAiDock(ctx, bridge.gate(), frame, kind, port)
    return {"taskPane": True, "host": "dock", "kind": kind}
