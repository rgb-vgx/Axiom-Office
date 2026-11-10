package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"axiomoffice/core/internal/office"
)

// fakeOtherWorkbook: bridge Excel thu hai - /commands tra catalog nho, /cmd ghi lai lenh nhan duoc.
func fakeOtherWorkbook(t *testing.T) (office.Session, *office.BridgeClient, func() []string) {
	t.Helper()
	var mu sync.Mutex
	received := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply := map[string]any{}
		if strings.HasSuffix(r.URL.Path, "/commands") {
			reply = map[string]any{"version": "1", "commands": []any{
				map[string]any{"name": "et.readRanges", "kind": "et", "agent": true, "params": []any{
					map[string]any{"name": "reads", "required": true}, map[string]any{"name": "workbook"}}},
				map[string]any{"name": "et.addSheet", "kind": "et", "agent": true, "params": []any{
					map[string]any{"name": "name", "required": true}, map[string]any{"name": "workbook"}}},
			}}
		} else {
			var request struct {
				Action string         `json:"action"`
				Params map[string]any `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			mu.Lock()
			workbook, _ := request.Params["workbook"].(string)
			received = append(received, request.Action+"@"+workbook)
			mu.Unlock()
			if request.Action == "et.readRanges" {
				reply = map[string]any{"reads": []any{map[string]any{"sheet": "Data", "values": []any{[]any{"Doanh thu", 120}}}}}
			}
		}
		data, _ := json.Marshal(map[string]any{"ok": true, "result": reply})
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	session := office.Session{App: "et", Port: port, Document: "Doanh thu.xlsx", DocumentPath: "/tmp/Doanh thu.xlsx"}
	return session, office.NewBridgeClient(server.Client(), "t"), func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string{}, received...)
	}
}

func TestOtherDocumentReadsAndAsksOnceBeforeWriting(t *testing.T) {
	session, bridge, received := fakeOtherWorkbook(t)
	asked := 0
	run := &RunContext{RunID: "r1", Office: &office.Session{App: "wps", Port: 1}, Bridge: bridge,
		Confirm: func(action, reason, preview string) (bool, error) {
			asked++
			if !strings.Contains(reason, "Doanh thu.xlsx") {
				t.Errorf("reason should name the other document: %q", reason)
			}
			return true, nil
		}}
	tool := NewOtherDocumentTool([]office.Session{session}, false, nil)
	if !strings.Contains(tool.Description(), "Doanh thu.xlsx (Excel, port ") {
		t.Fatalf("description should list the open document: %s", tool.Description())
	}

	listed := tool.Invoke(context.Background(), map[string]any{"document": "Doanh thu", "action": "commands"}, run)
	if !listed.OK || !strings.Contains(listed.JSON, "et.readRanges {reads}") || strings.Contains(listed.JSON, "workbook") {
		t.Fatalf("commands should list signatures without the target param: %s", listed.JSON)
	}
	if len(received()) != 0 {
		t.Fatalf("listing commands must not run a command: %v", received())
	}

	read := tool.Invoke(context.Background(), map[string]any{"document": "Excel", "action": "et.readRanges",
		"params": map[string]any{"reads": []any{map[string]any{"range": "Data!A1:B1"}}}}, run)
	if !read.OK || !strings.Contains(read.JSON, "120") || asked != 0 {
		t.Fatalf("read should run without asking: ok=%v asked=%d %s", read.OK, asked, read.JSON)
	}

	for i := 0; i < 2; i++ {
		write := tool.Invoke(context.Background(), map[string]any{"document": strconv.Itoa(session.Port),
			"action": "et.addSheet", "params": map[string]any{"name": "Tong hop " + strconv.Itoa(i)}}, run)
		if !write.OK || write.Mutating {
			t.Fatalf("write %d: ok=%v mutating=%v %s", i, write.OK, write.Mutating, write.JSON)
		}
	}
	if asked != 1 {
		t.Fatalf("changing another document asks once per run, asked %d times", asked)
	}
	want := []string{"et.readRanges@/tmp/Doanh thu.xlsx", "et.addSheet@/tmp/Doanh thu.xlsx", "et.addSheet@/tmp/Doanh thu.xlsx"}
	if got := received(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("commands should target the other workbook: %v", got)
	}
	if run.UndoSteps("et") != 0 || run.TargetDocument != "" {
		t.Fatalf("the run's own document state must not change")
	}
}

func TestOtherDocumentRefusesWritesWhenDeclinedBusyOrReadOnly(t *testing.T) {
	session, bridge, received := fakeOtherWorkbook(t)
	run := &RunContext{Office: &office.Session{App: "wps", Port: 1}, Bridge: bridge,
		Confirm: func(string, string, string) (bool, error) { return false, nil }}
	write := map[string]any{"document": "Doanh thu.xlsx", "action": "et.addSheet", "params": map[string]any{"name": "X"}}

	declined := NewOtherDocumentTool([]office.Session{session}, false, nil).Invoke(context.Background(), write, run)
	if declined.OK || !strings.Contains(declined.JSON, "user declined") {
		t.Fatalf("declined write should fail: %s", declined.JSON)
	}
	busy := NewOtherDocumentTool([]office.Session{session}, false, func(int) bool { return true }).
		Invoke(context.Background(), write, run)
	if busy.OK || !strings.Contains(busy.JSON, "another Axiom run") {
		t.Fatalf("busy document should refuse writes: %s", busy.JSON)
	}
	readOnly := NewOtherDocumentTool([]office.Session{session}, true, nil)
	if !strings.Contains(readOnly.Description(), "Only read commands") {
		t.Fatalf("read-only description: %s", readOnly.Description())
	}
	if result := readOnly.Invoke(context.Background(), write, run); result.OK || !strings.Contains(result.JSON, "not an available action") {
		t.Fatalf("read-only mode must hide writes: %s", result.JSON)
	}
	if got := received(); len(got) != 0 {
		t.Fatalf("no command should reach the bridge: %v", got)
	}
	if result := readOnly.Invoke(context.Background(), map[string]any{"document": "Bao cao", "action": "commands"}, run); result.OK ||
		!strings.Contains(result.JSON, "no open document") {
		t.Fatalf("unknown document should list the choices: %s", result.JSON)
	}
}
