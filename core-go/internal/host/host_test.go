package host

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"axiomoffice/core/internal/office"
)

func TestKindFamilyAndProfileForEveryApp(t *testing.T) {
	cases := []struct {
		app     string
		kind    Kind
		family  Family
		appName string
	}{
		{"wps", KindWPS, FamilyWord, "Microsoft Word / WPS Writer"},
		{"et", KindET, FamilySheet, "Microsoft Excel / WPS Spreadsheets"},
		{"wpp", KindWPP, FamilySlides, "Microsoft PowerPoint / WPS Presentation"},
		{"word", KindWord, FamilyWord, "Microsoft Word / WPS Writer"},
		{"excel", KindExcel, FamilySheet, "Microsoft Excel / WPS Spreadsheets"},
		{"ppt", KindPPT, FamilySlides, "Microsoft PowerPoint / WPS Presentation"},
	}
	for _, c := range cases {
		t.Run(c.app, func(t *testing.T) {
			if got := KindFromApp(c.app); got != c.kind {
				t.Fatalf("KindFromApp(%q) = %q, want %q", c.app, got, c.kind)
			}
			if got := FamilyOf(c.kind); got != c.family {
				t.Fatalf("FamilyOf(%q) = %q, want %q", c.kind, got, c.family)
			}
			profile := NewHostProfile(c.kind)
			if profile.Kind != c.kind || profile.Family != c.family {
				t.Fatalf("profile identity = %+v", profile)
			}
			if profile.AppName != c.appName {
				t.Fatalf("AppName = %q, want %q", profile.AppName, c.appName)
			}
			if strings.TrimSpace(profile.CitationRule) == "" {
				t.Fatal("CitationRule is empty")
			}
			families := profile.DefaultToolFamilies()
			if len(families) != 1 || families[0] != c.family {
				t.Fatalf("default tool families = %v, want [%s]", families, c.family)
			}
			if got := ProfileForApp(c.app).Kind; got != c.kind {
				t.Fatalf("ProfileForApp(%q).Kind = %q", c.app, got)
			}
		})
	}
}

func TestKindFromAppAliasesAndUnknown(t *testing.T) {
	cases := map[string]Kind{
		"  WPS  ":      KindWPS,
		"writer":       KindWPS,
		"WINWORD.EXE":  KindWord,
		"Excel.exe":    KindExcel,
		"powerpoint":   KindPPT,
		"powerpnt.exe": KindPPT,
		"spreadsheets": KindET,
		"presentation": KindWPP,
		"":             "",
		"notepad":      "",
		"libreoffice":  "",
	}
	for app, want := range cases {
		if got := KindFromApp(app); got != want {
			t.Errorf("KindFromApp(%q) = %q, want %q", app, got, want)
		}
	}
	if got := FamilyOf(Kind("unknown")); got != "" {
		t.Fatalf("FamilyOf(unknown) = %q, want empty", got)
	}
	profile := NewHostProfile(Kind("unknown"))
	if profile.AppName != "" || profile.CitationRule != "" || len(profile.ToolFamilies) != 0 {
		t.Fatalf("profile for unknown kind = %+v", profile)
	}
}

func TestParseCatalogMissingCatalog(t *testing.T) {
	for _, name := range []string{"nil", "empty"} {
		t.Run(name, func(t *testing.T) {
			var entries []CatalogEntry
			if name == "empty" {
				entries = []CatalogEntry{}
			}
			caps := ParseCatalog(entries)
			if caps.Commands == nil || caps.Features == nil {
				t.Fatalf("missing catalog returned nil maps: %+v", caps)
			}
			if len(caps.Commands) != 0 || len(caps.Features) != 0 {
				t.Fatalf("missing catalog returned capabilities: %+v", caps)
			}
			if caps.HasCommand("et.readRange") || caps.HasFeature("et.readRange") {
				t.Fatal("empty catalog must not report commands")
			}
		})
	}
}

func TestParseCatalogCommandsAndAgentCommands(t *testing.T) {
	caps := ParseCatalog([]CatalogEntry{
		{Name: "et.readRange", Agent: true},
		{Name: "et.writeRange", Agent: true},
		{Name: "app.info"},
		{Name: "  "},
		{Name: "et.readRange", Agent: true},
	})
	if len(caps.Commands) != 3 {
		t.Fatalf("commands = %v", caps.Commands)
	}
	if !caps.HasCommand("et.readRange") || !caps.HasCommand("app.info") {
		t.Fatalf("missing command: %v", caps.Commands)
	}
	if caps.HasCommand("") {
		t.Fatal("blank command name must be skipped")
	}
	if len(caps.Features) != 2 || !caps.HasFeature("et.readRange") || !caps.HasFeature("et.writeRange") {
		t.Fatalf("agent commands = %v", caps.Features)
	}
	if caps.HasFeature("app.info") {
		t.Fatal("non-agent command must not be a feature")
	}
}

