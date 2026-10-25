using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;

namespace AxiomOffice.Bridge
{
    // Hộp thoại modal đang mở trong process host (add-in chạy in-process nên là chính process này): tiêu đề + chữ.
    // Chỉ ĐỌC để báo người dùng, không bao giờ tự bấm - không phân biệt được hộp thoại lỗi với câu hỏi "lưu không?"
    // cần người dùng trả lời (docs/plans/19-h0 H2).
    internal static class HostDialogs
    {
        private delegate bool EnumProc(IntPtr hwnd, IntPtr lParam);

        [DllImport("user32.dll")]
        private static extern bool EnumWindows(EnumProc callback, IntPtr lParam);

        [DllImport("user32.dll")]
        private static extern bool EnumChildWindows(IntPtr parent, EnumProc callback, IntPtr lParam);

        [DllImport("user32.dll")]
        private static extern int GetWindowThreadProcessId(IntPtr hwnd, out int processId);

        [DllImport("user32.dll")]
        private static extern bool IsWindowVisible(IntPtr hwnd);

        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        private static extern int GetWindowText(IntPtr hwnd, StringBuilder text, int max);

        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        private static extern int GetClassName(IntPtr hwnd, StringBuilder name, int max);

        // "\"Microsoft PowerPoint\" - \"An error occurred while PowerPoint was saving the file.\"", hoặc null.
        public static string Describe()
        {
            try
            {
                int self = Process.GetCurrentProcess().Id;
                var found = new List<string>();
                EnumWindows(delegate (IntPtr hwnd, IntPtr lParam)
                {
                    int pid;
                    GetWindowThreadProcessId(hwnd, out pid);
                    if (pid == self && IsWindowVisible(hwnd) && ClassOf(hwnd) == "#32770")
                    {
                        var parts = new List<string> { Quote(TextOf(hwnd)) };
                        EnumChildWindows(hwnd, delegate (IntPtr child, IntPtr l)
                        {
                            if (ClassOf(child) == "Static")
                            {
                                string text = TextOf(child).Trim();
                                if (text.Length > 0)
                                {
                                    parts.Add(Quote(text));
                                }
                            }
                            return true;
                        }, IntPtr.Zero);
                        found.Add(string.Join(" - ", parts));
                    }
                    return true;
                }, IntPtr.Zero);
                return found.Count == 0 ? null : string.Join("; ", found);
            }
            catch (Exception ex)
            {
                Logger.Error("HostDialogs.Describe failed", ex);
                return null;
            }
        }

        private static string TextOf(IntPtr hwnd)
        {
            var text = new StringBuilder(1024);
            GetWindowText(hwnd, text, text.Capacity);
            return text.ToString();
        }

        private static string ClassOf(IntPtr hwnd)
        {
            var name = new StringBuilder(256);
            GetClassName(hwnd, name, name.Capacity);
            return name.ToString();
        }

        private static string Quote(string text)
        {
            return "\"" + text.Replace("\"", "'") + "\"";
        }
    }
}
