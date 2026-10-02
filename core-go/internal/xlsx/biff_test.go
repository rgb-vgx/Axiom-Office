package xlsx

import (
	"fmt"
	"math"

	"axiomoffice/core/internal/ooxml"
	"os"
	"path/filepath"
	"testing"
)

func biffFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("thieu file mau %s: %v", name, err)
	}
	return data
}

// Doc gia tri tu .xls: chuoi (co dau tieng Viet), so nguyen, so thuc, va ngay.
//
// File mau do chinh LibreOffice tao ra tu mot .xlsx da biet truoc, nen day la doi chieu voi mot ben
// doc doc lap chu khong phai tu kiem minh.
func TestReadBIFFValues(t *testing.T) {
	grids, err := ReadBIFF(biffFixture(t, "sample.xls"), "", true)
	if err != nil {
		t.Fatalf("ReadBIFF: %v", err)
	}
	if len(grids) != 2 {
		t.Fatalf("phai co 2 sheet, duoc %d", len(grids))
	}

	data := grids[0]
	if data.Name != "Data" {
		t.Fatalf("ten sheet dau = %q", data.Name)
	}
	if got := data.Get(1, 1); got != "Tên" {
		t.Errorf("A1 = %#v, muon 'Tên'", got)
	}
	if got := data.Get(1, 2); got != "Điểm" {
		t.Errorf("B1 = %#v, muon 'Điểm'", got)
	}
	if got := data.Get(2, 1); got != "An" {
		t.Errorf("A2 = %#v", got)
	}
	if got := data.Get(2, 2); got != 9.5 {
		t.Errorf("B2 = %#v, muon 9.5", got)
	}
	if got := data.Get(3, 1); got != "Bình" {
		t.Errorf("A3 = %#v, muon 'Bình'", got)
	}
	if got := data.Get(3, 2); got != int64(8) {
		t.Errorf("B3 = %#v, muon int64(8)", got)
	}
	// Ngay: BIFF luu bang so + dinh dang, phai ra chuoi ISO. Kem ca gio vi sheet.SerialToIso luon dinh
	// dang "2006-01-02T15:04:05" - duong doc .xlsx cung ra dung dang do, va ban C# COM truoc day dung
	// "yyyy-MM-ddTHH:mm:ss". Day la khop voi hai duong san co, khong phai y thich cua rieng ham nay.
	if got := data.Get(2, 3); got != "2026-10-02T00:00:00" {
		t.Errorf("C2 = %#v, muon '2026-10-02T00:00:00'", got)
	}
	if got := data.Get(3, 3); got != "2025-01-15T00:00:00" {
		t.Errorf("C3 = %#v, muon '2025-01-15T00:00:00'", got)
	}

	other := grids[1]
	if other.Name != "Trống" {
		t.Fatalf("ten sheet hai = %q", other.Name)
	}
	if got := other.Get(1, 1); got != "một hai ba" {
		t.Errorf("Trống!A1 = %#v", got)
	}
}

// Chi lay mot sheet theo ten, va sheet dau tien khi khong noi gi.
func TestReadBIFFSelectsSheets(t *testing.T) {
	data := biffFixture(t, "sample.xls")
	one, err := ReadBIFF(data, "Trống", false)
	if err != nil {
		t.Fatalf("ReadBIFF theo ten: %v", err)
	}
	if len(one) != 1 || one[0].Name != "Trống" {
		t.Fatalf("chon theo ten: %+v", one)
	}

	first, err := ReadBIFF(data, "", false)
	if err != nil {
		t.Fatalf("ReadBIFF khong ten: %v", err)
	}
	if len(first) != 1 || first[0].Name != "Data" {
		t.Fatalf("khong noi gi thi lay sheet dau: %+v", first)
	}

	if _, err := ReadBIFF(data, "Không có", false); err == nil {
		t.Fatal("ten sheet sai phai bao loi")
	}
}

// File lon: bang chuoi dung chung (SST) dai hon 8 KB nen bi cat qua nhieu ban ghi CONTINUE - day la cho
// kho nhat cua BIFF. Doc sai thi chuoi ra rac chu khong bao loi, nen phai kiem chuoi o CUOI bang.
func TestReadBIFFLargeSharedStrings(t *testing.T) {
	grids, err := ReadBIFF(biffFixture(t, "large.xls"), "", true)
	if err != nil {
		t.Fatalf("ReadBIFF: %v", err)
	}
	if len(grids) != 2 {
		t.Fatalf("phai co 2 sheet, duoc %d", len(grids))
	}
	data := grids[0]
	if data.MaxRow < 301 {
		t.Fatalf("MaxRow = %d, phai >= 301", data.MaxRow)
	}
	for _, row := range []int{1, 2, 150, 300, 301} {
		if got := data.Get(row, 1); got == nil {
			t.Errorf("dong %d cot A rong", row)
		}
	}
	if got := data.Get(2, 1); got != "Học sinh số 0" {
		t.Errorf("A2 = %#v", got)
	}
	if got := data.Get(301, 1); got != "Học sinh số 299" {
		t.Errorf("A301 = %#v", got)
	}
	// Chuoi dai, co dau, nam sau rat nhieu chuoi khac trong SST.
	if got, _ := data.Get(2, 4).(string); got == "" {
		t.Errorf("D2 = %#v, phai la chuoi dai", data.Get(2, 4))
	} else if want := "ghi chú dài dòng cho dòng 0 để stream vượt 4096 byte"; got != want {
		t.Errorf("D2 = %q\nmuon %q", got, want)
	}

	second := grids[1]
	if second.Name != "Phụ" {
		t.Fatalf("sheet hai = %q", second.Name)
	}
	if got := second.Get(1, 2); got != "tiếng Việt có dấu: ăâđêôơư" {
		t.Errorf("Phụ!B1 = %#v", got)
	}
}

