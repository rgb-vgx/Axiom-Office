package model

import (
	"context"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
)

// Feature: ten nang luc cua model de tra cuu bang ModelInfo.Has.
const (
	FeatureTools       = "tools"
	FeatureVision      = "vision"
	FeatureReasoning   = "reasoning"
	FeaturePromptCache = "prompt_cache"
)

// ModelInfo: thong tin nha cung cap khai cho mot model trong GET /models (0/false = khong khai).
type ModelInfo struct {
	ContextWindow int
	// MaxOutputTokens: tran token dau ra nha cung cap khai (max_completion_tokens / max_tokens...). 0 = khong khai.
	MaxOutputTokens int
	// NoTools: nha cung cap khai RO model khong goi tool (capabilities.tools = false). Khong khai thi false.
	NoTools bool
	// SupportsTools: nha cung cap khai model CO goi tool. Khong khai thi false (xem NoTools cho khai RO la khong).
	SupportsTools bool
	// SupportsVision: nha cung cap khai nhan dau vao hinh anh. Chi dat khi co modality image duoc khai.
	SupportsVision bool
	// VisionKnown: nha cung cap co khai RO vision (bat hoac tat) khong. false = khong khai, caller
	// giu hanh vi cu thay vi coi la khong ho tro.
	VisionKnown bool
	// SupportsReasoning: nha cung cap khai model suy luan.
	SupportsReasoning bool
	// SupportsPromptCache: nha cung cap khai prompt caching.
	SupportsPromptCache bool
	// InputCostPerMillion / OutputCostPerMillion: gia moi trieu token (USD). 0 = khong khai.
	InputCostPerMillion  float64
	OutputCostPerMillion float64
}

// Has: model co khai nang luc `feature` khong (false khi khong khai hoac khai RO la khong).
func (m ModelInfo) Has(feature string) bool {
	switch strings.ToLower(strings.TrimSpace(feature)) {
	case FeatureTools, "tool":
		return m.SupportsTools && !m.NoTools
	case FeatureVision, "image":
		return m.SupportsVision
	case FeatureReasoning, "reason":
		return m.SupportsReasoning
	case FeaturePromptCache, "prompt-cache", "cache":
		return m.SupportsPromptCache
	}
	return false
}

// modelInfoTimeout: GET /models chi de lay cua so ngu canh + nang luc model - khong duoc lam cham luot chay.
const modelInfoTimeout = 8 * time.Second

