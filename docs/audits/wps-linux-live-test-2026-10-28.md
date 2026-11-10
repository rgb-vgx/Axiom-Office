# Kiểm thử thực tế WPS Office for Linux và ba lỗi đã sửa

Ngày đo: **28/10/2026** · máy: Kubuntu 22.04 (KDE/X11) · WPS Office **12.1.2.29182.AK.preread.sw**
(`Application.Version` = 12.0), cài tại `/opt/kingsoft/wps-office`

- Baseline: commit `24f74f3` (nhánh `claude/inspiring-davinci-1fxpeg`).
- Nhánh sửa: `wps` — `d01d235` (add-in JS) và `5661243` (Core Go), đã push lên `origin/wps`.
- Phạm vi: build, chạy toàn bộ bộ test, live test với WPS thật trên Excel và Writer, chẩn đoán lỗi, sửa và push.

Đây là báo cáo kiểm thử có sửa mã nguồn (khác với `axiom-agent-runtime-deep-audit.md` chỉ đọc). Mọi kết luận dưới
đây đều có lệnh chạy lại được hoặc log/tệp kết quả trên máy đo.

---

## 1. Kết quả các lệnh kiểm thử

| Lệnh | Kết quả | Ghi chú |
|---|---|---|
| `make check-gen` | **PASS** | catalog, VERSION, i18n (417 khoá, thiếu 0, thừa 0) khớp |
| `make test-go` (`go test -race ./...`) | **PASS** | toàn bộ package `core-go/internal/...`, gồm test hồi quy mới của `office` |
| `make test-core` | **173 passed, 0 failed** | `tests/core/test_core_e2e.py` (fake_bridge, guards, skills, memory, mcp, visual, setup, shutdown, anthropic, embeddings, summarize, cancel, mcp_http, verify) |
| `make test-wps` | **210 passed, 0 failed** | `test_install_jsaddon.py` + 11 tệp `node --test` |
| `make test-wps-e2e` | **36 passed** (bridge) + **94 passed** (pane) | `test_wps_bridge_e2e.py`, `test_wps_pane_e2e.py` |
| `make lint-js` | **PASS** | eslint sạch |
| `make lint-go` | **FAIL — chỉ 2 lỗi U1000 có sẵn** | `estimateLines` (`template_compose.go:491`), `cardMaxFont` (`:501`); tệp không thuộc phạm vi WPS, giữ nguyên |

---

## 2. Ba lỗi tìm được và cách sửa

### 2.1 `appKind` không nhận tên "WPS Sheets" (add-in JS)

WPS 12.1.2.29182 báo `Application.Name` của bảng tính là **`WPS Sheets`**, không phải `WPS Spreadsheets` như các
bản trước. `appKind()` trong `src/AxiomOffice.WPS/commands.js` chỉ khớp tên cũ, nên:

- app kind trả về `wps` → add-in gửi catalog đã lọc theo `wps` (không có lệnh `et.*`);
- Core lọc catalog theo `appKind` của phiên (`et`) → **danh sách lệnh rỗng**; agent không gọi được lệnh bảng tính
  nào và tự nghĩ ra tên lệnh.

Sửa: nhận cả `WPS Sheets` (giữ `WPS Spreadsheets` cho bản cũ).

### 2.2 Thiếu handler `et.readRanges` (add-in JS)

Catalog C# (bản Windows) có `et.readRanges` nhưng add-in WPS chưa có handler. Đã thêm `cmdEtReadRanges` + helper
(`etRangeWithSheet`, `etFormulaMatrix`, `etValueMatrix`) bám đúng contract `EtReadRanges`
(`src/AxiomOffice/Bridge/CommandDispatcher.SpreadsheetBatch.cs`):

- wrapper `items`/`item`/`value`, `sheet`/`formulas` ở cấp trên làm giá trị mặc định;
- prefix `Sheet!A1:B6` (dấu `!` cuối cùng, bỏ quote, `''` → `'`);
- giới hạn 4000 ô, kiểm **trước** khi đọc;
- thông báo lỗi khớp nguyên văn bản C#.

Test: đọc 2 sheet, wrapper, 8 trường hợp lỗi tham số, giới hạn ô.

### 2.3 Cache `/commands` của Core khoá theo port — WPS dùng chung một port

`BridgeClient.GetCommands` cache catalog theo **port** với TTL 5 phút
(`core-go/internal/office/bridge.go`). Bridge WPS phục vụ **cả ba app trên cùng port 47861**, nên khi đổi app
trong vòng 5 phút (Writer → ET), lượt agent nhận catalog của app cũ; `appKind = et` lọc sạch các lệnh `kind = wps`
→ `office_action` rỗng (`Available actions are: . Only these exist; do not invent action names.`), model gọi liên
tiếp 14 tên lệnh tự nghĩ ra và trả lời sai.

Sửa:

- cache khoá `"port|app"`; thêm `GetCommandsFor(ctx, port, appKind, force)`; `GetCommands` cũ giữ nguyên hành vi
  (khoá port, appKind rỗng) cho các chỗ gọi không biết app;
- orchestrator dùng `GetCommandsFor` với `appKind` của phiên;
- test hồi quy: đổi app trên cùng port phải gọi lại `/commands`, quay lại app cũ vẫn hit cache.

---

## 3. Chẩn đoán: vì sao add-in không nạp trong tiến trình WPS mới

