package tools

import (
	"encoding/json"
	"strings"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
)

// Buoc 2 roadmap hieu nang (con-lai muc 7): cat response doc lon truoc khi vao turns cua agent -
// `turns` append moi tool result nguyen ven. Thuat toan nam o office.Truncation de MCP server dung
// chung; o day giu ten cu cho agent (contract da nam trong mo ta tool nen khong gan Hint).
const (
	ReadHeadRows   = 200
	ReadCellBudget = 6000
	CheckIssuesCap = 100
	TailRows       = 10
)

var agentTruncation = office.Truncation{
	HeadRows: ReadHeadRows, CellBudget: ReadCellBudget, TailRows: TailRows, IssuesCap: CheckIssuesCap,
}

// TruncateBridgeResult: cat payload cua lenh doc lon (et.readRange/checkRange, wpp.listSlides/checkLayout -
// xem office.Truncation). focusSlide: slide nguoi dung dang mo, giu nguyen (0 = khong biet).
func TruncateBridgeResult(action string, result map[string]any, raw string, focusSlide int) string {
	limits := agentTruncation
	limits.FocusSlide = focusSlide
	return limits.Apply(action, result, raw)
}

// attachResultRef: nếu kết quả vừa bị cắt, giữ bản đầy đủ trong ResultStore của lượt và gắn resultId +
// hướng dẫn đọc lại bằng read_result. Học từ Deep Agents (_message_eviction.py): preview + con trỏ tới
// bản đầy đủ, thay vì bỏ hẳn phần bị cắt.
func attachResultRef(truncated, original string, run *RunContext) string {
	if run == nil || run.Results == nil || !strings.Contains(truncated, `"truncated":true`) {
		return truncated
	}
	var payload map[string]any
	if json.Unmarshal([]byte(truncated), &payload) != nil {
		return truncated
	}
	result, _ := payload["result"].(map[string]any)
	if result == nil {
		return truncated
	}
	id := run.Results.Put(original)
	if id == "" {
		return truncated
	}
	result["resultId"] = id
	result["readMore"] = "The full result is stored. Call read_result {\"id\":\"" + id +
		"\",\"offset\":0,\"limit\":20000} to read it in parts; use nextOffset until truncated is false."
	return model.MarshalRelaxed(payload)
}
