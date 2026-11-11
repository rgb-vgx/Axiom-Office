# Bộ task test do GPT đề xuất

> Nguồn: phản hồi GPT ngày 12/11/2026, dùng để test tay Axiom Office trên Word/Excel/PowerPoint.
> Đây là bộ đề **tham khảo**, chưa phải bộ chính thức. Task nào chạy ổn mới chuyển sang
> `tests/bench/` hoặc `tests/live/tasks/` để tự động hoá.

## Prompt đã dùng

```text
Bạn là chuyên gia thiết kế bài kiểm thử cho AI agent làm việc với tài liệu văn phòng.
Hãy tạo một bộ task test thực tế cho sản phẩm dưới đây. Trả lời bằng tiếng Việt.

## 1. Sản phẩm

Axiom Office là agent sửa trực tiếp tài liệu ĐANG MỞ trong 3 ứng dụng:
- Word / WPS Writer — mã app `wps`, lệnh dạng `writer.*`
- Excel / WPS Spreadsheets — mã app `et`, lệnh dạng `et.*`
- PowerPoint / WPS Presentation — mã app `wpp`, lệnh dạng `wpp.*`

Agent chạy trên Windows (Office COM) và Linux (WPS). Người dùng là người không chuyên,
chạy model free/yếu (cửa sổ ngữ cảnh 8k–128k, có thể timeout 120 giây/request).

Kiến trúc cần biết khi ra đề:
- Agent có một tool duy nhất `office_action {action, params}`; `action` là lệnh bridge.
- Ví dụ lệnh: `writer.getText`, `writer.replaceMany`, `writer.insertStyledText`,
  `writer.insertTable`, `writer.formatTable`, `writer.checkLayout`;
  `et.readRange`, `et.writeRange`, `et.writeRanges`, `et.fillRange`, `et.addSheet`,
  `et.checkRange`, `et.addChart`, `et.pivot`, `et.formatRange`, `et.createTable`;
  `wpp.listSlides`, `wpp.addSlide`, `wpp.setShapeText`, `wpp.addShape`, `wpp.checkLayout`.
- Có sẵn skill hướng dẫn cho một số việc: văn bản hành chính, bảng điểm, báo cáo tháng,
  mô hình nhiều sheet, trình bày chuyên nghiệp, thiết kế văn phòng, chính tả tiếng Việt,
  công thức bảng tính, dịch thuật.
- Agent không có filesystem/shell; mọi thứ qua lệnh trên tài liệu đang mở.
- Có guard chống lặp, bắt đọc lại sau khi sửa, và cắt kết quả đọc quá lớn.

## 2. Mục tiêu bộ task

Tạo khoảng 18–24 task, chia 3 tầng:
- Smoke (6 task, 2 mỗi app, 3–5 phút): kiểm tra đường cơ bản.
- Core (10–12 task, 3–4 mỗi app, 5–10 phút): việc văn phòng thật.
- Stress/edge (6 task, 2 mỗi app, 10–15 phút): tài liệu dài, dữ liệu lớn, lỗi cài sẵn,
  nhiều bước, hoặc làm việc xuyên app (Word → PowerPoint).

Mỗi app phải có đủ: sửa nội dung, định dạng, bảng, và ít nhất một task nhiều bước.
Riêng Excel có thêm công thức, biểu đồ, dữ liệu lớn; PowerPoint có thêm bố cục,
số slide, chữ tràn khung; Word có thêm văn bản hành chính, mục lục, trang.

## 3. Năm thay đổi cần được kiểm tra

Thiết kế task để tự nhiên chạm vào các thay đổi sau (không cần nói với người dùng cuối):
1. Mô tả lệnh gọn: task cần dùng tham số tuỳ chọn thật (ví dụ căn lề, font, màu, border,
   số liệu định dạng) để phát hiện model thiếu hint mà gọi sai.
2. Ngân sách ngữ cảnh theo model: task dài cần giữ thông tin qua nhiều bước, hoặc đọc
   tài liệu lớn rồi sửa nhiều chỗ.
3. Tách prompt ổn định/động: task có 2–3 lượt trong CÙNG hội thoại, giữa các lượt người dùng
   đổi vùng chọn/slide/sheet — để xem cache và ngữ cảnh có ổn định không.
4. Checklist mỗi vòng: task 8–12 bước rõ ràng, dễ quên bước giữa chừng.
5. Đọc lại kết quả bị cắt: task đọc vùng rất lớn, thông tin cần nằm ở phần đuôi hoặc giữa
   (ngoài phần đầu/đuôi được giữ), buộc phải đọc tiếp mới làm đúng.

Ngoài ra phải có 3–4 task "đối chứng" không cố tình chạm thay đổi nào, để phát hiện regression.

## 4. Định dạng mỗi task

Với mỗi task, ghi rõ:
- `id`: ví dụ W01, E03, P02.
- `tier`: smoke / core / stress.
- `app`: wps / et / wpp.
- `title`: ngắn, tiếng Việt.
- `difficulty`: dễ / vừa / khó.
- `mục tiêu`: 1–2 câu.
- `tài liệu đầu vào`: mô tả chính xác cấu trúc và dữ liệu. Nếu tạo từ trống thì ghi rõ
  cần tạo gì; nếu là tài liệu có sẵn thì mô tả đủ để người khác dựng lại (số đoạn, heading,
  bảng, ô, sheet, slide, dữ liệu mẫu cụ thể).
- `prompt người dùng`: câu người dùng thật sẽ gõ, tiếng Việt tự nhiên, không nhắc tên lệnh.
- `tiêu chí đạt`: danh sách kiểm tra nhị phân, chấm được bằng XML/COM hoặc đọc ô/slide;
  ví dụ "đúng 5 vị trí được thay", "ô D2 có công thức =SUM(...)", "đủ 8 slide",
  "không còn đoạn trống", "bảng có 5 cột". Tránh tiêu chí mơ hồ kiểu "trông đẹp".
- `điều không được làm`: ví dụ không sửa ngoài phạm vi, không hardcode kết quả,
  không xoá dữ liệu gốc, không đổi tên sheet.
- `kiểm tra thủ công` (nếu cần): 1–2 câu về bố cục/thẩm mỹ.
- `chạm thay đổi nào`: ghi 1 trong 5 mục ở trên, hoặc "đối chứng".
- `dấu hiệu cần quan sát`: 2–3 dấu hiệu cụ thể.
- `thời gian gợi ý`: ước lượng phút.

## 5. Yêu cầu chất lượng

- Task phải giống việc văn phòng Việt Nam thật: đơn xin việc, tờ trình, báo cáo tháng,
  bảng điểm, danh sách nhân viên, bảng lương, kế hoạch, slide thuyết trình.
- Prompt người dùng phải tự nhiên, ngắn, có thể thiếu thông tin — không viết như đặc tả kỹ thuật.
- Tiêu chí đạt phải kiểm được, không phụ thuộc "LLM thấy đẹp".
- Không yêu cầu lệnh/app không có.
- Task stress phải vẫn làm được trong ~15 phút với model yếu, không phải bài 1 tiếng.
- Có ít nhất 2 task "sửa tài liệu có lỗi cài sẵn".
- Có ít nhất 1 task Word → PowerPoint.
- Với Excel, nêu rõ dữ liệu mẫu bằng bảng markdown nhỏ.
- Với PowerPoint, nêu rõ số slide, tiêu đề từng slide, số hình/khung cần có.

## 6. Định dạng trả lời

Phần 1: bảng tổng quan:
| id | tier | app | title | độ khó | phút | chạm thay đổi |

Phần 2: chi tiết từng task theo mẫu ở mục 4.

Phần 3: một bản JSON/YAML rút gọn của cùng bộ task, mỗi task gồm các trường
`id, tier, app, title, user_prompt, acceptance[], must_not[], feature, watch_for`.
Bản này để tôi đưa vào script test, nên đừng thêm chữ ngoài JSON.

Hãy ưu tiên task có thể chấm tự động; chỉ dùng "kiểm tra thủ công" cho phần bố cục/thẩm mỹ.
Đừng tạo task chỉ để khoe tính năng — mỗi task phải là một việc người dùng thật sẽ nhờ làm.
```

## Phản hồi


> Đã tách bản JSON máy đọc sang [gpt-test-tasks.json](gpt-test-tasks.json). Đây là bộ đề nháp để test tay; task nào ổn mới chuyển vào `tests/bench/` hoặc `tests/live/tasks/`.

## 1. Tổng quan

