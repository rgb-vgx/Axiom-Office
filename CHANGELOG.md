# Changelog

Mọi thay đổi đáng chú ý của project được ghi ở đây.
Format tham khảo [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added — Office ports + companion Office + excel-mcp nâng cấp (branch `main`)
- **Port tách theo host**: WPS giữ `Port` 47821-47823; Microsoft Office dùng
  `PortOffice` mới (mặc định 47831-47833) — mở song song WPS + Office thật
  không còn đụng port (bỏ workaround cũ)
- **Companion hỗ trợ Microsoft Office**: `word|excel|ppt` →
  `Word/Excel/PowerPoint.Application`; log `Companion resolved application: ...`
  (cảnh báo khi ProgID bị WPS đăng ký đè ở HKCU — trên máy dev này WPS chiếm
  cả 3 ProgID, companion sẽ tạo WPS compat component version 12.0; Office thật
  dùng in-proc add-in khi mở app)
- **excel-mcp nâng cấp** (tham khảo negokaz/excel-mcp-server): thêm
  `excel_create_sheet`, `excel_copy_sheet`, `excel_rename_sheet`,
  `excel_delete_sheet`, `excel_format_range` (font/fill/border/alignment/
  numFmt/decimalPlaces), `excel_create_table`; `excel_read` thêm
  `show_formula`; tổng 16 tools; migrate SDK `mcp` 2.x (`MCPServer`)
- Kiểm chứng trên file thật `bank-additional-full.xlsx` (41k dòng): profile
  0.01s; query trực tiếp ~5s; convert parquet + query 0.023s; MCP protocol
  roundtrip 16 tools pass

### Verified — Microsoft Office 2024 ProPlus x64 (branch `main`)
- Word (`WINWORD.EXE`), Excel (`EXCEL.EXE`), PowerPoint (`POWERPNT.EXE`): add-in
  nạp đầy đủ lifecycle (OnConnection → OnAddInsUpdate → GetCustomUI →
  OnStartupComplete), ribbon tab hiển thị, bridge hoạt động — không crash,
  không cần sửa signature nào thêm
- Excel/PowerPoint đã E2E trên app thật: ghi/đọc range (Excel), tạo slide +
  textbox + đọc lại (PowerPoint)
- Fix chuẩn hoá ô trống Excel: VT_ERROR `0x800A07FA` / `0x80020004` giờ trả
  `null` trong `readRange` (trước đó trả số `-2146826246`)
- Ghi chú: WPS ET và Excel thật cùng map port 47822 — cần đổi `Port` base khi
  chạy song song hai hệ

### Added — Ribbon UI + AI in-app + MCP server (branch `main`)
- Ribbon tab **"WPS AI Bridge"** (`IRibbonExtensibility.GetCustomUI`, XML chuẩn
  2006/01): nhóm Local bridge (Status / Copy API URL / Open Log) + nhóm AI
  (Ask AI..., Settings); callback qua IDispatch (`ClassInterfaceType.AutoDispatch`)
- **AI Settings**: dialog cấu hình provider (OpenAI-compatible | Anthropic),
  endpoint, API key, model; nút Test gọi thử; lưu vào
  `HKCU\Software\WpsAiBridge` (LlmProvider/LlmEndpoint/LlmApiKey/LlmModel)
- **Ask AI**: dialog prompt + context (None / Selection / Whole document
  ≤50k chars), gọi LLM async không treo UI, chèn kết quả vào tài liệu
  (Insert at cursor); `DocumentContext` đọc toàn văn Writer, used range ET
  (TSV, xử lý mảng 2D COM lower-bound=1), text các slide WPP
- `GET /config`: trả provider/endpoint/model + API key đã che (dạng `sk-a...7e5f`)
- Companion: chế độ `llm-test` kiểm tra cấu hình LLM từ CLI
- `tools/excel-mcp`: MCP server Python (venv riêng: mcp, openpyxl, xlrd,
  duckdb, pandas)
  - Tools file: `excel_profile`, `excel_read` (paging), `excel_query` (DuckDB
    SQL trên bảng "data"), `excel_write`, `excel_create`, `excel_convert`
    (parquet/csv)
  - Tools live: `wps_health`, `wps_live_command`, `wps_live_read_range`,
    `wps_live_write_range` — gọi HTTP bridge 47821-47823
  - Hỗ trợ `.xlsx/.xlsm/.xls/.csv/.tsv`; nhận dạng định dạng bằng magic bytes;
    xử lý file mislabeled (nội dung xlsx nhưng đuôi .xls) qua stream-based
    openpyxl; ghi .xlsm giữ VBA; PermissionError có hint "file đang mở"

### Branch `cpp-native-addin` (f2f0f00) — WIP port C++ native

### Added
- Port add-in in-proc sang C++ native thuần (không CLR):
  - `Json.h/cpp` — JSON parser + serializer UTF-8, không dependency
  - `Http.cpp` — HTTP server Winsock bound 127.0.0.1 (`/health`, `/cmd`)
  - `Com.h/cpp` — IDispatch late-binding, VARIANT ↔ JSON, SAFEARRAY 2D cho `Range.Value2`
  - `Commands.cpp` — đầy đủ parity command set với bản C#
  - `Addin.cpp` — COM object với raw vtable `[IUnknown + IDispatch + IDTExtensibility2]`,
    IClassFactory, `DllGetClassObject`/`DllCanUnloadNow`, marshaling con trỏ
    `Application` qua thread bằng `CoMarshalInterThreadInterfaceInStream`
- `scripts/build-native.ps1` — build bằng `cl.exe` VS2022 BuildTools, `/MT`
  (static CRT — WPS không cần VC runtime), x64, DLL ~320KB
- `scripts/install.ps1` — tự ưu tiên native DLL nếu có: `InprocServer32` trỏ
  thẳng DLL (bỏ mscoree/CodeBase/versioned subkey), thêm `ProgId` + `TypeLib` subkey

### Known issues
- WPS 12 nạp DLL native, tạo instance, QI `IDTExtensibility2` +
  `IRibbonExtensibility` + `ICustomTaskPaneConsumer` nhưng **không invoke
  `OnConnection`** → ghi `AddinsCL` (disable). Gate của WPS cho native add-in
  chưa xác định (nghi: WPS chỉ chạy đầy đủ flow `IDTExtensibility2` cho add-in
  managed qua mscoree). Bản C# trên `main` vẫn là đường chạy chính thức.

## [0.1.0] — 2026-09-29

Commit `a7b1132` (branch `main`). Bản đầu tiên chạy hoàn chỉnh trên WPS 12.1 x64.

### Added
- **In-proc add-in** `WpsAiBridge.dll` (C#, .NET Framework 4.8, x64):
  COM add-in `IDTExtensibility2`, mở HTTP server (HttpListener) trên localhost,
  dùng chung command dispatcher với companion.
- **Companion** `WpsAiBridge.Host.exe`: process riêng dùng COM automation
  (`KWPS/KET/KWPP.Application`) cho app chưa mở hoặc khi add-in không nạp;
  tự chuyển idle nếu port đã do add-in in-proc phục vụ.
- HTTP API: `GET /health`, `POST /cmd` với `{"action", "params"}`; token tuỳ
  chọn qua header `X-Auth-Token`.
- Command set: `app.info`; writer (`newDocument/open/getText/typeText/appendText/
  replaceAll/selection/save/saveAs`); et (`newWorkbook/open/listSheets/readRange/
  writeRange/save/saveAs`); wpp (`newPresentation/open/listSlides/addSlide/
  addTextBox/save/saveAs`).
- Port riêng từng app: 47821 Writer / 47822 Spreadsheets / 47823 Presentation
  (base +0/+1/+2, đổi qua `HKCU\Software\WpsAiBridge`).
- `scripts/build.ps1` (Roslyn csc, không cần dotnet SDK), `install.ps1`,
  `uninstall.ps1` — toàn bộ đăng ký trong HKCU, không cần admin:
  `Classes\CLSID` (mscoree + CodeBase), `Microsoft\Office\*\Addins`
  (`LoadBehavior=3`), `Kingsoft\Office\{WPS,ET,WPP}\AddinsWL` (whitelist).
- Logger dùng chung tại `%LOCALAPPDATA%\WpsAiBridge\bridge.log`.

### Fixed — root cause vụ crash `clr.dll` khi WPS nạp add-in
Triệu chứng: `wps.exe` crash `0xc0000005` trong `clr.dll` (exit code `80131506`)
ngay khi nạp add-in; WPS auto-restart, tự hạ `LoadBehavior 3→2`, ghi
`AddinsCL\WpsAiBridge.Connect=1` (disable) và hiện popup "running into problems".
Không có log nào từ `OnConnection`.

Quá trình khoanh vùng và fix:

1. **Cô lập host**: class `Probe` (không implement `IDTExtensibility2`) tạo được
   instance, không crash → lỗi nằm ở đường invoke `IDTExtensibility2`, không
   phải CLR-hosting nói chung.
2. **Loại trừ nghi phạm**: bỏ dần dynamic COM probe (`DetectAppKind`) khỏi
   `OnConnection`, tạm bỏ HttpBridge, thêm log từng bước → `OnConnection` chạy
   xong hoàn toàn rồi mới crash ở bước kế tiếp.
3. **Xác định điểm chết**: WPS invoke callback kế tiếp (`OnStartupComplete`) và
   marshal tham số `custom` khác kiểu so với `OnConnection` (VARIANT không phải
   SAFEARRAY) → CLR dispatch marshaler AV.
4. **Fix chính**: giữ `IDTExtensibility2` `[InterfaceIsIDispatch]` + `[DispId(1..5)]`,
   nhưng:
   - `OnConnection` dùng `[MarshalAs(UnmanagedType.SafeArray)] ref Array custom`
     (đúng shape WPS yêu cầu — đổi sang `ref object` thì WPS bỏ qua không gọi);
   - 4 callback còn lại (`OnDisconnection/OnAddInsUpdate/OnStartupComplete/
     OnBeginShutdown`) dùng `ref object custom` (VARIANT passthrough).
5. **Resiliency của WPS**: sau mỗi crash WPS tự hạ `LoadBehavior=2` khiến các
   lần sau add-in không được nạp — khi debug phải set lại `LoadBehavior=3` và
   xoá `AddinsCL` mỗi lần.
6. **Kiến trúc WPS 12**: mọi component chạy chung binary `wps.exe` với flag
   `/wps` / `/et` / `/wpp` → nhận diện app bằng COM probe (`Documents` /
   `Workbooks` / `Presentations`) trước, fallback tên process.

### Verified end-to-end
- Writer: `newDocument → typeText → getText → saveAs` (.docx ~10KB) OK
- Spreadsheets: `newWorkbook → writeRange A1:B2 → readRange → saveAs` (.xlsx) OK
- Presentation: `newPresentation → addSlide → addTextBox → listSlides → saveAs`
  (.pptx ~57KB) OK
- `/health` đúng cho cả 3 app chạy song song trên 3 port
