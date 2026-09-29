using System.Text;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Agent;
using AxiomOffice.Core.Logging;
using AxiomOffice.Core.Memory;

namespace AxiomOffice.Core.Api;

// Endpoint cua luot chay va hoi thoai (New_arch.md muc 7.3, 7.4):
// POST /v1/runs, GET /v1/runs/{id}, GET /v1/runs/{id}/events (SSE), POST /v1/runs/{id}/cancel,
// GET /v1/conversations, GET /v1/conversations/{id}, DELETE /v1/conversations/{id}.
public static class RunEndpoints
{
    public static void Map(WebApplication app, RunManager manager, Orchestrator orchestrator, CoreStores stores)
    {
        app.MapPost("/v1/runs", async (HttpContext context) =>
        {
            if (!stores.Available)
            {
                return ApiJson.Error("memory unavailable: " + stores.Error, 503);
            }

            (JsonNode? body, bool parsed) = await ReadJsonAsync(context);
            if (!parsed)
            {
                return ApiJson.Error("invalid JSON body", 400);
            }

            RunRequest? request = RunRequestParser.Parse(body, out string? error);
            if (request == null)
            {
                return ApiJson.Error(error ?? "invalid request", 400);
            }

            (RunState? run, string? startError) = manager.TryStart(request.Port);
            if (run == null)
            {
                return ApiJson.Error(startError ?? "cannot start run", 409);
            }

            run.Work = Task.Run(() => orchestrator.ExecuteAsync(run, request, run.Cancel.Token), CancellationToken.None);

            // Hoi thoai duoc tao ngay dau luot chay: cho toi da 10s de tra ve conversationId nhu tai lieu.
            var watch = System.Diagnostics.Stopwatch.StartNew();
            while (run.ConversationId == null && run.Status == RunStatus.Running && watch.ElapsedMilliseconds < 10_000)
            {
                await Task.Delay(25).ConfigureAwait(false);
            }

            return ApiJson.Ok(new JsonObject
            {
                ["runId"] = run.Id,
                ["conversationId"] = run.ConversationId,
                ["status"] = run.Status,
            });
        });

        app.MapGet("/v1/runs/{runId}", (string runId) =>
        {
            RunState? run = manager.Get(runId);
            return run == null ? ApiJson.Error("unknown run: " + runId, 404) : ApiJson.Ok(run.ToJson());
        });

        app.MapPost("/v1/runs/{runId}/cancel", (string runId) =>
        {
            RunState? run = manager.Get(runId);
            if (run == null)
            {
                return ApiJson.Error("unknown run: " + runId, 404);
            }

            return manager.Cancel(runId)
                ? ApiJson.Ok(new JsonObject { ["cancelled"] = true, ["status"] = run.Status })
                : ApiJson.Error($"run is not running (status {run.Status})", 409);
        });

        // Tra loi yeu cau xac nhan cua policy (New_arch.md 7.3, 8.6): {"confirmationId", "approved": true|false}.
        app.MapPost("/v1/runs/{runId}/confirm", async (HttpContext context, string runId) =>
        {
            RunState? run = manager.Get(runId);
            if (run == null)
            {
                return ApiJson.Error("unknown run: " + runId, 404);
            }

            JsonObject? body;
            try
            {
                body = await System.Text.Json.JsonSerializer.DeserializeAsync<JsonObject>(context.Request.Body, cancellationToken: context.RequestAborted);
            }
            catch (System.Text.Json.JsonException)
            {
                body = null;
            }

            string? id = body?["confirmationId"] is JsonValue idValue && idValue.TryGetValue(out string? text) ? text : null;
            bool? approved = body?["approved"] is JsonValue approvedValue && approvedValue.TryGetValue(out bool flag) ? flag : null;
            if (id == null || approved == null)
            {
                return ApiJson.Error("'confirmationId' and 'approved' (true|false) are required", 400);
            }

            return run.Confirmations.Resolve(id, approved.Value)
                ? ApiJson.Ok(new JsonObject { ["confirmationId"] = id, ["approved"] = approved.Value })
                : ApiJson.Error("no pending confirmation: " + id, 404);
        });

        app.MapGet("/v1/audit", (HttpContext context) =>
        {
            string? runId = context.Request.Query["runId"].FirstOrDefault();
            int limit = ClampLimit(context.Request.Query["limit"].FirstOrDefault(), 100);
            var list = new JsonArray();
            foreach (AuditRow row in stores.Runs.Audit(string.IsNullOrWhiteSpace(runId) ? null : runId, limit))
            {
                list.Add(new JsonObject
                {
                    ["id"] = row.Id,
                    ["runId"] = row.RunId,
                    ["seq"] = row.Seq,
                    ["tool"] = row.Tool,
                    ["action"] = row.Action,
                    ["params"] = row.ParamsJson,
                    ["ok"] = row.Ok,
                    ["error"] = row.Error,
                    ["ms"] = row.Ms,
                    ["createdAt"] = row.CreatedAt,
                });
            }

            return ApiJson.Ok(new JsonObject { ["calls"] = list });
        });

        app.MapGet("/v1/runs/{runId}/events", async (HttpContext context, string runId) =>
        {
            RunState? run = manager.Get(runId);
            if (run == null)
            {
                await ApiJson.WriteErrorAsync(context, 404, "unknown run: " + runId).ConfigureAwait(false);
                return;
            }

            long after = ParseAfter(context);
            context.Response.StatusCode = 200;
            context.Response.ContentType = "text/event-stream; charset=utf-8";
            context.Response.Headers.CacheControl = "no-cache";
            context.Response.Headers["X-Accel-Buffering"] = "no";

            var writer = new StreamWriter(context.Response.Body, new UTF8Encoding(false)) { AutoFlush = false };
            long cursor = after;
            DateTime lastWrite = DateTime.UtcNow;
            try
            {
                while (!context.RequestAborted.IsCancellationRequested)
                {
                    bool wrote = false;
                    foreach (RunEvent item in run.Events.Since(cursor))
                    {
                        cursor = item.Seq;
                        await WriteEventAsync(writer, item).ConfigureAwait(false);
                        wrote = true;
                    }

                    if (run.Events.Completed)
                    {
                        // Doc not su kien cuoi (neu co) roi ket thuc.
                        foreach (RunEvent item in run.Events.Since(cursor))
                        {
                            cursor = item.Seq;
                            await WriteEventAsync(writer, item).ConfigureAwait(false);
                            wrote = true;
                        }

                        break;
                    }

                    if (wrote)
                    {
                        lastWrite = DateTime.UtcNow;
                        continue;
                    }

                    TimeSpan wait = TimeSpan.FromSeconds(15) - (DateTime.UtcNow - lastWrite);
                    if (wait <= TimeSpan.Zero)
                    {
                        await WriteRawAsync(writer, "ping", new JsonObject
                        {
                            ["time"] = DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ"),
                        }).ConfigureAwait(false);
                        lastWrite = DateTime.UtcNow;
                        continue;
                    }

                    await run.Events.WaitForChangeAsync(wait, context.RequestAborted).ConfigureAwait(false);
                }
            }
            catch (Exception ex) when (ex is OperationCanceledException || ex is IOException || ex is ObjectDisposedException)
            {
                // Client ngat ket noi: binh thuong.
            }
            catch (Exception ex)
            {
                CoreLog.Error("SSE run " + runId + " failed", ex);
            }
            finally
            {
                try
                {
                    await writer.FlushAsync(CancellationToken.None).ConfigureAwait(false);
                }
                catch
                {
                }
            }
        });

        app.MapGet("/v1/conversations", (HttpContext context) =>
        {
            string? documentKey = context.Request.Query["documentKey"].FirstOrDefault();
            int limit = ClampLimit(context.Request.Query["limit"].FirstOrDefault(), 20);
            IReadOnlyList<ConversationRow> rows = stores.Conversations.List(
                string.IsNullOrWhiteSpace(documentKey) ? null : documentKey, limit);
            var list = new JsonArray();
            foreach (ConversationRow row in rows)
            {
                list.Add(ConversationJson(row));
            }

            return ApiJson.Ok(new JsonObject { ["conversations"] = list });
        });

        app.MapGet("/v1/conversations/{id}", (string id) =>
        {
            ConversationRow? row = stores.Conversations.Get(id);
            if (row == null)
            {
                return ApiJson.Error("unknown conversation: " + id, 404);
            }

            var messages = new JsonArray();
            foreach (MessageRow message in stores.Conversations.Messages(id))
            {
                messages.Add(new JsonObject
                {
                    ["seq"] = message.Seq,
                    ["role"] = message.Role,
                    ["content"] = message.Content,
                    ["createdAt"] = message.CreatedAt,
                });
            }

            JsonObject json = ConversationJson(row);
            json["messages"] = messages;
            return ApiJson.Ok(json);
        });

        app.MapDelete("/v1/conversations/{id}", (string id) =>
        {
            return stores.Conversations.Delete(id)
                ? ApiJson.Ok(new JsonObject { ["deleted"] = id })
                : ApiJson.Error("unknown conversation: " + id, 404);
        });
    }

