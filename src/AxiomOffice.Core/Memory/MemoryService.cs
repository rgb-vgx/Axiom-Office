using System.Globalization;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json.Nodes;
using System.Threading.Channels;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Memory;

// Mot lan trich xuat sau run (dua vao hang doi nen).
public sealed record ExtractionJob(string RunId, string ConversationId, string Prompt, string Reply, string? DocumentKey);

// Cua vao memory dai han cho Orchestrator, tool va API (New_arch.md muc 8.5). Doc lai MemoryEnabled /
// MemoryAutoExtract moi lan (doi trong Cai dat co hieu luc ngay). Mot worker nen cho trich xuat, tom tat
// hoi thoai va tinh bu embedding - khong chan pane, khong giu run.
public sealed class MemoryService : IDisposable
{
    private readonly SqliteMemoryStore _store;
    private readonly MemoryRetriever _retriever;
    private readonly ConversationStore _conversations;
    private readonly RunStore _runs;
    private readonly Func<CoreConfig> _config;
    private readonly Func<CoreConfig, ModelClient> _models;
    private readonly EmbeddingClient _embeddings;
    private readonly Channel<Func<CancellationToken, Task>> _queue = Channel.CreateUnbounded<Func<CancellationToken, Task>>();
    private readonly CancellationTokenSource _stop = new();
    private readonly Task _worker;
    private int _pending;

    public MemoryService(
        SqliteMemoryStore store,
        ConversationStore conversations,
        RunStore runs,
        Func<CoreConfig> config,
        Func<CoreConfig, ModelClient> models,
        HttpClient http,
        bool available)
    {
        _store = store;
        _retriever = new MemoryRetriever(store);
        _conversations = conversations;
        _runs = runs;
        _config = config;
        _models = models;
        _embeddings = new EmbeddingClient(http);
        Available = available;
        _worker = Task.Run(WorkAsync);
    }

    public bool Available { get; }

    public SqliteMemoryStore Store => _store;

    public MemoryRetriever Retriever => _retriever;

    public CoreConfig Config => _config();

    public bool Enabled => Available && _config().MemoryEnabled;

    // So viec con trong hang doi nen (test cho doi).
    public int Pending => Volatile.Read(ref _pending);

    public static DateOnly Today() => DateOnly.FromDateTime(DateTime.Now);

    // Ngu canh cho prompt (muc 8.5.7, 8.8 phan 3).
    public async Task<MemoryContext> ContextAsync(string prompt, string? documentKey, CancellationToken cancel)
    {
        if (!Enabled)
        {
            return MemoryContext.Empty;
        }

        try
        {
            IReadOnlyDictionary<string, double>? semantic = await SemanticScoresAsync(prompt, cancel).ConfigureAwait(false);
            return _retriever.BuildContext(prompt, documentKey, Today(), semantic);
        }
        catch (Exception ex)
        {
            CoreLog.Error("memory context failed", ex);
            return MemoryContext.Empty;
        }
    }

    public async Task<IReadOnlyList<MemoryHit>> SearchAsync(string query, string? documentKey, string? scope, int limit, CancellationToken cancel)
    {
        IReadOnlyDictionary<string, double>? semantic = await SemanticScoresAsync(query, cancel).ConfigureAwait(false);
        return _retriever.Search(query, documentKey, Today(), semantic, scope, limit);
    }

    // Ghi fact qua bo chong trung (muc 8.5.6): nguoi dung (API), agent (remember) hoac extract.
    public async Task<IReadOnlyList<MemoryOpResult>> AddAsync(IReadOnlyList<NewMemory> items, string actor, string? runId, CancellationToken cancel)
    {
        CoreConfig config = _config();
        IReadOnlyDictionary<NewMemory, float[]>? vectors = null;
        string? model = EmbeddingModelName(config);
        if (model != null && items.Count > 0)
        {
            float[][]? computed = await _embeddings.EmbedAsync(config, items.Select(i => i.Text).ToList(), cancel).ConfigureAwait(false);
            if (computed != null)
            {
                vectors = items.Zip(computed).ToDictionary(p => p.First, p => p.Second);
            }
        }

        return _store.AddBatch(items, actor, runId, vectors, vectors == null ? null : model);
    }

    public void TouchHits(IReadOnlyList<string> ids)
    {
        if (ids.Count > 0)
        {
            Enqueue(_ =>
            {
                _store.TouchHits(ids);
                return Task.CompletedTask;
            });
        }
    }

