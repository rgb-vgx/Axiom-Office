package api

import (
	"math"
	"net/http"
	"strings"

	"axiomoffice/core/internal/memory"
)

// API memory (New_arch.md muc 8.5.11). Moi thay doi o day la cua NGUOI DUNG (form "Quan ly ghi nho", API).
func mapMemory(mux *http.ServeMux, deps *Deps) {
	service := deps.Memory

	mux.HandleFunc("GET /v1/memory", func(w http.ResponseWriter, r *http.Request) {
		if !service.Available() {
			Error(w, http.StatusServiceUnavailable, "memory unavailable")
			return
		}
		query := r.URL.Query()
		scope := queryValue(query.Get("scope"))
		scopeKey := queryValue(query.Get("scopeKey"))
		if scopeKey == "" {
			scopeKey = queryValue(query.Get("documentKey"))
		}
		includeDeleted := query.Get("includeDeleted") == "1" || query.Get("includeDeleted") == "true"

		if q := queryValue(query.Get("q")); q != "" {
			found := []any{}
			for _, hit := range service.Search(r.Context(), q, scopeKey, scope, 50) {
				item := memoryJSON(hit.Item)
				item["score"] = math.Round(hit.Score*10000) / 10000
				found = append(found, item)
			}
			OK(w, map[string]any{"memories": found})
			return
		}

		list := []any{}
		for _, item := range service.Store().List(scope, scopeKey, includeDeleted, queryValue(query.Get("runId")), 500) {
			list = append(list, memoryJSON(item))
		}
		OK(w, map[string]any{"memories": list})
	})

	mux.HandleFunc("POST /v1/memory", func(w http.ResponseWriter, r *http.Request) {
		body, _ := readJSON(r)
		scope := textOf(body["scope"])
		if scope == "" {
			scope = memory.ScopeUser
		}
		text := textOf(body["text"])
		if !memory.IsValidScope(scope) || strings.TrimSpace(text) == "" {
			Error(w, http.StatusBadRequest, "'scope' (user|document|skill) and 'text' are required")
			return
		}
		scopeKey := textOf(body["scopeKey"])
		if scopeKey == "" {
			scopeKey = textOf(body["documentKey"])
		}
		if scope != memory.ScopeUser && strings.TrimSpace(scopeKey) == "" {
			Error(w, http.StatusBadRequest, "'scopeKey' is required for scope "+scope)
			return
		}

		fact := memory.NewMemory{Scope: scope, ScopeKey: scopeKey, Text: text, Category: textOf(body["category"]),
			ExpiresAt: textOf(body["expiresAt"])}
		result := service.Add(r.Context(), []memory.NewMemory{fact}, memory.ActorUser, "")
		if len(result) == 0 {
			Error(w, http.StatusServiceUnavailable, "memory unavailable")
			return
		}
		if result[0].Event == "SKIPPED" {
			Error(w, http.StatusBadRequest, "not saved: "+result[0].Reason)
			return
		}
		var item any
		if result[0].Item != nil {
			item = memoryJSON(*result[0].Item)
		}
		OK(w, map[string]any{"event": result[0].Event, "memory": item})
	})

	mux.HandleFunc("PATCH /v1/memory/{id}", func(w http.ResponseWriter, r *http.Request) {
		body, _ := readJSON(r)
		_, hasExpires := body["expiresAt"]
		var pinned *bool
		if value, ok := body["pinned"].(bool); ok {
			pinned = &value
		}
		item, err := service.Store().Update(r.PathValue("id"), textOf(body["text"]), textOf(body["category"]),
			textOf(body["expiresAt"]), hasExpires && body["expiresAt"] == nil, pinned, textOf(body["reason"]))
		switch {
		case err != nil:
			Error(w, http.StatusBadRequest, err.Error())
		case item == nil:
			Error(w, http.StatusNotFound, "unknown memory: "+r.PathValue("id"))
		default:
			OK(w, memoryJSON(*item))
		}
	})

	mux.HandleFunc("DELETE /v1/memory/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !service.Store().Delete(id, queryValue(r.URL.Query().Get("reason"))) {
			Error(w, http.StatusNotFound, "unknown memory: "+id)
			return
		}
		OK(w, map[string]any{"deleted": id})
	})

	mux.HandleFunc("POST /v1/memory/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !service.Store().Restore(id) {
			Error(w, http.StatusNotFound, "not a deleted memory: "+id)
			return
		}
		OK(w, map[string]any{"restored": id})
	})

	mux.HandleFunc("GET /v1/memory/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		list := []any{}
		for _, entry := range service.Store().History(r.PathValue("id")) {
			list = append(list, map[string]any{
				"event": entry.Event, "oldText": entry.OldText, "newText": entry.NewText, "actor": entry.Actor,
				"runId": entry.RunID, "reason": entry.Reason, "createdAt": entry.CreatedAt,
			})
		}
		OK(w, map[string]any{"history": list})
	})

	// Xoa cung toan bo: phai co ?scope=all&confirm=true (UI hoi lai truoc).
	mux.HandleFunc("DELETE /v1/memory", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		scope := queryValue(query.Get("scope"))
		if query.Get("confirm") != "true" || scope == "" || (scope != "all" && !memory.IsValidScope(scope)) {
			Error(w, http.StatusBadRequest, "use ?scope=all|user|document&confirm=true")
			return
		}
		purgeScope := scope
		if scope == "all" {
			purgeScope = ""
		}
		OK(w, map[string]any{"removed": service.Store().Purge(purgeScope)})
	})
}

func memoryJSON(item memory.Item) map[string]any {
	entities := item.Entities
	if entities == nil {
		entities = []string{}
	}
	links := item.Links
	if links == nil {
		links = []string{}
	}
	return map[string]any{
		"id": item.ID, "scope": item.Scope, "scopeKey": item.ScopeKey, "text": item.Text,
		"category": item.Category, "entities": entities, "linked": links, "expiresAt": item.ExpiresAt,
		"source": item.Source, "confidence": item.Confidence, "pinned": item.Pinned, "hits": item.Hits,
		"lastUsedAt": item.LastUsedAt, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
		"deletedAt": item.DeletedAt, "runId": item.CreatedRunID,
	}
}

func queryValue(value string) string { return strings.TrimSpace(value) }

// textOf: truong chuoi trong body JSON (khong co -> "").
func textOf(value any) string {
	text, _ := value.(string)
	return text
}
