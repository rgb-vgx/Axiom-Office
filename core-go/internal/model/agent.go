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
// Nang len 32768 ngay 02/10/2026 sau khi HAI luot chay that cung chet o finish_reason=length: model
// gom mot bang vai nghin dong vao mot lan goi, phan suy luan cong voi phan tra loi vuot 16384. Model
// dang dung (ocg/deepseek-v4.1-flash) khai maxOutput 384000, nen 32768 van con rong rai.
const DefaultMaxResponseTokens = 32768

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
}

func DefaultAgentOptions() AgentOptions {
	return AgentOptions{
		MaxTokens:            200_000,
		MaxResponseTokens:    DefaultMaxResponseTokens,
		Verify:               true,
		MaxIdenticalFailures: DefaultMaxIdenticalFailures,
	}
}

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

// RunAgent: goi model, chay tool model yeu cau, lap toi khi model tra loi xong (ModelClient.RunAgentAsync).
func (c *Client) RunAgent(ctx context.Context, systemPrompt string, prior []ConversationTurn, userPrompt string,
	tools []Tool, execute Executor, options AgentOptions, callbacks AgentCallbacks) AgentResult {
	started := time.Now()
	result := AgentResult{Transcript: []string{}}
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

	turns := make([]Message, 0, len(prior)+1)
	for _, turn := range prior {
		turns = append(turns, map[string]any{"role": turn.Role, "content": turn.Content})
	}
	turns = append(turns, map[string]any{"role": "user", "content": userPrompt})

	toolsEnabled := len(tools) > 0
	nudged, truncated := false, false
	// mutated: luot nay da sua tai lieu chua; verified: da yeu cau tu kiem chung chua. Moi luot chi
	// kiem chung MOT lan - du de bat loi that, khong du de quay vong vo tan.
	mutated, verified := false, false
	// Chan vong lap: chu ky cua vong truoc va so vong lien tiep lap lai y het.
	identicalFailures, lastFailure := 0, ""
	for options.MaxRounds <= 0 || result.Rounds < options.MaxRounds {
		result.ToolsDisabled = !toolsEnabled
		if runCtx.Err() != nil {
			return stopped()
		}
		result.Rounds++
		if callbacks.RoundStarted != nil {
			callbacks.RoundStarted(result.Rounds)
		}

		body := c.Codec.BuildRequest(c.Model, systemPrompt, turns, tools, toolsEnabled, options.MaxResponseTokens)
		responseText, errText, status := c.post(runCtx, ctx, body, c.Stream)
		if responseText == nil {
			if runCtx.Err() != nil {
				return stopped()
			}
			if toolsEnabled && errText != "" && c.Codec.IsToolUnsupported(errText) {
				// Provider khong ho tro tools: tat tools va chay nhu chat thuong.
				toolsEnabled = false
				addLine("(provider khong ho tro tools - chay che do thuong)")
				continue
			}
			detail := Truncate(errText, 300)
			if status > 0 {
				detail = fmt.Sprintf("HTTP %d: %s", status, detail)
			}
			return done(false, "", detail, "provider")
		}

		// Luot agent gui stream: true, nen body thuong la SSE; ParseStream tu roi ve Parse khi may
		// chu bo qua stream va tra JSON mot cuc.
		turn, parseError := c.Codec.ParseStream(responseText)
		if turn == nil {
			return done(false, "", parseError, "provider")
		}
		result.InputTokens += turn.InputTokens
		result.OutputTokens += turn.OutputTokens
		result.CachedTokens += turn.CachedTokens
		if turn.Reasoning != "" && callbacks.Reasoning != nil {
			callbacks.Reasoning(result.Rounds, turn.Reasoning)
		}

		if len(turn.ToolCalls) == 0 {
			reply := StripThoughts(turn.Text)
			if strings.TrimSpace(reply) == "" && !nudged {
				// Model free doi khi tra loi rong giua chung: nhac mot lan de no lam tiep hoac tom tat.
				nudged = true
				addLine("(model tra loi rong - nhac lam tiep)")
				turns = append(turns, map[string]any{"role": "user", "content": EmptyReplyNudge})
				continue
			}
			if strings.TrimSpace(reply) == "" {
				// Bi cat ngang vi het ngan sach token cua CHINH phan hoi nay: nhac viet nho lai roi lam
				// tiep, thay vi nem di ca luot chay da lam duoc nhieu. Chi thu mot lan.
				if turn.FinishReason == "length" && !truncated {
					truncated = true
					addLine("(phan hoi bi cat vi het ngan sach token - nhac viet nho lai)")
					turns = append(turns, map[string]any{"role": "user", "content": TruncatedReplyNudge})
					continue
				}
				// Kem ly do may chu dung lai: finish_reason=length nghia la model dot het ngan sach
				// token vao phan suy luan truoc khi viet duoc gi - thuong gap voi model reasoning.
				detail := "provider returned an empty reply"
				if turn.FinishReason != "" {
					detail += " (finish_reason=" + turn.FinishReason
					if turn.FinishReason == "length" {
						detail += " - model het ngan sach token truoc khi tra loi)"
					} else {
						detail += ")"
					}
				}
				return done(false, "", detail, "provider")
			}
			// Luot nay da sua tai lieu ma chua doc lai lan nao: bat model tu kiem chung truoc khi
			// ket thuc. Day la khac biet do duoc giua cac bo khung chay cung mot prompt - xem
			// AgentOptions.Verify. Cau tra loi hien tai duoc giu lai lam ban nhap de model doi chieu.
			if options.Verify && mutated && !verified {
				verified, result.Verified = true, true
				addLine("(sua tai lieu - yeu cau tu kiem chung truoc khi ket thuc)")
				turns = c.Codec.AppendAssistant(turns, turn)
				turns = append(turns, map[string]any{"role": "user", "content": VerifyWorkNudge})
				continue
			}
			return done(true, reply, "", "")
		}

		turns = c.Codec.AppendAssistant(turns, turn)
		results := make([]ToolResult, 0, len(turn.ToolCalls))
		for _, call := range turn.ToolCalls {
			if runCtx.Err() != nil {
				return stopped()
			}
			if callbacks.ToolStarted != nil {
				callbacks.ToolStarted(call)
			}
			toolStarted := time.Now()
			toolResult := safeExecute(runCtx, execute, call)
			toolResult.Ms = time.Since(toolStarted).Milliseconds()
			if toolResult.Mutating {
				mutated = true
			}
			results = append(results, toolResult)
			if callbacks.ToolFinished != nil {
				callbacks.ToolFinished(toolResult)
			}
			addLine(call.Name + " " + Truncate(call.Arguments, 150) + " -> " + Truncate(toolResult.ResultJSON, 150))
		}
		turns = c.Codec.AppendToolResults(turns, results)

		// Chan vong lap vo ich: model lap lai Y HET mot tool call da hong thi dung han, thay vi de no
		// dot het ngan sach (do ngay 02/10/2026: 90 lan goi cung mot lenh cung mot loi).
		if signature := failureSignature(turn.ToolCalls, results); signature == "" {
			identicalFailures, lastFailure = 0, ""
		} else if signature == lastFailure {
			identicalFailures++
		} else {
			identicalFailures, lastFailure = 1, signature
		}
		if options.MaxIdenticalFailures > 0 && identicalFailures >= options.MaxIdenticalFailures {
			call := turn.ToolCalls[0]
			return done(false, "", fmt.Sprintf("the model repeated the same failing call %d times in a row "+
				"(%s: %s) - stopping instead of repeating", identicalFailures, call.Name,
				Truncate(results[0].ResultJSON, 200)), "internal")
		}

		if billable := result.BillableTokens(); options.MaxTokens > 0 && billable > options.MaxTokens {
			result.Stopped = true
			return done(false, "", fmt.Sprintf("token budget exceeded (billable %d > %d; raw %d with %d from cache)",
				billable, options.MaxTokens, result.InputTokens+result.OutputTokens, result.CachedTokens), "provider")
		}
	}

	// Chi xay ra khi ben goi tu dat gioi han vong: bao chua xong, khong gia lam cau tra loi.
	return done(false, "", fmt.Sprintf("agent stopped after %d rounds without a final answer", options.MaxRounds), "provider")
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
