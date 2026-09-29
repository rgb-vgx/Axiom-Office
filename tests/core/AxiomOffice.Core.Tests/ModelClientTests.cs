using System.Net;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Tests;

// Vong lap agent (ModelClient): goi model, chay tool, fallback khong tools, huy, tran token
// (New_arch.md muc 8.1, 8.2). Port tu hanh vi da kiem chung cua Ai/LlmClient.cs.
public class ModelClientTests
{
    private static readonly ModelTool Tool = new("office_action", "doc va sua tai lieu", new JsonObject { ["type"] = "object" });

    private static (ModelClient Client, ScriptedHandler Handler) Create(
        Func<HttpRequestMessage, string, HttpResponseMessage> respond, string provider = "openai")
    {
        var handler = new ScriptedHandler(respond);
        var http = new HttpClient(handler) { Timeout = Timeout.InfiniteTimeSpan };
        return (new ModelClient(http, provider, "http://llm.test/v1", "key-test", "model-test"), handler);
    }

    [Fact]
    public async Task Tool_call_then_answer_chay_tool_va_ket_thuc()
    {
        int llmCalls = 0;
        (ModelClient client, ScriptedHandler handler) = Create((request, _) =>
        {
            if (request.RequestUri!.AbsolutePath.EndsWith("/chat/completions"))
            {
                llmCalls++;
                return llmCalls == 1
                    ? ScriptedHandler.Json(ScriptedHandler.OpenAiToolCall("office_action", """{"action":"et.writeRange","params":{"range":"A1"}}"""))
                    : ScriptedHandler.Json(ScriptedHandler.OpenAiText("Da ghi xong"));
            }

            return ScriptedHandler.Json("""{"ok":true,"result":{"written":2}}""");
        });

        var calls = new List<ToolCall>();
        AgentLoopResult result = await client.RunAgentAsync(
            "system", [], "ghi du lieu", [Tool],
            (call, _) =>
            {
                calls.Add(call);
                return Task.FromResult(new ToolCallResult(call.Id, call.Name, """{"ok":true,"result":{"written":2}}""", true, 0));
            },
            new AgentLoopOptions(Deadline: TimeSpan.FromSeconds(30)),
            new AgentLoopCallbacks(),
            CancellationToken.None);

        Assert.True(result.Ok);
        Assert.Equal("Da ghi xong", result.Text);
        Assert.Equal(2, result.Rounds);
        Assert.Equal(150, result.InputTokens);
        Assert.Equal(30, result.OutputTokens);
        Assert.Single(calls);
        Assert.Equal("et.writeRange", JsonNode.Parse(calls[0].ArgumentsJson)!["action"]!.GetValue<string>());
        Assert.Contains(result.Transcript, line => line.Contains("office_action"));
        // Lan goi thu hai phai mang theo ket qua tool (role=tool) trong body.
        Assert.Contains("\"role\":\"tool\"", handler.LastBody);
    }

