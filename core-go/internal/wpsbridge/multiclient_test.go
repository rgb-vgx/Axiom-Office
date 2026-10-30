package wpsbridge

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// poll gia lap mot lan poll cua add-in qua ServePoll that (mo listener cho client moi nhu Core).
func pollAs(t *testing.T, b *Bridge, body map[string]any) map[string]any {
	t.Helper()
	data, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/wps/v1/poll", strings.NewReader(string(data)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Auth-Token", "tok")
	recorder := httptest.NewRecorder()
	b.ServePoll(recorder, request)
	var reply map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &reply); err != nil {
		t.Fatalf("poll tra ve khong phai JSON: %s", recorder.Body.String())
	}
	result, _ := reply["result"].(map[string]any)
	return result
}

// getBridge goi GET <path> tren cong bridge that.
func getBridge(t *testing.T, port int, path string) map[string]any {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+path, nil)
	request.Header.Set("X-Auth-Token", "tok")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("GET %s tren cong %d: %v", path, port, err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("GET %s: %s", path, data)
	}
	result, _ := body["result"].(map[string]any)
	return result
}

func startedBridge(t *testing.T, mutate func(*Options)) *Bridge {
	t.Helper()
	b := testBridge(t, mutate)
	b.StartBridge()
	t.Cleanup(b.Stop)
	return b
}

// Writer va ET mo cung luc (docs/audits/wps-linux-live-test-2026-10-28-run2.md 4.5): moi app mot cong, mot file
// session, mot hang doi. Truoc day ca hai chung mot `app` = app poll gan nhat nen agent cua pane Writer doc nham ET.
func TestWriterAndSpreadsheetsKeepSeparateSessions(t *testing.T) {
	b := startedBridge(t, nil)
	// New() dua 0 ve mac dinh: dat sau de poll tra ngay, test khong phai cho.
	b.opts.PollSeconds = 0
	writerPoll := pollAs(t, b, map[string]any{"clientId": "w1", "app": "wps",
		"document": map[string]any{"name": "bao-cao.docx"}, "commands": []any{map[string]any{"name": "writer.getText"}}})
	etPoll := pollAs(t, b, map[string]any{"clientId": "e1", "app": "et",
		"document": map[string]any{"name": "ban-hang.xlsx"}, "commands": []any{map[string]any{"name": "et.readRange"}}})

	writerPort := int(writerPoll["port"].(float64))
	etPort := int(etPoll["port"].(float64))
	if writerPort != b.ActualBridgePort() {
		t.Fatalf("client dau tien giu cong chinh %d, duoc %d", b.ActualBridgePort(), writerPort)
	}
	if etPort == 0 || etPort == writerPort {
		t.Fatalf("client thu hai phai co cong rieng, duoc %d (cong chinh %d)", etPort, writerPort)
	}

	// Xen ke poll nhieu lan: moi cong van giu dung app cua no (truoc day /health dao app moi lan poll).
	for i := 0; i < 3; i++ {
		pollAs(t, b, map[string]any{"clientId": "w1", "app": "wps", "document": map[string]any{"name": "bao-cao.docx"}})
		pollAs(t, b, map[string]any{"clientId": "e1", "app": "et", "document": map[string]any{"name": "ban-hang.xlsx"}})
		if got := getBridge(t, writerPort, "/health")["app"]; got != "wps" {
			t.Fatalf("cong Writer bao app %v", got)
		}
		if got := getBridge(t, etPort, "/health")["app"]; got != "et" {
			t.Fatalf("cong ET bao app %v", got)
		}
	}
	if got := getBridge(t, etPort, "/session")["document"].(map[string]any)["name"]; got != "ban-hang.xlsx" {
		t.Fatalf("/session cong ET phai la tai lieu ET, duoc %v", got)
	}

	// Hai file session, moi file dung cong + app cua no.
	sessions := map[string]map[string]any{}
	for i := 0; i < 2; i++ {
		b.tickSessions()
	}
	entries, _ := os.ReadDir(b.opts.SessionDir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, _ := os.ReadFile(filepath.Join(b.opts.SessionDir, entry.Name()))
		var session map[string]any
		_ = json.Unmarshal(raw, &session)
		sessions[entry.Name()] = session
	}
	pid := strconv.Itoa(os.Getpid())
	if s := sessions[pid+"-wps.json"]; s == nil || s["app"] != "wps" || int(s["port"].(float64)) != writerPort {
		t.Fatalf("file session cong chinh phai la Writer: %v", sessions)
	}
	if s := sessions[pid+"-wps-"+strconv.Itoa(etPort)+".json"]; s == nil || s["app"] != "et" {
		t.Fatalf("file session cua ET phai rieng: %v", sessions)
	}

	// Lenh gui cho ET chi toi poll cua ET.
	et := b.Resolve("e1", "")
	replyCh := make(chan map[string]any, 1)
	go func() {
		reply, _ := et.Enqueue("et.readRange", map[string]any{"range": "A1"})
		replyCh <- reply
	}()
	time.Sleep(100 * time.Millisecond)
	if jobs := pollAs(t, b, map[string]any{"clientId": "w1", "app": "wps"})["jobs"].([]any); len(jobs) != 0 {
		t.Fatalf("poll cua Writer khong duoc nhan lenh cua ET: %v", jobs)
	}
	jobs := pollAs(t, b, map[string]any{"clientId": "e1", "app": "et"})["jobs"].([]any)
	if len(jobs) != 1 {
		t.Fatalf("poll cua ET phai nhan lenh: %v", jobs)
	}
	id := jobs[0].(map[string]any)["id"].(string)
	if !b.Complete(id, map[string]any{"ok": true, "result": "x"}) {
		t.Fatal("Complete job cua client phu phai true")
	}
	if reply := <-replyCh; reply["ok"] != true {
		t.Fatalf("ket qua phai ve dung nguoi goi: %v", reply)
	}
}

