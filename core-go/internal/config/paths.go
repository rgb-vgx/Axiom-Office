package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Paths la vi tri file cua Core (ban Go cua CorePaths.cs) - PHAI giong ban .NET de add-in/extension tim
// thay core.json va hai ban dung chung core.db:
//
//	Windows : %LOCALAPPDATA%\AxiomOffice\{core.json, core.log} + ...\core\core.db
//	Linux   : $XDG_DATA_HOME/axiom-office (mac dinh ~/.local/share/axiom-office)
//	AXIOM_CORE_DATA_DIR: moi thu nam thang trong thu muc do (test).
type Paths struct {
	Root          string
	IsOverride    bool
	CoreJSON      string
	LogFile       string
	DatabaseFile  string
	SkillsDir     string
	McpConfigFile string
}

func NewPaths(override string) Paths {
	root, isOverride := DefaultRoot(), false
	if strings.TrimSpace(override) != "" {
		if absolute, err := filepath.Abs(strings.TrimSpace(override)); err == nil {
			root = absolute
		} else {
			root = strings.TrimSpace(override)
		}
		isOverride = true
	}
	database := filepath.Join(root, "core", "core.db")
	if isOverride {
		database = filepath.Join(root, "core.db")
	}
	return Paths{
		Root:          root,
		IsOverride:    isOverride,
		CoreJSON:      filepath.Join(root, "core.json"),
		LogFile:       filepath.Join(root, "core.log"),
		DatabaseFile:  database,
		SkillsDir:     filepath.Join(root, "skills"),
		McpConfigFile: filepath.Join(root, "mcp.json"),
	}
}

// DefaultRoot: thu muc du lieu mac dinh theo nen tang.
func DefaultRoot() string {
	if runtime.GOOS == "windows" {
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			home, _ := os.UserHomeDir()
			local = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(local, "AxiomOffice")
	}
	root := os.Getenv("XDG_DATA_HOME")
	if strings.TrimSpace(root) == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(root, "axiom-office")
}

// DefaultSessionDir: noi bridge (add-in / extension) ghi session - giong SessionDirectory.DefaultDirectory.
func DefaultSessionDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(DefaultRoot(), "sessions")
	}
	base := os.Getenv("XDG_RUNTIME_DIR")
	if strings.TrimSpace(base) == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "axiom-office", "sessions")
}

func (p Paths) EnsureDirectories() error {
	if err := os.MkdirAll(p.Root, 0o755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(p.DatabaseFile), 0o755)
}
