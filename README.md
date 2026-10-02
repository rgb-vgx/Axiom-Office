# Axiom Office

AI agent làm việc ngay trong tài liệu đang mở của **Microsoft Office, WPS Office và LibreOffice**
(Word/Writer, Excel/Spreadsheets, PowerPoint/Presentation). Nội dung AI soạn xuất hiện
trực tiếp trên trang trước mắt người dùng; mỗi thao tác của AI trong Word là một bước Ctrl+Z.

Có ba cách dùng, chung một lõi:

| Cách dùng | Dành cho | Thành phần |
|---|---|---|
| **Ask AI** — task pane trong Word/Excel/PowerPoint/WPS | Người dùng cuối | `AxiomOffice.dll` (COM add-in) |
| **MCP server** — Claude Desktop, Claude Code, agent khác | AI agent bên ngoài | `AxiomOffice.Core.exe mcp` (`AxiomOffice.Host.exe mcp` chuyển tiếp) |
| **HTTP API** trên localhost (`/cmd`, `/events`, ...) | Script, tích hợp riêng | bridge trong add-in |

Không cần quyền admin, không cần Python. Gói cài ~250 KB.

LibreOffice dùng **extension Python UNO** (`src\AxiomOffice.LibreOffice`) nói đúng giao thức bridge
trên (cùng tên lệnh `writer.*`/`et.*`/`wpp.*`, cùng session registry) — xem
[mục LibreOffice](#libreoffice). Hiện đã chạy được trên Windows (giai đoạn L1); Linux là giai đoạn L2
(xem `LibreOffice_arch.md`).

> Trước đây dự án tên **WPS AI Bridge** (`WpsAiBridge`). Cài bản mới sẽ tự gỡ đăng ký của bản
> cũ và chuyển cấu hình (port, token, cài đặt AI) từ `HKCU\Software\WpsAiBridge` sang
> `HKCU\Software\AxiomOffice`. Thư mục log cũ `%LOCALAPPDATA%\WpsAiBridge` có thể xoá.

## Mục lục

- [Tình trạng dự án](STATUS.md) — đã làm được gì, chưa làm được gì
- [Kiến trúc](#kiến-trúc)
- [Cài đặt cho người dùng](#cài-đặt-cho-người-dùng)
- [Ask AI (agent trong app)](#ask-ai-agent-trong-app)
- [MCP server](#mcp-server)
- [HTTP API](#http-api)
- [Cấu hình](#cấu-hình)
- [Phát triển](#phát-triển)
- [Microsoft Office và WPS: lưu ý riêng](#microsoft-office-và-wps-lưu-ý-riêng)
- [LibreOffice](#libreoffice)
- [Troubleshooting](#troubleshooting)
- [Cấu trúc project](#cấu-trúc-project)

## Kiến trúc

> Tài liệu thiết kế đầy đủ, gồm High Level Design và Low Level Design, có sơ đồ mermaid: xem
> [ARCHITECTURE.MD](ARCHITECTURE.MD).

```
 Người dùng ── Ask AI (task pane) ──┐
                                    ▼
                   ┌─────────────────────────────────────┐        COM
 Script ── HTTP ──►│ AxiomOffice.dll (COM add-in in-proc) │ ─────────────► tài liệu đang mở
   (localhost)     │  bridge · ribbon · task pane · agent │     Word / Excel / PowerPoint
                   └─────────────────────────────────────┘     hoặc WPS Writer / ET / WPP
                                    ▲ HTTP (làn live)
 MCP client ── stdio ──► AxiomOffice.Core.exe mcp
                                    └── làn file: đọc/ghi .docx .xlsx .pptx .csv/.xls trực tiếp (không cần app)
```

| Thành phần | Vai trò |
|---|---|
| `AxiomOffice.dll` | COM add-in (`IDTExtensibility2`) nạp vào Word/Excel/PowerPoint và WPS: mở HTTP bridge trong process của app, thêm tab ribbon **Axiom Office**, task pane Ask AI và AI agent |
| `AxiomOffice.Host.exe` | `mcp [all\|word\|excel\|ppt]`: chuyển tiếp sang `AxiomOffice.Core.exe mcp` (giữ cho cấu hình MCP client đã có) · `wps\|et\|wpp\|word\|excel\|ppt`: companion tự tạo app qua COM automation và mở bridge (khi add-in không nạp được) · `commands`: danh sách lệnh bridge · `llm-test`: thử cấu hình AI |
| `AxiomOffice.Core.exe` | **Agent Core** viết bằng Go ([core-go/](core-go/README.md), theo [New_arch.md](New_arch.md)): process riêng chạy agent cho mọi app, một bản cho mỗi người dùng, chỉ nghe `127.0.0.1:47840`; add-in khởi động khi cần và tìm qua `%LOCALAPPDATA%\AxiomOffice\core.json`. Hiện có: vòng lặp agent (OpenAI-compatible + Anthropic), hội thoại liên tục theo tài liệu, audit tool call, SSE `/v1/runs/{id}/events`, hủy, trần thời gian/token, **skills** (`load_skill`, `read_skill_file`, `/v1/skills`); memory + xác nhận + MCP client ở các giai đoạn sau |

Mỗi app có port riêng; WPS và Microsoft Office dùng hai dải khác nhau nên chạy song song
được (đổi qua registry, xem [Cấu hình](#cấu-hình)):

| App | WPS (`Port`) | Microsoft Office (`PortOffice`) |
|---|---|---|
| Writer / Word | 47821 | 47831 |
| Spreadsheets / Excel | 47822 | 47832 |
| Presentation / PowerPoint | 47823 | 47833 |

**LibreOffice** dùng dải port thứ ba, 47851–47853, do extension Python UNO mở (cùng giao thức, cùng
session registry — xem [mục LibreOffice](#libreoffice)); ba app chung một tiến trình `soffice` nên một
process ghi ba file session. Thiết kế đầy đủ (Linux, sidebar, Core đa nền tảng, giai đoạn L0–L4):
[LibreOffice_arch.md](LibreOffice_arch.md).

## Cài đặt cho người dùng

**Yêu cầu:** Windows 10/11 x64 (có sẵn .NET Framework 4.8), Microsoft Office **x64** và/hoặc
WPS Office **x64**. Không cần admin, Python hay Visual Studio.

Máy đích **không phải cài gì thêm**: Agent Core (Go), skills và add-in đều nằm trong zip. Điều kiện duy
nhất dễ bỏ sót là **bitness của Office/WPS** — add-in đóng gói x64 nên Office 32-bit cài xong vẫn không
thấy tab:

```powershell
(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Office\ClickToRun\Configuration').Platform   # phải trả về x64
```

**Tạo gói** (trên máy có mã nguồn — Windows không có artifact CI như bên Linux):

```powershell
powershell -ExecutionPolicy Bypass -File scripts\package.ps1    # -> dist\AxiomOffice-<ver>-<ngày>-<commit>.zip
```

`package.ps1` chạy `build.ps1` trước (`-NoBuild` để dùng lại `bin\Release`), cần .NET Framework có sẵn để
gọi `csc`. Máy build **không có Go 1.26+** thì gói vẫn ra nhưng **không kèm `AxiomOffice.Core.exe`** (có
cảnh báo) — add-in vẫn chạy, chỉ mất phần agent riêng. Đang mở Word/Excel/WPS thì build **dừng lại và
liệt kê tiến trình đang giữ DLL** — lưu tài liệu rồi đóng app; chỉ dùng `-Kill` khi chấp nhận để script tự
tắt các app đó (Agent Core là tiến trình của chính dự án nên luôn được tắt trước một cách an toàn).

1. Giải nén `AxiomOffice-<version>-....zip` vào một chỗ cố
   định, ví dụ `C:\Tools\AxiomOffice`. Add-in chạy thẳng từ thư mục này; chuyển chỗ thì cài lại.
2. Đóng Word, Excel, PowerPoint, WPS.
3. Nhấp đúp **`install.cmd`**: tự gỡ nhãn "tải từ Internet" của file, đăng ký add-in (HKCU),
   whitelist cho WPS và in sẵn cấu hình MCP với đúng đường dẫn.
4. Mở Word/Excel/PowerPoint hoặc WPS: có tab **Axiom Office** trên ribbon.
5. **Thiết lập…**: wizard 5 bước tự mở khi pane chưa có cấu hình (xem mục dưới). Cài đặt nâng cao
   nằm trong bước 4 của wizard.

Gỡ: nhấp đúp **`uninstall.cmd`** (giữ cấu hình AI và token); `uninstall.cmd -Purge` xoá cả cấu
hình, token và log. Hướng dẫn chi tiết cho người nhận gói: `scripts/dist/HUONG-DAN-CAI-DAT.txt`.
Khoá API mã hoá bằng DPAPI theo tài khoản Windows nên **mỗi máy nhập khoá riêng**, không chép registry
sang máy khác được. Endpoint nội bộ (vd `http://localhost:20128/v1`) thì máy mới phải có đường tới đó
trước, nếu không bước **Kiểm tra kết nối** báo lỗi mạng chứ không phải lỗi khoá.

### Wizard thiết lập (người dùng không chuyên)

Cùng một bố cục và cùng câu chữ trên Windows (ribbon **Thiết lập…**, link **Thiết lập** ở header pane)
và LibreOffice trên Linux (menu **Axiom Office → Thiết lập…**):

| Bước | Việc người dùng làm |
|---|---|
| 1. Chào mừng | biết mình sắp làm gì, mất khoảng một phút |
| 2. Kiểm tra máy | từng dòng có ✓ / ! / ✗ kèm nút **Sửa** (khởi động lại Agent Core, tạo khoá bảo vệ mới) |
| 3. Kết nối máy chủ AI | chọn máy chủ công ty / OpenAI / Anthropic / Gemini, **tải danh sách model** để chọn, **Kiểm tra kết nối** báo lỗi bằng tiếng Việt dễ hiểu kèm gợi ý sửa |
| 4. Tính năng | "Nhớ những điều tôi đã dặn", "Tự rút ra điều đáng nhớ", "Cho AI xem ảnh trang tài liệu" + link **Tuỳ chọn nâng cao…** (dialog Cài đặt cũ: cổng Core, hạn giờ, model ghi nhớ) |
| 5. Hoàn tất | tóm tắt cấu hình + **Thử ngay**: chạy một lượt thật qua Agent Core trên tài liệu đang mở |

Câu chữ và preset (nhà cung cấp, endpoint, nơi lấy khoá, model gợi ý) nằm ở [`catalog/setup.json`](catalog/setup.json)
— nguồn duy nhất, sinh ra cả ba bản (C# cho add-in + Core, Python cho LibreOffice, Go cho Core bản Go).

Kiểm tra nhanh: `http://127.0.0.1:47831/health` (Word) hoặc `47821` (WPS Writer); log ở
`%LOCALAPPDATA%\AxiomOffice\bridge.log`.

## Ask AI (agent trong app)

Tab **Axiom Office** → **Ask AI...** mở task pane dock bên phải (`ICustomTaskPaneConsumer`,
cùng API cho Office và WPS; host không hỗ trợ thì mở cửa sổ nổi).

1. Gõ yêu cầu (*"Soạn đơn xin việc vị trí kế toán"*, *"Tạo bảng điểm 5 học sinh, có cột trung
   bình"*, *"Tạo 5 slide giới thiệu công ty"*) hoặc bấm một gợi ý → **Ask**. Enter gửi,
   Shift+Enter xuống dòng.
2. Agent gọi LLM với **tool-calling** và tự thực thi lệnh `writer.*` / `et.*` / `wpp.*` lên tài
   liệu đang mở — chỉ các lệnh có dấu ✓ ở cột **Ask AI** trong [bảng lệnh](#danh-sách-lệnh-post-cmd);
   lệnh khác (vd `writer.closeAll`, `ai.ask`) bị từ chối. Mỗi thao tác hiện thành một dòng: dấu tick
   xanh / x đỏ, nhãn tiếng Việt và mã action. Lượt chạy đi qua **Agent Core** (`AxiomOffice.Core.exe`)
   khi có; Core không chạy được (không khởi động/không nhận lượt chạy) thì pane tự chạy agent trong
   add-in và ghi chú "chế độ cơ bản" ở dòng trạng thái. Core đã nhận lượt chạy thì **không bao giờ**
   chạy lại in-process (tránh sửa tài liệu hai lần) — lỗi của Core hiện thẳng cho người dùng.
3. Xong thì AI trả lời ngắn. Với Word: mỗi thao tác của AI = **1 bước Ctrl+Z**; link **Chèn trả
   lời** chèn câu trả lời vào tài liệu.

Hành vi:

- **Không giới hạn số vòng** gọi model/tool. Lượt chạy dừng khi AI trả lời xong, khi bấm
  **Dừng** (hủy ngay request đang chờ) hoặc khi chạm trần **5 phút** (`LlmClient.AgentTimeoutMs`);
  mỗi request LLM tối đa 60s.
- Agent **không tự lưu / lưu thành / xuất PDF** nếu người dùng không yêu cầu: thay đổi đã hiện
  trong tài liệu đang mở, người dùng tự quyết khi nào lưu và lưu ở đâu.
- **Nhớ theo tài liệu**: các lượt trong cùng một tài liệu nối thành một cuộc trò chuyện (lưu ở
  Core, `%LOCALAPPDATA%\AxiomOffice\core\core.db`), nên "làm tiếp" hiểu ngữ cảnh lượt trước và mở
  lại tài liệu sau vẫn tiếp tục đúng mạch. Link **Trò chuyện mới** ở header (cạnh Cài đặt) để bắt đầu
  lại từ đầu; agent vẫn tự đọc tài liệu khi cần.
- **Hoàn tác lượt này** (Word): sau mỗi lượt có sửa tài liệu, link ở footer hoàn tác toàn bộ thao tác
  của lượt đó (mỗi thao tác AI = 1 bước Ctrl+Z). Excel/PowerPoint không hoàn tác được thay đổi qua COM.
- Lỗi hiện thành thẻ có **Thử lại** / **Mở Cài đặt**. Chưa cấu hình endpoint/model thì ô nhập bị
  khoá kèm hướng dẫn.
- Provider: OpenAI-compatible, Anthropic và **Google Gemini** (chọn trong Cài đặt: endpoint
  `https://generativelanguage.googleapis.com/v1beta/openai`, API key lấy ở aistudio.google.com, model
  vd `gemini-2.5-flash`). Provider không hỗ trợ tools thì tự chuyển sang chat thường. Provider quá tải
  hoặc lỗi tạm thời (429/5xx) thì tự thử lại tối đa 3 lần (1s → 2s → 4s). Phần suy nghĩ
  `<thought>…</thought>` của model không hiện trong câu trả lời. Đổi provider/model
  trong Cài đặt có hiệu lực từ lượt chạy tiếp theo, không cần khởi động lại.
- Transcript mỗi lượt nằm trong `bridge.log` (`AskAiPane: prompt=` / `AskAiPane progress:` /
  `AskAiPane: ok ... N tool calls, M rounds` / `AskAiPane failed`), ghi ngay cả khi pane đã đóng.
- Agent bên ngoài gọi cùng logic qua lệnh bridge `ai.ask` (chạy qua Agent Core khi có, response thêm
  `viaCore`; không có người bấm xác nhận nên lệnh cần xác nhận bị từ chối ngay).

### Kỹ năng (skills)

Agent Core nạp **skill theo chuẩn Agent Skills** (thư mục chứa `SKILL.md` có frontmatter `name` +
`description`; mở rộng tuỳ chọn `apps: [wps|et|wpp]`). Chỉ mục `name: description` của skill hợp app nằm
sẵn trong prompt; model tự gọi `load_skill` khi yêu cầu khớp (pane hiện dòng **Dùng kỹ năng: …**) và
`read_skill_file` để đọc `references/`, `tokens.json`… Sửa nhỏ ("in đậm dòng này") không nạp skill.

| Skill dựng sẵn | App | Dùng khi |
|---|---|---|
| `thiet-ke-van-phong` | mọi app | tạo mới / làm đẹp tài liệu, bảng, slide |
| `trinh-bay-chuyen-nghiep` | PowerPoint | thiết kế slide: lưới, cỡ chữ, mật độ |
| `the-thuc-van-ban` | Word | trình bày báo cáo, tờ trình: heading, bảng, căn lề |
| `bao-cao-du-lieu` | Excel | bảng số liệu: number format, công thức, hàng tổng |
| `bao-cao-thang` | PowerPoint | slide báo cáo tháng/quý |
| `bang-diem` | Excel | bảng điểm, điểm trung bình, xếp loại |
| `mo-hinh-nhieu-sheet` | Excel | mô hình nhiều sheet: giả định, dự báo/kịch bản, độ nhạy, dashboard, sheet kiểm tra (tài chính, danh mục dự án, tồn kho) |
| `van-ban-hanh-chinh` | Word | công văn, quyết định, tờ trình theo NĐ 30/2020 |

`skills/_design/tokens.json` là **token thiết kế dùng chung** (màu theo ngữ nghĩa, thang chữ, number
format); skill dùng đúng giá trị trong đó. Sau khi tạo, agent tự **soát cấu trúc** bằng
`wpp.checkLayout` / `et.checkRange` / `writer.checkTables` (tràn chữ, shape chồng, dữ liệu lạc ô…).

Nguồn skill (trùng tên thì nguồn sau thắng): `skills\` cạnh `AxiomOffice.Core.exe` → thư mục trong
`SkillDirs` (HKCU, phân cách `;`) → `%LOCALAPPDATA%\AxiomOffice\skills`. Thêm/sửa skill có hiệu lực sau
~2 giây (theo dõi thư mục) hoặc `POST /v1/skills/reload`; `GET /v1/skills?app=et` liệt kê skill và lỗi
nạp (skill sai frontmatter bị bỏ qua, không làm hỏng Core). Skill **không chạy script**; chỉ dùng skill từ
nguồn tin cậy.

### Ghi nhớ dài hạn (memory)

Agent Core **nhớ qua các phiên** (học từ mem0 2.2.1, tự làm bằng C#, dữ liệu nằm trong
`%LOCALAPPDATA%\AxiomOffice\core\core.db` trên máy):

- **Phạm vi**: `user` (đúng cho mọi tài liệu: bạn là ai, cơ quan, người ký, thói quen trình bày) và
  `document` (tiến độ/ghi chú của riêng một file đã lưu).
- **Ghi**: bạn tự thêm trong **Cài đặt → Quản lý ghi nhớ…**; AI gọi `remember` khi bạn nói điều đáng nhớ;
  và **tự trích xuất sau mỗi lượt** (chạy nền, một lệnh gọi LLM, **chỉ thêm mới**). Lệnh thao tác thuần
  ("in đậm dòng này") không tốn lệnh gọi trích xuất. Pane hiện dòng **Đã ghi nhớ: …** kèm link **Xoá**.
- **Thay đổi** (vd "tôi đã lên phó giám đốc") được ghi thành memory **mới** nói rõ chuyển từ gì sang gì và
  liên kết bản cũ; AI không bao giờ tự sửa/xoá memory — chỉ bạn sửa, ghim, đặt hạn dùng, xoá (khôi phục
  được 30 ngày), xem lịch sử, hoặc "Xoá toàn bộ".
- **Đọc**: memory liên quan tới yêu cầu (tìm không dấu, chấm điểm cộng dồn keyword + thực thể + embedding
  nếu có) và memory ghim được đưa vào prompt; hết hạn thì ẩn. Không lưu mật khẩu, số thẻ/CCCD, API key.
- Tắt trong Cài đặt: **Ghi nhớ dài hạn** (không đọc/ghi) hoặc **Tự ghi nhớ sau mỗi lượt** (chỉ ghi khi bạn
  hoặc AI chủ động). API: `GET/POST /v1/memory`, `PATCH/DELETE /v1/memory/{id}`,
  `POST /v1/memory/{id}/restore`, `GET /v1/memory/{id}/history`.

### Xác nhận, công cụ MCP và QA thị giác

- **Hỏi trước thao tác rủi ro**: khi AI định lưu/xuất file dù bạn không yêu cầu, ghi đè file đã có, xoá
  slide, thay thế toàn bộ trong tài liệu dài (> 20.000 ký tự) hoặc dùng công cụ MCP ngoài, pane hiện thẻ
  **Đồng ý / Từ chối**; không trả lời trong 120 giây (`ConfirmTimeoutSeconds`) = từ chối. Mọi thao tác ghi
  vào nhật ký (`GET /v1/audit`).
- **Công cụ MCP** cho agent: server dựng sẵn `office` (làn file của `AxiomOffice.Host.exe`: đọc/ghi
  docx/xlsx/pptx trên đĩa không cần mở app; sửa file đã có thì hỏi trước) và server bạn thêm trong
  `%LOCALAPPDATA%\AxiomOffice\mcp.json`:

  ```json
  {"mcpServers": {"tim-kiem": {"command": "python", "args": ["server.py"]},
                  "noi-bo": {"url": "http://127.0.0.1:9000/mcp", "trusted": true},
                  "office": {"disabled": true}}}
  ```

  Tool có tên `mcp__<server>__<tool>`; server không `"trusted": true` thì mỗi lần gọi đều hỏi. Xem
  trạng thái: `GET /v1/mcp`.
- **QA thị giác** (tắt mặc định): bật **Cho AI xem ảnh chụp cửa sổ** trong Cài đặt để agent chụp cửa sổ
  app (`app.screenshot`) và tự soát bố cục bằng mắt sau khi làm slide/bảng — tốn thêm token, cần model
  đọc được ảnh.

Giao diện vẽ bằng GDI+ theo design tokens trong `PaneTheme` (`src/AxiomOffice/Ai/PaneControls.cs`):
tương phản chữ ≥ 4.5:1, focus ring khi dùng bàn phím, scale theo DPI. Bubble nhận Tab/Ctrl+C
và có menu chuột phải **Sao chép**.

## MCP server

MCP server là **lệnh con `mcp` của Agent Core** (`AxiomOffice.Core.exe mcp`), một bản duy nhất dùng
chung Windows lẫn Linux:

```json
{
  "mcpServers": {
    "office": {
      "command": "C:\\Tools\\AxiomOffice\\src\\AxiomOffice\\bin\\Release\\AxiomOffice.Core.exe",
      "args": ["mcp"]
    }
  }
}
```

- `mcp` = 50 tool; muốn gọn thì `mcp word` (20), `mcp excel` (16), `mcp ppt` (16).
  `AxiomOffice.Core.exe mcp --list` in danh sách tool.
- `AxiomOffice.Host.exe mcp …` **vẫn chạy được** (người đã cấu hình từ trước không phải đổi gì): nó là
  cửa chuyển tiếp byte sang `AxiomOffice.Core.exe` nằm cạnh. Gói cài thiếu Core thì Host.exe báo rõ và
  thoát, chứ không còn bản MCP thứ hai để lệch khỏi bản Go.
- Giao thức MCP stdio (JSON-RPC 2.0, mỗi message một dòng): `initialize`, `tools/list`,
  `tools/call`, `ping`; protocol 2024-11-05 → 2025-11-25. Log: `MCP tool ... ok in Nms`.

| Nhóm | Tool |
|---|---|
| Word — file | `doc_profile`, `doc_get_text`, `doc_find_text`, `doc_extract_table`, `doc_create` |
| Word — live | `word_health`, `word_command`, `word_read_text`, `word_type_text`, `word_insert_styled_text`, `word_format_selection`, `word_heading`, `word_insert_table`, `word_insert_image`, `word_insert_hyperlink`, `word_replace_all`, `word_export_pdf`, `word_undo`, `word_save` |
| Excel — file | `excel_profile`, `excel_read`, `excel_write`, `excel_create`, `excel_convert` (csv), `excel_create_sheet`, `excel_copy_sheet`, `excel_rename_sheet`, `excel_delete_sheet`, `excel_format_range`, `excel_create_table` |
| Excel — live | `wps_health`, `wps_live_command`, `wps_live_read_range`, `wps_live_write_range` |
| PowerPoint — file | `ppt_profile`, `ppt_get_text`, `ppt_create`, `ppt_add_slide_file` |
| PowerPoint — live | `ppt_health`, `ppt_command`, `ppt_list_slides`, `ppt_add_slide`, `ppt_add_text`, `ppt_add_image`, `ppt_add_table`, `ppt_set_notes`, `ppt_delete_slide`, `ppt_export_pdf`, `ppt_save` |
| Chung | `office_sessions` — mọi bridge đang sống (Office + WPS): `app`, `family`, `port`, `host`, `document`, `healthy` |

**Làn live** gửi lệnh tới bridge trong app đang mở (tham số `app`: `word`/`excel`/`ppt` cho
Office, `wps`/`et`/`wpp` cho WPS, hoặc `port` lấy từ `office_sessions`).

**Làn file** đọc/ghi thẳng OOXML (ZipArchive + XML, không thư viện ngoài), không cần app mở.
Khi sửa file chỉ phần XML liên quan được ghi lại, nên **chart, ảnh, pivot, macro của file gốc
được giữ nguyên**; mọi lần ghi dùng file tạm rồi thay thế (atomic). `.xls` (BIFF8) **chỉ đọc**, và đọc
**thẳng** — không cần Excel, WPS hay LibreOffice trên máy (`internal/cfb` mở thùng OLE2, `biff.go` đọc
bản ghi BIFF8). Chỉ còn dùng COM Excel/WPS ở bản C# khi gói cài **không kèm** Agent Core. Không có truy
vấn SQL (`excel_query` của bản Python cũ).

### Trên Linux (`AxiomOffice.Core mcp`)

MCP server bản Linux là **subcommand của chính Agent Core** — gói Linux không có file MCP riêng
(`scripts/linux/package.sh` đóng gói, `install.sh` cài vào `~/.local/share/axiom-office/core`), **cùng 50
tool và cùng hợp đồng với `AxiomOffice.Host.exe mcp`**:

```json
{
  "mcpServers": {
    "office": { "command": "/home/<bạn>/.local/share/axiom-office/core/AxiomOffice.Core", "args": ["mcp", "all"] }
  }
}
```

- Chạy từ mã nguồn: `cd core-go && go build -o axiom-core ./cmd/axiom-core` rồi `./axiom-core mcp all`
  (`mcp word|excel|ppt`, `--list` để xem tool).
- Core dùng **chính binary này** làm server built-in `office` (hàm `officeMcpHost` trong
  `cmd/axiom-core/main.go`): trên Windows là `AxiomOffice.Host.exe`, trên Linux là binary Core — nên
  `mcp.json`, agent và add-in không phải đổi gì.
- `office_sessions` đọc session ở `$XDG_RUNTIME_DIR/axiom-office/sessions`, còn token và port
  (`PortLibreOffice`, mặc định 47851) lấy từ `~/.config/axiom-office/config.json` — **cùng chỗ** với
  Agent Core và extension, nên tool live nói chuyện được với LibreOffice đang mở ngay khi cài xong.
- `.xls` đọc thẳng bằng Go (BIFF8 trong thùng CFB), không cần cài thêm gì. Chỉ khi gặp `.xls` **cũ hơn
  BIFF8** (Excel 5 trở về trước) hoặc thùng hồng thì mới nhờ `soffice --headless --convert-to xlsx`
  (**profile riêng**, không đụng phiên LibreOffice đang mở); máy không có LibreOffice thì báo rõ cả hai
  đường đã thử (đặt `AXIOM_SOFFICE` nếu `soffice` không nằm trong PATH, hoặc `AXIOM_SOFFICE=none` để
  coi như máy không có LibreOffice).
- Khác bản Windows: không có lệnh `commands` (`AxiomOffice.Host.exe commands --json|--markdown` sinh bảng
  README) — danh sách lệnh cho mô tả tool `*_command` nằm ở nguồn chung `catalog/live-commands.json`,
  sinh bằng `scripts/generate_mcp_commands.py` (nhúng vào Go qua `internal/mcpserver/livecommands_gen.go`)
  và được `tests/lo/test_extension.py` so lại với registry của extension.
- Test: `python tests/mcp-host/test_mcp_portable.py <exe> [--live]` (không cần thư viện ngoài, chạy được
  cả hai hệ điều hành: giao thức, 50 tool, các tool file trên file thật, `.xls` qua LibreOffice, và tool
  live khi có app đang mở). Bộ parity đầy đủ `tests/mcp-host/test_mcp_host.py` chạy cho bản này bằng
  `AXIOM_MCP_CMD="<binary> mcp all" AXIOM_MCP_NO_CATALOG=1` (cần `mcp`/`python-docx`/`openpyxl`/`python-pptx`).
  `tests/mcp-host/oracle_diff.py` chạy cùng kịch bản trên bản Go **và** bản C# rồi so từng bước.

## HTTP API

Mọi endpoint nghe trên `127.0.0.1`. Bridge **từ chối request có header `Origin`** (chặn gọi từ
trình duyệt); mọi endpoint trừ `/health` cần header `X-Auth-Token` (token tự sinh khi cài, lưu ở
`HKCU\Software\AxiomOffice\Token`); `POST /cmd` bắt buộc `Content-Type: application/json`.

| Endpoint | Mô tả |
|---|---|
| `GET /health` | Không cần token: `{"ok":true,"result":{"app":"wps","pid":1234,"port":47831,"version":"1.0.0","log":"..."}}` |
| `GET /config` | Provider, endpoint, model (API key đã che) |
| `GET /commands` | Danh sách lệnh của DLL đang chạy (tên, loại app, cờ cho agent, tham số) + `version` — Agent Core dùng để dựng tool cho agent đúng phiên bản |
| `GET /session` | Thông tin bridge + tài liệu đang mở (đọc ngay lúc gọi) |
| `GET /events` | Server-Sent Events: vùng chọn, tài liệu đổi, ping |
| `POST /cmd` | `{"action": "...", "params": {...}}` → `{"ok":true,"result":{...}}` hoặc `{"ok":false,"error":"..."}` |

`app` trong kết quả là loại logic: `wps` = Writer/Word, `et` = Spreadsheets/Excel, `wpp` =
Presentation/PowerPoint; `family` = `office` | `wps`.

### Ví dụ

```powershell
$headers = @{ "X-Auth-Token" = (Get-ItemProperty "HKCU:\Software\AxiomOffice").Token }
$base = "http://127.0.0.1:47831"   # Word; WPS Writer: 47821
# Gửi body dạng byte UTF-8: PowerShell 5.1 mã hoá chuỗi -Body bằng ISO-8859-1 làm hỏng tiếng Việt.
function Cmd($body) { Invoke-RestMethod -Method Post -Uri "$base/cmd" -Headers $headers -ContentType "application/json; charset=utf-8" -Body ([Text.Encoding]::UTF8.GetBytes($body)) }

Cmd '{"action":"writer.newDocument"}'
Cmd '{"action":"writer.heading","params":{"level":1,"text":"Báo cáo tháng 9"}}'
Cmd '{"action":"writer.insertTable","params":{"values":[["Tên","Điểm"],["An",9.5]],"style":"Table Grid"}}'
Cmd '{"action":"writer.saveAs","params":{"path":"C:\\temp\\bao-cao.docx"}}'
```

```python
import json, urllib.request, winreg

with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\AxiomOffice") as key:
    token = winreg.QueryValueEx(key, "Token")[0]
body = json.dumps({"action": "et.writeRange",
                   "params": {"range": "A1", "values": [["Tên", "Điểm"], ["An", 9.5]]}}).encode()
request = urllib.request.Request("http://127.0.0.1:47832/cmd", data=body, method="POST",
                                 headers={"Content-Type": "application/json", "X-Auth-Token": token})
print(json.load(urllib.request.urlopen(request)))
```

### Danh sách lệnh (`POST /cmd`)

`values` (bảng, vùng ô) luôn là **mảng 2 chiều theo dòng**, ví dụ `[["Tên","Điểm"],["An",9.5]]`.
Cũng nhận chuỗi JSON và dạng bọc `{"item": ...}` mà một số model sinh ra (mỗi lớp bọc = một cấp
mảng: `{"item":{"item":["Tổng",17.5]}}` là một dòng; dòng bị bọc thừa một lớp cũng được gỡ). Tham
số số nhận cả `{"item": 6}`/`[6]`/`"6"`. `writer.insertTable`: dữ liệu bảng lỡ đặt vào `rows` được
dùng như `values`; bảng chèn ở cuối vùng chọn (trong bảng thì chèn sau bảng), không thay nội dung
đang chọn. `writer.replaceAll`: `\n` trong `find`/`replace` = ngắt đoạn (`^p`), `\v` = xuống dòng
thủ công. Sai dạng hoặc thiếu thì lệnh trả lỗi kèm ví
dụ (không âm thầm bỏ qua); tham số số/true-false sai kiểu cũng báo rõ tên tham số. Tham số `slide` của
lệnh `wpp.*` bỏ trống = slide cuối. `?` = tham số tuỳ chọn. Cột **Ask AI**: ✓ = agent trong
task pane (và `ai.ask`) được dùng lệnh này; HTTP API và MCP gọi được mọi lệnh.

Bảng dưới sinh từ registry lệnh bằng `AxiomOffice.Host.exe commands --markdown` (`--json` cho
công cụ); lệnh khai báo cạnh handler trong `src/AxiomOffice/Bridge/CommandDispatcher.*.cs`.

| Action | Params | Mô tả | Ask AI |
|---|---|---|:-:|
| `app.info` | — | Tên/version app, tài liệu đang mở, `state` (tài liệu, cửa sổ, visible) |  |
| `ai.ask` | `prompt` | Chạy AI agent trên tài liệu đang mở; trả `reply`, `transcript`, `seconds`, `rounds` |  |
| `ui.askpane` | — | Mở task pane Ask AI |  |
| `app.screenshot` | `maxWidth?` | Ảnh chụp cửa sổ app (PNG base64, thu nhỏ theo `maxWidth`, mặc định 1280) |  |
| `writer.newDocument` | — | Tạo tài liệu mới |  |
| `writer.open` | `path` | Mở .docx/.doc |  |
| `writer.getText` | `maxChars?` | Đọc toàn bộ text | ✓ |
| `writer.selection` | — | Text + vị trí đang chọn | ✓ |
| `writer.typeText` | `text` | Gõ tại con trỏ | ✓ |
| `writer.appendText` | `text` | Nối vào cuối tài liệu | ✓ |
| `writer.insertStyledText` | `text`, `bold?`, `italic?`, `underline?`, `size?`, `color?`, `font?` | Chèn text có định dạng tại con trỏ (`color` dạng `#RRGGBB`) | ✓ |
| `writer.heading` | `text?`, `level?`, `break?` | Heading 1-9 (`level`, mặc định 1) + tự xuống dòng (`break`, mặc định true) | ✓ |
| `writer.formatSelection` | `bold?`, `italic?`, `underline?`, `size?`, `color?`, `font?`, `alignment?` | Định dạng vùng chọn | ✓ |
| `writer.setParagraphAlignment` | `alignment` | Căn đoạn: left/center/right/justify | ✓ |
| `writer.insertTable` | `rows?`, `cols?`, `values?`, `style?` | Chèn bảng; `rows`/`cols` tự suy ra/nới theo `values` | ✓ |
| `writer.formatTable` | `table?`, `style?`, `font?`, `size?`, `color?`, `headerFill?`, `headerColor?`, `headerBold?`, `bandFill?`, `borderColor?`, `borders?`, `alignment?`, `autoFit?` | Định dạng bảng có sẵn (mặc định: bảng tại con trỏ, không có thì bảng cuối); phần host không hỗ trợ trả về trong `skipped` | ✓ |
| `writer.checkTables` | — | QA cấu trúc (chỉ đọc): số dòng/cột, ô trống, ô tiêu đề lẫn đoạn văn | ✓ |
| `writer.insertPageBreak` | — | Ngắt trang | ✓ |
| `writer.insertImage` | `path`, `width?`, `height?` | Chèn ảnh tại con trỏ (kích thước theo point) | ✓ |
| `writer.insertHyperlink` | `url`, `text?` | Chèn liên kết | ✓ |
| `writer.replaceAll` | `find`, `replace?` | Tìm và thay toàn bộ | ✓ |
| `writer.undo` | `count?` | Hoàn tác (mỗi thao tác AI = 1 bước) | ✓ |
| `writer.exportPdf` | `path` | Xuất PDF | ✓ |
| `writer.save` | — | Lưu | ✓ |
| `writer.saveAs` | `path` | Lưu thành file mới | ✓ |
| `writer.closeAll` | — | **Đóng mọi tài liệu, không lưu** |  |
| `et.newWorkbook` | — | Tạo workbook mới (agent không dùng: làm trên sổ đang mở; Excel trống thì `listSheets`/`writeRange`/`formatRange` tự tạo sổ) | |
| `et.open` | `path` | Mở .xlsx/.xls/.csv |  |
| `et.listSheets` | — | Danh sách sheet + sheet đang active | ✓ |
| `et.activateSheet` | `sheet` | Chuyển sheet | ✓ |
| `et.readRange` | `range`, `sheet?` | Đọc vùng, ví dụ `A1:C10` | ✓ |
| `et.writeRange` | `range`, `values`, `sheet?` | Ghi vùng bắt đầu từ ô trên-trái `range` | ✓ |
| `et.formatRange` | `range`, `bold?`, `italic?`, `fontSize?`, `fontColor?`, `fillColor?`, `numFmt?`, `horizontal?`, `wrap?`, `sheet?` | Định dạng vùng (màu dạng `#RRGGBB`, `horizontal` left/center/right) | ✓ |
| `et.checkRange` | `range?`, `sheet?` | QA cấu trúc (chỉ đọc): tiêu đề trống, kiểu lẫn lộn, số dạng chữ, số lẻ chưa có number format, ô lỗi, dữ liệu lạc ngoài bảng | ✓ |
| `et.undo` | `count?` | Hoàn tác | ✓ |
| `et.exportPdf` | `path` | Xuất PDF | ✓ |
| `et.save` | — | Lưu | ✓ |
| `et.saveAs` | `path` | Lưu thành file mới | ✓ |
| `wpp.newPresentation` | — | Tạo bản trình chiếu mới |  |
| `wpp.open` | `path` | Mở .pptx |  |
| `wpp.listSlides` | — | Số slide + text từng slide | ✓ |
| `wpp.addSlide` | `layout?` | Thêm slide cuối; `layout` mặc định 12 = trống (1 = tiêu đề, 2 = tiêu đề + nội dung, 11 = chỉ tiêu đề) | ✓ |
| `wpp.addText` | `text`, `slide?`, `left?`, `top?`, `width?`, `height?`, `fontSize?`, `bold?`, `color?`, `align?` | Textbox có định dạng (`color` dạng `#RRGGBB`, `align` left/center/right) | ✓ |
| `wpp.addTextBox` | `text`, `slide?`, `left?`, `top?`, `width?`, `height?` | Textbox |  |
| `wpp.addImage` | `path`, `slide?`, `left?`, `top?`, `width?`, `height?` | Chèn ảnh (kích thước gốc nếu bỏ trống `width`/`height`) | ✓ |
| `wpp.addTable` | `rows?`, `cols?`, `values?`, `slide?`, `left?`, `top?`, `width?`, `height?` | Bảng; `rows`/`cols` tự suy ra/nới theo `values` | ✓ |
| `wpp.setNotes` | `text`, `slide?` | Ghi chú thuyết trình | ✓ |
| `wpp.deleteSlide` | `slide?` | Xoá slide (mặc định slide cuối) | ✓ |
| `wpp.checkLayout` | `slide?` | QA cấu trúc (chỉ đọc): chữ tràn khung, shape ra ngoài slide, shape chồng nhau, chữ < 12pt, slide quá nhiều chữ | ✓ |
| `wpp.exportPdf` | `path` | Xuất PDF | ✓ |
| `wpp.save` | — | Lưu | ✓ |
| `wpp.saveAs` | `path` | Lưu thành file mới | ✓ |

Lệnh `writer.*` tự kích hoạt tài liệu có cửa sổ hiển thị nếu `ActiveDocument` là tài liệu ẩn.
App đang bận (dialog mở, đang gõ trong ô Excel) thì bridge tự thử lại lỗi COM "busy" tối đa 10 lần
(~5s) trước khi trả lỗi.

### `GET /session` và session registry

```json
{"ok":true,"result":{"pid":5128,"app":"wps","family":"office","port":47831,"host":"WINWORD.EXE",
 "version":"1.0.0","started":"2026-09-28T21:25:42.965Z","lastSeen":"...","lastSeenEpoch":1790630759.1,
 "document":"bao-cao.docx","documentPath":"C:\\...\\bao-cao.docx","sessionFile":"..."}}
```

Mỗi bridge (add-in lẫn companion) ghi `%LOCALAPPDATA%\AxiomOffice\sessions\{pid}.json` (cùng nội
dung `/session`), heartbeat **25s** và xoá file khi app đóng. Tên tài liệu cho heartbeat đọc nền,
chờ tối đa 2s, bỏ qua lượt khi app đang bận nên heartbeat không bao giờ trễ. `office_sessions`
(MCP) bỏ các file có process đã chết, hoặc heartbeat quá 90s mà `/health` cũng không trả lời;
bridge đang chạy lệnh dài (heartbeat còn mới) vẫn được giữ với `healthy: false`.

### `GET /events` — Server-Sent Events

```powershell
curl.exe -N -H "X-Auth-Token: <token>" http://127.0.0.1:47831/events
```

```
event: hello
data: {"app":"wps","family":"office","port":47831,"pid":5128,"subscribers":1}

event: document
data: {"app":"wps","name":"bao-cao.docx","fullName":"C:\\...\\bao-cao.docx"}

event: selection
data: {"app":"wps","text":"Báo cáo","start":0,"end":7}

event: ping
data: {"time":"2026-09-28T21:26:27.055Z","subscribers":1}
```

| Event | Khi nào | Data |
|---|---|---|
| `hello` | ngay khi kết nối | `app`, `family`, `port`, `pid`, `subscribers` |
| `document` | tài liệu active đổi (gửi lại cho subscriber mới) | `app`, `name`, `fullName` (`null` khi không có tài liệu) |
| `selection` | vùng chọn đổi | Word/Writer: `text` (≤ 200 ký tự), `start`, `end` · Excel/ET: `sheet`, `address`, `text` = giá trị ô trên-trái · PowerPoint/WPP: `slide`, `type`, `shape`, `text`, `start`, `end` |
| `ping` | mỗi 15s | `time`, `subscribers` |

Cơ chế poll-diff 500ms (không dùng COM event sink, nên chạy giống nhau trên Office và WPS);
poller chỉ chạy khi có subscriber, tối đa **5 subscriber** mỗi bridge (thứ 6 nhận `503`). Mỗi kết
nối có thread riêng nên `/cmd` không bị chặn.

### Truy cập COM tuần tự (`ComGate`)

Bridge HTTP, agent của pane, heartbeat và poller SSE đều gọi object model của app qua
`src/AxiomOffice/Bridge/ComGate.cs`: tại một thời điểm chỉ một thread của Axiom Office ở trong
app. Lệnh và tool của agent chờ tới lượt; heartbeat/poller chỉ chạy khi cổng rảnh. Object model
Office không an toàn đa luồng — thiếu cổng này Word từng crash (AV trong `wwlib.dll`).

## Cấu hình

Windows: `HKCU\Software\AxiomOffice`. Linux: `~/.config/axiom-office/config.json` (`$XDG_CONFIG_HOME`) —
**cùng tên khoá**, giá trị số/bool ghi dạng JSON (`SkillDirs` là mảng hoặc chuỗi phân cách `;`). Extension
LibreOffice và Agent Core đọc chung file này, nên đổi trong pane **Cài đặt** là cả hai bên thấy ngay.
Thứ tự ưu tiên: `AXIOM_*` (biến môi trường) → HKCU/config.json → mặc định.

`HKCU\Software\AxiomOffice`:

| Value | Kiểu | Mặc định | Ý nghĩa |
|---|---|---|---|
| `Port` | DWORD | 47821 | Port gốc cho WPS (Spreadsheets +1, Presentation +2) |
| `PortOffice` | DWORD | 47831 | Port gốc cho Microsoft Office (Excel +1, PowerPoint +2) |
| `Enabled` | DWORD | 1 | 0 = tắt HTTP bridge |
| `Token` | String | tự sinh khi cài (32 hex) | Header `X-Auth-Token` cho mọi endpoint trừ `/health`; MCP server tự đọc |
| `LlmProvider` / `LlmEndpoint` / `LlmModel` | String | — | Cấu hình AI (đặt qua **Cài đặt** trong pane hoặc ribbon) |
| `LlmApiKey` | String | — | Mã hoá DPAPI theo tài khoản Windows; key plaintext cũ vẫn đọc được |
| `CorePort` | DWORD | 47840 | Port của Agent Core (bận thì tự thử 47840–47849) |
| `CoreEnabled` | DWORD | 1 | 0 = pane luôn chạy agent trong add-in, không khởi động Core |
| `MemoryEnabled` | DWORD | 1 | 0 = không đọc/ghi memory dài hạn (xem [Ghi nhớ](#ghi-nhớ-dài-hạn-memory)) |
| `MemoryAutoExtract` | DWORD | 1 | 0 = không tự trích xuất sau lượt (chỉ ghi khi bạn/AI chủ động) |
| `MemoryModel` | String | = `LlmModel` | Model dùng cho trích xuất memory (có thể chọn model rẻ hơn) |
| `EmbeddingModel` / `EmbeddingEndpoint` | String | — | Tuỳ chọn: tìm memory theo ngữ nghĩa qua `{endpoint}/embeddings`; bỏ trống = chỉ từ khoá |
| `LlmRequestTimeoutSeconds` | DWORD | 120 | Hết giờ mỗi request tới model (Core); tăng nếu model chậm |
| `SkillDirs` | String | — | Thư mục skill của tổ chức, phân cách `;` (xem [Kỹ năng](#kỹ-năng-skills)) |
| `ConfirmTimeoutSeconds` | DWORD | 120 | Chờ bạn xác nhận thao tác rủi ro; hết giờ = từ chối |
| `VisualQaEnabled` | DWORD | 0 | 1 = cho AI xem ảnh chụp cửa sổ để soát bố cục (tốn token) |

Agent Core đọc cùng khoá trên. Biến môi trường `AXIOM_*` (`AXIOM_CORE_DATA_DIR`, `AXIOM_CORE_PORT`,
`AXIOM_SESSION_DIR`, `AXIOM_TOKEN`, `AXIOM_LLM_*`) ghi đè — dùng cho test, không cần cho người dùng.

## Phát triển

**Yêu cầu:** .NET Framework 4.8, Visual Studio 2022 Build Tools (`csc` Roslyn, C# 7.3). Python
chỉ cần cho bộ test. **Agent Core** (`AxiomOffice.Core.exe`) là bản Go trong `core-go/`
([thiết kế](core-go/README.md)) nên cần **Go 1.26+**; không có Go thì `build.ps1` vẫn build
add-in + Host và bỏ qua Core.

```powershell
scripts\build.ps1              # build AxiomOffice.dll + AxiomOffice.Host.exe vào src\AxiomOffice\bin\Release
scripts\build.ps1 -Kill        # tự tắt app đang giữ DLL (lưu tài liệu trước!)
scripts\install.ps1            # đăng ký từ repo (HKCU), in cấu hình MCP
scripts\uninstall.ps1 [-Purge] # gỡ đăng ký; -Purge xoá cả cấu hình và log
scripts\package.ps1 [-NoBuild] # tạo dist\AxiomOffice-<version>-<ngày>-<commit>.zip
```

- `build.ps1` hỏi Windows Restart Manager xem tiến trình nào đang giữ DLL/EXE; có thì dừng và
  in tên + pid. Biên dịch vào `bin\Release\.stage` rồi mới chép đè, nên lỗi build hay file bị
  lock đều giữ nguyên DLL cũ.
- `install.ps1` gỡ nhãn Zone.Identifier của DLL/EXE, gỡ đăng ký của bản `WpsAiBridge` cũ nếu có
  (`scripts/legacy.ps1`), đọc version từ DLL.
- `package.ps1` đóng gói DLL, EXE, script cài/gỡ, `install.cmd`/`uninstall.cmd`,
  `HUONG-DAN-CAI-DAT.txt` và `THIRD-PARTY-NOTICES.md`; tên zip có `-dirty` khi code chưa commit.

### Companion (không dùng add-in)

```powershell
& "src\AxiomOffice\bin\Release\AxiomOffice.Host.exe" word           # hoặc excel, ppt, wps, et, wpp
& "src\AxiomOffice\bin\Release\AxiomOffice.Host.exe" excel --visible # hiện cửa sổ app
& "src\AxiomOffice\bin\Release\AxiomOffice.Host.exe" llm-test        # thử cấu hình AI
& "src\AxiomOffice\bin\Release\AxiomOffice.Host.exe" commands        # danh sách lệnh bridge (--json | --markdown)
& "src\AxiomOffice\bin\Release\AxiomOffice.Core.exe"                 # Agent Core (log ra terminal + %LOCALAPPDATA%\AxiomOffice\core.log)
```

Companion tự tạo app qua COM và mở bridge ở cùng port; nếu add-in đã giữ port thì companion
chuyển sang idle. Lưu ý: trên máy có WPS, WPS có thể đăng ký đè ProgID
`Word/Excel/PowerPoint.Application`, khi đó companion `word|excel|ppt` tạo ra WPS thay vì Office
thật — muốn Office thật thì mở app trực tiếp (add-in tự nạp).

### Thêm lệnh bridge

Mỗi lệnh `POST /cmd` khai báo **một lần** bằng một dòng `Command(...)` cạnh handler trong
`src/AxiomOffice/Bridge/CommandDispatcher.{Writer,Spreadsheet,Presentation}.cs` (lệnh chung trong
`CommandDispatcher.cs`): tên, loại app, handler, mô tả, tham số (`Req`/`Opt`, kèm gợi ý cho model).
Từ đó tự có: dispatch + kiểm tra loại app, tool `office_action` của Ask AI (nếu gắn `.ForAgent()`),
danh sách lệnh trong mô tả tool MCP `word_command` / `ppt_command` / `wps_live_command`. Sau khi
thêm/sửa lệnh:

1. `AxiomOffice.Host.exe commands --markdown` → chép đè bảng [Danh sách lệnh](#danh-sách-lệnh-post-cmd)
   (test MCP báo lỗi nếu README lệch).
2. Thêm lệnh vào `tests/live/test_live_commands.py` (test báo lỗi nếu có lệnh chưa được gọi).

### Kiểm thử

```powershell
# Mọi lệnh bridge trên Word/Excel/PowerPoint thật (test tự mở app riêng, xong tự đóng; dừng nếu
# port đã có app của bạn). --wps: chạy trên WPS; --ai: thêm ai.ask (gọi LLM thật);
# --record/--compare golden.json: ghi / so kết quả từng lệnh trước-sau khi refactor.
tools\excel-mcp\.venv\Scripts\python.exe tests\live\test_live_commands.py [--wps] [--ai] [--compare golden.json]
# MCP server: MCP client chính thức + so kết quả với bản Python trên cùng file, registry lệnh
tools\excel-mcp\.venv\Scripts\python.exe tests\mcp-host\test_mcp_host.py <thư_mục_output>
# Tạo file mẫu bằng Office thật / mở file C# ghi ra bằng Office thật
powershell -ExecutionPolicy Bypass -File tests\mcp-host\office_roundtrip.ps1 -Dir <thư_mục_output> -Make
powershell -ExecutionPolicy Bypass -File tests\mcp-host\office_roundtrip.ps1 -Dir <thư_mục_output> -Verify
# Unit test của các MCP Python (legacy)
cd tools\word-mcp; .venv\Scripts\python.exe -m unittest discover -s tests
# Làn LibreOffice: 56 unit test (giải mã tham số + registry lệnh khớp bản C#, logic pane, theme + đọc SSE của
# Core) và test mọi lệnh trên LibreOffice thật (script tự mở LibreOffice bằng profile người dùng;
# --ui: bản có cửa sổ; --ai: ai.ask). Chạy được cả trên Linux (đường dẫn soffice tự dò, tắt app bằng
# SIGTERM, token đọc từ ~/.config/axiom-office/config.json).
python tests\lo\test_extension.py
python tests\lo\test_chat.py
python tests\live\test_live_libreoffice.py [--apps writer,calc,impress] [--ui] [--ai]
python3 tests/live/test_live_libreoffice.py [--apps writer,calc,impress] [--ui] [--ai]   # Linux
# Agent Core (Go): unit test (cần Go 1.26+; chạy được cả Windows lẫn Linux)
cd core-go && go test ./...
# Wizard thiết lập (Windows): render wizard thật ra ảnh từng bước để soi giao diện (không cần Office)
powershell -ExecutionPolicy Bypass -File tests\ui\shots.ps1 [-OutDir <thư mục>]   # ảnh vào tests\ui\out\
# Agent Core e2e: Core thật + LLM giả + bridge giả (không cần Office)
tools\excel-mcp\.venv\Scripts\python.exe tests\core\test_core_e2e.py
# ... chỉ vài phần (fake_bridge,guards,skills,memory,confirm,mcp,visual,setup,shutdown,anthropic,
#     embeddings,summarize,cancel,mcp_http),
#     hoặc với Core bản Go (core-go/README.md)
$env:AXIOM_E2E_CORE_EXE = "<đường dẫn axiom-core.exe>"; python tests\core\test_core_e2e.py --only setup
# ... hoặc trên Office thật: tự mở Excel, agent sửa tài liệu thật (cần add-in đã cài)
tools\excel-mcp\.venv\Scripts\python.exe tests\core\test_core_e2e.py --office
# ... hoặc với LLM thật trong HKCU (tốn token, không chạy mặc định): model có tự chọn đúng skill không
#     AXIOM_REAL_LLM_APPS=wps,et,wpp lọc app; AXIOM_REAL_LLM_MODEL=<model> thử model khác cùng endpoint
tools\excel-mcp\.venv\Scripts\python.exe tests\core\test_core_e2e.py --real-llm
```

Test MCP cần venv của `tools/word-mcp`, `tools/excel-mcp`, `tools/ppt-mcp` (python-docx, openpyxl,
python-pptx, mcp) để đối chiếu.

### MCP servers Python (`tools/`, legacy)

Ba server Python cũ (`tools/excel-mcp` 17 tool, `tools/word-mcp` 20, `tools/ppt-mcp` 16) được giữ
để đối chiếu trong test và cho `excel_query` (DuckDB SQL). Không phát triển thêm; cài đặt mới dùng
[MCP server C#](#mcp-server).

```powershell
cd tools\excel-mcp
python -m venv .venv
.venv\Scripts\python.exe -m pip install -r requirements.txt
.venv\Scripts\python.exe -m excel_mcp.server   # hoặc run.cmd
```

## Microsoft Office và WPS: lưu ý riêng

Cùng một DLL, một lần cài phục vụ cả hai bộ app:

- **Microsoft Office** đọc `HKCU\Software\Microsoft\Office\{Word,Excel,PowerPoint}\Addins\AxiomOffice.Connect`
  (`LoadBehavior=3`), không cần whitelist. Yêu cầu Office **x64** (xem `Platform` ở
  `HKLM\SOFTWARE\Microsoft\Office\ClickToRun\Configuration`). Đã kiểm chứng trên Office 2024 x64.
- **WPS** cần whitelist `HKCU\Software\Kingsoft\Office\{WPS,ET,WPP}\AddinsWL` (install.ps1 ghi).
  WPS 12 chạy mọi thành phần trong cùng `wps.exe` nên loại app nhận diện qua COM probe
  (`Documents` / `Workbooks` / `Presentations`), không qua tên process. Đã kiểm chứng trên WPS
  12.1.0.28485.
- Word luôn có một cửa sổ `OpusApp` **ẩn, không title** ngay từ khi khởi động — đó là cửa sổ của
  Word, không phải của add-in. Log ghi trạng thái tài liệu/cửa sổ ở `OnConnection`,
  `OnStartupComplete` và lệnh bridge đầu tiên; `app.info` trả `state` tương tự.
- Task pane của WPS 12 mở ra hẹp (~250px) và áp độ rộng trễ; pane tự đo lại và nới về 360px
  (log `Task pane width: ctp=... control=...px`).

## LibreOffice

Cùng giao thức, cùng tên lệnh với add-in — nhưng bridge nằm trong **extension Python UNO**
(`src\AxiomOffice.LibreOffice`), không phải COM add-in, nên không cần .NET và chạy được cả **Windows** lẫn
**Linux** (thiết kế: `LibreOffice_arch.md`). Trên Linux, Agent Core đọc cấu hình từ
`~/.config/axiom-office/config.json` thay cho HKCU, và **không cần .NET trên máy đích**: bản phát hành kèm
Core self-contained.

```powershell
# Đóng LibreOffice trước (unopkg từ chối chạy khi soffice đang mở); script không tự tắt app của bạn.
powershell -ExecutionPolicy Bypass -File scripts\libreoffice.ps1 -Install   # gói .oxt + unopkg add --force
powershell -ExecutionPolicy Bypass -File scripts\libreoffice.ps1 -Status    # extension đã cài, session, /health
powershell -ExecutionPolicy Bypass -File scripts\libreoffice.ps1 -Log       # 40 dòng cuối bridge.log
powershell -ExecutionPolicy Bypass -File scripts\libreoffice.ps1 -Uninstall
```

Mở LibreOffice (Writer/Calc/Impress, có cửa sổ hay `--headless` đều được): job `OnStartApp` của
extension bật bridge, cùng log `%LOCALAPPDATA%\AxiomOffice\bridge.log` và cùng registry session
`%LOCALAPPDATA%\AxiomOffice\sessions\` với add-in — nhưng một tiến trình `soffice` ghi **ba** session
`{pid}-{kind}.json` vì cả ba app dùng chung một process.

| App | Port | kind |
|---|---|---|
| Writer | 47851 | `wps` |
| Calc | 47852 | `et` |
| Impress | 47853 | `wpp` |

Port đổi được bằng `PortLibreOffice` (HKCU, mặc định 47851); token dùng chung `Token` với add-in.
Bận/đổi port thì bridge thử cổng kế tiếp (+10) tối đa 5 lần.

**Cách điều khiển:**

- **Pane Ask AI trong LibreOffice**: menubar có menu **Axiom Office → Ask AI** (và *Settings…*); hoặc lệnh
  `ui.askpane` qua bridge. Giao diện giống pane bên Word/Excel/PowerPoint (cùng màu, cỡ chữ, khoảng cách —
  `axiom/theme.py` là bản LibreOffice của `PaneTheme`): header có chấm accent + link **Trò chuyện mới /
  Ghi nhớ / Cài đặt**; màn hình đầu có **gợi ý** dạng chip (bấm để điền); bong bóng chat bo góc (người dùng
  bên phải nền indigo nhạt, AI bên trái nền xám có viền); mỗi thao tác một dòng "✓ Chèn bảng …
  `writer.insertTable`" (✗ đỏ kèm lỗi nếu hỏng); thẻ **Cần bạn xác nhận** (Đồng ý/Từ chối), thẻ **Đã ghi nhớ**
  (Xoá), thẻ lỗi (Mở Cài đặt); "• • •" khi đang chờ model; ô soạn bo góc với placeholder, nút **Gửi** indigo
  (Enter gửi, Shift+Enter xuống dòng, PageUp/PageDown cuộn hội thoại); footer trạng thái với **Dừng** /
  **Hoàn tác lượt này** (Writer/Calc/Impress — UNO hoàn tác được). Pane nói chuyện với Agent Core qua
  `core.json`; Core chưa chạy thì tự khởi động bằng `CoreExe`.
- **MCP**: `office_sessions` liệt kê cả bridge LibreOffice; gọi lệnh kèm `port`: `word_command
  {action: 'writer.appendText', params: {text: '...'}, port: 47851}`.
- **Agent Core**: `ai.ask` trên bridge (Core đọc `core.json`, chạy agent đầy đủ skill/memory/policy rồi
  gọi ngược `/cmd`). Đây là đường đi đã kiểm chứng end-to-end: model tự nạp skill, sửa tài liệu thật.

Pane mở ở một trong hai chỗ (cùng một giao diện):

- **Deck "Axiom Office" trong sidebar của LibreOffice** khi sidebar đang hiện (Calc, Impress mặc định) — biểu
  tượng bong bóng chat trên thanh tab sidebar; kéo mép sidebar để đổi bề rộng.
- **Pane neo bên phải** khi cửa sổ không có sidebar (Writer trên máy này): vùng tài liệu tự co lại nhường
  chỗ (tài liệu xếp dòng lại, có thanh cuộn riêng), phóng to/thu nhỏ cửa sổ thì xếp lại theo; nút **✕** đóng
  pane và trả lại bề rộng, mở lại giữ nguyên hội thoại.

Hạn chế: con lăn chuột không cuộn được hội thoại (awt không có sự kiện wheel cho khung tự vẽ) — dùng thanh
cuộn hoặc PageUp/PageDown trong ô soạn; hội thoại tự cuộn xuống cuối khi có dòng mới. Chi tiết kỹ thuật:
`LibreOffice_arch.md` mục 14.2–14.3.

Khác bản Office (do UNO):

- **Undo**: Calc và Impress cũng là một bước Ctrl+Z cho mỗi lệnh AI (`XUndoManager`), không như Excel
  qua COM. `writer.undo`/`et.undo` hoàn tác được cả bảng, định dạng, chèn ảnh.
- **`app.screenshot`** xuất **trang/slide hiện tại** ra PNG qua filter `*_png_Export` (không chụp cửa sổ);
  `PixelWidth` của filter chỉ là gợi ý nên ảnh có thể rộng hơn `maxWidth` vài pixel.
- **`writer.replaceAll`**: LibreOffice không tìm xuyên đoạn, nên `\n` chỉ được ở **cuối** chuỗi `find`
  (dịch thành `$`); `\n` ở giữa trả lỗi nêu rõ cách sửa.
- **Style bảng Word** được ánh xạ sang autoformat của LibreOffice (`"Grid Table 4 - Accent 1"` →
  `Box List Blue`); phần không hỗ trợ (`autoFit: content`) nằm trong `skipped` của kết quả
  `writer.formatTable`, không làm hỏng lệnh.
- **`writer.open`/`et.open`/`wpp.open`** trên file **đang mở** thì kích hoạt cửa sổ đó
  (`alreadyOpen: true`) thay vì gọi `loadComponentFromURL` — tránh hộp thoại "đã mở" chặn main thread.
- `/health` có thêm `stuck`: `true` khi main thread không trả lời (thường là một hộp thoại đang mở);
  lệnh khi đó trả lỗi `Busy` nêu rõ phải đóng hộp thoại.

### Linux

Yêu cầu: LibreOffice 7.x trở lên + Python UNO (`python3-uno`). **Không cần .NET, không cần Go trên máy
đích** — bản phát hành kèm Agent Core self-contained (`linux-x64`/`linux-arm64`).

**Máy Ubuntu mới tinh** — chỉ một lệnh apt, rồi cài; phần LLM để wizard lo:

```bash
sudo apt update
sudo apt install -y libreoffice python3-uno
sudo apt install -y libsecret-tools      # tuỳ chọn: lưu khoá API vào keyring thay vì file 0600

# Lấy gói: tải artifact "axiom-office-linux-x64" của job `linux` trong GitHub Actions,
# hoặc build tại máy có mã nguồn: bash scripts/linux/package.sh [--rid linux-arm64]
#   (cần Go 1.26+ và .NET SDK 10; chạy được cả trong Git Bash/WSL)
scp dist/axiom-office-linux-x64-<ver>.tar.gz nguoidung@may-ubuntu:~

tar -xzf axiom-office-linux-x64-0.1.0.tar.gz && cd axiom-office-linux-x64-0.1.0
./install.sh              # không cần tham số: cấu hình AI sau bằng wizard (menu Axiom Office -> Thiết lập…)
./install.sh --systemd    # tuỳ chọn: Core chạy thường trực (systemd --user)
```

- **Phải là LibreOffice bản cài từ gói distro (`.deb`/`.rpm`)**. Bản **snap/flatpak không được hỗ trợ**:
  sandbox chặn kết nối `127.0.0.1` tới Agent Core và quyền chạy binary ngoài. Kiểm tra bằng
  `readlink -f "$(command -v soffice)"` — ra `/snap/bin/soffice` thì gỡ snap rồi cài bản apt.
- Endpoint nội bộ (vd `http://localhost:20128/v1`) thì máy mới phải có đường tới đó trước (cùng mạng hoặc
  SSH tunnel), nếu không bước **Kiểm tra kết nối** báo lỗi mạng chứ không phải lỗi khoá.

```bash
# Hoặc điền sẵn LLM ngay khi cài (bỏ qua thì wizard hỏi sau):
./install.sh --endpoint http://localhost:20128/v1 --model <tên-model> --api-key -   # - = đọc key từ stdin
./install.sh --uninstall [--purge]                                                  # gỡ (--purge xoá cả cấu hình/dữ liệu)
```

- Cài **không cần root**, chỉ dùng `unopkg` của người dùng. Script **không tự tắt LibreOffice**: đang chạy
  thì dừng lại và nhắc bạn đóng.
- Vị trí: Core `~/.local/share/axiom-office/core/` (kèm `core/skills/` — 8 skill dựng sẵn, Core nạp sẵn
  không cần cấu hình), MCP `~/.local/share/axiom-office/mcp/`, cấu hình
  `~/.config/axiom-office/config.json` (quyền `0600`), session dùng chung
  `$XDG_RUNTIME_DIR/axiom-office/sessions`, log `~/.local/share/axiom-office/bridge.log`.
- **Khoá API trên Linux**: pane **Cài đặt** lưu vào **keyring** của người dùng qua `secret-tool` (libsecret)
  khi máy có — `config.json` chỉ giữ `libsecret:LlmApiKey`; máy không có keyring thì lưu thẳng trong file
  `0600`. Core giải mã cả hai, và hiểu cả `dpapi:` (coi như chưa cấu hình nếu không phải máy Windows đã mã hoá).
- **`--systemd`** (tuỳ chọn): cài unit `axiom-office-core.service` vào `~/.config/systemd/user/`,
  `enable --now`; gỡ bằng `--uninstall`. Mặc định Core chỉ chạy khi cần (pane tự khởi động) — unit chỉ để
  Core sẵn sàng từ đầu phiên.
- `CoreExe` trong cấu hình trỏ tới Core ở chỗ khác; bỏ trống thì pane tự dùng Core đã cài (kèm `install.sh`).
  Trên Windows, Core chưa chạy thì pane tự khởi động nó; trên Linux cũng vậy (tách session, không chết theo
  LibreOffice).
- Bỏ qua các tham số LLM khi cài thì cấu hình sau bằng wizard **Thiết lập…** hoặc pane **Cài đặt**
  (endpoint/model/API key) — cả hai ghi thẳng vào `config.json`.
- Đóng gói/cài/gỡ nhanh bản dev (không cần dựng tarball): `scripts/libreoffice.sh package|install|status|log|uninstall`.
- **CI**: `.github/workflows/ci.yml` — job `linux` chạy unit test extension + build MCP +
  cài LibreOffice/Python UNO rồi chạy toàn bộ test lệnh bridge và test MCP có tool live, cuối cùng đóng gói
  tarball làm artifact; job `windows` chạy unit test, `go test`, build net48 (add-in + Host) và test MCP.

Trạng thái đã kiểm chứng trên Ubuntu 24.04 + LibreOffice 24.2 (KDE Plasma X11): 159/159 test lệnh (headless
và có cửa sổ), 201/201 test Agent Core (Linux và Windows), 70/70 test MCP có tool live, parity MCP với bản
Python 103/103, và một lượt `ai.ask` thật trong cả ba app (Writer/Calc/Impress) — model tự gọi skill, sửa
tài liệu thật. Pane chạy ở **cả hai chỗ**: deck trong sidebar và pane neo bên phải.

## Troubleshooting

| Hiện tượng | Cách xử lý |
|---|---|
| Không thấy tab **Axiom Office** | Đóng hẳn app, chạy lại `install.cmd` / `scripts\install.ps1`, mở lại; xem `%LOCALAPPDATA%\AxiomOffice\bridge.log` (có dòng `Connect constructor ... from <đường dẫn DLL>` khi nạp được) |
| Office tự tắt add-in sau crash | Xoá mục trong `HKCU\Software\Microsoft\Office\16.0\{Word,Excel,PowerPoint}\Resiliency\DisabledItems`, chạy lại `install.ps1` |
| WPS tự tắt add-in sau crash | WPS hạ `LoadBehavior` 3→2 và ghi `AddinsCL\AxiomOffice.Connect`; chạy lại `install.ps1` |
| Word dừng ở hộp thoại *"start in safe mode?"* sau crash | Add-in chưa nạp nên `/health` không trả lời cho tới khi đóng hộp thoại |
| Lệnh báo `no active document` | Chưa có tài liệu mở: gọi `*.newDocument` / `*.newWorkbook` / `*.newPresentation` |
| `401 unauthorized` | Thiếu/sai header `X-Auth-Token` (xem `HKCU\Software\AxiomOffice\Token`) |
| `403` khi gọi từ trình duyệt | Chủ ý: bridge chặn request có header `Origin` |
| Port bận | Một process khác giữ port: `netstat -ano \| findstr :478` |
| Tìm bridge đang sống | `office_sessions` (MCP) hoặc `%LOCALAPPDATA%\AxiomOffice\sessions\*.json` |
| Ô `#N/A` trong Excel đọc ra `null` | Giới hạn của COM (`Value2` trả cùng mã với ô trống); các lỗi khác (`#DIV/0!`, `#REF!`...) trả đúng tên |
| Máy công ty không nạp add-in | Chính sách chỉ cho add-in có chữ ký số; DLL hiện chưa ký |

## Cấu trúc project

```
src/AxiomOffice/              COM add-in (net48)
  Connect.cs                  IDTExtensibility2 / ribbon / task pane, tạo bridge
  Ai/                         Ask AI pane (AskAiPane, PaneControls), agent (AiAgent, LlmClient, OfficeActionTool), Settings
  Bridge/                     HttpBridge, ComGate, SessionRegistry, EventStream, HostProbe, Config, Logger
    CommandDispatcher*.cs     lệnh POST /cmd: khai báo + handler theo app (Writer/Spreadsheet/Presentation), Params
    CommandCatalog.cs         kiểu khai báo lệnh + sinh mô tả (office_action, MCP, README)
  Interop/                    khai báo COM của Office (IDTExtensibility2, IRibbonExtensibility, ICustomTaskPaneConsumer)
  Ribbon/                     Ribbon XML + xử lý nút
src/AxiomOffice.Host/         AxiomOffice.Host.exe: companion + MCP server
  Mcp/                        giao thức MCP, tool file (OOXML) + live, template docx/pptx nhúng
core-go/                      Agent Core viết bằng Go (New_arch.md; thiết kế riêng: core-go/README.md)
  cmd/axiom-core/             main: một Core mỗi người dùng, chọn port, core.json, dừng êm khi nhận SIGTERM
  internal/agent/             Orchestrator, RunManager, luồng sự kiện SSE, prompt, ngữ cảnh, xác nhận
  internal/model/             codec OpenAI/Anthropic + vòng lặp agent; internal/skills, internal/memory, internal/mcp
  internal/api/               endpoint /v1 (health, runs, skills, memory, mcp, setup) + guard
  internal/mcpserver/         MCP server bản Linux (subcommand `mcp`): giao thức, 20 tool file + 30 tool live
  internal/ooxml/ sheet/ xlsx/ docx/ pptx/   engine OOXML + tiện ích bảng tính (port của Host/Mcp\*.cs)
tests/core/                   test Agent Core: e2e (fake_llm.py, test_core_e2e.py, fakes.py)
scripts/                      build, install, uninstall, legacy (gỡ bản WpsAiBridge), core (tắt Core), package, libreoffice
  libreoffice.sh              Linux: đóng gói/cài/gỡ .oxt bằng unopkg của người dùng
  package_oxt.py              đóng gói .oxt (Windows + Linux)
  generate_mcp_commands.py    sinh catalog/live-commands.json + livecommands_gen.go (từ registry extension)
  generate_templates.py       đồng bộ template docx/pptx cho bản Go (go:embed không đọc được ngoài module)
  linux/install.sh            cài cho người dùng cuối Linux: Core (kèm MCP) vào ~/.local/share, .oxt, config.json,
                              --systemd (unit axiom-office-core.service), in cấu hình MCP
  linux/package.sh            tarball linux-x64/arm64: Core (kèm MCP, binary Go) + skills/ + .oxt + install.sh
  linux/axiom-office-core.service.in  unit systemd --user cho Agent Core
scripts/dist/                 install.cmd, uninstall.cmd, HUONG-DAN-CAI-DAT.txt (vào gói cài)
src/AxiomOffice.LibreOffice/  extension Python UNO cho LibreOffice (nói cùng giao thức bridge)
  python/axiom_job.py         component UNO: job OnStartApp -> axiom.bridge.start
  python/axiom_panel.py       component UNO: factory panel cho sidebar
  python/axiom_dispatch.py    component UNO: xu ly URL org.axiomoffice.bridge:... (menu)
  python/pythonpath/axiom/    bridge (HTTP), gate (AsyncCallback), registry lệnh, writer/calc/impress/checks,
                              session; pane: core (client Agent Core), chat (logic hội thoại), theme (màu/cỡ
                              chữ như PaneTheme + PNG góc bo), widgets, chatview, panel, awt, dialogs, dispatch
tests/live/                   test mọi lệnh bridge trên Office/WPS thật
tests/lo/                     unit test extension LibreOffice (không cần LibreOffice: uno giả)
tests/mcp-host/               test parity MCP + registry lệnh + round-trip với Office thật
tests/ui/                     render wizard thiết lập (Windows) ra ảnh từng bước để soi giao diện
tools/*-mcp/                  MCP servers Python (legacy)
```

Đăng ký ghi hoàn toàn vào HKCU (không cần admin):

- `Software\Classes\AxiomOffice.Connect` → CLSID `{BDB3732A-A479-4A24-AD64-D35952035BBA}` (add-in)
- `Software\Classes\AxiomOffice.AskAiPane` → CLSID `{8001B0D7-F189-443A-B3CB-6EB98038C72E}` (task pane)
- CLSID → `mscoree.dll` + `CodeBase` trỏ tới `AxiomOffice.dll`
- `Software\Microsoft\Office\{Word,Excel,PowerPoint}\Addins\AxiomOffice.Connect` (`LoadBehavior=3`)
- `Software\Kingsoft\Office\{WPS,ET,WPP}\AddinsWL` → `AxiomOffice.Connect`

### Branches

| Branch | Nội dung |
|---|---|
| `main` | Add-in C# + companion/MCP — đường chạy chính thức |
| `cpp-native-addin` | Thử nghiệm add-in C++ native (raw COM, không CLR), dừng ở bước WPS chưa gọi `OnConnection` (commit `f2f0f00`); còn dùng tên cũ `WpsAiBridge` |

Lịch sử thay đổi và nguyên nhân các lỗi đã sửa: [CHANGELOG.md](CHANGELOG.md).
