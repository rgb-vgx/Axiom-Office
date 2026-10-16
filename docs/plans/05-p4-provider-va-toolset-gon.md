# 05 / P4 - Provider khai báo và quirk theo model

> Thêm model không sửa lỗi lung tung. Quirk của từng model nằm một chỗ.

## 1. Mục tiêu

- Đặc thù của từng nhà cung cấp/model (không nhận `thinking`, không nhận `temperature`, cửa sổ ngữ cảnh, stream,
  tool call) khai báo trong một registry thay vì rải `if`.
- Giữ 2 họ API đã kiểm chứng: OpenAI-compatible và Anthropic.

## 2. Đã có (đối chiếu code 16/10/2026)

- `core-go/internal/model/`: `client.go`, `openai.go`, `anthropic.go`, `sse.go`, `window.go` (cửa sổ theo model),
  `guards.go` (chuỗi guard), `catalog.go` (`GET /models`), model dự phòng (`LlmFallbackModels`).
- Tự nhận ra provider từ chối tham số `thinking` rồi thử lại không có nó (log 15/10/2026 với muse-spark).
- Preset nhà cung cấp trong `catalog/setup.json` (`company`, `openai`, `anthropic`, `gemini`) → wizard.
- Lỗi cấu hình dịch sang tiếng Việt: `core-go/internal/setup/llmerrors.go`.
- **Lọc tool theo chế độ đã có**: chế độ lập kế hoạch (`request.Plan`) chỉ mở tool đọc (`readOnly` trong
  `tools.go`, `ReadOnlyMCP`); `Interactive=false` (`ai.ask` của agent khác) từ chối lệnh cần xác nhận.
- Tool tìm web: `tavily_search` (Core, cần khóa Tavily); `web_search`/`web_fetch`/`web_research` là MCP qua Chrome
  (tên tool `mcp__<server>__…`).

## 3. Còn thiếu - việc cần làm

- **Registry quirk**: `ProviderProfile`/`ModelProfile` (tên, họ API, nhận `thinking`/`temperature` không, cửa sổ,
  stream usage, giới hạn tool) - nhớ lại quirk đã phát hiện lúc chạy (như "từ chối thinking") để lần sau không
  mất một request thử.
- `client.go` và `types.go` đọc registry thay cho `if provider == "anthropic"` (hiện 2 chỗ).
- Admin/wizard hiện rõ model đang dùng, model dự phòng, quirk đã biết; nút kiểm tra đã có (`Kiểm tra mô hình`).
- Không làm "toolset filter" riêng nữa (đã có chế độ lập kế hoạch). Chế độ "webhook-safe" chỉ làm khi có API
  ngoài gọi vào thật.

## 4. Tệp sẽ sửa

- `core-go/internal/model/*.go`, `core-go/internal/config/*.go`
- `src/AxiomOffice/Ai/CoreAdmin.cs`, `AdminForms.cs` (nếu hiện quirk)

## 5. Tiêu chí xong

- Thêm một model mới chỉ cần khai báo profile + test, không sửa `client.go`.
- Model đã biết không nhận `thinking` không tốn request thử ở lượt sau (đếm trong log).
- `go test ./internal/model/... ./internal/config/...` xanh.

## 6. Không làm

- Không OAuth phức tạp, không pool khóa, không màn hình chi phí kiểu Hermes.
