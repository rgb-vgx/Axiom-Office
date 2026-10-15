# Làm bài theo tệp mẫu đính kèm (PowerPoint)

Dùng khi người dùng đính kèm tệp mẫu (.pptx/.potx) và bảo "dùng template", "làm lại theo mẫu". Mẫu
thường đặt logo, dải màu trong **ảnh nền trên từng slide** và chữ trong textbox tự vẽ, nên **không** tự vẽ
lại mẫu bằng `wpp.addText`, **không** dùng `wpp.addSlide`: nhân bản slide mẫu rồi thay chữ theo tên khung.
Lưới, cỡ chữ, màu ở `SKILL.md` mục 1–2 **không áp dụng** — mẫu đã có sẵn.

**Mẫu là của người dùng — giữ đúng như họ làm.** Mẫu tự làm thường có chữ cỡ 17pt, tiêu đề khác màu với
dải màu, logo hơi lệch, hộp chữ rộng quá mép slide. Đó là lựa chọn của họ: **không** đổi màu, cỡ chữ,
phông, vị trí khung của mẫu, kể cả khi trái với `SKILL.md`. Lỗi có sẵn trong mẫu (logo mờ, ảnh nền hở
mép, hộp tràn mép) thì **nói một lần** ở báo cáo cuối, không sửa trên từng slide. Chỉ sửa chữ mình viết vào.

Chữ có sẵn trong mẫu thường là **nội dung thật của bài khác** (tên giảng viên, đơn vị, năm), không phải
chữ chờ `[...]`: ô nào không có nội dung mới thì xoá, đừng để tên người khác lọt vào bài.

## 0. Làm lại cả bài (nhiều slide) → `slides_from_template`

Khi có tool `slides_from_template`, **dùng nó** cho mọi yêu cầu làm lại từ vài slide trở lên; không gọi từng
lệnh `wpp.*` ở mục 2 cho các slide đó.

Mặc định tool **biên soạn lại** bài (chế độ `compose`) như một người thiết kế bài giảng chuyên nghiệp:

1. `slides_from_template {template, source}` — `source` = đường dẫn bài cũ (dòng "Open document … full path").
   Tool tạo bài mới cạnh bài cũ và trả phần đầu của bài cũ (`source`: đoạn `p` và ảnh `i` đánh số của 15 slide),
   `outline` (đề mục cả bài), `layouts` và `how` (cách trả lời).
2. Viết các slide MỚI của phần đó rồi gọi lại **cùng `first`/`last`** với `slides` (và `drop` nếu bỏ ảnh gần trùng):
   - `title` ngắn, đủ nghĩa ("XQUANG BKCB – TƯ THẾ ĐỨNG THẲNG"); `from` = đoạn gốc slide dựa vào (`"7"`, `"7:p2-4"`).
   - `content`: `points` ý ngắn (< 15 từ, `**từ khoá**`, ý con bắt đầu bằng `"- "`), `imgs` 1–4 ảnh (`"7:i1"`),
     `caption` dưới ảnh ("Trái: … • Phải: …"), `callout` một lưu ý / cạm bẫy.
   - `cards`: 2–4 thẻ `{title, points}` — Ưu điểm / Hạn chế / Chỉ định. `table`: `[[tiêu đề…], [dòng…]]` ≤ 8 dòng × 5
     cột — bệnh cảnh × kỹ thuật. `compare`: `left`/`right` `{title, points, imgs}` — bình thường / bệnh lý.
   - `section` (mở mỗi phần lớn, `subtitle`), `agenda` (mục tiêu học tập, nội dung bài), `cover`, `update`.
   - **Mỗi đoạn của phần phải nằm trong `from` của một slide**: nguyên văn tự chép xuống ghi chú slide đó — không
     chép đoạn dài lên slide. Mỗi ảnh đặt lên slide hoặc nằm trong `drop`.
   - Chỉ dùng kiến thức của bài gốc; thông tin mới chỉ trên slide `update` với link `web_search` đã trả.
   - Phần đầu: bìa, "MỤC TIÊU HỌC TẬP" (3 mục tiêu), nội dung bài (theo `outline`). Phần cuối: slide "TỔNG KẾT".
