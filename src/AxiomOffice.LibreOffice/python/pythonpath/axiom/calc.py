"""Lenh Calc (kind "et") - cung ten/ket qua voi CommandDispatcher.Spreadsheet.cs (LibreOffice_arch.md muc 7.2).

Khac Excel mot diem: LibreOffice hoan tac duoc bang undo manager nen et.undo la undo that.
"""
from __future__ import annotations

import hashlib

import uno

from . import documents, values
from .commands import command, opt, req

BOLD, NORMAL = 150.0, 100.0
ALIGN = {"left": "LEFT", "center": "CENTER", "right": "RIGHT", "justify": "BLOCK"}
PARSE_ERROR = 508      # Err:508 "Pair missing" - cong thuc khong doc duoc (thuong la sai dau phan cach)


def _sheets(doc):
    return doc.getSheets()


def _ensure_sheet(doc):
    """Calc trong (khong co sheet nao) thi tao mot sheet - giong EnsureWorkbook cua ban Windows."""
    sheets = _sheets(doc)
    if not sheets.getElementNames():
        sheets.insertNewByName("Sheet1", 0)
    return sheets


def _sheet(doc, params):
    sheets = _ensure_sheet(doc)
    name = values.string(params, "sheet")
    if name:
        if name not in tuple(sheets.getElementNames()):
            raise values.ParamError("no sheet named '%s' (sheets: %s)" % (name, ", ".join(sheets.getElementNames())))
        return sheets.getByName(name)
    return doc.getCurrentController().getActiveSheet()


def _active_name(doc):
    return _ensure_sheet(doc).getByName(doc.getCurrentController().getActiveSheet().getName()).getName()


def _range(sheet, address: str):
    try:
        return sheet.getCellRangeByName(address)
    except Exception:  # noqa: BLE001
        raise values.ParamError("'range' must be a valid address like 'A1:C10', got '%s'" % address)


def _to_matrix(data, empty_as_none: bool = True):
    """getDataArray -> list hang (so/chuoi; o trong -> None cho khop Value2 cua Excel)."""
    grid = data if isinstance(data, tuple) else (data,)
    out = []
    for row in grid:
        line = []
        for cell in row:
            if isinstance(cell, str):
                line.append(cell or (None if empty_as_none else ""))
            else:
                line.append(cell)
        out.append(line)
    return out


def _cell_text(value) -> str:
    if value is None:
        return ""
    if isinstance(value, bool):
        return "True" if value else "False"
    if isinstance(value, float) and value.is_integer():
        return str(int(value))
    return str(value)


def _number_format(doc, code: str) -> int:
    """Format code hieu theo locale en-US de '#,##0.00' giong Excel (queryKey/addNew nhan struct Locale)."""
    locale = uno.createUnoStruct("com.sun.star.lang.Locale")
    locale.Language, locale.Country = "en", "US"
    formats = doc.getNumberFormats()
    key = formats.queryKey(code, locale, False)
    if key == -1:
        key = formats.addNew(code, locale)
    return key


def _apply_font(target, params) -> None:
    if values.has(params, "bold"):
        target.CharWeight = BOLD if values.boolean(params, "bold", False) else NORMAL
    if values.has(params, "italic"):
        target.CharPosture = uno.Enum("com.sun.star.awt.FontSlant",
                                      "ITALIC" if values.boolean(params, "italic", False) else "NONE")
    if values.has(params, "fontSize"):
        target.CharHeight = float(values.number(params, "fontSize", 11))


def list_sheets(env, params):
    doc = env.document
    sheets = _ensure_sheet(doc)
    return {"workbook": doc.getTitle(), "activeSheet": _active_name(doc), "sheets": list(sheets.getElementNames())}


def activate_sheet(env, params):
    name = values.string(params, "sheet")
    if not name:
        raise values.ParamError("'sheet' is required")
    doc = env.document
    sheets = _ensure_sheet(doc)
    if name not in tuple(sheets.getElementNames()):
        raise values.ParamError("no sheet named '%s' (sheets: %s)" % (name, ", ".join(sheets.getElementNames())))
    doc.getCurrentController().setActiveSheet(sheets.getByName(name))
    return {"active": name}


def add_sheet(env, params):
    """Them sheet moi. Khong co lenh nay thi agent khong dung duoc so nhieu sheet tren tai lieu dang mo.

    Ten bo trong -> dat ten kieu Excel (Sheet1, Sheet2...) va tranh trung. `index` la vi tri chen (0-based,
    mac dinh la cuoi). Sheet moi duoc chuyen sang active de nguoi dung nhin thay no dang duoc dien.
    """
    doc = env.document
    sheets = _ensure_sheet(doc)
    existing = list(sheets.getElementNames())
    name = values.string(params, "name")
    if not name:
        # So nho nhat con trong (Sheet1, Sheet2...), giong ban C# - hai lan chay cung mot thu.
        index = 1
        while ("Sheet%d" % index) in existing:
            index += 1
        name = "Sheet%d" % index
    if name in existing:
        raise values.ParamError("a sheet named '%s' already exists (sheets: %s)" % (name, ", ".join(existing)))
    position = max(0, min(values.integer(params, "index", len(existing)), len(existing)))
    sheets.insertNewByName(name, position)
    doc.getCurrentController().setActiveSheet(sheets.getByName(name))
    return {"sheet": name, "sheets": list(sheets.getElementNames())}


