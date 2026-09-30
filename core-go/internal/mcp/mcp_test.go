package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigs(t *testing.T) {
	hostExe := filepath.Join(t.TempDir(), "AxiomOffice.Host.exe")
	if err := os.WriteFile(hostExe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	text := `{"mcpServers":{
		"mau":{"command":"python","args":["s.py"],"env":{"A":"1"}},
		"mau-tin":{"command":"python","args":["s.py"],"trusted":true},
		"hong":{"command":"/khong/ton/tai"},
		"tat":{"command":"python","disabled":true},
		"web":{"url":"http://127.0.0.1:1/mcp","headers":{"X":"1"}}}}`

	configs, problem := LoadConfigs(text, hostExe)
	if problem != "" {
		t.Fatalf("problem = %q", problem)
	}
	names := []string{}
	for _, config := range configs {
		names = append(names, config.Name)
	}
	// office built-in truoc, roi cac server trong mcp.json theo ten; server disabled bi bo.
	if strings.Join(names, ",") != "office,hong,mau,mau-tin,web" {
		t.Fatalf("names = %v (phai bo server disabled)", names)
	}

	office := configs[0]
	if !office.BuiltIn || !office.Trusted || office.Args[0] != "mcp" || !office.Allowed["doc_create"] || office.Allowed["word_command"] {
		t.Fatalf("office = %+v", office)
	}
	if configs[2].Env["A"] != "1" || configs[2].Trusted || configs[2].Args[0] != "s.py" {
		t.Fatalf("mau = %+v", configs[2])
	}
	if !configs[3].Trusted {
		t.Fatalf("mau-tin = %+v", configs[3])
	}
	if configs[4].URL == "" || configs[4].Headers["X"] != "1" {
		t.Fatalf("web = %+v", configs[4])
	}

	// Tat office built-in bang cau hinh, va mcp.json hong.
	disabled, _ := LoadConfigs(`{"mcpServers":{"office":{"disabled":true}}}`, hostExe)
	if len(disabled) != 0 {
		t.Fatalf("office disabled: %+v", disabled)
	}
	if _, problem := LoadConfigs("{khong phai json", hostExe); problem == "" {
		t.Fatal("mcp.json hong phai bao loi")
	}
	// Khong co Host.exe -> khong co server office.
	if without, _ := LoadConfigs("", filepath.Join(t.TempDir(), "khong-co.exe")); len(without) != 0 {
		t.Fatalf("thieu Host.exe: %+v", without)
	}
}

func TestToolNameAndConfirmPolicy(t *testing.T) {
	if got := ToolName("mau", "add"); got != "mcp__mau__add" {
		t.Fatalf("ToolName = %q", got)
	}
	if got := ToolName("may.chu/1", "lam:dep"); got != "mcp__may_chu_1__lam_dep" {
		t.Fatalf("ToolName lam sach = %q", got)
	}
	if got := ToolName(strings.Repeat("s", 50), strings.Repeat("t", 50)); len(got) != 64 {
		t.Fatalf("ToolName phai cat con 64: %d", len(got))
	}

	external := ServerTool{Server: &Server{Config: ServerConfig{Name: "mau"}}, Tool: ToolInfo{Name: "add"}}
	if reason := NeedsConfirm(external, nil); !strings.Contains(reason, "Dùng công cụ ngoài 'mau': add") {
		t.Fatalf("server ngoai phai hoi: %q", reason)
	}

	office := ServerTool{Server: &Server{Config: ServerConfig{Name: "office", BuiltIn: true, Trusted: true}},
		Tool: ToolInfo{Name: "doc_create"}}
	if reason := NeedsConfirm(office, map[string]any{"path": filepath.Join(t.TempDir(), "moi.docx")}); reason != "" {
		t.Fatalf("tao file moi khong hoi: %q", reason)
	}
	existing := filepath.Join(t.TempDir(), "da-co.docx")
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if reason := NeedsConfirm(office, map[string]any{"path": existing}); !strings.Contains(reason, "Sửa file trên đĩa") {
		t.Fatalf("ghi de file da co phai hoi: %q", reason)
	}
	readOnly := ServerTool{Server: office.Server, Tool: ToolInfo{Name: "doc_get_text"}}
	if reason := NeedsConfirm(readOnly, map[string]any{"path": existing}); reason != "" {
		t.Fatalf("tool chi doc khong hoi: %q", reason)
	}
}

// sampleServer: MCP server gia viet bang python (dung lai tests/core/mcp_sample_server.py cua repo).
func sampleServer(t *testing.T) string {
	t.Helper()
	script := filepath.Join("..", "..", "..", "tests", "core", "mcp_sample_server.py")
	absolute, err := filepath.Abs(script)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(absolute); err != nil {
		t.Skip("khong thay mcp_sample_server.py")
	}
	for _, candidate := range []string{"python", "python3"} {
		if path, err := exec.LookPath(candidate); err == nil {
			_ = path
			return absolute
		}
	}
	t.Skip("khong co python")
	return ""
}

func TestStdioServerHandshakeAndCall(t *testing.T) {
	script := sampleServer(t)
	python := "python"
	if _, err := exec.LookPath("python"); err != nil {
		python = "python3"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	server, err := StartServer(ctx, ServerConfig{Name: "mau", Command: python, Args: []string{script}}, http.DefaultClient)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	defer server.Close()

	names := []string{}
	for _, tool := range server.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "add,shout,fail" {
		t.Fatalf("tools = %v", names)
	}

	text, isError, err := server.Call(ctx, "add", map[string]any{"a": 2, "b": 3})
	if err != nil || isError || strings.TrimSpace(text) != "5" {
		t.Fatalf("add = %q isError=%t err=%v", text, isError, err)
	}
	text, isError, _ = server.Call(ctx, "fail", map[string]any{})
	if !isError || !strings.Contains(text, "luon loi") {
		t.Fatalf("fail = %q isError=%t", text, isError)
	}
}

func TestManagerHidesBrokenServer(t *testing.T) {
	hostExe := filepath.Join(t.TempDir(), "AxiomOffice.Host.exe")
	if err := os.WriteFile(hostExe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(t.TempDir(), "mcp.json")
	content := fmt.Sprintf(`{"mcpServers":{"hong":{"command":%q}}}`, filepath.Join(t.TempDir(), "khong-ton-tai.exe"))
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configFile, hostExe, http.DefaultClient)
	defer manager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tools := manager.Tools(ctx)
	if len(tools) != 0 {
		t.Fatalf("server loi khong duoc co tool: %d", len(tools))
	}
	if message := manager.Errors()["hong"]; message == "" {
		t.Fatalf("phai bao loi cho server hong: %v", manager.Errors())
	}
	names := []string{}
	for _, config := range manager.Configs() {
		names = append(names, config.Name)
	}
	if strings.Join(names, ",") != "office,hong" {
		t.Fatalf("configs = %v", names)
	}
}

// serverStub: stdio gia de kiem tra vong doc/ghi ma khong can python.
func TestStreamFraming(t *testing.T) {
	// Mot dong JSON mot thong diep: doc lai chinh xac nhung gi Core gui.
	message, err := encodeMessage(nil, "notifications/initialized", nil)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(message, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, hasParams := parsed["params"]; hasParams {
		t.Fatal("params rong phai duoc bo")
	}
	if parsed["jsonrpc"] != "2.0" || parsed["method"] != "notifications/initialized" {
		t.Fatalf("message = %s", message)
	}
	// Doc tung dong nhu server that.
	scanner := bufio.NewScanner(strings.NewReader(string(message) + "\n"))
	if !scanner.Scan() || scanner.Text() != string(message) {
		t.Fatal("thong diep phai nam gon trong mot dong")
	}
}
