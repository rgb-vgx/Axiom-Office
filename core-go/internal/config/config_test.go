package config

import (
	"os"
	"path/filepath"
	"testing"
)

type mapSource map[string]string

func (m mapSource) String(name string) (string, bool) {
	value, ok := m[name]
	return value, ok
}

func (m mapSource) Int(name string) (int, bool) { return 0, false }

func env(values map[string]string) Env {
	return func(key string) string { return values[key] }
}

func TestEnvironmentWinsOverSource(t *testing.T) {
	cfg := From(env(map[string]string{"AXIOM_LLM_MODEL": "env-model", "AXIOM_LLM_REQUEST_TIMEOUT": "9999"}),
		mapSource{"LlmModel": "reg-model", "LlmEndpoint": "http://x/v1", "MemoryEnabled": "0", "SkillDirs": "a; b;;c"})
	if cfg.LlmModel != "env-model" || cfg.LlmEndpoint != "http://x/v1" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.MemoryEnabled || !cfg.MemoryAutoExtract || cfg.LlmProvider != "openai" {
		t.Fatalf("flags/defaults wrong: %+v", cfg)
	}
	if cfg.LlmRequestTimeoutSeconds != 600 || cfg.ConfirmTimeoutSeconds != 120 {
		t.Fatalf("clamp wrong: %+v", cfg)
	}
	if len(cfg.SkillDirs) != 3 {
		t.Fatalf("SkillDirs = %q", cfg.SkillDirs)
	}
}

func TestJSONSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"LlmModel":"m","MemoryEnabled":false,"CorePort":47850,"SkillDirs":["x","y"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := From(env(nil), JSONSource{Path: path})
	if cfg.LlmModel != "m" || cfg.MemoryEnabled || cfg.CorePort != 47850 || len(cfg.SkillDirs) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if missing := From(env(nil), JSONSource{Path: filepath.Join(t.TempDir(), "none.json")}); missing.CorePort != DefaultPort {
		t.Fatalf("missing file should give defaults: %+v", missing)
	}
}

func TestPathsOverride(t *testing.T) {
	root := t.TempDir()
	paths := NewPaths(root)
	if paths.DatabaseFile != filepath.Join(root, "core.db") || paths.CoreJSON != filepath.Join(root, "core.json") {
		t.Fatalf("paths = %+v", paths)
	}
	if NewPaths("").DatabaseFile != filepath.Join(DefaultRoot(), "core", "core.db") {
		t.Fatal("default database path must match CorePaths.cs")
	}
}

func TestPlainSecretPassesThrough(t *testing.T) {
	if Unprotect("sk-plain") != "sk-plain" || Unprotect("") != "" {
		t.Fatal("plain values must pass through")
	}
}
