// Package protocol is the machine-readable source of truth for the Core run/SSE
// event types and their payload fields. Both the Go core and the generated
// JavaScript/Python consumers read from the embedded events.json manifest.
package protocol

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// eventsJSON is the embedded manifest. It is the single source of truth; the
// generated consumers are produced by scripts/generate_protocol.py.
//
//go:embed events.json
var eventsJSON []byte

// Event describes one event type published on the run SSE stream.
type Event struct {
	Type    string            `json:"type"`
	Summary string            `json:"summary"`
	Payload map[string]string `json:"payload"`
}

var (
	loadOnce sync.Once
	loaded   []Event
	loadErr  error
)

// Load parses the embedded event manifest. The returned slice is a copy, so
// callers may sort or modify it without disturbing later calls.
func Load() ([]Event, error) {
	loadOnce.Do(func() {
		if err := json.Unmarshal(eventsJSON, &loaded); err != nil {
			loadErr = fmt.Errorf("protocol: parse events.json: %w", err)
			return
		}
		seen := make(map[string]bool, len(loaded))
		for index, event := range loaded {
			if event.Type == "" {
				loadErr = fmt.Errorf("protocol: events.json entry %d has no type", index)
				return
			}
			if seen[event.Type] {
				loadErr = fmt.Errorf("protocol: duplicate event type %q", event.Type)
				return
			}
			seen[event.Type] = true
			if event.Payload == nil {
				loaded[index].Payload = map[string]string{}
			}
		}
	})
	if loadErr != nil {
		return nil, loadErr
	}
	return append([]Event(nil), loaded...), nil
}

// Events returns the embedded manifest and panics if it cannot be parsed.
// It is the convenience form for callers that treat the manifest as a constant.
func Events() []Event {
	events, err := Load()
	if err != nil {
		panic(err)
	}
	return events
}
