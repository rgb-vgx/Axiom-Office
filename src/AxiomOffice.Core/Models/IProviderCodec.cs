using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Models;

// Codec cua tung giao thuc LLM: dung body, doc response, ghep lich su hoi thoai theo dung dinh dang
// moi ben (OpenAI-compatible va Anthropic khac nhau o tool call / tool result).
// Vong lap agent nam o ModelClient va khong biet chi tiet giao thuc.
public interface IProviderCodec
{
    // Ten de log/goi y khi loi cau hinh ("openai", "anthropic").
    string Name { get; }

    // Duong dan noi vao endpoint (vd "/chat/completions").
    string Path { get; }

    // Provider bao khong ho tro tools -> tat tools va chay nhu chat thuong (giu hanh vi da kiem chung).
    bool IsToolUnsupported(string error);

    // Body cua mot lan goi. turns la lich su hoi thoai dang JsonObject theo dinh dang cua codec.
    JsonObject BuildRequest(string model, string systemPrompt, IReadOnlyList<JsonNode> turns, IReadOnlyList<ModelTool> tools, bool toolsEnabled, int maxTokens);

    // Doc response. Loi -> error != null va tra ve null.
    ModelTurn? Parse(string responseJson, out string? error);

    // Ghep luot assistant vua tra loi va ket qua cac tool vao lich su.
    void AppendAssistant(List<JsonNode> turns, ModelTurn turn);

    void AppendToolResults(List<JsonNode> turns, IReadOnlyList<ToolCallResult> results);
}
