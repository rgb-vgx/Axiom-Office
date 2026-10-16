package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func testWebSources(t *testing.T) *WebSources {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db.WebSources()
}

func TestWebSourcesSaveDedupe(t *testing.T) {
	sources := testWebSources(t)
	saved := sources.SaveDocs("r_1", "c_1", "research", "visa uc", []WebSourceDoc{
		{URL: "https://Visa5s.com.vn/visa-nong-nghiep-uc-403?utm_source=fb#top",
			Title: "Visa nong nghiep", Lang: "vi", Mode: "full", OK: true,
			Content: "Úc mở rộng diện visa nông nghiệp cho lao động Việt Nam"},
	})
	if saved != 1 {
		t.Fatalf("saved = %d, want 1", saved)
	}
	// Cung URL nhung khac tham so theo doi + fragment + www -> phai gop lam mot nguon, them ref.
	saved = sources.SaveDocs("r_2", "c_1", "search", "visa", []WebSourceDoc{
		{URL: "https://www.visa5s.com.vn/visa-nong-nghiep-uc-403/", Title: "Visa NN", Mode: "search", OK: true, Snippet: "trang dich vu visa"},
	})
	if saved != 1 {
		t.Fatalf("saved(2) = %d, want 1", saved)
	}
	if count := sources.Count(); count != 1 {
		t.Fatalf("count = %d, want 1 (URL trung phai gop)", count)
	}
	row, content := sources.Get("https://www.visa5s.com.vn/visa-nong-nghiep-uc-403?utm_campaign=x")
	if row == nil {
		t.Fatal("Get theo URL chuan hoa khong thay nguon")
	}
	if row.FetchCount != 2 {
		t.Fatalf("fetch_count = %d, want 2", row.FetchCount)
	}
	if row.Title != "Visa NN" {
		t.Fatalf("title = %q, want ban moi nhat", row.Title)
	}
	if !strings.Contains(content, "nông nghiệp") {
		t.Fatalf("content mat ban day du: %q", content)
	}
	if row.Domain != "visa5s.com.vn" {
		t.Fatalf("domain = %q", row.Domain)
	}
}

func TestWebSourcesSearchBoDau(t *testing.T) {
	sources := testWebSources(t)
	sources.SaveDocs("r_1", "c_1", "fetch", "q", []WebSourceDoc{
		{URL: "https://example.com/a", Title: "Chẩn đoán hình ảnh", Mode: "full", OK: true,
			Content: "Ứng dụng AI trong chẩn đoán hình ảnh tại bệnh viện Bạch Mai và Vinmec năm 2025"},
	})
	for _, query := range []string{"chan doan hinh anh", "chẩn đoán hình ảnh", "Bach Mai"} {
		hits := sources.Search(query, 8)
		if len(hits) != 1 {
			t.Fatalf("Search(%q) = %d ket qua, want 1", query, len(hits))
		}
	}
	if hits := sources.Search("tai chinh ngan hang", 8); len(hits) != 0 {
		t.Fatalf("Search tu khoa la = %d ket qua, want 0", len(hits))
	}
	// Truy van rong -> tra nguon moi nhat (khong loi).
	if hits := sources.Search("", 8); len(hits) != 1 {
		t.Fatalf("Search rong = %d ket qua, want 1", len(hits))
	}
}

func TestWebSourcesRecentByConversation(t *testing.T) {
	sources := testWebSources(t)
	sources.SaveDocs("r_1", "c_1", "research", "q1", []WebSourceDoc{
		{URL: "https://a.example/1", Title: "A", Mode: "lead", OK: true, Content: "noi dung A"},
	})
	sources.SaveDocs("r_2", "c_2", "research", "q2", []WebSourceDoc{
		{URL: "https://b.example/2", Title: "B", Mode: "lead", OK: true, Content: "noi dung B"},
	})
	hits := sources.RecentByConversation("c_1", 8)
	if len(hits) != 1 || hits[0].URL != "https://a.example/1" {
		t.Fatalf("RecentByConversation(c_1) = %+v", hits)
	}
	if len(sources.Recent(8)) != 2 {
		t.Fatal("Recent phai tra ca hai hoi thoai")
	}
}

func TestWebSourcesPurgeBySize(t *testing.T) {
	sources := testWebSources(t)
	big := strings.Repeat("x", 5000)
	sources.SaveDocs("r_1", "c_1", "fetch", "q", []WebSourceDoc{
		{URL: "https://old.example/1", Title: "Old", Mode: "full", OK: true, Content: big},
	})
	sources.SaveDocs("r_2", "c_2", "fetch", "q", []WebSourceDoc{
		{URL: "https://new.example/2", Title: "New", Mode: "full", OK: true, Content: big},
	})
	if total := sources.Bytes(); total != 10000 {
		t.Fatalf("bytes = %d, want 10000", total)
	}
	// Tran 6000 byte -> muc tieu 5400 (90%) -> xoa nguon cu nhat (5000), con 5000 <= 5400.
	if removed := sources.Purge(0, 6000); removed != 1 {
		t.Fatalf("purge = %d, want 1", removed)
	}
	rows := sources.Recent(8)
	if len(rows) != 1 || rows[0].URL != "https://new.example/2" {
		t.Fatalf("sau purge con %+v, want nguon moi", rows)
	}
}

func TestWebSourcesFailedFetchKeepsRef(t *testing.T) {
	sources := testWebSources(t)
	sources.SaveDocs("r_1", "c_1", "research", "q", []WebSourceDoc{
		{URL: "https://die.example/1", Title: "Die", Mode: "fail", OK: false, Error: "timeout"},
	})
	row, content := sources.Get("https://die.example/1")
	if row == nil {
		t.Fatal("nguon loi van phai co dong trong kho (de biet da tung gap)")
	}
	if content != "" {
		t.Fatalf("content = %q, want rong", content)
	}
	var refs, okFlag int
	if err := sources.db.sql.QueryRow(
		"SELECT COUNT(*), COALESCE(MAX(ok), -1) FROM web_source_refs WHERE source_id = ?", row.ID).
		Scan(&refs, &okFlag); err != nil {
		t.Fatalf("query refs: %v", err)
	}
	if refs != 1 || okFlag != 0 {
		t.Fatalf("refs = %d ok = %d, want 1/0", refs, okFlag)
	}
}
