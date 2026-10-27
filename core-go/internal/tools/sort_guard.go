package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Hoi truoc khi sap xep (et.sort): sap xep doi vi tri du lieu cua nguoi dung va Excel khong hoan tac duoc thay doi qua COM
// (F10). Hai truong hop hoi:
//   - vung co du lieu KHONG do chinh luot nay ghi (WrittenCells) va sheet khong do luot tao;
//   - trong vung co cong thuc tham chieu RA NGOAI vung voi hang TUONG DOI (vd =D5*2, D nam ngoai vung): sau khi doi hang,
//     cong thuc doc nham hang khac - du lieu cua chinh luot cung hong.
// Doc vung bang MOT lan et.readRanges (cong thuc + gia tri). Doc loi thi hoi (huong an toan).

// relativeRow: tham chieu co it nhat mot so hang KHONG co '$' dung truoc (A1, A$1:B2 thi B2 tuong doi).
var (
	relativeRow = regexp.MustCompile(`(^|[^$0-9])[0-9]+`)
	columnPart  = regexp.MustCompile(`\$?[A-Za-z]+`)
)

func refHasRelativeRow(ref FormulaRef) bool {
	text := ref.Text
	if index := strings.LastIndex(text, "!"); index >= 0 {
		text = text[index+1:]
	}
	if ref.Whole && ref.FromRow == 0 { // ca cot (A:C): khong co so hang
		return false
	}
	// Bo chu cot KEM '$' cua no ($D5 -> 5: cot tuyet doi, hang tuong doi) roi tim so hang khong co '$' dung truoc.
	return relativeRow.MatchString(columnPart.ReplaceAllString(text, ""))
}

// sortGuardReason: ly do hoi (rong = khong can hoi).
func sortGuardReason(ctx context.Context, run *RunContext, params map[string]any) string {
	address := strings.TrimSpace(overwriteStringParam(params, "range"))
	refs, _ := ParseFormulaRefs(address)
	if len(refs) != 1 || refs[0].Whole {
		return "" // bridge se bao loi tham so
	}
	area := refs[0]
	sheet := area.Sheet
	if sheet == "" {
		sheet = overwriteStringParam(params, "sheet")
	}
	rangeText := a1Name(area.FromRow, area.FromCol) + ":" + a1Name(area.ToRow, area.ToCol)
	read := map[string]any{"range": rangeText}
	if sheet != "" {
		read["sheet"] = sheet
	}
	formulasRead := map[string]any{"range": rangeText, "formulas": true}
	if sheet != "" {
		formulasRead["sheet"] = sheet
	}
	readParams := map[string]any{"reads": []any{read, formulasRead}}
	if target := strings.TrimSpace(run.TargetDocument); target != "" {
		readParams["workbook"] = target
	}
	result := run.Bridge.Command(ctx, run.Office.Port, "et.readRanges", readParams)
	list, _ := result.Result["reads"].([]any)
	if !result.OK || len(list) < 2 {
		return "Sắp xếp lại vùng " + address + " (không đọc trước được dữ liệu; Excel không hoàn tác được bằng Ctrl+Z)"
	}
	valuesEntry, _ := list[0].(map[string]any)
	formulasEntry, _ := list[1].(map[string]any)
	if name, _ := valuesEntry["sheet"].(string); strings.TrimSpace(name) != "" {
		sheet = strings.TrimSpace(name)
	}
	header := true
	if flag, ok := params["header"].(bool); ok {
		header = flag
	}
	firstData := area.FromRow
	if header {
		firstData++
	}
	// 1. Du lieu co san (khong do luot ghi).
	foreign := 0
	for r, row := range matrixOf(valuesEntry["values"]) {
		for c, value := range row {
			if isEmptyOverwriteCell(value) || run.CreatedSheets[sheet] {
				continue
			}
			if !run.WrittenCells[sheet][a1Name(area.FromRow+r, area.FromCol+c)] {
				foreign++
			}
		}
	}
	// 2. Cong thuc tham chieu ra ngoai vung theo hang tuong doi.
	outside := []string{}
	for r, row := range matrixOf(formulasEntry["formulas"]) {
		if area.FromRow+r < firstData {
			continue
		}
		for c, value := range row {
			formula, _ := value.(string)
			if formula == "" {
				continue
			}
			parsed, _ := ParseFormulaRefs(formula)
			for _, ref := range parsed {
				inside := (ref.Sheet == "" || strings.EqualFold(ref.Sheet, sheet)) && !ref.Whole &&
					ref.FromRow >= firstData && ref.ToRow <= area.ToRow && ref.FromCol >= area.FromCol && ref.ToCol <= area.ToCol
				if inside || !refHasRelativeRow(ref) {
					continue
				}
				if len(outside) < 3 {
					outside = append(outside, a1Name(area.FromRow+r, area.FromCol+c)+" "+formula)
				}
				break
			}
		}
	}
	if foreign == 0 && len(outside) == 0 {
		return ""
	}
	parts := []string{}
	if foreign > 0 {
		parts = append(parts, fmt.Sprintf("Sắp xếp lại %d ô dữ liệu có sẵn trong %s (Excel không hoàn tác được bằng Ctrl+Z)", foreign,
			qualifiedSheet(sheet)+"!"+rangeText))
	}
	if len(outside) > 0 {
		parts = append(parts, "có công thức tham chiếu ra ngoài vùng theo hàng tương đối, sắp xếp xong sẽ trỏ nhầm hàng (vd "+
			strings.Join(outside, "; ")+")")
	}
	return strings.Join(parts, "; ")
}
