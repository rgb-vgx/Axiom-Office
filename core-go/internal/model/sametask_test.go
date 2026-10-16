package model

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Loop "cung mot viec, khac tham so" (17-R0): 137 lan writer.replaceAll voi find khac nhau
// phai bi chan truoc khi dot token, du khong lan nao y het.
func TestRunAgentBlocksSameTaskLoop(t *testing.T) {
	officeTools := []Tool{{Name: "office_action", Description: "d", Parameters: nil}}
	makeReply := func(n int) string {
		args := fmt.Sprintf(`{"action":"writer.replaceAll","params":{"find":"muc %d","replace":"x"}}`, n)
		return fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c%d","function":{"name":"office_action","arguments":%q}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":1}}`, n, args)
	}
	replies := []string{}
	for i := 1; i <= 30; i++ {
		replies = append(replies, makeReply(i))
	}
	replies = append(replies, fmt.Sprintf(replyText, "da xong"))
	server, _ := verifyServer(t, replies)
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	executed := 0
	var blocked string
	options := DefaultAgentOptions()
	options.MaxRounds = 40
	options.Verify = false
	options.MaxSameTaskCalls = 10
	client.RunAgent(context.Background(), "s", nil, "u", officeTools,
		func(ctx context.Context, call ToolCall) ToolResult {
			executed++
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{ToolFinished: func(result ToolResult) {
			if strings.Contains(result.ResultJSON, "already ran too many times") {
				blocked = result.ResultJSON
			}
		}})
	if executed != 10 || blocked == "" {
		t.Fatalf("phai chay 10 lan roi chan (thuc te %d, loi chan: %q)", executed, blocked)
	}
	if !strings.Contains(blocked, "writer.replaceAll") || !strings.Contains(blocked, "replaceMany") {
		t.Fatalf("loi chan phai neu lenh va goi y gom lai: %q", blocked)
	}
}

// Ghi du lieu theo khoi duoc mien: 30 lan et.writeRange khac range khong bi chan.
func TestSameTaskGuardExemptsChunkedWrites(t *testing.T) {
	officeTools := []Tool{{Name: "office_action", Description: "d", Parameters: nil}}
	makeReply := func(n int) string {
		args := fmt.Sprintf(`{"action":"et.writeRange","params":{"range":"A%d:A%d"}}`, n, n+99)
		return fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c%d","function":{"name":"office_action","arguments":%q}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":1}}`, n, args)
	}
	replies := []string{}
	for i := 1; i <= 30; i++ {
		replies = append(replies, makeReply(i))
	}
	replies = append(replies, fmt.Sprintf(replyText, "da xong"))
	server, _ := verifyServer(t, replies)
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	executed := 0
	options := DefaultAgentOptions()
	options.MaxRounds = 35
	options.Verify = false
	options.MaxSameTaskCalls = 10
	client.RunAgent(context.Background(), "s", nil, "u", officeTools,
		func(ctx context.Context, call ToolCall) ToolResult {
			executed++
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{})
	if executed != 30 {
		t.Fatalf("ghi theo khoi khong duoc chan (thuc te %d/30)", executed)
	}
}

// Viet tai lieu tung phan duoc mien: 30 lan writer.heading khac text khong bi chan (luot 16/10/2026
// bi chan 37 lan giua pha viet lai tai lieu - xem guards.go sameTaskExempt).
func TestSameTaskGuardExemptsDocumentBuild(t *testing.T) {
	officeTools := []Tool{{Name: "office_action", Description: "d", Parameters: nil}}
	makeReply := func(n int) string {
		args := fmt.Sprintf(`{"action":"writer.heading","params":{"level":1,"text":"PHAN %d"}}`, n)
		return fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c%d","function":{"name":"office_action","arguments":%q}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":1}}`, n, args)
	}
	replies := []string{}
	for i := 1; i <= 30; i++ {
		replies = append(replies, makeReply(i))
	}
	replies = append(replies, fmt.Sprintf(replyText, "da xong"))
	server, _ := verifyServer(t, replies)
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	executed := 0
	options := DefaultAgentOptions()
	options.MaxRounds = 35
	options.Verify = false
	options.MaxSameTaskCalls = 10
	client.RunAgent(context.Background(), "s", nil, "u", officeTools,
		func(ctx context.Context, call ToolCall) ToolResult {
			executed++
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{})
	if executed != 30 {
		t.Fatalf("viet tai lieu tung phan khong duoc chan (thuc te %d/30)", executed)
	}
}

// Tat guard khi MaxSameTaskCalls = 0 (tuong thich cu).
func TestSameTaskKeyUnit(t *testing.T) {
	cases := []struct{ args, want string }{
		{`{"action":"writer.replaceAll","params":{"find":"a"}}`, "office_action writer.replaceAll replace"},
		{`{"action":"et.readRange","params":{"range":"A1:B2"}}`, "office_action et.readRange read"},
		{`{"action":"et.writeRange","params":{"range":"A1"}}`, ""},
		{`{"action":"writer.heading","params":{"text":"PHAN 1"}}`, ""},
		{`{"action":"writer.appendText","params":{"text":"x"}}`, ""},
		{`{"action":"writer.insertStyledText","params":{"text":"x"}}`, ""},
		{`{"action":"wpp.addSlideFromTemplate","params":{"tag":"s1"}}`, ""},
		{`not-json`, "office_action ?"},
		{`{}`, "office_action ?"},
	}
	for _, c := range cases {
		if got := sameTaskKey(ToolCall{Name: "office_action", Arguments: c.args}); got != c.want {
			t.Fatalf("args %s: got %q want %q", c.args, got, c.want)
		}
	}
	if got := sameTaskKey(ToolCall{Name: "tavily_search", Arguments: "{}"}); got != "tavily_search" {
		t.Fatalf("tool thuong lay ten tool: %q", got)
	}
}

// heavyRunGuard (17-R0).
func TestHeavyRunGuardAsksAtRound30(t *testing.T) {
	server, _ := verifyServer(t, []string{replyToolCall})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	asked := 0
	options := DefaultAgentOptions()
	options.MaxRounds = 50
	options.Verify = false
	options.MaxSameTaskCalls = 0
	options.MaxIdenticalCalls = 0
	cb := AgentCallbacks{AskContinue: func(round, billable int, reason string) bool { asked++; return false }}
	ex := func(ctx context.Context, call ToolCall) ToolResult { return ToolResult{CallID: call.ID, Name: call.Name, OK: true} }
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(), ex, options, cb)
	if result.OK || result.ErrorKind != "cancelled" { t.Fatal("phai dung") }
	if asked != 1 || result.Rounds != 30 { t.Fatal("phai hoi 1 lan o vong 30") }
}

// Dong y thi chay tiep het vong; nil thi cu chay nhu cu.
func TestHeavyRunGuardContinuesWhenApproved(t *testing.T) {
	server, _ := verifyServer(t, []string{replyToolCall})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	options := DefaultAgentOptions()
	options.MaxRounds = 35
	options.Verify = false
	options.MaxSameTaskCalls = 0
	options.MaxIdenticalCalls = 0
	asked := 0
	cb := AgentCallbacks{AskContinue: func(round, billable int, reason string) bool { asked++; return true }}
	ex := func(ctx context.Context, call ToolCall) ToolResult { return ToolResult{CallID: call.ID, Name: call.Name, OK: true} }
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(), ex, options, cb)
	if result.Rounds != 35 || asked != 1 { t.Fatal("dong y thi chay het") }
}
