//go:build live

package tools

// Smoke test Buoc 2 voi bridge LibreOffice THAT (shape du lieu that cua extension, khong phai ma tran
// gia trong truncate_test.go). Chi chay khi co ca build tag lan bien moi truong:
//
//	AXIOM_LIVE_PORT=47852 AXIOM_LIVE_TOKEN=<token> go test -tags live -run Live -v ./internal/tools/
//
// Can mot so tinh DANG MO va TRONG (test ghi de vao Sheet dang active). Danh sach muc kiem theo tu van
// ChatGPT 04/10 - xem C:\Users\ThuyetMT\test\bench\ab\README.md, muc "Smoke test Buoc 2".

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"axiomoffice/core/internal/office"
)

type liveEnv struct {
	tool *OfficeActionTool
	run  *RunContext
}

func newLive(t *testing.T) liveEnv {
	t.Helper()
	port, err := strconv.Atoi(os.Getenv("AXIOM_LIVE_PORT"))
	if err != nil || port == 0 {
		t.Skip("AXIOM_LIVE_PORT chua dat")
	}
	bridge := office.NewBridgeClient(&http.Client{Timeout: 5 * time.Minute}, os.Getenv("AXIOM_LIVE_TOKEN"))
	catalog := bridge.GetCommands(context.Background(), port, true)
	if catalog == nil || len(catalog.Commands) == 0 {
		t.Fatal("khong lay duoc danh sach lenh tu bridge")
	}
	return liveEnv{
		tool: NewOfficeActionTool(catalog, "et"),
		run:  &RunContext{Office: &office.Session{App: "et", Port: port}, Bridge: bridge},
	}
}

// do: goi qua Invoke (dung duong cua agent: allowlist -> bridge -> cat) va giai ma body.
func (l liveEnv) do(t *testing.T, action string, params map[string]any) (Result, map[string]any) {
	t.Helper()
	result := l.tool.Invoke(context.Background(), map[string]any{"action": action, "params": params}, l.run)
	var body map[string]any
	if err := json.Unmarshal([]byte(result.JSON), &body); err != nil {
		t.Fatalf("%s tra JSON hong: %v (%.200s)", action, err, result.JSON)
	}
	return result, body
}

func (l liveEnv) mustOK(t *testing.T, action string, params map[string]any) map[string]any {
	t.Helper()
	result, body := l.do(t, action, params)
	if !result.OK {
		t.Fatalf("%s loi: %.300s", action, result.JSON)
	}
	res, _ := body["result"].(map[string]any)
	return res
}

