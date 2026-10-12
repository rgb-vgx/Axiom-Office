# Nội dung từng slide của báo cáo tháng

**Toạ độ không nằm ở đây.** Mọi vị trí lấy từ lưới trong `trinh-bay-chuyen-nghiep` (mục 1 của SKILL.md)
và mẫu bố cục ở `trinh-bay-chuyen-nghiep/references/bo-cuc-slide.md`. Cố tình chỉ có **một** nơi giữ toạ
độ: sửa một chỗ thì mọi skill theo, không sinh ra hai bộ số lệch nhau. Cỡ chữ và màu ghi bằng tên token
`slide.type.*` / `colors.*`, không chép giá trị.

File này nói **viết gì** trên từng slide.

## 1. Bìa

- Tên: `"Báo cáo tháng MM/YYYY"` — `slide.type.hero`, `colors.primary`, canh giữa.
- Dòng phụ: `"<đơn vị> · <người trình bày> · <ngày>"` — `slide.type.subtitle`, `colors.muted`.

## 2. Tóm tắt cho người quyết định

Một hộp chữ, sáu dòng, mỗi dòng một phần, mỗi dòng ≤ 12 từ:

```text
Mục tiêu: <kỳ này nhắm cái gì>
Bối cảnh: <thị trường/nguồn lực đang thế nào>
Vấn đề: <điều đang chệch>
Cách giải quyết: <làm gì>
Lợi ích: <được gì, kèm số>
Việc cần làm: <xin gì>
```

Mỗi dòng đi thẳng vào số, không mở bài. Sáu dòng ở `slide.type.body` cần ~141pt — vừa vùng nội dung.

## 3. Bảng điểm chỉ tiêu

`values`: `["Chỉ tiêu", "Mục tiêu", "Thực tế", "% đạt", "Trạng thái"]` + tối đa 6 dòng dữ liệu.

- `% đạt` ghi dạng `"96%"`; `Trạng thái` ghi bằng **chữ** — `"Đạt"` / `"Theo dõi"` / `"Chậm"` — vì
  `wpp.addTable` **không tô được màu ô**, nên không thể dùng màu xanh/đỏ để báo trạng thái.
- Số trong bảng không được `checkLayout` soi: tự đếm ≤ 6 dòng.

## 4. Phân tích chênh lệch (1–2 slide)

- **Tiêu đề là kết luận của slide**, ví dụ `"Kênh trực tuyến bù được phần thiếu của kênh truyền thống"` —
  không phải `"Phân tích doanh thu"`.
- 3–4 dòng, mỗi dòng một bước: hiện tượng → con số chứng minh → hệ quả. Mỗi slide chỉ một nguyên nhân
  lớn; hai nguyên nhân ngang nhau thì hai slide.
- Chỉ tiêu lệch nhiều nhất phải có slide riêng — đó là chỗ người đọc muốn hỏi nhất.

## 5. Vấn đề và rủi ro

≤ 4 dòng, mỗi dòng: `"<vấn đề> → <cách xử lý> (<ai>, <khi nào>)"`. Vấn đề không kèm cách xử lý thì để
lại vào `wpp.setNotes`, đừng đưa lên slide.

## 6. Việc cần quyết

≤ 3 dòng, mỗi dòng: `"<quyết cái gì> — vì <lý do> — không quyết thì <hệ quả>"`. Đây là slide để người có
thẩm quyền nói "đồng ý" hoặc "không"; thiếu nó thì báo cáo chỉ là bản tin, không phải tài liệu điều hành.

## 7. Kế hoạch kỳ tới

≤ 4 dòng, mỗi dòng: `"<việc> — <người phụ trách> — <mốc>"`. Việc không có người hoặc mốc thì chưa phải
kế hoạch.

## 8. Phụ lục (tuỳ chọn)

Bảng chi tiết cho những chỉ tiêu không vừa slide 3. Để cuối bài vì người đọc tự nhảy tới khi cần.

## Ranh giới phải nhớ khi viết

- Một hộp chữ chỉ có **một** kiểu chữ: không in đậm riêng một từ hay một con số trong câu. Muốn nhấn thì
  tách hộp riêng hoặc đổi màu cho cả hộp — đừng hứa "in đậm số 12%".
- Không có lệnh đặt phông, chữ nghiêng, gạch đầu dòng, biểu đồ, tô nền.
