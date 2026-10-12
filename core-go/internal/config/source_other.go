//go:build !windows

package config

import "os"

// DefaultSource: Linux/macOS dung ~/.config/axiom-office/config.json (dung chung voi extension LibreOffice).
func DefaultSource() Source { return JSONSource{Path: DefaultJSONPath()} }

// DescribeSource mo ta nguon cau hinh nen tang de doctor in ra cho nguoi dung.
func DescribeSource() string {
	path := DefaultJSONPath()
	if _, err := os.Stat(path); err == nil {
		return path + " (ton tai)"
	}
	return path + " (khong ton tai)"
}
