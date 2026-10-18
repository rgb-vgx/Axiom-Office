package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func names(candidates []Candidate) []string {
	list := []string{}
	for _, candidate := range candidates {
		list = append(list, candidate.Name)
	}
	return list
}

func TestInspectSingleAndCollection(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "skills")

	// Thu muc chinh no la mot skill.
	single := writeSkill(t, filepath.Join(work, "ngoai"), "skill-cua-toi", "name: skill-cua-toi\ndescription: do nguoi dung viet", "than")
	candidates, failures, err := Inspect(single, dest)
	if err != nil || len(failures) != 0 {
		t.Fatalf("Inspect single = %v / %v / %v", names(candidates), failures, err)
	}
	if len(candidates) != 1 || candidates[0].Name != "skill-cua-toi" {
		t.Fatalf("candidates = %v", names(candidates))
	}
	if candidates[0].Source != SourceNameExternal || candidates[0].Conflict {
		t.Fatalf("candidate = %+v", candidates[0])
	}
	if want := filepath.Join(dest, "skill-cua-toi"); candidates[0].TargetDir != want {
		t.Fatalf("TargetDir = %q, want %q", candidates[0].TargetDir, want)
	}

	// Thu muc chua nhieu skill: _design va skill sai frontmatter khong lam hong ca lan soi.
	collection := filepath.Join(work, "gop")
	writeSkill(t, collection, "bang-diem", "name: bang-diem\ndescription: mot", "than")
	writeSkill(t, collection, "bao-cao", "name: bao-cao\napps: [et]\ndescription: hai", "than")
	writeSkill(t, collection, "_design", "name: _design", "goi tai nguyen")
	writeSkill(t, collection, "hong", "name: Ten Hong\ndescription: sai", "x")
	if err := os.MkdirAll(filepath.Join(collection, "khong-phai-skill"), 0o755); err != nil {
		t.Fatal(err)
	}

	candidates, failures, err = Inspect(collection, dest)
	if err != nil {
		t.Fatalf("Inspect collection: %v", err)
	}
	if got := strings.Join(names(candidates), ","); got != "bang-diem,bao-cao" {
		t.Fatalf("candidates = %q", got)
	}
	if len(failures) != 1 || !strings.Contains(failures[0].Error, "lowercase") {
		t.Fatalf("failures = %+v", failures)
	}

	// Trung ten trong cung mot lan soi.
	writeSkill(t, collection, "trung-ten", "name: bao-cao\ndescription: trung", "x")
	_, failures, err = Inspect(collection, dest)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, failure := range failures {
		if strings.Contains(failure.Error, "duplicate skill name") {
			found = true
		}
	}
	if !found {
		t.Fatalf("phai bao trung ten: %+v", failures)
	}

	// Da co san o noi nhan -> Conflict.
	writeSkill(t, dest, "bao-cao", "name: bao-cao\ndescription: ban cu", "x")
	candidates, _, err = Inspect(collection, dest)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.Name == "bang-diem" && candidate.Conflict {
			t.Fatal("bang-diem chua co o noi nhan, khong duoc coi la trung")
		}
	}
}

func TestInspectRejectsBadPaths(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "skills")

	file := filepath.Join(work, "tep.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(work, "rong")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct{ name, path, want string }{
		{"rong", "   ", "'path' is required"},
		{"khong ton tai", filepath.Join(work, "khong-co"), "folder not found"},
		{"khong phai thu muc", file, "'path' must be a folder"},
		{"khong co skill", empty, "no skill found"},
	}
	for _, item := range cases {
		if _, _, err := Inspect(item.path, dest); err == nil || !strings.Contains(err.Error(), item.want) {
			t.Errorf("%s: err = %v, want chua %q", item.name, err, item.want)
		}
	}
}

