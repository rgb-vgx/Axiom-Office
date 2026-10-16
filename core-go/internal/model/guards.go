package model

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Guard cua vong lap agent (hoc LangChain AgentMiddleware: before_model / wrap_model_call / wrap_tool_call /
// after_model, 13/10/2026). Moi co che chan/nhac/cuu cua RunAgent la mot guard rieng, cam vao mot hoac vai diem moc
// duoi day; RunAgent chi con goi model, chay tool va hoi chuoi guard. Them co che moi = them mot guard, khong
// sua vong lap.
//
// Diem moc (guard chi cai interface no can):
//   - replyGuard.onReply: model tra loi KHONG co tool call (cau tra loi cuoi, hoac rong).
//   - callGuard.beforeCall: truoc khi chay mot tool call - tra ket qua thay the = khong chay.
//   - roundGuard.afterTools: sau khi chay het tool call cua mot vong.
//   - recoverGuard.recover: goi model hong (HTTP, tra loi rong, khong doc duoc) - true = da sua, goi lai.
type replyGuard interface {
	onReply(loop *agentLoop, turn *Turn, reply string) *verdict
}

type callGuard interface {
	beforeCall(loop *agentLoop, call ToolCall) *ToolResult
}

type roundGuard interface {
	afterTools(loop *agentLoop, turn *Turn, results []ToolResult) *verdict
}

type recoverGuard interface {
	recover(loop *agentLoop, failure *modelFailure) bool
}

// verdict: quyet dinh cua mot guard. nil = de guard sau / vong lap di tiep.
type verdict struct {
	next    bool          // guard da them loi nhac: sang vong moi
	failure *modelFailure // loi phia model: qua cac recoverGuard (vd chuyen model du phong)
	end     bool          // ket thuc luot voi ok/text/err/kind
	ok      bool
	text    string
	err     string
	kind    string
}

// modelFailure: mot lan goi model khong dung duoc. status = 0 khi loi mang/het gio/tra loi rong.
type modelFailure struct {
	detail  string // thong bao cho nguoi dung ("HTTP 403: ...")
	errText string // than loi tho cua nha cung cap
	status  int
	empty   bool // model tra ve nhung khong co gi dung duoc (rong / khong doc duoc)
}

// agentLoop: trang thai mot luot ma guard doc va sua.
type agentLoop struct {
	client       *Client
	options      AgentOptions
	callbacks    AgentCallbacks
	result       *AgentResult
	systemPrompt string
	turns        []Message
	toolsEnabled bool
	// model: model dang goi - doi sang model du phong khi model chinh hong (fallbackGuard).
	model   string
	addLine func(string)
}

// say: them mot tin nhan user (loi nhac cua Core) vao ngu canh.
func (l *agentLoop) say(content string) {
	l.turns = append(l.turns, map[string]any{"role": "user", "content": content})
}

// guardChain: cac guard cua mot luot, theo thu tu chay.
type guardChain []any

// newGuards: chuoi guard theo AgentOptions. Thu tu co nghia: verify ghi nhan tool sua tai lieu truoc khi final
// ket thuc; don ngu canh truoc khi xet ngan sach; tools-unsupported -> tran ngu canh -> model du phong.
func newGuards(options AgentOptions) guardChain {
	chain := guardChain{&emptyReplyGuard{}}
	if options.Verify {
		chain = append(chain, &verifyGuard{})
	}
	if options.MaxIdenticalCalls > 0 {
		chain = append(chain, &identicalCallGuard{limit: options.MaxIdenticalCalls, counts: map[string]int{}})
	}
	if options.MaxSameTaskCalls > 0 {
		chain = append(chain, &sameTaskGuard{limit: options.MaxSameTaskCalls, counts: map[string]int{}})
	}
	if len(options.OncePerReply) > 0 {
		chain = append(chain, &oncePerReplyGuard{names: options.OncePerReply})
	}
	chain = append(chain, finalGuard{}, abortGuard{})
	if options.ContextClearAt > 0 {
		chain = append(chain, contextClearGuard{})
	}
	chain = append(chain, &heavyRunGuard{})
	if options.MaxIdenticalFailures > 0 {
		chain = append(chain, &repeatedFailureGuard{})
	}
	if options.MaxTokens > 0 {
		chain = append(chain, &budgetGuard{})
	}
	chain = append(chain, toolsUnsupportedGuard{}, &contextOverflowGuard{})
	if len(options.FallbackModels) > 0 {
		chain = append(chain, &fallbackGuard{})
	}
	return chain
}

func (c guardChain) onReply(loop *agentLoop, turn *Turn, reply string) *verdict {
	for _, item := range c {
		if guard, ok := item.(replyGuard); ok {
			if decision := guard.onReply(loop, turn, reply); decision != nil {
				return decision
			}
		}
	}
	return nil
}

func (c guardChain) beforeCall(loop *agentLoop, call ToolCall) *ToolResult {
	for _, item := range c {
		if guard, ok := item.(callGuard); ok {
			if replaced := guard.beforeCall(loop, call); replaced != nil {
				return replaced
			}
		}
	}
	return nil
}

func (c guardChain) afterTools(loop *agentLoop, turn *Turn, results []ToolResult) *verdict {
	for _, item := range c {
		if guard, ok := item.(roundGuard); ok {
			if decision := guard.afterTools(loop, turn, results); decision != nil {
				return decision
			}
		}
	}
	return nil
}

func (c guardChain) recover(loop *agentLoop, failure *modelFailure) bool {
	for _, item := range c {
		if guard, ok := item.(recoverGuard); ok && guard.recover(loop, failure) {
			return true
		}
	}
	return false
}

// emptyReplyGuard: model free doi khi tra loi rong giua chung - nhac mot lan; bi cat vi het max_tokens
// (finish_reason=length) thi nhac viet nho lai mot lan; van rong thi la loi phia model.
type emptyReplyGuard struct{ nudged, truncated bool }

func (g *emptyReplyGuard) onReply(loop *agentLoop, turn *Turn, reply string) *verdict {
	if strings.TrimSpace(reply) != "" {
		return nil
	}
	if !g.nudged {
		g.nudged = true
		loop.addLine("(model tra loi rong - nhac lam tiep)")
		loop.say(EmptyReplyNudge)
		return &verdict{next: true}
	}
	// Bi cat ngang vi het ngan sach token cua CHINH phan hoi nay: nhac viet nho lai roi lam tiep, thay vi nem
	// di ca luot chay da lam duoc nhieu. Chi thu mot lan.
	if turn.FinishReason == "length" && !g.truncated {
		g.truncated = true
		loop.addLine("(phan hoi bi cat vi het ngan sach token - nhac viet nho lai)")
		loop.say(TruncatedReplyNudge)
		return &verdict{next: true}
	}
	// Kem ly do may chu dung lai: finish_reason=length nghia la model dot het ngan sach token vao phan suy luan
	// truoc khi viet duoc gi - thuong gap voi model reasoning.
	detail := "provider returned an empty reply"
	if turn.FinishReason != "" {
		detail += " (finish_reason=" + turn.FinishReason
		if turn.FinishReason == "length" {
			detail += " - model het ngan sach token truoc khi tra loi)"
		} else {
			detail += ")"
		}
	}
	return &verdict{failure: &modelFailure{detail: detail, empty: true}}
}

// verifyGuard (AgentOptions.Verify): luot da sua tai lieu ma model dinh tra loi -> bat doc lai mot lan truoc khi
// ket thuc. Cau tra loi hien tai giu lai lam ban nhap de model doi chieu.
type verifyGuard struct{ mutated, asked bool }

func (g *verifyGuard) afterTools(_ *agentLoop, _ *Turn, results []ToolResult) *verdict {
	for _, result := range results {
		if result.Mutating {
			g.mutated = true
		}
	}
	return nil
}

func (g *verifyGuard) onReply(loop *agentLoop, turn *Turn, _ string) *verdict {
	// Tool da tat (het ngan sach, provider khong ho tro tool): khong doc lai duoc - nhac kiem chung chi lam model viet
	// "khong the kiem tra" (luot that 14/10/2026).
	if !g.mutated || g.asked || !loop.toolsEnabled {
		return nil
	}
	g.asked, loop.result.Verified = true, true
	loop.addLine("(sua tai lieu - yeu cau tu kiem chung truoc khi ket thuc)")
	loop.turns = loop.client.Codec.AppendAssistant(loop.turns, turn)
	loop.say(VerifyWorkNudge)
	return &verdict{next: true}
}

// identicalCallGuard (MaxIdenticalCalls): lan goi thu (n+1) Y HET (cung tool, cung doi so) trong luot bi chan -
// tai lieu khong doi tu lan truoc thi ket qua cung khong doi.
type identicalCallGuard struct {
	limit  int
	counts map[string]int
}

func (g *identicalCallGuard) beforeCall(_ *agentLoop, call ToolCall) *ToolResult {
	signature := call.Name + " " + call.Arguments
	g.counts[signature]++
	if g.counts[signature] <= g.limit {
		return nil
	}
	return &ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: ErrorJSON(fmt.Sprintf(
		"blocked: this exact call (same tool, same arguments) already ran %d times in this request; "+
			"use the earlier result, change the arguments, or answer the user", g.limit))}
}

