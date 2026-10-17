# Lộ trình Agent văn phòng Axiom

> Sửa ngay trên tài liệu đang mở, thấy trực tiếp, hoàn tác được.

Rà soát 16/10/2026: các phase được viết lại thành **danh sách phần còn thiếu**. Mỗi tệp có mục
"Đã có" dẫn tới code thật, để không làm lại thứ đang chạy. Trước khi làm một phase, đọc mục đó rồi kiểm lại
trong code: trạng thái có thể đã đổi.

## Tiến độ (đối chiếu code 17/10/2026, cập nhật cuối ngày)

- **P0 gần xong**: P0-01…P0-07 đã làm (heading path, ngân sách mục cấu trúc, mở `writer.outline`, `et.readRange`
  types/linkedSheets, `wpp.listSlides detail`, cắt outline). Còn P0-09 (đo bằng bộ `17`) và hai chỗ lệch phía
  khác nền tảng (WPS/LibreOffice `writer.outline` chưa có bookmark/caption bảng).
- **R0 một phần**: mục 3.3 đã có `sameTaskGuard` + `heavyRunGuard`/`AskContinue` trong code; còn 3.1 (bộ kịch bản)
  và 3.2 (bộ chạy) — tức chính cái thước đo vẫn chưa có.
- **M1 một phần**: lọc hội thoại theo tài liệu đang mở đã có (`GET /v1/conversations?documentKey=…`,
  `ConversationsForm`); còn nối theo vân tay khi đổi tên và kịch bản "không lẫn".
- **P1 một phần**: P1-01 thêm hai chỗ hỏi trước (`wpp.deleteShapes`/`deleteTagged`, `et.importCsv` ghi đè); hoàn tác
  cả lượt + chụp vùng còn kiểm bằng tay trên Office thật.
- **P2 một phần**: P2-01 xong (`not_for` cho 8 skill + hiện cho model; loader/prompt/test). P2-02…05 (rà nội dung,
  vùng cấm, kịch bản, đo bằng bộ `17`) còn.
- **P3/memory (M2–M5) phần lớn xong**: M2 `Redact` + ẩn PII trong fact; M3 `org-style` + phân quyền ghi + prompt
  riêng + **nhánh memory theo app** (17/10, M3-07: `org-style`/`doc-style` khoá bằng `scope_key`); M4 export/import
  JSON, xóa mềm theo tài liệu, tắt riêng scope; M5 job dọn định kỳ (gộp gần trùng, hết hạn).
  Còn: UI `MemoryForm`/`SettingsForm` (M3-05/M4-03), số đo M5-03, bộ đo `17`.