Bộ **22 task**: **6 smoke**, **10 core**, **6 stress**. Phân bố thực tế theo app là Word **8**, Excel **7**,
PowerPoint **7** (bản GPT ghi "6 task mỗi app" nhưng danh sách bên dưới là 8/7/7 — giữ theo danh sách).

| ID | Tier | App | Title | Độ khó | Phút | Chạm thay đổi |
|---|---|---|---|---|---|---|
| W01 | Smoke | wps | Sửa đơn xin việc | Dễ | 3 | Đối chứng |
| W02 | Smoke | wps | Chuẩn hóa bảng thông tin | Dễ | 4 | Đối chứng |
| E01 | Smoke | et | Tính bảng điểm | Dễ | 4 | Đối chứng |
| E02 | Smoke | et | Chuẩn hóa bảng nhân viên | Dễ | 4 | Đối chứng |
| P01 | Smoke | wpp | Sửa nội dung thuyết trình | Dễ | 3 | Đối chứng |
| P02 | Smoke | wpp | Thêm slide giới thiệu | Vừa | 5 | Đối chứng |
| W03 | Core | wps | Hoàn thiện báo cáo tháng | Vừa | 8 | Checklist 8–12 bước |
| W04 | Core | wps | Tạo mục lục và chuẩn hóa trang | Vừa | 8 | Tham số lệnh gọn |
| W05 | Core | wps | Hoàn thiện tờ trình hành chính | Vừa | 8 | Checklist 8–12 bước |
| W06 | Core | wps | Chỉnh sửa CV theo vị trí ứng tuyển | Vừa | 7 | Prompt ổn định/động |
| E03 | Core | et | Hoàn thiện bảng lương | Vừa | 8 | Tham số lệnh gọn |
| E04 | Core | et | Báo cáo doanh thu bằng biểu đồ | Vừa | 8 | Checklist 8–12 bước |
| E05 | Core | et | Lập kế hoạch nhân sự nhiều sheet | Khó | 10 | Prompt ổn định/động |
| P03 | Core | wpp | Báo cáo kết quả kinh doanh | Vừa | 8 | Tham số lệnh gọn |
| P04 | Core | wpp | Sửa slide bị tràn chữ | Vừa | 8 | Prompt ổn định/động |
| P05 | Core | wpp | Chuyển báo cáo Word thành slide | Khó | 10 | Checklist 8–12 bước |
| W07 | Stress | wps | Sửa tài liệu dài nhiều lỗi | Khó | 12 | Ngân sách ngữ cảnh |
| W08 | Stress | wps | Xử lý tài liệu dài và mục lục | Khó | 15 | Đọc lại kết quả bị cắt |
| E06 | Stress | et | Kiểm tra dữ liệu bán hàng lớn | Khó | 12 | Đọc lại kết quả bị cắt |
| E07 | Stress | et | Sửa mô hình doanh thu sai công thức | Khó | 15 | Ngân sách ngữ cảnh |
| P06 | Stress | wpp | Chuẩn hóa bộ slide 12 trang | Khó | 12 | Checklist 8–12 bước |
| P07 | Stress | wpp | Sửa bộ slide nhiều lỗi bố cục | Khó | 15 | Đọc lại kết quả bị cắt |

4 task đối chứng: **W01, W02, E01, P01** — kiểm tra đường thực thi cơ bản, không cố tình tạo áp lực lên
5 thay đổi kiến trúc.

### Ma trận độ phủ

| Khả năng | Word | Excel | PowerPoint |
|---|---|---|---|
| Sửa nội dung | Có | Có | Có |
| Định dạng | Có | Có | Có |
| Bảng | Có | Có | Có |
| Nhiều bước | Có | Có | Có |
| Dữ liệu lớn | Có | Có | Có |
| Lỗi cài sẵn | Có | Có | Có |
| Nhiều lượt hội thoại | Có | Có | Có |

## 2. Chi tiết từng task

Quy ước khi dựng benchmark:

- Mỗi task bắt đầu bằng một tài liệu đã được chuẩn bị sẵn, trừ khi ghi rõ tạo mới.
- Mỗi task chạy trong một hội thoại riêng, trừ task có chỉ dẫn nhiều lượt.
- Các lượt tiếp theo phải sử dụng cùng hội thoại và cùng tài liệu đang mở.
- Tiêu chí đạt là binary: đạt hoặc không đạt. Không dùng đánh giá cảm tính để thay thế kiểm tra cấu trúc.
- Kiểm tra sau sửa phải đối chiếu trạng thái thực tế của tài liệu, không chỉ dựa vào phản hồi của agent.

### A. Smoke tests — 6 task
#### W01 — Sửa đơn xin việc

Đối chứng
 Smoke

- id: W01
- tier: smoke
- app: wps
- title: Sửa đơn xin việc
- difficulty: dễ
- mục tiêu: Kiểm tra khả năng đọc, sửa nội dung văn bản cơ bản mà không làm thay đổi cấu trúc tài liệu.
- tài liệu đầu vào:
  - 1 trang A4.
  - 6 đoạn văn.
  - Tiêu đề: ĐƠN XIN VIỆC.
  - Đoạn 2 có câu: "Tôi mong muốn được lam việc tại Quý công ty."
  - Đoạn 4 có câu: "Tôi có kinh nghiêm làm việc với C++."
  - Các đoạn còn lại là thông tin cá nhân, vị trí ứng tuyển, lời cảm ơn và chữ ký.
- prompt người dùng:
Bạn kiểm tra và sửa lỗi chính tả trong đơn xin việc này giúp tôi. Giữ nguyên nội dung, chỉ sửa lỗi sai thôi nhé.




- tiêu chí đạt:
  - "lam việc" được sửa thành "làm việc".
  - "kinh nghiêm" được sửa thành "kinh nghiệm".
  - Không còn hai lỗi trên.
  - Số đoạn văn không thay đổi.
  - Không có đoạn văn bị xóa.
  - Nội dung ngoài hai vị trí sai được giữ nguyên.
- điều không được làm:
  - Không viết lại toàn bộ đơn.
  - Không tự thêm thông tin cá nhân.
  - Không thay đổi bố cục.
- kiểm tra thủ công: Kiểm tra tiêu đề và chữ ký còn nguyên vị trí tương đối.
- chạm thay đổi nào: Đối chứng.
- dấu hiệu cần quan sát:
  - Agent có đọc văn bản trước khi sửa không?
  - Có sửa đúng hai vị trí không?
  - Có đọc lại sau khi sửa không?
- thời gian gợi ý: 3 phút.
#### W02 — Chuẩn hóa bảng thông tin

Đối chứng
 Smoke

- id: W02
- tier: smoke
- app: wps
- title: Chuẩn hóa bảng thông tin
- difficulty: dễ
- mục tiêu: Kiểm tra định dạng bảng, font, căn lề và giữ nguyên dữ liệu.
- tài liệu đầu vào:
  - Một tài liệu Word.
  - Một bảng 5 hàng × 4 cột.
  - Hàng đầu là tiêu đề: STT, Họ tên, Phòng ban, Chức vụ.
  - 4 nhân viên:
    - Nguyễn Văn An | Kế toán | Nhân viên
    - Trần Thị Bình | Nhân sự | Trưởng nhóm
    - Lê Văn Cường | Kỹ thuật | Nhân viên
    - Phạm Thị Dung | Kinh doanh | Nhân viên
  - Bảng hiện chưa có màu nền, đường viền và căn chỉnh thống nhất.
- prompt người dùng:
Chỉnh lại bảng này cho dễ đọc nhé. Tiêu đề in đậm, nền xanh nhạt, chữ căn giữa. Các dòng dữ liệu căn giữa theo chiều dọc và có đường viền đầy đủ.




- tiêu chí đạt:
  - Bảng vẫn có 5 hàng × 4 cột.
  - Nội dung 16 ô không thay đổi.
  - Hàng đầu được in đậm.
  - Hàng đầu có màu nền xanh nhạt.
  - Nội dung các ô căn giữa theo chiều ngang.
  - Căn giữa theo chiều dọc.
  - Có border ở cả 4 cạnh bảng và đường phân cách các ô.
- điều không được làm:
  - Không thêm hoặc xóa hàng, cột.
  - Không thay đổi tên nhân viên.
- kiểm tra thủ công: Kiểm tra bảng không bị tràn lề trang.
- chạm thay đổi nào: Đối chứng.
- dấu hiệu cần quan sát:
  - Agent có truyền đúng tham số định dạng không?
  - Có áp dụng định dạng toàn bảng hay chỉ một ô?
  - Có kiểm tra lại bảng không?
