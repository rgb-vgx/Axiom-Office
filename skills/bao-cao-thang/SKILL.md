---
name: bao-cao-thang
description: >-
  Dựng bộ slide báo cáo tháng/quý trong PowerPoint: tóm tắt cho người quyết định, bảng điểm chỉ tiêu,
  phân tích chênh lệch, vấn đề - rủi ro, việc cần quyết và kế hoạch kỳ tới. Dùng khi người dùng muốn
  làm "slide báo cáo", "báo cáo tháng", "báo cáo quý", "slide tổng kết", "trình bày kết quả công việc".
apps: [wpp]
---

# Slide báo cáo tháng

Dùng cùng skill `trinh-bay-chuyen-nghiep` (kích thước slide, lưới, cỡ chữ, sức chứa, soát bố cục) —
skill đó là nguồn duy nhất cho mọi con số toạ độ. Ở đây chỉ nói **thứ tự slide**; mẫu bố cục từng kiểu
slide (bìa, ba số, bảng, hai cột, chia mục) ở
`read_skill_file {name: "trinh-bay-chuyen-nghiep", path: "references/bo-cuc-slide.md"}`.

## Cấu trúc mặc định (7–8 slide)

1. **Bìa**: "Báo cáo tháng MM/YYYY" + đơn vị · người trình bày.
2. **Tóm tắt cho người quyết định** — slide quan trọng nhất, đừng bỏ: Mục tiêu · Bối cảnh · Vấn đề ·
   Cách giải quyết · Lợi ích · Việc cần làm. Người chỉ đọc slide này vẫn ra được quyết định.
3. **Bảng điểm chỉ tiêu**: bảng "Chỉ tiêu | Mục tiêu | Thực tế | % đạt | Trạng thái", ≤ 6 dòng. Đây là
   chỗ để số liệu, không kể số trong đoạn văn.
4. **Phân tích chênh lệch** (1–2 slide): mỗi slide **một** nguyên nhân lớn giải thích chỉ tiêu lệch
   nhiều nhất, kèm số chứng minh. Nhiều nguyên nhân ngang nhau thì tách slide, đừng nhồi.
5. **Vấn đề và rủi ro**: ≤ 4 dòng, mỗi dòng nêu vấn đề **và** cách xử lý (ai, khi nào).
6. **Việc cần quyết**: mỗi dòng một việc cần người có thẩm quyền quyết — *quyết cái gì · vì sao bây giờ ·
   không quyết thì sao*. Slide này là lý do tồn tại của cả bộ báo cáo.
7. **Kế hoạch kỳ tới**: ≤ 4 việc, mỗi việc có người phụ trách hoặc mốc thời gian.
8. *(tuỳ chọn)* **Phụ lục**: bảng chi tiết, để cuối vì người đọc tự nhảy tới khi cần.

Báo cáo gửi kèm để đọc nên 7–8 slide là đủ; nếu người dùng muốn bản trình bày miệng 30 phút thì thêm
slide phân tích và đẩy chi tiết vào `wpp.setNotes`, đừng nhồi chữ lên slide.

Người dùng đưa số liệu thì dùng đúng số đó; thiếu số thì ghi chỗ trống rõ ràng ("[doanh thu]") và nói
trong câu trả lời — **không bịa số liệu**, kể cả số "ước tính hợp lý".

## Checklist

- [ ] `wpp.listSlides` — bài đang có gì (thêm vào cuối, không xoá slide cũ trừ khi được yêu cầu).
- [ ] `wpp.checkLayout` lấy `slideWidth`/`slideHeight`; tính lưới theo `trinh-bay-chuyen-nghiep`.
- [ ] Mỗi slide: `wpp.addSlide {layout: 12}` → hộp tiêu đề theo lưới → nội dung theo mẫu bố cục.
- [ ] Đọc liền các tiêu đề: phải thành một mạch (kết quả → vì sao → xin gì).
- [ ] Chi tiết/diễn giải → `wpp.setNotes`.
- [ ] **Soát** `wpp.checkLayout` **một lần** sau khi dựng xong hết slide — mặc định nó đã kiểm **mọi**
      slide, nên gọi từng slide là thừa lượt. Sửa đến khi `issueCount` = 0. Nhớ: nó **không** kiểm được
      chữ trong bảng, nên bảng phải tự đếm dòng (≤ 6).
