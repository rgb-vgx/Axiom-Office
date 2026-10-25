package tools

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"axiomoffice/core/internal/pptx"
)

// Test characterization cho phan hinh hoc va kiem bo cuc cua che do bien soan (docs/plans/18-s0-slide-do-va-spec.md):
// ghim dung so do hien tai truoc khi tach solver ra package rieng. So doi o day phai co ly do, khong sua cho qua.

var solverArea = [4]float64{48, 100, 840, 400}

func manyPoints(count, length int) []sourcePara {
	points := make([]sourcePara, count)
	for i := range points {
		points[i] = sourcePara{T: strings.Repeat("x", length)}
	}
	return points
}

func TestContentRegionsCharacterization(t *testing.T) {
	points := []sourcePara{{T: "a"}}
	cases := []struct {
		name                      string
		row                       composeRow
		pictures                  bool
		text, pictureBox, callout [4]float64
	}{
		{"chi chu: ca vung", composeRow{Points: points}, false,
			solverArea, solverArea, [4]float64{}},
		{"chu + anh: chu 42% ben trai, cach 16", composeRow{Points: points}, true,
			[4]float64{48, 100, 353, 400}, [4]float64{417, 100, 471, 400}, [4]float64{}},
		{"chi anh: khong vung chu", composeRow{}, true,
			[4]float64{}, solverArea, [4]float64{}},
		{"o luu y mot dong: dai 38 cuoi vung", composeRow{Points: points, Callout: "Lưu ý ngắn"}, false,
			[4]float64{48, 100, 840, 350}, [4]float64{48, 100, 840, 350}, [4]float64{48, 462, 840, 38}},
		{"o luu y dai: tran 90", composeRow{Points: points, Callout: strings.Repeat("x", 400)}, true,
			[4]float64{48, 100, 353, 298}, [4]float64{417, 100, 471, 298}, [4]float64{48, 410, 840, 90}},
	}
	for _, c := range cases {
		text, pictureBox, callout := contentRegions(solverArea, c.row, c.pictures)
		if text != c.text || pictureBox != c.pictureBox || callout != c.callout {
			t.Errorf("%s: text=%v pictures=%v callout=%v", c.name, text, pictureBox, callout)
		}
	}
}

func TestCardBoxesCharacterization(t *testing.T) {
	cases := map[int][][4]float64{
		2: {{48, 100, 413, 400}, {475, 100, 413, 400}},
		3: {{48, 100, 270.7, 400}, {332.7, 100, 270.7, 400}, {617.3, 100, 270.7, 400}},
		4: {{48, 100, 413, 193}, {475, 100, 413, 193}, {48, 307, 413, 193}, {475, 307, 413, 193}},
	}
	for count, want := range cases {
		if got := cardBoxes(solverArea, count); !reflect.DeepEqual(got, want) {
			t.Errorf("%d the = %v", count, got)
		}
	}
}

func TestFitCardsCharacterization(t *testing.T) {
	short := composeCard{Title: "A", Points: []sourcePara{{T: "b"}}}
	// Chu it: the cao toi thieu 120, ca cum can giua doc.
	want := [][4]float64{{48, 240, 270.7, 120}, {332.7, 240, 270.7, 120}, {617.3, 240, 270.7, 120}}
	if got := fitCards(solverArea, []composeCard{short, short, short}); !reflect.DeepEqual(got, want) {
		t.Errorf("ba the ngan = %v", got)
	}
	// Chu nhieu: tran o chieu cao vung, khong vuot.
	long := composeCard{Title: "Tiêu đề", Points: []sourcePara{{T: strings.Repeat("x", 300)}, {T: strings.Repeat("x", 300)}}}
	want = [][4]float64{{48, 100, 413, 400}, {475, 100, 413, 400}}
	if got := fitCards(solverArea, []composeCard{long, short}); !reflect.DeepEqual(got, want) {
		t.Errorf("the dai = %v", got)
	}
	if got := fitCards(solverArea, nil); len(got) != 0 {
		t.Errorf("khong the = %v", got)
	}
}

