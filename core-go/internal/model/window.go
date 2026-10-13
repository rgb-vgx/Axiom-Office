package model

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
)

// ModelInfo: thong tin nha cung cap khai cho mot model trong GET /models (0/false = khong khai).
type ModelInfo struct {
	ContextWindow int
	// NoTools: nha cung cap khai RO model khong goi tool (capabilities.tools = false). Khong khai thi false.
	NoTools bool
}

// modelInfoTimeout: GET /models chi de lay cua so ngu canh - khong duoc lam cham luot chay.
const modelInfoTimeout = 8 * time.Second

// ParseModelInfo: {"data":[{"id":..., "context_length":N, ...}]} -> thong tin theo ten model. Nhan cac ten truong
// pho bien: OpenRouter/gateway `context_length`, `capabilities.contextWindow`, `context_window`, Anthropic
// `max_input_tokens`, vLLM `max_model_len`, `top_provider.context_length`.
func ParseModelInfo(body []byte) map[string]ModelInfo {
	var root struct {
		Data []map[string]any `json:"data"`
	}
	infos := map[string]ModelInfo{}
	if decodeFirst(body, &root) != nil {
		return infos
	}
	for _, item := range root.Data {
		id, _ := item["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		capabilities, _ := item["capabilities"].(map[string]any)
		topProvider, _ := item["top_provider"].(map[string]any)
		info := ModelInfo{ContextWindow: firstPositive(item["context_length"], capabilities["contextWindow"],
			item["context_window"], item["max_input_tokens"], item["max_model_len"], item["max_context_length"],
			topProvider["context_length"])}
		if tools, ok := capabilities["tools"].(bool); ok && !tools {
			info.NoTools = true
		}
		infos[id] = info
	}
	return infos
}

func firstPositive(values ...any) int {
	for _, value := range values {
		if number, ok := value.(float64); ok && number > 0 {
			return int(number)
		}
	}
	return 0
}

// ModelInfo: thong tin cua mot model (doc GET /models mot lan cho moi client, loi thi coi nhu khong khai).
// ContextWindowOverride (cau hinh LlmContextWindow) thang moi so nha cung cap khai.
func (c *Client) ModelInfo(ctx context.Context, name string) ModelInfo {
	c.infoOnce.Do(func() { c.infos = c.fetchModelInfo(ctx) })
	info := c.infos[name]
	if c.ContextWindowOverride > 0 {
		info.ContextWindow = c.ContextWindowOverride
	}
	return info
}

func (c *Client) fetchModelInfo(ctx context.Context) map[string]ModelInfo {
	if c.endpoint == "" || c.http == nil {
		return nil
	}
	requestCtx, cancel := context.WithTimeout(ctx, modelInfoTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, BuildURL(c.endpoint, "/models"), nil)
	if err != nil {
		return nil
	}
	ApplyAuth(request, c.Codec.Name(), c.apiKey)
	response, err := c.http.Do(request)
	if err != nil {
		corelog.Info("model info: GET /models failed: %s", err.Error())
		return nil
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil
	}
	return ParseModelInfo(body)
}
