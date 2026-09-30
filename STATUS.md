# Tình trạng dự án Axiom Office

Cập nhật: **30/09/2026** · nhánh `main` (98 commit) · cây làm việc sạch · **chưa push remote**

Tài liệu này là ảnh chụp nhanh "đã làm được gì / chưa làm được gì". Chi tiết thiết kế nằm ở
[New_arch.md](New_arch.md) (Core + add-in) và [LibreOffice_arch.md](LibreOffice_arch.md) (LibreOffice);
hướng dẫn dùng ở [README.md](README.md); lịch sử thay đổi ở [CHANGELOG.md](CHANGELOG.md).

## 1. Bốn thành phần và trạng thái

| Thành phần | Nền tảng | Trạng thái |
|---|---|---|
| **Add-in COM** (`src/AxiomOffice`, net48) | Word/Excel/PowerPoint + WPS, Windows | Chạy được; đã kiểm chứng trên Office/WPS thật ở các đợt trước. Đợt này **chưa** chạy lại làn `--office` |
| **Agent Core** (`core-go/`, Go) | Windows + Linux | **Đã thay bản .NET làm bản phát hành** (G1–G10), qua toàn bộ e2e của bản .NET |
| **Agent Core cũ** (`src/AxiomOffice.Core`, .NET 10) | Windows + Linux | Vẫn nằm nguyên trong repo, dùng để đối chiếu (`build.ps1 -Core dotnet`); bộ test xUnit của nó vẫn chạy |
| **MCP server** (`src/AxiomOffice.Mcp`, net10) | Windows + Linux | Không đổi trong đợt này; parity 61/61 |
| **Extension LibreOffice** (Python UNO) | Linux (LibreOffice) | Chạy được; 159/159 trên LibreOffice thật + wizard 20/20 với Core Go |
| **Wizard thiết lập** | Windows (WinForms) + Linux (awt) | Bản Linux đã chạy thật (20/20); **bản Windows mới chỉ biên dịch sạch, chưa bấm tay trong Office** |

## 2. Đã làm được

### 2.1 Agent Core viết lại bằng Go (`core-go/`) — G1 → G10

Bản Go dùng **chung hợp đồng** với bản .NET: cùng Core API v1, cùng `core.json`, cùng `core.db` (schema 2),
cùng nguồn cấu hình (`AXIOM_*` → HKCU/DPAPI hoặc `~/.config/axiom-office/config.json` + libsecret), cùng tên
mutex một-phiên-bản. Nhờ vậy add-in/extension/MCP không phải đổi gì.

| Mốc | Nội dung |
|---|---|
| G1 | Cấu hình, `core.log`, `core.json`, một-Core-mỗi-người-dùng, chọn port, guard (Origin 403 / token 401 / Content-Type 415), `/health`, `/v1/admin/shutdown`, migrate `core.db`, model client + codec OpenAI/Anthropic, `/v1/setup`, `/v1/llm/test`, `/v1/llm/models` |
| G2 | `/v1/runs` + SSE + hủy + xác nhận, orchestrator, bridge client, session registry, `office_action`, policy, audit, hội thoại |
| G3 | Skills 3 tầng: `load_skill`, `read_skill_file`, `GET /v1/skills`, `POST /v1/skills/reload`, quét lại thư mục |
| G4 | Memory dài hạn: chống trùng (hash + cosine 0.96), liên kết chuyển đổi, lịch sử, ghim/hạn dùng/xoá mềm, truy hồi bm25 + entity boost + embedding, trích xuất sau mỗi lượt, toàn bộ `/v1/memory`, tool `remember`/`recall` |
| G5 | MCP client (stdio + Streamable HTTP, server built-in `office` = `AxiomOffice.Host.exe mcp`, xác nhận tool ngoài/ghi đè file), QA thị giác `look_at_document`, `GET /v1/mcp` |
| G6 | Đóng gói: `build.ps1` build Core Go thành `AxiomOffice.Core.exe`; `package.sh` đóng gói `core/AxiomOffice.Core`; CI chạy cả hai |
| G7 | **Vá lỗ hổng**: Core dừng khi đang chạy lượt → hủy lượt trước khi đóng HTTP server (trước đây agent có thể sửa dở tài liệu) |
| G8 | Phủ codec Anthropic (chạy thật) và embedding (cosine + tìm theo nghĩa) |
| G9 | Phủ tóm tắt hội thoại dài (chỉ khi vượt mốc 20 tin nhắn) |
| G10 | Phủ nút **Dừng** (`/cancel`), `interactive=false` (`ai.ask`), PATCH memory, xoá cứng toàn bộ |
| G11 | Phủ MCP transport **HTTP** (`mcp.json` dạng `url`): session id, response JSON và SSE |

