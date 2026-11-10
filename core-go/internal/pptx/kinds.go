package pptx

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // giai ma anh nen .jpg/.jpeg cua mau
	_ "image/png"  // giai ma anh nen .png cua mau
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"axiomoffice/core/internal/filesafe"
	"axiomoffice/core/internal/ooxml"
	"axiomoffice/core/internal/textutil"
)

// "Kieu slide" cua tep mau cho ppt_profile (docs/architecture/template-design.md muc 6.2). Thi nghiem 13/10/2026 tren mau cua
// truong: master/layout la Office mac dinh, nhan dien (logo, dai mau) nam trong ANH NEN phu kin tren tung slide,
// chu la textbox tu ve. Mau nhu vay dung bang cach nhan ban slide mau (wpp.addSlideFromTemplate) roi thay chu
// theo TEN shape (wpp.setShapeText) - nen o day ke ten tung o chu, suc chua, cac muc lap va vung an toan de
// model khong phai doan.

// maxKindSlides: tep nhieu slide hon thi la bai thuong, khong phai mau - bo phan kinds cho gon.
const maxKindSlides = 40

type rect struct{ X, Y, W, H float64 }

func (r rect) right() float64  { return r.X + r.W }
func (r rect) bottom() float64 { return r.Y + r.H }

func (r rect) contains(o rect, slack float64) bool {
	return o.X >= r.X-slack && o.Y >= r.Y-slack && o.right() <= r.right()+slack && o.bottom() <= r.bottom()+slack
}

func (r rect) overlapsX(o rect) bool { return r.X < o.right() && o.X < r.right() }

func (r rect) points() []float64 {
	return []float64{round1(r.X), round1(r.Y), round1(r.W), round1(r.H)}
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }

type shapeInfo struct {
	name   string
	kind   string // sp, pic, grpSp, graphicFrame, cxnSp
	box    rect
	hasBox bool
	text   string
	size   float64
	bold   bool
	color  string
	font   string
	ph     string
	idx    string
	embed  string
	geom   string
	// sizeSet: co chu doc duoc that (lan chu / kieu cua khung / ke thua) - khong thi size la mac dinh 18.
	sizeSet bool
}

func readShape(element *ooxml.Element) shapeInfo {
	info := shapeInfo{kind: element.Local}
	if cNvPr := firstDescendant(element, P, "cNvPr"); cNvPr != nil {
		info.name = cNvPr.AttrValue("", "name")
	}
	if ph := firstDescendant(element, P, "ph"); ph != nil {
		info.ph = ph.AttrValue("", "type")
		if info.ph == "" {
			info.ph = "body"
		}
		info.idx = ph.AttrValue("", "idx")
	}
	xfrm := firstDescendant(element, A, "xfrm")
	if element.Local == "graphicFrame" {
		xfrm = element.Child(P, "xfrm")
	}
	if xfrm != nil {
		off, ext := xfrm.Child(A, "off"), xfrm.Child(A, "ext")
		if off != nil && ext != nil {
			info.box = rect{emu(off.AttrValue("", "x")), emu(off.AttrValue("", "y")), emu(ext.AttrValue("", "cx")), emu(ext.AttrValue("", "cy"))}
			info.hasBox = true
		}
	}
	if geom := firstDescendant(element, A, "prstGeom"); geom != nil {
		info.geom = geom.AttrValue("", "prst")
	}
	if element.Local == "pic" {
		if blip := firstDescendant(element, A, "blip"); blip != nil {
			info.embed = blip.AttrValue(R, "embed")
		}
	}
	if element.Local == "sp" {
		body := element.Child(P, "txBody")
		info.text = strings.TrimSpace(textFrameText(body))
		info.size = 18
		if body != nil {
			if run := firstRunProps(body); run != nil {
				if sz, err := strconv.Atoi(run.AttrValue("", "sz")); err == nil && sz > 0 {
					info.size, info.sizeSet = float64(sz)/100, true
				}
				info.bold = run.AttrValue("", "b") == "1"
				if fill := run.Child(A, "solidFill"); fill != nil {
					info.color = colorValue(fill)
				}
				if latin := run.Child(A, "latin"); latin != nil && !strings.HasPrefix(latin.AttrValue("", "typeface"), "+") {
					info.font = latin.AttrValue("", "typeface")
				}
			}
		}
	}
	return info
}

