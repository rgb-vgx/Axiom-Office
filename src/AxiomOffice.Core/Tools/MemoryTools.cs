using System.Text.Json.Nodes;
using AxiomOffice.Core.Memory;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Tools;

// remember {scope, text, category?} -> ADD qua bo chong trung (New_arch.md muc 8.3, 8.5.6), phat memory.written.
public sealed class RememberTool(MemoryService memory, string? documentKey) : ITool
{
    public const string ToolName = "remember";

    public string Name => ToolName;

    public string Description =>
        "Save one long-term fact so it is available in future conversations: who the user is, their organisation, "
        + "who signs their letters, formatting they always want (scope 'user'), or the progress of this document "
        + "(scope 'document'). One self-contained sentence in the user's language, at most 300 characters. "
        + "Only call it when the user states something worth remembering; never save passwords, IDs, keys or document content.";

    public JsonNode ParametersSchema => new JsonObject
    {
        ["type"] = "object",
        ["properties"] = new JsonObject
        {
            ["scope"] = new JsonObject { ["type"] = "string", ["enum"] = new JsonArray("user", "document") },
            ["text"] = new JsonObject { ["type"] = "string", ["description"] = "The fact, self-contained, <= 300 characters" },
            ["category"] = new JsonObject
            {
                ["type"] = "string",
                ["enum"] = new JsonArray("identity", "preference", "format", "contact", "project", "progress", "other"),
            },
        },
        ["required"] = new JsonArray("scope", "text"),
    };

    public async Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel)
    {
        string scope = (Str(arguments?["scope"]) ?? MemoryScopes.User).Trim();
        string text = (Str(arguments?["text"]) ?? "").Trim();
        if (scope == MemoryScopes.Document && documentKey == null)
        {
            scope = MemoryScopes.User;
        }

        if (scope is not (MemoryScopes.User or MemoryScopes.Document))
        {
            return new ToolResult(ModelClient.ErrorJson("scope must be 'user' or 'document'"), false);
        }

        var fact = new NewMemory(scope, scope == MemoryScopes.Document ? documentKey : null, text, Str(arguments?["category"]));
        MemoryOpResult result = (await memory.AddAsync([fact], MemoryActors.Agent, context.RunId, cancel).ConfigureAwait(false))[0];
        if (result.Event == "SKIPPED")
        {
            return new ToolResult(ModelClient.ErrorJson("not saved: " + result.Reason), false);
        }

        if (result.Event == "ADD" && result.Item != null)
        {
            context.Event?.Invoke("memory.written", new JsonObject
            {
                ["id"] = result.Item.Id,
                ["event"] = "ADD",
                ["scope"] = result.Item.Scope,
                ["text"] = result.Item.Text,
            });
        }

        return new ToolResult(new JsonObject
        {
            ["ok"] = true,
            ["result"] = new JsonObject { ["event"] = result.Event, ["id"] = result.Item?.Id },
        }.ToJsonString(ModelClient.RelaxedJson), true);
    }

    internal static string? Str(JsonNode? node) => node is JsonValue value && value.TryGetValue(out string? text) ? text : null;
}

// recall {query, scope?, limit?} -> memory lien quan (cung retriever voi ngu canh prompt, muc 8.5.7).
public sealed class RecallTool(MemoryService memory, string? documentKey) : ITool
{
    public const string ToolName = "recall";

    public string Name => ToolName;

    public string Description =>
        "Search long-term memory for facts about the user, their organisation or this document when the facts already "
        + "listed in the system prompt are not enough.";

    public JsonNode ParametersSchema => new JsonObject
    {
        ["type"] = "object",
        ["properties"] = new JsonObject
        {
            ["query"] = new JsonObject { ["type"] = "string" },
            ["scope"] = new JsonObject { ["type"] = "string", ["enum"] = new JsonArray("user", "document") },
            ["limit"] = new JsonObject { ["type"] = "integer", ["minimum"] = 1, ["maximum"] = 20 },
        },
        ["required"] = new JsonArray("query"),
    };

    public async Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel)
    {
        string query = RememberTool.Str(arguments?["query"]) ?? "";
        string? scope = RememberTool.Str(arguments?["scope"]);
        int limit = arguments?["limit"] is JsonValue value && value.TryGetValue(out int parsed) ? Math.Clamp(parsed, 1, 20) : 8;
        IReadOnlyList<MemoryHit> hits = await memory.SearchAsync(query, documentKey, scope, limit, cancel).ConfigureAwait(false);
        var list = new JsonArray();
        foreach (MemoryHit hit in hits)
        {
            list.Add(new JsonObject
            {
                ["id"] = MemoryRetriever.ShortId(hit.Item.Id),
                ["scope"] = hit.Item.Scope,
                ["text"] = hit.Item.Text,
                ["score"] = Math.Round(hit.Score, 3),
                ["createdAt"] = hit.Item.CreatedAt,
            });
        }

        return new ToolResult(new JsonObject { ["ok"] = true, ["result"] = new JsonObject { ["memories"] = list } }.ToJsonString(ModelClient.RelaxedJson), true);
    }
}
