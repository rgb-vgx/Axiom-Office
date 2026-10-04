package office

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"axiomoffice/core/internal/model"
)

// Cat response doc lon cua bridge TRUOC khi no vao ngu canh cua mot LLM - dung chung cho agent cua
// Core (office_action, roadmap hieu nang Buoc 2) va tool live cua MCP server (client ngoai cung la
// LLM, cung bi phinh ngu canh). Do duoc: doc 50.001 dong = 1.000.101 byte (~250k token) moi lan.
//
// Nguyen tac vang (cung nhu trong extension): cat CHI TIET duoc, GIU DU SO LOI -
//
//	et.readRange : mau dau + mau duoi + truncated/totalRows/nextRange de doc tiep co nhiem vu
//	et.checkRange : issueCount NGUYEN ven, chi cat danh sach issue qua dai
//
// Va KHONG BAO GIO cat im lang: cat thi luon co truncated=true (thong le chung: cat im lang lam model
// tu tin tom tat ca phan no khong thay). Phan tich that bai -> tra lai nguyen ban.
type Truncation struct {
	// HeadRows: so dong dau toi da giu lai. 200 dong du de thay cau truc/bieu mau.
	HeadRows int
	// CellBudget: tran tong o cua mau dau (dong x cot) - 200 dong x 100 cot van con lon.
	CellBudget int
	// TailRows: mau DUOI kem mau dau (tu van ChatGPT 04/10: chi co dau thi model ket luan "ca vung
	// sach" tu mau dau - loi o cuoi bang, du lieu append cuoi...). Sentinel, khong thay nextRange.
	TailRows int
	// IssuesCap: so issue checkRange giu lai; issueCount da la so that nen van du so loi.
	IssuesCap int
	// Hint: cau huong dan gan vao ket qua bi cat (rong = khong gan). Agent cua Core da co contract
	// trong mo ta tool nen de rong; MCP gan vi khong kiem soat duoc client doc mo ta the nao.
	Hint string
}

// DefaultTruncation: gioi han mac dinh (cung co voi excel-mcp-server: 4.000 o/lan).
var DefaultTruncation = Truncation{HeadRows: 200, CellBudget: 6000, TailRows: 10, IssuesCap: 100}

// Apply cat payload cua MOT so lenh doc.
//
//	action : ten lenh da goi
//	result : map "result" da parse - se duoc sua tren cho
//	raw    : body nguyen van, tra lai nguyen neu khong cat gi (lenh khac, loi, JSON la)
func (t Truncation) Apply(action string, result map[string]any, raw string) string {
	if result == nil {
		return raw
	}
	changed := false
	switch action {
	case "et.readRange":
		changed = t.readRange(result)
	case "et.checkRange":
		changed = t.checkRange(result)
	}
	if !changed {
		return raw
	}
	if t.Hint != "" {
		result["hint"] = t.Hint
	}
	return model.MarshalRelaxed(map[string]any{"ok": true, "result": result})
}

// ApplyRaw: nhu Apply nhung tu parse body (dung cho duong chi co body nguyen van, vd MCP).
// Lenh loi ("ok":false) va body khong phai JSON object -> tra nguyen van.
func (t Truncation) ApplyRaw(action, raw string) string {
	if action != "et.readRange" && action != "et.checkRange" {
		return raw
	}
	var body map[string]any
	if json.Unmarshal([]byte(raw), &body) != nil {
		return raw
	}
	if ok, _ := body["ok"].(bool); !ok {
		return raw
	}
	result, _ := body["result"].(map[string]any)
	return t.Apply(action, result, raw)
}

func (t Truncation) readRange(result map[string]any) bool {
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
	if !ok || len(matrix) <= t.HeadRows {
		return false
	}
	cols := 0
	if first, ok := matrix[0].([]any); ok {
		cols = len(first)
	}
	head := t.HeadRows
	if cols > 0 {
		head = min(head, t.CellBudget/cols)
		head = max(head, 1)
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
	// Mau duoi: TailRows dong CUOI, khong trung mau dau. Ten khoa doi xung: values -> tailValues.
	if tailStart := max(head, totalRows-t.TailRows); t.TailRows > 0 && tailStart < totalRows {
		result["tail"+strings.ToUpper(key[:1])+key[1:]] = matrix[tailStart:]
	}
	// nextRange tu dia chi extension echo lai trong result["range"].
	if requested, ok := result["range"].(string); ok {
		if next := nextRange(requested, head, totalRows); next != "" {
			result["nextRange"] = next
		}
	}
	return true
}

func (t Truncation) checkRange(result map[string]any) bool {
	issues, ok := result["issues"].([]any)
	if !ok || len(issues) <= t.IssuesCap {
		return false
	}
	result["issues"] = issues[:t.IssuesCap]
	result["issuesOmitted"] = len(issues) - t.IssuesCap
	return true
}

// nextRange: tu dia chi da gui ("A1:C5000", "$A$1:$C$5000", "a1:c5000") tinh vung doc tiep.
// "" neu khong doc duoc dia chi (truncated/totalRows van con).
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

// parseCell: "A1" / "$B$12" / "c3" -> cot 0-based + dong 1-based.
func parseCell(ref string) (col, row int, ok bool) {
	ref = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(ref), "$", ""))
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

// colName: cot 0-based -> ten cot (0 -> "A").
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
