using System;
using Microsoft.Win32;

namespace AxiomOffice.Bridge
{
    internal static class Config
    {
        private const string KeyPath = @"Software\AxiomOffice";
        public const int DefaultPort = 47821;
        public const int DefaultPortOffice = 47831;

        public static int Port
        {
            get { return ReadInt("Port", DefaultPort); }
        }

        public static int PortOffice
        {
            get { return ReadInt("PortOffice", DefaultPortOffice); }
        }

        public static int PortForKind(string appKind)
        {
            return PortForKind(appKind, false);
        }

        public static int PortForKind(string appKind, bool officeHost)
        {
            int port = officeHost ? PortOffice : Port;
            if (appKind == "et")
            {
                return port + 1;
            }
            if (appKind == "wpp")
            {
                return port + 2;
            }
            return port;
        }

        public static string Token
        {
            get { return ReadString("Token", ""); }
        }

        public static bool Enabled
        {
            get { return ReadInt("Enabled", 1) != 0; }
        }

        // 0 = khong khoi dong Agent Core, nen pane va ai.ask khong chay duoc AI (docs/architecture/core.md muc 7.6).
        public static bool CoreEnabled
        {
            get { return ReadInt("CoreEnabled", 1) != 0; }
        }

        public static string LlmProvider
        {
            get { return ReadString("LlmProvider", "openai"); }
        }

        public static string LlmEndpoint
        {
            get { return ReadString("LlmEndpoint", ""); }
        }

        public static string LlmApiKey
        {
            get { return ReadSecret("LlmApiKey"); }
        }

        // Khoá tìm kiếm web (Tavily) của Agent Core - tuỳ chọn; lưu "dpapi:" như LlmApiKey.
        public static string TavilyApiKey
        {
            get { return ReadSecret("TavilyApiKey"); }
        }

        // Cho AI tìm kiếm web (toggle trên ribbon; Core đọc lại mỗi lượt). Tắt thì không đăng ký
        // tool dù đã có khoá — khác xoá khoá (xoá là mất, bật lại phải nhập lại).
        public static bool TavilyEnabled
        {
            get { return ReadInt("TavilyEnabled", 1) != 0; }
        }

        // Lưu nguồn web tải về (trang đã đọc) vào kho Core (core.db): trích dẫn lại, khỏi tải lại trang.
        // Bật mặc định; tắt = không lưu (các lượt vẫn tìm web bình thường).
        public static bool WebSourceStore
        {
            get { return ReadInt("WebSourceStore", 1) != 0; }
        }

        // Khoá lưu "dpapi:<base64>" (WriteSecret) hoặc chữ thường (cấu hình cũ / nhập tay).
        private static string ReadSecret(string name)
        {
            string raw = ReadString(name, "");
            if (raw.StartsWith("dpapi:", StringComparison.Ordinal))
            {
                try
                {
                    byte[] encrypted = Convert.FromBase64String(raw.Substring(6));
                    byte[] plain = System.Security.Cryptography.ProtectedData.Unprotect(
                        encrypted, null, System.Security.Cryptography.DataProtectionScope.CurrentUser);
                    return System.Text.Encoding.UTF8.GetString(plain);
                }
                catch
                {
                    return "";
                }
            }
            return raw;
        }

        // Ngôn ngữ giao diện add-in: "" = tự động theo Windows, "vi", "en"... (Ui.cs).
        public static string UiLanguage
        {
            get { return ReadString("UiLanguage", ""); }
        }

        public static string LlmModel
        {
            get { return ReadString("LlmModel", ""); }
        }

        // Model dự phòng (phân cách bằng dấu phẩy) và cửa sổ ngữ cảnh (token, 0 = theo nhà cung cấp) - Core đọc.
        public static string LlmFallbackModels
        {
            get { return ReadString("LlmFallbackModels", ""); }
        }

        public static int LlmContextWindow
        {
            get { return Math.Max(0, ReadInt("LlmContextWindow", 0)); }
        }

        public static bool WriteString(string name, string value)
        {
            try
            {
                using (RegistryKey key = Registry.CurrentUser.CreateSubKey(KeyPath))
                {
                    if (key != null)
                    {
                        key.SetValue(name, value ?? "", RegistryValueKind.String);
                        return true;
                    }
                }
            }
            catch
            {
            }
            return false;
        }

        // Ghi nhớ dài hạn của Agent Core (docs/architecture/core.md mục 8.5.10); Core đọc lại mỗi lượt.
        public static bool MemoryEnabled
        {
            get { return ReadInt("MemoryEnabled", 1) != 0; }
        }

        public static bool MemoryAutoExtract
        {
            get { return ReadInt("MemoryAutoExtract", 1) != 0; }
        }

        // QA thị giác: Agent Core cho model xem ảnh chụp cửa sổ (tắt mặc định, tốn token).
        public static bool VisualQaEnabled
        {
            get { return ReadInt("VisualQaEnabled", 0) != 0; }
        }

        // Tự kiểm chứng: sau khi sửa tài liệu, agent đọc lại rồi mới trả lời (bật mặc định).
        public static bool VerifyWorkEnabled
        {
            get { return ReadInt("VerifyWorkEnabled", 1) != 0; }
        }

        // Hiện suy luận của model trong khung chat (tắt mặc định: tốn token và chậm hơn).
        public static bool LlmShowReasoning
        {
            get { return ReadInt("LlmShowReasoning", 0) != 0; }
        }

        public static bool WriteDword(string name, int value)
        {
            try
            {
                using (RegistryKey key = Registry.CurrentUser.CreateSubKey(KeyPath))
                {
                    if (key != null)
                    {
                        key.SetValue(name, value, RegistryValueKind.DWord);
                        return true;
                    }
                }
            }
            catch
            {
            }
            return false;
        }

        public static bool WriteSecret(string name, string value)
        {
            try
            {
                byte[] plain = System.Text.Encoding.UTF8.GetBytes(value ?? "");
                byte[] encrypted = System.Security.Cryptography.ProtectedData.Protect(
                    plain, null, System.Security.Cryptography.DataProtectionScope.CurrentUser);
                return WriteString(name, "dpapi:" + Convert.ToBase64String(encrypted));
            }
            catch
            {
                return WriteString(name, value);
            }
        }

        private static int ReadInt(string name, int fallback)
        {
            try
            {
                using (RegistryKey key = Registry.CurrentUser.OpenSubKey(KeyPath))
                {
                    if (key != null)
                    {
                        object value = key.GetValue(name);
                        if (value is int)
                        {
                            return (int)value;
                        }
                        int parsed;
                        if (value != null && int.TryParse(Convert.ToString(value), out parsed))
                        {
                            return parsed;
                        }
                    }
                }
            }
            catch
            {
            }
            return fallback;
        }

        private static string ReadString(string name, string fallback)
        {
            try
            {
                using (RegistryKey key = Registry.CurrentUser.OpenSubKey(KeyPath))
                {
                    if (key != null)
                    {
                        object value = key.GetValue(name);
                        if (value != null)
                        {
                            return Convert.ToString(value);
                        }
                    }
                }
            }
            catch
            {
            }
            return fallback;
        }
    }
}
