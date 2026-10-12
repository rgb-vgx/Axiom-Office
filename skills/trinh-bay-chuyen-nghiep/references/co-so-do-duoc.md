# Cơ sở đo được (Microsoft PowerPoint)

Mọi con số trong `SKILL.md` và `bo-cuc-slide.md` đo trên **PowerPoint thật** (Office 16, qua COM) ngày
**13/10/2026**, hộp chữ dựng **đúng như `wpp.addText`** trên Windows (`Shapes.AddTextbox`, toạ độ số
nguyên, chỉ đặt cỡ chữ). Không phải ước lượng cho đẹp.

## 1. Đo lại

**Đóng PowerPoint trước** — script từ chối chạy nếu PowerPoint đang mở (nó mở bài không cửa sổ rồi bỏ đi
bằng Quit; chạy chung phiên với người dùng thì hoặc làm sập bài họ đang mở, hoặc để lại bài rác):

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tests\win\measure_slide_grid_ppt.ps1
```

Script in số đo từng mục dưới đây; so với bảng ở đây. Nó không tự đánh đỏ/xanh.

## 2. Slide và hộp chữ mặc định

| Đo | Giá trị |
|---|---|
| Slide mặc định | **960 × 540pt** = 33.87 × 19.05cm (16:9) |
| Phông của hộp `wpp.addText` | **Arial** — đủ dấu tiếng Việt; wpp không có lệnh đổi phông |
| Lề trong hộp chữ | trên/dưới **3.6**, trái/phải **7.2** |
| Ngắt dòng | bật (`WordWrap`) |
| Co giãn | **tắt** (`AutoSize` = 0): hộp giữ chiều cao đã đặt — xem mục 5 |
| `\n` trong `text` | xuống dòng trong **cùng một đoạn** (như Shift+Enter) — gạch đầu dòng `"• "` vẫn thẳng hàng |

## 3. Hai hằng số dàn chữ

| Hằng số | Giá trị đo | Ghi chú |
|---|---|---|
| Chiều cao hộp CẦN cho n dòng | **`7.2 + n × 1.21 × cỡ_chữ`** | khớp cả sáu cỡ 14 / 18 / 20 / 24 / 28 / 40pt |
| Bề ngang một ký tự | **0.48–0.53 × cỡ_chữ** | dò nhị phân trên hộp rộng 880 / 713 / 430 |

Ví dụ chiều cao hộp (pt) đo được:

| Cỡ | 1 dòng | 2 dòng | 4 dòng | 8 dòng |
|---|---|---|---|---|
| 14 | 24.2 | 41.1 | 75.1 | 142.9 |
| 20 | 31.4 | 55.7 | 104.1 | 201.1 |
| 28 | 41.1 | 75.1 | 142.9 | 278.6 |
| 40 | 55.7 | 104.1 | 201.1 | 395.0 |

Bề ngang: chuỗi chữ liền cho 0.49–0.53, văn xuôi có dấu cho 0.48–0.51 (chỗ ngắt từ ăn bớt phần dư ở hộp
hẹp). `SKILL.md` dùng **0.53** — đầu trên của khoảng đo, nên viết theo nó thì không tràn.

## 4. Sức chứa theo lưới chuẩn ở 960 × 540

Hộp nội dung rộng **880**, cột đôi rộng **430**, vùng nội dung cao **370**.

| Hộp | Cao | Dòng vừa | Ký tự/dòng đo được (văn xuôi) |
|---|---|---|---|
| Tiêu đề, 28pt | 80 | **2** (75.1) — dòng 3 cần 109 | 65 → 2 dòng ≈ **130** |
| Nội dung, 20pt | 370 | **14** (346.6) | 91 (cột đôi: 43) |
| Nguồn, 14pt | 24 | **1** (24.2) | 131 |

**Trần không phải mục tiêu**: `SKILL.md` cho viết tiêu đề ≤ 80 ký tự và nội dung 10 dòng, chừa dư cho câu
thật dài hơn ước lượng. Slide đầy nhất (tiêu đề 66 ký tự + 5 gạch đầu dòng ≈ 75 ký tự + nguồn, tổng **505
ký tự**) dựng thật: tiêu đề 2 dòng, mỗi gạch **1 dòng**, không lỗi.

`dense` bắn khi **quá 600 ký tự** một slide — thấp hơn sức chứa hình học (14 dòng × 89 ≈ 1.200), nên 600
mới là trần thật. Năm gạch đầu dòng 12 từ + tiêu đề + nguồn ≈ 470–500 ký tự: vừa dưới trần, khớp với
`slide.maxBulletsPerSlide` × `slide.maxWordsPerBullet` trong `tokens.json`.

## 5. Hộp tự giãn — `checkLayout` thấy gì, không thấy gì

`Shapes.AddTextbox` mặc định **tự co giãn theo chữ** (`AutoSize` = 1). Với hộp như vậy `overflow` không
bao giờ bắn (chữ không thể cao hơn hộp), chữ dài chỉ lộ qua `overlap` — ngưỡng **10%** diện tích hộp nhỏ
hơn. Đo được: tiêu đề **3 dòng** giãn hộp tới 109, đè lên nội dung **9pt** = 8.3% → **không ai báo**.

Vì vậy từ 13/10/2026 `wpp.addText` trên Windows **tắt tự giãn** (`AutoSize` = 0) và giữ đúng chiều cao
được truyền. Cùng tiêu đề 3 dòng: hộp 80, chữ cần 100.8 → `overflow` (luật: `BoundHeight` > chiều cao hộp
+ 2pt). Slide chuẩn ở mục 4 vẫn không lỗi nào. Hộp chữ không viền, không nền nên nhìn bằng mắt không khác
gì. (Cần build lại add-in — bản DLL cũ vẫn tự giãn.)

`BoundHeight` chỉ tính dòng chữ, không tính lề trong 7.2pt, nên `overflow` hơi dễ dãi: hộp nội dung 370
để lọt 15 dòng (360) dù nhìn thì dòng cuối sát mép. Viết 10 dòng như `SKILL.md` thì không chạm tới.

## 6. Bảng

| Đo | Giá trị |
|---|---|
| Chữ trong ô | **Arial 18pt** — `wpp.addTable` không nhận cỡ chữ |
| Chiều cao hàng | chia đều chiều cao bảng: 7 hàng trong 370 → **52.9** mỗi hàng; 10 hàng → 37.0 |
| Hàng thấp nhất (1 dòng 18pt) | ≈ 28.8 → bảng **13 hàng** đã cao 374.4, vượt vùng 370 |
| Ô chữ dài | hàng giãn ra: ô 69 ký tự ở cột 220 thành 4 dòng, bảng 7 hàng từ 370 lên **410.7** (đáy 540.7) |

`checkLayout` không đếm chữ trong ô, nhưng **có thấy khung bảng**: bảng giãn đè lên hộp "Nguồn" thì báo
`overlap`. Đáy 540.7 trên slide cao 540 vẫn nằm trong dung sai 2pt nên **không** có `offslide` — đó là lý
do `bo-cuc-slide.md` bắt luôn đặt hộp "Nguồn" khi có bảng.

## 7. Ảnh và layout

**Ảnh.** `Shapes.AddPicture` coi `-1` là "giữ cỡ gốc" cho **từng chiều riêng**: truyền `width` = 491 với
`height` = -1 làm ảnh 16:9 (1600 × 900px) thành **491 × 445.5** — méo. Từ 13/10/2026 `wpp.addImage` trên
Windows thêm ảnh ở cỡ gốc, khoá tỉ lệ rồi mới đặt chiều được truyền: cùng ảnh ra **491 × 276.2**, ảnh 4:3
ra 491 × 368.3. (Cần build lại add-in — bản DLL cũ vẫn kéo méo.)

**Layout** của `wpp.addSlide` — số placeholder, đều rỗng:

| layout | 1 | 2 | 11 | 12 |
|---|---|---|---|---|
| Placeholder | 2 (tiêu đề + phụ đề) | 2 | 1 | **0** |

`wpp.addText` luôn tạo hộp mới, không ghi vào placeholder. `checkLayout` bỏ qua placeholder rỗng (không
hiện khi trình chiếu), nhưng khi soạn chúng vẫn hiện "Bấm để thêm tiêu đề" — nên luôn `layout: 12`.

## 8. Tương phản màu — tính được, không phải cảm nhận

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

Cách tính lại: đọc `colors.*` trong `tokens.json`, chạy công thức trên — không cần mở PowerPoint.

## 9. Chưa đo

- **Slide 4:3 (720 × 540)** và các cỡ khác: hằng số dàn chữ ở mục 3 không phụ thuộc cỡ slide, nên lưới
  tính theo `W`, `H` vẫn đúng; sức chứa thì tính lại bằng công thức. Ví dụ 720 × 540: hộp nội dung rộng
  640 → ≈ 60 ký tự một dòng ở 20pt, vẫn 14 dòng.
- **PowerPoint cho Mac / PowerPoint trên web**: không đo, không hỗ trợ (wpp chạy qua COM trên Windows).
- **Tiêu đề bìa và nhãn ba con số** ở `bo-cuc-slide.md` tính bằng công thức, chưa dựng thật từng mẫu.
