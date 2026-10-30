// Package wpsbridge: Core lam bridge cho JS add-in WPS (docs/architecture/wps-bridge.md muc 2) - phuc vu file
// add-in, kenh /wps/v1/poll + /wps/v1/result tren Core API va bridge WPS tren cong rieng.
//
// Moi tien trinh WPS (Writer, Spreadsheets, Presentation) nap mot ban add-in rieng va poll Core rieng: do la mot
// Client. Moi client la MOT phien bridge voi cong + file session cua no (docs/architecture/wps-bridge.md muc 2.4),
// giong LibreOffice co ba cong cho ba app - nho vay agent, MCP va office_sessions chon dung app theo cong ma khong
// phai biet gi ve WPS. Truoc 31/10/2026 ca ba app chung mot hang doi va mot `app` (= app poll gan nhat): mo Writer
// va ET cung luc thi agent cua pane Writer doc nham bang tinh (docs/audits/wps-linux-live-test-2026-10-28-run2.md 4.5).
package wpsbridge

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultBridgePort: cong bridge WPS mac dinh (docs/architecture/wps-bridge.md 2.3) - cong cua client dau tien.
	DefaultBridgePort = 47861
	// DefaultPollSeconds: poll giu request bao lau khi chua co lenh.
	DefaultPollSeconds = 20
	// DefaultDisconnectSeconds: khong poll trong bao lau thi coi nhu mat ket noi.
	DefaultDisconnectSeconds = 45
	// CmdWaitSeconds: POST /cmd cho ket qua cua add-in toi da bao lau.
	CmdWaitSeconds = 300
	// SessionRefreshSeconds: ghi lai file session moi 25 giay khi van ket noi.
	SessionRefreshSeconds = 25
	// legacyClientID: add-in cu khong gui clientId - moi poll cua no gom vao mot client (hanh vi truoc 31/10/2026).
	legacyClientID = "legacy"
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

// Job la mot lenh Core giao cho add-in thuc thi.
type Job struct {
	ID     string
	Action string
	Params map[string]any
	wait   chan map[string]any
	// sentAt la luc job duoc giao cho add-in (takeJobs dat); zero nghia la chua giao.
	sentAt time.Time
}

// Client: mot tien trinh WPS dang chay add-in (mot app). Moi truong deu do Bridge.mu bao ve.
type Client struct {
	b  *Bridge
	id string
	// claimed: cong chinh da co chu (client chinh tao san luc New, chua ai poll thi claimed=false).
	claimed bool
	primary bool

	// lastPoll la moc song cuoi: luot poll gan nhat hoac ket qua vua tra.
	lastPoll     time.Time
	everPoll     bool
	app          string
	document     map[string]any
	commands     []any
	addinVersion string
	started      time.Time

	queue   []*Job
	waiting map[string]*Job

	// port: cong bridge cua client. Client chinh dung listener cua Bridge (cong cau hinh); client khac co listener rieng.
	port     int
	listener net.Listener
	server   *http.Server

	sessionPath  string
	lastWrite    time.Time
	lastDocKey   string
	lastAppKey   string
	wroteSession bool
}

