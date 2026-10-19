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
- **`Application.CreateTaskPane(url)` chạy được** (đo 12/10/2026, ET, biến thể A): trả về object task pane
  (`Visible` = `false`), đặt `Visible = true` là xong (`ok`, đọc lại `true`). URL tuyệt đối **có query vẫn nạp**
  (`GET /addin/pane.html?app=et` → 200), URL tương đối, dạng hai tham số (`url, title`) và dạng object đều không lỗi.
  `CreateTaskPane.length` = 0 và `String(...)` = `function CreateTaskPane() { [native code] }`; gọi **0 tham số** ném
  `An signal occurred during the jsapi EXECUTE call.`; đặt `Visible` bằng **chuỗi** ném `参数类型错误(第0个).`
  ⇒ lỗi `Error: arguments error` khi bấm nút Ask AI **không** đến từ lời gọi này.
- Pane do `CreateTaskPane` tạo **có** nạp trang (CEF GET trang pane, 200) nhưng script trong trang pane **không báo
  về** server probe — chưa rõ nguyên nhân, cần đo tiếp (nghi: script trong pane không chạy, hoặc pane không được
  render dưới Xvfb).
- Danh sách key liên quan task pane trên `Application`: `CreateTaskPane`, `GetTaskPane`,
  `DisplayDocumentActionTaskPane`, `DisplayXMLSourcePane`, `DisplayDocumentInformationPanel`. `window.wps` **chính là**
  `window.Application` (`wpsIsApplication` = true), `IsSandboxed` = `false`.
- Đo được **song song với WPS đang mở của người dùng**: kênh JSAPI dùng pipe `/tmp/JSAPI_*{guid}` với GUID
  **sinh theo từng phiên**, nên một tiến trình WPS thứ hai (HOME + DISPLAY riêng) không đụng phiên đang mở.
  (Lệnh `probe_run.py --probe <tên>` từng dùng cho các phép đo dưới đây **không còn trong repo**; `probe.js` chỉ
  còn là bản báo cáo cố định, không phải công cụ chạy JS tùy ý — muốn đo lại thì phải tự dựng lại lối vào.)

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
| Đóng bài | **`Presentation.Close()` không tham số** | `Close(false)` bị từ chối: `参数太多.` — xem mục dưới |
| Đóng sổ (ET) | `Workbook.Close(false)` chạy được | ET **nhận** tham số, WPP **không** — hai app khác arity |
| Còn bẩn? | `Presentation.Saved` = **-1** khi đã lưu, `0` khi còn thay đổi | như VBA |

### `wpp.closeAll` từng đóng 0 bài vì WPP từ chối tham số của `Close` (đo 12/10/2026)

Triệu chứng: lượt live ba app để lại đúng một phép kiểm đỏ — `wpp.closeAll` trả `{closed: 0, rescued: []}`
trong khi đang có bài mở, và lệnh vẫn báo `ok: true`.

Cách khoanh vùng (theo thứ tự, mỗi bước loại một giả thuyết):

1. **"WPS từ chối đóng bài cuối cùng"** → mở **hai** bài rồi gọi lại: vẫn `closed: 0` ⇒ sai.
2. **"Vòng lặp không chạy (`Presentations.Count` = 0)"** → thêm trường chẩn đoán tạm vào handler (đã khôi phục
   nguyên trạng, không commit): `dbgLen = 2` ⇒ vòng lặp **có** chạy, hai bài thật. `Saved` = `-1` cả hai ⇒
   `rescued: []` là đúng, không phải nhánh cứu bài hỏng.
3. **Nguyên nhân thật**: `dbgErr = 参数太多.` ("quá nhiều tham số") — `Presentation.Close(false)` **bị WPP từ
   chối vì có tham số**. `closed` chỉ tăng khi `Close` không ném lỗi, mà lỗi thì bị `catch (e3) {}` nuốt, nên
   lệnh báo thành công trong khi chẳng đóng gì.
