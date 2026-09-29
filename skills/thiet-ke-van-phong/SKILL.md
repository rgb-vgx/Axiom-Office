---
name: thiet-ke-van-phong
description: >-
  Nguyên tắc thiết kế chung cho tài liệu văn phòng (Word, Excel, PowerPoint): phân cấp chữ, bảng màu
  theo ngữ nghĩa, khoảng cách, độ tương phản, nhất quán. Dùng khi người dùng muốn tạo mới hoặc làm lại
  cho "đẹp", "chuyên nghiệp", "trình bày lại", "thiết kế" một tài liệu, bảng hay slide. Không dùng cho
  sửa nhỏ như in đậm một dòng hay sửa một con số.
---

# Thiết kế văn phòng

Mục tiêu: người đọc hiểu nội dung nhanh. Đẹp = rõ ràng + nhất quán, không phải nhiều màu.

## Lấy giá trị cụ thể từ token

Gọi `read_skill_file {name: "_design", path: "tokens.json"}` rồi dùng **đúng giá trị** trong đó cho
lệnh (màu `#RRGGBB`, cỡ chữ, number format). Không tự chế màu mới.

## Quy tắc

1. **Phân cấp**: tối đa 3 cấp chữ trên một trang/slide (tiêu đề, nội dung, chú thích). Cấp cao hơn to
   hơn hoặc đậm hơn — không dùng cả hai cùng lúc cho mọi thứ.
2. **Màu theo ngữ nghĩa**: `primary` cho tiêu đề/hàng tiêu đề bảng, `accent` để nhấn một điểm,
   `success`/`warning`/`danger` chỉ cho trạng thái tốt/cảnh báo/xấu. Tối đa 2 màu nhấn mỗi trang.
3. **Tương phản**: chữ trên nền `primary` dùng `onPrimary` (trắng). Chữ thường dùng `text`, phụ dùng
   `muted`. Không đặt chữ xám nhạt trên nền trắng.
4. **Nhất quán**: cùng loại phần tử cùng định dạng (mọi hàng tiêu đề bảng giống nhau, mọi slide nội
   dung cùng vị trí tiêu đề).
5. **Khoảng trắng**: đừng lấp đầy trang. Ít chữ hơn, lề rộng hơn.

## Quy trình

- [ ] Đọc tài liệu hiện tại trước (getText / readRange / listSlides) để biết đang có gì.
- [ ] Viết ra một **spec ngắn** trong đầu: font, 2–3 màu, cỡ chữ từng cấp — rồi bám theo cho mọi lệnh.
- [ ] Nạp thêm skill riêng của app nếu có trong danh sách: `the-thuc-van-ban` (Word),
      `bao-cao-du-lieu` (Excel), `trinh-bay-chuyen-nghiep` (PowerPoint).
- [ ] Làm xong thì **đọc lại để tự kiểm** bằng lệnh kiểm tra của app (`writer.checkTables`,
      `et.checkRange`, `wpp.checkLayout`) và sửa mọi `issues` trước khi trả lời.
- [ ] Có tool `look_at_document` thì nhìn ảnh chụp một lần sau cùng; không gọi lại nhiều lần (tốn token).
