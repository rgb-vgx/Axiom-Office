using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;

namespace AxiomOffice.Bridge
{
    // Đọc nhanh trạng thái tài liệu của host (Word/WPS Writer, Excel/ET, PowerPoint/WPP).
    // Mọi hàm có thể ném COMException (app bận, không có tài liệu) — caller tự bọc try/catch.
    internal static class HostProbe
    {
        // Tài liệu đang active: {"name","fullName"}; null nếu host không có tài liệu nào.
        public static Dictionary<string, object> ActiveDocument(IAppHost host)
        {
            dynamic app = host.Application;
            if (app == null)
            {
                return null;
            }
            dynamic doc;
            try
            {
                if (host.AppKind == "et")
                {
                    doc = app.ActiveWorkbook;
                }
                else if (host.AppKind == "wpp")
                {
                    // PowerPoint 16 có lúc không đổi ActivePresentation sau khi mở bài đã mở; dùng bài bridge
                    // vừa mở/tạo (CommandDispatcher.TrackPresentation) để ai.ask gửi đúng tài liệu.
                    doc = CommandDispatcher.CurrentPresentation(app);
                }
                else
                {
                    doc = app.ActiveDocument;
                }
            }
            catch (COMException ex)
            {
                if (IsRetryable(ex))
                {
                    throw;
                }
                // Word/PowerPoint ném lỗi khi không có tài liệu nào mở.
                return null;
            }
            if (doc == null)
            {
                return null;
            }
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(doc.Name) },
                { "fullName", Convert.ToString(doc.FullName) }
            };
        }

        // Chỗ người dùng đang nhìn, gửi kèm mỗi lượt cho Agent Core ("slide này", "đoạn đang chọn", "vùng này").
        // Lượt 13/10/2026: người dùng nói "cập nhật slide hiện tại", agent không biết slide nào nên dựng 6 slide
        // mới ở cuối bài. Từng phần đọc riêng: host thiếu thuộc tính (WPS) thì bỏ phần đó, không hỏng cả lượt.
        public static Dictionary<string, object> View(IAppHost host)
        {
            var view = new Dictionary<string, object>();
            dynamic app = host.Application;
            if (app == null)
            {
                return view;
            }
            if (host.AppKind == "wpp")
            {
                TryPut(view, "slideCount", () => Convert.ToInt32(CommandDispatcher.CurrentPresentation(app).Slides.Count));
                if (!TryPut(view, "slide", () => Convert.ToInt32(app.ActiveWindow.View.Slide.SlideIndex)))
                {
                    TryPut(view, "slide", () => Convert.ToInt32(app.ActiveWindow.Selection.SlideRange.SlideIndex));
                }
            }
            else if (host.AppKind == "et")
            {
                TryPut(view, "sheet", () => Convert.ToString(app.ActiveSheet.Name));
                TryPut(view, "range", () => Convert.ToString(app.Selection.Address(false, false)));
            }
            else
            {
                TryPut(view, "headingPath", () => HeadingPath(app));
                TryPut(view, "selection", () =>
                {
                    string text = Convert.ToString(app.Selection.Text) ?? "";
                    // Con trỏ đứng yên (không bôi đen) Word trả 1 ký tự: không phải vùng chọn.
                    return text.Trim().Length > 1 ? (text.Length > 2000 ? text.Substring(0, 2000) : text) : null;
                });
            }
            return view;
        }

        // P0-01 (16/10/2026): duong heading quanh con tro Word - toi da 6 muc.
        private static object HeadingPath(dynamic app)
        {
            try
            {
                dynamic doc = app.ActiveDocument;
                dynamic sel = app.Selection;
                if (doc == null || sel == null) return null;
                int cursor = System.Convert.ToInt32(sel.Range.Start);
                var names = new System.Collections.Generic.List<string>();
                foreach (dynamic paragraph in doc.Paragraphs)
                {
                    int start;
                    try { start = System.Convert.ToInt32(paragraph.Range.Start); }
                    catch { continue; }
                    if (start > cursor) break;
                    string style;
                    try { style = System.Convert.ToString(paragraph.Style.NameLocal); }
                    catch { continue; }
                    if (style == null || style.IndexOf("Heading") < 0) continue;
                    string text;
                    try { text = (System.Convert.ToString(paragraph.Range.Text) ?? "").Trim(); }
                    catch { continue; }
                    text = text.TrimEnd('\r', '\a').Trim();
                    if (text.Length == 0) continue;
                    if (text.Length > 120) text = text.Substring(0, 120);
                    string item = style + ": " + text;
                    if (names.Count == 0 || names[names.Count - 1] != item) names.Add(item);
                    if (names.Count > 12) names.RemoveAt(0);
                }
                if (names.Count == 0) return null;
                int from = System.Math.Max(0, names.Count - 6);
                return names.GetRange(from, names.Count - from).ToArray();
            }
            catch { return null; }
        }

        private static bool TryPut(Dictionary<string, object> map, string key, Func<object> read)
        {
            try
            {
                object value = read();
                if (value == null)
                {
                    return false;
                }
                map[key] = value;
                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        // Mô tả một dòng cho log chẩn đoán: số tài liệu, tài liệu active, các cửa sổ và trạng thái hiển thị.
        public static string Describe(IAppHost host)
        {
            var sb = new StringBuilder();
            dynamic app = host.Application;
            if (app == null)
            {
                return "no application";
            }
            string collection = host.AppKind == "et" ? "Workbooks" : host.AppKind == "wpp" ? "Presentations" : "Documents";
            sb.Append(collection).Append("=");
            sb.Append(Safe(delegate
            {
                if (host.AppKind == "et")
                {
                    return Convert.ToString(app.Workbooks.Count);
                }
                if (host.AppKind == "wpp")
                {
                    return Convert.ToString(app.Presentations.Count);
                }
                return Convert.ToString(app.Documents.Count);
            }));
            sb.Append(" active=").Append(Safe(delegate
            {
                Dictionary<string, object> doc = ActiveDocument(host);
                return doc == null ? "(none)" : "'" + doc["name"] + "' (" + doc["fullName"] + ")";
            }));
            sb.Append(" activeWindow=").Append(Safe(delegate
            {
                return WindowText(app.ActiveWindow);
            }));
            sb.Append(" windows=[").Append(Safe(delegate
            {
                dynamic windows = app.Windows;
                int count = Convert.ToInt32(windows.Count);
                var parts = new List<string>();
                for (int i = 1; i <= count && i <= 10; i++)
                {
                    parts.Add(WindowText(windows[i]));
                }
                return string.Join(", ", parts.ToArray());
            })).Append("]");
            sb.Append(" appVisible=").Append(Safe(delegate { return Convert.ToString(app.Visible); }));
            return sb.ToString();
        }

        // DocumentWindow của PowerPoint không có Visible (Word/Excel có): đọc riêng để không mất cả Caption,
        // thiếu thì ghi WindowState (1 = thường, 2 = thu nhỏ, 3 = phóng to).
        private static string WindowText(dynamic window)
        {
            string caption = Safe(delegate { return Convert.ToString(window.Caption); });
            string visible = Safe(delegate { return "visible=" + Convert.ToString(window.Visible); });
            if (visible.StartsWith("<", StringComparison.Ordinal))
            {
                visible = "state=" + Safe(delegate { return Convert.ToString(window.WindowState); });
            }
            return "'" + caption + "' " + visible;
        }

        public static bool IsRetryable(COMException ex)
        {
            int hr = ex.HResult;
            return hr == unchecked((int)0x80010001)
                || hr == unchecked((int)0x8001010A)
                || hr == unchecked((int)0x80010002)
                || hr == unchecked((int)0x800AC472);
        }

        private static string Safe(Func<string> read)
        {
            try
            {
                return read();
            }
            catch (Exception ex)
            {
                return "<" + ex.GetType().Name + ">";
            }
        }
    }
}
