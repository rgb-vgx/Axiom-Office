package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/corelog"
	"axiomoffice/core/internal/mcp"
	"axiomoffice/core/internal/memory"
	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/skills"
	"axiomoffice/core/internal/store"
	"axiomoffice/core/internal/tools"
)

// Request: body cua POST /v1/runs (docs/architecture/core.md muc 7.3).
type Request struct {
	Prompt         string
	ConversationID string
	Port           int
	Pid            int
	App            string
	Family         string
	Document       *DocumentContext
	Options        model.AgentOptions
	// Interactive: false (ai.ask cua agent ben ngoai) = khong co ai bam xac nhan -> tu choi lenh can xac nhan.
	Interactive bool
	// Plan: che do lap ke hoach (options.mode = "plan") - chi doc tai lieu, tra ve ke hoach, khong sua gi.
	// Buoc thuc hien la mot luot thuong tiep theo trong CUNG hoi thoai (model thay lai ke hoach trong lich su).
	Plan bool
	// QuickAction: id cua muc "Sua bang AI" (quickactions.go) khi luot chay den tu menu do; "" = nguoi dung tu go.
	// Muc nao cung lam viec tren doan boi den, nen khong boi den gi thi luot chay dung ngay (xem Execute).
	QuickAction string
	// ReadOnly: luot chi duoc DOC tai lieu - cung bo tool cua che do lap ke hoach, nhung tra loi nhu mot luot
	// thuong (khong co the ke hoach, khong PlanModeRule). Dung cho cac muc nhanh chi tra loi (Tom tat, Dich...).
	ReadOnly bool
	// RequireSelection (options.requireSelection): luot nay CHI lam viec tren doan boi den - khung chat mini canh doan
	// boi den gui kem. Khong co doan nao thi dung nhu lenh nhanh, de mot cau "ngan hon nua" khong bi hieu thanh sua ca
	// tai lieu.
	RequireSelection bool
}

const (
	// DefaultMaxSeconds = 0: khong dat tran thoi gian cho mot luot. Luot chay duoc kiem soat bang so
	// vong, so token va nut Dung cua nguoi dung - giong opencode va goclaw, ca hai deu khong co
	// wall-clock cho mot luot. Ben goi van dat duoc (maxSeconds > 0) khi can, vi du ai.ask goi dong bo
	// tu mot agent khac thi khong the cho vo han.
	DefaultMaxSeconds = 0
	MaxMaxSeconds     = 86400

	// DefaultMaxRounds: tran dem duoc thay cho tran dong ho. 100 vong la rong rai cho mot task tai
	// lieu (task "bang diem 5 hoc sinh" ton 12 vong) ma van chan duoc vong lap vo ich.
	DefaultMaxRounds = 100
	MaxMaxRounds     = 1_000

	// DefaultMaxTokens = 0: khong tran token cho mot luot (nguoi dung 13/10/2026: "fix cung ngan sach a, cu de tu do").
	// Luot van duoc chan bang so vong, chan lenh y het, dung khi lap mot loi va nut Dung; ben goi van dat tran duoc
	// (options.maxTokens, toi thieu MinMaxTokens) - vd agent khac goi ai.ask.
	DefaultMaxTokens = 0
	MinMaxTokens     = 1_000
)

