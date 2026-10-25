package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

// Bridge gia cho bai co 2 slide san: wpp.addSlide them slide 3, wpp.addText tra ten shape, wpp.deleteSlide xoa.
func ownedBridge(t *testing.T) (*RunContext, *OfficeActionTool, *[]string, *int) {
	t.Helper()
	sent := []string{}
	slides := 2
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(body, &request)
		sent = append(sent, request.Action)
		switch request.Action {
		case "wpp.addSlide":
			slides++
			_, _ = w.Write([]byte(`{"ok":true,"result":{"slide":` + strconv.Itoa(slides) + `,"layout":12}}`))
		case "wpp.addText":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"slide":` + strconv.Itoa(asInt(request.Params["slide"])) + `,"shape":"TextBox 7"}}`))
		case "wpp.deleteSlide":
			index := asInt(request.Params["slide"])
			slides--
			_, _ = w.Write([]byte(`{"ok":true,"result":{"deleted":` + strconv.Itoa(index) + `,"slideCount":` + strconv.Itoa(slides) + `}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		}
	}))
	t.Cleanup(server.Close)
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "wpp.addSlide", Kind: "wpp", Agent: true}, {Name: "wpp.addText", Kind: "wpp", Agent: true},
		{Name: "wpp.deleteSlide", Kind: "wpp", Agent: true}, {Name: "wpp.deleteShapes", Kind: "wpp", Agent: true},
	}}
	asked := 0
	run := &RunContext{
		Office: &office.Session{App: "wpp", Port: port},
		Bridge: office.NewBridgeClient(server.Client(), "t"),
		Prompt: "lam slide",
		Confirm: func(action, reason, preview string) (bool, error) {
			asked++
			return false, nil // khong ai bam: nhu interactive=false
		},
	}
	return run, NewOfficeActionTool(catalog, "wpp"), &sent, &asked
}

func call(tool *OfficeActionTool, run *RunContext, action string, params map[string]any) Result {
	return tool.Invoke(context.Background(), map[string]any{"action": action, "params": params}, run)
}

func TestDeletingWhatTheRunCreatedNeedsNoConfirmation(t *testing.T) {
	run, tool, _, asked := ownedBridge(t)

	// Slide 1 co san: van phai hoi (va bi tu choi vi khong ai bam).
	if result := call(tool, run, "wpp.deleteSlide", map[string]any{"slide": 1}); result.OK {
		t.Fatalf("xoa slide co san phai bi hoi va tu choi: %+v", result)
	}
	if *asked != 1 {
		t.Fatalf("phai hoi mot lan, hoi %d", *asked)
	}

	// Luot them slide 3 roi xoa no: khong hoi.
	call(tool, run, "wpp.addSlide", map[string]any{"layout": 12})
	if result := call(tool, run, "wpp.deleteSlide", map[string]any{"slide": 3}); !result.OK {
		t.Fatalf("xoa slide do luot tao phai chay: %+v", result)
	}
	if *asked != 1 {
		t.Fatalf("khong duoc hoi them, da hoi %d", *asked)
	}

	// Shape do luot tao tren slide co san: xoa khong hoi; shape la thi van hoi.
	call(tool, run, "wpp.addText", map[string]any{"slide": 2, "text": "x"})
	if result := call(tool, run, "wpp.deleteShapes", map[string]any{"slide": 2, "shapes": []any{"TextBox 7"}}); !result.OK {
		t.Fatalf("xoa shape do luot tao phai chay: %+v", result)
	}
	if result := call(tool, run, "wpp.deleteShapes", map[string]any{"slide": 2, "shapes": []any{"Title 1"}}); result.OK {
		t.Fatal("shape co san phai bi hoi")
	}
	if *asked != 2 {
		t.Fatalf("chi shape co san moi bi hoi, da hoi %d", *asked)
	}
	// Thieu so slide (lenh tu chon slide cuoi): khong chac la cua luot -> hoi.
	if result := call(tool, run, "wpp.deleteSlide", map[string]any{}); result.OK {
		t.Fatal("khong co so slide thi phai hoi")
	}
}

func TestOwnedSlidesShiftAfterDelete(t *testing.T) {
	o := (&RunContext{}).ownedObjects()
	o.record("wpp.addSlide", map[string]any{}, map[string]any{"slide": 3.0})
	o.record("wpp.addSlide", map[string]any{}, map[string]any{"slide": 4.0})
	o.record("wpp.addText", map[string]any{}, map[string]any{"slide": 4.0, "shape": "TextBox 2"})
	// Xoa slide 2 (co san): slide 3, 4 cua luot lui thanh 2, 3.
	o.record("wpp.deleteSlide", map[string]any{"slide": 2.0}, map[string]any{"deleted": 2.0})
	if !o.covers("wpp.deleteSlide", map[string]any{"slide": 2.0}) || !o.covers("wpp.deleteSlide", map[string]any{"slide": 3.0}) ||
		o.covers("wpp.deleteSlide", map[string]any{"slide": 4.0}) {
		t.Fatalf("so slide phai lui theo: %+v", o.slides)
	}
	if !o.covers("wpp.deleteShapes", map[string]any{"slide": 3.0, "shapes": []any{"TextBox 2"}}) {
		t.Fatalf("shape cung lui theo slide: %+v", o.shapes)
	}
	// Bai khac (tham so presentation): khong phai cua luot.
	if o.covers("wpp.deleteSlide", map[string]any{"slide": 2.0, "presentation": "C:/khac.pptx"}) {
		t.Fatal("slide cua bai khac khong duoc tinh")
	}
}
