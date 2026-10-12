# Dàn ý một bài thuyết trình

Tổng hợp từ hai đợt tìm nguồn trên internet; đợt **kiểm chứng lại ngày 13/10/2026** mở từng trang để đối
chiếu. Quy tắc ghi chép của file này:

- **Chỉ ghi URL đã mở được thật** và nội dung khớp với điều mình nói. Mọi URL dưới đây đã mở và đọc.
- Khẳng định nào **không tìm được nguồn** thì nằm ở mục 10 — giữ nguyên là "chưa rõ", đừng nâng thành quy
  tắc, và đừng gán bừa cho một tên nghe có vẻ uy tín.
- Các **tên khung** (Minto, BLUF, SCR/SCQA) ghi theo tên như tài liệu vẫn gọi. Đợt này không mở lại liên
  kết cho chúng, nên chúng ở mức "tên được dùng rộng rãi", không phải "đã đối chiếu bản gốc".

Ghi chú cho người kiểm lại: trong môi trường này **`WebSearch` trả về rỗng với mọi truy vấn**, nên đợt
kiểm chứng chạy hoàn toàn bằng `WebFetch` trực tiếp (đoán URL rồi mở). Vài nơi chặn hoặc đòi đăng nhập
(Nature trả về trang login, vài blog trả 403/404) — những chỗ đó ghi là "không mở được", không suy diễn.

## 1. Nguyên tắc lõi

- **Kết luận trước, lý lẽ sau** — tháp Minto (Barbara Minto, *The Minto Pyramid Principle*): mỗi slide và
  cả bài đi từ kết luận xuống chứng cứ, không đi từ dữ liệu lên kết luận.
- **BLUF** (bottom line up front): kết luận đặt ngay đầu văn bản — chuẩn viết báo cáo của quân đội Mỹ, nay
  phổ biến trong tài liệu điều hành.
- **SCR** (Situation – Complication – Resolution) là cách gọi của McKinsey; **SCQA** (… Question – Answer)
  là biến thể, hai tên này hay bị dùng lẫn — cùng một mạch, đừng coi là hai trường phái.
- **Một thông điệp một slide.** Slide thứ hai của một thông điệp thì cắt.
- Phép thử rẻ nhất: **đọc riêng tiêu đề các slide phải thành một câu chuyện hoàn chỉnh.** Nếu không, dàn
  ý sai chứ chưa phải hình thức sai.

## 2. Slide 2 nên là tóm tắt cho người quyết định

Sáu phần, gói trong một slide: **Mục tiêu · Bối cảnh · Vấn đề · Cách giải quyết · Lợi ích · Việc cần
làm**. Người chỉ đọc đúng slide này vẫn ra được quyết định. Với báo cáo tháng/quý, đây là slide đáng đầu
tư thời gian nhất — người quyết định thường không đọc quá nó.

## 3. Tiêu đề hành động

