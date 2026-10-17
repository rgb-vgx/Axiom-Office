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

## P4-01. Registry quirk - XONG

- [x] Tệp `core-go/internal/model/profile.go`: `Profile` (Name, Family, AuthHeader/AuthPrefix, ExtraHeaders,
      SupportsThinking/SupportsTemperature) + bảng `profiles` + `ProfileFor`/`RegisterProfile`.
- [x] `types.go` `NewCodec` và `client.go` `ApplyAuth` đọc registry (hết `if provider == "anthropic"`); cổng gửi
      `thinking` theo `ProfileFor(...).SupportsThinking`.
- [x] Test `TestProfileRegistryDrivesCodecAndAuth` (khai báo profile mới → codec + header đúng, không sửa
      client.go/types.go), `TestProfileDefaultsAndThinking`.
- [ ] Còn: `catalog.go` `GET /models` gắn profile/quirk cho add-in hiện (P4-03).

## P4-02. Nhớ quirk qua các lượt

- [ ] Ghi quirk ("từ chối `thinking`") vào nơi bền vững (HKCU/config.json **hoặc** bảng trong `core.db`).
      **Chưa làm có lý do**: `core.db` dùng CHUNG với bản .NET nên tăng `SchemaVersion` ở Go có thể làm bản .NET
      đọc hụt schema; còn ghi HKCU/config.json thì Go hiện chỉ *đọc* (add-in sở hữu phần ghi) - làm một mình dễ đè
      cấu hình người dùng. Cần thống nhất với phía add-in trước.
- [ ] `client.go`: đọc quirk đã nhớ trước khi gửi (chờ bước trên).
- [ ] Test hai "phiên" (dựng client mới) phiên sau không gửi `thinking` (chờ bước trên).

## P4-03. Admin/wizard hiện model và quirk

- [ ] `src/AxiomOffice/Ai/CoreAdmin.cs`, `AdminForms.cs`: hiện model đang dùng, model dự phòng, quirk đã biết.
- [ ] Nút "Kiểm tra mô hình" đã có: bổ sung hiện kết quả quirk phát hiện lúc kiểm.
- [ ] Chữ mới vào `src/AxiomOffice/lang/en.json`; `python scripts/check_i18n.py` không thiếu.

## P4-04. Hồi quy + changelog

- [x] `cd core-go && go test ./internal/model/... ./internal/config/...`.
- [x] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `05-p4`.

## Tiêu chí xong

- [ ] Thêm một model mới chỉ cần khai báo profile + test, không sửa `client.go`.
- [ ] Model đã biết không nhận `thinking` không tốn request thử ở lượt sau (đếm trong log).
- [ ] `go test ./internal/model/... ./internal/config/...` xanh.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/model/... ./internal/config/...
cd ..; python scripts/check_i18n.py
```