- thời gian gợi ý: 4 phút.
#### E01 — Tính bảng điểm

Đối chứng
 Smoke

- id: E01
- tier: smoke
- app: et
- title: Tính bảng điểm
- difficulty: dễ
- mục tiêu: Kiểm tra đọc dữ liệu, ghi công thức và định dạng số trong Excel.
- tài liệu đầu vào: Sheet BangDiem, dữ liệu:

| STT | Họ tên | Toán | Văn | Anh | Trung bình |
|---|---|---|---|---|---|
| 1 | An | 8 | 7 | 9 | Trống |
| 2 | Bình | 6 | 8 | 7 | Trống |
| 3 | Cường | 9 | 9 | 8 | Trống |
| 4 | Dung | 5 | 6 | 7 | Trống |
| 5 | Hà | 7 | 8 | 9 | Trống |


- prompt người dùng:
Tính điểm trung bình cho từng học sinh, làm tròn 2 chữ số thập phân. Định dạng tiêu đề cho dễ nhìn và kẻ bảng giúp tôi.




- tiêu chí đạt:
  - F2:F6 có công thức =AVERAGE(C2:E2) tương ứng từng dòng.
  - Kết quả lần lượt: 8, 7, 8.67, 6, 8.
  - F2:F6 hiển thị 2 chữ số thập phân.
  - A1:F1 được in đậm.
  - A1:F6 có border.
  - Dữ liệu C2:E6 không thay đổi.
- điều không được làm:
  - Không ghi đè dữ liệu điểm gốc.
  - Không thay công thức bằng giá trị cố định.
  - Không đổi tên sheet.
- kiểm tra thủ công: Kiểm tra bảng không bị che mất tiêu đề.
- chạm thay đổi nào: Đối chứng.
- dấu hiệu cần quan sát:
  - Agent có dùng công thức hay hardcode?
  - Có đọc vùng dữ liệu trước khi tính không?
  - Có kiểm tra lại kết quả tính không?
- thời gian gợi ý: 4 phút.
#### E02 — Chuẩn hóa bảng nhân viên
- id: E02
- tier: smoke
- app: et
- title: Chuẩn hóa bảng nhân viên
- difficulty: dễ
- mục tiêu: Kiểm tra thao tác định dạng vùng, tạo bảng và thiết lập hiển thị.
- tài liệu đầu vào: Sheet NhanVien, vùng A1:E6:

| Mã NV | Họ tên | Phòng ban | Lương | Ngày vào làm |
|---|---|---|---|---|
| NV01 | An | Kế toán | 12000000 | 01/01/2024 |
| NV02 | Bình | Nhân sự | 15000000 | 15/03/2023 |
| NV03 | Cường | Kỹ thuật | 18000000 | 10/06/2022 |
| NV04 | Dung | Kinh doanh | 13000000 | 20/08/2024 |
| NV05 | Hà | Kỹ thuật | 16000000 | 05/02/2023 |


- prompt người dùng:
Làm bảng nhân viên này chuyên nghiệp hơn, định dạng lương có dấu phân cách hàng nghìn, ngày theo dd/mm/yyyy, hàng tiêu đề nổi bật và bật bộ lọc giúp tôi.




- tiêu chí đạt:
  - Vùng A1:E6 được tạo thành Excel Table hoặc có cấu trúc bảng và AutoFilter tương đương.
  - A1:E1 in đậm, có màu nền.
  - D2:D6 hiển thị dạng số có phân cách hàng nghìn.
  - E2:E6 hiển thị dd/mm/yyyy.
  - Có bộ lọc ở 5 cột.
  - Dữ liệu gốc không thay đổi.
- điều không được làm:
  - Không sắp xếp lại nhân viên.
  - Không thay đổi mức lương.
  - Không tạo sheet mới.
- kiểm tra thủ công: Kiểm tra tiêu đề không bị cắt.
- chạm thay đổi nào: Đối chứng.
- dấu hiệu cần quan sát:
  - Có tạo đúng Table hay chỉ tô màu thủ công?
  - Có sử dụng đúng định dạng ngày và số?
  - Có kiểm tra AutoFilter?
- thời gian gợi ý: 4 phút.
#### P01 — Sửa nội dung thuyết trình

Đối chứng
 Smoke

- id: P01
- tier: smoke
- app: wpp
- title: Sửa nội dung thuyết trình
- difficulty: dễ
- mục tiêu: Kiểm tra đọc slide và sửa đúng nội dung trên shape hiện hữu.
- tài liệu đầu vào:
  - PowerPoint 3 slide.
  - Slide 1: "BÁO CÁO KẾT QUẢ KINH DOANH 2025", 2 shape: tiêu đề và tên phòng ban.
  - Slide 2: "Doanh thu tăng 15%" và "Lợi nhuận tăng 8%".
  - Slide 3: "Kế hoạch năm 2026", có 3 bullet.
  - Slide 2 có lỗi "Lợi nhuận tăng 8%%".
- prompt người dùng:
Sửa lỗi gõ thừa dấu % ở slide 2 giúp tôi. Những nội dung khác giữ nguyên.




- tiêu chí đạt:
  - Slide 2 có nội dung "Lợi nhuận tăng 8%".
  - Số slide vẫn là 3.
  - Số shape không thay đổi.
  - Không có nội dung khác bị thay đổi.
- điều không được làm:
  - Không thêm slide.
  - Không thay theme.
  - Không viết lại nội dung.
- kiểm tra thủ công: Không cần.
- chạm thay đổi nào: Đối chứng.
- dấu hiệu cần quan sát:
  - Có xác định đúng slide và shape không?
  - Có thay đổi ngoài phạm vi không?
  - Có kiểm tra lại nội dung không?
- thời gian gợi ý: 3 phút.
#### P02 — Thêm slide giới thiệu
- id: P02
- tier: smoke
- app: wpp
- title: Thêm slide giới thiệu
- difficulty: vừa
- mục tiêu: Kiểm tra khả năng thêm slide, shape và điền nội dung theo bố cục đơn giản.
- tài liệu đầu vào:
  - PowerPoint 2 slide:
    1. Giới thiệu công ty.
    2. Sản phẩm và dịch vụ.
  - Slide 1 có 2 shape: tiêu đề và phụ đề.
  - Slide 2 có 3 shape: tiêu đề và 2 khung nội dung.
- prompt người dùng:
Thêm một slide sau slide 1 để giới thiệu đội ngũ của công ty. Có tiêu đề và 3 ý: Ban lãnh đạo, Đội ngũ kỹ thuật, Bộ phận kinh doanh.




- tiêu chí đạt:
  - Tổng cộng 3 slide.
  - Slide mới nằm ở vị trí thứ 2.
  - Tiêu đề là "ĐỘI NGŨ CỦA CHÚNG TÔI".
  - Có đúng 3 nội dung được yêu cầu.
  - Có ít nhất 4 shape nội dung trên slide mới.
  - Slide cũ giữ nguyên nội dung.
- điều không được làm:
  - Không xóa slide cũ.
  - Không thay đổi nội dung slide 1 và 2.
- kiểm tra thủ công: Slide mới phải có tiêu đề và ba nội dung dễ đọc.
- chạm thay đổi nào: Đối chứng.
- dấu hiệu cần quan sát:
  - Có đọc danh sách slide trước khi thêm không?
  - Có chèn đúng vị trí không?
  - Có kiểm tra lại số slide và shape không?
- thời gian gợi ý: 5 phút.

### B. Core tests — 10 task
#### W03 — Hoàn thiện báo cáo tháng
Checklist 10 bước
- id: W03
- tier: core
- app: wps
- title: Hoàn thiện báo cáo tháng
- difficulty: vừa
- mục tiêu: Kiểm tra khả năng hoàn thiện một báo cáo hành chính có heading, bảng và danh sách. Đặc biệt đánh giá agent có thực hiện đủ các bước định dạng và xác minh hay không.
- tài liệu đầu vào:
  - Word 3 trang, 12 đoạn.
  - Tiêu đề: BÁO CÁO KẾT QUẢ CÔNG TÁC THÁNG 10/2026.
  - 3 heading hiện đang là đoạn văn thường:
    - I. Kết quả thực hiện
    - II. Khó khăn, tồn tại
    - III. Kế hoạch tháng tiếp theo
  - Có 1 bảng 4 hàng × 3 cột:

| Công việc | Kế hoạch | Thực hiện |
|---|---|---|
| Dự án A | 10 | 10 |
| Dự án B | 8 | 6 |
| Dự án C | 5 | 5 |


