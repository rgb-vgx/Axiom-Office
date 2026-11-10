package tools

import (
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

func hintedCatalog() *office.CommandCatalog {
	return &office.CommandCatalog{Version: "1", Commands: []office.Command{{
		Name: "writer.insertStyledText", Kind: "wps", Agent: true,
		Params: []office.CommandParam{
			{Name: "text", Required: true},
			{Name: "alignment", Hint: "left/center/right/justify; áp cho từng đoạn vừa chèn"},
			{Name: "document", Hint: "full path; default active"},
		},
	}}}
}

// Compact: chữ ký giữ tên tham số (kể cả tham số tuỳ chọn) nhưng bỏ hint dài; enum action không đổi.
func TestCompactOfficeActionDropsLongHints(t *testing.T) {
	full := NewOfficeActionTool(hintedCatalog(), "wps")
	compact := NewCompactOfficeActionTool(hintedCatalog(), "wps")

	if !strings.Contains(full.Description(), "left/center/right/justify") {
		t.Fatal("ban day du phai co hint")
	}
	if strings.Contains(compact.Description(), "left/center/right/justify") {
		t.Fatal("ban gon khong duoc co hint dai")
	}
	for _, want := range []string{"writer.insertStyledText {text,alignment}", "not a separate tool"} {
		if !strings.Contains(compact.Description(), want) {
			t.Fatalf("ban gon thieu %q: %s", want, compact.Description())
		}
	}
	if len(compact.AllowedActions()) != len(full.AllowedActions()) || compact.AllowedActions()[0] != full.AllowedActions()[0] {
		t.Fatalf("enum action phai giu nguyen")
	}
	if len(compact.Description()) >= len(full.Description()) {
		t.Fatalf("ban gon phai ngan hon: %d >= %d", len(compact.Description()), len(full.Description()))
	}
}
