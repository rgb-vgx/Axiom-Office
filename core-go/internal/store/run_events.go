package store

import (
	"time"

	"axiomoffice/core/internal/corelog"
)

// TimeText: moc thoi gian giong ban .NET (yyyy-MM-ddTHH:mm:ss.fffZ) - dung ca cho created_at va moc
// cat retention de so sanh chuoi trong SQL.
func TimeText(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}

// RunEventRow: mot su kien SSE da luu cua mot luot chay (schema v6). data_json giu nguyen payload JSON
// cua su kien de phat lai y nguyen cho pane (khong doi ten su kien hay hinh dang du lieu).
type RunEventRow struct {
	RunID     string
	Seq       int64
	Type      string
	DataJSON  string
	CreatedAt string
}

// AppendEvent: ghi mot su kien cua luot vao nhat ky ben (durable). INSERT OR REPLACE de ghi lai cung
// (run_id, seq) khong lam hong luot (vd sink bi goi hai lan vi mot duong thu lai).
func (r *Runs) AppendEvent(runID string, seq int64, eventType, dataJSON, createdAt string) error {
	_, err := r.db.sql.Exec(`INSERT OR REPLACE INTO run_events (run_id, seq, type, data_json, created_at)
		VALUES (?, ?, ?, ?, ?)`, runID, seq, eventType, dataJSON, createdAt)
	return err
}

// EventsSince: cac su kien co seq > afterSeq cua mot luot, theo thu tu seq - dung de phat lai SSE khi
// luot khong con trong RAM (Core khoi dong lai, hoac qua RetentionMinutes).
func (r *Runs) EventsSince(runID string, afterSeq int64) []RunEventRow {
	rows := []RunEventRow{}
	result, err := r.db.sql.Query(`SELECT run_id, seq, type, data_json, created_at FROM run_events
		WHERE run_id = ? AND seq > ? ORDER BY seq`, runID, afterSeq)
	if err != nil {
		corelog.Error("core.db: list run events for %s failed: %v", runID, err)
		return rows
	}
	defer func() { _ = result.Close() }()
	for result.Next() {
		var row RunEventRow
		if result.Scan(&row.RunID, &row.Seq, &row.Type, &row.DataJSON, &row.CreatedAt) == nil {
			rows = append(rows, row)
		}
	}
	return rows
}

// PruneRunEvents: xoa su kien cua cac luot da ket thuc truoc moc `before` (thuong la
// now - RetentionMinutes). Goi luc Core khoi dong va sau khi mot luot xong de core.db khong phinh mai.
// Su kien cua luot con dang chay (finished_at NULL) khong bi xoa du created_at cu.
func (r *Runs) PruneRunEvents(before string) int64 {
	result, err := r.db.sql.Exec(`DELETE FROM run_events WHERE run_id IN (
		SELECT id FROM runs WHERE finished_at IS NOT NULL AND finished_at < ?)`, before)
	if err != nil {
		corelog.Error("core.db: prune run events failed: %v", err)
		return 0
	}
	removed, _ := result.RowsAffected()
	return removed
}
