package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

func TestOfficeActionTargetsTheRunDocument(t *testing.T) {
	sent := []map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(body, &request)
		sent = append(sent, request.Params)
		if request.Action == "wpp.newPresentation" {
			_, _ = w.Write([]byte(`{"ok":true,"result":{"name":"Presentation7"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"slide":1}}`))
	}))
	defer server.Close()
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	target := office.CommandParam{Name: "presentation", Hint: "full path of the presentation to work on; default: the active one"}
	catalog := &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "wpp.addSlide", Kind: "wpp", Agent: true, Params: []office.CommandParam{{Name: "layout"}, target}},
		{Name: "wpp.newPresentation", Kind: "wpp", Agent: true},
	}}
	tool := NewOfficeActionTool(catalog, "wpp")
	if strings.Contains(tool.Description(), "presentation (full path") {
		t.Fatal("tham so chon tai lieu phai an khoi mo ta tool (Core tu gan)")
	}
	run := &RunContext{Office: &office.Session{App: "wpp", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "t"),
		TargetDocument: `C:\bao-cao\quy-3.pptx`}

	call(tool, run, "wpp.addSlide", map[string]any{"layout": 12})
	if sent[0]["presentation"] != `C:\bao-cao\quy-3.pptx` {
		t.Fatalf("phai gan tai lieu cua luot: %+v", sent[0])
	}
	// Model tu chon bai khac: giu nguyen.
	call(tool, run, "wpp.addSlide", map[string]any{"presentation": "khac.pptx"})
	if sent[1]["presentation"] != "khac.pptx" {
		t.Fatalf("khong duoc ghi de lua chon cua model: %+v", sent[1])
	}
	// Tao bai moi: cac lenh sau nham bai moi (benchmark s09 lan 1: bai moi khong thanh active).
	call(tool, run, "wpp.newPresentation", nil)
	call(tool, run, "wpp.addSlide", nil)
	if run.TargetDocument != "Presentation7" || sent[3]["presentation"] != "Presentation7" {
		t.Fatalf("sau newPresentation phai nham bai moi: target=%q params=%+v", run.TargetDocument, sent[3])
	}
}
