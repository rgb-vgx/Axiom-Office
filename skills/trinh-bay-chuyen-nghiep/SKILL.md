---
name: trinh-bay-chuyen-nghiep
description: >-
  Quy tắc trình bày slide PowerPoint chuyên nghiệp: một thông điệp mỗi slide, lưới và lề, thang cỡ chữ,
  mật độ chữ, bảng khi cần so sánh, chi tiết dài đưa vào ghi chú. Dùng khi tạo mới hoặc làm lại bài
  thuyết trình, slide, "làm slide cho đẹp", "thiết kế lại slide". Không dùng khi chỉ sửa một chữ.
apps: [wpp]
---

# Trình bày slide chuyên nghiệp (Microsoft PowerPoint)

**Có tệp mẫu đính kèm** ("dùng template", "làm theo mẫu") → đọc
`read_skill_file {name: "trinh-bay-chuyen-nghiep", path: "references/mau-dinh-kem.md"}` và làm theo đó
(cả bài: tool `slides_from_template`; vài slide lẻ: `wpp.newFromTemplate`, `wpp.addSlideFromTemplate`,
`wpp.setShapeText`, `wpp.copyPicture`); phần tự vẽ
(`references/tu-ve-bang-wpp.md`) không áp dụng. Không tự vẽ lại mẫu bằng `wpp.addText`.

**Có tool `slides_from_brief`** (PowerPoint): slide **mới** làm bằng tool này — chọn layout (`cover`,
`section`, `content`, `cards`, `table`, `figures`; quy trình `process`, dòng thời gian `timeline`, sơ đồ khối
`diagram` vẽ bằng hình khối và mũi tên sửa được; biểu đồ `chart` cột/thanh/đường/tròn là biểu đồ thật của
PowerPoint: số trong `values` là số trần, đơn vị để ở `unit`) và viết chữ; tool tự đặt vị trí, cỡ chữ, màu theo lưới chuẩn
và tự soát. Mũi tên của `diagram` được tự đi vòng quanh các hộp khác, nên vẽ đủ cả mũi tên quay về và mũi tên nhảy qua
một nút; nếu PowerPoint vẽ lệch và `checkLayout` báo dây cắt hộp/chồng dây, tool tự vẽ lại (tối đa 2 lần mỗi
đường) — đừng dựng lại slide vì lỗi đó. Mỗi tầng tối đa 4 nút. Thành phần chạy **xuyên suốt** mọi tầng (giám sát /
Observability, nhật ký audit) đặt vào `span: [{"text": "Observability: trace, log, metric"}]` (tối đa 2) — tool vẽ
thành dải dọc cạnh cả sơ đồ, không phải nút, không nối mũi tên tới nó; đừng bỏ thành phần đó để hết lỗi dây. Đề bài cần
phân biệt ai làm bước nào (vd AI và hệ thống nghiệp vụ) → gán `group` cho từng bước/nút (2–3 nhóm, tên ngắn): mỗi
nhóm một màu và có chú thích. Gặp lỗi `TOO_LONG` thì rút chữ theo gợi ý rồi gọi lại với **cả** danh sách slide.
Đã dựng xong mà `checkLayout` (hoặc `issues` trong kết quả) báo lỗi ở vài slide → gửi lại **chỉ** các slide đó, mỗi slide
kèm `replace: <số slide>` để dựng lại tại chỗ; không dựng lại cả bài, không xoá các slide còn lại. Không cần đọc `_design/tokens.json` hay
`references/bo-cuc-slide.md`: tool đã tự dùng. Mục 1 (nội dung) vẫn áp dụng.

**Không có tool `slides_from_brief`** (hoặc sửa lẻ một hộp chữ có sẵn bằng `wpp.addText`) → đọc
`read_skill_file {name: "trinh-bay-chuyen-nghiep", path: "references/tu-ve-bang-wpp.md"}`: lưới, cỡ chữ, sức chứa,
giới hạn của wpp và checklist soát.

**Nguồn là văn bản của cơ quan Đảng, Nhà nước** (nghị quyết, kế hoạch, báo cáo, chuyên đề có quốc hiệu
hoặc "ĐẢNG CỘNG SẢN VIỆT NAM" ở đầu) → đọc thêm
`read_skill_file {name: "trinh-bay-chuyen-nghiep", path: "references/van-ban-nha-nuoc.md"}`: giữ nguyên
văn, tiêu đề là tên mục của văn bản (thay cho mục 1.1), bìa lấy từ đầu văn bản.

## 1. Nội dung

0. **Đúng số slide người dùng xin.** "Làm một slide" là đúng **một** slide; "bài 6 slide" là 6 slide. **Không
   tự thêm** bìa, mục lục, slide chia mục, slide "Cảm ơn" khi không được xin, và không thêm slide nào có ý
   không có trong yêu cầu hay tài liệu nguồn. Chỉ khi xin cả bài thuyết trình mà không nói số slide mới mở
   đầu bằng bìa.
