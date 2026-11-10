// Package office: noi Core noi chuyen voi add-in/extension dang chay (session registry + bridge HTTP) -
// ban Go cua Office/SessionDirectory.cs va Office/BridgeClient.cs.
package office

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/config"
)

// StaleSeconds: session khong duoc lam moi trong bao lau thi coi la cu (muc 7.1).
const StaleSeconds = 90

// Session: mot bridge dang song (file trong .../axiom-office/sessions/{pid}.json).
type Session struct {
	Pid          int
	App          string
	Family       string
	Port         int
	Host         string
	Version      string
	AgeSeconds   float64
	Document     string
	DocumentPath string
	File         string
}

func (s Session) Describe() string {
	document := s.Document
	if document == "" {
		document = "(khong co tai lieu)"
	}
	return s.App + "/" + s.Family + " port " + strconv.Itoa(s.Port) + " pid " + strconv.Itoa(s.Pid) + " - " + document
}

// parseSession doc mot file session; file dang duoc ghi lai -> (nil, false).
func parseSession(data []byte, file string) (*Session, bool) {
	var raw struct {
		Pid           json.Number `json:"pid"`
		Port          json.Number `json:"port"`
		App           string      `json:"app"`
		Family        string      `json:"family"`
		Host          string      `json:"host"`
		Version       string      `json:"version"`
		LastSeenEpoch float64     `json:"lastSeenEpoch"`
		Document      string      `json:"document"`
		DocumentPath  string      `json:"documentPath"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return nil, false
	}
	pid, _ := strconv.Atoi(raw.Pid.String())
	port, _ := strconv.Atoi(raw.Port.String())
	if pid <= 0 || port <= 0 {
		return nil, false
	}
	// Thieu lastSeenEpoch thi lay theo thoi diem sua file, giong ban C#.
	modified := time.Now()
	if info, err := os.Stat(file); err == nil {
		modified = info.ModTime()
	}
	age := ageFromEpoch(raw.LastSeenEpoch, modified)
	return &Session{
		Pid: pid, App: raw.App, Family: raw.Family, Port: port, Host: raw.Host, Version: raw.Version,
		AgeSeconds: age, Document: raw.Document, DocumentPath: raw.DocumentPath, File: file,
	}, true
}

// Directory doc thu muc session registry. Khong xoa file cua nguoi khac (Host.exe lam viec prune);
// chi loc theo tuoi va theo tien trinh con song.
type Directory struct{ Path string }

func NewDirectory(path string) *Directory {
	if strings.TrimSpace(path) == "" {
		path = config.DefaultSessionDir()
	}
	return &Directory{Path: path}
}

func (d *Directory) List(includeStale bool) []Session {
	found := []Session{}
	entries, err := os.ReadDir(d.Path)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		file := filepath.Join(d.Path, entry.Name())
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		session, ok := parseSession(data, file)
		if !ok || !IsAlive(session.Pid) {
			continue
		}
		if !includeStale && session.AgeSeconds > StaleSeconds {
			continue
		}
		found = append(found, *session)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Port < found[j].Port })
	return found
}

func (d *Directory) Find(port int) *Session {
	for _, session := range d.List(false) {
		if session.Port == port {
			return &session
		}
	}
	return nil
}

// FindWoken: session qua han tren cong nay ma bridge van tra loi /health dung pid cua no - may vua thuc day sau khi
// ngu, heartbeat (25 s) chua kip lam moi. Luot 25/10/2026: may ngu 1,5 gio, thuc day la ca bo benchmark bao "office
// session not found" trong khi PowerPoint van chay (docs/plans/19-h0 H2). healthPid chi goi khi co ung vien.
func (d *Directory) FindWoken(port int, healthPid func() int) *Session {
	for _, session := range d.List(true) {
		if session.Port == port && session.AgeSeconds > StaleSeconds && healthPid() == session.Pid {
			return &session
		}
	}
	return nil
}

// DocumentKey chuan hoa duong dan tai lieu (muc 8.5.1): Windows chu thuong + '\'; tai lieu chua luu -> "".
func DocumentKey(fullName string) string {
	value := strings.TrimSpace(fullName)
	if value == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		// Windows khong phan biet hoa/thuong va chap nhan ca '/'.
		value = strings.ToLower(strings.ReplaceAll(value, "/", "\\"))
	}
	return value
}
