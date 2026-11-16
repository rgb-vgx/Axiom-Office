package host

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"axiomoffice/core/internal/office"
	"axiomoffice/core/internal/wpsbridge"
)

// OfficeHost adapts a live office.Session plus the office.BridgeClient used to
// talk to it. The same adapter serves the Windows COM bridge and the
// LibreOffice HTTP bridge because both expose the same /session, /commands and
// /cmd protocol.
type OfficeHost struct {
	client  *office.BridgeClient
	session office.Session
	kind    Kind

	mu      sync.Mutex
	loaded  bool
	catalog []CatalogEntry
}

// SessionHost is an alias kept for callers that name the adapter after the
// session it wraps.
type SessionHost = OfficeHost

var _ Host = (*OfficeHost)(nil)

// NewOfficeHost wraps an existing office.Session and BridgeClient.
func NewOfficeHost(client *office.BridgeClient, session office.Session) *OfficeHost {
	return &OfficeHost{client: client, session: session, kind: KindFromApp(session.App)}
}

// NewSessionHost is an alias for NewOfficeHost.
func NewSessionHost(client *office.BridgeClient, session office.Session) *OfficeHost {
	return NewOfficeHost(client, session)
}

// NewBridgeHost is an alias for NewOfficeHost.
func NewBridgeHost(client *office.BridgeClient, session office.Session) *OfficeHost {
	return NewOfficeHost(client, session)
}

func (h *OfficeHost) Kind() Kind { return h.kind }

func (h *OfficeHost) Family() Family { return FamilyOf(h.kind) }

// Catalog fetches and caches the command catalog for the session. A failed or
// missing fetch is retried on the next call.
func (h *OfficeHost) Catalog() []CatalogEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.loaded {
		return cloneCatalog(h.catalog)
	}
	if h.client == nil || h.session.Port <= 0 {
		return nil
	}
	catalog := h.client.GetCommandsFor(context.Background(), h.session.Port, string(h.kind), false)
	if catalog == nil {
		return nil
	}
	h.catalog = catalogFromOffice(catalog.Commands)
	h.loaded = true
	return cloneCatalog(h.catalog)
}

func (h *OfficeHost) Capabilities() Capabilities { return ParseCatalog(h.Catalog()) }

func (h *OfficeHost) Document() Document {
	return Document{
		Key:  office.DocumentKey(h.session.DocumentPath),
		Name: documentName(h.session.DocumentPath, h.session.Document),
		Path: h.session.DocumentPath,
	}
}

// Execute sends one command through the office bridge. Protocol-level failures
// are returned as errors; the bridge result map is returned unchanged on
// success.
func (h *OfficeHost) Execute(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	if h.client == nil {
		return nil, errors.New("host: no office bridge client")
	}
	if h.session.Port <= 0 {
		return nil, errors.New("host: office session has no port")
	}
	result := h.client.Command(ctx, h.session.Port, action, params)
	if result.OK {
		if result.Result == nil {
			return map[string]any{}, nil
		}
		return result.Result, nil
	}
	message := result.Error
	if message == "" {
		message = fmt.Sprintf("host: %s failed (HTTP %d)", action, result.Status)
	}
	return result.Result, errors.New(message)
}

// WPSHost adapts a wpsbridge.Client (one WPS app poller/port).
type WPSHost struct {
	client *wpsbridge.Client
}

// WPSClientHost is an alias kept for callers that name the adapter after the
// wpsbridge type it wraps.
type WPSClientHost = WPSHost

var _ Host = (*WPSHost)(nil)

// NewWPSHost wraps an existing wpsbridge.Client.
func NewWPSHost(client *wpsbridge.Client) *WPSHost {
	return &WPSHost{client: client}
}

// NewWPSClientHost is an alias for NewWPSHost.
func NewWPSClientHost(client *wpsbridge.Client) *WPSHost {
	return NewWPSHost(client)
}

func (h *WPSHost) Kind() Kind {
	if h.client == nil {
		return ""
	}
	return KindFromApp(h.client.App())
}

func (h *WPSHost) Family() Family { return FamilyOf(h.Kind()) }

func (h *WPSHost) Catalog() []CatalogEntry {
	if h.client == nil {
		return nil
	}
	_, commands, _, _ := h.client.Snapshot()
	return catalogFromWPS(commands)
}

func (h *WPSHost) Capabilities() Capabilities { return ParseCatalog(h.Catalog()) }

func (h *WPSHost) Document() Document {
	if h.client == nil {
		return Document{}
	}
	document, _, _, _ := h.client.Snapshot()
	return documentFromMap(document)
}

