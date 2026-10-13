package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tavilyServer(t *testing.T, status int, reply string, seen *map[string]any, auth *string) *WebSearchTool {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		*auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(seen)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(server.Close)
	tool := NewWebSearchTool("tvly-secret")
	tool.Endpoint = server.URL
	tool.Client = server.Client()
	return tool
}

func TestWebSearchCallsTavilyAndTrimsResults(t *testing.T) {
	seen, auth := map[string]any{}, ""
	long := strings.Repeat("x", 2000)
	tool := tavilyServer(t, 200, `{"answer":" GDP 2025 tang 7%. ","results":[`+
		`{"title":"Tong cuc Thong ke","url":"https://gso.gov.vn/a","content":"`+long+`","score":0.9}]}`, &seen, &auth)
	result := tool.Invoke(context.Background(), map[string]any{"query": " GDP Viet Nam 2025 ", "max_results": float64(20), "topic": "news"}, nil)
	if !result.OK || result.Mutating {
		t.Fatalf("expected ok read-only result: %+v", result)
	}
	if auth != "Bearer tvly-secret" {
		t.Fatalf("Authorization = %q", auth)
	}
	if seen["query"] != "GDP Viet Nam 2025" || seen["max_results"] != float64(webSearchMax) || seen["topic"] != "news" ||
		seen["include_answer"] != true {
		t.Fatalf("request body = %v", seen)
	}
	if !strings.Contains(result.JSON, `"answer":"GDP 2025 tang 7%."`) || !strings.Contains(result.JSON, "https://gso.gov.vn/a") {
		t.Fatalf("result = %s", result.JSON)
	}
	if len(result.JSON) > 1000 {
		t.Fatalf("snippet must be trimmed, got %d bytes", len(result.JSON))
	}
}

func TestWebSearchExplainsBadKeyAndQuota(t *testing.T) {
	seen, auth := map[string]any{}, ""
	tool := tavilyServer(t, 401, `{"detail":{"error":"Unauthorized: missing or invalid API key."}}`, &seen, &auth)
	result := tool.Invoke(context.Background(), map[string]any{"query": "x"}, nil)
	if result.OK || !strings.Contains(result.JSON, "API key is invalid") || !strings.Contains(result.JSON, "Thiết lập") {
		t.Fatalf("bad key: %s", result.JSON)
	}
	tool = tavilyServer(t, 432, `{"detail":{"error":"plan limit"}}`, &seen, &auth)
	if result = tool.Invoke(context.Background(), map[string]any{"query": "x"}, nil); result.OK || !strings.Contains(result.JSON, "quota") {
		t.Fatalf("quota: %s", result.JSON)
	}
	if result = tool.Invoke(context.Background(), map[string]any{"query": "  "}, nil); result.OK || !strings.Contains(result.JSON, "missing 'query'") {
		t.Fatalf("empty query: %s", result.JSON)
	}
}
