package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/skills"
)

// writeExternalSkill: mot thu muc skill nam ngoai thu muc du lieu cua Core.
func writeExternalSkill(t *testing.T, root, folder, frontmatter string) string {
	t.Helper()
	directory := filepath.Join(root, folder)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\n" + strings.TrimSpace(frontmatter) + "\n---\n# Than\nNoi dung"
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return directory
}

// newSkillsHandler: Deps toi thieu cho endpoint /v1/skills (khong can store).
func newSkillsHandler(t *testing.T, writeBuiltin func(builtin string)) (http.Handler, config.Paths) {
	t.Helper()
	root := t.TempDir()
	paths := config.NewPaths(root)
	builtin := filepath.Join(root, "builtin")
	if writeBuiltin != nil {
		if err := os.MkdirAll(builtin, 0o755); err != nil {
			t.Fatal(err)
		}
		writeBuiltin(builtin)
	}
	index := skills.New(skills.DefaultSources(builtin, nil, paths.SkillsDir))
	return Handler(&Deps{
		Config:  config.Config{Token: "tok"},
		Paths:   paths,
		Runtime: &Runtime{Port: 1234, Started: time.Now(), Version: "test"},
		Skills:  index,
	}, http.NewServeMux()), paths
}

func skillsHeaders() map[string]string {
	return map[string]string{"X-Auth-Token": "tok", "Content-Type": "application/json"}
}

