package skills

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// storeFixture: nguon builtin co san mot skill, nguon user rong (chua tao thu muc - giong may moi cai).
func storeFixture(t *testing.T) (*Store, string, string) {
	t.Helper()
	root := t.TempDir()
	builtin := filepath.Join(root, "core", "skills")
	user := filepath.Join(root, "data", "skills")
	writeSkill(t, builtin, "bang-diem", "name: bang-diem\ndescription: Lap bang diem\napps: [et]\nlicense: MIT", "# Bang diem\nCot STT")
	if err := os.MkdirAll(filepath.Join(builtin, "bang-diem", "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(builtin, "bang-diem", "references", "mau.md"), []byte("mau goc"), 0o644); err != nil {
		t.Fatal(err)
	}
	index := New([]Source{{Name: "builtin", Dir: builtin}, {Name: "user", Dir: user}})
	index.Reload()
	clock := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	return &Store{Index: index, Now: func() time.Time { clock = clock.Add(time.Second); return clock }}, builtin, user
}

// SKILL.md ghi lai phai nap duoc bang chinh bo doc, giu khoa la (license) va ky tu kho trong mo ta.
func TestComposeSkillRoundTripKeepsUnknownKeys(t *testing.T) {
	original := "---\nname: x\nlicense: MIT\ndescription: cu\nmetadata:\n  author: ai do\n---\nbody cu\n"
	edit := Edit{Description: `Dùng khi cần "báo cáo": a\b, c:d — ` + strings.Repeat("dài ", 40), Apps: []string{"wps", "et"},
		Body: "# Mới\r\nNội dung"}
	text, failure := composeSkill(original, "x", edit)
	if failure != nil {
		t.Fatal(failure)
	}
	skill, problem := Parse(text, "/x", "user")
	if problem != nil {
		t.Fatalf("khong nap lai duoc: %v\n%s", problem, text)
	}
	if skill.Description != strings.Join(strings.Fields(edit.Description), " ") {
		t.Fatalf("mo ta lech:\n%q\n%q", skill.Description, edit.Description)
	}
	if strings.Join(skill.Apps, ",") != "wps,et" || skill.Body != "# Mới\nNội dung" {
		t.Fatalf("apps/body lech: %v %q", skill.Apps, skill.Body)
	}
	if !strings.Contains(text, "license: MIT") || !strings.Contains(text, "metadata:\n  author: ai do") {
		t.Fatalf("mat khoa la:\n%s", text)
	}
	if strings.Index(text, "name:") > strings.Index(text, "license:") {
		t.Fatalf("phai giu thu tu khoa cu:\n%s", text)
	}
	// Bo het apps = dung cho moi app: khoa apps bien mat.
	text, _ = composeSkill(text, "x", Edit{Description: "ngan"})
	if strings.Contains(text, "apps:") {
		t.Fatalf("apps rong thi bo khoa:\n%s", text)
	}
	if _, failure := composeSkill("", "x", Edit{Description: "  "}); failure == nil || failure.Code != "invalid" {
		t.Fatalf("mo ta rong phai bi chan: %v", failure)
	}
	if _, failure := composeSkill("", "x", Edit{Description: "a", Apps: []string{"word"}}); failure == nil {
		t.Fatal("app la phai bi chan")
	}
}

// Sua skill san co: tao ban ca nhan che ban goc, ban goc khong doi; xoa ban ca nhan thi ban goc quay lai.
func TestSaveBuiltinMakesPersonalCopyAndDeleteRestores(t *testing.T) {
	store, builtin, user := storeFixture(t)
	detail, failure := store.Save("bang-diem", Edit{Description: "Bang diem cua toi", Apps: []string{"et"}, Body: "# Cua toi",
		Files: map[string]string{"references/them.md": "moi"}})
	if failure != nil {
		t.Fatal(failure)
	}
	if detail.Source != "user" || !detail.Overrides || detail.OriginalSource != "builtin" {
		t.Fatalf("phai la ban ca nhan che ban san co: %+v", detail)
	}
	if detail.Directory != filepath.Join(user, "bang-diem") {
		t.Fatalf("ban ca nhan sai cho: %s", detail.Directory)
	}
	original, _ := os.ReadFile(filepath.Join(builtin, "bang-diem", SkillFileName))
	if !strings.Contains(string(original), "Lap bang diem") {
		t.Fatal("ban goc bi sua")
	}
	// File tham chieu cua ban goc duoc chep sang; file moi duoc ghi; khoa la duoc giu.
	for _, path := range []string{"references/mau.md", "references/them.md"} {
		if _, err := os.Stat(filepath.Join(user, "bang-diem", filepath.FromSlash(path))); err != nil {
			t.Fatalf("thieu %s: %v", path, err)
		}
	}
	copied, _ := os.ReadFile(filepath.Join(user, "bang-diem", SkillFileName))
	if !strings.Contains(string(copied), "license: MIT") {
		t.Fatalf("mat khoa license:\n%s", copied)
	}
	if text, failure := store.ReadFile("bang-diem", "references/them.md"); failure != nil || text != "moi" {
		t.Fatalf("doc file: %q %v", text, failure)
	}

	trash, restored, failure := store.Delete("bang-diem")
	if failure != nil || !restored {
		t.Fatalf("xoa ban ca nhan: restored=%v %v", restored, failure)
	}
	if _, err := os.Stat(filepath.Join(trash, SkillFileName)); err != nil {
		t.Fatalf("ban ca nhan phai nam trong thung rac: %v", err)
	}
	if found := store.Index.Find("bang-diem"); found == nil || found.Source != "builtin" {
		t.Fatalf("ban goc phai quay lai: %+v", found)
	}
	if _, _, failure := store.Delete("bang-diem"); failure == nil || failure.Code != "not-user" {
		t.Fatalf("khong duoc xoa ban san co: %v", failure)
	}
}

func TestCreateValidatesAndRejectsDuplicates(t *testing.T) {
	store, _, user := storeFixture(t)
	if _, failure := store.Create(Edit{Name: "Bao Cao", Description: "x"}); failure == nil || !strings.Contains(failure.Message, "chữ thường") {
		t.Fatalf("ten sai phai bao bang tieng Viet: %v", failure)
	}
	if _, failure := store.Create(Edit{Name: "bang-diem", Description: "x"}); failure == nil || failure.Code != "exists" {
		t.Fatalf("trung ten skill san co: %v", failure)
	}
	detail, failure := store.Create(Edit{Name: "bao-cao-quy", Description: "Bao cao quy", Body: "# Quy"})
	if failure != nil || detail.Source != "user" || len(detail.Apps) != 0 {
		t.Fatalf("tao moi: %+v %v", detail, failure)
	}
	if _, err := os.Stat(filepath.Join(user, "bao-cao-quy", SkillFileName)); err != nil {
		t.Fatal(err)
	}
	if _, failure := store.Save("bao-cao-quy", Edit{Name: "ten-khac", Description: "x"}); failure == nil || failure.Code != "rename" {
		t.Fatalf("khong doi ten khi sua: %v", failure)
	}
	if _, failure := store.Save("bao-cao-quy", Edit{Description: "x", Files: map[string]string{"../ngoai.md": "x"}}); failure == nil {
		t.Fatal("file ngoai thu muc skill phai bi chan")
	}
	if _, failure := store.Save("bao-cao-quy", Edit{Description: "x", Files: map[string]string{"run.py": "x"}}); failure == nil {
		t.Fatal("duoi khong phai van ban phai bi chan")
	}
}

// Nhap thu muc chua nhieu skill: xem truoc khong ghi gi; nhap that bo file khong dung duoc (.py), danh dau xung dot.
func TestImportFolderPreviewThenInstall(t *testing.T) {
	store, _, user := storeFixture(t)
	pack := t.TempDir()
	writeSkill(t, pack, "hop-dong", "name: hop-dong\ndescription: Soan hop dong", "# Hop dong")
	if err := os.WriteFile(filepath.Join(pack, "hop-dong", "tao.py"), []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "hop-dong", "mau.md"), []byte("mau"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, pack, "bang-diem-moi", "name: bang-diem\ndescription: Ban khac", "# Khac")
	writeSkill(t, pack, "hong", "name: Hong Ten\ndescription: x", "")

	preview, failure := store.Import(pack, true, false)
	if failure != nil {
		t.Fatal(failure)
	}
	byName := map[string]ImportItem{}
	for _, item := range preview.Items {
		byName[item.Name] = item
	}
	if item := byName["hop-dong"]; item.Files != 1 || strings.Join(item.Skipped, ",") != "tao.py" || item.Conflict != "" {
		t.Fatalf("hop-dong: %+v", item)
	}
	if byName["bang-diem"].Conflict != "override" {
		t.Fatalf("trung ten skill san co phai la override: %+v", byName["bang-diem"])
	}
	if item := byName["hong"]; !strings.Contains(item.Error, "chữ thường") {
		t.Fatalf("skill hong phai co loi tieng Viet: %+v", preview.Items)
	}
	if _, err := os.Stat(user); !os.IsNotExist(err) {
		t.Fatal("xem truoc khong duoc ghi gi")
	}

	result, failure := store.Import(pack, false, false)
	if failure != nil {
		t.Fatal(failure)
	}
	imported := 0
	for _, item := range result.Items {
		if item.Imported {
			imported++
		}
	}
	if imported != 2 {
		t.Fatalf("phai nhap 2 skill hop le: %+v", result.Items)
	}
	if _, err := os.Stat(filepath.Join(user, "hop-dong", "tao.py")); !os.IsNotExist(err) {
		t.Fatal("file .py khong duoc chep")
	}
	if found := store.Index.Find("bang-diem"); found == nil || found.Source != "user" {
		t.Fatalf("ban nhap phai che ban san co: %+v", found)
	}
	// Nhap lai: xung dot "user", khong ghi de neu khong cho phep; cho phep thi ban cu vao thung rac.
	again, _ := store.Import(filepath.Join(pack, "hop-dong"), false, false)
	if again.Items[0].Conflict != "user" || again.Items[0].Imported {
		t.Fatalf("khong duoc ghi de khi chua cho phep: %+v", again.Items[0])
	}
	again, _ = store.Import(filepath.Join(pack, "hop-dong"), false, true)
	if !again.Items[0].Imported {
		t.Fatalf("overwrite phai nhap: %+v", again.Items[0])
	}
	trash, _ := filepath.Glob(filepath.Join(filepath.Dir(user), "skills-trash", "hop-dong-*"))
	if len(trash) != 1 {
		t.Fatalf("ban cu phai vao thung rac: %v", trash)
	}
	// Khong de lai thu muc tam.
	leftovers, _ := filepath.Glob(filepath.Join(user, ".import-*"))
	if len(leftovers) != 0 {
		t.Fatalf("con thu muc tam: %v", leftovers)
	}
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = entry.Write([]byte(content))
	}
	_ = writer.Close()
	_ = out.Close()
}

