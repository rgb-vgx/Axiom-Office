# 14 / M5 - Củng cố và quên

> DB không phình. Biết được memory có ích hay chỉ tốn token.

## 1. Đã có (đối chiếu code 16/10/2026)

- Chống trùng lúc ghi: hash + cosine 0.96 (`store.go`, `AddBatch`).
- Hạn dùng (`expires_at`), ghim (`pinned`), đếm lượt dùng (`hits`), xóa mềm.

## 2. Còn thiếu - việc cần làm

- **Dọn định kỳ** trong `service.go`: gộp fact gần trùng đã lọt qua lúc ghi, hạ `confidence` fact lâu không được
  dùng (`hits` không tăng), xóa mềm fact hết hạn.
- **Giữ theo scope**: `document` theo tệp (tệp không còn → đề xuất xóa), `user` lâu, `org-style` không tự hết hạn.
- **Số đo**: mỗi lượt ghi log số fact chèn vào prompt, số fact được model nhắc tới trong câu trả lời/tool call
  (heuristic trùng từ khóa); tổng hợp theo tuần.
- **Tài liệu hóa**: khi nào nhớ, khi nào quên, khi nào hỏi lại người dùng (cập nhật `04-p3`).

## 3. Tệp sẽ sửa

- `core-go/internal/memory/service.go`, `store.go`, `retriever.go`
- `core-go/internal/corelog/corelog.go`

## 4. Tiêu chí xong

- Seed 1.000 fact có trùng lặp → sau một lần dọn còn < 200 mà kịch bản `recall` trong test vẫn ra đúng.
- Truy hồi < 200 ms với 10.000 fact trên máy văn phòng thường.
- Có số đo tuần: số fact trùng, hết hạn, bị thay thế, tỉ lệ fact được dùng.

## 5. Kiểm thử

- `cd core-go && go test ./internal/memory/...` (+ test dọn, hết hạn, hiệu năng).

## 6. Không làm

- Không tự viết lại skill từ memory, không lịch dọn phức tạp.
