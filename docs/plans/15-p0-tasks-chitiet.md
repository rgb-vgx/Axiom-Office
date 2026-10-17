# 15 / P0 - Task chi tiết (Hiểu tài liệu sâu)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `01-p0` trước.
> P0-01…P0-07 đã làm ngày 16/10/2026 (xem `CHANGELOG.md [Unreleased]`); còn P0-09 và hai chỗ lệch WPS/LibreOffice.

## P0-01. Mở rộng DocumentContext (Go) - XONG

- [x] Tệp: `core-go/internal/agent/prompt.go` (`DocumentContext` nằm ở đây), `orchestrator.go` (`ParseRequest`).
- [x] Đã có: `Name`, `FullName`, `SelectionText`, `Slide`/`SlideCount`, `Sheet`/`Range`. Thêm `HeadingPath`.
- [x] Đọc từ `body["document"]["view"]["headingPath"]`, thiếu thì bỏ qua, không vỡ request cũ (tối đa 6 mục).
- [x] Test: `TestParseRequestHeadingPath` (`agent_test.go`).
- [x] Word: `HostProbe.View` gửi `headingPath` (mỗi mục 120 ký tự); WPS thiếu thuộc tính thì bỏ qua, không hỏng lượt.

## P0-02. Ngân sách cho mục cấu trúc tài liệu - XONG

- [x] Tệp: `core-go/internal/agent/context.go` (`ContextAssembler.Assemble`, `EstimateTokens`), `prompt.go`.
- [x] Mục cấu trúc tài liệu có ngân sách riêng (≈ 1.500 token), vượt thì cắt và ghi log.
- [x] Test: `TestCatOutlineQuaNganSach` (`internal/office/truncate_test.go`); `go test agent/tools/office` xanh.

## P0-03. Mở rộng `writer.outline` (C#) thay cho `writer.getStructure` - XONG (WPS còn bảng)

- [x] Tệp: `src/AxiomOffice/Bridge/CommandDispatcher.WriterDesign.cs` (`WriterOutline`).
- [x] Thêm style heading, bookmark, caption bảng; giữ nhãn `[khối: …]`/`[ảnh]` (`ShapeLabel`).
- [x] Mở cho agent (`ForAgent()`), mô tả rõ khi nào dùng thay `writer.getText`.
- [x] Registry Python (`axiom/writer.py`) + WPS (`commands.js`): cập nhật mô tả/tham số; `generate_mcp_commands.py --check` khớp.
- [x] LibreOffice: `outline` thật (duyệt body, `ParaStyleName`, hàng tiêu đề bảng), mở agent.
- [ ] **Còn**: WPS `cmdWriterOutline` chưa đọc bảng; LibreOffice chưa có bookmark/caption bảng.

## P0-04. Mở rộng `et.readRange formulas=true` thay cho `et.getFormulaMap` - XONG

- [x] Tệp: `src/AxiomOffice/Bridge/CommandDispatcher.Spreadsheet.cs` (`EtReadRange`, `ToFormulaMatrix`).
- [x] Thêm kiểu dữ liệu từng ô (`types`) và `linkedSheets` (sheet được công thức trỏ tới); giữ `nextRange`.
- [x] LibreOffice + WPS cùng hợp đồng (`calc.py` `_cell_types`/`_linked_sheets`; `commands.js`).
- [x] `truncate` cắt `types` song song `formulas`, giữ `nextRange` (`TestCatFormulasCatTypesSongSong`); LO 25/25.

## P0-05. `wpp.listSlides detail=true` thay cho `wpp.getOutline` - XONG

- [x] Tệp: `src/AxiomOffice/Bridge/CommandDispatcher.Presentation.cs` (dùng `ReadShapes` trong `.Checks.cs` làm mẫu).
- [x] Thêm tên layout, master, ghi chú khi `detail=true`; mặc định không đổi.
- [x] Cả 3 nền tảng (C#, WPS, LibreOffice); LO 25/25.

## P0-06. WPS và LibreOffice - CÒN HAI CHỖ LỆCH NHỎ

- [x] `src/AxiomOffice.WPS/commands.js`, `axiom/{writer,calc,impress}.py`: tham số mới đã làm ở mức có thể; chỗ chưa
      làm thì bỏ qua tham số (không lỗi) và ghi rõ trong mô tả.
- [ ] WPS `writer.outline` đọc bảng (số hàng/cột/hàng tiêu đề) như C# và LibreOffice.
- [ ] LibreOffice `writer.outline` thêm bookmark + caption bảng (C# đã có ở `TableCaption`).
- [ ] `python scripts/generate_mcp_commands.py --check` khớp sau khi sửa.

## P0-07. Cắt kết quả - XONG

- [x] Tệp: `core-go/internal/tools/truncate.go` (`TruncateBridgeResult`), `core-go/internal/office/truncate*.go`.
- [x] Cắt `writer.outline` dài (~1.500 token / ~6.000 ký tự), `formulas` theo vùng, cờ `truncated=true` +
      `totalItems`/`nextRange` + note; dùng chung agent và MCP.
- [x] Test: `go test ./internal/tools/ ./internal/office/ -v`.

## P0-08. Kiểm chứng bằng đọc lại thật - ĐÃ CÓ

- [x] `VerifyWorkNudge` (`internal/model/client.go`), verify guard (`internal/model/guards.go`),
      `internal/agent/verify_scope_test.go`. Chỉ thêm test khi đổi lệnh đọc ở P0-03…05.

## P0-09. Đo bằng bộ `17` + changelog

- [ ] Ba kịch bản P0 (tóm tắt giáo án dài, sửa bảng điểm có công thức, sửa slide giữ layout) trong bộ `17`.
- [ ] Ghi mốc trước P0, chạy lại sau P0, so theo tiêu chí ở `01-p0` mục 5.
- [ ] `cd core-go && go test ./...`; ghi `CHANGELOG.md [Unreleased]`.
- [ ] Gộp việc này vào `18-r0-tasks-chitiet.md` (bộ chạy R0.1/R0.2 là hạ tầng để chấm P0).

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/agent/... ./internal/office/... ./internal/tools/...
cd ..; python scripts/generate_mcp_commands.py --check
powershell -ExecutionPolicy Bypass -File scripts/build.ps1   # đóng Word/Excel/PowerPoint/WPS trước
```
