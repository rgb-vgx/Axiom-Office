package memory

import "testing"

// Redact: thay cho nhay cam bang [ẩn], khong cat oan so binh thuong (so trang, ngay, so tien, lop).
func TestRedactSensitive(t *testing.T) {
	cases := []string{
		"API key của tôi là sk-abcdefghijklmnopqrstuv",
		"Liên hệ qua a.b.c@example.com.vn nhé",
		"Gửi kèm Bearer abcdefghijklmnop1234 nhé",
		"token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.abcd1234",
		"Số CCCD của tôi là 012345678901",
		"Thẻ 4111 1111 1111 1111",
		"mật khẩu: abc123",
		"Số điện thoại 0912 345 678",
	}
	for _, text := range cases {
		clean, hit := Redact(text)
		if !hit || clean == text {
			t.Errorf("Redact(%q) = %q, %v - phai bi thay", text, clean, hit)
		}
	}
}

func TestRedactKeepsNormalText(t *testing.T) {
	cases := []string{
		"Times New Roman 13",
		"Ngày 16/10/2026 nộp báo cáo",
		"Số tiền 1.234.567 đồng",
		"Giáo án tuần 3",
		"Lớp 4A có 35 học sinh",
		"Nghị định 30/2020/NĐ-CP",
		"Người dùng là Trưởng phòng Kế toán",
	}
	for _, text := range cases {
		clean, hit := Redact(text)
		if hit || clean != text {
			t.Errorf("Redact(%q) = %q, %v - phai giu nguyen", text, clean, hit)
		}
	}
	if clean, hit := Redact(""); clean != "" || hit {
		t.Errorf("Redact rong = %q, %v", clean, hit)
	}
}
