package xlsx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"

	"axiomoffice/core/internal/cfb"
	"axiomoffice/core/internal/sheet"
)

// Doc file .xls (BIFF8 - Excel 97-2003) THANG, khong can LibreOffice cung khong can Excel.
//
// Truoc day .xls phai nho LibreOffice doi sang .xlsx (ban Linux) hoac COM Excel/WPS (ban Windows). Ca hai
// deu bat nguoi dung cai them mot thu ma ho khong nhat thiet co. Doc thang thi khong can gi.
//
// Chi BIFF8. BIFF5/BIFF2 (Excel 5 tro ve truoc) khac han ve bang chuoi va ma hoa, va gan nhu khong con
// trong thuc te - gap thi bao ro chu khong doan bua.
//
// Chi DOC gia tri. .xls truoc nay cung chi doc.

// Ma record BIFF8 dung o day.
const (
	recBOF        = 0x0809
	recEOF        = 0x000A
	recBoundSheet = 0x0085
	recSST        = 0x00FC
	recContinue   = 0x003C
	recXF         = 0x00E0
	recFormat     = 0x041E
	recDateMode   = 0x0022
	recDimensions = 0x0200

	recLabelSST = 0x00FD
	recNumber   = 0x0203
	recRK       = 0x027E
	recMulRK    = 0x00BD
	recFormula  = 0x0006
	recString   = 0x0207
	recBoolErr  = 0x0205
	recBlank    = 0x0201
	recMulBlank = 0x00BE

	// dt trong ban ghi BOF.
	bofWorkbookGlobals = 0x0005
	bofWorksheet       = 0x0010

	// BIFF8 la 0x0600; nho hon la BIFF cu hon.
	biff8Version = 0x0600
)

type biffRecord struct {
	id   uint16
	data []byte
}

