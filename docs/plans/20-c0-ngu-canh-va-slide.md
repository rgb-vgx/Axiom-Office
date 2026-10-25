# 20 / C0 - Bớt ngữ cảnh gửi lại và slide sơ đồ: đã làm, cách đo, việc tiếp

> Tài liệu bàn giao đợt **25/10/2026** để phiên sau làm tiếp mà không phải dò lại. Mọi số liệu dưới đây đo trên máy
> thật (Windows, Microsoft Office x64, model `ocg/beam-free` qua endpoint OpenAI-compatible), không suy đoán.
> Trước khi làm một mục "Việc tiếp", đọc lại code ở chỗ được trỏ tới: trạng thái có thể đã đổi.

**Ràng buộc giữ nguyên:** chỉ dùng model yếu đã cấu hình (không đổi `AXIOM_LLM_MODEL` sang model mạnh); chỉ Microsoft
Office trên Windows (LibreOffice dừng hỗ trợ, WPS để khi sang host Linux); `catalog/live-commands.json` là nguồn chuẩn
của lệnh bridge — sửa xong chạy `python scripts/generate_mcp_commands.py`; mở/đóng Word/Excel/PowerPoint để chạy thử thì
báo người dùng trước và tự đóng sau.

## 1. Bối cảnh

Mỗi vòng của vòng lặp agent, Core gửi lại **toàn bộ** ngữ cảnh cho model (định nghĩa tool, system prompt, hội thoại
trước, mọi tool call và kết quả của lượt). Đợt này làm hai việc song song:

1. **Bớt ngữ cảnh gửi lại** — nghiên cứu, đo, rồi sửa. Báo cáo nghiên cứu: [`docs/research/giam-ngu-canh-gui-lai-agent.md`](../research/giam-ngu-canh-gui-lai-agent.md)
   (đo 26 lượt thật: ~71% đầu vào là tiền tố cố định tools + system; ~25% là các vòng sau lần sửa tài liệu cuối;
   kết quả tool tích luỹ ~19,5%). Ý kiến phản biện của ChatGPT (qua MCP `chatgpt-lookup`, xem §6) đã gộp vào lộ trình.
2. **Slide sơ đồ cho đề thật** — đề "Thiết kế hệ thống AI Agent cho ngân hàng" (10 slide, sơ đồ kiến trúc, vòng đời
   yêu cầu, KPI, lộ trình): [`tests/live/tasks/ngan-hang-ai-agent.md`](../../tests/live/tasks/ngan-hang-ai-agent.md).

## 2. Đã làm (theo commit, cũ → mới)

