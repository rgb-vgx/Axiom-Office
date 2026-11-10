package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResultStorePutGetAndCap(t *testing.T) {
	store := NewResultStore()
	id := store.Put("noi dung day du")
	if id == "" || store.Len() != 1 {
		t.Fatalf("Put: id=%q len=%d", id, store.Len())
	}
	if got, ok := store.Get(id); !ok || got != "noi dung day du" {
		t.Fatalf("Get: %q %v", got, ok)
	}
	if store.Put(strings.Repeat("x", maxStoredResultBytes+1)) != "" {
		t.Fatal("qua lon thi khong luu")
	}
	if store.Put("") != "" {
		t.Fatal("rong thi khong luu")
	}
	if _, ok := store.Get("khong-co"); ok {
		t.Fatal("id la khong tra ve")
	}
}

// Kết quả bị truncate được giữ nguyên văn + gắn resultId/readMore; kết quả không truncate thì giữ nguyên.
func TestAttachResultRef(t *testing.T) {
	run := &RunContext{Results: NewResultStore()}
	original := `{"ok":true,"result":{"truncated":true,"values":[1,2,3]}}`
	truncated := `{"ok":true,"result":{"truncated":true,"values":[1]}}`
	got := attachResultRef(truncated, original, run)
	var payload map[string]any
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatal(err)
	}
	result := payload["result"].(map[string]any)
	id, _ := result["resultId"].(string)
	if id == "" || !strings.Contains(result["readMore"].(string), "read_result") {
		t.Fatalf("thieu resultId/readMore: %s", got)
	}
	if stored, _ := run.Results.Get(id); stored != original {
		t.Fatalf("phai luu ban day du, duoc %q", stored)
	}

	plain := `{"ok":true,"result":{"values":[1]}}`
	if attachResultRef(plain, original, run) != plain || run.Results.Len() != 1 {
		t.Fatal("khong truncate thi khong luu")
	}
}

func TestReadResultPagination(t *testing.T) {
	run := &RunContext{Results: NewResultStore()}
	id := run.Results.Put("0123456789")
	tool := NewReadResultTool()
	first := tool.Invoke(nil, map[string]any{"id": id, "offset": float64(0), "limit": float64(4)}, run)
	var payload map[string]any
	if !first.OK || json.Unmarshal([]byte(first.JSON), &payload) != nil {
		t.Fatalf("doc: %s", first.JSON)
	}
	result := payload["result"].(map[string]any)
	if result["text"] != "0123" || result["nextOffset"] != float64(4) || result["truncated"] != true {
		t.Fatalf("trang 1: %+v", result)
	}
	last := tool.Invoke(nil, map[string]any{"id": id, "offset": float64(8), "limit": float64(10)}, run)
	payload = nil
	_ = json.Unmarshal([]byte(last.JSON), &payload)
	result = payload["result"].(map[string]any)
	if result["text"] != "89" || result["truncated"] != false {
		t.Fatalf("trang cuoi: %+v", result)
	}

	for _, bad := range []map[string]any{{}, {"id": "la"}, {"id": id, "offset": float64(99)}} {
		reply := tool.Invoke(nil, bad, run)
		if reply.OK && bad["id"] != id {
			t.Fatalf("id sai phai loi: %s", reply.JSON)
		}
	}
	if reply := tool.Invoke(nil, map[string]any{"id": id}, nil); reply.OK {
		t.Fatal("khong co store phai loi")
	}
}
