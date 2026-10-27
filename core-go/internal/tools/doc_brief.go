package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DocFromBriefTool: dung van ban Word theo LOAI (to trinh, cong van, thong bao, bien ban, bao cao, ke hoach) dung the
// thuc NĐ 30/2020 (docs/plans/21-v0 P3). Model chi khai bao thong tin va noi dung; tool dung phan dau (bang dan trang:
// co quan | quoc hieu, tieu ngu, ngay thang), ten loai + trich yeu, Kinh gui, noi dung (writer.writeContent), phan ky
// (noi nhan | chuc vu, ho ten), so trang / muc luc, roi soat phan trang. Truoc day model yeu tu dung tung dong: bo do
// 26/10/2026 to trinh nua trang mat 33 vong, phan dau thieu dam/nghieng dung cho.
type DocFromBriefTool struct{}

func NewDocFromBriefTool() *DocFromBriefTool { return &DocFromBriefTool{} }

func (t *DocFromBriefTool) Name() string { return "doc_from_brief" }

// docKinds: loai van ban -> ten loai in tren van ban ("" = cong van: khong co ten loai, trich yeu "V/v" o cot trai).
var docKinds = map[string]string{
	"to-trinh": "TỜ TRÌNH", "cong-van": "", "thong-bao": "THÔNG BÁO", "bien-ban": "BIÊN BẢN",
	"bao-cao": "BÁO CÁO", "ke-hoach": "KẾ HOẠCH",
}

// docLongKinds: van ban dai - mac dinh so trang o chan trang.
var docLongKinds = map[string]bool{"bao-cao": true, "ke-hoach": true}

// docBlockKeys: khoi noi dung - cung hop dong voi writer.writeContent.
var docBlockKeys = []string{"heading", "text", "bullets", "numbers", "table", "pageBreak"}

const docPlaceholder = "…"

func (t *DocFromBriefTool) Description() string {
	return "Create a NEW Vietnamese administrative document in the open, EMPTY Word document, in the official format " +
		"(Decree 30/2020): kind to-trinh (tờ trình), cong-van (công văn), thong-bao, bien-ban (biên bản họp), bao-cao, " +
		"ke-hoach. You give the facts and the content; the tool builds the header (agency | national motto, number, " +
		"place and date), the document name and subject, \"Kính gửi\", the body, the recipients list and the signature, " +
		"then checks the page layout. Fields: kind, agency {parent, name}, number (e.g. \"12/TTr-THPT\"), place, date " +
		"(\"20/10/2026\"), subject (trích yếu, without \"Về việc\"/\"V/v\"), to (Kính gửi: list), body (blocks in order, " +
		"same as writer.writeContent: {heading, level} | {text} ('\\n' = new paragraph) | {bullets: [..]} | {numbers: [..]} " +
		"| {table: [[header..], [row..]], caption} | {pageBreak: true}), signer {position, name, onBehalf e.g. \"KT. HIỆU " +
		"TRƯỞNG\"}, recipients (Nơi nhận, e.g. [\"Như trên\", \"Lưu: VT\"]); bien-ban also meeting {time, place, chair, " +
		"secretary, attendees}; bao-cao/ke-hoach may add toc: true; maxPages = the page limit the user gave. Leave out " +
		"what the user did not give - the tool writes \"…\" there and lists it under placeholders; never invent names, " +
		"numbers or dates. Use this instead of building the header and signature tables yourself."
}

