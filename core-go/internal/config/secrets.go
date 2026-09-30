package config

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Khoa API trong cau hinh (ban Go cua Secrets.cs):
//   - "dpapi:<base64>": add-in Windows ma hoa DPAPI (CurrentUser, khong entropy) - chi giai duoc tren Windows.
//   - "libsecret:<ten khoa>": extension LibreOffice luu trong keyring qua secret-tool (Linux).
//   - con lai: gia tri thuong (config.json 0600).
//
// Giai ma that bai -> "" (coi nhu chua cau hinh), khong lam Core dung.
const (
	dpapiPrefix     = "dpapi:"
	libsecretPrefix = "libsecret:"
	serviceName     = "axiom-office"
)

// Unprotect tra ve khoa that tu gia tri luu trong cau hinh.
func Unprotect(raw string) string {
	switch {
	case raw == "":
		return ""
	case strings.HasPrefix(raw, libsecretPrefix):
		return libsecretLookup(strings.TrimPrefix(raw, libsecretPrefix))
	case strings.HasPrefix(raw, dpapiPrefix):
		return dpapiUnprotect(strings.TrimPrefix(raw, dpapiPrefix))
	default:
		return raw
	}
}

func libsecretLookup(key string) string {
	if runtime.GOOS == "windows" || strings.TrimSpace(key) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "secret-tool", "lookup", "service", serviceName, "key", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\r\n")
}
