# Changelog

Mọi thay đổi đáng chú ý của project được ghi ở đây.
Format tham khảo [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added — Event stream SSE `GET /events` (Phase 2)
- Mỗi bridge phát Server-Sent Events: `hello`, `document` (tài liệu active đổi),
  `selection` (Writer: text ≤ 200 ký tự + start/end; ET: sheet + address + giá trị
  ô trên-trái; WPP: slide/type/shape/text), `ping` mỗi 15s
- V1 poll-diff 500ms (không COM event sink — chạy giống nhau Office/WPS), poller chỉ
  chạy khi có subscriber, tối đa 5 subscriber (thứ 6 nhận 503); mỗi kết nối có thread
  ghi riêng nên `/cmd` không bị chặn (đo: 12 ms trong lúc stream); lỗi COM log tối đa
  1 lần/phút mỗi loại
- Python: `bridge.es_hint()` trong cả 3 MCP (URL `/events` + ví dụ curl)
- Đã verify: Word 47831 (selection khi bôi đen, document khi mở file khác, ping),
  Excel 47832 (selection theo địa chỉ `$B$2`, `$C$3:$D$5`), WPS Writer 47821

### Added — Session registry + `GET /session` (Phase 1)
- `Bridge/SessionRegistry.cs`: mỗi bridge ghi `%LOCALAPPDATA%\WpsAiBridge\sessions\{pid}.json`
  (`pid`, `app`, `family` office|wps, `port`, `host`, `started`, `lastSeen`, `document`),
  heartbeat 25s, xoá file khi Stop/OnDisconnection; đọc tên tài liệu chạy nền, chờ tối
  đa 2s, host bận thì giữ giá trị cũ (heartbeat không bao giờ trễ vì Word bận)
- `GET /session` (cần token): thông tin bridge + tài liệu đọc live lúc gọi
- Python `bridge.sessions()` trong cả 3 MCP: đọc registry, gọi `/health` (1.5s) để đánh
  dấu `healthy`, prune khi process chết hoặc heartbeat > 90s mà health cũng fail
  (bridge đang bận lệnh dài vẫn được giữ, `healthy: false`) + unittest 5 case
- MCP tool `office_sessions()` cho cả 3 server; docstring `wps_live_command` /
  `word_command` / `ppt_command` hướng dẫn truyền `port` lấy từ danh sách này
- Đã verify: Word + Excel cùng lúc → 2 session đúng port/family; WPS Writer → family `wps`;
  kill Word/WPS → session bị prune; regression `/health` 200, `/cmd` 401/415/403, `ai.ask` ok

### Changed — Ask AI pane thiết kế lại
- Design tokens `PaneTheme` (màu theo vai trò, font Segoe UI/Semibold tạo một lần, spacing,
  radius, scale theo DPI); nút Ask/link đổi sang `#4F46E5` vì trắng trên `#6366F1` chỉ đạt
  4.47:1; viền ô nhập `#8C93A0` (3.09:1)
- Empty state có tiêu đề theo loại tài liệu + 3 chip gợi ý xếp dọc (theo host Writer/ET/WPP);
  bubble có "đuôi", nhận Tab/Ctrl+C và menu Sao chép; dòng tool activity (tick xanh / x đỏ +
  nhãn tiếng Việt + mã action, tooltip là dòng log gốc); typing 3 chấm tôn trọng reduced motion
- Composer tự cao 2–5 dòng, Ask chỉ bật khi có chữ, khoá kèm hướng dẫn khi chưa cấu hình AI;
  footer có trạng thái + chấm "đang chạy" + link **Dừng** / **Chèn trả lời**; thẻ lỗi có
  **Thử lại** / **Mở Cài đặt**; trạng thái xong/lỗi được báo cho trình đọc màn hình
- `PromptBox`: edit multiline không phát `EN_CHANGE` khi text đặt bằng `WM_SETTEXT` (UIA,
  automation) — tự báo `TextChanged` để Ask bật

### Fixed
- **Word crash (AV `wwlib.dll`) khi nhiều thread cùng gọi COM**: heartbeat đọc
  `ActiveDocument` đúng lúc `ai.ask` đang ghi → cuộc gọi thứ hai được Word dispatch
  reentrant. `Bridge/ComGate.cs` tuần tự hoá mọi truy cập object model của bridge (Pump,
  agent của pane, heartbeat, poller); heartbeat/poller bỏ qua lượt khi cổng bận
- **Agent chạy mãi không kết thúc / kết thúc âm thầm** (BUG-1): trần 5 phút cho cả lượt
  (`LlmClient.AgentTimeoutMs`), hủy qua `CancellationToken` (abort request LLM đang chờ,
  không chạy thêm tool); log kết quả ghi ngay trên worker; `BeginInvoke` thất bại và lỗi
  trong callback UI đều được log thay vì `catch {}`; `ai.ask` log từng tool (`ai.ask progress:`).
  Không tái hiện được treo trên build hiện tại (cùng prompt bảng điểm: 16.9s, 2 tool calls)
- **Tài liệu ẩn** (BUG-2): cửa sổ `OpusApp` ẩn không title là của Word, có từ lúc khởi động
  (trước khi tạo pane). Log trạng thái tài liệu/cửa sổ ở `OnConnection`, `OnStartupComplete`,
  lệnh bridge đầu tiên và trong `app.info.state`; `writer.*` kích hoạt tài liệu đang hiển thị
  nếu `ActiveDocument` không có cửa sổ visible
- **Chip gợi ý không được layout lúc mở pane** (BUG-3): empty state đi qua `ChatList.AddBlock`
- **Task pane hẹp trên WPS** (BUG-6): pane đo lại độ rộng sau khi host layout, quy đổi đơn vị
  CTP theo tỉ lệ đo được và nới về 360px; WPS 12.1.0.28485: 250 → 360px
- `ui.askpane` tạo CTP trên UI thread chính (trước đây chạy trên thread HTTP nên RCW của pane
  thuộc apartment khác)
- Danh sách chat không cuộn hết xuống cuối (AutoScroll bỏ qua Padding đáy; scrollbar xuất hiện
  làm bubble cao thêm): `AutoScrollMargin` + giữ vị trí cuối khi đo lại
- **Build làm mất DLL** (BUG-8): `build.ps1` hỏi Restart Manager tiến trình nào đang lock DLL/EXE
  (assembly .NET không hiện trong `Process.Modules`), dừng kèm tên + pid hoặc `-Kill`; biên dịch
  vào `.stage` rồi mới chép đè; dọn `<guid>_WpsAiBridge.dll` còn sót

### Ideas (chưa làm)
- Event stream là nền cho timeline/transaction sau này: thêm event `change` (hash nội dung
  theo đoạn) để agent biết user vừa sửa gì mà không cần đọc lại cả tài liệu
- Poll-diff đủ cho selection/document; khi cần độ trễ thấp hơn có thể chuyển Writer sang
  COM event sink (`WindowSelectionChange`) nhưng phải giữ đường poll cho WPS


### Added — Ask AI dạng Task Pane + AI agent thao tác live
- **Ask AI = task pane dock trong app** (`ICustomTaskPaneConsumer`/`ICTPFactory`
  — cùng API cho Microsoft Office và WPS), fallback cửa sổ nổi nếu host không
  hỗ trợ; pane chat có Ask / Insert reply / Settings, transcript từng bước
- **Agent mode**: LLM **tool-calling** (OpenAI + Anthropic) gọi tool
  `office_action` → thực thi `writer.*/et.*/wpp.*` **trực tiếp lên tài liệu đang
  mở** (real-time trước mặt user); mỗi action Word = 1 Ctrl+Z (UndoRecord);
  tự fallback chat thường nếu provider không hỗ trợ tools
- Bridge command mới **`ai.ask {prompt}`** — gọi agent từ bên ngoài qua HTTP
- Debug notes (ghi lại để đời):
  - `CTPFactoryAvailable` tham số phải khai báo `object` (marshal trực tiếp
    `ICTPFactory` fail âm thầm — host vẫn "gọi" nhưng QI lỗi)
  - `ICustomTaskPaneConsumer` là **dispinterface**: khai báo `IUnknown` →
    host gọi vtable slot IDispatch → **crash WINWORD** (0xc0000005) ngay sau
    `OnAddInsUpdate`, trước `GetCustomUI`
- Đã verify trên Word 2024: prompt *"Soạn cho tôi một mẫu đơn xin việc"* →
  4 tool calls (getText → insertStyledText header → heading → insertStyledText
  thân đơn), 41s, docx 1326 ký tự tiếng Việt chuẩn Unicode

### Added — MCP server Word (`tools/word-mcp`) và PowerPoint (`tools/ppt-mcp`)
- `word-mcp` (19 tools): làn file python-docx (profile/get_text/find_text/
  extract_table/create — atomic save) + làn live qua bridge (health, command,
  read/type/styled text, format_selection, heading, insert_table/image/hyperlink,
  replace_all, export_pdf, undo, save) — mặc định Microsoft Word, `app="wps"` cho WPS
- `ppt-mcp` (15 tools): làn file python-pptx (profile/get_text/create/add_slide_file)
  + làn live (health, command, list/add slide, add_text, add_image, add_table,
  set_notes, delete_slide, export_pdf, save)
- Mỗi server: venv riêng, run.cmd, unittest (word 5/5, ppt 4/4), MCP protocol
  roundtrip pass trên Word/PowerPoint thật
- **fix**: `writer.undo` dùng `Document.Undo` (Word object model không có
  `Application.Undo`); undo theo UndoRecord hoạt động end-to-end — `undo x2`
  hoàn tác heading + table đúng 2 bước về text gốc

### Added — Live command pack v2 (tham khảo ppt-mcp / word-mcp-live)
- **Word** (10 lệnh mới): `insertStyledText`, `formatSelection`,
  `setParagraphAlignment`, `insertTable` (kèm dữ liệu + style), `insertPageBreak`,
  `insertImage`, `insertHyperlink`, `heading` (Heading 1-9, tự xuống dòng),
  `undo`, `exportPdf` — mọi thao tác ghi được bọc trong **Word UndoRecord**
  (mỗi action của AI = 1 bước Ctrl+Z, pattern từ word-mcp-live)
- **Excel** (4): `formatRange` (bold/italic/fontSize/fontColor/fillColor/numFmt/
  horizontal/wrap), `activateSheet`, `exportPdf`, `undo`
- **PowerPoint** (7): `addSlide` có `layout`, `addText` (fontSize/bold/color/
  align), `addImage`, `addTable` (kèm dữ liệu), `setNotes` (speaker notes),
  `deleteSlide`, `exportPdf`
- Đã kiểm chứng trên **Microsoft Office 2024 thật** (3 app chạy song song):
  Word chèn bảng/hyperlink/ảnh + PDF (48KB); Excel format + PDF (25KB) +
  saveAs đúng; PowerPoint slide/text/table/notes + PDF (27KB) + pptx (40KB);
  heading có paragraph break (`Tieu de\rThan bai.`)

### Fixed/Added — vòng 2 theo code review (#6 + minors)
- **Retry khi app bận**: request tự retry khi gặp `RPC_E_CALL_REJECTED` /
  `RPC_E_SERVERCALL_RETRYLATER` / `RPC_E_CALL_CANCELED` / `VBA_E_IGNORE`
  (0x800AC472 — Excel đang gõ ô) — tối đa 10 lần, backoff luỹ tiến ~5.5s,
  log mỗi lần retry; an toàn vì lỗi "call rejected" nghĩa là call chưa thực thi
- **Giá trị lỗi Excel**: `#DIV/0!` `#VALUE!` `#NAME?` `#REF!` `#NUM!` `#NULL!`
  trả về tên chuỗi thay vì số âm (probe thực tế trên Excel 16.0); `#N/A` trùng
  mã 0x800A07FA với ô trống qua `Value2` nên vẫn về `null` — giới hạn đã biết
- **Version đồng bộ**: `BridgeVersion` lấy từ assembly version (health trả
  `1.0.0` thay vì `0.1.0`)
- **DPAPI**: `LlmApiKey` mã hóa bằng `ProtectedData` (CurrentUser); key
  plaintext cũ vẫn đọc được (tương thích ngược); `SettingsForm` lưu qua
  `WriteSecret`; đã migrate key thật sang dạng mã hóa + `llm-test` gọi LLM
  qua key DPAPI thành công
- **Tests**: `tools/excel-mcp/tests/test_file_tools.py` — 9 unit test
  (roundtrip, show_formula, ragged write, format/table, sheet ops, DuckDB
  sandbox, locked-file không sót temp, csv) — chạy bằng
  `python -m unittest discover -s tests`

### Fixed — bảo mật + đúng đắn (theo code review)
- **CSRF**: bridge từ chối request có header `Origin`; `POST /cmd` bắt buộc
  `Content-Type: application/json` (chặn browser "simple request"); token
  ngẫu nhiên 32 hex tự sinh khi chạy `install.ps1`; `bridge.py` tự đọc token
  từ registry và gửi `X-Auth-Token`
- **et.writeRange**: dùng `Range.Resize[rowCount, colCount]` — ghi đúng ma trận
  khi truyền anchor 1 ô ("A1", trước đây Excel thật chỉ ghi được ô đầu); colCount
  lấy max qua các dòng (ragged rows được pad null thay vì cắt âm thầm)
- **Ribbon port**: nút Status / Copy API URL hiển thị đúng port theo host
  (truyền `IsOfficeHost` — Office hiện 4783x thay vì 4782x)
- `bridge.py`: `APP_PORTS` thêm `word/excel/ppt` (47831-47833); `wps_health`
  nhận tham số `port`
- **An toàn ghi file (excel-mcp)**: mọi thao tác ghi đi qua file tạm cùng thư
  mục + `os.replace` (atomic — lỗi giữa chừng không làm hỏng file gốc, vẫn giữ
  hint khi file bị khóa); docstring cảnh báo openpyxl có thể mất chart/image/pivot
- **Sandbox `excel_query`**: `SET enable_external_access=false` — SQL không thể
  `read_csv`/`COPY TO`/`INSTALL`/`ATTACH` (chống prompt injection đọc/ghi file
  tùy ý); đã test chặn cả 3 đường

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
