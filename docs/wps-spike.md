# Spike: JS add-in WPS Office for Linux

Add-in JS tu nap khi WPS Writer mo tai lieu va gui mot bao cao JSON ve Agent
(bao gom `wpsVersion` va `undoRecord`). Chi tiet nen (oem.ini, `strings`,
`wpsjs`): `tests/wps/REFERENCE.md`.

## Dieu kien de add-in nap (da xac minh tren WPS that, 08/10/2026)

1. `JsApiPlugin=true` trong `[Support]` cua
   `/opt/kingsoft/wps-office/office6/cfgs/oem.ini` (file cua root). Thieu dong
   nay WPS bo qua moi JS add-in, khong bao loi. Buoc admin mot lan moi may;
   trinh cai add-in khong duoc sua file nay.
2. Mo mot tai lieu (`wps blank.docx`). `wps` khong tham so dung o trang chu:
   khong co ribbon nen add-in khong bao gio nap.
3. Co `index.html` trong thu muc add-in de nap cac script. WPS khong tu sinh
   file nay; thieu no thi `onLoad="ribbon.OnAddinLoad"` bao
   `ReferenceError: ribbon is not defined`.
4. Dang ky offline trong `<HOME>/.local/share/Kingsoft/wps/jsaddons/publish.xml`:
   `<jsplugin name="axiomoffice" type="wps" url="axiomoffice_0.1.0"
   version="0.1.0" enable="enable_dev" install="null" customDomain=""/>`,
   voi thu muc `jsaddons/axiomoffice_0.1.0/` chua `ribbon.xml`, `index.html`,
   cac file JS.

`Office.conf` cho HOME moi (`<HOME>/.config/Kingsoft/Office.conf`, muc `[6.0]`):
`common\AcceptedEULA=true` (bo hop thoai license),
`wpsoffice\Application%20Settings\AppComponentMode=prome_independ` va
`...AppComponentModeInstall=prome_independ` (launcher mo thang Writer).

## Moi truong do duoc

| Truong | Gia tri |
|---|---|
| `Origin` cua request tu add-in | `file://` (POST `application/json` co preflight `OPTIONS`; `text/plain` thi khong) |
| Trinh duyet | CEF Chrome 87 (`Mozilla/5.0 (X11; Linux x86_64) ... Chrome/87.0.4280.20`) |
| Global | `Application`, `wps`, `external` co; `WpsInvoke`, `cef` khong |
| Transport | `fetch`, `XMLHttpRequest`, `WebSocket` deu co |
| `Application.Version` / `Name` | `12.0` / `WPS Writer` |
| `Application.UndoRecord` | **dung duoc**: `StartCustomRecord`/`EndCustomRecord` la `function`, goi tren tai lieu mo khong loi |

Do bang probe `src/AxiomOffice.WPS/probe.js`
(`AxiomProbe.collectReport(window.Application, window)`).

## Bo cuc cai dat

Nguon: `src/AxiomOffice.WPS/` (`ribbon.xml`, `index.html`, `main.js`,
`probe.js`; `config.js` do trinh cai sinh).

- `ribbon.xml`: `customUI` voi `onLoad="ribbon.OnAddinLoad"`, mot tab
  "Axiom Office" voi mot nut "Axiom" (`onAction="ribbon.OnAction"`) gui lai
  bao cao.
- `index.html`: WPS bat buoc (khong tu sinh); nap theo thu tu `config.js`,
  `probe.js`, `main.js` bang `<script src>` thuong, khong module, khong build.
- `main.js` (ES5): dinh nghia `window.ribbon = { OnAddinLoad, OnAction }`
  (`OnAddinLoad` tra ve `true`). Khi nap: cho den khi co
  `window.Application && Application.ActiveDocument` (poll 500 ms, bo cuoc sau
  30 s van gui) roi gui dung mot lan voi `trigger: "load"`; nut gui lai voi
  `trigger: "button"`. Bao cao = `AxiomProbe.collectReport(...)` + `trigger`,
  POST duoi dang text JSON voi `Content-Type: text/plain` (tranh preflight vi
  Origin la `file://`) bang `XMLHttpRequest` toi `window.AXIOM_REPORT_URL`.
  Moi thu boc `try/catch`, khong bao gio throw, khong `alert()`.

Cai dat:

    python3 scripts/wps/install_jsaddon.py --home <HOME> --report-url <URL>

- Chep toan bo file nguon vao
  `<HOME>/.local/share/Kingsoft/wps/jsaddons/axiomoffice_0.1.0/`
  (cai lai xoa sach thu muc nay roi chep lai), sinh `config.js`:
  `window.AXIOM_REPORT_URL = "<URL>";`.
- Ghi/cap nhat `publish.xml` voi dung mot entry cua add-in (giu entry cua
  add-in khac, cai lai khong trung).
- Ghi 3 khoa `Office.conf` tren vao muc `[6.0]` (giu cac dong khac).

Kiem tra: `python3 tests/wps/test_install_jsaddon.py` (offline),
`python3 tests/wps/spike_check.py --stage full` (mo `blank.docx` trong WPS duoi
Xvfb, doi bao cao co `loaded=true`, `wpsVersion` khong rong, `undoRecord`
boolean).

## He qua cho kien truc Axiom Office

- Origin `file://`: guard cua Agent Core (chan moi Origin) phai co cach cho
  phep add-in nay (danh sach allow hoac co che tin cay rieng), dong thoi giu
  `Content-Type: text/plain` de khong dinh preflight.
  Luu y: guard hien tai con bat buoc `Content-Type: application/json` cho
  `/cmd` (sai thi 415) - mau thuan voi `text/plain`; can quyet dinh cach mo
  cho add-in (chap nhan preflight voi CORS cho `file://`, hoac noi long
  Content-Type rieng cho kenh add-in).
- `JsApiPlugin` can admin mot lan moi may (sua `oem.ini` cua he thong) — buoc
  cai dat rieng, ngoai trinh cai user-level.
- `UndoRecord` dung duoc: mot tac vu AI co the gom thanh mot buoc Undo nhu
  Word (`StartCustomRecord`/`EndCustomRecord`).
