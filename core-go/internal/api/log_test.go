package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// GET /wps/v1/status: nut "Trang thai" tren ribbon WPS. Pane khong tu hoi duoc tang add-in (bridge nam
// o cong rieng) nen mot loi goi nay phai tra ca hai tang.
func TestWPSStatusReportsBothLayers(t *testing.T) {
	handler := newWPSHandler(t)
	reply := call(handler, "GET", "/wps/v1/status", "", wpsOrigin)
	if reply.Code != 200 {
		t.Fatalf("status -> %d %s", reply.Code, reply.Body.String())
	}
	var payload struct {
		OK     bool `json:"ok"`
		Result struct {
			Addin map[string]any `json:"addin"`
			Core  map[string]any `json:"core"`
		} `json:"result"`
	}
	if err := json.Unmarshal(reply.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	addin, core := payload.Result.Addin, payload.Result.Core
	if !payload.OK || addin == nil || core == nil {
		t.Fatalf("thieu mot trong hai tang: %s", reply.Body.String())
	}
	// Tang add-in: chua co add-in nao poll -> connected=false, va phai noi ro DA poll lan nao chua.
	if addin["connected"] != false || addin["everPoll"] != false {
		t.Fatalf("addin chua ket noi phai la connected=false/everPoll=false: %v", addin)
	}
	if _, ok := addin["lastPollSeconds"]; ok {
		t.Fatalf("chua poll lan nao thi khong duoc bia ra lastPollSeconds: %v", addin)
	}
	for _, key := range []string{"app", "bridgePort", "pollSeconds", "disconnectSeconds", "uptimeSeconds", "queued", "waiting", "running"} {
		if _, ok := addin[key]; !ok {
			t.Errorf("thieu truong addin.%s: %v", key, addin)
		}
	}
	// Tang Core: du de nut "Trang thai" va "Mo nhat ky" khong phai goi noi nao khac.
	for _, key := range []string{"version", "pid", "port", "uptimeSeconds", "dataDir", "logFile", "model", "endpoint", "configured", "memory", "skills"} {
		if _, ok := core[key]; !ok {
			t.Errorf("thieu truong core.%s: %v", key, core)
		}
	}
	// "configured" la thu nut "Kiem tra mo hinh" doc de biet co phai bao "chua cau hinh AI" khong, nen no
	// phai dung la he qua cua endpoint+model chu khong phai mot co doc lap co the lech.
	wantConfigured := core["endpoint"] != "" && core["model"] != ""
	if core["configured"] != wantConfigured {
		t.Errorf("configured = %v, want %v (endpoint=%q model=%q)",
			core["configured"], wantConfigured, core["endpoint"], core["model"])
	}
	if core["version"] != "1.0.0" || core["port"] != float64(1234) {
		t.Fatalf("core phai la Core dang chay: %v", core)
	}
}

func TestWPSStatusAuth(t *testing.T) {
	handler := newWPSHandler(t)
	cases := []struct {
		headers map[string]string
		want    int
	}{
		{wpsOrigin, 200},
		{nil, 401}, // khong Origin, khong token
		{map[string]string{"Origin": "http://evil"}, 403}, // Origin la
		{map[string]string{"X-Auth-Token": "tok"}, 200},   // duong script/MCP
	}
	for _, c := range cases {
		if got := call(handler, "GET", "/wps/v1/status", "", c.headers).Code; got != c.want {
			t.Errorf("GET /wps/v1/status %v -> %d, want %d", c.headers, got, c.want)
		}
	}
}

// GET /v1/log: nut "Mo nhat ky" tren ribbon WPS (ban Windows mo file bang trinh soan thao; trong CEF
// pane phai doc noi dung). Day la phep do cho phan DOC PHAN CUOI: file lon, cat khuc giua dong, va file
// chua ton tai.
func TestLogTail(t *testing.T) {
	root := t.TempDir()
	logFile := filepath.Join(root, "core.log")
	lines := make([]string, 0, 500)
	for i := 1; i <= 500; i++ {
		lines = append(lines, "2026-10-19 00:00:00.000 [INFO] [pid 1] dong so "+strconv.Itoa(i))
	}
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// mac dinh: 200 dong cuoi, moi dong con nguyen.
	payload, err := tailLog(logFile, defaultLogLines, maxLogBytes)
	if err != nil {
		t.Fatal(err)
	}
	got := payload["lines"].([]string)
	if len(got) != defaultLogLines {
		t.Fatalf("so dong = %d, want %d", len(got), defaultLogLines)
	}
	if !strings.HasSuffix(got[len(got)-1], "so 500") || !strings.HasSuffix(got[0], "so 301") {
		t.Fatalf("phai lay 200 dong CUOI: %q ... %q", got[0], got[len(got)-1])
	}
	if payload["truncated"] != true {
		t.Fatalf("con dong cu hon thi truncated phai la true: %v", payload)
	}
	if payload["exists"] != true || payload["path"] != logFile {
		t.Fatalf("payload = %v", payload)
	}

	// Cat khuc giua dong: dong dau khong tron ven phai bi bo, khong duoc tra ve mot dong cut.
	payload, err = tailLog(logFile, 3, 60)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range payload["lines"].([]string) {
		if !strings.HasPrefix(line, "2026-10-19") {
			t.Fatalf("dong cut giua khuc bi tra ve: %q", line)
		}
	}

	// File chua co: khong phai loi - pane phai noi duoc "chua co nhat ky".
	payload, err = tailLog(filepath.Join(root, "khong-co.log"), 10, maxLogBytes)
	if err != nil {
		t.Fatalf("file chua co khong duoc coi la loi: %v", err)
	}
	if payload["exists"] != false || len(payload["lines"].([]string)) != 0 {
		t.Fatalf("payload = %v", payload)
	}
}

func TestLogRoute(t *testing.T) {
	handler := newWPSHandler(t)
	// Duong pane: Origin dung origin cua Core.
	reply := call(handler, "GET", "/wps/v1/log", "", wpsOrigin)
	if reply.Code != 200 {
		t.Fatalf("GET /wps/v1/log -> %d", reply.Code)
	}
	// Duong token: /v1/log giu guard chung.
	if got := call(handler, "GET", "/v1/log", "", nil).Code; got != 401 {
		t.Fatalf("GET /v1/log khong token -> %d, want 401", got)
	}
	if got := call(handler, "GET", "/v1/log", "", map[string]string{"X-Auth-Token": "tok"}).Code; got != 200 {
		t.Fatalf("GET /v1/log co token -> %d, want 200", got)
	}
	// Origin la -> 403: duong nay doc noi dung file, khong duoc mo cho trang khac.
	if got := call(handler, "GET", "/wps/v1/log", "", map[string]string{"Origin": "http://evil"}).Code; got != 403 {
		t.Fatalf("Origin la -> %d, want 403", got)
	}
}

// Core chua ghi dong log nao (may moi cai) khong phai loi: pane phai noi duoc "chua co nhat ky" thay vi
// hien mot cau loi do.
func TestLogMissingFileIsNotAnError(t *testing.T) {
	handler := newWPSHandler(t)
	reply := call(handler, "GET", "/wps/v1/log", "", wpsOrigin)
	var payload struct {
		OK     bool `json:"ok"`
		Result struct {
			Exists bool     `json:"exists"`
			Path   string   `json:"path"`
			Lines  []string `json:"lines"`
		} `json:"result"`
	}
	if err := json.Unmarshal(reply.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.Result.Exists || payload.Result.Path == "" || len(payload.Result.Lines) != 0 {
		t.Fatalf("payload = %s", reply.Body.String())
	}
}
