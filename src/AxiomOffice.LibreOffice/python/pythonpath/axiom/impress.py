"""Lenh Impress (kind "wpp") - cung ten/ket qua voi CommandDispatcher.Presentation.cs (docs/architecture/libreoffice.md
muc 7.3). Kich thuoc trong UNO la 1/100 mm; tham so cua lenh tinh bang point (giong ban Windows)."""
from __future__ import annotations

import os

import uno

from . import documents, values
from .commands import command, opt, req

BOLD, NORMAL = 150.0, 100.0
ALIGN = {"left": "LEFT", "center": "CENTER", "centre": "CENTER", "right": "RIGHT", "justify": "BLOCK"}
# So layout kieu PowerPoint -> com.sun.star.presentation.DrawPageLayout (LibreOffice khong co kieu placeholder
# giong Office: 1 = chi tieu de, 2 = tieu de + noi dung, 11 = chi tieu de, 12 = trong).
LAYOUTS = {1: "AUTOLAYOUT_TITLE", 2: "AUTOLAYOUT_TITLE_CONTENT", 11: "AUTOLAYOUT_TITLE", 12: "AUTOLAYOUT_NONE"}
LAYOUT_VALUES = {"AUTOLAYOUT_NONE": 0, "AUTOLAYOUT_TITLE": 1, "AUTOLAYOUT_TITLE_CONTENT": 2}


def _layout_value(name: str) -> int:
    """Ten hang so tu API dang chay; thieu (ban LibreOffice cu) thi dung bang gia tri da biet."""
    try:
        return uno.getConstantByName("com.sun.star.presentation.DrawPageLayout." + name)
    except Exception:  # noqa: BLE001
        return LAYOUT_VALUES[name]


def _pages(doc):
    return doc.getDrawPages()


def _page(doc, params, required: bool = True):
    pages = _pages(doc)
    count = pages.getCount()
    index = values.integer(params, "slide", 0)
    if count == 0:
        if required:
            raise values.ParamError("presentation has no slides; call wpp.addSlide first")
        return None, 0
    if index < 1 or index > count:
        index = count
    return pages.getByIndex(index - 1), index


def _points(value: float) -> int:
    return int(round(value * values.POINT_TO_HMM))


def _size(params, name: str, default_pt: float) -> int:
    return _points(values.number(params, name, default_pt))


def _set_geometry(shape, params, defaults):
    left, top, width, height = defaults
    box = uno.createUnoStruct("com.sun.star.awt.Point")
    box.X, box.Y = _size(params, "left", left), _size(params, "top", top)
    shape.Position = box
    size = uno.createUnoStruct("com.sun.star.awt.Size")
    size.Width, size.Height = _size(params, "width", width), _size(params, "height", height)
    shape.Size = size


def _apply_text(shape, params, prefix=""):
    text = shape.getText()
    if values.has(params, "fontSize"):
        text.CharHeight = float(values.number(params, "fontSize", 18))
    if values.has(params, "bold"):
        text.CharWeight = BOLD if values.boolean(params, "bold", False) else NORMAL
    color = values.color(values.string(params, "color"))
    if color is not None:
        text.CharColor = color
    if values.has(params, "align"):
        name = (values.string(params, "align", "left") or "left").lower()
        if name not in ALIGN:
            raise values.ParamError("'align' must be left/center/right, got '%s'" % name)
        text.ParaAdjust = uno.Enum("com.sun.star.style.ParagraphAdjust", ALIGN[name])


def _shape_text(shape):
    try:
        if shape.supportsService("com.sun.star.drawing.Text"):
            return shape.getText().getString()
    except Exception:  # noqa: BLE001
        pass
    return ""


