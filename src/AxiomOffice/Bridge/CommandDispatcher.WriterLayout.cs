using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // Tự soát phân trang và viết nhiều khối trong một lệnh (docs/plans/21-v0 P2). Bộ đo 26/10/2026: báo cáo 4 trang còn một
    // trang trắng cuối (đoạn trống thừa sau đoạn cuối, ca w02); đề A dài 11 trang (đề 6–8) mà model không biết số trang;
    // mỗi đoạn một lệnh nên báo cáo 6 trang hết 60 vòng.
    internal static partial class CommandDispatcher
    {
        // Bảng nhỏ (<= số hàng này) không nên bị cắt sang trang: insertTable tự giữ liền, checkLayout báo khi bị cắt.
        private const int SmallTableRows = 15;
        private const int LayoutParagraphLimit = 5000;
        private const int LayoutHeadingLimit = 80;

        private static IEnumerable<CommandInfo> WriterLayoutCommands()
        {
            return new[]
            {
                Command("writer.checkLayout", "wps", WriterCheckLayout,
                    "Soát phân trang (chỉ đọc): số trang, số chữ, trang trắng, tiêu đề nằm cuối trang tách khỏi nội dung, bảng nhỏ bị cắt sang trang, đoạn trống liên tiếp; trang của từng tiêu đề - gọi sau khi viết xong, trước khi trả lời",
                    Opt("maxPages", "page limit the user asked for (e.g. 8 for '6-8 pages'): longer = issue too-long with how much to cut")).ForAgent(),
                Command("writer.writeContent", "wps", WriterWriteContent,
                    "Viết NHIỀU khối nối tiếp vào CUỐI bài trong MỘT lệnh (một bước hoàn tác) - dùng thay cho mỗi đoạn một lệnh: tiêu đề, đoạn văn, gạch đầu dòng, danh sách số, bảng kèm chú thích, ngắt trang",
                    Req("blocks", "array, in order, of {heading, level} | {text} ('\\n' = new paragraph; optional alignment, bold, italic, size) | {bullets: [..]} | {numbers: [..]} | {table: 2D array, caption, style} | {pageBreak: true}"),
                    Opt("alignment", "alignment of {text} blocks: justify (default) / left / center / right")).ForAgent(),
                Command("writer.addTableRows", "wps", WriterAddTableRows,
                    "Thêm hàng vào bảng CÓ SẴN (giữ định dạng của bảng, không dựng lại bảng): mặc định nối cuối bảng, `before` = số hàng để chèn vào giữa (1 = hàng tiêu đề)",
                    Req("rows", "2D array, one inner array of cell texts per new row"),
                    Opt("table", "table number (1 = first, as in writer.checkTables); default: table at cursor, else the last table"),
                    Opt("before", "row number to insert before; default: after the last row")).ForAgent(),
                Command("writer.deleteParagraphs", "wps", WriterDeleteParagraphs,
                    "Xoá các đoạn theo số của writer.outline / writer.checkLayout (đoạn trống thừa, đoạn chỉ có dấu ngắt trang, đoạn viết thừa); không xoá được đoạn trong bảng",
                    Req("paragraphs", "array of paragraph numbers")).ForAgent()
            };
        }

        private sealed class LayoutParagraph
        {
            public int N;
            public string Text;
            public bool Content;
            public bool PageBreak;
            public int Level;
            public int StartPage;
            public int Page;
            public double Top;
            public int Table;
        }

        private static Dictionary<string, object> WriterCheckLayout(IAppHost host, Dictionary<string, object> p)
        {
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            doc.Repaginate();
            // wdStatisticWords = 0, wdStatisticPages = 2.
            int pages = Convert.ToInt32(doc.ComputeStatistics(2));
            int words = Convert.ToInt32(doc.ComputeStatistics(0));
            var tables = TableSpans(doc);
            var items = new List<LayoutParagraph>();
            var contentPages = new HashSet<int>();
            var tableSeen = new HashSet<int>();
            var splitTables = new List<int>();
            int total = Convert.ToInt32(doc.Paragraphs.Count);
            int n = 0;
            foreach (dynamic paragraph in doc.Paragraphs)
            {
                n++;
                if (n > LayoutParagraphLimit)
                {
                    break;
                }
                dynamic range = paragraph.Range;
                int start = Convert.ToInt32(range.Start);
                int tableIndex = TableAt(tables, start);
                if (tableIndex >= 0)
                {
                    // Bảng: một mục cho cả bảng (ô của bảng 200 dòng là hàng trăm đoạn - không hỏi trang từng ô).
                    if (tableSeen.Add(tableIndex))
                    {
                        dynamic table = tables[tableIndex];
                        int first = Convert.ToInt32(table.Range.Characters.First.Information(3));
                        int last = Convert.ToInt32(table.Range.Characters.Last.Information(3));
                        for (int page = first; page <= last; page++)
                        {
                            contentPages.Add(page);
                        }
                        if (last > first && Convert.ToInt32(table.Rows.Count) <= SmallTableRows && !IsLayoutTable(table))
                        {
                            splitTables.Add(tableIndex + 1);
                        }
                        items.Add(new LayoutParagraph { N = n, Content = true, StartPage = first, Page = last, Level = 10, Table = tableIndex + 1, Text = "" });
                    }
                    continue;
                }
                string raw = Convert.ToString(range.Text) ?? "";
                var item = new LayoutParagraph { N = n, PageBreak = raw.IndexOf('\f') >= 0 };
                item.Text = raw.Replace("\f", "").TrimEnd('\r', '\a').Trim();
                bool shape = false;
                try
                {
                    shape = Convert.ToInt32(range.InlineShapes.Count) > 0;
                }
                catch
                {
                }
                // Đoạn của mục lục (trường TOC) tự cập nhật, không sửa/xoá tay được: coi là có nội dung, không đề xuất xoá.
                item.Content = item.Text.Length > 0 || shape || InsideToc(doc, start);
                item.Page = Convert.ToInt32(range.Information(3));
                item.StartPage = item.Page;
                item.Level = 10;
                if (item.Content)
                {
                    item.StartPage = Convert.ToInt32(doc.Range(start, start).Information(3));
                    for (int page = item.StartPage; page <= item.Page; page++)
                    {
                        contentPages.Add(page);
                    }
                    try
                    {
                        item.Level = Convert.ToInt32(paragraph.OutlineLevel);
                    }
                    catch
                    {
                    }
                    if (item.Level < 10)
                    {
                        // wdVerticalPositionRelativeToPage = 6.
                        item.Top = Convert.ToDouble(range.Information(6));
                    }
                }
                items.Add(item);
            }

            var issues = new List<Dictionary<string, object>>();
            // Trang trắng: không có chữ, ảnh hay bảng - chỉ đoạn trống / dấu ngắt trang (trang chứa đoạn trống cuối bài khi trang
            // trước vừa đầy, hoặc hai lần ngắt trang liền nhau).
            for (int page = 1; page <= pages; page++)
            {
                if (contentPages.Contains(page))
                {
                    continue;
                }
                // Chỉ các đoạn NẰM TRÊN trang trắng; dấu ngắt kết thúc trang trước là ngắt hợp lệ, xoá nó thì gộp hai trang.
                List<int> empties = items.Where(i => !i.Content && i.Table == 0 && i.Page == page).Select(i => i.N).ToList();
                string fix = empties.Count > 0
                    ? "delete the empty paragraph(s) / page break(s) " + string.Join(", ", empties) + " with writer.deleteParagraphs {\"paragraphs\": [" + string.Join(", ", empties) + "]}"
                    : "remove the extra page break or empty paragraphs around it";
                issues.Add(Issue("blank-page", "page " + page + " of " + pages + " is blank: " + fix,
                    new Dictionary<string, object> { { "page", page }, { "paragraphs", empties } }));
            }
            // Tiêu đề là thứ cuối cùng trên trang, nội dung của nó sang trang sau.
            for (int i = 0; i < items.Count; i++)
            {
                LayoutParagraph heading = items[i];
                if (!heading.Content || heading.Level >= 10)
                {
                    continue;
                }
                int j = i + 1;
                var between = new List<int>();
                while (j < items.Count && !items[j].Content)
                {
                    between.Add(items[j].N);
                    j++;
                }
                if (j >= items.Count || items[j].StartPage <= heading.Page)
                {
                    continue;
                }
                string fix = between.Count > 0
                    ? "delete the empty paragraph(s) " + string.Join(", ", between) + " after it (writer.deleteParagraphs) so it stays with its text"
                    : "start it on the next page: writer.insertPageBreak {\"before\": " + heading.N + "}";
                issues.Add(Issue("heading-at-page-bottom", "heading '" + Shorten(heading.Text, 50) + "' (paragraph " + heading.N + ") ends page "
                    + heading.Page + " but its text starts on page " + items[j].StartPage + ": " + fix,
                    new Dictionary<string, object> { { "paragraph", heading.N }, { "page", heading.Page } }));
            }
            foreach (int t in splitTables)
            {
                issues.Add(Issue("split-table", "table " + t + " has at most " + SmallTableRows + " rows but is cut across pages: keep it on one page with writer.formatTable {\"table\": " + t + ", \"keepTogether\": true}",
                    new Dictionary<string, object> { { "table", t } }));
            }
            // Đoạn trống liên tiếp dùng để dàn trang (ca w01): khoảng cách nên đến từ style, không từ dòng trống.
            for (int i = 0; i < items.Count; i++)
            {
                if (items[i].Content || items[i].Table > 0 || items[i].PageBreak)
                {
                    continue;
                }
                int j = i;
                while (j + 1 < items.Count && !items[j + 1].Content && items[j + 1].Table == 0 && !items[j + 1].PageBreak)
                {
                    j++;
                }
                if (j > i)
                {
                    List<int> run = items.Skip(i).Take(j - i + 1).Select(x => x.N).ToList();
                    issues.Add(Issue("empty-paragraphs", run.Count + " empty paragraphs in a row (" + run.First() + "-" + run.Last()
                        + ") used as spacing: delete them with writer.deleteParagraphs", new Dictionary<string, object> { { "paragraphs", run } }));
                }
                i = j;
            }
            // Dài hơn số trang người dùng yêu cầu (đề A 26-27/10/2026: 11 trang cho đề 6–8; model thấy số trang nhưng tự cho
            // là "chấp nhận được" vì chỉ là con số, không phải lỗi).
            int maxPages = ParamInt(p, "maxPages", 0);
            if (maxPages > 0 && pages > maxPages)
            {
                int perPage = Math.Max(1, words / Math.Max(1, pages));
                issues.Add(Issue("too-long", "the document has " + pages + " pages but the limit is " + maxPages + ": cut about "
                    + ((pages - maxPages) * perPage) + " words (~" + perPage + " words per page) - shorten long paragraphs and drop "
                    + "repeated points (writer.replaceMany / writer.deleteParagraphs), keep every required section and table",
                    new Dictionary<string, object> { { "pages", pages }, { "maxPages", maxPages } }));
            }
            var headings = items.Where(i => i.Content && i.Level < 10).Take(LayoutHeadingLimit)
                .Select(i => (object)new Dictionary<string, object> { { "n", i.N }, { "level", i.Level }, { "page", i.StartPage }, { "t", Shorten(i.Text, 60) } })
                .ToList();
            var result = new Dictionary<string, object>
            {
                { "pages", pages },
                { "words", words },
                { "paragraphs", total },
                { "tables", tables.Count },
                { "headings", headings },
                { "issueCount", issues.Count },
                { "issues", issues }
            };
            if (total > LayoutParagraphLimit)
            {
                result["checkedParagraphs"] = LayoutParagraphLimit;
            }
            return result;
        }

        private static List<Dictionary<string, object>> ParamBlocks(Dictionary<string, object> p, string name)
        {
            object raw;
            if (p == null || !p.TryGetValue(name, out raw) || !(raw is IEnumerable) || raw is string)
            {
                throw new ArgumentException("'" + name + "' must be an array of blocks, e.g. [{\"heading\": \"1. Mở đầu\", \"level\": 1}, {\"text\": \"...\"}]");
            }
            var blocks = new List<Dictionary<string, object>>();
            foreach (object item in (IEnumerable)raw)
            {
                var block = item as Dictionary<string, object>;
                if (block == null)
                {
                    throw new ArgumentException("block " + (blocks.Count + 1) + " is not an object");
                }
                blocks.Add(block);
            }
            if (blocks.Count == 0)
            {
                throw new ArgumentException("'" + name + "' is empty");
            }
            return blocks;
        }

        private static List<string> BlockLines(Dictionary<string, object> block, string key)
        {
            var lines = new List<string>();
            object raw;
            if (!block.TryGetValue(key, out raw) || raw == null)
            {
                return lines;
            }
            raw = UnwrapScalar(raw);
            if (raw is string)
            {
                lines.AddRange(((string)raw).Replace("\r\n", "\n").Split('\n'));
            }
            else if (raw is IEnumerable)
            {
                foreach (object line in (IEnumerable)raw)
                {
                    lines.Add(Convert.ToString(UnwrapScalar(line)));
                }
            }
            return lines.Select(l => (l ?? "").Trim()).Where(l => l.Length > 0).ToList();
        }

        private static string BlockKind(Dictionary<string, object> block, int index)
        {
            foreach (string kind in new[] { "heading", "text", "bullets", "numbers", "table", "pageBreak" })
            {
                if (block.ContainsKey(kind))
                {
                    return kind;
                }
            }
            throw new ArgumentException("block " + index + " has none of heading, text, bullets, numbers, table, pageBreak (keys: "
                + string.Join(", ", block.Keys) + ")");
        }

        private static Dictionary<string, object> WriterWriteContent(IAppHost host, Dictionary<string, object> p)
        {
            List<Dictionary<string, object>> blocks = ParamBlocks(p, "blocks");
            string bodyAlignment = ParamString(p, "alignment", "justify");
            // Soát hết trước khi ghi: lỗi giữa chừng để lại nửa bài.
            var kinds = new List<string>();
            for (int i = 0; i < blocks.Count; i++)
            {
                string kind = BlockKind(blocks[i], i + 1);
                kinds.Add(kind);
                if (kind == "table")
                {
                    List<IList> values = ParamMatrix(blocks[i], "table", false);
                    if (values == null || values.Count == 0)
                    {
                        throw new ArgumentException("block " + (i + 1) + ": 'table' must be a 2D array of rows");
                    }
                    RejectFormulaCells(values);
                }
                else if ((kind == "bullets" || kind == "numbers") && BlockLines(blocks[i], kind).Count == 0)
                {
                    throw new ArgumentException("block " + (i + 1) + ": '" + kind + "' must be a non-empty array of items");
                }
                else if (kind == "heading" && string.IsNullOrWhiteSpace(ParamString(blocks[i], "heading", null)))
                {
                    throw new ArgumentException("block " + (i + 1) + ": 'heading' is empty");
                }
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            var counts = new Dictionary<string, object>();
            using (new UndoRecordScope(app, "AI: write content"))
            {
                // Viết nối tiếp ở cuối bài (như appendText), không theo con trỏ người dùng đang để.
                app.Selection.EndKey(6);
                for (int i = 0; i < blocks.Count; i++)
                {
                    Dictionary<string, object> block = blocks[i];
                    string kind = kinds[i];
                    counts[kind] = (counts.ContainsKey(kind) ? (int)counts[kind] : 0) + 1;
                    switch (kind)
                    {
                        case "heading":
                            app.Selection.EndKey(6);
                            WriterHeading(host, new Dictionary<string, object>
                            {
                                { "text", ParamString(block, "heading", "").Trim() }, { "level", ParamInt(block, "level", 1) }
                            });
                            break;
                        case "text":
                            var textParams = new Dictionary<string, object>(block);
                            textParams["alignment"] = ParamString(block, "alignment", bodyAlignment);
                            WriterAppendText(host, textParams);
                            break;
                        case "bullets":
                        case "numbers":
                            AppendList(host, doc, BlockLines(block, kind), kind == "numbers");
                            break;
                        case "table":
                            string caption = ParamString(block, "caption", null);
                            if (!string.IsNullOrWhiteSpace(caption))
                            {
                                var captionParams = new Dictionary<string, object> { { "text", caption.Trim() } };
                                if (HasParam(block, "label"))
                                {
                                    captionParams["label"] = ParamString(block, "label", "Bảng");
                                }
                                WriterInsertCaption(host, captionParams);
                            }
                            else
                            {
                                app.Selection.EndKey(6);
                            }
                            var tableParams = new Dictionary<string, object> { { "values", block["table"] } };
                            if (HasParam(block, "style"))
                            {
                                tableParams["style"] = block["style"];
                            }
                            WriterInsertTable(host, tableParams);
                            break;
                        case "pageBreak":
                            if (ParamBool(block, "pageBreak", true))
                            {
                                AppendPageBreak(app, doc);
                            }
                            break;
                    }
                }
            }
            counts["blocks"] = blocks.Count;
            counts["paragraphs"] = Convert.ToInt32(doc.Paragraphs.Count);
            return counts;
        }

        // Danh sách thật (gạch đầu dòng / số tự động của Word), không phải dấu "-" gõ tay. Đoạn sau danh sách là đoạn thường:
        // ngắt đoạn từ mục cuối thì Word kéo danh sách sang cả đoạn kế.
        private static void AppendList(IAppHost host, dynamic doc, List<string> lines, bool numbered)
        {
            WriterAppendText(host, new Dictionary<string, object> { { "text", string.Join("\n", lines) }, { "alignment", "left" } });
            int total = Convert.ToInt32(doc.Paragraphs.Count);
            dynamic range = doc.Range(doc.Paragraphs[total - lines.Count + 1].Range.Start, doc.Paragraphs[total].Range.End);
            // wdBulletGallery = 1, wdNumberGallery = 2; ContinuePreviousList = false: mỗi danh sách số bắt đầu lại từ 1.
            dynamic template = ((dynamic)host.Application).ListGalleries[numbered ? 2 : 1].ListTemplates[1];
            range.ListFormat.ApplyListTemplate(template, false);
            doc.Content.InsertParagraphAfter();
            dynamic next = doc.Paragraphs.Last;
            next.Range.ListFormat.RemoveNumbers();
            next.Style = BuiltinStyle(doc, "normal");
            dynamic cursor = next.Range;
            cursor.Collapse(1);
            cursor.Select();
        }

        // Ngắt trang ở cuối bài, trên một đoạn riêng (không dính vào đoạn chữ trước).
        private static void AppendPageBreak(dynamic app, dynamic doc)
        {
            dynamic last = doc.Paragraphs.Last;
            if (Convert.ToString(last.Range.Text).Trim().Length > 0 || Convert.ToBoolean(last.Range.Information(12)))
            {
                doc.Content.InsertParagraphAfter();
            }
            app.Selection.EndKey(6);
            app.Selection.InsertBreak(7);
        }

        private static Dictionary<string, object> WriterDeleteParagraphs(IAppHost host, Dictionary<string, object> p)
        {
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            List<int> numbers = ParamIntList(p, "paragraphs").Distinct().OrderByDescending(x => x).ToList();
            if (numbers.Count == 0)
            {
                throw new ArgumentException("'paragraphs' must list paragraph numbers from writer.outline / writer.checkLayout");
            }
            int total = Convert.ToInt32(doc.Paragraphs.Count);
            foreach (int n in numbers)
            {
                if (n < 1 || n > total)
                {
                    throw new ArgumentException("paragraph " + n + " does not exist (the document has " + total + ")");
                }
                if (Convert.ToBoolean(doc.Paragraphs[n].Range.Information(12)))
                {
                    throw new ArgumentException("paragraph " + n + " is inside a table; edit the table instead");
                }
                if (InsideToc(doc, Convert.ToInt32(doc.Paragraphs[n].Range.Start)))
                {
                    throw new ArgumentException("paragraph " + n + " is part of the table of contents; it cannot be edited by hand - "
                        + "call writer.insertToc again to refresh it");
                }
            }
            var deleted = new List<object>();
            int length = Convert.ToInt32(doc.Content.End);
            Exception failure = null;
            using (new UndoRecordScope(host.Application, "AI: delete paragraphs"))
            {
                try
                {
                    foreach (int n in numbers)
                    {
                        string text = DeleteParagraph(doc, n);
                        if (text != null)
                        {
                            deleted.Add(Shorten(text, 60));
                        }
                    }
                }
                catch (Exception ex)
                {
                    failure = ex;
                }
            }
            if (failure != null)
            {
                // Tất cả hoặc không: lỗi giữa chừng (Word "Cannot edit Range"...) thì lùi phần đã xoá của chính lệnh này.
                if (Convert.ToInt32(doc.Content.End) != length)
                {
                    try
                    {
                        doc.Undo(1);
                    }
                    catch (Exception ex)
                    {
                        Logger.Info("writer.deleteParagraphs: không lùi được phần đã xoá: " + ex.Message);
                    }
                }
                throw new InvalidOperationException("nothing was deleted: " + failure.Message);
            }
            return new Dictionary<string, object>
            {
                { "deleted", deleted.Count }, { "texts", deleted }, { "paragraphs", Convert.ToInt32(doc.Paragraphs.Count) }
            };
        }

        // Thêm hàng vào bảng có sẵn. Bộ đo 27/10/2026 ca w09 ("thêm một việc vào bảng phân công"): không có lệnh này, model
        // thử replaceAll chuỗi có dấu ô, xoá đoạn trong bảng, undo... hết 60 vòng mà bảng vẫn 4 hàng.
        private static Dictionary<string, object> WriterAddTableRows(IAppHost host, Dictionary<string, object> p)
        {
            List<IList> rows = ParamMatrix(p, "rows", false);
            if (rows == null || rows.Count == 0)
            {
                throw new ArgumentException("'rows' must be a 2D array, e.g. [[\"4\", \"Rà soát hợp đồng\", \"Bà Hà\", \"31/10/2026\"]]");
            }
            RejectFormulaCells(rows);
            dynamic app = host.Application;
            dynamic table = TargetTable(app, p);
            int count = Convert.ToInt32(table.Rows.Count);
            int cols = Convert.ToInt32(table.Columns.Count);
            int before = ParamInt(p, "before", 0);
            if (before < 0 || before > count)
            {
                throw new ArgumentException("'before' must be between 1 and " + count + " (the table has " + count + " rows), got " + before);
            }
            for (int r = 0; r < rows.Count; r++)
            {
                int cells = rows[r] == null ? 0 : rows[r].Count;
                if (cells > cols)
                {
                    throw new ArgumentException("row " + (r + 1) + " has " + cells + " cells but the table has " + cols + " columns");
                }
            }
            using (new UndoRecordScope(app, "AI: add table rows"))
            {
                for (int r = 0; r < rows.Count; r++)
                {
                    // Rows.Add(hàng đích) chèn TRƯỚC hàng đó; hàng mới mang định dạng của hàng kề.
                    dynamic row = before > 0 ? table.Rows.Add(table.Rows[before + r]) : table.Rows.Add();
                    IList values = rows[r] ?? new object[0];
                    for (int c = 0; c < cols; c++)
                    {
                        row.Cells[c + 1].Range.Text = c < values.Count ? Convert.ToString(UnwrapScalar(values[c])) : "";
                    }
                }
            }
            return new Dictionary<string, object> { { "added", rows.Count }, { "rows", Convert.ToInt32(table.Rows.Count) }, { "cols", cols } };
        }

        // Xoá một đoạn; trả chữ đã xoá (null = không xoá được). Đoạn cuối bài không có dấu đoạn riêng để xoá: xoá dấu đoạn của
        // đoạn trước rồi trả lại style + định dạng đoạn cho đoạn trước (đoạn gộp mang định dạng của dấu đoạn còn lại).
        private static string DeleteParagraph(dynamic doc, int n)
        {
            dynamic paragraph = doc.Paragraphs[n];
            string text = Convert.ToString(paragraph.Range.Text).Replace("\f", "").TrimEnd('\r', '\a').Trim();
            int total = Convert.ToInt32(doc.Paragraphs.Count);
            if (n < total)
            {
                paragraph.Range.Delete();
                return text;
            }
            if (n == 1)
            {
                dynamic only = paragraph.Range;
                only.MoveEnd(1, -1);
                only.Delete();
                return text;
            }
            dynamic previous = doc.Paragraphs[n - 1];
            if (Convert.ToBoolean(previous.Range.Information(12)))
            {
                // Sau bảng cuối bài Word luôn giữ một đoạn: chỉ xoá chữ trong đó.
                dynamic body = paragraph.Range;
                body.MoveEnd(1, -1);
                body.Delete();
                return text.Length > 0 ? text : null;
            }
            object style = previous.Style;
            dynamic format = previous.Format.Duplicate;
            int from = Convert.ToInt32(previous.Range.End) - 1;
            int to = Convert.ToInt32(paragraph.Range.End) - 1;
            doc.Range(from, to).Delete();
            dynamic merged = doc.Paragraphs.Last;
            merged.Style = style;
            merged.Format = format;
            return text;
        }

        // Giữ bảng trên một trang: mọi hàng (trừ hàng cuối) "keep with next", không cho một hàng bị cắt ngang trang.
        private static void KeepTableTogether(dynamic table)
        {
            int rows = Convert.ToInt32(table.Rows.Count);
            table.Rows.AllowBreakAcrossPages = 0;
            if (rows > 1)
            {
                dynamic range = table.Range.Document.Range(table.Rows[1].Range.Start, table.Rows[rows - 1].Range.End);
                range.ParagraphFormat.KeepWithNext = -1;
            }
        }
    }
}
