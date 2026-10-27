package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"axiomoffice/core/internal/office"
)

// uncertainBridge: bridge gia - writer.appendText luon ghi vao "tai lieu" nhung tra loi bang `reply` (mo phong lenh xong
// trong Office sau khi bridge da bao HOST_BUSY).
func uncertainBridge(t *testing.T, reply string) (*OfficeActionTool, *RunContext, func() int) {
	t.Helper()
	var mu sync.Mutex
	applied := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Action string `json:"action"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request.Action == "writer.appendText" {
			mu.Lock()
			applied++
			mu.Unlock()
			_, _ = w.Write([]byte(reply))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"text":"a"}}`))
	}))
	t.Cleanup(server.Close)
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	catalog := &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "writer.appendText", Kind: "wps", Agent: true, Params: []office.CommandParam{{Name: "text", Required: true}}},
		{Name: "writer.getText", Kind: "wps", Agent: true},
	}}
	run := &RunContext{Office: &office.Session{App: "wps", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "t")}
	return NewOfficeActionTool(catalog, "wps"), run, func() int {
		mu.Lock()
		defer mu.Unlock()
		return applied
	}
}

func TestUncertainWriteIsTreatedAsChangedAndNotResentBlindly(t *testing.T) {
	busy := `{"ok":false,"error":"HOST_BUSY: the host has not finished 'writer.appendText' after 120 s (it is still running)."}`
	tool, run, applied := uncertainBridge(t, busy)
	params := map[string]any{"text": "Doan moi"}

	first := call(tool, run, "writer.appendText", params)
	if first.OK || !first.Mutating || !strings.Contains(first.JSON, "may still have been applied") {
		t.Fatalf("ket qua khong ro phai coi la CO THE da sua va noi cho model biet: %+v", first)
	}
	again := call(tool, run, "writer.appendText", params)
	if again.OK || !strings.Contains(again.JSON, "may already have been applied") || applied() != 1 {
		t.Fatalf("goi lai y het truoc khi doc lai phai bi chan, bridge nhan %d lan: %+v", applied(), again)
	}
	if other := call(tool, run, "writer.appendText", map[string]any{"text": "Doan khac"}); applied() != 2 || other.Mutating != true {
		t.Fatalf("lenh ghi khac van chay binh thuong: bridge nhan %d lan, %+v", applied(), other)
	}
	call(tool, run, "writer.getText", nil)
	call(tool, run, "writer.appendText", params)
	if applied() != 3 {
		t.Fatalf("doc lai tai lieu roi thi duoc gui lai, bridge nhan %d lan", applied())
	}
}

func TestUncertainWriteDoesNotCountAnUndoStep(t *testing.T) {
	tool, run, _ := uncertainBridge(t, `{"ok":false,"error":"bridge did not answer within 330s"}`)
	call(tool, run, "writer.appendText", map[string]any{"text": "x"})
	if run.undoable["writer"] != 0 {
		t.Fatalf("ket qua khong ro thi khong duoc dem buoc hoan tac chac chan: %v", run.undoable)
	}
}

func TestCommandThatWasNotRunIsAPlainFailure(t *testing.T) {
	for _, reply := range []string{
		`{"ok":false,"error":"HOST_BUSY: 'writer.appendText' was not run because 'writer.getText' is still running in the host."}`,
		`{"ok":false,"error":"WPS add-in disconnected before the command was sent (WPS closed?) - it was not sent, safe to retry"}`,
		`{"ok":false,"error":"'text' is required"}`,
	} {
		tool, run, applied := uncertainBridge(t, reply)
		params := map[string]any{"text": "y"}
		result := call(tool, run, "writer.appendText", params)
		if result.Mutating || strings.Contains(result.JSON, "may still have been applied") {
			t.Fatalf("lenh chac chan chua chay khong duoc coi la da sua (%s): %+v", reply, result)
		}
		call(tool, run, "writer.appendText", params)
		if applied() != 2 {
			t.Fatalf("lenh chua chay thi duoc gui lai (%s), bridge nhan %d lan", reply, applied())
		}
	}
}

func TestUncertainOutcomeMarkers(t *testing.T) {
	cases := map[string]bool{
		"HOST_BUSY: the host has not finished 'x' after 120 s (it is still running).": true,
		"HOST_BUSY: 'x' was not run because 'y' is still running in the host.":        false,
		"bridge timeout": true,
		"WPS add-in did not answer within 330s; the command may have been applied - check the document before retrying": true,
		"WPS add-in disconnected before the command was sent (WPS closed?) - it was not sent, safe to retry":            false,
		"cancelled": false,
	}
	for text, want := range cases {
		if got := uncertainOutcome(text); got != want {
			t.Errorf("uncertainOutcome(%q) = %v, muon %v", text, got, want)
		}
	}
}
