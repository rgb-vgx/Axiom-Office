package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

// `action` la enum dung danh sach lenh duoc phep (cua app nay, Agent=true); mo ta noi ro lenh la gia tri.
func TestOfficeActionSchemaActionEnum(t *testing.T) {
	tool := NewOfficeActionTool(calcCatalog(), "et")
	var schema struct {
		Properties struct {
			Action struct {
				Enum []string `json:"enum"`
			} `json:"action"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(tool.Parameters(), &schema); err != nil {
		t.Fatal(err)
	}
	if strings.Join(schema.Properties.Action.Enum, ",") != strings.Join(tool.AllowedActions(), ",") ||
		len(schema.Properties.Action.Enum) != 6 || strings.Join(schema.Required, ",") != "action" {
		t.Fatalf("%s", tool.Parameters())
	}
	if !strings.Contains(tool.Description(), "not a separate tool") {
		t.Fatal("mo ta phai noi lenh la gia tri cua action")
	}

	// Khong lenh nao: khong co enum rong (enum [] khong hop le voi nhieu nha cung cap).
	empty := NewOfficeActionTool(&office.CommandCatalog{}, "et")
	if strings.Contains(string(empty.Parameters()), "enum") {
		t.Fatalf("%s", empty.Parameters())
	}
}

func TestResolveAction(t *testing.T) {
	tool := NewOfficeActionTool(calcCatalog(), "et")
	for name, want := range map[string]string{"et.readRange": "et.readRange", "ET.READRANGE": "et.readRange", "et_writeRange": "et.writeRange"} {
		if got, ok := tool.ResolveAction(name); !ok || got != want {
			t.Fatalf("ResolveAction(%q) = %q %v", name, got, ok)
		}
	}
	if _, ok := tool.ResolveAction("et.readRang"); ok {
		t.Fatal("chi khop chinh xac, khong doan gan dung")
	}
}
