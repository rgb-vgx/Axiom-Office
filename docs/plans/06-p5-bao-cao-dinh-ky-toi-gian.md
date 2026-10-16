# 06 / P5 - Báo cáo định kỳ (cần thiết kế lại)

> Hẹn giờ báo cáo tuần/tháng. **Chưa làm** cho tới khi giải xong các câu hỏi ở mục 2.

## 1. Mục tiêu

- Hẹn giờ báo cáo tuần/tháng, tổng hợp dữ liệu từ tệp mẫu; chạy đúng giờ, thất bại có log và thử lại.

## 2. Vấn đề phải giải trước

- **Core không chạy khi Office đóng**: add-in bật Core khi người dùng hỏi AI lần đầu (`CoreClient`), build/gỡ cài
  đặt tắt Core. Office đóng thì không ai chạy job. Phương án:
  - Task Scheduler của Windows gọi `AxiomOffice.Core.exe` chạy một job rồi thoát (không cần Core thường trú), hoặc
  - Core thường trú tự khởi động cùng Windows (tốn tài nguyên, ảnh hưởng mutex một-Core-mỗi-người-dùng).
- **Không có ứng dụng mở**: job chạy làn tệp (MCP built-in đọc/ghi docx/xlsx/pptx) chứ không qua bridge; skill và
  tool hiện giả định có tài liệu đang mở.
- **Không có người để hỏi**: `Interactive=false` từ chối mọi lệnh cần xác nhận (ghi đè, lưu). Job phải ghi ra tệp
  mới, không bao giờ ghi đè tệp nguồn.

## 3. Phác thảo (sau khi chốt mục 2)

- Bảng `jobs` trong `core.db` (id, cron, prompt, skill, tệp nguồn, thư mục kết quả, lastRun, nextRun, lastError).
- Mỗi lần chạy: tạo `Request` nội bộ không tương tác, làn tệp, kết quả là tệp mới có ngày trong tên.
- UI hẹn giờ đơn giản trong khung chat ("⋯" → Báo cáo định kỳ) hoặc form riêng.

## 4. Tiêu chí xong (khi làm)

- Job tuần chạy đúng giờ 2 kỳ liên tiếp khi Office đóng.
- Thất bại có log, thử lại theo lịch, không treo Core, không ghi đè tệp nguồn.

## 5. Không làm

- Không gửi qua nhiều kênh (Telegram, email), không webhook, không worker tách rời phức tạp.
