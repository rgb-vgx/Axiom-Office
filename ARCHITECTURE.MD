# Kiến trúc Axiom Office

Tài liệu thiết kế của Axiom Office (branch `main`, sau đợt dọn nợ kỹ thuật tháng 9/2026), chia hai
phần:

- **Phần I: High Level Design (HLD)**: hệ thống nhìn từ ngoài vào: mục tiêu, bối cảnh, các khối
  lớn và cách chúng nói chuyện, kịch bản chính, triển khai, yêu cầu phi chức năng, quyết định kiến
  trúc.
- **Phần II: Low Level Design (LLD)**: từng module bên trong: class, luồng gọi, thread, schema dữ
  liệu, quy tắc xử lý, build và kiểm thử.

Cách cài và dùng xem [README.md](README.md); lịch sử thay đổi xem [CHANGELOG.md](CHANGELOG.md).

## Mục lục

**Phần I: High Level Design**

1. [Mục tiêu và phạm vi](#1-mục-tiêu-và-phạm-vi)
2. [Bối cảnh hệ thống](#2-bối-cảnh-hệ-thống)
3. [Kiến trúc tổng thể: các container](#3-kiến-trúc-tổng-thể-các-container)
4. [Các khối chức năng](#4-các-khối-chức-năng)
5. [Kịch bản chính](#5-kịch-bản-chính)
6. [Triển khai](#6-triển-khai)
7. [Yêu cầu phi chức năng](#7-yêu-cầu-phi-chức-năng)
8. [Quyết định kiến trúc](#8-quyết-định-kiến-trúc)
9. [Rủi ro và giới hạn](#9-rủi-ro-và-giới-hạn)

**Phần II: Low Level Design**

10. [Tổ chức mã nguồn](#10-tổ-chức-mã-nguồn)
11. [Add-in: vòng đời, nhận diện app, UI](#11-add-in-vòng-đời-nhận-diện-app-ui)
12. [Mô hình thread và ComGate](#12-mô-hình-thread-và-comgate)
13. [HTTP bridge](#13-http-bridge)
14. [Hệ lệnh: registry và dispatcher](#14-hệ-lệnh-registry-và-dispatcher)
15. [AI agent](#15-ai-agent)
16. [Session registry](#16-session-registry)
17. [Event stream (SSE)](#17-event-stream-sse)
18. [Host.exe: MCP server và companion](#18-hostexe-mcp-server-và-companion)
19. [Dữ liệu: cấu hình, đăng ký COM, file](#19-dữ-liệu-cấu-hình-đăng-ký-com-file)
20. [Build, cài đặt, đóng gói](#20-build-cài-đặt-đóng-gói)
21. [Thiết kế kiểm thử](#21-thiết-kế-kiểm-thử)

[Phụ lục: thuật ngữ](#phụ-lục-thuật-ngữ)

---

# Phần I: High Level Design

## 1. Mục tiêu và phạm vi

**Bài toán.** Người dùng văn phòng làm việc trên Microsoft Office hoặc WPS Office muốn AI thao tác
**ngay trên tài liệu đang mở**: thấy thay đổi trực tiếp trên màn hình và Undo được, thay vì copy
qua lại với chatbot. Đồng thời các AI agent bên ngoài (Claude Desktop, Claude Code, script) cần một
cách chuẩn để điều khiển app đang mở hoặc xử lý file Office.

**Mục tiêu**

| Mã | Mục tiêu |
|---|---|
| G1 | Một bản cài phục vụ cả **Microsoft Office** và **WPS Office**: Word/Writer, Excel/Spreadsheets, PowerPoint/Presentation |
| G2 | Ba cách dùng trên cùng một lõi: **Ask AI** trong app, **HTTP API** localhost, **MCP server** |
| G3 | Cài **không cần admin**, không cần Python hay runtime thêm (chỉ .NET Framework 4.8 có sẵn trên Windows) |
| G4 | **An toàn**: chỉ localhost, có token; AI không làm mất dữ liệu người dùng |
| G5 | **Dễ mở rộng**: thêm một lệnh chỉ sửa một chỗ |

**Ngoài phạm vi phiên bản này**: Office trên macOS/web; nhiều người dùng hay truy cập qua mạng;
agent nhớ hội thoại giữa các lần Ask; transaction nhiều bước, timeline, tracked changes, audit
journal; gói nghiệp vụ theo chuyên ngành (vd văn bản theo NĐ30).

**Tác nhân**

| Tác nhân | Dùng qua |
|---|---|
| Người dùng văn phòng | Tab *Axiom Office* trên ribbon → Ask AI pane |
| Developer / script tích hợp | HTTP API `127.0.0.1:478xx` |
| AI agent bên ngoài | MCP server `AxiomOffice.Host.exe mcp` |
| Người cài đặt | Gói zip → `install.cmd` |

## 2. Bối cảnh hệ thống

```mermaid
flowchart LR
    enduser(["Người dùng văn phòng"])
    dev(["Developer / script"])
    mcpc(["AI agent bên ngoài<br/>Claude Desktop, Claude Code"])
    installer(["Người cài đặt"])

    ax["Axiom Office"]

    office["Microsoft Office / WPS Office<br/>tài liệu đang mở"]
    llm["LLM provider<br/>OpenAI-compatible / Anthropic"]
    fs[("File Office trên đĩa")]
    win[("Windows<br/>HKCU, %LOCALAPPDATA%")]

    enduser -->|"yêu cầu bằng ngôn ngữ tự nhiên"| ax
    dev -->|"HTTP JSON trên localhost"| ax
    mcpc -->|"MCP qua stdio"| ax
    installer -->|"install.cmd"| ax
    ax -->|"COM automation"| office
    ax -->|"HTTPS, tool-calling"| llm
    ax -->|"đọc/ghi OOXML"| fs
    ax -->|"cấu hình, log, session"| win
```

Axiom Office không có server riêng, không có cơ sở dữ liệu: mọi thứ chạy trên máy người dùng, trong
phiên đăng nhập của họ. Kết nối ra ngoài duy nhất là tới LLM provider do người dùng cấu hình (có
thể là LLM chạy local như Ollama).

## 3. Kiến trúc tổng thể: các container

```mermaid
flowchart TB
    mcpc(["MCP client"])
    dev(["Script"])

    subgraph proc["Process Word / Excel / PowerPoint / WPS (mỗi instance một bộ)"]
        addin["Add-in AxiomOffice.dll<br/>.NET 4.8, COM in-process<br/>Ask AI · AI agent · HTTP bridge · hệ lệnh"]
        om["Object model của app"]
        addin -->|"COM late-bound"| om
    end

    hostexe["AxiomOffice.Host.exe<br/>.NET 4.8 console<br/>MCP server · companion · công cụ dev"]
    cfg[("Config store<br/>HKCU\\Software\\AxiomOffice")]
    rt[("Runtime files<br/>%LOCALAPPDATA%\\AxiomOffice<br/>bridge.log · sessions")]
    files[("File .docx .xlsx .pptx")]
    llm["LLM provider"]

    dev -->|"HTTP 127.0.0.1:478xx"| addin
    mcpc -->|"stdio JSON-RPC"| hostexe
    hostexe -->|"HTTP 127.0.0.1:478xx (làn live)"| addin
    hostexe -->|"ZipArchive + XLinq (làn file)"| files
    addin -->|"HTTPS"| llm
    addin --> cfg
    addin --> rt
    hostexe --> cfg
    hostexe --> rt
```

| Container | Công nghệ | Trách nhiệm | Giao tiếp |
|---|---|---|---|
| **Add-in `AxiomOffice.dll`** | .NET 4.8, C# 7.3, COM in-process, WinForms | UI trong app, AI agent, HTTP bridge, thực thi lệnh trên tài liệu | COM với app; HTTP server loopback; HTTPS tới LLM |
| **`AxiomOffice.Host.exe`** | .NET 4.8 console (biên dịch cùng source add-in) | MCP server (live + file), companion không cần add-in, công cụ dev | stdio với MCP client; HTTP client tới bridge; COM (companion, đọc `.xls`) |
| **Config store** | Registry HKCU | Port, token, cấu hình LLM (API key mã hoá DPAPI) | Đọc/ghi trực tiếp |
| **Runtime files** | File trong `%LOCALAPPDATA%` | Log chung, danh sách bridge đang sống | Đọc/ghi trực tiếp |

## 4. Các khối chức năng

```mermaid
flowchart TB
    subgraph ui["Giao diện"]
        ribbon["Ribbon"]
        pane["Ask AI pane"]
        settings["Cài đặt AI"]
    end
    subgraph app["Ứng dụng"]
        agent["AI agent<br/>vòng tool-calling"]
        mcptools["MCP tools<br/>live + file"]
    end
    subgraph io["Giao tiếp"]
        http["HTTP bridge"]
        mcpserver["MCP server"]
        discovery["Session registry · SSE"]
    end
    subgraph cmd["Hệ lệnh"]
        registry["Command registry + dispatcher<br/>47 lệnh"]
    end
    subgraph integ["Tích hợp host"]
        gate["COM gate · retry · nhận diện app"]
        om["Object model Office / WPS"]
    end
    subgraph filecore["Xử lý file"]
        ooxml["OOXML engine<br/>docx · xlsx · pptx"]
    end

    pane --> agent --> registry
    http --> registry
    mcpserver --> mcptools
    mcptools -->|"live"| http
    mcptools -->|"file"| ooxml
    registry --> gate --> om
    discovery --> gate
```

| Khối | Trách nhiệm | Nằm ở |
|---|---|---|
| Giao diện | Ribbon, task pane chat, form cài đặt AI | Add-in |
| AI agent | Gửi yêu cầu + danh sách lệnh được phép tới LLM, thực thi lệnh model gọi, báo tiến trình | Add-in |
| HTTP bridge | Cửa vào cho agent/script: xác thực, định tuyến, trả JSON | Add-in (và companion) |
| Session registry · SSE | Cho bên ngoài biết bridge nào đang sống, tài liệu nào đang mở, vùng chọn thay đổi | Add-in |
| **Hệ lệnh** | Danh sách lệnh duy nhất (47 lệnh `writer.*`, `et.*`, `wpp.*`, chung), kiểm tra tham số, thực thi | Add-in (Host dùng chung metadata) |
| Tích hợp host | Tuần tự hoá truy cập COM, thử lại khi app bận, nhận diện loại app | Add-in |
| MCP server + tools | 50 tool cho MCP client: làn live gọi bridge, làn file sửa file trực tiếp | Host.exe |
| OOXML engine | Đọc/ghi docx/xlsx/pptx không cần app, lưu atomic | Host.exe |

## 5. Kịch bản chính

### K1: Người dùng nhờ AI sửa tài liệu đang mở

```mermaid
sequenceDiagram
    actor U as Người dùng
    participant UI as Ask AI pane
    participant AG as AI agent
    participant LLM as LLM provider
    participant CMD as Hệ lệnh
    participant APP as Word / Excel / PowerPoint

    U->>UI: "Tạo bảng điểm 5 học sinh, có cột trung bình"
    UI->>AG: chạy nền (UI không bị khoá)
    loop tới khi model trả lời xong
        AG->>LLM: hội thoại + danh sách lệnh được phép
        LLM-->>AG: gọi lệnh, vd et.writeRange
        AG->>CMD: thực thi (chỉ lệnh trong danh sách)
        CMD->>APP: COM
        APP-->>U: thay đổi hiện ngay trên màn hình
        AG-->>UI: một dòng tiến trình
    end
    AG-->>UI: câu trả lời tóm tắt
    Note over U,APP: Mỗi thao tác AI trong Word = 1 bước Ctrl+Z. AI không tự lưu file.
```

### K2–K4: Agent bên ngoài

```mermaid
sequenceDiagram
    participant EXT as Script / MCP client
    participant HX as Host.exe (MCP)
    participant BR as HTTP bridge trong app
    participant APP as App đang mở
    participant F as File trên đĩa

    alt K2: script gọi thẳng HTTP API
        EXT->>BR: POST /cmd + token
        BR->>APP: lệnh
        BR-->>EXT: JSON kết quả
    else K3: MCP, làn live
        EXT->>HX: tools/call word_insert_table
        HX->>HX: chọn bridge (app hoặc port từ office_sessions)
        HX->>BR: POST /cmd + token
        BR->>APP: lệnh
        HX-->>EXT: kết quả
    else K4: MCP, làn file
        EXT->>HX: tools/call excel_write
        HX->>F: sửa OOXML, lưu atomic
        HX-->>EXT: kết quả
    end
```

### K5: Tìm các app đang mở

Mỗi bridge ghi một file session (app, port, tài liệu đang mở) và cập nhật định kỳ. MCP tool
`office_sessions` đọc các file này, hỏi `/health` từng bridge, dọn file của process đã chết, trả
danh sách để agent chọn đúng instance (vd khi mở cả Word và WPS Writer).

## 6. Triển khai

```mermaid
flowchart LR
    subgraph machine["Máy người dùng (một tài khoản Windows)"]
        direction LR
        subgraph office["Microsoft Office x64"]
            word["WINWORD.EXE<br/>bridge :47831"]
            excel["EXCEL.EXE<br/>bridge :47832"]
            ppt["POWERPNT.EXE<br/>bridge :47833"]
        end
        subgraph wps["WPS Office 12"]
            wpsexe["wps.exe (một process cho cả 3 thành phần)<br/>Writer :47821 · Spreadsheets :47822 · Presentation :47823"]
        end
        mcphost["AxiomOffice.Host.exe mcp<br/>(MCP client spawn, sống theo phiên)"]
        reg[("HKCU\\Software\\AxiomOffice")]
        local[("%LOCALAPPDATA%\\AxiomOffice")]
    end
    cloud[("LLM provider")]

    word & excel & ppt & wpsexe --> reg
    word & excel & ppt & wpsexe --> local
    mcphost -->|"HTTP 127.0.0.1"| word & excel & ppt & wpsexe
    mcphost --> local
    word & excel & ppt & wpsexe -->|"HTTPS"| cloud
```

- **Một instance app = một bridge = một port.** Port = port gốc của họ app (WPS 47821, Office
  47831) + 0/1/2 theo loại app (Word, Excel, PowerPoint).
- **Cài đặt**: giải nén zip → `install.cmd` → đăng ký COM và add-in trong HKCU, sinh token. Office
  nạp add-in qua khóa `Addins`, WPS qua whitelist `AddinsWL`. DLL chạy thẳng từ thư mục giải nén.
- **Nâng cấp**: build/giải nén bản mới đè lên, mở lại app. Bản tên cũ `WpsAiBridge` được tự gỡ và
  chuyển cấu hình khi cài.

## 7. Yêu cầu phi chức năng

| Thuộc tính | Yêu cầu | Cách đáp ứng |
|---|---|---|
| **Bảo mật** | Trang web, máy khác hay process lạ không điều khiển được app; không lộ API key | Chỉ nghe loopback; token 128-bit; chặn request có `Origin`; `/cmd` bắt buộc JSON; API key mã hoá DPAPI, `/config` chỉ trả key đã che |
| **An toàn dữ liệu** | AI không làm mất dữ liệu | Agent chỉ gọi lệnh trong allowlist (không `closeAll`); không tự lưu/xuất file; mỗi thao tác Word = 1 bước Undo; làn file lưu atomic, chỉ ghi lại part đã sửa |
| **Ổn định host** | Add-in không được làm crash/treo Office | Mọi truy cập COM đi qua một cổng tuần tự; COM call bọc try/catch; thử lại khi app bận; heartbeat/poller bỏ lượt khi app bận; interface host gọi đều là dispinterface |
| **Tương thích** | Office x64 và WPS 12 với cùng một DLL | Late-bound `dynamic`; lệnh viết theo loại app logic; fallback khi API khác nhau (`SaveAs2` → `SaveAs`...). Đã kiểm chứng Office 2024 x64, WPS 12.1 (Writer) |
| **Phản hồi** | UI không đơ; một lượt AI có giới hạn | Agent chạy nền, tiến trình đẩy về UI; 60s mỗi request LLM, 5 phút mỗi lượt; nút Dừng hủy request ngay |
| **Quan sát** | Chẩn đoán được lỗi ngay trên máy người dùng | `bridge.log` (pid từng dòng, transcript agent, trạng thái tài liệu khi nạp và ở lệnh đầu tiên); `/health`, `/session`, `office_sessions` |
| **Cài đặt, vận hành** | Không admin, không runtime thêm, gỡ sạch | Toàn bộ đăng ký trong HKCU; chỉ cần .NET 4.8; `uninstall -Purge` xoá cả cấu hình và log |
| **Bảo trì** | Thêm lệnh không phải sửa nhiều nơi | Registry lệnh là nguồn duy nhất; dispatcher, agent, MCP, README sinh từ đó |
| **Kiểm thử được** | Refactor không đổi hành vi | Test live trên app thật có so golden; test MCP so với bản Python; test đồng bộ registry–README |

## 8. Quyết định kiến trúc

| # | Quyết định | Lý do | Đánh đổi |
|---|---|---|---|
| AD-1 | COM add-in .NET Framework 4.8, C# 7.3, không NuGet | Có sẵn trên mọi Windows 10/11; Office và WPS đều nạp được qua mscoree | Không dùng được .NET hiện đại, thư viện ngoài |
| AD-2 | Late-bound `dynamic`, không dùng PIA | Một DLL cho cả Office và WPS, không phụ thuộc phiên bản Office | Không kiểm tra kiểu lúc biên dịch |
| AD-3 | Dispinterface cho mọi interface host gọi | Office/WPS gọi qua IDispatch; khai báo IUnknown làm host crash | Phải khai báo `DispId` thủ công |
| AD-4 | HTTP loopback làm cửa vào chung | Ngôn ngữ nào cũng gọi được, debug bằng curl | Tự lo token, chặn Origin |
| AD-5 | Một cổng COM toàn cục (`ComGate`) | Object model Office không an toàn đa luồng (từng crash Word) | Lệnh chạy tuần tự |
| AD-6 | SSE bằng poll-diff 500ms | Chạy giống nhau trên Office và WPS, không cần COM event sink | Trễ tới 500ms |
| AD-7 | Agent chỉ có một tool `office_action` | Prompt gọn, thêm lệnh không đổi schema tool | Model phải đọc danh sách lệnh trong mô tả |
| AD-8 | Registry lệnh khai báo cạnh handler | Một chỗ sửa; dispatcher, agent, MCP, README không lệch nhau | Mô tả tiếng Việt và gợi ý tiếng Anh nằm cùng dòng |
| AD-9 | MCP server C# trong cùng exe | Người dùng không cần Python; dùng chung cấu hình và registry | Tự viết JSON-RPC và engine OOXML |
| AD-10 | Mọi đăng ký trong HKCU | Không cần admin | Chỉ cài cho từng người dùng |

## 9. Rủi ro và giới hạn

| Rủi ro / giới hạn | Ảnh hưởng | Hướng xử lý |
|---|---|---|
| DLL chưa ký số | Máy có chính sách chỉ nạp add-in đã ký sẽ không nạp | Ký bằng certificate của tổ chức |
| Lệnh WPS Spreadsheets/Presentation chưa chạy test live | Có thể có khác biệt object model chưa phát hiện | Chạy `test_live_commands.py --wps` |
| Bridge xử lý HTTP tuần tự | Trong lúc `ai.ask` chạy (tới 5 phút), request khác tới cùng app phải chờ, kể cả `/health` | Tách `ai.ask` sang thread riêng nếu cần song song |
| WPS có thể đăng ký đè ProgID `Word/Excel/PowerPoint.Application` | Companion `word|excel|ppt` tạo ra WPS | Mở app Office trực tiếp (add-in tự nạp) |
| Token và key đọc được bởi mọi process cùng tài khoản | Mô hình tin cậy là tài khoản Windows | Chấp nhận (giống mọi app desktop) |
| Agent không nhớ hội thoại | Muốn "làm tiếp" phải mô tả lại | Tính năng tương lai |
| Excel: `#N/A` đọc ra `null`; `.xls` chỉ đọc; `et.undo` không hoàn tác thay đổi qua COM | Giới hạn của COM/Excel | Ghi rõ trong tài liệu |

---

# Phần II: Low Level Design

## 10. Tổ chức mã nguồn

`AxiomOffice.Host.exe` biên dịch từ **toàn bộ** `src/AxiomOffice/**/*.cs` cộng `src/AxiomOffice.Host/**/*.cs`,
nên Host dùng trực tiếp `CommandDispatcher.Commands`, `Config`, `Logger`, `HttpBridge` (companion).

```
src/AxiomOffice/                         → AxiomOffice.dll
  Connect.cs                    360      Điểm vào COM: IDTExtensibility2, Ribbon, CTP, IAppHost, nhận diện app
  Interop/                               IDTExtensibility2, IRibbonExtensibility, ICustomTaskPaneConsumer (dispinterface)
  Ribbon/RibbonUi.cs            101      Ribbon XML + xử lý nút
  Ai/
    AskAiPane.cs                657      Pane chat: vòng đời một lượt chạy, hủy, lỗi, chèn trả lời
    PaneControls.cs            1357      Control vẽ bằng GDI+, design token PaneTheme
    AiAgent.cs                   49      System prompt + tool office_action
    LlmClient.cs                659      OpenAI-compatible / Anthropic, vòng tool-calling, timeout, hủy
    OfficeActionTool.cs          94      Định nghĩa tool (từ registry), allowlist, gọi dispatcher
    SettingsForm.cs             160      Cấu hình LLM, nút Test
  Bridge/
    HttpBridge.cs               264      HttpListener, kiểm tra Origin/token/Content-Type, định tuyến
    CommandDispatcher.cs        329      Execute, retry, lệnh chung, builder Command/Req/Opt
    CommandDispatcher.Writer.cs 457      21 lệnh writer.*
    CommandDispatcher.Spreadsheet.cs 246 11 lệnh et.*
    CommandDispatcher.Presentation.cs 352 12 lệnh wpp.*
    CommandDispatcher.Params.cs 453      Tham số, values, chuyển giá trị COM, Undo, Save
    CommandCatalog.cs           137      CommandInfo/CommandParam, sinh chữ ký/markdown/JSON
    ComGate.cs                   41      Tuần tự hoá COM
    HostProbe.cs                126      Tài liệu active, mô tả trạng thái, mã COM retryable
    SessionRegistry.cs          238      sessions\{pid}.json, heartbeat
    EventStream.cs              513      SSE /events
    Config.cs                   172      HKCU, port, DPAPI
    Logger.cs                    57      bridge.log
    IAppHost.cs                   9      Trừu tượng host (add-in / companion)

src/AxiomOffice.Host/                    → AxiomOffice.Host.exe (+ toàn bộ source ở trên)
  HostProgram.cs                216      Chọn chế độ: mcp, companion, commands, llm-test
  Mcp/
    McpHost.cs                  115      Nhóm tool, template nhúng
    McpServer.cs                490      JSON-RPC stdio, schema tham số, ToolArgs
    LiveTools.cs                216      30 tool live
    BridgeClient.cs             240      Gọi bridge, tìm session, prune
    WordFiles.cs / PptFiles.cs  570/466  Tool file Word / PowerPoint
    ExcelFiles.cs               826      Tool file Excel, đọc .xls qua COM
    XlsxBook.cs / XlsxStyles.cs / Cells.cs   Mô hình workbook, style, ô / CSV
    OoxmlPackage.cs             523      Zip + XLinq, lưu atomic
    Templates/                           default.docx, default.pptx (MIT)

scripts/                                 build, install, uninstall, legacy, package; dist/ (install.cmd, hướng dẫn)
tests/live/                              test lệnh bridge trên app thật
tests/mcp-host/                          test MCP server, round-trip Office
tools/*-mcp/                             MCP server Python cũ (đối chiếu)
```

## 11. Add-in: vòng đời, nhận diện app, UI

### 11.1 Vòng đời

Office/WPS tạo `AxiomOffice.Connect` qua COM (`mscoree.dll` + `CodeBase` trong HKCU). `Connect`
implement `IDTExtensibility2`, `IRibbonExtensibility`, `ICustomTaskPaneConsumer` (đều là
dispinterface) và `IAppHost`.

```mermaid
sequenceDiagram
    autonumber
    participant H as Host app (UI thread)
    participant C as Connect
    participant B as HttpBridge
    participant S as SessionRegistry
    participant E as EventStream

    H->>C: CoCreateInstance(AxiomOffice.Connect)
    H->>C: OnConnection(Application)
    C->>C: lưu Application, tạo UI invoker (Control ẩn trên UI thread)
    C->>C: log HostProbe.Describe
    C->>B: new HttpBridge(this).Start()
    B->>B: Config.PortForKind(AppKind, IsOfficeHost)
    B->>B: HttpListener localhost + 127.0.0.1, thread AxiomOffice.Http (STA)
    B->>E: new EventStream (poller chưa chạy)
    B->>S: Start: ghi sessions\{pid}.json, thread heartbeat 25s
    H->>C: GetCustomUI → Ribbon XML (tab Axiom Office)
    H->>C: CTPFactoryAvailable(factory)
    H->>C: OnStartupComplete → log trạng thái tài liệu
    Note over H,C: ... người dùng làm việc ...
    H->>C: OnDisconnection
    C->>B: Stop
    B->>S: Stop: xoá file session
    B->>E: Stop: đóng mọi subscriber
```

Mọi hàm host gọi vào đều bọc try/catch và log: lỗi trong add-in không được lan ra host.

### 11.2 Nhận diện app và port

`Connect.AppKind` dò bằng COM chứ không bằng tên process (WPS 12 chạy mọi thành phần trong cùng
`wps.exe`): có `Documents` → `wps`, có `Workbooks` → `et`, có `Presentations` → `wpp`; thất bại mới
đoán theo tên process. `IsOfficeHost` = process là `winword` / `excel` / `powerpnt`.

| App | `AppKind` | `family` | Port (`Config.PortForKind`) |
|---|---|---|---|
| WPS Writer | `wps` | `wps` | `Port` = 47821 |
| WPS Spreadsheets | `et` | `wps` | `Port` + 1 = 47822 |
| WPS Presentation | `wpp` | `wps` | `Port` + 2 = 47823 |
| Microsoft Word | `wps` | `office` | `PortOffice` = 47831 |
| Microsoft Excel | `et` | `office` | `PortOffice` + 1 = 47832 |
| Microsoft PowerPoint | `wpp` | `office` | `PortOffice` + 2 = 47833 |

### 11.3 Ribbon và task pane

- **Ribbon** (tab *Axiom Office*): nhóm *Local bridge* gồm **Status**, **Copy API URL**, **Open Log**;
  nhóm *AI* gồm **Ask AI...**, **Settings**. Nút gọi `Connect.OnButtonAction(control)` → `RibbonUi.HandleButton(tag)`.
- **Task pane**: factory nhận ở `CTPFactoryAvailable`; `CreateCTP("AxiomOffice.AskAiPane", "Axiom Office")`
  luôn chạy trên UI thread qua `Connect.OnUiThread` (lệnh `ui.askpane` đến từ thread HTTP: tạo CTP ở
  đó thì RCW của pane thuộc apartment của thread HTTP). Host không hỗ trợ CTP → cửa sổ nổi
  `AskAiHostForm`.
- **Độ rộng pane**: WPS 12 mở pane hẹp (~250px) và áp độ rộng trễ; `AskAiPane.ScheduleWidthCheck`
  đo lại tối đa 3 lần và gọi `Connect.AdjustTaskPaneWidth` để nới về 360px (quy đổi point/pixel
  theo tỉ lệ đo được).

## 12. Mô hình thread và ComGate

Word từng crash (AV trong `wwlib.dll`) khi heartbeat đọc `ActiveDocument` đúng lúc `ai.ask` đang
ghi: cuộc gọi thứ hai được dispatch reentrant trong vòng message của Word. Vì vậy **mọi thread của
add-in gọi COM qua `ComGate`** (một `lock` toàn cục).

| Thread | Tạo bởi | Việc | Cách vào COM |
|---|---|---|---|
| UI thread của host | Office/WPS | Ribbon, task pane, tạo CTP | Trực tiếp |
| `AxiomOffice.Http` (STA) | `HttpBridge` | Nhận và xử lý **tuần tự** mọi request | `ComGate.Run` (chờ tới lượt) |
| ThreadPool worker | `AskAiPane.StartRun` | Một lượt agent của pane | Mỗi tool qua `ComGate.Run` |
| `AxiomOffice.Session` | `SessionRegistry` | Heartbeat 25s | `ComGate.TryRun(0)` trên threadpool, chờ tối đa 2s |
| `AxiomOffice.EventPoll` | `EventStream` | Poll 500ms khi có subscriber | `ComGate.TryRun(0)`: bận thì bỏ lượt |
| `AxiomOffice.Events#n` | `EventStream` | Ghi SSE cho từng subscriber | Không gọi COM |

```csharp
internal static class ComGate
{
    public static T Run<T>(Func<T> action);             // lock: chờ tới lượt (lệnh, tool của agent)
    public static bool TryRun(int timeoutMs, Action a); // Monitor.TryEnter: bận thì trả false
}
```

Hai lệnh không đi qua cổng ở mức dispatcher (`CommandInfo.Gated = false`): `ai.ask` (giữ lệnh hàng
chục giây, từng tool tự qua cổng) và `ui.askpane` (chuyển sang UI thread; giữ cổng có thể deadlock
với UI thread đang chờ cổng).

## 13. HTTP bridge

`HttpBridge` dùng `HttpListener` với prefix `http://localhost:{port}/` và `http://127.0.0.1:{port}/`.
Thread pump xử lý từng request một; `/events` giao cho thread riêng để pump quay lại ngay.

### 13.1 Endpoint

| Endpoint | Token | Trả về |
|---|---|---|
| `GET /health` | Không | `{"ok":true,"result":{"app","pid","port","version","log"}}` |
| `GET /config` | Có | `provider`, `endpoint`, `model`, `apiKey` đã che (`abcd...wxyz`) |
| `GET /session` | Có | Snapshot session (mục 16) + `sessionFile`; đọc tài liệu live, chờ cổng tối đa 2s |
| `GET /events` | Có | Server-Sent Events (mục 17) |
| `POST /cmd` | Có | `{"action","params"}` → `{"ok":true,"result":...}` / `{"ok":false,"error":...}` |

### 13.2 Pipeline xử lý request

```mermaid
flowchart TD
    req(["Request"]) --> origin{"Có header Origin?"}
    origin -->|"có"| r403["403"]
    origin -->|"không"| health{"GET /health?"}
    health -->|"có"| ok200h["200 Health"]
    health -->|"không"| tok{"Token trong HKCU khác rỗng<br/>và X-Auth-Token sai?"}
    tok -->|"có"| r401["401 unauthorized"]
    tok -->|"không"| route{"Đường dẫn"}
    route -->|"GET /config"| cfg["200 ConfigInfo"]
    route -->|"GET /session"| ses["200 SessionRegistry.Describe(live)"]
    route -->|"GET /events"| sse["EventStream.Accept<br/>(thread riêng)"]
    route -->|"POST /cmd"| ct{"Content-Type<br/>application/json?"}
    route -->|"khác"| r404["404"]
    ct -->|"không"| r415["415"]
    ct -->|"có"| parse{"JSON hợp lệ<br/>và có action?"}
    parse -->|"không"| r400["400"]
    parse -->|"có"| exec["CommandDispatcher.Execute → 200"]
```

### 13.3 Mô hình lỗi

| Tình huống | HTTP | Body |
|---|---|---|
| Lệnh chạy xong | 200 | `{"ok":true,"result":{...}}` |
| Lỗi nghiệp vụ (tham số sai, không có tài liệu, sai loại app, lệnh không tồn tại, COM lỗi) | 200 | `{"ok":false,"error":"KiểuLỗi: message"}` |
| Có `Origin` / thiếu token / sai Content-Type / JSON hỏng / thiếu `action` | 403 / 401 / 415 / 400 / 400 | `{"ok":false,"error":...}` |
| Session registry / event stream chưa chạy | 503 | `{"ok":false,"error":...}` |
| Exception không lường trước trong pump | 500 | `{"ok":false,"error":"internal error: ..."}` |

Nếu giá trị `Token` trong HKCU rỗng thì bỏ qua kiểm tra token (install luôn sinh token).

## 14. Hệ lệnh: registry và dispatcher

### 14.1 Mô hình

```mermaid
classDiagram
    class CommandInfo {
        +string Name
        +string Kind
        +CommandHandler Handler
        +string Summary
        +CommandParam[] Params
        +bool Agent
        +bool Gated
        +string Signature
        +ForAgent() CommandInfo
        +Ungated() CommandInfo
    }
    class CommandParam {
        +string Name
        +bool Required
        +string Hint
    }
    class CommandCatalog {
        +All IList~CommandInfo~
        +ForKind(kind) IEnumerable~CommandInfo~
        +Signatures(commands) string
        +AgentActions(kind) string
        +ToMarkdown() string
        +ToJson() string
    }
    class CommandDispatcher {
        +Commands CommandInfo[]
        +FindCommand(action) CommandInfo
        +Execute(host, action, p) object
        +Health(host, port) object
        +ConfigInfo() object
        -ExecuteAction(host, action, command, p) object
    }
    class OfficeActionTool {
        +Definition(kind) LlmToolDef
        +Execute(host, name, argumentsJson) string
    }
    class IAppHost {
        <<interface>>
        +object Application
        +string AppKind
        +bool IsOfficeHost
    }
    CommandInfo "1" *-- "*" CommandParam
    CommandDispatcher "1" o-- "47" CommandInfo : Commands
    CommandCatalog ..> CommandDispatcher : đọc Commands
    OfficeActionTool ..> CommandCatalog : AgentActions
    OfficeActionTool ..> CommandDispatcher : FindCommand, Execute
    CommandDispatcher ..> IAppHost
```

- `CommandHandler` = `delegate object (IAppHost host, Dictionary<string, object> p)`.
- `Kind`: `null` (mọi app), `wps`, `et`, `wpp`. `Agent`: có trong tool `office_action`. `Gated`: chạy
  trong `ComGate`.
- `Signature`: `et.readRange {range,sheet}`; tham số có hint: `values (2D array of rows)`.

Khai báo đặt cạnh handler, mỗi lệnh một dòng:

```csharp
Command("et.writeRange", "et", EtWriteRange, "Ghi vùng bắt đầu từ ô trên-trái `range`",
    Req("range", "top-left cell e.g. 'A1'"), Req("values", "2D array of rows e.g. ..."), Opt("sheet")).ForAgent(),
```

```mermaid
flowchart LR
    decl["Command(...) trong<br/>CommandDispatcher.Writer / Spreadsheet / Presentation.cs<br/>(lệnh chung: CommandDispatcher.cs)"]
    reg["CommandDispatcher.Commands<br/>+ CommandsByName"]
    decl --> reg
    reg --> d1["Dispatch: tìm handler,<br/>kiểm tra loại app, ComGate"]
    reg --> d2["Tool office_action của Ask AI<br/>(chỉ lệnh .ForAgent())"]
    reg --> d3["Mô tả tool MCP<br/>word_command / ppt_command / wps_live_command"]
    reg --> d4["Bảng lệnh README<br/>Host.exe commands --markdown"]
    reg --> d5["Test: commands --json<br/>(mọi lệnh phải có test live)"]
```

| File | Nội dung |
|---|---|
| `CommandDispatcher.cs` | `Execute`, retry, `EnsureVisibleWordDocument`, `Health`, `ConfigInfo`, builder `Command`/`Req`/`Opt`, lệnh chung |
| `CommandDispatcher.Writer.cs` | Lệnh `writer.*` |
| `CommandDispatcher.Spreadsheet.cs` | Lệnh `et.*` |
| `CommandDispatcher.Presentation.cs` | Lệnh `wpp.*` |
| `CommandDispatcher.Params.cs` | Đọc tham số, giải mã `values`, chuyển giá trị COM, `UndoRecordScope`, `SaveDocument` |
| `CommandCatalog.cs` | `CommandInfo`, `CommandParam`, sinh chữ ký / markdown / JSON |

| Nhóm | Số lệnh | Cho agent (`ForAgent`) | Không cho agent |
|---|---|---|---|
| Chung | 3 | 0 | `app.info`, `ai.ask`, `ui.askpane` |
| `writer.*` | 21 | 18 | `newDocument`, `open`, `closeAll` |
| `et.*` | 11 | 10 | `open` |
| `wpp.*` | 12 | 9 | `newPresentation`, `open`, `addTextBox` |
| **Tổng** | **47** | **37** | |

Danh sách đầy đủ kèm tham số: bảng *Danh sách lệnh* trong README (sinh bằng `commands --markdown`).

### 14.2 Luồng thực thi

```mermaid
sequenceDiagram
    autonumber
    participant Caller as HttpBridge / OfficeActionTool
    participant D as CommandDispatcher
    participant G as ComGate
    participant A as Host app (COM)

    Caller->>D: Execute(host, action, params)
    D->>D: FindCommand(action), gated = command.Gated
    D->>D: lệnh đầu tiên trong process: log HostProbe.Describe
    loop tối đa 10 lần khi COM báo bận
        D->>G: Run (nếu gated)
        G->>D: ExecuteAction
        D->>A: EnsureVisibleWordDocument (chỉ writer.*)
        alt không có lệnh
            D-->>Caller: ok:false "unknown action"
        else sai loại app
            D-->>Caller: ok:false "command requires 'et'..."
        else
            D->>A: command.Handler(host, params)
            A-->>D: kết quả / COMException
        end
        alt COMException retryable
            D->>D: chờ min(100ms x lần, 1000ms) rồi thử lại
        else lỗi khác
            D-->>Caller: ok:false "KiểuLỗi: message" (+ log)
        else thành công
            D-->>Caller: ok:true, result
        end
    end
```

- **COM bận** (thử lại, tổng chờ tối đa ~5,5s): `0x80010001` RPC_E_CALL_REJECTED, `0x8001010A`
  RPC_E_SERVERCALL_RETRYLATER, `0x80010002` RPC_E_CALL_CANCELED, `0x800AC472` (Excel đang sửa ô).
- **`EnsureVisibleWordDocument`**: với `writer.*` (trừ `newDocument`, `open`, `closeAll`), nếu
  `ActiveDocument` không có cửa sổ hiển thị mà có tài liệu khác đang hiển thị thì kích hoạt tài liệu
  đó trước (Word luôn có một cửa sổ ẩn lúc khởi động).
- **`RequireKind`** làm một lần ở dispatcher theo `CommandInfo.Kind`.
- **`CommandRejectedException`**: lỗi trả nguyên văn, không kèm tên kiểu, không log (vd `ui.askpane`
  gọi từ companion).
- **Undo**: handler sửa tài liệu Word bọc trong `UndoRecordScope`
  (`Application.UndoRecord.StartCustomRecord/EndCustomRecord`) nên mỗi thao tác = 1 bước Ctrl+Z;
  host không có `UndoRecord` thì bỏ qua.
- **Lưu**: `SaveDocument` dùng `SaveAs2(path)`, lỗi thì `SaveAs(path)`; `wpp.exportPdf` dùng
  `ExportAsFixedFormat(path, 2)`, lỗi thì `SaveAs(path, 32)`.

### 14.3 Đọc tham số

| Hàm | Quy tắc |
|---|---|
| `ParamString` | `Convert.ToString`, thiếu thì dùng mặc định |
| `ParamInt` | Nhận số và chuỗi số (`"44"`, `"12.0"`, invariant culture). Sai kiểu → `'layout' must be a whole number, got 'Title Only'` |
| `ParamBool` | Nhận `true/false` và chuỗi tương ứng. Sai kiểu → `'bold' must be true or false, got 'có'` |
| `ParamMatrix` | Mảng 2 chiều `values` (`et.writeRange`, `writer.insertTable`, `wpp.addTable`) |

`ParamMatrix`:

1. Chuỗi bắt đầu bằng `[` hoặc `{` → parse JSON.
2. `DecodeArrays`: mỗi lớp `{"item"|"items"|"row"|"rows"|"values": x}` là **một cấp mảng**; `x` là
   danh sách phần tử, hoặc phần tử duy nhất khi `x` không phải mảng. Ví dụ
   `{"item":{"item":["Tổng","=SUM(B2:B3)"]}}` = `[["Tổng","=SUM(B2:B3)"]]` (một dòng).
3. Bỏ lớp bọc thừa ngoài cùng: `[[["a","b"],["c","d"]]]` → `[["a","b"],["c","d"]]`.
4. Mảng 1 chiều = một cột (mỗi phần tử một dòng).
5. Ô bị bọc thành mảng một phần tử được gỡ; ô là mảng/object thật → lỗi.
6. Sai dạng → `ArgumentException` kèm ví dụ đúng để model tự sửa ở vòng sau; không bao giờ trả
   `ok:true` mà không ghi gì.

Chiều ra, `ToPlain` / `ToMatrix` chuyển giá trị COM sang JSON: `object[,]` 1-based của
`Range.Value2` thành list dòng, `DateTime` thành ISO 8601, mã lỗi Excel thành tên (`#DIV/0!`,
`#REF!`...); `#N/A` thành `null` (COM trả cùng mã với ô trống).

### 14.4 Thêm một lệnh

1. Viết handler `private static Dictionary<string, object> XxxYyy(IAppHost host, Dictionary<string, object> p)`
   trong file partial của app.
2. Thêm một dòng `Command("xxx.yyy", kind, XxxYyy, "mô tả tiếng Việt", Req(...), Opt(...))`, gắn
   `.ForAgent()` nếu agent của pane được dùng.
3. `AxiomOffice.Host.exe commands --markdown` → chép đè bảng lệnh trong README (test MCP báo lỗi nếu
   lệch).
4. Thêm lệnh vào `tests/live/test_live_commands.py` (test báo lỗi nếu có lệnh chưa được gọi).

## 15. AI agent

### 15.1 Thành phần

| Class | Vai trò |
|---|---|
| `AskAiPane` | UI chat: gửi prompt, hiện từng thao tác, Dừng, Thử lại, Chèn trả lời (`writer.insertStyledText`) |
| `PaneControls` | `PaneTheme` (design token, scale DPI), `ChatBubble`, `ToolLine`, `TypingBubble`, `PromptBox`, `ComposerPanel`, `ErrorCard`, `ChatList`... |
| `AiAgent` | Dựng system prompt theo loại app, gắn tool `office_action`, gọi `LlmClient.RunAgent` |
| `LlmClient` | Vòng tool-calling; `Chat` cho nút Test và `llm-test` |
| `OfficeActionTool` | Định nghĩa tool (mô tả sinh từ registry), allowlist, thực thi → `CommandDispatcher` |
| `SettingsForm` | Provider, endpoint, model, API key (lưu DPAPI), nút Test |

### 15.2 Một lượt chạy

```mermaid
sequenceDiagram
    autonumber
    actor U as Người dùng
    participant P as AskAiPane (UI thread)
    participant W as ThreadPool worker
    participant AG as AiAgent / LlmClient
    participant L as LLM provider
    participant T as OfficeActionTool
    participant D as CommandDispatcher

    U->>P: nhập yêu cầu, bấm Ask
    P->>W: QueueUserWorkItem (CancellationTokenSource)
    W->>AG: AiAgent.Run(host, prompt, progress, token)
    loop tới khi model trả lời không kèm tool call
        AG->>L: messages + tool office_action (timeout 60s/request)
        L-->>AG: tool_calls
        AG->>T: Execute(office_action, {action, params})
        alt action không có ForAgent
            T-->>AG: ok:false "not an available action"
        else
            T->>D: Execute(host, action, params)
            D-->>T: ok / error
            T-->>AG: JSON kết quả
        end
        AG-->>P: progress (BeginInvoke) → một dòng ToolLine
    end
    L-->>AG: câu trả lời cuối
    AG-->>W: LlmResult (Ok, Text, Transcript, Rounds, Seconds)
    W-->>P: FinishRun (BeginInvoke)
    P-->>U: bubble trả lời, link Chèn trả lời
```

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Running: Ask / Enter
    Running --> Running: tool call + progress
    Running --> Done: model trả lời xong
    Running --> Cancelled: bấm Dừng (Abort request đang chờ)
    Running --> TimedOut: quá 5 phút (AgentTimeoutMs)
    Running --> Failed: lỗi provider / cấu hình
    Done --> Idle
    Cancelled --> Idle
    TimedOut --> Idle: ErrorCard + Thử lại
    Failed --> Idle: ErrorCard + Thử lại / Mở Cài đặt
```

### 15.3 Giao thức với LLM

| Provider | Endpoint | Header | Tool format |
|---|---|---|---|
| `openai` (OpenAI-compatible, cả Ollama, LM Studio...) | `{endpoint}/chat/completions` | `Authorization: Bearer <key>` | `tools` + `tool_calls` / role `tool` |
| `anthropic` | `{endpoint}/messages` | `x-api-key`, `anthropic-version: 2023-06-01` | `tools` + `tool_use` / `tool_result` |

- `LlmResult`: `Ok`, `Text`, `Error`, `Seconds`, `Rounds`, `Transcript`, `Cancelled`, `TimedOut`,
  `StepLimitReached`.
- **Thời gian**: `TimeoutMs` = 60s mỗi request; `AgentTimeoutMs` = 300s cả lượt (linked
  `CancellationTokenSource`); hủy gọi `HttpWebRequest.Abort()` nên dừng ngay.
- **Số vòng**: không giới hạn (`maxIterations` = 0). `StepLimitReached` chỉ xảy ra khi bên gọi tự đặt
  giới hạn.
- **Provider không hỗ trợ tools**: lỗi có chữ "tool" → tắt tools, chạy tiếp như chat thường.

### 15.4 Quy tắc của agent

- **Một tool** `office_action {action, params}`; mô tả liệt kê chữ ký các lệnh `ForAgent()` của loại
  app đang mở (`CommandCatalog.AgentActions`), vd `et.writeRange {range (top-left cell e.g. 'A1'),values (...),sheet}`.
- **Allowlist**: `OfficeActionTool` từ chối lệnh không có `ForAgent` (vd `writer.closeAll`, `ai.ask`
  lồng nhau), log `office_action refused: <action>`. HTTP API và MCP không bị giới hạn này.
- **System prompt**: dùng `office_action` cho mọi thay đổi để người dùng thấy trực tiếp; đọc tài
  liệu khi cần; viết nội dung bằng ngôn ngữ của người dùng; **không `save` / `saveAs` / `exportPdf`
  nếu người dùng không yêu cầu**; không bịa kết quả tool; trả lời tóm tắt 1–2 câu.
- **Mỗi lần Ask là một phiên mới**.
- **Nhật ký** trong `bridge.log`: `AskAiPane: prompt=`, `AskAiPane progress:` (từng tool call + kết
  quả), `AskAiPane: ok ... N tool calls, M rounds`; ghi ngay trên worker kể cả khi pane đã đóng.
- **`ai.ask`** (lệnh bridge) chạy cùng `AiAgent` trên thread HTTP, trả `reply`, `transcript`,
  `seconds`, `rounds`, và log `ai.ask progress:`.

## 16. Session registry

File `%LOCALAPPDATA%\AxiomOffice\sessions\{pid}.json`:

| Trường | Ý nghĩa |
|---|---|
| `pid`, `host` | Process của app (`WINWORD.EXE`, `wps.exe`...) |
| `app`, `family` | `wps`/`et`/`wpp`; `office`/`wps` |
| `port`, `version` | Port bridge, version DLL |
| `started`, `lastSeen`, `lastSeenEpoch` | ISO UTC và epoch giây của lần heartbeat gần nhất |
| `document`, `documentPath` | Tài liệu đang active (cache) |

```json
{"pid":5128,"app":"wps","family":"office","port":47831,"host":"WINWORD.EXE","version":"1.0.0",
 "started":"2026-09-28T21:25:42.965Z","lastSeen":"...","lastSeenEpoch":1790630759.1,
 "document":"bao-cao.docx","documentPath":"C:\\...\\bao-cao.docx"}
```

- Ghi atomic (`.tmp` + `File.Replace`), heartbeat 25s. Đọc tài liệu trên threadpool qua
  `ComGate.TryRun(0)`, chờ tối đa 2s: host bận thì giữ tên đã cache nhưng `lastSeen` vẫn ghi đúng hạn.
- `GET /session` đọc live (chờ cổng tối đa 2s) rồi ghi lại file ngay.
- `OnDisconnection` xoá file. File mồ côi (process chết bất thường) bị `BridgeClient.Sessions` xoá
  khi process không còn, hoặc `lastSeen` quá 90s mà `/health` không trả đúng pid. Kết quả thêm
  `healthy`, `ageSeconds`, `file`, sắp theo `family` rồi `port`.

## 17. Event stream (SSE)

```mermaid
sequenceDiagram
    participant Cl as Client SSE
    participant Pump as Thread HTTP
    participant Wr as Thread Events#n
    participant Po as Thread EventPoll
    participant A as Host app

    Cl->>Pump: GET /events (token)
    Pump->>Wr: Accept: tạo subscriber (tối đa 5), trả pump ngay
    Wr-->>Cl: event hello (+ document/selection đã biết)
    loop mỗi 500ms khi còn subscriber
        Po->>A: ComGate.TryRun(0): đọc tài liệu + vùng chọn
        Po->>Wr: có thay đổi → document / selection
    end
    Wr-->>Cl: event document / selection
    Wr-->>Cl: ping mỗi 15s
    Cl--xWr: ngắt kết nối → gỡ subscriber, hết subscriber thì poller dừng
```

| Event | Dữ liệu |
|---|---|
| `hello` | `app`, `family`, `port`, `pid`, `subscribers` |
| `document` | `app`, `name`, `fullName` |
| `selection` (Writer) | `app`, `text`, `start`, `end` |
| `selection` (Spreadsheets) | `app`, `sheet`, `address`, `text` |
| `selection` (Presentation) | `app`, `slide`, `type`, `shape`, `text`, `start`, `end` |
| `ping` | `time`, `subscribers` |

`text` cắt 200 ký tự; vùng chọn Word dài hơn 5000 ký tự chỉ đọc phần đầu. Lỗi poll log tối đa 1
dòng/phút cho mỗi loại. Response đặt `Cache-Control: no-cache`, `X-Accel-Buffering: no`, chunked.

## 18. Host.exe: MCP server và companion

```mermaid
flowchart LR
    exe["AxiomOffice.Host.exe"] --> m1["mcp [all|word|excel|ppt] [--list]<br/>MCP server stdio"]
    exe --> m2["word | excel | ppt | wps | et | wpp [--visible]<br/>companion"]
    exe --> m3["commands [--json|--markdown]<br/>danh sách lệnh bridge"]
    exe --> m4["llm-test<br/>thử cấu hình LLM"]
```

### 18.1 MCP server

- JSON-RPC 2.0 qua stdio, mỗi message một dòng, hỗ trợ batch; tự viết, không dùng SDK. `stdout`
  chỉ dành cho JSON-RPC (`Console.SetOut(stderr)`), log vào `bridge.log`.
- Phương thức: `initialize` (protocol `2025-11-25`, `2025-06-18`, `2025-03-26`, `2024-11-05`; client
  xin version lạ thì trả bản mới nhất), `ping`, `tools/list`, `tools/call`; notification bỏ qua,
  method lạ → `-32601`.
- `tools/call` trả `content: [{type:"text", text}]`; exception trong tool → `isError: true` với
  `{"ok":false,"error":...}`.
- Nhóm: `all` (50 tool, server `office-tools`), `word` (20), `excel` (16), `ppt` (16); nhóm nào cũng
  có `office_sessions`.

| Nhóm | Làn file (không cần app) | Làn live (qua bridge) |
|---|---|---|
| Word | `doc_profile`, `doc_get_text`, `doc_find_text`, `doc_extract_table`, `doc_create` (5) | `word_health`, `word_command`, `word_read_text`, `word_type_text`, `word_insert_styled_text`, `word_format_selection`, `word_heading`, `word_insert_table`, `word_insert_image`, `word_insert_hyperlink`, `word_replace_all`, `word_export_pdf`, `word_undo`, `word_save` (14) |
| Excel | `excel_profile`, `excel_read`, `excel_write`, `excel_create`, `excel_convert`, `excel_format_range`, `excel_create_table`, `excel_create_sheet`, `excel_copy_sheet`, `excel_rename_sheet`, `excel_delete_sheet` (11) | `wps_health`, `wps_live_command`, `wps_live_read_range`, `wps_live_write_range` (4) |
| PowerPoint | `ppt_profile`, `ppt_get_text`, `ppt_create`, `ppt_add_slide_file` (4) | `ppt_health`, `ppt_command`, `ppt_list_slides`, `ppt_add_slide`, `ppt_add_text`, `ppt_add_image`, `ppt_add_table`, `ppt_set_notes`, `ppt_delete_slide`, `ppt_export_pdf`, `ppt_save` (11) |
| Chung | | `office_sessions` (1) |

Tên và tham số của 50 tool là hợp đồng với client (tương thích bản Python cũ); chỉ mô tả của
`*_command` sinh từ registry lệnh.

### 18.2 Làn live

```mermaid
sequenceDiagram
    participant C as MCP client
    participant M as McpServer
    participant LT as LiveTools
    participant BC as BridgeClient
    participant B as HttpBridge trong app

    C->>M: tools/call word_insert_table {rows, cols, values}
    M->>LT: handler(ToolArgs)
    LT->>BC: Command(app="word", "writer.insertTable", params)
    BC->>BC: ResolvePort: port tường minh, hoặc app → Config.PortForKind
    BC->>B: POST /cmd + X-Auth-Token (đọc từ HKCU), timeout 330s
    B-->>BC: {"ok":true,"result":...}
    BC-->>M: JSON nguyên văn
    M-->>C: content text, isError=false
```

- `app`: `word`/`excel`/`ppt` (Office) hoặc `wps`/`et`/`wpp` (WPS); `port` tường minh ưu tiên hơn.
- Timeout 330s vì `ai.ask` có trần 5 phút.
- Lỗi kết nối (app chưa mở, add-in chưa nạp) trả `{"ok":false,"error":...}` như kết quả bình thường
  để model đọc được, không phải lỗi giao thức MCP.

### 18.3 Làn file

- **`OoxmlPackage`**: đọc/ghi `.docx` / `.xlsx` / `.pptx` bằng `ZipArchive` + XLinq; relationships,
  content types, tên part mới. Chỉ part đã sửa được ghi lại nên chart, ảnh, pivot, macro giữ
  nguyên. Lưu **atomic** qua `FileSafety`: ghi bản sao tạm `.~<tên>.<id>.tmp` cạnh file gốc rồi
  `File.Replace`.
- **Word** (`WordFiles`), **PowerPoint** (`PptFiles`): tạo file mới từ template `default.docx` /
  `default.pptx` nhúng trong exe (lấy từ python-docx / python-pptx, MIT).
- **Excel** (`ExcelFiles`, `XlsxBook`, `XlsxStyles`, `Cells`, `CsvTable`): `.xlsx`/`.xlsm` sửa trực
  tiếp (dịch công thức khi chép sheet, style, bảng); `.csv`/`.tsv` đọc/ghi text; `.xls` (BIFF)
  **chỉ đọc** qua COM (`Excel.Application` → `KET.Application` → `ET.Application`). Không có truy
  vấn SQL (`excel_query` của bản Python cũ), không xuất parquet.

### 18.4 Companion

`AxiomOffice.Host.exe word|excel|ppt|wps|et|wpp [--visible]` tạo app qua ProgID
(`Word.Application`, `KWPS.Application`...), đặt `DisplayAlerts = false`, bọc bằng
`AutomationAppHost` (`IAppHost`) và chạy cùng `HttpBridge` trên cùng port. Bind port lỗi (add-in đã
giữ port) → idle. Ctrl+C dừng bridge và `Quit` app.

## 19. Dữ liệu: cấu hình, đăng ký COM, file

### 19.1 `HKCU\Software\AxiomOffice`

| Giá trị | Kiểu | Mặc định | Ý nghĩa |
|---|---|---|---|
| `Port` | DWORD | 47821 | Port gốc cho WPS |
| `PortOffice` | DWORD | 47831 | Port gốc cho Microsoft Office |
| `Enabled` | DWORD | 1 | 0 = không mở bridge |
| `Token` | String | 32 hex ngẫu nhiên, sinh khi cài | Header `X-Auth-Token` |
| `LlmProvider` | String | `openai` | `openai` hoặc `anthropic` |
| `LlmEndpoint` | String | rỗng | Base URL (tự nối `/chat/completions` hoặc `/messages`) |
| `LlmModel` | String | rỗng | Tên model |
| `LlmApiKey` | String | rỗng | `dpapi:<base64>` (DPAPI `CurrentUser`, không entropy) |

### 19.2 Đăng ký COM và add-in (do `install.ps1` ghi)

```mermaid
flowchart LR
    subgraph classes["HKCU\\Software\\Classes"]
        p1["AxiomOffice.Connect → CLSID {BDB3732A-...}"]
        p2["AxiomOffice.AskAiPane → CLSID {8001B0D7-...}"]
        clsid["CLSID\\{...}\\InprocServer32<br/>mscoree.dll, ThreadingModel=Both<br/>CodeBase=file:///...AxiomOffice.dll<br/>+ subkey version: Assembly, Class, RuntimeVersion"]
    end
    subgraph officekeys["HKCU\\Software\\Microsoft\\Office"]
        o1["Word | Excel | PowerPoint\\Addins\\AxiomOffice.Connect<br/>LoadBehavior=3, CommandLineSafe=1"]
    end
    subgraph wpskeys["HKCU\\Software\\Kingsoft\\Office"]
        w1["WPS | ET | WPP\\AddinsWL<br/>value AxiomOffice.Connect"]
        w2["AddinsCL: xoá AxiomOffice.Connect<br/>(WPS ghi vào đây khi tự tắt add-in)"]
    end
    p1 --> clsid
    p2 --> clsid
    o1 -.->|"Office nạp"| p1
    w1 -.->|"WPS nạp"| p1
```

| Định danh | Giá trị |
|---|---|
| `AxiomOffice.Connect` | CLSID `{BDB3732A-A479-4A24-AD64-D35952035BBA}` |
| `AxiomOffice.AskAiPane` | CLSID `{8001B0D7-F189-443A-B3CB-6EB98038C72E}` |
| Assembly GUID | `5A8E1ED9-10D7-4231-9E7A-01D732C81467` |

`scripts/legacy.ps1` gỡ ProgID/CLSID/Addins/whitelist của tên cũ `WpsAiBridge` và chuyển cấu hình từ
`HKCU\Software\WpsAiBridge`, chỉ xoá khóa cũ sau khi đọc lại thấy khớp từng giá trị.

### 19.3 File runtime

| Đường dẫn | Nội dung |
|---|---|
| `%LOCALAPPDATA%\AxiomOffice\bridge.log` | Log chung add-in và Host, có `[pid N]` từng dòng |
| `%LOCALAPPDATA%\AxiomOffice\sessions\{pid}.json` | Session registry (mục 16) |

## 20. Build, cài đặt, đóng gói

```mermaid
flowchart TD
    rm{"Restart Manager:<br/>tiến trình nào giữ DLL/EXE?"} -->|"có, không -Kill"| stop["Dừng, in tên + pid"]
    rm -->|"không"| csc1
    src1["src/AxiomOffice/**/*.cs"] --> csc1["csc /target:library<br/>/platform:x64 /langversion:7.3"]
    src1 --> csc2
    src2["src/AxiomOffice.Host/**/*.cs"] --> csc2["csc /target:exe<br/>+ System.IO.Compression, System.Xml.Linq<br/>+ /resource default.docx, default.pptx"]
    csc1 --> stage["bin\\Release\\.stage"]
    csc2 --> stage
    stage -->|"chép đè (lỗi thì giữ bản cũ)"| out["bin\\Release\\AxiomOffice.dll<br/>bin\\Release\\AxiomOffice.Host.exe"]
    out --> install["scripts\\install.ps1<br/>đăng ký HKCU, sinh token, in cấu hình MCP"]
    out --> package["scripts\\package.ps1<br/>dist\\AxiomOffice-version-ngày-commit[-dirty].zip"]
```

- **Build** (`scripts\build.ps1`): `csc` của VS 2022 Build Tools (hoặc csc của .NET Framework),
  `/codepage:65001`, chỉ tham chiếu assembly của framework. Hỏi Windows Restart Manager xem tiến
  trình nào đang giữ DLL/EXE (assembly .NET không hiện trong `Process.Modules`); `-Kill` tắt chúng.
  `AxiomOffice.csproj` gom `**\*.cs` giống script, chỉ để mở bằng IDE.
- **Cài** (`install.ps1` / `install.cmd`): gỡ Zone.Identifier, gỡ đăng ký cũ, ghi đăng ký (19.2),
  tạo `Port`/`PortOffice`/`Enabled`/`Token` nếu chưa có, in đoạn cấu hình MCP.
- **Gỡ** (`uninstall.ps1 [-Purge]`): gỡ mọi đăng ký (cả tên cũ); `-Purge` xoá cả cấu hình và log.
- **Gói** (`package.ps1 [-NoBuild] [-Kill] [-OutDir]`): DLL, EXE, script cài/gỡ, `legacy.ps1`,
  `install.cmd`/`uninstall.cmd`, `HUONG-DAN-CAI-DAT.txt`, `THIRD-PARTY-NOTICES.md`; entry zip dùng
  `/`; hậu tố `-dirty` khi code chưa commit.
- Script `.ps1` giữ ASCII không BOM (PowerShell 5.1 đọc sai UTF-8 không BOM).

## 21. Thiết kế kiểm thử

| Bộ test | Chạy trên | Kiểm tra |
|---|---|---|
| `tests/live/test_live_commands.py` | Word/Excel/PowerPoint thật (`--wps`: WPS) | Mọi lệnh bridge và lỗi tham số; bảo mật HTTP (401/403/415); `--ai`: `ai.ask` thật trên 3 app, không tự lưu/xuất; offline: `office_action` từ chối lệnh ngoài danh sách (reflection vào DLL); `--record` / `--compare`: so kết quả từng lệnh với bản ghi golden |
| `tests/mcp-host/test_mcp_host.py` | Offline | Giao thức MCP qua client chính thức; so kết quả 50 tool với bản Python cũ trên cùng file; mở lại file C# ghi bằng python-docx / openpyxl / python-pptx; README khớp `commands --markdown`; mô tả `*_command` đủ lệnh |
| `tests/mcp-host/office_roundtrip.ps1` | Office thật | File do Office tạo đọc được bằng C# và ngược lại |
| `tools/*-mcp/tests` | Offline | Unit test của 3 MCP server Python cũ |

- **Cách ly**: test live chỉ chạy trên app do chính nó mở; port đã có bridge của app người dùng
  thì dừng (test gọi `writer.closeAll`); chạy xong đóng app bằng WM_CLOSE, quá 20s mới kill.
- **Golden**: `--record` lưu kết quả từng lần gọi (đã bỏ pid, thời gian, thư mục tạm);
  `--compare` so các khóa có trong golden, khóa mới chỉ liệt kê. Dùng để chứng minh refactor không
  đổi hành vi.
- **Độ phủ**: test đọc `commands --json`; lệnh nào trong registry chưa được gọi thì báo lỗi.

---

## Phụ lục: thuật ngữ

| Thuật ngữ | Nghĩa |
|---|---|
| **Bridge** | HTTP server loopback trong mỗi instance app (hoặc companion) |
| **`AppKind`** | Loại app logic: `wps` (Word/Writer), `et` (Excel/Spreadsheets), `wpp` (PowerPoint/Presentation) |
| **`family`** | `office` (Microsoft Office) hoặc `wps` (WPS Office) |
| **Lệnh (command / action)** | Một thao tác `POST /cmd`, vd `et.writeRange`; khai báo trong registry |
| **Registry lệnh** | `CommandDispatcher.Commands`: nguồn duy nhất về tên, loại app, tham số, handler của lệnh |
| **`ForAgent`** | Cờ cho phép agent của Ask AI gọi lệnh |
| **Làn live / làn file** | MCP tool thao tác app đang mở qua bridge / sửa file trên đĩa không cần app |
| **Companion** | `AxiomOffice.Host.exe word|excel|...`: tự mở app qua COM và chạy bridge, không cần add-in |
| **CTP** | Custom Task Pane: pane dock của Office/WPS |
| **`ComGate`** | Cổng tuần tự hoá mọi truy cập COM của add-in |
| **Golden** | Bản ghi kết quả từng lệnh dùng để so trước/sau refactor |
