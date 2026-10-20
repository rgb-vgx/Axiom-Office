// Package wpsbridge: Core lam bridge cho JS add-in WPS (docs/architecture/wps-bridge.md muc 2) - phuc vu file
// add-in, kenh /wps/v1/poll + /wps/v1/result tren Core API va bridge WPS tren cong rieng.
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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultBridgePort: cong bridge WPS mac dinh (docs/architecture/wps-bridge.md 2.3).
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

	mu sync.Mutex
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

// Status: anh chup suc khoe kenh add-in, cho nut "Trang thai" tren ribbon WPS (docs/architecture/ribbon-ui.md muc 2).
//
// Snapshot() khong du: no tra tai lieu + catalog, nhung khong noi duoc add-in da poll BAO GIO CHUA va
// lan cuoi la khi nao - ma do chinh la ba cau nguoi dung hoi khi bam "Trang thai" ("add-in co tra loi
// khong", "dang mo tai lieu nao", "co phai dang chay lenh khong"). Bridge co cong RIENG nen pane khong
// tu hoi duoc, phai qua Core.
func (b *Bridge) Status() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	running := b.busyLocked(now)
	connected := running || (b.everPoll && now.Sub(b.lastPoll) <= time.Duration(b.opts.DisconnectSeconds)*time.Second)

	doc := map[string]any{}
	for key, value := range b.document {
		doc[key] = value
	}
	status := map[string]any{
		"connected":    connected,
		"everPoll":     b.everPoll,
		"app":          b.app,
		"document":     doc,
		"addinVersion": b.addinVersion,
		"running":      running,
		"queued":       len(b.queue),
		"waiting":      len(b.waiting),
		"bridgePort":   b.bridgePort,
		"sessionPath":  b.sessionPath,
		// Nguong dang ap dung: de nguoi dung tu doc ra "45 giay khong poll thi coi nhu mat ket noi"
		// thay vi doan tu mot con so "lastPollSeconds" tran.
		"pollSeconds":       b.opts.PollSeconds,
		"disconnectSeconds": b.opts.DisconnectSeconds,
	}
	if b.everPoll {
		status["lastPollSeconds"] = math.Round(now.Sub(b.lastPoll).Seconds()*10) / 10
	}
	// Uptime cua PHIEN add-in (tu luc bridge duoc tao), khong phai uptime cua Core.
	status["uptimeSeconds"] = math.Round(now.Sub(b.started).Seconds()*10) / 10
	return status
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
		// Add-in vua tra ket qua = bang chung con song moi hon lan poll cuoi. Add-in chay don
		// luong nen trong luc chay lenh dai no khong poll duoc, va Complete vua bo job khoi
		// waiting (busyLocked het hieu luc ngay): khong lam moi moc nay thi mot lenh dai hon
		// DisconnectSeconds lam nguoi goi khac bi bao "mat ket noi" ngay sau khi add-in vua
		// chung minh no dang song.
		b.lastPoll = time.Now()
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
