package agent

import "strings"

// docStyleFactPrefix: dau cua fact ghi kieu trinh bay da chon (M3-02). Doc lai lan sau de xep lua chon do len dau
// the `Choose` (van hoi, khong tu ap).
const docStyleFactPrefix = "kiểu trình bày tài liệu ưa thích:"

// rememberedDocStyle: kieu trinh bay da nho trong ngu canh memory ("full"|"basic"|"keep"|"custom"; "" = chua co).
func rememberedDocStyle(lines []string) string {
	for _, line := range lines {
		index := strings.Index(strings.ToLower(line), docStyleFactPrefix)
		if index < 0 {
			continue
		}
		rest := strings.TrimSpace(line[index+len(docStyleFactPrefix):])
		if rest == "" {
			continue
		}
		return strings.Fields(rest)[0]
	}
	return ""
}
