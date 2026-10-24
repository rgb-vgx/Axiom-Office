using System;
using System.Collections;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;

namespace AxiomOffice.Bridge
{
    // Trình bày lại tài liệu Word (14/10/2026, người dùng duyệt mẫu giáo án "GA HG - trinh bay moi"): nền chung
    // (writer.restyle) + khối bo tròn nền nhạt cho chỗ cần nhấn (writer.insertBlocks), không icon. Agent Core quyết định
    // khối nào đặt ở đâu (tool doc_design); bridge dựng theo mẫu cố định để model yếu không làm vỡ bố cục.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> WriterDesignCommands()
        {
            return new[]
            {
                Command("writer.outline", "wps", WriterOutline,
                    "Dàn bài tài liệu: các đoạn (số thứ tự toàn tài liệu, chữ rút gọn, số tự động) và bảng (số hàng, cột, hàng tiêu đề)",
                    Opt("maxChars", "characters per paragraph (default 160)")).ForAgent(),
                Command("writer.restyle", "wps", WriterRestyle,
                    "Trình bày lại nền chung KHÔNG đổi chữ: tiêu đề mục (I., A., *), tiêu đề con (1.), nhãn (a. ...:), gạch đầu dòng hai cấp, bỏ dòng trống thừa, bảng (viền mảnh, hàng tiêu đề nhạt lặp lại), lề trang, số trang",
                    Opt("accent", "#RRGGBB headings (default #2E5C8A)"), Opt("font", "default Times New Roman"), Opt("size", "body size (default 13)")),
                Command("writer.insertBlocks", "wps", WriterInsertBlocks,
                    "Chèn khối bo tròn nền nhạt (ô ghi chú, cảnh báo, thẻ cạnh nhau) - thay các đoạn [replaceFrom, replaceTo] hoặc chèn trước đoạn `before`",
                    Req("blocks", "array of {before | replaceFrom, replaceTo, kind: box|row, gap, after, cells: [{tone, lines: [{k: title|bullet|text|small|center, t, size, bold}]}]}"),
                    Opt("accent"), Opt("font"))
            };
        }

        private const string DesignAccent = "#2E5C8A";
        private const string DesignText = "#333333";
        private const string DesignMuted = "#6B7280";

        // (nền, viền, chữ nhấn) của từng tông - đúng mẫu đã duyệt.
        private static readonly Dictionary<string, string[]> DesignTones = new Dictionary<string, string[]>
        {
            { "blue", new[] { "F3F7FB", "D5E3F0", "2E5C8A" } },
            { "title", new[] { "E8F0F8", "C9DAEC", "2E5C8A" } },
            { "violet", new[] { "F6F3FB", "E1D8F0", "5B4A8A" } },
            { "amber", new[] { "FFF8EC", "F2DDB4", "9A6A12" } },
            { "red", new[] { "FDF3F2", "F2D3CF", "A8443A" } },
            { "green", new[] { "F1F8F3", "CFE6D6", "2F7A4A" } },
            { "gray", new[] { "F7F8FA", "E3E7EC", "4B5563" } }
        };

        // ---------- writer.outline ----------
        private static Dictionary<string, object> WriterOutline(IAppHost host, Dictionary<string, object> p)
        {
            int maxChars = Math.Max(40, ParamInt(p, "maxChars", 160));
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            var items = new List<object>();
            var tablesSeen = new HashSet<int>();
            var tables = TableSpans(doc);
            int index = 0;
            foreach (dynamic paragraph in doc.Paragraphs)
            {
                index++;
                dynamic range = paragraph.Range;
                int tableIndex = TableAt(tables, Convert.ToInt32(range.Start));
                if (tableIndex >= 0)
                {
                    dynamic table = tables[tableIndex];
                    if (tablesSeen.Add(tableIndex))
                    {
                        var header = new List<string>();
                        try
                        {
                            foreach (dynamic cell in table.Rows[1].Cells)
                            {
                                header.Add(CleanCellText(Convert.ToString(cell.Range.Text), 40));
                            }
                        }
                        catch
                        {
                        }
                        string caption = TableCaption(doc, table);
                        var tentry = new System.Collections.Generic.Dictionary<string, object>
                        {
                            { "n", index }, { "kind", "table" }, { "rows", Convert.ToInt32(table.Rows.Count) },
                            { "cols", Convert.ToInt32(table.Columns.Count) }, { "header", header }
                        };
                        if (!string.IsNullOrEmpty(caption)) tentry["caption"] = caption;
                        items.Add(tentry);
                    }
                    continue;
                }
                string text = Convert.ToString(range.Text).TrimEnd('\r', '\a').Trim();
                bool hasShape = false;
                try
                {
                    hasShape = Convert.ToInt32(range.InlineShapes.Count) > 0 || Convert.ToInt32(range.ShapeRange.Count) > 0;
                }
                catch
                {
                }
                if (text.Length == 0 && !hasShape)
                {
                    continue;
                }
                var entry = new Dictionary<string, object> { { "n", index }, { "t", text.Length > maxChars ? text.Substring(0, maxChars) + "…" : text } };
                string list = "";
                try
                {
                    list = Convert.ToString(range.ListFormat.ListString);
                }
                catch
                {
                }
                if (!string.IsNullOrEmpty(list))
                {
                    entry["list"] = list;
                }
                try
                {
                    var marks = new System.Collections.Generic.List<string>();
                    foreach (dynamic mark in range.Bookmarks)
                    {
                        try { marks.Add(System.Convert.ToString(mark.Name)); }
                        catch { }
                    }
                    if (marks.Count > 0) entry["bookmarks"] = marks;
                }
                catch
                {
                }
                try
                {
                    string style = Convert.ToString(paragraph.Style.NameLocal);
                    if (!string.IsNullOrEmpty(style) && style != "Normal" && style != "Mặc định")
                        entry["style"] = style;
                }
                catch
                {
                }
                if (hasShape && text.Trim('/', ' ').Length == 0)
                {
                    string label = "[ảnh]";
                    try
                    {
                        foreach (dynamic shape in range.ShapeRange)
                        {
                            label = ShapeLabel(shape);
                            break;
                        }
                    }
                    catch
                    {
                    }
                    entry["t"] = label.Length > maxChars ? label.Substring(0, maxChars) + "…" : label;
                }
                items.Add(entry);
            }
            return new Dictionary<string, object> { { "paragraphs", index }, { "items", items } };
        }