// Pane gui client id cua minh; khong co thi chon theo app; cuoi cung la client chinh.
func TestResolvePicksClientForPane(t *testing.T) {
	b := testBridge(t, nil)
	writer, _ := b.recordPoll(map[string]any{"clientId": "w1", "app": "wps"})
	et, _ := b.recordPoll(map[string]any{"clientId": "e1", "app": "et"})
	if got := b.Resolve("e1", "wps"); got != et {
		t.Fatal("client id thang app")
	}
	if got := b.Resolve("", "et"); got != et {
		t.Fatal("khong co client id -> client cua app do")
	}
	if got := b.Resolve("khong-co", "et"); got != et {
		t.Fatal("client id la -> theo app")
	}
	if got := b.Resolve("", "wpp"); got != writer {
		t.Fatal("khong co client cua app -> client chinh")
	}
	if got := b.Resolve("", ""); got != b.Primary() {
		t.Fatal("khong co gi -> client chinh")
	}
}

// Client chinh mat ket noi roi client moi xuat hien: client moi nhan cong chinh (cau hinh cu tro 47861 van dung),
// va lenh chua giao cua chu cu khong roi vao tay app moi.
func TestPrimaryPortMovesToNextClientAfterDisconnect(t *testing.T) {
	b := testBridge(t, func(o *Options) { o.DisconnectSeconds = 1 })
	first, _ := b.recordPoll(map[string]any{"clientId": "w1", "app": "wps"})
	b.mu.Lock()
	stale := &Job{ID: "cu", Action: "writer.getText", wait: make(chan map[string]any, 1)}
	first.queue = append(first.queue, stale)
	first.waiting[stale.ID] = stale
	b.mu.Unlock()

	time.Sleep(1200 * time.Millisecond)
	second, needsListener := b.recordPoll(map[string]any{"clientId": "e1", "app": "et"})
	if second != b.Primary() || needsListener {
		t.Fatal("client moi phai nhan cong chinh khi chu cu da mat ket noi")
	}
	if second.App() != "et" || second.ID() != "e1" {
		t.Fatalf("cong chinh phai mang trang thai cua client moi: %s/%s", second.ID(), second.App())
	}
	select {
	case reply := <-stale.wait:
		if !strings.Contains(textOf(reply["error"]), "was not sent") {
			t.Fatalf("lenh chua giao cua chu cu phai bao chua gui: %v", reply)
		}
	default:
		t.Fatal("lenh chua giao cua chu cu phai duoc tra loi")
	}
	if jobs := second.takeJobs(); len(jobs) != 0 {
		t.Fatalf("app moi khong duoc nhan lenh cua chu cu: %v", jobs)
	}
}

