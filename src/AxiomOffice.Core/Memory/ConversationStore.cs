using System.Globalization;
using Microsoft.Data.Sqlite;

namespace AxiomOffice.Core.Memory;

public sealed record ConversationRow(
    string Id,
    string? DocumentKey,
    string? DocumentName,
    string App,
    string Family,
    string? Title,
    string? Summary,
    string CreatedAt,
    string UpdatedAt);

public sealed record MessageRow(long Id, int Seq, string Role, string Content, string CreatedAt);

// Hoi thoai va tin nhan (New_arch.md muc 8.5.9): tin nhan user + cau tra loi cuoi + tom tat tool.
public sealed class ConversationStore(CoreDb db)
{
    public string Create(string app, string family, string? documentKey, string? documentName, string? title)
    {
        string id = "c_" + Guid.NewGuid().ToString("N");
        string now = Now();
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = """
            INSERT INTO conversations (id, created_at, updated_at, app, family, document_key, document_name, title, summary)
            VALUES ($id, $now, $now, $app, $family, $key, $name, $title, NULL)
            """;
        command.Parameters.AddWithValue("$id", id);
        command.Parameters.AddWithValue("$now", now);
        command.Parameters.AddWithValue("$app", app);
        command.Parameters.AddWithValue("$family", family);
        command.Parameters.AddWithValue("$key", (object?)documentKey ?? DBNull.Value);
        command.Parameters.AddWithValue("$name", (object?)documentName ?? DBNull.Value);
        command.Parameters.AddWithValue("$title", (object?)title ?? DBNull.Value);
        command.ExecuteNonQuery();
        return id;
    }

    public int AppendMessage(string conversationId, string role, string content)
    {
        using SqliteConnection connection = db.Open();
        using SqliteTransaction transaction = connection.BeginTransaction();
        int seq;
        using (SqliteCommand next = connection.CreateCommand())
        {
            next.Transaction = transaction;
            next.CommandText = "SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE conversation_id = $id";
            next.Parameters.AddWithValue("$id", conversationId);
            seq = Convert.ToInt32(next.ExecuteScalar() ?? 1);
        }

        using (SqliteCommand insert = connection.CreateCommand())
        {
            insert.Transaction = transaction;
            insert.CommandText = """
                INSERT INTO messages (conversation_id, seq, role, content, created_at)
                VALUES ($id, $seq, $role, $content, $now)
                """;
            insert.Parameters.AddWithValue("$id", conversationId);
            insert.Parameters.AddWithValue("$seq", seq);
            insert.Parameters.AddWithValue("$role", role);
            insert.Parameters.AddWithValue("$content", content);
            insert.Parameters.AddWithValue("$now", Now());
            insert.ExecuteNonQuery();
        }

        using (SqliteCommand touch = connection.CreateCommand())
        {
            touch.Transaction = transaction;
            touch.CommandText = "UPDATE conversations SET updated_at = $now WHERE id = $id";
            touch.Parameters.AddWithValue("$now", Now());
            touch.Parameters.AddWithValue("$id", conversationId);
            touch.ExecuteNonQuery();
        }

        transaction.Commit();
        return seq;
    }

    public IReadOnlyList<MessageRow> Messages(string conversationId, int limit = 0)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        // limit > 0: lay cac tin MOI NHAT roi dao lai de doc theo thu tu thoi gian.
        command.CommandText = limit > 0
            ? "SELECT id, seq, role, content, created_at FROM (SELECT * FROM messages WHERE conversation_id = $id ORDER BY seq DESC LIMIT $limit) ORDER BY seq"
            : "SELECT id, seq, role, content, created_at FROM messages WHERE conversation_id = $id ORDER BY seq";
        command.Parameters.AddWithValue("$id", conversationId);
        if (limit > 0)
        {
            command.Parameters.AddWithValue("$limit", limit);
        }

        var rows = new List<MessageRow>();
        using SqliteDataReader reader = command.ExecuteReader();
        while (reader.Read())
        {
            rows.Add(new MessageRow(
                reader.GetInt64(0),
                reader.GetInt32(1),
                reader.GetString(2),
                reader.GetString(3),
                reader.GetString(4)));
        }

        return rows;
    }

    public ConversationRow? Get(string id)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT id, document_key, document_name, app, family, title, summary, created_at, updated_at FROM conversations WHERE id = $id";
        command.Parameters.AddWithValue("$id", id);
        using SqliteDataReader reader = command.ExecuteReader();
        return reader.Read() ? Read(reader) : null;
    }

    public IReadOnlyList<ConversationRow> List(string? documentKey = null, int limit = 20)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = documentKey == null
            ? "SELECT id, document_key, document_name, app, family, title, summary, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT $limit"
            : "SELECT id, document_key, document_name, app, family, title, summary, created_at, updated_at FROM conversations WHERE document_key = $key ORDER BY updated_at DESC LIMIT $limit";
        command.Parameters.AddWithValue("$limit", limit);
        if (documentKey != null)
        {
            command.Parameters.AddWithValue("$key", documentKey);
        }

        var rows = new List<ConversationRow>();
        using SqliteDataReader reader = command.ExecuteReader();
        while (reader.Read())
        {
            rows.Add(Read(reader));
        }

        return rows;
    }

    // Hoi thoai gan nhat cua mot tai lieu: pane dung de "lam tiep" (muc 9).
    public ConversationRow? LatestForDocument(string documentKey)
    {
        IReadOnlyList<ConversationRow> rows = List(documentKey, 1);
        return rows.Count > 0 ? rows[0] : null;
    }

    public bool Rename(string id, string? title = null, string? summary = null)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = """
            UPDATE conversations
            SET title = COALESCE($title, title), summary = COALESCE($summary, summary), updated_at = $now
            WHERE id = $id
            """;
        command.Parameters.AddWithValue("$id", id);
        command.Parameters.AddWithValue("$title", (object?)title ?? DBNull.Value);
        command.Parameters.AddWithValue("$summary", (object?)summary ?? DBNull.Value);
        command.Parameters.AddWithValue("$now", Now());
        return command.ExecuteNonQuery() > 0;
    }

    public bool Delete(string id)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "DELETE FROM conversations WHERE id = $id";
        command.Parameters.AddWithValue("$id", id);
        return command.ExecuteNonQuery() > 0;
    }

    public static string Now()
    {
        return DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ", CultureInfo.InvariantCulture);
    }

    private static ConversationRow Read(SqliteDataReader reader)
    {
        return new ConversationRow(
            reader.GetString(0),
            reader.IsDBNull(1) ? null : reader.GetString(1),
            reader.IsDBNull(2) ? null : reader.GetString(2),
            reader.GetString(3),
            reader.GetString(4),
            reader.IsDBNull(5) ? null : reader.GetString(5),
            reader.IsDBNull(6) ? null : reader.GetString(6),
            reader.GetString(7),
            reader.GetString(8));
    }
}
