---
name: chinh-ta-tieng-viet
description: >-
  Soát chính tả tiếng Việt cả bài hay một đoạn: dấu hỏi/ngã, phụ âm đầu, vần, gõ nhầm Telex/VNI, lặp từ, dấu câu,
  viết hoa. Sửa bằng Track Changes, chỗ không chắc thì ghi chú. Dùng khi muốn "soát chính tả", "sửa lỗi đánh máy".
  Không viết lại câu.
apps: [wps]
---

# Soát chính tả tiếng Việt

Việc là **sửa lỗi**, không phải biên tập: câu đúng chính tả thì giữ nguyên dù có cách viết hay hơn. Mỗi chỗ sửa phải
là lỗi mà người viết cẩn thận nhìn vào cũng công nhận là sai.

Danh sách cặp hay nhầm kèm ví dụ: `read_skill_file {name: "chinh-ta-tieng-viet", path: "references/loi-thuong-gap.md"}`.
Đọc trước khi soát tài liệu dài.

## Sửa ngay (lỗi chắc chắn)

1. **Sai dấu hỏi/ngã, sai phụ âm đầu, sai vần** khi chỉ một cách viết có nghĩa trong ngữ cảnh: "sữa chữa" → "sửa chữa",
   "giải quyếc" → "giải quyết", "trân thành" → "chân thành", "sử lý" → "xử lý", "căng cứ" → "căn cứ".
2. **Gõ nhầm bộ gõ**: chữ Telex/VNI còn sót ("ddaay" → "đây", "Vieejt" → "Việt", "khoong" → "không", "nguwowif" →
   "người"), dấu đặt sai chữ ("hoạtđộng" → "hoạt động").
3. **Lặp từ** liền nhau do gõ: "của của", "và và", "các các", "được được".
4. **Khoảng trắng và dấu câu**: không có khoảng trắng trước `, . ; : ! ?` và một khoảng trắng sau; không có khoảng
   trắng ngay trong ngoặc `( … )` và ngoặc kép; hai khoảng trắng liền nhau thành một.
5. **Viết hoa** đầu câu, tên riêng người và địa danh ("hà nội" → "Hà Nội"). Tên cơ quan, tổ chức theo Nghị định
   30/2020/NĐ-CP: viết hoa chữ đầu của từ chỉ loại hình và chữ đầu của cụm chỉ lĩnh vực, không viết hoa mọi chữ:
   "Bộ Tài Chính" → "Bộ Tài chính", "Ủy Ban Nhân Dân" → "Ủy ban nhân dân".

## Không sửa, chỉ ghi chú khi đáng ngờ

- Câu tối nghĩa, từ có thể dùng sai nghĩa ("dành/giành", "nghỉ/nghĩ" khi ngữ cảnh không rõ), thuật ngữ gọi hai cách
  trong cùng tài liệu: ghi chú bằng `writer.addComment` (find = cụm đó), nêu lý do và gợi ý cách viết.
- Mỗi lỗi lặp nhiều lần chỉ ghi chú **một** lần ở chỗ đầu tiên, nói rõ "các chỗ khác tương tự".

## Không đụng tới

- **Hai cách viết đều được chấp nhận**: "hoà/hòa", "thuỷ/thủy", "khoẻ/khỏe" (vị trí dấu kiểu cũ/mới); "kỹ/kĩ", "lý/lí",
  "quy/qui" (i/y). Không đổi qua lại. Chỉ khi một tài liệu trộn lẫn hai kiểu cho **cùng một từ** thì ghi chú một lần.
- Tên riêng, tên cơ quan có trong văn bản gốc mà không chắc cách viết đúng, số hiệu văn bản, số liệu, ngày tháng, đơn
  vị, địa chỉ, email, đường link, trích dẫn nguyên văn, từ và câu tiếng nước ngoài, thuật ngữ chuyên ngành.
- Văn phong, độ dài câu, cách chọn từ đúng nhưng chưa hay: đó là việc của *Trau chuốt*, không phải soát chính tả.
- Khác biệt không nhìn thấy (Unicode dựng sẵn và tổ hợp): không tìm/thay được an toàn, bỏ qua.

## Cách sửa trong WPS

- Đọc cả bài bằng `writer.outline` (`maxChars` lớn, ví dụ 4000) để có số đoạn; tài liệu ngắn dùng `writer.getText`.
- Gom các chỗ sửa vào `writer.replaceMany` (tối đa 200 cặp một lần). `find` lấy cả cụm 2–4 từ quanh chỗ sai để **chỉ
  khớp đúng chỗ đó**; `replace` là chính cụm ấy đã sửa, giữ nguyên chữ hoa/thường. Xem `counts` và `notFound` trong kết
  quả: cặp nào thay nhiều hơn số chỗ định sửa là `find` quá ngắn — kiểm lại bằng `writer.listRevisions`.
- Lượt soát chính tả bật Track Changes: mỗi chỗ sửa hiện thành thay đổi được theo dõi để người dùng *Chấp nhận/Từ chối*
  trong WPS; không tự chấp nhận thay người dùng.
- Đang bôi đen một đoạn thì chỉ soát trong đoạn đó.

## Báo lại

Ngắn gọn: số chỗ đã sửa (theo nhóm: dấu, phụ âm/vần, gõ nhầm, dấu câu, viết hoa), số ghi chú, 3–5 ví dụ "sai → đúng".
Không thấy lỗi thì nói rõ là không thấy lỗi, không sửa cho có.