// Bridge giu trang thai kenh add-in: cac client, cong chinh va listener cua no.
type Bridge struct {
	opts Options

	mu      sync.Mutex
	started time.Time
	// primary: client giu cong cau hinh (47861) va file <pid>-wps.json - luon khac nil (tao san luc New).
	primary *Client
	// clients: client da poll it nhat mot lan, theo id (gom ca primary khi da co chu).
	clients map[string]*Client
	seq     uint64

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
	b := &Bridge{
		opts:       opts,
		started:    time.Now().UTC(),
		clients:    map[string]*Client{},
		stopCh:     make(chan struct{}),
		bridgePort: opts.BridgePort,
	}
	b.primary = &Client{
		b: b, primary: true, app: "wps", waiting: map[string]*Job{}, started: b.started,
		sessionPath: filepath.Join(opts.SessionDir, strconv.Itoa(os.Getpid())+"-wps.json"),
	}
	return b
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

// ---- Chon client ----

// Resolve chon client cho mot yeu cau tu pane/khung chat mini: dung clientId neu con, khong thi client dang ket noi
// cua app do poll gan nhat, cuoi cung la client chinh. Khong bao gio nil.
func (b *Bridge) Resolve(clientID, app string) *Client {
	b.mu.Lock()
	defer b.mu.Unlock()
	clientID = strings.TrimSpace(clientID)
	if clientID != "" {
		if client := b.clients[clientID]; client != nil {
			return client
		}
	}
	if validApp(app) {
		now := time.Now()
		var best *Client
		for _, client := range b.clients {
			if client.app != app || !client.connectedLocked(now) {
				continue
			}
			if best == nil || client.lastPoll.After(best.lastPoll) {
				best = client
			}
		}
		if best != nil {
			return best
		}
	}
	return b.primary
}

// Primary tra ve client giu cong chinh (duong tuong thich cho cac goi khong biet client).
func (b *Bridge) Primary() *Client {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.primary
}

func validApp(app string) bool {
	return app == "wps" || app == "et" || app == "wpp"
}

// ---- Trang thai cua mot client (moi ham tu khoa Bridge.mu) ----

// busyLocked tra ve true khi dang co job da giao cho add-in ma chua qua CmdWaitSeconds.
// Add-in chay don luong nen luc chay lenh dai no khong poll: coi nhu van ket noi.
// Chi goi khi dang giu b.mu; het han thi duong het gio cua Enqueue goi forget nen tu het.
func (c *Client) busyLocked(now time.Time) bool {
	limit := time.Duration(c.b.opts.CmdWaitSeconds) * time.Second
	for _, job := range c.waiting {
		if job.sentAt.IsZero() {
			continue
		}
		if now.Sub(job.sentAt) < limit {
			return true
		}
	}
	return false
}

func (c *Client) connectedLocked(now time.Time) bool {
	if c.busyLocked(now) {
		return true
	}
	return c.everPoll && now.Sub(c.lastPoll) <= time.Duration(c.b.opts.DisconnectSeconds)*time.Second
}

// Connected cho biet co poll trong DisconnectSeconds giay gan nhat, hoac dang co lenh chay.
func (c *Client) Connected() bool {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	return c.connectedLocked(time.Now())
}

// ID tra ve id cua client ("" khi la cong chinh chua co chu).
func (c *Client) ID() string {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	return c.id
}

// App tra ve app cua client theo poll cua add-in ("wps" khi chua biet).
func (c *Client) App() string {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	if c.app == "" {
		return "wps"
	}
	return c.app
}

// Port tra ve cong bridge cua client (cong chinh: cong dang nghe hoac cong cau hinh).
func (c *Client) Port() int {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	return c.portLocked()
}

func (c *Client) portLocked() int {
	if c.primary {
		if c.b.bridgePort != 0 {
			return c.b.bridgePort
		}
		return c.b.opts.BridgePort
	}
	return c.port
}

// Snapshot tra ve tai lieu + catalog moi nhat (cho /commands, /session).
func (c *Client) Snapshot() (map[string]any, []any, string, bool) {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	doc := map[string]any{}
	for key, value := range c.document {
		doc[key] = value
	}
	cmds := append([]any{}, c.commands...)
	return doc, cmds, c.addinVersion, c.connectedLocked(time.Now())
}

func (c *Client) everPolled() bool {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	return c.everPoll
}

// statusLocked: anh chup suc khoe cua mot client (nut "Trang thai" tren ribbon WPS - docs/architecture/ribbon-ui.md muc 2).
func (c *Client) statusLocked(now time.Time) map[string]any {
	running := c.busyLocked(now)
	doc := map[string]any{}
	for key, value := range c.document {
		doc[key] = value
	}
	app := c.app
	if app == "" {
		app = "wps"
	}
	status := map[string]any{
		"clientId":     c.id,
		"connected":    running || (c.everPoll && now.Sub(c.lastPoll) <= time.Duration(c.b.opts.DisconnectSeconds)*time.Second),
		"everPoll":     c.everPoll,
		"app":          app,
		"document":     doc,
		"addinVersion": c.addinVersion,
		"running":      running,
		"queued":       len(c.queue),
		"waiting":      len(c.waiting),
		"bridgePort":   c.portLocked(),
		"sessionPath":  c.sessionPath,
		// Nguong dang ap dung: de nguoi dung tu doc ra "45 giay khong poll thi coi nhu mat ket noi"
		// thay vi doan tu mot con so "lastPollSeconds" tran.
		"pollSeconds":       c.b.opts.PollSeconds,
		"disconnectSeconds": c.b.opts.DisconnectSeconds,
	}
	if c.everPoll {
		status["lastPollSeconds"] = math.Round(now.Sub(c.lastPoll).Seconds()*10) / 10
	}
	// Uptime cua PHIEN add-in (tu luc client xuat hien), khong phai uptime cua Core.
	status["uptimeSeconds"] = math.Round(now.Sub(c.started).Seconds()*10) / 10
	return status
}

// Status: anh chup cua client nay, kem danh sach moi client dang biet (WPS co the mo ca ba app cung luc).
//
// Snapshot() khong du: no tra tai lieu + catalog, nhung khong noi duoc add-in da poll BAO GIO CHUA va
// lan cuoi la khi nao - ma do chinh la ba cau nguoi dung hoi khi bam "Trang thai" ("add-in co tra loi
// khong", "dang mo tai lieu nao", "co phai dang chay lenh khong"). Bridge co cong RIENG nen pane khong
// tu hoi duoc, phai qua Core.
func (c *Client) Status() map[string]any {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	now := time.Now()
	status := c.statusLocked(now)
	status["clients"] = c.b.clientsLocked(now)
	return status
}

// clientsLocked: tom tat moi client dang biet, sap theo cong (on dinh cho giao dien va test).
func (b *Bridge) clientsLocked(now time.Time) []any {
	list := []*Client{}
	seen := map[*Client]bool{}
	for _, client := range b.clients {
		list = append(list, client)
		seen[client] = true
	}
	if !seen[b.primary] && b.primary.everPoll {
		list = append(list, b.primary)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].portLocked() < list[j].portLocked() })
	out := make([]any, 0, len(list))
	for _, client := range list {
		name, _ := client.document["name"].(string)
		out = append(out, map[string]any{
			"clientId": client.id, "app": client.app, "port": client.portLocked(), "primary": client.primary,
			"connected": client.connectedLocked(now), "document": name,
		})
	}
	return out
}

