//go:build !windows

// Package instance giu MOT Core cho moi nguoi dung (New_arch.md muc 4.3).
package instance

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Lock: Linux/macOS dung flock tren $XDG_RUNTIME_DIR/axiom-office/<ten>.lock (tu nha khi tien trinh chet,
// khong can don file). Ten lay tu MutexName ("Local\AxiomOffice.Core" -> "AxiomOffice.Core.lock").
type Lock struct{ file *os.File }

func Acquire(name string) (*Lock, bool, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if strings.TrimSpace(base) == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(base, "axiom-office")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, false, err
	}
	clean := strings.NewReplacer(`\`, "_", "/", "_", ":", "_").Replace(strings.TrimPrefix(name, `Local\`))
	file, err := os.OpenFile(filepath.Join(dir, clean+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if err == unix.EWOULDBLOCK {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &Lock{file: file}, true, nil
}

func (l *Lock) Release() {
	if l == nil {
		return
	}
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	_ = l.file.Close()
}
