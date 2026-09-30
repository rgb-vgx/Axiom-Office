"""Thanh phan giao dien cua pane tren awt (LibreOffice): bong bong bo goc, nhan bam duoc, nut.

awt khong ve duoc hinh bo goc hay tu dinh nghia control, nen moi thanh phan la mot nhom dieu khien co san:

* Hop bo goc = FixedText mau nen + 4 ImageControl goc (PNG ve san trong theme.py) + 4 duong vien 1px.
* Nut/chip/link = FixedText + XMouseListener (hover doi mau, bam = mouseReleased ben trong).

Nhung dieu rut ra khi thu tren LibreOffice 26.8 (xem LibreOffice_arch.md 14.3):
* Dieu khien them TRUOC nam TREN dieu khien them sau -> moi nhom them phan truoc (goc, chu) roi moi nen.
* Model khong nhan toa do -> dat vi tri bang setPosSize tren view (Widget.place).
* Chieu cao chu do bang XLayoutConstrains.calcAdjustedSize(Size(width, 0)) cua chinh FixedText.
"""
from __future__ import annotations

import itertools

import uno
import unohelper
from com.sun.star.awt import XFocusListener, XKeyListener, XMouseListener, XTextListener

from . import theme

_names = itertools.count(1)


def _font(model, name: str, size: float, weight: float = theme.WEIGHT_NORMAL) -> None:
    for key, value in (("FontName", name), ("FontHeight", size), ("FontWeight", weight)):
        if key == "FontName" and not value:
            continue
        try:
            model.setPropertyValue(key, value)
        except Exception:  # noqa: BLE001
            pass


def _valign_middle():
    return uno.Enum("com.sun.star.style.VerticalAlignment", "MIDDLE")


class Surface:
    """Mot UnoControlContainer da co peer: them/xoa dieu khien theo toa do pixel."""

    def __init__(self, ctx, container):
        self.ctx = ctx
        self.container = container
        self.sm = ctx.ServiceManager
        self._pointer = None

    def create(self, service: str, **props):
        control = self.sm.createInstanceWithContext("com.sun.star.awt.UnoControl" + service, self.ctx)
        model = self.sm.createInstanceWithContext("com.sun.star.awt.UnoControl" + service + "Model", self.ctx)
        font = props.pop("font", None)
        for key, value in props.items():
            try:
                model.setPropertyValue(key, value)
            except Exception:  # noqa: BLE001 - thuoc tinh khong co o ban nay
                pass
        if font:
            _font(model, *font)
        control.setModel(model)
        self.container.addControl("w%d" % next(_names), control)
        return control

    def remove(self, control) -> None:
        try:
            self.container.removeControl(control)
        except Exception:  # noqa: BLE001
            pass
        try:
            control.dispose()
        except Exception:  # noqa: BLE001
            pass

    def hand_pointer(self):
        if self._pointer is None:
            try:
                pointer = self.sm.createInstanceWithContext("com.sun.star.awt.Pointer", self.ctx)
                pointer.setType(uno.getConstantByName("com.sun.star.awt.SystemPointer.REFHAND"))
                self._pointer = pointer
            except Exception:  # noqa: BLE001
                self._pointer = False
        return self._pointer or None


def text_height(control, width: int, minimum: int = 16) -> int:
    """Chieu cao can de hien het chu cua FixedText khi xuong dong o be rong width."""
    try:
        size = uno.createUnoStruct("com.sun.star.awt.Size")
        size.Width, size.Height = max(20, width), 0
        return max(minimum, int(control.calcAdjustedSize(size).Height))
    except Exception:  # noqa: BLE001
        return minimum


def text_width(control, minimum: int = 10) -> int:
    try:
        return max(minimum, int(control.getPreferredSize().Width))
    except Exception:  # noqa: BLE001
        return minimum


