# Mẫu bố cục dùng lại (PowerPoint)

Toạ độ ở đây suy từ **một** lưới trong `SKILL.md`, không phải bộ số riêng. Mỗi hàng ghi công thức theo
`W` = `slideWidth`, `H` = `slideHeight` (lề `slide.margin`, rãnh `slide.gutter`), kèm giá trị ở cỡ slide
PowerPoint mặc định **W = 960, H = 540**. Slide khác cỡ thì tính lại theo công thức, **đừng bê số trong
ngoặc**.

Cỡ chữ lấy theo token `slide.type.*`: `slide.type.hero` (bìa), `slide.type.title`, `slide.type.subtitle`,
`slide.type.figure` (số lớn), `slide.type.body`, `slide.type.caption`. Màu: `colors.primary` cho tiêu đề,
`colors.accent` cho số liệu, `colors.text` cho chữ thường, `colors.muted` cho phụ chú.

Sức chứa tính bằng hai công thức đo trên PowerPoint (`co-so-do-duoc.md` mục 3): hộp n dòng cần cao
`7.2 + n × 1.21 × cỡ`, một dòng chứa ≈ `width / (0.53 × cỡ)` ký tự. Hộp chữ giữ chiều cao đã đặt; chữ dài
hơn thì `checkLayout` báo `overflow`. **Bảng thì khác**: hàng tự giãn theo chữ (mục 4).

## 1. Bìa

| Phần | left | top | width | height | align |
|---|---|---|---|---|---|
| Tên báo cáo (hero, bold, primary) | 40 | H × 0.34 (**184**) | W − 80 (**880**) | 110 | center |
| Đơn vị · người trình bày · ngày (subtitle, muted) | 40 | H × 0.34 + 122 (**306**) | W − 80 | 70 | center |

Tên báo cáo ở 40pt: 2 dòng cao 104.1, mỗi dòng ≈ 41 ký tự → **≤ 80 ký tự**. Dòng phụ ở 24pt: 2 dòng
cao 65.4, ≈ 69 ký tự mỗi dòng.

## 2. Chia mục (section divider)

Một dòng chữ lớn giữa slide, không gạch đầu dòng: `left: 40, top: (H − 110) / 2 (215), width: W − 80,
height: 110, bold: true, align: center`, `fontSize` lấy `slide.type.hero`, `color` lấy `colors.primary`.
Tên mục ≤ 40 ký tự để nằm trên một dòng.

**wpp không có lệnh tô nền hay vẽ khối**, nên không làm được dải màu hay hộp nền cho slide chia mục —
khoảng trắng là thứ duy nhất tách mục. Đừng hứa với người dùng là có "nền màu".

## 3. Ba con số lớn

Cột rộng `(W − 80 − 2 × 20) / 3` (**280**); ba cột đặt tại `left` = 40 / 40 + 300 / 40 + 600
(**40 / 340 / 640**), cột thứ ba kết thúc đúng ở `W − 40`.

| Phần | left | top | width | height | cỡ & màu |
|---|---|---|---|---|---|
| Con số | 40 / 340 / 640 | H/2 − 20 (**250**) | 280 | 60 | `slide.type.figure`, bold, `colors.accent`, center |
| Nhãn dưới số | như trên | H/2 + 45 (**315**) | 280 | 60 | `slide.type.body`, `colors.text`, center |

Tiêu đề slide vẫn ở hộp "Tiêu đề" của lưới. Đây là **slide duy nhất được bốn cỡ chữ** (tiêu đề, số, nhãn,
nguồn) — SKILL.md mục 2 ghi rõ ngoại lệ này; các mẫu khác giữ ba cỡ.

Sức chứa: con số ở 40pt cột 280 chỉ vừa **≈ 12 ký tự** — viết gọn bằng đơn vị ("1,25 tỷ", không phải
"1.250.000.000"). Nhãn ở 20pt được **2 dòng ≈ 50 ký tự** (≈ 8 từ); dài hơn thì `overflow` — rút chữ
hoặc bỏ bớt một con số.

## 4. Bảng so sánh

`wpp.addTable {values: [["Chỉ tiêu","Kỳ này","Kỳ trước","Thay đổi"], ...], left: 40, top: 130,
width: W − 80 (880), height: H − 170 (370)}` + dòng nguồn ở hộp "Nguồn" của lưới.

- Tối đa **6 dòng dữ liệu** + 1 hàng tiêu đề; cột "Thay đổi" ghi "+12%" / "−5%" (dấu trừ là ký tự `−`,
  không phải gạch nối).
- Bảng lấy **style mặc định của theme PowerPoint** (hàng đầu nền xanh chữ trắng đậm, các hàng tô sọc) —
  không đổi được sang `colors.primary`, và cũng không cần: nhìn đã gọn. Đừng hứa với người dùng "bảng màu
  theo nhận diện".
