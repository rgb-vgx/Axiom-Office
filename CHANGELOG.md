# Changelog

Mọi thay đổi đáng chú ý của project được ghi ở đây.
Format tham khảo [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Fixed — lỗi Ask AI thấy khi test Word qua Agent Core
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
