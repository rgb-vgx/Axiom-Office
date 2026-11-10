package agent

import (
	"fmt"
	"strings"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/tools"
)

// maxStateNoteChars / maxStateNoteItems: giữ câu trạng thái đủ ngắn để chèn lại mỗi vòng không tốn token.
const (
	maxStateNoteChars = 500
	maxStateNoteItems = 8
)

// todoStateNote: checklist hiện tại thành một câu trạng thái ngắn. Trả "" khi chưa có checklist.
//
// Học từ Deep Agents: goal state notice được chèn lại mỗi lượt (bounded, có nhãn framework) để model yếu
// không quên bước giữa một lượt dài. Axiom đã có write_todos nhưng trước đây chỉ nằm trong kết quả tool.
func todoStateNote(todos []tools.Todo) string {
	if len(todos) == 0 {
		return ""
	}
	done, active := 0, ""
	lines := make([]string, 0, maxStateNoteItems)
	for index, todo := range todos {
		content := strings.TrimSpace(todo.Content)
		switch todo.Status {
		case "completed":
			done++
			content = "[x] " + content
		case "in_progress":
			if active == "" {
				active = content
			}
			content = "[>] " + content
		default:
			content = "[ ] " + content
		}
		if index < maxStateNoteItems {
			lines = append(lines, model.Truncate(content, 80))
		}
	}
	head := fmt.Sprintf("Checklist %d/%d done", done, len(todos))
	if active != "" {
		head += "; doing: " + model.Truncate(active, 80)
	}
	note := "Core state (not a user request): " + head + ". Steps: " + strings.Join(lines, " | ")
	if len(todos) > maxStateNoteItems {
		note += fmt.Sprintf(" | ... %d more", len(todos)-maxStateNoteItems)
	}
	return model.Truncate(note, maxStateNoteChars)
}
