package doctor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"axiomoffice/core/internal/api"
	"axiomoffice/core/internal/config"
	"axiomoffice/core/internal/corefile"
)

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("ghi loi") }

func testOptions(root, sessionDir string) Options {
	cfg := config.Config{CorePort: 47840, SingleInstance: true, MutexName: "test-mutex"}
	paths := config.NewPaths(root)
	return Options{
		Cfg: cfg, Paths: paths, SessionDir: sessionDir,
		BridgeDir: root, Source: config.DefaultSource(),
		Environ: nil, Version: "0.1.0", Protocol: api.Protocol, Now: time.Now(),
	}
}

func reportText(o Options) string {
	lines := Report(o)
	var out strings.Builder
	for _, l := range lines {
		out.WriteString(l.String() + "\n")
	}
	return out.String()
}

func TestNoCoreJSON(t *testing.T) {
	root := t.TempDir()
	sessionDir := t.TempDir()
	text := reportText(testOptions(root, sessionDir))
	if !strings.Contains(text, "khong co core.json") {
		t.Fatalf("phai co dong khong co core.json, duoc:\n%s", text)
	}
	if strings.Contains(text, "con song") {
		t.Fatalf("khong co core.json thi khong duoc co dong pid con song, duoc:\n%s", text)
	}
}

func TestDeadCoreJSON(t *testing.T) {
	root := t.TempDir()
	sessionDir := t.TempDir()
	paths := config.NewPaths(root)
	corefile.Write(paths.CoreJSON, corefile.Info{Pid: 0, Port: 47840, Version: "0.1.0", Started: "x", Protocol: api.Protocol, Exe: "exe"})
	text := reportText(testOptions(root, sessionDir))
	if !strings.Contains(text, "[!!]") || !strings.Contains(text, "khong con song") {
		t.Fatalf("pid 0 phai bao [!!] khong con song, duoc:\n%s", text)
	}
}

func TestLiveSessionShown(t *testing.T) {
	root := t.TempDir()
	sessionDir := t.TempDir()
	pid := os.Getpid()
	port := 47899
	epoch := float64(time.Now().UnixMilli()) / 1000.0
	data := fmt.Sprintf(`{"pid":%d,"port":%d,"app":"wps","family":"wps-linux","host":"h","version":"v1","lastSeenEpoch":%f,"document":"tai lieu","documentPath":"/tmp/x"}`,
		pid, port, epoch)
	if err := os.WriteFile(filepath.Join(sessionDir, "100.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	text := reportText(testOptions(root, sessionDir))
	if !strings.Contains(text, "wps") || !strings.Contains(text, strconv.Itoa(port)) || !strings.Contains(text, strconv.Itoa(pid)) {
		t.Fatalf("phai in app/port/pid cua phien, duoc:\n%s", text)
	}
}

func TestSecretsMasked(t *testing.T) {
	root := t.TempDir()
	sessionDir := t.TempDir()
	o := testOptions(root, sessionDir)
	o.Environ = []string{"AXIOM_LLM_API_KEY=bi-mat-khong-duoc-lo", "AXIOM_TOKEN=token-khong-duoc-lo", "PATH=/usr/bin"}
	text := reportText(o)
	if strings.Contains(text, "bi-mat-khong-duoc-lo") || strings.Contains(text, "token-khong-duoc-lo") {
		t.Fatalf("bi mat khong duoc xuat hien trong bao cao, duoc:\n%s", text)
	}
	if !strings.Contains(text, "AXIOM_LLM_API_KEY") || !strings.Contains(text, "AXIOM_TOKEN") {
		t.Fatalf("phai co ten hai bien bi mat, duoc:\n%s", text)
	}
}

func TestBridgePortsFromConfig(t *testing.T) {
	root := t.TempDir()
	sessionDir := t.TempDir()
	o := testOptions(root, sessionDir)
	text := reportText(o)
	for _, kind := range []string{"wps", "et", "wpp"} {
		want := strconv.Itoa(config.BridgePortForKind(o.Source, kind, false))
		if !strings.Contains(text, want) {
			t.Fatalf("bao cao phai chua cong %s cua kind %s, duoc:\n%s", want, kind, text)
		}
	}
}

func TestRunExitCodes(t *testing.T) {
	root := t.TempDir()
	sessionDir := t.TempDir()
	cfg := config.Config{CorePort: 47840, SingleInstance: true, MutexName: "m", DataDirOverride: root, SessionDirectoryOverride: sessionDir}
	if code := Run(cfg, "0.1.0", errWriter{}); code == 0 {
		t.Fatal("ghi ra writer loi phai tra khac 0")
	}
	var buf bytes.Buffer
	if code := Run(cfg, "0.1.0", &buf); code != 0 {
		t.Fatalf("Run binh thuong phai tra 0, duoc %d", code)
	}
	if buf.Len() == 0 {
		t.Fatal("Run binh thuong phai in bao cao")
	}
}
