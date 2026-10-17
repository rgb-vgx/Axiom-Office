# 22 / M3 - Task chi tiết (Profile và chuẩn chung)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `12-m3` trước.
> Đích: soạn văn bản mới đúng font/heading/cách xưng hô đã duyệt mà không phải nhắc lại trong câu lệnh.

## M3-01. Scope `org-style` (chuẩn chung của đơn vị)

- [ ] `core-go/internal/memory/store.go`: thêm `ScopeOrgStyle = "org-style"` và nhận trong `IsValidScope`; trong
      `AddBatch` không gán `scope_key` theo tài liệu cho scope này.
- [ ] `core-go/internal/memory/retriever.go` `BuildContext`: luôn đưa fact `org-style` vào context (như `pinned`),
      không phụ thuộc điểm keyword; có ngân sách riêng để không chiếm hết chỗ của fact tài liệu.
- [ ] `core-go/internal/memory/service.go` `Active`: `org-style` không tự hết hạn, không bị dọn theo tệp.
- [ ] `core-go/internal/agent/prompt.go`: hiện nhóm "Chuẩn chung của đơn vị" tách khỏi memory cá nhân/tài liệu.
- [ ] Test: thêm `org-style` rồi hỏi một câu không liên quan → vẫn thấy trong context.

## M3-02. Nhớ kiểu trình bày đã chọn (`doc_design`)

- [ ] `core-go/internal/tools/doc_design.go` (`chooseDocStyle`, `run.docStyle`): sau khi người dùng chọn (kể cả
      `custom` tự gõ), ghi một fact `user` hoặc `document` mô tả lựa chọn (dùng `memory.Service.Add`).
- [ ] Lần sau mở tài liệu/người dùng đó: lựa chọn đã nhớ làm **lựa chọn đầu** trong thẻ `Choose`, vẫn hỏi (không tự
      áp im lặng).
- [ ] Test `doc_design_test.go`: chọn `full` → có memory; lượt sau thẻ chọn xếp `full` đầu.

## M3-03. Áp chuẩn chung đúng lúc

- [ ] `core-go/internal/agent/orchestrator.go`: memory context (đã gồm `org-style`) được nạp trước khi model chọn
      skill trình bày.
- [ ] `core-go/internal/agent/prompt.go`: danh sách skill + `not_for` (P2-01); chuẩn chung nhắc ở mục riêng để model
      ưu tiên cho `van-ban-hanh-chinh`, `trinh-bay-chuyen-nghiep`, `thiet-ke-van-phong`.
- [ ] Không sửa logic chọn skill tự động - chỉ làm input cho model rõ hơn.

## M3-04. Phân quyền tối thiểu cho `org-style`

- [ ] `core-go/internal/config/config.go`: thêm khóa kiểu `OrgStyleAdmin` (env `AXIOM_ORG_STYLE_ADMIN` + HKCU) hoặc
      đọc tệp chuẩn chung trong thư mục chung **chỉ đọc**; người dùng thường không ghi được `org-style`.
- [ ] `core-go/internal/api/memory.go`: POST/PATCH/DELETE với scope `org-style` từ phiên không phải Admin → 403.
- [ ] Test API: `TestOrgStyleRequiresAdmin`.

## M3-05. UI `MemoryForm`

- [ ] `src/AxiomOffice/Ai/MemoryForm.cs`: chia nhóm **Tài liệu | Cá nhân | Chuẩn chung**; nhóm "Chuẩn chung" chỉ
      hiện nút sửa/xóa khi là Admin.
- [ ] Đổi chữ mới phải có trong `src/AxiomOffice/lang/en.json` (`python scripts/check_i18n.py`).

## M3-06. Hồi quy + changelog

- [ ] `cd core-go && go test ./internal/memory/... ./internal/api/... ./internal/tools/...`.
- [ ] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `12-m3`.

## Tiêu chí xong

- [ ] Đặt chuẩn chung "font Times New Roman 13, lề trái 3 cm" → kịch bản soạn công văn trong bộ `17` đạt tiêu chí
      font/lề 5/5 lần mà không nhắc lại trong câu lệnh.
- [ ] Người dùng thường không sửa được `org-style` (test API).

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/memory/... ./internal/api/... ./internal/tools/...
cd ..; python scripts/check_i18n.py
```
