using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;

namespace AxiomOffice.Bridge
{
    internal static partial class CommandDispatcher
    {
        // Excel / WPS Spreadsheets (AppKind "et").
        private static IEnumerable<CommandInfo> SpreadsheetCommands()
        {
            return new[]
            {
                Command("et.newWorkbook", "et", EtNewWorkbook, "Tạo workbook mới"),
                Command("et.open", "et", EtOpen, "Mở .xlsx/.xls/.csv", Req("path")),
                Command("et.listSheets", "et", EtListSheets, "Danh sách sheet + sheet đang active").ForAgent(),
                Command("et.addSheet", "et", EtAddSheet, "Thêm sheet mới (bỏ trống `name` thì đặt tên Sheet1, Sheet2...)",
                    Opt("name"), Opt("index", "0-based, default: append")).ForAgent(),
                Command("et.renameSheet", "et", EtRenameSheet, "Đổi tên sheet", Req("sheet"), Req("name")).ForAgent(),
                Command("et.activateSheet", "et", EtActivateSheet, "Chuyển sheet", Req("sheet")).ForAgent(),
                Command("et.readRange", "et", EtReadRange, "Đọc vùng, ví dụ `A1:C10`", Req("range"), Opt("sheet")).ForAgent(),
                Command("et.writeRange", "et", EtWriteRange, "Ghi vùng bắt đầu từ ô trên-trái `range`",
                    Req("range", "top-left cell e.g. 'A1'"), Req("values", "2D array of rows e.g. [[\"Tên\",\"Điểm\"],[\"An\",9.5]]"), Opt("sheet")).ForAgent(),
                Command("et.fillRange", "et", EtFillRange,
                    "Viết MỘT công thức vào ô góc trên-trái của `range` rồi điền ra cả vùng, tham chiếu tương đối tự dịch (dùng cho bảng nghìn dòng: đừng gửi từng ô)",
                    Req("range", "the whole area e.g. 'B2:H1000'"), Opt("formula", "written to the top-left cell first"), Opt("sheet")).ForAgent(),
                Command("et.formatRange", "et", EtFormatRange, "Định dạng vùng (màu dạng `#RRGGBB`, `horizontal` left/center/right)",
                    Req("range"), Opt("bold"), Opt("italic"), Opt("fontSize"), Opt("fontColor"), Opt("fillColor"), Opt("numFmt"), Opt("horizontal"), Opt("wrap"), Opt("sheet")).ForAgent(),
                Command("et.addChart", "et", EtAddChart,
                    "Chèn biểu đồ từ vùng dữ liệu; `type` column/bar/line/pie/area/scatter, `width`/`height` tính bằng cm",
                    Req("range", "source data e.g. 'A1:B13'"), Opt("type", "column (default)/bar/line/pie/area/scatter"),
                    Opt("title"), Opt("name"), Opt("anchor", "top-left cell, default 'A1'"),
                    Opt("width", "cm, default 12"), Opt("height", "cm, default 7"), Opt("sheet")).ForAgent(),
                Command("et.listCharts", "et", EtListCharts, "Danh sách biểu đồ trên sheet", Opt("sheet")).ForAgent(),
                Command("et.setConditionalFormat", "et", EtSetConditionalFormat,
                    "Đặt định dạng điều kiện cho vùng: `rules` là mảng rule, mỗi rule có operator "
                    + "(less/lessEqual/greater/greaterEqual/equal/notEqual/between/notBetween/formula), formula1, "
                    + "formula2 (chỉ between), và bold/italic/fontColor/fillColor. Gọi lại trên cùng vùng thì THAY "
                    + "rule cũ của vùng đó, nên nhiều rule trên một vùng phải để trong MỘT lần gọi",
                    Req("range"), Req("rules", "array of rule objects"), Opt("sheet")).ForAgent(),
                Command("et.listConditionalFormats", "et", EtListConditionalFormats,
                    "Liệt kê định dạng điều kiện đang có (đọc lại để tự kiểm); bỏ trống `range` thì soi vùng đang dùng",
                    Opt("range", "chỉ xem một vùng"), Opt("sheet")).ForAgent(),
                Command("et.undo", "et", EtUndo, "Hoàn tác", Opt("count")).ForAgent(),
                Command("et.exportPdf", "et", EtExportPdf, "Xuất PDF", Req("path")).ForAgent(),
                Command("et.save", "et", (host, p) => SaveDocument(host, "et", null), "Lưu").ForAgent(),
                Command("et.saveAs", "et", (host, p) => SaveDocument(host, "et", ParamString(p, "path", null)), "Lưu thành file mới", Req("path")).ForAgent()
            };
        }

