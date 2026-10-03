package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

func matrix(rows, cols int) []any {
	out := make([]any, rows)
	for r := range rows {
		row := make([]any, cols)
		for c := range cols {
			row[c] = r*cols + c
		}
		out[r] = row
	}
	return out
}

func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body khong phai JSON: %v", err)
	}
	return body
}

func TestTruncateReadRangeNguyenVenNho(t *testing.T) {
	result := map[string]any{"range": "A1:C10", "values": matrix(10, 3)}
	if TruncateBridgeResult("et.readRange", result, "goc") != "goc" {
		t.Fatal("vung nho khong duoc cat")
	}
}

func TestTruncateReadRangeCatDau(t *testing.T) {
	result := map[string]any{"range": "A1:C5000", "values": matrix(5000, 3)}
	raw := TruncateBridgeResult("et.readRange", result, "")
	body := decodeBody(t, raw)
	if body["ok"] != true {
		t.Fatalf("mat ok: %v", body)
	}
	res := body["result"].(map[string]any)
	values := res["values"].([]any)
	if len(values) != ReadHeadRows {
		t.Fatalf("dau = %d, muon %d", len(values), ReadHeadRows)
	}
	if res["truncated"] != true || res["totalRows"] != float64(5000) || res["totalCols"] != float64(3) {
		t.Fatalf("thieu truncated/totalRows/totalCols: %v", res)
	}
	// dong 201..5000 con lai
	if res["nextRange"] != "A201:C5000" {
		t.Fatalf("nextRange = %v, muon A201:C5000", res["nextRange"])
	}
}

func TestTruncateReadRangeFormulasVaCellBudget(t *testing.T) {
	// 100 cot: budget 6000/100 = 60 dong dau thoi
	result := map[string]any{"range": "A1:CV5000", "formulas": matrix(5000, 100)}
	TruncateBridgeResult("et.readRange", result, "")
	if got := len(result["formulas"].([]any)); got != 60 {
		t.Fatalf("dau = %d, muon 60 (cell budget)", got)
	}
	if result["nextRange"] != "A61:CV5000" {
		t.Fatalf("nextRange = %v", result["nextRange"])
	}
}

func TestTruncateReadRangeDiaChiKhongDocDuoc(t *testing.T) {
	result := map[string]any{"range": "cai gi do", "values": matrix(1000, 2)}
	TruncateBridgeResult("et.readRange", result, "")
	if _, ok := result["nextRange"]; ok {
		t.Fatal("khong duoc nextRange khi dia chi la")
	}
	// luu y: map truc tiep giu int, chi JSON moi convert thanh float64
	if result["truncated"] != true || result["totalRows"] != 1000 {
		t.Fatal("van phai co truncated/totalRows")
	}
}

func TestTruncateCheckRangeGiuIssueCount(t *testing.T) {
	issues := make([]any, 250)
	for i := range issues {
		issues[i] = map[string]any{"kind": "x"}
	}
	result := map[string]any{"issueCount": 250, "issues": issues, "columns": []any{"a", "b"}}
	TruncateBridgeResult("et.checkRange", result, "")
	if len(result["issues"].([]any)) != CheckIssuesCap {
		t.Fatalf("issues = %d, muon %d", len(result["issues"].([]any)), CheckIssuesCap)
	}
	if result["issuesOmitted"] != 150 {
		t.Fatalf("issuesOmitted = %v", result["issuesOmitted"])
	}
	if result["issueCount"] != 250 {
		t.Fatal("issueCount PHAI giu nguyen 250 - do la so loi that")
	}
}

func TestTruncateCheckRangeNhoKhongCat(t *testing.T) {
	result := map[string]any{"issueCount": 2, "issues": []any{1, 2}}
	if TruncateBridgeResult("et.checkRange", result, "goc") != "goc" {
		t.Fatal("issue it khong duoc cat")
	}
}