class Widget:
    """Nhom dieu khien di chuyen cung nhau; toa do cac phan tinh tu goc (x, y) cua nhom."""

    def __init__(self, surface: Surface):
        self.surface = surface
        self.parts: list[list] = []      # [control, dx, dy, w, h]
        self.x = self.y = 0
        self.width = self.height = 0
        self.visible = True

    def add(self, service: str, dx: int, dy: int, w: int, h: int, **props):
        control = self.surface.create(service, **props)
        self.parts.append([control, dx, dy, w, h])
        return control

    def resize_part(self, control, dx: int = None, dy: int = None, w: int = None, h: int = None) -> None:
        for part in self.parts:
            if part[0] is control:
                for index, value in ((1, dx), (2, dy), (3, w), (4, h)):
                    if value is not None:
                        part[index] = value
                self._apply(part)
                return

    def place(self, x: int, y: int) -> None:
        self.x, self.y = x, y
        for part in self.parts:
            self._apply(part)

    def _apply(self, part) -> None:
        control, dx, dy, w, h = part
        try:
            control.setPosSize(self.x + dx, self.y + dy, max(0, w), max(0, h), 15)
        except Exception:  # noqa: BLE001
            pass

    def set_visible(self, visible: bool) -> None:
        if visible == self.visible:
            return
        self.visible = visible
        for part in self.parts:
            try:
                part[0].setVisible(visible)
            except Exception:  # noqa: BLE001
                pass

    def dispose(self) -> None:
        for part in self.parts:
            self.surface.remove(part[0])
        self.parts = []


# ---------------------------------------------------------------- hop bo goc

class RoundBox:
    """Nen bo goc trong mot Widget: goi add_corners() TRUOC khi them noi dung, add_background() SAU cung."""

    def __init__(self, widget: Widget, width: int, height: int, radius: int, fill: int, border: int | None = None,
                 outside: int = theme.PANE_BG):
        self.widget = widget
        self.width, self.height, self.radius = width, height, radius
        self.fill, self.border, self.outside = fill, border, outside
        self.corners = {}
        self.edges = []
        self.backgrounds = []

    def _background_rects(self):
        """Nen hinh chu thap: KHONG chong len goc/vien (khi nen ve lai - vd doi mau hover - VCL co luc ve de len
        dieu khien chong phia tren; tach rieng thi khong con gi de de)."""
        r, w, h = self.radius, self.width, self.height
        inset = 1 if self.border is not None else 0
        return ((r, inset, w - 2 * r, h - 2 * inset), (inset, r, w - 2 * inset, h - 2 * r))

    def _corner_url(self, which: str) -> str:
        path = theme.cached_png("corner", theme.corner_png, self.radius, which, self.fill, self.outside, self.border)
        return uno.systemPathToFileUrl(path)

    def _corner_rects(self):
        r, w, h = self.radius, self.width, self.height
        return {"tl": (0, 0), "tr": (w - r, 0), "bl": (0, h - r), "br": (w - r, h - r)}

    def add_corners(self) -> None:
        r = self.radius
        for which, (dx, dy) in self._corner_rects().items():
            self.corners[which] = self.widget.add("ImageControl", dx, dy, r, r, ImageURL=self._corner_url(which),
                                                  Border=0, ScaleImage=False, Tabstop=False)
        if self.border is not None:
            w, h = self.width, self.height
            for dx, dy, ew, eh in ((r, 0, w - 2 * r, 1), (r, h - 1, w - 2 * r, 1), (0, r, 1, h - 2 * r), (w - 1, r, 1, h - 2 * r)):
                self.edges.append(self.widget.add("FixedText", dx, dy, ew, eh, Label="", BackgroundColor=self.border, Border=0))

    def add_background(self) -> None:
        for dx, dy, w, h in self._background_rects():
            self.backgrounds.append(self.widget.add("FixedText", dx, dy, w, h, Label="", BackgroundColor=self.fill,
                                                    Border=0, Tabstop=False))

    @property
    def background(self):
        return self.backgrounds[0] if self.backgrounds else None

    def recolor(self, fill: int, border: int | None = None) -> None:
        """Doi mau nen/vien (hover nut, o soan co focus): doi anh goc + mau nen."""
        if fill == self.fill and border == self.border:
            return
        self.fill, self.border = fill, border if border is not None else self.border
        for which, control in self.corners.items():
            try:
                control.getModel().setPropertyValue("ImageURL", self._corner_url(which))
            except Exception:  # noqa: BLE001
                pass
        for edge in self.edges:
            try:
                edge.getModel().setPropertyValue("BackgroundColor", self.border)
            except Exception:  # noqa: BLE001
                pass
        for background in self.backgrounds:
            try:
                background.getModel().setPropertyValue("BackgroundColor", fill)
            except Exception:  # noqa: BLE001
                pass

    def resize(self, width: int, height: int) -> None:
        self.width, self.height = width, height
        r = self.radius
        for which, (dx, dy) in self._corner_rects().items():
            if which in self.corners:
                self.widget.resize_part(self.corners[which], dx, dy)
        if self.edges:
            rects = ((r, 0, width - 2 * r, 1), (r, height - 1, width - 2 * r, 1), (0, r, 1, height - 2 * r),
                     (width - 1, r, 1, height - 2 * r))
            for edge, (dx, dy, ew, eh) in zip(self.edges, rects):
                self.widget.resize_part(edge, dx, dy, ew, eh)
        for background, (dx, dy, w, h) in zip(self.backgrounds, self._background_rects()):
            self.widget.resize_part(background, dx, dy, w, h)