def rename_sheet(env, params):
    """Doi ten sheet. LibreOffice tu cap nhat moi cong thuc tro tên cu, nen khong phai sua tay."""
    old = values.string(params, "sheet")
    new = values.string(params, "name")
    if not old:
        raise values.ParamError("'sheet' is required")
    if not new:
        raise values.ParamError("'name' is required")
    doc = env.document
    sheets = _ensure_sheet(doc)
    existing = list(sheets.getElementNames())
    if old not in existing:
        raise values.ParamError("no sheet named '%s' (sheets: %s)" % (old, ", ".join(existing)))
    if new in existing and new != old:
        raise values.ParamError("a sheet named '%s' already exists (sheets: %s)" % (new, ", ".join(existing)))
    sheets.getByName(old).setName(new)
    return {"sheet": new, "sheets": list(sheets.getElementNames())}


def read_range(env, params):
    address = values.string(params, "range", "A1") or "A1"
    doc = env.document
    range_ = _range(_sheet(doc, params), address)
    return {"sheet": _active_sheet_name(env, params), "range": address, "values": _to_matrix(range_.getDataArray())}


def _active_sheet_name(env, params):
    doc = env.document
    name = values.string(params, "sheet")
    return name if name else _active_name(doc)


# ---------------------------------------------------------------- cong thuc

def _error(cell) -> int:
    """Ma loi cua o (0 = khong loi). Co ngay sau setFormula, khong phai tinh lai ca bang."""
    try:
        return int(cell.getError())
    except Exception:  # noqa: BLE001 - ban khac khong ho tro
        return 0


def _with_separator(formula: str, target: str):
    """Doi dau phan cach tham so (',' hoac ';') trong cong thuc; None neu khong co gi phai doi.

    Bo qua phan trong chuoi "...": dau phay trong "Trung binh, kha" khong phai dau phan cach.
    Ngoac kep doi ("") de thoat ky tu tu xu ly duoc vi cap do tu mo roi dong lai.

    Luu y: doi MOI dau phan cach ngoai chuoi, ke ca trong mang hang {1;2} (mang cua Calc dung ';'
    giua cac cot) - nen chi goi khi cong thuc vua ghi da loi va con duong khac de thu.
    """
    chars, in_string, changed = [], False, False
    for ch in formula:
        if ch == '"':
            in_string = not in_string
            chars.append(ch)
        elif not in_string and ch in ",;":
            chars.append(target)
            changed = changed or ch != target
        else:
            chars.append(ch)
    return "".join(chars) if changed else None


def _write_formula(cell, formula: str, state: dict):
    """Ghi cong thuc, tu sua dau phan cach tham so khi LibreOffice khong doc duoc. -> (loi, da ghi).

    Khong phai ban LibreOffice nao cung nhan ',' lam dau phan cach tham so: ban 24.2 tren mot so may
    doi ';' - va muc Tools > Options > Calc > Formula > Separators de doi da bi bo - nen cong thuc
    kieu en-US do agent sinh ra bi Err:508 "pair missing". Excel qua COM thi luon dung ',' bat ke
    locale, nen ban Windows khong dinh loi nay.

    Chi thu doi dau khi gap DUNG loi cu phap 508: loi luc tinh (#DIV/0! = 532, #REF!...) thi dau phan
    cach da dung roi, doi sang kieu kia chi lam cong thuc hong them. Do mot lan roi nho vao `state`
    de khong phai thu lai tung o trong cung mot lan ghi.
    """
    known = state.get("separator")
    if known:
        formula = _with_separator(formula, known) or formula
    cell.setFormula(formula)
    error = _error(cell)
    if error != PARSE_ERROR or known:
        return error, formula
    for target in (",", ";"):
        swapped = _with_separator(formula, target)
        if not swapped:
            continue
        cell.setFormula(swapped)
        found = _error(cell)
        if found != PARSE_ERROR:            # 0 hoac loi luc tinh: da doc duoc cong thuc
            state["separator"] = target
            return found, swapped
    cell.setFormula(formula)                # doi dau khong giup gi -> tra lai dung ban agent viet
    return error, formula


def _cell_address(start_col: int, start_row: int, c: int, r: int) -> str:
    from .checks import column_letter     # import muon: checks nhap calc o trong ham

    return "%s%d" % (column_letter(start_col + c + 1), start_row + r + 1)


