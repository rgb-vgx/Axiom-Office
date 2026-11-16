package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/store"
)

// newApprovalHandler: Deps co them Manager de kiem tra POST /v1/runs/{id}/confirm always=true.
func newApprovalHandler(t *testing.T) (*Deps, http.Handler) {
	t.Helper()
	root := t.TempDir()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	deps := &Deps{
		Config:  config.Config{Token: "tok", MemoryEnabled: true},
		Paths:   config.NewPaths(root),
		Runtime: &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:    http.DefaultClient,
		Stores:  stores,
		Manager: agent.NewManager(),
	}
	return deps, Handler(deps, http.NewServeMux())
}

func decodeOK(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload struct {
		OK     bool           `json:"ok"`
		Result map[string]any `json:"result"`
		Error  string         `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("body hong: %v (%s)", err, recorder.Body.String())
	}
	if !payload.OK {
		t.Fatalf("ok=false: %s", payload.Error)
	}
	return payload.Result
}

// API phe duyet: envelope {"ok":true,"result":...}, CRUD day du, ca duong /wps/v1 (routeBoth).
func TestApprovalsAPIEnvelope(t *testing.T) {
	_, handler := newApprovalHandler(t)
	auth := map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}

	list := decodeOK(t, call(handler, "GET", "/v1/approvals", "", auth))
	if approvals, ok := list["approvals"].([]any); !ok || len(approvals) != 0 {
		t.Fatalf("danh sach rong ban dau: %+v", list)
	}

	created := decodeOK(t, call(handler, "POST", "/v1/approvals",
		`{"action":"et.saveAs","resource":"C:\\Tai-Lieu\\bao-cao.xlsx","scope":"document","expiresAt":"2030-01-02T03:04:05Z"}`, auth))
	id, _ := created["id"].(string)
	if id == "" || created["action"] != "et.saveAs" || created["scope"] != "document" ||
		created["expiresAt"] != "2030-01-02T03:04:05Z" || created["createdAt"] == nil {
		t.Fatalf("POST result = %+v", created)
	}

	// Khong scope -> document; resource rong -> "*".
	minimal := decodeOK(t, call(handler, "POST", "/v1/approvals", `{"action":"wpp.deleteSlide"}`, auth))
	if minimal["scope"] != "document" || minimal["resource"] != "*" || minimal["expiresAt"] != nil {
		t.Fatalf("POST toi thieu = %+v", minimal)
	}

	list = decodeOK(t, call(handler, "GET", "/v1/approvals", "", auth))
	if approvals, ok := list["approvals"].([]any); !ok || len(approvals) != 2 {
		t.Fatalf("GET sau khi tao = %+v", list)
	}

	// WPS: cung endpoint qua routeBoth (token khi khong co Origin).
	deleted := decodeOK(t, call(handler, "DELETE", "/wps/v1/approvals/"+id, "", map[string]string{"X-Auth-Token": "tok"}))
	if deleted["deleted"] != id {
		t.Fatalf("DELETE qua /wps/v1 = %+v", deleted)
	}
	if code := call(handler, "DELETE", "/v1/approvals/"+id, "", auth).Code; code != http.StatusNotFound {
		t.Fatalf("xoa lan hai = %d, muon 404", code)
	}

	// Thieu action / scope la / expiresAt hong -> 400.
	for _, body := range []string{
		`{}`,
		`{"action":"et.saveAs","scope":"everything"}`,
		`{"action":"et.saveAs","expiresAt":"ngay mai"}`,
		// Lenh ghi chung khong duoc luu "luon dong y": se bo qua guard ghi de/sort.
		`{"action":"et.writeRange"}`,
	} {
		if code := call(handler, "POST", "/v1/approvals", body, auth).Code; code != http.StatusBadRequest {
			t.Errorf("POST %s = %d, muon 400", body, code)
		}
	}
}

// POST /v1/runs/{id}/confirm voi always=true: dong y nhu cu, va luu phe duyet cho action+resource.
func TestConfirmAlwaysPersistsApproval(t *testing.T) {
	deps, handler := newApprovalHandler(t)
	run, problem := deps.Manager.TryStart(47860)
	if run == nil {
		t.Fatalf("TryStart: %s", problem)
	}
	auth := map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}

	finished := make(chan bool, 1)
	go func() {
		finished <- run.Confirmations.RequestFor(context.Background(), run.Events, "et.saveAs", "/tmp/bao-cao.xlsx",
			"AI muốn lưu file", "{}", 2*time.Second)
	}()
	confirmationID := waitConfirmationID(t, run, 2*time.Second)
	body := `{"confirmationId":"` + confirmationID + `","approved":true,"always":true}`
	result := decodeOK(t, call(handler, "POST", "/v1/runs/"+run.ID+"/confirm", body, auth))
	if result["approved"] != true || result["always"] != true {
		t.Fatalf("confirm result = %+v", result)
	}
	if approval, ok := result["approval"].(map[string]any); !ok || approval["action"] != "et.saveAs" ||
		approval["resource"] != "/tmp/bao-cao.xlsx" || approval["scope"] != "document" {
		t.Fatalf("approval trong response = %+v", result["approval"])
	}
	if !<-finished {
		t.Fatal("nguoi dung dong y -> Request phai tra true")
	}

	rows := deps.Stores.Approvals.ListApprovals()
	if len(rows) != 1 || rows[0].Action != "et.saveAs" || rows[0].Resource != "/tmp/bao-cao.xlsx" ||
		rows[0].Scope != store.ApprovalScopeDocument || rows[0].ExpiresAt != "" {
		t.Fatalf("phe duyet da luu = %+v", rows)
	}
	if active := deps.Stores.Approvals.ActiveApproval("et.saveAs", "/tmp/bao-cao.xlsx"); active == nil {
		t.Fatal("phe duyet vua luu phai con hieu luc")
	}

	// always=false (hoac tu choi) khong luu them.
	run2, _ := deps.Manager.TryStart(47861)
	go func() {
		finished <- run2.Confirmations.RequestFor(context.Background(), run2.Events, "wpp.deleteSlide", "/tmp/bai.pptx",
			"Xoá slide", "{}", 2*time.Second)
	}()
	confirmationID = waitConfirmationID(t, run2, 2*time.Second)
	decodeOK(t, call(handler, "POST", "/v1/runs/"+run2.ID+"/confirm",
		`{"confirmationId":"`+confirmationID+`","approved":true}`, auth))
	<-finished
	if rows := deps.Stores.Approvals.ListApprovals(); len(rows) != 1 {
		t.Fatalf("always=false khong duoc luu phe duyet: %+v", rows)
	}
}

func waitConfirmationID(t *testing.T, run *agent.Run, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, item := range run.Events.Since(0) {
			if item.Type == "confirm.required" {
				if id, _ := item.Data["confirmationId"].(string); id != "" {
					return id
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("khong thay confirm.required")
	return ""
}
