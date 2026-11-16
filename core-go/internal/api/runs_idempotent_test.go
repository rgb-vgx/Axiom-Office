package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/store"
)

// POST /v1/runs voi cung clientRequestId: tra ngay conversationId (khong cho), va lan gui lai
// tra dung luot cu - khong ghi them dong runs/hoi thoai.
func TestStartRunIdempotentByClientRequestID(t *testing.T) {
	root := t.TempDir()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })

	// Khong co phien Office trong thu muc session: Execute dung ngay sau khi hoi thoai da duoc
	// Admit, du de kiem tra idempotency cua duong nhan luot.
	orchestrator := &agent.Orchestrator{
		Sessions:      office.NewDirectory(filepath.Join(root, "sessions")),
		Conversations: stores.Convs,
		Runs:          stores.Runs,
		Models:        model.NewSource(http.DefaultClient, func() config.Config { return config.Config{} }),
		Config:        func() config.Config { return config.Config{} },
		Assembler:     agent.NewContextAssembler(),
	}
	handler := Handler(&Deps{
		Config:       config.Config{Token: "tok", MemoryEnabled: true},
		Paths:        config.NewPaths(root),
		Runtime:      &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:         http.DefaultClient,
		Stores:       stores,
		Manager:      agent.NewManager(),
		Orchestrator: orchestrator,
	}, http.NewServeMux())

	headers := map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}
	body := `{"prompt":"hello","office":{"port":47861},"clientRequestId":"req-1"}`

	first := call(handler, "POST", "/v1/runs", body, headers)
	firstResult := decodeRunStart(t, first)
	runID, _ := firstResult["runId"].(string)
	conversationID, _ := firstResult["conversationId"].(string)
	if runID == "" || conversationID == "" {
		t.Fatalf("phai tra runId + conversationId ngay: %v", firstResult)
	}
	if firstResult["status"] != agent.StatusRunning {
		t.Fatalf("status = %v, want %s", firstResult["status"], agent.StatusRunning)
	}
	if stored := stores.Runs.Get(runID); stored == nil || stored.ConversationID == nil || *stored.ConversationID != conversationID {
		t.Fatalf("dong runs phai duoc tao truoc khi Execute chay: %+v", stored)
	}

	second := call(handler, "POST", "/v1/runs", body, headers)
	secondResult := decodeRunStart(t, second)
	if secondResult["runId"] != runID || secondResult["conversationId"] != conversationID {
		t.Fatalf("gui lai phai tra dung luot cu: %v != %v", secondResult, firstResult)
	}

	if count := countRows(t, stores, "runs"); count != 1 {
		t.Fatalf("runs rows = %d, want 1", count)
	}
	if count := countRows(t, stores, "conversations"); count != 1 {
		t.Fatalf("conversations rows = %d, want 1", count)
	}
}

func decodeRunStart(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Result map[string]any `json:"result"`
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Result == nil {
		t.Fatalf("body = %s, err = %v", recorder.Body.String(), err)
	}
	return body.Result
}

func countRows(t *testing.T, stores *store.Stores, table string) int {
	t.Helper()
	var count int
	if err := stores.DB.SQL().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
