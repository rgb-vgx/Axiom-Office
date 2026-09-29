using System.Text.Json.Nodes;
using AxiomOffice.Core.Tools;

namespace AxiomOffice.Core.Models;

// Anthropic (/messages): system la truong rieng, tool call nam trong content block type=tool_use,
// ket qua tool gui lai trong mot message role=user voi block type=tool_result.
public sealed class AnthropicCodec : IProviderCodec
{
    public string Name => "anthropic";

    public string Path => "/messages";

    public bool IsToolUnsupported(string error)
    {
        return error.Contains("tool", StringComparison.OrdinalIgnoreCase);
    }

    public JsonObject BuildRequest(string model, string systemPrompt, IReadOnlyList<JsonNode> turns, IReadOnlyList<ModelTool> tools, bool toolsEnabled, int maxTokens)
    {
        var messages = new JsonArray();
        foreach (JsonNode turn in turns)
        {
            messages.Add(turn.DeepClone());
        }

        var body = new JsonObject
        {
            ["model"] = model,
            ["max_tokens"] = maxTokens > 0 ? maxTokens : 4096,
            ["system"] = systemPrompt,
            ["messages"] = messages,
        };

        if (toolsEnabled && tools.Count > 0)
        {
            var definitions = new JsonArray();
            foreach (ModelTool tool in tools)
            {
                definitions.Add(new JsonObject
                {
                    ["name"] = tool.Name,
                    ["description"] = tool.Description,
                    ["input_schema"] = tool.ParametersSchema.DeepClone(),
                });
            }

            body["tools"] = definitions;
        }

        return body;
    }

    public ModelTurn? Parse(string responseJson, out string? error)
    {
        error = null;
        JsonNode? root;
        try
        {
            root = JsonNode.Parse(responseJson);
        }
        catch (Exception ex)
        {
            error = "invalid provider response: " + ex.Message;
            return null;
        }

        if (root?["content"] is not JsonArray blocks)
        {
            error = "unexpected provider response (no content blocks)";
            return null;
        }

        var texts = new List<string>();
        var calls = new List<ToolCall>();
        foreach (JsonNode? block in blocks)
        {
            if (block == null)
            {
                continue;
            }

            string type = block["type"]?.GetValue<string>() ?? "";
            if (type == "text")
            {
                texts.Add(block["text"]?.GetValue<string>() ?? "");
            }
            else if (type == "tool_use")
            {
                calls.Add(new ToolCall(
                    Id: block["id"]?.GetValue<string>() ?? "",
                    Name: block["name"]?.GetValue<string>() ?? "",
                    ArgumentsJson: block["input"]?.ToJsonString() ?? "{}"));
            }
        }

        string? text = texts.Count > 0 ? string.Join("\n", texts) : null;
        return new ModelTurn(text, calls, Usage(root, "input_tokens"), Usage(root, "output_tokens"), blocks.DeepClone());
    }

    public void AppendAssistant(List<JsonNode> turns, ModelTurn turn)
    {
        if (turn.RawAssistantContent != null)
        {
            turns.Add(new JsonObject
            {
                ["role"] = "assistant",
                ["content"] = turn.RawAssistantContent.DeepClone(),
            });
        }
    }

    public void AppendToolResults(List<JsonNode> turns, IReadOnlyList<ToolCallResult> results)
    {
        var blocks = new JsonArray();
        foreach (ToolCallResult result in results)
        {
            // Anh (QA thi giac, muc 8.4.6): tool_result cua Anthropic nhan duoc khoi image base64.
            JsonNode content = result.ImageDataUrl is { } dataUrl && ImageData.TrySplit(dataUrl, out string mediaType, out string base64)
                ? new JsonArray(
                    new JsonObject { ["type"] = "text", ["text"] = result.ResultJson },
                    new JsonObject
                    {
                        ["type"] = "image",
                        ["source"] = new JsonObject { ["type"] = "base64", ["media_type"] = mediaType, ["data"] = base64 },
                    })
                : JsonValue.Create(result.ResultJson)!;
            blocks.Add(new JsonObject
            {
                ["type"] = "tool_result",
                ["tool_use_id"] = result.CallId,
                ["content"] = content,
            });
        }

        turns.Add(new JsonObject
        {
            ["role"] = "user",
            ["content"] = blocks,
        });
    }

    private static int Usage(JsonNode? root, string name)
    {
        return root?["usage"]?[name]?.GetValue<int>() ?? 0;
    }
}
