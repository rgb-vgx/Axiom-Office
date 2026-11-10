package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TraceFormulaTool: lan nguoc (o nay doc tu dau) hoac lan xuoi (o nao doc o nay) cong thuc bang tinh, tim goc loi
// (#REF!, #DIV/0!...). Lam o Core tren ket qua et.readRanges / et.sheetInfo nen chay tren moi backend va theo duoc ca tham
// chieu sang sheet khac (Range.DirectPrecedents cua Excel thi khong). Chi doc.
type TraceFormulaTool struct{}

func NewTraceFormulaTool() *TraceFormulaTool { return &TraceFormulaTool{} }

func (t *TraceFormulaTool) Name() string { return "trace_formula" }

func (t *TraceFormulaTool) Description() string {
	return "Trace spreadsheet formulas (read-only). direction \"precedents\" (default): which cells the given cell reads, " +
		"level by level, with each cell's formula and value; error cells (#REF!, #DIV/0!, #N/A...) whose inputs are all " +
		"clean are reported as rootCauses. direction \"dependents\": which formula cells read the given cell (scans the " +
		"workbook). cell: e.g. \"C42\" or \"Summary!H15\"; depth: levels to follow (default 3, max 6). Use it to explain how " +
		"a number is computed or to find where an error starts, instead of reading ranges one by one. Named ranges, " +
		"INDIRECT/OFFSET and links to other workbooks are listed as unresolved."
}

