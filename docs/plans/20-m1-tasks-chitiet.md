# 20 / M1 - Task chi tiết (Nhớ theo tài liệu)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `10-m1` trước.
> Lịch sử theo tệp đã có 17/10/2026; còn nối theo vân tay khi đổi tên và kịch bản "không lẫn".

## M1-00. Lọc theo tài liệu - ĐÃ CÓ

- [x] `GET /v1/conversations?documentKey=…` (`core-go/internal/api/runs.go`), `Conversations.List(documentKey
      *string, limit)` (`internal/store/conversations.go`).
- [x] Pane mở hội thoại của tài liệu đang mở: `AskAiPane` menu "⋯" → `ConversationsForm(documentKey)`, ô "Chỉ tài
      liệu đang mở" (`src/AxiomOffice/Ai/AdminForms.cs`).
- [x] Tài liệu chưa lưu không dùng tên làm khóa: `conversationKey` (`orchestrator.go`) chỉ nhận đường dẫn thật;
      `TestConversationKeyOnlyForSavedDocuments`.

## M1-01. Dấu vân tay nội dung để nối sau khi đổi tên/tệp

- [ ] Tệp: `core-go/internal/office/session.go` (`DocumentKey(fullName)`), hoặc hàm mới `DocumentFingerprint`.
- [ ] Vân tay: hash vài trăm ký tự đầu + số trang/sheet/slide (lấy từ lệnh đọc đã có, không mở thêm đường đọc
      tài liệu). Lưu vào hội thoại/memory document: **không đổi schema** - thêm cột hoặc dùng `scope_key` phụ, cân
      nhắc trước khi đổi `core-go/internal/store/db.go` (`SchemaVersion = 3`).
- [ ] Khi mở tệp lạ mà vân tay khớp hội thoại cũ: **hỏi người dùng** (thẻ `Choose` trong `internal/agent/confirm.go`)
      có nối không; đồng ý thì gắn lại `documentKey`, không thì để mới.
- [ ] Không tự nối: "nhầm tệp nguy hiểm hơn quên" (`10-m1` mục 6).

## M1-02. Memory document theo tệp

- [ ] Kiểm truy hồi chỉ lấy đúng `documentKey` hiện tại (`internal/memory/retriever.go` `Search`/`BuildContext`,
      tham số `documentKey`).
- [ ] Khi tệp đổi tên và người dùng đã đồng ý nối: cập nhật `scope_key` các memory document cũ (dùng `Store.Update`
      hoặc một hàm chuyển scope mới) + ghi `memory_history` lý do "đổi tên tệp".

## M1-03. Kiểm "không lẫn" bằng bộ `17`

- [ ] Kịch bản `tests/regress/scenarios/memory-hai-tep.json`: mở tệp A ghi nhớ việc riêng, đóng, mở tệp B hỏi việc
      của A → AI **không** dùng memory của A.
- [ ] Chấm bằng `recall` + đọc context; ghi số đo vào bảng R0.

## M1-04. Hồi quy + changelog

- [ ] `cd core-go && go test ./internal/memory/... ./internal/store/...`.
- [ ] Golden hội thoại trong `tests/core/test_core_e2e.py`.
- [ ] Ghi `CHANGELOG.md [Unreleased]`, cập nhật `10-m1` nếu đổi.

## Tiêu chí xong

- [ ] Hai tệp khác nhau, hỏi việc tệp A khi đang mở tệp B → không lẫn, 5/5 lần.
- [ ] Đổi tên tệp A → AI hỏi có nối với lịch sử cũ không; đồng ý thì nhớ lại được.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/memory/... ./internal/store/... -run "TestDocumentKey|TestConversationKey"
cd ..; AXIOM_E2E_CORE_EXE=<binary> python tests/core/test_core_e2e.py --only memory
```
