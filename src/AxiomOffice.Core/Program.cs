using AxiomOffice.Core;
using AxiomOffice.Core.Agent;
using AxiomOffice.Core.Api;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;
using AxiomOffice.Core.Mcp;
using AxiomOffice.Core.Memory;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Office;
using AxiomOffice.Core.Skills;

// AxiomOffice.Core.exe - Agent Core (New_arch.md): process rieng cua agent, mot ban cho moi nguoi
// dung Windows, chi nghe 127.0.0.1. Add-in khoi dong Core khi can va tim no qua core.json.

var builder = WebApplication.CreateBuilder(args);

// Log cua Core di qua CoreLog (file core.log); khong dung console logger cua ASP.NET.
builder.Logging.ClearProviders();

CoreConfig config = CoreConfig.Load(builder.Configuration);
CorePaths paths = CorePaths.Create(config.DataDirOverride);
paths.EnsureDirectories();
CoreLog.Init(paths.LogFile);

// Mot Core cho moi nguoi dung: instance thu hai thoat ngay (New_arch.md muc 4.3).
Mutex? instanceLock = null;
if (config.SingleInstance)
{
    instanceLock = new Mutex(initiallyOwned: false, config.MutexName);
    bool acquired;
    try
    {
        acquired = instanceLock.WaitOne(TimeSpan.Zero);
    }
    catch (AbandonedMutexException)
    {
        // Instance truoc bi kill: ta tiep quan.
        acquired = true;
    }

    if (!acquired)
    {
        CoreLog.Info("Agent Core is already running; this instance exits.");
        return 0;
    }
}

// Port: uu tien CorePort trong HKCU, ban thi thu cac port lien tiep; 0 = de he dieu hanh cap (test).
int port = config.CorePort;
if (!config.DynamicPort)
{
    int? free = CoreRuntime.TryPickPort(config.CorePort);
    if (free == null)
    {
        string message = $"No free port in range {config.CorePort}..{config.CorePort + 9}; Agent Core cannot start.";
        CoreLog.Error(message);
        Console.Error.WriteLine(message);
        return 4;
    }

    port = free.Value;
}

var runtime = new CoreRuntime { Port = port };
builder.WebHost.UseUrls($"http://127.0.0.1:{port}");

WebApplication app = builder.Build();
CoreApiGuard.Use(app, config);

// Phu thuoc cua Core (khong dung DI container: it thanh phan, khoi tao mot lan).
var stores = new CoreStores(paths);
var http = new HttpClient { Timeout = Timeout.InfiniteTimeSpan };
var bridge = new BridgeClient(config, http, CoreLog.Info);
var sessions = new SessionDirectory(config.SessionDirectoryOverride ?? SessionDirectory.DefaultDirectory);
var models = new ModelSource(http, () => CoreConfig.Load(builder.Configuration));
var manager = new RunManager();
// Skill: skills\ canh exe -> SkillDirs cua to chuc -> %LOCALAPPDATA%\AxiomOffice\skills (muc 8.4.4).
var skills = new SkillIndex(SkillIndex.DefaultSources(
    Path.Combine(AppContext.BaseDirectory, "skills"), config.SkillDirs, paths.SkillsDirectory));
skills.Watch();
// Memory dai han (muc 8.5): MemoryModel (neu co) cho trich xuat, mac dinh = model chinh.
var memory = new MemoryService(
    new SqliteMemoryStore(stores.Db),
    stores.Conversations,
    stores.Runs,
    () => CoreConfig.Load(builder.Configuration),
    current => string.IsNullOrWhiteSpace(current.MemoryModel)
        ? models.Current()
        : new ModelClient(http, current.LlmProvider, current.LlmEndpoint, current.LlmApiKey, current.MemoryModel.Trim()),
    http,
    stores.Available);
if (stores.Available)
{
    memory.Enqueue(_ =>
    {
        int purged = memory.Store.PurgeSoftDeleted(DateTime.UtcNow);
        if (purged > 0)
        {
            CoreLog.Info($"memory: purged {purged} soft-deleted memories older than {SqliteMemoryStore.SoftDeleteRetentionDays} days");
        }

        return Task.CompletedTask;
    });
    memory.QueueEmbeddingBackfill();
}

