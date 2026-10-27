package agent

import (
	"regexp"

	"axiomoffice/core/internal/mcp"
	"axiomoffice/core/internal/store"
)

// Chi gui tool khi luot co the can toi no (progressive disclosure, docs/research/hoc-tu-agent-khac.md): dinh nghia tool
// chiem ~79% tien to gui lai MOI vong (do 27/10/2026: ~21-22k ky tu moi app). Bo tool chot MOT lan o dau luot va khong doi
// giua luot - them/bot tool giua chung pha cache tien to (bai hoc cua Claude Code). Chi an tool HIEM dung: an tool can
// dung thi model phai them mot vong de lam cach khac, dat hon phan tiet kiem.

// officeFileRef: ten/duong dan tep Office ma tool file MCP doc hoac tao (.docx/.xlsx/.pptx, .xls/.csv/.tsv cho
// excel_convert/excel_read). Pane gui tep dinh kem bang duong dan day du trong prompt ("[Attached files ...]").
var officeFileRef = regexp.MustCompile(`(?i)\.(docx|dotx|xlsx|xlsm|xltx|xls|csv|tsv|pptx|potx)\b`)

// mentionsOfficeFile: luot nay (hoac mot cau nguoi dung truoc do trong hoi thoai - "doc tiep file do") nhac toi mot tep
// Office tren dia.
func mentionsOfficeFile(prompt string, earlier []store.MessageRow) bool {
	if officeFileRef.MatchString(prompt) {
		return true
	}
	for _, message := range earlier {
		if message.Role == "user" && officeFileRef.MatchString(message.Content) {
			return true
		}
	}
	return false
}

// withoutOfficeFileTools: bo tool file cua server built-in "office" (doc_*, excel_*, ppt_* lam tren tep DONG) - luot sua
// tai lieu dang mo khong can chung. Tool cua server nguoi dung tu cau hinh giu nguyen.
func withoutOfficeFileTools(list []mcp.ServerTool) []mcp.ServerTool {
	kept := []mcp.ServerTool{}
	for _, item := range list {
		if item.Server != nil && item.Server.Config.Name == "office" {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

// webSourceWords: cau nguoi dung nhac toi nguon web / tra cuu - de dung lai nguon da luu tu hoi thoai khac.
var webSourceWords = regexp.MustCompile(`(?i)https?://|\b(web|internet|online|source|sources|url|link)\b|nguồn|tìm kiếm|tra cứu|trích dẫn|bài báo`)

// needsWebSources: tool web_sources (doc lai trang web da luu) chi co ich khi luot nay co tim web, hoi thoai nay da co nguon
// luu, hoac nguoi dung nhac toi nguon/web.
func needsWebSources(webSearch bool, conversationHasSources bool, prompt string) bool {
	return webSearch || conversationHasSources || webSourceWords.MatchString(prompt)
}