- Có 3 đoạn mô tả kết quả và 2 đoạn mô tả khó khăn.
- prompt người dùng:
Hoàn thiện báo cáo này theo phong cách văn bản nội bộ công ty nhé. Tiêu đề và các mục phải rõ ràng, bảng dễ đọc, bổ sung phần kết luận ngắn ở cuối và đánh số trang.




- tiêu chí đạt:
  - Tiêu đề được căn giữa và in đậm.
  - Ba mục chính được chuyển thành Heading 1.
  - Bảng vẫn có 4 hàng × 3 cột.
  - Hàng đầu của bảng in đậm.
  - Bảng có đầy đủ border.
  - Bảng có căn giữa theo chiều dọc.
  - Có thêm đúng một heading "IV. Kết luận".
  - Có ít nhất một đoạn kết luận dưới heading IV.
  - Có số trang trong footer.
  - Nội dung 3 dự án giữ nguyên.
- điều không được làm:
  - Không thay đổi số liệu thực hiện.
  - Không tự bịa thêm kết quả công việc.
  - Không xóa phần khó khăn.
- kiểm tra thủ công: Kiểm tra tiêu đề, bảng và footer không bị lệch.
- chạm thay đổi nào: Checklist mỗi vòng.
- dấu hiệu cần quan sát:
  - Agent có lập và theo dõi đủ 10 bước không?
  - Có bỏ sót số trang hoặc kết luận không?
  - Có đọc lại tài liệu sau khi hoàn thành không?
- thời gian gợi ý: 8 phút.
#### W04 — Tạo mục lục và chuẩn hóa trang
- id: W04
- tier: core
- app: wps
- title: Tạo mục lục và chuẩn hóa trang
- difficulty: vừa
- mục tiêu: Kiểm tra heading hierarchy, mục lục và các tham số định dạng trang cụ thể.
- tài liệu đầu vào:
  - Word 5 trang.
  - 1 tiêu đề chính.
  - 3 mục Heading 1:
    - Chương 1. Tổng quan
    - Chương 2. Quy trình
    - Chương 3. Kết quả
  - Mỗi chương có 2 mục Heading 2.
  - Tổng cộng 6 Heading 2.
  - Chưa có mục lục.
  - Khổ giấy A4, lề hiện tại: trái 2 cm, phải 2 cm, trên 2 cm, dưới 2 cm.
- prompt người dùng:
Đây là tài liệu hướng dẫn nội bộ. Thêm mục lục ở đầu, chuẩn hóa về A4, lề trái 3 cm, phải 2 cm, trên dưới 2 cm. Font Times New Roman 13, giãn dòng 1.5. Giữ nguyên nội dung nhé.




- tiêu chí đạt:
  - Có mục lục ở trước Chương 1.
  - Mục lục có 3 Heading 1 và 6 Heading 2.
  - Các heading có đúng cấp.
  - Khổ giấy A4.
  - Lề trái 3 cm, phải 2 cm, trên 2 cm, dưới 2 cm.
  - Font nội dung chính Times New Roman, cỡ 13.
  - Line spacing 1.5.
  - Nội dung gốc không bị mất.
- điều không được làm:
  - Không chuyển tất cả heading thành cùng một cấp.
  - Không xóa các đoạn nội dung.
  - Không tự viết lại chương.
- kiểm tra thủ công: Kiểm tra mục lục không bị tràn và số trang tương ứng.
- chạm thay đổi nào: Mô tả lệnh gọn.
- dấu hiệu cần quan sát:
  - Agent có truyền đủ thông số margin/font/spacing không?
  - Có tạo mục lục dựa trên heading thật không?
  - Có cập nhật hoặc kiểm tra mục lục sau chỉnh sửa không?
- thời gian gợi ý: 8 phút.
#### W05 — Hoàn thiện tờ trình hành chính
- id: W05
- tier: core
- app: wps
- title: Hoàn thiện tờ trình hành chính
- difficulty: vừa
- mục tiêu: Kiểm tra xử lý văn bản hành chính Việt Nam, cấu trúc nội dung, bảng dự toán và bố cục ký duyệt.
- tài liệu đầu vào:
  - Word 2 trang, 10 đoạn.
  - Tiêu đề hiện có: TỜ TRÌNH VỀ VIỆC MUA SẮM THIẾT BỊ.
  - Có thông tin đơn vị: Phòng Công nghệ thông tin.
  - Có nội dung đề xuất mua 5 máy tính.
  - Có bảng 4 hàng × 3 cột:

| Thiết bị | Số lượng | Đơn giá |
|---|---|---|
| Máy tính | 5 | 18000000 |
| Màn hình | 5 | 3500000 |
| Bàn phím | 5 | 500000 |


- Chưa có dòng tổng cộng.
- Cuối tài liệu có 2 đoạn ký tên chưa căn chỉnh.
- prompt người dùng:
Chỉnh lại tờ trình này theo thể thức văn bản hành chính. Thêm dòng tổng cộng cho bảng dự toán, làm rõ phần kính gửi và phần đề nghị, cuối văn bản bố trí hai vị trí ký: người lập và trưởng phòng.




- tiêu chí đạt:
  - Có phần Kính gửi.
  - Có phần Nội dung đề nghị.
  - Có dòng tổng cộng trong bảng.
  - Tổng tiền đúng 110.000.000 đồng.
  - Bảng có 5 hàng × 3 cột.
  - Có hai vị trí ký: Người lập và Trưởng phòng.
  - Không mất thông tin đơn vị.
  - Không mất nội dung đề xuất mua 5 máy tính.
- điều không được làm:
  - Không thay đổi số lượng và đơn giá.
  - Không tự tạo tên người ký.
  - Không thêm nội dung phê duyệt giả.
- kiểm tra thủ công: Kiểm tra bố cục phần quốc hiệu/tiêu ngữ nếu tài liệu đầu vào có sẵn và hai vị trí ký.
- chạm thay đổi nào: Checklist mỗi vòng.
- dấu hiệu cần quan sát:
  - Có thực hiện đầy đủ các bước nội dung, bảng, tổng tiền, chữ ký và xác minh không?
  - Có tính tổng từ dữ liệu gốc hay tự đoán?
  - Có làm hỏng bố cục hành chính không?
- thời gian gợi ý: 8 phút.
#### W06 — Chỉnh sửa CV theo vị trí ứng tuyển
Hai lượt hội thoại
- id: W06
- tier: core
- app: wps
- title: Chỉnh sửa CV theo vị trí ứng tuyển
- difficulty: vừa
- mục tiêu: Kiểm tra khả năng giữ ngữ cảnh giữa hai lượt, đồng thời xử lý thay đổi vùng nội dung được người dùng yêu cầu.
- tài liệu đầu vào:
  - CV 2 trang.
  - 5 heading: Thông tin cá nhân, Mục tiêu nghề nghiệp, Kinh nghiệm, Kỹ năng, Học vấn.
  - Kinh nghiệm có 2 công việc.
  - Kỹ năng có 6 bullet.
  - Có một bảng 2 hàng × 2 cột chứa thông tin liên hệ.
- prompt người dùng:
Lượt 1:

Giúp tôi chỉnh CV này để ứng tuyển Backend Developer. Làm phần kinh nghiệm và kỹ năng nổi bật hơn, định dạng lại cho chuyên nghiệp. Đừng tự thêm kinh nghiệm không có thật.

Lượt 2:

Tôi vừa chọn phần Kinh nghiệm. Chỉnh riêng phần này giúp tôi: làm nổi bật các công việc liên quan backend và giữ nguyên thứ tự thời gian.




- tiêu chí đạt:
  - Có đủ 5 heading ban đầu.
  - Có đủ 2 công việc.
  - Không phát sinh công ty hoặc thời gian làm việc mới.
  - Phần kỹ năng có đủ 6 kỹ năng ban đầu.
  - Lượt 2 chỉ tác động phần Kinh nghiệm.
  - Thứ tự thời gian không thay đổi.
  - Bảng liên hệ vẫn có 2 hàng × 2 cột.
- điều không được làm:
  - Không tự thêm công nghệ người dùng chưa khai báo.
  - Không xóa kinh nghiệm cũ.
  - Không chỉnh sửa phần Học vấn ở lượt 2.
- kiểm tra thủ công: Kiểm tra độ dễ đọc của CV sau định dạng.
- chạm thay đổi nào: Tách prompt ổn định/động.
- dấu hiệu cần quan sát:
  - Có giữ đúng ngữ cảnh của lượt 1 sang lượt 2 không?
  - Có nhận biết vùng chọn hiện tại thay đổi không?
  - Có vô tình chỉnh sửa toàn bộ tài liệu ở lượt 2 không?
