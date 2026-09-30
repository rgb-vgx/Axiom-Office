package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/skills"
)

func catalog() *office.CommandCatalog {
	return &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "et.writeRange", Kind: "et", Agent: true, Summary: "ghi vung", Params: []office.CommandParam{
			{Name: "range", Required: true, Hint: "top-left cell e.g. 'A1'"}, {Name: "values", Required: true, Hint: "2D array"},
		}},
		{Name: "et.save", Kind: "et", Agent: false},
		{Name: "et.saveAs", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "path", Required: true}}},
		{Name: "writer.closeAll", Kind: "wps", Agent: false},
		{Name: "wpp.deleteSlide", Kind: "wpp", Agent: true},
	}}
}

func TestOfficeActionAllowlistAndDescription(t *testing.T) {
	tool := NewOfficeActionTool(catalog(), "et")
	if strings.Join(tool.AllowedActions(), ",") != "et.saveAs,et.writeRange" {
		t.Fatalf("allowlist = %v", tool.AllowedActions())
	}
	// Mo ta tool liet ke chu ky lenh kem goi y tham so, va co quy tac khong undo.
	for _, want := range []string{"et.writeRange {range (top-left cell e.g. 'A1'),values (2D array)}", UndoRule} {
		if !strings.Contains(tool.Description(), want) {
			t.Errorf("mo ta thieu %q", want)
		}
	}

	run := &RunContext{Office: &office.Session{Port: 1}, Bridge: office.NewBridgeClient(nil, "")}
	if result := tool.Invoke(context.Background(), map[string]any{"action": "writer.closeAll"}, run); result.OK ||
		!strings.Contains(result.JSON, "not an available action") {
		t.Fatalf("lenh ngoai allowlist phai bi tu choi: %+v", result)
	}
	if result := tool.Invoke(context.Background(), map[string]any{}, run); !strings.Contains(result.JSON, "missing 'action'") {
		t.Fatalf("thieu action: %+v", result)
	}
	// Lenh cua app khac khong duoc co trong allowlist cua app nay.
	if strings.Contains(strings.Join(tool.AllowedActions(), ","), "wpp.") {
		t.Fatal("lenh cua app khac khong duoc lot vao allowlist")
	}
}

func TestCanonicalActionFixesTypos(t *testing.T) {
	allowed := []string{"et.writeRange", "writer.getText"}
	cases := map[string]string{
		"et_writeRange":    "et.writeRange",    // gach duoi thay vi dau cham
		"ET.WRITERANGE":    "et.writeRange",    // khac hoa/thuong
		"writer.getText":   "writer.getText",   // dung san
		" writer.getText ": "writer.getText",   // thua khoang trang
		"khong.co.ten.nay": "khong.co.ten.nay", // khong khop -> giu nguyen de bao loi
	}
	for input, want := range cases {
		if got := CanonicalAction(input, allowed); got != want {
			t.Errorf("CanonicalAction(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeParams(t *testing.T) {
	if params := NormalizeParams(`{"range":"A1"}`); params["range"] != "A1" {
		t.Fatalf("chuoi JSON phai duoc parse: %v", params)
	}
	if params := NormalizeParams(map[string]any{"range": "A1"}); params["range"] != "A1" {
		t.Fatalf("object phai giu nguyen: %v", params)
	}
	if params := NormalizeParams("  "); params != nil {
		t.Fatalf("chuoi rong -> nil, nhan duoc %v", params)
	}
	if params := NormalizeParams("{khong phai json}"); params != nil {
		t.Fatalf("JSON hong -> nil, nhan duoc %v", params)
	}
}

func TestSkillToolsWithEmptyIndex(t *testing.T) {
	// Chi muc rong: load_skill phai bao khong co skill nao (khong lam hong luot chay).
	index := skills.New(nil)
	load := NewLoadSkillTool(index, "et")
	result := load.Invoke(context.Background(), map[string]any{"name": "khong-co"}, &RunContext{})
	if result.OK || !strings.Contains(result.JSON, "unknown skill 'khong-co'") || !strings.Contains(result.JSON, "(none)") {
		t.Fatalf("load_skill = %+v", result)
	}

	read := NewReadSkillFileTool(index, "et")
	if result := read.Invoke(context.Background(), map[string]any{"name": "_design", "path": "tokens.json"}, &RunContext{}); result.OK {
		t.Fatalf("_design khong ton tai phai loi: %+v", result)
	}
}

func TestModelToolsShape(t *testing.T) {
	tool := NewOfficeActionTool(catalog(), "et")
	list := NewRegistry([]Tool{tool}).ModelTools()
	if len(list) != 1 || list[0].Name != "office_action" {
		t.Fatalf("ModelTools = %+v", list)
	}
	var schema map[string]any
	if err := json.Unmarshal(list[0].Parameters, &schema); err != nil || schema["type"] != "object" {
		t.Fatalf("schema = %s", list[0].Parameters)
	}
}