// MCP client (muc 8.7): server built-in "office" = AxiomOffice.Host.exe mcp canh exe (lan file) + mcp.json.
var mcp = new McpManager(paths.McpConfigFile, Path.Combine(AppContext.BaseDirectory, "AxiomOffice.Host.exe"), http);
var orchestrator = new Orchestrator(config, bridge, sessions, stores.Conversations, stores.Runs, models.Current, new ContextAssembler(), skills, memory, mcp);

CoreApi.Map(app, config, paths, runtime, stores);
RunEndpoints.Map(app, manager, orchestrator, stores);
SkillEndpoints.Map(app, skills);
MemoryEndpoints.Map(app, memory);
// Wizard thiet lap (SetupWizardForm tren Windows, setupwizard.py tren Linux): preset + thu ket noi LLM + danh sach model.
SetupEndpoints.Map(app, config, paths, runtime, http, skills, stores);

// Trang thai MCP: server da cau hinh (khong in env/headers vi co the chua key) + loi khoi dong.
app.MapGet("/v1/mcp", async (HttpContext context) =>
{
    IReadOnlyList<AxiomOffice.Core.Tools.ITool> tools = await mcp.ToolsAsync(context.RequestAborted);
    var servers = new System.Text.Json.Nodes.JsonArray();
    foreach (McpServerConfig server in mcp.Configs)
    {
        var names = new System.Text.Json.Nodes.JsonArray();
        foreach (AxiomOffice.Core.Tools.ITool tool in tools.Where(t => t.Name.StartsWith(McpTool.ToolName(server.Name, ""), StringComparison.Ordinal)))
        {
            names.Add(tool.Name);
        }

        servers.Add(new System.Text.Json.Nodes.JsonObject
        {
            ["name"] = server.Name,
            ["transport"] = server.Url != null ? "http" : "stdio",
            ["trusted"] = server.Trusted,
            ["builtIn"] = server.BuiltIn,
            ["tools"] = names,
            ["error"] = mcp.Errors.TryGetValue(server.Name, out string? error) ? error : null,
        });
    }

    return ApiJson.Ok(new System.Text.Json.Nodes.JsonObject { ["servers"] = servers });
});

app.Lifetime.ApplicationStarted.Register(() =>
{
    runtime.Port = CoreRuntime.ResolveBoundPort(app) ?? runtime.Port;
    CoreFile.Write(paths.CoreJson, new CoreFileInfo(
        Environment.ProcessId,
        runtime.Port,
        CoreVersion.Value,
        runtime.StartedUtc.ToString("yyyy-MM-ddTHH:mm:ss.fffZ"),
        CoreVersion.Protocol,
        Environment.ProcessPath ?? ""));
    CoreLog.Info($"Agent Core ready: port={runtime.Port} pid={Environment.ProcessId} version={CoreVersion.Value} "
        + $"protocol={CoreVersion.Protocol} dataDir={paths.Root} memory={stores.Status(config.MemoryEnabled)}");
});

app.Lifetime.ApplicationStopped.Register(() =>
{
    // Huy cac luot chay dang do de pane nhan run.cancelled thay vi treo.
    foreach (RunState run in manager.List())
    {
        if (run.Status == RunStatus.Running)
        {
            manager.Cancel(run.Id);
        }
    }

    memory.Dispose();
    mcp.Dispose();
    CoreFile.Delete(paths.CoreJson);
    CoreLog.Info("Agent Core stopped");
});

CoreLog.Info($"Agent Core starting: port={port} pid={Environment.ProcessId} version={CoreVersion.Value} "
    + $"singleInstance={config.SingleInstance} dataDir={paths.Root} sessionDir={sessions.Directory} "
    + $"provider={models.Current().Codec.Name} model={models.Current().Model}");

try
{
    await app.RunAsync();
}
catch (Exception ex)
{
    CoreLog.Error("Agent Core crashed", ex);
    CoreFile.Delete(paths.CoreJson);
    return 5;
}
finally
{
    http.Dispose();
    instanceLock?.Dispose();
}

CoreFile.Delete(paths.CoreJson);
return 0;

// Cho WebApplicationFactory trong test (tests/core/AxiomOffice.Core.Tests).
public partial class Program;