// sameTaskGuard (MaxSameTaskCalls): chan vong lap "lam cung mot viec, khac tham so" (17-R0 muc 3.3).
//
// identicalCallGuard chi bat "y het tool + y het doi so". Luot that 14/10/2026 (deepseek): 137 lan
// writer.replaceAll voi find khac nhau, 48 vong, 1,48 trieu token - khong lan nao y het nen khong bi
// chan. Guard nay dem theo NHOM VIEC: cung tool + cung lenh office_action + cung dang viec (replace,
// read, format). Vuot han muc thi chan, bao model doi cach (doc outline truoc, gom nhieu doi thanh
// writer.replaceMany, hoac tra loi). Lenh ghi du lieu theo khoi (et.writeRange...) duoc mien: bang
// lon can goi nhieu lan theo khoi, chan la hong viec (xem tools/limits.go).
//
// Luot that 15/10/2026 ("Lam toan bo", deepseek): 59 vong di xoa dau "/" ma tung cho mot - cung roi
// vao nhom replace nay va bi chan som.
type sameTaskGuard struct {
	limit  int
	counts map[string]int
}

// sameTaskExempt: nhom viec duoc mien khoi dem - ghi du lieu theo khoi hoac dung bai tung phan,
// moi lan goi la mot phan viec that (khong phai vong lap "sua tung cho mot").
var sameTaskExempt = map[string]bool{
	"office_action et.writeRange":   true,
	"office_action et.writeRanges":  true,
	"office_action et.fillRange":    true,
	"office_action et.importCsv":    true,
	"office_action et.addSheet":     true,
	"office_action writer.typeText": true,
	// Viet tai lieu / dung bai tung phan (16/10/2026): moi tieu de, moi doan, moi bang, moi slide la mot
	// phan viec that. Luot bao cao chan doan hinh anh 18:19 bi chan 37 lan (writer.heading/appendText/
	// insertStyledText het han muc 25 giua pha viet lai) nen model khong viet tiep duoc - phai mien nhom
	// "dung len" nay. Vong sua tung cho mot (replaceAll...), vong doc lai, va lap Y HET van bi chan boi
	// cac guard khac (identicalCallGuard, heavy-run watchdog).
	"office_action writer.heading":             true,
	"office_action writer.appendText":          true,
	"office_action writer.insertText":          true,
	"office_action writer.insertStyledText":    true,
	"office_action writer.insertTable":         true,
	"office_action writer.insertBlocks format": true,
	"office_action wpp.addSlide":               true,
	"office_action wpp.addSlideFromTemplate":   true,
}

