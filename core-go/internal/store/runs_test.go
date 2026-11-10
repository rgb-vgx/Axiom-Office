package store

import (
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *DB {
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

func addCall(t *testing.T, db *DB, runID, tool, params string) {
	t.Helper()
	db.Runs().AddToolCall(runID, 1, tool, nil, &params, true, nil, 5, 0)
}

func TestMarkInterruptedOnlyTouchesRunningRuns(t *testing.T) {
	db := testStore(t)
	runs := db.Runs()
	conversation := "c_1"
	runs.Insert("r_live", &conversation, "m")
	runs.Insert("r_done", nil, "m")
	runs.Finish("r_done", "completed", 2, 10, 5, 0, nil, nil)

	if got := runs.MarkInterrupted(); len(got) != 1 || got[0].ID != "r_live" || got[0].ConversationID == nil ||
		*got[0].ConversationID != "c_1" {
		t.Fatalf("phai doi dung 1 luot dang chay kem hoi thoai cua no, doi %+v", got)
	}
	live := runs.Get("r_live")
	if live.Status != StatusInterrupted || live.FinishedAt == nil || live.Error == nil {
		t.Fatalf("luot dang chay phai thanh interrupted kem gio ket thuc va loi: %+v", live)
	}
	if done := runs.Get("r_done"); done.Status != "completed" || done.Error != nil {
		t.Fatalf("luot da xong khong duoc dong vao: %+v", done)
	}
	if got := runs.MarkInterrupted(); len(got) != 0 {
		t.Fatalf("goi lan hai khong con luot nao, doi %+v", got)
	}
}

// P6-04: nhat ky hoat dong loc theo tai lieu dang mo.
func TestAuditFilterByDocument(t *testing.T) {
	db := testStore(t)
	keyA, keyB := `C:\w\a.docx`, `C:\w\b.docx`
	convA := db.Conversations().Create("wps", "writer", &keyA, nil, nil)
	convB := db.Conversations().Create("wps", "writer", &keyB, nil, nil)
	db.Runs().Insert("r_a", &convA, "m")
	db.Runs().Insert("r_b", &convB, "m")
	addCall(t, db, "r_a", "writer.replaceAll", `{"find":"x"}`)
	addCall(t, db, "r_b", "et.writeRange", `{"range":"A1"}`)

	rows := db.Runs().AuditForDocument(&keyA, nil, 50)
	if len(rows) != 1 || rows[0].Tool != "writer.replaceAll" {
		t.Fatalf("loc theo tai lieu A = %+v", rows)
	}
	if all := db.Runs().Audit(nil, 50); len(all) != 2 {
		t.Fatalf("khong loc phai co 2: %+v", all)
	}
}

// P6-02: audit cu hon han bi don, luot moi van giu.
func TestPurgeAuditKeepsRecent(t *testing.T) {
	db := testStore(t)
	conv := db.Conversations().Create("wps", "writer", nil, nil, nil)
	db.Runs().Insert("r_1", &conv, "m")
	addCall(t, db, "r_1", "old.tool", `{"a":1}`)
	addCall(t, db, "r_1", "new.tool", `{"a":2}`)

	old := time.Now().UTC().AddDate(0, 0, -120).Format("2006-01-02T15:04:05.000Z")
	if _, err := db.SQL().Exec("UPDATE tool_calls SET created_at = ? WHERE tool = 'old.tool'", old); err != nil {
		t.Fatal(err)
	}

	before := time.Now().UTC().AddDate(0, 0, -90).Format("2006-01-02T15:04:05.000Z")
	if purged := db.Runs().PurgeAudit(before); purged != 1 {
		t.Fatalf("purged = %d, want 1", purged)
	}
	left := db.Runs().Audit(nil, 50)
	if len(left) != 1 || left[0].Tool != "new.tool" {
		t.Fatalf("con lai = %+v", left)
	}
}
