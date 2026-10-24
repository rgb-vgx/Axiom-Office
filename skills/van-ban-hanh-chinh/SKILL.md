---
name: van-ban-hanh-chinh
description: >-
  Soạn công văn, quyết định, tờ trình, thông báo đúng thể thức văn bản hành chính Việt Nam (Nghị định
  30/2020/NĐ-CP): quốc hiệu, số ký hiệu, trích yếu, nơi nhận, người ký. Dùng khi người dùng muốn soạn
  "công văn", "quyết định", "tờ trình", "thông báo", "văn bản hành chính", "giấy mời".
apps: [wps]
---

# Văn bản hành chính

Thể thức chi tiết (vị trí, cỡ chữ, kiểu chữ từng thành phần):
`read_skill_file {name: "van-ban-hanh-chinh", path: "references/the-thuc.md"}` — đọc trước khi soạn.
Dùng cùng skill `the-thuc-van-ban` cho phần nội dung.

## Bố cục (từ trên xuống)

1. **Phần đầu** = bảng 2 cột không viền: trái = tên cơ quan chủ quản / tên cơ quan ban hành / số ký
   hiệu; phải = quốc hiệu / tiêu ngữ / địa danh, ngày tháng.
2. **Tên loại + trích yếu** (quyết định, tờ trình…) căn giữa; công văn thì trích yếu "V/v …" nằm dưới
   số ký hiệu ở cột trái.
3. **Kính gửi** (công văn).
4. **Nội dung**: căn đều, Times New Roman cỡ `type.body` (13).
5. **Phần cuối** = bảng 2 cột không viền: trái = "Nơi nhận:" + danh sách; phải = chức vụ người ký
   (in hoa, đậm), khoảng trống ký, họ tên (đậm).

## Cách dựng bằng lệnh

- Bảng phần đầu: `writer.insertTable {values: [["<CƠ QUAN>","CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM"],["Số: …/…","Độc lập - Tự do - Hạnh phúc"],["V/v …","<Địa danh>, ngày … tháng … năm …"]]}`
  rồi `writer.formatTable {borders: false, font: "Times New Roman", size: 13, alignment: "center"}`.
- **Mỗi lần gọi `writer.insertStyledText` là một KHỐI đoạn văn**: dùng `\n` để tách các dòng thành đoạn
  riêng và truyền `alignment` cho cả khối. **Không** gọi `writer.setParagraphAlignment` sau từng dòng —
  mọi dòng sẽ dồn vào một đoạn và lần căn cuối cùng đè lên cả khối.
  - Quốc hiệu + tiêu ngữ + tên loại/trích yếu: một lần, `alignment: "center"`.
  - Thân văn bản (Kính gửi, các đoạn nội dung): một lần, `alignment: "justify"`.
  - Phần ký (Trân trọng / Người làm đơn / họ tên): một lần, `alignment: "center"` (hoặc `right`).
  - Chỉ tách thêm lần gọi khi **định dạng chữ khác nhau** (đậm/nhạt/cỡ/màu); các dòng cùng định dạng thì gộp.
  - Khối có `alignment` **tự ngắt đoạn sau khi chèn** (`break` mặc định true), nên lần gọi kế tiếp bắt đầu
    đoạn mới — **không thêm dòng trống** (`\n\n`) để tạo khoảng cách; dùng một `\n` giữa các đoạn.
- `bold`/`italic`/`underline`/`color` được đặt tường minh cho từng lần gọi: khối không truyền `bold` sẽ là
  chữ thường, không kế thừa khối trước.
- Phần cuối dạng bảng: như trên, `borders: false`.

Thông tin người dùng chưa cho (số văn bản, ngày, người ký) → để "…" và nhắc trong câu trả lời, **không
bịa**.

## Checklist

- [ ] Đọc thể thức `references/the-thuc.md`.
- [ ] `writer.getText` — tài liệu trống hay đã có nội dung.
- [ ] Dựng phần đầu → tên loại/trích yếu → nội dung → phần cuối.
- [ ] **Soát**: `writer.checkTables` → hai bảng dàn trang phải có `layout: true` (nhờ `borders: false`), và
      không được có `long-header-cell` do gõ lọt chữ. `layout: true` nghĩa là ô trống trong đó là cố ý —
      **không điền gì vào** (khoảng trống ký, ô chờ điền). `writer.getText` đọc lại đủ các thành phần theo
      đúng thứ tự.
