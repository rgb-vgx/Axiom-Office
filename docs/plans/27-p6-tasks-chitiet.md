# 27 / P6 - Task chi tiết (Audit và an toàn dữ liệu)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `07-p6` trước.
> Đích: tra được ai sửa gì, khi nào; không lộ bí mật trong log; khôi phục được bản trước AI.

## P6-00. Đã có

- [x] Audit: `tool_calls` + `runs` trong `core.db` (`store/runs.go` `AddToolCall`, `Audit`; chèn ở
      `agent/orchestrator.go:400`); `GET /v1/audit?runId=…` (`api/runs.go`).
- [x] Khung chat "⋯" → Nhật ký hoạt động.
- [x] Làn tệp ghi atomic: `core-go/internal/filesafe/filesafe.go`.
- [x] Hoàn tác Word cả lượt: xem `16-p1`.

## P6-01. Ẩn bí mật trong audit - XONG

- [x] Phụ thuộc `Redact` của M2 (`21-m2` M2-01) - đã có.
- [x] Chèn redact ở **hai chỗ**: `agent/orchestrator.go` `redactedParams` bọc `o.Runs.AddToolCall(...)` (`params_json`
      sạch) và `paramsPreview` trong `tool.started`/`tool.finished`.
- [x] `store/runs.go` `AddToolCall`: giữ cắt ≤ 2KB như hiện tại; không ghi nội dung file.
- [x] Test `TestRedactedParamsHidesSecrets` (`agent/audit_redact_test.go`). Còn: đọc thẳng `core.db` trong một lượt
      thật làm cùng bộ đo R0.

## P6-02. Hạn lưu audit - XONG (Go)

- [x] `store/runs.go` `PurgeAudit(before)` xóa `tool_calls` cũ hơn mốc; cấu hình `AuditKeepDays` (env
      `AXIOM_AUDIT_KEEP_DAYS`, mặc định 90) trong `config.go`.
- [x] Chạy lúc Core khởi động + **hằng ngày** (`cmd/axiom-core/main.go`, khối `stores.Available`); `AuditKeepDays = 0`
      thì dừng vòng lặp.
- [ ] Còn: nút xóa trong admin (`src/AxiomOffice/Ai/AdminForms.cs`).
- [x] Test `TestPurgeAuditKeepsRecent`: audit cũ hơn hạn bị dọn; lượt mới vẫn ghi đủ.

## P6-03. Khôi phục lượt từ bản chụp

- [ ] Bản chụp từ `16-p1` (Excel `et.restoreSnapshot`, PowerPoint) gắn `run_id`.
- [ ] `POST /v1/runs/{id}/revert` (`api/runs.go`): gọi lệnh khôi phục khi còn bản chụp; hết bản chụp trả lỗi rõ.
- [ ] Nút "Hoàn tác lượt này" dùng chung cho mọi app (`AskAiPane.UndoLastRun`).
- [ ] Không làm riêng "digest trước/sau" - không đủ để khôi phục.

## P6-04. Nhật ký hoạt động lọc theo tài liệu - XONG (Core)

- [x] `GET /v1/audit?documentKey=…` (`Runs.AuditForDocument`, join `runs` → `conversations.document_key`).
- [ ] Còn: `AdminForms.cs` mặc định lọc theo tài liệu đang mở, có ô bỏ lọc.

## P6-05. Kiểm thử + changelog

- [x] `cd core-go && go test ./internal/store/... ./internal/api/...`.
- [x] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `07-p6`.

## Không làm

- [ ] Không lưu toàn bộ tệp mỗi lượt khi tệp lớn; chỉ chụp vùng bị sửa.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/store/... ./internal/api/... ./internal/agent/...
```
