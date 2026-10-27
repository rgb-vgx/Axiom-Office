package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

// makesUndoStep khop truong "undo" cua catalog (nguon su that cua lenh): lenh co buoc hoan tac thi dem, lenh khong co
// thi khong (dem du thi undo lui duoc qua viec cua luot - dung loi can chan).
func TestMakesUndoStepMatchesCatalog(t *testing.T) {
	data, err := os.ReadFile("../../../catalog/live-commands.json")
	if err != nil {
		t.Skip("khong doc duoc catalog:", err)
	}
	var commands []struct {
		Name string `json:"name"`
		Undo bool   `json:"undo"`
	}
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatal(err)
	}
	for _, command := range commands {
		if got := makesUndoStep(command.Name); got != command.Undo {
			t.Errorf("%s: makesUndoStep=%v, catalog undo=%v - sua noUndoStep (undo.go)", command.Name, got, command.Undo)
		}
	}
}

// Undo chi lui phan viec cua luot: luot chua sua gi -> tu choi, khong goi bridge; sua 2 lenh -> undo {count:30} chi
// lui 2. Bo do Word 26/10/2026 (w08): writer.undo {count:30} xoa sach noi dung co san.
func TestUndoLimitedToThisRun(t *testing.T) {
	undoCounts := []any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request.Action == "writer.undo" {
			undoCounts = append(undoCounts, request.Params["count"])
			count, _ := request.Params["count"].(float64)
			_, _ = w.Write([]byte(`{"ok":true,"result":{"undone":` + strconv.Itoa(int(count)) + `}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	catalog := &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "writer.undo", Kind: "wps", Agent: true, Params: []office.CommandParam{{Name: "count"}}},
		{Name: "writer.appendText", Kind: "wps", Agent: true, Params: []office.CommandParam{{Name: "text", Required: true}}},
		{Name: "writer.getText", Kind: "wps", Agent: true},
	}}
	tool := NewOfficeActionTool(catalog, "wps")
	run := &RunContext{Office: &office.Session{App: "wps", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "t")}

	if result := call(tool, run, "writer.undo", map[string]any{"count": 30}); result.OK || !strings.Contains(result.JSON, "nothing to undo") || len(undoCounts) != 0 {
		t.Fatalf("luot chua sua gi thi khong duoc undo: %+v, bridge nhan %v", result, undoCounts)
	}
	call(tool, run, "writer.appendText", map[string]any{"text": "a"})
	call(tool, run, "writer.getText", nil)
	call(tool, run, "writer.appendText", map[string]any{"text": "b"})
	if result := call(tool, run, "writer.undo", map[string]any{"count": 30}); !result.OK || len(undoCounts) != 1 || undoCounts[0] != 2.0 {
		t.Fatalf("2 lenh sua thi undo toi da 2 buoc: %+v, bridge nhan %v", result, undoCounts)
	}
	if result := call(tool, run, "writer.undo", nil); result.OK {
		t.Fatalf("da lui het viec cua luot thi undo tiep phai bi tu choi: %+v", result)
	}
}

func TestUndoStepsSubtractsTheRunsOwnUndo(t *testing.T) {
	run := &RunContext{}
	countUndoable(run, "writer.appendText", true, nil)
	countUndoable(run, "writer.appendText", true, nil)
	countUndoable(run, "writer.getText", false, nil)
	countUndoable(run, "writer.undo", false, map[string]any{"undone": 1.0})
	if got := run.UndoSteps("writer"); got != 1 {
		t.Fatalf("2 lenh ghi, agent tu undo 1 buoc -> con 1 buoc cho nut Hoan tac luot nay, co %d", got)
	}
	if got := (*RunContext)(nil).UndoSteps("writer"); got != 0 {
		t.Fatalf("luot chua co ngu canh tool thi 0 buoc, co %d", got)
	}
}
