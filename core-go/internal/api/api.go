// Package api: Core API v1 (New_arch.md muc 7.3) - ban Go cua Api/*.cs. Hinh dang response giong bridge:
// {"ok":true,"result":...} hoac {"ok":false,"error":"..."}.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/skills"
	"axiomoffice/core/internal/store"
)

// Protocol tang khi doi hop dong Core API.
const Protocol = 1

// Runtime: port dang nghe (co the do he dieu hanh cap) va thoi diem bat dau.
type Runtime struct {
	Port    int
	Started time.Time
	Version string
}

func (r *Runtime) StartedText() string {
	return r.Started.UTC().Format("2006-01-02T15:04:05.000Z")
}

// Deps: phu thuoc cua cac endpoint (khong dung DI container, giong ban .NET).
type Deps struct {
	Config  config.Config
	Paths   config.Paths
	Runtime *Runtime
	HTTP    *http.Client
	Stores  *store.Stores
	Skills  *skills.Index

	// Luot chay agent (giai doan 2).
	Manager      *agent.Manager
	Orchestrator *agent.Orchestrator

	// Stop dung Core (POST /v1/admin/shutdown).
	Stop func()
}

// Handler dung mux + guard cho toan bo Core API.
func Handler(deps *Deps, mux *http.ServeMux) http.Handler {
	mapCore(mux, deps)
	mapSetup(mux, deps)
	mapRuns(mux, deps)
	if deps.Skills != nil {
		mapSkills(mux, deps)
	}
	return guard(deps.Config.Token, mux)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	// Giu tieng Viet va ky tu <>& doc duoc (giong UnsafeRelaxedJsonEscaping).
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func OK(w http.ResponseWriter, result any) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
}

// Error: loi nghiep vu tra HTTP 200 kem ok:false (giong bridge); status khac cho loi giao thuc.
func Error(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": message})
}

// readJSON doc body; body rong -> (nil, true); JSON hong -> (nil, false).
func readJSON(r *http.Request) (map[string]any, bool) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, false
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, true
	}
	var body map[string]any
	if json.Unmarshal(data, &body) != nil {
		// Body la JSON hop le nhung khong phai object (vd mang) -> coi nhu khong co truong nao.
		var anything any
		return nil, json.Unmarshal(data, &anything) == nil
	}
	return body, true
}

func pid() int { return os.Getpid() }