    // Sau run completed: xep hang trich xuat (hoac danh dau skipped, khong ton lenh goi LLM).
    public string QueueExtraction(ExtractionJob job)
    {
        CoreConfig config = _config();
        string? reason = !Available ? "memory unavailable"
            : !config.MemoryEnabled ? "memory disabled"
            : !config.MemoryAutoExtract ? "auto extract disabled"
            : MemoryExtractor.ShouldSkip(job.Prompt, out string skip) ? skip
            : null;
        if (reason != null)
        {
            _runs.SetMemoryStatus(job.RunId, "skipped");
            CoreLog.Info($"memory extract skipped for {job.RunId}: {reason}");
            return "skipped";
        }

        _runs.SetMemoryStatus(job.RunId, "queued");
        Enqueue(cancel => ExtractAsync(job, cancel));
        return "queued";
    }

    public void Enqueue(Func<CancellationToken, Task> work)
    {
        Interlocked.Increment(ref _pending);
        if (!_queue.Writer.TryWrite(work))
        {
            Interlocked.Decrement(ref _pending);
        }
    }

    private async Task ExtractAsync(ExtractionJob job, CancellationToken cancel)
    {
        try
        {
            CoreConfig config = _config();
            IReadOnlyList<(string, string)> recent = _conversations.Messages(job.ConversationId, limit: 12)
                .Where(m => m.Role is "user" or "assistant")
                .Select(m => (m.Role, m.Content))
                .ToList();
            // Lien quan theo tu khoa, lap them bang memory gan nhat cung pham vi: "toi da len pho giam doc" khong
            // trung tu nao voi "truong phong Ke toan" nhung extractor can thay ban cu de ghi su chuyen doi + link.
            var related = _retriever.Search(job.Prompt, job.DocumentKey, Today(), null, null, MemoryExtractor.MaxRelatedMemories)
                .Select(h => h.Item)
                .ToList();
            foreach (MemoryItem recentItem in _store.Active(job.DocumentKey, Today()).OrderByDescending(m => m.CreatedAt, StringComparer.Ordinal))
            {
                if (related.Count >= MemoryExtractor.MaxRelatedMemories)
                {
                    break;
                }

                if (related.All(r => r.Id != recentItem.Id))
                {
                    related.Add(recentItem);
                }
            }
            var input = new ExtractionInput(job.Prompt, job.Reply, recent, related, Today(), job.DocumentKey);

            ModelClient model = _models(config);
            (IReadOnlyList<NewMemory>? facts, string? error) = await MemoryExtractor.ExtractAsync(model, input, cancel).ConfigureAwait(false);
            if (facts == null)
            {
                _runs.SetMemoryStatus(job.RunId, "failed");
                CoreLog.Info($"memory extract failed for {job.RunId}: {error}");
                return;
            }

            IReadOnlyList<MemoryOpResult> results = await AddAsync(facts, MemoryActors.Extract, job.RunId, cancel).ConfigureAwait(false);
            _runs.SetMemoryStatus(job.RunId, "done");
            CoreLog.Info($"memory extract {job.RunId}: {results.Count(r => r.Event == "ADD")} added, "
                + $"{results.Count(r => r.Event == "DUPLICATE")} duplicate, {results.Count(r => r.Event == "SKIPPED")} skipped");
        }
        catch (Exception ex)
        {
            _runs.SetMemoryStatus(job.RunId, "failed");
            CoreLog.Error("memory extract crashed for " + job.RunId, ex);
        }
    }

    // Tinh bu embedding (doi EmbeddingModel, memory them khi endpoint loi) - chay nen.
    public void QueueEmbeddingBackfill()
    {
        Enqueue(async cancel =>
        {
            CoreConfig config = _config();
            string? model = EmbeddingModelName(config);
            if (model == null || !Available)
            {
                return;
            }

            IReadOnlyList<MemoryItem> missing = _store.MissingVectors(model, 64);
            if (missing.Count == 0)
            {
                return;
            }

            float[][]? vectors = await _embeddings.EmbedAsync(config, missing.Select(m => m.Text).ToList(), cancel).ConfigureAwait(false);
            if (vectors == null)
            {
                return;
            }

            for (int i = 0; i < missing.Count; i++)
            {
                _store.SaveVector(missing[i].Id, model, vectors[i], MemoryText.Hash(missing[i].Text));
            }
        });
    }