    [Fact]
    public async Task Provider_tra_loi_http_thi_bao_loi_kem_ma()
    {
        (ModelClient client, _) = Create((_, _) => ScriptedHandler.Json("""{"error":"bad key"}""", HttpStatusCode.Unauthorized));

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool], (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, "{}", true, 0)),
            new AgentLoopOptions(), new AgentLoopCallbacks(), CancellationToken.None);

        Assert.False(result.Ok);
        Assert.Equal("provider", result.ErrorKind);
        Assert.Contains("HTTP 401", result.Error);
        Assert.Contains("bad key", result.Error);
    }

    [Fact]
    public async Task Provider_khong_ho_tro_tools_thi_tat_tools_va_chay_tiep()
    {
        int calls = 0;
        (ModelClient client, ScriptedHandler handler) = Create((_, _) =>
        {
            calls++;
            return calls == 1
                ? ScriptedHandler.Json("""{"error":{"message":"tools are not supported by this model"}}""", HttpStatusCode.BadRequest)
                : ScriptedHandler.Json(ScriptedHandler.OpenAiText("Tra loi khong tool"));
        });

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool], (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, "{}", true, 0)),
            new AgentLoopOptions(), new AgentLoopCallbacks(), CancellationToken.None);

        Assert.True(result.Ok);
        Assert.Equal("Tra loi khong tool", result.Text);
        Assert.True(result.ToolsDisabled);
        Assert.Contains(result.Transcript, line => line.Contains("khong ho tro tools"));
        // Lan goi thu hai khong con tools trong body.
        Assert.DoesNotContain("\"tools\"", handler.LastBody);
    }

    [Fact]
    public async Task Tran_token_thi_dung_voi_stopped()
    {
        (ModelClient client, _) = Create((_, _) =>
            ScriptedHandler.Json(ScriptedHandler.OpenAiToolCall("office_action", """{"action":"writer.getText"}""")));

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool],
            (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, """{"ok":true}""", true, 0)),
            new AgentLoopOptions(MaxTokens: 50, Deadline: TimeSpan.FromSeconds(30)),
            new AgentLoopCallbacks(), CancellationToken.None);

        Assert.False(result.Ok);
        Assert.True(result.Stopped);
        Assert.Contains("token budget exceeded", result.Error);
    }

    [Fact]
    public async Task Huy_tu_nguoi_dung_tra_ve_cancelled()
    {
        using var cts = new CancellationTokenSource();
        (ModelClient client, _) = Create((_, _) =>
        {
            cts.Cancel();
            return ScriptedHandler.Json(ScriptedHandler.OpenAiToolCall("office_action", """{"action":"writer.getText"}"""));
        });

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool],
            (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, "{}", true, 0)),
            new AgentLoopOptions(Deadline: TimeSpan.FromSeconds(30)), new AgentLoopCallbacks(), cts.Token);

        Assert.False(result.Ok);
        Assert.True(result.Cancelled);
        Assert.Equal("cancelled", result.ErrorKind);
    }

    [Fact]
    public async Task Het_thoi_gian_ca_luot_thi_timedout()
    {
        // Handler ton trong cancellation: request bi cat khi het tran thoi gian ca luot.
        var handler = new ScriptedHandler(async (_, _, ct) =>
        {
            await Task.Delay(1500, ct);
            return ScriptedHandler.Json(ScriptedHandler.OpenAiText("muon"));
        });
        var http = new HttpClient(handler) { Timeout = Timeout.InfiniteTimeSpan };
        var client = new ModelClient(http, "openai", "http://llm.test/v1", "key-test", "model-test");

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool],
            (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, "{}", true, 0)),
            new AgentLoopOptions(Deadline: TimeSpan.FromMilliseconds(300)), new AgentLoopCallbacks(), CancellationToken.None);

        Assert.False(result.Ok);
        Assert.True(result.TimedOut);
        Assert.Equal("timeout", result.ErrorKind);
        Assert.Contains("timed out", result.Error);
    }

    [Fact]
    public async Task Tra_loi_rong_thi_bao_loi_ro_rang()
    {
        (ModelClient client, _) = Create((_, _) => ScriptedHandler.Json(ScriptedHandler.OpenAiText("   ")));

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool],
            (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, "{}", true, 0)),
            new AgentLoopOptions(), new AgentLoopCallbacks(), CancellationToken.None);

        Assert.False(result.Ok);
        Assert.Equal("provider returned an empty reply", result.Error);
    }

    [Fact]
    public async Task Tool_nem_loi_khong_lam_hong_luot_chay()
    {
        int calls = 0;
        (ModelClient client, _) = Create((_, _) => ScriptedHandler.Json(
            ++calls == 1
                ? ScriptedHandler.OpenAiToolCall("office_action", "{}")
                : ScriptedHandler.OpenAiText("Van xong")));

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool],
            (_, _) => throw new InvalidOperationException("bridge sap"),
            new AgentLoopOptions(), new AgentLoopCallbacks(), CancellationToken.None);

        Assert.True(result.Ok);
        Assert.Contains(result.Transcript, line => line.Contains("bridge sap"));
    }

    [Fact]
    public async Task Thieu_cau_hinh_thi_bao_config()
    {
        var http = new HttpClient(new ScriptedHandler((_, _) => ScriptedHandler.Json("{}")));
        var client = new ModelClient(http, "openai", "", "", "");

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool],
            (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, "{}", true, 0)),
            new AgentLoopOptions(), new AgentLoopCallbacks(), CancellationToken.None);

        Assert.False(result.Ok);
        Assert.Equal("config", result.ErrorKind);
        Assert.Equal("Endpoint is not configured", result.Error);
    }

    [Fact]
    public async Task Anthropic_tool_call_chay_qua_vong_lap()
    {
        int calls = 0;
        (ModelClient client, ScriptedHandler handler) = Create((request, _) =>
        {
            Assert.Equal("2023-06-01", request.Headers.GetValues("anthropic-version").First());
            Assert.Equal("key-test", request.Headers.GetValues("x-api-key").First());
            Assert.Equal("/v1/messages", request.RequestUri!.AbsolutePath);
            return ScriptedHandler.Json(++calls == 1
                ? ScriptedHandler.AnthropicToolCall("office_action", """{"action":"wpp.addSlide","params":{"layout":12}}""")
                : ScriptedHandler.AnthropicText("Da them slide"));
        }, provider: "anthropic");

        AgentLoopResult result = await client.RunAgentAsync("s", [], "p", [Tool],
            (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, """{"ok":true,"result":{"slide":3}}""", true, 0)),
            new AgentLoopOptions(Deadline: TimeSpan.FromSeconds(30)), new AgentLoopCallbacks(), CancellationToken.None);

        Assert.True(result.Ok);
        Assert.Equal("Da them slide", result.Text);
        Assert.Equal(160, result.InputTokens);
        Assert.Contains("tool_result", handler.LastBody);
    }
}