// firstRunProps: dinh dang cua lan chu dau tien co ghi co chu (dinh dang ma setShapeText giu lai cho ca khung;
// lan chu chi co lang="vi-VN" thi bo qua - "XIN CAM ON" cua mau co rPr dau tien khong co sz).
func firstRunProps(body *ooxml.Element) *ooxml.Element {
	var fallback *ooxml.Element
	for _, paragraph := range body.Children(A, "p") {
		for _, run := range paragraph.Children(A, "r") {
			if props := run.Child(A, "rPr"); props != nil && props.HasAttr("", "sz") {
				return props
			}
		}
		if fallback == nil {
			if end := paragraph.Child(A, "endParaRPr"); end != nil && end.HasAttr("", "sz") {
				fallback = end
			}
		}
	}
	// Co chu dat o kieu cua chinh khung (lstStyle/lvl1pPr/defRPr) - "XIN CAM ON" cua mau: 36 pt.
	if style := body.Child(A, "lstStyle"); fallback == nil && style != nil {
		if level := firstDescendant(style, A, "defRPr"); level != nil && level.HasAttr("", "sz") {
			fallback = level
		}
	}
	return fallback
}

func emu(value string) float64 {
	number, _ := strconv.ParseFloat(value, 64)
	return number / emuPerPoint
}

// Kind: mot slide cua mau (JSON giu dung khoa ppt_profile da tra). Item cua o chu: so muc lap (1, 2...) ma o do
// thuoc ve; ItemBody = o noi dung chinh cua muc (o rong nhat), cac o khac trong muc (so La Ma...) giu nguyen chu mau.
type Kind struct {
	Kind       int        `json:"kind"`
	Use        string     `json:"use"`
	Fixed      string     `json:"fixed,omitempty"`
	Safe       []float64  `json:"safe"`
	PictureBox []float64  `json:"pictureBox,omitempty"`
	Items      [][]string `json:"items,omitempty"`
	Slots      []Slot     `json:"slots"`
}

type Slot struct {
	Shape    string    `json:"shape"`
	Role     string    `json:"role,omitempty"`
	Item     int       `json:"item,omitempty"`
	ItemBody bool      `json:"-"`
	Size     float64   `json:"size"`
	Bold     bool      `json:"bold,omitempty"`
	Color    string    `json:"color,omitempty"`
	Font     string    `json:"font,omitempty"`
	Text     string    `json:"text,omitempty"`
	Box      []float64 `json:"box,omitempty"`
	MaxLines int       `json:"maxLines,omitempty"`
	MaxChars int       `json:"maxChars,omitempty"`
}

// Title: o tieu de cua kieu (nil = khong co).
func (k Kind) Title() *Slot {
	for i := range k.Slots {
		if k.Slots[i].Role == "title" {
			return &k.Slots[i]
		}
	}
	return nil
}

// ItemBodies: o noi dung chinh cua tung muc lap, theo thu tu muc.
func (k Kind) ItemBodies() []Slot {
	bodies := []Slot{}
	for number := 1; number <= len(k.Items); number++ {
		for _, slot := range k.Slots {
			if slot.Item == number && slot.ItemBody {
				bodies = append(bodies, slot)
			}
		}
	}
	return bodies
}

// Template: kho slide + cac kieu cua mot tep mau.
type Template struct {
	Width, Height float64
	Kinds         []Kind
}