    private async Task<IReadOnlyDictionary<string, double>?> SemanticScoresAsync(string query, CancellationToken cancel)
    {
        CoreConfig config = _config();
        string? model = EmbeddingModelName(config);
        if (model == null)
        {
            return null;
        }

        float[][]? vectors = await _embeddings.EmbedAsync(config, [query], cancel).ConfigureAwait(false);
        if (vectors == null)
        {
            return null;
        }

        IReadOnlyDictionary<string, float[]> stored = _store.Vectors(model);
        if (stored.Count == 0)
        {
            return null;
        }

        return stored.ToDictionary(p => p.Key, p => SqliteMemoryStore.Cosine(vectors[0], p.Value), StringComparer.Ordinal);
    }

    private static string? EmbeddingModelName(CoreConfig config)
    {
        return string.IsNullOrWhiteSpace(config.EmbeddingModel) ? null : config.EmbeddingModel.Trim();
    }

    private async Task WorkAsync()
    {
        try
        {
            await foreach (Func<CancellationToken, Task> work in _queue.Reader.ReadAllAsync(_stop.Token).ConfigureAwait(false))
            {
                try
                {
                    await work(_stop.Token).ConfigureAwait(false);
                }
                catch (OperationCanceledException) when (_stop.IsCancellationRequested)
                {
                    return;
                }
                catch (Exception ex)
                {
                    CoreLog.Error("memory worker job failed", ex);
                }
                finally
                {
                    Interlocked.Decrement(ref _pending);
                }
            }
        }
        catch (OperationCanceledException)
        {
        }
    }

    public void Dispose()
    {
        _queue.Writer.TryComplete();
        _stop.Cancel();
        try
        {
            _worker.Wait(TimeSpan.FromSeconds(5));
        }
        catch (AggregateException)
        {
        }

        _stop.Dispose();
    }
}

// Embedding tuy chon (muc 8.5.8): {endpoint}/embeddings kieu OpenAI-compatible. Khong cau hinh / goi loi ->
// tra null de chay chi keyword (FTS5), khong bao loi cho nguoi dung; loi chi log mot lan moi 10 phut.
public sealed class EmbeddingClient(HttpClient http)
{
    private DateTime _mutedUntil = DateTime.MinValue;

    public async Task<float[][]?> EmbedAsync(CoreConfig config, IReadOnlyList<string> texts, CancellationToken cancel)
    {
        string endpoint = ModelClient.NormalizeEndpoint(string.IsNullOrWhiteSpace(config.EmbeddingEndpoint) ? config.LlmEndpoint : config.EmbeddingEndpoint);
        if (string.IsNullOrWhiteSpace(config.EmbeddingModel) || endpoint.Length == 0 || texts.Count == 0 || DateTime.UtcNow < _mutedUntil)
        {
            return null;
        }

        try
        {
            using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancel);
            timeout.CancelAfter(TimeSpan.FromSeconds(20));
            var input = new JsonArray();
            foreach (string item in texts)
            {
                input.Add(item);
            }

            var body = new JsonObject { ["model"] = config.EmbeddingModel.Trim(), ["input"] = input };
            using var request = new HttpRequestMessage(HttpMethod.Post, ModelClient.BuildUrl(endpoint, "/embeddings"))
            {
                Content = new StringContent(body.ToJsonString(), Encoding.UTF8, "application/json"),
            };
            if (!string.IsNullOrEmpty(config.LlmApiKey))
            {
                request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", config.LlmApiKey);
            }

            using HttpResponseMessage response = await http.SendAsync(request, timeout.Token).ConfigureAwait(false);
            string text = await response.Content.ReadAsStringAsync(CancellationToken.None).ConfigureAwait(false);
            if (!response.IsSuccessStatusCode)
            {
                return Mute("HTTP " + (int)response.StatusCode + ": " + ModelClient.Truncate(text, 200));
            }

            if (JsonNode.Parse(text)?["data"] is not JsonArray data || data.Count != texts.Count)
            {
                return Mute("unexpected embeddings response");
            }

            var vectors = new float[texts.Count][];
            foreach (JsonNode? item in data)
            {
                int index = item?["index"]?.GetValue<int>() ?? Array.IndexOf(data.ToArray(), item);
                vectors[index] = item?["embedding"]?.AsArray().Select(v => v!.GetValue<float>()).ToArray() ?? [];
            }

            return vectors;
        }
        catch (Exception ex) when (!cancel.IsCancellationRequested)
        {
            return Mute(ex.GetType().Name + ": " + ex.Message);
        }
    }

    private float[][]? Mute(string error)
    {
        _mutedUntil = DateTime.UtcNow.AddMinutes(10);
        CoreLog.Info("embeddings unavailable, keyword-only memory search for 10 minutes: " + error);
        return null;
    }
}
