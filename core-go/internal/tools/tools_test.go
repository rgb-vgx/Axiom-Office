package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/skills"
)

func catalog() *office.CommandCatalog {
	return &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "et.writeRange", Kind: "et", Agent: true, Summary: "ghi vung", Params: []office.CommandParam{
			{Name: "range", Required: true, Hint: "top-left cell e.g. 'A1'"}, {Name: "values", Required: true, Hint: "2D array"},
		}},
		{Name: "et.save", Kind: "et", Agent: false},
		{Name: "et.saveAs", Kind: "et", Agent: true, Params: []office.CommandParam{{Name: "path", Required: true}}},
		{Name: "writer.closeAll", Kind: "wps", Agent: false},
		{Name: "wpp.deleteSlide", Kind: "wpp", Agent: true},
	}}
}

func TestOfficeActionAllowlistAndDescription(t *testing.T) {
	tool := NewOfficeActionTool(catalog(), "et")
	if strings.Join(tool.AllowedActions(), ",") != "et.saveAs,et.writeRange" {
		t.Fatalf("allowlist = %v", tool.AllowedActions())
	}
	// Mo ta tool liet ke chu ky lenh kem goi y tham so, va co quy tac khong undo.
	for _, want := range []string{"et.writeRange {range (top-left cell e.g. 'A1'),values (2D array)}", UndoRule} {
		if !strings.Contains(tool.Description(), want) {
			t.Errorf("mo ta thieu %q", want)
		}
	}

	run := &RunContext{Office: &office.Session{Port: 1}, Bridge: office.NewBridgeClient(nil, "")}
	if result := tool.Invoke(context.Background(), map[string]any{"action": "writer.closeAll"}, run); result.OK ||
		!strings.Contains(result.JSON, "not an available action") {
		t.Fatalf("lenh ngoai allowlist phai bi tu choi: %+v", result)
	}
	if result := tool.Invoke(context.Background(), map[string]any{}, run); !strings.Contains(result.JSON, "missing 'action'") {
		t.Fatalf("thieu action: %+v", result)
	}
	// Lenh cua app khac khong duoc co trong allowlist cua app nay.
	if strings.Contains(strings.Join(tool.AllowedActions(), ","), "wpp.") {
		t.Fatal("lenh cua app khac khong duoc lot vao allowlist")
	}
}

