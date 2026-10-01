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

// ParseStream doc body SSE cua /chat/completions va gom cac delta thanh mot Turn.
//
// Hinh dang that (lay tu proxy 9router ngay 02/10/2026): moi frame la mot
// `chat.completion.chunk`; `choices[0].delta` co `content` va `reasoning_content` rieng. Tool call
// den lam hai pha: frame dau mang ca `id` va `function.name`, cac frame sau chi mang tung manh
// `function.arguments` - nen phai ghep theo `index`.
//
// Body khong phai SSE (may chu bo qua `stream: true`) thi roi ve Parse.
func (openAICodec) ParseStream(body []byte) (*Turn, string) {
	payloads := ssePayloads(body)
	if len(payloads) == 0 {
		return (openAICodec{}).Parse(body)
	}
	var text, reasoning strings.Builder
	type partial struct {
		id   string
		name string
		args strings.Builder
	}
	partials := map[int]*partial{}
	order := []int{}
	turn := &Turn{}
	for _, payload := range payloads {
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    *int   `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				Prompt     int `json:"prompt_tokens"`
				Completion int `json:"completion_tokens"`
				// DeepSeek bao phan doc tu cache o hai truong rieng...
				PromptCacheHit int `json:"prompt_cache_hit_tokens"`
				// ...con OpenAI bao trong prompt_tokens_details.
				PromptDetails struct {
					Cached int `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
			} `json:"usage"`
		}
		// Frame hong chi bi bo qua, khong lam hong ca luot.
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			turn.InputTokens, turn.OutputTokens = chunk.Usage.Prompt, chunk.Usage.Completion
			turn.CachedTokens = cachedFrom(chunk.Usage.PromptCacheHit, chunk.Usage.PromptDetails.Cached)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.FinishReason != "" {
			turn.FinishReason = choice.FinishReason
		}
		text.WriteString(choice.Delta.Content)
		reasoning.WriteString(choice.Delta.ReasoningContent)
		for _, call := range choice.Delta.ToolCalls {
			index := 0
			if call.Index != nil {
				index = *call.Index
			}
			entry, seen := partials[index]
			if !seen {
				entry = &partial{}
				partials[index] = entry
				order = append(order, index)
			}
			// id va name chi den mot lan; arguments den tung manh.
			if call.ID != "" {
				entry.id = call.ID
			}
			if call.Function.Name != "" && entry.name == "" {
				entry.name = call.Function.Name
			}
			entry.args.WriteString(call.Function.Arguments)
		}
	}
	turn.Text = text.String()
	turn.HasText = turn.Text != ""
	turn.Reasoning = reasoning.String()
	for _, index := range order {
		entry := partials[index]
		arguments := entry.args.String()
		if arguments == "" {
			arguments = "{}"
		}
		turn.ToolCalls = append(turn.ToolCalls, ToolCall{ID: entry.id, Name: entry.name, Arguments: arguments})
	}
	// Raw phai la MOT message assistant that (khong phai body SSE): AppendAssistant dua thang vao
	// hoi thoai gui lai cho nha cung cap. Ghep sai thi tin nhan bi bo va cac tin `role: tool` thanh
	// mo coi -> nha cung cap tra 400.
	var content any
	if turn.Text != "" {
		content = turn.Text
	}
	message := map[string]any{"role": "assistant", "content": content}
	if len(turn.ToolCalls) > 0 {
		calls := make([]any, 0, len(turn.ToolCalls))
		for _, call := range turn.ToolCalls {
			calls = append(calls, map[string]any{
				"type": "function", "id": call.ID,
				"function": map[string]any{"name": call.Name, "arguments": call.Arguments},
			})
		}
		message["tool_calls"] = calls
	}
	turn.Raw, _ = json.Marshal(message)
	return turn, ""
}

func (openAICodec) Parse(body []byte) (*Turn, string) {
	var root struct {
		Choices []struct {
			Message      json.RawMessage `json:"message"`
			FinishReason string          `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			Prompt         int `json:"prompt_tokens"`
			Completion     int `json:"completion_tokens"`
			PromptCacheHit int `json:"prompt_cache_hit_tokens"`
			PromptDetails  struct {
				Cached int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
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
		Content          json.RawMessage `json:"content"`
		ReasoningContent string          `json:"reasoning_content"`
		ToolCalls        []struct {
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
	turn := &Turn{InputTokens: root.Usage.Prompt, OutputTokens: root.Usage.Completion, Raw: raw,
		CachedTokens: cachedFrom(root.Usage.PromptCacheHit, root.Usage.PromptDetails.Cached),
		FinishReason: root.Choices[0].FinishReason, Reasoning: message.ReasoningContent}
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
