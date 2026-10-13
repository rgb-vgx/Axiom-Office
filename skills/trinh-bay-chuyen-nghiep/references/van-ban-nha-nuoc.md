# Slide từ văn bản của cơ quan Đảng, Nhà nước

Dùng khi nguồn là nghị quyết, kế hoạch, chương trình hành động, báo cáo sơ kết/tổng kết, chuyên đề bồi
dưỡng của cơ quan Đảng, chính quyền, Mặt trận, đoàn thể (đầu văn bản có "ĐẢNG CỘNG SẢN VIỆT NAM", "CỘNG
HOÀ XÃ HỘI CHỦ NGHĨA VIỆT NAM", "ỦY BAN NHÂN DÂN…", "ĐẢNG ỦY…", số ký hiệu kiểu `-NQ/TW`, `/KH-UBND`).
Rút từ bộ skill slide cho cán bộ, công chức đã dùng thật; lưới, cỡ chữ, sức chứa vẫn theo `SKILL.md`.

Người xem là cán bộ trong hội nghị hoặc lớp bồi dưỡng: họ cần **đúng nguyên văn** và sự trang trọng. Hai
ưu tiên trên hết là **giữ trọn nội dung** và **giữ đúng lưới**.

## 1. Khác với `SKILL.md` mục 4

- **Không viết lại thành "tiêu đề kết luận".** Tiêu đề là **tên mục của văn bản**, chép đúng chữ
  ("Mục đích, yêu cầu", "Nhiệm vụ 3: Phát triển kinh tế số"). Viết hoa chữ đầu câu, giữ hoa tên riêng và
  tên văn bản. Tên phần lớn ở slide chia mục viết IN HOA kèm số La Mã ("II. NHIỆM VỤ, GIẢI PHÁP").
- **Giữ nguyên văn.** Diễn giải lại một chữ có thể đổi nghĩa văn bản chính trị – pháp lý. Chỉ được:
  - tách một đoạn nhiều vế (ngăn bằng dấu chấm phẩy) thành nhiều gạch đầu dòng;
  - mở chữ viết tắt rõ ràng ("TP" → "Thành phố");
  - sửa lỗi chính tả hiển nhiên ("TRIẺN", "gía");
  - thêm dấu chấm cuối câu.
- **Không rút gọn câu để vừa hộp.** Thứ tự xử lý chữ dài ở `SKILL.md` mục 3 đổi thành: (1) tách sang
  slide sau; (2) đưa phần giải thích, ví dụ sang `wpp.setNotes`. Không cắt chữ của câu nguyên văn.
- Ngoặc kép cong “ ” và gạch ngang – trong văn bản là đúng chuẩn, giữ nguyên.

## 2. Những chỗ được thêm, và phải báo lại

- **Nhãn ngắn** đầu mỗi ý (3–7 chữ, rút từ chính câu gốc), ví dụ `"• Chuyển biến nhận thức: Nâng cao…"`.
- **Ô tóm tắt** (slide bố cục, slide chia mục): chỉ điền **cụm từ khoá** rút từ câu gốc, không chép cả
  đoạn — chép cả đoạn là tràn hộp (đã gặp: tóm tắt 130–150 ký tự trong ô chỉ chứa ≈ 80).
- Chữ viết tắt đã mở, lỗi chính tả đã sửa, ý đã tách.

Mọi thứ ở mục này là phần **tự thêm**: liệt kê trong báo cáo cuối (`SKILL.md` mục 7) để người dùng duyệt.

## 3. Bìa — lấy từ đầu văn bản

Đọc 25 dòng đầu văn bản, chép đúng chữ:

| Hộp bìa | Lấy từ |
|---|---|
| Dòng nhỏ trên cùng (IN HOA) | tên cơ quan ban hành, kèm cơ quan cấp trên nếu có ("ĐẢNG BỘ TỈNH …" / "ĐẢNG ỦY XÃ …") |
| Tên bài | tên loại + trích yếu ("Kế hoạch thực hiện Nghị quyết số 57-NQ/TW …") |
| Dòng phụ | địa danh, ngày ("…, ngày 12 tháng 3 năm 2025") |

Trích yếu dài hơn sức chứa tên bìa (≤ 80 ký tự) thì để tên loại + số hiệu ở tên bìa, trích yếu xuống
dòng phụ. Không đoán cơ quan: văn bản không có đầu (nội dung dán vào) thì hỏi người dùng cơ quan nào
trình bày, chưa có câu trả lời thì bỏ trống dòng đó.

## 4. Đảng hay chính quyền — đừng nhầm

Đầu văn bản quyết định, không phải thân văn bản (văn bản UBND thường trích dẫn nghị quyết Đảng):

- "ĐẢNG CỘNG SẢN VIỆT NAM", "ĐẢNG ỦY …", "ĐẢNG BỘ …", "CHI BỘ …", ký hiệu `-NQ/ĐU`, `-KH/TW` → **cơ quan
  Đảng**, kể cả đảng uỷ xã, phường. Biểu trưng: cờ Đảng – cờ Tổ quốc, **không** Quốc huy.
- Quốc hiệu "CỘNG HOÀ XÃ HỘI CHỦ NGHĨA VIỆT NAM", "ỦY BAN NHÂN DÂN", "HỘI ĐỒNG NHÂN DÂN", sở, bộ →
  **chính quyền, cơ quan nhà nước**. Biểu trưng: Quốc huy.
- Mặt trận, đoàn thể: biểu trưng riêng của tổ chức.

wpp không có sẵn ảnh biểu trưng: chỉ chèn (`wpp.addImage`, chỉ truyền `width`) khi người dùng gửi ảnh
hoặc tệp mẫu đã có. Tệp mẫu đính kèm có Quốc huy mà văn bản là của cơ quan Đảng (hoặc ngược lại) →
**báo người dùng**, không tự xoá hay đổi. Không bao giờ tự kéo giãn, cắt, tô màu biểu trưng.

## 5. Dàn ý thường gặp

Kế hoạch, chương trình hành động (không có .pptx gốc) thường đi theo khung:

Bìa → Bố cục (các phần lớn) → Mục đích / Yêu cầu → chia mục cho từng phần → mỗi nhiệm vụ một slide
(dài thì "(tiếp theo)") → Phân công thực hiện → Lộ trình, tiến độ → Kết thúc.

Chọn mẫu bố cục theo loại nội dung: bảng ở `bo-cuc-slide.md` mục 7. Mỗi slide một ý chính; một ý dài hơn
sức chứa thì tách sang slide sau, tiêu đề thêm "(tiếp theo)", **đánh số ý liên tục** (slide sau bắt đầu
từ ý 4, không quay lại 1).

## 6. Số liệu, ảnh

- **Chỉ dùng số có trong văn bản** của người dùng. Không làm tròn, không tự tính tỉ lệ mới.
- **Chữ nằm trong ảnh** (bảng, sơ đồ chụp lại trong bài cũ) không đọc được bằng `wpp.listSlides`: có tool
  `look_at_document` thì nhìn slide rồi gõ lại thành bảng hoặc gạch đầu dòng. Giữ ảnh chụp thật (chân dung,
  ảnh hiện trường).
- **Mã QR**: giữ nguyên ảnh gốc, không cắt, nhắc người dùng quét thử trước buổi họp.
- Không lấy ảnh hội nghị khác để minh hoạ hoạt động của đơn vị. Không có ảnh thật thì chọn bố cục
  không ảnh.

## 7. Làm một phần

Người dùng chỉ xin một phần ("6 slide đầu", "phần II"): làm đúng phần đó, báo rõ đã làm đến đâu và đề
nghị làm tiếp phần còn lại. Không tự làm cả bài.
