package model

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Tool bao da xong CA viec (Final): luot ket thuc ngay voi cau tra loi cua tool, khong goi model them. Luot that
// 13/10/2026: slides_from_template xong ca bai o vong 5, model con chay 25 vong roi het ngan sach.
func TestRunAgentEndsWhenToolReportsFinal(t *testing.T) {
	server, bodies := verifyServer(t, []string{replyToolCall, fmt.Sprintf(replyText, "khong duoc toi day")})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, Mutating: true, ResultJSON: `{"ok":true}`,
				Final: true, FinalText: "Đã dựng xong 95 slide."}
		}, DefaultAgentOptions(), AgentCallbacks{})
	if !result.OK || result.Text != "Đã dựng xong 95 slide." || result.Rounds != 1 || len(bodies()) != 1 {
		t.Fatalf("phai ket thuc sau vong 1: %+v, %d request", result, len(bodies()))
	}
}

// Lan goi y het thu 5 bi chan (khong chay tool), model nhan loi va doi cach.
func TestRunAgentBlocksIdenticalCalls(t *testing.T) {
	server, _ := verifyServer(t, []string{replyToolCall})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	executed := 0
	options := DefaultAgentOptions()
	options.MaxRounds = 7
	options.Verify = false
	var blocked string
	client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			executed++
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{ToolFinished: func(result ToolResult) {
			if strings.Contains(result.ResultJSON, "blocked") {
				blocked = result.ResultJSON
			}
		}})
	if executed != DefaultMaxIdenticalCalls || blocked == "" {
		t.Fatalf("chay %d lan (muon %d), loi chan: %q", executed, DefaultMaxIdenticalCalls, blocked)
	}
}

// Ngan sach 90%: tat tool va nhac tra loi; model tra loi duoc thay vi bi cat cung o 100%.
func TestRunAgentTurnsToolsOffNearBudget(t *testing.T) {
	heavy := `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c1","function":{"name":"t","arguments":"{\"n\":%d}"}}]}}],"usage":{"prompt_tokens":95000,"completion_tokens":1}}`
	server, bodies := verifyServer(t, []string{fmt.Sprintf(heavy, 1), fmt.Sprintf(heavy, 2), fmt.Sprintf(replyText, "Đã xong phần lớn, còn thiếu slide 90-95.")})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	options := DefaultAgentOptions()
	options.Verify = false
	options.MaxTokens = 200_000 // mac dinh khong tran: ben goi dat tran thi ngan sach ba muc moi chay
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{})
	sent := bodies()
	if !result.OK || !strings.Contains(result.Text, "còn thiếu") || len(sent) != 3 {
		t.Fatalf("phai tra loi o vong 3: %+v (%d request)", result, len(sent))
	}
	if strings.Contains(sent[2], `"tools"`) || !strings.Contains(sent[2], "tools are now off") {
		t.Fatalf("vong 3 phai tat tool va co loi nhac: %s", sent[2][:min(400, len(sent[2]))])
	}
	if strings.Contains(sent[1], "tools are now off") || !strings.Contains(sent[1], `"tools"`) {
		t.Fatal("vong 2 (47% ngan sach) chua duoc tat tool")
	}
}

func TestClearOldToolResultsKeepsRecentAndNeedsEnoughGain(t *testing.T) {
	big := strings.Repeat("x", 30_000)
	turns := []Message{
		map[string]any{"role": "user", "content": "u"},
		map[string]any{"role": "tool", "tool_call_id": "1", "content": big},
		map[string]any{"role": "tool", "tool_call_id": "2", "content": big},
		map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "3", "content": big}}},
		map[string]any{"role": "tool", "tool_call_id": "4", "content": "gan nhat 1"},
		map[string]any{"role": "tool", "tool_call_id": "5", "content": "gan nhat 2"},
		map[string]any{"role": "tool", "tool_call_id": "6", "content": "gan nhat 3"},
	}
	if cleared := clearOldToolResults(turns, 3, 40_000); cleared < 80_000 {
		t.Fatalf("thu hoi %d", cleared)
	}
	if turns[1].(map[string]any)["content"] != clearedToolResult || turns[6].(map[string]any)["content"] != "gan nhat 3" {
		t.Fatal("phai don ket qua cu, giu 3 ket qua gan nhat")
	}
	block := turns[3].(map[string]any)["content"].([]any)[0].(map[string]any)
	if block["content"] != clearedToolResult {
		t.Fatal("khoi tool_result (Anthropic) cung phai duoc don")
	}
	small := []Message{map[string]any{"role": "tool", "content": "a"}, map[string]any{"role": "tool", "content": "b"},
		map[string]any{"role": "tool", "content": "c"}, map[string]any{"role": "tool", "content": "d"}}
	if clearOldToolResults(small, 3, 40_000) != 0 || small[0].(map[string]any)["content"] != "a" {
		t.Fatal("thu hoi qua it thi khong don (giu cache)")
	}
}

// Ung dung Office da dong / sap: tool bao Abort -> dung luot ngay voi loi do, khong goi model them.
func TestRunAgentStopsWhenOfficeAppIsGone(t *testing.T) {
	server, bodies := verifyServer(t, []string{replyToolCall})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: `{"ok":false}`, Abort: "Word đã đóng"}
		}, DefaultAgentOptions(), AgentCallbacks{})
	if result.OK || result.ErrorKind != "office" || result.Error != "Word đã đóng" || len(bodies()) != 1 {
		t.Fatalf("phai dung sau vong 1: %+v (%d request)", result, len(bodies()))
	}
}