| Commit | Nội dung | Chỗ sửa chính |
|---|---|---|
| `11e3041` | Tool call gọi nhầm tên lệnh làm tên tool (`writer.getText` → `office_action`): `action` là enum, lỗi chỉ đường, `RepairToolCall` sửa cả `Turn.Raw` | `core-go/internal/agent/repair.go`, `model/repair.go` |
| `4d045ad` | Bỏ agent in-process của add-in (`AiAgent`, `LlmClient.RunAgent`, `LegacyAgent`); `ai.ask` không có Core thì báo lỗi | `src/AxiomOffice/Ai/*`, `Bridge/CommandDispatcher.cs` |
| `3daff16` | `ai.ask` trả `transcript` đủ dạng `office_action {...} -> {...}` khi chạy qua Core | `Ai/CoreClient.cs` (`ToolLine`) |
| `ce1b2d6` | `newFromTemplate`/`addSlideFromTemplate` khi mẫu đang mở; `lockSelection` cả tài liệu; định dạng điều kiện Excel; test live cho 24 lệnh | `Bridge/CommandDispatcher.Template.cs`, `WriterScope.cs`, `Spreadsheet.cs` |
| `c026a32` | Chế độ lập kế hoạch biết lệnh ghi và tool sẽ có khi chạy (`PlanCapabilities`) | `agent/prompt.go`, `orchestrator_execution.go` (`actTools`) |
| `72c2892` | `checkLayout` tính đường đi thật của đường nối (`connector-crossing`, `connector-overlap`, `path`/`from`/`to`); `addConnector` `fromSide`/`toSide`/`offset`; `RouteDiagram`; nhóm màu `group`; bìa căn giữa | `Bridge/CommandDispatcher.Connectors.cs`, `slidelayout/route.go`, `tools/slides_brief.go` |
| `1158e9e` | Bớt ngữ cảnh: `docStateGuard` (đọc lặp khi tài liệu chưa đổi → "unchanged"), `verifyGuard` theo bằng chứng, `AppMCP`, cache Anthropic, `StopRule`, bỏ `reasoning_content` non-stream, **đo từng vòng (schema 4)**, bench `s25` | `model/guards.go`, `agent/prompt.go`, `store/*`, `tools/mcp_tools.go` |
| `ffc8e9d` | `slides_from_brief` nhận trường lồng trong khoá cùng tên layout (`{"diagram":{"nodes":...}}`); lỗi thiếu nút/bước/mốc chỉ cách viết đúng; "không gọi `wpp.addSlide` trước" | `tools/slides_brief.go` (`unnestLayoutFields`) |
| `32a5f74` | Pane không gọi mọi lỗi là "Máy chủ AI báo lỗi" (đọc `ErrorKind` của Core; lặp lệnh hỏng có câu riêng) | `Ai/AskAiPane.cs` (`DescribeError`), `Ai/LlmClient.cs`, `CoreClient.cs` |
| `a779136` | Xoá slide **trống** không hỏi xác nhận (người dùng yêu cầu); màu nhóm 3 khác hẳn nhóm 1 | `policy/policy.go` (`BlankSlide`), `tools/office_action.go` |
| `bc1bc3f` | `replace: N` sửa đúng slide lỗi tại chỗ (chỉ slide do lượt dựng); `wpp.addSlide index`; `SAME_SPEC_SAME_ISSUES`; đường nối tránh tiêu đề + chú thích; lỗi bố cục vào `core.log`; `notes: "set"`; một nhóm thì không tô màu | `tools/slides_brief.go`, `tools/owned.go`, `slidelayout/route.go`, `CommandDispatcher.Presentation.cs` |

## 3. Kết quả đo trên đề thật

Chạy bằng `tests/live/run_ai_task.py` (ai.ask, **không** qua bước lập kế hoạch, không ai bấm xác nhận) trên bài
PowerPoint mới. Số vòng/token lấy từ `core.db` (§4).

| Lượt (run id) | Sau commit | Vòng | Token vào (cache) / ra | Thời gian | Ghi chú |
|---|---|---|---|---|---|
| `r_01462d7b…` | `a779136` | 20 | 508.263 (442.048) / 37.919 | 183 s | `checkLayout` báo 3 lỗi → model **dựng lại cả 10 slide** rồi xoá 10 slide cũ |
| `r_ae5221fe…` | `replace` (chưa tránh chú thích) | 42 | 1.261.160 (1.123.456) / 39.113 | 212 s | Đường nối cắt qua nhãn chú thích nhóm màu (log: `connector-crossing … 'TextBox 22'`); model gửi lại slide sơ đồ gần y hệt ~17 lần qua `replace` tới khi bị chặn |
| `r_894e42ac…` | `bc1bc3f` (trừ `notes`/một nhóm) | 30 | 841.059 (752.896) / 33.099 | 162 s | Hết cắt chú thích; `replace` 4 lần → sơ đồ sạch; **10 vòng `wpp.setNotes` thừa** (đã đặt notes qua brief) → đã sửa bằng `notes: "set"` |

Qua khung chat (có "Lập kế hoạch trước"): bước lập kế hoạch 3 vòng, ~27k token, 38 s (trước `c026a32` là 11 vòng,
160k token và kế hoạch vẽ sơ đồ bằng chữ). Bước thực hiện lượt 18:17 hỏng vì sơ đồ lồng (đã sửa ở `ffc8e9d`) — **chưa
chạy lại bước thực hiện qua khung chat** sau các sửa đổi.

Mỗi vòng giờ gửi ~7–35k token (tăng theo lịch sử của lượt), phần lớn trúng cache; bốn lệnh đầu lượt (2 `load_skill` +
2 `read_skill_file`) nạp ~28k ký tự skill/tokens ở mọi lượt slide.

Ảnh chụp các lần chạy nằm ở thư mục scratchpad của phiên (không commit). Chạy lại để có ảnh mới (§4).

## 4. Cách đo và cách chạy lại

