package corelog

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Cung ky vong voi CoreLogTests.cs cua ban .NET.
func TestLineFormat(t *testing.T) {
	file := filepath.Join(t.TempDir(), "core.log")
	Init(file)
	t.Cleanup(func() { Init("") })

	Info("Agent Core ready")

	text := readFile(t, file)
	pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} \[INFO\] \[pid \d+\] Agent Core ready\n$`)
	if !pattern.MatchString(text) {
		t.Fatalf("dong log sai dinh dang: %q", text)
	}
	if !strings.Contains(text, "[pid "+strconv.Itoa(os.Getpid())+"]") {
		t.Fatalf("thieu pid: %q", text)
	}
}

func TestErrorDetail(t *testing.T) {
	file := filepath.Join(t.TempDir(), "core.log")
	Init(file)
	t.Cleanup(func() { Init("") })

	ErrorDetail("khong ghi duoc file", errors.New("dia day"))
	ErrorDetail("chi co thong diep", nil)
	ErrorDetail("het gio", io.EOF)
	ErrorDetail("mo file", &os.PathError{Op: "open", Path: "/x/y", Err: errors.New("no such file")})

	text := readFile(t, file)
	for _, want := range []string{
		"[ERROR]",
		// errors.New -> khong co ten kieu (kieu noi bo "errorString" chi lam nhieu log)
		"khong ghi duoc file: dia day\n",
		"chi co thong diep\n",
		"het gio: EOF\n",
		// loi co kieu that -> giu ten kieu, giong GetType().Name cua ban .NET
		"mo file: PathError: open /x/y: no such file",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("thieu %q trong:\n%s", want, text)
		}
	}
}

func TestRotateKeepsConfiguredFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "core.log")
	Init(file)
	SetLimits(300, 2)
	t.Cleanup(func() {
		Init("")
		SetLimits(10*1024*1024, 3)
	})

	for index := range 60 {
		Info("dong so %d %s", index, strings.Repeat("x", 40))
	}

	if _, err := os.Stat(file); err != nil {
		t.Fatalf("core.log phai ton tai: %v", err)
	}
	if _, err := os.Stat(file + ".1"); err != nil {
		t.Fatalf("phai co file xoay core.log.1: %v", err)
	}
	if _, err := os.Stat(file + ".2"); err == nil {
		t.Fatal("keep = 2 nen khong duoc giu core.log.2")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("chi duoc co 2 file, dang co %d", len(entries))
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
