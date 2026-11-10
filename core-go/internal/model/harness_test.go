package model

import "testing"

// Model cửa sổ nhỏ tự nhận ngân sách nhỏ + mô tả tool gọn; model lớn giữ mặc định.
func TestHarnessForDerivesFromContextWindow(t *testing.T) {
	cases := []struct {
		name      string
		window    int
		budget    int
		compact   bool
		summaries int
	}{
		{"khong biet cua so", 0, 8000, false, 20},
		{"8k", 8_000, 2_000, true, 20},
		{"32k", 32_000, 4_000, true, 20},
		{"128k", 128_000, 8_000, false, 20},
	}
	for _, item := range cases {
		profile := HarnessFor("openai", "model-"+item.name, item.window)
		if profile.ContextBudgetTokens != item.budget || profile.CompactToolDescriptions != item.compact ||
			profile.SummarizeEvery != item.summaries {
			t.Fatalf("%s: %+v", item.name, profile)
		}
	}
}

// Profile đăng ký cụ thể thắng quy tắc tự động; profile theo provider áp cho mọi model của provider.
func TestRegisterHarnessPrecedence(t *testing.T) {
	RegisterHarness(HarnessProfile{Provider: "congty", ContextBudgetTokens: 1234, SummarizeEvery: 7})
	if got := HarnessFor("congty", "bat-ky", 8_000); got.ContextBudgetTokens != 1234 || got.SummarizeEvery != 7 {
		t.Fatalf("provider profile: %+v", got)
	}

	RegisterHarness(HarnessProfile{Provider: "congty", Model: "rieng", ContextBudgetTokens: 4321, CompactToolDescriptions: true})
	if got := HarnessFor("congty", "rieng", 8_000); got.ContextBudgetTokens != 4321 || !got.CompactToolDescriptions {
		t.Fatalf("model profile: %+v", got)
	}
	// Model khác của cùng provider vẫn dùng profile provider.
	if got := HarnessFor("congty", "khac", 8_000); got.ContextBudgetTokens != 1234 {
		t.Fatalf("provider fallback: %+v", got)
	}

	// Đăng ký lại cùng khoá thì ghi đè, không nhân đôi.
	RegisterHarness(HarnessProfile{Provider: "congty", Model: "rieng", ContextBudgetTokens: 9999})
	for _, profile := range harnessProfiles {
		if profile.Provider == "congty" && profile.Model == "rieng" {
			if profile.ContextBudgetTokens != 9999 {
				t.Fatalf("ghi de: %+v", profile)
			}
		}
	}
}
