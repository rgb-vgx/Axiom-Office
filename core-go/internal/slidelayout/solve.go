package slidelayout

import "math"

// Spec: phan HINH HOC cua mot slide (tang ngu nghia SlideSpec cua docs/plans/18-s0 muc 5.1 - chi nhung gi anh huong
// toi vi tri va co chu). Noi dung (chu cua tung y, gia tri bang, anh nguon) do ben dung ghep vao theo Box.Role/Index.
type Spec struct {
	// Layout: "cards", "table"; con lai la vung noi dung (chu, anh, chu thich).
	Layout string
	// HasPoints: slide co y - co anh thi chu chiem cot trai. ShowPoints: ve hop y (che do "update" lay y tu nguon khac).
	HasPoints, ShowPoints bool
	Callout, Caption      string
	Cards                 []Card
	// Pictures: kich thuoc goc tung anh (rong, cao); 0 = khong ro, coi la 4:3.
	Pictures [][2]float64
}

// Theme: co chu thiet ke va san co chu (bridge tu co tu Size xuong Min khi chu khong vua; Min = Size la khong co).
type Theme struct {
	Points, PointsMin   float64
	Card, CardMin       float64
	Callout, CalloutMin float64
	Caption, CaptionMin float64
}

// TemplateTheme: co chu cua che do bien soan theo mau (gia tri da chay that tu 10/2026; giu nguyen o M3).
var TemplateTheme = Theme{Points: 24, PointsMin: 14, Card: CardMaxFont, CardMin: 12, Callout: CalloutFont, CalloutMin: 12,
	Caption: 12, CaptionMin: 10}

// Box: mot hop da giai - vi tri, co chu, neo doc. Role: "points", "card", "grid", "caption", "picture", "callout";
// Index: the thu may / anh thu may.
type Box struct {
	Role      string
	Index     int
	Rect      Rect
	Size, Min float64
	Anchor    string
}

// Solve: giai hinh hoc cua mot slide trong vung noi dung `area`. Thu tu hop la thu tu dung (de anh/chu de dung lop).
func Solve(spec Spec, area Rect, theme Theme) []Box {
	withPictures := len(spec.Pictures) > 0
	text, pictureArea, callout := ContentRegions(area, spec.Callout, theme.Callout, spec.HasPoints, withPictures)
	boxes := []Box{}
	switch spec.Layout {
	case "cards":
		for c, rect := range FitCards(text, spec.Cards) {
			boxes = append(boxes, Box{Role: "card", Index: c, Rect: rect, Size: theme.Card, Min: theme.CardMin, Anchor: "middle"})
		}
	case "table":
		boxes = append(boxes, Box{Role: "grid", Rect: text})
	default:
		if spec.ShowPoints {
			anchor := "middle"
			if withPictures {
				anchor = "top"
			}
			boxes = append(boxes, Box{Role: "points", Rect: text, Size: theme.Points, Min: theme.PointsMin, Anchor: anchor})
		}
		if withPictures {
			box := pictureArea
			if spec.Caption != "" {
				height := math.Round(float64(EstimateLines(spec.Caption, box[2], theme.Caption))*1.21*theme.Caption + 10)
				box[3] -= height + 6
				boxes = append(boxes, Box{Role: "caption", Rect: Rect{box[0], box[1] + box[3] + 6, box[2], height},
					Size: theme.Caption, Min: theme.CaptionMin, Anchor: "top"})
			}
			for i, rect := range PictureBoxes([]float64{box[0], box[1], box[2], box[3]}, spec.Pictures) {
				boxes = append(boxes, Box{Role: "picture", Index: i, Rect: rect})
			}
		}
	}
	if spec.Callout != "" {
		boxes = append(boxes, Box{Role: "callout", Rect: callout, Size: theme.Callout, Min: theme.CalloutMin, Anchor: "middle"})
	}
	return boxes
}