// ReadBIFF doc mot file .xls tu bo nho. target rong = sheet dau tien; allSheets = moi sheet.
func ReadBIFF(data []byte, target string, allSheets bool) ([]*Grid, error) {
	file, err := cfb.OpenBytes(data)
	if err != nil {
		return nil, err
	}
	stream, ok := file.Stream("Workbook")
	if !ok {
		// BIFF5 dat ten stream la "Book"; ham nay se bao loi phien ban ngay sau day.
		stream, ok = file.Stream("Book")
		if !ok {
			return nil, fmt.Errorf("compound file has no Workbook stream (found: %s)", strings.Join(file.StreamNames(), ", "))
		}
	}

	book, err := parseWorkbookGlobals(stream)
	if err != nil {
		return nil, err
	}

	wanted := book.sheets
	if !allSheets {
		if target != "" {
			found := false
			for _, item := range book.sheets {
				if item.name == target {
					wanted = []sheetInfo{item}
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("sheet not found: %s", target)
			}
		} else if len(wanted) > 1 {
			wanted = wanted[:1]
		}
	}

	grids := make([]*Grid, 0, len(wanted))
	for _, info := range wanted {
		grid, err := book.readSheet(stream, info)
		if err != nil {
			return nil, err
		}
		grids = append(grids, grid)
	}
	return grids, nil
}

type sheetInfo struct {
	name     string
	position int
}

type workbookGlobals struct {
	sheets    []sheetInfo
	shared    []string
	xfFormats []int
	formats   map[int]string
	date1904  bool
}

// parseWorkbookGlobals doc phan dau cua stream: BOUNDSHEET (vi tri tung sheet), SST (bang chuoi dung
// chung), XF + FORMAT (de biet o nao la ngay), DATEMODE.
func parseWorkbookGlobals(stream []byte) (*workbookGlobals, error) {
	book := &workbookGlobals{formats: map[int]string{}}
	records := scanRecords(stream, 0)
	if len(records) == 0 {
		return nil, errors.New("workbook stream has no records")
	}
	if records[0].id != recBOF {
		return nil, errors.New("workbook stream does not start with a BOF record")
	}
	if version := leU16(records[0].data); version < biff8Version {
		return nil, fmt.Errorf("this .xls is BIFF %#x (Excel 5 or older); only BIFF8 (Excel 97-2003) is read directly - open and save it as .xlsx", version)
	}

	for index := 0; index < len(records); index++ {
		record := records[index]
		switch record.id {
		case recBoundSheet:
			info, err := parseBoundSheet(record.data)
			if err != nil {
				return nil, err
			}
			book.sheets = append(book.sheets, info)
		case recSST:
			// SST va cac CONTINUE ngay sau no la mot khoi: chuoi co the cat ngang ranh gioi record.
			chunks := [][]byte{record.data}
			for index+1 < len(records) && records[index+1].id == recContinue {
				index++
				chunks = append(chunks, records[index].data)
			}
			book.shared = parseSST(chunks)
		case recXF:
			if len(record.data) >= 4 {
				book.xfFormats = append(book.xfFormats, int(leU16(record.data[2:4])))
			} else {
				book.xfFormats = append(book.xfFormats, 0)
			}
		case recFormat:
			if code, id, ok := parseFormat(record.data); ok {
				book.formats[id] = code
			}
		case recDateMode:
			book.date1904 = leU16(record.data) != 0
		}
	}
	return book, nil
}

// readSheet doc mot sheet tu vi tri BOF cua no.
func (b *workbookGlobals) readSheet(stream []byte, info sheetInfo) (*Grid, error) {
	if info.position <= 0 || info.position >= len(stream) {
		return nil, fmt.Errorf("sheet %q points outside the workbook stream", info.name)
	}
	grid := NewGrid(info.name)
	records := scanRecords(stream, info.position)

	// O cong thuc tra ve chuoi: ban ghi STRING ngay sau FORMULA moi chua noi dung.
	pendingStringRow, pendingStringCol := -1, -1
	for _, record := range records {
		switch record.id {
		case recDimensions:
			if len(record.data) >= 12 {
				// rwMic, rwMac la chi so 0-based, rwMac = dong CUOI + 1.
				lastRow := int(leU32(record.data[4:8]))
				lastCol := int(leU16(record.data[10:12]))
				if lastRow > 0 {
					grid.MaxRow = lastRow
				}
				if lastCol > 0 {
					grid.MaxCol = lastCol
				}
			}
		case recLabelSST:
			if len(record.data) < 10 {
				continue
			}
			row, col := int(leU16(record.data)), int(leU16(record.data[2:4]))
			index := int(leU32(record.data[6:10]))
			if index >= 0 && index < len(b.shared) {
				grid.Set(row+1, col+1, b.shared[index])
			}
		case recNumber:
			if len(record.data) < 14 {
				continue
			}
			row, col := int(leU16(record.data)), int(leU16(record.data[2:4]))
			value := math.Float64frombits(leU64(record.data[6:14]))
			grid.Set(row+1, col+1, b.value(value, int(leU16(record.data[4:6]))))
		case recRK:
			if len(record.data) < 10 {
				continue
			}
			row, col := int(leU16(record.data)), int(leU16(record.data[2:4]))
			grid.Set(row+1, col+1, b.value(decodeRK(leU32(record.data[6:10])), int(leU16(record.data[4:6]))))
		case recMulRK:
			if len(record.data) < 6 {
				continue
			}
			row := int(leU16(record.data))
			first := int(leU16(record.data[2:4]))
			// Moi o 6 byte (ixfe + rk); hai byte cuoi la chi so cot cuoi.
			for offset, col := 4, first; offset+6 <= len(record.data)-2; offset, col = offset+6, col+1 {
				grid.Set(row+1, col+1, b.value(decodeRK(leU32(record.data[offset+2:offset+6])), int(leU16(record.data[offset:offset+2]))))
			}
		case recFormula:
			if len(record.data) < 16 {
				continue
			}
			row, col := int(leU16(record.data)), int(leU16(record.data[2:4]))
			format := int(leU16(record.data[4:6]))
			// 8 byte ket qua: byte 6..7 = 0xFFFF nghia la ket qua khong phai so.
			if leU16(record.data[12:14]) == 0xFFFF {
				switch record.data[14] {
				case 0: // chuoi: noi dung nam o ban ghi STRING ngay sau
					pendingStringRow, pendingStringCol = row, col
				case 1: // logic
					grid.Set(row+1, col+1, record.data[15] != 0)
				case 2: // loi
					// Loi khong co gia tri de tra ve; de trong nhu ban doc .xlsx van lam.
				case 3: // chuoi rong
					grid.Set(row+1, col+1, "")
				}
				continue
			}
			grid.Set(row+1, col+1, b.value(math.Float64frombits(leU64(record.data[6:14])), format))
		case recString:
			if pendingStringRow >= 0 {
				text := parseUnicodeString(record.data, 0)
				grid.Set(pendingStringRow+1, pendingStringCol+1, text)
				pendingStringRow, pendingStringCol = -1, -1
			}
		case recBoolErr:
			if len(record.data) < 8 {
				continue
			}
			row, col := int(leU16(record.data)), int(leU16(record.data[2:4]))
			if record.data[7] == 0 {
				grid.Set(row+1, col+1, record.data[6] != 0)
			}
		case recBlank, recMulBlank:
			// O trong co dinh dang: khong co gia tri de tra ve, nhung no van nam trong vung dung.
			continue
		}
	}
	// Dimensions chi la goi y; vung thuc te la hop cua cac o da doc duoc.
	if !grid.HasCells {
		grid.MaxRow, grid.MaxCol = 0, 0
	}
	return grid, nil
}

// value doi mot so cua BIFF thanh gia tri tra ve, dung DUNG quy uoc cua duong doc .xlsx: o co dinh dang
// ngay tra ve chuoi ISO, so nguyen tra ve int64, con lai float64.
func (b *workbookGlobals) value(number float64, formatIndex int) any {
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return nil
	}
	if b.isDateStyle(formatIndex) {
		return sheet.SerialToIso(number, b.date1904)
	}
	if number == math.Trunc(number) && math.Abs(number) < 1e15 {
		return int64(number)
	}
	return number
}

