// Package sheet: tien ich bang tinh dung chung cho cac lan file - dia chi A1, vung chon, doi
// serial Excel sang ISO, va doc/ghi CSV/TSV. Port cua Cells.cs + CsvTable.cs.
package sheet

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MaxColumns = 16384
	MaxRows    = 1048576
)

var (
	cellRefPattern = regexp.MustCompile(`^\$?([A-Za-z]{1,3})\$?([0-9]+)$`)
	colRefPattern  = regexp.MustCompile(`^\$?([A-Za-z]{1,3})$`)
	rowRefPattern  = regexp.MustCompile(`^\$?([0-9]+)$`)
)

// ToText doi gia tri thanh chuoi theo kieu str() cua Python: True/False, so nguyen gon,
// so thuc dang round-trip.
func ToText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case bool:
		if typed {
			return "True"
		}
		return "False"
	case float64:
		return roundTripFloat64(typed)
	case float32:
		return roundTripFloat64(float64(typed))
	case json.Number:
		return typed.String()
	case string:
		return typed
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return fmt.Sprint(typed)
	}
}

// IsNumber cho biet gia tri la so (ke ca json.Number vi ban Windows doc bang JavaScriptSerializer,
// ban Go doc bang json.Number).
func IsNumber(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		return true
	}
	return false
}

// AsFloat doi gia tri so sang float64.
func AsFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		converted, err := typed.Float64()
		return converted, err == nil
	}
	return 0, false
}

// roundTripFloat64 gan dung dinh dang "R" cua .NET: chuoi ngan nhat ma doc lai van dung,
// va dang mu dung chu E hoa voi so mu it nhat hai chu so.
func roundTripFloat64(value float64) string {
	text := strconv.FormatFloat(value, 'g', -1, 64)
	index := strings.IndexAny(text, "eE")
	if index < 0 {
		return text
	}
	mantissa, exponent := text[:index], text[index+1:]
	sign := "+"
	if strings.HasPrefix(exponent, "-") {
		sign = "-"
		exponent = exponent[1:]
	} else {
		exponent = strings.TrimPrefix(exponent, "+")
	}
	for len(exponent) < 2 {
		exponent = "0" + exponent
	}
	return mantissa + "E" + sign + exponent
}

// ColumnIndex doi ten cot ("A", "AB") thanh chi so 1-based.
func ColumnIndex(letters string) int {
	result := 0
	for _, character := range strings.ToUpper(letters) {
		result = result*26 + int(character-'A') + 1
	}
	return result
}

// ColumnLetters doi chi so cot 1-based thanh ten cot.
func ColumnLetters(index int) string {
	letters := []byte{}
	for index > 0 {
		remainder := (index - 1) % 26
		letters = append([]byte{byte('A' + remainder)}, letters...)
		index = (index - 1) / 26
	}
	return string(letters)
}

// Address dung dia chi A1 tu toa do 1-based.
func Address(row, col int) string {
	return ColumnLetters(col) + strconv.Itoa(row)
}

// ParseCell doc dia chi mot o, tra ve toa do 1-based.
func ParseCell(a1 string) (row, col int, err error) {
	match := cellRefPattern.FindStringSubmatch(strings.TrimSpace(a1))
	if match == nil {
		return 0, 0, fmt.Errorf("invalid cell reference '%s' (expected like 'B2')", a1)
	}
	col = ColumnIndex(match[1])
	row, err = strconv.Atoi(match[2])
	if err != nil || row < 1 || col < 1 || col > MaxColumns || row > MaxRows {
		return 0, 0, fmt.Errorf("cell reference out of range: %s", a1)
	}
	return row, col, nil
}

// Range la vung o; canh nao nil nghia la khong gioi han theo kieu cua openpyxl
// ("A1:C5" du ca bon, "A:C" khong gioi han dong, "2:5" khong gioi han cot).
type Range struct {
	MinCol *int
	MinRow *int
	MaxCol *int
	MaxRow *int
}

