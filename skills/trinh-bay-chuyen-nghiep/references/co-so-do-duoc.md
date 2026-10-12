# Cơ sở đo được của lưới slide

Toàn bộ con số trong `SKILL.md` không phải ước lượng cho đẹp: chúng đo trên **LibreOffice thật** ngày
**13/10/2026** (LibreOffice Impress mặc định, profile tạm, `xvfb-run` có màn hình ảo — chạy `--headless`
thì số đo chiều cao chữ sai). Đo lại:

```bash
xvfb-run -a python3 -I tests/lo/measure_slide_grid.py
```

Script dựng đúng lưới trong `SKILL.md` lên slide thật rồi in ra từng hộp: cần bao nhiêu, có bao nhiêu,
và `checkLayout` nói gì. Nó cũng kiểm giới hạn layout ở mục 4 của `SKILL.md` (cả bốn layout đều cho đúng
hai hộp rỗng, và 1 với 11 là cùng một kiểu). Nếu LibreOffice đổi cách dàn chữ, script đỏ trước khi người
dùng thấy slide xấu.

## 1. Kích thước slide

| Đo | Giá trị |
|---|---|
| Slide mặc định | **793.7 × 446.5pt** = 28 × 15.75cm, tỉ lệ 1.7778 (16:9) |
| Mặc định PowerPoint (tham chiếu) | 960 × 540pt = 33.87 × 19.05cm |

Hai bên đều 16:9 nhưng **không cùng cỡ**: slide LibreOffice nhỏ hơn **0.827 lần** theo chiều dài. Vì
vậy một cỡ chữ "chuẩn" của tài liệu PowerPoint (tiêu đề 36–44pt) khi bê nguyên sang đây sẽ **quá to**;
quy đổi tương đương là 36 × 0.827 ≈ **30pt**. Đó là lý do `slide.type.title` = **28** chứ không phải 36, và
lý do lưới cũ (`title` 36pt trong hộp cao 60pt) tự sinh `overflow`: một tiêu đề hành động 2 dòng ở 36pt
cần `7.1 + 2 × 1.12 × 36` = **87.7pt**, gấp rưỡi chiều cao hộp.

Chọn 28 chứ không phải 30 (30 vẫn vừa hộp 80pt: cần 74.3pt) vì **bề ngang ký tự có sai số ±20%** — xem
mục 2. Tiêu đề 2 dòng ở 28pt cần 69.8pt, tức còn **10pt dư**; ở 30pt chỉ còn 5.7pt, và đó là khoản dư duy
nhất bảo vệ tiêu đề khỏi tràn khi câu thật dài hơn ước lượng.

## 2. Ba hằng số dàn chữ

Đo bằng cách dựng hộp chữ rồi ép `Size.Height` về 1/100mm với `TextAutoGrowHeight = True`, đọc lại
chiều cao LibreOffice tự tính:

| Hằng số | Giá trị đo | Ghi chú |
|---|---|---|
| Chiều cao một dòng | **1.12 × cỡ chữ** | đo ở 18 / 20 / 28pt: 1.120 / 1.115 / 1.115 |
| Lề trong của hộp chữ (trên + dưới) | **7.1pt** | không đổi theo cỡ chữ |
| Bề ngang một ký tự | **≈ 0.55 × cỡ chữ** | đo được 0.48–0.64 tuỳ chữ — xem sai số bên dưới |

Suy ra:

```text
chiều cao cần  = 7.1 + số_dòng × 1.12 × cỡ_chữ
ký tự một dòng = width / (0.55 × cỡ_chữ)
```

**Sai số phải nhớ**: bề ngang ký tự dao động mạnh. Đo bằng cách **dò nhị phân** chuỗi dài nhất còn vừa
một dòng (13/10/2026, hộp rộng 713.7pt): **75 ký tự** ở 20pt và **53 ký tự** ở 28pt với chữ thường viết
liền — tức 1 ký tự = **0.48 × cỡ chữ**. Nhưng một mẫu văn xuôi thật có dấu chỉ được **55.5 ký tự/dòng**
ở 20pt, tức 0.64 × cỡ chữ, vì chỗ ngắt từ ăn mất phần dư. Vậy 0.55 là ước lượng **ở giữa**, và sai số
thật là **±20%**, không phải ±15%.

