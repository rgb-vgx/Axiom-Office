"""Dung giao dien awt bang code (khong can file .xdl/.ui).

Hai loai "host" cho cung mot bo dieu khien:

* `Container` - `com.sun.star.awt.UnoControlContainer`: cua so con de nhung (panel sidebar);
  dieu khien them bang `addControl(name, control)`.
* `Dialog` - `com.sun.star.awt.UnoControlDialog`: cua so noi (modeless) co tieu de, keo tha duoc;
  dieu khien them bang mo hinh (`model.insertByName`) - lop `Dialog` che di khac biet nay.

Ca hai deu tra XWindow qua `window()` va cho lay dieu khien qua `control(name)` (XControlContainer),
nen phan logic/layout trong panel.py chi can biet giao dien chung nay.

Don vi toa do/kich thuoc la pixel, cung don vi voi `setPosSize`/`getPosSize` cua cua so.
Chi chay tren main thread: pane goi chung qua `UnoGate` (xem `panel.py`).
"""
from __future__ import annotations

import uno
import unohelper
from com.sun.star.awt import XActionListener, XTextListener

BOLD = 150.0
NORMAL = 100.0


def log_error(message: str) -> None:
    from . import log

    log.error(message)


class ActionListener(unohelper.Base, XActionListener):
    """Goi lai theo `ActionCommand` cua nut (uoc giua nhieu nut tren cung mot host)."""

    def __init__(self, handler):
        self._handler = handler

    def actionPerformed(self, event):  # noqa: N802 - ten UNO
        self._handler(event.ActionCommand, event)

    def disposing(self, event):
        pass


class TextListener(unohelper.Base, XTextListener):
    def __init__(self, handler):
        self._handler = handler

    def textChanged(self, event):  # noqa: N802
        self._handler(event)

    def disposing(self, event):
        pass


class _Host:
    """Phan chung cua Container/Dialog: quan ly mo hinh dieu khien + bang ten."""

    GEOMETRY = ("PositionX", "PositionY", "Width", "Height")

    def __init__(self, ctx):
        self.ctx = ctx
        self.models: dict[str, object] = {}
        self.geometry: dict[str, dict] = {}
        self.peer_ready = False

    def add(self, service: str, name: str, **props):
        """service: 'Edit', 'Button', 'FixedText', 'FixedLine', 'ListBox', 'CheckBox', 'ProgressBar'."""
        model = self.ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.UnoControl" + service + "Model", self.ctx)
        self._remember_geometry(name, props)
        for key, value in props.items():
            if key not in self.GEOMETRY:
                self._try_set(model, key, value)
        self._insert(name, service, model)
        self.models[name] = model
        if self.peer_ready:
            # Dieu khien them sau khi tao peer (vd dialog mo sau pane): dat hinh hoc ngay.
            self.apply_geometry(name)
        return model

    def set(self, name: str, **props) -> None:
        self._remember_geometry(name, props)
        for key, value in props.items():
            if key in self.GEOMETRY and not self.model_geometry:
                continue        # lop con tu ap dung (xem Dialog.apply_geometry)
            self._try_set(self.models[name], key, value)
        if not self.model_geometry:
            self.apply_geometry(name)

    # Ban LibreOffice nay khong nhan PositionX/PositionY/Width/Height tren model dieu khien (chi co
    # thuoc tinh rieng cua tung loai: Text, Label, State...), nen hinh hoc phai dat len view sau khi
    # tao peer - xem apply_geometry.
    model_geometry = False

    def _remember_geometry(self, name: str, props: dict) -> None:
        found = {key: value for key, value in props.items() if key in self.GEOMETRY}
        if found:
            self.geometry.setdefault(name, {}).update(found)

    def apply_geometry(self, name: str = None) -> None:
        """Dat vi tri/kich thuoc len chinh dieu khien (view), don vi pixel."""
        names = [name] if name else list(self.geometry)
        for item in names:
            rect = self.geometry.get(item)
            if not rect or "Width" not in rect or "Height" not in rect:
                continue
            try:
                self.control(item).setPosSize(rect.get("PositionX", 0), rect.get("PositionY", 0),
                                              rect["Width"], rect["Height"], 15)
            except Exception:  # noqa: BLE001
                pass

    def value(self, name: str, prop: str):
        return self.models[name].getPropertyValue(prop)


    def show(self, name: str, visible: bool) -> None:
        self.set(name, Visible=visible)

    def enable(self, name: str, enabled: bool) -> None:
        self.set(name, Enabled=enabled)

    def _try_set(self, model, key: str, value) -> None:
        """Thuoc tinh khong co (khac ban LibreOffice) thi bo qua thay vi lam hong ca panel."""
        try:
            model.setPropertyValue(key, value)
        except Exception:  # noqa: BLE001
            pass


