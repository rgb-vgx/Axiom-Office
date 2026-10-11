package wpsbridge

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testBridge(t *testing.T, mutate func(*Options)) *Bridge {
	t.Helper()
	opts := Options{
		AddinDir: t.TempDir(), BridgePort: 0, PollSeconds: 1, DisconnectSeconds: 2,
		SessionDir: t.TempDir(), Token: "tok", Version: "9.9.9", CorePort: 47840,
	}
	if mutate != nil {
		mutate(&opts)
	}
	return New(opts)
}

// Quyet dinh xac thuc kenh add-in: Origin dung cua Core thi qua, Origin la thi 403, khong Origin
// thi doi token.
func TestDecideWPSAuth(t *testing.T) {
	own := OwnOrigin(47840)
	if code, ok := DecideWPSAuth(own, "", "tok", 47840, "POST", ""); !ok || code != 0 {
		t.Fatalf("own origin -> qua, duoc (%d, %v)", code, ok)
	}
	if code, ok := DecideWPSAuth("http://evil.example", "", "tok", 47840, "POST", ""); ok || code != 403 {
		t.Fatalf("origin la -> 403, duoc (%d, %v)", code, ok)
	}
	if code, ok := DecideWPSAuth("", "tok", "tok", 47840, "POST", ""); !ok || code != 0 {
		t.Fatalf("khong origin + dung token -> qua, duoc (%d, %v)", code, ok)
	}
	if code, ok := DecideWPSAuth("", "sai", "tok", 47840, "POST", ""); ok || code != 401 {
		t.Fatalf("khong origin + sai token -> 401, duoc (%d, %v)", code, ok)
	}
	if code, ok := DecideWPSAuth("", "", "tok", 47840, "POST", ""); ok || code != 401 {
		t.Fatalf("khong origin + thieu token -> 401, duoc (%d, %v)", code, ok)
	}
}

// Trinh duyet khong gui Origin cho GET cung origin nen EventSource cua pane bi 401 (11/10/2026):
// GET/HEAD khong Origin ma co Sec-Fetch-Site: same-origin thi cho qua; POST van doi token.
func TestDecideWPSAuthSecFetchSite(t *testing.T) {
	if code, ok := DecideWPSAuth("", "", "tok", 47840, "GET", "same-origin"); !ok || code != 0 {
		t.Fatalf("GET khong origin + same-origin -> qua, duoc (%d, %v)", code, ok)
	}
	if code, ok := DecideWPSAuth("", "", "tok", 47840, "HEAD", "same-origin"); !ok || code != 0 {
		t.Fatalf("HEAD khong origin + same-origin -> qua, duoc (%d, %v)", code, ok)
	}
	if code, ok := DecideWPSAuth("", "", "tok", 47840, "POST", "same-origin"); ok || code != 401 {
		t.Fatalf("POST khong origin chi co same-origin -> 401, duoc (%d, %v)", code, ok)
	}
	for _, value := range []string{"cross-site", "same-site", "none", ""} {
		if code, ok := DecideWPSAuth("", "", "tok", 47840, "GET", value); ok || code != 401 {
			t.Fatalf("GET khong origin + Sec-Fetch-Site %q -> 401, duoc (%d, %v)", value, code, ok)
		}
	}
	if code, ok := DecideWPSAuth("http://evil.example", "", "tok", 47840, "GET", "same-origin"); ok || code != 403 {
		t.Fatalf("Origin la du co same-origin van 403, duoc (%d, %v)", code, ok)
	}
}

// Guard bridge WPS: bat ky Origin nao cung 403 (ke ca origin cua Core), /health khong can token.
func TestDecideBridgeAuth(t *testing.T) {
	if code, ok := DecideBridgeAuth("http://127.0.0.1:47840", "tok", "tok", "/cmd", "POST"); ok || code != 403 {
		t.Fatalf("bridge co Origin -> 403, duoc (%d, %v)", code, ok)
	}
	if code, ok := DecideBridgeAuth("", "", "tok", "/health", "GET"); !ok || code != 0 {
		t.Fatalf("bridge /health khong token -> qua, duoc (%d, %v)", code, ok)
	}
	if code, ok := DecideBridgeAuth("", "", "tok", "/cmd", "POST"); ok || code != 401 {
		t.Fatalf("bridge /cmd thieu token -> 401, duoc (%d, %v)", code, ok)
	}
}

