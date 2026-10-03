# Tình trạng dự án Axiom Office

Cập nhật: **03/10/2026** · nhánh `main` · cây làm việc sạch · **7 commit chưa push remote**
(chỉ commit, không push — theo yêu cầu của bạn)

Tài liệu này là ảnh chụp nhanh "đã làm được gì / chưa làm được gì". Chi tiết thiết kế nằm ở
[New_arch.md](New_arch.md) (Core + add-in) và [LibreOffice_arch.md](LibreOffice_arch.md) (LibreOffice);
hướng dẫn dùng ở [README.md](README.md); lịch sử thay đổi ở [CHANGELOG.md](CHANGELOG.md).

## 1. Bốn thành phần và trạng thái

| Thành phần | Nền tảng | Trạng thái |
|---|---|---|
| **Add-in COM** (`src/AxiomOffice`, net48) | Word/Excel/PowerPoint + WPS, Windows | Chạy được; đã kiểm chứng trên Office/WPS thật ở các đợt trước. Đợt này **chưa** chạy lại làn `--office` |
| **Agent Core** (`core-go/`, Go) | Windows + Linux | **Đã thay bản .NET làm bản phát hành** (G1–G10), qua toàn bộ e2e của bản .NET |
| **MCP server** (`core-go/internal/mcpserver`, Go) | Windows + Linux | **Một bản duy nhất**, chạy bằng `AxiomOffice.Core.exe mcp`; 61/61 trên cả hai nền tảng (và 103/103 ở bộ parity sâu). Bản C# đã xoá; `AxiomOffice.Host.exe mcp` còn 232 dòng **chuyển tiếp byte** sang Core nên cấu hình cũ không phải đổi |
| **Extension LibreOffice** (Python UNO) | Linux (LibreOffice) | Chạy được; 281/281 trên LibreOffice thật + wizard 20/20 với Core Go |
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
| G5 | MCP client (stdio + Streamable HTTP, server built-in `office` = `AxiomOffice.Core.exe mcp`, xác nhận tool ngoài/ghi đè file), QA thị giác `look_at_document`, `GET /v1/mcp` |
| G6 | Đóng gói: `build.ps1` build Core Go thành `AxiomOffice.Core.exe`; `package.sh` đóng gói `core/AxiomOffice.Core`; CI chạy cả hai |
| G7 | **Vá lỗ hổng**: Core dừng khi đang chạy lượt → hủy lượt trước khi đóng HTTP server (trước đây agent có thể sửa dở tài liệu) |
| G8 | Phủ codec Anthropic (chạy thật) và embedding (cosine + tìm theo nghĩa) |
| G9 | Phủ tóm tắt hội thoại dài (chỉ khi vượt mốc 20 tin nhắn) |
| G10 | Phủ nút **Dừng** (`/cancel`), `interactive=false` (`ai.ask`), PATCH memory, xoá cứng toàn bộ |
| G11 | Phủ MCP transport **HTTP** (`mcp.json` dạng `url`): session id, response JSON và SSE |

Kích thước: Core Go **11,9 MB** (bản .NET self-contained: 51,6 MB), không cần runtime trên máy người dùng.

### 2.2 MCP server bản Linux chuyển sang Go (`core-go/internal/mcpserver/`) — G12

Bản C# đa nền tảng (`src/AxiomOffice.Mcp`, net10 — biên dịch lại `src/AxiomOffice.Host/Mcp/*.cs` với ký
hiệu `PORTABLE`) đã được thay bằng subcommand `AxiomOffice.Core mcp`; dự án .NET 10 cùng hai lớp shim
`Compat/` đã bị xoá, và các nhánh `#if PORTABLE` trong mã C# dùng chung cũng được dọn (bản C# còn đúng
một cấu hình: net48 cho Windows).

| Phần | Nội dung |
|---|---|
| Giao thức | JSON-RPC 2.0 mỗi message một dòng qua stdio, stdout chỉ dành giao thức, batch, `-32700/-32600/-32601/-32602`, `initialize`/`ping`/`tools/list`/`tools/call` |
| Engine OOXML | `ooxml` (zip/part/rels/content-types, chỉ ghi lại part bị sửa), `xlsx` (shared strings, style, bảng, công thức), `docx`, `pptx`, `sheet` (A1, CSV/TSV) |
| 20 tool file | `doc_*` (5), `excel_*` (11), `ppt_*` (4) — tên và tham số giống hệt bản C# |
| 30 tool live | `word_*` (14), `ppt_*` (11), `wps_*` (4) + `office_sessions`; gọi bridge qua HTTP, cổng tra từ cấu hình |
| Nguồn dùng chung | `catalog/live-commands.json` → sinh `core-go/internal/mcpserver/livecommands_gen.go`; template `default.docx`/`default.pptx` giữ một nguồn, Go giữ bản sao có `--check` trong CI |
| Đo lệch | `tests/mcp-host/oracle_diff.py` chạy cùng kịch bản trên bản Go và bản C#, so từng bước |

