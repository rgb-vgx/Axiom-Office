/*
 * Axiom Office - WPS JS add-in command registry + handlers.
 *
 * Global AxiomCommands { catalog(), catalogFor(kind), appKind(app), hasHandler(action),
 * execute(app, action, params) }.
 * CATALOG la ban SINH TU DONG tu registry Python (axiom.commands) bang
 * scripts/generate_mcp_commands.py - KHONG sua tay (vung giua hai moc BEGIN/END SINH TU DONG).
 * HANDLERS viet tay; MUTATING suy ra tu co `undo` cua CATALOG.
 * Port of src/AxiomOffice/Bridge/CommandDispatcher.Writer.cs (+ app.info) with the
 * same parameter names and result shapes as the LibreOffice handlers in
 * src/AxiomOffice.LibreOffice/python/pythonpath/axiom/writer.py.
 *
 * ES5 only (CEF Chrome 87): no let/const/arrow/template strings/optional chaining.
 * execute() never throws: errors become {ok:false, error}.
 */
(function (root) {
  "use strict";

  // Lenh Writer mo UndoRecord = lenh co `undo` trong registry (CATALOG ben duoi): moi lenh = dung mot buoc
  // Undo, khop voi so pane dem (pane-core.js EDIT_ACTIONS). Dien sau khi CATALOG co.
  var MUTATING = {};

  // --- BEGIN SINH TU DONG (scripts/generate_mcp_commands.py) - KHONG SUA TAY ---
  var CATALOG = [
    { name: "app.info", kind: null, agent: false, undo: false,
      summary: "Tên/version app, tài liệu đang mở, `state` (tài liệu, cửa sổ, visible)",
      params: [] },
    { name: "ai.ask", kind: null, agent: false, undo: false,
      summary: "Chạy AI agent trên tài liệu đang mở; trả `reply`, `transcript`, `seconds`, `rounds`",
      params: [{ name: "prompt", required: true, hint: null }] },
    { name: "ui.askpane", kind: null, agent: false, undo: false,
      summary: "Mở panel Ask AI trong sidebar",
      params: [] },
    { name: "ui.setup", kind: null, agent: false, undo: false,
      summary: "Mở wizard thiết lập (nhập máy chủ AI, model, kiểm tra kết nối)",
      params: [] },
    { name: "app.screenshot", kind: null, agent: false, undo: false,
      summary: "Ảnh chụp trang/slide hiện tại (PNG base64, thu nhỏ theo `maxWidth`, mặc định 1280)",
      params: [{ name: "maxWidth", required: false, hint: null }] },
    { name: "writer.newDocument", kind: "wps", agent: false, undo: false,
      summary: "Tạo tài liệu mới",
      params: [] },
    { name: "writer.open", kind: "wps", agent: false, undo: false,
      summary: "Mở .docx/.doc/.odt",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "writer.getText", kind: "wps", agent: true, undo: false,
      summary: "Đọc toàn bộ text",
      params: [
        { name: "maxChars", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.selection", kind: "wps", agent: true, undo: false,
      summary: "Text + vị trí đang chọn",
      params: [{ name: "document", required: false, hint: "full path of the document to work on; default: the active one" }] },
    { name: "writer.typeText", kind: "wps", agent: true, undo: true,
      summary: "Gõ tại con trỏ",
      params: [
        { name: "text", required: true, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.appendText", kind: "wps", agent: true, undo: true,
      summary: "Nối một KHỐI đoạn văn vào cuối tài liệu; `\\n` tách đoạn, tự đặt lại định dạng (không kế thừa), `alignment` mặc định left",
      params: [
        { name: "text", required: true, hint: null },
        { name: "bold", required: false, hint: null },
        { name: "italic", required: false, hint: null },
        { name: "underline", required: false, hint: null },
        { name: "size", required: false, hint: null },
        { name: "color", required: false, hint: null },
        { name: "font", required: false, hint: null },
        { name: "alignment", required: false, hint: "left/center/right/justify; mặc định left" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertStyledText", kind: "wps", agent: true, undo: true,
      summary: "Chèn text có định dạng tại con trỏ; `\\n` tách đoạn, `alignment` áp cho từng đoạn (`color` dạng `#RRGGBB`)",
      params: [
        { name: "text", required: true, hint: null },
        { name: "bold", required: false, hint: null },
        { name: "italic", required: false, hint: null },
        { name: "underline", required: false, hint: null },
        { name: "size", required: false, hint: null },
        { name: "color", required: false, hint: null },
        { name: "font", required: false, hint: null },
        { name: "alignment", required: false, hint: "left/center/right/justify; áp cho từng đoạn vừa chèn" },
        { name: "break", required: false, hint: "ngắt đoạn sau khối; mặc định true khi có `alignment`" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.heading", kind: "wps", agent: true, undo: true,
      summary: "Heading 1-9 (`level`, mặc định 1) + tự xuống dòng (`break`, mặc định true)",
      params: [
        { name: "text", required: false, hint: null },
        { name: "level", required: false, hint: null },
        { name: "break", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.formatSelection", kind: "wps", agent: true, undo: true,
      summary: "Định dạng vùng chọn",
      params: [
        { name: "bold", required: false, hint: null },
        { name: "italic", required: false, hint: null },
        { name: "underline", required: false, hint: null },
        { name: "size", required: false, hint: null },
        { name: "color", required: false, hint: null },
        { name: "font", required: false, hint: null },
        { name: "alignment", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.setParagraphAlignment", kind: "wps", agent: true, undo: true,
      summary: "Căn đoạn: left/center/right/justify",
      params: [
        { name: "alignment", required: true, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertTable", kind: "wps", agent: true, undo: true,
      summary: "Chèn bảng; `rows`/`cols` tự suy ra/nới theo `values`",
      params: [
        { name: "rows", required: false, hint: null },
        { name: "cols", required: false, hint: null },
        { name: "values", required: false, hint: "2D array of rows" },
        { name: "style", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.formatTable", kind: "wps", agent: true, undo: true,
      summary: "Định dạng bảng CÓ SẴN (không tạo lại): kiểu, font, màu hàng tiêu đề, màu sọc, viền, căn lề, co giãn",
      params: [
        { name: "table", required: false, hint: "1-based index; default: table at cursor, else last table" },
        { name: "style", required: false, hint: "e.g. 'Grid Table 4 - Accent 1'" },
        { name: "font", required: false, hint: null },
        { name: "size", required: false, hint: null },
        { name: "color", required: false, hint: "#RRGGBB text color" },
        { name: "headerFill", required: false, hint: "#RRGGBB" },
        { name: "headerColor", required: false, hint: "#RRGGBB header text" },
        { name: "headerBold", required: false, hint: null },
        { name: "bandFill", required: false, hint: "#RRGGBB every other data row" },
        { name: "borderColor", required: false, hint: "#RRGGBB" },
        { name: "borders", required: false, hint: "false = no borders (layout tables)" },
        { name: "alignment", required: false, hint: "left/center/right" },
        { name: "autoFit", required: false, hint: "content/window" },
        { name: "keepTogether", required: false, hint: "true = keep the whole table on one page" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertPageBreak", kind: "wps", agent: true, undo: true,
      summary: "Ngắt trang tại con trỏ, hoặc ngay trước đoạn `before`",
      params: [
        { name: "before", required: false, hint: "paragraph number from writer.outline / writer.checkLayout; default: at the cursor" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertImage", kind: "wps", agent: true, undo: true,
      summary: "Chèn ảnh tại con trỏ (kích thước theo point)",
      params: [
        { name: "path", required: true, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertHyperlink", kind: "wps", agent: true, undo: true,
      summary: "Chèn liên kết",
      params: [
        { name: "url", required: true, hint: null },
        { name: "text", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.replaceAll", kind: "wps", agent: true, undo: true,
      summary: "Tìm và thay toàn bộ",
      params: [
        { name: "find", required: true, hint: null },
        { name: "replace", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.replaceMany", kind: "wps", agent: true, undo: true,
      summary: "Thay NHIỀU chỗ trong một lần: `edits` = [{find, replace}] theo thứ tự (mỗi cặp thay mọi chỗ khớp) - dùng thay cho nhiều lần writer.replaceAll; trả số chỗ đã thay của từng cặp",
      params: [
        { name: "edits", required: true, hint: "array of {find, replace}; up to 200" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.lockSelection", kind: "wps", agent: false, undo: false,
      summary: "Khoá vùng đang bôi đen: tới khi mở khoá, lệnh writer.* chỉ được đổi chữ trong vùng đó (chars = 0: không bôi đen, không khoá)",
      params: [{ name: "document", required: false, hint: "full path of the document to work on; default: the active one" }] },
    { name: "writer.unlockSelection", kind: "wps", agent: false, undo: false,
      summary: "Mở khoá vùng bôi đen",
      params: [{ name: "document", required: false, hint: "full path of the document to work on; default: the active one" }] },
    { name: "writer.outline", kind: "wps", agent: true, undo: false,
      summary: "Dàn bài tài liệu: các đoạn (số thứ tự toàn tài liệu, chữ rút gọn, số tự động) và bảng (số hàng, cột, hàng tiêu đề)",
      params: [
        { name: "maxChars", required: false, hint: "characters per paragraph (default 160)" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.restyle", kind: "wps", agent: false, undo: true,
      summary: "Trình bày lại nền chung KHÔNG đổi chữ: tiêu đề mục (I., A., *), tiêu đề con (1.), nhãn (a. ...:), gạch đầu dòng hai cấp, bỏ dòng trống thừa, bảng (viền mảnh, hàng tiêu đề nhạt lặp lại), lề trang, số trang",
      params: [
        { name: "accent", required: false, hint: "#RRGGBB headings (default #2E5C8A)" },
        { name: "font", required: false, hint: "default Times New Roman" },
        { name: "size", required: false, hint: "body size (default 13)" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertBlocks", kind: "wps", agent: false, undo: true,
      summary: "Chèn khối bo tròn nền nhạt (ô ghi chú, cảnh báo, thẻ cạnh nhau) - thay các đoạn [replaceFrom, replaceTo] hoặc chèn trước đoạn `before`",
      params: [
        { name: "blocks", required: true, hint: "array of {before | replaceFrom, replaceTo, kind: box|row, gap, after, cells: [{tone, lines: [{k: title|bullet|text|small|center, t, size, bold}]}]}" },
        { name: "accent", required: false, hint: null },
        { name: "font", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.setStyle", kind: "wps", agent: true, undo: true,
      summary: "Gán style cho đoạn CÓ SẴN (không đổi chữ): `paragraphs` = số đoạn theo writer.outline; bỏ trống = các đoạn đang chọn. Style: Heading 1-9, Normal, Title, Subtitle, List Bullet, List Number, Caption, Quote",
      params: [
        { name: "style", required: true, hint: "e.g. 'Heading 2', 'Normal', 'List Bullet'" },
        { name: "paragraphs", required: false, hint: "array of paragraph numbers from writer.outline (n); default: the selected paragraphs" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertToc", kind: "wps", agent: true, undo: true,
      summary: "Mục lục tự động từ các Heading: chưa có thì chèn tại con trỏ (hoặc trước đoạn `before`), đã có thì cập nhật số trang - gọi lại sau khi viết xong",
      params: [
        { name: "levels", required: false, hint: "heading levels 1-9, default 3" },
        { name: "before", required: false, hint: "paragraph number from writer.outline to insert before; default: at the cursor" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.setHeaderFooter", kind: "wps", agent: true, undo: true,
      summary: "Đầu trang / chân trang cho mọi section (thay nội dung cũ) và số trang tự động",
      params: [
        { name: "header", required: false, hint: "header text; \"\" clears it" },
        { name: "footer", required: false, hint: "footer text; \"\" clears it" },
        { name: "pageNumber", required: false, hint: "true or left/center/right: page number field in the footer (after footer text)" },
        { name: "align", required: false, hint: "left/center/right for header and footer text, default center" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertCaption", kind: "wps", agent: true, undo: true,
      summary: "Chú thích có đánh số tự động (style Caption + trường SEQ) \"Bảng 1. ...\": mặc định ở CUỐI bài - gọi ngay trước writer.insertTable; `before` = số đoạn/bảng theo writer.outline để chú thích bảng có sẵn",
      params: [
        { name: "text", required: true, hint: "caption text without the number, e.g. 'Kết quả tuyển sinh 2024-2026'" },
        { name: "label", required: false, hint: "Bảng (default) / Hình / Biểu đồ..." },
        { name: "before", required: false, hint: "paragraph/table number from writer.outline to caption; default: end of the document" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.writeContent", kind: "wps", agent: true, undo: true,
      summary: "Viết NHIỀU khối nối tiếp vào CUỐI bài trong MỘT lệnh (một bước hoàn tác) - dùng thay cho mỗi đoạn một lệnh: tiêu đề, đoạn văn, gạch đầu dòng, danh sách số, bảng kèm chú thích, ngắt trang",
      params: [
        { name: "blocks", required: true, hint: "array, in order, of {heading, level} | {text} ('\\n' = new paragraph; optional alignment, bold, italic, size) | {bullets: [..]} | {numbers: [..]} | {table: 2D array, caption, style} | {pageBreak: true}" },
        { name: "alignment", required: false, hint: "alignment of {text} blocks: justify (default) / left / center / right" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.addTableRows", kind: "wps", agent: true, undo: true,
      summary: "Thêm hàng vào bảng CÓ SẴN (giữ định dạng của bảng, không dựng lại bảng): mặc định nối cuối bảng, `before` = số hàng để chèn vào giữa (1 = hàng tiêu đề)",
      params: [
        { name: "rows", required: true, hint: "2D array, one inner array of cell texts per new row" },
        { name: "table", required: false, hint: "table number (1 = first, as in writer.checkTables); default: table at cursor, else the last table" },
        { name: "before", required: false, hint: "row number to insert before; default: after the last row" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.deleteParagraphs", kind: "wps", agent: true, undo: true,
      summary: "Xoá các đoạn theo số của writer.outline / writer.checkLayout (đoạn trống thừa, đoạn chỉ có dấu ngắt trang, đoạn viết thừa); không xoá được đoạn trong bảng",
      params: [
        { name: "paragraphs", required: true, hint: "array of paragraph numbers" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.insertLayoutTable", kind: "wps", agent: true, undo: true,
      summary: "Bảng dàn trang KHÔNG viền ở CUỐI bài, định dạng từng dòng trong ô - phần đầu văn bản hành chính (cơ quan | quốc hiệu, tiêu ngữ, ngày tháng) và phần ký (nơi nhận | chức vụ, họ tên)",
      params: [
        { name: "rows", required: true, hint: "2D array of cells; a cell is a string ('\\n' = new line) or {lines: [string | {t, bold, italic, underline, size, upper}], align: center|left|right}" },
        { name: "widths", required: false, hint: "column widths in percent, e.g. [40, 60]; default equal" },
        { name: "font", required: false, hint: "default Times New Roman" },
        { name: "size", required: false, hint: "default 13" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.undo", kind: "wps", agent: true, undo: false,
      summary: "Hoàn tác (mỗi thao tác AI = 1 bước)",
      params: [
        { name: "count", required: false, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.exportPdf", kind: "wps", agent: true, undo: false,
      summary: "Xuất PDF",
      params: [
        { name: "path", required: true, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.save", kind: "wps", agent: true, undo: false,
      summary: "Lưu",
      params: [{ name: "document", required: false, hint: "full path of the document to work on; default: the active one" }] },
    { name: "writer.saveAs", kind: "wps", agent: true, undo: false,
      summary: "Lưu thành file mới",
      params: [
        { name: "path", required: true, hint: null },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "writer.closeAll", kind: "wps", agent: false, undo: false,
      summary: "**Đóng mọi tài liệu, không lưu**",
      params: [] },
    { name: "et.newWorkbook", kind: "et", agent: false, undo: false,
      summary: "Tạo sổ tính mới",
      params: [] },
    { name: "et.open", kind: "et", agent: false, undo: false,
      summary: "Mở .xlsx/.xls/.ods/.csv",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "et.listSheets", kind: "et", agent: true, undo: false,
      summary: "Danh sách sheet + sheet đang active",
      params: [{ name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }] },
    { name: "et.sheetInfo", kind: "et", agent: true, undo: false,
      summary: "Kích thước dùng thật của TỪNG sheet trong một lời gọi (`usedRange`/`rows`/`cols`/`empty`) — dùng để soát sheet nào thực sự có nội dung, không chỉ có tên",
      params: [{ name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }] },
    { name: "et.addSheet", kind: "et", agent: true, undo: true,
      summary: "Thêm sheet mới (bỏ trống `name` thì đặt tên Sheet1, Sheet2...)",
      params: [
        { name: "name", required: false, hint: null },
        { name: "index", required: false, hint: "0-based, default: append" },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.renameSheet", kind: "et", agent: true, undo: true,
      summary: "Đổi tên sheet",
      params: [
        { name: "sheet", required: true, hint: null },
        { name: "name", required: true, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.deleteSheet", kind: "et", agent: true, undo: false,
      summary: "Xoá một sheet (sheet thừa như Sheet1 của sổ mới, sheet chính lượt này dựng sai); không xoá được sheet cuối cùng",
      params: [
        { name: "sheet", required: true, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.activateSheet", kind: "et", agent: true, undo: false,
      summary: "Chuyển sheet",
      params: [
        { name: "sheet", required: true, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.readRange", kind: "et", agent: true, undo: false,
      summary: "Đọc vùng, ví dụ `A1:C10`; `formulas=true` trả về công thức (ô không có công thức = null)",
      params: [
        { name: "range", required: true, hint: null },
        { name: "sheet", required: false, hint: null },
        { name: "formulas", required: false, hint: "true = trả mảng `formulas` thay cho `values`" },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.readRanges", kind: "et", agent: true, undo: false,
      summary: "Đọc NHIỀU vùng (nhiều sheet) trong MỘT lời gọi: `reads` là mảng {range, sheet?, formulas?}; `range` nhận cả 'Sheet!A1:B6'. Dùng thay cho nhiều lần et.readRange khi soát lại (tối đa 4000 ô mỗi lần)",
      params: [
        { name: "reads", required: true, hint: "array of {range, sheet?, formulas?}" },
        { name: "sheet", required: false, hint: "default sheet for items without one" },
        { name: "formulas", required: false, hint: "true = formulas for every item" },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.whatIf", kind: "et", agent: true, undo: false,
      summary: "Thử ô điều khiển SỐNG hay CHẾT trong MỘT lời gọi (chỉ đọc): đổi tạm ô giả định `cell` thành `value`, tính lại, đọc các vùng `watch` trước/sau, rồi TỰ TRẢ LẠI giá trị cũ. `unchanged` = vùng không đổi theo (công thức không đọc ô đó). Thay cho tự ghi giá trị thử -> đọc -> ghi lại",
      params: [
        { name: "cell", required: true, hint: "one input cell e.g. 'Config!B2'" },
        { name: "value", required: true, hint: "test value e.g. 0.95" },
        { name: "watch", required: true, hint: "ranges that should follow e.g. ['Dashboard!B3:B6']" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.writeRange", kind: "et", agent: true, undo: true,
      summary: "Ghi vùng từ ô góc trên-trái; `values` là mảng 2 chiều. Công thức viết theo cú pháp en-US (dấu phẩy); nếu máy dùng dấu chấm phẩy thì tự đổi. Công thức còn lỗi trả về ở `formulaErrors`",
      params: [
        { name: "range", required: true, hint: "top-left cell e.g. 'A1'" },
        { name: "values", required: true, hint: "2D array of rows e.g. [[\"Tên\",\"Điểm\"],[\"An\",9.5]]" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.importCsv", kind: "et", agent: true, undo: true,
      summary: "Nạp một file CSV vào sổ: không có `range` thì tạo sheet mới đặt tên theo tên file, có `range` thì ghi vào ô góc đó. Ô trông như số thì thành số, còn lại là chuỗi; ngày để nguyên chuỗi",
      params: [
        { name: "path", required: true, hint: "e.g. 'D:\\data\\raw.csv'" },
        { name: "range", required: false, hint: "top-left cell; bỏ trống thì tạo sheet mới" },
        { name: "sheet", required: false, hint: null },
        { name: "delimiter", required: false, hint: "một ký tự, mặc định ','" },
        { name: "encoding", required: false, hint: "mặc định utf-8-sig" },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.writeRanges", kind: "et", agent: true, undo: true,
      summary: "Ghi NHIỀU vùng trong MỘT lời gọi: `writes` là mảng {range, values, sheet?}. Dùng khi cần viết nhiều khối trong cùng một phản hồi (bớt vòng qua bridge); công thức vẫn theo cú pháp en-US và lỗi công thức gom ở `formulaErrors`. Kiểm hết tham số trước khi ghi: một vùng sai thì không vùng nào được ghi",
      params: [
        { name: "writes", required: true, hint: "array of {range, values, sheet?}" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.fillRange", kind: "et", agent: true, undo: true,
      summary: "Viết MỘT công thức vào ô góc trên-trái của `range` rồi điền ra cả vùng, tham chiếu tương đối tự dịch (dùng cho bảng nghìn dòng: đừng gửi từng ô)",
      params: [
        { name: "range", required: true, hint: "the whole area e.g. 'B2:H1000'" },
        { name: "formula", required: false, hint: "written to the top-left cell first" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.formatRange", kind: "et", agent: true, undo: true,
      summary: "Định dạng MỘT vùng (màu dạng `#RRGGBB`, `horizontal` left/center/right); nhiều vùng thì dùng et.formatRanges",
      params: [
        { name: "range", required: true, hint: null },
        { name: "bold", required: false, hint: null },
        { name: "italic", required: false, hint: null },
        { name: "fontSize", required: false, hint: null },
        { name: "fontColor", required: false, hint: null },
        { name: "fillColor", required: false, hint: null },
        { name: "numFmt", required: false, hint: null },
        { name: "horizontal", required: false, hint: null },
        { name: "wrap", required: false, hint: null },
        { name: "columnWidth", required: false, hint: "width of the range's columns in characters, e.g. 20" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.formatRanges", kind: "et", agent: true, undo: true,
      summary: "Định dạng NHIỀU vùng trong MỘT lời gọi: `formats` là mảng {range, sheet?, bold, italic, fontSize, fontColor, fillColor, numFmt, horizontal, wrap, columnWidth} (cùng thuộc tính như et.formatRange). Dùng thay cho nhiều lần et.formatRange (tiêu đề, ô nhập liệu, định dạng số của mọi sheet trong một lần). Kiểm hết trước khi định dạng: một mục sai thì không vùng nào bị đổi",
      params: [
        { name: "formats", required: true, hint: "array of {range, sheet?, ...format}" },
        { name: "sheet", required: false, hint: "default sheet for items without one" },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.setConditionalFormat", kind: "et", agent: true, undo: true,
      summary: "Đặt định dạng điều kiện cho vùng: `rules` là mảng rule, mỗi rule có operator (less/lessEqual/greater/greaterEqual/equal/notEqual/between/notBetween/formula), formula1, formula2 (chỉ between), và bold/italic/fontColor/fillColor; HOẶC `type` trực quan: colorScale {colors: 2-3 màu thấp→cao}, dataBar {color}, iconSet {icons: 3arrows/3traffic/3flags/3symbols/4arrows/5arrows}, top/bottom {rank, percent} + màu, aboveAverage/belowAverage + màu, duplicate/unique + màu. Gọi lại trên cùng vùng thì THAY rule cũ của vùng đó, nên nhiều rule trên một vùng phải để trong MỘT lần gọi",
      params: [
        { name: "range", required: true, hint: null },
        { name: "rules", required: true, hint: "array of rule objects" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.listConditionalFormats", kind: "et", agent: true, undo: false,
      summary: "Liệt kê định dạng điều kiện đang có (đọc lại để tự kiểm); bỏ trống `range` thì cả sheet",
      params: [
        { name: "range", required: false, hint: "chỉ xem một vùng" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.addChart", kind: "et", agent: true, undo: true,
      summary: "Chèn biểu đồ từ vùng dữ liệu; `type` column/bar/line/pie/area/scatter, `width`/`height` tính bằng cm",
      params: [
        { name: "range", required: true, hint: "source data e.g. 'A1:B13'" },
        { name: "type", required: false, hint: "column (default)/bar/line/pie/area/scatter" },
        { name: "title", required: false, hint: null },
        { name: "name", required: false, hint: null },
        { name: "anchor", required: false, hint: "top-left cell, default 'A1'" },
        { name: "width", required: false, hint: "cm, default 12" },
        { name: "height", required: false, hint: "cm, default 7" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.listCharts", kind: "et", agent: true, undo: false,
      summary: "Danh sách biểu đồ trên sheet",
      params: [
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.createTable", kind: "et", agent: true, undo: true,
      summary: "Biến vùng dữ liệu (hàng đầu là tiêu đề) thành Excel Table: có nút lọc, tự mở rộng, công thức theo cột",
      params: [
        { name: "range", required: true, hint: "data incl. header row, e.g. 'A1:I4'" },
        { name: "sheet", required: false, hint: null },
        { name: "name", required: false, hint: "table name, e.g. 'tblProjects'" },
        { name: "style", required: false, hint: "e.g. 'TableStyleMedium2' (default)" },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.freezePanes", kind: "et", agent: true, undo: true,
      summary: "Cố định hàng/cột tiêu đề khi cuộn (rows = số hàng trên cùng, cols = số cột bên trái; 0/0 = bỏ cố định)",
      params: [
        { name: "rows", required: false, hint: "top rows to keep visible, default 1" },
        { name: "cols", required: false, hint: "left columns to keep visible, default 0" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.setValidation", kind: "et", agent: true, undo: true,
      summary: "Kiểm tra dữ liệu nhập cho vùng: danh sách chọn (`list` hoặc `source` là vùng chứa danh sách) hoặc số/ngày trong khoảng (`type` + min/max); thay quy tắc cũ của vùng",
      params: [
        { name: "range", required: true, hint: null },
        { name: "list", required: false, hint: "array of allowed values" },
        { name: "source", required: false, hint: "range holding the allowed values, e.g. 'Config!$B$2:$B$5'" },
        { name: "type", required: false, hint: "list (default when list/source) / whole / decimal / date" },
        { name: "min", required: false, hint: null },
        { name: "max", required: false, hint: null },
        { name: "message", required: false, hint: "error message shown on invalid input" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.undo", kind: "et", agent: true, undo: false,
      summary: "Hoàn tác",
      params: [
        { name: "count", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.exportPdf", kind: "et", agent: true, undo: false,
      summary: "Xuất PDF",
      params: [
        { name: "path", required: true, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.save", kind: "et", agent: true, undo: false,
      summary: "Lưu",
      params: [{ name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }] },
    { name: "et.saveAs", kind: "et", agent: true, undo: false,
      summary: "Lưu thành file mới",
      params: [
        { name: "path", required: true, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.closeAll", kind: "et", agent: false, undo: false,
      summary: "**Đóng mọi sổ, không lưu**",
      params: [] },
    { name: "wpp.newPresentation", kind: "wpp", agent: false, undo: false,
      summary: "Tạo bài trình bày mới",
      params: [] },
    { name: "wpp.open", kind: "wpp", agent: false, undo: false,
      summary: "Mở .pptx/.ppt/.odp",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "wpp.listSlides", kind: "wpp", agent: true, undo: false,
      summary: "Số slide + text từng slide",
      params: [
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" },
        { name: "detail", required: false, hint: "true = kem master/notes" }
      ] },
    { name: "wpp.addSlide", kind: "wpp", agent: true, undo: true,
      summary: "Thêm slide cuối; `layout` mặc định 12 = trống (1 = tiêu đề, 2 = tiêu đề + nội dung, 11 = chỉ tiêu đề)",
      params: [
        { name: "layout", required: false, hint: "number: 1 title, 2 title+content, 11 title only, 12 blank" },
        { name: "index", required: false, hint: "position of the new slide (1 = first); default: after the last slide" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addText", kind: "wpp", agent: true, undo: true,
      summary: "Textbox có định dạng (`color` dạng `#RRGGBB`, `align` left/center/right)",
      params: [
        { name: "text", required: true, hint: null },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "fontSize", required: false, hint: null },
        { name: "bold", required: false, hint: null },
        { name: "color", required: false, hint: null },
        { name: "align", required: false, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addTextBox", kind: "wpp", agent: false, undo: true,
      summary: "Textbox",
      params: [
        { name: "text", required: true, hint: null },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addImage", kind: "wpp", agent: true, undo: true,
      summary: "Chèn ảnh (kích thước gốc nếu bỏ trống `width`/`height`)",
      params: [
        { name: "path", required: true, hint: null },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addTable", kind: "wpp", agent: true, undo: true,
      summary: "Bảng; `rows`/`cols` tự suy ra/nới theo `values`",
      params: [
        { name: "rows", required: false, hint: null },
        { name: "cols", required: false, hint: null },
        { name: "values", required: false, hint: "2D array of rows" },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.setNotes", kind: "wpp", agent: true, undo: true,
      summary: "Ghi chú thuyết trình",
      params: [
        { name: "text", required: true, hint: null },
        { name: "slide", required: false, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.deleteSlide", kind: "wpp", agent: true, undo: true,
      summary: "Xoá slide (mặc định slide cuối)",
      params: [
        { name: "slide", required: false, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.exportPdf", kind: "wpp", agent: true, undo: false,
      summary: "Xuất PDF",
      params: [
        { name: "path", required: true, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.save", kind: "wpp", agent: true, undo: false,
      summary: "Lưu",
      params: [{ name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }] },
    { name: "wpp.saveAs", kind: "wpp", agent: true, undo: false,
      summary: "Lưu thành file mới",
      params: [
        { name: "path", required: true, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.newFromTemplate", kind: "wpp", agent: true, undo: false,
      summary: "Tạo bài MỚI từ tệp mẫu (.pptx/.potx): giữ khổ + master, bỏ hết slide mẫu, lưu ở `path` và mở ra",
      params: [
        { name: "template", required: true, hint: "path of the template file" },
        { name: "path", required: true, hint: "new .pptx to create; must not exist" }
      ] },
    { name: "wpp.addSlideFromTemplate", kind: "wpp", agent: true, undo: true,
      summary: "Chèn bản sao slide số `kind` của tệp mẫu (giữ ảnh nền, khung chữ, định dạng); trả tên các khung chữ",
      params: [
        { name: "template", required: true, hint: null },
        { name: "kind", required: true, hint: "slide number in the template (kinds of ppt_profile)" },
        { name: "slide", required: false, hint: "insert after this slide; default: at the end" },
        { name: "tag", required: false, hint: "source slide number; a slide already tagged so is replaced in place" },
        { name: "expect", required: false, hint: "shape name the inserted slide must have (checks it is the template slide)" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.sourceManifest", kind: "wpp", agent: true, undo: false,
      summary: "Đọc bài nguồn (vd bài cũ .ppt) theo slide: tiêu đề, chữ, danh sách ảnh - để dựng lại theo mẫu",
      params: [
        { name: "from", required: true, hint: "path of the source presentation" },
        { name: "first", required: false, hint: null },
        { name: "last", required: false, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.setShapeText", kind: "wpp", agent: true, undo: true,
      summary: "Thay chữ của một khung theo TÊN shape, giữ định dạng của khung (font, cỡ, màu)",
      params: [
        { name: "shape", required: true, hint: "shape name, e.g. 'TextBox 2'" },
        { name: "text", required: true, hint: "\\n = new paragraph" },
        { name: "slide", required: false, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.deleteShapes", kind: "wpp", agent: true, undo: true,
      summary: "Xoá các shape theo tên (mục lặp thừa của slide mẫu); không xoá ảnh nền",
      params: [
        { name: "shapes", required: false, hint: "array of shape names" },
        { name: "slide", required: false, hint: null },
        { name: "emptyPlaceholders", required: false, hint: "true: also delete every placeholder without text (the 'Click to add text' boxes)" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.copyPicture", kind: "wpp", agent: true, undo: true,
      summary: "Chép ảnh từ slide của bài khác (vd bài cũ) vào slide của bài đang làm, co vừa khung, giữ tỉ lệ",
      params: [
        { name: "from", required: true, hint: "path of the source presentation" },
        { name: "fromSlide", required: true, hint: null },
        { name: "shape", required: false, hint: "picture name; default: largest picture" },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "overlays", required: false, hint: "true: also copy the labels/arrows drawn over the picture" },
        { name: "format", required: false, hint: "png (default) or jpg (much smaller for photos and scans)" },
        { name: "region", required: false, hint: "[left, top, width, height] of the source slide to copy as one picture (a diagram or table drawn with shapes); replaces shape" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addParagraphs", kind: "wpp", agent: false, undo: true,
      summary: "Khung chữ nhiều đoạn giữ phân cấp (cấp thụt lề, đậm, nghiêng, số thứ tự, gạch đầu dòng); tự chọn cỡ chữ lớn nhất vừa khung",
      params: [
        { name: "paragraphs", required: true, hint: "array of {t: text, l: level 1-5, b: bold, i: italic, n: number prefix like '1.', u: bullet}" },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "fontSize", required: false, hint: "largest size to try (default 24)" },
        { name: "minFontSize", required: false, hint: "smallest size (default 16)" },
        { name: "color", required: false, hint: "#RRGGBB body text" },
        { name: "accent", required: false, hint: "#RRGGBB bold (heading) paragraphs" },
        { name: "font", required: false, hint: "font name" },
        { name: "anchor", required: false, hint: "top (default) or middle" },
        { name: "box", required: false, hint: "rounded or rect: draw a filled box (card, callout) instead of a plain text box" },
        { name: "fill", required: false, hint: "#RRGGBB box fill" },
        { name: "line", required: false, hint: "#RRGGBB box border (none when omitted)" },
        { name: "highlight", required: false, hint: "#RRGGBB colour of **emphasised** words (default: accent)" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addGrid", kind: "wpp", agent: false, undo: true,
      summary: "Bảng có định dạng: dòng đầu là tiêu đề (nền màu, chữ trắng đậm), dòng xen kẽ nền nhạt, cột rộng theo nội dung, tự chọn cỡ chữ vừa khung",
      params: [
        { name: "values", required: true, hint: "2D array; first row = header" },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "fontSize", required: false, hint: "largest size to try (default 18)" },
        { name: "minFontSize", required: false, hint: "smallest size (default 11)" },
        { name: "header", required: false, hint: "#RRGGBB header fill" },
        { name: "band", required: false, hint: "#RRGGBB fill of every second row" },
        { name: "color", required: false, hint: "#RRGGBB text" },
        { name: "font", required: false, hint: "font name" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.deleteTagged", kind: "wpp", agent: false, undo: true,
      summary: "Xoá các slide do công cụ dựng (có tag nguồn) mà tag bắt đầu trong [first, last] - dựng lại một lô theo bố cục mới",
      params: [
        { name: "first", required: true, hint: null },
        { name: "last", required: true, hint: null },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addShape", kind: "wpp", agent: false, undo: true,
      summary: "Hình khối có chữ (chữ nhật, bo góc, tròn, mũi tên chevron) - nền, viền, cỡ chữ, màu chữ",
      params: [
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "shape", required: false, hint: "rect, rounded, oval, chevron, line (default rect)" },
        { name: "text", required: false, hint: null },
        { name: "fontSize", required: false, hint: null },
        { name: "bold", required: false, hint: null },
        { name: "boldFirst", required: false, hint: "true = only the first line bold (heading of the box)" },
        { name: "color", required: false, hint: "#RRGGBB text" },
        { name: "fill", required: false, hint: "#RRGGBB fill; empty = none" },
        { name: "line", required: false, hint: "#RRGGBB outline; empty = none" },
        { name: "dash", required: false, hint: "true = dashed outline" },
        { name: "font", required: false, hint: "font name" },
        { name: "anchor", required: false, hint: "top or middle (default)" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.renderSlide", kind: "wpp", agent: false, undo: false,
      summary: "Ảnh PNG của một slide theo số (soát bằng mắt, benchmark) - base64, không đổi cửa sổ đang xem",
      params: [
        { name: "slide", required: true, hint: null },
        { name: "maxWidth", required: false, hint: "pixels (default 1280)" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addConnector", kind: "wpp", agent: false, undo: true,
      summary: "Đường nối có mũi tên giữa hai hình (bám theo hình khi kéo)",
      params: [
        { name: "from", required: true, hint: "shape name" },
        { name: "to", required: true, hint: "shape name" },
        { name: "slide", required: false, hint: null },
        { name: "elbow", required: false, hint: "true = elbow connector (default straight)" },
        { name: "direction", required: false, hint: "down, up, right or left: leave 'from' on that side and enter 'to' on the opposite side; default: nearest sides" },
        { name: "fromSide", required: false, hint: "top, bottom, left or right side of 'from' (overrides direction)" },
        { name: "toSide", required: false, hint: "top, bottom, left or right side of 'to' (overrides direction); same side as fromSide + elbow = a loop around the row" },
        { name: "offset", required: false, hint: "loop only: how far (pt) the loop goes beyond the farther end (default: PowerPoint's choice)" },
        { name: "color", required: false, hint: "#RRGGBB" },
        { name: "weight", required: false, hint: "points" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.addChart", kind: "wpp", agent: false, undo: true,
      summary: "Biểu đồ gốc (cột, thanh, đường, tròn) từ danh mục và chuỗi số - có nhãn số liệu, sửa được bằng Edit Data",
      params: [
        { name: "categories", required: true, hint: "array of labels" },
        { name: "series", required: true, hint: "array of {name, values: [numbers]}; pie: one series" },
        { name: "type", required: false, hint: "column (default), bar, line, pie" },
        { name: "unit", required: false, hint: "suffix of value labels, e.g. '%' or ' tỷ'" },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null },
        { name: "colors", required: false, hint: "array of #RRGGBB, one per series (pie: per slice)" },
        { name: "font", required: false, hint: "font name" },
        { name: "fontSize", required: false, hint: "points (default 16)" },
        { name: "color", required: false, hint: "#RRGGBB text" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "wpp.closeAll", kind: "wpp", agent: false, undo: false,
      summary: "**Đóng mọi bài trình bày, không lưu**",
      params: [] },
    { name: "writer.checkTables", kind: "wps", agent: true, undo: false,
      summary: "Soát các bảng (chỉ đọc): số dòng/cột, ô trống, ô tiêu đề lẫn đoạn văn. Bảng dàn trang — bảng đã gọi writer.formatTable với borders:false — được đánh dấu layout và KHÔNG báo empty-cells (ô trống ở đó là cố ý: khoảng trống ký, ô chờ điền)",
      params: [{ name: "document", required: false, hint: "full path of the document to work on; default: the active one" }] },
    { name: "writer.checkLayout", kind: "wps", agent: true, undo: false,
      summary: "Soát phân trang (chỉ đọc): số trang, số chữ, trang trắng, tiêu đề nằm cuối trang tách khỏi nội dung, bảng nhỏ bị cắt sang trang, đoạn trống liên tiếp; trang của từng tiêu đề - gọi sau khi viết xong, trước khi trả lời",
      params: [
        { name: "maxPages", required: false, hint: "page limit the user asked for (e.g. 8 for '6-8 pages'): longer = issue too-long with how much to cut" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "et.checkRange", kind: "et", agent: true, undo: false,
      summary: "Soát bảng dữ liệu (chỉ đọc): tiêu đề trống, kiểu lẫn lộn, số dạng chữ, number format, công thức trỏ ô trống; KHÔNG truyền range/sheet = soát CẢ SỔ trong một lần: `workbook` mỗi sheet một dòng (vùng dùng, số ô, công thức, biểu đồ, bảng, validation, định dạng điều kiện, loại lỗi; sheet rỗng = empty-sheet), lỗi mọi sheet gộp ở `issues` kèm `sheet`, ô lỗi (#DIV/0!, #REF!...) ở MỌI sheet. Sheet chia khối bằng hàng trống (Dashboard, Config) được soát theo TỪNG khối; `notes` (outside-table) là ghi chú, không tính vào issueCount",
      params: [
        { name: "range", required: false, hint: "default: the used range" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "wpp.checkLayout", kind: "wpp", agent: true, undo: false,
      summary: "Soát bố cục slide (chỉ đọc): chữ tràn khung, shape ra ngoài slide, shape chồng nhau, chữ quá nhỏ, đường nối cắt qua hộp khác hoặc chồng lên đường nối khác",
      params: [
        { name: "slide", required: false, hint: "slide number; default: every slide" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "writer.goto", kind: "wps", agent: false, undo: false,
      summary: "Chọn và cuộn tới một đoạn theo số của writer.outline (trích dẫn [§n] trong câu trả lời)",
      params: [
        { name: "paragraph", required: true, hint: "paragraph number from writer.outline" },
        { name: "text", required: false, hint: "first words of the paragraph, to find it if numbering moved" },
        { name: "document", required: false, hint: "full path of the document to work on; default: the active one" }
      ] },
    { name: "et.goto", kind: "et", agent: false, undo: false,
      summary: "Chọn và cuộn tới một ô/vùng (trích dẫn [Sheet!A1] trong câu trả lời)",
      params: [
        { name: "range", required: true, hint: "e.g. 'C42' or 'Data!C42'" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "wpp.goto", kind: "wpp", agent: false, undo: false,
      summary: "Chuyển tới một slide (trích dẫn [slide n] trong câu trả lời)",
      params: [
        { name: "slide", required: true, hint: "slide number" },
        { name: "presentation", required: false, hint: "full path of the presentation to work on; default: the active one" }
      ] },
    { name: "et.sort", kind: "et", agent: true, undo: true,
      summary: "Sắp xếp các hàng của một vùng theo 1-3 cột (`keys`: [{column: chữ tiêu đề hoặc chữ cột, order: asc|desc}]); mặc định hàng đầu là tiêu đề, không bị sắp xếp",
      params: [
        { name: "range", required: true, hint: "data incl. header row, e.g. 'A1:F200'" },
        { name: "keys", required: true, hint: "[{column: 'Doanh thu' or 'C', order: 'asc'|'desc'}], 1-3 keys" },
        { name: "header", required: false, hint: "true (default): first row is the header" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
    { name: "et.filter", kind: "et", agent: true, undo: false,
      summary: "Lọc hàng (AutoFilter) theo một cột: `values` = danh sách giá trị giữ lại, hoặc `criteria` như '>100'; `clear: true` bỏ mọi bộ lọc của sheet. Chỉ ẩn hàng, không xoá dữ liệu",
      params: [
        { name: "range", required: false, hint: "data incl. header row; default: the sheet's used range" },
        { name: "column", required: false, hint: "header text or column letter" },
        { name: "values", required: false, hint: "array of values to keep" },
        { name: "criteria", required: false, hint: "e.g. '>100', '<>0', '=Hà Nội'" },
        { name: "clear", required: false, hint: "true = remove all filters" },
        { name: "sheet", required: false, hint: null },
        { name: "workbook", required: false, hint: "full path of the workbook to work on; default: the active one" }
      ] },
  ];
  // --- END SINH TU DONG ---

  (function () {
    for (var i = 0; i < CATALOG.length; i++) {
      if (CATALOG[i].undo && CATALOG[i].kind === "wps") { MUTATING[CATALOG[i].name] = true; }
    }
  })();

  function errMessage(err) {
    try {
      if (err && typeof err.message === "string" && err.message) { return err.message; }
      return String(err);
    } catch (e) {
      return "unknown error";
    }
  }

  function hasParam(params, name) {
    return !!params && params[name] !== undefined && params[name] !== null;
  }

  function unwrapScalar(value) {
    var depth, k;
    for (depth = 0; depth < 8; depth++) {
      if (Array.isArray(value) && value.length === 1) {
        value = value[0];
        continue;
      }
      if (value !== null && typeof value === "object" && !Array.isArray(value)) {
        var keys = [];
        for (k in value) {
          if (Object.prototype.hasOwnProperty.call(value, k)) { keys.push(k); }
        }
        if (keys.length === 1 && (keys[0] === "item" || keys[0] === "items" || keys[0] === "value")) {
          value = value[keys[0]];
          continue;
        }
        return value;
      }
      return value;
    }
    return value;
  }

  function paramString(params, name, fallback) {
    if (!hasParam(params, name)) { return fallback; }
    var v = unwrapScalar(params[name]);
    if (v === undefined || v === null) { return fallback; }
    if (typeof v === "string") { return v; }
    return String(v);
  }

  function paramInt(params, name, fallback) {
    if (!hasParam(params, name)) { return fallback; }
    var v = unwrapScalar(params[name]);
    if (typeof v === "boolean") {
      throw new Error("'" + name + "' must be a whole number, got " + String(v));
    }
    var n;
    if (typeof v === "string") {
      var t = v.trim();
      if (t === "") { throw new Error("'" + name + "' must be a whole number, got ''"); }
      n = Number(t);
      if (isNaN(n) || !isFinite(n)) {
        throw new Error("'" + name + "' must be a whole number, got '" + v + "'");
      }
      return Math.round(n);
    }
    if (typeof v === "number") {
      if (isNaN(v) || !isFinite(v)) {
        throw new Error("'" + name + "' must be a whole number, got " + String(v));
      }
      return Math.round(v);
    }
    n = Number(v);
    if (isNaN(n) || !isFinite(n)) {
      throw new Error("'" + name + "' must be a whole number, got " + String(v));
    }
    return Math.round(n);
  }

  function paramBool(params, name, fallback) {
    if (!hasParam(params, name)) { return fallback; }
    var v = unwrapScalar(params[name]);
    if (typeof v === "boolean") { return v; }
    if (typeof v === "number") { return v !== 0; }
    var t = String(v).trim().toLowerCase();
    if (t === "true" || t === "1" || t === "yes" || t === "on") { return true; }
    if (t === "false" || t === "0" || t === "no" || t === "off" || t === "") { return t !== "" ? false : fallback; }
    if (t === "") { return fallback; }
    throw new Error("'" + name + "' must be true or false, got '" + String(v) + "'");
  }

  var MATRIX_WRAPPERS = { item: true, items: true, row: true, rows: true, values: true };

  function decodeArrays(value) {
    var k, keys, i, out;
    if (value !== null && typeof value === "object" && !Array.isArray(value)) {
      keys = [];
      for (k in value) {
        if (Object.prototype.hasOwnProperty.call(value, k)) { keys.push(k); }
      }
      if (keys.length === 1 && MATRIX_WRAPPERS[keys[0]]) {
        var inner = value[keys[0]];
        if (Array.isArray(inner)) {
          out = [];
          for (i = 0; i < inner.length; i++) { out.push(decodeArrays(inner[i])); }
          return out;
        }
        return [decodeArrays(inner)];
      }
      return value;
    }
    if (Array.isArray(value)) {
      out = [];
      for (i = 0; i < value.length; i++) { out.push(decodeArrays(value[i])); }
      return out;
    }
    return value;
  }

  function matrixHelp(name, got) {
    return "'" + name + "' must be a JSON 2D array (a list of rows), e.g. [[\"Họ tên\",\"Điểm\"],[\"An\",9.5]] - got " + got;
  }

  function describeValue(value) {
    if (value === null || value === undefined) { return "null"; }
    if (typeof value === "string") { return "'" + value + "'"; }
    if (Array.isArray(value)) { return "an array"; }
    return "a " + (typeof value) + " value";
  }

  function paramMatrix(params, name, required) {
    var raw = (params && params[name] !== undefined) ? params[name] : null;
    if (raw === undefined || raw === null) {
      if (required) { throw new Error(matrixHelp(name, "missing")); }
      return null;
    }
    if (typeof raw === "string") {
      var trimmed = raw.replace(/^\s+/, "");
      if (trimmed.charAt(0) === "[" || trimmed.charAt(0) === "{") {
        try { raw = JSON.parse(raw); } catch (e) {}
      } else {
        throw new Error(matrixHelp(name, describeValue(raw)));
      }
    }
    var data = decodeArrays(raw);
    if (!Array.isArray(data)) { throw new Error(matrixHelp(name, describeValue(data))); }
    while (data.length === 1 && Array.isArray(data[0]) && data[0].length > 0) {
      var only = data[0];
      var allLists = true;
      var j;
      for (j = 0; j < only.length; j++) {
        if (!Array.isArray(only[j])) { allLists = false; break; }
      }
      if (!allLists) { break; }
      data = only;
    }
    if (data.length === 0) {
      if (required) { throw new Error(matrixHelp(name, "an empty array")); }
      return [];
    }
    var allRows = true, allScalars = true, i;
    for (i = 0; i < data.length; i++) {
      if (Array.isArray(data[i])) { allScalars = false; }
      else if (data[i] !== null && typeof data[i] === "object") { allRows = false; allScalars = false; }
      else { allRows = false; }
    }
    if (!allRows && !allScalars) { throw new Error(matrixHelp(name, "a mix of rows and single values")); }
    var rows = [];
    for (var r = 0; r < data.length; r++) {
      var source = allRows ? data[r] : [data[r]];
      while (source.length === 1 && Array.isArray(source[0])) { source = source[0]; }
      var row = [];
      for (var c = 0; c < source.length; c++) {
        var cell = source[c];
        while (Array.isArray(cell) && cell.length === 1) { cell = cell[0]; }
        if (Array.isArray(cell) || (cell !== null && typeof cell === "object")) {
          throw new Error(matrixHelp(name, "a nested array/object at row " + (r + 1) + ", column " + (c + 1)));
        }
        row.push(cell);
      }
      rows.push(row);
    }
    return rows;
  }

  function cellText(value) {
    if (value === undefined || value === null) { return ""; }
    if (typeof value === "boolean") { return value ? "True" : "False"; }
    if (typeof value === "number" && isFinite(value) && Math.floor(value) === value) { return String(value); }
    return String(value);
  }

  function parseBgrColor(hex) {
    if (hex === undefined || hex === null) { return null; }
    var text = String(hex).trim();
    if (text.charAt(0) === "#") { text = text.substring(1); }
    text = text.trim();
    if (text.length !== 6) { return null; }
    var r, g, b;
    try {
      r = parseInt(text.substring(0, 2), 16);
      g = parseInt(text.substring(2, 4), 16);
      b = parseInt(text.substring(4, 6), 16);
    } catch (e) { return null; }
    if (isNaN(r) || isNaN(g) || isNaN(b)) { return null; }
    return r + (g << 8) + (b << 16);
  }

  function parseAlignment(name) {
    var key = (name === undefined || name === null) ? "left" : String(name).trim().toLowerCase();
    if (key === "center" || key === "centre") { return 1; }
    if (key === "right") { return 2; }
    if (key === "justify" || key === "justified" || key === "block") { return 3; }
    return 0;
  }

  function toWordFindText(text) {
    if (text === undefined || text === null) { return text; }
    var s = String(text);
    s = s.split("^").join("^^");
    s = s.split("\r\n").join("^p");
    s = s.split("\n").join("^p");
    s = s.split("\r").join("^p");
    s = s.split("\v").join("^l");
    s = s.split("\t").join("^t");
    return s;
  }

  function strProp(obj, name) {
    var v = obj[name];
    if (typeof v === "function") {
      try { v = v.call(obj); } catch (e) {}
    }
    if (v === undefined || v === null) { return ""; }
    return String(v);
  }

  // Doc mot thuoc ti so tu object model WPS. Tra null khi khong doc duoc (khong nem): object model WPS co
  // the tra ve ham thay vi gia tri, hoac nem loi tren mot so phien ban. Dung cho moi vi tri ky tu
  // (Selection.Start, Content.End...), la nhung thu KHONG the doan truoc giua cac ban WPS.
  function numProp(obj, name) {
    try {
      var v = obj[name];
      if (typeof v === "function") { v = v.call(obj); }
      if (v === undefined || v === null) { return null; }
      var n = Number(v);
      return isNaN(n) ? null : n;
    } catch (e) {
      return null;
    }
  }

  function applyFont(font, params) {
    if (hasParam(params, "bold")) { font.Bold = paramBool(params, "bold", false) ? -1 : 0; }
    if (hasParam(params, "italic")) { font.Italic = paramBool(params, "italic", false) ? -1 : 0; }
    if (hasParam(params, "underline")) { font.Underline = paramBool(params, "underline", false) ? 1 : 0; }
    if (hasParam(params, "size")) { font.Size = paramInt(params, "size", 12); }
    if (hasParam(params, "font")) { font.Name = paramString(params, "font", null); }
    var color = parseBgrColor(hasParam(params, "color") ? paramString(params, "color", null) : null);
    if (color !== null) { font.Color = color; }
  }

  function cmdAppInfo(app) {
    var kind = "wps";
    try { kind = appKind(app); } catch (eK) { kind = "wps"; }
    var name = "", version = "", document = null;
    try { name = strProp(app, "Name"); } catch (e) { name = ""; }
    try { version = strProp(app, "Version"); } catch (e2) { version = ""; }
    if (!version) {
      try {
        var build = strProp(app, "Build");
        if (build !== "") { version = build; }
      } catch (e4) {}
    }
    try {
      var doc = null;
      try {
        if (kind === "et") { doc = app.ActiveWorkbook; }
        else if (kind === "wpp") { doc = app.ActivePresentation; }
        else { doc = app.ActiveDocument; }
      } catch (eD) { doc = null; }
      if (doc !== undefined && doc !== null) {
        var dname = "", dfull = "";
        try { dname = strProp(doc, "Name"); } catch (e5) {}
        try { dfull = strProp(doc, "FullName"); } catch (e6) {}
        if (dname === "" && dfull === "") { document = null; }
        else { document = { name: dname, fullName: dfull }; }
      }
    } catch (e7) { document = null; }
    return { app: kind, name: name, version: version, document: document };
  }

  function cmdGetText(app, params) {
    var doc = app.ActiveDocument;
    var text = strProp(doc.Content, "Text");
    // strProp on Content.Text when Text is a plain string property works; when the
    // bridge exposes it differently, fall back to String().
    if (text === "" ) {
      try {
        var raw = doc.Content.Text;
        if (typeof raw === "function") { raw = raw.call(doc.Content); }
        if (raw !== undefined && raw !== null) {
          var s = String(raw);
          if (s !== "" && s !== "[object Object]") { text = s; }
          else if (s === "" ) { text = ""; }
        }
      } catch (e) {}
    }
    var total = text.length;
    var maxChars = paramInt(params, "maxChars", 0);
    var truncated = false;
    if (maxChars > 0 && text.length > maxChars) {
      text = text.substring(0, maxChars);
      truncated = true;
    }
    var name = "", fullName = "";
    try { name = strProp(doc, "Name"); } catch (e2) {}
    try { fullName = strProp(doc, "FullName"); } catch (e3) {}
    return { name: name, fullName: fullName, totalChars: total, truncated: truncated, text: text };
  }

  function cmdWriterOutline(app, params) {
    var doc = app.ActiveDocument;
    var maxChars = paramInt(params, "maxChars", 160);
    if (maxChars < 40) { maxChars = 40; }
    var items = [];
    var index = 0;
    var paras = doc.Paragraphs;
    var count = Number(paras.Count);
    for (var i = 1; i <= count; i++) {
      index++;
      var para = paras.Item(i);
      var range = para.Range;
      var text = "";
      // \x07 la ky tu ket o cua Word (van ban trong bang): bo de don cac dong bang.
      // eslint-disable-next-line no-control-regex -- \x07 la ky tu nghiep vu cua Word, khong phai loi
      try { text = String(range.Text || "").replace(/\r/g, "").replace(/\x07/g, "").trim(); } catch (eT) {}
      var style = "";
      try { style = String(para.Style.NameLocal || ""); } catch (eS) {}
      if (text === "" && (style === "Normal" || style === "")) { continue; }
      var entry = { n: index, t: text.length > maxChars ? text.substring(0, maxChars) + "..." : text };
      if (style !== "" && style !== "Normal") { entry.style = style; }
      items.push(entry);
    }
    return { paragraphs: index, items: items };
  }

  // Soat bang (chi doc) - cung quy tac voi checks.writer_check_tables cua LibreOffice
  // (src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py): o trong, va o tieu de lan doan van.
  // Nguong va cach tinh trung vi phai giu GIONG ban LibreOffice, neu khong cung mot tai lieu lai ra hai
  // ket qua khac nhau tuy ung dung - day la cung mot lenh trong cung mot hop dong.
  var LONG_HEADER_CHARS = 40;
  var LAYOUT_TABLE_NAME = "AxiomLayout";

  function tableCellText(raw) {
    // \r va \x07 la ky tu ket o cua Word/WPS: bo truoc khi do do dai, khong thi o trong bi tinh la co chu.
    // eslint-disable-next-line no-control-regex -- \x07 la ky tu nghiep vu cua Word, khong phai loi
    return String(raw === undefined || raw === null ? "" : raw).replace(/\r/g, "").replace(/\x07/g, "").trim();
  }

  // Bang dan trang: LibreOffice danh dau bang ten "AxiomLayout" (writer.formatTable {borders:false}).
  // WPS chua co writer.formatTable nen hom nay khong bang nao duoc danh dau - van doc ten de khi lenh do
  // duoc viet thi mien tru chay dung, va de truong `layout` trong bao cao noi that chu khong doan.
  function isLayoutTable(table) {
    var name = "";
    try { name = strProp(table, "Title"); } catch (e) { name = ""; }
    if (name === "") { try { name = strProp(table, "Name"); } catch (e2) { name = ""; } }
    return name.indexOf(LAYOUT_TABLE_NAME) === 0;
  }

  function cmdWriterCheckTables(app) {
    var doc = app.ActiveDocument;
    var tables = doc.Tables;
    var count = Number(tables.Count);
    if (isNaN(count)) { count = 0; }
    var issues = [], report = [], index, r, c, at, k;
    for (index = 1; index <= count; index++) {
      var table = tables.Item(index);
      var rows = Number(table.Rows.Count);
      var cols = Number(table.Columns.Count);
      var empty = 0, header = [];
      for (r = 1; r <= rows; r++) {
        for (c = 1; c <= cols; c++) {
          var text;
          // O bi gop: WPS nem loi khi hoi mot o khong ton tai - bo qua, giong ban LibreOffice.
          try { text = tableCellText(table.Cell(r, c).Range.Text); } catch (eCell) { continue; }
          if (text === "") { empty++; }
          if (r === 1) { header.push(text); }
        }
      }
      var layout = isLayoutTable(table);
      // O trong o bang dan trang la CO Y (khoang trong ky, o cho dien) nen khong bao; so lieu van de trong
      // report de agent thay. "long-header-cell" thi bao cho moi bang: voi bang dan trang do cung la loi that.
      if (empty > 0 && !layout) {
        issues.push({ type: "empty-cells", detail: "table " + index + " has " + empty + " empty cell(s)",
          table: index });
      }
      // Moi o tieu de so voi trung vi cac o tieu de CON LAI (leave-one-out), khong so voi trung vi ca nhom:
      // voi bang 2 cot, trung vi ca nhom chinh la o dai hon nen nguong "3 x trung vi" khong bao gio voi toi.
      // Bang 1 cot khong co nhom de so nen khong bao (xem docs/research/skills-review.md muc 4.7).
      for (at = 0; at < header.length; at++) {
        var others = [];
        for (k = 0; k < header.length; k++) { if (k !== at) { others.push(header[k].length); } }
        if (others.length === 0) { continue; }
        others.sort(function (a, b) { return a - b; });
        var typical = others[Math.floor(others.length / 2)];
        if (header[at].length > LONG_HEADER_CHARS && header[at].length > 3 * Math.max(1, typical)) {
          issues.push({ type: "long-header-cell",
            detail: "table " + index + " has a header cell with " + header[at].length + " characters ('" +
              header[at].substring(0, LONG_HEADER_CHARS) +
              "'): text that should be outside the table may have been typed into it",
            table: index });
        }
      }
      report.push({ index: index, rows: rows, cols: cols, emptyCells: empty, header: header, layout: layout });
    }
    return { tableCount: count, tables: report, issueCount: issues.length, issues: issues };
  }

  function cmdSelection(app) {
    var sel = app.Selection;
    return { text: strProp(sel, "Text"), start: numProp(sel, "Start"), end: numProp(sel, "End") };
  }

  function cmdTypeText(app, params) {
    var text = paramString(params, "text", null);
    if (text === null || text === undefined) { return { typed: 0 }; }
    app.Selection.TypeText(text);
    return { typed: text.length };
  }

  function cmdAppendText(app, params) {
    var text = paramString(params, "text", null);
    if (text === null || text === undefined || text === "") { return { appended: 0, paragraphs: 0 }; }
    var doc = app.ActiveDocument;
    var alignment = paramString(params, "alignment", "left");
    // '\n' dau chuoi chi la y dinh "mo doan": bo di, de chi con MOT lan ngat doan do needBreak quyet dinh
    // (tranh them doan rong khi caller da gui "\r...", va tranh Range bat dau tren paragraph mark cua doan cu).
    var normalized = String(text).replace(/\r\n/g, "\n").replace(/\r/g, "\n").replace(/^\n+/, "");
    if (!normalized.length) { return { appended: 0, paragraphs: 0 }; }
    var needBreak = false;
    try {
      var docParagraphs = doc.Paragraphs;
      var count = numProp(docParagraphs, "Count");
      if (count > 0) {
        var lastText = String(docParagraphs.Item(count).Range.Text)
          .replace(/\r/g, "").split("\u0007").join("").replace(/^\s+|\s+$/g, "");
        needBreak = lastText.length > 0;
      }
    } catch (eBreak) { needBreak = false; }
    var content = doc.Content;
    // Ngat doan TRUOC khi do vi tri bat dau: Range chi phu phan text moi, khong dat lai can le cua doan cu.
    if (needBreak) { content.InsertAfter("\r"); }
    var before = numProp(content, "End");
    content.InsertAfter(normalized.replace(/\n/g, "\r"));
    var after = numProp(content, "End");
    if (after > before) {
      try {
        var inserted = doc.Range(before, after);
        applyFontReset(inserted.Font, params);
        if (alignment) { inserted.ParagraphFormat.Alignment = parseAlignment(alignment); }
      } catch (eFmt) {
        // Host thieu doc.Range (ban WPS cu): text da noi duoc, chi bo qua dat lai dinh dang.
      }
    }
    return { appended: normalized.length, paragraphs: normalized.split("\n").length };
  }

  // Nhu applyFont nhung dat TUONG MINH ca thuoc tinh khong truyen: Selection giu bold/underline/mau cua lan
  // chen truoc, nen bo qua se lam dinh dinh dang sang cac doan sau (do 24/10/2026 tren Word). Chi dung cho
  // KHOI (append / insert co alignment); chen inline thi goi applyFont de giu dinh dang xung quanh.
  function applyFontReset(font, params) {
    font.Bold = hasParam(params, "bold") && paramBool(params, "bold", false) ? -1 : 0;
    font.Italic = hasParam(params, "italic") && paramBool(params, "italic", false) ? -1 : 0;
    font.Underline = hasParam(params, "underline") && paramBool(params, "underline", false) ? 1 : 0;
    font.Size = hasParam(params, "size") ? paramInt(params, "size", 13) : 13;
    font.Name = hasParam(params, "font") ? paramString(params, "font", "Times New Roman") : "Times New Roman";
    var color = parseBgrColor(hasParam(params, "color") ? paramString(params, "color", null) : null);
    font.Color = color !== null ? color : -16777216; // wdColorAutomatic
  }

  function cmdInsertStyledText(app, params) {
    var text = paramString(params, "text", null);
    if (!text) { return { inserted: 0, paragraphs: 0 }; }
    var alignment = paramString(params, "alignment", null);
    // Co alignment = mot KHOI doan van: tu ngat doan sau khoi de lan goi ke tiep khong dinh vao doan nay.
    // Khong alignment = chen inline, giu nguyen dinh dang xung quanh.
    var block = !!alignment;
    var breakAfter = hasParam(params, "break") ? paramBool(params, "break", true) : block;
    // '\n' (hoac '\r') = doan moi: model gui ca khoi van ban mot lan thay vi moi dong mot lenh.
    var lines = String(text).replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n");
    var sel = app.Selection;
    for (var i = 0; i < lines.length; i++) {
      if (block) { applyFontReset(sel.Font, params); } else { applyFont(sel.Font, params); }
      if (block) { sel.ParagraphFormat.Alignment = parseAlignment(alignment); }
      if (lines[i].length) { sel.TypeText(lines[i]); }
      if (i < lines.length - 1) { sel.TypeParagraph(); }
    }
    if (breakAfter && lines[lines.length - 1].length) { sel.TypeParagraph(); }
    return { inserted: text.length, paragraphs: lines.length };
  }

  function cmdHeading(app, params) {
    var level = paramInt(params, "level", 1);
    if (isNaN(level)) { level = 1; }
    level = Math.max(1, Math.min(9, level));
    var text = paramString(params, "text", null);
    var sel = app.Selection;
    if (text) { sel.TypeText(text); }
    var styleName = "Heading " + level;
    try {
      sel.set_Style(styleName);
    } catch (e) {
      sel.Style = styleName;
    }
    if (text && paramBool(params, "break", true)) {
      try { sel.TypeParagraph(); } catch (e2) {}
    }
    return { level: level };
  }

  function cmdFormatSelection(app, params) {
    applyFont(app.Selection.Font, params);
    if (hasParam(params, "alignment")) {
      app.Selection.ParagraphFormat.Alignment = parseAlignment(paramString(params, "alignment", "left"));
    }
    return { formatted: true };
  }

  function cmdSetParagraphAlignment(app, params) {
    var alignment = paramString(params, "alignment", "left");
    app.Selection.ParagraphFormat.Alignment = parseAlignment(alignment);
    return { alignment: alignment };
  }

  function tableAnchor(app) {
    var range = app.Selection.Range;
    try { range.Collapse(0); } catch (e) {}
    try {
      var info = null;
      try { info = range.Information(12); } catch (e2) { info = null; }
      var inside = false;
      try { inside = !!info; } catch (e3) {}
      // Number(mock deep) is 0 -> false; real Word returns -1/true when inside a table.
      try { inside = (Number(info) !== 0) && !!info; } catch (e4) {}
      if (inside) {
        range = range.Tables.Item(1).Range;
        try { range.Collapse(0); } catch (e5) {}
      }
    } catch (e6) {}
    return range;
  }

  function cmdInsertTable(app, params) {
    var valueRows = paramMatrix(params, "values", false);
    var rawRows = (params && params.rows !== undefined) ? params.rows : null;
    var rowsHoldData = valueRows === null && Array.isArray(rawRows);
    if (rowsHoldData) {
      valueRows = paramMatrix(params, "rows", false);
    }
    var rows = rowsHoldData ? 0 : paramInt(params, "rows", 0);
    var cols = paramInt(params, "cols", 0);
    if (valueRows !== null && valueRows.length > 0) {
      var maxCols = 0, i;
      for (i = 0; i < valueRows.length; i++) {
        if (valueRows[i].length > maxCols) { maxCols = valueRows[i].length; }
      }
      if (valueRows.length > rows) { rows = valueRows.length; }
      if (maxCols > cols) { cols = maxCols; }
    }
    if (!(rows > 0) || !(cols > 0)) {
      throw new Error("'rows' and 'cols' are required (or pass 'values' as a 2D array to size the table)");
    }
    var doc = app.ActiveDocument;
    var anchor = tableAnchor(app);
    var table = doc.Tables.Add(anchor, rows, cols);
    var filled = 0, r, c;
    if (valueRows !== null) {
      for (r = 0; r < valueRows.length && r < rows; r++) {
        var row = valueRows[r];
        for (c = 0; c < row.length && c < cols; c++) {
          table.Cell(r + 1, c + 1).Range.Text = cellText(row[c]);
          filled++;
        }
      }
    }
    var styleError = null;
    if (hasParam(params, "style")) {
      var styleName = paramString(params, "style", "Table Grid");
      try {
        try { table.Style = styleName; }
        catch (e) { table.set_Style(styleName); }
      } catch (e2) { styleError = errMessage(e2); }
    }
    try {
      var after = table.Range;
      after.Collapse(0);
      after.Select();
    } catch (e3) {}
    var result = { rows: rows, cols: cols, filled: filled };
    if (styleError !== null) { result.styleError = styleError; }
    return result;
  }

  function cmdInsertPageBreak(app) {
    app.Selection.InsertBreak(7);
    return { inserted: true };
  }

  function countOccurrences(haystack, needle) {
    if (needle === undefined || needle === null) { return 0; }
    var h = String(haystack === undefined || haystack === null ? "" : haystack);
    var n = String(needle);
    // Cung lop tuong duong voi toWordFindText: \r\n, \n, \r deu thanh ^p trong Find,
    // nen chuan hoa ve \n truoc khi dem (MatchCase=false -> so sanh thuong).
    h = h.split("\r\n").join("\n").split("\r").join("\n");
    n = n.split("\r\n").join("\n").split("\r").join("\n");
    h = h.toLowerCase();
    n = n.toLowerCase();
    if (n === "") { return 0; }
    var c = 0, pos = 0, idx;
    while (true) {
      idx = h.indexOf(n, pos);
      if (idx < 0) { break; }
      c++;
      pos = idx + n.length;
    }
    return c;
  }

  function cmdReplaceAll(app, params) {
    var find = paramString(params, "find", null);
    var replace = paramString(params, "replace", "");
    if (replace === null || replace === undefined) { replace = ""; }
    if (!find) { return { replaced: false, count: 0 }; }
    var doc = app.ActiveDocument;
    var content = doc.Content;
    // Dang khoa vung boi den: chi tim trong vung khoa (giong ban Windows), khong quet ca tai lieu. Khong
    // lay duoc Range thi tu choi han - tim ca tai lieu roi bi luoi an toan hoan tac chi lam loi kho hieu.
    if (selectionLock !== null) {
      var lockRange = lockBounds(app, selectionLock);
      if (lockRange !== null) {
        try {
          content = doc.Range(lockRange.start, lockRange.end);
        } catch (eLockRange) {
          throw new Error("cannot search inside the selection on this WPS build (Range.Find is unavailable)");
        }
      }
    }
    var findText = toWordFindText(find);
    var replaceText = toWordFindText(replace);
    // Dem truoc khi thay (khong tim lai chuoi vua thay) roi thay mot lan duy nhat.
    var count = 0;
    try {
      var docText = "";
      try { docText = strProp(content, "Text"); } catch (e) { docText = ""; }
      if (docText === "") {
        try {
          var raw = content.Text;
          if (typeof raw === "function") { raw = raw.call(content); }
          if (raw !== undefined && raw !== null) {
            var s = String(raw);
            if (s !== "[object Object]") { docText = s; }
          }
        } catch (e2) {}
      }
      count = countOccurrences(docText, find);
    } catch (e3) { count = 0; }
    content.Find.Execute(findText, false, false, false, false, false, true, 0, false, replaceText, 2);
    return { replaced: count > 0, count: count };
  }

  function cmdUndo(app, params) {
    var count = paramInt(params, "count", 1);
    if (isNaN(count)) { count = 1; }
    count = Math.max(1, count);
    var doc = app.ActiveDocument;
    var undone = 0;
    for (var i = 0; i < count; i++) {
      try {
        doc.Undo(1);
        undone++;
      } catch (e) { break; }
    }
    return { undone: undone };
  }

  function cmdSave(app) {
    var doc = app.ActiveDocument;
    doc.Save();
    var fullName = "";
    try { fullName = strProp(doc, "FullName"); } catch (e) {}
    return { saved: true, fullName: fullName };
  }

  function cmdSaveAs(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    var doc = app.ActiveDocument;
    try {
      doc.SaveAs2(path);
    } catch (e) {
      doc.SaveAs(path);
    }
    var fullName = "";
    try { fullName = strProp(doc, "FullName"); } catch (e2) {}
    if (!fullName) { fullName = path; }
    return { saved: true, fullName: fullName };
  }

  function pdfHex4(n) {
    return ("0000" + (n & 0xFFFF).toString(16)).slice(-4).toUpperCase();
  }

  // Ma hoa mot dong text thanh chuoi hex UTF-16BE co BOM (giu nguyen Unicode).
  function pdfHexUtf16(s) {
    var out = "FEFF";
    for (var i = 0; i < s.length; i++) {
      out += pdfHex4(s.charCodeAt(i));
    }
    return "<" + out + ">";
  }

  function pdfWrap(line, width) {
    var out = [];
    while (line.length > width) {
      var cut = width, sp = -1;
      for (var i = width; i > width - 30 && i > 0; i--) {
        if (line.charAt(i) === " ") { sp = i; break; }
      }
      if (sp > 0) { cut = sp; }
      out.push(line.substring(0, cut));
      line = (sp > 0) ? line.substring(cut + 1) : line.substring(cut);
    }
    out.push(line);
    return out;
  }

  // Dung tai lieu thanh cac dong ASCII-safe (da tach trang), toi da width ky tu.
  function pdfLines(text, width) {
    var t = String(text === undefined || text === null ? "" : text);
    t = t.split("\r\n").join("\n").split("\r").join("\n").split("\x07").join("\t");
    var raw = t.split("\n");
    var out = [];
    for (var i = 0; i < raw.length; i++) {
      // eslint-disable-next-line no-control-regex -- chu y: loc ky tu dieu khien khi lay text PDF
      var line = raw[i].split("\t").join("    ").replace(/[\x00-\x08\x0B\x0C\x0E-\x1F]/g, "");
      var parts = pdfWrap(line, width);
      for (var j = 0; j < parts.length; j++) { out.push(parts[j]); }
    }
    if (out.length === 0) { out.push(""); }
    return out;
  }

  function pdfUsedCodes(lines) {
    var seen = {}, codes = [], i, j;
    for (i = 0; i < lines.length; i++) {
      var s = lines[i];
      for (j = 0; j < s.length; j++) {
        var c = s.charCodeAt(j);
        if (!seen[c]) { seen[c] = true; codes.push(c); }
      }
    }
    codes.sort(function (a, b) { return a - b; });
    return codes;
  }

  // ToUnicode CMap: CID (= ma UTF-16) -> Unicode, chia khoi bfrange toi da 100 muc.
  function pdfCMap(codes) {
    var s = "/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n" +
      "/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n" +
      "/CMapName /AxiomUCS def\n/CMapType 2 def\n" +
      "1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n";
    var runs = [], start = -1, prev = -2, i;
    function flush() {
      if (start >= 0) { runs.push([start, prev]); start = -1; }
    }
    for (i = 0; i < codes.length; i++) {
      if (codes[i] === prev + 1 && start >= 0) { prev = codes[i]; }
      else { flush(); start = codes[i]; prev = codes[i]; }
    }
    flush();
    for (i = 0; i < runs.length; i += 100) {
      var block = runs.slice(i, i + 100);
      s += block.length + " beginbfrange\n";
      for (var j = 0; j < block.length; j++) {
        s += "<" + pdfHex4(block[j][0]) + "> <" + pdfHex4(block[j][1]) + "> <" + pdfHex4(block[j][0]) + ">\n";
      }
      s += "endbfrange\n";
    }
    s += "endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend";
    return s;
  }

  // PDF toi gian: A4, Helvetica, text Unicode (UTF-16BE + ToUnicode). Tat ca ASCII.
  function buildPdf(text) {
    var lines = pdfLines(text, 90);
    var perPage = 48, pages = [], i;
    for (i = 0; i < lines.length; i += perPage) {
      pages.push(lines.slice(i, i + perPage));
    }
    var cmap = pdfCMap(pdfUsedCodes(lines));
    var objs = [];
    // 1: catalog, 2: pages, 3: font, 4: cidfont, 5: tounicode; sau do tung cap page/content.
    var kids = [];
    for (i = 0; i < pages.length; i++) { kids.push((6 + i * 2) + " 0 R"); }
    objs[1] = "<< /Type /Catalog /Pages 2 0 R >>";
    objs[2] = "<< /Type /Pages /Kids [" + kids.join(" ") + "] /Count " + pages.length + " >>";
    objs[3] = "<< /Type /Font /Subtype /Type0 /BaseFont /Helvetica /Encoding /Identity-H " +
      "/DescendantFonts [4 0 R] /ToUnicode 5 0 R >>";
    objs[4] = "<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Helvetica " +
      "/CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /DW 1000 >>";
    objs[5] = "<< /Length " + cmap.length + " >>\nstream\n" + cmap + "\nendstream";
    for (i = 0; i < pages.length; i++) {
      var body = "BT /F1 11 Tf 14 TL 72 770 Td ";
      for (var j = 0; j < pages[i].length; j++) {
        if (pages[i][j] === "") { body += "T* "; }
        else { body += pdfHexUtf16(pages[i][j]) + " Tj T* "; }
      }
      body += "ET";
      var pid = 6 + i * 2, sid = 7 + i * 2;
      objs[pid] = "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] " +
        "/Resources << /Font << /F1 3 0 R >> >> /Contents " + sid + " 0 R >>";
      objs[sid] = "<< /Length " + body.length + " >>\nstream\n" + body + "\nendstream";
    }
    var maxId = 5 + pages.length * 2;
    var pdf = "%PDF-1.4\n";
    var offsets = [0];
    for (i = 1; i <= maxId; i++) {
      offsets[i] = pdf.length;
      pdf += i + " 0 obj\n" + objs[i] + "\nendobj\n";
    }
    var xrefAt = pdf.length;
    pdf += "xref\n0 " + (maxId + 1) + "\n";
    pdf += "0000000000 65535 f \n";
    for (i = 1; i <= maxId; i++) {
      var off = String(offsets[i]);
      while (off.length < 10) { off = "0" + off; }
      pdf += off + " 00000 n \n";
    }
    pdf += "trailer\n<< /Size " + (maxId + 1) + " /Root 1 0 R >>\n" +
      "startxref\n" + xrefAt + "\n%%EOF";
    return pdf;
  }

  function fsExists(fs, path) {
    try {
      var v = fs.Exists(path);
      if (typeof v === "function") { v = v.call(fs); }
      if (v === true || v === 1) { return true; }
      if (v === false || v === 0) { return false; }
    } catch (e) {}
    try { return !!fs.existsSync(path); } catch (e2) {}
    return false;
  }

  // Noi dung PDF (chi toan byte ASCII) -> chuoi nhi phan ma FileSystem.writeAsBinaryString nhan.
  function pdfBinaryString(pdf) {
    var codes = [];
    for (var i = 0; i < pdf.length; i++) { codes.push(pdf.charCodeAt(i) & 0xFF); }
    try {
      var parts = [];
      for (var k = 0; k < codes.length; k += 4096) {
        parts.push(String.fromCharCode.apply(null, codes.slice(k, k + 4096)));
      }
      return parts.join("");
    } catch (e) {
      var data = "";
      for (var m = 0; m < pdf.length; m++) { data += String.fromCharCode(pdf.charCodeAt(m) & 0xFF); }
      return data;
    }
  }

  // Duong du phong dung chung cho Writer/ET/WPP: tu dung PDF tu text roi ghi that, kiem tra lai file.
  function writePdf(fs, path, text, appLabel) {
    if (!fs || typeof fs.writeAsBinaryString !== "function") {
      throw new Error("WPS " + appLabel + " does not expose app.FileSystem, so the PDF could not be written or verified");
    }
    fs.writeAsBinaryString(path, pdfBinaryString(buildPdf(text)));
    if (!fsExists(fs, path)) { throw new Error("could not write PDF to '" + path + "'"); }
    return { exported: path };
  }

  function cmdExportPdf(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    var doc = app.ActiveDocument;
    // Xuat goc cua WPS truoc (dep nhat khi host ho tro). WPS Linux bo qua lenh nay
    // am tham nen phai kiem tra file that; that bai thi tu dung PDF text.
    try { doc.ExportAsFixedFormat(path, 17); } catch (e0) {}
    var wfs = null;
    try { wfs = app.FileSystem; } catch (e1) { wfs = null; }
    if (wfs === undefined || wfs === null) {
      // Khong co FileSystem thi khong kiem duoc file, cung khong tu dung duoc PDF: bao loi ro rang
      // thay vi tra {exported} trong khi tren dia khong co gi (ET/WPP cung nem o nhanh nay).
      throw new Error("WPS Writer does not expose app.FileSystem, so the PDF could not be written or verified");
    }
    if (fsExists(wfs, path)) { return { exported: path }; }
    var text = "";
    try { text = strProp(doc.Content, "Text"); } catch (e2) {}
    return writePdf(wfs, path, text, "Writer");
  }

  // ---- Khoa vung boi den (port cua CommandDispatcher.WriterScope.cs ben C#) --------------------
  //
  // Nguoi dung boi den roi hoi thi chi vung boi den duoc doi. Vung khoa nho bang phan chu TRUOC va SAU
  // no, khong bang mot Range: AI viet lai ca doan da chon thi Range cu co the ve mot diem, con "giua
  // phan truoc va phan sau" van dung du chua ben trong dai ra hay ngan di.
  //
  // `selectionLock` la trang thai cua CA add-in (mot tien trinh WPS = mot add-in), giong `_selectionLock`
  // ben C#. Vong doi: Core goi writer.lockSelection truoc luot chay va writer.unlockSelection khi luot
  // xong - Core giu bridge nen pane khong phai biet gi.
  var selectionLock = null;

  // Lenh chay nhu thuong khi dang khoa: lenh doc (khong doi tai lieu), writer.undo (hoan tac la viec
  // nguoi dung goi), va chinh hai lenh khoa - neu khong, lockBegin se keo con tro ve vung khoa CU truoc
  // khi cmdLockSelection kip doc vung nguoi dung vua boi den.
  var LOCK_READ_ONLY = {
    "writer.getText": true,
    "writer.selection": true,
    "writer.outline": true,
    "writer.undo": true,
    "writer.save": true,
    "writer.saveAs": true,
    "writer.exportPdf": true,
    "writer.lockSelection": true,
    "writer.unlockSelection": true
  };

  // Lenh luon cham ra ngoai vung chon: tu choi truoc, noi ro vi sao. Giu du ca cac lenh add-in Windows
  // co ma WPS chua viet handler (newDocument/open/closeAll/restyle/insertBlocks): khi nao co handler thi
  // khoa phai tu choi chung, va danh sach nay la cho ghi dieu do.
  var LOCK_REFUSED = {
    "writer.appendText": "appends at the end of the document",
    "writer.restyle": "restyles the whole document",
    "writer.insertBlocks": "inserts blocks across the document",
    "writer.newDocument": "creates another document",
    "writer.open": "opens another document",
    "writer.closeAll": "closes the documents",
    // InsertBreak THAY ca vung chon bang dau ngat: khoa vung luon co vung chon nen lenh nay xoa ca doan
    // da boi den. Muon ngat trang thi bo boi den roi yeu cau.
    "writer.insertPageBreak": "replaces the whole selected passage with a page break"
  };

  // Lenh THAY ca vung chon bang chu moi: vo hai voi vung chu, nhung lam MAT bang nam tron trong vung.
  // Luoi an toan sau khi chay (so bang truoc/sau) bat not cac lenh khong nam trong danh sach nay.
  var LOCK_TABLE_REFUSED = {
    "writer.typeText": "it would replace the table with plain text",
    "writer.insertStyledText": "it would replace the table with plain text",
    "writer.heading": "it would replace or restyle the table as a heading",
    "writer.insertHyperlink": "it would replace the table with the link text"
  };

  function lockMessage(problem) {
    return "Refused: the user selected part of the document, so only the selected text may change (" + problem +
      "). Edit inside the selection: writer.replaceAll searches only the selection, writer.typeText replaces it, " +
      "writer.formatSelection formats it. If the request really needs other parts, stop and tell the user to " +
      "clear the selection and ask again.";
  }

  function tableLockMessage(problem) {
    return "Refused: the selection contains a whole table; " + problem + ". The table is protected: edit text " +
      "in its cells with writer.replaceAll (it searches only inside the selection) and restyle with " +
      "writer.formatSelection. If the user really wants the table removed or its rows changed, stop and tell " +
      "them to clear the selection and ask again.";
  }

  // Chu cua doan [from, to) trong tai lieu. null = khong doc duoc (khac "" la doan rong).
  function docRangeText(doc, from, to) {
    try {
      var range = doc.Range(from, to);
      var raw = range.Text;
      if (typeof raw === "function") { raw = raw.call(range); }
      if (raw === undefined || raw === null) { return null; }
      return String(raw);
    } catch (e) {
      return null;
    }
  }

  // Cac bang cua tai lieu dang o [start, end) nao. known=false: build WPS nay khong doc duoc Tables.
  function lockTableRanges(doc) {
    var out = [];
    try {
      var tables = doc.Tables;
      var count = Number(tables.Count);
      if (isNaN(count)) { return null; }
      for (var i = 1; i <= count; i++) {
        var item = tables.Item(i);
        var range = item.Range;
        var s = numProp(range, "Start");
        var e = numProp(range, "End");
        if (s === null || e === null) { return null; }
        out.push([s, e]);
      }
      return out;
    } catch (err) {
      return null;
    }
  }

  // So bang NAM TRON trong [start, end): bang nam tron moi co the bi xoa ca khi thay vung chon; bang nam
  // mot phan (con tro trong o) van sua chu binh thuong duoc.
  function countWholeTables(ranges, start, end) {
    if (ranges === null) { return 0; }
    var whole = 0;
    for (var i = 0; i < ranges.length; i++) {
      if (ranges[i][0] >= start && ranges[i][1] <= end) { whole++; }
    }
    return whole;
  }

  // Vi tri dau/cuoi HIEN TAI cua vung khoa. null = khoa khong ap cho tai lieu nay (khac tai lieu thi
  // khoa coi nhu khong co, dung nhu C#: doi tai lieu thi khoa cu vo hieu).
  function lockBounds(app, state) {
    var doc = null;
    try { doc = app.ActiveDocument; } catch (eDoc) { return null; }
    if (doc === undefined || doc === null) { return null; }
    if (strProp(doc, "FullName") !== state.document) { return null; }
    var docEnd = numProp(doc.Content, "End");
    if (docEnd === null) { return null; }
    var end = docEnd - state.suffixLength;
    if (end < state.prefixEnd) { end = state.prefixEnd; }
    return { doc: doc, start: state.prefixEnd, end: end };
  }

  // Keo con tro/vung chon ve trong vung khoa truoc khi chay lenh theo con tro (typeText, heading...).
  // Khong keo duoc thi van chay: luoi an toan sau khi chay se hoan tac neu chu roi ra ngoai.
  function pullCursorInto(app, start, end) {
    try {
      var sel = app.Selection;
      var s = numProp(sel, "Start");
      var e = numProp(sel, "End");
      if (s === null || e === null) { return false; }
      if (s >= start && e <= end) { return true; }
      try { sel.SetRange(start, end); return true; } catch (eSet) {}
      try { sel.Start = start; sel.End = end; return true; } catch (eProp) {}
    } catch (e) {}
    return false;
  }

  // Phan NGOAI vung khoa con nguyen khong (so chu truoc + sau voi luc khoa).
  function lockOutsideUnchanged(bounds, state) {
    var docEnd = numProp(bounds.doc.Content, "End");
    if (docEnd === null) { return false; }
    var suffixStart = docEnd - state.suffixLength;
    if (suffixStart < state.prefixEnd) { return false; }
    var pre = docRangeText(bounds.doc, 0, state.prefixEnd);
    var suf = docRangeText(bounds.doc, suffixStart, docEnd);
    if (pre === null || suf === null) { return false; }
    return pre === state.prefix && suf === state.suffix;
  }

  // Chuan bi chay mot lenh trong vung khoa. Tra null (khong khoa / lenh doc / khac tai lieu) hoac
  // {app, doc, start, end}. NEM Error khi lenh bi tu choi truoc - execute() doi thanh {ok:false, error}.
  function lockBegin(app, action) {
    var state = selectionLock;
    if (state === null || action.indexOf("writer.") !== 0 || LOCK_READ_ONLY[action]) { return null; }
    var bounds = lockBounds(app, state);
    if (bounds === null) { return null; }
    var reason = LOCK_REFUSED[action];
    if (reason) { throw new Error(lockMessage(action + " " + reason)); }
    if (state.tables > 0 && LOCK_TABLE_REFUSED[action]) {
      throw new Error(tableLockMessage(LOCK_TABLE_REFUSED[action]));
    }
    pullCursorInto(app, bounds.start, bounds.end);
    return { app: app, doc: bounds.doc, start: bounds.start, end: bounds.end };
  }

  // Sau khi lenh chay xong (va buoc hoan tac da dong): phan ngoai vung doi thi hoan tac ngay va bao loi;
  // lenh nao lam mat mot bang nam tron trong vung thi cung vay (bat ca lenh khong nam trong danh sach chan).
  function lockVerify(lock, action) {
    var state = selectionLock;
    if (state === null) { return; }
    if (!lockOutsideUnchanged(lock, state)) {
      undoOneStep(lock.doc);
      throw new Error(lockMessage(action + " changed text outside the selection, so it was undone"));
    }
    if (state.tables > 0) {
      var now = lockBounds(lock.app, state);
      if (now !== null && countWholeTables(lockTableRanges(now.doc), now.start, now.end) < state.tables) {
        undoOneStep(lock.doc);
        throw new Error(tableLockMessage(action + " changed the table structure, so it was undone"));
      }
    }
  }

  function undoOneStep(doc) {
    try { doc.Undo(1); } catch (e) {}
  }

  function cmdLockSelection(app) {
    selectionLock = null;
    var sel = app.Selection;
    var start = numProp(sel, "Start");
    var end = numProp(sel, "End");
    var text = strProp(sel, "Text");
    // Con tro dung yen (Word/WPS tra 1 ky tu) hoac chi boi khoang trang: khong phai vung chon.
    if (start === null || end === null) {
      return { chars: 0, reason: "this WPS build does not report Selection.Start/End" };
    }
    if (end - start < 2 || text.trim().length < 2) {
      return { chars: 0, reason: "nothing selected" };
    }
    var doc = null;
    try { doc = app.ActiveDocument; } catch (eDoc) { doc = null; }
    if (doc === undefined || doc === null) { return { chars: 0, reason: "no document open" }; }
    var docEnd = numProp(doc.Content, "End");
    if (docEnd === null) {
      return { chars: 0, reason: "this WPS build does not report Content.End" };
    }
    var prefix = docRangeText(doc, 0, start);
    var suffix = docRangeText(doc, end, docEnd);
    // Khong doc duoc phan truoc/sau thi KHONG khoa: thieu chung thi sau nay khong co cach nao biet
    // mot lenh co lam doi phan ngoai vung hay khong, tuc luoi an toan khong ton tai.
    if (prefix === null || suffix === null) {
      return { chars: 0, reason: "this WPS build cannot read a document range (Range(from, to).Text)" };
    }
    var ranges = lockTableRanges(doc);
    selectionLock = {
      document: strProp(doc, "FullName"),
      prefixEnd: start,
      suffixLength: docEnd - end,
      prefix: prefix,
      suffix: suffix,
      tables: countWholeTables(ranges, start, end)
    };
    return {
      chars: end - start,
      start: start,
      end: end,
      docEnd: docEnd,
      tables: selectionLock.tables,
      tablesKnown: ranges !== null
    };
  }

  function cmdUnlockSelection() {
    selectionLock = null;
    return { locked: false };
  }

  // ---- ET (Spreadsheets): undo tu quan ly trong phien add-in (ET khong co UndoRecord).
  var etUndoStack = [];
  var etCondStore = {};

  function etColToNum(letters) {
    var n = 0, i;
    var s = String(letters).toUpperCase();
    for (i = 0; i < s.length; i++) { n = n * 26 + (s.charCodeAt(i) - 64); }
    return n;
  }

  function etNumToCol(n) {
    var s = "", x = n;
    while (x > 0) {
      var r = (x - 1) % 26;
      s = String.fromCharCode(65 + r) + s;
      x = Math.floor((x - 1) / 26);
    }
    return s;
  }

  function etNormAddr(addr) {
    return String(addr).replace(/\$/g, "").trim().toUpperCase();
  }

  function etParseAddr(addr) {
    var s = etNormAddr(addr);
    var m = s.match(/^([A-Z]+)(\d+)(?::([A-Z]+)(\d+))?$/);
    if (!m) { throw new Error("'range' must be a valid address like 'A1:C10', got '" + addr + "'"); }
    var c1 = etColToNum(m[1]), r1 = parseInt(m[2], 10);
    var c2 = m[3] ? etColToNum(m[3]) : c1, r2 = m[4] ? parseInt(m[4], 10) : r1;
    if (r1 < 1 || c1 < 1 || r2 < 1 || c2 < 1) {
      throw new Error("'range' must be a valid address like 'A1:C10', got '" + addr + "'");
    }
    return { r1: Math.min(r1, r2), c1: Math.min(c1, c2), r2: Math.max(r1, r2), c2: Math.max(c1, c2) };
  }

  function etAddrStr(r1, c1, r2, c2) {
    var a = etNumToCol(c1) + r1;
    if (r2 === r1 && c2 === c1) { return a; }
    return a + ":" + etNumToCol(c2) + r2;
  }

  function etTopLeft(addr) {
    var s = etNormAddr(addr);
    var parts = s.split(":");
    return parts[0];
  }

  function etWb(app) {
    var wb = app.ActiveWorkbook;
    if (wb === undefined || wb === null) { throw new Error("no active workbook (open one in WPS Spreadsheets first)"); }
    return wb;
  }

  // Khoa cua mot workbook: duong dan day du la dinh danh on dinh nhat (hai so cung ten "Book1" van khac nhau);
  // so chua luu lan nao thi dung ten. Ngan undo va kho dinh dang dieu kien deu theo khoa nay, khong theo
  // ten sheet tran - ten sheet lap lai o moi workbook.
  function etWbKey(wb) {
    var full = "";
    try { full = strProp(wb, "FullName"); } catch (e) {}
    if (full) { return "f:" + full; }
    var name = "";
    try { name = strProp(wb, "Name"); } catch (e2) {}
    return "n:" + name;
  }

  function etSheetKey(wb, sheetName) {
    return etWbKey(wb) + "\n" + String(sheetName);
  }

  function etFindWbByKey(app, key) {
    var col = app.Workbooks;
    var n = Number(col.Count);
    for (var i = 1; i <= n; i++) {
      var wb = col.Item(i);
      if (etWbKey(wb) === key) { return wb; }
    }
    return null;
  }

  // Doi ten sheet: buoc undo va kho dinh dang dieu kien cua sheet do phai di theo ten moi.
  function etRenameUndoSheet(list, bookKey, oldName, newName) {
    for (var i = 0; i < list.length; i++) {
      var e = list[i];
      if (e.kind === "group") {
        etRenameUndoSheet(e.entries, bookKey, oldName, newName);
      } else if (e.book === bookKey && e.sheet === oldName) {
        e.sheet = newName;
      }
    }
  }

  function etSheetNames(wb) {
    var names = [];
    var col = wb.Worksheets;
    var n = Number(numProp(col, "Count"));
    for (var i = 1; i <= n; i++) {
      names.push(strProp(col.Item(i), "Name"));
    }
    return names;
  }

  function etFindSheet(wb, name) {
    var col = wb.Worksheets;
    var n = Number(numProp(col, "Count"));
    for (var i = 1; i <= n; i++) {
      var ws = col.Item(i);
      if (strProp(ws, "Name") === String(name)) { return ws; }
    }
    return null;
  }

  function etResolveSheet(app, params) {
    var wb = etWb(app);
    var want = paramString(params, "sheet", null);
    if (want) {
      var names = etSheetNames(wb);
      var found = etFindSheet(wb, want);
      if (!found) { throw new Error("no sheet named '" + want + "' (sheets: " + names.join(", ") + ")"); }
      return { wb: wb, ws: found, name: want };
    }
    var active = app.ActiveSheet;
    if (active === undefined || active === null) { throw new Error("no active sheet (open one in WPS Spreadsheets first)"); }
    return { wb: wb, ws: active, name: strProp(active, "Name") };
  }

  function etRangeAddrOf(ws) {
    return etNormAddr(strProp(ws.UsedRange, "Address"));
  }

  function etStrictMatrix(raw) {
    var data = raw;
    if (typeof data === "string") {
      var t = data.replace(/^\s+/, "");
      if (t.charAt(0) === "[" || t.charAt(0) === "{") {
        try { data = JSON.parse(data); } catch (e) {}
      } else {
        throw new Error(matrixHelp("values", describeValue(data)));
      }
    }
    data = decodeArrays(data);
    if (!Array.isArray(data)) { throw new Error(matrixHelp("values", describeValue(data))); }
    if (data.length === 0) { throw new Error(matrixHelp("values", "an empty array")); }
    var i;
    for (i = 0; i < data.length; i++) {
      if (!Array.isArray(data[i])) { throw new Error(matrixHelp("values", "an array")); }
    }
    var rows = [];
    for (var r = 0; r < data.length; r++) {
      var row = [];
      for (var c = 0; c < data[r].length; c++) {
        var cell = data[r][c];
        while (Array.isArray(cell) && cell.length === 1) { cell = cell[0]; }
        if (Array.isArray(cell) || (cell !== null && typeof cell === "object")) {
          throw new Error(matrixHelp("values", "a nested array/object at row " + (r + 1) + ", column " + (c + 1)));
        }
        row.push(cell);
      }
      rows.push(row);
    }
    return rows;
  }

  function etSnapshotCells(ws, r0, c0, rows) {
    var snaps = [];
    var r, c;
    for (r = 0; r < rows.length; r++) {
      for (c = 0; c < rows[r].length; c++) {
        var rr = r0 + r, cc = c0 + c;
        var cell = ws.Cells.Item(rr, cc);
        var v = cell.Value2;
        if (typeof v === "function") { v = v.call(cell); }
        var f = cell.Formula;
        if (typeof f === "function") { f = f.call(cell); }
        if (Array.isArray(v)) { v = null; }
        if (Array.isArray(f)) { f = null; }
        snaps.push({ r: rr, c: cc, v: (v === undefined ? null : v), f: (f === undefined ? null : f) });
      }
    }
    return snaps;
  }

  function etRestoreCells(ws, snaps) {
    var i;
    for (i = 0; i < snaps.length; i++) {
      var s = snaps[i];
      var cell = ws.Cells.Item(s.r, s.c);
      if (s.f !== null && s.f !== undefined && typeof s.f === "string" && s.f.charAt(0) === "=") {
        cell.Formula = s.f;
      } else if (s.v === null || s.v === undefined || s.v === "") {
        cell.Value2 = null;
      } else {
        cell.Value2 = s.v;
      }
    }
  }

  function etPushCells(wb, sheetName, addrStr, snaps) {
    etUndoStack.push({ kind: "cells", book: etWbKey(wb), sheet: sheetName, range: addrStr, snaps: snaps });
  }

  function etActiveSheetName(app) {
    return strProp(app.ActiveSheet, "Name");
  }

  function etDocInfo(wb) {
    var saved = wb.Saved;
    if (typeof saved === "function") { saved = saved.call(wb); }
    return { name: strProp(wb, "Name"), fullName: strProp(wb, "FullName"), saved: !!saved };
  }

  function cmdEtNewWorkbook(app) {
    app.Workbooks.Add();
    var active = app.ActiveWorkbook;
    var info = etDocInfo(active);
    return { name: info.name, fullName: info.fullName, saved: info.saved, opened: true };
  }

  function cmdEtOpen(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    app.Workbooks.Open(path);
    var active = app.ActiveWorkbook;
    var info = etDocInfo(active);
    return { name: info.name, fullName: info.fullName, saved: info.saved, opened: true };
  }

  function cmdEtListSheets(app) {
    var wb = etWb(app);
    var info = etDocInfo(wb);
    return { workbook: info.name, activeSheet: etActiveSheetName(app), sheets: etSheetNames(wb) };
  }

  function etIsEmptySheet(ws) {
    var addr = etRangeAddrOf(ws);
    var box;
    try { box = etParseAddr(addr); } catch (e) { return true; }
    if (box.r1 !== box.r2 || box.c1 !== box.c2) { return false; }
    var cell = ws.Cells.Item(box.r1, box.c1);
    var v = cell.Value2;
    if (typeof v === "function") { v = v.call(cell); }
    var f = cell.Formula;
    if (typeof f === "function") { f = f.call(cell); }
    var t = cell.Text;
    if (typeof t === "function") { t = t.call(cell); }
    if (f !== null && f !== undefined && typeof f === "string" && f !== "") { return false; }
    if (v !== null && v !== undefined && v !== "") { return false; }
    if (t !== null && t !== undefined && String(t) !== "") { return false; }
    return true;
  }

  function cmdEtSheetInfo(app) {
    var wb = etWb(app);
    var active = etActiveSheetName(app);
    var names = etSheetNames(wb);
    var out = [];
    var i;
    for (i = 0; i < names.length; i++) {
      var ws = etFindSheet(wb, names[i]);
      var empty = etIsEmptySheet(ws);
      if (empty) {
        out.push({ name: names[i], usedRange: null, rows: 0, cols: 0, empty: true });
      } else {
        var addr = etRangeAddrOf(ws);
        var box = etParseAddr(addr);
        out.push({ name: names[i], usedRange: etAddrStr(box.r1, box.c1, box.r2, box.c2),
          rows: box.r2 - box.r1 + 1, cols: box.c2 - box.c1 + 1, empty: false });
      }
    }
    return { activeSheet: active, sheets: out };
  }

  function cmdEtAddSheet(app, params) {
    var wb = etWb(app);
    var existing = etSheetNames(wb);
    var name = paramString(params, "name", null);
    if (!name) {
      var idx = 1;
      while (existing.indexOf("Sheet" + idx) >= 0) { idx++; }
      name = "Sheet" + idx;
    }
    if (existing.indexOf(name) >= 0) {
      throw new Error("a sheet named '" + name + "' already exists (sheets: " + existing.join(", ") + ")");
    }
    var col = wb.Worksheets;
    var count = Number(col.Count);
    var ws;
    if (hasParam(params, "index")) {
      var pos = paramInt(params, "index", count);
      if (isNaN(pos)) { pos = count; }
      pos = Math.max(0, Math.min(pos, count));
      if (pos >= count) {
        ws = col.Add();
      } else {
        ws = col.Add(pos + 1);
      }
    } else {
      ws = col.Add();
    }
    ws.Name = name;
    try { ws.Activate(); } catch (e) {}
    etUndoStack.push({ kind: "addSheet", book: etWbKey(wb), sheet: name });
    return { sheet: name, sheets: etSheetNames(wb) };
  }

  function cmdEtRenameSheet(app, params) {
    var old = paramString(params, "sheet", null);
    if (!old) { throw new Error("'sheet' is required"); }
    var name = paramString(params, "name", null);
    if (!name) { throw new Error("'name' is required"); }
    var wb = etWb(app);
    var existing = etSheetNames(wb);
    if (existing.indexOf(old) < 0) {
      throw new Error("no sheet named '" + old + "' (sheets: " + existing.join(", ") + ")");
    }
    if (name !== old && existing.indexOf(name) >= 0) {
      throw new Error("a sheet named '" + name + "' already exists (sheets: " + existing.join(", ") + ")");
    }
    var ws = etFindSheet(wb, old);
    ws.Name = name;
    var bookKey = etWbKey(wb);
    etRenameUndoSheet(etUndoStack, bookKey, old, name);
    var oldKey = bookKey + "\n" + old;
    var newKey = bookKey + "\n" + name;
    if (etCondStore[oldKey]) {
      etCondStore[newKey] = etCondStore[oldKey];
      delete etCondStore[oldKey];
    }
    return { sheet: name, sheets: etSheetNames(wb) };
  }

  function cmdEtActivateSheet(app, params) {
    var want = paramString(params, "sheet", null);
    if (!want) { throw new Error("'sheet' is required"); }
    var wb = etWb(app);
    var names = etSheetNames(wb);
    if (names.indexOf(want) < 0) {
      throw new Error("no sheet named '" + want + "' (sheets: " + names.join(", ") + ")");
    }
    var ws = etFindSheet(wb, want);
    ws.Activate();
    return { active: want };
  }

  function etWrapSingle(v) { return [[(v === undefined ? null : v)]]; }

  function cmdEtReadRange(app, params) {
    var addr = paramString(params, "range", "A1") || "A1";
    var box = etParseAddr(addr);
    var r = etResolveSheet(app, params);
    var rng = r.ws.Range(etAddrStr(box.r1, box.c1, box.r2, box.c2));
    var wantFormulas = paramBool(params, "formulas", false);
    if (wantFormulas) {
      var f = rng.Formula;
      if (typeof f === "function") { f = f.call(rng); }
      var fm;
      if (box.r1 === box.r2 && box.c1 === box.c2) { fm = etWrapSingle(f); }
      else {
        fm = f;
        if (!Array.isArray(fm)) { fm = etWrapSingle(fm); }
      }
      var fo = [];
      var i, j;
      for (i = 0; i < fm.length; i++) {
        var row = [];
        var src = Array.isArray(fm[i]) ? fm[i] : [fm[i]];
        for (j = 0; j < src.length; j++) {
          var cell = src[j];
          row.push((typeof cell === "string" && cell.charAt(0) === "=") ? cell : null);
        }
        fo.push(row);
      }
      var types = [];
      var links = [];
      for (i = 0; i < fo.length; i++) {
        var trow = [];
        for (j = 0; j < fo[i].length; j++) {
          var ff = fo[i][j];
          if (typeof ff === "string" && ff.charAt(0) === "=") { trow.push("formula");
            var mm = /([^A-Za-z0-9_.]+)([^!]+)!/g; var mt;
            while ((mt = mm.exec(ff)) !== null) {
              var nm = mt[2].replace(/^'+|'+$/g, "");
              if (nm && nm !== r.name && links.indexOf(nm) < 0) { links.push(nm); }
            }
          } else { trow.push("text"); }
        }
        types.push(trow);
      }
      var fres = { sheet: r.name, range: etAddrStr(box.r1, box.c1, box.r2, box.c2), formulas: fo, types: types };
      if (links.length > 0) { fres.linkedSheets = links; }
      return fres;
    }
    var v = rng.Value2;
    if (typeof v === "function") { v = v.call(rng); }
    var vals;
    if (box.r1 === box.r2 && box.c1 === box.c2) { vals = etWrapSingle(v); }
    else {
      vals = v;
      if (!Array.isArray(vals)) { vals = etWrapSingle(vals); }
    }
    var out = [];
    var a, b;
    for (a = 0; a < vals.length; a++) {
      var line = [];
      var srow = Array.isArray(vals[a]) ? vals[a] : [vals[a]];
      for (b = 0; b < srow.length; b++) {
        var cv = srow[b];
        line.push((cv === undefined ? null : cv));
      }
      out.push(line);
    }
    return { sheet: r.name, range: etAddrStr(box.r1, box.c1, box.r2, box.c2), values: out };
  }

  function etDoWriteCells(wb, ws, sheetName, topAddr, rows) {
    var top = etParseAddr(topAddr);
    var r0 = top.r1, c0 = top.c1;
    var width = 1, i;
    for (i = 0; i < rows.length; i++) {
      if (rows[i].length > width) { width = rows[i].length; }
    }
    var norm = [];
    for (i = 0; i < rows.length; i++) {
      var line = [];
      var k;
      for (k = 0; k < width; k++) {
        line.push(k < rows[i].length ? rows[i][k] : null);
      }
      norm.push(line);
    }
    var snaps = etSnapshotCells(ws, r0, c0, norm);
    var addrStr = etAddrStr(r0, c0, r0 + norm.length - 1, c0 + width - 1);
    var rng = ws.Range(addrStr);
    rng.Value2 = norm;
    etPushCells(wb, sheetName, addrStr, snaps);
    return { written: norm.length * width, range: addrStr };
  }

  function cmdEtWriteRange(app, params) {
    var addr = paramString(params, "range", null);
    if (!addr) { throw new Error("'range' is required: the top-left cell to write at, e.g. 'A1'"); }
    if (params.values === undefined || params.values === null) {
      throw new Error(matrixHelp("values", "missing"));
    }
    var rows = etStrictMatrix(params.values);
    var top = etTopLeft(addr);
    etParseAddr(top);
    var r = etResolveSheet(app, params);
    var res = etDoWriteCells(r.wb, r.ws, r.name, top, rows);
    return { written: res.written, sheet: r.name };
  }

  function cmdEtWriteRanges(app, params) {
    var writes = params.writes;
    if (writes !== null && typeof writes === "object" && !Array.isArray(writes)) {
      var inner = null, kk;
      var keys = ["items", "item", "value"];
      for (kk = 0; kk < keys.length; kk++) {
        if (writes[keys[kk]] !== undefined) { inner = writes[keys[kk]]; break; }
      }
      writes = (inner !== null) ? inner : [writes];
    }
    if (!Array.isArray(writes) || writes.length === 0) {
      throw new Error("'writes' is required: a list of {range, values, sheet?}, e.g. " +
        "[{\"range\": \"A1\", \"values\": [[\"Tên\"]]}, {\"range\": \"B2\", \"sheet\": \"P&L\", \"values\": [[1, 2]]}]");
    }
    var i;
    var plans = [];
    for (i = 0; i < writes.length; i++) {
      var item = writes[i];
      if (item === null || typeof item !== "object" || Array.isArray(item)) {
        throw new Error("writes[" + i + "] must be an object like {range, values}, got " + describeValue(item));
      }
      var rg = (item.range !== undefined && item.range !== null) ? String(item.range) : "";
      if (!rg) { throw new Error("writes[" + i + "] needs 'range' (the top-left cell to write at)"); }
      if (item.values === undefined || item.values === null) {
        throw new Error("writes[" + i + "] needs 'values' (a JSON 2D array, e.g. [[\"Tên\"]])");
      }
      var rows = etStrictMatrix(item.values);
      var top = etTopLeft(rg);
      etParseAddr(top);
      plans.push({ index: i, range: rg, top: top, rows: rows, sheet: (item.sheet !== undefined && item.sheet !== null) ? String(item.sheet) : null });
    }
    var written = 0;
    var sheets = [];
    var dflt = paramString(params, "sheet", null);
    for (i = 0; i < plans.length; i++) {
      var p = plans[i];
      var px = { sheet: p.sheet || dflt };
      if (!px.sheet) { px = {}; }
      var rr = etResolveSheet(app, px.sheet ? px : {});
      // item co sheet rieng thi uu tien item
      if (p.sheet) { rr = etResolveSheet(app, { sheet: p.sheet }); }
      var out = etDoWriteCells(rr.wb, rr.ws, rr.name, p.top, p.rows);
      written += out.written;
      if (sheets.indexOf(rr.name) < 0) { sheets.push(rr.name); }
    }
    sheets.sort();
    return { ranges: plans.length, written: written, sheets: sheets };
  }

  function etCsvCell(text) {
    var s = String(text);
    var stripped = s.trim();
    if (stripped === "") { return ""; }
    if (/^[+-]?\d{1,15}$/.test(stripped)) {
      var iv = parseInt(stripped, 10);
      if (String(iv) === stripped || String(iv) === stripped.replace(/^\+/, "")) { return iv; }
      return iv;
    }
    if (/^[+-]?(\d+\.\d*|\.\d+|\d+)([eE][+-]?\d+)?$/.test(stripped)) { return parseFloat(stripped); }
    return s;
  }

  function cmdEtImportCsv(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required: the .csv file to read, e.g. 'D:\\data\\raw.csv'"); }
    var delim = paramString(params, "delimiter", ",");
    if (delim === null || delim === undefined || delim === "") { delim = ","; }
    if (delim.length !== 1) { throw new Error("'delimiter' must be a single character, got '" + delim + "'"); }
    var encoding = paramString(params, "encoding", null);
    if (encoding === null || encoding === undefined || String(encoding).trim() === "") { encoding = "utf-8-sig"; }
    // Chuan hoa encoding de nhan cac cach viet khac nhau, WPS chi giai ma UTF-8.
    var normEnc = String(encoding).trim().toLowerCase().replace(/_/g, "-");
    if (normEnc !== "utf-8" && normEnc !== "utf8" && normEnc !== "utf-8-sig" && normEnc !== "utf8-sig") {
      throw new Error("cannot read '" + path + "' as " + encoding + ": WPS only decodes UTF-8 (FileSystem.ReadFile takes no charset argument) - use encoding='utf-8-sig' or 'utf-8'");
    }
    var fs = app.FileSystem;
    var text;
    try {
      text = fs.ReadFile(path);
    } catch (e) {
      throw new Error("no file at '" + path + "'");
    }
    if (text === undefined || text === null) { throw new Error("no file at '" + path + "'"); }
    text = String(text);
    // Chi ban -sig moi cat BOM dau chuoi, ban utf-8 thuong giu nguyen nhu Python.
    if ((normEnc === "utf-8-sig" || normEnc === "utf8-sig") && text.charAt(0) === "\uFEFF") { text = text.substring(1); }
    var lines = text.split("\r\n").join("\n").split("\r").join("\n").split("\n");
    if (lines.length > 0 && lines[lines.length - 1] === "") { lines.pop(); }
    var rows = [];
    var li, parts, q;
    for (li = 0; li < lines.length; li++) {
      if (lines[li] === "" && rows.length === 0 && lines.length === 1) { continue; }
      parts = lines[li].split(delim);
      var conv = [];
      for (q = 0; q < parts.length; q++) { conv.push(etCsvCell(parts[q])); }
      rows.push(conv);
    }
    if (rows.length === 0) { return { path: path, rows: 0, columns: 0, sheet: null }; }
    var width = 1, w;
    for (w = 0; w < rows.length; w++) { if (rows[w].length > width) { width = rows[w].length; } }
    for (w = 0; w < rows.length; w++) {
      while (rows[w].length < width) { rows[w].push(""); }
    }
    var wb = etWb(app);
    var addr = paramString(params, "range", null);
    var sheetName, ws;
    var topAddr;
    if (addr) {
      var rs = etResolveSheet(app, params);
      ws = rs.ws; sheetName = rs.name;
      topAddr = etTopLeft(addr);
      etParseAddr(topAddr);
    } else {
      var base = path.split("/").pop().split("\\").pop();
      var dot = base.lastIndexOf(".");
      if (dot > 0) { base = base.substring(0, dot); }
      base = base.replace(/[[\]*?/\\:]/g, "_").substring(0, 31) || "Import";
      var existing = etSheetNames(wb);
      var nm = base, suffix = 2;
      while (existing.indexOf(nm) >= 0) {
        var tail = " " + suffix;
        nm = base.substring(0, 31 - tail.length) + tail;
        suffix++;
      }
      ws = wb.Worksheets.Add();
      ws.Name = nm;
      sheetName = nm;
      topAddr = "A1";
    }
    var res = etDoWriteCells(wb, ws, sheetName, topAddr, rows);
    void res;
    return { path: path, sheet: sheetName, range: topAddr, rows: rows.length, columns: width, cells: rows.length * width };
  }

  function cmdEtFillRange(app, params) {
    var addr = paramString(params, "range", null);
    if (!addr) { throw new Error("'range' is required: the whole area to fill, e.g. 'B2:H1000'"); }
    var box = etParseAddr(etNormAddr(addr));
    var r = etResolveSheet(app, params);
    var formula = paramString(params, "formula", null);
    var addrStr = etAddrStr(box.r1, box.c1, box.r2, box.c2);
    var rows = box.r2 - box.r1 + 1, cols = box.c2 - box.c1 + 1;
    var snaps = [];
    var rr, cc;
    for (rr = box.r1; rr <= box.r2; rr++) {
      for (cc = box.c1; cc <= box.c2; cc++) {
        var cell0 = r.ws.Cells.Item(rr, cc);
        var v0 = cell0.Value2;
        if (typeof v0 === "function") { v0 = v0.call(cell0); }
        var f0 = cell0.Formula;
        if (typeof f0 === "function") { f0 = f0.call(cell0); }
        snaps.push({ r: rr, c: cc, v: (v0 === undefined ? null : v0), f: (f0 === undefined ? null : f0) });
      }
    }
    if (formula) {
      var rng = r.ws.Range(addrStr);
      rng.Formula = formula;
    }
    etUndoStack.push({ kind: "cells", book: etWbKey(r.wb), sheet: r.name, range: addrStr, snaps: snaps });
    var got = r.ws.Range(addrStr).Formula;
    if (typeof got === "function") { got = got.call(r.ws.Range(addrStr)); }
    var mat = got;
    if (!Array.isArray(mat)) { mat = [[mat]]; }
    var sample = [];
    var sr, sc;
    for (sr = 0; sr < Math.min(rows, 3); sr++) {
      var line = [];
      var srow = Array.isArray(mat[sr]) ? mat[sr] : [mat[sr]];
      for (sc = 0; sc < Math.min(cols, 3); sc++) {
        line.push(sc < srow.length ? (srow[sc] === undefined ? null : srow[sc]) : null);
      }
      sample.push(line);
    }
    return { filled: rows * cols, range: addrStr, sheet: r.name, sample: sample };
  }

  function etHAlign(val) {
    var key = String(val).trim().toLowerCase();
    if (key === "left") { return -4131; }
    if (key === "center" || key === "centre") { return -4108; }
    if (key === "right") { return -4152; }
    if (key === "justify" || key === "justified" || key === "block") { return -4130; }
    throw new Error("'horizontal' must be left/center/right, got '" + val + "'");
  }

  function cmdEtFormatRange(app, params) {
    var addr = paramString(params, "range", null);
    if (!addr) { throw new Error("'range' is required"); }
    var box = etParseAddr(etNormAddr(addr));
    var r = etResolveSheet(app, params);
    var addrStr = etAddrStr(box.r1, box.c1, box.r2, box.c2);
    var rng = r.ws.Range(addrStr);
    if (hasParam(params, "bold")) { rng.Font.Bold = paramBool(params, "bold", false); }
    if (hasParam(params, "italic")) { rng.Font.Italic = paramBool(params, "italic", false); }
    if (hasParam(params, "fontSize")) {
      var sv = params.fontSize;
      if (sv !== null && typeof sv === "object" && !Array.isArray(sv)) { sv = unwrapScalar(sv); }
      if (Array.isArray(sv) && sv.length === 1) { sv = sv[0]; }
      var num = Number(sv);
      if (isNaN(num) || !isFinite(num)) { throw new Error("'fontSize' must be a number, got " + describeValue(params.fontSize)); }
      rng.Font.Size = num;
    }
    if (hasParam(params, "fontColor")) {
      var fc = parseBgrColor(paramString(params, "fontColor", null));
      if (fc !== null) { rng.Font.Color = fc; }
    }
    if (hasParam(params, "fillColor")) {
      var bc = parseBgrColor(paramString(params, "fillColor", null));
      if (bc !== null) { rng.Interior.Color = bc; }
    }
    if (hasParam(params, "numFmt")) { rng.NumberFormat = paramString(params, "numFmt", "General") || "General"; }
    if (hasParam(params, "horizontal")) { rng.HorizontalAlignment = etHAlign(paramString(params, "horizontal", "left")); }
    if (hasParam(params, "wrap")) { rng.WrapText = paramBool(params, "wrap", false); }
    return { sheet: r.name, range: addrStr };
  }

  var ET_CF_OPS = {
    "between": 1, "notbetween": 2, "equal": 3, "notequal": 4,
    "greater": 5, "less": 6, "greaterequal": 7, "lessequal": 8
  };
  var ET_CF_BACK = { 1: "between", 2: "notBetween", 3: "equal", 4: "notEqual", 5: "greater", 6: "less", 7: "greaterEqual", 8: "lessEqual" };

  function etCfKey(op) {
    return String(op).toLowerCase().replace(/_/g, "").replace(/ /g, "");
  }

  function etCfPlan(rules) {
    var plan = [], i;
    for (i = 0; i < rules.length; i++) {
      var rule = rules[i];
      if (rule === null || typeof rule !== "object" || Array.isArray(rule)) {
        throw new Error("rules[" + i + "] must be an object, got " + describeValue(rule));
      }
      var key = hasParam(rule, "operator") ? etCfKey(paramString(rule, "operator", "less")) : "less";
      if (key === "formula") {
        var ff = paramString(rule, "formula1", null);
        if (!ff) { throw new Error("rules[" + i + "] needs 'formula1' - the value to compare with, e.g. {\"operator\": \"less\", \"formula1\": \"50\"}"); }
        if (hasParam(rule, "styleName")) { throw new Error("rules[" + i + "] 'styleName' is not supported on WPS (ET has no named cell styles; use bold/italic/fontColor/fillColor)"); }
        plan.push({ input: "formula", opNum: null, type: 2, formula1: ff, formula2: null, rule: rule });
        continue;
      }
      if (!ET_CF_OPS[key]) {
        throw new Error("rules[" + i + "].operator must be one of between/equal/formula/greater/greaterEqual/less/lessEqual/notBetween/notEqual, got '" + paramString(rule, "operator", "") + "'");
      }
      var f1 = paramString(rule, "formula1", null);
      if (!f1) {
        throw new Error("rules[" + i + "] needs 'formula1' - the value to compare with, e.g. {\"operator\": \"less\", \"formula1\": \"50\"}");
      }
      var f2 = paramString(rule, "formula2", null);
      if ((key === "between" || key === "notbetween") && !f2) {
        throw new Error("rules[" + i + "]: operator '" + key + "' needs 'formula2' as well");
      }
      if (hasParam(rule, "styleName")) {
        throw new Error("rules[" + i + "] 'styleName' is not supported on WPS (ET has no named cell styles; use bold/italic/fontColor/fillColor)");
      }
      plan.push({ input: key, opNum: ET_CF_OPS[key], type: 1, formula1: f1, formula2: f2, rule: rule });
    }
    return plan;
  }

  function cmdEtSetCondFormat(app, params) {
    var addr = paramString(params, "range", null);
    if (!addr) { throw new Error("'range' is required: the area the rules apply to, e.g. 'B2:B100'"); }
    var raw = params.rules;
    if (raw !== null && typeof raw === "object" && !Array.isArray(raw)) {
      var k, inner = null;
      var ks = ["items", "item", "value"];
      for (k = 0; k < ks.length; k++) {
        if (raw[ks[k]] !== undefined) { inner = raw[ks[k]]; break; }
      }
      raw = (inner !== null) ? inner : [raw];
    }
    if (!Array.isArray(raw) || raw.length === 0) {
      throw new Error("'rules' is required: a list of rules, e.g. [{\"operator\": \"less\", \"formula1\": \"50\", \"styleName\": \"Bad\"}]");
    }
    var plan = etCfPlan(raw);
    var box = etParseAddr(etNormAddr(addr));
    var r = etResolveSheet(app, params);
    var addrStr = etAddrStr(box.r1, box.c1, box.r2, box.c2);
    var rng = r.ws.Range(addrStr);
    try { rng.FormatConditions.Delete(); } catch (e) {}
    var written = [], i;
    for (i = 0; i < plan.length; i++) {
      var p = plan[i];
      var fc;
      if (p.type === 2) {
        fc = rng.FormatConditions.Add(2, null, p.formula1);
      } else if (p.formula2) {
        fc = rng.FormatConditions.Add(1, p.opNum, p.formula1, p.formula2);
      } else {
        fc = rng.FormatConditions.Add(1, p.opNum, p.formula1);
      }
      var rule = p.rule;
      if (hasParam(rule, "fillColor")) {
        var col = parseBgrColor(paramString(rule, "fillColor", null));
        if (col !== null) { fc.Interior.Color = col; }
      }
      if (hasParam(rule, "fontColor")) {
        var fcol = parseBgrColor(paramString(rule, "fontColor", null));
        if (fcol !== null) { fc.Font.Color = fcol; }
      }
      if (hasParam(rule, "bold")) { fc.Font.Bold = paramBool(rule, "bold", false); }
      if (hasParam(rule, "italic")) { fc.Font.Italic = paramBool(rule, "italic", false); }
      written.push({ operator: p.input, formula1: p.formula1, formula2: p.formula2 || null });
    }
    var cfKey = etSheetKey(r.wb, r.name);
    if (!etCondStore[cfKey]) { etCondStore[cfKey] = []; }
    var bucket = etCondStore[cfKey];
    var bi;
    for (bi = bucket.length - 1; bi >= 0; bi--) {
      if (bucket[bi].range === addrStr) { bucket.splice(bi, 1); }
    }
    bucket.push({ range: addrStr, rules: written });
    return { sheet: r.name, range: addrStr, rules: written };
  }

  function cmdEtListCondFormats(app, params) {
    var r = etResolveSheet(app, params);
    var bucket = etCondStore[etSheetKey(r.wb, r.name)] || [];
    var addr = paramString(params, "range", null);
    if (addr) {
      var box = etParseAddr(etNormAddr(addr));
      var want = etAddrStr(box.r1, box.c1, box.r2, box.c2);
      var found = [];
      var i;
      for (i = 0; i < bucket.length; i++) {
        if (bucket[i].range === want) { found.push(bucket[i]); }
      }
      if (found.length > 0) { return { sheet: r.name, formats: found }; }
      // Doc that tu API khi store khong co (sheet mo tu file ngoai)
      var rng = r.ws.Range(want);
      var fcCol = rng.FormatConditions;
      var n = Number(fcCol.Count);
      if (n === 0) { return { sheet: r.name, formats: [] }; }
      var rules = [];
      for (i = 1; i <= n; i++) {
        var fc = fcCol.Item(i);
        var op = numProp(fc, "Operator");
        var f1 = fc.Formula1;
        if (typeof f1 === "function") { f1 = f1.call(fc); }
        var f2 = fc.Formula2;
        if (typeof f2 === "function") { f2 = f2.call(fc); }
        var key = ET_CF_BACK[Number(op)] || "less";
        var isBetween = (key === "between" || key === "notBetween");
        rules.push({ operator: key, formula1: (f1 === undefined || f1 === null) ? null : String(f1), formula2: isBetween ? String(f2) : null });
      }
      return { sheet: r.name, formats: [{ range: want, rules: rules }] };
    }
    if (bucket.length > 0) { return { sheet: r.name, formats: bucket.slice() }; }
    // Khong co range: quet vung dung that, gop cac dinh dang thay tren tung o
    var seen = {}, out = [];
    try {
      var used = etRangeAddrOf(r.ws);
      var ub = etParseAddr(used);
      var empty = etIsEmptySheet(r.ws);
      if (!empty) {
        var rr, cc;
        for (rr = ub.r1; rr <= ub.r2; rr++) {
          for (cc = ub.c1; cc <= ub.c2; cc++) {
            var one = r.ws.Range(etAddrStr(rr, cc, rr, cc));
            var col2 = one.FormatConditions;
            var cnt = Number(col2.Count);
            var k;
            for (k = 1; k <= cnt; k++) {
              var f = col2.Item(k);
              var t = numProp(f, "Type");
              var o = numProp(f, "Operator");
              var a1 = f.Formula1;
              if (typeof a1 === "function") { a1 = a1.call(f); }
              var a2 = f.Formula2;
              if (typeof a2 === "function") { a2 = a2.call(f); }
              var lab = (Number(t) === 2) ? "formula" : (ET_CF_BACK[Number(o)] || "less");
              var isB = (lab === "between" || lab === "notBetween");
              var sig = lab + "|" + String(a1) + "|" + (isB ? String(a2) : "");
              if (!seen[sig]) {
                seen[sig] = { range: etAddrStr(rr, cc, rr, cc), rules: [{ operator: lab, formula1: String(a1), formula2: isB ? String(a2) : null }] };
                out.push(seen[sig]);
              }
            }
          }
        }
      }
    } catch (e2) {}
    if (out.length > 0) { return { sheet: r.name, formats: out }; }
    // Cuoi cung: doc ca sheet qua UsedRange (truong hop store trong nhung API co)
    return { sheet: r.name, formats: [] };
  }

  var ET_CHART_TYPES = { "column": 51, "bar": 57, "line": 4, "pie": 5, "area": 1, "scatter": -4169 };
  var ET_CHART_BACK = { 51: "column", 57: "bar", 4: "line", 5: "pie", 1: "area", "-4169": "scatter" };

  function cmdEtAddChart(app, params) {
    var addr = paramString(params, "range", null);
    if (!addr) { throw new Error("'range' is required"); }
    var box = etParseAddr(etNormAddr(addr));
    var kind = (paramString(params, "type", "column") || "column").toLowerCase();
    if (!ET_CHART_TYPES[kind]) {
      throw new Error("'type' must be one of area/bar/column/line/pie/scatter, got '" + paramString(params, "type", "") + "'");
    }
    var r = etResolveSheet(app, params);
    var addrStr = etAddrStr(box.r1, box.c1, box.r2, box.c2);
    var anchor = paramString(params, "anchor", "A1") || "A1";
    var ab = etParseAddr(etNormAddr(anchor));
    var anchorStr = etAddrStr(ab.r1, ab.c1, ab.r1, ab.c1);
    var w = params.width !== undefined && params.width !== null ? Number(unwrapScalar(params.width)) : 12;
    var h = params.height !== undefined && params.height !== null ? Number(unwrapScalar(params.height)) : 7;
    if (isNaN(w)) { w = 12; }
    if (isNaN(h)) { h = 7; }
    var ptsW = w * 28.3465, ptsH = h * 28.3465;
    var cell = r.ws.Range(anchorStr);
    var left = numProp(cell, "Left"), top = numProp(cell, "Top");
    var objs = r.ws.ChartObjects();
    var obj = objs.Add(Number(left), Number(top), ptsW, ptsH);
    var chart = obj.Chart;
    try { chart.SetSourceData(r.ws.Range(addrStr)); } catch (e) {}
    chart.ChartType = ET_CHART_TYPES[kind];
    var title = paramString(params, "title", null);
    var titleApplied = false;
    if (title) {
      try {
        chart.HasTitle = true;
        chart.ChartTitle.Text = title;
        titleApplied = true;
      } catch (e2) { titleApplied = false; }
    }
    var nm = strProp(obj, "Name");
    var actual = ET_CHART_BACK[Number(numProp(chart, "ChartType"))] || kind;
    if (typeof actual === "string") {
      var chk = numProp(obj.Chart, "ChartType");
      actual = ET_CHART_BACK[Number(chk)] || kind;
    }
    return { chart: String(nm), sheet: r.name, type: kind, diagram: actual, typeApplied: (actual === kind), titleApplied: titleApplied, range: addrStr, anchor: anchorStr };
  }

  function cmdEtListCharts(app, params) {
    var r = etResolveSheet(app, params);
    var objs = r.ws.ChartObjects();
    var n = Number(numProp(objs, "Count"));
    var charts = [], i;
    for (i = 1; i <= n; i++) {
      var o = objs.Item(i);
      charts.push({ name: strProp(o, "Name"), diagram: ET_CHART_BACK[Number(numProp(o.Chart, "ChartType"))] || "" });
    }
    var shapes = [];
    try {
      var sh = r.ws.Shapes;
      var cnt = Number(numProp(sh, "Count"));
      for (i = 1; i <= cnt; i++) {
        shapes.push(strProp(sh.Item(i), "Name"));
      }
    } catch (e2) {}
    return { sheet: r.name, charts: charts, shapes: shapes };
  }

  // Hop dong voi pane: moi lenh et.* co `undo` trong registry = DUNG MOT muc tren etUndoStack (pane dem so lenh
  // nhu vay roi gui et.undo {count}). Lenh day nhieu muc (writeRanges) gop thanh mot; lenh khong day muc nao
  // (dinh dang, bieu do... WPS chua hoan tac duoc) giu cho bang muc "none" - neu khong, count lech va
  // "Hoan tac luot nay" go mat buoc cua luot truoc. Lenh loi: bo cac muc vua day (pane khong dem lenh loi).
  function etUndoMark(kind, entry) {
    return (kind === "et" && entry !== null && entry.undo) ? etUndoStack.length : -1;
  }

  function etUndoSeal(mark, action, ok) {
    if (mark < 0) { return; }
    var added = etUndoStack.splice(mark, etUndoStack.length - mark);
    if (!ok) { return; }
    if (added.length === 0) {
      etUndoStack.push({ kind: "none", action: action });
    } else if (added.length === 1) {
      etUndoStack.push(added[0]);
    } else {
      etUndoStack.push({ kind: "group", entries: added });
    }
  }

  // Go mot muc; tra false khi khong khoi phuc duoc gi.
  function etUndoEntry(app, entry, restored, skipped) {
    var k;
    if (entry.kind === "none") {
      skipped.push(entry.action);
      return false;
    }
    if (entry.kind === "group") {
      var any = false;
      for (k = entry.entries.length - 1; k >= 0; k--) {
        if (etUndoEntry(app, entry.entries[k], restored, skipped)) { any = true; }
      }
      return any;
    }
    if (entry.kind === "addSheet") {
      try {
        var wbAdd = etFindWbByKey(app, entry.book);
        var ws = wbAdd ? etFindSheet(wbAdd, entry.sheet) : null;
        if (ws) { ws.Delete(); }
      } catch (e) {}
      // Xoa ca ghi nhan dinh dang co dieu kien cua sheet do
      try { delete etCondStore[entry.book + "\n" + entry.sheet]; } catch (e2) {}
      restored.push({ sheet: entry.sheet, deleted: true });
      return true;
    }
    try {
      var wb2 = etFindWbByKey(app, entry.book);
      // Workbook da dong (hoac doi duong dan): bo qua, KHONG khoi phuc vao tai lieu dang mo.
      if (!wb2) { return false; }
      var ws2 = etFindSheet(wb2, entry.sheet);
      // Sheet da bi xoa/doi ten ngoai add-in: bo qua, KHONG lui ve sheet dang active.
      if (!ws2) { return false; }
      etRestoreCells(ws2, entry.snaps);
      restored.push({ sheet: entry.sheet, range: entry.range, cells: entry.snaps.length });
      return true;
    } catch (e3) {
      return false;
    }
  }

  function cmdEtUndo(app, params) {
    var count = paramInt(params, "count", 1);
    if (isNaN(count)) { count = 1; }
    count = Math.max(1, count);
    var restored = [], skipped = [], undone = 0, k;
    for (k = 0; k < count; k++) {
      if (etUndoStack.length === 0) { break; }
      if (etUndoEntry(app, etUndoStack.pop(), restored, skipped)) { undone++; }
    }
    var out = { undone: undone, restored: restored };
    if (skipped.length > 0) { out.skipped = skipped; }
    return out;
  }

  // Thu muc cuu ban chua luu: <tmpdir>/rescued. WPS Mkdir chi tao duoc MOT cap, ma tmpdir da ton tai
  // nen tao thang "/tmp/rescued" la du. Loi tao thu muc khong chan viec dong tai lieu.
  function rescueDir(app) {
    var tmp = "/tmp";
    var fs = null;
    try {
      fs = app.FileSystem;
      tmp = strProp(fs, "tmpdir") || "/tmp";
    } catch (e) {}
    var dir = tmp.replace(/[/\\]+$/, "") + "/rescued";
    try {
      if (fs && !fsExists(fs, dir) && typeof fs.Mkdir === "function") { fs.Mkdir(dir); }
    } catch (e2) {}
    return dir;
  }

  function rescueName(obj, fallback) {
    var nm = "";
    try { nm = strProp(obj, "Name"); } catch (e) {}
    return nm || fallback;
  }

  // Dong ca bo tai lieu dang mo: cuu ban CHUA LUU vao <tmpdir>/rescued truoc, roi dong tung cai.
  // closeDoc khac nhau giua ET (Close(false)) va WPP (Close() khong tham so) nen do ben goi truyen vao.
  function closeAllDocuments(app, collection, fallbackName, closeDoc) {
    var n = Number(collection.Count);
    var items = [], i;
    for (i = 1; i <= n; i++) { items.push(collection.Item(i)); }
    var dir = rescueDir(app);
    var rescued = [], closed = 0;
    for (i = 0; i < items.length; i++) {
      var doc = items[i];
      var saved = doc.Saved;
      if (typeof saved === "function") { saved = saved.call(doc); }
      if (!saved) {
        var rp = dir + "/" + rescueName(doc, fallbackName);
        try { doc.SaveAs(rp); rescued.push(rp); } catch (e2) {}
      }
      try { closeDoc(doc); closed++; } catch (e3) {}
    }
    return { closed: closed, rescued: rescued };
  }

  function cmdEtExportPdf(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    var wb = etWb(app);
    try { wb.ExportAsFixedFormat(path, 17); } catch (e0) {}
    try { wb.ExportAsFixedFormat(0, path); } catch (e1) {}
    var fs = app.FileSystem;
    if (fsExists(fs, path)) { return { exported: path }; }
    var text = "";
    try {
      var names = etSheetNames(wb);
      var lines = [], si, rr, cc;
      for (si = 0; si < names.length; si++) {
        var ws = etFindSheet(wb, names[si]);
        if (etIsEmptySheet(ws)) { continue; }
        var addr = etRangeAddrOf(ws);
        var box = etParseAddr(addr);
        for (rr = box.r1; rr <= box.r2; rr++) {
          var rowVals = [];
          for (cc = box.c1; cc <= box.c2; cc++) {
            rowVals.push(strProp(ws.Cells.Item(rr, cc), "Text"));
          }
          lines.push(rowVals.join("\t"));
        }
        lines.push("");
      }
      text = lines.join("\n");
    } catch (e2) {}
    return writePdf(fs, path, text, "Spreadsheets");
  }

  function cmdEtSave(app) {
    var wb = etWb(app);
    var p = strProp(wb, "Path");
    if (!p) { throw new Error("the document has never been saved - use saveAs with a 'path'"); }
    wb.Save();
    var info = etDocInfo(wb);
    return { saved: true, fullName: info.fullName };
  }

  function cmdEtSaveAs(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    var wb = etWb(app);
    wb.SaveAs(path);
    var info = etDocInfo(wb);
    return { saved: true, fullName: info.fullName };
  }

  function cmdEtCloseAll(app) {
    return closeAllDocuments(app, app.Workbooks, "workbook.xlsx", function (wb) { wb.Close(false); });
  }

  var ET_ERR_TEXT = ["#DIV/0!", "#REF!", "#NAME?", "#NUM!", "#NULL!", "#VALUE!", "#N/A", "#ERR:", "Err:"];

  function etStartsError(text) {
    var i;
    for (i = 0; i < ET_ERR_TEXT.length; i++) {
      if (text.indexOf(ET_ERR_TEXT[i]) === 0) { return true; }
    }
    return false;
  }

  function etParseNum(text) {
    var c = String(text).replace(/\xa0/g, "").split(" ").join("").split(",").join("");
    if (c === "") { return null; }
    var n = Number(c);
    if (isNaN(n) || !isFinite(n)) { return null; }
    return n;
  }

  function etReadCell(ws, r, c) {
    var cell = ws.Cells.Item(r, c);
    var v = cell.Value2;
    if (typeof v === "function") { v = v.call(cell); }
    var f = cell.Formula;
    if (typeof f === "function") { f = f.call(cell); }
    if (Array.isArray(v)) { v = null; }
    if (Array.isArray(f)) { f = null; }
    return { v: (v === undefined ? null : v), f: (f === undefined ? null : f),
      t: strProp(cell, "Text"), nf: strProp(cell, "NumberFormat") || "General" };
  }

  function cmdEtCheckRange(app, params) {
    var r = etResolveSheet(app, params);
    var ws = r.ws;
    var usedAddr = etRangeAddrOf(ws);
    var usedBox = etParseAddr(usedAddr);
    var reqRaw = paramString(params, "range", null);
    var box = reqRaw ? etParseAddr(etNormAddr(reqRaw)) : usedBox;
    var rows = box.r2 - box.r1 + 1, cols = box.c2 - box.c1 + 1;
    var rangeStr = etAddrStr(box.r1, box.c1, box.r2, box.c2);
    var usedStr = etAddrStr(usedBox.r1, usedBox.c1, usedBox.r2, usedBox.c2);
    // Doc toan bo vung vao cache
    var cache = {}, rr, cc;
    for (rr = box.r1; rr <= box.r2; rr++) {
      for (cc = box.c1; cc <= box.c2; cc++) {
        cache[rr + "," + cc] = etReadCell(ws, rr, cc);
      }
    }
    function at(rr2, cc2) { return cache[rr2 + "," + cc2]; }
    // Nhan dong tieu de
    var title = null, headerRow = box.r1;
    if (rows >= 2 && cols >= 2) {
      var filled = [];
      for (cc = box.c1; cc <= box.c2; cc++) {
        var h0 = at(box.r1, cc);
        var tx0 = h0.t.replace(/^\s+|\s+$/g, "");
        if (tx0 !== "") { filled.push({ c: cc, text: tx0, v: h0.v, f: h0.f }); }
      }
      if (filled.length === 1 && typeof filled[0].v === "string" && !filled[0].f) {
        var secondTexts = 0;
        for (cc = box.c1; cc <= box.c2; cc++) {
          var h1 = at(box.r1 + 1, cc);
          var tx1 = h1.t.replace(/^\s+|\s+$/g, "");
          if (tx1 === "") { continue; }
          if (typeof h1.v === "string" && !h1.f) { secondTexts++; }
          else if (h1.f && typeof h1.v === "string") { secondTexts++; }
          else if (h1.v !== null && h1.v !== undefined && h1.v !== "" && typeof h1.v !== "number") { secondTexts++; }
        }
        if (secondTexts * 2 >= cols) {
          title = filled[0].text;
          headerRow = box.r1 + 1;
        }
      }
    }
    var dataStart = headerRow + 1;
    var columns = [], issues = [];
    for (cc = box.c1; cc <= box.c2; cc++) {
      var letter = etNumToCol(cc);
      var hc = at(headerRow, cc);
      var headerText = hc.t.replace(/^\s+|\s+$/g, "");
      var numbers = 0, texts = 0, blanks = 0, numerics = 0, errors = 0, formulas = 0, fractional = 0;
      var errNames = [];
      for (rr = dataStart; rr <= box.r2; rr++) {
        var info = at(rr, cc);
        var isF = (typeof info.f === "string" && info.f.charAt(0) === "=");
        if (isF) { formulas++; }
        var txt = info.t.replace(/^\s+|\s+$/g, "");
        if (!isF && (info.v === null || info.v === undefined || info.v === "") && txt === "") {
          blanks++;
        } else if (isF && (info.v === null || info.v === undefined)) {
          errors++;
          errNames.push(info.t || "error");
        } else if ((info.v === null || info.v === undefined) && etStartsError(info.t)) {
          errors++;
          errNames.push(info.t);
        } else if (typeof info.v === "string") {
          texts++;
          if (etParseNum(txt) !== null) { numerics++; }
        } else if (typeof info.v === "number") {
          numbers++;
          if (Math.abs(info.v - Math.round(info.v)) > 1e-9) { fractional++; }
        } else if (typeof info.v === "boolean") {
          texts++;
        } else if (info.v === null || info.v === undefined) {
          if (txt === "") { blanks++; }
          else {
            texts++;
            if (etParseNum(txt) !== null) { numerics++; }
          }
        } else {
          var pn = etParseNum(txt);
          if (pn === null) { texts++; }
          else {
            numbers++;
            if (Math.abs(pn - Math.round(pn)) > 1e-9) { fractional++; }
          }
        }
      }
      var fmtRow = (dataStart <= box.r2) ? dataStart : headerRow;
      var fmt = at(fmtRow, cc).nf || "General";
      var dataCells = numbers + texts + errors;
      if (!headerText && dataCells > 0) {
        issues.push({ type: "empty-header", detail: "column " + letter + " has data but no header in the first row" });
      }
      if (numerics > 0 && numbers + numerics >= texts) {
        issues.push({ type: "numbers-as-text", detail: "column " + letter + " has " + numerics + " number(s) stored as text: write them as JSON numbers" });
      } else if (numbers > 0 && texts > 0) {
        issues.push({ type: "mixed-types", detail: "column " + letter + " mixes " + numbers + " number(s) and " + texts + " text value(s)" });
      }
      if (errors > 0) {
        var uniq = {}, ul = [], ei;
        for (ei = 0; ei < errNames.length; ei++) {
          if (!uniq[errNames[ei]]) { uniq[errNames[ei]] = true; ul.push(errNames[ei]); }
        }
        ul.sort();
        issues.push({ type: "error-values", detail: "column " + letter + " has " + errors + " error value(s): " + ul.join(", ") });
      }
      if (fractional > 0 && (!fmt || String(fmt).toLowerCase() === "general")) {
        issues.push({ type: "no-number-format", detail: "column " + letter + " has decimals shown with the General format: apply numFmt such as \"0.0\" or \"#,##0.00\"" });
      }
      columns.push({ column: letter, header: headerText || null, numbers: numbers, texts: texts,
        blanks: blanks, formulas: formulas, numberFormat: fmt });
    }
    // Cong thuc tro vao o trong
    var emptyRefs = [], scanned = 0;
    for (rr = box.r1; rr <= box.r2 && emptyRefs.length < 40 && scanned < 4000; rr++) {
      for (cc = box.c1; cc <= box.c2 && emptyRefs.length < 40 && scanned < 4000; cc++) {
        var fi = at(rr, cc);
        if (!(typeof fi.f === "string" && fi.f.charAt(0) === "=")) { continue; }
        scanned++;
        var cleaned = String(fi.f).replace(/"(?:[^"]|"")*"/g, function (m) {
          var s = "";
          for (var z = 0; z < m.length; z++) { s += " "; }
          return s;
        });
        var re = /(\$?)([A-Za-z]{1,3})(\$?)(\d+)/g;
        var m2, hit = null;
        while ((m2 = re.exec(cleaned)) !== null) {
          var si = m2.index, ei2 = si + m2[0].length;
          if (si > 0 && (cleaned.charAt(si - 1) === ":")) { continue; }
          if (ei2 < cleaned.length && (cleaned.charAt(ei2) === ":")) { continue; }
          if (si > 0 && /[A-Za-z0-9_]/.test(cleaned.charAt(si - 1))) { continue; }
          if (ei2 < cleaned.length && /[A-Za-z0-9_(]/.test(cleaned.charAt(ei2))) { continue; }
          var tc = etColToNum(m2[2]), trr = parseInt(m2[4], 10);
          if (!(tc >= 1 && trr >= 1)) { continue; }
          var tgt;
          var key2 = trr + "," + tc;
          if (cache[key2] !== undefined) { tgt = cache[key2]; }
          else {
            try { tgt = etReadCell(ws, trr, tc); } catch (e4) { continue; }
          }
          var tempty = (!tgt.f && (tgt.v === null || tgt.v === undefined || tgt.v === "") && tgt.t === "");
          if (tempty) {
            hit = r.name + "!" + etNumToCol(cc) + rr + " -> " + r.name + "!" + etNumToCol(tc) + trr + " (trong)";
            break;
          }
        }
        if (hit) { emptyRefs.push(hit); }
      }
    }
    if (emptyRefs.length > 0) {
      var show = emptyRefs.slice(0, 6).join("; ");
      issues.push({ type: "empty-reference", detail: emptyRefs.length + " cong thuc tro vao mot o dang TRONG (vi du: " + show + "). O trong duoc tinh la 0, nen mot o dieu khien bi tro sai cho se im lang chay bang 0 - hay kiem tra xem co phai cong thuc dang tro vao o tieu de / o ben canh thay vi o gia tri khong", examples: emptyRefs.slice(0, 6) });
    }
    return { sheet: r.name, range: rangeStr, usedRange: usedStr, table: "", rows: rows, cols: cols,
      title: title, columns: columns, issueCount: issues.length, issues: issues };
  }

  // ---- WPP (Presentation): khong co UndoRecord, khong lenh nao mo UndoRecord.
  // ---- Nhay toi cho duoc trich dan (pane goi khi nguoi dung bam [§12] / [Data!C42] / [slide 3]) - chi chon + cuon.
  function cmdWriterGoto(app, params) {
    var doc = app.ActiveDocument;
    var paras = doc.Paragraphs;
    var count = Number(numProp(paras, "Count"));
    var index = paramInt(params, "paragraph", 0);
    if (index < 1 || index > count) { throw new Error("'paragraph' must be between 1 and " + count); }
    var hint = String(paramString(params, "text", "") || "").replace(/^\s+|\s+$/g, "").substring(0, 30).toLowerCase();
    var startsWith = function (i) {
      var t = "";
      try { t = String(paras.Item(i).Range.Text || "").replace(/^\s+/, "").toLowerCase(); } catch (e) {}
      return t.indexOf(hint) === 0;
    };
    // Tai lieu da doi sau cau tra loi: so doan lech - co chu dau doan thi do quanh do (+-40 doan).
    if (hint && !startsWith(index)) {
      for (var offset = 1; offset <= 40; offset++) {
        if (index - offset >= 1 && startsWith(index - offset)) { index -= offset; break; }
        if (index + offset <= count && startsWith(index + offset)) { index += offset; break; }
      }
    }
    var range = paras.Item(index).Range;
    range.Select();
    try { app.ActiveWindow.ScrollIntoView(range, true); } catch (eScroll) {}
    var text = "";
    try { text = String(range.Text || "").replace(/\r/g, "").replace(/^\s+|\s+$/g, ""); } catch (eText) {}
    return { paragraph: index, text: text.length > 120 ? text.substring(0, 120) + "..." : text };
  }

  function cmdEtGoto(app, params) {
    var address = String(paramString(params, "range", "") || "").replace(/^\s+|\s+$/g, "");
    if (!address) { throw new Error("'range' is required, e.g. 'C42' or 'Data!C42'"); }
    var sheetParams = { sheet: paramString(params, "sheet", null) };
    var bang = address.lastIndexOf("!");
    if (bang > 0) {
      sheetParams.sheet = address.substring(0, bang).replace(/^'|'$/g, "").replace(/''/g, "'");
      address = address.substring(bang + 1);
    }
    var box = etParseAddr(address);
    var r = etResolveSheet(app, sheetParams);
    try { r.ws.Activate(); } catch (eAct) {}
    var rng = r.ws.Range(etAddrStr(box.r1, box.c1, box.r2, box.c2));
    rng.Select();
    return { sheet: r.name, range: etAddrStr(box.r1, box.c1, box.r2, box.c2) };
  }

  function cmdWppGoto(app, params) {
    var pres = wppPres(app);
    var count = Number(numProp(pres.Slides, "Count"));
    var slide = paramInt(params, "slide", 0);
    if (slide < 1 || slide > count) { throw new Error("'slide' must be between 1 and " + count); }
    try {
      app.ActiveWindow.View.GotoSlide(slide);
    } catch (eGo) {
      pres.Slides.Item(slide).Select();
    }
    return { slide: slide };
  }

  function wppPres(app) {
    var p = app.ActivePresentation;
    if (p === undefined || p === null) {
      throw new Error("no active presentation (open one in WPS Presentation first)");
    }
    return p;
  }

  function wppDocInfo(pres) {
    var saved = pres.Saved;
    if (typeof saved === "function") { saved = saved.call(pres); }
    return { name: strProp(pres, "Name"), fullName: strProp(pres, "FullName"), saved: !!saved };
  }

  function wppNumber(params, name, dflt) {
    if (!hasParam(params, name)) { return dflt; }
    var v = unwrapScalar(params[name]);
    var n = (typeof v === "number") ? v : Number(typeof v === "string" ? v.trim() : v);
    if (typeof n !== "number" || isNaN(n) || !isFinite(n)) {
      throw new Error("'" + name + "' must be a number, got " + describeValue(params[name]));
    }
    return n;
  }

  // Slide can thao tac: bo trong hoac ngoai khoang = slide CUOI (giong _page LibreOffice).
  function wppSlide(app, params) {
    var pres = wppPres(app);
    var count = Number(numProp(pres.Slides, "Count"));
    if (!(count > 0)) {
      throw new Error("presentation has no slides; call wpp.addSlide first");
    }
    var index = count;
    if (hasParam(params, "slide")) { index = paramInt(params, "slide", count); }
    if (!(index >= 1 && index <= count)) { index = count; }
    return { pres: pres, slide: pres.Slides.Item(index), index: index, count: count };
  }

  function wppShapeText(shape) {
    try {
      return String(shape.TextFrame.TextRange.Text || "");
    } catch (e) { return ""; }
  }

  function cmdWppNewPresentation(app) {
    app.Presentations.Add();
    var info = wppDocInfo(app.ActivePresentation);
    return { name: info.name, fullName: info.fullName, saved: info.saved, opened: true };
  }

  function cmdWppOpen(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    app.Presentations.Open(path);
    var info = wppDocInfo(app.ActivePresentation);
    return { name: info.name, fullName: info.fullName, saved: info.saved, opened: true };
  }

  function cmdWppListSlides(app, params) {
    var pres = wppPres(app);
    var count = Number(numProp(pres.Slides, "Count"));
    var slides = [], i, j;
    for (i = 1; i <= count; i++) {
      var slide = pres.Slides.Item(i);
      var layout = numProp(slide, "Layout");
      var texts = [];
      var n = Number(numProp(slide.Shapes, "Count"));
      for (j = 1; j <= n; j++) {
        var t = wppShapeText(slide.Shapes.Item(j));
        if (t) { texts.push(t); }
      }
      var entry = { index: i, layout: Number(layout), shapeTexts: texts };
      if (paramBool(params, "detail", false)) {
        try { entry.master = strProp(slide.Master, "Name") || ""; } catch (eM) {}
        try {
          var nn = []; var ns = slide.NotesPage.Shapes; var nc = Number(numProp(ns, "Count"));
          for (var k = 1; k <= nc; k++) { var nt = wppShapeText(ns.Item(k)); if (nt) { nn.push(nt); } }
          if (nn.length > 0) { entry.notes = nn; }
        } catch (eN) {}
      }
      slides.push(entry);
    }
    return { presentation: strProp(pres, "Name"), slideCount: count, slides: slides };
  }

  function cmdWppAddTable(app, params) {
    var valueRows = paramMatrix(params, "values", false);
    var rows = paramInt(params, "rows", 0);
    var cols = paramInt(params, "cols", 0);
    if (valueRows !== null && valueRows.length > 0) {
      var maxCols = 0, i;
      for (i = 0; i < valueRows.length; i++) {
        if (valueRows[i].length > maxCols) { maxCols = valueRows[i].length; }
      }
      if (valueRows.length > rows) { rows = valueRows.length; }
      if (maxCols > cols) { cols = maxCols; }
    }
    if (!(rows > 0) || !(cols > 0)) {
      throw new Error("'rows' and 'cols' are required (or pass 'values' as a 2D array to size the table)");
    }
    var left = wppNumber(params, "left", 60);
    var top = wppNumber(params, "top", 120);
    var width = wppNumber(params, "width", 600);
    var height = wppNumber(params, "height", 200);
    var r = wppSlide(app, params);
    var shape = r.slide.Shapes.AddTable(rows, cols, left, top, width, height);
    var filled = 0, rr, cc;
    if (valueRows !== null) {
      for (rr = 0; rr < valueRows.length && rr < rows; rr++) {
        for (cc = 0; cc < valueRows[rr].length && cc < cols; cc++) {
          shape.Table.Cell(rr + 1, cc + 1).Shape.TextFrame.TextRange.Text = cellText(valueRows[rr][cc]);
          filled++;
        }
      }
    }
    var nm = strProp(shape, "Name");
    return { slide: r.index, rows: rows, cols: cols, filled: filled, shape: String(nm) };
  }

  function cmdWppSetNotes(app, params) {
    var text = paramString(params, "text", "");
    if (text === null || text === undefined) { text = ""; }
    var r = wppSlide(app, params);
    r.slide.NotesPage.Shapes.Placeholders.Item(2).TextFrame.TextRange.Text = text;
    return { slide: r.index, notes: text.length };
  }

  function wppSlideTexts(pres) {
    var lines = [], i, j;
    var count = Number(pres.Slides.Count);
    for (i = 1; i <= count; i++) {
      var slide = pres.Slides.Item(i);
      var n = Number(slide.Shapes.Count);
      for (j = 1; j <= n; j++) {
        var t = wppShapeText(slide.Shapes.Item(j));
        if (t) { lines.push(t); }
      }
      lines.push("");
    }
    return lines.join("\n");
  }

  function cmdWppExportPdf(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    var pres = wppPres(app);
    try { pres.ExportAsFixedFormat(path, 32); } catch (e0) {}
    var fs = app.FileSystem;
    if (fsExists(fs, path)) { return { exported: path }; }
    var text = "";
    try { text = wppSlideTexts(pres); } catch (e1) {}
    return writePdf(fs, path, text, "Presentation");
  }

  function cmdWppSave(app) {
    var pres = wppPres(app);
    var p = strProp(pres, "Path");
    if (!p) { throw new Error("the document has never been saved - use saveAs with a 'path'"); }
    pres.Save();
    var info = wppDocInfo(pres);
    return { saved: true, fullName: info.fullName };
  }

  function cmdWppSaveAs(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    var pres = wppPres(app);
    pres.SaveAs(path);
    var info = wppDocInfo(pres);
    var fullName = info.fullName || path;
    return { saved: true, fullName: fullName };
  }

  function cmdWppCloseAll(app) {
    // WPP KHONG nhan tham so cho `Presentation.Close`: `Close(false)` bi tu choi bang "参数太多.", va vi loi
    // bi nuot ngay day nen lenh truoc kia bao "thanh cong" ma khong dong bai nao (do 12/10/2026: hai bai dang
    // mo ma `closed` = 0). Goi khong tham so moi dung: do duoc la dong that, `Presentations.Count` ve 0. Nhanh
    // du phong cho ban WPS khac chi nhan dang co tham so — ET thi `Workbook.Close(false)` chay duoc.
    // Do tren hai bai DA LUU (`Saved` = -1); duong cuu bai chua luu o tren giu cho Close() khong gap bai ban.
    return closeAllDocuments(app, app.Presentations, "presentation.pptx", function (pres) {
      try { pres.Close(); }
      catch (e3) { pres.Close(false); }
    });
  }

  // Soat bo cuc: khong bao `overflow` (WPS khong do duoc chieu cao chu that).
  function cmdWppCheckLayout(app, params) {
    var pres = wppPres(app);
    var count = Number(pres.Slides.Count);
    var only = 0;
    if (hasParam(params, "slide")) { only = paramInt(params, "slide", 0); }
    if (only < 0 || only > count) {
      throw new Error("'slide' must be between 1 and " + count + ", got " + only);
    }
    var slideWidth = Number(pres.PageSetup.SlideWidth);
    var slideHeight = Number(pres.PageSetup.SlideHeight);
    var start = only ? only : 1, end = only ? only : count;
    var slides = [], issueCount = 0, i, j, k, m;
    for (i = start; i <= end; i++) {
      var slide = pres.Slides.Item(i);
      var n = Number(slide.Shapes.Count);
      var boxes = [];
      for (j = 1; j <= n; j++) {
        var shape = slide.Shapes.Item(j);
        var nm = strProp(shape, "Name");
        var t = wppShapeText(shape);
        var size = 0;
        if (t) {
          try { size = Number(shape.TextFrame.TextRange.Font.Size) || 0; } catch (e) { size = 0; }
        }
        boxes.push({ name: String(nm), left: Number(shape.Left), top: Number(shape.Top),
          width: Number(shape.Width), height: Number(shape.Height), chars: t.length, minFont: size });
      }
      var issues = [], total = 0;
      for (j = 0; j < boxes.length; j++) {
        var b = boxes[j];
        total += b.chars;
        if (b.left < -2 || b.top < -2 ||
            b.left + b.width > slideWidth + 2 || b.top + b.height > slideHeight + 2) {
          issues.push({ type: "offslide",
            detail: "'" + b.name + "' goes outside the slide (" + Math.round(slideWidth) + "x" + Math.round(slideHeight) + "pt)",
            shapes: [b.name] });
        }
        if (b.chars && b.minFont > 0 && b.minFont < 12) {
          issues.push({ type: "small-font",
            detail: "'" + b.name + "' uses " + b.minFont + "pt text; keep body text at 12pt or more",
            shapes: [b.name] });
        }
      }
      var area = slideWidth * slideHeight;
      var content = [];
      for (j = 0; j < boxes.length; j++) {
        if (boxes[j].width * boxes[j].height < area * 0.9) { content.push(boxes[j]); }
      }
      for (j = 0; j < content.length; j++) {
        for (k = j + 1; k < content.length; k++) {
          var a = content[j], c = content[k];
          var w = Math.min(a.left + a.width, c.left + c.width) - Math.max(a.left, c.left);
          var h = Math.min(a.top + a.height, c.top + c.height) - Math.max(a.top, c.top);
          var over = (w > 0 && h > 0) ? w * h : 0;
          var smaller = Math.min(a.width * a.height, c.width * c.height);
          if (smaller > 0 && over > smaller * 0.1) {
            issues.push({ type: "overlap",
              detail: "'" + a.name + "' and '" + c.name + "' overlap (" + Math.round(100 * over / smaller) + "% of the smaller shape)",
              shapes: [a.name, c.name] });
          }
        }
      }
      if (total > 600) {
        issues.push({ type: "dense",
          detail: "the slide has " + total + " characters; keep one message per slide and move details to notes (wpp.setNotes)" });
      }
      issueCount += issues.length;
      var shapesOut = [];
      for (m = 0; m < boxes.length; m++) {
        shapesOut.push({ name: boxes[m].name, left: boxes[m].left, top: boxes[m].top,
          width: boxes[m].width, height: boxes[m].height, chars: boxes[m].chars });
      }
      slides.push({ index: i, shapes: shapesOut, issues: issues });
    }
    return { slideWidth: slideWidth, slideHeight: slideHeight, issueCount: issueCount, slides: slides };
  }

  function cmdWppDeleteSlide(app, params) {
    var r = wppSlide(app, params);
    r.slide.Delete();
    return { deleted: r.index, slideCount: Number(r.pres.Slides.Count) };
  }

  function wppAlignNum(raw) {
    var name = (raw === undefined || raw === null) ? "left" : String(raw).trim().toLowerCase();
    if (name === "left") { return 1; }
    if (name === "center" || name === "centre") { return 2; }
    if (name === "right") { return 3; }
    throw new Error("'align' must be left/center/right, got '" + String(raw) + "'");
  }

  function cmdWppAddText(app, params) {
    var text = paramString(params, "text", "");
    if (text === null || text === undefined) { text = ""; }
    // Kiem align TRUOC khi tao shape: sai thi khong de lai textbox rac.
    var align = 1, wantAlign = false;
    if (hasParam(params, "align")) {
      align = wppAlignNum(params.align);
      wantAlign = true;
    }
    var left = wppNumber(params, "left", 60);
    var top = wppNumber(params, "top", 60);
    var width = wppNumber(params, "width", 540);
    var height = wppNumber(params, "height", 120);
    var r = wppSlide(app, params);
    var shape = r.slide.Shapes.AddTextbox(1, left, top, width, height);
    shape.TextFrame.TextRange.Text = text;
    if (hasParam(params, "fontSize")) { shape.TextFrame.TextRange.Font.Size = wppNumber(params, "fontSize", 18); }
    if (hasParam(params, "bold")) { shape.TextFrame.TextRange.Font.Bold = paramBool(params, "bold", false) ? -1 : 0; }
    if (hasParam(params, "color")) {
      var bgr = parseBgrColor(paramString(params, "color", null));
      if (bgr !== null) { shape.TextFrame.TextRange.Font.Color.RGB = bgr; }
    }
    if (wantAlign) { shape.TextFrame.TextRange.ParagraphFormat.Alignment = align; }
    var nm = strProp(shape, "Name");
    return { slide: r.index, shape: String(nm) };
  }

  function cmdWppAddTextBox(app, params) {
    var text = paramString(params, "text", "");
    if (text === null || text === undefined) { text = ""; }
    var left = wppNumber(params, "left", 60);
    var top = wppNumber(params, "top", 60);
    var width = wppNumber(params, "width", 540);
    var height = wppNumber(params, "height", 120);
    var r = wppSlide(app, params);
    var shape = r.slide.Shapes.AddTextbox(1, left, top, width, height);
    shape.TextFrame.TextRange.Text = text;
    var nm = strProp(shape, "Name");
    return { slide: r.index, shape: String(nm) };
  }

  function cmdWppAddImage(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    var left = wppNumber(params, "left", 60);
    var top = wppNumber(params, "top", 60);
    var r = wppSlide(app, params);
    var fs = app.FileSystem;
    if (!fsExists(fs, path)) { throw new Error("image file not found: '" + path + "'"); }
    var shape;
    try {
      shape = r.slide.Shapes.AddPicture(path, 0, 0, left, top, -1, -1);
    } catch (e) {
      throw new Error("image file not found: '" + path + "'");
    }
    // Giu ty le goc khi chi cho mot chieu (doc kich thuoc goc tu shape vua chen).
    var origW = Number(shape.Width), origH = Number(shape.Height);
    if (hasParam(params, "width") && hasParam(params, "height")) {
      shape.Width = wppNumber(params, "width", origW);
      shape.Height = wppNumber(params, "height", origH);
    } else if (hasParam(params, "width")) {
      var w = wppNumber(params, "width", origW);
      shape.Width = w;
      if (origW && origH) { shape.Height = origH * w / origW; }
    } else if (hasParam(params, "height")) {
      var h = wppNumber(params, "height", origH);
      shape.Height = h;
      if (origW && origH) { shape.Width = origW * h / origH; }
    }
    var nm = strProp(shape, "Name");
    return { slide: r.index, shape: String(nm) };
  }

  var WPP_LAYOUTS = { 1: true, 2: true, 11: true, 12: true };

  function cmdWppAddSlide(app, params) {
    var layout = paramInt(params, "layout", 12);
    if (!WPP_LAYOUTS[layout]) {
      throw new Error("'layout' must be one of 1, 2, 11, 12, got " + layout);
    }
    var pres = wppPres(app);
    var count = Number(pres.Slides.Count);
    pres.Slides.Add(count + 1, layout);
    return { slide: count + 1, layout: layout };
  }

  var HANDLERS = {
    "app.info": cmdAppInfo,
    "writer.getText": cmdGetText,
    "writer.outline": cmdWriterOutline,
    "writer.selection": cmdSelection,
    "writer.typeText": cmdTypeText,
    "writer.appendText": cmdAppendText,
    "writer.insertStyledText": cmdInsertStyledText,
    "writer.heading": cmdHeading,
    "writer.formatSelection": cmdFormatSelection,
    "writer.setParagraphAlignment": cmdSetParagraphAlignment,
    "writer.insertTable": cmdInsertTable,
    "writer.insertPageBreak": cmdInsertPageBreak,
    "writer.checkTables": cmdWriterCheckTables,
    "writer.replaceAll": cmdReplaceAll,
    "writer.undo": cmdUndo,
    "writer.save": cmdSave,
    "writer.saveAs": cmdSaveAs,
    "writer.exportPdf": cmdExportPdf,
    "writer.lockSelection": cmdLockSelection,
    "writer.unlockSelection": cmdUnlockSelection,
    "et.newWorkbook": cmdEtNewWorkbook,
    "et.open": cmdEtOpen,
    "et.listSheets": cmdEtListSheets,
    "et.sheetInfo": cmdEtSheetInfo,
    "et.addSheet": cmdEtAddSheet,
    "et.renameSheet": cmdEtRenameSheet,
    "et.activateSheet": cmdEtActivateSheet,
    "et.readRange": cmdEtReadRange,
    "et.writeRange": cmdEtWriteRange,
    "et.writeRanges": cmdEtWriteRanges,
    "et.importCsv": cmdEtImportCsv,
    "et.fillRange": cmdEtFillRange,
    "et.formatRange": cmdEtFormatRange,
    "et.setConditionalFormat": cmdEtSetCondFormat,
    "et.listConditionalFormats": cmdEtListCondFormats,
    "et.addChart": cmdEtAddChart,
    "et.listCharts": cmdEtListCharts,
    "et.undo": cmdEtUndo,
    "et.exportPdf": cmdEtExportPdf,
    "et.save": cmdEtSave,
    "et.saveAs": cmdEtSaveAs,
    "et.closeAll": cmdEtCloseAll,
    "et.checkRange": cmdEtCheckRange,
    "writer.goto": cmdWriterGoto,
    "et.goto": cmdEtGoto,
    "wpp.goto": cmdWppGoto,
    "wpp.newPresentation": cmdWppNewPresentation,
    "wpp.open": cmdWppOpen,
    "wpp.listSlides": cmdWppListSlides,
    "wpp.addSlide": cmdWppAddSlide,
    "wpp.addText": cmdWppAddText,
    "wpp.addTextBox": cmdWppAddTextBox,
    "wpp.addImage": cmdWppAddImage,
    "wpp.addTable": cmdWppAddTable,
    "wpp.setNotes": cmdWppSetNotes,
    "wpp.deleteSlide": cmdWppDeleteSlide,
    "wpp.exportPdf": cmdWppExportPdf,
    "wpp.save": cmdWppSave,
    "wpp.saveAs": cmdWppSaveAs,
    "wpp.closeAll": cmdWppCloseAll,
    "wpp.checkLayout": cmdWppCheckLayout
  };

  function catalog() {
    var out = [];
    for (var i = 0; i < CATALOG.length; i++) {
      var c = CATALOG[i];
      var params = [];
      for (var j = 0; j < c.params.length; j++) {
        params.push({ name: c.params[j].name, required: c.params[j].required, hint: c.params[j].hint });
      }
      out.push({ name: c.name, kind: c.kind, agent: c.agent, undo: c.undo, summary: c.summary, params: params });
    }
    return out;
  }

  // appKind(app): "et" (Spreadsheets) | "wpp" (Presentation) | "wps" (Writer, mac dinh).
  // Cung mot add-in nap trong ca ba app (publish.xml type="wps,et,wpp") nen phai doc
  // app.Name dang chay. KHONG BAO GIO throw: app la/thieu Name/Name khong phai string
  // hay doc Name loi deu tra "wps".
  function appKind(app) {
    try {
      if (app === undefined || app === null) { return "wps"; }
      var name = app.Name;
      if (typeof name === "function") {
        try { name = name.call(app); } catch (e) { return "wps"; }
      }
      if (typeof name !== "string") { return "wps"; }
      if (/spreadsheet/i.test(name)) { return "et"; }
      if (/presentation|slide/i.test(name)) { return "wpp"; }
      return "wps";
    } catch (e2) {
      return "wps";
    }
  }

  // catalogFor(kind): chi entry dung chung moi app (kind null/""/undefined) hoac kind === tham so.
  // catalog() giu nguyen: tra day du registry (khong loc).
  function catalogFor(kind) {
    var out = [];
    for (var i = 0; i < CATALOG.length; i++) {
      var c = CATALOG[i];
      if (c.kind === null || c.kind === "" || c.kind === undefined || c.kind === kind) {
        var params = [];
        for (var j = 0; j < c.params.length; j++) {
          params.push({ name: c.params[j].name, required: c.params[j].required, hint: c.params[j].hint });
        }
        out.push({ name: c.name, kind: c.kind, agent: c.agent, undo: c.undo, summary: c.summary, params: params });
      }
    }
    return out;
  }

  function catalogEntry(action) {
    for (var i = 0; i < CATALOG.length; i++) {
      if (CATALOG[i].name === action) { return CATALOG[i]; }
    }
    return null;
  }

  function appDisplayName(app, fallback) {
    try {
      var name = strProp(app, "Name");
      if (name) { return name; }
    } catch (e) {}
    return fallback;
  }

  function execute(app, action, params) {
    try {
      if (app === undefined || app === null) {
        return { ok: false, error: "no Application object (open a document in WPS first)" };
      }
      var kind = "wps";
      try { kind = appKind(app); } catch (eK) { kind = "wps"; }
      var entry = catalogEntry(action);
      if (entry !== null && entry.kind !== null && entry.kind !== "" && entry.kind !== undefined &&
          entry.kind !== kind) {
        var running = appDisplayName(app, kind);
        return { ok: false, error: "\"" + action + "\" cannot run in \"" + running + "\" (this command needs the other WPS application)" };
      }
      var handler = HANDLERS[action];
      if (!handler) {
        return { ok: false, error: "unknown action: " + action };
      }
      var p = params;
      if (p === undefined || p === null) { p = {}; }
      // Vung khoa: chuan bi TRUOC khi chay (co the tu choi), kiem SAU khi buoc hoan tac da dong - dung thu
      // tu cua ban Windows (RunInSelectionLock): handler trong UndoRecordScope, dong scope, roi moi kiem.
      var lock = null;
      try {
        lock = lockBegin(app, action);
      } catch (lockErr) {
        return { ok: false, error: errMessage(lockErr) };
      }
      if (MUTATING[action]) {
        var undo = null;
        try { undo = app.UndoRecord; } catch (e) { undo = null; }
        var started = false;
        if (undo !== undefined && undo !== null) {
          try {
            undo.StartCustomRecord("AI: " + action);
            started = true;
          } catch (e2) { started = false; }
        }
        var outcome;
        try {
          outcome = { ok: true, result: handler(app, p) };
        } catch (err) {
          outcome = { ok: false, error: errMessage(err) };
        } finally {
          if (started) {
            try { undo.EndCustomRecord(); } catch (e3) {}
          }
        }
        if (outcome.ok && lock !== null) {
          try {
            lockVerify(lock, action);
          } catch (lockErr2) {
            return { ok: false, error: errMessage(lockErr2) };
          }
        }
        return outcome;
      }
      var etMark = etUndoMark(kind, entry);
      try {
        var res = handler(app, p);
        if (lock !== null) { lockVerify(lock, action); }
        etUndoSeal(etMark, action, true);
        return { ok: true, result: res };
      } catch (err2) {
        etUndoSeal(etMark, action, false);
        return { ok: false, error: errMessage(err2) };
      }
    } catch (fatal) {
      try {
        return { ok: false, error: errMessage(fatal) };
      } catch (e4) {
        return { ok: false, error: "unknown error" };
      }
    }
  }

  // hasHandler(action): add-in co chay duoc lenh nay khong (CATALOG la hop dong day du
  // registry, ke ca lenh chua co handler; main.js dung de loc danh sach khai voi Core).
  function hasHandler(action) {
    return typeof HANDLERS[action] === "function";
  }

  var api = { catalog: catalog, catalogFor: catalogFor, appKind: appKind, execute: execute, hasHandler: hasHandler };
  try {
    if (root) { root.AxiomCommands = api; }
  } catch (e) {}
  try {
    if (typeof module !== "undefined" && module && module.exports) {
      module.exports = api;
    }
  } catch (e2) {}
})(typeof window !== "undefined" ? window : (typeof globalThis !== "undefined" ? globalThis : this));