// ParseRequest doc body JSON thanh Request (loi -> mo ta tieng cho client).
func ParseRequest(body map[string]any) (*Request, string) {
	if body == nil {
		return nil, "invalid JSON body"
	}
	prompt, _ := body["prompt"].(string)
	// Lenh nhanh: cau lenh lay tu bang cua Core, khong tin chu pane gui kem - ba ban cung mot cau.
	quickID := strings.TrimSpace(textOf(body["quickAction"]))
	quick := QuickAction{}
	if quickID != "" {
		found, ok := FindQuickAction(quickID)
		if !ok {
			return nil, "unknown 'quickAction' \"" + quickID + "\" (see GET /v1/quick-actions)"
		}
		quick, prompt = found, found.Prompt
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, "'prompt' is required"
	}
	off, _ := body["office"].(map[string]any)
	port := intOf(off["port"])
	if port <= 0 {
		return nil, "'office.port' is required (find it with office_sessions / GET /session)"
	}

	maxSeconds := clamp(intOfDefault(tryMap(body["options"])["maxSeconds"], DefaultMaxSeconds), 0, MaxMaxSeconds)
	maxRounds := clamp(intOfDefault(tryMap(body["options"])["maxRounds"], DefaultMaxRounds), 1, MaxMaxRounds)
	maxTokens := max(intOfDefault(tryMap(body["options"])["maxTokens"], DefaultMaxTokens), 0)
	if maxTokens > 0 && maxTokens < MinMaxTokens {
		maxTokens = MinMaxTokens
	}

	var document *DocumentContext
	if raw, ok := body["document"].(map[string]any); ok {
		selection := ""
		locked := false
		if text, ok := raw["selection"].(map[string]any); ok {
			selection, _ = text["text"].(string)
			locked, _ = text["locked"].(bool)
		}
		name, _ := raw["name"].(string)
		fullName, _ := raw["fullName"].(string)
		document = &DocumentContext{Name: name, FullName: fullName, SelectionText: selection, SelectionLocked: locked}
		if view, ok := raw["view"].(map[string]any); ok {
			document.Slide = intOf(view["slide"])
			document.SlideCount = intOf(view["slideCount"])
			document.Sheet = strings.TrimSpace(textOf(view["sheet"]))
			document.Range = strings.TrimSpace(textOf(view["range"]))
			document.HeadingPath = headingPathOf(view["headingPath"])
		}
	}

	interactive := true
	if value, ok := tryMap(body["options"])["interactive"].(bool); ok {
		interactive = value
	}
	requireSelection, _ := tryMap(body["options"])["requireSelection"].(bool)
	mode := strings.TrimSpace(textOf(tryMap(body["options"])["mode"]))
	if mode != "" && mode != "plan" && mode != "act" {
		return nil, "'options.mode' must be \"plan\" or \"act\""
	}

	return &Request{
		Prompt: prompt,
		// Bo khoang trang thua: ban .NET giu nguyen nen " c_1 " se thanh hoi thoai moi (id do Core sinh,
		// khong bao gio co khoang trang, nen cat di la an toan).
		ConversationID: strings.TrimSpace(textOf(body["conversationId"])),
		Port:           port,
		Pid:            intOf(off["pid"]),
		App:            textOf(off["app"]),
		Family:         textOf(off["family"]),
		Document:       document,
		Options:        requestOptions(maxRounds, maxTokens, maxSeconds),
		Interactive:    interactive,
		// Lenh nhanh khong ket hop voi lap ke hoach: muc nao cung la mot viec nho, ro rang tren doan boi den.
		Plan:        mode == "plan" && quick.ID == "",
		QuickAction: quick.ID,
		ReadOnly:    quick.ID != "" && !quick.Edits,
		// Lenh nhanh nao cung lam tren doan boi den, nen luon can doan boi den.
		RequireSelection: quick.ID != "" || requireSelection,
	}, ""
}

// requestOptions: tuy chon cua luot = DefaultAgentOptions + gioi han ben goi gui. Truoc 13/10/2026 toi dung
// AgentOptions tu so 0 nen ba co che chong lam thua (don ket qua tool cu, chan lenh y het, dung khi lap mot loi)
// chi chay trong test, luot that khong co.
func requestOptions(maxRounds, maxTokens, maxSeconds int) model.AgentOptions {
	options := model.DefaultAgentOptions()
	options.MaxRounds, options.MaxTokens = maxRounds, maxTokens
	options.Deadline = time.Duration(maxSeconds) * time.Second
	return options
}

