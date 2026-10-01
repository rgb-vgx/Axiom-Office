package config

import "runtime"

// Cong bridge mac dinh cua tung ho ung dung (cung ten khoa voi add-in va extension).
const (
	DefaultBridgePortWPS         = 47821
	DefaultBridgePortOffice      = 47831
	DefaultBridgePortLibreOffice = 47851
)

// BridgePortForKind tra ve cong bridge cua mot ho ung dung, doc tu cung nguon cau hinh voi Core.
//
//	Windows: Port (WPS) hoac PortOffice (Microsoft Office).
//	Nen tang khac (Linux, extension LibreOffice): PortLibreOffice cho moi ho.
//
// "et" = +1, "wpp" = +2, con lai = cong goc (giong Config.PortForKind cua ban C#).
func BridgePortForKind(source Source, kind string, officeHost bool) int {
	var base int
	switch {
	case runtime.GOOS != "windows":
		base = intSetting(source, "PortLibreOffice", DefaultBridgePortLibreOffice)
	case officeHost:
		base = intSetting(source, "PortOffice", DefaultBridgePortOffice)
	default:
		base = intSetting(source, "Port", DefaultBridgePortWPS)
	}
	switch kind {
	case "et":
		return base + 1
	case "wpp":
		return base + 2
	}
	return base
}

func intSetting(source Source, name string, fallback int) int {
	if source == nil {
		return fallback
	}
	if value, ok := source.Int(name); ok {
		return value
	}
	return fallback
}
