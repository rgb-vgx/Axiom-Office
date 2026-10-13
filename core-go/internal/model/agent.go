package model

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DefaultMaxResponseTokens: tran token cho MOT phan hoi cua model (truong max_tokens gui len).
//
// Model suy luan (reasoning) dot ngan sach nay vao phan suy nghi TRUOC khi viet content hay tool
// call; het ngan sach thi tra ve finish_reason=length voi content rong. De 4096 thi cac luot lam
// viec that (prompt dai + nhieu tool) hay bi "provider returned an empty reply" - do that ngay
// 01/10/2026 voi ocg/deepseek-v4.1-flash.
//
// Nang 16384 -> 32768 (02/10/2026) roi 32768 -> 65536 cung ngay, moi lan deu vi mot luot chay that chet
// o finish_reason=length. Lan thu hai la de Project Portfolio Management (Test 3 cua benchmark): model
// lap ke hoach 5.000 task x 10 cot cong thuc, va phan SUY LUAN mot minh da an het 32768 token truoc khi
// kip viet lenh nao. Model dang dung (ocg/deepseek-v4.1-flash) khai maxOutput 384000, nen 65536 van con
// thua xa; va vi nha cung cap stream ca phan suy luan, watchdog "khong nhan duoc byte nao" van canh duoc
// mot phan hoi treo that.
const DefaultMaxResponseTokens = 65536

type AgentOptions struct {
	MaxRounds         int           // 0 = khong gioi han so vong
	MaxTokens         int           // tran token ca luot (mac dinh 200000)
	Deadline          time.Duration // 0 = khong tran thoi gian (mac dinh); > 0 moi dat
	MaxResponseTokens int           // mac dinh DefaultMaxResponseTokens
	// MaxIdenticalFailures: dung han khi model lap lai Y HET mot tool call da hong bay nhieu vong lien
	// tiep. Do ngay 02/10/2026: mot luot chay goi et.listSheets 90 lan voi cung mot loi ("no active
	// spreadsheet") va dot het 1.000.088 token vao do - model khong tu biet dung. 0 = khong kiem.
	MaxIdenticalFailures int

	// Verify: bat buoc agent TU KIEM CHUNG truoc khi ket thuc mot luot co sua tai lieu.
	//
	// Do duoc tu ba bo khung chay cung mot prompt (02/10/2026): bo khung tu kiem chung roi sua
	// (Claude Code: 36 lan sua sau khi doc lai) xong viec va khong can ai nhac; bo khung ghi mot lan
	// roi tra loi thi phai co nguoi nhac moi chay tiep. Cac lenh doc lai da co san tu truoc
	// (et.checkRange, writer.checkTables, wpp.checkLayout) nhung khong ai buoc model dung chung.
	Verify bool

	// MaxIdenticalCalls: lan goi thu (n+1) Y HET (cung tool, cung doi so) trong mot luot bi chan, khong chay -
	// ca khi cac lan truoc thanh cong (doc lai cung mot thu, sua cung mot cho). 0 = khong chan.
	MaxIdenticalCalls int
	// ContextClearAt: dau vao cua mot vong vuot so token nay thi thay ket qua tool cu bang mot dong tom tat, chi giu
	// ContextKeepResults ket qua gan nhat (Anthropic context editing / LangChain ClearToolUsesEdit). Chi don khi thu
	// hoi duoc it nhat contextClearMinChars ky tu - don lam hong cache tien to mot lan nen phai dang. 0 = tat.
	ContextClearAt     int
	ContextKeepResults int
	// ContextWindow: cua so ngu canh cua model (token; 0 = khong biet). Biet thi nguong don va max_tokens tinh theo
	// no (FitWindow) - model cua so nho khong tran truoc khi don.
	ContextWindow int
	// FallbackModels: model du phong cung nha cung cap, thu lan luot khi model dang dung hong ma thu lai khong qua
	// (fallbackGuard). Rong = khong co.
	FallbackModels []string
}

// FitWindow: chinh nguong theo cua so ngu canh cua model (LangChain trigger ("fraction", x)). Nguong don giu
// ContextClearAt nhung khong qua nua cua so (cua so 32k -> don o 16k). KHONG noi nguong len voi model cua so lon
// (deepseek-v4 1M): nguong 40k dat vi ngan sach token cua luot, khong vi cua so - noi len la dot ngan sach nhanh hon.
// max_tokens khong qua 1/4 cua so khi cua so nho hon hai lan max_tokens - mot so may chu tu choi request co dau vao
// + max_tokens vuot cua so.
func (o AgentOptions) FitWindow() AgentOptions {
	if o.ContextWindow <= 0 {
		return o
	}
	if o.ContextClearAt > 0 {
		o.ContextClearAt = min(o.ContextClearAt, o.ContextWindow/2)
	}
	if o.MaxResponseTokens > 0 && o.ContextWindow < 2*o.MaxResponseTokens {
		o.MaxResponseTokens = o.ContextWindow / 4
	}
	return o
}

