//go:build windows

package office

import "golang.org/x/sys/windows"

const stillActive = 259

// IsAlive: tien trinh con song khong (Windows mo handle roi doc exit code).
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Khong mo duoc: hoac khong ton tai, hoac bi chan quyen (coi nhu con song, giong ban .NET).
		return err == windows.ERROR_ACCESS_DENIED
	}
	defer windows.CloseHandle(handle)
	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return true
	}
	return code == stillActive
}
