package api

import (
	"io"
	"net/http"
	"strings"
)

// guard: cung quy tac voi bridge (New_arch.md muc 7.2) - chan request co header Origin (403), moi endpoint
// tru GET /health can X-Auth-Token (401) khi Token duoc cau hinh, request co body phai la application/json (415).
func guard(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			reject(w, r, http.StatusForbidden, "requests with an Origin header are not allowed")
			return
		}
		if r.Method == http.MethodGet && strings.TrimRight(r.URL.Path, "/") == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if token != "" && r.Header.Get("X-Auth-Token") != token {
			reject(w, r, http.StatusUnauthorized, "unauthorized")
			return
		}
		if hasBody(r) && !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
			reject(w, r, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// reject doc bo body nho truoc khi tu choi: tra loi khi client con dang gui body thi Windows co the
// dong ket noi bang RST va client nhan ConnectionReset thay vi ma loi.
func reject(w http.ResponseWriter, r *http.Request, status int, message string) {
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
	Error(w, status, message)
}

func hasBody(r *http.Request) bool {
	switch strings.ToUpper(r.Method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return r.ContentLength > 0 || len(r.TransferEncoding) > 0
	}
	return false
}
