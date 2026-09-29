using System.Text.Json.Nodes;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Office;

namespace AxiomOffice.Core.Tools;

// Ket qua mot lan chay tool. Json = nguyen van de gui cho model; Action = ten lenh office (neu co) de ghi audit.
public sealed record ToolResult(string Json, bool Ok, string? Action = null);

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
}

public interface ITool
{
    string Name { get; }

    string Description { get; }

    JsonNode ParametersSchema { get; }

    Task<ToolResult> InvokeAsync(JsonNode? arguments, RunContext context, CancellationToken cancel);
}
