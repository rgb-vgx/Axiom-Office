package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// Ghi su kien cua mot luot va doc lai tu ?after=: dung cho replay SSE sau khi Core khoi dong lai.
func TestRunEventsAppendAndSince(t *testing.T) {
	db := openTestDB(t)
	runs := db.Runs()
	runs.Insert("r_1", nil, "m")

	if err := runs.AppendEvent("r_1", 1, "run.started", `{"round":0}`, "2026-11-16T10:00:00.000Z"); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := runs.AppendEvent("r_1", 2, "tool.started", `{"tool":"writer.text"}`, "2026-11-16T10:00:01.000Z"); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := runs.AppendEvent("r_1", 3, "run.completed", `null`, "2026-11-16T10:00:02.000Z"); err != nil {
		t.Fatalf("append: %v", err)
	}

	all := runs.EventsSince("r_1", 0)
	if len(all) != 3 || all[0].Seq != 1 || all[1].Seq != 2 || all[2].Seq != 3 {
		t.Fatalf("EventsSince(0) = %+v", all)
	}
	if all[0].Type != "run.started" || all[0].DataJSON != `{"round":0}` || all[0].CreatedAt != "2026-11-16T10:00:00.000Z" {
		t.Fatalf("su kien phai giu nguyen type/data/thoi diem: %+v", all[0])
	}
	if got := runs.EventsSince("r_1", 2); len(got) != 1 || got[0].Type != "run.completed" {
		t.Fatalf("EventsSince(2) = %+v", got)
	}
	if got := runs.EventsSince("r_1", 3); len(got) != 0 {
		t.Fatalf("EventsSince(3) = %+v", got)
	}
	if got := runs.EventsSince("r_other", 0); len(got) != 0 {
		t.Fatalf("luot khac khong duoc thay su kien: %+v", got)
	}

	// Ghi lai cung (run_id, seq) thay dong cu, khong nhan doi.
	if err := runs.AppendEvent("r_1", 2, "tool.finished", `{"ok":true}`, "2026-11-16T10:00:03.000Z"); err != nil {
		t.Fatalf("append lai: %v", err)
	}
	all = runs.EventsSince("r_1", 0)
	if len(all) != 3 || all[1].Type != "tool.finished" || all[1].DataJSON != `{"ok":true}` {
		t.Fatalf("append cung seq phai thay dong cu: %+v", all)
	}
}

// Chi don su kien cua luot da xong qua RetentionMinutes; luot dang chay va luot vua xong giu nguyen.
func TestPruneRunEventsDropsOnlyExpiredRuns(t *testing.T) {
	db := openTestDB(t)
	runs := db.Runs()
	runs.Insert("r_old", nil, "m")
	runs.Finish("r_old", "completed", 1, 1, 1, 0, nil, nil)
	runs.Insert("r_just_done", nil, "m")
	runs.Finish("r_just_done", "completed", 1, 1, 1, 0, nil, nil)
	runs.Insert("r_live", nil, "m")

	// r_old xong tu 2 gio truoc; r_just_done xong vua xay ra.
	old := TimeText(time.Now().UTC().Add(-2 * time.Hour))
	if _, err := db.SQL().Exec("UPDATE runs SET finished_at = ? WHERE id = ?", old, "r_old"); err != nil {
		t.Fatalf("dat finished_at cu: %v", err)
	}
	for _, runID := range []string{"r_old", "r_just_done", "r_live"} {
		if err := runs.AppendEvent(runID, 1, "run.started", `null`, TimeText(time.Now().UTC())); err != nil {
			t.Fatalf("append %s: %v", runID, err)
		}
	}

	cutoff := TimeText(time.Now().UTC().Add(-30 * time.Minute))
	if removed := runs.PruneRunEvents(cutoff); removed != 1 {
		t.Fatalf("phai xoa 1 su kien cua luot qua han, xoa %d", removed)
	}
	if got := runs.EventsSince("r_old", 0); len(got) != 0 {
		t.Fatalf("su kien cua luot qua han phai bi xoa: %+v", got)
	}
	for _, runID := range []string{"r_just_done", "r_live"} {
		if got := runs.EventsSince(runID, 0); len(got) != 1 {
			t.Fatalf("su kien cua %s phai con: %+v", runID, got)
		}
	}
}
