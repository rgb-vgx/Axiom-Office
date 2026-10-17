# 21 / M2 - Task chi tiết (Fact lâu dài: an toàn và ngân sách)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `11-m2` trước.
> Mọi đường ghi fact đều đi qua `Service.Add` (`core-go/internal/memory/service.go`) - API, tool `remember`, extract.

## M2-00. Lọc nhạy cảm bằng boolean - ĐÃ CÓ

- [x] `IsSensitive` (`core-go/internal/memory/text.go`): chặn 9–12 chữ số liền (CCCD/số tài khoản), số thẻ,
      `mật khẩu`/`password`/`PIN`/`OTP`, khóa API (`sk-`, `AIza`, `ghp_`, `xox`…).
- [x] Bỏ fact ở hai chốt: `extractor.go` (`ParseFacts`) và `store.go` (`AddBatch`, `SKIPPED` lý do `sensitive`).
- [x] Test `TestSensitiveAndValidDate` (`memory_test.go`).

## M2-01. `Redact` - thay `[ẩn]` thay vì chỉ bỏ - XONG

- [x] `core-go/internal/memory/text.go`: `Redact(text) (string, bool)` dùng lại `apiKey` + thêm email, token
      JWT/`Bearer`, mật khẩu/PIN/OTP, số có dấu cách (điện thoại/thẻ), số 9–19 chữ số.
- [x] Giữ `IsSensitive` nguyên nghĩa (chốt chặn khi ghi); `Redact` là bản "thay chỗ nhạy cảm" để tái sử dụng.
- [x] Test `redact_test.go`: mẫu nhạy cảm + mẫu **không** được cắt oan (số trang, ngày, số tiền `1.234.567`,
      Nghị định `30/2020/NĐ-CP`, lớp `4A`).
- [x] Hàm thuần, không phụ thuộc store → `27-p6` (P6-01) gọi trực tiếp trước khi ghi `tool_calls`.

## M2-02. Dùng redact ở chỗ ghi (tuỳ chọn) - CHƯA LÀM

- [ ] Mặc định hiện tại (bỏ hẳn fact nhạy cảm khi ghi) vẫn đúng tinh thần "không lưu thứ không được lưu". Chỉ đổi
      sang `Service.Add` gọi `Redact` giữ fact nếu có nhu cầu rõ; đổi thì phải có test.

## M2-03. Số liệu phải kèm nguồn - XONG (một phần)

- [x] `core-go/internal/memory/prompts/extract.txt`: thêm quy tắc 14 - fact có số liệu phải kèm nguồn, câu chỉ có
      con số thì không trả.
- [x] `extractor.go` `ParseFacts`: bỏ fact chỉ toàn số/không có chữ (`hasDigitsWithoutWords`).
- [ ] Còn: nếu model thật còn trả số vô căn cứ, siết thêm bằng kiểm "số ≥4 chữ số mà không có tài liệu/vùng".

## M2-04. Ngân sách token + log số fact bị cắt - XONG

- [x] `core-go/internal/memory/retriever.go` `BuildContext`: đếm số fact bị cắt, ghi `corelog` (số fact vào prompt,
      số bị cắt, ngân sách, số ký tự đã dùng).
- [x] Giữ hành vi cũ: không có embedding vẫn tìm bằng keyword (nhánh bm25) - test có sẵn xanh.

## M2-05. Kiểm thử + changelog

- [x] `cd core-go && go test ./internal/memory/...` xanh.
- [ ] Seed 50 câu có số điện thoại/CCCD/token giả → không câu nào lọt vào `memories` (đọc `core.db`). (Đã có
      `IsSensitive` + `Redact`; seed tự động để lại làm khi có bộ đo R0.)
- [x] Chèn 100 fact → context memory không vượt ngân sách, log ghi số fact bị cắt (logic đã có; test hiệu năng làm
      cùng `24-m5`).
- [x] Ghi `CHANGELOG.md [Unreleased]`.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/memory/...
```
