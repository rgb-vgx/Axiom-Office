# Giao diện Axiom Office trên ribbon / menu — plan

Cập nhật: 20/10/2026 · trạng thái: **plan đã chốt; Windows/Windows-LibreOffice xong các đợt 1–3. WPS Linux đã
dựng đủ bộ nút; trên WPS thật đã thử icon ribbon, menu chuột phải *Sửa bằng AI* và khung chat mini (20/10) — các
nút ribbon thì chưa bấm thử từng nút (§5.3)**

Nguồn: một lượt tư vấn qua ChatGPT (bộ nút đầy đủ, phân tầng ưu tiên) rồi **đối chiếu lại với mã trong
repo**. Mọi dòng dưới đây đã được kiểm là *có thật trong mã*; chỗ nào chưa có API thì ghi rõ là **hoãn**,
không vẽ nút giả.

Hai phiên làm việc song song theo tài liệu này:

| Phiên | Phạm vi | Ghi chú |
|---|---|---|
| Máy Windows (có `csc` net48 + Office thật) | `src/AxiomOffice/` — add-in COM | Máy Linux **không có** `csc`/`mono`/`dotnet`, nên C# chỉ biên dịch được ở job `windows` của CI |
| Máy Linux (repo này) | `src/AxiomOffice.WPS/` + `src/AxiomOffice.LibreOffice/` | Kiểm được thật bằng `make lint`, `make test` |

## 1. Nguyên tắc

1. **Ribbon khởi động việc — pane điều phối — màn hình quản trị cấu hình.** Không nhân đôi giao diện.
2. **Chỉ MỘT tab "Axiom Office".** Người dùng chốt 13/10/2026: không có tab thứ hai, không tab "Quản lý"
   riêng. Màn hình quản trị nằm trong **nhóm *Quản lý* của chính tab đó**, nút ít dùng gom vào **menu *Khác ▸***
   cùng tab, và việc của khung chat (*Hội thoại*, *Nhật ký hoạt động*) nằm ở **menu ⋯ của pane** — vẫn một tab.
3. **Không biến API thành nút.** Nhãn là kết quả người dùng muốn ("Soát bố cục"), không phải tên lệnh
   (`wpp.checkLayout`).
4. **Không vẽ control giả.** Nút chỉ lên ribbon khi có đường thật đứng sau (API/lệnh đã có, hoặc ghi
   cấu hình mà Core đọc lại được).
5. **Không đổi hành vi lệnh bridge.** Các lệnh bridge (hợp đồng chung `catalog/live-commands.json`, 80 lệnh lúc
   20/10/2026) nằm trong hợp đồng parity với MCP; việc này
   chỉ **thêm UI**, không sửa tên/tham số/kết quả lệnh.

## 2. Bộ nút

Cột "Nền tảng": W = Windows COM · L = WPS Linux (ribbon) · O = LibreOffice (menu).

**Quyết định 13/10/2026 (người dùng):** chỉ **một tab "Axiom Office"** — không mở tab thứ hai "Axiom · Quản lý".
Màn hình quản trị vào nhóm **Quản lý** của chính tab đó; nút ít dùng gom vào menu **Khác ▸**. **Bỏ** hai nút
*Dừng* và *Xuất PDF* (pane đã có *Dừng*; xuất PDF để AI làm qua lệnh `*.exportPdf`). *Hội thoại* và *Nhật ký
hoạt động* **không lên ribbon** — chúng nằm trong menu **⋯** ở header của khung chat.

### Tab duy nhất — "Axiom Office"

| Nhãn | Nhóm | Control | Nền tảng | Đứng sau bằng gì | P |
|---|---|---|---|---|---|
| **Ask AI** | Trợ lý | button | W · **L ✅** · O | pane Ask AI (đã có) | P0 |
| **Nhìn tài liệu** | Kiểm tra | **toggle** | W ✅ · O ✅ · ~~L~~ | W: ghi HKCU `VisualQaEnabled`; **O: đã có** (checkbox trong "Cài đặt nâng cao", `dialogs.py:81`); **L: chặn** — xem §5.1. Core đọc lại **mỗi lượt** | P0 |
| **Soát bố cục** | Kiểm tra | button | **W ✅** · **L ✅** · **O ✅** | `wpp.checkLayout` — chỉ đọc, tất định, **0 token** | P0 |
| **Soát bảng** | Kiểm tra | button | W (chưa bấm) · **L ✅** · **O ✅** | `writer.checkTables` | P1 |
| **Soát vùng dữ liệu** | Kiểm tra | button | W (chưa bấm) · **L ✅** · **O ✅** | `et.checkRange` | P1 |
| **Bộ nhớ AI…** | Quản lý | button | W · **L ✅** · O | `GET/POST/PATCH/DELETE /v1/memory`, `/{id}/history`, `/{id}/restore` | P0 |
| **Kỹ năng…** | Quản lý | button | W · **L ✅** · O | `GET /v1/skills?app=` + `POST /v1/skills/reload` | P1 |
| **Thiết lập…** | Hệ thống | button | W · **L ✅** · O | wizard 5 bước (đã có) | P0 |
| **Trạng thái** | Hệ thống | button | W · **L ✅** · O | `GET /wps/v1/status` (một lời gọi, **cả hai tầng**: bridge + Core — §5.2) | P0 |
| **Mở nhật ký** | Hệ thống | button | W · **L ✅** | `GET /v1/log` + `/wps/v1/log` — trong CEF không mở được trình soạn thảo ngoài nên pane **đọc** phần cuối tệp (§5.2) | P1 |
| Khác ▸ **Cài đặt nâng cao…** | Hệ thống | menu | W · O · ~~L~~ | `SettingsForm` (đã có). Trên WPS **không vẽ**: màn hình *Thiết lập* của pane đã có đủ trường cấu hình, thêm nút mở lại đúng màn hình đó là nút giả | P0 |
| Khác ▸ **Kiểm tra mô hình** | Hệ thống | menu | W · **L ✅** · O | `POST /v1/llm/test` với cấu hình **đã lưu** (apiKey rỗng = khoá đã lưu) | P1 |
| Khác ▸ **Sao chép API URL** | Hệ thống | menu | W · **L ✅** | công cụ gỡ rối. Trên WPS sao chép `http://127.0.0.1:<bridgePort>/health` (cổng bridge lấy từ `/wps/v1/status`) — dán vào trình duyệt là chạy, vì chỉ `GET /health` của bridge được miễn token; API của Core thì **không** dán thẳng được (đòi token) | P1 |
| Khác ▸ **Tắt Agent Core…** | Hệ thống | menu | W · ~~L~~ | `POST /v1/admin/shutdown` — **bắt buộc xác nhận**. Trên WPS **không vẽ**: Core chính là thứ đang phục vụ pane, tắt nó là làm pane chết ngay trước mặt người dùng, mà đường `/v1/admin/shutdown` cũng không có trên kênh `/wps/v1/*` | P2 |

**"L ✅" = đã chạy thật trên Core thật, không cần WPS** (19/10/2026): `make test-wps-e2e` —
`tests/wps/test_wps_pane_e2e.py` gọi đúng những đường mà nút gọi, với add-in giả poll qua HTTP (94 phép kiểm, 20/10/2026).
Phép đo ghi ở `tests/wps/REFERENCE.md`. Việc **còn thiếu để "L ✅" thành nghiệm thu đầy đủ**: bấm nút trong
WPS thật (xem §5.3).

Vì sao gom vào menu chứ không thêm tab: cùng một tab thì người dùng không phải học "việc này ở tab nào"; đổi
lại hàng nút phải ngắn, nên thứ ít dùng rút vào **Khác ▸**. Ribbon tự thu nhóm khi cửa sổ hẹp, không cần cắt
bớt nút thật.

### Trong khung chat (không phải ribbon)

Từ 19/10/2026 header pane **WPS** là **tiêu đề + nút ⋯** (giống Copilot trong Word) chứa năm mục dưới đây. Hai
bản kia bố trí khác: **Windows** để *Trò chuyện mới* / *Thiết lập* thành liên kết ở header, menu ⋯ chỉ có
*Hội thoại trước…* / *Nhật ký hoạt động…* (`AskAiPane.cs`); **LibreOffice** chỉ có ba liên kết *Trò chuyện mới* /
*Ghi nhớ* / *Thiết lập* (`panel.py`), không có ⋯ — *Nhật ký hoạt động…* ở menubar.