def write_range(env, params):
    address = values.string(params, "range")
    if not address:
        raise values.ParamError("'range' is required: the top-left cell to write at, e.g. 'A1'")
    rows = values.matrix(params, "values", True)
    width = max(1, max(len(r) for r in rows))
    doc = env.document
    sheet = _sheet(doc, params)
    top_left = _range(sheet, address)
    start_col, start_row = top_left.RangeAddress.StartColumn, top_left.RangeAddress.StartRow
    target = sheet.getCellRangeByPosition(start_col, start_row, start_col + width - 1, start_row + len(rows) - 1)

    # Ghi TUNG O: setDataArray voi phan tu rong (None) lam LibreOffice ghi loi #N/A vao o, con
    # setFormulaArray + setDataArray tren cung vung thi o cong thuc bi ghi de mat cong thuc.
    #
    # DUNG tam tat tinh lai tu dong (enableAutomaticCalculation) o day. Da thu ngay 02/10/2026 va DO
    # duoc la cham hon han: cung mot khoi ghi, ban tat tinh roi calculateAll() mot lan chay 28,0s / 130,6s
    # so voi 12,4s / 32,6s khi de nguyen (24.008 o va 48.008 o tren so moi). LibreOffice da gop viec tinh
    # lai lai san, con calculateAll() o cuoi lai keo theo mot luot tinh toan bo tai lieu.
    state: dict = {}
    errors = []
    for r, row in enumerate(rows):
        for c in range(width):
            value = row[c] if c < len(row) else None
            cell = target.getCellByPosition(c, r)
            if value is None:
                cell.setFormula("")            # xoa o (khong tao o chuoi rong)
            elif isinstance(value, bool):
                cell.setFormula("TRUE()" if value else "FALSE()")
            elif isinstance(value, str):
                if value.startswith("="):
                    error, written = _write_formula(cell, value, state)
                    if error:
                        errors.append({"cell": _cell_address(start_col, start_row, c, r),
                                       "formula": written, "error": error, "text": cell.getString()})
                else:
                    cell.setString(value)
            else:
                cell.setValue(float(value))

    result = {"written": len(rows) * width, "sheet": _active_sheet_name(env, params)}
    # Cong thuc hong thi bao ra day: truoc day ghi xong tra ve {"written": n} nen agent tuong da xong.
    if errors:
        result["formulaErrors"] = errors
    return result


def write_ranges(env, params):
    """Ghi NHIỀU vùng trong MỘT lời gọi: `writes` là mảng `{range, values, sheet?}`.

    Vì sao cần: đo được ở Test 2/3/4, model hay gọi 5–10 `et.writeRange` liên tiếp trong **cùng một
    phản hồi**. Mỗi lời gọi là một vòng qua bridge (HTTP + gate + một lượt model trả lời), nên gộp lại
    bớt được từng ấy vòng.

    KHÔNG hứa nhanh hơn về tính toán: tắt tính tự động rồi `calculateAll()` một lần đã được đo là CHẬM
    hơn 2,3–4,0 lần và đã hoàn tác (xem `write_range` và tests/bench/results/test2-gaps.md). Ở đây chỉ
    gộp lời gọi, không đụng tới cách LibreOffice tính lại.
    """
    writes = params.get("writes")
    if isinstance(writes, dict):      # bridge có thể bọc một lớp, hoặc nhận một vùng đơn lẻ
        for key in ("items", "item", "value"):
            if key in writes:
                writes = writes[key]
                break
        else:
            writes = [writes]
    if not isinstance(writes, list) or not writes:
        raise values.ParamError("'writes' is required: a list of {range, values, sheet?}, e.g. "
                                "[{\"range\": \"A1\", \"values\": [[\"Tên\"]]}, "
                                "{\"range\": \"B2\", \"sheet\": \"P&L\", \"values\": [[1, 2]]}]")

    # Kiểm HẾT trước khi ghi: vùng thứ ba sai thì hai vùng đầu không được vào file.
    for index, item in enumerate(writes):
        if not isinstance(item, dict):
            raise values.ParamError("writes[%d] must be an object like {range, values}, got %s"
                                    % (index, values.describe(item)))
        if not values.string(item, "range"):
            raise values.ParamError("writes[%d] needs 'range' (the top-left cell to write at)" % index)
        values.matrix(item, "values", True)

    written = 0
    sheets = []
    errors = []
    for item in writes:
        result = write_range(env, item)
        written += int(result.get("written") or 0)
        sheets.append(result.get("sheet"))
        for problem in result.get("formulaErrors") or []:
            problem = dict(problem)
            problem["sheet"] = result.get("sheet")
            errors.append(problem)

    out = {"ranges": len(writes), "written": written, "sheets": sorted({name for name in sheets if name})}
    if errors:
        out["formulaErrors"] = errors
    return out


def fill_range(env, params):
    """Viet MOT cong thuc vao o goc roi dien ra ca vung, tham chieu tuong doi tu dich.

    Vi sao can: chi phi token cua agent tinh theo TUNG O no viet ra. Mot bang 1000 dong x 8 cot viet
    tung o la ~8000 gia tri model phai phat ra; viet mot cong thuc roi dien thi chi con mot gia tri.
    Do ngay 02/10/2026 bang probe UNO rieng: fillAuto(TO_BOTTOM/TO_RIGHT, 1) dich tham chieu tuong doi
    dung (=$D$1+ROW()*B1 -> *B2 -> *B3; =F2*2 -> =G2*2 -> =H2*2).
    """
    address = values.string(params, "range")
    if not address:
        raise values.ParamError("'range' is required: the whole area to fill, e.g. 'B2:H1000'")
    doc = env.document
    sheet = _sheet(doc, params)
    target = _range(sheet, address)
    bounds = target.RangeAddress
    rows = bounds.EndRow - bounds.StartRow + 1
    cols = bounds.EndColumn - bounds.StartColumn + 1

    formula = values.string(params, "formula")
    errors = []
    if formula:
        corner = target.getCellByPosition(0, 0)
        if formula.startswith("="):
            error, written = _write_formula(corner, formula, {})
            if error:
                errors.append({"cell": _cell_address(bounds.StartColumn, bounds.StartRow, 0, 0),
                               "formula": written, "error": error, "text": corner.getString()})
        else:
            corner.setString(formula)

    # Dien ngang truoc (hang dau), roi dien doc tung cot: Microsoft Excel lam dung thu tu nay, va nho
    # vay moi cot lay hang dau da dien xong lam nguon.
    if cols > 1:
        target.getCellRangeByPosition(0, 0, cols - 1, 0).fillAuto(
            uno.Enum("com.sun.star.sheet.FillDirection", "TO_RIGHT"), 1)
    if rows > 1:
        direction = uno.Enum("com.sun.star.sheet.FillDirection", "TO_BOTTOM")
        for c in range(cols):
            target.getCellRangeByPosition(c, 0, c, rows - 1).fillAuto(direction, 1)

    result = {"filled": rows * cols, "range": address, "sheet": sheet.Name,
              "sample": [[target.getCellByPosition(c, r).getFormula()
                          for c in range(min(cols, 3))] for r in range(min(rows, 3))]}
    if errors:
        result["formulaErrors"] = errors
    return result


