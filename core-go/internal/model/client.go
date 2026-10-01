package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
)

const (
	// DefaultChatTimeout: tran cho mot loi goi NGAN, khong stream (kiem tra cau hinh, tom tat hoi
	// thoai). Luot agent thi khong co tran nay.
	DefaultChatTimeout = 120 * time.Second

	// DefaultIdleTimeout: khong nhan duoc byte nao trong bao lau thi coi nhu ket noi treo va huy -
	// thay cho tran thoi gian tong. Lay theo chunkTimeout cua goclaw (300s); do that voi
	// ocg/deepseek-v4.1-flash: mot prompt ~2k token co luc nghi hon 2 phut truoc khi nha byte dau.
	DefaultIdleTimeout = 300 * time.Second

	MaxRetryAfter = 10 * time.Second

	EmptyReplyNudge = "Your last reply was empty. Continue the task with the tools, or if it is already done, reply with a short summary."

	// TruncatedReplyNudge: cau nhac khi mot phan hoi bi cat ngang vi het ngan sach token cua CHINH no
	// (finish_reason=length). Ngay 02/10/2026 hai luot chay that chet han o day: model gom ca bang vai
	// nghin dong vao mot lan goi, phan hoi bi cat truoc khi viet xong, content rong -> ca luot bi huy.
	// Nhac viet nho lai thi luot chay con cuu duoc, thay vi nem di toan bo cong da lam.
	TruncatedReplyNudge = "Your previous reply was cut off by the response token limit before it finished, " +
		"so nothing usable came back. Continue the task, but keep every reply small: write a few hundred " +
		"rows per call and continue from the next row in the following call, instead of one huge call."

	// VerifyWorkNudge: cau nhac khi luot da sua tai lieu (AgentOptions.Verify). YEU CAU DOC LAI bang
	// lenh that, khong phai tu hoi lai suy nghi - model tu danh gia bang tri nho thi luon thay dung.
	VerifyWorkNudge = "Before you finish: check the work you just did. Read the document back with the " +
		"actions available to you (for example et.readRange or et.checkRange, et.listSheets, writer.getText " +
		"or writer.checkTables, wpp.listSlides or wpp.checkLayout) and compare what is really there with what " +
		"the user asked for: values, ranges, cell types, sheet or slide names, headings and formatting. " +
		"Fix anything missing or wrong with more tool calls, then reply with the short summary."
)

// DefaultRetryDelays: loi tam thoi cua nha cung cap (429, 500/502/503/504, rot mang) thu lai co cho tang dan.
var DefaultRetryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// Client goi mot model (ModelClient.cs).
type Client struct {
	http     *http.Client
	Codec    Codec
	endpoint string
	apiKey   string
	Model    string

	// RequestTimeout: tran cho MOT request; 0 = khong tran (mac dinh cho luot agent). Loi goi ngan
	// nhu Chat tu dat tran rieng.
	RequestTimeout time.Duration
	// IdleTimeout: khong nhan duoc byte nao trong bao lau thi huy request; 0 = khong kiem.
	IdleTimeout time.Duration
	RetryDelays []time.Duration
	// Stream: gui `stream: true` va doc SSE. Tat cho loi goi ngan.
	Stream bool
	// ShowReasoning: bat phan suy luan cua model reasoning de nguoi dung nhin thay no dang nghi gi.
	//
	// Mac dinh TAT: model reasoning dot het `max_tokens` vao phan nghi roi tra ve content rong
	// (finish_reason=length) tren prompt lon - xem CHANGELOG 02/10/2026. Tat thi tra loi nhanh va
	// chac hon nhieu (do duoc: 2,0s so voi 15,6s). Bat len thi phan suy luan hien o pane.
	ShowReasoning bool
}

func NewClient(httpClient *http.Client, provider, endpoint, apiKey, model string) *Client {
	return &Client{
		http:        httpClient,
		Codec:       NewCodec(provider),
		endpoint:    NormalizeEndpoint(endpoint),
		apiKey:      apiKey,
		Model:       model,
		IdleTimeout: DefaultIdleTimeout,
		RetryDelays: DefaultRetryDelays,
		Stream:      true,
	}
}

func NormalizeEndpoint(endpoint string) string {
	value := strings.TrimSpace(endpoint)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		return value
	}
	return "http://" + value
}

func BuildURL(endpoint, suffix string) string {
	base := strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(strings.ToLower(base), strings.ToLower(suffix)) {
		return base
	}
	return base + suffix
}

