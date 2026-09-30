"""Lenh Writer (kind "wps") - cung ten, tham so va hinh dang ket qua voi CommandDispatcher.Writer.cs
(LibreOffice_arch.md muc 7.1). Moi lenh sua tai lieu chay trong mot undo context (undo=True)."""
from __future__ import annotations

import uno

from . import documents, values
from .commands import command, opt, req

PARAGRAPH_BREAK = 0          # com.sun.star.text.ControlCharacter.PARAGRAPH_BREAK
BOLD, NORMAL = 150.0, 100.0  # com.sun.star.awt.FontWeight
ALIGN = {"left": "LEFT", "center": "CENTER", "centre": "CENTER", "right": "RIGHT", "justify": "BLOCK", "justified": "BLOCK", "block": "BLOCK"}

# Style bang cua Word -> autoformat co san cua LibreOffice (gan dung mau; khong co thi bao skipped).
WORD_TABLE_STYLES = {
    "accent 1": "Box List Blue", "accent 5": "Box List Blue", "accent 2": "Box List Red",
    "accent 4": "Box List Yellow", "accent 6": "Box List Green", "accent 3": "Simple List Shaded",
}


# ---------- tien ich ----------

def _view_cursor(doc):
    return doc.getCurrentController().getViewCursor()


def _normalize(text: str) -> str:
    return text.replace("\r\n", "\n").replace("\r", "\n").replace("\v", "\n")


def _insert(text_obj, cursor, text: str) -> int:
    """Chen text tai cursor (cursor di theo); '\\n' -> ngat doan. Tra ve so ky tu (ngat doan tinh 1)."""
    parts = _normalize(text).split("\n")
    for index, part in enumerate(parts):
        if index:
            text_obj.insertControlCharacter(cursor, PARAGRAPH_BREAK, False)
        if part:
            text_obj.insertString(cursor, part, False)
    return sum(len(p) for p in parts) + len(parts) - 1


def _select_back(cursor, count: int) -> None:
    while count > 0:
        step = min(count, 30000)
        cursor.goLeft(step, True)
        count -= step


def _insert_at_view(doc, text: str):
    """Chen tai con tro man hinh; tra ve text cursor dang bao phu doan vua chen."""
    view = _view_cursor(doc)
    text_obj = view.getText()
    cursor = text_obj.createTextCursorByRange(view.getEnd())
    count = _insert(text_obj, cursor, text)
    view.gotoRange(cursor.getEnd(), False)
    _select_back(cursor, count)
    return cursor


def _apply_font(target, params) -> None:
    if values.has(params, "bold"):
        target.CharWeight = BOLD if values.boolean(params, "bold", False) else NORMAL
    if values.has(params, "italic"):
        target.CharPosture = uno.Enum("com.sun.star.awt.FontSlant", "ITALIC" if values.boolean(params, "italic", False) else "NONE")
    if values.has(params, "underline"):
        target.CharUnderline = 1 if values.boolean(params, "underline", False) else 0
    if values.has(params, "size"):
        target.CharHeight = float(values.number(params, "size", 12))
    if values.has(params, "font"):
        target.CharFontName = values.string(params, "font")
    color = values.color(values.string(params, "color"))
    if color is not None:
        target.CharColor = color


def _alignment(name: str):
    key = (name or "left").strip().lower()
    if key not in ALIGN:
        raise values.ParamError("'alignment' must be left/center/right/justify, got '%s'" % name)
    return uno.Enum("com.sun.star.style.ParagraphAdjust", ALIGN[key])


def _body_elements(doc):
    enumeration = doc.getText().createEnumeration()
    while enumeration.hasMoreElements():
        yield enumeration.nextElement()


def _is_table(element) -> bool:
    try:
        return element.supportsService("com.sun.star.text.TextTable")
    except Exception:  # noqa: BLE001
        return False


def tables_in_order(doc) -> list:
    return [e for e in _body_elements(doc) if _is_table(e)]


def table_rows(table) -> list:
    data = []
    for r in range(table.getRows().getCount()):
        row = []
        for c in range(table.getColumns().getCount()):
            cell = table.getCellByPosition(c, r) if _cell_exists(table, c, r) else None
            row.append(cell.getString() if cell is not None else "")
        data.append(row)
    return data


def _cell_exists(table, c, r) -> bool:
    try:
        return table.getCellByPosition(c, r) is not None
    except Exception:  # noqa: BLE001 - o bi gop
        return False


def full_text(doc) -> str:
    """Doan + bang theo thu tu (Word Content.Text co ca chu trong bang)."""
    lines = []
    for element in _body_elements(doc):
        if _is_table(element):
            lines.extend("\t".join(row) for row in table_rows(element))
        else:
            lines.append(element.getString())
    return "\n".join(lines)


