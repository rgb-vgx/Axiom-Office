package api

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"axiomoffice/core/internal/config"
	featureflag "axiomoffice/core/internal/flag"
	"axiomoffice/core/internal/store"
)

func newCapabilityHandler(t *testing.T) http.Handler {
	t.Helper()
	root := t.TempDir()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	return Handler(&Deps{
		Config:  config.Config{Token: "tok", MemoryEnabled: true, LlmProvider: "openai", LlmModel: "m"},
		Paths:   config.NewPaths(root),
		Runtime: &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:    http.DefaultClient,
		Stores:  stores,
	}, http.NewServeMux())
}

func TestHealthCapabilities(t *testing.T) {
	t.Setenv(featureflag.EnvVar, "")
	handler := newCapabilityHandler(t)
	result := decode(t, call(handler, "GET", "/health", "", nil).Body.String())["result"].(map[string]any)

	if result["protocol"] != float64(Protocol) || result["app"] != "core" || result["memory"] != "on" {
		t.Fatalf("truong cu cua /health doi: %+v", result)
	}
	caps, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("thieu capabilities: %+v", result)
	}
	for _, name := range []string{"runs.idempotency", "approvals.saved"} {
		if caps[name] != true {
			t.Errorf("capabilities[%q] = %v", name, caps[name])
		}
	}
	if _, found := caps["flags.khong-bat"]; found {
		t.Fatalf("khong duoc liet ke co dang tat: %+v", caps)
	}
}

func TestHealthCapabilitiesReflectFlags(t *testing.T) {
	t.Setenv(featureflag.EnvVar, "-config-on, env-on")
	root := t.TempDir()
	stores := store.OpenStores(filepath.Join(root, "core.db"))
	t.Cleanup(func() { _ = stores.DB.Close() })
	cfg := config.Config{
		Token: "tok", MemoryEnabled: true,
		Experimental: map[string]bool{"config-on": true, "config-off": false},
	}
	handler := Handler(&Deps{
		Config:     cfg,
		LoadConfig: func() config.Config { return cfg },
		Paths:      config.NewPaths(root),
		Runtime:    &Runtime{Port: 1234, Started: time.Now(), Version: "1.0.0"},
		HTTP:       http.DefaultClient,
		Stores:     stores,
	}, http.NewServeMux())

	caps := decode(t, call(handler, "GET", "/health", "", nil).Body.String())["result"].(map[string]any)["capabilities"].(map[string]any)
	if caps["flags.env-on"] != true {
		t.Fatalf("co bat tu AXIOM_FLAGS phai xuat hien: %+v", caps)
	}
	if _, found := caps["flags.config-on"]; found {
		t.Fatalf("AXIOM_FLAGS tat phai de len config: %+v", caps)
	}
	if _, found := caps["flags.config-off"]; found {
		t.Fatalf("co tat khong duoc liet ke: %+v", caps)
	}
}