class Container(_Host):
    """Container dieu khien awt (khong phai dialog): nhung vao cua so cha bat ky."""

    def __init__(self, ctx, toolkit, width: int = 300, height: int = 400):
        super().__init__(ctx)
        self.toolkit = toolkit
        self.container = ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.UnoControlContainer", ctx)
        model = ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.UnoControlContainerModel", ctx)
        model.setPropertyValue("Width", width)
        model.setPropertyValue("Height", height)
        self.container.setModel(model)
        self.model = model

    def _insert(self, name: str, service: str, model) -> None:
        control = self.ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.UnoControl" + service, self.ctx)
        control.setModel(model)
        self.container.addControl(name, control)

    def control(self, name: str):
        return self.container.getControl(name)

    def create_peer(self, parent_peer) -> None:
        self.container.createPeer(self.toolkit, parent_peer)
        self.peer_ready = True
        self.apply_geometry()

    def window(self):
        return self.container

    def resize(self, x: int, y: int, width: int, height: int) -> None:
        self.container.setPosSize(x, y, width, height, 15)

    def set_visible(self, visible: bool) -> None:
        self.container.setVisible(visible)

    def dispose(self) -> None:
        try:
            self.container.dispose()
        except Exception:  # noqa: BLE001
            pass


class Dialog(_Host):
    """Cua so noi (modeless) co tieu de rieng - dung khi sidebar khong kha dung.

    Mo hinh dieu khien cua dialog tinh bang map unit (1/100 mm), con cua so tinh bang pixel. PaneUI tinh
    layout theo pixel nen lop nay quy doi 4 thuoc tinh hinh hoc khi ghi xuong mo hinh (he so lay tu
    pixelToLogic cua chinh dialog, khac nhau theo DPI man hinh).
    """

    def __init__(self, ctx, toolkit, title: str, width: int = 400, height: int = 640):
        super().__init__(ctx)
        self.toolkit = toolkit
        self.dialog = ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.UnoControlDialog", ctx)
        model = ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.UnoControlDialogModel", ctx)
        self.dialog.setModel(model)
        self.model = model
        model.setPropertyValue("Title", title)
        # DesktopAsParent=True: dialog la con cua desktop nen nam DUOI cua so tai lieu; dat False de noi tren
        # cua so cha (pane phai luon thay duoc khi nguoi dung dang sua tai lieu).
        self._try_set(model, "DesktopAsParent", False)
        unit = 26.4583   # pixel -> 1/100 mm o 96 DPI; dialog khong nhan PositionX nen phai quy doi tay
        try:
            unit = self.dialog.pixelToLogic(1000, uno.getConstantByName("com.sun.star.util.MeasureUnit.MM_100TH")) / 1000.0
        except Exception:  # noqa: BLE001
            pass
        model.setPropertyValue("Width", int(round(width * unit)))
        model.setPropertyValue("Height", int(round(height * unit)))

    def _insert(self, name: str, service: str, model) -> None:
        self.model.insertByName(name, model)

    def control(self, name: str):
        return self.dialog.getControl(name)

    def create_peer(self, parent_peer=None) -> None:
        self.dialog.createPeer(self.toolkit, parent_peer)
        self.peer_ready = True
        self.apply_geometry()

    def window(self):
        return self.dialog

    def resize(self, x: int, y: int, width: int, height: int) -> None:
        self.dialog.setPosSize(x, y, width, height, 15)

    def set_visible(self, visible: bool) -> None:
        self.dialog.setVisible(visible)

    def end_execute(self) -> None:
        try:
            self.dialog.endExecute()
        except Exception:  # noqa: BLE001
            self.set_visible(False)

    def to_front(self) -> None:
        try:
            self.dialog.toFront()
        except Exception as exc:  # noqa: BLE001
            log_error("toFront: %s" % exc)
        try:
            self.dialog.setFocus()
        except Exception:  # noqa: BLE001
            pass

    def dispose(self) -> None:
        try:
            self.dialog.dispose()
        except Exception:  # noqa: BLE001
            pass

    @property
    def disposed(self) -> bool:
        try:
            return self.dialog.getPeer() is None
        except Exception:  # noqa: BLE001
            return True


class ChildWindow(Container):
    """Cua so con co vien trong cua so tai lieu: dung cho pane neo va cac dialog.

    Tren ban LibreOffice nay dialog roi (UnoControlDialog) khong hien len duoc, con cua so con thi hien
    dung va luon nam trong cua so tai lieu (khong bi che khuat).
    """

    def __init__(self, ctx, parent_window, width: int, height: int):
        super().__init__(ctx, toolkit(ctx), width, height)
        self.parent = parent_window
        self.ctx = ctx
        self.create_peer(peer_of(parent_window))

    def place(self, x: int, y: int, width: int, height: int) -> None:
        self.resize(x, y, width, height)
        self.set_visible(True)

    def close(self) -> None:
        self.dispose()


def bind_actions(host, names, handler) -> None:
    """Gan mot ActionListener chung; moi nut tu khai ActionCommand = ten cua no."""
    listener = ActionListener(handler)
    for name in names:
        host.models[name].setPropertyValue("ActionCommand", name)
        host.control(name).addActionListener(listener)


def toolkit(ctx):
    return ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.Toolkit", ctx)


def peer_of(window):
    return window.queryInterface(uno.getTypeByName("com.sun.star.awt.XWindowPeer"))


def message_box(ctx, parent_window, title: str, text: str, box_type: int = 1):
    """1 = INFO, 2 = WARNING, 3 = ERROR, 5 = QUERYBOX (co Yes/No), 6 = QUERYBOX_RETRY_CANCEL."""
    box = toolkit(ctx).createMessageBox(parent_window, box_type, 1, title, text)
    try:
        return box.execute()
    finally:
        try:
            box.dispose()
        except Exception:  # noqa: BLE001
            pass
