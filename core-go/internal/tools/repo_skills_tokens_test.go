package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Token thiet ke (skills/_design/tokens.json) la DU LIEU, khong phai van ban de chep lai: file tham chieu
// chi duoc ghi TEN token, de sua tokens.json la moi cho theo (docs/architecture/core.md muc 8.4.6). Do 11/10/2026
// (docs/research/skills-review.md muc 4.1): bo-cuc.md chep cung co chu (40/32/18) va mau (#1F4E79, #595959...)
// lech han tokens.json, nen bao-cao-thang va trinh-bay-chuyen-nghiep dung cung nhau ra hai co tieu de
// khac nhau - va sua tokens.json khong lan toi bo-cuc.md.
var (
	hexColor     = regexp.MustCompile(`#[0-9A-Fa-f]{6}\b`)
	bareFontSize = regexp.MustCompile(`fontSize:\s*[0-9]`)
	tokenName    = regexp.MustCompile("`((?:colors|fonts|slide|document|spreadsheet)\\.[A-Za-z0-9_.]+)`")
)

func repoSkillsRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("khong thay skills/ cua repo: %v", err)
	}
	return root
}

// resolveToken: "slide.type.hero" -> tokens["slide"]["type"]["hero"] co that.
func resolveToken(tokens map[string]any, path string) bool {
	var current any = tokens
	for part := range strings.SplitSeq(path, ".") {
		block, ok := current.(map[string]any)
		if !ok {
			return false
		}
		next, ok := block[part]
		if !ok {
			return false
		}
		current = next
	}
	return true
}

func TestSkillReferencesDoNotHardcodeDesignValues(t *testing.T) {
	root := repoSkillsRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "_design", "tokens.json"))
	if err != nil {
		t.Fatalf("doc tokens.json: %v", err)
	}
	var tokens map[string]any
	if err := json.Unmarshal(raw, &tokens); err != nil {
		t.Fatalf("tokens.json khong phai JSON hop le: %v", err)
	}

	references, err := filepath.Glob(filepath.Join(root, "*", "references", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(references) == 0 {
		t.Fatal("khong thay file references/*.md nao - glob sai?")
	}

	names := 0
	for _, path := range references {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		name, _ := filepath.Rel(root, path)
		if found := hexColor.FindString(text); found != "" {
			t.Errorf("%s chep cung mau %s: ghi ten token (colors.*) thay vi gia tri", name, found)
		}
		if found := bareFontSize.FindString(text); found != "" {
			t.Errorf("%s chep cung co chu (%q): ghi ten token (slide.type.*) thay vi so", name, found)
		}
		for _, match := range tokenName.FindAllStringSubmatch(text, -1) {
			names++
			if !resolveToken(tokens, match[1]) {
				t.Errorf("%s nhac token '%s' nhung tokens.json khong co duong do", name, match[1])
			}
		}
	}
	if names == 0 {
		t.Fatal("khong thay ten token nao trong references/*.md - regex sai?")
	}
	t.Logf("%d file references, %d ten token deu co that trong tokens.json", len(references), names)
}
