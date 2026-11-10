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

// dashboardRun: bridge gia - so co sheet Data (Thang | Doanh thu | Chi phi | Ghi chu) va `sheets`; Dashboard co chu thi
// `dashboardUsed`.
func dashboardRun(t *testing.T, sheets []string, dashboardUsed bool) (*RunContext, *[]docCall, func()) {
	t.Helper()
	calls := []docCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call docCall
		_ = json.NewDecoder(r.Body).Decode(&call)
		calls = append(calls, call)
		reply := map[string]any{}
		switch call.Action {
		case "et.listSheets":
			reply = map[string]any{"activeSheet": "Data", "sheets": sheets}
		case "et.readRange":
			if call.Params["sheet"] == "Data" {
				reply = map[string]any{"values": []any{
					[]any{"Tháng", "Doanh thu", "Chi phí", "Ghi chú"},
					[]any{"T1", 120.0, 80.0, "ok"}, []any{"T2", 150.0, 90.0, nil}, []any{"T3", 130.0, 85.0, nil}}}
			} else if dashboardUsed && call.Params["range"] == "A1:P60" {
				reply = map[string]any{"values": []any{[]any{"cũ"}}}
			} else {
				reply = map[string]any{"values": []any{[]any{"Tổng doanh thu"}, []any{400.0}}}
			}
		case "et.addChart":
			reply = map[string]any{"chart": "Chart" + strconv.Itoa(len(calls))}
		}
		data, _ := json.Marshal(map[string]any{"ok": true, "result": reply})
		_, _ = w.Write(data)
	}))
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	run := &RunContext{Office: &office.Session{App: "et", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "t")}
	return run, &calls, server.Close
}

func callsOf(calls []docCall, action string) []docCall {
	found := []docCall{}
	for _, call := range calls {
		if call.Action == action {
			found = append(found, call)
		}
	}
	return found
}

func TestDashboardFromDataBuildsFormulasAndCharts(t *testing.T) {
	run, calls, done := dashboardRun(t, []string{"Data"}, false)
	defer done()
	result := NewDashboardFromDataTool().Invoke(context.Background(), map[string]any{
		"source": "Data!A1:D4", "title": "Doanh thu quý 1",
		"kpis": []any{
			map[string]any{"label": "Tổng doanh thu", "column": "doanh thu", "agg": "sum", "format": "vnd"},
			map[string]any{"column": "C", "agg": "average"},
		},
		"charts": []any{
			map[string]any{"type": "column", "title": "Doanh thu và chi phí", "x": "Tháng", "y": []any{"Doanh thu", "Chi phí"}},
			map[string]any{"type": "pie", "x": "Tháng", "y": []any{"Chi phí"}},
		},
	}, run)
	if !result.OK || !result.Mutating || !result.Verifies {
		t.Fatalf("phai dung xong: %s", result.JSON)
	}
	if len(callsOf(*calls, "et.addSheet")) != 1 || callsOf(*calls, "et.addSheet")[0].Params["name"] != "Dashboard" {
		t.Fatalf("tao sheet Dashboard: %v", *calls)
	}
	writes := callsOf(*calls, "et.writeRange")
	kpi := briefText(writes[1].Params["values"])
	if writes[1].Params["range"] != "A3" || !strings.Contains(kpi, "=SUM(Data!$B$2:$B$4)") || !strings.Contains(kpi, "=AVERAGE(Data!$C$2:$C$4)") ||
		!strings.Contains(kpi, `"Chi phí"`) {
		t.Fatalf("o KPI la cong thuc tro sheet du lieu, nhan mac dinh = tieu de cot: %v", writes[1].Params)
	}
	charts := callsOf(*calls, "et.addChart")
	if len(charts) != 2 || charts[0].Params["range"] != "Data!A1:A4,B1:B4,C1:C4" || charts[1].Params["range"] != "Data!A1:A4,C1:C4" {
		t.Fatalf("bieu do doc thang vung Data (cot x + cot y): %v", charts)
	}
	if charts[0].Params["anchor"] != "A6" || charts[1].Params["anchor"] == "A6" {
		t.Fatalf("hai bieu do canh nhau duoi o KPI: %v / %v", charts[0].Params["anchor"], charts[1].Params["anchor"])
	}
	formats := briefText(callsOf(*calls, "et.formatRange"))
	if !strings.Contains(formats, `#,##0 \"đ\"`) || !strings.Contains(formats, `"columnWidth":24`) {
		t.Fatalf("dinh dang so theo format + noi cot KPI: %s", formats)
	}
}

