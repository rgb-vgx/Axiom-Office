# Axiom Office từ đầu tới cuối — hướng dẫn cho người mới

Cập nhật: **31/10/2026**. Viết cho người **chưa biết gì** về dự án: chưa từng viết add-in Office, chưa từng làm việc
với LLM agent. Đọc tuần tự từ trên xuống; mỗi mục chỉ dùng khái niệm đã giải thích ở mục trước. Khi tài liệu này
mâu thuẫn với code, **code đúng** — báo lại để sửa tài liệu.

Mất khoảng **một buổi** để đọc hết, và **một tuần** để làm theo lộ trình ở mục 14 cho tới khi tự sửa được một lỗi.

## Mục lục

1. [Axiom Office là gì](#1-axiom-office-là-gì)
2. [Kiến thức nền cần có](#2-kiến-thức-nền-cần-có)
3. [Bức tranh tổng: bốn mảnh ghép](#3-bức-tranh-tổng-bốn-mảnh-ghép)
4. [Bản đồ repo](#4-bản-đồ-repo)
5. [Theo chân một yêu cầu, từ cú bấm tới tài liệu](#5-theo-chân-một-yêu-cầu-từ-cú-bấm-tới-tài-liệu)
6. [Bên trong Agent Core](#6-bên-trong-agent-core)
7. [Bên trong add-in WPS Linux](#7-bên-trong-add-in-wps-linux)
8. [Dựng môi trường phát triển](#8-dựng-môi-trường-phát-triển)
9. [Kiểm thử](#9-kiểm-thử)
10. [Làm thay đổi đầu tiên](#10-làm-thay-đổi-đầu-tiên)
11. [Quy ước của dự án](#11-quy-ước-của-dự-án)
12. [Gỡ lỗi và chẩn đoán](#12-gỡ-lỗi-và-chẩn-đoán)
13. [Những cái bẫy đã có người dẫm](#13-những-cái-bẫy-đã-có-người-dẫm)
14. [Lộ trình tuần đầu](#14-lộ-trình-tuần-đầu)
15. [Đọc tiếp ở đâu](#15-đọc-tiếp-ở-đâu)
16. [Từ điển](#16-từ-điển)

---

## 1. Axiom Office là gì

Một **trợ lý AI làm việc ngay trong tài liệu văn phòng đang mở**. Người dùng mở Word (hoặc WPS Writer), gõ vào
khung chat bên phải "soạn đơn xin nghỉ phép 3 ngày", và chữ hiện ra **trực tiếp trên trang**, từng đoạn, có định dạng.
Mỗi thao tác của AI là một bước Ctrl+Z, nên người dùng luôn hoàn tác được.

Khác với chatbot thường (trả lời bằng chữ để người dùng tự chép), Axiom **điều khiển ứng dụng văn phòng**: gõ chữ,
tạo bảng, ghi công thức Excel, dựng slide — qua chính object model của ứng dụng.

### Ba cách dùng, chung một lõi

| Cách dùng | Ai dùng | Đi qua đâu |
|---|---|---|
| **Khung chat Ask AI** trong ứng dụng | Người dùng cuối | add-in trong app → Agent Core → LLM |
| **MCP server** | AI agent bên ngoài (Claude Desktop, Claude Code…) | `AxiomOffice.Core mcp` → bridge của app (hoặc đọc/ghi thẳng file `.docx/.xlsx/.pptx`) |
| **HTTP API** trên `127.0.0.1` | Script, tích hợp riêng | gọi thẳng bridge của app |

### Nền tảng và trạng thái (31/10/2026)

| Nền tảng | Ứng dụng | Thành phần | Trạng thái |
|---|---|---|---|
| Windows | Microsoft Word / Excel / PowerPoint **64-bit** | add-in COM C# (`src/AxiomOffice`) | Chạy thật trên Office 2019 x64 |
| **Linux** | **WPS Office for Linux** (Writer, Spreadsheets, Presentation) | add-in JS (`src/AxiomOffice.WPS`) | **Trọng tâm phát triển hiện tại** |
| Linux / Windows | LibreOffice | extension Python UNO (`src/AxiomOffice.LibreOffice`) | **Đóng băng** từ 25/10/2026 — không sửa, không test thường xuyên |

Mọi nền tảng dùng chung **Agent Core** viết bằng Go (`core-go/`) — bộ não chạy AI.

---

## 2. Kiến thức nền cần có

Bỏ qua mục nào bạn đã biết.

### 2.1 Add-in văn phòng và "object model"

Word, Excel, WPS… cho phép nạp **add-in**: mã của bên thứ ba chạy bên trong ứng dụng. Add-in điều khiển tài liệu qua
**object model** — một cây đối tượng do ứng dụng cung cấp: `Application.ActiveDocument.Paragraphs(1).Range.Text`,
`ActiveWorkbook.Sheets("Sheet1").Range("A1").Value2`… Mỗi nền tảng có kiểu add-in riêng:

| Nền tảng | Kiểu add-in | Ngôn ngữ ở repo này |
|---|---|---|
| Microsoft Office trên Windows | **COM add-in** (DLL đăng ký trong registry, ứng dụng nạp lúc khởi động) | C# .NET Framework 4.8 |
| WPS Office for Linux | **JS add-in**: một trang web chạy trong trình duyệt nhúng (CEF, tương đương Chrome 87) của WPS, gọi object model qua `window.Application` | JavaScript **ES5** |
| LibreOffice | **Extension UNO** (`.oxt`), mã chạy trong tiến trình `soffice` | Python |

Điểm quan trọng: add-in chạy **bên trong** tiến trình ứng dụng. Nó biết tài liệu nào đang mở, nhưng không nên chứa
logic AI phức tạp (mỗi nền tảng một ngôn ngữ, sẽ phải viết ba lần). Vì vậy Axiom tách đôi: add-in chỉ là "cánh tay"
thực thi lệnh nhỏ, còn "bộ não" nằm ở một tiến trình riêng — Agent Core.

### 2.2 LLM, agent và tool calling

**LLM** (Large Language Model — Claude, GPT, Gemini…) nhận văn bản và trả văn bản. Bản thân nó không bấm được gì.

**Tool calling**: khi gọi LLM, ta gửi kèm danh sách **tool** (tên + mô tả + JSON Schema tham số). Thay vì trả lời
bằng chữ, model có thể trả về "tôi muốn gọi tool `office_action` với `{"action": "writer.appendText", "params":
{"text": "Kính gửi..."}}`". Chương trình của ta chạy tool đó, gửi **kết quả** lại cho model, model quyết định bước
tiếp. Lặp lại tới khi model trả lời bằng chữ — đó là một **lượt** (run).

**Agent** = LLM + vòng lặp tool calling + các quy tắc giữ cho vòng lặp không đi sai. Gần như toàn bộ độ khó của
Axiom nằm ở "các quy tắc" đó: chặn model lặp mãi một lệnh, bắt nó đọc lại sau khi sửa, hỏi người dùng trước khi xoá
dữ liệu, không để nội dung tài liệu ra lệnh cho model…

Hai chuẩn API mà Core nói được: **OpenAI-compatible** (`/v1/chat/completions` — dùng cho OpenAI, Gemini, phần lớn
máy chủ nội bộ) và **Anthropic** (`/v1/messages`). Mã ở `core-go/internal/model/openai.go`, `anthropic.go`.

### 2.3 MCP

**Model Context Protocol** — chuẩn để một AI agent (Claude Desktop, Claude Code…) dùng tool do chương trình khác cung
cấp: JSON-RPC 2.0, mỗi message một dòng qua stdin/stdout (hoặc qua HTTP). Axiom vừa là **MCP server** (cho agent
bên ngoài dùng tool `word_*`, `excel_*`, `ppt_*`, `wps_*`, `doc_*`), vừa là **MCP client** (agent của Axiom dùng thêm
tool của server ngoài do người dùng cấu hình).

### 2.4 Mấy khái niệm web

- **localhost / `127.0.0.1`**: mọi cổng của Axiom chỉ nghe trên máy người dùng, không mở ra mạng.
- **Token + Origin**: để trang web lạ trên trình duyệt không gọi được API cục bộ, Core đòi header `X-Auth-Token` và
  chặn request có header `Origin` lạ (trình duyệt luôn tự gắn `Origin`, trang khác không giả được).
- **SSE (Server-Sent Events)**: một response HTTP để mở, server đẩy từng dòng `event:/data:`. Core dùng nó để đẩy tiến
  độ lượt chạy (đang gọi tool gì, câu trả lời) về khung chat.
- **Long-poll**: client gửi request, server *giữ* nó tới khi có việc (hoặc hết giờ) rồi mới trả; client gửi lại ngay.
  Add-in WPS dùng cách này để nhận lệnh, vì nó không mở được cổng nghe (mục 7).

### 2.5 OOXML

`.docx`, `.xlsx`, `.pptx` thực chất là **file zip chứa XML**. Core có engine riêng (`core-go/internal/ooxml`, `docx`,
`xlsx`, `pptx`) để đọc/ghi các file này **không cần mở ứng dụng** — dùng cho "làn file" của MCP server.

---

## 3. Bức tranh tổng: bốn mảnh ghép

```
                 ┌─────────────────────────── máy người dùng ────────────────────────────┐
                 │                                                                       │
  Người dùng ──► │  Ứng dụng văn phòng          Agent Core (Go, 1 tiến trình/người dùng)  │ ──► LLM
   gõ yêu cầu    │  ┌──────────────────┐        ┌───────────────────────────────────┐    │   (OpenAI-compatible
                 │  │ add-in = BRIDGE  │◄──────►│ vòng lặp agent · tool · skill ·    │    │    hoặc Anthropic)
                 │  │ (cánh tay)       │  HTTP  │ memory · policy · hội thoại · audit│    │
                 │  │ + khung chat     │        │ core.db (SQLite)                   │    │
                 │  └──────────────────┘        └───────────────────────────────────┘    │
                 │          ▲                               ▲ stdio (MCP)                │
                 │          └──── tài liệu đang mở          └── Claude Desktop / Code    │
                 └───────────────────────────────────────────────────────────────────────┘
```

| Mảnh | Làm gì | Ở đâu |
|---|---|---|
| **Bridge** | Nhận lệnh nhỏ (`writer.appendText`, `et.readRange`…) và thực thi bằng object model của app. Mỗi lệnh sửa = một bước Undo | trong add-in của từng nền tảng |
| **Khung chat (pane)** | Giao diện: ô gõ, các bước đang chạy, thẻ xác nhận, nút Hoàn tác | trong add-in |
| **Agent Core** | Bộ não: gọi LLM, chạy vòng lặp, quyết định gọi lệnh nào, lưu hội thoại/ghi nhớ | `core-go/`, binary `AxiomOffice.Core` |
| **LLM** | Mô hình ngôn ngữ ở máy chủ AI (công ty, OpenAI, Anthropic…) | ngoài máy |

### Giao thức bridge — "ngôn ngữ chung"

Mọi bridge (C#, Python, hay bridge WPS do Core đứng ra làm) nói **cùng một giao thức HTTP** trên một cổng loopback:

| Endpoint | Ý nghĩa |
|---|---|
| `GET /health` | còn sống không, app gì (`wps` = Writer, `et` = bảng tính, `wpp` = trình chiếu), pid |
| `GET /commands` | danh sách lệnh bridge này chạy được (catalog) |
| `GET /session` | tài liệu đang mở |
| `POST /cmd {action, params}` | chạy một lệnh → `{ok, result}` hoặc `{ok:false, error}` |

Mỗi bridge còn ghi một **file session** (JSON: pid, app, cổng, tài liệu) vào thư mục chung
(`$XDG_RUNTIME_DIR/axiom-office/sessions` trên Linux). Core đọc thư mục này để biết **ứng dụng nào đang mở ở cổng
nào** — đó là "session registry". Nhờ giao thức chung, Core không cần biết bridge viết bằng gì.

### Các cổng

| Cổng | Ai nghe |
|---|---|
| **47840** | Agent Core (API `/v1/*`, kênh WPS `/wps/*`) |
| 47821–47823 / 47831–47833 | add-in Windows (WPS for Windows / Microsoft Office): Writer/Word, Spreadsheets/Excel, Presentation/PowerPoint |
| 47851–47853 | extension LibreOffice (đóng băng) |
| **47861** (+ cổng do hệ điều hành cấp) | bridge WPS Linux — do Core mở thay add-in, mỗi tiến trình WPS một cổng (mục 7.3) |

---

## 4. Bản đồ repo

| Đường dẫn | Là gì | Ngôn ngữ |
|---|---|---|
| `core-go/` | **Agent Core** — đọc kỹ nhất. `cmd/axiom-core/main.go` là điểm vào; `internal/*` là các gói | Go 1.26 |
| `src/AxiomOffice.WPS/` | **add-in WPS Linux**: `main.js` (vòng poll, ribbon), `commands.js` (handler từng lệnh), `pane*.js` + `pane.html` (khung chat), `mini.*` (khung chat nhỏ cạnh đoạn bôi đen), `ribbon.xml` | JS ES5 |
| `src/AxiomOffice/` | add-in COM Windows: `Bridge/` (HTTP bridge + `CommandDispatcher.*.cs` — handler lệnh), `Ai/` (khung chat, wizard), `Ribbon/` | C# net48 |
| `src/AxiomOffice.Host/` | `Host.exe`: chuyển tiếp `mcp` sang Core, chế độ companion | C# |
| `src/AxiomOffice.LibreOffice/` | extension LibreOffice — **đóng băng** | Python |
| `catalog/live-commands.json` | **nguồn chuẩn** của mọi lệnh bridge (tên, app, tham số, có cho agent dùng không) | JSON |
| `catalog/setup.json` | câu chữ + preset của wizard thiết lập | JSON |
| `skills/` | 8 **skill** văn phòng tiếng Việt (mục 6.7) | Markdown |
| `scripts/` | build, đóng gói, cài đặt, **sinh mã** (`generate_*.py`), kiểm (`check_*.py`) | Python, bash, PowerShell |
| `tests/` | `core/` (e2e Core), `wps/` (add-in WPS), `linux/` (trình cài, gói), `mcp-host/`, `catalog/`, `bench/` (bộ đo chất lượng), `live/` (cần app thật), `oracle/`, `lo/` (đóng băng) | Python, Node |
| `docs/` | tài liệu — mục lục ở `docs/README.md` | Markdown |
| `README.md` · `STATUS.md` · `CHANGELOG.md` | giới thiệu dài · đã làm/chưa làm · lịch sử | |
| `Makefile` | **nguồn duy nhất** liệt kê lệnh test/lint/build — CI gọi đúng các đích này | |

Các gói Go trong `core-go/internal/` (theo thứ tự nên đọc):

| Gói | Vai trò |
|---|---|
| `config` | đọc cấu hình: biến môi trường `AXIOM_*` → `~/.config/axiom-office/config.json` (Linux) / registry HKCU (Windows) → mặc định |
| `api` | HTTP server của Core: `/v1/*` (runs, setup, memory, skills, …) và kênh WPS `/wps/*` |
| `agent` | **orchestrator** của một lượt: chuẩn bị ngữ cảnh, dựng prompt, chọn tool, chạy, kết thúc |
| `model` | gọi LLM (codec OpenAI/Anthropic) + **vòng lặp agent** + các guard |
| `tools` | các tool model gọi được: `office_action`, tool lớn (`slides_from_brief`, `doc_design`…), `remember`, `load_skill`… |
| `office` | client nói giao thức bridge + đọc session registry |
| `wpsbridge` | bridge WPS Linux (Core đứng ra làm bridge thay add-in) |
| `policy` | lệnh nào phải hỏi người dùng trước |
| `skills`, `memory`, `store` | skill, ghi nhớ dài hạn, SQLite `core.db` |
| `mcp`, `mcpserver` | MCP client / MCP server |
| `ooxml`, `docx`, `xlsx`, `pptx`, `sheet`, `cfb` | engine đọc/ghi file Office không cần app |
| `slidelayout` | tính bố cục slide, đi dây sơ đồ |

---

## 5. Theo chân một yêu cầu, từ cú bấm tới tài liệu

Kịch bản: trên **WPS Writer (Linux)**, người dùng gõ vào khung chat *"Thêm dòng Kính gửi Ban giám đốc ở cuối"*.

| # | Chuyện gì xảy ra | Ở đâu trong mã |
|---|---|---|
| 1 | Khung chat gọi `POST /wps/v1/runs {prompt, client, app:"wps"}` tới Core | `src/AxiomOffice.WPS/pane-core.js` → `send()` / `startRun()` |
| 2 | Core xác thực (Origin đúng của Core), chọn **phiên WPS của đúng app** gửi yêu cầu, gắn cổng bridge + tài liệu vào request | `core-go/internal/api/wps.go` (`wpsClientOf` → `Bridge.Resolve`) |
| 3 | Tạo lượt chạy, trả `runId` ngay; lượt chạy tiếp trên goroutine riêng | `api/runs.go` → `serveStartRun` → `go Orchestrator.Execute` |
| 4 | Khung chat mở SSE `GET /wps/v1/runs/{id}/events` để nhận tiến độ | `pane-core.js` (EventSource) |
| 5 | **Chuẩn bị**: tìm session theo cổng, hỏi `/health`, khoá vùng bôi đen nếu có, lấy/ tạo hội thoại, đọc `/commands`, chọn skill hợp app, truy hồi ghi nhớ, lắp danh sách tool (bỏ tool thiếu lệnh — mục 6.4), dựng system prompt | `agent/orchestrator_execution.go` → `prepareExecution` |
| 6 | **Vòng lặp**: gọi LLM với prompt + tool → model trả `office_action {action: "writer.appendText", ...}` | `model/agent.go` → `RunAgent` |
| 7 | Tool `office_action` kiểm: lệnh có trong catalog không, đúng app không, tham số hợp lệ không, có phải hỏi người dùng không (policy) | `tools/office_action.go`, `policy/` |
| 8 | Gửi `POST /cmd` tới bridge WPS (cổng của phiên) | `office/bridge.go` → `BridgeClient.Command` |
| 9 | Bridge WPS **xếp lệnh vào hàng đợi** của đúng tiến trình WPS và chờ | `wpsbridge/wpsbridge.go` → `Client.Enqueue` |
| 10 | Add-in WPS (đang long-poll `POST /wps/v1/poll`) nhận job, chạy `writer.appendText` bằng object model trong một bước Undo, gửi `POST /wps/v1/result` | `src/AxiomOffice.WPS/main.js` (`pollOnce`, `runJob`) → `commands.js` (`execute`) |
| 11 | Kết quả quay về model; model **đọc lại** để kiểm (`writer.getText`) — Core nhắc nếu nó quên (bước verify) | `model/agent.go`, `agent/prompt.go` (`CheckRule`) |
| 12 | Model trả lời bằng chữ → `run.completed`; Core lưu hội thoại, audit, rút ghi nhớ; mở khoá vùng bôi đen | `agent/orchestrator.go` → `finish` |

Trên Windows, bước 8–10 khác: add-in COM tự mở HTTP server, Core gửi `POST /cmd` thẳng vào nó. Các bước còn lại y hệt
— đó là lợi ích của giao thức chung.

---

## 6. Bên trong Agent Core

### 6.1 Một Core cho mỗi người dùng

Core khởi động khi cần (add-in gọi) hoặc chạy thường trực bằng `systemd --user` (Linux). Chỉ một bản chạy cho mỗi người
dùng (khoá `flock` trên Linux, mutex trên Windows); add-in tìm Core qua file `core.json` (pid, cổng, phiên bản). Token
truy cập API nằm trong cấu hình (khoá `Token`, hoặc biến `AXIOM_TOKEN`), không nằm trong `core.json`. Dữ liệu trên
Linux ở `~/.local/share/axiom-office/` (`core.log`, `core.json`, `core/core.db`, `traces/`), cấu hình ở
`~/.config/axiom-office/config.json`.

### 6.2 Catalog lệnh: một nguồn, nhiều bản sinh

`catalog/live-commands.json` liệt kê mọi lệnh bridge: tên (`writer.*` Writer/Word, `et.*` bảng tính, `wpp.*` trình
chiếu, `app.*` dùng chung), `kind` (app), `agent` (model có được gọi không), `undo`, tham số. Từ file này
`scripts/generate_mcp_commands.py` **sinh** bản Go (MCP server), mảng `CATALOG` trong `commands.js` (WPS), danh sách lệnh
hoàn tác được. **Không sửa tay file sinh** — `make check-gen` (chạy trong CI) báo lệch.

Mỗi bridge chỉ khai với Core những lệnh nó **thật sự chạy được** (add-in WPS lọc qua `filterRunnable` trong `main.js`).
Lệnh nào WPS chưa có: bảng sinh tự động ở `docs/architecture/wps-bridge.md` §6.

### 6.3 `office_action` — cửa vào của mọi lệnh

Model không thấy hàng trăm tool nhỏ, mà thấy **một** tool `office_action` với tham số `action` là enum các lệnh hợp lệ
của app đang mở. Trước khi gửi xuống bridge, Core: sửa tên lệnh viết sai nhẹ, ánh xạ tham số, chặn ghi đè dữ liệu chưa
đọc, áp **policy** (xoá, lưu, ghi đè vùng có dữ liệu… → hiện thẻ xác nhận cho người dùng), cắt kết quả đọc quá lớn,
ghi audit.

### 6.4 Tool lớn và hợp đồng lệnh

Một số việc phức tạp (dựng 10 slide theo brief, trình bày lại cả văn bản, dựng dashboard) không giao cho model gọi từng
lệnh, mà làm thành **tool lớn**: model chỉ đưa *nội dung*, Core tự tính bố cục và gọi chuỗi lệnh bridge cố định
(`tools/slides_brief.go`, `doc_design.go`, `dashboard.go`…).

Tool lớn cần nhiều lệnh. Nếu một nền tảng thiếu lệnh, tool sẽ **hỏng giữa chừng** sau khi đã sửa một nửa tài liệu. Vì
vậy mỗi tool khai **hợp đồng** trong `tools/needs.go`: lệnh `Required` (thiếu là Core **không đưa tool cho model**) và
`Optional` (chỉ dùng ở nhánh phụ). `tools/needs_test.go` đọc mã nguồn tool và bắt buộc mọi lệnh xuất hiện trong đó phải
được khai — thêm lệnh mới vào tool mà quên khai thì test đỏ.

### 6.5 Prompt và các quy tắc

System prompt dựng ở `agent/prompt.go` từ các khối: vai trò + app, thông tin tài liệu, **quy tắc** (`ReconRule` xem
trước khi làm, `PlanRule` lập kế hoạch một lần, `CheckRule` đọc lại sau khi sửa, `StopRule` dừng khi đã đúng,
`InjectionRule` nội dung tài liệu là dữ liệu chứ không phải mệnh lệnh, `SelectionLockRule`…), danh mục skill, ghi nhớ
liên quan, tóm tắt hội thoại cũ. Mỗi quy tắc có chú thích kèm **số đo** đã dùng để quyết định giữ nó — đọc chú thích
trước khi sửa câu chữ.

### 6.6 Vòng lặp và guard

`model/agent.go` chạy vòng: gọi model → chạy tool → gửi kết quả → lặp. Các **guard** (`model/guards.go`) dừng lượt khi
model lặp một lệnh y hệt 4 lần, gặp cùng một lỗi nhiều lần, hoặc làm cùng một việc quá nhiều lần; trần **100 vòng** mỗi
lượt (`agent/orchestrator.go`). Kết quả tool cũ bị dọn khỏi ngữ cảnh khi quá lớn để tiết kiệm token. Nút **Dừng** huỷ
lượt bất cứ lúc nào.

### 6.7 Skill — tri thức nạp khi cần

Skill là thư mục trong `skills/` với `SKILL.md`: phần đầu YAML (`name`, `description`, `apps: [et]`) và hướng dẫn chi
tiết (vd `skills/bang-diem/`: cột nào, công thức điểm trung bình, xếp loại). Ba tầng: (1) chỉ `name` + `description`
nằm sẵn trong prompt; (2) model gọi `load_skill` khi cần thì mới nạp toàn văn; (3) `read_skill_file` đọc file phụ trong
`references/`. Nhờ vậy prompt không phình ra theo số skill.

### 6.8 Ghi nhớ dài hạn

Sau mỗi lượt, Core có thể rút "điều đáng nhớ" (vd "người dùng muốn văn bản theo Nghị định 30") vào `core.db`; lượt sau
truy hồi bằng tìm kiếm từ khoá (bm25) + embedding và chèn vào prompt. Có chống trùng, ghim, hạn dùng, xoá mềm. Mã:
`internal/memory`, `internal/store`.

### 6.9 An toàn: những gì luôn được giữ

- Mỗi lệnh sửa = một bước Undo; nút *Hoàn tác lượt này* gỡ cả lượt.
- Bôi đen rồi hỏi → **khoá vùng**: bridge từ chối sửa ngoài vùng đó.
- Lệnh rủi ro → thẻ xác nhận; người dùng từ chối thì model nhận "declined".
- Chế độ *Lập kế hoạch trước*: chỉ cho lệnh đọc, model trả kế hoạch để duyệt.
- Mọi tool call ghi audit (`core.db`); khoá API không bao giờ vào log.

---

## 7. Bên trong add-in WPS Linux

### 7.1 Vì sao nó khác hai add-in kia

Add-in JS của WPS là một trang web trong CEF — **không mở được cổng nghe**. Nên đảo chiều: **Core đứng ra làm bridge**
(cổng 47861…), còn add-in long-poll Core để lấy lệnh:

```
agent / MCP ──POST /cmd──► Core: bridge WPS (:47861)  ──hàng đợi──►  add-in (main.js) poll /wps/v1/poll
                                                     ◄──/wps/v1/result──  chạy lệnh qua window.Application
```

Core cũng **phục vụ chính các file add-in** (`GET /wps/addin/...` từ thư mục `core/wps-addin`); WPS nạp add-in từ URL
đó (đăng ký trong `~/.local/share/Kingsoft/wps/jsaddons/publish.xml` bởi `scripts/wps/install_jsaddon.py`). Hệ quả:
**Core phải chạy trước khi mở WPS**, nếu không WPS bỏ qua add-in im lặng.

### 7.2 Các file

| File | Vai trò |
|---|---|
| `ribbon.xml` | tab *Axiom Office*, menu chuột phải. **Phải là XML hợp lệ** — một dấu `&` trần làm WPS bỏ cả add-in không báo lỗi |
| `main.js` | `ribbon.OnAddinLoad` → bắt đầu vòng poll; xử lý nút ribbon; mở khung chat (`CreateTaskPane`) và khung mini (`ShowDialog`) |
| `commands.js` | `AxiomCommands`: `catalog()`, `appKind(app)`, `execute(app, action, params)` — handler cho từng lệnh, không bao giờ `throw` |
| `pane-core.js` | logic khung chat (không DOM): gửi lượt, đọc SSE, xác nhận, hoàn tác — test được bằng Node |
| `pane.js`, `pane-dom.js`, `pane.html`, `pane.css` | phần giao diện khung chat và các màn hình phụ |
| `mini.*` | khung chat nhỏ cạnh đoạn bôi đen (*Sửa bằng AI*) |
| `probe.js` | dò khả năng của bản WPS đang chạy |

Bắt buộc **ES5** (không `let`/`const`/arrow/template string/`async`) vì CEF của WPS cũ — có test chặn.

### 7.3 Nhiều app WPS cùng mở

Writer, Spreadsheets, Presentation là **ba tiến trình**, mỗi cái nạp một bản add-in. Mỗi bản sinh một `clientId` lúc nạp
và gửi kèm mỗi lần poll. Core coi mỗi client là **một phiên bridge riêng**: client đầu giữ cổng 47861, client sau có cổng
riêng và file session riêng. Khung chat nhận `?client=&app=` qua URL và gửi lại khi gọi Core, nên lượt chạy luôn làm
trên đúng app. (Trước 31/10/2026 cả ba app chung một hàng đợi — mở Writer và Spreadsheets cùng lúc thì AI đọc nhầm bảng
tính.) Chi tiết: `docs/architecture/wps-bridge.md` §2.4.

---

## 8. Dựng môi trường phát triển

Máy chuẩn: **Ubuntu 24.04** (giống CI). Không cần Docker — tái hiện lỗi bằng `HOME`/profile tạm.

```bash
# Công cụ (phiên bản khớp CI: .github/workflows/ci.yml)
#   Go 1.26+ (go.mod; máy cũ hơn tự tải toolchain nhờ GOTOOLCHAIN=auto), Node 22, Python 3.12
go install honnef.co/go/tools/cmd/staticcheck@v0.8.1
python3 -m pip install "ruff==0.17.0"
npm ci                          # eslint cho JS

make help                       # danh sách đích
make core-build                 # -> dist/core/axiom-core (+ skills/)
make test-go test-wps           # hai bộ nhanh nhất, chạy trước mỗi commit
```

Chạy Core cục bộ **không đụng dữ liệu thật** (thư mục dữ liệu, cổng, token riêng):

```bash
AXIOM_CORE_DATA_DIR=/tmp/axiom-dev AXIOM_CORE_PORT=0 AXIOM_TOKEN=dev \
AXIOM_WPS_ADDIN_DIR=$PWD/src/AxiomOffice.WPS ./dist/core/axiom-core
# cổng thật nằm trong /tmp/axiom-dev/core.json
```

Thử với **WPS thật** (máy có WPS 12): đăng ký add-in vào `HOME` tạm để không sửa hồ sơ WPS của bạn:

```bash
python3 scripts/wps/install_jsaddon.py --home /tmp/wps-home --core-url http://127.0.0.1:<cổng Core> --test-profile
# Mở app dưới Xvfb với HOME tạm (không đụng màn hình và hồ sơ WPS của bạn):
HOME=/tmp/wps-home xvfb-run -a -s "-screen 0 1280x900x24" wps /tmp/wps-home/thu.docx &
```

Lần đầu mỗi app hỏi *"Addin … is loaded for the first time, trust or not?"* — bấm OK (dưới Xvfb: `xdotool mousemove
696 461 click 1`, lấy `DISPLAY`/`XAUTHORITY` từ `/proc/<pid wps>/environ`). Tài liệu cũ ghi WPS cần `JsApiPlugin=true` trong
`oem.ini` (cần sudo); bản 12.1.2 trên máy dev (31/10/2026) không cần — xem `tests/wps/REFERENCE.md` mục cuối.

Cấu hình LLM khi dev: biến môi trường `AXIOM_LLM_PROVIDER` (`openai`/`anthropic`), `AXIOM_LLM_ENDPOINT`,
`AXIOM_LLM_MODEL`, `AXIOM_LLM_API_KEY` — hoặc qua màn hình *Thiết lập* trong khung chat. Các bộ test dùng **LLM giả**
(`tests/core/fake_llm.py`) nên không cần khoá thật.

---

## 9. Kiểm thử

Bốn tầng, từ nhanh tới chậm:

| Tầng | Lệnh | Cần gì | Kiểm cái gì |
|---|---|---|---|
| Unit Go | `make test-go` (`go test -race ./...`) | không | từng gói Core |
| Unit add-in WPS | `make test-wps` | Node | `commands.js`, vòng poll, khung chat, ribbon — với `Application` giả (`tests/wps/addin_fake.mjs`) |
| E2E Core thật | `make test-core`, `make test-wps-e2e`, `make test-mcp` | build Core | Core thật qua HTTP, LLM giả, bridge/add-in giả |
| Live | `tests/live/*.py`, `tests/wps/test_wps_bridge_live.py` | app thật | lệnh trên WPS/Office thật — chỉ máy dev |

Ngoài ra: `make lint` (gofmt, vet, staticcheck, ruff, eslint), `make check-gen` (file sinh khớp nguồn), `make
test-install` / `test-package` (trình cài và gói Linux), `make test-bench` (bộ chấm slide). `make test` chạy mọi thứ
không cần app thật; job Linux của CI gọi đúng các đích `make` này (Makefile là nguồn duy nhất liệt kê test).

Cách viết test ở repo này:

- **Viết ca đỏ trước khi sửa**, và kiểm rằng nó đỏ *vì đúng lỗi* (không phải vì lý do khác).
- Test e2e tái hiện kịch bản của người dùng (`tests/wps/test_wps_multiapp_e2e.py` là ví dụ tốt: hai add-in giả, LLM
  giả, kiểm lệnh tới đúng app).
- Fake phải giống thật: nhiều lỗi từng lọt vì fake "thân thiện" hơn WPS thật (mục 13). Số đo trên app thật ghi ở
  `tests/wps/REFERENCE.md`.
- Test đỏ mà bạn nghĩ test sai? Chứng minh bằng code/catalog thật trước khi sửa test — đừng nới lỏng phép so sánh.

---

## 10. Làm thay đổi đầu tiên

### 10.1 Viết handler cho một lệnh WPS còn thiếu

Ví dụ lệnh `writer.insertHyperlink` (có trong catalog, WPS chưa có handler):

1. Đọc định nghĩa lệnh trong `catalog/live-commands.json` (tham số, `undo`) và cách bản Windows làm
   (`src/AxiomOffice/Bridge/CommandDispatcher.*.cs`).
2. Đo API trên **WPS thật** trước (`probe.js`, console của WPS) — đừng đoán từ tài liệu Microsoft.
3. Viết handler trong `src/AxiomOffice.WPS/commands.js` (ES5, trả `{ok, result|error}`, lệnh sửa chạy trong
   `UndoRecord`), thêm vào `fake` nếu cần, viết test node ở `tests/wps/`.
4. Xoá lệnh khỏi `CHUA_CO_HANDLER` trong `tests/wps/test_addin.mjs` (spec đỏ nếu quên).
5. `python3 scripts/generate_wps_gaps.py` cập nhật bảng ở tài liệu; `make test-wps check-gen lint-js`.
6. Thử trên WPS thật; ghi số đo mới vào `tests/wps/REFERENCE.md`.

### 10.2 Thêm một lệnh bridge hoàn toàn mới

`catalog/live-commands.json` → `python3 scripts/generate_mcp_commands.py` → handler ở từng bridge (C#, WPS) → nếu chưa
làm được ở WPS thì thêm vào `CHUA_CO_HANDLER` kèm lý do → nếu lệnh chỉ đọc/không sửa tài liệu thì thêm vào bảng
`noDocumentChange`/`readOnly` ở `core-go/internal/tools/tools.go`.

### 10.3 Thêm một tool Core gọi lệnh bridge trực tiếp

Viết tool trong `core-go/internal/tools/`, khai `Commands()` trong `tools/needs.go`, thêm vào `toolSources` của
`needs_test.go`, gắn vào danh sách tool ở `agent/orchestrator_execution.go` (`actTools`). Core tự ẩn tool trên backend
thiếu lệnh.

---

## 11. Quy ước của dự án

- **Ngôn ngữ**: tài liệu và giao diện tiếng Việt **có dấu**; chú thích trong mã tiếng Việt **không dấu** (ASCII); tên
  biến/hàm tiếng Anh. Chú thích giải thích **vì sao** (thường kèm ngày + số đo), không lặp lại mã làm gì.
- **Commit**: `loại(phạm vi): mô tả tiếng Việt không dấu` — vd `fix(bridge): ...`, `feat(wps): ...`, `docs(audits): ...`.
- **File sinh**: không sửa tay (có mốc `BEGIN/END SINH TU DONG` hoặc header "SINH TU DONG"); sửa nguồn rồi chạy script.
- **Đừng chép số vào tài liệu** (số lệnh, số test) nếu sinh được từ code — số chép tay luôn lệch.
- **Tài liệu sống vs ảnh chụp**: `README`, `STATUS`, `docs/architecture/`, `docs/user/` phải khớp code; `docs/audits/`,
  `docs/research/` là ảnh chụp có ngày — không sửa nội dung cũ, viết bản mới. `docs/plans/` là danh sách việc còn thiếu.
- **Đo trước khi tin**: hành vi WPS/Office lấy từ đo trên app thật, ghi lại nơi đo.
- **Không làm hỏng giao thức**: Core API v1 và giao thức bridge chỉ được **thêm**, không đổi nghĩa cái cũ (bản
  LibreOffice đóng băng và add-in cũ của người dùng vẫn phải chạy).
- Mỗi việc một nhánh; chạy đủ test; không commit thẳng `main` nếu chưa được đồng ý.

---

## 12. Gỡ lỗi và chẩn đoán

| Muốn biết | Xem ở đâu |
|---|---|
| Core đang làm gì | `~/.local/share/axiom-office/core.log` (Linux) · `%LOCALAPPDATA%\AxiomOffice\core.log` (Windows); trong khung chat WPS: ⋯ → *Nhật ký* / màn hình *Nhật ký Core* |
| Toàn bộ một lượt (prompt, tool call, kết quả, quyết định guard) | bật `DebugTrace` (khoá cấu hình, hoặc `AXIOM_DEBUG_TRACE=1`) → `traces/<runId>.jsonl` cạnh `core.log` |
| Chẩn đoán nhanh để dán vào chat | `AxiomOffice.Core doctor` |
| Add-in WPS có kết nối không, app nào ở cổng nào | nút *Trạng thái* trên ribbon WPS (= `GET /wps/v1/status`, có danh sách `clients`); `curl http://127.0.0.1:47861/health` |
| App nào Core đang thấy | thư mục session `$XDG_RUNTIME_DIR/axiom-office/sessions/*.json` |
| Tool nào bị ẩn vì thiếu lệnh | dòng `tools hidden for missing bridge commands` trong `core.log` |

WPS không hiện tab *Axiom Office*? Theo thứ tự: Core có chạy trước WPS không → `JsApiPlugin=true` trong `oem.ini` chưa →
`publish.xml` có dòng `axiomoffice` không → `ribbon.xml` có hợp lệ XML không (`tests/wps/test_addin_files.py`).

---

## 13. Những cái bẫy đã có người dẫm

Mỗi dòng là một lỗi thật, kèm nơi ghi chi tiết.

| Bẫy | Hậu quả | Nguồn |
|---|---|---|
| `&` trần trong `ribbon.xml` | WPS bỏ cả add-in, không báo lỗi gì | `docs/audits/wps-linux-live-test-2026-10-28-run2.md` §3.1 |
| Mở WPS trước khi Core chạy | add-in bị bỏ qua im lặng | `docs/architecture/wps-bridge.md` §6 |
| `getImage` trả data URI | WPS vẽ dấu `?` — phải trả đường dẫn file tương đối | `tests/wps/REFERENCE.md` |
| `FileSystem.readFileString` | bị sandbox, không đọc được file nào — dùng `FileSystem.ReadFile` | `wps-bridge.md` §6 |
| ET ép kiểu khi ghi `Value2` | `"42"` thành số, `"01/01"` thành ngày | `wps-bridge.md` §6 |
| `AutoFilter` của WPS ET | giá trị lọc phải là **chuỗi** (`["7"]`, không phải `[7]`) | audit WPS 28/10 lượt 2 §3.2 |
| ET/WPP không có `UndoRecord` | `et.undo` tự chụp giá trị ô trước khi ghi | `wps-bridge.md` §6 |
| Hai app WPS cùng mở (trước 31/10) | AI làm nhầm app | `wps-bridge.md` §2.4 |
| Fake test "thân thiện" hơn app thật | 71 test xanh trong khi lệnh không chạy trên WPS thật | `wps-bridge.md` §6 |
| `set -o pipefail` + `ls` không khớp | script thoát mã 2 không in gì | `scripts/linux/install.sh` (chú thích ở dòng `oxt=`) |
| Office **32-bit** | mọi add-in .NET làm Office 32-bit crash — chỉ hỗ trợ x64 | `STATUS.md` §2.9 |

---

## 14. Lộ trình tuần đầu

| Ngày | Việc | Xong khi |
|---|---|---|
| 1 | Đọc tài liệu này + `STATUS.md` §1, §3. Dựng môi trường (mục 8), chạy `make test-go test-wps` | hai bộ xanh trên máy bạn |
| 2 | Đi lại mục 5 **trong mã**: đặt breakpoint/log ở từng hàm trong bảng, chạy `tests/wps/test_wps_multiapp_e2e.py` (`AXIOM_E2E_CORE_EXE=$PWD/dist/core/axiom-core`) | kể lại được 12 bước bằng lời |
| 3 | Đọc `core-go/internal/agent/prompt.go`, `model/agent.go`, `tools/office_action.go`, `tools/needs.go` | giải thích được vì sao một tool bị ẩn trên WPS |
| 4 | Đọc `src/AxiomOffice.WPS/main.js`, `commands.js` (vài handler), `tests/wps/test_addin.mjs`; chạy Core cục bộ + WPS thật nếu có | tự gọi được một lệnh: `curl -H 'X-Auth-Token: dev' -H 'Content-Type: application/json' -d '{"action":"writer.getText"}' http://127.0.0.1:47861/cmd` (token = `AXIOM_TOKEN` lúc chạy Core) |
| 5 | Làm một việc nhỏ có test (mục 10.1 là ứng viên tốt), mở PR | PR có ca test đỏ trước, xanh sau |

---

## 15. Đọc tiếp ở đâu

| Khi cần | Đọc |
|---|---|
| Mục lục mọi tài liệu | `docs/README.md` |
| Đã làm gì / đang dở gì | `STATUS.md` |
| Thiết kế tổng thể (HLD) và chi tiết (LLD) | `docs/architecture/README.md` |
| Hợp đồng Core API v1, sự kiện SSE, module Core | `docs/architecture/core.md` |
| Bridge WPS Linux, khung chat WPS, giới hạn đã biết | `docs/architecture/wps-bridge.md` |
| Ribbon và menu của từng nền tảng | `docs/architecture/ribbon-ui.md` |
| Người dùng cuối thấy gì | `docs/user/user-manual.md` |
| Lịch sử thay đổi và lý do | `CHANGELOG.md` |
| Bài kiến trúc dạng trang web (ảnh chụp 12/10/2026) | `docs/architecture/for-interns.html` |

---

## 16. Từ điển

| Từ | Nghĩa |
|---|---|
| **Agent Core** / Core | tiến trình Go chạy AI, một bản mỗi người dùng, cổng 47840 |
| **Bridge** | phần nhận lệnh nhỏ và thực thi trong app; nói giao thức `/health`, `/commands`, `/session`, `/cmd` |
| **Catalog** | danh sách lệnh bridge (`catalog/live-commands.json`; mỗi bridge trả phần nó chạy được qua `/commands`) |
| **Lệnh** (action) | một thao tác bridge, vd `writer.appendText`, `et.readRange`, `wpp.addSlide` |
| `wps` / `et` / `wpp` | ba "app kind": văn bản / bảng tính / trình chiếu (tên theo WPS, dùng cho cả Office) |
| **Lượt** (run) | một yêu cầu của người dùng tới khi AI trả lời xong; có `runId` |
| **Vòng** (round) | một lần gọi LLM trong một lượt |
| **Tool** | thứ model gọi được: `office_action`, tool lớn, `load_skill`, `remember`… |
| **Tool lớn** | tool Core tự gọi chuỗi lệnh bridge cố định (`slides_from_brief`, `doc_design`…) |
| **Hợp đồng lệnh** | `Required`/`Optional` của tool lớn (`tools/needs.go`) |
| **Guard** | luật dừng vòng lặp hỏng (lặp lệnh, lặp lỗi) |
| **Policy** | luật lệnh nào phải hỏi người dùng trước |
| **Skill** | gói hướng dẫn nghiệp vụ trong `skills/`, nạp khi cần |
| **Session registry** | thư mục file JSON cho biết app nào mở ở cổng nào |
| **Client** (WPS) | một bản add-in = một tiến trình WPS = một phiên bridge |
| **Pane** | khung chat Ask AI |
| **Khung mini** | khung chat nhỏ mở từ menu chuột phải *Sửa bằng AI* |
| **Làn live / làn file** | MCP điều khiển app đang mở / đọc-ghi file Office không cần app |
| **SSE** | luồng sự kiện server đẩy về khung chat |
| **Long-poll** | add-in gửi request, Core giữ tới khi có lệnh |
| **ES5** | phiên bản JavaScript cũ mà CEF của WPS chạy được |
| **CEF** | trình duyệt Chromium nhúng trong WPS, nơi add-in JS chạy |
| **Đóng băng** | giữ mã và bản phát hành, không phát triển, không test thường xuyên (LibreOffice) |
