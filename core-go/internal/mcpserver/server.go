package mcpserver

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// Ban giao thuc MCP ma ban C# chap nhan, theo thu tu uu tien (McpServer.cs:21).
var supportedVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const instructions = "Office/WPS tools. File tools (doc_*, ppt_* file, excel_*) work on files on disk " +
	"without the app. Live tools talk to the Axiom Office add-in inside a running " +
	"Word/Excel/PowerPoint or WPS; call office_sessions() to find live bridges and their ports."

// version duoc gan luc build bang -ldflags "-X main.version=..." (xem cmd/axiom-core/main.go).
var version = "0.0.0"

func SetVersion(value string) {
	if strings.TrimSpace(value) != "" {
		version = value
	}
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type resultResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result"`
}

type errorResponse struct {
	JSONRPC string   `json:"jsonrpc"`
	ID      any      `json:"id"`
	Error   rpcError `json:"error"`
}

// Server: MCP server qua stdio. Moi message mot dong JSON-RPC 2.0; stdout chi danh cho giao thuc,
// log di ra stderr (xem logln).
type Server struct {
	name   string
	tools  []*Tool
	byName map[string]*Tool
}

// NewServer bo qua tool trung ten (giu ban dau tien), giong McpServer cua ban C#.
func NewServer(name string, tools []*Tool) *Server {
	server := &Server{name: name, byName: map[string]*Tool{}}
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if _, exists := server.byName[tool.Name]; exists {
			continue
		}
		server.tools = append(server.tools, tool)
		server.byName[tool.Name] = tool
	}
	return server
}

func (s *Server) ToolCount() int { return len(s.tools) }

func (s *Server) Tools() []*Tool { return s.tools }

// Run doc tung dong tu input, tra loi ra output. Tra ve ma thoat cua tien trinh.
func (s *Server) Run(input io.Reader, output io.Writer) int {
	logf("MCP server '%s' started with %d tools", s.name, len(s.tools))
	reader := bufio.NewReader(input)
	writer := bufio.NewWriter(output)
	for {
		line, err := reader.ReadString('\n')
		if line == "" && err != nil {
			break
		}
		if reply, ok := s.HandleLine(strings.TrimSpace(line)); ok {
			writer.WriteString(reply)
			writer.WriteByte('\n')
			writer.Flush()
		}
		if err != nil {
			break
		}
	}
	logf("MCP server '%s' stdin closed, exiting", s.name)
	return 0
}

// HandleLine xu ly mot dong JSON-RPC; ok=false khi khong can tra loi (notification).
func (s *Server) HandleLine(line string) (string, bool) {
	parsed, err := decode(line)
	if err != nil {
		return encode(errorResponse{JSONRPC: "2.0", Error: rpcError{Code: -32700, Message: "Parse error: " + err.Error()}}), true
	}
	if batch, isBatch := parsed.([]any); isBatch {
		replies := make([]any, 0, len(batch))
		for _, item := range batch {
			message, _ := item.(map[string]any)
			if reply, ok := s.handle(message); ok {
				replies = append(replies, reply)
			}
		}
		if len(replies) == 0 {
			return "", false
		}
		return encode(replies), true
	}
	message, _ := parsed.(map[string]any)
	reply, ok := s.handle(message)
	if !ok {
		return "", false
	}
	return encode(reply), true
}

func (s *Server) handle(message map[string]any) (any, bool) {
	if message == nil {
		return errorResponse{JSONRPC: "2.0", Error: rpcError{Code: -32600, Message: "Invalid Request"}}, true
	}
	id, hasID := message["id"]
	isRequest := hasID && id != nil
	method, hasMethod := message["method"]
	if !hasMethod || method == nil {
		// Response tu client (ta khong gui request nao) hoac message loi: bo qua.
		return nil, false
	}
	methodName := text(method)
	parameters, _ := message["params"].(map[string]any)
	switch methodName {
	case "initialize":
		return resultResponse{JSONRPC: "2.0", ID: id, Result: s.initialize(parameters)}, true
	case "ping":
		if isRequest {
			return resultResponse{JSONRPC: "2.0", ID: id, Result: newOrderedMap()}, true
		}
		return nil, false
	case "tools/list":
		return resultResponse{JSONRPC: "2.0", ID: id, Result: s.listTools()}, true
	case "tools/call":
		return s.callTool(id, parameters), true
	default:
		if strings.HasPrefix(methodName, "notifications/") || !isRequest {
			return nil, false
		}
		return errorResponse{JSONRPC: "2.0", ID: id, Error: rpcError{Code: -32601, Message: "Method not found: " + methodName}}, true
	}
}

