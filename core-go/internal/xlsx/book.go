package xlsx

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"axiomoffice/core/internal/ooxml"
	"axiomoffice/core/internal/sheet"
)

// SheetRef la mot sheet trong workbook.
type SheetRef struct {
	Name    string
	RelID   string
	Part    string
	SheetID int
	Element *ooxml.Element
}

// Grid la luoi o thua: chi giu cac o trong vung can doc; MaxRow/MaxCol la kich thuoc sheet.
type Grid struct {
	Name     string
	MaxRow   int
	MaxCol   int
	HasCells bool
	rows     map[int]map[int]any
}

// NewGrid tao mot luoi rong (dung khi doc csv roi tra ve qua tool excel_read).
func NewGrid(name string) *Grid {
	return &Grid{Name: name, rows: map[int]map[int]any{}}
}

func (g *Grid) Get(row, col int) any {
	return g.rows[row][col]
}

func (g *Grid) Set(row, col int, value any) {
	if g.rows[row] == nil {
		g.rows[row] = map[int]any{}
	}
	g.rows[row][col] = value
	g.HasCells = true
	if row > g.MaxRow {
		g.MaxRow = row
	}
	if col > g.MaxCol {
		g.MaxCol = col
	}
}

// Row tra ve cac o cua mot dong trong khoang cot [fromCol, toCol].
func (g *Grid) Row(row, fromCol, toCol int) []any {
	result := make([]any, 0, toCol-fromCol+1)
	for col := fromCol; col <= toCol; col++ {
		result = append(result, g.Get(row, col))
	}
	return result
}

// Book la workbook .xlsx/.xlsm dang mo.
type Book struct {
	Package      *ooxml.Package
	WorkbookPart string
	S            string
	Sheets       []*SheetRef

	date1904     bool
	shared       []string
	sharedIndex  map[string]int
	sharedLoaded bool
	dateStyles   map[int]bool
}

// Open doc workbook tu goi da mo.
func Open(pkg *ooxml.Package) (*Book, error) {
	main := pkg.RelOfType("", "officeDocument")
	if main == nil || !pkg.Exists(main.TargetPart) {
		return nil, fmt.Errorf("not an Excel workbook (no workbook part)")
	}
	book := &Book{Package: pkg, WorkbookPart: main.TargetPart, S: mainNS}
	doc, err := pkg.XML(book.WorkbookPart)
	if err != nil {
		return nil, err
	}
	if root := doc.Root(); root != nil && root.URI != "" {
		book.S = root.URI
	}
	if pr := doc.Root().ChildLocal("workbookPr"); pr != nil {
		book.date1904 = truthyAttribute(pr.AttrValue("", "date1904"))
	}
	book.LoadSheets()
	return book, nil
}

func (b *Book) Workbook() (*ooxml.Document, error) {
	return b.Package.XML(b.WorkbookPart)
}

func (b *Book) LoadSheets() {
	b.Sheets = nil
	rels := b.Package.Rels(b.WorkbookPart)
	doc, err := b.Workbook()
	if err != nil {
		return
	}
	sheets := doc.Root().Child(b.S, "sheets")
	if sheets == nil {
		return
	}
	for _, element := range sheets.Children(b.S, "sheet") {
		relID := element.AttrValue(relNS, "id")
		part := ""
		for _, rel := range rels {
			if rel.ID == relID {
				part = rel.TargetPart
				break
			}
		}
		sheetID, _ := strconv.Atoi(element.AttrValue("", "sheetId"))
		b.Sheets = append(b.Sheets, &SheetRef{
			Name:    element.AttrValue("", "name"),
			RelID:   relID,
			Part:    part,
			SheetID: sheetID,
			Element: element,
		})
	}
}

func (b *Book) SheetNames() []string {
	names := make([]string, 0, len(b.Sheets))
	for _, item := range b.Sheets {
		names = append(names, item.Name)
	}
	return names
}

// FindSheet tim sheet theo ten; loi neu do la chart sheet hoac thieu part.
func (b *Book) FindSheet(name string) (*SheetRef, error) {
	for _, item := range b.Sheets {
		if item.Name == name {
			if item.Part == "" || !b.Package.Exists(item.Part) {
				return nil, fmt.Errorf("sheet '%s' is not a worksheet (chart sheet or missing part)", name)
			}
			return item, nil
		}
	}
	return nil, fmt.Errorf("sheet not found: %s", name)
}