// ApplyAuth: Anthropic dung x-api-key + anthropic-version, con lai Bearer (dung chung voi ListModels).
func ApplyAuth(request *http.Request, provider, apiKey string) {
	if apiKey == "" {
		return
	}
	if provider == "anthropic" {
		request.Header.Set("x-api-key", apiKey)
		request.Header.Set("anthropic-version", "2023-06-01")
		return
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
}

// Chat goi mot lan, khong tool (tom tat hoi thoai, kiem tra cau hinh). ok = false khi loi HOAC model
// khong tra text (giong ban .NET tra Text = null).
func (c *Client) Chat(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, string, bool) {
	if c.endpoint == "" {
		return "", "Endpoint is not configured", false
	}
	if c.Model == "" {
		return "", "Model is not configured", false
	}
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	callCtx, cancel := context.WithTimeout(ctx, c.ShortCallTimeout())
	defer cancel()
	body := c.Codec.BuildRequest(c.Model, systemPrompt,
		[]Message{map[string]any{"role": "user", "content": userPrompt}}, nil, false, maxTokens)
	// Loi goi ngan: khong stream, va vi body khong phai SSE nen Parse thường dung duoc.
	body["stream"] = false
	text, errText, status := c.post(callCtx, ctx, body, false)
	if text == nil {
		if status > 0 {
			return "", fmt.Sprintf("HTTP %d: %s", status, Truncate(errText, 300)), false
		}
		return "", errText, false
	}
	turn, parseError := c.Codec.Parse(text)
	if turn == nil {
		return "", parseError, false
	}
	if !turn.HasText {
		return "", "", false
	}
	return StripThoughts(turn.Text), "", true
}

// ShortCallTimeout: tran cho mot loi goi NGAN - Chat (kiem tra cau hinh, tom tat) va trich xuat
// memory. Luot agent khong dung ham nay vi no khong co tran thoi gian.
func (c *Client) ShortCallTimeout() time.Duration {
	if c.RequestTimeout > 0 {
		return c.RequestTimeout
	}
	return DefaultChatTimeout
}

func IsTransient(status int, errText string) bool {
	switch status {
	case 429, 500, 502, 503, 504:
		return true
	}
	return status == 0 && strings.HasPrefix(errText, "HttpRequestException")
}

// post gui body, thu lai khi loi tam thoi. userCtx de phan biet nguoi dung huy voi het gio.
// stream = true thi bat SSE va xin kem usage (chi OpenAI hieu stream_options).
func (c *Client) post(ctx, userCtx context.Context, body map[string]any, stream bool) ([]byte, string, int) {
	if stream {
		body["stream"] = true
		if c.Codec.Name() == "openai" {
			// Khong co cai nay thi khung cuoi khong mang usage, mat so token cua luot.
			body["stream_options"] = map[string]any{"include_usage": true}
		}
	}
	// Tat suy luan cho model reasoning: khong co cai nay thi prompt lon bi dot het max_tokens vao
	// phan nghi va content tra ve rong. Chi gui cho codec OpenAI (Anthropic co `thinking` khac dang).
	if c.Codec.Name() == "openai" {
		mode := "disabled"
		if c.ShowReasoning {
			mode = "enabled"
		}
		body["thinking"] = map[string]any{"type": mode}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "invalid request: " + err.Error(), 0
	}
	droppedThinking := false
	for attempt := 0; ; attempt++ {
		text, errText, status, retryAfter := c.postOnce(ctx, userCtx, payload)
		// Nha cung cap khong biet tham so `thinking` (OpenAI that, vai gateway khac) -> bo no roi
		// thu lai ngay, va tat han cho cac lan sau. Tu lanh, nguoi dung khong phai cau hinh gi.
		if text == nil && status == 400 && !droppedThinking {
			droppedThinking = true
			delete(body, "thinking")
			if payload, err = json.Marshal(body); err == nil {
				corelog.Info("model %s: provider tu choi tham so thinking - bo qua va thu lai", c.Model)
				continue
			}
		}
		if text != nil || ctx.Err() != nil || attempt >= len(c.RetryDelays) || !IsTransient(status, errText) {
			return text, errText, status
		}
		delay := c.RetryDelays[attempt]
		if retryAfter > 0 {
			delay = min(retryAfter, MaxRetryAfter)
		}
		reason := errText
		if status > 0 {
			reason = "HTTP " + strconv.Itoa(status)
		}
		corelog.Info("model %s: %s - retry %d/%d in %ss", c.Model, reason, attempt+1, len(c.RetryDelays),
			strconv.FormatFloat(delay.Seconds(), 'f', -1, 64))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, "cancelled", 0
		}
	}
}

func (c *Client) postOnce(ctx, userCtx context.Context, payload []byte) ([]byte, string, int, time.Duration) {
	// Khong dat tran thoi gian tong: ben goi quyet dinh (Chat dat 120s, luot agent khong dat).
	// Ket noi treo bi chan bang watchdog "khong nhan duoc byte nao" ben duoi.
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, BuildURL(c.endpoint, c.Codec.Path()), bytes.NewReader(payload))
	if err != nil {
		return nil, "HttpRequestException: " + err.Error(), 0, 0
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	ApplyAuth(request, c.Codec.Name(), c.apiKey)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, transportError(err, ctx, userCtx, c.IdleTimeout), 0, 0
	}
	defer response.Body.Close()
	reader := newIdleReader(response.Body, c.IdleTimeout, cancel)
	text, err := io.ReadAll(reader)
	if err != nil {
		if reader.Stalled() {
			return nil, fmt.Sprintf("provider sent nothing for %ds", int(c.IdleTimeout.Seconds())), 0, 0
		}
		return nil, transportError(err, ctx, userCtx, c.IdleTimeout), 0, 0
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return text, "", response.StatusCode, 0
	}
	return nil, Truncate(string(text), 300), response.StatusCode, retryAfter(response.Header.Get("Retry-After"))
}

// transportError dat ten loi giong ban .NET: nguoi dung huy -> "cancelled", het gio -> "request timed out after Ns",
// con lai -> "HttpRequestException: ..." (IsTransient va setup.Describe dua vao tien to nay).
func transportError(err error, ctx, userCtx context.Context, timeout time.Duration) string {
	switch {
	case userCtx.Err() != nil:
		return "cancelled"
	case ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		return fmt.Sprintf("request timed out after %ds", int(timeout.Seconds()))
	}
	return "HttpRequestException: " + err.Error()
}

func isTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

func retryAfter(header string) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(header); err == nil {
		return time.Until(date)
	}
	return 0
}
