"""He thiet ke cua pane Ask AI - ban LibreOffice cua PaneTheme (src/AxiomOffice/Ai/PaneControls.cs).

Cung mau, co chu, khoang cach voi pane cua MS Office/WPS de hai ban trong nhu mot san pham. Module thuan
Python (khong import uno): mau/kich thuoc, nhan tieng Viet cho tung lenh, goi y theo loai app va ve cac PNG
nho (goc bo tron, icon trang thai) bang zlib - awt cua LibreOffice khong ve duoc hinh bo goc, nen bong bong
chat = nen mau + 4 anh goc (xem widgets.py).
"""
from __future__ import annotations

import hashlib
import os
import struct
import sys
import zlib

IS_WINDOWS = sys.platform.startswith("win")

# ---------------------------------------------------------------- mau (giong PaneTheme)
PANE_BG = 0xFFFFFF
DIVIDER = 0xE5E7EB
TEXT_PRIMARY = 0x111827
TEXT_SECONDARY = 0x4B5563
TEXT_MUTED = 0x6B7280
AI_BG = 0xF9FAFB
AI_BORDER = 0xE5E7EB
USER_BG = 0xE0E7FF
USER_FG = 0x1E1B4B
ACCENT = 0x6366F1          # chi de trang tri (trang tren nen nay chi dat 4.47:1)
ACCENT_FG = 0x4F46E5
ACTION_BG = 0x4F46E5
ACTION_HOVER = 0x4338CA
ON_ACTION = 0xFFFFFF
DISABLED_BG = 0xF3F4F6
DISABLED_FG = 0x9CA3AF
INPUT_BORDER = 0x8C93A0
FOCUS = 0x4F46E5
CHIP_BORDER = 0xC7D2FE
CHIP_HOVER = 0xEEF2FF
CHIP_FG = 0x4338CA
DANGER = 0xDC2626
DANGER_FG = 0xB91C1C
DANGER_BG = 0xFEF2F2
DANGER_BORDER = 0xFECACA
SUCCESS = 0x047857

# ---------------------------------------------------------------- chu (diem; font UI cua he dieu hanh)
FONT = "Segoe UI" if IS_WINDOWS else ""
FONT_SEMIBOLD = "Segoe UI Semibold" if IS_WINDOWS else ""
FONT_MONO = "Consolas" if IS_WINDOWS else "Monospace"
SIZE_TITLE = 10.0
SIZE_EMPTY_TITLE = 11.0
SIZE_BODY = 9.5
SIZE_SMALL = 9.0
SIZE_TOOL = 8.5
SIZE_TOOL_ID = 8.0
SIZE_CAPTION = 8.25
WEIGHT_NORMAL = 100.0
WEIGHT_SEMIBOLD = 110.0     # com.sun.star.awt.FontWeight.SEMIBOLD (dung khi khong co font Semibold rieng)

# ---------------------------------------------------------------- kich thuoc (pixel o 96 DPI)
PAD_X = 12
GAP_MESSAGE = 12
GAP_BEFORE_TOOLS = 8
GAP_TOOL_LINE = 2
GAP_CHIP = 6
RADIUS_BUBBLE = 10
RADIUS_CONTROL = 8
RADIUS_COMPOSER = 10
BUBBLE_PAD_X = 12
BUBBLE_PAD_Y = 8
BUBBLE_MAX_USER = 0.80
BUBBLE_MAX_AI = 0.92
HEADER_H = 48
FOOTER_H = 28
COMPOSER_H = 98
SEND_W = 64
SEND_H = 30
CHIP_H = 32
TOOL_ROW_H = 20
ICON = 12
SCROLL_W = 12

PROMPT_PLACEHOLDER = "Nhập yêu cầu cho tài liệu này…"
PROMPT_HINT = "Enter để gửi · Shift+Enter xuống dòng"
PROMPT_HINT_SHORT = "Enter để gửi"

