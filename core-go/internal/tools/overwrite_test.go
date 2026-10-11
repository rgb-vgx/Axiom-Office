package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"axiomoffice/core/internal/office"
)

// Spec (log 11/10/2026): agent ghi de 25 o du lieu goc tren sheet ADT (so/ngay luu dang chu -> so) ma khong
// hoi. Policy: lenh ghi o (et.writeRange, et.writeRanges, et.fillRange) vao sheet CO TU TRUOC luot chay ma
// dung vao o KHONG RONG va gia tri moi KHAC gia tri cu -> hoi nguoi dung truoc khi ghi. Ly do hoi neu ten
// sheet, dia chi o va gia tri cu. Dong y mot lan thi cac lenh sau tren CUNG sheet trong cung luot khong hoi
// lai. Sheet do chinh luot tao (et.addSheet) thi khong hoi. Khong doc duoc o (bridge loi) thi khong chan.

type fakeBook struct {
	mu     sync.Mutex
	active string
	sheets map[string]map[string]any // sheet -> "A1" -> gia tri
	writes []string                  // action da toi bridge (tru readRange)
	reads  int
}

func cellName(col, row int) string {
	letters := ""
	for col++; col > 0; col = (col - 1) / 26 {
		letters = string(rune('A'+(col-1)%26)) + letters
	}
	return letters + strconv.Itoa(row+1)
}

func parseCell(address string) (int, int) {
	address = strings.ReplaceAll(strings.ToUpper(address), "$", "")
	col, index := 0, 0
	for index < len(address) && address[index] >= 'A' && address[index] <= 'Z' {
		col = col*26 + int(address[index]-'A') + 1
		index++
	}
	row, _ := strconv.Atoi(address[index:])
	return col - 1, row - 1
}

func (b *fakeBook) serve(t *testing.T) (*httptest.Server, int) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		b.mu.Lock()
		defer b.mu.Unlock()
		sheet, _ := body.Params["sheet"].(string)
		if sheet == "" {
			sheet = b.active
		}
		switch body.Action {
		case "et.readRange":
			b.reads++
			cells, ok := b.sheets[sheet]
			if !ok {
				_, _ = w.Write([]byte(`{"ok":false,"error":"no sheet ` + sheet + `"}`))
				return
			}
			address, _ := body.Params["range"].(string)
			parts := strings.Split(address, ":")
			c1, r1 := parseCell(parts[0])
			c2, r2 := c1, r1
			if len(parts) == 2 {
				c2, r2 = parseCell(parts[1])
			}
			values := [][]any{}
			for row := r1; row <= r2; row++ {
				line := []any{}
				for col := c1; col <= c2; col++ {
					line = append(line, cells[cellName(col, row)])
				}
				values = append(values, line)
			}
			data, _ := json.Marshal(map[string]any{"ok": true, "result": map[string]any{"sheet": sheet, "range": address, "values": values}})
			_, _ = w.Write(data)
		case "et.addSheet":
			name, _ := body.Params["name"].(string)
			b.sheets[name] = map[string]any{}
			b.writes = append(b.writes, body.Action)
			_, _ = w.Write([]byte(`{"ok":true,"result":{"added":true}}`))
		default:
			b.writes = append(b.writes, body.Action)
			_, _ = w.Write([]byte(`{"ok":true,"result":{"written":1}}`))
		}
	}))
	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	return server, port
}

func newBook() *fakeBook {
	return &fakeBook{active: "ADT", sheets: map[string]map[string]any{
		"ADT": {"A1": "STT", "Q4": "15.553,8", "I4": "64,78", "C29": "26/03/2026", "S3": 5368534.0},
	}}
}

func calcCatalog() *office.CommandCatalog {
	return &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "et.readRange", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "range", Required: true}, {Name: "sheet"}}},
		{Name: "et.writeRange", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "range", Required: true}, {Name: "values", Required: true}, {Name: "sheet"}}},
		{Name: "et.writeRanges", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "writes", Required: true}, {Name: "sheet"}}},
		{Name: "et.fillRange", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "range", Required: true}, {Name: "formula"}, {Name: "sheet"}}},
		{Name: "et.addSheet", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "name", Required: true}}},
	}}
}

type asked struct{ action, reason string }

func overwriteRun(port int, server *httptest.Server, answer *bool, log *[]asked) *RunContext {
	run := &RunContext{
		Office: &office.Session{App: "et", Port: port},
		Bridge: office.NewBridgeClient(server.Client(), "t"),
		Prompt: "Tao dashboard cho sheet ADT",
	}
	if answer != nil {
		run.Confirm = func(action, reason, preview string) (bool, error) {
			*log = append(*log, asked{action, reason})
			return *answer, nil
		}
	}
	return run
}

func invoke(tool *OfficeActionTool, run *RunContext, action string, params map[string]any) Result {
	return tool.Invoke(context.Background(), map[string]any{"action": action, "params": params}, run)
}

