// Package policy: quy tac xac nhan truoc lenh rui ro (New_arch.md muc 8.6) - ban Go cua
// Agent/PolicyEngine.cs. Nam rieng mot goi de tools va agent cung dung ma khong vong import.
package policy

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"

	"axiomoffice/core/internal/office"
)

// LargeDocumentChars: writer.replaceAll tren tai lieu dai hon nguong nay thi phai hoi.
const LargeDocumentChars = 20_000

// Decision: can hoi nguoi dung truoc khi chay hay khong.
type Decision struct {
	NeedsConfirmation bool
	Reason            string
}

var Allow = Decision{}

func Ask(reason string) Decision { return Decision{NeedsConfirmation: true, Reason: reason} }

func IsSaveOrExport(action string) bool {
	return strings.HasSuffix(action, ".save") || strings.HasSuffix(action, ".saveAs") || strings.HasSuffix(action, ".exportPdf")
}

// Tu khoa y dinh luu/xuat (tieng Viet co/khong dau + tieng Anh).
var saveIntent = regexp.MustCompile(`(?i)(lưu|luu|save|xuất|xuat|export|pdf|ghi\s*file|ghi\s*ra\s*file)`)

func PromptAsksToSave(prompt string) bool { return saveIntent.MatchString(prompt) }

// Evaluate: allowlist da loc truoc do; day la cac truong hop con lai phai hoi nguoi dung.
func Evaluate(ctx context.Context, action string, params map[string]any, prompt string,
	documentLength func(context.Context) *int) Decision {
	if IsSaveOrExport(action) && !PromptAsksToSave(prompt) {
		return Ask("AI muốn lưu/xuất file dù yêu cầu của bạn không nhắc tới việc lưu")
	}

	if strings.HasSuffix(action, ".saveAs") || strings.HasSuffix(action, ".exportPdf") {
		if path, ok := params["path"].(string); ok && strings.TrimSpace(path) != "" {
			if _, err := os.Stat(strings.TrimSpace(path)); err == nil {
				return Ask("Ghi đè file đã có: " + strings.TrimSpace(path))
			}
		}
	}

	if action == "wpp.deleteSlide" {
		return Ask("Xoá slide khỏi bài thuyết trình")
	}

	if action == "writer.replaceAll" {
		if length := documentLength(ctx); length != nil && *length > LargeDocumentChars {
			return Ask("Thay thế toàn bộ trong tài liệu dài (" + groupDigits(*length) + " ký tự)")
		}
	}

	return Allow
}

// WordLength: do dai tai lieu Word qua bridge (writer.getText maxChars=1 tra totalChars).
func WordLength(ctx context.Context, bridge *office.BridgeClient, port int) *int {
	result := bridge.Command(ctx, port, "writer.getText", map[string]any{"maxChars": 1})
	if !result.OK {
		return nil
	}
	value, ok := result.Result["totalChars"]
	if !ok {
		return nil
	}
	switch number := value.(type) {
	case float64:
		length := int(number)
		return &length
	case int:
		length := number
		return &length
	case int64:
		length := int(number)
		return &length
	}
	return nil
}

// groupDigits: 20000 -> "20,000" (giong "{length:N0}" cua ban .NET).
func groupDigits(value int) string {
	digits := strconv.Itoa(value)
	negative := strings.HasPrefix(digits, "-")
	digits = strings.TrimPrefix(digits, "-")
	var builder strings.Builder
	if negative {
		builder.WriteByte('-')
	}
	for index, char := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			builder.WriteByte(',')
		}
		builder.WriteRune(char)
	}
	return builder.String()
}
