// SINH TU DONG tu catalog/live-commands.json bang scripts/generate_mcp_commands.py - KHONG sua tay.
package mcpserver

type liveCommandParam struct {
	Name string
	Hint string
}

type liveCommand struct {
	Name   string
	Kind   string
	Params []liveCommandParam
}

// liveCommands la danh sach lenh bridge (sinh tu catalog/live-commands.json).
var liveCommands = []liveCommand{
	{Name: "app.info", Kind: "", Params: []liveCommandParam{}},
	{Name: "ai.ask", Kind: "", Params: []liveCommandParam{{Name: "prompt", Hint: ""}}},
	{Name: "ui.askpane", Kind: "", Params: []liveCommandParam{}},
	{Name: "ui.setup", Kind: "", Params: []liveCommandParam{}},
	{Name: "app.screenshot", Kind: "", Params: []liveCommandParam{{Name: "maxWidth", Hint: ""}}},
	{Name: "writer.newDocument", Kind: "wps", Params: []liveCommandParam{}},
	{Name: "writer.open", Kind: "wps", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "writer.getText", Kind: "wps", Params: []liveCommandParam{{Name: "maxChars", Hint: ""}}},
	{Name: "writer.selection", Kind: "wps", Params: []liveCommandParam{}},
	{Name: "writer.typeText", Kind: "wps", Params: []liveCommandParam{{Name: "text", Hint: ""}}},
	{Name: "writer.appendText", Kind: "wps", Params: []liveCommandParam{{Name: "text", Hint: ""}}},
	{Name: "writer.insertStyledText", Kind: "wps", Params: []liveCommandParam{{Name: "text", Hint: ""}, {Name: "bold", Hint: ""}, {Name: "italic", Hint: ""}, {Name: "underline", Hint: ""}, {Name: "size", Hint: ""}, {Name: "color", Hint: ""}, {Name: "font", Hint: ""}}},
	{Name: "writer.heading", Kind: "wps", Params: []liveCommandParam{{Name: "text", Hint: ""}, {Name: "level", Hint: ""}, {Name: "break", Hint: ""}}},
	{Name: "writer.formatSelection", Kind: "wps", Params: []liveCommandParam{{Name: "bold", Hint: ""}, {Name: "italic", Hint: ""}, {Name: "underline", Hint: ""}, {Name: "size", Hint: ""}, {Name: "color", Hint: ""}, {Name: "font", Hint: ""}, {Name: "alignment", Hint: ""}}},
	{Name: "writer.setParagraphAlignment", Kind: "wps", Params: []liveCommandParam{{Name: "alignment", Hint: ""}}},
	{Name: "writer.insertTable", Kind: "wps", Params: []liveCommandParam{{Name: "rows", Hint: ""}, {Name: "cols", Hint: ""}, {Name: "values", Hint: "2D array of rows"}, {Name: "style", Hint: ""}}},
	{Name: "writer.formatTable", Kind: "wps", Params: []liveCommandParam{{Name: "table", Hint: "1-based index; default: table at cursor, else last table"}, {Name: "style", Hint: "e.g. 'Grid Table 4 - Accent 1'"}, {Name: "font", Hint: ""}, {Name: "size", Hint: ""}, {Name: "color", Hint: "#RRGGBB text color"}, {Name: "headerFill", Hint: "#RRGGBB"}, {Name: "headerColor", Hint: "#RRGGBB header text"}, {Name: "headerBold", Hint: ""}, {Name: "bandFill", Hint: "#RRGGBB every other data row"}, {Name: "borderColor", Hint: "#RRGGBB"}, {Name: "borders", Hint: "false = no borders (layout tables)"}, {Name: "alignment", Hint: "left/center/right"}, {Name: "autoFit", Hint: "content/window"}}},
	{Name: "writer.insertPageBreak", Kind: "wps", Params: []liveCommandParam{}},
	{Name: "writer.insertImage", Kind: "wps", Params: []liveCommandParam{{Name: "path", Hint: ""}, {Name: "width", Hint: ""}, {Name: "height", Hint: ""}}},
	{Name: "writer.insertHyperlink", Kind: "wps", Params: []liveCommandParam{{Name: "url", Hint: ""}, {Name: "text", Hint: ""}}},
	{Name: "writer.replaceAll", Kind: "wps", Params: []liveCommandParam{{Name: "find", Hint: ""}, {Name: "replace", Hint: ""}}},
	{Name: "writer.undo", Kind: "wps", Params: []liveCommandParam{{Name: "count", Hint: ""}}},
	{Name: "writer.exportPdf", Kind: "wps", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "writer.save", Kind: "wps", Params: []liveCommandParam{}},
	{Name: "writer.saveAs", Kind: "wps", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "writer.closeAll", Kind: "wps", Params: []liveCommandParam{}},
	{Name: "et.newWorkbook", Kind: "et", Params: []liveCommandParam{}},
	{Name: "et.open", Kind: "et", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "et.listSheets", Kind: "et", Params: []liveCommandParam{}},
	{Name: "et.addSheet", Kind: "et", Params: []liveCommandParam{{Name: "name", Hint: ""}, {Name: "index", Hint: "0-based, default: append"}}},
	{Name: "et.renameSheet", Kind: "et", Params: []liveCommandParam{{Name: "sheet", Hint: ""}, {Name: "name", Hint: ""}}},
	{Name: "et.activateSheet", Kind: "et", Params: []liveCommandParam{{Name: "sheet", Hint: ""}}},
	{Name: "et.readRange", Kind: "et", Params: []liveCommandParam{{Name: "range", Hint: ""}, {Name: "sheet", Hint: ""}}},
	{Name: "et.writeRange", Kind: "et", Params: []liveCommandParam{{Name: "range", Hint: "top-left cell e.g. 'A1'"}, {Name: "values", Hint: "2D array of rows e.g. [[\"Tên\",\"Điểm\"],[\"An\",9.5]]"}, {Name: "sheet", Hint: ""}}},
	{Name: "et.importCsv", Kind: "et", Params: []liveCommandParam{{Name: "path", Hint: "e.g. 'D:\\data\\raw.csv'"}, {Name: "range", Hint: "top-left cell; bỏ trống thì tạo sheet mới"}, {Name: "sheet", Hint: ""}, {Name: "delimiter", Hint: "một ký tự, mặc định ','"}, {Name: "encoding", Hint: "mặc định utf-8-sig"}}},
	{Name: "et.writeRanges", Kind: "et", Params: []liveCommandParam{{Name: "writes", Hint: "array of {range, values, sheet?}"}, {Name: "sheet", Hint: ""}}},
	{Name: "et.fillRange", Kind: "et", Params: []liveCommandParam{{Name: "range", Hint: "the whole area e.g. 'B2:H1000'"}, {Name: "formula", Hint: "written to the top-left cell first"}, {Name: "sheet", Hint: ""}}},
	{Name: "et.formatRange", Kind: "et", Params: []liveCommandParam{{Name: "range", Hint: ""}, {Name: "bold", Hint: ""}, {Name: "italic", Hint: ""}, {Name: "fontSize", Hint: ""}, {Name: "fontColor", Hint: ""}, {Name: "fillColor", Hint: ""}, {Name: "numFmt", Hint: ""}, {Name: "horizontal", Hint: ""}, {Name: "wrap", Hint: ""}, {Name: "sheet", Hint: ""}}},
	{Name: "et.setConditionalFormat", Kind: "et", Params: []liveCommandParam{{Name: "range", Hint: ""}, {Name: "rules", Hint: "array of rule objects"}, {Name: "sheet", Hint: ""}}},
	{Name: "et.listConditionalFormats", Kind: "et", Params: []liveCommandParam{{Name: "range", Hint: "chỉ xem một vùng"}, {Name: "sheet", Hint: ""}}},
	{Name: "et.addChart", Kind: "et", Params: []liveCommandParam{{Name: "range", Hint: "source data e.g. 'A1:B13'"}, {Name: "type", Hint: "column (default)/bar/line/pie/area/scatter"}, {Name: "title", Hint: ""}, {Name: "name", Hint: ""}, {Name: "anchor", Hint: "top-left cell, default 'A1'"}, {Name: "width", Hint: "cm, default 12"}, {Name: "height", Hint: "cm, default 7"}, {Name: "sheet", Hint: ""}}},
	{Name: "et.listCharts", Kind: "et", Params: []liveCommandParam{{Name: "sheet", Hint: ""}}},
	{Name: "et.undo", Kind: "et", Params: []liveCommandParam{{Name: "count", Hint: ""}}},
	{Name: "et.exportPdf", Kind: "et", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "et.save", Kind: "et", Params: []liveCommandParam{}},
	{Name: "et.saveAs", Kind: "et", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "et.closeAll", Kind: "et", Params: []liveCommandParam{}},
	{Name: "et.checkRange", Kind: "et", Params: []liveCommandParam{{Name: "range", Hint: "default: the used range"}, {Name: "sheet", Hint: ""}}},
	{Name: "wpp.newPresentation", Kind: "wpp", Params: []liveCommandParam{}},
	{Name: "wpp.open", Kind: "wpp", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "wpp.listSlides", Kind: "wpp", Params: []liveCommandParam{}},
	{Name: "wpp.addSlide", Kind: "wpp", Params: []liveCommandParam{{Name: "layout", Hint: "number: 1 title, 2 title+content, 11 title only, 12 blank"}}},
	{Name: "wpp.addText", Kind: "wpp", Params: []liveCommandParam{{Name: "text", Hint: ""}, {Name: "slide", Hint: ""}, {Name: "left", Hint: ""}, {Name: "top", Hint: ""}, {Name: "width", Hint: ""}, {Name: "height", Hint: ""}, {Name: "fontSize", Hint: ""}, {Name: "bold", Hint: ""}, {Name: "color", Hint: ""}, {Name: "align", Hint: ""}}},
	{Name: "wpp.addTextBox", Kind: "wpp", Params: []liveCommandParam{{Name: "text", Hint: ""}, {Name: "slide", Hint: ""}, {Name: "left", Hint: ""}, {Name: "top", Hint: ""}, {Name: "width", Hint: ""}, {Name: "height", Hint: ""}}},
	{Name: "wpp.addImage", Kind: "wpp", Params: []liveCommandParam{{Name: "path", Hint: ""}, {Name: "slide", Hint: ""}, {Name: "left", Hint: ""}, {Name: "top", Hint: ""}, {Name: "width", Hint: ""}, {Name: "height", Hint: ""}}},
	{Name: "wpp.addTable", Kind: "wpp", Params: []liveCommandParam{{Name: "rows", Hint: ""}, {Name: "cols", Hint: ""}, {Name: "values", Hint: "2D array of rows"}, {Name: "slide", Hint: ""}, {Name: "left", Hint: ""}, {Name: "top", Hint: ""}, {Name: "width", Hint: ""}, {Name: "height", Hint: ""}}},
	{Name: "wpp.setNotes", Kind: "wpp", Params: []liveCommandParam{{Name: "text", Hint: ""}, {Name: "slide", Hint: ""}}},
	{Name: "wpp.deleteSlide", Kind: "wpp", Params: []liveCommandParam{{Name: "slide", Hint: ""}}},
	{Name: "wpp.exportPdf", Kind: "wpp", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "wpp.save", Kind: "wpp", Params: []liveCommandParam{}},
	{Name: "wpp.saveAs", Kind: "wpp", Params: []liveCommandParam{{Name: "path", Hint: ""}}},
	{Name: "wpp.closeAll", Kind: "wpp", Params: []liveCommandParam{}},
	{Name: "wpp.checkLayout", Kind: "wpp", Params: []liveCommandParam{{Name: "slide", Hint: "slide number; default: every slide"}}},
	{Name: "writer.checkTables", Kind: "wps", Params: []liveCommandParam{}},
}