# ---------------------------------------------------------------- nhan tieng Viet (giong ToolLine.Labels)
ACTION_LABELS = {
    "writer.getText": "Đọc nội dung tài liệu",
    "writer.selection": "Đọc vùng chọn",
    "writer.newDocument": "Tạo tài liệu mới",
    "writer.open": "Mở tài liệu",
    "writer.typeText": "Gõ văn bản",
    "writer.appendText": "Viết thêm nội dung",
    "writer.replaceAll": "Thay thế văn bản",
    "writer.insertStyledText": "Chèn đoạn văn",
    "writer.formatSelection": "Định dạng vùng chọn",
    "writer.setParagraphAlignment": "Căn lề đoạn văn",
    "writer.insertTable": "Chèn bảng",
    "writer.insertPageBreak": "Ngắt trang",
    "writer.insertImage": "Chèn ảnh",
    "writer.insertHyperlink": "Chèn liên kết",
    "writer.heading": "Thêm tiêu đề",
    "writer.closeAll": "Đóng tài liệu",
    "writer.formatTable": "Định dạng bảng",
    "writer.checkTables": "Kiểm tra bảng",
    "et.listSheets": "Đọc danh sách sheet",
    "et.newWorkbook": "Tạo bảng tính mới",
    "et.open": "Mở bảng tính",
    "et.readRange": "Đọc vùng dữ liệu",
    "et.writeRange": "Ghi dữ liệu vào ô",
    "et.formatRange": "Định dạng ô",
    "et.activateSheet": "Chuyển sheet",
    "et.checkRange": "Kiểm tra bảng dữ liệu",
    "wpp.listSlides": "Đọc danh sách slide",
    "wpp.newPresentation": "Tạo bản trình chiếu mới",
    "wpp.open": "Mở bản trình chiếu",
    "wpp.addSlide": "Thêm slide",
    "wpp.addTextBox": "Thêm hộp văn bản",
    "wpp.addText": "Thêm chữ vào slide",
    "wpp.addImage": "Chèn ảnh vào slide",
    "wpp.addTable": "Thêm bảng vào slide",
    "wpp.setNotes": "Ghi chú thuyết trình",
    "wpp.deleteSlide": "Xoá slide",
    "wpp.checkLayout": "Kiểm tra bố cục slide",
}
SUFFIX_LABELS = {"save": "Lưu tệp", "saveAs": "Lưu tệp", "exportPdf": "Xuất PDF", "undo": "Hoàn tác thao tác trước"}
TOOL_LABELS = {
    "load_skill": "Dùng kỹ năng",
    "read_skill_file": "Đọc tài liệu kỹ năng",
    "remember": "Ghi nhớ",
    "recall": "Tra ghi nhớ",
    "look_at_document": "Xem ảnh tài liệu",
}


def label_for(action: str | None) -> str:
    """Nhan de doc cho mot lenh bridge / tool cua agent (khong ro thi 'Thao tac tren tai lieu')."""
    if not action:
        return "Thao tác trên tài liệu"
    if action in ACTION_LABELS:
        return ACTION_LABELS[action]
    if action in TOOL_LABELS:
        return TOOL_LABELS[action]
    suffix = action.rsplit(".", 1)[-1]
    if "." in action and suffix in SUFFIX_LABELS:
        return SUFFIX_LABELS[suffix]
    return "Thao tác trên tài liệu" if "." in action else action


NOUNS = {"wps": "văn bản", "et": "bảng tính", "wpp": "bản trình chiếu"}
APP_NAMES = {"wps": "Writer", "et": "Calc", "wpp": "Impress"}
SUGGESTIONS = {
    "wps": ("Soạn đơn xin việc vị trí kế toán", "Tóm tắt tài liệu này thành 5 ý", "Chèn bảng lịch họp tuần 3 cột"),
    "et": ("Tạo bảng điểm 5 học sinh, có cột trung bình", "Thêm cột Tổng có công thức cho bảng này",
           "Tô đậm và kẻ khung dòng tiêu đề"),
    "wpp": ("Tạo 5 slide giới thiệu công ty", "Thêm slide kế hoạch quý 4", "Viết ghi chú thuyết trình cho từng slide"),
}


def empty_state(kind: str) -> tuple[str, str, tuple]:
    """(tieu de, mo ta, goi y) cho man hinh trong - giong AskAiPane.AddGreeting."""
    return ("Bạn muốn làm gì với %s này?" % NOUNS.get(kind, "tài liệu"),
            "Tôi đọc và chỉnh sửa trực tiếp tài liệu đang mở. Thay đổi hiện ngay trên trang khi tôi làm.",
            SUGGESTIONS.get(kind, SUGGESTIONS["wps"]))


def plain_reply(text: str) -> str:
    """Tra loi cua model hien bang nhan thuong (khong co markdown): bo ** va ` de khoi lo ky hieu."""
    return (text or "").replace("**", "").replace("`", "").strip()


# ---------------------------------------------------------------- PNG

def rgb(color: int) -> tuple[int, int, int]:
    return (color >> 16) & 0xFF, (color >> 8) & 0xFF, color & 0xFF


def _png(width: int, height: int, pixel) -> bytes:
    """PNG RGB 8-bit; pixel(x, y) -> (r, g, b)."""
    rows = bytearray()
    for y in range(height):
        rows.append(0)
        for x in range(width):
            rows += bytes(pixel(x, y))

    def chunk(kind: bytes, payload: bytes) -> bytes:
        body = kind + payload
        return struct.pack(">I", len(payload)) + body + struct.pack(">I", zlib.crc32(body) & 0xFFFFFFFF)

    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(bytes(rows), 9)) + chunk(b"IEND", b""))


