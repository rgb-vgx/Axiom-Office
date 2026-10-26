package slidelayout

// Grid: luoi slide cua skill trinh-bay-chuyen-nghiep (SKILL.md muc 1, references/bo-cuc-slide.md) cho che do dung tu
// brief - khong co tep mau nen vung lay tu co slide (wpp.checkLayout) va le `margin` cua _design/tokens.json.
// Truoc 25/10/2026 model tu tinh cac toa do nay roi goi wpp.addText (docs/plans/18-s0 muc 2).
type Grid struct {
	Width, Height, Margin float64
}

// Title: o tieu de - left margin, top 30, rong W - 2*margin, cao 80.
func (g Grid) Title() Rect { return Rect{g.Margin, 30, g.Width - 2*g.Margin, 80} }

// Content: vung noi dung - top 130, cao H - 170.
func (g Grid) Content() Rect { return Rect{g.Margin, 130, g.Width - 2*g.Margin, g.Height - 170} }

// Source: dong nguon - cach day 36, cao 24.
func (g Grid) Source() Rect { return Rect{g.Margin, g.Height - 36, g.Width - 2*g.Margin, 24} }

// Cover: ten bao cao (H x 0,34, cao 110) va dong phu ngay duoi (cach 122, cao 70).
func (g Grid) Cover() (title, subtitle Rect) {
	top := g.Height * 0.34
	return Rect{g.Margin, top, g.Width - 2*g.Margin, 110}, Rect{g.Margin, top + 122, g.Width - 2*g.Margin, 70}
}

// Decor: mot khoi trang tri khong chu. Kind: "band" (mau chinh), "base" (mau chinh nhat), "rule" (mau nhan).
type Decor struct {
	Kind string
	Box  Rect
}

// CoverDecor: trang tri bia - dai doc ben trai (trong le), khoi nhat o day (duoi dong phu), vach ngan giua ten va dong
// phu. Khong khoi nao de len vung chu cua Cover().
func (g Grid) CoverDecor() []Decor {
	title, subtitle := g.Cover()
	band := Round1(g.Margin * 0.45)
	baseTop := subtitle[1] + subtitle[3] + 24
	gap := subtitle[1] - (title[1] + title[3])
	return []Decor{
		{"band", Rect{0, 0, band, g.Height}},
		{"base", Rect{0, Round1(baseTop), g.Width, Round1(g.Height - baseTop)}},
		{"rule", Rect{Round1(g.Width/2 - 40), Round1(title[1] + title[3] + gap/2 - 2), 80, 4}},
	}
}

// Section: mot dong chu lon giua slide.
func (g Grid) Section() Rect { return Rect{g.Margin, (g.Height - 110) / 2, g.Width - 2*g.Margin, 110} }

// Figures: count (2-3) cot con so lon - so o H/2 - 20 (cao 60), nhan o H/2 + 45 (cao 60); cot cach nhau rãnh `gutter`.
func (g Grid) Figures(count int, gutter float64) (values, labels []Rect) {
	if count < 1 {
		return nil, nil
	}
	width := (g.Width - 2*g.Margin - float64(count-1)*gutter) / float64(count)
	for i := 0; i < count; i++ {
		left := Round1(g.Margin + float64(i)*(width+gutter))
		values = append(values, Rect{left, g.Height/2 - 20, Round1(width), 60})
		labels = append(labels, Rect{left, g.Height/2 + 45, Round1(width), 60})
	}
	return values, labels
}
