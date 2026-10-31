package wpsbridge

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/corelog"
)

// ---- Bridge WPS tren cong rieng ----

// BridgeMux tra ve mux cho listener cua cong chinh (/health, /commands, /session, /cmd): moi request phuc vu client
// dang giu cong chinh luc do.
func (b *Bridge) BridgeMux() *http.ServeMux { return b.clientMux(b.Primary) }

// clientMux: mux bridge cho mot client - `current` tra client ma request nay phuc vu.
func (b *Bridge) clientMux(current func() *Client) *http.ServeMux {
	mux := http.NewServeMux()
	health := func(w http.ResponseWriter, r *http.Request) { current().serveBridgeHealth(w, r) }
	commands := func(w http.ResponseWriter, r *http.Request) { current().serveBridgeCommands(w, r) }
	session := func(w http.ResponseWriter, r *http.Request) { current().serveBridgeSession(w, r) }
	cmd := func(w http.ResponseWriter, r *http.Request) { current().serveBridgeCmd(w, r) }
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/health/", health)
	mux.HandleFunc("/commands", commands)
	mux.HandleFunc("/commands/", commands)
	mux.HandleFunc("/session", session)
	mux.HandleFunc("/session/", session)
	mux.HandleFunc("/cmd", cmd)
	mux.HandleFunc("/cmd/", cmd)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			writeErr(w, http.StatusForbidden, "requests with an Origin header are not allowed")
			return
		}
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
	})
	return mux
}

// guardBridge: guard chung cua moi listener bridge (giong bridge LibreOffice).
func (b *Bridge) guardBridge(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
}

// StartBridge mo listener cong chinh; cong ban thi log canh bao va tiep tuc khong co bridge.
func (b *Bridge) StartBridge() {
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(b.opts.BridgePort))
	if err != nil {
		corelog.Info("wps bridge: cannot listen on 127.0.0.1:%d (%s), continuing without the WPS bridge", b.opts.BridgePort, err.Error())
		return
	}
	server := &http.Server{Handler: b.guardBridge(b.BridgeMux()), ReadHeaderTimeout: 10 * time.Second}
	b.mu.Lock()
	b.bridgeListener = listener
	b.bridgePort = listener.Addr().(*net.TCPAddr).Port
	b.bridgeServer = server
	b.mu.Unlock()
	go func() {
		_ = server.Serve(listener)
	}()
	corelog.Info("wps bridge listening on http://127.0.0.1:%d/", b.ActualBridgePort())
}

// openClientListener mo cong rieng cho client phu (cong do he dieu hanh cap). Loi thi client van poll duoc nhung
// khong co phien bridge (khong ghi file session) - log de con tra.
func (b *Bridge) openClientListener(client *Client) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		corelog.Info("wps bridge: cannot open a port for WPS client %s (%s)", client.ID(), err.Error())
		return
	}
	server := &http.Server{
		Handler:           b.guardBridge(b.clientMux(func() *Client { return client })),
		ReadHeaderTimeout: 10 * time.Second,
	}
	port := listener.Addr().(*net.TCPAddr).Port
	b.mu.Lock()
	if b.stopped.Load() || b.clients[client.id] != client {
		// Core dang dung, hoac client da bi bo trong luc mo cong.
		b.mu.Unlock()
		_ = listener.Close()
		return
	}
	client.listener = listener
	client.server = server
	client.port = port
	client.sessionPath = filepath.Join(b.opts.SessionDir, strconv.Itoa(os.Getpid())+"-wps-"+strconv.Itoa(port)+".json")
	b.mu.Unlock()
	go func() {
		_ = server.Serve(listener)
	}()
	corelog.Info("wps bridge: client %s listening on http://127.0.0.1:%d/", client.ID(), port)
}

// ActualBridgePort tra ve cong chinh dang nghe (hoac cong cau hinh khi chua mo).
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

func (c *Client) serveBridgeHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
		return
	}
	writeOK(w, map[string]any{
		"app": c.App(), "family": "wps-linux", "pid": os.Getpid(),
		"port": c.Port(), "version": c.b.bridgeVersion(), "connected": c.Connected(),
	})
}

func (c *Client) serveBridgeCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
		return
	}
	_, cmds, addinVersion, _ := c.Snapshot()
	if !c.everPolled() {
		writeErr(w, http.StatusOK, "WPS add-in has never connected")
		return
	}
	version := addinVersion
	if version == "" {
		version = c.b.bridgeVersion()
	}
	if cmds == nil {
		cmds = []any{}
	}
	writeOK(w, map[string]any{"version": version, "commands": cmds})
}

func (c *Client) serveBridgeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusNotFound, "not found: "+r.URL.Path)
		return
	}
	doc, _, _, _ := c.Snapshot()
	if !c.everPolled() {
		writeErr(w, http.StatusOK, "WPS add-in has never connected")
		return
	}
	writeOK(w, map[string]any{
		"app": c.App(), "family": "wps-linux", "port": c.Port(), "pid": os.Getpid(),
		"document": doc,
	})
}

