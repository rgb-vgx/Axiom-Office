// SINH TU DONG tu catalog/setup.json bang scripts/generate_setup_catalog.py - KHONG sua tay.
// Dung chung cho add-in Windows (net48) va Agent Core (net10): chu cua wizard + preset nha cung cap.
using System;
using System.Collections.Generic;

namespace AxiomOffice.Setup
{
    internal sealed class SetupProviderInfo
    {
        public readonly string Id;
        public readonly string Group;
        public readonly string Label;
        public readonly string Description;
        public readonly string Endpoint;
        public readonly bool NeedsKey;
        public readonly string KeyUrl;
        public readonly string Codec;
        public readonly string[] SuggestedModels;

        public SetupProviderInfo(string id, string group, string label, string description, string endpoint,
            bool needsKey, string keyUrl, string codec, string[] suggestedModels)
        {
            Id = id;
            Group = group;
            Label = label;
            Description = description;
            Endpoint = endpoint;
            NeedsKey = needsKey;
            KeyUrl = keyUrl;
            Codec = codec;
            SuggestedModels = suggestedModels;
        }
    }

    internal sealed class SetupFeatureInfo
    {
        public readonly string Key;
        public readonly string Label;
        public readonly string Description;
        public readonly bool Recommended;

        public SetupFeatureInfo(string key, string label, string description, bool recommended)
        {
            Key = key;
            Label = label;
            Description = description;
            Recommended = recommended;
        }
    }

    internal sealed class SetupCheckInfo
    {
        public readonly string Id;
        public readonly string Label;
        public readonly bool Fixable;
        public readonly string FixLabel;
        public readonly string Help;

        public SetupCheckInfo(string id, string label, bool fixable, string fixLabel, string help)
        {
            Id = id;
            Label = label;
            Fixable = fixable;
            FixLabel = fixLabel;
            Help = help;
        }
    }

    internal sealed class SetupStepInfo
    {
        public readonly string Id;
        public readonly string Title;
        public readonly string Subtitle;

        public SetupStepInfo(string id, string title, string subtitle)
        {
            Id = id;
            Title = title;
            Subtitle = subtitle;
        }
    }

    internal static class SetupCatalog
    {
        public const int Version = 1;

        public static readonly IReadOnlyList<SetupStepInfo> Steps = new[]
        {
            new SetupStepInfo("welcome", "Chào mừng bạn đến với Axiom Office", "Chỉ mất khoảng một phút. Trợ lý AI cần kết nối tới một máy chủ AI để đọc và sửa tài liệu đang mở."),
            new SetupStepInfo("checks", "Kiểm tra máy", "Axiom Office tự kiểm tra những thứ cần thiết và sửa được thì sửa luôn."),
            new SetupStepInfo("connect", "Kết nối máy chủ AI", "Chọn nơi cung cấp AI. Không chắc thì hỏi quản trị viên hoặc dùng tài khoản cá nhân."),
            new SetupStepInfo("features", "Tính năng", "Bật/tắt những gì Axiom Office làm thêm cho bạn. Có thể đổi lại sau."),
            new SetupStepInfo("done", "Hoàn tất", "Kiểm tra lại lần cuối rồi bắt đầu làm việc."),
        };

        public static readonly IReadOnlyList<SetupProviderInfo> Providers = new[]
        {
            new SetupProviderInfo("company", "internal", "Máy chủ của công ty", "Máy chủ AI nội bộ (hoặc proxy) tương thích OpenAI. Hỏi quản trị viên địa chỉ và khoá truy cập.", "", false, "", "openai", new string[] {  }),
            new SetupProviderInfo("openai", "public", "OpenAI", "Dùng tài khoản OpenAI của bạn (trả phí theo lượng dùng).", "https://api.openai.com/v1", true, "https://platform.openai.com/api-keys", "openai", new string[] { "gpt-4o-mini", "gpt-4o", "gpt-4.1-mini" }),
            new SetupProviderInfo("anthropic", "public", "Anthropic (Claude)", "Dùng tài khoản Anthropic của bạn.", "https://api.anthropic.com/v1", true, "https://console.anthropic.com/settings/keys", "anthropic", new string[] {  }),
            new SetupProviderInfo("gemini", "public", "Google Gemini", "Dùng khoá lấy từ Google AI Studio.", "https://generativelanguage.googleapis.com/v1beta/openai", true, "https://aistudio.google.com/apikey", "openai", new string[] { "gemini-2.5-flash" }),
        };

