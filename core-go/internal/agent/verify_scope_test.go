package agent

import (
	"strings"
	"testing"

	"axiomoffice/core/internal/model"
)

// Spec (log 11/10/2026, sheet ADT that cua nguoi dung): quy tac "doi thu mot o dau vao roi dat lai" lam
// agent GHI vao du lieu goc de kiem chung - ADT!V40 (+1.000.000) va ADT!S3 (5.368.534 -> 6.368.534) roi
// moi ghi lai. Luot bi dung giua hai lan ghi la so lieu to khai sai ma khong ai biet.
//
// Quy tac phai GIOI HAN: chi duoc thu voi o dau vao do chinh luot nay tao ra; khong bao gio ghi vao o
// ma nguoi dung da co du lieu truoc luot; muon biet cong thuc co noi vao du lieu san co thi doc cong
// thuc (et.readRange voi formulas:true) chu khong doi gia tri.
func TestVerifyByChangingInputIsLimitedToInputsCreatedThisTurn(t *testing.T) {
	for name, text := range map[string]string{"CheckRule": CheckRule, "VerifyWorkNudge": model.VerifyWorkNudge} {
		lower := strings.ToLower(text)
		for _, want := range []string{
			"created in this turn", // chi o do luot nay tao
			"never change",         // cam doi...
			"already had",          // ...du lieu nguoi dung da co
			"formulas",             // cach thay the: doc cong thuc
		} {
			if !strings.Contains(lower, want) {
				t.Errorf("%s phai chua %q (gioi han kiem chung bang cach doi o):\n%s", name, want, text)
			}
		}
	}
}

// Prompt he thong (Build) phai mang quy tac da gioi han - khong duoc con cau cu khong dieu kien.
func TestSystemPromptCarriesTheLimitedRule(t *testing.T) {
	if !strings.Contains(strings.ToLower(CheckRule), "created in this turn") {
		t.Skip("CheckRule chua gioi han - xem test tren")
	}
	old := "change one yourself, read back what should move, then put it back: only that tells a live input from a dead one."
	if strings.Contains(CheckRule, old) && !strings.Contains(strings.ToLower(CheckRule), "never change") {
		t.Fatalf("CheckRule con cau cu khong dieu kien: %q", old)
	}
}
