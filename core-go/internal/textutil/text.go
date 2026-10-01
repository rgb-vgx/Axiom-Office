// Package textutil: dem va cat chuoi theo don vi UTF-16 cho khop String.Length / Substring cua
// ban C# (tieng Viet la 1 don vi, ky tu ngoai BMP nhu emoji la 2).
package textutil

import "unicode/utf16"

// Length dem so don vi UTF-16.
func Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}

// Truncate cat con limit don vi UTF-16 dau tien.
func Truncate(text string, limit int) string {
	units := utf16.Encode([]rune(text))
	if limit >= len(units) {
		return text
	}
	if limit <= 0 {
		return ""
	}
	return string(utf16.Decode(units[:limit]))
}
