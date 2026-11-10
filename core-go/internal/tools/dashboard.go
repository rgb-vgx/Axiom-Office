package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"axiomoffice/core/internal/corelog"
)

// DashboardFromDataTool: dung sheet dashboard tu mot bang du lieu co san (docs/plans/21-v0 P3): o KPI la CONG THUC tro
// thang vao sheet du lieu (doi du lieu -> KPI doi theo), bieu do doc thang vung o sheet du lieu, xep theo luoi. Model
// chi chon chi so va bieu do. Bo do 26/10/2026: de B (workbook 4 sheet) model yeu tu dung dashboard mat 51-60 vong, ca
// x09 tung chep du lieu sang lam bang trung gian va ve 4 bieu do rac.
type DashboardFromDataTool struct{}

func NewDashboardFromDataTool() *DashboardFromDataTool { return &DashboardFromDataTool{} }

func (t *DashboardFromDataTool) Name() string { return "dashboard_from_data" }

func (t *DashboardFromDataTool) Description() string {
	return "Build a dashboard sheet from a data table that is already in the workbook: KPI tiles are FORMULAS that read the " +
		"data sheet (they update when the data changes) and charts read the data sheet directly, laid out on a grid. " +
		"source: the data table with its header row, e.g. \"Data!A1:E13\"; sheet: dashboard sheet name (default " +
		"\"Dashboard\", created if missing; it must be empty); title; kpis: 1-6 of {label, column (header text or letter), " +
		"agg: sum|average|max|min|count, format: integer|decimal|percent|vnd}; charts: 0-4 of {type: column|bar|line|pie, " +
		"title, x: category column, y: [1-3 value columns] (pie: 1)}. To change a dashboard this tool built earlier in this " +
		"request (wrong chart type, another KPI), call it again with the whole spec and replace: true - it rebuilds that " +
		"sheet. Use this instead of writing KPI cells and charts one by one; never copy the data to the dashboard."
}

func (t *DashboardFromDataTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{` +
		`"source":{"type":"string","description":"data table with header row, e.g. Data!A1:E13"},` +
		`"sheet":{"type":"string"},"title":{"type":"string"},` +
		`"kpis":{"type":"array","items":{"type":"object","properties":{"label":{"type":"string"},"column":{"type":"string"},` +
		`"agg":{"type":"string","enum":["sum","average","max","min","count"]},"format":{"type":"string","enum":["integer","decimal","percent","vnd"]}}}},` +
		`"charts":{"type":"array","items":{"type":"object","properties":{"type":{"type":"string","enum":["column","bar","line","pie"]},` +
		`"title":{"type":"string"},"x":{"type":"string"},"y":{"type":"array","items":{"type":"string"}}}}},` +
		`"replace":{"type":"boolean","description":"rebuild the dashboard sheet this tool built earlier in this request"}},` +
		`"required":["source"]}`)
}

var cellRange = regexp.MustCompile(`^\$?([A-Za-z]{1,3})\$?(\d+):\$?([A-Za-z]{1,3})\$?(\d+)$`)

var dashboardAggs = map[string]string{"sum": "SUM", "average": "AVERAGE", "max": "MAX", "min": "MIN", "count": "COUNT"}

// dashboardFormats: numFmt cua _design/tokens.json (spreadsheet.numberFormats).
var dashboardFormats = map[string]string{"integer": "#,##0", "decimal": "#,##0.00", "percent": "0.0%", "vnd": `#,##0 "đ"`}

const (
	dashboardKpiWidth = 24.0 // ky tu
	dashboardChartCm  = 12.0
	dashboardChartH   = 7.0
	dashboardRowCm    = 0.53 // hang mac dinh 15pt
)

type dashboardSource struct {
	Sheet            string
	FirstCol, Row    int
	LastCol, LastRow int
	Headers          []string
	Rows             [][]any
}

func columnNumber(letters string) int {
	value := 0
	for _, ch := range strings.ToUpper(letters) {
		value = value*26 + int(ch-'A'+1)
	}
	return value
}

