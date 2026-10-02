package cfb

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("thieu file mau %s: %v", name, err)
	}
	return data
}

// Mot file .xls that la mot thung CFB, va stream chua bang tinh ten la "Workbook".
func TestOpenReadsWorkbookStream(t *testing.T) {
	data := fixture(t, "sample.xls")
	if !IsCFB(data) {
		t.Fatal("file mau phai la thung CFB")
	}
	file, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	workbook, ok := file.Stream("Workbook")
	if !ok {
		t.Fatalf("khong thay stream 'Workbook'; co: %v", file.StreamNames())
	}
	if len(workbook) == 0 {
		t.Fatal("stream Workbook rong")
	}
	if file.sectorSize != 512 {
		t.Fatalf("file .xls nho dung sector 512, duoc %d", file.sectorSize)
	}
	if _, ok := file.Stream("workbook"); !ok {
		t.Fatal("ten stream phai khong phan biet hoa thuong")
	}
}

// Du lieu tra ve phai DUNG TUNG BYTE, tren CA HAI duong ma CFB dung de chua stream:
//
//   - stream nho hon 4096 byte nam trong "mini stream" (tung khuc 64 byte, di theo mini FAT) - file .xls
//     nho di duong nay;
//   - stream lon nam trong cac sector thuong (di theo FAT) - file .xls co du lieu that di duong nay.
//
// Bo sot mot trong hai thi mot nua code khong bao gio duoc chay. Phep doi chieu o day tu lan chuoi
// theo cach rieng, khong dung code cua package.
func TestWorkbookStreamMatchesRawChain(t *testing.T) {
	for _, name := range []string{"sample.xls", "large.xls"} {
		data := fixture(t, name)
		file, err := OpenBytes(data)
		if err != nil {
			t.Fatalf("%s: OpenBytes: %v", name, err)
		}
		workbook, ok := file.Stream("Workbook")
		if !ok {
			t.Fatalf("%s: khong thay 'Workbook'", name)
		}

		entry := findDirectoryEntry(t, data, "Workbook")
		start := int(leUint32(entry[116:120]))
		size := int(leUint32(entry[120:124]))
		expected := followChain(t, data, start, size)

		if len(workbook) != len(expected) {
			t.Fatalf("%s: do dai stream %d, tu lan chuoi %d", name, len(workbook), len(expected))
		}
		if !bytes.Equal(workbook, expected) {
			for i := range expected {
				if workbook[i] != expected[i] {
					t.Fatalf("%s: byte %d khac nhau: %02x vs %02x", name, i, workbook[i], expected[i])
				}
			}
		}
		where := "sector thuong"
		if size < miniStreamCutoff {
			where = "mini stream"
		}
		t.Logf("%s: stream Workbook %d byte, di duong %s", name, size, where)
	}
}

// File hong khong duoc lam treo hay panic: cat cut o moi kich thuoc.
func TestCorruptFilesDoNotHang(t *testing.T) {
	data := fixture(t, "sample.xls")
	for cut := 0; cut < len(data); cut += 997 {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("cat o %d byte lam panic: %v", cut, recovered)
				}
			}()
			_, _ = OpenBytes(data[:cut])
		}()
	}
	if _, err := OpenBytes([]byte("khong phai thung nao ca")); err == nil {
		t.Fatal("du lieu la phai bao loi")
	}
	if _, err := OpenBytes(data); err != nil {
		t.Fatalf("file nguyen ven phai mo duoc: %v", err)
	}
}

// Byte dau la thu phan biet .xls voi .xlsx; so nham thu tu byte la hong ngay tai cua.
func TestIsCFBReadsTheSignature(t *testing.T) {
	if !IsCFB(fixture(t, "sample.xls")) {
		t.Fatal("phai nhan ra thung CFB")
	}
	if IsCFB([]byte("PK\x03\x04")) {
		t.Fatal("zip (OOXML) khong phai thung CFB")
	}
	if IsCFB([]byte{0xD0, 0xCF, 0x11}) {
		t.Fatal("du lieu ngan hon chu ky phai tra ve false")
	}
}

// Ten trong thu muc CFB la UTF-16LE, khac han phan con lai cua BIFF (8-bit hoac UTF-16 theo co). Doc sai
// cho nay thi khong bao gio tim thay "Workbook".
func TestStreamNameDecoding(t *testing.T) {
	file, err := OpenBytes(fixture(t, "sample.xls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range file.StreamNames() {
		if strings.ContainsRune(name, 0) {
			t.Fatalf("ten stream con ky tu NUL: %q", name)
		}
	}
}

// --- cac ham doc doc lap, de doi chieu ---

func followChain(t *testing.T, data []byte, start, size int) []byte {
	t.Helper()
	out := []byte{}
	sector := start
	if size < miniStreamCutoff {
		root := findDirectoryEntry(t, data, "Root Entry")
		miniStream := readSectors(t, data, int(leUint32(root[116:120])), int(leUint32(root[120:124])), 512)
		table := readMiniFat(t, data)
		for guard := 0; guard < 200000; guard++ {
			offset := sector * 64
			out = append(out, miniStream[offset:offset+64]...)
			if len(out) >= size {
				break
			}
			sector = int(table[sector])
		}
		return out[:size]
	}
	table := readFAT(t, data)
	for guard := 0; guard < 200000; guard++ {
		offset := (sector + 1) * 512
		out = append(out, data[offset:offset+512]...)
		if len(out) >= size {
			break
		}
		sector = int(table[sector])
	}
	return out[:size]
}

func readSectors(t *testing.T, data []byte, start, size, sectorSize int) []byte {
	t.Helper()
	out := []byte{}
	for sector, guard := start, 0; guard < 200000; guard++ {
		offset := (sector + 1) * sectorSize
		out = append(out, data[offset:offset+sectorSize]...)
		if len(out) >= size {
			break
		}
		sector = int(readFAT(t, data)[sector])
	}
	return out[:size]
}

func readFAT(t *testing.T, data []byte) []uint32 {
	t.Helper()
	count := int(leUint32(data[44:48]))
	fat := []uint32{}
	for i := 0; i < 109 && i < count; i++ {
		sector := int(leUint32(data[76+i*4 : 80+i*4]))
		offset := (sector + 1) * 512
		for j := 0; j < 128; j++ {
			fat = append(fat, leUint32(data[offset+j*4:offset+j*4+4]))
		}
	}
	return fat
}

func readMiniFat(t *testing.T, data []byte) []uint32 {
	t.Helper()
	start := int(leUint32(data[60:64]))
	count := int(leUint32(data[64:68]))
	raw := readSectors(t, data, start, count*512, 512)
	table := make([]uint32, 0, len(raw)/4)
	for i := 0; i+4 <= len(raw); i += 4 {
		table = append(table, leUint32(raw[i:i+4]))
	}
	return table
}

func findDirectoryEntry(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	first := int(leUint32(data[48:52]))
	offset := (first + 1) * 512
	for i := 0; i < 512/directoryEntrySize*8; i++ {
		entry := data[offset+i*directoryEntrySize : offset+(i+1)*directoryEntrySize]
		length := int(leUint16(entry[64:66]))
		if length >= 2 && length <= 64 && decodeUTF16(entry[:length-2]) == name {
			return entry
		}
	}
	t.Fatalf("khong thay muc thu muc '%s'", name)
	return nil
}

func leUint16(data []byte) uint16 { return uint16(data[0]) | uint16(data[1])<<8 }

func leUint32(data []byte) uint32 {
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
}
