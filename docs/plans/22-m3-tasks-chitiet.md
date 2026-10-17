# 22 / M3 - Task chi tiết (Profile và chuẩn chung)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `12-m3` trước.
> Đích: soạn văn bản mới đúng font/heading/cách xưng hô đã duyệt mà không phải nhắc lại trong câu lệnh.

## M3-01. Scope `org-style` (chuẩn chung của đơn vị) - XONG

- [x] `core-go/internal/memory/store.go`: `ScopeOrgStyle = "org-style"`, nhận trong `IsValidScope`; `AddBatch` không gán
      `scope_key` cho scope này.
- [x] `core-go/internal/memory/retriever.go` `BuildContext`: luôn đưa fact `org-style` vào context (như `pinned`), tách
      vào `Context.OrgLines` với trần riêng `MaxOrgStyleMemories = 12`.
- [x] `core-go/internal/memory/store.go` `Active`: `org-style` luôn có mặt, không gắn theo tệp.
- [x] `core-go/internal/agent/prompt.go`: mục "Organisation-wide standards" (`OrgStyleSection`) tách khỏi memory cá
      nhân/tài liệu.
- [x] Test `TestOrgStyleAlwaysInContext`: câu hỏi không liên quan vẫn thấy chuẩn chung; fact tài liệu khác không lọt.

## M3-02. Nhớ kiểu trình bày đã chọn (`doc_design`) - XONG

- [x] `core-go/internal/tools/doc_design.go` (`chooseDocStyle`): sau khi người dùng chọn xong ở thẻ, gọi
      `run.RememberDocStyle(style, label)`; Core ghi một fact `user` "Kiểu trình bày tài liệu ưa thích: <id> (<label>)"
      qua `memory.Service.Add` (`agent/orchestrator.go` đặt hook, chỉ khi có memory và không ở plan mode).
- [x] Lần sau: `agent/docstyle.go` `rememberedDocStyle` đọc lại từ ngữ cảnh memory; `preferChoice` xếp lựa chọn đã nhớ
      **lên đầu** thẻ `Choose` - vẫn hỏi, không tự áp im lặng.
- [x] Test `TestDocDesignRemembersChosenStyle` (chọn `full` → gọi ghi lại + kieu đã nhớ lên đầu),
      `TestRememberedDocStyle`.

## M3-03. Áp chuẩn chung đúng lúc - XONG

- [x] `core-go/internal/agent/orchestrator.go`: memory context (đã gồm `org-style`) nạp trước khi `Build` prompt
      (model chọn skill sau khi đọc prompt).
- [x] `core-go/internal/agent/prompt.go`: danh sách skill + `not_for` (P2-01) và mục "Organisation-wide standards"
      riêng để model ưu tiên cho `van-ban-hanh-chinh`, `trinh-bay-chuyen-nghiep`, `thiet-ke-van-phong`.
- [x] Không sửa logic chọn skill tự động - chỉ làm input cho model rõ hơn.

## M3-04. Phân quyền tối thiểu cho `org-style` - XONG

- [x] `core-go/internal/config/config.go`: cờ `OrgStyleAdmin` (env `AXIOM_ORG_STYLE_ADMIN`, mặc định false).
- [x] `core-go/internal/api/memory.go`: POST/PATCH/DELETE với scope `org-style` từ phiên không phải Admin → 403;
      `?scope=all` của người thường không đụng `org-style`.
- [x] Test `TestOrgStyleRequiresAdmin`.

## M3-05. UI `MemoryForm`

- [ ] `src/AxiomOffice/Ai/MemoryForm.cs`: chia nhóm **Tài liệu | Cá nhân | Chuẩn chung**; nhóm "Chuẩn chung" chỉ
      hiện nút sửa/xóa khi là Admin.
- [ ] Đổi chữ mới phải có trong `src/AxiomOffice/lang/en.json` (`python scripts/check_i18n.py`).

## M3-06. Hồi quy + changelog

- [x] `cd core-go && go test ./internal/memory/... ./internal/api/... ./internal/tools/...`.
- [x] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `12-m3`.

## Tiêu chí xong

- [ ] Đặt chuẩn chung "font Times New Roman 13, lề trái 3 cm" → kịch bản soạn công văn trong bộ `17` đạt tiêu chí
      font/lề 5/5 lần mà không nhắc lại trong câu lệnh.
- [ ] Người dùng thường không sửa được `org-style` (test API).

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/memory/... ./internal/api/... ./internal/tools/...
cd ..; python scripts/check_i18n.py
```
