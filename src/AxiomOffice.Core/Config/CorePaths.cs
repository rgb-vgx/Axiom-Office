namespace AxiomOffice.Core.Config;

// Vi tri file cua Core (New_arch.md muc 7.1, 8.5.1, 8.9):
//   mac dinh  : %LOCALAPPDATA%\AxiomOffice\{core.json, core.log} + ...\core\core.db
//   AXIOM_CORE_DATA_DIR: ca ba nam trong thu muc do (test khong lam ban du lieu that).
public sealed class CorePaths
{
    private CorePaths(string root, bool isOverride)
    {
        Root = root;
        IsOverride = isOverride;
        CoreJson = Path.Combine(root, "core.json");
        LogFile = Path.Combine(root, "core.log");
        DatabaseFile = isOverride ? Path.Combine(root, "core.db") : Path.Combine(root, "core", "core.db");
        SkillsDirectory = Path.Combine(root, "skills");
        McpConfigFile = Path.Combine(root, "mcp.json");
    }

    public string Root { get; }

    public bool IsOverride { get; }

    public string CoreJson { get; }

    public string LogFile { get; }

    public string DatabaseFile { get; }

    public string SkillsDirectory { get; }

    public string McpConfigFile { get; }

    public static CorePaths Create(string? dataDirOverride)
    {
        if (!string.IsNullOrWhiteSpace(dataDirOverride))
        {
            return new CorePaths(Path.GetFullPath(dataDirOverride.Trim()), true);
        }

        string local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        return new CorePaths(Path.Combine(local, "AxiomOffice"), false);
    }

    public void EnsureDirectories()
    {
        Directory.CreateDirectory(Root);
        Directory.CreateDirectory(Path.GetDirectoryName(DatabaseFile)!);
    }
}
