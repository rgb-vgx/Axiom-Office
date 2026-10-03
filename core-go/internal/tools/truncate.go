package tools

import (
	"fmt"
	"strconv"
	"strings"

	"axiomoffice/core/internal/model"
)

// Buoc 2 roadmap hieu nang (con-lai muc 7): chat response doc lon o WRAPPER Go truoc khi no vao
// turns cua agent - `turns` append moi tool result nghien nguyen, doc lai 50.001 dong mot lan thi
// input phinh toan bo phan con lai cua luot (Test 4 rong: 174 lan readRange).
//
// Nguyen tac vang (cung nhu trong extension): cat CHI TIET duoc, GIU DU SO LOI -
//   et.readRange : mau dau + truncated/totalRows/nextRange de doc tiep co nhiem vu
//   et.checkRange : issueCount va toan bo issue NGUYEN ven, chi cat list dai
//
// Neu phan tich that bai khong duoc gi -> tra lai nguyen ban, khong bao gio lam hong lenh.
const (
	// ReadHeadRows: so dong dau giu lai khi doc qua dai. Skill mo-hinh-nhieu-sheet quy oc
	// "vung lon hon vai tram dong chi doc mau dau/cuoi + kich thuoc" - 200 dong dau du de xem
	// cau truc/bieu mau, het thi doc tiep bang nextRange.
	ReadHeadRows = 200
	// ReadCellBudget: gioi han tong o (dong dau x cot) - doc 200 dong x 100 cot van con lon.
	// Dong hop ly nhat tinh tu ca hai: min(ReadHeadRows, budget/cot), it nhat 1 dong.
	ReadCellBudget = 6000
	// CheckIssuesCap: checkRange cung cap issue cho vung ban qua day. issueCount trong payload
	// da la so that (extension dem truoc) nen cat list van giu du so loi.
	CheckIssuesCap = 100
	// TailRows: mau DUOI cung kem voi mau dau (tu van ChatGPT 04/10: chi dau thi model co the ket
	// luan "toan bo vung sach" tu mau dau - dau va cuoi KHONG nhat thiet giong nhau: loi o cuoi
	// bang, append day cuoi, cong thuc cuoi lech...). Tail la SENTINEL, khong thay the nextRange.
	TailRows = 10
)

// TruncateBridgeResult: cat payload cua MOT so lenh doc sau khi bridge tra ve.
//   action : ten lenh da goi
//   result : map "result" da parse ( BridgeResult.Result ) - se duoc sua tren cho
//   raw    : body nguyen van, se duoc thay the neu co cat
// Tra ve body moi, hoac raw goc neu khong cat gi (lenh khac, loi, JSON la).
func TruncateBridgeResult(action string, result map[string]any, raw string) string {
	if result == nil {
		return raw
	}
	switch action {
	case "et.readRange":
		if truncateReadRange(result) {
			return model.MarshalRelaxed(map[string]any{"ok": true, "result": result})
		}
	case "et.checkRange":
		if truncateCheckRange(result) {
			return model.MarshalRelaxed(map[string]any{"ok": true, "result": result})
		}
	}
	return raw
}

// truncateReadRange: giu ReadHeadRows dong dau (nho hon neu vuot cell budget), kem
// truncated/totalRows/nextRange. Tra ve true neu co sua doi.
func truncateReadRange(result map[string]any) bool {
	key := ""
	if _, ok := result["values"]; ok {
		key = "values"
	} else if _, ok := result["formulas"]; ok {
		key = "formulas"
	}
	if key == "" {
		return false
	}
	matrix, ok := result[key].([]any)
	if !ok || len(matrix) <= ReadHeadRows {
		return false
	}
	// tong o: dem cot cua dong dau (ma tran doc ra thuong vuot)
	cols := 0
	if first, ok := matrix[0].([]any); ok {
		cols = len(first)
	}
	head := ReadHeadRows
	if cols > 0 {
		if byBudget := ReadCellBudget / cols; byBudget < head {
			head = byBudget
		}
		if head < 1 {
			head = 1
		}
	}
	if head >= len(matrix) {
		return false
	}
	totalRows := len(matrix)
	result[key] = matrix[:head]
	result["truncated"] = true
	result["totalRows"] = totalRows
	if cols > 0 {
		result["totalCols"] = cols
	}
	// Mau duoi: lay TailRows dong CUOI cua vung da doc (khong bi trung mau dau - neu vung qua ngan
	// thi tail bat dau tu dong sau mau dau). Ten khoa doi xung: values -> tailValues.
	if tailStart := maxInt(head, totalRows-TailRows); tailStart < totalRows {
		result["tail"+strings.ToUpper(key[:1])+key[1:]] = matrix[tailStart:]
	}
	// nextRange: doc tiep tu dong head+1 den pham vi THAT cua du lieu da doc.
	// Kieu dia chi goc lay trong result["range"] (extension echo lai tham so da gui).
	if requested, ok := result["range"].(string); ok {
		if next := nextRange(requested, head, totalRows); next != "" {
			result["nextRange"] = next
		}
	}
	return true
}

// truncateCheckRange: issueCount da dung (extension dem tu danh sach day du), chi cat list
// issue qua dai va ghi so da bo qua. Tra ve true neu co sua doi.
func truncateCheckRange(result map[string]any) bool {
	issues, ok := result["issues"].([]any)
	if !ok || len(issues) <= CheckIssuesCap {
		return false
	}
	omitted := len(issues) - CheckIssuesCap
	result["issues"] = issues[:CheckIssuesCap]
	result["issuesOmitted"] = omitted
	return true
}

// nextRange: tu kieu dia chi da gui ("A1:C5000", "$A$1:$C$5000", "A1"...) tinh vung doc tiep.
// Tra ve "" neu khong doc duoc dia chi (bo qua truong nay, truncated/totalRows van con).
func nextRange(requested string, consumedRows, totalRows int) string {
	parts := strings.SplitN(strings.TrimSpace(requested), ":", 2)
	startCol, startRow, ok := parseCell(parts[0])
	if !ok {
		return ""
	}
	endCol := startCol
	if len(parts) == 2 {
		col, _, ok := parseCell(parts[1])
		if !ok {
			return ""
		}
		endCol = col
	}
	nextStart := startRow + consumedRows
	endRow := startRow + totalRows - 1
	if endRow < nextStart {
		return ""
	}
	return fmt.Sprintf("%s%d:%s%d", colName(startCol), nextStart, colName(endCol), endRow)
}

// parseCell: "A1" / "$B$12" -> cot 0-based + dong 1-based.
func parseCell(ref string) (col, row int, ok bool) {
	ref = strings.ReplaceAll(strings.TrimSpace(ref), "$", "")
	if ref == "" {
		return 0, 0, false
	}
	index := 0
	for index < len(ref) && ref[index] >= 'A' && ref[index] <= 'Z' {
		col = col*26 + int(ref[index]-'A'+1)
		index++
	}
	if index == 0 || index == len(ref) {
		return 0, 0, false
	}
	row, err := strconv.Atoi(ref[index:])
	if err != nil || row < 1 || col < 1 {
		return 0, 0, false
	}
	return col - 1, row, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// colName: cot 0-based -> ten cot ("0" -> "A").
func colName(col int) string {
	name := ""
	col++
	for col > 0 {
		col--
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return name
}
