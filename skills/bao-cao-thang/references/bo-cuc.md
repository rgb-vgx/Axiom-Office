# Bố cục từng slide (slide 960×540pt, lề 40)

Mọi **cỡ chữ và màu** ở đây ghi bằng tên token, không ghi giá trị: đọc
`read_skill_file {name: "_design", path: "tokens.json"}` rồi thay giá trị thật vào lệnh. Toạ độ
(`left`/`top`/`width`/`height`) là bố cục riêng của bộ slide này, dùng đúng như dưới.

Nếu `wpp.checkLayout` cho kích thước khác, co giãn các số theo tỉ lệ.

## 1. Tiêu đề

- Tiêu đề bìa: `wpp.addText {text: "Báo cáo tháng 09/2026", left: 40, top: 180, width: 880, height: 80, bold: true, align: "center"}`
  — `fontSize` lấy `slide.type.hero`, `color` lấy `colors.primary`.
- Dòng dưới: `wpp.addText {text: "<đơn vị> · <người trình bày>", left: 40, top: 280, width: 880, height: 40, align: "center"}`
  — `fontSize` lấy `slide.type.body`, `color` lấy `colors.muted`.

## 2. Kết quả chính (3 số)

- Tiêu đề slide: `left: 40, top: 30, width: 880, height: 60, bold: true` — `fontSize` lấy
  `slide.type.title`, `color` lấy `colors.primary`.
- Ba khối ngang, mỗi khối rộng 280, cách nhau 20: `left` = 40 / 340 / 640, `top: 150`:
  - số: `height: 80, bold: true, align: "center"` — `fontSize` lấy `slide.type.figure`, `color` lấy
    `colors.accent`.
  - nhãn ngay dưới (`top: 240`): `height: 60, align: "center"` — `fontSize` lấy `slide.type.body`,
    `color` lấy `colors.text`.

## 3. Bảng so sánh

- Tiêu đề slide như trên.
- `wpp.addTable {values: [["Chỉ tiêu","Tháng này","Tháng trước","Thay đổi"], ...], left: 40, top: 110, width: 880, height: 300}`
- Tối đa 6 dòng dữ liệu; cột "Thay đổi" ghi dạng "+12%" / "−5%".

## 4. Vấn đề — 5. Kế hoạch

- Tiêu đề slide như trên.
- Một hộp nội dung: `left: 40, top: 110, width: 880, height: 380`, `fontSize` lấy `slide.type.body`,
  mỗi ý một dòng bắt đầu bằng "• ", nối các dòng bằng ký tự xuống dòng trong `text`.
- Quá 4 ý hoặc chữ tràn (`overflow`) → tách thành hai slide hoặc chuyển chi tiết sang `wpp.setNotes`.
