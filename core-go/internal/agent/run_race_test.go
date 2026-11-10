package agent

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
)

// Execute chay tren goroutine rieng va ghi trang thai luot (Status, Error, Finished...) trong khi
// GET /v1/runs/{id} (JSON), Manager (Running/TryStart/cleanup) va pane doc cung luc. Chay voi -race:
// truoc khi Run co khoa rieng, test nay bao DATA RACE.
func TestRunReadableWhileExecuteWrites(t *testing.T) {
	manager := NewManager()
	run, busy := manager.TryStart(41000)
	if run == nil {
		t.Fatal(busy)
	}
	orchestrator := &Orchestrator{
		Sessions: office.NewDirectory(filepath.Join(t.TempDir(), "sessions")),
		Models:   model.NewSource(http.DefaultClient, func() config.Config { return config.Config{} }),
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Khong co session cho cong nay -> fail + finish: ghi Status, Error, ErrorKind, Finished.
		orchestrator.Execute(context.Background(), run, &Request{Port: 41000})
	}()
	for finished := false; !finished; {
		select {
		case <-done:
			finished = true
		default:
		}
		_ = run.JSON()
		_ = manager.Running()
		_, _ = manager.TryStart(41000)
		_ = run.CurrentStatus()
	}
	if status := run.CurrentStatus(); status != StatusFailed {
		t.Fatalf("status = %q, want %q", status, StatusFailed)
	}
	if payload := run.JSON(); payload["errorKind"] != "office" || payload["finished"] == nil {
		t.Fatalf("payload = %v", payload)
	}
}