- Ô luôn **Arial 18pt** (`wpp.addTable` không nhận cỡ chữ). Các hàng chia đều chiều cao bảng: 7 hàng →
  mỗi hàng 52.9, đủ cho **2 dòng** chữ trong ô. Chữ dài hơn thì hàng giãn ra và cả bảng dài xuống.
- Chữ **một dòng** trong một ô, theo số cột của bảng rộng 880:

  | Số cột | 2 | 3 | 4 | 5 |
  |---|---|---|---|---|
  | Ký tự một dòng | 36 | 25 | 18 | 14 |

  Đo được: ô 69 ký tự ở bảng 4 cột thành **4 dòng** (ngắt từ chỉ còn ≈ 18 ký tự mỗi dòng), bảng 7 hàng
  giãn từ 370 lên **410.7**.
- **Luôn đặt hộp "Nguồn"** khi có bảng: bảng giãn sẽ đè lên nó và `checkLayout` báo `overlap`. Không có
  hộp nguồn thì bảng phải vượt mép slide quá 2pt mới bị báo `offslide`.

## 5. Hai cột

| Cột | left | top | width | height |
|---|---|---|---|---|
| Trái | 40 | 130 | W/2 − 50 (**430**) | H − 170 (**370**) |
| Phải | W/2 + 10 (**490**) | 130 | W/2 − 50 (**430**) | H − 170 |

Khe giữa hai cột đúng bằng `slide.gutter`. Dùng cho: chữ ↔ bảng, chữ ↔ ảnh, "hiện tại" ↔ "đề xuất".
Sức chứa một cột ở 20pt: **≈ 43 ký tự một dòng**, viết **10 dòng** (trần 14).

## 6. Ảnh

`wpp.addImage {path, slide, left, top, width}` — **chỉ truyền `width`**, ảnh tự giữ tỉ lệ (truyền cả
`width` lẫn `height` là kéo méo ảnh). Chiều cao suy ra = `width / tỉ_lệ_ảnh`, và nên ≤ **H − 190 (350)** —
vùng nội dung cao 370, nhưng ảnh cao hết 370 thì dính sát dòng nguồn trông chật (đã dựng thử):

| Tỉ lệ ảnh | width tối đa |
|---|---|
| 16:9 | 620 |
| 3:2 | 525 |
| 4:3 | 465 |
| 1:1 | 350 |

Căn giữa trong vùng nội dung: `left = (W − width) / 2`, `top = 130 + (370 − height) / 2`. Ảnh rộng quá
giới hạn sẽ đẩy đáy xuống đè hộp "Nguồn" (`overlap`) hoặc vượt `H` (`offslide`); sửa bằng cách giảm
`width`. Ảnh chụp màn hình (tỉ lệ lạ): truyền `width` = 400 rồi đọc lại `checkLayout`.

## 7. Chọn mẫu theo loại nội dung

Chọn mẫu **theo số ý thật**, không theo ý thích: ba con số chỉ khi có đúng 3 con số; hai cột chỉ khi có
đúng 2 vế. Nhiều ý hơn sức chứa → tách thành nhiều slide cùng mẫu, tiêu đề slide sau thêm "(tiếp theo)",
đánh số ý liên tục. Ít ý hơn → dùng mẫu ít ô hơn, không để ô trống.

| Nội dung | Mẫu |
|---|---|
| Trang đầu | Bìa (mục 1) |
| Bố cục bài, mở đầu từng phần | Chia mục (mục 2) — tên phần, không gạch đầu dòng |
| Mục đích, yêu cầu, nhiệm vụ, giải pháp (≤ 5 ý) | Gạch đầu dòng ở hộp "Nội dung" (lưới `SKILL.md` mục 1) |
| Kết quả nổi bật, đúng 3 chỉ tiêu | Ba con số (mục 3) |
| Chỉ tiêu – kế hoạch – thực hiện; so sánh cũ – mới; căn cứ pháp lý (số hiệu · ngày · trích yếu) | Bảng (mục 4) |
| Phân công (đơn vị · nhiệm vụ · thời hạn); lộ trình theo mốc; chương trình hội nghị | Bảng (mục 4), mỗi mốc/đơn vị một hàng |
| Kết quả – hạn chế; thuận lợi – khó khăn; hiện tại – đề xuất | Hai cột (mục 5) |
| Ảnh hoạt động, hiện trường kèm 2–3 ý | Hai cột: chữ ↔ ảnh (mục 5, 6) |
| Quy trình, thủ tục nhiều bước | Gạch đầu dòng đánh số "1. … 2. …"; quá 5 bước thì tách slide |

Đừng dùng **một mẫu cho cả bài**: hơn ba slide gạch đầu dòng liền nhau thì xem có ý nào thành bảng,
hai cột hay ba con số được không. Nhưng chỉ đổi khi nội dung thật sự hợp — đừng ép.
