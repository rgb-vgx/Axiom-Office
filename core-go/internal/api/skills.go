package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"axiomoffice/core/internal/skills"
)

// GET /v1/skills?app=wps|et|wpp, POST /v1/skills/reload (docs/architecture/core.md muc 7.3, 8.4.4).
func mapSkills(mux *http.ServeMux, deps *Deps) {
	routeBoth(mux, deps, "GET /v1/skills", func(w http.ResponseWriter, r *http.Request) {
		appKind := strings.TrimSpace(r.URL.Query().Get("app"))
		OK(w, skillListing(deps.Skills, appKind))
	})

	routeBoth(mux, deps, "POST /v1/skills/reload", func(w http.ResponseWriter, r *http.Request) {
		deps.Skills.Reload()
		OK(w, skillListing(deps.Skills, ""))
	})

	// Kho ky nang ca nhan (man hinh "Ky nang" cua pane): xem, sua, tao, xoa, nhap tu ngoai - skills/store.go. Moi thay doi
	// chi ghi vao thu muc ca nhan; loi tra {code, error} voi cau tieng Viet de pane hien thang.
	store := func() *skills.Store { return &skills.Store{Index: deps.Skills} }

	routeBoth(mux, deps, "GET /v1/skills/{name}", func(w http.ResponseWriter, r *http.Request) {
		detail, failure := store().Get(r.PathValue("name"))
		if failure != nil {
			skillError(w, failure)
			return
		}
		OK(w, detail)
	})

	routeBoth(mux, deps, "GET /v1/skills/{name}/file", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		text, failure := store().ReadFile(r.PathValue("name"), path)
		if failure != nil {
			skillError(w, failure)
			return
		}
		OK(w, map[string]any{"path": path, "text": text})
	})

	routeBoth(mux, deps, "POST /v1/skills", func(w http.ResponseWriter, r *http.Request) {
		edit, ok := readSkillEdit(w, r)
		if !ok {
			return
		}
		detail, failure := store().Create(edit)
		if failure != nil {
			skillError(w, failure)
			return
		}
		OK(w, detail)
	})

	routeBoth(mux, deps, "PUT /v1/skills/{name}", func(w http.ResponseWriter, r *http.Request) {
		edit, ok := readSkillEdit(w, r)
		if !ok {
			return
		}
		detail, failure := store().Save(r.PathValue("name"), edit)
		if failure != nil {
			skillError(w, failure)
			return
		}
		OK(w, detail)
	})

	routeBoth(mux, deps, "DELETE /v1/skills/{name}", func(w http.ResponseWriter, r *http.Request) {
		trash, restored, failure := store().Delete(r.PathValue("name"))
		if failure != nil {
			skillError(w, failure)
			return
		}
		OK(w, map[string]any{"trash": trash, "restored": restored})
	})

	// POST /v1/skills/import {path, dryRun, overwrite}: thu muc hoac .zip/.skill tren may nay. dryRun = xem truoc.
	routeBoth(mux, deps, "POST /v1/skills/import", func(w http.ResponseWriter, r *http.Request) {
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		dryRun, _ := body["dryRun"].(bool)
		overwrite, _ := body["overwrite"].(bool)
		result, failure := store().Import(textOf(body["path"]), dryRun, overwrite)
		if failure != nil {
			skillError(w, failure)
			return
		}
		OK(w, result)
	})
}

// readSkillEdit: body {name?, description, apps[], body, files{duong-dan: noi-dung}} - gioi han 1 MB.
func readSkillEdit(w http.ResponseWriter, r *http.Request) (skills.Edit, bool) {
	var edit skills.Edit
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
	if err != nil || len(data) > 1<<20 {
		Error(w, http.StatusBadRequest, "body too large or unreadable")
		return edit, false
	}
	if err := json.Unmarshal(data, &edit); err != nil {
		Error(w, http.StatusBadRequest, "invalid JSON body")
		return edit, false
	}
	return edit, true
}

func skillError(w http.ResponseWriter, failure *skills.StoreError) {
	status := http.StatusBadRequest
	switch failure.Code {
	case "not-found":
		status = http.StatusNotFound
	case "exists":
		status = http.StatusConflict
	case "io", "no-user-dir":
		status = http.StatusInternalServerError
	}
	ErrorCode(w, status, failure.Code, failure.Message)
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
		_, shadows := index.Shadowed(skill.Name)
		list = append(list, map[string]any{
			"name": skill.Name, "description": skill.Description, "apps": apps,
			"source": skill.Source, "directory": skill.Directory,
			// overrides: ban ca nhan dang che ban san co / to chuc cung ten.
			"overrides": shadows && skill.Source == "user",
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
