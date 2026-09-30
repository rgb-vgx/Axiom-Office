using System.Diagnostics;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Setup;
using AxiomOffice.Setup;
using AxiomOffice.Core.Skills;

namespace AxiomOffice.Core.Api;

// API cho wizard thiet lap (SetupWizardForm.cs cua add-in Windows va setupwizard.py cua extension LibreOffice):
//   GET  /v1/setup       preset nha cung cap + mo ta tinh nang + tinh trang hien tai cua Core
//   POST /v1/llm/test    thu ket noi voi gia tri NGUOI DUNG VUA NHAP (chua luu) -> loi da dich sang tieng Viet
//   GET  /v1/llm/models  danh sach model cua may chu de nguoi dung chon thay vi tu go ten
// Moi endpoint deu qua CoreApiGuard (token + chan Origin). KHONG ghi khoa API vao log.
public static class SetupEndpoints
{
    // Cau hoi rat ngan de tiet kiem token: chi can biet may chu co tra loi dung dinh dang khong.
    private const string TestSystemPrompt = "Bạn là trợ lý kiểm tra kết nối. Trả lời cực ngắn.";
    private const string TestUserPrompt = "Trả lời đúng một từ: OK";
    private const int TestTimeoutMs = 30_000;
    private const int TestMaxTokens = 24;

    public static void Map(WebApplication app, CoreConfig config, CorePaths paths, CoreRuntime runtime, HttpClient http,
        SkillIndex skills, CoreStores stores)
    {
        app.MapGet("/v1/setup", () => ApiJson.Ok(Payload(config, paths, runtime, skills, stores)));

        app.MapPost("/v1/llm/test", async (HttpContext context) =>
        {
            (JsonNode? body, bool parsed) = await ReadJsonAsync(context);
            if (!parsed)
            {
                return ApiJson.Error("invalid JSON body", 400);
            }

            AdHocRequest request = AdHocRequest.From(body) with { NeedsModel = true };
            LlmErrors.LlmError error = await TestAsync(http, config, request, context.RequestAborted);
            if (error.Kind == "ok")
            {
                CoreLog.Info($"setup: llm test ok provider={request.Codec} model={request.Model} seconds={error.Detail}");
                return ApiJson.Ok(new JsonObject
                {
                    ["seconds"] = double.TryParse(error.Detail, out double seconds) ? Math.Round(seconds, 2) : 0,
                    ["model"] = request.Model,
                    ["provider"] = request.Codec,
                    ["reply"] = Trim(error.Message, 200),
                });
            }

            CoreLog.Info($"setup: llm test failed kind={error.Kind} provider={request.Codec} model={request.Model} detail={error.Detail}");
            return ApiJson.Ok(ErrorJson(error));
        });

        app.MapGet("/v1/llm/models", async (HttpContext context) =>
        {
            AdHocRequest request = AdHocRequest.From(null, context.Request.Query) with { NeedsModel = false };
            if (request.Missing != null)
            {
                return ApiJson.Ok(ErrorJson(LlmErrors.Describe(request.Missing, request.ProviderId, request.Endpoint, request.Model)));
            }

            (List<string>? models, string? error) = await ModelCatalog.ListAsync(
                http, request.Codec, request.Endpoint, request.ApiKey, context.RequestAborted);
            if (models == null)
            {
                LlmErrors.LlmError failure = LlmErrors.Describe(error, request.ProviderId, request.Endpoint, request.Model);
                CoreLog.Info($"setup: list models failed kind={failure.Kind} detail={failure.Detail}");
                return ApiJson.Ok(ErrorJson(failure));
            }

            var list = new JsonArray();
            foreach (string name in models)
            {
                list.Add(name);
            }

            CoreLog.Info($"setup: list models ok provider={request.Codec} count={models.Count}");
            return ApiJson.Ok(new JsonObject { ["models"] = list, ["count"] = models.Count });
        });
    }

    private static JsonObject ErrorJson(LlmErrors.LlmError error)
    {
        return new JsonObject
        {
            ["kind"] = error.Kind,
            ["message"] = error.Message,
            ["hint"] = error.Hint,
            ["detail"] = error.Detail,
        };
    }

    // Tham so cua mot lan thu: tu body JSON (POST) hoac query string (GET).
    // Bo trong apiKey = dung khoa da luu (de "Kiem tra ket noi" chay duoc khi nguoi dung khong nhap lai khoa).
    private sealed record AdHocRequest
    {
        public string ProviderId { get; init; } = "company";

        public string Codec { get; init; } = "openai";

        public string Endpoint { get; init; } = "";

        public string Model { get; init; } = "";

        public string ApiKey { get; init; } = "";

        public bool NeedsModel { get; init; }

        public string? Missing => Endpoint.Length == 0 ? "Endpoint is not configured"
            : NeedsModel && Model.Length == 0 ? "Model is not configured"
            : null;

        public static AdHocRequest From(JsonNode? body, IQueryCollection? query = null)
        {
            string Read(string name)
            {
                if (body?[name] is JsonNode node && node.GetValueKind() == JsonValueKind.String)
                {
                    return node.GetValue<string>() ?? "";
                }

                return query?[name].FirstOrDefault() ?? "";
            }

            string endpoint = Read("endpoint").Trim();
            string codec = Read("provider").Trim();
            string providerId = Read("providerId").Trim();
            if (providerId.Length == 0)
            {
                providerId = SetupCatalog.GuessProviderId(endpoint, codec);
            }

            if (codec.Length == 0)
            {
                codec = SetupCatalog.FindProvider(providerId)?.Codec ?? "openai";
            }

            return new AdHocRequest
            {
                ProviderId = providerId,
                Codec = codec,
                Endpoint = endpoint,
                Model = Read("model").Trim(),
                ApiKey = Read("apiKey").Trim(),
            };
        }
    }