func skillsBody(t *testing.T, payload map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// resultOf: truong `result` cua mot response thanh cong.
func resultOf(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		OK     bool           `json:"ok"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("JSON hong: %v (%s)", err, recorder.Body.String())
	}
	if !envelope.OK {
		t.Fatalf("response khong ok: %s", recorder.Body.String())
	}
	return envelope.Result
}

func stringsOf(value any) []string {
	list := []string{}
	items, _ := value.([]any)
	for _, item := range items {
		text, _ := item.(string)
		list = append(list, text)
	}
	return list
}

func skillsIn(t *testing.T, payload map[string]any, key string) []map[string]any {
	t.Helper()
	items, _ := payload[key].([]any)
	list := []map[string]any{}
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("%s khong phai danh sach doi tuong: %v", key, item)
		}
		list = append(list, entry)
	}
	return list
}

// Nap skill tu thu muc ngoai: inspect khong ghi gi, import copy vao thu muc nguoi dung, trung ten thi tu choi
// cho tới khi overwrite=true.
func TestSkillsInspectAndImport(t *testing.T) {
	handler, paths := newSkillsHandler(t, nil)
	source := t.TempDir()
	skill := writeExternalSkill(t, source, "ngoai-skill", "name: ngoai-skill\ndescription: do nguoi dung viet")

	recorder := call(handler, "POST", "/v1/skills/inspect", skillsBody(t, map[string]any{"path": skill}), skillsHeaders())
	if recorder.Code != 200 {
		t.Fatalf("inspect -> %d %s", recorder.Code, recorder.Body.String())
	}
	result := resultOf(t, recorder)
	if result["target"] != paths.SkillsDir {
		t.Fatalf("target = %v, want %s", result["target"], paths.SkillsDir)
	}
	candidates := skillsIn(t, result, "candidates")
	if len(candidates) != 1 {
		t.Fatalf("candidates = %v", candidates)
	}
	if want := filepath.Join(paths.SkillsDir, "ngoai-skill"); candidates[0]["target"] != want {
		t.Fatalf("target = %v, want %s", candidates[0]["target"], want)
	}
	if candidates[0]["name"] != "ngoai-skill" || candidates[0]["conflict"] != false {
		t.Fatalf("candidate = %v", candidates[0])
	}
	if candidates[0]["source_dir"] != skill {
		t.Fatalf("source_dir = %v", candidates[0]["source_dir"])
	}
	if _, err := os.Stat(paths.SkillsDir); err == nil {
		t.Fatal("inspect khong duoc ghi len dia")
	}

	// import: copy that su.
	recorder = call(handler, "POST", "/v1/skills/import", skillsBody(t, map[string]any{"path": skill}), skillsHeaders())
	if recorder.Code != 200 {
		t.Fatalf("import -> %d %s", recorder.Code, recorder.Body.String())
	}
	result = resultOf(t, recorder)
	if strings.Join(stringsOf(result["imported"]), ",") != "ngoai-skill" {
		t.Fatalf("imported = %v", result["imported"])
	}
	if _, err := os.Stat(filepath.Join(paths.SkillsDir, "ngoai-skill", "SKILL.md")); err != nil {
		t.Fatalf("skill phai nam trong thu muc nguoi dung: %v", err)
	}
	listing := result["listing"].(map[string]any)
	found := false
	for _, entry := range skillsIn(t, listing, "skills") {
		if entry["name"] == "ngoai-skill" {
			found = true
			if entry["source"] != "user" {
				t.Fatalf("nguon phai la user: %v", entry)
			}
		}
	}
	if !found {
		t.Fatalf("listing thieu skill vua nhap: %v", listing["skills"])
	}

	// Nhap lai: tu choi khi chua cho ghi de.
	recorder = call(handler, "POST", "/v1/skills/import", skillsBody(t, map[string]any{"path": skill}), skillsHeaders())
	if recorder.Code != 400 || !strings.Contains(recorder.Body.String(), "already exists") {
		t.Fatalf("nhap lai -> %d %s", recorder.Code, recorder.Body.String())
	}

	// overwrite=true: ghi de.
	recorder = call(handler, "POST", "/v1/skills/import",
		skillsBody(t, map[string]any{"path": skill, "overwrite": true}), skillsHeaders())
	if recorder.Code != 200 {
		t.Fatalf("overwrite -> %d %s", recorder.Code, recorder.Body.String())
	}
	result = resultOf(t, recorder)
	if strings.Join(stringsOf(result["replaced"]), ",") != "ngoai-skill" {
		t.Fatalf("replaced = %v", result["replaced"])
	}

	// inspect lai: gio da trung ten o noi nhan.
	recorder = call(handler, "POST", "/v1/skills/inspect", skillsBody(t, map[string]any{"path": skill}), skillsHeaders())
	candidates = skillsIn(t, resultOf(t, recorder), "candidates")
	if candidates[0]["conflict"] != true {
		t.Fatalf("phai bao trung ten: %v", candidates[0])
	}
}

// Skill cung ten dang co o nguon khac: inspect noi ro dang o nguon nao de giao dien canh bao truoc khi ghi de.
func TestSkillsInspectShowsExistingSource(t *testing.T) {
	handler, _ := newSkillsHandler(t, func(builtin string) {
		writeExternalSkill(t, builtin, "bang-diem", "name: bang-diem\ndescription: ban dung san")
	})
	source := t.TempDir()
	skill := writeExternalSkill(t, source, "bang-diem", "name: bang-diem\ndescription: ban nguoi dung")

	recorder := call(handler, "POST", "/v1/skills/inspect", skillsBody(t, map[string]any{"path": skill}), skillsHeaders())
	if recorder.Code != 200 {
		t.Fatalf("inspect -> %d %s", recorder.Code, recorder.Body.String())
	}
	candidates := skillsIn(t, resultOf(t, recorder), "candidates")
	if len(candidates) != 1 {
		t.Fatalf("candidates = %v", candidates)
	}
	existing, ok := candidates[0]["existing"].(map[string]any)
	if !ok || existing["source"] != "builtin" {
		t.Fatalf("phai noi ro dang co o nguon builtin: %v", candidates[0])
	}
}

func TestSkillsImportRejectsBadInput(t *testing.T) {
	handler, _ := newSkillsHandler(t, nil)
	empty := t.TempDir()
	file := filepath.Join(empty, "tep.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, body, want string
		status           int
	}{
		{"JSON hong", "{bad", "invalid JSON body", 400},
		{"thieu path", "{}", "'path' is required", 400},
		{"thu muc khong ton tai", skillsBody(t, map[string]any{"path": filepath.Join(empty, "khong-co")}), "folder not found", 400},
		{"khong phai thu muc", skillsBody(t, map[string]any{"path": file}), "'path' must be a folder", 400},
		{"thu muc khong co skill", skillsBody(t, map[string]any{"path": empty}), "no skill found", 400},
	}
	for _, item := range cases {
		recorder := call(handler, "POST", "/v1/skills/import", item.body, skillsHeaders())
		if recorder.Code != item.status || !strings.Contains(recorder.Body.String(), item.want) {
			t.Errorf("%s: -> %d %s (want %d, chua %q)", item.name, recorder.Code, recorder.Body.String(), item.status, item.want)
		}
	}
}

// Skill sai frontmatter: ca lan nhap bi tu choi, khong nhap mot phan.
func TestSkillsImportRefusesBrokenSkill(t *testing.T) {
	handler, paths := newSkillsHandler(t, nil)
	collection := t.TempDir()
	writeExternalSkill(t, collection, "tot", "name: tot\ndescription: on")
	writeExternalSkill(t, collection, "hong", "name: Ten Hong\ndescription: sai")

	recorder := call(handler, "POST", "/v1/skills/import", skillsBody(t, map[string]any{"path": collection}), skillsHeaders())
	if recorder.Code != 400 || !strings.Contains(recorder.Body.String(), "nothing was imported") {
		t.Fatalf("-> %d %s", recorder.Code, recorder.Body.String())
	}
	if entries, err := os.ReadDir(paths.SkillsDir); err == nil && len(entries) != 0 {
		t.Fatalf("khong duoc nhap mot phan: %v", entries)
	}
}
