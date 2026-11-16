package api

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/store"
)

func newRunEventsHandler(t *testing.T, seed func(*store.Stores), manager *agent.Manager) http.Handler {
	t.Helper()
	root := t.TempDir()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	if !stores.Available {
		t.Fatalf("stores khong san sang: %s", stores.Error)
	}
	t.Cleanup(func() { _ = stores.DB.Close() })
	if seed != nil {
		seed(stores)
	}
	if manager == nil {
		manager = agent.NewManager()
	}
	return Handler(&Deps{
		Config:  config.Config{Token: "tok", MemoryEnabled: true},
		Paths:   config.NewPaths(root),
		Runtime: &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:    http.DefaultClient,
		Stores:  stores,
		Manager: manager,
	}, http.NewServeMux())
}

func seedStoredEvents(stores *store.Stores, runID string) {
	stores.Runs.AppendEvent(runID, 1, "run.started", `{"runId":"`+runID+`","round":0}`, "2026-11-16T10:00:00.000Z")
	stores.Runs.AppendEvent(runID, 2, "tool.started", `{"tool":"writer.text"}`, "2026-11-16T10:00:01.000Z")
	stores.Runs.AppendEvent(runID, 3, "run.completed", `{"reply":"xong"}`, "2026-11-16T10:00:02.000Z")
}

// Luot khong con trong RAM (Core vua khoi dong lai): SSE phat lai run_events tu ?after= roi ket thuc.
func TestRunEventsReplaysStoredEvents(t *testing.T) {
	handler := newRunEventsHandler(t, func(stores *store.Stores) {
		conversation := "c_1"
		stores.Runs.Insert("r_old", &conversation, "m")
		seedStoredEvents(stores, "r_old")
	}, nil)
	headers := map[string]string{"X-Auth-Token": "tok"}

	response := call(handler, "GET", "/v1/runs/r_old/events", "", headers)
	if response.Code != 200 {
		t.Fatalf("replay phai tra 200: %d %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q", got)
	}
	body := response.Body.String()
	for _, want := range []string{
		"id: 1\nevent: run.started\n", "id: 2\nevent: tool.started\n", "id: 3\nevent: run.completed\n",
		`"seq":3`, `"type":"run.completed"`, `"data":{"reply":"xong"}`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("replay thieu %q:\n%s", want, body)
		}
	}

	// ?after=1: chi su kien seq > 1.
	after := call(handler, "GET", "/v1/runs/r_old/events?after=1", "", headers).Body.String()
	if strings.Contains(after, "id: 1\n") || !strings.Contains(after, "id: 2\nevent: tool.started\n") {
		t.Fatalf("?after=1 replay sai:\n%s", after)
	}

	// Last-Event-ID tuong duong ?after=.
	resumed := call(handler, "GET", "/v1/runs/r_old/events", "",
		map[string]string{"X-Auth-Token": "tok", "Last-Event-ID": "2"}).Body.String()
	if strings.Contains(resumed, "id: 2\n") || !strings.Contains(resumed, "id: 3\nevent: run.completed\n") {
		t.Fatalf("Last-Event-ID replay sai:\n%s", resumed)
	}

	// Da o cuoi: van la luot da biet (co dong runs), stream rong va ket thuc.
	tail := call(handler, "GET", "/v1/runs/r_old/events?after=99", "", headers)
	if tail.Code != 200 || tail.Body.Len() != 0 {
		t.Fatalf("after=99 phai 200 va rong, duoc %d %q", tail.Code, tail.Body.String())
	}
}

// Su kien con nhung dong runs khong con (hoac chua tung co): van replay, khong 404.
func TestRunEventsReplaysWithoutRunRow(t *testing.T) {
	handler := newRunEventsHandler(t, func(stores *store.Stores) {
		seedStoredEvents(stores, "r_events_only")
	}, nil)

	response := call(handler, "GET", "/v1/runs/r_events_only/events", "", map[string]string{"X-Auth-Token": "tok"})
	if response.Code != 200 {
		t.Fatalf("co su kien thi phai 200: %d %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "id: 3\nevent: run.completed\n") {
		t.Fatalf("thieu su kien cuoi:\n%s", response.Body.String())
	}
}

// Chi 404 khi khong co ca dong runs lan su kien.
func TestRunEventsUnknownRunIs404(t *testing.T) {
	handler := newRunEventsHandler(t, nil, nil)
	response := call(handler, "GET", "/v1/runs/r_missing/events", "", map[string]string{"X-Auth-Token": "tok"})
	if response.Code != 404 {
		t.Fatalf("khong co gi ca phai 404: %d %s", response.Code, response.Body.String())
	}

	// Co dong runs nhung chua co su kien: 200, stream rong (luot da xong truoc khi nang cap schema).
	withRow := newRunEventsHandler(t, func(stores *store.Stores) {
		stores.Runs.Insert("r_empty", nil, "m")
	}, nil)
	if got := call(withRow, "GET", "/v1/runs/r_empty/events", "", map[string]string{"X-Auth-Token": "tok"}); got.Code != 200 {
		t.Fatalf("co dong runs thi 200 du chua co su kien: %d %s", got.Code, got.Body.String())
	}
}

// Luot con trong RAM: dung EventStream nhu cu (khong doc core.db).
func TestRunEventsUsesInMemoryStream(t *testing.T) {
	manager := agent.NewManager()
	run, problem := manager.TryStart(47901)
	if run == nil || problem != "" {
		t.Fatalf("TryStart = %+v %q", run, problem)
	}
	run.Events.Publish("run.started", map[string]any{"live": true})
	run.Events.Complete()

	handler := newRunEventsHandler(t, nil, manager)
	body := call(handler, "GET", "/v1/runs/"+run.ID+"/events", "", map[string]string{"X-Auth-Token": "tok"}).Body.String()
	if !strings.Contains(body, "id: 1\nevent: run.started\n") || !strings.Contains(body, `"live":true`) {
		t.Fatalf("phai doc tu EventStream trong RAM:\n%s", body)
	}
}
