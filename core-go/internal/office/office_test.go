package office

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeSessionFile: ghi mot file session (file dat ten theo pid nhu ban that, hoac ten rieng khi can
// nhieu ban ghi cua cung mot pid).
func writeSessionFile(t *testing.T, dir, name string, pid, port int, stale bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	seen := float64(time.Now().UnixMilli()) / 1000.0
	if stale {
		seen -= StaleSeconds + 30
	}
	payload := map[string]any{
		"pid": pid, "app": "et", "family": "office", "port": port, "host": "EXCEL.EXE", "version": "1.0.0",
		"lastSeenEpoch": seen, "document": "bao-cao.xlsx", "documentPath": "C:/work/bao-cao.xlsx",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryFiltersByLivenessAndAge(t *testing.T) {
	dir := t.TempDir()
	alive := os.Getpid()
	writeSessionFile(t, dir, strconv.Itoa(alive), alive, 47840, false) // con song, moi
	writeSessionFile(t, dir, "cu", alive, 47841, true)                 // con song nhung cu
	writeSessionFile(t, dir, "chet", 999_999_999, 47842, false)        // pid khong ton tai
	if err := os.WriteFile(filepath.Join(dir, "hong.json"), []byte("{khong phai json"), 0o644); err != nil {
		t.Fatal(err)
	}

	directory := NewDirectory(dir)
	fresh := directory.List(false)
	if len(fresh) != 1 || fresh[0].Port != 47840 || fresh[0].App != "et" {
		t.Fatalf("List = %+v", fresh)
	}
	if all := directory.List(true); len(all) != 2 {
		t.Fatalf("includeStale = %+v", all)
	}
	if found := directory.Find(47840); found == nil || found.Document != "bao-cao.xlsx" {
		t.Fatalf("Find = %+v", found)
	}
	if directory.Find(47899) != nil {
		t.Fatal("port khong co session phai tra nil")
	}
	if !strings.Contains(fresh[0].Describe(), "et/office port 47840") {
		t.Fatalf("Describe = %q", fresh[0].Describe())
	}
}

func TestDocumentKey(t *testing.T) {
	// Tai lieu chua luu -> "".
	if got := DocumentKey("  "); got != "" {
		t.Fatalf("DocumentKey rong = %q", got)
	}
	key := DocumentKey("C:/Work/Bao-Cao.xlsx")
	if runtime.GOOS == "windows" {
		if key != `c:\work\bao-cao.xlsx` {
			t.Fatalf("Windows phai chu thuong + \\\\: %q", key)
		}
	} else if key != "C:/Work/Bao-Cao.xlsx" {
		t.Fatalf("Linux giu nguyen: %q", key)
	}
}

func TestBridgeCommandAndCatalogCache(t *testing.T) {
	commands := 0
	seen := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") != "tok" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"pid":7,"port":47840}}`))
		case "/commands":
			commands++
			_, _ = w.Write([]byte(`{"ok":true,"result":{"version":"1.0.0","commands":[
				{"name":"et.writeRange","kind":"et","agent":true,"summary":"ghi vung",
				 "params":[{"name":"range","required":true,"hint":"A1"},{"name":"values","required":true}]},
				{"name":"et.save","kind":"et","agent":false,"params":[]}]}}`))
		case "/cmd":
			var body struct {
				Action string         `json:"action"`
				Params map[string]any `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			seen = append(seen, body.Action)
			_, _ = w.Write([]byte(`{"ok":true,"result":{"action":"` + body.Action + `","written":2}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":false,"error":"khong co"}`))
		}
	}))
	defer server.Close()
	port := portOf(t, server.URL)

	client := NewBridgeClient(server.Client(), "tok")
	if health := client.Health(context.Background(), port); health == nil || health["pid"] != float64(7) {
		t.Fatalf("Health = %+v", health)
	}

	catalog := client.GetCommands(context.Background(), port, false)
	if catalog == nil || catalog.Version != "1.0.0" || len(catalog.Commands) != 2 {
		t.Fatalf("catalog = %+v", catalog)
	}
	if catalog.Commands[0].Name != "et.writeRange" || !catalog.Commands[0].Agent ||
		catalog.Commands[0].Params[0].Hint != "A1" || catalog.Commands[1].Agent {
		t.Fatalf("lenh = %+v", catalog.Commands)
	}
	// Lan hai dung cache (khong goi lai /commands).
	if second := client.GetCommands(context.Background(), port, false); second == nil {
		t.Fatal("cache phai tra catalog")
	}
	if commands != 1 {
		t.Fatalf("/commands goi %d lan, phai la 1", commands)
	}
	if forced := client.GetCommands(context.Background(), port, true); forced == nil || commands != 2 {
		t.Fatalf("force phai goi lai: %d", commands)
	}

	result := client.Command(context.Background(), port, "et.writeRange", map[string]any{"range": "A1"})
	if !result.OK || result.Status != 200 || !strings.Contains(result.RawJSON, `"written":2`) {
		t.Fatalf("Command = %+v", result)
	}
	if len(seen) != 1 || seen[0] != "et.writeRange" {
		t.Fatalf("lenh xuong bridge = %v", seen)
	}

	// Bridge tat -> OK=false kem cau loi (khong nem loi giao thuc).
	dead := NewBridgeClient(server.Client(), "tok")
	deadResult := dead.Command(context.Background(), portOf(t, "http://127.0.0.1:1"), "et.writeRange", nil)
	if deadResult.OK || deadResult.Error == "" {
		t.Fatalf("bridge tat = %+v", deadResult)
	}
}

func portOf(t *testing.T, raw string) int {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url la: %s", raw)
	}
	if parsed.Port() == "" {
		if parsed.Scheme == "https" {
			return 443
		}
		return 80
	}
	value, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("port la: %s", raw)
	}
	return value
}

