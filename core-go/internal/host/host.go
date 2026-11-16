// Package host defines the app-independent seam for a document host that is
// currently open: its identity, capabilities, catalog, current document and
// command execution.
//
// Phase 1 keeps existing call sites on office/wpsbridge. The intended use is:
//
//	h := host.NewOfficeHost(bridge, session)
//	caps := h.Capabilities()
//	if caps.HasCommand("et.readRange") {
//		result, err := h.Execute(ctx, "et.readRange", map[string]any{"range": "A1:C10"})
//		_ = result
//		_ = err
//	}
//
// Nothing here imports agent or api, so the package can later be used by both
// without creating an import cycle.
package host

import (
	"context"
	"strings"
)

// Kind is the app kind reported by a live bridge. WPS uses wps/et/wpp; Microsoft
// Office uses word/excel/ppt.
type Kind string

const (
	KindWPS   Kind = "wps"
	KindET    Kind = "et"
	KindWPP   Kind = "wpp"
	KindWord  Kind = "word"
	KindExcel Kind = "excel"
	KindPPT   Kind = "ppt"
)

// Family is the document family shared by the Office and WPS app of a kind.
type Family string

const (
	FamilyWord   Family = "word"
	FamilySheet  Family = "sheet"
	FamilySlides Family = "slides"
)

// Limits describes per-host response limits. Zero means the host has no
// specific limit and callers keep their existing defaults.
type Limits struct {
	MaxCells  int
	MaxRows   int
	MaxChars  int
	MaxSlides int
}

// Capabilities is the capability lookup table for one host snapshot.
type Capabilities struct {
	// Commands holds every command advertised by the host catalog.
	Commands map[string]bool
	// Features holds the commands the host marks as safe for the agent.
	Features map[string]bool
	Limits   Limits
}

// HasCommand reports whether the host advertised name.
func (c Capabilities) HasCommand(name string) bool { return c.Commands[name] }

// HasFeature reports whether the host advertised name as an agent command.
func (c Capabilities) HasFeature(name string) bool { return c.Features[name] }

// Param describes one command parameter.
type Param struct {
	Name     string
	Required bool
	Hint     string
}

// CatalogEntry is one command in a host catalog. Undo marks commands whose
// effect can be undone by the host.
type CatalogEntry struct {
	Name    string
	Kind    string
	Agent   bool
	Undo    bool
	Summary string
	Params  []Param
}

// Document identifies the document currently open in a host.
type Document struct {
	Key   string
	Name  string
	Path  string
	Slide int
}

// Host is the behavior the rest of Core needs from an open document app.
type Host interface {
	Kind() Kind
	Family() Family
	Capabilities() Capabilities
	Catalog() []CatalogEntry
	Document() Document
	Execute(ctx context.Context, action string, params map[string]any) (map[string]any, error)
}

// ParseCatalog turns a host catalog into a capability lookup table. Every
// catalog entry becomes a command; entries marked Agent also become features.
// A missing or empty catalog returns initialized, empty maps rather than nil.
func ParseCatalog(entries []CatalogEntry) Capabilities {
	caps := Capabilities{Commands: map[string]bool{}, Features: map[string]bool{}}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		caps.Commands[name] = true
		if entry.Agent {
			caps.Features[name] = true
		}
	}
	return caps
}

// FamilyOf returns the document family of a kind. Unknown kinds return "".
func FamilyOf(kind Kind) Family {
	switch kind {
	case KindWPS, KindWord:
		return FamilyWord
	case KindET, KindExcel:
		return FamilySheet
	case KindWPP, KindPPT:
		return FamilySlides
	}
	return ""
}

// KindFromApp maps an app name reported by a bridge or API to a Kind. It accepts
// the six canonical names, common aliases such as writer/powerpoint, and Windows
// process names with an .exe suffix. Unknown names return "".
func KindFromApp(app string) Kind {
	normalized := strings.ToLower(strings.TrimSpace(app))
	normalized = strings.TrimSuffix(normalized, ".exe")
	switch normalized {
	case "wps", "writer", "wpswriter", "wps writer":
		return KindWPS
	case "et", "spreadsheet", "spreadsheets":
		return KindET
	case "wpp", "presentation":
		return KindWPP
	case "word", "winword", "doc":
		return KindWord
	case "excel", "sheet":
		return KindExcel
	case "ppt", "powerpoint", "powerpnt", "slides":
		return KindPPT
	}
	return ""
}

// HostProfile carries the stable prompt fragments and default tool family for a
// host kind. AppName and CitationRule mirror the existing agent prompt so
// moving them behind this seam does not change behavior.
type HostProfile struct {
	Kind         Kind
	Family       Family
	AppName      string
	CitationRule string
	// ToolFamilies lists the tool families a host of this kind enables by
	// default.
	ToolFamilies []Family
}

// NewHostProfile returns the profile for kind. Unknown kinds get an empty
// profile so callers can detect them instead of silently guessing.
func NewHostProfile(kind Kind) HostProfile {
	profile := HostProfile{Kind: kind, Family: FamilyOf(kind)}
	switch profile.Family {
	case FamilyWord:
		profile.AppName = "Microsoft Word / WPS Writer"
		profile.CitationRule = "Citations: when your answer points to specific places in the document, cite them as [§n], where n is the " +
			"paragraph number (field \"n\") from writer.outline; the user can click it to jump there. Cite only numbers you saw."
	case FamilySheet:
		profile.AppName = "Microsoft Excel / WPS Spreadsheets"
		profile.CitationRule = "Citations: when your answer points to specific cells, cite them as [Sheet!A1] or [Sheet!A1:B5] (the user " +
			"can click them to jump there). Cite only cells you actually read."
	case FamilySlides:
		profile.AppName = "Microsoft PowerPoint / WPS Presentation"
		profile.CitationRule = "Citations: when your answer points to a specific slide, cite it as [slide n] (the user can click it to jump " +
			"there). Cite only slides you actually read."
	default:
		return profile
	}
	profile.ToolFamilies = []Family{profile.Family}
	return profile
}

// ProfileFor is a short alias for NewHostProfile.
func ProfileFor(kind Kind) HostProfile { return NewHostProfile(kind) }

// ProfileForApp maps an app name to its HostProfile.
func ProfileForApp(app string) HostProfile { return NewHostProfile(KindFromApp(app)) }

// DefaultToolFamilies returns a copy of the profile's default tool families.
func (p HostProfile) DefaultToolFamilies() []Family {
	return append([]Family(nil), p.ToolFamilies...)
}
