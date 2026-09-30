# Changelog

Mọi thay đổi đáng chú ý của project được ghi ở đây.
Format tham khảo [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Fixed — Core dừng khi đang chạy lượt: agent phải dừng sửa tài liệu (`core-go/`)
- Bản Go trước đây không hủy các lượt đang chạy lúc Core dừng (bản .NET làm trong `ApplicationStopped`),
  nên agent có thể còn gửi lệnh xuống bridge sau khi Core đã tắt → tài liệu bị sửa dở. Nay Core hủy mọi
  lượt đang chạy **trước khi** đóng HTTP server (pane còn nhận được `run.cancelled`) rồi chờ tối đa 2s cho
  các lượt dừng hẳn
- Thêm phần e2e `shutdown` (dùng được cho cả hai bản): dừng Core giữa lúc agent đang ghi tài liệu → bridge
  không nhận thêm lệnh nào và tiến trình Core thoát

### Added — Hai vùng chức năng trước đây chưa có e2e: codec Anthropic và embedding
- Phần e2e **`anthropic`**: chạy một lượt thật qua `/messages` với máy chủ giả trong tiến trình — kiểm tra
  `x-api-key` + `anthropic-version`, `system` là trường riêng (không có message role system), tool theo dạng
  `input_schema`, `max_tokens`, vòng `tool_use` → `tool_result` giữ đúng `tool_use_id`, và ảnh chụp màn hình
  đi trong `tool_result` dạng khối `image` base64. Trước đây codec Anthropic chỉ được test đơn vị, chưa
  chạy qua lượt thật
- Phần e2e **`embeddings`** (`EmbeddingModel`): máy chủ `/embeddings` giả trả vector điều khiển được →
  kiểm tra chống trùng bằng cosine ≥ 0.96 (khác hash vẫn bị coi là trùng), vector khác thì thêm mới, và tìm
  được theo **nghĩa** dù truy vấn không chung từ khoá nào; mỗi lần ghi gửi cả lô trong một request
- Phần e2e **`summarize`**: 21 lượt trong cùng một hội thoại → Core tóm tắt ở hàng đợi nền, **chỉ khi vượt
  mốc 20 tin nhắn** (không tốn một lần gọi model mỗi lượt), và bản tóm tắt được đưa vào ngữ cảnh lượt sau
  thay cho các lượt cũ
- Phần `confirm` thêm ca **`interactive=false`** (đường `ai.ask` của agent bên ngoài, MCP, script): lệnh rủi
  ro bị từ chối **ngay** và không xuống bridge, thay vì chờ hết hạn xác nhận 8 giây
- Phần `memory` thêm các ca **PATCH** (đường dialog "Quản lý ghi nhớ" trên cả hai nền tảng): sửa nội dung,
  ghim/bỏ ghim, đặt và bỏ hạn dùng, hạn dùng sai định dạng → 400, id không tồn tại → 404, lịch sử ghi
  UPDATE/PIN/EXPIRES
- Phần e2e **`cancel`** (nút Dừng của pane): hủy giữa lúc agent đang ghi tài liệu → SSE kết thúc bằng
  `run.cancelled`, agent không sửa tài liệu nữa, `GET /v1/runs/{id}` báo `cancelled`, hủy lại → 409, và Core
  vẫn khỏe để chạy lượt mới. Phần `memory` thêm ca xóa cứng toàn bộ (`?scope=all&confirm=true`, thiếu
  `confirm` → 400) — nút "Xóa toàn bộ ghi nhớ" trong dialog
- Phần e2e **`mcp_http`**: MCP qua **Streamable HTTP** (`mcp.json` dạng `url`) — bắt tay `initialize`, giữ
  `Mcp-Session-Id` cho các request sau, gửi kèm `MCP-Protocol-Version`, đọc được cả response dạng JSON lẫn
  dạng SSE, tool đặt tên `mcp__<server>__<tool>` và vào audit. Trước đây chỉ transport stdio có test
- Bộ e2e giờ **155 kiểm tra** (14 phần chạy riêng được), tất cả đều xanh trên **cả hai bản** (.NET và Go);
  CI (Linux) chạy 13 phần (143 kiểm tra)

### Added — Wizard thiết lập cho người dùng không chuyên (Windows + Linux)
- **Một nội dung, hai bộ vẽ**: câu chữ + preset nhà cung cấp nằm ở `catalog/setup.json`, sinh ra
  `Setup/SetupCatalog.cs` (dùng chung add-in Windows và Agent Core) cùng `axiom/setup_catalog.py`;
  `scripts/generate_setup_catalog.py --check` chạy trong CI nên ba bản không thể lệch
- **Core API cho wizard**: `GET /v1/setup` (preset + bước + tính năng + việc cần làm + cấu hình đang chạy),
  `POST /v1/llm/test` (thử kết nối với giá trị *chưa lưu*), `GET /v1/llm/models` (danh sách model của máy chủ
  để chọn thay vì gõ tay); `Setup/LlmErrors.cs` dịch lỗi (401/403/404/429/5xx, DNS, mạng, hết giờ…) thành
  câu tiếng Việt kèm gợi ý sửa. Khoá API không bao giờ vào log hay response
- **Windows** (`Ai/SetupWizardForm.cs` + `Ai/CoreSetup.cs`): 5 bước Chào mừng → Kiểm tra máy (mỗi dòng có
  nút **Sửa**: khởi động lại Core, tạo khoá mới, sang bước kết nối) → **Kết nối máy chủ AI** (chọn máy chủ
  công ty/OpenAI/Anthropic/Gemini, tải danh sách model, *Kiểm tra kết nối* báo lỗi tiếng Việt, có nút xem
  khoá và mở trang lấy khoá) → Tính năng (lời thường + link **Tuỳ chọn nâng cao…** mở đúng `SettingsForm`
  cũ) → Hoàn tất (tóm tắt + **Thử ngay** chạy một lượt thật qua Core trên tài liệu đang mở). Điểm vào:
  ribbon `Settings` → **Thiết lập…** (thêm **Cài đặt nâng cao…** cho ngang bản Linux), link header pane
  `Cài đặt` → **Thiết lập**, và tự mở **một lần** khi
  pane mở mà chưa có endpoint/model (giống bản Linux, không làm phiền lần sau)
- **Linux**: `axiom/setup.py` (máy trạng thái + câu chữ, thuần Python nên test không cần LibreOffice) và
  `axiom/setupwizard.py` (dialog awt, cùng bố cục 5 bước); mục menu **Thiết lập…**, link header, thẻ mời tự mở
  một lần; tự sửa: khởi động lại Core, sinh `Token`, `chmod 0600`, gợi ý khi thiếu `python3-uno`
- Kiểm thử: `tests/lo/test_setup.py`, `SetupTests.cs` (xUnit) và `test_setup` trong e2e cho Core API;
  `tests/live/test_setup_wizard.py` bấm thật trong phiên X ảo (12 kiểm tra); CI chạy cả ba
- Ghi chú: wizard chỉ lo **cấu hình + tự sửa lỗi**, không thay installer — người dùng Linux vẫn chạy
  `install.sh` một lần như trước

### Changed — Agent Core phát hành bằng bản Go (`core-go/`, giai đoạn G6)
- `scripts/build.ps1` build Agent Core từ `core-go/` thành `AxiomOffice.Core.exe` (~11 MB, không cần .NET
  runtime trên máy người dùng; trước đây ~48 MB); `-Core dotnet` giữ đường cũ để đối chiếu
- `scripts/linux/package.sh` đóng gói `core/AxiomOffice.Core` từ bản Go (Go 1.26+, build chéo theo `--rid`);
  `AXIOM_CORE=dotnet` để đóng gói bản .NET
- Bản .NET vẫn nằm nguyên trong `src/AxiomOffice.Core` và bộ test xUnit của nó vẫn chạy: hai bản dùng chung
  `core.json`, `core.db` (schema 2) và Core API v1 nên đổi qua lại không mất dữ liệu hay phải cấu hình lại
- Kiểm chứng: e2e **94/94** với binary Go đặt đúng chỗ phát hành (`src\AxiomOfficein\Release\AxiomOffice.Core.exe`),
  82/82 trên Linux (trừ phần `mcp` cần `AxiomOffice.Host.exe`), và đóng gói Linux chạy được với `install.sh`

### Added — Agent Core bản Go, giai đoạn G4+G5: memory dài hạn, MCP client, QA thị giác (`core-go/`)
- **G4 — memory**: chuẩn hoá/bỏ dấu tiếng Việt (bảng tường minh như bản .NET), hash chống trùng, kho SQLite
  (thêm có chống trùng theo hash và theo cosine ≥ 0.96, liên kết chỉ tới memory còn sống, lịch sử ADD/UPDATE/
  PIN/UNPIN/EXPIRES/DELETE/RESTORE, ghim, hạn dùng, xoá mềm + dọn sau 30 ngày), truy hồi theo bm25 (FTS5)
  + entity boost + sigmoid như mem0 2.2.1, trích xuất sau mỗi lượt bằng LLM (bỏ qua prompt quá ngắn/lệnh thao
  tác thuần, lọc confidence/nhạy cảm/id bịa), tool `remember`/`recall`, và toàn bộ `/v1/memory` (tìm không
  dấu, sửa, xoá, khôi phục, lịch sử, xoá cứng có xác nhận)
- **G5 — MCP client**: transport stdio (JSON-RPC một dòng mỗi thông điệp, tự trả lời -32601 khi server hỏi
  ngược) + Streamable HTTP, `initialize`/`tools/list` có phân trang, tên tool `mcp__<server>__<tool>`, mcp.json
  nạp lại khi đổi, server lỗi bị ẩn tool kèm lý do; **server built-in `office`** = `AxiomOffice.Host.exe mcp`
  cạnh binary nhưng chỉ lấy tool đọc/ghi file (tool live trùng `office_action` bị loại); xác nhận trước khi
  gọi server ngoài (không tin cậy) và trước khi ghi đè file đã có; `/v1/mcp` liệt kê server + tool + lỗi
- **G5 — QA thị giác**: `look_at_document` chụp cửa sổ app qua bridge rồi gửi ảnh thật cho model (chỉ khi
  bật `VisualQaEnabled`), ảnh không vào audit
- Bản Go qua **94/94** kiểm tra e2e (bằng đúng con số của bản .NET, chạy trên Windows); trên Linux qua 82/82
  phần chạy được không cần `AxiomOffice.Host.exe`; `go test ./...` phủ thêm memory, mcp, office
- CI: job `windows` chạy trọn bộ e2e của bản Go (kèm Host.exe như gói phát hành), job `linux` chạy 7 phần và
  so sánh `prompts/extract.txt` giữa hai bản

### Added — Agent Core bản Go, giai đoạn G2+G3: lượt chạy agent và skills (`core-go/`)
- **G2 — lượt chạy**: `POST /v1/runs` (+ `GET /v1/runs/{id}`, `/cancel`, `/confirm`, SSE `events` với `?after=`),
  `GET /v1/audit`, `GET/DELETE /v1/conversations…`; orchestrator đầy đủ (kiểm tra session + `/health` của
  bridge, hội thoại theo tài liệu, ngữ cảnh 8000 token, tóm tắt khi vượt mốc 20 tin nhắn, allowlist lệnh qua
  `GET /commands`, ghi audit + transcript, `run.started|tool.*|run.completed|failed|cancelled|timedout|stopped`),
  `office_action` (chuẩn hoá tên lệnh viết sai nhẹ, `params` dạng chuỗi JSON), policy xác nhận
  (tự lưu/tự xuất, ghi đè `saveAs`, `wpp.deleteSlide`, `writer.replaceAll` trên tài liệu dài;
  `interactive:false` = từ chối ngay) và session registry (`flock`/mở handle để biết tiến trình còn sống)
- **G3 — skills**: `GET /v1/skills?app=…`, `POST /v1/skills/reload`, `load_skill`, `read_skill_file`
  (chặn `..`, đường dẫn tuyệt đối, symlink ra ngoài, giới hạn 64KB), frontmatter YAML (danh sách `- item`
  và `[a, b]`, khối `>`/`|`, nháy đơn/kép), nguồn `builtin` → `org` → `user` ghi đè theo thứ tự, gói tài
  nguyên `_design`, quét lại thư mục khi có thay đổi (Go không có FileSystemWatcher nên so dấu vết file mỗi 2s)
- Bản Go qua **60/60** kiểm tra e2e của các phần `fake_bridge`, `guards`, `skills`, `confirm`, `setup`
  (Windows và Linux); `go test ./...` phủ config, model/codec, dịch lỗi, guard + `/v1/setup`, skills
  (frontmatter, ghi đè nguồn, chặn thoát thư mục), tools (allowlist, chuẩn hoá tên lệnh, tham số) và
  agent (SSE, hàng đợi lượt chạy, xác nhận, prompt, ngân sách ngữ cảnh, mốc tóm tắt)
- Khác bản .NET có chủ ý: `conversationId` gửi kèm khoảng trắng được cắt bỏ (bản .NET coi là hội thoại mới);
  đóng kết nối keep-alive tới bridge khi Core dừng (bản .NET làm qua `HttpClient.Dispose()`) để bridge
  không nhận RST

### Added — Agent Core bản Go, giai đoạn G1 (`core-go/`, song song với bản .NET)
- Viết lại Agent Core bằng Go để phát hành **một binary ~11 MB không cần runtime** (Windows + Linux, build
  chéo không cần cgo nhờ `modernc.org/sqlite`). Bản .NET vẫn là bản phát hành cho tới khi bản Go qua toàn bộ
  e2e; kế hoạch G1–G6 trong `core-go/README.md`
- G1: cấu hình (`AXIOM_*` → HKCU + DPAPI / `config.json` + libsecret), `core.log` (cùng định dạng, xoay 10 MB),
  `core.json`, một-phiên-bản (cùng mutex với bản .NET; `flock` trên Linux), chọn port 47840–47849, guard
  (Origin 403 / token 401 / Content-Type 415), `/health`, `/v1/admin/shutdown`, migrate `core.db` schema 2
  giống hệt, model client (OpenAI-compatible + Anthropic, thử lại lỗi tạm thời, nhắc khi trả lời rỗng, bỏ
  `<think>`), vòng lặp agent, và API wizard `/v1/setup`, `/v1/llm/test`, `/v1/llm/models`
- `scripts/generate_setup_catalog.py` sinh thêm `core-go/internal/setup/catalog_gen.go` (vẫn một nguồn
  `catalog/setup.json`)
- `tests/core/test_core_e2e.py --only <phần,...>` chạy riêng từng phần (dùng để đối chiếu bản Go theo giai
  đoạn); `test_setup` dùng LLM giả riêng nên chạy độc lập được. Bản Go qua `/health` + 19/20 kiểm tra
  `setup` trên cả Windows và Linux (còn thiếu đếm skill, thuộc G3); `go test ./...` cho config, model/codec,
  dịch lỗi, guard + `/v1/setup`

### Added — Nốt phần L3 cho Linux: skill trong gói, libsecret, systemd, CI
- **Skill dựng sẵn trong gói**: `scripts/linux/package.sh` chép `skills/` (8 skill + `_design/tokens.json`)
  vào `core/skills` — Core nạp sẵn nguồn "builtin" cạnh binary, máy mới cài không cần cấu hình `SkillDirs`
- **libsecret cho khoá API trên Linux** (`LibreOffice_arch.md` mục 9): pane **Cài đặt** lưu khoá vào keyring
  qua `secret-tool` khi máy có, `config.json` chỉ giữ `libsecret:LlmApiKey`; Core (`Secrets.Unprotect`) đọc
  lại từ keyring. Máy không có `secret-tool`, keyring khoá, hoặc khoá `dpapi:` của máy Windows khác → coi
  như chưa cấu hình (không làm Core dừng), và vẫn giữ đường lưu thường trong file `0600`
- **systemd --user (tuỳ chọn)**: `install.sh --systemd` sinh unit từ `axiom-office-core.service.in`, trỏ
  thẳng tới Core vừa cài rồi `enable --now`; `--uninstall` gỡ unit. Phiên không có systemd --user thì bỏ qua
  và báo rõ. Mặc định Core vẫn chỉ chạy khi cần (pane tự khởi động)
- **CI GitHub Actions** (`.github/workflows/ci.yml`): job `linux` (Ubuntu 24.04) chạy unit test extension,
  xUnit của Core, build MCP, cài LibreOffice + `python3-uno` rồi chạy toàn bộ test lệnh bridge trên
  LibreOffice thật, test MCP kèm tool live, cuối cùng đóng gói tarball làm artifact; job `windows` chạy unit
  test, Core, build net48 (add-in + Host) và test MCP (không có Office/WPS trong runner nên không chạy làn live)
- Script shell trong repo (`scripts/libreoffice.sh`, `scripts/linux/*.sh`) được đánh dấu thực thi trong git
- Test: `tests/lo` 56 test (thêm `SecretStoreTests` cho `protect_secret`); `tests/core` 201 test (thêm
  `Api_key_libsecret_doc_tu_keyring` dùng `secret-tool` giả trong PATH, không đụng keyring thật)

### Added — MCP server đa nền tảng `axiom-office-mcp` (LibreOffice_arch.md mục 11)
- **`src/AxiomOffice.Mcp`** (`net10.0`, `axiom-office-mcp`): MCP stdio 50 tool — 20 tool file (docx/xlsx/
  pptx/csv: đọc, tạo, sửa, format, export) + tool live gọi bridge qua HTTP + `office_sessions`. **Dùng
  chung mã nguồn** với `AxiomOffice.Host` (compile lại `src/AxiomOffice.Host/Mcp/*.cs` với ký hiệu
  `PORTABLE`), nên hai bản không thể lệch nhau về hành vi tool
- Lớp `Compat/` cho bản .NET 10: `JavaScriptSerializer` trên `System.Text.Json` (cùng API mà `McpServer.cs`
  dùng, `DeserializeObject` trả `Dictionary<string, object>`/`object[]` như bản cũ), `Config`/`Logger`/
  `SessionRegistry` đọc HKCU + `%LOCALAPPDATA%` trên Windows và `~/.config/axiom-office/config.json` +
  `$XDG_RUNTIME_DIR/axiom-office` trên Linux, danh sách lệnh cho mô tả tool `*_command` nhúng sẵn
  (`live-commands.json`, sinh từ registry extension bằng `scripts/generate_mcp_commands.py`)
- `.xls` trên Linux: nhờ `soffice --headless --convert-to xlsx` với **profile riêng** (không đụng phiên
  LibreOffice đang mở) rồi đọc như file xlsx; không có LibreOffice thì báo lỗi rõ. Bản Windows vẫn dùng COM.
  `FindSoffice` dò `AXIOM_SOFFICE` → PATH → các vị trí cài quen thuộc (Linux + Windows)
- Gói Linux: `scripts/linux/package.sh` publish thêm `mcp/` (self-contained, máy đích không cần .NET);
  `install.sh` cài vào `~/.local/share/axiom-office/mcp`, gỡ bằng `--uninstall`, và in sẵn đoạn cấu hình
  `mcpServers` cho Claude Code/Desktop
- Test: `tests/mcp-host/test_mcp_portable.py` (không cần thư viện ngoài, chạy cả Windows lẫn Linux: giao
  thức, 50 tool, các tool file trên file thật, `.xls` qua LibreOffice, tool live khi có app mở, `--live`);
  `tests/mcp-host/test_mcp_host.py` chạy được cho bản .NET 10 qua `AXIOM_MCP_CMD` + `AXIOM_MCP_NO_CATALOG`;
  `tests/lo/test_extension.py` thêm test chống lệch giữa `live-commands.json` và registry
- Đã kiểm chứng: parity đầy đủ với bản Python (python-docx/openpyxl/python-pptx đọc file do bản .NET 10 ghi
  và ngược lại) **103/103 trên cả Windows và Linux**; `test_mcp_portable.py` 61/61 (không có app) và
  70/70 (LibreOffice Calc đang mở, có ghi/đọc ô thật qua bridge); bản net48 (`AxiomOffice.Host.exe`) vẫn
  biên dịch và chạy nguyên như trước

### Added — LibreOffice trên Linux (LibreOffice_arch.md giai đoạn L2–L3)
- **Agent Core chạy được trên Linux**: `TargetFramework` `net10.0` (bỏ `-windows`); cấu hình đọc từ
  `~/.config/axiom-office/config.json` (`JsonConfigSource`, cùng tên khoá với HKCU — giá trị bool ghi
  `1/0`, `SkillDirs` là mảng), khoá API để plaintext trong file `0600` (không có DPAPI), dữ liệu theo XDG
  (`~/.local/share/axiom-office`), session ở `$XDG_RUNTIME_DIR/axiom-office/sessions`. `DocumentKey` chỉ
  hạ chữ thường/đổi `\` trên Windows (Linux phân biệt hoa thường), memory tài liệu nhận đường dẫn `/`
- **Extension**: dùng chung một mã nguồn cho hai hệ điều hành; pane tự khởi động Core từ vị trí cài
  (`<data_dir>/core/AxiomOffice.Core`) khi chưa có `CoreExe`, và trên Linux tách hẳn session khỏi `soffice`
  (`start_new_session`) để Core không chết theo LibreOffice; `ai.ask` dùng chung đường khởi động với pane
  (trước đây có bản sao riêng, không biết vị trí cài)
- **Đóng gói & cài đặt cho Linux** (`scripts/linux/`): `package.sh` tạo
  `dist/axiom-office-linux-x64-<ver>.tar.gz` (Core self-contained + `.oxt` + `install.sh`, máy đích không
  cần .NET); `install.sh` cài không cần root (Core vào `~/.local/share/axiom-office/core`, `unopkg add
  --force`, ghi `config.json` `0600`, giữ cấu hình cũ khi nâng cấp, `--api-key -` đọc key từ stdin),
  `--uninstall [--purge]`. Bổ sung `scripts/libreoffice.sh` (Linux) và `scripts/package_oxt.py` (dùng chung
  hai hệ điều hành) bên cạnh bản PowerShell
- **Sửa lỗi chỉ thấy trên Linux**: footer pane bị thanh trạng thái che (sidebar VCL cấp chiều cao lớn hơn
  vùng vẽ thật — trừ 12px khi ở trong sidebar); mã lệnh bị cắt ở mép phải do font mono rộng hơn Consolas;
  dòng lỗi rút gọn tên kiểu lỗi (`ArgumentException: …`) để xuống dòng được trong cột nhãn hẹp
- **Dọn session của tiến trình đã chết**: `sweep()` xoá file `{pid}-{kind}.json` ngay khi khởi động nếu
  pid không còn sống (`kill 0` trên Linux, `OpenProcess` trên Windows), không phải chờ hết 10 phút theo
  heartbeat — trên Linux `soffice` hay bị tắt bằng SIGTERM/đăng xuất nên file cũ tồn đọng thành session ma
  trong `office_sessions`/Core
- Test: bộ live test chạy được trên Linux (`soffice` theo PATH, tắt app bằng SIGTERM, token đọc từ
  `config.json`, `winreg` import mềm); `tests/lo` 51 unit test (thêm `theme.error_summary`, khởi động Core
  theo nền tảng, dọn session cũ)
- Đã kiểm chứng trên Ubuntu 24.04 + LibreOffice 24.2 (KDE Plasma X11): 159/159 test lệnh headless và
  159/159 có cửa sổ, Agent Core 200/200 (Linux và Windows), lượt `ai.ask` thật trên Writer/Calc/Impress,
  pane chạy ở cả deck sidebar lẫn pane neo bên phải (ảnh chụp trong phiên X ảo và phiên KDE thật)

### Changed — pane Ask AI của LibreOffice làm lại theo thiết kế pane Office
- Giao diện giống pane Word/Excel/PowerPoint: `axiom/theme.py` (bản LibreOffice của `PaneTheme`: cùng mã
  màu, font Segoe UI, cỡ chữ, khoảng cách, nhãn tiếng Việt cho từng lệnh, gợi ý theo app), `axiom/widgets.py`
  (hộp bo góc = nền + 4 ảnh góc PNG vẽ sẵn bằng zlib + viền 1px; nhãn/nút/chip bấm được có hover),
  `axiom/chatview.py` (danh sách cuộn được: bong bóng người dùng/AI, dòng thao tác có icon ✓/✗ và mã lệnh,
  thẻ xác nhận / ghi nhớ / lỗi, "• • •" đang chờ, màn hình đầu có chip gợi ý). `chat.py` sinh thêm danh sách
  mục có cấu trúc (`items` + `rev`) để chỉ vẽ lại mục mới/đổi
- Pane mở trong **sidebar thật của LibreOffice** khi sidebar đang hiện (Calc/Impress): sửa factory đọc tham số
  `ParentWindow`/`Frame` theo tên, panel implement `XSidebarPanel`, ẩn thanh tiêu đề panel thừa; trạng thái
  sidebar đọc qua status của `.uno:Sidebar`
- Cửa sổ không có sidebar (Writer): pane **neo bên phải và co vùng tài liệu lại** như task pane Office (không
  còn đè lên thanh công cụ/tài liệu); ✕ đóng và trả lại bề rộng; mở lại giữ hội thoại
- Ô soạn: Enter gửi (sửa lỗi Edit chèn "\n" vào giữa tin nhắn khi con trỏ không ở cuối), Shift+Enter xuống
  dòng, PageUp/PageDown cuộn hội thoại, viền đổi màu khi có focus, placeholder
- Cài đặt/Ghi nhớ: nền trắng, viền mảnh, font như pane, đặt giữa vùng tài liệu (không đè pane)
- Bấm trên pane kích hoạt khi **nhấn** chuột: panel trong sidebar không nhận được `mouseReleased`
- `writer.heading`: tiêu đề luôn thành đoạn riêng khi con trỏ đang ở đoạn có chữ (trước đây nối vào đoạn cũ
  và biến cả đoạn thành Heading)
- Test: `tests/lo/test_chat.py` 23 test (thêm mục có cấu trúc, vòng đời thẻ xác nhận, nhãn, PNG góc/icon);
  live 159/159 (thêm ca heading), cả headless lẫn có cửa sổ; đã chạy thật bằng chuột/bàn phím: chip → Gửi,
  Enter giữa câu, Hoàn tác lượt này, ✕ đóng, mở lại qua menu, phóng to cửa sổ, lượt thật trong sidebar Calc

### Added — pane Ask AI trong LibreOffice (LibreOffice_arch.md mục 10, giai đoạn L2)
- **Pane nói chuyện với Agent Core bằng Python** (`axiom/core.py`, `axiom/chat.py`): POST `/v1/runs` (kèm
  `office.port` của bridge), đọc SSE `/v1/runs/{id}/events`, `cancel`/`confirm`, `GET /v1/memory`; Core
  chưa chạy thì tự khởi động bằng `CoreExe` trong cấu hình. Logic hội thoại (`chat.py`) thuần Python:
  transcript, số thao tác sửa để "Hoàn tác lượt này", thẻ xác nhận, ghi nhớ vừa ghi, dòng trạng thái
- **Giao diện pane** (`axiom/awt.py` + `axiom/panel.py`): transcript (dòng `✓ writer.…`, `✓ Dùng kỹ năng`,
  `✓ Đã ghi nhớ`), ô nhập (Enter = Gửi), **Gửi/Dừng**, thẻ **Đồng ý/Từ chối** khi policy cần xác nhận,
  **Trò chuyện mới**, **Hoàn tác lượt này** (Writer/Calc/Impress — UNO hoàn tác được, khác Excel qua COM),
  **Cài đặt**, **Ghi nhớ**, **Đóng**; mọi cập nhật UI đi qua `UnoGate` (awt chỉ chạy trên main thread, và
  không gọi lại gate khi đã ở main thread để tránh tự treo)
- **Cài đặt** (`axiom/dialogs.py`): provider/endpoint/model/API key/CoreExe + bật ghi nhớ, tự trích xuất,
  QA thị giác; ghi vào HKCU như add-in, API key mã hoá **DPAPI** (`dpapi:<base64>`, đúng định dạng
  `Secrets.Unprotect` của Core); có "Kiểm tra Core" và "Tắt Core". **Ghi nhớ**: liệt kê/tìm/xoá qua
  `/v1/memory`
- **Hai đường mở pane**: deck sidebar "Axiom Office" (`Sidebar.xcu` + `Factory.xcu` + `axiom_panel.py`:
  `XUIElementFactory`/`XUIElement`/`XToolPanel`) và menu **Axiom Office → Ask AI / Settings…**
  (`Addons.xcu` + `ProtocolHandler.xcu` + `axiom_dispatch.py`); lệnh `ui.askpane` mở deck nếu bản
  LibreOffice có sidebar, không thì mở pane dạng cửa sổ con neo bên phải cửa sổ tài liệu
- Test: `tests/lo/test_chat.py` (12 unit test: đếm thao tác, thẻ xác nhận, ghi nhớ, dòng trạng thái, đọc
  SSE bằng server giả); đã chạy thật trên LibreOffice: gửi yêu cầu → Core sửa tài liệu → hiện phản hồi
  (Writer 7–10s), **Hoàn tác lượt này** trả tài liệu về nguyên trạng, **Dừng** hủy giữa lượt, thẻ xác nhận
  `wpp.deleteSlide` trên Impress (đồng ý → xoá slide thật), dialog Cài đặt/Ghi nhớ hiện đúng cấu hình
- `LibreOffice_arch.md` mục 14.2: các khác biệt API phải xử lý khi làm pane (model điều khiển awt không
  nhận toạ độ → đặt lên view; dialog rời không hiện được → cửa sổ con; sidebar của LO 26.8 không có phần
  tử layout; `XInitialization` nằm ở `com.sun.star.lang`; tên node ProtocolHandler = implementation name;
  MRO khi vừa `XDispatchProvider` vừa `XDispatchProviderInterceptor`)

### Added — làn LibreOffice: extension Python UNO (LibreOffice_arch.md, giai đoạn L1)
- `src\AxiomOffice.LibreOffice`: extension `.oxt` (`org.axiomoffice.bridge`) chạy trong `soffice` — job
  `OnStartApp` bật bridge HTTP trên 47851/47852/47853 (Writer/Calc/Impress, kind `wps`/`et`/`wpp`),
  **giữ nguyên giao thức và tên lệnh** với add-in nên Agent Core, MCP (`office_sessions` + `*_command`
  kèm `port`), skill, memory và policy dùng lại không đổi. Port đổi qua `PortLibreOffice`, token dùng
  chung `Token`; bận cổng thì thử +10 (5 lần)
- `UnoGate`: mọi lệnh UNO chạy trên main thread qua `com.sun.star.awt.AsyncCallback`, khoá tuần tự như
  `ComGate`; `/health` thêm `stuck` khi main thread không trả lời (hộp thoại đang mở) và lỗi `Busy` nêu
  rõ phải đóng hộp thoại. Mỗi lệnh AI = **một bước Undo** (`XUndoManager`) — Calc/Impress cũng hoàn tác
  được, khác Excel qua COM
- Đủ bộ lệnh của bản C# (test so trực tiếp với `AxiomOffice.Host.exe commands --json`: cùng tên, kind,
  cờ `ForAgent`, tham số; chỉ thêm `et.closeAll`/`wpp.closeAll` cho test): `writer.*` (bảng, style theo
  autoformat LibreOffice, replaceAll biết giới hạn không tìm xuyên đoạn), `et.*` (ô `null` để trống
  thật, công thức tính đúng, `numFmt` theo locale en-US), `wpp.*` (layout Office 1/2/11/12 →
  `AUTOLAYOUT_*`, đo tràn chữ bằng `TextAutoGrowHeight`), `writer.checkTables`/`et.checkRange`/
  `wpp.checkLayout`, `app.info`, `app.screenshot` (xuất trang/slide ra PNG), `ai.ask` (chạy qua Agent Core)
- Session: một tiến trình `soffice` ghi ba file `{pid}-{kind}.json` (heartbeat 25s, xoá khi thoát, dọn
  file cũ > 10 phút); `writer.open`/`et.open`/`wpp.open` trên file **đang mở** thì kích hoạt cửa sổ đó
  (`alreadyOpen`) thay vì load lại — tránh hộp thoại "đã mở" chặn main thread
- `scripts\libreoffice.ps1`: `-Package` (đóng gói `.oxt` vào `dist`), `-Install`/`-Uninstall`
  (`unopkg add --force`, từ chối chạy khi LibreOffice đang mở), `-Status`, `-Log`
- Test: `tests\lo\test_extension.py` (18 unit test, không cần LibreOffice: uno giả — giải mã tham số
  `{"item":…}`/chuỗi JSON/số dạng chuỗi + registry khớp bản C#); `tests\live\test_live_libreoffice.py`
  (158 kiểm tra trên Writer/Calc/Impress thật, headless lẫn có cửa sổ, tự mở/đóng LibreOffice)
- Đã kiểm chứng `ai.ask` end-to-end: Core chạy agent đầy đủ (nạp skill, đọc tài liệu, chèn heading +
  bảng, tự soát bằng `writer.checkTables`) trên tài liệu LibreOffice thật

### Docs — `LibreOffice_arch.md`: thiết kế tích hợp LibreOffice trên Linux
- Bridge là extension Python UNO (`axiom-office.oxt`) chạy trong `soffice`, **giữ nguyên giao thức bridge và tên
  lệnh** (`writer.*`/`et.*`/`wpp.*`) để Agent Core, skill, memory, test dùng lại; ba session/port như WPS
  (47851–47853); `UnoGate` đưa lệnh về main thread qua `AsyncCallback`; mỗi thao tác AI = 1 bước Undo
  (`XUndoManager`, cả Calc/Impress); bảng ánh xạ từng lệnh sang UNO
- Agent Core đa nền tảng (`net10.0`, cấu hình `config.json` + XDG, libsecret, file lock), sidebar Ask AI,
  làn file `axiom-office-mcp`, cài không cần root (`unopkg` + tarball), kiểm thử headless + CI Ubuntu,
  giai đoạn L0–L4, rủi ro và câu hỏi mở

### Added — Agent Core giai đoạn 4: Mở rộng và an toàn (New_arch.md mục 7.7, 8.6, 8.7, 8.4.6)
- **Policy xác nhận** (`PolicyEngine` + `ConfirmationBroker`): hỏi trước khi model lưu/xuất file mà yêu cầu
  không nhắc tới lưu/xuất, `saveAs`/`exportPdf` ghi đè file đã có, `wpp.deleteSlide`, `writer.replaceAll` trên
  tài liệu > 20.000 ký tự, tool MCP ngoài; SSE `confirm.required` / `confirm.resolved`, `POST
  /v1/runs/{id}/confirm`, chờ tối đa `ConfirmTimeoutSeconds` (120s), hết giờ/hủy = từ chối, model nhận
  `user declined`. Pane: thẻ **Đồng ý / Từ chối**
- `GET /v1/audit?runId=&limit=`: nhật ký tool call (params ≤ 2KB; tool không phải `office_action` ghi đủ đối số)
- **MCP client**: stdio + Streamable HTTP, `mcp.json` (`command/args/env` hoặc `url/headers`, `trusted`,
  `disabled`), khởi động lười, lỗi server → ẩn tool; server built-in `office` = `Host.exe mcp` **chỉ mở tool
  làn file** (tool live đi vòng allowlist/policy bị lọc), ghi vào file đã có thì hỏi; `GET /v1/mcp`
- **`ai.ask` chạy qua Agent Core** (giữ hình dạng response, thêm `viaCore`; chế độ không tương tác từ chối
  xác nhận ngay); bridge xử lý `ai.ask` trên thread riêng để Core gọi ngược `/cmd` không bị kẹt
- **QA thị giác tuỳ chọn** (`VisualQaEnabled`, tắt mặc định): lệnh bridge `app.screenshot` (PrintWindow, thu
  nhỏ), tool `look_at_document` gửi ảnh cho model (OpenAI `image_url` / Anthropic khối `image`); Cài đặt có ô bật
- Pane (New_arch.md mục 9.2): nút **Hoàn tác lượt này** trong Word (đếm thao tác `writer.*` có sửa tài liệu
  của lượt, gọi `writer.undo {count: N}`; hiện cả sau khi bấm Dừng); link **Trò chuyện mới** chuyển lên header
  cạnh Cài đặt và luôn hiện khi dùng Agent Core (trước ở footer, khó thấy trong Word)
- `ARCHITECTURE.MD`: HLD cập nhật cho Agent Core (container, khối chức năng, kịch bản K1/K6/K7, triển khai,
  NFR, AD-11…AD-17, rủi ro) + LLD mục 22 Agent Core
- Test: 194 unit test Core; e2e 76 kiểm tra (xác nhận đồng ý/từ chối/hết giờ, MCP server mẫu Python stdlib,
  làn file office, QA thị giác, audit); `ai.ask` qua Core trên Excel thật; `app.screenshot` trên Excel thật

### Added — Agent Core giai đoạn 3: Memory dài hạn (New_arch.md mục 8.5, học từ mem0 2.2.1)
- Schema 2: `memories`, `memories_fts` (FTS5, text chuẩn hoá bỏ dấu kể cả `đ`), `memory_links`,
  `memory_history`, `memory_embeddings`; `runs.memory_status`
- `SqliteMemoryStore`: ADD theo lô trong một transaction (chống trùng hash SHA-256 trong lô + với memory cũ,
  gần-trùng cosine ≥ 0,96 nếu có embedding, link chỉ tới memory tồn tại, history); sửa/ghim/hạn dùng/xoá
  mềm/khôi phục/xoá cứng **chỉ do người dùng**; dọn xoá mềm sau 30 ngày
- `MemoryRetriever`: chấm điểm cộng dồn như `score_and_rank` của mem0 (keyword sigmoid theo độ dài truy
  vấn, entity boost giảm dần, semantic nếu có embedding; ngưỡng 0,1 chặn tín hiệu chính trước khi cộng);
  ngữ cảnh = ghim + ≤ 20 memory tài liệu + ≤ 8 memory user, ≤ ~1.500 token, bản chuyển đổi mới đứng trước
- `MemoryExtractor` **chỉ-ADD** (prompt tiếng Việt nhúng trong exe) chạy ở **hàng đợi nền**: id tạm chống
  bịa id, lọc confidence < 0,6 / > 300 ký tự / thông tin nhạy cảm (regex CCCD, số thẻ, mật khẩu, API key),
  bỏ qua lệnh thao tác thuần; JSON lỗi thì thử lại 1 lần. Tóm tắt hội thoại chuyển sang cùng hàng đợi
- Tool `remember` / `recall`; SSE `memory.written`; `/v1/memory` (list/tìm/thêm/sửa/xoá/khôi phục/lịch
  sử/xoá toàn bộ); embedding tuỳ chọn (`EmbeddingModel`, lỗi thì tự chạy chỉ keyword)
- Pane: dòng **Đã ghi nhớ: …** + **Xoá** (cả memory trích xuất nền của lượt vừa xong); Cài đặt thêm "Dùng
  Agent Core", "Ghi nhớ dài hạn", "Tự ghi nhớ sau mỗi lượt", form **Quản lý ghi nhớ…**
- Test: 33 unit test memory; e2e 18 kiểm tra (3 phiên + khởi động lại Core, chuyển đổi chức vụ có liên kết,
  trùng hash, id bịa, lệnh thao tác thuần, xoá/khôi phục/lịch sử, `MemoryEnabled=0`); 14 kiểm tra add-in
  `CoreClient` ↔ Core thật (thư mục dữ liệu tạm)

### Fixed
- Chuẩn hoá bỏ dấu tiếng Việt trong Core dùng bảng tường minh: Core chạy `InvariantGlobalization` nên
  `string.Normalize(FormD)` không tách dấu ("in đậm" không thành "in dam"); project test cũng chạy invariant

### Added — Agent Core giai đoạn 2: Skills (New_arch.md mục 8.4)
- **Skill theo chuẩn Agent Skills**: `Skills/SkillLoader` (frontmatter `name`/`description` + `apps` tuỳ
  chọn; validate tên ≤ 64 ký tự `a-z0-9-`, từ cấm `anthropic`/`claude`, mô tả ≤ 1024 không thẻ XML; field
  lạ bỏ qua), `SkillIndex` (3 nguồn: `skills\` cạnh exe → `SkillDirs` → `%LOCALAPPDATA%\AxiomOffice\skills`,
  nguồn sau thắng; thư mục `_*` là gói tài nguyên; FileSystemWatcher debounce 2s). Skill lỗi hiện ở
  `GET /v1/skills`, không làm hỏng Core
- Tool `load_skill` (tầng 2, phát SSE `skill.loaded`) và `read_skill_file` (tầng 3: text ≤ 64KB, file nhị
  phân → đường dẫn tuyệt đối; chặn `..`, đường dẫn tuyệt đối, symlink/junction ra ngoài; chỉ `.md .txt
  .json .csv .docx .xlsx .pptx .png .jpg`). Chỉ mục skill hợp app vào system prompt (tầng 1)
- `GET /v1/skills?app=`, `POST /v1/skills/reload`
- **Tầng thiết kế**: `skills/_design/tokens.json` + `thiet-ke-van-phong`, `trinh-bay-chuyen-nghiep`,
  `the-thuc-van-ban`, `bao-cao-du-lieu`; **tầng triển khai**: `bao-cao-thang` (PowerPoint), `bang-diem`
  (Excel), `van-ban-hanh-chinh` (Word, kèm `references/the-thuc.md` theo NĐ 30/2020)
- **QA cấu trúc** (lệnh bridge chỉ đọc, agent dùng được): `wpp.checkLayout` (chữ tràn khung, ra ngoài
  slide, shape chồng, chữ < 12pt, slide quá nhiều chữ), `et.checkRange` (tiêu đề trống, kiểu lẫn lộn, số
  dạng chữ, số lẻ chưa number format, ô lỗi, dữ liệu lạc ngoài bảng), `writer.checkTables` (ô trống, ô
  tiêu đề lẫn đoạn văn). `writer.formatTable` thêm `borders: false` (bảng dàn trang)
- Pane hiện dòng **Dùng kỹ năng: …**; thao tác `load_skill`/`read_skill_file` không hiện như thao tác tài liệu
- Test: 19 unit test skill; e2e 13 kiểm tra luồng `load_skill` với LLM giả; `test_core_e2e.py --real-llm`
  (LLM thật): muse-spark-1.3 chọn đúng `bao-cao-thang` / `bang-diem` / `van-ban-hanh-chinh` cho 3 yêu cầu
  mẫu và **không nạp skill** cho "in đậm dòng đầu"; test live dựng sẵn ca tràn chữ/chồng shape/dữ liệu lạc ô

### Changed
- Hết giờ mỗi request tới model: 60s → **120s**, chỉnh được (`LlmRequestTimeoutSeconds` /
  `AXIOM_LLM_REQUEST_TIMEOUT`): model free (oc/muse-spark) có lượt sinh công văn dài hơn 60s
- Model trả lời rỗng giữa chừng: Core nhắc **một lần** để làm tiếp/tóm tắt thay vì hỏng cả lượt

### Fixed — Excel hỏi lưu một sổ lạ / chạy ngầm sau khi đóng
- Agent không còn gọi được `et.newWorkbook` (giống `writer.newDocument`/`wpp.newPresentation`): log
  30/09 01:15 model tạo thêm Book2 dù Book1 đang mở, người dùng đóng Excel thì bị hỏi lưu một sổ họ
  không biết. Excel trống (chưa có sổ) thì `et.listSheets`/`et.writeRange`/`et.formatRange` tự tạo sổ
- Add-in nhả hẳn các tham chiếu COM (`Application`, CTP factory, task pane) và ép GC khi
  `OnDisconnection`: trước đây chỉ gán null nên RCW chờ finalizer
- Tên lệnh agent viết sai nhẹ (`et_writeRange`, khác hoa/thường — gặp ở `oc/mimo-v2.6-flash-free`)
  được quy về tên đúng thay vì bị từ chối và mất một vòng (Core + in-process)

### Added — độ bền khi gọi model
- **Tự thử lại khi nhà cung cấp lỗi tạm thời** (HTTP 429/500/502/503/504, rớt mạng) ở cả Agent Core
  (`ModelClient`) và agent in-process (`LlmClient`): tối đa 3 lần, chờ 1s → 2s → 4s, theo `Retry-After`
  nếu có (tối đa 10s); không thử lại khi hết giờ một request hay người dùng bấm Dừng. Trước đây một lỗi
  503 "high demand" (gemini-3.8-flash) hay 500 ngẫu nhiên (Gemma 4) làm hỏng cả lượt chạy
- Lọc `<thought>…</thought>` / `<think>…</think>` khỏi câu trả lời hiện cho người dùng (Gemma 4 qua
  endpoint của Google, DeepSeek/Qwen trả kèm phần suy nghĩ); tin nhắn gửi lại model vẫn giữ nguyên

### Added
- **Google Gemini** trong Cài đặt: chọn "Google Gemini" điền sẵn endpoint OpenAI-compatible của Google
  (`https://generativelanguage.googleapis.com/v1beta/openai`) và model `gemini-2.5-flash`; lưu dạng
  provider `openai` nên Core, agent in-process và bản cài cũ đều dùng được (không cần SDK
  `Google.GenAI` — SDK chỉ chạy được trong Core .NET 10, còn add-in là .NET Framework 4.8)
- Core đọc lại cấu hình LLM (provider/endpoint/key/model) **mỗi lượt chạy** (`Models/ModelSource.cs`):
  trước đây chỉ đọc lúc khởi động nên đổi model trong Cài đặt không có tác dụng tới khi Core khởi động lại
- `writer.formatTable` (agent dùng được): định dạng bảng **có sẵn** — kiểu, font, cỡ, màu chữ, màu
  hàng tiêu đề, màu sọc, viền, căn lề, co giãn. Trước đây không có lệnh này nên với "tô màu bảng cho
  đẹp" agent phải `undo` (xoá cả bảng và ghi chú) rồi dựng lại, mất 20 vòng/112s
- Mô tả tool `office_action`: sửa tại chỗ, không dùng `undo` để làm lại trừ khi người dùng yêu cầu

### Fixed — lỗi Ask AI thấy khi test Word qua Agent Core
- **Style bảng Word chưa bao giờ được áp**: `table.set_Style(...)` không tồn tại khi gọi late binding
  (`dynamic`) và lỗi bị nuốt, nên `style` của `writer.insertTable` luôn bị bỏ qua mà vẫn trả ok. Nay
  gán `Style` trực tiếp; lỗi (nếu có) trả về trong `styleError`
- Sau `writer.insertTable` con trỏ nằm ở ô (1,1) nên chữ chèn tiếp lọt vào ô tiêu đề (file thật:
  dòng ghi chú nằm trong ô "Thứ"). Nay con trỏ ra ngay sau bảng
- `writer.insertTable` lỗi COM "The requested member of the collection does not exist" khi chèn bảng
  rộng hơn ngay sau/trước một bảng khác: Word gộp hai bảng liền nhau. Nay tự chèn đoạn ngăn cách
- Dòng tiến trình của pane khi chạy qua Core chỉ hiện `office_action {}`: sự kiện `tool.finished`
  kèm `paramsPreview`, pane dựng lại `{"action": ..., "params": ...}` để hiện nhãn tiếng Việt; log
  "AskAiPane: ok ... N tool calls" đếm đúng số thao tác của Core
- **Mọi yêu cầu bị làm HAI lần** (Core và in-process chạy song song): `CoreClient` đọc `runId` ở cấp
  ngoài trong khi Core trả `{"ok":true,"result":{"runId":...}}`, nên luôn coi là "Core không trả
  runId" và chạy thêm in-process. Hệ quả: dữ liệu bị ghi hai lần (vd thừa một hàng "Trung bình"),
  lỗi COM `0x800A01A8` do hai agent sửa cùng lúc, luôn hiện "chế độ cơ bản", không có link "Cuộc trò
  chuyện mới"
- **Pane chạy một yêu cầu hai lần**: Core đã nhận lượt chạy (có `runId`) nhưng đọc SSE lỗi thì
  `CoreClient.Run` trả null và pane chạy lại in-process — tài liệu bị sửa hai lần, dòng trạng thái
  hiện cả "Xong" lẫn "chế độ cơ bản". Nay chỉ quay về in-process khi Core chưa nhận lượt chạy; mọi lý
  do quay về đều ghi `bridge.log` (`CoreClient: fallback to in-process - ...`)
- `writer.insertTable`/`et.writeRange`/... báo "nested array/object at row 1, column 1" khi model bọc
  mỗi dòng thêm một lớp (`{"item":[{"item":{"item":[...]}}]}`): gỡ lớp thừa của từng dòng
- "`rows` must be a whole number, got an object with keys [item]": tham số số nhận `{"item": 6}` và
  `[6]`; `writer.insertTable` lỡ nhận dữ liệu bảng trong `rows` thì dùng như `values`
- `writer.insertTable` lỗi COM "The range cannot be deleted": chèn ở cuối vùng chọn (trong bảng thì
  sau bảng) thay vì thay nội dung đang chọn
- `writer.replaceAll` không bao giờ khớp văn bản nhiều dòng (model gửi `\n`, Word dùng `\r`/`\v`) nên
  agent lặp tới hết ngân sách 200k token: đổi `\n` → `^p`, `\v` → `^l`, `\t` → `^t`, thoát `^`
- Core: `params` gửi dạng chuỗi JSON được parse thành object; tóm tắt hội thoại chỉ khi vượt mốc mới
  mỗi 20 tin nhắn (trước đây tóm tắt lại sau **mọi** lượt khi hội thoại > 20 tin, tốn thêm một lần
  gọi model mỗi lượt)

### Added — Agent Core giai đoạn 1 (phần 2): pane chạy qua Core
- Add-in: `Ai/CoreClient.cs` — tìm Core qua `core.json`, khởi động `AxiomOffice.Core.exe` khi cần,
  `POST /v1/runs` rồi đọc SSE; hủy lượt chạy cả hai phía (abort request + `POST .../cancel`)
- `Ai/AskAiPane.cs`: lượt chạy đi qua Agent Core khi có, **tự chạy agent in-process khi Core không
  dùng được** và ghi chú "chế độ cơ bản" ở dòng trạng thái; hội thoại **nhớ theo tài liệu** (mở lại
  tài liệu vẫn tiếp tục mạch cũ, hỏi Core qua `/v1/conversations?documentKey=`); link **Cuộc trò
  chuyện mới** ở footer; dòng tiến trình lấy từ sự kiện `tool.finished`; thông báo riêng khi dừng vì
  hết ngân sách token; đọc tên tài liệu qua `ComGate` (không gọi COM song song với thread bridge)
- `Bridge/Config.cs`: `CoreEnabled`; `LlmResult` thêm `ViaCore`, `Stopped`, `ConversationId`
- Sự kiện `tool.finished` của Core kèm `resultPreview` để pane hiện dòng giống chế độ in-process
- `build.ps1`: thêm `IncludeNativeLibrariesForSelfExtract` — **sửa lỗi đóng gói**: bản single-file
  trước đó thiếu `e_sqlite3.dll` nên Core báo "SqliteConnection type initializer threw" (49,2MB)
- Test e2e mới (`tests/core/fake_llm.py` + `tests/core/test_core_e2e.py`, 22 kiểm tra không cần
  Office + 6 kiểm tra `--office` trên Excel thật): thứ tự sự kiện SSE, hội thoại, audit, allowlist
  (lệnh bị chặn không xuống bridge), hủy, trần token, lỗi office, và tài liệu thật sự đổi

### Docs — `New_arch.md`: tầng thiết kế cho skills (design intelligence)
- Tách design intelligence khỏi skill triển khai theo app: **token thiết kế là dữ liệu**
  (`skills/_design/tokens.json`), skill thiết kế nạp khi cần (`thiet-ke-van-phong`,
  `trinh-bay-chuyen-nghiep`, `the-thuc-van-ban`, `bao-cao-du-lieu`), skill theo app dùng giá trị cụ
  thể trong lệnh. Không xây router riêng (model chọn theo `description`), không nhét design system
  vào `ppt/SKILL.md`, không nạp skill thiết kế cho sửa nhỏ
- **QA hai mức** cho điểm yếu "agent mù": giai đoạn 2 kiểm tra cấu trúc bằng số (tràn chữ, shape
  chồng, số dòng/cột, number format — không cần thị giác); giai đoạn 4 mới thêm `app.screenshot` +
  model thị giác, kèm xác nhận của người dùng vì tốn token

### Added — Agent Core (giai đoạn 0 của New_arch.md)
- `src/AxiomOffice.Core/` (.NET 10, `AxiomOffice.Core.exe`): process riêng của agent, một bản cho
  mỗi người dùng Windows (mutex `Local\AxiomOffice.Core`, instance thứ hai thoát ngay), chỉ nghe
  `127.0.0.1`. Giai đoạn 0 gồm: `GET /health` (không cần token), `POST /v1/admin/shutdown`, quy tắc
  bảo vệ giống bridge (chặn `Origin` → 403, thiếu token → 401, body không phải JSON → 415), ghi
  `%LOCALAPPDATA%\AxiomOffice\core.json` khi sẵn sàng và xoá khi thoát, log `core.log` (xoay 10MB × 3)
- Cấu hình Core: `HKCU\Software\AxiomOffice` (`CorePort` 47840, `CoreEnabled`, `MemoryEnabled`,
  `MemoryAutoExtract`, `LlmProvider/Endpoint/Model`, API key DPAPI) + override `AXIOM_*` để test
  không đụng cấu hình thật; port bận thì tự thử 47840–47849
- Bridge: `GET /commands` trả bộ lệnh của DLL đang chạy (tên, loại app, cờ agent, tham số, `version`)
  — Agent Core dùng để dựng tool cho agent đúng phiên bản
- `scripts/install-dotnet-sdk.ps1` (cài .NET 10 SDK không cần admin), `scripts/core.ps1` (tắt Core
  êm qua API), `build.ps1` publish Core self-contained single-file với version của DLL (bật
  `EnableCompressionInSingleFile`: 103MB → 47,8MB đo trên máy này), `package.ps1` đóng gói kèm Core
  + thư mục `skills\`, `install.ps1` tạo khoá cấu hình mới, `uninstall.ps1` tắt Core
- `tests/core/AxiomOffice.Core.Tests` (xUnit, 35 test): cấu hình (ưu tiên env > HKCU > mặc định,
  DPAPI, port không hợp lệ), core.json, log (xoay file), chọn port, và test vòng đời trên **tiến
  trình thật** (core.json ↔ tiến trình, /health, 401/403/415/404, instance thứ hai, shutdown)
- `tests/live/test_live_commands.py`: kiểm tra `/commands` khớp registry và version Core ↔ bridge khi
  Core đang chạy

### Docs — `New_arch.md`: yêu cầu triển khai Agent Core
- Bản yêu cầu cho phiên làm việc mới: tách agent ra process riêng `AxiomOffice.Core.exe` (.NET 10,
  một bản mỗi người dùng) với hội thoại liên tục, skills (`SKILL.md`), memory SQLite, policy/xác
  nhận, MCP client; add-in mỏng lại, giữ agent in-process làm dự phòng
- Hợp đồng Core API v1 + SSE, `GET /commands` trên bridge, schema DB, build/đóng gói, chiến lược
  test (fake LLM theo kịch bản), 4 giai đoạn có tiêu chí hoàn thành, quy tắc bắt buộc của dự án
- Memory tự làm bằng C#, học từ mem0 2.2.1 đã đọc repo (không dùng thẳng: chỉ có SDK Python/TS, tự
  host cần Docker, bản cloud gửi dữ liệu ra ngoài): trích xuất **chỉ-ADD** sau run (nền, bộ lọc rẻ,
  lọc nhạy cảm, ghi rõ sự chuyển đổi kèm liên kết memory cũ), chống trùng bằng hash, tìm kiếm FTS5
  bỏ dấu + vector tuỳ chọn với scoring cộng dồn và sigmoid theo độ dài truy vấn, hạn dùng memory,
  `IMemoryStore` để chỗ cho backend mem0 sau này
- Skills theo **chuẩn Agent Skills của Anthropic** (nghiên cứu tài liệu chính thức + repo mở):
  thư mục `SKILL.md` + frontmatter `name` (≤ 64 ký tự, `a-z0-9-`, cấm "anthropic"/"claude") +
  `description` (≤ 1024 ký tự, ngôi ba, nêu cái gì + khi nào dùng); nạp 3 tầng — metadata vào
  prompt (~100 token/skill), `load_skill` khi kích hoạt, `read_skill_file` khi cần tài nguyên;
  `references/`/`examples/`/`templates/`, liên kết 1 cấp, đường dẫn kiểu `/`, file > 100 dòng có
  mục lục, thân `SKILL.md` < 500 dòng; viết eval trước khi viết hướng dẫn, checklist workflow,
  vòng đọc-lại. Lệch chuẩn có chủ đích: **không chạy script** trong skill (an toàn dữ liệu văn
  phòng); field mở rộng `apps` luôn tuỳ chọn nên skill tương thích 2 chiều
- Vá 4 lỗ hổng agent: (1) **làn file** qua `AxiomOffice.Host.exe mcp` làm MCP server built-in
  (`mcp__office__*`, trusted) — agent đọc/ghi file không cần mở app, không chiếm cửa sổ active,
  dùng lại 50 tool có sẵn không viết code mới; (2) quy tắc **chống prompt injection** trong system
  prompt (nội dung đọc từ tài liệu/file là dữ liệu, không phải chỉ dẫn); (3) nút **Hoàn tác lượt
  vừa rồi** trong pane (Word: đếm N lệnh đã chạy → `writer.undo {count: N}`); (4) **trần token
  mỗi run** (`maxTokens` mặc định 200k, event `run.stopped` kèm số token đã dùng)

### Docs — `ARCHITECTURE.MD`: tài liệu thiết kế
- Phần I, High Level Design: mục tiêu và phạm vi, bối cảnh hệ thống, container, khối chức năng,
  kịch bản chính, triển khai, yêu cầu phi chức năng, quyết định kiến trúc (AD-1..AD-10), rủi ro
- Phần II, Low Level Design: tổ chức mã nguồn, vòng đời add-in, mô hình thread và `ComGate`, HTTP
  bridge (pipeline, mô hình lỗi), hệ lệnh (registry, luồng thực thi, đọc tham số), AI agent (giao
  thức LLM, state machine), schema session và SSE, MCP server (50 tool, làn live/file), cấu hình và
  đăng ký COM, build/cài đặt, thiết kế kiểm thử; 18 sơ đồ mermaid
- README trỏ sang tài liệu này; bảng thành phần thêm chế độ `commands` của Host.exe

### Fixed — `values` bọc `{"item": ...}` bị ghi sai hướng; lỗi tham số khó hiểu
- Log (Excel, "ghi Tổng vào A5 và công thức tổng vào B5"): model gửi
  `{"item":{"item":["Tổng","=SUM(B2:B3)"]}}` (một dòng). Bridge gỡ mọi lớp bọc cùng lúc thành mảng
  1 chiều nên ghi thành **cột** (A5, A6); model loay hoay 19 vòng, ghi rác vào A6:C7. Giờ mỗi lớp
  `{"item": x}` là một cấp mảng (x không phải mảng = phần tử duy nhất), bỏ lớp bọc thừa ngoài cùng
  và ô bị bọc `{"item":"a"}`; nhận cả chuỗi JSON dạng object. Cùng yêu cầu giờ xong trong 3 vòng.
  Các dạng vốn chạy đúng (`{"item":[{"item":[...]}]}`, mảng 1 chiều = cột...) giữ nguyên kết quả
- Tham số số / true-false sai kiểu báo tên tham số và giá trị (`'layout' must be a whole number,
  got 'Title Only'`) thay cho `FormatException: Input string was not in a correct format.`; số
  dạng chuỗi có phần thập phân (`"12.0"`) được nhận
- `wpp.addSlide`: mô tả tool ghi rõ `layout` là số (1 tiêu đề, 2 tiêu đề + nội dung, 11 chỉ tiêu
  đề, 12 trống) — trước đó model gửi `"Title Only"`

### Changed — Ask AI: giới hạn lệnh của agent, không tự lưu file
- Tool `office_action` từ chối lệnh không có trong mô tả tool (không gắn `ForAgent()`), vd
  `writer.closeAll` (đóng mọi tài liệu, không lưu) hay `ai.ask` lồng nhau; model nhận lỗi bảo dùng
  lệnh trong danh sách, log `office_action refused: <action>`. HTTP API / MCP vẫn gọi được mọi lệnh
- Agent có thêm `writer.typeText` và `writer.appendText` (trước đó phải "nối dòng" bằng
  `writer.replaceAll` chèn `\n`)
- System prompt: không `save` / `saveAs` / `exportPdf` nếu người dùng không yêu cầu (trước đó model
  tự lưu, vd tạo `Documents\Presentation1.pptx`)
- Bảng lệnh README thêm cột **Ask AI**; `test_live_commands.py --ai` chạy `ai.ask` trên cả 3 app,
  kiểm tra không tự lưu/xuất và tài liệu còn ở trạng thái chưa lưu; kiểm tra offline (reflection
  vào DLL) rằng `office_action` từ chối lệnh ngoài danh sách

### Changed — Dọn nợ kỹ thuật: registry lệnh bridge
- Mỗi lệnh `POST /cmd` khai báo một lần (`Command(...)`: tên, loại app, handler, mô tả, tham số)
  thay cho 3 nơi phải sửa tay: `switch` của dispatcher, chuỗi lệnh của tool `office_action`, mô tả
  tool MCP. Tool `office_action` (lệnh gắn `ForAgent()`), danh sách lệnh trong mô tả MCP
  `word_command` / `ppt_command` / `wps_live_command` và bảng lệnh README đều sinh từ registry.
  Tên + tham số của 50 tool MCP không đổi; model thấy cùng bộ lệnh như trước (`writer.heading` hiện
  thêm tham số `break` vốn đã có)
- `CommandDispatcher.cs` (1721 dòng) tách theo app: `CommandDispatcher.Writer/Spreadsheet/Presentation.cs`
  + `Params.cs` (đọc tham số, chuyển giá trị COM); kiểm tra loại app (`RequireKind`) làm một lần ở
  dispatcher thay vì đầu mỗi handler. Thân handler giữ nguyên
- `AxiomOffice.Host.exe commands [--json|--markdown]`: in danh sách lệnh bridge
- `AxiomOffice.csproj` gom `**\*.cs` như `build.ps1` thay vì liệt kê từng file
- Bỏ code chết: `Probe.cs` (class COM chẩn đoán không còn đăng ký), `Ai/DocumentContext.cs` (không
  còn được gọi)

### Added — Test tích hợp lệnh bridge
- `tests/live/test_live_commands.py`: gọi mọi lệnh (và các lỗi tham số `values`, thiếu token, sai
  Content-Type, có Origin) trên Word/Excel/PowerPoint thật (`--wps`: WPS); tự mở app riêng, dừng nếu
  port đã có app của người dùng, xong tự đóng. `--record`/`--compare` so kết quả từng lệnh trước-sau
  refactor: bản registry cho kết quả trùng khớp cả 67 lần gọi với bản cũ trên Office 2024
- `tests/mcp-host/test_mcp_host.py` kiểm tra README khớp `commands --markdown` và mô tả tool MCP
  `*_command` liệt kê đủ lệnh

### Docs — README viết lại cho Axiom Office
- Giới thiệu 3 cách dùng (Ask AI, MCP server, HTTP API), sơ đồ kiến trúc Office + WPS, cài đặt
  cho người dùng (gói zip + install.cmd) tách khỏi phần phát triển, bảng tool MCP theo nhóm
- API: bảng endpoint, bảng lệnh đầy đủ (thêm `ui.askpane`, `writer.closeAll`, quy ước `values`
  mảng 2 chiều, `slide` trống = slide cuối), ví dụ PowerShell/Python có token và gửi UTF-8
- Bỏ phần lỗi thời: `build-native.ps1` (chỉ có ở branch `cpp-native-addin`), `docs/office-integration.md`
  (không tồn tại), CLSID cũ; MCP Python gọn lại thành mục legacy; troubleshooting dạng bảng


### Changed — Đổi tên dự án: WPS AI Bridge → **Axiom Office**
- Tên hiển thị (ribbon, task pane, hộp thoại, hướng dẫn cài) và toàn bộ định danh kỹ thuật:
  namespace `AxiomOffice.*`, `AxiomOffice.dll` / `AxiomOffice.Host.exe`, ProgID
  `AxiomOffice.Connect` / `AxiomOffice.AskAiPane`, CLSID mới, khóa `HKCU\Software\AxiomOffice`,
  log `%LOCALAPPDATA%\AxiomOffice`, thư mục `src/AxiomOffice*`, gói `AxiomOffice-<version>-...zip`
- `scripts/legacy.ps1`: `install.ps1` gỡ ProgID/CLSID/Office Addins/whitelist WPS của bản
  `WpsAiBridge` (tránh nạp add-in 2 lần tranh port) và chuyển cấu hình sang khóa mới — chỉ xoá
  khóa cũ sau khi đọc lại thấy khớp từng giá trị (API key DPAPI không dùng entropy nên chép
  nguyên được); `uninstall.ps1` cũng gỡ đăng ký cũ, `-Purge` xoá cả cấu hình/log cũ
- `install.ps1` đọc version từ DLL thay vì ghi cứng

### Fixed — Agent lặp tới "(da dat gioi han 8 buoc)" mà không làm được gì
- Nguyên nhân (log 20:13, "Tạo bảng điểm 5 học sinh"): model gửi `values` dạng
  `{"item":[{"item":[...]}]}`; `et.writeRange` không ghi gì nhưng vẫn trả `ok:true`, model đọc lại
  thấy ô trống nên ghi lại mãi tới khi hết 8 vòng
- `values` của `et.writeRange` / `writer.insertTable` / `wpp.addTable` đọc qua `ParamMatrix`: gỡ lớp
  bọc `{"item": ...}`, nhận cả mảng dạng chuỗi JSON; sai dạng, thiếu `values`/`range` thì trả lỗi kèm
  ví dụ `[["Họ tên","Điểm"],["An",9.5]]` để model tự sửa. Bảng tự suy ra/nới `rows`/`cols` theo
  `values` thay vì cắt bớt dữ liệu. Mô tả tool ghi rõ định dạng mảng 2 chiều
- Bỏ giới hạn số vòng của agent (trước là 8): lượt chạy dừng khi AI trả lời xong, khi bấm Dừng
  hoặc chạm trần 5 phút. Nếu bên gọi tự đặt giới hạn thì kết quả báo "chưa xong" (không còn chuỗi
  `(da dat gioi han n buoc)` giả làm câu trả lời). Log/`ai.ask` trả thêm số vòng (`rounds`)
- Đã chạy lại đúng prompt trên Excel thật: 5 vòng, 16s, bảng 5 học sinh + cột Trung bình


### Added — Đóng gói cài đặt cho người khác
- `scripts/package.ps1`: build (hoặc `-NoBuild`) rồi tạo `dist/AxiomOffice-<version>-<ngày>-<commit>.zip`
  (~245 KB; tên có `-dirty` khi code chưa commit): DLL, `AxiomOffice.Host.exe`, `install.ps1`/
  `uninstall.ps1`, `install.cmd`/`uninstall.cmd` nhấp đúp, `HUONG-DAN-CAI-DAT.txt`, NOTICE template.
  Entry zip dùng `/` (tự ghi từng entry; `CreateFromDirectory` của PowerShell 5.1 ghi `\`)
- `install.ps1` gỡ nhãn Zone.Identifier của DLL/EXE (file từ zip tải về) và in cấu hình MCP với
  đường dẫn exe đúng; add-in log đường dẫn DLL đang nạp (`Connect constructor ... from <path>`)
- Đã test như người nhận: giải nén + gắn Zone.Identifier → `install.cmd` gỡ nhãn, đăng ký trỏ vào
  thư mục gói, Word nạp add-in từ đó, MCP trong gói chạy (office_sessions, doc_create)

### Fixed
- `uninstall.ps1` sót đăng ký COM của Ask AI pane (`AxiomOffice.AskAiPane`, CLSID `{D99F8693-...}`);
  thêm `-Purge` xoá cả `HKCU\Software\AxiomOffice` (token, cấu hình AI) và `%LOCALAPPDATA%\AxiomOffice`


### Added — MCP server C# trong `AxiomOffice.Host.exe mcp` (thay 3 server Python)
- Người dùng không còn phải cài Python/venv/pip: `AxiomOffice.Host.exe mcp` là MCP server stdio
  (JSON-RPC 2.0, protocol 2024-11-05 → 2025-11-25) với cả 50 tool của word/excel/ppt-mcp, cùng
  tên và tham số; `mcp word|excel|ppt` để nạp từng nhóm, `mcp --list` in danh sách
- Làn file đọc/ghi OOXML trực tiếp (ZipArchive + XLinq, không thư viện ngoài): docx (paragraph,
  bảng có ô gộp, section, style, core props), pptx (clone placeholder từ layout như python-pptx,
  ghi chú), xlsx/xlsm (đọc streaming, shared strings, ngày tháng theo numFmt, dịch shared formula
  cho `show_formula`, ghi shared strings, styles font/fill/border/alignment/numFmt, table,
  thêm/chép/đổi tên/xoá sheet), csv/tsv; `.xls` đọc qua Excel/WPS (COM)
- Sửa file chỉ ghi lại phần XML liên quan: chart/ảnh/pivot/macro của file gốc được giữ (openpyxl
  làm mất); ghi công thức thì bỏ calcChain + `fullCalcOnLoad` để Excel tính lại khi mở
- Làn live + `office_sessions()` gọi bridge như bản Python; lỗi kết nối trả `{"ok":false}`
- Template `default.docx`/`default.pptx` của python-docx/python-pptx (MIT) nhúng trong exe
  (`src/AxiomOffice.Host/Mcp/Templates/NOTICE.md`)
- Bỏ: `excel_query` (DuckDB SQL) và xuất parquet của `excel_convert` (chỉ còn csv)
- `tests/mcp-host/test_mcp_host.py`: MCP client Python chính thức + so parity từng tool file
  với `file_tools.py` bản Python trên file do C#, python-docx/openpyxl/python-pptx và Office thật
  tạo (152/152); `office_roundtrip.ps1` mở file C# ghi ra bằng Word/Excel/PowerPoint 16 thật.
  Làn live đã chạy qua Word thật (styled text, heading, undo)


### Added — Event stream SSE `GET /events` (Phase 2)
- Mỗi bridge phát Server-Sent Events: `hello`, `document` (tài liệu active đổi),
  `selection` (Writer: text ≤ 200 ký tự + start/end; ET: sheet + address + giá trị
  ô trên-trái; WPP: slide/type/shape/text), `ping` mỗi 15s
- V1 poll-diff 500ms (không COM event sink — chạy giống nhau Office/WPS), poller chỉ
  chạy khi có subscriber, tối đa 5 subscriber (thứ 6 nhận 503); mỗi kết nối có thread
  ghi riêng nên `/cmd` không bị chặn (đo: 12 ms trong lúc stream); lỗi COM log tối đa
  1 lần/phút mỗi loại
- Python: `bridge.es_hint()` trong cả 3 MCP (URL `/events` + ví dụ curl)
- Đã verify: Word 47831 (selection khi bôi đen, document khi mở file khác, ping),
  Excel 47832 (selection theo địa chỉ `$B$2`, `$C$3:$D$5`), WPS Writer 47821

### Added — Session registry + `GET /session` (Phase 1)
- `Bridge/SessionRegistry.cs`: mỗi bridge ghi `%LOCALAPPDATA%\AxiomOffice\sessions\{pid}.json`
  (`pid`, `app`, `family` office|wps, `port`, `host`, `started`, `lastSeen`, `document`),
  heartbeat 25s, xoá file khi Stop/OnDisconnection; đọc tên tài liệu chạy nền, chờ tối
  đa 2s, host bận thì giữ giá trị cũ (heartbeat không bao giờ trễ vì Word bận)
- `GET /session` (cần token): thông tin bridge + tài liệu đọc live lúc gọi
- Python `bridge.sessions()` trong cả 3 MCP: đọc registry, gọi `/health` (1.5s) để đánh
  dấu `healthy`, prune khi process chết hoặc heartbeat > 90s mà health cũng fail
  (bridge đang bận lệnh dài vẫn được giữ, `healthy: false`) + unittest 5 case
- MCP tool `office_sessions()` cho cả 3 server; docstring `wps_live_command` /
  `word_command` / `ppt_command` hướng dẫn truyền `port` lấy từ danh sách này
- Đã verify: Word + Excel cùng lúc → 2 session đúng port/family; WPS Writer → family `wps`;
  kill Word/WPS → session bị prune; regression `/health` 200, `/cmd` 401/415/403, `ai.ask` ok

### Changed — Ask AI pane thiết kế lại
- Design tokens `PaneTheme` (màu theo vai trò, font Segoe UI/Semibold tạo một lần, spacing,
  radius, scale theo DPI); nút Ask/link đổi sang `#4F46E5` vì trắng trên `#6366F1` chỉ đạt
  4.47:1; viền ô nhập `#8C93A0` (3.09:1)
- Empty state có tiêu đề theo loại tài liệu + 3 chip gợi ý xếp dọc (theo host Writer/ET/WPP);
  bubble có "đuôi", nhận Tab/Ctrl+C và menu Sao chép; dòng tool activity (tick xanh / x đỏ +
  nhãn tiếng Việt + mã action, tooltip là dòng log gốc); typing 3 chấm tôn trọng reduced motion
- Composer tự cao 2–5 dòng, Ask chỉ bật khi có chữ, khoá kèm hướng dẫn khi chưa cấu hình AI;
  footer có trạng thái + chấm "đang chạy" + link **Dừng** / **Chèn trả lời**; thẻ lỗi có
  **Thử lại** / **Mở Cài đặt**; trạng thái xong/lỗi được báo cho trình đọc màn hình
- `PromptBox`: edit multiline không phát `EN_CHANGE` khi text đặt bằng `WM_SETTEXT` (UIA,
  automation) — tự báo `TextChanged` để Ask bật

### Fixed
- **Word crash (AV `wwlib.dll`) khi nhiều thread cùng gọi COM**: heartbeat đọc
  `ActiveDocument` đúng lúc `ai.ask` đang ghi → cuộc gọi thứ hai được Word dispatch
  reentrant. `Bridge/ComGate.cs` tuần tự hoá mọi truy cập object model của bridge (Pump,
  agent của pane, heartbeat, poller); heartbeat/poller bỏ qua lượt khi cổng bận
- **Agent chạy mãi không kết thúc / kết thúc âm thầm** (BUG-1): trần 5 phút cho cả lượt
  (`LlmClient.AgentTimeoutMs`), hủy qua `CancellationToken` (abort request LLM đang chờ,
  không chạy thêm tool); log kết quả ghi ngay trên worker; `BeginInvoke` thất bại và lỗi
  trong callback UI đều được log thay vì `catch {}`; `ai.ask` log từng tool (`ai.ask progress:`).
  Không tái hiện được treo trên build hiện tại (cùng prompt bảng điểm: 16.9s, 2 tool calls)
- **Tài liệu ẩn** (BUG-2): cửa sổ `OpusApp` ẩn không title là của Word, có từ lúc khởi động
  (trước khi tạo pane). Log trạng thái tài liệu/cửa sổ ở `OnConnection`, `OnStartupComplete`,
  lệnh bridge đầu tiên và trong `app.info.state`; `writer.*` kích hoạt tài liệu đang hiển thị
  nếu `ActiveDocument` không có cửa sổ visible
- **Chip gợi ý không được layout lúc mở pane** (BUG-3): empty state đi qua `ChatList.AddBlock`
- **Task pane hẹp trên WPS** (BUG-6): pane đo lại độ rộng sau khi host layout, quy đổi đơn vị
  CTP theo tỉ lệ đo được và nới về 360px; WPS 12.1.0.28485: 250 → 360px
- `ui.askpane` tạo CTP trên UI thread chính (trước đây chạy trên thread HTTP nên RCW của pane
  thuộc apartment khác)
- Danh sách chat không cuộn hết xuống cuối (AutoScroll bỏ qua Padding đáy; scrollbar xuất hiện
  làm bubble cao thêm): `AutoScrollMargin` + giữ vị trí cuối khi đo lại
- **Build làm mất DLL** (BUG-8): `build.ps1` hỏi Restart Manager tiến trình nào đang lock DLL/EXE
  (assembly .NET không hiện trong `Process.Modules`), dừng kèm tên + pid hoặc `-Kill`; biên dịch
  vào `.stage` rồi mới chép đè; dọn `<guid>_AxiomOffice.dll` còn sót

### Ideas (chưa làm)
- Event stream là nền cho timeline/transaction sau này: thêm event `change` (hash nội dung
  theo đoạn) để agent biết user vừa sửa gì mà không cần đọc lại cả tài liệu
- Poll-diff đủ cho selection/document; khi cần độ trễ thấp hơn có thể chuyển Writer sang
  COM event sink (`WindowSelectionChange`) nhưng phải giữ đường poll cho WPS


### Added — Ask AI dạng Task Pane + AI agent thao tác live
- **Ask AI = task pane dock trong app** (`ICustomTaskPaneConsumer`/`ICTPFactory`
  — cùng API cho Microsoft Office và WPS), fallback cửa sổ nổi nếu host không
  hỗ trợ; pane chat có Ask / Insert reply / Settings, transcript từng bước
- **Agent mode**: LLM **tool-calling** (OpenAI + Anthropic) gọi tool
  `office_action` → thực thi `writer.*/et.*/wpp.*` **trực tiếp lên tài liệu đang
  mở** (real-time trước mặt user); mỗi action Word = 1 Ctrl+Z (UndoRecord);
  tự fallback chat thường nếu provider không hỗ trợ tools
- Bridge command mới **`ai.ask {prompt}`** — gọi agent từ bên ngoài qua HTTP
- Debug notes (ghi lại để đời):
  - `CTPFactoryAvailable` tham số phải khai báo `object` (marshal trực tiếp
    `ICTPFactory` fail âm thầm — host vẫn "gọi" nhưng QI lỗi)
  - `ICustomTaskPaneConsumer` là **dispinterface**: khai báo `IUnknown` →
    host gọi vtable slot IDispatch → **crash WINWORD** (0xc0000005) ngay sau
    `OnAddInsUpdate`, trước `GetCustomUI`
- Đã verify trên Word 2024: prompt *"Soạn cho tôi một mẫu đơn xin việc"* →
  4 tool calls (getText → insertStyledText header → heading → insertStyledText
  thân đơn), 41s, docx 1326 ký tự tiếng Việt chuẩn Unicode

### Added — MCP server Word (`tools/word-mcp`) và PowerPoint (`tools/ppt-mcp`)
- `word-mcp` (19 tools): làn file python-docx (profile/get_text/find_text/
  extract_table/create — atomic save) + làn live qua bridge (health, command,
  read/type/styled text, format_selection, heading, insert_table/image/hyperlink,
  replace_all, export_pdf, undo, save) — mặc định Microsoft Word, `app="wps"` cho WPS
- `ppt-mcp` (15 tools): làn file python-pptx (profile/get_text/create/add_slide_file)
  + làn live (health, command, list/add slide, add_text, add_image, add_table,
  set_notes, delete_slide, export_pdf, save)
- Mỗi server: venv riêng, run.cmd, unittest (word 5/5, ppt 4/4), MCP protocol
  roundtrip pass trên Word/PowerPoint thật
- **fix**: `writer.undo` dùng `Document.Undo` (Word object model không có
  `Application.Undo`); undo theo UndoRecord hoạt động end-to-end — `undo x2`
  hoàn tác heading + table đúng 2 bước về text gốc

### Added — Live command pack v2 (tham khảo ppt-mcp / word-mcp-live)
- **Word** (10 lệnh mới): `insertStyledText`, `formatSelection`,
  `setParagraphAlignment`, `insertTable` (kèm dữ liệu + style), `insertPageBreak`,
  `insertImage`, `insertHyperlink`, `heading` (Heading 1-9, tự xuống dòng),
  `undo`, `exportPdf` — mọi thao tác ghi được bọc trong **Word UndoRecord**
  (mỗi action của AI = 1 bước Ctrl+Z, pattern từ word-mcp-live)
- **Excel** (4): `formatRange` (bold/italic/fontSize/fontColor/fillColor/numFmt/
  horizontal/wrap), `activateSheet`, `exportPdf`, `undo`
- **PowerPoint** (7): `addSlide` có `layout`, `addText` (fontSize/bold/color/
  align), `addImage`, `addTable` (kèm dữ liệu), `setNotes` (speaker notes),
  `deleteSlide`, `exportPdf`
- Đã kiểm chứng trên **Microsoft Office 2024 thật** (3 app chạy song song):
  Word chèn bảng/hyperlink/ảnh + PDF (48KB); Excel format + PDF (25KB) +
  saveAs đúng; PowerPoint slide/text/table/notes + PDF (27KB) + pptx (40KB);
  heading có paragraph break (`Tieu de\rThan bai.`)

### Fixed/Added — vòng 2 theo code review (#6 + minors)
- **Retry khi app bận**: request tự retry khi gặp `RPC_E_CALL_REJECTED` /
  `RPC_E_SERVERCALL_RETRYLATER` / `RPC_E_CALL_CANCELED` / `VBA_E_IGNORE`
  (0x800AC472 — Excel đang gõ ô) — tối đa 10 lần, backoff luỹ tiến ~5.5s,
  log mỗi lần retry; an toàn vì lỗi "call rejected" nghĩa là call chưa thực thi
- **Giá trị lỗi Excel**: `#DIV/0!` `#VALUE!` `#NAME?` `#REF!` `#NUM!` `#NULL!`
  trả về tên chuỗi thay vì số âm (probe thực tế trên Excel 16.0); `#N/A` trùng
  mã 0x800A07FA với ô trống qua `Value2` nên vẫn về `null` — giới hạn đã biết
- **Version đồng bộ**: `BridgeVersion` lấy từ assembly version (health trả
  `1.0.0` thay vì `0.1.0`)
- **DPAPI**: `LlmApiKey` mã hóa bằng `ProtectedData` (CurrentUser); key
  plaintext cũ vẫn đọc được (tương thích ngược); `SettingsForm` lưu qua
  `WriteSecret`; đã migrate key thật sang dạng mã hóa + `llm-test` gọi LLM
  qua key DPAPI thành công
- **Tests**: `tools/excel-mcp/tests/test_file_tools.py` — 9 unit test
  (roundtrip, show_formula, ragged write, format/table, sheet ops, DuckDB
  sandbox, locked-file không sót temp, csv) — chạy bằng
  `python -m unittest discover -s tests`

### Fixed — bảo mật + đúng đắn (theo code review)
- **CSRF**: bridge từ chối request có header `Origin`; `POST /cmd` bắt buộc
  `Content-Type: application/json` (chặn browser "simple request"); token
  ngẫu nhiên 32 hex tự sinh khi chạy `install.ps1`; `bridge.py` tự đọc token
  từ registry và gửi `X-Auth-Token`
- **et.writeRange**: dùng `Range.Resize[rowCount, colCount]` — ghi đúng ma trận
  khi truyền anchor 1 ô ("A1", trước đây Excel thật chỉ ghi được ô đầu); colCount
  lấy max qua các dòng (ragged rows được pad null thay vì cắt âm thầm)
- **Ribbon port**: nút Status / Copy API URL hiển thị đúng port theo host
  (truyền `IsOfficeHost` — Office hiện 4783x thay vì 4782x)
- `bridge.py`: `APP_PORTS` thêm `word/excel/ppt` (47831-47833); `wps_health`
  nhận tham số `port`
- **An toàn ghi file (excel-mcp)**: mọi thao tác ghi đi qua file tạm cùng thư
  mục + `os.replace` (atomic — lỗi giữa chừng không làm hỏng file gốc, vẫn giữ
  hint khi file bị khóa); docstring cảnh báo openpyxl có thể mất chart/image/pivot
- **Sandbox `excel_query`**: `SET enable_external_access=false` — SQL không thể
  `read_csv`/`COPY TO`/`INSTALL`/`ATTACH` (chống prompt injection đọc/ghi file
  tùy ý); đã test chặn cả 3 đường

### Added — Office ports + companion Office + excel-mcp nâng cấp (branch `main`)
- **Port tách theo host**: WPS giữ `Port` 47821-47823; Microsoft Office dùng
  `PortOffice` mới (mặc định 47831-47833) — mở song song WPS + Office thật
  không còn đụng port (bỏ workaround cũ)
- **Companion hỗ trợ Microsoft Office**: `word|excel|ppt` →
  `Word/Excel/PowerPoint.Application`; log `Companion resolved application: ...`
  (cảnh báo khi ProgID bị WPS đăng ký đè ở HKCU — trên máy dev này WPS chiếm
  cả 3 ProgID, companion sẽ tạo WPS compat component version 12.0; Office thật
  dùng in-proc add-in khi mở app)
- **excel-mcp nâng cấp** (tham khảo negokaz/excel-mcp-server): thêm
  `excel_create_sheet`, `excel_copy_sheet`, `excel_rename_sheet`,
  `excel_delete_sheet`, `excel_format_range` (font/fill/border/alignment/
  numFmt/decimalPlaces), `excel_create_table`; `excel_read` thêm
  `show_formula`; tổng 16 tools; migrate SDK `mcp` 2.x (`MCPServer`)
- Kiểm chứng trên file thật `bank-additional-full.xlsx` (41k dòng): profile
  0.01s; query trực tiếp ~5s; convert parquet + query 0.023s; MCP protocol
  roundtrip 16 tools pass

### Verified — Microsoft Office 2024 ProPlus x64 (branch `main`)
- Word (`WINWORD.EXE`), Excel (`EXCEL.EXE`), PowerPoint (`POWERPNT.EXE`): add-in
  nạp đầy đủ lifecycle (OnConnection → OnAddInsUpdate → GetCustomUI →
  OnStartupComplete), ribbon tab hiển thị, bridge hoạt động — không crash,
  không cần sửa signature nào thêm
- Excel/PowerPoint đã E2E trên app thật: ghi/đọc range (Excel), tạo slide +
  textbox + đọc lại (PowerPoint)
- Fix chuẩn hoá ô trống Excel: VT_ERROR `0x800A07FA` / `0x80020004` giờ trả
  `null` trong `readRange` (trước đó trả số `-2146826246`)
- Ghi chú: WPS ET và Excel thật cùng map port 47822 — cần đổi `Port` base khi
  chạy song song hai hệ

### Added — Ribbon UI + AI in-app + MCP server (branch `main`)
- Ribbon tab **"Axiom Office"** (`IRibbonExtensibility.GetCustomUI`, XML chuẩn
  2006/01): nhóm Local bridge (Status / Copy API URL / Open Log) + nhóm AI
  (Ask AI..., Settings); callback qua IDispatch (`ClassInterfaceType.AutoDispatch`)
- **AI Settings**: dialog cấu hình provider (OpenAI-compatible | Anthropic),
  endpoint, API key, model; nút Test gọi thử; lưu vào
  `HKCU\Software\AxiomOffice` (LlmProvider/LlmEndpoint/LlmApiKey/LlmModel)
- **Ask AI**: dialog prompt + context (None / Selection / Whole document
  ≤50k chars), gọi LLM async không treo UI, chèn kết quả vào tài liệu
  (Insert at cursor); `DocumentContext` đọc toàn văn Writer, used range ET
  (TSV, xử lý mảng 2D COM lower-bound=1), text các slide WPP
- `GET /config`: trả provider/endpoint/model + API key đã che (dạng `sk-a...7e5f`)
- Companion: chế độ `llm-test` kiểm tra cấu hình LLM từ CLI
- `tools/excel-mcp`: MCP server Python (venv riêng: mcp, openpyxl, xlrd,
  duckdb, pandas)
  - Tools file: `excel_profile`, `excel_read` (paging), `excel_query` (DuckDB
    SQL trên bảng "data"), `excel_write`, `excel_create`, `excel_convert`
    (parquet/csv)
  - Tools live: `wps_health`, `wps_live_command`, `wps_live_read_range`,
    `wps_live_write_range` — gọi HTTP bridge 47821-47823
  - Hỗ trợ `.xlsx/.xlsm/.xls/.csv/.tsv`; nhận dạng định dạng bằng magic bytes;
    xử lý file mislabeled (nội dung xlsx nhưng đuôi .xls) qua stream-based
    openpyxl; ghi .xlsm giữ VBA; PermissionError có hint "file đang mở"

### Branch `cpp-native-addin` (f2f0f00) — WIP port C++ native

### Added
- Port add-in in-proc sang C++ native thuần (không CLR):
  - `Json.h/cpp` — JSON parser + serializer UTF-8, không dependency
  - `Http.cpp` — HTTP server Winsock bound 127.0.0.1 (`/health`, `/cmd`)
  - `Com.h/cpp` — IDispatch late-binding, VARIANT ↔ JSON, SAFEARRAY 2D cho `Range.Value2`
  - `Commands.cpp` — đầy đủ parity command set với bản C#
  - `Addin.cpp` — COM object với raw vtable `[IUnknown + IDispatch + IDTExtensibility2]`,
    IClassFactory, `DllGetClassObject`/`DllCanUnloadNow`, marshaling con trỏ
    `Application` qua thread bằng `CoMarshalInterThreadInterfaceInStream`
- `scripts/build-native.ps1` — build bằng `cl.exe` VS2022 BuildTools, `/MT`
  (static CRT — WPS không cần VC runtime), x64, DLL ~320KB
- `scripts/install.ps1` — tự ưu tiên native DLL nếu có: `InprocServer32` trỏ
  thẳng DLL (bỏ mscoree/CodeBase/versioned subkey), thêm `ProgId` + `TypeLib` subkey

### Known issues
- WPS 12 nạp DLL native, tạo instance, QI `IDTExtensibility2` +
  `IRibbonExtensibility` + `ICustomTaskPaneConsumer` nhưng **không invoke
  `OnConnection`** → ghi `AddinsCL` (disable). Gate của WPS cho native add-in
  chưa xác định (nghi: WPS chỉ chạy đầy đủ flow `IDTExtensibility2` cho add-in
  managed qua mscoree). Bản C# trên `main` vẫn là đường chạy chính thức.

## [0.1.0] — 2026-09-29

Commit `a7b1132` (branch `main`). Bản đầu tiên chạy hoàn chỉnh trên WPS 12.1 x64.

### Added
- **In-proc add-in** `WpsAiBridge.dll` (C#, .NET Framework 4.8, x64):
  COM add-in `IDTExtensibility2`, mở HTTP server (HttpListener) trên localhost,
  dùng chung command dispatcher với companion.
- **Companion** `WpsAiBridge.Host.exe`: process riêng dùng COM automation
  (`KWPS/KET/KWPP.Application`) cho app chưa mở hoặc khi add-in không nạp;
  tự chuyển idle nếu port đã do add-in in-proc phục vụ.
- HTTP API: `GET /health`, `POST /cmd` với `{"action", "params"}`; token tuỳ
  chọn qua header `X-Auth-Token`.
- Command set: `app.info`; writer (`newDocument/open/getText/typeText/appendText/
  replaceAll/selection/save/saveAs`); et (`newWorkbook/open/listSheets/readRange/
  writeRange/save/saveAs`); wpp (`newPresentation/open/listSlides/addSlide/
  addTextBox/save/saveAs`).
- Port riêng từng app: 47821 Writer / 47822 Spreadsheets / 47823 Presentation
  (base +0/+1/+2, đổi qua `HKCU\Software\WpsAiBridge`).
- `scripts/build.ps1` (Roslyn csc, không cần dotnet SDK), `install.ps1`,
  `uninstall.ps1` — toàn bộ đăng ký trong HKCU, không cần admin:
  `Classes\CLSID` (mscoree + CodeBase), `Microsoft\Office\*\Addins`
  (`LoadBehavior=3`), `Kingsoft\Office\{WPS,ET,WPP}\AddinsWL` (whitelist).
- Logger dùng chung tại `%LOCALAPPDATA%\WpsAiBridge\bridge.log`.

### Fixed — root cause vụ crash `clr.dll` khi WPS nạp add-in
Triệu chứng: `wps.exe` crash `0xc0000005` trong `clr.dll` (exit code `80131506`)
ngay khi nạp add-in; WPS auto-restart, tự hạ `LoadBehavior 3→2`, ghi
`AddinsCL\WpsAiBridge.Connect=1` (disable) và hiện popup "running into problems".
Không có log nào từ `OnConnection`.

Quá trình khoanh vùng và fix:

1. **Cô lập host**: class `Probe` (không implement `IDTExtensibility2`) tạo được
   instance, không crash → lỗi nằm ở đường invoke `IDTExtensibility2`, không
   phải CLR-hosting nói chung.
2. **Loại trừ nghi phạm**: bỏ dần dynamic COM probe (`DetectAppKind`) khỏi
   `OnConnection`, tạm bỏ HttpBridge, thêm log từng bước → `OnConnection` chạy
   xong hoàn toàn rồi mới crash ở bước kế tiếp.
3. **Xác định điểm chết**: WPS invoke callback kế tiếp (`OnStartupComplete`) và
   marshal tham số `custom` khác kiểu so với `OnConnection` (VARIANT không phải
   SAFEARRAY) → CLR dispatch marshaler AV.
4. **Fix chính**: giữ `IDTExtensibility2` `[InterfaceIsIDispatch]` + `[DispId(1..5)]`,
   nhưng:
   - `OnConnection` dùng `[MarshalAs(UnmanagedType.SafeArray)] ref Array custom`
     (đúng shape WPS yêu cầu — đổi sang `ref object` thì WPS bỏ qua không gọi);
   - 4 callback còn lại (`OnDisconnection/OnAddInsUpdate/OnStartupComplete/
     OnBeginShutdown`) dùng `ref object custom` (VARIANT passthrough).
5. **Resiliency của WPS**: sau mỗi crash WPS tự hạ `LoadBehavior=2` khiến các
   lần sau add-in không được nạp — khi debug phải set lại `LoadBehavior=3` và
   xoá `AddinsCL` mỗi lần.
6. **Kiến trúc WPS 12**: mọi component chạy chung binary `wps.exe` với flag
   `/wps` / `/et` / `/wpp` → nhận diện app bằng COM probe (`Documents` /
   `Workbooks` / `Presentations`) trước, fallback tên process.

### Verified end-to-end
- Writer: `newDocument → typeText → getText → saveAs` (.docx ~10KB) OK
- Spreadsheets: `newWorkbook → writeRange A1:B2 → readRange → saveAs` (.xlsx) OK
- Presentation: `newPresentation → addSlide → addTextBox → listSlides → saveAs`
  (.pptx ~57KB) OK
- `/health` đúng cho cả 3 app chạy song song trên 3 port
