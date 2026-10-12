/*
 * Axiom Office - WPS JS add-in command registry + handlers.
 *
 * Global AxiomCommands { catalog(), catalogFor(kind), appKind(app), hasHandler(action),
 * execute(app, action, params) }.
 * CATALOG la ban SINH TU DONG tu registry Python (axiom.commands) bang
 * scripts/generate_mcp_commands.py - KHONG sua tay (vung giua hai moc BEGIN/END SINH TU DONG).
 * HANDLERS + MUTATING viet tay.
 * Port of src/AxiomOffice/Bridge/CommandDispatcher.Writer.cs (+ app.info) with the
 * same parameter names and result shapes as the LibreOffice handlers in
 * src/AxiomOffice.LibreOffice/python/pythonpath/axiom/writer.py.
 *
 * ES5 only (CEF Chrome 87): no let/const/arrow/template strings/optional chaining.
 * execute() never throws: errors become {ok:false, error}.
 */
(function (root) {
  "use strict";

  var MUTATING = {
    "writer.typeText": true,
    "writer.appendText": true,
    "writer.insertStyledText": true,
    "writer.heading": true,
    "writer.formatSelection": true,
    "writer.setParagraphAlignment": true,
    "writer.insertTable": true,
    "writer.insertPageBreak": true,
    "writer.replaceAll": true
  };

  // --- BEGIN SINH TU DONG (scripts/generate_mcp_commands.py) - KHONG SUA TAY ---
  var CATALOG = [
    { name: "app.info", kind: null, agent: false,
      summary: "Tên/version app, tài liệu đang mở, `state` (tài liệu, cửa sổ, visible)",
      params: [] },
    { name: "ai.ask", kind: null, agent: false,
      summary: "Chạy AI agent trên tài liệu đang mở; trả `reply`, `transcript`, `seconds`, `rounds`",
      params: [{ name: "prompt", required: true, hint: null }] },
    { name: "ui.askpane", kind: null, agent: false,
      summary: "Mở panel Ask AI trong sidebar",
      params: [] },
    { name: "ui.setup", kind: null, agent: false,
      summary: "Mở wizard thiết lập (nhập máy chủ AI, model, kiểm tra kết nối)",
      params: [] },
    { name: "app.screenshot", kind: null, agent: false,
      summary: "Ảnh chụp trang/slide hiện tại (PNG base64, thu nhỏ theo `maxWidth`, mặc định 1280)",
      params: [{ name: "maxWidth", required: false, hint: null }] },
    { name: "writer.newDocument", kind: "wps", agent: false,
      summary: "Tạo tài liệu mới",
      params: [] },
    { name: "writer.open", kind: "wps", agent: false,
      summary: "Mở .docx/.doc/.odt",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "writer.getText", kind: "wps", agent: true,
      summary: "Đọc toàn bộ text",
      params: [{ name: "maxChars", required: false, hint: null }] },
    { name: "writer.selection", kind: "wps", agent: true,
      summary: "Text + vị trí đang chọn",
      params: [] },
    { name: "writer.typeText", kind: "wps", agent: true,
      summary: "Gõ tại con trỏ",
      params: [{ name: "text", required: true, hint: null }] },
    { name: "writer.appendText", kind: "wps", agent: true,
      summary: "Nối vào cuối tài liệu",
      params: [{ name: "text", required: true, hint: null }] },
    { name: "writer.insertStyledText", kind: "wps", agent: true,
      summary: "Chèn text có định dạng tại con trỏ (`color` dạng `#RRGGBB`)",
      params: [
        { name: "text", required: true, hint: null },
        { name: "bold", required: false, hint: null },
        { name: "italic", required: false, hint: null },
        { name: "underline", required: false, hint: null },
        { name: "size", required: false, hint: null },
        { name: "color", required: false, hint: null },
        { name: "font", required: false, hint: null }
      ] },
    { name: "writer.heading", kind: "wps", agent: true,
      summary: "Heading 1-9 (`level`, mặc định 1) + tự xuống dòng (`break`, mặc định true)",
      params: [
        { name: "text", required: false, hint: null },
        { name: "level", required: false, hint: null },
        { name: "break", required: false, hint: null }
      ] },
    { name: "writer.formatSelection", kind: "wps", agent: true,
      summary: "Định dạng vùng chọn",
      params: [
        { name: "bold", required: false, hint: null },
        { name: "italic", required: false, hint: null },
        { name: "underline", required: false, hint: null },
        { name: "size", required: false, hint: null },
        { name: "color", required: false, hint: null },
        { name: "font", required: false, hint: null },
        { name: "alignment", required: false, hint: null }
      ] },
    { name: "writer.setParagraphAlignment", kind: "wps", agent: true,
      summary: "Căn đoạn: left/center/right/justify",
      params: [{ name: "alignment", required: true, hint: null }] },
    { name: "writer.insertTable", kind: "wps", agent: true,
      summary: "Chèn bảng; `rows`/`cols` tự suy ra/nới theo `values`",
      params: [
        { name: "rows", required: false, hint: null },
        { name: "cols", required: false, hint: null },
        { name: "values", required: false, hint: "2D array of rows" },
        { name: "style", required: false, hint: null }
      ] },
    { name: "writer.formatTable", kind: "wps", agent: true,
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
        { name: "autoFit", required: false, hint: "content/window" }
      ] },
    { name: "writer.insertPageBreak", kind: "wps", agent: true,
      summary: "Ngắt trang",
      params: [] },
    { name: "writer.insertImage", kind: "wps", agent: true,
      summary: "Chèn ảnh tại con trỏ (kích thước theo point)",
      params: [
        { name: "path", required: true, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null }
      ] },
    { name: "writer.insertHyperlink", kind: "wps", agent: true,
      summary: "Chèn liên kết",
      params: [
        { name: "url", required: true, hint: null },
        { name: "text", required: false, hint: null }
      ] },
    { name: "writer.replaceAll", kind: "wps", agent: true,
      summary: "Tìm và thay toàn bộ",
      params: [
        { name: "find", required: true, hint: null },
        { name: "replace", required: false, hint: null }
      ] },
    { name: "writer.undo", kind: "wps", agent: true,
      summary: "Hoàn tác (mỗi thao tác AI = 1 bước)",
      params: [{ name: "count", required: false, hint: null }] },
    { name: "writer.exportPdf", kind: "wps", agent: true,
      summary: "Xuất PDF",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "writer.save", kind: "wps", agent: true,
      summary: "Lưu",
      params: [] },
    { name: "writer.saveAs", kind: "wps", agent: true,
      summary: "Lưu thành file mới",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "writer.closeAll", kind: "wps", agent: false,
      summary: "**Đóng mọi tài liệu, không lưu**",
      params: [] },
    { name: "et.newWorkbook", kind: "et", agent: false,
      summary: "Tạo sổ tính mới",
      params: [] },
    { name: "et.open", kind: "et", agent: false,
      summary: "Mở .xlsx/.xls/.ods/.csv",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "et.listSheets", kind: "et", agent: true,
      summary: "Danh sách sheet + sheet đang active",
      params: [] },
    { name: "et.sheetInfo", kind: "et", agent: true,
      summary: "Kích thước dùng thật của TỪNG sheet trong một lời gọi (`usedRange`/`rows`/`cols`/`empty`) — dùng để soát sheet nào thực sự có nội dung, không chỉ có tên",
      params: [] },
    { name: "et.addSheet", kind: "et", agent: true,
      summary: "Thêm sheet mới (bỏ trống `name` thì đặt tên Sheet1, Sheet2...)",
      params: [
        { name: "name", required: false, hint: null },
        { name: "index", required: false, hint: "0-based, default: append" }
      ] },
    { name: "et.renameSheet", kind: "et", agent: true,
      summary: "Đổi tên sheet",
      params: [
        { name: "sheet", required: true, hint: null },
        { name: "name", required: true, hint: null }
      ] },
    { name: "et.activateSheet", kind: "et", agent: true,
      summary: "Chuyển sheet",
      params: [{ name: "sheet", required: true, hint: null }] },
    { name: "et.readRange", kind: "et", agent: true,
      summary: "Đọc vùng, ví dụ `A1:C10`; `formulas=true` trả về công thức (ô không có công thức = null)",
      params: [
        { name: "range", required: true, hint: null },
        { name: "sheet", required: false, hint: null },
        { name: "formulas", required: false, hint: "true = trả mảng `formulas` thay cho `values`" }
      ] },
    { name: "et.writeRange", kind: "et", agent: true,
      summary: "Ghi vùng từ ô góc trên-trái; `values` là mảng 2 chiều. Công thức viết theo cú pháp en-US (dấu phẩy); nếu máy dùng dấu chấm phẩy thì tự đổi. Công thức còn lỗi trả về ở `formulaErrors`",
      params: [
        { name: "range", required: true, hint: "top-left cell e.g. 'A1'" },
        { name: "values", required: true, hint: "2D array of rows e.g. [[\"Tên\",\"Điểm\"],[\"An\",9.5]]" },
        { name: "sheet", required: false, hint: null }
      ] },
    { name: "et.importCsv", kind: "et", agent: true,
      summary: "Nạp một file CSV vào sổ: không có `range` thì tạo sheet mới đặt tên theo tên file, có `range` thì ghi vào ô góc đó. Ô trông như số thì thành số, còn lại là chuỗi; ngày để nguyên chuỗi",
      params: [
        { name: "path", required: true, hint: "e.g. 'D:\\data\\raw.csv'" },
        { name: "range", required: false, hint: "top-left cell; bỏ trống thì tạo sheet mới" },
        { name: "sheet", required: false, hint: null },
        { name: "delimiter", required: false, hint: "một ký tự, mặc định ','" },
        { name: "encoding", required: false, hint: "mặc định utf-8-sig" }
      ] },
    { name: "et.writeRanges", kind: "et", agent: true,
      summary: "Ghi NHIỀU vùng trong MỘT lời gọi: `writes` là mảng {range, values, sheet?}. Dùng khi cần viết nhiều khối trong cùng một phản hồi (bớt vòng qua bridge); công thức vẫn theo cú pháp en-US và lỗi công thức gom ở `formulaErrors`. Kiểm hết tham số trước khi ghi: một vùng sai thì không vùng nào được ghi",
      params: [
        { name: "writes", required: true, hint: "array of {range, values, sheet?}" },
        { name: "sheet", required: false, hint: null }
      ] },
    { name: "et.fillRange", kind: "et", agent: true,
      summary: "Viết MỘT công thức vào ô góc trên-trái của `range` rồi điền ra cả vùng, tham chiếu tương đối tự dịch (dùng cho bảng nghìn dòng: đừng gửi từng ô)",
      params: [
        { name: "range", required: true, hint: "the whole area e.g. 'B2:H1000'" },
        { name: "formula", required: false, hint: "written to the top-left cell first" },
        { name: "sheet", required: false, hint: null }
      ] },
    { name: "et.formatRange", kind: "et", agent: true,
      summary: "Định dạng vùng",
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
        { name: "sheet", required: false, hint: null }
      ] },
    { name: "et.setConditionalFormat", kind: "et", agent: true,
      summary: "Đặt định dạng điều kiện cho vùng: `rules` là mảng rule, mỗi rule có operator (less/lessEqual/greater/greaterEqual/equal/notEqual/between/notBetween/formula), formula1, formula2 (chỉ between), và styleName có sẵn (Good/Bad/Neutral/Warning/Error/...) hoặc bold/italic/fontColor/fillColor/numFmt. Gọi lại trên cùng vùng thì THAY rule cũ của vùng đó, nên nhiều rule trên một vùng phải để trong MỘT lần gọi",
      params: [
        { name: "range", required: true, hint: null },
        { name: "rules", required: true, hint: "array of rule objects" },
        { name: "sheet", required: false, hint: null }
      ] },
    { name: "et.listConditionalFormats", kind: "et", agent: true,
      summary: "Liệt kê định dạng điều kiện đang có (đọc lại để tự kiểm); bỏ trống `range` thì cả sheet",
      params: [
        { name: "range", required: false, hint: "chỉ xem một vùng" },
        { name: "sheet", required: false, hint: null }
      ] },
    { name: "et.addChart", kind: "et", agent: true,
      summary: "Chèn biểu đồ từ vùng dữ liệu; `type` column/bar/line/pie/area/scatter, `width`/`height` tính bằng cm",
      params: [
        { name: "range", required: true, hint: "source data e.g. 'A1:B13'" },
        { name: "type", required: false, hint: "column (default)/bar/line/pie/area/scatter" },
        { name: "title", required: false, hint: null },
        { name: "name", required: false, hint: null },
        { name: "anchor", required: false, hint: "top-left cell, default 'A1'" },
        { name: "width", required: false, hint: "cm, default 12" },
        { name: "height", required: false, hint: "cm, default 7" },
        { name: "sheet", required: false, hint: null }
      ] },
    { name: "et.listCharts", kind: "et", agent: true,
      summary: "Danh sách biểu đồ trên sheet",
      params: [{ name: "sheet", required: false, hint: null }] },
    { name: "et.undo", kind: "et", agent: true,
      summary: "Hoàn tác",
      params: [{ name: "count", required: false, hint: null }] },
    { name: "et.exportPdf", kind: "et", agent: true,
      summary: "Xuất PDF",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "et.save", kind: "et", agent: true,
      summary: "Lưu",
      params: [] },
    { name: "et.saveAs", kind: "et", agent: true,
      summary: "Lưu thành file mới",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "et.closeAll", kind: "et", agent: false,
      summary: "**Đóng mọi sổ, không lưu**",
      params: [] },
    { name: "wpp.newPresentation", kind: "wpp", agent: false,
      summary: "Tạo bài trình bày mới",
      params: [] },
    { name: "wpp.open", kind: "wpp", agent: false,
      summary: "Mở .pptx/.ppt/.odp",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "wpp.listSlides", kind: "wpp", agent: true,
      summary: "Số slide + text từng slide",
      params: [] },
    { name: "wpp.addSlide", kind: "wpp", agent: true,
      summary: "Thêm slide cuối; `layout` mặc định 12 = trống (1 = tiêu đề, 2 = tiêu đề + nội dung, 11 = chỉ tiêu đề)",
      params: [{ name: "layout", required: false, hint: "number: 1 title, 2 title+content, 11 title only, 12 blank" }] },
    { name: "wpp.addText", kind: "wpp", agent: true,
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
        { name: "align", required: false, hint: null }
      ] },
    { name: "wpp.addTextBox", kind: "wpp", agent: false,
      summary: "Textbox",
      params: [
        { name: "text", required: true, hint: null },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null }
      ] },
    { name: "wpp.addImage", kind: "wpp", agent: true,
      summary: "Chèn ảnh (kích thước gốc nếu bỏ trống `width`/`height`)",
      params: [
        { name: "path", required: true, hint: null },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null }
      ] },
    { name: "wpp.addTable", kind: "wpp", agent: true,
      summary: "Bảng; `rows`/`cols` tự suy ra/nới theo `values`",
      params: [
        { name: "rows", required: false, hint: null },
        { name: "cols", required: false, hint: null },
        { name: "values", required: false, hint: "2D array of rows" },
        { name: "slide", required: false, hint: null },
        { name: "left", required: false, hint: null },
        { name: "top", required: false, hint: null },
        { name: "width", required: false, hint: null },
        { name: "height", required: false, hint: null }
      ] },
    { name: "wpp.setNotes", kind: "wpp", agent: true,
      summary: "Ghi chú thuyết trình",
      params: [
        { name: "text", required: true, hint: null },
        { name: "slide", required: false, hint: null }
      ] },
    { name: "wpp.deleteSlide", kind: "wpp", agent: true,
      summary: "Xoá slide (mặc định slide cuối)",
      params: [{ name: "slide", required: false, hint: null }] },
    { name: "wpp.exportPdf", kind: "wpp", agent: true,
      summary: "Xuất PDF",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "wpp.save", kind: "wpp", agent: true,
      summary: "Lưu",
      params: [] },
    { name: "wpp.saveAs", kind: "wpp", agent: true,
      summary: "Lưu thành file mới",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "wpp.closeAll", kind: "wpp", agent: false,
      summary: "**Đóng mọi bài trình bày, không lưu**",
      params: [] },
    { name: "writer.checkTables", kind: "wps", agent: true,
      summary: "Soát các bảng (chỉ đọc): số dòng/cột, ô trống, ô tiêu đề lẫn đoạn văn. Bảng dàn trang — bảng đã gọi writer.formatTable với borders:false — được đánh dấu layout và KHÔNG báo empty-cells (ô trống ở đó là cố ý: khoảng trống ký, ô chờ điền)",
      params: [] },
    { name: "et.checkRange", kind: "et", agent: true,
      summary: "Soát bảng dữ liệu (chỉ đọc): tiêu đề trống, kiểu lẫn lộn, số dạng chữ, number format, dữ liệu lạc ngoài bảng (chỉ khi không truyền range)",
      params: [
        { name: "range", required: false, hint: "default: the used range" },
        { name: "sheet", required: false, hint: null }
      ] },
    { name: "wpp.checkLayout", kind: "wpp", agent: true,
      summary: "Soát bố cục slide (chỉ đọc): chữ tràn khung, shape ra ngoài slide, shape chồng nhau, chữ quá nhỏ",
      params: [{ name: "slide", required: false, hint: "slide number; default: every slide" }] },
  ];
  // --- END SINH TU DONG ---

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
    var depth, key, k;
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
        var build = app.Build;
        if (typeof build === "function") { try { build = build.call(app); } catch (e3) {} }
        if (build !== undefined && build !== null && String(build) !== "") { version = String(build); }
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

  function cmdSelection(app) {
    var sel = app.Selection;
    var text = "";
    try {
      var raw = sel.Text;
      if (typeof raw === "function") { raw = raw.call(sel); }
      text = (raw === undefined || raw === null) ? "" : String(raw);
    } catch (e) { throw e; }
    var start = null, end = null;
    try {
      var s = sel.Start;
      if (typeof s === "function") { s = s.call(sel); }
      if (s !== undefined && s !== null) {
        var n = Number(s);
        if (!isNaN(n)) { start = n; }
      }
    } catch (e2) {}
    try {
      var en = sel.End;
      if (typeof en === "function") { en = en.call(sel); }
      if (en !== undefined && en !== null) {
        var m = Number(en);
        if (!isNaN(m)) { end = m; }
      }
    } catch (e3) {}
    return { text: text, start: start, end: end };
  }

  function cmdTypeText(app, params) {
    var text = paramString(params, "text", null);
    if (text === null || text === undefined) { return { typed: 0 }; }
    app.Selection.TypeText(text);
    return { typed: text.length };
  }

  function cmdAppendText(app, params) {
    var text = paramString(params, "text", null);
    if (text === null || text === undefined) { return { appended: 0 }; }
    var doc = app.ActiveDocument;
    doc.Content.InsertAfter(text);
    return { appended: text.length };
  }

  function cmdInsertStyledText(app, params) {
    var text = paramString(params, "text", null);
    if (!text) { return { inserted: 0 }; }
    applyFont(app.Selection.Font, params);
    app.Selection.TypeText(text);
    return { inserted: text.length };
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
    try {
      content.Find.Execute(findText, false, false, false, false, false, true, 0, false, replaceText, 2);
    } catch (e4) { throw e4; }
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

  function cmdExportPdf(app, params) {
    var path = paramString(params, "path", null);
    if (!path) { throw new Error("'path' is required"); }
    var doc = app.ActiveDocument;
    // Xuat goc cua WPS truoc (dep nhat khi host ho tro). WPS Linux bo qua lenh nay
    // am tham nen phai kiem tra file that; that bai thi tu dung PDF text.
    try { doc.ExportAsFixedFormat(path, 17); } catch (e0) {}
    var wfs = null;
    try { wfs = app.FileSystem; } catch (e1) { wfs = null; }
    if (wfs !== undefined && wfs !== null) {
      if (fsExists(wfs, path)) { return { exported: path }; }
      var text = "";
      try {
        var raw = doc.Content.Text;
        if (typeof raw === "function") { raw = raw.call(doc.Content); }
        if (raw !== undefined && raw !== null) { text = String(raw); }
      } catch (e2) {}
      var pdf = buildPdf(text);
      var codes = [];
      for (var i = 0; i < pdf.length; i++) { codes.push(pdf.charCodeAt(i) & 0xFF); }
      var data = "";
      try {
        var parts = [];
        for (var k = 0; k < codes.length; k += 4096) {
          parts.push(String.fromCharCode.apply(null, codes.slice(k, k + 4096)));
        }
        data = parts.join("");
      } catch (e3) {
        data = "";
        for (var m = 0; m < pdf.length; m++) { data += String.fromCharCode(pdf.charCodeAt(m) & 0xFF); }
      }
      wfs.writeAsBinaryString(path, data);
      if (!fsExists(wfs, path)) { throw new Error("could not write PDF to '" + path + "'"); }
      return { exported: path };
    }
    return { exported: path };
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

  function etSheetNames(wb) {
    var names = [];
    var col = wb.Worksheets;
    var n = col.Count;
    if (typeof n === "function") { n = n.call(col); }
    n = Number(n);
    for (var i = 1; i <= n; i++) {
      var ws = col.Item(i);
      var nm = ws.Name;
      if (typeof nm === "function") { nm = nm.call(ws); }
      names.push(String(nm));
    }
    return names;
  }

  function etFindSheet(wb, name) {
    var col = wb.Worksheets;
    var n = Number(col.Count);
    for (var i = 1; i <= n; i++) {
      var ws = col.Item(i);
      var nm = ws.Name;
      if (typeof nm === "function") { nm = nm.call(ws); }
      if (String(nm) === String(name)) { return ws; }
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
    var aname = active.Name;
    if (typeof aname === "function") { aname = aname.call(active); }
    return { wb: wb, ws: active, name: String(aname) };
  }

  function etRangeAddrOf(ws) {
    var ur = ws.UsedRange;
    var fn = ur.Address;
    var abs = (typeof fn === "function") ? fn.call(ur) : fn;
    return etNormAddr(String(abs));
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

  function etPushCells(sheetName, addrStr, snaps) {
    etUndoStack.push({ kind: "cells", sheet: sheetName, range: addrStr, snaps: snaps });
  }

  function etActiveSheetName(app) {
    var s = app.ActiveSheet;
    var n = s.Name;
    if (typeof n === "function") { n = n.call(s); }
    return String(n);
  }

  function etDocInfo(wb) {
    var name = wb.Name;
    if (typeof name === "function") { name = name.call(wb); }
    var full = wb.FullName;
    if (typeof full === "function") { full = full.call(wb); }
    var saved = wb.Saved;
    if (typeof saved === "function") { saved = saved.call(wb); }
    return { name: String(name), fullName: String(full), saved: !!saved };
  }

  function cmdEtNewWorkbook(app) {
    var wb = app.Workbooks.Add();
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
    etUndoStack.push({ kind: "addSheet", sheet: name });
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
      return { sheet: r.name, range: etAddrStr(box.r1, box.c1, box.r2, box.c2), formulas: fo };
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

  function etDoWriteCells(ws, sheetName, topAddr, rows) {
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
    etPushCells(sheetName, addrStr, snaps);
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
    var res = etDoWriteCells(r.ws, r.name, top, rows);
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
      var out = etDoWriteCells(rr.ws, rr.name, p.top, p.rows);
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
      base = base.replace(/[\[\]\*\?\/\\:]/g, "_").substring(0, 31) || "Import";
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
    var res = etDoWriteCells(ws, sheetName, topAddr, rows);
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
    etUndoStack.push({ kind: "cells", sheet: r.name, range: addrStr, snaps: snaps });
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
    if (!etCondStore[r.name]) { etCondStore[r.name] = []; }
    var bucket = etCondStore[r.name];
    var bi;
    for (bi = bucket.length - 1; bi >= 0; bi--) {
      if (bucket[bi].range === addrStr) { bucket.splice(bi, 1); }
    }
    bucket.push({ range: addrStr, rules: written });
    return { sheet: r.name, range: addrStr, rules: written };
  }

  function cmdEtListCondFormats(app, params) {
    var r = etResolveSheet(app, params);
    var bucket = etCondStore[r.name] || [];
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
      var rules = [];
      for (i = 1; i <= n; i++) {
        var fc = fcCol.Item(i);
        var op = fc.Operator;
        if (typeof op === "function") { op = op.call(fc); }
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
              var t = f.Type;
              if (typeof t === "function") { t = t.call(f); }
              var o = f.Operator;
              if (typeof o === "function") { o = o.call(f); }
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
    var left = cell.Left, top = cell.Top;
    if (typeof left === "function") { left = left.call(cell); }
    if (typeof top === "function") { top = top.call(cell); }
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
    var nm = obj.Name;
    if (typeof nm === "function") { nm = nm.call(obj); }
    var actual = ET_CHART_BACK[Number(chart.ChartType)] || kind;
    if (typeof actual === "string") {
      var chk = obj.Chart.ChartType;
      if (typeof chk === "function") { chk = chk.call(obj.Chart); }
      actual = ET_CHART_BACK[Number(chk)] || kind;
    }
    return { chart: String(nm), sheet: r.name, type: kind, diagram: actual, typeApplied: (actual === kind), titleApplied: titleApplied, range: addrStr, anchor: anchorStr };
  }

  function cmdEtListCharts(app, params) {
    var r = etResolveSheet(app, params);
    var objs = r.ws.ChartObjects();
    var n = Number(objs.Count);
    var charts = [], i;
    for (i = 1; i <= n; i++) {
      var o = objs.Item(i);
      var nm = o.Name;
      if (typeof nm === "function") { nm = nm.call(o); }
      var ct = o.Chart.ChartType;
      if (typeof ct === "function") { ct = ct.call(o.Chart); }
      charts.push({ name: String(nm), diagram: ET_CHART_BACK[Number(ct)] || "" });
    }
    var shapes = [];
    try {
      var sh = r.ws.Shapes;
      var cnt = Number(sh.Count);
      for (i = 1; i <= cnt; i++) {
        var s = sh.Item(i);
        var label = null;
        try { label = s.Name; if (typeof label === "function") { label = label.call(s); } } catch (e) {}
        shapes.push(label === undefined || label === null ? "" : String(label));
      }
    } catch (e2) {}
    return { sheet: r.name, charts: charts, shapes: shapes };
  }

  function cmdEtUndo(app, params) {
    var count = paramInt(params, "count", 1);
    if (isNaN(count)) { count = 1; }
    count = Math.max(1, count);
    var restored = [], undone = 0, k;
    for (k = 0; k < count; k++) {
      if (etUndoStack.length === 0) { break; }
      var entry = etUndoStack.pop();
      if (entry.kind === "addSheet") {
        try {
          var wb = etWb(app);
          var ws = etFindSheet(wb, entry.sheet);
          if (ws) { ws.Delete(); }
        } catch (e) {}
        // Xoa ca ghi nhan dinh dang co dieu kien cua sheet do
        try { delete etCondStore[entry.sheet]; } catch (e2) {}
        restored.push({ sheet: entry.sheet, deleted: true });
        undone++;
      } else {
        try {
          var wb2 = etWb(app);
          var ws2 = etFindSheet(wb2, entry.sheet);
          if (!ws2) {
            // Sheet bi doi ten/xoa: thu sheet dang active
            ws2 = app.ActiveSheet;
          }
          etRestoreCells(ws2, entry.snaps);
          var cells = entry.snaps.length;
          restored.push({ sheet: entry.sheet, range: entry.range, cells: cells });
          undone++;
        } catch (e3) {}
      }
    }
    return { undone: undone, restored: restored };
  }

  function etPdfWrite(fs, path, pdf) {
    var codes = [];
    for (var i = 0; i < pdf.length; i++) { codes.push(pdf.charCodeAt(i) & 0xFF); }
    var data = "";
    try {
      var parts = [];
      for (var q = 0; q < codes.length; q += 4096) {
        parts.push(String.fromCharCode.apply(null, codes.slice(q, q + 4096)));
      }
      data = parts.join("");
    } catch (e) {
      data = "";
      for (var m = 0; m < pdf.length; m++) { data += String.fromCharCode(pdf.charCodeAt(m) & 0xFF); }
    }
    fs.writeAsBinaryString(path, data);
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
            var cell = ws.Cells.Item(rr, cc);
            var t = cell.Text;
            if (typeof t === "function") { t = t.call(cell); }
            rowVals.push(t === undefined || t === null ? "" : String(t));
          }
          lines.push(rowVals.join("\t"));
        }
        lines.push("");
      }
      text = lines.join("\n");
    } catch (e2) {}
    var pdf = buildPdf(text);
    etPdfWrite(fs, path, pdf);
    if (!fsExists(fs, path)) { throw new Error("could not write PDF to '" + path + "'"); }
    return { exported: path };
  }

  function cmdEtSave(app) {
    var wb = etWb(app);
    var p = wb.Path;
    if (typeof p === "function") { p = p.call(wb); }
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
    var wbs = app.Workbooks;
    var n = Number(wbs.Count);
    var items = [], i;
    for (i = 1; i <= n; i++) { items.push(wbs.Item(i)); }
    var tmp = "/tmp";
    try {
      var fs = app.FileSystem;
      var t = fs.tmpdir();
      if (typeof t === "function") { t = t.call(fs); }
      if (t) { tmp = String(t); }
    } catch (e) {}
    var rescued = [], closed = 0;
    for (i = 0; i < items.length; i++) {
      var wb = items[i];
      var saved = wb.Saved;
      if (typeof saved === "function") { saved = saved.call(wb); }
      if (!saved) {
        var nm = wb.Name;
        if (typeof nm === "function") { nm = nm.call(wb); }
        nm = String(nm) || "workbook.xlsx";
        var rp = tmp + "/rescued/" + nm;
        try { wb.SaveAs(rp); rescued.push(rp); } catch (e2) {}
      }
      try { wb.Close(false); closed++; } catch (e3) {}
    }
    return { closed: closed, rescued: rescued };
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
    var t = cell.Text;
    if (typeof t === "function") { t = t.call(cell); }
    var nf = cell.NumberFormat;
    if (typeof nf === "function") { nf = nf.call(cell); }
    if (Array.isArray(v)) { v = null; }
    if (Array.isArray(f)) { f = null; }
    return { v: (v === undefined ? null : v), f: (f === undefined ? null : f),
      t: (t === undefined || t === null ? "" : String(t)),
      nf: (nf === undefined || nf === null ? "General" : String(nf)) };
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
        } else if (isF && typeof info.v === "number") {
          numbers++;
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
  function wppPres(app) {
    var p = app.ActivePresentation;
    if (p === undefined || p === null) {
      throw new Error("no active presentation (open one in WPS Presentation first)");
    }
    return p;
  }

  function wppDocInfo(pres) {
    var name = pres.Name;
    if (typeof name === "function") { name = name.call(pres); }
    var full = pres.FullName;
    if (typeof full === "function") { full = full.call(pres); }
    var saved = pres.Saved;
    if (typeof saved === "function") { saved = saved.call(pres); }
    return { name: String(name), fullName: String(full), saved: !!saved };
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
    var count = Number(pres.Slides.Count);
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

  function cmdWppListSlides(app) {
    var pres = wppPres(app);
    var count = Number(pres.Slides.Count);
    var slides = [], i, j;
    for (i = 1; i <= count; i++) {
      var slide = pres.Slides.Item(i);
      var layout = slide.Layout;
      if (typeof layout === "function") { layout = layout.call(slide); }
      var texts = [];
      var n = Number(slide.Shapes.Count);
      for (j = 1; j <= n; j++) {
        var t = wppShapeText(slide.Shapes.Item(j));
        if (t) { texts.push(t); }
      }
      slides.push({ index: i, layout: Number(layout), shapeTexts: texts });
    }
    var pname = pres.Name;
    if (typeof pname === "function") { pname = pname.call(pres); }
    return { presentation: String(pname), slideCount: count, slides: slides };
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
    var nm = shape.Name;
    if (typeof nm === "function") { nm = nm.call(shape); }
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
    etPdfWrite(fs, path, buildPdf(text));
    if (!fsExists(fs, path)) { throw new Error("could not write PDF to '" + path + "'"); }
    return { exported: path };
  }

  function cmdWppSave(app) {
    var pres = wppPres(app);
    var p = pres.Path;
    if (typeof p === "function") { p = p.call(pres); }
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
    var col = app.Presentations;
    var n = Number(col.Count);
    var items = [], i;
    for (i = 1; i <= n; i++) { items.push(col.Item(i)); }
    var tmp = "/tmp";
    try {
      var fs = app.FileSystem;
      var t = fs.tmpdir();
      if (typeof t === "function") { t = t.call(fs); }
      if (t) { tmp = String(t); }
    } catch (e) {}
    var rescued = [], closed = 0;
    for (i = 0; i < items.length; i++) {
      var pres = items[i];
      var saved = pres.Saved;
      if (typeof saved === "function") { saved = saved.call(pres); }
      if (!saved) {
        var nm = pres.Name;
        if (typeof nm === "function") { nm = nm.call(pres); }
        nm = String(nm) || "presentation.pptx";
        var rp = tmp + "/rescued/" + nm;
        try { pres.SaveAs(rp); rescued.push(rp); } catch (e2) {}
      }
      // WPP KHONG nhan tham so cho `Presentation.Close`: `Close(false)` bi tu choi bang "参数太多.", va vi loi
      // bi nuot ngay day nen lenh truoc kia bao "thanh cong" ma khong dong bai nao (do 12/10/2026: hai bai dang
      // mo ma `closed` = 0). Goi khong tham so moi dung: do duoc la dong that, `Presentations.Count` ve 0. Nhanh
      // du phong cho ban WPS khac chi nhan dang co tham so — ET thi `Workbook.Close(false)` chay duoc.
      // Do tren hai bai DA LUU (`Saved` = -1); duong cuu bai chua luu o tren giu cho Close() khong gap bai ban.
      try { pres.Close(); closed++; }
      catch (e3) { try { pres.Close(false); closed++; } catch (e4) {} }
    }
    return { closed: closed, rescued: rescued };
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
        var nm = shape.Name;
        if (typeof nm === "function") { nm = nm.call(shape); }
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
    var nm = shape.Name;
    if (typeof nm === "function") { nm = nm.call(shape); }
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
    var nm = shape.Name;
    if (typeof nm === "function") { nm = nm.call(shape); }
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
    var nm = shape.Name;
    if (typeof nm === "function") { nm = nm.call(shape); }
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
    "writer.selection": cmdSelection,
    "writer.typeText": cmdTypeText,
    "writer.appendText": cmdAppendText,
    "writer.insertStyledText": cmdInsertStyledText,
    "writer.heading": cmdHeading,
    "writer.formatSelection": cmdFormatSelection,
    "writer.setParagraphAlignment": cmdSetParagraphAlignment,
    "writer.insertTable": cmdInsertTable,
    "writer.insertPageBreak": cmdInsertPageBreak,
    "writer.replaceAll": cmdReplaceAll,
    "writer.undo": cmdUndo,
    "writer.save": cmdSave,
    "writer.saveAs": cmdSaveAs,
    "writer.exportPdf": cmdExportPdf,
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
      out.push({ name: c.name, kind: c.kind, agent: c.agent, summary: c.summary, params: params });
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
        out.push({ name: c.name, kind: c.kind, agent: c.agent, summary: c.summary, params: params });
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
        try {
          var result = handler(app, p);
          return { ok: true, result: result };
        } catch (err) {
          return { ok: false, error: errMessage(err) };
        } finally {
          if (started) {
            try { undo.EndCustomRecord(); } catch (e3) {}
          }
        }
      } else {
        try {
          var res = handler(app, p);
          return { ok: true, result: res };
        } catch (err2) {
          return { ok: false, error: errMessage(err2) };
        }
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
