// Package cfb doc thung Compound File Binary (OLE2) - dinh dang chua file .xls, .doc, .ppt doi cu.
//
// Chi doc. Khong phu thuoc gi: OOXML la zip (nen `internal/ooxml` dung archive/zip), con .xls la mot
// "thung" rieng voi bang FAT ben trong, nen phai co reader cua no.
//
// Vi sao can: doc .xls truoc day nho LibreOffice doi sang .xlsx, tuc la may nguoi dung phai cai
// LibreOffice. Doc thang ra thi khong can gi (xem internal/xlsx/biff.go).
package cfb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
)

// signature: 8 byte dau cua moi thung CFB. So sanh BYTE chu khong qua uint32 - so nham thu tu byte
// (D0CF11E0 doc kieu little-endian ra 0xE011CFD0) la mot loi tung lam hong ca ham nhan dien.
var signature = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

const (
	headerSize = 512

	// Gia tri dac biet trong bang FAT.
	endOfChain = 0xFFFFFFFE
	freeSector = 0xFFFFFFFF

	// miniStreamCutoff: stream nho hon nguong nay nam trong mini stream (tung khuc 64 byte) chu khong
	// chiem sector day.
	miniStreamCutoff = 4096

	directoryEntrySize = 128
)

// File: mot thung da mo. Du lieu doc het vao bo nho khi Open (file .xls thuong chi vai tram KB).
type File struct {
	data       []byte
	sectorSize int
	miniSize   int
	fat        []uint32
	miniFat    []uint32
	directory  []byte
	miniStream []byte
	streams    map[string][]byte
}

// IsCFB: du lieu co phai mot thung CFB khong (doc 8 byte dau).
func IsCFB(data []byte) bool {
	return len(data) >= len(signature) && bytes.Equal(data[:len(signature)], signature[:])
}

// Open doc thung tu duong dan.
func Open(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return OpenBytes(data)
}

// OpenBytes doc thung tu bo nho.
func OpenBytes(data []byte) (*File, error) {
	if !IsCFB(data) {
		return nil, errors.New("not a compound file (missing the OLE2 signature)")
	}
	if len(data) < headerSize {
		return nil, errors.New("compound file is shorter than its header")
	}
	sectorShift := int(binary.LittleEndian.Uint16(data[30:32]))
	if sectorShift < 7 || sectorShift > 20 {
		return nil, fmt.Errorf("unsupported sector shift %d", sectorShift)
	}
	miniShift := int(binary.LittleEndian.Uint16(data[32:34]))
	if miniShift < 4 || miniShift > sectorShift {
		return nil, fmt.Errorf("unsupported mini sector shift %d", miniShift)
	}

	f := &File{
		data:       data,
		sectorSize: 1 << sectorShift,
		miniSize:   1 << miniShift,
		streams:    map[string][]byte{},
	}
	if err := f.loadFat(); err != nil {
		return nil, err
	}
	if err := f.loadDirectory(); err != nil {
		return nil, err
	}
	if err := f.loadStreams(); err != nil {
		return nil, err
	}
	return f, nil
}

// Stream tra ve noi dung mot stream theo ten (khong phan biet hoa thuong; "Workbook" cua .xls).
func (f *File) Stream(name string) ([]byte, bool) {
	data, ok := f.streams[strings.ToLower(name)]
	return data, ok
}

// StreamNames liet ke ten cac stream (de chan doan khi khong tim thay "Workbook").
func (f *File) StreamNames() []string {
	names := make([]string, 0, len(f.streams))
	for name := range f.streams {
		names = append(names, name)
	}
	return names
}

// sectorOffset: sector thu n duoc danh so lien tuc bo qua header, nen sector n bat dau o (n+1)*size.
func (f *File) sectorOffset(sector uint32) (int, bool) {
	start := (int(sector) + 1) * f.sectorSize
	if start < 0 || start+f.sectorSize > len(f.data) {
		return 0, false
	}
	return start, true
}

// maxChain: chan tren so khuc khi lan theo mot chuoi, de file hong khong lam treo (chuoi vong).
func (f *File) maxChain() int {
	return len(f.data)/f.sectorSize + 2
}

// loadFat doc bang FAT. Voi file lon, danh sach sector cua FAT nam rai trong cac DIFAT sector.
func (f *File) loadFat() error {
	count := int(binary.LittleEndian.Uint32(f.data[44:48]))
	if count > f.maxChain() {
		return fmt.Errorf("fat sector count %d is beyond the file size", count)
	}

	// 109 muc DIFAT dau tien nam ngay trong header.
	entries := make([]uint32, 0, count)
	for i := 0; i < 109 && len(entries) < count; i++ {
		entries = append(entries, binary.LittleEndian.Uint32(f.data[76+i*4:80+i*4]))
	}

	// Phan con lai nam trong cac DIFAT sector, noi tiep nhau.
	next := binary.LittleEndian.Uint32(f.data[68:72])
	perSector := f.sectorSize/4 - 1
	for guard := 0; next != endOfChain && next != freeSector && len(entries) < count; guard++ {
		if guard > f.maxChain() {
			return errors.New("difat chain is broken")
		}
		offset, ok := f.sectorOffset(next)
		if !ok {
			return errors.New("difat sector is outside the file")
		}
		for i := 0; i < perSector && len(entries) < count; i++ {
			entries = append(entries, binary.LittleEndian.Uint32(f.data[offset+i*4:offset+i*4+4]))
		}
		next = binary.LittleEndian.Uint32(f.data[offset+perSector*4 : offset+perSector*4+4])
	}

	f.fat = make([]uint32, 0, len(entries)*perSector+perSector)
	for _, sector := range entries {
		offset, ok := f.sectorOffset(sector)
		if !ok {
			// Sector FAT nam ngoai file: thung hong, nhung phan doc duoc van dung duoc.
			continue
		}
		for i := 0; i < f.sectorSize/4; i++ {
			f.fat = append(f.fat, binary.LittleEndian.Uint32(f.data[offset+i*4:offset+i*4+4]))
		}
	}
	if len(f.fat) == 0 {
		return errors.New("compound file has no FAT")
	}
	return nil
}

