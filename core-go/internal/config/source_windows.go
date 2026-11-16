//go:build windows

package config

import (
	"fmt"
	"strconv"
	"strings"

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

// Experimental: co experimental.<name> trong HKCU\Software\AxiomOffice. Nhan khoa phang
// "experimental.foo" (REG_SZ/REG_DWORD) va khoa "Experimental" dang chuoi "foo,-bar", cung quy
// tac voi config.json.
func (r RegistrySource) Experimental() (map[string]bool, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, r.KeyPath, registry.QUERY_VALUE)
	if err != nil {
		return nil, false
	}
	defer key.Close()
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, false
	}
	flags := map[string]bool{}
	found := false
	for _, name := range names {
		lower := strings.ToLower(strings.TrimSpace(name))
		switch {
		case lower == "experimental":
			if text, _, err := key.GetStringValue(name); err == nil {
				parsed := experimentalList(text)
				if len(parsed) > 0 {
					found = true
					mergeExperimental(flags, parsed)
				}
			}
		case strings.HasPrefix(lower, experimentalPrefix):
			flagName := strings.TrimSpace(name[len(experimentalPrefix):])
			if flagName == "" {
				continue
			}
			if enabled, ok := registryBool(key, name); ok {
				flags[flagName] = enabled
				found = true
			}
		}
	}
	return flags, found
}

func registryBool(key registry.Key, name string) (bool, bool) {
	if text, _, err := key.GetStringValue(name); err == nil {
		return experimentalBoolText(text)
	}
	if number, _, err := key.GetIntegerValue(name); err == nil {
		return number != 0, true
	}
	return false, false
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