func (c *Client) serveBridgeCmd(w http.ResponseWriter, r *http.Request) {
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
	if !c.Connected() {
		writeErr(w, http.StatusOK, "WPS add-in is not connected: open a document in WPS to connect the add-in")
		return
	}
	reply, ok := c.Enqueue(action, params)
	if !ok {
		writeErr(w, http.StatusOK, "WPS add-in is not connected: open a document in WPS to connect the add-in")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(reply)
}

// serveBridgeCmd (Bridge): /cmd cua client chinh - duong tuong thich cho test.
func (b *Bridge) serveBridgeCmd(w http.ResponseWriter, r *http.Request) {
	b.Primary().serveBridgeCmd(w, r)
}

// ---- File session ----

func docKey(doc map[string]any) string {
	name, _ := doc["name"].(string)
	full, _ := doc["fullName"].(string)
	return name + "\x00" + full
}

// maybeWriteSession ghi file session khi tai lieu hoac app doi (goi ngay sau poll).
func (c *Client) maybeWriteSession(force bool) {
	if !c.Connected() {
		return
	}
	c.b.mu.Lock()
	key := docKey(c.document)
	app := c.app
	if app == "" {
		app = "wps"
	}
	need := force || !c.wroteSession || key != c.lastDocKey || app != c.lastAppKey
	c.b.mu.Unlock()
	if need {
		c.writeSession()
	}
}

// writeSession ghi file session cua client nguyen tu (temp + rename). Client phu chua co cong thi bo qua.
func (c *Client) writeSession() {
	b := c.b
	b.mu.Lock()
	doc := map[string]any{}
	for key, value := range c.document {
		doc[key] = value
	}
	port := c.portLocked()
	now := time.Now().UTC()
	app := c.app
	if app == "" {
		app = "wps"
	}
	payload := map[string]any{
		"pid": os.Getpid(), "app": app, "family": "wps-linux", "port": port,
		"host": "wps-jsaddon", "version": b.bridgeVersion(),
		"started":       c.started.UTC().Format("2006-01-02T15:04:05.000Z"),
		"lastSeen":      now.Format("2006-01-02T15:04:05.000Z"),
		"lastSeenEpoch": float64(now.UnixNano()) / 1e9,
		"document":      doc["name"],
		"documentPath":  doc["fullName"],
	}
	path := c.sessionPath
	key := docKey(c.document)
	b.mu.Unlock()

	if strings.TrimSpace(b.opts.SessionDir) == "" || path == "" || port == 0 {
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
	c.lastWrite = time.Now()
	c.lastDocKey = key
	c.lastAppKey = app
	c.wroteSession = true
	b.mu.Unlock()
}

// writeSession (Bridge): file session cua client chinh - duong tuong thich cho test.
func (b *Bridge) writeSession() { b.Primary().writeSession() }

// removeSession xoa file session cua client (bo qua loi).
func (c *Client) removeSession() {
	c.b.mu.Lock()
	path := c.sessionPath
	c.wroteSession = false
	c.b.mu.Unlock()
	if path != "" {
		_ = os.Remove(path)
	}
}

// StartSessionLoop bat vong lam moi/xoa file session theo trang thai ket noi cua tung client, va bo client phu da
// mat ket noi (dong cong cua no).
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
				b.tickSessions()
			}
		}
	}()
}

// tickSessions: mot vong cua StartSessionLoop.
func (b *Bridge) tickSessions() {
	b.mu.Lock()
	clients := b.allClientsLocked()
	b.mu.Unlock()
	for _, client := range clients {
		if client.Connected() {
			b.mu.Lock()
			need := !client.wroteSession || time.Since(client.lastWrite) >= SessionRefreshSeconds*time.Second
			b.mu.Unlock()
			if need {
				client.writeSession()
			}
			continue
		}
		if client.sessionWritten() {
			client.removeSession()
		}
		if !client.primary {
			b.dropClient(client)
		}
	}
}

// dropClient bo client phu da mat ket noi: dong cong, tra loi cac lenh chua giao.
func (b *Bridge) dropClient(client *Client) {
	b.mu.Lock()
	if b.clients[client.id] == client {
		delete(b.clients, client.id)
	}
	client.failQueuedLocked()
	server := client.server
	client.server = nil
	client.listener = nil
	b.mu.Unlock()
	if server != nil {
		_ = server.Close()
		corelog.Info("wps bridge: client %s disconnected, port closed", client.ID())
	}
}

func (c *Client) sessionWritten() bool {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	return c.wroteSession
}

// Start khoi dong bridge (listener cong chinh + vong session).
func (b *Bridge) Start() {
	b.StartBridge()
	b.StartSessionLoop()
}

// Stop dung bridge: dong moi listener, xoa moi file session, tra loi moi lenh dang cho.
func (b *Bridge) Stop() {
	b.stopOnce.Do(func() {
		b.stopped.Store(true)
		close(b.stopCh)
		b.mu.Lock()
		servers := []*http.Server{}
		if b.bridgeServer != nil {
			servers = append(servers, b.bridgeServer)
		}
		clients := b.allClientsLocked()
		for _, client := range clients {
			if client.server != nil {
				servers = append(servers, client.server)
			}
		}
		b.mu.Unlock()
		for _, server := range servers {
			// Dong ngay de Core thoat nhanh khi shutdown (khong cho keep-alive).
			_ = server.Close()
		}
		for _, client := range clients {
			client.removeSession()
		}
		// Huy cac /cmd dang cho.
		b.mu.Lock()
		for _, client := range clients {
			for _, job := range client.waiting {
				select {
				case job.wait <- map[string]any{"ok": false, "error": "Core is stopping"}:
				default:
				}
			}
		}
		b.mu.Unlock()
	})
}
