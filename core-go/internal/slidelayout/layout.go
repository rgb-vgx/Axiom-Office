// Package slidelayout: hinh hoc slide thuan (khong bridge, khong pptx.Kind) - chia vung, xep the, xep anh, uoc
// luong chu. Tach tu internal/tools (template_compose.go, template_build.go) o buoc M2 cua docs/plans/18-s0:
// cong thuc giu NGUYEN VAN (ke ca cach lam tron) - test characterization va golden chuoi lenh bridge trong
// internal/tools ghim dieu do. Doi cong thuc la mot thay doi rieng, co so do tren PowerPoint.
package slidelayout

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Rect: [left, top, width, height] theo point.
type Rect = [4]float64

const (
	// PictureGap: khoang cach giua anh, giua anh va chu thich, giua vung noi dung va o luu y.
	PictureGap = 12.0
	// CalloutFont: co chu o luu y.
	CalloutFont = 15.0
	// CardGap: khoang cach giua cac the.
	CardGap = 14.0
	// CardMaxFont: co chu toi da trong the (luot that 14/10/2026: the cao het vung noi dung, chu 18 pt chiem mot goc).
	CardMaxFont = 22.0
	// MaxUpscale: anh khong phong qua 2 lan co tren slide nguon (anh chup/scan cu do phan giai thap se vo hat).
	MaxUpscale = 2.0
)

// Card: the voi tieu de va cac y (chu thuan, da bo dau nhan "**").
type Card struct {
	Title  string
	Points []string
}

func Round1(value float64) float64 { return math.Round(value*10) / 10 }

// EstimateLines: so dong cua mot doan (ky tu/dong = rong / (0,53 x co)).
func EstimateLines(text string, width, size float64) int {
	perLine := max(int(width/(0.53*size)), 1)
	return max(1, (len([]rune(text))+perLine-1)/perLine)
}

// TextFits: doan van vua khung (cong thuc do cua skill: dong cao 1,21 x co, ky tu/dong = rong / (0,53 x co)).
func TextFits(text string, width, height, size float64) bool {
	perLine := int(width / (0.53 * size))
	maxLines := int((height - 7.2) / (1.21 * size))
	if perLine < 1 || maxLines < 1 {
		return false
	}
	lines := 0
	for _, paragraph := range strings.Split(text, "\n") {
		lines += max(1, (len([]rune(paragraph))+perLine-1)/perLine)
	}
	return lines <= maxLines
}

// ContentRegions: chia vung noi dung thanh chu / anh / o luu y. Co anh: chu cot trai 42% (hoac khong co vung chu khi
// khong co y nao), anh ben phai; o luu y la dai cuoi vung (cao toi da 90), uoc theo co chu calloutSize.
func ContentRegions(area Rect, callout string, calloutSize float64, hasPoints, withPictures bool) (text, pictures, calloutBox Rect) {
	if callout != "" {
		lines := float64(EstimateLines(callout, area[2]-24, calloutSize))
		height := math.Min(math.Round(lines*1.21*calloutSize+20), 90)
		calloutBox = Rect{area[0], area[1] + area[3] - height, area[2], height}
		area[3] -= height + PictureGap
	}
	if !withPictures {
		return area, area, calloutBox
	}
	if !hasPoints {
		return Rect{}, area, calloutBox
	}
	width := math.Round(area[2] * 0.42)
	text = Rect{area[0], area[1], width, area[3]}
	pictures = Rect{area[0] + width + 16, area[1], area[2] - width - 16, area[3]}
	return text, pictures, calloutBox
}

// CardBoxes: 2-3 the mot hang; 4 the luoi 2x2.
func CardBoxes(area Rect, count int) []Rect {
	boxes := []Rect{}
	columns, rows := count, 1
	if count == 4 {
		columns, rows = 2, 2
	}
	width := (area[2] - CardGap*float64(columns-1)) / float64(columns)
	height := (area[3] - CardGap*float64(rows-1)) / float64(rows)
	for i := 0; i < count; i++ {
		column, row := i%columns, i/columns
		boxes = append(boxes, Rect{Round1(area[0] + float64(column)*(width+CardGap)), Round1(area[1] + float64(row)*(height+CardGap)),
			Round1(width), Round1(height)})
	}
	return boxes
}

