package tools

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/policy"
)

// OfficeActionTool: tool duy nhat ma agent dung de doc/sua tai lieu dang mo: office_action {action, params}.
// Mo ta tool liet ke chu ky cac lenh agent=true cua app dang mo (tu GET /commands) va CHI cho goi nhung lenh do.
type OfficeActionTool struct {
	allowed    map[string]office.Command
	names      []string
	signatures string
}

const (
	// UndoRule: khong co lenh dinh dang bang, model undo 2 lan (xoa bang + ghi chu) roi dung lai tu dau.
	// Luot 11/10/2026: them ngoai le cho "lam lai chinh viec vua lam" - co lenh khong co cach sua tai cho
	// (khong co writer.setCell), nen cam undo tuyet doi se khoa luon duong sua sai cua chinh luot nay va
	// day model sang writer.replaceAll - lenh thay MOI lan xuat hien trong ca tai lieu. Ngoai le nay chi mo
	// cho thao tac CUA CHINH LUOT NAY, khong mo cho tai lieu co san cua nguoi dung.
	UndoRule = "Change existing content in place (e.g. writer.formatTable to restyle a table); " +
		"never use undo to start over - only undo when the user asks. " +
		"Exception: undo is allowed to take back something YOU did in this same turn when there is no way " +
		"to correct it in place (e.g. undo the table you just inserted, then insert it again with the right " +
		"values). It never applies to content that was in the document before this turn."

	// ChunkRule: tham so cua MOT lan goi phai nam gon trong MOT phan hoi cua model. Bang vai nghin dong
	// gui trong mot lan thi chinh phan tra loi do het ngan sach token truoc khi viet xong - do duoc ngay
	// 02/10/2026: mot luot chay chet voi finish_reason=length khi co gui ca bang lon.
	ChunkRule = "Build up large sheets in chunks (a few hundred rows per call, continuing from the next " +
		"row): every argument of one call has to fit in a single reply, so one gigantic values array makes " +
		"the reply run out of tokens before it is finished. "

	// SheetRefRule: MOT cu phap cho moi app. Do 04/10/2026: luot Test 2 (mimo) mat ~4 lenh do xem
	// `Sheet1.A1` hay `Sheet1!A1`; LibreOffice tra #NAME? cho `!` - nay bridge LibreOffice tu doi `!`
	// sang `.`, Excel/WPS von dung `!`, nen noi thang mot kieu cho model khoi do.
	SheetRefRule = "In formulas, reference another sheet as Sheet1!A1 or 'My Sheet'!A1:B5 - this works in " +
		"every app (LibreOffice converts it), so do not test other syntaxes. Formulas use English function " +
		"names and commas between arguments. "
)

func NewOfficeActionTool(catalog *office.CommandCatalog, appKind string) *OfficeActionTool {
	allowed := map[string]office.Command{}
	names := []string{}
	signatures := []string{}
	for _, command := range catalog.Commands {
		if !command.Agent || (command.Kind != "" && command.Kind != appKind) {
			continue
		}
		allowed[command.Name] = command
		names = append(names, command.Name)
		signatures = append(signatures, signature(command))
	}
	sort.Strings(names)
	return &OfficeActionTool{allowed: allowed, names: names, signatures: strings.Join(signatures, "; ")}
}

// NewReadOnlyOfficeActionTool: office_action cho che do lap ke hoach - chi liet ke VA chi cho goi lenh doc
// (ReadOnly). Model yeu thay lenh sua trong mo ta tool thi hay goi thu; an han khoi danh sach thay vi chi chan.
func NewReadOnlyOfficeActionTool(catalog *office.CommandCatalog, appKind string) *OfficeActionTool {
	reads := &office.CommandCatalog{Version: catalog.Version}
	for _, command := range catalog.Commands {
		if ReadOnly(command.Name) {
			reads.Commands = append(reads.Commands, command)
		}
	}
	return NewOfficeActionTool(reads, appKind)
}

func signature(command office.Command) string {
	params := make([]string, 0, len(command.Params))
	hidden := targetParam(command.Name)
	for _, param := range command.Params {
		if param.Name == hidden {
			continue // Core tu gan tai lieu cua luot (target.go): model khong can doc tham so nay
		}
		if param.Hint == "" {
			params = append(params, param.Name)
			continue
		}
		params = append(params, param.Name+" ("+param.Hint+")")
	}
	return command.Name + " {" + strings.Join(params, ",") + "}"
}

