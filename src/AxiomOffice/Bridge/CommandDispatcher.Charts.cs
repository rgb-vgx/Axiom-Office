using System;
using System.Collections;
using System.Collections.Generic;
using System.Globalization;
using System.Text;

namespace AxiomOffice.Bridge
{
    // Biểu đồ gốc trên slide (docs/plans/18-s0 mục 9.3: brief biểu đồ tốn nhất và là lý do duy nhất hết giờ - agent vẽ
    // cột bằng hình hoặc ký tự). Số liệu ghi vào bảng tính nhúng của biểu đồ: người dùng "Edit Data" sửa được. Vị trí và
    // màu do Core tính (slides_from_brief, layout chart); model không thấy lệnh này (không ForAgent).
    internal static partial class CommandDispatcher
    {
        private static CommandInfo ChartCommand()
        {
            return Command("wpp.addChart", "wpp", WppAddChart,
                "Biểu đồ gốc (cột, thanh, đường, tròn) từ danh mục và chuỗi số - có nhãn số liệu, sửa được bằng Edit Data",
                Req("categories", "array of labels"), Req("series", "array of {name, values: [numbers]}; pie: one series"),
                Opt("type", "column (default), bar, line, pie"), Opt("unit", "suffix of value labels, e.g. '%' or ' tỷ'"),
                Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height"),
                Opt("colors", "array of #RRGGBB, one per series (pie: per slice)"), Opt("font", "font name"),
                Opt("fontSize", "points (default 16)"), Opt("color", "#RRGGBB text"),
                Opt("presentation", "full path of the presentation to work on; default: the active one"));
        }

