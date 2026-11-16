package protocol

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// emitterCall matches literal event names in Publish("..."), run.Event("...")
// and writeRaw(w, "..."). Dynamic forms such as Publish(eventType, data) are
// deliberately ignored.
var emitterCall = regexp.MustCompile(`\.(?:Publish|Event)\(\s*"([^"]+)"|writeRaw\(\s*\w+\s*,\s*"([^"]+)"`)

// outsideScan lists event types emitted through a call shape the regex above
// does not cover. Their presence is checked as a plain string in the source.
var outsideScan = map[string]bool{
	// api/wpspane.go writes the SSE frame directly instead of calling writeRaw.
	"pane.quick": true,
	// api/runs.go calls writeRaw(w, "ping", ...); kept explicit as the task allows.
	"ping": true,
}

func TestManifestLoad(t *testing.T) {
	events, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("events.json is empty")
	}
	seen := map[string]bool{}
	for _, event := range events {
		if event.Type == "" {
			t.Error("event with empty type")
		}
		if seen[event.Type] {
			t.Errorf("duplicate event type %q", event.Type)
		}
		seen[event.Type] = true
		if strings.TrimSpace(event.Summary) == "" {
			t.Errorf("event %q has no summary", event.Type)
		}
		if event.Payload == nil {
			t.Errorf("event %q has a nil payload (want an empty object for no fields)", event.Type)
		}
	}
	if again := Events(); len(again) != len(events) {
		t.Fatalf("Events() returned %d entries, Load() returned %d", len(again), len(events))
	}
}

func TestManifestMatchesSources(t *testing.T) {
	events, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	manifest := make(map[string]Event, len(events))
	for _, event := range events {
		manifest[event.Type] = event
	}

	root := internalDir(t)
	emitted := map[string][]string{}
	var source strings.Builder
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source.Write(data)
		source.WriteByte('\n')
		for _, match := range emitterCall.FindAllStringSubmatch(string(data), -1) {
			name := match[1]
			if name == "" {
				name = match[2]
			}
			relative, _ := filepath.Rel(root, path)
			emitted[name] = append(emitted[name], relative)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("scan %s: %v", root, walkErr)
	}

	// Every literal event name used by a publisher must be declared in the manifest.
	for name, files := range emitted {
		if _, ok := manifest[name]; !ok {
			t.Errorf("event %q emitted in %s is missing from events.json", name, strings.Join(files, ", "))
		}
	}

	// Every manifest entry must still be emitted somewhere in the source.
	text := source.String()
	for _, event := range events {
		if _, ok := emitted[event.Type]; ok {
			continue
		}
		if outsideScan[event.Type] {
			if !strings.Contains(text, event.Type) {
				t.Errorf("event %q is allowed outside the scan but does not appear in source", event.Type)
			}
			continue
		}
		t.Errorf("event %q is in events.json but no Publish/Event/writeRaw call emits it", event.Type)
	}
}

// internalDir is core-go/internal, the tree scanned for event publishers.
func internalDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate manifest_test.go")
	}
	return filepath.Dir(filepath.Dir(file))
}
