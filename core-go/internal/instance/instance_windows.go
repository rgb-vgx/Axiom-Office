//go:build windows

// Package instance giu MOT Core cho moi nguoi dung (New_arch.md muc 4.3).
package instance

import (
	"errors"

	"golang.org/x/sys/windows"
)

// Lock: Windows dung named mutex CUNG TEN voi ban .NET (Local\AxiomOffice.Core) - hai ban loai tru nhau,
// khong bao gio chay song song mot Core .NET va mot Core Go.
type Lock struct{ handle windows.Handle }

// Acquire tra ve (nil, false) neu da co Core khac giu khoa.
func Acquire(name string) (*Lock, bool, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, err
	}
	handle, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, false, err
	}
	event, err := windows.WaitForSingleObject(handle, 0)
	switch {
	case err != nil:
		_ = windows.CloseHandle(handle)
		return nil, false, err
	case event == windows.WAIT_OBJECT_0 || event == windows.WAIT_ABANDONED:
		// WAIT_ABANDONED: instance truoc bi kill - ta tiep quan (giong AbandonedMutexException cua .NET).
		return &Lock{handle: handle}, true, nil
	default:
		_ = windows.CloseHandle(handle)
		return nil, false, nil
	}
}

func (l *Lock) Release() {
	if l == nil {
		return
	}
	_ = windows.ReleaseMutex(l.handle)
	_ = windows.CloseHandle(l.handle)
}
