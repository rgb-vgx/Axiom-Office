package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/office"
)

type bridgeCall struct {
	Action string
	Params map[string]any
}

func trackingBridge(t *testing.T, author string) (*Orchestrator, int, func() []bridgeCall) {
	t.Helper()
	var mu sync.Mutex
	calls := []bridgeCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request bridgeCall
		_ = json.NewDecoder(r.Body).Decode(&request)
		mu.Lock()
		calls = append(calls, request)
		mu.Unlock()
		result := map[string]any{}
		if request.Action == "writer.setTracking" {
			result = map[string]any{"previous": false, "tracking": request.Params["on"], "author": author}
		}
		data, _ := json.Marshal(map[string]any{"ok": true, "result": result})
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	o := &Orchestrator{Bridge: office.NewBridgeClient(server.Client(), "t"), Config: func() config.Config { return config.Config{} }}
	return o, port, func() []bridgeCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]bridgeCall{}, calls...)
	}
}

var trackingCatalog = &office.CommandCatalog{Commands: []office.Command{{Name: "writer.setTracking", Kind: "wps"}}}

func TestTrackedRunIsUndoneByRejectingItsRevisions(t *testing.T) {
	o, port, calls := trackingBridge(t, TrackingAuthor)
	run := newRun("r_t", port)
	on := true
	o.startTracking(context.Background(), run, &Request{TrackChanges: &on}, "wps", trackingCatalog, false, `C:\hop-dong.docx`)
	if !run.Tracking || run.undoMode() != "revisions" {
		t.Fatalf("luot phai bat theo doi: %+v", run)
	}
	o.stopTracking(run)
	o.stopTracking(run) // lan hai khong goi lai bridge
	run.Status = StatusCompleted
	reply, problem := o.UndoRun(context.Background(), run)
	if problem != "" || reply["mode"] != "revisions" {
		t.Fatalf("hoan tac luot co theo doi = tu choi revision: %v %q", reply, problem)
	}
	if _, problem := o.UndoRun(context.Background(), run); !strings.Contains(problem, "already undone") {
		t.Fatalf("hoan tac lan hai phai bi tu choi: %q", problem)
	}
	got := calls()
	if len(got) != 3 {
		t.Fatalf("bat, tra lai, tu choi: %+v", got)
	}
	if got[0].Params["on"] != true || got[0].Params["author"] != TrackingAuthor || got[0].Params["document"] != `C:\hop-dong.docx` {
		t.Fatalf("bat theo doi voi tac gia Axiom AI tren tai lieu cua luot: %+v", got[0])
	}
	if got[1].Params["on"] != false || got[1].Params["restoreAuthor"] != true {
		t.Fatalf("cuoi luot tra lai trang thai cu va ten nguoi dung: %+v", got[1])
	}
	if got[2].Action != "writer.reviewRevisions" || got[2].Params["action"] != "reject" || got[2].Params["author"] != TrackingAuthor ||
		got[2].Params["since"] == nil {
		t.Fatalf("hoan tac = tu choi revision cua Axiom AI tu luc bat dau luot: %+v", got[2])
	}
}

func TestUntrackedRunIsUndoneByItsOwnSteps(t *testing.T) {
	o, port, calls := trackingBridge(t, TrackingAuthor)
	run := newRun("r_u", port)
	o.startTracking(context.Background(), run, &Request{}, "wps", trackingCatalog, false, "") // cau hinh tat
	if run.Tracking || len(calls()) != 0 {
		t.Fatal("khong bat theo doi khi cau hinh tat va khong co options.trackChanges")
	}
	if _, problem := o.UndoRun(context.Background(), run); !strings.Contains(problem, "still running") {
		t.Fatalf("luot dang chay khong hoan tac: %q", problem)
	}
	run.Status, run.UndoSteps = StatusCompleted, 2
	if _, problem := o.UndoRun(context.Background(), run); problem != "" {
		t.Fatal(problem)
	}
	got := calls()
	if len(got) != 1 || got[0].Action != "writer.undo" || got[0].Params["count"] != 2.0 {
		t.Fatalf("lui dung 2 buoc cua luot: %+v", got)
	}
	empty := newRun("r_e", port)
	empty.Status = StatusCompleted
	if _, problem := o.UndoRun(context.Background(), empty); !strings.Contains(problem, "nothing to undo") {
		t.Fatalf("luot khong sua gi: %q", problem)
	}
}

func TestParseRequestTrackChanges(t *testing.T) {
	request, _ := ParseRequest(map[string]any{"prompt": "x", "office": map[string]any{"port": 1.0},
		"options": map[string]any{"trackChanges": false}})
	if request.TrackChanges == nil || *request.TrackChanges {
		t.Fatalf("options.trackChanges=false phai ghi de cau hinh: %+v", request.TrackChanges)
	}
	request, _ = ParseRequest(map[string]any{"prompt": "x", "office": map[string]any{"port": 1.0}})
	if request.TrackChanges != nil {
		t.Fatal("khong gui thi theo cau hinh")
	}
}
