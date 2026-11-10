# Kiểm thử thực tế WPS 12.1.2 trên Linux — lượt 2 (28/10/2026)

Tài liệu này là báo cáo của **lượt đo thứ hai trong cùng ngày**; báo cáo lượt đầu là
`wps-linux-live-test-2026-10-28.md`. Lượt này chạy trên nhánh `wps-2` từ `origin/main` (`ccb8820`), có sửa mã
nguồn và push.

## 1. Môi trường

- Kubuntu 22.04, KDE/X11, màn hình thật `:0`, `xdotool`, `ffmpeg` (chụp màn hình).
- WPS Office **12.1.2.29182.AK.preread.sw** tại `/opt/kingsoft/wps-office`
  (`Application.Name` = `WPS Sheets` / `WPS Docs`, `Application.Version` = 12.0).
- Core: `make core-build` → `dist/core/axiom-core`, chạy tay với
  `AXIOM_WPS_ADDIN_DIR=/tmp/opencode/wps2/addin` (thư mục chép từ `src/AxiomOffice.WPS`).
- Add-in đăng ký bằng `scripts/wps/install_jsaddon.py --home /tmp/opencode/wps2/home
  --core-url http://127.0.0.1:47840 --test-profile` — **HOME tạm**, không dùng/không sửa hồ sơ WPS của người dùng.
  WPS hỏi "Addin … is loaded for the first time, trust or not?" một lần cho mỗi app; đã bấm OK.
- Model cho các lượt đo: `ocg/deepseek-v4.1-flash` qua router nội bộ `http://localhost:20128/v1`;
  `DebugTrace=1` để có `~/.local/share/axiom-office/traces/<runId>.jsonl`.
- Cấu hình người dùng đã khôi phục sau khi đo: `LlmModel=oc/beam-free`, `DebugTrace=0`
  (bản sao lưu: `/tmp/opencode/wps2/config.json.bak`).

## 2. Kết quả các lệnh kiểm thử

| Lệnh | Kết quả | Ghi chú |
|---|---|---|
| `make check-gen` | **PASS** | catalog, VERSION, i18n (418 khoá, thiếu 0, thừa 0) khớp |
| `make test-go` (`go test -race ./...`) | **PASS** | toàn bộ package, gồm `internal/agent` sau khi sửa SA4006 |
| `make test-core` | **179 passed, 0 failed** | có phần `cross_app` mới; đã xoá `dist/` sau khi chạy xong |
| `make test-wps` | **215 passed, 0 failed** | `test_install_jsaddon.py` + `test_addin_files.py` (mới) + 11 tệp `node --test` |
| `make test-wps-e2e` | **36 passed** (bridge) + **94 passed** (pane) | `test_wps_bridge_e2e.py`, `test_wps_pane_e2e.py` |
| `make lint-js` | **PASS** | eslint sạch |
| `make lint-go` | **FAIL — chỉ 2 lỗi U1000 cũ** | `estimateLines` (`template_compose.go:491`), `cardMaxFont` (`:501`); lỗi SA4006 mới của main đã sửa |

## 3. Lỗi tìm được và cách sửa

### 3.1 `ribbon.xml` không hợp lệ XML — WPS bỏ qua toàn bộ add-in (commit `cacf651`)

**Triệu chứng.** Sau khi Core phục vụ add-in, WPS **vẫn tải** `manifest.xml`, `ribbon.xml`, `index.html`,
`probe.js`, `commands.js`, `main.js` (thấy trong `strace`) nhưng:

- không có tab **Axiom Office** trên ribbon;
- `ribbon.OnAddinLoad` không được gọi → `main.js` không `start()` vòng poll;
- `GET /health` của bridge trả `connected=false`, không có job nào được giao;
- không có hộp thoại lỗi, không có dòng log nào trong Core.

