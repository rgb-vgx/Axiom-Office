---
name: van-ban-hanh-chinh
description: >-
  Soạn công văn, quyết định, tờ trình, thông báo đúng thể thức văn bản hành chính Việt Nam (Nghị định
  30/2020/NĐ-CP): tên cơ quan, quốc hiệu, tiêu ngữ, số ký hiệu, địa danh ngày tháng, trích yếu, nơi
  nhận, chức vụ và người ký. Dùng khi người dùng muốn soạn "công văn", "quyết định", "tờ trình",
  "thông báo", "văn bản hành chính", "giấy mời".
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
4. **Nội dung**: căn đều, Times New Roman 13–14.
5. **Phần cuối** = bảng 2 cột không viền: trái = "Nơi nhận:" + danh sách; phải = chức vụ người ký
   (in hoa, đậm), khoảng trống ký, họ tên (đậm).

## Cách dựng bằng lệnh

- Bảng phần đầu: `writer.insertTable {values: [["<CƠ QUAN>","CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM"],["Số: …/…","Độc lập - Tự do - Hạnh phúc"],["V/v …","<Địa danh>, ngày … tháng … năm …"]]}`
  rồi `writer.formatTable {borders: false, font: "Times New Roman", size: 13, alignment: "center"}`.
- Nội dung: `writer.insertStyledText {text, font: "Times New Roman", size: 14}` +
  `writer.setParagraphAlignment {alignment: "justify"}`.
- Phần cuối: bảng 2 cột như trên, `borders: false`.

Thông tin người dùng chưa cho (số văn bản, ngày, người ký) → để "…" và nhắc trong câu trả lời, **không
bịa**.

## Checklist

- [ ] Đọc thể thức `references/the-thuc.md`.
- [ ] `writer.getText` — tài liệu trống hay đã có nội dung.
- [ ] Dựng phần đầu → tên loại/trích yếu → nội dung → phần cuối.
- [ ] **Soát**: `writer.checkTables` (2 bảng dàn trang không được có `long-header-cell` do gõ lọt chữ),
      `writer.getText` đọc lại đủ các thành phần theo đúng thứ tự.
