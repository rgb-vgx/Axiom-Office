package model

import "strings"

// Ho API (family) ma mot nha cung cap dung.
const (
	FamilyOpenAI    = "openai"
	FamilyAnthropic = "anthropic"
)

// Profile: khai bao mot nha cung cap (P4-01). Them nha cung cap/model moi chi can them mot dong o `profiles`
// (hoac RegisterProfile cho nha cung cap khach) - khong phai sua client.go/types.go.
type Profile struct {
	// Name: ten provider nhu trong cau hinh (openai, anthropic, gemini, company...) hoac ten codec.
	Name string
	// Family: ho API (FamilyOpenAI | FamilyAnthropic) - quyet dinh codec nao duoc dung.
	Family string
	// AuthHeader / AuthPrefix: cach gui khoa. Anthropic: x-api-key (khong prefix); con lai: Authorization + Bearer.
	AuthHeader string
	AuthPrefix string
	// ExtraHeaders: header rieng cua nha cung cap (vd anthropic-version).
	ExtraHeaders map[string]string
	// SupportsThinking: nhan tham so `thinking` kieu OpenAI (bat phan suy luan). Anthropic co `thinking` khac dang
	// nen gia tri nay la false (client.go chi gui `thinking` khi profile cho phep).
	SupportsThinking bool
	// SupportsTemperature: nhan tham so `temperature`.
	SupportsTemperature bool
}

// defaultProfile: OpenAI-compatible - dung cho moi ten chua khai bao.
var defaultProfile = Profile{
	Name: "openai", Family: FamilyOpenAI,
	AuthHeader: "Authorization", AuthPrefix: "Bearer ",
	SupportsThinking: true, SupportsTemperature: true,
}

var profiles = map[string]Profile{
	"openai": defaultProfile,
	// Gemini va cac preset OpenAI-compatible (catalog/setup.json) di qua gateway kieu OpenAI.
	"gemini":  {Name: "gemini", Family: FamilyOpenAI, AuthHeader: "Authorization", AuthPrefix: "Bearer ", SupportsThinking: true, SupportsTemperature: true},
	"company": {Name: "company", Family: FamilyOpenAI, AuthHeader: "Authorization", AuthPrefix: "Bearer ", SupportsThinking: true, SupportsTemperature: true},
	"anthropic": {
		Name: "anthropic", Family: FamilyAnthropic,
		AuthHeader: "x-api-key", AuthPrefix: "",
		ExtraHeaders:     map[string]string{"anthropic-version": "2023-06-01"},
		SupportsThinking: false, SupportsTemperature: true,
	},
}

// ProfileFor: profile cua mot provider. Ten la khong khai bao -> mac dinh OpenAI-compatible.
func ProfileFor(provider string) Profile {
	if profile, ok := profiles[strings.ToLower(strings.TrimSpace(provider))]; ok {
		return profile
	}
	return defaultProfile
}

// RegisterProfile: dang ky/ghi de mot profile (nha cung cap moi, hoac test).
func RegisterProfile(profile Profile) {
	if name := strings.ToLower(strings.TrimSpace(profile.Name)); name != "" {
		profiles[name] = profile
	}
}
