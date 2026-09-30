package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunAgentEnforcesDeadline(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		calls.Add(1)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c","function":{"name":"t","arguments":"{}"}}]}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer server.Close()

	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	options := DefaultAgentOptions()
	options.Deadline = 2 * time.Second
	result := client.RunAgent(context.Background(), "s", nil, "u",
		[]Tool{{Name: "t", Description: "d", Parameters: json.RawMessage(`{"type":"object"}`)}},
		func(ctx context.Context, call ToolCall) ToolResult {
			return ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: `{"ok":true}`, OK: true}
		}, options, AgentCallbacks{})

	if result.OK || !result.TimedOut || result.ErrorKind != "timeout" {
		t.Fatalf("phai het gio: ok=%t timedOut=%t kind=%q rounds=%d calls=%d", result.OK, result.TimedOut, result.ErrorKind, result.Rounds, calls.Load())
	}
	// Server gia tra loi tuc thi nen so vong rat lon; dieu can khang dinh la THOI GIAN bi chan
	// (khong de luot chay quay mai) va tool van duoc chay that.
	if result.Seconds > 5 {
		t.Fatalf("vuot han qua xa: %.1fs", result.Seconds)
	}
	if calls.Load() == 0 || result.Rounds == 0 {
		t.Fatalf("phai co vong va tool call: rounds=%d calls=%d", result.Rounds, calls.Load())
	}
}