**Chạy đề thật** (mở PowerPoint, chạy, chụp từng slide, `checkLayout`, lưu bài, đóng PowerPoint):

```powershell
scripts\build.ps1            # Office phải đóng (DLL bị khoá thì build báo lỗi)
python tests/live/run_ai_task.py tests/live/tasks/ngan-hang-ai-agent.md <thu-muc-ra>
```

**Số liệu một lượt** — `%LOCALAPPDATA%\AxiomOffice\core\core.db`, mở **chỉ đọc**
(`sqlite3.connect('file:...?mode=ro', uri=True)`):

```sql
-- lượt mới nhất
SELECT id, rounds, input_tokens, cached_tokens, output_tokens FROM runs ORDER BY started_at DESC LIMIT 1;
-- từng vòng: kind = tools | final | nudge:<guard> | end:<guard> | error
SELECT round, input_tokens, cached_tokens, output_tokens, tool_calls, kind FROM run_rounds WHERE run_id = ? ORDER BY round;
-- từng lệnh: result_chars = kích thước kết quả đã vào ngữ cảnh (không lưu nội dung)
SELECT seq, tool, action, ok, result_chars, error, params_json FROM tool_calls WHERE run_id = ? ORDER BY id;
```

**Lỗi bố cục model đã thấy** (từ `bc1bc3f`) — `%LOCALAPPDATA%\AxiomOffice\core.log`:
`run <id>: slides_from_brief slide 4: connector-crossing - connector 'Elbow Connector 19' runs across 'Rounded Rectangle 11' …`
(và `wpp.checkLayout slide N: …`). Dòng kết thúc lượt: `run <id> completed: N rounds, … cached, no-tool rounds: final 1`.

**Lượt qua khung chat:** `%LOCALAPPDATA%\AxiomOffice\bridge.log` (`AskAiPane: prompt=`, `AskAiPane progress:`,
`AskAiPane: ok in …` / `AskAiPane failed …`). Nội dung kế hoạch / câu trả lời: bảng `messages` của `core.db`.

**Test tự động liên quan:** `go test ./...` (trong `core-go`), `python tests/core/test_core_e2e.py` (đặt
`AXIOM_E2E_CORE_EXE`, xem `.github/workflows/ci.yml`), `python -m unittest discover -s tests/catalog`, live:
`tests/live/test_live_slides.py` (cần PowerPoint mở, cổng 47833), `tests/live/test_live_commands.py [--ai]`.

## 5. Việc tiếp (theo thứ tự đề xuất)

### 5.1 Công cụ tự đi lại dây khi PowerPoint vẽ lệch mô hình — **nên làm trước**

- **Bằng chứng:** lượt `r_894e42ac…`, lần dựng đầu slide 4: 1 `connector-crossing` (đường cắt qua một nút) + 3
  `connector-overlap`, dù `RouteDiagram` đã chọn đường "sạch" theo mô hình. Model không sửa được lỗi đi dây (đó là
  việc của tool) nên chỉ tốn vòng gửi lại.
- **Hướng làm:** trong `buildBriefSlide`/`briefShapes` (`core-go/internal/tools/slides_brief.go`), sau `wpp.checkLayout`
  của slide, với mỗi `connector-*` thuộc đường do tool vẽ: xoá đường đó (`wpp.deleteShapes`), vẽ lại bằng ứng viên kế
  tiếp của `diagramCandidates` (`core-go/internal/slidelayout/route.go`) hoặc tăng `offset`, kiểm lại; tối đa 2 lần mỗi
  đường. Cần map tên shape đường nối ↔ cạnh (kết quả `wpp.addConnector` trả `shape`). `checkLayout` đã trả `path` của
  từng đường nối (toạ độ thật) để so với mô hình — dùng nó để sửa mô hình nếu lệch có quy luật.
- **Xong khi:** đề ngân hàng chạy 3 lần liền, lần dựng đầu không còn `connector-*` trong `core.log`.

### 5.2 Thành phần xuyên suốt trong sơ đồ (Observability, audit…)

- Hiện nút như vậy nằm lẻ một ô, không nối với ai (ảnh slide 4 các lượt). Đề bài yêu cầu "Observability thu thập
  trace, log và metric **xuyên suốt** hệ thống".
