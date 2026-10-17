# 22 / M3 - Task chi tiết (Profile và chuẩn chung)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `12-m3` trước.
> Đích: soạn văn bản mới đúng font/heading/cách xưng hô đã duyệt mà không phải nhắc lại trong câu lệnh.

## M3-01. Scope `org-style` (chuẩn chung của đơn vị) - XONG

- [x] `core-go/internal/memory/store.go`: `ScopeOrgStyle = "org-style"`, nhận trong `IsValidScope`; `AddBatch` không gán
      `scope_key` theo tài liệu cho scope này (từ 17/10/2026: `scope_key` chứa **app** — xem M3-07).
- [x] `core-go/internal/memory/retriever.go` `BuildContext`: luôn đưa fact `org-style` vào context (như `pinned`), tách
      vào `Context.OrgLines` với trần riêng `MaxOrgStyleMemories = 12`.
- [x] `core-go/internal/memory/store.go` `Active`: `org-style` luôn có mặt, không gắn theo tệp (từ 17/10/2026: lọc
      theo app — xem M3-07).
- [x] `core-go/internal/agent/prompt.go`: mục "Organisation-wide standards" (`OrgStyleSection`) tách khỏi memory cá
      nhân/tài liệu.
- [x] Test `TestOrgStyleAlwaysInContext`: câu hỏi không liên quan vẫn thấy chuẩn chung; fact tài liệu khác không lọt.

## M3-02. Nhớ kiểu trình bày đã chọn (`doc_design`) - XONG

- [x] `core-go/internal/tools/doc_design.go` (`chooseDocStyle`): sau khi người dùng chọn xong ở thẻ, gọi
      `run.RememberDocStyle(style, label)`; Core ghi một fact **`doc-style`** "Kiểu trình bày tài liệu ưa thích: <id>
      (<label>)" với `scope_key = app` qua `memory.Service.Add` (`agent/orchestrator.go` đặt hook, chỉ khi có memory và
      không ở plan mode). (Từ 17/10/2026 — trước đó ghi fact `user`; xem M3-07.)
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

- [ ] `src/AxiomOffice/Ai/MemoryForm.cs`: chia nhóm **Tài liệu | Cá nhân | Chuẩn chung** (nhóm Chuẩn chung ghi rõ app +
      kiểu trình bày đã chọn theo app — M3-07); nhóm "Chuẩn chung" chỉ hiện nút sửa/xóa khi là Admin.
- [ ] Đổi chữ mới phải có trong `src/AxiomOffice/lang/en.json` (`python scripts/check_i18n.py`).

## M3-06. Hồi quy + changelog

- [x] `cd core-go && go test ./internal/memory/... ./internal/api/... ./internal/tools/...`.
- [x] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `12-m3`.

## M3-07. Nhánh memory theo app (12-m3 mục 7) - XONG 17/10/2026

- [x] `core-go/internal/memory/store.go`: thêm `ScopeDocStyle = "doc-style"` (nhận trong `IsValidScope`); hằng app
      `AppWord/AppExcel/AppPowerPoint` + `NormalizeApp` (nhận cả `word`/`excel`/`powerpoint`, "" = dùng chung mọi app).
- [x] `AddBatch`/`Import`: `org-style`/`doc-style` lấy `scope_key = NormalizeApp(...)` thay vì bắt buộc rỗng; `user` vẫn
      rỗng; `document` vẫn theo đường dẫn tệp.
- [x] `Store.Active(app, documentKey, today)`: `user` (chung) + `org-style`/`doc-style` (khớp app HOẶC `scope_key` rỗng)
      + `document` của tệp hiện tại. Không đổi schema, không bump `SchemaVersion`.
- [x] `Retriever.Search`/`BuildContext` và `Service.Context`/`Search` nhận thêm `app`; `ExtractionJob.App`;
      `RecallTool` nhận `app`; `orchestrator` truyền `appKind`.
- [x] `SoftDeleteByScopeKey`: chỉ đụng scope `document` (tránh xóa nhầm chuẩn trinh bày khoá bằng app trùng chuỗi).
- [x] `mergeNearDuplicates`: bỏ qua cả `org-style` lẫn `doc-style`.
- [x] API `POST /v1/memory` nhận `app` cho `org-style`/`doc-style`; `?scope=all` của người thường xóa cả `doc-style`.
- [x] Test `TestStyleMemoriesSplitByApp`, `TestNormalizeApp`, `TestSoftDeleteByScopeKeyIgnoresStyleScopes`; cập nhật
      `TestOrgStyleAlwaysInContext`, `TestScopeFilterDisablesScopes`.

## Tiêu chí xong

- [ ] Đặt chuẩn chung "font Times New Roman 13, lề trái 3 cm" → kịch bản soạn công văn trong bộ `17` đạt tiêu chí
      font/lề 5/5 lần mà không nhắc lại trong câu lệnh.
- [ ] Người dùng thường không sửa được `org-style` (test API).

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/memory/... ./internal/api/... ./internal/tools/...
cd ..; python scripts/check_i18n.py
```
