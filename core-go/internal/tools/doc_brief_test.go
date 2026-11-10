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
)

type docCall struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params"`
}

// docBriefRun: bridge gia ghi lai lenh; tai lieu co `chars` ky tu; lenh `failOn` tra loi.
func docBriefRun(t *testing.T, chars int, failOn string) (*RunContext, *[]docCall, func()) {
	t.Helper()
	calls := []docCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call docCall
		_ = json.NewDecoder(r.Body).Decode(&call)
		calls = append(calls, call)
		switch {
		case call.Action == failOn:
			_, _ = w.Write([]byte(`{"ok":false,"error":"boom"}`))
		case call.Action == "writer.getText":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"totalChars":` + strconv.Itoa(chars) + `,"text":""}}`))
		case call.Action == "writer.checkLayout":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"pages":1,"issueCount":0,"issues":[]}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		}
	}))
	port, _ := strconv.Atoi(strings.TrimPrefix(server.URL, "http://127.0.0.1:"))
	run := &RunContext{Office: &office.Session{App: "wps", Port: port}, Bridge: office.NewBridgeClient(server.Client(), "t")}
	return run, &calls, server.Close
}

func briefText(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func TestDocFromBriefToTrinh(t *testing.T) {
	run, calls, done := docBriefRun(t, 1, "")
	defer done()
	result := NewDocFromBriefTool().Invoke(context.Background(), map[string]any{
		"kind":   "to-trinh",
		"agency": map[string]any{"parent": "Sở GD&ĐT Hà Nội", "name": "Trường THPT Nguyễn Du"},
		"place":  "Hà Nội", "date": "5/2/2026", "subject": "Về việc mua máy tính", "to": []any{"Giám đốc"},
		"body":   []any{map[string]any{"text": "Đề nghị mua 5 máy tính."}},
		"signer": map[string]any{"position": "Hiệu trưởng"},
	}, run)
	if !result.OK || !result.Mutating || !result.Verifies {
		t.Fatalf("phai dung xong va tu kiem: %+v", result)
	}
	actions := []string{}
	for _, call := range *calls {
		actions = append(actions, call.Action)
	}
	want := "writer.getText writer.insertLayoutTable writer.writeContent writer.writeContent writer.insertLayoutTable writer.checkLayout"
	if strings.Join(actions, " ") != want {
		t.Fatalf("thu tu lenh:\n got %v\nwant %s", actions, want)
	}
	header := briefText((*calls)[1].Params["rows"])
	for _, part := range []string{"SỞ GD\\u0026ĐT HÀ NỘI", "TRƯỜNG THPT NGUYỄN DU", "Số: …", "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM",
		"Hà Nội, ngày 05 tháng 02 năm 2026"} {
		if !strings.Contains(header, part) {
			t.Fatalf("phan dau thieu %q: %s", part, header)
		}
	}
	title := briefText((*calls)[2].Params["blocks"])
	if !strings.Contains(title, `"TỜ TRÌNH"`) || !strings.Contains(title, "Về việc mua máy tính") || !strings.Contains(title, "Kính gửi: Giám đốc.") {
		t.Fatalf("ten loai / trich yeu / kinh gui: %s", title)
	}
	sign := briefText((*calls)[4].Params["rows"])
	if !strings.Contains(sign, "Nơi nhận:") || !strings.Contains(sign, "- Lưu: VT.") || !strings.Contains(sign, "HIỆU TRƯỞNG") {
		t.Fatalf("phan ky: %s", sign)
	}
	var body struct {
		Result map[string]any `json:"result"`
	}
	_ = json.Unmarshal([]byte(result.JSON), &body)
	placeholders := briefText(body.Result["placeholders"])
	if !strings.Contains(placeholders, "number") || !strings.Contains(placeholders, "signer.name") {
		t.Fatalf("thieu so va ten nguoi ky phai bao placeholders: %s", result.JSON)
	}
	if run.undoable["writer"] != 4 {
		t.Fatalf("4 lenh sua = 4 buoc hoan tac cua luot, co %d", run.undoable["writer"])
	}
}

func TestDocFromBriefBienBanAndCongVan(t *testing.T) {
	run, calls, done := docBriefRun(t, 0, "")
	defer done()
	NewDocFromBriefTool().Invoke(context.Background(), map[string]any{
		"kind": "bien-ban", "subject": "Họp giao ban tuần 43", "date": "20/10/2026",
		"meeting": map[string]any{"chair": "ông Trần Văn Nam", "secretary": "bà Lê Thu Hà", "time": "8h00 ngày 20/10/2026"},
		"body":    []any{map[string]any{"heading": "1. Nội dung", "level": 1}},
	}, run)
	title := briefText((*calls)[2].Params["blocks"])
	if !strings.Contains(title, "BIÊN BẢN") || strings.Contains(title, "Về việc") || !strings.Contains(title, "Chủ trì: ông Trần Văn Nam") {
		t.Fatalf("bien ban: %s", title)
	}
	sign := briefText((*calls)[4].Params["rows"])
	if !strings.Contains(sign, "THƯ KÝ") || !strings.Contains(sign, "CHỦ TRÌ") || strings.Contains(sign, "Nơi nhận") {
		t.Fatalf("bien ban ky thu ky | chu tri: %s", sign)
	}

	run, calls, done2 := docBriefRun(t, 0, "")
	defer done2()
	NewDocFromBriefTool().Invoke(context.Background(), map[string]any{
		"kind": "cong-van", "subject": "đề nghị cung cấp số liệu", "to": []any{"Sở Tài chính", "Kho bạc"},
		"body": []any{map[string]any{"text": "Nội dung."}},
	}, run)
	header := briefText((*calls)[1].Params["rows"])
	title = briefText((*calls)[2].Params["blocks"])
	if !strings.Contains(header, "V/v đề nghị cung cấp số liệu") || strings.Contains(title, "Về việc") ||
		!strings.Contains(title, `- Sở Tài chính;\n- Kho bạc.`) {
		t.Fatalf("cong van: trich yeu o cot trai, kinh gui nhieu noi:\n%s\n%s", header, title)
	}
}

func TestDocFromBriefLongKindAddsPageNumberAndToc(t *testing.T) {
	run, calls, done := docBriefRun(t, 0, "")
	defer done()
	NewDocFromBriefTool().Invoke(context.Background(), map[string]any{
		"kind": "bao-cao", "subject": "Tổng kết năm học", "toc": true, "maxPages": float64(4),
		"body": []any{map[string]any{"heading": "1. Kết quả", "level": 1}},
	}, run)
	actions := []string{}
	for _, call := range *calls {
		actions = append(actions, call.Action)
	}
	want := "writer.getText writer.insertLayoutTable writer.writeContent writer.insertToc writer.writeContent " +
		"writer.insertLayoutTable writer.setHeaderFooter writer.insertToc writer.checkLayout"
	if strings.Join(actions, " ") != want {
		t.Fatalf("bao cao:\n got %v\nwant %s", actions, want)
	}
	if asInt((*calls)[len(*calls)-1].Params["maxPages"]) != 4 {
		t.Fatalf("maxPages phai chuyen cho checkLayout: %v", (*calls)[len(*calls)-1].Params)
	}
}

func TestDocFromBriefRefusals(t *testing.T) {
	run, calls, done := docBriefRun(t, 500, "")
	defer done()
	result := NewDocFromBriefTool().Invoke(context.Background(), map[string]any{"kind": "to-trinh",
		"body": []any{map[string]any{"text": "x"}}}, run)
	if result.OK || result.Mutating || !strings.Contains(result.JSON, "NEW document") || len(*calls) != 1 {
		t.Fatalf("tai lieu co san phai bi tu choi, khong ghi gi: %+v %v", result, *calls)
	}
	result = NewDocFromBriefTool().Invoke(context.Background(), map[string]any{"kind": "don-xin",
		"body": []any{map[string]any{"table": []any{[]any{"=B2*2"}}}, map[string]any{"para": "x"}}}, run)
	if result.OK || len(*calls) != 1 {
		t.Fatalf("brief sai phai bi tu choi truoc khi goi bridge: %+v", result)
	}
	for _, part := range []string{"kind must be", "formulas", "body[1] needs"} {
		if !strings.Contains(result.JSON, part) {
			t.Fatalf("thieu loi %q: %s", part, result.JSON)
		}
	}
}

func TestDocFromBriefPartialFailureTellsUndo(t *testing.T) {
	run, _, done := docBriefRun(t, 0, "writer.insertLayoutTable")
	defer done()
	result := NewDocFromBriefTool().Invoke(context.Background(), map[string]any{"kind": "thong-bao",
		"body": []any{map[string]any{"text": "x"}}}, run)
	if result.OK || result.Mutating || !strings.Contains(result.JSON, "boom") {
		t.Fatalf("hong o lenh dau: chua ghi gi: %+v", result)
	}
	run2, _, done2 := docBriefRun(t, 0, "writer.insertToc")
	defer done2()
	result = NewDocFromBriefTool().Invoke(context.Background(), map[string]any{"kind": "bao-cao", "toc": true,
		"body": []any{map[string]any{"text": "x"}}}, run2)
	if result.OK || !result.Mutating || !strings.Contains(result.JSON, `writer.undo {\"count\": 2}`) || run2.undoable["writer"] != 2 {
		t.Fatalf("hong giua chung: bao lui 2 buoc: %+v %v", result, run2.undoable)
	}
}
