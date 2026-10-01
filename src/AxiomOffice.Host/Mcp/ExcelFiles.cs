using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;   // chi cho COM .xls cua ban Windows
using System.Text.RegularExpressions;

namespace AxiomOffice.Host.Mcp
{
    // Làn file cho bảng tính (thay openpyxl/xlrd/pandas của excel-mcp).
    // .xlsx/.xlsm: đọc/sửa trực tiếp OOXML. .csv/.tsv: đọc/ghi text. .xls: chỉ đọc, qua Excel/WPS (COM).
    internal static class ExcelFiles
    {
        private static readonly string[] WorkbookExtensions = { ".xlsx", ".xlsm", ".xltx", ".xltm" };

        public static IEnumerable<McpTool> Tools()
        {
            yield return new McpTool("excel_profile",
                "Inspect a spreadsheet file (.xlsx .xlsm .xls .csv .tsv): sheet names, row/column counts, header row and a small sample per sheet.",
                new[] { Param.Str("path", null, true), Param.Str("sheet") },
                a => Profile(a.Req("path"), a.Str("sheet")));
            yield return new McpTool("excel_read",
                "Read a block of cells from a spreadsheet file (.xlsx .xlsm .xls .csv .tsv). cell_range like 'A1:C50'. Page large areas with offset/limit. show_formula=True returns formulas instead of cached values (xlsx/xlsm only).",
                new[] { Param.Str("path", null, true), Param.Str("cell_range"), Param.Str("sheet"), Param.Int("offset", null, false, 0), Param.Int("limit", null, false, 100), Param.Bool("show_formula", null, false, false) },
                a => Read(a.Req("path"), a.Str("sheet"), a.Str("cell_range"), a.Int("offset", 0), a.Int("limit", 100), a.Bool("show_formula", false)));
            yield return new McpTool("excel_create_sheet",
                "Create a new worksheet in an existing workbook (.xlsx/.xlsm). Atomic save; other content (charts, images, pivots, macros) is preserved.",
                new[] { Param.Str("path", null, true), Param.Str("sheet", null, true), Param.Bool("overwrite", null, false, false) },
                a => CreateSheet(a.Req("path"), a.Req("sheet"), a.Bool("overwrite", false)));
            yield return new McpTool("excel_copy_sheet",
                "Copy a worksheet inside the same workbook (values, styles, merged cells, formatting; charts/images/tables are not copied). Atomic save.",
                new[] { Param.Str("path", null, true), Param.Str("src_sheet", null, true), Param.Str("dst_sheet", null, true) },
                a => CopySheet(a.Req("path"), a.Req("src_sheet"), a.Req("dst_sheet")));
            yield return new McpTool("excel_rename_sheet",
                "Rename a worksheet (defined names follow the new name). Atomic save.",
                new[] { Param.Str("path", null, true), Param.Str("sheet", null, true), Param.Str("new_name", null, true) },
                a => RenameSheet(a.Req("path"), a.Req("sheet"), a.Req("new_name")));
            yield return new McpTool("excel_delete_sheet",
                "Delete a worksheet (cannot delete the only sheet in the workbook). Atomic save.",
                new[] { Param.Str("path", null, true), Param.Str("sheet", null, true) },
                a => DeleteSheet(a.Req("path"), a.Req("sheet")));
            yield return new McpTool("excel_format_range",
                "Format cells in a range (.xlsx/.xlsm). styles = one style object applied to every cell OR a 2D array matching the range size (null entries skip that cell). " +
                "Style object keys: font {bold, italic, underline, size, strike, color '#RRGGBB', name, vertAlign}, fill {pattern 'solid', color '#RRGGBB'}, " +
                "border [{type: left|right|top|bottom|diagonalUp|diagonalDown, style: thin|medium|thick|double|dashed|dotted|hair|mediumDashed|dashDot|... , color}], " +
                "alignment {horizontal, vertical, wrap, rotation}, numFmt (number format string), decimalPlaces (0-30). Atomic save.",
                new[] { Param.Str("path", null, true), Param.Str("sheet", null, true), Param.Str("cell_range", null, true), Param.Any("styles", "style object or 2D array of style objects", true, "object", "array") },
                a => FormatRange(a.Req("path"), a.Req("sheet"), a.Req("cell_range"), a.Raw("styles")));
            yield return new McpTool("excel_create_table",
                "Create an Excel table (ListObject) over a range with a header row, e.g. cell_range 'A1:D10'. table_name: letters/digits/underscore. Atomic save.",
                new[] { Param.Str("path", null, true), Param.Str("sheet", null, true), Param.Str("cell_range", null, true), Param.Str("table_name", null, true) },
                a => CreateTable(a.Req("path"), a.Req("sheet"), a.Req("cell_range"), a.Req("table_name")));
            yield return new McpTool("excel_write",
                "Write a 2D block of values into a spreadsheet file at start_cell (e.g. 'B2'), preserving the rest of the file (charts, images, pivots, macros kept). " +
                "Supports .xlsx .xlsm .csv .tsv; not .xls. Strings starting with '=' become formulas. Creates the file/sheet if missing. Saves atomically (temp + replace). " +
                "For files currently open in WPS/Office use wps_live_write_range.",
                new[] { Param.Str("path", null, true), Param.Str("sheet", null, true), Param.Str("start_cell", null, true), Param.Matrix("values", null, true) },
                a => Write(a.Req("path"), a.Req("sheet"), a.Req("start_cell"), a.Matrix("values")));
            yield return new McpTool("excel_create",
                "Create a new spreadsheet file (.xlsx .xlsm .csv .tsv). sheets example: [{\"name\": \"Data\", \"values\": [[\"Ten\", \"Diem\"], [\"An\", 9.5]]}]. Saves atomically.",
                new[] { Param.Str("path", null, true), Param.Arr("sheets", null, true) },
                a => CreateWorkbook(a.Req("path"), a.Arr("sheets")));
            yield return new McpTool("excel_convert",
                "Convert one sheet of a spreadsheet file (.xlsx .xlsm .xls .csv .tsv) to csv next to the source file (first row = header).",
                new[] { Param.Str("path", null, true), Param.Str("sheet"), Param.Str("to", "csv", false, "csv") },
                a => Convert(a.Req("path"), a.Str("sheet"), a.Str("to", "csv")));
        }

