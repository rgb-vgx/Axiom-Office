# Bridge WPS Office for Linux (JS add-in do Agent Core phục vụ)

Trạng thái: **đã chạy trên WPS thật** (11/10/2026): Core (T1), trình cài `--core-url` (T2a), add-in 16 lệnh đợt 1
(T2b, T2c). Đóng gói + `install.sh` (T3) xong. Còn lại: pane Ask AI trong WPS, lệnh Spreadsheets/Presentation. Cơ sở thực nghiệm: [wps-spike.md](wps-spike.md)
(hướng B) và [tests/wps/REFERENCE.md](../tests/wps/REFERENCE.md).

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
- `GET /health` → `{app:"wps", family:"wps-linux", pid:<pid Core>, port, version, connected}`.
- `GET /commands` → `{version, commands}` = catalog add-in gửi lần poll gần nhất (chưa từng kết nối → `ok:false`).
- `GET /session` → `{app, family, port, pid, document}`.
- `POST /cmd {action, params}` → chưa kết nối: `{"ok":false,"error":"WPS add-in is not connected: open a document in WPS Writer ..."}`;
  đã kết nối: đưa vào hàng đợi, chờ kết quả tối đa 300 giây, trả **nguyên** `{ok, result|error}` của add-in.
- **File session** `<thư mục session>/<pid Core>-wps.json` (cùng trường với LibreOffice: `pid, app:"wps",
  family:"wps-linux", port, host:"wps-jsaddon", version, started, lastSeen, lastSeenEpoch, document,
  documentPath`) — có khi add-in đã kết nối (ghi lại mỗi lần poll đổi tài liệu và mỗi 25 giây), **xoá** khi mất
  kết nối và khi Core dừng. Nhờ vậy `office_sessions`, agent và pane tìm thấy WPS như mọi bridge khác.

## 3. Phía add-in (`src/AxiomOffice.WPS/`, ES5 — CEF Chrome 87)
- Nạp từ `http://127.0.0.1:47840/wps/addin/` (`<jspluginonline>` trong `publish.xml`), nên gọi `/wps/v1/*`
  bằng URL tương đối, cùng origin, `fetch`/XHR `application/json` không preflight.
- Vòng lặp: poll → thực thi từng job → gửi result → poll tiếp; lỗi mạng thì chờ 2 giây rồi thử lại.
- **Registry lệnh** (`commands.js`): mỗi lệnh có metadata *giống hệt* registry của extension LibreOffice cho
  cùng tên (`name, kind, agent, summary, params[{name, required, hint}]`) và một handler gọi object model WPS
  (giống VBA/COM — viết theo handler C# trong `src/AxiomOffice/Bridge/CommandDispatcher.Writer.cs`).
  Lệnh sửa tài liệu chạy trong `Application.UndoRecord.StartCustomRecord("AI: <lệnh>")`/`EndCustomRecord()`
  ⇒ mỗi lệnh AI = một bước Undo. Lỗi handler → `{ok:false, error}`, không bao giờ làm treo vòng lặp.

## 4. Cài đặt
- Bước admin một lần mỗi máy: `JsApiPlugin=true` trong `/opt/kingsoft/wps-office/office6/cfgs/oem.ini`.
- `scripts/wps/install_jsaddon.py --home <HOME> --core-url http://127.0.0.1:47840` chỉ ghi `publish.xml`
  (`<jspluginonline name="axiomoffice" type="wps" url="<core-url>/wps/addin/" .../>`). Khoá `Office.conf`
  (bỏ EULA, mở thẳng Writer) **chỉ** ghi khi có `--test-profile` — không đổi giao diện WPS của người dùng thật.
- Core phải chạy trước khi mở WPS (unit `systemd --user` đã có); Core tắt thì WPS bỏ qua add-in im lặng.

## 5. Triển khai theo task
| Task | Phạm vi | Spec |
|---|---|---|
| T1 | Core: 2.1–2.3 | `tests/wps/test_wps_bridge_e2e.py` (Core thật + add-in giả bằng HTTP) |
| T2 | Add-in: vòng poll + registry + handler `writer.*` | test Node với object model giả + đối chiếu registry LibreOffice |
| T3 | Trình cài `--core-url`, gói Linux, test chạy WPS thật | `tests/wps/test_wps_bridge_live.py` |

## 6. Giới hạn đã biết
- `writer.undo {count}` gỡ N bước Undo gần nhất của tài liệu, **không phân biệt bước của AI hay của người**:
  object model WPS không cho đọc tên các bước Undo như LibreOffice (bản LibreOffice chỉ gỡ bước `AI: …`).
  Mỗi lệnh AI vẫn là đúng một bước Undo (UndoRecord).
- Add-in chỉ nạp khi Core đã chạy lúc mở WPS (Core tắt thì WPS bỏ qua add-in im lặng) và khi `JsApiPlugin=true`.
- Đợt 1 chỉ có `app.info` + 15 lệnh `writer.*`; Spreadsheets (`et.*`) và Presentation (`wpp.*`) chưa có.

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