func (t *OfficeActionTool) Name() string { return "office_action" }

// TruncationContract: noi RO truong hop response bi cat de model khong tu ket luan "het roi" tu
// mau dau (tu van ChatGPT 04/10: khong duoc ky vong model tu suy ra wrapper da bien doi response).
// Chi noi contract + hanh vi, khong noi implementation (wrapper cat o layer nao la chuyen cua Go).
const TruncationContract = "Large reads may come back truncated: truncated=true means values/formulas " +
	"hold only a sample (head plus tailValues/tailFormulas of the end), totalRows/totalCols give the " +
	"real size, and nextRange is the part not returned (reads stay capped, so read it in bounded " +
	"chunks). A truncated sample never proves anything about the rest of the range - before concluding " +
	"the whole range is clean, uniform, or complete, read the omitted part or use et.checkRange. "

// BatchEditRule: luot that 14/10/2026 sua giao an bang 137 lan writer.replaceAll (nhieu lan chi 1-2 ky tu) -> 48 vong,
// 1,48 trieu token. Gom vao writer.replaceMany thi vai vong; find ngan thi de thay nham cho khac.
const BatchEditRule = "For several text edits, send them together in ONE writer.replaceMany call (up to 200 " +
	"{find, replace} pairs, applied in order) instead of many writer.replaceAll calls; make each find long " +
	"enough to occur only where you mean it, because every occurrence is replaced. "

func (t *OfficeActionTool) Description() string {
	batch := ""
	if _, ok := t.allowed["writer.replaceMany"]; ok {
		batch = BatchEditRule
	}
	// Cau dau noi RO "lenh la gia tri cua action, khong phai tool": luot that 24/10/2026 (ocg/beam-free) goi
	// thang writer.getText nhu mot tool rieng -> "unknown tool", mat mot vong. Danh sach chu ky "ten {tham so}"
	// o cuoi mo ta trong giong mot loat ham, nen phai noi truoc.
	return "Read and modify the LIVE document that is currently open in the office application. " +
		"Every document command below (writer.*, et.*, wpp.*, app.*) is a value of this tool's `action` argument, " +
		"not a separate tool: call office_action {\"action\": \"writer.getText\", \"params\": {...}}. " +
		"Call this for every document change the user asks for so it happens immediately on screen. " +
		"Array params such as values must be real JSON arrays of rows, not objects. " +
		UndoRule + " " + ChunkRule + SheetRefRule + batch + TruncationContract +
		"Available actions (with params): " + t.signatures
}

// Parameters: `action` la enum dung danh sach lenh duoc phep - chuan JSON Schema cho gia tri dong (nhu web_sources,
// write_todos): model thay ro day la GIA TRI cua tham so, va nha cung cap co constrained decoding chan luon ten
// sai. Mo ta tool van giu chu ky "ten {tham so}" vi enum khong mang tham so cua tung lenh.
func (t *OfficeActionTool) Parameters() json.RawMessage {
	action := map[string]any{"type": "string",
		"description": "Document command to run, e.g. writer.insertStyledText or wpp.addSlide (one of the listed values)"}
	if len(t.names) > 0 {
		action["enum"] = t.names
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": action,
			"params": map[string]any{"type": "object", "description": "Action parameters as an object (may be omitted)"},
		},
		"required": []string{"action"},
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return json.RawMessage(`{"type":"object","properties":{"action":{"type":"string"},"params":{"type":"object"}},"required":["action"]}`)
	}
	return raw
}

// AllowedActions: danh sach lenh duoc phep (test dung).
func (t *OfficeActionTool) AllowedActions() []string { return t.names }

// ResolveAction: `name` (sau CanonicalAction: khac hoa/thuong, "et_writeRange") co phai mot lenh duoc phep khong.
// Dung de sua tool call goi nham ten lenh lam ten tool (agent/repair.go) - CHI khop chinh xac, khong doan gan dung.
func (t *OfficeActionTool) ResolveAction(name string) (string, bool) {
	action := CanonicalAction(name, t.names)
	_, ok := t.allowed[action]
	return action, ok
}

// AvailableLine: cau liet ke lenh con lai (cung nhom voi `action` neu co) - xem availableLine.
func (t *OfficeActionTool) AvailableLine(action string) string { return t.availableLine(action) }