# ---------------------------------------------------------------- tuong tac

class MouseListener(unohelper.Base, XMouseListener):
    """Hover + bam cho nhan/nut tu ve (FixedText khong co su kien click).

    Kich hoat khi NHAN chuot trai: panel trong sidebar cua LibreOffice (26.8) khong nhan duoc mouseReleased
    - chi co mousePressed - nen "bam = nha chuot" se khong bao gio chay o do.
    """

    LEFT = 1   # com.sun.star.awt.MouseButton.LEFT

    def __init__(self, on_click, on_hover=None):
        self._on_click = on_click
        self._on_hover = on_hover

    def mousePressed(self, event):  # noqa: N802 - ten UNO
        if not (event.Buttons & self.LEFT):
            return
        try:
            self._on_click()
        except Exception as exc:  # noqa: BLE001 - loi trong handler khong duoc lam sap LibreOffice
            from . import log

            log.error("pane click failed: %s" % exc)

    def mouseReleased(self, event):  # noqa: N802
        pass

    def mouseEntered(self, event):  # noqa: N802
        if self._on_hover:
            self._on_hover(True)

    def mouseExited(self, event):  # noqa: N802
        if self._on_hover:
            self._on_hover(False)

    def disposing(self, event):
        pass


def make_clickable(surface: Surface, controls, on_click, on_hover=None) -> None:
    listener = MouseListener(on_click, on_hover)
    pointer = surface.hand_pointer()
    for control in controls:
        try:
            control.addMouseListener(listener)
        except Exception:  # noqa: BLE001
            continue
        if pointer is not None:
            try:
                control.getPeer().setPointer(pointer)
            except Exception:  # noqa: BLE001
                pass


class KeyListener(unohelper.Base, XKeyListener):
    def __init__(self, on_pressed, on_released=None):
        self._on_pressed = on_pressed
        self._on_released = on_released

    def keyPressed(self, event):  # noqa: N802
        self._on_pressed(event)

    def keyReleased(self, event):  # noqa: N802
        if self._on_released:
            self._on_released(event)

    def disposing(self, event):
        pass


class FocusListener(unohelper.Base, XFocusListener):
    def __init__(self, handler):
        self._handler = handler

    def focusGained(self, event):  # noqa: N802
        self._handler(True)

    def focusLost(self, event):  # noqa: N802
        self._handler(False)

    def disposing(self, event):
        pass


class TextListener(unohelper.Base, XTextListener):
    def __init__(self, handler):
        self._handler = handler

    def textChanged(self, event):  # noqa: N802
        self._handler()

    def disposing(self, event):
        pass


