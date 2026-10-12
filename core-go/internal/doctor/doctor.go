// Package doctor la lenh chan doan mot-lan-chay: nguoi dung gap truc trac chay
// "axiom-core doctor" roi gui ket qua cho nguoi ho tro. Chi DOC, khong sua:
// khong giu mutex, khong mo cong, khong ghi/xoa core.json, khong xoa file
// phien, khong xoay/xoa log. Ngoai le duy nhat la file tham do .doctor-probe,
// ghi thu roi xoa ngay de kiem tra quyen ghi thu muc du lieu.
package doctor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"axiomoffice/core/internal/api"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/corefile"
	"axiomoffice/core/internal/office"
)

// Level la muc cua mot dong bao cao.
type Level string

const (
	// LevelOK la binh thuong.
	LevelOK Level = "ok"
	// LevelMissing la khong co / khong chay (cung la binh thuong).
	LevelMissing Level = "none"
	// LevelWarn la can chu y.
	LevelWarn Level = "warn"
)

// Line la mot dong bao cao. Khong tu in ra dau ca.
type Line struct {
	Level Level
	Text  string
}

// String in dong dang text thuong, khong mau ANSI, de dan vao chat/email.
func (l Line) String() string {
	switch l.Level {
	case LevelOK:
		return "[ok] " + l.Text
	case LevelMissing:
		return "[--] " + l.Text
	case LevelWarn:
		return "[!!] " + l.Text
	default:
		return l.Text
	}
}

func okLine(text string) Line   { return Line{Level: LevelOK, Text: text} }
func missLine(text string) Line { return Line{Level: LevelMissing, Text: text} }
func warnLine(text string) Line { return Line{Level: LevelWarn, Text: text} }
func rawLine(text string) Line  { return Line{Text: text} }

// Options gom moi thu Report can, truyen qua tham so de test duoc ma khong
// dung vao may that. Duong dan doc file/thu muc deu den tu tham so.
type Options struct {
	Cfg        config.Config
	Paths      config.Paths
	SessionDir string
	BridgeDir  string
	Source     config.Source
	Environ    []string
	Version    string
	Protocol   int
	Now        time.Time
}

// secretKeys la hai bien bi mat, tuyet doi khong in gia tri.
var secretKeys = map[string]bool{
	"AXIOM_LLM_API_KEY": true,
	"AXIOM_TOKEN":       true,
}

// Report dung bao cao chan doan tu tham so. Ham nay thuan doc: khong mo cong,
// khong goi HTTP, khong sua gi ngoai file tham do .doctor-probe tu xoa.
func Report(o Options) []Line {
	lines := []Line{}
	lines = append(lines, rawLine(fmt.Sprintf("axiom-core doctor version %s protocol %d", o.Version, o.Protocol)))

	lines = append(lines, rawLine("== Core =="))
	lines = append(lines, okLine(fmt.Sprintf("binary version %s protocol %d", o.Version, o.Protocol)))
	lines = append(lines, okLine(fmt.Sprintf("nguon cau hinh nen tang: %s", config.DescribeSource())))
	lines = append(lines, reportDataDir(o)...)
	lines = append(lines, reportSessionDir(o)...)
	lines = append(lines, reportCorePort(o)...)

	lines = append(lines, rawLine("== core.json =="))
	lines = append(lines, reportCoreJSON(o)...)

	lines = append(lines, rawLine("== Bridge =="))
	lines = append(lines, reportBridge(o)...)

	lines = append(lines, rawLine("== Log =="))
	lines = append(lines, reportLog(o)...)

	lines = append(lines, rawLine("== Bien moi truong AXIOM_* =="))
	lines = append(lines, reportEnv(o)...)

	warns := 0
	for _, l := range lines {
		if l.Level == LevelWarn {
			warns++
		}
	}
	if warns == 0 {
		lines = append(lines, okLine("khong thay van de"))
	} else {
		lines = append(lines, warnLine(fmt.Sprintf("%d muc can chu y", warns)))
	}
	return lines
}

func reportDataDir(o Options) []Line {
	root := o.Paths.Root
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return []Line{missLine(fmt.Sprintf("thu muc du lieu khong ton tai: %s", root))}
	}
	probe := filepath.Join(root, ".doctor-probe")
	if err := os.WriteFile(probe, []byte("probe"), 0o600); err != nil {
		return []Line{warnLine(fmt.Sprintf("thu muc du lieu %s khong ghi duoc: %v", root, err))}
	}
	// Tham do xong phai xoa ngay, ca khi doi ten/xoa loi cung bao de thay.
	if err := os.Remove(probe); err != nil {
		return []Line{warnLine(fmt.Sprintf("thu muc du lieu %s ghi duoc nhung khong xoa duoc file tham do .doctor-probe: %v", root, err))}
	}
	return []Line{okLine(fmt.Sprintf("thu muc du lieu %s ghi duoc (da thu .doctor-probe va xoa ngay)", root))}
}

