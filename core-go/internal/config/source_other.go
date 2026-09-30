//go:build !windows

package config

// DefaultSource: Linux/macOS dung ~/.config/axiom-office/config.json (dung chung voi extension LibreOffice).
func DefaultSource() Source { return JSONSource{Path: DefaultJSONPath()} }
