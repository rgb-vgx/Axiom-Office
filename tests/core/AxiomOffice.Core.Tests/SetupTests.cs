using System.Net;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Setup;
using AxiomOffice.Setup;

namespace AxiomOffice.Core.Tests;

// Wizard thiet lap: dich loi LLM sang cau tieng Viet (Setup/LlmErrors.cs) va doc danh sach model
// (Models/ModelCatalog.cs) - hai thu nguoi dung khong chuyen nhin thay truc tiep.
public class LlmErrorsTests
{
    private static LlmErrors.LlmError Describe(string error, string provider = "company") => LlmErrors.Describe(error, provider);

    [Theory]
    [InlineData("Endpoint is not configured", "config", "địa chỉ")]
    [InlineData("Model is not configured", "config", "model")]
    [InlineData("HTTP 401: {\"error\":{\"message\":\"invalid api key\"}}", "auth", "khoá truy cập")]
    [InlineData("HTTP 403: forbidden", "forbidden", "không được phép")]
    [InlineData("HTTP 404: {\"error\":\"unknown url\"}", "notFound", "Không tìm thấy máy chủ")]
    [InlineData("HTTP 404: {\"error\":{\"message\":\"model `gpt-9` does not exist\"}}", "model", "không có model")]
    [InlineData("HTTP 400: {\"error\":{\"message\":\"The model `gpt-9` does not exist\"}}", "model", "không có model")]
    [InlineData("HTTP 429: rate limited", "rate", "giới hạn tốc độ")]
    [InlineData("HTTP 503: high demand", "server", "đang lỗi")]
    [InlineData("request timed out after 30s", "timeout", "không trả lời")]
    [InlineData("HttpRequestException: Connection refused (127.0.0.1:9)", "network", "Không kết nối được")]
    [InlineData("HttpRequestException: No such host is known.", "dns", "Không tìm thấy địa chỉ")]
    [InlineData("unexpected provider response (no choices[0].message)", "reply", "không giống một máy chủ chat")]
    [InlineData("provider returned an empty reply", "reply", "không giống một máy chủ chat")]
    [InlineData("something else entirely", "internal", "Không kiểm tra được")]
    public void Dich_loi_thanh_cau_tieng_viet(string error, string kind, string fragment)
    {
        LlmErrors.LlmError described = Describe(error);

        Assert.Equal(kind, described.Kind);
        Assert.Contains(fragment, described.Message + " " + described.Hint);
        Assert.Equal(error, described.Detail);          // giu nguyen van de gui ho tro
        Assert.False(string.IsNullOrWhiteSpace(described.Hint));
    }

    [Fact]
    public void Goi_y_lay_khoa_theo_nha_cung_cap()
    {
        Assert.Contains("platform.openai.com", Describe("HTTP 401: bad key", "openai").Hint);
        Assert.Contains("console.anthropic.com", Describe("HTTP 401: bad key", "anthropic").Hint);
        Assert.Contains("quản trị viên", Describe("HTTP 401: bad key", "company").Hint);
    }

    [Fact]
    public void Loi_rong_tra_ve_internal()
    {
        Assert.Equal("internal", Describe("").Kind);
    }
}

public class ModelCatalogTests
{
    [Fact]
    public void Doc_danh_sach_model_kieu_openai_va_anthropic()
    {
        Assert.Equal(["gpt-4o", "gpt-4o-mini"],
            ModelCatalog.Parse("""{"object":"list","data":[{"id":"gpt-4o-mini"},{"id":"gpt-4o"},{"id":"gpt-4o"}]}"""));
        Assert.Equal(["claude-sonnet-5"],
            ModelCatalog.Parse("""{"data":[{"id":"claude-sonnet-5","display_name":"Claude Sonnet 5"}]}"""));
    }

    [Fact]
    public void Bo_qua_json_hong_hoac_khong_co_data()
    {
        Assert.Empty(ModelCatalog.Parse("khong phai json"));
        Assert.Empty(ModelCatalog.Parse("""{"object":"list"}"""));
        Assert.Empty(ModelCatalog.Parse("""{"data":[{"object":"model"},{"id":"   "}]}"""));
    }

    [Fact]
    public async Task Goi_models_dung_duong_dan_va_header()
    {
        var handler = new ScriptedHandler((request, body) => ScriptedHandler.Json("""{"data":[{"id":"m1"}]}"""));
        using var http = new HttpClient(handler);

        (List<string>? models, string? error) = await ModelCatalog.ListAsync(http, "openai", "http://may-chu/v1", "sk-1", CancellationToken.None);

        Assert.Null(error);
        Assert.Equal(["m1"], models);
        Assert.Equal("http://may-chu/v1/models", handler.Requests[0].Url);
        Assert.Equal("Bearer", handler.LastAuthorizationScheme);
        Assert.Null(handler.LastApiKeyHeader);
    }

    [Fact]
    public async Task Anthropic_dung_x_api_key()
    {
        var handler = new ScriptedHandler((request, body) => ScriptedHandler.Json("""{"data":[{"id":"m1"}]}"""));
        using var http = new HttpClient(handler);

        await ModelCatalog.ListAsync(http, "anthropic", "https://api.anthropic.com/v1", "sk-ant", CancellationToken.None);

        Assert.Equal("sk-ant", handler.LastApiKeyHeader);
        Assert.Equal("2023-06-01", handler.LastAnthropicVersion);
    }