// ---- Duong tuong thich: cac ham cu cua Bridge lam tren client chinh ----

// Connected: client chinh con ket noi khong.
func (b *Bridge) Connected() bool { return b.Primary().Connected() }

// Snapshot: tai lieu + catalog cua client chinh.
func (b *Bridge) Snapshot() (map[string]any, []any, string, bool) { return b.Primary().Snapshot() }

// Status: trang thai cua client chinh kem danh sach client.
func (b *Bridge) Status() map[string]any { return b.Primary().Status() }

// Enqueue: gui lenh cho client chinh.
func (b *Bridge) Enqueue(action string, params map[string]any) (map[string]any, bool) {
	return b.Primary().Enqueue(action, params)
}

// ---- Hang doi lenh ----

// newJobID sinh id duy nhat cho job (duy nhat tren moi client: /result tim job theo id tren moi client).
func (b *Bridge) newJobID() string {
	seq := atomic.AddUint64(&b.seq, 1)
	var random [8]byte
	_, _ = rand.Read(random[:])
	return fmt.Sprintf("wps-%d-%d-%s", os.Getpid(), seq, hex.EncodeToString(random[:]))
}

// Enqueue dua lenh vao hang doi cua client va cho ket qua toi da CmdWaitSeconds giay (theo cau hinh).
func (c *Client) Enqueue(action string, params map[string]any) (map[string]any, bool) {
	if !c.Connected() {
		return nil, false
	}
	b := c.b
	job := &Job{ID: b.newJobID(), Action: action, Params: params, wait: make(chan map[string]any, 1)}
	b.mu.Lock()
	c.queue = append(c.queue, job)
	c.waiting[job.ID] = job
	cmdWait := b.opts.CmdWaitSeconds
	b.mu.Unlock()

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
			c.forget(job)
			return map[string]any{"ok": false, "error": fmt.Sprintf("WPS add-in did not answer within %ds; the command may have been applied - check the document before retrying", cmdWait)}, true
		case <-ticker.C:
			b.mu.Lock()
			sent := !job.sentAt.IsZero()
			b.mu.Unlock()
			if sent {
				continue
			}
			if !c.Connected() {
				c.forget(job)
				return map[string]any{"ok": false, "error": "WPS add-in disconnected before the command was sent (WPS closed?) - it was not sent, safe to retry"}, true
			}
		case <-b.stopCh:
			return map[string]any{"ok": false, "error": "Core is stopping"}, true
		}
	}
}

// forget bo job khoi hang doi va danh sach cho ket qua (het gio / mat ket noi).
func (c *Client) forget(job *Job) {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	delete(c.waiting, job.ID)
	for i, item := range c.queue {
		if item == job {
			c.queue = append(c.queue[:i], c.queue[i+1:]...)
			break
		}
	}
}

// takeJobs lay toan bo job dang doi cho mot poll, danh dau gio giao de biet lenh dang chay.
func (c *Client) takeJobs() []*Job {
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	jobs := c.queue
	c.queue = nil
	now := time.Now()
	for _, job := range jobs {
		job.sentAt = now
	}
	return jobs
}

