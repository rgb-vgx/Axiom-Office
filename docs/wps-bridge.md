# Bridge WPS Office for Linux (JS add-in do Agent Core phục vụ)

Trạng thái: **đã chạy trên WPS thật** (11/10/2026): Core (T1), trình cài `--core-url` (T2a), add-in 16 lệnh đợt 1
(T2b, T2c). Đóng gói + `install.sh` (T3) xong, đăng ký cả ba app (T5) xong, nhận biết app + đủ 38 lệnh
`et.*`/`wpp.*` (T6–T8) xong. Còn lại: pane Ask AI trong WPS. Cơ sở thực nghiệm:
[wps-spike.md](wps-spike.md) (hướng B) và [tests/wps/REFERENCE.md](../tests/wps/REFERENCE.md).

## 1. Bài toán

Mọi thứ trong Axiom Office (agent của Core, MCP làn live, script) nói với app văn phòng qua **giao thức
bridge**: file session + `GET /health`, `GET /commands`, `GET /session`, `POST /cmd` trên một cổng loopback
(LibreOffice_arch.md LD-2). Add-in COM (Windows) và extension LibreOffice tự mở HTTP server trong process app.

JS add-in của WPS Linux **không mở được cổng nghe** — nó chỉ là trang web trong CEF. Nên đảo chiều: add-in
kết nối *tới* Core, còn Core đứng ra làm bridge thay cho WPS.

```
agent / MCP / script ──HTTP bridge (/health,/commands,/session,/cmd)──▶ Core: "bridge WPS" :47861
                                                                          │ hàng đợi lệnh
WPS Writer ── JS add-in (nạp từ http://127.0.0.1:47840/wps/addin/) ◀──long-poll /wps/v1/poll──┘
                     │ thực thi lệnh qua window.Application (UndoRecord)  ──/wps/v1/result──▶ Core
```

## 2. Phía Agent Core (Go)

### 2.1 Phục vụ file add-in — `GET /wps/addin/<file>`
- Thư mục nguồn: biến môi trường `AXIOM_WPS_ADDIN_DIR`, mặc định `<thư mục chứa binary Core>/wps-addin`
  (gói phát hành chép `src/AxiomOffice.WPS/` vào đó, giống `skills/`).
- **Không cần token** (WPS tải `ribbon.xml`/`index.html` không kèm header), chỉ `GET`, không liệt kê thư mục,
  chặn `..`/đường dẫn tuyệt đối (404). File không có bí mật. `Content-Type` theo đuôi (`.html`, `.js`, `.xml`,
  `.css`, `.png`, `.svg`, `.json`). Request có `Origin` khác origin của Core → 403 như mọi endpoint.

### 2.2 Kênh add-in — `/wps/v1/*`
- **Xác thực**: chấp nhận khi header `Origin` **đúng bằng origin của chính Core** (`http://127.0.0.1:<cổng Core>`
  — trang do Core phục vụ; trình duyệt không cho trang khác giả `Origin`), hoặc khi không có `Origin` mà có
  `X-Auth-Token` đúng (tool/test). `Origin` khác → 403; không `Origin` + sai/thiếu token → 401.
  Body vẫn phải `application/json` (415).
- `POST /wps/v1/poll` — body `{client, app:"wps", version, document:{name, fullName}, commands:[catalog]}`.
  Giữ request tới khi có lệnh hoặc hết `AXIOM_WPS_POLL_SECONDS` (mặc định 20) → `{"ok":true,"result":{"jobs":[{id, action, params}]}}`
  (`jobs` rỗng khi hết giờ). Mỗi lần poll = một heartbeat; `commands` và `document` lần mới nhất được ghi nhớ.
- `POST /wps/v1/result` — body `{id, ok:true, result}` hoặc `{id, ok:false, error}` → `{"ok":true}`;
  `id` không tồn tại/đã xong → `{"ok":false,"error":...}` (HTTP 200).
- **Đã kết nối** = có poll trong `AXIOM_WPS_DISCONNECT_SECONDS` giây gần nhất (mặc định 45).

### 2.3 Bridge WPS — cổng riêng `AXIOM_WPS_BRIDGE_PORT` (mặc định 47861)
Cùng giao thức và cùng guard với bridge LibreOffice (`Origin` → 403, token → 401 trừ `/health`, body JSON → 415):
- `GET /health` → `{app, family:"wps-linux", pid:<pid Core>, port, version, connected}`. `app` là app mà add-in
  báo trong lần poll gần nhất: `"wps"` (Writer, mặc định khi chưa poll lần nào) `| "et" | "wpp"`.
