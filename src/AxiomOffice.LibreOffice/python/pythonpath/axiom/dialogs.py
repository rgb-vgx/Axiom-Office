"""Dialog Cai dat + Quan ly ghi nho cua pane (LibreOffice_arch.md muc 10).

Moi dialog la mot cua so con (`awt.ChildWindow`) giua cua so tai lieu - dialog roi (UnoControlDialog) khong
hien duoc tren ban 26.8 (LibreOffice_arch.md 14.2). Kieu chu/mau theo theme.py nhu pane. Cau hinh ghi bang
`config.set_value` (HKCU tren Windows - cung cho voi add-in; config.json tren Linux) va khoa API ma hoa DPAPI
nhu add-in.

Sua xong Cau dat thi Core dang chay van doc cau hinh cu (Core doc luc khoi dong) -> dialog nhac nguoi dung
tat Core de nap lai (hoac bam "Tat Core" - dung API /v1/admin/shutdown).
"""
from __future__ import annotations

import os

from . import config, core, theme
from .awt import ChildWindow, bind_actions

DIALOGS: dict = {}   # ten -> cua so con dang mo (bam lai thi dong cai cu)


def _open(ctx, frame, name: str, width: int, height: int) -> ChildWindow:
    """Mo cua so con giua cua so tai lieu (dang bam lai thi dong cua so cu truoc)."""
    old = DIALOGS.pop(name, None)
    if old is not None:
        try:
            old.close()
        except Exception:  # noqa: BLE001
            pass
    if frame is None:
        from . import documents

        frame = documents.desktop(ctx).getCurrentFrame()
    parent = frame.ContainerWindow
    # Giua VUNG TAI LIEU (khong phai ca cua so): khong de len pane neo ben phai.
    rect = frame.ComponentWindow.getPosSize()
    x = max(10, int(rect.X) + (int(rect.Width) - width) // 2)
    y = max(10, int(rect.Y) + (int(rect.Height) - height) // 2)
    window = ChildWindow(ctx, parent, width, height)
    window.add("FixedText", "title", PositionX=12, PositionY=8, Width=width - 24, Height=20, Label=name,
               FontName=theme.FONT_SEMIBOLD, FontHeight=theme.SIZE_EMPTY_TITLE)
    window.add("FixedText", "title_line", PositionX=0, PositionY=34, Width=width, Height=1, Label="",
               BackgroundColor=theme.DIVIDER)
    window.place(x, y, width, height)
    DIALOGS[name] = window
    return window


SETTINGS_FIELDS = (
    ("LlmProvider", "Nhà cung cấp", "openai"),
    ("LlmEndpoint", "Endpoint", ""),
    ("LlmModel", "Model", ""),
    ("LlmApiKey", "API key", ""),
    ("CoreExe", "Agent Core (CoreExe)", ""),
)
SETTINGS_FLAGS = (
    ("MemoryEnabled", "Ghi nhớ dài hạn (MemoryEnabled)", True),
    ("MemoryAutoExtract", "Tự trích xuất ghi nhớ sau lượt (MemoryAutoExtract)", True),
    ("VisualQaEnabled", "QA thị giác: gửi ảnh trang cho model (VisualQaEnabled)", False),
)
MEMORY_LIST_TIMEOUT = 10


def _label(dialog: ChildWindow, name: str, y: int, text: str, width: int = 134) -> None:
    dialog.add("FixedText", name, PositionX=12, PositionY=y + 4, Width=width, Height=16, Label=text,
               TextColor=theme.TEXT_SECONDARY)


def _edit(dialog: ChildWindow, name: str, y: int, x: int, width: int, text: str = "", **extra) -> None:
    dialog.add("Edit", name, PositionX=x, PositionY=y, Width=width, Height=22, Text=text, **extra)


def _button(dialog: ChildWindow, name: str, x: int, y: int, width: int, text: str) -> None:
    dialog.add("Button", name, PositionX=x, PositionY=y, Width=width, Height=24, Label=text)


def open_settings(pane) -> None:
    ctx = pane.ctx
    dialog = _open(ctx, pane.frame if hasattr(pane, "frame") else None, "Cài đặt", 470, 380)
    if dialog is None:
        return
    y = 46
    for name, label, default in SETTINGS_FIELDS:
        current = config.value(name, default)
        if name == "LlmApiKey":
            # Gia tri da ma hoa (dpapi:...) khong hien lai; de trong = giu nguyen khoa cu.
            text = "" if str(current or "").startswith("dpapi:") else str(current or "")
            _edit(dialog, name, y, 150, 306, text, EchoChar=0x2022, HelpText="Để trống = giữ khoá đã lưu")
        else:
            _edit(dialog, name, y, 150, 306, str(current or ""))
        _label(dialog, name + "_label", y, label)
        y += 28

    y += 6
    for name, label, default in SETTINGS_FLAGS:
        dialog.add("CheckBox", name, PositionX=12, PositionY=y, Width=440, Height=18, Label=label,
                   State=1 if config.value(name, 1 if default else 0) not in (0, "0", "false", False) else 0)
        y += 20

    dialog.add("FixedText", "status", PositionX=12, PositionY=y + 6, Width=446, Height=40, MultiLine=True,
               TextColor=theme.TEXT_MUTED, FontHeight=theme.SIZE_CAPTION,
               Label="Cấu hình dùng chung với add-in (HKCU\\Software\\AxiomOffice). Agent Core đọc lúc khởi động.")
    _button(dialog, "save", 12, 342, 90, "Lưu")
    _button(dialog, "check", 108, 342, 110, "Kiểm tra Core")
    _button(dialog, "restart", 224, 342, 110, "Tắt Core")
    _button(dialog, "close", 368, 342, 90, "Đóng")

    def on_action(command: str, event) -> None:
        if command == "save":
            for name, _, _ in SETTINGS_FIELDS:
                text = str(dialog.value(name, "Text") or "").strip()
                if name == "LlmApiKey":
                    if text:
                        config.set_value(name, config.protect_secret(text))
                    continue
                config.set_value(name, text)
            for name, _, _ in SETTINGS_FLAGS:
                config.set_value(name, 1 if int(dialog.value(name, "State") or 0) else 0)
            dialog.set("status", Label="Đã lưu. Cấu hình Core đọc khi khởi động - bấm \"Tắt Core\" rồi gửi yêu cầu mới để Core nạp lại.")
        elif command == "check":
            try:
                base, port = core.ensure()
                info = core.health(base) or {}
                dialog.set("status", Label="Agent Core: OK (pid %s, cổng %s, model %s)" % (
                    info.get("pid"), port, config.value("LlmModel", "")))
            except core.CoreError as exc:
                dialog.set("status", Label="Không kết nối được Agent Core: %s" % exc)
        elif command == "restart":
            stopped = _shutdown_core()
            dialog.set("status", Label="Đã yêu cầu Core tắt." if stopped else "Không thấy Core đang chạy (sẽ tự khởi động ở lượt sau).")
        elif command == "close":
            dialog.close()
            DIALOGS.pop("Cài đặt", None)

    bind_actions(dialog, ("save", "check", "restart", "close"), on_action)


def _shutdown_core() -> bool:
    try:
        base, _ = core.ensure()
    except core.CoreError:
        return False
    try:
        core.call(base, "POST", "/v1/admin/shutdown", {}, timeout=5)
        return True
    except core.CoreError:
        return False


def open_memory(pane) -> None:
    ctx = pane.ctx
    dialog = _open(ctx, pane.frame if hasattr(pane, "frame") else None, "Ghi nhớ", 520, 440)
    if dialog is None:
        return
    dialog.add("Edit", "query", PositionX=12, PositionY=46, Width=426, Height=24)
    _button(dialog, "search", 444, 46, 64, "Tìm")
    dialog.add("ListBox", "items", PositionX=12, PositionY=78, Width=496, Height=270, StringItemList=())
    dialog.add("FixedText", "status", PositionX=12, PositionY=354, Width=496, Height=32, MultiLine=True,
               TextColor=theme.TEXT_MUTED, FontHeight=theme.SIZE_CAPTION, Label="Đang tải…")
    _button(dialog, "delete", 12, 402, 90, "Xoá")
    _button(dialog, "reload", 108, 402, 90, "Tải lại")
    _button(dialog, "close", 418, 402, 90, "Đóng")

    state = {"items": [], "error": ""}

    def load(query: str = "") -> None:
        state["items"] = []
        try:
            base, _ = core.ensure()
            rows = core.memories(base, query=query)
        except core.CoreError as exc:
            dialog.set("items", StringItemList=())
            dialog.set("status", Label="Không đọc được ghi nhớ: %s" % exc)
            return
        labels = []
        for row in rows:
            state["items"].append(row)
            text = str(row.get("text") or "").replace("\n", " ")
            labels.append("%s · %s" % (row.get("scope") or "user", text[:90]))
        dialog.set("items", StringItemList=tuple(labels))
        dialog.set("status", Label="%d ghi nhớ%s" % (len(labels), " · chọn rồi bấm Xoá" if labels else ""))

    def on_action(command: str, event) -> None:
        if command == "reload":
            load(str(dialog.value("query", "Text") or ""))
        elif command == "search":
            load(str(dialog.value("query", "Text") or ""))
        elif command == "delete":
            selected = dialog.value("items", "SelectedItems")
            indexes = list(selected) if selected else []
            if not indexes:
                dialog.set("status", Label="Chọn một ghi nhớ trước đã.")
                return
            row = state["items"][indexes[-1]]
            try:
                base, _ = core.ensure()
                core.delete_memory(base, row.get("id"))
                dialog.set("status", Label="Đã xoá: %s" % str(row.get("text"))[:60])
            except core.CoreError as exc:
                dialog.set("status", Label="Không xoá được: %s" % exc)
            load(str(dialog.value("query", "Text") or ""))
        elif command == "close":
            dialog.close()
            DIALOGS.pop("Ghi nhớ", None)

    bind_actions(dialog, ("search", "delete", "reload", "close"), on_action)
    load()
