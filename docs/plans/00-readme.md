# Lộ trình Agent văn phòng Axiom

> Sửa ngay trên tài liệu đang mở, thấy trực tiếp, hoàn tác được.

Rà soát 16/10/2026: các phase được viết lại thành **danh sách phần còn thiếu**. Mỗi tệp có mục
"Đã có" dẫn tới code thật, để không làm lại thứ đang chạy. Trước khi làm một phase, đọc mục đó rồi kiểm lại
trong code: trạng thái có thể đã đổi.

## Thứ tự làm

| Thứ tự | Tệp | Nội dung | Ghi chú |
|---|---|---|---|
| 1 | `17-r0-model-yeu-va-do-luong.md` | Bộ đo theo kịch bản với model yếu, chặn loop/đốt token | **Mới.** Là thước đo cho mọi phase sau |
| 2 | `01-p0-hieu-tai-lieu-sau.md` | Đọc đúng tài liệu: mở rộng lệnh đọc có sẵn | Thu hẹp: không viết 3 lệnh mới × 3 nền tảng |
| 3 | `02-p1-duyet-truoc-khi-luu.md` | Sửa → xem lại → hoàn tác cả lượt; chụp vùng cho Excel | Thay "preview diff" |
| 4 | `03-p2-skill-chuan-van-phong.md` | Chuẩn nội dung 8 skill, kiểm bằng tiêu chí cấu trúc | Bỏ golden chéo C#/Go |
| 5 | `04-p3-nho-theo-tai-lieu.md` + `10`–`14` | Memory: phần còn thiếu (export/import, org-style, ẩn PII, số đo) | Phần lớn đã có (mốc G4) |
| 6 | `05-p4-provider-va-toolset-gon.md` | Registry provider khai báo | Toolset filter phần lớn đã có (chế độ lập kế hoạch) |
| 7 | `07-p6-audit-va-an-toan-du-lieu.md` | Ẩn thông tin nhạy cảm trong audit, bản chụp để khôi phục | Audit đã có |
| 8 | `08-p7-ux-trong-app.md` | Dịch nốt giao diện, lỗi tiếng Việt dễ hiểu | Phần lớn đã có |
| 9 | `09-p8-libreoffice-parity.md` | Các lệnh còn để trống phía LibreOffice | LibreOffice đã chạy 281/281 |
| 10 | `18-s0-slide-do-va-spec.md` | Slide: bộ đo chất lượng (24 brief) và thiết kế spec + layout solver dùng chung cho slide từ brief và theo mẫu | **Mới.** Bộ đo xong phần chấm, chưa có baseline thật |
| 11 | `19-h0-nen-tang-ben-vung.md` | Nền tảng bền: lệnh nhắm tài liệu tường minh, bridge không treo vì hộp thoại/máy ngủ, đọc đủ những gì đã ghi, miễn xác nhận cho đối tượng do lượt tạo, năng lực từng backend | **Mới.** Bằng chứng từ benchmark slide 25/10/2026 |
| — | `06-p5-bao-cao-dinh-ky-toi-gian.md` | Báo cáo định kỳ | **Cần thiết kế lại**: Core không chạy khi Office đóng |
| — | `15-p0-tasks-chitiet.md`, `16-p1-tasks-chitiet.md` | Task chi tiết P0, P1 | Đã cập nhật theo hướng mới |

## Nguyên tắc

- Mỗi phase có **tiêu chí xong đo được**: kịch bản nào, model nào, chạy mấy lần, so với mốc nào (bộ đo ở `17`).
  "Chạy được trên máy mình một lần" không phải tiêu chí xong.
- Người dùng chính chạy **model free yếu** (mimo, deepseek flash): mọi tối ưu và mọi phép đo nhắm vào model yếu.
- Commit nhỏ theo tính năng, message tiếng Việt có dấu, mỗi commit build được; ghi `CHANGELOG.md [Unreleased]`.
- Danh mục lệnh sinh tự động từ registry Python (`scripts/generate_mcp_commands.py`): thêm lệnh ở registry,
  không sửa tay `catalog/live-commands.json`, `commands.js`, `livecommands_gen.go`.
- Không ôm terminal/trình duyệt/computer-use/kanban, không chat đa nền tảng, không tự sinh skill.

## Việc ngoài các phase (chưa có tệp riêng)

- Đóng gói, cập nhật, ký số DLL và exe (docs/architecture/README.md ghi DLL chưa ký số).
- Chất lượng tính năng trình bày: `doc_design` (Word), `slides_from_template` (PowerPoint) - đo bằng bộ `17`.
- Dịch giao diện: xong ribbon + khung chat Windows; còn wizard, các form quản lý, khung chat WPS/LibreOffice.
