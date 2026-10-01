"""Vung hoi thoai cua pane: danh sach muc cuon duoc (ban awt cua ChatList + ChatBubble/ToolLine/... o
src/AxiomOffice/Ai/PaneControls.cs).

Vung nay la mot UnoControlContainer con (tu cat phan tran ra ngoai) + mot ScrollBar doc. Moi muc cua
`chat.ChatSession.items` duoc dung thanh mot Widget; cuon = dat lai toa do y cua cac Widget. Muc da ve
duoc giu lai theo (index, rev): chi ve lai muc moi hoac muc vua doi (the xac nhan da tra loi...).
"""
from __future__ import annotations

import unohelper
from com.sun.star.awt import XAdjustmentListener

from . import theme
from .widgets import RoundBox, Surface, Widget, button, link, set_icon, text_height, _valign_middle

LINE_KINDS = ("tool", "skill", "info")


class _Adjust(unohelper.Base, XAdjustmentListener):
    def __init__(self, handler):
        self._handler = handler

    def adjustmentValueChanged(self, event):  # noqa: N802
        self._handler(int(event.Value))

    def disposing(self, event):
        pass


class ChatView:
    def __init__(self, ctx, parent: Surface, callbacks: dict):
        """callbacks: suggestion(text), confirm(item, approved), forget(item), settings()."""
        self.ctx = ctx
        self.parent = parent
        self.callbacks = callbacks
        self.viewport = parent.create("Container", BackgroundColor=theme.PANE_BG)
        self.surface = Surface(ctx, self.viewport)
        self.scrollbar = parent.create("ScrollBar", Orientation=1, LineIncrement=24, BlockIncrement=120, LiveScroll=True,
                                       ScrollValueMax=0, VisibleSize=1)
        self.scrollbar.addAdjustmentListener(_Adjust(self._on_scroll))
        self.rect = (0, 0, 300, 300)
        self.content_width = 260
        self.rendered: list[tuple[int, Widget]] = []   # (rev, widget) theo chi so item
        self.empty: Widget | None = None
        self.typing: Widget | None = None
        self.offset = 0
        self.total = 0
        self.kind = "wps"

    # ---------------------------------------------------------------- vi tri

    def set_rect(self, x: int, y: int, width: int, height: int, session) -> None:
        width_changed = width != self.rect[2]
        self.rect = (x, y, width, height)
        # Khung chat chua cot thanh cuon: hai dieu khien khong duoc chong nhau (cai them truoc se che cai sau).
        self.viewport.setPosSize(x, y, width - theme.SCROLL_W, height, 15)
        self.scrollbar.setPosSize(x + width - theme.SCROLL_W, y, theme.SCROLL_W, height, 15)
        self.content_width = max(160, width - theme.SCROLL_W - theme.PAD_X - 6)
        if width_changed:
            self.clear()
        self.sync(session, stick=True)

    def clear(self) -> None:
        for _, widget in self.rendered:
            widget.dispose()
        self.rendered = []
        for widget in (self.empty, self.typing):
            if widget is not None:
                widget.dispose()
        self.empty = self.typing = None

    # ---------------------------------------------------------------- dong bo voi session

    def sync(self, session, stick: bool = None) -> None:
        """Ve cac muc moi/doi, roi xep lai va cuon xuong cuoi neu dang o cuoi (hoac stick=True)."""
        at_bottom = self.offset >= self.total - self.rect[3] - 4
        # Giu lai de link "Hien/An" tren khoi suy luan bam nguoc duoc vao session (xem _toggle_reasoning).
        self.session = session
        items = session.items
        if len(items) < len(self.rendered):          # Tro chuyen moi
            self.clear()
        for index, item in enumerate(items):
            if index < len(self.rendered):
                rev, widget = self.rendered[index]
                if rev == item.get("rev", 0):
                    continue
                widget.dispose()
                self.rendered[index] = (item.get("rev", 0), self._build(item))
            else:
                self.rendered.append((item.get("rev", 0), self._build(item)))

        if not items and self.empty is None:
            self.empty = self._empty_state(session.kind)
        elif items and self.empty is not None:
            self.empty.dispose()
            self.empty = None

        if session.thinking and self.typing is None:
            self.typing = self._typing()
        elif not session.thinking and self.typing is not None:
            self.typing.dispose()
            self.typing = None

        self._layout(stick if stick is not None else at_bottom)

    def _layout(self, to_bottom: bool) -> None:
        y = theme.PAD_X
        previous = None
        blocks = []
        if self.empty is not None:
            blocks.append((None, self.empty))
        for index, (_, widget) in enumerate(self.rendered):
            blocks.append((widget.kind, widget))
        if self.typing is not None:
            blocks.append(("typing", self.typing))
        positions = []
        for kind, widget in blocks:
            if previous is not None:
                if kind in LINE_KINDS and previous in LINE_KINDS:
                    y += theme.GAP_TOOL_LINE
                elif kind in LINE_KINDS:
                    y += theme.GAP_BEFORE_TOOLS
                else:
                    y += theme.GAP_MESSAGE
            positions.append((widget, y))
            y += widget.height
            previous = kind or "empty"
        self.total = y + theme.PAD_X
        view_h = self.rect[3]
        max_offset = max(0, self.total - view_h)
        self.offset = max_offset if to_bottom else min(self.offset, max_offset)
        self._positions = positions
        self._place()
        self._update_scrollbar()

    def _place(self) -> None:
        view_h = self.rect[3]
        for widget, y in getattr(self, "_positions", []):
            top = y - self.offset
            x = theme.PAD_X + getattr(widget, "indent", 0)
            widget.place(x, top)
            widget.set_visible(top + widget.height > -40 and top < view_h + 40)

    def _update_scrollbar(self) -> None:
        view_h = self.rect[3]
        needed = self.total > view_h
        try:
            model = self.scrollbar.getModel()
            model.setPropertyValue("ScrollValueMax", max(1, self.total))
            model.setPropertyValue("VisibleSize", max(1, view_h))
            model.setPropertyValue("BlockIncrement", max(24, view_h - 40))
            self.scrollbar.setValue(self.offset)
            self.scrollbar.setVisible(needed)
        except Exception:  # noqa: BLE001
            pass

    def _on_scroll(self, value: int) -> None:
        self.offset = max(0, min(value, max(0, self.total - self.rect[3])))
        self._place()

    def scroll_by(self, delta: int) -> None:
        self._on_scroll(self.offset + delta)
        try:
            self.scrollbar.setValue(self.offset)
        except Exception:  # noqa: BLE001
            pass

    # ---------------------------------------------------------------- dung tung loai muc

    def _build(self, item: dict) -> Widget:
        builder = {
            "user": self._user,
            "ai": self._ai,
            "tool": self._tool,
            "skill": self._skill,
            "info": self._info,
            "memory": self._memory,
            "confirm": self._confirm,
            "reasoning": self._reasoning,
            "error": self._error,
        }.get(item["kind"], self._info)
        widget = builder(item)
        widget.kind = item["kind"]
        return widget

    def _bubble(self, text: str, user: bool) -> Widget:
        """Bong bong chat: nguoi dung = nen indigo nhat, ben phai; AI = xam nhat co vien, ben trai."""
        widget = Widget(self.surface)
        max_w = int(self.content_width * (theme.BUBBLE_MAX_USER if user else theme.BUBBLE_MAX_AI))
        fill = theme.USER_BG if user else theme.AI_BG
        pad_x, pad_y = theme.BUBBLE_PAD_X, theme.BUBBLE_PAD_Y
        # Do chu truoc (tam o be rong toi da) de biet kich thuoc hop.
        probe = self.surface.create("FixedText", Label=text, MultiLine=True, Border=0, NoLabel=True,
                                    font=(theme.FONT, theme.SIZE_BODY))
        natural = 0
        try:
            natural = int(probe.getPreferredSize().Width) + 2
        except Exception:  # noqa: BLE001
            natural = max_w
        inner_w = max(24, min(max_w - 2 * pad_x, natural))
        inner_h = text_height(probe, inner_w, 18)
        self.surface.remove(probe)
        box_w, box_h = inner_w + 2 * pad_x, inner_h + 2 * pad_y
        box = RoundBox(widget, box_w, box_h, theme.RADIUS_BUBBLE, fill, None if user else theme.AI_BORDER)
        box.add_corners()
        widget.add("FixedText", pad_x, pad_y, inner_w, inner_h, Label=text, MultiLine=True, BackgroundColor=fill,
                   TextColor=theme.USER_FG if user else theme.TEXT_PRIMARY, Border=0, NoLabel=True,
                   font=(theme.FONT, theme.SIZE_BODY))
        box.add_background()
        widget.width, widget.height = box_w, box_h
        widget.indent = self.content_width - box_w if user else 0
        return widget

    def _user(self, item: dict) -> Widget:
        return self._bubble(item.get("text", ""), True)

    def _ai(self, item: dict) -> Widget:
        return self._bubble(theme.plain_reply(item.get("text", "")), False)

    def _line(self, icon: str, text: str, color: int, action_id: str = "") -> Widget:
        """Dong thao tac: icon tron 12px + nhan + ma lenh (mo) ben phai - giong ToolLine."""
        widget = Widget(self.surface)
        width = self.content_width
        id_w = 0
        id_control = None
        if action_id:
            id_control = widget.add("FixedText", 0, 0, 10, theme.TOOL_ROW_H, Label=action_id, TextColor=theme.TEXT_MUTED,
                                    BackgroundColor=theme.PANE_BG, Border=0, Align=2, VerticalAlign=_valign_middle(),
                                    NoLabel=True, font=(theme.FONT_MONO, theme.SIZE_TOOL_ID))
            try:
                # getPreferredSize thieu vai pixel voi font mono: lay them theo so ky tu cho chac.
                measured = max(int(id_control.getPreferredSize().Width) + 10, len(action_id) * 7 + 8)
                # Uu tien hien du ma lenh (font mono tren Linux rong hon Consolas); nhan ben trai tu xuong dong.
                id_w = min(measured, max(int(width * 0.45), width - theme.ICON - 8 - 90))
            except Exception:  # noqa: BLE001
                id_w = 120
        label_left = theme.ICON + 8
        label_w = max(40, width - label_left - id_w - 8)
        label = widget.add("FixedText", label_left, 0, label_w, theme.TOOL_ROW_H, Label=text, MultiLine=True,
                           TextColor=color, BackgroundColor=theme.PANE_BG, Border=0, NoLabel=True,
                           font=(theme.FONT, theme.SIZE_TOOL))
        label_h = max(theme.TOOL_ROW_H, text_height(label, label_w, 14) + 4)
        widget.resize_part(label, dy=3 if label_h > theme.TOOL_ROW_H else 3, h=label_h)
        image = widget.add("ImageControl", 2, (theme.TOOL_ROW_H - theme.ICON) // 2, theme.ICON, theme.ICON, Border=0,
                           ScaleImage=False, BackgroundColor=theme.PANE_BG, Tabstop=False)
        set_icon(image, icon, theme.ICON)
        if id_control is not None:
            widget.resize_part(id_control, dx=width - id_w - 2, w=id_w)   # chua 2px: glyph mono sat mep bi cat
        widget.width, widget.height = width, label_h
        return widget

    def _tool(self, item: dict) -> Widget:
        action = item.get("action") or ""
        if item.get("state") == "error":
            return self._line("error", "%s — %s" % (theme.label_for(action), theme.error_summary(item.get("error"))),
                              theme.DANGER_FG, action)
        return self._line("ok", theme.label_for(action), theme.TEXT_SECONDARY, action)

    def _skill(self, item: dict) -> Widget:
        return self._line("dot", "Dùng kỹ năng: %s" % item.get("name", "?"), theme.TEXT_SECONDARY)

    def _info(self, item: dict) -> Widget:
        return self._line("running", item.get("text", ""), theme.TEXT_MUTED)

    def _card(self, fill: int, border: int, title: str, title_color: int, message: str, links, strip: int = None) -> Widget:
        """The: nen mau nhat + tieu de dam + noi dung + dong link (Dong y/Tu choi, Xoa, Mo Cai dat)."""
        widget = Widget(self.surface)
        width = self.content_width
        pad = 10
        left = pad + (4 if strip is not None else 0)
        inner_w = width - left - pad
        title_label = widget.add("FixedText", left, pad, inner_w, 18, Label=title, MultiLine=True, TextColor=title_color,
                                 BackgroundColor=fill, Border=0, NoLabel=True, font=(theme.FONT_SEMIBOLD, theme.SIZE_SMALL))
        title_h = text_height(title_label, inner_w, 16)
        widget.resize_part(title_label, h=title_h)
        y = pad + title_h + 2
        if message:
            body = widget.add("FixedText", left, y, inner_w, 18, Label=message, MultiLine=True, TextColor=theme.TEXT_PRIMARY,
                              BackgroundColor=fill, Border=0, NoLabel=True, font=(theme.FONT, theme.SIZE_SMALL))
            body_h = text_height(body, inner_w, 16)
            widget.resize_part(body, h=body_h)
            y += body_h + 4
        link_widgets = []
        x = left
        for text, handler in links:
            item = link(self.surface, text, handler, bold=True, size=theme.SIZE_SMALL)
            for part in item.parts:
                part[0].getModel().setPropertyValue("BackgroundColor", fill)
                widget.parts.append([part[0], x + part[1], y + part[2], part[3], part[4]])
            x += item.width + 14
            link_widgets.append(item)
        if links:
            y += 20
        height = y + pad - 2
        box = RoundBox(widget, width, height, theme.RADIUS_CONTROL, fill, border)
        # Goc + vien phai nam tren nen nhung duoi chu: them sau chu la duoc vi khong chong len chu.
        box.add_corners()
        if strip is not None:
            widget.add("FixedText", 0, theme.RADIUS_CONTROL, 3, height - 2 * theme.RADIUS_CONTROL, Label="",
                       BackgroundColor=strip, Border=0)
        box.add_background()
        widget.width, widget.height = width, height
        return widget

    def _confirm(self, item: dict) -> Widget:
        state = item.get("state")
        label = theme.label_for(item.get("action"))
        detail = item.get("reason") or ""
        preview = item.get("preview") or ""
        if preview and preview != "{}":
            detail += ("\n" if detail else "") + preview
        if state == "pending":
            return self._card(theme.CHIP_HOVER, theme.CHIP_BORDER, "Cần bạn xác nhận: " + label, theme.ACCENT_FG, detail,
                              [("Đồng ý", lambda: self.callbacks["confirm"](item, True)),
                               ("Từ chối", lambda: self.callbacks["confirm"](item, False))])
        title, color = {
            "approved": ("Đã đồng ý: " + label, theme.SUCCESS),
            "rejected": ("Đã từ chối: " + label, theme.TEXT_SECONDARY),
            "timeout": ("Hết giờ chờ xác nhận — đã từ chối", theme.TEXT_SECONDARY),
            "cancelled": ("Đã dừng — không thực hiện: " + label, theme.TEXT_SECONDARY),
        }.get(state, ("Đã trả lời: " + label, theme.TEXT_SECONDARY))
        return self._card(theme.CHIP_HOVER, theme.CHIP_BORDER, title, color, "", [])

    def _reasoning(self, item: dict) -> Widget:
        """Khoi suy luan cua model: the mo, khong phai bong bong tra loi - de phan biet ro cai model
        dang NGHI voi cai no TRA LOI nguoi dung."""
        round_number = item.get("round")
        title = "Suy luận · vòng %s" % round_number if round_number else "Suy luận"
        expanded = item.get("expanded")
        body = item.get("text", "") if expanded else theme.reasoning_preview(item.get("text", ""))
        label = "Ẩn" if expanded else "Hiện"
        return self._card(theme.AI_BG, theme.CHIP_BORDER, title, theme.TEXT_MUTED, body,
                          [(label, lambda: self._toggle_reasoning(item))])

    def _toggle_reasoning(self, item: dict) -> None:
        session = getattr(self, "session", None)
        if session is not None:
            session.toggle_reasoning(item)   # tang rev -> sync ve lai dung muc nay

    def _memory(self, item: dict) -> Widget:
        state = item.get("state")
        if state == "deleted":
            return self._card(theme.CHIP_HOVER, theme.CHIP_BORDER, "Đã xoá ghi nhớ.", theme.TEXT_SECONDARY, "", [])
        links = [] if state == "deleting" else [("Xoá", lambda: self.callbacks["forget"](item))]
        title = "Đã ghi nhớ" + (" (không xoá được)" if state == "failed" else "")
        return self._card(theme.CHIP_HOVER, theme.CHIP_BORDER, title, theme.ACCENT_FG, item.get("text", ""), links)

    def _error(self, item: dict) -> Widget:
        return self._card(theme.DANGER_BG, theme.DANGER_BORDER, item.get("title") or "Có lỗi xảy ra", theme.DANGER_FG,
                          item.get("message") or "", [("Mở Cài đặt", self.callbacks["settings"])], strip=theme.DANGER)

    def _typing(self) -> Widget:
        """Bong bong 'dang suy nghi' (ba cham mau accent) cho den khi co tra loi."""
        widget = Widget(self.surface)
        width, height = 56, 32
        box = RoundBox(widget, width, height, theme.RADIUS_BUBBLE, theme.AI_BG, theme.AI_BORDER)
        box.add_corners()
        widget.dots = widget.add("FixedText", 8, 4, width - 16, height - 8, Label="• • •", Align=1,
                                 VerticalAlign=_valign_middle(), TextColor=theme.ACCENT, BackgroundColor=theme.AI_BG,
                                 Border=0, NoLabel=True, font=(theme.FONT_SEMIBOLD, 11.0))
        box.add_background()
        widget.width, widget.height = width, height
        return widget

    def _empty_state(self, kind: str) -> Widget:
        """Man hinh dau: tieu de + mo ta + GOI Y (chip bam de dien vao o soan)."""
        title, description, suggestions = theme.empty_state(kind)
        widget = Widget(self.surface)
        width = self.content_width
        y = 4
        title_label = widget.add("FixedText", 0, y, width, 20, Label=title, MultiLine=True, TextColor=theme.TEXT_PRIMARY,
                                 BackgroundColor=theme.PANE_BG, Border=0, NoLabel=True,
                                 font=(theme.FONT_SEMIBOLD, theme.SIZE_EMPTY_TITLE))
        h = text_height(title_label, width, 20)
        widget.resize_part(title_label, h=h)
        y += h + 4
        desc = widget.add("FixedText", 0, y, width, 20, Label=description, MultiLine=True, TextColor=theme.TEXT_SECONDARY,
                          BackgroundColor=theme.PANE_BG, Border=0, NoLabel=True, font=(theme.FONT, theme.SIZE_BODY))
        h = text_height(desc, width, 18)
        widget.resize_part(desc, h=h)
        y += h + 16
        widget.add("FixedText", 0, y, width, 16, Label="GỢI Ý", TextColor=theme.TEXT_MUTED, BackgroundColor=theme.PANE_BG,
                   Border=0, NoLabel=True, font=(theme.FONT_SEMIBOLD, theme.SIZE_CAPTION))
        y += 16 + 8
        for text in suggestions:
            chip = button(self.surface, text, width, theme.CHIP_H,
                          (lambda value=text: self.callbacks["suggestion"](value)), primary=False)
            set_prop_all(chip, "Align", 0)
            for part in chip.parts:
                widget.parts.append([part[0], part[1], y + part[2], part[3], part[4]])
            chip_label = chip.label
            for part in widget.parts:
                if part[0] is chip_label:
                    part[1] = 12
                    part[3] = width - 24
            y += theme.CHIP_H + theme.GAP_CHIP
        widget.width, widget.height = width, y
        return widget


def set_prop_all(widget: Widget, name: str, value) -> None:
    try:
        widget.label.getModel().setPropertyValue(name, value)
    except Exception:  # noqa: BLE001
        pass