// Client phu mat ket noi: dong cong, xoa file session, bo khoi danh sach.
func TestSecondaryClientIsDroppedAfterDisconnect(t *testing.T) {
	b := startedBridge(t, func(o *Options) { o.DisconnectSeconds = 1 })
	// New() dua 0 ve mac dinh: dat sau de poll tra ngay, test khong phai cho.
	b.opts.PollSeconds = 0
	pollAs(t, b, map[string]any{"clientId": "w1", "app": "wps"})
	etPort := int(pollAs(t, b, map[string]any{"clientId": "e1", "app": "et"})["port"].(float64))
	b.tickSessions()
	sessionFile := filepath.Join(b.opts.SessionDir, strconv.Itoa(os.Getpid())+"-wps-"+strconv.Itoa(etPort)+".json")
	if _, err := os.Stat(sessionFile); err != nil {
		t.Fatalf("client phu phai co file session: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	b.tickSessions()
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Fatalf("mat ket noi thi file session phai bi xoa: %v", err)
	}
	if _, err := http.Get("http://127.0.0.1:" + strconv.Itoa(etPort) + "/health"); err == nil {
		t.Fatal("cong cua client phu phai dong")
	}
	clients := b.Status()["clients"].([]any)
	for _, raw := range clients {
		if raw.(map[string]any)["clientId"] == "e1" {
			t.Fatalf("client phu phai bi bo: %v", clients)
		}
	}
}

// WPS dong giua luc chay lenh (khong con tien trinh nao): bao loi trong vai giay, khong cho het CmdWaitSeconds, va van
// canh bao lenh co the da ghi.
func TestInFlightCommandFailsFastWhenWPSIsGone(t *testing.T) {
	b := testBridge(t, func(o *Options) { o.DisconnectSeconds = 1; o.CmdWaitSeconds = 60 })
	b.appAlive = func(string) (bool, bool) { return false, true }
	b.recordPoll(map[string]any{})
	started := time.Now()
	replyCh := make(chan map[string]any, 1)
	go func() {
		reply, _ := b.Enqueue("et.importCsv", map[string]any{})
		replyCh <- reply
	}()
	if jobs := takeJobsWithin(t, b, 3*time.Second); len(jobs) != 1 {
		t.Fatalf("khong lay duoc job: %v", jobs)
	}
	select {
	case reply := <-replyCh:
		message := textOf(reply["error"])
		if !strings.Contains(message, "WPS closed") || !strings.Contains(message, "may have been applied") {
			t.Fatalf("phai bao WPS da dong va canh bao lenh co the da ghi: %q", message)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WPS da dong thi phai bao loi trong vai giay")
	}
	if waited := time.Since(started); waited > 5*time.Second {
		t.Fatalf("cho qua lau: %v", waited)
	}
}

// WPS con chay (hoac khong biet) thi lenh dai van duoc cho - khong cat ngang lenh dang ghi.
func TestInFlightCommandWaitsWhileWPSIsRunning(t *testing.T) {
	for _, alive := range []func(string) (bool, bool){
		func(string) (bool, bool) { return true, true },
		func(string) (bool, bool) { return false, false },
	} {
		b := testBridge(t, func(o *Options) { o.DisconnectSeconds = 1; o.CmdWaitSeconds = 60 })
		b.appAlive = alive
		b.recordPoll(map[string]any{})
		replyCh := make(chan map[string]any, 1)
		go func() {
			reply, _ := b.Enqueue("et.importCsv", map[string]any{})
			replyCh <- reply
		}()
		jobs := takeJobsWithin(t, b, 3*time.Second)
		time.Sleep(2200 * time.Millisecond)
		select {
		case reply := <-replyCh:
			t.Fatalf("lenh dang chay khong duoc bi cat: %v", reply)
		default:
		}
		b.Complete(jobs[0].ID, map[string]any{"ok": true})
		<-replyCh
	}
}

// Doc /proc: tim thay tien trinh WPS theo ten; khong co /proc thi "khong biet".
func TestWPSAppAliveReadsProc(t *testing.T) {
	root := t.TempDir()
	old := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = old })
	write := func(pid, comm string) {
		_ = os.MkdirAll(filepath.Join(root, pid), 0o755)
		_ = os.WriteFile(filepath.Join(root, pid, "comm"), []byte(comm+"\n"), 0o644)
	}
	write("10", "bash")
	write("self", "wps")
	if alive, known := wpsAppAlive("wps"); alive || !known {
		t.Fatalf("khong co tien trinh WPS -> (false, true), duoc (%v, %v)", alive, known)
	}
	write("42", "et")
	if alive, known := wpsAppAlive("wps"); !alive || !known {
		t.Fatalf("con tien trinh WPS -> (true, true), duoc (%v, %v)", alive, known)
	}
	procRoot = filepath.Join(root, "khong-co")
	if _, known := wpsAppAlive("wps"); known {
		t.Fatal("khong doc duoc /proc -> khong biet")
	}
}
