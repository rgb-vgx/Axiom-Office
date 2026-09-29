using System.Text.Json.Nodes;
using AxiomOffice.Core.Agent;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Office;
using AxiomOffice.Core.Skills;
using AxiomOffice.Core.Tools;

namespace AxiomOffice.Core.Tests;

// Skills theo chuan Agent Skills (New_arch.md muc 8.4, 11): frontmatter, validate, uu tien nguon,
// load_skill / read_skill_file (chan '..', loc phan mo rong), skill loi khong lam hong Core.
public sealed class SkillTests : IDisposable
{
    private readonly string _root = Path.Combine(Path.GetTempPath(), "axiom-skills-" + Guid.NewGuid().ToString("N"));

    public SkillTests()
    {
        Directory.CreateDirectory(_root);
    }

    public void Dispose()
    {
        try
        {
            Directory.Delete(_root, true);
        }
        catch (IOException)
        {
        }
    }

    private string Source(string name)
    {
        string directory = Path.Combine(_root, name);
        Directory.CreateDirectory(directory);
        return directory;
    }

    private static string WriteSkill(string source, string folder, string frontmatter, string body = "# Huong dan\nLam theo cac buoc.")
    {
        string directory = Path.Combine(source, folder);
        Directory.CreateDirectory(directory);
        File.WriteAllText(Path.Combine(directory, "SKILL.md"), "---\n" + frontmatter.Trim() + "\n---\n" + body);
        return directory;
    }

    private static RunContext Context(List<(string Type, JsonNode? Data)>? events = null)
    {
        var config = new CoreConfig { Token = "t" };
        return new RunContext
        {
            RunId = "r_1",
            ConversationId = "c_1",
            Office = new OfficeSession(1, "wps", "office", 47831, "WINWORD.EXE", "1.0.0", 0, "a.docx", @"C:\a.docx", "f.json"),
            Config = config,
            Bridge = new BridgeClient(config, new HttpClient()),
            Event = (type, data) => events?.Add((type, data)),
        };
    }

    [Fact]
    public void Frontmatter_hop_le_voi_khoi_gap_va_apps()
    {
        (SkillDefinition? skill, SkillError? error) = SkillLoader.Parse(
            "---\nname: van-ban-hanh-chinh\ndescription: >-\n  Soan cong van dung the thuc.\n  Dung khi nguoi dung soan cong van.\napps: [wps]\nlicense: MIT\nmetadata:\n  author: to chuc\n---\n# Than\nNoi dung",
            @"C:\s\van-ban-hanh-chinh", "builtin");

        Assert.Null(error);
        Assert.NotNull(skill);
        Assert.Equal("van-ban-hanh-chinh", skill!.Name);
        Assert.Equal("Soan cong van dung the thuc. Dung khi nguoi dung soan cong van.", skill.Description);
        Assert.Equal(["wps"], skill.Apps);
        Assert.True(skill.AppliesTo("wps"));
        Assert.False(skill.AppliesTo("et"));
        Assert.StartsWith("# Than", skill.Body);
    }

    [Fact]
    public void Apps_bo_trong_la_moi_app_va_danh_sach_gach_dau_dong()
    {
        (SkillDefinition? all, _) = SkillLoader.Parse("---\nname: chung\ndescription: Dung cho moi app.\n---\nx", "d", "user");
        (SkillDefinition? two, _) = SkillLoader.Parse("---\nname: hai-app\ndescription: \"Hai app\"\napps:\n  - et\n  - wpp\n---\nx", "d", "user");

        Assert.True(all!.AppliesTo("wps") && all.AppliesTo("et") && all.AppliesTo("wpp"));
        Assert.Equal(["et", "wpp"], two!.Apps);
    }

