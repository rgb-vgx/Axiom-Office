package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