4. **Đường đi đúng**: `pres.Close()` **không tham số** — đo được `dbgNoArg = 2`, không lỗi, và
   `Presentations.Count` về **0** (đóng thật, không phải chỉ "không ném lỗi").
5. ET đối xứng mà **khác**: `Workbook.Close(false)` chạy được (`et.closeAll` xanh) ⇒ đừng suy từ app này sang
   app kia.

Bài học chung: **một `catch` rỗng biến lỗi arity thành "thành công"**. Với add-in này, "không ném lỗi" không
phải bằng chứng đã làm xong việc — phải kiểm trạng thái sau đó (`Presentations.Count`, file trên đĩa).

### `Application.FileSystem`: đọc/ghi file (đo 11/10/2026, byte đã biết)

Probe 2 (`/tmp/wps-probe-fs`): ghi file bằng byte biết trước rồi gọi từng API, in ra mã ký tự.
Bộ byte dùng để đo: `41 C3 AA 0A E2 82 AC 00 FF` (`'A'`, `ê` UTF-8, LF, `€` UTF-8, NUL, FF).

| API | Hành vi đo được |
|---|---|
| `ReadFile(path)` | **Đọc thật**, giải mã UTF-8. Thiếu file → **nem** `Error: "Wrong path!"`. Thừa tham số → `Error: 参数太多.` |
| `readFileString(path)` | **Sandbox**: đường dẫn có `/` hoặc `\` → nem `Error: path cannot contains "/" or "\"`; **tên file trần → `null`**; thừa tham số → `参数太多.` |
| `writeFileString(path, data)` | Cùng sandbox: đường dẫn đầy đủ → nem; **không ghi được file nào** |
| `readAsBinaryString(path)` | Mỗi byte `b >= 0x80` → mã ký tự `0xFF00 \| b` (hỏng dữ liệu); byte `< 0x80` giữ nguyên; thiếu file → `null` (không nem) |
| `writeAsBinaryString(path, data)` | **Ghi byte nguyên vẹn**: PDF 1811 byte ghi ra đọc lại đúng `%PDF-1.4` … `%%EOF` |
| `Exists(path)` / `existsSync(path)` | Trả `bool`, **cả hai đều nhận đường dẫn tuyệt đối** |
| `tmpdir()` | `"/tmp"` |

Hệ quả cho add-in: **`ReadFile` là API đọc duy nhất dùng được**; `readFileString`/`writeFileString` là bẫy
(chúng *trông* như API đọc file trong tài liệu WPS nhưng thực chất bị sandbox). Ghi file thì dùng
`writeAsBinaryString` (byte), đọc lại để kiểm bằng `ReadFile`/`Exists`.

`ReadFile` **cắt ở byte NUL đầu tiên**: bộ byte trên cho ra `"Aê\n€"` — mọi thứ từ NUL trở đi mất, kể cả `FF`.
Chưa đo được hành vi với BOM: fake (`tests/wps/addin_fake.mjs`) chọn hướng **giữ BOM** (đường khó hơn cho
add-in, tức nó phải tự cắt theo `utf-8-sig`).

### ET ép kiểu khi ghi vào `Value2` (đo 11/10/2026)

Gán chuỗi trông-như-số (`"42"`) hoặc trông-như-ngày (`"01/01"`) vào `Range.Value2` thì **WPS ET tự ép**:
ô thành số / thành ngày (`d-mmm`). Tức là **đường ghi không tạo được "số dạng text"**.

`numbers-as-text` vẫn có thật khi **file đã chứa sẵn** text trong ô trông-như-số (ô gieo từ trước) — đó là
đường đọc. Fake **không** mô phỏng việc ép kiểu này (xem ghi chú ở `setCellScalar`), nên spec chỉ được khẳng
định `numbers-as-text` trên ô gieo sẵn, không khẳng định qua đường ghi.

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

## Bridge: vòng poll, và vì sao một lệnh dài làm bridge báo "mất kết nối" oan (đo 12/10/2026)