        // ---- nhận dạng định dạng ----

        private static string Ext(string path)
        {
            return Path.GetExtension(path).ToLowerInvariant();
        }

        private static void CheckFormat(string path)
        {
            string ext = Ext(path);
            if (!WorkbookExtensions.Contains(ext) && ext != ".xls" && !CsvTable.IsCsvExtension(ext))
            {
                throw new ArgumentException("unsupported format '" + ext + "': supported are xlsx, xlsm, xls, csv, tsv");
            }
        }

        // Theo magic bytes như bản Python: PK = xlsx, D0CF11E0 = xls, còn lại theo đuôi file.
        private static string Kind(string path)
        {
            if (File.Exists(path))
            {
                var head = new byte[4];
                using (var stream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite))
                {
                    int read = stream.Read(head, 0, 4);
                    if (read >= 2 && head[0] == 'P' && head[1] == 'K')
                    {
                        return "xlsx";
                    }
                    if (read == 4 && head[0] == 0xD0 && head[1] == 0xCF && head[2] == 0x11 && head[3] == 0xE0)
                    {
                        return "xls";
                    }
                }
            }
            string ext = Ext(path);
            if (WorkbookExtensions.Contains(ext))
            {
                return "xlsx";
            }
            return ext == ".xls" ? "xls" : "csv";
        }

        private static string CsvSheetName(string path)
        {
            return Path.GetFileNameWithoutExtension(path);
        }

        // ---- đọc ----

