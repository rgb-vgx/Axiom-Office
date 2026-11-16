// Package flag doc co release-safe cho hanh vi moi cua Core.
//
// Thu tu uu tien: bien moi truong AXIOM_FLAGS > config.Experimental (JSON/HKCU, khoa
// experimental.<name>) > tat. AXIOM_FLAGS la danh sach phan cach bang dau phay: "foo" bat,
// "-foo" tat, "all" bat moi ten. Ten khong duoc nhac toi -> tat (khong co danh sach ten hop le
// co dinh: mot co moi chi can duoc doc o noi dung no).
package flag

import (
	"os"
	"sort"
	"strings"

	"axiomoffice/core/internal/config"
)

// EnvVar: danh sach co ghi de cau hinh, dung cho script/CI va de tat nhanh mot co tren may
// nguoi dung ma khong phai sua config.json/HKCU.
const EnvVar = "AXIOM_FLAGS"

// Enabled bao co `name` co bat khong. Doc lai cau hinh moi lan goi (giong cac endpoint khac cua
// Core) de doi config.json/HKCU co hieu luc ngay, khong phai khoi dong lai.
func Enabled(name string) bool {
	return EnabledIn(config.Load(), name)
}

// EnabledIn: nhu Enabled nhung dung anh chup cau hinh da co (endpoint /health doc cau hinh song
// mot lan roi hoi nhieu ten co).
func EnabledIn(cfg config.Config, name string) bool {
	return enabledIn(cfg, name, parse(os.Getenv(EnvVar)))
}

// Names tra ve ten cac co dang bat, sap xep on dinh - de /health liet ke flags.<name> ma khong
// phai biet truoc danh sach co. Gom ten trong config.Experimental va ten duoc nhac trong
// AXIOM_FLAGS (tru "all" vi no khong neu ten nao).
func Names(cfg config.Config) []string {
	flags := parse(os.Getenv(EnvVar))
	names := make(map[string]bool, len(cfg.Experimental)+len(flags.named))
	for name := range cfg.Experimental {
		if name = strings.TrimSpace(name); name != "" {
			names[name] = true
		}
	}
	for name := range flags.named {
		names[name] = true
	}
	enabled := make([]string, 0, len(names))
	for name := range names {
		if enabledIn(cfg, name, flags) {
			enabled = append(enabled, name)
		}
	}
	sort.Strings(enabled)
	return enabled
}

func enabledIn(cfg config.Config, name string, flags flagSet) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if enabled, ok := flags.named[name]; ok {
		return enabled
	}
	if flags.hasAll {
		return flags.all
	}
	return cfg.Experimental[name]
}

// flagSet: ket qua doc AXIOM_FLAGS. named thang all (cu the thang chung), va cac ten trong named
// giu nguyen ca luc tat de "-foo" chan duoc cau hinh bat foo.
type flagSet struct {
	all    bool
	hasAll bool
	named  map[string]bool
}

func parse(raw string) flagSet {
	flags := flagSet{named: map[string]bool{}}
	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		enabled := true
		switch {
		case strings.HasPrefix(token, "-"):
			enabled = false
			token = strings.TrimSpace(token[1:])
		case strings.HasPrefix(token, "+"):
			token = strings.TrimSpace(token[1:])
		}
		if token == "" {
			continue
		}
		if strings.EqualFold(token, "all") {
			flags.all, flags.hasAll = enabled, true
			continue
		}
		flags.named[token] = enabled
	}
	return flags
}
