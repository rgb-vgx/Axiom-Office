package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func designOutline() []outlineItem {
	return []outlineItem{
		{N: 1, Kind: "p", Text: "GIÁO ÁN HÌNH HỌC 8"},
		{N: 2, Kind: "p", Text: "I. MỤC TIÊU"},
		{N: 3, Kind: "p", Text: "- Học sinh nắm được định lí Pitago trong 45 phút, làm được 3 bài tập."},
		{N: 4, Kind: "p", Text: "- Rèn kĩ năng vẽ hình và trình bày lời giải."},
		{N: 6, Kind: "table", Rows: 5, Cols: 3, Header: []string{"ND", "ĐL", "Hoạt động"}},
		{N: 30, Kind: "p", Text: "Lưu ý: không dùng máy tính."},
	}
}

func blocksFrom(t *testing.T, text string) []any {
	t.Helper()
	var list []any
	if err := json.Unmarshal([]byte(text), &list); err != nil {
		t.Fatal(err)
	}
	return list
}

func TestPlanBlocksBuildsNoteAndCards(t *testing.T) {
	list := blocksFrom(t, `[
		{"type":"note","replace":"3-4","tone":"goal","title":"Mục tiêu","lines":["- Nắm định lí **Pitago** (45 phút, 3 bài tập)","- Rèn kĩ năng vẽ hình, trình bày lời giải"]},
		{"type":"cards","before":30,"cards":[{"title":"Sai","lines":["Bỏ đơn vị"]},{"title":"Đúng","lines":["Ghi đơn vị"]}]}
	]`)
	out, problems := planBlocks(list, designOutline(), 40, "")
	if len(problems) > 0 {
		t.Fatalf("problems: %+v", problems)
	}
	if len(out) != 2 || out[0]["kind"] != "box" || out[0]["replaceFrom"] != 3 || out[0]["replaceTo"] != 4 {
		t.Fatalf("note block: %+v", out[0])
	}
	cells := out[0]["cells"].([]map[string]any)
	if cells[0]["tone"] != "blue" {
		t.Fatalf("goal tone should map to blue: %+v", cells[0])
	}
	lines := cells[0]["lines"].([]map[string]any)
	if lines[0]["k"] != "title" || lines[1]["k"] != "bullet" || strings.HasPrefix(lines[1]["t"].(string), "-") {
		t.Fatalf("lines: %+v", lines)
	}
	if out[1]["kind"] != "row" || out[1]["before"] != 30 || len(out[1]["cells"].([]map[string]any)) != 2 {
		t.Fatalf("cards block: %+v", out[1])
	}
}

func TestPlanBlocksRejectsLostNumbersTablesAndOverlaps(t *testing.T) {
	cases := map[string]string{
		`[{"type":"note","replace":"3","lines":["Nắm định lí Pitago, làm được 3 bài tập trên lớp cho tốt"]}]`: "missing: 45",
		`[{"type":"note","replace":"4-6","lines":["Rèn kĩ năng vẽ hình và trình bày lời giải"]}]`:             "table",
		`[{"type":"note","replace":"3-4","lines":["Học sinh nắm được định lí Pitago trong 45 phút, làm được 3 bài tập", "Rèn kĩ năng vẽ hình và trình bày lời giải"]},
		  {"type":"note","replace":"4","lines":["Rèn kĩ năng vẽ hình và trình bày lời giải."]}]`: "overlaps block 1",
		`[{"type":"note","replace":"3-4","lines":["45 3"]}]`:      "characters",
		`[{"type":"cards","before":2,"cards":[{"title":"Một"}]}]`: "2-4 cards",
		`[{"type":"chart","before":2}]`:                           "unknown block type",
		`[{"type":"note","replace":"39-41","lines":["x"]}]`:       "outside the document",
		`[{"type":"note","lines":["x"]}]`:                         "replace",
	}
	for input, want := range cases {
		_, problems := planBlocks(blocksFrom(t, input), designOutline(), 40, "")
		joined := ""
		for _, problem := range problems {
			joined += problem.Error + "; "
		}
		if !strings.Contains(joined, want) {
			t.Errorf("%s\nwant %q, got %q", input, want, joined)
		}
	}
}