// Bounds la vung co du bon canh.
type Bounds struct {
	MinCol int
	MinRow int
	MaxCol int
	MaxRow int
}

// ParseRange doc vung chon. Phan sau "!" (ten sheet) bi bo qua.
func ParseRange(text string) (Range, error) {
	trimmed := strings.TrimSpace(text)
	if bang := strings.LastIndex(trimmed, "!"); bang >= 0 {
		trimmed = trimmed[bang+1:]
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return Range{}, fmt.Errorf("invalid range '%s'", text)
	}
	col1, row1, err := parseEdge(parts[0], text)
	if err != nil {
		return Range{}, err
	}
	col2, row2 := col1, row1
	if len(parts) == 2 {
		col2, row2, err = parseEdge(parts[1], text)
		if err != nil {
			return Range{}, err
		}
	}
	return Range{
		MinCol: minBound(col1, col2),
		MaxCol: maxBound(col1, col2),
		MinRow: minBound(row1, row2),
		MaxRow: maxBound(row1, row2),
	}, nil
}

// ParseBoundedRange doi vung phai co du bon canh tuong minh.
func ParseBoundedRange(text string) (Bounds, error) {
	found, err := ParseRange(text)
	if err != nil {
		return Bounds{}, err
	}
	if found.MinCol == nil || found.MinRow == nil || found.MaxCol == nil || found.MaxRow == nil {
		return Bounds{}, fmt.Errorf("range must have explicit cells like 'A1:D10': %s", text)
	}
	return Bounds{MinCol: *found.MinCol, MinRow: *found.MinRow, MaxCol: *found.MaxCol, MaxRow: *found.MaxRow}, nil
}

func parseEdge(edge, original string) (col, row *int, err error) {
	if match := cellRefPattern.FindStringSubmatch(edge); match != nil {
		rowNumber, convErr := strconv.Atoi(match[2])
		if convErr != nil {
			return nil, nil, fmt.Errorf("invalid range '%s'", original)
		}
		column := ColumnIndex(match[1])
		return &column, &rowNumber, nil
	}
	if match := colRefPattern.FindStringSubmatch(edge); match != nil {
		column := ColumnIndex(match[1])
		return &column, nil, nil
	}
	if match := rowRefPattern.FindStringSubmatch(edge); match != nil {
		rowNumber, convErr := strconv.Atoi(match[1])
		if convErr != nil {
			return nil, nil, fmt.Errorf("invalid range '%s'", original)
		}
		return nil, &rowNumber, nil
	}
	return nil, nil, fmt.Errorf("invalid range '%s'", original)
}

func minBound(a, b *int) *int {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case *a < *b:
		return a
	default:
		return b
	}
}

func maxBound(a, b *int) *int {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case *a > *b:
		return a
	default:
		return b
	}
}

// SerialToIso doi so serial cua Excel thanh chuoi ISO giong datetime.isoformat() cua Python.
func SerialToIso(serial float64, date1904 bool) string {
	// Serial nho hon 1 (khong phai he 1904) la gio trong ngay.
	if serial < 1 && serial >= 0 && !date1904 {
		seconds := math.Round(serial * 86400)
		value := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC).Add(secondsDuration(seconds))
		return value.Format("15:04:05")
	}
	adjusted := serial
	switch {
	case date1904:
		adjusted = serial + 1462
	case serial < 60:
		// Excel coi 1900 la nam nhuan (loi lich su), nen serial < 60 phai cong 1.
		adjusted = serial + 1
	}
	days := math.Floor(adjusted)
	seconds := math.Round((adjusted - days) * 86400)
	if seconds >= 86400 {
		seconds -= 86400
		days++
	}
	// Moc 0 cua OLE Automation la 1899-12-30; tach ngay va giay de tranh tran int64.
	value := oaEpoch.AddDate(0, 0, int(days)).Add(secondsDuration(seconds))
	return value.Format("2006-01-02T15:04:05")
}

var oaEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

func secondsDuration(seconds float64) time.Duration {
	return time.Duration(seconds) * time.Second
}
