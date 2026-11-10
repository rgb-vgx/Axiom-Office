package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

// recordingBridge: bridge gia ghi lai params nhan duoc (de kiem params da duoc sua truoc khi gui).
func recordingBridge(t *testing.T) (*RunContext, *[]map[string]any) {
	t.Helper()
	got := &[]map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(body, &decoded)
		params, _ := decoded["params"].(map[string]any)
		*got = append(*got, params)
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	t.Cleanup(server.Close)
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	return &RunContext{Office: &office.Session{App: "et", Port: port},
		Bridge: office.NewBridgeClient(server.Client(), "")}, got
}

func paramsCatalog() *office.CommandCatalog {
	return &office.CommandCatalog{Version: "1", Commands: []office.Command{
		{Name: "et.formatRange", Kind: "et", Agent: true,
			Params: []office.CommandParam{{Name: "range", Required: true}, {Name: "numFmt"}, {Name: "sheet"}}},
		{Name: "et.addSheet", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "name"}}},
		{Name: "et.readRange", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "range", Required: true}, {Name: "sheet"}}},
	}}
}

// Do duoc 03/10: et.formatRange {cell_range, numFmt, sheet} -> "'range' is required" x10.
func TestOfficeActionAliasCellRange(t *testing.T) {
	run, got := recordingBridge(t)
	tool := NewOfficeActionTool(paramsCatalog(), "et")
	result := tool.Invoke(context.Background(), map[string]any{"action": "et.formatRange",
		"params": map[string]any{"cell_range": "D2:E5001", "numFmt": "dd/mm/yyyy", "sheet": "Tasks"}}, run)
	if !result.OK || len(*got) != 1 {
		t.Fatalf("phai goi bridge thanh cong: %+v", result)
	}
	sent := (*got)[0]
	if sent["range"] != "D2:E5001" {
		t.Fatalf("cell_range phai thanh range: %v", sent)
	}
	if _, left := sent["cell_range"]; left {
		t.Fatalf("khong duoc gui ca cell_range: %v", sent)
	}
}

func TestOfficeActionAliasKhongDeTenDung(t *testing.T) {
	run, got := recordingBridge(t)
	tool := NewOfficeActionTool(paramsCatalog(), "et")
	tool.Invoke(context.Background(), map[string]any{"action": "et.formatRange",
		"params": map[string]any{"range": "A1", "cell_range": "B2"}}, run)
	if (*got)[0]["range"] != "A1" {
		t.Fatalf("da co range thi giu nguyen: %v", (*got)[0])
	}
	// lenh khong co tham so range thi khong dung cham
	tool.Invoke(context.Background(), map[string]any{"action": "et.addSheet",
		"params": map[string]any{"cell_range": "X"}}, run)
	if _, added := (*got)[1]["range"]; added {
		t.Fatalf("et.addSheet khong co range - khong duoc them: %v", (*got)[1])
	}
}

// Do duoc 03/10: params gui dang CHUOI JSON hong -> nil -> "'range' is required" du model DA gui range.
func TestOfficeActionParamsChuoiHongBaoDungLoi(t *testing.T) {
	run, got := recordingBridge(t)
	tool := NewOfficeActionTool(paramsCatalog(), "et")
	result := tool.Invoke(context.Background(), map[string]any{"action": "et.formatRange",
		"params": `{"range": "A1", "numFmt": "0.0", "bad": "line` + "\n" + `break"}`}, run)
	if result.OK {
		t.Fatal("chuoi JSON hong phai loi")
	}
	if !strings.Contains(result.JSON, "not valid JSON") || strings.Contains(result.JSON, "is required") {
		t.Fatalf("loi phai noi params hong, khong noi thieu range: %s", result.JSON)
	}
	if len(*got) != 0 {
		t.Fatal("params hong thi khong duoc goi bridge")
	}
}

func TestOfficeActionParamsChuoiCoRacPhiaSau(t *testing.T) {
	run, got := recordingBridge(t)
	tool := NewOfficeActionTool(paramsCatalog(), "et")
	result := tool.Invoke(context.Background(), map[string]any{"action": "et.formatRange",
		"params": `{"range": "A1:B2", "numFmt": "0.0"}}`}, run)
	if !result.OK || (*got)[0]["range"] != "A1:B2" {
		t.Fatalf("object dau tien phai duoc dung, rac phia sau bo qua: %+v %v", result, *got)
	}
}

func TestOfficeActionParamsChuoiHopLeVaNull(t *testing.T) {
	run, got := recordingBridge(t)
	tool := NewOfficeActionTool(paramsCatalog(), "et")
	if r := tool.Invoke(context.Background(), map[string]any{"action": "et.formatRange",
		"params": `{"range": "C3"}`}, run); !r.OK || (*got)[0]["range"] != "C3" {
		t.Fatalf("chuoi JSON hop le van phai chay: %+v", r)
	}
	if r := tool.Invoke(context.Background(), map[string]any{"action": "et.addSheet",
		"params": "null"}, run); !r.OK {
		t.Fatalf(`"null" = khong tham so, khong phai loi: %+v`, r)
	}
}

// Bo do 27/10/2026 ca B: 19 lan et.formatRange tung vung -> tu lan thu 3 ket qua nhac gom vao et.formatRanges.
func TestFormatRangeHintsBatchFromThirdCall(t *testing.T) {
	run, _ := recordingBridge(t)
	tool := NewOfficeActionTool(paramsCatalog(), "et")
	for call := 1; call <= 3; call++ {
		result := tool.Invoke(context.Background(), map[string]any{"action": "et.formatRange",
			"params": map[string]any{"range": "A" + strconv.Itoa(call), "numFmt": "0%"}}, run)
		hinted := strings.Contains(result.JSON, "et.formatRanges")
		if !result.OK || hinted != (call == 3) {
			t.Fatalf("lan %d: goi y chi tu lan thu 3: %s", call, result.JSON)
		}
	}
}

func TestReadRangeHintsBatchFromFourthCall(t *testing.T) {
	run, _ := recordingBridge(t)
	tool := NewOfficeActionTool(paramsCatalog(), "et")
	for call := 1; call <= 4; call++ {
		result := tool.Invoke(context.Background(), map[string]any{"action": "et.readRange",
			"params": map[string]any{"range": "A" + strconv.Itoa(call)}}, run)
		if hinted := strings.Contains(result.JSON, "et.readRanges"); !result.OK || hinted != (call == 4) {
			t.Fatalf("lan %d: goi y readRanges chi tu lan thu 4: %s", call, result.JSON)
		}
	}
}
