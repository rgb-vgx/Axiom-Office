package xlsx

import (
	"reflect"
	"strconv"
	"testing"

	"axiomoffice/core/internal/ooxml"
)

// openTemplate dung mot workbook trong roi mo lai, tra ve ca goi lan book.
func openTemplate(t *testing.T, extension, firstSheet string) (*ooxml.Package, *Book, *SheetRef) {
	t.Helper()
	data, err := BuildTemplate(extension, firstSheet)
	if err != nil {
		t.Fatalf("BuildTemplate loi: %v", err)
	}
	pkg, err := ooxml.OpenBytes(data)
	if err != nil {
		t.Fatalf("mo goi loi: %v", err)
	}
	book, err := Open(pkg)
	if err != nil {
		t.Fatalf("mo workbook loi: %v", err)
	}
	target, err := book.FindSheet(firstSheet)
	if err != nil {
		t.Fatalf("tim sheet loi: %v", err)
	}
	return pkg, book, target
}

func reopen(t *testing.T, pkg *ooxml.Package) (*ooxml.Package, *Book) {
	t.Helper()
	packed, err := pkg.Archive()
	if err != nil {
		t.Fatalf("dong goi lai: %v", err)
	}
	reopened, err := ooxml.OpenBytes(packed)
	if err != nil {
		t.Fatalf("mo lai goi: %v", err)
	}
	book, err := Open(reopened)
	if err != nil {
		t.Fatalf("mo lai workbook: %v", err)
	}
	return reopened, book
}

func readAll(t *testing.T, book *Book, target *SheetRef, formulas bool) *Grid {
	t.Helper()
	grid, err := book.Read(target, formulas, 1, 1000, 1, 100)
	if err != nil {
		t.Fatalf("doc sheet loi: %v", err)
	}
	return grid
}

func TestTemplateHasOneSheet(t *testing.T) {
	_, book, _ := openTemplate(t, ".xlsx", "Trang 1")
	if got := book.SheetNames(); !reflect.DeepEqual(got, []string{"Trang 1"}) {
		t.Fatalf("danh sach sheet sai: %v", got)
	}
	active, err := book.ActiveSheet()
	if err != nil || active.Name != "Trang 1" {
		t.Fatalf("sheet dang mo sai: %v (%v)", active, err)
	}
}

func TestValueRoundTrip(t *testing.T) {
	pkg, book, target := openTemplate(t, ".xlsx", "Sheet1")
	doc, err := book.SheetDoc(target)
	if err != nil {
		t.Fatalf("doc sheet: %v", err)
	}
	data := book.SheetData(doc)
	values := []struct {
		row, col int
		value    any
	}{
		{1, 1, "Xin chào thế giới"},
		{1, 2, 9.5},
		{1, 3, int64(10)},
		{2, 1, true},
		{2, 2, false},
		{2, 3, "=A1"},
	}
	for _, item := range values {
		if err := book.SetValue(data, item.row, item.col, item.value); err != nil {
			t.Fatalf("ghi %v: %v", item.value, err)
		}
	}
	book.UpdateDimension(doc)
	book.SaveSheet(target, doc)

	_, reopened := reopen(t, pkg)
	sheetRef, err := reopened.FindSheet("Sheet1")
	if err != nil {
		t.Fatalf("tim sheet: %v", err)
	}
	grid := readAll(t, reopened, sheetRef, false)
	for _, item := range []struct {
		row, col int
		want     any
	}{
		{1, 1, "Xin chào thế giới"},
		{1, 2, 9.5},
		{1, 3, int64(10)},
		{2, 1, true},
		{2, 2, false},
	} {
		if got := grid.Get(item.row, item.col); !reflect.DeepEqual(got, item.want) {
			t.Fatalf("o %s = %#v, muon %#v", addressOf(item.row, item.col), got, item.want)
		}
	}
	// Cong thuc chi hien khi hoi rieng.
	if got := readAll(t, reopened, sheetRef, true).Get(2, 3); got != "=A1" {
		t.Fatalf("cong thuc sai: %#v", got)
	}
	if got := grid.Get(2, 3); got != nil {
		t.Fatalf("khong hoi cong thuc thi phai la gia tri trong: %#v", got)
	}
}

func addressOf(row, col int) string {
	return string(rune('A'+col-1)) + strconv.Itoa(row)
}

