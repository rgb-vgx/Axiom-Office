# WPS AI Bridge

HTTP bridge cho phép AI agent (hoặc bất kỳ process nào) điều khiển WPS Office —
Writer, Spreadsheets, Presentation — qua JSON API trên localhost. Không cần
admin, không cần chỉnh sửa hay bật thủ công gì trong WPS.

```
AI agent ──HTTP/JSON──► WPS AI Bridge ──COM──► WPS Office
                          (in-proc hoặc companion)
```

## Kiến trúc

Hai chế độ chạy cùng một protocol, cùng command set:

| Chế độ | Cơ chế | Dùng khi |
|---|---|---|
| **In-proc add-in** (`WpsAiBridge.dll`) | COM add-in `IDTExtensibility2` nạp thẳng vào WPS, mở HTTP server trong process WPS + thêm tab **"WPS AI Bridge"** trên ribbon | WPS đang mở (điều khiển document đang mở của người dùng) |
| **Companion** (`WpsAiBridge.Host.exe`) | Process riêng dùng COM automation (`KWPS/KET/KWPP.Application`) | App chưa mở, hoặc add-in không nạp được |

Port mặc định (đổi qua registry, xem [Cấu hình](#cấu-hình)) — WPS và Microsoft
Office dùng 2 dải port riêng nên chạy song song không đụng nhau:

| App | WPS (`Port`, 47821) | Microsoft Office (`PortOffice`, 47831) |
|---|---|---|
| Writer / Word | 47821 | 47831 |
| Spreadsheets / Excel | 47822 | 47832 |
| Presentation / PowerPoint | 47823 | 47833 |

## Yêu cầu

- WPS Office 2019+ x64 (dev/test trên WPS 12.1.0.28485, bản quốc tế)
- (Tuỳ chọn) Microsoft Office x64 — cùng một đăng ký phục vụ cả hai bộ app,
  xem mục [Microsoft Office](#microsoft-office)
- .NET Framework 4.8 (cho add-in C# + companion)
- Build: Visual Studio 2022 Build Tools (không cần full VS)

## Build

```powershell
# Build add-in DLL + companion EXE (C#)
scripts\build.ps1

# Hoặc build bản native C++ (branch cpp-native-addin, xem mục Branches)
scripts\build-native.ps1
```

> `build.ps1` hỏi Windows Restart Manager xem tiến trình nào đang giữ
> `WpsAiBridge.dll` / `WpsAiBridge.Host.exe` (Word/Excel/PowerPoint/WPS/companion).
> Có thì dừng và in tên + pid; `scripts\build.ps1 -Kill` để tự tắt đúng các tiến
> trình đó (lưu tài liệu trước). Biên dịch ra `bin\Release\.stage` rồi mới chép
> đè, nên lỗi biên dịch hay file bị lock đều giữ nguyên DLL cũ.

## Cài đặt / Gỡ

```powershell
scripts\install.ps1            # đăng ký COM (HKCU, không cần admin) + whitelist WPS, in sẵn cấu hình MCP
scripts\uninstall.ps1          # gỡ đăng ký (add-in + Ask AI pane), giữ cấu hình AI/token
scripts\uninstall.ps1 -Purge   # gỡ và xoá luôn HKCU\Software\WpsAiBridge + %LOCALAPPDATA%\WpsAiBridge
```

`install.ps1` tự gỡ nhãn "tải từ Internet" (Zone.Identifier) của DLL/EXE: file giải nén từ zip
tải về mang nhãn này và .NET sẽ từ chối nạp add-in.

### Đóng gói cho người khác

```powershell
scripts\package.ps1            # build + tạo dist\WpsAiBridge-<version>-<ngày>-<commit>.zip
scripts\package.ps1 -NoBuild   # dùng bản đã build trong bin\Release
scripts\package.ps1 -Kill      # build.ps1 -Kill (tắt app đang giữ DLL)
```

Zip (~245 KB) chỉ gồm DLL add-in, `WpsAiBridge.Host.exe` (companion + MCP server), script cài/gỡ,
`install.cmd` / `uninstall.cmd` (nhấp đúp; tự Unblock rồi chạy PowerShell với `-ExecutionPolicy
Bypass`), `HUONG-DAN-CAI-DAT.txt` và `THIRD-PARTY-NOTICES.md`. Người nhận **không cần Python,
Visual Studio hay quyền admin**: giải nén vào chỗ cố định → đóng Office/WPS → nhấp đúp
`install.cmd`. Tên zip có `-dirty` khi đóng gói từ code chưa commit.

Sau khi cài, mở WPS lên là add-in tự nạp (kiểm tra `http://127.0.0.1:47821/health`).

Nếu add-in không nạp: trong WPS vào **Công cụ → COM加载项 / COM Add-ins** để kiểm
tra danh sách, và xem log tại `%LOCALAPPDATA%\WpsAiBridge\bridge.log`.

### Companion

```powershell
# WPS Office
& "src\WpsAiBridge\bin\Release\WpsAiBridge.Host.exe" wps   # hoặc: et, wpp
# Microsoft Office
& "src\WpsAiBridge\bin\Release\WpsAiBridge.Host.exe" word  # hoặc: excel, ppt

# Hiện cửa sổ app (để quan sát)
& "...\WpsAiBridge.Host.exe" excel --visible
```

Companion log rõ app nó tạo được (`Companion resolved application: Microsoft Excel 16.0`).
Lưu ý: một số máy (như máy dev này) WPS đăng ký đè các ProgID
`Word/Excel/PowerPoint.Application` ở HKCU → companion `word|excel|ppt` sẽ tạo
WPS compat component (version 12.0) thay vì Office thật; lệnh vẫn chạy đúng
(object model tương thích). Muốn dùng **Microsoft Office thật**: mở app trực tiếp
— add-in tự nạp và serve ở dải port Office (47831-47833).

Nếu port đã được add-in in-proc phục vụ, companion tự chuyển sang chế độ idle
(model bền process) và log lại — cả hai đường đều trả cùng kết quả.

## Ribbon UI

Add-in in-proc thêm tab **"WPS AI Bridge"** trên ribbon (WPS gọi
`IRibbonExtensibility.GetCustomUI` khi load — xem log) với 2 nhóm:

| Nút | Chức năng |
|---|---|
| **Status** | Hộp thoại hiển thị app, port, API base, health URL, đường dẫn log |
| **Copy API URL** | Copy `http://127.0.0.1:<port>/` vào clipboard |
| **Open Log** | Mở `%LOCALAPPDATA%\WpsAiBridge\bridge.log` bằng ứng dụng mặc định |
| **Ask AI...** | Mở **task pane dock trong app** (bên phải, cạnh thanh scroll) — xem mục Ask AI |
| **Settings** | Cấu hình LLM (provider/endpoint/key/model) |

Callback của nút đi qua `IDispatch` (class dùng `ClassInterfaceType.AutoDispatch`),
tag từng nút được log tại `OnButtonAction` trong bridge.log.

## Ask AI (agent mode)

Ask AI mở dạng **task pane gắn trong ứng dụng** (`ICustomTaskPaneConsumer` —
cùng API cho cả Microsoft Office và WPS; nếu host không hỗ trợ sẽ fallback
sang cửa sổ nổi). Trong pane:

1. Gõ yêu cầu (ví dụ *"Soạn cho tôi một mẫu đơn xin việc"*, *"Tạo 5 slide giới
   thiệu công ty"*, *"Bảng điểm cho 5 học sinh"*) hoặc bấm một gợi ý → **Ask**
   (Enter gửi, Shift+Enter xuống dòng; Ask chỉ bật khi ô nhập có chữ)
2. AI chạy **agent mode**: gọi LLM với **tool-calling**, tự thực thi các hành
   động đọc/ghi (`writer.*` / `et.*` / `wpp.*`) **trực tiếp lên tài liệu đang
   mở** — user thấy nội dung xuất hiện real-time; mỗi thao tác hiện thành một
   dòng (dấu tick xanh / dấu x đỏ + nhãn tiếng Việt + mã action)
3. Với Word, mỗi action của AI là **1 bước Ctrl+Z** (UndoRecord); link
   **Chèn trả lời** chèn câu trả lời cuối, **Cài đặt** mở cấu hình LLM
4. Link **Dừng** ở footer hủy lượt đang chạy ngay (abort request LLM đang chờ).
   **Không giới hạn số vòng** gọi model/tool: lượt chạy kết thúc khi AI trả lời xong, khi
   bấm Dừng hoặc khi chạm trần **5 phút** (`LlmClient.AgentTimeoutMs`); mỗi request LLM 60s.
   Lỗi hiện thành thẻ có **Thử lại** / **Mở Cài đặt**; chưa cấu hình endpoint/model
   thì ô nhập bị khoá kèm hướng dẫn

UI vẽ bằng GDI+ theo design tokens trong `PaneTheme` (`Ai/PaneControls.cs`;
chữ/nền đạt tương phản ≥ 4.5:1, focus ring cho bàn phím, scale theo DPI). Bubble
nhận Tab/Ctrl+C và có menu chuột phải **Sao chép**. Transcript của mỗi lượt
luôn có trong `bridge.log` (`AskAiPane: prompt=` / `AskAiPane progress:` /
`AskAiPane: ok|cancelled` / `AskAiPane failed`) — ghi từ worker nên vẫn còn dấu
vết kể cả khi pane đã đóng.

Hoạt động với provider OpenAI-compatible và Anthropic (tools); nếu provider
không hỗ trợ tools, tự fallback về chat thường. Từ bên ngoài, agent có thể gọi
cùng logic qua bridge command **`ai.ask {prompt}`**.

## MCP server (C#) — `WpsAiBridge.Host.exe mcp`

MCP server chạy ngay trong companion EXE: **không cần Python, venv hay pip**. Người dùng chỉ
cần `WpsAiBridge.Host.exe` (build cùng add-in, template docx/pptx nhúng sẵn trong exe).

```json
{
  "mcpServers": {
    "office": {
      "command": "C:\\Tools\\WpsAiBridge\\src\\WpsAiBridge\\bin\\Release\\WpsAiBridge.Host.exe",
      "args": ["mcp"]
    }
  }
}
```

- `mcp` = cả 50 tool (Word + Excel + PowerPoint + `office_sessions`); muốn ít tool hơn thì
  `mcp word` (20), `mcp excel` (16), `mcp ppt` (16) — cùng tên/tham số với 3 server Python.
- `WpsAiBridge.Host.exe mcp --list` in danh sách tool. Log ở `bridge.log` (`MCP tool ... ok in Nms`).
- Giao thức: MCP stdio (JSON-RPC 2.0, mỗi message một dòng), `initialize` / `tools/list` /
  `tools/call` / `ping`; protocol 2024-11-05 → 2025-11-25.

Làn file đọc/ghi thẳng OOXML (ZipArchive + XML, không thư viện ngoài). Khi sửa file chỉ các
phần XML liên quan được ghi lại, nên **chart, ảnh, pivot, macro của file gốc được giữ nguyên**
(openpyxl của bản Python làm mất). Khác bản Python:

| | Bản C# |
|---|---|
| `excel_query` (DuckDB SQL) | **bỏ** — đọc bằng `excel_read` rồi để agent tự tổng hợp |
| `excel_convert` | chỉ `to="csv"` (không còn parquet) |
| đọc `.xls` (BIFF) | qua Excel hoặc WPS Spreadsheets cài trên máy (COM), không cần xlrd; ô lỗi trả `#DIV/0!`... thay vì mã số |
| `excel_copy_sheet` | chép cả định dạng có điều kiện, data validation, ô gộp; bỏ chart/ảnh/bảng như openpyxl |
| `excel_rename_sheet` | cập nhật cả defined names trỏ tới sheet |

Kiểm tra: `tests/mcp-host/test_mcp_host.py` dùng MCP client Python chính thức, so kết quả
từng tool file với `tools/*-mcp/*/file_tools.py` trên cùng file (file do C# tạo, do
python-docx/openpyxl/python-pptx tạo, file Word/Excel/PowerPoint thật) và đọc lại file C# ghi
ra bằng python-docx/openpyxl/python-pptx (152/152). `tests/mcp-host/office_roundtrip.ps1
-Verify` mở các file đó bằng Microsoft Office thật.

```powershell
tools\excel-mcp\.venv\Scripts\python.exe tests\mcp-host\test_mcp_host.py <thư_mục_output>
powershell -ExecutionPolicy Bypass -File tests\mcp-host\office_roundtrip.ps1 -Dir <thư_mục_output> -Make   # tạo mẫu từ Office thật
powershell -ExecutionPolicy Bypass -File tests\mcp-host\office_roundtrip.ps1 -Dir <thư_mục_output> -Verify
```

## MCP servers Python (`tools/`, legacy)

Ba MCP server Python cùng pattern (làn file + làn live qua bridge). Vẫn giữ để đối chiếu và
cho `excel_query` (DuckDB); cài đặt mới nên dùng bản C# ở trên.

| Server | Thư mục | Tools | Làn file | Làn live |
|---|---|---|---|---|
| Excel | `tools/excel-mcp` | 17 | openpyxl/DuckDB (xlsx/xlsm/xls/csv/tsv) | `wps_live_*` → Excel/WPS ET |
| Word | `tools/word-mcp` | 20 | python-docx | `word_*` → Word/WPS Writer |
| PowerPoint | `tools/ppt-mcp` | 16 | python-pptx | `ppt_*` → PowerPoint/WPS WPP |

Cả 3 server có tool **`office_sessions()`** — liệt kê mọi bridge đang sống (Office +
WPS, nhiều app cùng lúc) từ session registry: `app`, `family`, `port`, `host`,
`document`, `healthy`. Truyền `port` lấy từ đây vào `wps_live_command` /
`word_command` / `ppt_command` để nhắm đúng instance thay vì đoán port. Trong
code Python: `bridge.sessions()` và `bridge.es_hint()` (URL + ví dụ curl cho `/events`).

### excel-mcp

MCP server Python cho AI agent thao tác Excel qua 2 làn (17 tools):

- **Làn file** (không cần app mở): `excel_profile`, `excel_read` (paging,
  `show_formula`), `excel_query` (DuckDB SQL, bảng `data`), `excel_write`,
  `excel_create`, `excel_convert` (parquet/csv), `excel_create_sheet`,
  `excel_copy_sheet`, `excel_rename_sheet`, `excel_delete_sheet`,
  `excel_format_range` (font/fill/border/alignment/numFmt/decimalPlaces —
  schema tham khảo [negokaz/excel-mcp-server](https://github.com/negokaz/excel-mcp-server)),
  `excel_create_table`
- **Làn live** (file đang mở trong WPS/Office): `wps_live_command` (passthrough
  mọi command bridge), `wps_live_read_range`, `wps_live_write_range`, `wps_health`

Hỗ trợ `.xlsx/.xlsm/.xls/.csv/.tsv`. File lớn: `excel_convert` sang parquet rồi
query — nhanh hơn ~250x (đo trên file 41k dòng).

An toàn dữ liệu: mọi thao tác ghi dùng file tạm cùng thư mục + `os.replace`
(atomic — lỗi giữa chừng không làm hỏng file gốc); openpyxl có thể mất
chart/image/pivot khi resave → file đang mở nên ghi qua làn live. `excel_query`
bị khóa `enable_external_access=false` — SQL không đọc/ghi được file ngoài
(chống prompt injection).

```powershell
cd tools\excel-mcp
python -m venv .venv
.venv\Scripts\python.exe -m pip install -r requirements.txt
.venv\Scripts\python.exe -m unittest discover -s tests   # chạy test suite
.venv\Scripts\python.exe -m excel_mcp.server   # hoặc run.cmd
```

Đăng ký với MCP client (Claude Code / Claude Desktop / ...):

```json
{
  "mcpServers": {
    "excel-tools": {
      "command": "<repo>\\tools\\excel-mcp\\.venv\\Scripts\\python.exe",
      "args": ["-m", "excel_mcp.server"],
      "env": { "PYTHONPATH": "<repo>\\tools\\excel-mcp" }
    }
  }
}
```

### word-mcp

```powershell
cd tools\word-mcp
python -m venv .venv
.venv\Scripts\python.exe -m pip install -r requirements.txt
.venv\Scripts\python.exe -m unittest discover -s tests
.venv\Scripts\python.exe -m word_mcp.server   # hoặc run.cmd
```

Live tools mặc định gọi Microsoft Word (47831); truyền `app="wps"` để dùng WPS
Writer (47821). Tools: `word_read_text`, `word_type_text`, `word_insert_styled_text`,
`word_format_selection`, `word_heading`, `word_insert_table`, `word_insert_image`,
`word_insert_hyperlink`, `word_replace_all`, `word_export_pdf`, `word_undo`,
`word_save`, `word_health`, `word_command` (passthrough). Mọi thao tác ghi của AI
= **1 bước Ctrl+Z** (Word UndoRecord).

```json
{
  "mcpServers": {
    "word-tools": {
      "command": "<repo>\\tools\\word-mcp\\.venv\\Scripts\\python.exe",
      "args": ["-m", "word_mcp.server"],
      "env": { "PYTHONPATH": "<repo>\\tools\\word-mcp" }
    }
  }
}
```

### ppt-mcp

```powershell
cd tools\ppt-mcp
python -m venv .venv
.venv\Scripts\python.exe -m pip install -r requirements.txt
.venv\Scripts\python.exe -m unittest discover -s tests
.venv\Scripts\python.exe -m ppt_mcp.server   # hoặc run.cmd
```

Live mặc định Microsoft PowerPoint (47833); `app="wpp"` cho WPS Presentation
(47823). Tools: `ppt_list_slides`, `ppt_add_slide`, `ppt_add_text`, `ppt_add_image`,
`ppt_add_table`, `ppt_set_notes`, `ppt_delete_slide`, `ppt_export_pdf`, `ppt_save`,
`ppt_health`, `ppt_command` (passthrough).

```json
{
  "mcpServers": {
    "ppt-tools": {
      "command": "<repo>\\tools\\ppt-mcp\\.venv\\Scripts\\python.exe",
      "args": ["-m", "ppt_mcp.server"],
      "env": { "PYTHONPATH": "<repo>\\tools\\ppt-mcp" }
    }
  }
}
```

## Microsoft Office

Add-in được thiết kế để chạy trên **cả Microsoft Office lẫn WPS Office** —
cùng một DLL, cùng một đăng ký (không cần cài thêm gì):

- MS Office đọc đúng vị trí `HKCU\Software\Microsoft\Office\{Word,Excel,PowerPoint}\Addins\<ProgID>`
  mà `install.ps1` đã ghi (`LoadBehavior=3`) — MS Office **không** cần whitelist
  `AddinsWL` (đó là cơ chế riêng của WPS, MS Office bỏ qua).
- COM class (`IDTExtensibility2` + `IRibbonExtensibility`) và Ribbon XML
  (schema 2006/01) là chuẩn Office — tab "WPS AI Bridge" xuất hiện tương tự.
- App kind nhận diện qua COM probe (`Documents` / `Workbooks` / `Presentations`);
  host là Microsoft Office sẽ dùng dải port **47831-47833** (registry `PortOffice`).

Yêu cầu: Office **x64** (kiểm tra `Platform` tại
`HKLM\SOFTWARE\Microsoft\Office\ClickToRun\Configuration`).

Kiểm tra nhanh (đã kiểm chứng trên Office 2024 ProPlus x64 — Word/Excel/PowerPoint:
lifecycle + ribbon + E2E qua bridge):

1. Mở Word/Excel/PowerPoint
2. `%LOCALAPPDATA%\WpsAiBridge\bridge.log` phải có `OnConnection` + `GetCustomUI`
3. `http://127.0.0.1:47831/health` (Word) / `47832` (Excel) / `47833` (PowerPoint)

Port tách biệt theo host: WPS dùng `Port` (47821-47823), Microsoft Office dùng
`PortOffice` (47831-47833) — mở song song cả hai hệ không đụng nhau.

Troubleshooting riêng cho Office:

- Add-in bị disable sau crash (cơ chế Resiliency của Office): xoá entry trong
  `HKCU\Software\Microsoft\Office\16.0\{Word,Excel,PowerPoint}\Resiliency\DisabledItems`
  rồi chạy lại `install.ps1` để khôi phục `LoadBehavior=3`.
- Sau khi Word crash, lần mở kế tiếp có thể dừng ở hộp thoại *"start in safe
  mode?"* — add-in chưa nạp nên `/health` không trả lời cho tới khi đóng hộp thoại.
- Word luôn có một cửa sổ `OpusApp` **ẩn, không title** ngay từ lúc khởi động
  (có trước cả khi tạo task pane) — đó là cửa sổ của Word, không phải của add-in.
  `app.info` trả `state` (số tài liệu, tài liệu active, các cửa sổ + visible); log
  ghi trạng thái này ở `OnConnection`, `OnStartupComplete` và lệnh bridge đầu tiên.
  Lệnh `writer.*` tự kích hoạt tài liệu có cửa sổ hiển thị nếu `ActiveDocument`
  là tài liệu ẩn.
- Companion hiện nhận `KWPS/KET/KWPP.Application` (WPS); hỗ trợ
  `Word/Excel/PowerPoint.Application` cho MS Office đang phát triển —
  xem `docs/office-integration.md` khi có.

## API

### `GET /health`

```json
{"ok":true,"result":{"app":"wps","pid":1234,"port":47821,"version":"0.1.0","log":"..."}}
```

### `POST /cmd`

Body: `{"action": "<tên>", "params": {...}}`
Response: `{"ok": true, "result": {...}}` hoặc `{"ok": false, "error": "..."}`

> **Bảo mật**: bridge từ chối mọi request có header `Origin` (chặn CSRF từ trình
> duyệt); `POST /cmd` bắt buộc `Content-Type: application/json` (chặn "simple
> request" của browser); token ngẫu nhiên tự sinh khi chạy `install.ps1` và bắt
> buộc cho `/cmd`, `/config`, `/session`, `/events`. `GET /health` không cần token (chỉ đọc).

### `GET /session` (cần token)

Thông tin của chính bridge này, đọc tài liệu đang mở **ngay lúc gọi**:

```json
{"ok":true,"result":{"pid":5128,"app":"wps","family":"office","port":47831,"host":"WINWORD.EXE",
 "version":"1.0.0","started":"2026-09-28T21:25:42.965Z","lastSeen":"...","lastSeenEpoch":1790630759.1,
 "document":"phase1_doc.docx","documentPath":"C:\\...\\phase1_doc.docx","sessionFile":"..."}}
```

`app` là loại logic (`wps` = Writer/Word, `et` = Spreadsheets/Excel, `wpp` =
Presentation/PowerPoint); `family` = `office` | `wps`. `document` = `null` khi
host không có tài liệu nào mở (host bận quá 2s thì trả giá trị đã cache).

### Session registry

Mỗi bridge (add-in in-proc lẫn companion) khi start ghi
`%LOCALAPPDATA%\WpsAiBridge\sessions\{pid}.json` (cùng nội dung `/session`),
heartbeat mỗi **25s** (cập nhật `lastSeen` + tên tài liệu) và xoá file khi
`OnDisconnection` / `Stop()`. Việc đọc tài liệu cho heartbeat chạy nền, chờ tối
đa 2s và bỏ qua lượt khi bridge đang chạy lệnh trong host — Word bận không làm
trễ heartbeat. `bridge.sessions()` (Python) prune file khi process đã chết, hoặc
heartbeat quá 90s mà `/health` cũng không trả lời; bridge đang bận một lệnh dài
(heartbeat còn mới) vẫn được giữ với `healthy: false`.

### `GET /events` (cần token) — Server-Sent Events

```powershell
curl.exe -N -H "X-Auth-Token: <token>" http://127.0.0.1:47831/events
```

```
event: hello
data: {"app":"wps","family":"office","port":47831,"pid":5128,"subscribers":1}

event: document
data: {"app":"wps","name":"phase2_other.docx","fullName":"C:\\...\\phase2_other.docx"}

event: selection
data: {"app":"wps","text":"m tra hoi q","start":4,"end":15}

event: ping
data: {"time":"2026-09-28T21:26:27.055Z","subscribers":1}
```

| Event | Khi nào | Data |
|---|---|---|
| `hello` | ngay khi kết nối | `app`, `family`, `port`, `pid`, `subscribers` |
| `document` | tài liệu active đổi (gửi lại cho subscriber mới) | `app`, `name`, `fullName` (`null` khi không có tài liệu) |
| `selection` | vùng chọn đổi | Writer/Word: `text` (≤ 200 ký tự), `start`, `end`; ET/Excel: `sheet`, `address`, `text` = giá trị ô trên-trái; WPP/PowerPoint: `slide`, `type` (text/shapes/slides/none), `shape`, `text`, `start`, `end` |
| `ping` | mỗi 15s | `time`, `subscribers` |

V1 dùng **poll-diff 500ms** (không COM event sink — chạy giống nhau trên Office
và WPS); poller chỉ chạy khi có subscriber, tối đa **5 subscriber** mỗi bridge
(cái thứ 6 nhận `503`). Mỗi kết nối có thread ghi riêng nên `/cmd` và các endpoint
khác không bị chặn. Host bận (hoặc bridge đang chạy lệnh) thì bỏ qua vòng poll
đó; lỗi COM được log tối đa 1 lần/phút mỗi loại.

### Truy cập COM tuần tự (`ComGate`)

Pump HTTP, agent của pane, heartbeat session và poller SSE đều gọi object model
của host qua `Bridge/ComGate.cs`: tại một thời điểm chỉ một thread của bridge ở
trong host. Lệnh `/cmd` và tool của agent chờ tới lượt; heartbeat/poller chỉ chạy
khi cổng rảnh. Object model Office không an toàn đa luồng — thiếu cổng này Word
đã crash (AV trong `wwlib.dll`) khi heartbeat đọc `ActiveDocument` đúng lúc
`ai.ask` đang ghi.

### Danh sách command

| Action | Params | Mô tả |
|---|---|---|
| `app.info` | — | Tên/version app, thông tin document đang mở |
| `ai.ask` | `prompt` | Chạy AI agent (tool-calling → thao tác live document), trả `reply` + `transcript` |
| `writer.newDocument` | — | Tạo document mới |
| `writer.open` | `path` | Mở file .docx/.doc |
| `writer.getText` | `maxChars?` | Đọc toàn bộ text |
| `writer.typeText` | `text` | Gõ text tại vị trí con trỏ |
| `writer.appendText` | `text` | Nối text vào cuối document |
| `writer.replaceAll` | `find`, `replace` | Tìm & thay thế toàn bộ |
| `writer.selection` | — | Text + vị trí đang chọn |
| `writer.save` / `writer.saveAs` | `path?` | Lưu / lưu thành file mới |
| `writer.insertStyledText` | `text`, `bold?`, `italic?`, `underline?`, `size?`, `color?`, `font?` | Chèn text kèm định dạng |
| `writer.formatSelection` | font (`bold?`... `color?`), `alignment?` | Định dạng vùng đang chọn |
| `writer.setParagraphAlignment` | `alignment` (left/center/right/justify) | Căn đoạn |
| `writer.insertTable` | `rows`, `cols`, `values?`, `style?` | Chèn bảng kèm dữ liệu (`rows`/`cols` tự suy ra/nới theo `values`) |
| `writer.insertPageBreak` | — | Ngắt trang |
| `writer.insertImage` | `path`, `width?`, `height?` | Chèn ảnh tại con trỏ |
| `writer.insertHyperlink` | `url`, `text?` | Chèn hyperlink |
| `writer.heading` | `level` (1-9), `text?`, `break?` | Heading style + tự xuống dòng |
| `writer.undo` | `count?` | Undo (mỗi action AI = 1 bước Ctrl+Z) |
| `writer.exportPdf` | `path` | Xuất PDF |
| `et.newWorkbook` | — | Tạo workbook mới |
| `et.open` | `path` | Mở file .xlsx/.xls/.csv |
| `et.listSheets` | — | Liệt kê sheet + sheet đang active |
| `et.readRange` | `range`, `sheet?` | Đọc vùng, ví dụ `A1:C10` |
| `et.writeRange` | `range`, `values` (ma trận 2D), `sheet?` | Ghi vùng (thiếu/sai `values` → lỗi kèm ví dụ) |
| `et.save` / `et.saveAs` | `path?` | Lưu / lưu thành file mới |
| `et.formatRange` | `range`, `bold?`, `italic?`, `fontSize?`, `fontColor?`, `fillColor?`, `numFmt?`, `horizontal?`, `wrap?`, `sheet?` | Định dạng vùng |
| `et.activateSheet` | `sheet` | Chuyển sang sheet khác |
| `et.exportPdf` | `path` | Xuất PDF |
| `et.undo` | `count?` | Undo |
| `wpp.newPresentation` | — | Tạo presentation mới |
| `wpp.open` | `path` | Mở file .pptx |
| `wpp.listSlides` | — | Số slide + text trên từng slide |
| `wpp.addSlide` | `layout?` (mặc định 12 = blank) | Thêm slide với layout |
| `wpp.addTextBox` | `slide?`, `text`, `left?`, `top?`, `width?`, `height?` | Thêm textbox |
| `wpp.addText` | `text`, `slide?`, vị trí/kích thước?, `fontSize?`, `bold?`, `color?`, `align?` | Textbox có định dạng |
| `wpp.addImage` | `path`, `slide?`, `left?`, `top?`, `width?`, `height?` | Chèn ảnh |
| `wpp.addTable` | `rows`, `cols`, `values?`, `slide?`, vị trí/kích thước? | Bảng kèm dữ liệu |
| `wpp.setNotes` | `text`, `slide?` | Speaker notes |
| `wpp.deleteSlide` | `slide` | Xóa slide |
| `wpp.exportPdf` | `path` | Xuất PDF |
| `wpp.save` / `wpp.saveAs` | `path?` | Lưu / lưu thành file mới |

### Ví dụ

```powershell
# Tạo document, gõ text, lưu file
$base = "http://127.0.0.1:47821"
Invoke-RestMethod -Method Post -Uri "$base/cmd" -ContentType "application/json" `
  -Body '{"action":"writer.newDocument"}'
Invoke-RestMethod -Method Post -Uri "$base/cmd" -ContentType "application/json" `
  -Body '{"action":"writer.typeText","params":{"text":"Xin chao!"}}'
Invoke-RestMethod -Method Post -Uri "$base/cmd" -ContentType "application/json" `
  -Body '{"action":"writer.saveAs","params":{"path":"C:\\temp\\hello.docx"}}'

# Ghi bảng tính
Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:47822/cmd" -ContentType "application/json" `
  -Body '{"action":"et.writeRange","params":{"range":"A1:B2","values":[["Ten","Diem"],["AI",9.5]]}}'
```

```python
import requests

r = requests.post("http://127.0.0.1:47822/cmd",
                  json={"action": "et.readRange", "params": {"range": "A1:B2"}})
print(r.json())
```

## Cấu hình

`HKCU\Software\WpsAiBridge`:

| Value | Kiểu | Mặc định | Ý nghĩa |
|---|---|---|---|
| `Port` | DWORD | 47821 | Port base cho WPS (Spreadsheets +1, Presentation +2) |
| `PortOffice` | DWORD | 47831 | Port base cho Microsoft Office (Excel +1, PowerPoint +2) |
| `Enabled` | DWORD | 1 | 0 = tắt HTTP bridge |
| `Token` | String | tự sinh khi install (32 hex) | Bắt buộc cho `/cmd` và `/config` (header `X-Auth-Token`); `tools/excel-mcp` tự đọc từ registry |
| `LlmProvider` / `LlmEndpoint` / `LlmApiKey` / `LlmModel` | String | — | Cấu hình AI cho Ask AI (đặt qua dialog Settings trên ribbon; API key được mã hóa DPAPI — key plaintext cũ vẫn đọc được) |

## Cấu trúc project

```
src/WpsAiBridge/            add-in C# + code dùng chung (HttpBridge, Dispatcher, Connect)
src/WpsAiBridge.Host/       entry point companion EXE
src/WpsAiBridge.Host/Mcp/   MCP server C# (stdio) + template docx/pptx nhúng
tests/mcp-host/             test parity MCP C# vs Python + round-trip Office thật
scripts/build.ps1           build C# (add-in DLL + companion EXE)
scripts/package.ps1         đóng gói zip cài đặt cho người khác (dùng scripts/dist/*)
scripts/install.ps1         đăng ký HKCU (branch cpp-native-addin: ưu tiên native DLL)
scripts/uninstall.ps1       gỡ đăng ký
```

Đăng ký được ghi toàn bộ vào HKCU — không cần quyền admin:

- `Software\Classes\CLSID\{F4524DFD-...}` — COM class (mscoree + CodeBase,
  hoặc trỏ thẳng DLL native)
- `Software\Microsoft\Office\{Word,Excel,PowerPoint}\Addins` — metadata
  (`LoadBehavior=3`)
- `Software\Kingsoft\Office\{WPS,ET,WPP}\AddinsWL` — whitelist add-in của WPS

## Troubleshooting

- **Add-in không load**: đóng WPS hoàn toàn, chạy lại `scripts\install.ps1`,
  mở WPS. Xem `%LOCALAPPDATA%\WpsAiBridge\bridge.log`.
- **WPS tự tắt add-in sau crash**: WPS (giống MS Office) tự hạ `LoadBehavior`
  3→2 và ghi `AddinsCL\WpsAiBridge.Connect` khi add-in lỗi. Chạy lại
  `install.ps1` để khôi phục `LoadBehavior=3` và xoá AddinsCL.
- **Health OK nhưng command lỗi `no active document`**: chưa có document mở —
  gọi `*.newDocument` / `*.newWorkbook` / `*.newPresentation` trước.
- **App đang bận** (đang gõ trong ô Excel, đang mở dialog...): bridge tự retry
  các lỗi COM busy (`RPC_E_CALL_REJECTED` / `SERVERCALL_RETRYLATER` /
  `VBA_E_IGNORE`) tối đa 10 lần (~5s) trước khi trả lỗi — xem log nếu cần.
- **Ô lỗi trong Excel**: `#DIV/0!` `#VALUE!` `#NAME?` `#REF!` `#NUM!` `#NULL!`
  trả về đúng tên chuỗi; riêng `#N/A` trùng mã với ô trống qua `Value2` nên
  vẫn về `null` (giới hạn đã biết của COM).
- **Port bận**: một app khác đang giữ port — kiểm tra `netstat -ano | findstr 4782`.
- **Tìm bridge đang sống**: xem `%LOCALAPPDATA%\WpsAiBridge\sessions\*.json` hoặc
  gọi `office_sessions()` từ MCP server.
- **Task pane hẹp trên WPS 12**: CTP của WPS mở ra ~250px và áp `Width` trễ; pane
  tự đo lại sau khi host layout (tối đa 3 lần, mỗi 500ms) và nới về 360px — log
  `Task pane width: ctp=... control=...px`. Đã kiểm chứng trên WPS 12.1.0.28485
  (250 → 360px).
- Kiến trúc WPS 12: mọi component (Writer/ET/WPP) chạy chung binary `wps.exe`
  với flag `/wps`, `/et`, `/wpp` — đừng tin tưởng tên process để phân biệt app,
  bridge dùng COM probe.

## Branches

| Branch | Nội dung |
|---|---|
| `main` | C# add-in + companion — đường chạy chính thức |
| `cpp-native-addin` | WIP port add-in sang C++ native (raw COM, không CLR). Hiện WPS tạo được instance + QI `IDTExtensibility2` nhưng chưa invoke `OnConnection` — xem commit `f2f0f00` để biết chi tiết trạng thái |

Lịch sử thay đổi và root-cause của các bug đã fix: xem [CHANGELOG.md](CHANGELOG.md).
