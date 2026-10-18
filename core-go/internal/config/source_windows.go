//go:build windows

package config

import (
	"fmt"
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// DescribeSource mo ta nguon cau hinh nen tang de doctor in ra cho nguoi dung.
func DescribeSource() string { return `HKCU\` + RegistryKeyPath }

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

// Set ghi mot khoa vao HKCU\Software\AxiomOffice - cung cho voi add-in .NET va extension LibreOffice
// (hai ban kia ghi REG_SZ, nen ghi chuoi cho dong dang).
func (r RegistrySource) Set(name string, value any) error {
	text, ok := SettingText(value)
	if !ok {
		return fmt.Errorf("unsupported value type for setting %s", name)
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, r.KeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(name, text)
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
