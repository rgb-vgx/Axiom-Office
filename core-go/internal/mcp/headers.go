package mcp

import (
	"os"
	"regexp"
)

// envReference: tham chieu bien moi truong trong gia tri header, vd "Bearer {env:MCP_TOKEN}".
var envReference = regexp.MustCompile(`\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// protocolHeaderNames: header giao thuc do transport tu dat; header nguoi dung trung ten bi bo qua de
// khong de len giao thuc (xem httpTransport.post).
var protocolHeaderNames = map[string]bool{
	"content-type":         true,
	"accept":               true,
	"mcp-protocol-version": true,
	"mcp-session-id":       true,
}

// resolveHeaders: thay {env:NAME} bang bien moi truong cua tien trinh Core va kiem tra ten/gia tri header.
// Loi KHONG chua gia tri header (co the la token) - chi ten header va ten bien moi truong.
func resolveHeaders(server string, headers map[string]string) (map[string]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	resolved := make(map[string]string, len(headers))
	for name, raw := range headers {
		if !validHeaderName(name) {
			return nil, fail("mcp server '%s': invalid header name %q", server, name)
		}
		value, missing := expandEnvReferences(raw)
		if missing != "" {
			return nil, fail("mcp server '%s': header %q references environment variable %q, which is not set",
				server, name, missing)
		}
		if !validHeaderValue(value) {
			return nil, fail("mcp server '%s': header %q has an invalid value (CR/LF or control characters are not allowed)",
				server, name)
		}
		resolved[name] = value
	}
	return resolved, nil
}

// expandEnvReferences: thay moi {env:NAME} bang gia tri bien moi truong; tra ve ten bien thieu dau tien (neu co).
func expandEnvReferences(value string) (string, string) {
	missing := ""
	expanded := envReference.ReplaceAllStringFunc(value, func(reference string) string {
		name := envReference.FindStringSubmatch(reference)[1]
		resolved, ok := os.LookupEnv(name)
		if !ok {
			if missing == "" {
				missing = name
			}
			return ""
		}
		return resolved
	})
	if missing != "" {
		return "", missing
	}
	return expanded, ""
}

// validHeaderName: token theo RFC 7230 (khong khoang trang, khong dau hai cham, khong CR/LF).
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '!' || c == '#' || c == '$' || c == '%' || c == '&' || c == '\'' || c == '*' ||
			c == '+' || c == '-' || c == '.' || c == '^' || c == '_' || c == '`' || c == '|' || c == '~':
		default:
			return false
		}
	}
	return true
}

// validHeaderValue: khong CR/LF, khong ky tu dieu khien khac tab (RFC 7230 field-value).
func validHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c == '\r' || c == '\n' || c == 0x7f || (c < 0x20 && c != '\t') {
			return false
		}
	}
	return true
}
