package model

import "strings"

// HarnessProfile: cách HARNESS hành xử với một model, tách khỏi Profile (cách dựng client: auth, codec).
// Y tuong lay từ Deep Agents: provider profile chỉ lo kết nối; harness profile lo prompt/tool/ngân sách.
//
// Axiom chủ yếu chạy model free yếu (mimo, deepseek flash): cùng một prompt và danh sách lệnh đầy đủ
// tốn quá nhiều token với cửa sổ 8k–32k. Profile này cho phép chỉnh theo model mà không rải
// `if model == ...` khắp prompt/tool.
type HarnessProfile struct {
	// Provider/Model: khoá áp dụng. Model rỗng = mọi model của provider; cả hai rỗng = mặc định.
	Provider string
	Model    string

	// ContextBudgetTokens: ngân sách token cho các lượt TRƯỚC trong prompt (ContextAssembler).
	// 0 = suy ra từ ContextWindow (xem withDefaults).
	ContextBudgetTokens int
	// CompactToolDescriptions: bỏ phần hint dài trong chữ ký lệnh của office_action.
	// Giảm ~65–75% phần chữ ký (đo trên catalog: Word ~12,7k → ~4,1k ký tự).
	CompactToolDescriptions bool
	// SummarizeEvery: số tin nhắn giữa hai lần tóm tắt hội thoại. 0 = 20 (như trước).
	SummarizeEvery int
	// MaxRounds: trần vòng cho một lượt. 0 = mặc định của agent (100).
	MaxRounds int
}

const (
	DefaultContextBudgetTokens = 8000
	DefaultSummarizeEvery      = 20
	// compactContextWindow: model có cửa sổ ≤ 32k tự dùng mô tả gọn nếu chưa đăng ký profile riêng.
	compactContextWindow = 32_000
)

var harnessProfiles []HarnessProfile

// RegisterHarness: đăng ký/ghi đè một harness profile. Cùng (Provider, Model) thì bản mới thay bản cũ.
func RegisterHarness(profile HarnessProfile) {
	profile.Provider = strings.ToLower(strings.TrimSpace(profile.Provider))
	profile.Model = strings.TrimSpace(profile.Model)
	for index := range harnessProfiles {
		if harnessProfiles[index].Provider == profile.Provider && harnessProfiles[index].Model == profile.Model {
			harnessProfiles[index] = profile
			return
		}
	}
	harnessProfiles = append(harnessProfiles, profile)
}

// HarnessFor: profile khớp cụ thể nhất cho provider/model. Thứ tự: provider:model → provider → model → mặc định.
// Model có cửa sổ nhỏ mà chưa có profile riêng thì tự bật mô tả gọn và hạ ngân sách ngữ cảnh.
func HarnessFor(provider, modelName string, contextWindow int) HarnessProfile {
	provider = strings.ToLower(strings.TrimSpace(provider))
	modelName = strings.TrimSpace(modelName)
	for _, match := range []func(HarnessProfile) bool{
		func(p HarnessProfile) bool {
			return p.Provider == provider && p.Model == modelName && provider != "" && modelName != ""
		},
		func(p HarnessProfile) bool { return p.Provider == provider && p.Model == "" && provider != "" },
		func(p HarnessProfile) bool { return p.Provider == "" && p.Model == modelName && modelName != "" },
	} {
		for _, profile := range harnessProfiles {
			if match(profile) {
				return profile.withDefaults(contextWindow)
			}
		}
	}
	profile := HarnessProfile{}.withDefaults(contextWindow)
	if contextWindow > 0 && contextWindow <= compactContextWindow {
		profile.CompactToolDescriptions = true
	}
	return profile
}

// withDefaults: điền ngân sách theo cửa sổ ngữ cảnh. Cửa sổ càng nhỏ, ngân sách lượt trước càng nhỏ
// để chừa chỗ cho system prompt + tool + câu trả lời.
func (p HarnessProfile) withDefaults(contextWindow int) HarnessProfile {
	if p.ContextBudgetTokens <= 0 {
		switch {
		case contextWindow <= 0:
			p.ContextBudgetTokens = DefaultContextBudgetTokens
		case contextWindow < 16_000:
			p.ContextBudgetTokens = 2_000
		case contextWindow < 64_000:
			p.ContextBudgetTokens = 4_000
		default:
			p.ContextBudgetTokens = DefaultContextBudgetTokens
		}
	}
	if p.SummarizeEvery <= 0 {
		p.SummarizeEvery = DefaultSummarizeEvery
	}
	return p
}
