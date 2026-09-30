//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Khoa API tren Linux nam trong keyring duoi dang "libsecret:<ten khoa>"; Core doc lai bang `secret-tool`.
// Dung secret-tool GIA trong PATH (cung cach bo test cua ban .NET) de khong dung keyring that cua nguoi chay.
func TestLibsecretLookupWithFakeTool(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "secret-tool")
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"lookup\" ] && [ \"$2\" = \"service\" ] && [ \"$3\" = \"axiom-office\" ] && [ \"$4\" = \"key\" ] && [ \"$5\" = \"LlmApiKey\" ]; then\n" +
		"  echo sk-tu-keyring\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo 'khong tim thay' >&2\n" +
		"exit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if got := Unprotect("libsecret:LlmApiKey"); got != "sk-tu-keyring" {
		t.Fatalf("Unprotect(libsecret:LlmApiKey) = %q, mong doi lay tu secret-tool", got)
	}
	// Keyring khong co / secret-tool loi -> coi nhu chua cau hinh (chuoi rong), khong lam Core dung.
	if got := Unprotect("libsecret:KhongCoTrongKeyring"); got != "" {
		t.Fatalf("khoa khong co trong keyring phai tra ve rong, nhan duoc %q", got)
	}

	// Doc tu config.json: gia tri libsecret: phai duoc thay bang khoa that tu keyring.
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"LlmApiKey":"libsecret:LlmApiKey","LlmModel":"m"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if cfg := From(env(nil), JSONSource{Path: path}); cfg.LlmApiKey != "sk-tu-keyring" || cfg.LlmModel != "m" {
		t.Fatalf("cau hinh doc tu config.json sai: khoa=%q model=%q", cfg.LlmApiKey, cfg.LlmModel)
	}
}
