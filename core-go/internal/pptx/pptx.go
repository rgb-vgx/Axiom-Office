// Package pptx: doc/tao file .pptx (thay python-pptx). Port cua src/AxiomOffice.Host/Mcp/PptFiles.cs.
package pptx

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/docx"
	"axiomoffice/core/internal/filesafe"
	"axiomoffice/core/internal/ooxml"
	"axiomoffice/core/internal/sheet"
	"axiomoffice/core/internal/templates"
	"axiomoffice/core/internal/textutil"
)

// Namespace dung trong bai thuyet trinh.
const (
	P = ooxml.PresentationNS
	A = ooxml.DrawingNS
	R = ooxml.OfficeRelNamespace

	slideContentType = "application/vnd.openxmlformats-officedocument.presentationml.slide+xml"
)

// p tao phan tu roi thuoc namespace PresentationML (tien to "p").
func p(local string) *ooxml.Element { return ooxml.NewElement(P, "p", local) }

// a tao phan tu roi thuoc namespace DrawingML (tien to "a").
func a(local string) *ooxml.Element { return ooxml.NewElement(A, "a", local) }

// withAttrs dat cac thuoc tinh khong tien to theo cap "ten", "gia tri".
func withAttrs(element *ooxml.Element, pairs ...string) *ooxml.Element {
	for index := 0; index+1 < len(pairs); index += 2 {
		element.SetAttr("", "", pairs[index], pairs[index+1])
	}
	return element
}

// ---- doc ----

// PresentationPart tra ve part bai thuyet trinh chinh.
func PresentationPart(pkg *ooxml.Package) (string, error) {
	main := pkg.RelOfType("", "officeDocument")
	if main == nil || !pkg.Exists(main.TargetPart) {
		return "", fmt.Errorf("not a PowerPoint presentation (no main part)")
	}
	return main.TargetPart, nil
}

// SlideParts liet ke cac part slide theo thu tu trong sldIdLst.
func SlideParts(pkg *ooxml.Package, presentationPart string) ([]string, error) {
	doc, err := pkg.XML(presentationPart)
	if err != nil {
		return nil, err
	}
	list := doc.Root().Child(P, "sldIdLst")
	if list == nil {
		return []string{}, nil
	}
	rels := pkg.Rels(presentationPart)
	result := []string{}
	for _, slideID := range list.Children(P, "sldId") {
		relID := slideID.AttrValue(R, "id")
		for _, rel := range rels {
			if rel.ID == relID && pkg.Exists(rel.TargetPart) {
				result = append(result, rel.TargetPart)
				break
			}
		}
	}
	return result, nil
}

var shapeKinds = map[string]bool{
	"sp": true, "grpSp": true, "graphicFrame": true, "cxnSp": true, "pic": true, "contentPart": true,
}

func shapeElements(spTree *ooxml.Element) []*ooxml.Element {
	if spTree == nil {
		return nil
	}
	shapes := []*ooxml.Element{}
	for _, element := range spTree.Elements() {
		if element.URI == P && shapeKinds[element.Local] {
			shapes = append(shapes, element)
		}
	}
	return shapes
}

func textFrameText(txBody *ooxml.Element) string {
	if txBody == nil {
		return ""
	}
	lines := []string{}
	for _, paragraph := range txBody.Children(A, "p") {
		var builder strings.Builder
		for _, item := range paragraph.Elements() {
			if item.URI != A {
				continue
			}
			switch item.Local {
			case "r", "fld":
				if text := item.Child(A, "t"); text != nil {
					builder.WriteString(text.Text())
				}
			case "br":
				// Ky tu xuong dong mem trong o text cua PowerPoint.
				builder.WriteString("\v")
			}
		}
		lines = append(lines, builder.String())
	}
	return strings.Join(lines, "\n")
}