- `GET /commands` → `{version, commands}` = catalog add-in gửi lần poll gần nhất (chưa từng kết nối → `ok:false`).
- `GET /session` → `{app, family, port, pid, document}`.
- `POST /cmd {action, params}` → chưa kết nối: `{"ok":false,"error":"WPS add-in is not connected: open a document
  in WPS to connect the add-in"}` (không nêu tên app cụ thể — cả ba app dùng chung một add-in);
  đã kết nối: đưa vào hàng đợi, chờ kết quả tối đa 300 giây, trả **nguyên** `{ok, result|error}` của add-in.
- **File session** `<thư mục session>/<pid Core>-wps.json` (cùng trường với LibreOffice: `pid, app,
  family:"wps-linux", port, host:"wps-jsaddon", version, started, lastSeen, lastSeenEpoch, document,
  documentPath`) — có khi add-in đã kết nối (ghi lại mỗi lần poll đổi tài liệu và mỗi 25 giây), **xoá** khi mất
  kết nối và khi Core dừng. Nhờ vậy `office_sessions`, agent và pane tìm thấy WPS như mọi bridge khác.
- Core lọc lệnh gửi agent theo `session.app` (`tools/office_action.go`: lệnh `kind` khác rỗng phải khớp app).
  Vì vậy `app` **phải** theo add-in, hard-code `"wps"` sẽ bỏ im lặng mọi lệnh `et.*`/`wpp.*`.

## 3. Phía add-in (`src/AxiomOffice.WPS/`, ES5 — CEF Chrome 87)
- Nạp từ `http://127.0.0.1:47840/wps/addin/` (`<jspluginonline>` trong `publish.xml`), nên gọi `/wps/v1/*`
  bằng URL tương đối, cùng origin, `fetch`/XHR `application/json` không preflight.
- **Một add-in cho cả ba app**: `publish.xml` khai `type="wps,et,wpp"` (xem `tests/wps/REFERENCE.md` — một entry
  nhiều `type` là cách duy nhất WPS nạp add-in trong cả Writer, Spreadsheets và Presentation).
- Biết app đang chạy: `AxiomCommands.appKind(app)` đọc `app.Name` (`/spreadsheet/i` → `"et"`,
  `/presentation|slide/i` → `"wpp"`, còn lại và tên lạ → `"wps"`). `catalogFor(kind)` lọc registry theo app
  (`kind` rỗng — `null` theo registry LibreOffice, ví dụ `app.info` — dùng chung mọi app); `catalog()` vẫn trả registry đầy đủ. Lệnh của app khác bị `execute()` từ
  chối (`{ok:false, error}` nêu tên lệnh và tên app) — không chạy nhầm object model.
- Vòng lặp: poll → thực thi từng job → gửi result → poll tiếp; lỗi mạng thì chờ 2 giây rồi thử lại. Body poll có
  `app` = app đang chạy, `commands` = catalog **đã lọc theo app đó**, `document` = tài liệu đang mở.
