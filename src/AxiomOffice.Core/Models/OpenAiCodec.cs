using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Models;

// OpenAI-compatible (/chat/completions): tools dang {"type":"function","function":{...}},
// ket qua tool la message {"role":"tool","tool_call_id":...}.
public sealed class OpenAiCodec : IProviderCodec
{
    public string Name => "openai";

    public string Path => "/chat/completions";

    public bool IsToolUnsupported(string error)
    {
        return error.Contains("tool", StringComparison.OrdinalIgnoreCase);
    }

    public JsonObject BuildRequest(string model, string systemPrompt, IReadOnlyList<JsonNode> turns, IReadOnlyList<ModelTool> tools, bool toolsEnabled, int maxTokens)
    {
        var messages = new JsonArray();
        messages.Add(new JsonObject { ["role"] = "system", ["content"] = systemPrompt });
        foreach (JsonNode turn in turns)
        {
            messages.Add(turn.DeepClone());
        }

        var body = new JsonObject
        {
            ["model"] = model,
            ["messages"] = messages,
        };

        if (toolsEnabled && tools.Count > 0)
        {
            var definitions = new JsonArray();
            foreach (ModelTool tool in tools)
            {
                definitions.Add(new JsonObject
                {
                    ["type"] = "function",
                    ["function"] = new JsonObject
                    {
                        ["name"] = tool.Name,
                        ["description"] = tool.Description,
                        ["parameters"] = tool.ParametersSchema.DeepClone(),
                    },
                });
            }

            body["tools"] = definitions;
        }

        if (maxTokens > 0)
        {
            body["max_tokens"] = maxTokens;
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

        JsonNode? message = root?["choices"]?[0]?["message"];
        if (message == null)
        {
            error = "unexpected provider response (no choices[0].message)";
            return null;
        }

        var calls = new List<ToolCall>();
        if (message["tool_calls"] is JsonArray toolCalls)
        {
            foreach (JsonNode? item in toolCalls)
            {
                if (item == null)
                {
                    continue;
                }

                calls.Add(new ToolCall(
                    Id: item["id"]?.GetValue<string>() ?? "",
                    Name: item["function"]?["name"]?.GetValue<string>() ?? "",
                    ArgumentsJson: item["function"]?["arguments"]?.GetValue<string>() ?? "{}"));
            }
        }

        string? text = message["content"]?.GetValue<string>();
        return new ModelTurn(text, calls, Usage(root, "prompt_tokens"), Usage(root, "completion_tokens"), message.DeepClone());
    }

    public void AppendAssistant(List<JsonNode> turns, ModelTurn turn)
    {
        if (turn.RawAssistantContent is JsonObject assistant)
        {
            turns.Add(assistant.DeepClone());
        }
    }

    public void AppendToolResults(List<JsonNode> turns, IReadOnlyList<ToolCallResult> results)
    {
        foreach (ToolCallResult result in results)
        {
            turns.Add(new JsonObject
            {
                ["role"] = "tool",
                ["tool_call_id"] = result.CallId,
                ["content"] = result.ResultJson,
            });
        }
    }

    private static int Usage(JsonNode? root, string name)
    {
        return root?["usage"]?[name]?.GetValue<int>() ?? 0;
    }
}