func sameTaskKey(call ToolCall) string {
	if call.Name != "office_action" {
		return call.Name
	}
	action, verb := officeActionTask(call.Arguments)
	if action == "" {
		return "office_action ?"
	}
	key := "office_action " + action
	if verb != "" {
		key += " " + verb
	}
	if sameTaskExempt[key] {
		return ""
	}
	return key
}

// officeActionTask: rut (ten lenh, dong tac) tu arguments JSON cua office_action.
// Dong tac chi phan loai tho (replace, read, format) de bat loop "sua tung cho mot".
func officeActionTask(arguments string) (string, string) {
	action := jsonActionField(arguments)
	if action == "" {
		return "", ""
	}
	verb := ""
	switch {
	case strings.HasSuffix(action, ".replaceAll") || strings.HasSuffix(action, ".replaceMany") ||
		strings.HasSuffix(action, ".replace"):
		verb = "replace"
	case strings.HasSuffix(action, ".readRange") || strings.HasSuffix(action, ".getText") ||
		strings.HasSuffix(action, ".listSlides") || strings.HasSuffix(action, ".outline") ||
		strings.HasSuffix(action, ".checkRange") || strings.HasSuffix(action, ".checkTables") ||
		strings.HasSuffix(action, ".checkLayout") || strings.HasSuffix(action, ".listSheets"):
		verb = "read"
	case strings.HasSuffix(action, ".formatRange") || strings.HasSuffix(action, ".formatTable") ||
		strings.HasSuffix(action, ".restyle") || strings.HasSuffix(action, ".insertBlocks"):
		verb = "format"
	}
	return action, verb
}

