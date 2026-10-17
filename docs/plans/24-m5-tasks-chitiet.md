# 24 / M5 - Task chi tiết (Củng cố và quên)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `14-m5` trước.
> Đích: DB không phình, và biết được memory có ích hay chỉ tốn token.

## M5-00. Đã có

- [x] Chống trùng lúc ghi: `Hash` + cosine 0.96 (`store.go` `AddBatch`, `findByHash`, `findNearDuplicate`).
- [x] Hạn dùng (`expires_at`), ghim (`pinned`), đếm lượt dùng (`hits`), xóa mềm + `PurgeSoftDeleted`, `Purge`.
- [x] `EntityCounts`, `MissingVectors` (dùng cho dọn/backfill embedding).

## M5-01. Dọn định kỳ (gộp + hạ điểm + hết hạn)

- [ ] `core-go/internal/memory/service.go`: thêm job dọn chạy nền (dùng `Service.Enqueue` sẵn có), không chạy khi
      đang bận lượt.
- [ ] Gộp fact gần trùng **đã lọt** qua lúc ghi: so cặp trong cùng `scope`+`scope_key`, cosine ≥ ngưỡng thì gộp
      (giữ fact cũ hơn/`pinned`, liên kết bằng `memory_links`, `Store.Update`/`Delete` lý do `merge`).
- [ ] Hạ `confidence` fact lâu không dùng: `hits` không tăng trong N ngày → giảm bậc; dưới ngưỡng thì xóa mềm.
- [ ] Xóa mềm fact hết hạn: gọi `PurgeSoftDeleted` theo lịch (mặc định hằng tuần).
- [ ] Test: seed 1.000 fact trùng lặp → sau một lần dọn còn < 200; kịch bản `recall` trong test vẫn ra đúng.

## M5-02. Giữ theo scope

- [ ] `document`: tệp không còn trong danh sách tài liệu đã mở → **đề xuất** xóa (không tự xóa; hỏi qua thẻ `Choose`).
- [ ] `user`: giữ lâu, chỉ hạ điểm theo M5-01.
- [ ] `org-style`: không tự hết hạn, không nằm trong job dọn (xem `22-m3`).

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
