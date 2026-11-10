package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"axiomoffice/core/internal/config"
)

func TestRunTraceWritesJSONLines(t *testing.T) {
	dir := t.TempDir()
	if trace := openTrace(config.Config{DataDirOverride: dir}, "r_off"); trace != nil {
		t.Fatal("DebugTrace tat thi khong duoc mo file")
	}
	var off *runTrace
	off.write("x", nil) // nil an toan
	off.close()
	if off.callback() != nil {
		t.Fatal("trace tat thi callback phai nil")
	}

	trace := openTrace(config.Config{DebugTrace: true, DataDirOverride: dir}, "r_on")
	if trace == nil {
		t.Fatal("DebugTrace bat thi phai mo file")
	}
	trace.callback()("tool", map[string]any{"name": "office_action", "result": strings.Repeat("x", 5000)})
	trace.write("result", map[string]any{"ok": true})
	trace.close()

	raw, err := os.ReadFile(filepath.Join(dir, "traces", "r_on.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("muon 2 dong, co %d", len(lines))
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first["event"] != "tool" || len(first["result"].(string)) != 5000 {
		t.Fatalf("dong dau phai giu nguyen ket qua: %v", first["event"])
	}
}