// isDateStyle: o dung dinh dang ngay khong? BIFF va OOXML dung chung danh so dinh dang dung san cho ngay
// thang, va dinh dang tu tao thi xet bang chuoi dinh dang.
func (b *workbookGlobals) isDateStyle(formatIndex int) bool {
	if formatIndex < 0 || formatIndex >= len(b.xfFormats) {
		return false
	}
	format := b.xfFormats[formatIndex]
	if code, ok := b.formats[format]; ok {
		return IsDateFormat(code)
	}
	return builtinDateFormat(format)
}

func builtinDateFormat(format int) bool {
	switch format {
	case 14, 15, 16, 17, 18, 19, 20, 21, 22, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 45, 46, 47,
		50, 51, 52, 53, 54, 55, 56, 57, 58:
		return true
	}
	return false
}

func parseBoundSheet(data []byte) (sheetInfo, error) {
	if len(data) < 8 {
		return sheetInfo{}, errors.New("truncated BOUNDSHEET record")
	}
	position := int(leU32(data[0:4]))
	// cch dem KY TU, khong phai byte: chuoi 16-bit chiem gap doi. Cat theo cch se lam ten cut nua
	// (ten "Trống" thanh "Tr") - va do la loi am tham, khong bao gi.
	length := int(data[6])
	if length <= 0 {
		return sheetInfo{}, errors.New("sheet name is empty in BOUNDSHEET record")
	}
	if data[7]&0x01 != 0 {
		length *= 2
	}
	if 8+length > len(data) {
		return sheetInfo{}, errors.New("truncated sheet name in BOUNDSHEET record")
	}
	name := decodeChars(data[8:8+length], data[7]&0x01 == 0)
	return sheetInfo{name: name, position: position}, nil
}

