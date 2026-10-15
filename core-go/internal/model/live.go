package model

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// LiveUpdate: phan hoi cua model DANG TOI (nguoi dung 14/10/2026: "co hien phan nghi do khong?" - truoc day khung chat
// dung yen 1-2 phut moi phan, suy luan chi hien khi ca vong xong). Moi truong la TOAN BO phan da nhan cua vong.
type LiveUpdate struct {
	Reasoning string
	Text      string
	// ToolName, ToolArgs: tool call dang viet (cai cuoi cung) - vd slides_from_template voi danh sach slide dai.
	ToolName string
	ToolArgs string
}

// liveInterval: khoang cach toi thieu giua hai lan bao (pane ve lai khoi suy luan moi lan).
const liveInterval = 500 * time.Millisecond

// liveStream: doc SSE cua model trong luc no toi (io.TeeReader tu postOnce) va bao dinh ky qua emit.
type liveStream struct {
	codec string
	emit  func(LiveUpdate)

	mu        sync.Mutex
	pending   []byte
	reasoning strings.Builder
	text      strings.Builder
	toolName  string
	toolArgs  strings.Builder
	last      time.Time
	changed   bool
}

func newLiveStream(codec string, emit func(LiveUpdate)) *liveStream {
	return &liveStream{codec: codec, emit: emit}
}

// Reset: lan gui lai (thu lai sau loi tam thoi) bat dau tu dau.
func (l *liveStream) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending = nil
	l.reasoning.Reset()
	l.text.Reset()
	l.toolName = ""
	l.toolArgs.Reset()
	l.changed = false
}

func (l *liveStream) Write(chunk []byte) (int, error) {
	l.mu.Lock()
	l.pending = append(l.pending, chunk...)
	for {
		end := bytes.IndexByte(l.pending, '\n')
		if end < 0 {
			break
		}
		line := strings.TrimRight(string(l.pending[:end]), "\r")
		l.pending = l.pending[end+1:]
		if value, ok := dataField(line); ok && value != "" && value != "[DONE]" {
			l.apply(value)
		}
	}
	var update *LiveUpdate
	if l.changed && time.Since(l.last) >= liveInterval {
		l.changed, l.last = false, time.Now()
		update = &LiveUpdate{Reasoning: l.reasoning.String(), Text: l.text.String(), ToolName: l.toolName, ToolArgs: l.toolArgs.String()}
	}
	l.mu.Unlock()
	if update != nil && l.emit != nil {
		l.emit(*update)
	}
	return len(chunk), nil
}

// apply: mot frame SSE - OpenAI (choices[0].delta) hoac Anthropic (content_block_start / content_block_delta).
func (l *liveStream) apply(payload string) {
	if l.codec == "anthropic" {
		var frame struct {
			Type         string `json:"type"`
			ContentBlock struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(payload), &frame) != nil {
			return
		}
		switch {
		case frame.Type == "content_block_start" && frame.ContentBlock.Type == "tool_use":
			l.toolName = frame.ContentBlock.Name
			l.toolArgs.Reset()
		case frame.Type == "content_block_delta":
			l.reasoning.WriteString(frame.Delta.Thinking)
			l.text.WriteString(frame.Delta.Text)
			l.toolArgs.WriteString(frame.Delta.PartialJSON)
		default:
			return
		}
		l.changed = true
		return
	}
	var frame struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if json.Unmarshal([]byte(payload), &frame) != nil || len(frame.Choices) == 0 {
		return
	}
	delta := frame.Choices[0].Delta
	l.reasoning.WriteString(delta.ReasoningContent)
	l.text.WriteString(delta.Content)
	for _, call := range delta.ToolCalls {
		if call.Function.Name != "" && call.Function.Name != l.toolName {
			l.toolName = call.Function.Name
			l.toolArgs.Reset()
		}
		l.toolArgs.WriteString(call.Function.Arguments)
	}
	l.changed = true
}
