package mcpserver

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"axiomoffice/core/internal/filesafe"
	"axiomoffice/core/internal/ooxml"
	"axiomoffice/core/internal/sheet"
	"axiomoffice/core/internal/xlsx"
)

// Lan file cho bang tinh (thay openpyxl/xlrd/pandas cua excel-mcp cu).
// .xlsx/.xlsm: doc/sua truc tiep OOXML. .csv/.tsv: doc/ghi text. .xls: chi doc, qua LibreOffice.
var workbookExtensions = []string{".xlsx", ".xlsm", ".xltx", ".xltm"}

// excelFileTools la lan file cua bang tinh. Thu tu tham so (nhat la cua excel_format_range)
// duoc test kiem tra nen phai giu dung.
func excelFileTools() []*Tool {
	return []*Tool{
		NewTool("excel_profile",
			"Inspect a spreadsheet file (.xlsx .xlsm .xls .csv .tsv): sheet names, row/column counts, header row and a small sample per sheet.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("sheet", "", false, nil)},
			func(a *ToolArgs) (any, error) { return excelProfile(a.Req("path"), a.Str("sheet", "")) }),
		NewTool("excel_read",
			"Read a block of cells from a spreadsheet file (.xlsx .xlsm .xls .csv .tsv). cell_range like 'A1:C50'. Page large areas with offset/limit. show_formula=True returns formulas instead of cached values (xlsx/xlsm only).",
			[]Param{ParamStr("path", "", true, nil), ParamStr("cell_range", "", false, nil), ParamStr("sheet", "", false, nil), ParamInt("offset", "", false, 0), ParamInt("limit", "", false, 100), ParamBool("show_formula", "", false, false)},
			func(a *ToolArgs) (any, error) {
				return excelRead(a.Req("path"), a.Str("sheet", ""), a.Str("cell_range", ""), a.Int("offset", 0), a.Int("limit", 100), a.Bool("show_formula", false))
			}),
		NewTool("excel_create_sheet",
			"Create a new worksheet in an existing workbook (.xlsx/.xlsm). Atomic save; other content (charts, images, pivots, macros) is preserved.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("sheet", "", true, nil), ParamBool("overwrite", "", false, false)},
			func(a *ToolArgs) (any, error) {
				return excelCreateSheet(a.Req("path"), a.Req("sheet"), a.Bool("overwrite", false))
			}),
		NewTool("excel_copy_sheet",
			"Copy a worksheet inside the same workbook (values, styles, merged cells, formatting; charts/images/tables are not copied). Atomic save.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("src_sheet", "", true, nil), ParamStr("dst_sheet", "", true, nil)},
			func(a *ToolArgs) (any, error) {
				return excelCopySheet(a.Req("path"), a.Req("src_sheet"), a.Req("dst_sheet"))
			}),
		NewTool("excel_rename_sheet",
			"Rename a worksheet (defined names follow the new name). Atomic save.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("sheet", "", true, nil), ParamStr("new_name", "", true, nil)},
			func(a *ToolArgs) (any, error) {
				return excelRenameSheet(a.Req("path"), a.Req("sheet"), a.Req("new_name"))
			}),
		NewTool("excel_delete_sheet",
			"Delete a worksheet (cannot delete the only sheet in the workbook). Atomic save.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("sheet", "", true, nil)},
			func(a *ToolArgs) (any, error) { return excelDeleteSheet(a.Req("path"), a.Req("sheet")) }),
		NewTool("excel_format_range",
			"Format cells in a range (.xlsx/.xlsm). styles = one style object applied to every cell OR a 2D array matching the range size (null entries skip that cell). "+
				"Style object keys: font {bold, italic, underline, size, strike, color '#RRGGBB', name, vertAlign}, fill {pattern 'solid', color '#RRGGBB'}, "+
				"border [{type: left|right|top|bottom|diagonalUp|diagonalDown, style: thin|medium|thick|double|dashed|dotted|hair|mediumDashed|dashDot|... , color}], "+
				"alignment {horizontal, vertical, wrap, rotation}, numFmt (number format string), decimalPlaces (0-30). Atomic save.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("sheet", "", true, nil), ParamStr("cell_range", "", true, nil), ParamAny("styles", "style object or 2D array of style objects", true, "object", "array")},
			func(a *ToolArgs) (any, error) {
				return excelFormatRange(a.Req("path"), a.Req("sheet"), a.Req("cell_range"), a.Raw("styles"))
			}),
		NewTool("excel_create_table",
			"Create an Excel table (ListObject) over a range with a header row, e.g. cell_range 'A1:D10'. table_name: letters/digits/underscore. Atomic save.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("sheet", "", true, nil), ParamStr("cell_range", "", true, nil), ParamStr("table_name", "", true, nil)},
			func(a *ToolArgs) (any, error) {
				return excelCreateTable(a.Req("path"), a.Req("sheet"), a.Req("cell_range"), a.Req("table_name"))
			}),
		NewTool("excel_write",
			"Write a 2D block of values into a spreadsheet file at start_cell (e.g. 'B2'), preserving the rest of the file (charts, images, pivots, macros kept). "+
				"Supports .xlsx .xlsm .csv .tsv; not .xls. Strings starting with '=' become formulas. Creates the file/sheet if missing. Saves atomically. "+
				"For files currently open in WPS/Office use wps_live_write_range.",
			[]Param{ParamStr("path", "", true, nil), ParamStr("sheet", "", true, nil), ParamStr("start_cell", "", true, nil), ParamMatrix("values", "", true)},
			func(a *ToolArgs) (any, error) {
				return excelWrite(a.Req("path"), a.Req("sheet"), a.Req("start_cell"), a.Matrix("values"))
			}),
		NewTool("excel_create",
			"Create a new spreadsheet file (.xlsx .xlsm .csv .tsv). sheets example: [{\"name\": \"Data\", \"values\": [[\"Ten\", \"Diem\"], [\"An\", 9.5]]}]. Saves atomically.",
			[]Param{ParamStr("path", "", true, nil), ParamArr("sheets", "", true, nil)},
			func(a *ToolArgs) (any, error) { return excelCreateWorkbook(a.Req("path"), a.Arr("sheets")) }),
		NewTool("excel_convert",
			"Convert one sheet of a spreadsheet file (.xlsx .xlsm .xls .csv .tsv) to csv next to the source file (first row = header).",
			[]Param{ParamStr("path", "", true, nil), ParamStr("sheet", "", false, nil), ParamStr("to", "csv", false, "csv")},
			func(a *ToolArgs) (any, error) {
				return excelConvert(a.Req("path"), a.Str("sheet", ""), a.Str("to", "csv"))
			}),
	}
}

func excelExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

func excelCheckFormat(path string) error {
	ext := excelExt(path)
	if !containsString(workbookExtensions, ext) && ext != ".xls" && !sheet.IsCsvExtension(ext) {
		return fmt.Errorf("unsupported format '%s': supported are xlsx, xlsm, xls, csv, tsv", ext)
	}
	return nil
}

// excelKind doan dinh dang that theo magic bytes (PK = xlsx, D0CF11E0 = xls), khong thi theo duoi file.
func excelKind(path string) string {
	if filesafe.Exists(path) {
		head := make([]byte, 4)
		if handle, err := os.Open(path); err == nil {
			read, _ := io.ReadFull(handle, head)
			handle.Close()
			if read >= 2 && head[0] == 'P' && head[1] == 'K' {
				return "xlsx"
			}
			if read == 4 && head[0] == 0xD0 && head[1] == 0xCF && head[2] == 0x11 && head[3] == 0xE0 {
				return "xls"
			}
		}
	}
	ext := excelExt(path)
	if containsString(workbookExtensions, ext) {
		return "xlsx"
	}
	if ext == ".xls" {
		return "xls"
	}
	return "csv"
}

// csvSheetName la ten "sheet" cua file csv: ten file bo duoi.
func csvSheetName(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func findSheetRef(book *xlsx.Book, name string) *xlsx.SheetRef {
	for _, item := range book.Sheets {
		if item.Name == name {
			return item
		}
	}
	return nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// ---- doc ----

func excelProfile(path, target string) (any, error) {
	if err := filesafe.RequireFile(path); err != nil {
		return nil, err
	}
	if err := excelCheckFormat(path); err != nil {
		return nil, err
	}
	kind := excelKind(path)
	sheets := []any{}
	switch kind {
	case "csv":
		if target != "" && target != csvSheetName(path) {
			return nil, fmt.Errorf("csv files have a single sheet")
		}
		rows, err := sheet.ReadCSV(path)
		if err != nil {
			return nil, err
		}
		header := []any{}
		if len(rows) > 0 {
			header = rows[0]
		}
		sample := []any{}
		for index := 1; index < len(rows) && index < 6; index++ {
			sample = append(sample, rows[index])
		}
		sheets = append(sheets, map[string]any{
			"name":    csvSheetName(path),
			"rows":    len(rows),
			"columns": widestRow(rows),
			"header":  header,
			"sample":  sample,
		})
	case "xls":
		grids, err := xlsx.ReadXLS(path, target, target == "")
		if err != nil {
			return nil, err
		}
		for _, grid := range grids {
			sheets = append(sheets, sheetProfile(grid))
		}
	default:
		pkg, err := ooxml.OpenRead(path)
		if err != nil {
			return nil, err
		}
		book, err := xlsx.Open(pkg)
		if err != nil {
			return nil, err
		}
		names := book.SheetNames()
		if target != "" {
			names = []string{target}
		}
		for _, name := range names {
			reference := findSheetRef(book, name)
			if reference == nil {
				return nil, fmt.Errorf("sheet not found: %s", name)
			}
			if reference.Part == "" || !pkg.Exists(reference.Part) {
				continue // chart sheet
			}
			grid, err := book.Read(reference, false, 1, 6, 1, math.MaxInt32)
			if err != nil {
				return nil, err
			}
			sheets = append(sheets, sheetProfile(grid))
		}
	}
	return map[string]any{"path": path, "format": kind, "sheets": sheets}, nil
}

func widestRow(rows [][]any) int {
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	return width
}

func sheetProfile(grid *xlsx.Grid) map[string]any {
	header := []any{}
	sample := []any{}
	if grid.MaxRow > 0 && grid.MaxCol > 0 && grid.HasCells {
		header = grid.Row(1, 1, grid.MaxCol)
		for row := 2; row <= min(grid.MaxRow, 6); row++ {
			sample = append(sample, grid.Row(row, 1, grid.MaxCol))
		}
	}
	return map[string]any{
		"name":    grid.Name,
		"rows":    grid.MaxRow,
		"columns": grid.MaxCol,
		"header":  header,
		"sample":  sample,
	}
}

func excelRead(path, target, cellRange string, offset, limit int, showFormula bool) (any, error) {
	if err := filesafe.RequireFile(path); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 1
	}
	if err := excelCheckFormat(path); err != nil {
		return nil, err
	}
	minCol, minRow := 1, 1
	var maxCol, maxRow *int
	if cellRange != "" {
		bounds, err := sheet.ParseRange(cellRange)
		if err != nil {
			return nil, err
		}
		if bounds.MinCol != nil {
			minCol = *bounds.MinCol
		}
		if bounds.MinRow != nil {
			minRow = *bounds.MinRow
		}
		maxCol, maxRow = bounds.MaxCol, bounds.MaxRow
	}
	start := minRow + max(offset, 0)
	kind := excelKind(path)

	var grid *xlsx.Grid
	switch kind {
	case "csv":
		if target != "" && target != csvSheetName(path) {
			return nil, fmt.Errorf("csv files have a single sheet")
		}
		rows, err := sheet.ReadCSV(path)
		if err != nil {
			return nil, err
		}
		grid = xlsx.NewGrid(csvSheetName(path))
		for rowIndex, row := range rows {
			for colIndex, value := range row {
				grid.Set(rowIndex+1, colIndex+1, value)
			}
		}
		grid.MaxRow = len(rows)
		grid.MaxCol = max(1, widestRow(rows))
	case "xls":
		grids, err := xlsx.ReadXLS(path, target, false)
		if err != nil {
			return nil, err
		}
		if len(grids) == 0 {
			return nil, fmt.Errorf("sheet not found: %s", target)
		}
		grid = grids[0]
	default:
		pkg, err := ooxml.OpenRead(path)
		if err != nil {
			return nil, err
		}
		book, err := xlsx.Open(pkg)
		if err != nil {
			return nil, err
		}
		var reference *xlsx.SheetRef
		if target != "" {
			reference, err = book.FindSheet(target)
		} else {
			reference, err = book.ActiveSheet()
		}
		if err != nil {
			return nil, err
		}
		rowTo := start + limit - 1
		if maxRow != nil {
			rowTo = min(*maxRow, start+limit-1)
		}
		colTo := math.MaxInt32
		if maxCol != nil {
			colTo = *maxCol
		}
		grid, err = book.Read(reference, showFormula, start, rowTo, minCol, colTo)
		if err != nil {
			return nil, err
		}
	}

	limitMaxRow := max(1, grid.MaxRow)
	if maxRow != nil {
		limitMaxRow = *maxRow
	}
	limitMaxCol := max(1, grid.MaxCol)
	if maxCol != nil {
		limitMaxCol = *maxCol
	}
	end := min(limitMaxRow, start+limit-1)
	colEnd := limitMaxCol
	if kind == "xls" {
		// Nhu xlrd: khong tra dong/cot vuot kich thuoc sheet.
		end = min(end, grid.MaxRow)
		colEnd = min(limitMaxCol, grid.MaxCol)
	}
	values := []any{}
	for row := max(1, start); row <= end; row++ {
		values = append(values, grid.Row(row, minCol, colEnd))
	}
	rangeText := cellRange
	if rangeText == "" {
		rangeText = sheet.Address(minRow, minCol) + ":" + sheet.Address(limitMaxRow, limitMaxCol)
	}
	totalRows := limitMaxRow - minRow + 1
	if kind == "csv" {
		totalRows = grid.MaxRow
	}
	return map[string]any{
		"path":       path,
		"sheet":      grid.Name,
		"range":      rangeText,
		"offset":     offset,
		"returned":   len(values),
		"total_rows": totalRows,
		"values":     values,
	}, nil
}

// excelAllRows doc toan bo sheet thanh bang gia tri (bo qua kich thuoc neu la csv).
func excelAllRows(path, target string) ([][]any, error) {
	if err := filesafe.RequireFile(path); err != nil {
		return nil, err
	}
	if err := excelCheckFormat(path); err != nil {
		return nil, err
	}
	rows := [][]any{}
	switch kind := excelKind(path); kind {
	case "csv":
		return sheet.ReadCSV(path)
	case "xls":
		grids, err := xlsx.ReadXLS(path, target, false)
		if err != nil {
			return nil, err
		}
		if len(grids) == 0 {
			return nil, fmt.Errorf("sheet not found: %s", target)
		}
		for row := 1; row <= grids[0].MaxRow; row++ {
			rows = append(rows, grids[0].Row(row, 1, grids[0].MaxCol))
		}
	default:
		pkg, err := ooxml.OpenRead(path)
		if err != nil {
			return nil, err
		}
		book, err := xlsx.Open(pkg)
		if err != nil {
			return nil, err
		}
		var reference *xlsx.SheetRef
		if target != "" {
			reference, err = book.FindSheet(target)
		} else {
			reference, err = book.ActiveSheet()
		}
		if err != nil {
			return nil, err
		}
		grid, err := book.Read(reference, false, 1, math.MaxInt32, 1, math.MaxInt32)
		if err != nil {
			return nil, err
		}
		for row := 1; row <= grid.MaxRow; row++ {
			rows = append(rows, grid.Row(row, 1, grid.MaxCol))
		}
	}
	return rows, nil
}

// ---- ghi ----

func excelWrite(path, target, startCell string, values [][]any) (any, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("values must not be empty")
	}
	if err := excelCheckFormat(path); err != nil {
		return nil, err
	}
	row0, col0, err := sheet.ParseCell(startCell)
	if err != nil {
		return nil, err
	}
	kind := "xlsx"
	switch {
	case filesafe.Exists(path):
		kind = excelKind(path)
	case excelExt(path) == ".xls":
		kind = "xls"
	case sheet.IsCsvExtension(excelExt(path)):
		kind = "csv"
	}
	if kind == "xls" {
		return nil, fmt.Errorf("writing .xls (BIFF) is not supported - save as xlsx or use the WPS live bridge")
	}
	written := 0
	if kind == "csv" {
		rows := [][]any{}
		if filesafe.Exists(path) {
			rows, err = sheet.ReadCSV(path)
			if err != nil {
				return nil, err
			}
		}
		for len(rows) < row0-1+len(values) {
			rows = append(rows, []any{})
		}
		for r := 0; r < len(values); r++ {
			targetRow := rows[row0-1+r]
			for len(targetRow) < col0-1+len(values[r]) {
				targetRow = append(targetRow, "")
			}
			for c := 0; c < len(values[r]); c++ {
				value := values[r][c]
				if value == nil {
					value = ""
				}
				targetRow[col0-1+c] = value
				written++
			}
			rows[row0-1+r] = targetRow
		}
		if err := sheet.WriteCSV(path, rows); err != nil {
			return nil, err
		}
		return map[string]any{"saved": path, "sheet": csvSheetName(path), "start_cell": startCell, "written": written}, nil
	}

	edit := func(pkg *ooxml.Package) error {
		book, err := xlsx.Open(pkg)
		if err != nil {
			return err
		}
		reference := findSheetRef(book, target)
		if reference == nil {
			reference, err = book.AddSheet(target, nil)
			if err != nil {
				return err
			}
		}
		doc, err := book.SheetDoc(reference)
		if err != nil {
			return err
		}
		data := book.SheetData(doc)
		for r := 0; r < len(values); r++ {
			for c := 0; c < len(values[r]); c++ {
				if err := book.SetValue(data, row0+r, col0+c, values[r][c]); err != nil {
					return err
				}
				written++
			}
		}
		book.UpdateDimension(doc)
		book.SaveSheet(reference, doc)
		return book.InvalidateCalculation()
	}
	if filesafe.Exists(path) {
		err = ooxml.Edit(path, edit)
	} else {
		if err := xlsx.ValidateSheetName(target); err != nil {
			return nil, err
		}
		template, buildErr := xlsx.BuildTemplate(excelExt(path), target)
		if buildErr != nil {
			return nil, buildErr
		}
		err = ooxml.CreateFromTemplate(path, template, edit)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"saved": path, "sheet": target, "start_cell": startCell, "written": written}, nil
}

