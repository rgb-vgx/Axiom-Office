package model

import (
	"io"
	"testing"
	"time"
)

// Hinh dang that lay tu proxy 9router ngay 02/10/2026: moi frame mot dong `data:`, ngan cach bang
// dong rong, ket thuc bang `data: [DONE]`.
func TestSSEPayloads(t *testing.T) {
	body := "data: {\"a\":1}\n\ndata: {\"b\":2}\n\n: keep-alive\n\nevent: ping\ndata: [DONE]\n\n"
	got := ssePayloads([]byte(body))
	if len(got) != 2 || got[0] != `{"a":1}` || got[1] != `{"b":2}` {
		t.Fatalf("ssePayloads = %q", got)
	}

	// Chap nhan ca "data:" khong co dau cach.
	if got := ssePayloads([]byte("data:{\"x\":1}\n\n")); len(got) != 1 || got[0] != `{"x":1}` {
		t.Fatalf("data khong dau cach = %q", got)
	}

	// Nhieu dong data trong cung mot event thi noi bang '\n' theo dac ta SSE.
	if got := ssePayloads([]byte("data: a\ndata: b\n\n")); len(got) != 1 || got[0] != "a\nb" {
		t.Fatalf("nhieu dong data = %q", got)
	}

	// Body khong phai SSE (may chu bo qua stream: true) -> rong, ben goi roi ve Parse thuong.
	if got := ssePayloads([]byte(`{"choices":[{"message":{"content":"x"}}]}`)); got != nil {
		t.Fatalf("JSON thuong khong duoc coi la SSE: %q", got)
	}
}

func TestOpenAIParseStreamCollectsText(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"nghi\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\",\"delta\":{\"content\":\"!\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":36,\"completion_tokens\":9}}\n\n" +
		"data: [DONE]\n\n"
	turn, errText := (openAICodec{}).ParseStream([]byte(body))
	if errText != "" {
		t.Fatalf("loi: %s", errText)
	}
	if turn.Text != "OK!" || !turn.HasText {
		t.Fatalf("text = %q", turn.Text)
	}
	// reasoning_content KHONG duoc tron vao cau tra loi.
	if turn.FinishReason != "stop" || turn.InputTokens != 36 || turn.OutputTokens != 9 {
		t.Fatalf("turn = %+v", turn)
	}
	if len(turn.ToolCalls) != 0 {
		t.Fatalf("khong duoc co tool call: %+v", turn.ToolCalls)
	}
}

// Tool call den lam hai pha: frame dau mang id + name, cac frame sau chi mang tung manh arguments.
func TestOpenAIParseStreamAssemblesToolCall(t *testing.T) {
	body := "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\",\"tool_calls\":[{\"index\":0,\"id\":\"call_00_abc\",\"type\":\"function\",\"function\":{\"name\":\"office_action\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"action\\\":\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"et.readRange\\\"}\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"finish_reason\":\"tool_calls\",\"delta\":{}}]}\n\n" +
		"data: [DONE]\n\n"
	turn, errText := (openAICodec{}).ParseStream([]byte(body))
	if errText != "" {
		t.Fatalf("loi: %s", errText)
	}
	if turn.Text != "" || turn.HasText {
		t.Fatalf("khong duoc co text: %q", turn.Text)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("phai co dung 1 tool call: %+v", turn.ToolCalls)
	}
	call := turn.ToolCalls[0]
	if call.ID != "call_00_abc" || call.Name != "office_action" {
		t.Fatalf("id/name sai: %+v", call)
	}
	if call.Arguments != `{"action":"et.readRange"}` {
		t.Fatalf("arguments ghep sai: %q", call.Arguments)
	}
	if turn.FinishReason != "tool_calls" {
		t.Fatalf("finish_reason = %q", turn.FinishReason)
	}
}

// May chu bo qua stream: true -> body la JSON mot cuc kem duoi rac; phai roi ve Parse thuong.
func TestParseStreamFallsBackToPlainJSON(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"role":"assistant","content":"OK"}}],"usage":{"prompt_tokens":3,"completion_tokens":1}}` + "data: [DONE]")
	turn, errText := (openAICodec{}).ParseStream(body)
	if errText != "" {
		t.Fatalf("loi: %s", errText)
	}
	if turn.Text != "OK" || turn.InputTokens != 3 {
		t.Fatalf("turn = %+v", turn)
	}
}

// Frame hong chi bi bo qua, khong lam hong ca luot.
func TestParseStreamSkipsBrokenFrames(t *testing.T) {
	body := "data: {hong\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"}}]}\n\ndata: [DONE]\n\n"
	turn, errText := (openAICodec{}).ParseStream([]byte(body))
	if errText != "" {
		t.Fatalf("loi: %s", errText)
	}
	if turn.Text != "OK" {
		t.Fatalf("text = %q", turn.Text)
	}
}

func TestAnthropicParseStream(t *testing.T) {
	body := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":11,\"output_tokens\":1}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Xin\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" chao\"}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"office_action\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"action\\\":\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\"app.info\\\"}\"}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":25}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	turn, errText := (anthropicCodec{}).ParseStream([]byte(body))
	if errText != "" {
		t.Fatalf("loi: %s", errText)
	}
	if turn.Text != "Xin chao" || turn.InputTokens != 11 || turn.OutputTokens != 25 {
		t.Fatalf("turn = %+v", turn)
	}
	if turn.FinishReason != "tool_calls" {
		t.Fatalf("stop_reason phai duoc chuan hoa: %q", turn.FinishReason)
	}
	if len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != "office_action" ||
		turn.ToolCalls[0].Arguments != `{"action":"app.info"}` {
		t.Fatalf("tool call = %+v", turn.ToolCalls)
	}
}

func TestIdleReaderCancelsWhenNoBytes(t *testing.T) {
	// Doc tu mot reader khong bao gio tra ve -> watchdog phai huy context.
	reader := &blockingReader{done: make(chan struct{})}
	defer close(reader.done)
	cancelled := make(chan struct{})
	idle := newIdleReader(reader, 20*time.Millisecond, func() { close(cancelled) })
	go func() { _, _ = io.Copy(io.Discard, idle) }()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("watchdog khong huy khi khong co byte nao")
	}
	if !idle.Stalled() {
		t.Fatal("phai danh dau la bi treo")
	}
}

type blockingReader struct{ done chan struct{} }

func (r *blockingReader) Read([]byte) (int, error) {
	<-r.done
	return 0, io.EOF
}
