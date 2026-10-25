package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/tools"
)

// repairRegistry: office_action cho Writer (writer.closeAll khong cho agent) + mot tool khac.
func repairRegistry() *tools.Registry {
	catalog := &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "writer.getText", Kind: "writer", Agent: true, Params: []office.CommandParam{{Name: "maxChars"}}},
		{Name: "writer.insertStyledText", Kind: "writer", Agent: true, Params: []office.CommandParam{{Name: "text", Required: true}}},
		{Name: "writer.closeAll", Kind: "writer", Agent: false},
		{Name: "et.writeRange", Kind: "et", Agent: true},
	}}
	return tools.NewRegistry([]tools.Tool{tools.NewOfficeActionTool(catalog, "writer"), tools.NewTodoTool()})
}

func repairedBody(t *testing.T, call model.ToolCall) map[string]any {
	t.Helper()
	if call.Name != "office_action" {
		t.Fatalf("ten tool = %q", call.Name)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(call.Arguments), &body); err != nil {
		t.Fatalf("doi so khong phai JSON: %v (%s)", err, call.Arguments)
	}
	return body
}

// Luot that 24/10/2026: "writer.getText" goi nhu tool, doi so phang -> office_action, ca doi so la params.
func TestRepairToolCallFlatArguments(t *testing.T) {
	fixed, ok := repairToolCall(repairRegistry(), model.ToolCall{ID: "c1", Name: "writer.getText", Arguments: `{"maxChars":10000}`})
	if !ok || fixed.ID != "c1" {
		t.Fatalf("phai sua, giu ID: %+v %v", fixed, ok)
	}
	body := repairedBody(t, fixed)
	params, _ := body["params"].(map[string]any)
	if body["action"] != "writer.getText" || params["maxChars"] != 10000.0 {
		t.Fatalf("%s", fixed.Arguments)
	}
}

func TestRepairToolCallNestedParamsAndVariants(t *testing.T) {
	registry := repairRegistry()
	fixed, ok := repairToolCall(registry, model.ToolCall{Name: "writer.insertStyledText",
		Arguments: `{"action":"x","params":{"text":"Xin chao"}}`})
	body := repairedBody(t, fixed)
	params, _ := body["params"].(map[string]any)
	if !ok || body["action"] != "writer.insertStyledText" || params["text"] != "Xin chao" || len(params) != 1 {
		t.Fatalf("params long nhau phai giu nguyen: %s", fixed.Arguments)
	}

	// "_" thay "." va khac hoa/thuong: CanonicalAction dua ve ten chuan; doi so rong -> khong co params.
	fixed, ok = repairToolCall(registry, model.ToolCall{Name: "Writer_GetText", Arguments: ""})
	body = repairedBody(t, fixed)
	if _, has := body["params"]; !ok || body["action"] != "writer.getText" || has {
		t.Fatalf("%s", fixed.Arguments)
	}

	// Doi so khong phai JSON: van chuyen ten de office_action bao loi params.
	fixed, ok = repairToolCall(registry, model.ToolCall{Name: "writer.getText", Arguments: `not json`})
	if body = repairedBody(t, fixed); !ok || body["action"] != "writer.getText" {
		t.Fatalf("%s", fixed.Arguments)
	}
}

// Khong sua: tool co that, lenh bi cam, lenh cua app khac, ten la; registry khong co office_action.
func TestRepairToolCallLeavesOthersAlone(t *testing.T) {
	registry := repairRegistry()
	for _, name := range []string{tools.TodoToolName, "office_action", "writer.closeAll", "et.writeRange", "writer.getTex", "search"} {
		call := model.ToolCall{ID: "c", Name: name, Arguments: `{}`}
		if fixed, ok := repairToolCall(registry, call); ok || fixed != call {
			t.Fatalf("%s khong duoc sua: %+v", name, fixed)
		}
	}
	bare := tools.NewRegistry([]tools.Tool{tools.NewTodoTool()})
	if _, ok := repairToolCall(bare, model.ToolCall{Name: "writer.getText"}); ok {
		t.Fatal("khong co office_action thi khong sua")
	}
	if _, ok := repairToolCall(nil, model.ToolCall{Name: "writer.getText"}); ok {
		t.Fatal("registry nil")
	}
}

func TestUnknownToolMessage(t *testing.T) {
	registry := repairRegistry()
	message := unknownToolMessage(registry, "writer.closeAll")
	if !strings.Contains(message, "is not a tool") || !strings.Contains(message, "office_action") ||
		!strings.Contains(message, "writer.getText") {
		t.Fatalf("lenh bi cam phai noi cach goi va lenh con lai: %s", message)
	}
	message = unknownToolMessage(registry, "search")
	if !strings.Contains(message, "unknown tool 'search'") || !strings.Contains(message, "office_action, "+tools.TodoToolName) ||
		!strings.Contains(message, "are not tools") {
		t.Fatalf("%s", message)
	}
	if message = unknownToolMessage(nil, "x"); !strings.Contains(message, "unknown tool 'x'") || strings.Contains(message, "office_action") {
		t.Fatalf("%s", message)
	}
}

func TestLooksLikeAction(t *testing.T) {
	for name, want := range map[string]bool{"writer.getText": true, "et_writeRange": true, " WPP.addSlide": true,
		"app.save": true, "search": false, "etc": false, "writer": false} {
		if got := looksLikeAction(name); got != want {
			t.Fatalf("looksLikeAction(%q) = %v", name, got)
		}
	}
}
