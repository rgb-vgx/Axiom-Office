using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;

namespace AxiomOffice.Core.Api;

// Endpoint cua Core API v1 (New_arch.md muc 7.3). Phase 0 moi co /health + /v1/admin/shutdown;
// cac endpoint run/skill/memory them o giai doan sau.
public static class CoreApi
{
    public static void Map(WebApplication app, CoreConfig config, CorePaths paths, CoreRuntime runtime)
    {
        // Khong can token: add-in va script dung de kiem tra Core con song.
        app.MapGet("/health", () => ApiJson.Ok(new Dictionary<string, object?>
        {
            ["app"] = "core",
            ["pid"] = Environment.ProcessId,
            ["port"] = runtime.Port,
            ["version"] = CoreVersion.Value,
            ["protocol"] = CoreVersion.Protocol,
            ["started"] = runtime.StartedUtc.ToString("yyyy-MM-ddTHH:mm:ss.fffZ"),
            ["uptimeSeconds"] = Math.Round(runtime.UptimeSeconds, 1),
            ["dataDir"] = paths.Root,
            ["memory"] = config.MemoryEnabled ? "on" : "off",
        }));

        // Dung boi build.ps1/uninstall.ps1 truoc khi ghi de file.
        app.MapPost("/v1/admin/shutdown", (IHostApplicationLifetime lifetime) =>
        {
            CoreLog.Info("shutdown requested via API");
            _ = Task.Run(async () =>
            {
                // Cho response di xong roi moi dung (neu dung ngay, client co the nhan loi ket noi).
                await Task.Delay(150);
                lifetime.StopApplication();
            });
            return ApiJson.Ok(new Dictionary<string, object?> { ["stopping"] = true });
        });
    }
}
