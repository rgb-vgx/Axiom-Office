using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text.RegularExpressions;

namespace AxiomOffice.Bridge
{
    // QA cấu trúc (docs/architecture/core.md mục 8.4.6, giai đoạn 2): agent "mù", chỉ đọc được text, nên sau khi tạo
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
                    "Soát các bảng (chỉ đọc): số dòng/cột, ô trống, ô tiêu đề lẫn đoạn văn. Bảng dàn trang — bảng đã gọi "
                    + "writer.formatTable với borders:false — được đánh dấu layout và KHÔNG báo empty-cells (ô trống ở đó "
                    + "là cố ý: khoảng trống ký, ô chờ điền)").ForAgent(),
                Command("et.checkRange", "et", EtCheckRange,
                    "Soát bảng dữ liệu (chỉ đọc): tiêu đề trống, kiểu lẫn lộn, số dạng chữ, number format, công thức trỏ ô trống; không truyền range/sheet thì soát thêm dữ liệu lạc ngoài bảng và ô lỗi (#DIV/0!, #REF!...) ở MỌI sheet",
                    Opt("range", "default: the used range"), Opt("sheet")).ForAgent(),
                Command("wpp.checkLayout", "wpp", WppCheckLayout,
                    "Soát bố cục slide (chỉ đọc): chữ tràn khung, shape ra ngoài slide, shape chồng nhau, chữ quá nhỏ, "
                    + "đường nối cắt qua hộp khác hoặc chồng lên đường nối khác",
                    Opt("slide", "slide number; default: every slide"), Opt("presentation", "full path of the presentation to work on; default: the active one")).ForAgent()
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
            // Mép trái của chữ (BoundLeft); NaN = không đọc được. Khung của mẫu có thể rộng quá mép slide
            // mà chữ căn giữa vẫn nằm trong slide ("TextBox 3" của mẫu trường: 45 + 938 > 960).
            public double TextLeft = double.NaN;
            // MsoShapeType (1 hình vẽ, 13 ảnh, 17 textbox...); 0 = không rõ.
            public int Type;
            public double MinFont;
            // Đường nối (Shape.Connector): là đường, không có diện tích - khung bao của đường gấp khúc trùm lên các hộp
            // nó nối, không phải chồng.
            public bool Connector;
            // Đường nối: đường đi thật (toạ độ slide) và tên hai hình nó bám vào (null = đầu tự do).
            public List<double[]> Path;
            public string BeginShape;
            public string EndShape;
        }

        private static Dictionary<string, object> WppCheckLayout(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
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
                    { "shapes", boxes.Select(b => (object)ShapeEntry(b)).ToList() },
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

        private static Dictionary<string, object> ShapeEntry(ShapeBox b)
        {
            var entry = new Dictionary<string, object>
            {
                { "name", b.Name },
                { "left", Math.Round(b.Left, 1) },
                { "top", Math.Round(b.Top, 1) },
                { "width", Math.Round(b.Width, 1) },
                { "height", Math.Round(b.Height, 1) },
                { "chars", b.Chars }
            };
            // Đường nối: đường đi thật (để soát/đo) và hai hình nó nối.
            if (b.Connector && b.Path != null)
            {
                entry["path"] = b.Path.Select(point => (object)new[] { Math.Round(point[0], 1), Math.Round(point[1], 1) }).ToList();
                entry["from"] = b.BeginShape;
                entry["to"] = b.EndShape;
            }
            return entry;
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
                    box.Connector = ComFlag(() => shape.Connector);
                    if (box.Connector)
                    {
                        ReadConnector(shape, box);
                    }
                    RotateBox(box, ComDouble(() => shape.Rotation));
                    try
                    {
                        if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.TextFrame.HasText) == -1)
                        {
                            dynamic text = shape.TextFrame.TextRange;
                            box.Chars = Convert.ToString(text.Text).Length;
                            box.TextHeight = Convert.ToDouble(text.BoundHeight);
                            box.TextWidth = Convert.ToDouble(text.BoundWidth);
                            box.TextLeft = Convert.ToDouble(text.BoundLeft);
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
                        box.Type = Convert.ToInt32(shape.Type);
                        emptyPlaceholder = box.Chars == 0 && box.Type == 14;
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
                        + "pt: shorten the text, move details to notes (wpp.setNotes) or split into two slides; keep fontSize at the design size",
                        new Dictionary<string, object> { { "shapes", new[] { b.Name } } }));
                }
                double bottom = b.Top + Math.Max(b.Height, b.TextHeight);
                // Khung chữ: xét mép chữ thật theo chiều ngang (khung của mẫu có thể rộng quá mép slide).
                bool textBounds = b.Chars > 0 && !double.IsNaN(b.TextLeft) && b.TextWidth > 0;
                double left = textBounds ? b.TextLeft : b.Left;
                double right = textBounds ? b.TextLeft + b.TextWidth : b.Left + b.Width;
                if (!IsBackground(b, slideWidth, slideHeight)
                    && (left < -LayoutTolerance || b.Top < -LayoutTolerance
                        || right > slideWidth + LayoutTolerance || bottom > slideHeight + LayoutTolerance))
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
            List<ShapeBox> content = boxes.Where(b => b.Width * b.Height < slideArea * 0.9 && !b.Connector).ToList();
            var spilled = new HashSet<string>();
            for (int a = 0; a < content.Count; a++)
            {
                for (int c = a + 1; c < content.Count; c++)
                {
                    double overlap = OverlapArea(content[a], content[c]);
                    LayeringKind layering = Layering(content[a], content[c], content, overlap);
                    if (layering == LayeringKind.Intended)
                    {
                        continue;
                    }
                    if (layering == LayeringKind.Spill)
                    {
                        ShapeBox text = content[a].Chars > 0 ? content[a] : content[c];
                        ShapeBox under = text == content[a] ? content[c] : content[a];
                        if (spilled.Add(text.Name))
                        {
                            issues.Add(Issue("overflow",
                                "text of '" + text.Name + "' runs out of '" + under.Name + "', the shape it sits on: shorten the text "
                                + "(maxChars of the slot in ppt_profile kinds) or move details to notes; keep fontSize at the design size",
                                new Dictionary<string, object> { { "shapes", new[] { text.Name, under.Name } } }));
                        }
                        continue;
                    }
                    double smaller = Math.Min(content[a].Width * content[a].Height, content[c].Width * content[c].Height);
                    if (smaller > 0 && overlap > smaller * 0.1)
                    {
                        issues.Add(Issue("overlap",
                            "'" + content[a].Name + "' and '" + content[c].Name + "' overlap (" + Math.Round(100 * overlap / smaller) + "% of the smaller shape)",
                            new Dictionary<string, object> { { "shapes", new[] { content[a].Name, content[c].Name } } }));
                    }
                }
            }
            ConnectorIssues(boxes, slideWidth, slideHeight, issues);
            if (totalChars > DenseSlideChars)
            {
                issues.Add(Issue("dense", "the slide has " + totalChars + " characters; keep one message per slide and move details to notes (wpp.setNotes)"));
            }
            return issues;
        }

        // Left/Top/Width/Height của PowerPoint là khung TRƯỚC khi xoay: xoay 90/270 độ (đường nối gấp khúc đi xuống hay
        // dùng) thì khung thật đổi rộng-cao quanh tâm. Góc khác: giữ nguyên (gần đúng).
        internal static void RotateBox(ShapeBox box, double rotation)
        {
            double turn = ((rotation % 180) + 180) % 180;
            if (Math.Abs(turn - 90) > 1)
            {
                return;
            }
            double centerX = box.Left + box.Width / 2, centerY = box.Top + box.Height / 2;
            double width = box.Width;
            box.Width = box.Height;
            box.Height = width;
            box.Left = centerX - box.Width / 2;
            box.Top = centerY - box.Height / 2;
        }

        // Ảnh nền của mẫu (logo, dải màu) phủ gần cả slide, có thể lệch vài pt ra ngoài mép: không phải lỗi.
        private static bool IsBackground(ShapeBox b, double slideWidth, double slideHeight)
        {
            return b.Type == 13 && b.Chars == 0 && b.Width * b.Height >= slideWidth * slideHeight * 0.9;
        }

        // Hình trang trí không chữ (hình vẽ, đường, textbox rỗng).
        private static bool IsDecoration(ShapeBox b)
        {
            return b.Chars == 0 && (b.Type == 1 || b.Type == 5 || b.Type == 9 || b.Type == 17);
        }

        private enum LayeringKind
        {
            Overlap,
            Intended,
            Spill
        }

        // Chồng có chủ ý của mẫu (đo trên mẫu trường 13/10/2026: mỗi mục I/II/III = ô màu + bóng xám + vòng tròn
        // + hai khung chữ đặt trên): hai hình trang trí chồng nhau, hoặc khung chữ nằm GỌN trong một hình trang trí
        // nào đó (nhãn - chồng lên hình trang trí khác bên cạnh cũng là chủ ý). Khung chữ đè phần lớn lên hình
        // trang trí mà không còn gọn trong hình nào = chữ dài làm khung giãn ra khỏi ô màu (Spill).
        private static LayeringKind Layering(ShapeBox a, ShapeBox b, IList<ShapeBox> all, double overlap)
        {
            if (IsDecoration(a) && IsDecoration(b))
            {
                return LayeringKind.Intended;
            }
            ShapeBox decoration = IsDecoration(a) ? a : (IsDecoration(b) ? b : null);
            ShapeBox text = decoration == a ? b : a;
            if (decoration == null || text.Chars == 0)
            {
                return LayeringKind.Overlap;
            }
            if (all.Any(d => IsDecoration(d) && Contains(d, text)))
            {
                return LayeringKind.Intended;
            }
            return overlap >= 0.5 * text.Width * Math.Max(text.Height, text.TextHeight) ? LayeringKind.Spill : LayeringKind.Overlap;
        }

        private static bool Contains(ShapeBox outer, ShapeBox inner)
        {
            double innerBottom = inner.Top + Math.Max(inner.Height, inner.TextHeight);
            return inner.Left >= outer.Left - LayoutTolerance && inner.Top >= outer.Top - LayoutTolerance
                && inner.Left + inner.Width <= outer.Left + outer.Width + LayoutTolerance
                && innerBottom <= outer.Top + outer.Height + LayoutTolerance;
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

            try
            {
                var empty = EmptyReferences(wb, sheet, formulas, rows, cols);
                if (empty.Count > 0)
                {
                    issues.Add(new Dictionary<string, object>
                    {
                        { "type", "empty-reference" },
                        { "detail", empty.Count + " công thức trỏ vào một ô đang TRỐNG (ví dụ: " +
                            string.Join("; ", empty.Take(6)) + "). Ô trống được tính là 0, nên một ô điều khiển bị " +
                            "trỏ sai chỗ sẽ im lặng chạy bằng 0 - hãy kiểm tra xem có phải công thức đang trỏ vào ô " +
                            "tiêu đề / ô bên cạnh thay vì ô giá trị không" },
                        { "examples", empty.Take(6).ToList() }
                    });
                }
            }
            catch (Exception)
            {
                // soát thêm không được làm hỏng cả báo cáo
            }

            // Không chỉ định vùng/sheet = soát cả sổ: ô lỗi (#DIV/0!, #REF!...) ở MỌI sheet, kể cả sheet đang không mở (plan 21 P2).
            if (string.IsNullOrEmpty(address) && string.IsNullOrEmpty(sheetName))
            {
                try
                {
                    List<string> errorCells = WorkbookErrorCells(wb);
                    if (errorCells.Count > 0)
                    {
                        issues.Add(Issue("error-cells", errorCells.Count + (errorCells.Count >= ErrorCellLimit ? "+" : "")
                            + " formula cell(s) in the workbook show an error: " + string.Join("; ", errorCells.Take(12))
                            + " - fix the formula (#DIV/0! = divide by zero/empty, #REF! = deleted reference, #NAME? = typo, #VALUE! = text in maths)",
                            new Dictionary<string, object> { { "cells", errorCells } }));
                    }
                }
                catch (Exception)
                {
                }
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

        private const int ErrorCellLimit = 40;

        // Ô công thức đang ra lỗi trên mọi sheet ("Sheet!D4 #DIV/0!"). SpecialCells(xlCellTypeFormulas = -4123, xlErrors = 16)
        // ném lỗi khi sheet không có ô nào như vậy.
        private static List<string> WorkbookErrorCells(dynamic wb)
        {
            var found = new List<string>();
            foreach (dynamic ws in wb.Worksheets)
            {
                dynamic cells;
                try
                {
                    cells = ws.UsedRange.SpecialCells(-4123, 16);
                }
                catch (Exception)
                {
                    continue;
                }
                foreach (dynamic cell in cells.Cells)
                {
                    string name = "#ERR";
                    try
                    {
                        name = ExcelErrorName(Convert.ToInt32(cell.Value2));
                    }
                    catch (Exception)
                    {
                    }
                    found.Add(Convert.ToString(ws.Name) + "!" + Convert.ToString(cell.Address(false, false)) + " " + name);
                    if (found.Count >= ErrorCellLimit)
                    {
                        return found;
                    }
                }
            }
            return found;
        }

        // Giới hạn khi soi tham chiếu: bảng 50.000 dòng có thể có hàng trăm nghìn công thức, soi hết thì
        // chậm mà cái cần thấy chỉ là vài chỗ. Lấy mẫu, và báo cả con số tổng để người đọc biết quy mô.
        private const int ReferenceScanLimit = 4000;
        private const int ReferenceLookupLimit = 60;

        // Tham chiếu tới MỘT ô trong cú pháp Excel: có thể có tên sheet phía trước (Inputs!B34,
        // 'My Sheet'!$B$34), và có thể có $.
        // Nhóm có tên của .NET là (?<ten>...), KHÔNG phải (?P<ten>...) như Python/PCRE. Viết sai thì regex
        // chỉ nổ lúc CHẠY (khởi tạo static), và cả lớp CommandDispatcher không dựng được -> Host.exe chết
        // ngay khi gọi - biên dịch vẫn xanh.
        private static readonly Regex CellReference = new Regex(
            @"(?:(?<sheet>'[^']+'|[A-Za-z_][A-Za-z0-9_ ]*)!)?(?<abs1>\$?)(?<col>[A-Za-z]{1,3})(?<abs2>\$?)(?<row>\d+)",
            RegexOptions.Compiled);

        /// <summary>
        /// Ví dụ về công thức trỏ vào MỘT ô đang TRỐNG.
        ///
        /// Vì sao cần: một ô điều khiển đặt cạnh nhãn của nó (vd "Random Seed" ở B17) nhưng công thức lỡ
        /// trỏ vào ô bên cạnh hoặc ô trên (B16/B34) thì không có gì báo lỗi - phép tính coi ô trống là 0 và
        /// chạy tiếp. Đo ngày 02/10/2026 trên đề Monte Carlo: một workbook 10.000 đường tự báo 18/18 PASS
        /// trong khi seed thật sự bằng 0, vì 80.008 công thức đọc ô tiêu đề mục thay vì ô giá trị. Đọc lại
        /// KHÔNG phát hiện được; phải soi xem công thức trỏ vào đâu.
        /// </summary>
        private static List<string> EmptyReferences(dynamic wb, dynamic sheet, object[,] formulas, int rows, int cols)
        {
            var found = new List<string>();
            var seen = new HashSet<string>(StringComparer.Ordinal);
            int scanned = 0;
            int lookups = 0;
            for (int r = 0; r < rows && scanned < ReferenceScanLimit && lookups < ReferenceLookupLimit; r++)
            {
                for (int c = 0; c < cols && scanned < ReferenceScanLimit && lookups < ReferenceLookupLimit; c++)
                {
                    string formula = formulas[r, c] as string;
                    if (string.IsNullOrEmpty(formula) || !formula.StartsWith("="))
                    {
                        continue;
                    }
                    scanned++;
                    foreach (Match match in CellReference.Matches(formula))
                    {
                        int start = match.Index;
                        int end = match.Index + match.Length;
                        // A1:B2 - cả hai đầu đều là tham chiếu hợp lệ, không phải trỏ nhầm.
                        if ((start > 0 && formula[start - 1] == ':') || (end < formula.Length && formula[end] == ':'))
                        {
                            continue;
                        }
                        dynamic targetSheet = sheet;
                        string name = match.Groups["sheet"].Success ? match.Groups["sheet"].Value.Trim('\'') : null;
                        if (!string.IsNullOrEmpty(name))
                        {
                            try
                            {
                                targetSheet = wb.Worksheets[name];
                            }
                            catch (Exception)
                            {
                                continue;
                            }
                            if (targetSheet == null)
                            {
                                continue;
                            }
                        }
                        int column = ColumnIndex(match.Groups["col"].Value);
                        int row = Convert.ToInt32(match.Groups["row"].Value);
                        string label = Convert.ToString(targetSheet.Name) + "!" + ColumnName(column) + row;
                        if (!seen.Add(label))
                        {
                            continue;
                        }
                        lookups++;
                        try
                        {
                            dynamic target = targetSheet.Cells[row, column];
                            if (Convert.ToString(target.Formula) != "" || target.Value2 != null)
                            {
                                continue;   // có nội dung thì không phải trỏ nhầm
                            }
                        }
                        catch (Exception)
                        {
                            continue;
                        }
                        found.Add(Convert.ToString(sheet.Name) + "!" + ColumnName(c + 1) + (r + 1) + " -> " + label + " (trống)");
                        if (found.Count >= 6)
                        {
                            return found;
                        }
                    }
                }
            }
            return found;
        }

        private static int ColumnIndex(string letters)
        {
            int value = 0;
            foreach (char ch in letters.ToUpperInvariant())
            {
                value = value * 26 + (ch - 'A' + 1);
            }
            return value;
        }

        private static string ColumnName(int column)
        {
            string name = "";
            while (column > 0)
            {
                int rem = (column - 1) % 26;
                name = (char)('A' + rem) + name;
                column = (column - 1) / 26;
            }
            return name;
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

        /// <summary>Mã lỗi Excel (Value2 trả về số nguyên âm) -> tên hiển thị trên ô. Lạ thì trả mã thô.</summary>
        internal static string ExcelErrorName(int code)
        {
            switch (code)
            {
                case -2146826288: return "#NULL!";
                case -2146826281: return "#DIV/0!";
                case -2146826273: return "#VALUE!";
                case -2146826265: return "#REF!";
                case -2146826259: return "#NAME?";
                case -2146826252: return "#NUM!";
                case -2146826246: return "#N/A";
                default: return "#ERR" + code.ToString(CultureInfo.InvariantCulture);
            }
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
                var errorNames = new List<string>();
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
                        errorNames.Add(ExcelErrorName((int)v));
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
                    // Mã lỗi THẬT (#DIV/0! = mẫu bằng 0, #REF! = tham chiếu hỏng...), không liệt kê ví dụ nữa:
                    // đoán mò làm người đọc đi tìm sai bệnh.
                    issues.Add(Issue("error-values", "column " + letter + " has " + errors + " error value(s): "
                        + string.Join(", ", new SortedSet<string>(errorNames))));
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
                // Ô trống ở bảng dàn trang là CỐ Ý (khoảng trống ký, ô chờ điền) nên không báo - cùng quy tắc với
                // LibreOffice (checks.py) và WPS. "long-header-cell" vẫn báo cho mọi bảng.
                bool layout = IsLayoutTable(table);
                if (empty > 0 && !layout)
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
                    { "header", header },
                    { "layout", layout }
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
