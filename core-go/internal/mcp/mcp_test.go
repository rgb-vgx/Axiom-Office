package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// stubRequest: mot request ma MCP server gia nhan duoc (method + header luc tren day).
type stubRequest struct {
	method string
	header http.Header
}

// mcpHTTPStub: MCP server gia (Streamable HTTP) tra JSON-RPC thanh cong va ghi lai header tung request.
type mcpHTTPStub struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []stubRequest
}

func newMCPHTTPStub(t *testing.T) *mcpHTTPStub {
	t.Helper()
	stub := &mcpHTTPStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var message struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &message)
		stub.mu.Lock()
		stub.requests = append(stub.requests, stubRequest{method: message.Method, header: r.Header.Clone()})
		stub.mu.Unlock()
		if message.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		result := map[string]any{}
		switch message.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": ProtocolVersion}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{
				"name": "add", "inputSchema": map[string]any{"type": "object"},
			}}}
		case "tools/call":
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "5"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": *message.ID, "result": result})
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *mcpHTTPStub) request(method string) (stubRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, request := range s.requests {
		if request.method == method {
			return request, true
		}
	}
	return stubRequest{}, false
}

func TestHTTPHeadersOnEveryRequest(t *testing.T) {
	stub := newMCPHTTPStub(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, err := StartServer(ctx, ServerConfig{
		Name: "web", URL: stub.server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer tok-123",
			"X-Api-Key":     "key-1",
			// Header giao thuc do transport tu dat: cau hinh khong duoc de len.
			"Content-Type":         "text/plain",
			"Accept":               "text/plain",
			"MCP-Protocol-Version": "1999-01-01",
			"Mcp-Session-Id":       "gia",
		},
	}, http.DefaultClient)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	defer server.Close()

	if text, isError, err := server.Call(ctx, "add", map[string]any{"a": 2, "b": 3}); err != nil || isError || text != "5" {
		t.Fatalf("add = %q isError=%t err=%v", text, isError, err)
	}

	for _, method := range []string{"initialize", "notifications/initialized", "tools/list", "tools/call"} {
		request, found := stub.request(method)
		if !found {
			t.Fatalf("server chua nhan request %q", method)
		}
		if got := request.header.Get("Authorization"); got != "Bearer tok-123" {
			t.Fatalf("%s Authorization = %q", method, got)
		}
		if got := request.header.Get("X-Api-Key"); got != "key-1" {
			t.Fatalf("%s X-Api-Key = %q", method, got)
		}
		if got := request.header.Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("%s Content-Type bi de = %q", method, got)
		}
		if got := request.header.Get("Accept"); got != "application/json, text/event-stream" {
			t.Fatalf("%s Accept bi de = %q", method, got)
		}
		if got := request.header.Get("MCP-Protocol-Version"); got != ProtocolVersion {
			t.Fatalf("%s MCP-Protocol-Version bi de = %q", method, got)
		}
		if got := request.header.Get("Mcp-Session-Id"); got != "" {
			t.Fatalf("%s Mcp-Session-Id bi de = %q", method, got)
		}
	}
}

func TestHTTPHeaderEnvReference(t *testing.T) {
	t.Setenv("AXIOM_MCP_TEST_TOKEN", "tok-abc")
	stub := newMCPHTTPStub(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, err := StartServer(ctx, ServerConfig{
		Name: "web", URL: stub.server.URL,
		Headers: map[string]string{"Authorization": "Bearer {env:AXIOM_MCP_TEST_TOKEN}"},
	}, http.DefaultClient)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	defer server.Close()

	request, found := stub.request("initialize")
	if !found {
		t.Fatal("server chua nhan initialize")
	}
	if got := request.header.Get("Authorization"); got != "Bearer tok-abc" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestHTTPHeaderMissingEnvRejected(t *testing.T) {
	_, err := StartServer(context.Background(), ServerConfig{
		Name: "web", URL: "http://127.0.0.1:1/mcp",
		Headers: map[string]string{"Authorization": "Bearer {env:AXIOM_MCP_TEST_MISSING}"},
	}, http.DefaultClient)
	if err == nil {
		t.Fatal("thieu bien moi truong phai bao loi")
	}
	if !strings.Contains(err.Error(), "AXIOM_MCP_TEST_MISSING") {
		t.Fatalf("loi phai neu ten bien moi truong: %v", err)
	}
	if strings.Contains(err.Error(), "Bearer") {
		t.Fatalf("loi khong duoc chua gia tri header: %v", err)
	}
}

func TestHTTPHeaderInvalidRejected(t *testing.T) {
	cases := map[string]map[string]string{
		"CRLF trong gia tri":  {"X-Evil": "a\r\nX-Injected: 1"},
		"CRLF trong ten":      {"X-Evil\r\nX-Injected": "1"},
		"ten co khoang trang": {"Bad Header": "1"},
		"ten rong":            {"": "1"},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := StartServer(context.Background(), ServerConfig{
				Name: "web", URL: "http://127.0.0.1:1/mcp", Headers: headers,
			}, http.DefaultClient)
			if err == nil {
				t.Fatalf("header %v phai bi tu choi", headers)
			}
		})
	}
}

// Header cua server stdio khong duoc gui di (stdio khong co header) va cung khong duoc phan giai: cau hinh
// headers tren server stdio khong lam server hong.
func TestStdioServerIgnoresHeaders(t *testing.T) {
	script := sampleServer(t)
	python := "python"
	if _, err := exec.LookPath("python"); err != nil {
		python = "python3"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server, err := StartServer(ctx, ServerConfig{
		Name: "mau", Command: python, Args: []string{script},
		Headers: map[string]string{
			"Authorization": "Bearer {env:AXIOM_MCP_TEST_MISSING}",
			"X-Evil":        "a\r\nX-Injected: 1",
		},
	}, http.DefaultClient)
	if err != nil {
		t.Fatalf("headers khong duoc anh huong server stdio: %v", err)
	}
	defer server.Close()

	if text, isError, err := server.Call(ctx, "add", map[string]any{"a": 2, "b": 3}); err != nil || isError || strings.TrimSpace(text) != "5" {
		t.Fatalf("add = %q isError=%t err=%v", text, isError, err)
	}
}

// Mot server co header hong khong duoc lam hong cac server khac.
func TestManagerKeepsGoodServerWhenHeadersInvalid(t *testing.T) {
	stub := newMCPHTTPStub(t)
	configFile := filepath.Join(t.TempDir(), "mcp.json")
	content := fmt.Sprintf(`{"mcpServers":{
		"hong":{"url":"http://127.0.0.1:1/mcp","headers":{"Authorization":"Bearer {env:AXIOM_MCP_TEST_MISSING}"}},
		"tot":{"url":%q,"headers":{"X-Api-Key":"key-1"}}}}`, stub.server.URL)
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(configFile, filepath.Join(t.TempDir(), "khong-co.exe"), http.DefaultClient)
	defer manager.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tools := manager.Tools(ctx)
	names := []string{}
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	if strings.Join(names, ",") != "mcp__tot__add" {
		t.Fatalf("tools = %v", names)
	}
	if message := manager.Errors()["hong"]; !strings.Contains(message, "AXIOM_MCP_TEST_MISSING") {
		t.Fatalf("loi server hong = %q", message)
	}
	if message := manager.Errors()["tot"]; message != "" {
		t.Fatalf("server tot khong duoc loi: %q", message)
	}
}
