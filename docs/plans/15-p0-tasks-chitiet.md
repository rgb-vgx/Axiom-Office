# 15 / P0 - Task chi tiết (Hiểu tài liệu sâu)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 16/10/2026; đọc `01-p0` trước.

## P0-01. Mở rộng DocumentContext (Go)

- [ ] Tệp: `core-go/internal/agent/prompt.go` (`DocumentContext` nằm ở đây), `orchestrator.go` (`ParseRequest`).
- [ ] Đã có: `Name`, `FullName`, `SelectionText`, `Slide`/`SlideCount`, `Sheet`/`Range`. Thêm `HeadingPath`.
- [ ] Đọc từ `body["document"]["view"]["headingPath"]`, thiếu thì bỏ qua, không vỡ request cũ.
- [ ] Test: `go test ./internal/agent/ -run TestParseRequest -v`.

## P0-02. Ngân sách cho mục cấu trúc tài liệu

- [ ] Tệp: `core-go/internal/agent/context.go` (`ContextAssembler.Assemble`, `EstimateTokens`), `prompt.go`.
- [ ] Mục cấu trúc tài liệu có ngân sách riêng (≈ 1.500 token), vượt thì cắt và ghi log.
- [ ] Test: `go test ./internal/agent/ -run "TestBuildPromptSections|TestAssemble" -v` (thêm test nếu thiếu).

## P0-03. Mở rộng `writer.outline` (C#) thay cho `writer.getStructure`

- [ ] Tệp: `src/AxiomOffice/Bridge/CommandDispatcher.WriterDesign.cs` (`WriterOutline`).
- [ ] Thêm style heading, bookmark, caption bảng; giữ nhãn `[khối: …]`/`[ảnh]` (`ShapeLabel`).
- [ ] Mở cho agent (`ForAgent()`), mô tả rõ khi nào dùng thay `writer.getText`.
- [ ] Registry Python (`axiom/writer.py`): cập nhật mô tả/tham số, chạy `python scripts/generate_mcp_commands.py`.
- [ ] Build `scripts/build.ps1` (đóng Office trước), thử tay `POST /cmd writer.outline` trên bản sao giáo án.

## P0-04. Mở rộng `et.readRange formulas=true` thay cho `et.getFormulaMap`

- [ ] Tệp: `src/AxiomOffice/Bridge/CommandDispatcher.Spreadsheet.cs` (`EtReadRange`, `ToFormulaMatrix`).
- [ ] Thêm kiểu dữ liệu từng ô và sheet được công thức tham chiếu; giữ phân trang `nextRange` của truncate.
- [ ] Thử: workbook mẫu có công thức liên sheet, so kết quả với Excel.

## P0-05. `wpp.listSlides detail=true` thay cho `wpp.getOutline`

- [ ] Tệp: `src/AxiomOffice/Bridge/CommandDispatcher.Presentation.cs` (dùng `ReadShapes` trong `.Checks.cs` làm mẫu).
- [ ] Thêm tên layout, master, ghi chú khi `detail=true`; không đổi kết quả mặc định.
- [ ] Thử: pptx mẫu 10 slide.

## P0-06. WPS và LibreOffice

- [ ] `src/AxiomOffice.WPS/commands.js`, `axiom/{writer,calc,impress}.py`: làm tham số mới khi làm được; chưa làm
      thì bỏ qua tham số (không lỗi), ghi rõ trong mô tả.
- [ ] `python scripts/generate_mcp_commands.py --check` khớp.

## P0-07. Cắt kết quả

- [ ] Tệp: `core-go/internal/tools/truncate.go` (`TruncateBridgeResult`), `core-go/internal/office/truncate*.go`.
- [ ] Cắt `writer.outline` dài, `formulas` theo vùng, cờ `truncated=true` + `nextRange`.
- [ ] Test: `go test ./internal/tools/ ./internal/office/ -v`.

## P0-08. Kiểm chứng bằng đọc lại thật - ĐÃ CÓ

- [x] `VerifyWorkNudge` (`internal/model/client.go`), verify guard (`internal/model/guards.go`),
      `internal/agent/verify_scope_test.go`. Chỉ thêm test khi đổi lệnh đọc ở P0-03…05.

## P0-09. Đo bằng bộ `17` + changelog

- [ ] Ba kịch bản P0 (tóm tắt giáo án dài, sửa bảng điểm có công thức, sửa slide giữ layout) trong bộ `17`.
- [ ] Ghi mốc trước P0, chạy lại sau P0, so theo tiêu chí ở `01-p0` mục 5.
- [ ] `cd core-go && go test ./...`; ghi `CHANGELOG.md [Unreleased]`.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/agent/... ./internal/office/... ./internal/tools/...
cd ..; python scripts/generate_mcp_commands.py --check
powershell -ExecutionPolicy Bypass -File scripts/build.ps1   # đóng Word/Excel/PowerPoint/WPS trước
```
