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

1. **Heading thật**: viết mới dùng `writer.heading {text, level}` (level 1 cho mục lớn, 2 cho mục con);
   đoạn **có sẵn** cần thành tiêu đề thì `writer.setStyle {paragraphs: [n], style: "Heading 2"}` (số đoạn từ
   `writer.outline`) — không giả heading bằng chữ đậm, không gõ lại chữ. Tối đa 3 cấp.
2. **Font**: văn bản hành chính và báo cáo dùng `fonts.document` (Times New Roman) cỡ `type.body`
   (13); chữ trong bảng cỡ `type.table`.
3. **Đoạn văn**: dùng `writer.insertStyledText {text, font, size, alignment: "justify"}` với `\n` giữa các
   đoạn — khối có `alignment` tự ngắt đoạn. **Không** gọi `writer.setParagraphAlignment` sau từng dòng và
   **không** gõ dòng trống: mọi dòng sẽ dồn vào một đoạn, lần căn cuối đè lên cả khối. Tiêu đề văn bản căn
   giữa; tiêu đề mục dùng `writer.heading`. `writer.appendText` chỉ để nối đoạn văn thường vào cuối — nó
   cũng tự đặt lại định dạng, không kế thừa bold/căn lề của đoạn trước.
   **Viết nội dung mới cả mục một lần** bằng `writer.writeContent {blocks}` — tiêu đề, đoạn văn (căn đều),
   gạch đầu dòng, danh sách số, bảng kèm chú thích, ngắt trang, theo đúng thứ tự, nối vào cuối bài:
   `[{heading: "1. Kết quả", level: 1}, {text: "Đoạn 1\nĐoạn 2"}, {bullets: ["Ý 1", "Ý 2"]},
   {table: [["Năm", "Số"], ["2026", "12"]], caption: "Kết quả 2026"}, {pageBreak: true}]` — một lệnh cho
   một hay vài mục thay vì mỗi đoạn một lệnh. Danh sách viết bằng `bullets`/`numbers`, không gõ "-" hay "1." tay.
4. **Bảng**: chèn bằng `writer.insertTable {values, style}` với `values` là mảng 2 chiều đầy đủ (hàng
   đầu là tiêu đề), rồi `writer.formatTable` với `style` = `document.tableStyle`, `font`, `size`.
   Chú thích bảng: `writer.insertCaption {text}` **ngay trước** `writer.insertTable` (tự đánh số "Bảng 1.");
   bảng có sẵn thì thêm `before` = số của bảng trong `writer.outline`. Thêm hàng vào bảng **có sẵn**:
   `writer.addTableRows {table, rows: [[...]]}` — không dựng lại bảng, không thay chuỗi bằng `replaceAll`.
   Số thập phân viết kiểu Việt Nam: `98,4`, `1.250.000` (không `98.4`).
5. **Ngắt trang**: văn bản dài thì mục lớn bắt đầu ở trang mới — `{pageBreak: true}` trong
   `writer.writeContent`, hoặc `writer.insertPageBreak {before: n}` trước heading có sẵn; đừng gõ dòng trống
   cho hết trang.
6. **Báo cáo nhiều trang**: mục lục `writer.insertToc` (sau trang bìa; gọi lại một lần khi viết xong để
   cập nhật số trang), số trang và đầu trang `writer.setHeaderFooter {header, pageNumber: true}` — không gõ
   "MỤC LỤC" hay "Trang 1" bằng tay.
7. **Độ dài theo số trang**: người dùng nói "khoảng N trang" thì tính **ngân sách chữ trước khi viết**: một trang
   A4 chữ Times 13 chứa khoảng **280 chữ**; trang bìa và mục lục mỗi thứ chiếm một trang, mỗi bảng nhỏ khoảng nửa
   trang. Ví dụ "6–8 trang" có bìa + mục lục + 2 bảng → phần chữ ~5 trang ≈ **1.400 chữ**, chia cho các mục.
8. **Không** tự lưu, không đổi tên file.

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
- [ ] **Soát phân trang** `writer.checkLayout {maxPages}` trước khi trả lời (`maxPages` = mức trên người dùng nói,
      "6–8 trang" → 8): `too-long` là **lỗi phải sửa** — rút gọn đoạn, bỏ ý trùng cho đến khi hết; `blank-page` / `empty-paragraphs` → `writer.deleteParagraphs` đúng các
      số đoạn được chỉ; `heading-at-page-bottom` và `split-table` → làm theo cách sửa trong `detail`.
