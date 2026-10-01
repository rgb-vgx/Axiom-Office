package xlsx

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"axiomoffice/core/internal/ooxml"
	"axiomoffice/core/internal/sheet"
)

// Thu tu phan tu con theo schema (chen phan tu moi dung cho).
var (
	styleSheetOrder = []string{"numFmts", "fonts", "fills", "borders", "cellStyleXfs", "cellXfs", "cellStyles", "dxfs", "tableStyles", "colors", "extLst"}
	fontOrder       = []string{"b", "i", "strike", "condense", "extend", "outline", "shadow", "u", "vertAlign", "sz", "color", "name", "family", "charset", "scheme"}
	borderOrder     = []string{"start", "left", "end", "right", "top", "bottom", "diagonal", "vertical", "horizontal"}
	xfOrder         = []string{"alignment", "protection", "extLst"}
)

// BuiltinFormats la ma dinh dang dung san cua Excel.
var BuiltinFormats = map[string]int{
	"General": 0, "0": 1, "0.00": 2, "#,##0": 3, "#,##0.00": 4, "0%": 9, "0.00%": 10, "0.00E+00": 11,
	"# ?/?": 12, "# ??/??": 13, "mm-dd-yy": 14, "d-mmm-yy": 15, "d-mmm": 16, "mmm-yy": 17, "h:mm AM/PM": 18,
	"h:mm:ss AM/PM": 19, "h:mm": 20, "h:mm:ss": 21, "m/d/yy h:mm": 22, "#,##0 ;(#,##0)": 37, "#,##0 ;[Red](#,##0)": 38,
	"#,##0.00;(#,##0.00)": 39, "#,##0.00;[Red](#,##0.00)": 40, "mm:ss": 45, "[h]:mm:ss": 46, "mmss.0": 47, "##0.0E+0": 48, "@": 49,
}

// Gia tri "" nghia la khong co style (C# dung null).
var borderStyles = map[string]string{
	"thin": "thin", "continuous": "thin", "medium": "medium", "thick": "thick", "dash": "dashed", "dashed": "dashed",
	"dot": "dotted", "dotted": "dotted", "double": "double", "hair": "hair", "mediumdash": "mediumDashed",
	"mediumdashed": "mediumDashed", "dashdot": "dashDot", "mediumdashdot": "mediumDashDot", "dashdotdot": "dashDotDot",
	"mediumdashdotdot": "mediumDashDotDot", "slantdashdot": "slantDashDot", "none": "",
}

var underlines = map[string]string{
	"none": "", "single": "single", "double": "double", "singleaccounting": "singleAccounting", "doubleaccounting": "doubleAccounting",
}

// Styles sinh xf moi tu xf hien co cua o cong voi spec (font/fill/border/alignment/numFmt).
type Styles struct {
	pkg   *ooxml.Package
	part  string
	doc   *ooxml.Document
	S     string
	cache map[string]int
}

func NewStyles(pkg *ooxml.Package, part, namespace string) (*Styles, error) {
	doc, err := pkg.XML(part)
	if err != nil {
		return nil, err
	}
	return &Styles{pkg: pkg, part: part, doc: doc, S: namespace, cache: map[string]int{}}, nil
}

func (s *Styles) section(name string) *ooxml.Element {
	found := s.doc.Root().Child(s.S, name)
	if found == nil {
		found = ooxml.NewElement(s.S, "", name)
		InsertInOrder(s.doc.Root(), found, styleSheetOrder)
	}
	return found
}

// appendItem them phan tu va cap nhat "count"; dem theo namespace cua chinh phan tu
// (giong section.Elements(item.Name).Count() cua ban C#).
func appendItem(section, item *ooxml.Element) int {
	section.Append(item)
	count := len(section.Children(item.URI, item.Local))
	section.SetAttr("", "", "count", strconv.Itoa(count))
	return count - 1
}

