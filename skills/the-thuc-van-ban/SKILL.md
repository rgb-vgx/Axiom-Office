---
name: the-thuc-van-ban
description: >-
  Quy tắc trình bày tài liệu Word: phân cấp heading, font và cỡ chữ, căn lề đoạn, bảng thống nhất, chú
  thích bảng, ngắt trang. Dùng khi soạn mới hoặc trình bày lại báo cáo, tờ trình, kế hoạch, biên bản,
  tài liệu Word dài, hay khi người dùng muốn văn bản "đẹp", "chuẩn". Không sửa một câu.
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
5. **Ngắt trang**: văn bản dài thì mục lớn bắt đầu ở trang mới — `writer.insertPageBreak` ngay trước
   heading đó; đừng gõ dòng trống cho hết trang.
6. **Không** tự lưu, không đổi tên file.

## Checklist

- [ ] Đọc tài liệu hiện có (`writer.getText`).
- [ ] Dựng khung: tiêu đề văn bản → các heading → nội dung.
- [ ] Điền nội dung; bảng dùng mảng 2 chiều, không gõ bảng bằng dấu tab.
- [ ] **Soát**: `writer.checkTables` → đọc `tables[].layout` để biết bảng nào là bảng dàn trang, rồi:
  - `empty-cells` trong bảng **dữ liệu**: điền cho đủ, hoặc ghi "—" nếu cố ý để trống.
  - Bảng có `layout: true` (bảng bạn đã gọi `formatTable {borders:false}`): ô trống ở đó là **cố ý**
    (khoảng trống ký, ô chờ điền) — **không điền gì vào**, điền vào là sai thể thức.
  - `long-header-cell` (đoạn văn lọt vào ô tiêu đề): nếu bảng do **chính lượt này** bạn chèn và sau đó
    chưa làm gì khác thì `writer.undo` lùi về trước `writer.insertTable` rồi chèn lại bằng `values`
    đúng. Bảng có sẵn trong tài liệu, hoặc đã làm việc khác sau đó, thì **báo người dùng**, đừng tự vá.
    **Không** dùng `writer.replaceAll` để vá một ô: nó thay **mọi** lần xuất hiện trong cả tài liệu và
    không phân biệt hoa/thường.
- [ ] Đọc lại `writer.getText` để chắc thứ tự mục đúng.
