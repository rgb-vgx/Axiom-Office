package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"axiomoffice/core/internal/model"
	"axiomoffice/core/internal/office"
)

// Lam viec xuyen ung dung (docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md C1): luot Word doc so lieu tu bang
// Excel dang mo, luot PowerPoint lay noi dung tu bao cao Word... Moi tai lieu khac la mot phien Office (cong bridge) rieng.
//
//   - Tiet lo dan (disclosure.go): tool chi mang DANH SACH tai lieu khac; lenh cua app do chi tra ve khi model goi
//     action "commands" - khong nhan doi mo ta office_action cua ba app vao moi vong.
//   - Moi tai lieu khac dung mot office_action rieng (catalog cua app do) va RunContext rieng: guard ghi de, sort guard,
//     policy, uncertain... deu chay nhu luot dang lam tren chinh tai lieu do; so buoc hoan tac khong lan vao tai lieu
//     chinh (nut "Hoan tac luot nay" chi lui tai lieu cua pane).
//   - Sua tai lieu khac: hoi nguoi dung MOT lan moi tai lieu moi luot; tai lieu dang co luot khac chay thi khong sua
//     (hai luot cung ghi mot tai lieu). Lap ke hoach / lenh nhanh chi doc: chi co lenh doc.

const OtherDocumentToolName = "other_document"

// OtherDocumentTool: doc/sua tai lieu Office khac dang mo trong luot.
type OtherDocumentTool struct {
	documents []office.Session
	readOnly  bool
	// busy: cong do dang co luot khac chay (Manager.BusyPort) - nil = khong kiem.
	busy func(port int) bool

	mu       sync.Mutex
	contexts map[int]*RunContext
	tools    map[int]*OfficeActionTool
	approved map[int]bool
}

func NewOtherDocumentTool(documents []office.Session, readOnly bool, busy func(port int) bool) *OtherDocumentTool {
	return &OtherDocumentTool{
		documents: documents, readOnly: readOnly, busy: busy,
		contexts: map[int]*RunContext{}, tools: map[int]*OfficeActionTool{}, approved: map[int]bool{},
	}
}

func (t *OtherDocumentTool) Name() string { return OtherDocumentToolName }

func (t *OtherDocumentTool) Description() string {
	lines := []string{}
	for _, document := range t.documents {
		lines = append(lines, DescribeOtherDocument(document))
	}
	text := "Work with ANOTHER open Office document (not the one this chat is attached to), e.g. read numbers from an " +
		"open workbook to write them into this document. Open documents: " + strings.Join(lines, "; ") + ". " +
		"First call {document, action: \"commands\"} to get that app's commands, then call {document, action, params} " +
		"exactly like office_action. Use office_action for the current document."
	if t.readOnly {
		text += " Only read commands are available in this mode."
	} else {
		text += " Changing another document asks the user first."
	}
	return text
}

