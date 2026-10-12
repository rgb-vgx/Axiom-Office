"""QA cau truc (chi doc) - cung quy tac voi CommandDispatcher.Checks.cs (LibreOffice_arch.md muc 7): agent
"mu", chi doc duoc text, nen sau khi sua no doc lai bang so de tu soat (tran chu, shape chong, o trong...)."""
from __future__ import annotations

import re

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
        # Bang dan trang (do formatTable {borders:false} danh dau - xem writer.LAYOUT_TABLE_NAME): o trong
        # la CO Y (khoang trong ky, o cho dien) nen KHONG bao "empty-cells"; so lieu van de trong report de
        # agent thay. "long-header-cell" thi bao cho moi bang: no bat chu lot vao o tieu de, ma voi bang dan
        # trang thi do cung la loi that (van-ban-hanh-chinh van doi soat no).
        layout = writer.is_layout_table(table)
        if empty and not layout:
            issues.append(issue("empty-cells", "table %d has %d empty cell(s)" % (index, empty), table=index))
        # Moi o tieu de so voi trung vi cac o tieu de CON LAI (leave-one-out), khong so voi trung vi ca nhom:
        # voi bang 2 cot, trung vi ca nhom chinh la o dai hon nen nguong "3 x trung vi" khong bao gio voi toi
        # (docs/skills-review.md muc 4.7) - mot dieu kien khong bao gio sai thi khong phai la kiem tra.
        # Bang 1 cot khong co nhom de so, nen khong bao (khong co ti le nao de noi o do dai BAT THUONG).
        for position, text in enumerate(header):
            others = sorted(len(other) for at, other in enumerate(header) if at != position)
            if not others:
                continue
            typical = others[len(others) // 2]
            if len(text) > LONG_HEADER_CHARS and len(text) > 3 * max(1, typical):
                issues.append(issue("long-header-cell",
                    "table %d has a header cell with %d characters ('%s'): text that should be outside the table may have been typed into it"
                    % (index, len(text), text[:40]), table=index))
        report.append({"index": index, "rows": rows, "cols": cols, "emptyCells": empty, "header": header,
                       "layout": layout})
    return {"tableCount": len(tables), "tables": report, "issueCount": len(issues), "issues": issues}


# ---------------------------------------------------------------- Calc

def _cell(sheet, col: int, row: int):
    return sheet.getCellByPosition(col, row)


def _format_string(doc, key: int) -> str:
    try:
        return doc.getNumberFormats().getByKey(key).FormatString
    except Exception:  # noqa: BLE001
        return ""


def _error_name(cell, text: str) -> str:
    """Ten loi hien tren o: 'Err:508' (loi cu phap) hoac '#DIV/0!' (loi luc tinh). Du phong ma so."""
    if text:
        return text
    code = cell.getError()
    return "Err:%d" % code if code else "error"


def _cell_type(cell) -> str:
    """com.sun.star.table.CellContentType -> ten ('EMPTY'/'VALUE'/'TEXT'/'FORMULA'); pyuno tra ve uno.Enum."""
    content = cell.getType()
    return str(getattr(content, "value", content)).upper()


def _formula_result_type(cell):
    """Ket qua cong thuc (log 11/10/2026): 1 = so, 2 = chuoi, 4 = loi; khong doc duoc thi None."""
    try:
        value = getattr(cell, "FormulaResultType2", None)
        if value is not None:
            return int(getattr(value, "value", value))
    except Exception:  # noqa: BLE001
        pass
    try:
        value = cell.getPropertyValue("FormulaResultType2")
        if value is not None:
            return int(getattr(value, "value", value))
    except Exception:  # noqa: BLE001
        pass
    return None


# Gioi han khi soi tham chieu: mot bang 50.000 dong co the co hang tram nghin cong thuc, soi het thi cham
# ma cai can thay chi la vai cho. Lay mau, va bao ca con so tong de nguoi doc biet quy mo.
REF_SCAN_LIMIT = 4000
REF_SAMPLE_LIMIT = 40

# Tham chieu toi MOT o: co the co ten sheet phia truoc (Inputs.B34, $'My Sheet'.$B$34), va co the co $.
# Log 11/10/2026: "=SUMIFS(...;\"01B1*\")" bao gia B1 va LOG10/ATAN2 bi doc thanh o, nen chi nhan o ranh gioi tu.
_REFERENCE = re.compile(r"(?<![A-Za-z0-9_])(?:(?P<sheet>\$?'[^']+'|\$?[A-Za-z_][A-Za-z0-9_ ]*)\.)?"
                        r"(?P<abs1>\$?)(?P<col>[A-Z]{1,3})(?P<abs2>\$?)(?P<row>\d+)(?![A-Za-z0-9_(])")
# Chuoi LibreOffice: "..." voi "" la dau nhay kep thoat, can che truoc khi soi tham chieu.
_STRING_LITERAL = re.compile(r'"(?:[^"]|"")*"')


def _column_index(letters: str) -> int:
    value = 0
    for ch in letters.upper():
        value = value * 26 + (ord(ch) - ord("A") + 1)
    return value - 1


def _empty_references(doc, sheet, bounds) -> list:
    """Vi du ve cong thuc tro vao MOT o dang TRONG.

    Vi sao can: mot o dieu khien dat canh nhan cua no (vd "Random Seed" o B17) nhung cong thuc lo tro vao
    o ben canh hoac o tren (B16/B34) thi khong co gi bao loi - phep tinh coi o trong la 0 va chay tiep.
    Do ngay 02/10/2026 tren de Monte Carlo: mot workbook 10.000 duong tu bao 18/18 PASS trong khi seed
    that su bang 0, vi 80.008 cong thuc doc o tieu de muc thay vi o gia tri. Doc lai KHONG phat hien duoc;
    phai soi xem cong thuc tro vao dau.

    Bo qua vung (A1:B2) vi hai dau cua vung deu la tham chieu hop le, va bo qua ten sheet khong ton tai.
    """
    found = []
    scanned = 0
    sheets = doc.getSheets()
    for r in range(bounds.StartRow, bounds.EndRow + 1):
        for c in range(bounds.StartColumn, bounds.EndColumn + 1):
            if scanned >= REF_SCAN_LIMIT or len(found) >= REF_SAMPLE_LIMIT:
                return found
            formula = _cell(sheet, c, r).getFormula()
            if not isinstance(formula, str) or not formula.startswith("="):
                continue
            scanned += 1
            # Bo qua noi dung trong chuoi "..." (ghi de bang khoang trang de giu vi tri), tranh doc "B1"
            # trong "01B1*" thanh tham chieu; log 11/10/2026.
            cleaned = _STRING_LITERAL.sub(lambda m: " " * len(m.group(0)), formula)
            for match in _REFERENCE.finditer(cleaned):
                start, end = match.span()
                # A1:B2 - ca hai dau deu la tham chieu hop le, khong phai tro nham.
                if (start > 0 and cleaned[start - 1] == ":") or (end < len(cleaned) and cleaned[end] == ":"):
                    continue
                # Ranh gioi tu: khong dinh lien chu/so/_ phia truoc, khong phai ten ham truoc "(".
                if start > 0 and (cleaned[start - 1].isalnum() or cleaned[start - 1] == "_"):
                    continue
                if end < len(cleaned) and (cleaned[end] == "(" or cleaned[end].isalnum() or cleaned[end] == "_"):
                    continue
                target_sheet = sheet
                name = match.group("sheet")
                if name:
                    try:
                        target_sheet = sheets.getByName(name.lstrip("$").strip("'"))
                    except Exception:  # noqa: BLE001
                        continue
                column = _column_index(match.group("col"))
                row = int(match.group("row")) - 1
                try:
                    target = target_sheet.getCellByPosition(column, row)
                except Exception:  # noqa: BLE001
                    continue
                if _cell_type(target) != "EMPTY":
                    continue
                found.append("%s!%s%d -> %s!%s%d (trong)" % (
                    sheet.getName(), column_letter(c + 1), r + 1,
                    target_sheet.getName(), column_letter(column + 1), row + 1))
                break
    return found


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

    # Nhan dong tieu de (log 11/10/2026): dong dau chi co dung mot o co chu (dang TEXT) va dong ke
    # tiep co chu o it nhat mot nua so cot thi dong dau la tieu de, dong ke tiep moi la header.
    title = None
    header_row = bounds.StartRow
    if rows >= 2 and cols >= 2:
        first_filled = []
        for c in range(bounds.StartColumn, bounds.EndColumn + 1):
            cell = _cell(sheet, c, bounds.StartRow)
            text = _cell_text(cell.getString())
            if text:
                first_filled.append((c, text, _cell_type(cell)))
        if len(first_filled) == 1 and first_filled[0][2] == "TEXT":
            second_texts = 0
            for c in range(bounds.StartColumn, bounds.EndColumn + 1):
                cell = _cell(sheet, c, bounds.StartRow + 1)
                text = _cell_text(cell.getString())
                if not text:
                    continue
                content = _cell_type(cell)
                if content == "TEXT":
                    second_texts += 1
                elif content == "FORMULA" and _formula_result_type(cell) == 2:
                    second_texts += 1
                elif content not in ("EMPTY", "VALUE", "FORMULA"):
                    second_texts += 1
            if second_texts * 2 >= cols:
                title = first_filled[0][1]
                header_row = bounds.StartRow + 1
    data_start = header_row + 1

    columns, issues = [], []
    for c in range(bounds.StartColumn, bounds.EndColumn + 1):
        letter = column_letter(c + 1)
        header = _cell(sheet, c, header_row)
        header_text = _cell_text(header.getString())
        numbers = texts = blanks = numeric_texts = errors = formulas = fractional = 0
        error_names = []
        for r in range(data_start, bounds.EndRow + 1):
            cell = _cell(sheet, c, r)
            content = _cell_type(cell)
            formula = cell.getFormula()
            if isinstance(formula, str) and formula.startswith("="):
                formulas += 1
            text = _cell_text(cell.getString())
            if content == "EMPTY" or not text:
                blanks += 1
            elif _is_error(cell, text):
                errors += 1
                error_names.append(_error_name(cell, text))
            elif content == "TEXT":
                # So nam trong o dang chu: Excel/Calc khong tinh vao cong thuc -> canh bao.
                texts += 1
                if _parse_number(text) is not None:
                    numeric_texts += 1
            elif content == "VALUE":
                # Phan loai theo gia tri that (log 11/10/2026): o VALUE la so du hien thi "26/03/2026".
                numbers += 1
                try:
                    value = float(cell.getValue())
                except Exception:  # noqa: BLE001
                    value = _parse_number(text)
                if value is not None and abs(value - round(value)) > 1e-9:
                    fractional += 1
            elif content == "FORMULA":
                # Phan loai theo kieu ket qua that (log 11/10/2026): 1 = so, 2 = chuoi, 4 = loi.
                result = _formula_result_type(cell)
                if result == 4:
                    errors += 1
                    error_names.append(_error_name(cell, text))
                elif result == 1:
                    numbers += 1
                    try:
                        value = float(cell.getValue())
                    except Exception:  # noqa: BLE001
                        value = _parse_number(text)
                    if value is not None and abs(value - round(value)) > 1e-9:
                        fractional += 1
                elif result == 2:
                    texts += 1
                    if _parse_number(text) is not None:
                        numeric_texts += 1
                else:
                    number = _parse_number(text)
                    if number is None:
                        texts += 1  # FORMULA tra ve chuoi khi khong doc duoc kieu ket qua
                    else:
                        numbers += 1
                        try:
                            value = float(cell.getValue())
                        except Exception:  # noqa: BLE001
                            value = number
                        if value is not None and abs(value - round(value)) > 1e-9:
                            fractional += 1
            else:
                number = _parse_number(text)
                if number is None:
                    texts += 1
                else:
                    numbers += 1
                    try:
                        value = float(cell.getValue())
                    except Exception:  # noqa: BLE001
                        value = number
                    if value is not None and abs(value - round(value)) > 1e-9:
                        fractional += 1
        # numberFormat lay tu dong du lieu dau (log 11/10/2026), khong lay o dong header.
        fmt_row = data_start if data_start <= bounds.EndRow else header_row
        key = _cell(sheet, c, fmt_row).getPropertyValue("NumberFormat")
        fmt = _format_string(doc, key)
        data_cells = numbers + texts + errors
        if not header_text and data_cells > 0:
            issues.append(issue("empty-header", "column %s has data but no header in the first row" % letter))
        if numeric_texts > 0 and numbers + numeric_texts >= texts:
            issues.append(issue("numbers-as-text", "column %s has %d number(s) stored as text: write them as JSON numbers" % (letter, numeric_texts)))
        elif numbers > 0 and texts > 0:
            issues.append(issue("mixed-types", "column %s mixes %d number(s) and %d text value(s)" % (letter, numbers, texts)))
        if errors > 0:
            # Ma loi THAT (Err:508 = sai dau phan cach tham so, #DIV/0! = mau bang 0...), khong liet ke
            # vi du nua: doan mo ho lam nguoi doc di tim sai benh.
            issues.append(issue("error-values", "column %s has %d error value(s): %s" % (
                letter, errors, ", ".join(sorted(set(error_names))))))
        if fractional > 0 and (not fmt or fmt.lower() == "general"):
            issues.append(issue("no-number-format",
                "column %s has decimals shown with the General format: apply numFmt such as \"0.0\" or \"#,##0.00\"" % letter))
        columns.append({"column": letter, "header": header_text or None, "numbers": numbers, "texts": texts,
                        "blanks": blanks, "formulas": formulas, "numberFormat": fmt})

    # Cong thuc tro vao MOT o dang TRONG: khong co gi bao loi, phep tinh coi o trong la 0 va chay tiep.
    # Day la cach mot o dieu khiển "chet" ma khong ai biet - xem _empty_references.
    try:
        empty = _empty_references(doc, sheet, bounds)
    except Exception:  # noqa: BLE001 - soat them khong duoc lam hong ca bao cao
        empty = []
    if empty:
        issues.append(issue("empty-reference",
            "%d cong thuc tro vao mot o dang TRONG (vi du: %s). O trong duoc tinh la 0, nen mot o dieu "
            "khien bi tro sai cho se im lang chay bang 0 - hay kiem tra xem co phai cong thuc dang tro "
            "vao o tieu de / o ben canh thay vi o gia tri khong" % (len(empty), "; ".join(empty[:6])),
            examples=empty[:6]))

    region_address = ""
    try:
        region = sheet.createCursorByRange(_cell(sheet, bounds.StartColumn, bounds.StartRow))
        region.collapseToCurrentRegion()
        region_address = address(region.getRangeAddress())
        region_bounds = region.getRangeAddress()
        used_cells = (used_bounds.EndRow - used_bounds.StartRow + 1) * (used_bounds.EndColumn - used_bounds.StartColumn + 1)
        region_cells = (region_bounds.EndRow - region_bounds.StartRow + 1) * (region_bounds.EndColumn - region_bounds.StartColumn + 1)
        if not requested and used_cells > region_cells:
            # Cau chu phai noi ro HAI kha nang: noi dung ngoai vung bang co the la du lieu NGUOI DUNG da co
            # (bang thu hai, ghi chu ben canh), khong chi la gia tri ghi lac. Loi dan cu "check for stray
            # values" bi doc thanh "xoa cac o do" va skill bao-cao-du-lieu noi thang ra dieu do - log
            # 11/10/2026. Muon soat rieng bang vua dung thi truyen 'range'.
            issues.append(issue("outside-table",
                "the sheet has content outside the table %s (used range %s): this may be data the user "
                "already had (a second table, notes beside the table) or values written to the wrong "
                "cells - do NOT clear a cell that already had data before this turn; to check only the "
                "table you just built, call et.checkRange with 'range'" % (region_address, address(used_bounds))))
    except Exception:  # noqa: BLE001
        pass

    return {"sheet": sheet.getName(), "range": address(bounds), "usedRange": address(used_bounds), "table": region_address,
            "rows": rows, "cols": cols, "title": title, "columns": columns, "issueCount": len(issues), "issues": issues}


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
        "Soát các bảng (chỉ đọc): số dòng/cột, ô trống, ô tiêu đề lẫn đoạn văn. Bảng dàn trang — bảng đã gọi "
        "writer.formatTable với borders:false — được đánh dấu layout và KHÔNG báo empty-cells (ô trống ở đó "
        "là cố ý: khoảng trống ký, ô chờ điền)", agent=True)
command("et.checkRange", "et", range_report,
        "Soát bảng dữ liệu (chỉ đọc): tiêu đề trống, kiểu lẫn lộn, số dạng chữ, number format, dữ liệu lạc ngoài bảng "
        "(chỉ khi không truyền range)", opt("range", "default: the used range"), opt("sheet"), agent=True)
command("wpp.checkLayout", "wpp", layout_report,
        "Soát bố cục slide (chỉ đọc): chữ tràn khung, shape ra ngoài slide, shape chồng nhau, chữ quá nhỏ",
        opt("slide", "slide number; default: every slide"), agent=True)
