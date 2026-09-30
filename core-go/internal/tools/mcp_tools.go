package tools

import (
	"context"
	"encoding/json"
	"strings"

	"axiomoffice/core/internal/mcp"
)

// MCPTool: mot tool MCP cho model: ten mcp__<server>__<tool>; server khong tin cay -> hoi nguoi dung truoc
// moi lan goi; server office (tin cay) -> hoi khi tool GHI vao file DA CO (cung quy tac voi saveAs).
type MCPTool struct{ bound mcp.ServerTool }

// WrapMCPTools: boc cac tool MCP thanh tool cua agent.
func WrapMCPTools(list []mcp.ServerTool) []Tool {
	wrapped := make([]Tool, 0, len(list))
	for _, item := range list {
		wrapped = append(wrapped, &MCPTool{bound: item})
	}
	return wrapped
}

func (t *MCPTool) Name() string { return t.bound.Name() }

func (t *MCPTool) Description() string { return t.bound.Description() }

func (t *MCPTool) Parameters() json.RawMessage { return t.bound.Schema() }

func (t *MCPTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	name := t.bound.Name()
	if reason := mcp.NeedsConfirm(t.bound, arguments); reason != "" {
		preview := truncateJSON(arguments)
		approved, err := run.ConfirmOrDecline(name, reason, preview)
		if err != nil || !approved {
			return Result{JSON: ErrorJSON("user declined: " + reason), Action: name}
		}
	}

	text, ok := t.bound.Call(ctx, arguments)
	return Result{JSON: mcp.ResultJSON(text, ok), OK: ok, Action: name}
}

func truncateJSON(arguments map[string]any) string {
	if len(arguments) == 0 {
		return "{}"
	}
	encoded := jsonEncode(arguments)
	if len([]rune(encoded)) > 200 {
		return string([]rune(encoded)[:200])
	}
	return encoded
}

func jsonEncode(value any) string {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(value) != nil {
		return "{}"
	}
	return strings.TrimRight(builder.String(), "\n")
}