- thời gian gợi ý: 7 phút.
#### E03 — Hoàn thiện bảng lương
- id: E03
- tier: core
- app: et
- title: Hoàn thiện bảng lương
- difficulty: vừa
- mục tiêu: Kiểm tra công thức có tham chiếu tương đối, định dạng tiền tệ và tổng hợp dữ liệu.
- tài liệu đầu vào: Sheet LuongThang10, vùng A1:F6:

| Mã NV | Họ tên | Lương cơ bản | Phụ cấp | Ngày công | Thực nhận |
|---|---|---|---|---|---|
| NV01 | An | 12000000 | 1000000 | 26 | Trống |
| NV02 | Bình | 15000000 | 1500000 | 24 | Trống |
| NV03 | Cường | 18000000 | 2000000 | 26 | Trống |
| NV04 | Dung | 13000000 | 1000000 | 22 | Trống |
| NV05 | Hà | 16000000 | 1500000 | 25 | Trống |


- prompt người dùng:
Tính lương thực nhận theo số ngày công, quy ước một tháng có 26 ngày. Thêm tổng quỹ lương ở cuối bảng. Định dạng tiền có dấu phân cách hàng nghìn và thêm màu cho hàng tiêu đề.




- tiêu chí đạt:
  - F2 có công thức =(C2+D2)*E2/26.
  - F3:F6 có công thức tương ứng.
  - Giá trị F2:F6 lần lượt là 13.000.000; 15.230.769,23; 20.000.000; 11.846.153,85; 16.826.923,08.
  - F7 có công thức tổng F2:F6.
  - F7 có giá trị xấp xỉ 76.903.846,15.
  - C2:F7 có định dạng số có phân cách hàng nghìn.
  - A1:F1 in đậm, có màu nền.
  - Dữ liệu gốc A2:E6 không thay đổi.
- điều không được làm:
  - Không hardcode kết quả.
  - Không sửa mức lương, phụ cấp hoặc ngày công.
  - Không xóa dòng tổng.
- kiểm tra thủ công: Kiểm tra độ dễ đọc.
- chạm thay đổi nào: Mô tả lệnh gọn.
- dấu hiệu cần quan sát:
  - Có dùng công thức tham chiếu tương đối?
  - Có truyền đúng định dạng số?
  - Có kiểm tra lại công thức và tổng quỹ lương?
- thời gian gợi ý: 8 phút.
#### E04 — Báo cáo doanh thu bằng biểu đồ
Checklist 9 bước
- id: E04
- tier: core
- app: et
- title: Báo cáo doanh thu bằng biểu đồ
- difficulty: vừa
- mục tiêu: Kiểm tra khả năng đọc dữ liệu, tính tổng, tạo biểu đồ và trình bày báo cáo.
- tài liệu đầu vào: Sheet DoanhThu, vùng A1:C7:

| Tháng | Miền Bắc | Miền Nam |
|---|---|---|
| T1 | 120 | 100 |
| T2 | 140 | 110 |
| T3 | 130 | 125 |
| T4 | 160 | 140 |
| T5 | 180 | 150 |
| T6 | 170 | 165 |


Đơn vị: triệu đồng. Cột D đang trống.

- prompt người dùng:
Tôi cần báo cáo doanh thu 6 tháng đầu năm. Thêm tổng doanh thu từng tháng, tính tổng cả hai miền, tạo biểu đồ so sánh Bắc và Nam theo tháng. Trình bày bảng và biểu đồ gọn trong cùng sheet.




- tiêu chí đạt:
  - D1 có tên "Tổng doanh thu".
  - D2:D7 có công thức =SUM(B2:C2) tương ứng.
  - D2:D7 có giá trị 220, 250, 255, 300, 330, 335.
  - D8 có công thức tổng D2:D7, giá trị 1690.
  - Có ít nhất một biểu đồ dạng cột.
  - Biểu đồ có 2 series: Miền Bắc và Miền Nam.
  - Trục danh mục có đủ T1 đến T6.
  - Biểu đồ có tiêu đề.
  - Dữ liệu gốc không thay đổi.
- điều không được làm:
  - Không gộp dữ liệu hai miền thành một series duy nhất.
  - Không hardcode tổng doanh thu.
  - Không xóa sheet.
- kiểm tra thủ công: Biểu đồ không che bảng dữ liệu.
- chạm thay đổi nào: Checklist mỗi vòng.
- dấu hiệu cần quan sát:
  - Có thực hiện đủ 9 bước không?
  - Có chọn đúng vùng dữ liệu cho biểu đồ không?
  - Có đọc lại dữ liệu và kiểm tra biểu đồ sau khi tạo không?
- thời gian gợi ý: 8 phút.
#### E05 — Lập kế hoạch nhân sự nhiều sheet
Hai lượt hội thoại
- id: E05
- tier: core
- app: et
- title: Lập kế hoạch nhân sự nhiều sheet
- difficulty: khó
- mục tiêu: Kiểm tra khả năng làm việc trên nhiều sheet và duy trì thông tin giữa các lượt khi vùng sheet thay đổi.
- tài liệu đầu vào:
Sheet NhanSu:

| Mã NV | Họ tên | Phòng ban | Lương |
|---|---|---|---|
| NV01 | An | IT | 18000000 |
| NV02 | Bình | HR | 15000000 |
| NV03 | Cường | IT | 20000000 |
| NV04 | Dung | Sales | 14000000 |
| NV05 | Hà | HR | 16000000 |


Sheet KeHoach:

| Phòng ban | Số nhân viên | Quỹ lương |
|---|---|---|
| IT | Trống | Trống |
| HR | Trống | Trống |
| Sales | Trống | Trống |


- prompt người dùng:
Lượt 1:

Từ dữ liệu nhân sự, hoàn thiện sheet kế hoạch. Tính số nhân viên và tổng quỹ lương từng phòng ban. Thêm dòng tổng cộng.

Lượt 2:

Tôi chuyển sang sheet NhanSu rồi. Thêm cột phân loại lương: dưới 15 triệu là Thấp, từ 15 đến dưới 18 triệu là Trung bình, từ 18 triệu trở lên là Cao.




- tiêu chí đạt:
  - KeHoach!B2:B4 có công thức đếm theo phòng ban.
  - KeHoach!C2:C4 có công thức tính tổng lương theo phòng ban.
  - Kết quả B2:B4 là 2, 2, 1.
  - Kết quả C2:C4 là 38.000.000; 31.000.000; 14.000.000.
  - Có dòng tổng nhân sự = 5.
  - Có dòng tổng quỹ lương = 83.000.000.
  - Sheet NhanSu có thêm cột E tên Phân loại lương.
  - E2:E6 có công thức phân loại.
  - Kết quả lần lượt: Cao, Trung bình, Cao, Thấp, Trung bình.
  - Hai sheet cũ vẫn tồn tại.
- điều không được làm:
  - Không đổi tên sheet.
  - Không di chuyển dữ liệu nhân viên.
  - Không hardcode kết quả.
  - Không ghi cột phân loại vào sheet KeHoach.
- kiểm tra thủ công: Không cần.
- chạm thay đổi nào: Tách prompt ổn định/động.
- dấu hiệu cần quan sát:
  - Agent có giữ được tên sheet và yêu cầu ở lượt 1 không?
  - Có sử dụng đúng sheet đang được chọn ở lượt 2 không?
  - Có ghi nhầm kết quả vào sheet khác không?
- thời gian gợi ý: 10 phút.
#### P03 — Báo cáo kết quả kinh doanh
- id: P03
- tier: core
- app: wpp
- title: Báo cáo kết quả kinh doanh
- difficulty: vừa
- mục tiêu: Kiểm tra tạo slide, cấu trúc shape, nội dung số liệu và định dạng hình thức.
- tài liệu đầu vào:
  - PowerPoint có 3 slide:
    1. Bìa: Báo cáo kinh doanh quý III/2026.
    2. Kết quả kinh doanh.
    3. Kế hoạch quý IV.
  - Slide 2 có 3 shape: tiêu đề, doanh thu 5 tỷ, lợi nhuận 800 triệu.
  - Slide 3 có 3 shape: tiêu đề và hai nội dung kế hoạch.
- prompt người dùng:
Bổ sung vào slide kết quả kinh doanh 3 chỉ số: doanh thu 5 tỷ, lợi nhuận 800 triệu, tăng trưởng 15%. Làm số liệu nổi bật, dùng bố cục 3 khối ngang. Giữ nguyên các slide khác.




