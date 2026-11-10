package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

// look_at_document: ban Windows xuat thang slide va bao slide/slideCount -> model phai duoc noi ro dang nhin
// slide nao (13/10/2026: model ghi "kiem tra slide 1" ma khong biet anh la slide nao). Ban LibreOffice khong
// bao slide -> giu ghi chu chung.
func visualRun(t *testing.T, reply string) (*RunContext, *[]string) {
	t.Helper()
	actions := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(strings.Builder)
		buffer := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buffer)
			body.Write(buffer[:n])
			if err != nil {
				break
			}
		}
		actions = append(actions, body.String())
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(server.Close)
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	return &RunContext{
		Office: &office.Session{App: "wpp", Port: port},
		Bridge: office.NewBridgeClient(server.Client(), "t"),
	}, &actions
}

func TestLookAtDocumentNamesTheSlide(t *testing.T) {
	run, actions := visualRun(t, `{"ok":true,"result":{"width":1280,"height":720,"mime":"image/png",`+
		`"base64":"iVBORw0KGgo=","slide":2,"slideCount":3}}`)
	result := NewVisualTool().Invoke(context.Background(), map[string]any{"reason": "slide 2"}, run)
	if !result.OK || result.ImageDataURL != "data:image/png;base64,iVBORw0KGgo=" {
		t.Fatalf("expected an image result, got ok=%v image=%q json=%s", result.OK, result.ImageDataURL, result.JSON)
	}
	if !strings.Contains(result.JSON, "slide 2 of 3") || !strings.Contains(result.JSON, `"slideCount":3`) {
		t.Fatalf("result must tell the model which slide it sees: %s", result.JSON)
	}
	if len(*actions) != 1 || !strings.Contains((*actions)[0], "app.screenshot") {
		t.Fatalf("expected one app.screenshot call, got %v", *actions)
	}
}

func TestLookAtDocumentWithoutSlideKeepsGenericNote(t *testing.T) {
	run, _ := visualRun(t, `{"ok":true,"result":{"width":640,"height":905,"mime":"image/png","base64":"iVBORw0KGgo="}}`)
	result := NewVisualTool().Invoke(context.Background(), map[string]any{}, run)
	if !result.OK || strings.Contains(result.JSON, "slide") {
		t.Fatalf("page images (LibreOffice/Word) must not mention a slide: ok=%v json=%s", result.OK, result.JSON)
	}
}

func TestLookAtDocumentReportsBridgeError(t *testing.T) {
	run, _ := visualRun(t, `{"ok":false,"error":"no visible application window to capture"}`)
	result := NewVisualTool().Invoke(context.Background(), map[string]any{}, run)
	if result.OK || result.ImageDataURL != "" || !strings.Contains(result.JSON, "no visible application window") {
		t.Fatalf("bridge error must reach the model without an image: ok=%v json=%s", result.OK, result.JSON)
	}
}

// Slide theo so: wpp.renderSlide (khong doi cua so nguoi dung); bridge khong co lenh do thi chup slide dang hien.
func TestLookAtDocumentRendersTheRequestedSlide(t *testing.T) {
	run, actions := visualRun(t, `{"ok":true,"result":{"width":1280,"height":720,"mime":"image/png",`+
		`"base64":"iVBORw0KGgo=","slide":4,"slideCount":6}}`)
	run.TargetDocument = "Presentation2"
	result := NewVisualTool().Invoke(context.Background(), map[string]any{"slide": 4.0}, run)
	if !result.OK || !strings.Contains(result.JSON, "slide 4 of 6") {
		t.Fatalf("anh slide 4: %s", result.JSON)
	}
	if len(*actions) != 1 || !strings.Contains((*actions)[0], `"wpp.renderSlide"`) || !strings.Contains((*actions)[0], `"presentation":"Presentation2"`) {
		t.Fatalf("phai goi wpp.renderSlide cho bai cua luot: %v", *actions)
	}

	old, oldActions := visualRun(t, `{"ok":false,"error":"unknown action: wpp.renderSlide"}`)
	_ = NewVisualTool().Invoke(context.Background(), map[string]any{"slide": 4.0}, old)
	if len(*oldActions) != 2 || !strings.Contains((*oldActions)[1], "app.screenshot") {
		t.Fatalf("bridge cu phai quay ve app.screenshot: %v", *oldActions)
	}
}
