package pptx

import (
	"strconv"

	"axiomoffice/core/internal/ooxml"
)

// Placeholder that ke thua vi tri va co chu tu layout roi master (mau truong Y Ha Noi 13/10/2026: Title 1, Content
// Placeholder 2... tren slide khong co xfrm; layout 1 cung khong co - phai len toi master). Khong doc phan nay thi moi
// o chu cua mau "khong co khung": vung noi dung bi tinh la ca slide, chu va anh de len dai mau ben trai va tieu de.

// inherited: vi tri / co chu cua mot placeholder o layout hoac master.
type inherited struct {
	box     rect
	hasBox  bool
	size    float64
	sizeSet bool
}

// phKey: khoa tim placeholder tuong ung - theo idx truoc, roi theo kieu (ctrTitle ~ title, subTitle/obj ~ body).
func phKeys(ph, idx string) []string {
	keys := []string{}
	if idx != "" {
		keys = append(keys, "idx:"+idx)
	}
	keys = append(keys, "type:"+ph)
	switch ph {
	case "ctrTitle":
		keys = append(keys, "type:title")
	case "subTitle", "obj":
		keys = append(keys, "type:body")
	}
	return keys
}

// placeholderMap: placeholder cua mot part (layout hoac master) theo idx va theo kieu.
func placeholderMap(pkg *ooxml.Package, part string) map[string]inherited {
	found := map[string]inherited{}
	if part == "" {
		return found
	}
	doc, err := pkg.XML(part)
	if err != nil || doc.Root() == nil {
		return found
	}
	var spTree *ooxml.Element
	if common := doc.Root().Child(P, "cSld"); common != nil {
		spTree = common.Child(P, "spTree")
	}
	for _, element := range shapeElements(spTree) {
		ph := firstDescendant(element, P, "ph")
		if ph == nil {
			continue
		}
		info := readShape(element)
		entry := inherited{box: info.box, hasBox: info.hasBox, size: info.size, sizeSet: info.sizeSet}
		if !entry.sizeSet {
			if body := element.Child(P, "txBody"); body != nil {
				if style := body.Child(A, "lstStyle"); style != nil {
					if level := firstDescendant(style, A, "defRPr"); level != nil && level.HasAttr("", "sz") {
						if sz, err := strconv.Atoi(level.AttrValue("", "sz")); err == nil && sz > 0 {
							entry.size, entry.sizeSet = float64(sz)/100, true
						}
					}
				}
			}
		}
		for _, key := range phKeys(info.ph, ph.AttrValue("", "idx")) {
			if _, taken := found[key]; !taken {
				found[key] = entry
			}
		}
	}
	return found
}

// masterTextSizes: co chu cap 1 cua tieu de / than chu dat o master (txStyles).
func masterTextSizes(pkg *ooxml.Package, master string) (float64, float64) {
	title, body := 0.0, 0.0
	doc, err := pkg.XML(master)
	if err != nil || doc.Root() == nil {
		return title, body
	}
	styles := doc.Root().Child(P, "txStyles")
	if styles == nil {
		return title, body
	}
	read := func(name string) float64 {
		if style := styles.Child(P, name); style != nil {
			if level := firstDescendant(style, A, "defRPr"); level != nil {
				if sz, err := strconv.Atoi(level.AttrValue("", "sz")); err == nil && sz > 0 {
					return float64(sz) / 100
				}
			}
		}
		return 0
	}
	return read("titleStyle"), read("bodyStyle")
}

// inheritPlaceholders: dien vi tri / co chu con thieu cua placeholder tren slide tu layout roi master.
func inheritPlaceholders(pkg *ooxml.Package, slidePart string, shapes []shapeInfo) {
	layout, master := "", ""
	if rel := pkg.RelOfType(slidePart, "slideLayout"); rel != nil {
		layout = rel.TargetPart
		if rel := pkg.RelOfType(layout, "slideMaster"); rel != nil {
			master = rel.TargetPart
		}
	}
	levels := []map[string]inherited{placeholderMap(pkg, layout), placeholderMap(pkg, master)}
	titleSize, bodySize := masterTextSizes(pkg, master)
	for i := range shapes {
		shape := &shapes[i]
		if shape.ph == "" {
			continue
		}
		for _, level := range levels {
			for _, key := range phKeys(shape.ph, shape.idx) {
				entry, ok := level[key]
				if !ok {
					continue
				}
				if !shape.hasBox && entry.hasBox {
					shape.box, shape.hasBox = entry.box, true
				}
				if !shape.sizeSet && entry.sizeSet {
					shape.size, shape.sizeSet = entry.size, true
				}
			}
		}
		if !shape.sizeSet {
			if isTitlePh(shape.ph) && titleSize > 0 {
				shape.size, shape.sizeSet = titleSize, true
			} else if !isTitlePh(shape.ph) && bodySize > 0 {
				shape.size, shape.sizeSet = bodySize, true
			}
		}
	}
}

func isTitlePh(ph string) bool { return ph == "title" || ph == "ctrTitle" }
