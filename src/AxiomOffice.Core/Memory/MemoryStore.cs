using System.Globalization;
using System.Text.Json;
using Microsoft.Data.Sqlite;

namespace AxiomOffice.Core.Memory;

// Kieu du lieu memory dai han (New_arch.md muc 8.5.2, 8.5.3).
public static class MemoryScopes
{
    public const string User = "user";
    public const string Document = "document";
    public const string Skill = "skill";

    public static bool IsValid(string? scope) => scope is User or Document or Skill;
}

public static class MemoryActors
{
    public const string User = "user";       // form / API
    public const string Agent = "agent";     // tool remember
    public const string Extract = "extract"; // tu trich xuat sau run
}

public sealed record NewMemory(
    string Scope,
    string? ScopeKey,
    string Text,
    string? Category = null,
    IReadOnlyList<string>? Entities = null,
    IReadOnlyList<string>? LinkedIds = null,
    string? ExpiresAt = null,
    double? Confidence = null);

public sealed record MemoryItem(
    string Id,
    long RowId,
    string Scope,
    string? ScopeKey,
    string Text,
    string? Category,
    IReadOnlyList<string> Entities,
    string? ExpiresAt,
    string Source,
    double? Confidence,
    bool Pinned,
    int Hits,
    string? LastUsedAt,
    string CreatedAt,
    string UpdatedAt,
    string? DeletedAt,
    string? CreatedRunId,
    IReadOnlyList<string> Links);

// Event: ADD | DUPLICATE | SKIPPED (kem ly do).
public sealed record MemoryOpResult(string Event, string Text, MemoryItem? Item, string? Reason = null);

public sealed record MemoryHistoryEntry(long Id, string MemoryId, string Event, string? OldText, string? NewText, string Actor, string? RunId, string? Reason, string CreatedAt);

// Kho memory SQLite (muc 8.5.3). Model chi ADD (qua AddBatch); sua/xoa/ghim/khoi phuc chi do nguoi dung.
public sealed class SqliteMemoryStore(CoreDb db)
{
    public const double NearDuplicateCosine = 0.96;
    public const int SoftDeleteRetentionDays = 30;

    private const string Columns =
        "rowid, id, scope, scope_key, text, category, entities_json, expires_at, source, confidence, pinned, hits, "
        + "last_used_at, created_at, updated_at, deleted_at, created_run_id";

    public static string NewId() => "m_" + Guid.CreateVersion7().ToString("N");

    public static string Now() => DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ", CultureInfo.InvariantCulture);

