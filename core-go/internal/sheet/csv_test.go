package sheet

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("ghi file: %v", err)
	}
	return path
}

func TestReadCSVCoercesNumbers(t *testing.T) {
	path := writeFile(t, "so.csv", "Tên,Điểm,Ghi chú\nAn,9.5,giỏi\nBình,10,\n")
	rows, err := ReadCSV(path)
	if err != nil {
		t.Fatalf("ReadCSV loi: %v", err)
	}
	expected := [][]any{
		{"Tên", "Điểm", "Ghi chú"},
		{"An", 9.5, "giỏi"},
		{"Bình", int64(10), ""},
	}
	if !reflect.DeepEqual(rows, expected) {
		t.Fatalf("doc sai:\n got %#v\nwant %#v", rows, expected)
	}
}

func TestReadCSVSniffsDelimiter(t *testing.T) {
	path := writeFile(t, "cham.csv", "a;b;c\n1;2;3\n4;5;6\n")
	rows, err := ReadCSV(path)
	if err != nil {
		t.Fatalf("ReadCSV loi: %v", err)
	}
	if len(rows) != 3 || len(rows[0]) != 3 || rows[1][2] != int64(3) {
		t.Fatalf("doan dau phan cach sai: %#v", rows)
	}
}

func TestReadCSVTsvAlwaysUsesTab(t *testing.T) {
	// Duoi .tsv thi dung tab, khong doan, ke ca khi trong noi dung co nhieu dau phay.
	path := writeFile(t, "bang.tsv", "a,b\tc\n1,2\t3\n")
	rows, err := ReadCSV(path)
	if err != nil {
		t.Fatalf("ReadCSV loi: %v", err)
	}
	if len(rows) != 2 || len(rows[0]) != 2 || rows[0][0] != "a,b" || rows[0][1] != "c" {
		t.Fatalf("tab sai: %#v", rows)
	}
}

func TestReadCSVHandlesQuotesAndNewlines(t *testing.T) {
	path := writeFile(t, "phuc.csv", "\"a,b\",\"nói \"\"to\"\"\",\"dòng 1\ndòng 2\"\r\nx,y,z\r\n")
	rows, err := ReadCSV(path)
	if err != nil {
		t.Fatalf("ReadCSV loi: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("so dong sai: %#v", rows)
	}
	if rows[0][0] != "a,b" || rows[0][1] != `nói "to"` || rows[0][2] != "dòng 1\ndòng 2" {
		t.Fatalf("giai dau ngoac sai: %#v", rows[0])
	}
}

func TestReadCSVStripsBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bom.csv")
	if err := os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, []byte("a,b\n")...), 0o644); err != nil {
		t.Fatalf("ghi file: %v", err)
	}
	rows, err := ReadCSV(path)
	if err != nil {
		t.Fatalf("ReadCSV loi: %v", err)
	}
	if len(rows) != 1 || rows[0][0] != "a" {
		t.Fatalf("BOM khong duoc bo: %#v", rows)
	}
}

func TestWriteCSVUsesBOMAndCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ghi.csv")
	rows := [][]any{{"a", 1}, {"có, phẩy", `dấu "`}}
	if err := WriteCSV(path, rows); err != nil {
		t.Fatalf("WriteCSV loi: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("doc lai: %v", err)
	}
	want := "\xEF\xBB\xBFa,1\r\n\"có, phẩy\",\"dấu \"\"\"\r\n"
	if string(content) != want {
		t.Fatalf("noi dung ghi sai:\n got %q\nwant %q", content, want)
	}
	// Doc lai phai ra gia tri tuong duong (tru kieu so da ep).
	back, err := ReadCSV(path)
	if err != nil {
		t.Fatalf("doc lai: %v", err)
	}
	if len(back) != 2 || back[0][0] != "a" || back[0][1] != int64(1) || back[1][0] != "có, phẩy" || back[1][1] != `dấu "` {
		t.Fatalf("vong tron sai: %#v", back)
	}
}

func TestCoerceKeepsUnparseableText(t *testing.T) {
	cases := []struct {
		text string
		want any
	}{
		{"1", int64(1)},
		{"-2", int64(-2)},
		{"1.0", 1.0},
		{"1.5", 1.5},
		{"007", "007"},
		{"+5", "+5"},
		{"1e5", "1e5"},
		{"", ""},
		// Khong ep duoc thanh so thi tra ve chuoi GOC, khong cat khoang trang (giong ban C#).
		{"  xin chào  ", "  xin chào  "},
	}
	for _, item := range cases {
		if got := coerce(item.text); !reflect.DeepEqual(got, item.want) {
			t.Fatalf("coerce(%q) = %#v, muon %#v", item.text, got, item.want)
		}
	}
}

func TestIsCsvExtension(t *testing.T) {
	for _, extension := range []string{".csv", ".tsv", ".txt"} {
		if !IsCsvExtension(extension) {
			t.Fatalf("%s phai duoc nhan", extension)
		}
	}
	for _, extension := range []string{".xlsx", ".xls", "csv", ""} {
		if IsCsvExtension(extension) {
			t.Fatalf("%s khong duoc nhan", extension)
		}
	}
}
