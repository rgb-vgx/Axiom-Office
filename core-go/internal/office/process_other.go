//go:build !windows

package office

import (
	"errors"

	"golang.org/x/sys/unix"
)

// IsAlive: tin hieu 0 kiem tra tien trinh con song (EPERM = cua nguoi khac -> coi nhu con song).
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := unix.Kill(pid, 0)
	return err == nil || errors.Is(err, unix.EPERM)
}
