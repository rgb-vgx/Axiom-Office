# 17 / R0 - Độ tin cậy với model yếu và bộ đo

> Người dùng chính chạy model free yếu (mimo, deepseek flash). Mọi phase khác đo bằng bộ này.

## 1. Vì sao làm trước

Các lượt thật gần đây cho thấy vấn đề lớn nhất không phải thiếu tính năng mà là **model yếu lạc đường và đốt token**:

| Ngày | Lượt | Chuyện gì xảy ra |
|---|---|---|
| 14/10/2026 | sửa giáo án (deepseek) | 137 lần `writer.replaceAll`, 48 vòng, 1,48 triệu token → đã thêm `writer.replaceMany` |
| 15/10/2026 | "Làm toàn bộ" (deepseek) | 59 vòng, 2 triệu token, 20 phút đi xóa dấu "/" ma; xóa luôn dấu ngày tháng → đã sửa `getText` + chặn find ký tự lẻ |
| 15/10/2026 | thiết kế lại giáo án (muse-spark) | bị từ chối `replace` thì chèn hết bằng `before` → chữ lặp hai lần, tự thêm "45 phút" → đã chặn |

Mỗi lần đều sửa sau khi người dùng gặp. Thiếu một bộ đo chạy lại được để thấy trước.

## 2. Đã có (đối chiếu code 16/10/2026)

- Ghi lại mọi lượt: `runs` (model, vòng, token vào/ra, cache, lỗi) và `tool_calls` (từng lệnh, tham số, thời gian)
  trong `%LOCALAPPDATA%\AxiomOffice\core\core.db`; dòng tổng kết mỗi lượt trong `core.log`.
- Chặn: giới hạn 100 vòng, chặn lệnh y hệt chạy quá 4 lần, dừng khi lặp một lỗi (`MaxIdenticalFailures`), hạn mức
  theo tool (`tools/limits.go`), guard kiểm chứng, dừng khi Office đóng. **Không có trần token mặc định** (bỏ từ
  13/10/2026 theo yêu cầu người dùng).
- `tests/bench/`: đối chiếu Claude Code với Axiom trên bài Calc (đọc transcript, so giá trị) - chưa phải bộ đo hồi quy.

## 3. Việc cần làm

### 3.1 Bộ kịch bản

- 10–15 kịch bản thật, mỗi kịch bản có: tệp nguồn (bản sao, không dùng tệp của người dùng), câu lệnh, app,
  **tiêu chí đạt đọc được bằng lệnh** (vd `et.readRange formulas=true` giống mốc, `writer.getText` không mất số
  liệu, `wpp.checkLayout` không lỗi, không chữ lặp).
- Phủ: giáo án Word (trình bày lại, sửa nội dung, tóm tắt), bảng điểm Excel (thêm cột công thức, định dạng), slide
  (dựng theo mẫu, sửa bố cục), memory (hai tệp không lẫn).

### 3.2 Bộ chạy

- Script chạy một kịch bản: mở bản sao trong app (như các script smoke đã dùng), gửi `POST /v1/runs` tới Core
  (`interactive=false`, thẻ chọn trả lời sẵn), chờ xong, chấm tiêu chí, đóng không lưu.
- Đọc số đo từ `core.db`: đạt/không, số vòng, token vào/ra, thời gian, số lệnh, lệnh lỗi, lệnh bị chặn.
- Mỗi kịch bản chạy N lần (mặc định 3) với mỗi model trong danh sách (mimo, deepseek flash, một model mạnh làm mốc).
- Xuất bảng so với lần chạy trước (CSV/Markdown); không cần dashboard.

### 3.3 Chặn đốt token (dùng số đo để chỉnh)

- Phát hiện vòng lặp "làm cùng một việc khác tham số" (vd 40 lệnh `replaceAll` một ký tự) chứ không chỉ "y hệt".
- Cảnh báo người dùng khi lượt vượt ngưỡng vòng/token (ví dụ 30 vòng hoặc 500k token): hỏi tiếp hay dừng (dùng
  thẻ `Choose`), thay vì im lặng chạy 20 phút.

## 4. Tiêu chí xong

- Bộ chạy được bằng một lệnh, ra bảng số đo cho mọi kịch bản × model.
- Ba sự cố ở mục 1 có kịch bản tái hiện; trên code hiện tại cả ba đạt.
- Không kịch bản nào với mimo/deepseek vượt 30 vòng hoặc 500k token mà không hỏi người dùng.

## 5. Không làm

- Không chấm bằng LLM khác (đắt, không ổn định); chỉ chấm bằng lệnh đọc tài liệu.
- Không chạy trên tệp thật của người dùng.