// FitCards: the cao vua noi dung (uoc theo co CardMaxFont, them le), khong cao het vung; ca cum can giua theo chieu doc.
func FitCards(area Rect, cards []Card) []Rect {
	boxes := CardBoxes(area, len(cards))
	if len(boxes) == 0 {
		return boxes
	}
	rows := 1
	if len(cards) == 4 {
		rows = 2
	}
	need := 0.0
	for c, card := range cards {
		lines := EstimateLines(card.Title, boxes[c][2]-20, CardMaxFont)
		for _, point := range card.Points {
			lines += EstimateLines(point, boxes[c][2]-40, CardMaxFont)
		}
		need = math.Max(need, float64(lines)*1.21*CardMaxFont+float64(len(card.Points))*0.35*CardMaxFont+28)
	}
	height := math.Min(boxes[0][3], math.Max(need, 120))
	total := height*float64(rows) + CardGap*float64(rows-1)
	top := area[1] + math.Max(0, (area[3]-total)/2)
	columns := max(1, len(boxes)/rows)
	for i := range boxes {
		boxes[i][1] = Round1(top + float64(i/columns)*(height+CardGap))
		boxes[i][3] = Round1(height)
	}
	return boxes
}

// PictureBoxes: xep anh theo HANG, anh cung hang cao bang nhau va lap day be ngang (giu ti le, cach nhau PictureGap);
// thu moi cach chia hang lien tiep theo thu tu doc, lay cach co TONG dien tich anh lon nhat; ca khoi can giua vung.
// Luot that 13/10/2026: chia o bang nhau thi anh khac ti le lech hang, chua khoang trang lon giua cac anh.
func PictureBoxes(area []float64, sizes [][2]float64) []Rect {
	count := len(sizes)
	if count == 0 || len(area) != 4 {
		return nil
	}
	const gap = PictureGap
	ratios := make([]float64, count)
	limits := make([]float64, count)
	for i, size := range sizes {
		width, height := size[0], size[1]
		limits[i] = math.Inf(1)
		if width <= 0 || height <= 0 {
			width, height = 4, 3
		} else {
			limits[i] = MaxUpscale * height
		}
		ratios[i] = width / height
	}
	var best []Rect
	bestArea := -1.0
	for mask := 0; mask < 1<<(count-1); mask++ {
		rows := [][]int{{0}}
		for i := 1; i < count; i++ {
			if mask&(1<<(i-1)) != 0 {
				rows = append(rows, []int{})
			}
			rows[len(rows)-1] = append(rows[len(rows)-1], i)
		}
		heights := make([]float64, len(rows))
		sum := 0.0
		for r, row := range rows {
			ratio, limit := 0.0, math.Inf(1)
			for _, i := range row {
				ratio += ratios[i]
				limit = math.Min(limit, limits[i])
			}
			heights[r] = math.Min((area[2]-gap*float64(len(row)-1))/ratio, limit)
			sum += heights[r]
		}
		free := area[3] - gap*float64(len(rows)-1)
		if free <= 0 {
			continue
		}
		if sum > free {
			for r := range heights {
				heights[r] *= free / sum
			}
			sum = free
		}
		boxes := make([]Rect, count)
		total := 0.0
		top := area[1] + (area[3]-sum-gap*float64(len(rows)-1))/2
		for r, row := range rows {
			width := gap * float64(len(row)-1)
			for _, i := range row {
				width += ratios[i] * heights[r]
			}
			left := area[0] + (area[2]-width)/2
			for _, i := range row {
				w := ratios[i] * heights[r]
				boxes[i] = Rect{Round1(left), Round1(top), Round1(w), Round1(heights[r])}
				total += w * heights[r]
				left += w + gap
			}
			top += heights[r] + gap
		}
		// Bang dien tich (sai so lam tron) thi giu cach it hang hon - da duyet truoc.
		if total > bestArea*1.001 {
			best, bestArea = boxes, total
		}
	}
	return best
}

// Tint: mau nhat cua mau nhan (tron voi trang) cho nen the / o luu y.
func Tint(hex string, share float64) string {
	value, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil || len(strings.TrimPrefix(hex, "#")) != 6 {
		return "#F2F2F2"
	}
	mix := func(channel uint64) uint64 { return uint64(math.Round(float64(channel)*(1-share) + 255*share)) }
	return fmt.Sprintf("#%02X%02X%02X", mix(value>>16&0xFF), mix(value>>8&0xFF), mix(value&0xFF))
}
