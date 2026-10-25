# Bridge WPS Office for Linux (JS add-in do Agent Core phục vụ)

Trạng thái: **đã chạy trên WPS thật** (11/10/2026): Core (T1), trình cài `--core-url` (T2a), add-in 16 lệnh đợt 1
(T2b, T2c). Đóng gói + `install.sh` (T3) xong, đăng ký cả ba app (T5) xong, nhận biết app + đủ 38 lệnh
`et.*`/`wpp.*` (T6–T8) xong, pane Ask AI trong WPS (T4) xong, ribbon đầy đủ (đợt C — `docs/architecture/ribbon-ui.md`), menu
chuột phải *Sửa bằng AI* + khung chat mini (20/10/2026, đã thử trên WPS thật — mục 7.3).
Còn lại: **22/80 lệnh registry chưa có handler ở add-in WPS** (danh sách + lý do ở mục 6). Cơ sở thực nghiệm:
[docs/research/wps-spike.md](../research/wps-spike.md) (hướng B) và [tests/wps/REFERENCE.md](../../tests/wps/REFERENCE.md).

## 1. Bài toán

Mọi thứ trong Axiom Office (agent của Core, MCP làn live, script) nói với app văn phòng qua **giao thức
bridge**: file session + `GET /health`, `GET /commands`, `GET /session`, `POST /cmd` trên một cổng loopback
(docs/architecture/libreoffice.md LD-2). Add-in COM (Windows) và extension LibreOffice tự mở HTTP server trong process app.

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
- **Đã kết nối** = có poll trong `AXIOM_WPS_DISCONNECT_SECONDS` giây gần nhất (mặc định 45), **hoặc** đang có
  lệnh đã giao cho add-in mà chưa có kết quả. Vế thứ hai cần thiết vì add-in chạy đơn luồng: trong lúc chạy một
  lệnh dài nó không poll, nên nếu chỉ tính poll thì lệnh đang chạy bị coi là mất kết nối (mục 6).

### 2.3 Bridge WPS — cổng riêng `AXIOM_WPS_BRIDGE_PORT` (mặc định 47861)
Cùng giao thức và cùng guard với bridge LibreOffice (`Origin` → 403, token → 401 trừ `/health`, body JSON → 415):
- `GET /health` → `{app, family:"wps-linux", pid:<pid Core>, port, version, connected}`. `app` là app mà add-in
  báo trong lần poll gần nhất: `"wps"` (Writer, mặc định khi chưa poll lần nào) `| "et" | "wpp"`.
- `GET /commands` → `{version, commands}` = catalog add-in gửi lần poll gần nhất (chưa từng kết nối → `ok:false`).
- `GET /session` → `{app, family, port, pid, document}`.
- `POST /cmd {action, params}` → chưa kết nối: `{"ok":false,"error":"WPS add-in is not connected: open a document
  in WPS to connect the add-in"}` (không nêu tên app cụ thể — cả ba app dùng chung một add-in);
  đã kết nối: đưa vào hàng đợi, chờ kết quả tối đa `AXIOM_WPS_CMD_WAIT_SECONDS` giây (mặc định 300), trả **nguyên**
  `{ok, result|error}` của add-in. Hai câu lỗi hết giờ phân biệt rõ (mục 6): lệnh **chưa giao được** cho add-in thì
  `...disconnected before the command was sent (WPS closed?) - it was not sent, safe to retry`; lệnh **đã chạy** mà
  không có kết quả thì `...did not answer within Ns; the command may have been applied - check the document before retrying`.
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
- **80 lệnh** trong `commands.js` (đo 19/10/2026): 2 `app.*` + 1 `ai.*` + 2 `ui.*` + 28 `writer.*` + 23 `et.*` +
  24 `wpp.*` (metadata lấy từ registry LibreOffice). 58 lệnh đã có handler; chỉ **9 lệnh `writer.*`** mở
  `UndoRecord` (danh sách `MUTATING`) — ET/WPP không có nó (mục 6). Lệnh chưa có handler trả
  `unknown action: <lệnh>`; `tests/wps/test_addin.mjs` ghim danh sách lệnh registry chưa có handler (mục 6) và
  **bắt bỏ tên khỏi danh sách ngay khi viết handler**, để lệnh registry không âm thầm rơi vào `unknown action`
  khi máy đang chạy WPS.

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
| T4 | Pane Ask AI trong WPS (mục 7) | `tests/wps/test_wps_pane_e2e.py` (Core) + `test_pane.mjs`, `test_pane_theme.mjs` (add-in) |
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
  còn lại (`offslide`, `small-font`, `overlap`, `dense`) giữ nguyên ngưỡng của LibreOffice. WPS cũng **không** báo
  `connector-crossing`/`connector-overlap` — hai loại này chỉ có ở bản Microsoft Office.
