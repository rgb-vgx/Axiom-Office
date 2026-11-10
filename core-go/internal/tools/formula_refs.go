package tools

import (
	"regexp"
	"strconv"
	"strings"
)

// Tach tham chieu o khoi cong thuc bang tinh (docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md X2). Lam o Core thay
// vi Range.DirectPrecedents cua Excel: DirectPrecedents chi theo duoc trong CUNG sheet va khong co tren WPS. Khong hieu ten
// vung (named range), INDIRECT/OFFSET va tham chieu sang so khac - nhung cho do duoc bao ro (Unresolved).

// FormulaRef: mot tham chieu trong cong thuc. Sheet rong = cung sheet voi o chua cong thuc. Ca cot/ca hang (A:A, 1:1) thi
// Whole = true va khong co toa do o.
type FormulaRef struct {
	Sheet            string
	FromRow, FromCol int
	ToRow, ToCol     int
	Whole            bool
	Text             string
}

// Cells: so o cua tham chieu (0 khi la ca cot/ca hang).
func (r FormulaRef) Cells() int {
	if r.Whole {
		return 0
	}
	return (r.ToRow - r.FromRow + 1) * (r.ToCol - r.FromCol + 1)
}

// Contains: o (row, col) cua sheet `sheet` nam trong tham chieu nay (sheetOfFormula: sheet chua cong thuc).
func (r FormulaRef) Contains(sheetOfFormula, sheet string, row, col int) bool {
	refSheet := r.Sheet
	if refSheet == "" {
		refSheet = sheetOfFormula
	}
	if !strings.EqualFold(refSheet, sheet) {
		return false
	}
	if r.Whole {
		if r.FromRow == 0 { // cot
			return col >= r.FromCol && col <= r.ToCol
		}
		return row >= r.FromRow && row <= r.ToRow
	}
	return row >= r.FromRow && row <= r.ToRow && col >= r.FromCol && col <= r.ToCol
}

// sheetPrefix: 'Ten co dau cach'! hoac Ten!.
const sheetPrefix = `(?:'((?:[^']|'')+)'|([\p{L}_][\p{L}\p{N}_.]*))!`

var (
	formulaRefPattern = regexp.MustCompile(`(?:` + sheetPrefix + `)?` +
		`(\$?[A-Za-z]{1,3}\$?[0-9]{1,7}(?::\$?[A-Za-z]{1,3}\$?[0-9]{1,7})?|\$?[A-Za-z]{1,3}:\$?[A-Za-z]{1,3}|\$?[0-9]{1,7}:\$?[0-9]{1,7})`)
	stringLiteral = regexp.MustCompile(`"(?:[^"]|"")*"`)
	unresolvedFns = regexp.MustCompile(`(?i)\b(INDIRECT|OFFSET)\s*\(`)
	externalBook  = regexp.MustCompile(`\[[^\]]+\]`)
)

// ParseFormulaRefs: cac tham chieu cua cong thuc `formula` (co hoac khong co dau '='). unresolved: ly do co phan khong
// lan duoc (INDIRECT/OFFSET, so khac) - rong khi lan du.
func ParseFormulaRefs(formula string) (refs []FormulaRef, unresolved []string) {
	text := strings.TrimPrefix(strings.TrimSpace(formula), "=")
	// Bo chuoi trong ngoac kep ("A1" la chu, khong phai tham chieu) - giu do dai de vi tri khong lech.
	text = stringLiteral.ReplaceAllStringFunc(text, func(s string) string { return strings.Repeat(" ", len(s)) })
	if unresolvedFns.MatchString(text) {
		unresolved = append(unresolved, "INDIRECT/OFFSET builds references at run time")
	}
	if externalBook.MatchString(text) {
		unresolved = append(unresolved, "reference to another workbook")
	}
	for _, match := range formulaRefPattern.FindAllStringSubmatchIndex(text, -1) {
		start, end := match[0], match[1]
		// Dung lien chu/so/_/. truoc hoac sau la mot ten khac (LOG10, Sheet1.A1 kieu LibreOffice, ten ham) - bo qua.
		if start > 0 && isNameChar(rune(text[start-1])) {
			continue
		}
		if end < len(text) && (isNameChar(rune(text[end])) || text[end] == '(') {
			continue
		}
		sheet := ""
		if match[2] >= 0 {
			sheet = strings.ReplaceAll(text[match[2]:match[3]], "''", "'")
		} else if match[4] >= 0 {
			sheet = text[match[4]:match[5]]
		}
		if ref, ok := parseRefBody(text[match[6]:match[7]]); ok {
			ref.Sheet = sheet
			ref.Text = text[start:end]
			refs = append(refs, ref)
		}
	}
	return refs, unresolved
}

func isNameChar(ch rune) bool {
	return ch == '_' || ch == '.' || ch == '!' || (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

// parseRefBody: "A1", "$A$1:B5", "A:C", "3:7".
func parseRefBody(body string) (FormulaRef, bool) {
	body = strings.ReplaceAll(body, "$", "")
	first, second, ranged := strings.Cut(body, ":")
	if !ranged {
		second = first
	}
	if row1, err1 := strconv.Atoi(first); err1 == nil {
		row2, err2 := strconv.Atoi(second)
		if err2 != nil {
			return FormulaRef{}, false
		}
		return FormulaRef{FromRow: min(row1, row2), ToRow: max(row1, row2), FromCol: 1, ToCol: 16384, Whole: true}, true
	}
	if isLetters(first) && isLetters(second) {
		col1, col2 := columnNumber(first), columnNumber(second)
		return FormulaRef{FromCol: min(col1, col2), ToCol: max(col1, col2), Whole: true}, true
	}
	row1, col1, ok1 := splitCell(first)
	row2, col2, ok2 := splitCell(second)
	if !ok1 || !ok2 {
		return FormulaRef{}, false
	}
	return FormulaRef{FromRow: min(row1, row2), FromCol: min(col1, col2), ToRow: max(row1, row2), ToCol: max(col1, col2)}, true
}

func isLetters(text string) bool {
	if text == "" {
		return false
	}
	for _, ch := range text {
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
			return false
		}
	}
	return true
}

// splitCell: "C42" -> (42, 3).
func splitCell(text string) (int, int, bool) {
	index := 0
	for index < len(text) && ((text[index] >= 'A' && text[index] <= 'Z') || (text[index] >= 'a' && text[index] <= 'z')) {
		index++
	}
	if index == 0 || index == len(text) {
		return 0, 0, false
	}
	row, err := strconv.Atoi(text[index:])
	if err != nil || row <= 0 {
		return 0, 0, false
	}
	return row, columnNumber(text[:index]), true
}

// a1Name: (42, 3) -> "C42".
func a1Name(row, col int) string { return columnLetters(col) + strconv.Itoa(row) }

// qualifiedSheet: ten sheet viet trong tham chieu - co nhay khi co ky tu ngoai chu/so/_.
func qualifiedSheet(sheet string) string {
	for _, ch := range sheet {
		if !(ch == '_' || (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) {
			return "'" + strings.ReplaceAll(sheet, "'", "''") + "'"
		}
	}
	return sheet
}