func TestLiveTruncation(t *testing.T) {
	l := newLive(t)
	// Du lieu tat dinh: A..C = so dong (de doi chieu dau/duoi), 50.001 dong - dung kich co bai Test 2.
	l.mustOK(t, "et.fillRange", map[string]any{"range": "A1:C50001", "formula": "=ROW()"})

	t.Run("1-vung-nho-nguyen-ven", func(t *testing.T) {
		res := l.mustOK(t, "et.readRange", map[string]any{"range": "A1:C150"})
		if _, cut := res["truncated"]; cut {
			t.Fatal("150 dong khong duoc cat")
		}
		if n := len(res["values"].([]any)); n != 150 {
			t.Fatalf("values = %d dong, muon 150", n)
		}
	})

	t.Run("2-201-dong", func(t *testing.T) {
		res := l.mustOK(t, "et.readRange", map[string]any{"range": "A1:C201"})
		if res["truncated"] != true || res["totalRows"] != float64(201) {
			t.Fatalf("truncated/totalRows sai: %v %v", res["truncated"], res["totalRows"])
		}
		if n := len(res["values"].([]any)); n != ReadHeadRows {
			t.Fatalf("values = %d, muon %d", n, ReadHeadRows)
		}
		if res["nextRange"] != "A201:C201" {
			t.Fatalf("nextRange = %v, muon A201:C201", res["nextRange"])
		}
	})

	t.Run("3-50001-dong-payload-bi-chan", func(t *testing.T) {
		result, body := l.do(t, "et.readRange", map[string]any{"range": "A1:C50001"})
		if len(result.JSON) > 40_000 {
			t.Fatalf("payload vao history = %d bytes, phai < 40KB", len(result.JSON))
		}
		res := body["result"].(map[string]any)
		tail := res["tailValues"].([]any)
		last := tail[len(tail)-1].([]any)
		// =ROW() o dong cuoi phai la 50001: tail lay dung DUOI cua vung, khong phai dau
		if last[0] != float64(50001) {
			t.Fatalf("dong cuoi cua tail = %v, muon 50001", last[0])
		}
		if res["nextRange"] != "A201:C50001" {
			t.Fatalf("nextRange = %v", res["nextRange"])
		}
		t.Logf("50.001 dong -> %d bytes trong history", len(result.JSON))
	})

	t.Run("4-100-cot-cell-budget", func(t *testing.T) {
		l.mustOK(t, "et.fillRange", map[string]any{"range": "E1:CZ201", "formula": "=ROW()"})
		res := l.mustOK(t, "et.readRange", map[string]any{"range": "E1:CZ201"})
		if n := len(res["values"].([]any)); n != ReadCellBudget/100 {
			t.Fatalf("values = %d dong, muon %d (6000/100)", n, ReadCellBudget/100)
		}
	})

	t.Run("5-range-giua-sheet", func(t *testing.T) {
		res := l.mustOK(t, "et.readRange", map[string]any{"range": "A1000:C5000"})
		if res["nextRange"] != "A1200:C5000" {
			t.Fatalf("nextRange = %v, muon A1200:C5000 (tinh tu dong 1000, khong tu dong 1)", res["nextRange"])
		}
		first := res["values"].([]any)[0].([]any)
		if first[0] != float64(1000) {
			t.Fatalf("dong dau = %v, muon 1000", first[0])
		}
	})

	t.Run("6-tail-cua-range-khong-phai-cua-sheet", func(t *testing.T) {
		res := l.mustOK(t, "et.readRange", map[string]any{"range": "A1000:C5000"})
		tail := res["tailValues"].([]any)
		if len(tail) != TailRows {
			t.Fatalf("tail = %d dong", len(tail))
		}
		if tail[0].([]any)[0] != float64(4991) || tail[TailRows-1].([]any)[0] != float64(5000) {
			t.Fatalf("tail = %v..%v, muon 4991..5000", tail[0], tail[TailRows-1])
		}
	})

	t.Run("6b-formulas-co-tailFormulas", func(t *testing.T) {
		res := l.mustOK(t, "et.readRange", map[string]any{"range": "A1:C3000", "formulas": true})
		if _, ok := res["tailFormulas"]; !ok || res["truncated"] != true {
			t.Fatal("formulas=true phai cat va co tailFormulas")
		}
	})

	t.Run("7-checkRange-tren-100-issue-giu-issueCount", func(t *testing.T) {
		// Sheet moi: 130 cot khong tieu de + loi #DIV/0! -> moi cot >= 2 issue (empty-header, error-values)
		l.mustOK(t, "et.addSheet", map[string]any{"name": "SmokeCheck"})
		l.mustOK(t, "et.fillRange", map[string]any{"range": "A2:DZ3", "formula": "=1/0", "sheet": "SmokeCheck"})
		res := l.mustOK(t, "et.checkRange", map[string]any{"range": "A1:DZ3", "sheet": "SmokeCheck"})
		count, _ := res["issueCount"].(float64)
		issues := res["issues"].([]any)
		if count <= CheckIssuesCap {
			t.Fatalf("issueCount = %v - du lieu thu chua du > %d issue", count, CheckIssuesCap)
		}
		if len(issues) != CheckIssuesCap {
			t.Fatalf("issues = %d, muon %d", len(issues), CheckIssuesCap)
		}
		if omitted, _ := res["issuesOmitted"].(float64); int(omitted) != int(count)-CheckIssuesCap {
			t.Fatalf("issuesOmitted = %v, muon %v", omitted, int(count)-CheckIssuesCap)
		}
		t.Logf("issueCount=%v giu nguyen, issues cat con %d", count, len(issues))
	})

	t.Run("7b-checkRange-it-issue-khong-cat", func(t *testing.T) {
		res := l.mustOK(t, "et.checkRange", map[string]any{"range": "A1:C150"})
		if _, cut := res["issuesOmitted"]; cut {
			t.Fatal("it issue khong duoc them issuesOmitted")
		}
	})

	t.Run("8-loi-tra-nguyen-van", func(t *testing.T) {
		result, body := l.do(t, "et.readRange", map[string]any{"range": "A1:B2", "sheet": "KhongTonTai_xyz"})
		if result.OK || body["ok"] != false {
			t.Fatalf("sheet khong ton tai phai loi: %.200s", result.JSON)
		}
		if _, ok := body["error"].(string); !ok {
			t.Fatalf("mat thong diep loi: %.200s", result.JSON)
		}
	})
}
