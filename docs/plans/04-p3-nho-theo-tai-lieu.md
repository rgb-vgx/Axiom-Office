# 04 / P3 - Nhớ theo tài liệu (tổng quan)

> Tổng quan phase memory. Chi tiết phần còn thiếu ở `10`–`14` (M1–M5).

## 1. Mục tiêu

- Mở lại tệp cũ thì nhớ việc đó; mở tệp khác thì không lẫn.
- Nhớ quy ước và kiểu trình bày đã duyệt, không nhớ sai số liệu.
- Local-first: SQLite một tệp, người dùng xem/sửa/xóa/mang sang máy khác được.

## 2. Đã có (mốc G4 trong STATUS.md, đối chiếu code 16/10/2026)

- Lưu trữ (`core-go/internal/memory/store.go`, `core.db`): bảng `memories` (`scope`, `scope_key`, `category`,
  `entities_json`, `expires_at`, `source`, `confidence`, `pinned`, `hits`), `memories_fts` (FTS5,
  `remove_diacritics 2`), `memory_embeddings`, `memory_links` (liên kết chuyển đổi), `memory_history`.
- Scope: `user`, `document` (theo `documentKey`), `skill`.
- Chống trùng: hash + cosine 0.96. Ghim, hạn dùng, xóa mềm + khôi phục, lịch sử sửa.
- Truy hồi (`retriever.go`): bm25 chuẩn hóa + entity boost + cosine khi có embedding; lọc theo `documentKey`.
- Trích xuất nền sau mỗi lượt (`service.go`, `extractor.go`, `prompts/extract.txt`), bỏ qua lượt chỉ thao tác tài liệu.
- Hội thoại theo tài liệu: `conversations.document_key`, pane mở lại hội thoại gần nhất của tài liệu.
- API `/v1/memory`: GET (lọc), POST, PATCH, DELETE (mềm), restore, history, xóa hàng loạt (`?scope=…&confirm=true`).
- Tool cho model: `remember`, `recall`. UI: `src/AxiomOffice/Ai/MemoryForm.cs`.

## 3. Còn thiếu (chi tiết ở các tệp con)

| Tệp | Phần còn thiếu |
|---|---|
| `10-m1` | Đổi tên/di chuyển tệp vẫn nối được `documentKey`; lịch sử theo tệp trong pane |
| `11-m2` | Ẩn thông tin nhạy cảm trước khi lưu; số liệu phải kèm nguồn; ngân sách token có log |
| `12-m3` | Scope `org-style` (chuẩn chung, chỉ Admin sửa) |
| `13-m4` | Export/import JSON; xóa theo tệp; nhật ký sửa tay hiện trong UI |
| `14-m5` | Gộp/hạ điểm định kỳ, số đo memory có ích hay rác |

**Không đổi schema** sang `memory_fts`/`superseded_by` như bản kế hoạch cũ: schema đang chạy đã có
`memory_links` + `memory_history` làm việc đó; đổi là phải migrate dữ liệu thật của người dùng mà không được gì.

## 4. Tiêu chí xong chung

- Đo bằng bộ `17`: hai tệp khác nhau, hỏi việc tệp A khi đang mở tệp B → không lẫn, 5/5 lần.
- Context memory có ngân sách token; vượt thì cắt và ghi log.
- Người dùng xem/sửa/xóa/xuất được từng scope.

## 5. Không làm

- Không tự sinh skill từ memory, không suy đoán chân dung người dùng, không đồng bộ cloud, không vector DB riêng.
