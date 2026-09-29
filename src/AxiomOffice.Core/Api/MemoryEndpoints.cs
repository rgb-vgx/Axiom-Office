using System.Text.Json;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Memory;

namespace AxiomOffice.Core.Api;

// API memory (New_arch.md muc 8.5.11). Moi thay doi o day la cua NGUOI DUNG (form "Quan ly ghi nho", API).
public static class MemoryEndpoints
{
    public static void Map(WebApplication app, MemoryService memory)
    {
        app.MapGet("/v1/memory", async (HttpContext context) =>
        {
            if (!memory.Available)
            {
                return ApiJson.Error("memory unavailable", 503);
            }

            string? scope = Query(context, "scope");
            string? scopeKey = Query(context, "scopeKey") ?? Query(context, "documentKey");
            string? q = Query(context, "q");
            string? runId = Query(context, "runId");
            bool includeDeleted = Query(context, "includeDeleted") is "1" or "true";
            if (!string.IsNullOrWhiteSpace(q))
            {
                IReadOnlyList<MemoryHit> hits = await memory.SearchAsync(q, scopeKey, scope, 50, context.RequestAborted);
                var found = new JsonArray();
                foreach (MemoryHit hit in hits)
                {
                    JsonObject json = Item(hit.Item);
                    json["score"] = Math.Round(hit.Score, 4);
                    found.Add(json);
                }

                return ApiJson.Ok(new JsonObject { ["memories"] = found });
            }

            var list = new JsonArray();
            foreach (MemoryItem item in memory.Store.List(scope, scopeKey, includeDeleted, runId))
            {
                list.Add(Item(item));
            }

            return ApiJson.Ok(new JsonObject { ["memories"] = list });
        });

        app.MapPost("/v1/memory", async (HttpContext context) =>
        {
            JsonObject? body = await ReadBody(context);
            string scope = Str(body?["scope"]) ?? MemoryScopes.User;
            string? text = Str(body?["text"]);
            if (!MemoryScopes.IsValid(scope) || string.IsNullOrWhiteSpace(text))
            {
                return ApiJson.Error("'scope' (user|document|skill) and 'text' are required", 400);
            }

            string? scopeKey = Str(body?["scopeKey"]) ?? Str(body?["documentKey"]);
            if (scope != MemoryScopes.User && string.IsNullOrWhiteSpace(scopeKey))
            {
                return ApiJson.Error("'scopeKey' is required for scope " + scope, 400);
            }

            var fact = new NewMemory(scope, scopeKey, text, Str(body?["category"]), ExpiresAt: Str(body?["expiresAt"]));
            MemoryOpResult result = (await memory.AddAsync([fact], MemoryActors.User, null, context.RequestAborted))[0];
            if (result.Event == "SKIPPED")
            {
                return ApiJson.Error("not saved: " + result.Reason, 400);
            }

            return ApiJson.Ok(new JsonObject { ["event"] = result.Event, ["memory"] = result.Item == null ? null : Item(result.Item) });
        });

        app.MapMethods("/v1/memory/{id}", ["PATCH"], async (string id, HttpContext context) =>
        {
            JsonObject? body = await ReadBody(context);
            try
            {
                bool clearExpires = body != null && body.ContainsKey("expiresAt") && body["expiresAt"] == null;
                bool? pinned = body?["pinned"] is JsonValue value && value.TryGetValue(out bool pin) ? pin : null;
                MemoryItem? item = memory.Store.Update(id, Str(body?["text"]), Str(body?["category"]), Str(body?["expiresAt"]),
                    clearExpires, pinned, Str(body?["reason"]));
                return item == null ? ApiJson.Error("unknown memory: " + id, 404) : ApiJson.Ok(Item(item));
            }
            catch (ArgumentException ex)
            {
                return ApiJson.Error(ex.Message, 400);
            }
        });

        app.MapDelete("/v1/memory/{id}", (string id, HttpContext context) =>
        {
            return memory.Store.Delete(id, Query(context, "reason"))
                ? ApiJson.Ok(new JsonObject { ["deleted"] = id })
                : ApiJson.Error("unknown memory: " + id, 404);
        });

        app.MapPost("/v1/memory/{id}/restore", (string id) =>
        {
            return memory.Store.Restore(id) ? ApiJson.Ok(new JsonObject { ["restored"] = id }) : ApiJson.Error("not a deleted memory: " + id, 404);
        });

        app.MapGet("/v1/memory/{id}/history", (string id) =>
        {
            var list = new JsonArray();
            foreach (MemoryHistoryEntry entry in memory.Store.History(id))
            {
                list.Add(new JsonObject
                {
                    ["event"] = entry.Event,
                    ["oldText"] = entry.OldText,
                    ["newText"] = entry.NewText,
                    ["actor"] = entry.Actor,
                    ["runId"] = entry.RunId,
                    ["reason"] = entry.Reason,
                    ["createdAt"] = entry.CreatedAt,
                });
            }

            return ApiJson.Ok(new JsonObject { ["history"] = list });
        });

        // Xoa cung toan bo: phai co ?scope=all&confirm=true (UI hoi lai truoc).
        app.MapDelete("/v1/memory", (HttpContext context) =>
        {
            string? scope = Query(context, "scope");
            if (Query(context, "confirm") != "true" || scope == null || (scope != "all" && !MemoryScopes.IsValid(scope)))
            {
                return ApiJson.Error("use ?scope=all|user|document&confirm=true", 400);
            }

            int removed = memory.Store.Purge(scope == "all" ? null : scope);
            return ApiJson.Ok(new JsonObject { ["removed"] = removed });
        });
    }

    public static JsonObject Item(MemoryItem item)
    {
        var entities = new JsonArray();
        foreach (string entity in item.Entities)
        {
            entities.Add(entity);
        }

        var links = new JsonArray();
        foreach (string link in item.Links)
        {
            links.Add(link);
        }

        return new JsonObject
        {
            ["id"] = item.Id,
            ["scope"] = item.Scope,
            ["scopeKey"] = item.ScopeKey,
            ["text"] = item.Text,
            ["category"] = item.Category,
            ["entities"] = entities,
            ["linked"] = links,
            ["expiresAt"] = item.ExpiresAt,
            ["source"] = item.Source,
            ["confidence"] = item.Confidence,
            ["pinned"] = item.Pinned,
            ["hits"] = item.Hits,
            ["lastUsedAt"] = item.LastUsedAt,
            ["createdAt"] = item.CreatedAt,
            ["updatedAt"] = item.UpdatedAt,
            ["deletedAt"] = item.DeletedAt,
            ["runId"] = item.CreatedRunId,
        };
    }

    private static string? Query(HttpContext context, string name)
    {
        string? value = context.Request.Query[name].FirstOrDefault();
        return string.IsNullOrWhiteSpace(value) ? null : value.Trim();
    }

    private static string? Str(JsonNode? node) => node is JsonValue value && value.TryGetValue(out string? text) ? text : null;

    private static async Task<JsonObject?> ReadBody(HttpContext context)
    {
        try
        {
            return await JsonSerializer.DeserializeAsync<JsonObject>(context.Request.Body, cancellationToken: context.RequestAborted);
        }
        catch (JsonException)
        {
            return null;
        }
    }
}
