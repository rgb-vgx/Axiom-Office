package benchflags

import "testing"

func TestVariants(t *testing.T) {
	cases := []struct {
		variant string
		want    map[string]bool
	}{
		{"B0", map[string]bool{}},
		{"B1", map[string]bool{Harness: true}},
		{"B2", map[string]bool{Compact: true}},
		{"B3", map[string]bool{Prompt: true}},
		{"B4", map[string]bool{Checklist: true}},
		{"B5", map[string]bool{Results: true}},
		{"B6", map[string]bool{Harness: true, Compact: true, Prompt: true, Checklist: true, Results: true}},
	}
	features := []string{Harness, Compact, Prompt, Checklist, Results}
	for _, item := range cases {
		t.Setenv(Env, item.variant)
		for _, feature := range features {
			if got := Enabled(feature); got != item.want[feature] {
				t.Fatalf("%s/%s = %v, muon %v", item.variant, feature, got, item.want[feature])
			}
		}
		if CompactAuto() {
			t.Fatalf("%s: compact phai do nhanh quyet dinh", item.variant)
		}
	}
}

func TestProductionDefaults(t *testing.T) {
	t.Setenv(Env, "")
	for _, feature := range []string{Harness, Compact, Prompt, Checklist, Results} {
		if !Enabled(feature) {
			t.Fatalf("production phai bat %s", feature)
		}
	}
	if !CompactAuto() || Summary() != "production" {
		t.Fatalf("production: auto=%v summary=%q", CompactAuto(), Summary())
	}
}

// Gõ sai nhánh: giữ production thay vì âm thầm tắt hết.
func TestUnknownVariantKeepsProduction(t *testing.T) {
	t.Setenv(Env, "b9-typo")
	if !Enabled(Harness) || !Enabled(Results) || !CompactAuto() {
		t.Fatal("nhanh la phai coi nhu production")
	}
}
