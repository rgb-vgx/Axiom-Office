# 24 / M5 - Task chi tiết (Củng cố và quên)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `14-m5` trước.
> Đích: DB không phình, và biết được memory có ích hay chỉ tốn token.

## M5-00. Đã có

- [x] Chống trùng lúc ghi: `Hash` + cosine 0.96 (`store.go` `AddBatch`, `findByHash`, `findNearDuplicate`).
- [x] Hạn dùng (`expires_at`), ghim (`pinned`), đếm lượt dùng (`hits`), xóa mềm + `PurgeSoftDeleted`, `Purge`.
- [x] `EntityCounts`, `MissingVectors` (dùng cho dọn/backfill embedding).

## M5-01. Dọn định kỳ (gộp + hạ điểm + hết hạn) - XONG (một phần)

- [x] `core-go/internal/memory/service.go` `RunCleanup` + `cmd/axiom-core/main.go`: job dọn chạy nền qua `Enqueue`
      sẵn có (chu kỳ `MemoryCleanupDays`, mặc định 7 ngày; 0 = tắt).
- [x] Gộp fact gần trùng **đã lọt** qua lúc ghi (`Store.mergeNearDuplicates`): cùng `scope`+`scope_key`, cùng text đã
      chuẩn hóa hoặc cosine ≥ `NearDuplicateCosine` → giữ bản cũ hơn, link bằng `memory_links`, xóa mềm lý do `merge`.
- [x] Xóa mềm fact hết hạn (`Store.softDeleteExpired`) và `PurgeSoftDeleted` theo lịch.
- [x] Test `TestCleanupMergesNearDuplicates` (200 fact trùng → còn < 100), `TestCleanupExpiresAndKeepsOrgStyle`.
- [ ] Còn: "hạ `confidence` fact lâu không dùng theo bậc" - chưa làm (rủi ro xóa fact đúng nhưng ít dùng; chờ số đo
      M5-03 rồi mới quyết ngưỡng).

## M5-02. Giữ theo scope - XONG (một phần)

- [ ] `document`: tệp không còn trong danh sách tài liệu đã mở → **đề xuất** xóa qua thẻ `Choose` (còn).
- [x] `user`: giữ lâu (không nằm trong job hết hạn/`softDeleteExpired`).
- [x] `org-style`: không tự hết hạn, không nằm trong job dọn (`mergeNearDuplicates`/`softDeleteExpired` bỏ qua).

## M5-03. Số đo memory có ích hay rác

- [ ] Mỗi lượt: log (`core-go/internal/corelog/corelog.go`) số fact chèn vào prompt (từ `BuildContext`) và số fact
      được model nhắc tới trong câu trả lời/tool call (heuristic trùng từ khóa).
- [ ] Tổng hợp theo tuần: số fact trùng, hết hạn, bị thay thế, tỉ lệ fact được dùng; xuất cùng bảng R0 (`18-r0`).
- [ ] Không dùng LLM khác để chấm.

## M5-04. Hiệu năng + tài liệu hóa

- [ ] Truy hồi < 200 ms với 10.000 fact trên máy văn phòng thường (test hiệu năng trong `internal/memory/`).
- [ ] Tài liệu hóa "khi nào nhớ, khi nào quên, khi nào hỏi lại người dùng" vào `04-p3`; ghi `CHANGELOG.md
      [Unreleased]`.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/memory/...
```
