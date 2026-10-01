package sheet

import (
	"testing"
)

func TestColumnIndexAndLetters(t *testing.T) {
	cases := []struct {
		letters string
		index   int
	}{
		{"A", 1}, {"Z", 26}, {"AA", 27}, {"AZ", 52}, {"BA", 53}, {"XFD", MaxColumns},
	}
	for _, item := range cases {
		if got := ColumnIndex(item.letters); got != item.index {
			t.Fatalf("ColumnIndex(%q) = %d, muon %d", item.letters, got, item.index)
		}
		if got := ColumnLetters(item.index); got != item.letters {
			t.Fatalf("ColumnLetters(%d) = %q, muon %q", item.index, got, item.letters)
		}
	}
	if got := Address(2, 2); got != "B2" {
		t.Fatalf("Address(2,2) = %q", got)
	}
	if got := Address(10, 28); got != "AB10" {
		t.Fatalf("Address(10,28) = %q", got)
	}
}

func TestParseCell(t *testing.T) {
	row, col, err := ParseCell("B2")
	if err != nil || row != 2 || col != 2 {
		t.Fatalf("ParseCell(B2) = %d,%d,%v", row, col, err)
	}
	// Dia chi tuyet doi van doc duoc.
	if row, col, err = ParseCell("$AB$10"); err != nil || row != 10 || col != 28 {
		t.Fatalf("ParseCell($AB$10) = %d,%d,%v", row, col, err)
	}
	if _, _, err := ParseCell("2B"); err == nil {
		t.Fatal("dia chi sai phai bao loi")
	}
	if _, _, err := ParseCell("A0"); err == nil {
		t.Fatal("dong 0 phai bao loi")
	}
	if _, _, err := ParseCell("XFE1"); err == nil {
		t.Fatal("cot vuot qua XFD phai bao loi")
	}
}

func TestParseRange(t *testing.T) {
	found, err := ParseRange("A1:C5")
	if err != nil {
		t.Fatalf("ParseRange loi: %v", err)
	}
	if *found.MinCol != 1 || *found.MinRow != 1 || *found.MaxCol != 3 || *found.MaxRow != 5 {
		t.Fatalf("A1:C5 sai: %+v", found)
	}
	// Mot o duy nhat.
	found, err = ParseRange("B2")
	if err != nil || *found.MinCol != 2 || *found.MaxCol != 2 || *found.MinRow != 2 || *found.MaxRow != 2 {
		t.Fatalf("B2 sai: %+v (%v)", found, err)
	}
	// Khong gioi han dong / cot, va bo qua ten sheet phia truoc.
	found, err = ParseRange("Data!A:C")
	if err != nil || found.MinRow != nil || found.MaxRow != nil || *found.MinCol != 1 || *found.MaxCol != 3 {
		t.Fatalf("Data!A:C sai: %+v (%v)", found, err)
	}
	if found, err = ParseRange("2:5"); err != nil || found.MinCol != nil || *found.MinRow != 2 || *found.MaxRow != 5 {
		t.Fatalf("2:5 sai: %+v (%v)", found, err)
	}
	for _, bad := range []string{"", "A1:B2:C3", "A1:"} {
		if _, err := ParseRange(bad); err == nil {
			t.Fatalf("vung %q phai bao loi", bad)
		}
	}
}

func TestParseBoundedRange(t *testing.T) {
	bounds, err := ParseBoundedRange("A1:D10")
	if err != nil || bounds != (Bounds{MinCol: 1, MinRow: 1, MaxCol: 4, MaxRow: 10}) {
		t.Fatalf("A1:D10 sai: %+v (%v)", bounds, err)
	}
	if _, err := ParseBoundedRange("A:C"); err == nil {
		t.Fatal("vung thieu canh phai bao loi")
	}
}

func TestSerialToIso(t *testing.T) {
	cases := []struct {
		serial   float64
		date1904 bool
		want     string
	}{
		{1, false, "1900-01-01T00:00:00"},
		{59, false, "1900-02-28T00:00:00"},
		// Excel co serial 60 = "1900-02-29" khong ton tai; DateTime.FromOADate cua .NET tra 1900-02-28.
		{60, false, "1900-02-28T00:00:00"},
		{61, false, "1900-03-01T00:00:00"},
		{45292, false, "2024-01-01T00:00:00"},
		{45292.5, false, "2024-01-01T12:00:00"},
		{0, true, "1904-01-01T00:00:00"},
	}
	for _, item := range cases {
		if got := SerialToIso(item.serial, item.date1904); got != item.want {
			t.Fatalf("SerialToIso(%v, %v) = %q, muon %q", item.serial, item.date1904, got, item.want)
		}
	}
	// He 1904 va he 1900 lech nhau dung 1462 ngay.
	if got, want := SerialToIso(43830, true), SerialToIso(45292, false); got != want {
		t.Fatalf("he 1904 lech: %q vs %q", got, want)
	}
	// Serial nho hon 1 (he 1900) la gio trong ngay.
	if got := SerialToIso(0.5, false); got != "12:00:00" {
		t.Fatalf("gio trong ngay sai: %q", got)
	}
}

func TestToText(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{nil, ""},
		{true, "True"},
		{false, "False"},
		{int64(42), "42"},
		{"xin chào", "xin chào"},
		{1.5, "1.5"},
		{2.0, "2"},
	}
	for _, item := range cases {
		if got := ToText(item.value); got != item.want {
			t.Fatalf("ToText(%v) = %q, muon %q", item.value, got, item.want)
		}
	}
}

func TestIsNumberAndAsFloat(t *testing.T) {
	if !IsNumber(int64(1)) || !IsNumber(1.5) {
		t.Fatal("so phai nhan ra la so")
	}
	if IsNumber("1") || IsNumber(nil) || IsNumber(true) {
		t.Fatal("chuoi/rong/bool khong phai so")
	}
	if value, ok := AsFloat(int64(3)); !ok || value != 3 {
		t.Fatalf("AsFloat(int64) sai: %v %v", value, ok)
	}
	if _, ok := AsFloat("3"); ok {
		t.Fatal("chuoi khong doi duoc thanh so")
	}
}
