package policy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cung bang ca voi PolicyTests.cs cua ban .NET (muc 8.6): lenh rui ro nao phai hoi nguoi dung.
func evaluate(action string, params map[string]any, prompt string, length *int) Decision {
	return Evaluate(context.Background(), action, params, prompt, func(context.Context) *int { return length })
}

func TestSaveOrExportWithoutUserIntentAsks(t *testing.T) {
	for _, action := range []string{"writer.save", "et.saveAs", "wpp.exportPdf", "writer.exportPdf", "et.save"} {
		if !IsSaveOrExport(action) {
			t.Errorf("%s phai la lenh luu/xuat", action)
		}
		decision := evaluate(action, map[string]any{"path": `C:\khong-ton-tai\x.docx`}, "Tạo bảng điểm", nil)
		if !decision.NeedsConfirmation || !strings.Contains(decision.Reason, "lưu") {
			t.Errorf("%s: phai hoi va ly do nhac den viec luu, nhan duoc %+v", action, decision)
		}
	}

	for _, action := range []string{"et.writeRange", "writer.insertStyledText", "wpp.addSlide", "writer.getText"} {
		if IsSaveOrExport(action) {
			t.Errorf("%s khong phai lenh luu/xuat", action)
		}
		if decision := evaluate(action, nil, "Tạo bảng điểm", nil); decision.NeedsConfirmation {
			t.Errorf("%s: khong duoc hoi, nhan duoc %+v", action, decision)
		}
	}
}

func TestUserIntentToSaveSkipsQuestion(t *testing.T) {
	cases := []struct {
		action, prompt string
	}{
		{"writer.save", "Lưu file giúp mình"},
		{"et.saveAs", "save as bang-diem.xlsx"},
		{"wpp.exportPdf", "Xuất PDF bài này"},
		{"writer.exportPdf", "xuat pdf"},
		{"et.save", "ghi ra file đi"},
		{"writer.save", "save it please"},
	}
	for _, item := range cases {
		if !PromptAsksToSave(item.prompt) {
			t.Errorf("%q phai duoc coi la co y dinh luu/xuat", item.prompt)
		}
		decision := evaluate(item.action, map[string]any{"path": filepath.Join(t.TempDir(), "moi.pdf")}, item.prompt, nil)
		if decision.NeedsConfirmation {
			t.Errorf("%s + %q: nguoi dung da yeu cau luu thi khong hoi, nhan duoc %+v", item.action, item.prompt, decision)
		}
	}

	for _, prompt := range []string{"Tạo bảng điểm", "Soạn công văn", "Làm slide báo cáo"} {
		if PromptAsksToSave(prompt) {
			t.Errorf("%q khong duoc coi la y dinh luu", prompt)
		}
	}
}

func TestOverwriteExistingFileAsksEvenWithIntent(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "da-co.docx")
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	decision := evaluate("writer.saveAs", map[string]any{"path": existing}, "Lưu thành file khác", nil)
	if !decision.NeedsConfirmation || !strings.Contains(decision.Reason, "Ghi đè") {
		t.Fatalf("ghi de file da co phai hoi: %+v", decision)
	}

	// File chua ton tai thi khong hoi (da co y dinh luu).
	fresh := evaluate("writer.saveAs", map[string]any{"path": filepath.Join(t.TempDir(), "moi.docx")}, "Lưu thành file khác", nil)
	if fresh.NeedsConfirmation {
		t.Fatalf("tao file moi khong can hoi: %+v", fresh)
	}
}

func TestDeleteSlideAndLargeReplaceAllAsk(t *testing.T) {
	if decision := evaluate("wpp.deleteSlide", nil, "tạo bảng", nil); !decision.NeedsConfirmation {
		t.Fatal("wpp.deleteSlide phai hoi")
	}

	long := LargeDocumentChars + 1
	decision := evaluate("writer.replaceAll", nil, "tạo bảng", &long)
	if !decision.NeedsConfirmation || !strings.Contains(decision.Reason, "20,001 ký tự") {
		t.Fatalf("replaceAll tren tai lieu dai phai hoi kem so ky tu: %+v", decision)
	}

	short := 500
	if decision := evaluate("writer.replaceAll", nil, "tạo bảng", &short); decision.NeedsConfirmation {
		t.Fatalf("tai lieu ngan khong hoi: %+v", decision)
	}
	// Khong doc duoc do dai tai lieu -> khong hoi (tha cho chay con hon chan nham).
	if decision := evaluate("writer.replaceAll", nil, "tạo bảng", nil); decision.NeedsConfirmation {
		t.Fatalf("khong biet do dai thi khong hoi: %+v", decision)
	}
}

// Chi writer.replaceAll moi can do dai tai lieu: cac lenh khac khong duoc goi bridge chi de kiem tra.
func TestDocumentLengthOnlyForReplaceAll(t *testing.T) {
	calls := 0
	length := 100
	Evaluate(context.Background(), "et.writeRange", nil, "tạo bảng", func(context.Context) *int {
		calls++
		return &length
	})
	if calls != 0 {
		t.Fatalf("et.writeRange khong duoc hoi do dai tai lieu (goi %d lan)", calls)
	}

	Evaluate(context.Background(), "writer.replaceAll", nil, "tạo bảng", func(context.Context) *int {
		calls++
		return &length
	})
	if calls != 1 {
		t.Fatalf("writer.replaceAll phai hoi do dai dung mot lan (goi %d lan)", calls)
	}
}
