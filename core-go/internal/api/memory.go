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
			Error(w, http.StatusBadRequest, "'scope' (user|document|skill|org-style) and 'text' are required")
			return
		}
		if denyOrgStyle(w, deps, scope) {
			return
		}
		scopeKey := textOf(body["scopeKey"])
		if scopeKey == "" {
			scopeKey = textOf(body["documentKey"])
		}
		if scope != memory.ScopeUser && scope != memory.ScopeOrgStyle && strings.TrimSpace(scopeKey) == "" {
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
		if existing := service.Store().Get(r.PathValue("id")); existing != nil && denyOrgStyle(w, deps, existing.Scope) {
			return
		}
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
		if existing := service.Store().Get(id); existing != nil && denyOrgStyle(w, deps, existing.Scope) {
			return
		}
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

	// Xoa cung: ?scope=all|user|document|skill|org-style&confirm=true, hoac xoa mem theo tai lieu
	// ?documentKey=…&confirm=true (M4-02). org-style chi quan tri vien (M3-04).
	mux.HandleFunc("DELETE /v1/memory", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("confirm") != "true" {
			Error(w, http.StatusBadRequest, "confirm=true is required")
			return
		}
		if documentKey := queryValue(query.Get("documentKey")); documentKey != "" {
			OK(w, map[string]any{"removed": service.Store().SoftDeleteByScopeKey(documentKey, "forget document")})
			return
		}
		scope := queryValue(query.Get("scope"))
		if scope == "" || (scope != "all" && !memory.IsValidScope(scope)) {
			Error(w, http.StatusBadRequest, "use ?scope=all|user|document|skill|org-style&confirm=true or ?documentKey=…&confirm=true")
			return
		}
		if denyOrgStyle(w, deps, scope) {
			return
		}
		if scope == "all" {
			if deps.Live().OrgStyleAdmin {
				OK(w, map[string]any{"removed": service.Store().Purge("")})
				return
			}
			// Nguoi thuong: "all" la toan bo memory cua ho, khong dung cham chuan chung cua don vi.
			removed := service.Store().Purge(memory.ScopeUser) + service.Store().Purge(memory.ScopeDocument) +
				service.Store().Purge(memory.ScopeSkill)
			OK(w, map[string]any{"removed": removed})
			return
		}
		OK(w, map[string]any{"removed": service.Store().Purge(scope)})
	})

	// M4-01: xuat / nhap JSON. Export giu id/pinned/han dung; import chong trung theo hash roi tinh lai embedding.
	mux.HandleFunc("GET /v1/memory/export", func(w http.ResponseWriter, r *http.Request) {
		if !service.Available() {
			Error(w, http.StatusServiceUnavailable, "memory unavailable")
			return
		}
		items := service.Store().Export()
		OK(w, map[string]any{"count": len(items), "items": items})
	})

	mux.HandleFunc("POST /v1/memory/import", func(w http.ResponseWriter, r *http.Request) {
		if !service.Available() {
			Error(w, http.StatusServiceUnavailable, "memory unavailable")
			return
		}
		body, _ := readJSON(r)
		raw, _ := body["items"].([]any)
		rows := make([]memory.ExportRow, 0, len(raw))
		for _, entry := range raw {
			object, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			rows = append(rows, memory.ExportRow{
				ID: textOf(object["id"]), Scope: textOf(object["scope"]), ScopeKey: textOf(object["scopeKey"]),
				Text: textOf(object["text"]), Category: textOf(object["category"]), ExpiresAt: textOf(object["expiresAt"]),
				Source: textOf(object["source"]), Confidence: numberOf(object["confidence"]),
				Pinned: object["pinned"] == true, Hits: int(numberOf(object["hits"])),
				CreatedAt: textOf(object["createdAt"]), UpdatedAt: textOf(object["updatedAt"]),
				Entities: stringList(object["entities"]), Links: stringList(object["links"]),
			})
		}
		result, err := service.Store().Import(rows)
		if err != nil {
			Error(w, http.StatusInternalServerError, "import failed: "+err.Error())
			return
		}
		// Embedding tinh lai o hang doi nen (khong chan cau tra loi).
		service.QueueEmbeddingBackfill()
		OK(w, map[string]any{"added": result.Added, "skipped": result.Skipped, "links": result.Links})
	})
}

// denyOrgStyle: chan ghi chuan chung cua don vi khi khong phai quan tri vien (M3-04).
func denyOrgStyle(w http.ResponseWriter, deps *Deps, scope string) bool {
	if scope == memory.ScopeOrgStyle && !deps.Live().OrgStyleAdmin {
		Error(w, http.StatusForbidden, "scope org-style requires admin (AXIOM_ORG_STYLE_ADMIN=1)")
		return true
	}
	return false
}

// stringList: mang chuoi trong body JSON (bo qua phan tu khong phai chuoi).
func stringList(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	list := []string{}
	for _, entry := range raw {
		if text, ok := entry.(string); ok && text != "" {
			list = append(list, text)
		}
	}
	return list
}

func numberOf(value any) float64 {
	number, _ := value.(float64)
	return number
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
