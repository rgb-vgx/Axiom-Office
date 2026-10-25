package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// Raw dang Anthropic (mang block): doi name/input theo ID, block khac giu nguyen; Raw la/hong thi khong dong.
func TestPatchRawToolCallsAnthropicBlocks(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"doc truoc"},{"type":"tool_use","id":"a","name":"writer.getText","input":{}},` +
		`{"type":"tool_use","id":"b","name":"t","input":{"n":1}}]`)
	out := patchRawToolCalls(raw, map[string]ToolCall{"a": {ID: "a", Name: "office_action", Arguments: `{"action":"writer.getText"}`}})
	var blocks []map[string]any
	if err := json.Unmarshal(out, &blocks); err != nil || len(blocks) != 3 {
		t.Fatalf("%s %v", out, err)
	}
	input, _ := blocks[1]["input"].(map[string]any)
	if blocks[1]["name"] != "office_action" || input["action"] != "writer.getText" || blocks[2]["name"] != "t" ||
		blocks[0]["text"] != "doc truoc" {
		t.Fatalf("%s", out)
	}
	for _, odd := range []string{``, `not json`, `"text"`} {
		if got := patchRawToolCalls(json.RawMessage(odd), map[string]ToolCall{"a": {}}); string(got) != odd {
			t.Fatalf("%q -> %q", odd, got)
		}
	}
}

func TestPatchRawToolCallsOpenAIMessage(t *testing.T) {
	raw := json.RawMessage(`{"role":"assistant","content":null,"reasoning_content":"r","tool_calls":[` +
		`{"type":"function","id":"a","function":{"name":"writer.getText","arguments":"{}"}}]}`)
	out := string(patchRawToolCalls(raw, map[string]ToolCall{"a": {ID: "a", Name: "office_action", Arguments: `{"action":"writer.getText"}`}}))
	if !strings.Contains(out, `"name":"office_action"`) || !strings.Contains(out, `"reasoning_content":"r"`) ||
		strings.Contains(out, `"name":"writer.getText"`) {
		t.Fatalf("%s", out)
	}
}
