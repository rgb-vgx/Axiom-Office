package store

import (
	"database/sql"

	"axiomoffice/core/internal/corelog"
)

// RunRow: mot luot chay da luu (docs/architecture/core.md muc 8.5.2).
type RunRow struct {
	ID             string
	ConversationID *string
	Status         string
	StartedAt      string
	FinishedAt     *string
	Model          string
	Rounds         int
	InputTokens    int
	OutputTokens   int
	Error          *string
	MemoryStatus   *string
}

// AuditRow: mot dong nhat ky tool call (GET /v1/audit).
type AuditRow struct {
	ID         int64
	RunID      string
	Seq        int
	Tool       string
	Action     *string
	ParamsJSON *string
	OK         bool
	Error      *string
	Ms         int64
	CreatedAt  string
}

// Runs: luot chay va audit tool call (muc 8.5.2, 8.6).
type Runs struct{ db *DB }

func (d *DB) Runs() *Runs { return &Runs{db: d} }

// exec: ghi audit/trang thai luot. Loi ghi khong lam hong luot chay, nhung phai de lai dau vet: truoc 27/10/2026 moi loi
// (o day, SQLITE_BUSY qua busy_timeout) bi bo im lang va audit mat ma khong ai biet (audit agent runtime F08).
func (r *Runs) exec(what, runID, query string, args ...any) {
	if _, err := r.db.sql.Exec(query, args...); err != nil {
		corelog.Error("core.db: %s for run %s failed: %v", what, runID, err)
	}
}

func (r *Runs) Insert(runID string, conversationID *string, model string) {
	r.exec("insert run", runID, `INSERT INTO runs (id, conversation_id, status, started_at, model, rounds, input_tokens, output_tokens)
		VALUES (?, ?, 'running', ?, ?, 0, 0, 0)`, runID, conversationID, Now(), model)
}

// SetMemoryStatus: null | queued | done | skipped | failed - doi sau khi run da xong (hang doi nen).
func (r *Runs) SetMemoryStatus(runID, memoryStatus string) {
	r.exec("set memory status", runID, "UPDATE runs SET memory_status = ? WHERE id = ?", memoryStatus, runID)
}

func (r *Runs) Finish(runID, status string, rounds, inputTokens, outputTokens, cachedTokens int, errText *string, memoryStatus *string) {
	r.exec("finish run", runID, `UPDATE runs
		SET status = ?, finished_at = ?, rounds = ?, input_tokens = ?, output_tokens = ?, cached_tokens = ?, error = ?,
		    memory_status = COALESCE(?, memory_status)
		WHERE id = ?`, status, Now(), rounds, inputTokens, outputTokens, cachedTokens, errText, memoryStatus, runID)
}

