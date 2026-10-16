# 16 / P1 - Task chi tiết (Duyệt và hoàn tác được)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 16/10/2026; đọc `02-p1` trước.

## P1-01. Chính sách hỏi trước - ĐÃ CÓ

- [x] `core-go/internal/policy/policy.go`: `writer.replaceAll`/`writer.replaceMany` trên tài liệu
      > `LargeDocumentChars` thì hỏi; lưu/xuất hỏi trừ khi người dùng đã nhờ (`PromptAsksToSave`).
- [x] `core-go/internal/tools/overwrite.go`: ghi đè ô có dữ liệu thì hỏi, sheet do lượt tạo (`CreatedSheets`) miễn.
- [x] `office_action.go`: từ chối find một dấu câu lẻ (`loneSymbolFind`).
- [ ] Chỉ còn: rà lại danh sách lệnh ghi mới (từ 13/10) có cần thêm vào policy không.

## P1-02. Thẻ xác nhận / nhiều lựa chọn - ĐÃ CÓ

- [x] `core-go/internal/agent/confirm.go`: `Request`, `Choose`, `ResolveAnswer` (ô tự gõ); hết giờ/hủy trả rỗng.
- [x] `POST /v1/runs/{id}/confirm` nhận `approved` hoặc `choice` + `text`; khung chat Windows và WPS hiện thẻ.
- [x] `Interactive=false` → từ chối lệnh cần hỏi, không treo.

## P1-03. Chụp vùng Excel trước khi ghi (C#)

- [ ] Tệp: `src/AxiomOffice/Bridge/CommandDispatcher.Spreadsheet.cs`.
- [ ] Trước mỗi lệnh ghi `et.*`: lưu giá trị + công thức + định dạng số của vùng sắp ghi vào bộ nhớ của bridge,
      gắn theo run (Core gửi `runId` trong params hoặc header).
- [ ] Giới hạn: vùng > N ô (vd 200.000) thì không chụp, báo Core để hỏi người dùng trước khi ghi.
- [ ] Lệnh `et.restoreSnapshot {runId}` khôi phục theo thứ tự ngược.
- [ ] Thử: bảng điểm 200 dòng, AI sửa, khôi phục, so `et.readRange formulas=true` với trước lượt.

## P1-04. PowerPoint: kiểm hoàn tác thật

- [ ] Thử Ctrl+Z sau lệnh ghi `wpp.*` qua COM; không được thì chụp slide bị sửa (duplicate ẩn hoặc lưu shape).

## P1-05. "Hoàn tác lượt này" cho mọi app

- [ ] Tệp: `src/AxiomOffice/Ai/AskAiPane.cs` (`UndoLastRun`): Word giữ `writer.undo {count}`; Excel gọi
      `et.restoreSnapshot`; PowerPoint theo P1-04.
- [ ] Nút chỉ hiện khi lượt có sửa và có cách hoàn tác.

## P1-06. Danh sách thay đổi của lượt

- [ ] Bridge trả `changes` rút gọn cho lệnh ghi (vùng/đoạn/slide, trước/sau ngắn).
- [ ] Core: gom vào sự kiện SSE `run.changes` (`core-go/internal/agent/run.go`, `events.go`); giữ `Protocol = 1`.
- [ ] Khung chat: sau lượt hiện danh sách, bấm một mục thì chọn vùng/đoạn đó trong tài liệu.
- [ ] Test: `go test ./internal/agent/ -run TestRunJSONShape -v`.

## P1-07. Rà bọc hoàn tác Word

- [ ] Liệt kê mọi lệnh ghi `writer.*` trong catalog, kiểm có `UndoRecordScope` (hiện 17 chỗ trong
      `src/AxiomOffice/Bridge/`); thiếu thì thêm.
- [ ] `doc_design` (restyle + insertBlocks) hoàn tác bằng số bước biết trước.

## P1-08. Hồi quy + changelog

- [ ] `cd core-go && go test ./...`; kịch bản hoàn tác trong bộ `17`.
- [ ] Ghi `CHANGELOG.md [Unreleased]`, cập nhật `STATUS.md`.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/policy/... ./internal/tools/... ./internal/agent/...
cd ..; powershell -ExecutionPolicy Bypass -File scripts/build.ps1   # đóng Office trước
```
