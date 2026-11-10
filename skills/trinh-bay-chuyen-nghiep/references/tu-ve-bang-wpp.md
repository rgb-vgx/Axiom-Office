# Tự vẽ slide bằng `wpp.addText` (không có `slides_from_brief`)

Chỉ đọc file này khi **không** có tool `slides_from_brief`, hoặc khi sửa lẻ một hộp chữ có sẵn. Có tool đó thì
tool tự đặt lưới, cỡ chữ, màu và tự soát — không cần gì ở đây.

Đọc `read_skill_file {name: "_design", path: "tokens.json"}` (khối `slide`, `colors`) và dùng đúng giá
trị ở đó. Cỡ chữ và màu ở đây ghi bằng **tên token**; số trong ngoặc là giá trị hiện tại để dễ
hình dung — `tokens.json` mới là chuẩn. Mọi số đo là của PowerPoint thật (`references/co-so-do-duoc.md`).

## 1. Lưới — dùng cho mọi slide

Gọi `wpp.checkLayout` lấy `slideWidth` = `W`, `slideHeight` = `H`. PowerPoint mặc định **960 × 540**;
bài mở từ file cũ có thể là 720 × 540 (4:3) hoặc cỡ khác — luôn tính theo `W`, `H`.

| Vùng | left | top | width | height | ở 960 × 540 |
|---|---|---|---|---|---|
| Tiêu đề | 40 | 30 | W − 80 | 80 | 40, 30, 880, 80 |
| Nội dung | 40 | 130 | W − 80 | H − 170 | 40, 130, 880, 370 |
| Hai cột | 40 và W/2 + 10 | 130 | W/2 − 50 | H − 170 | 40 / 490, 130, 430, 370 |
| Nguồn | 40 | H − 36 | W − 80 | 24 | 40, 504, 880, 24 |

Dựng slide bằng `wpp.addSlide {layout: 12}` (trống) rồi `wpp.addText` đúng toạ độ trên. **Mọi slide cùng
một lưới** — tiêu đề nhảy chỗ giữa các slide là lỗi trông nghiệp dư nhất. Mẫu bìa, chia mục, ba con số,
bảng, hai cột, ảnh: `references/bo-cuc-slide.md`.

## 2. Cỡ chữ — cố định, không hạ để nhét chữ

| Chữ | Token | Màu |
|---|---|---|
| Tiêu đề slide | `slide.type.title` (28), bold | `colors.primary` |
| Nội dung | `slide.type.body` (20) | `colors.text` |
| Dòng nguồn | `slide.type.caption` (14) | `colors.muted` |
| Tên bìa / chia mục | `slide.type.hero` (40), bold | `colors.primary` |
| Số lớn | `slide.type.figure` (40), bold | `colors.accent` |

- **Tiêu đề và nội dung không bao giờ hạ cỡ.** Chữ không vừa thì **cắt chữ**, không thu chữ. Chữ để đọc
  phải từ 18pt trở lên (hướng dẫn trợ năng của Microsoft); `slide.type.minimum` (12) chỉ là ngưỡng máy báo
  `small-font`, không phải cỡ được phép dùng.
- Nhiều nhất **ba cỡ chữ** trên một slide: tiêu đề, nội dung, nguồn. Ngoại lệ duy nhất là slide ba con số
  (thêm `slide.type.figure`).

## 3. Sức chứa — đếm trước khi viết

Đo trên PowerPoint ở lưới mục 1 (slide 960 × 540, phông Arial mặc định):

| Hộp | Trần đo được | Viết |
|---|---|---|
| Tiêu đề, 28pt | 2 dòng ≈ 130 ký tự | **≤ 80 ký tự**, 6–15 từ, một câu |
| Nội dung, 20pt | 14 dòng, ≈ 89 ký tự/dòng | **10 dòng** = 4–5 gạch đầu dòng, mỗi gạch ≤ 2 dòng |
| Một cột của hai cột, 20pt | 14 dòng, ≈ 43 ký tự/dòng | **10 dòng** |
| Nguồn, 14pt | 1 dòng ≈ 130 ký tự | một dòng, **≤ 80 ký tự** |
| Cả slide | 600 ký tự (`checkLayout` báo `dense`) | ≈ 450 ký tự |

Hộp khác cỡ: ký tự một dòng ≈ `width / (0.53 × cỡ_chữ)`; số dòng vừa = `(height − 7.2) / (1.21 × cỡ_chữ)`,
lấy phần nguyên. Chừa một dòng dư.