- **Lệnh đang chạy không bị coi là mất kết nối** (vá 11/10/2026): add-in chạy **đơn luồng**, nên trong lúc thực thi
  một lệnh dài (`et.importCsv` file lớn, `et.formatRange` cả bảng, `exportPdf`) nó **không poll**. Nếu vẫn áp dụng
  luật `AXIOM_WPS_DISCONNECT_SECONDS` (45 giây) thì lệnh bị cắt giữa chừng: kết quả bị bỏ, file session bị xoá,
  trong khi lệnh **có thể đã ghi vào tài liệu** — agent nhận "mất kết nối" rồi thử lại và ghi hai lần. Nay "đang
  chạy" được tính là còn kết nối; chỉ lệnh **chưa giao** cho add-in mới bị bỏ khi mất kết nối, và câu lỗi nói rõ
  `safe to retry`. Lệnh đã giao mà không có kết quả vẫn hết giờ theo `AXIOM_WPS_CMD_WAIT_SECONDS` (với câu lỗi
  `may have been applied`); hết giờ này thì job được bỏ nên vòng session mới xoá file session.
- Object model ET/WPP khác Writer ở vài chỗ (`Cells.Item(r,c)`, `UsedRange.Address()` là hàm, màu BGR, chỉ số
  1-based, `Application.Documents` không có) — bảng đầy đủ ở `tests/wps/REFERENCE.md`.
- **22 lệnh trong registry (80 lệnh) chưa có handler ở add-in WPS** (19/10/2026). Nguồn duy nhất của danh sách
  là `CHUA_CO_HANDLER` trong `tests/wps/test_addin.mjs` — **đừng chép số vào đây**, spec sẽ đỏ khi một lệnh được
  viết handler mà còn nằm trong danh sách, và khi add-in khai với Core một lệnh chưa có handler. Chia làm ba nhóm:
  - **Bốn lệnh `kind` rỗng** — `ai.ask`, `ui.askpane`, `ui.setup`, `app.screenshot`. **Sửa lại một câu cũ ở đây
    (19/10/2026):** trước kia tài liệu này viết chúng "không phải lệnh của app văn phòng, add-in không bao giờ
    nhận" — **sai**. Ba trong bốn (`ai.ask`, `ui.askpane`, `app.screenshot`) **là** lệnh của add-in ở bản Windows
    (`CommandDispatcher.cs`: `AiAsk`, `UiAskPane`, `AppScreenshot`); chỉ `ui.setup` là đường riêng của
    LibreOffice. Ở add-in WPS, `app.info` có handler còn ba lệnh kia thì không — nên Core gọi chúng qua bridge sẽ
    nhận `unknown action`. Hệ quả đang thấy được: thiếu `app.screenshot` là **đúng lý do** toggle *Nhìn tài liệu*
    chưa lên được ribbon WPS (`docs/architecture/ribbon-ui.md` §5.1).
  - **Chín lệnh `writer.*`**: `newDocument`, `open`, `closeAll`, `formatTable`, `insertImage`, `insertHyperlink`,
    `replaceMany`, cộng hai lệnh thiết kế chỉ có ở add-in Windows (`restyle`, `insertBlocks`). `writer.checkTables`
    **đã ra khỏi danh sách** — handler viết xong (`commands.js:873`), nên nút *Soát bảng* trên ribbon chạy được.
  - **Chín lệnh `wpp.*`**: chỉ PowerPoint/đường compose có (`newFromTemplate`, `addSlideFromTemplate`,
    `sourceManifest`, `setShapeText`, `deleteShapes`, `copyPicture`, `addParagraphs`, `addGrid`, `deleteTagged`).
  - **Mười trong 22 lệnh có `agent=true`** (`writer.formatTable`, `writer.insertImage`, `writer.insertHyperlink`,
    `writer.replaceMany`, và sáu lệnh `wpp.*`) — nghĩa là Core **vẫn đưa chúng cho agent như tool**, và trên máy
    đang chạy WPS agent sẽ nhận `unknown action`. Đây là khoảng trống thật, không phải lỗi hiển thị.
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
`Application.CreateTaskPane(url)`. Trang thứ hai là `/wps/addin/mini.html` — **khung chat mini**, mở từ menu chuột
phải bằng `Application.ShowDialog(url, "Sửa bằng AI", 440 × devicePixelRatio, 380 × devicePixelRatio, false)`
(không modal; `main.js: openMini`) — mục 7.3. Cùng origin với Core nên **không có token**: dùng kênh add-in với cùng xác thực
(`Origin` = origin của Core, hoặc token khi không có `Origin`):