func (t *TraceFormulaTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"cell":{"type":"string","description":"cell to trace, e.g. C42 or Summary!H15"},` +
		`"direction":{"type":"string","enum":["precedents","dependents"]},` +
		`"depth":{"type":"integer","description":"levels to follow, 1-6 (default 3)"}},"required":["cell"]}`)
}

const (
	traceMaxNodes       = 80
	traceExpandCells    = 25    // vung nho hon thi lan tung o; lon hon thi chi tim o loi trong vung
	traceReadCellsLimit = 3500  // duoi tran 4000 o / lan goi et.readRanges cua bridge
	traceScanCellsLimit = 40000 // tran o quet khi lan xuoi (dependents)
)

type traceCell struct {
	Sheet    string
	Row, Col int
}

func (c traceCell) key() string { return strings.ToLower(c.Sheet) + "!" + a1Name(c.Row, c.Col) }

func (c traceCell) ref() string { return qualifiedSheet(c.Sheet) + "!" + a1Name(c.Row, c.Col) }

type traceNode struct {
	Ref        string   `json:"ref"`
	Depth      int      `json:"depth"`
	Formula    string   `json:"formula,omitempty"`
	Value      any      `json:"value"`
	Error      string   `json:"error,omitempty"`
	Reads      []string `json:"reads,omitempty"`
	ReadBy     string   `json:"readBy,omitempty"`
	Unresolved []string `json:"unresolved,omitempty"`
	cell       traceCell
	children   []string
}

// traceReader: doc vung qua bridge (et.readRanges) - tach rieng de test bang bridge gia.
type traceReader struct {
	ctx  context.Context
	run  *RunContext
	call func(action string, params map[string]any) (map[string]any, string)
}

type traceRead struct {
	Sheet    string
	Range    string
	Formulas bool
	cells    int
}

// read: doc nhieu vung, chia lo duoi tran o cua bridge. Tra ma tran theo thu tu yeu cau.
func (r *traceReader) read(reads []traceRead) ([][][]any, string) {
	out := make([][][]any, len(reads))
	start := 0
	for start < len(reads) {
		end, cells := start, 0
		for end < len(reads) && (end == start || cells+reads[end].cells <= traceReadCellsLimit) {
			cells += reads[end].cells
			end++
		}
		batch := []any{}
		for _, item := range reads[start:end] {
			batch = append(batch, map[string]any{"sheet": item.Sheet, "range": item.Range, "formulas": item.Formulas})
		}
		result, problem := r.call("et.readRanges", map[string]any{"reads": batch})
		if problem != "" {
			return nil, problem
		}
		list, _ := result["reads"].([]any)
		for index := range reads[start:end] {
			if index >= len(list) {
				break
			}
			entry, _ := list[index].(map[string]any)
			key := "values"
			if reads[start+index].Formulas {
				key = "formulas"
			}
			out[start+index] = matrixOf(entry[key])
		}
		start = end
	}
	return out, ""
}

func matrixOf(value any) [][]any {
	rows, _ := value.([]any)
	matrix := make([][]any, 0, len(rows))
	for _, row := range rows {
		cells, _ := row.([]any)
		matrix = append(matrix, cells)
	}
	return matrix
}

func firstCell(matrix [][]any) any {
	if len(matrix) == 0 || len(matrix[0]) == 0 {
		return nil
	}
	return matrix[0][0]
}

// spreadsheetError: gia tri loi cua o (bridge tra loi Excel thanh chuoi "#REF!"...).
func spreadsheetError(value any) string {
	text, ok := value.(string)
	if !ok || !strings.HasPrefix(text, "#") {
		return ""
	}
	switch strings.ToUpper(strings.TrimSpace(text)) {
	case "#NULL!", "#DIV/0!", "#VALUE!", "#REF!", "#NAME?", "#NUM!", "#N/A", "#GETTING_DATA", "#SPILL!", "#CALC!", "#FIELD!":
		return strings.TrimSpace(text)
	}
	return ""
}

func (t *TraceFormulaTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	target := strings.TrimSpace(run.TargetDocument)
	reader := &traceReader{ctx: ctx, run: run}
	reader.call = func(action string, params map[string]any) (map[string]any, string) {
		if target != "" {
			params["workbook"] = target
		}
		result := run.Bridge.Command(ctx, run.Office.Port, action, params)
		run.trace("tool.bridge", map[string]any{"tool": "trace_formula", "action": action, "ok": result.OK, "error": result.Error})
		if !result.OK {
			return nil, action + ": " + result.Error
		}
		return result.Result, ""
	}
	direction := strings.ToLower(stringField(arguments["direction"]))
	if direction == "" {
		direction = "precedents"
	}
	if direction != "precedents" && direction != "dependents" {
		return Result{JSON: ErrorJSON("direction must be \"precedents\" or \"dependents\"")}
	}
	depth := 3
	if value := asInt(arguments["depth"]); value > 0 {
		depth = min(value, 6)
	}
	start, problem := t.startCell(reader, stringField(arguments["cell"]))
	if problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	var payload map[string]any
	if direction == "precedents" {
		payload, problem = tracePrecedents(reader, start, depth)
	} else {
		payload, problem = traceDependents(reader, start, depth)
	}
	if problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	payload["cell"], payload["direction"] = start.ref(), direction
	return Result{JSON: OKJSON(payload), OK: true, Verifies: true}
}

// startCell: "C42" (sheet dang mo) hoac "Summary!H15".
func (t *TraceFormulaTool) startCell(reader *traceReader, text string) (traceCell, string) {
	text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "="))
	if text == "" {
		return traceCell{}, "'cell' is required, e.g. \"C42\" or \"Summary!H15\""
	}
	refs, _ := ParseFormulaRefs(text)
	if len(refs) != 1 || refs[0].Whole || refs[0].Cells() != 1 || len(refs[0].Text) != len(text) {
		return traceCell{}, "'cell' must be ONE cell such as \"C42\" or \"Summary!H15\", got \"" + text + "\""
	}
	sheet := refs[0].Sheet
	if sheet == "" {
		info, problem := reader.call("et.sheetInfo", map[string]any{})
		if problem != "" {
			return traceCell{}, problem
		}
		sheet, _ = info["activeSheet"].(string)
	}
	return traceCell{Sheet: sheet, Row: refs[0].FromRow, Col: refs[0].FromCol}, ""
}

func tracePrecedents(reader *traceReader, start traceCell, depth int) (map[string]any, string) {
	nodes := map[string]*traceNode{}
	order := []*traceNode{}
	notes := []string{}
	level := []traceCell{start}
	parents := map[string]string{}
	for current := 0; current <= depth && len(level) > 0; current++ {
		reads := []traceRead{}
		for _, cell := range level {
			reads = append(reads,
				traceRead{Sheet: cell.Sheet, Range: a1Name(cell.Row, cell.Col), Formulas: true, cells: 1},
				traceRead{Sheet: cell.Sheet, Range: a1Name(cell.Row, cell.Col), cells: 1})
		}
		matrices, problem := reader.read(reads)
		if problem != "" {
			return nil, problem
		}
		next := []traceCell{}
		bigRanges := []FormulaRef{}
		bigOwners := []*traceNode{}
		for index, cell := range level {
			formula, _ := firstCell(matrices[index*2]).(string)
			value := firstCell(matrices[index*2+1])
			node := &traceNode{Ref: cell.ref(), Depth: current, Formula: formula, Value: value, Error: spreadsheetError(value),
				ReadBy: parents[cell.key()], cell: cell}
			nodes[cell.key()] = node
			order = append(order, node)
			if formula == "" || current == depth {
				continue
			}
			refs, unresolved := ParseFormulaRefs(formula)
			node.Unresolved = unresolved
			for _, ref := range refs {
				sheet := ref.Sheet
				if sheet == "" {
					sheet = cell.Sheet
				}
				switch {
				case ref.Whole:
					node.Reads = append(node.Reads, ref.Text)
					node.Unresolved = append(node.Unresolved, ref.Text+": whole row/column not followed")
				case ref.Cells() <= traceExpandCells:
					node.Reads = append(node.Reads, ref.Text)
					for row := ref.FromRow; row <= ref.ToRow; row++ {
						for col := ref.FromCol; col <= ref.ToCol; col++ {
							child := traceCell{Sheet: sheet, Row: row, Col: col}
							node.children = append(node.children, child.key())
							if _, seen := nodes[child.key()]; !seen && parents[child.key()] == "" {
								parents[child.key()] = cell.ref()
								next = append(next, child)
							}
						}
					}
				default:
					node.Reads = append(node.Reads, ref.Text)
					ref.Sheet = sheet
					bigRanges = append(bigRanges, ref)
					bigOwners = append(bigOwners, node)
				}
			}
		}
		// Vung lon (vd SUM(B2:B500)): khong lan tung o, chi tim o loi trong vung (goc loi hay nam o day).
		if len(bigRanges) > 0 && current < depth {
			reads := []traceRead{}
			for _, ref := range bigRanges {
				reads = append(reads, traceRead{Sheet: ref.Sheet, Range: a1Name(ref.FromRow, ref.FromCol) + ":" + a1Name(ref.ToRow, ref.ToCol),
					cells: ref.Cells()})
			}
			matrices, problem := reader.read(reads)
			if problem != "" {
				return nil, problem
			}
			for index, ref := range bigRanges {
				found := 0
				for r, row := range matrices[index] {
					for c, value := range row {
						if spreadsheetError(value) == "" || found >= 5 {
							continue
						}
						found++
						child := traceCell{Sheet: ref.Sheet, Row: ref.FromRow + r, Col: ref.FromCol + c}
						bigOwners[index].children = append(bigOwners[index].children, child.key())
						if _, seen := nodes[child.key()]; !seen && parents[child.key()] == "" {
							parents[child.key()] = bigOwners[index].Ref
							next = append(next, child)
						}
					}
				}
			}
		}
		if len(order)+len(next) > traceMaxNodes {
			next = next[:max(0, traceMaxNodes-len(order))]
			notes = append(notes, fmt.Sprintf("stopped at %d cells; trace a deeper cell separately", traceMaxNodes))
		}
		level = next
	}
	roots := []string{}
	for _, node := range order {
		if node.Error == "" {
			continue
		}
		clean := true
		for _, child := range node.children {
			if other, ok := nodes[child]; ok && other.Error != "" {
				clean = false
			}
		}
		// O loi ma moi o no doc deu sach (hoac go tay "#N/A", hoac cong thuc tro toi o da bi xoa "#REF!"): loi bat dau o day.
		if clean {
			roots = append(roots, node.Ref)
		}
	}
	return map[string]any{"nodes": order, "rootCauses": roots, "notes": notes}, ""
}

func traceDependents(reader *traceReader, start traceCell, depth int) (map[string]any, string) {
	info, problem := reader.call("et.sheetInfo", map[string]any{})
	if problem != "" {
		return nil, problem
	}
	sheets, _ := info["sheets"].([]any)
	type formulaCell struct {
		cell    traceCell
		formula string
		refs    []FormulaRef
	}
	all := []formulaCell{}
	notes := []string{}
	scanned := 0
	for _, item := range sheets {
		entry, _ := item.(map[string]any)
		name, _ := entry["name"].(string)
		used, _ := entry["usedRange"].(string)
		if name == "" || used == "" {
			continue
		}
		refs, _ := ParseFormulaRefs(used)
		if len(refs) != 1 || refs[0].Whole {
			continue
		}
		area := refs[0]
		if scanned+area.Cells() > traceScanCellsLimit {
			notes = append(notes, "sheet "+name+" not scanned (workbook too large to scan in one call)")
			continue
		}
		scanned += area.Cells()
		// Chia theo hang de moi lan doc duoi tran o cua bridge.
		rowsPerRead := max(1, traceReadCellsLimit/(area.ToCol-area.FromCol+1))
		reads := []traceRead{}
		for row := area.FromRow; row <= area.ToRow; row += rowsPerRead {
			last := min(area.ToRow, row+rowsPerRead-1)
			reads = append(reads, traceRead{Sheet: name, Range: a1Name(row, area.FromCol) + ":" + a1Name(last, area.ToCol), Formulas: true,
				cells: (last - row + 1) * (area.ToCol - area.FromCol + 1)})
		}
		matrices, problem := reader.read(reads)
		if problem != "" {
			return nil, problem
		}
		for index, matrix := range matrices {
			firstRow := area.FromRow + index*rowsPerRead
			for r, row := range matrix {
				for c, value := range row {
					formula, _ := value.(string)
					if !strings.HasPrefix(formula, "=") {
						continue
					}
					parsed, _ := ParseFormulaRefs(formula)
					all = append(all, formulaCell{cell: traceCell{Sheet: name, Row: firstRow + r, Col: area.FromCol + c},
						formula: formula, refs: parsed})
				}
			}
		}
	}
	nodes := []*traceNode{{Ref: start.ref(), Depth: 0, cell: start}}
	seen := map[string]bool{start.key(): true}
	level := []traceCell{start}
	for current := 1; current <= depth && len(level) > 0 && len(nodes) < traceMaxNodes; current++ {
		next := []traceCell{}
		for _, candidate := range all {
			if seen[candidate.cell.key()] {
				continue
			}
			for _, target := range level {
				readsTarget := false
				for _, ref := range candidate.refs {
					if ref.Contains(candidate.cell.Sheet, target.Sheet, target.Row, target.Col) {
						readsTarget = true
						break
					}
				}
				if readsTarget {
					seen[candidate.cell.key()] = true
					nodes = append(nodes, &traceNode{Ref: candidate.cell.ref(), Depth: current, Formula: candidate.formula,
						ReadBy: target.ref(), cell: candidate.cell})
					next = append(next, candidate.cell)
					break
				}
			}
			if len(nodes) >= traceMaxNodes {
				notes = append(notes, fmt.Sprintf("stopped at %d cells", traceMaxNodes))
				break
			}
		}
		level = next
	}
	// Gia tri hien tai cua cac o (mot lan doc).
	reads := []traceRead{}
	for _, node := range nodes {
		reads = append(reads, traceRead{Sheet: node.cell.Sheet, Range: a1Name(node.cell.Row, node.cell.Col), cells: 1})
	}
	matrices, problem := reader.read(reads)
	if problem != "" {
		return nil, problem
	}
	for index, node := range nodes {
		node.Value = firstCell(matrices[index])
		node.Error = spreadsheetError(node.Value)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Depth != nodes[j].Depth {
			return nodes[i].Depth < nodes[j].Depth
		}
		return nodes[i].Ref < nodes[j].Ref
	})
	return map[string]any{"nodes": nodes, "dependents": len(nodes) - 1, "notes": notes}, ""
}
