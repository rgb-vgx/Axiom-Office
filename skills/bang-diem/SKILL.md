---
name: bang-diem
description: >-
  Lập bảng điểm học sinh/sinh viên trong Excel: STT, họ tên, điểm từng môn, điểm trung bình bằng công
  thức, xếp loại, hàng tổng hợp, định dạng điểm một chữ số thập phân. Dùng khi người dùng muốn "bảng
  điểm", "điểm trung bình", "xếp loại học lực", "tổng kết điểm lớp".
apps: [et]
---

# Bảng điểm

Dùng cùng skill `bao-cao-du-lieu` (number format, hàng tiêu đề, soát). Chỉ tạo **đúng một** bảng trên
sheet đang mở; không tạo sổ mới.

## Cột mặc định

`STT | Họ và tên | <môn 1> | <môn 2> | … | Trung bình | Xếp loại`

- **Trung bình**: công thức `=ROUND(AVERAGE(C2:E2),1)` (điều chỉnh cột theo số môn) cho từng dòng.
- **Xếp loại** (thang 10): `=IF(F2>=8,"Giỏi",IF(F2>=6.5,"Khá",IF(F2>=5,"Trung bình","Yếu")))`.
- Hàng cuối **Trung bình lớp** ngay dưới dòng dữ liệu cuối: `=ROUND(AVERAGE(F2:F<n>),1)`. Chỉ một hàng
  tổng hợp — không thêm hàng trung bình thứ hai.

Người dùng không cho tên/điểm thì dùng dữ liệu mẫu hợp lý và nói rõ đó là dữ liệu mẫu.

## Checklist

- [ ] `et.listSheets` + `et.readRange` vùng A1:J30 để biết sheet có trống không; có dữ liệu thì hỏi ý
      qua câu trả lời thay vì ghi đè.
- [ ] Một lệnh `et.writeRange {range: "A1", values: [...]}` gồm tiêu đề, dữ liệu (điểm là số), công thức,
      hàng trung bình lớp.
- [ ] `et.formatRange`: hàng 1 theo `spreadsheet.header`; các cột điểm + trung bình `numFmt: "0.0"`,
      `horizontal: "center"`; hàng trung bình lớp theo `spreadsheet.totalRow`.
- [ ] **Soát** `et.checkRange` (không truyền range) → `issueCount` phải = 0, đặc biệt không có
      `outside-table`. Đọc lại `et.readRange` vùng bảng để chắc công thức ra số.