func TestOverwriteNonEmptyCellsAsksAndDeclineWritesNothing(t *testing.T) {
	book := newBook()
	server, port := book.serve(t)
	defer server.Close()
	no := false
	var log []asked
	run := overwriteRun(port, server, &no, &log)
	tool := NewOfficeActionTool(calcCatalog(), "et")

	result := invoke(tool, run, "et.writeRange", map[string]any{"range": "Q4", "sheet": "ADT", "values": []any{[]any{15553.8}}})
	if len(log) != 1 {
		t.Fatalf("ghi de o co du lieu phai hoi dung 1 lan, hoi %d lan", len(log))
	}
	for _, want := range []string{"ADT", "Q4", "15.553,8"} {
		if !strings.Contains(log[0].reason, want) {
			t.Errorf("ly do hoi phai neu %q: %q", want, log[0].reason)
		}
	}
	if result.OK || !strings.Contains(result.JSON, "user declined") {
		t.Fatalf("tu choi -> ket qua 'user declined', duoc %+v", result)
	}
	if len(book.writes) != 0 {
		t.Fatalf("tu choi thi khong duoc ghi: %v", book.writes)
	}
}

func TestApprovalIsRememberedPerSheetForTheRun(t *testing.T) {
	book := newBook()
	server, port := book.serve(t)
	defer server.Close()
	yes := true
	var log []asked
	run := overwriteRun(port, server, &yes, &log)
	tool := NewOfficeActionTool(calcCatalog(), "et")

	if result := invoke(tool, run, "et.writeRange", map[string]any{"range": "I4", "sheet": "ADT", "values": []any{[]any{64.78}}}); !result.OK {
		t.Fatalf("dong y -> phai ghi: %+v", result)
	}
	invoke(tool, run, "et.writeRange", map[string]any{"range": "C29", "sheet": "ADT", "values": []any{[]any{46107}}})
	if len(log) != 1 {
		t.Fatalf("da dong y tren sheet ADT trong luot nay thi khong hoi lai: hoi %d lan %v", len(log), log)
	}
	if len(book.writes) != 2 {
		t.Fatalf("ca hai lenh ghi phai toi bridge: %v", book.writes)
	}

	// Luot moi (RunContext moi) thi phai hoi lai.
	var log2 []asked
	run2 := overwriteRun(port, server, &yes, &log2)
	invoke(tool, run2, "et.writeRange", map[string]any{"range": "S3", "sheet": "ADT", "values": []any{[]any{6368534}}})
	if len(log2) != 1 {
		t.Fatalf("luot moi phai hoi lai: %v", log2)
	}
}

func TestNoQuestionForEmptyCellsSameValuesOrOwnSheets(t *testing.T) {
	book := newBook()
	server, port := book.serve(t)
	defer server.Close()
	yes := true
	var log []asked
	run := overwriteRun(port, server, &yes, &log)
	tool := NewOfficeActionTool(calcCatalog(), "et")

	invoke(tool, run, "et.writeRange", map[string]any{"range": "Z10", "sheet": "ADT", "values": []any{[]any{1, 2}, []any{3, 4}}})
	invoke(tool, run, "et.writeRange", map[string]any{"range": "S3", "values": []any{[]any{5368534}}}) // sheet active, cung gia tri
	invoke(tool, run, "et.addSheet", map[string]any{"name": "Dashboard"})
	book.sheets["Dashboard"]["A1"] = "tieu de cu" // o co du lieu tren sheet do luot tao
	invoke(tool, run, "et.writeRange", map[string]any{"range": "A1", "sheet": "Dashboard", "values": []any{[]any{"DASHBOARD"}}})
	if len(log) != 0 {
		t.Fatalf("o trong / cung gia tri / sheet do luot tao thi khong hoi: %v", log)
	}
	if len(book.writes) != 4 {
		t.Fatalf("ca 4 lenh phai toi bridge: %v", book.writes)
	}
}

func TestWriteRangesAndFillRangeAreChecked(t *testing.T) {
	book := newBook()
	server, port := book.serve(t)
	defer server.Close()
	no := false
	var log []asked
	run := overwriteRun(port, server, &no, &log)
	tool := NewOfficeActionTool(calcCatalog(), "et")

	invoke(tool, run, "et.writeRanges", map[string]any{"sheet": "ADT", "writes": []any{
		map[string]any{"range": "Z1", "values": []any{[]any{"moi"}}},
		map[string]any{"range": "I4", "values": []any{[]any{64.78}}},
	}})
	if len(log) != 1 || !strings.Contains(log[0].reason, "I4") || strings.Contains(log[0].reason, "Z1") {
		t.Fatalf("writeRanges: hoi 1 lan, neu I4 (co du lieu), khong neu Z1 (trong): %v", log)
	}
	invoke(tool, run, "et.fillRange", map[string]any{"range": "C28:C30", "sheet": "ADT", "formula": "=1"})
	if len(log) != 2 || !strings.Contains(log[1].reason, "C29") {
		t.Fatalf("fillRange de o co du lieu phai hoi va neu C29: %v", log)
	}
	if len(book.writes) != 0 {
		t.Fatalf("da tu choi thi khong ghi: %v", book.writes)
	}
}