def _current_table(doc):
    try:
        return _view_cursor(doc).getPropertyValue("TextTable")
    except Exception:  # noqa: BLE001
        return None


def _range_after_table(doc, table):
    found = False
    for element in _body_elements(doc):
        if found:
            return element.getStart()
        if _is_table(element) and element.getName() == table.getName():
            found = True
    return doc.getText().getEnd()


def _style_names(doc) -> list:
    try:
        return list(doc.getStyleFamilies().getByName("TableStyles").getElementNames())
    except Exception:  # noqa: BLE001
        return []


def _apply_table_style(doc, table, name: str) -> str:
    names = _style_names(doc)
    chosen = name if name in names else None
    if chosen is None:
        lower = (name or "").lower()
        chosen = next((v for k, v in WORD_TABLE_STYLES.items() if k in lower), None)
        if chosen is None and ("grid" in lower or "table" in lower):
            chosen = "Default Table Style"
        if chosen not in names:
            raise ValueError("no LibreOffice table style like '%s' (available: %s)" % (name, ", ".join(names)))
    table.autoFormat(chosen)
    return chosen


# ---------- handlers ----------

def get_text(env, params):
    doc = env.document
    text = full_text(doc)
    total = len(text)
    max_chars = values.integer(params, "maxChars", 0)
    truncated = max_chars > 0 and total > max_chars
    if truncated:
        text = text[:max_chars]
    info = documents.describe(doc)
    return {"name": info["name"], "fullName": info["fullName"], "totalChars": total, "truncated": truncated, "text": text}


def selection(env, params):
    doc = env.document
    view = _view_cursor(doc)
    text = view.getString()
    start = end = None
    try:
        cursor = doc.getText().createTextCursor()
        cursor.gotoStart(False)
        cursor.gotoRange(view.getStart(), True)
        start = len(cursor.getString())
        end = start + len(text)
    except Exception:  # noqa: BLE001 - con tro trong bang/khung: khong co vi tri trong than van ban
        pass
    return {"text": text, "start": start, "end": end}


def type_text(env, params):
    text = values.string(params, "text")
    if text is None:
        return {"typed": 0}
    _insert_at_view(env.document, text)
    return {"typed": len(text)}


def append_text(env, params):
    text = values.string(params, "text")
    if text is None:
        return {"appended": 0}
    body = env.document.getText()
    cursor = body.createTextCursorByRange(body.getEnd())
    _insert(body, cursor, text)
    return {"appended": len(text)}


def insert_styled_text(env, params):
    text = values.string(params, "text")
    if not text:
        return {"inserted": 0}
    cursor = _insert_at_view(env.document, text)
    _apply_font(cursor, params)
    return {"inserted": len(text)}


def heading(env, params):
    level = max(1, min(9, values.integer(params, "level", 1)))
    text = values.string(params, "text")
    doc = env.document
    style = "Heading %d" % level
    view = _view_cursor(doc)
    text_obj = view.getText()
    cursor = text_obj.createTextCursorByRange(view.getEnd())
    if text:
        _insert(text_obj, cursor, text)
    cursor.ParaStyleName = style
    if text and values.boolean(params, "break", True):
        text_obj.insertControlCharacter(cursor, PARAGRAPH_BREAK, False)
        try:
            follow = doc.getStyleFamilies().getByName("ParagraphStyles").getByName(style).FollowStyle
            cursor.ParaStyleName = follow or "Standard"
        except Exception:  # noqa: BLE001
            cursor.ParaStyleName = "Standard"
    view.gotoRange(cursor.getEnd(), False)
    return {"level": level}


def format_selection(env, params):
    view = _view_cursor(env.document)
    _apply_font(view, params)
    if values.has(params, "alignment"):
        view.ParaAdjust = _alignment(values.string(params, "alignment"))
    return {"formatted": True}


def set_paragraph_alignment(env, params):
    alignment = values.string(params, "alignment", "left")
    _view_cursor(env.document).ParaAdjust = _alignment(alignment)
    return {"alignment": alignment}


