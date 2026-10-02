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
