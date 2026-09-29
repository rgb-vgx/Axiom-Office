using System.Diagnostics;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Models;

// Mot luot hoi thoai cu (tu bang messages) de dua vao ngu canh.
public sealed record ConversationTurn(string Role, string Content);

public sealed record AgentLoopOptions(
    int MaxRounds = 0,            // 0 = khong gioi han so vong (giu hanh vi da kiem chung)
    int MaxTokens = 200_000,      // tran token ca luot (New_arch.md muc 8.1)
    TimeSpan? Deadline = null,    // tran thoi gian ca luot (mac dinh 300s)
    int MaxResponseTokens = 4096);

public sealed record AgentLoopCallbacks(
    Action<string>? Transcript = null,
    Action<int>? RoundStarted = null,
    Action<ToolCall>? ToolStarted = null,
    Action<ToolCallResult>? ToolFinished = null);

public sealed record AgentLoopResult(
    bool Ok,
    string? Text,
    string? Error,
    string ErrorKind,             // config | provider | internal | cancelled | timeout (rong khi Ok)
    int Rounds,
    int InputTokens,
    int OutputTokens,
    double Seconds,
    IReadOnlyList<string> Transcript,
    bool Cancelled,
    bool TimedOut,
    bool Stopped,                 // dung vi tran token
    bool ToolsDisabled);

// Vong lap agent: goi model, chay tool model yeu cau, lap toi khi model tra loi xong.
// Port tu Ai/LlmClient.cs cua add-in (da kiem chung tren Office that) sang .NET hien dai:
// timeout 60s moi request, tran ca luot, huy cat request dang cho ngay.
public sealed class ModelClient
{
    public const int RequestTimeoutMs = 60_000;
    public static readonly TimeSpan DefaultDeadline = TimeSpan.FromSeconds(300);

    private readonly HttpClient _http;
    private readonly string _endpoint;
    private readonly string _apiKey;
    private readonly string _model;

    public ModelClient(HttpClient http, string provider, string endpoint, string apiKey, string model)
    {
        _http = http;
        Codec = provider == "anthropic" ? new AnthropicCodec() : new OpenAiCodec();
        _endpoint = NormalizeEndpoint(endpoint);
        _apiKey = apiKey;
        _model = model;
    }

    public IProviderCodec Codec { get; }

    public string Model => _model;

    public static string NormalizeEndpoint(string endpoint)
    {
        string value = (endpoint ?? "").Trim();
        if (value.Length == 0)
        {
            return "";
        }

        return value.Contains("://", StringComparison.Ordinal) ? value : "http://" + value;
    }

    public static string BuildUrl(string endpoint, string suffix)
    {
        string baseUrl = endpoint.TrimEnd('/');
        return baseUrl.EndsWith(suffix, StringComparison.OrdinalIgnoreCase) ? baseUrl : baseUrl + suffix;
    }

