package ooxml

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"

	"axiomoffice/core/internal/filesafe"
	"testing"
)

const (
	contentTypesSample = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/></Types>`
	rootRelsSample     = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
	bookSample         = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Trang 1" sheetId="1" r:id="rId1"/></sheets></workbook>`
	imageSample        = "\x89PNG\r\n\x1a\n\x00\x01\x02\xff\xfe"
)

// buildSamplePackage dung mot goi nho co du: content types, .rels goc, workbook va mot file nhi phan
// (gia lap anh) de kiem tra phan khong bi sua duoc giu nguyen byte.
func buildSamplePackage(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range []struct {
		name    string
		content string
	}{
		{ContentTypesPart, contentTypesSample},
		{RootRelsPart, rootRelsSample},
		{"xl/workbook.xml", bookSample},
		{"xl/media/image1.png", imageSample},
	} {
		handle, err := writer.Create(entry.name)
		if err != nil {
			t.Fatalf("tao entry %s: %v", entry.name, err)
		}
		if _, err := handle.Write([]byte(entry.content)); err != nil {
			t.Fatalf("ghi entry %s: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("dong zip: %v", err)
	}
	return buffer.Bytes()
}

func openSample(t *testing.T) *Package {
	t.Helper()
	pkg, err := OpenBytes(buildSamplePackage(t))
	if err != nil {
		t.Fatalf("mo goi loi: %v", err)
	}
	return pkg
}

func TestRelsResolveTargetPart(t *testing.T) {
	pkg := openSample(t)
	rel := pkg.RelOfType("", "officeDocument")
	if rel == nil {
		t.Fatal("khong tim thay quan he officeDocument")
	}
	if rel.TargetPart != "xl/workbook.xml" {
		t.Fatalf("TargetPart sai: %q", rel.TargetPart)
	}
	if rel.External {
		t.Fatal("quan he noi bo khong duoc danh dau External")
	}
	if pkg.RelOfType("", "core-properties") != nil {
		t.Fatal("khong duoc co quan he core-properties")
	}
}

func TestEditKeepsUntouchedPartsByteForByte(t *testing.T) {
	pkg := openSample(t)
	workbook, err := pkg.XML("xl/workbook.xml")
	if err != nil {
		t.Fatalf("doc workbook: %v", err)
	}
	sheet := workbook.Root().Child(SpreadsheetNS, "sheets").Child(SpreadsheetNS, "sheet")
	sheet.SetAttr("", "", "name", "Dữ liệu")
	pkg.Put("xl/workbook.xml", workbook)

	packed, err := pkg.Archive()
	if err != nil {
		t.Fatalf("dong goi: %v", err)
	}
	reopened, err := OpenBytes(packed)
	if err != nil {
		t.Fatalf("mo lai: %v", err)
	}

	// Part da sua: thay doi co hieu luc.
	updated := reopened.XMLOrNil("xl/workbook.xml")
	if name := updated.Root().Child(SpreadsheetNS, "sheets").Child(SpreadsheetNS, "sheet").AttrValue("", "name"); name != "Dữ liệu" {
		t.Fatalf("sua khong co hieu luc: %q", name)
	}
	// Part khong dung toi: giu nguyen tung byte (day la tinh chat bao ve chart/anh/pivot/macro).
	image, err := reopened.Bytes("xl/media/image1.png")
	if err != nil || string(image) != imageSample {
		t.Fatalf("anh bi thay doi: %q (loi %v)", image, err)
	}
	types, err := reopened.Bytes(ContentTypesPart)
	if err != nil || string(types) != contentTypesSample {
		t.Fatalf("content types bi thay doi: %q (loi %v)", types, err)
	}
	rels, err := reopened.Bytes(RootRelsPart)
	if err != nil || string(rels) != rootRelsSample {
		t.Fatalf(".rels goc bi thay doi: %q (loi %v)", rels, err)
	}
}

func TestReadOnlyPartIsNotRewritten(t *testing.T) {
	pkg := openSample(t)
	if _, err := pkg.XML("xl/workbook.xml"); err != nil {
		t.Fatalf("doc workbook: %v", err)
	}
	packed, err := pkg.Archive()
	if err != nil {
		t.Fatalf("dong goi: %v", err)
	}
	reopened, err := OpenBytes(packed)
	if err != nil {
		t.Fatalf("mo lai: %v", err)
	}
	book, err := reopened.Bytes("xl/workbook.xml")
	if err != nil {
		t.Fatalf("doc lai workbook: %v", err)
	}
	if string(book) != bookSample {
		t.Fatalf("part chi doc bi ghi lai: %s", book)
	}
}

func TestAddRelCreatesRootRelsWhenMissing(t *testing.T) {
	pkg := openSample(t)
	id := pkg.AddRel("xl/workbook.xml", "worksheet", "xl/worksheets/sheet1.xml")
	if id != "rId1" {
		t.Fatalf("Id dau tien phai la rId1: %q", id)
	}
	relsPart := RelsPartFor("xl/workbook.xml")
	if relsPart != "xl/_rels/workbook.xml.rels" {
		t.Fatalf("ten part .rels sai: %q", relsPart)
	}
	rel := pkg.Rels("xl/workbook.xml")
	if len(rel) != 1 {
		t.Fatalf("phai co 1 quan he: %v", rel)
	}
	// Target tuong doi tu xl/ toi xl/worksheets/...
	if rel[0].Target != "worksheets/sheet1.xml" {
		t.Fatalf("Target sai: %q", rel[0].Target)
	}
	if rel[0].Type != RelTypeBase+"worksheet" {
		t.Fatalf("Type phai duoc ghep tien to: %q", rel[0].Type)
	}
	if rel[0].TargetPart != "xl/worksheets/sheet1.xml" {
		t.Fatalf("TargetPart sai: %q", rel[0].TargetPart)
	}
}

func TestAddRelCapsIdAtExistingCount(t *testing.T) {
	pkg := openSample(t)
	// Goi goc da co rId1 -> quan he moi phai la rId2.
	if id := pkg.AddRel("", "worksheet", "xl/worksheets/sheet1.xml"); id != "rId2" {
		t.Fatalf("Id tiep theo phai la rId2: %q", id)
	}
}

func TestResolveAndRelativeTarget(t *testing.T) {
	cases := []struct{ source, target, want string }{
		{"xl/workbook.xml", "worksheets/sheet1.xml", "xl/worksheets/sheet1.xml"},
		{"xl/workbook.xml", "/xl/styles.xml", "xl/styles.xml"},
		{"xl/worksheets/sheet1.xml", "../styles.xml", "xl/styles.xml"},
		{"", "xl/workbook.xml", "xl/workbook.xml"},
		{"word/document.xml", "media/image1.png", "word/media/image1.png"},
	}
	for _, item := range cases {
		if got := ResolveTarget(item.source, item.target); got != item.want {
			t.Fatalf("ResolveTarget(%q, %q) = %q, muon %q", item.source, item.target, got, item.want)
		}
	}
	relative := []struct{ source, target, want string }{
		{"xl/workbook.xml", "xl/worksheets/sheet1.xml", "worksheets/sheet1.xml"},
		{"", "xl/workbook.xml", "xl/workbook.xml"},
		{"word/document.xml", "word/media/image1.png", "media/image1.png"},
	}
	for _, item := range relative {
		if got := RelativeTarget(item.source, item.target); got != item.want {
			t.Fatalf("RelativeTarget(%q, %q) = %q, muon %q", item.source, item.target, got, item.want)
		}
	}
}

func TestContentTypeOverride(t *testing.T) {
	pkg := openSample(t)
	if got := pkg.ContentTypeOf("xl/workbook.xml"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml" {
		t.Fatalf("Override sai: %q", got)
	}
	if got := pkg.ContentTypeOf("xl/media/image1.png"); got != "image/png" {
		t.Fatalf("Default theo duoi file sai: %q", got)
	}
	if err := pkg.SetContentTypeOverride("xl/worksheets/sheet1.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"); err != nil {
		t.Fatalf("dat override: %v", err)
	}
	if got := pkg.ContentTypeOf("xl/worksheets/sheet1.xml"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml" {
		t.Fatalf("Override moi sai: %q", got)
	}
	// Xoa Override -> roi ve Default theo duoi file (giong ban C#).
	pkg.RemoveContentTypeOverride("xl/worksheets/sheet1.xml")
	if got := pkg.ContentTypeOf("xl/worksheets/sheet1.xml"); got != "application/xml" {
		t.Fatalf("sau khi xoa Override phai roi ve Default: %q", got)
	}
}

func TestDeleteRemovesPartRelsAndOverride(t *testing.T) {
	pkg := openSample(t)
	pkg.Delete("xl/workbook.xml")
	if pkg.Exists("xl/workbook.xml") {
		t.Fatal("part phai bi xoa")
	}
	if got := pkg.ContentTypeOf("xl/workbook.xml"); got != "application/xml" {
		t.Fatalf("override phai bi xoa theo (roi ve Default): %q", got)
	}
}

func TestNextPartName(t *testing.T) {
	pkg := openSample(t)
	if got := pkg.NextPartName("xl/worksheets/sheet%d.xml"); got != "xl/worksheets/sheet1.xml" {
		t.Fatalf("ten part ke tiep sai: %q", got)
	}
}

func TestOpenReadRejectsNonZip(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "khong-phai-zip.xlsx")
	if err := os.WriteFile(path, []byte("day khong phai la zip"), 0o644); err != nil {
		t.Fatalf("ghi file: %v", err)
	}
	if _, err := OpenRead(path); err == nil {
		t.Fatal("phai bao loi voi file khong phai zip")
	}
}

func TestEditWritesFileAndPreservesMode(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "book.xlsx")
	if err := os.WriteFile(path, buildSamplePackage(t), 0o640); err != nil {
		t.Fatalf("ghi file: %v", err)
	}
	err := Edit(path, func(pkg *Package) error {
		workbook, err := pkg.XML("xl/workbook.xml")
		if err != nil {
			return err
		}
		workbook.Root().Child(SpreadsheetNS, "sheets").Child(SpreadsheetNS, "sheet").SetAttr("", "", "name", "Đã sửa")
		pkg.Put("xl/workbook.xml", workbook)
		return nil
	})
	if err != nil {
		t.Fatalf("Edit loi: %v", err)
	}
	reopened, err := OpenRead(path)
	if err != nil {
		t.Fatalf("mo lai: %v", err)
	}
	name := reopened.XMLOrNil("xl/workbook.xml").Root().Child(SpreadsheetNS, "sheets").Child(SpreadsheetNS, "sheet").AttrValue("", "name")
	if name != "Đã sửa" {
		t.Fatalf("sua khong duoc ghi ra: %q", name)
	}
	// Khong con file tam nao sot lai.
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("doc thu muc: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("con file tam sot lai: %v", entries)
	}
}

func TestEditReportsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "khong-co.xlsx")
	err := Edit(path, func(*Package) error { return nil })
	if err == nil || err.Error() != "file not found: "+path {
		t.Fatalf("thong bao loi sai: %v", err)
	}
}

func TestRequireHelpers(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "co-roi.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("ghi file: %v", err)
	}
	if err := filesafe.RequireFile(""); err == nil || err.Error() != "path is required" {
		t.Fatalf("thong bao thieu path sai: %v", err)
	}
	if err := filesafe.RequireFile(filepath.Join(directory, "khong-co.txt")); err == nil {
		t.Fatal("phai bao loi file khong ton tai")
	}
	if err := filesafe.RequireNewOrOverwrite(path, false); err == nil || err.Error() != "file already exists - pass overwrite=true to replace it" {
		t.Fatalf("thong bao ghi de sai: %v", err)
	}
	if err := filesafe.RequireNewOrOverwrite(path, true); err != nil {
		t.Fatalf("overwrite=true phai cho qua: %v", err)
	}
}

func TestCreateFromTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "moi.xlsx")
	err := CreateFromTemplate(path, buildSamplePackage(t), func(pkg *Package) error {
		workbook := pkg.XMLOrNil("xl/workbook.xml")
		workbook.Root().Child(SpreadsheetNS, "sheets").Child(SpreadsheetNS, "sheet").SetAttr("", "", "name", "Tạo mới")
		pkg.Put("xl/workbook.xml", workbook)
		return nil
	})
	if err != nil {
		t.Fatalf("CreateFromTemplate loi: %v", err)
	}
	reopened, err := OpenRead(path)
	if err != nil {
		t.Fatalf("mo lai: %v", err)
	}
	sheet := reopened.XMLOrNil("xl/workbook.xml").Root().Child(SpreadsheetNS, "sheets").Child(SpreadsheetNS, "sheet")
	if name := sheet.AttrValue("", "name"); name != "Tạo mới" {
		t.Fatalf("file tao tu template sai: %q", name)
	}
}