// parseFormat: FORMAT = ifmt:u16, cch:u8, grbitChr:u8, roi ky tu.
//
// Doc cch o byte 4 (thay vi byte 2) thi chuoi dinh dang ra sai - va hau qua khong phai "khong doc duoc
// ngay" ma la "ngay tra ve so": o ngay mat dinh dang, roi bi coi nhu so thuong. Do la loi am tham.
func parseFormat(data []byte) (string, int, bool) {
	if len(data) < 4 {
		return "", 0, false
	}
	id := int(leU16(data[0:2]))
	length := int(data[2])
	if data[3]&0x01 != 0 {
		length *= 2 // cch dem ky tu, chuoi 16-bit chiem gap doi
	}
	if 4+length > len(data) {
		return "", 0, false
	}
	return decodeChars(data[4:4+length], data[3]&0x01 == 0), id, true
}

// decodeChars doc chuoi BIFF: mot co cho biet 8-bit (Latin-1) hay UTF-16LE.
func decodeChars(data []byte, compressed bool) string {
	if !compressed {
		units := make([]uint16, 0, len(data)/2)
		for index := 0; index+2 <= len(data); index += 2 {
			units = append(units, leU16(data[index:index+2]))
		}
		return string(utf16.Decode(units))
	}
	runes := make([]rune, 0, len(data))
	for _, char := range data {
		runes = append(runes, rune(char))
	}
	return string(runes)
}

// decodeRK: so nén trong 4 byte. Bit 1 = so nguyen 30 bit; bit 0 = chia 100.
func decodeRK(raw uint32) float64 {
	var value float64
	if raw&0x02 != 0 {
		value = float64(int32(raw) >> 2)
	} else {
		value = math.Float64frombits(uint64(raw&0xFFFFFFFC) << 32)
	}
	if raw&0x01 != 0 {
		value /= 100
	}
	return value
}

// scanRecords doc cac ban ghi lien tiep tu offset, dung lai sau ban ghi EOF cua substream do.
func scanRecords(stream []byte, offset int) []biffRecord {
	records := []biffRecord{}
	for offset+4 <= len(stream) {
		id := leU16(stream[offset : offset+2])
		length := int(leU16(stream[offset+2 : offset+4]))
		start := offset + 4
		if start+length > len(stream) {
			break
		}
		records = append(records, biffRecord{id: id, data: stream[start : start+length]})
		offset = start + length
		if id == recEOF {
			break
		}
	}
	return records
}

// parseSST doc bang chuoi dung chung tu SST va cac CONTINUE ngay sau no.
func parseSST(chunks [][]byte) []string {
	reader := &sstReader{chunks: chunks}
	if _, ok := reader.readU32(); !ok { // cstTotal
		return nil
	}
	unique, ok := reader.readU32()
	if !ok {
		return nil
	}
	count := int(unique)
	if count < 0 || count > 1_000_000 {
		return nil
	}
	strings := make([]string, 0, count)
	for index := 0; index < count; index++ {
		text, ok := reader.readString()
		if !ok {
			break
		}
		strings = append(strings, text)
	}
	return strings
}

// sstReader doc xuyen qua nhieu khoi (SST + CONTINUE).
//
// Cho kho duy nhat cua BIFF: mot chuoi co the cat ngang ranh gioi record, va byte DAU cua record tiep
// theo lai la CO 8-bit/16-bit MOI cho phan con lai cua chuoi dang do - khong phai mo ta cua mot chuoi
// moi. Hieu sai cho nay thi chuoi ra rac, ma chi lo voi file du lon de chuoi bi cat.
type sstReader struct {
	chunks [][]byte
	chunk  int
	pos    int
}

func (r *sstReader) advance() bool {
	if r.chunk+1 >= len(r.chunks) {
		return false
	}
	r.chunk++
	r.pos = 0
	return true
}

func (r *sstReader) ended() bool {
	return r.chunk >= len(r.chunks)
}

