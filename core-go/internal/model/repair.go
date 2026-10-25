package model

import "encoding/json"

// repairTurnToolCalls: chay AgentOptions.RepairToolCall tren tung tool call va ghi lai Turn.Raw cho khop.
// AppendAssistant gui lai Raw (message tho cua nha cung cap), KHONG phai ToolCalls - chi sua ToolCalls thi lich su
// van mang ten sai, lan sau model thay minh "da goi" writer.getText va ket qua tool tro vao mot cuoc goi khong khop.
func repairTurnToolCalls(turn *Turn, repair func(call ToolCall) (ToolCall, bool)) int {
	fixed := map[string]ToolCall{}
	for index, call := range turn.ToolCalls {
		if repaired, ok := repair(call); ok {
			turn.ToolCalls[index] = repaired
			fixed[call.ID] = repaired
		}
	}
	if len(fixed) > 0 {
		turn.Raw = patchRawToolCalls(turn.Raw, fixed)
	}
	return len(fixed)
}

// patchRawToolCalls: doi ten/doi so cac tool call trong Raw theo ID. Hai dang Raw cua codec:
// OpenAI la MOT message {"tool_calls":[{"id","function":{"name","arguments"}}]}, Anthropic la MANG block
// [{"type":"tool_use","id","name","input"}]. Dang la (hoac hong) thi giu nguyen.
func patchRawToolCalls(raw json.RawMessage, fixed map[string]ToolCall) json.RawMessage {
	var message map[string]any
	if json.Unmarshal(raw, &message) == nil {
		calls, _ := message["tool_calls"].([]any)
		for _, entry := range calls {
			call, _ := entry.(map[string]any)
			repaired, ok := fixed[stringField(call, "id")]
			function, isMap := call["function"].(map[string]any)
			if ok && isMap {
				function["name"], function["arguments"] = repaired.Name, repaired.Arguments
			}
		}
		return remarshal(raw, message)
	}
	var blocks []any
	if json.Unmarshal(raw, &blocks) == nil {
		for _, entry := range blocks {
			block, _ := entry.(map[string]any)
			repaired, ok := fixed[stringField(block, "id")]
			if !ok || stringField(block, "type") != "tool_use" {
				continue
			}
			var input any = map[string]any{}
			_ = json.Unmarshal([]byte(repaired.Arguments), &input)
			block["name"], block["input"] = repaired.Name, input
		}
		return remarshal(raw, blocks)
	}
	return raw
}

func stringField(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return value
}

func remarshal(original json.RawMessage, value any) json.RawMessage {
	out, err := json.Marshal(value)
	if err != nil {
		return original
	}
	return out
}