func DefaultAgentOptions() AgentOptions {
	return AgentOptions{
		MaxTokens:            200_000,
		MaxResponseTokens:    DefaultMaxResponseTokens,
		Verify:               true,
		MaxIdenticalFailures: DefaultMaxIdenticalFailures,
		MaxIdenticalCalls:    DefaultMaxIdenticalCalls,
		ContextClearAt:       DefaultContextClearAt,
		ContextKeepResults:   DefaultContextKeepResults,
	}
}

const (
	DefaultMaxIdenticalCalls  = 4
	DefaultContextClearAt     = 40_000
	DefaultContextKeepResults = 3
	// contextClearMinChars: ~10k token - don it hon thi khong bu duoc cache bi hong.
	contextClearMinChars = 40_000
	// budgetEconomyShare / budgetFinalShare: phan ngan sach (MaxTokens) bat dau tiet kiem / tat tool.
	budgetEconomyShare = 0.8
	budgetFinalShare   = 0.9
)

// DefaultMaxIdenticalFailures: so vong lap lai y het mot tool call hong truoc khi dung luot chay.
const DefaultMaxIdenticalFailures = 3

type AgentCallbacks struct {
	Transcript   func(line string)
	RoundStarted func(round int)
	ToolStarted  func(call ToolCall)
	ToolFinished func(result ToolResult)
	// Reasoning: phan suy nghi cua model cho mot vong (rong khi model khong suy luan hoac da tat).
	// Goi mot lan moi vong co suy luan, voi TOAN BO phan suy luan cua vong do.
	Reasoning func(round int, text string)
	// ModelSwitched: luot chuyen sang model du phong (from -> to) vi model dang dung hong (reason).
	ModelSwitched func(from, to, reason string)
}

type AgentResult struct {
	OK            bool
	Text          string
	Error         string
	ErrorKind     string // config | provider | internal | cancelled | timeout (rong khi OK)
	Rounds        int
	InputTokens   int
	OutputTokens  int
	CachedTokens  int // phan InputTokens doc tu cache cua nha cung cap (0 = khong cache/khong bao)
	Seconds       float64
	Transcript    []string
	Cancelled     bool
	TimedOut      bool
	Stopped       bool // dung vi tran token
	ToolsDisabled bool
	// Verified: luot nay co sua tai lieu nen da duoc yeu cau tu kiem chung truoc khi tra loi.
	Verified bool
	// Model: model tra loi cuoi cung (khac Client.Model khi da chuyen sang model du phong).
	Model string
}

// BillableTokens: so token phai tra that, tuc la phan nha cung cap THAT SU phai tinh.
//
// Phan doc tu cache gan nhu mien phi (DeepSeek, OpenAI va Anthropic deu tinh khoang 1/10 gia thuong),
// nen dem no vao ngan sach thi ngan sach bi an mat phan lon: do ngay 02/10/2026, mot luot chay dung
// 1.005.301 token trong do 806.400 doc tu cache - 97% la trung, ma van bi dung vi "het ngan sach".
func (r AgentResult) BillableTokens() int {
	// Nha cung cap bao cache nhieu hon ca so token vao la vo ly: khi do BO QUA han con so cache thay vi
	// lay no lam input mien phi - huong sai nay chi lam ngan sach chat hon, khong the bi lay bot.
	cached := r.CachedTokens
	if cached < 0 || cached > r.InputTokens {
		cached = 0
	}
	return r.InputTokens - cached + r.OutputTokens
}

// Executor chay mot tool. Loi (panic) cua tool khong lam hong luot chay: tra ve model de no tu xu ly.
type Executor func(ctx context.Context, call ToolCall) ToolResult

