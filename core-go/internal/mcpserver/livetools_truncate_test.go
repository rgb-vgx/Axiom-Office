package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// fakeReadBridge: bridge gia tra et.readRange n dong x 3 cot cho moi lenh doc, {"ok":true} cho lenh khac.
func fakeReadBridge(t *testing.T, n int) int {
	t.Helper()
	matrix := make([][]int, n)
	for r := range matrix {
		matrix[r] = []int{r + 1, r + 1, r + 1}
	}
	readBody, _ := json.Marshal(map[string]any{"ok": true, "result": map[string]any{
		"range": "A1:C" + strconv.Itoa(n), "values": matrix}})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["action"] == "et.readRange" {
			_, _ = w.Write(readBody)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"done":true}}`))
	}))
	t.Cleanup(server.Close)
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func liveTool(t *testing.T, name string) *Tool {
	t.Helper()
	for _, tool := range excelLiveTools() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("khong co tool %s", name)
	return nil
}

func decodeResult(t *testing.T, value any) map[string]any {
	t.Helper()
	text, ok := value.(string)
	if !ok {
		t.Fatalf("ket qua khong phai chuoi: %T", value)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("JSON hong: %v (%.200s)", err, text)
	}
	result, _ := body["result"].(map[string]any)
	return result
}

// wps_live_command voi et.readRange: client ngoai nhan ban da cat + hint, khong phai 1 MB.
func TestLiveCommandCatReadRange(t *testing.T) {
	port := fakeReadBridge(t, 50001)
	value, err := liveTool(t, "wps_live_command").Handler(NewToolArgs(map[string]any{
		"app": "et", "action": "et.readRange", "params": map[string]any{"range": "A1:C50001"}, "port": float64(port)}))
	if err != nil {
		t.Fatal(err)
	}
	if text := value.(string); len(text) > 40_000 {
		t.Fatalf("ket qua MCP = %d byte, phai bi chan", len(text))
	}
	res := decodeResult(t, value)
	if res["truncated"] != true || res["nextRange"] != "A201:C50001" || res["totalRows"] != float64(50001) {
		t.Fatalf("thieu truncated/nextRange/totalRows: %v", res)
	}
	if hint, _ := res["hint"].(string); !strings.Contains(hint, "nextRange") || strings.Contains(hint, "csv") {
		t.Fatalf("hint phai chi duong doc tiep, khong goi y saveAs csv: %q", hint)
	}
}

// Lenh khac di qua wps_live_command: nguyen van.
func TestLiveCommandLenhKhacNguyenVan(t *testing.T) {
	port := fakeReadBridge(t, 10)
	value, _ := liveTool(t, "wps_live_command").Handler(NewToolArgs(map[string]any{
		"app": "et", "action": "et.writeRange", "params": map[string]any{"range": "A1"}, "port": float64(port)}))
	if value.(string) != `{"ok":true,"result":{"done":true}}` {
		t.Fatalf("lenh ghi phai nguyen van: %v", value)
	}
}

// max_cells nang gioi han (co tran cung 20000).
func TestReadTruncationMaxCells(t *testing.T) {
	port := fakeReadBridge(t, 50001)
	cases := []struct {
		maxCells any
		next     string
	}{
		{nil, "A201:C50001"},              // mac dinh 200 dong
		{float64(15000), "A5001:C50001"},  // 15000 o / 3 cot = 5000 dong
		{float64(999999), "A6667:C50001"}, // chan o 20000 o -> 6666 dong
	}
	for _, c := range cases {
		args := map[string]any{"cell_range": "A1:C50001"}
		if c.maxCells != nil {
			args["max_cells"] = c.maxCells
		}
		a := NewToolArgs(args)
		value, err := liveLimited(a, "et", "et.readRange", map[string]any{"range": "A1:C50001"}, port, true, readTruncation(a))
		if err != nil {
			t.Fatal(err)
		}
		if res := decodeResult(t, value); res["nextRange"] != c.next {
			t.Errorf("max_cells=%v: nextRange = %v, muon %s", c.maxCells, res["nextRange"], c.next)
		}
	}
}

func TestReadRangeSchemaCoMaxCells(t *testing.T) {
	tool := liveTool(t, "wps_live_read_range")
	raw, err := json.Marshal(tool.InputSchema())
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.Properties["max_cells"]; !ok {
		t.Fatalf("schema thieu max_cells: %s", raw)
	}
	for _, name := range schema.Required {
		if name == "max_cells" {
			t.Fatal("max_cells phai tuy chon - client cu khong gui van chay nhu truoc")
		}
	}
	if !strings.Contains(tool.Description, "truncated") {
		t.Fatal("mo ta phai noi ve truncated")
	}
}
