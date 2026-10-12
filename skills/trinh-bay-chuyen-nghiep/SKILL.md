---
name: trinh-bay-chuyen-nghiep
description: >-
  Quy tắc trình bày slide PowerPoint chuyên nghiệp: một thông điệp mỗi slide, lưới và lề, thang cỡ chữ,
  mật độ chữ, bảng khi cần so sánh, chi tiết dài đưa vào ghi chú. Dùng khi tạo mới hoặc làm lại bài
  thuyết trình, slide, "làm slide cho đẹp", "thiết kế lại slide". Không dùng khi chỉ sửa một chữ.
apps: [wpp]
---

# Trình bày slide chuyên nghiệp

Đọc `read_skill_file {name: "_design", path: "tokens.json"}` (khối `slide`, `colors`) và dùng đúng giá
trị ở đó. Mọi con số dưới đây đo trên LibreOffice thật ngày 13/10/2026 — cách đo lại ở
`references/co-so-do-duoc.md`.

## 1. Lấy kích thước slide trước khi vẽ

Gọi `wpp.checkLayout` để lấy `slideWidth` × `slideHeight` rồi tính toạ độ từ đó. **Đừng đoán**: bản
LibreOffice mặc định là **793.7 × 446.5pt** (28 × 15.75cm), không phải 960×540 — đó là mặc định của
PowerPoint, và slide LibreOffice nhỏ hơn 0.83 lần nên cùng cỡ chữ trông to hơn.

Với `W` = `slideWidth`, `H` = `slideHeight`, lề `margin` = 40, rãnh `gutter` = 20:

| Vùng | left | top | width | height |
|---|---|---|---|---|
| Tiêu đề slide | 40 | 30 | W − 80 | 80 |
| Nội dung | 40 | 130 | W − 80 | H − 170 |
| Hai cột | 40 và W/2 + 10 | 130 | W/2 − 50 | H − 170 |
| Nguồn (chú thích) | 40 | H − 36 | W − 80 | 24 |

Dựng bằng `wpp.addSlide {layout: 12}` (trống) rồi `wpp.addText` theo đúng toạ độ trên. **Giữ nguyên
lưới này cho mọi slide** — dịch vị trí giữa các slide là thứ trông nghiệp dư nhất.

## 2. Cỡ chữ và sức chứa thật

Cỡ chữ lấy theo token: tiêu đề `slide.type.title`, nội dung `slide.type.body`, chú thích
`slide.type.caption`, chữ bìa `slide.type.hero`, số lớn `slide.type.figure`; **không dưới
`slide.type.minimum`** (cũng là ngưỡng `small-font` của `checkLayout`). Giá trị hiện tại: 28 / 20 / 14 /
40 / 40 / 12 — đọc lại `tokens.json` mỗi lần, đừng nhớ theo bài này.

Đo trên LibreOffice: mỗi dòng cao **1.12 × cỡ chữ**, hộp chữ có **7.1pt lề trong**, mỗi ký tự chiếm
khoảng **0.55 × cỡ chữ** chiều ngang. Thành ra:

- Hộp cần cao: `7.1 + số_dòng × 1.12 × cỡ_chữ`.
- Ký tự vừa một dòng: `width / (0.55 × cỡ_chữ)`.

Với lưới ở mục 1 và cỡ chữ hiện tại (tiêu đề 28, nội dung 20): tiêu đề vừa **2 dòng** (≈ 92 ký tự ≈ 15
từ) và dòng thứ ba tràn; nội dung vừa **12 dòng** (≈ 5 gạch đầu dòng × 2 dòng). Tính trước khi viết chữ,
đừng để `checkLayout` phải bắt.
Cả slide nên dưới **600 ký tự** — `checkLayout` báo `dense` khi quá.

Nhiều nhất **ba cỡ chữ trên một slide** (tiêu đề, nội dung, chú thích). Cỡ thứ tư không làm slide phong
phú hơn, chỉ làm nó bớt rõ. `slide.type.minimum` là **sàn kỹ thuật**, không phải cỡ để dùng: nó chỉ bảo
đảm `checkLayout` không báo `small-font`. **Sàn trợ năng cao hơn hẳn**: hướng dẫn trợ năng của Microsoft
cho PowerPoint nói chữ thường nên từ **18pt** trở lên — nên **chữ để đọc** (tiêu đề, nội dung) cứ dùng
`slide.type.title` / `slide.type.body`, đừng hạ cỡ cho vừa chữ; muốn vừa thì cắt chữ. Chỉ **dòng phụ**
(chú thích nguồn ở `slide.type.caption`) mới được dưới 18pt, và khi đó nó là **chữ thường** theo WCAG nên
phải đủ **4.5:1**: dùng `colors.text` hoặc `colors.muted`, tuyệt đối không `colors.warning`, và không
`colors.accent` / `colors.success` trên nền xám. Biết phòng chiếu rộng bao nhiêu thì tra bảng khoảng cách
người xem ở `references/cau-chuyen.md` mục 8; không biết thì chọn cỡ lớn.

## 3. Quy tắc nội dung

