package wpsbridge

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// OwnOrigin tra ve origin cua chinh Core (trang add-in do Core phuc vu).
func OwnOrigin(corePort int) string {
	return "http://127.0.0.1:" + strconv.Itoa(corePort)
}

// DecideWPSAuth quyet dinh xac thuc cho kenh /wps/v1/*: Origin dung bang origin cua Core thi cho qua
// khong can token; khong Origin thi doi token; Origin khac thi 403. Trinh duyet khong gui Origin
// cho GET cung origin nen EventSource cua pane bi 401 (11/10/2026): voi GET/HEAD khong Origin ma co
// Sec-Fetch-Site: same-origin thi cho qua (header do trinh duyet tu dat, trang khac khong gia duoc).
// Tra ve (0, true) khi cho qua, nguoc lai (ma loi, false).
func DecideWPSAuth(origin, tokenHeader, token string, corePort int, method, secFetchSite string) (int, bool) {
	if origin != "" {
		if origin == OwnOrigin(corePort) {
			return 0, true
		}
		return http.StatusForbidden, false
	}
	// GET/HEAD cung origin khong co Origin nhung co Sec-Fetch-Site: same-origin thi cho qua.
	if (method == http.MethodGet || method == http.MethodHead) && secFetchSite == "same-origin" {
		return 0, true
	}
	if token != "" && tokenHeader != token {
		return http.StatusUnauthorized, false
	}
	return 0, true
}

// DecideBridgeAuth guard cua bridge WPS (giong bridge LibreOffice): bat ky Origin nao cung 403,
// thieu/sai token thi 401 (tru GET /health).
func DecideBridgeAuth(origin, tokenHeader, token, path, method string) (int, bool) {
	if origin != "" {
		return http.StatusForbidden, false
	}
	trimmed := strings.TrimRight(path, "/")
	if method == http.MethodGet && trimmed == "/health" {
		return 0, true
	}
	if token != "" && tokenHeader != token {
		return http.StatusUnauthorized, false
	}
	return 0, true
}

// NeedsJSON cho biet request co body can doi application/json nhu guard cua Core.
func NeedsJSON(r *http.Request) bool {
	switch strings.ToUpper(r.Method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return r.ContentLength > 0 || len(r.TransferEncoding) > 0
	}
	return false
}

// CheckJSONContent tra ve true khi Content-Type chap nhan duoc (hoac khong co body).
func CheckJSONContent(r *http.Request) bool {
	if !NeedsJSON(r) {
		return true
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json")
}

// CleanAddinRel kiem tra duong dan sau /wps/addin/ khong thoat khoi thu muc: chan "..", duong dan
// tuyet doi va dang ma hoa %2e/%2f/%5c. Tra ve duong dan tuong doi da giai ma khi hop le.
func CleanAddinRel(rawURI, prefix string) (string, bool) {
	raw := rawURI
	if index := strings.Index(raw, "?"); index >= 0 {
		raw = raw[:index]
	}
	if index := strings.Index(raw, "#"); index >= 0 {
		raw = raw[:index]
	}
	if !strings.HasPrefix(raw, prefix) {
		return "", false
	}
	rel := raw[len(prefix):]
	if rel == "" {
		return "", false
	}
	lower := strings.ToLower(rel)
	if strings.Contains(lower, "%2e") || strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
		return "", false
	}
	unescaped, err := url.PathUnescape(rel)
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(unescaped, "/") || strings.HasPrefix(unescaped, "\\") {
		return "", false
	}
	for _, part := range strings.Split(strings.ReplaceAll(unescaped, "\\", "/"), "/") {
		if part == ".." {
			return "", false
		}
	}
	if strings.Contains(unescaped, "\x00") {
		return "", false
	}
	cleaned := strings.ReplaceAll(unescaped, "\\", "/")
	if cleaned == "" || cleaned == "." {
		return "", false
	}
	return unescaped, true
}

// ContentTypeFor tra ve Content-Type theo duoi file add-in.
func ContentTypeFor(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}
