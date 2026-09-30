package api

import (
	"net/http"
	"os"
	"strings"

	"axiomoffice/core/internal/skills"
)

// GET /v1/skills?app=wps|et|wpp, POST /v1/skills/reload (New_arch.md muc 7.3, 8.4.4).
func mapSkills(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("GET /v1/skills", func(w http.ResponseWriter, r *http.Request) {
		appKind := strings.TrimSpace(r.URL.Query().Get("app"))
		OK(w, skillListing(deps.Skills, appKind))
	})

	mux.HandleFunc("POST /v1/skills/reload", func(w http.ResponseWriter, r *http.Request) {
		deps.Skills.Reload()
		OK(w, skillListing(deps.Skills, ""))
	})
}

func skillListing(index *skills.Index, appKind string) map[string]any {
	list := []any{}
	items := index.All()
	if appKind != "" {
		items = index.ForApp(appKind)
	}
	for _, skill := range items {
		apps := skill.Apps
		if apps == nil {
			apps = []string{}
		}
		list = append(list, map[string]any{
			"name": skill.Name, "description": skill.Description, "apps": apps,
			"source": skill.Source, "directory": skill.Directory,
		})
	}

	failures := []any{}
	for _, failure := range index.Errors() {
		failures = append(failures, map[string]any{
			"directory": failure.Directory, "source": failure.Source, "error": failure.Error,
		})
	}

	sources := []any{}
	for _, source := range index.Sources() {
		_, err := os.Stat(source.Dir)
		sources = append(sources, map[string]any{"name": source.Name, "directory": source.Dir, "exists": err == nil})
	}

	return map[string]any{"skills": list, "errors": failures, "sources": sources}
}
