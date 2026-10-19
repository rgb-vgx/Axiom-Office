package api

import (
	"net/http"

	"axiomoffice/core/internal/agent"
)

// GET /v1/quick-actions (pane WPS: /wps/v1/quick-actions): danh sach muc cua menu "Sua bang AI" - id, nhan,
// va muc do co sua tai lieu khong. Pane ve menu tu day chu khong chep tay, nen them hay doi mot muc chi can
// sua agent/quickactions.go. Cau lenh (prompt) KHONG tra ra: ben goi chi gui lai id trong POST /v1/runs
// {quickAction}, Core tu lay cau lenh.
func mapQuickActions(mux *http.ServeMux, deps *Deps) {
	routeBoth(mux, deps, "GET /v1/quick-actions", func(w http.ResponseWriter, r *http.Request) {
		list := []any{}
		for _, action := range agent.QuickActions {
			list = append(list, map[string]any{"id": action.ID, "label": action.Label, "edits": action.Edits})
		}
		OK(w, map[string]any{"actions": list})
	})
}
