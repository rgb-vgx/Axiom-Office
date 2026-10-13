package tools

import (
	"context"
	"strings"
	"testing"
)

// write_todos: luu danh sach, phat run.todos cho pane (buoc x/y), tu choi dau vao sai.
func TestTodoToolPublishesProgress(t *testing.T) {
	var event map[string]any
	run := &RunContext{Event: func(eventType string, data map[string]any) {
		if eventType == "run.todos" {
			event = data
		}
	}}
	tool := NewTodoTool()
	result := tool.Invoke(context.Background(), map[string]any{"todos": []any{
		map[string]any{"content": "Đọc dữ liệu", "status": "completed"},
		map[string]any{"content": " Dựng bảng ", "status": "in_progress"},
		map[string]any{"content": "Vẽ biểu đồ", "status": "pending"},
	}}, run)
	if !result.OK || result.Mutating || len(run.Todos) != 3 || run.Todos[1].Content != "Dựng bảng" {
		t.Fatalf("%+v %+v", result, run.Todos)
	}
	if event["done"] != 1 || event["total"] != 3 {
		t.Fatalf("su kien: %+v", event)
	}
	for _, bad := range []any{"x", []any{map[string]any{"content": "a", "status": "done"}}, []any{map[string]any{"status": "pending"}}} {
		if result := tool.Invoke(context.Background(), map[string]any{"todos": bad}, run); result.OK || !strings.Contains(result.JSON, "error") {
			t.Fatalf("phai tu choi %v: %s", bad, result.JSON)
		}
	}
	if len(run.Todos) != 3 {
		t.Fatal("dau vao sai khong duoc xoa danh sach cu")
	}
}