// RunAgent: goi model, chay tool model yeu cau, lap toi khi model tra loi xong (ModelClient.RunAgentAsync). Cac co
// che chan/nhac/cuu (tu kiem chung, ngan sach, don ngu canh, model du phong...) nam trong guards.go.
func (c *Client) RunAgent(ctx context.Context, systemPrompt string, prior []ConversationTurn, userPrompt string,
	tools []Tool, execute Executor, options AgentOptions, callbacks AgentCallbacks) AgentResult {
	started := time.Now()
	result := AgentResult{Transcript: []string{}, Model: c.Model}
	addLine := func(line string) {
		result.Transcript = append(result.Transcript, line)
		if callbacks.Transcript != nil {
			callbacks.Transcript(line)
		}
	}
	done := func(ok bool, text, errText, kind string) AgentResult {
		result.OK, result.Text, result.Error, result.ErrorKind = ok, text, errText, kind
		result.Seconds = time.Since(started).Seconds()
		if ctx.Err() != nil {
			result.Cancelled = true
		}
		return result
	}

	if c.endpoint == "" {
		return done(false, "", "Endpoint is not configured", "config")
	}
	if c.Model == "" {
		return done(false, "", "Model is not configured", "config")
	}

	// Khong dat tran thoi gian mac dinh: mot luot duoc kiem soat bang so vong, ngan sach token va
	// nut Dung cua nguoi dung - giong opencode va goclaw (ca hai deu khong co wall-clock cho luot).
	// Deadline > 0 van duoc ton trong khi ben goi chu dong dat (vi du ai.ask goi dong bo).
	var cancel context.CancelFunc = func() {}
	runCtx := ctx
	if options.Deadline > 0 {
		runCtx, cancel = context.WithTimeout(ctx, options.Deadline)
	}
	defer cancel()
	stopped := func() AgentResult {
		if ctx.Err() != nil {
			result.Cancelled = true
			return done(false, "", "cancelled by user", "cancelled")
		}
		result.TimedOut = true
		return done(false, "", fmt.Sprintf("agent timed out after %.0fs", options.Deadline.Seconds()), "timeout")
	}

	options = options.FitWindow()
	loop := &agentLoop{client: c, options: options, callbacks: callbacks, result: &result, systemPrompt: systemPrompt,
		toolsEnabled: len(tools) > 0, model: c.Model, addLine: addLine}
	loop.turns = make([]Message, 0, len(prior)+1)
	for _, turn := range prior {
		loop.turns = append(loop.turns, map[string]any{"role": turn.Role, "content": turn.Content})
	}
	loop.say(userPrompt)
	guards := newGuards(options)
	finish := func(decision *verdict) AgentResult {
		return done(decision.ok, decision.text, decision.err, decision.kind)
	}

	for options.MaxRounds <= 0 || result.Rounds < options.MaxRounds {
		result.ToolsDisabled = !loop.toolsEnabled
		if runCtx.Err() != nil {
			return stopped()
		}
		result.Rounds++
		if callbacks.RoundStarted != nil {
			callbacks.RoundStarted(result.Rounds)
		}

		turn, failure := loop.ask(runCtx, ctx, tools)
		if failure == nil {
			result.InputTokens += turn.InputTokens
			result.OutputTokens += turn.OutputTokens
			result.CachedTokens += turn.CachedTokens
			if turn.Reasoning != "" && callbacks.Reasoning != nil {
				callbacks.Reasoning(result.Rounds, turn.Reasoning)
			}
			if len(turn.ToolCalls) == 0 {
				reply := StripThoughts(turn.Text)
				decision := guards.onReply(loop, turn, reply)
				if decision == nil {
					return done(true, reply, "", "")
				}
				if decision.next {
					continue
				}
				if decision.failure == nil {
					return finish(decision)
				}
				failure = decision.failure
			}
		}
		if failure != nil {
			if runCtx.Err() != nil {
				return stopped()
			}
			if guards.recover(loop, failure) {
				continue
			}
			return done(false, "", failure.detail, "provider")
		}

		loop.turns = c.Codec.AppendAssistant(loop.turns, turn)
		results := make([]ToolResult, 0, len(turn.ToolCalls))
		for _, call := range turn.ToolCalls {
			if runCtx.Err() != nil {
				return stopped()
			}
			if callbacks.ToolStarted != nil {
				callbacks.ToolStarted(call)
			}
			toolStarted := time.Now()
			var toolResult ToolResult
			if replaced := guards.beforeCall(loop, call); replaced != nil {
				toolResult = *replaced
			} else {
				toolResult = safeExecute(runCtx, execute, call)
			}
			toolResult.Ms = time.Since(toolStarted).Milliseconds()
			results = append(results, toolResult)
			if callbacks.ToolFinished != nil {
				callbacks.ToolFinished(toolResult)
			}
			addLine(call.Name + " " + Truncate(call.Arguments, 150) + " -> " + Truncate(toolResult.ResultJSON, 150))
		}
		loop.turns = c.Codec.AppendToolResults(loop.turns, results)
		if decision := guards.afterTools(loop, turn, results); decision != nil && decision.end {
			return finish(decision)
		}
	}

	// Chi xay ra khi ben goi tu dat gioi han vong: bao chua xong, khong gia lam cau tra loi.
	return done(false, "", fmt.Sprintf("agent stopped after %d rounds without a final answer", options.MaxRounds), "provider")
}