    // Ghi mot lo fact trong MOT transaction (muc 8.5.6): bo trung hash trong lo va voi memory cung (scope,
    // scope_key); gan-trung bang cosine >= 0.96 neu co vector; link chi toi memory ton tai; ghi history ADD.
    public IReadOnlyList<MemoryOpResult> AddBatch(
        IEnumerable<NewMemory> items,
        string actor,
        string? runId,
        IReadOnlyDictionary<NewMemory, float[]>? vectors = null,
        string? embeddingModel = null)
    {
        var results = new List<MemoryOpResult>();
        using SqliteConnection connection = db.Open();
        using SqliteTransaction transaction = connection.BeginTransaction();
        var batchHashes = new HashSet<string>(StringComparer.Ordinal);
        foreach (NewMemory raw in items)
        {
            string text = (raw.Text ?? "").Trim();
            if (text.Length == 0 || text.Length > MemoryText.MaxFactLength)
            {
                results.Add(new MemoryOpResult("SKIPPED", text, null, text.Length == 0 ? "empty" : "too long"));
                continue;
            }

            if (!MemoryScopes.IsValid(raw.Scope))
            {
                results.Add(new MemoryOpResult("SKIPPED", text, null, "invalid scope"));
                continue;
            }

            if (MemoryText.IsSensitive(text))
            {
                results.Add(new MemoryOpResult("SKIPPED", text, null, "sensitive"));
                continue;
            }

            string? scopeKey = raw.Scope == MemoryScopes.User ? null : raw.ScopeKey;
            string hash = MemoryText.Hash(text);
            if (!batchHashes.Add(raw.Scope + "|" + scopeKey + "|" + hash))
            {
                results.Add(new MemoryOpResult("DUPLICATE", text, null, "duplicate in batch"));
                continue;
            }

            MemoryItem? existing = FindByHash(connection, transaction, raw.Scope, scopeKey, hash);
            float[]? vector = vectors != null && vectors.TryGetValue(raw, out float[]? v) ? v : null;
            existing ??= vector != null ? FindNearDuplicate(connection, transaction, raw.Scope, scopeKey, vector) : null;
            if (existing != null)
            {
                Execute(connection, transaction, "UPDATE memories SET hits = hits + 1, last_used_at = $now WHERE id = $id",
                    ("$now", Now()), ("$id", existing.Id));
                results.Add(new MemoryOpResult("DUPLICATE", text, existing, "same as " + existing.Id));
                continue;
            }

            string id = NewId();
            string now = Now();
            Execute(connection, transaction, """
                INSERT INTO memories (id, scope, scope_key, text, hash, category, entities_json, expires_at, source,
                  confidence, pinned, hits, created_at, updated_at, created_run_id)
                VALUES ($id, $scope, $key, $text, $hash, $category, $entities, $expires, $source, $confidence, 0, 0, $now, $now, $run)
                """,
                ("$id", id), ("$scope", raw.Scope), ("$key", scopeKey), ("$text", text), ("$hash", hash),
                ("$category", raw.Category), ("$entities", JsonSerializer.Serialize(CleanEntities(raw.Entities))),
                ("$expires", MemoryText.ValidDate(raw.ExpiresAt)), ("$source", actor),
                ("$confidence", actor == MemoryActors.User ? 1.0 : raw.Confidence), ("$now", now), ("$run", runId));
            long rowId = LastRowId(connection, transaction);
            IndexText(connection, transaction, rowId, text);

            foreach (string linked in (raw.LinkedIds ?? []).Distinct(StringComparer.Ordinal))
            {
                if (Exists(connection, transaction, linked))
                {
                    Execute(connection, transaction,
                        "INSERT OR IGNORE INTO memory_links (memory_id, linked_memory_id, created_at) VALUES ($id, $linked, $now)",
                        ("$id", id), ("$linked", linked), ("$now", now));
                }
            }

            AddHistory(connection, transaction, id, "ADD", null, text, actor, runId, null);
            if (vector != null && embeddingModel != null)
            {
                SaveVector(connection, transaction, id, embeddingModel, vector, hash);
            }

            results.Add(new MemoryOpResult("ADD", text, Get(connection, transaction, id)));
        }

        transaction.Commit();
        return results;
    }

    public MemoryItem? Get(string id)
    {
        using SqliteConnection connection = db.Open();
        return Get(connection, null, id);
    }

    public IReadOnlyList<MemoryItem> List(string? scope = null, string? scopeKey = null, bool includeDeleted = false, string? runId = null, int limit = 500)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        var where = new List<string>();
        if (!includeDeleted)
        {
            where.Add("deleted_at IS NULL");
        }

        if (scope != null)
        {
            where.Add("scope = $scope");
            command.Parameters.AddWithValue("$scope", scope);
        }

        if (scopeKey != null)
        {
            where.Add("scope_key = $key");
            command.Parameters.AddWithValue("$key", scopeKey);
        }

        if (runId != null)
        {
            where.Add("created_run_id = $run");
            command.Parameters.AddWithValue("$run", runId);
        }

