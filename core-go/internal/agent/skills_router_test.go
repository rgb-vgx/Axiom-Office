package agent

import (
	"os"
	"path/filepath"
	"testing"

	"axiomoffice/core/internal/skills"
)

// Tang 1 cua chuan Agent Skills (SkillsSection) chi gui `skillDescriptionLimit` ky tu dau cua description;
// phan duoi KHONG toi model. Voi skill ma danh sach "Dung khi ..." nam o cuoi - thu duy nhat giup router
// khop dung viec - bi cat nghia la skill coi nhu vo hinh (docs/research/skills-review.md muc 4.9). Test nay giu
// moi description trong repo lot qua router.
func TestRepoSkillDescriptionsFitRouterLimit(t *testing.T) {
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

	seen := 0
	for _, skill := range index.All() {
		seen++
		// flatten(desc, len(desc)): dung chuoi router nhin thay (mot dong, gop het xuong dong) ma max
		// bang do dai nen khong bao gio bi cat - nho vay do duoc phan se mat.
		full := []rune(flatten(skill.Description, len([]rune(skill.Description))))
		t.Logf("%-22s %3d/%d", skill.Name, len(full), skillDescriptionLimit)
		if len(full) <= skillDescriptionLimit {
			continue
		}
		cut := string(full[skillDescriptionLimit:])
		if len([]rune(cut)) > 60 {
			cut = string([]rune(cut)[:60]) + "..."
		}
		t.Errorf("%s: description %d ky tu > %d, router cat mat %d ky tu cuoi (%q)",
			skill.Name, len(full), skillDescriptionLimit, len(full)-skillDescriptionLimit, cut)
	}
	if seen == 0 {
		t.Fatal("khong nap duoc skill nao - duong dan sai?")
	}
	t.Logf("%d skill: moi description deu lot qua router (%d ky tu)", seen, skillDescriptionLimit)
}