// AddToolCall: params rut gon <= 2KB, khong ghi noi dung file (muc 8.6). resultChars: kich thuoc ket qua DA VAO ngu
// canh model (sau khi cat) - khong ghi noi dung, chi do.
func (r *Runs) AddToolCall(runID string, seq int, tool string, action, paramsJSON *string, ok bool, errText *string, ms int64,
	resultChars int) {
	r.exec("add tool call", runID, `INSERT INTO tool_calls (run_id, seq, tool, action, params_json, ok, error, ms, created_at, result_chars)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, runID, seq, tool, action, truncatePtr(paramsJSON, 2048),
		boolToInt(ok), truncatePtr(errText, 500), ms, Now(), resultChars)
}

// RoundRow: so lieu mot vong goi model (schema v4).
type RoundRow struct {
	Round, InputTokens, CachedTokens, OutputTokens, ToolCalls int
	Kind                                                      string
}

// AddRound: mot vong goi model - de do vong nao gui bao nhieu, vong nao khong goi tool va vi sao.
func (r *Runs) AddRound(runID string, row RoundRow) {
	r.exec("add round", runID, `INSERT INTO run_rounds (run_id, round, input_tokens, cached_tokens, output_tokens, tool_calls, kind, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, runID, row.Round, row.InputTokens, row.CachedTokens, row.OutputTokens, row.ToolCalls,
		row.Kind, Now())
}

// StatusInterrupted: luot dang chay thi Core dung dot ngot (crash, bi kill, may tat) - chi co trong core.db, dat luc Core
// khoi dong lai. Tai lieu co the da bi sua mot phan ma khong co su kien ket thuc nao.
const StatusInterrupted = "interrupted"

// InterruptedRun: mot luot vua duoc doi sang 'interrupted' - hoi thoai cua no can mot dong bao cho luot sau.
type InterruptedRun struct {
	ID             string
	ConversationID *string
}

// MarkInterrupted: luot con 'running' trong core.db khi Core vua khoi dong la luot cua process truoc (moi nguoi dung chi
// mot Core, va luot chi song trong RAM) - doi sang 'interrupted' de khong treo 'running' mai mai. Tra cac luot da doi.
func (r *Runs) MarkInterrupted() []InterruptedRun {
	rows, err := r.db.sql.Query("SELECT id, conversation_id FROM runs WHERE status = 'running'")
	if err != nil {
		corelog.Error("core.db: list running runs failed: %v", err)
		return nil
	}
	found := []InterruptedRun{}
	for rows.Next() {
		var (
			item         InterruptedRun
			conversation sql.NullString
		)
		if rows.Scan(&item.ID, &conversation) == nil {
			item.ConversationID = nullString(conversation)
			found = append(found, item)
		}
	}
	_ = rows.Close()
	marked := []InterruptedRun{}
	for _, item := range found {
		result, err := r.db.sql.Exec(`UPDATE runs SET status = ?, finished_at = ?,
			error = COALESCE(error, 'Agent Core stopped while this run was in progress')
			WHERE id = ? AND status = 'running'`, StatusInterrupted, Now(), item.ID)
		if err != nil {
			corelog.Error("core.db: mark run %s interrupted failed: %v", item.ID, err)
			continue
		}
		if count, _ := result.RowsAffected(); count > 0 {
			marked = append(marked, item)
		}
	}
	return marked
}

// Rounds: cac vong cua mot luot theo thu tu.
func (r *Runs) Rounds(runID string) []RoundRow {
	rows := []RoundRow{}
	result, err := r.db.sql.Query(`SELECT round, input_tokens, cached_tokens, output_tokens, tool_calls, kind FROM run_rounds
		WHERE run_id = ? ORDER BY round`, runID)
	if err != nil {
		return rows
	}
	defer result.Close()
	for result.Next() {
		var row RoundRow
		if result.Scan(&row.Round, &row.InputTokens, &row.CachedTokens, &row.OutputTokens, &row.ToolCalls, &row.Kind) == nil {
			rows = append(rows, row)
		}
	}
	return rows
}

func (r *Runs) Get(runID string) *RunRow {
	row := r.db.sql.QueryRow(`SELECT id, conversation_id, status, started_at, finished_at, model, rounds,
		input_tokens, output_tokens, error, memory_status FROM runs WHERE id = ?`, runID)
	var (
		run                    RunRow
		conversation, finished sql.NullString
		errText, memoryStatus  sql.NullString
	)
	if row.Scan(&run.ID, &conversation, &run.Status, &run.StartedAt, &finished, &run.Model, &run.Rounds,
		&run.InputTokens, &run.OutputTokens, &errText, &memoryStatus) != nil {
		return nil
	}
	run.ConversationID = nullString(conversation)
	run.FinishedAt = nullString(finished)
	run.Error = nullString(errText)
	run.MemoryStatus = nullString(memoryStatus)
	return &run
}

// Audit: moi tool call, moi nhat truoc (GET /v1/audit, muc 7.3, 8.6).
func (r *Runs) Audit(runID *string, limit int) []AuditRow {
	query := `SELECT id, run_id, seq, tool, action, params_json, ok, error, ms, created_at FROM tool_calls`
	args := []any{}
	if runID != nil {
		query += " WHERE run_id = ?"
		args = append(args, *runID)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows := []AuditRow{}
	result, err := r.db.sql.Query(query, args...)
	if err != nil {
		return rows
	}
	defer result.Close()
	for result.Next() {
		var (
			row                 AuditRow
			run, action, params sql.NullString
			errText             sql.NullString
			ok                  int
		)
		if result.Scan(&row.ID, &run, &row.Seq, &row.Tool, &action, &params, &ok, &errText, &row.Ms, &row.CreatedAt) != nil {
			continue
		}
		row.RunID = run.String
		row.Action = nullString(action)
		row.ParamsJSON = nullString(params)
		row.OK = ok != 0
		row.Error = nullString(errText)
		rows = append(rows, row)
	}
	if result.Err() != nil {
		return []AuditRow{}
	}
	return rows
}

func truncatePtr(value *string, max int) *string {
	if value == nil {
		return nil
	}
	runes := []rune(*value)
	if len(runes) <= max {
		return value
	}
	truncated := string(runes[:max]) + "..."
	return &truncated
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
