# 21 / M2 - Task chi tiết (Fact lâu dài: an toàn và ngân sách)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `11-m2` trước.
> Mọi đường ghi fact đều đi qua `Service.Add` (`core-go/internal/memory/service.go`) - API, tool `remember`, extract.

## M2-00. Lọc nhạy cảm bằng boolean - ĐÃ CÓ

- [x] `IsSensitive` (`core-go/internal/memory/text.go`): chặn 9–12 chữ số liền (CCCD/số tài khoản), số thẻ,
      `mật khẩu`/`password`/`PIN`/`OTP`, khóa API (`sk-`, `AIza`, `ghp_`, `xox`…).
- [x] Bỏ fact ở hai chốt: `extractor.go` (`ParseFacts`) và `store.go` (`AddBatch`, `SKIPPED` lý do `sensitive`).
- [x] Test `TestSensitiveAndValidDate` (`memory_test.go`).
- **Việc còn**: hiện chỉ *bỏ hẳn* fact; chưa có hàm trả chuỗi đã thay `[ẩn]` để P6 dùng lại cho `params_json` audit.

## M2-01. `Redact` - thay `[ẩn]` thay vì chỉ bỏ

- [ ] `core-go/internal/memory/text.go`: thêm `Redact(text string) (clean string, hit bool)` dùng lại các mẫu có
      sẵn (`longDigits`, `cardNumber`, `secretWords`, `apiKey`) và **bổ sung**: email, số điện thoại VN có dấu
      cách/chấm, token JWT/`Bearer`.
- [ ] Giữ `IsSensitive` nguyên nghĩa (chốt chặn khi ghi); `Redact` là bản "thay chỗ nhạy cảm" để tái sử dụng.
- [ ] Test `redact_test.go`: mẫu thật + mẫu **không** được cắt oan (số trang, năm học, mã môn học, số tiền
      `1.234.567`).
- [ ] `Redact` phải **thuần** (không phụ thuộc store) để `07-p6` gọi trực tiếp trước khi ghi `tool_calls`.

## M2-02. Dùng redact ở chỗ ghi (tuỳ chọn)

- [ ] Cân nhắc: nếu muốn giữ fact mà bỏ riêng phần nhạy cảm thì gọi `Redact` trong `Service.Add` trước embedding.
      Mặc định hiện tại (bỏ hẳn fact) vẫn đúng tinh thần "không lưu thứ không được lưu"; đổi thì phải có test.

## M2-03. Số liệu phải kèm nguồn

- [ ] `core-go/internal/memory/prompts/extract.txt`: fact có số phải kèm tài liệu/vùng/thời điểm, thiếu thì **không
      trả** fact đó.
- [ ] `core-go/internal/memory/extractor.go` `ParseFacts`: fact có số mà không có nguồn (tài liệu hoặc vùng) thì
      bỏ; ghi log số fact bị bỏ vì thiếu nguồn.
- [ ] Test: seed câu có số không nguồn → không vào `memories` (đọc thẳng bảng).

## M2-04. Ngân sách token + log số fact bị cắt

- [ ] `core-go/internal/memory/retriever.go` `BuildContext`: nay đang cắt im lặng theo `MaxContextChars`; ghi log
      (`corelog`) số fact đưa vào và số bị cắt.
- [ ] Giữ hành vi cũ: không có embedding vẫn tìm bằng keyword (nhánh bm25) - test có sẵn phải xanh.

## M2-05. Kiểm thử + changelog

- [ ] `cd core-go && go test ./internal/memory/...`.
- [ ] Seed 50 câu có số điện thoại/CCCD/token giả → **không câu nào** lọt vào `memories` (đọc `core.db`).
- [ ] Chèn 100 fact → context memory không vượt ngân sách, log ghi số fact bị cắt.
- [ ] Ghi `CHANGELOG.md [Unreleased]` (mục Added - redact memory).

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/memory/...
```
