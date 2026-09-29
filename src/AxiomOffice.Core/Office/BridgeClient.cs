using System.Diagnostics;
using System.Text;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;

namespace AxiomOffice.Core.Office;

// Ket qua mot lan goi bridge. RawJson la body nguyen van - dung lam ket qua tra cho model
// (giong executor cua add-in, model doc duoc ca ok lan error).
public sealed record BridgeResult(bool Ok, string RawJson, JsonNode? Result, string? Error, int Status, long Ms);

// Mot lenh trong registry cua DLL dang chay (GET /commands).
public sealed record OfficeCommandParam(string Name, bool Required, string? Hint);

public sealed record OfficeCommand(string Name, string? Kind, bool Agent, string Summary, IReadOnlyList<OfficeCommandParam> Params);

public sealed record OfficeCommandCatalog(string Version, IReadOnlyList<OfficeCommand> Commands);

// Goi HTTP vao bridge trong app (127.0.0.1:port). Token lay tu cau hinh (HKCU), chi gui trong header.
public sealed class BridgeClient(CoreConfig config, HttpClient http, Action<string>? log = null)
{
    public const int DefaultCommandTimeoutMs = 90_000;   // wpp.exportPdf / writer.exportPdf co the lau
    public const int HealthTimeoutMs = 5_000;

    private readonly Dictionary<int, (string Version, OfficeCommandCatalog Catalog, DateTime FetchedAt)> _catalogs = [];

    // Gui mot lenh POST /cmd. Khong nem loi giao thuc: tra Ok=false kem RawJson de model tu xu ly.
    public async Task<BridgeResult> CommandAsync(int port, string action, JsonNode? parameters, CancellationToken cancel, int timeoutMs = DefaultCommandTimeoutMs)
    {
        var body = new JsonObject
        {
            ["action"] = action,
            ["params"] = parameters?.DeepClone() ?? new JsonObject(),
        };

        return await SendAsync(port, "/cmd", body, cancel, timeoutMs).ConfigureAwait(false);
    }

    public async Task<JsonNode?> HealthAsync(int port, CancellationToken cancel, int timeoutMs = HealthTimeoutMs)
    {
        BridgeResult result = await SendAsync(port, "/health", null, cancel, timeoutMs).ConfigureAwait(false);
        return result.Ok ? result.Result : null;
    }

    // Bo lenh cua DLL dang chay, cache theo version (muc 7.5): Core dung de dung tool cho agent.
    public async Task<OfficeCommandCatalog?> GetCommandsAsync(int port, CancellationToken cancel, bool force = false)
    {
        if (!force && _catalogs.TryGetValue(port, out (string Version, OfficeCommandCatalog Catalog, DateTime FetchedAt) cached)
            && DateTime.UtcNow - cached.FetchedAt < TimeSpan.FromMinutes(5))
        {
            return cached.Catalog;
        }

        BridgeResult result = await SendAsync(port, "/commands", null, cancel, 15_000).ConfigureAwait(false);
        if (!result.Ok || result.Result == null)
        {
            log?.Invoke($"GET /commands port {port} failed: {result.Error ?? "unknown"}");
            return null;
        }

        string version = result.Result["version"]?.GetValue<string>() ?? "";
        var commands = new List<OfficeCommand>();
        if (result.Result["commands"] is JsonArray list)
        {
            foreach (JsonNode? item in list)
            {
                if (item == null)
                {
                    continue;
                }

                var parameters = new List<OfficeCommandParam>();
                if (item["params"] is JsonArray paramList)
                {
                    foreach (JsonNode? p in paramList)
                    {
                        if (p == null)
                        {
                            continue;
                        }

                        parameters.Add(new OfficeCommandParam(
                            p["name"]?.GetValue<string>() ?? "",
                            p["required"]?.GetValue<bool>() ?? false,
                            p["hint"]?.GetValue<string>()));
                    }
                }

                commands.Add(new OfficeCommand(
                    item["name"]?.GetValue<string>() ?? "",
                    item["kind"]?.GetValue<string>(),
                    item["agent"]?.GetValue<bool>() ?? false,
                    item["summary"]?.GetValue<string>() ?? "",
                    parameters));
            }
        }

        var catalog = new OfficeCommandCatalog(version, commands);
        _catalogs[port] = (version, catalog, DateTime.UtcNow);
        return catalog;
    }

    private async Task<BridgeResult> SendAsync(int port, string path, JsonObject? body, CancellationToken cancel, int timeoutMs)
    {
        var watch = Stopwatch.StartNew();
        string url = $"http://127.0.0.1:{port}{path}";
        using var timeoutCts = CancellationTokenSource.CreateLinkedTokenSource(cancel);
        timeoutCts.CancelAfter(timeoutMs);
        try
        {
            using var request = new HttpRequestMessage(body == null ? HttpMethod.Get : HttpMethod.Post, url);
            if (body != null)
            {
                request.Content = new StringContent(body.ToJsonString(), Encoding.UTF8, "application/json");
            }

            if (!string.IsNullOrEmpty(config.Token))
            {
                request.Headers.Add("X-Auth-Token", config.Token);
            }

            using HttpResponseMessage response = await http.SendAsync(request, timeoutCts.Token).ConfigureAwait(false);
            string text = await response.Content.ReadAsStringAsync(CancellationToken.None).ConfigureAwait(false);
            watch.Stop();

            JsonNode? parsed = null;
            try
            {
                parsed = JsonNode.Parse(text);
            }
            catch
            {
                // Khong phai JSON: tra nguyen van de con chan doan.
            }

            bool ok = parsed?["ok"]?.GetValue<bool>() ?? false;
            JsonNode? result = parsed?["result"];
            string? error = parsed?["error"]?.GetValue<string>();
            if (!ok && error == null)
            {
                error = $"HTTP {(int)response.StatusCode}";
            }

            return new BridgeResult(ok, text, result, error, (int)response.StatusCode, watch.ElapsedMilliseconds);
        }
        catch (OperationCanceledException) when (!cancel.IsCancellationRequested)
        {
            watch.Stop();
            CoreLog.Info($"Bridge call {path} port {port} timed out after {timeoutMs}ms");
            return new BridgeResult(false, JsonError($"bridge did not answer within {timeoutMs / 1000}s"),
                null, "bridge timeout", 0, watch.ElapsedMilliseconds);
        }
        catch (Exception ex)
        {
            watch.Stop();
            string message = ex is OperationCanceledException ? "cancelled" : ex.GetType().Name + ": " + ex.Message;
            return new BridgeResult(false, JsonError(message), null, message, 0, watch.ElapsedMilliseconds);
        }
    }

    private static string JsonError(string message)
    {
        return new JsonObject { ["ok"] = false, ["error"] = message }.ToJsonString();
    }
}