func (s *Server) initialize(parameters map[string]any) *orderedMap {
	requested := ""
	if parameters != nil && parameters["protocolVersion"] != nil {
		requested = text(parameters["protocolVersion"])
	}
	chosen := supportedVersions[0]
	if slices.Contains(supportedVersions, requested) {
		chosen = requested
	}
	client := ""
	if parameters != nil {
		if info, ok := parameters["clientInfo"].(map[string]any); ok && info["name"] != nil {
			client = text(info["name"]) + " " + text(info["version"])
		}
	}
	logf("MCP initialize: client=%s protocol=%s -> %s", strings.TrimSpace(client), requested, chosen)

	capabilities := newOrderedMap()
	toolsCapability := newOrderedMap()
	toolsCapability.Set("listChanged", false)
	capabilities.Set("tools", toolsCapability)

	serverInfo := newOrderedMap()
	serverInfo.Set("name", s.name)
	serverInfo.Set("version", version)

	result := newOrderedMap()
	result.Set("protocolVersion", chosen)
	result.Set("capabilities", capabilities)
	result.Set("serverInfo", serverInfo)
	result.Set("instructions", instructions)
	return result
}

func (s *Server) listTools() *orderedMap {
	list := make([]any, 0, len(s.tools))
	for _, tool := range s.tools {
		entry := newOrderedMap()
		entry.Set("name", tool.Name)
		entry.Set("description", tool.Description)
		entry.Set("inputSchema", tool.InputSchema())
		list = append(list, entry)
	}
	result := newOrderedMap()
	result.Set("tools", list)
	return result
}

func (s *Server) callTool(id any, parameters map[string]any) any {
	name := ""
	if parameters != nil && parameters["name"] != nil {
		name = text(parameters["name"])
	}
	tool, exists := s.byName[name]
	if !exists {
		return errorResponse{JSONRPC: "2.0", ID: id, Error: rpcError{Code: -32602, Message: "Unknown tool: " + name}}
	}
	arguments, _ := parameters["arguments"].(map[string]any)
	body, isError := invoke(tool, arguments)
	content := newOrderedMap()
	content.Set("type", "text")
	content.Set("text", body)
	result := newOrderedMap()
	result.Set("content", []any{content})
	result.Set("isError", isError)
	return resultResponse{JSONRPC: "2.0", ID: id, Result: result}
}

// invoke chay handler, doi moi loi (ke ca panic cua ToolArgs) thanh {"ok":false,"error":...}.
func invoke(tool *Tool, arguments map[string]any) (body string, isError bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			body = toolErrorBody(panicMessage(recovered))
			isError = true
			logf("MCP tool %s error: %s", tool.Name, panicMessage(recovered))
		}
	}()
	result, err := tool.Handler(NewToolArgs(arguments))
	if err != nil {
		logf("MCP tool %s error: %s", tool.Name, err.Error())
		return toolErrorBody(err.Error()), true
	}
	if asText, ok := result.(string); ok {
		return asText, false
	}
	return encode(result), false
}

// toolErrorBody giu dung thu tu "ok" roi "error" nhu Dictionary cua ban C#.
func toolErrorBody(message string) string {
	body := newOrderedMap()
	body.Set("ok", false)
	body.Set("error", message)
	return encode(body)
}

func panicMessage(recovered any) string {
	switch typed := recovered.(type) {
	case argError:
		return typed.message
	case error:
		return typed.Error()
	default:
		return fmt.Sprint(typed)
	}
}

// logf ghi ra stderr: stdout chi danh cho JSON-RPC.
func logf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, "[mcp] "+format+"\n", values...)
}