func excelCreateWorkbook(path string, sheetsArg []any) (any, error) {
	if len(sheetsArg) == 0 {
		return nil, fmt.Errorf("sheets must not be empty")
	}
	if err := excelCheckFormat(path); err != nil {
		return nil, err
	}
	ext := excelExt(path)
	if ext == ".xls" {
		return nil, fmt.Errorf("creating .xls is not supported - use xlsx")
	}
	specs := make([]map[string]any, 0, len(sheetsArg))
	for _, item := range sheetsArg {
		spec, ok := item.(map[string]any)
		if !ok {
			spec = map[string]any{}
		}
		specs = append(specs, spec)
	}
	if sheet.IsCsvExtension(ext) {
		if len(specs) > 1 {
			return nil, fmt.Errorf("csv supports a single sheet")
		}
		rows := specMatrix(specs[0], "values")
		filled := make([][]any, 0, len(rows))
		for _, row := range rows {
			values := make([]any, 0, len(row))
			for _, value := range row {
				if value == nil {
					value = ""
				}
				values = append(values, value)
			}
			filled = append(filled, values)
		}
		if err := sheet.WriteCSV(path, filled); err != nil {
			return nil, err
		}
		return map[string]any{"created": path, "sheets": []string{csvSheetName(path)}}, nil
	}

	names := make([]string, 0, len(specs))
	for index, spec := range specs {
		name := "Sheet" + strconv.Itoa(index+1)
		if value, exists := spec["name"]; exists && value != nil {
			name = sheet.ToText(value)
		}
		names = append(names, name)
	}
	for _, name := range names {
		if err := xlsx.ValidateSheetName(name); err != nil {
			return nil, err
		}
	}
	template, err := xlsx.BuildTemplate(ext, names[0])
	if err != nil {
		return nil, err
	}
	err = ooxml.CreateFromTemplate(path, template, func(pkg *ooxml.Package) error {
		book, err := xlsx.Open(pkg)
		if err != nil {
			return err
		}
		for index, spec := range specs {
			var reference *xlsx.SheetRef
			if index == 0 {
				reference, err = book.FindSheet(names[0])
			} else {
				reference, err = book.AddSheet(names[index], nil)
			}
			if err != nil {
				return err
			}
			doc, err := book.SheetDoc(reference)
			if err != nil {
				return err
			}
			data := book.SheetData(doc)
			rows := specMatrix(spec, "values")
			for r := 0; r < len(rows); r++ {
				for c := 0; c < len(rows[r]); c++ {
					if err := book.SetValue(data, r+1, c+1, rows[r][c]); err != nil {
						return err
					}
				}
			}
			book.UpdateDimension(doc)
			book.SaveSheet(reference, doc)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"created": path, "sheets": names}, nil
}

// specMatrix doc mang 2 chieu tu mot muc trong spec cua excel_create.
func specMatrix(spec map[string]any, key string) [][]any {
	raw, ok := spec[key].([]any)
	if !ok {
		return [][]any{}
	}
	rows := make([][]any, 0, len(raw))
	for _, item := range raw {
		if cells, ok := item.([]any); ok {
			rows = append(rows, cells)
			continue
		}
		rows = append(rows, []any{item})
	}
	return rows
}

func excelConvert(path, target, to string) (any, error) {
	to = strings.ToLower(strings.TrimSpace(to))
	if to == "" {
		to = "csv"
	}
	if to == "parquet" {
		return nil, fmt.Errorf("parquet output is not available in this server - use to='csv'")
	}
	if to != "csv" {
		return nil, fmt.Errorf("to must be 'csv'")
	}
	rows, err := excelAllRows(path, target)
	if err != nil {
		return nil, err
	}
	header := []any{}
	if len(rows) > 0 {
		header = rows[0]
	}
	columns := uniqueColumns(header)
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	output := filepath.Join(filepath.Dir(absolute), csvSheetName(path)+".csv")
	if strings.EqualFold(output, absolute) {
		return nil, fmt.Errorf("source is already a csv file")
	}
	data := [][]any{stringsToAny(columns)}
	for _, row := range rows[1:] {
		padded := make([]any, 0, len(columns))
		for index := 0; index < len(columns); index++ {
			if index < len(row) {
				padded = append(padded, row[index])
			} else {
				padded = append(padded, nil)
			}
		}
		data = append(data, padded)
	}
	if err := sheet.WriteCSV(output, data); err != nil {
		return nil, err
	}
	return map[string]any{
		"output":  output,
		"rows":    max(0, len(rows)-1),
		"columns": columns,
		"format":  "csv",
	}, nil
}

func stringsToAny(values []string) []any {
	converted := make([]any, 0, len(values))
	for _, value := range values {
		converted = append(converted, value)
	}
	return converted
}

// uniqueColumns dat ten cot tu dong cho tieu de trong hoac trung nhau.
func uniqueColumns(header []any) []string {
	seen := map[string]int{}
	columns := []string{}
	for index, value := range header {
		name := ""
		if value != nil {
			name = strings.TrimSpace(sheet.ToText(value))
		}
		if name == "" {
			name = "col" + strconv.Itoa(index+1)
		}
		if count, exists := seen[name]; exists {
			seen[name] = count + 1
			name = name + "_" + strconv.Itoa(count+1)
		} else {
			seen[name] = 1
		}
		columns = append(columns, name)
	}
	return columns
}

// ---- thao tac sheet / style / table ----

// excelEditWorkbook mo workbook de sua (chi nhan .xlsx/.xlsm, khong nhan csv/xls).
func excelEditWorkbook(path string, edit func(*xlsx.Book) error) error {
	if err := filesafe.RequireFile(path); err != nil {
		return err
	}
	if err := excelCheckFormat(path); err != nil {
		return err
	}
	switch kind := excelKind(path); kind {
	case "xls":
		return fmt.Errorf("editing .xls (BIFF) is not supported - save as xlsx or use the WPS live bridge")
	case "csv":
		return fmt.Errorf("this operation requires a workbook format (xlsx/xlsm), not csv/tsv")
	}
	return ooxml.Edit(path, func(pkg *ooxml.Package) error {
		book, err := xlsx.Open(pkg)
		if err != nil {
			return err
		}
		return edit(book)
	})
}

func excelCreateSheet(path, name string, overwrite bool) (any, error) {
	var names []string
	err := excelEditWorkbook(path, func(book *xlsx.Book) error {
		existing := findSheetRef(book, name)
		if existing != nil {
			if !overwrite {
				return fmt.Errorf("sheet already exists: %s", name)
			}
			if len(book.Sheets) <= 1 {
				// Khong duoc xoa sheet duy nhat: them sheet moi truoc roi xoa sheet cu.
				fresh, err := book.AddSheet(name+"~new", nil)
				if err != nil {
					return err
				}
				old, err := book.FindSheet(name)
				if err != nil {
					return err
				}
				if err := book.DeleteSheet(old); err != nil {
					return err
				}
				if err := book.RenameSheet(fresh, name); err != nil {
					return err
				}
				names = book.SheetNames()
				return nil
			}
			if err := book.DeleteSheet(existing); err != nil {
				return err
			}
		}
		if _, err := book.AddSheet(name, nil); err != nil {
			return err
		}
		names = book.SheetNames()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "sheet": name, "sheets": names}, nil
}

func excelCopySheet(path, source, target string) (any, error) {
	var names []string
	err := excelEditWorkbook(path, func(book *xlsx.Book) error {
		src, err := book.FindSheet(source)
		if err != nil {
			return err
		}
		if book.HasSheet(target) {
			return fmt.Errorf("sheet already exists: %s", target)
		}
		if _, err := book.CopySheet(src, target); err != nil {
			return err
		}
		names = book.SheetNames()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "src": source, "dst": target, "sheets": names}, nil
}

func excelRenameSheet(path, name, newName string) (any, error) {
	var names []string
	err := excelEditWorkbook(path, func(book *xlsx.Book) error {
		reference := findSheetRef(book, name)
		if reference == nil {
			return fmt.Errorf("sheet not found: %s", name)
		}
		if err := book.RenameSheet(reference, newName); err != nil {
			return err
		}
		names = book.SheetNames()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "old": name, "new": newName, "sheets": names}, nil
}

func excelDeleteSheet(path, name string) (any, error) {
	var names []string
	err := excelEditWorkbook(path, func(book *xlsx.Book) error {
		reference := findSheetRef(book, name)
		if reference == nil {
			return fmt.Errorf("sheet not found: %s", name)
		}
		if len(book.Sheets) <= 1 {
			return fmt.Errorf("cannot delete the only sheet in the workbook")
		}
		if err := book.DeleteSheet(reference); err != nil {
			return err
		}
		names = book.SheetNames()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "deleted": name, "sheets": names}, nil
}

func excelFormatRange(path, name, cellRange string, styles any) (any, error) {
	bounds, err := sheet.ParseBoundedRange(cellRange)
	if err != nil {
		return nil, err
	}
	rowCount := bounds.MaxRow - bounds.MinRow + 1
	colCount := bounds.MaxCol - bounds.MinCol + 1
	single, isSingle := styles.(map[string]any)
	matrix, isMatrix := styles.([]any)
	if !isSingle && !isMatrix {
		return nil, fmt.Errorf("styles must be an object or a 2D array")
	}
	if isMatrix {
		matches := len(matrix) == rowCount
		if matches {
			for _, row := range matrix {
				cells, ok := row.([]any)
				if !ok || len(cells) != colCount {
					matches = false
					break
				}
			}
		}
		if !matches {
			return nil, fmt.Errorf("styles matrix size must match the range size")
		}
	}

	styled := 0
	err = excelEditWorkbook(path, func(book *xlsx.Book) error {
		reference, err := book.FindSheet(name)
		if err != nil {
			return err
		}
		doc, err := book.SheetDoc(reference)
		if err != nil {
			return err
		}
		data := book.SheetData(doc)
		styleSheet, err := book.Styles()
		if err != nil {
			return err
		}
		for r := 0; r < rowCount; r++ {
			for c := 0; c < colCount; c++ {
				var spec map[string]any
				if isSingle {
					spec = single
				} else {
					spec, _ = matrix[r].([]any)[c].(map[string]any)
				}
				if spec == nil {
					continue
				}
				cell := book.GetOrCreateCell(data, bounds.MinRow+r, bounds.MinCol+c)
				current, _ := strconv.Atoi(cell.AttrValue("", "s"))
				index, err := styleSheet.Derive(current, spec)
				if err != nil {
					return err
				}
				cell.SetAttr("", "", "s", strconv.Itoa(index))
				styled++
			}
		}
		book.UpdateDimension(doc)
		book.SaveSheet(reference, doc)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "sheet": name, "range": cellRange, "styled_cells": styled}, nil
}

var (
	tableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	tableCellPattern = regexp.MustCompile(`^[A-Za-z]{1,3}[0-9]+$`)
	tableRCPattern   = regexp.MustCompile(`^[RrCc][0-9]*$`)
)

func excelCreateTable(path, name, cellRange, tableName string) (any, error) {
	if !tableNamePattern.MatchString(tableName) {
		return nil, fmt.Errorf("table_name must start with a letter/underscore and contain only letters, digits, underscores")
	}
	if tableCellPattern.MatchString(tableName) || tableRCPattern.MatchString(tableName) {
		return nil, fmt.Errorf("table_name must not look like a cell reference (e.g. 'A1', 'R1C1')")
	}
	var tables []string
	err := excelEditWorkbook(path, func(book *xlsx.Book) error {
		reference, err := book.FindSheet(name)
		if err != nil {
			return err
		}
		for _, existing := range book.AllTableNames() {
			if strings.EqualFold(existing, tableName) {
				return fmt.Errorf("table already exists: %s", tableName)
			}
		}
		if err := book.AddTable(reference, cellRange, tableName); err != nil {
			return err
		}
		tables = book.SheetTableNames(reference)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "sheet": name, "range": cellRange, "table": tableName, "tables": tables}, nil
}