Hệ quả: đừng thiết kế sát trần. Chừa **một dòng dư** ở hộp nội dung, và để `checkLayout` xác nhận —
`checkLayout` đo chiều cao chữ thật nên nó đúng hơn mọi công thức ở đây.

Phông chữ mặc định của bản LibreOffice này là **Liberation Sans**, hiển thị tiếng Việt đúng. wpp **không
có lệnh đặt phông**, nên hướng dẫn "dùng phông X" là vô nghĩa — chỉ cỡ chữ, đậm, màu, căn lề là điều
khiển được.

## 3. Sức chứa theo lưới chuẩn

Với `W` = 793.7 (mặc định): hộp nội dung rộng **713.7**, cột đôi rộng **346.85**.

| Cỡ chữ | Dòng cao | Ký tự/dòng (713.7) | Ký tự/dòng (cột 346.85) |
|---|---|---|---|
| 28 = `slide.type.title` | 31.4 | 46 | 22 |
| 20 = `slide.type.body` | 22.4 | 65 | 31 |
| 18 (ô bảng, cố định) | 20.2 | 72 | 35 |
| 14 = `slide.type.caption` | 15.7 | 93 | 45 |
| 40 = `slide.type.hero`, `slide.type.figure` | 44.8 | 32 | 16 |

Cột "Ký tự/dòng" tính bằng 0.55 nên là **ước lượng thận trọng** (đo được ở 28pt là 53 chứ không phải
46). Viết theo cột này thì không tràn; viết quá nó chưa chắc đã tràn, nhưng phải để `checkLayout` xác nhận.

Số dòng vừa hộp, tính từ công thức trên:

| Hộp | Cao | Ở 28pt | Ở 20pt | Ở 14pt |
|---|---|---|---|---|
| Tiêu đề (top 30) | 80 | **2** | — | — |
| Nội dung (top 130) | 276.5 | — | **12** | — |
| Nguồn (top H−36) | 24 | — | — | **1** |
| Bìa (hero 40) | 100 | — | — | — (2 dòng) |

**12 dòng là trần hình học, không phải mục tiêu**: `7.1 + 12 × 1.12 × 20` = 275.9pt, chỉ dư **0.6pt** trong
hộp 276.5pt (`checkLayout` cho qua vì `LAYOUT_TOLERANCE` 2pt). Viết tới 12 dòng thì mọi sai số ngắt dòng
đều thành `overflow` — nhắm **10 dòng** (5 gạch đầu dòng × 2 dòng), đúng như SKILL.md nói.

Đối chiếu với `checkLayout`: `dense` bắn khi **quá 600 ký tự** một slide. Hộp nội dung 20pt chứa được
tới ~780 ký tự, tức **trần thật là 600 ký tự**, không phải sức chứa hình học. Năm gạch đầu dòng 12 từ
(≈ 70 ký tự mỗi dòng) + tiêu đề + nguồn ≈ **470 ký tự** — vừa dưới trần, đúng bằng
`slide.maxBulletsPerSlide` × `slide.maxWordsPerBullet` trong `tokens.json`. Hai con số đó khớp nhau, không
phải đặt tuỳ ý. (Slide đầy nhất dựng thật trong `measure_slide_grid.py`, viết dài hơn mức này một chút, ra
**501 ký tự** — vẫn dưới 600.)

## 4. Tương phản màu — tính được, không phải cảm nhận

Tỉ lệ tương phản theo WCAG 2.1 = `(L_sáng + 0.05) / (L_tối + 0.05)`, trong đó
`L = 0.2126·R + 0.7152·G + 0.0722·B` với mỗi kênh đã giải gamma
(`c/12.92` nếu `c ≤ 0.03928`, ngược lại `((c+0.055)/1.055)^2.4`). Ngưỡng: **4.5:1** cho chữ thường,
**3:1** cho chữ lớn (≥ 18pt, hoặc ≥ 14pt nếu đậm) và cho đường viền/đồ thị có nghĩa.