func (t *OtherDocumentTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"document":{"type":"string","description":"document name or port from the list"},` +
		`"action":{"type":"string","description":"\"commands\" to list the commands, or a command name like et.readRanges"},` +
		`"params":{"type":"object","description":"the command's parameters"}},"required":["document","action"]}`)
}

// DescribeOtherDocument: "Bao cao.xlsx (Excel, port 50213)".
func DescribeOtherDocument(session office.Session) string {
	name := strings.TrimSpace(session.Document)
	if name == "" {
		name = "(untitled)"
	}
	return name + " (" + appLabel(session.App) + ", port " + strconv.Itoa(session.Port) + ")"
}

func appLabel(app string) string {
	switch app {
	case "wps":
		return "Word"
	case "et":
		return "Excel"
	case "wpp":
		return "PowerPoint"
	}
	return app
}

// find: tai lieu theo cong, ten day du, ten khong duoi, hoac duy nhat theo app ("Excel", "et").
func (t *OtherDocumentTool) find(key string) (*office.Session, string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, "missing 'document': " + t.choices()
	}
	if port, err := strconv.Atoi(key); err == nil {
		for i := range t.documents {
			if t.documents[i].Port == port {
				return &t.documents[i], ""
			}
		}
	}
	lower := strings.ToLower(key)
	matches := []int{}
	for i, document := range t.documents {
		name := strings.ToLower(strings.TrimSpace(document.Document))
		path := strings.ToLower(strings.TrimSpace(document.DocumentPath))
		stem := name
		if dot := strings.LastIndex(stem, "."); dot > 0 {
			stem = stem[:dot]
		}
		if name == lower || path == lower || stem == lower || strings.ToLower(DescribeOtherDocument(document)) == lower {
			return &t.documents[i], ""
		}
		if strings.EqualFold(document.App, key) || strings.EqualFold(appLabel(document.App), key) ||
			(name != "" && strings.Contains(name, lower)) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 1 {
		return &t.documents[matches[0]], ""
	}
	if len(matches) > 1 {
		return nil, "'" + key + "' matches several documents - give the port: " + t.choices()
	}
	return nil, "no open document '" + key + "': " + t.choices()
}

func (t *OtherDocumentTool) choices() string {
	lines := []string{}
	for _, document := range t.documents {
		lines = append(lines, DescribeOtherDocument(document))
	}
	return strings.Join(lines, "; ")
}

// childFor: office_action + RunContext rieng cho tai lieu khac (giu qua cac lan goi trong luot). Catalog lenh cua app do
// chi doc khi model dung toi tai lieu nay lan dau.
func (t *OtherDocumentTool) childFor(ctx context.Context, session *office.Session, parent *RunContext) (*OfficeActionTool,
	*RunContext, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	port := session.Port
	if tool, ok := t.tools[port]; ok {
		return tool, t.contexts[port], ""
	}
	catalog := parent.Bridge.GetCommandsFor(ctx, port, session.App, false)
	if catalog == nil {
		return nil, nil, "cannot read the commands of " + DescribeOtherDocument(*session) + " (is it still open?)"
	}
	tool := NewOfficeActionTool(catalog, session.App)
	if t.readOnly {
		tool = NewReadOnlyOfficeActionTool(catalog, session.App)
	}
	copied := *session
	target := strings.TrimSpace(copied.DocumentPath)
	if target == "" {
		target = strings.TrimSpace(copied.Document)
	}
	child := &RunContext{
		RunID: parent.RunID, ConversationID: parent.ConversationID, Office: &copied, Bridge: parent.Bridge,
		Prompt: parent.Prompt, Event: parent.Event, Confirm: parent.Confirm, Choose: parent.Choose, Trace: parent.Trace,
		TargetDocument: target,
	}
	t.tools[port], t.contexts[port] = tool, child
	return tool, child, ""
}

func (t *OtherDocumentTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	document, problem := t.find(stringArg(arguments, "document"))
	if problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	action := strings.TrimSpace(stringArg(arguments, "action"))
	tool, child, problem := t.childFor(ctx, document, run)
	if problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	label := DescribeOtherDocument(*document)
	if action == "" || strings.EqualFold(action, "commands") {
		data, _ := json.Marshal(map[string]any{
			"document": label, "app": document.App, "commands": tool.signatures,
			"note": "call other_document {document, action, params} with one of these; the document parameter is filled in for you",
		})
		return Result{JSON: string(data), OK: true, Action: OtherDocumentToolName}
	}
	action = CanonicalAction(action, tool.names)
	if _, ok := tool.allowed[action]; ok && !ReadOnly(action) {
		if t.busy != nil && t.busy(document.Port) {
			return Result{JSON: ErrorJSON(label + " has another Axiom run in progress: wait for it to finish before changing it"),
				Action: action}
		}
		t.mu.Lock()
		approved := t.approved[document.Port]
		t.mu.Unlock()
		if !approved {
			reason := "Sửa tài liệu khác đang mở: " + label
			preview := model.Truncate(action+" "+model.MarshalRelaxed(arguments["params"]), 200)
			ok, err := run.ConfirmOrDecline(action, reason, preview)
			if err != nil || !ok {
				return Result{JSON: ErrorJSON("user declined: " + reason), Action: action}
			}
			t.mu.Lock()
			t.approved[document.Port] = true
			t.mu.Unlock()
		}
	}
	output := tool.Invoke(ctx, map[string]any{"action": action, "params": arguments["params"]}, child)
	// Sua tai lieu khac khong lam doi tai lieu cua luot: khong tang revision / khong buoc kiem chung tai lieu chinh.
	output.Mutating, output.Verifies = false, false
	return output
}
