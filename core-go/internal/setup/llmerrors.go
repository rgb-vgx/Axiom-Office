package setup

import (
	"regexp"
	"strconv"
	"strings"
)

// LlmError: cau tieng Viet cho nguoi dung khong chuyen + goi y sua; Detail giu nguyen van de gui ho tro.
// Kind: config | auth | forbidden | notFound | model | rate | server | dns | network | timeout | reply | cancelled | internal
type LlmError struct {
	Kind    string
	Message string
	Hint    string
	Detail  string
}

var (
	httpStatus     = regexp.MustCompile(`(?s)^HTTP (\d{3}):\s*(.*)$`)
	timeoutSeconds = regexp.MustCompile(`(?:after|sau)\s+(\d+)\s*s`)
	dnsError       = regexp.MustCompile(`(?i)No such host|Name or service not known|nodename nor servname`)
	// Them cac dang loi cua net/http trong Go (dial tcp, connection reset, x509) ben canh chuoi cua ban .NET.
	networkError = regexp.MustCompile(`(?i)HttpRequestException|SocketException|Connection refused|actively refused|Network is unreachable|SSL|certificate|dial tcp|connection reset|x509`)
)

// Describe dich loi cua ModelClient/ModelCatalog (chuoi ky thuat tieng Anh) - giong het LlmErrors.Describe.
func Describe(errorText, providerID, endpoint, model string) LlmError {
	detail := strings.TrimSpace(errorText)
	keyURL := ""
	if provider := FindProvider(providerID); provider != nil {
		keyURL = provider.KeyURL
	}
	where := " Hỏi quản trị viên để lấy khoá mới."
	if keyURL != "" {
		where = " Lấy khoá mới tại " + keyURL + "."
	}
	lower := strings.ToLower(detail)

	if detail == "" {
		return LlmError{"internal", "Không rõ lỗi.", "Thử lại; nếu vẫn lỗi, mở tệp log (core.log) để xem chi tiết.", detail}
	}

	if strings.Contains(lower, "is not configured") {
		missing := "tên model"
		if strings.Contains(lower, "endpoint") {
			missing = "địa chỉ máy chủ AI"
		}
		return LlmError{"config", "Chưa điền " + missing + ".",
			"Điền đủ Địa chỉ máy chủ và Model rồi bấm Kiểm tra kết nối.", detail}
	}

	if match := httpStatus.FindStringSubmatch(detail); match != nil {
		status, _ := strconv.Atoi(match[1])
		body := match[2]
		mentionsModel := strings.Contains(strings.ToLower(body), "model")
		switch {
		case (status == 400 || status == 422) && mentionsModel:
			return modelMissing(body, detail)
		// 404 cua nhieu may chu la "model khong ton tai" -> uu tien kiem tra truoc khi ket luan sai dia chi.
		case (status == 404 || status == 405) && mentionsModel:
			return modelMissing(body, detail)
		case status == 401:
			return LlmError{"auth", "Máy chủ AI từ chối khoá truy cập (401).",
				"Có thể khoá sai, hết hạn, hoặc dán thiếu ký tự." + where, detail}
		case status == 403:
			return LlmError{"forbidden", "Khoá hợp lệ nhưng không được phép dùng model này (403).",
				"Nhờ quản trị viên cấp quyền cho khoá/tài khoản này, hoặc chọn model khác.", detail}
		case status == 404 || status == 405:
			return LlmError{"notFound", "Không tìm thấy máy chủ AI ở địa chỉ này (mã " + match[1] + ").",
				"Kiểm tra lại địa chỉ: thường phải kết thúc bằng /v1 (ví dụ http://may-chu-cong-ty/v1), " +
					"và chọn đúng nhà cung cấp (OpenAI-compatible hay Anthropic).", detail}
		case status == 429:
			return LlmError{"rate", "Máy chủ AI đang giới hạn tốc độ (429).",
				"Chờ vài giây rồi kiểm tra lại; nếu lặp lại nhiều lần, dùng model khác hoặc báo quản trị viên.", detail}
		case status >= 500:
			return LlmError{"server", "Máy chủ AI đang lỗi (mã " + match[1] + ").",
				"Thử lại sau ít phút; nếu vẫn lỗi, báo quản trị viên máy chủ.", detail}
		case mentionsModel:
			return modelMissing(body, detail)
		default:
			return LlmError{"server", "Máy chủ AI trả lỗi (mã " + match[1] + ").",
				"Kiểm tra lại địa chỉ, model và khoá truy cập.", detail}
		}
	}

	if strings.Contains(lower, "timed out") {
		seconds := ""
		if match := timeoutSeconds.FindStringSubmatch(detail); match != nil {
			seconds = " trong " + match[1] + " giây"
		}
		return LlmError{"timeout", "Máy chủ AI không trả lời" + seconds + ".",
			"Thử lại; máy chủ nội bộ chậm thì tăng \"Hết giờ mỗi yêu cầu\" ở Tuỳ chọn nâng cao, hoặc chọn model nhẹ hơn.", detail}
	}

	if dnsError.MatchString(detail) {
		return LlmError{"dns", "Không tìm thấy địa chỉ máy chủ AI.",
			"Kiểm tra lại địa chỉ (có thể gõ sai tên miền hoặc thiếu phần /v1).", detail}
	}

	if networkError.MatchString(detail) {
		return LlmError{"network", "Không kết nối được tới máy chủ AI.",
			"Kiểm tra mạng/VPN và địa chỉ máy chủ. Máy chủ nội bộ chỉ truy cập được khi đang trong mạng công ty.", detail}
	}

	if strings.Contains(lower, "empty reply") || strings.Contains(lower, "invalid provider response") ||
		strings.Contains(lower, "unexpected provider response") {
		return LlmError{"reply", "Máy chủ trả lời nhưng không giống một máy chủ chat.",
			"Kiểm tra địa chỉ có phải loại tương thích OpenAI (/v1) không, và đã chọn đúng nhà cung cấp chưa.", detail}
	}

	if strings.Contains(lower, "cancelled") {
		return LlmError{"cancelled", "Đã dừng kiểm tra.", "Bấm Kiểm tra kết nối để thử lại.", detail}
	}

	return LlmError{"internal", "Không kiểm tra được kết nối.", "Xem chi tiết kỹ thuật bên dưới hoặc tệp core.log.", detail}
}

func modelMissing(body, detail string) LlmError {
	if body == "" {
		detail = "model"
	}
	return LlmError{"model", "Máy chủ không có model bạn chọn.",
		"Bấm \"Tải danh sách model\" rồi chọn lại từ danh sách (tên model phải viết đúng từng ký tự).", detail}
}
