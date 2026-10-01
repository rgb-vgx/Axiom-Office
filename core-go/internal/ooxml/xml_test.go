package ooxml

import (
	"strings"
	"testing"
)

const workbookSample = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Dữ liệu &amp; hơn" sheetId="1" r:id="rId1"/></sheets><fileVersion appName="xl"/></workbook>`

func parse(t *testing.T, source string) *Document {
	t.Helper()
	document, err := ParseXML([]byte(source))
	if err != nil {
		t.Fatalf("parse loi: %v", err)
	}
	return document
}

func TestParseResolvesNamespacesAndKeepsPrefixes(t *testing.T) {
	document := parse(t, workbookSample)
	root := document.Root()
	if root == nil {
		t.Fatal("thieu phan tu goc")
	}
	if root.Local != "workbook" || root.Prefix != "" {
		t.Fatalf("goc sai: prefix=%q local=%q", root.Prefix, root.Local)
	}
	if root.URI != SpreadsheetNS {
		t.Fatalf("namespace mac dinh phai duoc phan giai: %q", root.URI)
	}
	sheet := root.Child(SpreadsheetNS, "sheets").Child(SpreadsheetNS, "sheet")
	if sheet == nil {
		t.Fatal("khong tim thay sheet")
	}
	// Thuoc tinh co tien to dung namespace cua quan he, khong phai namespace mac dinh.
	if got := sheet.AttrValue(OfficeRelNamespace, "id"); got != "rId1" {
		t.Fatalf("r:id sai: %q", got)
	}
	// Thuoc tinh khong tien to thuoc namespace rong.
	if got := sheet.AttrValue("", "name"); got != "Dữ liệu & hơn" {
		t.Fatalf("name sai (entity phai duoc giai): %q", got)
	}
}

// Ghi lai giu nguyen tung byte, tru mot cho: xuong dong ngay sau khai bao XML bi bo. XDocument
// cua ban C# cung vay (khong bieu dien duoc text o cap tai lieu), nen ban Go phai khop hanh vi do.
func TestSerializeRoundTripsByteForByte(t *testing.T) {
	document := parse(t, workbookSample)
	rendered := string(document.Serialize())
	expected := strings.Replace(workbookSample, "?>\n", "?>", 1)
	if rendered != expected {
		t.Fatalf("ghi lai khong giu nguyen:\n got: %s\nwant: %s", rendered, expected)
	}
	// Ghi lai lan hai phai on dinh (khong doi them).
	again, err := ParseXML([]byte(rendered))
	if err != nil {
		t.Fatalf("doc lai loi: %v", err)
	}
	if string(again.Serialize()) != rendered {
		t.Fatal("ghi lai lan hai khac lan dau")
	}
}

func TestEditKeepsPrefixesAndWhitespace(t *testing.T) {
	document := parse(t, workbookSample)
	sheets := document.Root().Child(SpreadsheetNS, "sheets")
	added := sheets.AppendElement(SpreadsheetNS, "", "sheet")
	added.SetAttr("", "", "name", "Trang 2")
	added.SetAttr("", "", "sheetId", "2")
	added.SetAttr(OfficeRelNamespace, "r", "id", "rId2")

	rendered := string(document.Serialize())
	if !strings.Contains(rendered, `<sheet name="Trang 2" sheetId="2" r:id="rId2"/>`) {
		t.Fatalf("phan tu moi sai: %s", rendered)
	}
	if !strings.Contains(rendered, `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`) {
		t.Fatalf("khai bao xmlns:r bi mat: %s", rendered)
	}
	// Khong sinh them xmlns o phan tu con (tien to da co hieu luc tu goc).
	if strings.Count(rendered, "xmlns") != 2 {
		t.Fatalf("so khai bao xmlns thay doi: %s", rendered)
	}
}

func TestEscapeTextAndAttributes(t *testing.T) {
	element := NewElement("", "w", "t")
	element.SetAttr("", "", "val", "dấu \" và < > &")
	element.SetText("a < b & c > d")
	rendered := string(NewDocument(element).Serialize())
	if !strings.Contains(rendered, `val="dấu &quot; và &lt; &gt; &amp;"`) {
		t.Fatalf("escape thuoc tinh sai: %s", rendered)
	}
	if !strings.Contains(rendered, "a &lt; b &amp; c &gt; d") {
		t.Fatalf("escape text sai: %s", rendered)
	}
}

func TestDefaultNamespaceDeclarationMatches(t *testing.T) {
	// _rels/.rels va [Content_Types].xml dung namespace mac dinh, khong co tien to.
	source := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="t" Target="xl/workbook.xml"/></Relationships>`
	document := parse(t, source)
	relations := document.Root().Children(RelNamespace, "Relationship")
	if len(relations) != 1 || relations[0].AttrValue("", "Id") != "rId1" {
		t.Fatalf("khop theo namespace mac dinh that bai: %v", relations)
	}
}

func TestCloneIsDeep(t *testing.T) {
	document := parse(t, workbookSample)
	original := document.Root().Child(SpreadsheetNS, "sheets")
	copied := original.Clone()
	copied.Child(SpreadsheetNS, "sheet").SetAttr("", "", "name", "Đổi tên")
	if original.Child(SpreadsheetNS, "sheet").AttrValue("", "name") == "Đổi tên" {
		t.Fatal("clone phai la ban sao sau")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := ParseXML([]byte("khong phai xml")); err == nil {
		t.Fatal("phai bao loi voi du lieu khong phai XML")
	}
}
