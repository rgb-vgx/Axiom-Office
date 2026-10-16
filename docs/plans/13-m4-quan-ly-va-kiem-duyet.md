# 13 / M4 - Quản lý và kiểm duyệt memory

> Người dùng tự dọn được memory sai mà không cần xóa DB.

## 1. Đã có (đối chiếu code 16/10/2026)

- API `core-go/internal/api/memory.go`: `GET /v1/memory` (lọc), `POST`, `PATCH /v1/memory/{id}`,
  `DELETE /v1/memory/{id}` (xóa mềm), `POST /v1/memory/{id}/restore`, `GET /v1/memory/{id}/history`,
  `DELETE /v1/memory?scope=…&confirm=true` (xóa hàng loạt).
- Lịch sử sửa: bảng `memory_history` (actor, run, lý do).
- UI `src/AxiomOffice/Ai/MemoryForm.cs` (nút "Bộ nhớ AI…" trên ribbon): xem, sửa, ghim, đặt hạn dùng, xóa.
- Pane hiện dòng "Đã ghi nhớ: …" kèm link Xóa ngay sau lượt.

## 2. Còn thiếu - việc cần làm

- **Export/import JSON** để chuyển máy: `POST /v1/memory/export`, `POST /v1/memory/import` (giữ id, ghim, hạn dùng;
  embedding tính lại sau khi import).
- **Xóa theo tệp**: `DELETE /v1/memory?documentKey=…` + nút trong MemoryForm ("Quên mọi thứ về tài liệu này").
- **Lịch sử trong UI**: MemoryForm hiện `memory_history` của từng mục.
- **Tắt theo scope**: tắt riêng memory tài liệu hoặc cá nhân, có hiệu lực ngay (đọc lại cấu hình mỗi lần như
  `MemoryEnabled`).

## 3. Tệp sẽ sửa

- `core-go/internal/api/memory.go`, `core-go/internal/memory/store.go`
- `src/AxiomOffice/Ai/MemoryForm.cs`, `CoreAdmin.cs`

## 4. Tiêu chí xong

- Xóa một fact sai → lượt sau AI không dùng lại (kiểm bằng `recall` và context).
- Xóa theo tệp → mở tệp đó như mới.
- Export → import sang `core.db` trống → cùng số mục, cùng ghim/hạn dùng, tìm được bằng keyword.

## 5. Kiểm thử

- `cd core-go && go test ./internal/api/... ./internal/memory/...` (thêm test round-trip export/import).

## 6. Không làm

- Không phân quyền phức tạp hơn Admin/User, không đồng bộ máy chủ.