1. **Một thông điệp mỗi slide.** Tiêu đề là **câu kết luận** ("Doanh thu tăng 12% nhờ kênh trực
   tuyến"), không phải nhãn ("Doanh thu"). Viết chữ thường, không viết hoa cả câu. Đọc liền các tiêu đề
   phải thành một mạch lập luận.
2. Tối đa `slide.maxBulletsPerSlide` gạch đầu dòng, mỗi gạch ≤ `slide.maxWordsPerBullet` từ — **nhắm 4**,
   trần là để chặn chứ không phải để chạm. Gạch đầu dòng là ký tự `"• "` thật, nối dòng bằng `\n` trong
   `text` (wpp không có lệnh bullet).
3. So sánh số liệu → bảng (`table`, ≤ 6 dòng dữ liệu), ba con số lớn (`figures`) hoặc biểu đồ (`chart`). Đừng kể
   số trong đoạn văn. Lộ trình, các giai đoạn → `timeline`; các bước nối tiếp → `process`.
4. Slide có số liệu → ghi nguồn (`source`).
5. **Một màu nhấn.** Nền trắng; màu theo token (tool tự chọn; tự vẽ thì theo `references/tu-ve-bang-wpp.md` mục 2). Nhấn mọi thứ thì không còn gì nổi bật.
6. **Màu không bao giờ là dấu hiệu duy nhất**: "Đạt" / "Chậm" phải có chữ — người mù màu và bản in đen
   trắng không thấy màu.
7. **Màu cấm** (thiếu tương phản, bảng tính ở `references/co-so-do-duoc.md` mục 8; chỉ cần khi tự chọn màu):
   - `colors.warning` không dùng cho chữ thường — chỉ chữ ≥ 18pt hoặc in đậm.
   - `colors.accent`, `colors.success` không dùng cho chữ thường trên nền `colors.surface`.
   - Dòng nguồn chỉ dùng `colors.muted` hoặc `colors.text`.

## 2. Checklist (có `slides_from_brief`)

- [ ] `wpp.listSlides` — bài đang có gì. Thêm vào cuối; không xoá slide cũ trừ khi được yêu cầu.
- [ ] Dàn ý: đúng số slide người dùng xin (mục 1.0); mỗi slide một câu kết luận; đọc các tiêu đề thấy thành mạch.
- [ ] **Một** lần gọi `slides_from_brief` với cả bài; lỗi `TOO_LONG` → rút chữ về `max` ký tự rồi gọi lại cả bài.
- [ ] Kết quả còn `issues` ở vài slide → gửi lại chỉ các slide đó kèm `replace`. Slide đã 0 lỗi thì để yên.
- [ ] Đối chiếu với nguồn: không sót ý nào, thứ tự giữ theo mạch bản gốc, số liệu đều có trong nguồn.

## 3. Báo cáo cho người dùng

Người đọc là nhân viên văn phòng, không phải lập trình viên. Viết ngắn, tiếng Việt thường, **không** đưa
tên lệnh (`wpp.*`), số kind, toạ độ. Gọi slide bằng số trang và tên dễ hiểu ("slide 5 — bốn mục đích").

- Một câu: bao nhiêu slide (so với bản gốc nếu làm lại), đã soát lỗi.
- **Khác bản gốc**: slide nào gộp, tách ("(tiếp theo)"), đổi thứ tự; phần nào chuyển vào ghi chú.
- **Chỗ tự thêm hoặc sửa** để người dùng duyệt: nhãn ngắn, chữ đã rút gọn, lỗi chính tả, chữ viết tắt.
- **Cần lưu ý**: việc không làm được (vd không có `slides_from_brief`: biểu đồ → đã làm bảng), slide còn lỗi `checkLayout`, nhắc lưu
  (Ctrl+S).

## Đọc thêm

- `references/tu-ve-bang-wpp.md` — tự vẽ bằng `wpp.addText` khi không có `slides_from_brief`: lưới, cỡ chữ,
  sức chứa, giới hạn của wpp, checklist soát.
- `references/bo-cuc-slide.md` — toạ độ từng mẫu: bìa, chia mục, ba con số, bảng, hai cột, ảnh; bảng
  chọn mẫu theo loại nội dung (mục 7).
- `references/van-ban-nha-nuoc.md` — slide từ nghị quyết, kế hoạch, báo cáo của cơ quan Đảng, Nhà nước:
  giữ nguyên văn, bìa từ đầu văn bản, phân biệt Đảng / chính quyền.
- `references/cau-chuyen.md` — dàn ý cả bài: slide tóm tắt cho người quyết định, tiêu đề hành động, số
  slide, cỡ chữ theo khoảng cách người xem, nguồn tham khảo.
- `references/co-so-do-duoc.md` — số đo trên PowerPoint, cách đo lại, bảng tương phản màu.
- `references/mau-dinh-kem.md` — làm bài theo tệp mẫu đính kèm: đọc `kinds` của `ppt_profile`, nhân bản slide
  mẫu, thay chữ theo tên khung, chép ảnh từ bài cũ.