// writeAppSession: nhu writeSessionFile nhung doi duoc ten app (helper kia co dinh "et").
func writeAppSession(t *testing.T, dir, name string, pid, port int, app string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"pid": pid, "app": app, "family": "libreoffice", "port": port, "host": "soffice", "version": "0.1.0",
		"lastSeenEpoch": float64(time.Now().UnixMilli()) / 1000.0,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Ports: cong cua cac bridge thuoc mot ho ung dung, doc THANG tu file session (khong hoi /health).
//
// Dung o duong phan giai cong cua tool MCP live. Do 02/10/2026: tren Windows ten app ("et"/"wps"/"wpp")
// luon tra ve cong WPS, nen wps_live_write_range tro vao 47822 trong khi bridge LibreOffice dang chay o
// 47852 - va chinh office_sessions bao dung 47852. Registry la su that, cau hinh chi la phong doan.
func TestPortsListsLiveSessionsOfOneApp(t *testing.T) {
	dir := t.TempDir()
	alive := os.Getpid()
	writeAppSession(t, dir, "et-1", alive, 47852, "et")
	writeAppSession(t, dir, "et-2", alive, 47822, "et")
	writeAppSession(t, dir, "wps-1", alive, 47851, "wps")
	writeAppSession(t, dir, "et-chet", 999_999_999, 47899, "et") // pid khong ton tai
	if err := os.WriteFile(filepath.Join(dir, "hong.json"), []byte("{khong phai json"), 0o644); err != nil {
		t.Fatal(err)
	}

	directory := &Directory{Path: dir}
	if got := directory.Ports("et"); len(got) != 2 || got[0] != 47822 || got[1] != 47852 {
		t.Fatalf("Ports(et) = %v (phai sap xep, bo bridge chet va file hong)", got)
	}
	if got := directory.Ports("wps"); len(got) != 1 || got[0] != 47851 {
		t.Fatalf("Ports(wps) = %v", got)
	}
	if got := directory.Ports("khong-co-ho-nay"); len(got) != 0 {
		t.Fatalf("ho la phai tra rong: %v", got)
	}
	if got := (&Directory{Path: filepath.Join(dir, "khong-ton-tai")}).Ports("et"); len(got) != 0 {
		t.Fatalf("thu muc khong ton tai phai tra rong: %v", got)
	}
}
