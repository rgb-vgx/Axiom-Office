//go:build windows

package config

import (
	"os"
	"strings"
	"testing"
)

// Khoa API cua nguoi dung nam trong HKCU dang "dpapi:<base64>" (add-in ghi bang ProtectedData, CurrentUser,
// khong entropy). Core Go phai doc lai duoc dung dinh dang do - neu khong thi moi cau hinh da luu deu
// thanh "chua co khoa API" va nguoi dung phai nhap lai.
func TestDpapiRoundTrip(t *testing.T) {
	secret := "sk-thu-nghiem-mot-khoa-dai-0123456789"
	protected := Protect(secret)
	if !strings.HasPrefix(protected, "dpapi:") {
		t.Fatalf("Protect phai tra ve dinh dang dpapi:..., nhan duoc %q", protected[:20])
	}
	if strings.Contains(protected, secret) {
		t.Fatal("ban ma hoa khong duoc chua khoa nguyen van")
	}
	if got := Unprotect(protected); got != secret {
		t.Fatalf("Unprotect(Protect(x)) = %q, mong doi %q", got, secret)
	}
}

// Dinh dang sai (file cua may khac, ma hoa hong, base64 hong) -> coi nhu chua cau hinh, KHONG lam Core dung.
func TestDpapiBadInput(t *testing.T) {
	for _, value := range []string{"dpapi:", "dpapi:khong-phai-base64!!", "dpapi:AAAA", "sk-plain"} {
		got := Unprotect(value)
		if value == "sk-plain" {
			if got != "sk-plain" {
				t.Fatalf("gia tri thuong phai giu nguyen: %q", got)
			}
			continue
		}
		if got != "" {
			t.Fatalf("Unprotect(%q) = %q, mong doi chuoi rong", value, got)
		}
	}
}

// Gia tri DPAPI do ban .NET tao ra (scripts: xem HUONG-DAN trong test) phai doc duoc.
// Chay tay khi can doi chieu: AXIOM_TEST_DPAPI_VALUE=<gia tri dpapi:... cua .NET> go test -run TestCrossDpapi
func TestCrossDpapi(t *testing.T) {
	value := os.Getenv("AXIOM_TEST_DPAPI_VALUE")
	if value == "" {
		t.Skip("dat AXIOM_TEST_DPAPI_VALUE de doi chieu voi ban .NET")
	}
	if got := Unprotect(value); got == "" {
		t.Fatal("khong giai ma duoc gia tri cua ban .NET")
	}
}