func TestDashboardFromDataRefusals(t *testing.T) {
	run, calls, done := dashboardRun(t, []string{"Data", "Dashboard"}, true)
	defer done()
	result := NewDashboardFromDataTool().Invoke(context.Background(), map[string]any{"source": "Data!A1:D4",
		"kpis": []any{map[string]any{"column": "Doanh thu"}}}, run)
	if result.OK || result.Mutating || !strings.Contains(result.JSON, "already has content") {
		t.Fatalf("sheet Dashboard da co chu: tu choi: %s", result.JSON)
	}
	for _, call := range *calls {
		if call.Action == "et.writeRange" || call.Action == "et.addSheet" {
			t.Fatalf("khong duoc ghi: %v", *calls)
		}
	}
	run, _, done2 := dashboardRun(t, []string{"Data"}, false)
	defer done2()
	result = NewDashboardFromDataTool().Invoke(context.Background(), map[string]any{"source": "Data!A1:D4",
		"kpis":   []any{map[string]any{"column": "Lợi nhuận"}, map[string]any{"column": "Ghi chú", "agg": "sum"}},
		"charts": []any{map[string]any{"type": "pie", "x": "Tháng", "y": []any{"Doanh thu", "Chi phí"}}}}, run)
	for _, part := range []string{"not a column", "has no numbers", "pie: exactly 1"} {
		if result.OK || !strings.Contains(result.JSON, part) {
			t.Fatalf("thieu loi %q: %s", part, result.JSON)
		}
	}
	result = NewDashboardFromDataTool().Invoke(context.Background(), map[string]any{"source": "Data!A1:D4", "sheet": "data",
		"kpis": []any{map[string]any{"column": "Doanh thu"}}}, run)
	if result.OK || !strings.Contains(result.JSON, "must not be the data sheet") {
		t.Fatalf("dashboard trung sheet du lieu: %s", result.JSON)
	}
}

func TestColumnLettersRoundTrip(t *testing.T) {
	for _, n := range []int{1, 26, 27, 52, 703} {
		if columnNumber(columnLetters(n)) != n {
			t.Fatalf("cot %d -> %s", n, columnLetters(n))
		}
	}
	if quoteSheet("Kế hoạch") != "'Kế hoạch'" || quoteSheet("Data") != "Data" {
		t.Fatal("ten sheet co dau / khoang trang phai nam trong nhay don")
	}
}

// Dung lai dashboard do chinh luot tao (replace: true): xoa sheet roi dung lai; sheet khong do luot tao thi tu choi.
func TestDashboardFromDataReplace(t *testing.T) {
	run, calls, done := dashboardRun(t, []string{"Data", "Dashboard"}, true)
	defer done()
	spec := map[string]any{"source": "Data!A1:D4", "kpis": []any{map[string]any{"column": "Doanh thu"}}}
	result := NewDashboardFromDataTool().Invoke(context.Background(), spec, run)
	if result.OK || !strings.Contains(result.JSON, "already has content") {
		t.Fatalf("sheet khong do luot tao: tu choi: %s", result.JSON)
	}
	run.CreatedSheets = map[string]bool{"Dashboard": true}
	result = NewDashboardFromDataTool().Invoke(context.Background(), spec, run)
	if result.OK || !strings.Contains(result.JSON, "replace: true") {
		t.Fatalf("sheet do luot tao, chua replace: chi cach dung lai: %s", result.JSON)
	}
	spec["replace"] = true
	result = NewDashboardFromDataTool().Invoke(context.Background(), spec, run)
	deleted := callsOf(*calls, "et.deleteSheet")
	if !result.OK || len(deleted) != 1 || deleted[0].Params["sheet"] != "Dashboard" || len(callsOf(*calls, "et.addSheet")) != 1 {
		t.Fatalf("replace: xoa roi dung lai: %s %v", result.JSON, *calls)
	}
	result = NewDashboardFromDataTool().Invoke(context.Background(), map[string]any{"source": "Data!A1:D4",
		"kpis": []any{map[string]any{"agg": "sum"}}}, run)
	if result.OK || !strings.Contains(result.JSON, "needs column") {
		t.Fatalf("KPI thieu column: chi cach dien: %s", result.JSON)
	}
}
