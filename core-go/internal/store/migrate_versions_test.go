package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

// TestMigrateFromOlderVersions: nang cap tu v1/v2/v3 len SchemaVersion hien tai phai tao du cac
// doi tuong moi va dat user_version dung. Chi dung DB tam trong t.TempDir, khong dung core.db that.
func TestMigrateFromOlderVersions(t *testing.T) {
	for _, from := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("from_v%d", from), func(t *testing.T) {
			db, err := Open(filepath.Join(t.TempDir(), "core.db"))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })

			// Dung lai DB o dung phien ban cu: v1 luon co, cac bang v2/v3 chi co khi da qua muc do.
			statements := []string{schemaV1}
			if from >= 2 {
				statements = append(statements, schemaV2)
			}
			if from >= 3 {
				statements = append(statements, schemaV3)
			}
			for _, statement := range statements {
				if _, err := db.SQL().Exec(statement); err != nil {
					t.Fatalf("tao schema cu (v%d): %v", from, err)
				}
			}
			if _, err := db.SQL().Exec(fmt.Sprintf("PRAGMA user_version = %d", from)); err != nil {
				t.Fatalf("dat user_version=%d: %v", from, err)
			}

			if err := db.Migrate(); err != nil {
				t.Fatalf("migrate tu v%d: %v", from, err)
			}

			if got := userVersion(t, db.SQL()); got != SchemaVersion {
				t.Fatalf("user_version sau migrate = %d, muon %d", got, SchemaVersion)
			}
			for _, table := range []string{
				"memories", "memories_fts", "web_sources", "web_sources_fts", "run_rounds",
			} {
				if !tableExists(t, db.SQL(), table) {
					t.Errorf("thieu bang %q sau migrate tu v%d", table, from)
				}
			}
			for _, column := range []struct{ table, name string }{
				{"tool_calls", "result_chars"},
				{"runs", "cached_tokens"},
			} {
				if !columnExists(t, db.SQL(), column.table, column.name) {
					t.Errorf("thieu cot %s.%s sau migrate tu v%d", column.table, column.name, from)
				}
			}
		})
	}
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("doc user_version: %v", err)
	}
	return version
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(
		"SELECT count(*) FROM sqlite_master WHERE type IN ('table', 'view') AND name = ?", name,
	).Scan(&count); err != nil {
		t.Fatalf("kiem tra bang %q: %v", name, err)
	}
	return count > 0
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("doc cot bang %q: %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("doc cot bang %q: %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("doc cot bang %q: %v", table, err)
	}
	return false
}