func TestMissingNumbersIgnoresTrailingPunctuation(t *testing.T) {
	if missing := missingNumbers("Bước 1. Làm 2,5 phút; mục 3.2.", "Bước 1: 2,5 phút - mục 3.2"); len(missing) != 0 {
		t.Fatalf("missing: %v", missing)
	}
	if missing := missingNumbers("Thời gian 15 phút", "Thời gian ngắn"); len(missing) != 1 || missing[0] != "15" {
		t.Fatalf("missing: %v", missing)
	}
}

func TestOutlineViewShowsTablesOnOneLine(t *testing.T) {
	view := outlineView(designOutline())
	if !strings.Contains(view, "6| [table 5x3: ND | ĐL | Hoạt động]") || !strings.Contains(view, "2| I. MỤC TIÊU") {
		t.Fatalf("view:\n%s", view)
	}
}

// Luot muse-spark 15/10/2026: bi tu choi "replace" thi model chen "before" - khung tieu de va khoi "Ket thuc" lap lai
// nguyen doan ben duoi, khung tieu de con them "45 phut" giao an khong co.
func TestPlanBlocksRejectsRepeatsAndInventedNumbers(t *testing.T) {
	outline := []outlineItem{
		{N: 1, Kind: "p", Text: "Ngày soạn: 10/10/2026   Họ tên giáo viên: Trần Thị Dương"},
		{N: 2, Kind: "p", Text: "CHỦ ĐỀ 3: KĨ THUẬT ĐÁNH CẦU THẤP TAY (13 tiết)"},
		{N: 3, Kind: "p", Text: "BÀI 1: CÁCH CẦM VỢT, CẦU, TƯ THẾ CHUẨN BỊ VÀ KĨ THUẬT ĐÁNH CẦU THẤP THUẬN TAY (4 tiết)"},
		{N: 4, Kind: "p", Text: "Tiết 4: HOÀN THIỆN KĨ THUẬT ĐÁNH CẦU THẤP THUẬN TAY"},
		{N: 5, Kind: "p", Text: "I. MỤC TIÊU"},
	}
	doc := "Ngày soạn: 10/10/2026\rCHỦ ĐỀ 3 (13 tiết)\rBÀI 1 (4 tiết)\rTiết 4\rHOẠT ĐỘNG MỞ ĐẦU (8-10’)"
	repeat := `[{"type":"title","before":2,"title":"CHỦ ĐỀ 3: KĨ THUẬT ĐÁNH CẦU THẤP TAY (13 tiết)","lines":["Bài 1: Cách cầm vợt, cầu, tư thế chuẩn bị và kĩ thuật đánh cầu thấp thuận tay (4 tiết)","Tiết 4: Hoàn thiện kĩ thuật đánh cầu thấp thuận tay"]}]`
	_, problems := planBlocks(blocksFrom(t, repeat), outline, 5, doc)
	if len(problems) != 1 || !strings.Contains(problems[0].Error, `"replace": "2-4"`) {
		t.Fatalf("khung lap lai doan 2-4 phai bi tu choi, goi y replace: %+v", problems)
	}
	invented := `[{"type":"title","replace":"2-4","title":"CHỦ ĐỀ 3: KĨ THUẬT ĐÁNH CẦU THẤP TAY (13 tiết)","lines":["Bài 1: Cách cầm vợt, cầu, tư thế chuẩn bị và kĩ thuật đánh cầu thấp thuận tay (4 tiết)","Tiết 4: Hoàn thiện kĩ thuật đánh cầu thấp thuận tay - 45 phút"]}]`
	_, problems = planBlocks(blocksFrom(t, invented), outline, 5, doc)
	if len(problems) != 1 || !strings.Contains(problems[0].Error, "not in the document: 45") {
		t.Fatalf("so 45 tu nghi ra phai bi tu choi: %+v", problems)
	}
	ok := `[{"type":"title","replace":"2-4","title":"CHỦ ĐỀ 3: KĨ THUẬT ĐÁNH CẦU THẤP TAY (13 tiết)","lines":["Bài 1: Cách cầm vợt, cầu, tư thế chuẩn bị và kĩ thuật đánh cầu thấp thuận tay (4 tiết)","Tiết 4: Hoàn thiện kĩ thuật đánh cầu thấp thuận tay"]},
		{"type":"note","before":5,"tone":"green","title":"Mẹo","lines":["- Mở đầu 8-10’: ôn nhanh động tác, ngày 10/10"]}]`
	if _, problems = planBlocks(blocksFrom(t, ok), outline, 5, doc); len(problems) != 0 {
		t.Fatalf("khoi thay dung va meo ngan phai qua: %+v", problems)
	}
}

