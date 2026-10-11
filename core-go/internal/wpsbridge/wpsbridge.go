// Package wpsbridge: Core lam bridge cho JS add-in WPS (docs/wps-bridge.md muc 2) - phuc vu file
// add-in, kenh /wps/v1/poll + /wps/v1/result tren Core API va bridge WPS tren cong rieng.
package wpsbridge

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"axiomoffice/core/internal/corelog"
)

const (
	// DefaultBridgePort: cong bridge WPS mac dinh (docs/wps-bridge.md 2.3).
	DefaultBridgePort = 47861
	// DefaultPollSeconds: poll giu request bao lau khi chua co lenh.
	DefaultPollSeconds = 20
	// DefaultDisconnectSeconds: khong poll trong bao lau thi coi nhu mat ket noi.
	DefaultDisconnectSeconds = 45
	// CmdWaitSeconds: POST /cmd cho ket qua cua add-in toi da bao lau.
	CmdWaitSeconds = 300
	// SessionRefreshSeconds: ghi lai file session moi 25 giay khi van ket noi.
	SessionRefreshSeconds = 25
)

// Options cau hinh bridge WPS, doc tu bien moi truong o main.
type Options struct {
	AddinDir          string
	BridgePort        int
	PollSeconds       int
	DisconnectSeconds int
	CmdWaitSeconds    int
	SessionDir        string
	Token             string
	Version           string
	CorePort          int
}

// ResolveOptions doc cau hinh tu ham lay bien moi truong (truyen os.Getenv o main).
func ResolveOptions(getenv func(string) string, exeDir, sessionDir, token, version string, corePort int) Options {
	addinDir := strings.TrimSpace(getenv("AXIOM_WPS_ADDIN_DIR"))
	if addinDir == "" {
		addinDir = filepath.Join(exeDir, "wps-addin")
	}
	return Options{
		AddinDir:          addinDir,
		BridgePort:        intEnv(getenv, "AXIOM_WPS_BRIDGE_PORT", DefaultBridgePort),
		PollSeconds:       intEnv(getenv, "AXIOM_WPS_POLL_SECONDS", DefaultPollSeconds),
		DisconnectSeconds: intEnv(getenv, "AXIOM_WPS_DISCONNECT_SECONDS", DefaultDisconnectSeconds),
		CmdWaitSeconds:    intEnv(getenv, "AXIOM_WPS_CMD_WAIT_SECONDS", CmdWaitSeconds),
		SessionDir:        sessionDir,
		Token:             token,
		Version:           version,
		CorePort:          corePort,
	}
}

func intEnv(getenv func(string) string, key string, fallback int) int {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value <= 0 {
		return fallback
	}
	return value
}

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

// Job la mot lenh Core giao cho add-in thuc thi.
type Job struct {
	ID     string
	Action string
	Params map[string]any
	wait   chan map[string]any
	// sentAt la luc job duoc giao cho add-in (takeJobs dat); zero nghia la chua giao.
	sentAt time.Time
}

// Bridge giu trang thai kenh add-in: hang doi lenh, catalog, tai lieu va file session.
type Bridge struct {
	opts Options

	mu           sync.Mutex
	lastPoll     time.Time
	everPoll     bool
	app          string
	document     map[string]any
	commands     []any
	addinVersion string
	started      time.Time

	queue   []*Job
	waiting map[string]*Job
	seq     uint64

	sessionPath  string
	lastWrite    time.Time
	lastDocKey   string
	lastAppKey   string
	wroteSession bool

	bridgeListener net.Listener
	bridgeServer   *http.Server
	bridgePort     int

	stopCh   chan struct{}
	stopOnce sync.Once
	stopped  atomic.Bool
}