def set_prop(control, name: str, value) -> None:
    try:
        control.getModel().setPropertyValue(name, value)
    except Exception:  # noqa: BLE001
        pass


# ---------------------------------------------------------------- thanh phan dung san

def link(surface: Surface, text: str, on_click, color: int = theme.ACCENT_FG, hover: int = theme.ACTION_HOVER,
         size: float = theme.SIZE_CAPTION, bold: bool = False) -> Widget:
    """Link chu (Tro chuyen moi, Cai dat, Dung...): mau accent, dam mau hon khi hover."""
    widget = Widget(surface)
    label = widget.add("FixedText", 0, 0, 10, 18, Label=text, TextColor=color, BackgroundColor=theme.PANE_BG, Border=0,
                       VerticalAlign=_valign_middle(), NoLabel=True,
                       font=(theme.FONT_SEMIBOLD if bold else theme.FONT, size))
    width = text_width(label) + 2
    widget.resize_part(label, w=width)
    widget.width, widget.height = width, 18
    widget.label = label
    make_clickable(surface, [label], on_click, lambda inside: set_prop(label, "TextColor", hover if inside else color))
    return widget


def button(surface: Surface, text: str, width: int, height: int, on_click, primary: bool = True,
           radius: int = theme.RADIUS_CONTROL) -> Widget:
    """Nut bo goc: primary = nen indigo chu trang (nut Gui), khong primary = vien chip."""
    widget = Widget(surface)
    fill = theme.ACTION_BG if primary else theme.PANE_BG
    border = None if primary else theme.CHIP_BORDER
    box = RoundBox(widget, width, height, radius, fill, border)
    box.add_corners()
    label = widget.add("FixedText", radius // 2, 1, width - radius, height - 2, Label=text, Align=1,
                       VerticalAlign=_valign_middle(), BackgroundColor=fill,
                       TextColor=theme.ON_ACTION if primary else theme.CHIP_FG, Border=0, NoLabel=True,
                       font=(theme.FONT_SEMIBOLD, theme.SIZE_SMALL))
    box.add_background()
    widget.width, widget.height = width, height
    widget.box, widget.label = box, label
    widget.enabled = True
    widget.primary = primary

    def paint(hover: bool) -> None:
        if not widget.enabled:
            box.recolor(theme.DISABLED_BG)
            set_prop(label, "BackgroundColor", theme.DISABLED_BG)
            set_prop(label, "TextColor", theme.DISABLED_FG)
            return
        if primary:
            color = theme.ACTION_HOVER if hover else theme.ACTION_BG
            text_color = theme.ON_ACTION
        else:
            color = theme.CHIP_HOVER if hover else theme.PANE_BG
            text_color = theme.CHIP_FG
        box.recolor(color)
        set_prop(label, "BackgroundColor", color)
        set_prop(label, "TextColor", text_color)

    def set_enabled(enabled: bool) -> None:
        widget.enabled = enabled
        paint(False)

    def click() -> None:
        if widget.enabled:
            on_click()

    widget.set_enabled = set_enabled
    make_clickable(surface, [label] + box.backgrounds + list(box.corners.values()), click,
                   lambda inside: paint(inside and widget.enabled))
    return widget


def dot(surface: Surface, kind: str = "dot", size: int = 8, background: int = theme.PANE_BG) -> Widget:
    widget = Widget(surface)
    path = theme.cached_png("icon", theme.icon_png, kind, size, background)
    widget.image = widget.add("ImageControl", 0, 0, size, size, ImageURL=uno.systemPathToFileUrl(path), Border=0,
                              ScaleImage=False, BackgroundColor=background, Tabstop=False)
    widget.width = widget.height = size
    return widget


def set_icon(control, kind: str, size: int, background: int = theme.PANE_BG) -> None:
    path = theme.cached_png("icon", theme.icon_png, kind, size, background)
    set_prop(control, "ImageURL", uno.systemPathToFileUrl(path))