| Endpoint | Tương đương | Ghi chú |
|---|---|---|
| `POST /wps/v1/runs {prompt, conversationId?, options?}` | `POST /v1/runs` | Core tự gắn `office` = bridge WPS, `document` = lần poll gần nhất; add-in chưa kết nối → **409**. Gửi `{quickAction: "<id>"}` thay cho `prompt` = một mục của menu *Sửa bằng AI* (câu lệnh do Core lấy theo id; id lạ → 400). `options.requireSelection: true` (khung chat mini gửi kèm mọi lượt): không có đoạn bôi đen thì dừng ngay, `run.failed` kind `selection` (chưa bôi đen) hoặc `selection-lock` (có bôi đen nhưng add-in không khoá được) |
| `GET /wps/v1/runs/{id}` | `GET /v1/runs/{id}` | |
| `GET /wps/v1/runs/{id}/events` | SSE `/v1/runs/{id}/events` | `EventSource` cùng origin |
| `POST /wps/v1/runs/{id}/cancel` · `/confirm` | như `/v1/runs/...` | nút Dừng, thẻ xác nhận |
| `GET /wps/v1/memory` · `POST /wps/v1/memory` · `PATCH`/`DELETE /wps/v1/memory/{id}` · `POST /wps/v1/memory/{id}/restore` · `GET /wps/v1/memory/{id}/history` · `DELETE /wps/v1/memory` | `routeBoth` → y hệt `/v1/memory...` | màn hình *Bộ nhớ AI* |
| `GET /wps/v1/audit` | `GET /v1/audit` | màn hình *Nhật ký hoạt động* |
| `GET /wps/v1/conversations` · `GET`/`DELETE /wps/v1/conversations/{id}` | `routeBoth` | màn hình *Hội thoại trước…* (mục 7.2) |
| `POST /wps/v1/pane/request {quickAction}` · `GET /wps/v1/pane/events` (SSE) | **chỉ có ở kênh WPS** | trạm trung chuyển menu chuột phải → khung chat lớn đang mở (`api/wpspane.go`) — **đường dự phòng** khi WPS không có `ShowDialog`: `main.js` (`runQuick`) hiện khung chat lớn rồi gửi, khung chat Writer nghe sự kiện `pane.quick` rồi chạy mục đó **trong hội thoại của nó**. Mỗi yêu cầu giao cho **một** khung chat, giữ 20 giây cho khung chat vừa tạo kịp nối vào; bấm liền hai mục thì mục sau thắng |
| `GET /wps/v1/quick-actions` | `routeBoth` | `{actions: [{id, label, edits}]}`: khung chat mini dựng chip từ đây, cả hai khung dùng để bong bóng hiện nhãn ("Trau chuốt") thay vì id. Câu lệnh **không** trả ra — nằm trong `agent/quickactions.go` |
| `GET /wps/v1/setup` · `POST /wps/v1/llm/test` · `GET /wps/v1/llm/models` | `routeBoth` | màn hình *Thiết lập*, nút *Kiểm tra mô hình* |
| `GET /wps/v1/skills` · `POST /wps/v1/skills/reload` | `routeBoth` | màn hình *Kỹ năng* |
| `GET /wps/v1/log` | `GET /v1/log` (đăng ký riêng, chỉ khi bridge WPS bật) | màn hình *Mở nhật ký* (log Core, mục 5.2 của `docs/architecture/ribbon-ui.md`) |
| `GET /wps/v1/status` | **chỉ có ở kênh WPS** | gộp hai tầng: `{addin: <Bridge.Status()>, core: <coreStatus()>}` — pane không phải đoán `Origin` cho `/health` của bridge |
| `POST /wps/v1/exec {action, params}` | **chỉ có ở kênh WPS** | chạy thẳng một lệnh bridge, **không qua agent**: ba nút *Soát* trên ribbon và *Hoàn tác lượt này* / *Chèn trả lời* trong pane. 409 kèm `code`: `run-in-progress` (đang có lượt chạy) hoặc `addin-disconnected`; khi chạy được thì trả **nguyên phong bì** của add-in, HTTP 200 **kể cả khi add-in trả `ok:false`** — đọc `ok`, đừng đọc mã HTTP |
| `POST /wps/v1/config {TênKhoá: giá trị}` | **chỉ có ở kênh WPS** | ghi cấu hình nền tảng (Linux `config.json`, Windows HKCU) — đường thật đứng sau nút *Thiết lập*; khoá ngoài danh sách cho phép → 400 |

