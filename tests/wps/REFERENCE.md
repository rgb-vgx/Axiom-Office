# WPS Office for Linux — JS add-in: những gì đã biết (tài liệu tham khảo cho spike)

Máy dev: WPS Office for Linux **11.1.0.11723**, cài ở `/opt/kingsoft/wps-office` (launcher `/usr/bin/wps`).
Không cần (và agent trong worktree không được phép) đọc `/opt` — mọi thứ cần biết đã chép ở đây.

## Đã xác minh trên WPS thật (08/10/2026, chạy add-in dưới Xvfb)

Add-in **nạp được và gửi được báo cáo** khi có đủ 4 điều kiện:

1. **`/opt/kingsoft/wps-office/office6/cfgs/oem.ini` (file của root) có `JsApiPlugin=true` trong `[Support]`.**
   Thiếu dòng này WPS bỏ qua mọi JS add-in, không báo lỗi. Công cụ chính thức `wpsjs` cũng sửa file này (bằng
   sudo trên Linux), kèm `JsApiShowWebDebugger=true`. Trên máy dev đã bật (bản sao lưu `oem.ini.axiom-bak`).
   Trình cài add-in **không** được sửa file này — đó là bước admin riêng.
2. **Mở một tài liệu.** `wps` không tham số dừng ở trang chủ: không có ribbon nên add-in không bao giờ nạp.
   test chạy thật (`tests/wps/test_wps_bridge_live.py`) mở `blank.docx` (chép từ `core-go/internal/templates/default.docx`).
3. **Có `index.html` trong thư mục add-in** nạp các script (`<script src="main.js">`). WPS mở `index.html` này
   (`location.href = file:///<HOME>/.local/share/Kingsoft/wps/jsaddons/NAME_VERSION/index.html`); không có nó
   thì `ribbon.xml` vẫn được đọc nhưng `onLoad="ribbon.OnAddinLoad"` báo `ReferenceError: ribbon is not defined`
   (hộp thoại modal, chặn luôn script khác). WPS **không** tự sinh `index.html`.
4. **Đăng ký offline** trong `<HOME>/.local/share/Kingsoft/wps/jsaddons/publish.xml`:
   `<jsplugins><jsplugin name="NAME" type="wps" url="NAME_VERSION" version="VERSION" enable="enable_dev" install="null" customDomain=""/></jsplugins>`
   với thư mục `jsaddons/NAME_VERSION/` chứa `ribbon.xml`, `index.html`, các file JS.

`Office.conf` cho HOME mới (trong `<HOME>/.config/Kingsoft/Office.conf`, mục `[6.0]`): `common\AcceptedEULA=true`
(bỏ hộp thoại license) và `wpsoffice\Application%20Settings\AppComponentMode=prome_independ` +
`...AppComponentModeInstall=prome_independ` (launcher mở thẳng Writer). Hộp thoại "thiếu font Symbol" có hiện
nhưng không chặn add-in.

Môi trường JS đo được (`src/AxiomOffice.WPS/probe.js`, nhánh `wps-probe`):

| Trường | Giá trị |
|---|---|
| `Origin` của request từ add-in | `file://` (POST `application/json` có preflight `OPTIONS`; `text/plain` thì không) |
| Trình duyệt | CEF Chrome 87 (`Mozilla/5.0 (X11; Linux x86_64) ... Chrome/87.0.4280.20`) |
| Global | `Application`, `wps`, `external` có; `WpsInvoke`, `cef` không |
| Transport | `fetch`, `XMLHttpRequest`, `WebSocket` đều có |
| `Application.Version` / `Name` | `12.0` / `WPS Writer` |
| `Application.UndoRecord` | **dùng được**: `StartCustomRecord`/`EndCustomRecord` là `function`, gọi trên tài liệu mở không lỗi |

## Đã xác minh trên WPS thật (11/10/2026: đăng ký cả ba app, object model ET/WPP)

Probe: `/tmp/wps-probe` (Xvfb + HOME tạm + `publish.xml` sinh theo biến thể, add-in probe POST báo cáo JSON).
Máy dev: cùng bản WPS 11.1.0.11723.

### `publish.xml`: một entry, `type` là danh sách

Ba biến thể đã thử, **chỉ biến thể A nạp được add-in trong cả ba app**:

| Biến thể | `publish.xml` | Kết quả |
|---|---|---|
| **A (chọn)** | **một** `<jspluginonline name="axiomoffice" type="wps,et,wpp" url="..."/>` | Writer, Spreadsheets, Presentation đều nạp |
| B | ba entry, ba `name` khác nhau (`axiomoffice`, `-et`, `-wpp`) | chỉ app khớp `name` đầu |
| C | ba entry, cùng `name`, ba `type` | chỉ entry đầu có hiệu lực |

