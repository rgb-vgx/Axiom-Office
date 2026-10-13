package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// TodoTool: write_todos - model ghi danh sach viec cua mot task nhieu buoc va danh dau dan (LangChain
// TodoListMiddleware, 13/10/2026). Pane hien "buoc 3/5" qua su kien run.todos; model yeu do quen buoc giua mot luot
// dai. Moi lan goi GHI DE ca danh sach, nen chi mot lan moi phan hoi (model.AgentOptions.OncePerReply).
type TodoTool struct{}

const (
	TodoToolName = "write_todos"
	maxTodos     = 20
	maxTodoChars = 200
)

// Todo: mot viec trong danh sach.
type Todo struct {
	Content string `json:"content"`
	Status  string `json:"status"` // pending | in_progress | completed
}

func NewTodoTool() *TodoTool { return &TodoTool{} }

func (t *TodoTool) Name() string { return TodoToolName }

func (t *TodoTool) Description() string {
	return "Keep a short checklist for a task that needs 3 or more distinct steps (for example: read the data, " +
		"build a table, add a chart, format it). The user sees it as progress. Send the WHOLE list each time - it " +
		"replaces the previous one. Mark exactly one item in_progress while you work on it and mark it completed " +
		"as soon as it is done; add items you discover. Skip this tool for simple requests of one or two steps, " +
		"and call it at most once per reply. Write the items in the user's language."
}

func (t *TodoTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"todos":{"type":"array","items":{"type":"object",` +
		`"properties":{"content":{"type":"string","description":"The step, short (under 15 words)"},` +
		`"status":{"type":"string","enum":["pending","in_progress","completed"]}},"required":["content","status"]}}},` +
		`"required":["todos"]}`)
}

func (t *TodoTool) Invoke(_ context.Context, arguments map[string]any, run *RunContext) Result {
	raw, ok := arguments["todos"].([]any)
	if !ok {
		return Result{JSON: ErrorJSON("'todos' must be an array of {content, status}"), Action: TodoToolName}
	}
	if len(raw) > maxTodos {
		return Result{JSON: ErrorJSON(fmt.Sprintf("too many items (%d): keep the list to %d steps", len(raw), maxTodos)),
			Action: TodoToolName}
	}
	todos := make([]Todo, 0, len(raw))
	done, active := 0, 0
	for index, item := range raw {
		entry, _ := item.(map[string]any)
		content := stringArg(entry, "content")
		status := stringArg(entry, "status")
		if content == "" {
			return Result{JSON: ErrorJSON(fmt.Sprintf("item %d has no 'content'", index+1)), Action: TodoToolName}
		}
		switch status {
		case "completed":
			done++
		case "in_progress":
			active++
		case "pending":
		default:
			return Result{JSON: ErrorJSON(fmt.Sprintf("item %d: status must be pending, in_progress or completed", index+1)),
				Action: TodoToolName}
		}
		if runes := []rune(content); len(runes) > maxTodoChars {
			content = string(runes[:maxTodoChars])
		}
		todos = append(todos, Todo{Content: strings.TrimSpace(content), Status: status})
	}
	run.Todos = todos
	if run.Event != nil {
		run.Event("run.todos", map[string]any{"todos": todos, "done": done, "total": len(todos)})
	}
	result := map[string]any{"saved": len(todos), "completed": done}
	if active > 1 {
		result["note"] = "more than one item is in_progress; finish one before starting the next"
	}
	return Result{JSON: OKJSON(result), OK: true, Action: TodoToolName}
}
