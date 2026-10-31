package api

import (
	"net/http"

	"axiomoffice/core/internal/agent"
)

// GET /v1/quick-actions (pane WPS: /wps/v1/quick-actions): danh sach muc cua menu "Sua bang AI" - id, nhan,
// va muc do co sua tai lieu khong. Pane ve menu tu day chu khong chep tay, nen them hay doi mot muc chi can
// sua agent/quickactions.go. Cau lenh (prompt) KHONG tra ra: ben goi chi gui lai id trong POST /v1/runs
// {quickAction}, Core tu lay cau lenh. `documentActions`: lenh ca tai lieu cua nut ribbon (Soat chinh ta ca bai, Lam dep
// bang) - tach khoi `actions` de khung chat mini va menu chuot phai khong hien chung.
func mapQuickActions(mux *http.ServeMux, deps *Deps) {
	routeBoth(mux, deps, "GET /v1/quick-actions", func(w http.ResponseWriter, r *http.Request) {
		OK(w, map[string]any{"actions": listActions(agent.QuickActions), "documentActions": listActions(agent.DocumentActions)})
	})
}

func listActions(actions []agent.QuickAction) []any {
	list := []any{}
	for _, action := range actions {
		item := map[string]any{"id": action.ID, "label": action.Label, "edits": action.Edits}
		// app/input chi co o lenh ca tai lieu can (nut ribbon theo ung dung, lenh cho nguoi dung go yeu cau).
		if action.App != "" {
			item["app"] = action.App
		}
		if action.Input != "" {
			item["input"] = action.Input
		}
		list = append(list, item)
	}
	return list
}