// New tao bridge WPS tu Options (chua mo cong, chua ghi session).
func New(opts Options) *Bridge {
	if opts.PollSeconds <= 0 {
		opts.PollSeconds = DefaultPollSeconds
	}
	if opts.DisconnectSeconds <= 0 {
		opts.DisconnectSeconds = DefaultDisconnectSeconds
	}
	if opts.BridgePort <= 0 {
		opts.BridgePort = DefaultBridgePort
	}
	if opts.CmdWaitSeconds <= 0 {
		opts.CmdWaitSeconds = CmdWaitSeconds
	}
	return &Bridge{
		opts:        opts,
		app:         "wps",
		waiting:     map[string]*Job{},
		sessionPath: filepath.Join(opts.SessionDir, strconv.Itoa(os.Getpid())+"-wps.json"),
		started:     time.Now().UTC(),
		stopCh:      make(chan struct{}),
		bridgePort:  opts.BridgePort,
	}
}

// SetCorePort cap nhat cong Core (port chi biet sau khi listen).
func (b *Bridge) SetCorePort(port int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.opts.CorePort = port
}

func (b *Bridge) corePort() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.opts.CorePort
}

// busyLocked tra ve true khi dang co job da giao cho add-in ma chua qua CmdWaitSeconds.
// Add-in chay don luong nen luc chay lenh dai no khong poll: coi nhu van ket noi.
// Chi goi khi dang giu b.mu; het han thi duong het gio cua Enqueue goi forget nen tu het.
func (b *Bridge) busyLocked(now time.Time) bool {
	wait := b.opts.CmdWaitSeconds
	if wait <= 0 {
		wait = CmdWaitSeconds
	}
	limit := time.Duration(wait) * time.Second
	for _, job := range b.waiting {
		if job.sentAt.IsZero() {
			continue
		}
		if now.Sub(job.sentAt) < limit {
			return true
		}
	}
	return false
}

// Connected cho biet co poll trong DisconnectSeconds giay gan nhat, hoac dang co lenh chay.
func (b *Bridge) Connected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.busyLocked(time.Now()) {
		return true
	}
	if !b.everPoll {
		return false
	}
	return time.Since(b.lastPoll) <= time.Duration(b.opts.DisconnectSeconds)*time.Second
}

// Snapshot tra ve tai lieu + catalog moi nhat (cho /commands, /session).
func (b *Bridge) Snapshot() (map[string]any, []any, string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	connected := b.busyLocked(now) || (b.everPoll && now.Sub(b.lastPoll) <= time.Duration(b.opts.DisconnectSeconds)*time.Second)
	doc := map[string]any{}
	for key, value := range b.document {
		doc[key] = value
	}
	cmds := append([]any{}, b.commands...)
	return doc, cmds, b.addinVersion, connected
}

// newJobID sinh id duy nhat cho job.
func (b *Bridge) newJobID() string {
	seq := atomic.AddUint64(&b.seq, 1)
	var random [8]byte
	_, _ = rand.Read(random[:])
	return fmt.Sprintf("wps-%d-%d-%s", os.Getpid(), seq, hex.EncodeToString(random[:]))
}

// Enqueue dua lenh vao hang doi va cho ket qua toi da CmdWaitSeconds giay (theo cau hinh).
func (b *Bridge) Enqueue(action string, params map[string]any) (map[string]any, bool) {
	if !b.Connected() {
		return nil, false
	}
	job := &Job{ID: b.newJobID(), Action: action, Params: params, wait: make(chan map[string]any, 1)}
	b.mu.Lock()
	b.queue = append(b.queue, job)
	b.waiting[job.ID] = job
	cmdWait := b.opts.CmdWaitSeconds
	b.mu.Unlock()
	if cmdWait <= 0 {
		cmdWait = CmdWaitSeconds
	}

	// Add-in mat ket noi giua chung (nguoi dung dong WPS) thi bao loi ngay, khong treo toi CmdWaitSeconds.
	// Lenh da giao cho add-in (dang chay) thi bo qua luat disconnect: add-in don luong khong the vua
	// chay vua poll, chi het gio theo CmdWaitSeconds.
	timeout := time.NewTimer(time.Duration(cmdWait) * time.Second)
	defer timeout.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case reply := <-job.wait:
			return reply, true
		case <-timeout.C:
			b.forget(job)
			return map[string]any{"ok": false, "error": fmt.Sprintf("WPS add-in did not answer within %ds; the command may have been applied - check the document before retrying", cmdWait)}, true
		case <-ticker.C:
			b.mu.Lock()
			sent := !job.sentAt.IsZero()
			b.mu.Unlock()
			if sent {
				continue
			}
			if !b.Connected() {
				b.forget(job)
				return map[string]any{"ok": false, "error": "WPS add-in disconnected before the command was sent (WPS closed?) - it was not sent, safe to retry"}, true
			}
		case <-b.stopCh:
			return map[string]any{"ok": false, "error": "Core is stopping"}, true
		}
	}
}

