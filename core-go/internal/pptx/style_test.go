package pptx

import (
	"strings"
	"testing"
)

func TestAspectNames(t *testing.T) {
	cases := map[string][2]int64{
		"16:9":  {12192000, 6858000}, // mau "mau bai giang.pptx"
		"4:3":   {9144000, 6858000},  // bai .ppt cu 720x540 pt
		"16:10": {12192000, 7620000},
	}
	for want, size := range cases {
		if got := aspect(size[0], size[1]); got != want {
			t.Errorf("aspect(%d, %d) = %q, muon %q", size[0], size[1], got, want)
		}
	}
	if got := slideSize(12192000, 6858000); got["widthPt"] != 960.0 || got["heightPt"] != 540.0 {
		t.Fatalf("slideSize = %v", got)
	}
	if slideSize(0, 6858000) != nil {
		t.Fatal("kich thuoc hong thi khong tra gi")
	}
}

func TestTopOrdersByUseThenName(t *testing.T) {
	got := top(map[string]int{"#C00000": 9, "#FFFFFF": 12, "#1F4E79": 9, "#000000": 1}, 3)
	if strings.Join(got, ",") != "#FFFFFF,#1F4E79,#C00000" {
		t.Fatalf("top = %v", got)
	}
}
