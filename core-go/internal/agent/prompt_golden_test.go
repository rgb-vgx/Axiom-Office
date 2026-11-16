package agent

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/tools"
)

// updatePromptGoldens: `go test ./internal/agent -run TestPromptGoldens -update-prompt-goldens`
// ghi lai testdata/*.golden tu code hien tai. Mac dinh test chi so sanh.
var updatePromptGoldens = flag.Bool("update-prompt-goldens", false, "ghi lai testdata/*.golden tu prompt hien tai")

// TestPromptGoldens khoa lai system prompt on dinh cho tung app va che do lap ke hoach.
// Dau vao co dinh: khong skill, khong memory, khong tom tat, khong duong dan tuyet doi, khong thoi gian.
func TestPromptGoldens(t *testing.T) {
	wpsSession := &office.Session{App: "wps", Document: "giao-an.docx"}
	wpsDocument := &DocumentContext{
		Name:        "giao-an.docx",
		HeadingPath: []string{"Giao an", "Tuan 3"},
	}
	etSession := &office.Session{App: "et", Document: "bang-diem.xlsx"}
	etDocument := &DocumentContext{
		Name:  "bang-diem.xlsx",
		Sheet: "Sheet1",
		Range: "A1:C10",
	}
	wppSession := &office.Session{App: "wpp", Document: "bai-giang.pptx"}
	wppDocument := &DocumentContext{
		Name:       "bai-giang.pptx",
		Slide:      3,
		SlideCount: 10,
	}

	cases := []struct {
		name   string
		prompt string
	}{
		{"wps", Build("wps", wpsSession, wpsDocument, nil, nil, nil, "", nil)},
		{"et", Build("et", etSession, etDocument, nil, nil, nil, "", nil)},
		{"wpp", Build("wpp", wppSession, wppDocument, nil, nil, nil, "", nil)},
		{
			"plan",
			// Giong orchestrator_execution.go: prompt thuong + PlanModeRule + PlanCapabilities.
			// Catalog rong -> khong co lenh ghi nao, PlanCapabilities tra "" (dau vao co dinh).
			Build("wpp", wppSession, wppDocument, nil, nil, nil, "", nil) + "\n\n" +
				PlanModeRule + PlanCapabilities(tools.NewOfficeActionTool(&office.CommandCatalog{}, "wpp"), nil),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("testdata", tc.name+".golden")
			want := tc.prompt + "\n"
			if *updatePromptGoldens {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatalf("tao testdata: %v", err)
				}
				if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
					t.Fatalf("ghi golden: %v", err)
				}
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("doc golden: %v", err)
			}
			if string(data) != want {
				t.Fatalf("prompt %s khac golden %s\n--- got ---\n%s\n--- want ---\n%s", tc.name, path, want, data)
			}
		})
	}
}
