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

	"axiomoffice/core/internal/cfb"
	"axiomoffice/core/internal/filesafe"
	"axiomoffice/core/internal/ooxml"
)

// ReadXLS doc file .xls (BIFF8) THANG, khong can cai them gi.
//
// Truoc day phai nho LibreOffice doi sang .xlsx, tuc la may nguoi dung phai co LibreOffice - trong khi
// ban Windows thi dung COM Excel/WPS. Doc thang thi khong can ca hai.
//
// Con duong LibreOffice van giu lam du phong cho thu ma reader truc tiep khong doc duoc: BIFF5/BIFF2
// (Excel 5 tro ve truoc) va thung CFB hong. May nao co LibreOffice thi van doc duoc chung.
//
// allSheets=false thi chi doc sheet chi dinh (rong = sheet dau tien cua workbook).
func ReadXLS(path, target string, allSheets bool) ([]*Grid, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var direct error
	if cfb.IsCFB(data) {
		var grids []*Grid
		grids, direct = ReadBIFF(data, target, allSheets)
		if direct == nil {
			return grids, nil
		}
	}
	return readViaLibreOffice(path, target, allSheets, direct)
}

// readViaLibreOffice: nho soffice doi sang .xlsx roi doc nhu file thuong (LibreOffice_arch.md muc 11).
// direct la loi cua duong doc thang (neu co) - bao ca hai de nguoi doc biet vi sao.
func readViaLibreOffice(path, target string, allSheets bool, direct error) ([]*Grid, error) {
	converted, directory, err := convertToXLSX(path)
	if err != nil {
		if direct != nil {
			return nil, fmt.Errorf("%v; %w", direct, err)
		}
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

// NoSoffice: gia tri cua AXIOM_SOFFICE de coi nhu may KHONG co LibreOffice.
//
// Can co vi day la mot tinh huong that (may Windows chi co Office/WPS) ma kho tai hien: dat duong dan
// sai thi FindSoffice van tim thay LibreOffice o cac duong quen thuoc. Co loi nay thi bai test chay
// duoc dung cai nguoi dung gap, thay vi chi chay duoc o noi co LibreOffice.
const NoSoffice = "none"

// FindSoffice tim LibreOffice: bien AXIOM_SOFFICE, cac duong dan quen thuoc, roi den PATH.
func FindSoffice() string {
	if configured := os.Getenv("AXIOM_SOFFICE"); configured != "" {
		if configured == NoSoffice {
			return ""
		}
		if filesafe.Exists(configured) {
			return configured
		}
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
