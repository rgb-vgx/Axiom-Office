using System.Net;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Agent;
using AxiomOffice.Core.Api;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Memory;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Office;

namespace AxiomOffice.Core.Tests;

// Luot chay that voi bridge gia (HttpClient gia) va model gia: kiem tra thu tu su kien, hoi thoai,
// audit, va cac truong hop loi (New_arch.md muc 8.1, 7.4).
public class OrchestratorTests : IDisposable
{
    private const int Port = 47833;

    private readonly string _dir = Path.Combine(Path.GetTempPath(), "axiom-orch-" + Guid.NewGuid().ToString("N")[..8]);
    private readonly CorePaths _paths;
    private readonly CoreStores _stores;

    public OrchestratorTests()
    {
        Directory.CreateDirectory(_dir);
        _paths = CorePaths.Create(_dir);
        _paths.EnsureDirectories();
        _stores = new CoreStores(_paths);
    }

    public void Dispose()
    {
        try
        {
            Directory.Delete(_dir, recursive: true);
        }
        catch
        {
        }

        GC.SuppressFinalize(this);
    }

    // Session file tro vao TIENTRINH TEST (dang song) nen SessionDirectory tim thay.
    private SessionDirectory WriteSessionFile(string document = @"C:\docs\bao-cao.docx")
    {
        string sessionsDir = Path.Combine(_dir, "sessions");
        Directory.CreateDirectory(sessionsDir);
        var session = new JsonObject
        {
            ["pid"] = Environment.ProcessId,
            ["app"] = "et",
            ["family"] = "office",
            ["port"] = Port,
            ["host"] = "EXCEL.EXE",
            ["version"] = "1.0.0",
            ["lastSeenEpoch"] = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() / 1000.0,
            ["document"] = Path.GetFileName(document),
            ["documentPath"] = document,
        };
        File.WriteAllText(Path.Combine(sessionsDir, Environment.ProcessId + ".json"), session.ToJsonString());
        return new SessionDirectory(sessionsDir);
    }

    private static string CommandCatalogJson()
    {
        return """
            {"ok":true,"result":{"version":"1.0.0","commands":[
              {"name":"et.writeRange","kind":"et","agent":true,"summary":"ghi vung","params":[{"name":"range","required":true,"hint":"top-left cell e.g. 'A1'"},{"name":"values","required":true,"hint":"2D array of rows"}]},
              {"name":"et.save","kind":"et","agent":false,"summary":"luu","params":[]}
            ]}}
            """;
    }

    // Handler dinh tuyen: URL 127.0.0.1 = bridge gia, con lai = model gia.
    private static ScriptedHandler RoutingHandler(
        Func<HttpRequestMessage, string, CancellationToken, Task<HttpResponseMessage>> llmRespond,
        Action<JsonObject>? onCommand = null,
        bool healthy = true)
    {
        return new ScriptedHandler((request, body, ct) =>
        {
            string path = request.RequestUri!.AbsolutePath;
            if (request.RequestUri.Host == "127.0.0.1")
            {
                if (path == "/health")
                {
                    if (!healthy)
                    {
                        return Task.FromResult(ScriptedHandler.Json("""{"ok":false,"error":"khong tra loi"}""", HttpStatusCode.ServiceUnavailable));
                    }

                    return Task.FromResult(ScriptedHandler.Json(new JsonObject
                    {
                        ["ok"] = true,
                        ["result"] = new JsonObject { ["pid"] = Environment.ProcessId, ["port"] = Port },
                    }.ToJsonString()));
                }

                if (path == "/commands")
                {
                    return Task.FromResult(ScriptedHandler.Json(CommandCatalogJson()));
                }

                if (path == "/cmd")
                {
                    onCommand?.Invoke(JsonNode.Parse(body)!.AsObject());
                    return Task.FromResult(ScriptedHandler.Json("""{"ok":true,"result":{"written":2}}"""));
                }

                return Task.FromResult(ScriptedHandler.Json("""{"ok":false,"error":"not found"}""", HttpStatusCode.NotFound));
            }

            return llmRespond(request, body, ct);
        });
    }

