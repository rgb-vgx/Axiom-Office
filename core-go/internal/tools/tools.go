// Package tools: cac tool ma agent goi duoc trong mot luot chay - ban Go cua Tools/*.cs.
package tools

import (
	"context"
	"encoding/json"
	"strings"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
)

// Result: ket qua mot lan chay tool. JSON = nguyen van de gui cho model; Action = ten lenh office (de ghi audit).
// ImageDataURL: anh tool tra kem (QA thi giac) - codec gui cho model duoi dang anh that.
type Result struct {
	JSON         string
	OK           bool
	Action       string
	ImageDataURL string
	// Mutating: lenh nay DA lam thay doi noi dung tai lieu. Chi luot co thay doi that moi can tu
	// kiem chung truoc khi tra loi (model.AgentOptions.Verify) - luot chi doc ma bat doc lai thi chi
	// ton them mot vong vo ich.
	Mutating bool
	// Verifies: ket qua la bang chung kiem tra tai lieu (lenh doc/kiem, hoac tool ghi tu kiem sau khi ghi) -
	// model.ToolResult.Verifies.
	Verifies bool
	// Final / FinalText: tool da xong CA viec nguoi dung yeu cau (va tu kiem chung) - luot ket thuc voi FinalText,
	// khong goi model them (model.ToolResult.Final).
	Final     bool
	FinalText string
	// Abort: khong lam tiep duoc (ung dung Office da dong / sap) - luot dung voi thong bao nay.
	Abort string
}

// noDocumentChange: cac lenh KHONG doi noi dung tai lieu dang mo - doc de tra loi, chay chan doan,
// hoac ghi tai lieu ra file (save/saveAs/exportPdf khong sua tai lieu, chi chep no ra cho khac).
//
// Moi lenh khac - ke ca lenh them sau nay chua co trong bang nay - duoc coi la CO sua tai lieu. Do la
// huong sai an toan: kiem chung thua chi ton mot vong doc lai, con bo sot thi cau tra loi sai duoc
// gui cho nguoi dung nhu dung roi.
var noDocumentChange = map[string]bool{
	"app.info": true, "app.screenshot": true,
	"et.readRange": true, "et.checkRange": true, "et.listSheets": true,
	// et.whatIf ghi tam mot o roi tu tra lai (ket qua = bang chung kiem tra, tai lieu khong doi).
	"et.readRanges": true, "et.whatIf": true,
	"et.save": true, "et.saveAs": true, "et.exportPdf": true,
	"writer.getText": true, "writer.selection": true, "writer.checkTables": true, "writer.outline": true,
	"writer.checkLayout": true,
	"writer.save":        true, "writer.saveAs": true, "writer.exportPdf": true,
	"wpp.listSlides": true, "wpp.checkLayout": true,
	"wpp.save": true, "wpp.saveAs": true, "wpp.exportPdf": true,
}

// ChangesDocument: lenh nay co lam doi noi dung tai lieu dang mo khong.
func ChangesDocument(action string) bool { return !noDocumentChange[action] }

// readOnly: lenh CHI DOC - tap con cua noDocumentChange ma che do lap ke hoach (plan mode) cho goi.
// save/saveAs/exportPdf khong sua tai lieu nhung GHI FILE ra dia nen khong nam day. Lenh moi chua co
// trong bang nay bi chan trong plan mode - cung huong sai an toan nhu noDocumentChange.
var readOnly = map[string]bool{
	"app.info": true, "app.screenshot": true,
	"et.readRange": true, "et.checkRange": true, "et.listSheets": true,
	// et.whatIf ghi tam mot o roi tu tra lai (ket qua = bang chung kiem tra, tai lieu khong doi).
	"et.readRanges": true, "et.whatIf": true,
	"writer.getText": true, "writer.selection": true, "writer.checkTables": true, "writer.outline": true,
	"writer.checkLayout": true,
	"wpp.listSlides":     true, "wpp.checkLayout": true,
}

// ReadOnly: lenh nay chi doc tai lieu, khong sua va khong ghi file.
func ReadOnly(action string) bool { return readOnly[action] }

// Verifies: lenh doc noi dung / kiem tai lieu - ket qua thanh cong la bang chung kiem tra o phien ban hien tai
// (app.info chi la thong tin ung dung, khong tinh).
func Verifies(action string) bool { return readOnly[action] && action != "app.info" }