- Nêu **kết luận hoặc con số**, không nêu chủ đề: "Doanh thu tăng 12% nhờ kênh trực tuyến", không phải
  "Tình hình doanh thu". Đây đúng là nguyên tắc **assertion–evidence**: tiêu đề là một *khẳng định*, phần
  thân là chứng cứ cho nó, thay cho gạch đầu dòng liệt kê chủ đề
  (<https://www.assertion-evidence.com/>).
- **Độ dài** — hai nguồn lệch nhau, và cả hai đều không vượt trần kỹ thuật của lưới ta:
  - PresentationLoad: tiêu đề slide lý tưởng **6–10 từ** — "the ideal length for headings on PowerPoint
    slides is 6-10 words" (<https://presentationload.com/blogs/blog/powerpoint-heading-length>).
  - Slideworks, theo cách viết action title của McKinsey: **≤ 15 từ và không quá 2 dòng** — "Keep an
    action title under 15 words and never longer than two lines"
    (<https://slideworks.io/resources/how-to-write-action-titles-like-mckinsey>).
  - Bộ skill này chọn **≤ 15 từ, ≤ 2 dòng**, vì trần kỹ thuật của lưới trong `SKILL.md` cũng đúng 2 dòng ở
    28pt (≈ 92 ký tự ≈ 15 từ tiếng Việt) — hai bên khớp nhau. Muốn chắc ăn thì nhắm **6–10 từ**.
- Câu chữ thường (không viết hoa kiểu Title Case, càng không ALL CAPS).

## 4. Hình thức — kỷ luật, không phải khiếu thẩm mỹ

- Vị trí tiêu đề, nội dung, nguồn, số trang **cố định trên mọi slide**; không phá lề. Với công cụ đặt toạ
  độ bằng số như wpp, đây là quy tắc dễ vi phạm nhất và cũng dễ sửa nhất: dùng đúng một lưới.
- **Một màu nhấn + xám.** Nhấn mọi thứ thì không còn gì nổi bật.
- Bỏ trang trí của template (khung, hoa văn, logo to) — thứ không mang thông tin thì bỏ.
- Nhiều nhất **ba** cỡ chữ trên một slide (tiêu đề, nội dung, chú thích — đúng ba cấp mà
  `thiet-ke-van-phong` cho phép), và **trong phần nội dung thì nhiều nhất hai** cỡ, chênh nhau ~2pt. Thang
  cỡ chữ ở `tokens.json` đã đủ rộng; đừng thêm cỡ thứ tư để "phân cấp" thêm.
- Hướng dẫn trợ năng của Microsoft cho PowerPoint nói slide nên dùng "a larger font size (18pt or larger),
  sans serif fonts, and sufficient white space" — tức **chữ thường ≥ 18pt** và **chừa khoảng trắng**. Cỡ
  chữ theo đó ở mục 8; khoảng trắng chính là lưới ở `SKILL.md`.

## 5. Dài bao nhiêu slide

Các nguồn **không thống nhất**, nên phải chọn theo mục đích chứ không có một con số đúng:

| Kiểu | Số slide các nguồn nêu |
|---|---|
| Báo cáo điều hành / quý (QBR) | 10–25 |
| Trình bày trước hội trường | ≥ 1 slide mỗi phút |
| Tài liệu tư vấn gửi để **đọc** | 50+ (người đọc tự nhịp, không ai thuyết trình) |

**Lựa chọn của bộ skill này** (quyết định của dự án, không phải số đo): báo cáo gửi kèm để đọc → **10–20
slide**; trình bày miệng 30 phút → **15–20 slide**, phần dư đẩy vào `wpp.setNotes` và phụ lục. Đợt kiểm
chứng này **không tìm được nguồn** cho các con số trong bảng — xem mục 10.

## 6. Dữ liệu

- Bảng khi cần so sánh **chính xác**; biểu đồ khi cần thấy **xu hướng**. wpp **không có lệnh biểu đồ**,
  nên phần "xu hướng" phải chuyển thành bảng hoặc 3 con số lớn — và nói thẳng trong câu trả lời là không
  vẽ được biểu đồ.
- **Không dùng hai trục tung** (dual-axis) trên một biểu đồ: Stephen Few, *Dual-Scaled Axes in Graphs* —
  nguyên văn "It is inappropriate to use more than one quantitative scale on a single axis, because, to
  some degree, this encourages people to compare magnitudes of values between them, but this is
  meaningless"
  (<https://www.perceptualedge.com/articles/visual_business_intelligence/dual-scaled_axes.pdf>). Đây là
  một trong số ít quy tắc trong file này **có nguồn mở được** — nói ra thì không sợ sai.
- **"Biểu đồ tròn ≤ 5 phần" thì ngược lại**: nghe rất chắc nhưng **không có nguồn**. Đã mở bài nổi tiếng
  nhất về chủ đề này (Stephen Few, *Save the Pies for Dessert*) — bài bàn về hạn chế của biểu đồ tròn
  nhưng **không nêu con số 5**, Wikipedia cũng không. Xem mục 10.
- Mọi slide có số liệu: ghi nguồn ở dòng chú thích cuối slide.

## 7. Tiếng Việt

- Dấu thanh **xếp tầng** (ế, ộ, ữ, ằ…) nên ở cỡ nhỏ nét dấu dính vào nhau; vì vậy không xuống dưới 12pt,
  không viết hoa cả câu, tránh phông mảnh (thin/light). Lưu ý: con số 12pt ở đây là **đo được** trên bản
  LibreOffice này (xem `co-so-do-duoc.md`), không phải khuyến nghị trợ năng — sàn trợ năng cao hơn, ở mục 8.
- Phông có đủ dấu tiếng Việt: **Noto Sans, Be Vietnam Pro, DejaVu Sans, Vollkorn**. wpp **không đặt được
  phông**, nên thông tin này chỉ dùng khi tổ chức cấu hình sẵn phông mặc định cho LibreOffice — đừng hứa
  với người dùng là sẽ đổi được phông.
- Hướng dẫn thiết kế slide **riêng cho doanh nghiệp/nhà nước Việt Nam**: đợt tìm này **không có nguồn
  nào**. Các mục ở trên là thực hành chung, không phải chuẩn của Việt Nam.

## 8. Cỡ chữ nhỏ nhất và khoảng cách người xem

Ba con số khác nhau, nhưng **không mâu thuẫn** — chúng nói về ba tình huống khác nhau:

| Nguồn | Con số | Nguồn đó nói trong ngữ cảnh nào |
|---|---|---|
| Microsoft, hướng dẫn trợ năng cho PowerPoint | chữ thường **≥ 18pt** | slide phải đọc được với người thị lực kém |
| Guy Kawasaki, quy tắc 10/20/30 | **không nhỏ hơn 30pt** | pitch gọi vốn: 10 slide, 20 phút |
| Bảng khoảng cách của Paradi (ThinkOutsideTheSlide) | tuỳ khoảng cách, xem dưới | chọn cỡ chữ khi biết phòng và màn chiếu |

- <https://support.microsoft.com/en-us/office/make-your-powerpoint-presentations-accessible-to-people-with-disabilities-6f7772b2-2f33-4bd2-8ca7-dae3b2b3ef25>
- <https://guykawasaki.com/the_102030_rule/> — nguyên văn "no font smaller than thirty points"
- <https://www.thinkoutsidetheslide.com/selecting-the-correct-font-size/>

**Sàn 18pt của Microsoft là sàn trợ năng**, khác hẳn `slide.type.minimum` = 12 trong `tokens.json` (đó là
ngưỡng `small-font` của `checkLayout`, tức **sàn kỹ thuật**: dưới nó thì máy báo lỗi, chứ không phải trên
nó thì đọc được). `slide.type.body` = 20pt nằm trên cả hai sàn. Quy tắc 10/20/30 của Kawasaki là cho **pitch
gọi vốn**, đừng áp nguyên con số 30pt vào báo cáo nội bộ — nhưng hướng của nó thì đúng: chữ to hơn thì an
toàn hơn.

**Khi biết phòng chiếu**: bảng của Paradi
(<https://thinkoutsidetheslide.com/wp-content/uploads/2012/08/ViewingDistanceTable16x9.pdf>) cho màn **32
inch**: 24pt đọc được tới **15 ft** (≈ 4,6m), 28pt tới **18 ft** (≈ 5,5m). Hai dòng đó khớp với công thức
`khoảng_cách_ft ≈ 0.0201 × cỡ_chữ × đường_chéo_inch` — nhưng công thức là **suy ra từ bảng**, nguồn không
viết công thức này ra; cần số thì tra bảng, đừng trích công thức như thể nguồn đã nói. Không biết phòng
rộng bao nhiêu thì chọn cỡ lớn (20–28pt), đừng chọn 14pt.

## 9. Màu và trợ năng

- **Tương phản** — WCAG 2.1 SC 1.4.3: **4.5:1** cho chữ thường, **3:1** cho chữ lớn; "chữ lớn" được định
  nghĩa là **≥ 18pt, hoặc ≥ 14pt nếu in đậm** (<https://www.w3.org/TR/WCAG21/#contrast-minimum>). Đây là
  ngưỡng duy nhất trong tài liệu này có nguồn chuẩn; bảng tính đầy đủ cho `tokens.json` ở
  `co-so-do-duoc.md` mục 4.
- **Màu không được là dấu hiệu duy nhất** — WCAG 2.1 SC 1.4.1, nguyên văn "Color is not used as the only
  visual means of conveying information, indicating an action, prompting a response, or distinguishing a
  visual element" (<https://www.w3.org/TR/WCAG21/#use-of-color>).
- **Bảng màu an toàn cho người mù màu** — Okabe & Ito, *Color Universal Design*
  (<https://jfly.uni-koeln.de/color/>), có cả dải màu cụ thể (vermillion, bluish green, reddish purple…) và
  lưu ý tránh dải vàng-lục. Cùng trang đó cũng nói "Do not convey information in color only. Show
  difference BOTH in color and shape" — độc lập với WCAG mà cùng một kết luận.
- **Tỉ lệ người mù màu**: Okabe & Ito ghi **1/12 nam gốc Âu (8%)**, **1/20 gốc Á (5%)**, **1/25 gốc Phi
  (4%)**. Nên câu "≈ 8% nam giới" chỉ đúng cho nhóm gốc Âu — viết gọn thì ghi khoảng 5–8% nam giới, đừng
  lấy 8% làm con số chung. Cùng trang: trong một phòng 250 người thì "hơn mười người" mù màu.
- Bảng màu **Wong** (Nature Methods 2011) được trích dẫn nhiều, nhưng bài gốc **bị paywall** — đợt này
  không mở được. Nhắc tên thì được, đừng chép số hiệu bài như thể đã đọc.

## 10. Chưa kiểm chứng được — đừng biến thành quy tắc

Đợt kiểm chứng 13/10/2026 **không mở được trang nào nói đúng** những điều sau — có trang mở được nhưng
nội dung không khớp, phần lớn là không tìm ra trang nào cả. Không phải chúng sai — chỉ là chưa có nguồn,
nên đừng viết vào slide với giọng "theo chuẩn X", cũng đừng gán cho tên tác giả nào:

- **Indezine nói cỡ chữ tối thiểu 24pt**: mở nhiều trang indezine.com, không có con số 24pt (trang gần
  nhất chỉ nhắc lại 10/20/30 của Kawasaki = 30pt). Nếu cần một nguồn thật nêu 24pt thì đó là Paradi với
  "24 to 32 point fonts for your body text" — nhưng trang đó đợt này không mở.
- **Kosslyn** (*Clear and to the Point*) với các ngưỡng "> 4 gạch đầu dòng / > 2 dòng mỗi gạch / < 20pt":
  không có nguồn mở được.
- Quy tắc **"6x6"** (6 gạch × 6 từ) và việc nó bị phản đối: không có nguồn.
- **Thí nghiệm của Mayer** về học đa phương tiện: mở được trang phòng nghiên cứu
  (<https://mayerlab.psych.ucsb.edu/publications>) — xác nhận có chương trình nghiên cứu và sách *Multimedia
  Learning* (tái bản lần 3, 2021), nhưng **không có số liệu thí nghiệm cụ thể**; đừng trích số. **Tufte phản
  đối PowerPoint**: không có nguồn.
- **Garner & Alley 2013**: bài có thật, được liệt kê ở <https://www.assertion-evidence.com/research-papers.html>,
  nhưng các chỉ số hay được trích (t(93.43) = 3.67, d = 0.81, 21,2 so với 41,5 từ/slide) **không có trên
  trang đó** — cần bản PDF bài báo, đợt này không tìm được. Trang đó có nêu "p < .01". Đừng trích các chỉ
  số chi tiết.
- **"Action safe 90%"**, **lề = 1/20 chiều rộng (≈ 5%)**, **lưới 12 cột**, **nhịp 8pt/4pt**: không có
  nguồn. (Trang Wikipedia *Overscan* — <https://en.wikipedia.org/wiki/Overscan> — nói 85% và "3,5%/5%", có
  nơi nói 5%, có nơi nói 10%, và bản thân nó ghi "some say 5%, some say 10%": tức chính lĩnh vực này cũng
  không thống nhất.) Lưới trong `SKILL.md` vì vậy đặt lề 40pt theo **số đo và tỉ lệ slide**, không viện
  dẫn quy tắc phần trăm nào.
- **Tỉ lệ màu 60-30-10**: không tìm được nguồn nào mở được.
- **Máy chiếu làm màu nhạt đi 20–30%**: không có nguồn. Lời khuyên "chừa dư tương phản" vẫn hợp lý vì
  điều kiện chiếu thật không kiểm soát được, nhưng **đừng kèm con số 20–30%** như thể đo được.
- **Halation** (viền sáng quanh chữ trắng trên nền đen): trang Wikipedia *Halation*
  (<https://en.wikipedia.org/wiki/Halation>) nói về hiện tượng này trong **nhiếp ảnh phim**, không nói về
  chữ trên slide. Hiện tượng có thật với màn hình, nhưng đừng trích trang đó làm nguồn.
- **Cam/vàng hay trượt tương phản**: không có nguồn. Cái **đo được** là `colors.warning` của bộ token này
  chỉ đạt 4.33:1 — dùng con số của mình, đừng suy ra quy tắc chung cho mọi màu cam.
- **Biểu đồ tròn ≤ 5 phần**: thực hành phổ biến, không có nguồn mở được. Bài của Few
  (<https://www.perceptualedge.com/articles/visual_business_intelligence/save_the_pies_for_dessert.pdf>)
  đã mở nhưng chỉ bàn hạn chế chung của biểu đồ tròn, không nêu ngưỡng số phần. ("Tránh hai trục tung" thì
  **có** nguồn — xem mục 6.)
- Mọi con số "số slide" ở mục 5: **thực hành phổ biến**, không phải chuẩn đo được.
- Quy ước "số lớn / KPI" (một slide một con số, số phải to hơn cỡ X, có nhãn phụ…): **không có nguồn
  chắc**. Dùng `slide.type.figure` vì nó hợp lưới, đừng viện dẫn "chuẩn".
- Quy tắc canh lề trong ô bảng (số canh phải, chữ canh trái…): **không có nguồn chắc**.
- Thuật ngữ **"ghost deck"**: không xác minh được nghĩa được dùng.