Vậy `type` nhận **danh sách ngăn bằng dấu phẩy** và một add-in phục vụ cả ba app. Hệ quả: add-in phải tự biết
app nào đang chạy (`Application.Name`) và chỉ khai báo lệnh của app đó — xem `docs/wps-bridge.md` mục 5.

- `GET` mà WPS gọi khi nạp add-in: `ribbon.xml`, `index.html`, rồi script trong `index.html` — y hệt bản một app.
- Tên app đo được (`Application.Name`): `WPS Writer`, `WPS Spreadsheets` (ET), `WPS Presentation` (WPP).
  `Application.Version` = `12.0` cả ba; `Application.Build` = `11723` (WPP trả `11.1.0.11723`).
- `Application` trong cả ba app đều có `CommandBars.ExecuteMso`, `CreateTaskPane`, `FileSystem`
  (`existsSync`/`ReadFile`/`WriteFile`/... — dùng được để tự kiểm file xuất ra).

### ET (Spreadsheets) — khác Writer ở đâu

| Việc | Cách đúng | Ghi chú |
|---|---|---|
| Vào ô | `ActiveSheet.Range("A1")`, `ActiveSheet.Cells.Item(r, c)` | `Cells(r, c)` **không** gọi được: `Cells` là object, không phải function |
| Vùng dùng thật | `ActiveSheet.UsedRange.Address()` | `.Address` là **hàm**, không phải thuộc tính |
| Ghi cả khối | `Range("M1:N2").Value2 = [[1,2],[3,4]]` | mảng 2 chiều; **scalar điền kín cả vùng** (`Range("O1:O3").Value2 = "x"` → o1=o3="x") |
| Điền công thức | `Range("B1:B2").Formula = "=A1*2"` | WPS **tự dịch tham chiếu tương đối**: B1 `=A1*2`, B2 `=A2*2` |
| Đọc ô lỗi | `.Text` = `#DIV/0!`, `.Value2` = `null`, `.Formula` = `=1/0` | ô lỗi vẫn giữ công thức |
| Màu | số nguyên **BGR** (`Interior.Color`, `Font.Color`) | `16777215` = trắng |
| Chỉ số | sheet/ô **1-based** (`Worksheets.Item(1)`); `Sheets.Item(0)` → `null` | |
| Đơn vị | `Range.Left`/`Top` tính bằng **point** (1 ô = 63,35 × 14,4 pt; A1 ≈ left 316,75 / top 57,6) | đủ để neo biểu đồ |
| Sheet mới | `Worksheets.Add()` → `Sheets.Count` 1→2 | |
| Sổ mới/mở | `Workbooks.Add()`, `Workbooks.Open(path)` là function; **`Application.Documents` không có** | |
| Lưu | `Workbook.Save()`, `Workbook.SaveAs(path)` **ghi file thật** (`.xlsx` 12 KB) | |
| Đóng | `Workbook.Close(false)` chạy được, `Workbooks.Count` 1→0 | |

### ET/WPP: hai thứ **không** có (khác LibreOffice)

- **`UndoRecord` không tồn tại**: `Application.UndoRecord`, `ActiveWorkbook.UndoRecord`, `ActiveSheet.UndoRecord`
  đều `undefined` (WPP cũng vậy). `Application.Undo()` và `CommandBars.ExecuteMso("Undo")` gọi được nhưng
  **không đổi gì** trên trang probe. ⇒ `et.undo` (lệnh duy nhất cần undo; registry không có `wpp.undo`) không thể
  dựa vào undo của WPS.
- **Xuất PDF không ghi file**: `Workbook.ExportAsFixedFormat(0|1, path)`, `Workbook.SaveAs(path.pdf, 57)`,
  `Presentation.ExportAsFixedFormat(path, 2)` đều trả về bình thường nhưng `exists=false` và không có file nào.
  ⇒ phải làm như `writer.exportPdf` của LibreOffice: thử API gốc, **kiểm bằng `FileSystem.existsSync`**, không có
  thì tự dựng PDF.

### WPP (Presentation) — object model