// ask: goi model dang dung (loop.model) mot lan. Loi -> modelFailure de chuoi guard quyet dinh (goi lai sau khi don
// ngu canh, chuyen model du phong, hay dung luot).
func (l *agentLoop) ask(runCtx, userCtx context.Context, tools []Tool) (*Turn, *modelFailure) {
	c := l.client
	body := c.Codec.BuildRequest(l.model, l.systemPrompt, l.turns, tools, l.toolsEnabled, l.options.MaxResponseTokens)
	responseText, errText, status := c.post(runCtx, userCtx, body, c.Stream)
	if responseText == nil {
		detail := Truncate(errText, 300)
		if status > 0 {
			detail = fmt.Sprintf("HTTP %d: %s", status, detail)
		}
		return nil, &modelFailure{detail: detail, errText: errText, status: status}
	}
	// Luot agent gui stream: true, nen body thuong la SSE; ParseStream tu roi ve Parse khi may
	// chu bo qua stream va tra JSON mot cuc.
	turn, parseError := c.Codec.ParseStream(responseText)
	if turn == nil {
		return nil, &modelFailure{detail: parseError, empty: true}
	}
	return turn, nil
}

// failureSignature: chu ky cua mot vong CHI toan loi - rong khi co it nhat mot tool chay duoc.
// Gop ten tool + doi so + ket qua, nen "cung mot loi" phai la cung mot loi that, khong phai trung
// nhau o chuoi thong bao chung.
func failureSignature(calls []ToolCall, results []ToolResult) string {
	if len(results) == 0 {
		return ""
	}
	parts := make([]string, 0, len(results))
	for index, result := range results {
		if result.OK {
			return ""
		}
		arguments := ""
		if index < len(calls) {
			arguments = calls[index].Arguments
		}
		parts = append(parts, result.Name+" "+arguments+" "+result.ResultJSON)
	}
	return strings.Join(parts, "\n")
}

func safeExecute(ctx context.Context, execute Executor, call ToolCall) (result ToolResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: ErrorJSON(fmt.Sprint(recovered)), OK: false}
		}
	}()
	return execute(ctx, call)
}

// clearedToolResult: noi dung thay cho ket qua tool cu da don.
const clearedToolResult = `{"cleared":true,"note":"older tool result removed to keep the context small; call the tool again if you still need it"}`

// clearOldToolResults: thay noi dung moi ket qua tool tru keep cai gan nhat bang clearedToolResult, NEU tong so ky
// tu thu hoi >= minChars (khong thi de nguyen - don it khong bu duoc cache bi hong). Hieu ca hai dang: OpenAI
// ({"role":"tool","content":"..."}) va Anthropic (tin user co khoi tool_result). Tra so ky tu da thu hoi.
func clearOldToolResults(turns []Message, keep, minChars int) int {
	type slot struct {
		holder map[string]any
		size   int
	}
	slots := []slot{}
	for _, message := range turns {
		turn, ok := message.(map[string]any)
		if !ok {
			continue
		}
		if turn["role"] == "tool" {
			if text, ok := turn["content"].(string); ok && text != clearedToolResult {
				slots = append(slots, slot{turn, len(text)})
			}
			continue
		}
		blocks, ok := turn["content"].([]any)
		if !ok {
			continue
		}
		for _, block := range blocks {
			entry, ok := block.(map[string]any)
			if !ok || entry["type"] != "tool_result" {
				continue
			}
			if text, ok := entry["content"].(string); ok {
				if text != clearedToolResult {
					slots = append(slots, slot{entry, len(text)})
				}
				continue
			}
			slots = append(slots, slot{entry, len(fmt.Sprint(entry["content"]))})
		}
	}
	if len(slots) <= keep {
		return 0
	}
	old := slots[:len(slots)-keep]
	reclaim := 0
	for _, item := range old {
		reclaim += item.size - len(clearedToolResult)
	}
	if reclaim < minChars {
		return 0
	}
	for _, item := range old {
		item.holder["content"] = clearedToolResult
	}
	return reclaim
}
