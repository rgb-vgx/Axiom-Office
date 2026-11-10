package tools

import (
	"context"
	"encoding/json"
	"strings"

	"axiomoffice/core/internal/mcp"
)

// MCPTool: mot tool MCP cho model: ten mcp__<server>__<tool>; server khong tin cay -> hoi nguoi dung truoc
// moi lan goi; server office (tin cay) -> hoi khi tool GHI vao file DA CO (cung quy tac voi saveAs).
type MCPTool struct{ bound mcp.ServerTool }

// WrapMCPTools: boc cac tool MCP thanh tool cua agent.
func WrapMCPTools(list []mcp.ServerTool) []Tool {
	wrapped := make([]Tool, 0, len(list))
	for _, item := range list {
		wrapped = append(wrapped, &MCPTool{bound: item})
	}
	return wrapped
}

// ReadOnlyMCP: cong cu MCP duoc dung trong che do lap ke hoach - chi cong cu DOC FILE cua server built-in
// "office" (mcp.OfficeReadOnlyTools: doc file .docx/.xlsx/.pptx dinh kem). Server nguoi dung tu cau hinh khong
// biet cong cu nao ghi -> bo het.
func ReadOnlyMCP(list []mcp.ServerTool) []mcp.ServerTool {
	kept := []mcp.ServerTool{}
	for _, item := range list {
		if item.Server != nil && item.Server.Config.Name == "office" && mcp.OfficeReadOnlyTools[item.Tool.Name] {
			kept = append(kept, item)
		}
	}
	return kept
}

// AppMCP: cong cu MCP cho mot lan chay that - server built-in "office" chi con tool CUA app dang lam (doc_
// cho wps, excel_ cho et, ppt_ cho wpp - doc va ghi deu giu) cong lenh DOC FILE cua app khac (doc file
// .xlsx doi chieu so lieu trong bai Word, doc file .docx trich doan van trong bai Excel); lenh GHI cua app
// khac thi bo - luot nay khong bao gio dung chung, ma moi dinh nghia tool gui lai moi vong deu la tien to
// phai tra (~8k ky tu - bao cao "Giam ngu canh gui lai agent", muc 3). Server nguoi dung giu nguyen (khong
// du doan cong cu nao ghi cua ho). Thu tu giu nguyen - bo loc khong lam doi thu tu tool nen bo hop tool co
// dinh theo app, khong pha cache (bai hoc Claude Code: khong thay doi bo tool giua mot phien).
func AppMCP(list []mcp.ServerTool, appKind string) []mcp.ServerTool {
	kept := []mcp.ServerTool{}
	for _, item := range list {
		if item.Server == nil || item.Server.Config.Name != "office" {
			kept = append(kept, item)
			continue
		}
		if mcp.FileToolApp(item.Tool.Name) == appKind || mcp.OfficeReadOnlyTools[item.Tool.Name] {
			kept = append(kept, item)
		}
	}
	return kept
}

func (t *MCPTool) Name() string { return t.bound.Name() }

func (t *MCPTool) Description() string { return t.bound.Description() }

func (t *MCPTool) Parameters() json.RawMessage { return t.bound.Schema() }

func (t *MCPTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	name := t.bound.Name()
	if reason := mcp.NeedsConfirm(t.bound, arguments); reason != "" {
		preview := truncateJSON(arguments)
		approved, err := run.ConfirmOrDecline(name, reason, preview)
		if err != nil || !approved {
			return Result{JSON: ErrorJSON("user declined: " + reason), Action: name}
		}
	}

	text, ok := t.bound.Call(ctx, arguments)
	// Trailer AXIOM_WEB (tool chrome-search): tach nguon de luu kho + danh dau nguon cho luot TRUOC khi
	// ket qua vao ngu canh model. Khong co trailer thi ket qua giu nguyen.
	if ok {
		if clean, trailer := ParseWebTrailer(text); trailer != nil {
			text = clean
			for _, doc := range trailer.Sources {
				run.noteWebSource(doc.URL, doc.Title)
			}
			run.SaveWebSources(trailer.Kind, trailer.Query, trailer.Sources)
		}
	}
	return Result{JSON: mcp.ResultJSON(text, ok), OK: ok, Action: name,
		Mutating: ok && mcpToolChangesState(t.bound)}
}

// mcpToolChangesState: name() la ten day du "mcp__office__ppt_profile" - danh sach chi doc ghi ten tool tran cua
// server built-in, tra theo ten day du thi khong bao gio khop (26/10/2026: ppt_profile luc lap ke hoach bi tinh la
// lenh ghi, tang revision).
func mcpToolChangesState(bound mcp.ServerTool) bool {
	if bound.Server != nil && bound.Server.Config.BuiltIn {
		return MCPChangesState(bound.Tool.Name)
	}
	return MCPChangesState(bound.Name())
}

// mcpReadOnly: cac tool MCP chi doc trang thai, khong ghi file nao. Danh sach nay gom dung cac tool
// doc cua server office built-in (xem mcpserver/filetools_*.go); moi tool khac - ke ca tool cua server
// nguoi dung tu cau hinh, va cac tool lenh live chay duoc ca doc lan ghi - deu tinh la CO ghi.
//
// Cung nguyen tac voi noDocumentChange: doan sai theo huong "co ghi" chi ton mot vong doc lai, doan
// sai theo huong "chi doc" thi bo qua buoc kiem chung cua mot lan ghi that.
var mcpReadOnly = map[string]bool{
	"echo": true, "office_sessions": true,
	"doc_get_text": true, "doc_find_text": true, "doc_extract_table": true, "doc_profile": true,
	"excel_read": true, "excel_profile": true,
	"ppt_get_text": true, "ppt_list_slides": true, "ppt_profile": true, "ppt_health": true,
	"word_read_text": true, "word_health": true,
	"wps_health": true, "wps_live_read_range": true,
}

// MCPChangesState: tool MCP nay co ghi gi khong (theo ten tool).
func MCPChangesState(name string) bool { return !mcpReadOnly[name] }

func truncateJSON(arguments map[string]any) string {
	if len(arguments) == 0 {
		return "{}"
	}
	encoded := jsonEncode(arguments)
	if len([]rune(encoded)) > 200 {
		return string([]rune(encoded)[:200])
	}
	return encoded
}

func jsonEncode(value any) string {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(value) != nil {
		return "{}"
	}
	return strings.TrimRight(builder.String(), "\n")
}