func TestSharedStringsAreDeduplicated(t *testing.T) {
	pkg, book, target := openTemplate(t, ".xlsx", "Sheet1")
	doc, err := book.SheetDoc(target)
	if err != nil {
		t.Fatalf("doc sheet: %v", err)
	}
	data := book.SheetData(doc)
	for index, text := range []string{"lặp", "khác", "lặp"} {
		if err := book.SetValue(data, index+1, 1, text); err != nil {
			t.Fatalf("ghi: %v", err)
		}
	}
	book.SaveSheet(target, doc)

	reopenedPkg, reopened := reopen(t, pkg)
	part := reopened.SharedStringsPart()
	if part == "" {
		t.Fatal("phai co part sharedStrings")
	}
	doc, err = reopenedPkg.XML(part)
	if err != nil {
		t.Fatalf("doc sharedStrings: %v", err)
	}
	entries := doc.Root().ChildrenLocal("si")
	if len(entries) != 2 {
		t.Fatalf("phai gop chuoi trung: %d muc", len(entries))
	}
	if got := doc.Root().AttrValue("", "uniqueCount"); got != "2" {
		t.Fatalf("uniqueCount sai: %q", got)
	}
	if got := RichText(entries[0]); got != "lặp" {
		t.Fatalf("noi dung shared string sai: %q", got)
	}
}

func TestSheetOperations(t *testing.T) {
	pkg, book, _ := openTemplate(t, ".xlsx", "Sheet1")
	if _, err := book.AddSheet("Dữ liệu", nil); err != nil {
		t.Fatalf("them sheet: %v", err)
	}
	if _, err := book.AddSheet("Sheet1", nil); err == nil {
		t.Fatal("them sheet trung ten phai bao loi")
	}
	if err := ValidateSheetName("a/b"); err == nil {
		t.Fatal("ten sheet co / phai bao loi")
	}
	if err := ValidateSheetName(""); err == nil {
		t.Fatal("ten sheet rong phai bao loi")
	}
	if err := ValidateSheetName("0123456789012345678901234567890123"); err == nil {
		t.Fatal("ten sheet qua 31 ky tu phai bao loi")
	}

	second, err := book.FindSheet("Dữ liệu")
	if err != nil {
		t.Fatalf("tim sheet moi: %v", err)
	}
	if err := book.RenameSheet(second, "Báo cáo"); err != nil {
		t.Fatalf("doi ten: %v", err)
	}

	pkg, reopened := reopen(t, pkg)
	if got := reopened.SheetNames(); !reflect.DeepEqual(got, []string{"Sheet1", "Báo cáo"}) {
		t.Fatalf("sau khi doi ten: %v", got)
	}

	target, err := reopened.FindSheet("Báo cáo")
	if err != nil {
		t.Fatalf("tim sheet sau doi ten: %v", err)
	}
	if err := reopened.DeleteSheet(target); err != nil {
		t.Fatalf("xoa sheet: %v", err)
	}
	pkg, again := reopen(t, pkg)
	if got := again.SheetNames(); !reflect.DeepEqual(got, []string{"Sheet1"}) {
		t.Fatalf("sau khi xoa: %v", got)
	}
	if again.Package.Exists("xl/worksheets/sheet2.xml") {
		t.Fatal("part cua sheet bi xoa phai duoc don")
	}
}

func TestCopySheetDropsTableParts(t *testing.T) {
	_, book, target := openTemplate(t, ".xlsx", "Sheet1")
	doc, err := book.SheetDoc(target)
	if err != nil {
		t.Fatalf("doc sheet: %v", err)
	}
	data := book.SheetData(doc)
	if err := book.SetValue(data, 1, 1, "giữ lại"); err != nil {
		t.Fatalf("ghi: %v", err)
	}
	// Them mot phan tu thuoc nhom phai bi bo khi sao chep.
	drawing := ooxml.NewElement(book.S, "", "drawing")
	doc.Root().Append(drawing)
	book.SaveSheet(target, doc)

	copied, err := book.CopySheet(target, "Bản sao")
	if err != nil {
		t.Fatalf("sao chep: %v", err)
	}
	if copied.Part == target.Part {
		t.Fatal("ban sao phai la part khac")
	}
	copiedDoc, err := book.SheetDoc(copied)
	if err != nil {
		t.Fatalf("doc ban sao: %v", err)
	}
	if copiedDoc.Root().Child(book.S, "drawing") != nil {
		t.Fatal("drawing phai bi bo khi sao chep")
	}
	copiedData := book.SheetData(copiedDoc)
	if got := book.CellText(copiedData, 1, 1); got != "giữ lại" {
		t.Fatalf("gia tri khong duoc sao chep: %q", got)
	}
}