| Nhãn | Chỗ | Nền tảng | Đứng sau bằng gì |
|---|---|---|---|
| **Trò chuyện mới** | menu **⋯** (WPS) · liên kết header (W, O) | W O **L ✅** | xoá khung chat đang hiện (không gọi Core; đoạn hội thoại cũ vẫn còn trong *Hội thoại trước…*) |
| **Hội thoại trước…** | menu **⋯** ở header pane | W · **L ✅** · ~~O~~ (chưa có) | `GET /v1/conversations` (danh sách) → `GET /v1/conversations/{id}` (kèm `messages[]`); `DELETE` để xoá. Windows lọc theo tài liệu đang mở; WPS liệt kê **30 hội thoại gần nhất của mọi tài liệu** (`conversations?limit=30`, không gửi khoá tài liệu) |
| **Ghi nhớ** | menu **⋯** (WPS) · liên kết header (O) · ribbon *Bộ nhớ AI…* (W) | W O **L ✅** | Windows: đủ `GET/POST/PATCH/DELETE /v1/memory` (`MemoryForm.cs`). WPS chỉ gọi `GET`, `DELETE /{id}`, `/{id}/restore`, `DELETE ?scope&confirm` — **không** sửa / ghim / đặt hạn |
| **Nhật ký** | menu **⋯** (W, WPS) · menubar *Nhật ký hoạt động…* (O) | W O **L ✅** | `GET /v1/audit` (100 dòng mới nhất, giờ máy) |
| **Thiết lập** | menu **⋯** (WPS) · liên kết header (W, O) | W O **L ✅** | W/O: wizard 5 bước. WPS: `GET /wps/v1/setup` (đọc) + `POST /wps/v1/config` (ghi) — Core **không** có `POST /v1/setup` |
| ~~**✦ Sửa bằng AI ▸** ở chân ô soạn~~ — **đã bỏ 20/10/2026** (người dùng: chỉ để ở menu chuột phải). Chín mục: Trau chuốt · Viết lại · Rút gọn · Mở rộng · Sửa lỗi chính tả · Trang trọng hơn │ Tóm tắt · Giải thích · Dịch | nay **chỉ** ở menu chuột phải (dòng dưới) | ~~W O~~ **L ✅** | `GET /v1/quick-actions` (danh sách) → `POST /v1/runs {quickAction}`. Câu lệnh nằm **trong Core** (`agent/quickactions.go`), ba bản chỉ gửi id. Nhóm sau vạch ngăn **chỉ trả lời**: Core cho lượt đó bộ tool chỉ-đọc của chế độ lập kế hoạch, nên "không sửa tài liệu" là Core **chặn thật**. Không bôi đen gì → dừng **trước khi gọi model**, `run.failed` kind `selection` (pane nói "Hãy bôi đen…") |
| **Chuột phải** khi bôi đen ▸ **Sửa bằng AI ▸** (9 mục như trên) · **Hỏi AI về đoạn này** | menu ngữ cảnh `ContextMenuText` + `ContextMenuList` (chỉ Writer) | W (chỉ *Hỏi AI…*) · **L ✅ menu hiện trên WPS thật** | `ribbon.xml` `<contextMenus>` → `main.js` mở **khung chat mini** `mini.html` bằng `Application.ShowDialog` (không modal, 440×380): mục *Sửa bằng AI* chạy ngay (`?quick=<id>`), *Hỏi AI về đoạn này* mở ra chip + ô nhập. Gõ thêm ("ngắn hơn nữa") là lượt tiếp trong cùng hội thoại, **mọi lượt** gửi `options.requireSelection`. Có *Hoàn tác*, *Dừng*, *Đóng* (đang chạy thì dừng lượt trước khi đóng). Bản WPS không có `ShowDialog` → quay về khung chat lớn qua `POST /wps/v1/pane/request` → `GET /wps/v1/pane/events` |

**Vì sao *Sửa bằng AI* nằm ở menu chuột phải**: bôi đen rồi chuột phải là đúng chỗ tay đang ở. Lịch sử: bản đầu
(19/10/2026) đặt menu ở **chân ô soạn**, vì trên WPS ribbon không nói thẳng được vào pane đang mở (§7). Ngày 20/10
có đường vòng qua Core (§7) và menu chuột phải; theo yêu cầu người dùng menu ở ô soạn **bị bỏ**. Cùng ngày (đợt I)
mục chuột phải chuyển sang mở **khung chat mini** riêng (`mini.html`, hội thoại riêng của khung đó); đường vòng qua
Core chỉ còn là **dự phòng** khi WPS không có `ShowDialog` — khi đó mục chạy trong hội thoại của khung chat lớn.
Hai khung vẫn hỏi `GET /quick-actions` để bong bóng hiện nhãn ("Trau chuốt") thay vì id, và khung mini dựng chip.
Bản Windows và LibreOffice **chưa** có menu này; Core đã sẵn (`quickAction` là một trường của `POST /v1/runs`),
nhưng bản LibreOffice phải có **khoá vùng bôi đen** trước — thiếu khoá thì một mục *Trau chuốt* không có gì giữ
AI trong đoạn đã chọn.

Trên WPS, **⋯ là một khối `hidden`** chứ không phải menu của hệ điều hành: bấm nút thì bật/tắt, bấm ra
ngoài hoặc `Esc` thì đóng (nghe ở `document.body` — pane không bao giờ tràn ra ngoài khung). Cố ý **không**
đặt `role="menu"`/`menuitem`: đã không làm được điều hướng bằng phím mũi tên thì đừng khai báo ARIA của
một menu thật. Cùng lý do, *Hội thoại trước…* là **một màn hình của pane** (danh sách → chi tiết → xoá có
xác nhận) chứ không phải hộp thoại riêng — giữ mọi việc của khung chat ở một chỗ. (WPS có mở được cửa sổ thứ hai
bằng `Application.ShowDialog` — dùng cho khung chat mini — nhưng không dùng cho màn hình này.)

### Rút gọn theo nền tảng

- **WPS Linux**: đúng **một** tab `axiomTab` trong `ribbon.xml`; các nhóm trên là nhóm của tab đó và
  **Khác ▸** là `<menu>` trong cùng tab. `ribbon.xml` **đã dựng đủ** bộ nút ở bảng trên (19/10/2026): nhóm
  *Trợ lý* (Ask AI), *Kiểm tra* (ba nút Soát, `getVisible` theo app), *Quản lý* (Bộ nhớ AI, Kỹ năng),
  *Hệ thống* (Thiết lập, Trạng thái, Mở nhật ký, Khác ▸ Kiểm tra mô hình + Sao chép API URL). **Bốn control
  của bản Windows cố ý KHÔNG vẽ** — *Nhìn tài liệu* (§5.1: add-in thiếu handler `app.screenshot`),
  *Cài đặt nâng cao…*, *Tắt Agent Core…*, và menu ngôn ngữ (WPS không có tệp `lang/*.json`) — lý do ghi
  ngay trong chú thích đầu `ribbon.xml`. Menu *Tìm web* cũng không vẽ: chọn nguồn tìm kiếm ghi vào
  `mcp.json`, mà kênh `/wps/v1/*` không có route ghi tệp đó.
  Icon: **nút ribbon dùng `imageMso`** — bộ icon có sẵn của chính WPS, giống cách bản Windows dùng bộ icon của
  Office (bảng tên ở §5.3, `test_pane.mjs` ghim). **Menu chuột phải** vẫn dùng `getImage="ribbon.OnGetImage"`:
  `main.js` trả đường dẫn tương đối `"images/<tag>.svg"`, hai tệp `askAi.svg` và `quickMenu.svg` (20×20, nét 1.5,
  màu `#0F6CBD`) nằm ở `src/AxiomOffice.WPS/images/`; spec khớp hai chiều control ↔ tệp ↔ `ICON_FILES` của `main.js`.
  `ribbon.xml` còn có `<contextMenus>` (`ContextMenuText` + `ContextMenuList`): **Sửa bằng AI ▸** 9 mục +
  **Hỏi AI về đoạn này**, cả hai mở khung chat mini `mini.html`. Header pane có menu **⋯** chứa năm lối vào
  *Trò chuyện mới / Hội thoại trước… / Ghi nhớ / Nhật ký / Thiết lập* (Windows và LibreOffice bố trí khác —
  xem bảng *Trong khung chat*).
- **LibreOffice**: **một** mục menu *Axiom Office* (`Addons.xcu`, `OfficeMenuBar`) + một deck sidebar. Menu
  đang phẳng 6 mục (askai / check / skills / audit / setup / settings); theo bảng trên thì chúng thành các dải
  phân cách và **Khác ▸** là menu con cùng cấp. Không dựng cửa sổ tiến trình riêng — dùng chung lối hiển thị
  của pane.
- **Windows**: một tab với các nhóm *Trợ lý / Kiểm tra / Quản lý / Hệ thống*, **Khác ▸** là `menu` trong tab
  (đã dựng, `Ribbon/RibbonUi.cs`), và menu **⋯** của pane cho *Hội thoại* / *Nhật ký*.

## 3. Vì sao các nút này là thật (đối chiếu mã)