        public static Dictionary<string, object> Profile(string path, string sheet)
        {
            FileSafety.RequireFile(path);
            CheckFormat(path);
            string kind = Kind(path);
            var sheets = new List<object>();
            if (kind == "csv")
            {
                if (sheet != null && sheet != CsvSheetName(path))
                {
                    throw new ArgumentException("csv files have a single sheet");
                }
                List<List<object>> rows = CsvTable.Read(path);
                sheets.Add(new Dictionary<string, object>
                {
                    { "name", CsvSheetName(path) },
                    { "rows", rows.Count },
                    { "columns", rows.Count == 0 ? 0 : rows.Max(r => r.Count) },
                    { "header", rows.Count > 0 ? rows[0] : new List<object>() },
                    { "sample", rows.Skip(1).Take(5).ToList() }
                });
            }
            else if (kind == "xls")
            {
                foreach (SheetGrid grid in XlsAutomation.Read(path, sheet, sheet == null))
                {
                    sheets.Add(SheetProfile(grid));
                }
            }
            else
            {
                using (OoxmlPackage package = OoxmlPackage.OpenRead(path))
                {
                    var book = new XlsxBook(package);
                    foreach (string name in sheet != null ? new List<string> { sheet } : book.SheetNames)
                    {
                        SheetRef reference = book.Sheets.FirstOrDefault(s => s.Name == name);
                        if (reference == null)
                        {
                            throw new ArgumentException("sheet not found: " + name);
                        }
                        if (reference.Part == null || !package.Exists(reference.Part))
                        {
                            continue; // chart sheet
                        }
                        sheets.Add(SheetProfile(book.Read(reference, false, 1, 6)));
                    }
                }
            }
            return new Dictionary<string, object> { { "path", path }, { "format", kind }, { "sheets", sheets } };
        }

        private static Dictionary<string, object> SheetProfile(SheetGrid grid)
        {
            var header = new List<object>();
            var sample = new List<object>();
            if (grid.MaxRow > 0 && grid.MaxCol > 0 && grid.HasCells)
            {
                header = grid.Row(1, 1, grid.MaxCol);
                for (int r = 2; r <= Math.Min(grid.MaxRow, 6); r++)
                {
                    sample.Add(grid.Row(r, 1, grid.MaxCol));
                }
            }
            return new Dictionary<string, object>
            {
                { "name", grid.Name },
                { "rows", grid.MaxRow },
                { "columns", grid.MaxCol },
                { "header", header },
                { "sample", sample }
            };
        }

        public static Dictionary<string, object> Read(string path, string sheet, string cellRange, int offset, int limit, bool showFormula)
        {
            FileSafety.RequireFile(path);
            if (limit <= 0)
            {
                limit = 1;
            }
            CheckFormat(path);
            string kind = Kind(path);
            int? minColOpt = 1, minRowOpt = 1, maxColOpt = null, maxRowOpt = null;
            if (!string.IsNullOrEmpty(cellRange))
            {
                Cells.ParseRange(cellRange, out minColOpt, out minRowOpt, out maxColOpt, out maxRowOpt);
            }
            int minCol = minColOpt ?? 1;
            int minRow = minRowOpt ?? 1;
            int start = minRow + Math.Max(offset, 0);

            SheetGrid grid;
            if (kind == "csv")
            {
                if (sheet != null && sheet != CsvSheetName(path))
                {
                    throw new ArgumentException("csv files have a single sheet");
                }
                grid = new SheetGrid { Name = CsvSheetName(path) };
                List<List<object>> rows = CsvTable.Read(path);
                for (int r = 0; r < rows.Count; r++)
                {
                    for (int c = 0; c < rows[r].Count; c++)
                    {
                        grid.Set(r + 1, c + 1, rows[r][c]);
                    }
                }
                grid.MaxRow = rows.Count;
                grid.MaxCol = Math.Max(1, rows.Count == 0 ? 0 : rows.Max(r => r.Count));
            }
            else if (kind == "xls")
            {
                grid = XlsAutomation.Read(path, sheet, false).First();
            }
            else
            {
                using (OoxmlPackage package = OoxmlPackage.OpenRead(path))
                {
                    var book = new XlsxBook(package);
                    SheetRef reference = sheet != null ? book.FindSheet(sheet) : book.ActiveSheet();
                    int rowTo = maxRowOpt.HasValue ? Math.Min(maxRowOpt.Value, start + limit - 1) : start + limit - 1;
                    grid = book.Read(reference, showFormula, start, rowTo, minCol, maxColOpt ?? int.MaxValue);
                }
            }

            int maxRow = maxRowOpt ?? Math.Max(1, grid.MaxRow);
            int maxCol = maxColOpt ?? Math.Max(1, grid.MaxCol);
            int end = Math.Min(maxRow, start + limit - 1);
            int colEnd = maxCol;
            if (kind == "xls")
            {
                // Như xlrd: không trả dòng/cột vượt kích thước sheet.
                end = Math.Min(end, grid.MaxRow);
                colEnd = Math.Min(maxCol, grid.MaxCol);
            }
            var values = new List<object>();
            for (int r = Math.Max(1, start); r <= end; r++)
            {
                values.Add(grid.Row(r, minCol, colEnd));
            }
            return new Dictionary<string, object>
            {
                { "path", path },
                { "sheet", grid.Name },
                { "range", string.IsNullOrEmpty(cellRange) ? Cells.Address(minRow, minCol) + ":" + Cells.Address(maxRow, maxCol) : cellRange },
                { "offset", offset },
                { "returned", values.Count },
                { "total_rows", kind == "csv" ? grid.MaxRow : maxRow - minRow + 1 },
                { "values", values }
            };
        }

