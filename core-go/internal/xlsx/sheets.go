package xlsx

import (
	"fmt"
	"strconv"
	"strings"

	"axiomoffice/core/internal/ooxml"
	"axiomoffice/core/internal/sheet"
)

// ValidateSheetName kiem tra ten sheet theo gioi han cua Excel.
func ValidateSheetName(name string) error {
	if name == "" || len([]rune(name)) > 31 || strings.ContainsAny(name, `:\/?*[]`) ||
		strings.HasPrefix(name, "'") || strings.HasSuffix(name, "'") {
		return fmt.Errorf("invalid sheet name '%s' (1-31 chars, no : \\ / ? * [ ] and no leading/trailing apostrophe)", name)
	}
	return nil
}

// AddSheet them sheet moi; content nil thi tao worksheet trong.
func (b *Book) AddSheet(name string, content *ooxml.Document) (*SheetRef, error) {
	if err := ValidateSheetName(name); err != nil {
		return nil, err
	}
	if b.HasSheet(name) {
		return nil, fmt.Errorf("sheet already exists: %s", name)
	}
	part := b.Package.NextPartName("xl/worksheets/sheet%d.xml")
	doc := content
	if doc == nil {
		root := ooxml.NewRoot(b.S, "worksheet")
		root.DeclareNamespace("r", relNS)
		child(root, b.S, "sheetData")
		doc = ooxml.NewDocument(root)
	}
	b.Package.Put(part, doc)
	if err := b.Package.SetContentTypeOverride(part, WorksheetContentType); err != nil {
		return nil, err
	}
	relID := b.Package.AddRel(b.WorkbookPart, "worksheet", part)

	workbook, err := b.Workbook()
	if err != nil {
		return nil, err
	}
	sheets := workbook.Root().Child(b.S, "sheets")
	if sheets == nil {
		sheets = ooxml.NewElement(b.S, "", "sheets")
		InsertInOrder(workbook.Root(), sheets, WorkbookOrder)
	}
	sheetID := 0
	for _, item := range b.Sheets {
		if item.SheetID > sheetID {
			sheetID = item.SheetID
		}
	}
	sheetID++
	entry := child(sheets, b.S, "sheet", "name", name, "sheetId", strconv.Itoa(sheetID))
	entry.SetAttr(relNS, "r", "id", relID)
	b.Package.Put(b.WorkbookPart, workbook)
	b.LoadSheets()
	return b.FindSheet(name)
}

// DeleteSheet xoa sheet: quan he, part, ten dinh nghia cuc bo va chi so bookView.
func (b *Book) DeleteSheet(target *SheetRef) error {
	index := -1
	for position, item := range b.Sheets {
		if item == target {
			index = position
			break
		}
	}
	workbook, err := b.Workbook()
	if err != nil {
		return err
	}
	if parent := findParent(workbook.Root(), target.Element); parent != nil {
		parent.RemoveChild(target.Element)
	}
	b.Package.RemoveRel(b.WorkbookPart, target.RelID)
	if target.Part != "" && b.Package.Exists(target.Part) {
		b.Package.Delete(target.Part)
	}
	if names := workbook.Root().Child(b.S, "definedNames"); names != nil {
		for _, defined := range names.Children(b.S, "definedName") {
			local, err := strconv.Atoi(defined.AttrValue("", "localSheetId"))
			if err != nil {
				continue
			}
			switch {
			case local == index:
				names.RemoveChild(defined)
			case local > index:
				defined.SetAttr("", "", "localSheetId", strconv.Itoa(local-1))
			}
		}
		if len(names.Elements()) == 0 {
			workbook.Root().RemoveChild(names)
		}
	}
	remaining := len(b.Sheets) - 1
	if views := workbook.Root().Child(b.S, "bookViews"); views != nil {
		for _, view := range views.Children(b.S, "workbookView") {
			for _, attribute := range []string{"activeTab", "firstSheet"} {
				value, err := strconv.Atoi(view.AttrValue("", attribute))
				if err == nil && value >= remaining {
					view.SetAttr("", "", attribute, strconv.Itoa(max(0, remaining-1)))
				}
			}
		}
	}
	b.Package.Put(b.WorkbookPart, workbook)
	b.LoadSheets()
	return nil
}

