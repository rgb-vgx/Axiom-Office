using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Models;

// Tool ma model duoc thay trong mot luot chay (New_arch.md muc 8.3).
public sealed record ModelTool(string Name, string Description, JsonNode ParametersSchema);

// Model yeu cau goi mot tool.
public sealed record ToolCall(string Id, string Name, string ArgumentsJson);

// Ket qua tra ve model sau khi chay tool. Ok = false khi tool loi (khong lam hong luot chay).
// ImageDataUrl: anh (data:image/png;base64,...) tool tra kem - codec gui cho model duoi dang anh that, khong vao audit.
public sealed record ToolCallResult(string CallId, string Name, string ResultJson, bool Ok, long Ms, string? ImageDataUrl = null);

// Mot luot tra loi cua model: hoac co tool call, hoac co cau tra loi cuoi.
// RawAssistantContent giu nguyen phan assistant de lan sau gui lai y nguyen (moi giao thuc mot dang).
public sealed record ModelTurn(
    string? Text,
    IReadOnlyList<ToolCall> ToolCalls,
    int InputTokens,
    int OutputTokens,
    JsonNode? RawAssistantContent);
