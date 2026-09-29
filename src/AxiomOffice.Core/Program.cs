using AxiomOffice.Core;
using AxiomOffice.Core.Api;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;

// AxiomOffice.Core.exe - Agent Core (New_arch.md): process rieng cua agent, mot ban cho moi nguoi
// dung Windows, chi nghe 127.0.0.1. Giai doan 0: vong doi + /health + shutdown + cau hinh.
// Add-in khoi dong Core khi can va tim no qua %LOCALAPPDATA%\AxiomOffice\core.json.

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
CoreApi.Map(app, config, paths, runtime);

// core.json chi duoc ghi khi Core da san sang; xoa khi thoat (dung ApplicationStopped de ca khi
// host bi dung boi WebApplicationFactory trong test cung khong de lai file).
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
        + $"protocol={CoreVersion.Protocol} dataDir={paths.Root} memory={(config.MemoryEnabled ? "on" : "off")}");
});

app.Lifetime.ApplicationStopped.Register(() =>
{
    CoreFile.Delete(paths.CoreJson);
    CoreLog.Info("Agent Core stopped");
});

CoreLog.Info($"Agent Core starting: port={port} pid={Environment.ProcessId} version={CoreVersion.Value} "
    + $"singleInstance={config.SingleInstance} dataDir={paths.Root}");

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
    instanceLock?.Dispose();
}

CoreFile.Delete(paths.CoreJson);
return 0;

// Cho WebApplicationFactory trong test (tests/core/AxiomOffice.Core.Tests).
public partial class Program;