def format_range(env, params):
    address = values.string(params, "range")
    if not address:
        raise values.ParamError("'range' is required")
    doc = env.document
    target = _range(_sheet(doc, params), address)
    _apply_font(target, params)
    color = values.color(values.string(params, "fontColor"))
    if color is not None:
        target.CharColor = color
    fill = values.color(values.string(params, "fillColor"))
    if fill is not None:
        target.CellBackColor = fill
    if values.has(params, "numFmt"):
        target.NumberFormat = _number_format(doc, values.string(params, "numFmt", "General") or "General")
    if values.has(params, "horizontal"):
        name = (values.string(params, "horizontal", "left") or "left").lower()
        if name not in ALIGN:
            raise values.ParamError("'horizontal' must be left/center/right, got '%s'" % name)
        target.HoriJustify = uno.Enum("com.sun.star.table.CellHoriJustify", ALIGN[name])
    if values.has(params, "wrap"):
        target.IsTextWrapped = values.boolean(params, "wrap", False)
    return {"sheet": _active_sheet_name(env, params), "range": address}


def _shape_names(sheet) -> set:
    """Ten moi hinh tren trang ve cua sheet.

    Moi chart la mot hinh dat tren trang ve, nen ten chart phai tranh ca ten hinh - khong chi tranh ten
    trong tap `sheet.Charts`. Doc loi thi tra ve rong: day chi la buoc kiem tra them.
    """
    names = set()
    try:
        page = sheet.DrawPage
        for index in range(page.Count):
            try:
                names.add(page.getByIndex(index).Name)
            except Exception:  # noqa: BLE001
                pass
    except Exception:  # noqa: BLE001
        pass
    return names


def _taken_chart_names(doc) -> set:
    """Moi ten chart da dung trong CA TAI LIEU, khong chi sheet dang lam.

    LibreOffice dat ten chart duy nhat tren toan tai lieu: dat "Chart1" o sheet A roi dat "Chart1" o sheet
    B thi lan thu hai nem RuntimeException. Guard cu chi hoi `sheet.Charts.hasByName` - voi sheet chua co
    chart thi luon tra ve "chua co" - nen no cho qua roi `addNewByName` moi no, va cai no nem ra la mot loi
    pyuno khong doc duoc ("Couldn't convert <traceback object ...>").

    Do ngay 02/10/2026 tren file Test 4: Statistics giu Chart1..Chart5, nen moi lan them chart MAC DINH
    tren sheet khac deu hong (ten tu sinh luon bat dau lai tu Chart1) - 11 lan lien tiep trong mot luot
    chay, va agent khong co thong tin gi de sua.

    Gop ca ten hinh tren trang ve cua moi sheet: chart la mot hinh, hinh do co the con lai sau mot lan
    tao do dang bi bo quen.
    """
    taken = set()
    sheets = doc.getSheets()
    try:
        names = sheets.getElementNames()
    except Exception:  # noqa: BLE001
        return taken
    for sheet_name in names:
        try:
            other = sheets.getByName(sheet_name)
            taken |= set(other.Charts.getElementNames())
            taken |= _shape_names(other)
        except Exception:  # noqa: BLE001
            pass
    return taken


def _diagram(kind: str):
    """Kieu bieu do theo ten nguoi dung go -> service diagram cua UNO."""
    services = {
        "column": "com.sun.star.chart.BarDiagram",
        "bar": "com.sun.star.chart.BarDiagram",
        "line": "com.sun.star.chart.LineDiagram",
        "pie": "com.sun.star.chart.PieDiagram",
        "area": "com.sun.star.chart.AreaDiagram",
        "scatter": "com.sun.star.chart.XYDiagram",
    }
    if kind not in services:
        raise values.ParamError("'type' must be one of %s, got '%s'" % ("/".join(sorted(services)), kind))
    return services[kind], kind == "bar"


