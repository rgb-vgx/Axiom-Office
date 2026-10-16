# 10 / M1 - Nhớ theo tài liệu (documentKey)

> Mở lại tệp cũ thì nhớ việc đó. Tệp khác không lẫn.

## 1. Đã có (đối chiếu code 16/10/2026)

- `documentKey` gắn vào hội thoại (`conversations.document_key`) và memory scope `document` (`scope_key`).
- Pane Windows mở lại hội thoại gần nhất của tài liệu (`CoreClient.FindConversationForDocument`).
- Truy hồi lọc theo `documentKey` (`retriever.go`, `Search(query, documentKey, …)`).
- Menu "⋯" của khung chat: hội thoại trước, nhật ký hoạt động.

## 2. Còn thiếu - việc cần làm

- **Đổi tên/di chuyển tệp**: hiện khóa theo đường dẫn nên đổi tên là mất nhớ. Thêm dấu vân tay nội dung (ví dụ
  hash vài trăm ký tự đầu + số trang/sheet/slide) để gợi ý nối; nối thì hỏi người dùng, không tự nối.
- **Lịch sử theo tệp**: menu "⋯" lọc hội thoại theo tài liệu đang mở (`GET /v1/conversations?documentKey=…`),
  mở lại được hội thoại cũ.
- **Kiểm không lẫn**: kịch bản trong bộ `17`.

## 3. Tệp sẽ sửa

- `core-go/internal/store/conversations.go`, `core-go/internal/api/` (lọc theo `documentKey`)
- `core-go/internal/memory/retriever.go` (nếu thêm nối theo vân tay)
- `src/AxiomOffice/Ai/AskAiPane.cs` (menu "⋯"), `CoreClient.cs`

## 4. Tiêu chí xong

- Hai tệp khác nhau, hỏi việc tệp A khi đang mở tệp B → AI không dùng memory của A, 5/5 lần.
- Đổi tên tệp A → AI hỏi có nối với lịch sử cũ không; đồng ý thì nhớ lại được.

## 5. Kiểm thử

- `cd core-go && go test ./internal/memory/... ./internal/store/...`
- Golden hội thoại trong `tests/core/test_core_e2e.py`.

## 6. Không làm

- Không tự nối khi không chắc; nhầm tệp nguy hiểm hơn quên.
