package mcpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/office"
)

// Lan live: gui lenh toi bridge trong Word/Excel/PowerPoint hoac WPS dang mo.
// Loi ket noi tra {"ok":false,"error":...} nhu mot ket qua binh thuong (khong phai isError),
// giong ban C#/Python. Port cua LiveTools.cs + BridgeClient.cs.

// portTip nang nguoi dung tu tim bridge dang mo khi khong ro cong.
const portTip = " Tip: call office_sessions() to list every live bridge (Office + WPS) and pass its port here to target that exact instance."

// commandTimeout: ai.ask co the chay toi 5 phut nen tran phai rong hon mac dinh cua Core.
const commandTimeout = 330 * time.Second

// liveApps anh xa ten app nguoi dung go -> (ho ung dung, co phai Microsoft Office khong).
var liveApps = map[string]struct {
	kind       string
	officeHost bool
}{
	"wps": {"wps", false}, "et": {"et", false}, "wpp": {"wpp", false},
	"word": {"wps", true}, "excel": {"et", true}, "ppt": {"wpp", true}, "powerpoint": {"wpp", true},
}

// liveHTTPClient dung chung de tai su dung ket noi; token doc lai moi lan goi vi nguoi dung co the
// doi cau hinh trong luc server dang chay.
var liveHTTPClient = &http.Client{}

func bridgeClient() *office.BridgeClient {
	client := office.NewBridgeClient(liveHTTPClient, config.Load().Token)
	client.CommandTimeout = commandTimeout
	return client
}

// resolvePort: cong nguoi dung truyen thang, khong thi tra theo ten app trong cau hinh.
func resolvePort(app string, port int, hasPort bool) (int, error) {
	if hasPort && port > 0 {
		return port, nil
	}
	target, ok := liveApps[strings.ToLower(app)]
	if !ok {
		return 0, errUnknownApp(app)
	}
	guessed := config.BridgePortForKind(config.DefaultSource(), target.kind, target.officeHost)

	// Session registry la su THAT (bridge nao dang chay), cau hinh chi la phong doan theo ten app. Tren
	// Windows ten app khong noi duoc ho nao: "et"/"wps"/"wpp" luon tra ve cong WPS, nen bridge
	// LibreOffice dang chay (47852) khong bao gio khop. Do ngay 02/10/2026: wps_live_write_range tren
	// Windows tro vao 47822 trong khi LibreOffice o 47852, va chinh office_sessions bao dung 47852 -
	// hai tool cua cung mot server noi nguoc nhau.
	//
	// Giu nguyen hanh vi cu khi cau hinh dung: co session o dung cong do thi dung cong do. Chi khi cau
	// hinh tro vao mot cong KHONG co bridge nao thi moi lay cong cua session dang chay.
	ports := office.NewDirectory("").Ports(target.kind)
	if len(ports) == 0 {
		return guessed, nil
	}
	for _, port := range ports {
		if port == guessed {
			return guessed, nil
		}
	}
	return ports[0], nil
}

type unknownAppError struct{ app string }

func (e unknownAppError) Error() string {
	return "unknown app '" + e.app + "' (expected wps/et/wpp for WPS or word/excel/ppt for Microsoft Office, or pass port)"
}

func errUnknownApp(app string) error { return unknownAppError{app: app} }

// safe doi moi loi thanh ket qua {"ok":false,"error":...} - KHONG phai loi MCP.
func safe(action func() (string, error)) (any, error) {
	text, err := action()
	if err != nil {
		return toolErrorBody(err.Error()), nil
	}
	return text, nil
}

// live gui mot lenh bridge. Cac tham so duoc danh gia o phia goi giong ban C#, nen thieu tham so
// bat buoc van la loi tool (isError) chu khong bi bien thanh {"ok":false}.
func live(a *ToolArgs, defaultApp, action string, params map[string]any, port int, hasPort bool) (any, error) {
	return liveLimited(a, defaultApp, action, params, port, hasPort, mcpTruncation)
}

