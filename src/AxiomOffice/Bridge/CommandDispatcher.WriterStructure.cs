using System;
using System.Collections;
using System.Collections.Generic;

namespace AxiomOffice.Bridge
{
    // Cấu trúc tài liệu Word có ngữ nghĩa (docs/plans/21-v0 P1): gán style cho đoạn có sẵn, mục lục, đầu/chân trang + số
    // trang, chú thích bảng/hình có đánh số. Bộ đo Word 26/10/2026: thiếu các lệnh này thì model gõ "MỤC LỤC" bằng tay,
    // thay chữ tạm "@@H1@@" rồi gõ lại để có Heading (ca w08 mất cả bài), caption là đoạn chữ thường.
    internal static partial class CommandDispatcher
    {
        // Style dựng sẵn của Word theo hằng WdBuiltinStyle: tên style bị dịch theo ngôn ngữ giao diện ("Heading 1" có thể
        // là "Đề mục 1"), hằng số thì không.
        private static readonly Dictionary<string, int> BuiltinStyles = new Dictionary<string, int>(StringComparer.OrdinalIgnoreCase)
        {
            { "normal", -1 }, { "heading 1", -2 }, { "heading 2", -3 }, { "heading 3", -4 }, { "heading 4", -5 },
            { "heading 5", -6 }, { "heading 6", -7 }, { "heading 7", -8 }, { "heading 8", -9 }, { "heading 9", -10 },
            { "caption", -35 }, { "list bullet", -49 }, { "list number", -50 }, { "title", -63 }, { "subtitle", -75 },
            { "quote", -181 }
        };

        private static IEnumerable<CommandInfo> WriterStructureCommands()
        {
            return new[]
            {
                Command("writer.setStyle", "wps", WriterSetStyle,
                    "Gán style cho đoạn CÓ SẴN (không đổi chữ): `paragraphs` = số đoạn theo writer.outline; bỏ trống = các đoạn đang chọn. Style: Heading 1-9, Normal, Title, Subtitle, List Bullet, List Number, Caption, Quote",
                    Req("style", "e.g. 'Heading 2', 'Normal', 'List Bullet'"),
                    Opt("paragraphs", "array of paragraph numbers from writer.outline (n); default: the selected paragraphs")).ForAgent(),
                Command("writer.insertToc", "wps", WriterInsertToc,
                    "Mục lục tự động từ các Heading: chưa có thì chèn tại con trỏ (hoặc trước đoạn `before`), đã có thì cập nhật số trang - gọi lại sau khi viết xong",
                    Opt("levels", "heading levels 1-9, default 3"),
                    Opt("before", "paragraph number from writer.outline to insert before; default: at the cursor")).ForAgent(),
                Command("writer.setHeaderFooter", "wps", WriterSetHeaderFooter,
                    "Đầu trang / chân trang cho mọi section (thay nội dung cũ) và số trang tự động",
                    Opt("header", "header text; \"\" clears it"), Opt("footer", "footer text; \"\" clears it"),
                    Opt("pageNumber", "true or left/center/right: page number field in the footer (after footer text)"),
                    Opt("align", "left/center/right for header and footer text, default center")).ForAgent(),
                Command("writer.insertCaption", "wps", WriterInsertCaption,
                    "Chú thích có đánh số tự động (style Caption + trường SEQ) \"Bảng 1. ...\": mặc định ở CUỐI bài - gọi ngay trước writer.insertTable; `before` = số đoạn/bảng theo writer.outline để chú thích bảng có sẵn",
                    Req("text", "caption text without the number, e.g. 'Kết quả tuyển sinh 2024-2026'"),
                    Opt("label", "Bảng (default) / Hình / Biểu đồ..."),
                    Opt("before", "paragraph/table number from writer.outline to caption; default: end of the document")).ForAgent()
            };
        }

        private static dynamic BuiltinStyle(dynamic doc, string name)
        {
            string key = (name ?? "").Trim();
            int id;
            if (BuiltinStyles.TryGetValue(key, out id))
            {
                return doc.Styles[(object)id];
            }
            // Style riêng của tài liệu (tên hiển thị): Word báo lỗi nếu không có - chuyển thành lỗi đọc được.
            try
            {
                return doc.Styles[(object)key];
            }
            catch (Exception)
            {
                throw new ArgumentException("unknown style '" + name + "'; use Heading 1-9, Normal, Title, Subtitle, List Bullet, List Number, Caption or Quote");
            }
        }

