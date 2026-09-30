package mcp

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/model"
)

// MaxResultChars: tran ket qua tra cho model.
const MaxResultChars = 64 * 1024

// ServerTool: mot tool MCP da gan voi server cua no. Vong ngoai (goi tools cua Core) boc lai thanh tool
// cua agent - giu goi nay khong phu thuoc goi tools de khong vong import.
type ServerTool struct {
	Server *Server
	Tool   ToolInfo
}

func (t ServerTool) Name() string { return ToolName(t.Server.Config.Name, t.Tool.Name) }

func (t ServerTool) Description() string {
	return model.Truncate("["+t.Server.Config.Name+"] "+t.Tool.Description, 1000)
}

func (t ServerTool) Schema() json.RawMessage { return t.Tool.InputSchema }

// Call: goi server; loi -> ok=false kem cau loi (khong lam hong luot chay).
func (t ServerTool) Call(ctx context.Context, arguments any) (string, bool) {
	text, isError, err := t.Server.Call(ctx, t.Tool.Name, arguments)
	if err != nil {
		return ErrorJSON("mcp server '" + t.Server.Config.Name + "' error: " + err.Error()), false
	}
	return text, !isError
}

// ErrorJSON: {"ok":false,"error":"..."} (giong ModelClient.ErrorJson).
func ErrorJSON(message string) string { return model.ErrorJSON(message) }

// ResultJSON: {"ok":true,"result":"..."} hoac {"ok":false,"error":"..."} kem tran 64KB.
func ResultJSON(text string, ok bool) string {
	payload := map[string]any{"ok": ok}
	if ok {
		payload["result"] = model.Truncate(text, MaxResultChars)
	} else {
		payload["error"] = model.Truncate(text, MaxResultChars)
	}
	return model.MarshalRelaxed(payload)
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// ToolName: "mcp__<server>__<tool>" (ky tu la -> '_', toi da 64 ky tu).
func ToolName(serverName, toolName string) string {
	name := "mcp__" + unsafeName.ReplaceAllString(serverName, "_") + "__" + unsafeName.ReplaceAllString(toolName, "_")
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// NeedsConfirm: server khong tin cay -> hoi truoc moi lan goi; server office (tin cay) -> hoi khi tool GHI
// vao file DA CO tren dia (cung quy tac voi saveAs). Tra ve ly do hoi, hoac "" khi khong can.
func NeedsConfirm(tool ServerTool, arguments map[string]any) string {
	if !tool.Server.Config.Trusted {
		return "Dùng công cụ ngoài '" + tool.Server.Config.Name + "': " + tool.Tool.Name
	}
	if !tool.Server.Config.BuiltIn || OfficeReadOnlyTools[tool.Tool.Name] {
		return ""
	}
	path, _ := arguments["path"].(string)
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return "Sửa file trên đĩa: " + path
}

func itoaSeconds(duration time.Duration) string {
	return strconv.FormatFloat(duration.Seconds(), 'f', -1, 64)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CoreClientInfo: ten + phien ban ma Core khai voi server.
var clientVersion = "0.0.0"

func SetClientVersion(version string) {
	if strings.TrimSpace(version) != "" {
		clientVersion = version
	}
}

func CoreClientInfo() map[string]any {
	return map[string]any{"name": "axiom-office-core", "version": clientVersion}
}
