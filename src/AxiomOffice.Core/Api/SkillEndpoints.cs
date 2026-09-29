using System.Text.Json.Nodes;
using AxiomOffice.Core.Skills;

namespace AxiomOffice.Core.Api;

// GET /v1/skills?app=wps|et|wpp, POST /v1/skills/reload (New_arch.md muc 7.3, 8.4.4).
public static class SkillEndpoints
{
    public static void Map(WebApplication app, SkillIndex index)
    {
        app.MapGet("/v1/skills", (HttpContext context) =>
        {
            string? appKind = context.Request.Query["app"].FirstOrDefault();
            return ApiJson.Ok(Listing(index, string.IsNullOrWhiteSpace(appKind) ? null : appKind.Trim()));
        });

        app.MapPost("/v1/skills/reload", () =>
        {
            index.Reload();
            return ApiJson.Ok(Listing(index, null));
        });
    }

    public static JsonObject Listing(SkillIndex index, string? appKind)
    {
        var skills = new JsonArray();
        foreach (SkillDefinition skill in appKind == null ? index.All : index.ForApp(appKind))
        {
            var apps = new JsonArray();
            foreach (string app in skill.Apps)
            {
                apps.Add(app);
            }

            skills.Add(new JsonObject
            {
                ["name"] = skill.Name,
                ["description"] = skill.Description,
                ["apps"] = apps,
                ["source"] = skill.Source,
                ["directory"] = skill.Directory,
            });
        }

        var errors = new JsonArray();
        foreach (SkillError error in index.Errors)
        {
            errors.Add(new JsonObject
            {
                ["directory"] = error.Directory,
                ["source"] = error.Source,
                ["error"] = error.Error,
            });
        }

        var sources = new JsonArray();
        foreach (SkillSource source in index.Sources)
        {
            sources.Add(new JsonObject
            {
                ["name"] = source.Name,
                ["directory"] = source.Directory,
                ["exists"] = Directory.Exists(source.Directory),
            });
        }

        return new JsonObject { ["skills"] = skills, ["errors"] = errors, ["sources"] = sources };
    }
}
