package slidelayout

import (
	"reflect"
	"testing"
)

var content = Rect{40, 130, 880, 370} // vung noi dung cua luoi o 960 x 540

func inside(t *testing.T, name string, box Rect) {
	t.Helper()
	if box[0] < content[0]-0.1 || box[1] < content[1]-0.1 || box[0]+box[2] > content[0]+content[2]+0.1 ||
		box[1]+box[3] > content[1]+content[3]+0.1 {
		t.Fatalf("%s ra ngoai vung noi dung: %v", name, box)
	}
}

func overlaps(a, b Rect) bool {
	return a[0] < b[0]+b[2] && b[0] < a[0]+a[2] && a[1] < b[1]+b[3] && b[1] < a[1]+a[3]
}

func TestProcessStaysInsideAndLeavesRoomForArrows(t *testing.T) {
	for count := 2; count <= 5; count++ {
		boxes := Process(content, count)
		for i, box := range boxes {
			inside(t, "buoc", box)
			if i > 0 && boxes[i][0]-(boxes[i-1][0]+boxes[i-1][2]) < FlowGap-0.2 {
				t.Fatalf("%d buoc: khe cho mui ten qua hep %v", count, boxes)
			}
		}
	}
}

func TestTimelineColumnsDoNotOverlap(t *testing.T) {
	axis, dots, dates, texts := Timeline(content, 4)
	inside(t, "truc", axis)
	for i := range dots {
		inside(t, "ngay", dates[i])
		inside(t, "noi dung", texts[i])
		if overlaps(dates[i], texts[i]) || overlaps(dates[i], dots[i]) || overlaps(texts[i], dots[i]) {
			t.Fatalf("moc %d chong nhau: %v %v %v", i, dates[i], dots[i], texts[i])
		}
		if i > 0 && overlaps(texts[i-1], texts[i]) {
			t.Fatalf("hai cot chong nhau: %v %v", texts[i-1], texts[i])
		}
	}
}

func TestDiagramLayersAreCentredAndSeparate(t *testing.T) {
	rows := Diagram(content, []int{1, 1, 3})
	if len(rows) != 3 || len(rows[2]) != 3 {
		t.Fatalf("so do = %v", rows)
	}
	for _, row := range rows {
		for _, box := range row {
			inside(t, "nut", box)
		}
	}
	// Tang 1 nut can giua ngang; tang sau nam duoi tang truoc, cach LayerGap.
	if center := rows[0][0][0] + rows[0][0][2]/2; center != 480 {
		t.Fatalf("nut don phai can giua: %v", rows[0][0])
	}
	if gap := rows[1][0][1] - (rows[0][0][1] + rows[0][0][3]); gap < LayerGap-0.2 {
		t.Fatalf("hai tang qua sat: %v", rows)
	}
}

// Solve giu thu tu dung: chu truoc, anh sau, o luu y cuoi.
func TestSolveOrder(t *testing.T) {
	boxes := Solve(Spec{Layout: "content", HasPoints: true, ShowPoints: true, Caption: "c", Callout: "!",
		Pictures: [][2]float64{{4, 3}}}, content, TemplateTheme)
	roles := []string{}
	for _, box := range boxes {
		roles = append(roles, box.Role)
	}
	if !reflect.DeepEqual(roles, []string{"points", "caption", "picture", "callout"}) {
		t.Fatalf("thu tu = %v", roles)
	}
}

// Trang tri bia nam ngoai vung chu (ten, dong phu) va trong slide.
func TestCoverDecorStaysOffText(t *testing.T) {
	for _, grid := range []Grid{{960, 540, 40}, {720, 540, 40}} {
		title, subtitle := grid.Cover()
		decor := grid.CoverDecor()
		if len(decor) != 3 {
			t.Fatalf("3 khoi trang tri: %v", decor)
		}
		for _, item := range decor {
			box := item.Box
			if box[0] < 0 || box[1] < 0 || box[0]+box[2] > grid.Width+0.1 || box[1]+box[3] > grid.Height+0.1 || box[3] <= 0 {
				t.Fatalf("%s ra ngoai slide: %v", item.Kind, box)
			}
			for _, text := range []Rect{title, subtitle} {
				if box[0] < text[0]+text[2] && text[0] < box[0]+box[2] && box[1] < text[1]+text[3] && text[1] < box[1]+box[3] {
					t.Fatalf("%s %v de len vung chu %v", item.Kind, box, text)
				}
			}
		}
	}
}