// liveLimited: nhu live nhung cat et.readRange / et.checkRange lon theo gioi han cho truoc. Client MCP
// cung la LLM: doc 50.001 dong la 1 MB do thang vao ngu canh cua no (thong le cua cac server Excel
// MCP: phan trang co gioi han, vd excel-mcp-server 4.000 o/lan). Lenh khac va lenh loi: nguyen van.
func liveLimited(a *ToolArgs, defaultApp, action string, params map[string]any, port int, hasPort bool,
	limits office.Truncation) (any, error) {
	return safe(func() (string, error) {
		resolved, err := resolvePort(a.Str("app", defaultApp), port, hasPort)
		if err != nil {
			return "", err
		}
		raw := bridgeClient().CommandRaw(context.Background(), resolved, action, params)
		return limits.ApplyRaw(action, raw), nil
	})
}

const (
	// maxCellsCeiling: tran cung khi client xin doc nhieu hon (max_cells). ~20.000 o ~ 130 KB ~ 35k
	// token - da qua muc mac dinh 25k token cua Claude Code, nen chi mo khi client chu dong xin.
	maxCellsCeiling = 20000
	// truncationHint gan vao ket qua bi cat. KHONG goi y saveAs .csv: lenh do luu DE tai lieu dang mo
	// thanh file CSV (con mot sheet) chu khong xuat ban sao - goi y no la dan client lam hong tai lieu.
	truncationHint = "Only part of the range was returned (truncated=true): values holds the first rows " +
		"and tailValues the last rows. Do not draw conclusions about the omitted rows from this sample; " +
		"read nextRange in bounded chunks, or pass max_cells to wps_live_read_range to get more per call."
)

var mcpTruncation = func() office.Truncation {
	limits := office.DefaultTruncation
	limits.Hint = truncationHint
	return limits
}()

// readTruncation: gioi han cho wps_live_read_range - max_cells (neu co) nang ca so dong lan so o,
// bi chan o maxCellsCeiling; khong co thi dung mac dinh.
func readTruncation(a *ToolArgs) office.Truncation {
	cells, ok := a.IntOpt("max_cells")
	if !ok || cells <= 0 {
		return mcpTruncation
	}
	cells = min(cells, maxCellsCeiling)
	limits := mcpTruncation
	limits.HeadRows, limits.CellBudget = cells, cells
	return limits
}

func liveHealth(a *ToolArgs, defaultApp string) (any, error) {
	port, hasPort := a.IntOpt("port")
	return safe(func() (string, error) {
		resolved, err := resolvePort(a.Str("app", defaultApp), port, hasPort)
		if err != nil {
			return "", err
		}
		return bridgeClient().HealthRaw(context.Background(), resolved), nil
	})
}

func appParam(def string) Param {
	return ParamStr("app", "wps/et/wpp (WPS) or word/excel/ppt (Microsoft Office)", false, def)
}

func portParam() Param {
	return ParamInt("port", "explicit bridge port (from office_sessions)", false, nil)
}

// commandSignature dinh dang "et.readRange {range,sheet}" - giong CommandInfo.Signature cua ban C#.
func commandSignature(command liveCommand) string {
	params := make([]string, 0, len(command.Params))
	for _, param := range command.Params {
		if param.Hint == "" {
			params = append(params, param.Name)
			continue
		}
		params = append(params, param.Name+" ("+param.Hint+")")
	}
	return command.Name + " {" + strings.Join(params, ",") + "}"
}

// commandList liet ke moi lenh cua cac ho duoc hoi (kind "" = dung cho moi app).
func commandList(kinds ...string) string {
	signatures := []string{}
	for _, command := range liveCommands {
		if command.Kind == "" || containsString(kinds, command.Kind) {
			signatures = append(signatures, commandSignature(command))
		}
	}
	return " Commands {params}: " + strings.Join(signatures, "; ") + "."
}

