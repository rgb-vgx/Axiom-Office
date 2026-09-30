"""QA cau truc (chi doc) - cung quy tac voi CommandDispatcher.Checks.cs (LibreOffice_arch.md muc 7): agent
"mu", chi doc duoc text, nen sau khi sua no doc lai bang so de tu soat (tran chu, shape chong, o trong...)."""
from __future__ import annotations

import uno

from . import values
from .commands import command, opt

LAYOUT_TOLERANCE = 2.0        # point
MIN_FONT_SIZE = 12.0          # point
DENSE_SLIDE_CHARS = 600
LONG_HEADER_CHARS = 40
HMM_TO_POINT = 1 / values.POINT_TO_HMM


def issue(kind, detail, **extra):
    item = {"type": kind, "detail": detail}
    item.update(extra)
    return item


def column_letter(column: int) -> str:
    letter = ""
    while column > 0:
        letter = chr(ord("A") + (column - 1) % 26) + letter
        column = (column - 1) // 26
    return letter


def address(bounds) -> str:
    """"A1:C10" tu com.sun.star.table.CellRangeAddress."""
    start = "%s%d" % (column_letter(bounds.StartColumn + 1), bounds.StartRow + 1)
    end = "%s%d" % (column_letter(bounds.EndColumn + 1), bounds.EndRow + 1)
    return start if start == end else start + ":" + end


# ---------------------------------------------------------------- Writer

def _cell_text(raw: str) -> str:
    return (raw or "").replace("\r", "").replace("\x07", "").strip()


