// Package docx: doc/tao file .docx (chi paragraph/table cap body, nhu python-docx).
// Port cua src/AxiomOffice.Host/Mcp/WordFiles.cs.
package docx

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/filesafe"
	"axiomoffice/core/internal/ooxml"
	"axiomoffice/core/internal/sheet"
	"axiomoffice/core/internal/templates"
	"axiomoffice/core/internal/textutil"
)

// W la namespace WordprocessingML (dung lam ten ngan trong file nay).
const W = ooxml.WordprocessingNS

// Namespace cua phan thuoc tinh tai lieu.
const (
	corePropertiesNS = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
	dcNS             = "http://purl.org/dc/elements/1.1/"
	dcTermsNS        = "http://purl.org/dc/terms/"
	xsiNS            = "http://www.w3.org/2001/XMLSchema-instance"
)

// Model la cay ngu canh doc duoc tu mot file .docx.
type Model struct {
	Paragraphs            []*ooxml.Element
	Tables                []*ooxml.Element
	Sections              int
	StyleNames            map[string]string
	DefaultParagraphStyle string
	Core                  *ooxml.Document
}

var headingPattern = regexp.MustCompile(`^heading ([1-9])$`)

// Load mo file .docx va dung model.
func Load(path string) (*Model, error) {
	if err := filesafe.RequireFile(path); err != nil {
		return nil, err
	}
	pkg, err := ooxml.OpenRead(path)
	if err != nil {
		return nil, err
	}
	documentPart, err := MainPart(pkg)
	if err != nil {
		return nil, err
	}
	document, err := pkg.XML(documentPart)
	if err != nil {
		return nil, err
	}
	model := &Model{StyleNames: map[string]string{}, DefaultParagraphStyle: "Normal"}
	if body := document.Root().Child(W, "body"); body != nil {
		model.Paragraphs = body.Children(W, "p")
		model.Tables = body.Children(W, "tbl")
		model.Sections = len(body.Children(W, "sectPr"))
		for _, paragraph := range model.Paragraphs {
			if properties := paragraph.Child(W, "pPr"); properties != nil && properties.Child(W, "sectPr") != nil {
				model.Sections++
			}
		}
	}
	if styles := pkg.RelOfType(documentPart, "styles"); styles != nil && pkg.Exists(styles.TargetPart) {
		stylesDoc, err := pkg.XML(styles.TargetPart)
		if err == nil && stylesDoc.Root() != nil {
			for _, style := range stylesDoc.Root().Children(W, "style") {
				id := style.AttrValue(W, "styleId")
				raw := id
				if name := style.Child(W, "name"); name != nil {
					raw = name.AttrValue(W, "val")
				}
				if id != "" {
					model.StyleNames[id] = uiName(raw)
				}
				if style.AttrValue(W, "type") == "paragraph" && isOn(style.AttrValue(W, "default")) && id != "" {
					model.DefaultParagraphStyle = model.StyleNames[id]
				}
			}
		}
	}
	if core := pkg.RelOfType("", "metadata/core-properties"); core != nil && pkg.Exists(core.TargetPart) {
		if coreDoc, err := pkg.XML(core.TargetPart); err == nil {
			model.Core = coreDoc
		}
	}
	return model, nil
}

// MainPart tra ve part tai lieu chinh cua goi.
func MainPart(pkg *ooxml.Package) (string, error) {
	main := pkg.RelOfType("", "officeDocument")
	if main == nil || !pkg.Exists(main.TargetPart) {
		return "", fmt.Errorf("not a Word document (no main document part)")
	}
	return main.TargetPart, nil
}

// uiName doi ten style noi bo thanh ten hien thi nhu python-docx ("heading 1" -> "Heading 1").
func uiName(name string) string {
	if name == "" {
		return "None"
	}
	if match := headingPattern.FindStringSubmatch(name); match != nil {
		return "Heading " + match[1]
	}
	switch name {
	case "caption":
		return "Caption"
	case "header":
		return "Header"
	case "footer":
		return "Footer"
	}
	return name
}