Triệu chứng: tab "Axiom Office" không hiện, không poll. Log HTTP của Core chỉ thấy hai request
`manifest.xml` + `ribbon.xml` rồi dừng.

Nguyên nhân gốc **không phải** WPS chặn add-in, mà là **Core phục vụ add-in từ `exeDir/wps-addin`**
(`wpsbridge.ResolveOptions`, ghi đè bằng `AXIOM_WPS_ADDIN_DIR`). Bản Core build tay chạy ở `/tmp/opencode/` resolve
thành `/tmp/opencode/wps-addin` — thư mục không tồn tại → **mọi tệp add-in 404** → WPS dừng ở `ribbon.xml`.

Thứ tự request đo được trên WPS 12.1.2 (đã ghi vào `tests/wps/REFERENCE.md`):

1. `GET <url>manifest.xml` — **không bắt buộc** (404 vẫn đi tiếp);
2. `GET <url>ribbon.xml` — **bắt buộc 200**; thiếu là dừng im lặng, không có tab, không báo lỗi;
3. `GET <url>index.html` → `probe.js`, `commands.js`, `main.js` → `POST /wps/v1/poll`.

Bài học phụ: app đang mở từ trước vẫn "chạy được" sau khi đổi Core, vì JS đã nạp từ Core cũ và chỉ poll tiếp sang
Core mới. Chỉ khi restart WPS sạch mới lộ 404.

`~/.local/share/Kingsoft/wps/jsaddons/jsaddinblockhost.ini` là danh sách chặn do **chính WPS ghi** (cơ chế giống
Windows: add-in làm chậm khởi động bị tự động tắt). Tệp này **không** phải nguyên nhân lần này và **không được
xoá/sửa** để lách; cách đúng là giữ add-in khởi động nhanh.

---

## 4. Live test với WPS thật

Thiết lập: Core build sạch từ nhánh `wps`, chạy với
`AXIOM_WPS_ADDIN_DIR=~/.local/share/axiom-office/core/wps-addin`; add-in deploy từ `src/AxiomOffice.WPS`;
model dùng cho lượt đo: `ocg/deepseek-v4.1-flash` (free tier của OpenCode trả 429 trong ngày đo).

### 4.1 Excel — `trace_formula` chỉ đúng ô gốc lỗi

Dữ liệu trên sheet `Trống`: `A1=10`, `B1=0`, `C1==A1/B1`, `D1==C1*2`; chọn `D1`; hỏi nguyên văn
**"vì sao ô này lỗi?"**.

Agent gọi lần lượt `et.sheetInfo` → `et.readRanges` → **`trace_formula` (C1, direction precedents)** →
`et.readRange` → `et.checkRange`, rồi trả lời: ô lỗi là **C1** (`=A1/B1`, mẫu số B1 = 0 → `#DIV/0!`), D1 lỗi theo;
trích dẫn `[Trống!C1]`, `[Trống!D1]` bấm được.

- Run: `r_cc2f4639a4755f38309be3d142fcca54`
- Kết quả: `~/Documents/axiom-live-test/et-trace-final2.json`

### 4.2 Writer — trích dẫn `[§n]` từ `writer.outline`

Tài liệu 3 đoạn: nghỉ phép năm 12 ngày; nghỉ ốm tối đa 10 ngày/năm; nghỉ thai sản 6 tháng. Hỏi
**"Nghỉ phép năm bao nhiêu ngày? Trích dẫn số đoạn [§n] để tôi bấm nhảy tới."**

Agent gọi **`writer.outline`** (n = 1..3) và trả lời *"12 ngày làm việc **[§1]**"* — đúng đoạn chứa câu trả lời.
Tab **Axiom Office** hiện trên ribbon Writer (ảnh chụp xác nhận trong phiên đo).

- Run: `r_5838caa3c388722dc0703082adc32f78`
- Kết quả: `~/Documents/axiom-live-test/writer-citation-final.json`

### 4.3 Bằng chứng khác

- Log Core: `~/.local/share/axiom-office/core.log`.
- Trace từng lượt (khi bật `DebugTrace`): `~/.local/share/axiom-office/traces/<runId>.jsonl` — transcript ghi rõ
  từng `office_action`/`trace_formula` và kết quả.
- Health bridge: `GET http://127.0.0.1:47861/health` → `app=et|wps`, `connected=true`.

---

## 5. Trạng thái máy sau khi đo

- `axiom-office-core.service` chạy lại bình thường; add-in kết nối lại (`app=et`, `connected=true`).
- Add-in trong `~/.local/share/axiom-office/core/wps-addin/` đã là bản có hai fix JS.
- Cấu hình đã khôi phục: `LlmModel=oc/beam-free`, `DebugTrace=0`.
- `jsaddinblockhost.ini` giữ nguyên (không xoá).

## 6. Việc còn lại

- **Binary Core đang cài** (`~/.local/share/axiom-office/core/AxiomOffice.Core`, bản 20/10) **chưa có**
  `trace_formula` và fix cache §2.3. Muốn hai tính năng này chạy trên máy cài đặt phải build lại và thay binary
  (hoặc chạy bản build mới với `AXIOM_WPS_ADDIN_DIR` như lúc đo).
- Hai lỗi staticcheck U1000 ở `internal/tools/template_compose.go` vẫn còn (dead code cũ, ngoài phạm vi WPS).
- Free tier của OpenCode trả 429 account-wide trong ngày đo; các lượt live phải dùng model `ocg/*`.
