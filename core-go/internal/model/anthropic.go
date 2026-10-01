package model

import (
	"bytes"
	"encoding/json"
	"strings"
)

// anthropicCodec: /messages; system la truong rieng, tool call la block tool_use, ket qua tool gui lai
// trong message role=user voi block tool_result.
type anthropicCodec struct{}

const anthropicDefaultMaxTokens = 4096

func (anthropicCodec) Name() string { return "anthropic" }

func (anthropicCodec) Path() string { return "/messages" }

func (anthropicCodec) IsToolUnsupported(errorText string) bool {
	return strings.Contains(strings.ToLower(errorText), "tool")
}

func (anthropicCodec) BuildRequest(model, systemPrompt string, turns []Message, tools []Tool, toolsEnabled bool, maxTokens int) map[string]any {
	if maxTokens <= 0 {
		maxTokens = anthropicDefaultMaxTokens
	}
	messages := append([]any{}, turns...)
	body := map[string]any{"model": model, "max_tokens": maxTokens, "system": systemPrompt, "messages": messages}
	if toolsEnabled && len(tools) > 0 {
		definitions := make([]any, 0, len(tools))
		for _, tool := range tools {
			definitions = append(definitions, map[string]any{
				"name":         tool.Name,
				"description":  tool.Description,
				"input_schema": tool.Parameters,
			})
		}
		body["tools"] = definitions
	}
	return body
}

func (anthropicCodec) Parse(body []byte) (*Turn, string) {
	var root struct {
		Content    json.RawMessage `json:"content"`
		StopReason string          `json:"stop_reason"`
		Usage      struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := decodeFirst(body, &root); err != nil {
		return nil, "invalid provider response: " + err.Error()
	}
	var blocks []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if len(root.Content) == 0 || json.Unmarshal(root.Content, &blocks) != nil || blocks == nil {
		return nil, "unexpected provider response (no content blocks)"
	}
	turn := &Turn{InputTokens: root.Usage.Input, OutputTokens: root.Usage.Output, Raw: root.Content,
		FinishReason: root.StopReason}
	var texts []string
	for _, block := range blocks {
		switch block.Type {
		case "text":
			texts = append(texts, block.Text)
		case "tool_use":
			arguments := "{}"
			if len(block.Input) > 0 && string(block.Input) != "null" {
				arguments = compactJSON(block.Input)
			}
			turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: block.ID, Name: block.Name, Arguments: arguments})
		}
	}
	if len(texts) > 0 {
		turn.Text, turn.HasText = strings.Join(texts, "\n"), true
	}
	return turn, ""
}

func (anthropicCodec) AppendAssistant(turns []Message, turn *Turn) []Message {
	if len(turn.Raw) > 0 {
		turns = append(turns, map[string]any{"role": "assistant", "content": json.RawMessage(turn.Raw)})
	}
	return turns
}

func (anthropicCodec) AppendToolResults(turns []Message, results []ToolResult) []Message {
	blocks := make([]any, 0, len(results))
	for _, result := range results {
		var content any = result.ResultJSON
		// Anh (QA thi giac, muc 8.4.6): tool_result cua Anthropic nhan khoi image base64.
		if mediaType, data, ok := SplitDataURL(result.ImageDataURL); ok {
			content = []any{
				map[string]any{"type": "text", "text": result.ResultJSON},
				map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": mediaType, "data": data}},
			}
		}
		blocks = append(blocks, map[string]any{"type": "tool_result", "tool_use_id": result.CallID, "content": content})
	}
	return append(turns, map[string]any{"role": "user", "content": blocks})
}

// SplitDataURL: "data:image/png;base64,AAAA" -> ("image/png", "AAAA").
func SplitDataURL(dataURL string) (string, string, bool) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", "", false
	}
	header, data, found := strings.Cut(dataURL[len("data:"):], ",")
	mediaType, isBase64 := strings.CutSuffix(header, ";base64")
	if !found || !isBase64 || mediaType == "" || data == "" {
		return "", "", false
	}
	return mediaType, data, true
}

// compactJSON giu nguyen thu tu khoa va so (khong di qua map) - giong JsonNode.ToJsonString cua ban .NET.
func compactJSON(raw json.RawMessage) string {
	var buffer bytes.Buffer
	if json.Compact(&buffer, raw) != nil {
		return string(raw)
	}
	return buffer.String()
}
