using System.Text;
using System.Text.Json.Nodes;
using System.Text.RegularExpressions;
using AxiomOffice.Core.Logging;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Tools;

namespace AxiomOffice.Core.Mcp;

// Cau hinh mot MCP server (mcp.json theo dinh dang quen thuoc {"mcpServers": {...}}, muc 8.7).
public sealed record McpServerConfig(
    string Name,
    string? Command,
    IReadOnlyList<string> Args,
    IReadOnlyDictionary<string, string> Env,
    string? Url,
    IReadOnlyDictionary<string, string> Headers,
    bool Trusted,
    bool Disabled,
    IReadOnlySet<string>? AllowedTools = null,
    bool BuiltIn = false);

public sealed record McpToolInfo(string Name, string Description, JsonNode InputSchema);

// Mot ket noi MCP da bat tay (initialize -> notifications/initialized -> tools/list).
public sealed class McpServer : IDisposable
{
    public const string ProtocolVersion = "2025-06-18";

    private readonly IMcpTransport _transport;

    private McpServer(McpServerConfig config, IMcpTransport transport, IReadOnlyList<McpToolInfo> tools)
    {
        Config = config;
        _transport = transport;
        Tools = tools;
    }

    public McpServerConfig Config { get; }

    public IReadOnlyList<McpToolInfo> Tools { get; }

    public bool IsAlive => _transport.IsAlive;

    public static async Task<McpServer> StartAsync(McpServerConfig config, HttpClient http, CancellationToken cancel)
    {
        IMcpTransport transport = config.Url != null
            ? new HttpMcpTransport(config.Name, config.Url, config.Headers, http)
            : new StdioMcpTransport(config.Name, config.Command!, config.Args, config.Env, Path.GetDirectoryName(config.Command!) is { Length: > 0 } dir && Directory.Exists(dir) ? dir : null);
        try
        {
            await transport.RequestAsync("initialize", new JsonObject
            {
                ["protocolVersion"] = ProtocolVersion,
                ["capabilities"] = new JsonObject(),
                ["clientInfo"] = new JsonObject { ["name"] = "axiom-office-core", ["version"] = CoreVersion.Value },
            }, cancel).ConfigureAwait(false);
            await transport.NotifyAsync("notifications/initialized", null, cancel).ConfigureAwait(false);

            var tools = new List<McpToolInfo>();
            string? cursor = null;
            do
            {
                JsonNode? page = await transport.RequestAsync("tools/list", cursor == null ? new JsonObject() : new JsonObject { ["cursor"] = cursor }, cancel).ConfigureAwait(false);
                foreach (JsonNode? tool in page?["tools"]?.AsArray() ?? [])
                {
                    string? name = tool?["name"]?.GetValue<string>();
                    if (string.IsNullOrEmpty(name) || (config.AllowedTools != null && !config.AllowedTools.Contains(name)))
                    {
                        continue;
                    }

                    tools.Add(new McpToolInfo(name, tool?["description"]?.GetValue<string>() ?? "", tool?["inputSchema"]?.DeepClone() ?? new JsonObject { ["type"] = "object" }));
                }

                cursor = page?["nextCursor"]?.GetValue<string>();
            }
            while (!string.IsNullOrEmpty(cursor));

            return new McpServer(config, transport, tools);
        }
        catch
        {
            transport.Dispose();
            throw;
        }
    }

    // tools/call -> (text noi cac content text, isError).
    public async Task<(string Text, bool IsError)> CallAsync(string tool, JsonNode? arguments, CancellationToken cancel)
    {
        JsonNode? result = await _transport.RequestAsync("tools/call", new JsonObject
        {
            ["name"] = tool,
            ["arguments"] = arguments?.DeepClone() ?? new JsonObject(),
        }, cancel).ConfigureAwait(false);
        var text = new StringBuilder();
        foreach (JsonNode? content in result?["content"]?.AsArray() ?? [])
        {
            if (content?["type"]?.GetValue<string>() == "text")
            {
                text.Append(content["text"]?.GetValue<string>()).Append('\n');
            }
            else if (content != null)
            {
                text.Append('[').Append(content["type"]?.GetValue<string>() ?? "content").Append("]\n");
            }
        }

        if (text.Length == 0 && result?["structuredContent"] is JsonNode structured)
        {
            text.Append(structured.ToJsonString());
        }

        bool isError = result?["isError"]?.GetValue<bool>() ?? false;
        return (text.ToString().TrimEnd(), isError);
    }

    public void Dispose() => _transport.Dispose();
}

