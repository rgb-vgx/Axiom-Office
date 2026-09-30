//go:build windows

package config

import (
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// RegistrySource doc HKCU\Software\AxiomOffice (dung chung voi add-in). Loi truy cap -> khong co gia tri.
type RegistrySource struct{ KeyPath string }

// DefaultSource: Windows dung HKCU nhu add-in.
func DefaultSource() Source { return RegistrySource{KeyPath: RegistryKeyPath} }

func (r RegistrySource) String(name string) (string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, r.KeyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	if text, _, err := key.GetStringValue(name); err == nil {
		return text, true
	}
	if number, _, err := key.GetIntegerValue(name); err == nil {
		return strconv.FormatUint(number, 10), true
	}
	return "", false
}

func (r RegistrySource) Int(name string) (int, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, r.KeyPath, registry.QUERY_VALUE)
	if err != nil {
		return 0, false
	}
	defer key.Close()
	if number, _, err := key.GetIntegerValue(name); err == nil {
		return int(int32(uint32(number))), true
	}
	if text, _, err := key.GetStringValue(name); err == nil {
		if value, err := strconv.Atoi(text); err == nil {
			return value, true
		}
	}
	return 0, false
}
