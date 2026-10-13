package pptx

import (
	"strings"
	"testing"
)

// Toa do lay tu mau that "mau bai giang.pptx" (slide 2: sidebar, ba muc I/II/III).
func sidebarShapes() []shapeInfo {
	shapes := []shapeInfo{{name: "TextBox 2", kind: "sp", box: rect{291.6, 39.6, 594, 46.8}, hasBox: true, text: "MỤC TIÊU HỌC TẬP", size: 28, bold: true}}
	for row, top := range []float64{0, 102, 204} {
		n := 3 + row*5
		name := func(prefix string, offset int) string {
			return prefix + " " + string(rune('0'+(n+offset)/10)) + string(rune('0'+(n+offset)%10))
		}
		shapes = append(shapes,
			shapeInfo{name: name("Rectangle", 0), kind: "sp", geom: "rect", box: rect{320, 156 + top, 583, 59}, hasBox: true},
			shapeInfo{name: name("Rectangle", 1), kind: "sp", geom: "rect", box: rect{310, 151 + top, 583, 59}, hasBox: true},
			shapeInfo{name: name("Oval", 2), kind: "sp", geom: "ellipse", box: rect{263, 144 + top, 68, 68}, hasBox: true},
			shapeInfo{name: name("TextBox", 3), kind: "sp", geom: "rect", box: rect{263, 155 + top, 68, 35}, hasBox: true, text: "I", size: 19, bold: true},
			shapeInfo{name: name("TextBox", 4), kind: "sp", geom: "rect", box: rect{340, 156 + top, 529, 48}, hasBox: true, text: "Trình bày được", size: 17, bold: true},
		)
	}
	return shapes
}

func TestRepeatedItemsFindsThreeRows(t *testing.T) {
	items := repeatedItems(sidebarShapes())
	if len(items) != 3 {
		t.Fatalf("muc lap = %d, muon 3", len(items))
	}
	for _, item := range items {
		if len(item) != 5 {
			t.Fatalf("moi muc 5 shape, duoc %d", len(item))
		}
	}
	names := []string{}
	for _, shape := range items[1] {
		names = append(names, shape.name)
	}
	if got := strings.Join(names, ","); !strings.Contains(got, "TextBox 12") || strings.Contains(got, "TextBox 7") {
		t.Fatalf("muc 2 = %s", got)
	}
}

func TestRepeatedItemsNoneForDistinctShapes(t *testing.T) {
	logos := []shapeInfo{
		{name: "Picture 65", kind: "pic", box: rect{223, 46, 165, 197}, hasBox: true},
		{name: "Picture 9", kind: "pic", box: rect{401, 37, 274, 205}, hasBox: true},
	}
	if items := repeatedItems(logos); items != nil {
		t.Fatalf("logo khac co khong phai muc lap: %v", items)
	}
}

func TestCapacityUsesShapeBelowAndContainer(t *testing.T) {
	shapes := sidebarShapes()
	slide := rect{0, 0, 960, 540}
	// Tieu de 28 pt: Oval muc I o y=144 chan duoi -> 2 dong (do that: 57 ky tu = 2 dong).
	lines, perLine := capacity(shapes[0], shapes, slide)
	if lines != 2 || perLine < 34 || perLine > 40 {
		t.Fatalf("tieu de: %d dong x %d ky tu", lines, perLine)
	}
	// O noi dung muc I nam trong o mau cao 59 pt -> 2 dong, ~56 ky tu/dong (do that 57).
	lines, perLine = capacity(shapes[5], shapes, slide)
	if lines != 2 || perLine < 52 || perLine > 60 {
		t.Fatalf("o muc: %d dong x %d ky tu", lines, perLine)
	}
}

func TestCoversBackgroundEvenWhenShifted(t *testing.T) {
	slide := rect{0, 0, 960, 540}
	if !covers(rect{0, -22, 960, 540}, slide) {
		t.Fatal("anh bia lech 22 pt van la nen")
	}
	if covers(rect{401, 37, 274, 205}, slide) {
		t.Fatal("logo khong phai nen")
	}
}

func TestLargestTrueFindsWhiteAreaRightOfSidebar(t *testing.T) {
	grid := make([][]bool, 6)
	for r := range grid {
		grid[r] = make([]bool, 10)
		for c := range grid[r] {
			grid[r][c] = c >= 3 // cot 0-2 la sidebar
		}
	}
	c0, r0, c1, r1, ok := largestTrue(grid)
	if !ok || c0 != 3 || c1 != 10 || r0 != 0 || r1 != 6 {
		t.Fatalf("largestTrue = %d,%d - %d,%d %v", c0, r0, c1, r1, ok)
	}
}
