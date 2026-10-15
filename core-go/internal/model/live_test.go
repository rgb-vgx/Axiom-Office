package model

import (
	"strings"
	"testing"
	"time"
)

// Phan hoi dang toi bi cat giua dong: van gom dung suy luan va tool call dang viet; bao toi da ~2 lan/giay.
func TestLiveStreamReadsOpenAIChunks(t *testing.T) {
	updates := []LiveUpdate{}
	live := newLiveStream("openai", func(update LiveUpdate) { updates = append(updates, update) })
	stream := `data: {"choices":[{"delta":{"reasoning_content":"Phần 1 có "}}]}` + "\n\n" +
		`data: {"choices":[{"delta":{"reasoning_content":"3 mục."}}]}` + "\n\n" +
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"slides_from_template","arguments":"{\"slides\":[{\"layout\""}}]}}]}` + "\n\n"
	cut := len(stream) / 3
	_, _ = live.Write([]byte(stream[:cut]))
	_, _ = live.Write([]byte(stream[cut:]))
	if len(updates) != 1 || updates[0].Reasoning == "" {
		t.Fatalf("lan bao dau: %+v", updates)
	}
	live.last = time.Now().Add(-time.Second)
	_, _ = live.Write([]byte("data: [DONE]\n\n"))
	got := updates[len(updates)-1]
	if got.Reasoning != "Phần 1 có 3 mục." || got.ToolName != "slides_from_template" || !strings.Contains(got.ToolArgs, `"layout"`) {
		t.Fatalf("cuoi: %+v", got)
	}
	live.Reset()
	if live.reasoning.Len() != 0 || live.toolName != "" {
		t.Fatal("Reset phai xoa het")
	}
}

func TestLiveStreamReadsAnthropicChunks(t *testing.T) {
	var last LiveUpdate
	live := newLiveStream("anthropic", func(update LiveUpdate) { last = update })
	_, _ = live.Write([]byte(`data: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"nghĩ"}}` + "\n" +
		`data: {"type":"content_block_start","content_block":{"type":"tool_use","name":"web_search"}}` + "\n" +
		`data: {"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"query\":"}}` + "\n"))
	if last.Reasoning != "nghĩ" || last.ToolName != "web_search" || last.ToolArgs != `{"query":` {
		t.Fatalf("%+v", last)
	}
}
