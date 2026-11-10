# ĐỀ BÀI: WORKBOOK QUẢN LÝ NGÂN SÁCH VÀ TIẾN ĐỘ DỰ ÁN AI

Dựng trong sổ Excel đang mở một workbook có dữ liệu mẫu cho 3 dự án trong 6 tháng (tháng 1–6/2026), gồm đúng 4 sheet:

- `Dashboard`: tổng ngân sách, tổng chi phí thực tế, phần ngân sách còn lại và tỷ lệ hoàn thành; có biểu đồ so sánh
  kế hoạch với thực tế theo tháng.
- `Project_Data`: danh sách dự án với các cột Mã dự án, Tên dự án, Người phụ trách, Ngân sách, Chi phí thực tế, Ngày
  bắt đầu, Ngày kết thúc, Tỷ lệ hoàn thành, Trạng thái.
- `Monthly_Budget`: kế hoạch và chi phí thực tế theo tháng của từng dự án.
- `Config`: ngưỡng cảnh báo vượt ngân sách (ví dụ 90%) và danh sách trạng thái hợp lệ (Chưa bắt đầu, Đang thực hiện,
  Tạm dừng, Hoàn thành).

## Tiêu chí nghiệm thu

1. Phân biệt rõ ô nhập liệu, ô công thức và kết quả tổng hợp (ví dụ màu nền ô nhập liệu khác ô công thức).
2. Mọi con số tổng hợp là công thức tham chiếu đúng vùng; không nhập cứng kết quả tính toán. Chi phí thực tế của
   `Project_Data` là tổng theo tháng từ `Monthly_Budget`.
3. Có Excel Table (hoặc ít nhất bộ lọc), freeze panes ở hàng tiêu đề và data validation cho cột Trạng thái lấy danh
   sách từ `Config`.
4. Có định dạng có điều kiện phản ánh điều kiện thật: chi phí vượt ngưỡng cảnh báo của `Config`, dự án quá hạn (đã
   qua ngày kết thúc mà chưa Hoàn thành), ô dữ liệu bắt buộc bị trống.
5. Định dạng số tiền (đồng, phân cách hàng nghìn), phần trăm và ngày tháng (dd/mm/yyyy) nhất quán.
6. Biểu đồ ở `Dashboard` lấy dữ liệu từ vùng nguồn để tự cập nhật khi dữ liệu đổi.
7. Dữ liệu mẫu là giả định: ghi rõ điều đó trong workbook.
8. Không có ô lỗi công thức (#REF!, #VALUE!, #DIV/0!, #NAME?).

Không lưu tệp; làm xong thì báo ngắn gọn đã làm gì và phần nào chưa làm được.
