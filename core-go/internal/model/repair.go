package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

// dropInvalidArguments: tool call co doi so KHONG phai JSON object (model yeu bi cat giua chung) -> doi so trong lich su
// thay bang "{}" va tra lai doi so goc theo ID. Bo do 27/10/2026 (ca x11, beam-free): doi so dashboard_from_data dut o cot
// 200; Core van gui nguyen chuoi hong trong lich su va nha cung cap tu choi CA request ("arguments must be a valid JSON
// object string") - luot hong han. Lenh do khong chay; model nhan loi va goi lai.
func dropInvalidArguments(turn *Turn) map[string]string {
	invalid := map[string]string{}
	fixed := map[string]ToolCall{}
	for index, call := range turn.ToolCalls {
		text := strings.TrimSpace(call.Arguments)
		var object map[string]any
		if text == "" || json.Unmarshal([]byte(text), &object) == nil {
			continue
		}
		invalid[call.ID] = text
		call.Arguments = "{}"
		turn.ToolCalls[index] = call
		fixed[call.ID] = call
	}
	if len(fixed) > 0 {
		turn.Raw = patchRawToolCalls(turn.Raw, fixed)
	}
	return invalid
}

// invalidArgumentsResult: ket qua thay cho lenh co doi so hong - noi ro khong co gi chay va cach goi lai.
func invalidArgumentsResult(call ToolCall, original string) ToolResult {
	tail := original
	if runes := []rune(tail); len(runes) > 80 {
		tail = "…" + string(runes[len(runes)-80:])
	}
	return ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: ErrorJSON(fmt.Sprintf("the arguments of this %s call are "+
		"not valid JSON (%d characters, ends with %q) - it was cut off, so NOTHING ran. Send the call again with complete JSON; "+
		"if it is long, shorten the text or split the work into smaller calls", call.Name, len(original), tail))}
}