func (b *Book) HasSheet(name string) bool {
	for _, item := range b.Sheets {
		if strings.EqualFold(item.Name, name) {
			return true
		}
	}
	return false
}

// ActiveSheet lay sheet dang mo theo workbookView@activeTab (nhu wb.active cua openpyxl).
func (b *Book) ActiveSheet() (*SheetRef, error) {
	if len(b.Sheets) == 0 {
		return nil, fmt.Errorf("workbook has no sheets")
	}
	index := 0
	doc, err := b.Workbook()
	if err != nil {
		return nil, err
	}
	if views := doc.Root().Child(b.S, "bookViews"); views != nil {
		if view := views.Child(b.S, "workbookView"); view != nil {
			active := view.AttrValue("", "activeTab")
			if active == "" {
				active = "0"
			}
			index, _ = strconv.Atoi(active)
		}
	}
	index = max(0, min(index, len(b.Sheets)-1))
	return b.FindSheet(b.Sheets[index].Name)
}

// ---- doc ----

// Read doc sheet, chi giu o trong [rowFrom..rowTo] x [colFrom..colTo]. Kich thuoc lay tu
// <dimension> khi co (giong che do read-only cua openpyxl) va dung som khi da qua rowTo.
func (b *Book) Read(target *SheetRef, formulas bool, rowFrom, rowTo, colFrom, colTo int) (*Grid, error) {
	doc, err := b.Package.XML(target.Part)
	if err != nil {
		return nil, err
	}
	grid := NewGrid(target.Name)
	shared := map[string]sharedFormula{}
	dimensionKnown := false
	if dimension := doc.Root().ChildLocal("dimension"); dimension != nil {
		if bounds, err := sheet.ParseRange(dimension.AttrValue("", "ref")); err == nil {
			if bounds.MaxCol != nil && bounds.MaxRow != nil {
				// Rieng "A1" (Excel ghi cho sheet trong, vai thu vien ghi sai) thi van quet het
				// de khong cat mat du lieu.
				grid.MaxRow = *bounds.MaxRow
				grid.MaxCol = *bounds.MaxCol
				dimensionKnown = *bounds.MaxCol > 1 || *bounds.MaxRow > 1
			}
		}
	}

	data := doc.Root().ChildLocal("sheetData")
	if data == nil {
		return grid, nil
	}
	row := 0
	col := 0
	for _, rowElement := range data.ChildrenLocal("row") {
		if text := rowElement.AttrValue("", "r"); text != "" {
			row, _ = strconv.Atoi(text)
		} else {
			row++
		}
		col = 0
		if dimensionKnown && row > rowTo {
			break
		}
		for _, cell := range rowElement.ChildrenLocal("c") {
			if cell.HasAttr("", "r") {
				reference := cell.AttrValue("", "r")
				if reference != "" {
					if _, parsedCol, err := sheet.ParseCell(reference); err == nil {
						col = parsedCol
					}
				}
			} else {
				col++
			}
			wanted := row >= rowFrom && row <= rowTo && col >= colFrom && col <= colTo
			value := b.cellValue(cell, formulas, row, col, shared, wanted)
			grid.HasCells = true
			if row > grid.MaxRow {
				grid.MaxRow = row
			}
			if col > grid.MaxCol {
				grid.MaxCol = col
			}
			if wanted && value != nil {
				grid.Set(row, col, value)
			}
		}
	}
	return grid, nil
}

type sharedFormula struct {
	formula string
	row     int
	col     int
}

