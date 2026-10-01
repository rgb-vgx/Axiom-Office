// Package filesafe: ghi file kieu atomic (ghi ra file tam roi thay the) va cac kiem tra duong dan.
// Port cua FileSafety trong src/AxiomOffice.Host/Mcp/OoxmlPackage.cs.
package filesafe

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Exists cho biet duong dan tro toi mot file thuong (khong phai thu muc).
func Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// TempPathFor tra ve duong dan file tam cung thu muc voi file dich, de buoc thay the chi la doi ten
// trong cung o dia (nguyen tu) chu khong phai chep xuyen o dia.
func TempPathFor(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	directory := filepath.Dir(absolute)
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return "", fmt.Errorf("folder does not exist: %s", directory)
	}
	return filepath.Join(directory, ".~"+filepath.Base(absolute)+"."+randomSuffix()+".tmp"), nil
}

func randomSuffix() string {
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return "00000000"
	}
	return fmt.Sprintf("%08x", buffer)
}

// Replace thay file dich bang file tam. Loi thuong gap nhat la file dang bi Word/Excel/WPS giu.
func Replace(temp, path string) error {
	if err := os.Rename(temp, path); err != nil {
		DeleteQuietly(temp)
		return fmt.Errorf("cannot write '%s': file is locked (open in Word/Excel/PowerPoint/WPS?) - close it first or use the live bridge tools", path)
	}
	return nil
}

func DeleteQuietly(path string) {
	_ = os.Remove(path)
}

// WriteAtomic ghi noi dung ra file tam roi thay the file dich; giu nguyen quyen cua file cu.
func WriteAtomic(path string, content []byte) error {
	temp, err := TempPathFor(path)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(temp, content, mode); err != nil {
		DeleteQuietly(temp)
		return err
	}
	return Replace(temp, path)
}

// RequireFile kiem tra duong dan ton tai (thong bao giong ban C#).
func RequireFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	if !Exists(path) {
		return fmt.Errorf("file not found: %s", path)
	}
	return nil
}

// RequireNewOrOverwrite chan ghi de khi overwrite=false.
func RequireNewOrOverwrite(path string, overwrite bool) error {
	if Exists(path) && !overwrite {
		return fmt.Errorf("file already exists - pass overwrite=true to replace it")
	}
	return nil
}
