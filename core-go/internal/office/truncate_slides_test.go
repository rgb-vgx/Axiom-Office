package office

import (
	"encoding/json"
	"strings"
	"testing"
)

// Bai 87 slide nhu luot 13/10/2026: ket qua doc ca bai phai gon lai nhung giu du so va giu slide dang mo.
func deck(slides int, textPerSlide string, issuesEvery int) (map[string]any, map[string]any) {
	list := []any{}
	layout := []any{}
	issueCount := 0
	for i := 1; i <= slides; i++ {
		list = append(list, map[string]any{"index": float64(i), "shapeTexts": []any{"Tieu de " + itoa(i), textPerSlide}})
		issues := []any{}
		if issuesEvery > 0 && i%issuesEvery == 0 {
			issues = append(issues, map[string]any{"type": "overflow", "detail": "x"}, map[string]any{"type": "overlap", "detail": "y"})
			issueCount += 2
		}
		layout = append(layout, map[string]any{"index": float64(i), "issues": issues,
			"shapes": []any{map[string]any{"name": "Title", "left": 1.0}, map[string]any{"name": "Body", "left": 2.0}}})
	}
	return map[string]any{"presentation": "a.ppt", "slideCount": float64(slides), "slides": list},
		map[string]any{"slideWidth": 720.0, "slideHeight": 540.0, "issueCount": float64(issueCount), "slides": layout}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func TestListSlidesShortensButKeepsFocusSlide(t *testing.T) {
	long := strings.Repeat("noi dung dai ", 60)
	list, _ := deck(87, long, 0)
	limits := DefaultTruncation
	limits.FocusSlide = 40
	raw := limits.Apply("wpp.listSlides", list, "goc")
	if raw == "goc" || !strings.Contains(raw, `"truncated":true`) || !strings.Contains(raw, `"slideCount":87`) {
		t.Fatalf("listSlides phai rut gon va giu slideCount: %.300s", raw)
	}
	if len(raw) > 2*SlideTextBudget {
		t.Fatalf("van qua lon: %d ky tu", len(raw))
	}
	slides := list["slides"].([]any)
	for _, index := range []int{39, 40, 41} {
		if _, full := slides[index-1].(map[string]any)["shapeTexts"]; !full {
			t.Fatalf("slide %d (dang mo hoac ke ben) phai giu nguyen", index)
		}
	}
	if text, _ := slides[0].(map[string]any)["text"].(string); !strings.HasPrefix(text, "Tieu de 1 |") {
		t.Fatalf("slide xa phai con tieu de: %q", text)
	}

	short, _ := deck(5, "ngan", 0)
	if DefaultTruncation.Apply("wpp.listSlides", short, "goc") != "goc" {
		t.Fatal("bai nho khong duoc dong cham")
	}
}

func TestCheckLayoutDropsCleanSlidesAndKeepsCounts(t *testing.T) {
	_, layout := deck(87, "", 10) // 8 slide co loi (10, 20, ..., 80), 16 loi
	limits := DefaultTruncation
	limits.FocusSlide = 33
	raw := limits.Apply("wpp.checkLayout", layout, "goc")
	if raw == "goc" || !strings.Contains(raw, `"issueCount":16`) || !strings.Contains(raw, `"slidesChecked":87`) ||
		!strings.Contains(raw, `"slidesWithoutIssues":78`) {
		t.Fatalf("checkLayout phai giu so: %.400s", raw)
	}
	kept := layout["slides"].([]any)
	if len(kept) != 9 { // 8 slide co loi + slide dang mo
		t.Fatalf("giu %d slide, muon 9", len(kept))
	}
	withShapes := 0
	for _, item := range kept {
		slide := item.(map[string]any)
		if _, ok := slide["shapes"]; ok {
			withShapes++
		}
	}
	if withShapes != LayoutDetailSlides+1 {
		t.Fatalf("toa do khung giu cho %d slide, muon %d", withShapes, LayoutDetailSlides+1)
	}

	_, one := deck(1, "", 1)
	if DefaultTruncation.Apply("wpp.checkLayout", one, "goc") != "goc" {
		t.Fatal("soat mot slide phai giu nguyen")
	}
}
