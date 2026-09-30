package corefile

import (
	"os"
	"path/filepath"
	"testing"
)

// Cung ky vong voi CoreFileTests.cs cua ban .NET: add-in doc core.json de tim Core (muc 7.1).
func sample(port int) Info {
	return Info{
		Pid: 4242, Port: port, Version: "1.0.0", Started: "2026-10-01T08:00:00.000Z",
		Protocol: 1, Exe: `C:\thu muc co dau\AxiomOffice.Core.exe`,
	}
}

func TestWriteThenReadBack(t *testing.T) {
	file := filepath.Join(t.TempDir(), "core.json")
	Write(file, sample(47840))

	info, ok := Read(file)
	if !ok {
		t.Fatal("phai doc lai duoc core.json")
	}
	if info.Pid != 4242 || info.Port != 47840 || info.Version != "1.0.0" || info.Protocol != 1 ||
		info.Exe != `C:\thu muc co dau\AxiomOffice.Core.exe` || info.Started == "" {
		t.Fatalf("thieu truong sau khi doc lai: %+v", info)
	}
}

func TestSecondWriteWinsAndNoTempFileLeft(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "core.json")
	Write(file, sample(47840))
	Write(file, sample(47841))

	if info, _ := Read(file); info.Port != 47841 {
		t.Fatalf("lan ghi sau phai thay the: %+v", info)
	}
	if _, err := os.Stat(file + ".tmp"); err == nil {
		t.Fatal("khong duoc de lai file .tmp")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("chi duoc co mot file trong thu muc, dang co %d", len(entries))
	}
}

func TestReadMissingAndBrokenFileIsSafe(t *testing.T) {
	dir := t.TempDir()
	if _, ok := Read(filepath.Join(dir, "khong-co.json")); ok {
		t.Fatal("file khong ton tai -> phai tra false")
	}

	broken := filepath.Join(dir, "hong.json")
	if err := os.WriteFile(broken, []byte("{ khong phai json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(broken); ok {
		t.Fatal("file hong -> phai tra false, khong nem loi")
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	file := filepath.Join(t.TempDir(), "core.json")
	Write(file, sample(47840))
	Delete(file)
	if _, err := os.Stat(file); err == nil {
		t.Fatal("Delete phai xoa file")
	}

	Delete(file) // lan hai: khong duoc nem loi
	if _, err := os.Stat(file); err == nil {
		t.Fatal("van con file sau khi Delete lan hai")
	}
}

func TestWriteCreatesParentDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "chua", "co", "core.json")
	Write(file, sample(47840))
	if _, ok := Read(file); !ok {
		t.Fatal("phai tao duoc thu muc cha va ghi file")
	}
}
