# 11 / M2 - Fact lâu dài: an toàn và ngân sách

> Hỏi việc tháng trước vẫn tìm được. Không lưu thứ không được lưu.

## 1. Đã có (đối chiếu code 16/10/2026)

- Truy hồi lai: bm25 (FTS5 bỏ dấu) + entity boost + cosine khi có embedding; không có embedding thì chỉ dùng
  keyword (`retriever.go`). Embedding lỗi không chặn lượt.
- Tool `recall` cho model tìm memory chủ động - **chính là `session_search` của bản kế hoạch cũ**, không viết thêm.
- Trích xuất trả `{text, category, scope, entities, confidence, expires_at}` (`prompts/extract.txt`).
- **Đã có lọc nhạy cảm**: `IsSensitive` (`core-go/internal/memory/text.go`) chặn 9–12 chữ số liền (CCCD/số tài
  khoản), số thẻ, `mật khẩu`/`password`/`PIN`/`OTP`, khóa API (`sk-`, `AIza`, `ghp_`…); bỏ fact ở cả
  `extractor.go` (`ParseFacts`) và `store.go` (`AddBatch`, `SKIPPED` lý do `sensitive`). Test
  `TestSensitiveAndValidDate`.

## 2. Còn thiếu - việc cần làm

- **`Redact` thay vì chỉ chặn**: hiện chỉ có hàm boolean bỏ hẳn fact; chưa có hàm trả về chuỗi đã thay bằng `[ẩn]`
  để dùng lại cho audit ở P6 (`07-p6`). Bổ sung mẫu còn thiếu: email, số điện thoại có dấu cách/chấm, token dạng
  JWT/`Bearer`.
- **Số liệu phải kèm nguồn**: fact có số mà không kèm tài liệu/vùng/thời điểm thì không lưu (sửa `extract.txt` +
  kiểm trong `extractor.go`).
- **Ngân sách token có log**: `BuildContext` (`retriever.go`) đang cắt im lặng; ghi log số fact đưa vào và số bị
  cắt để biết khi nào cần chỉnh.

## 3. Tệp sẽ sửa

- `core-go/internal/memory/text.go` (thêm `Redact`), `extractor.go`, `prompts/extract.txt`, `retriever.go`
- Không tạo file `redact.go` mới nếu `text.go` đủ chỗ.

## 4. Tiêu chí xong

- Seed 50 câu có số điện thoại/CCCD/token giả → không câu nào lọt vào `memories` (đọc thẳng `core.db`). (Phần
  CCCD/token đã có `IsSensitive`; còn email/số có dấu cách.)
- Chèn 100 fact → context memory không vượt ngân sách, log ghi số fact bị cắt.
- Tắt embedding vẫn tìm được bằng keyword (test có sẵn tiếp tục xanh).

## 5. Kiểm thử

- `cd core-go && go test ./internal/memory/...`