// availableLine: cau noi ro con nhung lenh nao. Cung nhom (cung tien to "et."/"writer."/"wpp.") thi chi
// liet ke nhom do - model go sai tien to thi can biet ca nhom con lai; go sai giua chung thi can thay het.
func (t *OfficeActionTool) availableLine(action string) string {
	prefix := ""
	if index := strings.Index(action, "."); index >= 0 {
		prefix = action[:index+1]
	}
	same := make([]string, 0, len(t.names))
	all := make([]string, 0, len(t.names))
	for _, name := range t.names {
		all = append(all, name)
		if prefix != "" && strings.HasPrefix(name, prefix) {
			same = append(same, name)
		}
	}
	if len(same) > 0 {
		return "Available " + strings.TrimSuffix(prefix, ".") + " actions are: " + strings.Join(same, ", ") +
			". Only these exist; do not invent action names."
	}
	return "Available actions are: " + strings.Join(all, ", ") + ". Only these exist; do not invent action names."
}

func (t *OfficeActionTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	action := stringArg(arguments, "action")
	if action == "" {
		return Result{JSON: ErrorJSON("missing 'action' argument")}
	}

	action = CanonicalAction(action, t.names)
	if _, ok := t.allowed[action]; !ok {
		// Model chi duoc goi lenh co trong mo ta tool (chan writer.closeAll, ai.ask long nhau...).
		//
		// Loi phai noi NGAY co nhung lenh nao: log 02/10/2026, mot luot chay doan ~30 ten lenh tu nghi ra
		// (et.createSheet, et.insertSheet, et.addSheet...) chi vi khong tin rang lenh no can khong ton tai.
		// Liet ke lai danh sach ngay tai cho bao loi cat vong doan do som hon la bat model doc lai mo ta.
		message := "'" + action + "' is not an available action. " + t.availableLine(action)
		return Result{JSON: ErrorJSON(message), Action: action}
	}

	params, problem := normalizeParamsChecked(arguments["params"])
	if problem != "" {
		// Bao DUNG loi o day: truoc 04/10 params hong bi bo im thanh nil, bridge bao "'range' is
		// required" trong khi model DA gui range -> model gui lai y het (variant1 lap 5 lan).
		return Result{JSON: ErrorJSON(problem), Action: action}
	}
	params = aliasParams(t.allowed[action], params)
	params = withTarget(t.allowed[action], action, params, run.TargetDocument)
	if problem := loneSymbolFind(action, params); problem != "" {
		return Result{JSON: ErrorJSON(problem), Action: action}
	}
	if isUndo(action) {
		var problem string
		if params, problem = limitUndo(run, action, params); problem != "" {
			return Result{JSON: ErrorJSON(problem), Action: action}
		}
	}

	// Guard chong ghi de (log 11/10/2026): lenh ghi o vao sheet co tu truoc ma dung vao o
	// khong rong voi gia tri moi khac gia tri cu thi hoi nguoi dung truoc - xem overwrite.go.
	if reason, sheets := checkOverwriteGuard(ctx, run, action, params); reason != "" {
		preview := model.Truncate(model.MarshalRelaxed(params), 200)
		if preview == "null" {
			preview = "{}"
		}
		approved, err := run.ConfirmOrDecline(action, reason, preview)
		if err != nil || !approved {
			return Result{JSON: ErrorJSON("user declined: " + reason), Action: action}
		}
		rememberOverwriteApproval(run, sheets)
	}

	// Policy xac nhan (muc 8.6): hoi nguoi dung truoc lenh rui ro.
	decision := policy.Evaluate(ctx, action, params, run.Prompt, func(inner context.Context) *int {
		return policy.WordLength(inner, run.Bridge, run.Office.Port)
	})
	if decision.NeedsConfirmation && run.ownedObjects().covers(action, params) {
		// Agent sua lai chinh thu no vua dung trong luot: khong hoi (owned.go). Ghi log de soat lai duoc.
		corelog.Info("run %s: %s khong can xac nhan - chi dung toi doi tuong do luot tao (%s)", run.RunID, action,
			model.Truncate(model.MarshalRelaxed(params), 200))
	} else if decision.NeedsConfirmation && action == "wpp.deleteSlide" &&
		policy.BlankSlide(ctx, run.Bridge, run.Office.Port, params) {
		corelog.Info("run %s: wpp.deleteSlide khong can xac nhan - slide trong (%s)", run.RunID,
			model.Truncate(model.MarshalRelaxed(params), 200))
	} else if decision.NeedsConfirmation {
		preview := model.Truncate(model.MarshalRelaxed(params), 200)
		if preview == "null" {
			preview = "{}"
		}
		approved, err := run.ConfirmOrDecline(action, decision.Reason, preview)
		if err != nil || !approved {
			return Result{JSON: ErrorJSON("user declined: " + decision.Reason), Action: action}
		}
	}

	result := run.Bridge.Command(ctx, run.Office.Port, action, params)
	// Sheet vua tao thanh cong trong luot nay thi lenh ghi sau len do khong can hoi ghi de.
	if result.OK && action == "et.addSheet" {
		rememberCreatedSheet(run, params)
	}
	if result.OK {
		rememberWrittenCells(run, action, params, result.Result)
	}
	if result.OK && action == "wpp.checkLayout" {
		// Loi bo cuc vao core.log (chi loai + ten hinh): soat lai duoc model da thay gi truoc khi sua.
		slides, _ := result.Result["slides"].([]any)
		for _, item := range slides {
			entry, _ := item.(map[string]any)
			issues, _ := entry["issues"].([]any)
			logIssues(run.RunID, action, asInt(entry["index"]), issues)
		}
	}
	if result.OK {
		run.ownedObjects().record(action, params, result.Result)
		if opensDocument(action) {
			if opened := openedDocument(params, result.Result); opened != "" {
				run.TargetDocument = opened
			}
		}
		if renamed := savedDocument(action, params, result.Result, run.TargetDocument); renamed != "" {
			run.TargetDocument = renamed
		}
	}
	// Buoc 2 roadmap: cat response doc lon TRUOC khi vao turns (chi thanh cong moi cat;
	// loi van tra nguyen van de model doc thong diep loi day du) - xem truncate.go.
	raw := result.RawJSON
	if result.OK {
		raw = TruncateBridgeResult(action, result.Result, raw, run.FocusSlide)
	}
	// Lenh that bai thi tai lieu khong doi -> khong tinh la da sua. Lenh ghi tu bao "changed": false (vd wpp.setShapeText
	// y het chu dang co) cung vay: revision giu nguyen nen lenh doc truoc do van dung (docStateGuard).
	unchanged := false
	if flag, ok := result.Result["changed"].(bool); ok && !flag {
		unchanged = true
	}
	output := Result{JSON: raw, OK: result.OK, Action: action, Mutating: result.OK && ChangesDocument(action) && !unchanged,
		Verifies: result.OK && Verifies(action)}
	countUndoable(run, action, output.Mutating, result.Result)
	if !result.OK && AppGone(result.Error) {
		output.Abort = appGoneMessage
	}
	return output
}

