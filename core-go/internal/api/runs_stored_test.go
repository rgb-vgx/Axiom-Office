package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/store"
)

// Luot khong con trong RAM (Core vua khoi dong lai): GET /v1/runs/{id} tra trang thai da luu thay vi 404.
func TestGetRunFallsBackToStoredRun(t *testing.T) {
	root := t.TempDir()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	conversation := "c_1"
	stores.Runs.Insert("r_old", &conversation, "m")
	stores.Runs.MarkInterrupted()
	handler := Handler(&Deps{
		Config:  config.Config{Token: "tok", MemoryEnabled: true},
		Paths:   config.NewPaths(root),
		Runtime: &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:    http.DefaultClient,
		Stores:  stores,
		Manager: agent.NewManager(),
	}, http.NewServeMux())
	headers := map[string]string{"X-Auth-Token": "tok"}

	response := call(handler, "GET", "/v1/runs/r_old", "", headers)
	var body struct {
		Result map[string]any `json:"result"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &body) != nil {
		t.Fatalf("luot da luu phai tra 200: %d %s", response.Code, response.Body.String())
	}
	if body.Result["status"] != store.StatusInterrupted || body.Result["stored"] != true || body.Result["conversationId"] != "c_1" {
		t.Fatalf("phai tra trang thai luu trong core.db: %v", body.Result)
	}
	if got := call(handler, "GET", "/v1/runs/r_none", "", headers).Code; got != 404 {
		t.Fatalf("luot khong co o dau ca -> 404, co %d", got)
	}
}
