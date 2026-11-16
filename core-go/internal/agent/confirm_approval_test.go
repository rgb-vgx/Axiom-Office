package agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"axiomoffice/core/internal/store"
)

func openAgentApprovalStore(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// Phe duyet da luu: bo qua cau hoi, khong de lai confirm.required, audit ghi "approved-by-policy".
func TestSavedApprovalSkipsConfirmation(t *testing.T) {
	db := openAgentApprovalStore(t)
	orchestrator := &Orchestrator{Approvals: db.Approvals(), Runs: db.Runs()}
	run, problem := NewManager().TryStart(47850)
	if run == nil {
		t.Fatalf("TryStart: %s", problem)
	}
	if _, err := db.Approvals().AddApproval("et.saveAs", `C:\Tai-Lieu\bao-cao.xlsx`, store.ApprovalScopeDocument, ""); err != nil {
		t.Fatalf("add approval: %v", err)
	}

	// Khac hoa/thuong + kieu dau phan cach van phai khop tai lieu.
	if !orchestrator.requestConfirmation(context.Background(), run, "et.saveAs", `c:/tai-lieu/bao-cao.xlsx`,
		"AI muốn lưu file", "{}", 50*time.Millisecond) {
		t.Fatal("phe duyet da luu phai bo qua cau hoi")
	}
	if run.Confirmations.PendingCount() != 0 {
		t.Fatalf("khong duoc de yeu cau nao cho: %d", run.Confirmations.PendingCount())
	}
	required, resolved := 0, 0
	for _, item := range run.Events.Since(0) {
		switch item.Type {
		case "confirm.required":
			required++
		case "confirm.resolved":
			resolved++
			if item.Data["approved"] != true || item.Data["by"] != "policy" {
				t.Fatalf("confirm.resolved = %+v", item.Data)
			}
		}
	}
	if required != 0 || resolved != 1 {
		t.Fatalf("required=%d resolved=%d, muon 0/1", required, resolved)
	}
	audit := db.Runs().Audit(&run.ID, 10)
	if len(audit) != 1 || audit[0].Tool != "policy.approval" || audit[0].Action == nil || *audit[0].Action != "et.saveAs" {
		t.Fatalf("audit approved-by-policy = %+v", audit)
	}
}

// Khong co phe duyet (hoac phe duyet cho tai lieu khac) thi van hoi nguoi dung nhu cu.
func TestMissingApprovalStillAsks(t *testing.T) {
	db := openAgentApprovalStore(t)
	orchestrator := &Orchestrator{Approvals: db.Approvals(), Runs: db.Runs()}
	run, _ := NewManager().TryStart(47851)
	if _, err := db.Approvals().AddApproval("et.saveAs", "/tmp/tai-lieu-khac.xlsx", store.ApprovalScopeDocument, ""); err != nil {
		t.Fatalf("add approval: %v", err)
	}
	// Het gio = tu choi (khong co approval khop).
	if orchestrator.requestConfirmation(context.Background(), run, "et.saveAs", "/tmp/bao-cao.xlsx", "ly do", "{}",
		50*time.Millisecond) {
		t.Fatal("khong co phe duyet khop thi phai hoi va het gio = tu choi")
	}
	required := 0
	for _, item := range run.Events.Since(0) {
		if item.Type == "confirm.required" {
			required++
		}
	}
	if required != 1 {
		t.Fatalf("phai hoi dung mot lan, nhan duoc %d", required)
	}
	if len(db.Runs().Audit(&run.ID, 10)) != 0 {
		t.Fatal("khong duoc ghi audit approved-by-policy khi chua duoc phe duyet")
	}
}