// Quan ly MCP server cua Core (muc 8.7): server built-in "office" (lan file cua Host.exe) + mcp.json cua nguoi
// dung. Khoi dong luoi o run dau tien can; loi mot server -> an tool cua no, log, khong hong run. mcp.json doi
// -> nap lai o run sau.
public sealed class McpManager : IDisposable
{
    // Lan file cua Host.exe: chi tool doc/ghi file tren dia. Tool live (word_*, ppt_* live, wps_live_*...) trung
    // voi office_action nhung KHONG qua allowlist + policy xac nhan nen khong dua cho agent.
    public static readonly IReadOnlySet<string> OfficeFileTools = new HashSet<string>(StringComparer.Ordinal)
    {
        "doc_profile", "doc_get_text", "doc_find_text", "doc_extract_table", "doc_create",
        "excel_profile", "excel_read", "excel_create_sheet", "excel_copy_sheet", "excel_rename_sheet", "excel_delete_sheet",
        "excel_format_range", "excel_create_table", "excel_write", "excel_create", "excel_convert",
        "ppt_profile", "ppt_get_text", "ppt_create", "ppt_add_slide_file",
    };

    public static readonly IReadOnlySet<string> OfficeReadOnlyTools = new HashSet<string>(StringComparer.Ordinal)
    {
        "doc_profile", "doc_get_text", "doc_find_text", "doc_extract_table", "excel_profile", "excel_read", "ppt_profile", "ppt_get_text",
    };

    public static readonly TimeSpan StartTimeout = TimeSpan.FromSeconds(20);

    private readonly string _configFile;
    private readonly string? _hostExe;
    private readonly HttpClient _http;
    private readonly SemaphoreSlim _gate = new(1, 1);
    private readonly Dictionary<string, McpServer> _servers = new(StringComparer.Ordinal);
    private readonly Dictionary<string, string> _errors = new(StringComparer.Ordinal);
    private DateTime _configStamp = DateTime.MinValue;
    private IReadOnlyList<McpServerConfig> _configs = [];

    public McpManager(string configFile, string? hostExe, HttpClient http)
    {
        _configFile = configFile;
        _hostExe = hostExe;
        _http = http;
    }

    public IReadOnlyDictionary<string, string> Errors
    {
        get
        {
            lock (_errors)
            {
                return new Dictionary<string, string>(_errors);
            }
        }
    }

    public IReadOnlyList<McpServerConfig> Configs => _configs;

    // Doc cau hinh: built-in office (tru khi "office": {"disabled": true}) + mcp.json.
    public static IReadOnlyList<McpServerConfig> LoadConfigs(string? json, string? hostExe, out string? error)
    {
        error = null;
        var configs = new List<McpServerConfig>();
        JsonObject? servers = null;
        if (!string.IsNullOrWhiteSpace(json))
        {
            try
            {
                servers = JsonNode.Parse(json)?["mcpServers"] as JsonObject;
            }
            catch (System.Text.Json.JsonException ex)
            {
                error = "mcp.json is not valid JSON: " + ex.Message;
            }
        }

        JsonObject? officeOverride = servers?["office"] as JsonObject;
        bool officeDisabled = officeOverride?["disabled"]?.GetValue<bool>() ?? false;
        if (hostExe != null && File.Exists(hostExe) && !officeDisabled && (officeOverride?["command"] == null && officeOverride?["url"] == null))
        {
            configs.Add(new McpServerConfig("office", hostExe, ["mcp"], new Dictionary<string, string>(), null,
                new Dictionary<string, string>(), Trusted: true, Disabled: false, OfficeFileTools, BuiltIn: true));
        }

        foreach ((string name, JsonNode? node) in servers ?? [])
        {
            if (node is not JsonObject server || (name == "office" && server["command"] == null && server["url"] == null))
            {
                continue;
            }

            var args = server["args"] is JsonArray list ? list.Select(a => a?.ToString() ?? "").ToList() : [];
            var env = server["env"] is JsonObject envObject ? envObject.ToDictionary(p => p.Key, p => p.Value?.ToString() ?? "") : [];
            var headers = server["headers"] is JsonObject headerObject ? headerObject.ToDictionary(p => p.Key, p => p.Value?.ToString() ?? "") : [];
            configs.Add(new McpServerConfig(
                name,
                server["command"]?.ToString(),
                args,
                env,
                server["url"]?.ToString(),
                headers,
                Trusted: server["trusted"]?.GetValue<bool>() ?? false,
                Disabled: server["disabled"]?.GetValue<bool>() ?? false));
        }

        return configs.Where(c => !c.Disabled && (c.Command != null || c.Url != null)).ToList();
    }

    // Tool MCP cho mot run (khoi dong server can thiet lan dau).
    public async Task<IReadOnlyList<ITool>> ToolsAsync(CancellationToken cancel)
    {
        await _gate.WaitAsync(cancel).ConfigureAwait(false);
        try
        {
            ReloadConfigIfChanged();
            var starts = new List<Task>();
            foreach (McpServerConfig config in _configs)
            {
                if (_servers.TryGetValue(config.Name, out McpServer? running) && running.IsAlive)
                {
                    continue;
                }

                if (running != null)
                {
                    running.Dispose();
                    _servers.Remove(config.Name);
                }

                starts.Add(StartOneAsync(config, cancel));
            }

            await Task.WhenAll(starts).ConfigureAwait(false);
            var tools = new List<ITool>();
            foreach (McpServer server in _servers.Values)
            {
                tools.AddRange(server.Tools.Select(t => (ITool)new McpTool(server, t)));
            }

            return tools;
        }
        finally
        {
            _gate.Release();
        }
    }

