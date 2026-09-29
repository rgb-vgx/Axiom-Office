---
name: the-thuc-van-ban
description: >-
  Quy tắc trình bày tài liệu Word: phân cấp tiêu đề (heading), font và cỡ chữ, căn lề đoạn, bảng có
  kiểu thống nhất, chú thích bảng, ngắt trang hợp lý. Dùng khi soạn mới hoặc trình bày lại báo cáo,
  tờ trình, kế hoạch, biên bản, tài liệu Word dài, hay khi người dùng muốn văn bản "đẹp", "chuẩn".
  Không dùng cho sửa một câu hay in đậm một từ.
apps: [wps]
---

# Thể thức trình bày văn bản Word

Đọc `tokens.json` của `_design` (khối `document`, `colors`, `fonts`) và dùng đúng giá trị đó.

## Quy tắc

1. **Heading thật**: dùng `writer.heading {text, level}` (level 1 cho mục lớn, 2 cho mục con) — không
   giả heading bằng chữ đậm. Tối đa 3 cấp.
2. **Font**: văn bản hành chính và báo cáo dùng `fonts.document` (Times New Roman) cỡ `type.body`
   (13); chữ trong bảng cỡ `type.table`.
3. **Đoạn văn**: nội dung căn đều (`writer.setParagraphAlignment {alignment: "justify"}`), tiêu đề văn
   bản căn giữa.
4. **Bảng**: chèn bằng `writer.insertTable {values, style}` với `values` là mảng 2 chiều đầy đủ (hàng
   đầu là tiêu đề), rồi `writer.formatTable` với `style` = `document.tableStyle`, `font`, `size`.
   Đặt một dòng chú thích ngắn ngay dưới bảng ("Bảng 1. …").
5. **Không** tự lưu, không đổi tên file.

## Checklist

- [ ] Đọc tài liệu hiện có (`writer.getText`).
- [ ] Dựng khung: tiêu đề văn bản → các heading → nội dung.
- [ ] Điền nội dung; bảng dùng mảng 2 chiều, không gõ bảng bằng dấu tab.
- [ ] **Soát**: `writer.checkTables` → sửa `empty-cells` (điền hoặc ghi "—") và `long-header-cell`
      (đoạn văn bị lọt vào ô tiêu đề: sửa lại ô đó bằng `writer.replaceAll`). Đọc lại `writer.getText`
      để chắc thứ tự mục đúng.
