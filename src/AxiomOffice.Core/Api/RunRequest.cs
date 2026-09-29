using System.Text.Json.Nodes;
using AxiomOffice.Core.Agent;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Api;

// Body cua POST /v1/runs (New_arch.md muc 7.3).
public sealed record RunRequest(
    string Prompt,
    string? ConversationId,
    int Port,
    int? Pid,
    string App,
    string Family,
    DocumentContext? Document,
    AgentLoopOptions Options);

public static class RunRequestParser
{
    public const int DefaultMaxSeconds = 300;
    public const int MaxMaxSeconds = 900;
    public const int DefaultMaxTokens = 200_000;
    public const int MaxMaxTokens = 1_000_000;

    public static RunRequest? Parse(JsonNode? body, out string? error)
    {
        error = null;
        if (body == null)
        {
            error = "invalid JSON body";
            return null;
        }

        string prompt = body["prompt"]?.GetValue<string>() ?? "";
        if (string.IsNullOrWhiteSpace(prompt))
        {
            error = "'prompt' is required";
            return null;
        }

        JsonNode? office = body["office"];
        int port = office?["port"]?.GetValue<int>() ?? 0;
        if (port <= 0)
        {
            error = "'office.port' is required (find it with office_sessions / GET /session)";
            return null;
        }

        JsonNode? document = body["document"];
        JsonNode? options = body["options"];
        int maxSeconds = Clamp(options?["maxSeconds"]?.GetValue<int>() ?? DefaultMaxSeconds, 30, MaxMaxSeconds);
        int maxTokens = Clamp(options?["maxTokens"]?.GetValue<int>() ?? DefaultMaxTokens, 1_000, MaxMaxTokens);

        return new RunRequest(
            Prompt: prompt,
            ConversationId: Nullable(body["conversationId"]?.GetValue<string>()),
            Port: port,
            Pid: office?["pid"]?.GetValue<int>(),
            App: office?["app"]?.GetValue<string>() ?? "",
            Family: office?["family"]?.GetValue<string>() ?? "",
            Document: document == null
                ? null
                : new DocumentContext(
                    Nullable(document["name"]?.GetValue<string>()),
                    Nullable(document["fullName"]?.GetValue<string>()),
                    Nullable(document["selection"]?["text"]?.GetValue<string>())),
            Options: new AgentLoopOptions(
                MaxRounds: 0,
                MaxTokens: maxTokens,
                Deadline: TimeSpan.FromSeconds(maxSeconds)));
    }

    private static string? Nullable(string? value)
    {
        return string.IsNullOrWhiteSpace(value) ? null : value;
    }

    private static int Clamp(int value, int min, int max)
    {
        return value < min ? min : value > max ? max : value;
    }
}
