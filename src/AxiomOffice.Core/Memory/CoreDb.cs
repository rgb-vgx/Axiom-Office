using AxiomOffice.Core.Logging;
using Microsoft.Data.Sqlite;

namespace AxiomOffice.Core.Memory;

// SQLite cua Core: hoi thoai, tin nhan, luot chay, audit tool call (giai doan 1) va memory dai han
// (giai doan 3). Migration danh so bang PRAGMA user_version (New_arch.md muc 8.5.1, 8.5.2).
public sealed class CoreDb(string path)
{
    public const int SchemaVersion = 1;

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
