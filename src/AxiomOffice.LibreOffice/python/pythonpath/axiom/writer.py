"""Lenh Writer (kind "wps") - cung ten, tham so va hinh dang ket qua voi CommandDispatcher.Writer.cs
(docs/architecture/libreoffice.md muc 7.1). Moi lenh sua tai lieu chay trong mot undo context (undo=True)."""
from __future__ import annotations

import uno

from . import documents, values
from .commands import command, opt, req

PARAGRAPH_BREAK = 0          # com.sun.star.text.ControlCharacter.PARAGRAPH_BREAK
BOLD, NORMAL = 150.0, 100.0  # com.sun.star.awt.FontWeight
ALIGN = {"left": "LEFT", "center": "CENTER", "centre": "CENTER", "right": "RIGHT", "justify": "BLOCK", "justified": "BLOCK", "block": "BLOCK"}

# Tem "bang dan trang" cho checks.py (log 11/10/2026): van ban hanh chinh dung bang KHONG KE KHUNG lam bo cuc
# (phần đầu, phần cuối), trong đó ô trống là CỐ Ý (khoảng trống ký, ô chờ điền). checks.py báo "empty-cells"
# cho chúng thì agent đi "sửa" bằng cách điền "—" vào chỗ ký - tức là phá đúng thể thức văn bản.
#
# Không đoán theo nét kẻ được, vì hai chiều đều sai:
#   - bảng DỮ LIỆU chưa định dạng cũng không có nét kẻ nào (live test chèn bảng trần rồi đòi checkTables bắt
#     empty-cells), nên "không nét kẻ" KHÔNG suy ra được "bảng dàn trang";
#   - bảng dàn trang có kẻ khung (biểu mẫu điền) thì vẫn là bảng dàn trang.
# Nên dấu hiệu duy nhất đáng tin là tem do chính formatTable {borders:false} đặt. Tem nằm ở tên bảng nên
# theo .odt sang lần mở sau (đã đo: đặt Name -> lưu -> mở lại, tên còn nguyên).
LAYOUT_TABLE_NAME = "AxiomLayout"   # tên bảng dàn trang do Axiom đặt
DATA_TABLE_NAME = "AxiomTable"      # tên trả lại khi bảng được kẻ khung trở lại

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


def _apply_font_reset(target, params) -> None:
    """Nhu _apply_font nhung dat tuong minh ca thuoc tinh khong truyen, cho KHOI doan van: cursor giu
    bold/underline/mau/co/font cua lan chen truoc nen bo qua se lam dinh sang cac doan sau (do 24/10/2026
    tren Word). Khoi khong truyen size/font nhan mac dinh than van ban cua design (Times New Roman 13)."""
    target.CharWeight = BOLD if values.boolean(params, "bold", False) else NORMAL
    target.CharPosture = uno.Enum("com.sun.star.awt.FontSlant",
                                  "ITALIC" if values.boolean(params, "italic", False) else "NONE")
    target.CharUnderline = 1 if values.boolean(params, "underline", False) else 0
    target.CharHeight = float(values.number(params, "size", 13)) if values.has(params, "size") else 13.0
    target.CharFontName = values.string(params, "font") if values.has(params, "font") else "Times New Roman"
    color = values.color(values.string(params, "color"))
    target.CharColor = color if color is not None else -1  # -1 = automatic


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
    if not text:
        return {"appended": 0, "paragraphs": 0}
    alignment = values.string(params, "alignment", "left")
    # '\n' dau chuoi chi la y dinh "mo doan": bo di, de chi con MOT lan ngat doan do doan cuoi quyet dinh.
    normalized = _normalize(text).lstrip("\n")
    if not normalized:
        return {"appended": 0, "paragraphs": 0}
    parts = normalized.split("\n")
    body = env.document.getText()
    cursor = body.createTextCursorByRange(body.getEnd())
    # Ngat doan TRUOC khi chen: chi dat dinh dang/can le cho doan moi, khong cham doan cuoi.
    if _paragraph_has_text_before(body, cursor):
        body.insertControlCharacter(cursor, PARAGRAPH_BREAK, False)
    for index, part in enumerate(parts):
        _apply_font_reset(cursor, params)
        if alignment:
            cursor.ParaAdjust = _alignment(alignment)
        if part:
            body.insertString(cursor, part, False)
        if index < len(parts) - 1:
            body.insertControlCharacter(cursor, PARAGRAPH_BREAK, False)
    return {"appended": len(normalized), "paragraphs": len(parts)}


