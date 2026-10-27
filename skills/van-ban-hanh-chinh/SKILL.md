---
name: van-ban-hanh-chinh
description: >-
  Soạn công văn, quyết định, tờ trình, thông báo đúng thể thức văn bản hành chính Việt Nam (Nghị định
  30/2020/NĐ-CP): quốc hiệu, số ký hiệu, trích yếu, nơi nhận, người ký. Dùng khi người dùng muốn soạn
  "công văn", "quyết định", "tờ trình", "thông báo", "văn bản hành chính", "giấy mời".
apps: [wps]
---

# Văn bản hành chính

**Có tool `doc_from_brief` và đang soạn văn bản MỚI** (tờ trình, công văn, thông báo, biên bản, báo cáo, kế hoạch):
gọi `doc_from_brief` **ngay ở lượt tiếp theo** với đủ thông tin và nội dung — tool tự kiểm tài liệu trống, tự dựng
đúng thể thức và tự soát phân trang. **Không** cần đọc `references/the-thuc.md`, **không** gọi `writer.getText`
trước, không dựng tay từng bảng. Xong thì trả lời, nhắc người dùng các chỗ còn "…" (`placeholders`).

Chỉ khi không có tool đó, hoặc sửa văn bản có sẵn: đọc thể thức chi tiết
`read_skill_file {name: "van-ban-hanh-chinh", path: "references/the-thuc.md"}` rồi dựng theo phần dưới.
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

**Văn bản MỚI** (tờ trình, công văn, thông báo, biên bản, báo cáo, kế hoạch) trong tài liệu trống: dùng tool
`doc_from_brief` — một lần gọi, tool tự dựng phần đầu, tên loại + trích yếu, Kính gửi, phần ký và nơi nhận đúng
thể thức; bạn chỉ khai báo thông tin và nội dung (`body` cùng dạng khối với `writer.writeContent`). Thông tin
người dùng chưa cho thì **bỏ trống** — tool ghi "…" và trả `placeholders` để nhắc người dùng.

Khi không có tool đó, hoặc sửa văn bản có sẵn:

- Phần đầu / phần ký: `writer.insertLayoutTable {rows: [[{lines: [{t: "TRƯỜNG THPT A", bold: true}, "Số: …/…"]},
  {lines: [{t: "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM", bold: true, size: 12}, {t: "Độc lập - Tự do - Hạnh phúc", bold: true},
  {t: "Hà Nội, ngày … tháng … năm …", italic: true}]}]], widths: [40, 60]}` — bảng không viền, định dạng từng dòng.

- (Cách cũ) Bảng phần đầu: `writer.insertTable {values: [["<CƠ QUAN>","CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM"],["Số: …/…","Độc lập - Tự do - Hạnh phúc"],["V/v …","<Địa danh>, ngày … tháng … năm …"]]}`
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

## Checklist (khi dựng tay - không có `doc_from_brief`)

- [ ] Đọc thể thức `references/the-thuc.md`.
- [ ] `writer.getText` **một lần** — tài liệu trống hay đã có nội dung.
- [ ] Dựng phần đầu → tên loại/trích yếu → nội dung → phần cuối.
- [ ] **Soát**: `writer.checkTables` → hai bảng dàn trang phải có `layout: true` (nhờ `borders: false`), và
      không được có `long-header-cell` do gõ lọt chữ. `layout: true` nghĩa là ô trống trong đó là cố ý —
      **không điền gì vào** (khoảng trống ký, ô chờ điền). `writer.getText` đọc lại đủ các thành phần theo
      đúng thứ tự.