// RunContext: trang thai cua mot luot chay ma tool can (muc 8.3).
type RunContext struct {
	RunID          string
	ConversationID string
	Office         *office.Session
	Bridge         *office.BridgeClient
	Prompt         string
	// Event phat su kien ra SSE cua luot chay (vd skill.loaded).
	Event func(eventType string, data map[string]any)
	// Confirm hoi nguoi dung truoc lenh rui ro (confirm.required -> POST /v1/runs/{id}/confirm).
	// nil = khong co ai de hoi (ai.ask cua agent ben ngoai) -> coi nhu tu choi.
	Confirm func(action, reason, paramsPreview string) (bool, error)
	// Choose hoi nguoi dung chon mot trong vai lua chon (the nhieu lua chon trong khung chat); tra id lua chon va chu
	// nguoi dung go neu lua chon do la o nhap; "" = khong tra loi (het gio, huy). nil = khong co ai de hoi.
	Choose func(action, question string, choices []Choice) (choice, text string, err error)
	// CreatedSheets: ten sheet da tao thanh cong trong luot nay (et.addSheet) - ghi de len
	// sheet do luot tao thi khong hoi. Log 11/10/2026: agent ghi de 25 o du lieu goc ma khong hoi.
	CreatedSheets map[string]bool
	// OverwriteApproved: sheet da duoc nguoi dung dong y ghi de trong luot nay - da dong y mot
	// lan thi cac lenh sau tren cung sheet trong cung luot khong hoi lai.
	OverwriteApproved map[string]bool
	// WrittenCells: o do chinh luot nay vua ghi (sheet -> dia chi): ghi de len chung khong hoi - bo do Excel 26/10/2026,
	// model ghi so tam roi thay bang cong thuc bi hoi tung lan (khung chat) hoac bi tu choi het (ai.ask, luot hong).
	WrittenCells map[string]map[string]bool
	// undoable: so buoc hoan tac do CHINH luot nay tao, theo app ("writer", "et", "wpp") - undo.go.
	undoable map[string]int
	// TargetDocument: tai lieu cua luot - Core tu gan vao lenh writer.*/et.*/wpp.* model khong chon tai lieu (target.go).
	// Doi theo lenh tao/mo tai lieu thanh cong trong luot.
	TargetDocument string
	// owned: slide/shape do chinh luot nay tao - xoa chung khong can hoi (owned.go).
	owned *ownedObjects
	// briefSpecs: mo ta slide (JSON) da dung qua slides_from_brief trong luot -> so loi bo cuc cua lan dung do.
	briefSpecs map[string]int
	// slideIssues: slide do slides_from_brief dung trong luot -> so loi bo cuc lan dung gan nhat; cleanRebuilds: so lan
	// goi da dung lai slide sach (checkCleanRebuilds).
	slideIssues   map[int]int
	cleanRebuilds int
	// FocusSlide: slide nguoi dung dang mo (pane gui document.view) - ket qua doc ca bai giu nguyen slide nay.
	FocusSlide int
	// Trace: su kien DebugTrace tu ben trong tool (vd slides_from_brief di lai day) - nil khi tat; goi qua run.trace.
	Trace func(event string, fields map[string]any)
	// Todos: danh sach viec model ghi gan nhat (write_todos).
	Todos []Todo
	// lastTemplate, lastSource, lastTarget: tham so slides_from_template lan truoc trong luot (model hay bo o lan sau).
	lastTemplate, lastSource, lastTarget string
	// droppedPictures: anh bo co chu dich trong che do bien soan (bao trong cau tra loi cuoi).
	droppedPictures []string
	// webSources: link tavily_search da tra trong luot (url -> tieu de) - slide "Cap nhat" chi duoc trich nhung link nay.
	webSources map[string]string
	// WebSources: noi luu nguon web tai ve trong luot (Core ghi core.db); nil = khong luu (chi song trong luot).
	WebSources WebSourceSaver
	// calls: so lan goi tung tool/lenh trong luot (CallLimit).
	calls map[string]int
	// singleCalls: so lan lenh mot vung (et.formatRange, et.readRange) thanh cong trong luot (batchHint).
	singleCalls map[string]int
	// templateBuilt: so slide slides_from_template da dung trong CA luot (cac lo truoc + lo nay) - cau tra loi cuoi
	// tach "dung trong lan nay" voi "co tu luot truoc".
	templateBuilt int
	// docRestyled: doc_design da trinh bay lai nen chung trong luot (so doan cua dan bai da doi theo).
	docRestyled bool
	// docStyle: kieu trinh bay nguoi dung chon cho doc_design trong luot (full | basic | keep | custom; "" = chua hoi).
	docStyle string
	// docWish: kieu nguoi dung tu go (docStyle custom); docOptions: accent/font/size cho writer.restyle va insertBlocks.
	docWish    string
	docOptions map[string]any
}

