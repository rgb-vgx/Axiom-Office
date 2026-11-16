package config

import (
	"encoding/json"
	"strings"
)

// ExperimentalSource: nguon cau hinh doc duoc co experimental.<name> (JSON tren Linux, HKCU tren
// Windows). Khai bao rieng thay vi them vao Source de khong phai sua moi ban gia cua Source trong test.
type ExperimentalSource interface {
	Experimental() (map[string]bool, bool)
}

const experimentalKey = "Experimental"
const experimentalPrefix = "experimental."

// experimental: doc co experimental.<name> tu nguon nen tang. Nguon khong ho tro ban do thi lui ve
// khoa "Experimental" dang chuoi phan cach bang dau phay (vd "foo,-bar").
func experimental(source Source) map[string]bool {
	if source == nil {
		return nil
	}
	if typed, ok := source.(ExperimentalSource); ok {
		flags, found := typed.Experimental()
		if !found || len(flags) == 0 {
			return nil
		}
		return flags
	}
	if raw, ok := source.String(experimentalKey); ok {
		if flags := experimentalList(raw); len(flags) > 0 {
			return flags
		}
	}
	return nil
}

// experimentalList: "foo, -bar" -> foo=true, bar=false. Muc "all" khong co nghia trong cau hinh
// (khong liet ke duoc ten), nen bo qua.
func experimentalList(raw string) map[string]bool {
	flags := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		enabled := true
		switch {
		case strings.HasPrefix(token, "-"):
			enabled = false
			token = strings.TrimSpace(token[1:])
		case strings.HasPrefix(token, "+"):
			token = strings.TrimSpace(token[1:])
		}
		if token == "" || strings.EqualFold(token, "all") {
			continue
		}
		flags[token] = enabled
	}
	return flags
}

// experimentalJSONValue: gia tri cua khoa "Experimental" trong config.json. Ho tro ca ban do
// {"foo": true}, mang ten ["foo"], va chuoi "foo,-bar".
func experimentalJSONValue(raw json.RawMessage) (map[string]bool, bool) {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		flags := map[string]bool{}
		for name, item := range typed {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if enabled, ok := experimentalBool(item); ok {
				flags[name] = enabled
			}
		}
		return flags, true
	case []any:
		flags := map[string]bool{}
		for _, item := range typed {
			if text, ok := item.(string); ok {
				mergeExperimental(flags, experimentalList(text))
			}
		}
		return flags, true
	case string:
		return experimentalList(typed), true
	default:
		return nil, false
	}
}

func experimentalBool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case float64:
		return typed != 0, true
	case string:
		return experimentalBoolText(typed)
	default:
		return false, false
	}
}

// experimentalBoolText: cung tu vung voi config.flag de nguoi dung khong phai hoc hai kieu.
func experimentalBoolText(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func mergeExperimental(into, from map[string]bool) {
	for name, enabled := range from {
		into[name] = enabled
	}
}
