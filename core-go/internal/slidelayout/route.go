package slidelayout

import "math"

// Di day cho so do (docs/plans/18-s0): chon canh ra/vao cua moi duong noi sao cho duong KHONG cat qua hop khac va
// khong chong len duong da ve. Luot that 25/10/2026 (so do kien truc 10 slide): noi hai hop cung hang khong ke nhau
// bang duong thang xuyen qua hop o giua, duong quay ve tang tren chong len duong khac - checkLayout khong bat duoc.
// Hinh hoc o day la gan dung cua PowerPoint (bentConnector3); wpp.checkLayout do lai bang duong that.

const (
	SideTop    = "top"
	SideBottom = "bottom"
	SideLeft   = "left"
	SideRight  = "right"
	// LoopOffset: duong vong chu U di ra ngoai dau xa hon bao nhieu; moi duong vong them cung phia cua cung hang lui
	// them LoopStep de khong trung nhau.
	LoopOffset = 16.0
	LoopStep   = 10.0
)

// Point: toa do tren slide (pt).
type Point [2]float64

// Route: duong noi cua mot canh so do. Offset > 0 chi voi duong vong (FromSide == ToSide). Clear = khong cat hop nao va
// khong chong duong nao da xep.
type Route struct {
	FromSide, ToSide string
	Elbow            bool
	Offset           float64
	Clear            bool
	Path             []Point
}

func site(box Rect, side string) Point {
	switch side {
	case SideTop:
		return Point{box[0] + box[2]/2, box[1]}
	case SideBottom:
		return Point{box[0] + box[2]/2, box[1] + box[3]}
	case SideLeft:
		return Point{box[0], box[1] + box[3]/2}
	default:
		return Point{box[0] + box[2], box[1] + box[3]/2}
	}
}

func vertical(side string) bool { return side == SideTop || side == SideBottom }

// ConnectorPath: duong di gan dung cua duong noi tu canh fromSide cua `from` toi canh toSide cua `to`. Thang: mot doan.
// Gap khuc canh doi dien: ra, re o giua, vao. Cung mot canh (duong vong): ra xa hon dau xa hon `offset`, chay doc, vao.
func ConnectorPath(from, to Rect, fromSide, toSide string, elbow bool, offset float64) []Point {
	begin, end := site(from, fromSide), site(to, toSide)
	if !elbow {
		return []Point{begin, end}
	}
	if fromSide == toSide {
		if offset <= 0 {
			offset = LoopOffset
		}
		switch fromSide {
		case SideBottom:
			y := math.Max(begin[1], end[1]) + offset
			return []Point{begin, {begin[0], y}, {end[0], y}, end}
		case SideTop:
			y := math.Min(begin[1], end[1]) - offset
			return []Point{begin, {begin[0], y}, {end[0], y}, end}
		case SideRight:
			x := math.Max(begin[0], end[0]) + offset
			return []Point{begin, {x, begin[1]}, {x, end[1]}, end}
		default:
			x := math.Min(begin[0], end[0]) - offset
			return []Point{begin, {x, begin[1]}, {x, end[1]}, end}
		}
	}
	if vertical(fromSide) {
		middle := (begin[1] + end[1]) / 2
		return []Point{begin, {begin[0], middle}, {end[0], middle}, end}
	}
	middle := (begin[0] + end[0]) / 2
	return []Point{begin, {middle, begin[1]}, {middle, end[1]}, end}
}

// PathCrosses: duong co di qua phan trong cua hop khong (sat mep 2 pt khong tinh).
func PathCrosses(path []Point, box Rect) bool {
	const inset = 2.0
	left, top, right, bottom := box[0]+inset, box[1]+inset, box[0]+box[2]-inset, box[1]+box[3]-inset
	for i := 1; i < len(path); i++ {
		if segmentHitsRect(path[i-1], path[i], left, top, right, bottom) {
			return true
		}
	}
	return false
}

