package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Guard chong ghi de o co du lieu (log 11/10/2026: agent xay dashboard da viet de 25 o du lieu
// goc tren sheet ADT - so/ngay luu dang chu doi thanh so - ma khong hoi nguoi dung).
// Lenh ghi o (et.writeRange, et.writeRanges, et.fillRange) vao sheet co tu truoc luot chay ma
// dung vao o KHONG RONG va gia tri moi KHAC gia tri cu -> hoi nguoi dung truoc khi ghi.

// overwriteCell: mot o bi ghi de (gia tri cu khac gia tri moi).
type overwriteCell struct {
	sheet string
	cell  string
	old   any
	new   any
}

// overwriteStringParam: doc truong chuoi trong params (trim space, khong phan biet kieu).
func overwriteStringParam(params map[string]any, name string) string {
	if params == nil {
		return ""
	}
	value, _ := params[name].(string)
	return strings.TrimSpace(value)
}

// overwriteColName: cot 0-based -> chu cai ("A", "Z", "AA"...).
func overwriteColName(col int) string {
	letters := ""
	for c := col + 1; c > 0; c = (c - 1) / 26 {
		letters = string(rune('A'+(c-1)%26)) + letters
	}
	return letters
}

// overwriteCellAddr: dia chi o tu cot/dong 0-based.
func overwriteCellAddr(col, row int) string {
	return overwriteColName(col) + strconv.Itoa(row+1)
}

// parseOverwriteCell: "Q4" -> (col, row) 0-based. Bo tien to sheet ("Sheet1!A1", "'Ten'!A1") va "$".
func parseOverwriteCell(address string) (int, int, bool) {
	text := strings.TrimSpace(address)
	if index := strings.LastIndex(text, "!"); index >= 0 {
		text = text[index+1:]
	}
	text = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(text)), "$", "")
	col, index := 0, 0
	for index < len(text) && text[index] >= 'A' && text[index] <= 'Z' {
		col = col*26 + int(text[index]-'A') + 1
		index++
	}
	if col == 0 || index >= len(text) {
		return 0, 0, false
	}
	row, err := strconv.Atoi(text[index:])
	if err != nil || row < 1 {
		return 0, 0, false
	}
	return col - 1, row - 1, true
}

// parseOverwriteRange: "A1" hoac "A1:C3" -> goc trai-tren va goc phai-duoi (da chuan hoa min/max).
func parseOverwriteRange(address string) (int, int, int, int, bool) {
	text := strings.TrimSpace(address)
	if text == "" {
		return 0, 0, 0, 0, false
	}
	parts := strings.Split(text, ":")
	c1, r1, ok := parseOverwriteCell(parts[0])
	if !ok {
		return 0, 0, 0, 0, false
	}
	if len(parts) == 1 {
		return c1, r1, c1, r1, true
	}
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}
	c2, r2, ok := parseOverwriteCell(parts[1])
	if !ok {
		return 0, 0, 0, 0, false
	}
	return min(c1, c2), min(r1, r2), max(c1, c2), max(r1, r2), true
}

// asOverwriteMatrix: params/bridge tra ma tran dang []any cua cac dong []any.
func asOverwriteMatrix(value any) [][]any {
	rows, ok := value.([]any)
	if !ok {
		return nil
	}
	matrix := make([][]any, 0, len(rows))
	for _, line := range rows {
		if cells, ok := line.([]any); ok {
			matrix = append(matrix, cells)
			continue
		}
		matrix = append(matrix, []any{line})
	}
	return matrix
}

// readOverwriteCell: doc mot o qua bridge (et.readRange {range, sheet}). Doc loi -> khong
// chan (ok=false). Co "sheet" thi gui kem, khong thi doc sheet dang active (khong gui sheet).
// Doc theo tung o de lenh dau tien giu nguyen range goc agent gui (e2e test_core_e2e doi
// bridge.commands[0].params.range == "A1" cho writeRange {range "A1"}).
func readOverwriteCell(ctx context.Context, run *RunContext, sheet, cell string) (any, string, bool) {
	// Huy giua chung thi dung doc tiep (tranh lenh read roi xuong bridge sau diem dung).
	if ctx != nil && ctx.Err() != nil {
		return nil, sheet, false
	}
	readParams := map[string]any{"range": cell}
	if sheet != "" {
		readParams["sheet"] = sheet
	}
	result := run.Bridge.Command(ctx, run.Office.Port, "et.readRange", readParams)
	if !result.OK || result.Result == nil {
		return nil, sheet, false
	}
	resolved := sheet
	if name, _ := result.Result["sheet"].(string); strings.TrimSpace(name) != "" {
		resolved = strings.TrimSpace(name)
	}
	var old any
	for _, line := range asOverwriteMatrix(result.Result["values"]) {
		if len(line) > 0 {
			old = line[0]
			break
		}
	}
	return old, resolved, true
}