def insert_styled_text(env, params):
    text = values.string(params, "text")
    if not text:
        return {"inserted": 0, "paragraphs": 0}
    alignment = values.string(params, "alignment")
    # Co alignment = mot KHOI doan van: tu ngat doan sau khoi de lan goi ke tiep khong dinh vao doan nay.
    # Khong alignment = chen inline, giu nguyen hanh vi cu.
    block = bool(alignment)
    break_after = values.boolean(params, "break", True) if values.has(params, "break") else block
    view = _view_cursor(env.document)
    text_obj = view.getText()
    cursor = text_obj.createTextCursorByRange(view.getEnd())
    parts = _normalize(text).split("\n")
    for index, part in enumerate(parts):
        # Khoi: dat lai toan bo dinh dang (tranh dinh bold/underline/mau tu doan truoc). Inline: chi dat
        # thuoc tinh duoc truyen, de chu chen giua cau giu dinh dang xung quanh.
        if block:
            _apply_font_reset(cursor, params)
            cursor.ParaAdjust = _alignment(alignment)
        else:
            _apply_font(cursor, params)
        if part:
            text_obj.insertString(cursor, part, False)
        if index < len(parts) - 1:
            text_obj.insertControlCharacter(cursor, PARAGRAPH_BREAK, False)
    if break_after and parts[-1]:
        text_obj.insertControlCharacter(cursor, PARAGRAPH_BREAK, False)
    view.gotoRange(cursor.getEnd(), False)
    return {"inserted": len(text), "paragraphs": len(parts)}


def _paragraph_has_text_before(text_obj, cursor) -> bool:
    """Doan chua cursor co chu dung truoc vi tri cursor khong."""
    try:
        probe = text_obj.createTextCursorByRange(cursor.getStart())
        probe.gotoStartOfParagraph(True)
        return bool(probe.getString().strip())
    except Exception:  # noqa: BLE001
        return False


def heading(env, params):
    level = max(1, min(9, values.integer(params, "level", 1)))
    text = values.string(params, "text")
    doc = env.document
    style = "Heading %d" % level
    view = _view_cursor(doc)
    text_obj = view.getText()
    cursor = text_obj.createTextCursorByRange(view.getEnd())
    if text and _paragraph_has_text_before(text_obj, cursor):
        # Con tro dang o cuoi/giua mot doan co chu (vd sau appendText): tieu de phai la doan RIENG, neu khong
        # chu tieu de noi vao doan cu va ca doan bi doi thanh Heading.
        text_obj.insertControlCharacter(cursor, PARAGRAPH_BREAK, False)
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


def _rename_table(table, base: str) -> bool:
    """Dat ten bang theo `base`, them '-2', '-3'... khi trung ten. Tra False neu het cach.

    Ten bang Writer khong duoc chua dau cach hay dau cham (loi unotbl.cxx:2958 - dau cham danh rieng cho
    tham chieu <TenBang.A1>) va khong duoc trung bang khac (2971), nen phai thu lan luot.
    """
    for attempt in range(1, 51):
        try:
            table.Name = base if attempt == 1 else "%s-%d" % (base, attempt)
            return True
        except Exception:  # noqa: BLE001 - trung ten thi thu ten ke tiep
            continue
    return False


def _mark_layout(table, layout: bool) -> bool:
    """Danh dau / bo danh dau bang dan trang bang ten bang. Khong bao gio nem loi: day chi la tem cho
    checks.py, khong duoc lam hong lenh formatTable dang chay."""
    try:
        current = table.Name or ""
    except Exception:  # noqa: BLE001
        return False
    if layout:
        return True if current.startswith(LAYOUT_TABLE_NAME) else _rename_table(table, LAYOUT_TABLE_NAME)
    return _rename_table(table, DATA_TABLE_NAME) if current.startswith(LAYOUT_TABLE_NAME) else True


