package mcpserver

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// echoTool: tool gia de kiem tra duong thanh cong / loi / tham so bat buoc.
func echoTool() *Tool {
	return NewTool("echo", "Echo back the arguments.",
		[]Param{
			ParamStr("text", "Text to echo.", true, nil),
			ParamInt("count", "How many times.", false, 1),
			ParamBool("shout", "Uppercase?", false, false),
		},
		func(a *ToolArgs) (any, error) {
			if a.Bool("fail", false) {
				return nil, errors.New("boom")
			}
			return encode(map[string]any{"text": a.Req("text"), "count": a.Int("count", 1), "shout": a.Bool("shout", false)}), nil
		})
}

func testServer(t *testing.T) *Server {
	t.Helper()
	return NewServer("office-tools", []*Tool{echoTool()})
}

func send(t *testing.T, server *Server, line string) map[string]any {
	t.Helper()
	reply, ok := server.HandleLine(line)
	if !ok {
		t.Fatalf("khong co tra loi cho %s", line)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(reply), &parsed); err != nil {
		t.Fatalf("tra loi khong phai JSON: %v (%s)", err, reply)
	}
	return parsed
}

func result(t *testing.T, server *Server, line string) map[string]any {
	t.Helper()
	parsed := send(t, server, line)
	value, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatalf("thieu result: %s", encode(parsed))
	}
	return value
}

