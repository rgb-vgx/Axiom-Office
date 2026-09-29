# Axiom Office

AI agent làm việc ngay trong tài liệu đang mở của **Microsoft Office và WPS Office**
(Word/Writer, Excel/Spreadsheets, PowerPoint/Presentation). Nội dung AI soạn xuất hiện
trực tiếp trên trang trước mắt người dùng; mỗi thao tác của AI trong Word là một bước Ctrl+Z.

Có ba cách dùng, chung một lõi:

| Cách dùng | Dành cho | Thành phần |
|---|---|---|
| **Ask AI** — task pane trong Word/Excel/PowerPoint/WPS | Người dùng cuối | `AxiomOffice.dll` (COM add-in) |
| **MCP server** — Claude Desktop, Claude Code, agent khác | AI agent bên ngoài | `AxiomOffice.Host.exe mcp` |
| **HTTP API** trên localhost (`/cmd`, `/events`, ...) | Script, tích hợp riêng | bridge trong add-in |

Không cần quyền admin, không cần Python. Gói cài ~250 KB.

> Trước đây dự án tên **WPS AI Bridge** (`WpsAiBridge`). Cài bản mới sẽ tự gỡ đăng ký của bản
> cũ và chuyển cấu hình (port, token, cài đặt AI) từ `HKCU\Software\WpsAiBridge` sang
> `HKCU\Software\AxiomOffice`. Thư mục log cũ `%LOCALAPPDATA%\WpsAiBridge` có thể xoá.

## Mục lục