func TestNonInteractiveOverwriteIsDeclined(t *testing.T) {
	book := newBook()
	server, port := book.serve(t)
	defer server.Close()
	run := overwriteRun(port, server, nil, nil) // ai.ask: khong ai de hoi
	tool := NewOfficeActionTool(calcCatalog(), "et")
	result := invoke(tool, run, "et.writeRange", map[string]any{"range": "Q4", "sheet": "ADT", "values": []any{[]any{1}}})
	if result.OK || len(book.writes) != 0 {
		t.Fatalf("khong tuong tac thi ghi de bi tu choi: %+v writes=%v", result, book.writes)
	}
}

func TestReasonIsCappedForLargeOverwrites(t *testing.T) {
	book := newBook()
	for row := 0; row < 60; row++ {
		book.sheets["ADT"][cellName(1, row+100)] = fmt.Sprintf("v%d", row)
	}
	server, port := book.serve(t)
	defer server.Close()
	no := false
	var log []asked
	run := overwriteRun(port, server, &no, &log)
	tool := NewOfficeActionTool(calcCatalog(), "et")
	values := []any{}
	for row := 0; row < 60; row++ {
		values = append(values, []any{"x"})
	}
	invoke(tool, run, "et.writeRange", map[string]any{"range": "B101", "sheet": "ADT", "values": values})
	if len(log) != 1 || !strings.Contains(log[0].reason, "60") || len(log[0].reason) > 1200 {
		t.Fatalf("60 o: ly do neu tong so 60 va duoc rut gon (<1200 ky tu): %d ky tu %q", len(log[0].reason), log)
	}
}

// Hieu nang (review 11/10/2026): ban dau guard doc TUNG O qua bridge - ghi bang 1.000 dong x 10 cot la
// 10.000 lan goi bridge. Phai doc ca vung dich trong MOT lan et.readRange moi vung (moi muc writes).
func TestOverwriteGuardReadsEachTargetAreaOnce(t *testing.T) {
	book := newBook()
	server, port := book.serve(t)
	defer server.Close()
	yes := true
	var log []asked
	run := overwriteRun(port, server, &yes, &log)
	tool := NewOfficeActionTool(calcCatalog(), "et")

	values := []any{}
	for row := 0; row < 500; row++ {
		values = append(values, []any{row, row, row, row})
	}
	invoke(tool, run, "et.writeRange", map[string]any{"range": "AA1", "sheet": "ADT", "values": values})
	if book.reads > 1 {
		t.Fatalf("ghi 2.000 o (500x4) phai doc vung dich 1 lan, doc %d lan", book.reads)
	}

	book.reads = 0
	invoke(tool, run, "et.writeRanges", map[string]any{"sheet": "ADT", "writes": []any{
		map[string]any{"range": "AF1", "values": values},
		map[string]any{"range": "AK1", "values": values},
	}})
	if book.reads > 2 {
		t.Fatalf("writeRanges 2 vung phai doc toi da 2 lan, doc %d lan", book.reads)
	}

	book.reads = 0
	invoke(tool, run, "et.fillRange", map[string]any{"range": "AP1:AS500", "sheet": "ADT", "formula": "=1"})
	if book.reads > 1 {
		t.Fatalf("fillRange 2.000 o phai doc 1 lan, doc %d lan", book.reads)
	}
}

// Da dong y ghi de tren sheet, hoac sheet do luot tao: khong can doc gi nua (khong ton vong bridge).
func TestNoReadsForApprovedOrOwnSheets(t *testing.T) {
	book := newBook()
	server, port := book.serve(t)
	defer server.Close()
	yes := true
	var log []asked
	run := overwriteRun(port, server, &yes, &log)
	tool := NewOfficeActionTool(calcCatalog(), "et")

	invoke(tool, run, "et.writeRange", map[string]any{"range": "Q4", "sheet": "ADT", "values": []any{[]any{1}}})
	if len(log) != 1 {
		t.Fatalf("lan dau phai hoi: %v", log)
	}
	book.reads = 0
	invoke(tool, run, "et.writeRange", map[string]any{"range": "I4", "sheet": "ADT", "values": []any{[]any{2}}})
	if book.reads != 0 {
		t.Fatalf("sheet ADT da duoc dong y trong luot: khong doc nua, doc %d lan", book.reads)
	}
	invoke(tool, run, "et.addSheet", map[string]any{"name": "Dashboard"})
	book.reads = 0
	invoke(tool, run, "et.writeRange", map[string]any{"range": "A1", "sheet": "Dashboard", "values": []any{[]any{"x"}}})
	if book.reads != 0 {
		t.Fatalf("sheet do luot tao: khong doc, doc %d lan", book.reads)
	}
}
