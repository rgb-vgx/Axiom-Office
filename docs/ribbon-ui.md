# Giao diện Axiom Office trên ribbon / menu — plan

Cập nhật: 13/10/2026 · trạng thái: **plan đã chốt, đang triển khai**

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
2. **Không biến API thành nút.** Nhãn là kết quả người dùng muốn ("Soát bố cục"), không phải tên lệnh
   (`wpp.checkLayout`).
3. **Không vẽ control giả.** Nút chỉ lên ribbon khi có đường thật đứng sau (API/lệnh đã có, hoặc ghi
   cấu hình mà Core đọc lại được).
4. **Không đổi hành vi lệnh bridge.** 62 lệnh của add-in COM nằm trong hợp đồng parity với MCP; việc này
   chỉ **thêm UI**, không sửa tên/tham số/kết quả lệnh.

## 2. Bộ nút

Cột "Nền tảng": W = Windows COM · L = WPS Linux (ribbon) · O = LibreOffice (menu).

### Tab 1 — "Axiom Office" (việc thường ngày)

| Nhãn | Nhóm | Control | Nền tảng | Đứng sau bằng gì | P |
|---|---|---|---|---|---|
| **Ask AI** | Trợ lý | button | W L O | pane Ask AI (đã có) | P0 |
| **Dừng** | Trợ lý | button | W L | `POST /v1/runs/{id}/cancel`; pane đã có nút Dừng (`AskAiPane.cs:105`) | P2 |
| **Nhìn tài liệu** | Kiểm tra | **toggle** | W · O ✅ · ~~L~~ | W: ghi HKCU `VisualQaEnabled`; **O: đã có** (checkbox trong "Cài đặt nâng cao", `dialogs.py:81`); **L: chặn** — xem §5.1. Core đọc lại **mỗi lượt** | P0 |
| **Soát bố cục** | Kiểm tra | button | W · **O ✅** · L | `wpp.checkLayout` — chỉ đọc, tất định, **0 token** | P0 |
| **Soát bảng** | Kiểm tra | button | W · **O ✅** · L | `writer.checkTables` | P1 |
| **Soát vùng dữ liệu** | Kiểm tra | button | W · **O ✅** · L | `et.checkRange` | P1 |
| **Thiết lập…** | Truy cập nhanh | button | W L O | wizard 5 bước (đã có) | P0 |
| **Trạng thái** | Truy cập nhanh | button | W L O | `GET /health` + session registry + cổng theo app | P0 |
| **Mở nhật ký** | Truy cập nhanh | button | W L | `%LOCALAPPDATA%\AxiomOffice\{bridge,core}.log` | P1 |

### Tab 2 — "Axiom · Quản lý"

| Nhãn | Nhóm | Control | Nền tảng | Đứng sau bằng gì | P |
|---|---|---|---|---|---|
| **Thiết lập ban đầu** | Cấu hình AI | button | W O | wizard (đã có) | P0 |
| **Cấu hình nâng cao…** | Cấu hình AI | button | W O | `SettingsForm` (đã có) | P0 |
| **Kiểm tra mô hình** | Cấu hình AI | button | W O | `POST /v1/llm/test` | P1 |
| **Kỹ năng…** | Kỹ năng | button | W L O | `GET /v1/skills?app=` + `POST /v1/skills/reload` | P1 |
| **Bộ nhớ AI…** | Bộ nhớ | button | W L O | `GET/POST/PATCH/DELETE /v1/memory`, `/{id}/history`, `/{id}/restore` | P0 |
| **Hội thoại…** | Dữ liệu | button | W O | `GET/DELETE /v1/conversations` | P1 |
| **Nhật ký hoạt động…** | Dữ liệu | button | W O | `GET /v1/audit` | P1 |
| **Trạng thái bridge** | Kết nối | button | W | `/health` + `sessions/{pid}.json` | P0 |
| **Sao chép API URL** | Kết nối | button | W | (đã có — hạ khỏi vị trí nổi bật, đây là công cụ gỡ rối) | P1 |
| **Tắt Core** | Kết nối | button | W | `POST /v1/admin/shutdown` — **bắt buộc xác nhận** | P2 |

Tab thứ hai **chỉ mở khi đã có ≥3 màn hình thật**; chưa làm gì thì nó là tab rỗng.

### Rút gọn theo nền tảng

- **WPS Linux** (ribbon.xml đơn giản): *Ask AI · Soát tài liệu · Thiết lập · Trạng thái · Quản lý…*.
  **Nhìn tài liệu chưa lên được** (§5.1). Nếu chỉ giữ được 3: **Ask AI · Soát tài liệu · Thiết lập**.
  Bộ nhớ/kỹ năng phải vào được bằng đường khác (mục **Quản lý…**), không được bỏ.
