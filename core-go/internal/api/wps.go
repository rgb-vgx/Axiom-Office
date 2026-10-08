package api

import (
	"net/http"
	"strings"
)

// Kenh add-in WPS (docs/wps-bridge.md 2.1-2.2): phuc vu file tinh va /wps/v1/poll + /wps/v1/result.
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
	// Duong dan la duoi /wps/* (vd ".." da duoc server chuan hoa): tra 404 truc tiep, khong lo
	// vo guard 401. /wps/v1/* la van ap dung xac thuc kenh (Origin la -> 403).
	mux.HandleFunc("/wps/v1/", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServeUnknownV1(w, r)
	})
	mux.HandleFunc("/wps/", func(w http.ResponseWriter, r *http.Request) {
		deps.WPS.ServeUnknown(w, r)
	})
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