// Choice: mot lua chon cua the hoi nguoi dung (RunContext.Choose).
type Choice struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
	// Input: lua chon nay la o nguoi dung tu go (vd "Kieu khac") - cau tra loi kem chu.
	Input bool `json:"input,omitempty"`
}

func (c *RunContext) ConfirmOrDecline(action, reason, paramsPreview string) (bool, error) {
	if c.Confirm == nil {
		return false, nil
	}
	return c.Confirm(action, reason, paramsPreview)
}

// Tool: mot tool trong luot chay.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result
}

// Registry: tap tool cua mot luot chay.
type Registry struct{ tools []Tool }

func NewRegistry(list []Tool) *Registry { return &Registry{tools: list} }

func (r *Registry) All() []Tool { return r.tools }

func (r *Registry) Find(name string) Tool {
	for _, tool := range r.tools {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
}

// ModelTools chuyen sang dinh dang ma model client can.
func (r *Registry) ModelTools() []model.Tool {
	list := make([]model.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		list = append(list, model.Tool{Name: tool.Name(), Description: tool.Description(), Parameters: tool.Parameters()})
	}
	return list
}

// ErrorJSON: {"ok":false,"error":"..."} khong escape ky tu (thong bao doc duoc nguyen van).
func ErrorJSON(message string) string { return model.ErrorJSON(message) }

// FailJSON: {"ok":false,"error":"...", ...chi tiet} - tool that bai nhung co them du lieu de model sua (vd danh sach loi
// tung truong). Truoc 26/10/2026 slides_from_brief tu choi bang {"ok":true,"result":{"built":0,"errors":...}}: model
// va khung chat thay "ok":true trong mot lan goi hong.
func FailJSON(message string, details map[string]any) string {
	body := map[string]any{"ok": false, "error": message}
	for key, value := range details {
		if key != "ok" && key != "error" {
			body[key] = value
		}
	}
	return model.MarshalRelaxed(body)
}

// OKJSON: {"ok":true,"result":...}; tra nguyen van neu phan result khong marshal duoc.
func OKJSON(result any) string {
	return model.MarshalRelaxed(map[string]any{"ok": true, "result": result})
}

// utf8BOM: ky tu BOM dau file (dinh nghia bang byte de nguon khong lan BOM that).
var utf8BOM = string([]byte{0xEF, 0xBB, 0xBF})

// stringArg doc mot truong chuoi trong tham so model gui.
func stringArg(arguments map[string]any, name string) string {
	value, _ := arguments[name].(string)
	return strings.TrimSpace(value)
}

// WebSourceSaver: cau noi sang kho nguon web (agent adapter -> store.WebSources).
type WebSourceSaver interface {
	SaveWebSources(runID, conversationID, kind, query string, docs []WebSourceDoc) int
}

// SaveWebSources: goi khi tool web tra ve nguon (nil-safe; chua bat luu = bo qua im lang).
func (c *RunContext) SaveWebSources(kind, query string, docs []WebSourceDoc) {
	if c == nil || c.WebSources == nil || len(docs) == 0 {
		return
	}
	c.WebSources.SaveWebSources(c.RunID, c.ConversationID, kind, query, docs)
}

// noteWebSource: ghi lai mot link web vua tra trong luot (run nil: tool goi ngoai luot chay, vd test).
func (c *RunContext) noteWebSource(url, title string) {
	if c == nil || strings.TrimSpace(url) == "" {
		return
	}
	if c.webSources == nil {
		c.webSources = map[string]string{}
	}
	c.webSources[normalizeURL(url)] = strings.TrimSpace(title)
}

// WebSource: link nay co phai do tavily_search tra trong luot khong.
func (c *RunContext) WebSource(url string) bool {
	if c == nil || c.webSources == nil {
		return false
	}
	_, ok := c.webSources[normalizeURL(url)]
	return ok
}

func normalizeURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	url = strings.TrimPrefix(url, "www.")
	return strings.TrimRight(url, "/")
}

// trace: su kien DebugTrace cua tool (khong lam gi khi tat).
func (r *RunContext) trace(event string, fields map[string]any) {
	if r != nil && r.Trace != nil {
		r.Trace(event, fields)
	}
}