- **LibreOffice**: menu *Axiom Office* + sidebar là trung tâm. Không dựng cửa sổ tiến trình riêng — dùng
  chung lối hiển thị của pane.

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
- **Còn chưa UI nào gọi**: `GET /v1/conversations` (**Hội thoại…**), `PATCH /v1/memory/{id}` (sửa / ghim / đặt
  hạn), `POST /v1/memory/{id}/restore` + `GET /v1/memory/{id}/history` + `DELETE /v1/memory?scope&confirm`
  (khôi phục / lịch sử / xoá cứng). Nghĩa là dialog **Ghi nhớ** mới chỉ **đọc, lọc, xoá mềm** — chưa đủ
  nghiệm thu của đợt 2, và khoảng trống đó nằm ở UI chứ không phải ở Core.

## 4. Đợt triển khai

| Đợt | Nội dung | Nghiệm thu |
|---|---|---|
| **1** | Toggle **Nhìn tài liệu** + ba nút **Soát** (bố cục/bảng/vùng) | Bấm ra kết quả ngay; toggle đổi được và lượt sau Core hành xử khác; ba nút Soát không tốn token · **LibreOffice ✅ 13/10** (mục menu *Soát tài liệu…*, §8) · Windows: chờ phiên máy Windows |
| **2** | **Bộ nhớ AI…** — form 3 vùng (danh sách / chi tiết / lịch sử); phân biệt rõ **xoá mềm ≠ khôi phục ≠ xoá cứng** | Sửa, ghim, đặt hạn, xoá mềm, khôi phục, xoá cứng đều chạy; lọc được theo phạm vi user/document · **LibreOffice: mới 1/3** — dialog *Ghi nhớ* có **đọc / lọc / xoá mềm / tải lại**; thiếu sửa, ghim, hạn, khôi phục, xoá cứng (§3) |
| **3** | **Kỹ năng…**, **Nhật ký hoạt động…**, **Hội thoại…** → mở tab thứ hai | Thấy đúng skill của app đang mở; nạp lại được; audit đọc được · **LibreOffice ✅ 13/10** cho *Kỹ năng…* + *Nhật ký hoạt động…* (§8) · **Hội thoại…**: chưa (route `GET /v1/conversations` có sẵn) |
| **4** | Nút theo từng app, xuất PDF, lối tắt soạn thảo đã kiểm thử | Chỉ hiện nút hợp với app |

Cổng kiểm mỗi đợt: `make lint`, `make check-gen`, `make test` (Linux) · `scripts\build.ps1` +
`scripts\package.ps1` và CI job `windows` (Windows).

## 5. Hoãn (có lý do, không phải bỏ quên)

### 5.1 Chặn thật: toggle "Nhìn tài liệu" trên WPS Linux

Đã kiểm bằng danh sách route đầy đủ của Core: **không có endpoint nào ghi cấu hình** (không `POST
/v1/setup`, không `PATCH /v1/config`; chỉ có `POST /v1/llm/test` là ghi-gì-đó-và `POST /v1/skills/reload`).
Add-in WPS là JavaScript trong CEF nên cũng không ghi được `~/.config/axiom-office/config.json` (đã đo:
`fs.readFileString` từ chối mọi đường dẫn có dấu phân cách). Vậy muốn có toggle trên ribbon WPS thì phải
**thêm endpoint ghi cấu hình vào Core (Go) trước** — đó là việc của Core, không phải của UI, và phải làm
trước khi vẽ nút. Cho tới lúc đó, đường duy nhất trên Linux là **Cài đặt nâng cao của LibreOffice**.

Còn một đường chưa khảo sát: add-in WPS *chính là* bên thi hành lệnh bridge, nên nút ribbon có thể gọi
handler trong tiến trình (như `CommandDispatcher.Execute` ở Windows) — nhưng vòng poll của add-in là
**đơn luồng**, lệnh chậm sẽ chặn nó (đã gặp lỗi này ở live gate). Chưa kết luận được, cần khảo sát `main.js`.


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
| Ribbon thành danh mục API (65 lệnh) | Chỉ đưa nút có workflow hoàn chỉnh; còn lại qua pane. Tên nhãn theo kết quả, không theo lệnh |
| Người dùng không biết AI đang làm gì | Pane là nguồn duy nhất hiển thị tiến trình (công cụ, skill, ghi nhớ, xác nhận, kết quả) trên cả ba nền tảng |
| UI hiển thị một trạng thái, Core làm trạng thái khác | Toggle đọc trạng thái **thật** (`GET /v1/setup`), ghi thành công rồi mới đổi hình; không suy ra "Core khỏe" từ việc mở được cổng TCP |

