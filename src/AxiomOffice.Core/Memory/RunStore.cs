using Microsoft.Data.Sqlite;

namespace AxiomOffice.Core.Memory;

public sealed record RunRow(
    string Id,
    string? ConversationId,
    string Status,
    string StartedAt,
    string? FinishedAt,
    string Model,
    int Rounds,
    int InputTokens,
    int OutputTokens,
    string? Error);

public sealed record ToolCallRow(long Id, int Seq, string Tool, string? Action, int Ok, string? Error, long Ms, string CreatedAt);

// Luot chay va audit tool call (New_arch.md muc 8.5.2, 8.6).
public sealed class RunStore(CoreDb db)
{
    public void Insert(string runId, string? conversationId, string model)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = """
            INSERT INTO runs (id, conversation_id, status, started_at, model, rounds, input_tokens, output_tokens)
            VALUES ($id, $cid, 'running', $now, $model, 0, 0, 0)
            """;
        command.Parameters.AddWithValue("$id", runId);
        command.Parameters.AddWithValue("$cid", (object?)conversationId ?? DBNull.Value);
        command.Parameters.AddWithValue("$now", ConversationStore.Now());
        command.Parameters.AddWithValue("$model", model);
        command.ExecuteNonQuery();
    }

    public void Finish(string runId, string status, int rounds, int inputTokens, int outputTokens, string? error, string? memoryStatus = null)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = """
            UPDATE runs
            SET status = $status, finished_at = $now, rounds = $rounds,
                input_tokens = $in, output_tokens = $out, error = $error, memory_status = COALESCE($memory, memory_status)
            WHERE id = $id
            """;
        command.Parameters.AddWithValue("$id", runId);
        command.Parameters.AddWithValue("$status", status);
        command.Parameters.AddWithValue("$now", ConversationStore.Now());
        command.Parameters.AddWithValue("$rounds", rounds);
        command.Parameters.AddWithValue("$in", inputTokens);
        command.Parameters.AddWithValue("$out", outputTokens);
        command.Parameters.AddWithValue("$error", (object?)error ?? DBNull.Value);
        command.Parameters.AddWithValue("$memory", (object?)memoryStatus ?? DBNull.Value);
        command.ExecuteNonQuery();
    }

    public void AddToolCall(string runId, int seq, string tool, string? action, string? paramsJson, bool ok, string? error, long ms)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = """
            INSERT INTO tool_calls (run_id, seq, tool, action, params_json, ok, error, ms, created_at)
            VALUES ($id, $seq, $tool, $action, $params, $ok, $error, $ms, $now)
            """;
        command.Parameters.AddWithValue("$id", runId);
        command.Parameters.AddWithValue("$seq", seq);
        command.Parameters.AddWithValue("$tool", tool);
        command.Parameters.AddWithValue("$action", (object?)action ?? DBNull.Value);
        // Params rut gon <= 2KB, khong ghi noi dung file (muc 8.6).
        command.Parameters.AddWithValue("$params", (object?)(paramsJson == null ? null : Truncate(paramsJson, 2048)) ?? DBNull.Value);
        command.Parameters.AddWithValue("$ok", ok ? 1 : 0);
        command.Parameters.AddWithValue("$error", (object?)(error == null ? null : Truncate(error, 500)) ?? DBNull.Value);
        command.Parameters.AddWithValue("$ms", ms);
        command.Parameters.AddWithValue("$now", ConversationStore.Now());
        command.ExecuteNonQuery();
    }

    public RunRow? Get(string runId)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT id, conversation_id, status, started_at, finished_at, model, rounds, input_tokens, output_tokens, error FROM runs WHERE id = $id";
        command.Parameters.AddWithValue("$id", runId);
        using SqliteDataReader reader = command.ExecuteReader();
        if (!reader.Read())
        {
            return null;
        }

        return new RunRow(
            reader.GetString(0),
            reader.IsDBNull(1) ? null : reader.GetString(1),
            reader.GetString(2),
            reader.GetString(3),
            reader.IsDBNull(4) ? null : reader.GetString(4),
            reader.GetString(5),
            reader.GetInt32(6),
            reader.GetInt32(7),
            reader.GetInt32(8),
            reader.IsDBNull(9) ? null : reader.GetString(9));
    }

    public IReadOnlyList<ToolCallRow> ToolCalls(string runId)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT id, seq, tool, action, ok, error, ms, created_at FROM tool_calls WHERE run_id = $id ORDER BY seq, id";
        command.Parameters.AddWithValue("$id", runId);
        var rows = new List<ToolCallRow>();
        using SqliteDataReader reader = command.ExecuteReader();
        while (reader.Read())
        {
            rows.Add(new ToolCallRow(
                reader.GetInt64(0),
                reader.GetInt32(1),
                reader.GetString(2),
                reader.IsDBNull(3) ? null : reader.GetString(3),
                reader.GetInt32(4),
                reader.IsDBNull(5) ? null : reader.GetString(5),
                reader.GetInt64(6),
                reader.GetString(7)));
        }

        return rows;
    }

    private static string Truncate(string value, int max)
    {
        return value.Length <= max ? value : value[..max] + "...";
    }
}
