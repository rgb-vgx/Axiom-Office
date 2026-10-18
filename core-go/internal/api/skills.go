package api

import (
	"net/http"
	"os"
	"strings"

	"axiomoffice/core/internal/skills"
)

// GET /v1/skills?app=wps|et|wpp, POST /v1/skills/reload (New_arch.md muc 7.3, 8.4.4).
// POST /v1/skills/inspect + POST /v1/skills/import: nguoi dung nap skill tu thu muc ngoai bang cach COPY vao
// thu muc skill cua nguoi dung (khong tro tới thu muc ngoai), nen ba app dung chung mot ket qua.
func mapSkills(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("GET /v1/skills", func(w http.ResponseWriter, r *http.Request) {
		appKind := strings.TrimSpace(r.URL.Query().Get("app"))
		OK(w, skillListing(deps.Skills, appKind))
	})

	mux.HandleFunc("POST /v1/skills/reload", func(w http.ResponseWriter, r *http.Request) {
		deps.Skills.Reload()
		OK(w, skillListing(deps.Skills, ""))
	})

	// inspect: soi truoc khi nhap (thu muc co gi, ten nao trung) - khong ghi gi len dia.
	mux.HandleFunc("POST /v1/skills/inspect", func(w http.ResponseWriter, r *http.Request) {
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		target, ok := skillsFolder(w, deps)
		if !ok {
			return
		}
		path := textOf(body["path"])
		candidates, failures, err := skills.Inspect(path, target)
		if err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		OK(w, inspectPayload(deps, path, target, candidates, failures))
	})

	mux.HandleFunc("POST /v1/skills/import", func(w http.ResponseWriter, r *http.Request) {
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		target, ok := skillsFolder(w, deps)
		if !ok {
			return
		}
		result, err := skills.Import(textOf(body["path"]), target, flagOf(body["overwrite"]))
		if err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		deps.Skills.Reload()
		OK(w, map[string]any{
			"imported": result.Imported, "replaced": result.Replaced, "target": target,
			"listing": skillListing(deps.Skills, ""),
		})
	})
}

// skillsFolder: thu muc skill cua nguoi dung - noi duy nhat Core ghi duoc skill (thu muc canh binary la hang
// doc, con SkillDirs cua to chuc do add-in quan ly).
func skillsFolder(w http.ResponseWriter, deps *Deps) (string, bool) {
	folder := strings.TrimSpace(deps.Paths.SkillsDir)
	if folder == "" {
		Error(w, http.StatusServiceUnavailable, "Core has no user skills folder")
		return "", false
	}
	return folder, true
}

func inspectPayload(deps *Deps, path, target string, candidates []skills.Candidate, failures []skills.Failure) map[string]any {
	list := []any{}
	for _, candidate := range candidates {
		apps := candidate.Apps
		if apps == nil {
			apps = []string{}
		}
		item := map[string]any{
			"name": candidate.Name, "description": candidate.Description, "not_for": candidate.NotFor, "apps": apps,
			"source_dir": candidate.Directory, "target": candidate.TargetDir, "conflict": candidate.Conflict,
		}
		// Cung ten voi skill dang co: noi ro dang o nguon nao (nhap vao = ghi de).
		if existing := deps.Skills.Find(candidate.Name); existing != nil {
			item["existing"] = map[string]any{"source": existing.Source, "directory": existing.Directory}
		}
		list = append(list, item)
	}

	failuresJSON := []any{}
	for _, failure := range failures {
		failuresJSON = append(failuresJSON, map[string]any{"directory": failure.Directory, "error": failure.Error})
	}
	return map[string]any{"path": path, "target": target, "candidates": list, "errors": failuresJSON}
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

// flagOf: truong bool trong body JSON; nhan them chuoi "true"/"1" cho tien khi goi bang tay.
func flagOf(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}
