using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Config;

// Cau hinh tren Linux (LibreOffice_arch.md muc 9): ~/.config/axiom-office/config.json (quyen 0600), cung
// ten khoa voi HKCU\Software\AxiomOffice tren Windows (LlmEndpoint, LlmModel, Token, CorePort...). Extension
// LibreOffice ghi file nay (Cai dat trong pane, install.sh); Core chi doc. Doc lai moi lan hoi de ModelSource
// thay cau hinh moi ma khong can khoi dong lai Core. Loi (khong co file, JSON hong) -> null nhu registry.
public sealed class JsonConfigSource(string path) : IRegistrySource
{
    public string Path => path;

    public static string DefaultPath
    {
        get
        {
            string? xdg = Environment.GetEnvironmentVariable("XDG_CONFIG_HOME");
            string root = string.IsNullOrWhiteSpace(xdg)
                ? System.IO.Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".config")
                : xdg;
            return System.IO.Path.Combine(root, "axiom-office", "config.json");
        }
    }

    public string? GetString(string name)
    {
        try
        {
            if (!File.Exists(path))
            {
                return null;
            }

            JsonNode? value = JsonNode.Parse(File.ReadAllText(path))?[name];
            return value switch
            {
                null => null,
                JsonValue scalar when scalar.TryGetValue(out string? text) => text,
                JsonValue scalar when scalar.TryGetValue(out bool flag) => flag ? "1" : "0",
                JsonValue scalar => scalar.ToJsonString(),
                JsonArray list => string.Join(';', list.Select(item => item?.ToString() ?? "")),
                _ => null,
            };
        }
        catch
        {
            return null;
        }
    }

    public int? GetInt(string name)
    {
        string? raw = GetString(name);
        return int.TryParse(raw, out int value) ? value : null;
    }
}