// loneSymbolFind: luot that deepseek 15/10/2026 thay "/" -> "" tren ca giao an de xoa dong "/" ma (thuc ra la
// hinh/khoi) -> mat dau ngay "10/10/2026", "quả cầu/1 lần"; roi doi qua ~40 ky tu giong "/" (59 vong, 2 trieu token).
// find chi mot ky tu dau cau (hoac xoa han mot ky tu) thi tu choi: dau do nam trong ngay thang, so lieu, link.
func loneSymbolFind(action string, params map[string]any) string {
	var edits []map[string]any
	switch action {
	case "writer.replaceAll":
		edits = []map[string]any{params}
	case "writer.replaceMany":
		if list, ok := params["edits"].([]any); ok {
			for _, item := range list {
				if edit, ok := item.(map[string]any); ok {
					edits = append(edits, edit)
				}
			}
		}
	default:
		return ""
	}
	for _, edit := range edits {
		find, _ := edit["find"].(string)
		replace, _ := edit["replace"].(string)
		runes := []rune(find)
		if len(runes) != 1 {
			continue
		}
		symbol := !unicode.IsLetter(runes[0]) && !unicode.IsDigit(runes[0]) && !unicode.IsSpace(runes[0])
		if replace == "" || (symbol && strings.ContainsRune("/.,:-", runes[0])) {
			return "find '" + find + "' alone changes every occurrence in the whole document, including dates " +
				"(10/10/2026), numbers and links. Put the surrounding words in find so it matches only the place you " +
				"mean. A line that shows only '/' or '[...]' in writer.getText is a picture or design block, not text: leave it."
		}
	}
	return ""
}