// ReadTemplate: doc kieu slide cua tep mau .pptx/.potx (dung chung voi ppt_profile).
func ReadTemplate(path string) (*Template, error) {
	if err := filesafe.RequireFile(path); err != nil {
		return nil, err
	}
	pkg, err := ooxml.OpenRead(path)
	if err != nil {
		return nil, err
	}
	presentation, err := PresentationPart(pkg)
	if err != nil {
		return nil, err
	}
	slideParts, err := SlideParts(pkg, presentation)
	if err != nil {
		return nil, err
	}
	doc, err := pkg.XML(presentation)
	if err != nil || doc.Root() == nil || doc.Root().Child(P, "sldSz") == nil {
		return nil, fmt.Errorf("'%s' has no slide size", path)
	}
	size := doc.Root().Child(P, "sldSz")
	width := emu(size.AttrValue("", "cx"))
	height := emu(size.AttrValue("", "cy"))
	return &Template{Width: width, Height: height, Kinds: slideKinds(pkg, slideParts, width, height)}, nil
}

// slideKinds: moi slide cua mau la mot kieu de nhan ban.
func slideKinds(pkg *ooxml.Package, slideParts []string, width, height float64) []Kind {
	kinds := []Kind{}
	slide := rect{0, 0, width, height}
	for index, slidePart := range slideParts {
		doc, err := pkg.XML(slidePart)
		if err != nil || doc.Root() == nil {
			continue
		}
		var spTree *ooxml.Element
		if commonSlide := doc.Root().Child(P, "cSld"); commonSlide != nil {
			spTree = commonSlide.Child(P, "spTree")
		}
		shapes := []shapeInfo{}
		for _, element := range shapeElements(spTree) {
			shapes = append(shapes, readShape(element))
		}
		inheritPlaceholders(pkg, slidePart, shapes)
		kind := describeKind(pkg, slidePart, shapes, slide)
		kind.Kind = index + 1
		kind.Use = kindUse(index, len(slideParts), kind, slide)
		kinds = append(kinds, kind)
	}
	return kinds
}

// kindUse: cong dung cua kieu - cover, list (muc lap I/II/III), closing, compare (hai o noi dung canh nhau), section
// (tieu de lon giua slide: "PHAN 1. ..."), agenda (tieu de "Muc tieu / Noi dung / Agenda"), con lai content.
func kindUse(index, count int, kind Kind, slide rect) string {
	switch {
	case index == 0:
		return "cover"
	case len(kind.Items) > 0:
		return "list"
	case index == count-1 && count >= 3:
		return "closing"
	}
	bodies := kind.Bodies()
	if len(bodies) == 2 && len(bodies[0].Box) == 4 && len(bodies[1].Box) == 4 {
		a, b := bodies[0].Box, bodies[1].Box
		if a[1] < b[1]+b[3] && b[1] < a[1]+a[3] && (a[0]+a[2] <= b[0]+4 || b[0]+b[2] <= a[0]+4) {
			return "compare"
		}
	}
	if title := kind.Title(); title != nil {
		if agendaTitle.MatchString(title.Text) {
			return "agenda"
		}
		if len(title.Box) == 4 && title.Box[1]+title.Box[3]/2 > slide.H*0.25 && title.Size >= 32 {
			return "section"
		}
	}
	return "content"
}

// agendaTitle: tieu de mau cua slide muc tieu / noi dung chinh.
var agendaTitle = regexp.MustCompile(`(?i)^\s*(m[uụ]c ti[eê]u|n[oộ]i dung ch[ií]nh|m[uụ]c l[uụ]c|agenda|objectives|outline)`)

// Bodies: o noi dung (placeholder than chu, khong phai tieu de, khong thuoc muc lap), trai sang phai.
func (k Kind) Bodies() []Slot {
	bodies := []Slot{}
	for _, slot := range k.Slots {
		if slot.Role == "body" {
			bodies = append(bodies, slot)
		}
	}
	sort.SliceStable(bodies, func(i, j int) bool {
		if len(bodies[i].Box) == 4 && len(bodies[j].Box) == 4 {
			return bodies[i].Box[0] < bodies[j].Box[0]
		}
		return false
	})
	return bodies
}