// segmentHitsRect: Liang-Barsky (cung cach voi wpp.checkLayout ben C#).
func segmentHitsRect(a, b Point, left, top, right, bottom float64) bool {
	dx, dy := b[0]-a[0], b[1]-a[1]
	enter, leave := 0.0, 1.0
	p := []float64{-dx, dx, -dy, dy}
	q := []float64{a[0] - left, right - a[0], a[1] - top, bottom - a[1]}
	for i := range p {
		if math.Abs(p[i]) < 1e-9 {
			if q[i] <= 0 {
				return false
			}
			continue
		}
		t := q[i] / p[i]
		if p[i] < 0 {
			enter = math.Max(enter, t)
		} else {
			leave = math.Min(leave, t)
		}
		if enter >= leave {
			return false
		}
	}
	return leave-enter > 1e-6
}

// PathsOverlap: hai duong co doan ngang (hoac doc) trung nhau tren mot quang dai hon 8 pt.
func PathsOverlap(first, second []Point) bool {
	for i := 1; i < len(first); i++ {
		for j := 1; j < len(second); j++ {
			if segmentsOverlap(first[i-1], first[i], second[j-1], second[j]) {
				return true
			}
		}
	}
	return false
}

func segmentsOverlap(a1, a2, b1, b2 Point) bool {
	run := func(a1, a2, b1, b2 float64) float64 {
		return math.Min(math.Max(a1, a2), math.Max(b1, b2)) - math.Max(math.Min(a1, a2), math.Min(b1, b2))
	}
	const parallel, minRun = 2.0, 8.0
	horizontal := func(a, b Point) bool { return math.Abs(a[1]-b[1]) < 0.5 }
	upright := func(a, b Point) bool { return math.Abs(a[0]-b[0]) < 0.5 }
	if horizontal(a1, a2) && horizontal(b1, b2) && math.Abs(a1[1]-b1[1]) < parallel {
		return run(a1[0], a2[0], b1[0], b2[0]) > minRun
	}
	if upright(a1, a2) && upright(b1, b2) && math.Abs(a1[0]-b1[0]) < parallel {
		return run(a1[1], a2[1], b1[1], b2[1]) > minRun
	}
	return false
}

// Edge: canh so do theo vi tri (tang, thu tu trong tang) cua hai dau.
type Edge struct{ FromLayer, FromPlace, ToLayer, ToPlace int }

// Placed: duong da xep, kem hai dau (duong cung xuat phat hoac cung dich duoc dung chung doan than - kieu so do to chuc).
type Placed struct {
	From, To [2]int
	Path     []Point
}

// RouteDiagram: chon duong cho tung canh theo thu tu. Ung vien (dau tien khong cat hop / khong chong duong = chon):
// tang ke duoi/tren -> day xuong dinh (gap khuc, re o giua khe); cung tang ke nhau -> thang ngang; cung tang khong ke
// nhau -> vong duoi hang, roi vong tren; vuot tang hoac quay nguoc -> vong ben phai/trai (phia gan hon truoc), roi
// duong doc. Khong ung vien nao sach thi lay ung vien dau, Clear = false (checkLayout se bao).
func RouteDiagram(rows [][]Rect, edges []Edge) []Route {
	routes := []Route{}
	placed := []Placed{}
	loops := map[[2]int]int{} // (tang, phia) -> so duong vong da xep
	for _, edge := range edges {
		from, to := rows[edge.FromLayer][edge.FromPlace], rows[edge.ToLayer][edge.ToPlace]
		candidates := diagramCandidates(rows, edge, from, to)
		chosen := -1
		for c := range candidates {
			candidate := &candidates[c]
			if candidate.FromSide == candidate.ToSide && candidate.Elbow {
				candidate.Offset = LoopOffset + LoopStep*float64(loops[loopKey(edge, candidate.FromSide)]) +
					clearance(rows, edge, from, to, candidate.FromSide)
			}
			candidate.Path = ConnectorPath(from, to, candidate.FromSide, candidate.ToSide, candidate.Elbow, candidate.Offset)
			if chosen < 0 && routeClear(rows, edge, candidate.Path, placed) {
				chosen = c
			}
		}
		if chosen < 0 {
			chosen = 0
		} else {
			candidates[chosen].Clear = true
		}
		route := candidates[chosen]
		if route.FromSide == route.ToSide && route.Elbow {
			loops[loopKey(edge, route.FromSide)]++
		}
		placed = append(placed, Placed{From: [2]int{edge.FromLayer, edge.FromPlace}, To: [2]int{edge.ToLayer, edge.ToPlace},
			Path: route.Path})
		routes = append(routes, route)
	}
	return routes
}

