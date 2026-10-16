# 07 / P6 - Audit và an toàn dữ liệu

> Tra được ai sửa gì, khi nào. Không lộ bí mật trong log. Khôi phục được bản trước AI.

## 1. Đã có (đối chiếu code 16/10/2026)

- Audit từng lệnh: bảng `tool_calls` (`run_id`, `seq`, `tool`, `action`, `params_json`, `ok`, `error`, `ms`,
  `created_at`) và `runs` (model, số vòng, token, lỗi) trong `core.db`; `GET /v1/audit?runId=…`.
- Khung chat: "⋯" → Nhật ký hoạt động.
- Làn tệp ghi atomic: `core-go/internal/filesafe/filesafe.go`.
- Hoàn tác Word cả lượt (xem `02-p1`).

## 2. Còn thiếu - việc cần làm

- **Ẩn bí mật trong audit**: `params_json` đang lưu thô (có thể chứa khóa API trong lệnh MCP, nội dung nhạy cảm
  trong `replaceMany`). Lọc như M2 (`memory/redact.go` dùng chung) trước khi ghi; cắt chuỗi dài.
- **Hạn lưu audit**: `tool_calls` hiện giữ mãi; thêm giữ N ngày (mặc định 90) + nút xóa trong admin.
- **Khôi phục**: bản chụp từ `02-p1` (Excel/PowerPoint) gắn với `run_id`; `POST /v1/runs/{id}/revert` khi còn
  bản chụp. Digest trước/sau không đủ để khôi phục nên không làm riêng.
- **Lịch sử theo tài liệu**: Nhật ký hoạt động lọc theo tài liệu đang mở.

## 3. Tệp sẽ sửa

- `core-go/internal/store/runs.go`, `db.go`
- `core-go/internal/api/runs.go`
- `src/AxiomOffice/Ai/AdminForms.cs` (nhật ký hoạt động)

## 4. Tiêu chí xong

- Chạy lượt có khóa API giả trong tham số → `tool_calls.params_json` không chứa khóa (đọc thẳng `core.db`).
- Audit cũ hơn hạn bị dọn; lượt mới vẫn ghi đủ.
- Revert một lượt Excel có bản chụp → dữ liệu giống trước lượt.

## 5. Kiểm thử

- `cd core-go && go test ./internal/store/... ./internal/api/...`

## 6. Không làm

- Không lưu toàn bộ tệp mỗi lượt khi tệp lớn; chỉ chụp vùng bị sửa.