        // Các bảng cấp ngoài, giữ Range (Word tự dời Start/End khi chữ phía trên đổi). Range.Information(wdWithInTable)
        // bắt Word dàn trang lại ở mỗi đoạn (bản thử 14/10/2026: restyle 285 đoạn mất ~100 s).
        // P0-03 (16/10/2026): caption bang = doan Caption/Bang/Chu thich ke truoc bang.
        private static string TableCaption(dynamic doc, dynamic table)
        {
            try
            {
                int start = System.Convert.ToInt32(table.Range.Start);
                string last = null;
                foreach (dynamic paragraph in doc.Paragraphs)
                {
                    int ps;
                    try { ps = System.Convert.ToInt32(paragraph.Range.Start); }
                    catch { continue; }
                    if (ps >= start) break;
                    string style;
                    try { style = System.Convert.ToString(paragraph.Style.NameLocal); }
                    catch { continue; }
                    if (style == null) continue;
                    string slow = style.ToLower();
                    if (slow.IndexOf("caption") < 0 && slow.IndexOf("bang") < 0) continue;
                    try { last = (System.Convert.ToString(paragraph.Range.Text) ?? "").Trim(); }
                    catch { continue; }
                    if (last != null && last.Length > 160) last = last.Substring(0, 160);
                }
                return last;
            }
            catch { return null; }
        }

        private static List<dynamic> TableSpans(dynamic doc)
        {
            var spans = new List<dynamic>();
            foreach (dynamic table in doc.Tables)
            {
                spans.Add(table);
            }
            return spans;
        }

        private static int TableAt(List<dynamic> tables, int position)
        {
            for (int i = 0; i < tables.Count; i++)
            {
                dynamic range = tables[i].Range;
                if (position >= Convert.ToInt32(range.Start) && position < Convert.ToInt32(range.End))
                {
                    return i;
                }
            }
            return -1;
        }

        // Hình và khối (writer.insertBlocks) hiện trong Range.Text là một dấu "/". Bản chạy deepseek 15/10/2026: model
        // tưởng là rác, xoá mọi "/" (mất dấu ngày tháng) rồi dò ~40 ký tự giống "/" suốt 59 vòng. Thay bằng nhãn
        // kèm chữ trong khối để model biết đó là gì và không mất dấu nội dung đã đưa vào khối.
        private static string LabelObjects(dynamic doc, string text)
        {
            var marks = new List<KeyValuePair<int, string>>();
            try
            {
                foreach (dynamic shape in doc.Shapes)
                {
                    try
                    {
                        marks.Add(new KeyValuePair<int, string>(Convert.ToInt32(shape.Anchor.Start), ShapeLabel(shape)));
                    }
                    catch
                    {
                    }
                }
                foreach (dynamic picture in doc.InlineShapes)
                {
                    try
                    {
                        string label = ShapeLabel(picture);
                        marks.Add(new KeyValuePair<int, string>(Convert.ToInt32(picture.Range.Start), label == "[hình]" ? "[ảnh]" : label));
                    }
                    catch
                    {
                    }
                }
            }
            catch
            {
                return text;
            }
            if (marks.Count == 0 || marks.Count > 200)
            {
                return text;
            }
            int docStart = Convert.ToInt32(doc.Content.Start);
            var sb = new StringBuilder(text);
            foreach (var mark in marks.OrderByDescending(m => m.Key))
            {
                int offset;
                try
                {
                    // Vị trí Range khác chỉ số trong Text (mã trường của link bị ẩn): đo bằng độ dài Text phía trước.
                    offset = Convert.ToString(doc.Range(docStart, mark.Key).Text ?? "").Length;
                }
                catch
                {
                    continue;
                }
                if (offset > sb.Length)
                {
                    continue;
                }
                if (offset < sb.Length && sb[offset] == '/')
                {
                    sb.Remove(offset, 1);
                }
                sb.Insert(offset, mark.Value);
            }
            return sb.ToString();
        }

        private static string ShapeLabel(dynamic shape)
        {
            string inner = "";
            try
            {
                if (Convert.ToInt32(shape.TextFrame.HasText) != 0)
                {
                    inner = Convert.ToString(shape.TextFrame.TextRange.Text);
                }
            }
            catch
            {
            }
            inner = Regex.Replace(inner ?? "", @"[\r\n\v\a]+", " | ").Trim(' ', '|');
            if (inner.Length == 0)
            {
                return "[hình]";
            }
            return "[khối: " + (inner.Length > 400 ? inner.Substring(0, 400) + "…" : inner) + "]";
        }