- **P4 một phần**: P4-01 registry `Profile` xong (`client.go`/`types.go` đọc registry). Còn P4-02 (nhớ quirk qua khởi
  động lại - vướng schema `core.db` dùng chung và quyền ghi HKCU) và P4-03 (admin/wizard, C#).
- **P5 tạm hoãn (17/10/2026)**: `26` **TẠM HOÃN** — đã chốt cơ chế (Task Scheduler gọi `job` một lần, `jobs.db` riêng,
  nhường Core đang chạy) nhưng vướng Q7: job "tính thật trong app" mâu thuẫn "chạy khi Office đóng". Thiết kế đã chốt
  ghi ở `06-p5` mục 6; phần nhánh memory theo app (Q1–Q3) đã tách sang `12-m3` mục 7.
- **P6 phần lớn xong (Core)**: P6-01 ẩn bí mật trong audit, P6-02 hạn lưu audit + dọn lúc khởi động, P6-04 lọc nhật
  ký theo tài liệu. Còn P6-02 nút admin + chạy định kỳ, P6-03 (hoàn tác - C#).
- **P7 một phần**: P7-03 xong (câu trạng thái + thẻ hỏi kiểu trình bày theo ngôn ngữ gửi kèm). Còn dịch wizard/form
  (P7-01), khung chat WPS/LibreOffice (P7-02), lỗi hay gặp (P7-04), nhãn Core trên ribbon (P7-05/06), thử người thật.
- **P8 không nằm trong phạm vi** đợt này (chỉ kiểm trên Microsoft Office máy này).

## Thứ tự làm

| Thứ tự | Tệp | Nội dung | Ghi chú |
|---|---|---|---|
| 1 | `17-r0-model-yeu-va-do-luong.md` + `18` | Bộ đo theo kịch bản với model yếu, chặn loop/đốt token | **Mới.** Là thước đo cho mọi phase sau; 3.3 xong, 3.1/3.2 còn |
| 2 | `01-p0-hieu-tai-lieu-sau.md` + `15` | Đọc đúng tài liệu: mở rộng lệnh đọc có sẵn | P0-01…07 xong 16/10; còn P0-09 + WPS/LO outline |
| 3 | `02-p1-duyet-truoc-khi-luu.md` + `16` | Sửa → xem lại → hoàn tác cả lượt; chụp vùng cho Excel | Thay "preview diff"; P1-01 thêm hai chỗ hỏi |
| 4 | `03-p2-skill-chuan-van-phong.md` + `19` | Chuẩn nội dung 8 skill, kiểm bằng tiêu chí cấu trúc | Bỏ golden chéo C#/Go |
| 5 | `04-p3-nho-theo-tai-lieu.md` + `10`–`14`, `20`–`24` | Memory: phần còn thiếu (export/import, org-style, ẩn PII, số đo) | Phần lớn đã có (mốc G4); M1 phần lọc theo tệp đã có |
| 6 | `05-p4-provider-va-toolset-gon.md` + `25` | Registry provider khai báo | Toolset filter phần lớn đã có (chế độ lập kế hoạch) |
| — | `06-p5-bao-cao-dinh-ky-toi-gian.md` + `26` | Báo cáo định kỳ | **TẠM HOÃN 17/10/2026** (vướng Q7, xem `06-p5` mục 6) |
| 7 | `07-p6-audit-va-an-toan-du-lieu.md` + `27` | Ẩn thông tin nhạy cảm trong audit, bản chụp để khôi phục | Audit đã có; `memory/redact.go` chưa tồn tại (làm ở M2) |
| 8 | `08-p7-ux-trong-app.md` + `28` | Dịch nốt giao diện, lỗi tiếng Việt dễ hiểu | Phần lớn đã có |
| 9 | `09-p8-libreoffice-parity.md` + `29` | Các lệnh còn để trống phía LibreOffice | LibreOffice đã chạy 281/281 |

**Tệp task chi tiết** (`15`–`29`): P0 = `15`, P1 = `16`, R0 = `18`, P2 = `19`, memory M1–M5 = `20`–`24`,
P4 = `25`, P5 = `26`, P6 = `27`, P7 = `28`, P8 = `29`. Mẫu chung: chia tới từng hàm, mỗi task có tệp sửa,
cách thử và lệnh đối chiếu; làm xong đánh dấu `[x]`.

## Nguyên tắc

- Mỗi phase có **tiêu chí xong đo được**: kịch bản nào, model nào, chạy mấy lần, so với mốc nào (bộ đo ở `17`).
  "Chạy được trên máy mình một lần" không phải tiêu chí xong.
- Người dùng chính chạy **model free yếu** (mimo, deepseek flash): mọi tối ưu và mọi phép đo nhắm vào model yếu.
- Commit nhỏ theo tính năng, message tiếng Việt có dấu, mỗi commit build được; ghi `CHANGELOG.md [Unreleased]`.
- Danh mục lệnh sinh tự động từ registry Python (`scripts/generate_mcp_commands.py`): thêm lệnh ở registry,
  không sửa tay `catalog/live-commands.json`, `commands.js`, `livecommands_gen.go`.
- Không ôm terminal/trình duyệt/computer-use/kanban, không chat đa nền tảng, không tự sinh skill.

## Việc ngoài các phase (chưa có tệp riêng)

- Đóng gói, cập nhật, ký số DLL và exe (ARCHITECTURE.MD ghi DLL chưa ký số).
- Chất lượng tính năng trình bày: `doc_design` (Word), `slides_from_template` (PowerPoint) - đo bằng bộ `17`.
- Dịch giao diện: xong ribbon + khung chat Windows; còn wizard, các form quản lý, khung chat WPS/LibreOffice.