1. **Một thông điệp mỗi slide.** Tiêu đề là **câu kết luận** ("Doanh thu tăng 12% nhờ kênh trực
   tuyến"), không phải nhãn ("Doanh thu"). Câu chữ thường, ≤ 15 từ, ≤ 2 dòng — trần này có nguồn
   (Slideworks/McKinsey) và cũng đúng bằng trần kỹ thuật của lưới; PresentationLoad còn chặt hơn, 6–10 từ
   (nguồn ở `references/cau-chuyen.md` mục 3). Đọc liền các tiêu đề phải thành một mạch lập luận.
2. Tối đa `slide.maxBulletsPerSlide` gạch đầu dòng, mỗi dòng ≤ `slide.maxWordsPerBullet` từ; **nhắm 4**
   — trần là để chặn, không phải để chạm. Diễn giải dài → `wpp.setNotes`. Gạch đầu dòng là ký tự `"• "`
   thật, nối các dòng bằng `\n` trong `text` — wpp không có API bullet.
3. Số liệu so sánh → `wpp.addTable` (≤ 6 dòng dữ liệu); hoặc 3 con số lớn cạnh nhau. Đừng kể số trong
   đoạn văn.
4. Màu: **một** màu nhấn. Tiêu đề `colors.primary`, số liệu quan trọng `colors.accent`, chữ thường
   `colors.text`, phụ chú `colors.muted`, nền để trắng. Nhấn mọi thứ thì không còn gì nổi bật. Ba giới
   hạn tính được từ `tokens.json` (bảng đầy đủ ở `references/co-so-do-duoc.md`): `colors.warning` không
   đủ tương phản cho chữ thường nên chỉ dùng ở cỡ lớn hoặc in đậm; `colors.accent` và `colors.success`
   mất chuẩn AA khi đặt trên `colors.surface`; và **màu không bao giờ là dấu hiệu duy nhất** — "Đạt" /
   "Chậm" phải có chữ, vì người mù màu và bản in đen trắng không thấy màu.
5. Tiếng Việt: không viết hoa cả câu, không xuống dưới `slide.type.minimum`; dấu thanh xếp tầng nên chữ
   nhỏ khó đọc hơn tiếng Anh cùng cỡ.
6. Slide có số liệu thì ghi nguồn ở dòng chú thích cuối slide (dòng "Nguồn" trong lưới).

## 4. Giới hạn thật của wpp — đừng hứa rồi kẹt

- `wpp.addText` chỉ nhận `text, slide, left, top, width, height, fontSize, bold, color, align`: **không
  có** phông chữ, chữ nghiêng, gạch đầu dòng, giãn dòng. Cỡ chữ, đậm, màu áp cho **cả hộp** — không in
  đậm riêng một từ hay một con số trong câu. Muốn nhấn một số thì tách nó thành hộp riêng.
- **Không có** lệnh biểu đồ, hình khối, sơ đồ, tô nền; `wpp.addTable` không nhận cỡ chữ, style hay màu ô
  (ô mặc định 18pt). Muốn "biểu đồ" thì dùng bảng hoặc 3 con số lớn.
- **`checkLayout` không thấy chữ trong bảng** (ô bảng không phải text shape): bảng tràn thì nó im lặng.
  Giữ bảng ≤ 6 dòng và phải nhìn bằng mắt (hoặc `look_at_document` nếu có).
- Layout của wpp nhỏ hơn vẻ ngoài: **1 và 11 là cùng một kiểu** (`AUTOLAYOUT_TITLE`). Đo cả bốn layout
  1/2/11/12: mỗi layout cho đúng **hai hộp rỗng**, không hộp nào có chữ, và **không lệnh nào ghi được vào
  chúng** (mọi `wpp.addText` đều tạo hộp chữ mới của riêng nó). Nên luôn `layout: 12` + hộp chữ tự đặt
  theo lưới ở mục 1 — đừng trông vào placeholder.

## 5. Checklist

- [ ] `wpp.listSlides` — bài đang có gì (thêm vào cuối, không xoá slide cũ trừ khi được yêu cầu).
- [ ] `wpp.checkLayout` lấy `slideWidth`/`slideHeight`; tính lưới ở mục 1.
- [ ] Lập dàn ý: mỗi slide một câu kết luận; đọc các tiêu đề thành mạch.
- [ ] Dựng slide theo lưới; tính sức chứa trước khi viết (mục 2).
- [ ] Chi tiết/diễn giải → `wpp.setNotes`.
- [ ] **Soát**: gọi `wpp.checkLayout` **một lần** (mặc định đã kiểm mọi slide). Sửa mọi `overflow` (rút
      chữ, hạ `fontSize` nhưng ≥ 12, hoặc tăng `height`), `offslide`, `overlap`, `small-font`, `dense` →
      đến khi `issueCount` = 0. Từ 13/10/2026 gọi lại cho kết quả y như lần đầu, nên soát → sửa → soát
      lại được.
- [ ] Nếu có tool `look_at_document` (người dùng bật QA thị giác): gọi **một lần** sau bước soát để nhìn
      slide (tràn chữ trong bảng, lệch lưới, tương phản) rồi sửa điểm còn lỗi.

## Đọc thêm

- `references/co-so-do-duoc.md` — số đo, cách đo lại, sức chứa từng cỡ chữ.
- `references/cau-chuyen.md` — dàn ý bài thuyết trình: tóm tắt cho người quyết định, tiêu đề hành động,
  độ dài, nguồn tham khảo và những điểm **chưa** kiểm chứng được.
- `references/bo-cuc-slide.md` — mẫu bố cục dùng lại: bìa, chia mục, 3 số, bảng, hai cột, slide chốt.