| Việc | Cách đúng | Ghi chú |
|---|---|---|
| Trang | `ActivePresentation.Slides.Add(index, layout)`, `.Slides.Count`, `.Slides.Item(i)` | `Add` trả về slide mới; `Slide.SlideIndex`, `Slide.Name` = `Slide 1` |
| Layout | `CustomLayouts.Count` = **12** | 1 = tiêu đề, 2 = tiêu đề + nội dung, 11 = chỉ tiêu đề, 12 = trống |
| Khổ slide | `PageSetup.SlideWidth` / `SlideHeight` = **720 × 540** (point, 4:3) | |
| Textbox | `Slide.Shapes.AddTextbox(orientation, left, top, w, h)` → shape | `Slide.Shapes.Count`, `Shapes.Item(i).Name` (`Title 1`, `Text Box 3`) |
| Chữ | `shape.TextFrame.TextRange.Text`; `.Font.Size`, `.Font.Bold` (**-1** = true), `.Font.Color.RGB` (BGR), `.ParagraphFormat.Alignment` (2 = giữa) | tên shape do WPS đặt |
| Bảng | `Slide.Shapes.AddTable(rows, cols, left, top, w, h)`; `shape.Table.Cell(r, c).Shape.TextFrame.TextRange.Text` | |
| Ảnh / hình | `Shapes.AddPicture(path, ...)`, `Shapes.AddShape(type, ...)` là function | |
| Ghi chú | `Slide.NotesPage.Placeholders` (2 placeholder), đặt được text | |
| Xoá slide | `Slide.Delete()` chạy được | |
| Lưu | `Presentation.Save()`, `Presentation.SaveAs(path)` **ghi file thật** (`.pptx` 48 KB) | |
| Sổ mới/mở | `Presentations.Add()`, `Presentations.Open(path)`; `Application.ActivePresentation` | `Application.Documents` không có |

## Từ `strings` trên thư viện của WPS

- `office6/addons/jsapi/libjsapisubserver.so` chứa:
  `/wps/jsaddons/publish.xml`, `jsplugins`, `jsplugin`, `jspluginonline`, `online`, `name`, `version`,
  `enable_dev`, `authwebsite.xml`, `JsApiAuthWebsite`, `/wps/jsaddons/`, `/addons/jsapi/`, `/wpsjsclient`,
  `/jsapisubserver`, `Forbidden`.
  → đường dẫn đầy đủ là `<HOME>/.local/share/Kingsoft/wps/jsaddons/publish.xml` (thư mục `jsaddons` có sẵn
  trong HOME của người dùng thật trên máy này).
- `office6/libjsapiservice.so`: `JsApiShowWebDebugger`, `/wps/jsaddons/binary/WPSNativeX.conf`,
  `getJsApiRemoteDebuggingPort`, `getJsApiUserDataDir`, `IJSPluginStorage`, `onLoadFinish(int)`,
  `onLoadError(int, qint64, int, const QString&, const QString&)`.
- `office6/` có `libjswpsapi.so`, `libjsetapi.so`, `libjswppapi.so` (object model JS cho Writer/ET/WPP) và
  `addons/cef/` (trình duyệt nhúng chạy add-in).
- HOME mới tinh: lần chạy `wps` đầu tiên tạo `~/.local/share/Kingsoft/office6/...`, `~/.local/share/Kingsoft/WPS Cloud
  Files/` và `~/.config/Kingsoft/Office.conf`.

## Theo công cụ `wpsjs` của WPS (bản online chưa thử)

`publish.xml`:

```xml
<jsplugins>
  <!-- Bản "online": WPS nạp add-in từ URL http (wpsjs debug dùng cách này, kèm enable="enable_dev") -->
  <jspluginonline name="axiomoffice" type="wps" url="http://127.0.0.1:PORT/" enable="enable_dev" debug="" install="null"/>
  <!-- Bản "offline": url trỏ tới gói NAME_VERSION.7z; giải nén ở jsaddons/NAME_VERSION/ -->
  <jsplugin name="axiomoffice" type="wps" url="http://.../axiomoffice_1.0.0.7z" version="1.0.0"/>
</jsplugins>
```

- `type`: `wps` (Writer), `et` (Spreadsheets), `wpp` (Presentation).
- Thư mục add-in (online: gốc của URL; offline: `jsaddons/NAME_VERSION/`) có `ribbon.xml` (customUI, thuộc tính
  `onLoad="OnAddinLoad"` gọi hàm JS khi nạp) và `index.html` nạp `main.js` (các hàm callback của ribbon).
- Trong JS: `window.Application` (hoặc `wps.WpsApplication()` / `wps.EtApplication()`), API giống VBA/COM:
  `Application.ActiveDocument`, `Application.Version`, `Application.UndoRecord`...
- Bản online cần một HTTP server phục vụ thư mục add-in suốt lúc WPS chạy — trong spike có thể là một server
  nhỏ do trình cài khởi động KHÔNG được (trình cài phải thoát); cân nhắc bản offline, hoặc ghi rõ nếu online là
  cách duy nhất chạy được.
- `authwebsite.xml` (cùng thư mục `jsaddons`) có thể dùng để cấp quyền cho origin gọi API.
