---
name: trinh-bay-chuyen-nghiep
description: >-
  Quy tắc trình bày slide PowerPoint chuyên nghiệp: một thông điệp mỗi slide, lưới và lề, thang cỡ chữ,
  mật độ chữ, khi nào dùng bảng, chuyển chi tiết sang ghi chú thuyết trình. Dùng khi tạo mới hoặc làm
  lại bài thuyết trình, slide, "làm slide cho đẹp", "thiết kế lại slide". Không dùng khi chỉ sửa một chữ.
apps: [wpp]
---

# Trình bày slide chuyên nghiệp

Đọc `tokens.json` của `_design` (khối `slide`, `colors`) và dùng đúng giá trị đó.

## Lưới và vị trí (đơn vị point)

Gọi `wpp.checkLayout` một lần để lấy `slideWidth` × `slideHeight` (16:9 thường 960×540). Với lề
`margin` = 40:

| Vùng | left | top | width | height |
|---|---|---|---|---|
| Tiêu đề slide | 40 | 30 | slideWidth − 80 | 60 |
| Nội dung | 40 | 110 | slideWidth − 80 | slideHeight − 150 |
| Hai cột | 40 và slideWidth/2 + 10 | 110 | slideWidth/2 − 50 | slideHeight − 150 |

Dùng `wpp.addSlide {layout: 12}` (trống) rồi `wpp.addText` với các toạ độ trên — kiểm soát được bố cục.

## Quy tắc nội dung

1. **Một thông điệp mỗi slide**: tiêu đề slide là câu kết luận ("Doanh thu tăng 12%"), không phải nhãn
   ("Doanh thu").
2. Tối đa `maxBulletsPerSlide` gạch đầu dòng, mỗi dòng ≤ `maxWordsPerBullet` từ. Phần giải thích dài
   → `wpp.setNotes`.
3. Cỡ chữ: tiêu đề `type.title`, nội dung `type.body`, chú thích `type.caption`; **không dưới 12pt**.
4. Số liệu so sánh → `wpp.addTable` (≤ 6 dòng) thay vì liệt kê trong đoạn văn.
5. Màu: tiêu đề `primary`, nhấn số liệu quan trọng bằng `accent`; nền để trắng.

## Checklist

- [ ] Đọc bài hiện có (`wpp.listSlides`).
- [ ] Lập dàn ý: mỗi slide = một câu kết luận.
- [ ] Tạo slide theo lưới ở trên.
- [ ] **Soát**: `wpp.checkLayout` → sửa mọi `overflow` (rút gọn chữ, giảm `fontSize` nhưng ≥ 12, hoặc tăng
      `height`), `offslide`, `overlap`, `small-font`, `dense`. Lặp lại đến khi `issueCount` = 0.
- [ ] Nếu có tool `look_at_document` (người dùng bật QA thị giác): gọi **một lần** sau bước soát trên để nhìn
      slide (tràn chữ, lệch lưới, tương phản) rồi sửa điểm còn lỗi.