// wordLiveTools la cac tool live cua Word/WPS Writer.
func wordLiveTools() []*Tool {
	return []*Tool{
		NewTool("word_health",
			"Check the live Word bridge. app: word (Microsoft Word, 47831) or wps (WPS Writer, 47821).",
			[]Param{appParam("word"), portParam()},
			func(a *ToolArgs) (any, error) { return liveHealth(a, "word") }),
		NewTool("word_command",
			"Send any bridge command to the live Word/WPS Writer session. Examples: writer.getText; writer.undo; writer.replaceAll params={'find':'a','replace':'b'}; writer.exportPdf params={'path':'C:/tmp/out.pdf'}."+commandList("wps")+portTip,
			[]Param{ParamStr("action", "", true, nil), ParamObj("params", "", false), appParam("word"), portParam()},
			func(a *ToolArgs) (any, error) {
				port, hasPort := a.IntOpt("port")
				return live(a, "word", a.Req("action"), a.Obj("params"), port, hasPort)
			}),
		NewTool("word_read_text",
			"Read the full text of the document currently open in Word/WPS.",
			[]Param{appParam("word"), ParamInt("max_chars", "", false, 0)},
			func(a *ToolArgs) (any, error) {
				var limit any
				if maxChars := a.Int("max_chars", 0); maxChars > 0 {
					limit = maxChars
				}
				port, hasPort := a.IntOpt("port")
				return live(a, "word", "writer.getText", Clean("maxChars", limit), port, hasPort)
			}),
		NewTool("word_type_text",
			"Type text at the current cursor position in the open document.",
			[]Param{ParamStr("text", "", true, nil), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.typeText", Clean("text", a.Req("text")), 0, false)
			}),
		NewTool("word_insert_styled_text",
			"Insert text with formatting (color as '#RRGGBB'). Each call is one Ctrl+Z step.",
			[]Param{ParamStr("text", "", true, nil), ParamBool("bold", "", false, nil), ParamBool("italic", "", false, nil), ParamBool("underline", "", false, nil), ParamInt("size", "", false, nil), ParamStr("color", "", false, nil), ParamStr("font", "", false, nil), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.insertStyledText", Clean(
					"text", a.Req("text"),
					"bold", a.BoolOrNil("bold"), "italic", a.BoolOrNil("italic"), "underline", a.BoolOrNil("underline"),
					"size", a.IntOrNil("size"), "color", a.StrOrNil("color"), "font", a.StrOrNil("font")), 0, false)
			}),
		NewTool("word_format_selection",
			"Format the currently selected text (alignment: left/center/right/justify).",
			[]Param{ParamBool("bold", "", false, nil), ParamBool("italic", "", false, nil), ParamBool("underline", "", false, nil), ParamInt("size", "", false, nil), ParamStr("color", "", false, nil), ParamStr("font", "", false, nil), ParamStr("alignment", "", false, nil), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.formatSelection", Clean(
					"bold", a.BoolOrNil("bold"), "italic", a.BoolOrNil("italic"), "underline", a.BoolOrNil("underline"),
					"size", a.IntOrNil("size"), "color", a.StrOrNil("color"), "font", a.StrOrNil("font"),
					"alignment", a.StrOrNil("alignment")), 0, false)
			}),
		NewTool("word_heading",
			"Insert a heading (Heading 1-9 style) with an automatic paragraph break.",
			[]Param{ParamStr("text", "", false, nil), ParamInt("level", "", false, 1), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.heading", Clean("text", a.StrOrNil("text"), "level", a.Int("level", 1)), 0, false)
			}),
		NewTool("word_insert_table",
			"Insert a table at the cursor with optional 2D values and an optional table style (e.g. 'Table Grid').",
			[]Param{ParamInt("rows", "", true, nil), ParamInt("cols", "", true, nil), ParamMatrix("values", "", false), ParamStr("style", "", false, nil), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.insertTable", Clean(
					"rows", a.Int("rows", 0), "cols", a.Int("cols", 0),
					"values", a.Raw("values"), "style", a.StrOrNil("style")), 0, false)
			}),
		NewTool("word_insert_image",
			"Insert an image at the cursor (path to png/jpg; width/height in points).",
			[]Param{ParamStr("path", "", true, nil), ParamInt("width", "", false, nil), ParamInt("height", "", false, nil), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.insertImage", Clean(
					"path", a.Req("path"), "width", a.IntOrNil("width"), "height", a.IntOrNil("height")), 0, false)
			}),
		NewTool("word_insert_hyperlink",
			"Insert a hyperlink at the cursor.",
			[]Param{ParamStr("url", "", true, nil), ParamStr("text", "", false, nil), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.insertHyperlink", Clean("url", a.Req("url"), "text", a.StrOrNil("text")), 0, false)
			}),
		NewTool("word_replace_all",
			"Find and replace all occurrences in the open document (one undo step).",
			[]Param{ParamStr("find", "", true, nil), ParamStr("replace", "", false, ""), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.replaceAll", Clean("find", a.Req("find"), "replace", a.Str("replace", "")), 0, false)
			}),
		NewTool("word_export_pdf",
			"Export the open document to PDF.",
			[]Param{ParamStr("path", "", true, nil), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.exportPdf", Clean("path", a.Req("path")), 0, false)
			}),
		NewTool("word_undo",
			"Undo the last N AI actions (each AI action is a single undo record).",
			[]Param{ParamInt("count", "", false, 1), appParam("word")},
			func(a *ToolArgs) (any, error) {
				return live(a, "word", "writer.undo", Clean("count", a.Int("count", 1)), 0, false)
			}),
		NewTool("word_save",
			"Save the open document (optionally to a new path).",
			[]Param{ParamStr("path", "", false, nil), appParam("word")},
			func(a *ToolArgs) (any, error) {
				if a.Has("path") {
					return live(a, "word", "writer.saveAs", Clean("path", a.StrOrNil("path")), 0, false)
				}
				return live(a, "word", "writer.save", nil, 0, false)
			}),
	}
}