// jsonActionField: doc truong "action" trong arguments JSON ma khong giai ma het
// (arguments co the rat lon nen chi quet chuoi).
func jsonActionField(arguments string) string {
	text := strings.TrimSpace(arguments)
	if len(text) < 12 || text[0] != 123 {
		return ""
	}
	idx := strings.Index(text, "\"action\"")
	quote := "\""
	if idx < 0 {
		idx = strings.Index(text, "'action'")
		quote = "'"
		if idx < 0 {
			return ""
		}
	}
	rest := text[idx+8:]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[colon+1:])
	if rest == "" || rest[:1] != quote {
		return ""
	}
	end := strings.Index(rest[1:], quote)
	if end < 0 {
		return ""
	}
	return rest[1 : 1+end]
}

func (g *sameTaskGuard) beforeCall(_ *agentLoop, call ToolCall) *ToolResult {
	key := sameTaskKey(call)
	if key == "" {
		return nil
	}
	g.counts[key]++
	if g.counts[key] <= g.limit {
		return nil
	}
	return &ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: ErrorJSON("blocked: " + key + " already ran too many times in this request with different arguments - stop repeating the same task one item at a time; read the outline first, batch the changes (e.g. writer.replaceMany instead of many writer.replaceAll), or answer the user")}
}

// oncePerReplyGuard (OncePerReply): tool thay CA trang thai (write_todos ghi de ca danh sach) chi chay mot lan moi
// phan hoi - hai lan song song thi khong biet lan nao thang (LangChain TodoListMiddleware).
type oncePerReplyGuard struct {
	names []string
	round int
	seen  map[string]bool
}

func (g *oncePerReplyGuard) beforeCall(loop *agentLoop, call ToolCall) *ToolResult {
	if g.round != loop.result.Rounds {
		g.round, g.seen = loop.result.Rounds, map[string]bool{}
	}
	limited := false
	for _, name := range g.names {
		limited = limited || name == call.Name
	}
	if !limited {
		return nil
	}
	if g.seen[call.Name] {
		return &ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: ErrorJSON(
			call.Name + " may be called only once per reply (each call replaces the whole state); this extra call was ignored")}
	}
	g.seen[call.Name] = true
	return nil
}

// finalGuard: tool da xong CA viec va tu kiem chung -> ket thuc ngay, khong goi model them (tu van ChatGPT
// 13/10/2026: "xong" la trang thai cua Core, khong phai cau nhac trong ket qua tool).
type finalGuard struct{}

func (finalGuard) afterTools(loop *agentLoop, _ *Turn, results []ToolResult) *verdict {
	for _, result := range results {
		if result.Final && result.OK && strings.TrimSpace(result.FinalText) != "" {
			loop.result.Verified = true
			loop.addLine("(" + result.Name + " bao da xong toan bo - ket thuc, khong goi model them)")
			return &verdict{end: true, ok: true, text: result.FinalText}
		}
	}
	return nil
}

// abortGuard: tool bao khong the lam tiep (ung dung Office da dong / sap) -> dung luot voi loi do, khong goi model them.
type abortGuard struct{}

func (abortGuard) afterTools(loop *agentLoop, _ *Turn, results []ToolResult) *verdict {
	for _, result := range results {
		if result.Abort != "" {
			loop.addLine("(" + result.Name + ": " + result.Abort + ")")
			return &verdict{end: true, kind: "office", err: result.Abort}
		}
	}
	return nil
}

// contextClearGuard (ContextClearAt): dau vao vong vua roi vuot nguong -> thay ket qua tool cu bang mot dong tom tat.
type contextClearGuard struct{}

func (contextClearGuard) afterTools(loop *agentLoop, turn *Turn, _ []ToolResult) *verdict {
	if turn.InputTokens <= loop.options.ContextClearAt {
		return nil
	}
	if cleared := clearOldToolResults(loop.turns, loop.options.ContextKeepResults, contextClearMinChars); cleared > 0 {
		loop.addLine(fmt.Sprintf("(ngu canh %d token - don ket qua tool cu, thu hoi %d ky tu)", turn.InputTokens, cleared))
	}
	return nil
}

// repeatedFailureGuard (MaxIdenticalFailures): model lap lai Y HET mot tool call da hong bay nhieu vong lien tiep ->
// dung han (do ngay 02/10/2026: 90 lan goi cung mot lenh cung mot loi, dot 1.000.088 token).
type repeatedFailureGuard struct {
	count int
	last  string
}

