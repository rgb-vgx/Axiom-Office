package wpsbridge

import (
	"os"
	"path/filepath"
	"strings"
)

// Add-in chay don luong: trong luc chay mot lenh no khong poll duoc, nen Core khong phan biet duoc "lenh dai" voi
// "WPS da dong giua chung" va truoc day phai cho het CmdWaitSeconds (300 giay). Core nhin thang bang tien trinh:
// khong con tien trinh WPS nao thi lenh dang chay khong bao gio co ket qua.
//
// Co y THAN TRONG: chi ket luan "da dong" khi khong con BAT KY tien trinh WPS nao (khong xet theo tung app). Ban WPS
// gop nhieu app vao mot tien trinh `wpsoffice`, va ten tien trinh tung app chua do tren moi ban; ket luan sai "da dong"
// se cat ngang mot lenh dang ghi tai lieu, te hon nhieu so voi cho het gio.
var wpsProcessNames = map[string]bool{"wps": true, "et": true, "wpp": true, "wpsoffice": true}

// procRoot: goc /proc (test doi sang thu muc gia).
var procRoot = "/proc"

// wpsAppAlive: con tien trinh WPS nao chay khong. known=false khi khong doc duoc /proc (Windows, sandbox) - luc do
// Core giu hanh vi cu (chi het gio).
func wpsAppAlive(string) (alive, known bool) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return false, false
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || name[0] < '0' || name[0] > '9' {
			continue
		}
		comm, err := os.ReadFile(filepath.Join(procRoot, name, "comm"))
		if err != nil {
			continue
		}
		if wpsProcessNames[strings.TrimSpace(string(comm))] {
			return true, true
		}
	}
	return false, true
}

// appGone: tien trinh cua app chac chan da mat (khong biet thi false).
func (c *Client) appGone(app string) bool {
	check := c.b.appAlive
	if check == nil {
		return false
	}
	alive, known := check(app)
	return known && !alive
}
