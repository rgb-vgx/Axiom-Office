package tools

import "axiomoffice/core/internal/office"

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
