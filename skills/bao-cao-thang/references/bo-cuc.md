# Bố cục từng slide (slide 960×540pt, lề 40)

Nếu `wpp.checkLayout` cho kích thước khác, co giãn các số theo tỉ lệ.

## 1. Tiêu đề

- `wpp.addText {text: "Báo cáo tháng 09/2026", left: 40, top: 180, width: 880, height: 80, fontSize: 40, bold: true, color: "#1F4E79", align: "center"}`
- `wpp.addText {text: "<đơn vị> · <người trình bày>", left: 40, top: 280, width: 880, height: 40, fontSize: 20, color: "#595959", align: "center"}`

## 2. Kết quả chính (3 số)

- Tiêu đề slide: `left: 40, top: 30, width: 880, height: 60, fontSize: 32, bold: true, color: "#1F4E79"`.
- Ba khối ngang, mỗi khối rộng 280, cách nhau 20: `left` = 40 / 340 / 640, `top: 150`:
  - số: `height: 80, fontSize: 40, bold: true, color: "#2E75B6", align: "center"`
  - nhãn ngay dưới (`top: 240`): `height: 60, fontSize: 18, color: "#262626", align: "center"`

## 3. Bảng so sánh

- Tiêu đề slide như trên.
- `wpp.addTable {values: [["Chỉ tiêu","Tháng này","Tháng trước","Thay đổi"], ...], left: 40, top: 110, width: 880, height: 300}`
- Tối đa 6 dòng dữ liệu; cột "Thay đổi" ghi dạng "+12%" / "−5%".

## 4. Vấn đề — 5. Kế hoạch

- Tiêu đề slide như trên.
- Một hộp nội dung: `left: 40, top: 110, width: 880, height: 380, fontSize: 20`, mỗi ý một dòng bắt đầu
  bằng "• ", nối các dòng bằng ký tự xuống dòng trong `text`.
- Quá 4 ý hoặc chữ tràn (`overflow`) → tách thành hai slide hoặc chuyển chi tiết sang `wpp.setNotes`.
