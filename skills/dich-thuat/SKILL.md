---
name: dich-thuat
description: >-
  Dịch văn bản công việc Việt–Anh (và tiếng nước ngoài sang tiếng Việt): dịch song ngữ đặt bản dịch ngay dưới đoạn
  gốc, dịch đoạn bôi đen, giữ đúng thuật ngữ theo bảng thuật ngữ của người dùng. Dùng khi người dùng muốn "dịch",
  "song ngữ", "bản tiếng Anh", "thuật ngữ".
---

# Dịch thuật văn bản công việc

Trước khi dịch, đọc bảng thuật ngữ `references/thuat-ngu.md` (`read_skill_file`). Thuật ngữ có trong bảng thì **bắt
buộc** dùng đúng bản dịch trong bảng, kể cả khi có cách dịch khác hay hơn. Thuật ngữ chuyên ngành không có trong bảng:
dịch theo nghĩa thông dụng của ngành, lần đầu xuất hiện thì giữ nguyên gốc trong ngoặc, ví dụ "Bảo hiểm xã hội (Social
Insurance)".

## Quy tắc chung

1. **Giữ nguyên** số liệu, ngày tháng, số hiệu văn bản (`Số: 12/2026/QĐ-UBND`), tên riêng người và tổ chức, địa chỉ,
   email, đường link. Tiền tệ giữ đơn vị gốc (`1.250.000 đồng` → `VND 1,250,000`); dấu phân cách hàng nghìn theo ngôn
   ngữ đích (tiếng Việt dùng `.`, tiếng Anh dùng `,`).
2. Văn bản hành chính: tên cơ quan, chức danh dịch theo cách gọi chính thức (bảng thuật ngữ); "Kính gửi" → "To",
   "Nơi nhận" → "Recipients", "Căn cứ" → "Pursuant to".
3. Giữ giọng văn của bản gốc: thư trang trọng dịch trang trọng; không thêm lời giải thích của người dịch vào bản dịch.
4. Không dịch từng chữ: dịch ý cả câu, câu dài tiếng Việt có thể tách thành hai câu tiếng Anh nếu rõ nghĩa hơn.
5. Đoạn đã là ngôn ngữ đích (ví dụ trích dẫn tiếng Anh trong văn bản tiếng Việt khi dịch sang tiếng Anh): giữ nguyên.

## Dịch song ngữ trong tài liệu

- Bản dịch đặt **ngay dưới đoạn gốc tương ứng**, mỗi đoạn gốc một đoạn dịch; không gom bản dịch xuống cuối.
- Không sửa chữ của bản gốc. Chèn bằng `writer.replaceAll`: tìm câu cuối của đoạn gốc (đủ dài để chỉ khớp một chỗ),
  thay bằng chính câu đó + xuống dòng + bản dịch.
- Tiêu đề và đề mục cũng dịch theo cách trên; bảng thì dịch nội dung ô bằng `writer.replaceAll` trong ô, không chèn
  dòng mới vào bảng.

## Kiểm tra trước khi trả lời

Đọc lại phần vừa chèn (`writer.getText`): mỗi đoạn gốc có đúng một đoạn dịch ngay dưới, thuật ngữ trong bảng đã dùng
đúng, số liệu trùng khớp với bản gốc.
