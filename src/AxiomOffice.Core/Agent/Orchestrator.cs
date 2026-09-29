using System.Collections.Concurrent;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Logging;
using AxiomOffice.Core.Mcp;
using AxiomOffice.Core.Memory;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Office;
using AxiomOffice.Core.Skills;
using AxiomOffice.Core.Tools;

namespace AxiomOffice.Core.Agent;

// Chay mot luot agent (New_arch.md muc 8.1): dung prompt, goi model, chay tool qua bridge, phat su
// kien cho pane, luu hoi thoai + audit. Khong goi COM truc tiep - moi thao tac tai lieu qua /cmd.
public sealed class Orchestrator(
    CoreConfig config,
    BridgeClient bridge,
    SessionDirectory sessions,
    ConversationStore conversations,
    RunStore runs,
    Func<ModelClient> models,
    ContextAssembler assembler,
    SkillIndex? skills = null,
    MemoryService? memory = null,
    McpManager? mcp = null)
{
    public async Task ExecuteAsync(RunState run, Api.RunRequest request, CancellationToken cancel)
    {
        // Moi luot lay client theo cau hinh hien tai (doi model trong Cai dat khong can khoi dong lai Core).
        ModelClient model = models();
        var actions = new ConcurrentDictionary<string, (string? Action, string? Params)>(StringComparer.Ordinal);
        try
        {
            OfficeSession? session = sessions.Find(request.Port);
            if (session == null)
            {
                Fail(run, $"office session not found for port {request.Port} (is the app still open?)", "office");
                return;
            }

            JsonNode? health = await bridge.HealthAsync(request.Port, cancel).ConfigureAwait(false);
            if (health == null)
            {
                Fail(run, $"the bridge on port {request.Port} is not answering /health", "office");
                return;
            }

            int healthPid = health["pid"]?.GetValue<int>() ?? 0;
            if (request.Pid is int expectedPid && healthPid != 0 && healthPid != expectedPid)
            {
                Fail(run, $"port {request.Port} is now served by pid {healthPid}, not {expectedPid}", "office");
                return;
            }

            string appKind = session.App.Length > 0 ? session.App : request.App;
            string documentKey = DocumentKey(request.Document?.FullName ?? session.DocumentPath);
            ConversationRow? conversation = request.ConversationId is { Length: > 0 } id ? conversations.Get(id) : null;
            if (conversation == null)
            {
                string newId = conversations.Create(
                    appKind,
                    session.Family,
                    documentKey,
                    request.Document?.Name ?? session.Document,
                    Title(request.Prompt));
                conversation = conversations.Get(newId);
            }

            if (conversation == null)
            {
                Fail(run, "cannot create the conversation record", "internal");
                return;
            }

            run.ConversationId = conversation.Id;
            runs.Insert(run.Id, conversation.Id, model.Model);
            run.Events.Publish("run.started", new JsonObject
            {
                ["conversationId"] = conversation.Id,
                ["model"] = model.Model,
                ["app"] = appKind,
                ["port"] = request.Port,
            });

            int userSeq = conversations.AppendMessage(conversation.Id, "user", request.Prompt);

            // Ngu canh: cac luot TRUOC luot nay (khong lap lai prompt dang gui).
            IReadOnlyList<MessageRow> history = conversations.Messages(conversation.Id, limit: 60);
            MessageRow[] before = history.Where(m => m.Seq < userSeq).ToArray();
            AssembledContext context = assembler.Assemble(conversation, before);

            OfficeCommandCatalog? catalog = await bridge.GetCommandsAsync(request.Port, cancel).ConfigureAwait(false);
            if (catalog == null)
            {
                Fail(run, "cannot read GET /commands from the bridge (uninstall/upgrade the add-in?)", "office");
                return;
            }

            var officeTool = new OfficeActionTool(catalog, appKind);
            // Skill hop app nay: chi muc vao prompt (tang 1), load_skill/read_skill_file nap khi can (tang 2, 3).
            IReadOnlyList<SkillDefinition> appSkills = skills?.ForApp(appKind) ?? [];
            var tools = new List<ITool> { officeTool };
            if (appSkills.Count > 0 && skills != null)
            {
                tools.Add(new LoadSkillTool(skills, appKind));
                tools.Add(new ReadSkillFileTool(skills, appKind));
            }

            // Memory dai han (giai doan 3, muc 8.5): tai lieu chua luu (khong co duong dan) -> khong co memory tai lieu.
            string? memoryDocumentKey = documentKey != null && documentKey.Contains('\\') ? documentKey : null;
            MemoryContext memoryContext = MemoryContext.Empty;
            if (memory is { Enabled: true })
            {
                memoryContext = await memory.ContextAsync(request.Prompt, memoryDocumentKey, cancel).ConfigureAwait(false);
                tools.Add(new RememberTool(memory, memoryDocumentKey));
                tools.Add(new RecallTool(memory, memoryDocumentKey));
            }

            // MCP (giai doan 4, muc 8.7): lan file office + server nguoi dung cau hinh; loi server khong hong run.
            if (mcp != null)
            {
                try
                {
                    tools.AddRange(await mcp.ToolsAsync(cancel).ConfigureAwait(false));
                }
                catch (Exception ex) when (ex is not OperationCanceledException)
                {
                    CoreLog.Error("mcp tools unavailable for run " + run.Id, ex);
                }
            }

            var registry = new ToolRegistry(tools);
            var modelTools = registry.All
                .Select(t => new ModelTool(t.Name, t.Description, t.ParametersSchema))
                .ToList();

            string systemPrompt = PromptBuilder.Build(
                appKind, session, request.Document,
                skills: appSkills.Select(s => new SkillSummary(s.Name, s.Description)).ToList(),
                memories: memoryContext.Lines, context.Summary, context.RecentActionLines);

            var runContext = new RunContext
            {
                RunId = run.Id,
                ConversationId = conversation.Id,
                Office = session,
                Config = config,
                Bridge = bridge,
                Event = (type, data) => run.Events.Publish(type, data),
                Prompt = request.Prompt,
                Confirm = (action, reason, preview, ct) => run.Confirmations.RequestAsync(
                    run.Events, action, reason, preview, TimeSpan.FromSeconds(config.ConfirmTimeoutSeconds), ct),
            };

            var callbacks = new AgentLoopCallbacks(
                Transcript: line => run.Transcript.Add(line),
                ToolStarted: call =>
                {
                    run.ToolCalls++;
                    (string? action, string? parameters) = ReadCall(call);
                    actions[call.Id] = (action, parameters);
                    run.Events.Publish("tool.started", new JsonObject
                    {
                        ["callId"] = call.Id,
                        ["tool"] = call.Name,
                        ["action"] = action,
                        ["paramsPreview"] = ModelClient.Truncate(parameters, 200),
                    });
                },
                ToolFinished: result =>
                {
                    actions.TryGetValue(result.CallId, out (string? Action, string? Params) info);
                    string? error = result.Ok ? null : ExtractError(result.ResultJson);
                    run.Events.Publish("tool.finished", new JsonObject
                    {
                        ["callId"] = result.CallId,
                        ["tool"] = result.Name,
                        ["action"] = info.Action,
                        ["paramsPreview"] = ModelClient.Truncate(info.Params, 200),
                        ["ok"] = result.Ok,
                        ["error"] = error,
                        ["ms"] = result.Ms,
                        // Ban rut gon cua ket qua de pane hien thi giong che do in-process.
                        ["resultPreview"] = ModelClient.Truncate(result.ResultJson, 150),
                    });
                    runs.AddToolCall(run.Id, run.Transcript.Count, result.Name, info.Action, info.Params, result.Ok, error, result.Ms);
                    conversations.AppendMessage(conversation.Id, "tool_summary",
                        (info.Action ?? result.Name) + (result.Ok ? " (ok)" : " (loi: " + ModelClient.Truncate(error, 120) + ")"));
                });

            AgentLoopResult result = await model.RunAgentAsync(
                systemPrompt,
                context.PriorTurns,
                request.Prompt,
                modelTools,
                (call, ct) => ExecuteToolAsync(registry, call, runContext, ct),
                request.Options,
                callbacks,
                cancel).ConfigureAwait(false);

            run.Rounds = result.Rounds;
            run.InputTokens = result.InputTokens;
            run.OutputTokens = result.OutputTokens;
            run.Seconds = result.Seconds;

            if (result.Ok)
            {
                run.Status = RunStatus.Completed;
                run.Reply = result.Text;
                conversations.AppendMessage(conversation.Id, "assistant", result.Text ?? "");
                run.Events.Publish("run.completed", new JsonObject
                {
                    ["reply"] = result.Text,
                    ["rounds"] = result.Rounds,
                    ["seconds"] = Math.Round(result.Seconds, 2),
                    ["inputTokens"] = result.InputTokens,
                    ["outputTokens"] = result.OutputTokens,
                });
            }
            else if (result.Cancelled)
            {
                run.Status = RunStatus.Cancelled;
                run.Events.Publish("run.cancelled", new JsonObject { ["seconds"] = Math.Round(result.Seconds, 2) });
            }
            else if (result.TimedOut)
            {
                run.Status = RunStatus.TimedOut;
                run.Error = result.Error;
                run.ErrorKind = "timeout";
                run.Events.Publish("run.timedout", new JsonObject { ["seconds"] = Math.Round(result.Seconds, 2), ["error"] = result.Error });
            }
            else if (result.Stopped)
            {
                run.Status = RunStatus.Stopped;
                run.Error = result.Error;
                run.ErrorKind = "budget";
                run.Events.Publish("run.stopped", new JsonObject
                {
                    ["error"] = result.Error,
                    ["rounds"] = result.Rounds,
                    ["inputTokens"] = result.InputTokens,
                    ["outputTokens"] = result.OutputTokens,
                });
            }
            else
            {
                Fail(run, result.Error ?? "agent failed", result.ErrorKind.Length == 0 ? "provider" : result.ErrorKind);
            }

            if (result.ToolsDisabled)
            {
                run.Transcript.Add("(provider khong ho tro tools - chay che do thuong)");
            }

            runs.Finish(run.Id, run.Status, run.Rounds, run.InputTokens, run.OutputTokens, run.Error);
            if (memory != null)
            {
                memory.TouchHits(memoryContext.Ids);
                if (result.Ok)
                {
                    memory.QueueExtraction(new ExtractionJob(run.Id, conversation.Id, request.Prompt, result.Text ?? "", memoryDocumentKey));
                }
                else
                {
                    runs.SetMemoryStatus(run.Id, "skipped");
                }

                // Tom tat hoi thoai o hang doi nen (muc 8.5.9): khong bat pane cho sau khi run xong.
                ConversationRow summarizeTarget = conversation;
                memory.Enqueue(ct => MaybeSummarizeAsync(model, summarizeTarget, userSeq, ct));
            }
            else
            {
                await MaybeSummarizeAsync(model, conversation, userSeq, cancel).ConfigureAwait(false);
            }
        }
        catch (Exception ex)
        {
            CoreLog.Error("run " + run.Id + " crashed", ex);
            Fail(run, ex.GetType().Name + ": " + ex.Message, "internal");
        }
        finally
        {
            run.FinishedUtc = DateTime.UtcNow;
            run.Events.Complete();
            CoreLog.Info($"run {run.Id} {run.Status}: {run.Rounds} rounds, {run.ToolCalls} tool calls, "
                + $"{run.InputTokens}+{run.OutputTokens} tokens, {run.Seconds:0.0}s"
                + (run.Error == null ? "" : " - " + run.Error));
        }
    }

    private static async Task<ToolCallResult> ExecuteToolAsync(ToolRegistry registry, ToolCall call, RunContext context, CancellationToken cancel)
    {
        ITool? tool = registry.Find(call.Name);
        if (tool == null)
        {
            return new ToolCallResult(call.Id, call.Name, ModelClient.ErrorJson("unknown tool: " + call.Name), false, 0);
        }

        JsonNode? arguments = null;
        try
        {
            arguments = JsonNode.Parse(call.ArgumentsJson);
        }
        catch
        {
            // Tham so khong phai JSON: tool se bao thieu 'action'.
        }

        ToolResult result = await tool.InvokeAsync(arguments, context, cancel).ConfigureAwait(false);
        return new ToolCallResult(call.Id, call.Name, result.Json, result.Ok, 0);
    }

    // Luu tom tat khi hoi thoai vuot ngan sach ngu canh (muc 8.5.9) - mot lan goi model cuoi luot.
    // Chi tom tat khi hoi thoai vuot qua moc moi (moi SummaryEvery tin nhan), khong phai sau MOI luot:
    // truoc day hoi thoai >20 tin nhan bi tom tat lai o tat ca cac luot sau, ton them mot lan goi model.
    public static bool ShouldSummarize(int firstSeqOfRun, int lastSeq)
    {
        if (lastSeq <= SummaryEvery)
        {
            return false;
        }

        return lastSeq / SummaryEvery > (firstSeqOfRun - 1) / SummaryEvery;
    }

    private const int SummaryEvery = 20;

    private async Task MaybeSummarizeAsync(ModelClient model, ConversationRow conversation, int userSeq, CancellationToken cancel)
    {
        IReadOnlyList<MessageRow> history = conversations.Messages(conversation.Id, limit: 60);
        if (history.Count == 0 || !ShouldSummarize(userSeq, history[^1].Seq))
        {
            return;
        }

        var lines = new List<string>();
        foreach (MessageRow message in history)
        {
            lines.Add(message.Role + ": " + ModelClient.Truncate(message.Content, 500));
        }

        (string? summary, string? error) = await model
            .ChatAsync("You summarize office-document conversations for future context.", string.Join("\n", lines) + "\n\n" + ContextAssembler.SummaryPrompt(history.Count), cancel)
            .ConfigureAwait(false);

        if (summary != null)
        {
            conversations.Rename(conversation.Id, summary: ModelClient.Truncate(summary, 2000));
            CoreLog.Info("conversation " + conversation.Id + " summarized (" + history.Count + " messages)");
        }
        else if (error != null)
        {
            CoreLog.Info("summarize skipped: " + error);
        }
    }

    private static (string? Action, string? Params) ReadCall(ToolCall call)
    {
        try
        {
            JsonNode? arguments = JsonNode.Parse(call.ArgumentsJson);
            string? action = arguments?["action"]?.GetValue<string>();
            // office_action: {action, params}; tool khac (mcp__*, load_skill, remember...): ca doi so la params (audit day du).
            string? parameters = action != null ? arguments?["params"]?.ToJsonString() : arguments?.ToJsonString();
            return (action, parameters);
        }
        catch
        {
            return (null, null);
        }
    }

    private static string? ExtractError(string resultJson)
    {
        try
        {
            JsonNode? node = JsonNode.Parse(resultJson);
            return node?["error"]?.GetValue<string>();
        }
        catch
        {
            return ModelClient.Truncate(resultJson, 200);
        }
    }

    private static void Fail(RunState run, string error, string kind)
    {
        run.Status = RunStatus.Failed;
        run.Error = error;
        run.ErrorKind = kind;
        run.Events.Publish("run.failed", new JsonObject { ["error"] = error, ["kind"] = kind });
        CoreLog.Error($"run {run.Id} failed [{kind}]: {error}");
    }

    // Duong dan tai lieu chuan hoa (muc 8.5.1): chu thuong, '\'. Tai lieu chua luu -> null.
    public static string? DocumentKey(string? fullName)
    {
        if (string.IsNullOrWhiteSpace(fullName))
        {
            return null;
        }

        string value = fullName.Trim().Replace('/', '\\').ToLowerInvariant();
        return value.Length == 0 ? null : value;
    }

    private static string Title(string prompt)
    {
        string title = prompt.Replace("\r", " ").Replace("\n", " ").Trim();
        return title.Length <= 80 ? title : title[..80];
    }
}