// Orchestrator: chay mot luot agent (docs/architecture/core.md muc 8.1) - dung prompt, goi model, chay tool qua bridge,
// phat su kien cho pane, luu hoi thoai + audit. Khong goi COM truc tiep: moi thao tac tai lieu qua /cmd.
type Orchestrator struct {
	Bridge        *office.BridgeClient
	Sessions      *office.Directory
	Conversations *store.Conversations
	Runs          *store.Runs
	Models        *model.Source
	Skills        *skills.Index
	Memory        *memory.Service
	MCP           *mcp.Manager
	// WebSources: kho nguon web tai ve (nhieu luot). nil = memory/kho khong mo duoc - bo qua tinh nang nay.
	WebSources *store.WebSources
	Config     func() config.Config
	Assembler  *ContextAssembler
}

func (o *Orchestrator) Execute(ctx context.Context, run *Run, request *Request) {
	ex := o.prepareExecution(ctx, run, request)
	if ex == nil {
		return
	}
	result := ex.client.RunAgent(ctx, ex.systemPrompt, ex.assembled.PriorTurns, request.Prompt, ex.registry.ModelTools(),
		func(inner context.Context, call model.ToolCall) model.ToolResult {
			return executeTool(inner, ex.registry, call, ex.runContext)
		}, ex.options, o.buildCallbacks(ex))
	o.applyResult(ctx, ex, result)
	o.finish(run)
}

// lockSelection: hoi add-in WPS khoa vung dang boi den, va ghi vung do vao ngu canh cua luot de prompt
// noi cho model biet. Khong boi den - hoac ban WPS khong do duoc vung (thieu Selection.Start/End,
// doc.Range) - thi `chars` = 0 va luot chay chay nhu thuong: day KHONG phai loi cua luot.
//
// Tra ve loi cua add-in khi no TU CHOI khoa (vd ban WPS thieu API); "" khi khoa duoc, khi khong boi den gi,
// hoac khi app nay khong co khoa vung.
func (o *Orchestrator) lockSelection(ctx context.Context, run *Run, request *Request, appKind string) string {
	if o.Bridge == nil || appKind != "wps" {
		return ""
	}
	// writer.selection la lenh doc: goi truoc hay sau khi khoa deu ra cung vung (khoa khong doi vung chon).
	selection := o.Bridge.Command(ctx, request.Port, "writer.selection", nil)
	locked := o.Bridge.Command(ctx, request.Port, "writer.lockSelection", nil)
	if !locked.OK {
		corelog.Info("run %s: selection not locked: %s", run.ID, locked.Error)
		return locked.Error
	}
	chars := intOf(locked.Result["chars"])
	if chars <= 0 {
		return ""
	}
	run.SelectionLock = true
	if request.Document == nil {
		request.Document = &DocumentContext{}
	}
	request.Document.SelectionLocked = true
	if selection.OK {
		if text, ok := selection.Result["text"].(string); ok {
			request.Document.SelectionText = text
		}
	}
	tables := intOf(locked.Result["tables"])
	corelog.Info("run %s: selection locked (%d chars, %d whole table(s))", run.ID, chars, tables)
	return ""
}

// hasSelection: luot nay co doan boi den de lam viec khong. WPS: add-in da khoa duoc mot vung (doc SONG,
// khong tin pane). Ban khong co khoa vung (Windows, LibreOffice): doan chu ben goi gui kem trong document.
func hasSelection(run *Run, request *Request) bool {
	if run.SelectionLock {
		return true
	}
	return request.Document != nil && strings.TrimSpace(request.Document.SelectionText) != ""
}

// unlockSelection: mo khoa vung boi den. Chay trong finish() nen phai dung context RIENG: context cua
// luot da bi huy o moi duong ket thuc (nguoi dung bam Huy, het gio, loi), dung no thi lenh mo khoa
// khong bao gio toi duoc add-in va khoa cu con treo lai, chan luot sau mot cach vo co.
func (o *Orchestrator) unlockSelection(run *Run) {
	if o.Bridge == nil || !run.SelectionLock {
		return
	}
	run.SelectionLock = false
	unlockCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if result := o.Bridge.Command(unlockCtx, run.Port, "writer.unlockSelection", nil); !result.OK {
		corelog.Error("run %s: cannot unlock the selection: %s", run.ID, result.Error)
	}
}

