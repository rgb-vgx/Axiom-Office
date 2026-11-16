package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/policy"
	"axiomoffice/core/internal/store"
)

const (
	ssePingEvery = 15 * time.Second
)

// Endpoint cua luot chay va hoi thoai (docs/architecture/core.md muc 7.3, 7.4):
// POST /v1/runs, GET /v1/runs/{id}, GET /v1/runs/{id}/events (SSE), POST /v1/runs/{id}/cancel,
// POST /v1/runs/{id}/confirm, GET /v1/audit, GET/DELETE /v1/conversations...
// Cac ham serve* dung chung cho /v1/runs va kenh add-in /wps/v1/runs (docs/architecture/wps-bridge.md muc 7).
func mapRuns(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("POST /v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if !deps.Stores.Available {
			ErrorCode(w, http.StatusServiceUnavailable, CodeCoreUnavailable, "memory unavailable: "+deps.Stores.Error)
			return
		}
		body, parsed := readJSON(r)
		if !parsed {
			ErrorCode(w, http.StatusBadRequest, CodeInvalidRequest, "invalid JSON body")
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

	// "Hoan tac luot nay" (agent/tracking.go UndoRun): luot co Track Changes thi tu choi revision cua luot, khong thi lui
	// dung so buoc cua luot. Pane goi day thay vi tu dem buoc (audit F02/F03).
	routeBoth(mux, deps, "POST /v1/runs/{runId}/undo", func(w http.ResponseWriter, r *http.Request) {
		serveUndoRun(w, r, deps, r.PathValue("runId"))
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
	run, startError, existing := deps.Manager.TryStartRequest(request.Port, request.ClientRequestID)
	if run == nil {
		if startError == "" {
			startError = "cannot start run"
		}
		ErrorCode(w, http.StatusConflict, CodeBusy, startError)
		return
	}
	// Gui lai cung clientRequestId: tra ve luot cu (200, cung runId/conversationId), khong chay lan hai.
	if existing {
		status, conversationID := run.StatusAndConversation()
		OK(w, map[string]any{
			"runId": run.ID, "conversationId": nilIfEmpty(conversationID), "status": status,
		})
		return
	}
	// Hoi thoai + dong runs tao NGAY (truoc goroutine) de tra conversationId ma khong phai cho.
	if deps.Orchestrator != nil {
		deps.Orchestrator.Admit(run, request)
	}
	status, conversationID := run.StatusAndConversation()
	if deps.Orchestrator != nil {
		go deps.Orchestrator.Execute(run.Context(), run, request)
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
		ErrorCode(w, http.StatusNotFound, CodeNotFound, "unknown run: "+runID)
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
		ErrorCode(w, http.StatusNotFound, CodeNotFound, "unknown run: "+runID)
		return
	}
	if !deps.Manager.Cancel(run.ID) {
		Error(w, http.StatusConflict, "run is not running (status "+run.CurrentStatus()+")")
		return
	}
	OK(w, map[string]any{"cancelled": true, "status": run.CurrentStatus()})
}

// serveUndoRun: hoan tac ca luot (dung chung cho /v1 va /wps). Luot khac dang chay tren cung cong thi 409.
func serveUndoRun(w http.ResponseWriter, r *http.Request, deps *Deps, runID string) {
	run := deps.Manager.Get(runID)
	if run == nil {
		ErrorCode(w, http.StatusNotFound, CodeNotFound, "unknown run: "+runID)
		return
	}
	if deps.Manager.BusyPort(run.Port) {
		ErrorCode(w, http.StatusConflict, CodeBusy, "a run is in progress on this document: wait for it to finish")
		return
	}
	result, problem := deps.Orchestrator.UndoRun(r.Context(), run)
	if problem != "" {
		Error(w, http.StatusConflict, problem)
		return
	}
	OK(w, result)
}

// serveConfirmRun: tra loi yeu cau xac nhan cua policy (dung chung cho /v1 va /wps).
// "always": true (tuy chon) = luu phe duyet cho action+resource cua cau hoi nay de luot sau khong hoi lai.
func serveConfirmRun(w http.ResponseWriter, deps *Deps, runID string, body map[string]any) {
	run := deps.Manager.Get(runID)
	if run == nil {
		ErrorCode(w, http.StatusNotFound, CodeNotFound, "unknown run: "+runID)
		return
	}
	id, hasID := body["confirmationId"].(string)
	approved, hasApproved := body["approved"].(bool)
	always, _ := body["always"].(bool)
	// The nhieu lua chon: {"confirmationId", "choice": "<id>"}.
	choice, hasChoice := body["choice"].(string)
	hasChoice = hasChoice && strings.TrimSpace(choice) != ""
	if !hasID || (!hasApproved && !hasChoice) || strings.TrimSpace(id) == "" {
		Error(w, http.StatusBadRequest, "'confirmationId' and 'approved' (true|false) or 'choice' are required")
		return
	}
	// Doc action/resource truoc khi Resolve (yeu cau da xu ly xong la khong con trong danh sach cho).
	action, resource, hasPending := run.Confirmations.PendingAction(id)
	resolved := false
	if hasChoice {
		// O tu go ("Kieu khac"): {"choice": "custom", "text": "..."}.
		text, _ := body["text"].(string)
		resolved = run.Confirmations.ResolveAnswer(id, strings.TrimSpace(choice), text)
	} else {
		resolved = run.Confirmations.Resolve(id, approved)
	}
	if !resolved {
		ErrorCode(w, http.StatusNotFound, CodeNotFound, "no pending confirmation: "+id)
		return
	}
	response := map[string]any{"confirmationId": id}
	if hasChoice {
		response["choice"] = strings.TrimSpace(choice)
	} else {
		response["approved"] = approved
	}
	// always=true chi co nghia khi DONG Y mot lenh; cau hoi khong biet action (vd the lua chon) thi khong luu.
	if !hasChoice && approved && always && hasPending && strings.TrimSpace(action) != "" {
		if row, saved := saveApproval(deps, action, resource); saved {
			response["always"] = true
			response["approval"] = approvalJSON(row)
		}
	}
	OK(w, response)
}

// saveApproval: luu phe duyet "luon dong y" cho mot cau hoi da duoc nguoi dung chap nhan. Khong biet tai
// lieu (resource rong) -> scope global; nguoc lai mot tai lieu. Loi ghi chi ghi log: cau tra loi dong y
// cua nguoi dung khong duoc hong vi viec luu phe duyet.
func saveApproval(deps *Deps, action, resource string) (store.ApprovalRow, bool) {
	if deps.Stores == nil || deps.Stores.Approvals == nil {
		return store.ApprovalRow{}, false
	}
	// Chi lop lenh rui ro mac dinh moi duoc luu: "luon dong y" cho et.writeRange se bo qua guard
	// ghi de/sort cua moi tai lieu, khong phai y nguoi dung muon khi bam hop thoai.
	if !policy.Approvable(action) {
		return store.ApprovalRow{}, false
	}
	scope := store.ApprovalScopeDocument
	if strings.TrimSpace(resource) == "" {
		scope, resource = store.ApprovalScopeGlobal, "*"
	}
	row, err := deps.Stores.Approvals.AddApproval(action, resource, scope, "")
	if err != nil {
		corelog.Error("save approval for %s failed: %v", action, err)
		return store.ApprovalRow{}, false
	}
	corelog.Info("approval %s saved for %s on %q (scope %s)", row.ID, row.Action, row.Resource, row.Scope)
	return row, true
}

// serveRunEvents: SSE cua mot luot chay (dung chung cho /v1 va /wps).
//
// Luot con trong RAM: nghe truc tiep EventStream nhu cu. Luot khong con (Core vua khoi dong lai, hoac da
// qua RetentionMinutes): phat lai run_events da luu tu ?after=/Last-Event-ID roi ket thuc - pane thay duoc
// tien trinh va ky thuat ho tro xem lai duoc mot luot da xong. 404 chi khi khong co ca dong runs lan su kien.
func serveRunEvents(w http.ResponseWriter, r *http.Request, deps *Deps, runID string) {
	if deps.Manager != nil {
		if run := deps.Manager.Get(runID); run != nil {
			streamEvents(w, r, run)
			return
		}
	}
	events, stored := storedRunEvents(deps, runID, parseAfter(r))
	if !stored {
		ErrorCode(w, http.StatusNotFound, CodeNotFound, "unknown run: "+runID)
		return
	}
	streamStoredEvents(w, events)
}

// storedRunEvents: su kien da luu cua mot luot (schema v6) sau moc after. stored = co dong runs hoac co
// su kien nao do cua run id nay (ke ca khi tat ca deu <= after) - chi tra false khi khong co gi ca.
func storedRunEvents(deps *Deps, runID string, after int64) ([]store.RunEventRow, bool) {
	if deps.Stores == nil || !deps.Stores.Available || deps.Stores.Runs == nil {
		return nil, false
	}
	events := deps.Stores.Runs.EventsSince(runID, after)
	if deps.Stores.Runs.Get(runID) != nil || len(events) > 0 {
		return events, true
	}
	// Client da o cuoi (after >= seq cuoi) nhung luot van co su kien: van la luot da biet.
	if after > 0 && len(deps.Stores.Runs.EventsSince(runID, 0)) > 0 {
		return events, true
	}
	return nil, false
}

// streamStoredEvents: SSE phat lai su kien da luu roi ket thuc (khong con gi de cho). Cung dinh dang
// `id:` + `event:` + `data:` nhu stream song.
func streamStoredEvents(w http.ResponseWriter, events []store.RunEventRow) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	for _, row := range events {
		var data map[string]any
		if row.DataJSON != "" && row.DataJSON != "null" {
			_ = json.Unmarshal([]byte(row.DataJSON), &data)
		}
		item := agent.Event{Seq: row.Seq, Type: row.Type, Data: data, At: row.CreatedAt}
		if writeEvent(w, item) != nil {
			return
		}
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
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