        public static readonly IReadOnlyList<SetupFeatureInfo> Features = new[]
        {
            new SetupFeatureInfo("MemoryEnabled", "Nhớ những điều tôi đã dặn", "Axiom Office ghi nhớ quy ước của bạn (ví dụ: luôn dùng Times New Roman 13) để lần sau khỏi nhắc lại.", true),
            new SetupFeatureInfo("MemoryAutoExtract", "Tự rút ra điều đáng nhớ sau mỗi lượt", "Tiện hơn, nhưng tốn thêm một lượt gọi model sau mỗi yêu cầu.", true),
            new SetupFeatureInfo("VisualQaEnabled", "Cho AI xem ảnh trang tài liệu", "Giúp AI soát bố cục; tốn nhiều token và cần model đọc được ảnh.", false),
        };

        public static readonly IReadOnlyList<SetupCheckInfo> Checks = new[]
        {
            new SetupCheckInfo("app", "Axiom Office đã nạp trong ứng dụng", false, "", "Nếu dòng này đỏ, hãy cài lại gói (install.cmd trên Windows, install.sh trên Linux) rồi mở lại ứng dụng."),
            new SetupCheckInfo("core", "Agent Core đang chạy", true, "Khởi động Core", "Agent Core là bộ não chạy nền: hội thoại, kỹ năng, ghi nhớ. Pane tự khởi động khi cần."),
            new SetupCheckInfo("bridge", "Cầu nối trong ứng dụng trả lời được", true, "Kiểm tra lại", "Cầu nối là đường Axiom Office điều khiển tài liệu đang mở."),
            new SetupCheckInfo("config", "Đã có cấu hình AI", true, "Thiết lập ngay", "Cần địa chỉ máy chủ AI và tên model."),
            new SetupCheckInfo("token", "Khoá bảo vệ giữa ứng dụng và Agent Core", true, "Tạo khoá mới", "Khoá này chỉ đi trên máy của bạn, để ứng dụng khác không điều khiển được Agent Core."),
            new SetupCheckInfo("configFile", "Tệp cấu hình đọc/ghi được", true, "Sửa tệp cấu hình", "Trên Windows là HKCU\\Software\\AxiomOffice; trên Linux là ~/.config/axiom-office/config.json (quyền 0600)."),
            new SetupCheckInfo("mcp", "MCP cho Claude Code / Claude Desktop (tuỳ chọn)", false, "", "Chỉ cần nếu bạn muốn Claude điều khiển Word/LibreOffice qua MCP."),
        };

        public static SetupProviderInfo FindProvider(string id)
        {
            if (string.IsNullOrWhiteSpace(id))
            {
                return null;
            }

            string wanted = id.Trim();
            foreach (SetupProviderInfo provider in Providers)
            {
                if (string.Equals(provider.Id, wanted, StringComparison.OrdinalIgnoreCase))
                {
                    return provider;
                }
            }

            return null;
        }

        public static SetupCheckInfo FindCheck(string id)
        {
            foreach (SetupCheckInfo check in Checks)
            {
                if (string.Equals(check.Id, id, StringComparison.OrdinalIgnoreCase))
                {
                    return check;
                }
            }

            return null;
        }

        // Doan nha cung cap tu dia chi dang cau hinh (mo lai wizard thi chon dung lua chon cu).
        public static string GuessProviderId(string endpoint, string codec)
        {
            string value = (endpoint ?? string.Empty).Trim();
            if (value.Length == 0)
            {
                return "company";
            }

            if (value.IndexOf("api.openai.com", StringComparison.OrdinalIgnoreCase) >= 0)
            {
                return "openai";
            }

            if (value.IndexOf("anthropic.com", StringComparison.OrdinalIgnoreCase) >= 0)
            {
                return "anthropic";
            }

            if (value.IndexOf("generativelanguage.googleapis.com", StringComparison.OrdinalIgnoreCase) >= 0)
            {
                return "gemini";
            }

            return "company";
        }
    }
}