func reportSessionDir(o Options) []Line {
	if info, err := os.Stat(o.SessionDir); err != nil || !info.IsDir() {
		return []Line{missLine(fmt.Sprintf("thu muc phien khong ton tai: %s", o.SessionDir))}
	}
	return []Line{okLine(fmt.Sprintf("thu muc phien: %s (ton tai)", o.SessionDir))}
}

func reportCorePort(o Options) []Line {
	port := o.Cfg.CorePort
	lines := []Line{}
	if port == 0 {
		lines = append(lines, okLine("cong Core: 0 (de he dieu hanh cap cong dong)"))
	} else {
		lines = append(lines, okLine(fmt.Sprintf("cong Core: %d (Core thu %d..%d)", port, port, port+9)))
		lines = append(lines, okLine(fmt.Sprintf("URL thu: http://127.0.0.1:%d (tu curl tren may nay)", port)))
	}
	lines = append(lines, rawLine("doctor KHONG tu goi HTTP: chi doc file/cau hinh, chay duoc khong can mang"))
	if o.Cfg.SingleInstance {
		lines = append(lines, okLine(fmt.Sprintf("mot-instance: bat (mutex %s)", o.Cfg.MutexName)))
	} else {
		lines = append(lines, missLine(fmt.Sprintf("mot-instance: tat (mutex %s) - nhieu Core co the cung chay", o.Cfg.MutexName)))
	}
	return lines
}

func reportCoreJSON(o Options) []Line {
	info, ok := corefile.Read(o.Paths.CoreJSON)
	if !ok {
		if _, err := os.Stat(o.Paths.CoreJSON); err != nil {
			return []Line{missLine(fmt.Sprintf("khong co core.json (%s) - binh thuong khi chua mo app", o.Paths.CoreJSON))}
		}
		return []Line{warnLine(fmt.Sprintf("co file core.json nhung doc khong duoc (%s) - file hong, nho nguoi ho tro huong dan xu ly", o.Paths.CoreJSON))}
	}
	lines := []Line{
		rawLine(fmt.Sprintf("core.json: pid=%d port=%d version=%s started=%s protocol=%d exe=%s",
			info.Pid, info.Port, info.Version, info.Started, info.Protocol, info.Exe)),
	}
	if office.IsAlive(info.Pid) {
		lines = append(lines, okLine(fmt.Sprintf("pid %d con song - co Core khac dang chay", info.Pid)))
	} else {
		lines = append(lines, warnLine(fmt.Sprintf("pid %d khong con song - core.json la file cu (Core thoat khong sach), hay xoa file nay: %s (doctor khong tu xoa)", info.Pid, o.Paths.CoreJSON)))
	}
	if info.Protocol != o.Protocol {
		lines = append(lines, warnLine(fmt.Sprintf("protocol trong file (%d) khac protocol cua binary nay (%d) - Core dang chay la ban khac", info.Protocol, o.Protocol)))
	} else {
		lines = append(lines, okLine(fmt.Sprintf("protocol khop: %d", o.Protocol)))
	}
	if info.Version != o.Version {
		lines = append(lines, missLine(fmt.Sprintf("version trong file (%s) khac version binary nay (%s) - Core cu hon khong phai loi", info.Version, o.Version)))
	} else {
		lines = append(lines, okLine(fmt.Sprintf("version khop: %s", o.Version)))
	}
	return lines
}

func reportBridge(o Options) []Line {
	lines := []Line{}
	kinds := []string{"wps", "et", "wpp"}
	base := config.BridgePortForKind(o.Source, "wps", false)
	lines = append(lines, rawLine(fmt.Sprintf("co so cong: nen tang %s, base %d (wps=base, et=base+1, wpp=base+2)", runtime.GOOS, base)))
	ports := map[string]int{}
	for _, kind := range kinds {
		port := config.BridgePortForKind(o.Source, kind, false)
		ports[kind] = port
		lines = append(lines, okLine(fmt.Sprintf("bridge %s: cong cau hinh %d", kind, port)))
	}

	dir := office.NewDirectory(o.SessionDir)
	live := dir.List(false)
	if len(live) == 0 {
		lines = append(lines, missLine("khong co phien nao dang song - binh thuong khi chua mo tai lieu"))
	} else {
		for _, s := range live {
			lines = append(lines, okLine(fmt.Sprintf("phien song: %s tuoi %.1fs version %s file %s", s.Describe(), s.AgeSeconds, s.Version, s.File)))
		}
	}

	// File phien ma List bo qua: doc do, sai dinh dang, hoac pid da chet.
	if entries, err := os.ReadDir(o.SessionDir); err == nil {
		seen := map[string]bool{}
		for _, s := range live {
			seen[s.File] = true
			seen[filepath.Base(s.File)] = true
		}
		names := []string{}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
				continue
			}
			full := filepath.Join(o.SessionDir, e.Name())
			if seen[full] || seen[e.Name()] {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			age := "khong ro"
			if info, err := os.Stat(filepath.Join(o.SessionDir, name)); err == nil && !o.Now.IsZero() {
				secs := o.Now.Sub(info.ModTime()).Seconds()
				if secs < 0 {
					secs = 0
				}
				age = fmt.Sprintf("%.0fs", secs)
			}
			lines = append(lines, missLine(fmt.Sprintf("file phien bi bo qua (List khong tra ve): %s tuoi %s - co the doc do, sai dinh dang, hoac pid da chet", name, age)))
		}
	}

	// Doi chieu cau hinh voi su that: cau hinh chi la phong doan, so phien moi la su that.
	for _, kind := range kinds {
		actual := dir.Ports(kind)
		want := ports[kind]
		if len(actual) == 0 || containsInt(actual, want) {
			continue
		}
		lines = append(lines, warnLine(fmt.Sprintf("kind %s: cau hinh tro cong %d nhung bridge dang chay o cong %s - cau hinh chi la phong doan, so phien moi la su that", kind, want, intsString(actual))))
	}
	return lines
}

