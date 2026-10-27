---
name: bao-cao-du-lieu
description: >-
  Quy tắc trình bày bảng dữ liệu và báo cáo Excel: hàng tiêu đề, number format theo loại số, hàng tổng,
  công thức thay cho số tính tay, mật độ trình bày. Dùng khi tạo mới hoặc làm đẹp bảng tính,
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
   `date` → `numFmt` cho cả cột dữ liệu.
5. **Hàng tiêu đề**: các giá trị của `spreadsheet.header`. **Hàng tổng**: giá trị `spreadsheet.totalRow`.
   **Gom mọi định dạng vào MỘT `et.formatRanges`**: `{formats: [{sheet: "Data", range: "A1:I1", bold: true,
   fillColor: "#DDEBF7"}, {sheet: "Data", range: "D2:E40", numFmt: "#,##0"}, {sheet: "Config", range: "B2:B4",
   fillColor: "#FFF2CC"}, ...]}` — tiêu đề, ô nhập liệu, number format của mọi sheet trong một lần; đừng gọi
   `et.formatRange` từng vùng (mỗi lần một vòng).
6. Ghi dữ liệu **một lần** bằng mảng 2 chiều đầy đủ; không ghi từng ô.
7. **Bảng dùng được**: bảng dữ liệu → `et.createTable {range}` (có nút lọc), `et.freezePanes {rows: 1}`
   giữ hàng tiêu đề; cột chỉ nhận vài giá trị → `et.setValidation {range, source: "Config!$A$2:$A$5"}`
   (hoặc `list`). Biểu đồ đọc thẳng vùng ở sheet khác: `et.addChart {sheet: "Dashboard", range:
   "Data!A1:C7"}` — không chép dữ liệu sang làm bảng trung gian.
8. **Dashboard / trang tổng quan**: tool `dashboard_from_data {source: "Data!A1:E13", kpis, charts}` (ô KPI là
   công thức, biểu đồ đọc thẳng sheet dữ liệu) thay cho tự ghi từng ô KPI và từng biểu đồ.
9. **Tô màu theo mức** dùng kiểu trực quan của `et.setConditionalFormat` thay cho công thức tự viết: `colorScale`
   (thang màu thấp→cao), `dataBar`, `iconSet {icons: "3traffic"}`, `top`/`bottom {rank: 3, fillColor}`,
   `aboveAverage`, `duplicate {fillColor}`. Nhiều rule trên **cùng một vùng** phải gửi trong MỘT lần gọi (gọi lại trên
   đúng vùng đó thì rule cũ bị thay; rule của vùng khác giữ nguyên). `top` xếp hạng MỌI ô của vùng: tô cả hàng của
   top 3 thì dùng rule `formula` trên cả hàng, vd `{operator: "formula", formula1: "RANK($B2,$B$2:$B$13)<=3", fillColor}`.

## Checklist

- [ ] Đọc sheet hiện có (`et.listSheets`, `et.readRange`) để không ghi đè dữ liệu của người dùng.
- [ ] Ghi bảng bằng một lệnh `et.writeRange` (tiêu đề + dữ liệu + công thức).
- [ ] `formulaErrors` trong kết quả `et.writeRange` phải rỗng; còn thì sửa công thức và ghi lại.
- [ ] Định dạng: tiêu đề, number format, hàng tổng — một lần `et.formatRanges`.
- [ ] **Soát**: `et.checkRange` → sửa `numbers-as-text`, `mixed-types`, `no-number-format`, `empty-header`,
      `error-values`. Lặp đến khi hết các loại đó.
- [ ] Sổ nhiều sheet: `et.checkRange` **không** truyền `range`/`sheet` còn soát ô lỗi ở **mọi** sheet
      (`error-cells`: "Sheet!D4 #DIV/0!") — sửa hết trước khi trả lời.
- [ ] Riêng `outside-table` (sheet đã có nội dung ngoài vùng bảng): **không phải lỗi luôn**. Nếu sheet có
      sẵn dữ liệu khác cạnh bảng thì đó là bảng/dữ liệu của người dùng — **không xoá**. Chỉ xoá ô mà
      **chính lượt này** bạn đã ghi nhầm; không chắc thì báo người dùng. Muốn chỉ soát đúng bảng bạn vừa
      dựng thì gọi `et.checkRange {range}`.