def writer_check_tables(env, params):
    from . import writer

    doc = env.document
    tables = writer.tables_in_order(doc)
    issues, report = [], []
    for index, table in enumerate(tables, start=1):
        rows = table.getRows().getCount()
        cols = table.getColumns().getCount()
        empty, header = 0, []
        for r in range(rows):
            for c in range(cols):
                try:
                    text = _cell_text(table.getCellByPosition(c, r).getString())
                except Exception:  # noqa: BLE001 - o bi gop
                    continue
                if not text:
                    empty += 1
                if r == 0:
                    header.append(text)
        if empty:
            issues.append(issue("empty-cells", "table %d has %d empty cell(s)" % (index, empty), table=index))
        lengths = sorted(len(h) for h in header)
        if len(header) > 1:
            typical = lengths[len(lengths) // 2]
            for text in header:
                if len(text) > LONG_HEADER_CHARS and len(text) > 3 * max(1, typical):
                    issues.append(issue("long-header-cell",
                        "table %d has a header cell with %d characters ('%s'): text that should be outside the table may have been typed into it"
                        % (index, len(text), text[:40]), table=index))
        report.append({"index": index, "rows": rows, "cols": cols, "emptyCells": empty, "header": header})
    return {"tableCount": len(tables), "tables": report, "issueCount": len(issues), "issues": issues}


# ---------------------------------------------------------------- Calc

def _cell(sheet, col: int, row: int):
    return sheet.getCellByPosition(col, row)


def _format_string(doc, key: int) -> str:
    try:
        return doc.getNumberFormats().getByKey(key).FormatString
    except Exception:  # noqa: BLE001
        return ""


def _cell_type(cell) -> str:
    """com.sun.star.table.CellContentType -> ten ('EMPTY'/'VALUE'/'TEXT'/'FORMULA'); pyuno tra ve uno.Enum."""
    content = cell.getType()
    return str(getattr(content, "value", content)).upper()


def range_report(env, params):
    doc = env.document
    from . import calc

    sheet = calc._sheet(doc, params)  # noqa: SLF001 - cung quy tac chon sheet voi et.readRange
    used = sheet.createCursor()
    used.gotoStartOfUsedArea(False)
    used.gotoEndOfUsedArea(True)
    used_bounds = used.getRangeAddress()
    requested = values.string(params, "range")
    if requested:
        try:
            bounds = sheet.getCellRangeByName(requested).getRangeAddress()
        except Exception:  # noqa: BLE001
            raise values.ParamError("'range' must be a valid address like 'A1:C10', got '%s'" % requested)
    else:
        bounds = used_bounds
    rows = bounds.EndRow - bounds.StartRow + 1
    cols = bounds.EndColumn - bounds.StartColumn + 1

    columns, issues = [], []
    for c in range(bounds.StartColumn, bounds.EndColumn + 1):
        letter = column_letter(c + 1)
        header = _cell(sheet, c, bounds.StartRow)
        header_text = _cell_text(header.getString())
        numbers = texts = blanks = numeric_texts = errors = formulas = fractional = 0
        for r in range(bounds.StartRow + 1, bounds.EndRow + 1):
            cell = _cell(sheet, c, r)
            content = _cell_type(cell)
            formula = cell.getFormula()
            if isinstance(formula, str) and formula.startswith("="):
                formulas += 1
            text = _cell_text(cell.getString())
            number = _parse_number(text)
            if content == "EMPTY" or not text:
                blanks += 1
            elif _is_error(cell, text):
                errors += 1
            elif content == "TEXT":
                # So nam trong o dang chu: Excel/Calc khong tinh vao cong thuc -> canh bao.
                texts += 1
                if number is not None:
                    numeric_texts += 1
            elif number is None:
                texts += 1  # VALUE/FORMULA tra ve chuoi
            else:
                numbers += 1
                if abs(number - round(number)) > 1e-9:
                    fractional += 1
        key = _cell(sheet, c, min(bounds.StartRow + 1, bounds.EndRow)).getPropertyValue("NumberFormat")
        fmt = _format_string(doc, key)
        data_cells = numbers + texts + errors
        if not header_text and data_cells > 0:
            issues.append(issue("empty-header", "column %s has data but no header in the first row" % letter))
        if numeric_texts > 0 and numbers + numeric_texts >= texts:
            issues.append(issue("numbers-as-text", "column %s has %d number(s) stored as text: write them as JSON numbers" % (letter, numeric_texts)))
        elif numbers > 0 and texts > 0:
            issues.append(issue("mixed-types", "column %s mixes %d number(s) and %d text value(s)" % (letter, numbers, texts)))
        if errors > 0:
            issues.append(issue("error-values", "column %s has %d error value(s) (#DIV/0!, #REF!...)" % (letter, errors)))
        if fractional > 0 and (not fmt or fmt.lower() == "general"):
            issues.append(issue("no-number-format",
                "column %s has decimals shown with the General format: apply numFmt such as \"0.0\" or \"#,##0.00\"" % letter))
        columns.append({"column": letter, "header": header_text or None, "numbers": numbers, "texts": texts,
                        "blanks": blanks, "formulas": formulas, "numberFormat": fmt})

    region_address = ""
    try:
        region = sheet.createCursorByRange(_cell(sheet, bounds.StartColumn, bounds.StartRow))
        region.collapseToCurrentRegion()
        region_address = address(region.getRangeAddress())
        region_bounds = region.getRangeAddress()
        used_cells = (used_bounds.EndRow - used_bounds.StartRow + 1) * (used_bounds.EndColumn - used_bounds.StartColumn + 1)
        region_cells = (region_bounds.EndRow - region_bounds.StartRow + 1) * (region_bounds.EndColumn - region_bounds.StartColumn + 1)
        if not requested and used_cells > region_cells:
            issues.append(issue("outside-table",
                "the sheet has content outside the table %s (used range %s): check for stray values written to the wrong cells"
                % (region_address, address(used_bounds))))
    except Exception:  # noqa: BLE001
        pass

    return {"sheet": sheet.getName(), "range": address(bounds), "usedRange": address(used_bounds), "table": region_address,
            "rows": rows, "cols": cols, "columns": columns, "issueCount": len(issues), "issues": issues}


ERROR_TEXT = ("#DIV/0!", "#REF!", "#NAME?", "#NUM!", "#NULL!", "#VALUE!", "#N/A", "#ERR:")


def _parse_number(text: str):
    """Chuoi hien thi -> so (bo dau phan nghin kieu en-US); khong phai so thi None."""
    cleaned = text.replace("\xa0", "").replace(" ", "").replace(",", "")
    try:
        return float(cleaned)
    except ValueError:
        return None


def _is_error(cell, text: str) -> bool:
    try:
        if cell.getError() != 0:
            return True
    except Exception:  # noqa: BLE001
        pass
    return text.startswith(ERROR_TEXT)


# ---------------------------------------------------------------- Impress

def _shape_text_height(shape) -> float:
    """Chieu cao chu can (point): bat TextAutoGrowHeight tam de LibreOffice do roi tra lai."""
    try:
        was = shape.TextAutoGrowHeight
    except Exception:  # noqa: BLE001
        return 0.0
    try:
        shape.TextAutoGrowHeight = False
        shape.TextAutoGrowHeight = True
        return shape.Size.Height * HMM_TO_POINT
    except Exception:  # noqa: BLE001
        return 0.0
    finally:
        try:
            shape.TextAutoGrowHeight = was
        except Exception:  # noqa: BLE001
            pass


def _min_font(shape) -> float:
    """Co nho nhat trong cac portion; khong doc duoc thi lay co cua ca shape."""
    try:
        text = shape.getText()
        sizes = []
        for paragraph in text.createEnumeration():
            for portion in paragraph.createEnumeration():
                height = portion.CharHeight
                if height:
                    sizes.append(float(height))
        if not sizes and text.CharHeight:
            sizes.append(float(text.CharHeight))
        return min(sizes) if sizes else 0.0
    except Exception:  # noqa: BLE001
        return 0.0


def _shape_text(shape) -> str:
    try:
        if shape.supportsService("com.sun.star.drawing.Text"):
            return shape.getText().getString()
    except Exception:  # noqa: BLE001
        pass
    return ""


# Placeholder trong (khong chu) cua layout khong hien khi trinh chieu: khong tinh (tranh bao chong gia).
PLACEHOLDERS = ("com.sun.star.presentation.TitleTextShape", "com.sun.star.presentation.SubtitleShape",
                "com.sun.star.presentation.OutlinerShape", "com.sun.star.presentation.NotesShape")


def _is_empty_placeholder(shape, chars: int) -> bool:
    """IsEmptyPresentationObject dung nhu Shape.Type == msoPlaceholder cua ban Windows (tao anh placeholder
    cua template khong mang ten service TitleTextShape/OutlinerShape nen phai doc thuoc tinh nay truoc)."""
    if chars:
        return False
    try:
        return bool(shape.getPropertyValue("IsEmptyPresentationObject"))
    except Exception:  # noqa: BLE001
        pass
    try:
        return any(shape.supportsService(name) for name in PLACEHOLDERS)
    except Exception:  # noqa: BLE001
        return False


def _box(shape, index):
    position, size = shape.Position, shape.Size
    chars = len(_shape_text(shape))
    text_height = _shape_text_height(shape) if chars else 0.0
    return {"name": shape.Name or ("shape %d" % index), "left": position.X * HMM_TO_POINT, "top": position.Y * HMM_TO_POINT,
            "width": size.Width * HMM_TO_POINT, "height": size.Height * HMM_TO_POINT, "chars": chars,
            "textHeight": text_height, "minFont": _min_font(shape) if chars else 0.0}


def _overlap(a, b) -> float:
    width = min(a["left"] + a["width"], b["left"] + b["width"]) - max(a["left"], b["left"])
    height = min(a["top"] + max(a["height"], a["textHeight"]), b["top"] + max(b["height"], b["textHeight"])) - max(a["top"], b["top"])
    return width * height if width > 0 and height > 0 else 0.0


def _layout_issues(boxes, slide_width, slide_height):
    issues = []
    total_chars = 0
    for box in boxes:
        total_chars += box["chars"]
        if box["chars"] and box["textHeight"] and (box["textHeight"] > box["height"] + LAYOUT_TOLERANCE):
            issues.append(issue("overflow",
                "text of '%s' needs %dpt height but the box is %dpt: shorten the text, lower fontSize or enlarge the box"
                % (box["name"], round(box["textHeight"]), round(box["height"])), shapes=[box["name"]]))
        bottom = box["top"] + max(box["height"], box["textHeight"])
        if (box["left"] < -LAYOUT_TOLERANCE or box["top"] < -LAYOUT_TOLERANCE
                or box["left"] + box["width"] > slide_width + LAYOUT_TOLERANCE or bottom > slide_height + LAYOUT_TOLERANCE):
            issues.append(issue("offslide", "'%s' goes outside the slide (%dx%dpt)" % (box["name"], round(slide_width), round(slide_height)),
                                shapes=[box["name"]]))
        if box["chars"] and 0 < box["minFont"] < MIN_FONT_SIZE:
            issues.append(issue("small-font", "'%s' uses %gpt text; keep body text at %gpt or more" % (box["name"], box["minFont"], MIN_FONT_SIZE),
                                shapes=[box["name"]]))
    slide_area = slide_width * slide_height
    content = [b for b in boxes if b["width"] * b["height"] < slide_area * 0.9]
    for i, first in enumerate(content):
        for second in content[i + 1:]:
            overlap = _overlap(first, second)
            smaller = min(first["width"] * first["height"], second["width"] * second["height"])
            if smaller > 0 and overlap > smaller * 0.1:
                issues.append(issue("overlap", "'%s' and '%s' overlap (%d%% of the smaller shape)"
                                    % (first["name"], second["name"], round(100 * overlap / smaller)),
                                    shapes=[first["name"], second["name"]]))
    if total_chars > DENSE_SLIDE_CHARS:
        issues.append(issue("dense", "the slide has %d characters; keep one message per slide and move details to notes (wpp.setNotes)"
                            % total_chars))
    return issues


def layout_report(env, params):
    doc = env.document
    pages = doc.getDrawPages()
    count = pages.getCount()
    only = values.integer(params, "slide", 0)
    if only < 0 or only > count:
        raise values.ParamError("'slide' must be between 1 and %d, got %d" % (count, only))
    slide_width = pages.getByIndex(0).Width * HMM_TO_POINT if count else 0.0
    slide_height = pages.getByIndex(0).Height * HMM_TO_POINT if count else 0.0
    slides, issue_count = [], 0
    for index in range(only - 1 if only else 0, only if only else count):
        page = pages.getByIndex(index)
        boxes = []
        for s in range(page.getCount()):
            shape = page.getByIndex(s)
            box = _box(shape, s + 1)
            if _is_empty_placeholder(shape, box["chars"]):
                continue
            if box["chars"] or box["width"] or box["height"]:
                boxes.append(box)
        issues = _layout_issues(boxes, slide_width, slide_height)
        issue_count += len(issues)
        slides.append({"index": index + 1, "shapes": [{k: (round(v, 1) if isinstance(v, float) else v)
                                                       for k, v in box.items() if k in ("name", "left", "top", "width", "height", "chars")}
                                                      for box in boxes], "issues": issues})
    return {"slideWidth": round(slide_width, 1), "slideHeight": round(slide_height, 1),
            "issueCount": issue_count, "slides": slides}


command("writer.checkTables", "wps", writer_check_tables,
        "Soát các bảng (chỉ đọc): số dòng/cột, ô trống, ô tiêu đề lẫn đoạn văn", agent=True)
command("et.checkRange", "et", range_report,
        "Soát bảng dữ liệu (chỉ đọc): tiêu đề trống, kiểu lẫn lộn, số dạng chữ, number format, dữ liệu lạc ngoài bảng",
        opt("range", "default: the used range"), opt("sheet"), agent=True)
command("wpp.checkLayout", "wpp", layout_report,
        "Soát bố cục slide (chỉ đọc): chữ tràn khung, shape ra ngoài slide, shape chồng nhau, chữ quá nhỏ",
        opt("slide", "slide number; default: every slide"), agent=True)