// Derive tao (hoac lay lai tu cache) mot chi so style moi dua tren xf co san.
func (s *Styles) Derive(baseXf int, spec map[string]any) (int, error) {
	key, err := styleKey(baseXf, spec)
	if err != nil {
		return 0, err
	}
	if cached, ok := s.cache[key]; ok {
		return cached, nil
	}
	cellXfs := s.section("cellXfs")
	xfs := cellXfs.Children(s.S, "xf")
	if len(xfs) == 0 {
		font := child(s.section("fonts"), s.S, "font")
		child(font, s.S, "sz", "val", "11")
		child(font, s.S, "name", "val", "Calibri")
		child(child(s.section("fills"), s.S, "fill"), s.S, "patternFill", "patternType", "none")
		border := child(s.section("borders"), s.S, "border")
		for _, side := range []string{"left", "right", "top", "bottom", "diagonal"} {
			child(border, s.S, side)
		}
		appendItem(cellXfs, elementIn(s.S, "xf",
			"numFmtId", "0", "fontId", "0", "fillId", "0", "borderId", "0", "xfId", "0"))
		xfs = cellXfs.Children(s.S, "xf")
	}
	index := 0
	if baseXf >= 0 && baseXf < len(xfs) {
		index = baseXf
	}
	xf := xfs[index].Clone()

	if font, ok := spec["font"].(map[string]any); ok {
		current := s.itemAt("fonts", "font", xfAttrInt(xf, "fontId"))
		if current == nil {
			current = ooxml.NewElement(s.S, "", "font")
		}
		built, err := s.buildFont(current, font)
		if err != nil {
			return 0, err
		}
		xf.SetAttr("", "", "fontId", strconv.Itoa(appendItem(s.section("fonts"), built)))
		xf.SetAttr("", "", "applyFont", "1")
	}

	if fill, ok := spec["fill"].(map[string]any); ok {
		pattern := "solid"
		if value, exists := fill["pattern"]; exists && value != nil {
			pattern = sheet.ToText(value)
		}
		color, exists := fill["color"]
		if !exists {
			color = fill["fgColor"]
		}
		rgb, err := Rgb(color)
		if err != nil {
			return 0, err
		}
		patternFill := elementIn(s.S, "patternFill", "patternType", pattern)
		child(patternFill, s.S, "fgColor", "rgb", "FF"+rgb)
		child(patternFill, s.S, "bgColor", "indexed", "64")
		wrap := ooxml.NewElement(s.S, "", "fill")
		wrap.Append(patternFill)
		xf.SetAttr("", "", "fillId", strconv.Itoa(appendItem(s.section("fills"), wrap)))
		xf.SetAttr("", "", "applyFill", "1")
	}

	if border, ok := spec["border"].([]any); ok {
		current := s.itemAt("borders", "border", xfAttrInt(xf, "borderId"))
		if current == nil {
			current = ooxml.NewElement(s.S, "", "border")
		}
		built, err := s.buildBorder(current, border)
		if err != nil {
			return 0, err
		}
		xf.SetAttr("", "", "borderId", strconv.Itoa(appendItem(s.section("borders"), built)))
		xf.SetAttr("", "", "applyBorder", "1")
	}

	if alignment, ok := spec["alignment"].(map[string]any); ok {
		align := xf.Child(s.S, "alignment")
		if align == nil {
			align = ooxml.NewElement(s.S, "", "alignment")
			InsertInOrder(xf, align, xfOrder)
		}
		if value, exists := alignment["horizontal"]; exists {
			setOrRemove(align, "horizontal", nullableText(value))
		}
		if value, exists := alignment["vertical"]; exists {
			setOrRemove(align, "vertical", nullableText(value))
		}
		if value, exists := alignment["wrap"]; exists {
			if truthy(value) {
				align.SetAttr("", "", "wrapText", "1")
			} else {
				align.RemoveAttr("", "wrapText")
			}
		}
		if value, exists := alignment["rotation"]; exists && value != nil {
			align.SetAttr("", "", "textRotation", strconv.Itoa(asInt(value)))
		}
		xf.SetAttr("", "", "applyAlignment", "1")
	}

	numFmt := ""
	if value, exists := spec["numFmt"]; exists && value != nil {
		numFmt = sheet.ToText(value)
	}
	if numFmt == "" {
		if value, exists := spec["decimalPlaces"]; exists && value != nil {
			places := max(0, min(30, asInt(value)))
			if places == 0 {
				numFmt = "0"
			} else {
				numFmt = "0." + strings.Repeat("0", places)
			}
		}
	}
	if numFmt != "" {
		id, err := s.numFmtID(numFmt)
		if err != nil {
			return 0, err
		}
		xf.SetAttr("", "", "numFmtId", strconv.Itoa(id))
		xf.SetAttr("", "", "applyNumberFormat", "1")
	}

	created := appendItem(cellXfs, xf)
	s.pkg.Put(s.part, s.doc)
	s.cache[key] = created
	return created, nil
}