def is_layout_table(table) -> bool:
    """Bang dan trang (do formatTable {borders:false} danh dau) - xem LAYOUT_TABLE_NAME."""
    try:
        return (table.Name or "").startswith(LAYOUT_TABLE_NAME)
    except Exception:  # noqa: BLE001
        return False


def _require_layout_mark(table):
    """Xac nhan tem da dat sau khi bo khung. Dat tem that bai thi checkTables se bao "empty-cells" nham cho
    bang dan trang - phai hien ra o `skipped` chu khong im lang (xem LAYOUT_TABLE_NAME)."""
    if not is_layout_table(table):
        raise ValueError("could not mark the table as a layout table (its name may be taken)")


def _set_borders(table, enabled: bool, color: int = 0):
    border = table.TableBorder2
    line = _border_line(18 if enabled else 0, color)
    for side in ("TopLine", "BottomLine", "LeftLine", "RightLine", "HorizontalLine", "VerticalLine"):
        setattr(border, side, line)
        setattr(border, "Is%sValid" % side, True)
    table.TableBorder2 = border
    # Ke khung = bảng dữ liệu, bỏ khung = bảng dàn trang: kèm luôn tem cho checks.py (xem LAYOUT_TABLE_NAME).
    _mark_layout(table, not enabled)


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
        if not enabled:
            # Doc lai ten bang de XAC NHAN tem bang dan trang da dat that (xem LAYOUT_TABLE_NAME).
            step("layout-table", lambda: _require_layout_mark(table))
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


def replace_many(env, params):
    """Nhieu cap tim/thay trong mot lenh (gom cho model, bot vong goi - luot that 14/10/2026: 137 lan replaceAll)."""
    edits = params.get("edits") if isinstance(params, dict) else None
    if not isinstance(edits, list) or not edits:
        raise values.ParamError("'edits' must be a non-empty array of {find, replace}")
    if len(edits) > 200:
        raise values.ParamError("'edits' has %d items; send at most 200 per call" % len(edits))
    counts, not_found = [], []
    for edit in edits:
        if not isinstance(edit, dict) or not values.string(edit, "find"):
            counts.append(0)
            continue
        count = replace_all(env, edit).get("count", 0)
        counts.append(count)
        if not count:
            not_found.append(values.string(edit, "find")[:60])
    result = {"edits": len(edits), "replaced": sum(counts), "counts": counts}
    if not_found:
        result["notFound"] = not_found
    return result


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
command("writer.appendText", "wps", append_text,
        "Nối một KHỐI đoạn văn vào cuối tài liệu; `\\n` tách đoạn, tự đặt lại định dạng (không kế thừa), `alignment` mặc định left",
        req("text"), opt("bold"), opt("italic"), opt("underline"), opt("size"), opt("color"), opt("font"),
        opt("alignment", "left/center/right/justify; mặc định left"), agent=True, undo=True)
command("writer.insertStyledText", "wps", insert_styled_text,
        "Chèn text có định dạng tại con trỏ; `\\n` tách đoạn, `alignment` áp cho từng đoạn (`color` dạng `#RRGGBB`)",
        req("text"), opt("bold"), opt("italic"), opt("underline"), opt("size"), opt("color"), opt("font"),
        opt("alignment", "left/center/right/justify; áp cho từng đoạn vừa chèn"),
        opt("break", "ngắt đoạn sau khối; mặc định true khi có `alignment`"), agent=True, undo=True)
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
command("writer.replaceMany", "wps", replace_many,
        "Thay NHIỀU chỗ trong một lần: `edits` = [{find, replace}] theo thứ tự (mỗi cặp thay mọi chỗ khớp) - dùng thay cho nhiều lần writer.replaceAll; trả số chỗ đã thay của từng cặp",
        req("edits", "array of {find, replace}; up to 200"), agent=True, undo=True)