def _mix(a, b, t: float):
    return tuple(int(round(a[i] * t + b[i] * (1 - t))) for i in range(3))


SAMPLES = 4


def corner_png(radius: int, which: str, fill: int, outside: int, border: int | None = None) -> bytes:
    """Mot goc bo tron radius x radius: 'tl'/'tr'/'bl'/'br'. Vien 1px (neu co) ve theo cung tron.

    Khu rang cua bang sieu mau 4x4. Ngoai cung tron = mau nen pane (awt khong co trong suot that).
    """
    cx = radius if which in ("tl", "bl") else 0.0
    cy = radius if which in ("tl", "tr") else 0.0
    fill_c, outside_c = rgb(fill), rgb(outside)
    border_c = rgb(border) if border is not None else None

    def pixel(x, y):
        inside = ring = 0
        for sy in range(SAMPLES):
            for sx in range(SAMPLES):
                px, py = x + (sx + .5) / SAMPLES, y + (sy + .5) / SAMPLES
                dist = ((px - cx) ** 2 + (py - cy) ** 2) ** 0.5
                if dist <= radius:
                    inside += 1
                    if border_c is not None and dist > radius - 1.0:
                        ring += 1
        total = SAMPLES * SAMPLES
        body = _mix(fill_c, outside_c, inside / total)
        if border_c is None or not ring:
            return body
        return _mix(border_c, body, ring / total)

    return _png(radius, radius, pixel)


def icon_png(kind: str, size: int = ICON, background: int = PANE_BG) -> bytes:
    """Icon trang thai tron: 'ok' (xanh, dau check), 'error' (do, dau x), 'running' (vong xam), 'dot' (cham nhan)."""
    bg = rgb(background)
    r = size / 2.0
    color = {"ok": rgb(SUCCESS), "error": rgb(DANGER), "running": rgb(TEXT_MUTED), "dot": rgb(ACCENT)}.get(kind, rgb(TEXT_MUTED))
    white = (255, 255, 255)

    def on_mark(px, py) -> bool:
        # dau check / dau x ve bang doan thang day ~1.4px trong he toa do 0..1
        u, v = px / size, py / size

        def near(ax, ay, bx, by, width=0.11):
            dx, dy = bx - ax, by - ay
            t = max(0.0, min(1.0, ((u - ax) * dx + (v - ay) * dy) / (dx * dx + dy * dy)))
            return ((u - ax - t * dx) ** 2 + (v - ay - t * dy) ** 2) ** 0.5 <= width / 2 * 1.4

        if kind == "ok":
            return near(0.27, 0.52, 0.44, 0.68) or near(0.44, 0.68, 0.74, 0.36)
        if kind == "error":
            return near(0.32, 0.32, 0.68, 0.68) or near(0.68, 0.32, 0.32, 0.68)
        return False

    def pixel(x, y):
        solid = ring_hits = mark = 0
        for sy in range(SAMPLES):
            for sx in range(SAMPLES):
                px, py = x + (sx + .5) / SAMPLES, y + (sy + .5) / SAMPLES
                dist = ((px - r) ** 2 + (py - r) ** 2) ** 0.5
                if kind in ("running",):
                    if r - 1.6 <= dist <= r - 0.2:
                        ring_hits += 1
                elif kind == "dot":
                    if dist <= r:
                        solid += 1
                elif dist <= r - 0.2:
                    solid += 1
                    if on_mark(px, py):
                        mark += 1
        total = SAMPLES * SAMPLES
        if kind == "running":
            return _mix(color, bg, ring_hits / total)
        base = _mix(color, bg, solid / total)
        return _mix(white, base, mark / total) if mark else base

    return _png(size, size, pixel)


def cache_dir() -> str:
    from . import config

    path = os.path.join(config.data_dir(), "ui-cache")
    os.makedirs(path, exist_ok=True)
    return path


def cached_png(name: str, builder, *args) -> str:
    """Ghi PNG mot lan vao ui-cache (ten theo tham so) va tra ve duong dan he thong."""
    key = hashlib.sha1(repr((name, args)).encode("utf-8")).hexdigest()[:16]
    path = os.path.join(cache_dir(), "%s-%s.png" % (name, key))
    if not os.path.exists(path):
        data = builder(*args)
        temp = path + ".tmp"
        with open(temp, "wb") as handle:
            handle.write(data)
        os.replace(temp, path)
    return path