Add-in **không listen được** (CEF Chrome 87), nên Core giữ bridge còn add-in **poll**: mỗi lời gọi
`/wps/poll` là một request dài mà Core giữ tới **20 giây** nếu chưa có lệnh (`DefaultPollSeconds`,
`wpsbridge.go:30`). "Còn sống" vì thế là một phép đo **thời gian**, không phải một kết nối:

```
Connected() = busyLocked() || (everPoll && now - lastPoll <= DisconnectSeconds)   // wpsbridge.go:312-322
```

- `DisconnectSeconds` mặc định của sản phẩm là **45** (`wpsbridge.go:32`); tầng e2e dùng 4 giây
  (`tests/wps/test_wps_bridge_e2e.py:92`) vì mọi lệnh ở đó đều nhanh.
- `busyLocked()` **chỉ che khi job còn nằm trong `waiting`**, mà job bị bỏ khỏi `waiting` **ngay lúc kết quả
  về** (`wpsbridge.go:423`). Nên `busy` thôi che đúng vào lúc lệnh vừa xong.

**Hệ quả đo được**: add-in **đơn luồng** — trong lúc nó đang chạy một lệnh thì nó **không poll được**. Với
`et.exportPdf` đo được **11 giây** (16:40:11 → 16:40:22), `lastPoll` đã cũ hơn ngưỡng 4 giây ngay khi lệnh
xong, nên lệnh kế bị từ chối **trước khi gửi**:
`WPS add-in is not connected: open a document in WPS to connect the add-in` (`wpsbridge.go:859-861`).
Lượt đo có probe theo dõi tiến trình cho thấy ET **ALIVE liên tục** trong suốt thời gian đó — app không hề chết.

Đây là **cấu hình quá chặt của tầng test**, không phải lỗi app: tầng live nay truyền ngưỡng mặc định của sản
phẩm (45 giây) qua `extra_env`. Nhưng phép đo **cũng phơi ra một lỗi thật**: với ngưỡng 45 giây, một lệnh chạy
quá 45 giây vẫn làm mọi người gọi khác thấy bridge mất kết nối, và vì `StartSessionLoop` **xoá file session**
khi `!Connected()` (`wpsbridge.go:984`), pane giữa chừng báo "chưa mở tài liệu" rồi lại thấy lại. Hướng sửa:
coi "add-in vừa trả kết quả" là bằng chứng còn sống (làm mới mốc hoạt động khi giao kết quả) — đã ghi thành
việc riêng (T8b trong hàng đợi tái cấu trúc).

Bài học cho người viết test live: một phép kiểm *mong đợi thất bại* sẽ xanh **kể cả khi** bridge đã mất kết
nối. Sau mỗi lệnh dài phải có một phép kiểm **khẳng định bridge còn sống**, và phép kiểm lệnh sai phải đòi
lỗi vì **tên lệnh sai**, không phải vì mất kết nối.

## Khóa vùng bôi đen trên WPS thật (đo 18/10/2026)

Bản WPS của `CommandDispatcher.WriterScope.cs` (add-in Windows). Vùng khóa **không** nhớ bằng một `Range`
(một lệnh AI viết lại là `Range` sập) mà bằng **phần chữ trước và sau** nó; trạng thái nằm trong biến module
`selectionLock` của `commands.js` — add-in là tiến trình duy nhất thấy mọi lệnh.

**Bốn API mà thiết kế dựa vào đều có trên WPS 11.1.0.11723** (đo bằng cách bôi đen thật rồi gọi
`writer.lockSelection`; handler **từ chối khóa** và trả `reason` nếu thiếu bất kỳ API nào, nên `chars > 0`
là bằng chứng cả bốn đều đọc được):

| Bôi đen | `writer.lockSelection` trả về |
|---|---|
| `Ctrl+A` (cả tài liệu, có 1 bảng đã chèn trước đó) | `{chars: 68, start: 0, end: 68, docEnd: 68, tables: 1, tablesKnown: true}` |
| `Ctrl+Home` rồi `Shift+Right` × 5 | `{chars: 5, start: 0, end: 5, docEnd: 68, tables: 0, tablesKnown: true}` |