        private static List<List<object>> AllRows(string path, string sheet)
        {
            FileSafety.RequireFile(path);
            CheckFormat(path);
            string kind = Kind(path);
            if (kind == "csv")
            {
                return CsvTable.Read(path);
            }
            SheetGrid grid;
            if (kind == "xls")
            {
                grid = XlsAutomation.Read(path, sheet, false).First();
            }
            else
            {
                using (OoxmlPackage package = OoxmlPackage.OpenRead(path))
                {
                    var book = new XlsxBook(package);
                    grid = book.Read(sheet != null ? book.FindSheet(sheet) : book.ActiveSheet(), false);
                }
            }
            var rows = new List<List<object>>();
            for (int r = 1; r <= grid.MaxRow; r++)
            {
                rows.Add(grid.Row(r, 1, grid.MaxCol));
            }
            return rows;
        }

        // ---- ghi ----

        public static Dictionary<string, object> Write(string path, string sheet, string startCell, object[][] values)
        {
            if (values == null || values.Length == 0)
            {
                throw new ArgumentException("values must not be empty");
            }
            CheckFormat(path);
            int row0, col0;
            Cells.ParseCell(startCell, out row0, out col0);
            string kind = File.Exists(path) ? Kind(path) : (Ext(path) == ".xls" ? "xls" : CsvTable.IsCsvExtension(Ext(path)) ? "csv" : "xlsx");
            if (kind == "xls")
            {
                throw new ArgumentException("writing .xls (BIFF) is not supported - save as xlsx or use the WPS live bridge");
            }
            int written = 0;
            if (kind == "csv")
            {
                List<List<object>> rows = File.Exists(path) ? CsvTable.Read(path) : new List<List<object>>();
                while (rows.Count < row0 - 1 + values.Length)
                {
                    rows.Add(new List<object>());
                }
                for (int r = 0; r < values.Length; r++)
                {
                    List<object> target = rows[row0 - 1 + r];
                    while (target.Count < col0 - 1 + values[r].Length)
                    {
                        target.Add("");
                    }
                    for (int c = 0; c < values[r].Length; c++)
                    {
                        target[col0 - 1 + c] = values[r][c] ?? "";
                        written++;
                    }
                }
                CsvTable.WriteAtomic(path, rows.Cast<IList<object>>());
                return new Dictionary<string, object> { { "saved", path }, { "sheet", CsvSheetName(path) }, { "start_cell", startCell }, { "written", written } };
            }

            Action<OoxmlPackage> edit = package =>
            {
                var book = new XlsxBook(package);
                SheetRef target = book.Sheets.FirstOrDefault(s => s.Name == sheet) ?? book.AddSheet(sheet);
                var doc = book.SheetDoc(target);
                var data = book.SheetData(doc);
                for (int r = 0; r < values.Length; r++)
                {
                    for (int c = 0; c < values[r].Length; c++)
                    {
                        book.SetValue(data, row0 + r, col0 + c, values[r][c]);
                        written++;
                    }
                }
                book.UpdateDimension(doc);
                book.SaveSheet(target, doc);
                book.InvalidateCalculation();
            };
            if (File.Exists(path))
            {
                OoxmlPackage.Edit(path, edit);
            }
            else
            {
                XlsxBook.ValidateSheetName(sheet);
                OoxmlPackage.CreateFromTemplate(path, XlsxTemplate.Build(Ext(path), sheet), edit);
            }
            return new Dictionary<string, object> { { "saved", path }, { "sheet", sheet }, { "start_cell", startCell }, { "written", written } };
        }

