package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, folder, frontmatter, body string) string {
	t.Helper()
	directory := filepath.Join(root, folder)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\n" + strings.TrimSpace(frontmatter) + "\n---\n" + body
	if err := os.WriteFile(filepath.Join(directory, SkillFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestParseFrontmatterForms(t *testing.T) {
	// apps dang danh sach "- item", mo ta dang khoi '>'.
	text := "---\nname: quy-trinh\napps:\n  - et\n  - wps\ndescription: >\n  Dong mot\n  dong hai\n---\n# Buoc\nNoi dung"
	skill, failure := Parse(text, "/tmp/x", "org")
	if failure != nil {
		t.Fatalf("failure = %+v", failure)
	}
	if skill.Name != "quy-trinh" || strings.Join(skill.Apps, ",") != "et,wps" {
		t.Fatalf("skill = %+v", skill)
	}
	if skill.Description != "Dong mot dong hai" || !strings.Contains(skill.Body, "# Buoc") {
		t.Fatalf("description/body = %q / %q", skill.Description, skill.Body)
	}
	if !skill.AppliesTo("et") || !skill.AppliesTo("ET") || skill.AppliesTo("wpp") {
		t.Fatal("AppliesTo sai")
	}

	// apps dang [a, b], mo ta co nhay kep.
	inline, failure := Parse("---\nname: b\napps: [et]\ndescription: \"Mo ta \\\"x\\\"\"\n---\nthan", "/tmp/y", "user")
	if failure != nil || strings.Join(inline.Apps, ",") != "et" || inline.Description != `Mo ta "x"` {
		t.Fatalf("inline = %+v (%+v)", inline, failure)
	}

	// Skill khong ghi apps: ap dung cho moi app.
	everywhere, failure := Parse("---\nname: chung\ndescription: dung chung\n---\nx", "/tmp/z", "org")
	if failure != nil || !everywhere.AppliesTo("wpp") {
		t.Fatalf("skill khong co apps phai dung cho moi app: %+v (%+v)", everywhere, failure)
	}
}

func TestParseRejectsBadFrontmatter(t *testing.T) {
	cases := []struct{ name, text string }{
		{"thieu frontmatter", "khong phai frontmatter"},
		{"thieu name", "---\ndescription: mo ta\n---\nx"},
		{"ten viet hoa", "---\nname: Ten Hong\ndescription: mo ta\n---\nx"},
		{"ten co tu khoa danh rieng", "---\nname: claude-helper\ndescription: mo ta\n---\nx"},
		{"ten qua dai", "---\nname: " + strings.Repeat("a", 65) + "\ndescription: mo ta\n---\nx"},
		{"mo ta co the XML", "---\nname: ok\ndescription: <b>dam</b>\n---\nx"},
		{"thieu mo ta", "---\nname: ok\n---\nx"},
	}
	for _, item := range cases {
		if _, failure := Parse(item.text, "/tmp/z", "org"); failure == nil {
			t.Errorf("%s: phai bao loi", item.name)
		}
	}
}

func TestIndexSourcesAndOverrides(t *testing.T) {
	work := t.TempDir()
	builtin, org, user := filepath.Join(work, "builtin"), filepath.Join(work, "org"), filepath.Join(work, "user")
	writeSkill(t, builtin, "bang-diem", "name: bang-diem\ndescription: dung san", "than")
	writeSkill(t, builtin, "_design", "name: _design", "goi tai nguyen")
	writeSkill(t, org, "quy-trinh-rieng", "name: quy-trinh-rieng\napps: [et]\ndescription: cua to chuc", "than")
	writeSkill(t, org, "hong", "name: Ten Hong\ndescription: sai", "x")
	writeSkill(t, user, "bang-diem", "name: bang-diem\ndescription: ban nguoi dung de len", "than")

	index := New(DefaultSources(builtin, []string{org}, user))
	names := []string{}
	for _, skill := range index.All() {
		names = append(names, skill.Name)
	}
	if strings.Join(names, ",") != "bang-diem,quy-trinh-rieng" {
		t.Fatalf("All = %v", names)
	}
	if skill := index.Find("bang-diem"); skill == nil || skill.Description != "ban nguoi dung de len" || skill.Source != "user" {
		t.Fatalf("nguon sau phai ghi de nguon truoc: %+v", skill)
	}
	if index.ResourceDirectory("_design") == "" {
		t.Fatal("_design phai la goi tai nguyen")
	}
	if len(index.ForApp("et")) != 2 || len(index.ForApp("wpp")) != 1 {
		t.Fatalf("ForApp sai: et=%d wpp=%d", len(index.ForApp("et")), len(index.ForApp("wpp")))
	}
	if len(index.Errors()) != 1 {
		t.Fatalf("errors = %+v", index.Errors())
	}

	// Reload sau khi them skill moi.
	writeSkill(t, org, "skill-moi", "name: skill-moi\ndescription: moi", "than")
	index.Reload()
	if index.Find("skill-moi") == nil {
		t.Fatal("Reload phai thay skill moi")
	}
}

func TestResolveBlocksEscapes(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "a", "name: a\ndescription: d", "x")
	dir := filepath.Join(root, "a")
	references := filepath.Join(dir, "references")
	if err := os.MkdirAll(references, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(references, "the-thuc.md"), []byte("# Quy dinh"), 0o644); err != nil {
		t.Fatal(err)
	}

	if file, problem := Resolve(dir, "references/the-thuc.md"); file == "" || problem != "" {
		t.Fatalf("file hop le bi tu choi: %q %q", file, problem)
	}
	for _, path := range []string{"../ngoai.md", "/etc/passwd", "C:/windows/x.md", "", "references/../ngoai.md"} {
		if _, problem := Resolve(dir, path); problem == "" {
			t.Errorf("path %q phai bi tu choi", path)
		}
	}
	if _, problem := Resolve(dir, "references/script.sh"); !strings.Contains(problem, "not allowed") {
		t.Errorf("phan mo rong la phai bi tu choi, nhan duoc %q", problem)
	}
	if files := List(dir); len(files) != 1 || files[0] != "references/the-thuc.md" {
		t.Errorf("List = %v", files)
	}
}
