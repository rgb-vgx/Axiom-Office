package tools

import (
	"strings"
	"testing"
)

func TestRefHasRelativeRow(t *testing.T) {
	cases := map[string]bool{"D5": true, "$D5": true, "D$5": false, "$D$5": false, "A$1:B2": true, "Data!$A$1:$A$9": false,
		"A:C": false, "3:5": true, "$3:$5": false}
	for text, want := range cases {
		refs, _ := ParseFormulaRefs("=" + text)
		if len(refs) != 1 {
			t.Fatalf("%s: khong tach duoc", text)
		}
		if got := refHasRelativeRow(refs[0]); got != want {
			t.Errorf("refHasRelativeRow(%s) = %v, muon %v", text, got, want)
		}
	}
}

func TestSortGuard(t *testing.T) {
	cells := map[string]fakeCell{
		"Data!A1": {"", "Tên"}, "Data!B1": {"", "Doanh thu"}, "Data!C1": {"", "Thuế"},
		"Data!A2": {"", "An"}, "Data!B2": {"", 100.0}, "Data!C2": {"=B2*$F$1", 10.0},
		"Data!A3": {"", "Bình"}, "Data!B3": {"", 50.0}, "Data!C3": {"=B3*$F$1", 5.0},
	}
	run, _ := fakeWorkbookBridge(t, cells, nil)
	params := map[string]any{"range": "Data!A1:C3", "keys": []any{map[string]any{"column": "Doanh thu"}}}

	// Du lieu co san: hoi, noi ro Excel khong hoan tac duoc.
	reason := sortGuardReason(t.Context(), run, params)
	if !strings.Contains(reason, "Sắp xếp lại 9 ô") || strings.Contains(reason, "trỏ nhầm") {
		t.Fatalf("du lieu co san phai hoi (cong thuc $F$1 tuyet doi la an toan): %q", reason)
	}
	// Du lieu do chinh luot ghi: khong hoi.
	run.WrittenCells = map[string]map[string]bool{"Data": {}}
	for _, cell := range []string{"A1", "B1", "C1", "A2", "B2", "C2", "A3", "B3", "C3"} {
		run.WrittenCells["Data"][cell] = true
	}
	if reason := sortGuardReason(t.Context(), run, params); reason != "" {
		t.Fatalf("o do luot ghi, cong thuc an toan: khong hoi, co %q", reason)
	}
	// Cong thuc tham chieu ra ngoai vung theo hang tuong doi (E2 nam ngoai A:C): hoi du la o cua luot.
	cells["Data!C2"] = fakeCell{"=B2*E2", 0.0}
	if reason := sortGuardReason(t.Context(), run, params); !strings.Contains(reason, "C2 =B2*E2") {
		t.Fatalf("cong thuc =B2*E2 se tro nham hang sau khi sap xep: %q", reason)
	}
}
