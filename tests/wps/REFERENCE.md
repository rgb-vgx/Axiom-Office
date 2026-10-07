# WPS Office for Linux — JS add-in: những gì đã biết (tài liệu tham khảo cho spike)

Máy dev: WPS Office for Linux **11.1.0.11723**, cài ở `/opt/kingsoft/wps-office` (launcher `/usr/bin/wps`).
Không cần (và agent trong worktree không được phép) đọc `/opt` — mọi thứ cần biết đã chép ở đây.

## Đã xác minh (từ `strings` trên thư viện của WPS)

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

## Chưa xác minh (theo công cụ `wpsjs` của WPS — dùng làm điểm xuất phát)

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