Phần lớn bảng trên đi qua **`routeBoth`** (`core-go/internal/api/wpsadmin.go`): cùng một handler đăng ký dưới
**cả hai** tiền tố — `/v1/*` cho script/MCP (guard cũ: token) và `/wps/v1/*` cho pane (guard kênh add-in). Nhờ vậy
pane không cần token mà vẫn đọc/ghi được đúng những gì bản Windows gọi. Các đường `routeBoth` luôn có cả hai tiền
tố; các đường **riêng** của kênh WPS (`runs*`, `status`, `exec`, `config`, `pane/*`, `log`) chỉ có khi bridge WPS
bật (`mapWPS` thoát sớm khi `deps.WPS == nil`). Bản chạy thật luôn bật bridge; khác biệt này chỉ lộ ra trong test.

`/v1/runs` giữ guard cũ (mọi `Origin` → 403). Spec: `tests/wps/test_wps_pane_e2e.py` (T4a, phía Core).

Trình duyệt **không gửi `Origin` cho GET cùng origin** (`EventSource` của pane → 401, phát hiện 11/10/2026). Với
**GET** trên `/wps/v1/*`, Core chấp nhận thêm khi không có `Origin` mà có `Sec-Fetch-Site: same-origin` (header trình
duyệt tự đặt, trang khác không giả được; CEF Chrome 87 có gửi). POST vẫn cần `Origin`; `/v1/*` không đổi.

### 7.1 Giao diện dùng chung hệ thiết kế với pane LibreOffice

Hai pane phải trông như **một sản phẩm**. `axiom/theme.py` (bản LibreOffice của `PaneTheme`,
`src/AxiomOffice/Ai/PaneControls.cs`) là **nguồn duy nhất** cho màu, cỡ chữ và khoảng cách — chính nó tự khai
"cùng màu, cỡ chữ, khoảng cách với pane của MS Office/WPS" — nên pane WPS không tự nghĩ ra bảng màu riêng (bản cũ
dùng màu Google Material: `#e8f0fe`, `#f1f3f4`, `#f9ab00`…).

- `pane.html` khai mọi token của `theme.py` trong một khối `:root` (`--axiom-<tên hằng viết thường>`) rồi dùng qua
  `var()`. Không còn mã màu `#rrggbb` nào ngoài khối đó, và không token nào khai mà không dùng. Cỡ chữ để bằng `pt`
  (theme.py tính bằng điểm; CSS `pt` = 96/72px nên ra đúng số pixel như bản LibreOffice, không cần bảng đổi).