func TestPictureBoxesCharacterization(t *testing.T) {
	area := []float64{48, 100, 840, 400}
	cases := []struct {
		name  string
		sizes [][2]float64
		want  [][4]float64
	}{
		{"mot anh 4:3 can giua, cao het vung", [][2]float64{{400, 300}},
			[][4]float64{{201.3, 100, 533.3, 400}}},
		{"ba anh khac ti le: hai hang", [][2]float64{{400, 300}, {300, 400}, {800, 200}},
			[][4]float64{{197.6, 100, 338.5, 253.9}, {548, 100, 190.4, 253.9}, {199.7, 365.9, 536.5, 134.1}}},
		{"anh khong ro co (4:3) + anh nho khong phong qua 2 lan", [][2]float64{{0, 0}, {100, 50}},
			[][4]float64{{244.8, 100, 446.5, 334.8}, {414.8, 446.8, 106.3, 53.2}}},
	}
	for _, c := range cases {
		if got := pictureBoxes(area, c.sizes); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// Ma loi cua checkCompose theo tung kieu: model doc ma nay de sua dong ke hoach, nen ma la mot phan cua hop dong.
func TestCheckComposeErrorCodes(t *testing.T) {
	_, roles := testRoles(t)
	byUse := map[string]*pptx.Kind{"content": roles.picture, "cover": roles.cover}
	point := func(text string) []sourcePara { return []sourcePara{{T: text}} }
	card := composeCard{Title: "Ưu điểm", Points: point("Rẻ, nhanh")}
	table := func(rows, cols int, cell string) [][]string {
		out := make([][]string, rows)
		for r := range out {
			out[r] = make([]string, cols)
			for c := range out[r] {
				out[r][c] = cell
			}
		}
		return out
	}
	cases := []struct {
		name, field, code string
		row               composeRow
	}{
		{"thieu tieu de", "title", "MISSING", composeRow{Layout: "content", Points: point("a")}},
		{"kieu la", "layout", "UNKNOWN_LAYOUT", composeRow{Layout: "timeline", Title: "T"}},
		{"bia co y", "points", "ONLY_TITLE_AND_SUBTITLE", composeRow{Layout: "cover", Title: "T", Points: point("a")}},
		{"muc tieu rong", "points", "NEED_1_TO_8_POINTS", composeRow{Layout: "agenda", Title: "T"}},
		{"noi dung rong", "points", "EMPTY", composeRow{Layout: "content", Title: "T"}},
		{"y qua dai", "points", "POINT_TOO_LONG", composeRow{Layout: "content", Title: "T", Points: point(strings.Repeat("x", 221))}},
		{"noi dung khong vua", "points", "TOO_LONG", composeRow{Layout: "content", Title: "T", Points: manyPoints(15, 200)}},
		{"chu thich khong anh", "caption", "CAPTION_WITHOUT_PICTURES", composeRow{Layout: "content", Title: "T", Points: point("a"), Caption: "c"}},
		{"mot the", "cards", "NEED_2_TO_4_CARDS", composeRow{Layout: "cards", Title: "T", Cards: []composeCard{card}}},
		{"nam the", "cards", "NEED_2_TO_4_CARDS", composeRow{Layout: "cards", Title: "T", Cards: []composeCard{card, card, card, card, card}}},
		{"the thieu y", "cards[1]", "NEEDS_TITLE_AND_POINTS", composeRow{Layout: "cards", Title: "T", Cards: []composeCard{card, {Title: "B"}}}},
		{"the qua dai", "cards[0]", "TOO_LONG", composeRow{Layout: "cards", Title: "T",
			Cards: []composeCard{{Title: "A", Points: point(strings.Repeat("x", 2000))}, card}}},
		{"bang mot dong", "table", "TABLE_SIZE", composeRow{Layout: "table", Title: "T", Table: table(1, 3, "a")}},
		{"bang sau cot", "table", "TABLE_SIZE", composeRow{Layout: "table", Title: "T", Table: table(3, 6, "a")}},
		{"o bang dai", "table", "CELL_TOO_LONG", composeRow{Layout: "table", Title: "T", Table: table(2, 2, strings.Repeat("x", 91))}},
	}
	for _, c := range cases {
		_, errors, _ := checkCompose([]composeRow{c.row}, nil, nil, byUse, nil)
		found := []string{}
		ok := false
		for _, e := range errors {
			found = append(found, fmt.Sprintf("%s:%s", e.Field, e.Error))
			ok = ok || (e.Field == c.field && e.Error == c.code)
		}
		if !ok {
			t.Errorf("%s: muon %s:%s, co %v", c.name, c.field, c.code, found)
		}
	}

	// Dong hop le cua tung kieu khong bao loi (khong co doan nguon nen khong co loi coverage).
	valid := []composeRow{
		{Layout: "cover", Title: "Bài giảng", Subtitle: "ThS. A"},
		{Layout: "agenda", Title: "Mục tiêu", Points: []sourcePara{{T: "Một"}, {T: "Hai"}, {T: "Ba"}}},
		{Layout: "content", Title: "Nội dung", Points: point("Ý chính"), Callout: "⚠ Lưu ý"},
		{Layout: "cards", Title: "Ưu và nhược", Cards: []composeCard{card, {Title: "Hạn chế", Points: point("Độ nhạy thấp")}}},
		{Layout: "table", Title: "So sánh", Table: table(3, 3, "a")},
	}
	if _, errors, _ := checkCompose(valid, nil, nil, byUse, nil); len(errors) > 0 {
		t.Fatalf("dong hop le bi bao loi: %+v", errors)
	}
}
