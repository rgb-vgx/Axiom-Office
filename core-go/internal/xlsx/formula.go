package xlsx

import (
	"regexp"
	"strconv"
	"strings"

	"axiomoffice/core/internal/sheet"
)

// formulaReference la phan loi cua regex trong FormulaShift.cs. Ban C# boc them lookbehind
// (?<![A-Za-z0-9_.]) va lookahead (?![0-9A-Za-z_(]); Go dung RE2 khong ho tro hai thu do, nen
// o day tim ung vien roi tu kiem tra ky tu ke ben - ket qua tuong duong.
var formulaReference = regexp.MustCompile(`(\$?)([A-Za-z]{1,3})(\$?)([0-9]{1,7})`)

// ShiftFormula doi tham chieu tuong doi cua shared formula (o con cua cong thuc dung chung).
// Phan trong dau ngoac kep (chuoi) duoc giu nguyen.
func ShiftFormula(formula string, dRow, dCol int) string {
	var builder strings.Builder
	start := 0
	inString := false
	for index := 0; index <= len(formula); index++ {
		end := index == len(formula)
		switch {
		case !end && formula[index] == '"':
			if !inString {
				builder.WriteString(shiftSegment(formula[start:index], dRow, dCol))
				start = index
			} else {
				builder.WriteString(formula[start : index+1])
				start = index + 1
			}
			inString = !inString
		case end:
			tail := formula[start:]
			if inString {
				builder.WriteString(tail)
			} else {
				builder.WriteString(shiftSegment(tail, dRow, dCol))
			}
		}
	}
	return builder.String()
}

func shiftSegment(segment string, dRow, dCol int) string {
	matches := formulaReference.FindAllStringSubmatchIndex(segment, -1)
	if matches == nil {
		return segment
	}
	var builder strings.Builder
	last := 0
	for _, match := range matches {
		begin, end := match[0], match[1]
		// Lookbehind: khong dung ngay sau chu, so, gach duoi hay dau cham.
		if begin > 0 && isReferenceHead(segment[begin-1]) {
			continue
		}
		// Lookahead: khong dung ngay truoc chu, so, gach duoi hay mo ngoac don.
		if end < len(segment) && isReferenceTail(segment[end]) {
			continue
		}
		absoluteCol := segment[match[2]:match[3]]
		letters := segment[match[4]:match[5]]
		absoluteRow := segment[match[6]:match[7]]
		digits := segment[match[8]:match[9]]

		col := sheet.ColumnIndex(letters)
		row, _ := strconv.Atoi(digits)
		if absoluteCol == "" {
			col += dCol
		}
		if absoluteRow == "" {
			row += dRow
		}
		builder.WriteString(segment[last:begin])
		if col < 1 || row < 1 {
			builder.WriteString("#REF!")
		} else {
			builder.WriteString(absoluteCol)
			builder.WriteString(sheet.ColumnLetters(col))
			builder.WriteString(absoluteRow)
			builder.WriteString(strconv.Itoa(row))
		}
		last = end
	}
	builder.WriteString(segment[last:])
	return builder.String()
}

func isReferenceHead(character byte) bool {
	return isLetter(character) || isDigit(character) || character == '_' || character == '.'
}

func isReferenceTail(character byte) bool {
	return isLetter(character) || isDigit(character) || character == '_' || character == '('
}

func isLetter(character byte) bool {
	return (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z')
}

func isDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

// QuoteSheet boc ten sheet trong nhay don khi ten khong phai dang don gian.
func QuoteSheet(name string) string {
	if simpleSheetName.MatchString(name) {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

var simpleSheetName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// ReplaceSheetReference doi ten sheet trong cong thuc cua definedNames.
func ReplaceSheetReference(formula, oldName, newName string) string {
	quotedOld := "'" + strings.ReplaceAll(oldName, "'", "''") + "'!"
	replacement := QuoteSheet(newName) + "!"
	result := strings.ReplaceAll(formula, quotedOld, replacement)
	return replaceUnquotedSheetReference(result, oldName, replacement)
}

// replaceUnquotedSheetReference thay "Ten!" khi ky tu ngay truoc khong phai chu/so/gach duoi/
// cham/nhay don - tuong duong lookbehind (?<![A-Za-z0-9_.']) cua ban C#.
func replaceUnquotedSheetReference(formula, oldName, replacement string) string {
	needle := oldName + "!"
	var builder strings.Builder
	last := 0
	for last < len(formula) {
		offset := strings.Index(formula[last:], needle)
		if offset < 0 {
			break
		}
		begin := last + offset
		end := begin + len(needle)
		if begin > 0 && strings.IndexByte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_.'", formula[begin-1]) >= 0 {
			builder.WriteString(formula[last:end])
		} else {
			builder.WriteString(formula[last:begin])
			builder.WriteString(replacement)
		}
		last = end
	}
	builder.WriteString(formula[last:])
	return builder.String()
}