        private static string CleanCellText(string text, int limit)
        {
            text = (text ?? "").Replace("\r", " ").Replace("\a", "").Trim();
            return text.Length > limit ? text.Substring(0, limit) + "…" : text;
        }

        // ---------- writer.restyle ----------
        private static readonly Regex SectionHeading = new Regex(@"^(?:[IVX]+\.|[A-DĐ]\.|\*)\s*\S");
        private static readonly Regex SubHeading = new Regex(@"^\d+(?:\.\d+)*\.?\s+\S");
        private static readonly Regex Label = new Regex(@"^([a-dđ][\.\)]\s*[^:]{2,40}:)");
        private static readonly Regex TableEmphasis = new Regex(@"^(Bước \d|Nhiệm vụ \d|Lưu ý|Luyện tập)", RegexOptions.IgnoreCase);

        private static Dictionary<string, object> WriterRestyle(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            string font = ParamString(p, "font", "Times New Roman");
            double size = ParamDouble(p, "size", 13);
            int accent = ParseBgrColor(ParamString(p, "accent", DesignAccent)) ?? 0x8A5C2E;
            int text = ParseBgrColor(DesignText).Value;
            int muted = ParseBgrColor(DesignMuted).Value;
            // Vài trăm đoạn x nhiều lệnh COM (bản thử: ~90 s cho 285 đoạn) - tắt vẽ lại màn hình trong lúc sửa.
            // Dàn trang lại sau mỗi lần sửa là phần tốn nhất: tạm chuyển sang chế độ xem Nháp, tắt phân trang nền.
            bool screen = true, pagination = true;
            int view = 3;
            dynamic window = null;
            try
            {
                screen = Convert.ToBoolean(app.ScreenUpdating);
                pagination = Convert.ToBoolean(app.Options.Pagination);
                window = doc.ActiveWindow;
                view = Convert.ToInt32(window.View.Type);
                app.ScreenUpdating = false;
                app.Options.Pagination = false;
                window.View.Type = 1;
            }
            catch
            {
            }
            try
            {
                return RestyleBody(app, doc, font, size, accent, text, muted);
            }
            finally
            {
                try
                {
                    if (window != null)
                    {
                        window.View.Type = view;
                    }
                    app.Options.Pagination = pagination;
                    app.ScreenUpdating = screen;
                }
                catch
                {
                }
            }
        }

