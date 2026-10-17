# 23 / M4 - Task chi tiết (Quản lý và kiểm duyệt memory)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `13-m4` trước.
> Đích: người dùng tự dọn memory sai mà không phải xóa DB.

## M4-00. Quản lý cơ bản - ĐÃ CÓ

- [x] API `core-go/internal/api/memory.go`: `GET /v1/memory`, `POST`, `PATCH /v1/memory/{id}`,
      `DELETE /v1/memory/{id}` (xóa mềm), `POST /v1/memory/{id}/restore`, `GET /v1/memory/{id}/history`,
      `DELETE /v1/memory?scope=…&confirm=true`.
- [x] `Store.Update/Delete/Restore/History/Purge` (`internal/memory/store.go`); lịch sử `memory_history`.
- [x] UI `src/AxiomOffice/Ai/MemoryForm.cs` (xem, sửa, ghim, hạn dùng, xóa); pane hiện "Đã ghi nhớ: …" kèm link Xóa.

## M4-01. Export / import JSON

- [ ] API mới `GET /v1/memory/export` (trả `{items, links, history}` hoặc chỉ `{items, links}`) và
      `POST /v1/memory/import`; giữ `id`, `pinned`, `expires_at`; embedding tính lại sau khi import
      (`service.QueueEmbeddingBackfill`).
- [ ] Chống trùng khi import: dùng `findByHash`/`findNearDuplicate` sẵn có (`store.go`), báo số mục bỏ qua.
- [ ] Test round-trip: DB trống → import → cùng số mục, ghim/hạn dùng, tìm được bằng keyword.

## M4-02. Xóa theo tệp

- [ ] `DELETE /v1/memory?documentKey=…&confirm=true` trong `internal/api/memory.go` (mở rộng nhánh `scope=…` sẵn
      có); xóa mềm theo `scope_key`.
- [ ] `src/AxiomOffice/Ai/MemoryForm.cs`: nút "Quên mọi thứ về tài liệu này" khi đang mở tài liệu.
- [ ] Test `TestPurgeByDocumentKey`.

## M4-03. Lịch sử sửa trong UI

- [ ] `MemoryForm.cs`: hiện `GET /v1/memory/{id}/history` của mục đang chọn (actor, run, lý do, trước/sau).
- [ ] Chữ mới vào `src/AxiomOffice/lang/en.json`.

## M4-04. Tắt memory theo scope

- [ ] `core-go/internal/config/config.go`: cờ tắt riêng memory tài liệu / cá nhân (đọc lại mỗi lượt như
      `MemoryEnabled` - xem `service.go` `config()`, `Enabled()`).
- [ ] `service.Context`/`BuildContext`: scope bị tắt thì không đưa vào context, nhưng **không** xóa dữ liệu; có hiệu
      lực ngay không cần khởi động lại.
- [ ] UI: hai ô trong Cài đặt nâng cao (`SettingsForm.cs`) hoặc `MemoryForm`.

## M4-05. Hồi quy + changelog

- [ ] `cd core-go && go test ./internal/api/... ./internal/memory/...`.
- [ ] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `13-m4`.

## Tiêu chí xong

- [ ] Xóa một fact sai → lượt sau AI không dùng lại (kiểm bằng `recall` và context).
- [ ] Xóa theo tệp → mở tệp đó như mới.
- [ ] Export → import sang `core.db` trống → cùng số mục, cùng ghim/hạn dùng, tìm được bằng keyword.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/api/... ./internal/memory/...
cd ..; python scripts/check_i18n.py
```