// RenameSheet doi ten sheet; ten dinh nghia tro toi sheet cung duoc cap nhat.
func (b *Book) RenameSheet(target *SheetRef, newName string) error {
	if err := ValidateSheetName(newName); err != nil {
		return err
	}
	for _, item := range b.Sheets {
		if item != target && strings.EqualFold(item.Name, newName) {
			return fmt.Errorf("sheet already exists: %s", newName)
		}
	}
	workbook, err := b.Workbook()
	if err != nil {
		return err
	}
	oldName := target.Name
	target.Element.SetAttr("", "", "name", newName)
	if names := workbook.Root().Child(b.S, "definedNames"); names != nil {
		for _, defined := range names.Children(b.S, "definedName") {
			defined.SetText(ReplaceSheetReference(defined.Text(), oldName, newName))
		}
	}
	b.Package.Put(b.WorkbookPart, workbook)
	b.LoadSheets()
	return nil
}

// CopySheet sao chep nhu copy_worksheet cua openpyxl: giu o/style/dinh dang, bo drawing/table/anh.
func (b *Book) CopySheet(source *SheetRef, newName string) (*SheetRef, error) {
	doc, err := b.SheetDoc(source)
	if err != nil {
		return nil, err
	}
	copyRoot := doc.Root().Clone()
	dropped := map[string]bool{
		"drawing": true, "legacyDrawing": true, "legacyDrawingHF": true, "drawingHF": true,
		"picture": true, "oleObjects": true, "controls": true, "tableParts": true, "webPublishItems": true,
	}
	for _, element := range copyRoot.Elements() {
		if dropped[element.Local] {
			copyRoot.RemoveChild(element)
		}
	}
	if hyperlinks := copyRoot.Child(b.S, "hyperlinks"); hyperlinks != nil {
		for _, link := range hyperlinks.Elements() {
			if link.HasAttr(relNS, "id") {
				hyperlinks.RemoveChild(link)
			}
		}
		if len(hyperlinks.Elements()) == 0 {
			copyRoot.RemoveChild(hyperlinks)
		}
	}
	for _, view := range descendants(copyRoot, "sheetView") {
		view.RemoveAttr("", "tabSelected")
	}
	// codeName cua VBA phai duy nhat trong .xlsm; de Excel tu dat cho sheet moi.
	if sheetPr := copyRoot.Child(b.S, "sheetPr"); sheetPr != nil {
		sheetPr.RemoveAttr("", "codeName")
	}
	return b.AddSheet(newName, ooxml.NewDocument(copyRoot))
}

// ---- table (ListObject) ----

// AllTableNames liet ke ten moi bang trong workbook.
func (b *Book) AllTableNames() []string {
	names := []string{}
	for _, part := range b.Package.PartNames() {
		lower := strings.ToLower(part)
		if !strings.HasPrefix(lower, "xl/tables/") || !strings.HasSuffix(lower, ".xml") {
			continue
		}
		doc, err := b.Package.XML(part)
		if err != nil || doc.Root() == nil {
			continue
		}
		names = append(names, tableName(doc.Root()))
	}
	return names
}

// SheetTableNames liet ke ten bang tren mot sheet.
func (b *Book) SheetTableNames(target *SheetRef) []string {
	names := []string{}
	for _, rel := range b.Package.Rels(target.Part) {
		if !strings.HasSuffix(rel.Type, "/table") || !b.Package.Exists(rel.TargetPart) {
			continue
		}
		doc, err := b.Package.XML(rel.TargetPart)
		if err != nil || doc.Root() == nil {
			continue
		}
		names = append(names, tableName(doc.Root()))
	}
	return names
}

func tableName(root *ooxml.Element) string {
	if value := root.AttrValue("", "displayName"); value != "" {
		return value
	}
	return root.AttrValue("", "name")
}