        private static List<int> ParamIntList(Dictionary<string, object> p, string name)
        {
            var numbers = new List<int>();
            object raw;
            if (p == null || !p.TryGetValue(name, out raw) || raw == null)
            {
                return numbers;
            }
            raw = UnwrapScalar(raw);
            IList list = raw as IList;
            if (list == null || raw is string)
            {
                list = new[] { raw };
            }
            foreach (object item in list)
            {
                int value;
                if (int.TryParse(Convert.ToString(UnwrapScalar(item), System.Globalization.CultureInfo.InvariantCulture), out value))
                {
                    numbers.Add(value);
                }
            }
            return numbers;
        }

        private static Dictionary<string, object> WriterSetStyle(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            string styleName = ParamString(p, "style", null);
            if (string.IsNullOrEmpty(styleName))
            {
                throw new ArgumentException("'style' is required, e.g. 'Heading 2'");
            }
            List<int> numbers = ParamIntList(p, "paragraphs");
            if (numbers.Count > 0 && SelectionLocked)
            {
                throw new InvalidOperationException("the user's selection is locked: call writer.setStyle without 'paragraphs' to style the selected paragraphs only");
            }
            dynamic style = BuiltinStyle(doc, styleName);
            int total = Convert.ToInt32(doc.Paragraphs.Count);
            var applied = new List<object>();
            using (new UndoRecordScope(host.Application, "AI: set style"))
            {
                if (numbers.Count == 0)
                {
                    foreach (dynamic paragraph in app.Selection.Paragraphs)
                    {
                        paragraph.Style = style;
                        applied.Add(Convert.ToString(paragraph.Range.Text).Trim());
                    }
                }
                else
                {
                    foreach (int n in numbers)
                    {
                        if (n < 1 || n > total)
                        {
                            throw new ArgumentException("paragraph " + n + " does not exist (the document has " + total + "; numbers come from writer.outline)");
                        }
                    }
                    foreach (int n in numbers)
                    {
                        dynamic paragraph = doc.Paragraphs[n];
                        paragraph.Style = style;
                        string text = Convert.ToString(paragraph.Range.Text).Trim();
                        applied.Add(text.Length > 60 ? text.Substring(0, 60) + "…" : text);
                    }
                }
            }
            return new Dictionary<string, object>
            {
                { "style", Convert.ToString(style.NameLocal) }, { "paragraphs", applied.Count }, { "texts", applied }
            };
        }

        private static Dictionary<string, object> WriterInsertToc(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            int levels = Math.Max(1, Math.Min(9, ParamInt(p, "levels", 3)));
            int existing = Convert.ToInt32(doc.TablesOfContents.Count);
            using (new UndoRecordScope(host.Application, "AI: table of contents"))
            {
                if (existing > 0)
                {
                    foreach (dynamic toc in doc.TablesOfContents)
                    {
                        toc.Update();
                    }
                    return new Dictionary<string, object> { { "updated", existing }, { "entries", TocEntries(doc) } };
                }
                dynamic where;
                int before = ParamInt(p, "before", 0);
                if (before > 0)
                {
                    int total = Convert.ToInt32(doc.Paragraphs.Count);
                    if (before > total)
                    {
                        throw new ArgumentException("paragraph " + before + " does not exist (the document has " + total + ")");
                    }
                    where = doc.Paragraphs[before].Range;
                    where.InsertParagraphBefore();
                    // Đoạn mới mang style của đoạn đích (thường là Heading): đặt Normal để không thành tiêu đề trống.
                    doc.Paragraphs[before].Style = BuiltinStyle(doc, "normal");
                    where = doc.Paragraphs[before].Range;
                    where.Collapse(1);
                }
                else
                {
                    where = app.Selection.Range;
                    where.Collapse(1);
                }
                // UseHeadingStyles = true, UpperHeadingLevel 1, LowerHeadingLevel = levels.
                dynamic created = doc.TablesOfContents.Add(where, true, 1, levels);
                created.Update();
                // Con trỏ ra đoạn SAU mục lục (ngoài trường TOC - chữ trong đó không sửa được: "Cannot edit Range"); mục lục
                // ở cuối bài thì thêm một đoạn trống để viết tiếp.
                try
                {
                    dynamic last = created.Range.Paragraphs.Last;
                    dynamic next = last.Next();
                    if (next == null)
                    {
                        last.Range.InsertParagraphAfter();
                        next = last.Next();
                        next.Style = BuiltinStyle(doc, "normal");
                    }
                    dynamic cursor = next.Range;
                    cursor.Collapse(1);
                    cursor.Select();
                    if (before > 0)
                    {
                        // Mục lục chèn lên trên phần đã viết: viết tiếp ở cuối bài (wdStory = 6).
                        app.Selection.EndKey(6);
                    }
                }
                catch
                {
                }
            }
            return new Dictionary<string, object> { { "inserted", true }, { "levels", levels }, { "entries", TocEntries(doc) } };
        }