**Nguyên nhân.** `src/AxiomOffice.WPS/ribbon.xml` có label `Bộ nhớ & kỹ năng` — dấu `&` trần không hợp lệ
trong XML (phải là `&amp;`). Parser XML của WPS 12.1.2 thất bại im lặng và bỏ toàn bộ ribbon, kéo theo
`onLoad` không chạy. Đây là lý do đúng của "add-in không nạp" trong lượt đo này; khác với nguyên nhân 404
`AXIOM_WPS_ADDIN_DIR` đã ghi ở báo cáo lượt đầu.

**Sửa.** `&` → `&amp;`. Thêm `tests/wps/test_addin_files.py`: parse `ribbon.xml` bằng `xml.dom.minidom`,
kiểm `customUI/@onLoad="ribbon.OnAddinLoad"` và tab `Axiom Office`; chạy trong `make test-wps`.

**Kiểm chứng.** Trước khi sửa: 0 tab, `connected=false`. Sau khi sửa + mở lại WPS: tab **Axiom Office** xuất
hiện, bridge `connected=true`, agent chạy được các lượt dưới đây.

### 3.2 WPS ET đòi giá trị `values` của AutoFilter là **chuỗi** (trong commit `344f285`)

**Triệu chứng.** Trong lúc làm `et.filter`, gọi thật `AutoFilter(2, [7], 7)` trả `visibleRows: 0` dù có đúng
một hàng mang giá trị 7; `AutoFilter(2, ["7"], 7)` trả `visibleRows: 1`.

**Sửa.** Handler `et.filter` ép mọi phần tử `values` về chuỗi trước khi gọi `Range.AutoFilter`
(`etFilterValues`), kèm ghi chú đo được trên WPS 12.1.2. Không đổi contract C# (vẫn nhận `values` kiểu gì cũng được).

### 3.3 `make lint-go` báo SA4006 ở `tracking.go` (commit `8cbe414`)

**Triệu chứng.** `staticcheck` báo `this value of params is never used (SA4006)` tại
`core-go/internal/agent/tracking.go:89`.

**Nguyên nhân.** `params` được khởi tạo `map[string]any{}` rồi mọi nhánh `switch` đều gán lại hoặc `return`.

**Sửa.** `var params map[string]any` (không khởi tạo); hành vi không đổi. `go test ./internal/agent` xanh.

## 4. Kết quả live test

### 4.1 Excel/ET — `trace_formula` chỉ đúng ô gốc lỗi: **PASS**

Sheet `Trống`: `A1=10`, `B1=0`, `C1==A1/B1`, `D1==C1*2`, chọn `D1`; hỏi nguyên văn **"vì sao ô này lỗi?"**
qua pane. Run `r_8e42f90197642d76b46c39c2b123eebb` (5 vòng, 5 tool call):

`et.sheetInfo` → `et.readRange(A1:D2, formulas)` → **`trace_formula {cell:"Trống!C1", direction:"precedents"}`**
→ `et.readRange(A1:D1)` → `et.checkRange(C1:D1)`. Trả lời chỉ ra **C1** (`=A1/B1`, B1=0 → `#DIV/0!`), D1 lỗi theo.

### 4.2 Excel/ET — bấm trích dẫn `[Trống!C1]` trong pane: **PASS**

Handler `et.goto` **đã có** ở add-in WPS (ghi chú cũ trong yêu cầu đã lỗi thời). Bấm link trong câu trả lời
(pane → `POST /wps/v1/exec` → add-in): Name Box và thanh công thức chuyển từ `A5` sang **`C1`, `=A1/B1`**;
ô C1 được chọn. Kiểm chứng chéo: gọi `et.goto` qua bridge trả `{"range":"C1","sheet":"Trống"}` và chọn đúng ô.

### 4.3 Writer — trích dẫn `[§1]` nhảy đúng đoạn: **PASS**

Tài liệu 3 đoạn (nghỉ phép 12 ngày; nghỉ ốm 10 ngày; thai sản 6 tháng). Hỏi *"Nghỉ phép năm bao nhiêu ngày?
Trích dẫn số đoạn [§n] để tôi bấm nhảy tới."* → `writer.outline`, trả lời có `[§1] [§2] [§3]`. Bấm `[§1]`:
`writer.selection` trả `start=0, end=47, text="Nhân viên được nghỉ phép năm 12 ngày làm việc.\r"` — đúng đoạn 1.

