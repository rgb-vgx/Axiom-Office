package model

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// StateNote chi duoc chèn khi chuỗi đổi: checklist không đổi thì không lặp lại mỗi vòng.
func TestStateNoteInjectedOnlyWhenChanged(t *testing.T) {
	server, bodies := verifyServer(t, []string{replyToolCall, fmt.Sprintf(replyText, "xong")})
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	options := DefaultAgentOptions()
	options.Verify = false
	seen := 0
	result := client.RunAgent(context.Background(), "s", nil, "u", mutatingTools(),
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, OK: true, ResultJSON: `{"ok":true}`}
		}, options, AgentCallbacks{StateNote: func(round int) string {
			seen++
			if round <= 2 {
				return "checklist A"
			}
			return ""
		}})
	if !result.OK || result.Rounds != 2 || seen != 2 {
		t.Fatalf("luot: %+v, stateNote goi %d lan", result, seen)
	}
	sent := bodies()
	if len(sent) != 2 || !strings.Contains(sent[0], "[state] checklist A") {
		t.Fatalf("vong 1 phai co state note: %v", sent)
	}
	if strings.Count(sent[1], "[state] checklist A") != 1 {
		t.Fatalf("vong 2 khong duoc chen lap: %s", sent[1])
	}
}
