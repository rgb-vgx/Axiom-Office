package agent

import (
	"fmt"
	"regexp"
	"strings"

	"axiomoffice/core/internal/model"
)

// liveReasoningChars: khoi suy luan truc tiep chi mang phan cuoi (suy luan dai hang chuc nghin ky tu, gui lai moi nua
// giay); ban day du van den khi vong xong (run.reasoning).
const liveReasoningChars = 6000

var (
	liveFirst = regexp.MustCompile(`"first"\s*:\s*(\d+)`)
	liveLast  = regexp.MustCompile(`"last"\s*:\s*(\d+)`)
)

// liveProgress: mot dong trang thai cho phan hoi dang toi; rong = khong co gi dang ke.
func liveProgress(update model.LiveUpdate) string {
	args := update.ToolArgs
	switch {
	case update.ToolName == "slides_from_template":
		part := ""
		if first := liveFirst.FindStringSubmatch(args); first != nil {
			part = " phần " + first[1]
			if last := liveLast.FindStringSubmatch(args); last != nil {
				part += "–" + last[1]
			}
		}
		if slides := strings.Count(args, `"layout"`); slides > 0 {
			return fmt.Sprintf("Đang thiết kế%s: %d slide…", part, slides)
		}
		return "Đang chuẩn bị dựng slide" + part + "…"
	case update.ToolName != "" && len(args) >= 1500:
		return fmt.Sprintf("Đang viết lệnh %s (%d KB)…", update.ToolName, (len(args)+512)/1024)
	case update.ToolName != "":
		return ""
	case strings.TrimSpace(update.Text) != "":
		return "Đang viết câu trả lời…"
	case update.Reasoning != "":
		return "Đang suy nghĩ…"
	}
	return ""
}

// tailText: phan cuoi cua chu dai (giu nguyen ky tu tieng Viet), mo dau bang "…".
func tailText(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return "…" + string(runes[len(runes)-limit:])
}
