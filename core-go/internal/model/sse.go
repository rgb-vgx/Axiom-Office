package model

import "strings"

// ssePayloads tách body SSE thành các payload của những dòng `data:`.
//
// Bỏ: dòng rỗng, dòng bắt đầu bằng ':' (comment, dùng làm keep-alive), và các field khác
// (`event:`, `id:`, `retry:`). `data: [DONE]` là dấu kết thúc hợp lệ nên cũng bị bỏ - đây đúng là
// chỗ trước kia làm hỏng cả response khi máy chủ nối nó vào sau một JSON hoàn chỉnh.
//
// Nhiều dòng `data:` liên tiếp trong cùng một event được nối bằng '\n' theo đúng đặc tả SSE.
// Body không phải SSE (máy chủ bỏ qua `stream: true`, trả JSON một cục) -> trả về rỗng, bên gọi
// rơi về đường Parse thường.
func ssePayloads(body []byte) []string {
	text := string(body)
	if !strings.Contains(text, "data:") {
		return nil
	}
	payloads := []string{}
	var current []string
	flush := func() {
		if len(current) > 0 {
			payloads = append(payloads, strings.Join(current, "\n"))
			current = nil
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		value, ok := dataField(line)
		if !ok {
			continue
		}
		if value == "[DONE]" {
			flush()
			continue
		}
		if value != "" {
			current = append(current, value)
		}
	}
	flush()
	return payloads
}

// dataField doc mot dong SSE: tra ve gia tri cua `data:` (chap nhan ca "data:" va "data: ").
func dataField(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "), true
}