    private static async Task<(JsonNode? Body, bool Parsed)> ReadJsonAsync(HttpContext context)
    {
        using var reader = new StreamReader(context.Request.Body, Encoding.UTF8);
        string text = await reader.ReadToEndAsync(context.RequestAborted).ConfigureAwait(false);
        if (string.IsNullOrWhiteSpace(text))
        {
            return (null, true);
        }

        try
        {
            return (JsonNode.Parse(text), true);
        }
        catch
        {
            return (null, false);
        }
    }

    private static long ParseAfter(HttpContext context)
    {
        string? raw = context.Request.Query["after"].FirstOrDefault() ?? context.Request.Headers["Last-Event-ID"].FirstOrDefault();
        return long.TryParse(raw, out long value) && value > 0 ? value : 0;
    }

    private static int ClampLimit(string? raw, int fallback)
    {
        return int.TryParse(raw, out int value) && value > 0 ? Math.Min(value, 200) : fallback;
    }

    private static JsonObject ConversationJson(ConversationRow row)
    {
        return new JsonObject
        {
            ["id"] = row.Id,
            ["documentKey"] = row.DocumentKey,
            ["documentName"] = row.DocumentName,
            ["app"] = row.App,
            ["family"] = row.Family,
            ["title"] = row.Title,
            ["summary"] = row.Summary,
            ["createdAt"] = row.CreatedAt,
            ["updatedAt"] = row.UpdatedAt,
        };
    }

    private static async Task WriteEventAsync(StreamWriter writer, RunEvent item)
    {
        await WriteRawAsync(writer, item.Type, item.Data, item.Seq).ConfigureAwait(false);
    }

    private static async Task WriteRawAsync(StreamWriter writer, string type, JsonNode? data, long seq = 0)
    {
        var json = new JsonObject
        {
            ["seq"] = seq,
            ["type"] = type,
            ["time"] = DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ss.fffZ"),
            ["data"] = data?.DeepClone(),
        };
        if (seq > 0)
        {
            await writer.WriteAsync("id: " + seq + "\n").ConfigureAwait(false);
        }

        await writer.WriteAsync("event: " + type + "\n").ConfigureAwait(false);
        await writer.WriteAsync("data: " + json.ToJsonString() + "\n\n").ConfigureAwait(false);
        await writer.FlushAsync(CancellationToken.None).ConfigureAwait(false);
    }
}