        public static Dictionary<string, object> CreateWorkbook(string path, object[] sheets)
        {
            if (sheets == null || sheets.Length == 0)
            {
                throw new ArgumentException("sheets must not be empty");
            }
            CheckFormat(path);
            string ext = Ext(path);
            if (ext == ".xls")
            {
                throw new ArgumentException("creating .xls is not supported - use xlsx");
            }
            var specs = sheets.Select(s => s as Dictionary<string, object> ?? new Dictionary<string, object>()).ToList();
            if (CsvTable.IsCsvExtension(ext))
            {
                if (specs.Count > 1)
                {
                    throw new ArgumentException("csv supports a single sheet");
                }
                object[][] rows = Matrix(specs[0], "values");
                CsvTable.WriteAtomic(path, rows.Select(r => (IList<object>)r.Select(v => v ?? "").ToList()));
                return new Dictionary<string, object> { { "created", path }, { "sheets", new[] { CsvSheetName(path) } } };
            }
            var names = specs.Select((s, i) => s.ContainsKey("name") && s["name"] != null ? System.Convert.ToString(s["name"]) : "Sheet" + (i + 1)).ToList();
            foreach (string name in names)
            {
                XlsxBook.ValidateSheetName(name);
            }
            OoxmlPackage.CreateFromTemplate(path, XlsxTemplate.Build(ext, names[0]), package =>
            {
                var book = new XlsxBook(package);
                for (int i = 0; i < specs.Count; i++)
                {
                    SheetRef target = i == 0 ? book.FindSheet(names[0]) : book.AddSheet(names[i]);
                    var doc = book.SheetDoc(target);
                    var data = book.SheetData(doc);
                    object[][] rows = Matrix(specs[i], "values");
                    for (int r = 0; r < rows.Length; r++)
                    {
                        for (int c = 0; c < rows[r].Length; c++)
                        {
                            book.SetValue(data, r + 1, c + 1, rows[r][c]);
                        }
                    }
                    book.UpdateDimension(doc);
                    book.SaveSheet(target, doc);
                }
            });
            return new Dictionary<string, object> { { "created", path }, { "sheets", names } };
        }

        private static object[][] Matrix(Dictionary<string, object> spec, string key)
        {
            var rows = spec.ContainsKey(key) ? spec[key] as object[] : null;
            return rows == null ? new object[0][] : rows.Select(r => r as object[] ?? new[] { r }).ToArray();
        }

        public static Dictionary<string, object> Convert(string path, string sheet, string to)
        {
            to = (to ?? "csv").ToLowerInvariant();
            if (to == "parquet")
            {
                throw new ArgumentException("parquet output is not available in the C# server - use to='csv'");
            }
            if (to != "csv")
            {
                throw new ArgumentException("to must be 'csv'");
            }
            List<List<object>> rows = AllRows(path, sheet);
            List<string> columns = UniqueColumns(rows.Count > 0 ? rows[0] : new List<object>());
            string output = Path.Combine(Path.GetDirectoryName(Path.GetFullPath(path)), Path.GetFileNameWithoutExtension(path) + ".csv");
            if (string.Equals(Path.GetFullPath(output), Path.GetFullPath(path), StringComparison.OrdinalIgnoreCase))
            {
                throw new ArgumentException("source is already a csv file");
            }
            var data = new List<IList<object>> { columns.Cast<object>().ToList() };
            foreach (List<object> row in rows.Skip(1))
            {
                var padded = row.Take(columns.Count).ToList();
                while (padded.Count < columns.Count)
                {
                    padded.Add(null);
                }
                data.Add(padded);
            }
            CsvTable.WriteAtomic(output, data);
            return new Dictionary<string, object> { { "output", output }, { "rows", Math.Max(0, rows.Count - 1) }, { "columns", columns }, { "format", "csv" } };
        }

