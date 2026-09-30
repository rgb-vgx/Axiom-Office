using System.Text.RegularExpressions;
using AxiomOffice.Setup;

namespace AxiomOffice.Core.Setup;

// Dich loi cua model (ModelClient.ChatAsync tra ve chuoi ky thuat tieng Anh) thanh cau tieng Viet de hieu
// cho nguoi dung khong chuyen, kem goi y sua. Wizard thiet lap hien cap (message, hint); `detail` giu
// nguyen van de gui ho tro.
public static partial class LlmErrors
{
    public sealed record LlmError(string Kind, string Message, string Hint, string Detail);

    // Kind: config | auth | forbidden | notFound | model | rate | server | dns | network | timeout | reply | cancelled | internal
    public static LlmError Describe(string? error, string providerId = "openai", string endpoint = "", string model = "")
    {
        string detail = (error ?? "").Trim();
        string keyUrl = SetupCatalog.FindProvider(providerId)?.KeyUrl ?? "";
        string where = keyUrl.Length > 0 ? $" Lấy khoá mới tại {keyUrl}." : " Hỏi quản trị viên để lấy khoá mới.";

        if (detail.Length == 0)
        {
            return new LlmError("internal", "Không rõ lỗi.", "Thử lại; nếu vẫn lỗi, mở tệp log (core.log) để xem chi tiết.", detail);
        }

        if (detail.Contains("is not configured", StringComparison.OrdinalIgnoreCase))
        {
            string missing = detail.Contains("Endpoint", StringComparison.OrdinalIgnoreCase) ? "địa chỉ máy chủ AI" : "tên model";
            return new LlmError("config", $"Chưa điền {missing}.",
                "Điền đủ Địa chỉ máy chủ và Model rồi bấm Kiểm tra kết nối.", detail);
        }

        Match http = HttpStatus().Match(detail);
        if (http.Success)
        {
            int status = int.Parse(http.Groups[1].Value);
            string body = http.Groups[2].Value;
            return status switch
            {
                400 or 422 when MentionsModel(body) => ModelMissing(body, detail),
                // 404 cua nhieu may chu la "model khong ton tai" -> uu tien kiem tra truoc khi ket luan sai dia chi.
                404 or 405 when MentionsModel(body) => ModelMissing(body, detail),
                401 => new LlmError("auth", "Máy chủ AI từ chối khoá truy cập (401).",
                    "Có thể khoá sai, hết hạn, hoặc dán thiếu ký tự." + where, detail),
                403 => new LlmError("forbidden", "Khoá hợp lệ nhưng không được phép dùng model này (403).",
                    "Nhờ quản trị viên cấp quyền cho khoá/tài khoản này, hoặc chọn model khác.", detail),
                404 or 405 => new LlmError("notFound", "Không tìm thấy máy chủ AI ở địa chỉ này (mã " + status + ").",
                    "Kiểm tra lại địa chỉ: thường phải kết thúc bằng /v1 (ví dụ http://may-chu-cong-ty/v1), "
                    + "và chọn đúng nhà cung cấp (OpenAI-compatible hay Anthropic).", detail),
                429 => new LlmError("rate", "Máy chủ AI đang giới hạn tốc độ (429).",
                    "Chờ vài giây rồi kiểm tra lại; nếu lặp lại nhiều lần, dùng model khác hoặc báo quản trị viên.", detail),
                >= 500 => new LlmError("server", $"Máy chủ AI đang lỗi (mã {status}).",
                    "Thử lại sau ít phút; nếu vẫn lỗi, báo quản trị viên máy chủ.", detail),
                _ when MentionsModel(body) => ModelMissing(body, detail),
                _ => new LlmError("server", $"Máy chủ AI trả lỗi (mã {status}).",
                    "Kiểm tra lại địa chỉ, model và khoá truy cập.", detail),
            };
        }

        if (detail.Contains("timed out", StringComparison.OrdinalIgnoreCase))
        {
            string seconds = TimeoutSeconds().Match(detail) is { Success: true } match ? match.Groups[1].Value : "";
            return new LlmError("timeout", $"Máy chủ AI không trả lời{(seconds.Length > 0 ? " trong " + seconds + " giây" : "")}.",
                "Thử lại; máy chủ nội bộ chậm thì tăng \"Hết giờ mỗi yêu cầu\" ở Tuỳ chọn nâng cao, hoặc chọn model nhẹ hơn.", detail);
        }

        if (Dns().IsMatch(detail))
        {
            return new LlmError("dns", "Không tìm thấy địa chỉ máy chủ AI.",
                "Kiểm tra lại địa chỉ (có thể gõ sai tên miền hoặc thiếu phần /v1).", detail);
        }

        if (Network().IsMatch(detail))
        {
            return new LlmError("network", "Không kết nối được tới máy chủ AI.",
                "Kiểm tra mạng/VPN và địa chỉ máy chủ. Máy chủ nội bộ chỉ truy cập được khi đang trong mạng công ty.", detail);
        }

        if (detail.Contains("empty reply", StringComparison.OrdinalIgnoreCase)
            || detail.Contains("invalid provider response", StringComparison.OrdinalIgnoreCase)
            || detail.Contains("unexpected provider response", StringComparison.OrdinalIgnoreCase))
        {
            return new LlmError("reply", "Máy chủ trả lời nhưng không giống một máy chủ chat.",
                "Kiểm tra địa chỉ có phải loại tương thích OpenAI (/v1) không, và đã chọn đúng nhà cung cấp chưa.", detail);
        }

        if (detail.Contains("cancelled", StringComparison.OrdinalIgnoreCase))
        {
            return new LlmError("cancelled", "Đã dừng kiểm tra.", "Bấm Kiểm tra kết nối để thử lại.", detail);
        }

        return new LlmError("internal", "Không kiểm tra được kết nối.", "Xem chi tiết kỹ thuật bên dưới hoặc tệp core.log.", detail);
    }

    private static LlmError ModelMissing(string body, string detail)
    {
        return new LlmError("model", "Máy chủ không có model bạn chọn.",
            "Bấm \"Tải danh sách model\" rồi chọn lại từ danh sách (tên model phải viết đúng từng ký tự).",
            body.Length > 0 ? detail : "model");
    }

    private static bool MentionsModel(string body)
    {
        return body.Contains("model", StringComparison.OrdinalIgnoreCase);
    }

    [GeneratedRegex(@"^HTTP (\d{3}):\s*(.*)$", RegexOptions.Singleline)]
    private static partial Regex HttpStatus();

    [GeneratedRegex(@"(?:after|sau)\s+(\d+)\s*s")]
    private static partial Regex TimeoutSeconds();

    [GeneratedRegex(@"No such host|Name or service not known|nodename nor servname", RegexOptions.IgnoreCase)]
    private static partial Regex Dns();

    [GeneratedRegex(@"HttpRequestException|SocketException|Connection refused|actively refused|Network is unreachable|SSL|certificate",
        RegexOptions.IgnoreCase)]
    private static partial Regex Network();
}