func TestImportZipAndRejectUnsafePaths(t *testing.T) {
	store, _, user := storeFixture(t)
	dir := t.TempDir()
	good := filepath.Join(dir, "hop-dong.skill")
	writeZip(t, good, map[string]string{
		"hop-dong/SKILL.md":            "---\nname: hop-dong\ndescription: Soan hop dong\n---\n# Hop dong\n",
		"hop-dong/references/mau.md":   "mau",
		"__MACOSX/hop-dong/._SKILL.md": "rac",
	})
	result, failure := store.Import(good, false, false)
	if failure != nil || len(result.Items) != 1 || !result.Items[0].Imported {
		t.Fatalf("nhap zip: %+v %v", result, failure)
	}
	if data, _ := os.ReadFile(filepath.Join(user, "hop-dong", "references", "mau.md")); string(data) != "mau" {
		t.Fatalf("file tham chieu: %q", data)
	}
	// SKILL.md o goc zip (khong co thu muc bao ngoai) cung nhap duoc.
	flat := filepath.Join(dir, "phang.zip")
	writeZip(t, flat, map[string]string{"SKILL.md": "---\nname: phang\ndescription: Goc zip\n---\nx\n"})
	if result, failure := store.Import(flat, false, false); failure != nil || !result.Items[0].Imported {
		t.Fatalf("zip phang: %+v %v", result, failure)
	}

	evil := filepath.Join(dir, "evil.zip")
	writeZip(t, evil, map[string]string{"x/SKILL.md": "---\nname: x\ndescription: x\n---\n", "x/../../thoat.md": "x"})
	if _, failure := store.Import(evil, false, false); failure == nil || failure.Code != "bad-zip" {
		t.Fatalf("zip-slip phai bi chan: %v", failure)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(user), "thoat.md")); !os.IsNotExist(err) {
		t.Fatal("khong duoc ghi ra ngoai")
	}
	if _, failure := store.Import(filepath.Join(dir, "khong-co.zip"), true, false); failure == nil || failure.Code != "bad-path" {
		t.Fatalf("duong dan sai: %v", failure)
	}
	empty := filepath.Join(dir, "rong.zip")
	writeZip(t, empty, map[string]string{"readme.txt": "x"})
	if _, failure := store.Import(empty, true, false); failure == nil || failure.Code != "empty" {
		t.Fatalf("goi khong co SKILL.md: %v", failure)
	}
}

// Thu muc ca nhan cung ten da co nhung hong (khong nap duoc): tao moi / luu ban rieng khong duoc ghi de mu.
func TestBrokenPersonalFolderIsNeverOverwritten(t *testing.T) {
	store, _, user := storeFixture(t)
	writeSkill(t, user, "bang-diem", "name: Sai Ten\ndescription: x", "giu nguyen")
	writeSkill(t, user, "moi-hong", "description: thieu ten", "giu nguyen")
	store.Index.Reload()
	if _, failure := store.Save("bang-diem", Edit{Description: "y"}); failure == nil || failure.Code != "exists" {
		t.Fatalf("luu ban rieng de len thu muc hong: %v", failure)
	}
	if _, failure := store.Create(Edit{Name: "moi-hong", Description: "y"}); failure == nil || failure.Code != "exists" {
		t.Fatalf("tao moi de len thu muc hong: %v", failure)
	}
	for _, folder := range []string{"bang-diem", "moi-hong"} {
		data, _ := os.ReadFile(filepath.Join(user, folder, SkillFileName))
		if !strings.Contains(string(data), "giu nguyen") {
			t.Fatalf("%s bi ghi de: %s", folder, data)
		}
	}
}
