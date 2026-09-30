# Agent Core viết lại bằng Go (`core-go/`)

Bản Go của `src/AxiomOffice.Core` (.NET 10), để phát hành **một file binary nhỏ, không cần runtime**
(~11 MB sau `-s -w`, so với ~52 MB single-file .NET hoặc 19 MB Native AOT — bản AOT làm hỏng JSON reflection).
Hai bản **dùng chung hợp đồng**: cùng Core API v1, cùng `core.json`, cùng `core.db` (schema 2), cùng nguồn
cấu hình (`AXIOM_*` → HKCU / `~/.config/axiom-office/config.json` → mặc định), cùng tên mutex một-phiên-bản.
Nhờ vậy add-in, extension LibreOffice và MCP không phải đổi gì khi thay bản Core.

**Cách làm: song song dần.** Bản .NET vẫn là bản phát hành. Bản Go chỉ thay được khi qua **toàn bộ**
`tests/core/test_core_e2e.py`; mỗi giai đoạn chạy các phần tương ứng bằng `--only`.

| Giai đoạn | Nội dung | Phần e2e | Trạng thái |
|---|---|---|---|
| G1 | cấu hình (HKCU/DPAPI, config.json/libsecret), `core.log`, `core.json`, một-phiên-bản, chọn port, guard (Origin/token/Content-Type), `/health`, `/v1/admin/shutdown`, migrate `core.db`, model client + codec OpenAI/Anthropic + vòng lặp agent, `/v1/setup`, `/v1/llm/test`, `/v1/llm/models` | `/health`, `setup` | ✔ (trừ đếm skill, chờ G3) |
| G2 | `/v1/runs` + SSE, orchestrator, bridge client, session registry, `office_action`, policy + xác nhận, audit, hội thoại | `fake_bridge`, `guards`, `confirm` | |
| G3 | skills (`load_skill`, `read_skill_file`, `/v1/skills`, theo dõi thư mục) | `skills` | |
| G4 | memory dài hạn (FTS5, trích xuất, embedding) | `memory` | |
| G5 | MCP client + QA thị giác | `mcp`, `visual` | |
| G6 | đóng gói thay bản .NET (`build.ps1`, `scripts/linux/package.sh`, CI) | toàn bộ | |

## Build

Cần Go ≥ 1.26 (máy có Go cũ hơn tự tải toolchain nhờ `GOTOOLCHAIN=auto`). Không dùng cgo: SQLite là
`modernc.org/sqlite` (thuần Go) nên build chéo được từ một máy.

```bash
cd core-go
go test ./...
go build -trimpath -ldflags "-s -w -X main.version=1.2.3" -o axiom-core.exe ./cmd/axiom-core
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o axiom-core ./cmd/axiom-core
```

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
| `internal/api` | `Api/CoreApi.cs`, `CoreApiGuard.cs`, `ApiJson.cs`, `SetupEndpoints.cs` |

Khác biệt có chủ ý: bản Go **giữ luôn listener** của port tìm được (bản .NET thả port rồi Kestrel mới bind lại,
có khe để tiến trình khác chiếm) và dừng êm khi nhận SIGTERM (systemd `--user`).
