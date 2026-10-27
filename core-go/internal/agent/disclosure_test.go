package agent

import (
	"testing"

	"axiomoffice/core/internal/mcp"
	"axiomoffice/core/internal/store"
)

func TestMentionsOfficeFile(t *testing.T) {
	attached := "tom tat giup\n\n[Attached files - read them ...]\n- C:\\Users\\a\\Bao cao Q3.XLSX"
	cases := []struct {
		prompt  string
		earlier []store.MessageRow
		want    bool
	}{
		{"sua lai doan mo dau cho gon", nil, false},
		{attached, nil, true},
		{"tao file danh-sach.csv tu bang nay", nil, true},
		{"doc tiep file do", []store.MessageRow{{Role: "user", Content: attached}}, true},
		// Cau tra loi cua AI nhac ten tep khong tinh: chi cau nguoi dung moi la yeu cau doc tep.
		{"lam tiep", []store.MessageRow{{Role: "assistant", Content: "Da luu vao Book1.xlsx"}}, false},
		{"them muc 2.docxyz", nil, false},
	}
	for _, c := range cases {
		if got := mentionsOfficeFile(c.prompt, c.earlier); got != c.want {
			t.Errorf("mentionsOfficeFile(%q) = %v, muon %v", c.prompt, got, c.want)
		}
	}
}

func TestWithoutOfficeFileToolsKeepsUserServers(t *testing.T) {
	office := &mcp.Server{Config: mcp.ServerConfig{Name: "office"}}
	user := &mcp.Server{Config: mcp.ServerConfig{Name: "crm"}}
	list := []mcp.ServerTool{
		{Server: office, Tool: mcp.ToolInfo{Name: "excel_read"}},
		{Server: user, Tool: mcp.ToolInfo{Name: "lookup"}},
		{Server: office, Tool: mcp.ToolInfo{Name: "doc_get_text"}},
	}
	kept := withoutOfficeFileTools(list)
	if len(kept) != 1 || kept[0].Server != user {
		t.Fatalf("chi bo tool cua server built-in office, giu server nguoi dung: %+v", kept)
	}
}

func TestNeedsWebSources(t *testing.T) {
	cases := []struct {
		search, has bool
		prompt      string
		want        bool
	}{
		{false, false, "dinh dang lai bang", false},
		{true, false, "dinh dang lai bang", true},
		{false, true, "viet tiep muc 3", true},
		{false, false, "dùng lại nguồn đã tìm hôm trước", true},
		{false, false, "trích dẫn bài https://example.com/a", true},
		{false, false, "chèn link tới trang chủ", true},
		{false, false, "chèn bảng tổng kết", false},
	}
	for _, c := range cases {
		if got := needsWebSources(c.search, c.has, c.prompt); got != c.want {
			t.Errorf("needsWebSources(%v, %v, %q) = %v, muon %v", c.search, c.has, c.prompt, got, c.want)
		}
	}
}
