package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/store"
)

const (
	conversationWaitLimit = 10 * time.Second
	ssePingEvery          = 15 * time.Second
)

// Endpoint cua luot chay va hoi thoai (docs/architecture/core.md muc 7.3, 7.4):
// POST /v1/runs, GET /v1/runs/{id}, GET /v1/runs/{id}/events (SSE), POST /v1/runs/{id}/cancel,
// POST /v1/runs/{id}/confirm, GET /v1/audit, GET/DELETE /v1/conversations...
// Cac ham serve* dung chung cho /v1/runs va kenh add-in /wps/v1/runs (docs/architecture/wps-bridge.md muc 7).
func mapRuns(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("POST /v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if !deps.Stores.Available {
			Error(w, http.StatusServiceUnavailable, "memory unavailable: "+deps.Stores.Error)
			return
		}
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		request, problem := agent.ParseRequest(body)
		if request == nil {
			if problem == "" {
				problem = "invalid request"
			}
			Error(w, http.StatusBadRequest, problem)
			return
		}

		serveStartRun(w, deps, request)
	})

	routeBoth(mux, deps, "GET /v1/runs/{runId}", func(w http.ResponseWriter, r *http.Request) {
		serveGetRun(w, deps, r.PathValue("runId"))
	})

	routeBoth(mux, deps, "POST /v1/runs/{runId}/cancel", func(w http.ResponseWriter, r *http.Request) {
		serveCancelRun(w, deps, r.PathValue("runId"))
	})

	// Tra loi yeu cau xac nhan cua policy (muc 7.3, 8.6): {"confirmationId", "approved": true|false}.
	routeBoth(mux, deps, "POST /v1/runs/{runId}/confirm", func(w http.ResponseWriter, r *http.Request) {
		body, parsed := readJSON(r)
		if !parsed {
			body = nil
		}
		serveConfirmRun(w, deps, r.PathValue("runId"), body)
	})

	routeBoth(mux, deps, "GET /v1/audit", func(w http.ResponseWriter, r *http.Request) {
		runID := strings.TrimSpace(r.URL.Query().Get("runId"))
		limit := clampLimit(r.URL.Query().Get("limit"), 100)
		var filter *string
		if runID != "" {
			filter = &runID
		}
		calls := []any{}
		for _, row := range deps.Stores.Runs.Audit(filter, limit) {
			calls = append(calls, map[string]any{
				"id": row.ID, "runId": row.RunID, "seq": row.Seq, "tool": row.Tool, "action": row.Action,
				"params": row.ParamsJSON, "ok": row.OK, "error": row.Error, "ms": row.Ms, "createdAt": row.CreatedAt,
			})
		}
		OK(w, map[string]any{"calls": calls})
	})

	routeBoth(mux, deps, "GET /v1/runs/{runId}/events", func(w http.ResponseWriter, r *http.Request) {
		serveRunEvents(w, r, deps, r.PathValue("runId"))
	})

	routeBoth(mux, deps, "GET /v1/conversations", func(w http.ResponseWriter, r *http.Request) {
		documentKey := strings.TrimSpace(r.URL.Query().Get("documentKey"))
		limit := clampLimit(r.URL.Query().Get("limit"), 20)
		var key *string
		if documentKey != "" {
			key = &documentKey
		}
		list := []any{}
		for _, row := range deps.Stores.Convs.List(key, limit) {
			list = append(list, conversationJSON(row))
		}
		OK(w, map[string]any{"conversations": list})
	})

	routeBoth(mux, deps, "GET /v1/conversations/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		row := deps.Stores.Convs.Get(id)
		if row == nil {
			Error(w, http.StatusNotFound, "unknown conversation: "+id)
			return
		}
		messages := []any{}
		for _, message := range deps.Stores.Convs.Messages(id, 0) {
			messages = append(messages, map[string]any{
				"seq": message.Seq, "role": message.Role, "content": message.Content, "createdAt": message.CreatedAt,
			})
		}
		payload := conversationJSON(*row)
		payload["messages"] = messages
		OK(w, payload)
	})

	routeBoth(mux, deps, "DELETE /v1/conversations/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !deps.Stores.Convs.Delete(id) {
			Error(w, http.StatusNotFound, "unknown conversation: "+id)
			return
		}
		OK(w, map[string]any{"deleted": id})
	})
}