        private static Dictionary<string, object> RestyleBody(dynamic app, dynamic doc, string font, double size, int accent, int text, int muted)
        {
            int headings = 0, bullets = 0, removed = 0, tables = 0;
            var watch = System.Diagnostics.Stopwatch.StartNew();
            using (new UndoRecordScope(app, "AI: restyle"))
            {
                // Đọc số tự động của MỌI đoạn trước khi sửa: đổi từng đoạn sang chữ thì danh sách còn lại đánh số lại
                // từ 1 (bản thử 14/10/2026: "1. Địa điểm", "1. Thiết bị").
                var paragraphs = new List<dynamic>();
                var lists = new List<Tuple<int, int, string>>();
                bool administrative = false;
                foreach (dynamic paragraph in doc.Paragraphs)
                {
                    paragraphs.Add(paragraph);
                    if (!administrative)
                    {
                        try
                        {
                            administrative = IsNationalTitle(Convert.ToString(paragraph.Range.Text));
                        }
                        catch
                        {
                        }
                    }
                    int listType = 0, listLevel = 1;
                    string listString = "";
                    try
                    {
                        dynamic listFormat = paragraph.Range.ListFormat;
                        listType = Convert.ToInt32(listFormat.ListType);
                        if (listType != 0)
                        {
                            listLevel = Convert.ToInt32(listFormat.ListLevelNumber);
                            listString = Convert.ToString(listFormat.ListString);
                        }
                    }
                    catch
                    {
                    }
                    lists.Add(Tuple.Create(listType, listLevel, listString));
                }
                // Văn bản hành chính (có quốc hiệu): thể thức Nghị định 30/2020 - chữ đen, không tô màu nhấn, không
                // kẻ gạch dưới tiêu đề mục. Đo 24/10/2026: lá đơn có quốc hiệu căn giữa + đậm bị tô #2E5C8A.
                if (administrative)
                {
                    accent = 0;
                    text = 0;
                    muted = 0;
                }
                var tableList = TableSpans(doc);
                var empty = new List<dynamic>();
                for (int at = 0; at < paragraphs.Count; at++)
                {
                    dynamic paragraph = paragraphs[at];
                    dynamic range = paragraph.Range;
                    if (TableAt(tableList, Convert.ToInt32(range.Start)) >= 0)
                    {
                        continue;
                    }
                    // Số tự động -> chữ (để nhận dạng "A. HOẠT ĐỘNG", "1. Ổn định"); gạch đầu dòng tự động -> gạch tay
                    // (trừ khi chữ đã tự có dấu gạch: "•  - KHBD" ở bản thử).
                    int listType = lists[at].Item1;
                    if (listType != 0)
                    {
                        range.ListFormat.RemoveNumbers();
                        range = paragraph.Range;
                        string body = Convert.ToString(range.Text).Trim();
                        bool bullet = listType == 2 || listType == 6 || !lists[at].Item3.Any(char.IsLetterOrDigit);
                        if (!bullet)
                        {
                            range.InsertBefore(lists[at].Item3 + " ");
                        }
                        else if (body.Length > 0 && "-–•·+".IndexOf(body[0]) < 0)
                        {
                            range.InsertBefore(lists[at].Item2 > 1 ? "+ " : "- ");
                        }
                        range = paragraph.Range;
                    }
                    string raw = Convert.ToString(range.Text).TrimEnd('\r', '\a');
                    string trimmed = raw.Trim().Replace("\t", " ");
                    bool hasShape = false;
                    try
                    {
                        hasShape = Convert.ToInt32(range.InlineShapes.Count) > 0 || Convert.ToInt32(range.ShapeRange.Count) > 0;
                    }
                    catch
                    {
                    }
                    if (trimmed.Length == 0 || (trimmed.Length == 1 && "-–•·+".IndexOf(trimmed[0]) >= 0))
                    {
                        if (!hasShape)
                        {
                            empty.Add(paragraph);
                        }
                        continue;
                    }
                    if (hasShape)
                    {
                        continue;
                    }
                    dynamic format = paragraph.Format;
                    range.Font.Name = font;
                    format.SpaceBefore = 0;
                    format.SpaceAfter = 2;
                    format.LineSpacingRule = 5;
                    format.LineSpacing = 12 * 1.12f;
                    try
                    {
                        paragraph.Borders.Enable = 0;
                    }
                    catch
                    {
                    }
                    bool centered = Convert.ToInt32(format.Alignment) == 1;
                    bool upper = trimmed.Length > 3 && trimmed == trimmed.ToUpper(CultureInfo.GetCultureInfo("vi-VN"));
                    if (SectionHeading.IsMatch(trimmed) && upper && trimmed.Length <= 120)
                    {
                        if (trimmed.StartsWith("*", StringComparison.Ordinal))
                        {
                            ReplaceLeading(range, raw, raw.IndexOf('*') + 1, "");
                            range = paragraph.Range;
                        }
                        format.LeftIndent = 0;
                        format.FirstLineIndent = 0;
                        format.Alignment = 0;
                        format.SpaceBefore = 10;
                        format.SpaceAfter = 6;
                        format.KeepWithNext = -1;
                        range.Font.Size = 13.5f;
                        range.Font.Bold = -1;
                        range.Font.Italic = 0;
                        range.Font.Color = accent;
                        if (!administrative)
                        {
                            dynamic bottom = paragraph.Borders[-3];
                            bottom.LineStyle = 1;
                            bottom.LineWidth = 4;
                            bottom.Color = ParseBgrColor("#C9DAEC").Value;
                            paragraph.Borders.DistanceFromBottom = 3;
                        }
                        headings++;
                        continue;
                    }
                    range.Font.Size = (float)size;
                    if (centered)
                    {
                        range.Font.Color = Convert.ToInt32(range.Font.Bold) == -1 ? accent : text;
                        continue;
                    }
                    if (SubHeading.IsMatch(trimmed) && trimmed.Length <= 90 && !trimmed.EndsWith(".", StringComparison.Ordinal))
                    {
                        format.LeftIndent = 0;
                        format.FirstLineIndent = 0;
                        format.SpaceBefore = 4;
                        format.KeepWithNext = -1;
                        range.Font.Bold = -1;
                        range.Font.Color = text;
                        continue;
                    }
                    Match label = Label.Match(trimmed);
                    if (label.Success)
                    {
                        format.LeftIndent = 0;
                        format.FirstLineIndent = 0;
                        format.SpaceBefore = 3;
                        range.Font.Color = text;
                        int offset = raw.IndexOf(trimmed.Substring(0, 1), StringComparison.Ordinal);
                        dynamic head = doc.Range(Convert.ToInt32(range.Start) + Math.Max(0, offset),
                            Convert.ToInt32(range.Start) + Math.Max(0, offset) + label.Groups[1].Length);
                        head.Font.Bold = -1;
                        head.Font.Color = accent;
                        continue;
                    }
                    char first = trimmed[0];
                    if (first == '-' || first == '–' || first == '•' || first == '·' || first == '+')
                    {
                        bool second = first == '+';
                        int markerEnd = raw.IndexOf(first) + 1;
                        while (markerEnd < raw.Length && (raw[markerEnd] == ' ' || raw[markerEnd] == '\t'))
                        {
                            markerEnd++;
                        }
                        ReplaceLeading(range, raw, markerEnd, second ? "–  " : "•  ");
                        range = paragraph.Range;
                        format.LeftIndent = (float)(second ? 34 : 17);
                        format.FirstLineIndent = -12.75f;
                        range.Font.Color = text;
                        dynamic marker = doc.Range(Convert.ToInt32(range.Start), Convert.ToInt32(range.Start) + 1);
                        marker.Font.Bold = -1;
                        marker.Font.Color = second ? muted : accent;
                        bullets++;
                        continue;
                    }
                    range.Font.Color = text;
                }
                long paragraphMs = watch.ElapsedMilliseconds;
                // Văn bản hành chính: dòng trống giữa chức danh và họ tên là khoảng trống KÝ - không xoá.
                for (int i = empty.Count - 1; i >= 0 && !administrative; i--)
                {
                    try
                    {
                        empty[i].Range.Delete();
                        removed++;
                    }
                    catch
                    {
                    }
                }
                int layoutTables = 0;
                foreach (dynamic table in doc.Tables)
                {
                    // Bảng dàn trang (formatTable {borders:false}) cố ý không viền: kẻ khung lại là sai bố cục.
                    if (IsLayoutTable(table))
                    {
                        layoutTables++;
                        continue;
                    }
                    RestyleTable(doc, table, font, accent, text);
                    tables++;
                }
                RestylePage(doc, muted);
                Logger.Info("writer.restyle: " + paragraphs.Count + " paragraphs " + paragraphMs + " ms, total " + watch.ElapsedMilliseconds
                    + " ms" + (administrative ? " (administrative)" : ""));
                return new Dictionary<string, object>
                {
                    { "headings", headings }, { "bullets", bullets }, { "removedEmpty", removed }, { "tables", tables },
                    { "layoutTables", layoutTables }, { "mode", administrative ? "administrative" : "design" }
                };
            }
        }