    [Theory]
    [InlineData("name: Viet-Hoa\ndescription: x", "lowercase")]
    [InlineData("name: co khoang trang\ndescription: x", "lowercase")]
    [InlineData("name: claude-helper\ndescription: x", "reserved word 'claude'")]
    [InlineData("name: my-anthropic\ndescription: x", "reserved word 'anthropic'")]
    [InlineData("description: x", "'name' is required")]
    [InlineData("name: ok-name", "'description' is required")]
    [InlineData("name: ok-name\ndescription: Dung <b>the</b> nay", "XML tags")]
    public void Frontmatter_sai_bao_loi_ro(string frontmatter, string expected)
    {
        (SkillDefinition? skill, SkillError? error) = SkillLoader.Parse("---\n" + frontmatter + "\n---\nbody", "d", "user");

        Assert.Null(skill);
        Assert.Contains(expected, error!.Error);
    }

    [Fact]
    public void Ten_va_mo_ta_qua_dai_bi_tu_choi()
    {
        string longName = new('a', 65);
        string longDescription = new('m', 1025);

        Assert.Contains("64", SkillLoader.Parse($"---\nname: {longName}\ndescription: x\n---\n", "d", "u").Error!.Error);
        Assert.Contains("1024", SkillLoader.Parse($"---\nname: ok\ndescription: {longDescription}\n---\n", "d", "u").Error!.Error);
        Assert.Null(SkillLoader.Parse($"---\nname: {new string('a', 64)}\ndescription: {new string('m', 1024)}\n---\n", "d", "u").Error);
    }

    [Fact]
    public void Thieu_frontmatter_bao_loi()
    {
        Assert.Contains("frontmatter", SkillLoader.Parse("# Chi co than", "d", "u").Error!.Error);
    }

    [Fact]
    public void Nguon_sau_ghi_de_nguon_truoc_va_skill_loi_khong_lam_hong_ca_chi_muc()
    {
        string builtin = Source("builtin");
        string user = Source("user");
        WriteSkill(builtin, "bang-diem", "name: bang-diem\ndescription: Ban dung san\napps: [et]");
        WriteSkill(builtin, "hong", "name: Hong Ten\ndescription: x");
        WriteSkill(user, "bang-diem", "name: bang-diem\ndescription: Ban cua nguoi dung\napps: [et]");
        Directory.CreateDirectory(Path.Combine(builtin, "_design"));
        File.WriteAllText(Path.Combine(builtin, "_design", "tokens.json"), "{\"font\":\"Calibri\"}");
        Directory.CreateDirectory(Path.Combine(builtin, "khong-co-skill-md"));

        using var index = new SkillIndex([new SkillSource("builtin", builtin), new SkillSource("org", Path.Combine(_root, "khong-ton-tai")), new SkillSource("user", user)]);

        SkillDefinition skill = Assert.Single(index.All);
        Assert.Equal("Ban cua nguoi dung", skill.Description);
        Assert.Equal("user", skill.Source);
        SkillError error = Assert.Single(index.Errors);
        Assert.EndsWith("hong", error.Directory);
        Assert.Equal(Path.Combine(builtin, "_design"), index.ResourceDirectory("_design"));
        Assert.Empty(index.ForApp("wps"));
        Assert.Single(index.ForApp("et"));
    }

    [Fact]
    public void Reload_thay_skill_moi()
    {
        string user = Source("user");
        using var index = new SkillIndex([new SkillSource("user", user)]);
        Assert.Empty(index.All);

        WriteSkill(user, "moi", "name: moi\ndescription: Skill moi");
        index.Reload();

        Assert.Equal("moi", Assert.Single(index.All).Name);
    }

