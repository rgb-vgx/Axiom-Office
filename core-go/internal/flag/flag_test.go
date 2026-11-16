package flag

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"axiomoffice/core/internal/config"
)

func TestEnabledParsesAndTrims(t *testing.T) {
	t.Setenv(EnvVar, "  foo , -bar ,\t baz  , - qux ")
	for _, name := range []string{"foo", "baz"} {
		if !EnabledIn(config.Config{}, name) {
			t.Errorf("%q phai bat", name)
		}
	}
	for _, name := range []string{"bar", "qux", "khong-khai", ""} {
		if EnabledIn(config.Config{}, name) {
			t.Errorf("%q phai tat", name)
		}
	}
	if EnabledIn(config.Config{}, " foo ") != true {
		t.Fatal("ten co phai duoc trim")
	}
}

func TestEnabledAllAndDisable(t *testing.T) {
	t.Setenv(EnvVar, "all, -bar")
	if !EnabledIn(config.Config{}, "bat-ky") {
		t.Fatal("all phai bat moi ten")
	}
	if EnabledIn(config.Config{}, "bar") {
		t.Fatal("-bar phai tat bar du co all")
	}

	// -all tat mac dinh, ten duoc neu sau do van bat (cu the thang chung).
	t.Setenv(EnvVar, "-all, foo")
	if !EnabledIn(config.Config{}, "foo") {
		t.Fatal("foo phai bat")
	}
	if EnabledIn(config.Config{}, "khac") {
		t.Fatal("-all phai tat ten khong duoc neu")
	}
}

func TestEnabledConfigFallbackAndEnvWins(t *testing.T) {
	t.Setenv(EnvVar, "")
	cfg := config.Config{Experimental: map[string]bool{"config-on": true, "config-off": false}}
	if !EnabledIn(cfg, "config-on") || EnabledIn(cfg, "config-off") || EnabledIn(cfg, "khong-khai") {
		t.Fatalf("config fallback sai: %+v", cfg.Experimental)
	}

	t.Setenv(EnvVar, "-config-on, config-off")
	if EnabledIn(cfg, "config-on") {
		t.Fatal("AXIOM_FLAGS phai de len config")
	}
	if !EnabledIn(cfg, "config-off") {
		t.Fatal("AXIOM_FLAGS bat duoc co config dang tat")
	}

	// AXIOM_FLAGS khong nhac toi ten -> van roi ve config (khong phai tat tat ca).
	t.Setenv(EnvVar, "khac")
	if !EnabledIn(cfg, "config-on") {
		t.Fatal("ten khong duoc nhac trong AXIOM_FLAGS phai doc config")
	}
}

func TestEnabledReadsConfigFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv(EnvVar, "")
	path := filepath.Join(root, "axiom-office", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"Experimental":{"file-on":true,"file-off":false}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !Enabled("file-on") {
		t.Fatal("co trong config.json phai bat")
	}
	if Enabled("file-off") || Enabled("khong-khai") {
		t.Fatal("co tat/khong khai phai tat")
	}
}

func TestNamesFiltersAndSorts(t *testing.T) {
	t.Setenv(EnvVar, "-z, env-on, -off-env")
	cfg := config.Config{Experimental: map[string]bool{"z": true, "a": true, "off": false}}
	got := Names(cfg)
	want := []string{"a", "env-on"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names = %v, want %v", got, want)
	}
}