def add_chart(env, params):
    """Chen bieu do tu mot vung du lieu. Khong co lenh nay thi Dashboard chi la bang so."""
    address = values.string(params, "range")
    if not address:
        raise values.ParamError("'range' is required")
    kind = (values.string(params, "type", "column") or "column").lower()
    service, horizontal = _diagram(kind)
    doc = env.document
    sheet = _sheet(doc, params)
    source = _range(sheet, address)
    anchor = values.string(params, "anchor", "A1") or "A1"

    # Kich thuoc tinh bang cm (nguoi dung de hinh dung hon 1/100 mm cua UNO).
    width = int(round(values.number(params, "width", 12.0) * 1000))
    height = int(round(values.number(params, "height", 7.0) * 1000))
    cell = _range(sheet, anchor)
    position = cell.Position
    rect = uno.createUnoStruct("com.sun.star.awt.Rectangle")
    rect.X, rect.Y, rect.Width, rect.Height = position.X, position.Y, max(1000, width), max(1000, height)

    taken = _taken_chart_names(doc)
    name = values.string(params, "name")
    if name:
        if name in taken:
            raise values.ParamError("a chart named '%s' already exists in this workbook; chart names are "
                                    "unique per DOCUMENT, not per sheet - pick another name" % name)
    else:
        index = 1
        while ("Chart%d" % index) in taken:
            index += 1
        name = "Chart%d" % index
    # Tham so 3 nhan CellRangeAddress; 4/5 bao UNO lay ten cot/dong lam nhan chu giai.
    sheet.Charts.addNewByName(name, rect, (source.RangeAddress,), True, True)

    chart = sheet.Charts.getByName(name).getEmbeddedObject()
    type_applied = True
    try:
        chart.Diagram = chart.createInstance(service)
        if horizontal:
            chart.Diagram.Vertical = False        # BarDiagram nam ngang
    except Exception:  # noqa: BLE001 - giu kieu mac dinh con hon hong ca lenh
        type_applied = False

    title = values.string(params, "title")
    title_applied = False
    if title:
        try:
            # Thuoc tinh la HasMainTitle, KHONG phai HasTitle: ban LibreOffice nay khong co HasTitle
            # (do ngay 02/10/2026 bang probe UNO rieng).
            chart.HasMainTitle = True
            chart.Title.String = title
            title_applied = True
        except Exception:  # noqa: BLE001
            pass

    # Doc lai kieu THAT SU cua bieu do thay vi lap lai dieu minh vua xin: neu ban LibreOffice tu choi
    # doi kieu thi nguoi goi phai biet, khong duoc nghe loi hua.
    actual = _diagram_kind(chart)
    return {"chart": name, "sheet": sheet.Name, "type": kind, "diagram": actual,
            "typeApplied": type_applied and actual == kind, "titleApplied": title_applied,
            "range": address, "anchor": anchor}


def _diagram_kind(chart) -> str:
    """Kieu that cua bieu do, quy ve cung bo ten voi ban C# (column/bar/line/pie/area/scatter).

    LibreOffice gop cot va thanh lam mot BarDiagram, phan biet bang thuoc tinh Vertical - neu tra ve
    nguyen ten service thi hai ban se tra hai kieu gia tri khac nhau cho cung mot hop dong.
    """
    try:
        service = chart.Diagram.getDiagramType()
    except Exception:  # noqa: BLE001
        return ""
    names = {"BarDiagram": "bar", "LineDiagram": "line", "PieDiagram": "pie",
             "AreaDiagram": "area", "XYDiagram": "scatter"}
    kind = names.get(service.rsplit(".", 1)[-1])
    if kind == "bar":
        try:
            if chart.Diagram.Vertical:
                return "column"
        except Exception:  # noqa: BLE001
            pass
    return kind or service


def list_charts(env, params):
    """Ten + kieu that cua tung bieu do - de agent tu kiem chung Dashboard, khong phai tin loi hua."""
    sheet = _sheet(env.document, params)
    items = []
    for name in sheet.Charts.getElementNames():
        entry = {"name": name, "diagram": ""}
        try:
            entry["diagram"] = _diagram_kind(sheet.Charts.getByName(name).getEmbeddedObject())
        except Exception:  # noqa: BLE001
            pass
        items.append(entry)
    # shapes: moi hinh tren trang ve. Chart la mot hinh, nhung hinh co the con lai sau mot lan tao do
    # dang bi bo quen - va khi do ten no van chiem cho, khien et.addChart dung ten. Liet ra day de thay.
    return {"sheet": sheet.Name, "charts": items, "shapes": sorted(_shape_names(sheet))}


# --- Dinh dang dieu kien -------------------------------------------------------------------------
#
# Duong lam duoc la duong VONG, va hai lan do truoc do da ket luan sai la "khong co duong nao" vi chi
# thao tac tren VAT CHUA (sheet.ConditionalFormats) chu khong hoi doi tuong THEO VUNG:
#
#   1. tap vung  = doc.createInstance("com.sun.star.sheet.SheetCellRanges") + addRangeAddress
#      (ServiceManager tra None; sheet.getCellRangesByName khong co trong pyuno)
#   2. formats.createByRange(tap vung) -> tra ve SO ID, khong phai doi tuong
#   3. formats.getConditionalFormats() -> day doi tuong theo vung; moi doi tuong CO createEntry
#   4. createEntry(Type, Position) -> TAO that, nhung pyuno tra ve None: phai lay lai bang getByIndex
#
# Do bang probe that tren LibreOffice 26.8.0.3: tests/bench/probe_cf3.py va probe_cf5.py, ket qua o
# tests/bench/results/probe-conditional-format-3.txt.
CF_TYPE_CONDITION = 0      # com.sun.star.sheet.ConditionEntryType.CONDITION (COLORSCALE/DATABAR/... la kieu khac)