func (b *Book) cellValue(cell *ooxml.Element, formulas bool, row, col int, shared map[string]sharedFormula, wanted bool) any {
	if f := cell.ChildLocal("f"); formulas && f != nil {
		text := f.Text()
		formulaType := f.AttrValue("", "t")
		si := f.AttrValue("", "si")
		if formulaType == "shared" && si != "" {
			if len(text) > 0 {
				shared[si] = sharedFormula{formula: text, row: row, col: col}
			} else if master, ok := shared[si]; ok {
				text = ShiftFormula(master.formula, row-master.row, col-master.col)
			}
		}
		if len(text) > 0 {
			if wanted {
				return "=" + text
			}
			return nil
		}
	}
	if !wanted {
		return nil
	}
	value := cell.ChildLocal("v")
	switch cell.AttrValue("", "t") {
	case "s":
		if value == nil {
			return nil
		}
		index, err := strconv.Atoi(value.Text())
		if err != nil {
			return nil
		}
		strings := b.SharedStrings()
		if index >= 0 && index < len(strings) {
			return strings[index]
		}
		return nil
	case "inlineStr":
		inline := cell.ChildLocal("is")
		if inline == nil {
			return nil
		}
		return RichText(inline)
	case "str":
		if value == nil {
			return ""
		}
		return value.Text()
	case "b":
		if value == nil {
			return nil
		}
		return value.Text() == "1" || value.Text() == "true"
	case "e", "d":
		if value == nil {
			return nil
		}
		return value.Text()
	default:
		if value == nil || value.Text() == "" {
			return nil
		}
		raw := strings.TrimSpace(value.Text())
		number, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return raw
		}
		style, _ := strconv.Atoi(cell.AttrValue("", "s"))
		if b.DateStyles()[style] {
			return sheet.SerialToIso(number, b.date1904)
		}
		if !strings.ContainsAny(raw, ".Ee") {
			if integer, err := strconv.ParseInt(raw, 10, 64); err == nil {
				return integer
			}
		}
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return nil
		}
		return number
	}
}

// RichText noi cac <t> trong mot khoi rich text, bo phan phien am <rPh>.
func RichText(container *ooxml.Element) string {
	var builder strings.Builder
	var walk func(element *ooxml.Element, inPhonetic bool)
	walk = func(element *ooxml.Element, inPhonetic bool) {
		for _, kid := range element.Elements() {
			inside := inPhonetic || kid.Local == "rPh"
			if kid.Local == "t" && !inside {
				builder.WriteString(kid.Text())
			}
			walk(kid, inside)
		}
	}
	walk(container, false)
	return builder.String()
}

func (b *Book) SharedStringsPart() string {
	rel := b.Package.RelOfType(b.WorkbookPart, "sharedStrings")
	if rel != nil && b.Package.Exists(rel.TargetPart) {
		return rel.TargetPart
	}
	return ""
}

func (b *Book) SharedStrings() []string {
	if b.sharedLoaded {
		return b.shared
	}
	b.sharedLoaded = true
	b.shared = []string{}
	part := b.SharedStringsPart()
	if part == "" {
		return b.shared
	}
	doc, err := b.Package.XML(part)
	if err != nil || doc.Root() == nil {
		return b.shared
	}
	for _, si := range doc.Root().ChildrenLocal("si") {
		b.shared = append(b.shared, RichText(si))
	}
	return b.shared
}

func (b *Book) StylesPart() string {
	rel := b.Package.RelOfType(b.WorkbookPart, "styles")
	if rel != nil && b.Package.Exists(rel.TargetPart) {
		return rel.TargetPart
	}
	return ""
}

var builtinDateFormats = []int{14, 15, 16, 17, 18, 19, 20, 21, 22, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 45, 46, 47, 50, 51, 52, 53, 54, 55, 56, 57, 58}

// DateStyles tra ve tap chi so style duoc dinh dang ngay thang.
func (b *Book) DateStyles() map[int]bool {
	if b.dateStyles != nil {
		return b.dateStyles
	}
	b.dateStyles = map[int]bool{}
	part := b.StylesPart()
	if part == "" {
		return b.dateStyles
	}
	doc, err := b.Package.XML(part)
	if err != nil || doc.Root() == nil {
		return b.dateStyles
	}
	custom := map[int]string{}
	if numFmts := doc.Root().Child(b.S, "numFmts"); numFmts != nil {
		for _, format := range numFmts.Children(b.S, "numFmt") {
			id, err := strconv.Atoi(format.AttrValue("", "numFmtId"))
			if err == nil {
				custom[id] = format.AttrValue("", "formatCode")
			}
		}
	}
	cellXfs := doc.Root().Child(b.S, "cellXfs")
	if cellXfs == nil {
		return b.dateStyles
	}
	for index, xf := range cellXfs.Children(b.S, "xf") {
		formatID, _ := strconv.Atoi(xf.AttrValue("", "numFmtId"))
		code, isCustom := custom[formatID]
		if isCustom {
			if IsDateFormat(code) {
				b.dateStyles[index] = true
			}
			continue
		}
		if slices.Contains(builtinDateFormats, formatID) {
			b.dateStyles[index] = true
		}
	}
	return b.dateStyles
}