        // Quốc hiệu (Nghị định 30/2020, Phụ lục I): dấu hiệu văn bản hành chính. So sánh sau khi bỏ khoảng trắng
        // thừa và ký tự kết ô/đoạn của Word, không phân biệt hoa/thường.
        internal static bool IsNationalTitle(string raw)
        {
            if (string.IsNullOrEmpty(raw))
            {
                return false;
            }
            string value = Regex.Replace(raw.Replace("\r", " ").Replace("\a", " ").Replace("\v", " "), @"\s+", " ").Trim()
                .Normalize(NormalizationForm.FormC).ToUpper(CultureInfo.GetCultureInfo("vi-VN"))
                .Replace("HOÀ", "HÒA");   // dấu đặt kiểu cũ/mới đều gặp trong văn bản thật
            return value.Length >= 30 && value.Length <= 60 && value.Contains("CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM");
        }

        // Thay phần đầu đoạn (dấu gạch, "* ") bằng `replacement` mà không đụng phần chữ còn lại.
        private static void ReplaceLeading(dynamic range, string raw, int length, string replacement)
        {
            dynamic head = range.Document.Range(Convert.ToInt32(range.Start), Convert.ToInt32(range.Start) + Math.Max(0, length));
            head.Text = replacement;
        }

        private static void RestyleTable(dynamic doc, dynamic table, string font, int accent, int text)
        {
            int border = ParseBgrColor("#D5DDE6").Value;
            int headerFill = ParseBgrColor("#E8F0F8").Value;
            try
            {
                foreach (int side in new[] { -1, -2, -3, -4, -5, -6 })
                {
                    dynamic b = table.Borders[side];
                    b.LineStyle = 1;
                    b.LineWidth = 4;
                    b.Color = border;
                }
            }
            catch
            {
            }
            table.TopPadding = 3;
            table.BottomPadding = 3;
            table.LeftPadding = 5;
            table.RightPadding = 5;
            int columns = Convert.ToInt32(table.Columns.Count);
            // Bảng tiến trình ND | ĐL | Hoạt động: cột giữa hẹp.
            try
            {
                string middle = columns == 3 ? CleanCellText(Convert.ToString(table.Cell(1, 2).Range.Text), 20) : "";
                if (columns == 3 && middle.Length > 0 && middle.Length <= 6)
                {
                    double width = Convert.ToDouble(doc.PageSetup.PageWidth) - Convert.ToDouble(doc.PageSetup.LeftMargin) - Convert.ToDouble(doc.PageSetup.RightMargin);
                    double[] share = { 0.29, 0.09, 0.62 };
                    for (int c = 1; c <= 3; c++)
                    {
                        table.Columns[c].PreferredWidthType = 3;
                        table.Columns[c].PreferredWidth = (float)(width * share[c - 1]);
                    }
                }
            }
            catch
            {
            }
            // Duyệt theo ô (không theo hàng): bảng có ô gộp dọc không truy cập Rows[r] được.
            try
            {
                table.Rows.AllowBreakAcrossPages = -1;
            }
            catch
            {
            }
            foreach (dynamic cell in table.Range.Cells)
            {
                int r = Convert.ToInt32(cell.RowIndex);
                dynamic cellRange = cell.Range;
                cellRange.Font.Name = font;
                cellRange.Font.Size = 12;
                if (r == 1)
                {
                    cell.Shading.BackgroundPatternColor = headerFill;
                    cellRange.Font.Bold = -1;
                    cellRange.Font.Color = accent;
                    cellRange.ParagraphFormat.Alignment = 1;
                }
                // Dòng trống cuối ô đẩy bảng sang trang mới (bản thử 14/10/2026).
                int count = Convert.ToInt32(cellRange.Paragraphs.Count);
                for (int k = count; k > 1; k--)
                {
                    dynamic last = cell.Range.Paragraphs[k];
                    string value = Convert.ToString(last.Range.Text).Replace("\r", "").Replace("\a", "").Trim();
                    if (value.Length > 0 || Convert.ToInt32(last.Range.InlineShapes.Count) > 0)
                    {
                        break;
                    }
                    try
                    {
                        doc.Range(Convert.ToInt32(last.Range.Start) - 1, Convert.ToInt32(last.Range.Start)).Delete();
                    }
                    catch
                    {
                        break;
                    }
                }
                if (r > 1)
                {
                    foreach (dynamic paragraph in cell.Range.Paragraphs)
                    {
                        string value = Convert.ToString(paragraph.Range.Text).Trim();
                        paragraph.SpaceAfter = 1;
                        if (TableEmphasis.IsMatch(value))
                        {
                            paragraph.Range.Font.Bold = -1;
                            paragraph.Range.Font.Color = accent;
                        }
                    }
                }
            }
            try
            {
                table.Rows[1].HeadingFormat = -1;
            }
            catch
            {
            }
        }

