package agent

import "strings"

// Ngon ngu giao dien ma Core dung cho cac cau TRANG THAI tu gui len khung chat (P7-03). Cau tra loi cua model
// van theo ngon ngu nguoi dung go; o day chi la chu cua Core.
const (
	LanguageVietnamese = "vi"
	LanguageEnglish    = "en"
)

// NormalizeLanguage: "en", "en-US", "English" -> "en"; con lai (ke ca rong) -> "vi" (giu nguyen hanh vi cu).
func NormalizeLanguage(value string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "en") {
		return LanguageEnglish
	}
	return LanguageVietnamese
}

// localized: chon chu theo ngon ngu da chuan hoa.
func localized(language, vietnamese, english string) string {
	if NormalizeLanguage(language) == LanguageEnglish {
		return english
	}
	return vietnamese
}