func (r *sstReader) readByte() (byte, bool) {
	if r.ended() || r.pos >= len(r.chunks[r.chunk]) {
		return 0, false
	}
	value := r.chunks[r.chunk][r.pos]
	r.pos++
	return value, true
}

func (r *sstReader) readU16() (uint16, bool) {
	low, ok := r.readByte()
	if !ok {
		return 0, false
	}
	high, ok := r.readByte()
	if !ok {
		return 0, false
	}
	return uint16(low) | uint16(high)<<8, true
}

func (r *sstReader) readU32() (uint32, bool) {
	low, ok := r.readU16()
	if !ok {
		return 0, false
	}
	high, ok := r.readU16()
	if !ok {
		return 0, false
	}
	return uint32(low) | uint32(high)<<16, true
}

// readString doc mot muc trong SST: co chuoi, tuy chon khoi rich-text va khoi phien am, roi noi dung.
func (r *sstReader) readString() (string, bool) {
	length, ok := r.readU16()
	if !ok {
		return "", false
	}
	flags, ok := r.readByte()
	if !ok {
		return "", false
	}
	richRuns := 0
	if flags&0x08 != 0 {
		runs, ok := r.readU16()
		if !ok {
			return "", false
		}
		richRuns = int(runs)
	}
	extendedSize := 0
	if flags&0x04 != 0 {
		size, ok := r.readU32()
		if !ok {
			return "", false
		}
		extendedSize = int(size)
	}
	text, ok := r.readChars(int(length), flags&0x01 != 0)
	if !ok {
		return "", false
	}
	// Bo qua khoi rich-text (moi luot 4 byte) va khoi phien am.
	for index := 0; index < richRuns*4; index++ {
		if _, ok := r.readByte(); !ok {
			return "", false
		}
	}
	for index := 0; index < extendedSize; index++ {
		if _, ok := r.readByte(); !ok {
			return "", false
		}
	}
	return text, true
}

// readChars doc phan noi dung chuoi, co the vat qua ranh gioi record.
func (r *sstReader) readChars(count int, high bool) (string, bool) {
	units := make([]uint16, 0, count)
	for len(units) < count {
		if r.ended() {
			return "", false
		}
		if r.pos >= len(r.chunks[r.chunk]) {
			if !r.advance() {
				return "", false
			}
			// Co moi cho phan con lai cua chinh chuoi nay.
			flag, ok := r.readByte()
			if !ok {
				return "", false
			}
			high = flag&0x01 != 0
			continue
		}
		if high {
			unit, ok := r.readU16()
			if !ok {
				return "", false
			}
			units = append(units, unit)
			continue
		}
		char, ok := r.readByte()
		if !ok {
			return "", false
		}
		units = append(units, uint16(char))
	}
	// Co 8-bit nghia la tung byte la mot ky tu Latin-1; giai ma UTF-16 tren day se sai.
	if !high {
		runes := make([]rune, 0, len(units))
		for _, unit := range units {
			runes = append(runes, rune(unit))
		}
		return string(runes), true
	}
	return string(utf16.Decode(units)), true
}

// parseUnicodeString doc chuoi o dau mot ban ghi STRING (khac SST: co 2 byte do dai roi moi toi co).
func parseUnicodeString(data []byte, offset int) string {
	if offset+3 > len(data) {
		return ""
	}
	length := int(leU16(data[offset : offset+2]))
	flags := data[offset+2]
	start := offset + 3
	if flags&0x08 != 0 {
		return ""
	}
	if flags&0x01 != 0 {
		end := start + length*2
		if end > len(data) {
			end = len(data)
		}
		return decodeChars(data[start:end], false)
	}
	end := start + length
	if end > len(data) {
		end = len(data)
	}
	return decodeChars(data[start:end], true)
}

func leU16(data []byte) uint16 {
	if len(data) < 2 {
		return 0
	}
	return binary.LittleEndian.Uint16(data)
}

func leU32(data []byte) uint32 {
	if len(data) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}

func leU64(data []byte) uint64 {
	if len(data) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(data)
}
