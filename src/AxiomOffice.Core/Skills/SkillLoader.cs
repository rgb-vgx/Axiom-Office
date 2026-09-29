using System.Text;
using System.Text.RegularExpressions;

namespace AxiomOffice.Core.Skills;

// Mot skill hop le theo chuan Agent Skills (New_arch.md muc 8.4): thu muc chua SKILL.md co frontmatter
// name + description. Apps la phan mo rong tuy chon cua Axiom (rong = moi app).
public sealed record SkillDefinition(
    string Name,
    string Description,
    IReadOnlyList<string> Apps,
    string Directory,
    string Source,
    string Body)
{
    public bool AppliesTo(string appKind)
    {
        return Apps.Count == 0 || Apps.Contains(appKind, StringComparer.OrdinalIgnoreCase);
    }
}

// Skill loi (frontmatter sai...): bo qua skill do, hien loi o GET /v1/skills, khong lam hong Core.
public sealed record SkillError(string Directory, string Source, string Error);

public static partial class SkillLoader
{
    public const string SkillFileName = "SKILL.md";
    public const int MaxNameLength = 64;
    public const int MaxDescriptionLength = 1024;
    private static readonly string[] ReservedWords = ["anthropic", "claude"];

    // Doc va validate mot thu muc skill. Tra dung mot trong hai: skill hoac loi.
    public static (SkillDefinition? Skill, SkillError? Error) Load(string directory, string source)
    {
        string file = Path.Combine(directory, SkillFileName);
        string text;
        try
        {
            text = File.ReadAllText(file, Encoding.UTF8);
        }
        catch (Exception ex)
        {
            return (null, new SkillError(directory, source, "cannot read SKILL.md: " + ex.Message));
        }

        return Parse(text, directory, source);
    }

    public static (SkillDefinition? Skill, SkillError? Error) Parse(string text, string directory, string source)
    {
        SkillError Fail(string message) => new(directory, source, message);

        if (!TrySplitFrontmatter(text, out string frontmatter, out string body))
        {
            return (null, Fail("SKILL.md must start with a YAML frontmatter block delimited by '---'"));
        }

        Dictionary<string, object> fields = Frontmatter.Parse(frontmatter);
        string name = (fields.GetValueOrDefault("name") as string ?? "").Trim();
        string description = (fields.GetValueOrDefault("description") as string ?? "").Trim();

        string? error = ValidateName(name) ?? ValidateDescription(description);
        if (error != null)
        {
            return (null, Fail(error));
        }

        var apps = new List<string>();
        switch (fields.GetValueOrDefault("apps"))
        {
            case List<string> list:
                apps.AddRange(list.Select(a => a.Trim()).Where(a => a.Length > 0));
                break;
            case string single when single.Trim().Length > 0:
                apps.AddRange(single.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries));
                break;
        }

        return (new SkillDefinition(name, description, apps, directory, source, body.Trim()), null);
    }

    public static string? ValidateName(string name)
    {
        if (name.Length == 0)
        {
            return "frontmatter 'name' is required";
        }

        if (name.Length > MaxNameLength)
        {
            return $"'name' must be at most {MaxNameLength} characters";
        }

        if (!NamePattern().IsMatch(name))
        {
            return "'name' may only contain lowercase letters, digits and '-'";
        }

        foreach (string word in ReservedWords)
        {
            if (name.Contains(word, StringComparison.OrdinalIgnoreCase))
            {
                return $"'name' must not contain the reserved word '{word}'";
            }
        }

        return null;
    }

    public static string? ValidateDescription(string description)
    {
        if (description.Length == 0)
        {
            return "frontmatter 'description' is required";
        }

        if (description.Length > MaxDescriptionLength)
        {
            return $"'description' must be at most {MaxDescriptionLength} characters";
        }

        return XmlTag().IsMatch(description) ? "'description' must not contain XML tags" : null;
    }

    private static bool TrySplitFrontmatter(string text, out string frontmatter, out string body)
    {
        frontmatter = "";
        body = "";
        string normalized = text.Replace("\r\n", "\n").TrimStart('﻿');
        if (!normalized.StartsWith("---\n", StringComparison.Ordinal) && normalized != "---")
        {
            return false;
        }

        int end = normalized.IndexOf("\n---", 4, StringComparison.Ordinal);
        if (end < 0)
        {
            return false;
        }

        frontmatter = normalized[4..end];
        int bodyStart = normalized.IndexOf('\n', end + 4);
        body = bodyStart < 0 ? "" : normalized[(bodyStart + 1)..];
        return true;
    }

    [GeneratedRegex("^[a-z0-9-]+$")]
    private static partial Regex NamePattern();

    [GeneratedRegex(@"<\s*/?\s*[A-Za-z][^>]*>")]
    private static partial Regex XmlTag();
}

// Tap con YAML du cho frontmatter cua skill: `key: value`, chuoi co nhay, khoi `>`/`>-`/`|`/`|-`,
// danh sach `[a, b]` va `- item`. Key long nhau / field la: bo qua (khong loi) de tuong thich xuoi.
public static class Frontmatter
{
    public static Dictionary<string, object> Parse(string text)
    {
        var result = new Dictionary<string, object>(StringComparer.Ordinal);
        string[] lines = text.Split('\n');
        int i = 0;
        while (i < lines.Length)
        {
            string line = lines[i].TrimEnd('\r');
            if (line.Length == 0 || line.TrimStart().StartsWith('#') || char.IsWhiteSpace(line[0]))
            {
                i++;
                continue;
            }

            int colon = line.IndexOf(':');
            if (colon <= 0)
            {
                i++;
                continue;
            }

            string key = line[..colon].Trim();
            string value = line[(colon + 1)..].Trim();
            i++;

            if (value is ">" or ">-" or ">+" or "|" or "|-" or "|+")
            {
                var block = new List<string>();
                while (i < lines.Length && (lines[i].Length == 0 || char.IsWhiteSpace(lines[i][0])))
                {
                    block.Add(lines[i].Trim());
                    i++;
                }

                result[key] = value.StartsWith('>')
                    ? string.Join(" ", block.Where(l => l.Length > 0))
                    : string.Join("\n", block).Trim('\n');
                continue;
            }

            if (value.Length == 0)
            {
                // Danh sach "- item" hoac map long nhau (bo qua).
                var items = new List<string>();
                bool isList = false;
                while (i < lines.Length && (lines[i].Length == 0 || char.IsWhiteSpace(lines[i][0])))
                {
                    string inner = lines[i].Trim();
                    if (inner.StartsWith("- ", StringComparison.Ordinal) || inner == "-")
                    {
                        isList = true;
                        items.Add(Unquote(inner.Length > 1 ? inner[2..].Trim() : ""));
                    }

                    i++;
                }

                if (isList)
                {
                    result[key] = items;
                }

                continue;
            }

            if (value.StartsWith('[') && value.EndsWith(']'))
            {
                result[key] = value[1..^1]
                    .Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
                    .Select(Unquote)
                    .ToList();
                continue;
            }

            result[key] = Unquote(value);
        }

        return result;
    }

    private static string Unquote(string value)
    {
        if (value.Length >= 2 && ((value[0] == '"' && value[^1] == '"') || (value[0] == '\'' && value[^1] == '\'')))
        {
            string inner = value[1..^1];
            return value[0] == '"' ? inner.Replace("\\\"", "\"").Replace("\\n", "\n") : inner.Replace("''", "'");
        }

        return value;
    }
}
