using System.Text.Json.Nodes;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Office;

namespace AxiomOffice.Core.Tools;

// Ket qua mot lan chay tool. Json = nguyen van de gui cho model; Action = ten lenh office (neu co) de ghi audit.
public sealed record ToolResult(string Json, bool Ok, string? Action = null, string? ImageDataUrl = null);

// Trang thai cua mot luot chay ma tool can (muc 8.3).
public sealed class RunContext
{
    public required string RunId { get; init; }

    public required string? ConversationId { get; init; }

    public required OfficeSession Office { get; init; }

    public required CoreConfig Config { get; init; }

    public required BridgeClient Bridge { get; init; }

    // Phat event ra SSE cua luot chay (vd memory.written o giai doan 3).
    public Action<string, JsonNode?>? Event { get; init; }

    // Cau yeu cau cua luot (policy "khong tu luu" doc y dinh luu/xuat cua nguoi dung).
    public string Prompt { get; init; } = "";

    // Hoi nguoi dung (confirm.required -> POST /v1/runs/{id}/confirm): action, ly do, params rut gon -> dong y?
    // null = khong co ai de hoi (test) -> coi nhu tu choi.
    public Func<string, string, string?, CancellationToken, Task<bool>>? Confirm { get; init; }

    public async Task<bool> ConfirmAsync(string action, string reason, string? paramsPreview, CancellationToken cancel)
    {
        return Confirm != null && await Confirm(action, reason, paramsPreview, cancel).ConfigureAwait(false);
    }
}

public interface ITool
{
    string Name { get; }

    string Description { get; }

    JsonNode ParametersSchema { get; }

    Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel);
}
