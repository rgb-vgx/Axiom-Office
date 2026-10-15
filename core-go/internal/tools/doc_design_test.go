package tools

import (
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
	out, problems := planBlocks(list, designOutline(), 40)
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
		_, problems := planBlocks(blocksFrom(t, input), designOutline(), 40)
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