- tiêu chí đạt:
  - Tổng số slide vẫn là 3.
  - Slide 2 có tiêu đề.
  - Có đúng 3 khối nội dung số liệu.
  - Có đủ 3 số liệu chính xác.
  - Mỗi số liệu có một shape nội dung riêng.
  - Slide 1 và slide 3 không bị thay đổi nội dung.
  - Có tối thiểu 6 shape trên slide 2 (tiêu đề, 3 khối số liệu và 2 shape phụ trợ nếu sử dụng).
- điều không được làm:
  - Không thêm số liệu khác.
  - Không xóa slide.
  - Không sửa nội dung slide 3.
- kiểm tra thủ công: Ba khối số liệu phải nằm cùng một hàng, không chồng lấn.
- chạm thay đổi nào: Mô tả lệnh gọn.
- dấu hiệu cần quan sát:
  - Có sử dụng đúng tọa độ, kích thước shape?
  - Có truyền màu và căn chỉnh đúng?
  - Có kiểm tra layout sau khi tạo?
- thời gian gợi ý: 8 phút.
#### P04 — Sửa slide bị tràn chữ
Hai lượt hội thoại
- id: P04
- tier: core
- app: wpp
- title: Sửa slide bị tràn chữ
- difficulty: vừa
- mục tiêu: Kiểm tra xử lý overflow và khả năng nhận biết slide được chọn thay đổi giữa hai lượt.
- tài liệu đầu vào:
  - PowerPoint 4 slide.
  - Slide 1: tiêu đề, phụ đề.
  - Slide 2: tiêu đề, 1 textbox dài 450 ký tự, kích thước 10 × 2 cm.
  - Slide 3: tiêu đề, 1 textbox dài 500 ký tự, kích thước 10 × 2 cm.
  - Slide 4: tiêu đề, 2 textbox.
  - Slide 2 và 3 có textbox vượt quá chiều cao khung.
- prompt người dùng:
Lượt 1:

Slide 2 và 3 bị tràn chữ. Chỉnh lại cho vừa khung, ưu tiên giữ font dễ đọc, nếu cần thì tăng chiều cao khung.

Lượt 2:

Tôi đang chọn slide 4. Căn chỉnh hai khung nội dung trên slide này cho thẳng hàng và có khoảng cách đều nhau.




- tiêu chí đạt:
  - Slide 2 không còn text overflow.
  - Slide 3 không còn text overflow.
  - Slide 4 có đúng 2 textbox nội dung.
  - Hai textbox slide 4 có cùng chiều rộng.
  - Hai textbox slide 4 có cùng chiều cao.
  - Hai textbox có cùng tọa độ X nếu được xếp dọc.
  - Không thay đổi số slide.
  - Không xóa nội dung văn bản.
- điều không được làm:
  - Không xóa chữ để xử lý overflow.
  - Không chỉnh sửa slide 1.
  - Không tiếp tục chỉnh slide 2 và 3 ở lượt 2.
- kiểm tra thủ công: Đọc thử toàn bộ nội dung trên slide 2 và 3.
- chạm thay đổi nào: Tách prompt ổn định/động.
- dấu hiệu cần quan sát:
  - Có nhận ra slide đang chọn thay đổi ở lượt 2?
  - Có xử lý overflow thay vì chỉ giảm font tùy tiện?
  - Có kiểm tra lại layout của cả ba slide liên quan?
- thời gian gợi ý: 8 phút.
#### P05 — Chuyển báo cáo Word thành slide
- id: P05
- tier: core
- app: wpp
- title: Chuyển báo cáo Word thành slide
- difficulty: khó
- mục tiêu: Kiểm tra khả năng đọc tài liệu Word đang mở và chuyển nội dung thành bài thuyết trình có cấu trúc.
- tài liệu đầu vào:
  - Word đang mở có 7 đoạn và 2 bảng.
  - Heading:
    1. Tổng quan
    2. Kết quả
    3. Khó khăn
    4. Kế hoạch
  - Bảng 1 có 4 hàng × 3 cột: chỉ tiêu, kế hoạch, thực hiện.
  - Bảng 2 có 3 hàng × 2 cột: vấn đề, giải pháp.
  - Có 4 số liệu quan trọng: 120 nhân sự, 95% hoàn thành, 3 dự án, 2 dự án chậm tiến độ.
- prompt người dùng:
Từ báo cáo Word đang mở, tạo cho tôi một bài thuyết trình ngắn để báo cáo lãnh đạo. Khoảng 8 slide, có số liệu, khó khăn và kế hoạch. Đừng bỏ các thông tin quan trọng.




- tiêu chí đạt:
  - Có đúng 8 slide.
  - Slide 1: Tổng quan.
  - Slide 2: Mục tiêu và phạm vi.
  - Slide 3: Kết quả thực hiện.
  - Slide 4: Bảng số liệu.
  - Slide 5: Khó khăn.
  - Slide 6: Nguyên nhân và giải pháp.
  - Slide 7: Kế hoạch tiếp theo.
  - Slide 8: Kết luận.
  - Có ít nhất 4 shape nội dung ở mỗi slide, trừ slide bìa.
  - Số liệu 120, 95%, 3 và 2 được giữ chính xác.
  - Bảng số liệu có đủ 4 hàng × 3 cột nếu được thể hiện dưới dạng bảng.
  - Không có slide rỗng.
  - Word nguồn không bị sửa.
- điều không được làm:
  - Không bịa số liệu.
  - Không bỏ phần khó khăn.
  - Không sửa tài liệu Word nguồn.
- kiểm tra thủ công: Kiểm tra nội dung có mạch lạc, không nhồi quá nhiều chữ và có sự phân cấp thị giác.
- chạm thay đổi nào: Checklist mỗi vòng.
- dấu hiệu cần quan sát:
  - Có đọc và trích xuất đủ nội dung Word không?
  - Có lập kế hoạch 8 slide trước khi tạo không?
  - Có bỏ sót bảng hoặc số liệu không?
- thời gian gợi ý: 10 phút.

### C. Stress / Edge tests — 6 task
#### W07 — Sửa tài liệu dài nhiều lỗi
- id: W07
- tier: stress
- app: wps
- title: Sửa tài liệu dài nhiều lỗi
- difficulty: khó
- mục tiêu: Kiểm tra ngân sách ngữ cảnh khi đọc tài liệu dài, ghi nhớ yêu cầu và sửa nhiều vị trí rải rác.
- tài liệu đầu vào:
  - Word 25 trang.
  - Khoảng 180 đoạn văn.
  - 12 heading.
  - 4 bảng.
  - 6.000–8.000 từ.
  - Có 5 lỗi cài sẵn:
    1. Trang 2: "Công nghê thông tin".
    2. Trang 7: "quy trinh xử lý".
    3. Trang 13: "phân tich dữ liệu".
    4. Trang 19: "hiêu quả công việc".
    5. Trang 24: "triên khai hệ thống".
  - Các lỗi được đặt ở những vị trí không liền nhau.
- prompt người dùng:
Kiểm tra tài liệu này, sửa các lỗi chính tả tiếng Việt và chuẩn hóa cách viết thuật ngữ. Đặc biệt chú ý các từ liên quan đến công nghệ. Không viết lại nội dung.




- tiêu chí đạt:
  - Sửa đúng 5 lỗi đã cài.
  - Không còn các chuỗi lỗi.
  - Không mất đoạn văn.
  - Không thay đổi số heading.
  - Không thay đổi số bảng.
  - Không thay đổi nội dung số liệu trong bảng.
  - Không tạo thêm nội dung mới.
- điều không được làm:
  - Không thay toàn bộ văn bản bằng bản tóm tắt.
  - Không xóa đoạn dài để tiết kiệm ngữ cảnh.
  - Không tự sửa các thuật ngữ đúng.
- kiểm tra thủ công: Kiểm tra ngữ cảnh xung quanh 5 vị trí.
- chạm thay đổi nào: Ngân sách ngữ cảnh theo model.
- dấu hiệu cần quan sát:
  - Có đọc toàn bộ tài liệu hay chỉ phần đầu?
  - Có duy trì danh sách vị trí cần sửa qua nhiều lần gọi không?
  - Có đọc lại và xác minh đủ 5 vị trí không?
