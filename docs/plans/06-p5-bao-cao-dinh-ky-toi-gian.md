# 06 / P5 - Báo cáo định kỳ (TẠM HOÃN)

> Hẹn giờ báo cáo tuần/tháng. **TẠM HOÃN 17/10/2026** (xem mục 6) cho tới khi giải xong câu Q7. Không bắt đầu.

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

## 6. Quyết định thiết kế (chốt dần, 17/10/2026)

- **Q1 — Nền tảng dữ liệu:** giữ **một `core.db`**; **memory tách theo nhánh app** (fact/nhánh của Word không lọt
  vào ngữ cảnh Excel). Không tách file DB theo app (giữ tương thích bản .NET + lịch sử hội thoại xuyên app).
- **Q2 — Phạm vi nhánh app:** tách theo app cho `org-style` và **kiểu trình bày** (chuẩn trình bày Word ≠ Excel ≠
  PowerPoint); `user` (hiểu biết về người dùng) giữ **chung** mọi app. `document` vốn đã khoá theo đường dẫn tệp nên
  thực tế đã per-app sẵn.
- **Q3 — Lưu chiều app:** dùng **`scope_key` chứa app** (`et`/`wps`/`wpp`) cho `org-style` và scope `doc-style`; **không
  đổi schema, không bump `SchemaVersion`** ⇒ bản .NET không bị ảnh hưởng. Phải sửa bất biến "org-style rỗng scope_key"
  và test cũ cho khớp.
- **Q4 — Cơ chế chạy job:** **BẮT BUỘC chạy khi Office đóng** ⇒ **Task Scheduler gọi `AxiomOffice.Core.exe job <id>`
  một lần rồi thoát** (thêm nhánh `job` cạnh `mcp`/`doctor`, chạy TRƯỚC mutex). Không Core thường trú.
- **Q5 — Core đang chạy lúc tới giờ:** tiến trình `job` **nhường Core đang chạy** (đọc `core.json`, gọi HTTP
  `POST /v1/jobs/<id>/run` rồi thoát); chỉ tự chạy một lần khi Core không chạy. Một tiến trình ghi `core.db`.
- **Q6 — Nơi lưu job:** **file riêng `jobs.db`** cạnh `core.db` (không bump `SchemaVersion` của `core.db`, bản .NET
  không bị đụng); lịch sử mỗi lần chạy ghi vào bảng `runs` sẵn có.
- **Q7 — Vướng chưa gỡ:** job **bắt buộc tính thật trong app** (công thức/pivot) mâu thuẫn với Q4: Core chỉ nói với
  app qua bridge (AD-12, không gọi COM), mà Office đóng thì không có bridge. Ba cách gỡ đã đưa ra — (1) job tự mở
  Office ẩn qua companion `Host.exe excel|word|ppt`, (2) đánh dấu `missed` rồi chạy bù khi user mở Office, (3) bỏ Q4.
  **Chưa chọn.**
- **Kết luận 17/10/2026:** **TẠM HOÃN cả P5** cho tới khi giải xong Q7. Riêng Q1–Q3 (nhánh memory theo app) **độc lập
  với P5** và đã chuyển sang `12-m3` mục 7 để làm riêng khi cần.