## 7. Ràng buộc kỹ thuật

- **File sinh — sửa ở nguồn, không sửa bản sinh**: `catalog/setup.json` → `src/AxiomOffice/Setup/SetupCatalog.cs`,
  `src/AxiomOffice.LibreOffice/python/pythonpath/axiom/setup_catalog.py`,
  `core-go/internal/setup/catalog_gen.go`; registry Python → `src/AxiomOffice.WPS/commands.js`;
  `catalog/live-commands.json` → `core-go/internal/mcpserver/livecommands_gen.go`. `make check-gen` sẽ đỏ nếu lệch.
- **Máy Linux không biên dịch được C#** (`csc`/`mono`/`dotnet` đều không có) → mọi thay đổi `src/AxiomOffice/**`
  chỉ được kiểm bằng CI job `windows` (push) và bằng bấm tay trên máy Windows.
- **WPS Linux**: `tests/wps/test_pane.mjs:151` ghim nội dung `ribbon.xml` — thêm nút thì phải mở rộng spec.
- **LibreOffice**: `ACTIONS` trong `axiom/dispatch.py` là **danh sách trắng** — thêm mục menu vào
  `Addons.xcu` mà quên thêm tên vào đó thì bấm menu **không làm gì cả, im lặng**. Đã có spec chặn:
  `tests/lo/test_dispatch.py` đối chiếu hai bên.
- Giữ vững hợp đồng parity của 62 lệnh bridge (xem nguyên tắc 4).

## 8. Nhật ký triển khai

| Ngày | Nền tảng | Việc |
|---|---|---|
| 13/10/2026 | LibreOffice | Mục menu **Soát tài liệu…** (`Addons.xcu` + `dispatch.py` action `check` + `dialogs.open_check`): chạy trong tiến trình qua `bridge.run_on_main`, tự nhận loại tài liệu đang mở, liệt kê vấn đề bằng nhãn tiếng Việt (`checks.ISSUE_LABELS`) kèm chi tiết gốc. Chỉ đọc, 0 token. Spec mới: `tests/lo/test_dispatch.py` (9), `tests/lo/test_report.py` (8), `tests/lo/test_dialogs.py` (6). |
| 13/10/2026 | (kế hoạch) | Chốt plan; phát hiện §5.1 (toggle trên WPS Linux bị chặn vì Core không có endpoint ghi cấu hình). Giao phần Windows cho phiên trên máy Windows. |
| 13/10/2026 | LibreOffice | Hai mục menu **Kỹ năng…** (`org.axiomoffice.bridge:skills`) và **Nhật ký hoạt động…** (`:audit`): `Addons.xcu` → `dispatch.ACTIONS` → `dialogs.open_skills` / `open_audit`. `core.py` thêm `skill_listing` (trả **cả payload**, không chỉ `skills`, để thư mục kỹ năng nạp lỗi không bị im lặng bỏ qua), `reload_skills`, `audit`. Nút *Nạp lại* gọi `POST /v1/skills/reload` (đọc lại thư mục, **không** khởi động lại Core); nhật ký đọc 100 dòng mới nhất và đổi giờ UTC của Core sang **giờ máy**. Spec mới: `SkillLinesTest`, `SkillStatusTest`, `AuditLinesTest`, `ClockTest` (`tests/lo/test_dialogs.py`), `CoreListingTests` (`tests/lo/test_chat.py`), và một ca dispatch cho hai URL. |
| 13/10/2026 | LibreOffice | `tests/live/open_dialogs.py` — probe **không** phải unittest: dựng LibreOffice thật trong profile/XDG tạm, cài `.oxt`, gửi **đúng URL của menu** qua UNO rồi **chụp ảnh màn hình** (`xwd` + `ffmpeg`). Cần vì dialog ở đây là `awt.ChildWindow` — cửa sổ con **không có cửa sổ X riêng**, nên `xdotool search --name` không tìm được như với wizard. Đã chạy và xem ảnh: *Soát tài liệu* (bảng dựng thật bằng `writer.insertTable` → *"Có ô trống"*), *Kỹ năng* (3 kỹ năng, "nguồn: sẵn có"), *Nhật ký* (nhánh rỗng). Ảnh ở `/tmp/axiom-dialogs`. |
