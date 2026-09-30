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
	// DefaultRequestTimeout: model free cham co luc can hon 60s cho mot luot tra loi dai
	// (chinh bang LlmRequestTimeoutSeconds / AXIOM_LLM_REQUEST_TIMEOUT). Tran ca luot van la 300s.
	DefaultRequestTimeout = 120 * time.Second
	DefaultDeadline       = 300 * time.Second
	MaxRetryAfter         = 10 * time.Second

	EmptyReplyNudge = "Your last reply was empty. Continue the task with the tools, or if it is already done, reply with a short summary."
)

// DefaultRetryDelays: loi tam thoi cua nha cung cap (429, 500/502/503/504, rot mang) thu lai co cho tang dan.
var DefaultRetryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// Client goi mot model (ModelClient.cs).
type Client struct {
	http           *http.Client
	Codec          Codec
	endpoint       string
	apiKey         string
	Model          string
	RequestTimeout time.Duration
	RetryDelays    []time.Duration
}

func NewClient(httpClient *http.Client, provider, endpoint, apiKey, model string) *Client {
	return &Client{
		http:           httpClient,
		Codec:          NewCodec(provider),
		endpoint:       NormalizeEndpoint(endpoint),
		apiKey:         apiKey,
		Model:          model,
		RequestTimeout: DefaultRequestTimeout,
		RetryDelays:    DefaultRetryDelays,
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
	callCtx, cancel := context.WithTimeout(ctx, c.RequestTimeout)
	defer cancel()
	body := c.Codec.BuildRequest(c.Model, systemPrompt,
		[]Message{map[string]any{"role": "user", "content": userPrompt}}, nil, false, maxTokens)
	text, errText, status := c.post(callCtx, ctx, body)
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

func IsTransient(status int, errText string) bool {
	switch status {
	case 429, 500, 502, 503, 504:
		return true
	}
	return status == 0 && strings.HasPrefix(errText, "HttpRequestException")
}

// post gui body, thu lai khi loi tam thoi. userCtx de phan biet nguoi dung huy voi het gio.
func (c *Client) post(ctx, userCtx context.Context, body map[string]any) ([]byte, string, int) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "invalid request: " + err.Error(), 0
	}
	for attempt := 0; ; attempt++ {
		text, errText, status, retryAfter := c.postOnce(ctx, userCtx, payload)
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
	requestCtx, cancel := context.WithTimeout(ctx, c.RequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, BuildURL(c.endpoint, c.Codec.Path()), bytes.NewReader(payload))
	if err != nil {
		return nil, "HttpRequestException: " + err.Error(), 0, 0
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	ApplyAuth(request, c.Codec.Name(), c.apiKey)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, transportError(err, ctx, userCtx, c.RequestTimeout), 0, 0
	}
	defer response.Body.Close()
	text, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, transportError(err, ctx, userCtx, c.RequestTimeout), 0, 0
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
