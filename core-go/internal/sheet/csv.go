package sheet

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"axiomoffice/core/internal/filesafe"
)

// bom la dau BOM UTF-8: file CSV ghi ra co BOM (giong ban Python), file doc vao thi bo qua.
var bom = []byte{0xEF, 0xBB, 0xBF}

// IsCsvExtension kiem tra duoi file CSV/TSV/TXT; tham so truyen kem dau cham (vd ".csv").
func IsCsvExtension(extension string) bool {
	return extension == ".csv" || extension == ".tsv" || extension == ".txt"
}

// ReadCSV doc file CSV/TSV. Dấu phân cách cua .tsv la tab, con lai thi doan tu noi dung.
// Gia tri duoc ep kieu nhu ban Python: so nguyen, so thuc, con lai giu nguyen chuoi.
func ReadCSV(path string) ([][]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(stripBOM(data))
	delimiter := ','
	if strings.ToLower(filepath.Ext(path)) == ".tsv" {
		delimiter = '\t'
	} else {
		delimiter = sniffDelimiter(text)
	}
	rows := [][]any{}
	for _, raw := range parseCSV(text, delimiter) {
		row := make([]any, 0, len(raw))
		for _, field := range raw {
			row = append(row, coerce(field))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// WriteCSV ghi bang ra CSV: UTF-8 co BOM, xuong dong CRLF, luon dung dau phay.
func WriteCSV(path string, rows [][]any) error {
	var builder strings.Builder
	builder.Write(bom)
	for _, row := range rows {
		fields := make([]string, 0, len(row))
		for _, value := range row {
			fields = append(fields, quoteCSV(value))
		}
		builder.WriteString(strings.Join(fields, ","))
		builder.WriteString("\r\n")
	}
	return filesafe.WriteAtomic(path, []byte(builder.String()))
}

func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == bom[0] && data[1] == bom[1] && data[2] == bom[2] {
		return data[3:]
	}
	return data
}

// sniffDelimiter chon dau phan cach xuat hien deu dan nhat trong cac dong dau (",;\t|").
func sniffDelimiter(text string) rune {
	candidates := []rune{',', ';', '\t', '|'}
	lines := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if len(lines) >= 20 {
			break
		}
	}
	best := ','
	bestScore := 0
	for _, candidate := range candidates {
		counts := make([]int, 0, len(lines))
		maximum := 0
		for _, line := range lines {
			count := countOutsideQuotes(line, candidate)
			counts = append(counts, count)
			if count > maximum {
				maximum = count
			}
		}
		if len(counts) == 0 || maximum == 0 {
			continue
		}
		mode := modeOf(counts)
		consistent := 0
		for _, count := range counts {
			if count == mode && count > 0 {
				consistent++
			}
		}
		if score := consistent*1000 + mode; score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best
}

// modeOf tra ve gia tri xuat hien nhieu nhat; bang nhau thi lay gia tri xuat hien truoc.
func modeOf(counts []int) int {
	order := []int{}
	frequencies := map[int]int{}
	for _, count := range counts {
		if _, seen := frequencies[count]; !seen {
			order = append(order, count)
		}
		frequencies[count]++
	}
	mode, bestCount := 0, -1
	for _, value := range order {
		if frequencies[value] > bestCount {
			mode, bestCount = value, frequencies[value]
		}
	}
	return mode
}

func countOutsideQuotes(line string, delimiter rune) int {
	count := 0
	quoted := false
	for _, character := range line {
		switch {
		case character == '"':
			quoted = !quoted
		case character == delimiter && !quoted:
			count++
		}
	}
	return count
}

// parseCSV la may trang thai doc CSV: "" long nhau, "" doi thanh ", CRLF va LF deu ket thuc dong.
func parseCSV(text string, delimiter rune) [][]string {
	rows := [][]string{}
	row := []string{}
	var field strings.Builder
	quoted := false
	any := false
	characters := []rune(text)
	for index := 0; index < len(characters); index++ {
		character := characters[index]
		any = true
		if quoted {
			if character == '"' {
				if index+1 < len(characters) && characters[index+1] == '"' {
					field.WriteRune('"')
					index++
				} else {
					quoted = false
				}
			} else {
				field.WriteRune(character)
			}
			continue
		}
		switch {
		case character == '"' && field.Len() == 0:
			quoted = true
		case character == delimiter:
			row = append(row, field.String())
			field.Reset()
		case character == '\r' || character == '\n':
			if character == '\r' && index+1 < len(characters) && characters[index+1] == '\n' {
				index++
			}
			row = append(row, field.String())
			field.Reset()
			rows = append(rows, row)
			row = []string{}
			any = false
		default:
			field.WriteRune(character)
		}
	}
	if any || field.Len() > 0 || len(row) > 0 {
		row = append(row, field.String())
		rows = append(rows, row)
	}
	return rows
}

// coerce ep chuoi thanh so neu bieu dien khop hoan toan, khong thi giu nguyen chuoi goc.
func coerce(text string) any {
	stripped := strings.TrimSpace(text)
	if stripped == "" {
		return ""
	}
	if value, err := strconv.ParseInt(stripped, 10, 64); err == nil && strconv.FormatInt(value, 10) == stripped {
		return value
	}
	if value, err := strconv.ParseFloat(stripped, 64); err == nil {
		representation := roundTripFloat64(value)
		if !strings.ContainsAny(representation, ".Ee") {
			representation += ".0"
		}
		if representation == stripped {
			return value
		}
	}
	return text
}

func quoteCSV(value any) string {
	text := ""
	if value != nil {
		text = ToText(value)
	}
	if strings.ContainsAny(text, ",\"\r\n") {
		return `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
	}
	return text
}
