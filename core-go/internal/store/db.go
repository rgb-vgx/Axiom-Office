// Package store la SQLite cua Core (ban Go cua CoreDb.cs). SCHEMA PHAI GIONG HET ban .NET (cung
// PRAGMA user_version) de hai ban doc chung core.db cua nguoi dung khi chuyen qua lai.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // SQLite thuan Go: build cheo khong can cgo

	"axiomoffice/core/internal/corelog"
)

const SchemaVersion = 2

type DB struct {
	Path string
	sql  *sql.DB
}

// Open mo (tao neu chua co) core.db voi WAL + khoa ngoai + cho khoa 5s, giong CoreDb.Open.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(4)
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &DB{Path: path, sql: conn}, nil
}

func (d *DB) SQL() *sql.DB { return d.sql }

func (d *DB) Close() error { return d.sql.Close() }

// Migrate chay migration theo user_version. Loi -> Core bao memory "unavailable" (khong dung).
func (d *DB) Migrate() error {
	var version int
	if err := d.sql.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version >= SchemaVersion {
		return nil
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if version < 1 {
		if _, err := tx.Exec(schemaV1); err != nil {
			return fmt.Errorf("schema v1: %w", err)
		}
	}
	if version < 2 {
		if _, err := tx.Exec(schemaV2); err != nil {
			return fmt.Errorf("schema v2: %w", err)
		}
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	corelog.Info("CoreDb migrated to schema %d: %s", SchemaVersion, d.Path)
	return nil
}

// Chep nguyen van tu CoreDb.cs (schema v1: hoi thoai, tin nhan, luot chay, audit).
const schemaV1 = `
CREATE TABLE conversations (
  id TEXT PRIMARY KEY, created_at TEXT, updated_at TEXT,
  app TEXT, family TEXT, document_key TEXT, document_name TEXT, title TEXT, summary TEXT);
CREATE INDEX ix_conversations_document ON conversations(document_key, updated_at DESC);

CREATE TABLE messages (
  id INTEGER PRIMARY KEY, conversation_id TEXT REFERENCES conversations(id) ON DELETE CASCADE,
  seq INTEGER, role TEXT, content TEXT, created_at TEXT);
CREATE INDEX ix_messages_conversation ON messages(conversation_id, seq);

CREATE TABLE runs (
  id TEXT PRIMARY KEY, conversation_id TEXT, status TEXT, started_at TEXT, finished_at TEXT,
  model TEXT, rounds INTEGER, input_tokens INTEGER, output_tokens INTEGER, error TEXT,
  memory_status TEXT);
CREATE INDEX ix_runs_conversation ON runs(conversation_id, started_at DESC);

CREATE TABLE tool_calls (
  id INTEGER PRIMARY KEY, run_id TEXT, seq INTEGER, tool TEXT, action TEXT,
  params_json TEXT, ok INTEGER, error TEXT, ms INTEGER, created_at TEXT);
CREATE INDEX ix_tool_calls_run ON tool_calls(run_id, seq);
`

// Schema v2: memory dai han (memories_fts luu text DA CHUAN HOA bo dau, dong bo trong code cung transaction).
const schemaV2 = `
CREATE TABLE memories (
  id TEXT PRIMARY KEY,
  scope TEXT NOT NULL,
  scope_key TEXT,
  text TEXT NOT NULL,
  hash TEXT NOT NULL,
  category TEXT,
  entities_json TEXT,
  expires_at TEXT,
  source TEXT NOT NULL,
  confidence REAL,
  pinned INTEGER DEFAULT 0,
  hits INTEGER DEFAULT 0, last_used_at TEXT,
  created_at TEXT, updated_at TEXT, deleted_at TEXT,
  created_run_id TEXT);
CREATE INDEX ix_memories_scope ON memories(scope, scope_key) WHERE deleted_at IS NULL;
CREATE INDEX ix_memories_hash ON memories(scope, scope_key, hash) WHERE deleted_at IS NULL;
CREATE VIRTUAL TABLE memories_fts USING fts5(text, tokenize = 'unicode61 remove_diacritics 2');

CREATE TABLE memory_links (
  memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
  linked_memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
  created_at TEXT, PRIMARY KEY (memory_id, linked_memory_id));

CREATE TABLE memory_history (
  id INTEGER PRIMARY KEY, memory_id TEXT, event TEXT,
  old_text TEXT, new_text TEXT, actor TEXT,
  run_id TEXT, reason TEXT, created_at TEXT);
CREATE INDEX ix_memory_history_memory ON memory_history(memory_id, id);

CREATE TABLE memory_embeddings (
  memory_id TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
  model TEXT, dim INTEGER, vector BLOB,
  text_hash TEXT, created_at TEXT);
`

// Stores gom kho du lieu + trang thai cho /health ("on" | "off" | "unavailable").
type Stores struct {
	DB        *DB
	Convs     *Conversations
	Runs      *Runs
	Available bool
	Error     string
}

func OpenStores(path string) *Stores {
	stores := &Stores{}
	db, err := Open(path)
	if err == nil {
		err = db.Migrate()
	}
	if err != nil {
		stores.Error = err.Error()
		corelog.Error("memory unavailable: %s: %v", path, err)
		if db != nil {
			stores.DB = db
		}
		return stores
	}
	stores.DB = db
	stores.Convs = db.Conversations()
	stores.Runs = db.Runs()
	stores.Available = true
	return stores
}

func (s *Stores) Status(memoryEnabled bool) string {
	if !memoryEnabled {
		return "off"
	}
	if s.Available {
		return "on"
	}
	return "unavailable"
}
