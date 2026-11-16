package api

import (
	"net/http"
	"strings"
	"time"

	"axiomoffice/core/internal/store"
)

// Phe duyet da luu cho lenh rui ro (schema v5): nguoi dung dong y mot lan thi cac luot sau khong bi hoi
// lai (agent/confirm.go requestConfirmation). Ca ba duong deu routeBoth de pane WPS goi duoc.
func mapApprovals(mux *http.ServeMux, deps *Deps) {
	routeBoth(mux, deps, "GET /v1/approvals", func(w http.ResponseWriter, r *http.Request) {
		approvals, ok := approvalStore(w, deps)
		if !ok {
			return
		}
		list := []any{}
		for _, row := range approvals.ListApprovals() {
			list = append(list, approvalJSON(row))
		}
		OK(w, map[string]any{"approvals": list})
	})

	routeBoth(mux, deps, "POST /v1/approvals", func(w http.ResponseWriter, r *http.Request) {
		approvals, ok := approvalStore(w, deps)
		if !ok {
			return
		}
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		action, _ := body["action"].(string)
		action = strings.TrimSpace(action)
		if action == "" {
			Error(w, http.StatusBadRequest, "'action' is required")
			return
		}
		resource, _ := body["resource"].(string)
		scope, _ := body["scope"].(string)
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope == "" {
			scope = store.ApprovalScopeDocument
		}
		if scope != store.ApprovalScopeDocument && scope != store.ApprovalScopeGlobal {
			Error(w, http.StatusBadRequest, "'scope' must be \"document\" or \"global\"")
			return
		}
		expiresAt, _ := body["expiresAt"].(string)
		expiresAt = strings.TrimSpace(expiresAt)
		if expiresAt != "" {
			parsedTime, err := time.Parse(time.RFC3339, expiresAt)
			if err != nil {
				Error(w, http.StatusBadRequest, "'expiresAt' must be RFC3339 (for example 2026-12-31T00:00:00Z)")
				return
			}
			expiresAt = parsedTime.UTC().Format(time.RFC3339)
		}
		row, err := approvals.AddApproval(action, resource, scope, expiresAt)
		if err != nil {
			Error(w, http.StatusInternalServerError, "cannot save approval: "+err.Error())
			return
		}
		OK(w, approvalJSON(row))
	})

	routeBoth(mux, deps, "DELETE /v1/approvals/{id}", func(w http.ResponseWriter, r *http.Request) {
		approvals, ok := approvalStore(w, deps)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !approvals.DeleteApproval(id) {
			Error(w, http.StatusNotFound, "unknown approval: "+id)
			return
		}
		OK(w, map[string]any{"deleted": id})
	})
}

// approvalStore: kho phe duyet, hoac 503 khi core.db khong mo duoc.
func approvalStore(w http.ResponseWriter, deps *Deps) (*store.Approvals, bool) {
	if deps.Stores == nil || deps.Stores.Approvals == nil {
		Error(w, http.StatusServiceUnavailable, "memory unavailable: approvals need core.db")
		return nil, false
	}
	return deps.Stores.Approvals, true
}

func approvalJSON(row store.ApprovalRow) map[string]any {
	return map[string]any{
		"id": row.ID, "action": row.Action, "resource": row.Resource, "scope": row.Scope,
		"expiresAt": nilIfEmpty(row.ExpiresAt), "createdAt": row.CreatedAt,
	}
}