    private Orchestrator Build(ScriptedHandler handler)
    {
        var config = new CoreConfig { Token = "t", LlmEndpoint = "http://llm.test/v1", LlmModel = "model-test", LlmApiKey = "k" };
        var http = new HttpClient(handler) { Timeout = Timeout.InfiniteTimeSpan };
        var bridge = new BridgeClient(config, http);
        var model = new ModelClient(http, "openai", config.LlmEndpoint, config.LlmApiKey, config.LlmModel);
        return new Orchestrator(config, bridge, WriteSessionFile(), _stores.Conversations, _stores.Runs, () => model, new ContextAssembler());
    }

    private (Orchestrator Orchestrator, ScriptedHandler Handler) CreateOrchestrator(
        Func<HttpRequestMessage, string, HttpResponseMessage> llmRespond,
        Action<JsonObject>? onCommand = null,
        bool healthy = true)
    {
        ScriptedHandler handler = RoutingHandler((request, body, _) => Task.FromResult(llmRespond(request, body)), onCommand, healthy);
        return (Build(handler), handler);
    }

    private static RunRequest Request(string prompt = "ghi bang diem", string? conversationId = null, int? maxTokens = null)
    {
        return new RunRequest(
            Prompt: prompt,
            ConversationId: conversationId,
            Port: Port,
            Pid: Environment.ProcessId,
            App: "et",
            Family: "office",
            Document: new DocumentContext("bao-cao.docx", @"C:\docs\bao-cao.docx", null),
            Options: new AgentLoopOptions(MaxTokens: maxTokens ?? 200_000, Deadline: TimeSpan.FromSeconds(30)));
    }

    [Fact]
    public async Task Luot_chay_thanh_cong_ghi_hoi_thoai_audit_va_phat_du_su_kien()
    {
        int calls = 0;
        (Orchestrator orchestrator, ScriptedHandler handler) = CreateOrchestrator((_, _) => ScriptedHandler.Json(
            ++calls == 1
                ? ScriptedHandler.OpenAiToolCall("office_action", """{"action":"et.writeRange","params":{"range":"A1","values":[["a",1]]}}""")
                : ScriptedHandler.OpenAiText("Da ghi bang diem")));

        var manager = new RunManager();
        (RunState? run, _) = manager.TryStart(Port);
        Assert.NotNull(run);

        await orchestrator.ExecuteAsync(run!, Request(), CancellationToken.None);

        Assert.Equal(RunStatus.Completed, run!.Status);
        Assert.Equal("Da ghi bang diem", run.Reply);
        Assert.Equal(2, run.Rounds);
        Assert.Equal(1, run.ToolCalls);
        Assert.NotNull(run.ConversationId);

        // Su kien dung thu tu va co day du thong tin cho pane.
        string[] types = run.Events.Since(0).Select(e => e.Type).ToArray();
        Assert.Equal(["run.started", "tool.started", "tool.finished", "run.completed"], types);
        JsonNode started = run.Events.Since(0)[0].Data!;
        Assert.Equal(run.ConversationId, started["conversationId"]!.GetValue<string>());
        JsonNode toolStarted = run.Events.Since(0)[1].Data!;
        Assert.Equal("et.writeRange", toolStarted["action"]!.GetValue<string>());
        Assert.True(run.Events.Completed);

        // Hoi thoai: user -> tom tat tool -> assistant.
        IReadOnlyList<MessageRow> messages = _stores.Conversations.Messages(run.ConversationId!);
        Assert.Equal(["user", "tool_summary", "assistant"], messages.Select(m => m.Role));
        Assert.Contains("et.writeRange", messages[1].Content);

        // Audit + trang thai luot chay trong DB.
        RunRow? row = _stores.Runs.Get(run.Id);
        Assert.Equal("completed", row!.Status);
        Assert.Equal(2, row.Rounds);
        IReadOnlyList<ToolCallRow> calls2 = _stores.Runs.ToolCalls(run.Id);
        Assert.Single(calls2);
        Assert.Equal("et.writeRange", calls2[0].Action);

        // Lenh duoc gui nguyen van xuong bridge kem token.
        Assert.Contains("\"action\":\"et.writeRange\"", handler.Requests.First(r => r.Url.EndsWith("/cmd")).Body);
    }

