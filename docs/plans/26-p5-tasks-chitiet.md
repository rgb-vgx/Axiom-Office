# 26 / P5 - Task chi tiết (Báo cáo định kỳ)

> Chia nhỏ tới từng hàm. Đối chiếu code 17/10/2026; đọc `06-p5` trước.
>
> **TẠM HOÃN 17/10/2026**: P5 chưa bắt đầu. Đã chốt cơ chế (Task Scheduler gọi `job`, `jobs.db` riêng, nhường Core
> đang chạy) nhưng vướng câu **Q7** ở `06-p5` mục 6 (job "tính thật trong app" mâu thuẫn "chạy khi Office đóng"). Khi
> quyết lại Q7 thì làm tiếp, và ghi đè theo các quyết định Q1–Q6: bảng `jobs` nằm ở **`jobs.db`**, **không** bump
> `SchemaVersion` của `core.db`.

## P5-00. Chốt thiết kế (làm trước khi viết code)

- [ ] Chọn cơ chế chạy: **Task Scheduler gọi `AxiomOffice.Core.exe job <id>` chạy rồi thoát** (đề xuất - không cần
      Core thường trú, không đụng mutex một-Core-mỗi-người-dùng) thay vì Core tự khởi động cùng Windows.
- [ ] Xác nhận làn việc: job chạy **làn tệp** (MCP built-in `doc_*`/`excel_*`/`ppt_*` + `ooxml`), không qua bridge,
      không giả định có tài liệu đang mở.
- [ ] Xác nhận `Interactive=false`: từ chối mọi lệnh cần xác nhận; job **chỉ ghi ra tệp mới**, không bao giờ ghi
      đè tệp nguồn.
- [ ] Ghi quyết định vào `06-p5` rồi mới làm tiếp.

## P5-01. Bảng `jobs`

- [ ] `core-go/internal/store/db.go`: thêm `jobs` (id, cron, prompt, skill, tệp nguồn, thư mục kết quả, lastRun,
      nextRun, lastError) - tăng `SchemaVersion`, migration `schemaV4`, giữ tương thích tên .NET cùng `PRAGMA
      user_version`.
- [ ] `core-go/internal/store/jobs.go` (mới): CRUD + `Due(now)` (job tới hạn) + `MarkRun(id, time, err)`.
- [ ] Test: thêm/sửa/xóa job, `Due` đúng theo cron.

## P5-02. Lệnh `job` trong Core

- [ ] `core-go/cmd/axiom-core/main.go`: thêm nhánh `job <id>` cạnh `mcp`/`doctor`, gọi runner một lần rồi thoát.
- [ ] Runner tạo `Request` nội bộ (`interactive=false`, không có bridge), dùng lại orchestrator + làn tệp sẵn có.
- [ ] Kết quả: tệp mới trong thư mục kết quả, **có ngày trong tên**; thất bại ghi `lastError` và không treo Core.
- [ ] Test: chạy `job` với LLM giả, sinh tệp mới, không ghi đè tệp nguồn.

## P5-03. Lên lịch (Task Scheduler)

- [ ] `scripts/` (PowerShell): đăng ký/gỡ task Windows gọi `AxiomOffice.Core.exe job <id>`; chạy khi Office đóng.
- [ ] `doctor.go`: kiểm tra task đã đăng ký chưa, cửa sổ thời gian tới hạn.
- [ ] Test tay: hẹn 2 phút, đóng Office, xem tệp kết quả sinh ra.

## P5-04. UI hẹn giờ

- [ ] Khung chat ("⋯" → Báo cáo định kỳ) hoặc form riêng trong `src/AxiomOffice/Ai/`.
- [ ] Chữ mới vào `src/AxiomOffice/lang/en.json`; `python scripts/check_i18n.py` không thiếu.

## P5-05. Kiểm thử + changelog

- [ ] `cd core-go && go test ./internal/store/... ./internal/api/...`.
- [ ] Job tuần chạy đúng giờ 2 kỳ liên tiếp khi Office đóng; thất bại có log, thử lại theo lịch.
- [ ] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `06-p5` (bỏ nhãn "cần thiết kế lại").

## Không làm

- [ ] Không gửi qua Telegram/email, không webhook, không worker tách rời phức tạp.
