package model

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/corelog"
)

const (
	CatalogTimeout = 20 * time.Second
	maxModels      = 300
)

// ListModels: GET {endpoint}/models (ModelCatalog.cs). Loi cung dang voi Client de setup.Describe dich nhu nhau.
func ListModels(ctx context.Context, httpClient *http.Client, provider, endpoint, apiKey string, timeout time.Duration) ([]string, string) {
	base := NormalizeEndpoint(endpoint)
	if base == "" {
		return nil, "Endpoint is not configured"
	}
	if timeout <= 0 {
		timeout = CatalogTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, BuildURL(base, "/models"), nil)
	if err != nil {
		return nil, "HttpRequestException: " + err.Error()
	}
	ApplyAuth(request, provider, apiKey)
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, transportError(err, requestCtx, ctx, timeout)
	}
	defer response.Body.Close()
	text, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, transportError(err, requestCtx, ctx, timeout)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body := strings.TrimSpace(string(text))
		if runes := []rune(body); len(runes) > 300 {
			body = string(runes[:300])
		}
		return nil, fmt.Sprintf("HTTP %d: %s", response.StatusCode, body)
	}
	return ParseModels(text), ""
}

// ParseModels: {"data":[{"id":...}]} -> ten model, bo trung, sap xep, toi da 300.
func ParseModels(body []byte) []string {
	var root struct {
		Data []struct {
			ID any `json:"id"`
		} `json:"data"`
	}
	names := []string{}
	if decodeFirst(body, &root) != nil {
		return names
	}
	seen := map[string]bool{}
	for _, item := range root.Data {
		id, _ := item.ID.(string)
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			names = append(names, id)
		}
	}
	sort.Strings(names)
	if len(names) > maxModels {
		names = names[:maxModels]
	}
	return names
}

// Source: client cho tung luot chay, doc lai cau hinh LLM moi lan (doi provider/model/key co hieu luc ngay).
type Source struct {
	http      *http.Client
	load      func() config.Config
	mu        sync.Mutex
	current   *Client
	signature string
}

func NewSource(httpClient *http.Client, load func() config.Config) *Source {
	return &Source{http: httpClient, load: load}
}

func (s *Source) Current() *Client {
	cfg := s.load()
	signature := strings.Join([]string{cfg.LlmProvider, cfg.LlmEndpoint, cfg.LlmModel, cfg.LlmApiKey,
		fmt.Sprint(cfg.LlmRequestTimeoutSeconds), fmt.Sprint(cfg.LlmShowReasoning)}, "\n")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil || signature != s.signature {
		if s.current != nil {
			corelog.Info("LLM config changed: provider=%s model=%s", cfg.LlmProvider, cfg.LlmModel)
		}
		client := NewClient(s.http, cfg.LlmProvider, cfg.LlmEndpoint, cfg.LlmApiKey, cfg.LlmModel)
		if cfg.LlmRequestTimeoutSeconds > 0 {
			client.RequestTimeout = time.Duration(cfg.LlmRequestTimeoutSeconds) * time.Second
		}
		client.ShowReasoning = cfg.LlmShowReasoning
		s.current, s.signature = client, signature
	}
	return s.current
}
