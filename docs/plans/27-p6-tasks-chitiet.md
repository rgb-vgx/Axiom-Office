# 27 / P6 - Task chi tiết (Audit và an toàn dữ liệu)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `07-p6` trước.
> Đích: tra được ai sửa gì, khi nào; không lộ bí mật trong log; khôi phục được bản trước AI.

## P6-00. Đã có

- [x] Audit: `tool_calls` + `runs` trong `core.db` (`store/runs.go` `AddToolCall`, `Audit`; chèn ở
      `agent/orchestrator.go:400`); `GET /v1/audit?runId=…` (`api/runs.go`).
- [x] Khung chat "⋯" → Nhật ký hoạt động.
- [x] Làn tệp ghi atomic: `core-go/internal/filesafe/filesafe.go`.
- [x] Hoàn tác Word cả lượt: xem `16-p1`.

## P6-01. Ẩn bí mật trong audit

- [ ] Phụ thuộc `Redact` của M2 (`21-m2` M2-01) - làm M2 trước.
- [ ] Chèn redact ở **hai chỗ**: `agent/orchestrator.go` trước `o.Runs.AddToolCall(...)` (để `params_json` sạch) và
      trước khi publish `tool.finished` (`paramsPreview`).
- [ ] `store/runs.go` `AddToolCall`: giữ cắt ≤ 2KB như hiện tại; không ghi nội dung file.
- [ ] Test: chạy lượt có khóa API giả trong tham số → `tool_calls.params_json` không chứa khóa (đọc thẳng `core.db`).

## P6-02. Hạn lưu audit

- [ ] `store/runs.go` / `store/db.go`: hàm `PurgeAudit(before)` xóa `tool_calls` (và `runs` nếu cần) cũ hơn N ngày
      (mặc định 90).
- [ ] Chạy lúc Core khởi động + định kỳ; nút xóa trong admin (`src/AxiomOffice/Ai/AdminForms.cs`).
- [ ] Test: audit cũ hơn hạn bị dọn; lượt mới vẫn ghi đủ.

## P6-03. Khôi phục lượt từ bản chụp

- [ ] Bản chụp từ `16-p1` (Excel `et.restoreSnapshot`, PowerPoint) gắn `run_id`.
- [ ] `POST /v1/runs/{id}/revert` (`api/runs.go`): gọi lệnh khôi phục khi còn bản chụp; hết bản chụp trả lỗi rõ.
- [ ] Nút "Hoàn tác lượt này" dùng chung cho mọi app (`AskAiPane.UndoLastRun`).
- [ ] Không làm riêng "digest trước/sau" - không đủ để khôi phục.

## P6-04. Nhật ký hoạt động lọc theo tài liệu

- [ ] `GET /v1/audit` thêm lọc theo `documentKey` (join `conversations.document_key`) hoặc lọc theo `run_id` của
      hội thoại tài liệu đang mở.
- [ ] `AdminForms.cs`: mặc định lọc theo tài liệu đang mở, có ô bỏ lọc.

## P6-05. Kiểm thử + changelog

- [ ] `cd core-go && go test ./internal/store/... ./internal/api/...`.
- [ ] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `07-p6`.

## Không làm

- [ ] Không lưu toàn bộ tệp mỗi lượt khi tệp lớn; chỉ chụp vùng bị sửa.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/store/... ./internal/api/... ./internal/agent/...
```