// serveStartRun: khoi dong luot chay tu Request da parse (dung chung cho /v1/runs va /wps/v1/runs).
func serveStartRun(w http.ResponseWriter, deps *Deps, request *agent.Request) {
	run, startError := deps.Manager.TryStart(request.Port)
	if run == nil {
		if startError == "" {
			startError = "cannot start run"
		}
		Error(w, http.StatusConflict, startError)
		return
	}
	go deps.Orchestrator.Execute(run.Context(), run, request)

	// Hoi thoai duoc tao ngay dau luot chay: cho toi da 10s de tra ve conversationId nhu tai lieu.
	deadline := time.Now().Add(conversationWaitLimit)
	status, conversationID := run.StatusAndConversation()
	for conversationID == "" && status == agent.StatusRunning && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		status, conversationID = run.StatusAndConversation()
	}
	OK(w, map[string]any{
		"runId": run.ID, "conversationId": nilIfEmpty(conversationID), "status": status,
	})
}

// serveGetRun: trang thai mot luot chay (dung chung cho /v1/runs/{id} va /wps/v1/runs/{id}).
func serveGetRun(w http.ResponseWriter, deps *Deps, runID string) {
	run := deps.Manager.Get(runID)
	if run == nil {
		// Khong con trong RAM (Core khoi dong lai, hoac qua RetentionMinutes): tra trang thai da luu trong core.db de pane
		// ket noi lai thay luot da ket thuc the nao (Grok Build: su kien ket thuc luot duoc luu va phat lai), thay vi 404.
		if stored := storedRunJSON(deps, runID); stored != nil {
			OK(w, stored)
			return
		}
		Error(w, http.StatusNotFound, "unknown run: "+runID)
		return
	}
	OK(w, run.JSON())
}

// storedRunJSON: hinh dang GET /v1/runs/{id} dung tu dong runs trong core.db (khong co reply/transcript - chi luu trong RAM).
// "stored": true de ben goi biet day la ban luu, khong phai luot dang song.
func storedRunJSON(deps *Deps, runID string) map[string]any {
	if deps.Stores == nil || !deps.Stores.Available || deps.Stores.Runs == nil {
		return nil
	}
	row := deps.Stores.Runs.Get(runID)
	if row == nil {
		return nil
	}
	return map[string]any{
		"runId": row.ID, "conversationId": row.ConversationID, "status": row.Status, "started": row.StartedAt,
		"finished": row.FinishedAt, "reply": nil, "error": row.Error, "rounds": row.Rounds,
		"inputTokens": row.InputTokens, "outputTokens": row.OutputTokens, "model": row.Model, "stored": true,
	}
}

// serveCancelRun: huy luot dang chay (dung chung cho /v1 va /wps).
func serveCancelRun(w http.ResponseWriter, deps *Deps, runID string) {
	run := deps.Manager.Get(runID)
	if run == nil {
		Error(w, http.StatusNotFound, "unknown run: "+runID)
		return
	}
	if !deps.Manager.Cancel(run.ID) {
		Error(w, http.StatusConflict, "run is not running (status "+run.CurrentStatus()+")")
		return
	}
	OK(w, map[string]any{"cancelled": true, "status": run.CurrentStatus()})
}

