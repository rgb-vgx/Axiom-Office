package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"axiomoffice/core/internal/filesafe"
)

// JSONSource doc ~/.config/axiom-office/config.json (Linux) - ban Go cua JsonConfigSource.cs. Doc lai moi lan
// hoi de doi cau hinh co hieu luc ngay; file thieu/hong -> coi nhu khong co gia tri.
type JSONSource struct{ Path string }

// DefaultJSONPath: $XDG_CONFIG_HOME/axiom-office/config.json (mac dinh ~/.config).
func DefaultJSONPath() string {
	root := os.Getenv("XDG_CONFIG_HOME")
	if strings.TrimSpace(root) == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".config")
	}
	return filepath.Join(root, "axiom-office", "config.json")
}

func (s JSONSource) String(name string) (string, bool) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return "", false
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(data, &values) != nil {
		return "", false
	}
	raw, ok := values[name]
	if !ok {
		return "", false
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		return typed, true
	case bool:
		if typed {
			return "1", true
		}
		return "0", true
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				parts = append(parts, text)
			} else if item != nil {
				encoded, _ := json.Marshal(item)
				parts = append(parts, string(encoded))
			}
		}
		return strings.Join(parts, ";"), true
	case map[string]any:
		return "", false
	default:
		// So: giu nguyen dang JSON (47845, 1.5) nhu JsonValue.ToJsonString() cua ban .NET.
		return strings.TrimSpace(string(raw)), true
	}
}

func (s JSONSource) Int(name string) (int, bool) {
	raw, ok := s.String(name)
	if !ok {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	return value, err == nil
}

// Experimental: co experimental.<name> trong config.json. Nhan ca ban do long nhau
// {"Experimental":{"foo":true}} va khoa phang {"experimental.foo":true} - hai cach viet cung
// mot y, de file cau hinh dung chung voi extension LibreOffice khong phai doi dinh dang.
func (s JSONSource) Experimental() (map[string]bool, bool) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, false
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(data, &values) != nil {
		return nil, false
	}
	flags := map[string]bool{}
	found := false
	for name, raw := range values {
		lower := strings.ToLower(strings.TrimSpace(name))
		switch {
		case lower == "experimental":
			parsed, ok := experimentalJSONValue(raw)
			if ok {
				found = true
				mergeExperimental(flags, parsed)
			}
		case strings.HasPrefix(lower, experimentalPrefix):
			flagName := strings.TrimSpace(name[len(experimentalPrefix):])
			if flagName == "" {
				continue
			}
			var value any
			if json.Unmarshal(raw, &value) != nil {
				continue
			}
			if enabled, ok := experimentalBool(value); ok {
				flags[flagName] = enabled
				found = true
			}
		}
	}
	return flags, found
}

// Set ghi mot khoa vao config.json, giu nguyen moi khoa khac (file dung chung voi extension
// LibreOffice nen khong duoc ghi de ca file). Ghi kieu JSON that (true/false, chuoi, so) - dung dang
// config.py:set_value da ghi, de hai ben doc lai nhu nhau. File hong -> ghi lai tu dau.
func (s JSONSource) Set(name string, value any) error {
	values := map[string]any{}
	if data, err := os.ReadFile(s.Path); err == nil {
		_ = json.Unmarshal(data, &values)
	}
	if values == nil {
		values = map[string]any{}
	}
	values[name] = value
	encoded, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	directory := filepath.Dir(s.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temp, err := filesafe.TempPathFor(s.Path)
	if err != nil {
		return err
	}
	// 0600: file co the chua khoa API (giong config.py cua extension LibreOffice).
	if err := os.WriteFile(temp, encoded, 0o600); err != nil {
		filesafe.DeleteQuietly(temp)
		return err
	}
	return filesafe.Replace(temp, s.Path)
}