// describeKind: nen co dinh, o chu (kem suc chua), muc lap, vung an toan va o dat anh.
func describeKind(pkg *ooxml.Package, slidePart string, shapes []shapeInfo, slide rect) Kind {
	kind := Kind{}
	content := []shapeInfo{}
	var background *shapeInfo
	for i := range shapes {
		shape := shapes[i]
		if background == nil && shape.kind == "pic" && shape.hasBox && covers(shape.box, slide) {
			background = &shapes[i]
			continue
		}
		content = append(content, shape)
	}
	safe := rect{36, 36, slide.W - 72, slide.H - 72}
	if background != nil {
		kind.Fixed = background.name
		if area, ok := backgroundSafeArea(pkg, slidePart, *background, slide); ok {
			safe = area
		}
	}
	kind.Safe = safe.points()

	items := repeatedItems(content)
	itemOf := map[string]int{}
	bodyOf := map[int]string{}
	if len(items) >= 2 {
		for number, item := range items {
			names := []string{}
			widest := -1.0
			for _, shape := range item {
				names = append(names, shape.name)
				itemOf[shape.name] = number + 1
				if isTextSlot(shape) && shape.box.W > widest {
					widest, bodyOf[number+1] = shape.box.W, shape.name
				}
			}
			kind.Items = append(kind.Items, names)
		}
	}

	// Tieu de = o chu ngoai muc lap co chu lon nhat.
	title := ""
	best := 0.0
	for _, shape := range content {
		if isTextSlot(shape) && itemOf[shape.name] == 0 && shape.size > best {
			best, title = shape.size, shape.name
		}
	}
	pictureTop := safe.Y
	for _, shape := range content {
		if !isTextSlot(shape) {
			continue
		}
		slot := Slot{Shape: shape.name, Size: shape.size, Bold: shape.bold, Color: shape.color, Font: shape.font, Item: itemOf[shape.name]}
		if shape.name == title {
			slot.Role = "title"
		} else if shape.ph != "" && !isTitlePh(shape.ph) && slot.Item == 0 && shape.ph != "dt" && shape.ph != "ftr" && shape.ph != "sldNum" {
			slot.Role = "body"
		}
		slot.ItemBody = slot.Item > 0 && bodyOf[slot.Item] == shape.name
		if shape.text != "" {
			slot.Text = textutil.Truncate(strings.ReplaceAll(shape.text, "\n", " / "), 40)
		}
		if shape.hasBox {
			lines, perLine := capacity(shape, content, slide)
			slot.Box = shape.box.points()
			slot.MaxLines = lines
			slot.MaxChars = lines * perLine
			if shape.name == title {
				pictureTop = math.Max(pictureTop, shape.box.Y+float64(lines)*1.21*shape.size+7.2+8)
			}
		}
		kind.Slots = append(kind.Slots, slot)
	}
	if pictureTop < safe.bottom()-60 {
		kind.PictureBox = rect{safe.X, pictureTop, safe.W, safe.bottom() - pictureTop}.points()
	}
	// Mau co placeholder noi dung: vung noi dung la khung cua no (hop cac o neu hai cot) - tranh dai mau, logo cua
	// layout/master ma vung an toan tinh tu anh nen khong thay.
	if bodies := kind.Bodies(); len(bodies) > 0 && len(bodies[0].Box) == 4 {
		area := rect{bodies[0].Box[0], bodies[0].Box[1], bodies[0].Box[2], bodies[0].Box[3]}
		for _, body := range bodies[1:] {
			if len(body.Box) == 4 {
				right := math.Max(area.right(), body.Box[0]+body.Box[2])
				bottom := math.Max(area.bottom(), body.Box[1]+body.Box[3])
				area.X, area.Y = math.Min(area.X, body.Box[0]), math.Min(area.Y, body.Box[1])
				area.W, area.H = right-area.X, bottom-area.Y
			}
		}
		if area.H >= 60 {
			kind.PictureBox = area.points()
		}
	}
	return kind
}

func isTextSlot(shape shapeInfo) bool {
	if shape.kind != "sp" {
		return false
	}
	return shape.text != "" || shape.ph == "title" || shape.ph == "ctrTitle" || shape.ph == "body" || shape.ph == "subTitle"
}