- **Registry lệnh** (`commands.js`): mỗi lệnh có metadata *giống hệt* registry của extension LibreOffice cho
  cùng tên (`name, kind, agent, summary, params[{name, required, hint}]`) và một handler gọi object model WPS
  (giống VBA/COM — viết theo handler C# trong `src/AxiomOffice/Bridge/CommandDispatcher.Writer.cs`).
  Lệnh sửa tài liệu chạy trong `Application.UndoRecord.StartCustomRecord("AI: <lệnh>")`/`EndCustomRecord()`
  ⇒ mỗi lệnh AI = một bước Undo. Lỗi handler → `{ok:false, error}`, không bao giờ làm treo vòng lặp.
- **54 lệnh** trong `commands.js`: `app.info` + 15 `writer.*` + 23 `et.*` + 15 `wpp.*` (metadata của cả 54 lấy từ
  registry LibreOffice). Chỉ 15 lệnh `writer.*` mở `UndoRecord` — ET/WPP không có nó (mục 6). Lệnh chưa có
  handler trả `unknown action: <lệnh>`.

## 4. Cài đặt
- Bước admin một lần mỗi máy: `JsApiPlugin=true` trong `/opt/kingsoft/wps-office/office6/cfgs/oem.ini`.
- `scripts/wps/install_jsaddon.py --home <HOME> --core-url http://127.0.0.1:47840` chỉ ghi `publish.xml`
  (`<jspluginonline name="axiomoffice" type="wps,et,wpp" url="<core-url>/wps/addin/" .../>` — một entry cho cả
  ba app). Khoá `Office.conf`
  (bỏ EULA, mở thẳng Writer) **chỉ** ghi khi có `--test-profile` — không đổi giao diện WPS của người dùng thật.
- Core phải chạy trước khi mở WPS (unit `systemd --user` đã có); Core tắt thì WPS bỏ qua add-in im lặng.
- Đã đăng ký xong mà WPS đang mở thì **phải mở lại WPS** mới thấy add-in (WPS đọc `publish.xml` lúc khởi động).

## 5. Triển khai theo task
| Task | Phạm vi | Spec |
|---|---|---|
| T1 | Core: 2.1–2.3 | `tests/wps/test_wps_bridge_e2e.py` (Core thật + add-in giả bằng HTTP) |
| T2 | Add-in: vòng poll + registry + handler `writer.*` | test Node với object model giả + đối chiếu registry LibreOffice |
| T3 | Trình cài `--core-url`, gói Linux, test chạy WPS thật | `tests/wps/test_wps_bridge_live.py` |
| T4 | Pane Ask AI trong WPS (mục 7) | `tests/wps/test_wps_pane_e2e.py` (Core) + `test_pane.mjs` (add-in) |
| T5 | Đăng ký một entry `type="wps,et,wpp"` cho cả ba app | `tests/wps/test_install_jsaddon.py` |
| T6 | Add-in nhận biết app đang chạy (`appKind`/`catalogFor`, từ chối lệnh app khác) + Core ghi `app` theo poll | `tests/wps/test_addin.mjs`, `core-go/internal/wpsbridge/wpsbridge_test.go` |
| T7 | Add-in: 23 lệnh `et.*` (Spreadsheets) | `tests/wps/test_addin_et.mjs` (+ `addin_fake.mjs`) |
| T8 | Add-in: 15 lệnh `wpp.*` (Presentation) | `tests/wps/test_addin_wpp.mjs` (+ `addin_fake.mjs`) |

## 6. Giới hạn đã biết
- Add-in chỉ nạp khi Core đã chạy lúc mở WPS (Core tắt thì WPS bỏ qua add-in im lặng) và khi `JsApiPlugin=true`;
  đăng ký add-in xong phải mở lại WPS.
- **Một add-in chỉ nói chuyện với một app tại một thời điểm**: bridge WPS có một hàng đợi và một file session, và
  `app` của phiên là app poll gần nhất. Mở Writer và Spreadsheets cùng lúc thì chỉ app nào vừa poll được phục vụ.
- `writer.undo {count}` gỡ N bước Undo gần nhất của tài liệu, **không phân biệt bước của AI hay của người**:
  object model WPS không cho đọc tên các bước Undo như LibreOffice (bản LibreOffice chỉ gỡ bước `AI: …`).
  Mỗi lệnh AI vẫn là đúng một bước Undo (UndoRecord).
- **ET/WPP không có `UndoRecord`** (`Application.UndoRecord` là `undefined`, `Application.Undo()` không đổi gì —
  đo trên WPS thật 11/10/2026). Nên `et.undo` không dùng undo của WPS: add-in **tự chụp giá trị ô
  trước khi ghi** (và ghi nhận sheet do lượt tạo) rồi khôi phục — khác LibreOffice (chỉ gỡ bước `AI: …`), và chỉ
  hoàn tác được các lệnh **trong cùng phiên add-in**. Registry **không có** `wpp.undo` (bản LibreOffice cũng vậy),
  và không lệnh `et.*`/`wpp.*` nào mở `UndoRecord`.
- **ET/WPP không xuất được PDF bằng API gốc**: `ExportAsFixedFormat`/`SaveAs(.pdf)` trả về bình thường nhưng
  không ghi file nào. `et.exportPdf`/`wpp.exportPdf` thử API gốc, kiểm bằng `FileSystem.existsSync`, không có thì
  tự dựng PDF (như LibreOffice). PDF này là bản tối giản: có chữ, không giữ định dạng/biểu đồ.
- `et.save`/`wpp.save` từ chối khi tài liệu **chưa lưu lần nào** (`Path` rỗng) bằng đúng câu của LibreOffice
  ("the document has never been saved - use saveAs with a 'path'"): `Save()` của WPS sẽ mở hộp thoại Save As và
  chặn vòng poll.
- WPP: `Font.Bold` nhận `msoTrue` = **-1** (không phải `true` như ET); `wpp.setNotes` ghi vào placeholder
  **cuối** của `NotesPage` (trang ghi chú đo được có 2 placeholder: 1 = số trang, 2 = ghi chú).
- `wpp.checkLayout` **không** báo `overflow` (cần chiều cao chữ thật, object model WPS không đo được); bốn loại
  còn lại (`offslide`, `small-font`, `overlap`, `dense`) giữ nguyên ngưỡng của LibreOffice.
- Object model ET/WPP khác Writer ở vài chỗ (`Cells.Item(r,c)`, `UsedRange.Address()` là hàm, màu BGR, chỉ số
  1-based, `Application.Documents` không có) — bảng đầy đủ ở `tests/wps/REFERENCE.md`.
- **`FileSystem.readFileString`/`writeFileString` bị sandbox** (đo 11/10/2026): mọi đường dẫn có `/` hoặc `\` đều
  ném `path cannot contains "/" or "\"`, tên file trần trả `null`, và chúng không đọc/ghi được file nào — dù tài
  liệu WPS mô tả chúng như API đọc file. Đọc file thật phải bằng **`FileSystem.ReadFile`** (UTF-8, ném
  `"Wrong path!"` khi thiếu file, cắt ở byte NUL đầu tiên); ghi byte bằng `writeAsBinaryString`. Đây là lớp lỗi
  mà fake "thân thiện" che mất: `et.importCsv` đã đọc bằng `readFileString` nên **không bao giờ chạy được trên WPS
  thật** trong khi 71 test node vẫn xanh. Bảng API đầy đủ ở `tests/wps/REFERENCE.md`,
  `tests/wps/test_addin_fake.mjs` ghim lại từng hành vi.
- **ET ép kiểu khi ghi `Value2`**: gán chuỗi trông-như-số (`"42"`) hay trông-như-ngày (`"01/01"`) thì WPS tự ép
  thành số/ngày — đường ghi **không** tạo được "số dạng text". `numbers-as-text` chỉ còn đúng với ô đã có sẵn
  text trong file. Fake **không** mô phỏng việc ép kiểu này (ghi chú ở `setCellScalar` trong `addin_fake.mjs`),
  nên spec chỉ khẳng định `numbers-as-text` trên ô gieo sẵn.

## 7. Pane Ask AI trong WPS
Pane là một trang khác của add-in (`/wps/addin/pane.html`), mở bằng nút trên ribbon qua
`Application.CreateTaskPane(url)`. Cùng origin với Core nên **không có token**: dùng kênh add-in với cùng xác thực
(`Origin` = origin của Core, hoặc token khi không có `Origin`):

| Endpoint | Tương đương | Ghi chú |
|---|---|---|
| `POST /wps/v1/runs {prompt, conversationId?, options?}` | `POST /v1/runs` | Core tự gắn `office` = bridge WPS, `document` = lần poll gần nhất; add-in chưa kết nối → **409** |
| `GET /wps/v1/runs/{id}` | `GET /v1/runs/{id}` | |
| `GET /wps/v1/runs/{id}/events` | SSE `/v1/runs/{id}/events` | `EventSource` cùng origin |
| `POST /wps/v1/runs/{id}/cancel` · `/confirm` | như `/v1/runs/...` | nút Dừng, thẻ xác nhận |

`/v1/runs` giữ guard cũ (mọi `Origin` → 403). Spec: `tests/wps/test_wps_pane_e2e.py` (T4a, phía Core).

Trình duyệt **không gửi `Origin` cho GET cùng origin** (`EventSource` của pane → 401, phát hiện 11/10/2026). Với
**GET** trên `/wps/v1/*`, Core chấp nhận thêm khi không có `Origin` mà có `Sec-Fetch-Site: same-origin` (header trình
duyệt tự đặt, trang khác không giả được; CEF Chrome 87 có gửi). POST vẫn cần `Origin`; `/v1/*` không đổi.