        // Đoạn trống để đặt chú thích. Mặc định ở CUỐI bài (model viết nối tiếp bằng appendText/insertTable - bảng vừa
        // chèn nằm ngay dưới); `before` = số đoạn theo writer.outline (một bảng là số đoạn của ô đầu) thì chèn ngay trước
        // đoạn/bảng đó. Không dựa vào con trỏ: bộ thử 26/10/2026 con trỏ nằm trong mục lục hoặc đầu một Heading làm hỏng
        // chú thích và bảng sau đó.
        private static dynamic CaptionParagraph(dynamic app, dynamic doc, int before)
        {
            if (before <= 0)
            {
                dynamic last = doc.Paragraphs.Last;
                if (Convert.ToString(last.Range.Text).Trim().Length == 0 && !Convert.ToBoolean(last.Range.Information(12)))
                {
                    return last;
                }
                doc.Content.InsertParagraphAfter();
                return doc.Paragraphs.Last;
            }
            int total = Convert.ToInt32(doc.Paragraphs.Count);
            if (before > total)
            {
                throw new ArgumentException("paragraph " + before + " does not exist (the document has " + total + "; numbers come from writer.outline)");
            }
            dynamic anchor = doc.Paragraphs[before].Range;
            if (Convert.ToBoolean(anchor.Information(12)))
            {
                dynamic table = anchor.Tables[1];
                int start = Convert.ToInt32(table.Range.Start);
                if (start == 0)
                {
                    // Bảng ở đầu tài liệu: tách tại hàng đầu thì Word thêm một đoạn ngay trên bảng.
                    table.Rows[1].Range.Select();
                    app.Selection.SplitTable();
                    return doc.Paragraphs[1];
                }
                // Tách ở CUỐI CHỮ của đoạn ngay trên bảng: chèn dấu đoạn sát trước bảng thì Word đưa đoạn mới vào ô đầu.
                dynamic previous = doc.Range(start - 1, start - 1).Paragraphs[1];
                dynamic split = previous.Range;
                split.MoveEnd(1, -1);
                split.Collapse(0);
                split.InsertParagraphAfter();
                // Đoạn trống vừa tạo là đoạn ngay trước điểm bắt đầu (mới) của bảng; Paragraph.Next() ở đây trỏ vào ô đầu.
                int moved = Convert.ToInt32(table.Range.Start);
                return doc.Range(moved - 1, moved - 1).Paragraphs[1];
            }
            anchor.InsertParagraphBefore();
            return doc.Paragraphs[before];
        }

        private static bool InsideToc(dynamic doc, int position)
        {
            try
            {
                foreach (dynamic toc in doc.TablesOfContents)
                {
                    if (position >= Convert.ToInt32(toc.Range.Start) && position <= Convert.ToInt32(toc.Range.End))
                    {
                        return true;
                    }
                }
            }
            catch
            {
            }
            return false;
        }

        private static int TocEntries(dynamic doc)
        {
            try
            {
                return Convert.ToInt32(doc.TablesOfContents[1].Range.Paragraphs.Count);
            }
            catch
            {
                return 0;
            }
        }

        private static int AlignmentOf(string value, int fallback)
        {
            switch ((value ?? "").Trim().ToLowerInvariant())
            {
                case "left": return 0;
                case "center": return 1;
                case "right": return 2;
                default: return fallback;
            }
        }