func reportLog(o Options) []Line {
	lines := []Line{}
	logFile := o.Paths.LogFile
	if info, err := os.Stat(logFile); err != nil {
		lines = append(lines, missLine(fmt.Sprintf("khong co core.log (%s)", logFile)))
	} else {
		lines = append(lines, okLine(fmt.Sprintf("core.log: %s (%d byte, sua lan cuoi %s)", logFile, info.Size(), info.ModTime().Format(time.RFC3339))))
		for i := 1; i <= 2; i++ {
			name := logFile + "." + strconv.Itoa(i)
			if info, err := os.Stat(name); err == nil {
				lines = append(lines, okLine(fmt.Sprintf("%s: %d byte, sua lan cuoi %s", filepath.Base(name), info.Size(), info.ModTime().Format(time.RFC3339))))
			}
		}
	}
	lines = append(lines, rawLine("nguong xoay: core.log >10MB, giu 3 file (core.log, core.log.1, core.log.2); doctor khong xoay va khong xoa log"))
	bridgeLog := filepath.Join(o.BridgeDir, "bridge.log")
	if info, err := os.Stat(bridgeLog); err != nil {
		lines = append(lines, missLine(fmt.Sprintf("khong co bridge.log (%s)", bridgeLog)))
	} else {
		lines = append(lines, okLine(fmt.Sprintf("bridge.log: %s (%d byte, sua lan cuoi %s)", bridgeLog, info.Size(), info.ModTime().Format(time.RFC3339))))
	}
	lines = append(lines, rawLine("bridge.log do extension LibreOffice ghi; bridge WPS nam trong Core nen ghi vao core.log"))
	if o.Paths.IsOverride && o.Paths.Root != o.BridgeDir {
		lines = append(lines, warnLine(fmt.Sprintf("AXIOM_CORE_DATA_DIR duoc dat: log Core o %s, log bridge o %s - hai noi khac nhau", o.Paths.Root, o.BridgeDir)))
	}
	return lines
}

func reportEnv(o Options) []Line {
	found := map[string]string{}
	for _, kv := range o.Environ {
		name, value, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(name, "AXIOM_") {
			continue
		}
		// Lay gia tri dau tien thay duoc, giong cach moi truong hoat dong.
		if _, dup := found[name]; !dup {
			found[name] = value
		}
	}
	if len(found) == 0 {
		return []Line{
			missLine("khong co bien AXIOM_* nao duoc dat - dang dung cau hinh nen tang/mac dinh"),
		}
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := []Line{}
	for _, name := range names {
		value := found[name]
		if secretKeys[name] {
			if value == "" {
				lines = append(lines, missLine(fmt.Sprintf("%s: trong (chua dat)", name)))
			} else {
				lines = append(lines, okLine(fmt.Sprintf("%s: da dat (%d ky tu) (gia tri duoc che)", name, len(value))))
			}
			continue
		}
		lines = append(lines, okLine(fmt.Sprintf("%s=%s", name, value)))
	}
	return lines
}

func containsInt(values []int, want int) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func intsString(values []int) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, strconv.Itoa(v))
	}
	return strings.Join(parts, ",")
}

// Run la vo mong cho CLI: dung Options tu moi truong that roi in ra out.
// Tra 0 khi in duoc bao cao (ke ca khi co dong [!!]); khac 0 chi khi khong
// ghi duoc bao cao ra out.
func Run(cfg config.Config, version string, out io.Writer) int {
	paths := config.NewPaths(cfg.DataDirOverride)
	sessionDir := cfg.SessionDirectoryOverride
	if sessionDir == "" {
		sessionDir = config.DefaultSessionDir()
	}
	o := Options{
		Cfg:        cfg,
		Paths:      paths,
		SessionDir: sessionDir,
		BridgeDir:  config.DefaultRoot(),
		Source:     config.DefaultSource(),
		Environ:    os.Environ(),
		Version:    version,
		Protocol:   api.Protocol,
		Now:        time.Now(),
	}
	for _, l := range Report(o) {
		if _, err := io.WriteString(out, l.String()+"\n"); err != nil {
			return 1
		}
	}
	return 0
}
