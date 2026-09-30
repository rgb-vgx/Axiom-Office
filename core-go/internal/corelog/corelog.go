// Package corelog ghi core.log cung dinh dang ban .NET (CoreLog.cs):
// "yyyy-MM-dd HH:mm:ss.fff [LEVEL] [pid N] message", xoay khi > 10MB, giu 3 file.
package corelog

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	mu       sync.Mutex
	path     string
	maxBytes int64 = 10 * 1024 * 1024
	keep           = 3
	console        = isConsole()
)

// Chi in ra man hinh khi stdout la terminal (giong Console.IsOutputRedirected cua ban .NET).
func isConsole() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Init dat file log (goi mot lan luc khoi dong).
func Init(file string) {
	mu.Lock()
	defer mu.Unlock()
	path = file
}

// SetLimits cho test xoay file.
func SetLimits(max int64, files int) {
	mu.Lock()
	defer mu.Unlock()
	maxBytes, keep = max, files
	if keep < 1 {
		keep = 1
	}
}

func Info(format string, args ...any) { write("INFO", fmt.Sprintf(format, args...)) }

func Error(format string, args ...any) { write("ERROR", fmt.Sprintf(format, args...)) }

func write(level, message string) {
	line := time.Now().Format("2006-01-02 15:04:05.000") + " [" + level + "] [pid " + fmt.Sprint(os.Getpid()) + "] " + message
	mu.Lock()
	defer mu.Unlock()
	if path != "" {
		rotate()
		if file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			_, _ = file.WriteString(line + "\n")
			_ = file.Close()
		}
		// Khong ghi duoc log (dia day, bi khoa): bo qua, khong duoc lam Core dung.
	}
	if console {
		fmt.Println(line)
	}
}

// core.log -> core.log.1 -> ... -> core.log.(keep-1): keep la TONG so file giu lai.
func rotate() {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxBytes {
		return
	}
	if keep <= 1 {
		_ = os.Remove(path)
		return
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", path, keep-1))
	for i := keep - 2; i >= 1; i-- {
		source := fmt.Sprintf("%s.%d", path, i)
		if _, err := os.Stat(source); err == nil {
			_ = os.Rename(source, fmt.Sprintf("%s.%d", path, i+1))
		}
	}
	_ = os.Rename(path, path+".1")
}
