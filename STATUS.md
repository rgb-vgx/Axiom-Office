# Tình trạng dự án Axiom Office

Cập nhật: **26/10/2026** · nhánh `main` · đã push đến §2.12 (ba đợt 25/10: bớt ngữ cảnh, slide sơ đồ, tự đi lại dây).
Số liệu kiểm chứng ở §2.4 là lượt đo **01/10/2026**; số mới của đợt WPS ở §2.6, lượt đo lại toàn bộ ở §2.7, lượt đo
lại job Windows ở §2.8, đợt Office x64 + sửa định dạng văn bản ở §2.9 và ba đợt 25/10 ở §2.10–2.12.

Tài liệu này là ảnh chụp nhanh "đã làm được gì / chưa làm được gì". Chi tiết thiết kế nằm ở
[docs/architecture/core.md](docs/architecture/core.md) (Core + add-in) và [docs/architecture/libreoffice.md](docs/architecture/libreoffice.md) (LibreOffice);
hướng dẫn dùng ở [README.md](README.md); lịch sử thay đổi ở [CHANGELOG.md](CHANGELOG.md).

## 1. Năm thành phần và trạng thái

| Thành phần | Nền tảng | Trạng thái |
|---|---|---|
| **Add-in COM** (`src/AxiomOffice`, net48) | Word/Excel/PowerPoint + WPS, Windows | Chạy trên **Office 2019 x64** (24/10: Word đủ lifecycle, Excel e2e 6/6, LLM thật — §2.9). **Không hỗ trợ Office 32-bit**: mọi add-in .NET làm Office 32-bit crash (§2.9) |
| **Agent Core** (`core-go/`, Go) | Windows + Linux | **Đã thay bản .NET làm bản phát hành** (G1–G10), qua toàn bộ e2e của bản .NET |
| **MCP server** (`core-go/internal/mcpserver`, Go) | Windows + Linux | **Một bản duy nhất**, chạy bằng `AxiomOffice.Core.exe mcp`; 61/61 trên cả hai nền tảng (và 103/103 ở bộ parity sâu). Bản C# đã xoá; `AxiomOffice.Host.exe mcp` còn 232 dòng **chuyển tiếp byte** sang Core nên cấu hình cũ không phải đổi |
| **Extension LibreOffice** (Python UNO) | Windows + Linux (LibreOffice) | **Dừng hỗ trợ từ 25/10/2026**: không phát triển tiếp, không đưa vào plan mới. Lần đo cuối: 281/281 trên LibreOffice thật + wizard 20/20 với Core Go (01/10). Code và CI còn nguyên, gỡ là quyết định riêng |
| **Add-in WPS Linux** (`src/AxiomOffice.WPS`, JS ES5; Core phục vụ + bridge cổng 47861) | Linux (WPS 12) | Chạy trên WPS thật: 58/84 mục registry có handler (26 chưa: 25 lệnh Windows + `ui.setup` — `docs/architecture/wps-bridge.md` §6); ba tool lớn `slides_from_brief`/`slides_from_template`/`doc_design` chưa chạy trên WPS; ribbon 4 nhóm có icon; khung chat với menu ⋯ và 8 màn hình phụ; menu chuột phải *Sửa bằng AI* + khung chat mini (§2.6) |
| **Wizard thiết lập** | Windows (WinForms) + Linux (awt) | Bản Linux đã chạy thật (20/20); **bản Windows đã dùng thật trong Word** (24/10: lưu cấu hình + `Thử ngay` OK — §2.9) |

## 2. Đã làm được

### 2.1 Agent Core viết lại bằng Go (`core-go/`) — G1 → G10

