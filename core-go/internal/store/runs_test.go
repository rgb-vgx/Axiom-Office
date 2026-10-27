package store

import (
	"path/filepath"
	"testing"
)

func TestMarkInterruptedOnlyTouchesRunningRuns(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	runs := db.Runs()
	runs.Insert("r_live", nil, "m")
	runs.Insert("r_done", nil, "m")
	runs.Finish("r_done", "completed", 2, 10, 5, 0, nil, nil)

	if got := runs.MarkInterrupted(); got != 1 {
		t.Fatalf("phai doi dung 1 luot dang chay, doi %d", got)
	}
	live := runs.Get("r_live")
	if live.Status != StatusInterrupted || live.FinishedAt == nil || live.Error == nil {
		t.Fatalf("luot dang chay phai thanh interrupted kem gio ket thuc va loi: %+v", live)
	}
	if done := runs.Get("r_done"); done.Status != "completed" || done.Error != nil {
		t.Fatalf("luot da xong khong duoc dong vao: %+v", done)
	}
	if got := runs.MarkInterrupted(); got != 0 {
		t.Fatalf("goi lan hai khong con luot nao, doi %d", got)
	}
}