    private async Task StartOneAsync(McpServerConfig config, CancellationToken cancel)
    {
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancel);
        timeout.CancelAfter(StartTimeout);
        try
        {
            McpServer server = await McpServer.StartAsync(config, _http, timeout.Token).ConfigureAwait(false);
            lock (_servers)
            {
                _servers[config.Name] = server;
            }

            lock (_errors)
            {
                _errors.Remove(config.Name);
            }

            CoreLog.Info($"mcp {config.Name}: {server.Tools.Count} tools" + (config.Trusted ? " (trusted)" : " (confirm required)"));
        }
        catch (Exception ex) when (!cancel.IsCancellationRequested)
        {
            string message = ex is OperationCanceledException ? "start timed out after " + StartTimeout.TotalSeconds + "s" : ex.Message;
            lock (_errors)
            {
                _errors[config.Name] = message;
            }

            CoreLog.Info($"mcp {config.Name} unavailable (tools hidden): {message}");
        }
    }

    private void ReloadConfigIfChanged()
    {
        DateTime stamp = File.Exists(_configFile) ? File.GetLastWriteTimeUtc(_configFile) : DateTime.MinValue;
        if (stamp == _configStamp && _configStamp != DateTime.MinValue)
        {
            return;
        }

        bool first = _configStamp == DateTime.MinValue && _configs.Count == 0;
        _configStamp = stamp == DateTime.MinValue ? DateTime.MaxValue : stamp;
        string? json = File.Exists(_configFile) ? File.ReadAllText(_configFile) : null;
        _configs = LoadConfigs(json, _hostExe, out string? error);
        if (error != null)
        {
            CoreLog.Info(error);
        }

        if (!first)
        {
            foreach (McpServer server in _servers.Values)
            {
                server.Dispose();
            }

            _servers.Clear();
        }
    }

    public void Dispose()
    {
        foreach (McpServer server in _servers.Values)
        {
            server.Dispose();
        }

        _servers.Clear();
    }
}

// Mot tool MCP cho model: ten mcp__<server>__<tool>; server khong tin cay -> hoi nguoi dung truoc moi lan goi;
// server office (tin cay) -> hoi khi tool GHI vao file DA CO (cung quy tac voi saveAs).
public sealed partial class McpTool(McpServer server, McpToolInfo tool) : ITool
{
    public const int MaxResultChars = 64 * 1024;

    public string Name { get; } = ToolName(server.Config.Name, tool.Name);

    public string Description => ModelClient.Truncate("[" + server.Config.Name + "] " + tool.Description, 1000);

    public JsonNode ParametersSchema => tool.InputSchema.DeepClone();

    public static string ToolName(string serverName, string toolName)
    {
        string name = "mcp__" + Unsafe().Replace(serverName, "_") + "__" + Unsafe().Replace(toolName, "_");
        return name.Length <= 64 ? name : name[..64];
    }

    [GeneratedRegex("[^A-Za-z0-9_-]")]
    private static partial Regex Unsafe();

    public async Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel)
    {
        string? reason = null;
        if (!server.Config.Trusted)
        {
            reason = $"Dùng công cụ ngoài '{server.Config.Name}': {tool.Name}";
        }
        else if (server.Config.BuiltIn && !McpManager.OfficeReadOnlyTools.Contains(tool.Name)
                 && arguments?["path"] is JsonValue pathValue && pathValue.TryGetValue(out string? path) && File.Exists(path))
        {
            reason = "Sửa file trên đĩa: " + path;
        }

        if (reason != null)
        {
            string preview = ModelClient.Truncate(arguments?.ToJsonString(ModelClient.RelaxedJson) ?? "{}", 200);
            if (!await context.ConfirmAsync(Name, reason, preview, cancel).ConfigureAwait(false))
            {
                return new ToolResult(ModelClient.ErrorJson("user declined: " + reason), false, Name);
            }
        }

        try
        {
            (string text, bool isError) = await server.CallAsync(tool.Name, arguments, cancel).ConfigureAwait(false);
            var json = new JsonObject
            {
                ["ok"] = !isError,
                [isError ? "error" : "result"] = ModelClient.Truncate(text, MaxResultChars),
            };
            return new ToolResult(json.ToJsonString(ModelClient.RelaxedJson), !isError, Name);
        }
        catch (Exception ex) when (ex is McpException or IOException or InvalidOperationException)
        {
            return new ToolResult(ModelClient.ErrorJson($"mcp server '{server.Config.Name}' error: {ex.Message}"), false, Name);
        }
    }
}
