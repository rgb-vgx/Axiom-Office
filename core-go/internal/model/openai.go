package model

import (
	"bytes"
	"encoding/json"
	"strings"
)

// openAICodec: /chat/completions; tools dang {"type":"function","function":{...}},
// ket qua tool la message {"role":"tool","tool_call_id":...}.
type openAICodec struct{}

func (openAICodec) Name() string { return "openai" }

func (openAICodec) Path() string { return "/chat/completions" }

func (openAICodec) IsToolUnsupported(errorText string) bool {
	return strings.Contains(strings.ToLower(errorText), "tool")
}

func (openAICodec) BuildRequest(model, systemPrompt string, turns []Message, tools []Tool, toolsEnabled bool, maxTokens int) map[string]any {
	messages := make([]any, 0, len(turns)+1)
	messages = append(messages, map[string]any{"role": "system", "content": systemPrompt})
	messages = append(messages, turns...)
	body := map[string]any{"model": model, "messages": messages}
	if toolsEnabled && len(tools) > 0 {
		definitions := make([]any, 0, len(tools))
		for _, tool := range tools {
			definitions = append(definitions, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  tool.Parameters,
				},
			})
		}
		body["tools"] = definitions
	}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	return body
}

func (openAICodec) Parse(body []byte) (*Turn, string) {
	var root struct {
		Choices []struct {
			Message json.RawMessage `json:"message"`
		} `json:"choices"`
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := decodeFirst(body, &root); err != nil {
		return nil, "invalid provider response: " + err.Error()
	}
	if len(root.Choices) == 0 || len(root.Choices[0].Message) == 0 || string(root.Choices[0].Message) == "null" {
		return nil, "unexpected provider response (no choices[0].message)"
	}
	raw := root.Choices[0].Message
	var message struct {
		Content   json.RawMessage `json:"content"`
		ToolCalls []struct {
			ID       string `json:"id"`
			Function struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal(raw, &message); err != nil {
		return nil, "invalid provider response: " + err.Error()
	}
	turn := &Turn{InputTokens: root.Usage.Prompt, OutputTokens: root.Usage.Completion, Raw: raw}
	var text string
	if json.Unmarshal(message.Content, &text) == nil && len(message.Content) > 0 && string(message.Content) != "null" {
		turn.Text, turn.HasText = text, true
	}
	for _, call := range message.ToolCalls {
		arguments := "{}"
		var asString string
		switch {
		case json.Unmarshal(call.Function.Arguments, &asString) == nil:
			arguments = asString
		case len(bytes.TrimSpace(call.Function.Arguments)) > 0 && string(call.Function.Arguments) != "null":
			// Mot so may chu tuong thich tra arguments la object thay vi chuoi JSON.
			arguments = string(call.Function.Arguments)
		}
		turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: arguments})
	}
	return turn, ""
}

func (openAICodec) AppendAssistant(turns []Message, turn *Turn) []Message {
	if len(turn.Raw) > 0 && bytes.HasPrefix(bytes.TrimSpace(turn.Raw), []byte("{")) {
		turns = append(turns, json.RawMessage(turn.Raw))
	}
	return turns
}

func (openAICodec) AppendToolResults(turns []Message, results []ToolResult) []Message {
	for _, result := range results {
		turns = append(turns, map[string]any{"role": "tool", "tool_call_id": result.CallID, "content": result.ResultJSON})
	}
	// Tin "tool" chi nhan text: anh (QA thi giac) gui kem bang mot tin user co image_url NGAY SAU cac ket qua tool.
	for _, result := range results {
		if result.ImageDataURL == "" {
			continue
		}
		turns = append(turns, map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": "Screenshot returned by " + result.Name + " (inspect the layout):"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": result.ImageDataURL}},
			},
		})
	}
	return turns
}