func slideTexts(spTree *ooxml.Element) []string {
	texts := []string{}
	for _, shape := range shapeElements(spTree) {
		if shape.Local != "sp" {
			continue
		}
		text := textFrameText(shape.Child(P, "txBody"))
		if strings.TrimSpace(text) != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func notesText(pkg *ooxml.Package, slidePart string) string {
	notes := pkg.RelOfType(slidePart, "notesSlide")
	if notes == nil || !pkg.Exists(notes.TargetPart) {
		return ""
	}
	doc, err := pkg.XML(notes.TargetPart)
	if err != nil || doc.Root() == nil {
		return ""
	}
	commonSlide := doc.Root().Child(P, "cSld")
	if commonSlide == nil {
		return ""
	}
	spTree := commonSlide.Child(P, "spTree")
	if spTree == nil {
		return ""
	}
	for _, shape := range spTree.Children(P, "sp") {
		placeholder := firstDescendant(shape, P, "ph")
		if placeholder != nil && placeholder.AttrValue("", "type") == "body" {
			return textFrameText(shape.Child(P, "txBody"))
		}
	}
	return ""
}

func firstDescendant(element *ooxml.Element, uri, local string) *ooxml.Element {
	for _, kid := range element.Elements() {
		if kid.URI == uri && kid.Local == local {
			return kid
		}
		if found := firstDescendant(kid, uri, local); found != nil {
			return found
		}
	}
	return nil
}

// Profile thong ke tung slide: layout, so hinh, text va ghi chu.
func Profile(path string) (map[string]any, error) {
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
	slides := []any{}
	for index, slidePart := range slideParts {
		doc, err := pkg.XML(slidePart)
		if err != nil {
			return nil, err
		}
		var spTree *ooxml.Element
		if commonSlide := doc.Root().Child(P, "cSld"); commonSlide != nil {
			spTree = commonSlide.Child(P, "spTree")
		}
		layoutName := ""
		if layout := pkg.RelOfType(slidePart, "slideLayout"); layout != nil && pkg.Exists(layout.TargetPart) {
			if layoutDoc, err := pkg.XML(layout.TargetPart); err == nil && layoutDoc.Root() != nil {
				if commonSlide := layoutDoc.Root().Child(P, "cSld"); commonSlide != nil {
					layoutName = commonSlide.AttrValue("", "name")
				}
			}
		}
		notes := notesText(pkg, slidePart)
		slides = append(slides, map[string]any{
			"index":  index + 1,
			"layout": layoutName,
			"shapes": len(shapeElements(spTree)),
			"texts":  slideTexts(spTree),
			"notes":  textutil.Truncate(notes, 300),
		})
	}
	var width, height any
	if presentationDoc, err := pkg.XML(presentation); err == nil && presentationDoc.Root() != nil {
		if size := presentationDoc.Root().Child(P, "sldSz"); size != nil {
			width, _ = strconv.ParseInt(size.AttrValue("", "cx"), 10, 64)
			height, _ = strconv.ParseInt(size.AttrValue("", "cy"), 10, 64)
		}
	}
	return map[string]any{
		"path":        path,
		"slide_count": len(slideParts),
		"size":        map[string]any{"width": width, "height": height},
		"slides":      slides,
	}, nil
}

// GetText gop text cua tat ca slide (kem ghi chu).
func GetText(path string, maxChars int) (map[string]any, error) {
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
	parts := []string{}
	for index, slidePart := range slideParts {
		parts = append(parts, fmt.Sprintf("[Slide %d]", index+1))
		doc, err := pkg.XML(slidePart)
		if err != nil {
			return nil, err
		}
		var spTree *ooxml.Element
		if commonSlide := doc.Root().Child(P, "cSld"); commonSlide != nil {
			spTree = commonSlide.Child(P, "spTree")
		}
		parts = append(parts, slideTexts(spTree)...)
		if notes := notesText(pkg, slidePart); strings.TrimSpace(notes) != "" {
			parts = append(parts, "[Notes] "+notes)
		}
	}
	text := strings.Join(parts, "\n")
	total := textutil.Length(text)
	truncated := false
	if maxChars > 0 && total > maxChars {
		text = textutil.Truncate(text, maxChars)
		truncated = true
	}
	return map[string]any{"path": path, "chars": total, "truncated": truncated, "text": text}, nil
}

// ---- ghi ----

// Create tao file .pptx tu template nhung san.
func Create(path string, slides []any, overwrite bool) (map[string]any, error) {
	if err := filesafe.RequireNewOrOverwrite(path, overwrite); err != nil {
		return nil, err
	}
	created := 0
	err := ooxml.CreateFromTemplate(path, templates.Pptx(), func(pkg *ooxml.Package) error {
		created = 0
		for _, item := range slides {
			var spec map[string]any
			switch typed := item.(type) {
			case string:
				spec = map[string]any{"title": typed}
			case map[string]any:
				spec = typed
			default:
				continue
			}
			layout := 1
			if value, ok := spec["layout"]; ok && value != nil {
				layout = asInt(value)
			}
			title := ""
			if value, ok := spec["title"]; ok && value != nil {
				title = sheet.ToText(value)
			}
			var bullets []string
			if value, ok := spec["bullets"].([]any); ok {
				for _, bullet := range value {
					bullets = append(bullets, sheet.ToText(bullet))
				}
			} else if value, ok := spec["text"]; ok && value != nil {
				bullets = []string{sheet.ToText(value)}
			}
			if err := AddSlide(pkg, layout, title, bullets); err != nil {
				return err
			}
			created++
		}
		return ResetCore(pkg)
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"created": path, "slides": created}, nil
}

// AddSlideFile them mot slide vao file .pptx co san.
func AddSlideFile(path, title string, bullets []any, layout int) (map[string]any, error) {
	if err := filesafe.RequireFile(path); err != nil {
		return nil, err
	}
	count := 0
	err := ooxml.Edit(path, func(pkg *ooxml.Package) error {
		lines := []string(nil)
		if bullets != nil {
			lines = []string{}
			for _, bullet := range bullets {
				lines = append(lines, sheet.ToText(bullet))
			}
		}
		if err := AddSlide(pkg, layout, title, lines); err != nil {
			return err
		}
		presentation, err := PresentationPart(pkg)
		if err != nil {
			return err
		}
		slideParts, err := SlideParts(pkg, presentation)
		if err != nil {
			return err
		}
		count = len(slideParts)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "slide_count": count}, nil
}

// AddSlide dung mot slide moi theo layout, sao chep placeholder cua layout (bo ngay/footer/so trang).
func AddSlide(pkg *ooxml.Package, layoutIndex int, title string, bullets []string) error {
	presentation, err := PresentationPart(pkg)
	if err != nil {
		return err
	}
	layouts, err := LayoutParts(pkg, presentation)
	if err != nil {
		return err
	}
	if len(layouts) == 0 {
		return fmt.Errorf("presentation has no slide layouts")
	}
	layoutPart := layouts[max(0, min(layoutIndex, len(layouts)-1))]
	slidePart := pkg.NextPartName("ppt/slides/slide%d.xml")

	spTree := p("spTree")
	nonVisual := p("nvGrpSpPr")
	nonVisual.Append(withAttrs(p("cNvPr"), "id", "1", "name", ""))
	nonVisual.Append(p("cNvGrpSpPr"))
	nonVisual.Append(p("nvPr"))
	spTree.Append(nonVisual)
	groupProps := p("grpSpPr")
	transform := a("xfrm")
	transform.Append(withAttrs(a("off"), "x", "0", "y", "0"))
	transform.Append(withAttrs(a("ext"), "cx", "0", "cy", "0"))
	transform.Append(withAttrs(a("chOff"), "x", "0", "y", "0"))
	transform.Append(withAttrs(a("chExt"), "cx", "0", "cy", "0"))
	groupProps.Append(transform)
	spTree.Append(groupProps)

	layoutDoc, err := pkg.XML(layoutPart)
	if err != nil {
		return err
	}
	var layoutTree *ooxml.Element
	if commonSlide := layoutDoc.Root().Child(P, "cSld"); commonSlide != nil {
		layoutTree = commonSlide.Child(P, "spTree")
	}
	if layoutTree == nil {
		return fmt.Errorf("slide layout has no shape tree: %s", layoutPart)
	}

	nextID := 2
	for _, shape := range layoutTree.Children(P, "sp") {
		placeholder := firstDescendant(shape, P, "ph")
		if placeholder == nil {
			continue
		}
		kind := placeholder.AttrValue("", "type")
		if kind == "" {
			kind = "obj"
		}
		if kind == "dt" || kind == "ftr" || kind == "sldNum" {
			continue
		}
		name := "Placeholder " + strconv.Itoa(nextID)
		if layoutCnvPr := firstDescendant(shape, P, "cNvPr"); layoutCnvPr != nil {
			if value := layoutCnvPr.AttrValue("", "name"); value != "" {
				name = value
			}
		}
		clone := p("ph")
		clone.Attrs = append(clone.Attrs, placeholder.Attrs...)
		sp := p("sp")
		nonVisualProps := p("nvSpPr")
		nonVisualProps.Append(withAttrs(p("cNvPr"), "id", strconv.Itoa(nextID), "name", name))
		spLocks := a("spLocks")
		spLocks.SetAttr("", "", "noGrp", "1")
		cnvSpPr := p("cNvSpPr")
		cnvSpPr.Append(spLocks)
		nonVisualProps.Append(cnvSpPr)
		nvPr := p("nvPr")
		nvPr.Append(clone)
		nonVisualProps.Append(nvPr)
		sp.Append(nonVisualProps)
		sp.Append(p("spPr"))
		switch kind {
		case "title", "ctrTitle", "subTitle", "body", "obj", "vertTitle":
			txBody := p("txBody")
			txBody.Append(a("bodyPr"))
			txBody.Append(a("lstStyle"))
			txBody.Append(a("p"))
			sp.Append(txBody)
		}
		spTree.Append(sp)
		nextID++
	}

	if title != "" {
		if titleShape := PlaceholderByIdx(spTree, 0); titleShape != nil {
			SetText(titleShape, []string{title})
		}
	}
	if len(bullets) > 0 {
		if body := PlaceholderByIdx(spTree, 1); body != nil {
			SetText(body, bullets)
		}
	}

	slide := p("sld")
	slide.DeclareNamespace("a", A)
	slide.DeclareNamespace("r", R)
	slide.DeclareNamespace("p", P)
	commonSlide := p("cSld")
	commonSlide.Append(spTree)
	slide.Append(commonSlide)
	clrMapOvr := p("clrMapOvr")
	clrMapOvr.Append(a("masterClrMapping"))
	slide.Append(clrMapOvr)
	pkg.Put(slidePart, ooxml.NewDocument(slide))
	if err := pkg.SetContentTypeOverride(slidePart, slideContentType); err != nil {
		return err
	}
	pkg.AddRel(slidePart, "slideLayout", layoutPart)
	relID := pkg.AddRel(presentation, "slide", slidePart)

	pres, err := pkg.XML(presentation)
	if err != nil {
		return err
	}
	list := pres.Root().Child(P, "sldIdLst")
	if list == nil {
		list = p("sldIdLst")
		anchor := pres.Root().Child(P, "handoutMasterIdLst")
		if anchor == nil {
			anchor = pres.Root().Child(P, "notesMasterIdLst")
		}
		if anchor == nil {
			anchor = pres.Root().Child(P, "sldMasterIdLst")
		}
		if anchor != nil {
			pres.Root().InsertAt(pres.Root().ChildIndex(anchor)+1, list)
		} else {
			pres.Root().InsertAt(0, list)
		}
	}
	highest := int64(255)
	for _, existing := range list.Children(P, "sldId") {
		if value, err := strconv.ParseInt(existing.AttrValue("", "id"), 10, 64); err == nil && value > highest {
			highest = value
		}
	}
	entry := p("sldId")
	entry.SetAttr("", "", "id", strconv.FormatInt(highest+1, 10))
	entry.SetAttr(R, "r", "id", relID)
	list.Append(entry)
	pkg.Put(presentation, pres)
	return nil
}

// LayoutParts liet ke layout theo thu tu sldLayoutIdLst cua slide master dau tien.
func LayoutParts(pkg *ooxml.Package, presentation string) ([]string, error) {
	doc, err := pkg.XML(presentation)
	if err != nil {
		return nil, err
	}
	masterList := doc.Root().Child(P, "sldMasterIdLst")
	if masterList == nil {
		return []string{}, nil
	}
	masterID := masterList.Child(P, "sldMasterId")
	if masterID == nil {
		return []string{}, nil
	}
	var masterPart string
	for _, rel := range pkg.Rels(presentation) {
		if rel.ID == masterID.AttrValue(R, "id") {
			masterPart = rel.TargetPart
			break
		}
	}
	if masterPart == "" {
		return []string{}, nil
	}
	masterDoc, err := pkg.XML(masterPart)
	if err != nil {
		return nil, err
	}
	list := masterDoc.Root().Child(P, "sldLayoutIdLst")
	if list == nil {
		return []string{}, nil
	}
	masterRels := pkg.Rels(masterPart)
	result := []string{}
	for _, layoutID := range list.Children(P, "sldLayoutId") {
		for _, rel := range masterRels {
			if rel.ID == layoutID.AttrValue(R, "id") {
				result = append(result, rel.TargetPart)
				break
			}
		}
	}
	return result, nil
}

// PlaceholderByIdx tim hinh theo chi so placeholder (idx).
func PlaceholderByIdx(spTree *ooxml.Element, idx int) *ooxml.Element {
	for _, shape := range spTree.Children(P, "sp") {
		placeholder := firstDescendant(shape, P, "ph")
		if placeholder == nil {
			continue
		}
		value, _ := strconv.Atoi(placeholder.AttrValue("", "idx"))
		if value == idx {
			return shape
		}
	}
	return nil
}

// SetText ghi cac doan text vao o text cua hinh.
func SetText(shape *ooxml.Element, paragraphs []string) {
	txBody := shape.Child(P, "txBody")
	if txBody == nil {
		txBody = p("txBody")
		txBody.Append(a("bodyPr"))
		txBody.Append(a("lstStyle"))
		shape.Append(txBody)
	}
	for _, existing := range txBody.Children(A, "p") {
		txBody.RemoveChild(existing)
	}
	lines := []string{}
	for _, paragraph := range paragraphs {
		lines = append(lines, strings.Split(strings.ReplaceAll(paragraph, "\r\n", "\n"), "\n")...)
	}
	for _, line := range lines {
		txBodyParagraph := a("p")
		if line != "" {
			run := a("r")
			run.Append(withAttrs(a("rPr"), "lang", "vi-VN", "dirty", "0"))
			textElement := a("t")
			textElement.SetText(line)
			run.Append(textElement)
			txBodyParagraph.Append(run)
		}
		txBody.Append(txBodyParagraph)
	}
}

// ResetCore bo dau vet cua template trong thuoc tinh tai lieu.
func ResetCore(pkg *ooxml.Package) error {
	core := pkg.RelOfType("", "metadata/core-properties")
	if core == nil || !pkg.Exists(core.TargetPart) {
		return nil
	}
	doc, err := pkg.XML(core.TargetPart)
	if err != nil {
		return err
	}
	now := timeNow()
	docx.SetCore(doc, "http://purl.org/dc/elements/1.1/", "creator", "")
	docx.SetCore(doc, "http://schemas.openxmlformats.org/package/2006/metadata/core-properties", "lastModifiedBy", "")
	docx.SetCore(doc, "http://purl.org/dc/terms/", "created", now)
	docx.SetCore(doc, "http://purl.org/dc/terms/", "modified", now)
	pkg.Put(core.TargetPart, doc)
	return nil
}

func asInt(value any) int {
	number, ok := sheet.AsFloat(value)
	if !ok {
		return 0
	}
	return int(math.Round(number))
}

// timeNow dung cung dinh dang voi DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ") cua ban C#.
func timeNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
