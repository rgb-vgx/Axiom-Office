using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;

namespace AxiomOffice.Bridge
{
    // PivotTable và xuất biểu đồ thành ảnh (docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md X3, P2). PivotTable luôn
    // tạo trên sheet MỚI (không đè dữ liệu người dùng). Biểu đồ sang PowerPoint/Word: et.exportChart ghi PNG vào thư mục tạm,
    // agent chèn ảnh đó bằng wpp.addImage / writer.insertImage qua other_document - không cần clipboard giữa hai tiến trình.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> SpreadsheetPivotCommands()
        {
            return new[]
            {
                Command("et.pivot", "et", EtPivot,
                    "Tạo PivotTable trên sheet MỚI từ vùng dữ liệu có hàng tiêu đề: `rows`/`columns` = cột nhóm, `values` = [{column, function: sum|count|average|max|min}]",
                    Req("range", "data incl. header row, e.g. 'A1:F500'"),
                    Req("rows", "array of header texts or column letters to group rows by"),
                    Req("values", "[{column: 'Doanh thu', function: 'sum'}]; function default sum"),
                    Opt("columns", "array of header texts/column letters to spread across columns"),
                    Opt("name", "new sheet name, default 'Pivot'"), Opt("sheet")).ForAgent(),
                Command("et.exportChart", "et", EtExportChart,
                    "Xuất một biểu đồ thành ảnh PNG (thư mục tạm) - trả `path` để chèn sang PowerPoint/Word bằng wpp.addImage / writer.insertImage",
                    Opt("chart", "chart name from et.listCharts or 1-based index; default: the only chart"), Opt("sheet")).ForAgent(),
            };
        }

        // Mã hàm tổng hợp của Excel (XlConsolidationFunction).
        private static int PivotFunction(string name, out string label)
        {
            switch ((name ?? "sum").Trim().ToLowerInvariant())
            {
                case "":
                case "sum":
                    label = "Tổng";
                    return -4157;
                case "count":
                    label = "Số lượng";
                    return -4112;
                case "average":
                case "avg":
                case "mean":
                    label = "Trung bình";
                    return -4106;
                case "max":
                    label = "Lớn nhất";
                    return -4136;
                case "min":
                    label = "Nhỏ nhất";
                    return -4139;
            }
            throw new ArgumentException("function '" + name + "' is not supported: use sum, count, average, max or min");
        }

        private static string UniqueSheetName(dynamic wb, string wanted)
        {
            List<string> existing = SheetNames(wb);
            string name = string.IsNullOrWhiteSpace(wanted) ? "Pivot" : wanted.Trim();
            string candidate = name;
            for (int i = 2; existing.Any(item => string.Equals(item, candidate, StringComparison.CurrentCultureIgnoreCase)); i++)
            {
                candidate = name + " " + i;
            }
            return candidate;
        }