def insert_table(env, params):
    rows_values = values.matrix(params, "values", False)
    raw_rows = params.get("rows") if isinstance(params, dict) else None
    rows_hold_data = rows_values is None and isinstance(values.unwrap_scalar(raw_rows), list)
    if rows_hold_data:
        rows_values = values.matrix(params, "rows", False)  # model dat du lieu vao 'rows'
    rows = 0 if rows_hold_data else values.integer(params, "rows", 0)
    cols = values.integer(params, "cols", 0)
    if rows_values:
        rows = max(rows, len(rows_values))
        cols = max(cols, max(len(r) for r in rows_values))
    if rows <= 0 or cols <= 0:
        raise values.ParamError("'rows' and 'cols' are required (or pass 'values' as a 2D array to size the table)")
    doc = env.document
    view = _view_cursor(doc)
    current = _current_table(doc)
    anchor = _range_after_table(doc, current) if current is not None else view.getEnd()
    table = doc.createInstance("com.sun.star.text.TextTable")
    table.initialize(rows, cols)
    anchor_text = anchor.getText()
    anchor_text.insertTextContent(anchor, table, False)
    filled = 0
    for r, row in enumerate(rows_values or []):
        for c, cell in enumerate(row[:cols]):
            table.getCellByPosition(c, r).setString("" if cell is None else _cell_text(cell))
            filled += 1
    result = {"rows": rows, "cols": cols, "filled": filled}
    if values.has(params, "style"):
        try:
            result["style"] = _apply_table_style(doc, table, values.string(params, "style"))
        except Exception as exc:  # noqa: BLE001
            result["styleError"] = str(exc)
    # Con tro ra sau bang de lenh go/chen tiep theo khong ghi vao o dau.
    try:
        view.gotoRange(_range_after_table(doc, table), False)
    except Exception:  # noqa: BLE001
        pass
    return result


def _cell_text(value) -> str:
    if isinstance(value, bool):
        return "True" if value else "False"
    if isinstance(value, float) and value.is_integer():
        return str(int(value))
    return str(value)


def _border_line(width: int, color: int):
    line = uno.createUnoStruct("com.sun.star.table.BorderLine2")
    line.OuterLineWidth = width
    line.LineWidth = width
    line.Color = color
    return line


def _set_borders(table, enabled: bool, color: int = 0):
    border = table.TableBorder2
    line = _border_line(18 if enabled else 0, color)
    for side in ("TopLine", "BottomLine", "LeftLine", "RightLine", "HorizontalLine", "VerticalLine"):
        setattr(border, side, line)
        setattr(border, "Is%sValid" % side, True)
    table.TableBorder2 = border


def format_table(env, params):
    doc = env.document
    tables = tables_in_order(doc)
    if not tables:
        raise values.ParamError("the document has no table - use writer.insertTable first")
    index = values.integer(params, "table", 0)
    table = _current_table(doc) if index == 0 else None
    if table is None:
        if index == 0:
            index = len(tables)
        if index < 1 or index > len(tables):
            raise values.ParamError("'table' must be between 1 and %d, got %d" % (len(tables), index))
        table = tables[index - 1]
    rows = table.getRows().getCount()
    cols = table.getColumns().getCount()
    whole = table.getCellRangeByPosition(0, 0, cols - 1, rows - 1)
    applied, skipped = [], []

    def step(name, body):
        try:
            body()
            applied.append(name)
        except Exception as exc:  # noqa: BLE001
            skipped.append("%s: %s" % (name, getattr(exc, "Message", None) or exc))

    def row_range(r):
        return table.getCellRangeByPosition(0, r, cols - 1, r)

    if values.has(params, "style"):
        step("style", lambda: _apply_table_style(doc, table, values.string(params, "style")))
    if values.has(params, "font"):
        step("font", lambda: setattr(whole, "CharFontName", values.string(params, "font")))
    if values.has(params, "size"):
        step("size", lambda: setattr(whole, "CharHeight", float(values.number(params, "size", 11))))
    color = values.color(values.string(params, "color"))
    if color is not None:
        step("color", lambda: setattr(whole, "CharColor", color))
    if values.has(params, "alignment"):
        step("alignment", lambda: setattr(whole, "ParaAdjust", _alignment(values.string(params, "alignment"))))
    if values.has(params, "borders"):
        enabled = values.boolean(params, "borders", True)
        step("borders", lambda: _set_borders(table, enabled))
    border_color = values.color(values.string(params, "borderColor"))
    if border_color is not None:
        step("borderColor", lambda: _set_borders(table, True, border_color))
    band = values.color(values.string(params, "bandFill"))
    if band is not None:
        def banding():
            for r in range(2, rows, 2):
                row_range(r).BackColor = band
        step("bandFill", banding)
    header_fill = values.color(values.string(params, "headerFill"))
    header_color = values.color(values.string(params, "headerColor"))
    header_bold = values.boolean(params, "headerBold", header_fill is not None or header_color is not None)
    if header_fill is not None:
        step("headerFill", lambda: setattr(row_range(0), "BackColor", header_fill))
    if header_color is not None:
        step("headerColor", lambda: setattr(row_range(0), "CharColor", header_color))
    if values.has(params, "headerBold") or header_fill is not None or header_color is not None:
        def bold_header():
            row_range(0).CharWeight = BOLD if header_bold else NORMAL
            table.RepeatHeadline = True
        step("headerBold", bold_header)
    if values.has(params, "autoFit"):
        fit = (values.string(params, "autoFit", "content") or "content").strip().lower()

        def auto_fit():
            if fit != "window":
                raise ValueError("autoFit 'content' is not supported by LibreOffice UNO")
            table.HoriOrient = 6  # com.sun.star.text.HoriOrientation.FULL
        step("autoFit", auto_fit)
    result = {"table": "at cursor" if index == 0 else index, "rows": rows, "cols": cols, "applied": applied}
    if skipped:
        result["skipped"] = skipped
    return result


