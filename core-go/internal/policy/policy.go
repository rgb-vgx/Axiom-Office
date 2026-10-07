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

// Y dinh luu/xuat (tieng Viet co/khong dau + tieng Anh). Doan sai theo huong "co y dinh" la bo qua buoc hoi
// nguoi dung, nen chi tinh tu khoa dung nghia luu/xuat file: bo tu ghep ("lưu lượng", "xuất hiện", "đề xuất")
// va cau phu dinh ("đừng lưu", "không xuất PDF"). Bo sot thi chi hoi thua mot lan.
var (
	clauseSplit = regexp.MustCompile(`[,.;:!?\n\r()]+`)
	wordPattern = regexp.MustCompile(`[\p{L}\p{N}']+`)

	saveWords = map[string]bool{
		"lưu": true, "luu": true, "save": true, "saving": true, "xuất": true, "xuat": true, "export": true, "pdf": true,
	}
	// Tu dung NGAY SAU tu khoa lam no thanh tu khac nghia.
	notSaveAfter = map[string]map[string]bool{
		"lưu":  set("lượng", "ý", "hành", "thông", "vong", "niệm", "trú", "động", "loát", "manh", "vực", "huyết"),
		"luu":  set("luong", "y", "hanh", "thong", "vong", "niem", "tru", "dong", "loat", "manh", "vuc", "huyet"),
		"xuất": set("hiện", "sắc", "phát", "xứ", "khẩu", "nhập", "thân", "hành", "huyết", "chúng", "thần", "kho", "cảnh"),
		"xuat": set("hien", "sac", "phat", "xu", "khau", "nhap", "than", "hanh", "huyet", "chung", "kho", "canh"),
	}
	// Tu dung NGAY TRUOC tu khoa lam no thanh tu khac nghia.
	notSaveBefore = map[string]map[string]bool{
		"lưu":  set("giao", "hạ", "thượng", "phong", "đối", "chi", "dòng"),
		"luu":  set("giao", "ha", "thuong", "phong", "doi", "chi"),
		"xuất": set("sản", "đề", "kiết", "trích", "lối"),
		"xuat": set("san", "de", "kiet", "trich", "loi"),
	}
	// Phu dinh trong 3 tu truoc tu khoa (cung menh de). Khong dung "khong dau" cua "đừng" ("dung" = "dùng",
	// "nội dung").
	negations = set("đừng", "không", "khong", "chưa", "chua", "chẳng", "chang", "cấm", "khỏi", "khoi",
		"not", "don't", "dont", "never", "no", "without", "avoid")
)

func set(words ...string) map[string]bool {
	result := make(map[string]bool, len(words))
	for _, word := range words {
		result[word] = true
	}
	return result
}

func PromptAsksToSave(prompt string) bool {
	for _, clause := range clauseSplit.Split(strings.ToLower(prompt), -1) {
		words := wordPattern.FindAllString(clause, -1)
		for index, word := range words {
			if !saveWords[word] && !isWriteFile(words, index) {
				continue
			}
			if index+1 < len(words) && notSaveAfter[word][words[index+1]] {
				continue
			}
			if index > 0 && notSaveBefore[word][words[index-1]] {
				continue
			}
			if negated(words, index) {
				continue
			}
			return true
		}
	}
	return false
}

// isWriteFile: "ghi file" / "ghi ra file" / "ghi vào file".
func isWriteFile(words []string, index int) bool {
	if words[index] != "ghi" {
		return false
	}
	for next := index + 1; next < len(words) && next <= index+2; next++ {
		if words[next] == "file" || words[next] == "tệp" || words[next] == "tep" {
			return true
		}
	}
	return false
}

func negated(words []string, index int) bool {
	for previous := index - 1; previous >= 0 && previous >= index-3; previous-- {
		if negations[words[previous]] {
			return true
		}
	}
	return false
}

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
