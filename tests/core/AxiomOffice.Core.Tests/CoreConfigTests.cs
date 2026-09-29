using AxiomOffice.Core.Config;
using Microsoft.Extensions.Configuration;

namespace AxiomOffice.Core.Tests;

// Cau hinh: AXIOM_* > HKCU > mac dinh (New_arch.md muc 7.6). Dung IConfiguration in-memory thay cho
// bien moi truong that, va registry gia — test khong dung vao cau hinh cua nguoi dung.
public class CoreConfigTests
{
    private sealed class FakeRegistry(Dictionary<string, string> values) : IRegistrySource
    {
        public string? GetString(string name) => values.TryGetValue(name, out string? value) ? value : null;

        public int? GetInt(string name) => int.TryParse(GetString(name), out int value) ? value : null;
    }

    private static CoreConfig Load(Dictionary<string, string>? env = null, Dictionary<string, string>? registry = null)
    {
        IConfiguration config = new ConfigurationBuilder()
            .AddInMemoryCollection((env ?? []).Select(p => new KeyValuePair<string, string?>(p.Key, p.Value)))
            .Build();
        return CoreConfig.From(config, new FakeRegistry(registry ?? []));
    }

    [Fact]
    public void Mac_dinh_khi_khong_co_cau_hinh()
    {
        CoreConfig config = Load();

        Assert.Equal(47840, config.CorePort);
        Assert.False(config.DynamicPort);
        Assert.Equal("", config.Token);
        Assert.Equal("openai", config.LlmProvider);
        Assert.True(config.MemoryEnabled);
        Assert.True(config.MemoryAutoExtract);
        Assert.True(config.SingleInstance);
        Assert.Null(config.DataDirOverride);
        Assert.Empty(config.SkillDirs);
    }

    [Fact]
    public void Bien_moi_truong_thang_registry()
    {
        CoreConfig config = Load(
            env: new() { ["AXIOM_CORE_PORT"] = "41000", ["AXIOM_TOKEN"] = "env-token", ["AXIOM_LLM_MODEL"] = "env-model" },
            registry: new() { ["CorePort"] = "40000", ["Token"] = "reg-token", ["LlmModel"] = "reg-model" });

        Assert.Equal(41000, config.CorePort);
        Assert.Equal("env-token", config.Token);
        Assert.Equal("env-model", config.LlmModel);
    }

    [Fact]
    public void Registry_duoc_dung_khi_khong_co_bien_moi_truong()
    {
        CoreConfig config = Load(registry: new()
        {
            ["CorePort"] = "49999",
            ["Token"] = "reg-token",
            ["LlmProvider"] = "anthropic",
            ["LlmEndpoint"] = "https://api.example.com",
            ["SkillDirs"] = @"C:\skills\org;D:\shared\skills",
            ["MemoryEnabled"] = "0",
        });

        Assert.Equal(49999, config.CorePort);
        Assert.Equal("reg-token", config.Token);
        Assert.Equal("anthropic", config.LlmProvider);
        Assert.Equal("https://api.example.com", config.LlmEndpoint);
        Assert.Equal([@"C:\skills\org", @"D:\shared\skills"], config.SkillDirs);
        Assert.False(config.MemoryEnabled);
    }

    [Theory]
    [InlineData("0", true)]
    [InlineData("65535", false)]
    [InlineData("47840", false)]
    public void Port_hop_le(string raw, bool dynamic)
    {
        Assert.Equal(dynamic, Load(env: new() { ["AXIOM_CORE_PORT"] = raw }).DynamicPort);
    }

    [Theory]
    [InlineData("99999")]
    [InlineData("-1")]
    [InlineData("khong-phai-so")]
    public void Port_khong_hop_le_thi_ve_mac_dinh(string raw)
    {
        CoreConfig config = Load(env: new() { ["AXIOM_CORE_PORT"] = raw }, registry: new() { ["CorePort"] = "40000" });

        // So sai kieu -> roi ve registry; gia tri ngoai khoang -> ve mac dinh.
        Assert.Equal(raw == "khong-phai-so" ? 40000 : 47840, config.CorePort);
    }

    [Theory]
    [InlineData("1", true)]
    [InlineData("true", true)]
    [InlineData("0", false)]
    [InlineData("off", false)]
    [InlineData("banana", true)]
    public void Co_dang_bat_tat(string raw, bool expected)
    {
        Assert.Equal(expected, Load(env: new() { ["AXIOM_MEMORY_ENABLED"] = raw }).MemoryEnabled);
    }

    [Fact]
    public void Api_key_dpapi_duoc_giai_ma_con_khoa_thuong_giu_nguyen()
    {
        string encrypted = Secrets.Protect("sk-bi-mat-123");
        Assert.StartsWith("dpapi:", encrypted);

        CoreConfig fromRegistry = Load(registry: new() { ["LlmApiKey"] = encrypted });
        CoreConfig plain = Load(registry: new() { ["LlmApiKey"] = "sk-tho" });
        CoreConfig broken = Load(registry: new() { ["LlmApiKey"] = "dpapi:@@@khong-phai-base64" });

        Assert.Equal("sk-bi-mat-123", fromRegistry.LlmApiKey);
        Assert.Equal("sk-tho", plain.LlmApiKey);
        Assert.Equal("", broken.LlmApiKey);
    }

    [Fact]
    public void Data_dir_override_va_single_instance_tat_duoc()
    {
        CoreConfig config = Load(env: new()
        {
            ["AXIOM_CORE_DATA_DIR"] = @"C:\temp\axiom-core-test",
            ["AXIOM_CORE_SINGLE_INSTANCE"] = "0",
        });

        Assert.Equal(@"C:\temp\axiom-core-test", config.DataDirOverride);
        Assert.False(config.SingleInstance);
    }
}
