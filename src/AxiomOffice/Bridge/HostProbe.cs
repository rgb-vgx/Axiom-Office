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
                    doc = app.ActivePresentation;
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