// loadDirectory doc bang thu muc (moi muc 128 byte) va mini FAT.
func (f *File) loadDirectory() error {
	start := binary.LittleEndian.Uint32(f.data[48:52])
	directory, err := f.readChain(start, -1, false)
	if err != nil {
		return fmt.Errorf("directory: %w", err)
	}
	f.directory = directory

	// Mini FAT: chuoi sector rieng, moi muc 4 byte tro tới khuc ke trong mini stream.
	miniStart := binary.LittleEndian.Uint32(f.data[60:64])
	miniCount := int(binary.LittleEndian.Uint32(f.data[64:68]))
	if miniCount > 0 && miniStart != endOfChain {
		raw, err := f.readChain(miniStart, miniCount*f.sectorSize, false)
		if err == nil {
			f.miniFat = make([]uint32, 0, len(raw)/4)
			for i := 0; i+4 <= len(raw); i += 4 {
				f.miniFat = append(f.miniFat, binary.LittleEndian.Uint32(raw[i:i+4]))
			}
		}
	}
	return nil
}

// loadStreams doc moi stream trong thu muc. Stream nho nam trong mini stream cua muc goc, stream lon
// nam trong cac sector thuong.
func (f *File) loadStreams() error {
	for offset := 0; offset+directoryEntrySize <= len(f.directory); offset += directoryEntrySize {
		entry := f.directory[offset : offset+directoryEntrySize]
		kind := entry[66]
		nameLength := int(binary.LittleEndian.Uint16(entry[64:66]))
		if kind == 0 || nameLength < 2 || nameLength > 64 {
			continue
		}
		name := decodeUTF16(entry[0 : nameLength-2])
		if name == "" {
			continue
		}
		start := binary.LittleEndian.Uint32(entry[116:120])
		size := int(binary.LittleEndian.Uint32(entry[120:124]))

		if kind == 5 { // muc goc: chinh la mini stream
			mini, err := f.readChain(start, size, false)
			if err == nil {
				f.miniStream = mini
			}
			continue
		}
		if kind != 2 { // 1 = storage (thu muc con), khong phai stream
			continue
		}
		var data []byte
		var err error
		if size < miniStreamCutoff {
			data, err = f.readChain(start, size, true)
		} else {
			data, err = f.readChain(start, size, false)
		}
		if err != nil {
			continue // mot stream hong khong lam hong ca thung
		}
		f.streams[strings.ToLower(name)] = data
	}
	return nil
}

// readChain lan theo chuoi sector (hoac khuc mini) va tra ve du lieu.
//
// size < 0 = doc den het chuoi (dung cho thu muc, noi khong biet truoc do dai).
func (f *File) readChain(start uint32, size int, mini bool) ([]byte, error) {
	unit := f.sectorSize
	table := f.fat
	if mini {
		unit = f.miniSize
		table = f.miniFat
	}
	limit := f.maxChain() * 4
	out := []byte{}
	sector := start
	for guard := 0; sector != endOfChain && sector != freeSector; guard++ {
		if guard > limit {
			return nil, errors.New("sector chain is broken (loop?)")
		}
		offset := int(sector) * unit
		var chunk []byte
		if mini {
			if offset < 0 || offset+unit > len(f.miniStream) {
				return nil, errors.New("mini sector is outside the mini stream")
			}
			chunk = f.miniStream[offset : offset+unit]
		} else {
			absolute, ok := f.sectorOffset(sector)
			if !ok {
				return nil, errors.New("sector is outside the file")
			}
			chunk = f.data[absolute : absolute+unit]
		}
		out = append(out, chunk...)
		if size >= 0 && len(out) >= size {
			break
		}
		if int(sector) >= len(table) {
			break
		}
		sector = table[sector]
	}
	if size >= 0 && len(out) > size {
		out = out[:size]
	}
	if size > 0 && len(out) == 0 {
		return nil, errors.New("stream is empty")
	}
	return out, nil
}

// decodeUTF16 doc chuoi UTF-16LE (ten muc trong thu muc CFB la UTF-16, khac voi phan con lai cua BIFF).
func decodeUTF16(data []byte) string {
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+2 <= len(data); i += 2 {
		units = append(units, binary.LittleEndian.Uint16(data[i:i+2]))
	}
	return string(utf16.Decode(units))
}