        private static void RestylePage(dynamic doc, int muted)
        {
            foreach (dynamic section in doc.Sections)
            {
                dynamic setup = section.PageSetup;
                setup.TopMargin = 56.7f;
                setup.BottomMargin = 56.7f;
                setup.LeftMargin = 70.9f;
                setup.RightMargin = 51f;
                dynamic footer = section.Footers[1].Range;
                if (Convert.ToString(footer.Text).Trim().Length == 0)
                {
                    footer.Text = "Trang ";
                    footer.Font.Size = 10.5f;
                    footer.Font.Color = muted;
                    footer.ParagraphFormat.Alignment = 1;
                    // Vị trí trong chân trang thuộc story riêng: doc.Range(...) là thân bài, phải lấy từ range của footer.
                    dynamic end = section.Footers[1].Range;
                    end.MoveEnd(1, -1);
                    end.Collapse(0);
                    end.Fields.Add(end, 33);
                }
            }
        }

        // ---------- writer.insertBlocks ----------
        private static Dictionary<string, object> WriterInsertBlocks(IAppHost host, Dictionary<string, object> p)
        {
            object raw;
            if (p == null || !p.TryGetValue("blocks", out raw) || !(raw is IEnumerable) || raw is string)
            {
                throw new ArgumentException("'blocks' must be an array");
            }
            var blocks = ((IEnumerable)raw).Cast<object>().OfType<Dictionary<string, object>>().ToList();
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            string font = ParamString(p, "font", "Times New Roman");
            double width = Convert.ToDouble(doc.PageSetup.PageWidth) - Convert.ToDouble(doc.PageSetup.LeftMargin) - Convert.ToDouble(doc.PageSetup.RightMargin);
            int inserted = 0;
            // Từ dưới lên: chèn/xoá phía dưới không làm lệch số đoạn phía trên.
            blocks = blocks.OrderByDescending(b => Math.Max(ParamInt(b, "before", 0), ParamInt(b, "replaceFrom", 0))).ToList();
            using (new UndoRecordScope(app, "AI: design blocks"))
            {
                foreach (var block in blocks)
                {
                    int from = ParamInt(block, "replaceFrom", 0);
                    int to = ParamInt(block, "replaceTo", from);
                    int before = ParamInt(block, "before", 0);
                    int count = Convert.ToInt32(doc.Paragraphs.Count);
                    dynamic target;
                    if (from > 0)
                    {
                        if (to < from || to > count)
                        {
                            throw new ArgumentException("replaceFrom/replaceTo out of range (1-" + count + ")");
                        }
                        int start = Convert.ToInt32(doc.Paragraphs[from].Range.Start);
                        int end = Convert.ToInt32(doc.Paragraphs[to].Range.End);
                        dynamic old = doc.Range(start, end);
                        old.Delete();
                        target = doc.Range(start, start);
                    }
                    else if (before > 0 && before <= count)
                    {
                        int start = Convert.ToInt32(doc.Paragraphs[before].Range.Start);
                        target = doc.Range(start, start);
                    }
                    else
                    {
                        int end = Convert.ToInt32(doc.Content.End) - 1;
                        target = doc.Range(end, end);
                    }
                    string xml = BlockPackage(BlockBody(block, width, font));
                    target.InsertXML(xml);
                    inserted++;
                }
            }
            return new Dictionary<string, object> { { "inserted", inserted } };
        }

        private static string BlockPackage(string bodyXml)
        {
            return "<?xml version=\"1.0\" standalone=\"yes\"?>"
                + "<pkg:package xmlns:pkg=\"http://schemas.microsoft.com/office/2006/xmlPackage\">"
                + "<pkg:part pkg:name=\"/_rels/.rels\" pkg:contentType=\"application/vnd.openxmlformats-package.relationships+xml\"><pkg:xmlData>"
                + "<Relationships xmlns=\"http://schemas.openxmlformats.org/package/2006/relationships\">"
                + "<Relationship Id=\"rId1\" Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument\" Target=\"word/document.xml\"/>"
                + "</Relationships></pkg:xmlData></pkg:part>"
                + "<pkg:part pkg:name=\"/word/document.xml\" pkg:contentType=\"application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml\"><pkg:xmlData>"
                + "<w:document xmlns:w=\"http://schemas.openxmlformats.org/wordprocessingml/2006/main\""
                + " xmlns:wp=\"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing\""
                + " xmlns:a=\"http://schemas.openxmlformats.org/drawingml/2006/main\""
                + " xmlns:wps=\"http://schemas.microsoft.com/office/word/2010/wordprocessingShape\"><w:body>"
                + bodyXml
                + "</w:body></w:document></pkg:xmlData></pkg:part></pkg:package>";
        }

        private static int _shapeSerial = 5000;