- `pane-core.js` mang phần **dữ liệu** của hệ thiết kế (`ACTION_LABELS`, `SUFFIX_LABELS`, `TOOL_LABELS`, `NOUNS`,
  `APP_NAMES`, `SUGGESTIONS`, `labelFor`, `emptyState`, `plainReply`) chép từ `theme.py`, nên nhãn lệnh, gợi ý và
  màn hình trống đọc ra giống hệt bản LibreOffice mà vẫn test được khi không có DOM.
- `tests/wps/test_pane_theme.mjs` (9 test) đối chiếu **trực tiếp** với `theme.py` qua `tests/wps/theme_dump.py`
  (không chép tay giá trị nào vào test), nên spec không lệch khỏi nguồn được.

**Bảng màu (19/10/2026):** `theme.py` đổi sang **xanh Fluent `#0F6CBD`** — màu của Copilot trong MS Word — và
`PaneControls.cs` (Windows) cùng `pane.html` (`:root`) đổi theo, ba bản vẫn khớp từng token. `theme_dump.py`
gom **mọi hằng int > 0xFFFF** ở cấp module trong `theme.py` thành một màu, nên thêm một hằng màu là **bắt
buộc** thêm một token `--axiom-*` có dùng thật, nếu không `test_pane_theme.mjs` đỏ. Đổi màu thì phải sửa **cả
ba** tệp (cộng `mini.html` — `:root` của nó chép nguyên từ `pane.html`, `test_mini_view.mjs` bắt hai bản giống
hệt) — và cả **hai tệp `src/AxiomOffice.WPS/images/*.svg`** (icon menu chuột phải) nếu muốn cùng màu, vì ảnh do
WPS vẽ bằng bộ nạp ảnh của nó (nó không biết `var()`); `test_pane.mjs` kiểm màu từng tệp. Icon trên ribbon là
`imageMso` (bộ icon của WPS) nên không theo bảng màu này. Xem `docs/architecture/ribbon-ui.md` §5.3.

Pane biết mình đang ở app nào qua query string: `main.js:paneUrl()` thêm `?app=<wps|et|wpp>`
(`currentAppKind()`), pane đọc lại và mặc định `"wps"` nếu thiếu — nhờ đó tiêu đề và gợi ý đúng loại tài liệu
(Writer/Calc/Impress). `CleanAddinRel` đã cắt `?query`/`#fragment` trước khi resolve nên Core không phải đổi gì.

### 7.2 Pane WPS so với hai bản kia (đo 19/10/2026)