func TestTruncateKhongDoiLenhKhac(t *testing.T) {
	result := map[string]any{"values": matrix(5000, 3)}
	if TruncateBridgeResult("et.writeRange", result, "goc") != "goc" {
		t.Fatal("lenh khac khong duoc dot")
	}
	if TruncateBridgeResult("et.readRange", nil, "goc") != "goc" {
		t.Fatal("result rong -> goc")
	}
}

func TestParseCellVaColName(t *testing.T) {
	cases := []struct {
		ref  string
		col  int
		row  int
		ok   bool
	}{
		{"A1", 0, 1, true},
		{"$B$12", 1, 12, true},
		{"CV999", 99, 999, true},
		{"A", 0, 0, false},
		{"1", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		col, row, ok := parseCell(c.ref)
		if col != c.col || row != c.row || ok != c.ok {
			t.Errorf("parseCell(%q) = %d,%d,%v muon %d,%d,%v", c.ref, col, row, ok, c.col, c.row, c.ok)
		}
	}
	for _, c := range []struct {
		col  int
		want string
	}{{0, "A"}, {1, "B"}, {2, "C"}, {25, "Z"}, {26, "AA"}, {27, "AB"}, {99, "CV"}} {
		if got := colName(c.col); got != c.want {
			t.Errorf("colName(%d) = %q muon %q", c.col, got, c.want)
		}
	}
}

func TestTruncateBodyLaJSONHopLe(t *testing.T) {
	result := map[string]any{"range": "A1:A3000", "values": matrix(3000, 1)}
	raw := TruncateBridgeResult("et.readRange", result, "")
	if !strings.Contains(raw, `"truncated":true`) {
		t.Fatalf("body thieu truncated: %s", raw)
	}
	decodeBody(t, raw) // JSON phai hop le
}

// Wiring: qua LAI Invoke that su (bridge gia) - ket qua tra cho model da bi cat chua.
func TestInvokeCatReadRangeQuaBridgeGia(t *testing.T) {
	body, err := json.Marshal(map[string]any{"ok": true, "result": map[string]any{
		"range": "A1:C5000", "values": matrix(5000, 3)}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	cat := &office.CommandCatalog{Version: "1", Commands: []office.Command{
		{Name: "et.readRange", Kind: "et", Agent: true, Summary: "doc vung"},
	}}
	tool := NewOfficeActionTool(cat, "et")
	run := &RunContext{Office: &office.Session{App: "et", Port: port},
		Bridge: office.NewBridgeClient(server.Client(), "")}

	result := tool.Invoke(context.Background(),
		map[string]any{"action": "et.readRange", "params": map[string]any{"range": "A1:C5000"}}, run)

	if !result.OK {
		t.Fatalf("lenh loi: %s", result.JSON)
	}
	for _, want := range []string{`"truncated":true`, `"totalRows":5000`, `"nextRange":"A201:C5000"`} {
		if !strings.Contains(result.JSON, want) {
			t.Fatalf("ket qua thieu %s: %.200s", want, result.JSON)
		}
	}
}

// Lenh loi (ok:false) van phai tra NGUYEN VAN - model phai doc duoc thong diep loi.
func TestInvokeLenhLoiKhongBiCat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error":"no active spreadsheet"}`))
	}))
	defer server.Close()
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	cat := &office.CommandCatalog{Version: "1", Commands: []office.Command{
		{Name: "et.readRange", Kind: "et", Agent: true, Summary: "doc vung"},
	}}
	tool := NewOfficeActionTool(cat, "et")
	run := &RunContext{Office: &office.Session{App: "et", Port: port},
		Bridge: office.NewBridgeClient(server.Client(), "")}

	result := tool.Invoke(context.Background(),
		map[string]any{"action": "et.readRange", "params": map[string]any{"range": "A1"}}, run)
	if result.OK || !strings.Contains(result.JSON, "no active spreadsheet") {
		t.Fatalf("loi phai tra nguyen van: %s", result.JSON)
	}
}