# Ten goi gon cho agent -> ten hang so trong com.sun.star.sheet.ConditionOperator.
CF_OPERATORS = {
    "equal": "EQUAL", "notequal": "NOT_EQUAL", "greater": "GREATER", "greaterequal": "GREATER_EQUAL",
    "less": "LESS", "lessequal": "LESS_EQUAL", "between": "BETWEEN", "notbetween": "NOT_BETWEEN",
    "formula": "FORMULA",
}

# So nguyen cua tung toan tu. Phai co bang so nay vi thuoc tinh Operator DOC RA la int, con hang so
# cua module la doi tuong uno.Enum - khong so sanh truc tiep duoc, va uno.Enum(...) tu tao thi bi tu
# choi khi gan (AttributeError: Operator). So do bang cach gan roi doc lai tren LibreOffice 26.8.0.3
# (tests/bench/probe_cf5.py); bai test song kiem vong ten -> ghi -> doc -> ten cho MOI toan tu nen
# neu LibreOffice doi so thi test do ngay, khong am tham sai.
CF_OPERATOR_VALUES = {
    "EQUAL": 1, "NOT_EQUAL": 2, "GREATER": 3, "GREATER_EQUAL": 4, "LESS": 5, "LESS_EQUAL": 6,
    "BETWEEN": 7, "NOT_BETWEEN": 8, "FORMULA": 9,
}
# Ten DOC RA phai la cung tu vung voi ten NHAN VAO (camelCase), khong phai ten hang so UNO tho
# (LESS_EQUAL): neu tra ve ten UNO thi agent phai biet hai cach viet cho cung mot thu. Bai live kiem
# vong ten -> ghi -> doc -> ten cho MOI toan tu, nen bang nay thieu ten nao la lo ra ngay ("unknown(n)").
CF_OPERATOR_LABELS = {
    "EQUAL": "equal", "NOT_EQUAL": "notEqual", "GREATER": "greater", "GREATER_EQUAL": "greaterEqual",
    "LESS": "less", "LESS_EQUAL": "lessEqual", "BETWEEN": "between", "NOT_BETWEEN": "notBetween",
    "FORMULA": "formula",
}
CF_OPERATOR_NAMES = {CF_OPERATOR_VALUES[uno]: label for uno, label in CF_OPERATOR_LABELS.items()}
CF_USES_FORMULA2_NAMES = ("between", "notBetween")
CF_STYLE_PREFIX = "Axiom CF "


def _cf_operator_module():
    """Module hang so ConditionOperator, xin SAN TAT CA ten se dung.

    pyuno sinh module kieu luoi: chi ten nao da duoc yeu cau qua fromlist moi co, getattr cho ten
    khac nem AttributeError (da mac dung loi nay: dir() chi tra ve dung mot ten).
    """
    return __import__("com.sun.star.sheet.ConditionOperator", fromlist=sorted(set(CF_OPERATORS.values())))


def _cf_style(doc, rule):
    """Ten cell style de gan cho mot rule: style co san, hoac style sinh ra tu mau/co chu cua rule.

    LibreOffice gan dinh dang dieu kien bang TEN STYLE chu khong bang mau truc tiep (khac Excel COM).
    Khong noi gi thi dung style "Good" co san cua Calc.
    """
    wanted = values.string(rule, "styleName")
    styles = doc.StyleFamilies.getByName("CellStyles")
    available = tuple(styles.getElementNames())
    if wanted:
        if wanted not in available:
            raise values.ParamError("no cell style named '%s' (co san: %s)" % (wanted, ", ".join(available)))
        return wanted

    spec = {}
    for key in ("bold", "italic"):
        if values.has(rule, key):
            spec[key] = values.boolean(rule, key, False)
    for key in ("fontColor", "fillColor", "numFmt"):
        text = values.string(rule, key)
        if text:
            spec[key] = text
    if not spec:
        return "Good"

    # Ten style sinh ra tu chinh noi dung dinh dang: cung mot kieu thi dung lai style cu, khong de lai
    # rac trong danh sach style moi lan goi.
    name = CF_STYLE_PREFIX + hashlib.md5(repr(sorted(spec.items())).encode("utf-8")).hexdigest()[:6]
    if name in available:
        return name
    style = doc.createInstance("com.sun.star.style.CellStyle")
    if "bold" in spec:
        style.CharWeight = BOLD if spec["bold"] else NORMAL
    if "italic" in spec:
        style.CharPosture = uno.Enum("com.sun.star.awt.FontSlant", "ITALIC" if spec["italic"] else "NONE")
    if "fontColor" in spec:
        color = values.color(spec["fontColor"])
        if color is not None:
            style.CharColor = color
    if "fillColor" in spec:
        color = values.color(spec["fillColor"])
        if color is not None:
            style.CellBackColor = color
    if "numFmt" in spec:
        style.NumberFormat = _number_format(doc, spec["numFmt"])
    styles.insertByName(name, style)
    return name