        private static Dictionary<string, object> EtListSheets(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            int count = Convert.ToInt32(wb.Worksheets.Count);
            var sheets = new List<object>();
            for (int i = 1; i <= count; i++)
            {
                sheets.Add(Convert.ToString(wb.Worksheets[i].Name));
            }
            return new Dictionary<string, object>
            {
                { "workbook", Convert.ToString(wb.Name) },
                { "activeSheet", Convert.ToString(app.ActiveSheet.Name) },
                { "sheets", sheets }
            };
        }

        // Sổ đang mở; chưa có sổ nào (Excel trống) thì tạo một sổ. Agent không còn được gọi et.newWorkbook
        // (log 30/09 01:15: model tạo thêm Book2 dù Book1 đang mở, người dùng đóng Excel thì bị hỏi lưu một sổ
        // họ không biết) - giống Word/PowerPoint, agent làm việc trên tài liệu đang mở.
        private static dynamic EnsureWorkbook(dynamic app)
        {
            dynamic wb = app.ActiveWorkbook;
            if (wb == null)
            {
                wb = app.Workbooks.Add();
                Logger.Info("et: no workbook open, created " + Convert.ToString(wb.Name));
            }
            return wb;
        }

        private static Dictionary<string, object> EtNewWorkbook(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = app.Workbooks.Add();
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(wb.Name) }
            };
        }

        private static Dictionary<string, object> EtOpen(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.Workbooks.Open(path);
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(wb.Name) },
                { "fullName", Convert.ToString(wb.FullName) }
            };
        }

        private static Dictionary<string, object> EtReadRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", "A1");
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic range = sheet.Range[address];
            object value = range.Value2;
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "range", address },
                { "values", ToMatrix(value) }
            };
        }

        private static Dictionary<string, object> EtWriteRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new ArgumentException("'range' is required: the top-left cell to write at, e.g. 'A1'");
            }
            string sheetName = ParamString(p, "sheet", null);
            List<IList> rowList = ParamMatrix(p, "values", true);

            int rowCount = rowList.Count;
            int colCount = Math.Max(1, rowList.Max(row => row.Count));
            object[,] matrix = new object[rowCount, colCount];
            for (int r = 0; r < rowCount; r++)
            {
                IList row = rowList[r];
                for (int c = 0; c < colCount; c++)
                {
                    matrix[r, c] = c < row.Count ? row[c] : null;
                }
            }

            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic target = sheet.Range[address];
            try
            {
                target = target.Resize[rowCount, colCount];
            }
            catch
            {
            }
            target.Value2 = matrix;

            return new Dictionary<string, object>
            {
                { "written", rowCount * colCount },
                { "sheet", Convert.ToString(sheet.Name) }
            };
        }

        // Viết MỘT công thức vào ô góc rồi điền ra cả vùng (AutoFill của Excel tự dịch tham chiếu tương
        // đối, và làm được cả khối 2D trong một lần). Chi phí token của agent tính theo TỪNG Ô nó viết
        // ra, nên bảng nghìn dòng phải đi bằng đường này chứ không gửi từng ô.
        private static Dictionary<string, object> EtFillRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required: the whole area to fill, e.g. 'B2:H1000'");
            }
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic target = sheet.Range[address];
            dynamic corner = target.Cells[1, 1];

            string formula = ParamString(p, "formula", null);
            if (!string.IsNullOrEmpty(formula))
            {
                corner.Formula = formula;
            }

            int rows = Convert.ToInt32(target.Rows.Count);
            int cols = Convert.ToInt32(target.Columns.Count);
            if (rows > 1 || cols > 1)
            {
                corner.AutoFill(target, 0);   // xlFillDefault
            }

            var sample = new List<object>();
            for (int r = 1; r <= Math.Min(rows, 3); r++)
            {
                var line = new List<object>();
                for (int c = 1; c <= Math.Min(cols, 3); c++)
                {
                    line.Add(Convert.ToString(target.Cells[r, c].Formula));
                }
                sample.Add(line);
            }
            return new Dictionary<string, object>
            {
                { "filled", rows * cols }, { "range", address }, { "sheet", Convert.ToString(sheet.Name) },
                { "sample", sample }
            };
        }

        private static Dictionary<string, object> EtFormatRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required");
            }
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic range = sheet.Range[address];
            dynamic font = range.Font;
            if (HasParam(p, "bold"))
            {
                font.Bold = ParamBool(p, "bold", false);
            }
            if (HasParam(p, "italic"))
            {
                font.Italic = ParamBool(p, "italic", false);
            }
            if (HasParam(p, "fontSize"))
            {
                font.Size = ParamInt(p, "fontSize", 11);
            }
            int? fontColor = ParseBgrColor(ParamString(p, "fontColor", null));
            if (fontColor.HasValue)
            {
                font.Color = fontColor.Value;
            }
            int? fillColor = ParseBgrColor(ParamString(p, "fillColor", null));
            if (fillColor.HasValue)
            {
                range.Interior.Color = fillColor.Value;
            }
            if (HasParam(p, "numFmt"))
            {
                range.NumberFormat = ParamString(p, "numFmt", "General");
            }
            if (HasParam(p, "horizontal"))
            {
                string horizontal = ParamString(p, "horizontal", "left").ToLowerInvariant();
                range.HorizontalAlignment = horizontal == "center" ? -4108 : (horizontal == "right" ? -4152 : -4131);
            }
            if (HasParam(p, "wrap"))
            {
                range.WrapText = ParamBool(p, "wrap", false);
            }
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "range", address }
            };
        }

        private static Dictionary<string, object> EtActivateSheet(IAppHost host, Dictionary<string, object> p)
        {
            string sheetName = ParamString(p, "sheet", null);
            if (string.IsNullOrEmpty(sheetName))
            {
                throw new InvalidOperationException("'sheet' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            wb.Worksheets[sheetName].Activate();
            return new Dictionary<string, object> { { "active", sheetName } };
        }

        // Thêm sheet mới. Không có lệnh này thì agent không dựng được sổ nhiều sheet trên tài liệu đang mở
        // (log 02/10: model thử ~30 tên lệnh tự nghĩ ra rồi bỏ cuộc). Tên bỏ trống -> Sheet1, Sheet2...
        // tránh trùng; `index` là vị trí chèn (1-based như Excel/WPS, mặc định là cuối).
        private static Dictionary<string, object> EtAddSheet(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            var existing = new List<string>();
            int count = Convert.ToInt32(wb.Worksheets.Count);
            for (int i = 1; i <= count; i++)
            {
                existing.Add(Convert.ToString(wb.Worksheets[i].Name));
            }

            string name = ParamString(p, "name", null);
            if (string.IsNullOrEmpty(name))
            {
                // So nho nhat con trong (Sheet1, Sheet2...), giong ban LibreOffice - hai lan chay cung mot thu.
                int suffix = 1;
                while (existing.Contains("Sheet" + suffix))
                {
                    suffix++;
                }
                name = "Sheet" + suffix;
            }
            else if (existing.Contains(name))
            {
                throw new InvalidOperationException("a sheet named '" + name + "' already exists (sheets: " +
                    string.Join(", ", existing) + ")");
            }

            // `index` 0-based giong ban LibreOffice (Excel/WPS dem sheet tu 1, nen phai doi o day).
            int insertAt = Math.Max(0, Math.Min(ParamInt(p, "index", count), count));
            dynamic sheet = insertAt >= count
                ? wb.Worksheets.Add(Type.Missing, wb.Worksheets[count])
                : wb.Worksheets.Add(Type.Missing, wb.Worksheets[insertAt + 1]);
            sheet.Name = name;
            sheet.Activate();
            return new Dictionary<string, object> { { "sheet", name }, { "sheets", SheetNames(wb) } };
        }

        private static Dictionary<string, object> EtRenameSheet(IAppHost host, Dictionary<string, object> p)
        {
            string oldName = ParamString(p, "sheet", null);
            string newName = ParamString(p, "name", null);
            if (string.IsNullOrEmpty(oldName))
            {
                throw new InvalidOperationException("'sheet' is required");
            }
            if (string.IsNullOrEmpty(newName))
            {
                throw new InvalidOperationException("'name' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            var names = SheetNames(wb);
            if (!names.Contains(oldName))
            {
                throw new InvalidOperationException("no sheet named '" + oldName + "' (sheets: " +
                    string.Join(", ", names) + ")");
            }
            if (newName != oldName && names.Contains(newName))
            {
                throw new InvalidOperationException("a sheet named '" + newName + "' already exists (sheets: " +
                    string.Join(", ", names) + ")");
            }
            wb.Worksheets[oldName].Name = newName;   // Excel tự cập nhật công thức trỏ tên cũ
            return new Dictionary<string, object> { { "sheet", newName }, { "sheets", SheetNames(wb) } };
        }

        // Kiểu biểu đồ theo tên người dùng gõ -> hằng số XlChartType của Excel.
        private static int ChartType(string kind)
        {
            switch (kind)
            {
                case "column": return 51;     // xlColumnClustered
                case "bar": return 57;        // xlBarClustered
                case "line": return 4;        // xlLine
                case "pie": return 5;         // xlPie
                case "area": return 1;        // xlArea
                case "scatter": return -4169; // xlXYScatter
                default:
                    throw new InvalidOperationException(
                        "'type' must be one of area/bar/column/line/pie/scatter, got '" + kind + "'");
            }
        }

        // Chèn biểu đồ từ một vùng dữ liệu. Không có lệnh này thì Dashboard chỉ là bảng số.
        private static Dictionary<string, object> EtAddChart(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required");
            }
            string kind = (ParamString(p, "type", "column") ?? "column").ToLowerInvariant();
            int xlType = ChartType(kind);
            string sheetName = ParamString(p, "sheet", null);
            string anchor = ParamString(p, "anchor", "A1") ?? "A1";
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];

            // cm -> point (Excel dùng point cho vị trí/kích thước shape).
            double width = Math.Max(30, ParamDouble(p, "width", 12.0) * 28.3465);
            double height = Math.Max(30, ParamDouble(p, "height", 7.0) * 28.3465);
            dynamic anchorCell = sheet.Range[anchor];

            // Tên phải tránh MỌI hình trên sheet, không chỉ hình là chart: một hình tên "Chart3" không phải
            // chart vẫn chiếm chỗ, mà "ChartObjects().Count + 1" thì có thể sinh đúng cái tên đó.
            // (Bản LibreOffice phải tránh tên chart của CẢ TÀI LIỆU - tên chart bên đó duy nhất theo tài
            // liệu chứ không theo sheet, và đó là một lỗi thật đã đo được; xem calc.py _taken_chart_names.)
            var taken = new HashSet<string>(StringComparer.Ordinal);
            try
            {
                foreach (dynamic existingShape in sheet.Shapes)
                {
                    taken.Add(Convert.ToString(existingShape.Name));
                }
            }
            catch (Exception)
            {
                // host thiếu Shapes: quay về chỉ kiểm chart
                foreach (dynamic existing in sheet.ChartObjects())
                {
                    taken.Add(Convert.ToString(existing.Name));
                }
            }

            string name = ParamString(p, "name", null);
            if (string.IsNullOrEmpty(name))
            {
                int index = 1;
                while (taken.Contains("Chart" + index))
                {
                    index++;
                }
                name = "Chart" + index;
            }
            else if (taken.Contains(name))
            {
                throw new InvalidOperationException("a chart named '" + name +
                    "' already exists on sheet '" + Convert.ToString(sheet.Name) + "'");
            }

            dynamic shape = sheet.Shapes.AddChart2(-1, xlType, anchorCell.Left, anchorCell.Top, width, height);
            shape.Name = name;
            shape.Chart.SetSourceData(sheet.Range[address]);
            string title = ParamString(p, "title", null);
            bool titleApplied = false;
            if (!string.IsNullOrEmpty(title))
            {
                shape.Chart.HasTitle = true;
                shape.Chart.ChartTitle.Text = title;
                titleApplied = true;
            }
            // Đọc lại kiểu THẬT SỰ của biểu đồ thay vì lặp lại điều vừa xin (giống bản LibreOffice).
            string diagram = ChartKindName(shape.Chart);
            return new Dictionary<string, object>
            {
                { "chart", name }, { "sheet", Convert.ToString(sheet.Name) },
                { "type", kind }, { "diagram", diagram },
                { "typeApplied", diagram == kind }, { "titleApplied", titleApplied },
                { "range", address }, { "anchor", anchor }
            };
        }

        // Kiểu thật -> cùng bộ tên với bản LibreOffice (column/bar/line/pie/area/scatter).
        private static string ChartKindName(dynamic chart)
        {
            try
            {
                switch (Convert.ToInt32(chart.ChartType))
                {
                    case 51: return "column";
                    case 57: return "bar";
                    case 4: return "line";
                    case 5: return "pie";
                    case 1: return "area";
                    case -4169: return "scatter";
                    default: return "type" + Convert.ToInt32(chart.ChartType);
                }
            }
            catch (Exception)
            {
                return "";
            }
        }

        private static Dictionary<string, object> EtListCharts(IAppHost host, Dictionary<string, object> p)
        {
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            var charts = new List<object>();
            foreach (dynamic item in sheet.ChartObjects())
            {
                charts.Add(new Dictionary<string, object>
                {
                    { "name", Convert.ToString(item.Name) },
                    { "diagram", ChartKindName(item.Chart) }
                });
            }
            return new Dictionary<string, object> { { "sheet", Convert.ToString(sheet.Name) }, { "charts", charts } };
        }

        private static List<string> SheetNames(dynamic wb)
        {
            var names = new List<string>();
            int count = Convert.ToInt32(wb.Worksheets.Count);
            for (int i = 1; i <= count; i++)
            {
                names.Add(Convert.ToString(wb.Worksheets[i].Name));
            }
            return names;
        }

        private static Dictionary<string, object> EtExportPdf(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            wb.ExportAsFixedFormat(0, path);
            return new Dictionary<string, object> { { "exported", path } };
        }

        // --- Định dạng điều kiện (Excel/WPS dùng Range.FormatConditions) ---------------------------
        //
        // Bản LibreOffice phải đi đường vòng qua UNO (xem calc.py: createByRange rồi createEntry);
        // bên này API phẳng hơn nên gọi thẳng. Tên lệnh/tham số hai bên giống nhau để agent thấy cùng
        // một hợp đồng, nhưng cách đặt style thì khác: LibreOffice gắn bằng TÊN CELL STYLE, Excel gắn
        // màu trực tiếp nên `styleName` chỉ có tác dụng bên LibreOffice.

        // XlFormatConditionOperator: xlBetween=1, xlNotBetween=2, xlEqual=3, xlNotEqual=4,
        // xlGreater=5, xlLess=6, xlGreaterEqual=7, xlLessEqual=8.
        private static readonly Dictionary<string, int> ConditionalOperators =
            new Dictionary<string, int>(StringComparer.Ordinal)
            {
                { "between", 1 }, { "notbetween", 2 }, { "equal", 3 }, { "notequal", 4 },
                { "greater", 5 }, { "less", 6 }, { "greaterequal", 7 }, { "lessequal", 8 }
            };

        private static readonly Dictionary<int, string> ConditionalOperatorNames =
            new Dictionary<int, string>
            {
                { 1, "between" }, { 2, "notBetween" }, { 3, "equal" }, { 4, "notEqual" },
                { 5, "greater" }, { 6, "less" }, { 7, "greaterEqual" }, { 8, "lessEqual" }
            };

        private const int XlCellValue = 1;      // XlFormatConditionType
        private const int XlExpression = 2;

        private static Dictionary<string, object> EtSetConditionalFormat(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required: the area the rules apply to, e.g. 'B2:B100'");
            }
            List<Dictionary<string, object>> rules = ParamObjects(p, "rules");
            if (rules == null || rules.Count == 0)
            {
                throw new InvalidOperationException("'rules' is required: a list of rules, e.g. "
                    + "[{\"operator\": \"less\", \"formula1\": \"50\", \"fillColor\": \"#FFCCCC\"}]");
            }
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            string sheetName = ParamString(p, "sheet", null);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic range = sheet.Range[address];

            // Kiểm hết rule trước khi ghi: rule thứ ba sai thì hai rule đầu cũng không được vào file.
            var plan = new List<Dictionary<string, object>>();
            for (int i = 0; i < rules.Count; i++)
            {
                string key = NormalizeOperator(ParamString(rules[i], "operator", "less"));
                bool isFormula = key == "formula";
                if (!isFormula && !ConditionalOperators.ContainsKey(key))
                {
                    throw new InvalidOperationException(string.Format(
                        "'rules[{0}].operator' must be one of less/lessEqual/greater/greaterEqual/equal/notEqual/"
                        + "between/notBetween/formula, got '{1}'", i, key));
                }
                string formula1 = ParamString(rules[i], "formula1", null);
                if (string.IsNullOrEmpty(formula1))
                {
                    throw new InvalidOperationException(string.Format(
                        "'rules[{0}]' needs 'formula1' - the value to compare with, e.g. "
                        + "{{\"operator\": \"less\", \"formula1\": \"50\"}}", i));
                }
                string formula2 = ParamString(rules[i], "formula2", null);
                if ((key == "between" || key == "notbetween") && string.IsNullOrEmpty(formula2))
                {
                    throw new InvalidOperationException(string.Format(
                        "'rules[{0}]': operator '{1}' needs 'formula2' as well", i, key));
                }
                plan.Add(new Dictionary<string, object>
                {
                    { "key", key }, { "formula1", formula1 }, { "formula2", formula2 }, { "rule", rules[i] }
                });
            }

            range.FormatConditions.Delete();        // thay rule cũ của vùng: gọi lại không chồng lên nhau

            var written = new List<object>();
            foreach (Dictionary<string, object> item in plan)
            {
                string key = Convert.ToString(item["key"]);
                string formula1 = Convert.ToString(item["formula1"]);
                string formula2 = Convert.ToString(item["formula2"]);
                Dictionary<string, object> rule = (Dictionary<string, object>)item["rule"];
                dynamic condition;
                if (key == "formula")
                {
                    // Công thức trần: Excel nhận kèm dấu =, và không dùng toán tử.
                    condition = range.FormatConditions.Add(XlExpression, Type.Missing, "=" + formula1.TrimStart('='));
                }
                else if (!string.IsNullOrEmpty(formula2))
                {
                    condition = range.FormatConditions.Add(XlCellValue, ConditionalOperators[key], formula1, formula2);
                }
                else
                {
                    condition = range.FormatConditions.Add(XlCellValue, ConditionalOperators[key], formula1);
                }
                if (ParamBool(rule, "bold", false))
                {
                    condition.Font.Bold = true;
                }
                if (ParamBool(rule, "italic", false))
                {
                    condition.Font.Italic = true;
                }
                int? fontColor = ParseBgrColor(ParamString(rule, "fontColor", null));
                if (fontColor.HasValue)
                {
                    condition.Font.Color = fontColor.Value;
                }
                int? fillColor = ParseBgrColor(ParamString(rule, "fillColor", null));
                if (fillColor.HasValue)
                {
                    condition.Interior.Color = fillColor.Value;
                }
                written.Add(new Dictionary<string, object>
                {
                    { "operator", key }, { "formula1", formula1 },
                    { "formula2", string.IsNullOrEmpty(formula2) ? null : formula2 }
                });
            }

            return Ok(new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) }, { "range", Convert.ToString(range.Address) }, { "rules", written }
            });
        }

        private static Dictionary<string, object> EtListConditionalFormats(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            // Excel không có "mọi định dạng điều kiện của sheet" như LibreOffice: FormatConditions chỉ
            // đọc được từ một Range. Bỏ trống `range` thì soi vùng đang dùng thật.
            dynamic scope = string.IsNullOrEmpty(address) ? sheet.UsedRange : sheet.Range[address];

            var items = new List<object>();
            dynamic conditions = scope.FormatConditions;
            int count = Convert.ToInt32(conditions.Count);
            for (int i = 1; i <= count; i++)
            {
                dynamic condition = conditions[i];
                object operatorName = null;
                object type = null;
                try
                {
                    type = Convert.ToInt32(condition.Type);
                }
                catch (Exception)
                {
                }
                try
                {
                    int code = Convert.ToInt32(condition.Operator);
                    operatorName = Convert.ToInt32(type) == XlExpression
                        ? "formula"
                        : (ConditionalOperatorNames.ContainsKey(code) ? ConditionalOperatorNames[code] : "unknown(" + code + ")");
                }
                catch (Exception)
                {
                }
                items.Add(new Dictionary<string, object>
                {
                    { "operator", operatorName },
                    { "formula1", SafeFormula(condition, "Formula1") },
                    { "formula2", SafeFormula(condition, "Formula2") },
                    { "type", type }
                });
            }
            return Ok(new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "range", Convert.ToString(scope.Address) },
                { "formats", items }
            });
        }

        // Công thức của FormatCondition có thể ném COMException (vùng gộp, điều kiện kiểu thanh/màu) -
        // đọc hỏng một trường không được làm hỏng cả lệnh đọc.
        private static string SafeFormula(dynamic condition, string name)
        {
            try
            {
                return Convert.ToString(name == "Formula1" ? condition.Formula1 : condition.Formula2);
            }
            catch (Exception)
            {
                return null;
            }
        }

        // "lessEqual" / "less_equal" / "less equal" / "LESS" -> "lessequal"
        private static string NormalizeOperator(string value)
        {
            return (value ?? "less").Replace("_", "").Replace(" ", "").ToLowerInvariant();
        }

        private static Dictionary<string, object> EtUndo(IAppHost host, Dictionary<string, object> p)
        {
            int count = Math.Max(1, ParamInt(p, "count", 1));
            dynamic app = host.Application;
            int undone = 0;
            for (int i = 0; i < count; i++)
            {
                try
                {
                    app.Undo();
                    undone++;
                }
                catch (COMException ex)
                {
                    if (IsRetryableComError(ex))
                    {
                        throw;
                    }
                    Logger.Info("et.undo stopped: " + ex.Message);
                    break;
                }
                catch (Exception ex)
                {
                    Logger.Info("et.undo stopped: " + ex.Message);
                    break;
                }
            }
            return new Dictionary<string, object> { { "undone", undone } };
        }
    }
}