// Phep thu manh nhat: CUNG mot bang, hai dinh dang, hai ben doc DOC LAP.
//
// sample.xlsx la ban goc; sample.xls la chinh no sau khi LibreOffice doi sang Excel 97-2003. Doc ca hai
// roi so tung o - khong phai toi tu kiem minh, ma la doi chieu hai duong doc khac han nhau (OOXML zip
// va BIFF8 trong thung CFB).
func TestReadBIFFMatchesXlsxReader(t *testing.T) {
	package_, err := ooxml.OpenBytes(biffFixture(t, "sample.xlsx"))
	if err != nil {
		t.Fatalf("mo .xlsx: %v", err)
	}
	book, err := Open(package_)
	if err != nil {
		t.Fatalf("doc .xlsx: %v", err)
	}
	fromXlsx := map[string]any{}
	for _, name := range book.SheetNames() {
		reference, err := book.FindSheet(name)
		if err != nil {
			t.Fatal(err)
		}
		grid, err := book.Read(reference, false, 1, math.MaxInt32, 1, math.MaxInt32)
		if err != nil {
			t.Fatal(err)
		}
		for row := 1; row <= grid.MaxRow; row++ {
			for col := 1; col <= grid.MaxCol; col++ {
				if value := grid.Get(row, col); value != nil {
					fromXlsx[cellKey(name, row, col)] = value
				}
			}
		}
	}

	grids, err := ReadBIFF(biffFixture(t, "sample.xls"), "", true)
	if err != nil {
		t.Fatalf("ReadBIFF: %v", err)
	}
	checked := 0
	for _, grid := range grids {
		for row := 1; row <= grid.MaxRow; row++ {
			for col := 1; col <= grid.MaxCol; col++ {
				value := grid.Get(row, col)
				if value == nil {
					continue
				}
				key := cellKey(grid.Name, row, col)
				want, ok := fromXlsx[key]
				if !ok {
					t.Errorf("%s: .xls co gia tri %#v nhung .xlsx khong co", key, value)
					continue
				}
				if !sameValue(value, want) {
					t.Errorf("%s: .xls ra %#v (%T), .xlsx ra %#v (%T)", key, value, value, want, want)
				}
				checked++
			}
		}
	}
	if checked < 10 {
		t.Fatalf("chi so duoc %d o, qua it de co y nghia", checked)
	}
	t.Logf("doi chieu %d o giua duong doc .xls va duong doc .xlsx", checked)
}

// sameValue: so sanh gia tri giua hai duong doc. So nguyen va so thuc cung gia tri thi coi la khop
// (duong .xlsx tra int64, duong BIFF co the tra float64 tuy ban ghi luu no).
func sameValue(a, b any) bool {
	if a == b {
		return true
	}
	left, leftOK := toFloat(a)
	right, rightOK := toFloat(b)
	return leftOK && rightOK && left == right
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	case float64:
		return typed, true
	}
	return 0, false
}

func cellKey(sheet string, row, col int) string {
	return fmt.Sprintf("%s!%d:%d", sheet, row, col)
}

// Bang chung cho dung tinh huong nguoi dung gap: may Windows chi co Office/WPS, KHONG co LibreOffice.
//
// AXIOM_SOFFICE=none lam FindSoffice tra ve rong, nen neu con duong doc .xls nao con nho LibreOffice thi
// bai nay do ngay. Truoc day day chinh la truong hop lam .xls khong doc duoc.
func TestReadXLSWithoutLibreOffice(t *testing.T) {
	t.Setenv("AXIOM_SOFFICE", NoSoffice)
	if FindSoffice() != "" {
		t.Fatal("AXIOM_SOFFICE=none phai lam FindSoffice tra ve rong")
	}
	grids, err := ReadXLS(filepath.Join("testdata", "sample.xls"), "", true)
	if err != nil {
		t.Fatalf("doc .xls khi khong co LibreOffice: %v", err)
	}
	if len(grids) != 2 {
		t.Fatalf("phai co 2 sheet, duoc %d", len(grids))
	}
	if got := grids[0].Get(1, 1); got != "Tên" {
		t.Errorf("A1 = %#v", got)
	}
	if got := grids[0].Get(2, 3); got != "2026-10-02T00:00:00" {
		t.Errorf("ngay C2 = %#v", got)
	}

	// File .xls khong doc duoc (khong phai thung CFB) thi van phai bao loi ro, va loi phai noi ca hai
	// duong da thu.
	if _, err := ReadXLS(filepath.Join("testdata", "sample.xlsx"), "", true); err == nil {
		t.Fatal("file khong phai .xls phai bao loi")
	}
}

// Du lieu khong phai .xls thi bao loi ro, khong panic.
func TestReadBIFFRejectsOtherFiles(t *testing.T) {
	if _, err := ReadBIFF([]byte("khong phai .xls"), "", true); err == nil {
		t.Fatal("du lieu la phai bao loi")
	}
	// Zip (OOXML) khong phai thung CFB.
	if _, err := ReadBIFF([]byte("PK\x03\x04\x00\x00\x00\x00"), "", true); err == nil {
		t.Fatal("file .xlsx khong duoc coi la .xls")
	}
}