### 4.4 Writer — "Hoàn tác lượt này" sau lượt sửa 2 chỗ: **PASS (qua route của nút)**

Lượt sửa `14 ngày`→`15 ngày` và `làm việc`→`làm việc chính thức` (hai lệnh `writer.replaceAll`, 2 bước undo).
`POST /v1/runs/<id>/undo` trả `{"mode":"steps","result":{"undone":2}}`; `writer.getText` sau đó trả lại
**"14 ngày làm việc"**, hai đoạn còn lại (`10 ngày`, `6 tháng`) **không bị lùi** — không lùi vào chữ có từ trước.

Ghi chú thao tác: nút *Hoàn tác lượt này* nằm ở hàng trạng thái sát mép dưới pane, bị cắt một phần; click
chuột tổng hợp của `xdotool` tại vị trí đó không phát sự kiện (đã thử focus pane trước). Route mà nút gọi
(`POST /v1/runs/{id}/undo`) được kiểm trực tiếp và đúng. Đây là hạn chế điều khiển GUI, không phải lỗi handler.

### 4.5 Đổi qua lại Writer ↔ ET trong 5 phút: **FAIL khi cả hai app cùng mở**

Mở đồng thời Writer và ET rồi hỏi trong pane Writer: agent đọc **`et-div0.xlsx`** và gọi `et.sheetInfo`,
`et.readRange`, `et.checkRange` — trả lời về bảng tính, không về tài liệu Writer. Đo `/health` + `/commands`
mỗi 2 giây cho thấy bridge **đảo app liên tục**:

```
1 app=et  first=app.info:28
...
6 app=wps first=app.info:21
...
8 app=et  first=app.info:28
```

Nguyên nhân: bridge WPS có **một hàng đợi và một phiên**, `app` = app poll gần nhất (đã ghi ở
`docs/architecture/wps-bridge.md` §6). Hai tiến trình WPS cùng poll nên app "đang mở" không xác định.
Khi **chỉ một** app mở, catalog đúng: ET-only 28 lệnh `et.*`, Writer-only 21 lệnh `writer.*` (đã chạy lại).

Chưa sửa trong lượt này: cần định tuyến job theo app (mỗi job mang `targetApp`, chỉ giao cho poll của app đó),
là thay đổi nhiều tầng (wpsbridge + office client + agent) chứ không phải sửa tối thiểu. Ghi ở mục 7.

### 4.6 Lệnh chỉ có ở Windows — từ chối rõ ràng, không treo: **PASS**

Gọi qua bridge trên đúng app (ET cho `et.*`, Writer cho `writer.*`), mỗi lệnh trả ngay:

| Lệnh | Kết quả trên WPS |
|---|---|
| `et.pivot` | `{"ok":false,"error":"unknown action: et.pivot"}` |
| `et.exportChart` | `{"ok":false,"error":"unknown action: et.exportChart"}` |
| `writer.setTracking` | `{"ok":false,"error":"unknown action: writer.setTracking"}` |
| `writer.listComments` / `writer.addComment` / `writer.replyComment` | `unknown action: …` |
| `writer.listRevisions` / `writer.reviewRevisions` | `unknown action: …` |

Gọi sai app trả `"et.sort" cannot run in "WPS Docs" (this command needs the other WPS application)`.
Không lệnh nào treo; một lần gọi ngay sau khi `kill` tiến trình ET trả rỗng do bridge còn coi app cũ là
"connected" trong ~45 giây, gọi lại trả ngay — đúng ngưỡng `AXIOM_WPS_DISCONNECT_SECONDS` đã ghi.

### 4.7 `et.sort` / `et.filter` trên WPS thật: **PASS**

- `et.sort {range:"A1:B4", keys:[{column:"Diem", order:"desc"}]}` → `{"sheet":"Data","range":"A1:B4","rows":3,"keys":[{"column":2,"order":"desc"}]}`;
  đọc lại `A1:B4` = `Ten/Diem`, `Chi/9`, `Binh/8`, `An/7` (giữ nguyên hàng tiêu đề).