func isOn(value string) bool {
	return value == "1" || value == "true" || value == "on"
}

// StyleOf tra ve ten hien thi cua style doan dang dung.
func StyleOf(model *Model, paragraph *ooxml.Element) string {
	properties := paragraph.Child(W, "pPr")
	if properties == nil {
		return model.DefaultParagraphStyle
	}
	style := properties.Child(W, "pStyle")
	if style == nil {
		return model.DefaultParagraphStyle
	}
	id := style.AttrValue(W, "val")
	if id == "" {
		return model.DefaultParagraphStyle
	}
	if name, ok := model.StyleNames[id]; ok {
		return name
	}
	return id
}

// ParagraphText gop text nhu paragraph.text cua python-docx: run truc tiep va run trong
// hyperlink/ins/smartTag/fldSimple.
func ParagraphText(paragraph *ooxml.Element) string {
	var builder strings.Builder
	appendRuns(paragraph, &builder)
	return builder.String()
}

func appendRuns(container *ooxml.Element, builder *strings.Builder) {
	for _, child := range container.Elements() {
		// Chi lay phan tu thuoc namespace WordprocessingML.
		if child.URI != W {
			continue
		}
		switch child.Local {
		case "r":
			for _, item := range child.Elements() {
				if item.URI != W {
					continue
				}
				switch item.Local {
				case "t":
					builder.WriteString(item.Text())
				case "tab":
					builder.WriteString("\t")
				case "br", "cr":
					builder.WriteString("\n")
				case "noBreakHyphen":
					builder.WriteString("-")
				}
			}
		case "hyperlink", "ins", "smartTag", "fldSimple", "customXml", "sdt", "sdtContent":
			appendRuns(child, builder)
		}
	}
}

// TableRows doc bang thanh chuoi 2 chieu, xu ly o gop nhu row.cells cua python-docx:
// gridSpan lap lai o, vMerge "continue" lay o phia tren.
func TableRows(table *ooxml.Element) [][]string {
	rows := [][]string{}
	var previous []string
	for _, row := range table.Children(W, "tr") {
		cells := []string{}
		for _, cell := range row.Children(W, "tc") {
			span := 1
			continueMerge := false
			if properties := cell.Child(W, "tcPr"); properties != nil {
				if gridSpan := properties.Child(W, "gridSpan"); gridSpan != nil {
					if value, err := strconv.Atoi(gridSpan.AttrValue(W, "val")); err == nil {
						span = max(1, value)
					}
				}
				if merge := properties.Child(W, "vMerge"); merge != nil {
					kind := merge.AttrValue(W, "val")
					if kind == "" {
						kind = "continue"
					}
					continueMerge = kind == "continue"
				}
			}
			texts := []string{}
			for _, paragraph := range cell.Children(W, "p") {
				texts = append(texts, ParagraphText(paragraph))
			}
			text := strings.Join(texts, "\n")
			for index := 0; index < span; index++ {
				column := len(cells)
				if continueMerge && previous != nil && column < len(previous) {
					cells = append(cells, previous[column])
				} else {
					cells = append(cells, text)
				}
			}
		}
		rows = append(rows, cells)
		previous = cells
	}
	return rows
}

// Profile thong ke doan/bang/section, cach dung style va thuoc tinh tai lieu.
func Profile(path string) (map[string]any, error) {
	model, err := Load(path)
	if err != nil {
		return nil, err
	}
	styleCounts := map[string]any{}
	for _, paragraph := range model.Paragraphs {
		name := StyleOf(model, paragraph)
		if count, ok := styleCounts[name].(int); ok {
			styleCounts[name] = count + 1
		} else {
			styleCounts[name] = 1
		}
	}
	return map[string]any{
		"path":         path,
		"paragraphs":   len(model.Paragraphs),
		"tables":       len(model.Tables),
		"sections":     model.Sections,
		"style_counts": styleCounts,
		"core": map[string]any{
			"title":    coreValue(model.Core, dcNS, "title"),
			"author":   coreValue(model.Core, dcNS, "creator"),
			"created":  coreValue(model.Core, dcTermsNS, "created"),
			"modified": coreValue(model.Core, dcTermsNS, "modified"),
		},
	}, nil
}