func TestCanonicalActionFixesTypos(t *testing.T) {
	allowed := []string{"et.writeRange", "writer.getText"}
	cases := map[string]string{
		"et_writeRange":    "et.writeRange",    // gach duoi thay vi dau cham
		"ET.WRITERANGE":    "et.writeRange",    // khac hoa/thuong
		"writer.getText":   "writer.getText",   // dung san
		" writer.getText ": "writer.getText",   // thua khoang trang
		"khong.co.ten.nay": "khong.co.ten.nay", // khong khop -> giu nguyen de bao loi
	}
	for input, want := range cases {
		if got := CanonicalAction(input, allowed); got != want {
			t.Errorf("CanonicalAction(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeParams(t *testing.T) {
	if params := NormalizeParams(`{"range":"A1"}`); params["range"] != "A1" {
		t.Fatalf("chuoi JSON phai duoc parse: %v", params)
	}
	if params := NormalizeParams(map[string]any{"range": "A1"}); params["range"] != "A1" {
		t.Fatalf("object phai giu nguyen: %v", params)
	}
	if params := NormalizeParams("  "); params != nil {
		t.Fatalf("chuoi rong -> nil, nhan duoc %v", params)
	}
	if params := NormalizeParams("{khong phai json}"); params != nil {
		t.Fatalf("JSON hong -> nil, nhan duoc %v", params)
	}
}

func TestSkillToolsWithEmptyIndex(t *testing.T) {
	// Chi muc rong: load_skill phai bao khong co skill nao (khong lam hong luot chay).
	index := skills.New(nil)
	load := NewLoadSkillTool(index, "et")
	result := load.Invoke(context.Background(), map[string]any{"name": "khong-co"}, &RunContext{})
	if result.OK || !strings.Contains(result.JSON, "unknown skill 'khong-co'") || !strings.Contains(result.JSON, "(none)") {
		t.Fatalf("load_skill = %+v", result)
	}

	read := NewReadSkillFileTool(index, "et")
	if result := read.Invoke(context.Background(), map[string]any{"name": "_design", "path": "tokens.json"}, &RunContext{}); result.OK {
		t.Fatalf("_design khong ton tai phai loi: %+v", result)
	}
}

func TestModelToolsShape(t *testing.T) {
	tool := NewOfficeActionTool(catalog(), "et")
	list := NewRegistry([]Tool{tool}).ModelTools()
	if len(list) != 1 || list[0].Name != "office_action" {
		t.Fatalf("ModelTools = %+v", list)
	}
	var schema map[string]any
	if err := json.Unmarshal(list[0].Parameters, &schema); err != nil || schema["type"] != "object" {
		t.Fatalf("schema = %s", list[0].Parameters)
	}
}

// Cung ky vong voi PolicyTests.cs cua ban .NET: nguoi dung tu choi thi tool tra "user declined"
// va KHONG gui lenh nao xuong bridge.
func TestOfficeActionDeclinedDoesNotCallBridge(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	port, err := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}

	catalog := &office.CommandCatalog{Version: "1.0.0", Commands: []office.Command{
		{Name: "wpp.deleteSlide", Kind: "wpp", Agent: true, Summary: "xoa slide"},
	}}
	tool := NewOfficeActionTool(catalog, "wpp")
	run := &RunContext{
		Office: &office.Session{App: "wpp", Port: port},
		Bridge: office.NewBridgeClient(server.Client(), "t"),
		Prompt: "xoa slide cuoi",
		Confirm: func(action, reason, preview string) (bool, error) {
			if action != "wpp.deleteSlide" || !strings.Contains(reason, "Xoá slide") {
				t.Errorf("ly do hoi sai: action=%q reason=%q", action, reason)
			}
			return false, nil
		},
	}

	result := tool.Invoke(context.Background(), map[string]any{"action": "wpp.deleteSlide"}, run)

	if result.OK || !strings.Contains(result.JSON, "user declined") {
		t.Fatalf("phai bi tu choi: %+v", result)
	}
	if calls != 0 {
		t.Fatalf("da tu choi thi khong duoc goi bridge (goi %d lan)", calls)
	}
}

// Lenh khong co trong allowlist thi loi phai noi NGAY con nhung lenh nao cung nhom. Log 02/10/2026: mot
// luot chay doan ~30 ten lenh tu nghi ra (et.createSheet, et.insertSheet, et.addSheet...) chi vi khong
// tin rang lenh no can khong ton tai - moi vong doan la mot vong goi model bi dot.
func TestOfficeActionUnknownActionListsAvailableActions(t *testing.T) {
	tool := NewOfficeActionTool(catalog(), "et")
	result := tool.Invoke(context.Background(), map[string]any{"action": "et.createSheet"}, &RunContext{})

	if result.OK {
		t.Fatalf("lenh la phai bi tu choi: %+v", result)
	}
	for _, wanted := range []string{"not an available action", "et.writeRange", "do not invent action names"} {
		if !strings.Contains(result.JSON, wanted) {
			t.Errorf("thieu %q trong loi: %s", wanted, result.JSON)
		}
	}
	// Chi liet ke nhom cung tien to, khong do ca danh sach cua moi app.
	if strings.Contains(result.JSON, "wpp.deleteSlide") {
		t.Errorf("go sai trong nhom et thi chi can liet ke nhom et: %s", result.JSON)
	}

	// Go sai tien to thi phai thay HET lenh cua app nay (tool "et" chi co lenh et + lenh chung).
	other := tool.Invoke(context.Background(), map[string]any{"action": "vu.vut"}, &RunContext{})
	if !strings.Contains(other.JSON, "Available actions are: ") ||
		!strings.Contains(other.JSON, "et.writeRange") || !strings.Contains(other.JSON, "et.saveAs") {
		t.Errorf("khong ro nhom thi phai liet ke het: %s", other.JSON)
	}
}

// Vai lenh phai duoc coi la CO sua tai lieu: day la dau vao quyet dinh luot chay co phai tu kiem
// chung khong. Doan sai theo huong "co sua" chi ton mot vong doc lai; doan sai theo huong "chi doc"
// thi mot lan ghi that bi bo qua buoc kiem chung.
func TestChangesDocumentClassification(t *testing.T) {
	readOnly := []string{"et.readRange", "et.checkRange", "et.listSheets", "writer.getText",
		"writer.selection", "writer.checkTables", "wpp.listSlides", "wpp.checkLayout", "app.info"}
	for _, action := range readOnly {
		if ChangesDocument(action) {
			t.Errorf("%s chi doc, khong duoc tinh la sua tai lieu", action)
		}
	}
	// save/saveAs/exportPdf chep tai lieu ra file, khong doi tai lieu dang mo.
	for _, action := range []string{"et.save", "et.saveAs", "et.exportPdf", "writer.saveAs", "wpp.exportPdf"} {
		if ChangesDocument(action) {
			t.Errorf("%s khong doi tai lieu dang mo", action)
		}
	}
	// Lenh sua, VA ca lenh la chua tung thay bao gio - mac dinh phai la "co sua".
	for _, action := range []string{"et.writeRange", "et.formatRange", "writer.typeText", "writer.undo",
		"wpp.addSlide", "wpp.deleteSlide", "lenh.moi.them.sau.nay"} {
		if !ChangesDocument(action) {
			t.Errorf("%s phai tinh la co sua tai lieu", action)
		}
	}

	// Tool MCP: tool doc cua server office built-in biet truoc; tool nguoi dung tu cau hinh thi khong.
	for _, name := range []string{"office_sessions", "excel_read", "word_read_text", "ppt_list_slides", "echo"} {
		if MCPChangesState(name) {
			t.Errorf("%s chi doc", name)
		}
	}
	for _, name := range []string{"excel_write", "word_insert_table", "ppt_add_slide", "may-chu-nguoi-dung.dat.lich"} {
		if !MCPChangesState(name) {
			t.Errorf("%s phai tinh la co ghi", name)
		}
	}
}
