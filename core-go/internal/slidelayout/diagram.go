package slidelayout

import "math"

// Hinh hoc cho slide quy trinh, dong thoi gian, so do (docs/plans/18-s0 P2). Ben dung ve bang hinh khoi va duong noi
// goc cua PowerPoint (wpp.addShape, wpp.addConnector) nen nguoi dung keo sua duoc; duong noi bam theo hinh.

const (
	// FlowGap: khe giua hai buoc / hai nut cung tang - du cho mui ten.
	FlowGap = 36.0
	// LayerGap: khe giua hai tang cua so do.
	LayerGap = 44.0
)

// Process: `count` buoc xep mot hang, cao toi da 170, ca hang can giua doc trong vung.
func Process(area Rect, count int) []Rect {
	if count < 1 {
		return nil
	}
	width := (area[2] - FlowGap*float64(count-1)) / float64(count)
	height := math.Min(area[3], 170)
	top := area[1] + (area[3]-height)/2
	boxes := []Rect{}
	for i := 0; i < count; i++ {
		boxes = append(boxes, Rect{Round1(area[0] + float64(i)*(width+FlowGap)), Round1(top), Round1(width), Round1(height)})
	}
	return boxes
}

// Timeline: truc ngang o 40% chieu cao vung; moi moc mot cot rong bang nhau - cham tron tren truc, ngay o tren,
// noi dung o duoi.
func Timeline(area Rect, count int) (axis Rect, dots, dates, texts []Rect) {
	if count < 1 {
		return Rect{}, nil, nil, nil
	}
	column := area[2] / float64(count)
	y := area[1] + area[3]*0.4
	const dot = 18.0
	axis = Rect{Round1(area[0] + column/2), Round1(y - 1.5), Round1(column * float64(count-1)), 3}
	for i := 0; i < count; i++ {
		center := area[0] + column*(float64(i)+0.5)
		dots = append(dots, Rect{Round1(center - dot/2), Round1(y - dot/2), dot, dot})
		dates = append(dates, Rect{Round1(center - column/2 + 4), Round1(y - dot/2 - 8 - 40), Round1(column - 8), 40})
		texts = append(texts, Rect{Round1(center - column/2 + 4), Round1(y + dot/2 + 10), Round1(column - 8),
			Round1(math.Max(40, area[1]+area[3]-(y+dot/2+10)))})
	}
	return axis, dots, dates, texts
}

// Diagram: layers[i] = so nut cua tang i (tren xuong duoi). Nut rong toi da 240, cao toi da 90; moi tang can giua
// ngang, ca so do can giua doc.
func Diagram(area Rect, layers []int) [][]Rect {
	if len(layers) == 0 {
		return nil
	}
	rows := float64(len(layers))
	height := math.Min(90, (area[3]-LayerGap*(rows-1))/rows)
	total := height*rows + LayerGap*(rows-1)
	top := area[1] + (area[3]-total)/2
	result := [][]Rect{}
	for l, count := range layers {
		row := []Rect{}
		if count > 0 {
			width := math.Min(240, (area[2]-FlowGap*float64(count-1))/float64(count))
			span := width*float64(count) + FlowGap*float64(count-1)
			left := area[0] + (area[2]-span)/2
			y := top + float64(l)*(height+LayerGap)
			for i := 0; i < count; i++ {
				row = append(row, Rect{Round1(left + float64(i)*(width+FlowGap)), Round1(y), Round1(width), Round1(height)})
			}
		}
		result = append(result, row)
	}
	return result
}
