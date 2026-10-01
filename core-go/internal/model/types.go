// Package model: goi LLM (OpenAI-compatible va Anthropic) va vong lap agent - ban Go cua Models/*.cs.
// Chuoi loi giu NGUYEN DANG ban .NET ("HTTP 401: ...", "request timed out after Ns", "HttpRequestException: ...")
// de setup.Describe va ben goi (pane, audit) doc nhu nhau.
package model

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Tool ma model duoc thay trong mot luot chay (New_arch.md muc 8.3).
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// ToolCall: model yeu cau goi mot tool. Arguments la JSON (chuoi).
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ToolResult: ket qua gui lai model. OK = false khi tool loi (khong lam hong luot chay).
// ImageDataURL: anh (data:image/png;base64,...) tool tra kem - codec gui cho model duoi dang anh that.
type ToolResult struct {
	CallID       string
	Name         string
	ResultJSON   string
	OK           bool
	Ms           int64
	ImageDataURL string
}

// Turn: mot luot tra loi cua model (co tool call hoac cau tra loi cuoi). Raw giu nguyen phan assistant
// de gui lai y nguyen o vong sau.
type Turn struct {
	Text         string
	HasText      bool
	ToolCalls    []ToolCall
	InputTokens  int
	OutputTokens int
	Raw          json.RawMessage

	// FinishReason la ly do may chu dung lai ("stop", "tool_calls", "length"...). Rong khi may chu
	// khong tra. Dung de giai thich ca "model tra loi rong": het ngan sach token giua phan suy luan
	// thi content rong va finish_reason = "length".
	FinishReason string
}

// ConversationTurn: mot luot cu (bang messages) dua vao ngu canh.
type ConversationTurn struct {
	Role    string
	Content string
}

// Message: mot phan tu lich su hoi thoai theo dinh dang cua codec (map hoac JSON tho).
type Message = any

// Codec cua tung giao thuc LLM (IProviderCodec).
type Codec interface {
	Name() string
	Path() string
	IsToolUnsupported(errorText string) bool
	BuildRequest(model, systemPrompt string, turns []Message, tools []Tool, toolsEnabled bool, maxTokens int) map[string]any
	Parse(body []byte) (*Turn, string)
	AppendAssistant(turns []Message, turn *Turn) []Message
	AppendToolResults(turns []Message, results []ToolResult) []Message
}

func NewCodec(provider string) Codec {
	if provider == "anthropic" {
		return anthropicCodec{}
	}
	return openAICodec{}
}

var thoughtBlock = regexp.MustCompile(`(?i)<(thought|think|thinking)>[\s\S]*?</(thought|think|thinking)>`)

// StripThoughts bo <thought>/<think>/<thinking>...</...> mot so model tra kem trong content (ModelText.cs).
// RE2 khong co backreference: kiem tra the mo/dong trung ten bang tay.
func StripThoughts(text string) string {
	if text == "" {
		return text
	}
	stripped := thoughtBlock.ReplaceAllStringFunc(text, func(block string) string {
		match := thoughtBlock.FindStringSubmatch(block)
		if len(match) == 3 && strings.EqualFold(match[1], match[2]) {
			return ""
		}
		return block
	})
	return strings.TrimSpace(stripped)
}

// Truncate cat chuoi qua dai (dem theo rune de khong cat giua ky tu tieng Viet) va them "...".
func Truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

// ErrorJSON: {"ok":false,"error":"..."} khong escape ky tu (giu thong bao doc duoc nguyen van).
func ErrorJSON(message string) string {
	return MarshalRelaxed(map[string]any{"ok": false, "error": message})
}

// MarshalRelaxed: JSON khong escape <, >, & (giong UnsafeRelaxedJsonEscaping cua ban .NET).
func MarshalRelaxed(value any) string {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "null"
	}
	return strings.TrimRight(builder.String(), "\n")
}
