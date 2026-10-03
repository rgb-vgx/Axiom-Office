package tools

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"axiomoffice/core/internal/skills"
)

// Skill trong repo (skills/ o goc) phai nap duoc, va moi cho SKILL.md bao model goi
// `read_skill_file {name: "...", path: "..."}` phai tro toi file CO THAT - mot tham chieu treo la
// model mat mot vong goi tool roi lam tiep khong co huong dan (vd references/du-lieu-gia-lap.md).
var skillFileMention = regexp.MustCompile(`read_skill_file \{name: "([^"]+)", path: "([^"]+)"\}`)

func TestRepoSkillsLoadAndReferencesResolve(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("khong thay skills/ cua repo: %v", err)
	}
	index := skills.New([]skills.Source{{Name: "builtin", Dir: root}})
	if failures := index.Errors(); len(failures) > 0 {
		t.Fatalf("skill loi khi nap: %+v", failures)
	}
	all := index.All()
	if len(all) == 0 {
		t.Fatal("khong nap duoc skill nao")
	}

	mentions := 0
	for _, skill := range all {
		text, err := os.ReadFile(filepath.Join(skill.Directory, "SKILL.md"))
		if err != nil {
			t.Fatalf("%s: %v", skill.Name, err)
		}
		app := "et"
		if len(skill.Apps) > 0 {
			app = skill.Apps[0]
		}
		read := NewReadSkillFileTool(index, app)
		for _, match := range skillFileMention.FindAllStringSubmatch(string(text), -1) {
			mentions++
			result := read.Invoke(context.Background(),
				map[string]any{"name": match[1], "path": match[2]}, &RunContext{})
			if !result.OK {
				t.Errorf("%s/SKILL.md tro toi %s/%s nhung read_skill_file loi: %.200s",
					skill.Name, match[1], match[2], result.JSON)
			}
		}
	}
	if mentions == 0 {
		t.Fatal("khong tim thay cho nao goi read_skill_file - regex sai?")
	}
	t.Logf("%d skill, %d tham chieu read_skill_file deu doc duoc", len(all), mentions)
}