Kích thước: Core Go **11,9 MB** (bản .NET self-contained: 51,6 MB), không cần runtime trên máy người dùng.

### 2.2 Wizard thiết lập cho người dùng không chuyên

- **Core API**: `GET /v1/setup`, `POST /v1/llm/test`, `GET /v1/llm/models` + `Setup/LlmErrors.cs` dịch lỗi sang
  tiếng Việt kèm gợi ý sửa. Khoá API không vào log/response.
- **Linux**: `axiom/setup.py` (máy trạng thái, test được không cần LibreOffice) + `setupwizard.py` (dialog awt),
  mục menu **Thiết lập…**, link header, tự mở một lần khi chưa cấu hình.
- **Windows**: `Ai/SetupWizardForm.cs` + `Ai/CoreSetup.cs`, ribbon **Thiết lập…** và **Cài đặt nâng cao…**,
  link header pane, tự mở một lần, bước 5 có **Thử ngay** (chạy một lượt thật qua Core).
- Câu chữ + preset nằm ở một nguồn `catalog/setup.json` → sinh ra `SetupCatalog.cs` (C#, dùng chung add-in và
  Core), `axiom/setup_catalog.py` (Python) và `core-go/internal/setup/catalog_gen.go` (Go); CI kiểm tra không lệch.

### 2.3 Kiểm chứng (số liệu mới nhất, 30/09/2026)

| Bộ kiểm thử | Kết quả |
|---|---|
| e2e Core — **bản Go** (14 phần, Windows) | **155/155** |
| e2e Core — **bản .NET** (đối chiếu) | **155/155** |
| e2e trên **Linux** đúng lệnh CI (13 phần) | **143/143** |
| LibreOffice thật (gói mới, Core Go) | **159/159** |
| Wizard thiết lập (dialog thật, X ảo) | **20/20** |
| `go test ./...` (core-go) | 10/10 gói |
| .NET xUnit · `tests/lo` · MCP parity | 228/228 · 89 · 61/61 |
| `--real-llm` (model thật trong HKCU) | **4/4** — nạp đúng skill (`bao-cao-thang`, `bang-diem`, `van-ban-hanh-chinh`), câu "in đậm" không nạp skill thiết kế |
| Đối chiếu khoá API DPAPI với bản .NET | cùng plaintext (dài 35, sha256 `bec24e97c4893251`) |
| libsecret (keyring Linux, qua `secret-tool` giả) | đọc được khoá; thiếu khoá → coi như chưa cấu hình |
| `systemd --user` (Linux) | `enabled` + `active`, `/health` OK, đúng port 47840 |
| Đóng gói | Windows: `bin\Release` có Core Go 1.0.0 + DLL + Host + skills; Linux: tar.gz 39 MB với `core/AxiomOffice.Core` là ELF chạy độc lập |

### 2.4 CI

`.github/workflows/ci.yml`: job **linux** chạy unit test extension, xUnit, build MCP, `go vet`/`go test`, e2e 12
phần với binary Go, so sánh `prompts/extract.txt` giữa hai bản, cài LibreOffice rồi chạy bộ live + wizard, và
đóng gói tarball; job **windows** chạy unit test, xUnit, `go test`, build net48, và **toàn bộ e2e của bản Go**
(kèm `AxiomOffice.Host.exe` như gói phát hành).

## 3. Chưa làm / chưa kiểm chứng

