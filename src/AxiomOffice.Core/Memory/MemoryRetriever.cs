namespace AxiomOffice.Core.Memory;

public sealed record MemoryHit(MemoryItem Item, double Score, double Keyword, double Semantic, double Entity);

// Ket qua dua vao prompt: cac dong "- [id ngan] text" + id da dung (tang hits sau run).
public sealed record MemoryContext(IReadOnlyList<string> Lines, IReadOnlyList<string> Ids)
{
    public static readonly MemoryContext Empty = new([], []);
}

// Tim memory lien quan (New_arch.md muc 8.5.7) - cham diem cong don nhu score_and_rank cua mem0 2.2.1.
public sealed class MemoryRetriever(SqliteMemoryStore store)
{
    public const double KeywordWeight = 1.0;
    public const double SemanticWeight = 1.0;
    public const double EntityWeight = 0.5;      // ENTITY_BOOST_WEIGHT cua mem0
    public const double Threshold = 0.1;          // chan tin hieu chinh TRUOC khi cong
    public const int MaxDocumentMemories = 20;
    public const int MaxUserMemories = 8;
    public const int MaxContextChars = 6000;      // ~1.500 token

    // Tham so sigmoid theo do dai truy van (get_bm25_params cua mem0).
    public static (double Midpoint, double Steepness) Bm25Params(int queryTerms)
    {
        return queryTerms switch
        {
            <= 3 => (5, 0.7),
            <= 6 => (7, 0.6),
            <= 9 => (9, 0.5),
            <= 15 => (10, 0.5),
            _ => (12, 0.5),
        };
    }

    public static double NormalizeBm25(double rawScore, int queryTerms)
    {
        (double midpoint, double steepness) = Bm25Params(queryTerms);
        return 1.0 / (1.0 + Math.Exp(-steepness * (rawScore - midpoint)));
    }

    // Boost thuc the giam khi thuc the gan qua nhieu memory (memory_count_weight cua mem0).
    public static double EntityBoost(int memoriesWithEntity)
    {
        int n = Math.Max(1, memoriesWithEntity);
        return EntityWeight * (1.0 / (1.0 + 0.001 * Math.Pow(n - 1, 2)));
    }

    // semantic: cosine theo memory id (null = khong co embedding).
    public IReadOnlyList<MemoryHit> Search(
        string query,
        string? documentKey,
        DateOnly today,
        IReadOnlyDictionary<string, double>? semantic = null,
        string? scope = null,
        int limit = 10)
    {
        IReadOnlyList<MemoryItem> candidates = store.Active(documentKey, today)
            .Where(m => scope == null || m.Scope == scope)
            .ToList();
        return Score(candidates, query, semantic).Take(limit).ToList();
    }

    public IReadOnlyList<MemoryHit> Score(IReadOnlyList<MemoryItem> candidates, string query, IReadOnlyDictionary<string, double>? semantic)
    {
        if (candidates.Count == 0)
        {
            return [];
        }

        IReadOnlyList<string> terms = MemoryText.QueryTerms(query);
        string? fts = MemoryText.FtsQuery(terms);
        IReadOnlyDictionary<long, double> bm25 = fts == null ? new Dictionary<long, double>() : store.KeywordScores(fts);

        string normalizedQuery = " " + MemoryText.Normalize(query) + " ";
        IReadOnlyDictionary<string, int> entityCounts = candidates.Any(c => c.Entities.Count > 0)
            ? store.EntityCounts()
            : new Dictionary<string, int>();
        bool entityEnabled = false;
        var entityScores = new Dictionary<string, double>(StringComparer.Ordinal);
        foreach (MemoryItem item in candidates)
        {
            double best = 0;
            foreach (string entity in item.Entities.Select(MemoryText.Normalize).Where(e => e.Length > 1))
            {
                if (normalizedQuery.Contains(" " + entity + " ", StringComparison.Ordinal))
                {
                    best = Math.Max(best, EntityBoost(entityCounts.GetValueOrDefault(entity, 1)));
                }
            }

            if (best > 0)
            {
                entityEnabled = true;
                entityScores[item.Id] = best;
            }
        }

        bool hasSemantic = semantic is { Count: > 0 };
        double maxPossible = KeywordWeight + (hasSemantic ? SemanticWeight : 0) + (entityEnabled ? EntityWeight : 0);
        var hits = new List<MemoryHit>();
        var termSet = new HashSet<string>(terms, StringComparer.Ordinal);
        foreach (MemoryItem item in candidates)
        {
            double keyword = KeywordScore(item, termSet, bm25, terms.Count);
            double sem = hasSemantic && semantic!.TryGetValue(item.Id, out double cosine) ? Math.Max(0, cosine) : 0;
            double entity = entityScores.GetValueOrDefault(item.Id);

            // Nguong chan tin hieu chinh: co embedding -> cosine; khong -> keyword. Tai lieu hien tai va ghim luon giu.
            double primary = hasSemantic ? sem : keyword;
            bool keep = item.Pinned || item.Scope == MemoryScopes.Document || primary >= Threshold || entity > 0;
            if (!keep)
            {
                continue;
            }

            double combined = (keyword * KeywordWeight + sem * SemanticWeight + entity) / maxPossible;
            hits.Add(new MemoryHit(item, combined, keyword, sem, entity));
        }

        // Bang diem thi uu tien ban MOI hon (ban ghi chuyen doi thang ban cu).
        return hits
            .OrderByDescending(h => Math.Round(h.Score, 6))
            .ThenByDescending(h => h.Item.CreatedAt, StringComparer.Ordinal)
            .ToList();
    }