        private static List<string> UniqueColumns(List<object> header)
        {
            var seen = new Dictionary<string, int>(StringComparer.Ordinal);
            var columns = new List<string>();
            for (int i = 0; i < header.Count; i++)
            {
                string name = header[i] == null ? "" : Cells.ToText(header[i]).Trim();
                if (name.Length == 0)
                {
                    name = "col" + (i + 1);
                }
                int count;
                if (seen.TryGetValue(name, out count))
                {
                    seen[name] = count + 1;
                    name = name + "_" + (count + 1);
                }
                else
                {
                    seen[name] = 1;
                }
                columns.Add(name);
            }
            return columns;
        }

        // ---- thao tác sheet / style / table ----

        private static void EditWorkbook(string path, Action<XlsxBook> edit)
        {
            FileSafety.RequireFile(path);
            CheckFormat(path);
            string kind = Kind(path);
            if (kind == "xls")
            {
                throw new ArgumentException("editing .xls (BIFF) is not supported - save as xlsx or use the WPS live bridge");
            }
            if (kind == "csv")
            {
                throw new ArgumentException("this operation requires a workbook format (xlsx/xlsm), not csv/tsv");
            }
            OoxmlPackage.Edit(path, package => edit(new XlsxBook(package)));
        }

        public static Dictionary<string, object> CreateSheet(string path, string sheet, bool overwrite)
        {
            List<string> names = null;
            EditWorkbook(path, book =>
            {
                SheetRef existing = book.Sheets.FirstOrDefault(s => s.Name == sheet);
                if (existing != null)
                {
                    if (!overwrite)
                    {
                        throw new ArgumentException("sheet already exists: " + sheet);
                    }
                    if (book.Sheets.Count <= 1)
                    {
                        // Không được xoá sheet duy nhất: thêm sheet mới trước rồi xoá sheet cũ.
                        SheetRef fresh = book.AddSheet(sheet + "~new");
                        book.DeleteSheet(book.FindSheet(sheet));
                        book.RenameSheet(book.FindSheet(fresh.Name), sheet);
                        names = book.SheetNames;
                        return;
                    }
                    book.DeleteSheet(existing);
                }
                book.AddSheet(sheet);
                names = book.SheetNames;
            });
            return new Dictionary<string, object> { { "path", path }, { "sheet", sheet }, { "sheets", names } };
        }

        public static Dictionary<string, object> CopySheet(string path, string source, string target)
        {
            List<string> names = null;
            EditWorkbook(path, book =>
            {
                SheetRef src = book.FindSheet(source);
                if (book.HasSheet(target))
                {
                    throw new ArgumentException("sheet already exists: " + target);
                }
                book.CopySheet(src, target);
                names = book.SheetNames;
            });
            return new Dictionary<string, object> { { "path", path }, { "src", source }, { "dst", target }, { "sheets", names } };
        }

        public static Dictionary<string, object> RenameSheet(string path, string sheet, string newName)
        {
            List<string> names = null;
            EditWorkbook(path, book =>
            {
                SheetRef reference = book.Sheets.FirstOrDefault(s => s.Name == sheet);
                if (reference == null)
                {
                    throw new ArgumentException("sheet not found: " + sheet);
                }
                book.RenameSheet(reference, newName);
                names = book.SheetNames;
            });
            return new Dictionary<string, object> { { "path", path }, { "old", sheet }, { "new", newName }, { "sheets", names } };
        }

