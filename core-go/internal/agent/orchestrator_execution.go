package agent

import (
	"context"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/memory"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/skills"
	"axiomoffice/core/internal/store"
	"axiomoffice/core/internal/tools"
)

// execution: moi thu mot luot can, da chuan bi xong truoc khi goi model. Execute chi con ba buoc:
// prepare -> RunAgent -> applyResult.
type execution struct {
	request           *Request
	run               *Run
	client            *model.Client
	actions           *actionLog
	session           *office.Session
	appKind           string
	conversation      *store.ConversationRow
	userSeq           int
	memoryDocumentKey string
	memoryContext     memory.Context
	assembled         AssembledContext
	registry          *tools.Registry
	runContext        *tools.RunContext
	systemPrompt      string
	options           model.AgentOptions
	choose            func(string, string, []tools.Choice) (string, string, error)
}

// prepareExecution lam phan "truoc khi goi model": tim phien Office, khoa vung boi den, tao hoi thoai,
// dung ngu canh + danh sach tool, va doc cau hinh moi luot. Tra nil khi da bao loi va dong luot.
func (o *Orchestrator) prepareExecution(ctx context.Context, run *Run, request *Request) *execution {
	client := o.Models.Current()
	actions := newActionLog()

	session := o.Sessions.Find(request.Port)
	if session == nil {
		o.fail(run, "office session not found for port "+strconv.Itoa(request.Port)+" (is the app still open?)", "office")
		o.finish(run)
		return nil
	}

	health := o.Bridge.Health(ctx, request.Port)
	if health == nil {
		o.fail(run, "the bridge on port "+strconv.Itoa(request.Port)+" is not answering /health", "office")
		o.finish(run)
		return nil
	}
	if healthPid := intOf(health["pid"]); request.Pid != 0 && healthPid != 0 && healthPid != request.Pid {
		o.fail(run, "port "+strconv.Itoa(request.Port)+" is now served by pid "+strconv.Itoa(healthPid)+
			", not "+strconv.Itoa(request.Pid), "office")
		o.finish(run)
		return nil
	}

	appKind := session.App
	if appKind == "" {
		appKind = request.App
	}
	documentPath := session.DocumentPath
	if request.Document != nil && request.Document.FullName != "" {
		documentPath = request.Document.FullName
	}
	documentKey := office.DocumentKey(documentPath)

	// Khoa vung boi den: nguoi dung boi den roi hoi thi AI chi duoc sua trong vung do. Lenh nay doc
	// vung chon SONG trong add-in chu khong tin pane - POST /wps/v1/runs ghi de `document` bang ban
	// chup cua add-in, ma ban chup do khong co `selection` - nen no cung dung cho MCP/script.
	lockError := o.lockSelection(ctx, run, request, appKind)

	// Lenh nhanh (va khung chat mini) lam viec tren doan boi den: khong co doan nao thi dung NGAY, truoc khi tao hoi thoai - de
	// model tu doan "doan nao" thi no se sua ca tai lieu. Hai ma loi rieng de pane noi dung ly do bang tieng
	// Viet: "selection" = nguoi dung chua boi den; "selection-lock" = co boi den nhung add-in khong khoa duoc
	// (ban WPS thieu API do vung) - chay ma khong khoa thi khong con gi giu AI trong doan do.
	if request.RequireSelection && !hasSelection(run, request) {
		what := "this request"
		if request.QuickAction != "" {
			what = "quick action \"" + request.QuickAction + "\""
		}
		if lockError != "" {
			o.fail(run, what+" needs a locked selection, but the add-in cannot lock it: "+lockError, "selection-lock")
		} else {
			o.fail(run, what+" works on the selected text: select it first", "selection")
		}
		o.finish(run)
		return nil
	}

	conversation := o.conversation(request, appKind, session, documentKey)
	if conversation == nil {
		o.fail(run, "cannot create the conversation record", "internal")
		o.finish(run)
		return nil
	}

	run.update(func() { run.ConversationID = conversation.ID })
	o.Runs.Insert(run.ID, &conversation.ID, client.Model)
	mode := "act"
	if request.Plan {
		mode = "plan"
	}
	run.Events.Publish("run.started", map[string]any{
		"conversationId": conversation.ID, "model": client.Model, "app": appKind, "port": request.Port, "mode": mode,
	})

	userSeq := o.Conversations.AppendMessage(conversation.ID, "user", request.Prompt)

	// Ngu canh: cac luot TRUOC luot nay (khong lap lai prompt dang gui).
	before := []store.MessageRow{}
	for _, message := range o.Conversations.Messages(conversation.ID, 60) {
		if message.Seq < userSeq {
			before = append(before, message)
		}
	}
	assembled := o.Assembler.Assemble(conversation, before, 10)

	catalog := o.Bridge.GetCommands(ctx, request.Port, false)
	if catalog == nil {
		o.fail(run, "cannot read GET /commands from the bridge (uninstall/upgrade the add-in?)", "office")
		o.finish(run)
		return nil
	}

	// Skill hop app nay: chi muc vao prompt (tang 1), load_skill/read_skill_file nap khi can (tang 2, 3).
	appSkills := []skills.Definition{}
	if o.Skills != nil {
		appSkills = o.Skills.ForApp(appKind)
	}
	// readOnly: lap ke hoach, hoac lenh nhanh chi tra loi (Tom tat, Dich...) - ca hai chi duoc goi lenh doc.
	readOnly := request.Plan || request.ReadOnly
	officeTool := tools.NewOfficeActionTool(catalog, appKind)
	if readOnly {
		officeTool = tools.NewReadOnlyOfficeActionTool(catalog, appKind)
	}
	list := []tools.Tool{officeTool}
	// Dung lai ca bai theo mau tung lo (docs/architecture/template-design.md muc 6.6): chi trinh chieu, chi khi sua duoc, va
	// chi khi bridge co lenh doc bai nguon (PowerPoint; LibreOffice/WPS chua co).
	if appKind == "wpp" && !readOnly && catalog.Has("wpp.sourceManifest") {
		list = append(list, tools.NewSlidesFromTemplateTool())
	}
	// Trinh bay lai tai lieu Word theo mau (khoi bo tron nen nhat): chi khi sua duoc va bridge co lenh dung khoi.
	// Dang khoa vung boi den thi khong mo (doc_design trinh bay lai ca tai lieu, bridge se tu choi).
	locked := request.Document != nil && request.Document.SelectionLocked
	if appKind == "wps" && !readOnly && !locked && catalog.Has("writer.insertBlocks") {
		list = append(list, tools.NewDocDesignTool())
	}
	// Danh sach viec (write_todos): chi khi lam that - lap ke hoach da tra ve ke hoach bang chu.
	if !readOnly {
		list = append(list, tools.NewTodoTool())
	}
	if len(appSkills) > 0 && o.Skills != nil {
		list = append(list, tools.NewLoadSkillTool(o.Skills, appKind), tools.NewReadSkillFileTool(o.Skills, appKind))
	}

	// Memory dai han (giai doan 3, muc 8.5): tai lieu chua luu (khong co duong dan) -> khong co memory tai lieu.
	memoryDocumentKey := ""
	if strings.HasPrefix(documentKey, "/") || strings.Contains(documentKey, `\`) {
		memoryDocumentKey = documentKey
	}
	memoryContext := memory.EmptyContext()
	if o.Memory != nil && o.Memory.Enabled() {
		memoryContext = o.Memory.Context(ctx, request.Prompt, memoryDocumentKey)
		// Lap ke hoach khong ghi nho (remember la ghi): chi tra cuu.
		if !request.Plan {
			list = append(list, tools.NewRememberTool(o.Memory, memoryDocumentKey))
		}
		list = append(list, tools.NewRecallTool(o.Memory, memoryDocumentKey))
	}
	// Tim kiem web (Tavily): chi khi co khoa trong Thiet lap VA bat TavilyEnabled (toggle tren ribbon);
	// chi doc web nen co ca trong plan mode.
	if cfg := o.Config(); cfg.TavilyEnabled {
		if key := cfg.TavilyApiKey; key != "" {
			list = append(list, tools.NewWebSearchTool(key))
		}
	}
	// Kho nguon web da tai (web_sources): doc lai/trich dan khoi tai lai - chi doc nen co ca plan mode.
	if o.WebSources != nil {
		list = append(list, tools.NewWebSourcesTool(o.WebSources))
	}
	// QA thi giac (muc 8.4.6): chi khi nguoi dung bat VisualQaEnabled (doc lai moi luot qua cau hinh).
	if o.Config().VisualQaEnabled {
		list = append(list, tools.NewVisualTool())
	}

	// MCP (muc 8.7): lan file office + server nguoi dung cau hinh; loi server khong hong run.
	if o.MCP != nil {
		bound := o.MCP.Tools(ctx)
		if readOnly {
			bound = tools.ReadOnlyMCP(bound)
		}
		if len(bound) > 0 {
			list = append(list, tools.WrapMCPTools(bound)...)
		}
	}
	registry := tools.NewRegistry(list)

	var confirm func(string, string, string) (bool, error)
	var choose func(string, string, []tools.Choice) (string, string, error)
	if request.Interactive {
		timeout := time.Duration(o.Config().ConfirmTimeoutSeconds) * time.Second
		confirm = func(action, reason, preview string) (bool, error) {
			return run.Confirmations.Request(ctx, run.Events, action, reason, preview, timeout), nil
		}
		choose = func(action, question string, choices []tools.Choice) (string, string, error) {
			choice, text := run.Confirmations.Choose(ctx, run.Events, action, question, choices, timeout)
			return choice, text, nil
		}
	}

	runContext := &tools.RunContext{
		RunID:          run.ID,
		ConversationID: conversation.ID,
		Office:         session,
		Bridge:         o.Bridge,
		Prompt:         request.Prompt,
		Event: func(eventType string, data map[string]any) {
			run.Events.Publish(eventType, data)
		},
		Confirm: confirm,
		Choose:  choose,
	}
	if request.Document != nil {
		runContext.FocusSlide = request.Document.Slide
	}
	// Luu nguon web tai ve trong luot (WebSourceStore bat + kho mo duoc): tool web ghi qua RunContext.
	if o.WebSources != nil {
		if cfg := o.Config(); cfg.WebSourceStore {
			runContext.WebSources = newWebSourcesSaver(o.WebSources, o.WebSources.Path(), cfg.WebSourceMaxMB)
		}
	}

	systemPrompt := Build(appKind, session, request.Document, appSkills, memoryContext.Lines, assembled.Summary,
		assembled.RecentActionLines)
	if request.Plan {
		systemPrompt += "\n\n" + PlanModeRule
	}
	// Lenh nhanh "Sua bang AI": nguoi dung bam lenh la da quyet - sua ngay, khong hoi lai (quickactions.go).
	if request.QuickAction != "" {
		if request.ReadOnly {
			systemPrompt += "\n\n" + QuickAnswerRule
		} else {
			systemPrompt += "\n\n" + QuickEditRule
		}
	}

	// Tu kiem chung doc lai tu cau hinh moi luot (nhu VisualQaEnabled): doi trong Cai dat co hieu luc ngay.
	options := request.Options
	options.Verify = o.Config().VerifyWorkEnabled && !readOnly
	// Model du phong + cua so ngu canh (GET /models doc mot lan cho moi cau hinh LLM).
	options.FallbackModels = o.Config().LlmFallbackModels
	options.ContextWindow = client.ModelInfo(ctx, client.Model).ContextWindow
	options.OncePerReply = []string{tools.TodoToolName}
	// Lop 3 cua repair.go: ten lenh dung lam ten tool -> office_action (chi khi khop chinh xac lenh duoc phep).
	options.RepairToolCall = func(call model.ToolCall) (model.ToolCall, bool) {
		fixed, ok := repairToolCall(registry, call)
		if ok {
			corelog.Info("run %s: tool call '%s' repaired to office_action %s", run.ID, call.Name, model.Truncate(fixed.Arguments, 120))
		}
		return fixed, ok
	}

	return &execution{
		request:           request,
		run:               run,
		client:            client,
		actions:           actions,
		session:           session,
		appKind:           appKind,
		conversation:      conversation,
		userSeq:           userSeq,
		memoryDocumentKey: memoryDocumentKey,
		memoryContext:     memoryContext,
		assembled:         assembled,
		registry:          registry,
		runContext:        runContext,
		systemPrompt:      systemPrompt,
		options:           options,
		choose:            choose,
	}
}

// buildCallbacks: su kien cua model -> su kien SSE cua pane + audit + lich su hoi thoai.
func (o *Orchestrator) buildCallbacks(ex *execution) model.AgentCallbacks {
	run, conversation, client, actions := ex.run, ex.conversation, ex.client, ex.actions
	liveReasoning, liveStatus := "", ""
	return model.AgentCallbacks{
		Transcript: run.appendTranscript,
		// Suy luan cua model hien cho nguoi dung xem (muc 7.4): chi co khi nguoi dung bat
		// LlmShowReasoning - xem model.Client.ShowReasoning. Pane nao khong biet su kien nay thi bo qua.
		Reasoning: func(round int, text string) {
			run.Events.Publish("run.reasoning", map[string]any{
				"round": round, "text": text, "model": client.Model,
			})
		},
		// Phan hoi dang toi (nguoi dung 14/10/2026: khung chat dung yen 1-2 phut moi phan bai): suy luan hien dan vao khoi
		// suy luan, dong trang thai noi model dang lam gi ("Dang thiet ke phan 1-15: 7 slide...").
		Live: func(round int, update model.LiveUpdate) {
			if update.Reasoning != "" && update.Reasoning != liveReasoning {
				liveReasoning = update.Reasoning
				run.Events.Publish("run.reasoning.live", map[string]any{
					"round": round, "text": tailText(update.Reasoning, liveReasoningChars), "model": client.Model,
				})
			}
			if text := liveProgress(update); text != "" && text != liveStatus {
				liveStatus = text
				run.Events.Publish("run.progress", map[string]any{"round": round, "text": text})
			}
		},
		ModelSwitched: func(from, to, reason string) {
			corelog.Info("run %s: model %s failed (%s) - switching to fallback %s", run.ID, from, reason, to)
			run.Events.Publish("run.model_switched", map[string]any{"from": from, "to": to, "reason": reason})
		},
		AskContinue: func(round, billable int, reason string) bool {
			if ex.choose == nil {
				return true
			}
			q := "Luot nay " + reason + " (vong " + strconv.Itoa(round) + ", " + strconv.Itoa(billable) + " token billable). Cho chay tiep?"
			choice, _, err := ex.choose("agent.heavy_run", q, []tools.Choice{{ID: "continue", Label: "Chay tiep"}, {ID: "stop", Label: "Dung lai"}})
			if err != nil || choice == "" {
				return true
			}
			return choice == "continue"
		},

		ToolStarted: func(call model.ToolCall) {
			run.update(func() { run.ToolCalls++ })
			action, params := readCall(call)
			actions.put(call.ID, action, params)
			run.Events.Publish("tool.started", map[string]any{
				"callId": call.ID, "tool": call.Name, "action": action, "paramsPreview": model.Truncate(params, 200),
			})
		},
		ToolFinished: func(result model.ToolResult) {
			info := actions.get(result.CallID)
			errorText := ""
			if !result.OK {
				errorText = extractError(result.ResultJSON)
			}
			var errorJSON any
			if errorText != "" {
				errorJSON = errorText
			}
			run.Events.Publish("tool.finished", map[string]any{
				"callId": result.CallID, "tool": result.Name, "action": info.action,
				"paramsPreview": model.Truncate(info.params, 200), "ok": result.OK, "error": errorJSON,
				"ms": result.Ms, "resultPreview": model.Truncate(result.ResultJSON, 150),
				// resultBytes: kich thuoc ket qua DA VAO ngu canh model (sau khi cat) - de soi tool nao hay
				// tra qua lon (thong le: ghi kich thuoc tung lan goi tool, canh bao khi p95 vuot nguong).
				"resultBytes": len(result.ResultJSON),
			})
			o.Runs.AddToolCall(run.ID, run.transcriptLen(), result.Name, optional(info.action), optional(info.params),
				result.OK, optional(errorText), result.Ms)
			label := info.action
			if label == "" {
				label = result.Name
			}
			if result.OK {
				o.Conversations.AppendMessage(conversation.ID, "tool_summary", label+" (ok)")
			} else {
				o.Conversations.AppendMessage(conversation.ID, "tool_summary", label+" (loi: "+model.Truncate(errorText, 120)+")")
			}
		},
	}
}

// applyResult ghi ket qua cua luot vao Run/hoi thoai/audit va phat su kien ket thuc tuong ung.
func (o *Orchestrator) applyResult(ctx context.Context, ex *execution, result model.AgentResult) {
	run, conversation, request := ex.run, ex.conversation, ex.request
	run.update(func() {
		run.Rounds = result.Rounds
		run.InputTokens = result.InputTokens
		run.OutputTokens = result.OutputTokens
		run.CachedTokens = result.CachedTokens
		run.Verified = result.Verified
		run.Seconds = result.Seconds
	})

	switch {
	case result.OK:
		run.update(func() { run.Status, run.Reply = StatusCompleted, result.Text })
		o.Conversations.AppendMessage(conversation.ID, "assistant", result.Text)
		run.Events.Publish("run.completed", map[string]any{
			"reply": result.Text, "rounds": result.Rounds, "seconds": round2(result.Seconds),
			"inputTokens": result.InputTokens, "outputTokens": result.OutputTokens,
			"cachedTokens": result.CachedTokens, "billableTokens": result.BillableTokens(),
			"verified": result.Verified, "model": result.Model,
		})
	case result.Cancelled:
		run.update(func() { run.Status = StatusCancelled })
		run.Events.Publish("run.cancelled", map[string]any{"seconds": round2(result.Seconds)})
	case result.TimedOut:
		run.update(func() { run.Status, run.Error, run.ErrorKind = StatusTimedOut, result.Error, "timeout" })
		run.Events.Publish("run.timedout", map[string]any{"seconds": round2(result.Seconds), "error": result.Error})
	case result.Stopped:
		run.update(func() { run.Status, run.Error, run.ErrorKind = StatusStopped, result.Error, "budget" })
		run.Events.Publish("run.stopped", map[string]any{
			"error": result.Error, "rounds": result.Rounds,
			"inputTokens": result.InputTokens, "outputTokens": result.OutputTokens,
			"cachedTokens": result.CachedTokens, "billableTokens": result.BillableTokens(),
		})
	default:
		kind := result.ErrorKind
		if kind == "" {
			kind = "provider"
		}
		errorText := result.Error
		if errorText == "" {
			errorText = "agent failed"
		}
		o.fail(run, errorText, kind)
	}

	if result.ToolsDisabled {
		run.appendTranscript("(provider khong ho tro tools - chay che do thuong)")
	}

	o.Runs.Finish(run.ID, run.Status, run.Rounds, run.InputTokens, run.OutputTokens, o.errorPtr(run), nil)
	if o.Memory != nil {
		o.Memory.TouchHits(ex.memoryContext.IDs)
		if result.OK && !request.Plan {
			o.Memory.QueueExtraction(memory.ExtractionJob{
				RunID: run.ID, ConversationID: conversation.ID, Prompt: request.Prompt,
				Reply: result.Text, DocumentKey: ex.memoryDocumentKey,
			})
		} else {
			o.Runs.SetMemoryStatus(run.ID, "skipped")
		}

		// Tom tat hoi thoai o hang doi nen (muc 8.5.9): khong bat pane cho sau khi run xong.
		target, seq := conversation, ex.userSeq
		o.Memory.Enqueue(func(inner context.Context) error {
			o.maybeSummarize(inner, o.Models.Current(), target, seq)
			return nil
		})
	} else if result.OK {
		o.maybeSummarize(ctx, ex.client, conversation, ex.userSeq)
	}
}
