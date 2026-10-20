package wpsbridge

import (
	"encoding/json"
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
	if got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("index.html -> Cache-Control no-store, duoc %q", got.Header().Get("Cache-Control"))
	}
	if got := call("/wps/addin/../secret.txt", nil); got.Code != 404 {
		t.Fatalf("thoat thu muc -> 404, duoc %d", got.Code)
	} else if got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("thoat thu muc -> Cache-Control no-store, duoc %q", got.Header().Get("Cache-Control"))
	}
	if got := call("/wps/addin/index.html", map[string]string{"Origin": "http://evil.example"}); got.Code != 403 {
		t.Fatalf("Origin la -> 403, duoc %d", got.Code)
	} else if got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Origin la -> Cache-Control no-store, duoc %q", got.Header().Get("Cache-Control"))
	}
}

// Add-in ngung poll (nguoi dung dong WPS) khi /cmd dang cho, va lenh CHUA duoc giao cho add-in: bao loi
// ngay khi mat ket noi, khong treo 300s. Cau loi phai noi ro lenh chua gui duoc, de agent biet thu lai
// la an toan (khac han truong hop lenh da chay - xem TestEnqueueInFlightTimeoutWarnsBeforeRetrying).
func TestEnqueueFailsFastWhenAddinDisconnects(t *testing.T) {
	bridge := testBridge(t, nil) // DisconnectSeconds: 2
	bridge.recordPoll(map[string]any{})
	started := time.Now()
	reply, ok := bridge.Enqueue("writer.getText", map[string]any{})
	waited := time.Since(started)
	if !ok || reply["ok"] != false || !strings.Contains(reply["error"].(string), "disconnected") {
		t.Fatalf("mat ket noi -> ok:false 'disconnected', duoc %v %v", ok, reply)
	}
	message, _ := reply["error"].(string)
	if !strings.Contains(message, "was not sent") || !strings.Contains(message, "safe to retry") {
		t.Fatalf("lenh chua gui duoc thi phai noi ro de agent yen tam thu lai, duoc %q", message)
	}
	if waited > 5*time.Second {
		t.Fatalf("phai bao loi trong vai giay sau khi mat ket noi, cho %v", waited)
	}
	if len(bridge.waiting) != 0 || len(bridge.queue) != 0 {
		t.Fatalf("job phai bi bo khoi hang doi: waiting=%d queue=%d", len(bridge.waiting), len(bridge.queue))
	}
}