func (t *DocFromBriefTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"kind":{"type":"string","enum":["to-trinh","cong-van","thong-bao","bien-ban","bao-cao","ke-hoach"]},` +
		`"agency":{"type":"object","properties":{"parent":{"type":"string"},"name":{"type":"string"}}},` +
		`"number":{"type":"string"},"place":{"type":"string"},"date":{"type":"string"},"subject":{"type":"string"},` +
		`"to":{"type":"array","items":{"type":"string"}},` +
		`"meeting":{"type":"object","properties":{"time":{"type":"string"},"place":{"type":"string"},"chair":{"type":"string"},` +
		`"secretary":{"type":"string"},"attendees":{"type":"string"}}},` +
		`"body":{"type":"array","items":{"type":"object"}},` +
		`"signer":{"type":"object","properties":{"position":{"type":"string"},"name":{"type":"string"},"onBehalf":{"type":"string"}}},` +
		`"recipients":{"type":"array","items":{"type":"string"}},` +
		`"toc":{"type":"boolean"},"pageNumber":{"type":"boolean"},"maxPages":{"type":"integer"}},` +
		`"required":["kind","body"]}`)
}

// docBrief: brief da doc. Truong trong = nguoi dung chua cho -> "…" + ghi vao placeholders.
type docBrief struct {
	Kind, Parent, Agency, Number, Place, Date, Subject string
	To, Recipients                                     []string
	Meeting                                            map[string]string
	Body                                               []any
	Position, Signer, OnBehalf                         string
	TOC, PageNumber                                    bool
	MaxPages                                           int
	placeholders                                       []string
}

func stringField(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func briefLines(value any) []string {
	list := []string{}
	switch typed := value.(type) {
	case string:
		for _, line := range strings.Split(typed, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list = append(list, line)
			}
		}
	case []any:
		for _, item := range typed {
			if text := stringField(item); text != "" {
				list = append(list, text)
			}
		}
	}
	return list
}

func parseDocBrief(arguments map[string]any) (docBrief, []string) {
	brief := docBrief{Kind: strings.ToLower(stringField(arguments["kind"]))}
	problems := []string{}
	if _, ok := docKinds[brief.Kind]; !ok {
		problems = append(problems, fmt.Sprintf("kind must be one of to-trinh, cong-van, thong-bao, bien-ban, bao-cao, ke-hoach (got %q)", brief.Kind))
	}
	agency, _ := arguments["agency"].(map[string]any)
	brief.Parent, brief.Agency = stringField(agency["parent"]), stringField(agency["name"])
	brief.Number, brief.Place, brief.Date = stringField(arguments["number"]), stringField(arguments["place"]), stringField(arguments["date"])
	brief.Subject = stringField(arguments["subject"])
	brief.To, brief.Recipients = briefLines(arguments["to"]), briefLines(arguments["recipients"])
	meeting, _ := arguments["meeting"].(map[string]any)
	brief.Meeting = map[string]string{}
	for _, key := range []string{"time", "place", "chair", "secretary", "attendees"} {
		brief.Meeting[key] = stringField(meeting[key])
	}
	signer, _ := arguments["signer"].(map[string]any)
	brief.Position, brief.Signer, brief.OnBehalf = stringField(signer["position"]), stringField(signer["name"]), stringField(signer["onBehalf"])
	brief.TOC, _ = arguments["toc"].(bool)
	brief.PageNumber = docLongKinds[brief.Kind]
	if flag, ok := arguments["pageNumber"].(bool); ok {
		brief.PageNumber = flag
	}
	brief.MaxPages = asInt(arguments["maxPages"])
	brief.Body, _ = arguments["body"].([]any)
	if len(brief.Body) == 0 {
		problems = append(problems, "body is empty: give the content as blocks, e.g. [{\"text\": \"...\"}]")
	}
	for i, item := range brief.Body {
		block, _ := item.(map[string]any)
		if block == nil {
			problems = append(problems, fmt.Sprintf("body[%d] is not an object", i))
			continue
		}
		known := false
		for _, key := range docBlockKeys {
			if _, ok := block[key]; ok {
				known = true
			}
		}
		if !known {
			problems = append(problems, fmt.Sprintf("body[%d] needs one of heading, text, bullets, numbers, table, pageBreak", i))
		}
		if rows, ok := block["table"].([]any); ok {
			for _, row := range rows {
				cells, _ := row.([]any)
				for _, cell := range cells {
					if text := strings.TrimSpace(fmt.Sprint(cell)); wordFormula.MatchString(text) {
						problems = append(problems, fmt.Sprintf("body[%d]: Word tables do not calculate formulas (%q) - write the computed number", i, text))
					}
				}
			}
		}
	}
	return brief, problems
}

// wordFormula: o bang Word kieu Excel ("=C2/B2") - Word in nguyen chuoi (cung quy tac voi writer.insertTable).
var wordFormula = regexp.MustCompile(`^=\s*[A-Za-z(]`)

func (b *docBrief) or(value, field string) string {
	if value != "" {
		return value
	}
	b.placeholders = append(b.placeholders, field)
	return docPlaceholder
}

var numericDate = regexp.MustCompile(`^(\d{1,2})[/.-](\d{1,2})[/.-](\d{4})$`)

// dateLine: "Hà Nội, ngày 05 tháng 02 năm 2026" - ngay < 10 va thang 1, 2 them so 0 (NĐ 30, Phu luc I).
func (b *docBrief) dateLine() string {
	place := b.or(b.Place, "place")
	date := b.Date
	if match := numericDate.FindStringSubmatch(date); match != nil {
		day, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		monthText := strconv.Itoa(month)
		if month < 3 {
			monthText = fmt.Sprintf("%02d", month)
		}
		date = fmt.Sprintf("ngày %02d tháng %s năm %s", day, monthText, match[3])
	}
	if date == "" {
		b.placeholders = append(b.placeholders, "date")
		date = "ngày … tháng … năm …"
	} else if !strings.HasPrefix(strings.ToLower(date), "ngày") {
		date = "ngày " + date
	}
	return place + ", " + date
}

func layoutLine(text string, bold, italic bool, size int) map[string]any {
	line := map[string]any{"t": text}
	if bold {
		line["bold"] = true
	}
	if italic {
		line["italic"] = true
	}
	if size > 0 {
		line["size"] = size
	}
	return line
}

// headerRows: phan dau - trai: co quan chu quan, co quan ban hanh (dam), so ky hieu (+ trich yeu cong van); phai: quoc
// hieu, tieu ngu (dam), dia danh ngay thang (nghieng).
func (b *docBrief) headerRows() []any {
	left := []any{}
	if b.Parent != "" {
		left = append(left, layoutLine(strings.ToUpper(b.Parent), false, false, 13))
	}
	left = append(left, layoutLine(strings.ToUpper(b.or(b.Agency, "agency.name")), true, false, 13))
	left = append(left, layoutLine("Số: "+b.or(b.Number, "number"), false, false, 13))
	if b.Kind == "cong-van" {
		left = append(left, layoutLine("V/v "+b.or(strings.TrimPrefix(b.Subject, "V/v "), "subject"), false, false, 12))
	}
	right := []any{
		layoutLine("CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM", true, false, 12),
		layoutLine("Độc lập - Tự do - Hạnh phúc", true, false, 13),
		layoutLine("", false, false, 0),
		layoutLine(b.dateLine(), false, true, 13),
	}
	return []any{[]any{map[string]any{"lines": left}, map[string]any{"lines": right}}}
}

// titleBlocks: ten loai + trich yeu (dam, giua), Kinh gui; bien ban: thong tin cuoc hop.
func (b *docBrief) titleBlocks() []any {
	blocks := []any{}
	if name := docKinds[b.Kind]; name != "" {
		blocks = append(blocks, map[string]any{"text": name, "alignment": "center", "bold": true, "size": 14})
		subject := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(b.Subject, "Về việc"), "V/v"))
		if subject != "" {
			prefix := "Về việc "
			if b.Kind == "bien-ban" || b.Kind == "bao-cao" || b.Kind == "ke-hoach" {
				prefix = ""
			}
			blocks = append(blocks, map[string]any{"text": prefix + subject, "alignment": "center", "bold": true, "size": 14})
		} else {
			b.placeholders = append(b.placeholders, "subject")
		}
	}
	switch len(b.To) {
	case 0:
		if b.Kind == "to-trinh" || b.Kind == "cong-van" {
			b.placeholders = append(b.placeholders, "to")
			blocks = append(blocks, map[string]any{"text": "Kính gửi: " + docPlaceholder, "alignment": "center"})
		}
	case 1:
		blocks = append(blocks, map[string]any{"text": "Kính gửi: " + strings.TrimSuffix(b.To[0], ".") + ".", "alignment": "center"})
	default:
		lines := []string{"Kính gửi:"}
		for i, to := range b.To {
			end := ";"
			if i == len(b.To)-1 {
				end = "."
			}
			lines = append(lines, "- "+strings.TrimRight(to, ";.")+end)
		}
		blocks = append(blocks, map[string]any{"text": strings.Join(lines, "\n"), "alignment": "center"})
	}
	if b.Kind == "bien-ban" {
		lines := []string{}
		for _, field := range []struct{ key, label string }{{"time", "Thời gian"}, {"place", "Địa điểm"},
			{"chair", "Chủ trì"}, {"secretary", "Thư ký"}, {"attendees", "Thành phần tham dự"}} {
			lines = append(lines, field.label+": "+b.or(b.Meeting[field.key], "meeting."+field.key))
		}
		blocks = append(blocks, map[string]any{"text": strings.Join(lines, "\n")})
	}
	return blocks
}

// signRows: phan ky - trai: noi nhan (nghieng dam 12, danh sach 11); phai: chuc vu (dam), khoang ky, ho ten (dam).
// Bien ban: thu ky | chu tri.
func (b *docBrief) signRows() []any {
	signature := func(position, name, field string) map[string]any {
		lines := []any{}
		if b.OnBehalf != "" && field == "signer" {
			lines = append(lines, layoutLine(strings.ToUpper(b.OnBehalf), true, false, 13))
		}
		lines = append(lines, layoutLine(strings.ToUpper(position), true, false, 13))
		lines = append(lines, layoutLine("", false, false, 0), layoutLine("", false, false, 0), layoutLine("", false, false, 0))
		lines = append(lines, layoutLine(b.or(name, field+".name"), true, false, 13))
		return map[string]any{"lines": lines}
	}
	if b.Kind == "bien-ban" {
		return []any{[]any{signature("THƯ KÝ", b.Meeting["secretary"], "meeting.secretary"),
			signature("CHỦ TRÌ", b.Meeting["chair"], "meeting.chair")}}
	}
	recipients := b.Recipients
	if len(recipients) == 0 {
		recipients = []string{"Như trên", "Lưu: VT"}
	}
	left := []any{layoutLine("Nơi nhận:", true, true, 12)}
	for i, to := range recipients {
		end := ";"
		if i == len(recipients)-1 {
			end = "."
		}
		left = append(left, layoutLine("- "+strings.TrimRight(strings.TrimPrefix(to, "- "), ";.")+end, false, false, 11))
	}
	return []any{[]any{map[string]any{"lines": left, "align": "left"}, signature(b.or(b.Position, "signer.position"), b.Signer, "signer")}}
}

func (t *DocFromBriefTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	brief, problems := parseDocBrief(arguments)
	if len(problems) > 0 {
		return Result{JSON: FailJSON(strings.Join(problems, "; "), map[string]any{"built": 0,
			"how": "nothing was built: fix these and call again with the whole brief"})}
	}
	target := strings.TrimSpace(run.TargetDocument)
	steps := 0
	bridge := func(action string, params map[string]any) (map[string]any, string) {
		if target != "" {
			params["document"] = target
		}
		result := run.Bridge.Command(ctx, run.Office.Port, action, params)
		run.trace("tool.bridge", map[string]any{"tool": "doc_from_brief", "action": action, "params": params,
			"ok": result.OK, "error": result.Error})
		if !result.OK {
			return nil, action + ": " + result.Error
		}
		if makesUndoStep(action) {
			steps++
		}
		return result.Result, ""
	}
	// Chi dung van ban MOI: tai lieu co san thi phan dau/phan ky chen vao cuoi se sai cho.
	text, problem := bridge("writer.getText", map[string]any{"maxChars": 1})
	if problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	if chars := asInt(text["totalChars"]); chars > 20 {
		return Result{JSON: ErrorJSON(fmt.Sprintf("the document already has %d characters - doc_from_brief builds a NEW "+
			"document in an empty one: open a new document (writer.newDocument) first, or edit this one with "+
			"writer.writeContent / writer.replaceMany", chars))}
	}
	fail := func(problem string) Result {
		run.addUndoSteps("writer.writeContent", steps)
		return Result{JSON: FailJSON(problem, map[string]any{"built": "partly", "undoSteps": steps,
			"how": fmt.Sprintf("remove the partial document with writer.undo {\"count\": %d}, fix the problem and call again", steps)}),
			Mutating: steps > 0}
	}
	if _, problem := bridge("writer.insertLayoutTable", map[string]any{"rows": brief.headerRows(), "widths": []any{40, 60}}); problem != "" {
		return fail(problem)
	}
	if _, problem := bridge("writer.writeContent", map[string]any{"blocks": brief.titleBlocks()}); problem != "" {
		return fail(problem)
	}
	if brief.TOC {
		if _, problem := bridge("writer.insertToc", map[string]any{}); problem != "" {
			return fail(problem)
		}
	}
	if _, problem := bridge("writer.writeContent", map[string]any{"blocks": brief.Body}); problem != "" {
		return fail(problem)
	}
	if _, problem := bridge("writer.insertLayoutTable", map[string]any{"rows": brief.signRows(), "widths": []any{45, 55}}); problem != "" {
		return fail(problem)
	}
	if brief.PageNumber {
		if _, problem := bridge("writer.setHeaderFooter", map[string]any{"pageNumber": true}); problem != "" {
			return fail(problem)
		}
	}
	if brief.TOC {
		if _, problem := bridge("writer.insertToc", map[string]any{}); problem != "" {
			return fail(problem)
		}
	}
	run.addUndoSteps("writer.writeContent", steps)
	check := map[string]any{}
	if brief.MaxPages > 0 {
		check["maxPages"] = brief.MaxPages
	}
	layout, problem := bridge("writer.checkLayout", check)
	summary := map[string]any{"built": brief.Kind, "undoSteps": steps}
	if problem == "" {
		summary["pages"] = layout["pages"]
		summary["layoutIssues"] = layout["issues"]
		summary["verified"] = map[string]any{"checked": "page layout (writer.checkLayout: blank pages, headings at a page " +
			"bottom, split tables, page limit)", "issueCount": layout["issueCount"],
			"notChecked": "wording and facts - compare with the request yourself"}
	}
	if len(brief.placeholders) > 0 {
		summary["placeholders"] = brief.placeholders
		summary["next"] = "tell the user which details are still \"…\" (" + strings.Join(brief.placeholders, ", ") +
			") so they can fill them in; do not invent them"
	}
	return Result{JSON: OKJSON(summary), OK: true, Mutating: true, Verifies: problem == ""}
}