func TestAddTableCreatesHeadersAndPart(t *testing.T) {
	pkg, book, target := openTemplate(t, ".xlsx", "Sheet1")
	doc, err := book.SheetDoc(target)
	if err != nil {
		t.Fatalf("doc sheet: %v", err)
	}
	data := book.SheetData(doc)
	if err := book.SetValue(data, 1, 1, "  "); err != nil { // tieu de trong -> tu dat ten cot
		t.Fatalf("ghi: %v", err)
	}
	if err := book.SetValue(data, 1, 2, "Điểm"); err != nil {
		t.Fatalf("ghi: %v", err)
	}
	if err := book.SetValue(data, 2, 1, "An"); err != nil {
		t.Fatalf("ghi: %v", err)
	}
	if err := book.SetValue(data, 2, 2, int64(9)); err != nil {
		t.Fatalf("ghi: %v", err)
	}
	book.SaveSheet(target, doc)

	if err := book.AddTable(target, "A1:B2", "Bang1"); err != nil {
		t.Fatalf("them bang: %v", err)
	}
	if err := book.AddTable(target, "A1:B1", "Bang2"); err == nil {
		t.Fatal("vung chi co tieu de phai bao loi")
	}

	_, reopened := reopen(t, pkg)
	if got := reopened.AllTableNames(); !reflect.DeepEqual(got, []string{"Bang1"}) {
		t.Fatalf("ten bang sai: %v", got)
	}
	sheetRef, err := reopened.FindSheet("Sheet1")
	if err != nil {
		t.Fatalf("tim sheet: %v", err)
	}
	if got := reopened.SheetTableNames(sheetRef); !reflect.DeepEqual(got, []string{"Bang1"}) {
		t.Fatalf("bang cua sheet sai: %v", got)
	}
	// Tieu de trong duoc dat ten tu dong, va giu nguyen chu co san.
	sheetDoc, err := reopened.SheetDoc(sheetRef)
	if err != nil {
		t.Fatalf("doc sheet: %v", err)
	}
	sheetData := reopened.SheetData(sheetDoc)
	if got := reopened.CellText(sheetData, 1, 1); got != "Column1" {
		t.Fatalf("tieu de trong phai thanh Column1: %q", got)
	}
	if got := reopened.CellText(sheetData, 1, 2); got != "Điểm" {
		t.Fatalf("tieu de co san bi doi: %q", got)
	}
	if sheetDoc.Root().ChildLocal("tableParts") == nil {
		t.Fatal("sheet phai co tableParts")
	}
}

func TestStylesDeriveAndDateCell(t *testing.T) {
	pkg, book, target := openTemplate(t, ".xlsx", "Sheet1")
	styles, err := book.Styles()
	if err != nil {
		t.Fatalf("mo styles: %v", err)
	}
	dateStyle, err := styles.Derive(0, map[string]any{"numFmt": "yyyy-mm-dd"})
	if err != nil {
		t.Fatalf("tao style ngay: %v", err)
	}
	boldStyle, err := styles.Derive(0, map[string]any{
		"font": map[string]any{"bold": true, "color": "#FF0000"},
		"fill": map[string]any{"color": "#FFFF00"},
	})
	if err != nil {
		t.Fatalf("tao style dam: %v", err)
	}
	// Cung spec thi phai dung lai chi so cu (co cache).
	again, err := styles.Derive(0, map[string]any{
		"font": map[string]any{"bold": true, "color": "#FF0000"},
		"fill": map[string]any{"color": "#FFFF00"},
	})
	if err != nil || again != boldStyle {
		t.Fatalf("cache style sai: %d vs %d (%v)", again, boldStyle, err)
	}

	doc, err := book.SheetDoc(target)
	if err != nil {
		t.Fatalf("doc sheet: %v", err)
	}
	data := book.SheetData(doc)
	if err := book.SetValue(data, 1, 1, int64(45292)); err != nil {
		t.Fatalf("ghi ngay: %v", err)
	}
	book.GetOrCreateCell(data, 1, 1).SetAttr("", "", "s", strconv.Itoa(dateStyle))
	if err := book.SetValue(data, 2, 1, "đậm"); err != nil {
		t.Fatalf("ghi chu: %v", err)
	}
	book.GetOrCreateCell(data, 2, 1).SetAttr("", "", "s", strconv.Itoa(boldStyle))
	book.SaveSheet(target, doc)

	_, reopened := reopen(t, pkg)
	sheetRef, err := reopened.FindSheet("Sheet1")
	if err != nil {
		t.Fatalf("tim sheet: %v", err)
	}
	grid := readAll(t, reopened, sheetRef, false)
	if got := grid.Get(1, 1); got != "2024-01-01T00:00:00" {
		t.Fatalf("o dinh dang ngay phai doc ra ISO: %#v", got)
	}
	if got := grid.Get(2, 1); got != "đậm" {
		t.Fatalf("o chu sai: %#v", got)
	}
}