// finish: moc ket thuc + dong SSE + log (luon chay, ke ca khi loi).
func (o *Orchestrator) finish(run *Run) {
	o.unlockSelection(run)
	now := time.Now().UTC()
	run.update(func() { run.Finished = &now })
	run.Events.Complete()
	message := "run " + run.ID + " " + run.Status + ": " + strconv.Itoa(run.Rounds) + " rounds, " +
		strconv.Itoa(run.ToolCalls) + " tool calls, " + strconv.Itoa(run.InputTokens) + "+" +
		strconv.Itoa(run.OutputTokens) + " tokens, " + strconv.FormatFloat(run.Seconds, 'f', 1, 64) + "s"
	// So token doc tu cache la so DO chat luong tien to: tien to doi moi vong thi cache khong bao gio
	// trung, va moi vong tra tien day du. Ghi ra de con biet duong ma sua.
	if run.CachedTokens > 0 {
		message += ", " + strconv.Itoa(run.CachedTokens) + " cached"
	}
	if run.Verified {
		message += ", self-verified"
	}
	if run.RoundKinds != "" {
		message += ", no-tool rounds: " + run.RoundKinds
	}
	if run.Error != "" {
		message += " - " + run.Error
	}
	corelog.Info("%s", message)
}

func (o *Orchestrator) fail(run *Run, errorText, kind string) {
	run.update(func() { run.Status, run.Error, run.ErrorKind = StatusFailed, errorText, kind })
	run.Events.Publish("run.failed", map[string]any{"error": errorText, "kind": kind})
	corelog.Error("run %s failed [%s]: %s", run.ID, kind, errorText)
}

func (o *Orchestrator) errorPtr(run *Run) *string {
	if run.Error == "" {
		return nil
	}
	errorText := run.Error
	return &errorText
}

func (o *Orchestrator) conversation(request *Request, appKind string, session *office.Session, documentKey string) *store.ConversationRow {
	if request.ConversationID != "" {
		if existing := o.Conversations.Get(request.ConversationID); existing != nil {
			return existing
		}
	}
	name := session.Document
	if request.Document != nil && request.Document.Name != "" {
		name = request.Document.Name
	}
	var key *string
	if saved := conversationKey(documentKey); saved != nil {
		key = saved
	}
	id := o.Conversations.Create(appKind, session.Family, key, optional(name), optional(title(request.Prompt)))
	return o.Conversations.Get(id)
}

// conversationKey: chi DUONG DAN that moi lam khoa hoi thoai; tai lieu chua luu ("Document1")
// tra nil. Moi file moi deu ten "Document1" nen neu lay ten lam khoa thi luot hoi moi se resume
// nham hoi thoai cu cua mot tai lieu khac (loi that 16/10/2026: luot "chan doan hinh anh y te"
// keo hoi thoai "soan dieu le giai bong ban" tu 30/09 vao - model yeu tron hai viec).
func conversationKey(documentKey string) *string {
	if strings.Contains(documentKey, "\\") || strings.HasPrefix(documentKey, "/") {
		key := documentKey
		return &key
	}
	return nil
}

// SummaryEvery: chi tom tat khi hoi thoai vuot qua moc moi (moi 20 tin nhan), khong phai sau MOI luot.
const SummaryEvery = 20

func ShouldSummarize(firstSeqOfRun, lastSeq int) bool {
	if lastSeq <= SummaryEvery {
		return false
	}
	return lastSeq/SummaryEvery > (firstSeqOfRun-1)/SummaryEvery
}

