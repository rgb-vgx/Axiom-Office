package xlsx

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"axiomoffice/core/internal/filesafe"
	"axiomoffice/core/internal/ooxml"
)

// ReadXLS doc file .xls (BIFF cu) bang cach nho LibreOffice doi sang .xlsx roi doc nhu file thuong
// (LibreOffice_arch.md muc 11). May khong co LibreOffice thi bao ro.
// allSheets=false thi chi doc sheet chi dinh (rong = sheet dau tien cua workbook).
func ReadXLS(path, target string, allSheets bool) ([]*Grid, error) {
	converted, directory, err := convertToXLSX(path)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)

	pkg, err := ooxml.OpenRead(converted)
	if err != nil {
		return nil, err
	}
	book, err := Open(pkg)
	if err != nil {
		return nil, err
	}
	names := book.SheetNames()
	if !allSheets && target != "" {
		names = []string{target}
	}
	grids := []*Grid{}
	for _, name := range names {
		reference, err := book.FindSheet(name)
		if err != nil {
			return nil, err
		}
		grid, err := book.Read(reference, false, 1, math.MaxInt32, 1, math.MaxInt32)
		if err != nil {
			return nil, err
		}
		grids = append(grids, grid)
	}
	return grids, nil
}

// convertToXLSX chay soffice --convert-to xlsx trong thu muc tam. Tra ve duong dan file xlsx va
// thu muc tam (ben goi co trach nhiem xoa).
func convertToXLSX(path string) (string, string, error) {
	soffice := FindSoffice()
	if soffice == "" {
		return "", "", fmt.Errorf("reading .xls needs LibreOffice (soffice) on this machine - or save the file as .xlsx")
	}
	directory, err := os.MkdirTemp("", "axiom-mcp-xls-")
	if err != nil {
		return "", "", err
	}
	cleanup := func() { os.RemoveAll(directory) }

	// Profile rieng: LibreOffice dang mo ma dung chung profile thi no chi chuyen tiep lenh
	// roi thoat ma khong doi file.
	profile := filepath.Join(directory, "lo-profile")
	if err := os.MkdirAll(profile, 0o755); err != nil {
		cleanup()
		return "", "", err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		cleanup()
		return "", "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, soffice,
		"-env:UserInstallation="+fileURI(profile),
		"--headless", "--norestore",
		"--convert-to", "xlsx",
		"--outdir", directory,
		absolute)
	command.Dir = directory
	output, _ := command.CombinedOutput()
	if ctx.Err() != nil {
		cleanup()
		return "", "", fmt.Errorf("soffice did not finish converting %s within 120s", path)
	}

	converted := filepath.Join(directory, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+".xlsx")
	if !filesafe.Exists(converted) {
		cleanup()
		return "", "", fmt.Errorf("soffice could not convert %s to xlsx: %s", path, strings.TrimSpace(string(output)))
	}
	return converted, directory, nil
}

// fileURI doi duong dan tuyet doi thanh URL file:// - phai co ba dau "/" (file:///C:/... tren
// Windows, file:///tmp/... tren Linux) neu khong LibreOffice se khong dung profile rieng.
func fileURI(path string) string {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file://" + slashed
}

// FindSoffice tim LibreOffice: bien AXIOM_SOFFICE, cac duong dan quen thuoc, roi den PATH.
func FindSoffice() string {
	if configured := os.Getenv("AXIOM_SOFFICE"); configured != "" && filesafe.Exists(configured) {
		return configured
	}
	known := []string{
		"/usr/bin/soffice", "/usr/lib/libreoffice/program/soffice", "/snap/bin/libreoffice",
		"/usr/lib64/libreoffice/program/soffice", "/opt/libreoffice/program/soffice",
		`C:\Program Files\LibreOffice\program\soffice.exe`,
		`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
	}
	if programFiles := os.Getenv("ProgramW6432"); programFiles != "" {
		known = append(known, filepath.Join(programFiles, "LibreOffice", "program", "soffice.exe"))
	}
	for _, candidate := range known {
		if filesafe.Exists(candidate) {
			return candidate
		}
	}
	for _, folder := range filepath.SplitList(os.Getenv("PATH")) {
		if folder == "" {
			continue
		}
		for _, name := range []string{"soffice", "soffice.exe"} {
			candidate := filepath.Join(folder, name)
			if filesafe.Exists(candidate) {
				return candidate
			}
		}
	}
	return ""
}