def list_slides(env, params):
    doc = env.document
    pages = _pages(doc)
    count = pages.getCount()
    slides = []
    for index in range(count):
        page = pages.getByIndex(index)
        texts = []
        for s in range(page.getCount()):
            text = _shape_text(page.getByIndex(s))
            if text:
                texts.append(text)
        try:
            layout = page.Layout
        except Exception:  # noqa: BLE001
            layout = None
        entry = {"index": index + 1, "layout": layout, "shapeTexts": texts}
        if values.boolean(params, "detail", False):
            try:
                entry["master"] = str(page.getMasterPage().getName() if hasattr(page, "getMasterPage") else "")
            except Exception:
                pass
            try:
                notes = page.getNotesPage()
                lines_ = []
                for s in range(notes.getCount()):
                    t = _shape_text(notes.getByIndex(s))
                    if t:
                        lines_.append(t)
                if lines_:
                    entry["notes"] = lines_
            except Exception:
                pass
        slides.append(entry)
    return {"presentation": doc.getTitle(), "slideCount": count, "slides": slides}


def add_slide(env, params):
    # Kiem tra layout TRUOC khi chen: lenh sai khong duoc de lai slide rac trong bai (va undo context
    # cua lenh do bi bo khi loi).
    layout = values.integer(params, "layout", 12)
    if layout not in LAYOUTS:
        raise values.ParamError("'layout' must be one of %s, got %d" % (sorted(LAYOUTS), layout))
    doc = env.document
    pages = _pages(doc)
    index = pages.getCount() + 1
    page = pages.insertNewByIndex(index - 1)
    try:
        page.Layout = _layout_value(LAYOUTS[layout])
    except Exception as exc:  # noqa: BLE001
        try:
            pages.remove(page)
        except Exception:  # noqa: BLE001
            pass
        raise values.ParamError("cannot apply layout %d: %s" % (layout, exc))
    return {"slide": index, "layout": layout}


def add_text(env, params):
    text = values.string(params, "text", "")
    doc = env.document
    page, index = _page(doc, params)
    shape = doc.createInstance("com.sun.star.drawing.TextShape")
    shape.Name = "AxiomText"
    page.add(shape)
    _set_geometry(shape, params, (60, 60, 540, 120))
    shape.TextAutoGrowHeight = False
    shape.getText().setString(text)
    _apply_text(shape, params)
    return {"slide": index, "shape": shape.Name}


def add_image(env, params):
    path = values.string(params, "path")
    if not path:
        raise values.ParamError("'path' is required")
    if not os.path.isfile(path):
        raise values.ParamError("image file not found: '%s'" % path)
    doc = env.document
    page, index = _page(doc, params)
    provider = env.ctx.ServiceManager.createInstanceWithContext("com.sun.star.graphic.GraphicProvider", env.ctx)
    graphic = provider.queryGraphic((documents.prop("URL", documents.to_url(path)),))
    shape = doc.createInstance("com.sun.star.drawing.GraphicObjectShape")
    shape.Name = "AxiomImage"
    shape.Graphic = graphic
    page.add(shape)
    left = _size(params, "left", 60)
    top = _size(params, "top", 60)
    point = uno.createUnoStruct("com.sun.star.awt.Point")
    point.X, point.Y = left, top
    shape.Position = point
    size = uno.createUnoStruct("com.sun.star.awt.Size")
    if values.has(params, "width") or values.has(params, "height"):
        source = graphic.Size100thMM
        width, height = source.Width, source.Height
        if not width or not height:
            pixels = graphic.SizePixel
            width, height = pixels.Width * 2540 // 96, pixels.Height * 2540 // 96
        if values.has(params, "width") and values.has(params, "height"):
            width, height = _size(params, "width", 0), _size(params, "height", 0)
        elif values.has(params, "width"):
            width = _size(params, "width", 0)
            height = int(height * width / source.Width) if source.Width else height
        else:
            height = _size(params, "height", 0)
            width = int(source.Width * height / source.Height) if source.Height else width
    else:
        width, height = graphic.Size100thMM.Width, graphic.Size100thMM.Height
        if not width or not height:
            pixels = graphic.SizePixel
            width, height = pixels.Width * 2540 // 96, pixels.Height * 2540 // 96
    size.Width, size.Height = width, height
    shape.Size = size
    return {"slide": index, "shape": shape.Name}