Hộp của `wpp.addText` **giữ đúng chiều cao đã đặt**: chữ dài hơn hộp thì tràn ra ngoài khung và
`checkLayout` báo `overflow`. Đếm ký tự trước khi viết vẫn rẻ hơn viết rồi sửa.

**Chữ không vừa thì làm theo thứ tự**: (1) viết ngắn lại; (2) chuyển diễn giải sang `wpp.setNotes`;
(3) tách thành hai slide, mỗi slide một thông điệp — slide sau giữ tiêu đề, thêm "(tiếp theo)", đánh số
ý nối tiếp. Không hạ `fontSize`, không dời hộp ra khỏi lưới. Người dùng đã nói số slide (SKILL.md mục 1.0) thì
**không** dùng bước (3): rút gọn và chuyển diễn giải vào ghi chú.

## 4. Giới hạn thật của wpp — đừng hứa rồi kẹt

- `wpp.addText` chỉ nhận `text, slide, left, top, width, height, fontSize, bold, color, align`. **Không
  có** phông chữ (luôn là Arial), chữ nghiêng, gạch đầu dòng, giãn dòng. Cỡ, đậm, màu áp cho **cả hộp** —
  muốn nhấn một con số thì tách nó thành hộp riêng.
- Biểu đồ, hình khối, mũi tên, sơ đồ chỉ làm được qua `slides_from_brief` (`chart`, `process`,
  `timeline`, `diagram`). Không có tool đó thì **không có** biểu đồ và hình khối: người dùng xin biểu đồ →
  làm bảng hoặc ba con số lớn và **nói thẳng** là không vẽ được biểu đồ. Không vẽ cột bằng hình chữ nhật
  hay ký tự. Không có tô nền.
- `wpp.addTable` không nhận cỡ chữ, style hay màu ô: ô luôn Arial 18pt. Chữ dài trong ô làm **hàng giãn
  ra** và cả bảng dài xuống — giữ chữ mỗi ô theo bảng ở `references/bo-cuc-slide.md` mục 4.
- `wpp.addImage`: chỉ truyền `width` (ảnh tự giữ tỉ lệ). Truyền cả `width` lẫn `height` là kéo méo ảnh.
- Luôn `layout: 12`. Layout 1, 2, 11 tạo placeholder ("Bấm để thêm tiêu đề") mà `wpp.addText` **không
  ghi vào được** — chữ thật nằm ở hộp mới, placeholder rỗng vẫn hiện khi soạn.

## 5. Checklist

- [ ] `wpp.listSlides` — bài đang có gì. Thêm vào cuối; không xoá slide cũ trừ khi được yêu cầu.
- [ ] `wpp.checkLayout` lấy `W`, `H`; tính lưới mục 1.
- [ ] Dàn ý: đúng số slide người dùng xin (SKILL.md mục 1.0); mỗi slide một câu kết luận; đọc các tiêu đề thấy
      thành mạch.
- [ ] Dựng slide theo lưới, cỡ chữ mục 2, chữ trong cột **Viết** mục 3 (tiêu đề ≤ 80 ký tự). Diễn giải
      → `wpp.setNotes`.
- [ ] **Soát**: gọi `wpp.checkLayout` (mặc định kiểm mọi slide). Sửa đến khi `issueCount` = 0:
  - `overflow` → chữ dài hơn hộp: làm theo thứ tự ở cuối mục 3. **Không hạ `fontSize`.**
  - `overlap` giữa bảng/ảnh và hộp "Nguồn" → bảng giãn hoặc ảnh quá to: rút chữ trong ô, bớt hàng, giảm
    `width` ảnh. `overlap` giữa hai hộp chữ → đưa hộp về đúng toạ độ lưới.
  - `offslide` → hộp, bảng hoặc ảnh vượt mép slide: như trên.
  - `dense` → quá 600 ký tự: chuyển bớt sang `wpp.setNotes` hoặc tách slide.
  - `small-font` → trả cỡ chữ về token ở mục 2.
- [ ] Nếu có tool `look_at_document`: gọi **một lần** sau khi soát xong để nhìn slide (lệch lưới, màu,
      bảng), sửa chỗ còn lỗi.
- [ ] Đối chiếu với nguồn: không sót ý nào, thứ tự giữ theo mạch bản gốc, số liệu đều có trong nguồn.