    private static async Task<LlmErrors.LlmError> TestAsync(HttpClient http, CoreConfig config, AdHocRequest request, CancellationToken cancel)
    {
        if (request.Missing != null)
        {
            return LlmErrors.Describe(request.Missing, request.ProviderId, request.Endpoint, request.Model);
        }

        string apiKey = request.ApiKey.Length > 0 ? request.ApiKey : config.LlmApiKey;
        var client = new ModelClient(http, request.Codec, request.Endpoint, apiKey, request.Model)
        {
            RequestTimeoutMs = TestTimeoutMs,
            RetryDelays = [],
        };

        var watch = Stopwatch.StartNew();
        (string? text, string? error) = await client.ChatAsync(TestSystemPrompt, TestUserPrompt, cancel, TestMaxTokens);
        double seconds = watch.Elapsed.TotalSeconds;
        if (text == null)
        {
            return LlmErrors.Describe(error, request.ProviderId, request.Endpoint, request.Model);
        }

        string reply = text.Trim();
        return new LlmErrors.LlmError("ok", reply.Length == 0 ? "OK" : reply, "", seconds.ToString("0.0"));
    }

    private static JsonObject Payload(CoreConfig config, CorePaths paths, CoreRuntime runtime, SkillIndex skills, CoreStores stores)
    {
        var presets = new JsonArray();
        foreach (SetupProviderInfo provider in SetupCatalog.Providers)
        {
            var models = new JsonArray();
            foreach (string model in provider.SuggestedModels)
            {
                models.Add(model);
            }

            presets.Add(new JsonObject
            {
                ["id"] = provider.Id,
                ["label"] = provider.Label,
                ["description"] = provider.Description,
                ["endpoint"] = provider.Endpoint,
                ["needsKey"] = provider.NeedsKey,
                ["keyUrl"] = provider.KeyUrl,
                ["codec"] = provider.Codec,
                ["suggestedModels"] = models,
            });
        }

        var steps = new JsonArray();
        foreach (SetupStepInfo step in SetupCatalog.Steps)
        {
            steps.Add(new JsonObject { ["id"] = step.Id, ["title"] = step.Title, ["subtitle"] = step.Subtitle });
        }

        var features = new JsonArray();
        foreach (SetupFeatureInfo feature in SetupCatalog.Features)
        {
            features.Add(new JsonObject
            {
                ["key"] = feature.Key,
                ["label"] = feature.Label,
                ["description"] = feature.Description,
                ["recommended"] = feature.Recommended,
            });
        }

        var checks = new JsonArray();
        foreach (SetupCheckInfo check in SetupCatalog.Checks)
        {
            checks.Add(check.Id);
        }

        // problems: danh sach viec can lam, de wizard hien o buoc "Kiem tra may" (id khop catalog/setup.json).
        var problems = new JsonArray();
        if (config.LlmEndpoint.Length == 0 || config.LlmModel.Length == 0)
        {
            problems.Add(Problem("config", "error",
                "Chưa cấu hình AI", "Mở bước \"Kết nối máy chủ AI\" để nhập địa chỉ máy chủ và model."));
        }

        if (config.Token.Length == 0)
        {
            problems.Add(Problem("token", "warning",
                "Chưa có khoá bảo vệ giữa ứng dụng và Agent Core", "Bấm \"Tạo khoá mới\" rồi khởi động lại Core."));
        }

        if (!stores.Available)
        {
            problems.Add(Problem("memory", "warning",
                "Không mở được dữ liệu ghi nhớ", stores.Error ?? "Kiểm tra ổ đĩa và thư mục dữ liệu."));
        }

        return new JsonObject
        {
            ["version"] = CoreVersion.Value,
            ["problems"] = problems,
            ["presets"] = presets,
            ["steps"] = steps,
            ["features"] = features,
            ["checks"] = checks,
            ["providerId"] = SetupCatalog.GuessProviderId(config.LlmEndpoint, config.LlmProvider),
            ["current"] = new JsonObject
            {
                ["provider"] = config.LlmProvider,
                ["endpoint"] = config.LlmEndpoint,
                ["model"] = config.LlmModel,
                ["hasKey"] = config.LlmApiKey.Length > 0,
                ["memoryEnabled"] = config.MemoryEnabled,
                ["memoryAutoExtract"] = config.MemoryAutoExtract,
                ["visualQaEnabled"] = config.VisualQaEnabled,
                ["tokenSet"] = config.Token.Length > 0,
                ["configured"] = config.LlmEndpoint.Length > 0 && config.LlmModel.Length > 0,
            },
            ["core"] = new JsonObject
            {
                ["pid"] = Environment.ProcessId,
                ["port"] = runtime.Port,
                ["dataDir"] = paths.Root,
                ["logFile"] = paths.LogFile,
                ["skills"] = skills.All.Count,
                ["skillErrors"] = skills.Errors.Count,
            },
        };
    }

    private static JsonObject Problem(string id, string severity, string label, string hint)
    {
        SetupCheckInfo? meta = SetupCatalog.FindCheck(id);
        return new JsonObject
        {
            ["id"] = id,
            ["severity"] = severity,
            ["label"] = label,
            ["hint"] = hint,
            ["fixable"] = meta?.Fixable ?? false,
            ["fixLabel"] = meta?.FixLabel ?? "",
        };
    }

    private static string Trim(string text, int limit)
    {
        return text.Length <= limit ? text : text[..limit];
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
}
