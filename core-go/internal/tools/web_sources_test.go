package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"axiomoffice/core/internal/store"
)

func testWebSourcesTool(t *testing.T) (*WebSourcesTool, *store.WebSources) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sources := db.WebSources()
	return NewWebSourcesTool(sources), sources
}

func invokeWebSources(t *testing.T, tool *WebSourcesTool, run *RunContext, arguments map[string]any) map[string]any {
	t.Helper()
	result := tool.Invoke(context.Background(), arguments, run)
	if !result.OK {
		t.Fatalf("Invoke(%v) loi: %s", arguments, result.JSON)
	}
	return resultMap(t, result.JSON)
}

func resultMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	var payload struct {
		OK     bool           `json:"ok"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("ket qua khong phai JSON: %s", raw)
	}
	return payload.Result
}

func TestWebSourcesToolSearchGetList(t *testing.T) {
	tool, sources := testWebSourcesTool(t)
	sources.SaveDocs("r_1", "c_1", "research", "visa", []store.WebSourceDoc{
		{URL: "https://a.example/visa-uc", Title: "Visa nông nghiệp Úc", Lang: "vi", Mode: "full", OK: true,
			Content: "Úc mở rộng diện tích visa nông nghiệp cho Việt Nam từ tháng 10/2026. " + strings.Repeat("chi tiet ", 50)},
	})
	run := &RunContext{RunID: "r_2", ConversationID: "c_1"}

	// search bo dau
	result := invokeWebSources(t, tool, run, map[string]any{"action": "search", "query": "visa nong nghiep uc"})
	items, _ := result["sources"].([]any)
	if len(items) != 1 {
		t.Fatalf("search tra %d muc, want 1: %v", len(items), result)
	}
	hit := items[0].(map[string]any)
	if hit["url"] != "https://a.example/visa-uc" {
		t.Fatalf("hit = %v", hit)
	}

	// get theo id
	result = invokeWebSources(t, tool, run, map[string]any{"action": "get", "id": float64(hit["id"].(float64)), "max_chars": float64(1000)})
	if !strings.Contains(result["content"].(string), "nông nghiệp") {
		t.Fatalf("get thieu noi dung: %v", result["content"])
	}

	// get theo url
	result = invokeWebSources(t, tool, run, map[string]any{"action": "get", "url": "https://a.example/visa-uc"})
	if result["id"].(float64) != hit["id"].(float64) {
		t.Fatalf("get theo url sai nguon: %v", result)
	}

	// list theo hoi thoai
	result = invokeWebSources(t, tool, run, map[string]any{"action": "list"})
	items, _ = result["sources"].([]any)
	if len(items) != 1 {
		t.Fatalf("list tra %d muc, want 1", len(items))
	}
}

func TestWebSourcesToolEmptyAndMissing(t *testing.T) {
	tool, _ := testWebSourcesTool(t)
	result := invokeWebSources(t, tool, nil, map[string]any{"action": "search", "query": "gi do"})
	if result["count"].(float64) != 0 {
		t.Fatalf("kho rong phai tra 0: %v", result)
	}
	if result := tool.Invoke(context.Background(), map[string]any{"action": "get", "id": float64(99)}, nil); result.OK {
		t.Fatal("get nguon khong ton tai phai loi")
	}
	if result := tool.Invoke(context.Background(), map[string]any{"action": "hom qua"}, nil); result.OK {
		t.Fatal("action la phai loi")
	}
}

func TestWebSourcesToolNilStore(t *testing.T) {
	tool := NewWebSourcesTool(nil)
	if result := tool.Invoke(context.Background(), map[string]any{"action": "list"}, nil); result.OK {
		t.Fatal("kho khong mo duoc phai bao loi ro rang")
	}
}