3. Có `errors` → sửa đúng các slide được nêu và gửi lại cả danh sách. Không lỗi → tool dựng, lưu, trả phần kế tiếp.
4. Hết bài tool tự thêm slide cảm ơn và kết thúc lượt.

`mode: "design"` khi người dùng muốn **giữ nguyên văn** (chỉ bố cục lại: `use` trỏ đoạn/ảnh gốc, tool đặt nguyên văn).

Chỉ khi người dùng muốn **giữ nguyên từng slide** mới dùng `mode: "copy"` (mỗi slide cũ thành một slide mới; tool
trả `needsPlan` cho slide nó không tự quyết được — viết một dòng `{"src", "title", "items"|"imgs", "notes"}` cho mỗi
mục rồi gọi lại cùng `first`/`last`; hết bài gọi `closing: true`).

Mục 1–3 dưới đây dành cho sửa vài slide lẻ bằng tay.

## 1. Đọc mẫu (một lần)

`ppt_profile {path: <tệp mẫu>}` → phần `kinds`, mỗi phần tử là một slide mẫu:

| Trường | Nghĩa |
|---|---|
| `kind` | số slide trong mẫu — truyền cho `wpp.addSlideFromTemplate` |
| `use` | `cover` bìa · `agenda` mục tiêu · `section` chuyển phần · `content` · `compare` hai cột · `list` slide có mục lặp · `closing` slide cuối |
| `fixed` | ảnh nền (logo, dải màu) — không bao giờ xoá |
| `slots` | khung chữ: `shape` (tên), `role: title`, `maxChars`, `maxLines`, `item` (thuộc mục lặp số mấy) |
| `items` | các mục lặp (vd I / II / III), mỗi mục là danh sách tên shape |
| `safe` | vùng an toàn `[left, top, width, height]` — chữ/ảnh thêm vào phải nằm trong đây |
| `pictureBox` | ô đặt ảnh lớn (vùng an toàn trừ phần tiêu đề) |

## 2. Quy trình

1. **Lệnh sửa ĐẦU TIÊN luôn là `wpp.newFromTemplate {template, path}`** — không sửa bài cũ, không chèn slide
   mẫu vào bài cũ (`addSlideFromTemplate` vào bài khác khổ mẫu sẽ bị từ chối). `path` là tệp MỚI cạnh bài cũ
   (đường dẫn bài cũ có ở dòng "Open document … full path" của prompt), vd
   `<thư mục bài cũ>\<tên bài cũ> - mau moi.pptx`. Lệnh từ chối nếu tệp đã có: đổi tên khác.
2. Dàn ý: mỗi slide cũ → chọn `kind` + viết chữ theo `maxChars` của từng ô.
   - Bìa → `kind` có `use: cover`. Slide cảm ơn → `use: closing`.
   - Mục tiêu / danh sách ngắn → `use: list`, mỗi ý một mục lặp.
   - Slide có **ảnh** (XQ, siêu âm, CLVT, hình giải phẫu) → kind có `pictureBox`, xoá hết mục lặp, chép ảnh.
3. Mỗi slide: `wpp.addSlideFromTemplate {template, kind}` → trả `slide` và `textShapes` (tên khung).
4. `wpp.setShapeText {slide, shape, text}` cho từng ô; `\n` = xuống đoạn. Ô của mẫu giữ font, cỡ, màu.
   **Không vượt `maxChars`** — dài hơn thì viết gọn lại, chuyển diễn giải sang `wpp.setNotes`.
   Ô ngắn (nhãn, tóm tắt dưới một tên phần) chỉ điền **cụm từ khoá** rút từ câu gốc, không chép cả đoạn.
