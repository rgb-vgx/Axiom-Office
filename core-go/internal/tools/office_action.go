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

func (t *OfficeActionTool) Description() string {
	return "Read and modify the LIVE document that is currently open in the office application. " +
		"Call this for every document change the user asks for so it happens immediately on screen. " +
		"Array params such as values must be real JSON arrays of rows, not objects. " +
		UndoRule + " " +
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

func (t *OfficeActionTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	action := stringArg(arguments, "action")
	if action == "" {
		return Result{JSON: ErrorJSON("missing 'action' argument")}
	}

	action = CanonicalAction(action, t.names)
	if _, ok := t.allowed[action]; !ok {
		// Model chi duoc goi lenh co trong mo ta tool (chan writer.closeAll, ai.ask long nhau...).
		return Result{
			JSON: ErrorJSON("'" + action + "' is not an available action; use one of the actions listed " +
				"in the office_action tool description"),
			Action: action,
		}
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
	return Result{JSON: result.RawJSON, OK: result.OK, Action: action}
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
