//go:build windows

package config

import (
	"encoding/base64"
	"unsafe"

	"golang.org/x/sys/windows"
)

// dpapiUnprotect giai "dpapi:<base64>" do add-in ghi (ProtectedData.Protect, CurrentUser, khong entropy).
func dpapiUnprotect(encoded string) string {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 {
		return ""
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return ""
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return string(unsafe.Slice(out.Data, out.Size))
}

// Protect ma hoa DPAPI giong add-in (dung cho test doi chieu hai chieu).
func Protect(value string) string {
	if value == "" {
		return ""
	}
	data := []byte(value)
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return value
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return dpapiPrefix + base64.StdEncoding.EncodeToString(unsafe.Slice(out.Data, out.Size))
}
