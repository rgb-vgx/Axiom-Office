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
                Command("writer.appendText", "wps", WriterAppendText, "Nối vào cuối tài liệu", Req("text")).ForAgent(),
                Command("writer.insertStyledText", "wps", WriterInsertStyledText, "Chèn text có định dạng tại con trỏ (`color` dạng `#RRGGBB`)",
                    Req("text"), Opt("bold"), Opt("italic"), Opt("underline"), Opt("size"), Opt("color"), Opt("font")).ForAgent(),
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
            string text = Convert.ToString(doc.Content.Text);
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
            if (text == null)
            {
                return new Dictionary<string, object> { { "appended", 0 } };
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            using (new UndoRecordScope(host.Application, "AI: append text"))
            {
                doc.Content.InsertAfter(text);
            }
            return new Dictionary<string, object> { { "appended", text.Length } };
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
            dynamic result;
            using (new UndoRecordScope(host.Application, "AI: replace all"))
            {
                result = doc.Content.Find.Execute(ToWordFindText(find), false, false, false, false, false, true, 1, false, ToWordFindText(replace), 2);
            }
            return new Dictionary<string, object> { { "replaced", Convert.ToBoolean(result) } };
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

        private static Dictionary<string, object> WriterInsertStyledText(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", null);
            if (string.IsNullOrEmpty(text))
            {
                return new Dictionary<string, object> { { "inserted", 0 } };
            }
            dynamic app = host.Application;
            using (new UndoRecordScope(host.Application, "AI: insert styled text"))
            {
                ApplyWriterFont(app.Selection.Font, p);
                app.Selection.TypeText(text);
            }
            return new Dictionary<string, object> { { "inserted", text.Length } };
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
                    step("borders", delegate { table.Borders.Enable = borders ? 1 : 0; });
                }
                int? borderColor = ParseBgrColor(ParamString(p, "borderColor", null));
                if (borderColor.HasValue)
                {
                    step("borderColor", delegate
                    {
                        table.Borders.Enable = 1;
                        table.Borders.OutsideColor = borderColor.Value;
                        table.Borders.InsideColor = borderColor.Value;
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
        private static dynamic TableAnchor(dynamic app)
        {
            dynamic range = app.Selection.Range;
            range.Collapse(0);
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
