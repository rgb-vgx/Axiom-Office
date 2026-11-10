package agent

import (
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/skills"
)

// Phần ổn định (Base + quy tắc + skill) đi trước và cache được; tài liệu/memory/hội thoại nằm ở phần động.
func TestBuildPartsSplitsStableAndDynamic(t *testing.T) {
	parts := BuildParts("wps", &office.Session{App: "wps", Document: "a.docx"},
		&DocumentContext{Name: "a.docx"}, []skills.Definition{{Name: "van-ban", Description: "soan van ban"}},
		[]string{"nguoi dung thich font 13"}, nil, "tom tat", []string{"user: lam don"})

	if !strings.Contains(parts.Stable, "Microsoft Word / WPS Writer") ||
		!strings.Contains(parts.Stable, "Available skills") {
		t.Fatalf("stable thieu Base/skill: %q", parts.Stable)
	}
	for _, dynamic := range []string{"Open document", "font 13", "Earlier in this conversation"} {
		if strings.Contains(parts.Stable, dynamic) {
			t.Fatalf("stable khong duoc chua phan dong %q: %q", dynamic, parts.Stable)
		}
		if !strings.Contains(parts.Dynamic, dynamic) {
			t.Fatalf("dynamic thieu %q: %q", dynamic, parts.Dynamic)
		}
	}
	if got := parts.String(); !strings.HasPrefix(got, parts.Stable) || !strings.HasSuffix(got, parts.Dynamic) {
		t.Fatalf("String phai noi stable roi dynamic")
	}
}

func TestShouldSummarizeEvery(t *testing.T) {
	if ShouldSummarizeEvery(1, 20, 20) || !ShouldSummarizeEvery(1, 21, 20) {
		t.Fatal("mac dinh 20 tin nhan")
	}
	if ShouldSummarizeEvery(1, 8, 8) || !ShouldSummarizeEvery(1, 9, 8) {
		t.Fatal("chu ky 8")
	}
	if !ShouldSummarizeEvery(1, 2, 1) {
		t.Fatal("chu ky 1: moi tin nhan deu tom tat")
	}
}
