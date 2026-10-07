# Agent Core viết lại bằng Go (`core-go/`)

Agent Core của Axiom Office, phát hành dưới dạng **một file binary nhỏ, không cần runtime** (~12 MB sau
`-s -w`, thay cho ~52 MB single-file của bản .NET cũ hoặc 19 MB Native AOT — bản AOT làm hỏng JSON reflection).
Bản .NET (`src/AxiomOffice.Core`) đã được **bỏ khỏi repo ngày 01/10/2026**; `git log` còn giữ lịch sử.

Hợp đồng với phần còn lại **không đổi**: Core API v1, `core.json`, `core.db` (schema 2), nguồn cấu hình
(`AXIOM_*` → HKCU / `~/.config/axiom-office/config.json` → mặc định), tên mutex một-phiên-bản. Nhờ vậy
add-in, extension LibreOffice và MCP không phải đổi gì.

**Cách làm: song song dần.** Bản Go đã qua **toàn bộ** `tests/core/test_core_e2e.py` (155 kiểm tra) và được
đóng gói phát hành từ G6 (`scripts/build.ps1`, `scripts/linux/package.sh`). Trước khi bỏ bản .NET, bộ e2e đã
chạy trên **cả hai** bản (155/155 mỗi bản) để chắc không lệch hành vi; bốn vùng test chỉ bản .NET có
(policy, xoay vòng log, `core.json`, chọn port + khoá một-phiên-bản) đã được port sang Go trước khi xoá.

| Giai đoạn | Nội dung | Phần e2e | Trạng thái |
|---|---|---|---|
| G1 | cấu hình (HKCU/DPAPI, config.json/libsecret), `core.log`, `core.json`, một-phiên-bản, chọn port, guard (Origin/token/Content-Type), `/health`, `/v1/admin/shutdown`, migrate `core.db`, model client + codec OpenAI/Anthropic + vòng lặp agent, `/v1/setup`, `/v1/llm/test`, `/v1/llm/models` | `/health`, `setup` | ✔ |
| G2 | `/v1/runs` + SSE, orchestrator, bridge client, session registry, `office_action`, policy + xác nhận, audit, hội thoại | `fake_bridge`, `guards`, `confirm` | ✔ |
| G3 | skills (`load_skill`, `read_skill_file`, `/v1/skills`, theo dõi thư mục) | `skills` | ✔ |
| G4 | memory dài hạn (FTS5, trích xuất, embedding) | `memory` | ✔ |
| G5 | MCP client + QA thị giác | `mcp`, `visual` | ✔ |
| G6 | đóng gói thay bản .NET (`build.ps1`, `scripts/linux/package.sh`, CI) | toàn bộ | ✔ |
| G7 | vá lỗ hổng phát hiện khi soát lại: hủy lượt đang chạy lúc Core dừng | `shutdown` | ✔ |
| G8 | phủ nốt hai vùng chưa có e2e: codec Anthropic (chạy thật) và embedding (cosine + tìm theo nghĩa) | `anthropic`, `embeddings` | ✔ |
| G9 | phủ đường tóm tắt hội thoại dài (chỉ khi vượt mốc 20 tin nhắn) | `summarize` | ✔ |
| G10 | phủ nút Dừng, `interactive=false` (`ai.ask`), PATCH memory và xóa cứng toàn bộ | `cancel`, `confirm`, `memory` | ✔ |
| G11 | phủ MCP transport HTTP (URL server, session id, response JSON + SSE) | `mcp_http` | ✔ |
| G12 | **MCP server bản Linux**: subcommand `mcp [all|word|excel|ppt]`, 50 tool (20 file + 30 live) | `test_mcp_portable.py`, `test_mcp_host.py` | ✔ |

## Build

Cần Go ≥ 1.26 (máy có Go cũ hơn tự tải toolchain nhờ `GOTOOLCHAIN=auto`). Không dùng cgo: SQLite là
`modernc.org/sqlite` (thuần Go) nên build chéo được từ một máy.

```bash
cd core-go
go test ./...
go build -trimpath -ldflags "-s -w -X main.version=1.2.3" -o axiom-core.exe ./cmd/axiom-core
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o axiom-core ./cmd/axiom-core
```

## MCP server bản Linux (`AxiomOffice.Core mcp`)

Trên Windows, MCP server nằm trong `AxiomOffice.Host.exe mcp` (net48, `src/AxiomOffice.Host/Mcp/`).
Trên Linux, nó là **subcommand của chính binary Core** — không thêm file nào vào gói:

```bash
AxiomOffice.Core mcp [all|word|excel|ppt] [--list]
```

Đúng cùng hợp đồng với bản C#: JSON-RPC 2.0 mỗi message một dòng qua stdio, stdout chỉ dành cho giao
thức, 50 tool (20 tool file docx/xlsx/pptx/csv + 30 tool live gọi bridge) và `office_sessions`.

- Subcommand được chặn **trước** mutex một-phiên-bản: Core sinh tiến trình con này bằng chính binary của
  nó, nên nó không được đi qua nhánh khoá.
