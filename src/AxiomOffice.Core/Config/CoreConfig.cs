using System.Globalization;
using Microsoft.Extensions.Configuration;

namespace AxiomOffice.Core.Config;

// Cau hinh Agent Core: HKCU\Software\AxiomOffice (giong add-in) + override bang bien moi truong
// AXIOM_* de test khong dung cau hinh that cua nguoi dung (New_arch.md muc 7.6).
// Thu tu uu tien: AXIOM_* > HKCU > mac dinh.
public sealed class CoreConfig
{
    public const string RegistryKeyPath = @"Software\AxiomOffice";
    public const int DefaultPort = 47840;
    public const string DefaultMutexName = @"Local\AxiomOffice.Core";

    // Port nghe. 0 = de he dieu hanh cap (chi dung cho test).
    public int CorePort { get; init; } = DefaultPort;

    // false = cho phep nhieu instance (test); mac dinh true: mot Core cho moi nguoi dung Windows.
    public bool SingleInstance { get; init; } = true;

    // Ten mutex giu mot instance (New_arch.md muc 4.3). AXIOM_CORE_MUTEX_NAME de test khong dung
    // vao Core that dang chay cua nguoi dung.
    public string MutexName { get; init; } = @"Local\AxiomOffice.Core";

    // Rong = khong kiem tra token (giong bridge khi Token trong HKCU rong).
    public string Token { get; init; } = "";

    public bool CoreEnabled { get; init; } = true;

    public bool MemoryEnabled { get; init; } = true;

    public bool MemoryAutoExtract { get; init; } = true;

    public string LlmProvider { get; init; } = "openai";

    public string LlmEndpoint { get; init; } = "";

    public string LlmModel { get; init; } = "";

    public string LlmApiKey { get; init; } = "";

    public string MemoryModel { get; init; } = "";

    public string EmbeddingModel { get; init; } = "";

    public string EmbeddingEndpoint { get; init; } = "";

    // Thu muc du lieu (AXIOM_CORE_DATA_DIR). null = dung vi tri mac dinh trong %LOCALAPPDATA%.
    public string? DataDirOverride { get; init; }

    // Thu muc skill cua to chuc (SkillDirs trong HKCU, phan cach ';').
    public IReadOnlyList<string> SkillDirs { get; init; } = [];

    public bool DynamicPort => CorePort == 0;

    public static CoreConfig Load(IConfiguration config)
    {
        return From(config, new RegistrySource(RegistryKeyPath));
    }

    public static CoreConfig From(IConfiguration config, IRegistrySource registry)
    {
        int port = Int(config, "AXIOM_CORE_PORT", registry, "CorePort", DefaultPort);
        if (port is < 0 or > 65535)
        {
            port = DefaultPort;
        }

        return new CoreConfig
        {
            CorePort = port,
            SingleInstance = Flag(config, "AXIOM_CORE_SINGLE_INSTANCE", registry, null, true),
            MutexName = Text(config, "AXIOM_CORE_MUTEX_NAME", null, null) ?? DefaultMutexName,
            Token = Text(config, "AXIOM_TOKEN", registry, "Token") ?? "",
            CoreEnabled = Flag(config, "AXIOM_CORE_ENABLED", registry, "CoreEnabled", true),
            MemoryEnabled = Flag(config, "AXIOM_MEMORY_ENABLED", registry, "MemoryEnabled", true),
            MemoryAutoExtract = Flag(config, "AXIOM_MEMORY_AUTO_EXTRACT", registry, "MemoryAutoExtract", true),
            LlmProvider = Text(config, "AXIOM_LLM_PROVIDER", registry, "LlmProvider") ?? "openai",
            LlmEndpoint = Text(config, "AXIOM_LLM_ENDPOINT", registry, "LlmEndpoint") ?? "",
            LlmModel = Text(config, "AXIOM_LLM_MODEL", registry, "LlmModel") ?? "",
            LlmApiKey = Secrets.Unprotect(Text(config, "AXIOM_LLM_API_KEY", registry, "LlmApiKey")),
            MemoryModel = Text(config, "AXIOM_MEMORY_MODEL", registry, "MemoryModel") ?? "",
            EmbeddingModel = Text(config, "AXIOM_EMBEDDING_MODEL", registry, "EmbeddingModel") ?? "",
            EmbeddingEndpoint = Text(config, "AXIOM_EMBEDDING_ENDPOINT", registry, "EmbeddingEndpoint") ?? "",
            DataDirOverride = Text(config, "AXIOM_CORE_DATA_DIR", null, null),
            SkillDirs = List(Text(config, "AXIOM_SKILL_DIRS", registry, "SkillDirs")),
        };
    }

    // "1"/"true"/"yes"/"on" = bat; "0"/"false"/"no"/"off" = tat. Gia tri khac -> mac dinh.
    private static bool Flag(IConfiguration config, string envKey, IRegistrySource? registry, string? registryName, bool fallback)
    {
        string? raw = Text(config, envKey, registry, registryName);
        if (string.IsNullOrWhiteSpace(raw))
        {
            return fallback;
        }

        return raw.Trim().ToLowerInvariant() switch
        {
            "1" or "true" or "yes" or "on" => true,
            "0" or "false" or "no" or "off" => false,
            _ => fallback,
        };
    }

    private static int Int(IConfiguration config, string envKey, IRegistrySource registry, string registryName, int fallback)
    {
        string? raw = config[envKey];
        if (!string.IsNullOrWhiteSpace(raw) && int.TryParse(raw.Trim(), NumberStyles.Integer, CultureInfo.InvariantCulture, out int fromEnv))
        {
            return fromEnv;
        }

        return registry.GetInt(registryName) ?? fallback;
    }

    private static string? Text(IConfiguration config, string envKey, IRegistrySource? registry, string? registryName)
    {
        string? raw = config[envKey];
        if (!string.IsNullOrWhiteSpace(raw))
        {
            return raw.Trim();
        }

        if (registry == null || registryName == null)
        {
            return null;
        }

        string? value = registry.GetString(registryName);
        return string.IsNullOrWhiteSpace(value) ? null : value.Trim();
    }

    private static IReadOnlyList<string> List(string? raw)
    {
        if (string.IsNullOrWhiteSpace(raw))
        {
            return [];
        }

        return raw.Split(';', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries).ToArray();
    }
}
