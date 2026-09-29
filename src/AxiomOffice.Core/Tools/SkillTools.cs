using System.Text;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Skills;

namespace AxiomOffice.Core.Tools;

// load_skill {name} -> noi dung SKILL.md + danh sach file dinh kem (tang 2 cua chuan Agent Skills, muc 8.4.3).
public sealed class LoadSkillTool(SkillIndex index, string appKind) : ITool
{
    public const string ToolName = "load_skill";
    public const int MaxListedFiles = 50;

    public string Name => ToolName;

    public string Description =>
        "Load the full instructions of one skill from the 'Available skills' list in the system prompt. "
        + "Call it before starting a task that matches a skill description (creating or redesigning a document, "
        + "slide deck, report or table). Do not load skills for small edits such as making one line bold.";

    public JsonNode ParametersSchema => new JsonObject
    {
        ["type"] = "object",
        ["properties"] = new JsonObject
        {
            ["name"] = new JsonObject { ["type"] = "string", ["description"] = "Skill name exactly as listed" },
        },
        ["required"] = new JsonArray("name"),
    };

    public Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel)
    {
        string name = (arguments?["name"]?.GetValue<string>() ?? "").Trim();
        SkillDefinition? skill = index.Find(name);
        if (skill == null || !skill.AppliesTo(appKind))
        {
            string available = string.Join(", ", index.ForApp(appKind).Select(s => s.Name));
            return Task.FromResult(new ToolResult(
                SkillToolJson.Error($"unknown skill '{name}'; available skills: {(available.Length == 0 ? "(none)" : available)}"), false));
        }

        context.Event?.Invoke("skill.loaded", new JsonObject { ["name"] = skill.Name, ["source"] = skill.Source });
        var files = new JsonArray();
        foreach (string file in SkillFiles.List(skill.Directory).Take(MaxListedFiles))
        {
            files.Add(file);
        }

        var result = new JsonObject
        {
            ["name"] = skill.Name,
            ["instructions"] = skill.Body,
            ["files"] = files,
            ["readFilesWith"] = "read_skill_file {name, path}",
        };
        return Task.FromResult(new ToolResult(SkillToolJson.Ok(result), true));
    }
}

// read_skill_file {name, path} -> text (<= 64KB) hoac duong dan tuyet doi cho file nhi phan (muc 8.4.3, 8.4.4).
// name co the la mot skill hoac goi tai nguyen dung chung (vd "_design" -> tokens.json).
public sealed class ReadSkillFileTool(SkillIndex index, string appKind) : ITool
{
    public const string ToolName = "read_skill_file";

    public string Name => ToolName;

    public string Description =>
        "Read a file that belongs to a skill (for example references/..., examples/..., or tokens.json of the shared "
        + "'_design' pack). Text files return their content; templates (.docx/.xlsx/.pptx) and images return an absolute path.";

    public JsonNode ParametersSchema => new JsonObject
    {
        ["type"] = "object",
        ["properties"] = new JsonObject
        {
            ["name"] = new JsonObject { ["type"] = "string", ["description"] = "Skill name, or a shared pack such as _design" },
            ["path"] = new JsonObject { ["type"] = "string", ["description"] = "Relative path inside the skill, e.g. references/the-thuc.md" },
        },
        ["required"] = new JsonArray("name", "path"),
    };

    public Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel)
    {
        string name = (arguments?["name"]?.GetValue<string>() ?? "").Trim();
        string path = (arguments?["path"]?.GetValue<string>() ?? "").Trim();

        string? root = null;
        SkillDefinition? skill = index.Find(name);
        if (skill != null && skill.AppliesTo(appKind))
        {
            root = skill.Directory;
        }
        else if (name.StartsWith('_'))
        {
            root = index.ResourceDirectory(name);
        }

        if (root == null)
        {
            return Task.FromResult(new ToolResult(SkillToolJson.Error($"unknown skill '{name}'"), false));
        }

        (string? file, string? error) = SkillFiles.Resolve(root, path);
        if (file == null)
        {
            return Task.FromResult(new ToolResult(SkillToolJson.Error(error!), false));
        }

        string extension = Path.GetExtension(file).ToLowerInvariant();
        if (SkillFiles.BinaryExtensions.Contains(extension))
        {
            return Task.FromResult(new ToolResult(SkillToolJson.Ok(new JsonObject
            {
                ["name"] = name,
                ["path"] = path,
                ["absolutePath"] = file,
                ["binary"] = true,
            }), true));
        }

        byte[] bytes = File.ReadAllBytes(file);
        bool truncated = bytes.Length > SkillFiles.MaxTextBytes;
        string text = Encoding.UTF8.GetString(bytes, 0, truncated ? SkillFiles.MaxTextBytes : bytes.Length).TrimStart('﻿');
        var result = new JsonObject { ["name"] = name, ["path"] = path, ["content"] = text };
        if (truncated)
        {
            result["truncated"] = true;
        }

        return Task.FromResult(new ToolResult(SkillToolJson.Ok(result), true));
    }
}