        command.CommandText = "SELECT " + Columns + " FROM memories"
            + (where.Count > 0 ? " WHERE " + string.Join(" AND ", where) : "")
            + " ORDER BY pinned DESC, created_at DESC LIMIT $limit";
        command.Parameters.AddWithValue("$limit", limit);
        return ReadItems(connection, null, command);
    }

    // Ung vien cho ngu canh: chua xoa, chua het han (expires_at > hom qua), thuoc user + document hien tai.
    public IReadOnlyList<MemoryItem> Active(string? documentKey, DateOnly today)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT " + Columns + " FROM memories WHERE deleted_at IS NULL"
            + " AND (expires_at IS NULL OR expires_at > $yesterday)"
            + " AND (scope = 'user' OR (scope = 'document' AND scope_key = $doc))";
        command.Parameters.AddWithValue("$yesterday", today.AddDays(-1).ToString("yyyy-MM-dd", CultureInfo.InvariantCulture));
        command.Parameters.AddWithValue("$doc", (object?)documentKey ?? DBNull.Value);
        return ReadItems(connection, null, command);
    }

    // bm25 cua FTS5 cho cac rowid khop truy van (diem cang THAP cang khop).
    public IReadOnlyDictionary<long, double> KeywordScores(string ftsQuery)
    {
        var scores = new Dictionary<long, double>();
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT rowid, bm25(memories_fts) FROM memories_fts WHERE memories_fts MATCH $q";
        command.Parameters.AddWithValue("$q", ftsQuery);
        using SqliteDataReader reader = command.ExecuteReader();
        while (reader.Read())
        {
            scores[reader.GetInt64(0)] = reader.GetDouble(1);
        }

        return scores;
    }

    public IReadOnlyDictionary<string, float[]> Vectors(string model)
    {
        var vectors = new Dictionary<string, float[]>(StringComparer.Ordinal);
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT memory_id, vector FROM memory_embeddings WHERE model = $model";
        command.Parameters.AddWithValue("$model", model);
        using SqliteDataReader reader = command.ExecuteReader();
        while (reader.Read())
        {
            vectors[reader.GetString(0)] = FromBlob((byte[])reader.GetValue(1));
        }

        return vectors;
    }

    public void SaveVector(string id, string model, float[] vector, string textHash)
    {
        using SqliteConnection connection = db.Open();
        SaveVector(connection, null, id, model, vector, textHash);
    }

    // Memory chua co vector cua model hien tai (tinh bu o nen khi doi EmbeddingModel).
    public IReadOnlyList<MemoryItem> MissingVectors(string model, int limit)
    {
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT " + Columns + " FROM memories m WHERE deleted_at IS NULL AND NOT EXISTS "
            + "(SELECT 1 FROM memory_embeddings e WHERE e.memory_id = m.id AND e.model = $model AND e.text_hash = m.hash) LIMIT $limit";
        command.Parameters.AddWithValue("$model", model);
        command.Parameters.AddWithValue("$limit", limit);
        return ReadItems(connection, null, command);
    }

    // So memory dang dung gan voi moi thuc the (cho entity boost giam dan).
    public IReadOnlyDictionary<string, int> EntityCounts()
    {
        var counts = new Dictionary<string, int>(StringComparer.Ordinal);
        foreach (MemoryItem item in List(limit: 100_000))
        {
            foreach (string entity in item.Entities.Select(MemoryText.Normalize).Where(e => e.Length > 0).Distinct())
            {
                counts[entity] = counts.GetValueOrDefault(entity) + 1;
            }
        }

        return counts;
    }

    public void TouchHits(IEnumerable<string> ids)
    {
        string[] list = ids.Distinct(StringComparer.Ordinal).ToArray();
        if (list.Length == 0)
        {
            return;
        }

        using SqliteConnection connection = db.Open();
        using SqliteTransaction transaction = connection.BeginTransaction();
        string now = Now();
        foreach (string id in list)
        {
            Execute(connection, transaction, "UPDATE memories SET hits = hits + 1, last_used_at = $now WHERE id = $id", ("$now", now), ("$id", id));
        }

        transaction.Commit();
    }

    // ---- Chi nguoi dung (form / API) ----

    public MemoryItem? Update(string id, string? newText, string? category, string? expiresAt, bool clearExpires, bool? pinned, string? reason)
    {
        using SqliteConnection connection = db.Open();
        using SqliteTransaction transaction = connection.BeginTransaction();
        MemoryItem? item = Get(connection, transaction, id);
        if (item == null || item.DeletedAt != null)
        {
            return null;
        }

        string now = Now();
        if (newText != null)
        {
            string text = newText.Trim();
            if (text.Length == 0 || text.Length > MemoryText.MaxFactLength)
            {
                throw new ArgumentException($"text must be 1-{MemoryText.MaxFactLength} characters");
            }

            if (text != item.Text)
            {
                Execute(connection, transaction, "UPDATE memories SET text = $text, hash = $hash, updated_at = $now WHERE id = $id",
                    ("$text", text), ("$hash", MemoryText.Hash(text)), ("$now", now), ("$id", id));
                Execute(connection, transaction, "DELETE FROM memories_fts WHERE rowid = $row", ("$row", item.RowId));
                IndexText(connection, transaction, item.RowId, text);
                Execute(connection, transaction, "DELETE FROM memory_embeddings WHERE memory_id = $id", ("$id", id));
                AddHistory(connection, transaction, id, "UPDATE", item.Text, text, MemoryActors.User, null, reason);
            }
        }

        if (category != null)
        {
            Execute(connection, transaction, "UPDATE memories SET category = $c, updated_at = $now WHERE id = $id", ("$c", category), ("$now", now), ("$id", id));
        }

        if (clearExpires || expiresAt != null)
        {
            string? date = clearExpires ? null : MemoryText.ValidDate(expiresAt) ?? throw new ArgumentException("expiresAt must be YYYY-MM-DD");
            Execute(connection, transaction, "UPDATE memories SET expires_at = $e, updated_at = $now WHERE id = $id", ("$e", date), ("$now", now), ("$id", id));
            AddHistory(connection, transaction, id, "EXPIRES", item.ExpiresAt, date, MemoryActors.User, null, reason);
        }

        if (pinned is bool pin && pin != item.Pinned)
        {
            Execute(connection, transaction, "UPDATE memories SET pinned = $p, updated_at = $now WHERE id = $id", ("$p", pin ? 1 : 0), ("$now", now), ("$id", id));
            AddHistory(connection, transaction, id, pin ? "PIN" : "UNPIN", null, null, MemoryActors.User, null, reason);
        }

        transaction.Commit();
        return Get(id);
    }

    public bool Delete(string id, string? reason)
    {
        using SqliteConnection connection = db.Open();
        using SqliteTransaction transaction = connection.BeginTransaction();
        MemoryItem? item = Get(connection, transaction, id);
        if (item == null || item.DeletedAt != null)
        {
            return false;
        }

        Execute(connection, transaction, "UPDATE memories SET deleted_at = $now, updated_at = $now WHERE id = $id", ("$now", Now()), ("$id", id));
        Execute(connection, transaction, "DELETE FROM memories_fts WHERE rowid = $row", ("$row", item.RowId));
        AddHistory(connection, transaction, id, "DELETE", item.Text, null, MemoryActors.User, null, reason);
        transaction.Commit();
        return true;
    }

    public bool Restore(string id)
    {
        using SqliteConnection connection = db.Open();
        using SqliteTransaction transaction = connection.BeginTransaction();
        MemoryItem? item = Get(connection, transaction, id);
        if (item == null || item.DeletedAt == null)
        {
            return false;
        }

        Execute(connection, transaction, "UPDATE memories SET deleted_at = NULL, updated_at = $now WHERE id = $id", ("$now", Now()), ("$id", id));
        IndexText(connection, transaction, item.RowId, item.Text);
        AddHistory(connection, transaction, id, "RESTORE", null, item.Text, MemoryActors.User, null, null);
        transaction.Commit();
        return true;
    }

    public IReadOnlyList<MemoryHistoryEntry> History(string id)
    {
        var entries = new List<MemoryHistoryEntry>();
        using SqliteConnection connection = db.Open();
        using SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT id, memory_id, event, old_text, new_text, actor, run_id, reason, created_at FROM memory_history WHERE memory_id = $id ORDER BY id";
        command.Parameters.AddWithValue("$id", id);
        using SqliteDataReader reader = command.ExecuteReader();
        while (reader.Read())
        {
            entries.Add(new MemoryHistoryEntry(reader.GetInt64(0), reader.GetString(1), reader.GetString(2),
                reader.IsDBNull(3) ? null : reader.GetString(3), reader.IsDBNull(4) ? null : reader.GetString(4),
                reader.GetString(5), reader.IsDBNull(6) ? null : reader.GetString(6),
                reader.IsDBNull(7) ? null : reader.GetString(7), reader.GetString(8)));
        }

        return entries;
    }

    // Xoa cung (scope null = tat ca). Dung cho "Xoa toan bo ghi nho" va don xoa mem cu.
    public int Purge(string? scope)
    {
        using SqliteConnection connection = db.Open();
        using SqliteTransaction transaction = connection.BeginTransaction();
        string filter = scope == null ? "" : " WHERE scope = $scope";
        (string, object?)[] args = scope == null ? [] : [("$scope", scope)];
        Execute(connection, transaction, "DELETE FROM memories_fts WHERE rowid IN (SELECT rowid FROM memories" + filter + ")", args);
        Execute(connection, transaction, "DELETE FROM memory_history WHERE memory_id IN (SELECT id FROM memories" + filter + ")", args);
        int removed = Execute(connection, transaction, "DELETE FROM memories" + filter, args);
        transaction.Commit();
        return removed;
    }

    public int PurgeSoftDeleted(DateTime utcNow)
    {
        string cutoff = utcNow.AddDays(-SoftDeleteRetentionDays).ToString("yyyy-MM-ddTHH:mm:ss.fffZ", CultureInfo.InvariantCulture);
        using SqliteConnection connection = db.Open();
        using SqliteTransaction transaction = connection.BeginTransaction();
        int removed = Execute(connection, transaction, "DELETE FROM memories WHERE deleted_at IS NOT NULL AND deleted_at < $cutoff", ("$cutoff", cutoff));
        transaction.Commit();
        return removed;
    }

    // ---- noi bo ----

    private static IEnumerable<string> CleanEntities(IReadOnlyList<string>? entities)
    {
        return (entities ?? []).Select(e => e.Trim()).Where(e => e.Length > 0 && e.Length <= 100).Distinct(StringComparer.OrdinalIgnoreCase).Take(10);
    }

    private static void IndexText(SqliteConnection connection, SqliteTransaction? transaction, long rowId, string text)
    {
        Execute(connection, transaction, "INSERT INTO memories_fts (rowid, text) VALUES ($row, $text)", ("$row", rowId), ("$text", MemoryText.Normalize(text)));
    }

    private static MemoryItem? FindByHash(SqliteConnection connection, SqliteTransaction transaction, string scope, string? scopeKey, string hash)
    {
        using SqliteCommand command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = "SELECT " + Columns + " FROM memories WHERE deleted_at IS NULL AND scope = $scope AND scope_key IS $key AND hash = $hash LIMIT 1";
        command.Parameters.AddWithValue("$scope", scope);
        command.Parameters.AddWithValue("$key", (object?)scopeKey ?? DBNull.Value);
        command.Parameters.AddWithValue("$hash", hash);
        return ReadItems(connection, transaction, command).FirstOrDefault();
    }

    private static MemoryItem? FindNearDuplicate(SqliteConnection connection, SqliteTransaction transaction, string scope, string? scopeKey, float[] vector)
    {
        using SqliteCommand command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = "SELECT e.memory_id, e.vector FROM memory_embeddings e JOIN memories m ON m.id = e.memory_id "
            + "WHERE m.deleted_at IS NULL AND m.scope = $scope AND m.scope_key IS $key AND e.dim = $dim";
        command.Parameters.AddWithValue("$scope", scope);
        command.Parameters.AddWithValue("$key", (object?)scopeKey ?? DBNull.Value);
        command.Parameters.AddWithValue("$dim", vector.Length);
        string? best = null;
        using (SqliteDataReader reader = command.ExecuteReader())
        {
            while (reader.Read())
            {
                if (Cosine(vector, FromBlob((byte[])reader.GetValue(1))) >= NearDuplicateCosine)
                {
                    best = reader.GetString(0);
                    break;
                }
            }
        }

        return best == null ? null : Get(connection, transaction, best);
    }

    public static double Cosine(float[] a, float[] b)
    {
        if (a.Length != b.Length || a.Length == 0)
        {
            return 0;
        }

        double dot = 0, na = 0, nb = 0;
        for (int i = 0; i < a.Length; i++)
        {
            dot += a[i] * b[i];
            na += a[i] * a[i];
            nb += b[i] * b[i];
        }

        return na == 0 || nb == 0 ? 0 : dot / (Math.Sqrt(na) * Math.Sqrt(nb));
    }

    private static void SaveVector(SqliteConnection connection, SqliteTransaction? transaction, string id, string model, float[] vector, string textHash)
    {
        Execute(connection, transaction, """
            INSERT INTO memory_embeddings (memory_id, model, dim, vector, text_hash, created_at)
            VALUES ($id, $model, $dim, $vector, $hash, $now)
            ON CONFLICT(memory_id) DO UPDATE SET model = $model, dim = $dim, vector = $vector, text_hash = $hash, created_at = $now
            """,
            ("$id", id), ("$model", model), ("$dim", vector.Length), ("$vector", ToBlob(vector)), ("$hash", textHash), ("$now", Now()));
    }

    private static byte[] ToBlob(float[] vector)
    {
        var bytes = new byte[vector.Length * 4];
        Buffer.BlockCopy(vector, 0, bytes, 0, bytes.Length);
        return bytes;
    }

    private static float[] FromBlob(byte[] bytes)
    {
        var vector = new float[bytes.Length / 4];
        Buffer.BlockCopy(bytes, 0, vector, 0, vector.Length * 4);
        return vector;
    }

    private static bool Exists(SqliteConnection connection, SqliteTransaction transaction, string id)
    {
        using SqliteCommand command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = "SELECT 1 FROM memories WHERE id = $id AND deleted_at IS NULL";
        command.Parameters.AddWithValue("$id", id);
        return command.ExecuteScalar() != null;
    }

    private static MemoryItem? Get(SqliteConnection connection, SqliteTransaction? transaction, string id)
    {
        using SqliteCommand command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = "SELECT " + Columns + " FROM memories WHERE id = $id";
        command.Parameters.AddWithValue("$id", id);
        return ReadItems(connection, transaction, command).FirstOrDefault();
    }

    private static List<MemoryItem> ReadItems(SqliteConnection connection, SqliteTransaction? transaction, SqliteCommand command)
    {
        var rows = new List<MemoryItem>();
        using (SqliteDataReader reader = command.ExecuteReader())
        {
            while (reader.Read())
            {
                string? entitiesJson = reader.IsDBNull(6) ? null : reader.GetString(6);
                IReadOnlyList<string> entities = string.IsNullOrEmpty(entitiesJson) ? [] : JsonSerializer.Deserialize<List<string>>(entitiesJson) ?? [];
                rows.Add(new MemoryItem(
                    reader.GetString(1), reader.GetInt64(0), reader.GetString(2), reader.IsDBNull(3) ? null : reader.GetString(3),
                    reader.GetString(4), reader.IsDBNull(5) ? null : reader.GetString(5), entities,
                    reader.IsDBNull(7) ? null : reader.GetString(7), reader.GetString(8),
                    reader.IsDBNull(9) ? null : reader.GetDouble(9), reader.GetInt64(10) != 0, (int)reader.GetInt64(11),
                    reader.IsDBNull(12) ? null : reader.GetString(12), reader.GetString(13), reader.GetString(14),
                    reader.IsDBNull(15) ? null : reader.GetString(15), reader.IsDBNull(16) ? null : reader.GetString(16), []));
            }
        }

        if (rows.Count == 0)
        {
            return rows;
        }

        // Lien ket (memory moi -> memory cu) cho tung memory.
        for (int i = 0; i < rows.Count; i++)
        {
            using SqliteCommand links = connection.CreateCommand();
            links.Transaction = transaction;
            links.CommandText = "SELECT linked_memory_id FROM memory_links WHERE memory_id = $id";
            links.Parameters.AddWithValue("$id", rows[i].Id);
            var ids = new List<string>();
            using (SqliteDataReader reader = links.ExecuteReader())
            {
                while (reader.Read())
                {
                    ids.Add(reader.GetString(0));
                }
            }

            if (ids.Count > 0)
            {
                rows[i] = rows[i] with { Links = ids };
            }
        }

        return rows;
    }

    private static void AddHistory(SqliteConnection connection, SqliteTransaction transaction, string id, string @event, string? oldText, string? newText, string actor, string? runId, string? reason)
    {
        Execute(connection, transaction, """
            INSERT INTO memory_history (memory_id, event, old_text, new_text, actor, run_id, reason, created_at)
            VALUES ($id, $event, $old, $new, $actor, $run, $reason, $now)
            """,
            ("$id", id), ("$event", @event), ("$old", oldText), ("$new", newText), ("$actor", actor), ("$run", runId), ("$reason", reason), ("$now", Now()));
    }

    private static long LastRowId(SqliteConnection connection, SqliteTransaction transaction)
    {
        using SqliteCommand command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = "SELECT last_insert_rowid()";
        return Convert.ToInt64(command.ExecuteScalar());
    }

    private static int Execute(SqliteConnection connection, SqliteTransaction? transaction, string sql, params (string Name, object? Value)[] parameters)
    {
        using SqliteCommand command = connection.CreateCommand();
        command.Transaction = transaction;
        command.CommandText = sql;
        foreach ((string name, object? value) in parameters)
        {
            command.Parameters.AddWithValue(name, value ?? DBNull.Value);
        }

        return command.ExecuteNonQuery();
    }
}
