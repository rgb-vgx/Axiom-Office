using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // QA cấu trúc (New_arch.md mục 8.4.6, giai đoạn 2): agent "mù", chỉ đọc được text, nên sau khi tạo
    // slide/bảng nó đọc lại bằng các lệnh này để tự soát lỗi trình bày bằng số - tràn chữ, shape chồng,
    // ra ngoài slide, ô trống, dữ liệu lạc ra ngoài bảng (ca Excel ghi rác sang C7), number format.
    // Chỉ đọc, không sửa tài liệu. Mỗi phần đọc COM bọc try riêng: host thiếu thuộc tính (WPS) thì bỏ qua.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> CheckCommands()
        {
            return new[]
            {
                Command("writer.checkTables", "wps", WriterCheckTables,
                    "Soát các bảng (chỉ đọc): số dòng/cột, ô trống, ô tiêu đề lẫn đoạn văn").ForAgent(),
                Command("et.checkRange", "et", EtCheckRange,
                    "Soát bảng dữ liệu (chỉ đọc): tiêu đề trống, kiểu lẫn lộn, số dạng chữ, number format, dữ liệu lạc ngoài bảng",
                    Opt("range", "default: the used range"), Opt("sheet")).ForAgent(),
                Command("wpp.checkLayout", "wpp", WppCheckLayout,
                    "Soát bố cục slide (chỉ đọc): chữ tràn khung, shape ra ngoài slide, shape chồng nhau, chữ quá nhỏ",
                    Opt("slide", "slide number; default: every slide")).ForAgent()
            };
        }

        private static Dictionary<string, object> Issue(string type, string detail, Dictionary<string, object> extra = null)
        {
            var issue = new Dictionary<string, object> { { "type", type }, { "detail", detail } };
            if (extra != null)
            {
                foreach (KeyValuePair<string, object> item in extra)
                {
                    issue[item.Key] = item.Value;
                }
            }
            return issue;
        }

        // ---------------------------------------------------------------- PowerPoint / WPS Presentation

        private const double LayoutTolerance = 2.0;
        private const int MinFontSize = 12;
        private const int DenseSlideChars = 600;

        internal sealed class ShapeBox
        {
            public string Name;
            public double Left;
            public double Top;
            public double Width;
            public double Height;
            public int Chars;
            public double TextHeight;
            public double TextWidth;
            public double MinFont;
        }

        private static Dictionary<string, object> WppCheckLayout(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
            int count = Convert.ToInt32(pres.Slides.Count);
            int only = ParamInt(p, "slide", 0);
            if (only > count || only < 0)
            {
                throw new ArgumentException("'slide' must be between 1 and " + count + ", got " + only);
            }
            double slideWidth = Convert.ToDouble(pres.PageSetup.SlideWidth);
            double slideHeight = Convert.ToDouble(pres.PageSetup.SlideHeight);
            var slides = new List<object>();
            int issueCount = 0;
            for (int i = only > 0 ? only : 1; i <= (only > 0 ? only : count); i++)
            {
                dynamic slide = pres.Slides[i];
                List<ShapeBox> boxes = ReadShapes(slide);
                List<Dictionary<string, object>> issues = LayoutIssues(boxes, slideWidth, slideHeight);
                issueCount += issues.Count;
                slides.Add(new Dictionary<string, object>
                {
                    { "index", i },
                    { "shapes", boxes.Select(b => (object)new Dictionary<string, object>
                        {
                            { "name", b.Name },
                            { "left", Math.Round(b.Left, 1) },
                            { "top", Math.Round(b.Top, 1) },
                            { "width", Math.Round(b.Width, 1) },
                            { "height", Math.Round(b.Height, 1) },
                            { "chars", b.Chars }
                        }).ToList() },
                    { "issues", issues }
                });
            }
            return new Dictionary<string, object>
            {
                { "slideWidth", Math.Round(slideWidth, 1) },
                { "slideHeight", Math.Round(slideHeight, 1) },
                { "issueCount", issueCount },
                { "slides", slides }
            };
        }

        private static List<ShapeBox> ReadShapes(dynamic slide)
        {
            var boxes = new List<ShapeBox>();
            int shapeCount = Convert.ToInt32(slide.Shapes.Count);
            for (int s = 1; s <= shapeCount; s++)
            {
                try
                {
                    dynamic shape = slide.Shapes[s];
                    var box = new ShapeBox
                    {
                        Name = Convert.ToString(shape.Name),
                        Left = Convert.ToDouble(shape.Left),
                        Top = Convert.ToDouble(shape.Top),
                        Width = Convert.ToDouble(shape.Width),
                        Height = Convert.ToDouble(shape.Height),
                        MinFont = 0
                    };
                    try
                    {
                        if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.TextFrame.HasText) == -1)
                        {
                            dynamic text = shape.TextFrame.TextRange;
                            box.Chars = Convert.ToString(text.Text).Length;
                            box.TextHeight = Convert.ToDouble(text.BoundHeight);
                            box.TextWidth = Convert.ToDouble(text.BoundWidth);
                            box.MinFont = MinRunFontSize(text);
                        }
                    }
                    catch (Exception)
                    {
                    }
                    // Placeholder trống của layout không hiện khi trình chiếu: không tính (tránh báo chồng giả).
                    bool emptyPlaceholder = false;
                    try
                    {
                        emptyPlaceholder = box.Chars == 0 && Convert.ToInt32(shape.Type) == 14;
                    }
                    catch (Exception)
                    {
                    }
                    if (!emptyPlaceholder)
                    {
                        boxes.Add(box);
                    }
                }
                catch (Exception)
                {
                }
            }
            return boxes;
        }

        private static double MinRunFontSize(dynamic text)
        {
            double min = 0;
            try
            {
                int runs = Convert.ToInt32(text.Runs().Count);
                for (int r = 1; r <= runs; r++)
                {
                    double size = Convert.ToDouble(text.Runs(r).Font.Size);
                    if (size > 0 && (min == 0 || size < min))
                    {
                        min = size;
                    }
                }
            }
            catch (Exception)
            {
                try
                {
                    min = Convert.ToDouble(text.Font.Size);
                }
                catch (Exception)
                {
                }
            }
            return min;
        }

        internal static List<Dictionary<string, object>> LayoutIssues(IList<ShapeBox> boxes, double slideWidth, double slideHeight)
        {
            var issues = new List<Dictionary<string, object>>();
            int totalChars = 0;
            foreach (ShapeBox b in boxes)
            {
                totalChars += b.Chars;
                if (b.Chars > 0 && (b.TextHeight > b.Height + LayoutTolerance || b.TextWidth > b.Width + LayoutTolerance))
                {
                    issues.Add(Issue("overflow",
                        "text of '" + b.Name + "' needs " + Math.Round(b.TextHeight) + "pt height but the box is " + Math.Round(b.Height)
                        + "pt: shorten the text, lower fontSize or enlarge the box",
                        new Dictionary<string, object> { { "shapes", new[] { b.Name } } }));
                }
                double bottom = b.Top + Math.Max(b.Height, b.TextHeight);
                if (b.Left < -LayoutTolerance || b.Top < -LayoutTolerance
                    || b.Left + b.Width > slideWidth + LayoutTolerance || bottom > slideHeight + LayoutTolerance)
                {
                    issues.Add(Issue("offslide",
                        "'" + b.Name + "' goes outside the slide (" + Math.Round(slideWidth) + "x" + Math.Round(slideHeight) + "pt)",
                        new Dictionary<string, object> { { "shapes", new[] { b.Name } } }));
                }
                if (b.Chars > 0 && b.MinFont > 0 && b.MinFont < MinFontSize)
                {
                    issues.Add(Issue("small-font",
                        "'" + b.Name + "' uses " + b.MinFont + "pt text; keep body text at " + MinFontSize + "pt or more",
                        new Dictionary<string, object> { { "shapes", new[] { b.Name } } }));
                }
            }
            // Shape nền phủ gần cả slide là chủ ý thiết kế: không tính là chồng.
            double slideArea = slideWidth * slideHeight;
            List<ShapeBox> content = boxes.Where(b => b.Width * b.Height < slideArea * 0.9).ToList();
            for (int a = 0; a < content.Count; a++)
            {
                for (int c = a + 1; c < content.Count; c++)
                {
                    double overlap = OverlapArea(content[a], content[c]);
                    double smaller = Math.Min(content[a].Width * content[a].Height, content[c].Width * content[c].Height);
                    if (smaller > 0 && overlap > smaller * 0.1)
                    {
                        issues.Add(Issue("overlap",
                            "'" + content[a].Name + "' and '" + content[c].Name + "' overlap (" + Math.Round(100 * overlap / smaller) + "% of the smaller shape)",
                            new Dictionary<string, object> { { "shapes", new[] { content[a].Name, content[c].Name } } }));
                    }
                }
            }
            if (totalChars > DenseSlideChars)
            {
                issues.Add(Issue("dense", "the slide has " + totalChars + " characters; keep one message per slide and move details to notes (wpp.setNotes)"));
            }
            return issues;
        }

        private static double OverlapArea(ShapeBox a, ShapeBox b)
        {
            double width = Math.Min(a.Left + a.Width, b.Left + b.Width) - Math.Max(a.Left, b.Left);
            double height = Math.Min(a.Top + Math.Max(a.Height, a.TextHeight), b.Top + Math.Max(b.Height, b.TextHeight)) - Math.Max(a.Top, b.Top);
            return width > 0 && height > 0 ? width * height : 0;
        }

        // ---------------------------------------------------------------- Excel / WPS Spreadsheets

        private static Dictionary<string, object> EtCheckRange(IAppHost host, Dictionary<string, object> p)
        {
            string sheetName = ParamString(p, "sheet", null);
            string address = ParamString(p, "range", null);
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            if (wb == null)
            {
                throw new InvalidOperationException("no workbook is open");
            }
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic used = sheet.UsedRange;
            dynamic range = string.IsNullOrEmpty(address) ? used : sheet.Range[address];
            int rows = Convert.ToInt32(range.Rows.Count);
            int cols = Convert.ToInt32(range.Columns.Count);
            object[,] values = AsMatrix(range.Value2, rows, cols);
            object[,] formulas = AsMatrix(range.Formula, rows, cols);
            var formats = new List<string>();
            for (int c = 1; c <= cols; c++)
            {
                string format = "";
                try
                {
                    format = Convert.ToString(range.Cells[Math.Min(2, rows), c].NumberFormat);
                }
                catch (Exception)
                {
                }
                formats.Add(format);
            }

            var issues = new List<Dictionary<string, object>>();
            var columns = ColumnStats(values, formulas, formats, rows, cols, Convert.ToInt32(range.Column), issues);

            // Dữ liệu lạc ra ngoài bảng chính (vùng liền kề của ô đầu): UsedRange rộng hơn CurrentRegion.
            string usedAddress = Convert.ToString(used.Address(false, false));
            string regionAddress = "";
            try
            {
                dynamic region = range.Cells[1, 1].CurrentRegion;
                regionAddress = Convert.ToString(region.Address(false, false));
                int usedCells = Convert.ToInt32(used.Rows.Count) * Convert.ToInt32(used.Columns.Count);
                int regionCells = Convert.ToInt32(region.Rows.Count) * Convert.ToInt32(region.Columns.Count);
                if (string.IsNullOrEmpty(address) && usedCells > regionCells)
                {
                    issues.Add(Issue("outside-table",
                        "the sheet has content outside the table " + regionAddress + " (used range " + usedAddress
                        + "): check for stray values written to the wrong cells"));
                }
            }
            catch (Exception)
            {
            }

            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "range", Convert.ToString(range.Address(false, false)) },
                { "usedRange", usedAddress },
                { "table", regionAddress },
                { "rows", rows },
                { "cols", cols },
                { "columns", columns },
                { "issueCount", issues.Count },
                { "issues", issues }
            };
        }

        private static object[,] AsMatrix(object value, int rows, int cols)
        {
            var matrix = new object[rows, cols];
            var array = value as Array;
            if (array == null)
            {
                matrix[0, 0] = value;
                return matrix;
            }
            int r0 = array.GetLowerBound(0);
            int c0 = array.GetLowerBound(1);
            for (int r = 0; r < rows; r++)
            {
                for (int c = 0; c < cols; c++)
                {
                    matrix[r, c] = array.GetValue(r0 + r, c0 + c);
                }
            }
            return matrix;
        }

        internal static List<object> ColumnStats(object[,] values, object[,] formulas, IList<string> formats, int rows, int cols, int firstColumn,
            List<Dictionary<string, object>> issues)
        {
            var columns = new List<object>();
            for (int c = 0; c < cols; c++)
            {
                string letter = ColumnLetter(firstColumn + c);
                object header = values[0, c];
                int numbers = 0, texts = 0, blanks = 0, numericTexts = 0, errors = 0, formulaCount = 0, fractional = 0;
                for (int r = 1; r < rows; r++)
                {
                    object v = values[r, c];
                    string f = formulas[r, c] as string;
                    if (f != null && f.StartsWith("=", StringComparison.Ordinal))
                    {
                        formulaCount++;
                    }
                    if (v == null || (v is string && ((string)v).Trim().Length == 0))
                    {
                        blanks++;
                    }
                    else if (v is int && (int)v < -2146820000)
                    {
                        errors++;
                    }
                    else if (v is double || v is int || v is decimal)
                    {
                        numbers++;
                        double d = Convert.ToDouble(v, CultureInfo.InvariantCulture);
                        if (Math.Abs(d - Math.Round(d)) > 1e-9)
                        {
                            fractional++;
                        }
                    }
                    else
                    {
                        texts++;
                        double parsed;
                        if (double.TryParse(Convert.ToString(v, CultureInfo.InvariantCulture), NumberStyles.Float, CultureInfo.InvariantCulture, out parsed))
                        {
                            numericTexts++;
                        }
                    }
                }
                string format = c < formats.Count ? formats[c] : "";
                bool headerBlank = header == null || Convert.ToString(header).Trim().Length == 0;
                int dataCells = numbers + texts + errors;
                if (headerBlank && dataCells > 0)
                {
                    issues.Add(Issue("empty-header", "column " + letter + " has data but no header in the first row"));
                }
                if (numericTexts > 0 && numbers + numericTexts >= texts)
                {
                    issues.Add(Issue("numbers-as-text", "column " + letter + " has " + numericTexts + " number(s) stored as text: write them as JSON numbers"));
                }
                else if (numbers > 0 && texts > 0)
                {
                    issues.Add(Issue("mixed-types", "column " + letter + " mixes " + numbers + " number(s) and " + texts + " text value(s)"));
                }
                if (errors > 0)
                {
                    issues.Add(Issue("error-values", "column " + letter + " has " + errors + " error value(s) (#DIV/0!, #REF!...)"));
                }
                if (fractional > 0 && (format.Length == 0 || string.Equals(format, "General", StringComparison.OrdinalIgnoreCase)))
                {
                    issues.Add(Issue("no-number-format", "column " + letter + " has decimals shown with the General format: apply numFmt such as \"0.0\" or \"#,##0.00\""));
                }
                columns.Add(new Dictionary<string, object>
                {
                    { "column", letter },
                    { "header", header },
                    { "numbers", numbers },
                    { "texts", texts },
                    { "blanks", blanks },
                    { "formulas", formulaCount },
                    { "numberFormat", format }
                });
            }
            return columns;
        }

        private static string ColumnLetter(int column)
        {
            string letter = "";
            while (column > 0)
            {
                int rem = (column - 1) % 26;
                letter = (char)('A' + rem) + letter;
                column = (column - 1) / 26;
            }
            return letter;
        }

        // ---------------------------------------------------------------- Word / WPS Writer

        private const int LongHeaderChars = 40;

        private static Dictionary<string, object> WriterCheckTables(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            int count = Convert.ToInt32(doc.Tables.Count);
            var tables = new List<object>();
            var issues = new List<Dictionary<string, object>>();
            for (int t = 1; t <= count; t++)
            {
                dynamic table = doc.Tables[t];
                int rows = Convert.ToInt32(table.Rows.Count);
                int cols = Convert.ToInt32(table.Columns.Count);
                int empty = 0;
                var header = new List<string>();
                dynamic cells = table.Range.Cells;
                int cellCount = Convert.ToInt32(cells.Count);
                for (int i = 1; i <= cellCount; i++)
                {
                    try
                    {
                        dynamic cell = cells[i];
                        string text = CellText(Convert.ToString(cell.Range.Text));
                        if (text.Length == 0)
                        {
                            empty++;
                        }
                        if (Convert.ToInt32(cell.RowIndex) == 1)
                        {
                            header.Add(text);
                        }
                    }
                    catch (Exception)
                    {
                    }
                }
                if (empty > 0)
                {
                    issues.Add(Issue("empty-cells", "table " + t + " has " + empty + " empty cell(s)", new Dictionary<string, object> { { "table", t } }));
                }
                // Ô tiêu đề lẫn cả đoạn văn (log 30/09: dòng ghi chú lọt vào ô "Thứ").
                if (header.Count > 1)
                {
                    double typical = header.Select(h => h.Length).OrderBy(n => n).ElementAt(header.Count / 2);
                    foreach (string cellText in header.Where(h => h.Length > LongHeaderChars && h.Length > 3 * Math.Max(1, typical)))
                    {
                        issues.Add(Issue("long-header-cell",
                            "table " + t + " has a header cell with " + cellText.Length + " characters ('" + Shorten(cellText, 40)
                            + "'): text that should be outside the table may have been typed into it",
                            new Dictionary<string, object> { { "table", t } }));
                    }
                }
                tables.Add(new Dictionary<string, object>
                {
                    { "index", t },
                    { "rows", rows },
                    { "cols", cols },
                    { "emptyCells", empty },
                    { "header", header }
                });
            }
            return new Dictionary<string, object>
            {
                { "tableCount", count },
                { "tables", tables },
                { "issueCount", issues.Count },
                { "issues", issues }
            };
        }

        private static string CellText(string raw)
        {
            return (raw ?? "").TrimEnd('\r', '\a', '\u0007').Trim();
        }

        private static string Shorten(string text, int max)
        {
            return text.Length <= max ? text : text.Substring(0, max) + "...";
        }
    }
}