// isEmptyOverwriteCell: o trong = null hoac chuoi rong.
func isEmptyOverwriteCell(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok && text == "" {
		return true
	}
	return false
}

// isOverwriteFormula: chuoi cong thuc ("=...") - luon coi la khac gia tri cu.
func isOverwriteFormula(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(text), "=")
}

// overwriteToFloat: doi gia tri so (ca so dang chuoi "123", "12.5") ve float64 de so sanh so hoc.
func overwriteToFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	case json.Number:
		if parsed, err := number.Float64(); err == nil {
			return parsed, true
		}
		return 0, false
	case string:
		trimmed := strings.TrimSpace(number)
		if trimmed == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parsed, true
		}
		return 0, false
	}
	return 0, false
}

// formatOverwriteValue: chuoi hien thi ngan gon cho ly do hoi (giu nguyen so dang chu).
func formatOverwriteValue(value any) string {
	switch data := value.(type) {
	case nil:
		return ""
	case string:
		return data
	case float64:
		return strconv.FormatFloat(data, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(data), 'f', -1, 32)
	case json.Number:
		return data.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

// overwriteValuesDiffer: gia tri moi co khac gia tri cu khong (o cu da biet khong rong).
func overwriteValuesDiffer(old, new any) bool {
	// Chuoi cong thuc luon coi la khac (khong so sanh cong thuc voi gia tri hien).
	if isOverwriteFormula(old) || isOverwriteFormula(new) {
		return true
	}
	oldNum, oldIsNum := overwriteToFloat(old)
	newNum, newIsNum := overwriteToFloat(new)
	if oldIsNum && newIsNum {
		return oldNum != newNum
	}
	return formatOverwriteValue(old) != formatOverwriteValue(new)
}

// ensureOverwriteMaps: khoi tao map luu trang thai luot chay (tranh nil map).
func ensureOverwriteMaps(run *RunContext) {
	if run.CreatedSheets == nil {
		run.CreatedSheets = map[string]bool{}
	}
	if run.OverwriteApproved == nil {
		run.OverwriteApproved = map[string]bool{}
	}
}

// rememberCreatedSheet: luu ten sheet vua tao thanh cong bang et.addSheet trong luot nay.
func rememberCreatedSheet(run *RunContext, params map[string]any) {
	name := overwriteStringParam(params, "name")
	if name == "" {
		name = overwriteStringParam(params, "sheet")
	}
	if name == "" {
		return
	}
	ensureOverwriteMaps(run)
	run.CreatedSheets[name] = true
}

// rememberOverwriteApproval: danh dau cac sheet da duoc dong y ghi de (chi hoi mot lan mot sheet).
func rememberOverwriteApproval(run *RunContext, sheets []string) {
	if len(sheets) == 0 {
		return
	}
	ensureOverwriteMaps(run)
	for _, sheet := range sheets {
		if sheet != "" {
			run.OverwriteApproved[sheet] = true
		}
	}
}

// effectiveOverwriteSheet: ten sheet de doi chieu tao moi/dong y (param sheet, khong thi sheet active).
func effectiveOverwriteSheet(paramSheet, resolved string) string {
	if paramSheet != "" {
		return paramSheet
	}
	return resolved
}

// filterOverwriteCells: bo o thuoc sheet do luot tao hoac da duoc dong y trong luot nay.
func filterOverwriteCells(run *RunContext, cells []overwriteCell) []overwriteCell {
	kept := cells[:0]
	for _, cell := range cells {
		if cell.sheet != "" {
			if run.CreatedSheets[cell.sheet] || run.OverwriteApproved[cell.sheet] {
				continue
			}
		}
		kept = append(kept, cell)
	}
	return kept
}

// collectWriteRangeOverwrites: writeRange = o goc trai-tren cua "range" mo rong theo ma tran "values".
func collectWriteRangeOverwrites(ctx context.Context, run *RunContext, params map[string]any, topSheet string) []overwriteCell {
	rangeText := overwriteStringParam(params, "range")
	matrix := asOverwriteMatrix(params["values"])
	if rangeText == "" || len(matrix) == 0 {
		return nil
	}
	parts := strings.Split(rangeText, ":")
	originCol, originRow, ok := parseOverwriteCell(parts[0])
	if !ok {
		return nil
	}
	sheet := overwriteStringParam(params, "sheet")
	if sheet == "" {
		sheet = topSheet
	}
	// Sheet da xu ly xong trong luot nay (do luot tao hoac da dong y) thi khong can doc.
	if sheet != "" && (run.CreatedSheets[sheet] || run.OverwriteApproved[sheet]) {
		return nil
	}
	var cells []overwriteCell
	for r, line := range matrix {
		for c, newValue := range line {
			cell := overwriteCellAddr(originCol+c, originRow+r)
			oldValue, resolved, ok := readOverwriteCell(ctx, run, sheet, cell)
			if !ok {
				continue
			}
			if isEmptyOverwriteCell(oldValue) || !overwriteValuesDiffer(oldValue, newValue) {
				continue
			}
			cells = append(cells, overwriteCell{sheet: effectiveOverwriteSheet(sheet, resolved),
				cell: cell, old: oldValue, new: newValue})
		}
	}
	return cells
}

// collectFillRangeOverwrites: fillRange = chep cong thuc cho CA vung "range" (moi o nhan formula).
func collectFillRangeOverwrites(ctx context.Context, run *RunContext, params map[string]any) []overwriteCell {
	rangeText := overwriteStringParam(params, "range")
	if rangeText == "" {
		return nil
	}
	formula, present := params["formula"]
	if !present || formula == nil {
		return nil
	}
	c1, r1, c2, r2, ok := parseOverwriteRange(rangeText)
	if !ok {
		return nil
	}
	sheet := overwriteStringParam(params, "sheet")
	// Sheet da xu ly xong trong luot nay (do luot tao hoac da dong y) thi khong can doc.
	if sheet != "" && (run.CreatedSheets[sheet] || run.OverwriteApproved[sheet]) {
		return nil
	}
	var cells []overwriteCell
	for row := r1; row <= r2; row++ {
		for col := c1; col <= c2; col++ {
			cell := overwriteCellAddr(col, row)
			oldValue, resolved, ok := readOverwriteCell(ctx, run, sheet, cell)
			if !ok {
				continue
			}
			if isEmptyOverwriteCell(oldValue) || !overwriteValuesDiffer(oldValue, formula) {
				continue
			}
			cells = append(cells, overwriteCell{sheet: effectiveOverwriteSheet(sheet, resolved),
				cell: cell, old: oldValue, new: formula})
		}
	}
	return cells
}

// buildOverwriteReason: ly do hoi tieng Viet - ten sheet, tong so o, toi da ~10 vi du, duoi 1200 ky tu.
func buildOverwriteReason(cells []overwriteCell) string {
	if len(cells) == 0 {
		return ""
	}
	bySheet := map[string]int{}
	order := []string{}
	for _, cell := range cells {
		if _, seen := bySheet[cell.sheet]; !seen {
			order = append(order, cell.sheet)
		}
		bySheet[cell.sheet]++
	}
	examples := []string{}
	for _, cell := range cells {
		if len(examples) >= 10 {
			break
		}
		name := cell.cell
		if len(order) > 1 && cell.sheet != "" {
			name = cell.sheet + "!" + cell.cell
		}
		examples = append(examples, name+" '"+formatOverwriteValue(cell.old)+"' → "+formatOverwriteValue(cell.new))
	}
	shown := strings.Join(examples, ", ")
	if len(cells) > len(examples) {
		shown += ", ..."
	}
	var reason string
	if len(order) == 1 {
		reason = "Ghi đè " + strconv.Itoa(len(cells)) + " ô đang có dữ liệu trên sheet " + order[0] + ": " + shown
	} else {
		parts := []string{}
		for _, sheet := range order {
			parts = append(parts, sheet+" ("+strconv.Itoa(bySheet[sheet])+" ô)")
		}
		reason = "Ghi đè " + strconv.Itoa(len(cells)) + " ô đang có dữ liệu trên các sheet " +
			strings.Join(parts, ", ") + ": " + shown
	}
	if len([]rune(reason)) > 1200 {
		runes := []rune(reason)
		reason = string(runes[:1200])
	}
	return reason
}

// checkOverwriteGuard: truoc policy/bridge - tra ly do hoi + cac sheet lien quan (rong = khong can hoi).
func checkOverwriteGuard(ctx context.Context, run *RunContext, action string, params map[string]any) (string, []string) {
	if run == nil || run.Bridge == nil || run.Office == nil {
		return "", nil
	}
	// Luot da dung/huy thi khong doc them (tranh lenh read roi xuong bridge sau diem dung).
	if ctx != nil && ctx.Err() != nil {
		return "", nil
	}
	ensureOverwriteMaps(run)
	var cells []overwriteCell
	switch action {
	case "et.writeRange":
		cells = collectWriteRangeOverwrites(ctx, run, params, "")
	case "et.writeRanges":
		topSheet := overwriteStringParam(params, "sheet")
		writes, _ := params["writes"].([]any)
		for _, item := range writes {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			cells = append(cells, collectWriteRangeOverwrites(ctx, run, entry, topSheet)...)
		}
	case "et.fillRange":
		cells = collectFillRangeOverwrites(ctx, run, params)
	default:
		return "", nil
	}
	cells = filterOverwriteCells(run, cells)
	if len(cells) == 0 {
		return "", nil
	}
	seen := map[string]bool{}
	var sheets []string
	for _, cell := range cells {
		if cell.sheet != "" && !seen[cell.sheet] {
			seen[cell.sheet] = true
			sheets = append(sheets, cell.sheet)
		}
	}
	return buildOverwriteReason(cells), sheets
}
