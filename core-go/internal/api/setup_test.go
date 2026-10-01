package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/store"
)

// "Thu ket noi" phai dung khoa MOI NHAT, khong phai anh chup luc Core khoi dong: nguoi dung hay
// nhap khoa trong wizard sau khi Core da chay (khoa nam trong keyring tren Linux, HKCU/DPAPI tren
// Windows). Truoc day endpoint lay deps.Config.LlmApiKey nen gui di ma khong co khoa -> 401
// "Missing API key" trong khi cac luot chay that van chay duoc (chung doc config song).
func TestLlmTestUsesLiveApiKey(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		// Kem ca duoi `data: [DONE]` nhu proxy that, de test phu luon ca phan chiu duoi rac.
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"OK"}}],"usage":{}}data: [DONE]`))
	}))
	defer server.Close()

	root := t.TempDir()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })

	handler := Handler(&Deps{
		// Anh chup luc khoi dong KHONG co khoa - dung nhu may Linux luc 23:15.
		Config:     config.Config{Token: "tok"},
		LoadConfig: func() config.Config { return config.Config{Token: "tok", LlmApiKey: "live-key"} },
		Paths:      config.NewPaths(root),
		Runtime:    &Runtime{},
		HTTP:       server.Client(),
		Stores:     stores,
	}, http.NewServeMux())

	body := call(handler, "POST", "/v1/llm/test",
		`{"provider":"openai","endpoint":"`+server.URL+`","model":"m"}`,
		map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}).Body.String()

	if authorization != "Bearer live-key" {
		t.Fatalf("Authorization = %q (phai lay khoa tu LoadConfig, khong phai anh chup); body = %s", authorization, body)
	}
	if !strings.Contains(body, `"reply":"OK"`) {
		t.Fatalf("body = %s", body)
	}
}
