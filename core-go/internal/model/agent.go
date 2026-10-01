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
const DefaultMaxResponseTokens = 16384

type AgentOptions struct {
	MaxRounds         int           // 0 = khong gioi han so vong
	MaxTokens         int           // tran token ca luot (mac dinh 200000)
	Deadline          time.Duration // 0 = khong tran thoi gian (mac dinh); > 0 moi dat
	MaxResponseTokens int           // mac dinh DefaultMaxResponseTokens
}

func DefaultAgentOptions() AgentOptions {
	return AgentOptions{MaxTokens: 200_000, MaxResponseTokens: DefaultMaxResponseTokens}
}

type AgentCallbacks struct {
	Transcript   func(line string)
	RoundStarted func(round int)
	ToolStarted  func(call ToolCall)
	ToolFinished func(result ToolResult)
}

type AgentResult struct {
	OK            bool
	Text          string
	Error         string
	ErrorKind     string // config | provider | internal | cancelled | timeout (rong khi OK)
	Rounds        int
	InputTokens   int
	OutputTokens  int
	Seconds       float64
	Transcript    []string
	Cancelled     bool
	TimedOut      bool
	Stopped       bool // dung vi tran token
	ToolsDisabled bool
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
	nudged := false
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
		responseText, errText, status := c.post(runCtx, ctx, body)
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

		turn, parseError := c.Codec.Parse(responseText)
		if turn == nil {
			return done(false, "", parseError, "provider")
		}
		result.InputTokens += turn.InputTokens
		result.OutputTokens += turn.OutputTokens

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
			results = append(results, toolResult)
			if callbacks.ToolFinished != nil {
				callbacks.ToolFinished(toolResult)
			}
			addLine(call.Name + " " + Truncate(call.Arguments, 150) + " -> " + Truncate(toolResult.ResultJSON, 150))
		}
		turns = c.Codec.AppendToolResults(turns, results)

		if total := result.InputTokens + result.OutputTokens; options.MaxTokens > 0 && total > options.MaxTokens {
			result.Stopped = true
			return done(false, "", fmt.Sprintf("token budget exceeded (%d > %d)", total, options.MaxTokens), "provider")
		}
	}

	// Chi xay ra khi ben goi tu dat gioi han vong: bao chua xong, khong gia lam cau tra loi.
	return done(false, "", fmt.Sprintf("agent stopped after %d rounds without a final answer", options.MaxRounds), "provider")
}

func safeExecute(ctx context.Context, execute Executor, call ToolCall) (result ToolResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = ToolResult{CallID: call.ID, Name: call.Name, ResultJSON: ErrorJSON(fmt.Sprint(recovered)), OK: false}
		}
	}()
	return execute(ctx, call)
}
