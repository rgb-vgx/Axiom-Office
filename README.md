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

Port mặc định (đổi qua registry, xem [Cấu hình](#cấu-hình)):

| App | Port |
|---|---|
| Writer | 47821 |
| Spreadsheets | 47822 |
| Presentation | 47823 |

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
# Điều khiển Spreadsheets (instance riêng, ẩn)
& "src\WpsAiBridge\bin\Release\WpsAiBridge.Host.exe" et

# Hiện cửa sổ WPS (để người dùng quan sát)
& "...\WpsAiBridge.Host.exe" et --visible
```

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

## Microsoft Office

Add-in được thiết kế để chạy trên **cả Microsoft Office lẫn WPS Office** —
cùng một DLL, cùng một đăng ký (không cần cài thêm gì):

- MS Office đọc đúng vị trí `HKCU\Software\Microsoft\Office\{Word,Excel,PowerPoint}\Addins\<ProgID>`
  mà `install.ps1` đã ghi (`LoadBehavior=3`) — MS Office **không** cần whitelist
  `AddinsWL` (đó là cơ chế riêng của WPS, MS Office bỏ qua).
- COM class (`IDTExtensibility2` + `IRibbonExtensibility`) và Ribbon XML
  (schema 2006/01) là chuẩn Office — tab "WPS AI Bridge" xuất hiện tương tự.
- Bridge ports giữ nguyên: Word 47821 / Excel 47822 / PowerPoint 47823
  (nhận diện app qua COM probe `Documents` / `Workbooks` / `Presentations`).

Yêu cầu: Office **x64** (kiểm tra `Platform` tại
`HKLM\SOFTWARE\Microsoft\Office\ClickToRun\Configuration`).

Kiểm tra nhanh (đã kiểm chứng trên Office 2024 ProPlus x64 — Word/Excel/PowerPoint:
lifecycle + ribbon + E2E qua bridge):

1. Mở Word/Excel/PowerPoint
2. `%LOCALAPPDATA%\WpsAiBridge\bridge.log` phải có `OnConnection` + `GetCustomUI`
3. `http://127.0.0.1:47821/health` (Word) / `47822` (Excel) / `47823` (PowerPoint)

Lưu ý chạy song song WPS + Office: WPS ET và Microsoft Excel **cùng map vào port
47822** (port theo app kind) — nếu mở cả hai cùng lúc, chỉ một bên bind được.
Cách xử lý tạm: đổi `Port` base trong registry (ví dụ 47831) trước khi mở app
thứ hai, xong đổi lại.

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
| `et.newWorkbook` | — | Tạo workbook mới |
| `et.open` | `path` | Mở file .xlsx/.xls/.csv |
| `et.listSheets` | — | Liệt kê sheet + sheet đang active |
| `et.readRange` | `range`, `sheet?` | Đọc vùng, ví dụ `A1:C10` |
| `et.writeRange` | `range`, `values` (ma trận 2D), `sheet?` | Ghi vùng |
| `et.save` / `et.saveAs` | `path?` | Lưu / lưu thành file mới |
| `wpp.newPresentation` | — | Tạo presentation mới |
| `wpp.open` | `path` | Mở file .pptx |
| `wpp.listSlides` | — | Số slide + text trên từng slide |
| `wpp.addSlide` | — | Thêm slide trống cuối presentation |
| `wpp.addTextBox` | `slide?`, `text`, `left?`, `top?`, `width?`, `height?` | Thêm textbox |
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
| `Port` | DWORD | 47821 | Port base (ET +1, WPP +2) |
| `Enabled` | DWORD | 1 | 0 = tắt HTTP bridge |
| `Token` | String | (trống) | Nếu đặt, mọi request phải kèm header `X-Auth-Token` |

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