func (g *repeatedFailureGuard) afterTools(loop *agentLoop, turn *Turn, results []ToolResult) *verdict {
	if signature := failureSignature(turn.ToolCalls, results); signature == "" {
		g.count, g.last = 0, ""
	} else if signature == g.last {
		g.count++
	} else {
		g.count, g.last = 1, signature
	}
	if g.count < loop.options.MaxIdenticalFailures {
		return nil
	}
	call := turn.ToolCalls[0]
	return &verdict{end: true, kind: "internal", err: fmt.Sprintf("the model repeated the same failing call %d times "+
		"in a row (%s: %s) - stopping instead of repeating", g.count, call.Name, Truncate(results[0].ResultJSON, 200))}
}

// heavyRunGuard (17-R0 muc 3.3): canh bao khi luot vuot nguong vong/token - hoi tiep hay dung.
//
// Luot that 15/10/2026 (deepseek, "Lam toan bo"): 59 vong, 2 trieu token, 20 phut chay im lang.
// Guard hoi nguoi dung bang the Choose (qua AgentCallbacks.AskContinue): tiep tuc hay dung.
// Khong co ai de hoi (AskContinue nil, Interactive=false) thi cu chay tiep nhu cu - khong doi
// hanh vi ai.ask. Moi nguong chi hoi MOT lan de khong spam.
type heavyRunGuard struct {
	roundAsked bool
	tokenAsked bool
}

// heavyRunRoundAt / heavyRunTokenAt: nguong hoi (vong 30, billable 500k) theo 17-R0 muc 4.
const (
	heavyRunRoundAt = 30
	heavyRunTokenAt = 500_000
)

func (g *heavyRunGuard) afterTools(loop *agentLoop, _ *Turn, _ []ToolResult) *verdict {
	ask := loop.callbacks.AskContinue
	if ask == nil {
		return nil
	}
	result := loop.result
	if !g.roundAsked && result.Rounds >= heavyRunRoundAt {
		g.roundAsked = true
		loop.addLine("(luot da chay toi vong 30 - hoi nguoi dung co chay tiep)")
		if !ask(result.Rounds, result.BillableTokens(), "run da chay 30 vong") {
			return &verdict{end: true, kind: "cancelled", err: "cancelled by user at round 30 (heavy run check)"}
		}
		loop.addLine("(nguoi dung cho chay tiep sau vong 30)")
	}
	if !g.tokenAsked && result.BillableTokens() >= heavyRunTokenAt {
		g.tokenAsked = true
		loop.addLine("(luot da dot 500k token billable - hoi nguoi dung co chay tiep)")
		if !ask(result.Rounds, result.BillableTokens(), "run da dot 500k token") {
			return &verdict{end: true, kind: "cancelled", err: "cancelled by user at 500k tokens (heavy run check)"}
		}
		loop.addLine("(nguoi dung cho chay tiep sau 500k token)")
	}
	return nil
}
// budgetGuard (MaxTokens): ngan sach ba muc - 80% nhac tiet kiem, 90% tat tool de model viet cau tra loi cuoi,
// vuot 100% thi dung. Chi dem token phai tra (BillableTokens).
type budgetGuard struct{ economyNudged bool }

func (g *budgetGuard) afterTools(loop *agentLoop, _ *Turn, _ []ToolResult) *verdict {
	result, options := loop.result, loop.options
	billable := result.BillableTokens()
	if billable > options.MaxTokens {
		result.Stopped = true
		return &verdict{end: true, kind: "provider", err: fmt.Sprintf(
			"token budget exceeded (billable %d > %d; raw %d with %d from cache)",
			billable, options.MaxTokens, result.InputTokens+result.OutputTokens, result.CachedTokens)}
	}
	share := float64(billable) / float64(options.MaxTokens)
	if share >= budgetFinalShare && loop.toolsEnabled {
		loop.toolsEnabled = false
		loop.addLine(fmt.Sprintf("(da dung %.0f%% ngan sach token - tat tool, yeu cau tra loi)", share*100))
		loop.say(BudgetFinalNudge)
	} else if share >= budgetEconomyShare && !g.economyNudged {
		g.economyNudged = true
		loop.addLine(fmt.Sprintf("(da dung %.0f%% ngan sach token - nhac tiet kiem)", share*100))
		loop.say(BudgetEconomyNudge)
	}
	return nil
}

