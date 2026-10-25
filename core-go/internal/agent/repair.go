package agent

import (
	"encoding/json"
	"sort"
	"strings"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/tools"
)

// Tool call goi nham ten LENH lam ten TOOL (luot that 24/10/2026, ocg/beam-free: goi thang "writer.getText" ->
// "unknown tool", mat mot vong). Ba lop, theo thu tu:
//  1. phong: `action` cua office_action la enum + mo ta/prompt noi ro lenh la gia tri (tools/office_action.go, prompt.go);
//  2. sai van xay ra: loi chi dung cach goi (unknownToolMessage);
//  3. sua ho (repairToolCall): CHI khi ten khop CHINH XAC mot lenh duoc phep - cuoc goi da sua di qua dung duong
//     office_action (allowlist, policy, xac nhan, audit) nhu moi lenh khac. Lenh bi cam (writer.closeAll...) khong
//     sua, nen khong mo them cua nao.

// officeTool: office_action cua luot (nil neu luot khong co, vd tool rong).
func officeTool(registry *tools.Registry) *tools.OfficeActionTool {
	if registry == nil {
		return nil
	}
	tool, _ := registry.Find("office_action").(*tools.OfficeActionTool)
	return tool
}

// repairToolCall: tool khong ton tai ma ten la mot lenh duoc phep -> office_action {action, params}.
// Doi so: model gui {"params": {...}} thi lay params do; gui phang ({"maxChars": 10000}) thi ca doi so la params.
func repairToolCall(registry *tools.Registry, call model.ToolCall) (model.ToolCall, bool) {
	if registry == nil || registry.Find(call.Name) != nil {
		return call, false
	}
	office := officeTool(registry)
	if office == nil {
		return call, false
	}
	action, ok := office.ResolveAction(call.Name)
	if !ok {
		return call, false
	}
	var arguments map[string]any
	if strings.TrimSpace(call.Arguments) != "" && json.Unmarshal([]byte(call.Arguments), &arguments) != nil {
		// Doi so khong phai JSON object: van chuyen ten, de office_action bao loi params ro rang.
		arguments = nil
	}
	var params any = arguments
	if nested, ok := arguments["params"]; ok {
		params = nested
	} else if len(arguments) == 0 {
		params = nil
	} else {
		delete(arguments, "action")
	}
	body := map[string]any{"action": action}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return call, false
	}
	return model.ToolCall{ID: call.ID, Name: "office_action", Arguments: string(raw)}, true
}

// unknownToolMessage: loi cho tool khong ton tai - noi DUNG cach sua thay vi chi "unknown tool".
func unknownToolMessage(registry *tools.Registry, name string) string {
	office := officeTool(registry)
	if office != nil && looksLikeAction(name) {
		// Ten lenh nhung khong duoc phep (writer.closeAll, lenh cua app khac...): khong sua ho.
		action, _ := office.ResolveAction(name)
		return "'" + name + "' is not a tool: document commands are values of office_action's action argument " +
			"(office_action {\"action\": \"...\", \"params\": {...}}), and '" + action + "' is not an available one. " +
			office.AvailableLine(action)
	}
	names := []string{}
	if registry != nil {
		for _, tool := range registry.All() {
			names = append(names, tool.Name())
		}
	}
	sort.Strings(names)
	message := "unknown tool '" + name + "'. Available tools: " + strings.Join(names, ", ") + "."
	if office != nil {
		message += " Document commands (writer.*, et.*, wpp.*) are not tools: call office_action {\"action\": \"<command>\", \"params\": {...}}."
	}
	return message
}

// looksLikeAction: "writer.x", "et_x", "wpp.x", "app.x"... - dang ten lenh bridge, ke ca khi viet bang "_".
func looksLikeAction(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{"writer", "et", "wpp", "app", "ai", "ui"} {
		if strings.HasPrefix(lower, prefix+".") || strings.HasPrefix(lower, prefix+"_") {
			return true
		}
	}
	return false
}