        private static Dictionary<string, object> WppAddChart(IAppHost host, Dictionary<string, object> p)
        {
            string kind = (ParamString(p, "type", "column") ?? "column").Trim().ToLowerInvariant();
            int type;
            switch (kind)
            {
                case "column": type = 51; break; // xlColumnClustered
                case "bar": type = 57; break;    // xlBarClustered
                case "line": type = 65; break;   // xlLineMarkers
                case "pie": type = 5; break;     // xlPie
                default: throw new ArgumentException("'type' must be column, bar, line or pie, got " + kind);
            }
            List<string> categories = ParamStrings(p, "categories");
            List<Dictionary<string, object>> series = ParamObjects(p, "series") ?? new List<Dictionary<string, object>>();
            if (categories.Count < 1 || series.Count < 1 || (kind == "pie" && series.Count != 1))
            {
                throw new ArgumentException("need 'categories' and 'series' (" + (kind == "pie" ? "exactly one series for pie" : "one or more") + ")");
            }
            var values = new List<double[]>();
            foreach (Dictionary<string, object> item in series)
            {
                var list = DecodeArrays(item.ContainsKey("values") ? item["values"] : null) as IList;
                if (list == null || list.Count != categories.Count)
                {
                    throw new ArgumentException("every series needs one value per category (" + categories.Count + ")");
                }
                var numbers = new double[list.Count];
                for (int i = 0; i < list.Count; i++)
                {
                    numbers[i] = Convert.ToDouble(UnwrapScalar(list[i]), CultureInfo.InvariantCulture);
                }
                values.Add(numbers);
            }
            string unit = ParamString(p, "unit", "") ?? "";
            if (unit.IndexOf('"') >= 0 || unit.IndexOf('\\') >= 0)
            {
                throw new ArgumentException("'unit' must not contain quotes or backslashes");
            }
            // Định dạng số của Excel: "General" giữ số như người dùng nhập, đơn vị là chuỗi hằng (không nhân 100 như %).
            string format = unit.Length == 0 ? "General" : "General\"" + unit + "\"";

            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            // AddChart2 (PowerPoint 2013+): style -1 = mặc định của kiểu; mở sẵn một bảng tính nhúng có dữ liệu mẫu.
            dynamic shape = slide.Shapes.AddChart2(-1, type, ParamDouble(p, "left", 40), ParamDouble(p, "top", 130),
                ParamDouble(p, "width", 880), ParamDouble(p, "height", 370), true);
            dynamic chart = shape.Chart;
            FillChartData(chart, categories, series, values, format);

            chart.HasTitle = false; // tiêu đề slide nói kết luận, không lặp trong biểu đồ
            chart.HasLegend = kind == "pie" ? false : series.Count > 1;
            if (Convert.ToBoolean(chart.HasLegend))
            {
                chart.Legend.Position = -4107; // xlLegendPositionBottom
            }
            List<string> colors = ParamStrings(p, "colors");
            int seriesCount = Convert.ToInt32(chart.SeriesCollection().Count);
            for (int s = 1; s <= seriesCount; s++)
            {
                dynamic one = chart.SeriesCollection(s);
                if (kind == "pie")
                {
                    for (int point = 1; point <= categories.Count && colors.Count > 0; point++)
                    {
                        SetChartFill(one.Points(point), colors[(point - 1) % colors.Count], false);
                    }
                }
                else if (colors.Count > 0)
                {
                    SetChartFill(one, colors[(s - 1) % colors.Count], kind == "line");
                }
                one.HasDataLabels = true;
                dynamic labels = one.DataLabels();
                labels.NumberFormat = format;
                if (kind == "pie")
                {
                    labels.ShowCategoryName = true;
                    labels.ShowValue = true;
                    labels.Separator = "\n";
                    labels.Position = 2; // xlLabelPositionOutsideEnd: chữ đậm không nằm trên lát màu đậm
                }
                else
                {
                    labels.Position = kind == "line" ? 0 : 2; // trên điểm / ngoài đầu cột
                }
            }
            if (kind != "pie")
            {
                // Nhãn số đã nói giá trị: bỏ lưới và trục giá trị cho gọn (skill: một thông điệp mỗi slide).
                try
                {
                    dynamic axis = chart.Axes(2);
                    axis.HasMajorGridlines = false;
                    axis.Delete();
                }
                catch (Exception)
                {
                }
                try
                {
                    chart.ChartGroups(1).GapWidth = 80;
                }
                catch (Exception)
                {
                }
            }
            StyleChartText(chart, ParamString(p, "font", null), ParamDouble(p, "fontSize", 16), ParseBgrColor(ParamString(p, "color", "#262626")));
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(shape.Name) }
            };
        }

        // Ghi dữ liệu vào bảng tính nhúng (A1 trống, hàng 1 tên chuỗi, cột A danh mục) rồi trỏ biểu đồ vào đúng vùng.
        private static void FillChartData(dynamic chart, List<string> categories, List<Dictionary<string, object>> series,
            List<double[]> values, string format)
        {
            dynamic data = chart.ChartData;
            try
            {
                data.ActivateChartDataWindow(); // cửa sổ dữ liệu nhỏ, không mở Excel đầy đủ
            }
            catch (Exception)
            {
                data.Activate();
            }
            dynamic workbook = data.Workbook;
            try
            {
                dynamic sheet = workbook.Worksheets(1);
                sheet.Range("A1:Z100").ClearContents();
                int rows = categories.Count + 1, cols = series.Count + 1;
                var cells = new object[rows, cols];
                cells[0, 0] = "";
                for (int s = 0; s < series.Count; s++)
                {
                    cells[0, s + 1] = ParamString(series[s], "name", "Series " + (s + 1));
                }
                for (int c = 0; c < categories.Count; c++)
                {
                    cells[c + 1, 0] = categories[c];
                    for (int s = 0; s < series.Count; s++)
                    {
                        cells[c + 1, s + 1] = values[s][c];
                    }
                }
                dynamic range = sheet.Range("A1").Resize(rows, cols);
                // Danh mục là chữ ("2024" không thành một chuỗi số), số mang đơn vị trong định dạng.
                sheet.Range("A1").Resize(rows, 1).NumberFormat = "@";
                range.Value2 = cells;
                sheet.Range("B2").Resize(rows - 1, cols - 1).NumberFormat = format;
                try
                {
                    sheet.ListObjects(1).Resize(range);
                }
                catch (Exception)
                {
                }
                string address = "='" + Convert.ToString(sheet.Name) + "'!$A$1:$" + (char)('A' + cols - 1) + "$" + rows;
                chart.SetSourceData(address, 2); // xlColumns: mỗi cột một chuỗi
            }
            finally
            {
                try
                {
                    workbook.Close();
                }
                catch (Exception)
                {
                }
            }
        }

        private static void SetChartFill(dynamic target, string hex, bool line)
        {
            int? color = ParseBgrColor(hex);
            if (!color.HasValue)
            {
                return;
            }
            if (line)
            {
                target.Format.Line.ForeColor.RGB = color.Value;
                target.MarkerBackgroundColor = color.Value;
                target.MarkerForegroundColor = color.Value;
                return;
            }
            target.Format.Fill.Visible = -1;
            target.Format.Fill.Solid();
            target.Format.Fill.ForeColor.RGB = color.Value;
        }

        private static void StyleChartText(dynamic chart, string font, double size, int? color)
        {
            try
            {
                dynamic text = chart.ChartArea.Format.TextFrame2.TextRange.Font;
                text.Size = (float)size;
                if (!string.IsNullOrWhiteSpace(font))
                {
                    text.Name = font;
                }
                if (color.HasValue)
                {
                    text.Fill.ForeColor.RGB = color.Value;
                }
            }
            catch (Exception)
            {
            }
        }

        // Chữ của biểu đồ cho wpp.listSlides: tiêu đề, rồi mỗi chuỗi "tên: danh mục giá trị+đơn vị" - agent soát được số
        // liệu đã vẽ (trước chỉ thấy tiêu đề; docs/plans/19-h0 H3).
        private static string ChartText(dynamic chart)
        {
            var text = new StringBuilder("[chart]");
            if (ComFlag(() => chart.HasTitle))
            {
                text.Append(' ').Append(Convert.ToString(chart.ChartTitle.Text));
            }
            try
            {
                AppendSeriesText(text, chart);
            }
            catch (Exception)
            {
                // Biểu đồ không đọc được chuỗi (kiểu lạ, liên kết hỏng): vẫn trả phần đã đọc, không bỏ cả slide.
            }
            return text.ToString();
        }

        private static void AppendSeriesText(StringBuilder text, dynamic chart)
        {
            int count = Convert.ToInt32(chart.SeriesCollection().Count);
            for (int s = 1; s <= count; s++)
            {
                dynamic series = chart.SeriesCollection(s);
                var names = series.XValues as Array;
                var numbers = series.Values as Array;
                string format = ComFlag(() => series.HasDataLabels) ? Convert.ToString(series.DataLabels().NumberFormat) : "General";
                text.Append(" | ").Append(Convert.ToString(series.Name)).Append(':');
                if (numbers == null)
                {
                    continue;
                }
                int index = 0;
                foreach (object value in numbers)
                {
                    object name = names != null && index < names.Length ? names.GetValue(names.GetLowerBound(0) + index) : null;
                    text.Append(index == 0 ? " " : ", ").Append(Convert.ToString(name, CultureInfo.InvariantCulture)).Append(' ')
                        .Append(FormatChartValue(value, format));
                    index++;
                }
            }
        }

        // Giá trị như nhãn hiển thị: chuỗi hằng trong ngoặc kép là đơn vị; dấu % ngoài ngoặc thì nhân 100.
        internal static string FormatChartValue(object value, string format)
        {
            double number;
            try
            {
                number = Convert.ToDouble(value, CultureInfo.InvariantCulture);
            }
            catch (Exception)
            {
                return Convert.ToString(value, CultureInfo.InvariantCulture);
            }
            var suffix = new StringBuilder();
            bool quoted = false, percent = false;
            foreach (char ch in format ?? "")
            {
                if (ch == '"')
                {
                    quoted = !quoted;
                }
                else if (quoted)
                {
                    suffix.Append(ch);
                }
                else if (ch == '%')
                {
                    percent = true;
                }
            }
            if (percent)
            {
                number *= 100;
                suffix.Append('%');
            }
            return Math.Round(number, 2).ToString("0.##", CultureInfo.InvariantCulture) + suffix;
        }

        private static List<string> ParamStrings(Dictionary<string, object> p, string name)
        {
            var items = new List<string>();
            var list = DecodeArrays(p != null && p.ContainsKey(name) ? p[name] : null) as IList;
            if (list == null)
            {
                return items;
            }
            foreach (object item in list)
            {
                items.Add(Convert.ToString(UnwrapScalar(item), CultureInfo.InvariantCulture));
            }
            return items;
        }
    }
}
