// Package api: Core API v1 (docs/architecture/core.md muc 7.3) - ban Go cua Api/*.cs. Hinh dang response giong bridge:
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
	"axiomoffice/core/internal/mcp"
	"axiomoffice/core/internal/memory"
	"axiomoffice/core/internal/skills"
	"axiomoffice/core/internal/store"
	"axiomoffice/core/internal/wpsbridge"
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
	// Config la anh chup luc Core khoi dong (guard token, ten mutex, duong dan...). Truong nao nguoi
	// dung co the doi khi Core dang chay - nhat la LlmApiKey nam trong keyring/HKCU - phai doc qua
	// LoadConfig, khong duoc dung anh chup.
	Config config.Config

	// LoadConfig doc lai cau hinh tu nguon moi lan goi. nil = dung Config (cho test).
	LoadConfig func() config.Config

	Paths   config.Paths
	Runtime *Runtime
	HTTP    *http.Client
	Stores  *store.Stores
	Skills  *skills.Index

	// Luot chay agent (giai doan 2).
	Manager      *agent.Manager
	Orchestrator *agent.Orchestrator

	// Memory dai han (giai doan 3, muc 8.5).
	Memory *memory.Service

	// MCP client (giai doan 4, muc 8.7).
	MCP *mcp.Manager

	// Stop dung Core (POST /v1/admin/shutdown).
	Stop func()

	// WPS: bridge cho JS add-in WPS (docs/architecture/wps-bridge.md muc 2). nil = tat (cho test cu).
	WPS *wpsbridge.Bridge
}

// Live tra ve cau hinh hien tai, khong phai anh chup luc khoi dong.
func (d *Deps) Live() config.Config {
	if d.LoadConfig != nil {
		return d.LoadConfig()
	}
	return d.Config
}

// Handler dung mux + guard cho toan bo Core API.
func Handler(deps *Deps, mux *http.ServeMux) http.Handler {
	mapCore(mux, deps)
	mapWPS(mux, deps)
	mapSetup(mux, deps)
	mapLog(mux, deps)
	mapRuns(mux, deps)
	mapQuickActions(mux, deps)
	if deps.Skills != nil {
		mapSkills(mux, deps)
	}
	if deps.Memory != nil {
		mapMemory(mux, deps)
	}
	if deps.MCP != nil {
		mapMCP(mux, deps)
	}
	// Ngoai le Origin chi cho /wps/v1/* va /wps/addin/* (xac thuc rieng trong wpsbridge); moi
	// endpoint khac giu guard cu (bat ky Origin nao cung 403).
	guarded := guard(deps.Config.Token, mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isWPSPath(r.URL.Path) {
			mux.ServeHTTP(w, r)
			return
		}
		guarded.ServeHTTP(w, r)
	})
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

// ErrorCode: nhu Error nhung kem ma may doc duoc, de ben goi dich duoc sang tieng cua no ma khong phai
// do chuoi tieng Anh cua Core. Dung cho cac ca pane WPS phai noi mot cau khac han nhau (xem
// wpsadmin.go: /wps/v1/exec).
func ErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": message, "code": code})
}

// readJSON doc body; body rong -> (nil, true); JSON hong -> (nil, false).
// maxJSONBodyBytes: cung muc 8 MB voi wpsbridge.readBody. Body lon hon -> false (handler tra 400),
// khong de mot request giu RAM tuy y.
const maxJSONBodyBytes = 8 << 20

func readJSON(r *http.Request) (map[string]any, bool) {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	if err != nil || len(data) > maxJSONBodyBytes {
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