// covers: anh chiem >= 90% dien tich slide (anh nen; mau co the dat lech vai pt ra ngoai mep).
func covers(box, slide rect) bool {
	w := math.Min(box.right(), slide.right()) - math.Max(box.X, slide.X)
	h := math.Min(box.bottom(), slide.bottom()) - math.Max(box.Y, slide.Y)
	return w > 0 && h > 0 && w*h >= 0.9*slide.W*slide.H
}

// capacity: so dong vua va ky tu moi dong (bang do cua skill trinh-bay-chuyen-nghiep: dong cao 1,21 x co, le
// tren+duoi 7,2; ky tu/dong = rong / (0,53 x co), chu dam rong hon ~5%). Day khung la mep tren cua shape gan
// nhat phia duoi co cung khoang ngang, hoac mep duoi cua shape bao quanh no (o chu nam trong o mau) - khung
// tu gian theo chu (spAutoFit) nen chieu cao luu trong tep khong phai suc chua.
func capacity(shape shapeInfo, others []shapeInfo, slide rect) (int, int) {
	limit := slide.bottom()
	for _, other := range others {
		if other.name == shape.name || !other.hasBox {
			continue
		}
		if other.box.contains(shape.box, 2) && !shape.box.contains(other.box, 0) {
			limit = math.Min(limit, other.box.bottom())
			continue
		}
		if other.box.Y >= shape.box.Y+shape.box.H/2 && other.box.overlapsX(shape.box) && !shape.box.contains(other.box, 0) {
			limit = math.Min(limit, other.box.Y)
		}
	}
	lines := max(int((limit-shape.box.Y-7.2)/(1.21*shape.size)), 1)
	width := math.Min(shape.box.right(), slide.right()) - math.Max(shape.box.X, slide.X)
	factor := 0.53
	if shape.bold {
		factor *= 1.05
	}
	perLine := max(int(width/(factor*shape.size)), 1)
	return lines, perLine
}

// repeatedItems: cac nhom shape lap lai (muc I/II/III: cung bo hinh, lech nhau theo truc y hoac x). Chi lay shape
// co kich thuoc xuat hien >= 2 lan, gom theo hang (hoac cot); hop le khi moi nhom co cung so shape.
func repeatedItems(shapes []shapeInfo) [][]shapeInfo {
	count := map[string]int{}
	for _, shape := range shapes {
		count[signature(shape)]++
	}
	candidates := []shapeInfo{}
	for _, shape := range shapes {
		if shape.hasBox && count[signature(shape)] >= 2 {
			candidates = append(candidates, shape)
		}
	}
	if len(candidates) < 2 {
		return nil
	}
	for _, vertical := range []bool{true, false} {
		groups := cluster(candidates, vertical)
		if len(groups) >= 2 && sameShape(groups) {
			return groups
		}
	}
	return nil
}

func signature(shape shapeInfo) string {
	return shape.kind + "/" + shape.geom + "/" + strconv.Itoa(int(math.Round(shape.box.W))) + "x" + strconv.Itoa(int(math.Round(shape.box.H)))
}

// cluster: gom shape chong nhau theo truc y (vertical = cac hang xep doc) hoac truc x.
func cluster(shapes []shapeInfo, vertical bool) [][]shapeInfo {
	start := func(s shapeInfo) float64 {
		if vertical {
			return s.box.Y
		}
		return s.box.X
	}
	end := func(s shapeInfo) float64 {
		if vertical {
			return s.box.bottom()
		}
		return s.box.right()
	}
	sorted := append([]shapeInfo(nil), shapes...)
	sort.SliceStable(sorted, func(i, j int) bool { return start(sorted[i]) < start(sorted[j]) })
	groups := [][]shapeInfo{}
	reach := math.Inf(-1)
	for _, shape := range sorted {
		if len(groups) == 0 || start(shape) >= reach {
			groups = append(groups, []shapeInfo{shape})
			reach = end(shape)
			continue
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], shape)
		reach = math.Max(reach, end(shape))
	}
	return groups
}

func sameShape(groups [][]shapeInfo) bool {
	want := groupSignature(groups[0])
	if len(groups[0]) < 2 {
		return false
	}
	for _, group := range groups[1:] {
		if groupSignature(group) != want {
			return false
		}
	}
	return true
}

