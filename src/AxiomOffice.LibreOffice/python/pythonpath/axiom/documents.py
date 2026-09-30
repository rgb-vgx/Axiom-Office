"""Tim tai lieu dich theo loai app va gom moi lenh thanh mot buoc Undo (LibreOffice_arch.md muc 5.2, LD-5)."""
from __future__ import annotations

import contextlib

import uno

SERVICES = {
    "wps": "com.sun.star.text.TextDocument",
    "et": "com.sun.star.sheet.SpreadsheetDocument",
    "wpp": "com.sun.star.presentation.PresentationDocument",
}
NAMES = {"wps": "document", "et": "spreadsheet", "wpp": "presentation"}


class NoDocumentError(Exception):
    pass


def desktop(ctx):
    return ctx.ServiceManager.createInstanceWithContext("com.sun.star.frame.Desktop", ctx)


def _is(component, kind: str) -> bool:
    try:
        return component is not None and component.supportsService(SERVICES[kind])
    except Exception:  # noqa: BLE001
        return False


def active(ctx, kind: str, required: bool = True):
    """Tai lieu cung loai dang active; khong thi tai lieu cung loai cuoi cung trong danh sach."""
    top = desktop(ctx)
    current = top.getCurrentComponent()
    if _is(current, kind):
        return current
    found = None
    enumeration = top.getComponents().createEnumeration()
    while enumeration.hasMoreElements():
        component = enumeration.nextElement()
        if _is(component, kind):
            found = component
    if found is None and required:
        raise NoDocumentError("no active %s (open one in LibreOffice first)" % NAMES[kind])
    return found


def describe(document) -> dict:
    if document is None:
        return {"name": None, "fullName": None, "saved": None}
    url = document.getURL()
    try:
        title = document.getTitle()
    except Exception:  # noqa: BLE001
        title = None
    path = uno.fileUrlToSystemPath(url) if url.startswith("file:") else None
    return {"name": title, "fullName": path or title, "saved": not document.isModified()}


@contextlib.contextmanager
def undo_step(document, title: str):
    """Moi lenh cua AI = 1 buoc Ctrl+Z (tuong duong UndoRecordScope cua Word)."""
    manager = None
    try:
        manager = document.getUndoManager()
        manager.enterUndoContext("AI: " + title)
    except Exception:  # noqa: BLE001 - tai lieu khong co undo manager
        manager = None
    try:
        yield
    finally:
        if manager is not None:
            try:
                manager.leaveUndoContext()
            except Exception:  # noqa: BLE001
                pass


FACTORIES = {"wps": "private:factory/swriter", "et": "private:factory/scalc", "wpp": "private:factory/simpress"}
PDF_FILTERS = {"wps": "writer_pdf_Export", "et": "calc_pdf_Export", "wpp": "impress_pdf_Export"}
# Luu theo duoi file: giu dinh dang Office khi model/nguoi dung dat ten .docx/.xlsx/.pptx.
SAVE_FILTERS = {
    ".docx": "MS Word 2007 XML", ".doc": "MS Word 97", ".odt": "writer8", ".rtf": "Rich Text Format", ".txt": "Text",
    ".xlsx": "Calc MS Excel 2007 XML", ".xls": "MS Excel 97", ".ods": "calc8", ".csv": "Text - txt - csv (StarCalc)",
    ".pptx": "Impress MS PowerPoint 2007 XML", ".ppt": "MS PowerPoint 97", ".odp": "impress8",
}


def find_open(ctx, url: str):
    """Tai lieu da mo voi dung URL nay (so sanh getURL, khong dung ten file)."""
    enumeration = desktop(ctx).getComponents().createEnumeration()
    while enumeration.hasMoreElements():
        component = enumeration.nextElement()
        try:
            if component.getURL() == url:
                return component
        except Exception:  # noqa: BLE001 - component khong co URL (vd Start Center)
            continue
    return None


def activate(document) -> None:
    try:
        frame = document.getCurrentController().getFrame()
        frame.activate()
        window = frame.getContainerWindow()
        if window is not None:
            window.setFocus()
    except Exception:  # noqa: BLE001 - headless: khong co cua so
        pass


def open_document(ctx, kind: str, path: str | None):
    if path:
        import os

        url = to_url(os.path.abspath(path))
        existing = find_open(ctx, url)
        if existing is not None:
            # Mo lai chinh file dang mo: loadComponentFromURL se hien hop thoai "already open" (chan main
            # thread -> moi lenh sau deu Busy) hoac mo ban read-only; kich hoat cua so dang co la dung y.
            activate(existing)
            result = describe(existing)
            result.update(opened=False, alreadyOpen=True)
            return result
    else:
        url = FACTORIES[kind]
    doc = desktop(ctx).loadComponentFromURL(url, "_blank", 0, ())
    if doc is None:
        raise RuntimeError("cannot open " + (path or FACTORIES[kind]))
    result = describe(doc)
    result["opened"] = True
    return result


def close_all(ctx, kind: str) -> dict:
    """Dong moi tai lieu cung loai, KHONG luu (lenh cho test, khong ForAgent)."""
    closed = 0
    enumeration = desktop(ctx).getComponents().createEnumeration()
    targets = []
    while enumeration.hasMoreElements():
        component = enumeration.nextElement()
        if _is(component, kind):
            targets.append(component)
    for doc in targets:
        try:
            doc.setModified(False)
            doc.close(True)
            closed += 1
        except Exception:  # noqa: BLE001
            pass
    return {"closed": closed}


def save(document, kind: str, path: str | None) -> dict:
    import os

    if path:
        extension = os.path.splitext(path)[1].lower()
        if extension == ".pdf":
            return export_pdf(document, kind, path)
        args = [prop("Overwrite", True)]
        if extension in SAVE_FILTERS:
            args.append(prop("FilterName", SAVE_FILTERS[extension]))
        document.storeAsURL(to_url(os.path.abspath(path)), tuple(args))
    else:
        if not document.hasLocation():
            raise ValueError("the document has never been saved - use saveAs with a 'path'")
        document.store()
    return {"saved": True, "fullName": describe(document)["fullName"]}


def export_pdf(document, kind: str, path: str) -> dict:
    import os

    if not path:
        raise ValueError("'path' is required")
    document.storeToURL(to_url(os.path.abspath(path)), props(FilterName=PDF_FILTERS[kind], Overwrite=True))
    return {"exported": path}


def undo(document, count: int) -> dict:
    manager = document.getUndoManager()
    undone = 0
    for _ in range(max(1, count)):
        if not manager.isUndoPossible():
            break
        manager.undo()
        undone += 1
    return {"undone": undone}


def prop(name: str, value):
    item = uno.createUnoStruct("com.sun.star.beans.PropertyValue")
    item.Name = name
    item.Value = value
    return item


def props(**values):
    return tuple(prop(k, v) for k, v in values.items())


def to_url(path: str) -> str:
    return uno.systemPathToFileUrl(path)