// AddTable tao ListObject tren vung da co san du lieu.
func (b *Book) AddTable(target *SheetRef, cellRange, name string) error {
	bounds, err := sheet.ParseBoundedRange(cellRange)
	if err != nil {
		return err
	}
	if bounds.MaxRow <= bounds.MinRow {
		return fmt.Errorf("table range needs a header row and at least one data row")
	}
	doc, err := b.SheetDoc(target)
	if err != nil {
		return err
	}
	data := b.SheetData(doc)

	// Excel yeu cau o tieu de la chu, khong trung, va khop ten cot cua bang.
	columns := []string{}
	used := map[string]bool{}
	for col := bounds.MinCol; col <= bounds.MaxCol; col++ {
		header := strings.TrimSpace(b.CellText(data, bounds.MinRow, col))
		if header == "" {
			header = "Column" + strconv.Itoa(col-bounds.MinCol+1)
		}
		unique := header
		for suffix := 2; used[strings.ToLower(unique)]; suffix++ {
			unique = header + strconv.Itoa(suffix)
		}
		used[strings.ToLower(unique)] = true
		columns = append(columns, unique)
		if err := b.SetValue(data, bounds.MinRow, col, unique); err != nil {
			return err
		}
	}
	b.UpdateDimension(doc)

	tableID := 1
	for _, part := range b.Package.PartNames() {
		lower := strings.ToLower(part)
		if !strings.HasPrefix(lower, "xl/tables/") || !strings.HasSuffix(lower, ".xml") {
			continue
		}
		tableDoc, err := b.Package.XML(part)
		if err != nil || tableDoc.Root() == nil {
			continue
		}
		if id, err := strconv.Atoi(tableDoc.Root().AttrValue("", "id")); err == nil {
			tableID = max(tableID, id+1)
		}
	}
	reference := sheet.Address(bounds.MinRow, bounds.MinCol) + ":" + sheet.Address(bounds.MaxRow, bounds.MaxCol)

	tableRoot := ooxml.NewRoot(b.S, "table")
	tableRoot.SetAttr("", "", "id", strconv.Itoa(tableID))
	tableRoot.SetAttr("", "", "name", name)
	tableRoot.SetAttr("", "", "displayName", name)
	tableRoot.SetAttr("", "", "ref", reference)
	tableRoot.SetAttr("", "", "totalsRowShown", "0")
	child(tableRoot, b.S, "autoFilter", "ref", reference)
	tableColumns := child(tableRoot, b.S, "tableColumns", "count", strconv.Itoa(len(columns)))
	for index, columnName := range columns {
		child(tableColumns, b.S, "tableColumn", "id", strconv.Itoa(index+1), "name", columnName)
	}
	child(tableRoot, b.S, "tableStyleInfo",
		"name", "TableStyleMedium9", "showFirstColumn", "0", "showLastColumn", "0",
		"showRowStripes", "1", "showColumnStripes", "0")

	tablePart := b.Package.NextPartName("xl/tables/table%d.xml")
	b.Package.Put(tablePart, ooxml.NewDocument(tableRoot))
	if err := b.Package.SetContentTypeOverride(tablePart, tableContentType); err != nil {
		return err
	}
	relID := b.Package.AddRel(target.Part, "table", tablePart)

	tableParts := doc.Root().Child(b.S, "tableParts")
	if tableParts == nil {
		tableParts = ooxml.NewElement(b.S, "", "tableParts")
		InsertInOrder(doc.Root(), tableParts, WorksheetOrder)
	}
	entry := child(tableParts, b.S, "tablePart")
	entry.SetAttr(relNS, "r", "id", relID)
	tableParts.SetAttr("", "", "count", strconv.Itoa(len(tableParts.Children(b.S, "tablePart"))))
	// tablePart vua dung r:id nen phai chac chan tien to r da duoc khai bao o goc.
	if !doc.Root().HasPrefix("r") {
		doc.Root().DeclareNamespace("r", relNS)
	}
	b.SaveSheet(target, doc)
	return nil
}

// findParent tim phan tu cha truc tiep cua node trong cay.
func findParent(root *ooxml.Element, node *ooxml.Element) *ooxml.Element {
	for _, kid := range root.Elements() {
		if kid == node {
			return root
		}
		if parent := findParent(kid, node); parent != nil {
			return parent
		}
	}
	return nil
}

// descendants liet ke moi phan tu con (moi cap) co ten cuc bo cho truoc.
func descendants(root *ooxml.Element, local string) []*ooxml.Element {
	found := []*ooxml.Element{}
	var walk func(element *ooxml.Element)
	walk = func(element *ooxml.Element) {
		for _, kid := range element.Elements() {
			if kid.Local == local {
				found = append(found, kid)
			}
			walk(kid)
		}
	}
	walk(root)
	return found
}