        // Một khối (box) hoặc nhiều khối cạnh nhau (row, trong bảng ẩn viền) - đúng mẫu đã duyệt.
        private static string BlockBody(Dictionary<string, object> block, double contentWidth, string font)
        {
            string kind = ParamString(block, "kind", "box");
            double after = ParamDouble(block, "after", 6);
            double gap = ParamDouble(block, "gap", 10);
            var cells = new List<Dictionary<string, object>>();
            object raw;
            if (block.TryGetValue("cells", out raw) && raw is IEnumerable && !(raw is string))
            {
                cells.AddRange(((IEnumerable)raw).Cast<object>().OfType<Dictionary<string, object>>());
            }
            if (cells.Count == 0)
            {
                throw new ArgumentException("each block needs 'cells'");
            }
            var sb = new StringBuilder();
            if (kind == "row" && cells.Count > 1)
            {
                double width = (contentWidth - gap * (cells.Count - 1)) / cells.Count;
                double height = cells.Max(cell => TextHeight(cell, width - 20, font)) + 28;
                sb.Append("<w:tbl><w:tblPr><w:tblW w:w=\"0\" w:type=\"auto\"/><w:tblLayout w:type=\"fixed\"/>")
                    .Append("<w:tblBorders><w:top w:val=\"nil\"/><w:left w:val=\"nil\"/><w:bottom w:val=\"nil\"/><w:right w:val=\"nil\"/>")
                    .Append("<w:insideH w:val=\"nil\"/><w:insideV w:val=\"nil\"/></w:tblBorders>")
                    .Append("<w:tblCellMar><w:top w:w=\"0\" w:type=\"dxa\"/><w:left w:w=\"0\" w:type=\"dxa\"/><w:bottom w:w=\"0\" w:type=\"dxa\"/><w:right w:w=\"0\" w:type=\"dxa\"/></w:tblCellMar>")
                    .Append("</w:tblPr><w:tblGrid>");
                for (int i = 0; i < cells.Count; i++)
                {
                    sb.Append("<w:gridCol w:w=\"").Append(Twips(width + (i < cells.Count - 1 ? gap : 0))).Append("\"/>");
                }
                sb.Append("</w:tblGrid><w:tr><w:trPr><w:cantSplit/></w:trPr>");
                for (int i = 0; i < cells.Count; i++)
                {
                    sb.Append("<w:tc><w:tcPr><w:tcW w:w=\"").Append(Twips(width + (i < cells.Count - 1 ? gap : 0)))
                        .Append("\" w:type=\"dxa\"/></w:tcPr><w:p><w:pPr><w:spacing w:after=\"0\"/></w:pPr>")
                        .Append(ShapeRun(cells[i], width, height, false, font)).Append("</w:p></w:tc>");
                }
                sb.Append("</w:tr></w:tbl>");
                sb.Append("<w:p><w:pPr><w:spacing w:after=\"0\" w:line=\"").Append(Twips(after)).Append("\" w:lineRule=\"exact\"/></w:pPr></w:p>");
                return sb.ToString();
            }
            double single = TextHeight(cells[0], contentWidth - 20, font) + 20;
            sb.Append("<w:p><w:pPr><w:spacing w:after=\"").Append(Twips(after)).Append("\"/>");
            if (ParamBool(block, "keepWithNext", false))
            {
                sb.Append("<w:keepNext/>");
            }
            sb.Append("</w:pPr>").Append(ShapeRun(cells[0], contentWidth, single, true, font)).Append("</w:p>");
            return sb.ToString();
        }

        private static string Twips(double points)
        {
            return ((int)Math.Round(points * 20)).ToString(CultureInfo.InvariantCulture);
        }

        private static List<Dictionary<string, object>> CellLines(Dictionary<string, object> cell)
        {
            object raw;
            if (cell.TryGetValue("lines", out raw) && raw is IEnumerable && !(raw is string))
            {
                return ((IEnumerable)raw).Cast<object>().OfType<Dictionary<string, object>>().ToList();
            }
            return new List<Dictionary<string, object>>();
        }

        private static double LineSize(Dictionary<string, object> line, double baseSize)
        {
            string k = ParamString(line, "k", "text");
            double size = ParamDouble(line, "size", 0);
            if (size > 0)
            {
                return size;
            }
            return k == "small" ? baseSize - 1.5 : k == "title" ? baseSize + 0.5 : baseSize;
        }

        // Chiều cao chữ đo bằng font thật (GDI+, điểm): xuống dòng theo từ, dòng cao 1,32 x cỡ (Word, giãn 1,12).
        private static double TextHeight(Dictionary<string, object> cell, double width, string font)
        {
            double baseSize = ParamDouble(cell, "size", 12.5);
            double total = 0;
            using (var bitmap = new System.Drawing.Bitmap(1, 1))
            using (var graphics = System.Drawing.Graphics.FromImage(bitmap))
            {
                graphics.PageUnit = System.Drawing.GraphicsUnit.Point;
                foreach (var line in CellLines(cell))
                {
                    string k = ParamString(line, "k", "text");
                    double size = LineSize(line, baseSize);
                    bool bold = k == "title" || (k == "center" && ParamBool(line, "bold", false));
                    string text = ParamString(line, "t", "").Replace("**", "");
                    // Word ngắt dòng sớm hơn GDI+ một chút: thẻ "Năng lực riêng" bị cắt dòng cuối ở bản thử 14/10/2026.
                    double usable = (width - (k == "bullet" ? 13 : 0)) * 0.95 - 2;
                    using (var measure = new System.Drawing.Font(font, (float)size, bold ? System.Drawing.FontStyle.Bold : System.Drawing.FontStyle.Regular, System.Drawing.GraphicsUnit.Point))
                    {
                        total += WrappedLines(graphics, measure, text, usable) * size * 1.32 + (k == "title" ? 4 : 2);
                    }
                }
            }
            return total;
        }

        private static int WrappedLines(System.Drawing.Graphics graphics, System.Drawing.Font font, string text, double width)
        {
            var format = System.Drawing.StringFormat.GenericTypographic;
            double space = graphics.MeasureString("a a", font, 10000, format).Width - graphics.MeasureString("aa", font, 10000, format).Width;
            int lines = 1;
            double current = 0;
            foreach (string word in text.Split(new[] { ' ' }, StringSplitOptions.RemoveEmptyEntries))
            {
                double w = graphics.MeasureString(word, font, 10000, format).Width;
                if (current > 0 && current + space + w > width)
                {
                    lines++;
                    current = w;
                }
                else
                {
                    current += (current > 0 ? space : 0) + w;
                }
            }
            return lines;
        }