    [Fact]
    public async Task Loi_http_tra_ve_chuoi_de_LlmErrors_dich_duoc()
    {
        var handler = new ScriptedHandler((request, body) => ScriptedHandler.Json("""{"error":"nope"}""", HttpStatusCode.Unauthorized));
        using var http = new HttpClient(handler);

        (List<string>? models, string? error) = await ModelCatalog.ListAsync(http, "openai", "http://may-chu/v1", "sk-sai", CancellationToken.None);

        Assert.Null(models);
        Assert.StartsWith("HTTP 401:", error);
        Assert.Equal("auth", LlmErrors.Describe(error, "openai").Kind);
    }

    [Fact]
    public async Task Thieu_dia_chi_thi_bao_ngay_khong_goi_mang()
    {
        var handler = new ScriptedHandler((request, body) => ScriptedHandler.Json("{}"));
        using var http = new HttpClient(handler);

        (List<string>? models, string? error) = await ModelCatalog.ListAsync(http, "openai", "", "", CancellationToken.None);

        Assert.Null(models);
        Assert.Equal("Endpoint is not configured", error);
        Assert.Equal(0, handler.Count);
    }
}

// Catalog sinh tu catalog/setup.json phai du de UI khong phai tu viet chu: hai UI doc cung du lieu nay.
public class SetupCatalogTests
{
    [Fact]
    public void Preset_co_du_nha_cung_cap_va_moi_preset_hop_le()
    {
        Assert.True(SetupCatalog.Providers.Count >= 4);
        Assert.Equal(["company", "openai", "anthropic", "gemini"], SetupCatalog.Providers.Select(p => p.Id));

        foreach (SetupProviderInfo provider in SetupCatalog.Providers)
        {
            Assert.False(string.IsNullOrWhiteSpace(provider.Label));
            Assert.False(string.IsNullOrWhiteSpace(provider.Description));
            Assert.Contains(provider.Codec, new[] { "openai", "anthropic" });
            if (provider.Group == "public")
            {
                Assert.StartsWith("https://", provider.Endpoint);
                Assert.True(provider.NeedsKey);
                Assert.StartsWith("https://", provider.KeyUrl);
            }

            Assert.Equal(provider.SuggestedModels.Length, provider.SuggestedModels.Distinct(StringComparer.Ordinal).Count());
        }
    }

    [Fact]
    public void Moi_buoc_va_moi_tinh_nang_deu_co_chu_tieng_Viet()
    {
        Assert.Equal(["welcome", "checks", "connect", "features", "done"], SetupCatalog.Steps.Select(s => s.Id));
        foreach (SetupStepInfo step in SetupCatalog.Steps)
        {
            Assert.False(string.IsNullOrWhiteSpace(step.Title));
            Assert.False(string.IsNullOrWhiteSpace(step.Subtitle));
        }

        Assert.Equal(["MemoryEnabled", "MemoryAutoExtract", "VisualQaEnabled"], SetupCatalog.Features.Select(f => f.Key));
        foreach (SetupFeatureInfo feature in SetupCatalog.Features)
        {
            Assert.False(string.IsNullOrWhiteSpace(feature.Label));
            Assert.False(string.IsNullOrWhiteSpace(feature.Description));
        }
    }

    [Fact]
    public void Moi_check_deu_co_nhan_va_check_sua_duoc_thi_co_nhan_nut()
    {
        // "core", "config", "token" la ba id ma Core tra ve trong `problems` cua GET /v1/setup.
        Assert.Contains(SetupCatalog.Checks, c => c.Id == "core");
        Assert.Contains(SetupCatalog.Checks, c => c.Id == "config");
        Assert.Contains(SetupCatalog.Checks, c => c.Id == "token");

        foreach (SetupCheckInfo check in SetupCatalog.Checks)
        {
            Assert.False(string.IsNullOrWhiteSpace(check.Label));
            Assert.False(string.IsNullOrWhiteSpace(check.Help));
            if (check.Fixable)
            {
                Assert.False(string.IsNullOrWhiteSpace(check.FixLabel));
            }
        }
    }

    [Fact]
    public void Doan_nha_cung_cap_tu_dia_chi()
    {
        Assert.Equal("openai", SetupCatalog.GuessProviderId("https://api.openai.com/v1", "openai"));
        Assert.Equal("anthropic", SetupCatalog.GuessProviderId("https://api.anthropic.com/v1", "anthropic"));
        Assert.Equal("gemini", SetupCatalog.GuessProviderId("https://generativelanguage.googleapis.com/v1beta/openai", "openai"));
        Assert.Equal("company", SetupCatalog.GuessProviderId("http://may-chu-noi-bo:20128/v1", "openai"));
        Assert.Equal("company", SetupCatalog.GuessProviderId("", "openai"));
        Assert.Null(SetupCatalog.FindProvider("khong-co"));
        Assert.Equal("openai", SetupCatalog.FindProvider("OpenAI").Id);
    }
}