// serveConfirmRun: tra loi yeu cau xac nhan cua policy (dung chung cho /v1 va /wps).
func serveConfirmRun(w http.ResponseWriter, deps *Deps, runID string, body map[string]any) {
	run := deps.Manager.Get(runID)
	if run == nil {
		Error(w, http.StatusNotFound, "unknown run: "+runID)
		return
	}
	id, hasID := body["confirmationId"].(string)
	approved, hasApproved := body["approved"].(bool)
	// The nhieu lua chon: {"confirmationId", "choice": "<id>"}.
	choice, hasChoice := body["choice"].(string)
	hasChoice = hasChoice && strings.TrimSpace(choice) != ""
	if !hasID || (!hasApproved && !hasChoice) || strings.TrimSpace(id) == "" {
		Error(w, http.StatusBadRequest, "'confirmationId' and 'approved' (true|false) or 'choice' are required")
		return
	}
	resolved := false
	if hasChoice {
		// O tu go ("Kieu khac"): {"choice": "custom", "text": "..."}.
		text, _ := body["text"].(string)
		resolved = run.Confirmations.ResolveAnswer(id, strings.TrimSpace(choice), text)
	} else {
		resolved = run.Confirmations.Resolve(id, approved)
	}
	if !resolved {
		Error(w, http.StatusNotFound, "no pending confirmation: "+id)
		return
	}
	if hasChoice {
		OK(w, map[string]any{"confirmationId": id, "choice": strings.TrimSpace(choice)})
		return
	}
	OK(w, map[string]any{"confirmationId": id, "approved": approved})
}

// serveRunEvents: SSE cua mot luot chay (dung chung cho /v1 va /wps).
func serveRunEvents(w http.ResponseWriter, r *http.Request, deps *Deps, runID string) {
	run := deps.Manager.Get(runID)
	if run == nil {
		Error(w, http.StatusNotFound, "unknown run: "+runID)
		return
	}
	streamEvents(w, r, run)
}

// streamEvents: SSE - phat lai su kien cu (neu client ket noi lai voi ?after=) roi tiep tuc nghe.
func streamEvents(w http.ResponseWriter, r *http.Request, run *agent.Run) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, canFlush := w.(http.Flusher)
	flush := func() {
		if canFlush {
			flusher.Flush()
		}
	}

	cursor := parseAfter(r)
	lastWrite := time.Now()
	for {
		if r.Context().Err() != nil {
			return
		}
		wrote := false
		for _, item := range run.Events.Since(cursor) {
			cursor = item.Seq
			if writeEvent(w, item) != nil {
				return
			}
			wrote = true
		}
		if wrote {
			flush()
			lastWrite = time.Now()
			continue
		}
		if run.Events.Completed() {
			break
		}

		wait := ssePingEvery - time.Since(lastWrite)
		if wait <= 0 {
			if writePing(w) != nil {
				return
			}
			flush()
			lastWrite = time.Now()
			continue
		}
		if !run.Events.WaitForChange(wait, r.Context()) {
			break
		}
	}
	flush()
}

func writeEvent(w http.ResponseWriter, item agent.Event) error {
	payload := model.MarshalRelaxed(map[string]any{
		"seq": item.Seq, "type": item.Type, "time": item.At, "data": item.Data,
	})
	if _, err := w.Write([]byte("id: " + strconv.FormatInt(item.Seq, 10) + "\n")); err != nil {
		return err
	}
	return writeRaw(w, item.Type, payload)
}

func writePing(w http.ResponseWriter) error {
	payload := model.MarshalRelaxed(map[string]any{
		"seq": 0, "type": "ping", "time": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), "data": nil,
	})
	return writeRaw(w, "ping", payload)
}

func writeRaw(w http.ResponseWriter, eventType, payload string) error {
	_, err := w.Write([]byte("event: " + eventType + "\ndata: " + payload + "\n\n"))
	return err
}

// LogSSEError: SSE hong vi client ngat ket noi la binh thuong, cac loi khac thi ghi log.
func LogSSEError(runID string, err error) {
	if err == nil {
		return
	}
	corelog.Error("SSE run %s failed: %v", runID, err)
}

func parseAfter(r *http.Request) int64 {
	raw := r.URL.Query().Get("after")
	if raw == "" {
		raw = r.Header.Get("Last-Event-ID")
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func clampLimit(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	if value > 200 {
		return 200
	}
	return value
}

func conversationJSON(row store.ConversationRow) map[string]any {
	return map[string]any{
		"id": row.ID, "documentKey": row.DocumentKey, "documentName": row.DocumentName, "app": row.App,
		"family": row.Family, "title": row.Title, "summary": row.Summary,
		"createdAt": row.CreatedAt, "updatedAt": row.UpdatedAt,
	}
}

func nilIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
