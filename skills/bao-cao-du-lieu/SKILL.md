---
name: bao-cao-du-lieu
description: >-
  Quy tắc trình bày bảng dữ liệu và báo cáo Excel: hàng tiêu đề, number format theo loại số, hàng tổng,
  công thức thay cho số tính tay, mật độ và độ rộng bảng. Dùng khi tạo mới hoặc làm đẹp bảng tính,
  bảng số liệu, báo cáo doanh thu, thống kê, KPI trong Excel. Không dùng khi chỉ sửa một ô.
apps: [et]
---

# Trình bày dữ liệu Excel

Đọc `tokens.json` của `_design` (khối `spreadsheet`, `colors`) và dùng đúng giá trị đó.

## Quy tắc

1. **Bảng bắt đầu ở A1** (hoặc dưới một dòng tiêu đề), hàng đầu là tiêu đề cột, không để cột/hàng trống
   giữa bảng.
2. **Số là số**: trong `values` của `et.writeRange` ghi số dạng JSON number (`9.5`, không phải `"9.5"`).
3. **Công thức** cho mọi giá trị tính được: `"=SUM(B2:B10)"`, `"=AVERAGE(B2:D2)"`, `"=B2/C2"` — không tính
   tay rồi ghi số.
   - Viết công thức **theo cú pháp en-US, dấu `,` giữa các tham số**: `=ROUND(AVERAGE(C2:E2),1)`,
     `=IF(A1>=8,"Giỏi",IF(A1>=5,"Khá","Yếu"))`. Đừng tự đổi sang `;` — `et.writeRange` tự đổi nếu máy
     đó cần (LibreOffice cài ở một số nơi đòi `;`, Excel qua COM thì luôn nhận `,`).
   - Sau khi ghi, **đọc `formulaErrors`** trong kết quả. Có phần tử nghĩa là ô đó vẫn lỗi: xem `text`
     (`Err:508` = sai dấu phân cách tham số, `#DIV/0!` = mẫu bằng 0, `#REF!` = tham chiếu hỏng...),
     sửa công thức rồi ghi lại — đừng coi `"written": n` là đã xong.
4. **Number format** theo loại: số nguyên `integer`, tiền `vnd`, tỉ lệ `percent`, điểm `score`, ngày
   `date` → `et.formatRange {range, numFmt}` cho cả cột dữ liệu.
5. **Hàng tiêu đề**: `et.formatRange` với các giá trị của `spreadsheet.header`. **Hàng tổng**: giá trị
   `spreadsheet.totalRow`.
6. Ghi dữ liệu **một lần** bằng mảng 2 chiều đầy đủ; không ghi từng ô.

## Checklist

- [ ] Đọc sheet hiện có (`et.listSheets`, `et.readRange`) để không ghi đè dữ liệu của người dùng.
- [ ] Ghi bảng bằng một lệnh `et.writeRange` (tiêu đề + dữ liệu + công thức).
- [ ] `formulaErrors` trong kết quả `et.writeRange` phải rỗng; còn thì sửa công thức và ghi lại.
- [ ] Định dạng: tiêu đề, number format, hàng tổng.
- [ ] **Soát**: `et.checkRange` → sửa mọi `issues`: `outside-table` (dữ liệu ghi lạc ô — xoá bằng cách ghi
      `""` vào đúng các ô đó), `numbers-as-text`, `mixed-types`, `no-number-format`, `empty-header`,
      `error-values`. Lặp đến khi `issueCount` = 0.
