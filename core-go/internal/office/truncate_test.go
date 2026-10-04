package office

import (
	"encoding/json"
	"strings"
	"testing"
)

func rows(n, cols int) []any {
	out := make([]any, n)
	for r := range n {
		row := make([]any, cols)
		for c := range cols {
			row[c] = r*cols + c
		}
		out[r] = row
	}
	return out
}

func rawRead(t *testing.T, address string, n, cols int) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"ok": true, "result": map[string]any{"range": address, "values": rows(n, cols)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestParseCellVaColName(t *testing.T) {
	cases := []struct {
		ref      string
		col, row int
		ok       bool
	}{
		{"A1", 0, 1, true},
		{"$B$12", 1, 12, true},
		{"CV999", 99, 999, true},
		{"c3", 2, 3, true}, // chu thuong: truoc 04/10 tra false -> mat nextRange
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
	}{{0, "A"}, {25, "Z"}, {26, "AA"}, {99, "CV"}} {
		if got := colName(c.col); got != c.want {
			t.Errorf("colName(%d) = %q muon %q", c.col, got, c.want)
		}
	}
	if next := nextRange("a1:c5000", 200, 5000); next != "A201:C5000" {
		t.Errorf("nextRange chu thuong = %q", next)
	}
}

func TestApplyRawCatVaGanHint(t *testing.T) {
	limits := DefaultTruncation
	limits.Hint = "doc tiep bang nextRange"
	out := limits.ApplyRaw("et.readRange", rawRead(t, "A1:C5000", 5000, 3))
	var body map[string]any
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatalf("JSON hong: %v", err)
	}
	res := body["result"].(map[string]any)
	if res["truncated"] != true || res["hint"] != "doc tiep bang nextRange" || res["nextRange"] != "A201:C5000" {
		t.Fatalf("thieu truncated/hint/nextRange: %v", res)
	}
	if len(res["values"].([]any)) != 200 {
		t.Fatalf("values = %d", len(res["values"].([]any)))
	}
}

func TestApplyRawKhongCatThiNguyenVan(t *testing.T) {
	limits := DefaultTruncation
	limits.Hint = "x"
	small := rawRead(t, "A1:C10", 10, 3)
	if limits.ApplyRaw("et.readRange", small) != small {
		t.Fatal("vung nho phai nguyen van, khong gan hint")
	}
	for _, raw := range []string{`{"ok":false,"error":"no active spreadsheet"}`, `khong phai json`, `{"ok":true}`} {
		if limits.ApplyRaw("et.readRange", raw) != raw {
			t.Fatalf("phai nguyen van: %s", raw)
		}
	}
	big := rawRead(t, "A1:C5000", 5000, 3)
	if limits.ApplyRaw("et.writeRange", big) != big {
		t.Fatal("lenh khac khong duoc dong toi")
	}
}

func TestGioiHanTuyChinhNangMaxCells(t *testing.T) {
	// MCP max_cells=15000 voi 3 cot -> 5000 dong (khong bi chan o 200)
	limits := Truncation{HeadRows: 15000, CellBudget: 15000, TailRows: 10, IssuesCap: 100}
	out := limits.ApplyRaw("et.readRange", rawRead(t, "A1:C50001", 50001, 3))
	if !strings.Contains(out, `"nextRange":"A5001:C50001"`) {
		t.Fatalf("max_cells 15000 / 3 cot phai giu 5000 dong: %.300s", out[len(out)-300:])
	}
}