5. Số mục phải khớp số ý. Ưu tiên `kind` có **đúng** số mục. Mục lặp thừa (mẫu có 3, cần 2) →
   `wpp.deleteShapes {slide, shapes: [...]}` với **đủ tên** của mục đó trong `items` — xoá **mục cuối**,
   vì wpp không dời được các mục còn lại cho cân. Thiếu mục (cần 5, mẫu có 3) → tách thành hai slide cùng
   `kind`, tiêu đề slide sau thêm "(tiếp theo)", đánh số nối tiếp; không chép thêm mục.
   **Không để sót chữ mẫu**: mọi tên trong `textShapes` phải được `setShapeText` hoặc xoá. Ô không có nội
   dung thì xoá cả mục, đừng để ô có khung mà trống chữ.
6. Ảnh từ bài cũ → `wpp.copyPicture {from: <bài cũ>, fromSlide, slide, left, top, width, height}` với
   `left, top, width, height` = `pictureBox`. Bỏ `shape` để lấy ảnh lớn nhất slide cũ. Ảnh giữ tỉ lệ, căn giữa ô.
   **Không thêm nhãn, mũi tên, chú thích chữ lên hay cạnh ảnh y khoa**: bạn không nhìn thấy ảnh nên không
   biết bên nào là gì (lượt thật 13/10/2026: nhãn "GAN TRÁI / GAN PHẢI" đặt ngược với hình). Diễn giải ảnh đưa
   vào `wpp.setNotes`. Hai ảnh trên một slide: chia đôi `pictureBox` theo chiều ngang.
   Ảnh minh hoạ có sẵn trong slide mẫu (không thuộc `fixed`) chỉ để thấy bố cục: thay bằng ảnh thật của
   người dùng, không có thì chọn `kind` không ảnh — đừng để ảnh của người khác minh hoạ việc của đơn vị.
7. Soát: `wpp.checkLayout {slide}` sau mỗi slide; `overflow` → rút chữ ô đó. Không hạ cỡ chữ.
8. Xong: `wpp.save`.

## 3. Ví dụ (mẫu 3 slide: bìa, sidebar có 3 mục, cảm ơn)

```
wpp.newFromTemplate {template: "C:\...\mau.pptx", path: "C:\...\Bai giang - mau moi.pptx"}
wpp.addSlideFromTemplate {template: "C:\...\mau.pptx", kind: 1}          -> slide 1
wpp.setShapeText {slide: 1, shape: "TextBox 3", text: "CHẨN ĐOÁN HÌNH ẢNH U GAN"}
wpp.addSlideFromTemplate {template: "C:\...\mau.pptx", kind: 2}          -> slide 2 (mục tiêu)
wpp.setShapeText {slide: 2, shape: "TextBox 2", text: "MỤC TIÊU HỌC TẬP"}
wpp.setShapeText {slide: 2, shape: "TextBox 7", text: "Nhận biết hình ảnh điển hình của u máu gan..."}
wpp.deleteShapes {slide: 2, shapes: ["Rectangle 13","Rectangle 14","Oval 15","TextBox 16","TextBox 17"]}
wpp.addSlideFromTemplate {template: "C:\...\mau.pptx", kind: 2}          -> slide 3 (ảnh)
wpp.setShapeText {slide: 3, shape: "TextBox 2", text: "Đại thể gan"}
wpp.deleteShapes {slide: 3, shapes: [<mọi tên trong items>]}
wpp.copyPicture {from: "C:\...\bai cu.ppt", fromSlide: 2, slide: 3, left: 222, top: 122.6, width: 726, height: 405.4}
```

## 4. Giới hạn

- Chỉ chạy trên Microsoft PowerPoint. LibreOffice / WPS báo lỗi "not available" → nói thẳng với người dùng.
- `copyPicture` mở bài cũ (chỉ đọc) nếu chưa mở và để nguyên cho người dùng tự đóng.
- Bài cũ nhiều slide: làm từng phần (vd 10 slide một lượt) và báo người dùng đã xong đến slide nào.