func (s *Styles) itemAt(section, item string, index int) *ooxml.Element {
	container := s.doc.Root().Child(s.S, section)
	if container == nil {
		return nil
	}
	return container.ElementAt(index)
}

func xfAttrInt(element *ooxml.Element, name string) int {
	value, err := strconv.Atoi(element.AttrValue("", name))
	if err != nil {
		return 0
	}
	return value
}

func (s *Styles) buildFont(current *ooxml.Element, spec map[string]any) (*ooxml.Element, error) {
	parts := map[string]*ooxml.Element{}
	for _, existing := range current.Elements() {
		parts[existing.Local] = existing.Clone()
	}
	flag := func(key, element string) {
		value, exists := spec[key]
		if !exists {
			return
		}
		if truthy(value) {
			parts[element] = ooxml.NewElement(s.S, "", element)
		} else {
			delete(parts, element)
		}
	}
	flag("bold", "b")
	flag("italic", "i")
	flag("strike", "strike")

	if value, exists := spec["size"]; exists && value != nil {
		number, _ := sheet.AsFloat(value)
		sized := ooxml.NewElement(s.S, "", "sz")
		sized.SetAttr("", "", "val", sheet.ToText(number))
		parts["sz"] = sized
	}
	if value, exists := spec["name"]; exists && value != nil && sheet.ToText(value) != "" {
		named := ooxml.NewElement(s.S, "", "name")
		named.SetAttr("", "", "val", sheet.ToText(value))
		parts["name"] = named
		// Font theo theme se de ten font, nen bo scheme di.
		delete(parts, "scheme")
	}
	if value, exists := spec["color"]; exists && value != nil {
		rgb, err := Rgb(value)
		if err != nil {
			return nil, err
		}
		colored := ooxml.NewElement(s.S, "", "color")
		colored.SetAttr("", "", "rgb", "FF"+rgb)
		parts["color"] = colored
	}
	if value, exists := spec["underline"]; exists {
		kind := ""
		switch typed := value.(type) {
		case bool:
			if typed {
				kind = "single"
			}
		case nil:
		default:
			text := sheet.ToText(value)
			resolved, known := underlines[strings.ToLower(text)]
			if known {
				kind = resolved
			} else {
				kind = text
			}
		}
		if kind == "" {
			delete(parts, "u")
		} else if kind == "single" {
			parts["u"] = ooxml.NewElement(s.S, "", "u")
		} else {
			underline := ooxml.NewElement(s.S, "", "u")
			underline.SetAttr("", "", "val", kind)
			parts["u"] = underline
		}
	}
	if value, exists := spec["vertAlign"]; exists {
		text := ""
		if value != nil {
			text = strings.ToLower(sheet.ToText(value))
		}
		if text == "superscript" || text == "subscript" {
			align := ooxml.NewElement(s.S, "", "vertAlign")
			align.SetAttr("", "", "val", text)
			parts["vertAlign"] = align
		} else {
			delete(parts, "vertAlign")
		}
	}

	font := ooxml.NewElement(s.S, "", "font")
	for _, name := range fontOrder {
		if part, ok := parts[name]; ok {
			font.Append(part)
		}
	}
	return font, nil
}

