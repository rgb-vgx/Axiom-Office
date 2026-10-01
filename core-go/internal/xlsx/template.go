// Package xlsx: doc/ghi workbook .xlsx/.xlsm. Port cua XlsxBook.cs + XlsxStyles.cs.
package xlsx

import (
	"axiomoffice/core/internal/ooxml"
)

// Content type cua cac part trong workbook.
const (
	WorksheetContentType     = "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"
	sharedStringsContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"
	tableContentType         = "application/vnd.openxmlformats-officedocument.spreadsheetml.table+xml"
	stylesContentType        = "application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"
	workbookContentType      = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
	macroWorkbookContentType = "application/vnd.ms-excel.sheet.macroEnabled.main+xml"
	templateContentType      = "application/vnd.openxmlformats-officedocument.spreadsheetml.template.main+xml"
	macroTemplateContentType = "application/vnd.ms-excel.template.macroEnabled.main+xml"
)

// WorkbookPart mac dinh cua workbook moi.
const defaultWorkbookPart = "xl/workbook.xml"

const (
	mainNS = ooxml.SpreadsheetNS
	relNS  = ooxml.OfficeRelNamespace
)

// elementIn tao phan tu roi (chua gan vao cay) voi thuoc tinh khong tien to.
func elementIn(uri, local string, pairs ...string) *ooxml.Element {
	element := ooxml.NewElement(uri, "", local)
	for index := 0; index+1 < len(pairs); index += 2 {
		element.SetAttr("", "", pairs[index], pairs[index+1])
	}
	return element
}

// child tao phan tu con voi cac thuoc tinh khong tien to, theo cap "ten", "gia tri".
func child(parent *ooxml.Element, uri, local string, pairs ...string) *ooxml.Element {
	element := elementIn(uri, local, pairs...)
	parent.Append(element)
	return element
}

// document boc phan tu goc thanh tai lieu XML hoan chinh.
func document(root *ooxml.Element) *ooxml.Document {
	return ooxml.NewDocument(root)
}

// StylesXML dung styles.xml toi thieu thay cho openpyxl.Workbook().
func StylesXML(uri string) *ooxml.Document {
	root := ooxml.NewRoot(uri, "styleSheet")

	fonts := child(root, mainNS, "fonts", "count", "1")
	font := child(fonts, mainNS, "font")
	child(font, mainNS, "sz", "val", "11")
	child(font, mainNS, "color", "theme", "1")
	child(font, mainNS, "name", "val", "Calibri")
	child(font, mainNS, "family", "val", "2")
	child(font, mainNS, "scheme", "val", "minor")

	fills := child(root, mainNS, "fills", "count", "2")
	child(child(fills, mainNS, "fill"), mainNS, "patternFill", "patternType", "none")
	child(child(fills, mainNS, "fill"), mainNS, "patternFill", "patternType", "gray125")

	borders := child(root, mainNS, "borders", "count", "1")
	border := child(borders, mainNS, "border")
	for _, side := range []string{"left", "right", "top", "bottom", "diagonal"} {
		child(border, mainNS, side)
	}

	cellStyleXfs := child(root, mainNS, "cellStyleXfs", "count", "1")
	child(cellStyleXfs, mainNS, "xf", "numFmtId", "0", "fontId", "0", "fillId", "0", "borderId", "0")

	cellXfs := child(root, mainNS, "cellXfs", "count", "1")
	child(cellXfs, mainNS, "xf", "numFmtId", "0", "fontId", "0", "fillId", "0", "borderId", "0", "xfId", "0")

	cellStyles := child(root, mainNS, "cellStyles", "count", "1")
	child(cellStyles, mainNS, "cellStyle", "name", "Normal", "xfId", "0", "builtinId", "0")

	return document(root)
}

// BuildTemplate dong goi mot workbook trong co dung mot sheet ten firstSheet.
// Kieu noi dung cua part chinh doi theo duoi file (.xlsm/.xltx/.xltm).
func BuildTemplate(extension, firstSheet string) ([]byte, error) {
	mainType := workbookContentType
	switch extension {
	case ".xlsm":
		mainType = macroWorkbookContentType
	case ".xltx":
		mainType = templateContentType
	case ".xltm":
		mainType = macroTemplateContentType
	}

	pkg := ooxml.NewPackage()

	// [Content_Types].xml
	typesRoot := ooxml.NewRoot(ooxml.ContentTypeNS, "Types")
	child(typesRoot, ooxml.ContentTypeNS, "Default", "Extension", "rels", "ContentType", "application/vnd.openxmlformats-package.relationships+xml")
	child(typesRoot, ooxml.ContentTypeNS, "Default", "Extension", "xml", "ContentType", "application/xml")
	child(typesRoot, ooxml.ContentTypeNS, "Override", "PartName", "/xl/workbook.xml", "ContentType", mainType)
	child(typesRoot, ooxml.ContentTypeNS, "Override", "PartName", "/xl/worksheets/sheet1.xml", "ContentType", WorksheetContentType)
	child(typesRoot, ooxml.ContentTypeNS, "Override", "PartName", "/xl/styles.xml", "ContentType", stylesContentType)
	pkg.Put(ooxml.ContentTypesPart, document(typesRoot))

	// _rels/.rels
	rootRels := ooxml.NewRoot(ooxml.RelNamespace, "Relationships")
	child(rootRels, ooxml.RelNamespace, "Relationship",
		"Id", "rId1", "Type", ooxml.RelTypeBase+"officeDocument", "Target", "xl/workbook.xml")
	pkg.Put(ooxml.RootRelsPart, document(rootRels))

	// xl/workbook.xml
	workbookRoot := ooxml.NewRoot(mainNS, "workbook")
	workbookRoot.DeclareNamespace("r", relNS)
	views := child(workbookRoot, mainNS, "bookViews")
	child(views, mainNS, "workbookView", "activeTab", "0")
	sheets := child(workbookRoot, mainNS, "sheets")
	sheet := child(sheets, mainNS, "sheet", "name", firstSheet, "sheetId", "1")
	sheet.SetAttr(relNS, "r", "id", "rId1")
	child(workbookRoot, mainNS, "calcPr", "fullCalcOnLoad", "1")
	pkg.Put(defaultWorkbookPart, document(workbookRoot))

	// xl/_rels/workbook.xml.rels
	workbookRelsRoot := ooxml.NewRoot(ooxml.RelNamespace, "Relationships")
	child(workbookRelsRoot, ooxml.RelNamespace, "Relationship",
		"Id", "rId1", "Type", ooxml.RelTypeBase+"worksheet", "Target", "worksheets/sheet1.xml")
	child(workbookRelsRoot, ooxml.RelNamespace, "Relationship",
		"Id", "rId2", "Type", ooxml.RelTypeBase+"styles", "Target", "styles.xml")
	pkg.Put("xl/_rels/workbook.xml.rels", document(workbookRelsRoot))

	// xl/worksheets/sheet1.xml
	sheetRoot := ooxml.NewRoot(mainNS, "worksheet")
	sheetRoot.DeclareNamespace("r", relNS)
	sheetViews := child(sheetRoot, mainNS, "sheetViews")
	child(sheetViews, mainNS, "sheetView", "tabSelected", "1", "workbookViewId", "0")
	child(sheetRoot, mainNS, "sheetData")
	pkg.Put("xl/worksheets/sheet1.xml", document(sheetRoot))

	// xl/styles.xml
	pkg.Put("xl/styles.xml", StylesXML(mainNS))

	return pkg.Archive()
}
