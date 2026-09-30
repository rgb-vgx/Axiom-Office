"""Dung giao dien awt bang code (khong can file .xdl/.ui).

`Container` boc `com.sun.star.awt.UnoControlContainer` (cua so con nhung vao cua so khac, dieu khien them
bang ten); `ChildWindow` la Container nen trang vien mong dung cho dialog Cai dat/Ghi nho (dialogs.py).
Khong dung UnoControlDialog: tren ban 26.8 dialog roi khong hien len (LibreOffice_arch.md 14.2).
Pane Ask AI dung widgets.py (ve theo toa do, khong theo ten).

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
    """Quan ly mo hinh dieu khien + bang ten (lop con cung cap _insert/control)."""

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
                continue        # dat len view trong apply_geometry
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


class ChildWindow(Container):
    """Cua so con co vien trong cua so tai lieu: dung cho pane neo va cac dialog.

    Tren ban LibreOffice nay dialog roi (UnoControlDialog) khong hien len duoc, con cua so con thi hien
    dung va luon nam trong cua so tai lieu (khong bi che khuat).
    """

    def __init__(self, ctx, parent_window, width: int, height: int):
        from . import theme

        super().__init__(ctx, toolkit(ctx), width, height)
        self.parent = parent_window
        self.ctx = ctx
        # Nen trang + vien mong nhu the/pane (theme.py), khong dung mau xam mac dinh cua awt.
        self._try_set(self.model, "BackgroundColor", theme.PANE_BG)
        self._try_set(self.model, "Border", 2)
        self._try_set(self.model, "BorderColor", theme.INPUT_BORDER)
        self.create_peer(peer_of(parent_window))

    def add(self, service: str, name: str, **props):
        """Mac dinh font/mau cua pane cho moi dieu khien (nhan, o nhap, o danh dau, nut)."""
        from . import theme

        if service in ("FixedText", "Edit", "CheckBox", "Button", "ListBox"):
            props.setdefault("FontName", theme.FONT)
            props.setdefault("FontHeight", theme.SIZE_SMALL)
        if service in ("FixedText", "CheckBox"):
            props.setdefault("BackgroundColor", theme.PANE_BG)
            props.setdefault("TextColor", theme.TEXT_PRIMARY)
        if not props.get("FontName"):
            props.pop("FontName", None)
        return super().add(service, name, **props)

    def place(self, x: int, y: int, width: int, height: int) -> None:
        self.resize(x, y, width, height)
        self.set_visible(True)

    def close(self) -> None:
        self.dispose()


class Dialog(_Host):
    """Cua so noi that (UnoControlDialog) - co X window rieng, khac `ChildWindow`.

    Tren LibreOffice 24.2/Linux, cua so con VCL ve TRONG vung tai lieu nhung KHONG nhan duoc su kien chuot:
    cua so tai lieu la mot X window that nam tren cung nen moi cu bam deu roi vao no (do bang xdotool: pane
    trong sidebar/dock bam duoc, dialog kieu cua so con thi khong). Dialog that co cua so rieng nen bam duoc -
    day la duong dung cho dialog cua extension.

    Dung giong `ChildWindow`: `.add(service, name, **props)` (tra ve model), `.container` de tao
    `widgets.Surface`, `.place(x, y, w, h)`, `.close()`.
    """

    def __init__(self, ctx, width: int, height: int, title: str = ""):
        from . import theme

        super().__init__(ctx)
        sm = ctx.ServiceManager
        self.container = sm.createInstanceWithContext("com.sun.star.awt.UnoControlDialog", ctx)
        model = sm.createInstanceWithContext("com.sun.star.awt.UnoControlDialogModel", ctx)
        model.setPropertyValue("Width", width)
        model.setPropertyValue("Height", height)
        model.setPropertyValue("Title", title)
        model.setPropertyValue("BackgroundColor", theme.PANE_BG)
        self.container.setModel(model)
        self.model = model
        # None = cua so doc lap, khong bi cua so tai lieu che (do la nguyen nhan khong bam duoc cua so con).
        self.container.createPeer(toolkit(ctx), None)
        self.peer_ready = True

    def add(self, service: str, name: str, **props):
        """Nhu ChildWindow.add: mac dinh font/mau cua pane cho dieu khien thong thuong."""
        from . import theme

        if service in ("FixedText", "Edit", "CheckBox", "Button", "ListBox", "RadioButton"):
            props.setdefault("FontName", theme.FONT)
            props.setdefault("FontHeight", theme.SIZE_SMALL)
        if service in ("FixedText", "CheckBox", "RadioButton"):
            props.setdefault("BackgroundColor", theme.PANE_BG)
            props.setdefault("TextColor", theme.TEXT_PRIMARY)
        if not props.get("FontName"):
            props.pop("FontName", None)
        return super().add(service, name, **props)

    def _insert(self, name: str, service: str, model) -> None:
        control = self.ctx.ServiceManager.createInstanceWithContext("com.sun.star.awt.UnoControl" + service, self.ctx)
        control.setModel(model)
        self.container.addControl(name, control)

    def control(self, name: str):
        return self.container.getControl(name)

    def window(self):
        return self.container

    def set_title(self, title: str) -> None:
        try:
            self.container.getPeer().setPropertyValue("Title", title)
        except Exception:  # noqa: BLE001 - ban khac khong co thuoc tinh nay
            pass

    def place(self, x: int, y: int, width: int, height: int) -> None:
        self.container.setPosSize(x, y, width, height, 15)
        self.container.setVisible(True)

    def close(self) -> None:
        try:
            self.container.setVisible(False)
            self.container.dispose()
        except Exception:  # noqa: BLE001
            pass

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