func TestInitializeEchoesSupportedVersion(t *testing.T) {
	server := testServer(t)
	value := result(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if value["protocolVersion"] != "2025-06-18" {
		t.Fatalf("phai echo dung phien ban client gui: %v", value["protocolVersion"])
	}
	info := value["serverInfo"].(map[string]any)
	if info["name"] != "office-tools" || info["version"] == "" {
		t.Fatalf("serverInfo sai: %v", info)
	}
	if !strings.Contains(value["instructions"].(string), "office_sessions") {
		t.Fatal("instructions phai nhac office_sessions")
	}
	capabilities := value["capabilities"].(map[string]any)
	if capabilities["tools"].(map[string]any)["listChanged"] != false {
		t.Fatal("capabilities.tools.listChanged phai la false")
	}
}

func TestInitializeFallsBackToNewest(t *testing.T) {
	server := testServer(t)
	for _, request := range []string{`{"id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
		`{"id":1,"method":"initialize","params":{}}`, `{"id":1,"method":"initialize"}`} {
		value := result(t, server, request)
		if value["protocolVersion"] != supportedVersions[0] {
			t.Fatalf("phai lui ve ban moi nhat: %v", value["protocolVersion"])
		}
	}
}

func TestPingAndNotificationsAreSilent(t *testing.T) {
	server := testServer(t)
	if value := result(t, server, `{"id":7,"method":"ping"}`); len(value) != 0 {
		t.Fatalf("ping phai tra object rong: %v", value)
	}
	for _, line := range []string{`{"method":"ping"}`, `{"method":"notifications/initialized"}`} {
		if _, ok := server.HandleLine(line); ok {
			t.Fatalf("notification phai im lang: %s", line)
		}
	}
}

func TestUnknownMethodAndTool(t *testing.T) {
	server := testServer(t)
	parsed := send(t, server, `{"id":3,"method":"resources/list"}`)
	code := parsed["error"].(map[string]any)["code"].(float64)
	if code != -32601 {
		t.Fatalf("method la phai -32601, nhan %v", code)
	}
	parsed = send(t, server, `{"id":4,"method":"tools/call","params":{"name":"missing"}}`)
	code = parsed["error"].(map[string]any)["code"].(float64)
	if code != -32602 {
		t.Fatalf("tool la phai -32602, nhan %v", code)
	}
	if message := parsed["error"].(map[string]any)["message"].(string); message != "Unknown tool: missing" {
		t.Fatalf("thong bao sai: %s", message)
	}
}

func TestCallToolSuccessAndError(t *testing.T) {
	server := testServer(t)
	value := result(t, server, `{"id":5,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi","count":2}}}`)
	if value["isError"] != false {
		t.Fatalf("goi thanh cong thi isError=false: %v", value)
	}
	content := value["content"].([]any)[0].(map[string]any)
	if content["type"] != "text" {
		t.Fatalf("content phai la text: %v", content)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(content["text"].(string)), &payload); err != nil {
		t.Fatalf("text phai la JSON: %v", err)
	}
	if payload["text"] != "hi" || payload["count"].(float64) != 2 || payload["shout"] != false {
		t.Fatalf("ket qua sai: %v", payload)
	}

	// Loi trong tool -> van la result (khong phai loi JSON-RPC) voi isError=true.
	value = result(t, server, `{"id":6,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi","fail":true}}}`)
	if value["isError"] != true {
		t.Fatalf("loi tool phai isError=true: %v", value)
	}
	content = value["content"].([]any)[0].(map[string]any)
	if content["text"] != `{"ok":false,"error":"boom"}` {
		t.Fatalf("text loi sai: %v", content["text"])
	}
}

func TestMissingRequiredArgumentIsToolError(t *testing.T) {
	server := testServer(t)
	value := result(t, server, `{"id":8,"method":"tools/call","params":{"name":"echo","arguments":{}}}`)
	if value["isError"] != true {
		t.Fatalf("thieu tham so bat buoc phai isError=true: %v", value)
	}
	text := value["content"].([]any)[0].(map[string]any)["text"].(string)
	if text != `{"ok":false,"error":"missing required argument 'text'"}` {
		t.Fatalf("thong bao sai: %s", text)
	}
}

func TestBatchAndParseErrors(t *testing.T) {
	server := testServer(t)
	// Batch toan notification -> khong tra loi gi.
	if _, ok := server.HandleLine(`[{"method":"notifications/initialized"}]`); ok {
		t.Fatal("batch chi gom notification phai im lang")
	}
	reply, ok := server.HandleLine(`[{"id":1,"method":"ping"},{"method":"notifications/initialized"}]`)
	if !ok {
		t.Fatal("batch co request phai tra loi")
	}
	var batch []map[string]any
	if err := json.Unmarshal([]byte(reply), &batch); err != nil {
		t.Fatalf("batch phai la mang JSON: %v (%s)", err, reply)
	}
	if len(batch) != 1 || batch[0]["id"].(float64) != 1 {
		t.Fatalf("batch phai chi tra loi request: %v", batch)
	}

	parsed := send(t, server, `{"id":1,`)
	if code := parsed["error"].(map[string]any)["code"].(float64); code != -32700 {
		t.Fatalf("JSON hong phai -32700, nhan %v", code)
	}
	if parsed["id"] != nil {
		t.Fatalf("loi parse phai co id=null: %v", parsed["id"])
	}
	parsed = send(t, server, `5`)
	if code := parsed["error"].(map[string]any)["code"].(float64); code != -32600 {
		t.Fatalf("message khong phai object phai -32600, nhan %v", code)
	}
}

func TestListToolsSchema(t *testing.T) {
	server := testServer(t)
	value := result(t, server, `{"id":9,"method":"tools/list"}`)
	tools := value["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("phai co 1 tool: %v", tools)
	}
	schema := tools[0].(map[string]any)["inputSchema"].(map[string]any)
	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "text" {
		t.Fatalf("required sai: %v", required)
	}
	properties := schema["properties"].(map[string]any)
	if properties["text"].(map[string]any)["type"] != "string" {
		t.Fatalf("type cua text sai: %v", properties["text"])
	}
	if _, exists := properties["text"].(map[string]any)["default"]; exists {
		t.Fatal("tham so khong co mac dinh thi khong duoc ghi \"default\"")
	}
	// Mac dinh false va 0 van phai duoc ghi ra (khac null).
	if properties["shout"].(map[string]any)["default"] != false {
		t.Fatalf("default false phai duoc ghi: %v", properties["shout"])
	}
	if properties["count"].(map[string]any)["default"].(float64) != 1 {
		t.Fatalf("default 1 phai duoc ghi: %v", properties["count"])
	}
}

func TestSchemaUnionTypeAndOrder(t *testing.T) {
	schema := Schema([]Param{
		ParamStr("path", "Path.", true, nil),
		ParamAny("styles", "Style.", true, "object", "array"),
	})
	rendered := encode(schema)
	if !strings.Contains(rendered, `"required":["path","styles"]`) {
		t.Fatalf("required phai giu thu tu khai bao: %s", rendered)
	}
	if !strings.Contains(rendered, `"type":["object","array"]`) {
		t.Fatalf("kieu ghep phai la mang: %s", rendered)
	}
	// "required" vang mat khi khong co tham so bat buoc.
	if rendered := encode(Schema([]Param{ParamStr("a", "", false, nil)})); strings.Contains(rendered, "required") {
		t.Fatalf("khong duoc co required rong: %s", rendered)
	}
}

func TestServerNamePerGroup(t *testing.T) {
	for group, expected := range map[string]string{"all": "office-tools", "word": "word-tools", "powerpoint": "powerpoint-tools"} {
		if _, ok := Catalog(group); !ok {
			t.Fatalf("nhom %s phai hop le", group)
		}
		name := group + "-tools"
		if group == "all" {
			name = "office-tools"
		}
		if name != expected {
			t.Fatalf("ten server cua nhom %s sai: %s", group, name)
		}
	}
	if _, ok := Catalog("publisher"); ok {
		t.Fatal("nhom la phai bi tu choi")
	}
}