def _cf_plan(doc, rules):
    """Kiem HET rule truoc khi ghi mot cai nao: rule thu 3 sai thi khong duoc de 2 rule dau da vao file."""
    plan = []
    for index, rule in enumerate(rules):
        if not isinstance(rule, dict):
            raise values.ParamError("rules[%d] must be an object, got %s" % (index, values.describe(rule)))
        key = (values.string(rule, "operator", "less") or "less").lower().replace("_", "").replace(" ", "")
        if key not in CF_OPERATORS:
            raise values.ParamError("rules[%d].operator must be one of %s, got '%s'"
                                    % (index, "/".join(sorted(CF_OPERATORS)), key))
        operator = CF_OPERATORS[key]
        formula1 = values.string(rule, "formula1")
        if not formula1:
            raise values.ParamError("rules[%d] needs 'formula1' - the value to compare with, "
                                    "e.g. {\"operator\": \"less\", \"formula1\": \"50\"}" % index)
        formula2 = values.string(rule, "formula2")
        if operator in ("BETWEEN", "NOT_BETWEEN") and not formula2:
            raise values.ParamError("rules[%d]: operator '%s' needs 'formula2' as well" % (index, key))
        if operator == "FORMULA":
            formula1 = formula1.lstrip("=")      # Calc nhan cong thuc tran, khong co dau =
        plan.append({"operator": operator, "formula1": formula1, "formula2": formula2,
                     "styleName": _cf_style(doc, rule), "input": key})
    return plan


def set_conditional_format(env, params):
    """Dat dinh dang dieu kien cho mot vung. Goi lai tren cung vung thi THAY the rule cu cua vung do.

    Nhieu rule tren cung mot vung thi de chung trong MOT loi goi (`rules`), khong phai nhieu lan goi:
    moi lan goi la mot lan thay the, nen goi nam lan se chi con rule cuoi cung.
    """
    address = values.string(params, "range")
    if not address:
        raise values.ParamError("'range' is required: the area the rules apply to, e.g. 'B2:B100'")
    rules = params.get("rules")
    # Bridge co the boc mang mot lop ({"items": [...]}, {"value": [...]}); mot rule don le thi cung nhan.
    if isinstance(rules, dict):
        for key in ("items", "item", "value"):
            if key in rules:
                rules = rules[key]
                break
        else:
            rules = [rules]
    if not isinstance(rules, list) or not rules:
        raise values.ParamError("'rules' is required: a list of rules, e.g. "
                                "[{\"operator\": \"less\", \"formula1\": \"50\", \"styleName\": \"Bad\"}]")
    doc = env.document
    sheet = _sheet(doc, params)
    target = _range(sheet, address)
    plan = _cf_plan(doc, rules)

    formats = sheet.ConditionalFormats
    wanted = doc.createInstance("com.sun.star.sheet.SheetCellRanges")
    wanted.addRangeAddress(target.RangeAddress, False)

    # Vung da co dinh dang dieu kien thi go han di roi lam lai: giu lai se chong len nhau, va lan goi
    # thu hai cua agent (sau khi doc lai thay sai) se thanh hai bo rule cung song.
    for item in formats.getConditionalFormats():
        if item.Range.AbsoluteName == wanted.AbsoluteName:
            formats.removeByID(item.ID)
            break
    formats.createByRange(wanted)                       # -> ID, khong phai doi tuong
    holder = formats.getConditionalFormats()[-1]
    module = _cf_operator_module()

    written = []
    for rule in plan:
        holder.createEntry(CF_TYPE_CONDITION, holder.Count)     # (Type, Position)
        entry = holder.getByIndex(holder.Count - 1)             # createEntry tra ve None
        entry.Operator = getattr(module, rule["operator"])
        entry.Formula1 = rule["formula1"]
        if rule["formula2"]:
            entry.Formula2 = rule["formula2"]
        entry.StyleName = rule["styleName"]
        written.append({"operator": rule["input"], "formula1": rule["formula1"],
                        "formula2": rule["formula2"], "styleName": rule["styleName"]})

    return {"sheet": sheet.Name, "range": wanted.AbsoluteName, "rules": written}


def list_conditional_formats(env, params):
    """Doc lai dinh dang dieu kien dang co - de agent tu kiem, khong phai tin loi goi khong loi.

    `range` (khong bat buoc) de chi xem mot vung; bo trong thi liet ke ca sheet.
    """
    sheet = _sheet(env.document, params)
    address = values.string(params, "range")
    wanted = None
    if address:
        holder = env.document.createInstance("com.sun.star.sheet.SheetCellRanges")
        holder.addRangeAddress(_range(sheet, address).RangeAddress, False)
        wanted = holder.AbsoluteName
    items = []
    for item in sheet.ConditionalFormats.getConditionalFormats():
        if wanted is not None and item.Range.AbsoluteName != wanted:
            continue
        rules = []
        for index in range(item.Count):
            entry = item.getByIndex(index)
            operator = CF_OPERATOR_NAMES.get(entry.Operator, "unknown(%s)" % entry.Operator)
            # LibreOffice luon tra Formula2 = "0" cho cac toan tu khong dung no; tra nguyen "0" thi agent
            # tuong rule co hai nguong. Chi bao khi toan tu that su dung nguong thu hai.
            formula2 = entry.Formula2 if operator in CF_USES_FORMULA2_NAMES else None
            rules.append({"operator": operator, "formula1": entry.Formula1, "formula2": formula2,
                          "styleName": entry.StyleName, "type": int(entry.Type)})
        items.append({"range": item.Range.AbsoluteName, "rules": rules})
    return {"sheet": sheet.Name, "formats": items}


def undo(env, params):
    return documents.undo(env.document, values.integer(params, "count", 1))


def check_range(env, params):
    from . import checks

    return checks.range_report(env, params)


