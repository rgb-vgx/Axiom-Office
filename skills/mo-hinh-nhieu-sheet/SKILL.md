---
name: mo-hinh-nhieu-sheet
description: >-
  Dựng MÔ HÌNH NHIỀU SHEET trong Excel và tự soát trước khi trả bài: tách sheet dữ liệu thô / giả định /
  tính toán / dashboard / sheet kiểm tra PASS-FAIL, mọi con số đọc từ ô giả định, mọi sheet đề yêu cầu
  đều phải được dựng, và phải đổi một giả định rồi đọc lại mới coi là xong. Dùng cho "mô hình tài chính",
  "FP&A", "ngân sách", "P&L", "dự báo", "phân tích độ nhạy", "kịch bản base/optimistic/pessimistic",
  "mô phỏng Monte Carlo", "danh mục dự án / project portfolio", "quản lý tiến độ - nguồn lực - chi phí",
  "quản lý tồn kho / tối ưu đặt hàng", "dashboard KPI", "bảng kiểm tra tài liệu". Không dùng khi chỉ cần
  một bảng đơn lẻ hoặc chỉ cần làm đẹp một bảng đã có.
apps: [et]
---

# Mô hình nhiều sheet

Mỗi quy tắc dưới đây là một lỗi **đã đo được** trên chính mô hình do agent dựng, không phải lời khuyên
chung. Số liệu ở `tests/bench/results/`.

## Quy tắc

1. **Dựng ĐỦ sheet đề yêu cầu, và làm xong một nhánh trước khi nhân ra.** Đo được: một lượt làm xong 4
   sheet dữ liệu rồi hết ngân sách, để **9/13 sheet đề yêu cầu TRỐNG HOÀN TOÀN** — mà phần bị chấm điểm
   lại nằm ở cuối danh sách (`test3-gaps.md`). Đừng làm hết dữ liệu rồi mới tới phân tích; dựng một
   nhánh dọc cho xong (dữ liệu → công thức → dashboard/checks) rồi mới nhân ra. Đối chiếu bằng
   `et.listSheets` với danh sách đề bài, và đọc lại vài ô của **từng** sheet để chắc nó không rỗng.

2. **Ô giả định phải có GIÁ TRỊ trước khi công thức đọc nó.** Công thức trỏ vào ô trống **không báo
   lỗi** — Calc coi ô trống là 0, nên mô hình vẫn ra số và không ai biết nó đang chạy bằng 0. Đo được:
   một workbook có **60.006 công thức** đọc `Inputs.C25` trống, trong khi ô seed được dán nhãn
   "Seed Value" nằm ở `B22` và **không công thức nào đọc** (`test4-dung-sai.md`). Ghi giả định bằng
   một `et.writeRange` có cả giá trị, rồi `et.checkRange` — `empty-reference` phải bằng 0 trước khi
   coi là xong.

3. **Giả định nằm ở MỘT sheet, công thức chỉ đọc từ đó** — không gõ lại số vào công thức, không tính
   tay rồi ghi kết quả. Đổi một giả định ở sheet `Assumptions` mà không phải sửa chỗ nào khác.

4. **Sheet kiểm tra phải kiểm ĐÚNG ô điều khiển.** Đo được: một check so ô `C22` (chữ `"integer"`) với
   một con số nên **PASS oan**. Mỗi dòng check: `=IF(<ô cần kiểm> = <giá trị mong đợi>, "PASS", "FAIL")`
   — so giá trị, không so nhãn; và `PASS/FAIL` phải là công thức, không phải chữ gõ sẵn.

5. **Phép thử duy nhất phân biệt ô điều khiển SỐNG với ô CHẾT là đổi rồi đọc lại.** Đọc không phân
   biệt được. Trước khi trả lời người dùng: đổi một giả định (ví dụ seed/giá), `et.readRange` các ô
   phải phụ thuộc (KPI, tổng, kết quả check), xác nhận chúng **đổi thật**, rồi trả lại giá trị cũ.
   Nếu không đổi thì ô điều khiển chết — sửa công thức trước khi báo xong.

