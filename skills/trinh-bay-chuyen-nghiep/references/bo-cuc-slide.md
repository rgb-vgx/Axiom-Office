# Mẫu bố cục dùng lại

Toạ độ ở đây suy từ **một** lưới trong `SKILL.md`, không phải bộ số riêng. Mỗi hàng ghi công thức theo
`W` = `slideWidth`, `H` = `slideHeight` (lề `slide.margin`, rãnh `slide.gutter`), kèm giá trị tại cỡ slide
LibreOffice mặc định **W = 793.7, H = 446.5**. Slide khác cỡ thì tính lại theo công thức, **đừng bê số
của cột "giá trị mặc định"**.

Cỡ chữ lấy theo token `slide.type.*`: `slide.type.hero` (bìa), `slide.type.title`, `slide.type.subtitle`,
`slide.type.figure` (số lớn), `slide.type.body`, `slide.type.caption`. Màu: `colors.primary` cho tiêu đề,
`colors.accent` cho số liệu, `colors.text` cho chữ thường, `colors.muted` cho phụ chú.

## 1. Bìa

| Phần | left | top | width | height | align |
|---|---|---|---|---|---|
| Tên báo cáo (hero, bold, primary) | 40 | H×0.34 ≈ **150** | W − 80 (**713.7**) | 100 | center |
| Đơn vị · người trình bày · ngày (subtitle, muted) | 40 | 262 | W − 80 | 70 | center |

Hộp 100pt chứa **2 dòng** ở 40pt (mỗi dòng 44.8pt) và dòng thứ ba tràn; dòng phụ 70pt chứa 2 dòng ở
24pt. Tên báo cáo dài hơn ~64 ký tự thì hạ xuống `slide.type.title`.

## 2. Chia mục (section divider)

Một dòng chữ lớn giữa slide, không gạch đầu dòng: `left: 40, top: 150, width: W − 80, height: 100,
bold: true, align: center`, `fontSize` lấy `slide.type.hero`, `color` lấy `colors.primary`.

**wpp không có lệnh tô nền hay vẽ khối**, nên không làm được dải màu hay hộp nền cho slide chia mục —
khoảng trắng là thứ duy nhất tách mục. Đừng hứa với người dùng là có "nền màu".

## 3. Ba con số lớn

Cột rộng `(W − 80 − 2×20) / 3` = **224.6 → dùng 224**; ba cột đặt tại `left` = 40 / **284** / **528**
(cột thứ ba kết thúc ở 752, còn lề phải 41.7 — xấp xỉ lề trái, đủ đều mắt).

| Phần | left | top | width | height | cỡ & màu |
|---|---|---|---|---|---|
| Con số | 40 / 284 / 528 | 150 | 224 | 60 | `slide.type.figure`, bold, `colors.accent`, center |
| Nhãn dưới số | như trên | 220 | 224 | 60 | `slide.type.body`, `colors.text`, center |

Sức chứa: ở 40pt cột 224 chỉ vừa **~10 ký tự một dòng** — viết gọn bằng đơn vị ("1,25 tỷ", không phải
"1.250.000.000"), hộp 60pt **không** cho xuống dòng. Nhãn ở 20pt cột 224 được **2 dòng ≈ 40 ký tự** (≈ 4
từ); dài hơn thì rút hoặc bỏ bớt một con số.

## 4. Bảng so sánh

`wpp.addTable {values: [["Chỉ tiêu","Kỳ này","Kỳ trước","Thay đổi"], ...], left: 40, top: 130,
width: W − 80 (713.7), height: H − 170 (276.5)}` + dòng nguồn ở hộp "Nguồn" của lưới.

- Tối đa **6 dòng dữ liệu**; cột "Thay đổi" ghi "+12%" / "−5%" (dấu trừ là ký tự `−`, không phải gạch nối).
- `wpp.addTable` **không nhận cỡ chữ**: ô luôn ở cỡ mặc định (đo được 18pt). Đừng hứa chữ trong bảng nhỏ
  hơn để nhét thêm dòng.
- **`checkLayout` không thấy chữ trong bảng** — bảng tràn thì không có cảnh báo nào. Giữ ≤ 6 dòng và
  nhìn bằng mắt (hoặc `look_at_document`).

## 5. Hai cột

| Cột | left | top | width | height |
|---|---|---|---|---|
| Trái | 40 | 130 | W/2 − 50 (**346.85**) | H − 170 |
| Phải | W/2 + 10 (**406.85**) | 130 | W/2 − 50 (**346.85**) | H − 170 |

Khe giữa hai cột đúng bằng `slide.gutter`. Dùng cho: chữ ↔ bảng, chữ ↔ ảnh, "hiện tại" ↔ "đề xuất".
Sức chứa một cột ở 20pt: **~31 ký tự một dòng, 12 dòng**.

## 6. Ảnh

`wpp.addImage {path, slide, left, top, width}` — **chỉ truyền `width`**, để LibreOffice tự giữ tỉ lệ
(truyền cả `width` lẫn `height` là kéo méo ảnh). Chiều cao suy ra = `width / tỉ_lệ_ảnh`, và phải ≤
**276.5** (chiều cao vùng nội dung):

| Tỉ lệ ảnh | width tối đa |
|---|---|
| 16:9 | 491 |
| 3:2 | 414 |
| 4:3 | 368 |
| 1:1 | 276 |

Căn giữa trong vùng nội dung: `left = (W − width) / 2`, `top = 130 + (276.5 − height) / 2`. Ảnh rộng
quá giới hạn sẽ đẩy `bottom` vượt `H` → `checkLayout` báo `offslide`; sửa bằng cách giảm `width`. Nếu
người dùng đưa ảnh chụp màn hình (tỉ lệ lạ), truyền `width` = 491 rồi đọc lại `checkLayout`.