def insert_page_break(env, params):
    doc = env.document
    view = _view_cursor(doc)
    text_obj = view.getText()
    cursor = text_obj.createTextCursorByRange(view.getEnd())
    text_obj.insertControlCharacter(cursor, PARAGRAPH_BREAK, False)
    cursor.BreakType = uno.Enum("com.sun.star.style.BreakType", "PAGE_BEFORE")
    view.gotoRange(cursor.getEnd(), False)
    return {"inserted": True}


def insert_image(env, params):
    import os

    path = values.string(params, "path")
    if not path:
        raise values.ParamError("'path' is required")
    if not os.path.isfile(path):
        raise values.ParamError("image file not found: '%s'" % path)
    doc = env.document
    ctx = env.ctx
    provider = ctx.ServiceManager.createInstanceWithContext("com.sun.star.graphic.GraphicProvider", ctx)
    graphic = provider.queryGraphic((documents.prop("URL", documents.to_url(path)),))
    shape = doc.createInstance("com.sun.star.text.TextGraphicObject")
    shape.Graphic = graphic
    shape.AnchorType = uno.Enum("com.sun.star.text.TextContentAnchorType", "AS_CHARACTER")
    size = graphic.Size100thMM
    if not size.Width or not size.Height:
        pixels = graphic.SizePixel
        size.Width, size.Height = pixels.Width * 2540 // 96, pixels.Height * 2540 // 96
    width_pt, height_pt = size.Width / values.POINT_TO_HMM, size.Height / values.POINT_TO_HMM
    has_w, has_h = values.has(params, "width"), values.has(params, "height")
    if has_w and has_h:
        width_pt, height_pt = values.number(params, "width", width_pt), values.number(params, "height", height_pt)
    elif has_w:
        new_w = values.number(params, "width", width_pt)
        width_pt, height_pt = new_w, height_pt * new_w / width_pt
    elif has_h:
        new_h = values.number(params, "height", height_pt)
        width_pt, height_pt = width_pt * new_h / height_pt, new_h
    target = uno.createUnoStruct("com.sun.star.awt.Size")
    target.Width, target.Height = int(round(width_pt * values.POINT_TO_HMM)), int(round(height_pt * values.POINT_TO_HMM))
    shape.setSize(target)
    view = _view_cursor(doc)
    view.getText().insertTextContent(view.getEnd(), shape, False)
    return {"width": round(width_pt, 2), "height": round(height_pt, 2)}


def insert_hyperlink(env, params):
    url = values.string(params, "url")
    if not url:
        raise values.ParamError("'url' is required")
    text = values.string(params, "text") or url
    cursor = _insert_at_view(env.document, text)
    cursor.HyperLinkURL = url
    return {"text": text, "url": url}


_REGEX_SPECIAL = set("\\.^$|?*+()[]{}")


def _regex_escape(text: str) -> str:
    return "".join("\\" + ch if ch in _REGEX_SPECIAL else ch for ch in text)


def replace_all(env, params):
    find = values.string(params, "find")
    replace = values.string(params, "replace", "") or ""
    if not find:
        return {"replaced": False}
    find, replace = _normalize(find), _normalize(replace)
    descriptor = env.document.createReplaceDescriptor()
    if "\n" in find or "\n" in replace:
        # LibreOffice khong tim xuyen doan: chi ho tro ngat doan o CUOI chuoi tim ('$' = cuoi doan).
        core = find.rstrip("\n")
        if "\n" in core:
            raise values.ParamError("'find' can only contain a line break at its end in LibreOffice (search runs paragraph by paragraph)")
        pattern = _regex_escape(core) + ("$" if core != find else "")
        if core != find and not core:
            raise values.ParamError("'find' cannot be only line breaks")
        descriptor.SearchRegularExpression = True
        descriptor.SearchString = pattern
        descriptor.ReplaceString = replace.replace("\\", "\\\\").replace("&", "\\&").replace("$", "\\$").replace("\n", "\\n")
    else:
        descriptor.SearchString = find
        descriptor.ReplaceString = replace
    descriptor.SearchCaseSensitive = False
    count = env.document.replaceAll(descriptor)
    return {"replaced": count > 0, "count": count}


