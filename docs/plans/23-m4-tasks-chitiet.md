# 23 / M4 - Task chi tiết (Quản lý và kiểm duyệt memory)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `13-m4` trước.
> Đích: người dùng tự dọn memory sai mà không phải xóa DB.

## M4-00. Quản lý cơ bản - ĐÃ CÓ

- [x] API `core-go/internal/api/memory.go`: `GET /v1/memory`, `POST`, `PATCH /v1/memory/{id}`,
      `DELETE /v1/memory/{id}` (xóa mềm), `POST /v1/memory/{id}/restore`, `GET /v1/memory/{id}/history`,
      `DELETE /v1/memory?scope=…&confirm=true`.
- [x] `Store.Update/Delete/Restore/History/Purge` (`internal/memory/store.go`); lịch sử `memory_history`.
- [x] UI `src/AxiomOffice/Ai/MemoryForm.cs` (xem, sửa, ghim, hạn dùng, xóa); pane hiện "Đã ghi nhớ: …" kèm link Xóa.

## M4-01. Export / import JSON - XONG (Core)

- [x] API `GET /v1/memory/export` (`{count, items}`) và `POST /v1/memory/import`; giữ `id`, `pinned`, `expires_at`,
      `hits`; embedding tính lại sau khi import (`service.QueueEmbeddingBackfill`).
- [x] Chống trùng khi import: `findByHash` (`store.go`), trả `added`/`skipped`/`links`.
- [x] Test `TestExportImportRoundTrip` (DB trống → import → cùng số mục, ghim/hạn dùng, tìm bằng keyword) và
      `TestMemoryExportImportAPI`.
- [ ] Còn: nút Export/Import trong `MemoryForm.cs`.

## M4-02. Xóa theo tệp - XONG (Core)

- [x] `DELETE /v1/memory?documentKey=…&confirm=true` (`Store.SoftDeleteByScopeKey` - xóa mềm theo `scope_key`).
- [x] Test `TestSoftDeleteByScopeKey`, `TestMemoryDeleteByDocumentAPI`.
- [ ] Còn: nút "Quên mọi thứ về tài liệu này" trong `MemoryForm.cs` khi đang mở tài liệu.

## M4-03. Lịch sử sửa trong UI

- [ ] `MemoryForm.cs`: hiện `GET /v1/memory/{id}/history` của mục đang chọn (actor, run, lý do, trước/sau).
- [ ] Chữ mới vào `src/AxiomOffice/lang/en.json`.

## M4-04. Tắt memory theo scope - XONG (Core)

- [x] `core-go/internal/config/config.go`: `MemoryDocumentEnabled`, `MemoryUserEnabled` (env
      `AXIOM_MEMORY_DOCUMENT_ENABLED`, `AXIOM_MEMORY_USER_ENABLED`).
- [x] `Service.Context` → `Retriever.BuildContext(..., ScopeFilter{...})`: scope bị tắt không vào context, **không**
      xóa dữ liệu; đọc lại mỗi lượt nên có hiệu lực ngay.
- [x] Test `TestScopeFilterDisablesScopes`.
- [ ] Còn: hai ô trong Cài đặt nâng cao (`SettingsForm.cs`).

## M4-05. Hồi quy + changelog

- [x] `cd core-go && go test ./internal/api/... ./internal/memory/...`.
- [x] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `13-m4`.

## Tiêu chí xong

- [ ] Xóa một fact sai → lượt sau AI không dùng lại (kiểm bằng `recall` và context).
- [ ] Xóa theo tệp → mở tệp đó như mới.
- [ ] Export → import sang `core.db` trống → cùng số mục, cùng ghim/hạn dùng, tìm được bằng keyword.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/api/... ./internal/memory/...
cd ..; python scripts/check_i18n.py
```
