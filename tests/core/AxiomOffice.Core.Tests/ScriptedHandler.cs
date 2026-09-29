using System.Net;
using System.Text;
using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Tests;

// HttpMessageHandler gia: tra ve response theo kich ban va ghi lai request de kiem tra body.
internal sealed class ScriptedHandler : HttpMessageHandler
{
    private readonly Func<HttpRequestMessage, string, CancellationToken, Task<HttpResponseMessage>> _respond;
    private readonly List<(string Url, string Body)> _requests = [];

    public ScriptedHandler(Func<HttpRequestMessage, string, HttpResponseMessage> respond)
        : this((request, body, _) => Task.FromResult(respond(request, body)))
    {
    }

    public ScriptedHandler(Func<HttpRequestMessage, string, CancellationToken, Task<HttpResponseMessage>> respond)
    {
        _respond = respond;
    }

    public IReadOnlyList<(string Url, string Body)> Requests
    {
        get
        {
            lock (_requests)
            {
                return _requests.ToList();
            }
        }
    }

    public int Count
    {
        get
        {
            lock (_requests)
            {
                return _requests.Count;
            }
        }
    }

    public string LastBody
    {
        get
        {
            lock (_requests)
            {
                return _requests.Count == 0 ? "" : _requests[^1].Body;
            }
        }
    }

    protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        string body = request.Content == null ? "" : await request.Content.ReadAsStringAsync(cancellationToken);
        lock (_requests)
        {
            _requests.Add((request.RequestUri?.ToString() ?? "", body));
        }

        return await _respond(request, body, cancellationToken);
    }

    public static HttpResponseMessage Json(string json, HttpStatusCode status = HttpStatusCode.OK)
    {
        return new HttpResponseMessage(status) { Content = new StringContent(json, Encoding.UTF8, "application/json") };
    }

    // Response OpenAI-compatible goi tool.
    public static string OpenAiToolCall(string action, string argumentsJson, string id = "call_1")
    {
        var payload = new JsonObject
        {
            ["choices"] = new JsonArray
            {
                new JsonObject
                {
                    ["message"] = new JsonObject
                    {
                        ["role"] = "assistant",
                        ["content"] = null,
                        ["tool_calls"] = new JsonArray
                        {
                            new JsonObject
                            {
                                ["id"] = id,
                                ["type"] = "function",
                                ["function"] = new JsonObject { ["name"] = action, ["arguments"] = argumentsJson },
                            },
                        },
                    },
                },
            },
            ["usage"] = new JsonObject { ["prompt_tokens"] = 100, ["completion_tokens"] = 20 },
        };
        return payload.ToJsonString();
    }

    public static string OpenAiText(string text)
    {
        var payload = new JsonObject
        {
            ["choices"] = new JsonArray
            {
                new JsonObject { ["message"] = new JsonObject { ["role"] = "assistant", ["content"] = text } },
            },
            ["usage"] = new JsonObject { ["prompt_tokens"] = 50, ["completion_tokens"] = 10 },
        };
        return payload.ToJsonString();
    }

    public static string AnthropicToolCall(string action, string inputJson, string id = "toolu_1")
    {
        var payload = new JsonObject
        {
            ["content"] = new JsonArray
            {
                new JsonObject { ["type"] = "text", ["text"] = "Dang lam..." },
                new JsonObject
                {
                    ["type"] = "tool_use",
                    ["id"] = id,
                    ["name"] = action,
                    ["input"] = JsonNode.Parse(inputJson),
                },
            },
            ["usage"] = new JsonObject { ["input_tokens"] = 120, ["output_tokens"] = 30 },
        };
        return payload.ToJsonString();
    }

    public static string AnthropicText(string text)
    {
        var payload = new JsonObject
        {
            ["content"] = new JsonArray { new JsonObject { ["type"] = "text", ["text"] = text } },
            ["usage"] = new JsonObject { ["input_tokens"] = 40, ["output_tokens"] = 8 },
        };
        return payload.ToJsonString();
    }
}
