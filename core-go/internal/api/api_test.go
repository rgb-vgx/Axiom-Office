package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/store"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	root := t.TempDir()
	paths := config.NewPaths(root)
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	return Handler(&Deps{
		Config:  config.Config{Token: "tok", MemoryEnabled: true, LlmProvider: "openai", LlmModel: "m", LlmApiKey: "secret-key"},
		Paths:   paths,
		Runtime: &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:    http.DefaultClient,
		Stores:  stores,
	}, http.NewServeMux())
}

func call(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestGuard(t *testing.T) {
	handler := newHandler(t)
	json := map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}
	cases := []struct {
		method, path, body string
		headers            map[string]string
		want               int
	}{
		{"GET", "/health", "", nil, 200},
		{"GET", "/health", "", map[string]string{"Origin": "http://evil"}, 403},
		{"GET", "/v1/setup", "", nil, 401},
		{"GET", "/v1/setup", "", map[string]string{"X-Auth-Token": "wrong"}, 401},
		{"GET", "/v1/setup", "", map[string]string{"X-Auth-Token": "tok"}, 200},
		{"POST", "/v1/llm/test", "x", map[string]string{"X-Auth-Token": "tok", "Content-Type": "text/plain"}, 415},
		{"POST", "/v1/llm/test", "{bad", json, 400},
	}
	for _, c := range cases {
		if got := call(handler, c.method, c.path, c.body, c.headers).Code; got != c.want {
			t.Errorf("%s %s %v -> %d, want %d", c.method, c.path, c.headers, got, c.want)
		}
	}
}

func TestHealthAndSetupShape(t *testing.T) {
	handler := newHandler(t)
	var health struct {
		OK     bool           `json:"ok"`
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal(call(handler, "GET", "/health", "", nil).Body.Bytes(), &health)
	if !health.OK || health.Result["app"] != "core" || health.Result["memory"] != "on" || health.Result["protocol"] != float64(1) {
		t.Fatalf("health = %+v", health)
	}
	body := call(handler, "GET", "/v1/setup", "", map[string]string{"X-Auth-Token": "tok"}).Body.String()
	if strings.Contains(body, "secret-key") {
		t.Fatal("/v1/setup must never return the API key")
	}
	for _, want := range []string{`"hasKey":true`, `"id":"company"`, `"id":"welcome"`, `"configured":false`, `"id":"config"`} {
		if !strings.Contains(body, want) {
			t.Errorf("/v1/setup missing %s", want)
		}
	}
}

func TestLlmTestMissingEndpoint(t *testing.T) {
	handler := newHandler(t)
	body := call(handler, "POST", "/v1/llm/test", `{"model":"m"}`,
		map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}).Body.String()
	if !strings.Contains(body, `"kind":"config"`) {
		t.Fatalf("body = %s", body)
	}
}
