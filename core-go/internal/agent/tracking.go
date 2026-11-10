package agent

import (
	"context"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/office"
)

// Che do "AI sua bang Track Changes" (docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md W4): luot Word bat theo doi
// voi tac gia "Axiom AI"; "Hoan tac luot nay" = tu choi dung cac thay doi cua tac gia do tu luc bat dau luot - khong phu
// thuoc ngan undo cua Word, nen khong lui vao chu nguoi dung go xen giua (audit F03). Bat o dau luot, tra lai trong finish()
// (moi duong ket thuc deu di qua) bang context rieng.

// TrackingAuthor: ten tac gia cua thay doi do AI lam khi bat che do nay.
const TrackingAuthor = "Axiom AI"

// startTracking: bat theo doi cho luot neu duoc yeu cau (options.trackChanges, khong co thi TrackChangesEnabled).
func (o *Orchestrator) startTracking(ctx context.Context, run *Run, request *Request, appKind string,
	catalog *office.CommandCatalog, readOnly bool, target string) {
	if appKind != "wps" || readOnly || o.Bridge == nil || !catalog.Has("writer.setTracking") {
		return
	}
	want := o.Config().TrackChangesEnabled
	if request.TrackChanges != nil {
		want = *request.TrackChanges
	}
	if !want {
		return
	}
	params := map[string]any{"on": true, "author": TrackingAuthor}
	if target != "" {
		params["document"] = target
	}
	result := o.Bridge.Command(ctx, run.Port, "writer.setTracking", params)
	if !result.OK {
		corelog.Info("run %s: track changes not started: %s", run.ID, result.Error)
		return
	}
	previous, _ := result.Result["previous"].(bool)
	author, _ := result.Result["author"].(string)
	if strings.TrimSpace(author) != TrackingAuthor {
		// Word ghi tac gia theo tai khoan dang nhap: van theo doi, hoan tac luot tu choi theo ten do + thoi diem bat dau.
		corelog.Info("run %s: track changes author is %q, not %q (Office uses the signed-in account)", run.ID, author, TrackingAuthor)
	}
	run.update(func() {
		run.Tracking, run.TrackingPrevious, run.TrackingAuthor = true, previous, strings.TrimSpace(author)
		run.TrackingSince, run.TargetDocument = time.Now().UTC(), target
	})
	run.Events.Publish("run.tracking", map[string]any{"author": author})
}

// stopTracking: tra lai trang thai theo doi va ten nguoi dung nhu truoc luot.
func (o *Orchestrator) stopTracking(run *Run) {
	if o.Bridge == nil || !run.Tracking || run.trackingStopped {
		return
	}
	run.trackingStopped = true
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	params := map[string]any{"on": run.TrackingPrevious, "restoreAuthor": true}
	if run.TargetDocument != "" {
		params["document"] = run.TargetDocument
	}
	if result := o.Bridge.Command(stopCtx, run.Port, "writer.setTracking", params); !result.OK {
		corelog.Error("run %s: cannot restore track changes / user name: %s", run.ID, result.Error)
	}
}

// UndoRun: "Hoan tac luot nay" (POST /v1/runs/{id}/undo) - luot co theo doi thi tu choi revision cua luot, khong thi lui
// dung UndoSteps buoc. Mot lan moi luot.
func (o *Orchestrator) UndoRun(ctx context.Context, run *Run) (map[string]any, string) {
	if o.Bridge == nil {
		return nil, "no bridge"
	}
	run.mu.Lock()
	status, undone, tracking := run.Status, run.undone, run.Tracking
	steps, author, since, target, port := run.UndoSteps, run.TrackingAuthor, run.TrackingSince, run.TargetDocument, run.Port
	run.mu.Unlock()
	if status == StatusRunning {
		return nil, "the run is still running"
	}
	if undone {
		return nil, "this run was already undone"
	}
	action := ""
	var params map[string]any
	switch {
	case tracking && author != "":
		action = "writer.reviewRevisions"
		params = map[string]any{"action": "reject", "author": author, "since": since.Format(time.RFC3339)}
	case steps > 0:
		action = "writer.undo"
		params = map[string]any{"count": steps}
	default:
		return nil, "nothing to undo: this run did not change the document"
	}
	if target != "" {
		params["document"] = target
	}
	result := o.Bridge.Command(ctx, port, action, params)
	if !result.OK {
		return nil, action + ": " + result.Error
	}
	run.update(func() { run.undone, run.UndoSteps = true, 0 })
	corelog.Info("run %s undone by %s %s", run.ID, action, strconv.Quote(describeUndo(params)))
	reply := map[string]any{"mode": "steps", "result": result.Result}
	if action == "writer.reviewRevisions" {
		reply["mode"] = "revisions"
	}
	return reply, ""
}

// describeUndo: tham so hoan tac de ghi core.log.
func describeUndo(params map[string]any) string {
	parts := []string{}
	for _, key := range []string{"action", "author", "since", "count"} {
		if value, ok := params[key]; ok {
			parts = append(parts, key+"="+strings.TrimSpace(textOfAny(value)))
		}
	}
	return strings.Join(parts, " ")
}

func textOfAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case int:
		return strconv.Itoa(typed)
	case bool:
		return strconv.FormatBool(typed)
	}
	return ""
}

// undoMode: cach "Hoan tac luot nay" se lam (pane hien chu phu hop).
func (r *Run) undoMode() string {
	if r.Tracking {
		return "revisions"
	}
	return "steps"
}