command("et.newWorkbook", "et", lambda env, p: documents.open_document(env.ctx, "et", None), "Tạo sổ tính mới")
command("et.open", "et", lambda env, p: documents.open_document(env.ctx, "et", values.string(p, "path")), "Mở .xlsx/.xls/.ods/.csv", req("path"))
command("et.listSheets", "et", list_sheets, "Danh sách sheet + sheet đang active", agent=True)
command("et.addSheet", "et", add_sheet,
        "Thêm sheet mới (bỏ trống `name` thì đặt tên Sheet1, Sheet2...)", opt("name"), opt("index", "0-based, default: append"),
        agent=True, undo=True)
command("et.renameSheet", "et", rename_sheet, "Đổi tên sheet", req("sheet"), req("name"), agent=True, undo=True)
command("et.activateSheet", "et", activate_sheet, "Chuyển sheet", req("sheet"), agent=True)
command("et.readRange", "et", read_range, "Đọc vùng, ví dụ `A1:C10`", req("range"), opt("sheet"), agent=True)
command("et.writeRange", "et", write_range,
        "Ghi vùng từ ô góc trên-trái; `values` là mảng 2 chiều. Công thức viết theo cú pháp en-US "
        "(dấu phẩy); nếu máy dùng dấu chấm phẩy thì tự đổi. Công thức còn lỗi trả về ở `formulaErrors`",
        req("range", "top-left cell e.g. 'A1'"), req("values", "2D array of rows e.g. [[\"Tên\",\"Điểm\"],[\"An\",9.5]]"), opt("sheet"),
        agent=True, undo=True)
command("et.writeRanges", "et", write_ranges,
        "Ghi NHIỀU vùng trong MỘT lời gọi: `writes` là mảng {range, values, sheet?}. Dùng khi cần viết "
        "nhiều khối trong cùng một phản hồi (bớt vòng qua bridge); công thức vẫn theo cú pháp en-US và "
        "lỗi công thức gom ở `formulaErrors`. Kiểm hết tham số trước khi ghi: một vùng sai thì không "
        "vùng nào được ghi",
        req("writes", "array of {range, values, sheet?}"), opt("sheet"), agent=True, undo=True)
command("et.fillRange", "et", fill_range,
        "Viết MỘT công thức vào ô góc trên-trái của `range` rồi điền ra cả vùng, tham chiếu tương đối tự "
        "dịch (dùng cho bảng nghìn dòng: đừng gửi từng ô)",
        req("range", "the whole area e.g. 'B2:H1000'"), opt("formula", "written to the top-left cell first"),
        opt("sheet"), agent=True, undo=True)
command("et.formatRange", "et", format_range, "Định dạng vùng", req("range"), opt("bold"), opt("italic"), opt("fontSize"),
        opt("fontColor"), opt("fillColor"), opt("numFmt"), opt("horizontal"), opt("wrap"), opt("sheet"), agent=True, undo=True)
command("et.setConditionalFormat", "et", set_conditional_format,
        "Đặt định dạng điều kiện cho vùng: `rules` là mảng rule, mỗi rule có operator "
        "(less/lessEqual/greater/greaterEqual/equal/notEqual/between/notBetween/formula), formula1, "
        "formula2 (chỉ between), và styleName có sẵn (Good/Bad/Neutral/Warning/Error/...) hoặc "
        "bold/italic/fontColor/fillColor/numFmt. Gọi lại trên cùng vùng thì THAY rule cũ của vùng đó, "
        "nên nhiều rule trên một vùng phải để trong MỘT lần gọi",
        req("range"), req("rules", "array of rule objects"), opt("sheet"), agent=True, undo=True)
command("et.listConditionalFormats", "et", list_conditional_formats,
        "Liệt kê định dạng điều kiện đang có (đọc lại để tự kiểm); bỏ trống `range` thì cả sheet",
        opt("range", "chỉ xem một vùng"), opt("sheet"), agent=True)
command("et.addChart", "et", add_chart,
        "Chèn biểu đồ từ vùng dữ liệu; `type` column/bar/line/pie/area/scatter, `width`/`height` tính bằng cm",
        req("range", "source data e.g. 'A1:B13'"), opt("type", "column (default)/bar/line/pie/area/scatter"),
        opt("title"), opt("name"), opt("anchor", "top-left cell, default 'A1'"),
        opt("width", "cm, default 12"), opt("height", "cm, default 7"), opt("sheet"), agent=True, undo=True)
command("et.listCharts", "et", list_charts, "Danh sách biểu đồ trên sheet", opt("sheet"), agent=True)
command("et.undo", "et", undo, "Hoàn tác", opt("count"), agent=True)
command("et.exportPdf", "et", lambda env, p: documents.export_pdf(env.document, "et", values.string(p, "path")),
        "Xuất PDF", req("path"), agent=True)
command("et.save", "et", lambda env, p: documents.save(env.document, "et", None), "Lưu", agent=True)
command("et.saveAs", "et", lambda env, p: documents.save(env.document, "et", values.string(p, "path")),
        "Lưu thành file mới", req("path"), agent=True)
command("et.closeAll", "et", lambda env, p: documents.close_all(env.ctx, "et"), "**Đóng mọi sổ, không lưu**")
command("et.checkRange", "et", check_range, "Soát vùng (chỉ đọc): lỗi công thức, ô trống, vùng ngoài bảng", opt("range", "default: the used range"), opt("sheet"), agent=True)
