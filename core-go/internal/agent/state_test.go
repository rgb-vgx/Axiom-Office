package agent

import (
	"strings"
	"testing"

	"axiomoffice/core/internal/tools"
)

func TestTodoStateNote(t *testing.T) {
	if note := todoStateNote(nil); note != "" {
		t.Fatalf("chua co checklist thi khong chen: %q", note)
	}
	note := todoStateNote([]tools.Todo{
		{Content: "doc bang", Status: "completed"},
		{Content: "tao dashboard", Status: "in_progress"},
		{Content: "kiem tra", Status: "pending"},
	})
	for _, want := range []string{"Checklist 1/3 done", "doing: tao dashboard", "[x] doc bang", "[>] tao dashboard", "[ ] kiem tra", "not a user request"} {
		if !strings.Contains(note, want) {
			t.Fatalf("thieu %q: %s", want, note)
		}
	}

	many := make([]tools.Todo, 12)
	for index := range many {
		many[index] = tools.Todo{Content: "buoc", Status: "pending"}
	}
	if got := todoStateNote(many); len([]rune(got)) > maxStateNoteChars || !strings.Contains(got, "more") {
		t.Fatalf("phai cat bot: %d chars, %s", len([]rune(got)), got)
	}
}