        private static Dictionary<string, object> EtPivot(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrWhiteSpace(address))
            {
                throw new ArgumentException("'range' is required: the data incl. header row, e.g. 'A1:F500'");
            }
            List<string> rowFields = ParamStrings(p, "rows") ?? new List<string>();
            List<string> columnFields = ParamStrings(p, "columns") ?? new List<string>();
            List<Dictionary<string, object>> valueFields = ParamObjects(p, "values");
            if (rowFields.Count == 0 && columnFields.Count == 0)
            {
                throw new ArgumentException("give 'rows' (and/or 'columns'): the columns to group by");
            }
            if (valueFields == null || valueFields.Count == 0)
            {
                throw new ArgumentException("'values' is required, e.g. [{\"column\": \"Doanh thu\", \"function\": \"sum\"}]");
            }
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sourceSheet;
            dynamic source = SheetRange(wb, ParamString(p, "sheet", null), address.Trim(), "'range'", out sourceSheet);
            if (Convert.ToInt32(source.Rows.Count) < 2)
            {
                throw new ArgumentException("the range needs a header row and at least one data row");
            }
            // Tên trường = chữ tiêu đề của cột (cho phép model đưa chữ cột "C").
            Func<string, string> header = column =>
            {
                int index = ColumnInRange(source, column, true);
                string text = (Convert.ToString(source.Cells[1, index].Text) ?? "").Trim();
                if (text.Length == 0)
                {
                    throw new ArgumentException("column '" + column + "' has an empty header: PivotTable needs header text");
                }
                return text;
            };
            string sheetName = UniqueSheetName(wb, ParamString(p, "name", null));
            dynamic target = wb.Worksheets.Add(Missing.Value, wb.Worksheets[wb.Worksheets.Count]);
            target.Name = sheetName;
            string tableName = "Axiom" + sheetName.Replace(" ", "");
            dynamic table;
            try
            {
                dynamic cache = wb.PivotCaches().Create(1, source); // xlDatabase = 1
                table = cache.CreatePivotTable(target.Range("A3"), tableName);
            }
            catch (Exception ex)
            {
                if (ex is System.Runtime.InteropServices.COMException && IsRetryableComError((System.Runtime.InteropServices.COMException)ex))
                {
                    throw;
                }
                Logger.Info("et.pivot: PivotCaches().Create unavailable, falling back to PivotTableWizard: " + ex.Message);
                table = target.PivotTableWizard(1, source, target.Range("A3"), tableName);
            }
            var report = new Dictionary<string, object>();
            var rowsDone = new List<string>();
            for (int i = 0; i < rowFields.Count; i++)
            {
                string field = header(rowFields[i]);
                dynamic pivotField = table.PivotFields(field);
                pivotField.Orientation = 1; // xlRowField
                pivotField.Position = i + 1;
                rowsDone.Add(field);
            }
            var columnsDone = new List<string>();
            for (int i = 0; i < columnFields.Count; i++)
            {
                string field = header(columnFields[i]);
                dynamic pivotField = table.PivotFields(field);
                pivotField.Orientation = 2; // xlColumnField
                pivotField.Position = i + 1;
                columnsDone.Add(field);
            }
            var valuesDone = new List<object>();
            foreach (Dictionary<string, object> value in valueFields)
            {
                string field = header(ParamString(value, "column", null));
                string label;
                int function = PivotFunction(ParamString(value, "function", "sum"), out label);
                string caption = label + " " + field;
                table.AddDataField(table.PivotFields(field), caption, function);
                valuesDone.Add(new Dictionary<string, object> { { "column", field }, { "caption", caption } });
            }
            report["sheet"] = sheetName;
            report["table"] = Convert.ToString(table.Name);
            report["rows"] = rowsDone;
            report["columns"] = columnsDone;
            report["values"] = valuesDone;
            try
            {
                report["range"] = Convert.ToString(table.TableRange1.Address(false, false));
            }
            catch
            {
            }
            return report;
        }

        // Chart.Export chỉ ghi PNG khi chart object đang là chart được chọn; Activate() trước rồi export.
        private static bool ExportChartPng(dynamic chartObject, string file)
        {
            try { chartObject.Activate(); } catch { }
            try
            {
                return Convert.ToBoolean(chartObject.Chart.Export(file, "PNG"));
            }
            catch (Exception ex)
            {
                if (ex is System.Runtime.InteropServices.COMException && IsRetryableComError((System.Runtime.InteropServices.COMException)ex))
                {
                    throw;
                }
                Logger.Info("et.exportChart: " + ex.Message);
                return false;
            }
        }

        private static Dictionary<string, object> EtExportChart(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = SheetFor(wb, p);
            dynamic charts = sheet.ChartObjects();
            int count = Convert.ToInt32(charts.Count);
            if (count == 0)
            {
                throw new ArgumentException("sheet '" + Convert.ToString(sheet.Name) + "' has no chart (et.addChart creates one)");
            }
            var names = new List<string>();
            for (int i = 1; i <= count; i++)
            {
                names.Add(Convert.ToString(charts.Item(i).Name));
            }
            string wanted = (ParamString(p, "chart", null) ?? "").Trim();
            int index;
            if (wanted.Length == 0)
            {
                if (count > 1)
                {
                    throw new ArgumentException("the sheet has " + count + " charts - give 'chart': " + string.Join(", ", names));
                }
                index = 1;
            }
            else if (!int.TryParse(wanted, out index))
            {
                index = names.FindIndex(name => string.Equals(name, wanted, StringComparison.CurrentCultureIgnoreCase)) + 1;
            }
            if (index < 1 || index > count)
            {
                throw new ArgumentException("no chart '" + wanted + "' on the sheet: " + string.Join(", ", names));
            }
            dynamic chartObject = charts.Item(index);
            string directory = Path.Combine(Path.GetTempPath(), "AxiomOffice", "charts");
            Directory.CreateDirectory(directory);
            string safe = new string(names[index - 1].Select(c => Path.GetInvalidFileNameChars().Contains(c) ? '_' : c).ToArray());
            string file = Path.Combine(directory, safe + "-" + DateTime.Now.ToString("yyyyMMdd-HHmmss-fff") + ".png");
            // Chart.Export trả true nhưng ghi file 0 byte khi biểu đồ chưa được kích hoạt (đo 28/10/2026 trên Excel
            // 16.0.20527: chart tạo bằng Shapes.AddChart2 rồi export ngay -> 0 byte; Activate() trước thì ra PNG thật).
            bool exported = ExportChartPng(chartObject, file);
            if (!exported || !File.Exists(file) || new FileInfo(file).Length == 0)
            {
                try { if (File.Exists(file)) File.Delete(file); } catch { }
                exported = ExportChartPng(chartObject, file);
            }
            if (!exported || !File.Exists(file) || new FileInfo(file).Length == 0)
            {
                throw new InvalidOperationException("the chart could not be exported as PNG (Excel wrote an empty file)");
            }
            return new Dictionary<string, object>
            {
                { "path", file }, { "chart", names[index - 1] }, { "sheet", Convert.ToString(sheet.Name) },
                { "width", Convert.ToDouble(chartObject.Width) }, { "height", Convert.ToDouble(chartObject.Height) }
            };
        }
    }
}