        public static Dictionary<string, object> DeleteSheet(string path, string sheet)
        {
            List<string> names = null;
            EditWorkbook(path, book =>
            {
                SheetRef reference = book.Sheets.FirstOrDefault(s => s.Name == sheet);
                if (reference == null)
                {
                    throw new ArgumentException("sheet not found: " + sheet);
                }
                if (book.Sheets.Count <= 1)
                {
                    throw new ArgumentException("cannot delete the only sheet in the workbook");
                }
                book.DeleteSheet(reference);
                names = book.SheetNames;
            });
            return new Dictionary<string, object> { { "path", path }, { "deleted", sheet }, { "sheets", names } };
        }

        public static Dictionary<string, object> FormatRange(string path, string sheet, string cellRange, object styles)
        {
            int minCol, minRow, maxCol, maxRow;
            Cells.ParseBoundedRange(cellRange, out minCol, out minRow, out maxCol, out maxRow);
            int rowCount = maxRow - minRow + 1;
            int colCount = maxCol - minCol + 1;
            var single = styles as Dictionary<string, object>;
            var matrix = styles as object[];
            if (single == null && matrix == null)
            {
                throw new ArgumentException("styles must be an object or a 2D array");
            }
            if (matrix != null && (matrix.Length != rowCount || matrix.Any(r => !(r is object[]) || ((object[])r).Length != colCount)))
            {
                throw new ArgumentException("styles matrix size must match the range size");
            }
            int styled = 0;
            EditWorkbook(path, book =>
            {
                SheetRef reference = book.FindSheet(sheet);
                var doc = book.SheetDoc(reference);
                var data = book.SheetData(doc);
                XlsxStyles styleSheet = book.Styles();
                for (int r = 0; r < rowCount; r++)
                {
                    for (int c = 0; c < colCount; c++)
                    {
                        var spec = single ?? ((object[])matrix[r])[c] as Dictionary<string, object>;
                        if (spec == null)
                        {
                            continue;
                        }
                        var cell = book.GetOrCreateCell(data, minRow + r, minCol + c);
                        int current;
                        int.TryParse((string)cell.Attribute("s") ?? "0", out current);
                        cell.SetAttributeValue("s", styleSheet.Derive(current, spec));
                        styled++;
                    }
                }
                book.UpdateDimension(doc);
                book.SaveSheet(reference, doc);
            });
            return new Dictionary<string, object> { { "path", path }, { "sheet", sheet }, { "range", cellRange }, { "styled_cells", styled } };
        }