- thời gian gợi ý: 12 phút.
#### W08 — Xử lý tài liệu dài và mục lục
- id: W08
- tier: stress
- app: wps
- title: Xử lý tài liệu dài và mục lục
- difficulty: khó
- mục tiêu: Kiểm tra khả năng xử lý kết quả đọc lớn bị cắt, đặc biệt thông tin cần thiết nằm giữa tài liệu.
- tài liệu đầu vào:
  - Word 40 trang.
  - Khoảng 250 đoạn văn.
  - 20 heading.
  - 6 bảng.
  - 10.000–12.000 từ.
  - Heading đầu tiên ở trang 1.
  - Heading cuối cùng ở trang 39: "PHỤ LỤC D. QUY ĐỊNH TRIỂN KHAI".
  - Heading giữa trang 21: "CHƯƠNG IV. KIỂM SOÁT RỦI RO".
  - Tài liệu chưa có mục lục.
  - Mục đích: buộc agent phải phát hiện và đọc tiếp phần nội dung bị cắt, không chỉ sử dụng thông tin đầu/cuối.
- prompt người dùng:
Tạo mục lục cho toàn bộ tài liệu này. Tôi cần đủ các chương và phụ lục, kể cả phần kiểm soát rủi ro ở giữa tài liệu. Đặt mục lục sau trang bìa và giữ nguyên nội dung các chương.




- tiêu chí đạt:
  - Có mục lục sau trang bìa.
  - Mục lục chứa đủ 20 heading.
  - Có "CHƯƠNG IV. KIỂM SOÁT RỦI RO".
  - Có "PHỤ LỤC D. QUY ĐỊNH TRIỂN KHAI".
  - Không có heading trùng lặp.
  - Không bỏ heading ở phần giữa.
  - Không mất nội dung 6 bảng.
  - Không xóa nội dung gốc.
- điều không được làm:
  - Không tạo mục lục chỉ từ phần đầu/cuối.
  - Không tự suy đoán tên heading.
  - Không tóm tắt hoặc rút ngắn tài liệu.
- kiểm tra thủ công: Kiểm tra mục lục có đúng thứ tự và số trang.
- chạm thay đổi nào: Đọc lại kết quả bị cắt.
- dấu hiệu cần quan sát:
  - Agent có nhận biết kết quả đọc bị cắt không?
  - Có gọi đọc tiếp hoặc đọc theo vùng/đoạn không?
  - Có xác minh heading ở giữa tài liệu thay vì chỉ lấy đầu/cuối không?
- thời gian gợi ý: 15 phút.
#### E06 — Kiểm tra dữ liệu bán hàng lớn
- id: E06
- tier: stress
- app: et
- title: Kiểm tra dữ liệu bán hàng lớn
- difficulty: khó
- mục tiêu: Kiểm tra khả năng xử lý dữ liệu lớn, kết quả đọc bị cắt và phát hiện lỗi nằm ở giữa vùng dữ liệu.
- tài liệu đầu vào:
  - Sheet BanHang.
  - 2.000 dòng dữ liệu, hàng 1 là header.
  - Các cột: MaDH, Ngay, MaSP, SoLuong, DonGia, ThanhTien.
  - Dữ liệu tạo theo quy luật:
    - MaDH: DH0001 đến DH2000.
    - Ngày: từ 01/01/2026, tăng một ngày mỗi 10 đơn.
    - MaSP: SP01 đến SP20, luân phiên.
    - SoLuong: 1–5, lặp theo chu kỳ.
    - DonGia: 100000, 200000, 300000, 400000, 500000, luân phiên.
    - ThanhTien: ban đầu là giá trị trống.
  - Tạo lỗi cài sẵn ở hàng 1000: MaSP = SP99.
  - Tạo lỗi cài sẵn ở hàng 1500: SoLuong = -2.
  - Tạo lỗi cài sẵn ở hàng 1990: DonGia = -100000.
- prompt người dùng:
Kiểm tra bảng bán hàng này, tính thành tiền cho từng đơn, tìm các dòng dữ liệu bất thường và tổng hợp doanh thu theo mã sản phẩm. Tôi nghi có một vài dòng nhập sai ở đâu đó.




- tiêu chí đạt:
  - F2:F2001 có công thức =D2*E2 tương ứng.
  - Phát hiện SP99 tại hàng 1000.
  - Phát hiện số lượng âm tại hàng 1500.
  - Phát hiện đơn giá âm tại hàng 1990.
  - Có bảng tổng hợp theo mã sản phẩm.
  - Có đúng 20 mã sản phẩm hợp lệ trong bảng tổng hợp.
  - Không tự động xóa dữ liệu bất thường.
  - Không bỏ sót dữ liệu từ hàng 1000 trở đi.
- điều không được làm:
  - Không sửa hoặc xóa dòng lỗi.
  - Không thay đổi mã đơn hàng.
  - Không tự thay giá trị âm bằng số dương.
  - Không chỉ tính 100 dòng đầu.
- kiểm tra thủ công: Kiểm tra bảng tổng hợp có thể đọc được.
- chạm thay đổi nào: Đọc lại kết quả bị cắt.
- dấu hiệu cần quan sát:
  - Agent có phát hiện giới hạn dữ liệu đọc không?
  - Có đọc tiếp vùng giữa và cuối không?
  - Có giữ được thông tin bất thường khi chuyển sang bước tổng hợp không?
- thời gian gợi ý: 12 phút.
#### E07 — Sửa mô hình doanh thu sai công thức
- id: E07
- tier: stress
- app: et
- title: Sửa mô hình doanh thu sai công thức
- difficulty: khó
- mục tiêu: Kiểm tra khả năng phát hiện công thức sai, xử lý nhiều sheet và duy trì ngữ cảnh qua nhiều bước.
- tài liệu đầu vào:
Sheet DonHang:

| Mã đơn | Số lượng | Đơn giá | Thành tiền |
|---|---|---|---|
| DH01 | 2 | 100000 | 200000 |
| DH02 | 3 | 200000 | 600000 |
| DH03 | 1 | 300000 | 300000 |
| DH04 | 4 | 150000 | 600000 |
| DH05 | 2 | 500000 | 1000000 |


- Mở rộng dữ liệu thành 500 dòng theo quy luật luân phiên trên.
- Có công thức sai ở các dòng 100, 250 và 450.
- Sheet BaoCao có tổng doanh thu đang dùng vùng tham chiếu thiếu dòng cuối.
- Sheet TongHop có bảng doanh thu theo mã đơn nhưng thiếu một số công thức.
- prompt người dùng:
Mô hình doanh thu này có vẻ tính sai. Kiểm tra công thức ở các sheet, sửa những chỗ sai và đảm bảo báo cáo tổng hợp khớp với dữ liệu đơn hàng. Đừng thay đổi dữ liệu gốc.




- tiêu chí đạt:
  - Công thức thành tiền ở 500 dòng đều là số lượng × đơn giá.
  - Các công thức sai tại hàng 100, 250, 450 được sửa.
  - BaoCao tham chiếu đủ 500 đơn hàng.
  - TongHop có đủ dữ liệu tổng hợp.
  - Tổng doanh thu trên BaoCao bằng tổng thành tiền trên DonHang.
  - Không có công thức lỗi #REF!, #VALUE!, #DIV/0!.
  - Dữ liệu gốc không thay đổi.
- điều không được làm:
  - Không xóa các sheet.
  - Không thay công thức bằng kết quả cố định.
  - Không xóa đơn hàng.
  - Không bỏ qua các hàng ở giữa dữ liệu.
- kiểm tra thủ công: Không cần.
- chạm thay đổi nào: Ngân sách ngữ cảnh theo model.
- dấu hiệu cần quan sát:
  - Có xây dựng danh sách các vùng cần kiểm tra không?
  - Có giữ được quan hệ giữa ba sheet không?
  - Có kiểm tra tính nhất quán giữa các sheet sau sửa không?
- thời gian gợi ý: 15 phút.
#### P06 — Chuẩn hóa bộ slide 12 trang
- id: P06
- tier: stress
- app: wpp
- title: Chuẩn hóa bộ slide 12 trang
- difficulty: khó
- mục tiêu: Kiểm tra checklist nhiều bước, khả năng xử lý nhiều slide và tránh bỏ sót các bước định dạng.
- tài liệu đầu vào:
  - PowerPoint 12 slide.
  - Tiêu đề từng slide:
    1. Báo cáo tháng 10
    2. Tổng quan
    3. Kết quả kinh doanh
    4. Doanh thu
    5. Chi phí
    6. Lợi nhuận
    7. Nhân sự
    8. Tiến độ dự án
    9. Khó khăn
    10. Giải pháp
    11. Kế hoạch tháng 11
    12. Kết luận
  - Slide 1 có 2 shape.
  - Slide 2–12 mỗi slide có 4–6 shape.
  - Tất cả slide đang dùng font và màu không thống nhất.
  - Slide 5 và 9 có textbox tràn.
