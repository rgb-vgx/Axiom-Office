package tools

import (
	"fmt"
	"strings"
)

// CallLimits: so lan toi da moi luot cho mot so tool/lenh hay bi goi thua (tu van ChatGPT 13/10/2026, LangChain
// ToolCallLimitMiddleware). Luot that 13/10/2026: sau khi dung xong 95 slide, model goi wpp.sourceManifest 4 lan
// (moi lan mot khoi chu lon), checkLayout tung slide va setNotes 97 lan -> het ngan sach token. Lenh ghi du lieu
// (et.writeRange...) KHONG co han muc: bang lon can goi nhieu lan theo khoi.
var CallLimits = map[string]int{
	"wpp.sourceManifest":        3,
	"wpp.listSlides":            6,
	"wpp.checkLayout":           20,
	"wpp.setNotes":              20,
	"slides_from_template":      20,
	"web_search":                5,
	"write_todos":               30,
	"mcp__office__ppt_get_text": 4,
	"mcp__office__ppt_profile":  4,
	"mcp__office__doc_get_text": 4,
}

// CountCall: dem mot lan goi `key` (ten tool, hoac ten lenh cua office_action); vuot han muc -> thong bao loi.
func (c *RunContext) CountCall(key string) string {
	limit, limited := CallLimits[key]
	if !limited {
		return ""
	}
	if c.calls == nil {
		c.calls = map[string]int{}
	}
	if c.calls[key] >= limit {
		hint := "use the results you already have"
		if strings.HasPrefix(key, "wpp.") || key == "slides_from_template" {
			hint = "slides_from_template already checks layout and sets notes; answer the user with what is done"
		}
		return fmt.Sprintf("limit reached: %s may run at most %d times per request (%s)", key, limit, hint)
	}
	c.calls[key]++
	return ""
}