        private static string ShapeRun(Dictionary<string, object> cell, double width, double height, bool autofit, string font)
        {
            string tone = ParamString(cell, "tone", "blue");
            string[] colors;
            if (!DesignTones.TryGetValue(tone, out colors))
            {
                colors = DesignTones["blue"];
            }
            double baseSize = ParamDouble(cell, "size", 12.5);
            const double pad = 10;
            long cx = (long)Math.Round(width * 12700), cy = (long)Math.Round(height * 12700), ins = (long)(pad * 12700);
            int id = ++_shapeSerial;
            var sb = new StringBuilder();
            sb.Append("<w:r><w:drawing><wp:inline distT=\"0\" distB=\"0\" distL=\"0\" distR=\"0\">")
                .Append("<wp:extent cx=\"").Append(cx).Append("\" cy=\"").Append(cy).Append("\"/><wp:effectExtent l=\"0\" t=\"0\" r=\"0\" b=\"0\"/>")
                .Append("<wp:docPr id=\"").Append(id).Append("\" name=\"Khoi ").Append(id).Append("\"/>")
                .Append("<a:graphic><a:graphicData uri=\"http://schemas.microsoft.com/office/word/2010/wordprocessingShape\"><wps:wsp><wps:cNvSpPr/><wps:spPr>")
                .Append("<a:xfrm><a:off x=\"0\" y=\"0\"/><a:ext cx=\"").Append(cx).Append("\" cy=\"").Append(cy).Append("\"/></a:xfrm>")
                .Append("<a:prstGeom prst=\"roundRect\"><a:avLst><a:gd name=\"adj\" fmla=\"val 9000\"/></a:avLst></a:prstGeom>")
                .Append("<a:solidFill><a:srgbClr val=\"").Append(colors[0]).Append("\"/></a:solidFill>")
                .Append("<a:ln w=\"9525\"><a:solidFill><a:srgbClr val=\"").Append(colors[1]).Append("\"/></a:solidFill></a:ln>")
                .Append("</wps:spPr><wps:txbx><w:txbxContent>");
            foreach (var line in CellLines(cell))
            {
                sb.Append(LineXml(line, baseSize, colors[2], font));
            }
            sb.Append("</w:txbxContent></wps:txbx><wps:bodyPr lIns=\"").Append(ins).Append("\" tIns=\"").Append(ins)
                .Append("\" rIns=\"").Append(ins).Append("\" bIns=\"").Append(ins).Append("\" anchor=\"t\">")
                .Append(autofit ? "<a:spAutoFit/>" : "<a:noAutofit/>")
                .Append("</wps:bodyPr></wps:wsp></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>");
            return sb.ToString();
        }

        private static string LineXml(Dictionary<string, object> line, double baseSize, string accent, string font)
        {
            string k = ParamString(line, "k", "text");
            string text = ParamString(line, "t", "");
            double size = LineSize(line, baseSize);
            var sb = new StringBuilder("<w:p><w:pPr>");
            switch (k)
            {
                case "title":
                    sb.Append("<w:spacing w:after=\"80\" w:line=\"269\" w:lineRule=\"auto\"/>");
                    break;
                case "bullet":
                    sb.Append("<w:spacing w:after=\"40\" w:line=\"269\" w:lineRule=\"auto\"/><w:ind w:left=\"255\" w:hanging=\"255\"/>");
                    break;
                case "center":
                    sb.Append("<w:spacing w:after=\"20\" w:line=\"269\" w:lineRule=\"auto\"/><w:jc w:val=\"center\"/>");
                    break;
                default:
                    sb.Append("<w:spacing w:after=\"40\" w:line=\"269\" w:lineRule=\"auto\"/>");
                    break;
            }
            sb.Append("</w:pPr>");
            if (k == "title")
            {
                sb.Append(RunXml(text, true, false, size, accent, font));
            }
            else if (k == "small")
            {
                sb.Append(RunXml(text, false, true, size, DesignMuted.TrimStart('#'), font));
            }
            else
            {
                if (k == "bullet")
                {
                    sb.Append(RunXml("•  ", true, false, size, accent, font));
                }
                bool center = k == "center";
                bool bold = center && ParamBool(line, "bold", false);
                string plain = center ? accent : DesignText.TrimStart('#');
                string[] parts = text.Split(new[] { "**" }, StringSplitOptions.None);
                for (int i = 0; i < parts.Length; i++)
                {
                    if (parts[i].Length > 0)
                    {
                        bool emphasis = i % 2 == 1;
                        sb.Append(RunXml(parts[i], emphasis || bold, false, size, emphasis ? accent : plain, font));
                    }
                }
            }
            sb.Append("</w:p>");
            return sb.ToString();
        }

        private static string RunXml(string text, bool bold, bool italic, double size, string color, string font)
        {
            string escaped = System.Security.SecurityElement.Escape(text) ?? "";
            int half = (int)Math.Round(size * 2);
            return "<w:r><w:rPr><w:rFonts w:ascii=\"" + font + "\" w:hAnsi=\"" + font + "\" w:cs=\"" + font + "\" w:eastAsia=\"" + font + "\"/>"
                + (bold ? "<w:b/>" : "") + (italic ? "<w:i/>" : "")
                + "<w:color w:val=\"" + color + "\"/><w:sz w:val=\"" + half + "\"/><w:szCs w:val=\"" + half + "\"/></w:rPr>"
                + "<w:t xml:space=\"preserve\">" + escaped + "</w:t></w:r>";
        }
    }
}