- [Kiến trúc](#kiến-trúc)
- [Cài đặt cho người dùng](#cài-đặt-cho-người-dùng)
- [Ask AI (agent trong app)](#ask-ai-agent-trong-app)
- [MCP server](#mcp-server)
- [HTTP API](#http-api)
- [Cấu hình](#cấu-hình)
- [Phát triển](#phát-triển)
- [Microsoft Office và WPS: lưu ý riêng](#microsoft-office-và-wps-lưu-ý-riêng)
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
 MCP client ── stdio ──► AxiomOffice.Host.exe mcp
                                    └── làn file: đọc/ghi .docx .xlsx .pptx .csv trực tiếp (không cần app)
```

| Thành phần | Vai trò |
|---|---|
| `AxiomOffice.dll` | COM add-in (`IDTExtensibility2`) nạp vào Word/Excel/PowerPoint và WPS: mở HTTP bridge trong process của app, thêm tab ribbon **Axiom Office**, task pane Ask AI và AI agent |
| `AxiomOffice.Host.exe` | `mcp [all\|word\|excel\|ppt]`: MCP server stdio · `wps\|et\|wpp\|word\|excel\|ppt`: companion tự tạo app qua COM automation và mở bridge (khi add-in không nạp được) · `commands`: danh sách lệnh bridge · `llm-test`: thử cấu hình AI |
| `AxiomOffice.Core.exe` | **Agent Core** (theo [New_arch.md](New_arch.md)): process riêng chạy agent cho mọi app, một bản cho mỗi người dùng, chỉ nghe `127.0.0.1:47840`; add-in khởi động khi cần và tìm qua `%LOCALAPPDATA%\AxiomOffice\core.json`. Hiện có: vòng lặp agent (OpenAI-compatible + Anthropic), hội thoại liên tục theo tài liệu, audit tool call, SSE `/v1/runs/{id}/events`, hủy, trần thời gian/token, **skills** (`load_skill`, `read_skill_file`, `/v1/skills`); memory + xác nhận + MCP client ở các giai đoạn sau |

Mỗi app có port riêng; WPS và Microsoft Office dùng hai dải khác nhau nên chạy song song
được (đổi qua registry, xem [Cấu hình](#cấu-hình)):

| App | WPS (`Port`) | Microsoft Office (`PortOffice`) |
|---|---|---|
| Writer / Word | 47821 | 47831 |
| Spreadsheets / Excel | 47822 | 47832 |
| Presentation / PowerPoint | 47823 | 47833 |

## Cài đặt cho người dùng

**Yêu cầu:** Windows 10/11 x64 (có sẵn .NET Framework 4.8), Microsoft Office **x64** và/hoặc
WPS Office **x64**. Không cần admin, Python hay Visual Studio.

1. Giải nén `AxiomOffice-<version>-....zip` (tạo bằng `scripts\package.ps1`) vào một chỗ cố
   định, ví dụ `C:\Tools\AxiomOffice`. Add-in chạy thẳng từ thư mục này; chuyển chỗ thì cài lại.
2. Đóng Word, Excel, PowerPoint, WPS.
3. Nhấp đúp **`install.cmd`**: tự gỡ nhãn "tải từ Internet" của file, đăng ký add-in (HKCU),
   whitelist cho WPS và in sẵn cấu hình MCP với đúng đường dẫn.
4. Mở Word/Excel/PowerPoint hoặc WPS: có tab **Axiom Office** trên ribbon.
5. **Ask AI → Cài đặt**: nhập provider, endpoint, model, API key.

Gỡ: nhấp đúp **`uninstall.cmd`** (giữ cấu hình AI và token); `uninstall.cmd -Purge` xoá cả cấu
hình, token và log. Hướng dẫn chi tiết cho người nhận gói: `scripts/dist/HUONG-DAN-CAI-DAT.txt`.

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
  lại tài liệu sau vẫn tiếp tục đúng mạch. Link **Cuộc trò chuyện mới** ở footer để bắt đầu lại từ
  đầu; agent vẫn tự đọc tài liệu khi cần.
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
- Agent bên ngoài gọi cùng logic qua lệnh bridge `ai.ask`.

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

Giao diện vẽ bằng GDI+ theo design tokens trong `PaneTheme` (`src/AxiomOffice/Ai/PaneControls.cs`):
tương phản chữ ≥ 4.5:1, focus ring khi dùng bàn phím, scale theo DPI. Bubble nhận Tab/Ctrl+C
và có menu chuột phải **Sao chép**.

## MCP server

MCP server chạy trong `AxiomOffice.Host.exe` (đi kèm gói cài, template docx/pptx nhúng sẵn):

```json
{
  "mcpServers": {
    "office": {
      "command": "C:\\Tools\\AxiomOffice\\src\\AxiomOffice\\bin\\Release\\AxiomOffice.Host.exe",
      "args": ["mcp"]
    }
  }
}
```

- `mcp` = 50 tool; muốn gọn thì `mcp word` (20), `mcp excel` (16), `mcp ppt` (16).
  `AxiomOffice.Host.exe mcp --list` in danh sách tool.
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
được giữ nguyên**; mọi lần ghi dùng file tạm rồi thay thế (atomic). `.xls` (BIFF) chỉ đọc, qua
Excel hoặc WPS Spreadsheets cài trên máy. Không có truy vấn SQL (`excel_query` của bản Python cũ).

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

Agent Core đọc cùng khoá trên. Biến môi trường `AXIOM_*` (`AXIOM_CORE_DATA_DIR`, `AXIOM_CORE_PORT`,
`AXIOM_SESSION_DIR`, `AXIOM_TOKEN`, `AXIOM_LLM_*`) ghi đè — dùng cho test, không cần cho người dùng.

## Phát triển

**Yêu cầu:** .NET Framework 4.8, Visual Studio 2022 Build Tools (`csc` Roslyn, C# 7.3). Python
chỉ cần cho bộ test. Muốn build thêm **Agent Core** (`AxiomOffice.Core.exe`, .NET 10) thì cần
.NET 10 SDK — cài không cần admin bằng `scripts\install-dotnet-sdk.ps1`; không có SDK thì
`build.ps1` vẫn build add-in + Host và bỏ qua Core.

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
# Agent Core: unit test + test vòng đời trên tiến trình thật (cần .NET 10 SDK)
dotnet test tests\core\AxiomOffice.Core.Tests
# Agent Core e2e: Core thật + LLM giả + bridge giả (không cần Office)
tools\excel-mcp\.venv\Scripts\python.exe tests\core\test_core_e2e.py
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
src/AxiomOffice.Core/         Agent Core (.NET 10, theo New_arch.md)
  Agent/                      Orchestrator, RunManager, RunEventStream (SSE), PromptBuilder, ContextAssembler
  Models/                     codec OpenAI/Anthropic + vòng lặp agent (ModelClient)
  Office/                     đọc session registry, gọi bridge (/health, /cmd, /commands)
  Tools/                      tool registry + office_action (allowlist theo ForAgent)
  Memory/                     SQLite: conversations, messages, runs, tool_calls
  Api/, Config/, Logging/     endpoint v1, cấu hình HKCU + AXIOM_*, log core.log
tests/core/                   test Agent Core: xUnit + e2e (fake_llm.py, test_core_e2e.py)
scripts/                      build, install, uninstall, legacy (gỡ bản WpsAiBridge), core (tắt Core), install-dotnet-sdk, package
scripts/dist/                 install.cmd, uninstall.cmd, HUONG-DAN-CAI-DAT.txt (vào gói cài)
tests/live/                   test mọi lệnh bridge trên Office/WPS thật
tests/mcp-host/               test parity MCP + registry lệnh + round-trip với Office thật
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
