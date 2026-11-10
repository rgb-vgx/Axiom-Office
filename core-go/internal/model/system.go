package model

import "strings"

// SystemPrompt: system prompt tách hai phần để prompt caching chỉ phải giữ phần ổn định.
//
// Học từ Deep Agents (_prompt_caching.py + cách LocalContextMiddleware chèn context động thành message
// riêng): phần ổn định (Base + quy tắc + danh mục skill + tool) không đổi giữa các lượt nên đánh dấu
// cache; phần động (tài liệu đang mở, memory, hội thoại) đứng sau và không làm hỏng cache của phần đầu.
//
// Codec OpenAI gộp hai phần thành một system message (nhiều nhà cung cấp không nhận nhiều system message);
// codec Anthropic gửi hai block, chỉ block đầu mang cache_control.
type SystemPrompt struct {
	Stable  string
	Dynamic string
}

// String: nội dung đầy đủ theo đúng thứ tự model nhìn thấy (ổn định trước, động sau).
func (p SystemPrompt) String() string {
	switch {
	case p.Stable == "":
		return p.Dynamic
	case p.Dynamic == "":
		return p.Stable
	default:
		return p.Stable + "\n\n" + p.Dynamic
	}
}

// AppendStable: thêm quy tắc ổn định (PlanRule, QuickActionRule...) vào phần được cache.
func (p SystemPrompt) AppendStable(text string) SystemPrompt {
	if strings.TrimSpace(text) == "" {
		return p
	}
	if p.Stable == "" {
		p.Stable = text
	} else {
		p.Stable += "\n\n" + text
	}
	return p
}
