package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
)

// StartTimeout: server phai bat tay xong trong bao lau (server loi khong lam hong run).
const StartTimeout = 20 * time.Second

// ServerConfig: cau hinh mot MCP server (mcp.json theo dinh dang quen thuoc {"mcpServers": {...}}, muc 8.7).
type ServerConfig struct {
	Name     string
	Command  string
	Args     []string
	Env      map[string]string
	URL      string
	Headers  map[string]string
	Trusted  bool
	Disabled bool
	Allowed  map[string]bool
	BuiltIn  bool
}

// ToolInfo: mot tool cua server.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// Server: mot ket noi MCP da bat tay (initialize -> notifications/initialized -> tools/list).
type Server struct {
	Config ServerConfig
	Tools  []ToolInfo
	conn   transport
}

func (s *Server) Alive() bool { return s.conn.Alive() }

// StartServer bat tay voi mot server (initialize -> initialized -> tools/list, co phan trang).
func StartServer(ctx context.Context, config ServerConfig, httpClient *http.Client) (*Server, error) {
	var conn transport
	if config.URL != "" {
		conn = newHTTPTransport(config.Name, config.URL, config.Headers, httpClient)
	} else {
		directory := ""
		if dir := filepath.Dir(config.Command); dir != "" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				directory = dir
			}
		}
		stdio, err := newStdioTransport(config.Name, config.Command, config.Args, config.Env, directory)
		if err != nil {
			return nil, err
		}
		conn = stdio
	}

	client := CoreClientInfo()
	if _, err := conn.Request(ctx, "initialize", map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      client,
	}); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Notify(ctx, "notifications/initialized", nil); err != nil {
		conn.Close()
		return nil, err
	}

	tools := []ToolInfo{}
	cursor := ""
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		page, err := conn.Request(ctx, "tools/list", params)
		if err != nil {
			conn.Close()
			return nil, err
		}
		var listing struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if json.Unmarshal(page, &listing) == nil {
			for _, tool := range listing.Tools {
				if tool.Name == "" || (config.Allowed != nil && !config.Allowed[tool.Name]) {
					continue
				}
				schema := tool.InputSchema
				if len(schema) == 0 {
					schema = json.RawMessage(`{"type":"object"}`)
				}
				tools = append(tools, ToolInfo{Name: tool.Name, Description: tool.Description, InputSchema: schema})
			}
		}
		if listing.NextCursor == "" {
			break
		}
		cursor = listing.NextCursor
	}
	return &Server{Config: config, Tools: tools, conn: conn}, nil
}

// Call: tools/call -> (text noi cac content text, isError).
func (s *Server) Call(ctx context.Context, tool string, arguments any) (string, bool, error) {
	result, err := s.conn.Request(ctx, "tools/call", map[string]any{"name": tool, "arguments": argumentsOrEmpty(arguments)})
	if err != nil {
		return "", true, err
	}
	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		IsError           bool            `json:"isError"`
	}
	_ = json.Unmarshal(result, &payload)
	parts := []string{}
	for _, content := range payload.Content {
		if content.Type == "text" {
			parts = append(parts, content.Text)
			continue
		}
		name := content.Type
		if name == "" {
			name = "content"
		}
		parts = append(parts, "["+name+"]")
	}
	text := strings.Join(parts, "\n")
	if text == "" && len(payload.StructuredContent) > 0 {
		text = string(payload.StructuredContent)
	}
	return strings.TrimRight(text, "\n"), payload.IsError, nil
}

func (s *Server) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}

func argumentsOrEmpty(arguments any) any {
	if arguments == nil {
		return map[string]any{}
	}
	return arguments
}

// Manager: quan ly MCP server cua Core (muc 8.7): server built-in "office" (lan file cua Host.exe) + mcp.json
// cua nguoi dung. Khoi dong luoi o run dau tien can; loi mot server -> an tool cua no, log, khong hong run.
type Manager struct {
	configFile string
	hostExe    string
	httpClient *http.Client

	mu         sync.Mutex
	servers    map[string]*Server
	errors     map[string]string
	configs    []ServerConfig
	configSeen bool
	stamp      time.Time
}

func NewManager(configFile, hostExe string, httpClient *http.Client) *Manager {
	return &Manager{configFile: configFile, hostExe: hostExe, httpClient: httpClient,
		servers: map[string]*Server{}, errors: map[string]string{}}
}

func (m *Manager) Errors() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyOf := map[string]string{}
	for name, message := range m.errors {
		copyOf[name] = message
	}
	return copyOf
}

// Configs: cac server trong mcp.json dang bat (ke ca server chua khoi dong duoc).
func (m *Manager) Configs() []ServerConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ServerConfig{}, m.configs...)
}

// Tools: tool MCP cho mot run (khoi dong server can thiet lan dau).
func (m *Manager) Tools(ctx context.Context) []ServerTool {
	m.mu.Lock()
	m.reloadConfigIfChanged()
	configs := append([]ServerConfig{}, m.configs...)
	m.mu.Unlock()

	for _, config := range configs {
		m.ensure(ctx, config)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	list := []ServerTool{}
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		server := m.servers[name]
		for _, tool := range server.Tools {
			list = append(list, ServerTool{Server: server, Tool: tool})
		}
	}
	return list
}