    public const double RawPerMatchedTerm = 2.5;

    // Diem keyword tho = -bm25 (FTS5) + 2.5 x so tu truy van khop, roi sigmoid theo do dai truy van.
    // Ly do cong so tu khop: IDF cua bm25 trong FTS5 bi kep ~1e-6 khi kho it memory (may moi dung) nen mot
    // memory khop hoan toan cung chi duoc ~0 diem va khong bao gio qua nguong 0.1; thang 2.5/tu gan voi
    // diem rank_bm25 ma mem0 dua vao sigmoid.
    public static double KeywordScore(MemoryItem item, IReadOnlySet<string> terms, IReadOnlyDictionary<long, double> bm25, int termCount)
    {
        if (terms.Count == 0)
        {
            return 0;
        }

        int matched = MemoryText.Normalize(item.Text).Split(' ', StringSplitOptions.RemoveEmptyEntries)
            .Distinct(StringComparer.Ordinal)
            .Count(terms.Contains);
        double raw = (bm25.TryGetValue(item.RowId, out double score) ? -score : 0) + RawPerMatchedTerm * matched;
        return raw <= 0 ? 0 : NormalizeBm25(raw, termCount);
    }

    // Ngu canh cho prompt: moi memory ghim; <= 20 memory tai lieu; <= 8 memory user tren nguong; <= ~1.500 token.
    public MemoryContext BuildContext(string prompt, string? documentKey, DateOnly today, IReadOnlyDictionary<string, double>? semantic = null)
    {
        IReadOnlyList<MemoryItem> candidates = store.Active(documentKey, today);
        if (candidates.Count == 0)
        {
            return MemoryContext.Empty;
        }

        IReadOnlyList<MemoryHit> ranked = Score(candidates, prompt, semantic);
        var chosen = new List<MemoryItem>();
        chosen.AddRange(candidates.Where(c => c.Pinned).OrderByDescending(c => c.CreatedAt, StringComparer.Ordinal));
        chosen.AddRange(ranked.Where(h => !h.Item.Pinned && h.Item.Scope == MemoryScopes.Document).Take(MaxDocumentMemories).Select(h => h.Item));
        chosen.AddRange(ranked
            .Where(h => !h.Item.Pinned && h.Item.Scope == MemoryScopes.User && (h.Keyword >= Threshold || h.Semantic >= Threshold || h.Entity > 0))
            .Take(MaxUserMemories)
            .Select(h => h.Item));

        var lines = new List<string>();
        var ids = new List<string>();
        int used = 0;
        foreach (MemoryItem item in chosen.DistinctBy(i => i.Id))
        {
            string line = "[" + ShortId(item.Id) + "] " + item.Text + (item.Scope == MemoryScopes.Document ? " (tài liệu này)" : "");
            if (used + line.Length > MaxContextChars)
            {
                break;
            }

            used += line.Length;
            lines.Add(line);
            ids.Add(item.Id);
        }

        return new MemoryContext(lines, ids);
    }

    public static string ShortId(string id) => id.Length > 6 ? id[^6..] : id;
}