def _table_from_shape(shape):
    for name in ("Model", "Table"):
        try:
            return shape.getPropertyValue(name)
        except Exception:  # noqa: BLE001
            continue
    return None


def add_table(env, params):
    rows_values = values.matrix(params, "values", False)
    rows = values.integer(params, "rows", 0)
    cols = values.integer(params, "cols", 0)
    if rows_values:
        rows = max(rows, len(rows_values))
        cols = max(cols, max(len(r) for r in rows_values))
    if rows <= 0 or cols <= 0:
        raise values.ParamError("'rows' and 'cols' are required (or pass 'values' as a 2D array to size the table)")
    doc = env.document
    page, index = _page(doc, params)
    shape = doc.createInstance("com.sun.star.drawing.TableShape")
    shape.Name = "AxiomTable"
    page.add(shape)
    _set_geometry(shape, params, (60, 120, 600, 200))
    model = _table_from_shape(shape)
    if model is None:
        raise RuntimeError("the LibreOffice build has no table model behind TableShape")
    for collection, wanted in ((model.getRows(), rows), (model.getColumns(), cols)):
        while collection.getCount() < wanted:
            collection.insertByIndex(collection.getCount(), 1)
        while collection.getCount() > wanted:
            collection.removeByIndex(collection.getCount() - 1, 1)
    filled = 0
    for r, row in enumerate(rows_values or []):
        for c, cell in enumerate(row[:cols]):
            model.getCellByPosition(c, r).setString("" if cell is None else _cell_text(cell))
            filled += 1
    return {"slide": index, "rows": rows, "cols": cols, "filled": filled}


def _cell_text(value) -> str:
    if isinstance(value, bool):
        return "True" if value else "False"
    if isinstance(value, float) and value.is_integer():
        return str(int(value))
    return str(value)


def _notes_shape(page):
    notes = page.getNotesPage()
    fallback = None
    for index in range(notes.getCount()):
        shape = notes.getByIndex(index)
        try:
            if shape.supportsService("com.sun.star.presentation.NotesShape"):
                return shape
            if fallback is None and shape.supportsService("com.sun.star.drawing.Text"):
                fallback = shape
        except Exception:  # noqa: BLE001
            continue
    return fallback


def set_notes(env, params):
    text = values.string(params, "text", "") or ""
    doc = env.document
    page, index = _page(doc, params)
    shape = _notes_shape(page)
    if shape is None:
        shape = doc.createInstance("com.sun.star.drawing.TextShape")
        shape.Name = "AxiomNotes"
        page.getNotesPage().add(shape)
        size = uno.createUnoStruct("com.sun.star.awt.Size")
        size.Width, size.Height = page.Width, int(page.Height * 0.4)
        shape.Size = size
        point = uno.createUnoStruct("com.sun.star.awt.Point")
        point.X, point.Y = 0, int(page.Height * 0.55)
        shape.Position = point
    shape.getText().setString(text)
    return {"slide": index, "notes": len(text)}


def delete_slide(env, params):
    doc = env.document
    page, index = _page(doc, params)
    _pages(doc).remove(page)
    return {"deleted": index, "slideCount": _pages(doc).getCount()}


command("wpp.newPresentation", "wpp", lambda env, p: documents.open_document(env.ctx, "wpp", None), "Tạo bài trình bày mới")
command("wpp.open", "wpp", lambda env, p: documents.open_document(env.ctx, "wpp", values.string(p, "path")), "Mở .pptx/.ppt/.odp", req("path"))
command("wpp.listSlides", "wpp", list_slides, "Số slide + text từng slide", opt("presentation", "full path of the presentation to work on; default: the active one"), opt("detail", "true = kem master/notes"), agent=True)
command("wpp.addSlide", "wpp", add_slide, "Thêm slide cuối; `layout` mặc định 12 = trống (1 = tiêu đề, 2 = tiêu đề + nội dung, 11 = chỉ tiêu đề)",
        opt("layout", "number: 1 title, 2 title+content, 11 title only, 12 blank"), agent=True, undo=True)