func columnLetters(number int) string {
	name := ""
	for number > 0 {
		rem := (number - 1) % 26
		name = string(rune('A'+rem)) + name
		number = (number - 1) / 26
	}
	return name
}

func quoteSheet(name string) string {
	if regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// column: so cot (tuyet doi) theo ten tieu de (khong phan biet hoa/thuong) hoac chu cot trong vung nguon.
func (s dashboardSource) column(name string) (int, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	for i, header := range s.Headers {
		if strings.ToLower(strings.TrimSpace(header)) == key {
			return s.FirstCol + i, true
		}
	}
	if regexp.MustCompile(`^[A-Za-z]{1,3}$`).MatchString(key) {
		col := columnNumber(key)
		if col >= s.FirstCol && col <= s.LastCol {
			return col, true
		}
	}
	return 0, false
}

func (s dashboardSource) numeric(col int) bool {
	for _, row := range s.Rows {
		if i := col - s.FirstCol; i < len(row) {
			if _, ok := row[i].(float64); ok {
				return true
			}
		}
	}
	return false
}

// dataRef: vung du lieu (bo hang tieu de) cua mot cot, kem ten sheet: 'Data'!C2:C13.
func (s dashboardSource) dataRef(col int) string {
	letter := columnLetters(col)
	return fmt.Sprintf("%s!$%s$%d:$%s$%d", quoteSheet(s.Sheet), letter, s.Row+1, letter, s.LastRow)
}

type dashboardKpi struct{ Label, Formula, Format string }

type dashboardChart struct {
	Type, Title, Range string
}

func (t *DashboardFromDataTool) plan(arguments map[string]any, source dashboardSource) ([]dashboardKpi, []dashboardChart, []string) {
	problems := []string{}
	kpis := []dashboardKpi{}
	list, _ := arguments["kpis"].([]any)
	if len(list) > 6 {
		problems = append(problems, fmt.Sprintf("kpis: at most 6, got %d", len(list)))
	}
	for i, item := range list {
		entry, _ := item.(map[string]any)
		label, column := stringField(entry["label"]), stringField(entry["column"])
		agg := strings.ToLower(stringField(entry["agg"]))
		if agg == "" {
			agg = "sum"
		}
		col, ok := source.column(column)
		switch {
		case !ok && column == "":
			problems = append(problems, fmt.Sprintf("kpis[%d] needs column: the header of the data column to %s, e.g. "+
				"{\"label\": \"Tổng doanh thu\", \"column\": %q, \"agg\": \"sum\"} (headers: %s)", i, agg, source.Headers[len(source.Headers)-1],
				strings.Join(source.Headers, ", ")))
			continue
		case !ok:
			problems = append(problems, fmt.Sprintf("kpis[%d].column %q is not a column of the source (headers: %s)", i, column, strings.Join(source.Headers, ", ")))
			continue
		case dashboardAggs[agg] == "":
			problems = append(problems, fmt.Sprintf("kpis[%d].agg must be sum, average, max, min or count", i))
			continue
		case agg != "count" && !source.numeric(col):
			problems = append(problems, fmt.Sprintf("kpis[%d]: column %q has no numbers to %s", i, column, agg))
			continue
		}
		if label == "" {
			label = source.Headers[col-source.FirstCol]
		}
		format := strings.ToLower(stringField(entry["format"]))
		if format == "" {
			format = "integer"
			if agg == "average" {
				format = "decimal"
			}
		}
		if dashboardFormats[format] == "" {
			problems = append(problems, fmt.Sprintf("kpis[%d].format must be integer, decimal, percent or vnd", i))
			continue
		}
		kpis = append(kpis, dashboardKpi{Label: label, Formula: "=" + dashboardAggs[agg] + "(" + source.dataRef(col) + ")", Format: format})
	}
	charts := []dashboardChart{}
	chartList, _ := arguments["charts"].([]any)
	if len(chartList) > 4 {
		problems = append(problems, fmt.Sprintf("charts: at most 4, got %d", len(chartList)))
	}
	for i, item := range chartList {
		entry, _ := item.(map[string]any)
		kind := strings.ToLower(stringField(entry["type"]))
		if kind == "" {
			kind = "column"
		}
		if kind != "column" && kind != "bar" && kind != "line" && kind != "pie" {
			problems = append(problems, fmt.Sprintf("charts[%d].type must be column, bar, line or pie", i))
			continue
		}
		x, ok := source.column(stringField(entry["x"]))
		if !ok {
			problems = append(problems, fmt.Sprintf("charts[%d].x %q is not a column of the source", i, stringField(entry["x"])))
			continue
		}
		ys := briefLines(entry["y"])
		if len(ys) == 0 || len(ys) > 3 || (kind == "pie" && len(ys) != 1) {
			problems = append(problems, fmt.Sprintf("charts[%d].y needs 1-3 value columns (pie: exactly 1)", i))
			continue
		}
		parts := []string{fmt.Sprintf("%s%d:%s%d", columnLetters(x), source.Row, columnLetters(x), source.LastRow)}
		for _, y := range ys {
			col, ok := source.column(y)
			if !ok || !source.numeric(col) {
				problems = append(problems, fmt.Sprintf("charts[%d].y %q is not a numeric column of the source", i, y))
				continue
			}
			parts = append(parts, fmt.Sprintf("%s%d:%s%d", columnLetters(col), source.Row, columnLetters(col), source.LastRow))
		}
		title := stringField(entry["title"])
		charts = append(charts, dashboardChart{Type: kind, Title: title, Range: quoteSheet(source.Sheet) + "!" + strings.Join(parts, ",")})
	}
	if len(kpis) == 0 && len(charts) == 0 && len(problems) == 0 {
		problems = append(problems, "give at least one kpi or chart")
	}
	return kpis, charts, problems
}

// chartAnchor: o goc tren-trai cua bieu do thu i - 2 bieu do mot hang, duoi cac o KPI.
func chartAnchor(i, kpiColumns, top int) string {
	row := top + (i/2)*int(math.Ceil((dashboardChartH+0.6)/dashboardRowCm))
	if i%2 == 0 {
		return fmt.Sprintf("A%d", row)
	}
	// Cot dau tien ma tong do rong phia truoc >= bieu do + le: cot KPI rong dashboardKpiWidth, cot khac 8.43 ky tu
	// (~0.19 cm/ky tu).
	width, col := 0.0, 1
	for width < dashboardChartCm+0.5 {
		chars := 8.43
		if col <= kpiColumns {
			chars = dashboardKpiWidth
		}
		width += chars * 0.19
		col++
	}
	return fmt.Sprintf("%s%d", columnLetters(col), row)
}

func (t *DashboardFromDataTool) Invoke(ctx context.Context, arguments map[string]any, run *RunContext) Result {
	sourceText := stringField(arguments["source"])
	sheetName := stringField(arguments["sheet"])
	if sheetName == "" {
		sheetName = "Dashboard"
	}
	target := strings.TrimSpace(run.TargetDocument)
	steps := 0
	bridge := func(action string, params map[string]any) (map[string]any, string) {
		if target != "" {
			params["workbook"] = target
		}
		result := run.Bridge.Command(ctx, run.Office.Port, action, params)
		run.trace("tool.bridge", map[string]any{"tool": "dashboard_from_data", "action": action, "params": params,
			"ok": result.OK, "error": result.Error})
		if !result.OK {
			return nil, action + ": " + result.Error
		}
		if makesUndoStep(action) && ChangesDocument(action) {
			steps++
		}
		run.ownedObjects().record(action, params, result.Result)
		// Sheet va o do tool tao la cua luot: model sua lai sau do khong phai hoi ghi de (overwrite.go).
		if action == "et.addSheet" {
			rememberCreatedSheet(run, params)
		}
		rememberWrittenCells(run, action, params, result.Result)
		return result.Result, ""
	}
	sheets, problem := bridge("et.listSheets", map[string]any{})
	if problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	source := dashboardSource{Sheet: stringField(sheets["activeSheet"])}
	address := sourceText
	if bang := strings.LastIndex(sourceText, "!"); bang > 0 {
		source.Sheet = strings.ReplaceAll(strings.Trim(strings.TrimSpace(sourceText[:bang]), "'"), "''", "'")
		address = sourceText[bang+1:]
	}
	match := cellRange.FindStringSubmatch(strings.TrimSpace(address))
	if match == nil {
		return Result{JSON: ErrorJSON(fmt.Sprintf("source must be a range with its header row, e.g. \"Data!A1:E13\" (got %q)", sourceText))}
	}
	source.FirstCol, source.LastCol = columnNumber(match[1]), columnNumber(match[3])
	fmt.Sscan(match[2], &source.Row)
	fmt.Sscan(match[4], &source.LastRow)
	if source.LastRow <= source.Row || source.LastCol < source.FirstCol {
		return Result{JSON: ErrorJSON("source needs a header row and at least one data row, e.g. \"Data!A1:E13\"")}
	}
	if strings.EqualFold(source.Sheet, sheetName) {
		return Result{JSON: ErrorJSON("the dashboard sheet must not be the data sheet - pass another sheet name")}
	}
	data, problem := bridge("et.readRange", map[string]any{"sheet": source.Sheet, "range": strings.TrimSpace(address)})
	if problem != "" {
		return Result{JSON: ErrorJSON(problem)}
	}
	rows, _ := data["values"].([]any)
	for r, row := range rows {
		cells, _ := row.([]any)
		if r == 0 {
			for _, cell := range cells {
				source.Headers = append(source.Headers, strings.TrimSpace(fmt.Sprint(nilToEmpty(cell))))
			}
			continue
		}
		source.Rows = append(source.Rows, cells)
	}
	kpis, charts, problems := t.plan(arguments, source)
	if len(problems) > 0 {
		return Result{JSON: FailJSON(strings.Join(problems, "; "), map[string]any{"built": 0,
			"how": "nothing was built: fix these and call again"})}
	}
	exists := false
	names, _ := sheets["sheets"].([]any)
	for _, name := range names {
		if strings.EqualFold(fmt.Sprint(name), sheetName) {
			exists = true
		}
	}
	replace, _ := arguments["replace"].(bool)
	if exists && replace && run.CreatedSheets[sheetName] {
		// Dashboard do chinh luot nay dung: xoa sheet roi dung lai theo spec moi.
		if _, problem := bridge("et.deleteSheet", map[string]any{"sheet": sheetName}); problem != "" {
			return Result{JSON: ErrorJSON(problem)}
		}
		delete(run.CreatedSheets, sheetName)
		exists = false
	}
	if exists {
		used, problem := bridge("et.readRange", map[string]any{"sheet": sheetName, "range": "A1:P60"})
		if problem != "" {
			return Result{JSON: ErrorJSON(problem)}
		}
		if !emptyMatrix(used["values"]) {
			if run.CreatedSheets[sheetName] {
				return Result{JSON: ErrorJSON(fmt.Sprintf("sheet %q was built earlier in this request - call again with "+
					"replace: true and the whole spec to rebuild it", sheetName))}
			}
			return Result{JSON: ErrorJSON(fmt.Sprintf("sheet %q already has content - pass another sheet name for the dashboard", sheetName))}
		}
	}
	fail := func(problem string) Result {
		run.addUndoSteps("et.writeRange", steps)
		return Result{JSON: FailJSON(problem, map[string]any{"built": "partly",
			"how": "the dashboard sheet is incomplete: delete what was added there or fix the problem and call again with another sheet name"}),
			Mutating: steps > 0}
	}
	if !exists {
		if _, problem := bridge("et.addSheet", map[string]any{"name": sheetName}); problem != "" {
			return fail(problem)
		}
	}
	title := stringField(arguments["title"])
	top := 1
	if title != "" {
		if _, problem := bridge("et.writeRange", map[string]any{"sheet": sheetName, "range": "A1", "values": []any{[]any{title}}}); problem != "" {
			return fail(problem)
		}
		if _, problem := bridge("et.formatRange", map[string]any{"sheet": sheetName, "range": "A1", "bold": true, "fontSize": 16,
			"fontColor": "#1F4E79"}); problem != "" {
			return fail(problem)
		}
		top = 3
	}
	chartTop := top
	if len(kpis) > 0 {
		labels, formulas := []any{}, []any{}
		for _, kpi := range kpis {
			labels = append(labels, kpi.Label)
			formulas = append(formulas, kpi.Formula)
		}
		last := columnLetters(len(kpis))
		labelRange := fmt.Sprintf("A%d:%s%d", top, last, top)
		valueRange := fmt.Sprintf("A%d:%s%d", top+1, last, top+1)
		written, problem := bridge("et.writeRange", map[string]any{"sheet": sheetName, "range": fmt.Sprintf("A%d", top),
			"values": []any{labels, formulas}})
		if problem != "" {
			return fail(problem)
		}
		if errors, _ := written["formulaErrors"].([]any); len(errors) > 0 {
			return fail(fmt.Sprintf("KPI formulas show errors: %v", errors))
		}
		formats := []map[string]any{
			{"range": labelRange, "bold": true, "fontColor": "#FFFFFF", "fillColor": "#1F4E79", "horizontal": "center", "columnWidth": dashboardKpiWidth},
			{"range": valueRange, "bold": true, "fontSize": 18, "fontColor": "#1F4E79", "fillColor": "#DEEAF6", "horizontal": "center"},
		}
		for k, kpi := range kpis {
			cell := fmt.Sprintf("%s%d", columnLetters(k+1), top+1)
			formats = append(formats, map[string]any{"range": cell, "numFmt": dashboardFormats[kpi.Format]})
		}
		for _, format := range formats {
			format["sheet"] = sheetName
			if _, problem := bridge("et.formatRange", format); problem != "" {
				return fail(problem)
			}
		}
		chartTop = top + 3
	}
	builtCharts := []any{}
	for i, chart := range charts {
		params := map[string]any{"sheet": sheetName, "range": chart.Range, "type": chart.Type,
			"anchor": chartAnchor(i, len(kpis), chartTop), "width": dashboardChartCm, "height": dashboardChartH}
		if chart.Title != "" {
			params["title"] = chart.Title
		}
		result, problem := bridge("et.addChart", params)
		if problem != "" {
			return fail(problem)
		}
		builtCharts = append(builtCharts, map[string]any{"chart": result["chart"], "type": chart.Type, "source": chart.Range})
	}
	run.addUndoSteps("et.writeRange", steps)
	if _, problem := bridge("et.activateSheet", map[string]any{"sheet": sheetName}); problem != "" {
		corelog.Info("run %s: dashboard_from_data: could not activate %s: %s", run.RunID, sheetName, problem)
	}
	summary := map[string]any{"sheet": sheetName, "charts": builtCharts}
	// Doc lai gia tri KPI: tu kiem cong thuc ra so, khong loi.
	if len(kpis) > 0 {
		values, problem := bridge("et.readRange", map[string]any{"sheet": sheetName,
			"range": fmt.Sprintf("A%d:%s%d", top, columnLetters(len(kpis)), top+1)})
		if problem == "" {
			summary["kpis"] = values["values"]
		}
		formulas := []string{}
		for _, kpi := range kpis {
			formulas = append(formulas, kpi.Label+" "+kpi.Formula)
		}
		summary["formulas"] = formulas
	}
	summary["verified"] = map[string]any{"checked": "KPI formulas read the data sheet and return values; charts read the data sheet",
		"notChecked": "whether these are the indicators the user wanted - compare with the request yourself"}
	return Result{JSON: OKJSON(summary), OK: true, Mutating: true, Verifies: true}
}

func nilToEmpty(value any) any {
	if value == nil {
		return ""
	}
	return value
}

func emptyMatrix(value any) bool {
	rows, _ := value.([]any)
	for _, row := range rows {
		cells, _ := row.([]any)
		for _, cell := range cells {
			if cell != nil && strings.TrimSpace(fmt.Sprint(cell)) != "" {
				return false
			}
		}
	}
	return true
}