- `Selection.Start/End`, `Content.End`, `Range(from, to).Text`, `Tables.Count` + `Tables.Item(i).Range.Start/End`
  đều đọc được. `tablesKnown: true` quan trọng: `false` nghĩa là `doc.Tables` không đọc được và **lưới an toàn
  mất phần bảng** trong im lặng.
- Chọn cả tài liệu thì `Selection.End == Content.End` (68 = 68) — không lệch 1 như Word; đây là số đo, đừng
  dựa vào nó như một quy luật.
- **`Selection.SetRange` kéo con trỏ được**: đặt con trỏ ra cuối tài liệu (`Ctrl+End`) rồi `writer.typeText`
  trong lúc khóa → chữ **rơi vào vùng khóa** (`Selection.SetRange(0,5)`), không phải chỗ con trỏ đang đứng.
  Nghĩa là trên bản WPS này lưới an toàn (hoàn tác + báo lỗi) là lưới **thứ hai**, không phải đường chính.
- Từ chối **trước** khi chạm tài liệu đúng như thiết kế: `writer.appendText` (ghi ra ngoài vùng) trả
  `ok:false` kèm `Refused: the user selected part...`, và tài liệu **không đổi một ký tự**.
- `writer.unlockSelection` mở khóa xong thì lệnh ghi cả tài liệu chạy lại bình thường.

**Bôi đen thật trong test live**: `xdotool` (XTEST) — `tests/wps/test_wps_bridge_live.py` đọc `DISPLAY`/
`XAUTHORITY` từ `/proc/<pid>/environ` của tiến trình WPS vì `xvfb-run -a` cấp số hiển thị động. Dưới Xvfb
**không có window manager**, nên `xdotool windowactivate` báo `_NET_ACTIVE_WINDOW` và hỏng; `windowfocus`
(XSetInputFocus) là đủ. Đã kiểm chứng đường này trước khi viết vào test: `Ctrl+A` rồi gõ `ZZZ` rồi `Ctrl+S`
làm `blank.docx` chỉ còn `ZZZ`.

Bẫy nhỏ khi viết test sau: **`xdotool key` thành công mà không in gì ra stdout** — phán "gửi được phím" bằng
`bool(stdout)` là luôn sai (đã mắc, 3 phép kiểm đỏ oan trong khi vùng chọn đã đổi thật); phải xét mã thoát.

**Một khóa cho mỗi bridge, không phải một khóa cho mỗi lượt chạy**: `selectionLock` là biến module của
add-in, giống `_selectionLock` bên C#. Hai lượt chạy cùng lúc trên một WPS sẽ giành khóa của nhau — nhưng
`POST /v1/runs` và `POST /wps/v1/runs` đều đi qua `Manager.TryStart(port)` (`api/runs.go:129`), mà port là
port của bridge WPS, nên mỗi bridge chỉ có **một** lượt chạy. Không cần khóa theo lượt.

## Ba đường mới của pane/ribbon trên Core thật (đo 19/10/2026, KHÔNG cần WPS)

`tests/wps/test_wps_pane_e2e.py` (chạy bằng `make test-wps-e2e`: Core thật + add-in giả poll qua HTTP) nay đo
luôn ba đường mà nút trên ribbon/pane gọi — trước đây chúng chỉ có unit test Go và test DOM giả, chưa lần nào
chạy trên Core thật. Kết quả lượt đo: **68 phép kiểm pass** (thêm 18 so với trước), bridge e2e 36 pass.

### `GET /wps/v1/status` — một lời gọi, hai tầng

- Trả `{addin: <Bridge.Status()>, core: <coreStatus>}`. Tầng add-in đo được: `connected:true`, `app:"wps"`,
  `document.name` = tên tài liệu của **lần poll gần nhất** (add-in giả khai `bao-cao-tuan.docx`),
  `addinVersion:"0.1.0"` (bản add-in khai trong poll, không phải bản Core), `everPoll:true`, `queued:0`,
  `waiting:0`. Tầng Core: `port` khớp cổng thật của tiến trình, `pid` thật, `uptimeSeconds ≥ 0`,
  `dataDir` = `AXIOM_CORE_DATA_DIR`, `logFile` = `<dataDir>/core.log`, `model:"fake-model"` và
  `configured:true` (env có endpoint + model) — pane dựa vào `configured` để nút "Kiểm tra mô hình" biết đã
  chọn AI chưa, thay vì đoán.