    public async Task<AgentLoopResult> RunAgentAsync(
        string systemPrompt,
        IReadOnlyList<ConversationTurn> priorTurns,
        string userPrompt,
        IReadOnlyList<ModelTool> tools,
        Func<ToolCall, CancellationToken, Task<ToolCallResult>> execute,
        AgentLoopOptions options,
        AgentLoopCallbacks callbacks,
        CancellationToken cancel)
    {
        var transcript = new List<string>();
        var watch = Stopwatch.StartNew();

        AgentLoopResult Done(
            bool ok, string? text, string? error, string kind,
            int rounds, int inputTokens, int outputTokens,
            bool cancelled = false, bool timedOut = false, bool stopped = false, bool toolsDisabled = false)
        {
            return new AgentLoopResult(ok, text, error, kind, rounds, inputTokens, outputTokens, watch.Elapsed.TotalSeconds,
                transcript, cancelled || cancel.IsCancellationRequested, timedOut, stopped, toolsDisabled);
        }

        if (_endpoint.Length == 0)
        {
            return Done(false, null, "Endpoint is not configured", "config", 0, 0, 0);
        }

        if (string.IsNullOrEmpty(_model))
        {
            return Done(false, null, "Model is not configured", "config", 0, 0, 0);
        }

        TimeSpan deadline = options.Deadline ?? DefaultDeadline;
        using var runCts = CancellationTokenSource.CreateLinkedTokenSource(cancel);
        runCts.CancelAfter(deadline);

        var turns = new List<JsonNode>();
        foreach (ConversationTurn turn in priorTurns)
        {
            turns.Add(new JsonObject { ["role"] = turn.Role, ["content"] = turn.Content });
        }

        turns.Add(new JsonObject { ["role"] = "user", ["content"] = userPrompt });

        bool toolsEnabled = tools.Count > 0;
        int rounds = 0;
        int inputTokens = 0;
        int outputTokens = 0;

        while (options.MaxRounds <= 0 || rounds < options.MaxRounds)
        {
            if (runCts.IsCancellationRequested)
            {
                return Done(false, null, StopMessage(cancel, deadline), StopKind(cancel),
                    rounds, inputTokens, outputTokens, cancel.IsCancellationRequested, !cancel.IsCancellationRequested, false, !toolsEnabled);
            }

            rounds++;
            callbacks.RoundStarted?.Invoke(rounds);

            JsonObject body = Codec.BuildRequest(_model, systemPrompt, turns, tools, toolsEnabled, options.MaxResponseTokens);
            (string? responseText, string? error, int status) = await PostAsync(body, runCts.Token).ConfigureAwait(false);
            if (responseText == null)
            {
                if (runCts.IsCancellationRequested)
                {
                    return Done(false, null, StopMessage(cancel, deadline), StopKind(cancel),
                        rounds, inputTokens, outputTokens, cancel.IsCancellationRequested, !cancel.IsCancellationRequested, false, !toolsEnabled);
                }

                if (toolsEnabled && error != null && Codec.IsToolUnsupported(error))
                {
                    // Provider khong ho tro tools: tat tools va chay nhu chat thuong (hanh vi cu).
                    toolsEnabled = false;
                    AddTranscript(transcript, callbacks, "(provider khong ho tro tools - chay che do thuong)");
                    continue;
                }

                return Done(false, null, DescribeHttpError(status, error), "provider", rounds, inputTokens, outputTokens, toolsDisabled: !toolsEnabled);
            }

            ModelTurn? turn = Codec.Parse(responseText, out string? parseError);
            if (turn == null)
            {
                return Done(false, null, parseError, "provider", rounds, inputTokens, outputTokens, toolsDisabled: !toolsEnabled);
            }

            inputTokens += turn.InputTokens;
            outputTokens += turn.OutputTokens;

            if (turn.ToolCalls.Count == 0)
            {
                if (string.IsNullOrWhiteSpace(turn.Text))
                {
                    return Done(false, null, "provider returned an empty reply", "provider", rounds, inputTokens, outputTokens, toolsDisabled: !toolsEnabled);
                }

                return Done(true, turn.Text, null, "", rounds, inputTokens, outputTokens, toolsDisabled: !toolsEnabled);
            }

            Codec.AppendAssistant(turns, turn);

            var results = new List<ToolCallResult>();
            foreach (ToolCall call in turn.ToolCalls)
            {
                if (runCts.IsCancellationRequested)
                {
                    return Done(false, null, StopMessage(cancel, deadline), StopKind(cancel),
                        rounds, inputTokens, outputTokens, cancel.IsCancellationRequested, !cancel.IsCancellationRequested, false, !toolsEnabled);
                }

                callbacks.ToolStarted?.Invoke(call);
                var toolWatch = Stopwatch.StartNew();
                ToolCallResult result = await ExecuteToolAsync(execute, call, runCts.Token).ConfigureAwait(false);
                toolWatch.Stop();
                result = result with { Ms = toolWatch.ElapsedMilliseconds };
                results.Add(result);
                callbacks.ToolFinished?.Invoke(result);
                AddTranscript(transcript, callbacks, ToolLine(call, result));
            }

            Codec.AppendToolResults(turns, results);

            if (options.MaxTokens > 0 && inputTokens + outputTokens > options.MaxTokens)
            {
                return Done(false, null, $"token budget exceeded ({inputTokens + outputTokens} > {options.MaxTokens})",
                    "provider", rounds, inputTokens, outputTokens, stopped: true, toolsDisabled: !toolsEnabled);
            }
        }

        // Chi xay ra khi ben goi tu dat gioi han vong: bao chua xong, khong gia lam cau tra loi.
        return Done(false, null, $"agent stopped after {options.MaxRounds} rounds without a final answer",
            "provider", rounds, inputTokens, outputTokens, toolsDisabled: !toolsEnabled);
    }

