# 25 / P4 - Task chi tiết (Provider khai báo và quirk)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `05-p4` trước.
> Đích: thêm model chỉ khai báo profile + test, không sửa `client.go`; quirk đã biết không tốn request thử lại.

## P4-00. Đã có

- [x] `core-go/internal/model/`: `client.go`, `openai.go`, `anthropic.go` (`Codec`), `sse.go`, `window.go` (cửa sổ
      ngữ cảnh theo model), `guards.go`, `catalog.go` (`GET /models`), model dự phòng.
- [x] Tự nhận provider từ chối `thinking` rồi thử lại không có nó: `client.go` (`thinkingRejected atomic.Bool`,
      `TestThinkingRejectionIsRemembered`). **Chỉ nhớ trong phiên** (mất khi Core khởi động lại).
- [x] Preset nhà cung cấp `catalog/setup.json` (`company`, `openai`, `anthropic`, `gemini`) → wizard.
- [x] Lỗi cấu hình dịch tiếng Việt: `core-go/internal/setup/llmerrors.go`.
- [x] Lọc tool theo chế độ đã có: `request.Plan` (chỉ đọc), `Interactive=false` (từ chối lệnh cần xác nhận).

## P4-01. Registry quirk

- [ ] Tệp: `core-go/internal/model/profile.go` (mới) + `config.go`: `ProviderProfile`/`ModelProfile` (tên, họ API,
      nhận `thinking`/`temperature` không, cửa sổ, stream usage, giới hạn tool).
- [ ] `client.go` và `types.go` đọc registry thay cho `if provider == "anthropic"` (đang có **2 chỗ**:
      `client.go:145` `ApplyAuth`, `types.go:98` `NewCodec`).
- [ ] `catalog.go`: `GET /models` gắn profile/quirk của từng model cho add-in hiện.
- [ ] Test: thêm một profile giả vào bảng + `go test ./internal/model/...` không phải sửa `client.go`.

## P4-02. Nhớ quirk qua các lượt

- [ ] Quirk phát hiện lúc chạy ("từ chối `thinking`", phải bỏ `temperature`…) phải **bền vững**, không chỉ
      `atomic.Bool` trong phiên: ghi vào cấu hình người dùng (HKCU/`config.json`) hoặc bảng nhỏ trong `core.db`
      (cân nhắc `SchemaVersion`, xem `store/db.go`).
- [ ] `client.go`: đọc quirk đã nhớ trước khi gửi, khỏi mất một request thử.
- [ ] Test: chạy hai "phiên" (dựng client mới) → phiên sau **không** gửi `thinking` (đếm request).

## P4-03. Admin/wizard hiện model và quirk

- [ ] `src/AxiomOffice/Ai/CoreAdmin.cs`, `AdminForms.cs`: hiện model đang dùng, model dự phòng, quirk đã biết.
- [ ] Nút "Kiểm tra mô hình" đã có: bổ sung hiện kết quả quirk phát hiện lúc kiểm.
- [ ] Chữ mới vào `src/AxiomOffice/lang/en.json`; `python scripts/check_i18n.py` không thiếu.

## P4-04. Hồi quy + changelog

- [ ] `cd core-go && go test ./internal/model/... ./internal/config/...`.
- [ ] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `05-p4`.

## Tiêu chí xong

- [ ] Thêm một model mới chỉ cần khai báo profile + test, không sửa `client.go`.
- [ ] Model đã biết không nhận `thinking` không tốn request thử ở lượt sau (đếm trong log).
- [ ] `go test ./internal/model/... ./internal/config/...` xanh.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/model/... ./internal/config/...
cd ..; python scripts/check_i18n.py
```
