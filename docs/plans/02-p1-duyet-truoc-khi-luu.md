# 02 / P1 - Duyệt và hoàn tác được

> Không còn ghi đè âm thầm. Người dùng xem lại được việc AI làm và quay lại trạng thái trước lượt đó.

## 1. Mục tiêu

- Sửa xong thì cho **xem lại** từng thay đổi và **hoàn tác cả lượt** ở mọi app, kể cả Excel.
- Mọi ghi đè dữ liệu có sẵn đều hỏi trước.

Đổi hướng so với bản cũ: bỏ chế độ "preview trả diff mà không thực hiện". Với lệnh định dạng, model không biết
kết quả trông ra sao nếu chưa chạy, nên mỗi lệnh sẽ cần một chế độ chạy thử. Thực tế hơn là: chạy thật → cho xem
lại → hoàn tác được.

## 2. Đã có (đối chiếu code 16/10/2026)

- Hỏi trước khi chạy:
  - `core-go/internal/policy/policy.go`: `writer.replaceAll` **và** `writer.replaceMany` trên tài liệu
    > `LargeDocumentChars` (20.000) thì hỏi; lưu/xuất thì hỏi trừ khi người dùng đã nhờ.
  - `core-go/internal/tools/overwrite.go`: ghi vào ô có dữ liệu khác thì hỏi; sheet do chính lượt tạo
    (`CreatedSheets`) thì miễn; đã đồng ý một lần cho sheet thì không hỏi lại trong lượt.
  - `office_action.go`: từ chối find chỉ một dấu câu lẻ (`loneSymbolFind`).
- Thẻ hỏi: `core-go/internal/agent/confirm.go`: `Request` (Đồng ý/Từ chối), `Choose` (nhiều lựa chọn, có ô tự gõ),
  hết giờ/hủy trả rỗng. Khung chat Windows và WPS hiện thẻ. `Interactive=false` (agent khác gọi `ai.ask`) thì từ chối.
- Chỉ đọc: chế độ lập kế hoạch (`request.Plan`, tool chỉ đọc, `ReadOnlyMCP`).
- Hoàn tác Word: lệnh ghi bọc `UndoRecordScope` (17 chỗ trong `src/AxiomOffice/Bridge/`); nút "Hoàn tác lượt này"
  trong khung chat gọi `writer.undo {count}`.

## 3. Còn thiếu - việc cần làm

### 3.1 Excel: chụp vùng trước khi ghi

Ghi qua COM **xóa sạch lịch sử Undo của Excel**, nên Ctrl+Z không cứu được. Cần:

- Trước mỗi lệnh ghi `et.*`: chụp giá trị + công thức + định dạng số của vùng sắp ghi (giới hạn kích thước,
  vùng quá lớn thì chụp bản sao tệp).
- Lệnh `et.restoreSnapshot {runId}` khôi phục; nút "Hoàn tác lượt này" dùng cho cả Excel.
- PowerPoint: kiểm lại hoàn tác thật (COM có thể cũng phá Undo); không được thì chụp slide bị sửa.

### 3.2 Xem lại thay đổi của lượt

- Bridge trả "trước/sau" rút gọn cho lệnh ghi (vùng, đoạn, slide bị đổi); Core gom thành sự kiện SSE `run.changes`.
- Khung chat: danh sách thay đổi sau lượt (bấm để chọn vùng/đoạn trong tài liệu), nút hoàn tác cả lượt.

### 3.3 Kiểm hoàn tác Word đủ chưa

- Rà mọi lệnh ghi `writer.*` có bọc `UndoRecordScope` chưa (đếm theo catalog, không theo cảm giác).
- `writer.restyle` + `writer.insertBlocks` (doc_design) hoàn tác bằng một số bước biết trước.

## 4. Tệp sẽ sửa

- `src/AxiomOffice/Bridge/CommandDispatcher.Spreadsheet.cs` (+ lệnh snapshot), `.Presentation.cs`
- `core-go/internal/agent/run.go`, `events.go`, `core-go/internal/api/runs.go`
- `src/AxiomOffice/Ai/AskAiPane.cs`, `PaneControls.cs`
- WPS/LibreOffice: stub báo rõ trước, làm thật sau

## 5. Tiêu chí xong

- Bảng điểm 200 dòng: AI sửa xong, bấm "Hoàn tác lượt này" → giá trị và công thức giống hệt trước lượt (so bằng
  `et.readRange formulas=true`), 5/5 lần.
- Word: sau lượt `doc_design`, hoàn tác cả lượt → `writer.getText` giống trước lượt.
- Không có lệnh ghi nào trong catalog thiếu bọc hoàn tác (Word) hoặc thiếu chụp vùng (Excel).

## 6. Kiểm thử

- `cd core-go && go test ./internal/policy/... ./internal/tools/... ./internal/agent/...`
- Test tay trên Word/Excel/PowerPoint thật, đóng tài liệu không lưu sau khi thử.

## 7. Không làm

- Không tự động lưu tệp; không bỏ xác nhận cho agent ngoài (`Interactive=false` vẫn từ chối lệnh cần hỏi).