// Execute sends one command to the add-in through its queue. The queue does not
// take a context, so a cancelled context is checked before enqueueing only.
func (h *WPSHost) Execute(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	if h.client == nil {
		return nil, errors.New("host: no WPS bridge client")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reply, ok := h.client.Enqueue(action, params)
	if !ok {
		return nil, errors.New("host: WPS add-in is not connected")
	}
	return wpsResult(reply)
}

// WPSBridgeHost adapts a whole wpsbridge.Bridge and resolves the client for one
// app (or pane) on every call.
type WPSBridgeHost struct {
	bridge   *wpsbridge.Bridge
	clientID string
	app      string
}

var _ Host = (*WPSBridgeHost)(nil)

// NewWPSBridgeHost wraps a WPS bridge. With one optional argument it selects
// that app; with two it selects a clientID then an app. An empty app resolves
// the primary client.
func NewWPSBridgeHost(bridge *wpsbridge.Bridge, target ...string) *WPSBridgeHost {
	host := &WPSBridgeHost{bridge: bridge}
	switch len(target) {
	case 1:
		host.app = target[0]
	case 2:
		host.clientID, host.app = target[0], target[1]
	}
	return host
}

// NewWPSBridgeClientHost wraps a WPS bridge for a specific pane client.
func NewWPSBridgeClientHost(bridge *wpsbridge.Bridge, clientID, app string) *WPSBridgeHost {
	return NewWPSBridgeHost(bridge, clientID, app)
}

func (h *WPSBridgeHost) resolve() *wpsbridge.Client {
	if h.bridge == nil {
		return nil
	}
	return h.bridge.Resolve(h.clientID, h.app)
}

func (h *WPSBridgeHost) Kind() Kind {
	if kind := KindFromApp(h.app); kind != "" {
		return kind
	}
	if client := h.resolve(); client != nil {
		return KindFromApp(client.App())
	}
	return ""
}

func (h *WPSBridgeHost) Family() Family { return FamilyOf(h.Kind()) }

func (h *WPSBridgeHost) Catalog() []CatalogEntry {
	client := h.resolve()
	if client == nil {
		return nil
	}
	_, commands, _, _ := client.Snapshot()
	return catalogFromWPS(commands)
}

func (h *WPSBridgeHost) Capabilities() Capabilities { return ParseCatalog(h.Catalog()) }

func (h *WPSBridgeHost) Document() Document {
	client := h.resolve()
	if client == nil {
		return Document{}
	}
	document, _, _, _ := client.Snapshot()
	return documentFromMap(document)
}

func (h *WPSBridgeHost) Execute(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	client := h.resolve()
	if client == nil {
		return nil, errors.New("host: no WPS bridge client")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reply, ok := client.Enqueue(action, params)
	if !ok {
		return nil, errors.New("host: WPS add-in is not connected")
	}
	return wpsResult(reply)
}

func catalogFromOffice(commands []office.Command) []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(commands))
	for _, command := range commands {
		params := make([]Param, 0, len(command.Params))
		for _, param := range command.Params {
			params = append(params, Param{Name: param.Name, Required: param.Required, Hint: param.Hint})
		}
		entries = append(entries, CatalogEntry{
			Name: command.Name, Kind: command.Kind, Agent: command.Agent,
			Summary: command.Summary, Params: params,
		})
	}
	return entries
}

// catalogFromWPS converts the untyped command list in a WPS poll/snapshot.
func catalogFromWPS(commands []any) []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(commands))
	for _, item := range commands {
		command, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := command["name"].(string)
		kind, _ := command["kind"].(string)
		agent, _ := command["agent"].(bool)
		undo, _ := command["undo"].(bool)
		summary, _ := command["summary"].(string)
		params := []Param{}
		if list, ok := command["params"].([]any); ok {
			for _, raw := range list {
				param, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				paramName, _ := param["name"].(string)
				required, _ := param["required"].(bool)
				hint, _ := param["hint"].(string)
				params = append(params, Param{Name: paramName, Required: required, Hint: hint})
			}
		}
		entries = append(entries, CatalogEntry{
			Name: name, Kind: kind, Agent: agent, Undo: undo, Summary: summary, Params: params,
		})
	}
	return entries
}

func documentFromMap(document map[string]any) Document {
	name, _ := document["name"].(string)
	path, _ := document["fullName"].(string)
	slide := intValue(document["slide"])
	if slide == 0 {
		if view, ok := document["view"].(map[string]any); ok {
			slide = intValue(view["slide"])
		}
	}
	return Document{Key: office.DocumentKey(path), Name: documentName(path, name), Path: path, Slide: slide}
}

func intValue(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		return int(number)
	}
	return 0
}

func wpsResult(reply map[string]any) (map[string]any, error) {
	if reply == nil {
		return map[string]any{}, nil
	}
	if ok, present := reply["ok"].(bool); present && !ok {
		message, _ := reply["error"].(string)
		if message == "" {
			message = "host: WPS command failed"
		}
		return reply, errors.New(message)
	}
	if result, ok := reply["result"].(map[string]any); ok {
		return result, nil
	}
	return reply, nil
}

func cloneCatalog(entries []CatalogEntry) []CatalogEntry {
	if entries == nil {
		return nil
	}
	cloned := make([]CatalogEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

// documentName returns the display name for a document path. It is used when a
// snapshot only carries a full path.
func documentName(path, name string) string {
	if name != "" {
		return name
	}
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