func groupSignature(group []shapeInfo) string {
	parts := []string{}
	for _, shape := range group {
		parts = append(parts, signature(shape))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// backgroundSafeArea: hinh chu nhat TRANG lon nhat tren anh nen (tren luoi 96 x 54 o), doi ra pt va chua le
// 12 pt - phan con lai cua anh la logo/dai mau, chu va anh dat vao do se de len nhan dien.
func backgroundSafeArea(pkg *ooxml.Package, slidePart string, background shapeInfo, slide rect) (rect, bool) {
	if background.embed == "" {
		return rect{}, false
	}
	var target string
	for _, rel := range pkg.Rels(slidePart) {
		if rel.ID == background.embed && !rel.External {
			target = rel.TargetPart
		}
	}
	if target == "" {
		return rect{}, false
	}
	data, err := pkg.Bytes(target)
	if err != nil {
		return rect{}, false
	}
	picture, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return rect{}, false
	}
	const cols, rows = 96, 54
	white := whiteGrid(picture, cols, rows)
	c0, r0, c1, r1, ok := largestTrue(white)
	if !ok {
		return rect{}, false
	}
	box := background.box
	cellW, cellH := box.W/cols, box.H/rows
	area := rect{box.X + float64(c0)*cellW, box.Y + float64(r0)*cellH, float64(c1-c0) * cellW, float64(r1-r0) * cellH}
	// Cat theo slide, chua le.
	left := math.Max(area.X, slide.X) + 12
	top := math.Max(area.Y, slide.Y) + 12
	right := math.Min(area.right(), slide.right()) - 12
	bottom := math.Min(area.bottom(), slide.bottom()) - 12
	if right-left < 100 || bottom-top < 100 {
		return rect{}, false
	}
	return rect{left, top, right - left, bottom - top}, true
}

// whiteGrid: o la "trang" khi moi diem lay mau trong o deu gan trang (kenh nho nhat >= 0xE6).
func whiteGrid(picture image.Image, cols, rows int) [][]bool {
	bounds := picture.Bounds()
	grid := make([][]bool, rows)
	for r := range rows {
		grid[r] = make([]bool, cols)
		for c := range cols {
			x0 := bounds.Min.X + c*bounds.Dx()/cols
			x1 := bounds.Min.X + (c+1)*bounds.Dx()/cols
			y0 := bounds.Min.Y + r*bounds.Dy()/rows
			y1 := bounds.Min.Y + (r+1)*bounds.Dy()/rows
			isWhite := true
			for y := y0; y < y1 && isWhite; y += max(1, (y1-y0)/4) {
				for x := x0; x < x1; x += max(1, (x1-x0)/4) {
					red, green, blue, _ := picture.At(x, y).RGBA()
					if min(red, green, blue)>>8 < 0xE6 {
						isWhite = false
						break
					}
				}
			}
			grid[r][c] = isWhite
		}
	}
	return grid
}

// largestTrue: hinh chu nhat lon nhat toan o true (thuat toan histogram tung hang). Tra [c0, c1) x [r0, r1).
func largestTrue(grid [][]bool) (int, int, int, int, bool) {
	if len(grid) == 0 {
		return 0, 0, 0, 0, false
	}
	cols := len(grid[0])
	heights := make([]int, cols)
	bestArea := 0
	var c0, r0, c1, r1 int
	for r := range grid {
		for c := range cols {
			if grid[r][c] {
				heights[c]++
			} else {
				heights[c] = 0
			}
		}
		stack := []int{}
		for c := 0; c <= cols; c++ {
			h := 0
			if c < cols {
				h = heights[c]
			}
			for len(stack) > 0 && heights[stack[len(stack)-1]] >= h {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				left := 0
				if len(stack) > 0 {
					left = stack[len(stack)-1] + 1
				}
				if area := heights[top] * (c - left); area > bestArea {
					bestArea = area
					c0, c1 = left, c
					r0, r1 = r-heights[top]+1, r+1
				}
			}
			if c < cols {
				stack = append(stack, c)
			}
		}
	}
	return c0, r0, c1, r1, bestArea > 0
}