func (m *Manager) ensure(ctx context.Context, config ServerConfig) {
	m.mu.Lock()
	running, found := m.servers[config.Name]
	m.mu.Unlock()
	if found && running.Alive() {
		return
	}
	if found {
		running.Close()
	}

	startCtx, cancel := context.WithTimeout(ctx, StartTimeout)
	defer cancel()
	server, err := StartServer(startCtx, config, m.httpClient)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		message := err.Error()
		if startCtx.Err() != nil && ctx.Err() == nil {
			message = "start timed out after " + itoaSeconds(StartTimeout) + "s"
		}
		m.errors[config.Name] = message
		delete(m.servers, config.Name)
		corelog.Info("mcp %s unavailable (tools hidden): %s", config.Name, message)
		return
	}
	m.servers[config.Name] = server
	delete(m.errors, config.Name)
	suffix := " (confirm required)"
	if config.Trusted {
		suffix = " (trusted)"
	}
	corelog.Info("mcp %s: %d tools%s", config.Name, len(server.Tools), suffix)
}

// reloadConfigIfChanged: mcp.json doi -> nap lai o run sau (dong cac server cu).
func (m *Manager) reloadConfigIfChanged() {
	info, err := os.Stat(m.configFile)
	stamp := time.Time{}
	if err == nil {
		stamp = info.ModTime().UTC()
	}
	if m.configSeen && stamp.Equal(m.stamp) {
		return
	}
	first := !m.configSeen
	m.configSeen, m.stamp = true, stamp

	content := ""
	if data, err := os.ReadFile(m.configFile); err == nil {
		content = string(data)
	}
	configs, problem := LoadConfigs(content, m.hostExe)
	if problem != "" {
		corelog.Info("%s", problem)
	}
	m.configs = configs
	if first {
		return
	}
	for _, server := range m.servers {
		server.Close()
	}
	m.servers = map[string]*Server{}
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, server := range m.servers {
		server.Close()
	}
	m.servers = map[string]*Server{}
}

// LoadConfigs: doc cau hinh - built-in office (tru khi "office": {"disabled": true}) + mcp.json.
func LoadConfigs(jsonText, hostExe string) ([]ServerConfig, string) {
	problem := ""
	servers := map[string]map[string]any{}
	if strings.TrimSpace(jsonText) != "" {
		var root struct {
			McpServers map[string]map[string]any `json:"mcpServers"`
		}
		if err := json.Unmarshal([]byte(jsonText), &root); err != nil {
			problem = "mcp.json is not valid JSON: " + err.Error()
		} else {
			servers = root.McpServers
		}
	}

	configs := []ServerConfig{}
	office := servers["office"]
	officeDisabled, _ := office["disabled"].(bool)
	_, officeHasCommand := office["command"]
	_, officeHasURL := office["url"]
	if hostExe != "" && exists(hostExe) && !officeDisabled && !officeHasCommand && !officeHasURL {
		configs = append(configs, ServerConfig{
			Name: "office", Command: hostExe, Args: []string{"mcp"}, Env: map[string]string{}, Headers: map[string]string{},
			Trusted: true, Allowed: OfficeFileTools, BuiltIn: true,
		})
	}

	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		server := servers[name]
		if _, hasCommand := server["command"]; !hasCommand {
			if _, hasURL := server["url"]; !hasURL {
				continue
			}
		}
		command, _ := server["command"].(string)
		url, _ := server["url"].(string)
		if command == "" && url == "" {
			continue
		}
		trusted, _ := server["trusted"].(bool)
		disabled, _ := server["disabled"].(bool)
		configs = append(configs, ServerConfig{
			Name: name, Command: command, Args: stringList(server["args"]), Env: stringMap(server["env"]),
			URL: url, Headers: stringMap(server["headers"]), Trusted: trusted, Disabled: disabled,
		})
	}

	enabled := []ServerConfig{}
	for _, config := range configs {
		if !config.Disabled && (config.Command != "" || config.URL != "") {
			enabled = append(enabled, config)
		}
	}
	return enabled, problem
}

// Lan file cua Host.exe: chi tool doc/ghi file tren dia. Tool live (word_*, ppt_* live, wps_live_*...) trung
// voi office_action nhung KHONG qua allowlist + policy xac nhan nen khong dua cho agent.
var OfficeFileTools = setOf(
	"doc_profile", "doc_get_text", "doc_find_text", "doc_extract_table", "doc_create",
	"excel_profile", "excel_read", "excel_create_sheet", "excel_copy_sheet", "excel_rename_sheet", "excel_delete_sheet",
	"excel_format_range", "excel_create_table", "excel_write", "excel_create", "excel_convert",
	"ppt_profile", "ppt_get_text", "ppt_create", "ppt_add_slide_file",
)

var OfficeReadOnlyTools = setOf(
	"doc_profile", "doc_get_text", "doc_find_text", "doc_extract_table", "excel_profile", "excel_read",
	"ppt_profile", "ppt_get_text",
)

// stringList: mang chuoi trong mcp.json (phan tu khong phai chuoi -> bo).
func stringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{}
	}
	list := []string{}
	for _, item := range items {
		if text, ok := item.(string); ok {
			list = append(list, text)
		}
	}
	return list
}

// stringMap: object chuoi trong mcp.json (env, headers).
func stringMap(value any) map[string]string {
	object, ok := value.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	result := map[string]string{}
	for key, item := range object {
		if text, ok := item.(string); ok {
			result[key] = text
			continue
		}
		result[key] = model.Truncate(model.MarshalRelaxed(item), 200)
	}
	return result
}

func setOf(names ...string) map[string]bool {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	return set
}