// pptLiveTools la cac tool live cua PowerPoint/WPS Presentation.
func pptLiveTools() []*Tool {
	return []*Tool{
		NewTool("ppt_health",
			"Check the live PowerPoint bridge. app: ppt (Microsoft PowerPoint, 47833) or wpp (WPS Presentation, 47823).",
			[]Param{appParam("ppt"), portParam()},
			func(a *ToolArgs) (any, error) { return liveHealth(a, "ppt") }),
		NewTool("ppt_command",
			"Send any bridge command to the live PowerPoint/WPS session. Examples: wpp.listSlides; wpp.exportPdf params={'path':'C:/tmp/out.pdf'}; wpp.saveAs params={'path':'C:/tmp/out.pptx'}."+commandList("wpp")+portTip,
			[]Param{ParamStr("action", "", true, nil), ParamObj("params", "", false), appParam("ppt"), portParam()},
			func(a *ToolArgs) (any, error) {
				port, hasPort := a.IntOpt("port")
				return live(a, "ppt", a.Req("action"), a.Obj("params"), port, hasPort)
			}),
		NewTool("ppt_list_slides",
			"List slides (count + texts) of the presentation currently open in PowerPoint/WPS.",
			[]Param{appParam("ppt")},
			func(a *ToolArgs) (any, error) { return live(a, "ppt", "wpp.listSlides", nil, 0, false) }),
		NewTool("ppt_add_slide",
			"Add a slide to the live presentation. layout: 1=title, 2=title+text, 12=blank (default).",
			[]Param{ParamInt("layout", "", false, 12), appParam("ppt")},
			func(a *ToolArgs) (any, error) {
				return live(a, "ppt", "wpp.addSlide", Clean("layout", a.Int("layout", 12)), 0, false)
			}),
		NewTool("ppt_add_text",
			"Add a formatted text box to a live slide (color '#RRGGBB', align left/center/right).",
			[]Param{ParamStr("text", "", true, nil), ParamInt("slide", "", false, nil), ParamInt("left", "", false, nil), ParamInt("top", "", false, nil), ParamInt("width", "", false, nil), ParamInt("height", "", false, nil),
				ParamInt("font_size", "", false, nil), ParamBool("bold", "", false, nil), ParamStr("color", "", false, nil), ParamStr("align", "", false, nil), appParam("ppt")},
			func(a *ToolArgs) (any, error) {
				return live(a, "ppt", "wpp.addText", Clean(
					"text", a.Req("text"), "slide", a.IntOrNil("slide"), "left", a.IntOrNil("left"), "top", a.IntOrNil("top"),
					"width", a.IntOrNil("width"), "height", a.IntOrNil("height"), "fontSize", a.IntOrNil("font_size"),
					"bold", a.BoolOrNil("bold"), "color", a.StrOrNil("color"), "align", a.StrOrNil("align")), 0, false)
			}),
		NewTool("ppt_add_image",
			"Insert an image into a live slide (natural size when width/height omitted).",
			[]Param{ParamStr("path", "", true, nil), ParamInt("slide", "", false, nil), ParamInt("left", "", false, nil), ParamInt("top", "", false, nil), ParamInt("width", "", false, nil), ParamInt("height", "", false, nil), appParam("ppt")},
			func(a *ToolArgs) (any, error) {
				return live(a, "ppt", "wpp.addImage", Clean(
					"path", a.Req("path"), "slide", a.IntOrNil("slide"), "left", a.IntOrNil("left"), "top", a.IntOrNil("top"),
					"width", a.IntOrNil("width"), "height", a.IntOrNil("height")), 0, false)
			}),
		NewTool("ppt_add_table",
			"Add a table with optional 2D values to a live slide.",
			[]Param{ParamInt("rows", "", true, nil), ParamInt("cols", "", true, nil), ParamMatrix("values", "", false), ParamInt("slide", "", false, nil), ParamInt("left", "", false, nil), ParamInt("top", "", false, nil),
				ParamInt("width", "", false, nil), ParamInt("height", "", false, nil), appParam("ppt")},
			func(a *ToolArgs) (any, error) {
				return live(a, "ppt", "wpp.addTable", Clean(
					"rows", a.Int("rows", 0), "cols", a.Int("cols", 0), "values", a.Raw("values"), "slide", a.IntOrNil("slide"),
					"left", a.IntOrNil("left"), "top", a.IntOrNil("top"), "width", a.IntOrNil("width"), "height", a.IntOrNil("height")), 0, false)
			}),
		NewTool("ppt_set_notes",
			"Set speaker notes of a live slide.",
			[]Param{ParamStr("text", "", true, nil), ParamInt("slide", "", false, nil), appParam("ppt")},
			func(a *ToolArgs) (any, error) {
				return live(a, "ppt", "wpp.setNotes", Clean("text", a.Req("text"), "slide", a.IntOrNil("slide")), 0, false)
			}),
		NewTool("ppt_delete_slide",
			"Delete a slide from the live presentation (default: last slide).",
			[]Param{ParamInt("slide", "", false, nil), appParam("ppt")},
			func(a *ToolArgs) (any, error) {
				return live(a, "ppt", "wpp.deleteSlide", Clean("slide", a.IntOrNil("slide")), 0, false)
			}),
		NewTool("ppt_export_pdf",
			"Export the live presentation to PDF.",
			[]Param{ParamStr("path", "", true, nil), appParam("ppt")},
			func(a *ToolArgs) (any, error) {
				return live(a, "ppt", "wpp.exportPdf", Clean("path", a.Req("path")), 0, false)
			}),
		NewTool("ppt_save",
			"Save the live presentation (optionally to a new path).",
			[]Param{ParamStr("path", "", false, nil), appParam("ppt")},
			func(a *ToolArgs) (any, error) {
				if a.Has("path") {
					return live(a, "ppt", "wpp.saveAs", Clean("path", a.StrOrNil("path")), 0, false)
				}
				return live(a, "ppt", "wpp.save", nil, 0, false)
			}),
	}
}