func TestImportCopiesTreeAndIndexSeesIt(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "AxiomOffice", "skills")
	source := writeSkill(t, filepath.Join(work, "ngoai"), "the-thuc-rieng", "name: the-thuc-rieng\ndescription: cua toi", "# Than")

	// File tham chieu duoc copy; .git bi bo qua.
	references := filepath.Join(source, "references")
	if err := os.MkdirAll(references, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(references, "mau.md"), []byte("# Mau"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "config"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Import(source, dest, false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if strings.Join(result.Imported, ",") != "the-thuc-rieng" || len(result.Replaced) != 0 {
		t.Fatalf("result = %+v", result)
	}

	installed := filepath.Join(dest, "the-thuc-rieng")
	if _, err := os.Stat(filepath.Join(installed, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md phai duoc copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installed, "references", "mau.md")); err != nil {
		t.Fatalf("references phai duoc copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installed, ".git")); err == nil {
		t.Fatal(".git khong duoc copy")
	}
	// Thu muc tam khong duoc de lai.
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), ".import")); err == nil {
		t.Fatal("thu muc tam phai duoc don")
	}

	index := New([]Source{{Name: "user", Dir: dest}})
	if skill := index.Find("the-thuc-rieng"); skill == nil || skill.Source != "user" {
		t.Fatalf("chi muc phai thay skill vua nhap: %+v", skill)
	}
}

func TestImportConflictAndOverwrite(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "skills")
	first := writeSkill(t, filepath.Join(work, "lan-mot"), "quy-trinh", "name: quy-trinh\ndescription: ban mot", "x")
	if _, err := Import(first, dest, false); err != nil {
		t.Fatalf("Import dau: %v", err)
	}

	// Nhap lai khi chua cho phep ghi de: tu choi va giu nguyen ban cu.
	second := writeSkill(t, filepath.Join(work, "lan-hai"), "quy-trinh", "name: quy-trinh\ndescription: ban hai", "x")
	if _, err := Import(second, dest, false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("phai tu choi khi trung ten: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(dest, "quy-trinh", SkillFileName)); err != nil || !strings.Contains(string(content), "ban mot") {
		t.Fatalf("ban cu phai duoc giu: %q %v", content, err)
	}

	// overwrite=true: thay bang ban moi.
	result, err := Import(second, dest, true)
	if err != nil {
		t.Fatalf("Import ghi de: %v", err)
	}
	if strings.Join(result.Replaced, ",") != "quy-trinh" || strings.Join(result.Imported, ",") != "quy-trinh" {
		t.Fatalf("result = %+v", result)
	}
	if content, err := os.ReadFile(filepath.Join(dest, "quy-trinh", SkillFileName)); err != nil || !strings.Contains(string(content), "ban hai") {
		t.Fatalf("phai la ban moi: %q %v", content, err)
	}
}

func TestImportIsAllOrNothing(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "skills")
	collection := filepath.Join(work, "gop")
	writeSkill(t, collection, "tot", "name: tot\ndescription: on", "x")
	writeSkill(t, collection, "hong", "name: Ten Hong\ndescription: sai", "x")

	if _, err := Import(collection, dest, false); err == nil || !strings.Contains(err.Error(), "nothing was imported") {
		t.Fatalf("phai tu choi ca lan nhap: %v", err)
	}
	if entries, err := os.ReadDir(dest); err == nil && len(entries) != 0 {
		t.Fatalf("khong duoc nhap mot phan: %v", entries)
	}

	// Mot skill trong nhom trung ten: cung tu choi ca lan.
	writeSkill(t, collection, "hong", "name: tot\ndescription: trung ten", "x")
	if _, err := Import(collection, dest, false); err == nil || !strings.Contains(err.Error(), "duplicate skill name") {
		t.Fatalf("phai tu choi khi trung ten trong nhom: %v", err)
	}
	if entries, err := os.ReadDir(dest); err == nil && len(entries) != 0 {
		t.Fatalf("khong duoc nhap mot phan: %v", entries)
	}
}

func TestImportRefusesSymlinks(t *testing.T) {
	work := t.TempDir()
	dest := filepath.Join(work, "skills")
	outside := filepath.Join(work, "ngoai.md")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := writeSkill(t, filepath.Join(work, "src"), "co-link", "name: co-link\ndescription: d", "x")
	if err := os.Symlink(outside, filepath.Join(source, "link.md")); err != nil {
		t.Skipf("may nay khong tao duoc symlink: %v", err)
	}
	if _, err := Import(source, dest, false); err == nil || !strings.Contains(err.Error(), "symbolic links") {
		t.Fatalf("symlink phai bi tu choi: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "co-link")); err == nil {
		t.Fatal("bi tu choi thi khong duoc copy gi")
	}
}
