package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"axiomoffice/core/internal/model"
)

// WebSearchTool: tavily_search - tim tren web qua Tavily (REST API, khong can CLI/MCP). Chi dang ky khi nguoi dung
// nhap khoa Tavily trong Thiet lap (TavilyApiKey). Chi doc web, khong dung toi tai lieu -> dung duoc ca trong
// che do lap ke hoach. Ket qua cat gon (cau tra loi + vai ket qua co link): model free ngu canh nho.
type WebSearchTool struct {
	Key      string
	Endpoint string // mac dinh TavilyEndpoint; test tro sang httptest
	Client   *http.Client
}

const (
	WebSearchToolName = "tavily_search"
	TavilyEndpoint    = "https://api.tavily.com"
	webSearchMax      = 8
	webSnippetChars   = 500
	// webSourceSnippetChars: doan trich luu vao kho nguon (rong hon phan gui model de tra cuu lai duoc).
	webSourceSnippetChars = 2000
)

func NewWebSearchTool(key string) *WebSearchTool {
	return &WebSearchTool{Key: key, Endpoint: TavilyEndpoint, Client: &http.Client{Timeout: 30 * time.Second}}
}

func (t *WebSearchTool) Name() string { return WebSearchToolName }

func (t *WebSearchTool) Description() string {
	return "Search the web (Tavily) for up-to-date facts, figures, definitions or sources that are NOT in the " +
		"document. Returns a short answer plus the top results with title, url and snippet. Use it only when the " +
		"task needs outside information; mention the source (site or url) of facts you take from it. " +
		"One or two well-phrased queries are usually enough."
}

func (t *WebSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"query":{"type":"string","description":"What to search for, in the language that gives the best results"},` +
		`"max_results":{"type":"integer","description":"1-8, default 5"},` +
		`"topic":{"type":"string","enum":["general","news"],"description":"news = recent events"}},` +
		`"required":["query"]}`)
}

func (t *WebSearchTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	query := strings.TrimSpace(textArg(arguments["query"]))
	if query == "" {
		return Result{JSON: ErrorJSON("missing 'query'"), Action: WebSearchToolName}
	}
	maxResults := 5
	if value, ok := arguments["max_results"].(float64); ok && value >= 1 {
		maxResults = int(value)
	}
	if maxResults > webSearchMax {
		maxResults = webSearchMax
	}
	topic := textArg(arguments["topic"])
	if topic != "news" {
		topic = "general"
	}

	body, _ := json.Marshal(map[string]any{
		"query": query, "max_results": maxResults, "topic": topic,
		"search_depth": "basic", "include_answer": true,
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(t.Endpoint, "/")+"/search", bytes.NewReader(body))
	if err != nil {
		return Result{JSON: ErrorJSON("web search: " + err.Error()), Action: WebSearchToolName}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+t.Key)
	response, err := t.Client.Do(request)
	if err != nil {
		return Result{JSON: ErrorJSON("web search unavailable: " + err.Error()), Action: WebSearchToolName}
	}
	defer func() { _ = response.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if response.StatusCode != http.StatusOK {
		return Result{JSON: ErrorJSON(tavilyError(response.StatusCode, raw)), Action: WebSearchToolName}
	}

	var reply struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		return Result{JSON: ErrorJSON("web search: unexpected reply from Tavily"), Action: WebSearchToolName}
	}
	results := []map[string]any{}
	docs := []WebSourceDoc{}
	for _, item := range reply.Results {
		run.noteWebSource(item.URL, item.Title)
		results = append(results, map[string]any{
			"title": item.Title, "url": item.URL, "snippet": model.Truncate(strings.TrimSpace(item.Content), webSnippetChars),
		})
		docs = append(docs, WebSourceDoc{
			URL: item.URL, Title: item.Title, Mode: "search", OK: true,
			Snippet: model.Truncate(strings.TrimSpace(item.Content), webSourceSnippetChars),
		})
	}
	run.SaveWebSources("search", query, docs)
	summary := map[string]any{"query": query, "results": results}
	if strings.TrimSpace(reply.Answer) != "" {
		summary["answer"] = strings.TrimSpace(reply.Answer)
	}
	return Result{JSON: OKJSON(summary), OK: true, Action: WebSearchToolName}
}

// Loi Tavily -> cau ro rang cho model (va cho dong loi tren pane): khoa sai/het han muc la viec nguoi dung sua.
func tavilyError(status int, raw []byte) string {
	detail := ""
	var body struct {
		Detail any `json:"detail"`
	}
	if json.Unmarshal(raw, &body) == nil && body.Detail != nil {
		if encoded, err := json.Marshal(body.Detail); err == nil {
			detail = ": " + model.Truncate(string(encoded), 200)
		}
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "web search: the Tavily API key is invalid - the user must fix it in Setup (Thiết lập)" + detail
	case http.StatusTooManyRequests, 432, 433:
		return "web search: Tavily quota or rate limit reached - try later or answer without searching" + detail
	}
	return "web search failed (HTTP " + http.StatusText(status) + ")" + detail
}

func textArg(value any) string {
	text, _ := value.(string)
	return text
}