6. **Ngưỡng và cảnh báo dùng `et.setConditionalFormat`**, đừng tô màu tay từng ô theo số đã biết: tô
   tay đứng yên khi dữ liệu đổi. Rule nhận `operator` (`less`, `lessEqual`, `greater`, `greaterEqual`,
   `equal`, `notEqual`, `between`, `notBetween`, `formula`), `formula1`/`formula2`, và `styleName` có
   sẵn (`Good`, `Bad`, `Neutral`, `Warning`, `Error`) hoặc `fillColor`/`fontColor`/`bold`. Nhiều rule
   trên cùng một vùng thì để trong **một** lần gọi; gọi lại trên cùng vùng là **thay** rule cũ.

7. **Bảng lớn đi bằng công thức + điền**: `et.fillRange` viết một công thức vào ô góc rồi điền ra cả
   vùng (tham chiếu tương đối tự dịch) — đừng gửi từng ô. Dữ liệu thô gửi **một lần** bằng mảng hai
   chiều đầy đủ.

8. **Tự kiểm bằng số, không bằng cấu trúc.** Sau khi dựng, chọn một dòng đại diện và **tự tính tay
   theo công thức** rồi so với giá trị đọc lại (ví dụ `I=H*C`, `J=H-I-D`, `K=J-Investment/duration`).
   Đếm được bao nhiêu sheet/công thức không nói lên mô hình có đúng.

9. **Lập kế hoạch MỘT lần trước khi viết, rồi giữ nguyên khi làm (plan-then-fill).** Đo được:
   Test 1 đốt 18/25 vòng đầu vào thí nghiệm công thức nhỏ (`DATE`, `EOMONTH`, dấu `,`/`;`) trước
   khi ghi dòng đầu tiên — tất cả đều có sẵn trong mô tả tool (`test1-gaps.md`). Trước lệnh đầu
   tiên: liệt kê sheet cần dựng, vùng nguồn, công thức nào đi bằng `fillRange` ở đâu, output nào để
   pass; viết gọn trong một lượt suy nghĩ rồi làm theo. Chỉ lập lại kế hoạch khi điều kiện đầu vào
   sai hoặc `checkRange` báo lỗi — không xét lại toàn bộ sau mỗi lần ghi thành công. Tin mô tả
   `office_action`; gọi nhầm tên lệnh thì đọc danh sách trả về ngay trong lỗi, không thử biến thể.

10. **Đọc để làm tiếp, không đọc để yên tâm.** Đo được: lượt Test 4 rộng gọi `et.readRange` 174 lần
    — mỗi lần kéo cả bảng về là input phình mà không thêm quyết định mới. Đọc 1–2 vùng mục tiêu
    trước khi sửa; sau khi sửa chỉ đọc lại vùng vừa sửa + `et.checkRange`; không đọc lại vùng chưa
    đổi. Vùng lớn (hơn vài trăm dòng) thì đọc mẫu đầu/cuối + đếm kích thước, không kéo hết 50.001
    dòng về.

## Checklist trước khi trả lời

- [ ] Danh sách sheet đề yêu cầu ↔ `et.listSheets`: đủ, không sheet nào rỗng.
- [ ] `et.checkRange`: `issueCount` = 0, `empty-reference` = 0.
- [ ] Không số ghi cứng cho giá trị tính được; công thức đọc từ sheet giả định.
- [ ] `et.setConditionalFormat` cho các ngưỡng/cảnh báo; không tô tay.
- [ ] Đã đổi một giả định → ô phụ thuộc đổi thật → trả lại.
- [ ] Đọc lại vài ô đại diện và tự tính tay một dòng để đối chiếu.
- [ ] Đã viết plan một lần (sheet → nguồn → fill → check) trước lệnh đầu tiên; không probe quá 2 call.
- [ ] Không đọc lại vùng chưa đổi; vùng lớn chỉ đọc mẫu đầu/cuối + kích thước.