// forget bo job khoi hang doi va danh sach cho ket qua (het gio / mat ket noi).
func (b *Bridge) forget(job *Job) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.waiting, job.ID)
	for i, item := range b.queue {
		if item == job {
			b.queue = append(b.queue[:i], b.queue[i+1:]...)
			break
		}
	}
}

// takeJobs lay toan bo job dang doi cho mot poll, danh dau gio giao de biet lenh dang chay.
func (b *Bridge) takeJobs() []*Job {
	b.mu.Lock()
	defer b.mu.Unlock()
	jobs := b.queue
	b.queue = nil
	now := time.Now()
	for _, job := range jobs {
		job.sentAt = now
	}
	return jobs
}

// Complete ghi nhan ket qua cua add-in; id la thi false.
func (b *Bridge) Complete(id string, reply map[string]any) bool {
	b.mu.Lock()
	job, ok := b.waiting[id]
	if ok {
		delete(b.waiting, id)
	}
	b.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case job.wait <- reply:
	default:
	}
	return true
}

// bridgeApp tra ve app cua phien theo poll cua add-in ("wps" khi chua biet).
func (b *Bridge) bridgeApp() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.app == "" {
		return "wps"
	}
	return b.app
}

// recordPoll luu heartbeat + catalog + tai lieu tu body poll.
func (b *Bridge) recordPoll(body map[string]any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.lastPoll = now
	b.everPoll = true
	// App do add-in bao: chi nhan "wps" | "et" | "wpp"; thieu hoac la thi giu app dang biet.
	if raw, ok := body["app"].(string); ok {
		switch raw {
		case "wps", "et", "wpp":
			b.app = raw
		}
	}
	if doc, ok := body["document"].(map[string]any); ok {
		next := map[string]any{}
		for key, value := range doc {
			next[key] = value
		}
		b.document = next
	}
	if cmds, ok := body["commands"].([]any); ok {
		b.commands = append([]any{}, cmds...)
	} else {
		b.commands = nil
	}
	if version, ok := body["version"].(string); ok {
		b.addinVersion = version
	}
}

// ServeAddin phuc vu file tinh GET /wps/addin/<file>: khong token, chi GET/HEAD, chan thoat thu muc.
func (b *Bridge) ServeAddin(w http.ResponseWriter, r *http.Request) {
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

// ---- Bridge WPS tren cong rieng ----

// BridgeMux tra ve mux cho listener rieng (/health, /commands, /session, /cmd).
func (b *Bridge) BridgeMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", b.serveBridgeHealth)
	mux.HandleFunc("/health/", b.serveBridgeHealth)
	mux.HandleFunc("/commands", b.serveBridgeCommands)
	mux.HandleFunc("/commands/", b.serveBridgeCommands)
	mux.HandleFunc("/session", b.serveBridgeSession)
	mux.HandleFunc("/session/", b.serveBridgeSession)
	mux.HandleFunc("/cmd", b.serveBridgeCmd)
	mux.HandleFunc("/cmd/", b.serveBridgeCmd)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			writeErr(w, http.StatusForbidden, "requests with an Origin header are not allowed")
			return
		}
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
	})
	return mux
}

