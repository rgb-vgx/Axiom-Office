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

func TestJSONSourceExperimental(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"Experimental":{" nested-on ":true,"nested-off":false},"experimental.dotted-on":"1","experimental.dotted-off":"0"}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := From(env(nil), JSONSource{Path: path})
	want := map[string]bool{"nested-on": true, "nested-off": false, "dotted-on": true, "dotted-off": false}
	for name, enabled := range want {
		if cfg.Experimental[name] != enabled {
			t.Fatalf("Experimental[%q] = %v, want %v (all: %+v)", name, cfg.Experimental[name], enabled, cfg.Experimental)
		}
	}
	if len(cfg.Experimental) != len(want) {
		t.Fatalf("Experimental = %+v", cfg.Experimental)
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

// policy.rules: mang JSON (bien moi truong), object noi bang ';' (JSONSource noi phan tu mang), va
// object hong bi bo qua tung cai.
func TestPolicyRulesFromEnvironmentAndSource(t *testing.T) {
	array := `[{"action":"writer.save","resource":"**","effect":"allow"},` +
		`{"action":"wpp.**","effect":"DENY"}]`
	cfg := From(env(map[string]string{"AXIOM_POLICY_RULES": array}), mapSource{})
	if len(cfg.Policy) != 2 || cfg.Policy[0].Action != "writer.save" || cfg.Policy[0].Effect != "allow" ||
		cfg.Policy[1].Action != "wpp.**" || cfg.Policy[1].Effect != "deny" {
		t.Fatalf("policy tu env = %+v", cfg.Policy)
	}

	joined := `{"action":"et.writeRange","resource":"Ban1.xlsx","effect":"ask"};` +
		`{"action":"et.deleteSheet","effect":"deny"};hỏng`
	cfg = From(env(nil), mapSource{"policy.rules": joined})
	if len(cfg.Policy) != 2 || cfg.Policy[0].Action != "et.writeRange" || cfg.Policy[0].Resource != "Ban1.xlsx" ||
		cfg.Policy[1].Effect != "deny" {
		t.Fatalf("policy tu nguon = %+v", cfg.Policy)
	}

	// Thieu action hoac effect -> bo qua; khong co khoa -> rong.
	if cfg = From(env(nil), mapSource{"policy.rules": `{"action":"","effect":"deny"};{"action":"et.save"}`}); len(cfg.Policy) != 0 {
		t.Fatalf("quy tac thieu truong phai bi bo: %+v", cfg.Policy)
	}
	if cfg = From(env(nil), mapSource{}); len(cfg.Policy) != 0 {
		t.Fatalf("khong co policy.rules -> rong: %+v", cfg.Policy)
	}

	// config.json: khoa phang "policy.rules" hoac dang long {"policy":{"rules":[...]}}.
	nested := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(nested, []byte(`{"policy":{"rules":[{"action":"et.save","effect":"deny"}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg = From(env(nil), JSONSource{Path: nested})
	if len(cfg.Policy) != 1 || cfg.Policy[0].Action != "et.save" || cfg.Policy[0].Effect != "deny" {
		t.Fatalf("policy dang long trong config.json = %+v", cfg.Policy)
	}
}