// ParseModelInfo: {"data":[{"id":..., "context_length":N, ...}]} -> thong tin theo ten model. Nhan cac ten truong
// pho bien: OpenRouter/gateway `context_length`, `capabilities.contextWindow`, `context_window`, Anthropic
// `max_input_tokens`, vLLM `max_model_len`, `top_provider.context_length`.
//
// Nang luc doc tu nhieu dang /models: OpenRouter `architecture.input_modalities` (image => vision),
// `supported_parameters` (tools, reasoning, include_reasoning), `top_provider.max_completion_tokens`,
// `pricing.prompt/completion` (gia theo token -> doi sang moi trieu); gateway `capabilities.vision/tools`.
// Chi dat co khi nha cung cap khai RO (khong suy doan): khong co modality image thi SupportsVision = false.
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
		architecture, _ := item["architecture"].(map[string]any)
		pricing, _ := item["pricing"].(map[string]any)
		parameters := item["supported_parameters"]
		info := ModelInfo{
			ContextWindow: firstPositive(item["context_length"], capabilities["contextWindow"],
				item["context_window"], item["max_input_tokens"], item["max_model_len"], item["max_context_length"],
				topProvider["context_length"]),
			MaxOutputTokens: firstPositive(topProvider["max_completion_tokens"], item["max_completion_tokens"],
				item["max_output_tokens"], item["max_tokens"], capabilities["maxOutput"],
				capabilities["max_output_tokens"]),
			InputCostPerMillion:  perMillion(pricing["prompt"], pricing["input"]),
			OutputCostPerMillion: perMillion(pricing["completion"], pricing["output"]),
		}

		// Vision: chi tin modality image duoc khai (OpenRouter) hoac co vision duoc khai RO.
		// VisionKnown tach "khai RO khong ho tro" khoi "khong khai": caller chi an tool khi biet
		// model khong doc duoc anh, khong an chi vi gateway thieu truong.
		if raw, ok := architecture["input_modalities"]; ok && raw != nil {
			info.VisionKnown = true
			info.SupportsVision = imageModality(raw)
		}
		if raw, ok := item["input_modalities"]; ok && raw != nil {
			info.VisionKnown = true
			info.SupportsVision = imageModality(raw)
		}
		for _, key := range []string{"vision", "image_input", "imageInput"} {
			if raw, ok := capabilities[key]; ok {
				if declared, ok := boolField(raw); ok {
					info.VisionKnown = true
					info.SupportsVision = declared
				}
			}
		}

		// Tools: chi dat false (NoTools) khi nha cung cap khai RO false.
		if tools, ok := boolField(capabilities["tools"]); ok {
			if tools {
				info.SupportsTools = true
			} else {
				info.NoTools = true
			}
		}
		if listHas(parameters, "tools", "tool_choice") {
			info.SupportsTools = true
		}

		if listHas(parameters, "reasoning", "include_reasoning", "reasoning_effort") ||
			declaredTrue(capabilities["reasoning"], capabilities["thinking"], capabilities["include_reasoning"]) {
			info.SupportsReasoning = true
		}

		if listHas(parameters, "cache_control", "prompt_cache", "prompt_cache_key") ||
			declaredTrue(capabilities["prompt_cache"], capabilities["promptCache"], capabilities["prompt_caching"],
				capabilities["caching"]) || pricing["input_cache_read"] != nil || pricing["cache_read"] != nil {
			info.SupportsPromptCache = true
		}

		infos[id] = info
	}
	return infos
}

func firstPositive(values ...any) int {
	for _, value := range values {
		if number, ok := floatField(value); ok && number > 0 {
			return int(number)
		}
	}
	return 0
}

// floatField: so trong JSON co the la float64 hoac chuoi (OpenRouter ghi gia dang chuoi).
func floatField(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
		return parsed, err == nil
	}
	return 0, false
}

// perMillion: gia theo token -> gia moi trieu token, lam tron de tranh sai so float (3.0000000000000004).
func perMillion(values ...any) float64 {
	for _, value := range values {
		if number, ok := floatField(value); ok && number > 0 {
			return math.Round(number*1_000_000*1_000_000) / 1_000_000
		}
	}
	return 0
}

// boolField: co the la bool hoac object {"supported":true} / {"enabled":true} (Anthropic).
func boolField(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case map[string]any:
		if declared, ok := typed["supported"].(bool); ok {
			return declared, true
		}
		if declared, ok := typed["enabled"].(bool); ok {
			return declared, true
		}
	}
	return false, false
}

func declaredTrue(values ...any) bool {
	for _, value := range values {
		if declared, ok := boolField(value); ok && declared {
			return true
		}
	}
	return false
}

// imageModality: danh sach modality co "image" (OpenRouter architecture.input_modalities).
func imageModality(value any) bool {
	for _, modality := range textList(value) {
		switch strings.ToLower(strings.TrimSpace(modality)) {
		case "image", "images", "vision":
			return true
		}
	}
	return false
}

func listHas(value any, wanted ...string) bool {
	for _, entry := range textList(value) {
		for _, name := range wanted {
			if strings.EqualFold(strings.TrimSpace(entry), name) {
				return true
			}
		}
	}
	return false
}

func textList(value any) []string {
	switch typed := value.(type) {
	case []any:
		list := make([]string, 0, len(typed))
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				list = append(list, text)
			}
		}
		return list
	case []string:
		return typed
	case string:
		return []string{typed}
	}
	return nil
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