func (o *Orchestrator) maybeSummarize(ctx context.Context, client *model.Client, conversation *store.ConversationRow, userSeq int) {
	history := o.Conversations.Messages(conversation.ID, 60)
	if len(history) == 0 || !ShouldSummarize(userSeq, history[len(history)-1].Seq) {
		return
	}
	lines := make([]string, 0, len(history))
	for _, message := range history {
		lines = append(lines, message.Role+": "+model.Truncate(message.Content, 500))
	}
	summary, errText, ok := client.Chat(ctx, "You summarize office-document conversations for future context.",
		strings.Join(lines, "\n")+"\n\n"+summaryPrompt(len(history)), 2048)
	if !ok {
		corelog.Info("summarize skipped: %s", errText)
		return
	}
	text := model.Truncate(summary, 2000)
	o.Conversations.Rename(conversation.ID, nil, &text)
	corelog.Info("conversation %s summarized (%d messages)", conversation.ID, len(history))
}

func executeTool(ctx context.Context, registry *tools.Registry, call model.ToolCall, run *tools.RunContext) model.ToolResult {
	tool := registry.Find(call.Name)
	if tool == nil {
		return model.ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: tools.ErrorJSON(unknownToolMessage(registry, call.Name))}
	}
	var arguments map[string]any
	if json.Unmarshal([]byte(call.Arguments), &arguments) != nil {
		// Tham so khong phai JSON: tool se bao thieu 'action'.
		arguments = nil
	}
	// Han muc theo tool/lenh trong mot luot (tools.CallLimits): het han muc thi khong chay, bao model doi cach.
	key := call.Name
	if action, _ := arguments["action"].(string); call.Name == "office_action" && action != "" {
		key = action
	}
	if problem := run.CountCall(key); problem != "" {
		return model.ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: tools.ErrorJSON(problem)}
	}
	output := tool.Invoke(ctx, arguments, run)
	return model.ToolResult{
		CallID: call.ID, Name: call.Name, ResultJSON: output.JSON, OK: output.OK,
		ImageDataURL: output.ImageDataURL, Mutating: output.Mutating, Verifies: output.Verifies,
		Final: output.Final, FinalText: output.FinalText, Abort: output.Abort,
	}
}

// readCall: office_action -> {action, params}; tool khac (load_skill...) -> ca doi so la params (audit day du).
func readCall(call model.ToolCall) (string, string) {
	var arguments map[string]any
	if json.Unmarshal([]byte(call.Arguments), &arguments) != nil {
		return "", ""
	}
	action, _ := arguments["action"].(string)
	if action != "" {
		params, _ := arguments["params"]
		return action, model.MarshalRelaxed(params)
	}
	return "", model.MarshalRelaxed(arguments)
}

func extractError(resultJSON string) string {
	var parsed map[string]any
	if json.Unmarshal([]byte(resultJSON), &parsed) == nil {
		if message, ok := parsed["error"].(string); ok {
			return message
		}
	}
	return model.Truncate(resultJSON, 200)
}

// title: tieu de hoi thoai lay tu cau dau tien cua nguoi dung (toi da 80 ky tu).
func title(prompt string) string {
	text := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(prompt, "\r", " "), "\n", " "))
	runes := []rune(text)
	if len(runes) <= 80 {
		return text
	}
	return string(runes[:80])
}

func optional(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

type callInfo struct {
	action string
	params string
}

type actionLog struct {
	mu      sync.Mutex
	entries map[string]callInfo
}

func newActionLog() *actionLog { return &actionLog{entries: map[string]callInfo{}} }

func (l *actionLog) put(callID, action, params string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[callID] = callInfo{action: action, params: params}
}

func (l *actionLog) get(callID string) callInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.entries[callID]
}

func tryMap(value any) map[string]any {
	parsed, _ := value.(map[string]any)
	if parsed == nil {
		return map[string]any{}
	}
	return parsed
}

func headingPathOf(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	path := []string{}
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if runes := []rune(text); len(runes) > 120 {
			text = string(runes[:120])
		}
		path = append(path, text)
		if len(path) >= 6 {
			break
		}
	}
	return path
}

func textOf(value any) string {
	text, _ := value.(string)
	return text
}

func intOf(value any) int {
	number, ok := value.(float64)
	if !ok {
		return 0
	}
	return int(number)
}

func intOfDefault(value any, fallback int) int {
	if _, ok := value.(float64); !ok {
		return fallback
	}
	return intOf(value)
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