command("wpp.addText", "wpp", add_text, "Textbox có định dạng (`color` dạng `#RRGGBB`, `align` left/center/right)",
        req("text"), opt("slide"), opt("left"), opt("top"), opt("width"), opt("height"), opt("fontSize"), opt("bold"), opt("color"), opt("align"),
        opt("presentation", "full path of the presentation to work on; default: the active one"), agent=True, undo=True)
command("wpp.addTextBox", "wpp", add_text, "Textbox", req("text"), opt("slide"), opt("left"), opt("top"), opt("width"), opt("height"), undo=True)
command("wpp.addImage", "wpp", add_image, "Chèn ảnh (kích thước gốc nếu bỏ trống `width`/`height`)",
        req("path"), opt("slide"), opt("left"), opt("top"), opt("width"), opt("height"), agent=True, undo=True)
command("wpp.addTable", "wpp", add_table, "Bảng; `rows`/`cols` tự suy ra/nới theo `values`",
        opt("rows"), opt("cols"), opt("values", "2D array of rows"), opt("slide"), opt("left"), opt("top"), opt("width"), opt("height"),
        agent=True, undo=True)
command("wpp.setNotes", "wpp", set_notes, "Ghi chú thuyết trình", req("text"), opt("slide"), opt("presentation", "full path of the presentation to work on; default: the active one"), agent=True, undo=True)
command("wpp.deleteSlide", "wpp", delete_slide, "Xoá slide (mặc định slide cuối)", opt("slide"), agent=True, undo=True)
command("wpp.exportPdf", "wpp", lambda env, p: documents.export_pdf(env.document, "wpp", values.string(p, "path")),
        "Xuất PDF", req("path"), agent=True)
command("wpp.save", "wpp", lambda env, p: documents.save(env.document, "wpp", None), "Lưu", opt("presentation", "full path of the presentation to work on; default: the active one"), agent=True)
command("wpp.saveAs", "wpp", lambda env, p: documents.save(env.document, "wpp", values.string(p, "path")),
        "Lưu thành file mới", req("path"), agent=True)

def _template_unsupported(env, params):
    # Lam bai theo mau (docs/architecture/template-design.md muc 6) moi co tren PowerPoint (CommandDispatcher.Template.cs):
    # can InsertFromFile + Shape.Export cua COM. Ban LibreOffice lam o phien Linux.
    raise values.ParamError("not available on LibreOffice yet: build the slides with wpp.addSlide/wpp.addText "
                            "and wpp.addImage, using the template's colors from ppt_profile")


command("wpp.newFromTemplate", "wpp", _template_unsupported,
        "Tạo bài MỚI từ tệp mẫu (.pptx/.potx): giữ khổ + master, bỏ hết slide mẫu, lưu ở `path` và mở ra",
        req("template", "path of the template file"), req("path", "new .pptx to create; must not exist"), agent=True)
command("wpp.addSlideFromTemplate", "wpp", _template_unsupported,
        "Chèn bản sao slide số `kind` của tệp mẫu (giữ ảnh nền, khung chữ, định dạng); trả tên các khung chữ",
        req("template"), req("kind", "slide number in the template (kinds of ppt_profile)"),
        opt("slide", "insert after this slide; default: at the end"),
        opt("tag", "source slide number; a slide already tagged so is replaced in place"),
        opt("expect", "shape name the inserted slide must have (checks it is the template slide)"), opt("presentation", "full path of the presentation to work on; default: the active one"), agent=True, undo=True)
command("wpp.sourceManifest", "wpp", _template_unsupported,
        "Đọc bài nguồn (vd bài cũ .ppt) theo slide: tiêu đề, chữ, danh sách ảnh - để dựng lại theo mẫu",
        req("from", "path of the source presentation"), opt("first"), opt("last"), agent=True)
