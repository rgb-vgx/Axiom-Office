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
                        table.set_Style(ParamString(p, "style", "Table Grid"));
                    }
                    catch
                    {
                    }
                }
            }
            return new Dictionary<string, object>
            {
                { "rows", rows },
                { "cols", cols },
                { "filled", filled }
            };
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
            }
            catch (Exception)
            {
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