        public static Dictionary<string, object> CreateTable(string path, string sheet, string cellRange, string tableName)
        {
            if (!Regex.IsMatch(tableName ?? "", "^[A-Za-z_][A-Za-z0-9_]*$"))
            {
                throw new ArgumentException("table_name must start with a letter/underscore and contain only letters, digits, underscores");
            }
            if (Regex.IsMatch(tableName, "^[A-Za-z]{1,3}[0-9]+$") || Regex.IsMatch(tableName, "^[RrCc][0-9]*$"))
            {
                throw new ArgumentException("table_name must not look like a cell reference (e.g. 'A1', 'R1C1')");
            }
            List<string> tables = null;
            EditWorkbook(path, book =>
            {
                SheetRef reference = book.FindSheet(sheet);
                if (book.AllTableNames().Any(n => string.Equals(n, tableName, StringComparison.OrdinalIgnoreCase)))
                {
                    throw new ArgumentException("table already exists: " + tableName);
                }
                book.AddTable(reference, cellRange, tableName);
                tables = book.SheetTableNames(reference);
            });
            return new Dictionary<string, object> { { "path", path }, { "sheet", sheet }, { "range", cellRange }, { "table", tableName }, { "tables", tables } };
        }
    }

    // .xls (BIFF): Windows đọc qua Excel/WPS (COM); Linux nhờ LibreOffice chuyển sang .xlsx rồi đọc như
    // file xlsx thường (LibreOffice_arch.md mục 11) — máy không có LibreOffice thì báo rõ.
    internal static class XlsAutomation
    {
        public static List<SheetGrid> Read(string path, string sheet, bool allSheets)
        {
            Type type = Type.GetTypeFromProgID("Excel.Application") ?? Type.GetTypeFromProgID("KET.Application") ?? Type.GetTypeFromProgID("ET.Application");
            if (type == null)
            {
                throw new InvalidOperationException("reading .xls needs Microsoft Excel or WPS Spreadsheets installed - or save the file as .xlsx");
            }
            dynamic app = null;
            dynamic workbook = null;
            var grids = new List<SheetGrid>();
            try
            {
                app = Activator.CreateInstance(type);
                try
                {
                    app.DisplayAlerts = false;
                    app.Visible = false;
                }
                catch (Exception)
                {
                }
                workbook = app.Workbooks.Open(Path.GetFullPath(path), 0, true);
                var targets = new List<dynamic>();
                if (allSheets)
                {
                    foreach (dynamic ws in workbook.Worksheets)
                    {
                        targets.Add(ws);
                    }
                }
                else if (sheet != null)
                {
                    dynamic ws;
                    try
                    {
                        ws = workbook.Worksheets[sheet];
                    }
                    catch (Exception)
                    {
                        throw new ArgumentException("sheet not found: " + sheet);
                    }
                    targets.Add(ws);
                }
                else
                {
                    targets.Add(workbook.Worksheets[1]);
                }
                foreach (dynamic ws in targets)
                {
                    grids.Add(ReadSheet(ws));
                }
                return grids;
            }
            finally
            {
                try
                {
                    if (workbook != null)
                    {
                        workbook.Close(false);
                        Marshal.FinalReleaseComObject(workbook);
                    }
                }
                catch (Exception)
                {
                }
                try
                {
                    if (app != null)
                    {
                        app.Quit();
                        Marshal.FinalReleaseComObject(app);
                    }
                }
                catch (Exception)
                {
                }
                GC.Collect();
                GC.WaitForPendingFinalizers();
            }
        }

        private static SheetGrid ReadSheet(dynamic ws)
        {
            var grid = new SheetGrid { Name = System.Convert.ToString(ws.Name) };
            dynamic used = ws.UsedRange;
            int lastRow = System.Convert.ToInt32(used.Row) + System.Convert.ToInt32(used.Rows.Count) - 1;
            int lastCol = System.Convert.ToInt32(used.Column) + System.Convert.ToInt32(used.Columns.Count) - 1;
            object raw = ws.Range(ws.Cells[1, 1], ws.Cells[lastRow, lastCol]).Value;
            var values = raw as object[,];
            if (values == null)
            {
                if (raw != null)
                {
                    grid.Set(1, 1, Clean(raw));
                }
            }
            else
            {
                int rowBase = values.GetLowerBound(0);
                int colBase = values.GetLowerBound(1);
                for (int r = rowBase; r <= values.GetUpperBound(0); r++)
                {
                    for (int c = colBase; c <= values.GetUpperBound(1); c++)
                    {
                        object value = Clean(values[r, c]);
                        if (value != null)
                        {
                            grid.Set(r - rowBase + 1, c - colBase + 1, value);
                        }
                    }
                }
            }
            bool empty = lastRow == 1 && lastCol == 1 && raw == null;
            grid.MaxRow = empty ? 0 : lastRow;
            grid.MaxCol = empty ? 0 : lastCol;
            return grid;
        }

        private static object Clean(object value)
        {
            if (value == null)
            {
                return null;
            }
            if (value is DateTime)
            {
                return ((DateTime)value).ToString("yyyy-MM-ddTHH:mm:ss", CultureInfo.InvariantCulture);
            }
            if (value is double)
            {
                double d = (double)value;
                if (double.IsNaN(d) || double.IsInfinity(d))
                {
                    return null;
                }
                return d;
            }
            if (value is decimal)
            {
                return (double)(decimal)value;
            }
            if (value is int)
            {
                // Excel trả mã lỗi (#DIV/0!...) dạng int CVErr.
                return ErrorText((int)value);
            }
            return value;
        }

        private static object ErrorText(int code)
        {
            switch (code)
            {
                case -2146826281: return "#DIV/0!";
                case -2146826246: return "#N/A";
                case -2146826259: return "#NAME?";
                case -2146826288: return "#NULL!";
                case -2146826252: return "#NUM!";
                case -2146826265: return "#REF!";
                case -2146826273: return "#VALUE!";
                default: return code;
            }
        }
    }
}