// takeJobs (Bridge): job cua client chinh - duong tuong thich cho test.
func (b *Bridge) takeJobs() []*Job { return b.Primary().takeJobs() }

// failQueuedLocked tra loi "chua gui duoc" cho moi job CHUA giao cua client (client bi bo hoac nhuong cong chinh).
// Job da giao thi giu: ket qua co the van toi qua /result, khong thi het gio theo CmdWaitSeconds.
func (c *Client) failQueuedLocked() {
	for _, job := range c.queue {
		delete(c.waiting, job.ID)
		select {
		case job.wait <- map[string]any{"ok": false, "error": "WPS add-in disconnected before the command was sent (WPS closed?) - it was not sent, safe to retry"}:
		default:
		}
	}
	c.queue = nil
}

// Complete ghi nhan ket qua cua add-in; id la thi false. Id job duy nhat tren moi client nen tim o tat ca.
func (b *Bridge) Complete(id string, reply map[string]any) bool {
	b.mu.Lock()
	var job *Job
	for _, client := range b.allClientsLocked() {
		if found, ok := client.waiting[id]; ok {
			job = found
			delete(client.waiting, id)
			// Add-in vua tra ket qua = bang chung con song moi hon lan poll cuoi. Add-in chay don
			// luong nen trong luc chay lenh dai no khong poll duoc, va Complete vua bo job khoi
			// waiting (busyLocked het hieu luc ngay): khong lam moi moc nay thi mot lenh dai hon
			// DisconnectSeconds lam nguoi goi khac bi bao "mat ket noi" ngay sau khi add-in vua
			// chung minh no dang song.
			client.lastPoll = time.Now()
			break
		}
	}
	b.mu.Unlock()
	if job == nil {
		return false
	}
	select {
	case job.wait <- reply:
	default:
	}
	return true
}

// allClientsLocked: moi client dang biet, gom client chinh (ke ca khi chua co chu).
func (b *Bridge) allClientsLocked() []*Client {
	list := []*Client{b.primary}
	for _, client := range b.clients {
		if client != b.primary {
			list = append(list, client)
		}
	}
	return list
}

// ---- Poll ----

// recordPoll luu heartbeat + catalog + tai lieu tu body poll vao client cua no (tao client moi khi can).
// Tra ve client va true khi client nay vua duoc tao va can listener rieng (khong phai client chinh).
func (b *Bridge) recordPoll(body map[string]any) (*Client, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	id := strings.TrimSpace(textOf(body["clientId"]))
	if id == "" {
		id = legacyClientID
	}
	client := b.clients[id]
	needsListener := false
	if client == nil {
		client, needsListener = b.newClientLocked(id, now)
	}
	client.lastPoll = now
	client.everPoll = true
	// App do add-in bao: chi nhan "wps" | "et" | "wpp"; thieu hoac la thi giu app dang biet.
	if raw, ok := body["app"].(string); ok && validApp(raw) {
		client.app = raw
	}
	if doc, ok := body["document"].(map[string]any); ok {
		next := map[string]any{}
		for key, value := range doc {
			next[key] = value
		}
		client.document = next
	}
	if cmds, ok := body["commands"].([]any); ok {
		client.commands = append([]any{}, cmds...)
	} else {
		client.commands = nil
	}
	if version, ok := body["version"].(string); ok {
		client.addinVersion = version
	}
	return client, needsListener
}

// newClientLocked: client moi nhan cong chinh khi cong chua co chu hoac chu cu da mat ket noi (giu cong 47861 va
// <pid>-wps.json cho cau hinh cu); khong thi la client phu, can listener rieng.
func (b *Bridge) newClientLocked(id string, now time.Time) (*Client, bool) {
	primary := b.primary
	if !primary.claimed || !primary.connectedLocked(now) {
		if primary.claimed {
			delete(b.clients, primary.id)
		}
		// Lenh chua giao cua chu cu khong duoc roi vao tay app moi (ET nhan lenh viet cho Writer).
		primary.failQueuedLocked()
		primary.id = id
		primary.claimed = true
		primary.app = "wps"
		primary.document = nil
		primary.commands = nil
		primary.addinVersion = ""
		primary.started = now
		b.clients[id] = primary
		return primary, false
	}
	client := &Client{b: b, id: id, app: "wps", waiting: map[string]*Job{}, started: now}
	b.clients[id] = client
	return client, true
}

func textOf(value any) string {
	text, _ := value.(string)
	return text
}
