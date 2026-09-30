package store

import "database/sql"

// RunRow: mot luot chay da luu (New_arch.md muc 8.5.2).
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

func (r *Runs) Insert(runID string, conversationID *string, model string) {
	_, _ = r.db.sql.Exec(`INSERT INTO runs (id, conversation_id, status, started_at, model, rounds, input_tokens, output_tokens)
		VALUES (?, ?, 'running', ?, ?, 0, 0, 0)`, runID, conversationID, Now(), model)
}

// SetMemoryStatus: null | queued | done | skipped | failed - doi sau khi run da xong (hang doi nen).
func (r *Runs) SetMemoryStatus(runID, memoryStatus string) {
	_, _ = r.db.sql.Exec("UPDATE runs SET memory_status = ? WHERE id = ?", memoryStatus, runID)
}

func (r *Runs) Finish(runID, status string, rounds, inputTokens, outputTokens int, errText *string, memoryStatus *string) {
	_, _ = r.db.sql.Exec(`UPDATE runs
		SET status = ?, finished_at = ?, rounds = ?, input_tokens = ?, output_tokens = ?, error = ?,
		    memory_status = COALESCE(?, memory_status)
		WHERE id = ?`, status, Now(), rounds, inputTokens, outputTokens, errText, memoryStatus, runID)
}

// AddToolCall: params rut gon <= 2KB, khong ghi noi dung file (muc 8.6).
func (r *Runs) AddToolCall(runID string, seq int, tool string, action, paramsJSON *string, ok bool, errText *string, ms int64) {
	_, _ = r.db.sql.Exec(`INSERT INTO tool_calls (run_id, seq, tool, action, params_json, ok, error, ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, runID, seq, tool, action, truncatePtr(paramsJSON, 2048),
		boolToInt(ok), truncatePtr(errText, 500), ms, Now())
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
