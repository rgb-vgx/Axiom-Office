package tools

import (
	"strings"
	"testing"

	"axiomoffice/core/internal/pptx"
)

func composeViews() []sourceView {
	return []sourceView{
		{slide: sourceSlide{N: 1, Title: "CĐHA CẤP CỨU BỤNG"}, paras: []sourcePara{{T: "THS PHAN NHẬT ANH"}}},
		{slide: sourceSlide{N: 2, Title: "1. Xquang thường qui", Pictures: []sourcePicture{{N: 1, W: 400, H: 300}, {N: 2, W: 400, H: 300}}},
			paras: []sourcePara{{T: "Kỹ thuật: đứng, bụng áp vào phim"}, {T: "Tia trung tâm: điểm nối đường giữa"}}},
		{slide: sourceSlide{N: 3, Title: "1. Xquang thường qui", Pictures: []sourcePicture{{N: 1, W: 400, H: 300}}},
			paras: []sourcePara{{T: "Ưu điểm: rẻ, nhanh"}, {T: "Hạn chế: độ nhạy thấp"}}},
	}
}

// Bien soan: thieu "from" / anh -> bao loi; du -> ghi chu mang nguyen van, anh bo co chu dich duoc ghi lai.
func TestComposeChecksCoverageAndKeepsOriginalInNotes(t *testing.T) {
	_, roles := testRoles(t)
	byUse := map[string]*pptx.Kind{"content": roles.picture, "cover": roles.cover}
	rows, parseErrors := parseCompose([]any{
		map[string]any{"layout": "cover", "title": "CĐHA CẤP CỨU BỤNG", "subtitle": "ThS. Phan Nhật Anh", "from": []any{"1"}},
		map[string]any{"layout": "content", "title": "XQUANG BKCB – TƯ THẾ ĐỨNG", "from": []any{"2"},
			"points": []any{"**Đứng**, bụng áp phim", "- Tia trung tâm ở đường giữa"}, "imgs": []any{"2:i1"},
			"caption": "Tư thế đứng chụp bụng", "callout": "⚠ Nghi thủng: bắt buộc lấy vòm hoành"},
	})
	if len(parseErrors) > 0 || rows[1].Points[1].L != 2 || !rows[1].Points[0].U {
		t.Fatalf("doc dong: %+v %+v", rows, parseErrors)
	}
	_, errors, _ := checkCompose(rows, nil, composeViews(), byUse, nil)
	joined := ""
	for _, e := range errors {
		joined += e.Error + " " + e.Allowed + "\n"
	}
	for _, want := range []string{"NOT_COVERED", "2:i2", "3:p1", "3:i1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("thieu %q trong loi:\n%s", want, joined)
		}
	}

	rows = append(rows, composeRow{Layout: "cards", Title: "VAI TRÒ XQUANG BKCB", From: []string{"3:p1-2"},
		Cards: []composeCard{{Title: "Ưu điểm", Points: []sourcePara{{T: "Rẻ, nhanh", U: true}}},
			{Title: "Hạn chế", Points: []sourcePara{{T: "Độ nhạy thấp", U: true}}}}})
	plans, errors, dropped := checkCompose(rows, []string{"2:i2", "3:i1"}, composeViews(), byUse, nil)
	if len(errors) > 0 {
		t.Fatalf("bo cuc du phai qua: %+v", errors)
	}
	if !strings.Contains(plans[1].notes, "Nguyên văn bài gốc:\nKỹ thuật: đứng, bụng áp vào phim\nTia trung tâm") ||
		len(dropped) != 2 || len(plans[1].pictures) != 1 {
		t.Fatalf("notes=%q dropped=%v", plans[1].notes, dropped)
	}
}

func TestComposeLayoutHelpers(t *testing.T) {
	if got := tint("#C00000", 0.9); got != "#F9E6E6" {
		t.Fatalf("tint = %s", got)
	}
	boxes := cardBoxes([4]float64{0, 0, 900, 400}, 4)
	if len(boxes) != 4 || boxes[1][0] != 457 || boxes[2][1] != 207 {
		t.Fatalf("luoi the 2x2 = %v", boxes)
	}
	text, pictures, callout := contentRegions([4]float64{0, 100, 900, 400}, composeRow{Points: []sourcePara{{T: "a"}}, Callout: "Lưu ý"}, true)
	if callout[3] == 0 || text[2] != 378 || pictures[0] != 394 || text[1]+text[3] > callout[1] {
		t.Fatalf("vung: text=%v pictures=%v callout=%v", text, pictures, callout)
	}
}

// The it chu khong cao het vung (slide 19-22 luot that 14/10/2026): cao vua noi dung, cum the can giua.
func TestFitCardsShrinksToContent(t *testing.T) {
	area := [4]float64{48, 100, 840, 400}
	boxes := fitCards(area, []composeCard{{Title: "Chỉ định", Points: []sourcePara{{T: "Tắc ruột non không điển hình"}}},
		{Title: "Mục đích", Points: []sourcePara{{T: "Chẩn đoán vị trí tắc"}}}})
	if boxes[0][3] >= 300 || boxes[0][1] <= 100 || boxes[0][1]+boxes[0][3] >= 500 || boxes[1][1] != boxes[0][1] {
		t.Fatalf("the = %v", boxes)
	}
	four := fitCards(area, make([]composeCard, 4))
	if four[2][1] <= four[0][1] || four[1][1] != four[0][1] {
		t.Fatalf("luoi 2x2 = %v", four)
	}
}