Hai chỗ cố ý giữ nguyên hành vi của bản C# dù trông như lỗi: `ReplaceSheetReference` thay dạng **có**
nháy bằng tên mới **không** nháy khi tên đơn giản, và `IsDateFormat` chỉ xét phần trước dấu `;` đầu tiên.
Cả hai đều nằm trong hợp đồng mà bộ parity kiểm tra, nên sửa chúng sẽ làm lệch bản Windows.

### 2.3 Wizard thiết lập cho người dùng không chuyên

- **Core API**: `GET /v1/setup`, `POST /v1/llm/test`, `GET /v1/llm/models` + `Setup/LlmErrors.cs` dịch lỗi sang
  tiếng Việt kèm gợi ý sửa. Khoá API không vào log/response.
- **Linux**: `axiom/setup.py` (máy trạng thái, test được không cần LibreOffice) + `setupwizard.py` (dialog awt),
  mục menu **Thiết lập…**, link header, tự mở một lần khi chưa cấu hình.
- **Windows**: `Ai/SetupWizardForm.cs` + `Ai/CoreSetup.cs`, ribbon **Thiết lập…** và **Cài đặt nâng cao…**,
  link header pane, tự mở một lần, bước 5 có **Thử ngay** (chạy một lượt thật qua Core).
- Câu chữ + preset nằm ở một nguồn `catalog/setup.json` → sinh ra `SetupCatalog.cs` (C#, dùng chung add-in và
  Core), `axiom/setup_catalog.py` (Python) và `core-go/internal/setup/catalog_gen.go` (Go); CI kiểm tra không lệch.

### 2.4 Kiểm chứng (số liệu mới nhất, 01/10/2026)

