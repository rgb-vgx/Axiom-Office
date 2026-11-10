package config

import (
	"fmt"
	"strconv"
)

// Ghi cau hinh: ban Go cua config.py:set_value cua extension LibreOffice (Windows: HKCU, Linux:
// ~/.config/axiom-office/config.json - cung file Core doc). Truoc 19/10/2026 Core chi DOC cau hinh,
// nen moi nut doi cau hinh tren giao dien (toggle "Nhin tai lieu" cua ribbon WPS) khong co duong that
// dung sau (docs/architecture/ribbon-ui.md 5.1).

// WritableSource: nguon cau hinh ghi duoc (JSONSource tren Linux, RegistrySource tren Windows).
// Khai bao rieng thay vi them vao Source de khong phai sua moi ban gia cua Source trong test.
type WritableSource interface {
	Set(name string, value any) error
}

// SetValue ghi mot khoa cau hinh vao nguon nen tang. Nguon chi doc -> loi (khong im lang bo qua).
func SetValue(source Source, name string, value any) error {
	if _, ok := SettingText(value); !ok {
		return fmt.Errorf("unsupported value type for setting %s", name)
	}
	writable, ok := source.(WritableSource)
	if !ok {
		return fmt.Errorf("configuration source is read-only: %s", DescribeSource())
	}
	return writable.Set(name, value)
}

// SettingText doi gia tri JSON thanh chuoi cau hinh - dung cho RegistrySource (ghi REG_SZ nhu add-in
// .NET va extension LibreOffice) va de kiem tra kieu truoc khi ghi.
// "1"/"0" cho bat/tat: flag() doc lai dung ca hai dinh dang.
func SettingText(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case bool:
		if typed {
			return "1", true
		}
		return "0", true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case int:
		return strconv.Itoa(typed), true
	default:
		return "", false
	}
}