    [Fact]
    public async Task Tiep_tuc_hoi_thoai_cu_thi_dua_ngu_canh_vao_prompt()
    {
        int calls = 0;
        (Orchestrator orchestrator, ScriptedHandler handler) = CreateOrchestrator((_, _) => ScriptedHandler.Json(
            ++calls == 1 ? ScriptedHandler.OpenAiText("Da tao bang diem xong") : ScriptedHandler.OpenAiText("Da them cot trung binh")));

        var manager = new RunManager();
        (RunState? first, _) = manager.TryStart(Port);
        await orchestrator.ExecuteAsync(first!, Request("tao bang diem"), CancellationToken.None);
        Assert.Equal(RunStatus.Completed, first!.Status);

        (RunState? second, _) = manager.TryStart(Port);
        await orchestrator.ExecuteAsync(second!, Request("lam tiep: them cot trung binh", first.ConversationId), CancellationToken.None);

        // Luot thu hai gui lai hoi thoai cu trong body (prompt + cau tra loi truoc).
        string lastBody = handler.Requests.Last(r => r.Url.Contains("llm.test")).Body;
        Assert.Contains("tao bang diem", lastBody);
        Assert.Contains("Da tao bang diem xong", lastBody);
        Assert.Equal(first.ConversationId, second!.ConversationId);
        // Hai luot: user + assistant moi luot (khong tinh dong tom tat tool).
        Assert.Equal(4, _stores.Conversations.Messages(first.ConversationId!).Count(m => m.Role != "tool_summary"));
    }

    [Fact]
    public async Task Khong_tim_thay_session_thi_bao_loi_office()
    {
        (Orchestrator orchestrator, _) = CreateOrchestrator((_, _) => ScriptedHandler.Json(ScriptedHandler.OpenAiText("khong bao gio toi day")));
        var manager = new RunManager();
        (RunState? run, _) = manager.TryStart(47999);   // port khong co session

        await orchestrator.ExecuteAsync(run!, Request() with { Port = 47999 }, CancellationToken.None);

        Assert.Equal(RunStatus.Failed, run!.Status);
        Assert.Equal("office", run.ErrorKind);
        Assert.Contains("office session not found", run.Error);
        Assert.Contains(run.Events.Since(0), e => e.Type == "run.failed");
    }

    [Fact]
    public async Task Bridge_khong_tra_loi_health_thi_bao_loi_office()
    {
        (Orchestrator orchestrator, _) = CreateOrchestrator((_, _) => ScriptedHandler.Json(ScriptedHandler.OpenAiText("x")), healthy: false);
        var manager = new RunManager();
        (RunState? run, _) = manager.TryStart(Port);

        await orchestrator.ExecuteAsync(run!, Request(), CancellationToken.None);

        Assert.Equal("office", run!.ErrorKind);
        Assert.Contains("not answering /health", run.Error);
    }

    [Fact]
    public async Task Model_loi_thi_luu_trang_thai_failed_va_khong_co_tin_assistant()
    {
        (Orchestrator orchestrator, _) = CreateOrchestrator((_, _) =>
            ScriptedHandler.Json("""{"error":"het quota"}""", HttpStatusCode.TooManyRequests));
        var manager = new RunManager();
        (RunState? run, _) = manager.TryStart(Port);

        await orchestrator.ExecuteAsync(run!, Request(), CancellationToken.None);

        Assert.Equal(RunStatus.Failed, run!.Status);
        Assert.Equal("provider", run.ErrorKind);
        Assert.Contains("HTTP 429", run.Error);
        Assert.Equal("failed", _stores.Runs.Get(run.Id)!.Status);
        IReadOnlyList<MessageRow> messages = _stores.Conversations.Messages(run.ConversationId!);
        Assert.DoesNotContain(messages, m => m.Role == "assistant");
    }

    [Fact]
    public async Task Huy_giua_luot_thi_luu_cancelled()
    {
        // Bridge tra loi ngay, chi model cham - de huy xay ra trong luc cho model.
        ScriptedHandler handler = RoutingHandler(async (_, _, ct) =>
        {
            await Task.Delay(5000, ct);
            return ScriptedHandler.Json(ScriptedHandler.OpenAiText("muon"));
        });
        Orchestrator orchestrator = Build(handler);
        var manager = new RunManager();
        (RunState? run, _) = manager.TryStart(Port);

        Task work = orchestrator.ExecuteAsync(run!, Request(), run!.Cancel.Token);
        await Task.Delay(150);
        Assert.True(manager.Cancel(run.Id));
        await work;

        Assert.True(run.Status == RunStatus.Cancelled, $"status={run.Status} kind={run.ErrorKind} error={run.Error}");
        Assert.Contains(run.Events.Since(0), e => e.Type == "run.cancelled");
        Assert.Equal("cancelled", _stores.Runs.Get(run.Id)!.Status);
    }