Bản Go dùng **chung hợp đồng** với bản .NET: cùng Core API v1, cùng `core.json`, cùng `core.db` (schema 4,
cùng nguồn cấu hình (`AXIOM_*` → HKCU/DPAPI hoặc `~/.config/axiom-office/config.json` + libsecret), cùng tên
mutex một-phiên-bản. Nhờ vậy add-in/extension/MCP không phải đổi gì.

| Mốc | Nội dung |
|---|---|
| G1 | Cấu hình, `core.log`, `core.json`, một-Core-mỗi-người-dùng, chọn port, guard (Origin 403 / token 401 / Content-Type 415), `/health`, `/v1/admin/shutdown`, migrate `core.db`, model client + codec OpenAI/Anthropic, `/v1/setup`, `/v1/llm/test`, `/v1/llm/models` |
| G2 | `/v1/runs` + SSE + hủy + xác nhận, orchestrator, bridge client, session registry, `office_action`, policy, audit, hội thoại |
| G3 | Skills 3 tầng: `load_skill`, `read_skill_file`, `GET /v1/skills`, `POST /v1/skills/reload`, quét lại thư mục |
| G4 | Memory dài hạn: chống trùng (hash + cosine 0.96), liên kết chuyển đổi, lịch sử, ghim/hạn dùng/xoá mềm, truy hồi bm25 + entity boost + embedding, trích xuất sau mỗi lượt, toàn bộ `/v1/memory`, tool `remember`/`recall` |
| G5 | MCP client (stdio + Streamable HTTP, server built-in `office` = `AxiomOffice.Core.exe mcp`, xác nhận tool ngoài/ghi đè file), QA thị giác `look_at_document`, `GET /v1/mcp` |
| G6 | Đóng gói: `build.ps1` build Core Go thành `AxiomOffice.Core.exe`; `package.sh` đóng gói `core/AxiomOffice.Core`; CI chạy cả hai |
| G7 | **Vá lỗ hổng**: Core dừng khi đang chạy lượt → hủy lượt trước khi đóng HTTP server (trước đây agent có thể sửa dở tài liệu) |
| G8 | Phủ codec Anthropic (chạy thật) và embedding (cosine + tìm theo nghĩa) |
| G9 | Phủ tóm tắt hội thoại dài (chỉ khi vượt mốc 20 tin nhắn) |
| G10 | Phủ nút **Dừng** (`/cancel`), `interactive=false` (`ai.ask`), PATCH memory, xoá cứng toàn bộ |
| G11 | Phủ MCP transport **HTTP** (`mcp.json` dạng `url`): session id, response JSON và SSE |

Kích thước: Core Go **11,9 MB** (bản .NET self-contained: 51,6 MB), không cần runtime trên máy người dùng.

### 2.2 MCP server bản Linux chuyển sang Go (`core-go/internal/mcpserver/`) — G12

Bản C# đa nền tảng (`src/AxiomOffice.Mcp`, net10 — biên dịch lại `src/AxiomOffice.Host/Mcp/*.cs` với ký
hiệu `PORTABLE`) đã được thay bằng subcommand `AxiomOffice.Core mcp`; dự án .NET 10 cùng hai lớp shim
`Compat/` đã bị xoá, và các nhánh `#if PORTABLE` trong mã C# dùng chung cũng được dọn (bản C# còn đúng
một cấu hình: net48 cho Windows).

| Phần | Nội dung |
|---|---|
| Giao thức | JSON-RPC 2.0 mỗi message một dòng qua stdio, stdout chỉ dành giao thức, batch, `-32700/-32600/-32601/-32602`, `initialize`/`ping`/`tools/list`/`tools/call` |
| Engine OOXML | `ooxml` (zip/part/rels/content-types, chỉ ghi lại part bị sửa), `xlsx` (shared strings, style, bảng, công thức), `docx`, `pptx`, `sheet` (A1, CSV/TSV) |
| 20 tool file | `doc_*` (5), `excel_*` (11), `ppt_*` (4) — tên và tham số giống hệt bản C# |
| 30 tool live | `word_*` (14), `ppt_*` (11), `wps_*` (4) + `office_sessions`; gọi bridge qua HTTP, cổng tra từ cấu hình |
| Nguồn dùng chung | `catalog/live-commands.json` → sinh `core-go/internal/mcpserver/livecommands_gen.go`; template `default.docx`/`default.pptx` giữ một nguồn, Go giữ bản sao có `--check` trong CI |
| Đo lệch | `tests/mcp-host/oracle_diff.py` chạy cùng kịch bản trên bản Go và bản C#, so từng bước |

Hai chỗ cố ý giữ nguyên hành vi của bản C# dù trông như lỗi: `ReplaceSheetReference` thay dạng **có**
nháy bằng tên mới **không** nháy khi tên đơn giản, và `IsDateFormat` chỉ xét phần trước dấu `;` đầu tiên.
Cả hai đều nằm trong hợp đồng mà bộ parity kiểm tra, nên sửa chúng sẽ làm lệch bản Windows.

### 2.3 Wizard thiết lập cho người dùng không chuyên

- **Core API**: `GET /v1/setup`, `POST /v1/llm/test`, `GET /v1/llm/models` + `Setup/LlmErrors.cs` dịch lỗi sang
  tiếng Việt kèm gợi ý sửa. Khoá API không vào log/response.
- **Linux**: `axiom/setup.py` (máy trạng thái, test được không cần LibreOffice) + `setupwizard.py` (dialog awt),
  mục menu **Thiết lập…**, link header, tự mở một lần khi chưa cấu hình.
- **Windows**: `Ai/SetupWizardForm.cs` + `Ai/CoreSetup.cs`, ribbon **Thiết lập…** và **Cài đặt nâng cao…**,
  link header pane, tự mở một lần, bước 5 có **Thử ngay** (chạy một lượt thật qua Core).
- Câu chữ + preset nằm ở một nguồn `catalog/setup.json` → sinh ra `SetupCatalog.cs` (C#, dùng chung add-in và
  Core), `axiom/setup_catalog.py` (Python) và `core-go/internal/setup/catalog_gen.go` (Go); CI kiểm tra không lệch.

### 2.4 Kiểm chứng (số liệu 01/10/2026 — chưa đo lại; đợt WPS ở §2.6)

