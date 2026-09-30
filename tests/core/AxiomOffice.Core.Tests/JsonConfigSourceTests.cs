using AxiomOffice.Core.Config;
using Microsoft.Extensions.Configuration;

namespace AxiomOffice.Core.Tests;

// Cau hinh tren Linux: ~/.config/axiom-office/config.json (LibreOffice_arch.md muc 9) - cung ten khoa voi HKCU,
// extension LibreOffice ghi so/bool/chuoi dang JSON; Core doc nhu registry.
public class JsonConfigSourceTests : IDisposable
{
    private readonly string _dir = Path.Combine(Path.GetTempPath(), "axiom-jsoncfg-" + Guid.NewGuid().ToString("N"));

    public JsonConfigSourceTests()
    {
        Directory.CreateDirectory(_dir);
    }

    public void Dispose()
    {
        try
        {
            Directory.Delete(_dir, recursive: true);
        }
        catch (IOException)
        {
        }
    }

    private JsonConfigSource Write(string json)
    {
        string path = Path.Combine(_dir, "config.json");
        File.WriteAllText(path, json);
        return new JsonConfigSource(path);
    }

    [Fact]
    public void Doc_chuoi_so_bool_va_danh_sach()
    {
        JsonConfigSource source = Write("""
            {"Token": "abc", "CorePort": 47845, "MemoryEnabled": 0, "VisualQaEnabled": true,
             "SkillDirs": ["/opt/skills", "/srv/skills"], "LlmModel": "oc/muse"}
            """);

        Assert.Equal("abc", source.GetString("Token"));
        Assert.Equal(47845, source.GetInt("CorePort"));
        Assert.Equal("0", source.GetString("MemoryEnabled"));
        Assert.Equal("1", source.GetString("VisualQaEnabled"));
        Assert.Equal("/opt/skills;/srv/skills", source.GetString("SkillDirs"));
        Assert.Null(source.GetString("KhongCo"));
    }

    [Fact]
    public void File_thieu_hoac_hong_tra_null()
    {
        Assert.Null(new JsonConfigSource(Path.Combine(_dir, "khong-co.json")).GetString("Token"));
        Assert.Null(Write("{ day khong phai json").GetString("Token"));
    }

    [Fact]
    public void CoreConfig_doc_tu_config_json()
    {
        JsonConfigSource source = Write("""
            {"Token": "tok", "LlmEndpoint": "http://localhost:20128/v1", "LlmModel": "m1", "MemoryEnabled": 0,
             "SkillDirs": ["/a", "/b"], "LlmApiKey": "sk-thuong"}
            """);
        IConfiguration env = new ConfigurationBuilder().Build();

        CoreConfig config = CoreConfig.From(env, source);

        Assert.Equal("tok", config.Token);
        Assert.Equal("http://localhost:20128/v1", config.LlmEndpoint);
        Assert.Equal("m1", config.LlmModel);
        Assert.False(config.MemoryEnabled);
        Assert.Equal(["/a", "/b"], config.SkillDirs);
        Assert.Equal("sk-thuong", config.LlmApiKey);
    }

    [Fact]
    public void Nguon_mac_dinh_theo_he_dieu_hanh()
    {
        IRegistrySource source = CoreConfig.DefaultSource();
        if (OperatingSystem.IsWindows())
        {
            Assert.IsType<RegistrySource>(source);
            return;
        }

        var json = Assert.IsType<JsonConfigSource>(source);
        Assert.EndsWith(Path.Combine("axiom-office", "config.json"), json.Path);
    }

    [Fact]
    public void Thu_muc_du_lieu_va_session_theo_he_dieu_hanh()
    {
        string root = CorePaths.DefaultRoot();
        string sessions = Office.SessionDirectory.DefaultDirectory;
        if (OperatingSystem.IsWindows())
        {
            Assert.EndsWith("AxiomOffice", root);
            Assert.EndsWith(Path.Combine("AxiomOffice", "sessions"), sessions);
            return;
        }

        // Cung cho voi extension LibreOffice (axiom/config.py: data_dir, runtime_dir).
        Assert.EndsWith("axiom-office", root);
        Assert.EndsWith(Path.Combine("axiom-office", "sessions"), sessions);
    }
}
