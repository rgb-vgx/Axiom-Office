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
	"axiomoffice/core/internal/skills"
	"axiomoffice/core/internal/store"
	"axiomoffice/core/internal/wpsbridge"
)

// newSkillsHandler: Core co kenh WPS + kho skill (builtin co san "bang-diem", thu muc ca nhan rong).
func newSkillsHandler(t *testing.T) (http.Handler, string) {
	t.Helper()
	root := t.TempDir()
	builtin := filepath.Join(root, "builtin")
	user := filepath.Join(root, "user")
	if err := os.MkdirAll(filepath.Join(builtin, "bang-diem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(builtin, "bang-diem", "SKILL.md"),
		[]byte("---\nname: bang-diem\ndescription: Lap bang diem\napps: [et]\n---\n# Bang diem\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index := skills.New([]skills.Source{{Name: "builtin", Dir: builtin}, {Name: "user", Dir: user}})
	index.Reload()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	return Handler(&Deps{
		Config:  config.Config{Token: "tok"},
		Paths:   config.NewPaths(root),
		Runtime: &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:    http.DefaultClient,
		Stores:  stores,
		Skills:  index,
		WPS:     wpsbridge.New(wpsbridge.Options{AddinDir: root, SessionDir: root, Token: "tok", CorePort: 1234}),
	}, http.NewServeMux()), user
}

func decode(t *testing.T, body string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("khong phai JSON: %s", body)
	}
	return payload
}

// Pane goi qua /wps/v1 (Origin cua Core, khong token): sua skill san co thanh ban rieng, xoa ban rieng thi ban goc
// quay lai; loi tra {code, error} tieng Viet.
func TestSkillEditorEndpoints(t *testing.T) {
	handler, user := newSkillsHandler(t)

	if got := call(handler, "PUT", "/wps/v1/skills/bang-diem", `{"description":"x"}`, map[string]string{"Origin": "http://evil"}).Code; got != 403 {
		t.Fatalf("Origin la phai 403, duoc %d", got)
	}
	reply := call(handler, "GET", "/wps/v1/skills/bang-diem", "", wpsOrigin)
	if reply.Code != 200 || !strings.Contains(reply.Body.String(), `"source":"builtin"`) {
		t.Fatalf("GET chi tiet: %d %s", reply.Code, reply.Body.String())
	}

	reply = call(handler, "PUT", "/wps/v1/skills/bang-diem",
		`{"description":"Bang diem cua toi","apps":["et"],"body":"# Cua toi","files":{"references/ghi-chu.md":"ghi chu"}}`, wpsOrigin)
	result, _ := decode(t, reply.Body.String())["result"].(map[string]any)
	if reply.Code != 200 || result["source"] != "user" || result["overrides"] != true {
		t.Fatalf("sua ban san co: %d %s", reply.Code, reply.Body.String())
	}
	listing := call(handler, "GET", "/wps/v1/skills", "", wpsOrigin).Body.String()
	if !strings.Contains(listing, `"overrides":true`) {
		t.Fatalf("danh sach phai danh dau ban rieng: %s", listing)
	}
	reply = call(handler, "GET", "/wps/v1/skills/bang-diem/file?path=references/ghi-chu.md", "", wpsOrigin)
	if !strings.Contains(reply.Body.String(), `"text":"ghi chu"`) {
		t.Fatalf("doc file: %s", reply.Body.String())
	}

	reply = call(handler, "DELETE", "/wps/v1/skills/bang-diem", "", wpsOrigin)
	if reply.Code != 200 || !strings.Contains(reply.Body.String(), `"restored":true`) {
		t.Fatalf("xoa ban rieng: %d %s", reply.Code, reply.Body.String())
	}
	reply = call(handler, "DELETE", "/wps/v1/skills/bang-diem", "", wpsOrigin)
	payload := decode(t, reply.Body.String())
	if reply.Code != 400 || payload["code"] != "not-user" || !strings.Contains(payload["error"].(string), "cá nhân") {
		t.Fatalf("xoa ban san co phai bi chan bang tieng Viet: %d %s", reply.Code, reply.Body.String())
	}

	reply = call(handler, "POST", "/wps/v1/skills", `{"name":"Sai Ten","description":"x"}`, wpsOrigin)
	if reply.Code != 400 || decode(t, reply.Body.String())["code"] != "invalid" {
		t.Fatalf("ten sai: %d %s", reply.Code, reply.Body.String())
	}
	reply = call(handler, "POST", "/wps/v1/skills", `{"name":"bao-cao-quy","description":"Bao cao quy","body":"# Quy"}`, wpsOrigin)
	if reply.Code != 200 {
		t.Fatalf("tao moi: %d %s", reply.Code, reply.Body.String())
	}
	if got := call(handler, "POST", "/wps/v1/skills", `{"name":"bao-cao-quy","description":"x"}`, wpsOrigin).Code; got != 409 {
		t.Fatalf("trung ten -> 409, duoc %d", got)
	}
	if got := call(handler, "GET", "/wps/v1/skills/khong-co", "", wpsOrigin).Code; got != 404 {
		t.Fatalf("khong co -> 404, duoc %d", got)
	}
	if _, err := os.Stat(filepath.Join(user, "bao-cao-quy", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSkillImportEndpoint(t *testing.T) {
	handler, user := newSkillsHandler(t)
	pack := filepath.Join(t.TempDir(), "hop-dong")
	if err := os.MkdirAll(pack, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "SKILL.md"), []byte("---\nname: hop-dong\ndescription: Soan hop dong\n---\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"path": pack, "dryRun": true})
	reply := call(handler, "POST", "/wps/v1/skills/import", string(body), wpsOrigin)
	if reply.Code != 200 || !strings.Contains(reply.Body.String(), `"name":"hop-dong"`) || strings.Contains(reply.Body.String(), `"imported":true`) {
		t.Fatalf("xem truoc: %d %s", reply.Code, reply.Body.String())
	}
	body, _ = json.Marshal(map[string]any{"path": pack})
	reply = call(handler, "POST", "/wps/v1/skills/import", string(body), wpsOrigin)
	if !strings.Contains(reply.Body.String(), `"imported":true`) {
		t.Fatalf("nhap: %s", reply.Body.String())
	}
	if !strings.Contains(call(handler, "GET", "/wps/v1/skills", "", wpsOrigin).Body.String(), `"name":"hop-dong"`) {
		t.Fatal("skill vua nhap phai co trong danh sach ngay")
	}
	if _, err := os.Stat(filepath.Join(user, "hop-dong", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	reply = call(handler, "POST", "/wps/v1/skills/import", `{"path":"/khong/co/dau"}`, wpsOrigin)
	if reply.Code != 400 || decode(t, reply.Body.String())["code"] != "bad-path" {
		t.Fatalf("duong dan sai: %d %s", reply.Code, reply.Body.String())
	}
}
