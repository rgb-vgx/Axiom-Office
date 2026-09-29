using System.Text.Json.Nodes;
using AxiomOffice.Core.Office;

namespace AxiomOffice.Core.Tools;

// Tool duy nhat ma agent dung de doc/sua tai lieu dang mo: office_action {action, params}.
// Mo ta tool liet ke chu ky cac lenh agent=true cua app dang mo (tu GET /commands) va CHI cho goi
// nhung lenh do - allowlist giong OfficeActionTool cua add-in (New_arch.md muc 8.3, 8.6).
public sealed class OfficeActionTool : ITool
{
    public const string ToolName = "office_action";

    // Log 30/09 01:47: khong co lenh dinh dang bang, model undo 2 lan (xoa bang + ghi chu) roi dung lai tu dau.
    public const string UndoRule =
        "Change existing content in place (e.g. writer.formatTable to restyle a table); " +
        "never use undo to start over - only undo when the user asks.";

    private readonly Dictionary<string, OfficeCommand> _allowed;
    private readonly string _signatures;

    public OfficeActionTool(OfficeCommandCatalog catalog, string appKind)
    {
        var allowed = new List<OfficeCommand>();
        foreach (OfficeCommand command in catalog.Commands)
        {
            if (!command.Agent)
            {
                continue;
            }

            if (command.Kind != null && command.Kind != appKind)
            {
                continue;
            }

            allowed.Add(command);
        }

        _allowed = allowed.ToDictionary(c => c.Name, StringComparer.Ordinal);
        _signatures = string.Join("; ", allowed.Select(Signature));
    }

    public string Name => ToolName;

    public string Description =>
        "Read and modify the LIVE document that is currently open in the office application. " +
        "Call this for every document change the user asks for so it happens immediately on screen. " +
        "Array params such as values must be real JSON arrays of rows, not objects. " +
        UndoRule + " " +
        "Available actions (with params): " + _signatures;

    public JsonNode ParametersSchema => new JsonObject
    {
        ["type"] = "object",
        ["properties"] = new JsonObject
        {
            ["action"] = new JsonObject
            {
                ["type"] = "string",
                ["description"] = "Action name, e.g. writer.insertStyledText or wpp.addSlide",
            },
            ["params"] = new JsonObject
            {
                ["type"] = "object",
                ["description"] = "Action parameters as an object (may be omitted)",
            },
        },
        ["required"] = new JsonArray("action"),
    };

    public IReadOnlyCollection<string> AllowedActions => _allowed.Keys;

    public async Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel)
    {
        string? action = arguments?["action"]?.GetValue<string>();
        if (string.IsNullOrWhiteSpace(action))
        {
            return new ToolResult(Error("missing 'action' argument"), false);
        }

        if (!_allowed.ContainsKey(action))
        {
            // Model chi duoc goi lenh co trong mo ta tool (chan writer.closeAll, ai.ask long nhau...).
            return new ToolResult(Error($"'{action}' is not an available action; use one of the actions listed in the office_action tool description"), false, action);
        }

        JsonNode? parameters = NormalizeParams(arguments?["params"]);
        BridgeResult result = await context.Bridge
            .CommandAsync(context.Office.Port, action, parameters, cancel)
            .ConfigureAwait(false);

        return new ToolResult(result.RawJson, result.Ok, action);
    }

    // Model doi khi gui params la chuoi JSON ("{\"rows\":3}") thay vi object: parse ra. Gia tri bi boc
    // kieu mang XML ({"item": ...}) de bridge go (CommandDispatcher.Params) - dung chung cho MCP/in-process.
    public static JsonNode? NormalizeParams(JsonNode? parameters)
    {
        if (parameters is JsonValue value && value.TryGetValue(out string? text))
        {
            string trimmed = text.Trim();
            if (trimmed.Length == 0)
            {
                return null;
            }

            if (trimmed.StartsWith('{'))
            {
                try
                {
                    return JsonNode.Parse(trimmed) as JsonObject ?? parameters;
                }
                catch (System.Text.Json.JsonException)
                {
                    return parameters;
                }
            }
        }

        return parameters?.DeepClone();
    }

    private static string Signature(OfficeCommand command)
    {
        string parameters = string.Join(",", command.Params.Select(p => p.Hint == null ? p.Name : p.Name + " (" + p.Hint + ")"));
        return command.Name + " {" + parameters + "}";
    }

    private static string Error(string message)
    {
        return new JsonObject { ["ok"] = false, ["error"] = message }.ToJsonString(Models.ModelClient.RelaxedJson);
    }
}

// Tap tool cua mot luot chay (giai doan 2/3/4 them load_skill, remember, mcp__*).
public sealed class ToolRegistry(IEnumerable<ITool> tools)
{
    private readonly List<ITool> _tools = tools.ToList();

    public IReadOnlyList<ITool> All => _tools;

    public ITool? Find(string name)
    {
        return _tools.FirstOrDefault(t => string.Equals(t.Name, name, StringComparison.Ordinal));
    }
}