| Việc | Tình trạng | Ghi chú |
|---|---|---|
| **Wizard Windows bấm tay trong Word/Excel/WPS** | Chưa | Đã biên dịch sạch và dựng vào `bin\Release`; cần bạn mở app bấm thử (máy này không chạy Office) |
| e2e `--office` (Office thật + add-in, `test_office`) | Chưa chạy trong đợt này | Cần mở Excel thật; đã có từ các đợt trước |
| Trích xuất memory với **LLM thật** | Chưa | Đã kiểm bằng LLM giả (e2e) và unit test `ParseFacts`; chưa chạy model thật |
| Đọc/ghi **HKCU** bằng test tự động trên Windows | Một phần | Đối chiếu tay: Core Go đọc đúng endpoint/model/token/DPAPI của bạn; test tự động hiện dùng `AXIOM_*` |
| **Push remote** | Chưa | Bạn yêu cầu chỉ commit |
| Merge `feat/setup-wizard` + `feat/core-go` vào `main` | **Đã xong** | Fast-forward, lịch sử thẳng |
| Bản Core .NET | Giữ lại | Không còn là bản phát hành nhưng vẫn build/test được để đối chiếu; hai bản chia sẻ `SetupCatalog.cs` và `prompts/extract.txt` nên sửa Core dùng chung phải chạy cả hai bộ |
| Nền tảng khác | Chưa | Chỉ build được Windows (amd64) và Linux (amd64/arm64); chưa có macOS hay Windows ARM |
| Embedding với máy chủ thật | Chưa | Chỉ kiểm bằng máy chủ giả trả vector điều khiển được |

## 4. Khác biệt có chủ ý giữa Core Go và Core .NET

Ghi ở [core-go/README.md](core-go/README.md); tóm tắt:

1. Bản Go **giữ luôn listener** của port tìm được (bản .NET thả port rồi Kestrel bind lại — có khe để tiến
   trình khác chiếm).
2. Bản Go **dừng êm khi nhận SIGTERM** (dùng cho `systemd --user`).
3. Bản Go **hủy lượt đang chạy trước khi đóng HTTP server** (bản .NET hủy sau khi server đã dừng) → pane còn
   nhận được `run.cancelled`.
4. `conversationId` gửi kèm khoảng trắng được cắt bỏ (bản .NET coi là hội thoại mới).
5. `Normalize` của memory dùng bảng bỏ dấu tiếng Việt tường minh (giống bản .NET); chữ Latin có dấu khác chỉ
   được bỏ dấu nếu văn bản đã ở dạng phân rã sẵn.

## 5. Lệnh hay dùng

```powershell
scripts\build.ps1                 # add-in + Host + Agent Core (Go) vào src\AxiomOffice\bin\Release
scripts\build.ps1 -Core dotnet    # đóng gói Core .NET thay vì bản Go (đối chiếu)
scripts\install.ps1               # đăng ký add-in (HKCU), in cấu hình MCP
scripts\package.ps1               # zip phát hành
```

```bash
cd core-go && go test ./... && go build -o axiom-core.exe ./cmd/axiom-core     # Core Go
AXIOM_E2E_CORE_EXE=<binary> python tests/core/test_core_e2e.py [--only <phần,...>]
bash scripts/linux/package.sh     # gói Linux (AXIOM_CORE=dotnet để dùng bản .NET)
```

Các phần e2e chạy riêng được: `fake_bridge`, `guards`, `skills`, `memory`, `confirm`, `mcp`, `visual`,
`setup`, `shutdown`, `anthropic`, `embeddings`, `summarize`, `cancel`, `mcp_http`.

## 6. Việc nên làm tiếp (đề xuất)

1. **Bạn bấm thử wizard Windows** trong Word/Excel/WPS (5 bước, nhất là **Thử ngay**) rồi báo lại.
2. Chạy `--real-llm` cho **memory** (trích xuất bằng model thật) khi muốn chắc chắn chất lượng fact.
3. Cân nhắc **bỏ hẳn bản Core .NET** sau một thời gian chạy thật ổn định (khi đó `SetupCatalog.cs` chỉ còn
   dùng cho add-in, và `prompts/extract.txt` chỉ còn một bản).
