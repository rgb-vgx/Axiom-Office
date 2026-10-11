/*
 * Axiom Office - WPS JS add-in command registry + handlers.
 *
 * Global AxiomCommands { catalog(), execute(app, action, params) }.
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

  var CATALOG = [
    { name: "app.info", kind: null, agent: false,
      summary: "Tên/version app, tài liệu đang mở, `state` (tài liệu, cửa sổ, visible)",
      params: [] },
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
    { name: "writer.insertPageBreak", kind: "wps", agent: true,
      summary: "Ngắt trang",
      params: [] },
    { name: "writer.replaceAll", kind: "wps", agent: true,
      summary: "Tìm và thay toàn bộ",
      params: [
        { name: "find", required: true, hint: null },
        { name: "replace", required: false, hint: null }
      ] },
    { name: "writer.undo", kind: "wps", agent: true,
      summary: "Hoàn tác (mỗi thao tác AI = 1 bước)",
      params: [{ name: "count", required: false, hint: null }] },
    { name: "writer.save", kind: "wps", agent: true,
      summary: "Lưu",
      params: [] },
    { name: "writer.saveAs", kind: "wps", agent: true,
      summary: "Lưu thành file mới",
      params: [{ name: "path", required: true, hint: null }] },
    { name: "writer.exportPdf", kind: "wps", agent: true,
      summary: "Xuất PDF",
      params: [{ name: "path", required: true, hint: null }] }
  ];

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
      var doc = app.ActiveDocument;
      if (doc !== undefined && doc !== null) {
        var dname = "", dfull = "";
        try { dname = strProp(doc, "Name"); } catch (e5) {}
        try { dfull = strProp(doc, "FullName"); } catch (e6) {}
        document = { name: dname, fullName: dfull };
      }
    } catch (e7) { document = null; }
    return { app: "wps", name: name, version: version, document: document };
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
    "writer.exportPdf": cmdExportPdf
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

  function execute(app, action, params) {
    try {
      if (app === undefined || app === null) {
        return { ok: false, error: "no Application object (open a document in WPS Writer first)" };
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

  var api = { catalog: catalog, execute: execute };
  try {
    if (root) { root.AxiomCommands = api; }
  } catch (e) {}
  try {
    if (typeof module !== "undefined" && module && module.exports) {
      module.exports = api;
    }
  } catch (e2) {}
})(typeof window !== "undefined" ? window : (typeof globalThis !== "undefined" ? globalThis : this));
