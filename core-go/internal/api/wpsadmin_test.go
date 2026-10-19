package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/store"
	"axiomoffice/core/internal/wpsbridge"
)

// newWPSHandler: Core co bat kenh WPS (mapWPS chi dang ky /wps/v1/* khi deps.WPS khac nil).
// Cau hinh doc lai moi lan goi (LoadConfig) va ghi vao XDG_CONFIG_HOME tam - dung nhu luc chay that,
// de kiem duoc ca vong "ghi roi doc lai".
func newWPSHandler(t *testing.T) http.Handler {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	paths := config.NewPaths(root)
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	bridge := wpsbridge.New(wpsbridge.Options{
		AddinDir: root, SessionDir: root, Token: "tok", CorePort: 1234, Version: "1.0.0",
	})
	return Handler(&Deps{
		Config:     config.Config{Token: "tok", MemoryEnabled: true, LlmProvider: "openai", LlmModel: "m"},
		LoadConfig: config.Load,
		Paths:      paths,
		Runtime:    &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:       http.DefaultClient,
		Stores:     stores,
		WPS:        bridge,
	}, http.NewServeMux())
}

// origin: header Origin dung bang origin cua Core - trinh duyet gui header nay khi pane (trang cua
// Core) goi API, va no khong gia duoc tu trang khac.
var wpsOrigin = map[string]string{"Origin": "http://127.0.0.1:1234", "Content-Type": "application/json"}

func TestWPSAdminAuth(t *testing.T) {
	handler := newWPSHandler(t)
	cases := []struct {
		method, path string
		headers      map[string]string
		want         int
	}{
		{"GET", "/wps/v1/setup", wpsOrigin, 200},
		{"GET", "/wps/v1/setup", nil, 401},                                        // khong Origin, khong token
		{"GET", "/wps/v1/setup", map[string]string{"Origin": "http://evil"}, 403}, // Origin la
		{"GET", "/wps/v1/setup", map[string]string{"X-Auth-Token": "tok"}, 200},   // duong script/MCP
		{"GET", "/wps/v1/skills", wpsOrigin, 404},                                 // chua co Skills -> khong dang ky
		{"GET", "/wps/v1/audit", wpsOrigin, 200},                                  //
		{"GET", "/wps/v1/conversations", wpsOrigin, 200},                          //
		{"GET", "/v1/setup", wpsOrigin, 403},                                      // /v1/* giu guard cu
	}
	for _, c := range cases {
		if got := call(handler, c.method, c.path, "", c.headers).Code; got != c.want {
			t.Errorf("%s %s %v -> %d, want %d", c.method, c.path, c.headers, got, c.want)
		}
	}
}

func TestWPSConfigWrite(t *testing.T) {
	handler := newWPSHandler(t)

	// Khoa la -> 400 (khong ghi gi).
	reply := call(handler, "POST", "/wps/v1/config", `{"Token":"khac"}`, wpsOrigin)
	if reply.Code != 400 || !strings.Contains(reply.Body.String(), "cannot be changed") {
		t.Fatalf("khoa la phai bi tu choi: %d %s", reply.Code, reply.Body.String())
	}
	// Body rong -> 400.
	if got := call(handler, "POST", "/wps/v1/config", `{}`, wpsOrigin).Code; got != 400 {
		t.Fatalf("body rong -> %d, want 400", got)
	}

	reply = call(handler, "POST", "/wps/v1/config", `{"VisualQaEnabled":true,"LlmModel":"m2"}`, wpsOrigin)
	if reply.Code != 200 {
		t.Fatalf("ghi cau hinh -> %d %s", reply.Code, reply.Body.String())
	}
	var payload struct {
		OK     bool `json:"ok"`
		Result struct {
			Saved   []string       `json:"saved"`
			Ignored []string       `json:"ignored"`
			Current map[string]any `json:"current"`
		} `json:"result"`
	}
	if err := json.Unmarshal(reply.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Result.Saved) != 2 || payload.Result.Current["visualQaEnabled"] != true ||
		payload.Result.Current["model"] != "m2" {
		t.Fatalf("ket qua ghi cau hinh = %+v", payload.Result)
	}

	// Ghi that vao config.json (cung file extension LibreOffice dung), giu nguyen khoa khac.
	path := config.DefaultJSONPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config.json phai duoc ghi: %v", err)
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	if values["VisualQaEnabled"] != true || values["LlmModel"] != "m2" {
		t.Fatalf("config.json = %v", values)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config.json phai la 0600: %v %v", info, err)
	}

	// Core doc lai duoc: nut toggle doc trang thai THAT chu khong suy dien tu viec ghi thanh cong.
	body := call(handler, "GET", "/wps/v1/setup", "", wpsOrigin).Body.String()
	if !strings.Contains(body, `"visualQaEnabled":true`) {
		t.Fatalf("/wps/v1/setup phai thay cau hinh vua ghi: %s", body)
	}
}

// Bien moi truong AXIOM_* de len cau hinh (config.From: env > nguon nen tang): ghi "thanh cong" nhung
// gia tri song khong doi - phai bao ro de nguoi dung khong tuong nut hong.
func TestWPSConfigReportsEnvOverride(t *testing.T) {
	handler := newWPSHandler(t)
	t.Setenv("AXIOM_VISUAL_QA", "0")
	reply := call(handler, "POST", "/wps/v1/config", `{"VisualQaEnabled":true}`, wpsOrigin)
	var payload struct {
		Result struct {
			Ignored []string `json:"ignored"`
		} `json:"result"`
	}
	if err := json.Unmarshal(reply.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Result.Ignored) != 1 || payload.Result.Ignored[0] != "VisualQaEnabled" {
		t.Fatalf("phai bao bien moi truong de: %s", reply.Body.String())
	}
}

func TestWPSExecNeedsConnectedAddin(t *testing.T) {
	handler := newWPSHandler(t)
	// Chua co add-in poll -> 409, khong xep hang lenh nao. Kem ma may doc duoc: pane dich hai ca 409
	// sang hai cau tieng Viet khac nhau (pane.html: execError) chu khong hien chuoi tieng Anh cua Core.
	reply := call(handler, "POST", "/wps/v1/exec", `{"action":"writer.checkTables"}`, wpsOrigin)
	if reply.Code != 409 || !strings.Contains(reply.Body.String(), "not connected") {
		t.Fatalf("exec khi chua ket noi -> %d %s", reply.Code, reply.Body.String())
	}
	var conflict struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(reply.Body.Bytes(), &conflict); err != nil {
		t.Fatal(err)
	}
	if conflict.Code != "addin-disconnected" {
		t.Fatalf("409 phai mang ma addin-disconnected: %s", reply.Body.String())
	}
	// Thieu action -> 400.
	if got := call(handler, "POST", "/wps/v1/exec", `{}`, wpsOrigin).Code; got != 400 {
		t.Fatalf("thieu action -> %d, want 400", got)
	}
	// Origin la -> 403 (khong chay lenh nao len tai lieu).
	if got := call(handler, "POST", "/wps/v1/exec", `{"action":"writer.save"}`,
		map[string]string{"Origin": "http://evil", "Content-Type": "application/json"}).Code; got != 403 {
		t.Fatalf("Origin la -> %d, want 403", got)
	}
}