        private static Dictionary<string, object> WriterSetHeaderFooter(IAppHost host, Dictionary<string, object> p)
        {
            dynamic doc = ((dynamic)host.Application).ActiveDocument;
            bool hasHeader = HasParam(p, "header");
            bool hasFooter = HasParam(p, "footer");
            object rawNumber;
            string numberAlign = null;
            if (p != null && p.TryGetValue("pageNumber", out rawNumber) && rawNumber != null)
            {
                object value = UnwrapScalar(rawNumber);
                if (value is bool)
                {
                    numberAlign = (bool)value ? "center" : null;
                }
                else
                {
                    string text = Convert.ToString(value).Trim().ToLowerInvariant();
                    numberAlign = text == "false" || text == "" ? null : (text == "true" ? "center" : text);
                }
            }
            if (!hasHeader && !hasFooter && numberAlign == null)
            {
                throw new ArgumentException("pass 'header', 'footer' and/or 'pageNumber'");
            }
            int align = AlignmentOf(ParamString(p, "align", "center"), 1);
            int sections = 0;
            using (new UndoRecordScope(host.Application, "AI: header/footer"))
            {
                foreach (dynamic section in doc.Sections)
                {
                    sections++;
                    // wdHeaderFooterPrimary = 1.
                    if (hasHeader)
                    {
                        dynamic header = section.Headers[1].Range;
                        header.Text = ParamString(p, "header", "") ?? "";
                        header.ParagraphFormat.Alignment = align;
                    }
                    if (hasFooter || numberAlign != null)
                    {
                        dynamic footer = section.Footers[1].Range;
                        if (hasFooter)
                        {
                            footer.Text = ParamString(p, "footer", "") ?? "";
                        }
                        if (numberAlign != null)
                        {
                            string existing = Convert.ToString(section.Footers[1].Range.Text).Trim();
                            dynamic end = section.Footers[1].Range;
                            if (existing.Length > 0)
                            {
                                end.InsertParagraphAfter();
                                end = section.Footers[1].Range;
                            }
                            end.MoveEnd(1, -1);
                            end.Collapse(0);
                            end.InsertAfter("Trang ");
                            end.Collapse(0);
                            // wdFieldPage = 33.
                            end.Fields.Add(end, 33);
                            dynamic last = section.Footers[1].Range.Paragraphs.Last;
                            last.Alignment = AlignmentOf(numberAlign, 1);
                        }
                        else
                        {
                            footer.ParagraphFormat.Alignment = align;
                        }
                    }
                }
            }
            return new Dictionary<string, object>
            {
                { "sections", sections }, { "header", hasHeader }, { "footer", hasFooter }, { "pageNumber", numberAlign != null }
            };
        }

        private static Dictionary<string, object> WriterInsertCaption(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            string text = ParamString(p, "text", null);
            if (string.IsNullOrEmpty(text))
            {
                throw new ArgumentException("'text' is required");
            }
            string label = (ParamString(p, "label", "Bảng") ?? "Bảng").Trim();
            if (label.Length == 0)
            {
                label = "Bảng";
            }
            string number;
            int before = ParamInt(p, "before", 0);
            using (new UndoRecordScope(host.Application, "AI: caption"))
            {
                dynamic target = CaptionParagraph(app, doc, before);
                target.Style = BuiltinStyle(doc, "caption");
                // Dựng bằng Range (không gõ theo con trỏ): "Nhãn #. chữ" rồi thay "#" bằng trường SEQ - đếm theo nhãn, "Bảng"
                // và "Hình" đánh số riêng. wdFieldEmpty = -1, PreserveFormatting = false.
                dynamic body = target.Range;
                body.MoveEnd(1, -1);
                body.Text = label + " #. " + text;
                int hash = Convert.ToInt32(body.Start) + label.Length + 1;
                dynamic slot = doc.Range(hash, hash + 1);
                string seq = label.Replace(" ", "_");
                dynamic field = doc.Fields.Add(slot, -1, "SEQ " + seq + " \\* ARABIC", false);
                field.Update();
                number = Convert.ToString(field.Result.Text);
                if (before <= 0)
                {
                    // Cuối bài: thêm đoạn Normal sau chú thích, con trỏ ở đó - bảng/hình chèn tiếp theo nằm ngay dưới.
                    doc.Content.InsertParagraphAfter();
                    dynamic next = doc.Paragraphs.Last;
                    next.Style = BuiltinStyle(doc, "normal");
                    dynamic cursor = next.Range;
                    cursor.Collapse(1);
                    cursor.Select();
                }
                if (before > 0)
                {
                    // Chú thích cho bảng/đoạn có sẵn thì số thứ tự các chú thích sau đó cũng đổi.
                    doc.Fields.Update();
                }
            }
            return new Dictionary<string, object> { { "caption", label + " " + number + ". " + text }, { "number", number } };
        }
    }
}
