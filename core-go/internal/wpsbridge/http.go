package wpsbridge

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ServeAddin phuc vu file tinh GET /wps/addin/<file>: khong token, chi GET/HEAD, chan thoat thu muc.
func (b *Bridge) ServeAddin(w http.ResponseWriter, r *http.Request) {
	// File add-in duoc CEF/WPS nap tu localhost; ban moi phai thang ban cache, va cac file nay nho nen nap lai khong dang ke.
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != OwnOrigin(b.corePort()) {
		writeErr(w, http.StatusForbidden, "requests with an Origin header are not allowed")
		return
	}
	rel, ok := CleanAddinRel(r.RequestURI, "/wps/addin/")
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	full := filepath.Join(b.opts.AddinDir, filepath.FromSlash(strings.ReplaceAll(rel, "/", string(filepath.Separator))))
	absBase, errBase := filepath.Abs(b.opts.AddinDir)
	absFull, errFull := filepath.Abs(full)
	if errBase != nil || errFull != nil || (absFull != absBase && !strings.HasPrefix(absFull, absBase+string(filepath.Separator))) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	info, err := os.Stat(absFull)
	if err != nil || info.IsDir() {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	data, err := os.ReadFile(absFull)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", ContentTypeFor(absFull))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

// checkWPSAuth ap dung DecideWPSAuth + doi body JSON cho /wps/v1/*.
func (b *Bridge) checkWPSAuth(w http.ResponseWriter, r *http.Request) bool {
	if code, ok := DecideWPSAuth(r.Header.Get("Origin"), r.Header.Get("X-Auth-Token"), b.opts.Token, b.corePort(), r.Method, r.Header.Get("Sec-Fetch-Site")); !ok {
		if code == http.StatusForbidden {
			writeErr(w, code, "requests with an Origin header are not allowed")
		} else {
			writeErr(w, code, "unauthorized")
		}
		return false
	}
	if !CheckJSONContent(r) {
		writeErr(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	return true
}

// ServePoll xu ly POST /wps/v1/poll: giu request toi khi co lenh hoac het PollSeconds.
func (b *Bridge) ServePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !b.checkWPSAuth(w, r) {
		return
	}
	body, ok := readBody(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	b.recordPoll(body)
	b.maybeWriteSession(false)

	deadline := time.Now().Add(time.Duration(b.opts.PollSeconds) * time.Second)
	for {
		jobs := b.takeJobs()
		if len(jobs) > 0 {
			writeOK(w, map[string]any{"jobs": jobItems(jobs)})
			return
		}
		if time.Now().After(deadline) || time.Until(deadline) <= 0 {
			writeOK(w, map[string]any{"jobs": []any{}})
			return
		}
		select {
		case <-b.stopCh:
			writeOK(w, map[string]any{"jobs": []any{}})
			return
		case <-time.After(50 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			// Kiem tra lan cuoi truoc khi het gio: lenh co the vua vao hang doi.
			jobs := b.takeJobs()
			if len(jobs) > 0 {
				writeOK(w, map[string]any{"jobs": jobItems(jobs)})
				return
			}
			writeOK(w, map[string]any{"jobs": []any{}})
			return
		}
	}
}

// jobItems: dang JSON cua cac job tra cho add-in trong mot poll.
func jobItems(jobs []*Job) []any {
	items := make([]any, 0, len(jobs))
	for _, job := range jobs {
		params := job.Params
		if params == nil {
			params = map[string]any{}
		}
		items = append(items, map[string]any{"id": job.ID, "action": job.Action, "params": params})
	}
	return items
}

// ServeResult xu ly POST /wps/v1/result.
func (b *Bridge) ServeResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !b.checkWPSAuth(w, r) {
		return
	}
	body, ok := readBody(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body == nil {
		writeErr(w, http.StatusOK, "missing 'id' field")
		return
	}
	id, _ := body["id"].(string)
	if strings.TrimSpace(id) == "" {
		writeErr(w, http.StatusOK, "missing 'id' field")
		return
	}
	flag, _ := body["ok"].(bool)
	var reply map[string]any
	if flag {
		var result any
		if value, present := body["result"]; present {
			result = value
		}
		reply = map[string]any{"ok": true, "result": result}
	} else {
		message, _ := body["error"].(string)
		reply = map[string]any{"ok": false, "error": message}
	}
	if !b.Complete(id, reply) {
		writeErr(w, http.StatusOK, "unknown job id")
		return
	}
	writeOK(w, nil)
}

// ServeUnknown tra 404 cho duong dan la duoi /wps/*.
func (b *Bridge) ServeUnknown(w http.ResponseWriter, r *http.Request) {
	writeErr(w, http.StatusNotFound, "not found")
}

// ServeUnknownV1 tra 404 cho duong dan la duoi /wps/v1/*, sau khi ap dung xac thuc kenh.
func (b *Bridge) ServeUnknownV1(w http.ResponseWriter, r *http.Request) {
	if !b.checkWPSAuth(w, r) {
		return
	}
	writeErr(w, http.StatusNotFound, "not found")
}

// readBody doc JSON object; body rong -> (nil, true); JSON hong -> (nil, false).
func readBody(r *http.Request) (map[string]any, bool) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return nil, false
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, true
	}
	var body map[string]any
	if json.Unmarshal(data, &body) != nil {
		var anything any
		if json.Unmarshal(data, &anything) != nil {
			return nil, false
		}
		return map[string]any{}, true
	}
	if body == nil {
		return map[string]any{}, true
	}
	return body, true
}

func writeOK(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	payload := map[string]any{"ok": true}
	if result != nil {
		payload["result"] = result
	} else {
		// /wps/v1/result thanh cong khong co truong result: chi {"ok":true}.
		payload = map[string]any{"ok": true}
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, message string) {
	// Loi nghiep vu (ok:false kem HTTP 200) dung cho id la cua /result va /cmd khi chua ket noi;
	// loi giao thuc (401/403/404/405/415) cung dang {"ok":false,"error"} nhu guard cua Core.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(map[string]any{"ok": false, "error": message})
}
