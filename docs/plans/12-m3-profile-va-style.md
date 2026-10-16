# 12 / M3 - Profile và chuẩn chung

> Soạn văn bản mới đúng font, heading, cách xưng hô đã duyệt.

## 1. Đã có (đối chiếu code 16/10/2026)

- Scope `user` (sở thích cá nhân) và `skill`; `category` phân loại fact khi trích xuất.
- Token thiết kế dùng chung: `skills/_design/tokens.json` (màu, cỡ chữ).
- `doc_design` hỏi người dùng kiểu trình bày mỗi lượt (có ô tự gõ) - chưa nhớ lựa chọn.

## 2. Còn thiếu - việc cần làm

- **Scope `org-style`**: chuẩn chung của đơn vị (mẫu công văn, mẫu báo cáo, mẫu slide), chỉ Admin sửa. Cần cơ
  chế phân quyền tối thiểu (ví dụ giá trị HKCU do quản trị đặt, hoặc tệp trong thư mục chung chỉ đọc).
- **Nhớ kiểu trình bày đã chọn**: lựa chọn ở thẻ `doc_design` (kể cả kiểu tự gõ) lưu thành memory `user` hoặc
  `document`; lần sau đưa làm lựa chọn đầu, vẫn hỏi.
- **Áp đúng lúc**: chuẩn chung đọc trước khi model chọn skill trình bày (`van-ban-hanh-chinh`,
  `trinh-bay-chuyen-nghiep`, `thiet-ke-van-phong`).
- **UI**: `MemoryForm.cs` chia nhóm Tài liệu | Cá nhân | Chuẩn chung.

## 3. Tệp sẽ sửa

- `core-go/internal/memory/*`, `core-go/internal/agent/prompt.go`
- `core-go/internal/tools/doc_design.go` (lưu lựa chọn kiểu)
- `src/AxiomOffice/Ai/MemoryForm.cs`

## 4. Tiêu chí xong

- Đặt chuẩn chung "font Times New Roman 13, lề trái 3 cm" → kịch bản soạn công văn trong bộ `17` đạt tiêu chí
  font/lề 5/5 lần mà không nhắc lại trong câu lệnh.
- Người dùng thường không sửa được `org-style` (test API).

## 5. Không làm

- Không suy đoán chân dung người dùng, không đồng bộ cloud.