// toolsUnsupportedGuard: provider khong ho tro tools -> tat tools va chay nhu chat thuong.
type toolsUnsupportedGuard struct{}

func (toolsUnsupportedGuard) recover(loop *agentLoop, failure *modelFailure) bool {
	if !loop.toolsEnabled || failure.errText == "" || !loop.client.Codec.IsToolUnsupported(failure.errText) {
		return false
	}
	loop.toolsEnabled = false
	loop.addLine("(provider khong ho tro tools - chay che do thuong)")
	return true
}

// contextOverflowGuard: nha cung cap bao vuot cua so ngu canh -> don manh ket qua tool cu (chi giu ket qua gan nhat)
// roi goi lai, thay vi bo ca luot da lam duoc nhieu (LangChain SummarizationMiddleware bat ContextOverflowError).
// Toi da maxOverflowRecoveries lan moi luot; khong con gi de don thi de guard sau (model du phong) xu ly.
type contextOverflowGuard struct{ used int }

const maxOverflowRecoveries = 2

func (g *contextOverflowGuard) recover(loop *agentLoop, failure *modelFailure) bool {
	if g.used >= maxOverflowRecoveries || !IsContextOverflow(failure.status, failure.errText) {
		return false
	}
	cleared := clearOldToolResults(loop.turns, 1, 0)
	if cleared <= 0 {
		return false
	}
	g.used++
	loop.addLine(fmt.Sprintf("(vuot cua so ngu canh cua model - don ket qua tool cu, thu hoi %d ky tu, goi lai)", cleared))
	return true
}

// contextOverflowText: cach cac nha cung cap bao vuot cua so ngu canh (OpenAI context_length_exceeded, Anthropic
// "prompt is too long", DeepSeek/vLLM "maximum context length", gateway "context window"...).
var contextOverflowText = regexp.MustCompile(`(?i)context[_ ]length[_ ]exceeded|maximum context length|context window|` +
	`prompt is too long|input is too long|too many (input )?tokens|exceeds? the (model'?s? )?(max(imum)? )?context|` +
	`reduce the length of the (messages|prompt)|request too large`)

// IsContextOverflow: loi nay la vuot cua so ngu canh (gui it lai thi qua duoc).
func IsContextOverflow(status int, errText string) bool {
	if status == 413 {
		return true
	}
	return (status == 400 || status == 0 || status == 422) && contextOverflowText.MatchString(errText)
}

// fallbackGuard (FallbackModels): model dang dung hong ma thu lai khong qua (het han muc free 403/429, model bi go
// 404, may chu 5xx, mat ket noi, tra loi rong) -> chuyen sang model du phong ke tiep cho phan con lai cua luot
// (LangChain ModelFallbackMiddleware). Khong chuyen voi loi do cau hinh (401 sai khoa, 400 request sai) vi model nao
// cung se hong nhu vay. Luot sau lai bat dau tu model chinh.
type fallbackGuard struct{ next int }

func (g *fallbackGuard) recover(loop *agentLoop, failure *modelFailure) bool {
	models := loop.options.FallbackModels
	if !failure.switchable() {
		return false
	}
	for g.next < len(models) {
		candidate := strings.TrimSpace(models[g.next])
		g.next++
		if candidate == "" || candidate == loop.model {
			continue
		}
		// Nha cung cap khai ro model nay khong goi tool: chuyen sang no thi luot lam viec cung hong.
		if loop.toolsEnabled && loop.client.ModelInfo(context.Background(), candidate).NoTools {
			loop.addLine("(bo qua model du phong " + candidate + " - nha cung cap khai khong ho tro tool)")
			continue
		}
		previous := loop.model
		loop.model = candidate
		loop.result.Model = candidate
		loop.addLine(fmt.Sprintf("(model %s loi: %s - chuyen sang model du phong %s)", previous,
			Truncate(failure.detail, 120), candidate))
		if loop.callbacks.ModelSwitched != nil {
			loop.callbacks.ModelSwitched(previous, candidate, failure.detail)
		}
		return true
	}
	return false
}

// switchable: loi nay co the do RIENG model dang dung (doi model thi co the qua).
func (f *modelFailure) switchable() bool {
	if f.empty {
		return true
	}
	switch {
	case f.status == 0:
		return f.errText != "cancelled"
	case f.status == 402, f.status == 403, f.status == 404, f.status == 408, f.status == 409, f.status == 429:
		return true
	case f.status >= 500:
		return true
	}
	return IsContextOverflow(f.status, f.errText)
}
