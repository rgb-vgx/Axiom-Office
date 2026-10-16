# 01 / P0 - Hiểu tài liệu sâu

> Agent văn phòng sửa tại chỗ, thấy ngay, hoàn tác được. P0 làm cho agent "đọc đúng" trước khi "viết hay".

## 1. Mục tiêu

- Agent đọc được cấu trúc tài liệu (heading, bảng, công thức, bố cục slide) với số token vừa phải, đủ để model
  yếu không lạc khi tài liệu dài.
- Giữ công thức Excel, tham chiếu chéo Word, master slide PowerPoint khi sửa.

## 2. Đã có (đối chiếu code 16/10/2026)

- `DocumentContext` nằm ở `core-go/internal/agent/prompt.go` (không phải `orchestrator.go`): `Name`, `FullName`,
  `SelectionText`, `Slide`/`SlideCount`, `Sheet`/`Range`. Pane Windows gửi `document.view`.
- Lệnh đọc có cấu trúc **đã có**, chỉ chưa mở hết cho agent:
  - `writer.outline` (từ `doc_design`): đoạn đánh số, chữ rút gọn, số tự động, bảng (số hàng/cột/hàng tiêu đề);
    khối và ảnh hiện thành `[khối: …]`/`[ảnh]`. Hiện `agent: false`.
  - `writer.checkTables`, `writer.getText` (khối/ảnh có nhãn, không còn dấu "/" ma).
  - `et.readRange` với `formulas=true` trả mảng công thức (`ToFormulaMatrix` trong `CommandDispatcher.Spreadsheet.cs`);
    `et.checkRange`.
  - `wpp.listSlides`, `wpp.sourceManifest` (tóm tắt cả bài), `wpp.checkLayout`.
- Cắt kết quả lớn: `core-go/internal/tools/truncate.go` (`TruncateBridgeResult`) và `core-go/internal/office/truncate*.go`.
- Tự đọc lại sau khi sửa: `VerifyWorkNudge` (`internal/model/client.go`), verify guard (`internal/model/guards.go`),
  `internal/agent/verify_scope_test.go`.
- Catalog: 78 lệnh, 57 lệnh cho agent (không phải 47).

## 3. Còn thiếu - việc cần làm

### 3.1 Mở rộng lệnh có sẵn thay vì viết lệnh mới

Không viết `writer.getStructure`, `et.getFormulaMap`, `wpp.getOutline` (trùng lệnh có sẵn, mỗi lệnh phải làm ở 3
nền tảng C#/WPS JS/Python UNO). Thay vào đó:

- `writer.outline`: thêm style của heading, bookmark, caption bảng; mở cho agent (`ForAgent()`), có `maxChars`.
- `et.readRange formulas=true`: trả kèm kiểu dữ liệu và sheet liên kết khi công thức trỏ sheet khác; giữ phân
  trang `nextRange` của truncate.
- `wpp.listSlides`: thêm tên layout/master và ghi chú khi `detail=true` (hoặc dùng `wpp.sourceManifest`).
- Mỗi thay đổi: C# trước, rồi WPS JS (`src/AxiomOffice.WPS/commands.js`) và Python UNO (`axiom/*.py`) ở mức có thể;
  phía chưa làm thì để stub báo rõ, giống `_design_unsupported`.

### 3.2 Giàu DocumentContext có ngân sách

- Thêm `HeadingPath` (đường heading quanh con trỏ) cho Word; pane gửi trong `document.view`.
- `ContextAssembler.Assemble` (`internal/agent/context.go`): mục cấu trúc tài liệu có ngân sách token riêng, quá
  thì cắt bằng truncate có sẵn.

### 3.3 Đo bằng bộ `17`

- Kịch bản: tóm tắt giáo án 20k+ ký tự, sửa bảng điểm có công thức, sửa slide giữ layout.
- Tiêu chí xong tính bằng số đo, không bằng cảm nhận.

## 4. Tệp sẽ sửa

- `core-go/internal/agent/prompt.go`, `context.go`, `orchestrator.go` (`ParseRequest`)
- `core-go/internal/tools/truncate.go`, `core-go/internal/office/truncate*.go`
- `src/AxiomOffice/Bridge/CommandDispatcher.WriterDesign.cs` (`writer.outline`), `.Spreadsheet.cs`, `.Presentation.cs`
- `src/AxiomOffice.WPS/commands.js`, `src/AxiomOffice.LibreOffice/python/pythonpath/axiom/*.py`
- Danh mục lệnh: chạy `python scripts/generate_mcp_commands.py`

## 5. Tiêu chí xong (đo bằng bộ `17`)

- Kịch bản "tóm tắt giáo án 20k+ ký tự" với mimo và deepseek: đủ các mục lớn của tài liệu ở ≥ 4/5 lần chạy, số
  token đầu vào không tăng quá 20% so với mốc trước P0.
- Kịch bản "sửa bảng điểm": công thức sau khi sửa giống trước (so bằng `et.readRange formulas=true`) ở 5/5 lần.
- Kịch bản slide: layout từng slide không đổi (so `wpp.listSlides`) ở 5/5 lần.

## 6. Kiểm thử

- `cd core-go && go test ./internal/agent/... ./internal/office/... ./internal/tools/...`
- `python scripts/generate_mcp_commands.py --check`
- `tests/live/test_live_commands.py` (cần app thật) cho các lệnh đã mở rộng.

## 7. Không làm

- Không thêm trình duyệt/computer-use, không tự sinh skill, không lưu nguyên văn tài liệu dài vào memory.
