using System.Net;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Tests;

// Thu lai khi nha cung cap loi tam thoi (503/429/500...) va loc <thought> khoi cau tra loi (log 30/09:
// gemini-3.8-flash 503 "high demand", gemma-4 500 ngau nhien va tra kem <thought>...</thought>).
public class ModelRetryTests
{
    private static readonly ModelTool Tool = new("office_action", "doc va sua tai lieu", new JsonObject { ["type"] = "object" });

    private static ModelClient Client(ScriptedHandler handler, int retries = 3)
    {
        var http = new HttpClient(handler) { Timeout = Timeout.InfiniteTimeSpan };
        return new ModelClient(http, "openai", "http://llm.test/v1", "k", "model-test")
        {
            RetryDelays = Enumerable.Repeat(TimeSpan.FromMilliseconds(10), retries).ToList(),
        };
    }

    private static Task<AgentLoopResult> Run(ModelClient client, CancellationToken cancel = default)
    {
        return client.RunAgentAsync("s", [], "p", [Tool],
            (c, _) => Task.FromResult(new ToolCallResult(c.Id, c.Name, "{}", true, 0)),
            new AgentLoopOptions(Deadline: TimeSpan.FromSeconds(30)), new AgentLoopCallbacks(), cancel);
    }

    [Theory]
    [InlineData(HttpStatusCode.ServiceUnavailable)]
    [InlineData(HttpStatusCode.TooManyRequests)]
    [InlineData(HttpStatusCode.InternalServerError)]
    [InlineData(HttpStatusCode.BadGateway)]
    public async Task Loi_tam_thoi_thu_lai_roi_thanh_cong(HttpStatusCode status)
    {
        int calls = 0;
        var handler = new ScriptedHandler((_, _) => ++calls <= 2
            ? ScriptedHandler.Json("""{"error":{"message":"high demand"}}""", status)
            : ScriptedHandler.Json(ScriptedHandler.OpenAiText("xong")));

        AgentLoopResult result = await Run(Client(handler));

        Assert.True(result.Ok, result.Error);
        Assert.Equal("xong", result.Text);
        Assert.Equal(3, handler.Count);
    }

    [Fact]
    public async Task Het_so_lan_thu_thi_bao_loi_nha_cung_cap()
    {
        var handler = new ScriptedHandler((_, _) => ScriptedHandler.Json("""{"error":"overloaded"}""", HttpStatusCode.ServiceUnavailable));

        AgentLoopResult result = await Run(Client(handler, retries: 2));

        Assert.False(result.Ok);
        Assert.Equal("provider", result.ErrorKind);
        Assert.Contains("503", result.Error);
        Assert.Equal(3, handler.Count);
    }

    [Theory]
    [InlineData(HttpStatusCode.BadRequest)]
    [InlineData(HttpStatusCode.Unauthorized)]
    [InlineData(HttpStatusCode.NotFound)]
    public async Task Loi_khong_tam_thoi_khong_thu_lai(HttpStatusCode status)
    {
        var handler = new ScriptedHandler((_, _) => ScriptedHandler.Json("""{"error":"bad"}""", status));

        AgentLoopResult result = await Run(Client(handler));

        Assert.False(result.Ok);
        Assert.Equal(1, handler.Count);
    }

    [Fact]
    public async Task Huy_trong_luc_cho_thu_lai_thi_dung_ngay()
    {
        using var cancel = new CancellationTokenSource();
        var handler = new ScriptedHandler((_, _) =>
        {
            cancel.Cancel();
            return ScriptedHandler.Json("{}", HttpStatusCode.ServiceUnavailable);
        });
        var http = new HttpClient(handler) { Timeout = Timeout.InfiniteTimeSpan };
        var client = new ModelClient(http, "openai", "http://llm.test/v1", "k", "m") { RetryDelays = [TimeSpan.FromSeconds(30)] };

        AgentLoopResult result = await Run(client, cancel.Token);

        Assert.True(result.Cancelled);
        Assert.Equal(1, handler.Count);
    }

    [Fact]
    public async Task Retry_After_cua_nha_cung_cap_duoc_ton_trong()
    {
        int calls = 0;
        var handler = new ScriptedHandler((_, _) =>
        {
            if (++calls == 1)
            {
                HttpResponseMessage busy = ScriptedHandler.Json("{}", HttpStatusCode.TooManyRequests);
                busy.Headers.RetryAfter = new System.Net.Http.Headers.RetryConditionHeaderValue(TimeSpan.FromMilliseconds(1));
                return busy;
            }

            return ScriptedHandler.Json(ScriptedHandler.OpenAiText("ok"));
        });
        var http = new HttpClient(handler) { Timeout = Timeout.InfiniteTimeSpan };
        // Delay mac dinh 30s: neu khong dung Retry-After thi test se cham ro rang.
        var client = new ModelClient(http, "openai", "http://llm.test/v1", "k", "m") { RetryDelays = [TimeSpan.FromSeconds(30)] };

        var watch = System.Diagnostics.Stopwatch.StartNew();
        AgentLoopResult result = await Run(client);

        Assert.True(result.Ok, result.Error);
        Assert.True(watch.Elapsed < TimeSpan.FromSeconds(10), watch.Elapsed.ToString());
    }

    [Fact]
    public void Loi_mang_la_tam_thoi_con_het_gio_request_thi_khong()
    {
        Assert.True(ModelClient.IsTransient(0, "HttpRequestException: No connection could be made"));
        Assert.False(ModelClient.IsTransient(0, "request timed out after 60s"));
        Assert.False(ModelClient.IsTransient(0, "cancelled"));
        Assert.False(ModelClient.IsTransient(401, "unauthorized"));
    }

    [Theory]
    [InlineData("<thought>nghi nhieu</thought>Da ghi xong.", "Da ghi xong.")]
    [InlineData("<think>\nabc\n</think>\n\nKet qua", "Ket qua")]
    [InlineData("<THOUGHT>x</THOUGHT>A<thinking>y</thinking>B", "AB")]
    [InlineData("Khong co the nao", "Khong co the nao")]
    [InlineData("<thought>chua dong the", "<thought>chua dong the")]
    public void Loc_phan_suy_nghi(string input, string expected)
    {
        Assert.Equal(expected, ModelText.StripThoughts(input));
    }

    [Fact]
    public async Task Cau_tra_loi_cuoi_da_loc_suy_nghi_con_rong_thi_bao_loi()
    {
        var handler = new ScriptedHandler((_, _) => ScriptedHandler.Json(ScriptedHandler.OpenAiText("<thought>Nguoi dung muon ghi A1.</thought>Da ghi HELLO vao A1.")));
        AgentLoopResult result = await Run(Client(handler));
        Assert.Equal("Da ghi HELLO vao A1.", result.Text);

        var empty = new ScriptedHandler((_, _) => ScriptedHandler.Json(ScriptedHandler.OpenAiText("<thought>chi nghi</thought>")));
        AgentLoopResult none = await Run(Client(empty));
        Assert.False(none.Ok);
        Assert.Contains("empty reply", none.Error);
    }
}
