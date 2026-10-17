package api

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/memory"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/store"
)

// newMemoryServer: handler co dang ky route memory. admin la co cho phep ghi org-style (doc lai moi luot).
func newMemoryServer(t *testing.T, admin *bool) (http.Handler, *memory.Service) {
	t.Helper()
	root := t.TempDir()
	paths := config.NewPaths(root)
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	load := func() config.Config {
		return config.Config{Token: "tok", MemoryEnabled: true, OrgStyleAdmin: *admin,
			MemoryDocumentEnabled: true, MemoryUserEnabled: true}
	}
	service := memory.NewService(stores.DB, stores.Convs, stores.Runs, load,
		func(config.Config) *model.Client { return nil }, http.DefaultClient, true)
	t.Cleanup(service.Close)
	handler := Handler(&Deps{
		Config: load(), LoadConfig: load, Paths: paths, Runtime: &Runtime{Port: 1234},
		HTTP: http.DefaultClient, Stores: stores, Memory: service,
	}, http.NewServeMux())
	return handler, service
}

// M3-04: nguoi dung thuong khong ghi duoc chuan chung cua don vi; quan tri vien thi duoc.
func TestOrgStyleRequiresAdmin(t *testing.T) {
	admin := false
	handler, _ := newMemoryServer(t, &admin)
	json := map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}
	body := `{"scope":"org-style","text":"Đơn vị dùng Times New Roman 13, lề trái 3 cm"}`

	if code := call(handler, "POST", "/v1/memory", body, json).Code; code != http.StatusForbidden {
		t.Fatalf("nguoi thuong ghi org-style = %d, want 403", code)
	}
	admin = true
	if code := call(handler, "POST", "/v1/memory", body, json).Code; code != http.StatusOK {
		t.Fatalf("quan tri vien ghi org-style = %d, want 200", code)
	}
}

// M4-01: export roi import sang may khac qua API.
func TestMemoryExportImportAPI(t *testing.T) {
	admin := false
	handler, _ := newMemoryServer(t, &admin)
	json := map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}
	if code := call(handler, "POST", "/v1/memory", `{"scope":"user","text":"Người dùng là Trưởng phòng Kế toán"}`, json).Code; code != http.StatusOK {
		t.Fatalf("tao memory = %d", code)
	}
	exported := call(handler, "GET", "/v1/memory/export", "", map[string]string{"X-Auth-Token": "tok"}).Body.String()
	if !strings.Contains(exported, `"count":1`) || !strings.Contains(exported, "Trưởng phòng Kế toán") {
		t.Fatalf("export = %s", exported)
	}
	result := call(handler, "POST", "/v1/memory/import", `{"items":[{"scope":"user","text":"Người dùng là Phó giám đốc"}]}`, json).Body.String()
	if !strings.Contains(result, `"added":1`) {
		t.Fatalf("import = %s", result)
	}
}

// M4-02: xoa mem theo tai lieu qua API.
func TestMemoryDeleteByDocumentAPI(t *testing.T) {
	admin := false
	handler, _ := newMemoryServer(t, &admin)
	json := map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}
	call(handler, "POST", "/v1/memory", `{"scope":"document","scopeKey":"C:\\a.docx","text":"Báo cáo tháng 10 có 3 phụ lục"}`, json)
	removed := call(handler, "DELETE", "/v1/memory?documentKey=C%3A%5Ca.docx&confirm=true", "", json).Body.String()
	if !strings.Contains(removed, `"removed":1`) {
		t.Fatalf("xoa theo tai lieu = %s", removed)
	}
}
