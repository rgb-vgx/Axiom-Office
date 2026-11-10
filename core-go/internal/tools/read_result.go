package tools

import (
	"context"
	"encoding/json"
)

// ReadResultTool: đọc lại một kết quả tool đã bị truncate, theo offset/limit KÝ TỰ (Unicode code point,
// không phải byte) - tiếng Việt có dấu là nhiều byte, cắt theo byte sẽ vỡ giữa một ký tự.
//
// Học từ Deep Agents (_message_eviction.py / _overflow_clip.py): thay vì bỏ phần bị cắt, giữ nguyên
// kết quả và cho model đọc từng phần. Kết quả gốc chỉ nằm trong bộ nhớ của lượt (ResultStore), không
// ghi ra đĩa.
const ReadResultToolName = "read_result"

type ReadResultTool struct{}

func NewReadResultTool() *ReadResultTool { return &ReadResultTool{} }

func (t *ReadResultTool) Name() string { return ReadResultToolName }

func (t *ReadResultTool) Description() string {
	return "Read a large tool result that was truncated. When a result contains resultId, its full text is still " +
		"available: call read_result with that id and offset/limit (Unicode characters) to read the next part. Use the " +
		"nextOffset returned until truncated is false. Do not re-run the original document command just to see the " +
		"omitted part when a resultId is present."
}

func (t *ReadResultTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"id":{"type":"string","description":"resultId from a truncated result"},` +
		`"offset":{"type":"integer","description":"Unicode character offset to start at (default 0)"},` +
		`"limit":{"type":"integer","description":"maximum Unicode characters to return (default 20000, max 40000)"}},` +
		`"required":["id"]}`)
}

func (t *ReadResultTool) Invoke(_ context.Context, arguments map[string]any, run *RunContext) Result {
	id := stringArg(arguments, "id")
	if id == "" {
		return Result{JSON: ErrorJSON("'id' is required (resultId from a truncated result)"), Action: ReadResultToolName}
	}
	if run == nil || run.Results == nil {
		return Result{JSON: ErrorJSON("no stored results in this run"), Action: ReadResultToolName}
	}
	content, ok := run.Results.Get(id)
	if !ok {
		return Result{JSON: ErrorJSON("unknown resultId '" + id + "'"), Action: ReadResultToolName}
	}
	runes := []rune(content)
	total := len(runes)
	offset := intArg(arguments, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	limit := intArg(arguments, "limit", 0)
	if limit <= 0 {
		limit = ReadResultDefaultMax
	}
	if limit > ReadResultMaxLimit {
		limit = ReadResultMaxLimit
	}
	end := offset + limit
	if end > total {
		end = total
	}
	next := end
	payload := map[string]any{
		"id": id, "offset": offset, "totalChars": total, "text": string(runes[offset:end]),
		"truncated": next < total, "nextOffset": next,
	}
	return Result{JSON: OKJSON(payload), OK: true, Action: ReadResultToolName}
}