// Quy tac file cua skill: chi doc ben trong thu muc skill, chan '..' va symlink ra ngoai, chi phan mo rong da biet.
public static class SkillFiles
{
    public const int MaxTextBytes = 64 * 1024;

    public static readonly HashSet<string> TextExtensions = new(StringComparer.OrdinalIgnoreCase) { ".md", ".txt", ".json", ".csv" };

    public static readonly HashSet<string> BinaryExtensions = new(StringComparer.OrdinalIgnoreCase) { ".docx", ".xlsx", ".pptx", ".png", ".jpg" };

    public static (string? File, string? Error) Resolve(string root, string relative)
    {
        string normalized = relative.Replace('\\', '/').Trim();
        if (normalized.Length == 0)
        {
            return (null, "'path' is required");
        }

        if (normalized.StartsWith('/') || normalized.Contains(':') || normalized.Split('/').Any(part => part == ".."))
        {
            return (null, "'path' must be a relative path inside the skill (no '..' or absolute paths)");
        }

        string extension = Path.GetExtension(normalized);
        if (!TextExtensions.Contains(extension) && !BinaryExtensions.Contains(extension))
        {
            return (null, $"file type '{extension}' is not allowed (allowed: {string.Join(", ", TextExtensions.Concat(BinaryExtensions))})");
        }

        string rootFull = Path.GetFullPath(root).TrimEnd(Path.DirectorySeparatorChar) + Path.DirectorySeparatorChar;
        string full = Path.GetFullPath(Path.Combine(rootFull, normalized.Replace('/', Path.DirectorySeparatorChar)));
        if (!full.StartsWith(rootFull, StringComparison.OrdinalIgnoreCase))
        {
            return (null, "'path' must stay inside the skill directory");
        }

        if (!File.Exists(full))
        {
            return (null, $"file not found: {normalized}");
        }

        // Symlink/junction (file hoac thu muc tren duong dan) tro ra ngoai thu muc skill: tu choi.
        FileSystemInfo? target = new FileInfo(full).ResolveLinkTarget(returnFinalTarget: true);
        if (target != null && !Path.GetFullPath(target.FullName).StartsWith(rootFull, StringComparison.OrdinalIgnoreCase))
        {
            return (null, "'path' is a link that points outside the skill directory");
        }

        for (DirectoryInfo? folder = new FileInfo(full).Directory;
             folder != null && folder.FullName.TrimEnd(Path.DirectorySeparatorChar).Length > rootFull.Length - 1;
             folder = folder.Parent)
        {
            if (folder.LinkTarget != null)
            {
                return (null, "'path' goes through a link to another directory");
            }
        }

        return (full, null);
    }

    // Danh sach file cua skill (tru SKILL.md), duong dan kieu '/', chi phan mo rong doc duoc.
    public static IEnumerable<string> List(string directory)
    {
        if (!Directory.Exists(directory))
        {
            yield break;
        }

        string root = Path.GetFullPath(directory).TrimEnd(Path.DirectorySeparatorChar) + Path.DirectorySeparatorChar;
        foreach (string file in Directory.EnumerateFiles(directory, "*", SearchOption.AllDirectories).OrderBy(f => f, StringComparer.OrdinalIgnoreCase))
        {
            string relative = Path.GetFullPath(file)[root.Length..].Replace(Path.DirectorySeparatorChar, '/');
            string extension = Path.GetExtension(relative);
            if (relative.Equals(SkillLoader.SkillFileName, StringComparison.OrdinalIgnoreCase)
                || (!TextExtensions.Contains(extension) && !BinaryExtensions.Contains(extension)))
            {
                continue;
            }

            yield return relative;
        }
    }
}

internal static class SkillToolJson
{
    public static string Ok(JsonNode result)
    {
        return new JsonObject { ["ok"] = true, ["result"] = result }.ToJsonString(ModelClient.RelaxedJson);
    }

    public static string Error(string message)
    {
        return ModelClient.ErrorJson(message);
    }
}
