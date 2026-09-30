// Package corefile doc/ghi core.json (ban Go cua CoreFile.cs): Core ghi khi san sang de add-in, extension va
// MCP tim thay, xoa khi thoat. Ghi atomic (.tmp + rename). Ten truong camelCase giong ban .NET.
package corefile

import (
	"encoding/json"
	"os"
	"path/filepath"

	"axiomoffice/core/internal/corelog"
)

type Info struct {
	Pid      int    `json:"pid"`
	Port     int    `json:"port"`
	Version  string `json:"version"`
	Started  string `json:"started"`
	Protocol int    `json:"protocol"`
	Exe      string `json:"exe"`
}

func Write(path string, info Info) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		corelog.Error("Cannot write %s: %v", path, err)
		return
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		corelog.Error("Cannot write %s: %v", path, err)
		return
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		corelog.Error("Cannot write %s: %v", path, err)
		return
	}
	if err := os.Rename(temp, path); err != nil {
		// Khong ghi duoc core.json thi add-in se khoi dong Core moi lan - khong lam Core dung.
		corelog.Error("Cannot write %s: %v", path, err)
	}
}

func Read(path string) (Info, bool) {
	var info Info
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &info) != nil {
		return Info{}, false
	}
	return info, true
}

func Delete(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		corelog.Error("Cannot delete %s: %v", path, err)
	}
}