// Lam sach duong dan add-in: chan .., ma hoa %2e/%2f va duong dan tuyet doi.
func TestCleanAddinRel(t *testing.T) {
	for _, raw := range []string{
		"/wps/addin/../secret.txt", "/wps/addin/%2e%2e/secret.txt", "/wps/addin/..%2fsecret.txt",
		"/wps/addin/%2Fetc/passwd", "/wps/addin//etc/passwd", "/wps/addin/",
	} {
		if _, ok := CleanAddinRel(raw, "/wps/addin/"); ok {
			t.Errorf("%s phai bi chan", raw)
		}
	}
	rel, ok := CleanAddinRel("/wps/addin/sub/main.js", "/wps/addin/")
	if !ok || rel != "sub/main.js" {
		t.Fatalf("duong dan hop le bi chan: %q %v", rel, ok)
	}
}

// Hang doi: poll lay job da enqueue, result danh thuc /cmd dang cho.
func TestEnqueueCompleteRoundtrip(t *testing.T) {
	b := testBridge(t, nil)
	b.recordPoll(map[string]any{"document": map[string]any{"name": "a.docx"}, "commands": []any{}})
	replyCh := make(chan map[string]any, 1)
	go func() {
		reply, _ := b.Enqueue("writer.getText", map[string]any{})
		replyCh <- reply
	}()
	deadline := time.Now().Add(3 * time.Second)
	var jobs []*Job
	for time.Now().Before(deadline) {
		if jobs = b.takeJobs(); len(jobs) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(jobs) != 1 || jobs[0].Action != "writer.getText" {
		t.Fatalf("jobs = %+v", jobs)
	}
	answer := map[string]any{"ok": true, "result": map[string]any{"n": 1}}
	if !b.Complete(jobs[0].ID, answer) {
		t.Fatal("Complete id vua lay phai true")
	}
	select {
	case reply := <-replyCh:
		if reply["ok"] != true {
			t.Fatalf("reply = %v", reply)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Enqueue khong nhan duoc ket qua")
	}
	if b.Complete(jobs[0].ID, answer) {
		t.Fatal("Complete lan hai phai false (id da xong)")
	}
	if b.Complete("khong-co", answer) {
		t.Fatal("Complete id la phai false")
	}
}

// Mat ket noi: qua DisconnectSeconds khong poll thi Connected false.
func TestConnectedExpires(t *testing.T) {
	b := testBridge(t, func(o *Options) { o.DisconnectSeconds = 1 })
	b.recordPoll(map[string]any{})
	if !b.Connected() {
		t.Fatal("vua poll phai connected")
	}
	time.Sleep(1200 * time.Millisecond)
	if b.Connected() {
		t.Fatal("qua han poll phai mat ket noi")
	}
}

// Phuc vu file add-in: dung Content-Type, chan thoat thu muc, chan Origin la.
func TestServeAddin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := testBridge(t, func(o *Options) { o.AddinDir = dir })
	call := func(uri string, headers map[string]string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:47840"+uri, nil)
		request.RequestURI = uri
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		recorder := httptest.NewRecorder()
		b.ServeAddin(recorder, request)
		return recorder
	}
	got := call("/wps/addin/index.html", nil)
	if got.Code != 200 || !strings.Contains(got.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("index.html -> 200 text/html, duoc %d %q", got.Code, got.Header().Get("Content-Type"))
	}
	if got := call("/wps/addin/../secret.txt", nil); got.Code != 404 {
		t.Fatalf("thoat thu muc -> 404, duoc %d", got.Code)
	}
	if got := call("/wps/addin/index.html", map[string]string{"Origin": "http://evil.example"}); got.Code != 403 {
		t.Fatalf("Origin la -> 403, duoc %d", got.Code)
	}
}

// Add-in ngung poll (nguoi dung dong WPS) khi /cmd dang cho: bao loi ngay khi mat ket noi, khong treo 300s.
func TestEnqueueFailsFastWhenAddinDisconnects(t *testing.T) {
	bridge := testBridge(t, nil) // DisconnectSeconds: 2
	bridge.recordPoll(map[string]any{})
	started := time.Now()
	reply, ok := bridge.Enqueue("writer.getText", map[string]any{})
	waited := time.Since(started)
	if !ok || reply["ok"] != false || !strings.Contains(reply["error"].(string), "disconnected") {
		t.Fatalf("mat ket noi -> ok:false 'disconnected', duoc %v %v", ok, reply)
	}
	if waited > 5*time.Second {
		t.Fatalf("phai bao loi trong vai giay sau khi mat ket noi, cho %v", waited)
	}
	if len(bridge.waiting) != 0 || len(bridge.queue) != 0 {
		t.Fatalf("job phai bi bo khoi hang doi: waiting=%d queue=%d", len(bridge.waiting), len(bridge.queue))
	}
}