// StartBridge mo listener bridge; cong ban thi log canh bao va tiep tuc khong co bridge.
func (b *Bridge) StartBridge() {
	mux := b.BridgeMux()
	guarded := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if code, ok := DecideBridgeAuth(r.Header.Get("Origin"), r.Header.Get("X-Auth-Token"), b.opts.Token, path, r.Method); !ok {
			if code == http.StatusForbidden {
				writeErr(w, code, "requests with an Origin header are not allowed")
			} else {
				writeErr(w, code, "unauthorized")
			}
			return
		}
		if strings.HasSuffix(strings.TrimRight(path, "/"), "/cmd") && r.Method == http.MethodPost && !CheckJSONContent(r) {
			writeErr(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		if strings.HasPrefix(strings.TrimRight(path, "/"), "/cmd") && r.Method != http.MethodPost {
			// GET /cmd khong co trong giao thuc: 404 nhu bridge LibreOffice.
			writeErr(w, http.StatusNotFound, "not found: "+path)
			return
		}
		mux.ServeHTTP(w, r)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(b.opts.BridgePort))
	if err != nil {
		corelog.Info("wps bridge: cannot listen on 127.0.0.1:%d (%s), continuing without the WPS bridge", b.opts.BridgePort, err.Error())
		return
	}
	b.mu.Lock()
	b.bridgeListener = listener
	b.bridgePort = listener.Addr().(*net.TCPAddr).Port
	b.mu.Unlock()
	server := &http.Server{Handler: guarded, ReadHeaderTimeout: 10 * time.Second}
	b.mu.Lock()
	b.bridgeServer = server
	b.mu.Unlock()
	go func() {
		_ = server.Serve(listener)
	}()
	corelog.Info("wps bridge listening on http://127.0.0.1:%d/", b.ActualBridgePort())
}

// ActualBridgePort tra ve cong bridge dang nghe (hoac cong cau hinh khi chua mo).
func (b *Bridge) ActualBridgePort() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.bridgePort != 0 {
		return b.bridgePort
	}
	return b.opts.BridgePort
}

func (b *Bridge) bridgeVersion() string {
	if b.opts.Version != "" {
		return b.opts.Version
	}
	return "0.0.0"
}

func (b *Bridge) serveBridgeHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
		return
	}
	writeOK(w, map[string]any{
		"app": b.bridgeApp(), "family": "wps-linux", "pid": os.Getpid(),
		"port": b.ActualBridgePort(), "version": b.bridgeVersion(), "connected": b.Connected(),
	})
}

func (b *Bridge) serveBridgeCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
		return
	}
	_, cmds, addinVersion, connected := b.Snapshot()
	if !b.everPolled() {
		_ = connected
		writeErr(w, http.StatusOK, "WPS add-in has never connected")
		return
	}
	version := addinVersion
	if version == "" {
		version = b.bridgeVersion()
	}
	if cmds == nil {
		cmds = []any{}
	}
	writeOK(w, map[string]any{"version": version, "commands": cmds})
}

func (b *Bridge) everPolled() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.everPoll
}

func (b *Bridge) serveBridgeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
		return
	}
	doc, _, _, _ := b.Snapshot()
	if !b.everPolled() {
		writeErr(w, http.StatusOK, "WPS add-in has never connected")
		return
	}
	writeOK(w, map[string]any{
		"app": b.bridgeApp(), "family": "wps-linux", "port": b.ActualBridgePort(), "pid": os.Getpid(),
		"document": doc,
	})
}

func (b *Bridge) serveBridgeCmd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
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
	action, _ := body["action"].(string)
	if strings.TrimSpace(action) == "" {
		writeErr(w, http.StatusBadRequest, "missing 'action' field")
		return
	}
	params := map[string]any{}
	if raw, ok := body["params"].(map[string]any); ok {
		params = raw
	}
	if !b.Connected() {
		writeErr(w, http.StatusOK, "WPS add-in is not connected: open a document in WPS to connect the add-in")
		return
	}
	reply, _ := b.Enqueue(action, params)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(reply)
}

// ---- File session ----

func docKey(doc map[string]any) string {
	name, _ := doc["name"].(string)
	full, _ := doc["fullName"].(string)
	return name + "\x00" + full
}