def undo(env, params):
    return documents.undo(env.document, values.integer(params, "count", 1))


command("writer.newDocument", "wps", lambda env, p: documents.open_document(env.ctx, "wps", None), "Tạo tài liệu mới")
command("writer.open", "wps", lambda env, p: documents.open_document(env.ctx, "wps", values.string(p, "path")), "Mở .docx/.doc/.odt", req("path"))
command("writer.getText", "wps", get_text, "Đọc toàn bộ text", opt("maxChars"), agent=True)
command("writer.selection", "wps", selection, "Text + vị trí đang chọn", agent=True)
command("writer.typeText", "wps", type_text, "Gõ tại con trỏ", req("text"), agent=True, undo=True)
command("writer.appendText", "wps", append_text, "Nối vào cuối tài liệu", req("text"), agent=True, undo=True)
command("writer.insertStyledText", "wps", insert_styled_text, "Chèn text có định dạng tại con trỏ (`color` dạng `#RRGGBB`)",
        req("text"), opt("bold"), opt("italic"), opt("underline"), opt("size"), opt("color"), opt("font"), agent=True, undo=True)
command("writer.heading", "wps", heading, "Heading 1-9 (`level`, mặc định 1) + tự xuống dòng (`break`, mặc định true)",
        opt("text"), opt("level"), opt("break"), agent=True, undo=True)
command("writer.formatSelection", "wps", format_selection, "Định dạng vùng chọn",
        opt("bold"), opt("italic"), opt("underline"), opt("size"), opt("color"), opt("font"), opt("alignment"), agent=True, undo=True)
command("writer.setParagraphAlignment", "wps", set_paragraph_alignment, "Căn đoạn: left/center/right/justify",
        req("alignment"), agent=True, undo=True)
command("writer.insertTable", "wps", insert_table, "Chèn bảng; `rows`/`cols` tự suy ra/nới theo `values`",
        opt("rows"), opt("cols"), opt("values", "2D array of rows"), opt("style"), agent=True, undo=True)
command("writer.formatTable", "wps", format_table,
        "Định dạng bảng CÓ SẴN (không tạo lại): kiểu, font, màu hàng tiêu đề, màu sọc, viền, căn lề, co giãn",
        opt("table", "1-based index; default: table at cursor, else last table"), opt("style", "e.g. 'Grid Table 4 - Accent 1'"),
        opt("font"), opt("size"), opt("color", "#RRGGBB text color"),
        opt("headerFill", "#RRGGBB"), opt("headerColor", "#RRGGBB header text"), opt("headerBold"),
        opt("bandFill", "#RRGGBB every other data row"), opt("borderColor", "#RRGGBB"), opt("borders", "false = no borders (layout tables)"),
        opt("alignment", "left/center/right"), opt("autoFit", "content/window"), agent=True, undo=True)
command("writer.insertPageBreak", "wps", insert_page_break, "Ngắt trang", agent=True, undo=True)
command("writer.insertImage", "wps", insert_image, "Chèn ảnh tại con trỏ (kích thước theo point)",
        req("path"), opt("width"), opt("height"), agent=True, undo=True)
command("writer.insertHyperlink", "wps", insert_hyperlink, "Chèn liên kết", req("url"), opt("text"), agent=True, undo=True)
command("writer.replaceAll", "wps", replace_all, "Tìm và thay toàn bộ", req("find"), opt("replace"), agent=True, undo=True)
command("writer.undo", "wps", undo, "Hoàn tác (mỗi thao tác AI = 1 bước)", opt("count"), agent=True)
command("writer.exportPdf", "wps", lambda env, p: documents.export_pdf(env.document, "wps", values.string(p, "path")),
        "Xuất PDF", req("path"), agent=True)
command("writer.save", "wps", lambda env, p: documents.save(env.document, "wps", None), "Lưu", agent=True)
command("writer.saveAs", "wps", lambda env, p: documents.save(env.document, "wps", values.string(p, "path")),
        "Lưu thành file mới", req("path"), agent=True)
command("writer.closeAll", "wps", lambda env, p: documents.close_all(env.ctx, "wps"), "**Đóng mọi tài liệu, không lưu**")