- Hướng: thêm vào layout `diagram` một trường kiểu `span: [{text, side: right|bottom}]` vẽ thành dải dọc/ngang chạy
  cạnh các tầng, có đường nét đứt (hoặc chú thích) — không phải một nút trong lưới. Ghi vào mô tả tool + skill
  `trinh-bay-chuyen-nghiep`. Cần thêm `dash` cho `wpp.addConnector`/`wpp.addShape` (C#) nếu dùng nét đứt.

### 5.3 Bớt vòng đầu lượt nạp skill

- 4 lệnh đầu mọi lượt slide nạp ~28k ký tự (`load_skill trinh-bay-chuyen-nghiep` ~13k, `thiet-ke-van-phong` ~3k,
  `_design/tokens.json` ~3k, `references/bo-cuc-slide.md` ~8k) — gửi lại ở **mọi** vòng sau đó.
- Hướng: khi có `slides_from_brief`, tool đã tự đọc `tokens.json` và tự đặt lưới → skill nên nói rõ không cần đọc
  `tokens.json`/`bo-cuc-slide.md` (chỉ cần khi tự vẽ bằng `wpp.addText`); cân nhắc tách phần "dùng slides_from_brief"
  thành file ngắn riêng. Đo lại `result_chars` của 4 lệnh đầu.

### 5.4 Thiết kế còn đơn điệu

- Bìa chỉ có chữ (đề đòi "điểm nhấn thị giác"); nhiều slide dạng thẻ giống nhau; slide lộ trình thường thành `cards`
  thay vì `timeline` (timeline hiện chỉ có `{date, text}`, không có mục tiêu/điều kiện/rủi ro).
- Hướng: layout `cover` có dải màu/khối nhấn theo token; `timeline` cho phép 2–3 dòng ngắn mỗi mốc; gợi ý trong skill
  "lộ trình → timeline, KPI → figures/chart".

### 5.5 Kiểm lại qua khung chat có "Lập kế hoạch trước"

- Bước thực hiện qua khung chat chưa chạy lại sau `ffc8e9d`…`bc1bc3f`. Cần người dùng bấm (Claude không điều khiển
  được pane). Đọc log theo §4.

### 5.6 Còn từ báo cáo nghiên cứu / tư vấn ChatGPT

- **Benchmark có lỗi cài sẵn mà đề KHÔNG nói chỗ lỗi** (đo khả năng tự phát hiện, không chỉ sửa) — `s25` hiện nói rõ
  lỗi. Thêm brief vào `tests/bench/slides/briefs.json`.
- **Phân loại vòng không gọi tool** — giờ có `run_rounds.kind`; tổng hợp trên nhiều lượt rồi quyết định.
- **Phiên bản tài liệu cho lệnh ghi** (từ chối ghi khi tài liệu đã đổi so với lần đọc) — hiện `docStateGuard` chỉ biết
  thay đổi do chính lượt ghi; người dùng sửa tay giữa lượt thì không thấy.
- **Xoá kết quả tool cũ theo lô** — chỉ làm khi số đo cho thấy đáng (lượt ngắn, cache cao).

### 5.7 Nhỏ

- `docStateGuard` trả "unchanged" cho mọi lệnh không ghi lặp y hệt (kể cả `web_search`, `load_skill`) — câu ghi chú nói
  về "tài liệu"; nếu gây nhầm thì giới hạn cho `office_action` đọc.
- Cache Anthropic dùng `cache_control` cấp request (automatic): proxy "tương thích Anthropic" có thể không nhận trường
  này → nếu gặp HTTP 400 thì nghi chỗ này trước (`core-go/internal/model/anthropic.go`).

## 6. Công cụ hỗ trợ

- **MCP `chatgpt-lookup`** (`D:\ThuyetMT\mcp\chatgpt-lookup`, đăng ký user scope trong Claude Code): tool `chatgpt_ask`
  gửi một câu hỏi tới ChatGPT bản free qua Chrome ẩn, trả markdown. Dùng để xin phản biện thiết kế. Phiên này đã hỏi
  về lộ trình bớt ngữ cảnh; câu trả lời đã gộp vào §5.6 và `StopRule`.
- **Thư mục nghiên cứu cục bộ** `reports/`, `research_notes/` ở gốc repo **không commit** (ghi chú thô của các hướng
  nghiên cứu); bản báo cáo đã đưa vào `docs/research/`.