// maybeWriteSession ghi file session khi tai lieu doi (goi ngay sau poll).
func (b *Bridge) maybeWriteSession(force bool) {
	if !b.Connected() {
		return
	}
	b.mu.Lock()
	key := docKey(b.document)
	app := b.app
	if app == "" {
		app = "wps"
	}
	need := force || !b.wroteSession || key != b.lastDocKey || app != b.lastAppKey
	b.mu.Unlock()
	if need {
		b.writeSession()
	}
}

// writeSession ghi file <pid>-wps.json nguyen tu (temp + rename).
func (b *Bridge) writeSession() {
	b.mu.Lock()
	doc := map[string]any{}
	for key, value := range b.document {
		doc[key] = value
	}
	port := b.bridgePort
	if port == 0 {
		port = b.opts.BridgePort
	}
	now := time.Now().UTC()
	app := b.app
	if app == "" {
		app = "wps"
	}
	payload := map[string]any{
		"pid": os.Getpid(), "app": app, "family": "wps-linux", "port": port,
		"host": "wps-jsaddon", "version": b.bridgeVersion(),
		"started":       b.started.Format("2006-01-02T15:04:05.000Z"),
		"lastSeen":      now.Format("2006-01-02T15:04:05.000Z"),
		"lastSeenEpoch": float64(now.UnixNano()) / 1e9,
		"document":      doc["name"],
		"documentPath":  doc["fullName"],
	}
	path := b.sessionPath
	key := docKey(b.document)
	b.mu.Unlock()

	if strings.TrimSpace(b.opts.SessionDir) == "" {
		return
	}
	_ = os.MkdirAll(b.opts.SessionDir, 0o755)
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var pretty bytes.Buffer
	if json.Indent(&pretty, data, "", "  ") == nil {
		data = pretty.Bytes()
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return
	}
	b.mu.Lock()
	b.lastWrite = time.Now()
	b.lastDocKey = key
	b.lastAppKey = app
	b.wroteSession = true
	b.mu.Unlock()
}

// removeSession xoa file session (bo qua loi).
func (b *Bridge) removeSession() {
	b.mu.Lock()
	path := b.sessionPath
	b.wroteSession = false
	b.mu.Unlock()
	_ = os.Remove(path)
}

// StartSessionLoop bat vong lam moi/xoa file session theo trang thai ket noi.
func (b *Bridge) StartSessionLoop() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-b.stopCh:
				return
			case <-ticker.C:
				if b.stopped.Load() {
					return
				}
				if b.Connected() {
					b.mu.Lock()
					need := !b.wroteSession || time.Since(b.lastWrite) >= SessionRefreshSeconds*time.Second
					b.mu.Unlock()
					if need {
						b.writeSession()
					}
				} else if b.sessionExists() {
					b.removeSession()
				}
			}
		}
	}()
}

func (b *Bridge) sessionExists() bool {
	b.mu.Lock()
	path := b.sessionPath
	wrote := b.wroteSession
	b.mu.Unlock()
	if !wrote {
		// Van kiem tra file that: co the do lan chay truoc de lai cung pid? Chi xoa khi ta da tung ghi.
		return wrote
	}
	_, err := os.Stat(path)
	return err == nil
}

// Start khoi dong bridge (listener + vong session).
func (b *Bridge) Start() {
	b.StartBridge()
	b.StartSessionLoop()
}

// Stop dung bridge: dong listener rieng, xoa file session.
func (b *Bridge) Stop() {
	b.stopOnce.Do(func() {
		b.stopped.Store(true)
		close(b.stopCh)
		b.mu.Lock()
		server := b.bridgeServer
		b.mu.Unlock()
		if server != nil {
			// Dong ngay de Core thoat nhanh khi shutdown (khong cho keep-alive).
			_ = server.Close()
		}
		b.removeSession()
		// Huy cac poll va /cmd dang cho.
		b.mu.Lock()
		for _, job := range b.waiting {
			select {
			case job.wait <- map[string]any{"ok": false, "error": "Core is stopping"}:
			default:
			}
		}
		b.mu.Unlock()
	})
}