    [Fact]
    public async Task Load_skill_tra_huong_dan_danh_sach_file_va_phat_skill_loaded()
    {
        string user = Source("user");
        string directory = WriteSkill(user, "bao-cao-thang", "name: bao-cao-thang\ndescription: Slide bao cao thang\napps: [wpp]", "# Buoc\n1. Doc slide");
        Directory.CreateDirectory(Path.Combine(directory, "references"));
        File.WriteAllText(Path.Combine(directory, "references", "bo-cuc.md"), "bo cuc");
        File.WriteAllText(Path.Combine(directory, "script.ps1"), "rm -rf /");
        using var index = new SkillIndex([new SkillSource("user", user)]);
        var events = new List<(string Type, JsonNode? Data)>();

        ToolResult result = await new LoadSkillTool(index, "wpp").InvokeAsync(new JsonObject { ["name"] = "bao-cao-thang" }, Context(events), CancellationToken.None);

        Assert.True(result.Ok);
        JsonNode json = JsonNode.Parse(result.Json)!["result"]!;
        Assert.Contains("1. Doc slide", json["instructions"]!.GetValue<string>());
        Assert.Equal(["references/bo-cuc.md"], json["files"]!.AsArray().Select(f => f!.GetValue<string>()));
        (string type, JsonNode? data) = Assert.Single(events);
        Assert.Equal("skill.loaded", type);
        Assert.Equal("bao-cao-thang", data!["name"]!.GetValue<string>());
    }

    [Fact]
    public async Task Load_skill_sai_ten_hoac_sai_app_bao_danh_sach_co_san()
    {
        string user = Source("user");
        WriteSkill(user, "bang-diem", "name: bang-diem\ndescription: Bang diem\napps: [et]");
        using var index = new SkillIndex([new SkillSource("user", user)]);

        ToolResult unknown = await new LoadSkillTool(index, "et").InvokeAsync(new JsonObject { ["name"] = "khong-co" }, Context(), CancellationToken.None);
        ToolResult wrongApp = await new LoadSkillTool(index, "wps").InvokeAsync(new JsonObject { ["name"] = "bang-diem" }, Context(), CancellationToken.None);

        Assert.False(unknown.Ok);
        Assert.Contains("available skills: bang-diem", unknown.Json);
        Assert.False(wrongApp.Ok);
    }

    [Fact]
    public async Task Read_skill_file_doc_text_tra_duong_dan_nhi_phan_va_chan_ra_ngoai()
    {
        string user = Source("user");
        string directory = WriteSkill(user, "van-ban-hanh-chinh", "name: van-ban-hanh-chinh\ndescription: Cong van");
        Directory.CreateDirectory(Path.Combine(directory, "references"));
        Directory.CreateDirectory(Path.Combine(directory, "templates"));
        File.WriteAllText(Path.Combine(directory, "references", "the-thuc.md"), "Quoc hieu in hoa");
        File.WriteAllBytes(Path.Combine(directory, "templates", "cong-van.docx"), [1, 2, 3]);
        File.WriteAllText(Path.Combine(directory, "run.ps1"), "x");
        File.WriteAllText(Path.Combine(user, "bi-mat.md"), "khong duoc doc");
        Directory.CreateDirectory(Path.Combine(user, "_design"));
        File.WriteAllText(Path.Combine(user, "_design", "tokens.json"), "{\"primary\":\"#1F4E79\"}");
        using var index = new SkillIndex([new SkillSource("user", user)]);
        var tool = new ReadSkillFileTool(index, "wps");

        async Task<ToolResult> Read(string name, string path) =>
            await tool.InvokeAsync(new JsonObject { ["name"] = name, ["path"] = path }, Context(), CancellationToken.None);

        ToolResult text = await Read("van-ban-hanh-chinh", "references/the-thuc.md");
        Assert.True(text.Ok);
        Assert.Contains("Quoc hieu in hoa", text.Json);

        ToolResult backslash = await Read("van-ban-hanh-chinh", @"references\the-thuc.md");
        Assert.True(backslash.Ok);

        ToolResult binary = await Read("van-ban-hanh-chinh", "templates/cong-van.docx");
        Assert.True(binary.Ok);
        Assert.Equal(Path.Combine(directory, "templates", "cong-van.docx"), JsonNode.Parse(binary.Json)!["result"]!["absolutePath"]!.GetValue<string>());

        ToolResult tokens = await Read("_design", "tokens.json");
        Assert.True(tokens.Ok);
        Assert.Contains("#1F4E79", tokens.Json);

        Assert.Contains("'..'", (await Read("van-ban-hanh-chinh", "../bi-mat.md")).Json);
        Assert.Contains("'..'", (await Read("van-ban-hanh-chinh", "references/../../bi-mat.md")).Json);
        Assert.False((await Read("van-ban-hanh-chinh", Path.Combine(user, "bi-mat.md"))).Ok);
        Assert.Contains("not allowed", (await Read("van-ban-hanh-chinh", "run.ps1")).Json);
        Assert.Contains("not found", (await Read("van-ban-hanh-chinh", "references/khong-co.md")).Json);
        Assert.Contains("unknown skill", (await Read("khong-co", "a.md")).Json);
    }

