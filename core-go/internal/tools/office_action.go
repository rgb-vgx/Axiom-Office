package tools

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

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
	UndoRule = "Change existing content in place (e.g. writer.formatTable to restyle a table); " +
		"never use undo to start over - only undo when the user asks."

	// ChunkRule: tham so cua MOT lan goi phai nam gon trong MOT phan hoi cua model. Bang vai nghin dong
	// gui trong mot lan thi chinh phan tra loi do het ngan sach token truoc khi viet xong - do duoc ngay
	// 02/10/2026: mot luot chay chet voi finish_reason=length khi co gui ca bang lon.
	ChunkRule = "Build up large sheets in chunks (a few hundred rows per call, continuing from the next " +
		"row): every argument of one call has to fit in a single reply, so one gigantic values array makes " +
		"the reply run out of tokens before it is finished. "
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

func signature(command office.Command) string {
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

func (t *OfficeActionTool) Name() string { return "office_action" }

// TruncationContract: noi RO truong hop response bi cat de model khong tu ket luan "het roi" tu
// mau dau (tu van ChatGPT 04/10: khong duoc ky vong model tu suy ra wrapper da bien doi response).
// Chi noi contract + hanh vi, khong noi implementation (wrapper cat o layer nao la chuyen cua Go).
const TruncationContract = "Large reads may come back truncated: truncated=true means values/formulas " +
	"hold only a sample (head plus tailValues/tailFormulas of the end), totalRows/totalCols give the " +
	"real size, and nextRange is the part not returned (reads stay capped, so read it in bounded " +
	"chunks). A truncated sample never proves anything about the rest of the range - before concluding " +
	"the whole range is clean, uniform, or complete, read the omitted part or use et.checkRange. "

func (t *OfficeActionTool) Description() string {
	return "Read and modify the LIVE document that is currently open in the office application. " +
		"Call this for every document change the user asks for so it happens immediately on screen. " +
		"Array params such as values must be real JSON arrays of rows, not objects. " +
		UndoRule + " " + ChunkRule + TruncationContract +
		"Available actions (with params): " + t.signatures
}

func (t *OfficeActionTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"action":{"type":"string","description":"Action name, e.g. writer.insertStyledText or wpp.addSlide"},` +
		`"params":{"type":"object","description":"Action parameters as an object (may be omitted)"}},` +
		`"required":["action"]}`)
}

// AllowedActions: danh sach lenh duoc phep (test dung).
func (t *OfficeActionTool) AllowedActions() []string { return t.names }

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

	params := NormalizeParams(arguments["params"])

	// Policy xac nhan (muc 8.6): hoi nguoi dung truoc lenh rui ro.
	decision := policy.Evaluate(ctx, action, params, run.Prompt, func(inner context.Context) *int {
		return policy.WordLength(inner, run.Bridge, run.Office.Port)
	})
	if decision.NeedsConfirmation {
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
	// Buoc 2 roadmap: cat response doc lon TRUOC khi vao turns (chi thanh cong moi cat;
	// loi van tra nguyen van de model doc thong diep loi day du) - xem truncate.go.
	raw := result.RawJSON
	if result.OK {
		raw = TruncateBridgeResult(action, result.Result, raw)
	}
	// Lenh that bai thi tai lieu khong doi -> khong tinh la da sua.
	return Result{JSON: raw, OK: result.OK, Action: action,
		Mutating: result.OK && ChangesDocument(action)}
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