- `mcp.LoadConfigs` không đổi: server built-in `office` vẫn là `{command, args: ["mcp"]}`, chỉ khác
  `command` — Windows là `AxiomOffice.Host.exe`, Linux là chính binary Core (`officeMcpHost`).
- Engine OOXML (`internal/ooxml`, `internal/xlsx`, `internal/docx`, `internal/pptx`, `internal/sheet`) là
  bản port của `src/AxiomOffice.Host/Mcp/*.cs`; `tests/mcp-host/oracle_diff.py` chạy cùng một kịch bản
  trên **cả hai** bản và so từng bước để phát hiện lệch.

## Kiểm thử bằng e2e dùng chung với bản .NET

```bash
# Chép skills/ cạnh binary (giống gói phát hành) rồi chạy các phần đã port:
AXIOM_E2E_CORE_EXE=/duong/dan/axiom-core python tests/core/test_core_e2e.py --only setup
```

## Bố cục

| Gói | Tương ứng bản .NET |
|---|---|
| `cmd/axiom-core` | `Program.cs`, `CoreRuntime.cs` |
| `internal/config` | `Config/*` (`CoreConfig`, `CorePaths`, `RegistrySource`, `JsonConfigSource`, `Secrets`) |
| `internal/corelog`, `internal/corefile` | `Logging/CoreLog.cs`, `Config/CoreFile.cs` |
| `internal/instance` | mutex `Local\AxiomOffice.Core` (Windows) / `flock` trong `$XDG_RUNTIME_DIR/axiom-office` (Linux) |
| `internal/store` | `Memory/CoreDb.cs`, `Api/CoreStores.cs` |
| `internal/model` | `Models/*` |
| `internal/setup` | `Setup/LlmErrors.cs` + catalog sinh từ `catalog/setup.json` (`scripts/generate_setup_catalog.py`) |
| `internal/office` | `Office/SessionDirectory.cs`, `Office/BridgeClient.cs` |
| `internal/skills` | `Skills/SkillIndex.cs`, `SkillLoader.cs` (phần quy tắc file nằm trong `Tools/SkillTools.cs`) |
| `internal/policy` | `Agent/PolicyEngine.cs` (tách riêng gói để `tools` và `agent` dùng chung, không vòng import) |
| `internal/tools` | `Tools/ITool.cs`, `OfficeActionTool.cs`, `SkillTools.cs` |
| `internal/agent` | `Agent/Orchestrator.cs`, `RunManager.cs`, `RunEventStream.cs`, `PolicyEngine.cs` (phần xác nhận), `PromptBuilder.cs`, `ContextAssembler.cs` |
| `internal/api` | `Api/CoreApi.cs`, `CoreApiGuard.cs`, `ApiJson.cs`, `SetupEndpoints.cs`, `RunEndpoints.cs`, `SkillEndpoints.cs` |
| `internal/store` | `Memory/CoreDb.cs`, `ConversationStore.cs`, `RunStore.cs`, `Api/CoreStores.cs` |
| `internal/memory` | `Memory/MemoryText.cs`, `MemoryStore.cs`, `MemoryRetriever.cs`, `MemoryExtractor.cs`, `MemoryService.cs` |
| `internal/mcp` | `Mcp/McpTransport.cs`, `McpManager.cs`, `Tools/VisualTool.cs` (phần quy tắc đã tách sang `internal/tools`) |
| `internal/mcpserver` | **MCP server** (subcommand `mcp`): `McpServer.cs`, `McpHost.cs`, `LiveTools.cs`, `WordFiles.cs`, `ExcelFiles.cs`, `PptFiles.cs` |
| `internal/ooxml` | `OoxmlPackage.cs` (engine gói OOXML: zip, part, rels, content-types) |
| `internal/sheet` | `Cells.cs`, `CsvTable.cs` |
| `internal/xlsx` | `XlsxBook.cs`, `XlsxStyles.cs`, `XlsxTemplate` |
| `internal/docx`, `internal/pptx` | `WordFiles.cs` (phần nội dung), `PptFiles.cs` |
| `internal/filesafe` | `FileSafety` (lưu file kiểu atomic) |
| `internal/textutil` | đếm/cắt chuỗi theo đơn vị UTF-16 cho khớp `String.Length` của C# |
| `internal/templates` | `Templates.Docx`/`Templates.Pptx` (template python-docx/python-pptx, MIT — xem `NOTICE.md` cạnh đó) |

`prompts/extract.txt` (prompt của bộ trích xuất memory) là **nguồn duy nhất** trong repo, nhúng vào binary
bằng `go:embed`.

Khác biệt có chủ ý: bản Go **giữ luôn listener** của port tìm được (bản .NET thả port rồi Kestrel mới bind lại,
có khe để tiến trình khác chiếm), dừng êm khi nhận SIGTERM (systemd `--user`), và **hủy các lượt đang chạy
trước khi đóng HTTP server** (bản .NET hủy sau khi server đã dừng) nên pane còn nhận được `run.cancelled`.
