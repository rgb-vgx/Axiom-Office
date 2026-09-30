//go:build !windows

package config

// Khong co DPAPI ngoai Windows: khoa "dpapi:" tu may Windows khac coi nhu chua cau hinh.
func dpapiUnprotect(string) string { return "" }

// Protect: Linux luu thuong (config.json 0600 hoac keyring do extension ghi).
func Protect(value string) string { return value }
