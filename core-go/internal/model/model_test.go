package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStripThoughts(t *testing.T) {
	cases := map[string]string{
		"<think>hmm</think> Xin chào":   "Xin chào",
		"<Thought>a\nb</Thought>ok":     "ok",
		"<think>a</thought> giu nguyen": "<think>a</thought> giu nguyen",
		"khong co gi":                   "khong co gi",
	}
	for input, want := range cases {
		if got := StripThoughts(input); got != want {
			t.Errorf("StripThoughts(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOpenAIParseToolCalls(t *testing.T) {
	body := `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[
		{"id":"c1","function":{"name":"office_action","arguments":"{\"action\":\"et.read\"}"}},
		{"id":"c2","function":{"name":"x","arguments":{"a":1}}}]}}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`
	turn, errText := openAICodec{}.Parse([]byte(body))
	if turn == nil {
		t.Fatal(errText)
	}
	if turn.HasText || len(turn.ToolCalls) != 2 || turn.ToolCalls[0].Arguments != `{"action":"et.read"}` ||
		turn.ToolCalls[1].Arguments != `{"a":1}` || turn.InputTokens != 7 || turn.OutputTokens != 3 {
		t.Fatalf("turn = %+v", turn)
	}
	if _, errText := (openAICodec{}).Parse([]byte(`{"choices":[]}`)); !strings.Contains(errText, "no choices[0].message") {
		t.Fatalf("errText = %q", errText)
	}
}

func TestAnthropicRoundTrip(t *testing.T) {
	body := `{"content":[{"type":"text","text":"xem"},{"type":"tool_use","id":"t1","name":"n","input":{"z":1,"a":2}}],"usage":{"input_tokens":5,"output_tokens":2}}`
	codec := anthropicCodec{}
	turn, errText := codec.Parse([]byte(body))
	if turn == nil {
		t.Fatal(errText)
	}
	if turn.Text != "xem" || turn.ToolCalls[0].Arguments != `{"z":1,"a":2}` {
		t.Fatalf("turn = %+v", turn)
	}
	turns := codec.AppendAssistant(nil, turn)
	turns = codec.AppendToolResults(turns, []ToolResult{{CallID: "t1", ResultJSON: `{"ok":true}`, ImageDataURL: "data:image/png;base64,QUJD"}})
	request := codec.BuildRequest("m", "sys", turns, nil, false, 0)
	data, _ := json.Marshal(request)
	for _, want := range []string{`"max_tokens":4096`, `"tool_use_id":"t1"`, `"media_type":"image/png"`, `"data":"QUJD"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("request missing %s: %s", want, data)
		}
	}
}

func TestParseModels(t *testing.T) {
	got := ParseModels([]byte(`{"data":[{"id":"b"},{"id":"a"},{"id":"b"},{"id":" "},{"id":3}]}`))
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("got %v", got)
	}
	if len(ParseModels([]byte(`nope`))) != 0 {
		t.Fatal("bad JSON must give an empty list")
	}
}

func TestChatRetriesTransientAndSendsAuth(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" || r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(400)
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"<think>x</think>OK"}}]}`)
	}))
	defer server.Close()
	client := NewClient(server.Client(), "openai", server.URL+"/v1", "k", "m")
	client.RetryDelays = []time.Duration{time.Millisecond}
	text, errText, ok := client.Chat(context.Background(), "s", "u", 0)
	if !ok || text != "OK" || calls.Load() != 2 {
		t.Fatalf("text=%q err=%q calls=%d", text, errText, calls.Load())
	}
}

func TestChatErrorShapes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "slow") {
			time.Sleep(300 * time.Millisecond)
		}
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"error":"bad key"}`)
	}))
	defer server.Close()
	client := NewClient(server.Client(), "openai", server.URL, "k", "m")
	if _, errText, _ := client.Chat(context.Background(), "s", "u", 0); errText != `HTTP 401: {"error":"bad key"}` {
		t.Fatalf("errText = %q", errText)
	}
	slow := NewClient(server.Client(), "openai", server.URL+"/slow", "k", "m")
	slow.RequestTimeout = 50 * time.Millisecond
	if _, errText, _ := slow.Chat(context.Background(), "s", "u", 0); !strings.HasPrefix(errText, "request timed out after") {
		t.Fatalf("errText = %q", errText)
	}
	dead := NewClient(http.DefaultClient, "openai", "http://127.0.0.1:1/v1", "", "m")
	if _, errText, _ := dead.Chat(context.Background(), "s", "u", 0); !strings.HasPrefix(errText, "HttpRequestException: ") {
		t.Fatalf("errText = %q", errText)
	}
}

func TestRunAgentToolLoopAndNudge(t *testing.T) {
	var step atomic.Int32
	replies := []string{
		`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c1","function":{"name":"t","arguments":"{}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":1}}`,
		`{"choices":[{"message":{"role":"assistant","content":""}}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"xong"}}]}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := int(step.Add(1)) - 1
		_, _ = io.WriteString(w, replies[min(index, len(replies)-1)])
	}))
	defer server.Close()
	client := NewClient(server.Client(), "openai", server.URL, "", "m")
	tools := []Tool{{Name: "t", Description: "d", Parameters: json.RawMessage(`{"type":"object"}`)}}
	executed := 0
	result := client.RunAgent(context.Background(), "sys", nil, "lam di", tools,
		func(ctx context.Context, call ToolCall) ToolResult {
			executed++
			return ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: `{"ok":true}`, OK: true}
		}, DefaultAgentOptions(), AgentCallbacks{})
	if !result.OK || result.Text != "xong" || executed != 1 || result.Rounds != 3 || result.InputTokens != 10 {
		t.Fatalf("result = %+v", result)
	}
	if fmt.Sprint(result.Transcript) == "[]" {
		t.Fatal("transcript should record the tool call and the nudge")
	}
}

func TestRunAgentCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Phai doc het body thi net/http moi phat hien client ngat ket noi (huy context cua request).
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	result := NewClient(server.Client(), "openai", server.URL, "", "m").RunAgent(ctx, "s", nil, "u", nil,
		nil, DefaultAgentOptions(), AgentCallbacks{})
	if result.OK || result.ErrorKind != "cancelled" || !result.Cancelled {
		t.Fatalf("result = %+v", result)
	}
}