// loopKey: duong vong cung phia cua cung mot tang (vong duoi/tren) hoac cung mot phia slide (vong trai/phai) xep lan.
func loopKey(edge Edge, side string) [2]int {
	sideIndex := map[string]int{SideTop: 0, SideBottom: 1, SideLeft: 2, SideRight: 3}[side]
	if vertical(side) {
		return [2]int{edge.FromLayer, sideIndex}
	}
	return [2]int{-1, sideIndex}
}

func diagramCandidates(rows [][]Rect, edge Edge, from, to Rect) []Route {
	gap := edge.ToLayer - edge.FromLayer
	step := edge.ToPlace - edge.FromPlace
	switch {
	case gap == 1:
		return []Route{{FromSide: SideBottom, ToSide: SideTop, Elbow: true}, {FromSide: SideRight, ToSide: SideRight, Elbow: true},
			{FromSide: SideLeft, ToSide: SideLeft, Elbow: true}}
	case gap == -1:
		return []Route{{FromSide: SideTop, ToSide: SideBottom, Elbow: true}, {FromSide: SideRight, ToSide: SideRight, Elbow: true},
			{FromSide: SideLeft, ToSide: SideLeft, Elbow: true}}
	case gap == 0 && step == 1:
		return []Route{{FromSide: SideRight, ToSide: SideLeft}, {FromSide: SideBottom, ToSide: SideBottom, Elbow: true}}
	case gap == 0 && step == -1:
		return []Route{{FromSide: SideLeft, ToSide: SideRight}, {FromSide: SideBottom, ToSide: SideBottom, Elbow: true}}
	case gap == 0:
		return []Route{{FromSide: SideBottom, ToSide: SideBottom, Elbow: true}, {FromSide: SideTop, ToSide: SideTop, Elbow: true}}
	}
	// Vuot tang hoac quay nguoc: vong ben hong, phia gan hai dau hon truoc.
	center := 0.0
	count := 0.0
	for _, row := range rows {
		for _, box := range row {
			center += box[0] + box[2]/2
			count++
		}
	}
	center /= math.Max(count, 1)
	near, far := SideRight, SideLeft
	if (from[0]+from[2]/2)+(to[0]+to[2]/2) < 2*center {
		near, far = SideLeft, SideRight
	}
	vertical := Route{FromSide: SideBottom, ToSide: SideTop, Elbow: true}
	if gap < 0 {
		vertical = Route{FromSide: SideTop, ToSide: SideBottom, Elbow: true}
	}
	return []Route{{FromSide: near, ToSide: near, Elbow: true}, {FromSide: far, ToSide: far, Elbow: true}, vertical}
}

func routeClear(rows [][]Rect, edge Edge, path []Point, placed []Placed) bool {
	for l, row := range rows {
		for p, box := range row {
			if (l == edge.FromLayer && p == edge.FromPlace) || (l == edge.ToLayer && p == edge.ToPlace) {
				continue
			}
			if PathCrosses(path, box) {
				return false
			}
		}
	}
	from, to := [2]int{edge.FromLayer, edge.FromPlace}, [2]int{edge.ToLayer, edge.ToPlace}
	for _, other := range placed {
		if other.From == from || other.To == to {
			continue
		}
		if PathsOverlap(path, other.Path) {
			return false
		}
	}
	return true
}

// clearance: duong vong ben hong phai ra ngoai hop xa nhat cua cac tang no di qua, khong chi ra ngoai hai dau.
func clearance(rows [][]Rect, edge Edge, from, to Rect, side string) float64 {
	if vertical(side) {
		return 0
	}
	low, high := min(edge.FromLayer, edge.ToLayer), max(edge.FromLayer, edge.ToLayer)
	begin, end := site(from, side), site(to, side)
	if side == SideRight {
		limit := math.Max(begin[0], end[0])
		extent := limit
		for l := low; l <= high; l++ {
			for _, box := range rows[l] {
				extent = math.Max(extent, box[0]+box[2])
			}
		}
		return extent - limit
	}
	limit := math.Min(begin[0], end[0])
	extent := limit
	for l := low; l <= high; l++ {
		for _, box := range rows[l] {
			extent = math.Min(extent, box[0])
		}
	}
	return limit - extent
}