| Token | Trên nền trắng | Trên `colors.surface` | Kết luận |
|---|---|---|---|
| `colors.text` | **15.13:1** | 13.52:1 | đạt AAA |
| `colors.primary` | **8.66:1** | 7.74:1 | đạt AAA — chữ trắng trên nền `primary` cũng 8.66:1 |
| `colors.muted` | **7.00:1** | 6.26:1 | AAA trên trắng (sát ngưỡng), tụt còn AA trên `surface` |
| `colors.danger` | **6.48:1** | 5.79:1 | đạt AA |
| `colors.accent` | **4.84:1** | 4.33:1 | AA trên trắng; trên `surface` **chỉ chữ lớn** |
| `colors.success` | **4.54:1** | 4.06:1 | AA trên trắng; trên `surface` **trượt AA** |
| `colors.warning` | **4.33:1** | 3.87:1 | **trượt AA cho chữ thường** — chỉ dùng ≥ 18pt hoặc in đậm |
| `colors.border` | 2.35:1 | 2.10:1 | không đủ 3:1 cho viền có nghĩa |

Ba hệ quả thực dụng:

- **`colors.warning` không được là chữ thường ở đâu cả** (4.33:1 trên trắng, 3.87:1 trên `surface` — cả
  hai đều dưới 4.5), và **`colors.accent` với `colors.success` không được là chữ thường trên
  `colors.surface`**. Cả ba vẫn dùng tốt ở cỡ lớn (số liệu, tiêu đề) — đúng chỗ ta cần chúng.
- **Màu không được là dấu hiệu duy nhất** (WCAG 1.4.1). Đỏ/xanh để báo "chậm/đạt" là vô nghĩa với người
  mù màu — khoảng **5–8% nam giới** tuỳ nhóm gốc (8% gốc Âu, 5% gốc Á, 4% gốc Phi; nguồn Okabe & Ito ở
  `cau-chuyen.md` mục 9) — và với bản in đen trắng. Luôn kèm một chữ: "Chậm", "Đạt". `wpp.addTable` cũng
  không tô được màu ô, nên quy tắc này khớp luôn với giới hạn công cụ.
- **Điều kiện chiếu không kiểm soát được**, nên chọn màu **dư** tương phản thay vì vừa khít ngưỡng. Nói
  cho đúng: đợt kiểm chứng 13/10/2026 **không tìm được nguồn** cho con số "máy chiếu làm màu nhạt đi
  20–30%" từng được ghi ở đây — lời khuyên giữ nguyên, con số bỏ.

Cách tính lại: đọc `colors.*` trong `tokens.json`, chạy công thức trên — không cần LibreOffice.

## 5. Cái chưa đo được — đừng bịa

- **Chiều cao hàng của bảng chưa đo.** Biết chắc: ô bảng mặc định `CharHeight` = 18pt, và `wpp.addTable`
  **không nhận cỡ chữ**. Chưa đo được hàng cao bao nhiêu khi `height` bị ép, nên quy tắc "≤ 6 dòng dữ
  liệu" là **kinh nghiệm**, không phải số đo — muốn chắc thì dựng bảng rồi nhìn bằng mắt.
- **`checkLayout` không kiểm được chữ trong bảng**: ô bảng không phải text shape nên `chars` = 0, không
  có `overflow` cho bảng. Bảng tràn là lỗi duy nhất trong bộ lệnh này mà máy không bắt được.
- **`_shape_text_height` bị chặn ở 900pt** khi bật autogrow. Trên slide cao 446.5pt thì hộp như vậy đã
  bị `offslide` từ trước, nên trần này không ảnh hưởng thực tế — ghi lại để người sau khỏi tưởng số đo
  sai.