command("wpp.setShapeText", "wpp", _template_unsupported,
        "Thay chữ của một khung theo TÊN shape, giữ định dạng của khung (font, cỡ, màu)",
        req("shape", "shape name, e.g. 'TextBox 2'"), req("text", "\\n = new paragraph"), opt("slide"), opt("presentation", "full path of the presentation to work on; default: the active one"),
        agent=True, undo=True)
command("wpp.deleteShapes", "wpp", _template_unsupported,
        "Xoá các shape theo tên (mục lặp thừa của slide mẫu); không xoá ảnh nền",
        opt("shapes", "array of shape names"), opt("slide"), opt("emptyPlaceholders", "true: also delete every placeholder without text (the 'Click to add text' boxes)"), opt("presentation", "full path of the presentation to work on; default: the active one"), agent=True, undo=True)
command("wpp.copyPicture", "wpp", _template_unsupported,
        "Chép ảnh từ slide của bài khác (vd bài cũ) vào slide của bài đang làm, co vừa khung, giữ tỉ lệ",
        req("from", "path of the source presentation"), req("fromSlide"), opt("shape", "picture name; default: largest picture"),
        opt("slide"), opt("left"), opt("top"), opt("width"), opt("height"),
        opt("overlays", "true: also copy the labels/arrows drawn over the picture"),
        opt("format", "png (default) or jpg (much smaller for photos and scans)"),
        opt("region", "[left, top, width, height] of the source slide to copy as one picture (a diagram or table drawn with shapes); replaces shape"),
        opt("presentation", "full path of the presentation to work on; default: the active one"), agent=True, undo=True)
command("wpp.addParagraphs", "wpp", _template_unsupported,
        "Khung chữ nhiều đoạn giữ phân cấp (cấp thụt lề, đậm, nghiêng, số thứ tự, gạch đầu dòng); tự chọn cỡ chữ lớn nhất vừa khung",
        req("paragraphs", "array of {t: text, l: level 1-5, b: bold, i: italic, n: number prefix like '1.', u: bullet}"),
        opt("slide"), opt("left"), opt("top"), opt("width"), opt("height"),
        opt("fontSize", "largest size to try (default 24)"), opt("minFontSize", "smallest size (default 16)"),
        opt("color", "#RRGGBB body text"), opt("accent", "#RRGGBB bold (heading) paragraphs"), opt("font", "font name"),
        opt("anchor", "top (default) or middle"),
        opt("box", "rounded or rect: draw a filled box (card, callout) instead of a plain text box"),
        opt("fill", "#RRGGBB box fill"), opt("line", "#RRGGBB box border (none when omitted)"),
        opt("highlight", "#RRGGBB colour of **emphasised** words (default: accent)"),
        opt("presentation", "full path of the presentation to work on; default: the active one"), undo=True)
command("wpp.addGrid", "wpp", _template_unsupported,
        "Bảng có định dạng: dòng đầu là tiêu đề (nền màu, chữ trắng đậm), dòng xen kẽ nền nhạt, cột rộng theo nội dung, tự chọn cỡ chữ vừa khung",
        req("values", "2D array; first row = header"), opt("slide"), opt("left"), opt("top"), opt("width"), opt("height"),
        opt("fontSize", "largest size to try (default 18)"), opt("minFontSize", "smallest size (default 11)"),
        opt("header", "#RRGGBB header fill"), opt("band", "#RRGGBB fill of every second row"),
        opt("color", "#RRGGBB text"), opt("font", "font name"),
        opt("presentation", "full path of the presentation to work on; default: the active one"), undo=True)
command("wpp.deleteTagged", "wpp", _template_unsupported,
        "Xoá các slide do công cụ dựng (có tag nguồn) mà tag bắt đầu trong [first, last] - dựng lại một lô theo bố cục mới",
        req("first"), req("last"),
        opt("presentation", "full path of the presentation to work on; default: the active one"), undo=True)

command("wpp.closeAll", "wpp", lambda env, p: documents.close_all(env.ctx, "wpp"), "**Đóng mọi bài trình bày, không lưu**")