var (
	dateFormatStrip = regexp.MustCompile(`"[^"]*"|\[[^\]]*\]|\\.|_.|\*.`)
	dateFormatChars = regexp.MustCompile(`[dmyhsDMYHS]`)
)

// IsDateFormat theo cach cua openpyxl: bo chuoi "...", [..] va ky tu escape roi tim d/m/y/h/s.
func IsDateFormat(code string) bool {
	if code == "" {
		return false
	}
	first, _, _ := strings.Cut(code, ";")
	stripped := dateFormatStrip.ReplaceAllString(first, "")
	if strings.EqualFold(stripped, "General") {
		return false
	}
	return dateFormatChars.MatchString(stripped)
}

func truthyAttribute(value string) bool {
	return value == "1" || value == "true"
}

// ---- sua ----

func (b *Book) SheetDoc(target *SheetRef) (*ooxml.Document, error) {
	return b.Package.XML(target.Part)
}

// SheetData lay (hoac tao) phan tu <sheetData> dung cho viec ghi.
func (b *Book) SheetData(doc *ooxml.Document) *ooxml.Element {
	data := doc.Root().ChildLocal("sheetData")
	if data == nil {
		data = child(doc.Root(), b.S, "sheetData")
	}
	return data
}

// SetValue ghi gia tri nhu openpyxl: chuoi bat dau '=' la cong thuc, nil xoa gia tri (giu style).
func (b *Book) SetValue(data *ooxml.Element, row, col int, value any) error {
	cell := b.GetOrCreateCell(data, row, col)
	cell.RemoveChildrenLocal("f")
	cell.RemoveChildrenLocal("v")
	cell.RemoveChildrenLocal("is")
	cell.RemoveAttr("", "t")
	cell.RemoveAttr("", "vm")
	cell.RemoveAttr("", "cm")
	if value == nil {
		return nil
	}
	if text, ok := value.(string); ok {
		if strings.HasPrefix(text, "=") && len(text) > 1 {
			child(cell, b.S, "f").SetText(text[1:])
		} else {
			cell.SetAttr("", "", "t", "s")
			child(cell, b.S, "v").SetText(strconv.Itoa(b.SharedStringIndex(text)))
		}
		return nil
	}
	if boolean, ok := value.(bool); ok {
		cell.SetAttr("", "", "t", "b")
		if boolean {
			child(cell, b.S, "v").SetText("1")
		} else {
			child(cell, b.S, "v").SetText("0")
		}
		return nil
	}
	if sheet.IsNumber(value) {
		number, _ := sheet.AsFloat(value)
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return nil
		}
		child(cell, b.S, "v").SetText(sheet.ToText(value))
		return nil
	}
	return fmt.Errorf("unsupported cell value at %s: use string, number, boolean or null", sheet.Address(row, col))
}

// GetOrCreateCell tim o theo dia chi, tao moi dung cho neu chua co (giu thu tu cot).
func (b *Book) GetOrCreateCell(data *ooxml.Element, row, col int) *ooxml.Element {
	rowElement := b.GetOrCreateRow(data, row)
	var before *ooxml.Element
	expected := 0
	for _, cell := range rowElement.ChildrenLocal("c") {
		if cell.HasAttr("", "r") {
			if _, parsedCol, err := sheet.ParseCell(cell.AttrValue("", "r")); err == nil {
				expected = parsedCol
			}
		} else {
			expected++
			cell.SetAttr("", "", "r", sheet.Address(row, expected))
		}
		if expected == col {
			return cell
		}
		if expected > col {
			before = cell
			break
		}
	}
	// Tao roi (chua gan vao cay): neu Append truoc roi InsertAt nua thi o se bi nhan doi.
	cell := elementIn(b.S, "c", "r", sheet.Address(row, col))
	if style := rowDefaultStyle(rowElement); style > 0 {
		cell.SetAttr("", "", "s", strconv.Itoa(style))
	}
	ext := rowElement.Child(b.S, "extLst")
	switch {
	case before != nil:
		rowElement.InsertAt(rowElement.ChildIndex(before), cell)
	case ext != nil:
		rowElement.InsertAt(rowElement.ChildIndex(ext), cell)
	default:
		rowElement.Append(cell)
	}
	return cell
}