func (s *Styles) buildBorder(current *ooxml.Element, items []any) (*ooxml.Element, error) {
	border := current.Clone()
	for _, item := range items {
		side, ok := item.(map[string]any)
		if !ok {
			continue
		}
		edge := ""
		if value, exists := side["type"]; exists {
			edge = strings.ToLower(sheet.ToText(value))
		}
		styleName := "thin"
		if value, exists := side["style"]; exists && value != nil {
			styleName = sheet.ToText(value)
		}
		style, known := borderStyles[strings.ToLower(styleName)]
		if !known {
			style = "thin"
		}
		colorValue, exists := side["color"]
		if !exists || colorValue == nil {
			colorValue = "#000000"
		}
		color, err := Rgb(colorValue)
		if err != nil {
			return nil, err
		}
		element := ""
		switch edge {
		case "left", "right", "top", "bottom", "diagonal":
			element = edge
		case "diagonalup":
			element = "diagonal"
			border.SetAttr("", "", "diagonalUp", "1")
		case "diagonaldown":
			element = "diagonal"
			border.SetAttr("", "", "diagonalDown", "1")
		default:
			continue
		}
		sideElement := ooxml.NewElement(s.S, "", element)
		if style != "" {
			sideElement.SetAttr("", "", "style", style)
			colored := ooxml.NewElement(s.S, "", "color")
			colored.SetAttr("", "", "rgb", "FF"+color)
			sideElement.Append(colored)
		}
		border.RemoveChildrenLocal(element)
		// Ban C# con xoa ca "start"/"end" tuong ung (left/right ban moi cua schema).
		switch element {
		case "left":
			border.RemoveChildrenLocal("start")
		case "right":
			border.RemoveChildrenLocal("end")
		}
		InsertInOrder(border, sideElement, borderOrder)
	}
	return border, nil
}

func (s *Styles) numFmtID(code string) (int, error) {
	if builtin, ok := BuiltinFormats[code]; ok {
		return builtin, nil
	}
	numFmts := s.section("numFmts")
	for _, existing := range numFmts.Children(s.S, "numFmt") {
		if existing.AttrValue("", "formatCode") == code {
			return xfAttrInt(existing, "numFmtId"), nil
		}
	}
	highest := 163
	for _, existing := range numFmts.Children(s.S, "numFmt") {
		highest = max(highest, xfAttrInt(existing, "numFmtId"))
	}
	id := max(163, highest) + 1
	child(numFmts, s.S, "numFmt", "numFmtId", strconv.Itoa(id), "formatCode", code)
	return id, nil
}

// Rgb chuan hoa mau ve #RRGGBB (tra ve 6 ky tu hex hoa).
func Rgb(value any) (string, error) {
	if array, ok := value.([]any); ok {
		if len(array) > 0 {
			value = array[0]
		} else {
			value = nil
		}
	}
	text := ""
	if value != nil {
		text = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(sheet.ToText(value)), "#"))
	}
	if len(text) == 6 && isHex(text) {
		return text, nil
	}
	if len(text) == 8 && isHex(text) {
		return text[2:], nil
	}
	return "", fmt.Errorf("invalid color '%s' (expected #RRGGBB)", text)
}

func isHex(text string) bool {
	for _, character := range text {
		if !((character >= '0' && character <= '9') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func truthy(value any) bool {
	if value == nil {
		return false
	}
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	text := strings.ToLower(sheet.ToText(value))
	return text == "true" || text == "1"
}

func nullableText(value any) string {
	if value == nil {
		return ""
	}
	return sheet.ToText(value)
}

func setOrRemove(element *ooxml.Element, name, value string) {
	if value == "" {
		element.RemoveAttr("", name)
		return
	}
	element.SetAttr("", "", name, value)
}

func asInt(value any) int {
	number, ok := sheet.AsFloat(value)
	if !ok {
		return 0
	}
	return int(math.Round(number))
}

// styleKey dung khoa cache giong ban C# (baseXf + "|" + JSON cua spec).
func styleKey(baseXf int, spec map[string]any) (string, error) {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(baseXf) + "|" + string(encoded), nil
}
