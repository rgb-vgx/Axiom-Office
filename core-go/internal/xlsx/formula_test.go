package xlsx

import "testing"

// ShiftFormula thay cho regex co lookbehind/lookahead cua ban C#; cac ca duoi day chinh la
// nhung cho hai thu do phai chan.
func TestShiftFormula(t *testing.T) {
	cases := []struct {
		formula string
		dRow    int
		dCol    int
		want    string
	}{
		{"A1+B2", 1, 0, "A2+B3"},
		{"A1", 0, 1, "B1"},
		// Tham chieu tuyet doi thi khong doi.
		{"$A$1", 3, 3, "$A$1"},
		// $ chi khoa mot chieu.
		{"A$1", 1, 1, "B$1"},
		{"$A1", 1, 1, "$A2"},
		// Vung chon.
		{"SUM(A1:A5)", 1, 0, "SUM(A2:A6)"},
		// Ten ham co chu so khong duoc phep doi (lookahead chan dau mo ngoac).
		{"LOG10(A1)", 1, 0, "LOG10(A2)"},
		// Ma dinh danh dinh lien: ca hai deu khong duoc doi (lookbehind/lookahead).
		{"X1A1", 1, 0, "X1A1"},
		{"_A1", 1, 0, "_A1"},
		{"A1_x", 1, 0, "A1_x"},
		{".A1", 1, 0, ".A1"},
		// Chuoi trong ngoac kep giu nguyen.
		{`"A1"&B2`, 1, 1, `"A1"&C3`},
		{`IF(A1="x1","A1",B2)`, 1, 0, `IF(A2="x1","A1",B3)`},
		// Day ra ngoai bang tinh -> #REF!.
		{"A1", -1, 0, "#REF!"},
		{"A1", 0, -1, "#REF!"},
		// Khong co tham chieu nao.
		{"1+2", 1, 1, "1+2"},
	}
	for _, item := range cases {
		if got := ShiftFormula(item.formula, item.dRow, item.dCol); got != item.want {
			t.Fatalf("ShiftFormula(%q, %d, %d) = %q, muon %q", item.formula, item.dRow, item.dCol, got, item.want)
		}
	}
}

func TestQuoteSheet(t *testing.T) {
	cases := map[string]string{
		"Data":     "Data",
		"_a1":      "_a1",
		"a.b":      "a.b",
		"Trang 1":  "'Trang 1'",
		"a'b":      "'a''b'",
		"1sheet":   "'1sheet'",
		"a-b":      "'a-b'",
	}
	for name, want := range cases {
		if got := QuoteSheet(name); got != want {
			t.Fatalf("QuoteSheet(%q) = %q, muon %q", name, got, want)
		}
	}
}

func TestReplaceSheetReference(t *testing.T) {
	// Ten co nhay don: dang da boc nhay.
	if got := ReplaceSheetReference("'Trang 1'!A1", "Trang 1", "Trang 2"); got != "'Trang 2'!A1" {
		t.Fatalf("thay ten co nhay sai: %q", got)
	}
	// Ten don gian, khong nhay.
	if got := ReplaceSheetReference("Data!A1+Data!B2", "Data", "Moi"); got != "Moi!A1+Moi!B2" {
		t.Fatalf("thay ten don gian sai: %q", got)
	}
	// Lookbehind: khong duoc thay khi nam trong mot ma dinh danh dai hon.
	if got := ReplaceSheetReference("xData!A1", "Data", "Moi"); got != "xData!A1" {
		t.Fatalf("lookbehind sai: %q", got)
	}
	// Dang co nhay duoc thay bang ten moi KHONG nhay khi ten moi don gian (giong ban C#).
	if got := ReplaceSheetReference("'Data'!A1", "Data", "Moi"); got != "Moi!A1" {
		t.Fatalf("dang co nhay sai: %q", got)
	}
	// Ten moi can nhay thi van duoc boc.
	if got := ReplaceSheetReference("'Data'!A1", "Data", "Trang 2"); got != "'Trang 2'!A1" {
		t.Fatalf("ten moi can nhay sai: %q", got)
	}
}

func TestIsDateFormat(t *testing.T) {
	dates := []string{"yyyy-mm-dd", "dd/mm/yyyy", "h:mm:ss", "mmm-yy", "yyyy;[Red]0.00"}
	for _, code := range dates {
		if !IsDateFormat(code) {
			t.Fatalf("%q phai la dinh dang ngay", code)
		}
	}
	// "General" va cac dinh dang so khong tinh la ngay. Phan sau dau ';' dau tien bi bo qua
	// (giong is_date_format cua openpyxl), nen "0.00;[Red]yyyy" KHONG phai dinh dang ngay.
	notDates := []string{"", "General", "0.00", "#,##0", `"ngay" 0.00`, "[Red]0.00", `\d0`, "0.00;[Red]yyyy"}
	for _, code := range notDates {
		if IsDateFormat(code) {
			t.Fatalf("%q khong duoc tinh la dinh dang ngay", code)
		}
	}
}