func rowDefaultStyle(row *ooxml.Element) int {
	if !truthyAttribute(row.AttrValue("", "customFormat")) {
		return 0
	}
	style, err := strconv.Atoi(row.AttrValue("", "s"))
	if err != nil {
		return 0
	}
	return style
}

func (b *Book) GetOrCreateRow(data *ooxml.Element, row int) *ooxml.Element {
	expected := 0
	for _, existing := range data.ChildrenLocal("row") {
		index, err := strconv.Atoi(existing.AttrValue("", "r"))
		if err != nil {
			index = expected + 1
			existing.SetAttr("", "", "r", strconv.Itoa(index))
		}
		expected = index
		if index == row {
			existing.RemoveAttr("", "spans")
			return existing
		}
		if index > row {
			created := elementIn(b.S, "row", "r", strconv.Itoa(row))
			data.InsertAt(data.ChildIndex(existing), created)
			return created
		}
	}
	appended := elementIn(b.S, "row", "r", strconv.Itoa(row))
	data.Append(appended)
	return appended
}

func (b *Book) CellText(data *ooxml.Element, row, col int) string {
	cell := b.GetOrCreateCell(data, row, col)
	value := cell.Child(b.S, "v")
	switch cell.AttrValue("", "t") {
	case "s":
		if value == nil {
			return ""
		}
		index, err := strconv.Atoi(value.Text())
		if err != nil {
			return ""
		}
		strings := b.SharedStrings()
		if index < len(strings) {
			return strings[index]
		}
		return ""
	case "inlineStr":
		inline := cell.Child(b.S, "is")
		if inline == nil {
			return ""
		}
		return RichText(inline)
	}
	if value == nil {
		return ""
	}
	return value.Text()
}

// SharedStringIndex tra ve chi so cua chuoi trong sharedStrings, them moi neu chua co.
func (b *Book) SharedStringIndex(text string) int {
	strings := b.SharedStrings()
	if b.sharedIndex == nil {
		b.sharedIndex = map[string]int{}
		for index, value := range strings {
			if _, exists := b.sharedIndex[value]; !exists {
				b.sharedIndex[value] = index
			}
		}
	}
	if existing, ok := b.sharedIndex[text]; ok {
		return existing
	}
	part := b.SharedStringsPart()
	var doc *ooxml.Document
	if part == "" {
		part = "xl/sharedStrings.xml"
		doc = ooxml.NewDocument(ooxml.NewRoot(b.S, "sst"))
		b.Package.AddRel(b.WorkbookPart, "sharedStrings", part)
		if err := b.Package.SetContentTypeOverride(part, sharedStringsContentType); err != nil {
			return 0
		}
	} else {
		existing, err := b.Package.XML(part)
		if err != nil {
			return 0
		}
		doc = existing
	}
	entry := child(doc.Root(), b.S, "si")
	textElement := child(entry, b.S, "t")
	if len(text) > 0 && (isSpace(text[0]) || isSpace(text[len(text)-1])) {
		textElement.SetAttr(ooxml.XMLNamespace, "xml", "space", "preserve")
	}
	textElement.SetText(text)

	b.shared = append(strings, text)
	index := len(b.shared) - 1
	b.sharedIndex[text] = index
	doc.Root().SetAttr("", "", "uniqueCount", strconv.Itoa(len(b.shared)))
	count, _ := strconv.Atoi(doc.Root().AttrValue("", "count"))
	doc.Root().SetAttr("", "", "count", strconv.Itoa(max(count+1, len(b.shared))))
	b.Package.Put(part, doc)
	return index
}

func isSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}