// Nguoi dung 15/10/2026: co nguoi khong thich kieu nay -> lan goi dau hoi chon kieu; "keep" thi khong dong vao tai lieu.
func TestDocDesignAsksStyleFirst(t *testing.T) {
	asked := 0
	run := &RunContext{Choose: func(action, question string, choices []Choice) (string, string, error) {
		asked++
		if action != "doc_design" || len(choices) != 4 || choices[0].ID != "full" || !choices[3].Input {
			t.Fatalf("the hoi: %s %v", action, choices)
		}
		return "keep", "", nil
	}}
	tool := NewDocDesignTool()
	first := tool.Invoke(context.Background(), map[string]any{}, run)
	if !first.OK || first.Final || first.Mutating || !strings.Contains(first.JSON, "keep the document") {
		t.Fatalf("giu kieu cu: %+v", first)
	}
	again := tool.Invoke(context.Background(), map[string]any{"blocks": []any{}}, run)
	if again.OK || asked != 1 || !strings.Contains(again.JSON, "keep the current look") {
		t.Fatalf("goi lai sau khi chon giu kieu: %+v (hoi %d lan)", again, asked)
	}

	silent := &RunContext{Choose: func(string, string, []Choice) (string, string, error) { return "", "", nil }}
	if result := tool.Invoke(context.Background(), map[string]any{}, silent); !result.Final || result.Mutating {
		t.Fatalf("khong tra loi -> dung, khong doi gi: %+v", result)
	}

	// Yeu cau da noi ro kieu thi khong hoi; khong co ai de hoi thi dung kieu day du.
	run = &RunContext{Choose: func(string, string, []Choice) (string, string, error) {
		t.Fatal("khong duoc hoi")
		return "", "", nil
	}}
	if style, _, problem := chooseDocStyle(map[string]any{"style": "basic"}, run); style != "basic" || problem != "" {
		t.Fatalf("style=basic: %q %q", style, problem)
	}
	if style, _, _ := chooseDocStyle(map[string]any{"custom": map[string]any{"font": "Arial"}}, run); style != "custom" {
		t.Fatalf("yeu cau da mo ta kieu -> custom, khong hoi: %q", style)
	}
	if style, _, _ := chooseDocStyle(map[string]any{}, &RunContext{}); style != "full" {
		t.Fatalf("khong co ai de hoi: %q", style)
	}
	if _, _, problem := chooseDocStyle(map[string]any{"style": "fancy"}, run); problem == "" {
		t.Fatal("style la phai bao loi")
	}
}

// Nguoi dung 16/10/2026: them lua chon tu go kieu -> tool tra lai loi mo ta, model goi lai voi custom.
func TestDocDesignCustomStyle(t *testing.T) {
	run := &RunContext{Choose: func(string, string, []Choice) (string, string, error) {
		return "custom", "tiêu đề màu xanh lá, font Arial", nil
	}}
	tool := NewDocDesignTool()
	first := tool.Invoke(context.Background(), map[string]any{}, run)
	if !first.OK || first.Mutating || !strings.Contains(first.JSON, "xanh lá") || !strings.Contains(first.JSON, "custom") {
		t.Fatalf("phai tra loi mo ta cho model: %+v", first)
	}
	if again := tool.Invoke(context.Background(), map[string]any{}, run); again.OK || !strings.Contains(again.JSON, "xanh lá") {
		t.Fatalf("goi lai khong co custom -> nhac lai loi mo ta: %+v", again)
	}
	if bad := tool.Invoke(context.Background(), map[string]any{"custom": map[string]any{"accent": "green"}}, run); bad.OK ||
		!strings.Contains(bad.JSON, "#2E7D32") {
		t.Fatalf("mau khong dung #RRGGBB phai bao loi: %+v", bad)
	}

	options, boxes, problem := customStyle(map[string]any{"accent": "2e7d32", "font": " Arial ", "size": float64(14)})
	if problem != "" || boxes || options["accent"] != "#2E7D32" || options["font"] != "Arial" || options["size"] != float64(14) {
		t.Fatalf("custom: %v %v %q", options, boxes, problem)
	}
	if _, boxes, _ := customStyle(map[string]any{"boxes": true}); !boxes {
		t.Fatal("boxes true -> co khoi")
	}
	if _, _, problem := customStyle(map[string]any{"size": float64(40)}); problem == "" {
		t.Fatal("co chu 40 phai bao loi")
	}
}
