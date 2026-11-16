package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openApprovalStore(t *testing.T) *Approvals {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db.Approvals()
}

func TestApprovalsCRUD(t *testing.T) {
	approvals := openApprovalStore(t)
	if rows := approvals.ListApprovals(); len(rows) != 0 {
		t.Fatalf("DB moi phai rong: %+v", rows)
	}

	first, err := approvals.AddApproval("et.saveAs", `/tmp/bao-cao.xlsx`, ApprovalScopeDocument, "")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if first.ID == "" || first.CreatedAt == "" || first.Scope != ApprovalScopeDocument {
		t.Fatalf("row = %+v", first)
	}
	second, err := approvals.AddApproval("wpp.deleteSlide", "*", ApprovalScopeGlobal, "")
	if err != nil {
		t.Fatalf("add global: %v", err)
	}

	rows := approvals.ListApprovals()
	if len(rows) != 2 || rows[0].ID != second.ID || rows[1].ID != first.ID {
		t.Fatalf("list phai moi nhat truoc: %+v", rows)
	}

	if !approvals.DeleteApproval(first.ID) {
		t.Fatal("xoa phe duyet ton tai phai tra true")
	}
	if approvals.DeleteApproval(first.ID) {
		t.Fatal("xoa lan hai phai tra false")
	}
	if rows := approvals.ListApprovals(); len(rows) != 1 || rows[0].ID != second.ID {
		t.Fatalf("list sau xoa = %+v", rows)
	}
}

func TestActiveApproval(t *testing.T) {
	approvals := openApprovalStore(t)
	if active := approvals.ActiveApproval("et.saveAs", `/tmp/a.xlsx`); active != nil {
		t.Fatalf("chua co phe duyet: %+v", active)
	}

	document, err := approvals.AddApproval("et.saveAs", `C:\Tai-Lieu\bao-cao.xlsx`, ApprovalScopeDocument, "")
	if err != nil {
		t.Fatal(err)
	}
	// Cung tai lieu: khac hoa/thuong va kieu dau phan cach van khop.
	if active := approvals.ActiveApproval("ET.SAVEAS", `c:/tai-lieu/bao-cao.xlsx`); active == nil || active.ID != document.ID {
		t.Fatalf("phe duyet theo tai lieu phai khop: %+v", active)
	}
	// Tai lieu khac thi khong.
	if active := approvals.ActiveApproval("et.saveAs", `/tmp/tai-lieu-khac.xlsx`); active != nil {
		t.Fatalf("tai lieu khac khong duoc khop: %+v", active)
	}
	// Action khac thi khong.
	if active := approvals.ActiveApproval("writer.save", `C:\Tai-Lieu\bao-cao.xlsx`); active != nil {
		t.Fatalf("action khac khong duoc khop: %+v", active)
	}

	if _, err := approvals.AddApproval("wpp.deleteSlide", "", ApprovalScopeGlobal, ""); err != nil {
		t.Fatal(err)
	}
	if active := approvals.ActiveApproval("wpp.deleteSlide", `/bat-ky/bai.pptx`); active == nil {
		t.Fatal("scope global phai khop moi tai lieu")
	}

	// Het han (va moc thoi gian hong) khong con hieu luc.
	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if _, err := approvals.AddApproval("et.deleteSheet", "", ApprovalScopeGlobal, past); err != nil {
		t.Fatal(err)
	}
	if active := approvals.ActiveApproval("et.deleteSheet", "Book1"); active != nil {
		t.Fatalf("phe duyet het han khong duoc khop: %+v", active)
	}
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if _, err := approvals.AddApproval("et.replaceAll", "", ApprovalScopeGlobal, future); err != nil {
		t.Fatal(err)
	}
	if active := approvals.ActiveApproval("et.replaceAll", ""); active == nil {
		t.Fatal("phe duyet con han phai khop")
	}
	if _, err := approvals.AddApproval("et.replaceAll", "x", ApprovalScopeGlobal, "khong-phai-rfc3339"); err != nil {
		t.Fatal(err)
	}
	// Ban moi nhat (hong moc thoi gian) khong duoc khop -> roi xuong ban con han.
	if active := approvals.ActiveApproval("et.replaceAll", "x"); active == nil {
		t.Fatal("moc thoi gian hong phai bi bo qua, khong chan ban con han")
	}
}