func TestOfficeHostWrapsSessionAndBridge(t *testing.T) {
	var commandsHit, executeHit int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/commands":
			commandsHit++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": map[string]any{"version": "1", "commands": []any{
					map[string]any{"name": "et.readRange", "kind": "et", "agent": true, "summary": "read"},
					map[string]any{"name": "app.info", "agent": false, "params": []any{
						map[string]any{"name": "path", "required": true, "hint": "file"},
					}},
				}},
			})
		case "/cmd":
			executeHit++
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"values": []any{}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	port := portOf(t, server.URL)

	session := office.Session{
		App: "et", Family: "libreoffice", Port: port,
		Document: "bao-cao.xlsx", DocumentPath: "/work/bao-cao.xlsx",
	}
	client := office.NewBridgeClient(server.Client(), "tok")
	h := NewOfficeHost(client, session)

	if h.Kind() != KindET || h.Family() != FamilySheet {
		t.Fatalf("kind/family = %q/%q", h.Kind(), h.Family())
	}
	document := h.Document()
	if document.Name != "bao-cao.xlsx" || document.Path != "/work/bao-cao.xlsx" ||
		document.Key != office.DocumentKey("/work/bao-cao.xlsx") {
		t.Fatalf("document = %+v", document)
	}
	catalog := h.Catalog()
	if len(catalog) != 2 || catalog[0].Name != "et.readRange" || !catalog[0].Agent ||
		catalog[1].Name != "app.info" || len(catalog[1].Params) != 1 ||
		catalog[1].Params[0] != (Param{Name: "path", Required: true, Hint: "file"}) {
		t.Fatalf("catalog = %+v", catalog)
	}
	caps := h.Capabilities()
	if !caps.HasCommand("et.readRange") || !caps.HasFeature("et.readRange") || caps.HasFeature("app.info") {
		t.Fatalf("capabilities = %+v", caps)
	}
	if commandsHit != 1 {
		t.Fatalf("catalog fetched %d times, want 1 (cached)", commandsHit)
	}

	result, err := h.Execute(context.Background(), "et.readRange", map[string]any{"range": "A1"})
	if err != nil || result == nil {
		t.Fatalf("Execute = %v, %v", result, err)
	}
	if _, ok := result["values"]; !ok || executeHit != 1 {
		t.Fatalf("Execute result = %v (hits %d)", result, executeHit)
	}
}

func TestOfficeHostExecuteReportsBridgeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "no such command"})
	}))
	defer server.Close()
	h := NewOfficeHost(office.NewBridgeClient(server.Client(), ""), office.Session{App: "wpp", Port: portOf(t, server.URL)})
	if _, err := h.Execute(context.Background(), "wpp.nope", nil); err == nil || !strings.Contains(err.Error(), "no such command") {
		t.Fatalf("Execute error = %v", err)
	}
	missing := NewOfficeHost(nil, office.Session{})
	if _, err := missing.Execute(context.Background(), "app.info", nil); err == nil {
		t.Fatal("Execute without a bridge client must fail")
	}
}

func TestWPSHostWithoutClientIsEmpty(t *testing.T) {
	h := NewWPSHost(nil)
	if h.Kind() != "" || h.Family() != "" || len(h.Catalog()) != 0 {
		t.Fatalf("empty WPS host = %+v", h)
	}
	if document := h.Document(); document != (Document{}) {
		t.Fatalf("empty WPS document = %+v", document)
	}
	if caps := h.Capabilities(); len(caps.Commands) != 0 || len(caps.Features) != 0 {
		t.Fatalf("empty WPS capabilities = %+v", caps)
	}
}

func TestCatalogFromWPSKeepsUndoAndParams(t *testing.T) {
	entries := catalogFromWPS([]any{
		map[string]any{
			"name": "wpp.addSlide", "kind": "wpp", "agent": true, "undo": true, "summary": "add",
			"params": []any{map[string]any{"name": "index", "required": true, "hint": "1-based"}},
		},
		"not a map",
	})
	if len(entries) != 1 || entries[0].Name != "wpp.addSlide" || !entries[0].Agent || !entries[0].Undo ||
		len(entries[0].Params) != 1 || entries[0].Params[0].Hint != "1-based" {
		t.Fatalf("catalogFromWPS = %+v", entries)
	}
}

func portOf(t *testing.T, raw string) int {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	for _, digit := range parsed.Port() {
		port = port*10 + int(digit-'0')
	}
	return port
}
