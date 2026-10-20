package wpsbridge

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
)

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
