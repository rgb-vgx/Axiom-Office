package pptx

import (
	"fmt"
	"sort"
	"strings"

	"axiomoffice/core/internal/ooxml"
)

// Phong cach cua mot tep .pptx cho ppt_profile: kho slide de doc, bang mau + font cua theme, va mau/font THUC
// SU dung tren slide. Luot 13/10/2026: nguoi dung dinh kem mau de "dung template", model chi doc duoc chu nen tu
// chon mau xanh trong khi mau co tong do do, va khong nhan ra mau 16:9 con bai dang mo 4:3.

const emuPerPoint = 12700

// slideSize: kich thuoc theo diem (pt) + ti le de doc ("16:9", "4:3", ...).
func slideSize(width, height int64) map[string]any {
	if width <= 0 || height <= 0 {
		return nil
	}
	return map[string]any{
		"widthPt":  float64(width) / emuPerPoint,
		"heightPt": float64(height) / emuPerPoint,
		"aspect":   aspect(width, height),
	}
}

func aspect(width, height int64) string {
	ratio := float64(width) / float64(height)
	for _, known := range []struct {
		name  string
		value float64
	}{{"16:9", 16.0 / 9}, {"16:10", 16.0 / 10}, {"4:3", 4.0 / 3}, {"A4", 297.0 / 210}} {
		if ratio > known.value*0.985 && ratio < known.value*1.015 {
			return known.name
		}
	}
	return fmt.Sprintf("%.2f:1", ratio)
}

// themeStyle: bang mau (dk1..accent6) va font tieu de/noi dung cua theme ma slide master dung.
func themeStyle(pkg *ooxml.Package, presentation string) map[string]any {
	master := pkg.RelOfType(presentation, "slideMaster")
	if master == nil || !pkg.Exists(master.TargetPart) {
		return nil
	}
	theme := pkg.RelOfType(master.TargetPart, "theme")
	if theme == nil || !pkg.Exists(theme.TargetPart) {
		return nil
	}
	doc, err := pkg.XML(theme.TargetPart)
	if err != nil || doc.Root() == nil {
		return nil
	}
	elements := doc.Root().Child(A, "themeElements")
	if elements == nil {
		return nil
	}
	style := map[string]any{}
	if scheme := elements.Child(A, "clrScheme"); scheme != nil {
		colors := map[string]any{}
		for _, slot := range scheme.Elements() {
			if value := colorValue(slot); value != "" {
				colors[slot.Local] = value
			}
		}
		if len(colors) > 0 {
			style["colors"] = colors
		}
	}
	if scheme := elements.Child(A, "fontScheme"); scheme != nil {
		fonts := map[string]any{}
		for _, kind := range []string{"majorFont", "minorFont"} {
			if font := scheme.Child(A, kind); font != nil {
				if latin := font.Child(A, "latin"); latin != nil && latin.AttrValue("", "typeface") != "" {
					fonts[strings.TrimSuffix(kind, "Font")] = latin.AttrValue("", "typeface")
				}
			}
		}
		if len(fonts) > 0 {
			style["fonts"] = fonts
		}
	}
	return style
}

// colorValue: <a:dk1><a:srgbClr val="1F4E79"/></a:dk1> hoac <a:sysClr lastClr="000000"/>.
func colorValue(slot *ooxml.Element) string {
	if rgb := slot.Child(A, "srgbClr"); rgb != nil {
		return "#" + strings.ToUpper(rgb.AttrValue("", "val"))
	}
	if system := slot.Child(A, "sysClr"); system != nil && system.AttrValue("", "lastClr") != "" {
		return "#" + strings.ToUpper(system.AttrValue("", "lastClr"))
	}
	return ""
}

// usage dem mau RGB va font goi thang tren slide, layout va master (mau thuong to thang len khung, khong qua theme).
type usage struct {
	colors map[string]int
	fonts  map[string]int
}

func newUsage() *usage { return &usage{colors: map[string]int{}, fonts: map[string]int{}} }

func (u *usage) scan(element *ooxml.Element) {
	if element == nil {
		return
	}
	if element.URI == A {
		switch element.Local {
		case "srgbClr":
			if value := element.AttrValue("", "val"); value != "" {
				u.colors["#"+strings.ToUpper(value)]++
			}
		case "latin", "ea", "cs":
			if face := element.AttrValue("", "typeface"); face != "" && !strings.HasPrefix(face, "+") {
				u.fonts[face]++
			}
		}
	}
	for _, child := range element.Elements() {
		u.scan(child)
	}
}

func (u *usage) scanPart(pkg *ooxml.Package, part string) {
	if part == "" || !pkg.Exists(part) {
		return
	}
	if doc, err := pkg.XML(part); err == nil && doc.Root() != nil {
		u.scan(doc.Root())
	}
}

// top: toi da n gia tri dung nhieu nhat, nhieu truoc (bang nhau thi theo ten cho on dinh).
func top(counts map[string]int, n int) []string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	if len(names) > n {
		names = names[:n]
	}
	return names
}
