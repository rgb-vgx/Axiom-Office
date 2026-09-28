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

> Đóng WPS trước khi build (DLL đang được WPS giữ sẽ gây lỗi ghi file).

## Cài đặt / Gỡ

```powershell
scripts\install.ps1     # đăng ký COM (HKCU, không cần admin) + whitelist WPS
scripts\uninstall.ps1   # gỡ đăng ký
```

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
`IRibbonExtensibility.GetCustomUI` khi load — xem log) với 3 nút:

| Nút | Chức năng |
|---|---|
| **Status** | Hộp thoại hiển thị app, port, API base, health URL, đường dẫn log |
| **Copy API URL** | Copy `http://127.0.0.1:<port>/` vào clipboard |
| **Open Log** | Mở `%LOCALAPPDATA%\WpsAiBridge\bridge.log` bằng ứng dụng mặc định |

Callback của nút đi qua `IDispatch` (class dùng `ClassInterfaceType.AutoDispatch`),
tag từng nút được log tại `OnButtonAction` trong bridge.log.

## MCP servers (`tools/`)

Ba MCP server Python cùng pattern (làn file + làn live qua bridge):

| Server | Thư mục | Tools | Làn file | Làn live |
|---|---|---|---|---|
| Excel | `tools/excel-mcp` | 16 | openpyxl/DuckDB (xlsx/xlsm/xls/csv/tsv) | `wps_live_*` → Excel/WPS ET |
| Word | `tools/word-mcp` | 19 | python-docx | `word_*` → Word/WPS Writer |
| PowerPoint | `tools/ppt-mcp` | 15 | python-pptx | `ppt_*` → PowerPoint/WPS WPP |

### excel-mcp

MCP server Python cho AI agent thao tác Excel qua 2 làn (16 tools):

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
> buộc cho `/cmd` + `/config`. `GET /health` không cần token (chỉ đọc).

### Danh sách command

| Action | Params | Mô tả |
|---|---|---|
| `app.info` | — | Tên/version app, thông tin document đang mở |
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
| `writer.insertTable` | `rows`, `cols`, `values?`, `style?` | Chèn bảng kèm dữ liệu |
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
| `et.writeRange` | `range`, `values` (ma trận 2D), `sheet?` | Ghi vùng |
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
scripts/build.ps1           build C# (add-in DLL + companion EXE)
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
- Kiến trúc WPS 12: mọi component (Writer/ET/WPP) chạy chung binary `wps.exe`
  với flag `/wps`, `/et`, `/wpp` — đừng tin tưởng tên process để phân biệt app,
  bridge dùng COM probe.

## Branches

| Branch | Nội dung |
|---|---|
| `main` | C# add-in + companion — đường chạy chính thức |
| `cpp-native-addin` | WIP port add-in sang C++ native (raw COM, không CLR). Hiện WPS tạo được instance + QI `IDTExtensibility2` nhưng chưa invoke `OnConnection` — xem commit `f2f0f00` để biết chi tiết trạng thái |

Lịch sử thay đổi và root-cause của các bug đã fix: xem [CHANGELOG.md](CHANGELOG.md).
