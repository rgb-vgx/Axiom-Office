using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;

namespace AxiomOffice.Bridge
{
    internal static partial class CommandDispatcher
    {
        // Word / WPS Writer (AppKind "wps").
        private static IEnumerable<CommandInfo> WriterCommands()
        {
            return new[]
            {
                Command("writer.newDocument", "wps", WriterNewDocument, "Tạo tài liệu mới"),
                Command("writer.open", "wps", WriterOpen, "Mở .docx/.doc", Req("path")),
                Command("writer.getText", "wps", WriterGetText, "Đọc toàn bộ text", Opt("maxChars")).ForAgent(),
                Command("writer.selection", "wps", WriterSelection, "Text + vị trí đang chọn").ForAgent(),
                Command("writer.typeText", "wps", WriterTypeText, "Gõ tại con trỏ", Req("text")).ForAgent(),
                Command("writer.appendText", "wps", WriterAppendText,
                    "Nối một KHỐI đoạn văn vào cuối tài liệu; `\\n` tách đoạn, tự đặt lại định dạng (không kế thừa), `alignment` mặc định left",
                    Req("text"), Opt("bold"), Opt("italic"), Opt("underline"), Opt("size"), Opt("color"), Opt("font"),
                    Opt("alignment", "left/center/right/justify; mặc định left")).ForAgent(),
                Command("writer.insertStyledText", "wps", WriterInsertStyledText,
                    "Chèn text có định dạng tại con trỏ; `\\n` tách đoạn, `alignment` áp cho từng đoạn (`color` dạng `#RRGGBB`)",
                    Req("text"), Opt("bold"), Opt("italic"), Opt("underline"), Opt("size"), Opt("color"), Opt("font"),
                    Opt("alignment", "left/center/right/justify; áp cho từng đoạn vừa chèn"),
                    Opt("break", "ngắt đoạn sau khối; mặc định true khi có `alignment`")).ForAgent(),
                Command("writer.heading", "wps", WriterHeading, "Heading 1-9 (`level`, mặc định 1) + tự xuống dòng (`break`, mặc định true)",
                    Opt("text"), Opt("level"), Opt("break")).ForAgent(),
                Command("writer.formatSelection", "wps", WriterFormatSelection, "Định dạng vùng chọn",
                    Opt("bold"), Opt("italic"), Opt("underline"), Opt("size"), Opt("color"), Opt("font"), Opt("alignment")).ForAgent(),
                Command("writer.setParagraphAlignment", "wps", WriterSetParagraphAlignment, "Căn đoạn: left/center/right/justify", Req("alignment")).ForAgent(),
                Command("writer.insertTable", "wps", WriterInsertTable, "Chèn bảng; `rows`/`cols` tự suy ra/nới theo `values`",
                    Opt("rows"), Opt("cols"), Opt("values", "2D array of rows"), Opt("style")).ForAgent(),
                Command("writer.formatTable", "wps", WriterFormatTable,
                    "Định dạng bảng CÓ SẴN (không tạo lại): kiểu, font, màu hàng tiêu đề, màu sọc, viền, căn lề, co giãn",
                    Opt("table", "1-based index; default: table at cursor, else last table"), Opt("style", "e.g. 'Grid Table 4 - Accent 1'"),
                    Opt("font"), Opt("size"), Opt("color", "#RRGGBB text color"),
                    Opt("headerFill", "#RRGGBB"), Opt("headerColor", "#RRGGBB header text"), Opt("headerBold"),
                    Opt("bandFill", "#RRGGBB every other data row"), Opt("borderColor", "#RRGGBB"), Opt("borders", "false = no borders (layout tables)"),
                    Opt("alignment", "left/center/right"), Opt("autoFit", "content/window")).ForAgent(),
                Command("writer.insertPageBreak", "wps", WriterInsertPageBreak, "Ngắt trang").ForAgent(),
                Command("writer.insertImage", "wps", WriterInsertImage, "Chèn ảnh tại con trỏ (kích thước theo point)", Req("path"), Opt("width"), Opt("height")).ForAgent(),
                Command("writer.insertHyperlink", "wps", WriterInsertHyperlink, "Chèn liên kết", Req("url"), Opt("text")).ForAgent(),
                Command("writer.replaceAll", "wps", WriterReplaceAll, "Tìm và thay toàn bộ", Req("find"), Opt("replace")).ForAgent(),
                Command("writer.replaceMany", "wps", WriterReplaceMany,
                    "Thay NHIỀU chỗ trong một lần: `edits` = [{find, replace}] theo thứ tự (mỗi cặp thay mọi chỗ khớp) - dùng thay cho nhiều lần writer.replaceAll; trả số chỗ đã thay của từng cặp",
                    Req("edits", "array of {find, replace}; up to 200")).ForAgent(),
                Command("writer.undo", "wps", WriterUndo, "Hoàn tác (mỗi thao tác AI = 1 bước)", Opt("count")).ForAgent(),
                Command("writer.exportPdf", "wps", WriterExportPdf, "Xuất PDF", Req("path")).ForAgent(),
                Command("writer.save", "wps", (host, p) => SaveDocument(host, "wps", null), "Lưu").ForAgent(),
                Command("writer.saveAs", "wps", (host, p) => SaveDocument(host, "wps", ParamString(p, "path", null)), "Lưu thành file mới", Req("path")).ForAgent(),
                Command("writer.closeAll", "wps", WriterCloseAll, "**Đóng mọi tài liệu, không lưu**")
            };
        }