// Lenh DA giao cho add-in ma chua co ket qua = add-in DANG chay lenh do. Add-in chay don luong nen no
// khong poll trong luc chay, vi vay khong duoc ap dung luat DisconnectSeconds cho no: lenh dai
// (et.importCsv file lon, et.formatRange ca bang, exportPdf) ma bi cat giua chung thi ket qua bi bo -
// trong khi lenh co the da ghi vao tai lieu - va agent nhan "mat ket noi" roi thu lai => ghi hai lan.
func TestEnqueueKeepsWaitingWhileCommandIsInFlight(t *testing.T) {
	b := testBridge(t, func(o *Options) { o.DisconnectSeconds = 1; o.CmdWaitSeconds = 5 })
	b.recordPoll(map[string]any{})
	replyCh := make(chan map[string]any, 1)
	go func() {
		reply, _ := b.Enqueue("et.importCsv", map[string]any{"path": "/tmp/ban-hang.csv"})
		replyCh <- reply
	}()
	jobs := takeJobsWithin(t, b, 3*time.Second)
	if len(jobs) != 1 {
		t.Fatalf("khong lay duoc job: %+v", jobs)
	}
	// Add-in dang chay lenh: khong poll nua, qua ca DisconnectSeconds (1s).
	time.Sleep(1500 * time.Millisecond)
	if !b.Connected() {
		t.Fatal("dang chay lenh thi phai coi la con ket noi (add-in khong the vua chay vua poll)")
	}
	if !b.Complete(jobs[0].ID, map[string]any{"ok": true, "result": map[string]any{"rows": 1000}}) {
		t.Fatal("Complete phai true: job van dang cho, chua bi bo")
	}
	select {
	case reply := <-replyCh:
		if reply["ok"] != true {
			t.Fatalf("ket qua cua lenh dai phai toi duoc nguoi goi, duoc %v", reply)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Enqueue khong nhan duoc ket qua cua lenh dai")
	}
}

// Lenh dang chay ma qua CmdWaitSeconds khong co ket qua: het gio that, va cau loi phai canh bao lenh
// CO THE da duoc ghi de agent kiem tra truoc khi thu lai - khac han "chua gui duoc".
func TestEnqueueInFlightTimeoutWarnsBeforeRetrying(t *testing.T) {
	b := testBridge(t, func(o *Options) { o.DisconnectSeconds = 1; o.CmdWaitSeconds = 2 })
	b.recordPoll(map[string]any{})
	started := time.Now()
	replyCh := make(chan map[string]any, 1)
	go func() {
		reply, _ := b.Enqueue("et.formatRange", map[string]any{})
		replyCh <- reply
	}()
	if jobs := takeJobsWithin(t, b, 3*time.Second); len(jobs) != 1 {
		t.Fatalf("khong lay duoc job: %+v", jobs)
	}
	var reply map[string]any
	select {
	case reply = <-replyCh:
	case <-time.After(10 * time.Second):
		t.Fatal("Enqueue phai het gio theo CmdWaitSeconds, khong duoc treo mai")
	}
	if waited := time.Since(started); waited < 1500*time.Millisecond {
		t.Fatalf("dang chay lenh thi khong duoc cat som theo DisconnectSeconds (1s), chi cho %v", waited)
	}
	message, _ := reply["error"].(string)
	if reply["ok"] != false || !strings.Contains(message, "did not answer") {
		t.Fatalf("het gio khi dang chay -> ok:false 'did not answer', duoc %v", reply)
	}
	if !strings.Contains(message, "may have been applied") {
		t.Fatalf("phai canh bao lenh co the da duoc ghi (dung thu lai mu), duoc %q", message)
	}
	if strings.Contains(message, "was not sent") {
		t.Fatalf("lenh DA gui cho add-in thi khong duoc bao la chua gui: %q", message)
	}
	// Job da bi bo thi khong con "dang chay": vong session moi don duoc file session.
	if b.Connected() {
		t.Fatal("het gio roi thi khong con lenh nao dang chay")
	}
}

// takeJobsWithin cho job dau tien duoc dua cho add-in trong han cho truoc.
func takeJobsWithin(t *testing.T, b *Bridge, limit time.Duration) []*Job {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if jobs := b.takeJobs(); len(jobs) > 0 {
			return jobs
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// T6 (docs/architecture/wps-bridge.md muc 5): WPS nap CUNG mot add-in trong ca ba app (publish.xml type="wps,et,wpp"),
// nen app cua phien phai theo app ma add-in bao trong poll, khong duoc hard-code "wps".
// Core loc lenh theo session.App (tools/office_action.go), sai app thi et.*/wpp.* bi bo im lang.
func TestAppFollowsAddinPoll(t *testing.T) {
	b := testBridge(t, nil)

	getJSON := func(handler http.HandlerFunc, uri string) map[string]any {
		t.Helper()
		recorder := httptest.NewRecorder()
		handler(recorder, httptest.NewRequest(http.MethodGet, uri, nil))
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: body khong phai JSON: %s", uri, recorder.Body.String())
		}
		result, _ := body["result"].(map[string]any)
		if result == nil {
			t.Fatalf("%s: thieu result: %s", uri, recorder.Body.String())
		}
		return result
	}

	// Chua poll lan nao: khong biet app nao -> mac dinh "wps" (Writer) de khong doi hanh vi cu.
	if got := getJSON(b.serveBridgeHealth, "/health"); got["app"] != "wps" {
		t.Fatalf("chua poll -> app 'wps', duoc %v", got["app"])
	}

	b.recordPoll(map[string]any{"app": "et", "document": map[string]any{"name": "ban-hang.xlsx"}})
	if got := getJSON(b.serveBridgeHealth, "/health"); got["app"] != "et" {
		t.Fatalf("/health phai bao app cua add-in: %v", got["app"])
	}
	if got := getJSON(b.serveBridgeSession, "/session"); got["app"] != "et" {
		t.Fatalf("/session phai bao app cua add-in: %v", got["app"])
	}
	if got := getJSON(b.serveBridgeSession, "/session"); got["family"] != "wps-linux" {
		t.Fatalf("family van la wps-linux: %v", got["family"])
	}

	b.writeSession()
	raw, err := os.ReadFile(b.sessionPath)
	if err != nil {
		t.Fatalf("doc file session: %v", err)
	}
	var session map[string]any
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatalf("file session khong phai JSON: %v", err)
	}
	if session["app"] != "et" || session["document"] != "ban-hang.xlsx" {
		t.Fatalf("file session phai co app cua add-in: %v", session)
	}

	// Add-in cu (khong gui app) va gia tri la: giu nguyen app dang biet, khong doan bua.
	b.recordPoll(map[string]any{"document": map[string]any{"name": "khac.xlsx"}})
	if got := getJSON(b.serveBridgeHealth, "/health"); got["app"] != "et" {
		t.Fatalf("poll khong co app -> giu app cu: %v", got["app"])
	}
	b.recordPoll(map[string]any{"app": "khong-phai-app", "document": map[string]any{"name": "khac.xlsx"}})
	if got := getJSON(b.serveBridgeHealth, "/health"); got["app"] != "et" {
		t.Fatalf("app la -> giu app cu: %v", got["app"])
	}

	// Doi sang WPP khi nguoi dung lam viec trong Presentation.
	b.recordPoll(map[string]any{"app": "wpp", "document": map[string]any{"name": "bao-cao.pptx"}})
	if got := getJSON(b.serveBridgeSession, "/session"); got["app"] != "wpp" {
		t.Fatalf("doi app theo poll moi: %v", got["app"])
	}
}

// Add-in vua tra ket qua la bang chung con song: lenh dai hon DisconnectSeconds lam lastPoll cu di,
// Complete xoa job khoi waiting (busyLocked het hieu luc ngay) nen phai lam moi moc song, neu khong
// nguoi goi ke tiep bi bao "mat ket noi" ngay sau khi add-in vua chung minh no dang song.
func TestCompleteRefreshesLiveness(t *testing.T) {
	b := testBridge(t, func(o *Options) { o.DisconnectSeconds = 1; o.CmdWaitSeconds = 5 })
	b.recordPoll(map[string]any{})
	replyCh := make(chan map[string]any, 1)
	go func() {
		reply, _ := b.Enqueue("writer.getText", map[string]any{})
		replyCh <- reply
	}()
	jobs := takeJobsWithin(t, b, 3*time.Second)
	if len(jobs) != 1 {
		t.Fatalf("khong lay duoc job: %+v", jobs)
	}
	// Lenh dang chay: qua DisconnectSeconds (1s) nhung van connected nho busyLocked.
	time.Sleep(1500 * time.Millisecond)
	if !b.Connected() {
		t.Fatal("dang chay lenh thi phai coi la con ket noi (add-in khong the vua chay vua poll)")
	}
	if !b.Complete(jobs[0].ID, map[string]any{"ok": true, "result": map[string]any{"n": 1}}) {
		t.Fatal("Complete lan dau phai true")
	}
	// Ngay sau Complete, khong poll them lan nao: van phai connected.
	if !b.Connected() {
		t.Fatal("add-in vua tra ket qua thi phai coi la con ket noi, khong duoc bao mat ket noi")
	}
	if _, _, _, connected := b.Snapshot(); !connected {
		t.Fatal("Snapshot sau Complete phai connected (duong /commands, /session dung chung cong thuc)")
	}
	select {
	case <-replyCh:
	case <-time.After(3 * time.Second):
		t.Fatal("Enqueue khong nhan duoc ket qua")
	}
	// Chot canh: id la khong phai bang chung con song, khong duoc lam moi moc song.
	b2 := testBridge(t, func(o *Options) { o.DisconnectSeconds = 1; o.CmdWaitSeconds = 5 })
	b2.recordPoll(map[string]any{})
	time.Sleep(1500 * time.Millisecond)
	if b2.Complete("khong-co", map[string]any{}) {
		t.Fatal("Complete id la phai false")
	}
	if b2.Connected() {
		t.Fatal("Complete id la khong duoc lam moi moc song")
	}
}

// Thong diep khi add-in chua ket noi khong duoc chi noi ve mot app cu the: nguoi dung co the dang mo
// Spreadsheets hoac Presentation, ma ca ba app dung chung mot add-in.
func TestNotConnectedMessageIsAppNeutral(t *testing.T) {
	b := testBridge(t, nil)
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"action":"et.listSheets"}`)
	b.serveBridgeCmd(recorder, httptest.NewRequest(http.MethodPost, "/cmd", body))
	text := recorder.Body.String()
	if !strings.Contains(text, "not connected") {
		t.Fatalf("phai bao chua ket noi: %s", text)
	}
	if strings.Contains(text, "Writer") {
		t.Fatalf("thong diep khong duoc chi noi ve Writer: %s", text)
	}
}