    // Goi mot lan, khong tool: dung cho tom tat hoi thoai va kiem tra cau hinh.
    public async Task<(string? Text, string? Error)> ChatAsync(string systemPrompt, string userPrompt, CancellationToken cancel)
    {
        if (_endpoint.Length == 0)
        {
            return (null, "Endpoint is not configured");
        }

        if (string.IsNullOrEmpty(_model))
        {
            return (null, "Model is not configured");
        }

        using var cts = CancellationTokenSource.CreateLinkedTokenSource(cancel);
        cts.CancelAfter(RequestTimeoutMs);
        JsonObject body = Codec.BuildRequest(_model, systemPrompt,
            [new JsonObject { ["role"] = "user", ["content"] = userPrompt }], [], toolsEnabled: false, maxTokens: 1024);
        (string? responseText, string? error, int status) = await PostAsync(body, cts.Token).ConfigureAwait(false);
        if (responseText == null)
        {
            return (null, status > 0 ? $"HTTP {status}: {Truncate(error, 300)}" : error);
        }

        ModelTurn? turn = Codec.Parse(responseText, out string? parseError);
        return turn == null ? (null, parseError) : (turn.Text, null);
    }

    private static string StopMessage(CancellationToken userCancel, TimeSpan deadline)
    {
        return userCancel.IsCancellationRequested ? "cancelled by user" : $"agent timed out after {deadline.TotalSeconds:0}s";
    }

    private static string StopKind(CancellationToken userCancel)
    {
        return userCancel.IsCancellationRequested ? "cancelled" : "timeout";
    }

    private async Task<(string? Text, string? Error, int Status)> PostAsync(JsonObject body, CancellationToken cancel)
    {
        using var requestCts = CancellationTokenSource.CreateLinkedTokenSource(cancel);
        requestCts.CancelAfter(RequestTimeoutMs);
        try
        {
            using var request = new HttpRequestMessage(HttpMethod.Post, BuildUrl(_endpoint, Codec.Path))
            {
                Content = new StringContent(body.ToJsonString(), Encoding.UTF8, "application/json"),
            };
            if (!string.IsNullOrEmpty(_apiKey))
            {
                if (Codec.Name == "anthropic")
                {
                    request.Headers.Add("x-api-key", _apiKey);
                    request.Headers.Add("anthropic-version", "2023-06-01");
                }
                else
                {
                    request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _apiKey);
                }
            }

            using HttpResponseMessage response = await _http.SendAsync(request, requestCts.Token).ConfigureAwait(false);
            string text = await response.Content.ReadAsStringAsync(CancellationToken.None).ConfigureAwait(false);
            return response.IsSuccessStatusCode ? (text, null, (int)response.StatusCode) : (null, Truncate(text, 300), (int)response.StatusCode);
        }
        catch (OperationCanceledException) when (!cancel.IsCancellationRequested)
        {
            return (null, $"request timed out after {RequestTimeoutMs / 1000}s", 0);
        }
        catch (OperationCanceledException)
        {
            return (null, "cancelled", 0);
        }
        catch (Exception ex)
        {
            return (null, ex.GetType().Name + ": " + ex.Message, 0);
        }
    }

    private static async Task<ToolCallResult> ExecuteToolAsync(
        Func<ToolCall, CancellationToken, Task<ToolCallResult>> execute, ToolCall call, CancellationToken cancel)
    {
        try
        {
            return await execute(call, cancel).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (cancel.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            // Loi tool khong lam hong luot chay: tra ve model de no tu xu ly.
            return new ToolCallResult(call.Id, call.Name, ErrorJson(ex.Message), false, 0);
        }
    }

    public static string ErrorJson(string message)
    {
        // Encoder tho: de thong bao loi doc duoc nguyen van (khong bi escape thanh ').
        return new JsonObject { ["ok"] = false, ["error"] = message }.ToJsonString(RelaxedJson);
    }

    internal static readonly JsonSerializerOptions RelaxedJson = new()
    {
        Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
    };

    private static string ToolLine(ToolCall call, ToolCallResult result)
    {
        return call.Name + " " + Truncate(call.ArgumentsJson, 150) + " -> " + Truncate(result.ResultJson, 150);
    }

    public static string Truncate(string? value, int max)
    {
        if (string.IsNullOrEmpty(value) || value.Length <= max)
        {
            return value ?? "";
        }

        return value[..max] + "...";
    }

    private static void AddTranscript(List<string> transcript, AgentLoopCallbacks callbacks, string line)
    {
        transcript.Add(line);
        callbacks.Transcript?.Invoke(line);
    }

    private static string DescribeHttpError(int status, string? error)
    {
        string detail = Truncate(error, 300);
        return status > 0 ? $"HTTP {status}: {detail}" : detail;
    }
}
