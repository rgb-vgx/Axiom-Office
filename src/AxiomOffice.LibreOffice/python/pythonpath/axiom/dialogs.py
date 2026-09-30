"""Dialog Cai dat + Quan ly ghi nho cua pane (LibreOffice_arch.md muc 10).

Mac dinh moi dialog la mot cua so con (`awt.ChildWindow`) giua cua so tai lieu. Mo TU WIZARD thi truyen
`floating=True` de dung cua so roi (`awt.Dialog`): wizard cung la cua so roi nen cua so con nam duoi bi che,
va tren Linux cua so con khong nhan chuot (xem awt.Dialog). Kieu chu/mau theo theme.py nhu pane. Cau hinh ghi
bang `config.set_value` (HKCU tren Windows - cung cho voi add-in; config.json tren Linux) va khoa API ma hoa
DPAPI nhu add-in.

Cau hinh LLM (dia chi/model/khoa) CO HIEU LUC NGAY: Core doc lai moi luot chay (Models/ModelSource.cs), khong
phai khoi dong lai. Chi `Token` va `CorePort` moi chot luc Core khoi dong - doi thi bam "Tat Core" (dung API
/v1/admin/shutdown) roi gui yeu cau moi de Core tu chay lai.
"""
from __future__ import annotations

import uno

import os

from . import config, core, theme
from .awt import ChildWindow, Dialog, bind_actions

DIALOGS: dict = {}   # ten -> cua so dang mo (ChildWindow hoac Dialog; bam lai thi dong cai cu)


def _open(ctx, frame, name: str, width: int, height: int, floating: bool = False):
    """Mo cua so giua cua so tai lieu (dang bam lai thi dong cua so cu truoc).

    floating=True: cua so roi (`awt.Dialog`) - dung khi mo tu wizard thiet lap (cung la cua so roi): cua so
    con nam DUOI wizard nen bi che, va tren Linux cua so con khong nhan chuot (xem awt.Dialog).
    """
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
    if floating:
        window = Dialog(ctx, width, height, name)
        window.on_close(lambda: _forget(name, window))   # bam X tren thanh tieu de
    else:
        window = ChildWindow(ctx, parent, width, height)
    window.add("FixedText", "title", PositionX=12, PositionY=8, Width=width - 24, Height=20, Label=name,
               FontName=theme.FONT_SEMIBOLD, FontHeight=theme.SIZE_EMPTY_TITLE)
    window.add("FixedText", "title_line", PositionX=0, PositionY=34, Width=width, Height=1, Label="",
               BackgroundColor=theme.DIVIDER)
    window.place(x, y, width, height)
    DIALOGS[name] = window
    return window


def _forget(name: str, window) -> None:
    """Dong cua so roi va bo khoi DIALOGS (chi khi van la cua so dang ghi - cua so cu bi thay thi thoi).

    Mot lan bam X bao ba lan (windowClosing, windowClosed, disposing) nen phai chiu duoc goi lai.
    """
    if DIALOGS.get(name) is not window:
        return               # da bi cua so khac thay (hoac da don roi) - khong dong len cua so moi
    DIALOGS.pop(name, None)
    window.close()


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
# Noi luu cau hinh: noi ro theo he dieu hanh de nguoi dung biet dang sua file nao.
STORAGE_HINT = (r"Cấu hình dùng chung với add-in (HKCU\Software\AxiomOffice). Địa chỉ/model/khoá dùng được ngay; "
                "chỉ 'Tắt Core' khi vừa đổi Token." if config.IS_WINDOWS else
                "Cấu hình dùng chung với Agent Core (~/.config/axiom-office/config.json). Khoá API lưu vào "
                "keyring nếu máy có secret-tool, không thì trong file quyền 0600. Địa chỉ/model/khoá dùng "
                "được ngay, không cần khởi động lại.")


def _label(dialog: ChildWindow, name: str, y: int, text: str, width: int = 134) -> None:
    dialog.add("FixedText", name, PositionX=12, PositionY=y + 4, Width=width, Height=16, Label=text,
               TextColor=theme.TEXT_SECONDARY)


def _edit(dialog: ChildWindow, name: str, y: int, x: int, width: int, text: str = "", **extra) -> None:
    dialog.add("Edit", name, PositionX=x, PositionY=y, Width=width, Height=22, Text=text, **extra)


def _button(dialog: ChildWindow, name: str, x: int, y: int, width: int, text: str) -> None:
    dialog.add("Button", name, PositionX=x, PositionY=y, Width=width, Height=24, Label=text)


def open_settings(pane, floating: bool = False) -> None:
    ctx = pane.ctx
    dialog = _open(ctx, pane.frame if hasattr(pane, "frame") else None, "Cài đặt", 470, 380, floating)
    if dialog is None:
        return
    y = 46
    for name, label, default in SETTINGS_FIELDS:
        current = config.value(name, default)
        if name == "LlmApiKey":
            # Gia tri da ma hoa (dpapi:/libsecret:) khong hien lai; de trong = giu nguyen khoa cu.
            stored = str(current or "")
            text = "" if stored.startswith(("dpapi:", config.LIBSECRET_PREFIX)) else stored
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
               Label=STORAGE_HINT)
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
                        config.set_value(name, config.protect_secret(text, name))
                    continue
                config.set_value(name, text)
            for name, _, _ in SETTINGS_FLAGS:
                config.set_value(name, 1 if int(dialog.value(name, "State") or 0) else 0)
            dialog.set("status", Label="Đã lưu. Địa chỉ/model/khoá mới dùng được ngay; nếu vừa đổi khoá bảo vệ "
                                           "(Token) thì bấm \"Tắt Core\" rồi gửi yêu cầu mới.")
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
    dialog.add("ListBox", "items", PositionX=12, PositionY=78, Width=496, Height=270,
               StringItemList=uno.Any("[]string", ()))
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
            uno.invoke(dialog.models["items"], "setPropertyValue", ("StringItemList", uno.Any("[]string", ())))
            dialog.set("status", Label="Không đọc được ghi nhớ: %s" % exc)
            return
        labels = []
        for row in rows:
            state["items"].append(row)
            text = str(row.get("text") or "").replace("\n", " ")
            labels.append("%s · %s" % (row.get("scope") or "user", text[:90]))
        # pyuno can kieu tuong minh cho sequence<string> va uno.Any chi dung duoc qua uno.invoke (dat tuple
        # truc tiep thi bao "Unable to convert the given value for the property StringItemList").
        uno.invoke(dialog.models["items"], "setPropertyValue",
                   ("StringItemList", uno.Any("[]string", tuple(labels))))
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