- **"Nhìn tài liệu" là toggle thật, không phải toggle giả — nhưng chỉ ở nơi ghi được cấu hình.** Core đọc
  lại cấu hình **mỗi lượt** (`core-go/internal/agent/orchestrator.go:207` — *"doc lai moi luot qua cau
  hinh"*, `cmd/axiom-core/main.go:181` ghi `LoadConfig: config.Load, // cau hinh song`); `GET /v1/setup`
  trả `visualQaEnabled` để nút đọc **trạng thái thật** (`core-go/internal/api/setup.go:226`).
  Nơi ghi: **Windows** = HKCU (`src/AxiomOffice/Bridge/Config.cs:97`); **Linux** =
  `~/.config/axiom-office/config.json` — *cùng file Core đọc* (`core-go/internal/config/source_other.go`:
  "dung chung voi extension LibreOffice"), và extension LibreOffice ghi được file đó
  (`axiom/config.py:set_value`, đã dùng cho checkbox sẵn có). Add-in **WPS Linux thì không** (§5.1).
- **Ba lệnh "soát" không cần model.** Chúng chỉ đọc và tất định, nên nút gọi thẳng
  `CommandDispatcher.Execute(host, "wpp.checkLayout", …)` **ngay trong tiến trình** → kết quả tức thì,
  **0 token, không cần Core, không cần mạng**. Cờ `agent=true` trong `catalog/live-commands.json` chỉ
  nghĩa *"model thấy lệnh này trong tool office_action"* (`src/AxiomOffice/Bridge/CommandCatalog.cs:49`),
  **không** phải "chỉ agent được gọi".
  Phía Linux chúng đã có sẵn bằng Python thuần: `axiom/checks.py` (`writer_check_tables`, `range_report`,
  `layout_report`), có test riêng, và cửa chạy tại chỗ là
  `bridge.run_on_main(ctx, gate, kind, action, params)`.
- **Bộ nhớ / kỹ năng / hội thoại / audit đều có API đầy đủ**; LibreOffice đã gọi được ba nhóm đầu:
  `GET /v1/memory` + `DELETE /v1/memory/{id}` (dialog **Ghi nhớ**, mở từ liên kết ở header pane),
  `GET /v1/skills?app=` + `POST /v1/skills/reload` (menu **Kỹ năng…**), `GET /v1/audit` (menu **Nhật ký…**).
  Danh sách này **không còn là hàm chết**: `axiom/core.py` có `skill_listing` / `reload_skills` / `audit`
  (`skills()` cũ bị thay vì nó trả về ít hơn payload thật — xem chú thích trong `skill_listing`).
- **LibreOffice còn chưa UI nào gọi**: `GET /v1/conversations` (**Hội thoại…**), `PATCH /v1/memory/{id}` (sửa /
  ghim / đặt hạn), `POST /v1/memory/{id}/restore` + `GET /v1/memory/{id}/history` + `DELETE
  /v1/memory?scope&confirm` (khôi phục / lịch sử / xoá cứng). Nghĩa là dialog **Ghi nhớ** của LibreOffice mới
  chỉ **đọc, lọc, xoá mềm** — chưa đủ nghiệm thu của đợt 2, và khoảng trống đó nằm ở UI chứ không phải ở Core.
  Pane **WPS** đã gọi `conversations*`, `restore` và `DELETE ?scope&confirm`; cả hai bản Linux đều **chưa** gọi
  `PATCH /v1/memory/{id}` và `GET /v1/memory/{id}/history`.
  Trên **Windows** những đường đó đã có UI từ 13/10 (`Ai/MemoryForm.cs`, `Ai/AdminForms.cs`, `Ai/CoreAdmin.cs`).

## 4. Đợt triển khai

| Đợt | Nội dung | Nghiệm thu |
|---|---|---|
| **1** | Toggle **Nhìn tài liệu** + ba nút **Soát** (bố cục/bảng/vùng) | Bấm ra kết quả ngay; toggle đổi được và lượt sau Core hành xử khác; ba nút Soát không tốn token · **LibreOffice ✅ 13/10** (mục menu *Soát tài liệu…*, §8) · **Windows ✅ PowerPoint 13/10** (Word/Excel/WPS Windows chưa bấm, §8) · **WPS Linux 19/10**: ba nút Soát đã dựng và **đo trên Core thật** (§5.2), nhưng **chưa bấm trong WPS**; toggle *Nhìn tài liệu* **chưa** — add-in thiếu handler `app.screenshot` (§5.1) |
| **2** | **Bộ nhớ AI…** — form 3 vùng (danh sách / chi tiết / lịch sử); phân biệt rõ **xoá mềm ≠ khôi phục ≠ xoá cứng** | Sửa, ghim, đặt hạn, xoá mềm, khôi phục, xoá cứng đều chạy; lọc được theo phạm vi user/document · **LibreOffice: mới 1/3** — dialog *Ghi nhớ* có **đọc / lọc / xoá mềm / tải lại**; thiếu sửa, ghim, hạn, khôi phục, xoá cứng (§3) · **Windows: viết xong 13/10** (lọc phạm vi + *Xoá tạm* / *Xoá vĩnh viễn…* theo phạm vi; chưa bấm thử) · **WPS Linux 19/10: 3/6 + lọc** — màn hình *Bộ nhớ AI* có xoá tạm, khôi phục, xoá vĩnh viễn theo phạm vi, cùng đọc, lọc phạm vi, tải lại; **thiếu sửa, ghim, đặt hạn** (không vẽ nút khi chưa có đường) |
| **3** | **Kỹ năng…** (ribbon), **Nhật ký hoạt động…** + **Hội thoại…** (menu ⋯ của pane) — **không** mở tab thứ hai (§2) | Thấy đúng skill của app đang mở; nạp lại được; audit đọc được · **LibreOffice ✅ 13/10** cho *Kỹ năng…* + *Nhật ký hoạt động…* (§8) · **Hội thoại…**: chưa (route `GET /v1/conversations` có sẵn) · **Windows: viết xong 13/10**, gọi thử API thật OK, chưa bấm trong Office · **WPS Linux 19/10**: *Kỹ năng…* ✅ (lọc theo app, nguồn, lỗi nạp, *Nạp lại*) và *Nhật ký hoạt động…* ✅ (100 dòng mới nhất, giờ máy); *Hội thoại trước…* ✅ (menu ⋯, đợt E 19/10: danh sách → chi tiết → xoá có xác nhận) |
| **4** | ~~Nút theo từng app, xuất PDF, lối tắt soạn thảo~~ — **bỏ** 13/10: soát theo app đã làm ở đợt 1 (`getVisible`); xuất PDF người dùng không muốn trên ribbon; "lối tắt soạn thảo" chưa ai định nghĩa là nút nào (nguyên tắc 3) | — |

**Windows — đợt 2:** dùng lại `src/AxiomOffice/Ai/MemoryForm.cs` (đã có sửa, ghim, hạn dùng, khôi phục, lịch sử),
thêm lọc theo phạm vi và tách *Xoá tạm* (khôi phục được 30 ngày) khỏi *Xoá vĩnh viễn…*. Core **không có xoá cứng
từng mục** — chỉ `DELETE /v1/memory?scope=all|user|document&confirm=true` — nên *Xoá vĩnh viễn…* xoá theo phạm vi
đang lọc và nói rõ điều đó trong hộp xác nhận (không vẽ nút xoá cứng từng mục khi API không có).

Cổng kiểm mỗi đợt: `make lint`, `make check-gen`, `make test` (Linux) · `scripts\build.ps1` +
`scripts\package.ps1` và CI job `windows` (Windows).

## 5. Hoãn (có lý do, không phải bỏ quên)

### 5.1 Chặn thật: toggle "Nhìn tài liệu" trên WPS Linux

> **Đã gỡ 19/10/2026** (Đợt A): Core có `POST /wps/v1/config` ghi thẳng `~/.config/axiom-office/config.json`
> (`config.SetValue` + `JSONSource.Set`), và pane gọi được `GET /wps/v1/setup` để đọc trạng thái **thật**.
> Đoạn dưới giữ lại làm dấu vết: nó là lý do Đợt A phải làm trước khi vẽ nút.

Đã kiểm bằng danh sách route đầy đủ của Core: **không có endpoint nào ghi cấu hình** (không `POST
/v1/setup`, không `PATCH /v1/config`; chỉ có `POST /v1/llm/test` là ghi-gì-đó-và `POST /v1/skills/reload`).
Add-in WPS là JavaScript trong CEF nên cũng không ghi được `~/.config/axiom-office/config.json` (đã đo:
`fs.readFileString` từ chối mọi đường dẫn có dấu phân cách). Vậy muốn có toggle trên ribbon WPS thì phải
**thêm endpoint ghi cấu hình vào Core (Go) trước** — đó là việc của Core, không phải của UI, và phải làm
trước khi vẽ nút. Cho tới lúc đó, đường duy nhất trên Linux là **Cài đặt nâng cao của LibreOffice**.

Còn một đường chưa khảo sát: add-in WPS *chính là* bên thi hành lệnh bridge, nên nút ribbon có thể gọi
handler trong tiến trình (như `CommandDispatcher.Execute` ở Windows) — nhưng vòng poll của add-in là
**đơn luồng**, lệnh chậm sẽ chặn nó (đã gặp lỗi này ở live gate). Chưa kết luận được, cần khảo sát `main.js`.

**Chặn thứ hai, tìm ra 19/10/2026 — và lần này không nằm ở Core:** ghi được cấu hình vẫn **chưa đủ** để bật
toggle, vì bật lên thì mỗi lượt chạy sẽ hỏng ở chính bước "nhìn tài liệu". Core gửi lệnh `app.screenshot`
qua bridge, mà add-in WPS **không có handler cho lệnh đó**: `app.screenshot` có trong `CATALOG`
(`commands.js:45`) nhưng **không** có trong `HANDLERS` (`commands.js:3235` trở đi) — `execute()` sẽ trả
`{ok:false, error:"unknown action: app.screenshot"}`. Đối chiếu: bản Windows có
`CommandDispatcher.AppScreenshot`, bản LibreOffice có `general.py:app_screenshot`. Vậy muốn có toggle này
trên ribbon WPS thì phải **viết handler `app.screenshot` cho add-in WPS trước** (việc của add-in, không phải
của UI). Trong lúc đó, màn hình *Thiết lập* của pane **vẫn hiện** ô "Nhìn tài liệu" (vì cấu hình ghi được
thật, và Core đọc lại thật) nhưng kèm một dòng nói thẳng rằng bật ở đây chưa có tác dụng — thà nói ra còn
hơn để người dùng bật rồi mọi lượt chạy đều lỗi.

### 5.2 Ba đường mới phục vụ pane/ribbon WPS (đo 19/10/2026)

Ba đường này không nằm trong đợt A (đợt A chỉ mở lại các đường `/v1/*` sẵn có dưới tiền tố `/wps/v1/*`):

- **`GET /v1/log` + `/wps/v1/log`** — nút **Mở nhật ký**. Bản Windows mở tệp bằng trình soạn thảo của hệ
  điều hành (`Process.Start`); trong CEF **không mở được chương trình ngoài**, nên pane phải **đọc** nội
  dung rồi tự hiển thị. Trả về đúng tệp mà bản Windows mở, **chỉ phần cuối** và có trần
  (`lines` mặc định 200 / tối đa 2000, tối đa 512 KB từ cuối tệp; dòng dài bị cắt và `truncated` nói rõ) —
  `corelog` xoay tệp ở 10 MB nên đọc cả tệp vừa phí vừa có thể treo pane. Tệp **chưa có** trả
  `exists:false` (không phải lỗi): Core mới cài chưa ghi dòng nào là chuyện bình thường. Đây là đường
  `routeBoth` — `/v1/log` giữ guard token cho script/MCP.
- **`GET /wps/v1/status`** — nút **Trạng thái**. Pane **không tự hỏi được**: bridge nằm ở cổng riêng, mà
  `/health` của Core thì bị guard chung chặn khi request có `Origin`. Một lời gọi trả **cả hai tầng**
  (`{addin: Bridge.Status(), core: coreStatus}`), nên pane không phải đoán. Thêm `endpoint`/`model`/
  `configured` vào tầng Core để nút *Kiểm tra mô hình* biết đã chọn AI chưa (bản Windows đọc thẳng
  `Config.LlmEndpoint/LlmModel`; thiếu thì pane phải đoán, mà đoán sai thì nút báo "chưa cấu hình" ngay cả
  khi đã cấu hình).
- **`POST /wps/v1/exec`** — ba nút **Soát** và *Hoàn tác lượt này*: chạy thẳng một lệnh bridge, không qua
  agent, không tốn token. Hai ca từ chối trả **409 kèm mã máy đọc được** (`ErrorCode`):
  `addin-disconnected` (chưa mở tài liệu) và `run-in-progress` (đang có lượt AI chạy — chạy chen vào thì
  agent mất dấu trạng thái tài liệu nó đang theo dõi). Pane dịch **mã** sang câu tiếng Việt riêng
  (`pane.html: execError`), không hiện chuỗi tiếng Anh của Core và cũng không dò chuỗi (dò chuỗi là mong
  manh). Lệnh add-in **không có** trả HTTP 200 kèm `{ok:false, error:"unknown action: …"}` như
  `commands.js execute()` — nên pane **phải đọc `ok`**, đọc mã HTTP là hiện báo cáo rỗng như thể đã soát xong.

Số đo đầy đủ (hình dạng JSON, ngữ nghĩa `lines`/`truncated`, cái bẫy phong bì của bridge `/health`):
`tests/wps/REFERENCE.md`. Spec: `tests/wps/test_wps_pane_e2e.py` (18 phép kiểm mới) +
`internal/api/wpsadmin_test.go`, `internal/api/log_test.go`.

### 5.3 WPS trên máy thật: đã nghiệm thu gì, còn lại gì

- **`getImage` của WPS: trả data URI thì WPS vẽ ra dấu `?` — đã đo, và đã sửa.** Ribbon WPS gắn
  `getImage="ribbon.OnGetImage"` cho **mọi** control. Bản đầu trả **data URI SVG**; mở WPS lên thì chỗ
  icon toàn **dấu `?`** (đo trong WPS thật, 19/10/2026). Điều đó trả lời luôn câu hỏi cũ: WPS **có** gọi
  `getImage` — nó nhận chuỗi trả về, nhưng đem **phân giải như một đường dẫn**, trượt thì vẽ `?`; data URI
  không phải đường dẫn nên trượt.
  Cách đúng nằm ngay trong **SDK chính thức của WPS** đã cài trên máy này —
  `~/.local/lib/node_modules/wpsjs/packages/@wps-jsapi/wps/js/ribbon.js`:
  `function GetImage(control) { … return "images/1.svg" }`, tức **đường dẫn tệp tương đối so với gốc
  add-in**, và bộ SDK ship kèm đúng những tệp đó (`images/1.svg` — một thẻ `<svg>` **chỉ có `viewBox`**,
  không `width`/`height`, để WPS tự co giãn theo cỡ nút). Nay `main.js` trả `"images/<tag>.svg"` và các tệp
  nằm ở `src/AxiomOffice.WPS/images/`. Gốc add-in ở đây là `<core>/wps/addin/`
  (`publish.xml`: `jspluginonline url="http://127.0.0.1:47840/wps/addin/"`), nên đường dẫn đó thành URL
  thật, và `ServeAddin` phục vụ bằng `image/svg+xml` — `ContentTypeFor` **đã có sẵn** nhánh `.svg`.
  Đo trên máy: `curl` ba tệp bất kỳ qua Core đều `HTTP 200  image/svg+xml`. **Đã nghiệm thu trong WPS
  thật (19/10/2026)**: khởi động lại WPS, tab *Axiom Office* hiện đủ icon, không còn dấu `?`.
- **`imageMso` trên WPS Linux — đã nghiệm thu trên WPS thật (20/10/2026): hiện đúng icon.** Sau khi tự vẽ 12 icon
  SVG, ribbon WPS trông nhạt so với bộ icon của Office trên bản Windows. Thư viện ribbon của WPS (`libkshell.so`)
  có nhận thuộc tính `imageMso`, và WPS có **bảng lệnh riêng** ánh xạ tên kiểu Office sang icon của nó:
  `/opt/kingsoft/wps-office/office6/res/commands/<wps|et|wpp>/ongmani/*.kuip` — mỗi lệnh có `idMso="…"` và
  `icon="…_kd"` (vd `idMso="SmartArtInsert"` → `smart_diagram_kd`). Bảng chia **theo từng app**, mà một ribbon
  dùng chung cho cả ba, nên: nút dùng chung phải là tên có ở **cả ba** bảng; ba nút *Soát* (mỗi nút một app) dùng
  tên của đúng app đó. Năm tên bản Windows dùng **không có** trong WPS nên được thay:

  | Nút | `imageMso` | Có ở | Ghi chú |
  |---|---|---|---|
  | Ask AI | `Help` | cả ba | như Windows |
  | Soát bố cục | `SlideShowFromCurrent` | trình chiếu | như Windows |
  | Soát bảng | `TableInsertDialogWord` | văn bản | Windows dùng `TableInsert` — Writer của WPS không có |
  | Soát vùng dữ liệu | `DataValidation` | bảng tính | như Windows |
  | Bộ nhớ AI… | `FileSave` | cả ba | thay `RecordsSaveRecord` |
  | Kỹ năng… | `JsAddInsDialog` | cả ba | thay `SmartArtInsert` (chỉ có ở văn bản) |
  | Thiết lập… | `ApplicationOptionsDialog` | cả ba | thay `ControlWizards` |
  | Trạng thái | `About` | cả ba | thay `Info` |
  | Mở nhật ký | `FileOpen` | cả ba | như Windows |
  | Khác ▸ | `FileProperties` | cả ba | thay `PropertySheet` |
  | Kiểm tra mô hình | `ReviewNewComment` | cả ba | thay `AcceptInvitation` |
  | Sao chép API URL | `Copy` | cả ba | như Windows |

  Đổi tên nào thì phải tra lại bảng lệnh của WPS và thử trên WPS thật — tên không có thì nút không có icon. Người
  dùng xác nhận "hiện icon" (20/10/2026) nhưng chưa nhìn riêng từng app — ba nút *Soát* dùng tên chỉ có ở một app,
  nên trong app khác (nếu WPS bỏ qua `getVisible`) chúng có thể không có icon.
- **Màu icon menu chuột phải**: hằng số `#0F6CBD` nằm trong **từng tệp** `images/<tag>.svg` (không đọc từ
  `theme.py` — ảnh do WPS vẽ, nó không biết `var(--axiom-*)`); đổi bảng màu thì sửa cả hai tệp, `test_pane.mjs`
  kiểm màu từng tệp. Icon ribbon (`imageMso`) là của WPS, theo giao diện WPS.
- **Menu chuột phải — đã nghiệm thu trên WPS thật (20/10/2026)**: WPS Linux nhận `<contextMenus>` (ghi chú
  "WPS không nhận chắc" trong `RibbonUi.cs` của bản Windows không đúng với WPS Linux). Bấm *Sửa lỗi chính tả*
  chạy trọn tới Core (§8). Lần đầu model **hỏi lại** thay vì sửa — đã thêm `QuickEditRule` (§8, đợt I).
- **Khung chat mini — người dùng thử trên WPS thật (20/10/2026)**: đánh giá "khá ổn". Chưa đo riêng vị trí
  cửa sổ (`ShowDialog` không nhận toạ độ) và trường hợp gõ thêm sau khi vùng chọn co lại.
- **Bấm từng nút ribbon trong WPS thật**: các nút *Soát*, *Bộ nhớ AI*, *Trạng thái*… mới đo trên Core thật +
  add-in giả qua HTTP; chưa bấm từng nút trên ribbon WPS thật rồi xem kết quả — cần một lượt chạy tay.
- **`app.screenshot` cho add-in WPS** — điều kiện để toggle *Nhìn tài liệu* lên ribbon (§5.1).
- **Ràng buộc của khung chat**: `main.js` mở pane bằng `Application.CreateTaskPane(url)`. Cửa sổ task pane
  **không có kênh trực tiếp nào từ ribbon xuống trang** (từ 20/10 có đường vòng qua Core — §7 — nhưng chỉ chở
  mục *Sửa bằng AI* khi không mở được khung chat mini; chuyển màn hình vẫn đi bằng `?view=`) — bấm *Trạng thái* / *Mở nhật ký* trên ribbon mà pane đang mở
  thì pane **không tự biết** để chuyển màn hình (xem §7). Vì vậy việc chuyển màn hình đi bằng **query của
  URL** (`?view=status`, `?view=log`, `?do=testModel`, `?check=writer.checkTables`): ribbon mở **một pane
  cho mỗi màn hình** (khoá = tên màn hình, khung chat dùng lại pane cũ để không mất đoạn hội thoại đang dở;
  nút có `?do=` thì **không** dùng lại — bấm *Kiểm tra mô hình* lần hai phải chạy lại thật). Giá phải trả:
  dữ liệu của một màn hình đã mở có thể **cũ**, và cách duy nhất để làm mới là nút *Tải lại* trong chính màn
  hình đó — không có cách nào từ ribbon đẩy lệnh vào trang đang mở.

### 5.4 Hoãn khác (có lý do)


- **Chọn model bằng dropDown**: làm được (HKCU + cấu hình sống) nhưng đổi model giữa lượt dễ gây
  "lúc chạy lúc không"; để trong **Cấu hình nâng cao**.
- **Chẩn đoán (`axiom-core doctor`)**: là subcommand của binary, add-in phải spawn tiến trình riêng;
  nút **Trạng thái** + **Mở nhật ký** đã phủ phần lớn sự cố.
- **Ép chọn kỹ năng thủ công**: Core tự chọn skill; muốn người dùng ép thì phải bổ sung cơ chế gửi lựa
  chọn xuống Core + xác thực + quy tắc ưu tiên. **Không giả lập bằng UI.**
- **Nút "Xác nhận" / "Từ chối" trên ribbon**: luồng xác nhận đã ở pane; đặt hai nơi dễ lệch trạng thái.

## 6. Rủi ro & cách chống

| Rủi ro | Cách chống |
|---|---|
| Ribbon thành danh mục API (80 lệnh trong `catalog/live-commands.json`) | Chỉ đưa nút có workflow hoàn chỉnh; còn lại qua pane. Tên nhãn theo kết quả, không theo lệnh |
| Một tab duy nhất bị nhồi nút | Thứ ít dùng nằm trong **Khác ▸** hoặc menu **⋯** của pane (vẫn cùng một tab) — thêm nút mới thì phải bỏ một nút ra khỏi hàng, không nới hàng |
| Người dùng không biết AI đang làm gì | Pane là nguồn duy nhất hiển thị tiến trình (công cụ, skill, ghi nhớ, xác nhận, kết quả) trên cả ba nền tảng |
| UI hiển thị một trạng thái, Core làm trạng thái khác | Toggle đọc trạng thái **thật** (`GET /v1/setup`), ghi thành công rồi mới đổi hình; không suy ra "Core khỏe" từ việc mở được cổng TCP |

## 7. Ràng buộc kỹ thuật

- **File sinh — sửa ở nguồn, không sửa bản sinh**: `catalog/setup.json` → `src/AxiomOffice/Setup/SetupCatalog.cs`,
  `src/AxiomOffice.LibreOffice/python/pythonpath/axiom/setup_catalog.py`,
  `core-go/internal/setup/catalog_gen.go`; registry Python → `src/AxiomOffice.WPS/commands.js`;
  `catalog/live-commands.json` → `core-go/internal/mcpserver/livecommands_gen.go`. `make check-gen` sẽ đỏ nếu lệch.
- **Máy Linux không biên dịch được C#** (`csc`/`mono`/`dotnet` đều không có) → mọi thay đổi `src/AxiomOffice/**`
  chỉ được kiểm bằng CI job `windows` (push) và bằng bấm tay trên máy Windows.
- **WPS Linux**: `tests/wps/test_pane.mjs` ghim nội dung `ribbon.xml` (ca "ribbon co nut Ask AI…" và khối icon /
  menu chuột phải gần cuối tệp) — thêm nút hay mục menu thì phải mở rộng spec.
- **WPS Linux — ribbon KHÔNG nói THẲNG được vào trang pane đang mở** (từ 20/10/2026 có một đường vòng qua Core,
  xem cuối mục này). `Application.CreateTaskPane(url)` chỉ *mở*
  một cửa sổ CEF mới; giữa add-in và trang đó **không có kênh nào** (không `postMessage`, không API của
  task pane để đẩy dữ liệu vào — `Application.GetTaskPane` chỉ trả object pane, không phải `window` của
  trang). Hệ quả: mọi việc "mở màn hình X" phải đi qua **query của URL lúc nạp trang**, và dữ liệu của một
  màn hình đã mở chỉ làm mới được bằng nút *Tải lại* trong chính màn hình đó. Đừng vẽ nút nào cần "pane
  đang mở tự đổi theo" — không có đường thật đứng sau. (`main.js` ghi cùng ràng buộc này.)
  **Đường vòng qua Core (20/10/2026)**: cả add-in lẫn khung chat đều nói được với Core, nên Core làm trạm trung
  chuyển — `POST /wps/v1/pane/request` → SSE `GET /wps/v1/pane/events` (`api/wpspane.go`). Từ đợt I đây là đường
  **dự phòng**, chỉ dùng khi không mở được khung chat mini (`openMini` trả false). Hiện chỉ chở **một**
  loại việc: một mục *Sửa bằng AI* chọn từ menu chuột phải. Muốn chở thêm loại khác (vd chuyển màn hình) thì thêm
  sự kiện vào đúng kênh này, đừng mở kênh thứ hai.
- **WPS Linux — add-in không ghi được tệp ra ngoài thư mục add-in**: `fs.readFileString` từ chối mọi đường
  dẫn có dấu phân cách. Nên mọi thứ cần ghi (cấu hình, phiên) phải đi qua Core; add-in chỉ *chạy lệnh*.
- **LibreOffice**: `ACTIONS` trong `axiom/dispatch.py` là **danh sách trắng** — thêm mục menu vào
  `Addons.xcu` mà quên thêm tên vào đó thì bấm menu **không làm gì cả, im lặng**. Đã có spec chặn:
  `tests/lo/test_dispatch.py` đối chiếu hai bên.
- Giữ vững hợp đồng parity của các lệnh bridge (`catalog/live-commands.json`; xem nguyên tắc 5).

## 8. Nhật ký triển khai

| Ngày | Nền tảng | Việc |
|---|---|---|
| 13/10/2026 | LibreOffice | Mục menu **Soát tài liệu…** (`Addons.xcu` + `dispatch.py` action `check` + `dialogs.open_check`): chạy trong tiến trình qua `bridge.run_on_main`, tự nhận loại tài liệu đang mở, liệt kê vấn đề bằng nhãn tiếng Việt (`checks.ISSUE_LABELS`) kèm chi tiết gốc. Chỉ đọc, 0 token. Spec mới: `tests/lo/test_dispatch.py` (9), `tests/lo/test_report.py` (8), `tests/lo/test_dialogs.py` (6). |
| 13/10/2026 | LibreOffice | **Nút trong cả năm dialog là nút chết — đã tìm ra và sửa.** `awt.bind_actions` gắn **một** `ActionListener` chung rồi đọc `event.ActionCommand` để biết nút nào vừa bấm; đo trong LibreOffice thật (dialog *Kỹ năng*, cắm log tạm) thì `event.ActionCommand` đến tay người nghe là **chuỗi rỗng**: listener có chạy nhưng `command == "reload"` / `== "close"` đều sai, nên bấm "Nạp lại" không nạp, bấm "Đóng" không đóng. Sửa gốc: mỗi nút một listener **mang sẵn tên của nó** (`awt.ActionListener(handler, name)`), vẫn đặt `ActionCommand` lên model cho bản khác. Spec mới: `BindActionsTest` (`tests/lo/test_dialogs.py`). Kiểm lại bằng LibreOffice thật: `tests/live/open_dialogs.py skills --click-window "Kỹ năng:72,414" --click-window "Kỹ năng:503,414"` → trạng thái đổi thành *"Đã nạp lại · 3 kỹ năng cho Writer"* rồi cửa sổ **mất hẳn** khỏi X (`xwininfo`: không còn cửa sổ nào), và dialog *Soát tài liệu* cũng đóng được. |
| 13/10/2026 | (kế hoạch) | **Chốt: chỉ một tab "Axiom Office"** (người dùng yêu cầu). Bỏ "Tab 2 — Axiom · Quản lý" khỏi §2; các mục của nó rút vào nhóm *Quản lý* / *Hệ thống*, menu **Khác ▸** và menu **⋯** của pane — vẫn một tab. Kiểm lại mã: WPS đã đúng một tab (`ribbon.xml`, `axiomTab`) và LibreOffice đã đúng một mục menu (`Addons.xcu`, `OfficeMenuBar`) — không phải sửa mã, chỉ sửa plan cho khỏi phình ra tab thứ hai về sau. Cùng ngày, phiên trên máy Windows **đã dựng** đúng một tab theo quyết định này (xem dòng Windows COM bên dưới). |
| 13/10/2026 | (kế hoạch) | Chốt plan; phát hiện §5.1 (toggle trên WPS Linux bị chặn vì Core không có endpoint ghi cấu hình). Giao phần Windows cho phiên trên máy Windows. |
| 13/10/2026 | LibreOffice | Hai mục menu **Kỹ năng…** (`org.axiomoffice.bridge:skills`) và **Nhật ký hoạt động…** (`:audit`): `Addons.xcu` → `dispatch.ACTIONS` → `dialogs.open_skills` / `open_audit`. `core.py` thêm `skill_listing` (trả **cả payload**, không chỉ `skills`, để thư mục kỹ năng nạp lỗi không bị im lặng bỏ qua), `reload_skills`, `audit`. Nút *Nạp lại* gọi `POST /v1/skills/reload` (đọc lại thư mục, **không** khởi động lại Core); nhật ký đọc 100 dòng mới nhất và đổi giờ UTC của Core sang **giờ máy**. Spec mới: `SkillLinesTest`, `SkillStatusTest`, `AuditLinesTest`, `ClockTest` (`tests/lo/test_dialogs.py`), `CoreListingTests` (`tests/lo/test_chat.py`), và một ca dispatch cho hai URL. |
| 13/10/2026 | LibreOffice | `tests/live/open_dialogs.py` — probe **không** phải unittest: dựng LibreOffice thật trong profile/XDG tạm, cài `.oxt`, gửi **đúng URL của menu** qua UNO rồi **chụp ảnh màn hình** (`xwd` + `ffmpeg`). Cần vì dialog ở đây là `awt.ChildWindow` — cửa sổ con **không có cửa sổ X riêng**, nên `xdotool search --name` không tìm được như với wizard. Đã chạy và xem ảnh: *Soát tài liệu* (bảng dựng thật bằng `writer.insertTable` → *"Có ô trống"*), *Kỹ năng* (3 kỹ năng, "nguồn: sẵn có"), *Nhật ký* (nhánh rỗng). Ảnh ở `/tmp/axiom-dialogs`. |
| 13/10/2026 | Windows COM | Ribbon tab **Axiom Office** sắp lại thành *Trợ lý / Kiểm tra / Truy cập nhanh* (`Ribbon/RibbonUi.cs`). **Nhìn tài liệu** = `toggleButton`: ghi HKCU `VisualQaEnabled`, ghi hỏng thì vẽ lại theo HKCU; luồng nền hỏi `GET /v1/setup` lấy trạng thái thật của Core (biến môi trường `AXIOM_VISUAL_QA` đè được HKCU → nút theo Core + báo); mờ khi `CoreEnabled=0`. Ba nút **Soát** hiện theo app (`getVisible`), gọi `CommandDispatcher.Execute` ở **luồng nền**, kết quả đổi sang tiếng Việt (`Ribbon/RibbonChecks.cs`, tối đa 15 dòng). Đã bấm thật trên **PowerPoint**: toggle ghi HKCU và Core báo `visualQaEnabled=True` khớp; *Soát bố cục* trên bài 3 slide ra kết quả ~0,5s, 0 lượt gọi model. Kèm sửa `HostProbe` (cửa sổ PowerPoint không có `Visible` làm mất cả tên cửa sổ trong log). Chưa bấm: Word, Excel, WPS Windows. |
| 13/10/2026 | Windows COM | Theo quyết định **một tab** (§2): thêm nhóm **Quản lý** (*Bộ nhớ AI…* = `MemoryForm` có lọc phạm vi + xoá tạm/vĩnh viễn; *Kỹ năng…* = `SkillsForm`, `Ai/AdminForms.cs`) và nhóm **Hệ thống** (*Trạng thái* kiểm thật bridge/Core/phiên ở luồng nền; menu *Khác*: Cài đặt nâng cao, Kiểm tra mô hình, Sao chép API URL, Tắt Agent Core có xác nhận). *Hội thoại trước…* / *Nhật ký hoạt động…* vào menu **⋯** ở header pane (`ConversationsForm`, `AuditForm`). Client mới ở `Ai/CoreAdmin.cs`. Đã gọi thử mọi API đọc trên Core thật (3 kỹ năng wpp, audit, 3 hội thoại, lọc ghi nhớ user/document); **chưa bấm trong Office**. |
| 13/10/2026 | Windows COM + Core | Bấm thật *Nhìn tài liệu* trên PowerPoint thì thấy ảnh model nhận là **cả cửa sổ** (slide ~30% ảnh) — lệch hợp đồng `app.screenshot` ("trang/slide hiện tại"). Sửa: trình chiếu xuất thẳng slide đang mở (`Slide.Export`, kèm `slide`/`slideCount`), `look_at_document` nói với model đó là slide số mấy. Kèm sửa pane ẩn các bước ngoài `office_action` (trông như đơ) và vẽ tích xanh cho bước lỗi. Chi tiết: CHANGELOG. |
| 13/10/2026 | Core + Windows | **Plan mode** (`options.mode="plan"`: chỉ đọc, trả kế hoạch; pane có công tắc + thẻ *Thực hiện kế hoạch*), **đính kèm tệp** .docx/.xlsx/.pptx (pane gửi đường dẫn đầy đủ; Core đọc bằng tool MCP đọc tệp) và **tìm kiếm web Tavily** (tool `tavily_search` của Core, khoá trong wizard bước 4). **Việc của phiên Linux**: pane LibreOffice/WPS thêm công tắc lập kế hoạch (gửi `options.mode`), nút đính kèm, và ô khoá Tavily ghi `TavilyApiKey` vào `~/.config/axiom-office/config.json` — Core đã sẵn sàng. Sub-agent: tạm hoãn theo yêu cầu người dùng. |
| 19/10/2026 | Core | **Đợt A — mở đường cho pane/ribbon WPS** (gỡ §5.1 và mọi nút "đọc/ghi qua Core" của pane). (1) **Ghi cấu hình**: `config.SetValue` + `WritableSource`, `JSONSource.Set` (Linux, `~/.config/axiom-office/config.json`, ghi 0600, **giữ nguyên khoá khác** vì file dùng chung với extension LibreOffice, ghi kiểu JSON thật như `config.py:set_value`) và `RegistrySource.Set` (Windows, REG_SZ như add-in .NET). (2) **Kênh `/wps/v1/*`**: `routeBoth` đăng ký thêm bản `/wps/v1/*` (xác thực kênh add-in) cho `setup`, `llm/test`, `llm/models`, `skills`, `memory*`, `audit`, `conversations*` — pane là trang của Core nhưng **không có token** nên trước đây mọi đường `/v1/*` đều 401. `/v1/*` giữ nguyên guard cũ (mọi `Origin` → 403). (3) Hai đường mới: `POST /wps/v1/config {Khoá: giá trị}` (danh sách trắng, từ chối `Token`/`CorePort`, trả `saved` + `ignored` khi bị `AXIOM_*` đè + `current`) và `POST /wps/v1/exec {action, params}` (chạy thẳng một lệnh bridge, không qua agent, không tốn token — cho nút **Soát** và *Hoàn tác lượt này*; **409 khi đang có lượt chạy** để agent không mất dấu tài liệu). Spec: `internal/api/wpsadmin_test.go` (4 ca) + `tests/wps/test_wps_pane_e2e.py` (+11 ca, tổng 44). |
| 19/10/2026 | WPS Linux | **Đợt B — khung chat WPS đủ màn hình.** Pane từ 1 màn hình (chat) thành **7**: chat, *Bộ nhớ AI*, *Nhật ký hoạt động*, *Thiết lập*, *Kỹ năng*, *Trạng thái*, *Mở nhật ký* — cộng màn hình *Soát tài liệu* (gọi từ ba nút Soát). Ba màn hình quản trị hay dùng có lối vào **ngay trong pane** (`Ghi nhớ` / `Nhật ký` / `Thiết lập` ở header, đã có từ trước); bốn màn hình còn lại mở từ ribbon qua `?view=` (§5.3). Báo cáo soát **pane tự render** bằng tiếng Việt: Core/Go không chạy được `checks.py`, nên pane giữ bảng nhãn `ISSUE_LABELS` + luật dựng dòng y hệt `checks.py` của LibreOffice, và có **spec chặn lệch**: `test_pane_view.mjs` đọc `ISSUE_LABELS` từ **cả hai** tệp rồi so từng dòng hai chiều. Sửa một lỗi thật: `POST /v1/llm/test` đọc khoá **chữ thường** (`providerId`/`endpoint`/`model`/`apiKey`) còn pane gửi chữ hoa — nút *Kiểm tra mô hình* chưa bao giờ chạy đúng. **Chưa làm**: *Hội thoại trước…* (§2 xếp nó vào menu **⋯** của pane; pane WPS **không có** menu ⋯ và không có màn hình này — route `GET /wps/v1/conversations` đã sẵn, chỉ thiếu UI). Spec: `tests/wps/test_pane_view.mjs` (28 ca), `pane_dom.mjs` (DOM giả, không cần trình duyệt). |
| 19/10/2026 | WPS Linux | **Đợt C — ribbon WPS đủ bộ nút** (§2). `ribbon.xml`: nhóm *Trợ lý / Kiểm tra / Quản lý / Hệ thống* + **Khác ▸**, cùng nhãn và cùng nhóm với bản Windows. Ba nút Soát mang `?check=<lệnh>` (mỗi nút một lệnh, mỗi lệnh một app); *Trạng thái* / *Mở nhật ký* mang `?view=`; *Kiểm tra mô hình* / *Sao chép API URL* mang `?do=`. **Bốn control của bản Windows không vẽ** vì không có đường thật đứng sau — *Nhìn tài liệu* (thiếu handler `app.screenshot`, §5.1), *Cài đặt nâng cao…*, *Tắt Agent Core…*, menu ngôn ngữ — và menu *Tìm web* (không có route ghi `mcp.json`). Ô "Nhìn tài liệu" trong màn hình *Thiết lập* của pane **vẫn hiện** nhưng kèm câu nói thẳng rằng bật ở đây chưa có tác dụng, thay vì hứa suông. Supertip *Sao chép API URL* nói đúng thứ nó sao chép (`/health` của bridge, không phải API của Core). |
| 19/10/2026 | WPS Linux + Core | **Đợt D — đo sống trên Core thật** (`make test-wps-e2e`, không cần WPS). Thêm **18 phép kiểm** vào `tests/wps/test_wps_pane_e2e.py` cho đúng ba đường mà nút gọi: `GET /wps/v1/status` (cả hai tầng, `configured`, `logFile`, `Origin` lạ → 403), `GET /wps/v1/log` (đúng tệp Core đang ghi; `?lines=2` → hai dòng cuối + `truncated`; tệp chưa có → 200 `exists:false`; `/v1/log` vẫn đòi token) và `POST /wps/v1/exec` (báo cáo soát đi qua nguyên vẹn; lệnh add-in không có → 200 + `ok:false`; hai mã 409 `addin-disconnected` / `run-in-progress`). Phép đo còn **kiểm chứng địa chỉ mà nút *Sao chép API URL* sao chép**: dán `http://127.0.0.1:<bridgePort>/health` vào trình duyệt (không Origin, không token) → 200 `connected:true`. Kết quả: pane e2e **68 pass**, bridge e2e 36 pass. Số đo + một cái bẫy hình dạng JSON ghi ở `tests/wps/REFERENCE.md`. **Chưa** bấm nút trong WPS thật (§5.3). |
| 19/10/2026 | Cả ba nền tảng | **Đợt E — làm pane và ribbon giống MS Word.** (1) **Bảng màu**: `theme.py` đổi từ indigo sang **xanh Fluent `#0F6CBD`** (màu Copilot/Word), kéo theo `PaneControls.cs` và `pane.html` — ba bản phải khớp nhau và `test_pane_theme.mjs` (9 phép kiểm) là thứ ràng buộc điều đó: mọi màu trong `theme.py` phải có token `--axiom-*` dùng thật, và **không** được có mã `#RRGGBB` nào ngoài khối `:root`. (2) **Pane**: trả lời của AI **bỏ bong bóng** — chữ nằm thẳng trên nền pane như Copilot, chỉ tin nhắn người dùng mới có nền; header thành **tiêu đề + nút ⋯** với menu *Trò chuyện mới / Hội thoại trước… / Ghi nhớ / Nhật ký / Thiết lập*; thêm màn hình **Hội thoại trước…** (danh sách → chi tiết có `messages[]` → xoá có xác nhận), mở đúng route `conversations*` đã có sẵn trên kênh `/wps/v1/*` từ đợt A. (3) **Ribbon WPS**: mọi control có **icon** qua `getImage` (trước đó ribbon chỉ có chữ) — 12 icon, `viewBox 0 0 20 20`, nét 1.5. Spec mới: `test_pane.mjs` +2 ca ghim **hai chiều** `ribbon.xml` ↔ icon (thêm nút quên icon, hoặc icon mồ côi, đều đỏ), `test_pane_view.mjs` +7 ca cho menu ⋯ và màn hình hội thoại. Tổng: **160 pass** ở `make test-wps`, eslint sạch. Cách trả ảnh của đợt này (data URI SVG) **sai** — xem đợt F. |
| 19/10/2026 | WPS Linux | **Đợt F — sửa icon ribbon: data URI → đường dẫn tệp.** Mở WPS thật sau đợt E thì chỗ icon **toàn dấu `?`**. Điều đó trả lời luôn câu hỏi còn treo ở đợt E: WPS **có** gọi `getImage`, nhưng đem chuỗi trả về **phân giải như một đường dẫn** — data URI không phải đường dẫn nên trượt, và nó vẽ `?` thay vì bỏ trống. Cách đúng nằm sẵn trong **SDK chính thức của WPS** đã cài trên máy (`@wps-jsapi/wps/js/ribbon.js`: `return "images/1.svg"`), và bộ SDK ship kèm đúng những tệp đó — `<svg>` **chỉ có `viewBox`**, không `width`/`height`, để WPS tự co giãn theo cỡ nút. Nay: 12 tệp `src/AxiomOffice.WPS/images/<tag>.svg` (tên tệp **chính là** `tag`, nên không còn bảng tra nào phải giữ cho khớp), `main.js` trả `"images/<tag>.svg"`, xoá hẳn `ICON_BODY` / `svgIcon` / `ICON_COLOR` khỏi JS. **Không phải sửa Core**: `ServeAddin` vốn phục vụ mọi tệp dưới gốc add-in và `ContentTypeFor` **đã có sẵn** nhánh `.svg` → `image/svg+xml` (đo: `curl` ba tệp bất kỳ qua Core đều `200  image/svg+xml`). Spec `test_pane.mjs` đổi sang ghim `ribbon.xml` ↔ **tệp** trong `images/`, và kiểm **từng tệp**: một thẻ `<svg>` khớp kín, `viewBox 0 0 20 20`, **không** `width`/`height` ở thẻ mở, đúng màu accent `#0F6CBD`. **Đã nghiệm thu trong WPS thật**: khởi động lại WPS, icon hiện đủ, hết dấu `?` (§5.3). |
| 19/10/2026 | WPS Linux + Core | **Đợt G — menu *Sửa bằng AI* cho đoạn bôi đen** (đối chiếu WPS AI cài sẵn trên máy: phần dùng nhiều nhất của nó là sửa vùng chọn — trau chuốt, viết lại, rút gọn, đổi giọng, tóm tắt, dịch). **Core**: `agent/quickactions.go` giữ 9 mục (6 sửa, 3 chỉ trả lời) cùng câu lệnh tiếng Việt — câu lệnh được lưu nguyên văn làm tin nhắn của người dùng, và mỗi câu dặn **giữ ngôn ngữ của đoạn**; `POST /v1/runs {quickAction}` lấy câu lệnh theo id, **bỏ qua** `prompt` bên gọi gửi kèm và **không** đi cùng lập kế hoạch; `GET /v1/quick-actions` (`routeBoth`). Mục chỉ trả lời dùng bộ tool chỉ-đọc (`Request.ReadOnly`) nhưng vẫn là lượt thường (`mode` = `act`, không có thẻ kế hoạch). Không có đoạn bôi đen → dừng **trước khi tạo hội thoại và gọi model**: kind `selection`, hoặc `selection-lock` khi có bôi đen mà add-in từ chối khoá. **Pane WPS**: nút *✦ Sửa bằng AI* ở chân ô soạn, menu bật lên phía trên, vạch ngăn giữa hai nhóm; Core không trả mục nào (Calc/Impress, Core cũ, lỗi) thì **ẩn nút**. Spec: Go `quickactions_test.go` (2 ca), e2e +16 phép kiểm (84), `test_pane.mjs` +3, `test_pane_view.mjs` +5 (168). Đã **thử phá có chủ đích** — tắt chặn chỉ-đọc, tắt kiểm bôi đen, bỏ ẩn nút, bỏ dịch mã lỗi, bỏ vạch ngăn — lần nào cũng có ca đỏ; một ca e2e xanh nhầm (model giả lặp câu trả lời cuối) đã đổi sang đếm số tin nhắn. **Chưa** thử với model thật trong WPS thật. |
| 20/10/2026 | WPS Linux + Core | **Đợt H — chuột phải ra thẳng *Sửa bằng AI*.** `ribbon.xml` thêm `<contextMenus>` (`ContextMenuText`, `ContextMenuList`): **Sửa bằng AI ▸** 9 mục + **Hỏi AI về đoạn này**. Ribbon không nói thẳng được vào khung chat đang mở (§7), nên Core làm **trạm trung chuyển** (`api/wpspane.go`): `main.js` hiện khung chat (dùng lại cái đang mở, giữ hội thoại) rồi `POST /wps/v1/pane/request`; khung chat Writer nghe SSE `/wps/v1/pane/events` và chạy mục đó như bấm trong menu ở ô soạn. Một yêu cầu giao cho **một** khung chat, giữ 20 giây cho khung chat vừa tạo kịp nối vào, bấm liền hai mục thì mục sau thắng; đang có lượt chạy thì khung chat nói rõ đã bỏ qua. Danh sách mục trong `ribbon.xml` sinh từ `agent/quickactions.go` và **spec ghim hai bên** (thêm mục ở Core mà quên menu chuột phải là đỏ). Icon mới `images/quickMenu.svg` (bút + tia sáng; bản đầu tia sáng chạm đầu bút, ở 16 px dính thành một cục — đã dời sang góc trên trái). Spec: Go `wpspane_test.go` (chạy cả `-race`), e2e +6 (90), `make test-wps` +5 (173); thử phá có chủ đích ba chỗ đều đỏ. **Chưa nghiệm thu trong WPS thật**: thư viện ribbon của WPS có đủ tên thẻ `contextMenus`/`contextMenu`/`menuSeparator`/`insertBeforeMso`, nhưng bản Windows từng ghi "WPS không nhận chắc" — phải mở WPS mới biết. |
| 20/10/2026 | WPS Linux | **Sửa chân ô soạn** (ảnh chụp của người dùng): dòng gợi ý *Enter để gửi · Shift+Enter xuống dòng* gãy hai dòng và dính vào chữ *✦ Sửa bằng AI*. Gốc: `updateHint()` chọn câu theo **ngưỡng cố định 340 px**, đặt từ trước khi chân ô soạn có thêm nút *Sửa bằng AI*. Nay đo **chỗ trống thật** (`scrollWidth` so với `clientWidth`): đủ chỗ thì câu đầy đủ, chật thì *Enter để gửi*, vẫn chật thì bỏ hẳn; gợi ý **không bao giờ xuống dòng** (`white-space: nowrap`), có lề hai bên; câu đầy đủ vẫn ở **tooltip nút Gửi**. Chưa đo được layout thì đoán theo bề rộng, có tính nút *Sửa bằng AI*. Spec `test_pane_view.mjs` +2 (175), thử phá hai chỗ đều đỏ. |
| 20/10/2026 | WPS Linux | **Bỏ menu *Sửa bằng AI* khỏi khung chat** (yêu cầu người dùng: chỉ để ở menu chuột phải). Xoá nút *✦ Sửa bằng AI*, menu bật lên và CSS của chúng khỏi `pane.html`; **giữ** việc khung chat nghe `/wps/v1/pane/events` và hỏi `GET /quick-actions` (để bong bóng hiện nhãn mục). Core không đổi. Spec: bỏ 5 ca của menu trong khung chat, thêm ca ghim **không** có nút đó, chuyển ca "chưa bôi đen → thẻ lỗi tiếng Việt" sang đường chuột phải (172). **Lưu ý**: nếu WPS không hiện menu chuột phải (chưa nghiệm thu, đợt H) thì *Sửa bằng AI* không còn lối vào nào. |
| 20/10/2026 | WPS Linux + Core | **Nghiệm thu menu chuột phải trên WPS thật: menu HIỆN** (WPS Linux nhận `contextMenus` — ghi chú "không nhận chắc" của bản Windows là không đúng với WPS Linux). Chuỗi chạy trọn (đọc `core.db`): chuột phải → *Sửa lỗi chính tả* → Core khoá 120 ký tự bôi đen → model trả lời sau 27 s. Nhưng **0 thao tác**: model tìm ra 2 lỗi rồi **hỏi** "Bạn muốn tôi áp dụng luôn 2 sửa này…?" — người dùng bấm lệnh mà không thấy gì đổi. Câu lệnh trong tin nhắn đã ghi "sửa thẳng vào tài liệu" mà không đủ (lượt đó là tin thứ 15 của một hội thoại dài). Sửa: system prompt nối thêm `QuickEditRule` (bấm lệnh là đã quyết — sửa ngay, không xin phép, hoàn tác một cú bấm; lỗi nhỏ dùng `replaceAll` để giữ định dạng) hoặc `QuickAnswerRule` cho mục chỉ trả lời; e2e ghim cả ba trường hợp (93). **Chưa** đo lại với model thật. |
| 20/10/2026 | WPS Linux + Core | **Đợt I — khung chat mini cạnh đoạn bôi đen** (người dùng đề xuất; làm thẳng, không qua bản thử). Mục chuột phải mở `mini.html` bằng `Application.ShowDialog` (hàm có trong SDK chính thức `@wps-jsapi/*/js/ribbon.js`, gọi y cách SDK: kích thước × `devicePixelRatio`, không modal). Trang dùng lại `pane-core.js`; khác duy nhất là `create({requireSelection: true})` — **Core** thêm `options.requireSelection` (`Request.RequireSelection`, lệnh nhanh luôn bật) để câu gõ tay trong khung mini không bị hiểu thành sửa cả tài liệu. `:root` của `mini.html` chép nguyên từ `pane.html`, spec bắt hai bản giống hệt. Đường relay qua Core (đợt H) giữ làm **dự phòng** khi WPS không có `ShowDialog`. Spec: `test_mini_view.mjs` (7), `test_pane.mjs` +2 (181), e2e +1 (94), Go +1; thử phá bốn chỗ đều đỏ. **Chưa biết**: `ShowDialog` không nhận toạ độ nên cửa sổ có thể **không** nằm sát đoạn bôi đen; và sau khi model `typeText` thay cả đoạn, vùng chọn có thể co về con trỏ — khi đó câu gõ thêm sẽ báo "Hãy bôi đen…" thay vì sửa tiếp (an toàn, nhưng phải bôi đen lại). |
| 20/10/2026 | WPS Linux | **Thử khung chat mini trên WPS thật**: người dùng đánh giá "khá ổn" và cho commit. Chưa có số đo riêng về vị trí cửa sổ, về việc model còn hỏi lại hay không sau `QuickEditRule`, hay về gõ thêm sau khi vùng chọn co lại — ghi vào đây khi có. |
| 20/10/2026 | WPS Linux | **Icon ribbon chuyển sang `imageMso`** (người dùng thấy ribbon WPS kém bản Word: icon SVG tự vẽ một màu nét mảnh, trong khi bản Windows dùng bộ icon của Office). WPS có bảng lệnh riêng ánh xạ `idMso` → icon của nó (`res/commands/<app>/ongmani/*.kuip`, 965 tên); chọn tên có ở cả ba app cho nút dùng chung, tên của đúng app cho ba nút *Soát*; năm tên Windows không có trong WPS được thay (bảng §5.3). Thử trên bản cài trước (mã nguồn chưa đụng) → người dùng xác nhận **hiện icon**. Rồi mới đưa vào nguồn: 12 nút ribbon `imageMso`, bỏ `getImage`; xoá 11 tệp SVG mồ côi, còn `askAi.svg` + `quickMenu.svg` cho menu chuột phải; `OnGetImage` chỉ trả đường dẫn cho tag có tệp (`ICON_FILES`) — tag khác trả rỗng chứ không trả đường dẫn 404 (WPS sẽ vẽ `?`). Spec `test_pane.mjs` ghim bảng tên đã đo, khớp control ↔ tệp ↔ `ICON_FILES` (182); thử phá ba chỗ đều đỏ. |
