package tools

import (
	"context"
	"encoding/json"
	"strconv"

	"axiomoffice/core/internal/store"
)

// WebSourcesTool ("web_sources"): cho model doc lai kho nguon web da tai trong cac luot truoc (core.db) -
// khoi tai lai trang. search: tim theo tu khoa (FTS bo dau); get: doc day du mot nguon; list: nguon dung
// gan day. Chi doc du lieu cuc bo nen dung duoc ca trong che do lap ke hoach.
type WebSourcesTool struct{ Sources *store.WebSources }

const (
	WebSourcesToolName = "web_sources"
	// webSourcesReadDefault/Max: gioi han so ky tu tra o action get (tranh dot ngu canh).
	webSourcesReadDefault = 20000
	webSourcesReadMax     = 60000
)

func NewWebSourcesTool(sources *store.WebSources) *WebSourcesTool {
	return &WebSourcesTool{Sources: sources}
}

func (t *WebSourcesTool) Name() string { return WebSourcesToolName }

func (t *WebSourcesTool) Description() string {
	return "Read web pages saved by earlier turns into the local store (no network needed). " +
		"Actions: search (full-text over saved pages: query, limit), get (read one saved page in full: id or url, max_chars), " +
		"list (recently used pages of this conversation, or of the whole store). " +
		"Use it to re-check, quote or compare sources seen before instead of fetching them again; " +
		"it also works when the network is down. Sources fetched by web_search/web_fetch/web_research " +
		"are saved automatically, so prefer search here before repeating a web lookup."
}

func (t *WebSourcesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"action":{"type":"string","enum":["search","get","list"],"description":"what to do"},` +
		`"query":{"type":"string","description":"search: keywords (Vietnamese with or without diacritics)"},` +
		`"id":{"type":"integer","description":"get: id returned by a previous search/list"},` +
		`"url":{"type":"string","description":"get: page url, used when no id"},` +
		`"max_chars":{"type":"integer","description":"get: max chars of content (default 20000)"},` +
		`"limit":{"type":"integer","description":"search/list: max items, default 8"},` +
		`"scope":{"type":"string","enum":["conversation","all"],"description":"list: default conversation"}},` +
		`"required":["action"]}`)
}

func (t *WebSourcesTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	if t.Sources == nil {
		return Result{JSON: ErrorJSON("web source store is not available"), Action: WebSourcesToolName}
	}
	switch stringArg(arguments, "action") {
	case "search":
		return t.search(arguments)
	case "get":
		return t.get(arguments)
	case "list":
		return t.list(arguments, run)
	}
	return Result{JSON: ErrorJSON("unknown action: use search, get or list"), Action: WebSourcesToolName}
}

func (t *WebSourcesTool) search(arguments map[string]any) Result {
	hits := t.Sources.Search(stringArg(arguments, "query"), intArg(arguments, "limit", 8))
	items := []map[string]any{}
	for _, hit := range hits {
		items = append(items, hitJSON(hit))
	}
	return Result{JSON: OKJSON(map[string]any{"sources": items, "count": len(items)}), OK: true, Action: WebSourcesToolName}
}

func (t *WebSourcesTool) get(arguments map[string]any) Result {
	reference := stringArg(arguments, "url")
	if reference == "" {
		if id, ok := arguments["id"].(float64); ok && id > 0 {
			reference = strconv.FormatInt(int64(id), 10)
		}
	}
	row, content := t.Sources.Get(reference)
	if row == nil {
		return Result{JSON: ErrorJSON("source not found in the local store (search it first)"), Action: WebSourcesToolName}
	}
	maxChars := intArg(arguments, "max_chars", webSourcesReadDefault)
	if maxChars < 1000 {
		maxChars = webSourcesReadDefault
	}
	if maxChars > webSourcesReadMax {
		maxChars = webSourcesReadMax
	}
	truncated := false
	if len(content) > maxChars {
		content = cutChars(content, maxChars)
		truncated = true
	}
	payload := hitJSON(store.WebSourceHit{WebSourceRow: *row})
	payload["content"] = content
	payload["truncated"] = truncated
	return Result{JSON: OKJSON(payload), OK: true, Action: WebSourcesToolName}
}

func (t *WebSourcesTool) list(arguments map[string]any, run *RunContext) Result {
	limit := intArg(arguments, "limit", 8)
	scope := stringArg(arguments, "scope")
	hits := []store.WebSourceHit{}
	if scope != "all" && run != nil && run.ConversationID != "" {
		hits = t.Sources.RecentByConversation(run.ConversationID, limit)
	}
	if len(hits) == 0 {
		hits = t.Sources.Recent(limit)
	}
	items := []map[string]any{}
	for _, hit := range hits {
		items = append(items, hitJSON(hit))
	}
	return Result{JSON: OKJSON(map[string]any{"sources": items, "count": len(items)}), OK: true, Action: WebSourcesToolName}
}

func hitJSON(hit store.WebSourceHit) map[string]any {
	item := map[string]any{
		"id": hit.ID, "url": hit.URL, "title": hit.Title, "domain": hit.Domain,
		"fetchedAt": hit.FetchedAt,
	}
	if hit.Lang != "" {
		item["lang"] = hit.Lang
	}
	if hit.ContentChars > 0 {
		item["chars"] = hit.ContentChars
	}
	if hit.Snippet != "" {
		item["snippet"] = hit.Snippet
	}
	return item
}