    [Fact]
    public async Task Read_skill_file_cat_file_qua_64KB()
    {
        string user = Source("user");
        string directory = WriteSkill(user, "lon", "name: lon\ndescription: File lon");
        File.WriteAllText(Path.Combine(directory, "big.txt"), new string('x', SkillFiles.MaxTextBytes + 100));
        using var index = new SkillIndex([new SkillSource("user", user)]);

        ToolResult result = await new ReadSkillFileTool(index, "wps").InvokeAsync(new JsonObject { ["name"] = "lon", ["path"] = "big.txt" }, Context(), CancellationToken.None);

        JsonNode json = JsonNode.Parse(result.Json)!["result"]!;
        Assert.True(json["truncated"]!.GetValue<bool>());
        Assert.Equal(SkillFiles.MaxTextBytes, json["content"]!.GetValue<string>().Length);
    }

    private static string RepoSkills()
    {
        for (DirectoryInfo? dir = new(AppContext.BaseDirectory); dir != null; dir = dir.Parent)
        {
            if (File.Exists(Path.Combine(dir.FullName, "New_arch.md")))
            {
                return Path.Combine(dir.FullName, "skills");
            }
        }

        throw new InvalidOperationException("repo root not found");
    }

    [Fact]
    public void Skill_dung_san_trong_repo_hop_le_va_dung_app()
    {
        using var index = new SkillIndex([new SkillSource("builtin", RepoSkills())]);

        Assert.Empty(index.Errors);
        string[] expected = ["bang-diem", "bao-cao-du-lieu", "bao-cao-thang", "the-thuc-van-ban", "thiet-ke-van-phong", "trinh-bay-chuyen-nghiep", "van-ban-hanh-chinh"];
        Assert.Equal(expected, index.All.Select(s => s.Name));
        Assert.Equal(["the-thuc-van-ban", "thiet-ke-van-phong", "van-ban-hanh-chinh"], index.ForApp("wps").Select(s => s.Name));
        Assert.Equal(["bang-diem", "bao-cao-du-lieu", "thiet-ke-van-phong"], index.ForApp("et").Select(s => s.Name));
        Assert.Equal(["bao-cao-thang", "thiet-ke-van-phong", "trinh-bay-chuyen-nghiep"], index.ForApp("wpp").Select(s => s.Name));
        foreach (SkillDefinition skill in index.All)
        {
            // Chuan: mo ta ngoi ba co "Dung khi" (khi nao dung); than gon (< 500 dong).
            Assert.Contains("Dùng khi", skill.Description);
            Assert.True(skill.Body.Split('\n').Length < 500, skill.Name);
            Assert.Equal(skill.Name, Path.GetFileName(skill.Directory));
        }

        string tokens = Path.Combine(index.ResourceDirectory("_design")!, "tokens.json");
        JsonNode parsed = JsonNode.Parse(File.ReadAllText(tokens))!;
        Assert.Equal("#1F4E79", parsed["colors"]!["primary"]!.GetValue<string>());
    }

    [Fact]
    public void Prompt_liet_ke_skill_cat_300_ky_tu()
    {
        string section = PromptBuilder.SkillsSection([new SkillSummary("bang-diem", new string('d', 400))]);

        Assert.Contains("load_skill", section);
        Assert.Contains("- bang-diem: ", section);
        Assert.DoesNotContain(new string('d', 301), section);
    }
}
