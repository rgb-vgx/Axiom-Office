package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

// fakeWorkbook: bridge gia tra et.readRanges / et.sheetInfo tu mot so tinh trong bo nho. cells: "Sheet!A1" -> {formula, value}.
type fakeCell struct {
	formula string
	value   any
}

func fakeWorkbookBridge(t *testing.T, cells map[string]fakeCell, used map[string]string) (*RunContext, *int) {
	t.Helper()
	calls := 0
	lookup := func(sheet string, row, col int) fakeCell { return cells[sheet+"!"+a1Name(row, col)] }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		reply := map[string]any{}
		switch request.Action {
		case "et.sheetInfo":
			sheets := []any{}
			for name, area := range used {
				sheets = append(sheets, map[string]any{"name": name, "usedRange": area})
			}
			reply = map[string]any{"activeSheet": "Summary", "sheets": sheets}
		case "et.readRanges":
			results := []any{}
			for _, item := range request.Params["reads"].([]any) {
				read := item.(map[string]any)
				refs, _ := ParseFormulaRefs(read["range"].(string))
				ref := refs[0]
				matrix := []any{}
				for row := ref.FromRow; row <= ref.ToRow; row++ {
					line := []any{}
					for col := ref.FromCol; col <= ref.ToCol; col++ {
						cell := lookup(read["sheet"].(string), row, col)
						if read["formulas"] == true {
							if cell.formula == "" {
								line = append(line, nil)
							} else {
								line = append(line, cell.formula)
							}
						} else {
							line = append(line, cell.value)
						}
					}
					matrix = append(matrix, line)
				}
				key := "values"
				if read["formulas"] == true {
					key = "formulas"
				}
				results = append(results, map[string]any{"sheet": read["sheet"], key: matrix})
			}
			reply = map[string]any{"reads": results}
		}
		data, _ := json.Marshal(map[string]any{"ok": true, "result": reply})
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	return &RunContext{Office: &office.Session{App: "et", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "t")}, &calls
}

func traceResult(t *testing.T, result Result) map[string]any {
	t.Helper()
	if !result.OK {
		t.Fatalf("trace_formula loi: %s", result.JSON)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(result.JSON), &body); err != nil {
		t.Fatal(err)
	}
	return body["result"].(map[string]any)
}

func TestTracePrecedentsFindsRootCauseAcrossSheets(t *testing.T) {
	cells := map[string]fakeCell{
		"Summary!H15":    {"=Calc!B2*2", "#DIV/0!"},
		"Calc!B2":        {"=Data!A1/Data!A2", "#DIV/0!"},
		"Data!A1":        {"", 10.0},
		"Data!A2":        {"", 0.0},
		"Summary!H16":    {"=SUM('Bao cao Q3'!B2:B100)", "#N/A"},
		"Bao cao Q3!B50": {"", "#N/A"},
	}
	run, _ := fakeWorkbookBridge(t, cells, nil)
	tool := NewTraceFormulaTool()

	body := traceResult(t, tool.Invoke(t.Context(), map[string]any{"cell": "H15"}, run))
	roots := body["rootCauses"].([]any)
	if body["cell"] != "Summary!H15" || len(roots) != 1 || roots[0] != "Calc!B2" {
		t.Fatalf("goc loi phai la Calc!B2 (chia cho Data!A2 = 0): %v", body)
	}
	if nodes := body["nodes"].([]any); len(nodes) != 4 {
		t.Fatalf("H15 -> Calc!B2 -> Data!A1, Data!A2: %v", nodes)
	}

	// Vung lon: khong lan tung o, chi tim o loi trong vung.
	body = traceResult(t, tool.Invoke(t.Context(), map[string]any{"cell": "Summary!H16"}, run))
	roots = body["rootCauses"].([]any)
	if len(roots) != 1 || roots[0] != "'Bao cao Q3'!B50" {
		t.Fatalf("goc loi phai la o #N/A go tay trong vung lon: %v", body)
	}
}

func TestTraceDependentsScansWorkbook(t *testing.T) {
	cells := map[string]fakeCell{
		"Data!A1":    {"", 5.0},
		"Data!B1":    {"=A1*2", 10.0},
		"Calc!C3":    {"=SUM(Data!A1:A9)", 5.0},
		"Summary!D4": {"=Calc!C3+1", 6.0},
		"Summary!D5": {"=\"A1\"", "A1"},
	}
	used := map[string]string{"Data": "A1:B1", "Calc": "A1:C3", "Summary": "A1:D5"}
	run, _ := fakeWorkbookBridge(t, cells, used)
	body := traceResult(t, NewTraceFormulaTool().Invoke(t.Context(), map[string]any{"cell": "Data!A1", "direction": "dependents"}, run))
	refs := []string{}
	for _, item := range body["nodes"].([]any) {
		refs = append(refs, item.(map[string]any)["ref"].(string))
	}
	got := strings.Join(refs, ",")
	if got != "Data!A1,Calc!C3,Data!B1,Summary!D4" {
		t.Fatalf("dependents: Data!B1 va Calc!C3 doc A1, Summary!D4 doc Calc!C3; chuoi \"A1\" khong tinh: %s", got)
	}
}

func TestTraceRejectsRanges(t *testing.T) {
	run, calls := fakeWorkbookBridge(t, nil, nil)
	result := NewTraceFormulaTool().Invoke(t.Context(), map[string]any{"cell": "A1:B2"}, run)
	if result.OK || *calls != 0 {
		t.Fatalf("chi nhan mot o: %+v", result)
	}
}

func TestParseFormulaRefs(t *testing.T) {
	cases := map[string]string{
		"=A1+$B$2":                           "A1|$B$2",
		"=SUM('Bao cao Q3'!B2:B100)*Data!C1": "'Bao cao Q3'!B2:B100|Data!C1",
		"=LOG10(A1)&\"B2\"":                  "A1",
		"=VLOOKUP(A2,Data!A:C,3,FALSE)":      "A2|Data!A:C",
		"=SUM(3:5)":                          "3:5",
		"=Sheet1.A1":                         "",
	}
	for formula, want := range cases {
		refs, _ := ParseFormulaRefs(formula)
		texts := []string{}
		for _, ref := range refs {
			texts = append(texts, ref.Text)
		}
		if got := strings.Join(texts, "|"); got != want {
			t.Errorf("ParseFormulaRefs(%q) = %q, muon %q", formula, got, want)
		}
	}
	if _, unresolved := ParseFormulaRefs("=INDIRECT(\"A\"&B1)"); len(unresolved) == 0 {
		t.Error("INDIRECT phai bao khong lan duoc")
	}
}