func TestRgbValidation(t *testing.T) {
	cases := map[string]string{
		"#FF0000":  "FF0000",
		"ff0000":   "FF0000",
		"#00ff00":  "00FF00",
		"FF112233": "112233",
	}
	for input, want := range cases {
		if got, err := Rgb(input); err != nil || got != want {
			t.Fatalf("Rgb(%q) = %q, %v; muon %q", input, got, err, want)
		}
	}
	for _, bad := range []any{"", "đỏ", "#12345", "#GGGGGG", nil} {
		if _, err := Rgb(bad); err == nil {
			t.Fatalf("Rgb(%v) phai bao loi", bad)
		}
	}
}

// Ghi lai cung mot o nhieu lan khong duoc sinh them o/dong trung (loi tung co: Append roi InsertAt).
func TestRepeatedWriteDoesNotDuplicate(t *testing.T) {
	_, book, target := openTemplate(t, ".xlsx", "Sheet1")
	doc, err := book.SheetDoc(target)
	if err != nil {
		t.Fatalf("doc sheet: %v", err)
	}
	data := book.SheetData(doc)
	for round := 0; round < 3; round++ {
		// Ghi cot 2 truoc cot 1 de ep dung nhanh "chen truoc".
		if err := book.SetValue(data, 1, 2, "b"); err != nil {
			t.Fatalf("ghi B1: %v", err)
		}
		if err := book.SetValue(data, 1, 1, "a"); err != nil {
			t.Fatalf("ghi A1: %v", err)
		}
		if err := book.SetValue(data, 5, 3, "c"); err != nil {
			t.Fatalf("ghi C5: %v", err)
		}
	}
	rows := data.ChildrenLocal("row")
	if len(rows) != 2 {
		t.Fatalf("phai co dung 2 dong, dang co %d", len(rows))
	}
	if len(rows[0].ChildrenLocal("c")) != 2 {
		t.Fatalf("dong 1 phai co dung 2 o, dang co %d", len(rows[0].ChildrenLocal("c")))
	}
	// Thu tu cot phai tang dan.
	first := rows[0].ChildrenLocal("c")
	if first[0].AttrValue("", "r") != "A1" || first[1].AttrValue("", "r") != "B1" {
		t.Fatalf("thu tu o sai: %q %q", first[0].AttrValue("", "r"), first[1].AttrValue("", "r"))
	}
	if rows[1].ChildrenLocal("c")[0].AttrValue("", "r") != "C5" {
		t.Fatalf("dong 5 sai: %q", rows[1].ChildrenLocal("c")[0].AttrValue("", "r"))
	}
}

func TestUntouchedPartsSurviveSheetEdit(t *testing.T) {
	pkg, book, target := openTemplate(t, ".xlsx", "Sheet1")
	before, err := pkg.Bytes("xl/styles.xml")
	if err != nil {
		t.Fatalf("doc styles: %v", err)
	}
	doc, err := book.SheetDoc(target)
	if err != nil {
		t.Fatalf("doc sheet: %v", err)
	}
	if err := book.SetValue(book.SheetData(doc), 1, 1, "chỉ sửa sheet"); err != nil {
		t.Fatalf("ghi: %v", err)
	}
	book.SaveSheet(target, doc)

	reopenedPkg, _ := reopen(t, pkg)
	after, err := reopenedPkg.Bytes("xl/styles.xml")
	if err != nil {
		t.Fatalf("doc styles sau: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("styles.xml phai duoc giu nguyen khi chi sua sheet")
	}
}

func TestOpenRejectsNonWorkbook(t *testing.T) {
	// Goi docx (khong co part officeDocument) phai bi tu choi.
	pkg := ooxml.NewPackage()
	root := ooxml.NewRoot("http://schemas.openxmlformats.org/package/2006/relationships", "Relationships")
	pkg.Put(ooxml.RootRelsPart, ooxml.NewDocument(root))
	if _, err := Open(pkg); err == nil {
		t.Fatal("goi khong phai workbook phai bao loi")
	}
}
