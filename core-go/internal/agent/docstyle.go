package agent

import "strings"

// docStyleFactPrefix: dau cua fact ghi kieu trinh bay da chon (M3-02). Doc lai lan sau de xep lua chon do len dau
// the `Choose` (van hoi, khong tu ap).
const docStyleFactPrefix = "kiểu trình bày tài liệu ưa thích:"

// rememberedDocStyle: kieu trinh bay da nho trong ngu canh memory ("full"|"basic"|"keep"|"custom"; "" = chua co).
// Nhan nhieu nhom dong (memory thuong + nhom chuan trinh bay) vi doc-style nam trong nhom org-style tu 12-m3 muc 7.
func rememberedDocStyle(groups ...[]string) string {
	for _, lines := range groups {
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
	}
	return ""
}