    [Fact]
    public async Task Tran_token_phat_run_stopped()
    {
        (Orchestrator orchestrator, _) = CreateOrchestrator((_, _) =>
            ScriptedHandler.Json(ScriptedHandler.OpenAiToolCall("office_action", """{"action":"et.writeRange"}""")));
        var manager = new RunManager();
        (RunState? run, _) = manager.TryStart(Port);

        await orchestrator.ExecuteAsync(run!, Request(maxTokens: 50), CancellationToken.None);

        Assert.Equal(RunStatus.Stopped, run!.Status);
        Assert.Contains(run.Events.Since(0), e => e.Type == "run.stopped");
    }

    [Fact]
    public void RunManager_chi_cho_mot_luot_moi_port()
    {
        var manager = new RunManager();
        (RunState? first, string? error) = manager.TryStart(Port);
        (RunState? second, string? secondError) = manager.TryStart(Port);

        Assert.NotNull(first);
        Assert.Null(error);
        Assert.Null(second);
        Assert.Contains("busy", secondError);

        // Port khac chay song song duoc.
        (RunState? other, _) = manager.TryStart(Port + 1);
        Assert.NotNull(other);

        first!.Status = RunStatus.Completed;
        first.FinishedUtc = DateTime.UtcNow;
        (RunState? third, _) = manager.TryStart(Port);
        Assert.NotNull(third);
    }

    [Fact]
    public void RunManager_huy_luot_dang_chay()
    {
        var manager = new RunManager();
        (RunState? run, _) = manager.TryStart(Port);

        Assert.True(manager.Cancel(run!.Id));
        Assert.True(run.Cancel.IsCancellationRequested);
        run.Status = RunStatus.Cancelled;
        Assert.False(manager.Cancel(run.Id));
    }

    [Fact]
    public void SessionDirectory_doc_file_va_loc_theo_tien_trinh()
    {
        SessionDirectory sessions = WriteSessionFile();
        OfficeSession? found = sessions.Find(Port);

        Assert.NotNull(found);
        Assert.Equal("et", found!.App);
        Assert.Equal("office", found.Family);
        Assert.Equal(@"C:\docs\bao-cao.docx", found.DocumentPath);
        Assert.Contains("bao-cao.docx", found.Describe());

        // File cua tien trinh da chet bi bo qua.
        string dead = Path.Combine(sessions.Directory, "999999.json");
        File.WriteAllText(dead, new JsonObject
        {
            ["pid"] = 999999,
            ["port"] = 47899,
            ["app"] = "wps",
            ["family"] = "office",
            ["lastSeenEpoch"] = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() / 1000.0,
        }.ToJsonString());
        Assert.Null(sessions.Find(47899));
        Assert.DoesNotContain(sessions.List(), s => s.Port == 47899);
    }

    [Fact]
    public async Task RunEventStream_phat_lai_su_kien_cu_cho_client_ket_noi_lai()
    {
        var stream = new RunEventStream();
        stream.Publish("run.started", new JsonObject { ["a"] = 1 });
        stream.Publish("tool.started", null);
        stream.Publish("run.completed", null);
        stream.Complete();

        List<RunEvent> replayed = [];
        await foreach (RunEvent item in stream.SubscribeAsync(afterSeq: 1, CancellationToken.None))
        {
            replayed.Add(item);
        }

        Assert.Equal(2, replayed.Count);                 // bo su kien seq 1, phat lai tu seq 2
        Assert.Equal([2L, 3L], replayed.Select(e => e.Seq));
        Assert.Equal("tool.started", replayed[0].Type);
    }

    [Fact]
    public async Task RunEventStream_ket_thuc_khi_luot_chay_xong()
    {
        var stream = new RunEventStream();
        List<RunEvent> received = [];

        Task reading = Task.Run(async () =>
        {
            await foreach (RunEvent item in stream.SubscribeAsync(afterSeq: 0, CancellationToken.None))
            {
                received.Add(item);
            }
        });

        stream.Publish("run.started", null);
        await Task.Delay(50);
        stream.Publish("run.completed", null);
        stream.Complete();

        await reading.WaitAsync(TimeSpan.FromSeconds(5));
        Assert.Equal(["run.started", "run.completed"], received.Select(e => e.Type));
    }
}