| Bộ kiểm thử | Kết quả |
|---|---|
| e2e Core — **bản Go** (14 phần, Windows) | **155/155** |
| e2e Core — **bản .NET** (đối chiếu) | **155/155** |
| e2e trên **Linux** đúng lệnh CI (13 phần) | **143/143** |
| LibreOffice thật (gói mới, Core Go) | **159/159** |
| Wizard thiết lập (dialog thật, X ảo) | **20/20** |
| `go test ./...` (core-go) | 18 gói |
| `tests/lo` · MCP parity | 106 |
| `test_mcp_portable.py` — binary Go (`AxiomOffice.Core mcp all`) | **61/61** |
| `test_mcp_portable.py` — `AxiomOffice.Host.exe mcp all` (Windows; nay là cửa CHUYỂN TIẾP sang Core, bản C# đã xoá) | **61/61** |
| `test_mcp_host.py` — parity sâu với python-docx/openpyxl/python-pptx, binary Go | **103/103** |
| `oracle_diff.py` — Go ↔ C# trên cùng kịch bản (docx/pptx/excel) | 61/62 (1 khác là câu chữ thông báo `parquet`, cố ý) |
| `--real-llm` (model thật trong HKCU) | **4/4** — nạp đúng skill (`bao-cao-thang`, `bang-diem`, `van-ban-hanh-chinh`), câu "in đậm" không nạp skill thiết kế |
| Đối chiếu khoá API DPAPI với bản .NET | cùng plaintext (dài 35, sha256 `bec24e97c4893251`) |
| libsecret (keyring Linux, qua `secret-tool` giả) | đọc được khoá; thiếu khoá → coi như chưa cấu hình |
| `systemd --user` (Linux) | `enabled` + `active`, `/health` OK, đúng port 47840 |
| Đóng gói | Windows: `bin\Release` có Core Go 1.0.0 + DLL + Host + skills; Linux: tar.gz 39 MB với `core/AxiomOffice.Core` là ELF chạy độc lập |

### 2.5 CI (cập nhật 20/10/2026)

`.github/workflows/ci.yml`: job **linux** không còn cài .NET — chạy unit test extension, kiểm ba file sinh
không lệch nguồn, `go vet`/`go test`, e2e **15 phần** (đã gồm `mcp`, vì Core tự làm MCP server, và `verify`), unit
test add-in WPS (Node), e2e kênh WPS + pane (`make test-wps-e2e`), bộ MCP portable nhắm vào binary Go, trình cài
`install.sh`, rồi cài LibreOffice chạy bộ live + wizard và đóng gói tarball. Job **windows**
chạy unit test, `go test`, build net48, **toàn bộ e2e của bản Go** (kèm `AxiomOffice.Host.exe` như gói phát
hành) và bộ MCP portable nhắm vào `AxiomOffice.Host.exe mcp`. Cả hai job **xanh** (đợt 24/10 — §2.8).

### 2.6 Đợt 18–20/10/2026: WPS Office for Linux

- **Khoá vùng bôi đen** (`writer.lockSelection`/`unlockSelection`): bôi đen rồi hỏi thì AI chỉ sửa trong đoạn đó;
  Core khoá trước mỗi lượt, mở khoá ở `finish()`.
- **Ribbon và khung chat đủ chức năng như bản Windows**: kênh `/wps/v1/*` (`routeBoth` cho setup, llm, skills,
  memory, audit, conversations; riêng WPS: status, exec, config, log), các màn hình phụ của pane, ba nút *Soát*.
- **Trông giống MS Word**: bảng màu Fluent `#0F6CBD` cho cả ba nền tảng; trả lời AI không bong bóng; menu ⋯; màn
  hình *Hội thoại trước…*; icon ribbon là tệp SVG (data URI làm WPS vẽ dấu `?`).
- **Sửa bằng AI**: 9 mục câu lệnh nằm trong Core (`agent/quickactions.go`, `GET /v1/quick-actions`,
  `POST /v1/runs {quickAction}`); menu chuột phải trên WPS mở **khung chat mini** (`mini.html`,
  `Application.ShowDialog`); `options.requireSelection`; `QuickEditRule` (sửa ngay, không hỏi lại).
- **Đã thử trên WPS thật**: icon ribbon, menu chuột phải (chạy trọn tới Core), khung chat mini (người dùng: "khá ổn").
- **Số liệu 20/10/2026**: `make test-wps` 181 · `make test-wps-e2e` 94 · bridge e2e 36 · Core e2e 169 · `go test` xanh.
  (Đo lại 22/10: `make test-wps` **189**, xem §2.7.)

### 2.7 Đợt 20–22/10/2026: sửa lỗi, refactor và đo lại

**Sửa lỗi (20/10)** — chi tiết ở `CHANGELOG.md`:

- WPS: ngăn undo của ET và kho định dạng điều kiện khoá theo **workbook + sheet** thay vì tên sheet trần (`Sheet1`
  lặp lại ở mọi sổ); `closeAll` tạo thư mục cứu hộ trước khi ghi, `exportPdf` báo lỗi khi không có `FileSystem`.
- LibreOffice: `Env` nhận `params` và chọn đúng tài liệu qua `documents.find_open_path` (trước đây luôn lấy tài liệu
  active); client Core gom về `core.call`, `wait_for_run` có `timeout` và **huỷ lượt** khi hết giờ.
- Core: `AddBatch` dùng `SAVEPOINT` từng mục (trước đây mục lỗi giữa lô vẫn để lại các mục trước); `readJSON` chặn
  8 MB; `round1` dùng `math.Round` (số âm trước đây làm tròn sai).
- CI: job Windows hết vỡ console cp1252 khi in tên lệnh tiếng Việt; job Linux cài `requirements.txt` cho oracle.

**Refactor (20/10)**: `Execute` tách `prepareExecution`/`buildCallbacks`/`applyResult`; `wpsbridge.go` tách
`auth.go`/`http.go`/`session.go`; CSS/JS pane và mini WPS tách khỏi HTML, helper DOM dùng chung `pane-dom.js`;
LibreOffice đi một đường chạy lệnh duy nhất, bỏ mã chết.

**Đo lại toàn bộ tại máy (22/10/2026)**:

| Bộ | Kết quả |
|---|---|
| `tests/lo` (`make test-lo`) | **225 ok**, 1 skip |
| `make test-wps` | **189 pass** |
| `make test-core` — e2e Core Go | **169 passed, 0 failed** |
| `go test ./...` (core-go) | 24 gói ok (4 gói không có test) |
| `make check-gen` | ba file sinh khớp nguồn, `VERSION` khớp 4 chỗ, i18n 0 thiếu/thừa |
| `gofmt` + `go vet` + `ruff` + `eslint` | sạch |
| `tests/lo/test_impress_checks.py` (mới) | **8/8**; trên `checks.py` trước bản vá 13/10: 4/8 đỏ |
| `tests/lo/test_bridge_env.py` (sửa 22/10) | **3/3**; trước khi sửa: đỏ khi mô phỏng `ntpath` (Windows) |

### 2.8 Đợt 24/10/2026: sửa CI Windows

Job `windows` trước đó đỏ ở **hai** chỗ (đều do nguyên nhân riêng, không liên quan đến nhau):

- **`go test ./...` đỏ**: `TestWPSConfigWrite` giả định nguồn cấu hình là file `config.json`, nhưng
  `config.DefaultSource()` trên Windows là **HKCU** → handler `POST /wps/v1/config` ghi registry nên không có
  file để đọc lại. Sửa: `api.Deps.ConfigSource` (nil = `DefaultSource()` như production) + test inject
  `config.JSONSource` trỏ file tạm → test chạy giống nhau trên hai nền tảng; **test cũ còn ghi đè cấu hình thật
  của máy dev trong registry** (`LlmModel`, `VisualQaEnabled`) — nay không còn; kiểm tra quyền `0600` chỉ áp
  dụng cho non-Windows.
- **Build net48 đỏ**: `src/AxiomOffice/Ai/AskAiPane.cs` có một dấu `}` thừa (dòng 1128, sót từ lần thay khối
  `switch` ở commit `d808b28`) khiến class `AskAiPane` đóng sớm (`CS1519/CS1513/CS1022`) — add-in Windows
  **không biên dịch được** (kể cả `build.ps1` local). Đã bỏ dấu `}` thừa.

**Đo lại job `windows` tại máy (24/10/2026, cùng lệnh với CI)**:

| Bước CI | Kết quả |
|---|---|
| Unit test extension LibreOffice | **225 ok**, 1 skip |
| `go test ./...` (core-go) | xanh (trước đây đỏ 1 test) |
| Build add-in + Host (net48) | OK (trước đây lỗi cú pháp C#) |
| e2e Core (binary Go) | **169 passed, 0 failed** |
| `test_mcp_portable.py` (binary Go) | **61 passed, 0 failed** |
| `scripts/generate_mcp_commands.py --check` | khớp (khối sinh tự động trong `AskAiPane.cs` không đổi) |

~~Còn thiếu: job `windows` chưa chạy `make lint` / `make check-gen` / test WPS (Node).~~ Đã thêm ngày
24/10/2026 (§2.9): job `windows` gọi `build.ps1`, chạy check-gen + i18n + ESLint + test WPS (Node). Ruff/gofmt/
staticcheck vẫn chỉ ở job `linux` (không phụ thuộc nền tảng).

### 2.9 Đợt 24/10/2026 (tiếp): Office x64, LLM thật và sửa định dạng văn bản

- **Office 32-bit: không hỗ trợ.** Add-in tối giản chỉ có `IDTExtensibility2` (build cả AnyCPU lẫn x86)
  cũng làm Word/Excel/PowerPoint crash trong `mso98win32client.dll` (`0xc0000005`, offset `0x002152ad`)
  ngay sau `OnAddInsUpdate`; Office tự đưa add-in vào `Resiliency\DisabledItems`. Đã bỏ các thay đổi
  32-bit (`Wow6432Node`, `-Platform anycpu`); README vẫn yêu cầu Office x64.
- **Office x64 (2019, 16.0.14026.20302):** Word nạp đủ lifecycle (`OnConnection → OnAddInsUpdate →
  CTPFactoryAvailable → GetCustomUI → OnRibbonLoad → OnStartupComplete`), không crash; Excel e2e
  `--office` **6/6**; tài liệu thật đổi đúng.
- **LLM thật** (endpoint LAN + `ocg/beam-free` qua wizard): 3 lượt *"Soạn đơn xin việc vị trí kế toán"*
  hoàn tất (lượt cuối 36,3 s, 12 vòng, 10 tool, tự kiểm chứng); memory trích 1 fact.
- **Sửa lỗi định dạng** (phát hiện từ `Đơn xin.docx`: 1 đoạn, toàn bộ bold + gạch chân + xanh, căn giữa):
  - `writer.insertStyledText` (C#/WPS/LO): `\n` = đoạn thật; **reset tường minh** bold/italic/underline/
    màu; thêm `alignment` áp từng đoạn; khối có `alignment` tự ngắt đoạn sau (`break`).
  - `writer.appendText`: cùng reset/alignment, tự mở đoạn mới khi đoạn cuối có chữ.
  - Skill `van-ban-hanh-chinh`/`the-thuc-van-ban`: bỏ `setParagraphAlignment` sau từng dòng, dặn `\n`
    và không gõ dòng trống.
  - `CoreClient`: lỗi callback không còn che lỗi lượt chạy; log kèm stack.
- **Đo lại:** `make test-wps` **201**; `tests/lo` **225 OK**; `go test ./...` xanh; `check-gen` + i18n
  khớp; e2e Excel **6/6**. Test định dạng tất định trên Word: 12 đoạn, đúng căn lề, 0 màu, 0 đoạn rỗng.
- **Bảng dàn trang + `restyle` (Word):** `formatTable {borders:false}` bản C# nay đặt tem `AxiomLayout`
  (`Table.Title`) như LibreOffice/WPS; `checkTables` trả `layout` và miễn `empty-cells`. `restyle` nhận văn
  bản hành chính (có quốc hiệu): chữ đen, giữ khoảng trống ký, không kẻ khung bảng dàn trang.
  `tests/live/test_writer_format.py` **19/19**.
- **CI Windows** gọi `build.ps1` (bỏ lệnh `csc` chép tay), thêm check-gen/i18n/ESLint/test WPS (Node); test
  LibreOffice chạy sau build nên đối chiếu registry C# không còn bị skip.
- **Còn lại:** model vẫn có thể tự dùng dòng trống (`\n\n`) — tạo đoạn rỗng (lượt thật cuối: 14 đoạn);
  nội dung và định dạng đúng, chỉ là khoảng cách. Live suite còn check *"mọi lệnh trong registry đều có
  test"* fail **từ trước**: 24 lệnh (`et.addChart`, `wpp.addGrid`, `writer.restyle`…) chưa có ca live.

### 2.10 Đợt 25/10/2026: trả token cho agent — bớt vòng thừa, bớt tool không dùng, đánh dấu cache

Nguồn: báo cáo đo 26 lượt thật (24–25/10, PowerPoint, `ocg/beam-free`, 212 vòng, 1.541.217 token đầu vào) —
~71% đầu vào là tiền tố cố định (tools + system prompt), ~25% là các vòng sau lần sửa tài liệu cuối; chỉ dẫn
đầy đủ ở `docs/research/giam-ngu-canh-gui-lai-agent.md`. Chi tiết kỹ thuật từng mục: `CHANGELOG.md` (Unreleased).

| Việc | Nội dung |
|---|---|
| `docStateGuard` | Lệnh đọc y hệt khi tài liệu chưa đổi → Core không chạy lại tool, trả `"unchanged": true` ("kết quả ở trên vẫn đúng"); revision tăng mỗi khi ghi thành công, kết quả đọc gắn với revision lúc đọc |
| `verifyGuard` | Đã có bằng chứng sau lần sửa cuối (lệnh đọc/kiểm ở phiên bản hiện tại, hoặc lệnh ghi tự báo `Verifies`) → lượt tính `Verified` ngay, không thêm vòng nhắc; ghi mới sau đó thì được nhắc như thường |
| `AppMCP` | Server built-in `office` lọc theo app: `doc_*` cho Word, `excel_*` cho Sheets, `ppt_*` cho Slides + 8 lệnh đọc của app khác; lệnh ghi của app khác bị bỏ (~8k ký tự tiền tố/vòng — mục 3 báo cáo); server `mcp.json` của người dùng giữ nguyên, thứ tự tool giữ nguyên |
| Anthropic cache | `cache_control: ephemeral` ở khối system cuối (tools + system thành một tiền tố cố định) và cấp request (`automatic` — tự đánh dấu phần đuôi mới nhất mỗi vòng); các nhà kiểu OpenAI tự cache nên phần đó không đổi |
| `reasoning_content` | Non-stream (`Parse`) tách `turn.Reasoning` rồi cắt khỏi `Raw` (DeepSeek trả 400 nếu gửi lại; đường streaming vốn không kèm) |
| `StopRule` | System prompt thêm quy tắc dừng: đọc lại đã đúng thì trả lời và kết thúc; không quét lại cả bài khi tài liệu không đổi, không lặp phép kiểm đã qua; chỉ kiểm lại khi sắp ghi thêm, một phép kiểm vừa thất bại, hoặc phần đó chưa từng kiểm (công thức tư vấn ChatGPT 25/10/2026) |
| Đo từng vòng | `RoundStat` + `AgentCallbacks.RoundFinished`; core.db **schema 4** (`tool_calls.result_chars`, `runs.cached_tokens`, bảng `run_rounds`); dòng finish thêm `no-tool rounds: ...` |
| Bench "cài sẵn lỗi" | `s25-loi-cai-san` (`tests/bench/slides/briefs.json` version 2): `seed` chạy lệnh bridge cài số sai trên bài mới trước lượt, `mustRemove` chấm khoá cụm lỗi phải biến mất — đo việc model đọc tài liệu có sẵn để sửa |

**Kiểm** (25/10/2026): `go test ./...` 24 gói xanh; **e2e Core 170 passed, 0 failed** (đợt §2.9 là 169 — tăng
1 phép kiểm `cache_control`; cập nhật 2 phép characterization: system Anthropic thành block có `cache_control`
và ca MCP chạy phiên Word vì `doc_` giờ là tool của Word); bench slides **9/9**; `make check-gen` khớp;
`gofmt` + `go vet` + `ruff` sạch.

### 2.11 Đợt 25/10/2026 (tiếp): slide sơ đồ trên đề thật

Đề "Thiết kế hệ thống AI Agent cho ngân hàng" (10 slide, `tests/live/tasks/ngan-hang-ai-agent.md`) chạy bằng
`tests/live/run_ai_task.py`: 20 → 42 → **30 vòng** qua các lần sửa; bản cuối 10 slide, `checkLayout` 0 lỗi, sơ đồ kiến
trúc không dây cắt hộp. Sửa: sơ đồ lồng trong khoá `diagram`, `replace: N` sửa đúng slide lỗi, đường nối tránh tiêu
đề/chú thích, `SAME_SPEC_SAME_ISSUES`, lỗi bố cục vào `core.log`, xoá slide trống không hỏi, nhãn lỗi của khung chat.
**Bàn giao chi tiết + việc tiếp: `docs/plans/20-c0-ngu-canh-va-slide.md`.**

### 2.12 Đợt 25/10/2026 (tiếp): sơ đồ tự đi lại dây

- **`slides_from_brief` tự sửa lỗi đi dây**: giữ tên từng đường nối theo cạnh; sau `wpp.checkLayout` của slide, lỗi
  `connector-crossing`/`connector-overlap` thuộc đường do tool vẽ → vẽ đường mới bằng ứng viên kế tiếp
  (`slidelayout.DiagramAlternatives`, tăng `offset` với đường vòng, bỏ qua đường đã thử), xoá đường cũ, kiểm lại;
  tối đa **2 lần mỗi đường**. Kết quả trả `connectorsRerouted` để model không dựng lại slide.
- **Đoạn nguồn là vật cản** của dây vòng (trước đây dây dưới hàng cuối có thể cắt qua dòng nguồn).
- **Kiểm:** `TestDiagramAlternativesSkipCurrentAndTried`, `TestSlidesFromBriefReroutesConnectorPowerPointDrawsAcrossBox`,
  `TestSlidesFromBriefStopsReroutingAfterTwoAttempts`; `go test ./...` 24 gói xanh; `go vet ./...` sạch.
- **Tài liệu:** mô tả `wpp.checkLayout` thêm `connector-*` (catalog, README, kiến trúc); WPS ghi rõ không có hai
  loại này; skill `trinh-bay-chuyen-nghiep` nhắc tool tự vẽ lại dây.
- **Còn:** chưa chạy đề ngân hàng 3 lần trên PowerPoint thật để đạt tiêu chí của plan 20 §5.1 (máy đã cài PowerPoint
  nhưng chưa cấu hình LLM/Core để chạy lượt thật).

## 3. Chưa làm / chưa kiểm chứng

| Việc | Tình trạng | Ghi chú |
|---|---|---|
| **Wizard Windows bấm tay trong Word/Excel/WPS** | **Đã (24/10/2026, trong Word x64)** | Lưu cấu hình + `Thử ngay` OK (§2.9) |
| e2e `--office` (Office thật + add-in, `test_office`) | **Đã chạy 24/10/2026** | Excel x64: 6/6 (§2.9) |
| Trích xuất memory với **LLM thật** | **Đã (24/10/2026)** | Lượt thật trích 1 fact (`core.log`: `1 added`) |
| **Office 32-bit** | **Không hỗ trợ** | Add-in .NET làm Office 32-bit crash `mso98win32client.dll`; kiểm bằng add-in tối giản (§2.9) |
| Đọc/ghi **HKCU** bằng test tự động trên Windows | Một phần | Đối chiếu tay: Core Go đọc đúng endpoint/model/token/DPAPI của bạn; test tự động hiện dùng `AXIOM_*`. Từ 24/10 test cũng **không còn ghi vào HKCU thật** (nguồn cấu hình inject file tạm — §2.8) |
| **Push remote** | Đã push 26/10/2026 | Đợt 18–22/10 (11 commit), sửa CI Windows 24/10 (§2.7–2.8) và ba đợt 25/10 — "trả token cho agent" (§2.10), slide sơ đồ (§2.11), tự đi lại dây (§2.12) — đều đã lên `origin/main` |
| CI job **windows** | **Xanh (24/10/2026)** | Trước đó đỏ ở bước `go test` (`TestWPSConfigWrite` giả định nguồn cấu hình là `config.json`, nhưng trên Windows `DefaultSource()` là HKCU) và ở bước build net48 (`}` thừa trong `AskAiPane.cs`). Đã sửa cả hai — xem §2.8 |
| Nghiệm thu plan 20 §5.1 trên đề thật | Chưa | Code + test tự đi lại dây đã xong (§2.12); cần chạy đề ngân hàng 3 lần trên PowerPoint thật, lần dựng đầu không còn `connector-*` |
| Merge `feat/setup-wizard` + `feat/core-go` vào `main` | **Đã xong** | Fast-forward, lịch sử thẳng |
| Nền tảng khác | Chưa | Chỉ build được Windows (amd64) và Linux (amd64/arm64); chưa có macOS hay Windows ARM |
| Embedding với máy chủ thật | Chưa | Chỉ kiểm bằng máy chủ giả trả vector điều khiển được |
| Lỗi `install.sh` xoá `core.db` | **Đã sửa 20/10/2026** | Cài lại / nâng cấp chuyển `core.db*` sang bản mới; `--uninstall` không `--purge` giữ lại; `tests/linux/test_install_sh.py` +2 ca (đỏ trước khi sửa). Gói phát hành trước ngày này vẫn mang `install.sh` cũ |
| *Sửa bằng AI* / khung chat mini trên Windows và LibreOffice | Chưa | Mới có trên WPS Linux; Core đã sẵn. LibreOffice cần **khoá vùng bôi đen** trước (chưa có) |
| Add-in WPS: 26 mục registry chưa có handler (25 lệnh Windows + `ui.setup`) | Chưa | `docs/architecture/wps-bridge.md` §6; hậu quả: `slides_from_brief`, `slides_from_template`, `doc_design` không chạy trên WPS; thiếu `app.screenshot`/`wpp.renderSlide` nên chưa có QA thị giác |
| Bộ nhớ AI trên WPS: sửa / ghim / đặt hạn | Chưa | Màn hình WPS có xoá tạm, khôi phục, xoá vĩnh viễn, lọc phạm vi |
| Bấm từng nút ribbon WPS trên WPS thật | Chưa | Mới đo qua Core thật + add-in giả; menu chuột phải và khung mini đã thử thật |
| *Sửa bằng AI* với model thật trên nhiều trường hợp | **Đã chạy 3 lượt thật (24/10/2026)** | Soạn đơn xin việc: 36,3 s / 12 vòng / 10 tool, tự kiểm chứng; đã sửa lỗi định dạng phát hiện từ file kết quả (§2.9). Vị trí cửa sổ `ShowDialog` vẫn không chỉnh được |

## 4. Ghi chú hành vi (khác bản .NET đã bỏ ngày 01/10/2026)

Ghi ở [core-go/README.md](core-go/README.md); tóm tắt:

1. Bản Go **giữ luôn listener** của port tìm được (bản .NET thả port rồi Kestrel bind lại — có khe để tiến
   trình khác chiếm).
2. Bản Go **dừng êm khi nhận SIGTERM** (dùng cho `systemd --user`).
3. Bản Go **hủy lượt đang chạy trước khi đóng HTTP server** (bản .NET hủy sau khi server đã dừng) → pane còn
   nhận được `run.cancelled`.
4. `conversationId` gửi kèm khoảng trắng được cắt bỏ (bản .NET coi là hội thoại mới).
5. `Normalize` của memory dùng bảng bỏ dấu tiếng Việt tường minh (giống bản .NET); chữ Latin có dấu khác chỉ
   được bỏ dấu nếu văn bản đã ở dạng phân rã sẵn.
6. **Không còn agent in-process** (bỏ 25/10/2026, trước đó là opt-in `LegacyAgent=1`): đường này bỏ qua
   policy/xác nhận/audit/memory của Core. Core lỗi thì pane báo lỗi; `ai.ask` ở chế độ companion
   (`Host.exe word|excel|...`) trả `ok:false` vì Core chỉ chạy lượt từ add-in.

## 5. Lệnh hay dùng

```powershell
scripts\build.ps1                 # add-in + Host + Agent Core (Go) vào src\AxiomOffice\bin\Release
scripts\install.ps1               # đăng ký add-in (HKCU), in cấu hình MCP
scripts\package.ps1               # zip phát hành
```

```bash
cd core-go && go test ./... && go build -o axiom-core.exe ./cmd/axiom-core     # Core Go
AXIOM_E2E_CORE_EXE=<binary> python tests/core/test_core_e2e.py [--only <phần,...>]
bash scripts/linux/package.sh     # gói Linux (Core Go + .oxt; MCP nằm trong Core)

# MCP server (cả hai nền tảng; Host.exe mcp chỉ chuyển tiếp sang Core):
AXIOM_MCP_ARGS="mcp all" python tests/mcp-host/test_mcp_portable.py <binary>
python tests/mcp-host/oracle_diff.py    # đối chiếu Go ↔ C# trên cùng kịch bản
```

Các phần e2e chạy riêng được: `fake_bridge`, `guards`, `skills`, `memory`, `confirm`, `mcp`, `visual`,
`setup`, `shutdown`, `anthropic`, `embeddings`, `summarize`, `cancel`, `mcp_http`, `verify`.

## 6. Việc nên làm tiếp (đề xuất)

1. **Bạn bấm thử wizard Windows** trong Word/Excel/WPS (5 bước, nhất là **Thử ngay**) rồi báo lại.
2. Chạy `--real-llm` cho **memory** (trích xuất bằng model thật) khi muốn chắc chắn chất lượng fact.
3. Chạy `test_mcp_host.py` (parity sâu) **trên Linux** — đợt này mới chạy được trên Windows; máy Linux
   không kết nối được lúc kiểm. Trên Linux cần venv có `mcp`/`python-docx`/`openpyxl`/`python-pptx`
   (xem ghi chú máy kiểm chứng) và đặt `AXIOM_MCP_NO_CATALOG=1`.
4. Nhánh C# **không còn đóng băng**: từ 04/10 add-in Windows có thêm ribbon đủ nhóm (Trợ lý / Tìm web / Kiểm tra /
   Quản lý / Hệ thống), chọn ngôn ngữ hiển thị, khoá vùng chọn trong Word, dialog *Công cụ ngoài…*, màu Fluent. `Host.exe mcp` vẫn chỉ là
   shim chuyển tiếp byte sang Core Go (`src/AxiomOffice.Host/Mcp/McpHost.cs`).
5. Phát hành gói Linux mới để người dùng có `install.sh` đã sửa (§3).
6. Chạy đề ngân hàng **3 lần trên PowerPoint thật** để nghiệm thu 5.1 (tool tự đi lại dây — code + test đã xong, xem
   §2.12); sau đó làm 5.2–5.3 trong `docs/plans/20-c0-ngu-canh-va-slide.md` §5.

## 7. Đối chiếu benchmark & lộ trình hiệu năng (02–03/10/2026)

Bộ đề 4 bài LibreOffice Calc so với Claude Code nằm ở `tests/bench/` (README cách chạy, số liệu ở
`results/`). Lộ trình hiệu năng theo tư vấn ngoài: **Bước 0–3 ĐÃ LÀM** (Bước 3 chưa đo với LLM)
(`results/con-lai.md` mục 7):

| Bước | Nội dung | Trạng thái |
|---|---|---|
| 0 | Dòng `METRIC` trong `run_prompt.py` (billable, by_tool, dup_reads, wall…) | ✅ |
| 1 | `PlanRule` trong prompt.go + skill quy tắc 9–10 | ✅ **đã A/B, giữ PlanRule** |
| 2 | Cắt response `readRange`/`checkRange` ở wrapper Go (`truncate.go`) + tail + contract | ✅ unit + **live 10/10** trên LO (50k dòng: 1 MB → 2,8 KB) |
| 3 | Sinh dữ liệu bằng công thức tất định (skill quy tắc 11 + `references/du-lieu-gia-lap.md`) | ✅ đo trên LO (11 cột × 50k dòng 9,7 s); chưa A/B với LLM |

**Kết quả A/B Test 3 đợt mimo (n=3/cặp, model `oc/mimo-v2.6-flash-free`, 03/10/2026)**:
6/6 lượt `completed + verified=True`, 13/13 sheet bắt buộc — variant **rẻ hơn 3/3 (median
−11,4% billable, −24,2% tool call)**, wall median −9,9%, điểm yếu: chart (control 3/3 lượt vs
variant 1/3). Chi tiết + caveat: `results/con-lai.md` mục 7; hạ tầng A/B + driver + 9 bài học
vận hành ngoài repo: `C:\Users\ThuyetMT\test\bench\ab\README.md`.

Cảnh báo vận hành đã ghi nhận: model free có lượt `provider returned an empty reply` (tính là lỗi
hạ tầng, loại khỏi số liệu); **không sửa script khi nó đang chạy** (bash đọc lệch offset → mất log);
LO thật là process `soffice.bin`.
