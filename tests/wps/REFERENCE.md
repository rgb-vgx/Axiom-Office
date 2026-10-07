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
   `spike_check.py` mở `blank.docx` (chép từ `core-go/internal/templates/default.docx`).
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