// excelLiveTools la cac tool live cua bang tinh (WPS et / Microsoft Excel).
func excelLiveTools() []*Tool {
	return []*Tool{
		NewTool("wps_health",
			"Check the live bridge. app: wps/et/wpp (WPS Office) or word/excel/ppt (Microsoft Office); or pass an explicit port.",
			[]Param{appParam("wps"), portParam()},
			func(a *ToolArgs) (any, error) { return liveHealth(a, "wps") }),
		NewTool("wps_live_command",
			"Send any command to the live bridge (works on the document the user has open). Apps: wps=47821, et=47822, wpp=47823 (WPS); word=47831, excel=47832, ppt=47833 (Microsoft Office). "+
				"Examples: action='app.info'; action='writer.insertStyledText' params={'text':'hello','bold':true,'color':'#FF0000'}; action='writer.heading' params={'level':1,'text':'Title'}; "+
				"action='writer.insertTable' params={'rows':2,'cols':2,'values':[[1,2],[3,4]]}; action='writer.exportPdf' params={'path':'C:/tmp/out.pdf'}; "+
				"action='et.formatRange' params={'range':'A1:B1','bold':true,'fillColor':'#FFFF00'}; action='et.readRange' params={'range':'A1:C10'}; "+
				"action='et.writeRange' params={'range':'A1','values':[[1,2],[3,4]]}; action='wpp.addSlide' params={'layout':1}; "+
				"action='wpp.addText' params={'text':'Hi','fontSize':28,'color':'#FF0000'}; action='wpp.addTable' params={'rows':2,'cols':3}; "+
				"action='wpp.setNotes' params={'text':'notes'}; action='writer.undo' OR action='et.undo'. "+
				"Large et.readRange / et.checkRange results are truncated (truncated=true, nextRange, issueCount "+
				"stays exact); use wps_live_read_range with max_cells to get more rows per call."+
				commandList("wps", "et", "wpp")+portTip,
			[]Param{ParamStr("app", "wps/et/wpp or word/excel/ppt", true, nil), ParamStr("action", "", true, nil), ParamObj("params", "", false), portParam()},
			func(a *ToolArgs) (any, error) {
				port, hasPort := a.IntOpt("port")
				return live(a, a.Req("app"), a.Req("action"), a.Obj("params"), port, hasPort)
			}),
		NewTool("wps_live_read_range",
			"Read a range from the spreadsheet currently open in WPS (live). cell_range like 'A1:F100'. "+
				"Large ranges come back truncated: truncated=true, the first rows in values, the last rows in "+
				"tailValues, totalRows/totalCols for the real size and nextRange for the part not returned "+
				"(read it in bounded chunks). Default limit 200 rows / 6000 cells per call; max_cells raises it "+
				"(up to 20000).",
			[]Param{ParamStr("cell_range", "", true, nil), appParam("et"), ParamStr("sheet", "", false, nil),
				ParamInt("max_cells", "cells to return per call before truncating (default 6000, max 20000)", false, nil)},
			func(a *ToolArgs) (any, error) {
				return liveLimited(a, "et", "et.readRange",
					Clean("range", a.Req("cell_range"), "sheet", a.StrOrNil("sheet")), 0, false, readTruncation(a))
			}),
		NewTool("wps_live_write_range",
			"Write a 2D block into the spreadsheet currently open in WPS (live). cell_range is the top-left anchor like 'A1'.",
			[]Param{ParamStr("cell_range", "", true, nil), ParamMatrix("values", "", true), appParam("et"), ParamStr("sheet", "", false, nil)},
			func(a *ToolArgs) (any, error) {
				return live(a, "et", "et.writeRange", Clean(
					"range", a.Req("cell_range"), "values", a.Raw("values"), "sheet", a.StrOrNil("sheet")), 0, false)
			}),
	}
}

// sessionsTool liet ke moi bridge dang mo (Office + WPS).
func sessionsTool() *Tool {
	return NewTool("office_sessions",
		"List all live Office/WPS bridge sessions: app, port, host, open document.",
		[]Param{},
		func(a *ToolArgs) (any, error) {
			return safe(func() (string, error) {
				directory := office.NewDirectory("")
				return encode(bridgeClient().Sessions(directory, true)), nil
			})
		})
}
