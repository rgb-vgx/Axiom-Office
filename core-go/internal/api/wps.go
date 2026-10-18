package api

import (
	"net/http"

	"axiomoffice/core/internal/agent"
	"axiomoffice/core/internal/wpsbridge"
	"strings"
)

// Kenh add-in WPS (docs/wps-bridge.md 2.1-2.2, muc 7): phuc vu file tinh, /wps/v1/poll + /wps/v1/result
// va luot chay agent cho pane Ask AI (/wps/v1/runs...).
// Xac thuc rieng (Origin dung bang origin cua Core hoac token khi khong Origin) nen di thang vao
// mux, khong qua guard chung (guard chung chan moi Origin).
func mapWPS(mux *http.ServeMux, deps *Deps) {
	if deps.WPS == nil {
		return
	}
	mux.HandleFunc("/wps/addin/", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServeAddin(w, r)
	})
	mux.HandleFunc("/wps/v1/poll", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServePoll(w, r)
	})
	mux.HandleFunc("/wps/v1/poll/", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServePoll(w, r)
	})
	mux.HandleFunc("/wps/v1/result", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServeResult(w, r)
	})
	mux.HandleFunc("/wps/v1/result/", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServeResult(w, r)
	})
	mapWPSRuns(mux, deps)
	mapWPSAdmin(mux, deps)
	// Duong dan la duoi /wps/* (vd ".." da duoc server chuan hoa): tra 404 truc tiep, khong lo
	// vo guard 401. /wps/v1/* la van ap dung xac thuc kenh (Origin la -> 403).
	mux.HandleFunc("/wps/v1/", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServeUnknownV1(w, r)
	})
	mux.HandleFunc("/wps/", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServeUnknown(w, r)
	})
}

// mapWPSRuns: luot chay agent cho pane Ask AI (docs/wps-bridge.md muc 7) - cung hanh vi va response
// nhu /v1/runs, nhung Core tu gan office (bridge WPS) va document (lan poll gan nhat cua add-in).
// Xac thuc kenh add-in (DecideWPSAuth + doi body JSON); /v1/runs giu guard cu.
func mapWPSRuns(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("POST /wps/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		if !deps.Stores.Available {
			Error(w, http.StatusServiceUnavailable, "memory unavailable: "+deps.Stores.Error)
			return
		}
		if !deps.WPS.Connected() {
			Error(w, http.StatusConflict, "WPS add-in is not connected: open a document in WPS to connect the add-in")
			return
		}
		body, parsed := readJSON(r)
		if !parsed {
			Error(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if body == nil {
			body = map[string]any{}
		}
		// Office do Core tu gan (giong /health, /session cua bridge WPS); pane khong gui office.
		body["office"] = map[string]any{
			"port": float64(deps.WPS.ActualBridgePort()), "pid": float64(pid()),
			"app": "wps", "family": "wps-linux",
		}
		// Tai lieu do add-in bao trong lan poll gan nhat.
		if doc, _, _, _ := deps.WPS.Snapshot(); len(doc) > 0 {
			body["document"] = doc
		}
		request, problem := agent.ParseRequest(body)
		if request == nil {
			if problem == "" {
				problem = "invalid request"
			}
			Error(w, http.StatusBadRequest, problem)
			return
		}
		serveStartRun(w, deps, request)
	})

	mux.HandleFunc("GET /wps/v1/runs/{runId}", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		serveGetRun(w, deps, r.PathValue("runId"))
	})

	mux.HandleFunc("GET /wps/v1/runs/{runId}/events", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		serveRunEvents(w, r, deps, r.PathValue("runId"))
	})

	mux.HandleFunc("POST /wps/v1/runs/{runId}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		serveCancelRun(w, deps, r.PathValue("runId"))
	})

	mux.HandleFunc("POST /wps/v1/runs/{runId}/confirm", func(w http.ResponseWriter, r *http.Request) {
		if !checkWPSAuth(w, r, deps) {
			return
		}
		body, parsed := readJSON(r)
		if !parsed {
			body = nil
		}
		serveConfirmRun(w, deps, r.PathValue("runId"), body)
	})
}

// checkWPSAuth: xac thuc kenh add-in cho moi endpoint /wps/v1/* (giong wpsbridge.checkWPSAuth: Origin dung
// bang origin cua Core thi qua, Origin la -> 403, khong Origin thi doi token -> 401, body JSON -> 415).
// Trinh duyet khong gui Origin cho GET cung origin nen EventSource cua pane bi 401 (11/10/2026):
// GET/HEAD khong Origin ma co Sec-Fetch-Site: same-origin thi cho qua; POST van doi token.
func checkWPSAuth(w http.ResponseWriter, r *http.Request, deps *Deps) bool {
	if code, ok := wpsbridge.DecideWPSAuth(r.Header.Get("Origin"), r.Header.Get("X-Auth-Token"), deps.Config.Token, deps.Runtime.Port, r.Method, r.Header.Get("Sec-Fetch-Site")); !ok {
		if code == http.StatusForbidden {
			Error(w, code, "requests with an Origin header are not allowed")
		} else {
			Error(w, code, "unauthorized")
		}
		return false
	}
	if !wpsbridge.CheckJSONContent(r) {
		Error(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	return true
}

// isWPSPath: ngoai le Origin chi ap dung cho /wps/* (xac thuc rieng trong wpsbridge, duong dan la
// tra 404 truc tiep); moi endpoint khac giu guard cu. Bao ca dang "/wps/secret.txt" sau khi server
// chuan hoa duong dan ".." de khong lo vo guard 401.
func isWPSPath(path string) bool {
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "/wps" {
		return true
	}
	return strings.HasPrefix(trimmed, "/wps/")
}
