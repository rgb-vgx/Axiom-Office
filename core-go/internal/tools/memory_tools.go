package tools

import (
	"context"
	"encoding/json"
	"math"

	"axiomoffice/core/internal/memory"
)

// RememberTool: remember {scope, text, category?} -> ADD qua bo chong trung (muc 8.3, 8.5.6), phat memory.written.
type RememberTool struct {
	memory      *memory.Service
	documentKey string
}

func NewRememberTool(service *memory.Service, documentKey string) *RememberTool {
	return &RememberTool{memory: service, documentKey: documentKey}
}

func (t *RememberTool) Name() string { return "remember" }

func (t *RememberTool) Description() string {
	return "Save one long-term fact so it is available in future conversations: who the user is, their organisation, " +
		"who signs their letters, formatting they always want (scope 'user'), or the progress of this document " +
		"(scope 'document'). One self-contained sentence in the user's language, at most 300 characters. " +
		"Only call it when the user states something worth remembering; never save passwords, IDs, keys or document content."
}

func (t *RememberTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"scope":{"type":"string","enum":["user","document"]},` +
		`"text":{"type":"string","description":"The fact, self-contained, <= 300 characters"},` +
		`"category":{"type":"string","enum":["identity","preference","format","contact","project","progress","other"]}},` +
		`"required":["scope","text"]}`)
}

func (t *RememberTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	scope := stringArg(arguments, "scope")
	if scope == "" {
		scope = memory.ScopeUser
	}
	text := stringArg(arguments, "text")
	if scope == memory.ScopeDocument && t.documentKey == "" {
		scope = memory.ScopeUser
	}
	if scope != memory.ScopeUser && scope != memory.ScopeDocument {
		return Result{JSON: ErrorJSON("scope must be 'user' or 'document'")}
	}

	scopeKey := ""
	if scope == memory.ScopeDocument {
		scopeKey = t.documentKey
	}
	fact := memory.NewMemory{Scope: scope, ScopeKey: scopeKey, Text: text, Category: stringArg(arguments, "category")}
	results := t.memory.Add(ctx, []memory.NewMemory{fact}, memory.ActorAgent, run.RunID)
	if len(results) == 0 {
		return Result{JSON: ErrorJSON("not saved: memory is unavailable")}
	}
	result := results[0]
	if result.Event == "SKIPPED" {
		return Result{JSON: ErrorJSON("not saved: " + result.Reason)}
	}
	if result.Event == "ADD" && result.Item != nil && run.Event != nil {
		run.Event("memory.written", map[string]any{
			"id": result.Item.ID, "event": "ADD", "scope": result.Item.Scope, "text": result.Item.Text,
		})
	}
	var id any
	if result.Item != nil {
		id = result.Item.ID
	}
	return Result{OK: true, JSON: OKJSON(map[string]any{"event": result.Event, "id": id})}
}

// RecallTool: recall {query, scope?, limit?} -> memory lien quan (cung retriever voi ngu canh prompt, muc 8.5.7).
type RecallTool struct {
	memory      *memory.Service
	documentKey string
}

func NewRecallTool(service *memory.Service, documentKey string) *RecallTool {
	return &RecallTool{memory: service, documentKey: documentKey}
}

func (t *RecallTool) Name() string { return "recall" }

func (t *RecallTool) Description() string {
	return "Search long-term memory for facts about the user, their organisation or this document when the facts already " +
		"listed in the system prompt are not enough."
}

func (t *RecallTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"query":{"type":"string"},` +
		`"scope":{"type":"string","enum":["user","document"]},` +
		`"limit":{"type":"integer","minimum":1,"maximum":20}},` +
		`"required":["query"]}`)
}

func (t *RecallTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	query := stringArg(arguments, "query")
	scope := stringArg(arguments, "scope")
	limit := 8
	if value, ok := arguments["limit"].(float64); ok {
		limit = int(value)
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 20 {
		limit = 20
	}

	hits := t.memory.Search(ctx, query, t.documentKey, scope, limit)
	list := make([]any, 0, len(hits))
	for _, hit := range hits {
		list = append(list, map[string]any{
			"id": memory.ShortID(hit.Item.ID), "scope": hit.Item.Scope, "text": hit.Item.Text,
			"score": math.Round(hit.Score*1000) / 1000, "createdAt": hit.Item.CreatedAt,
		})
	}
	return Result{OK: true, JSON: OKJSON(map[string]any{"memories": list})}
}