def outline(env, params):
    doc = env.document
    max_chars = max(40, values.integer(params, "maxChars", 160))
    items = []
    index = 0
    for element in _body_elements(doc):
        index += 1
        if _is_table(element):
            rows = table_rows(element)
            header = [c[:40] for c in rows[0]] if rows else []
            items.append({"n": index, "kind": "table", "rows": len(rows),
                          "cols": len(rows[0]) if rows else 0, "header": header})
            continue
        text = (element.getString() or "").strip()
        if not text:
            continue
        short = text if len(text) <= max_chars else text[:max_chars] + "..."
        entry = {"n": index, "t": short}
        try:
            style = str(element.getPropertyValue("ParaStyleName") or "")
            if style and style.lower() not in ("standard", "text body", "default paragraph style"):
                entry["style"] = style
        except Exception:
            pass
        items.append(entry)
    return {"paragraphs": index, "items": items}


def _design_unsupported(env, params):
    # Trinh bay lai tai lieu (tool doc_design) moi co tren Word (CommandDispatcher.WriterDesign.cs): khoi bo tron
    # dung bang InsertXML + wps shape. Ban LibreOffice lam sau.
    raise values.ParamError("not available on LibreOffice yet: format with writer.formatSelection/writer.formatTable")


def _lock_unsupported(env, params):
    # Khoa vung boi den moi co tren Word (CommandDispatcher.WriterScope.cs); ban LibreOffice lam sau.
    raise values.ParamError("not available on LibreOffice yet")


command("writer.lockSelection", "wps", _lock_unsupported,
        "Khoá vùng đang bôi đen: tới khi mở khoá, lệnh writer.* chỉ được đổi chữ trong vùng đó (chars = 0: không bôi đen, không khoá)")
command("writer.unlockSelection", "wps", _lock_unsupported, "Mở khoá vùng bôi đen")
command("writer.outline", "wps", outline,
        "Dàn bài tài liệu: các đoạn (số thứ tự toàn tài liệu, chữ rút gọn, số tự động) và bảng (số hàng, cột, hàng tiêu đề)",
        opt("maxChars", "characters per paragraph (default 160)"), agent=True)
command("writer.restyle", "wps", _design_unsupported,
        "Trình bày lại nền chung KHÔNG đổi chữ: tiêu đề mục (I., A., *), tiêu đề con (1.), nhãn (a. ...:), gạch đầu dòng hai cấp, bỏ dòng trống thừa, bảng (viền mảnh, hàng tiêu đề nhạt lặp lại), lề trang, số trang",
        opt("accent", "#RRGGBB headings (default #2E5C8A)"), opt("font", "default Times New Roman"), opt("size", "body size (default 13)"), undo=True)
command("writer.insertBlocks", "wps", _design_unsupported,
        "Chèn khối bo tròn nền nhạt (ô ghi chú, cảnh báo, thẻ cạnh nhau) - thay các đoạn [replaceFrom, replaceTo] hoặc chèn trước đoạn `before`",
        req("blocks", "array of {before | replaceFrom, replaceTo, kind: box|row, gap, after, cells: [{tone, lines: [{k: title|bullet|text|small|center, t, size, bold}]}]}"),
        opt("accent"), opt("font"), undo=True)
command("writer.undo", "wps", undo, "Hoàn tác (mỗi thao tác AI = 1 bước)", opt("count"), agent=True)
command("writer.exportPdf", "wps", lambda env, p: documents.export_pdf(env.document, "wps", values.string(p, "path")),
        "Xuất PDF", req("path"), agent=True)
command("writer.save", "wps", lambda env, p: documents.save(env.document, "wps", None), "Lưu", agent=True)
command("writer.saveAs", "wps", lambda env, p: documents.save(env.document, "wps", values.string(p, "path")),
        "Lưu thành file mới", req("path"), agent=True)
command("writer.closeAll", "wps", lambda env, p: documents.close_all(env.ctx, "wps"), "**Đóng mọi tài liệu, không lưu**")
