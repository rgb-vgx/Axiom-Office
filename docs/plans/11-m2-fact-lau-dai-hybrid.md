# 11 / M2 - Fact lâu dài: an toàn và ngân sách

> Hỏi việc tháng trước vẫn tìm được. Không lưu thứ không được lưu.

## 1. Đã có (đối chiếu code 16/10/2026)

- Truy hồi lai: bm25 (FTS5 bỏ dấu) + entity boost + cosine khi có embedding; không có embedding thì chỉ dùng
  keyword (`retriever.go`). Embedding lỗi không chặn lượt.
- Tool `recall` cho model tìm memory chủ động - **chính là `session_search` của bản kế hoạch cũ**, không viết thêm.
- Trích xuất trả `{text, category, scope, entities, confidence, expires_at}` (`prompts/extract.txt`).

## 2. Còn thiếu - việc cần làm

- **Ẩn thông tin nhạy cảm trước khi lưu**: hiện chưa có chỗ nào redact. Lọc số điện thoại, email, số CCCD/CMND,
  số tài khoản, khóa API/token trước khi `AddBatch`; gặp thì bỏ hẳn fact hoặc thay bằng `[ẩn]`.
- **Số liệu phải kèm nguồn**: fact có số mà không kèm tài liệu/vùng/thời điểm thì không lưu (sửa `extract.txt` +
  kiểm trong `extractor.go`).
- **Ngân sách token**: `BuildContext` cắt theo ngân sách; ghi log số fact bị cắt để biết khi nào cần chỉnh.

## 3. Tệp sẽ sửa

- `core-go/internal/memory/extractor.go`, `text.go`, `prompts/extract.txt`, `retriever.go`
- (mới) `core-go/internal/memory/redact.go` + test

## 4. Tiêu chí xong

- Seed 50 câu có số điện thoại/CCCD/token giả → không câu nào lọt vào `memories` (đọc thẳng `core.db`).
- Chèn 100 fact → context memory không vượt ngân sách, log ghi số fact bị cắt.
- Tắt embedding vẫn tìm được bằng keyword (test có sẵn tiếp tục xanh).

## 5. Kiểm thử

- `cd core-go && go test ./internal/memory/...`