// UpdateDimension cap nhat <dimension> theo cac o thuc co.
func (b *Book) UpdateDimension(doc *ooxml.Document) {
	data := b.SheetData(doc)
	minRow, minCol := math.MaxInt32, math.MaxInt32
	maxRow, maxCol := 0, 0
	for _, row := range data.ChildrenLocal("row") {
		for _, cell := range row.ChildrenLocal("c") {
			reference := cell.AttrValue("", "r")
			if reference == "" {
				continue
			}
			rowIndex, colIndex, err := sheet.ParseCell(reference)
			if err != nil {
				continue
			}
			minRow = min(minRow, rowIndex)
			minCol = min(minCol, colIndex)
			maxRow = max(maxRow, rowIndex)
			maxCol = max(maxCol, colIndex)
		}
	}
	reference := "A1"
	switch {
	case maxRow == 0:
	case minRow == maxRow && minCol == maxCol:
		reference = sheet.Address(minRow, minCol)
	default:
		reference = sheet.Address(minRow, minCol) + ":" + sheet.Address(maxRow, maxCol)
	}
	dimension := doc.Root().ChildLocal("dimension")
	if dimension == nil {
		dimension = ooxml.NewElement(b.S, "", "dimension")
		InsertInOrder(doc.Root(), dimension, WorksheetOrder)
	}
	dimension.SetAttr("", "", "ref", reference)
}

// InvalidateCalculation bo calcChain va yeu cau Excel tinh lai khi mo.
func (b *Book) InvalidateCalculation() error {
	calcChain := b.Package.RelOfType(b.WorkbookPart, "calcChain")
	if calcChain != nil {
		b.Package.RemoveRel(b.WorkbookPart, calcChain.ID)
		if b.Package.Exists(calcChain.TargetPart) {
			b.Package.Delete(calcChain.TargetPart)
		}
	}
	doc, err := b.Workbook()
	if err != nil {
		return err
	}
	calcPr := doc.Root().Child(b.S, "calcPr")
	if calcPr == nil {
		calcPr = ooxml.NewElement(b.S, "", "calcPr")
		InsertInOrder(doc.Root(), calcPr, WorkbookOrder)
	}
	calcPr.SetAttr("", "", "fullCalcOnLoad", "1")
	b.Package.Put(b.WorkbookPart, doc)
	return nil
}

func (b *Book) SaveSheet(target *SheetRef, doc *ooxml.Document) {
	b.Package.Put(target.Part, doc)
}

// Styles mo part styles.xml, tao moi neu workbook chua co.
func (b *Book) Styles() (*Styles, error) {
	part := b.StylesPart()
	if part == "" {
		part = "xl/styles.xml"
		b.Package.Put(part, StylesXML(b.S))
		b.Package.AddRel(b.WorkbookPart, "styles", part)
		if err := b.Package.SetContentTypeOverride(part, stylesContentType); err != nil {
			return nil, err
		}
	}
	return NewStyles(b.Package, part, b.S)
}

// WorksheetOrder la thu tu phan tu con cua <worksheet> theo schema.
var WorksheetOrder = []string{
	"sheetPr", "dimension", "sheetViews", "sheetFormatPr", "cols", "sheetData", "sheetCalcPr", "sheetProtection", "protectedRanges",
	"scenarios", "autoFilter", "sortState", "dataConsolidate", "customSheetViews", "mergeCells", "phoneticPr", "conditionalFormatting",
	"dataValidations", "hyperlinks", "printOptions", "pageMargins", "pageSetup", "headerFooter", "rowBreaks", "colBreaks",
	"customProperties", "cellWatches", "ignoredErrors", "smartTags", "drawing", "legacyDrawing", "legacyDrawingHF", "drawingHF",
	"picture", "oleObjects", "controls", "webPublishItems", "tableParts", "extLst",
}

// WorkbookOrder la thu tu phan tu con cua <workbook> theo schema.
var WorkbookOrder = []string{
	"fileVersion", "fileSharing", "workbookPr", "workbookProtection", "bookViews", "sheets", "functionGroups", "externalReferences",
	"definedNames", "calcPr", "oleSize", "customWorkbookViews", "pivotCaches", "smartTagPr", "smartTagTypes", "webPublishing",
	"fileRecoveryPr", "webPublishObjects", "extLst",
}

// InsertInOrder chen phan tu vao dung vi tri theo schema (bang thu tu ten phan tu).
func InsertInOrder(parent, element *ooxml.Element, order []string) {
	rank := indexOf(order, element.Local)
	for _, existing := range parent.Elements() {
		if indexOf(order, existing.Local) > rank {
			parent.InsertAt(parent.ChildIndex(existing), element)
			return
		}
	}
	parent.Append(element)
}

func indexOf(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
}