// appGoneMessage: ung dung Office da dong / sap giua luot - dung luot thay vi de model goi lai mai (luot that
// 14/10/2026: Word sap sau writer.replaceAll chuoi dai, model goi writer.getText vao bridge da chet nhieu vong).
const appGoneMessage = "Ứng dụng Office đã đóng hoặc bị sập giữa chừng nên không làm tiếp được. Hãy mở lại tài liệu " +
	"(Word/Excel/PowerPoint có thể đã tự khôi phục bản đang sửa), kiểm tra nội dung rồi gửi lại yêu cầu."

// AppGone: loi ket noi toi bridge cho thay ung dung Office khong con chay (khong ai nghe o cong).
func AppGone(errorText string) bool {
	text := strings.ToLower(errorText)
	return strings.Contains(text, "actively refused") || strings.Contains(text, "connection refused") ||
		strings.Contains(text, "no connection could be made")
}

// CanonicalAction: ten lenh viet sai nhe ("et_writeRange"; hoa/thuong khac) thi quy ve ten dung trong
// allowlist thay vi tu choi va mat mot vong. Chi khop khi ket qua la mot lenh duoc phep.
func CanonicalAction(action string, allowed []string) string {
	trimmed := strings.TrimSpace(action)
	dotted := trimmed
	if !strings.Contains(trimmed, ".") {
		if index := strings.Index(trimmed, "_"); index >= 0 {
			dotted = trimmed[:index] + "." + trimmed[index+1:]
		}
	}
	for _, name := range allowed {
		if strings.EqualFold(name, trimmed) || strings.EqualFold(name, dotted) {
			return name
		}
	}
	return action
}

// normalizeParamsChecked: nhu NormalizeParams nhung chuoi JSON hong thi tra loi ro rang thay vi nil.
// Chuoi co rac phia sau object (model viet them "}" hoac giai thich) thi lay object DAU TIEN.
func normalizeParamsChecked(value any) (map[string]any, string) {
	text, isString := value.(string)
	if !isString {
		return NormalizeParams(value), ""
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || trimmed == "null" {
		return nil, ""
	}
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	var parsed map[string]any
	err := decoder.Decode(&parsed)
	if err == nil && parsed != nil {
		return parsed, ""
	}
	detail := "params is not a JSON object"
	if err != nil {
		detail = err.Error()
		if offset := decoder.InputOffset(); offset > 0 {
			start := max(0, int(offset)-40)
			end := min(len(trimmed), int(offset)+20)
			detail += " near: " + trimmed[start:end]
		}
	}
	return nil, "params was sent as a string that is not valid JSON (" + detail + "). Send params as a " +
		"JSON object, not a string; inside formulas escape double quotes once (\\\"), and do not put raw " +
		"line breaks inside strings."
}

// paramAliases: ten tham so model hay dung nham -> ten that. Do duoc 03/10/2026: 10 lan et.formatRange
// gui `cell_range` (ten cua tool file MCP excel_* ma agent cung nhin thay) -> "'range' is required".
// Chi doi khi lenh THAT SU co tham so dich va model chua gui ten dung.
var paramAliases = map[string][]string{
	"range": {"cell_range", "cellRange"},
}

func aliasParams(command office.Command, params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	for target, aliases := range paramAliases {
		if _, present := params[target]; present || !commandHasParam(command, target) {
			continue
		}
		for _, alias := range aliases {
			if value, ok := params[alias]; ok {
				params[target] = value
				delete(params, alias)
				break
			}
		}
	}
	return params
}

func commandHasParam(command office.Command, name string) bool {
	for _, param := range command.Params {
		if param.Name == name {
			return true
		}
	}
	return false
}

// NormalizeParams: model doi khi gui params la chuoi JSON ("{\"rows\":3}") thay vi object.
func NormalizeParams(value any) map[string]any {
	switch params := value.(type) {
	case nil:
		return nil
	case map[string]any:
		return params
	case string:
		trimmed := strings.TrimSpace(params)
		if trimmed == "" {
			return nil
		}
		if strings.HasPrefix(trimmed, "{") {
			var parsed map[string]any
			if json.Unmarshal([]byte(trimmed), &parsed) == nil {
				return parsed
			}
		}
	}
	return nil
}