Pane WPS nay có **đủ bảy màn hình** (chat, *Bộ nhớ AI*, *Nhật ký hoạt động*, *Thiết lập*, *Kỹ năng*, *Trạng thái*,
*Mở nhật ký*) cộng màn hình *Soát tài liệu* gọi từ ba nút Soát trên ribbon và màn hình ***Hội thoại trước…***
(danh sách → chi tiết → xoá có xác nhận). Header WPS là **tiêu đề cộng nút ⋯** (giống Copilot trong Word) chứa
*Trò chuyện mới* / *Hội thoại trước…* / *Ghi nhớ* / *Nhật ký* / *Thiết lập* — bản LibreOffice vẫn là ba liên kết
chữ *Trò chuyện mới* / *Ghi nhớ* / *Thiết lập*; footer `Hoàn tác lượt này` / `Chèn trả lời` / `Dừng`, phụ đề `App · model`. Bốn dòng cũ của bảng
này ("chưa có route memory/setup", "chưa có route hoàn tác lượt", "pane không được cấp thông tin model", "không
có menu ⋯") **đã hết đúng** — route có từ `routeBoth` và `POST /wps/v1/exec`, menu ⋯ có từ đợt E.

Cách trình bày cũng đổi cho giống Word: **trả lời của AI không còn bong bóng** — chữ nằm thẳng trên nền pane như
Copilot, chỉ tin nhắn của người dùng mới có nền; nền và viền của thẻ chỉ còn dùng cho `.card` (tiến trình / kết
quả).

Còn thiếu thật (chưa vẽ nút vì chưa có đường, hoặc đường có mà UI chưa):

| Việc | Bản có nó | Tình trạng ở WPS |
|---|---|---|
| **Sửa / ghim / đặt hạn** ghi nhớ | chỉ Windows (`MemoryForm.cs`) — dialog *Ghi nhớ* của LibreOffice **cũng thiếu** ba việc này | màn hình *Bộ nhớ AI* của WPS mới **3/6** việc của nghiệm thu đợt 2 (xoá tạm, khôi phục, xoá vĩnh viễn) cộng đọc, lọc phạm vi, tải lại (LibreOffice còn thiếu cả khôi phục và xoá vĩnh viễn); `PATCH /wps/v1/memory/{id}` đã sẵn nhưng chưa có form |
| *Cài đặt nâng cao…*, *Tắt Agent Core…*, *Công cụ ngoài…*/MCP, **Tìm web**, menu ngôn ngữ | ribbon Windows | **cố ý không vẽ** trên WPS — xem `docs/architecture/ribbon-ui.md` §2 |

Những thứ WPS **có thêm** so với bản LibreOffice: *Nhật ký* và *Hội thoại trước…* trong menu ⋯, nút *Chèn trả lời*
ở footer, menu chuột phải *Sửa bằng AI* và khung chat mini (mục 7.3).

**Đã xong (không còn thiếu)** — *Icon ribbon*: như Windows, ribbon WPS dùng **`imageMso`** — bộ icon có sẵn của
WPS (đã thử trên WPS thật 20/10/2026; năm tên Windows dùng không có trong WPS nên được thay — bảng ở
`docs/architecture/ribbon-ui.md` §5.3). Menu chuột phải dùng `getImage` trả **đường dẫn tệp tương đối** `images/<tag>.svg`
(data URI thì WPS vẽ dấu `?` — đo 19/10/2026).

Khác biệt nhỏ còn lại: khi chưa biết model, phụ đề WPS rơi về `Axiom Office` còn bản LibreOffice rơi về
tên app.

Màu, cỡ chữ, khoảng cách thì khớp từng token; phần **nhìn thấy thật** (font hệ thống, scrollbar của CEF, độ nét
của chữ) chỉ kiểm chứng được khi mở pane trong WPS — spec chốt token, không chốt pixel.

### 7.3 Menu chuột phải *Sửa bằng AI* và khung chat mini (20/10/2026)

- `ribbon.xml` có `<contextMenus>` trên `ContextMenuText` và `ContextMenuList`: **Sửa bằng AI ▸** (9 mục, sinh từ
  `core-go/internal/agent/quickactions.go`, spec ghim hai bên) và **Hỏi AI về đoạn này**. **Đã thử trên WPS thật**:
  menu hiện và chạy trọn tới Core.
- Bấm một mục: `main.js: openMini` mở `mini.html` bằng `Application.ShowDialog` (không modal). Mục *Sửa bằng AI*
  chạy ngay (`?quick=<id>`); *Hỏi AI về đoạn này* mở ra chip + ô nhập. Khung mini dùng lại `pane-core.js` với
  `create({requireSelection: true})`: **mọi lượt** gửi `options.requireSelection`, không có đoạn bôi đen thì Core
  dừng trước khi gọi model. Có *Hoàn tác* (`writer.undo {count}`), *Dừng*, *Đóng* (đang chạy thì huỷ lượt trước).
  Người dùng thử trên WPS thật: "khá ổn"; chưa đo vị trí cửa sổ (`ShowDialog` không nhận toạ độ).
- Lượt từ lệnh nhanh có thêm luật trong system prompt: `QuickEditRule` (sửa ngay, không hỏi lại — lượt thật đầu
  tiên model tìm ra lỗi rồi *hỏi* "có áp dụng không?") hoặc `QuickAnswerRule` (ba mục chỉ trả lời; Core cho lượt
  đó bộ tool chỉ-đọc).
- WPS không có `ShowDialog` (hoặc gọi lỗi) thì quay về khung chat lớn qua `POST /wps/v1/pane/request` → SSE
  `GET /wps/v1/pane/events` (bảng ở đầu mục 7).
- Spec: `tests/wps/test_mini_view.mjs`, các ca menu chuột phải trong `test_pane.mjs`, e2e
  `test_wps_pane_e2e.py` (lệnh nhanh, `requireSelection`, trạm trung chuyển).