- `et.filter {range:"A1:B4", column:"Diem", criteria:">7"}` → `visibleRows: 2`.
- `et.filter {… values:[7]}` → `visibleRows: 1` (sau khi ép chuỗi, xem 3.2); `column:"Ten", values:["Chi"]` → 1.
- `clear: true` → `{"cleared":true}`; dữ liệu không bị xoá.

## 5. Spike API WPS JS (chỉ đọc + thử, WPS 12.1.2.29182)

Cách đo: handler tạm `debug.spike` trong bản add-in **ở /tmp** (không commit), gọi qua `POST /cmd`.

| API | Có trên 12.1.2? | Kết quả đo |
|---|---|---|
| `Application.UserName` | **Có** | đọc `"thuyetmt"`; ghi được (`"Axiom Spike"`), đã khôi phục |
| `Document.TrackRevisions` | **Có trên Writer** | `boolean`, ghi được `true` rồi trả lại; **không có** trên workbook ET (`undefined`) |
| `Document.Comments` | **Có trên Writer** | collection: `Count=0`, `Add` là function |
| `Document.Revisions` | **Có trên Writer** | collection: `Count=0`, `Item` là function |
| `Range.Sort` | **Có** | `function`; sắp xếp thật đúng (3 khoá theo vị trí) |
| `Range.AutoFilter` | **Có** | `function`; áp dụng thật (`AutoFilterMode=true`); `values` phải là **chuỗi** |
| `Workbook.PivotCaches` | **Có** (dạng hàm) | `doc.PivotCaches()` trả collection, `Count=0`; chưa thử tạo cache từ vùng dữ liệu |
| `Chart.Export` | **Có** | `Shapes.AddChart(...).Chart.Export(path,"PNG")` ghi file PNG thật (7.167 byte); `Workbook.Charts.Count` vẫn 0 với chart dạng shape |
| `Worksheet.Sort` (Excel 2007+) | **Không** | chỉ có `Range.Sort` |

## 6. Việc còn lại

1. **Định tuyến job theo app** để Writer ↔ ET cùng mở vẫn đúng (mục 4.5). Hướng: job mang `targetApp`, poll
   khai app, bridge chỉ giao job khớp app; pane gửi app của nó khi tạo run.
2. **Viết handler** cho các lệnh đã chứng minh API tồn tại nhưng chưa làm: `writer.setTracking` (TrackRevisions +
   UserName), comment/revision (`writer.listComments`/`addComment`/`replyComment`/`listRevisions`/`reviewRevisions`),
   `et.pivot` (PivotCaches), `et.exportChart` (Chart.Export). Lượt này chỉ ghi kết quả spike.
3. **Binary Core đang cài** (`~/.local/share/axiom-office/core/AxiomOffice.Core`, bản 20/10) vẫn chưa có
   `trace_formula` và fix cache `/commands`; muốn dùng trên máy cài phải thay binary mới.
4. Hai lỗi staticcheck U1000 ở `internal/tools/template_compose.go` vẫn còn (dead code cũ, ngoài phạm vi WPS).
5. Nút *Hoàn tác lượt này* ở mép dưới pane bị cắt; nên kiểm lại bố cục pane ở cửa sổ thấp (chưa sửa vì chưa
   chứng minh được là lỗi CSS chứ không phải do cửa sổ 1080p của máy đo).

## 7. Trạng thái máy sau khi đo

- WPS đang chạy bằng **HOME tạm** `/tmp/opencode/wps2/home`; hồ sơ WPS thật của người dùng không bị sửa.
- `~/.local/share/Kingsoft/wps/jsaddons/jsaddinblockhost.ini` **không bị chạm**.
- Add-in trong `~/.local/share/axiom-office/core/wps-addin/` đã được chép bản `src/AxiomOffice.WPS` **có fix**
  (ribbon + `et.sort`/`et.filter`) để lần mở WPS sau của người dùng dùng được.
- Cấu hình đã khôi phục: `LlmModel=oc/beam-free`, `DebugTrace=0`; service `axiom-office-core` được bật lại.