func coreValue(core *ooxml.Document, uri, local string) any {
	if core == nil || core.Root() == nil {
		return nil
	}
	element := core.Root().Child(uri, local)
	if element == nil {
		return nil
	}
	if value := element.Text(); value != "" {
		return value
	}
	return nil
}

// GetText gop text cua doan va (tuy chon) bang.
func GetText(path string, maxChars int, includeTables bool) (map[string]any, error) {
	model, err := Load(path)
	if err != nil {
		return nil, err
	}
	parts := []string{}
	for _, paragraph := range model.Paragraphs {
		parts = append(parts, ParagraphText(paragraph))
	}
	if includeTables {
		for _, table := range model.Tables {
			for _, row := range TableRows(table) {
				parts = append(parts, strings.Join(row, " | "))
			}
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

// FindText tim doan chua chuoi truy van (khong phan biet hoa thuong).
func FindText(path, query string, maxResults int) (map[string]any, error) {
	model, err := Load(path)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	matches := []any{}
	for index := 0; index < len(model.Paragraphs) && needle != ""; index++ {
		text := ParagraphText(model.Paragraphs[index])
		if strings.Contains(strings.ToLower(text), needle) {
			clipped := text
			if textutil.Length(clipped) > 300 {
				clipped = textutil.Truncate(clipped, 300)
			}
			matches = append(matches, map[string]any{
				"index": index,
				"style": StyleOf(model, model.Paragraphs[index]),
				"text":  clipped,
			})
			if len(matches) >= maxResults {
				break
			}
		}
	}
	return map[string]any{"query": query, "count": len(matches), "matches": matches}, nil
}

// ExtractTable lay mot bang theo chi so thanh mang 2 chieu.
func ExtractTable(path string, index int) (map[string]any, error) {
	model, err := Load(path)
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(model.Tables) {
		return nil, fmt.Errorf("table index out of range (file has %d tables)", len(model.Tables))
	}
	rows := TableRows(model.Tables[index])
	return map[string]any{"path": path, "index": index, "row_count": len(rows), "rows": rows}, nil
}

// ---- tao ----

// Create tao file .docx tu template nhung san.
func Create(path string, paragraphs []any, title, author string, overwrite bool) (map[string]any, error) {
	if err := filesafe.RequireNewOrOverwrite(path, overwrite); err != nil {
		return nil, err
	}
	added := 0
	err := ooxml.CreateFromTemplate(path, templates.Docx(), func(pkg *ooxml.Package) error {
		documentPart, err := MainPart(pkg)
		if err != nil {
			return err
		}
		document, err := pkg.XML(documentPart)
		if err != nil {
			return err
		}
		body := document.Root().Child(W, "body")
		if body == nil {
			return fmt.Errorf("template has no document body")
		}
		sectPr := body.Child(W, "sectPr")
		styleIds := paragraphStyleIDs(pkg, documentPart)
		blockWidth := blockWidth(sectPr)
		added = 0
		for _, spec := range paragraphs {
			var block *ooxml.Element
			switch typed := spec.(type) {
			case string:
				block = paragraph(typed, "")
			case map[string]any:
				if raw, ok := typed["table"]; ok {
					data := toMatrix(raw)
					if len(data) == 0 {
						continue
					}
					block = table(data, blockWidth)
				} else {
					style := ""
					if value, ok := typed["style"]; ok && value != nil {
						style = sheet.ToText(value)
					}
					styleID := ""
					if style != "" {
						found, ok := styleIds[strings.ToLower(style)]
						if !ok {
							return fmt.Errorf("no paragraph style with name '%s'", style)
						}
						styleID = found
					}
					text := ""
					if value, ok := typed["text"]; ok && value != nil {
						text = sheet.ToText(value)
					}
					block = paragraph(text, styleID)
				}
			default:
				continue
			}
			if sectPr != nil {
				body.InsertAt(body.ChildIndex(sectPr), block)
			} else {
				body.Append(block)
			}
			added++
		}
		pkg.Put(documentPart, document)
		return writeCore(pkg, title, author)
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"created": path, "blocks": added}, nil
}

// paragraphStyleIDs doi ten style (ca ten noi bo lan ten hien thi, khong phan biet hoa thuong)
// thanh styleId, chi lay style doan.
func paragraphStyleIDs(pkg *ooxml.Package, documentPart string) map[string]string {
	result := map[string]string{}
	styles := pkg.RelOfType(documentPart, "styles")
	if styles == nil || !pkg.Exists(styles.TargetPart) {
		return result
	}
	doc, err := pkg.XML(styles.TargetPart)
	if err != nil || doc.Root() == nil {
		return result
	}
	for _, style := range doc.Root().Children(W, "style") {
		if style.AttrValue(W, "type") != "paragraph" {
			continue
		}
		id := style.AttrValue(W, "styleId")
		raw := id
		if name := style.Child(W, "name"); name != nil {
			if value := name.AttrValue(W, "val"); value != "" {
				raw = value
			}
		}
		if id == "" || raw == "" {
			continue
		}
		result[strings.ToLower(raw)] = id
		result[strings.ToLower(uiName(raw))] = id
		if _, exists := result[strings.ToLower(id)]; !exists {
			result[strings.ToLower(id)] = id
		}
	}
	return result
}

// blockWidth tinh be rong vung soan thao tu kich thuoc trang va le (don vi twip).
func blockWidth(sectPr *ooxml.Element) int {
	if sectPr == nil {
		return 8640
	}
	size := sectPr.Child(W, "pgSz")
	margin := sectPr.Child(W, "pgMar")
	if size == nil || margin == nil {
		return 8640
	}
	width, err := strconv.Atoi(size.AttrValue(W, "w"))
	if err != nil {
		return 8640
	}
	left, err := strconv.Atoi(margin.AttrValue(W, "left"))
	if err != nil {
		return 8640
	}
	right, err := strconv.Atoi(margin.AttrValue(W, "right"))
	if err != nil {
		return 8640
	}
	return max(1440, width-left-right)
}

func paragraph(text, styleID string) *ooxml.Element {
	element := ooxml.NewElement(W, "w", "p")
	if styleID != "" {
		properties := element.AppendElement(W, "w", "pPr")
		properties.AppendElement(W, "w", "pStyle").SetAttr(W, "w", "val", styleID)
	}
	if text != "" {
		element.Append(run(text))
	}
	return element
}

// run dung mot <w:r>, doi "\n" thanh <w:br/> va "\t" thanh <w:tab/> nhu run.text cua python-docx.
func run(text string) *ooxml.Element {
	element := ooxml.NewElement(W, "w", "r")
	var buffer strings.Builder
	flush := func() {
		if buffer.Len() == 0 {
			return
		}
		textElement := element.AppendElement(W, "w", "t")
		textElement.SetAttr(ooxml.XMLNamespace, "xml", "space", "preserve")
		textElement.AddText(buffer.String())
		buffer.Reset()
	}
	for _, character := range strings.ReplaceAll(text, "\r\n", "\n") {
		switch character {
		case '\n', '\r':
			flush()
			element.AppendElement(W, "w", "br")
		case '\t':
			flush()
			element.AppendElement(W, "w", "tab")
		default:
			buffer.WriteRune(character)
		}
	}
	flush()
	return element
}

func table(data [][]any, blockWidth int) *ooxml.Element {
	columns := 1
	for _, row := range data {
		if len(row) > columns {
			columns = len(row)
		}
	}
	columnWidth := blockWidth / columns
	element := ooxml.NewElement(W, "w", "tbl")

	properties := element.AppendElement(W, "w", "tblPr")
	properties.AppendElement(W, "w", "tblStyle").SetAttr(W, "w", "val", "TableGrid")
	width := properties.AppendElement(W, "w", "tblW")
	width.SetAttr(W, "w", "type", "auto")
	width.SetAttr(W, "w", "w", "0")
	look := properties.AppendElement(W, "w", "tblLook")
	for _, pair := range [][2]string{
		{"firstColumn", "1"}, {"firstRow", "1"}, {"lastColumn", "0"},
		{"lastRow", "0"}, {"noHBand", "0"}, {"noVBand", "1"}, {"val", "04A0"},
	} {
		look.SetAttr(W, "w", pair[0], pair[1])
	}

	grid := element.AppendElement(W, "w", "tblGrid")
	for index := 0; index < columns; index++ {
		grid.AppendElement(W, "w", "gridCol").SetAttr(W, "w", "w", strconv.Itoa(columnWidth))
	}

	for _, row := range data {
		rowElement := element.AppendElement(W, "w", "tr")
		for index := 0; index < columns; index++ {
			var value any
			if index < len(row) {
				value = row[index]
			}
			cell := rowElement.AppendElement(W, "w", "tc")
			cellProperties := cell.AppendElement(W, "w", "tcPr")
			cellWidth := cellProperties.AppendElement(W, "w", "tcW")
			cellWidth.SetAttr(W, "w", "type", "dxa")
			cellWidth.SetAttr(W, "w", "w", strconv.Itoa(columnWidth))
			cell.Append(paragraph(sheet.ToText(value), ""))
		}
	}
	return element
}

func toMatrix(value any) [][]any {
	rows, ok := value.([]any)
	if !ok {
		return [][]any{}
	}
	matrix := make([][]any, 0, len(rows))
	for _, row := range rows {
		if cells, ok := row.([]any); ok {
			matrix = append(matrix, cells)
			continue
		}
		matrix = append(matrix, []any{row})
	}
	return matrix
}

// writeCore ghi thuoc tinh tai lieu, bo dau vet "python-docx" cua template.
func writeCore(pkg *ooxml.Package, title, author string) error {
	core := pkg.RelOfType("", "metadata/core-properties")
	if core == nil || !pkg.Exists(core.TargetPart) {
		return nil
	}
	doc, err := pkg.XML(core.TargetPart)
	if err != nil || doc.Root() == nil {
		return err
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	SetCore(doc, dcNS, "title", title)
	SetCore(doc, dcNS, "creator", author)
	SetCore(doc, dcNS, "description", "")
	SetCore(doc, corePropertiesNS, "lastModifiedBy", author)
	SetCore(doc, dcTermsNS, "created", now)
	SetCore(doc, dcTermsNS, "modified", now)
	pkg.Put(core.TargetPart, doc)
	return nil
}

// SetCore dat (hoac tao) mot phan tu thuoc tinh tai lieu.
func SetCore(doc *ooxml.Document, uri, local, value string) {
	if doc.Root() == nil {
		return
	}
	element := doc.Root().Child(uri, local)
	if element == nil {
		prefix := prefixFor(uri)
		element = ooxml.NewElement(uri, prefix, local)
		if uri == dcTermsNS {
			element.SetAttr(xsiNS, "xsi", "type", "dcterms:W3CDTF")
		}
		// Phai chac chan tien to da duoc khai bao o goc, neu khong file se sai namespace.
		if prefix != "" && !doc.Root().HasPrefix(prefix) {
			doc.Root().DeclareNamespace(prefix, uri)
		}
		doc.Root().Append(element)
	}
	element.SetText(value)
}

func prefixFor(uri string) string {
	switch uri {
	case dcNS:
		return "dc"
	case dcTermsNS:
		return "dcterms"
	case corePropertiesNS:
		return "cp"
	case ooxml.XMLNamespace:
		return "xml"
	}
	return ""
}
