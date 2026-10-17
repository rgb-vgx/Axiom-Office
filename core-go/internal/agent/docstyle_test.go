package agent

import "testing"

// M3-02: doc lai kieu trinh bay da nho tu cac dong ngu canh memory.
func TestRememberedDocStyle(t *testing.T) {
	lines := []string{
		"[ab12cd34] Người dùng là Trưởng phòng Kế toán",
		"[ef56ab78] Kiểu trình bày tài liệu ưa thích: full (Hiện đại, có khối nhấn) (tài liệu này)",
	}
	if got := rememberedDocStyle(lines); got != "full" {
		t.Fatalf("got %q", got)
	}
	if got := rememberedDocStyle(nil); got != "" {
		t.Fatalf("rong phai la \"\": %q", got)
	}
	if got := rememberedDocStyle([]string{"[x] kiểu trình bày tài liệu ưa thích:"}); got != "" {
		t.Fatalf("thieu id phai la \"\": %q", got)
	}
}
