package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/corelog"
)

// runTrace: DebugTrace - moi luot mot file traces/<run id>.jsonl, moi dong mot su kien {"t": ms tu dau luot, "event": ...}.
// Ghi NGUYEN VAN prompt, tool call va ket qua (core.db chi giu 200 ky tu loi, ~2.000 ky tu tham so): de doc lai dung
// cai model da thay khi soi loi. nil = khong bat; moi phuong thuc deu an toan voi nil.
type runTrace struct {
	mu      sync.Mutex
	file    *os.File
	started time.Time
}

// openTrace: tao traces/<runID>.jsonl canh core.log. Loi tao file chi ghi log - luot van chay.
func openTrace(cfg config.Config, runID string) *runTrace {
	if !cfg.DebugTrace {
		return nil
	}
	dir := filepath.Join(config.NewPaths(cfg.DataDirOverride).Root, "traces")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		corelog.Info("debug trace disabled for %s: %v", runID, err)
		return nil
	}
	path := filepath.Join(dir, runID+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		corelog.Info("debug trace disabled for %s: %v", runID, err)
		return nil
	}
	corelog.Info("run %s: debug trace %s", runID, path)
	return &runTrace{file: file, started: time.Now()}
}

func (t *runTrace) write(event string, fields map[string]any) {
	if t == nil {
		return
	}
	line := map[string]any{"t": time.Since(t.started).Milliseconds(), "event": event}
	for key, value := range fields {
		line[key] = value
	}
	raw, err := json.Marshal(line)
	if err != nil {
		raw, _ = json.Marshal(map[string]any{"t": line["t"], "event": event, "marshalError": err.Error()})
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = t.file.Write(append(raw, '\n'))
}

// callback: ham gan vao AgentCallbacks.Trace / RunContext.Trace (nil khi khong bat).
func (t *runTrace) callback() func(string, map[string]any) {
	if t == nil {
		return nil
	}
	return t.write
}

func (t *runTrace) close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_ = t.file.Close()
}

// traceStart: dong dau file - prompt, system prompt va danh sach tool nguyen van nhu model nhan duoc.
func (ex *execution) traceStart() {
	if ex.trace == nil {
		return
	}
	toolList := []map[string]any{}
	toolChars := 0
	for _, tool := range ex.registry.ModelTools() {
		size := len(tool.Name) + len(tool.Description) + len(tool.Parameters)
		toolChars += size
		toolList = append(toolList, map[string]any{"name": tool.Name, "chars": size, "description": tool.Description,
			"parameters": tool.Parameters})
	}
	prior := []map[string]any{}
	for _, turn := range ex.assembled.PriorTurns {
		prior = append(prior, map[string]any{"role": turn.Role, "content": turn.Content})
	}
	ex.trace.write("run", map[string]any{"run": ex.run.ID, "conversation": ex.conversation.ID, "app": ex.appKind,
		"model": ex.client.Model, "plan": ex.request.Plan, "prompt": ex.request.Prompt, "target": ex.runContext.TargetDocument,
		"options": map[string]any{"verify": ex.options.Verify, "maxRounds": ex.options.MaxRounds,
			"maxIdenticalCalls": ex.options.MaxIdenticalCalls, "contextClearAt": ex.options.ContextClearAt,
			"contextWindow": ex.options.ContextWindow},
		"systemPromptChars": len(ex.systemPrompt), "systemPrompt": ex.systemPrompt,
		"toolCount": len(toolList), "toolChars": toolChars, "tools": toolList, "prior": prior})
}