- prompt người dùng:
Chuẩn hóa toàn bộ bộ slide này thành một báo cáo tháng chuyên nghiệp. Đồng nhất font, màu sắc, tiêu đề, căn lề; sửa hai slide bị tràn chữ. Giữ nguyên thứ tự và nội dung các slide.




- tiêu chí đạt:
  - Tổng số slide vẫn là 12.
  - 12 tiêu đề không thay đổi.
  - Tất cả tiêu đề có cùng font, cỡ chữ và màu.
  - Các shape tiêu đề có cùng tọa độ Y.
  - Các shape nội dung không vượt khỏi vùng slide.
  - Slide 5 không còn overflow.
  - Slide 9 không còn overflow.
  - Không có slide bị rỗng.
  - Không xóa shape nội dung.
  - Không thay đổi thứ tự slide.
- điều không được làm:
  - Không thay đổi số liệu.
  - Không xóa slide.
  - Không rút ngắn nội dung để che lỗi tràn.
  - Không thay đổi nội dung báo cáo.
- kiểm tra thủ công: Kiểm tra tính nhất quán thị giác giữa 12 slide và khả năng đọc trên màn hình.
- chạm thay đổi nào: Checklist mỗi vòng.
- dấu hiệu cần quan sát:
  - Có lập checklist cho 12 slide không?
  - Có xử lý đủ 2 slide overflow không?
  - Có xác minh lại toàn bộ 12 slide sau khi định dạng không?
- thời gian gợi ý: 12 phút.
#### P07 — Sửa bộ slide nhiều lỗi bố cục
- id: P07
- tier: stress
- app: wpp
- title: Sửa bộ slide nhiều lỗi bố cục
- difficulty: khó
- mục tiêu: Kiểm tra đọc danh sách slide lớn, phát hiện lỗi ở giữa bộ slide và khả năng đọc lại kết quả bị cắt.
- tài liệu đầu vào:
  - PowerPoint 20 slide.
  - Tiêu đề:
    1. Tổng quan
    2. Giới thiệu
    3. Mục tiêu
    4. Phạm vi
    5. Quy trình
    6. Kiến trúc
    7. Thành phần
    8. Dữ liệu
    9. Kết quả
    10. Hiệu suất
    11. Rủi ro
    12. Bảo mật
    13. Chi phí
    14. Nhân sự
    15. Tiến độ
    16. Giải pháp
    17. Kế hoạch
    18. Đánh giá
    19. Đề xuất
    20. Kết luận
  - Mỗi slide có 4–8 shape.
  - Slide 2 có shape nằm ngoài mép phải.
  - Slide 10 có hai shape chồng lấn.
  - Slide 15 có textbox tràn.
  - Slide 19 thiếu một shape tiêu đề.
  - Slide 20 có một textbox rỗng.
- prompt người dùng:
Kiểm tra toàn bộ slide giúp tôi. Sửa các lỗi bố cục, shape bị chồng, chữ tràn và tiêu đề bị thiếu. Đặc biệt đừng bỏ qua các slide ở giữa nhé. Nội dung số liệu phải giữ nguyên.




- tiêu chí đạt:
  - Vẫn có 20 slide.
  - Slide 2 không có shape nằm ngoài mép phải.
  - Slide 10 không còn hai shape chồng lấn.
  - Slide 15 không còn overflow.
  - Slide 19 có đúng một shape tiêu đề.
  - Slide 20 không còn textbox rỗng.
  - Các tiêu đề slide khác không thay đổi.
  - Không có shape chứa số liệu bị xóa.
  - Không có slide nào bị mất nội dung.
- điều không được làm:
  - Không xóa slide.
  - Không xóa shape chứa số liệu.
  - Không thay đổi nội dung.
  - Không chỉ kiểm tra slide đầu và cuối.
- kiểm tra thủ công: Kiểm tra tính cân đối và khả năng đọc của các slide sau sửa.
- chạm thay đổi nào: Đọc lại kết quả bị cắt.
- dấu hiệu cần quan sát:
  - Có đọc tiếp danh sách slide khi kết quả bị cắt không?
  - Có phát hiện lỗi ở slide 10 và 15 không?
  - Có đọc lại từng slide liên quan sau khi sửa không?
- thời gian gợi ý: 15 phút.


## 3. Bản JSON cho script

Bản máy đọc nằm ở [gpt-test-tasks.json](gpt-test-tasks.json): 22 task, cùng các trường
`id, tier, app, title, user_prompt, acceptance, must_not, feature, watch_for`.
Dùng file này khi muốn đưa bộ đề vào script; bản Markdown giữ phần giải thích để đọc.

## 4. Khuyến nghị triển khai

Để bộ task này thực sự đo được chất lượng Axiom Office thay vì chỉ đo khả năng hoàn thành một vài thao tác,
có 5 nguyên tắc khi chạy.

### 1. Không chỉ chấm kết quả cuối

Nên lưu lại execution trace cho từng task:

| Trường | Ý nghĩa |
|---|---|
| `tool_calls` | Tổng số lần gọi `office_action` |
| `read_calls` | Số lần đọc tài liệu |
| `write_calls` | Số lần ghi/sửa |
| `retry_count` | Số lần retry |
| `timeout` | Có timeout hay không |
| `context_tokens` | Token sử dụng |
| `duration_ms` | Tổng thời gian |
| `verification_calls` | Số lần đọc xác minh sau sửa |
| `final_state` | Trạng thái tài liệu cuối |

### 2. Phân biệt lỗi agent và lỗi bridge

Một task thất bại cần được phân loại:

- `planning_failure`: Agent hiểu sai yêu cầu hoặc bỏ bước.
- `tool_selection_failure`: Chọn sai lệnh.
- `parameter_failure`: Gọi đúng lệnh nhưng tham số sai.
- `context_failure`: Quên thông tin trước đó.
- `truncation_failure`: Không đọc tiếp kết quả bị cắt.
- `bridge_failure`: Bridge thực thi sai hoặc lỗi.
- `verification_failure`: Không phát hiện kết quả sai sau khi ghi.
- `timeout`: Vượt giới hạn thời gian.

### 3. Chạy trên nhiều mức model

Không nên chỉ benchmark một model mạnh.

| Profile | Context | Timeout | Mục đích |
|---|---:|---:|---|
| Small | 8K | 120s | Kiểm tra model yếu |
| Medium | 32K | 120s | Mức triển khai phổ thông |
| Large | 128K | 120s | Đánh giá khả năng mở rộng |

Không bắt buộc tất cả task đều chạy ở cả ba profile. Nên ưu tiên W07, W08, E06, E07, P06, P07 cho phép thử context.

### 4. Quy định điều kiện PASS

Một task chỉ PASS khi đồng thời thỏa mãn:

1. Tất cả tiêu chí acceptance đạt.
2. Không vi phạm `must_not`.
3. Tài liệu thực tế được lưu đúng.
4. Không có lỗi bridge nghiêm trọng.
5. Thời gian thực thi nằm trong giới hạn.

Đặc biệt, agent nói đã hoàn thành không đồng nghĩa với task PASS. Phải kiểm tra tài liệu sau khi agent kết thúc.

### 5. Bổ sung các metric tổng hợp

| Metric | Công thức |
|---|---|
| Task Success Rate | Task PASS / Tổng task |
| Exactness | Số acceptance đạt / Tổng acceptance |
| Preservation Rate | Số dữ liệu ngoài phạm vi được giữ / Tổng dữ liệu cần giữ |
| Verification Rate | Task có xác minh sau sửa / Tổng task |
| Tool Efficiency | Số tool call trung bình / Task |
| Regression Rate | Task đối chứng PASS / Tổng task đối chứng |
| Timeout Rate | Task timeout / Tổng task |

Một lưu ý quan trọng: Với bộ 22 task này, tôi sẽ không dùng điểm trung bình duy nhất để quyết định chất lượng model. Một model có thể đạt 90% smoke/core nhưng thất bại hoàn toàn ở việc đọc tài liệu lớn. Vì vậy nên báo cáo riêng ba tầng và từng ứng dụng.

Bộ này phù hợp làm benchmark regression ban đầu cho Axiom Office, sau đó có thể bổ sung các biến thể dữ liệu và chạy lặp để đo độ ổn định giữa các lần thực thi.
