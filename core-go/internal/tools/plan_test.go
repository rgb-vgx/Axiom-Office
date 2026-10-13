package tools

import (
	"context"
	"strings"
	"testing"

	"axiomoffice/core/internal/mcp"
	"axiomoffice/core/internal/office"
)

// Che do lap ke hoach: office_action chi con lenh DOC - lenh sua va lenh ghi file (saveAs, exportPdf) vua
// bi an khoi mo ta tool vua bi tu choi neu model van goi.
func TestReadOnlyOfficeActionHidesAndBlocksWrites(t *testing.T) {
	source := &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "wpp.listSlides", Kind: "wpp", Agent: true},
		{Name: "wpp.checkLayout", Kind: "wpp", Agent: true},
		{Name: "wpp.addText", Kind: "wpp", Agent: true},
		{Name: "wpp.saveAs", Kind: "wpp", Agent: true, Params: []office.CommandParam{{Name: "path", Required: true}}},
		{Name: "wpp.exportPdf", Kind: "wpp", Agent: true},
	}}
	tool := NewReadOnlyOfficeActionTool(source, "wpp")
	if got := strings.Join(tool.AllowedActions(), ","); got != "wpp.checkLayout,wpp.listSlides" {
		t.Fatalf("plan mode allowlist = %q", got)
	}
	for _, hidden := range []string{"wpp.addText", "wpp.saveAs", "wpp.exportPdf"} {
		if strings.Contains(tool.Description(), hidden) {
			t.Errorf("mo ta tool plan mode khong duoc liet ke %s", hidden)
		}
	}
	run := &RunContext{Office: &office.Session{Port: 1}, Bridge: office.NewBridgeClient(nil, "")}
	result := tool.Invoke(context.Background(), map[string]any{"action": "wpp.addText", "params": map[string]any{}}, run)
	if result.OK || !strings.Contains(result.JSON, "not an available action") {
		t.Fatalf("lenh sua phai bi chan trong plan mode: %+v", result)
	}
	// Ban thuong van du lenh (khong lam hong che do thuc hien).
	if full := NewOfficeActionTool(source, "wpp"); len(full.AllowedActions()) != 5 {
		t.Fatalf("che do thuong mat lenh: %v", full.AllowedActions())
	}
}

func TestReadOnlyMCPKeepsOnlyOfficeReaders(t *testing.T) {
	officeServer := &mcp.Server{Config: mcp.ServerConfig{Name: "office"}}
	other := &mcp.Server{Config: mcp.ServerConfig{Name: "github"}}
	tools := []mcp.ServerTool{
		{Server: officeServer, Tool: mcp.ToolInfo{Name: "excel_read"}},
		{Server: officeServer, Tool: mcp.ToolInfo{Name: "excel_profile"}},
		{Server: officeServer, Tool: mcp.ToolInfo{Name: "ppt_get_text"}},
		{Server: officeServer, Tool: mcp.ToolInfo{Name: "doc_get_text"}},
		{Server: officeServer, Tool: mcp.ToolInfo{Name: "excel_write"}},
		{Server: officeServer, Tool: mcp.ToolInfo{Name: "ppt_save"}},
		{Server: officeServer, Tool: mcp.ToolInfo{Name: "doc_create"}},
		{Server: other, Tool: mcp.ToolInfo{Name: "issue_read"}},
	}
	names := []string{}
	for _, item := range ReadOnlyMCP(tools) {
		names = append(names, item.Tool.Name)
	}
	if got := strings.Join(names, ","); got != "excel_read,excel_profile,ppt_get_text,doc_get_text" {
		t.Fatalf("plan mode MCP = %q", got)
	}
}