| Bộ kiểm thử | Kết quả |
|---|---|
| e2e Core — **bản Go** (14 phần, Windows) | **155/155** |
| e2e Core — **bản .NET** (đối chiếu) | **155/155** |
| e2e trên **Linux** đúng lệnh CI (13 phần) | **143/143** |
| LibreOffice thật (gói mới, Core Go) | **159/159** |
| Wizard thiết lập (dialog thật, X ảo) | **20/20** |
| `go test ./...` (core-go) | 18 gói |
| `tests/lo` · MCP parity | 106 |
| `test_mcp_portable.py` — binary Go (`AxiomOffice.Core mcp all`) | **61/61** |
| `test_mcp_portable.py` — `AxiomOffice.Host.exe mcp all` (Windows; nay là cửa CHUYỂN TIẾP sang Core, bản C# đã xoá) | **61/61** |
| `test_mcp_host.py` — parity sâu với python-docx/openpyxl/python-pptx, binary Go | **103/103** |
| `oracle_diff.py` — Go ↔ C# trên cùng kịch bản (docx/pptx/excel) | 61/62 (1 khác là câu chữ thông báo `parquet`, cố ý) |
| `--real-llm` (model thật trong HKCU) | **4/4** — nạp đúng skill (`bao-cao-thang`, `bang-diem`, `van-ban-hanh-chinh`), câu "in đậm" không nạp skill thiết kế |
| Đối chiếu khoá API DPAPI với bản .NET | cùng plaintext (dài 35, sha256 `bec24e97c4893251`) |
| libsecret (keyring Linux, qua `secret-tool` giả) | đọc được khoá; thiếu khoá → coi như chưa cấu hình |
| `systemd --user` (Linux) | `enabled` + `active`, `/health` OK, đúng port 47840 |
| Đóng gói | Windows: `bin\Release` có Core Go 1.0.0 + DLL + Host + skills; Linux: tar.gz 39 MB với `core/AxiomOffice.Core` là ELF chạy độc lập |

### 2.5 CI

`.github/workflows/ci.yml`: job **linux** không còn cài .NET — chạy unit test extension, kiểm ba file sinh
không lệch nguồn, `go vet`/`go test`, e2e **14 phần** (đã gồm `mcp`, vì Core tự làm MCP server), bộ MCP
portable nhắm vào binary Go, rồi cài LibreOffice chạy bộ live + wizard và đóng gói tarball. Job **windows**
chạy unit test, `go test`, build net48, **toàn bộ e2e của bản Go** (kèm `AxiomOffice.Host.exe` như gói phát
hành) và bộ MCP portable nhắm vào `AxiomOffice.Host.exe mcp`.

## 3. Chưa làm / chưa kiểm chứng

| Việc | Tình trạng | Ghi chú |
|---|---|---|
| **Wizard Windows bấm tay trong Word/Excel/WPS** | Chưa | Đã biên dịch sạch và dựng vào `bin\Release`; cần bạn mở app bấm thử (máy này không chạy Office) |
| e2e `--office` (Office thật + add-in, `test_office`) | Chưa chạy trong đợt này | Cần mở Excel thật; đã có từ các đợt trước |
| Trích xuất memory với **LLM thật** | Chưa | Đã kiểm bằng LLM giả (e2e) và unit test `ParseFacts`; chưa chạy model thật |
| Đọc/ghi **HKCU** bằng test tự động trên Windows | Một phần | Đối chiếu tay: Core Go đọc đúng endpoint/model/token/DPAPI của bạn; test tự động hiện dùng `AXIOM_*` |
| **Push remote** | Chưa | Bạn yêu cầu chỉ commit |
| Merge `feat/setup-wizard` + `feat/core-go` vào `main` | **Đã xong** | Fast-forward, lịch sử thẳng |
| Nền tảng khác | Chưa | Chỉ build được Windows (amd64) và Linux (amd64/arm64); chưa có macOS hay Windows ARM |
| Embedding với máy chủ thật | Chưa | Chỉ kiểm bằng máy chủ giả trả vector điều khiển được |

## 4. Ghi chú hành vi (khác bản .NET đã bỏ ngày 01/10/2026)

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
scripts\install.ps1               # đăng ký add-in (HKCU), in cấu hình MCP
scripts\package.ps1               # zip phát hành
```

```bash
cd core-go && go test ./... && go build -o axiom-core.exe ./cmd/axiom-core     # Core Go
AXIOM_E2E_CORE_EXE=<binary> python tests/core/test_core_e2e.py [--only <phần,...>]
bash scripts/linux/package.sh     # gói Linux (Core Go + .oxt; MCP nằm trong Core)

# MCP server (cả hai nền tảng; Host.exe mcp chỉ chuyển tiếp sang Core):
AXIOM_MCP_ARGS="mcp all" python tests/mcp-host/test_mcp_portable.py <binary>
python tests/mcp-host/oracle_diff.py    # đối chiếu Go ↔ C# trên cùng kịch bản
```

Các phần e2e chạy riêng được: `fake_bridge`, `guards`, `skills`, `memory`, `confirm`, `mcp`, `visual`,
`setup`, `shutdown`, `anthropic`, `embeddings`, `summarize`, `cancel`, `mcp_http`.

## 6. Việc nên làm tiếp (đề xuất)

1. **Bạn bấm thử wizard Windows** trong Word/Excel/WPS (5 bước, nhất là **Thử ngay**) rồi báo lại.
2. Chạy `--real-llm` cho **memory** (trích xuất bằng model thật) khi muốn chắc chắn chất lượng fact.
3. Dọn `tools/` (các MCP server Python cũ) nếu không còn dùng.
4. Chạy `test_mcp_host.py` (parity sâu) **trên Linux** — đợt này mới chạy được trên Windows; máy Linux
   không kết nối được lúc kiểm. Trên Linux cần venv có `mcp`/`python-docx`/`openpyxl`/`python-pptx`
   (xem ghi chú máy kiểm chứng) và đặt `AXIOM_MCP_NO_CATALOG=1`.
5. `src/AxiomOffice.Core/` và `src/WpsAiBridge.Native/` chỉ còn là thư mục build cũ (không được git
   theo dõi) — xoá được nếu muốn cây làm việc gọn.

## 7. Đối chiếu benchmark & lộ trình hiệu năng (02–03/10/2026)

Bộ đề 4 bài LibreOffice Calc so với Claude Code nằm ở `tests/bench/` (README cách chạy, số liệu ở
`results/`). Lộ trình hiệu năng theo tư vấn ngoài: **Bước 0–3 ĐÃ LÀM** (Bước 3 chưa đo với LLM)
(`results/con-lai.md` mục 7):

| Bước | Nội dung | Trạng thái |
|---|---|---|
| 0 | Dòng `METRIC` trong `run_prompt.py` (billable, by_tool, dup_reads, wall…) | ✅ |
| 1 | `PlanRule` trong prompt.go + skill quy tắc 9–10 | ✅ **đã A/B, giữ PlanRule** |
| 2 | Cắt response `readRange`/`checkRange` ở wrapper Go (`truncate.go`) + tail + contract | ✅ unit + **live 10/10** trên LO (50k dòng: 1 MB → 2,8 KB) |
| 3 | Sinh dữ liệu bằng công thức tất định (skill quy tắc 11 + `references/du-lieu-gia-lap.md`) | ✅ đo trên LO (11 cột × 50k dòng 9,7 s); chưa A/B với LLM |

**Kết quả A/B Test 3 đợt mimo (n=3/cặp, model `oc/mimo-v2.6-flash-free`, 03/10/2026)**:
6/6 lượt `completed + verified=True`, 13/13 sheet bắt buộc — variant **rẻ hơn 3/3 (median
−11,4% billable, −24,2% tool call)**, wall median −9,9%, điểm yếu: chart (control 3/3 lượt vs
variant 1/3). Chi tiết + caveat: `results/con-lai.md` mục 7; hạ tầng A/B + driver + 9 bài học
vận hành ngoài repo: `C:\Users\ThuyetMT\test\bench\ab\README.md`.

Cảnh báo vận hành đã ghi nhận: model free có lượt `provider returned an empty reply` (tính là lỗi
hạ tầng, loại khỏi số liệu); **không sửa script khi nó đang chạy** (bash đọc lệch offset → mất log);
LO thật là process `soffice.bin`.