- `Origin` lạ → 403 (đo được, không chỉ suy từ `checkWPSAuth`).

### Địa chỉ nút "Sao chép API URL" — và cái bẫy hình dạng JSON

Pane sao chép `http://127.0.0.1:<bridgePort>/health` (lấy `bridgePort` từ chính `/wps/v1/status`). Đo trên
Core thật: dán địa chỉ đó vào trình duyệt (không Origin, không token) → **200**, `connected:true`,
`port` khớp. Đúng như `DecideBridgeAuth` hứa: chỉ `GET /health` của bridge được miễn token.

**Bẫy đã mắc khi viết phép đo**: `/health` của bridge trả **phong bì** `{"ok":true,"result":{...}}`, không
phải object trần — phép kiểm đầu tiên đọc thẳng `.connected` nên đỏ oan trong khi đường vẫn chạy đúng. Ai
viết tiếp cái gì đọc `/health` của bridge thì nhớ bóc `result`.

### `GET /wps/v1/log` — phần cuối nhật ký

- Đọc đúng tệp Core đang ghi (`<dataDir>/core.log`), `exists:true`, `count == len(lines) ≤ 200`, có dòng
  `Agent Core starting` thật của lượt khởi động.
- Ngữ nghĩa `lines` đo được: `?lines=2` trả **đúng hai dòng cuối** và `truncated:true` (còn dòng phía trên);
  `?lines=1` trả dòng cuối cùng. `truncated` = "có dòng bị bỏ", không phải "tệp hỏng" — pane dựa vào đó để
  hiện nút "Xem thêm".
- **Tệp chưa có → 200 `exists:false`, `lines:[]`**, không phải lỗi (đo bằng cách đổi tên tệp log sang
  `.bak` rồi trả lại ngay; `corelog` giữ tệp mở nên Linux ghi tiếp theo inode, không hỏng gì).
- `/v1/log` giữ guard cũ: không token → 401/403; có token → cùng tệp.
- Ghi chú về phép đo: để khẳng định "hai dòng cuối" một cách tất định, test **tự ghi thêm hai dòng mốc** vào
  chính tệp log đó rồi đọc lại (lúc ấy chưa có lượt chạy nào nên Core không ghi xen vào). Đó là phép đo
  `lines`/phần-cuối, không phải phép đo nội dung log.

### `POST /wps/v1/exec` — lệnh trực tiếp, và hai mã 409

- Kết quả add-in trả về đi qua **nguyên vẹn**: báo cáo `writer.checkTables` giữ đúng
  `tableCount/issueCount/issues[].type` (`empty-cells`, `long-header-cell`) và tiếng Việt có dấu trong
  `header` (không bị ép ASCII). Pane dịch `issues[].type` sang nhãn tiếng Việt bằng chính khoá đó
  (`ISSUE_LABELS`, khớp `checks.py`) — đổi tên khoá là pane hiện nhãn rỗng/sai.
- Lệnh add-in **không có** → HTTP **200** kèm `{ok:false, error:"unknown action: ..."}`, đúng như
  `commands.js execute()`. Pane phải đọc `ok` chứ không đọc mã HTTP, nếu không nút "Soát bảng" sẽ hiện một
  báo cáo rỗng như thể đã soát xong.
- Hai ca 409 mang **mã máy đọc được**, đo trên HTTP thật: `addin-disconnected` (chưa mở tài liệu) và
  `run-in-progress` (đang có lượt AI chạy — bắn trong lúc lượt 2 đang chạy). Pane dịch mã sang câu tiếng
  Việt riêng, nên phép đo phải bám `code`; bám chuỗi tiếng Anh của Core là mong manh.

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
