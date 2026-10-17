package memory

import (
	"fmt"
	"strings"
	"testing"
)

// M4-01: xuat roi nhap lai sang DB trong -> cung so muc, ghim/han dung, tim lai duoc bang keyword.
func TestExportImportRoundTrip(t *testing.T) {
	source := newTestStore(t)
	source.AddBatch([]NewMemory{{Scope: ScopeUser, Text: "Người dùng là Trưởng phòng Kế toán"}}, ActorUser, "", nil, "")
	source.AddBatch([]NewMemory{{Scope: ScopeDocument, ScopeKey: `C:\a.docx`,
		Text: "Báo cáo tháng 10 có 3 phụ lục", ExpiresAt: "2027-01-01"}}, ActorUser, "", nil, "")

	items := source.List("", "", false, "", 10)
	pinned := true
	if _, err := source.Update(items[0].ID, "", "", "", false, &pinned, "test"); err != nil {
		t.Fatalf("pin: %v", err)
	}

	rows := source.Export()
	if len(rows) != 2 {
		t.Fatalf("export = %d muc", len(rows))
	}

	target := newTestStore(t)
	result, err := target.Import(rows)
	if err != nil || result.Added != 2 {
		t.Fatalf("import = %+v, %v", result, err)
	}
	after := target.List("", "", false, "", 10)
	if len(after) != 2 {
		t.Fatalf("sau import = %d muc", len(after))
	}
	foundPinned, foundExpiry := false, false
	for _, item := range after {
		if item.Pinned {
			foundPinned = true
		}
		if item.ExpiresAt != nil && *item.ExpiresAt == "2027-01-01" {
			foundExpiry = true
		}
	}
	if !foundPinned || !foundExpiry {
		t.Fatalf("thieu ghim/han dung sau import: %+v", after)
	}
	if hits := NewRetriever(target).Search("trưởng phòng kế toán", "", Today(), nil, ScopeUser, 5); len(hits) == 0 {
		t.Fatal("khong tim lai duoc bang keyword sau import")
	}

	// Import lai chinh no -> chong trung theo hash, khong nhan doi.
	again, err := target.Import(rows)
	if err != nil || again.Added != 0 || again.Skipped != 2 {
		t.Fatalf("import lai = %+v, %v", again, err)
	}
}

// M4-02: xoa mem theo tai lieu, khong dung cham tai lieu khac.
func TestSoftDeleteByScopeKey(t *testing.T) {
	repo := newTestStore(t)
	repo.AddBatch([]NewMemory{{Scope: ScopeDocument, ScopeKey: `C:\a.docx`, Text: "Báo cáo tháng 10 có 3 phụ lục"}}, ActorUser, "", nil, "")
	repo.AddBatch([]NewMemory{{Scope: ScopeDocument, ScopeKey: `C:\b.docx`, Text: "Bảng điểm lớp 4A có 35 học sinh"}}, ActorUser, "", nil, "")

	if removed := repo.SoftDeleteByScopeKey(`C:\a.docx`, "test"); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if items := repo.List(ScopeDocument, `C:\a.docx`, false, "", 10); len(items) != 0 {
		t.Fatalf("tai lieu a phai rong: %+v", items)
	}
	if items := repo.List(ScopeDocument, `C:\b.docx`, false, "", 10); len(items) != 1 {
		t.Fatalf("tai lieu b phai con: %+v", items)
	}
}

// M4-04: tat rieng memory tai lieu / ca nhan co hieu luc ngay, khong xoa du lieu.
func TestScopeFilterDisablesScopes(t *testing.T) {
	repo := newTestStore(t)
	repo.AddBatch([]NewMemory{{Scope: ScopeUser, Text: "Người dùng là Trưởng phòng Kế toán"}}, ActorUser, "", nil, "")
	repo.AddBatch([]NewMemory{{Scope: ScopeDocument, ScopeKey: `C:\a.docx`, Text: "Báo cáo tháng 10 có 3 phụ lục"}}, ActorUser, "", nil, "")
	retriever := NewRetriever(repo)
	query := "Trưởng phòng Kế toán báo cáo tháng 10"

	if all := retriever.BuildContext(query, `C:\a.docx`, Today(), nil); len(all.Lines) != 2 {
		t.Fatalf("du 2 fact: %+v", all.Lines)
	}
	noDoc := retriever.BuildContext(query, `C:\a.docx`, Today(), nil, ScopeFilter{User: true})
	for _, line := range noDoc.Lines {
		if strings.Contains(line, "Báo cáo tháng 10") {
			t.Fatalf("tat tai lieu ma van con: %+v", noDoc.Lines)
		}
	}
	noUser := retriever.BuildContext(query, `C:\a.docx`, Today(), nil, ScopeFilter{Document: true})
	for _, line := range noUser.Lines {
		if strings.Contains(line, "Trưởng phòng Kế toán") {
			t.Fatalf("tat ca nhan ma van con: %+v", noUser.Lines)
		}
	}
	if items := repo.List("", "", false, "", 10); len(items) != 2 {
		t.Fatalf("tat ngu canh khong duoc xoa du lieu: %+v", items)
	}
}

// M5-01: fact gan trung da lot qua luc ghi (chua co vector khi ghi) bi gop trong mot lan don.
func TestCleanupMergesNearDuplicates(t *testing.T) {
	repo := newTestStore(t)
	const seeded = 200
	for i := 0; i < seeded; i++ {
		repo.AddBatch([]NewMemory{{Scope: ScopeUser, Text: fmt.Sprintf("Người dùng thích mẫu báo cáo kiểu %d", i)}},
			ActorExtract, "", nil, "")
	}
	// Gia lap embedding giong het nhau (ban ghi da vao truoc khi co embedding).
	for _, item := range repo.List(ScopeUser, "", false, "", 10_000) {
		if err := saveVector(repo.db.SQL(), item.ID, "m", []float32{1, 0}, "h"); err != nil {
			t.Fatal(err)
		}
	}
	stats := repo.Cleanup(Today())
	if stats.Merged == 0 {
		t.Fatal("khong gop duoc ban gan trung")
	}
	if left := len(repo.List(ScopeUser, "", false, "", 10_000)); left >= seeded/2 {
		t.Fatalf("sau don con %d, mong doi < %d", left, seeded/2)
	}
}

// M5-01/M5-02: fact het han bi xoa mem; org-style khong bi dong toi.
func TestCleanupExpiresAndKeepsOrgStyle(t *testing.T) {
	repo := newTestStore(t)
	repo.AddBatch([]NewMemory{{Scope: ScopeUser, Text: "Người dùng là Trưởng phòng Kế toán"}}, ActorUser, "", nil, "")
	repo.AddBatch([]NewMemory{{Scope: ScopeDocument, ScopeKey: `C:\a.docx`, Text: "Báo cáo tháng 10 có 3 phụ lục",
		ExpiresAt: "2020-01-01"}}, ActorUser, "", nil, "")
	repo.AddBatch([]NewMemory{{Scope: ScopeOrgStyle, Text: "Đơn vị dùng Times New Roman 13"}}, ActorUser, "", nil, "")

	stats := repo.Cleanup(Today())
	if stats.Expired != 1 {
		t.Fatalf("expired = %d, want 1", stats.Expired)
	}
	if items := repo.List(ScopeOrgStyle, "", false, "", 10); len(items) != 1 {
		t.Fatalf("org-style khong duoc bi don: %+v", items)
	}
	if items := repo.List(ScopeDocument, `C:\a.docx`, false, "", 10); len(items) != 0 {
		t.Fatalf("fact het han phai bi xoa mem: %+v", items)
	}
}
