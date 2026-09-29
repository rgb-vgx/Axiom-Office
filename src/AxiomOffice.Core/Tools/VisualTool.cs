using System.Text.Json.Nodes;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Office;

namespace AxiomOffice.Core.Tools;

// look_at_document: chup cua so app (bridge app.screenshot) va gui ANH cho model de soat bo cuc bang mat
// (QA thi giac, New_arch.md muc 8.4.6 giai doan 4). Chi dang ky khi nguoi dung bat VisualQaEnabled (ton token).
public sealed class VisualTool : ITool
{
    public const string ToolName = "look_at_document";
    public const int MaxWidth = 1280;

    public string Name => ToolName;

    public string Description =>
        "Take a screenshot of the document window and look at it, to check the visual layout (text overflowing, "
        + "overlapping shapes, alignment, contrast) after creating or redesigning slides, tables or pages. "
        + "It costs many tokens: call it at most once or twice per task, after the structural checks.";

    public JsonNode ParametersSchema => new JsonObject
    {
        ["type"] = "object",
        ["properties"] = new JsonObject
        {
            ["reason"] = new JsonObject { ["type"] = "string", ["description"] = "What you want to check" },
        },
    };

    public async Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel)
    {
        BridgeResult shot = await context.Bridge.CommandAsync(context.Office.Port, "app.screenshot", new JsonObject { ["maxWidth"] = MaxWidth }, cancel).ConfigureAwait(false);
        string? base64 = shot.Result?["base64"]?.GetValue<string>();
        if (!shot.Ok || string.IsNullOrEmpty(base64))
        {
            return new ToolResult(ModelClient.ErrorJson("screenshot unavailable: " + (shot.Error ?? "no image")), false, ToolName);
        }

        string mime = shot.Result?["mime"]?.GetValue<string>() ?? "image/png";
        var summary = new JsonObject
        {
            ["ok"] = true,
            ["result"] = new JsonObject
            {
                ["width"] = shot.Result?["width"]?.DeepClone(),
                ["height"] = shot.Result?["height"]?.DeepClone(),
                ["note"] = "The screenshot is attached as an image.",
            },
        };
        return new ToolResult(summary.ToJsonString(ModelClient.RelaxedJson), true, ToolName, "data:" + mime + ";base64," + base64);
    }
}

public static class ImageData
{
    // "data:image/png;base64,AAAA" -> (image/png, AAAA).
    public static bool TrySplit(string dataUrl, out string mediaType, out string base64)
    {
        mediaType = "";
        base64 = "";
        const string prefix = "data:";
        int marker = dataUrl.IndexOf(";base64,", StringComparison.Ordinal);
        if (!dataUrl.StartsWith(prefix, StringComparison.Ordinal) || marker < 0)
        {
            return false;
        }

        mediaType = dataUrl[prefix.Length..marker];
        base64 = dataUrl[(marker + ";base64,".Length)..];
        return mediaType.Length > 0 && base64.Length > 0;
    }
}