        private static Dictionary<string, object> WriterGetText(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            string text = LabelObjects(doc, Convert.ToString(doc.Content.Text));
            int totalChars = text.Length;
            int maxChars = ParamInt(p, "maxChars", 0);
            bool truncated = false;
            if (maxChars > 0 && text.Length > maxChars)
            {
                text = text.Substring(0, maxChars);
                truncated = true;
            }
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(doc.Name) },
                { "fullName", Convert.ToString(doc.FullName) },
                { "totalChars", totalChars },
                { "truncated", truncated },
                { "text", text }
            };
        }

        private static Dictionary<string, object> WriterNewDocument(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic doc = app.Documents.Add();
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(doc.Name) }
            };
        }

        private static Dictionary<string, object> WriterOpen(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic doc = app.Documents.Open(path);
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(doc.Name) },
                { "fullName", Convert.ToString(doc.FullName) }
            };
        }

        private static Dictionary<string, object> WriterSelection(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic selection = app.Selection;
            return new Dictionary<string, object>
            {
                { "text", Convert.ToString(selection.Text) },
                { "start", Convert.ToInt32(selection.Start) },
                { "end", Convert.ToInt32(selection.End) }
            };
        }

        private static Dictionary<string, object> WriterTypeText(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", null);
            if (text == null)
            {
                return new Dictionary<string, object> { { "typed", 0 } };
            }
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: type text"))
            {
                app.Selection.TypeText(text);
            }
            return new Dictionary<string, object> { { "typed", text.Length } };
        }

        private static Dictionary<string, object> WriterAppendText(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", null);
            if (string.IsNullOrEmpty(text))
            {
                return new Dictionary<string, object> { { "appended", 0 }, { "paragraphs", 0 } };
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            string alignment = ParamString(p, "alignment", "left");
            // Word nhan '\r' la paragraph mark. '\n' dau chuoi chi la y dinh "mo doan": bo di, de chi con
            // MOT lan ngat doan do needBreak quyet dinh. Tranh Range bat dau tren paragraph mark cua doan cu
            // (se dat lai can le ca doan cu) va tranh doan rong khi caller da gui "\r...".
            string normalized = text.Replace("\r\n", "\n").Replace('\r', '\n').TrimStart('\n');
            if (normalized.Length == 0)
            {
                return new Dictionary<string, object> { { "appended", 0 }, { "paragraphs", 0 } };
            }
            normalized = normalized.Replace("\n", "\r");
            bool needBreak = false;
            try
            {
                dynamic docParagraphs = doc.Paragraphs;
                int count = Convert.ToInt32(docParagraphs.Count);
                if (count > 0)
                {
                    dynamic last = docParagraphs.Item(count);
                    string lastText = Convert.ToString(last.Range.Text).Replace("\r", "").Replace("\a", "").Trim();
                    needBreak = lastText.Length > 0;
                }
            }
            catch
            {
                needBreak = false;
            }
            using (new UndoRecordScope(host.Application, "AI: append text"))
            {
                // Ngat doan TRUOC khi do vi tri bat dau: Range chi phu text moi, khong dat lai can le doan cu.
                if (needBreak)
                {
                    doc.Content.InsertAfter("\r");
                }
                int before = Convert.ToInt32(doc.Content.End);
                doc.Content.InsertAfter(normalized);
                int after = Convert.ToInt32(doc.Content.End);
                if (after > before)
                {
                    dynamic inserted = doc.Range(before, after);
                    ApplyWriterFontReset(inserted.Font, p);
                    if (!string.IsNullOrEmpty(alignment))
                    {
                        inserted.ParagraphFormat.Alignment = ParseAlignment(alignment);
                    }
                }
                // Con trỏ về cuối chữ vừa nối: lệnh làm tại con trỏ (heading, insertTable) nối tiếp đúng chỗ thay vì chèn
                // vào vị trí cũ phía trên (bộ thử 26/10/2026: bảng gọi sau appendText nằm trên đoạn vừa nối). Không thêm
                // đoạn trống (tài liệu không được có đoạn rỗng thừa); heading/insertTable tự sang đoạn mới khi con trỏ ở
                // cuối một đoạn có chữ.
                try
                {
                    dynamic cursor = doc.Paragraphs.Last.Range;
                    cursor.MoveEnd(1, -1);
                    cursor.Collapse(0);
                    cursor.Select();
                }
                catch (Exception ex)
                {
                    Logger.Info("writer.appendText: không đưa được con trỏ về cuối: " + ex.Message);
                }
            }
            int paragraphs = normalized.Split('\r').Length;
            return new Dictionary<string, object> { { "appended", normalized.Length }, { "paragraphs", paragraphs } };
        }

        private static Dictionary<string, object> WriterReplaceAll(IAppHost host, Dictionary<string, object> p)
        {
            string find = ParamString(p, "find", null);
            string replace = ParamString(p, "replace", "");
            if (string.IsNullOrEmpty(find))
            {
                return new Dictionary<string, object> { { "replaced", false } };
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            // Không dùng Find.Execute kiểu "thay tất cả" (Replace = wdReplaceAll): lượt thật 14/10/2026 trên Word 365, giáo án
            // GA HG.docx, lệnh này làm SẬP Word cả với chuỗi ngắn (127/167 ký tự); chuỗi > 255 ký tự thì Word báo "String
            // parameter too long" rồi sập ở lần gọi sau. Find chỉ để TÌM (tối đa WordFindLimit ký tự đầu), kiểm đúng nguyên
            // văn rồi gán Range.Text - đã chạy ổn trên Word thật với đoạn 459 ký tự.
            int count;
            using (new UndoRecordScope(host.Application, "AI: replace all"))
            {
                count = ReplaceLong(doc, find, replace);
            }
            return new Dictionary<string, object> { { "replaced", count > 0 }, { "count", count } };
        }

        // Nhiều cặp tìm/thay trong MỘT lệnh (một bước hoàn tác): lượt thật 14/10/2026 sửa giáo án bằng 137 lần
        // writer.replaceAll (nhiều lần chỉ đổi 1-2 ký tự) -> 48 vòng, 1,48 triệu token. Gom lại thì vài vòng là xong.
        private static Dictionary<string, object> WriterReplaceMany(IAppHost host, Dictionary<string, object> p)
        {
            object raw;
            var edits = new List<KeyValuePair<string, string>>();
            if (p != null && p.TryGetValue("edits", out raw) && raw is IEnumerable && !(raw is string))
            {
                foreach (object item in (IEnumerable)raw)
                {
                    var entry = item as Dictionary<string, object>;
                    if (entry == null)
                    {
                        continue;
                    }
                    object find, replace;
                    entry.TryGetValue("find", out find);
                    entry.TryGetValue("replace", out replace);
                    string findText = Convert.ToString(find);
                    if (!string.IsNullOrEmpty(findText))
                    {
                        edits.Add(new KeyValuePair<string, string>(findText, Convert.ToString(replace) ?? ""));
                    }
                }
            }
            if (edits.Count == 0)
            {
                throw new ArgumentException("'edits' must be a non-empty array of {find, replace}");
            }
            if (edits.Count > 200)
            {
                throw new ArgumentException("'edits' has " + edits.Count + " items; send at most 200 per call");
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            var counts = new List<int>();
            var notFound = new List<string>();
            using (new UndoRecordScope(host.Application, "AI: replace many"))
            {
                foreach (var edit in edits)
                {
                    int count = ReplaceLong(doc, edit.Key, edit.Value);
                    counts.Add(count);
                    if (count == 0)
                    {
                        notFound.Add(edit.Key.Length > 60 ? edit.Key.Substring(0, 60) + "..." : edit.Key);
                    }
                }
            }
            var result = new Dictionary<string, object>
            {
                { "edits", edits.Count },
                { "replaced", counts.Sum() },
                { "counts", counts }
            };
            if (notFound.Count > 0)
            {
                result["notFound"] = notFound;
            }
            return result;
        }

        // Giới hạn an toàn của chuỗi Find (Word: 255 ký tự sau khi đổi ^p, ^t...).
        private const int WordFindLimit = 240;

        // Thay chuỗi dài: Find chỉ tìm phần đầu (<= WordFindLimit), kiểm cả đoạn đúng nguyên văn rồi gán Range.Text.
        private static int ReplaceLong(dynamic doc, string find, string replace)
        {
            string target = find.Replace("\r\n", "\r").Replace("\n", "\r");
            string replacement = (replace ?? "").Replace("\r\n", "\r").Replace("\n", "\r");
            string prefix = target;
            while (prefix.Length > 1 && ToWordFindText(prefix).Length > WordFindLimit)
            {
                prefix = prefix.Substring(0, Math.Max(1, Math.Min(prefix.Length - 1, WordFindLimit - (ToWordFindText(prefix).Length - prefix.Length) - 1)));
            }
            int count = 0;
            int start = 0;
            int lockStart, lockEnd;
            // Đang khoá vùng bôi đen: chỉ tìm trong vùng đó (cận cuối tính lại mỗi vòng vì chữ thay có thể dài/ngắn hơn).
            bool locked = LockBounds((object)doc, out lockStart, out lockEnd);
            if (locked)
            {
                start = lockStart;
            }
            int lastAt = -1;
            for (int guard = 0; guard < 200; guard++)
            {
                int end = Convert.ToInt32(doc.Content.End);
                if (locked && LockBounds((object)doc, out lockStart, out lockEnd))
                {
                    end = lockEnd;
                }
                if (start >= end)
                {
                    break;
                }
                dynamic range = doc.Range(start, end);
                dynamic finder = range.Find;
                finder.ClearFormatting();
                bool found = Convert.ToBoolean(finder.Execute(ToWordFindText(prefix), false, false, false, false, false, true, 0));
                if (!found)
                {
                    break;
                }
                int at = Convert.ToInt32(range.Start);
                // Find của Word có lúc trả lại đúng chỗ vừa thay (lượt thật 14/10/2026: thay "+ Cách sửa: " bằng chuỗi bắt đầu
                // bằng chính nó -> chèn lặp 74 lần rồi Word báo "no document is open"). Không tiến lên được thì dừng.
                if (at < start || at <= lastAt)
                {
                    break;
                }
                lastAt = at;
                dynamic whole = doc.Range(at, Math.Min(at + target.Length, end));
                if (string.Equals(Convert.ToString(whole.Text), target, StringComparison.Ordinal))
                {
                    whole.Text = replacement;
                    count++;
                    start = at + replacement.Length;
                }
                else
                {
                    start = Convert.ToInt32(range.End);
                }
            }
            return count;
        }

        private static void ApplyWriterFont(dynamic font, Dictionary<string, object> p)
        {
            if (HasParam(p, "bold"))
            {
                font.Bold = ParamBool(p, "bold", false) ? -1 : 0;
            }
            if (HasParam(p, "italic"))
            {
                font.Italic = ParamBool(p, "italic", false) ? -1 : 0;
            }
            if (HasParam(p, "underline"))
            {
                font.Underline = ParamBool(p, "underline", false) ? 1 : 0;
            }
            if (HasParam(p, "size"))
            {
                font.Size = ParamInt(p, "size", 12);
            }
            if (HasParam(p, "font"))
            {
                font.Name = ParamString(p, "font", null);
            }
            int? color = ParseBgrColor(ParamString(p, "color", null));
            if (color.HasValue)
            {
                font.Color = color.Value;
            }
        }

        // Nhu ApplyWriterFont nhung dat TUONG MINH ca thuoc tinh khong truyen, cho KHOI doan van: Selection
        // giu bold/underline/mau/cỡ/font cua lan chen truoc, nen bo qua se lam dinh sang cac doan sau (do
        // 24/10/2026: ca la don bi bold + gach chan + mau xanh vi hai lan dau bat bold/underline). Khoi
        // khong truyen size/font nhan mac dinh than van ban cua design (Times New Roman 13).
        private static void ApplyWriterFontReset(dynamic font, Dictionary<string, object> p)
        {
            font.Bold = HasParam(p, "bold") && ParamBool(p, "bold", false) ? -1 : 0;
            font.Italic = HasParam(p, "italic") && ParamBool(p, "italic", false) ? -1 : 0;
            font.Underline = HasParam(p, "underline") && ParamBool(p, "underline", false) ? 1 : 0;
            font.Size = HasParam(p, "size") ? ParamInt(p, "size", 13) : 13;
            font.Name = HasParam(p, "font") ? ParamString(p, "font", "Times New Roman") : "Times New Roman";
            int? color = ParseBgrColor(ParamString(p, "color", null));
            font.Color = color.HasValue ? color.Value : -16777216; // wdColorAutomatic
        }

        private static Dictionary<string, object> WriterInsertStyledText(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", null);
            if (string.IsNullOrEmpty(text))
            {
                return new Dictionary<string, object> { { "inserted", 0 }, { "paragraphs", 0 } };
            }
            dynamic app = host.Application;
            string alignment = ParamString(p, "alignment", null);
            // Có alignment = một KHỐI đoạn văn: tự ngắt đoạn sau khối để lần gọi kế tiếp không dính vào
            // đoạn này (đo 24/10/2026: 4 lời gọi header rời nhau dồn thành một đoạn). Không alignment =
            // chèn inline, giữ nguyên hành vi cũ.
            bool block = !string.IsNullOrEmpty(alignment);
            bool breakAfter = HasParam(p, "break") ? ParamBool(p, "break", true) : block;
            // '\n' (hoac '\r') = doan moi: model gui ca khoi van ban mot lan thay vi moi dong mot lenh.
            string[] lines = text.Replace("\r\n", "\n").Replace('\r', '\n').Split('\n');
            using (new UndoRecordScope(host.Application, "AI: insert styled text"))
            {
                for (int i = 0; i < lines.Length; i++)
                {
                    // Khoi: dat lai toan bo dinh dang. Inline: chi dat thuoc tinh duoc truyen, de chu chen
                    // giua cau giu bold/mau cua van ban xung quanh (nut "Chen tra loi" chi gui text).
                    if (block)
                    {
                        ApplyWriterFontReset(app.Selection.Font, p);
                        app.Selection.ParagraphFormat.Alignment = ParseAlignment(alignment);
                    }
                    else
                    {
                        ApplyWriterFont(app.Selection.Font, p);
                    }
                    if (lines[i].Length > 0)
                    {
                        app.Selection.TypeText(lines[i]);
                    }
                    if (i < lines.Length - 1)
                    {
                        app.Selection.TypeParagraph();
                    }
                }
                // Nếu text đã kết thúc bằng '\n' thì con trỏ đã ở đoạn mới; không thêm dòng trống nữa.
                if (breakAfter && lines[lines.Length - 1].Length > 0)
                {
                    app.Selection.TypeParagraph();
                }
            }
            return new Dictionary<string, object> { { "inserted", text.Length }, { "paragraphs", lines.Length } };
        }

        private static Dictionary<string, object> WriterFormatSelection(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: format selection"))
            {
                ApplyWriterFont(app.Selection.Font, p);
                if (HasParam(p, "alignment"))
                {
                    app.Selection.ParagraphFormat.Alignment = ParseAlignment(ParamString(p, "alignment", "left"));
                }
            }
            return new Dictionary<string, object> { { "formatted", true } };
        }

        private static Dictionary<string, object> WriterSetParagraphAlignment(IAppHost host, Dictionary<string, object> p)
        {
            string alignment = ParamString(p, "alignment", "left");
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: paragraph alignment"))
            {
                app.Selection.ParagraphFormat.Alignment = ParseAlignment(alignment);
            }
            return new Dictionary<string, object> { { "alignment", alignment } };
        }

        private static Dictionary<string, object> WriterInsertTable(IAppHost host, Dictionary<string, object> p)
        {
            List<IList> valueRows = ParamMatrix(p, "values", false);
            object rawRows;
            bool rowsHoldData = valueRows == null && p != null && p.TryGetValue("rows", out rawRows)
                && UnwrapScalar(rawRows) is IList;
            if (rowsHoldData)
            {
                // Model nhầm: đặt dữ liệu bảng vào 'rows' thay vì 'values'.
                valueRows = ParamMatrix(p, "rows", false);
            }
            int rows = rowsHoldData ? 0 : ParamInt(p, "rows", 0);
            int cols = ParamInt(p, "cols", 0);
            if (valueRows != null && valueRows.Count > 0)
            {
                // Bảng phải chứa đủ dữ liệu: suy ra/nới rows, cols theo values thay vì cắt bớt.
                rows = Math.Max(rows, valueRows.Count);
                cols = Math.Max(cols, valueRows.Max(row => row.Count));
            }
            if (rows <= 0 || cols <= 0)
            {
                throw new ArgumentException("'rows' and 'cols' are required (or pass 'values' as a 2D array to size the table)");
            }
            // Bảng Word không tính công thức: "=C2/B2*100" hiện nguyên chuỗi cho người đọc (bộ đo 26/10/2026 ca w03). Từ chối
            // trước khi chèn để model ghi số đã tính.
            if (valueRows != null)
            {
                foreach (IList row in valueRows)
                {
                    foreach (object cell in row ?? new object[0])
                    {
                        string text = Convert.ToString(cell);
                        if (text != null && System.Text.RegularExpressions.Regex.IsMatch(text.Trim(), @"^=\s*[A-Za-z(]"))
                        {
                            throw new ArgumentException("Word tables do not calculate formulas: cell '" + text.Trim() +
                                "' would be shown as text. Compute the value yourself and write the number (e.g. 96%)");
                        }
                    }
                }
            }
            dynamic app = host.Application;
            int filled = 0;
            string styleError = null;
            using (new UndoRecordScope(host.Application, "AI: insert table"))
            {
                dynamic table = app.ActiveDocument.Tables.Add(TableAnchor(app), rows, cols);
                if (valueRows != null)
                {
                    for (int r = 0; r < valueRows.Count && r < rows; r++)
                    {
                        IList row = valueRows[r] as IList;
                        if (row == null)
                        {
                            continue;
                        }
                        for (int c = 0; c < row.Count && c < cols; c++)
                        {
                            table.Cell(r + 1, c + 1).Range.Text = Convert.ToString(row[c]);
                            filled++;
                        }
                    }
                }
                // Hàng đầu là tiêu đề: lặp lại ở đầu mỗi trang khi bảng dài qua trang (plan 21 P1; bảng 30 dòng ca w04).
                try
                {
                    table.Rows[1].HeadingFormat = -1;
                }
                catch
                {
                }
                if (HasParam(p, "style"))
                {
                    try
                    {
                        SetTableStyle(table, ParamString(p, "style", "Table Grid"));
                    }
                    catch (Exception ex)
                    {
                        styleError = ex.Message;
                    }
                }
                // Con trỏ ra sau bảng: trước đây Selection nằm ở ô đầu nên lệnh gõ/chèn chữ tiếp theo ghi
                // vào ô (1,1) (log 30/09 01:48: dòng ghi chú lọt vào ô tiêu đề).
                try
                {
                    dynamic after = table.Range;
                    after.Collapse(0);
                    after.Select();
                }
                catch (Exception ex)
                {
                    Logger.Info("writer.insertTable: không đưa được con trỏ ra sau bảng: " + ex.Message);
                }
            }
            var result = new Dictionary<string, object>
            {
                { "rows", rows },
                { "cols", cols },
                { "filled", filled }
            };
            if (styleError != null)
            {
                result["styleError"] = styleError;
            }
            return result;
        }

        // Gán style cho bảng. Với late binding (dynamic) Word KHÔNG có set_Style (chỉ interop mới có) - trước
        // đây lỗi này bị nuốt nên style bảng chưa bao giờ được áp. Gán thuộc tính Style trước, set_Style dự phòng.
        private static void SetTableStyle(dynamic table, string name)
        {
            try
            {
                table.Style = name;
            }
            catch (Microsoft.CSharp.RuntimeBinder.RuntimeBinderException)
            {
                table.set_Style(name);
            }
        }

        // Định dạng bảng đã có. Trước đây không có lệnh này nên agent phải undo rồi dựng lại cả bảng
        // (log 30/09 01:47). Mỗi phần định dạng bọc try riêng: phần nào host không hỗ trợ (WPS) thì bỏ qua
        // và báo trong "skipped" thay vì làm hỏng cả lệnh.
        // Bảng dàn trang (phần đầu / phần ký văn bản hành chính) mang tem trong Table.Title (Word 2010+), giữ
        // theo .docx sang lần mở sau. Cùng tên với LibreOffice (tên bảng) và WPS (Title) để báo cáo thống nhất.
        internal const string LayoutTableName = "AxiomLayout";
        private const string DataTableName = "AxiomTable";

        private static void MarkLayoutTable(dynamic table, bool layout)
        {
            string current = "";
            try
            {
                current = Convert.ToString(table.Title) ?? "";
            }
            catch
            {
            }
            if (layout)
            {
                table.Title = LayoutTableName;
            }
            else if (current.StartsWith(LayoutTableName, StringComparison.Ordinal))
            {
                table.Title = DataTableName;
            }
        }

        internal static bool IsLayoutTable(dynamic table)
        {
            try
            {
                return (Convert.ToString(table.Title) ?? "").StartsWith(LayoutTableName, StringComparison.Ordinal);
            }
            catch
            {
                return false;
            }
        }

        private static Dictionary<string, object> WriterFormatTable(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            int count = Convert.ToInt32(doc.Tables.Count);
            if (count == 0)
            {
                throw new ArgumentException("the document has no table - use writer.insertTable first");
            }
            int index = ParamInt(p, "table", 0);
            dynamic table = null;
            if (index == 0)
            {
                try
                {
                    if (Convert.ToBoolean(app.Selection.Information(12)))
                    {
                        table = app.Selection.Tables[1];
                    }
                }
                catch (Exception)
                {
                }
                if (table == null)
                {
                    index = count;
                }
            }
            if (table == null)
            {
                if (index < 1 || index > count)
                {
                    throw new ArgumentException("'table' must be between 1 and " + count + ", got " + index);
                }
                table = doc.Tables[index];
            }

            var skipped = new List<string>();
            var applied = new List<string>();
            Action<string, Action> step = delegate(string name, Action body)
            {
                try
                {
                    body();
                    applied.Add(name);
                }
                catch (Exception ex)
                {
                    skipped.Add(name + ": " + ex.Message);
                }
            };

            using (new UndoRecordScope(host.Application, "AI: format table"))
            {
                if (HasParam(p, "style"))
                {
                    step("style", delegate { SetTableStyle(table, ParamString(p, "style", "Table Grid")); });
                }
                if (HasParam(p, "font"))
                {
                    step("font", delegate { table.Range.Font.Name = ParamString(p, "font", null); });
                }
                if (HasParam(p, "size"))
                {
                    step("size", delegate { table.Range.Font.Size = ParamInt(p, "size", 11); });
                }
                int? color = ParseBgrColor(ParamString(p, "color", null));
                if (color.HasValue)
                {
                    step("color", delegate { table.Range.Font.Color = color.Value; });
                }
                if (HasParam(p, "alignment"))
                {
                    step("alignment", delegate { table.Range.ParagraphFormat.Alignment = ParseAlignment(ParamString(p, "alignment", "left")); });
                }
                if (HasParam(p, "borders"))
                {
                    // Bảng dàn trang (vd phần đầu công văn: cơ quan | quốc hiệu) không có viền.
                    bool borders = ParamBool(p, "borders", true);
                    // Kẻ khung = bảng dữ liệu, bỏ khung = bảng dàn trang: kèm luôn tem (cùng tên với LibreOffice/WPS)
                    // để checkTables miễn empty-cells và restyle không kẻ lại khung. Không đoán theo nét kẻ - bảng
                    // dữ liệu trần cũng không có nét kẻ.
                    step("borders", delegate
                    {
                        table.Borders.Enable = borders ? 1 : 0;
                        MarkLayoutTable(table, !borders);
                    });
                    if (!borders)
                    {
                        // Đọc lại để XÁC NHẬN tem đã ghi thật (Table.Title cần Word 2010+), như bản LibreOffice.
                        step("layout-table", delegate
                        {
                            if (!IsLayoutTable(table))
                            {
                                throw new InvalidOperationException("could not mark the table as a layout table (Table.Title)");
                            }
                        });
                    }
                }
                int? borderColor = ParseBgrColor(ParamString(p, "borderColor", null));
                if (borderColor.HasValue)
                {
                    step("borderColor", delegate
                    {
                        table.Borders.Enable = 1;
                        table.Borders.OutsideColor = borderColor.Value;
                        table.Borders.InsideColor = borderColor.Value;
                        MarkLayoutTable(table, false);
                    });
                }

                int rowCount = Convert.ToInt32(table.Rows.Count);
                int? bandFill = ParseBgrColor(ParamString(p, "bandFill", null));
                if (bandFill.HasValue)
                {
                    step("bandFill", delegate
                    {
                        for (int r = 3; r <= rowCount; r += 2)
                        {
                            table.Rows[r].Shading.BackgroundPatternColor = bandFill.Value;
                        }
                    });
                }
                int? headerFill = ParseBgrColor(ParamString(p, "headerFill", null));
                int? headerColor = ParseBgrColor(ParamString(p, "headerColor", null));
                bool headerBold = ParamBool(p, "headerBold", headerFill.HasValue || headerColor.HasValue);
                if (headerFill.HasValue)
                {
                    step("headerFill", delegate { table.Rows[1].Shading.BackgroundPatternColor = headerFill.Value; });
                }
                if (headerColor.HasValue)
                {
                    step("headerColor", delegate { table.Rows[1].Range.Font.Color = headerColor.Value; });
                }
                if (HasParam(p, "headerBold") || headerFill.HasValue || headerColor.HasValue)
                {
                    step("headerBold", delegate
                    {
                        table.Rows[1].Range.Font.Bold = headerBold ? -1 : 0;
                        table.Rows[1].HeadingFormat = -1;
                    });
                }
                if (HasParam(p, "autoFit"))
                {
                    string fit = (ParamString(p, "autoFit", "content") ?? "content").Trim().ToLowerInvariant();
                    step("autoFit", delegate { table.AutoFitBehavior(fit == "window" ? 2 : 1); });
                }
            }

            var result = new Dictionary<string, object>
            {
                { "table", index == 0 ? (object)"at cursor" : index },
                { "rows", Convert.ToInt32(table.Rows.Count) },
                { "cols", Convert.ToInt32(table.Columns.Count) },
                { "applied", applied.ToArray() }
            };
            if (skipped.Count > 0)
            {
                result["skipped"] = skipped.ToArray();
            }
            return result;
        }

        // Chỗ chèn bảng: điểm cuối vùng chọn (Tables.Add trên vùng chọn có nội dung sẽ thay nó, gặp nội
        // dung không xoá được thì lỗi "The range cannot be deleted"); đang ở trong bảng thì chèn sau bảng.
        // Vị trí thu gọn nằm ở cuối chữ của một đoạn có chữ (không phải đoạn trống, không trong bảng).
        private static bool AtEndOfFilledParagraph(dynamic range)
        {
            try
            {
                if (Convert.ToInt32(range.Start) != Convert.ToInt32(range.End) || Convert.ToBoolean(range.Information(12)))
                {
                    return false;
                }
                dynamic paragraph = range.Paragraphs[1].Range;
                string text = Convert.ToString(paragraph.Text).TrimEnd('\r', '\a');
                return text.Trim().Length > 0 && Convert.ToInt32(range.Start) >= Convert.ToInt32(paragraph.End) - 1;
            }
            catch
            {
                return false;
            }
        }

        private static dynamic TableAnchor(dynamic app)
        {
            dynamic range = app.Selection.Range;
            range.Collapse(0);
            // Con trỏ ở cuối một đoạn có chữ (vd sau writer.appendText): bảng thành đoạn riêng NGAY SAU đoạn đó.
            if (AtEndOfFilledParagraph(range))
            {
                range.InsertParagraphAfter();
                range.Collapse(0);
            }
            try
            {
                if (Convert.ToBoolean(range.Information(12)))
                {
                    range = range.Tables[1].Range;
                    range.Collapse(0);
                }
                // Bảng mới liền ngay sau bảng cũ thì Word GỘP hai bảng: bảng trả về mang số cột của bảng cũ nên
                // Cell(r, c) lỗi "The requested member of the collection does not exist". Chèn đoạn ngăn cách.
                int start = Convert.ToInt32(range.Start);
                if (start > 0)
                {
                    dynamic before = app.ActiveDocument.Range(start - 1, start);
                    if (Convert.ToBoolean(before.Information(12)))
                    {
                        range.InsertParagraphBefore();
                        range.Collapse(0);
                    }
                }
                // Tương tự khi đoạn chứa điểm chèn nằm ngay TRƯỚC một bảng.
                int paragraphEnd = Convert.ToInt32(range.Paragraphs[1].Range.End);
                int documentEnd = Convert.ToInt32(app.ActiveDocument.Content.End);
                if (paragraphEnd < documentEnd)
                {
                    dynamic after = app.ActiveDocument.Range(paragraphEnd, paragraphEnd + 1);
                    if (Convert.ToBoolean(after.Information(12)))
                    {
                        range.InsertParagraphAfter();
                        range.Collapse(1);
                    }
                }
            }
            catch (Exception ex)
            {
                Logger.Info("writer.insertTable: kiểm tra vị trí chèn bỏ qua: " + ex.Message);
            }
            return range;
        }

        // Văn bản Word: đoạn = \r, xuống dòng thủ công = \v. Model gửi \n nên Find không bao giờ khớp;
        // đổi sang mã đặc biệt của Find (^p, ^l, ^t) - dấu ^ có sẵn phải thoát thành ^^.
        internal static string ToWordFindText(string text)
        {
            if (string.IsNullOrEmpty(text))
            {
                return text;
            }
            return text.Replace("^", "^^")
                .Replace("\r\n", "^p")
                .Replace("\n", "^p")
                .Replace("\r", "^p")
                .Replace("\v", "^l")
                .Replace("\t", "^t");
        }

        private static Dictionary<string, object> WriterInsertPageBreak(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: page break"))
            {
                app.Selection.InsertBreak(7);
            }
            return new Dictionary<string, object> { { "inserted", true } };
        }

        private static Dictionary<string, object> WriterInsertImage(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: insert image"))
            {
                dynamic shape = app.Selection.InlineShapes.AddPicture(path, false, true);
                if (HasParam(p, "width"))
                {
                    shape.Width = ParamInt(p, "width", 0);
                }
                if (HasParam(p, "height"))
                {
                    shape.Height = ParamInt(p, "height", 0);
                }
                return new Dictionary<string, object>
                {
                    { "width", Convert.ToDouble(shape.Width) },
                    { "height", Convert.ToDouble(shape.Height) }
                };
            }
        }

        private static Dictionary<string, object> WriterInsertHyperlink(IAppHost host, Dictionary<string, object> p)
        {
            string url = ParamString(p, "url", null);
            if (string.IsNullOrEmpty(url))
            {
                throw new InvalidOperationException("'url' is required");
            }
            string text = ParamString(p, "text", null);
            if (string.IsNullOrEmpty(text))
            {
                text = url;
            }
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: insert hyperlink"))
            {
                int start = Convert.ToInt32(app.Selection.Start);
                app.Selection.TypeText(text);
                int end = Convert.ToInt32(app.Selection.Start);
                dynamic range = app.ActiveDocument.Range(start, end);
                app.ActiveDocument.Hyperlinks.Add(range, url);
            }
            return new Dictionary<string, object> { { "text", text }, { "url", url } };
        }

        private static Dictionary<string, object> WriterHeading(IAppHost host, Dictionary<string, object> p)
        {
            int level = Math.Max(1, Math.Min(9, ParamInt(p, "level", 1)));
            string text = ParamString(p, "text", null);
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: heading"))
            {
                if (!string.IsNullOrEmpty(text))
                {
                    // Con trỏ ở cuối một đoạn có chữ (vd sau writer.appendText): sang đoạn mới, không biến đoạn đó thành tiêu đề.
                    if (AtEndOfFilledParagraph(app.Selection.Range))
                    {
                        app.Selection.TypeParagraph();
                    }
                    app.Selection.TypeText(text);
                }
                string styleName = "Heading " + level;
                try
                {
                    app.Selection.set_Style(styleName);
                }
                catch
                {
                    app.Selection.Style = styleName;
                }
                if (!string.IsNullOrEmpty(text) && ParamBool(p, "break", true))
                {
                    try
                    {
                        app.Selection.TypeParagraph();
                    }
                    catch
                    {
                    }
                }
            }
            return new Dictionary<string, object> { { "level", level } };
        }

        private static Dictionary<string, object> WriterUndo(IAppHost host, Dictionary<string, object> p)
        {
            int count = Math.Max(1, ParamInt(p, "count", 1));
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            int undone = 0;
            for (int i = 0; i < count; i++)
            {
                try
                {
                    doc.Undo(1);
                    undone++;
                }
                catch (COMException ex)
                {
                    if (IsRetryableComError(ex))
                    {
                        throw;
                    }
                    Logger.Info("writer.undo stopped: " + ex.Message);
                    break;
                }
                catch (Exception ex)
                {
                    Logger.Info("writer.undo stopped: " + ex.Message);
                    break;
                }
            }
            return new Dictionary<string, object> { { "undone", undone } };
        }

        private static Dictionary<string, object> WriterExportPdf(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            doc.ExportAsFixedFormat(path, 17);
            return new Dictionary<string, object> { { "exported", path } };
        }

        private static Dictionary<string, object> WriterCloseAll(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            int closed = 0;
            try
            {
                app.DisplayAlerts = 0;
            }
            catch
            {
            }
            try
            {
                dynamic documents = app.Documents;
                int count = Convert.ToInt32(documents.Count);
                for (int i = count; i >= 1; i--)
                {
                    try
                    {
                        documents[i].Close(0);
                        closed++;
                    }
                    catch
                    {
                    }
                }
            }
            finally
            {
                try
                {
                    app.DisplayAlerts = -1;
                }
                catch
                {
                }
            }
            return new Dictionary<string, object> { { "closed", closed } };
        }
    }
}
