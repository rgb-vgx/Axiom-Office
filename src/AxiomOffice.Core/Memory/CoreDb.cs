using AxiomOffice.Core.Logging;
using Microsoft.Data.Sqlite;

namespace AxiomOffice.Core.Memory;

// SQLite cua Core: hoi thoai, tin nhan, luot chay, audit tool call (giai doan 1) va memory dai han
// (giai doan 3). Migration danh so bang PRAGMA user_version (New_arch.md muc 8.5.1, 8.5.2).
public sealed class CoreDb(string path)
{
    public const int SchemaVersion = 2;

    public string Path { get; } = path;

    public SqliteConnection Open()
    {
        var connection = new SqliteConnection(new SqliteConnectionStringBuilder
        {
            DataSource = Path,
            Mode = SqliteOpenMode.ReadWriteCreate,
            Cache = SqliteCacheMode.Shared,
            Pooling = true,
        }.ToString());
        connection.Open();
        using SqliteCommand pragma = connection.CreateCommand();
        pragma.CommandText = "PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;";
        pragma.ExecuteNonQuery();
        return connection;
    }

    // Chay migration. Loi -> nem ra ngoai de Core bao memory unavailable (khong lam Core dung).
    public void Migrate()
    {
        Directory.CreateDirectory(System.IO.Path.GetDirectoryName(Path)!);
        using SqliteConnection connection = Open();
        int version = ReadVersion(connection);
        if (version >= SchemaVersion)
        {
            return;
        }

        using SqliteTransaction transaction = connection.BeginTransaction();
        if (version < 1)
        {
            Execute(connection, transaction, """
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
                """);
        }

        if (version < 2)
        {
            // Memory dai han (giai doan 3, muc 8.5.2). memories_fts la bang FTS5 doc lap, rowid = memories.rowid,
            // luu text DA CHUAN HOA (bo dau, d -> d) va dong bo trong C# cung transaction: unicode61
            // remove_diacritics khong bo duoc 'đ' nen "dong" se khong khop "đồng" neu de FTS tu tach.
            Execute(connection, transaction, """
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
                """);
        }

        using (SqliteCommand command = connection.CreateCommand())
        {
            command.Transaction = transaction;
            command.CommandText = "PRAGMA user_version = " + SchemaVersion;
            command.ExecuteNonQuery();
        }

        transaction.Commit();
        CoreLog.Info($"CoreDb migrated to schema {SchemaVersion}: {Path}");
    }

    private static int ReadVersion(SqliteConnection connection)
    {
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "PRAGMA user_version";
        return Convert.ToInt32(command.ExecuteScalar() ?? 0);
    }

    private static void Execute(SqliteConnection connection, SqliteTransaction transaction, string sql)
    {
        using SqliteCommand command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = sql;
        command.ExecuteNonQuery();
    }
}
