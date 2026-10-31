---
name: cong-thuc-bang-tinh
description: >-
  Viết, giải thích công thức bảng tính việc văn phòng: đọc số tiền thành chữ, năm sinh/giới tính từ CCCD, tuổi, thâm
  niên, ngày công, đếm không trùng, tra cứu. Dùng khi muốn "công thức", "hàm", "đọc số thành chữ", "giải thích
  công thức".
apps: [et]
---

# Công thức bảng tính cho việc văn phòng

## Cách làm

1. Đọc vùng dữ liệu trước (`et.readRange`) để biết cột nào là gì, dữ liệu là **số, ngày hay chữ** (CCCD, mã số
   thường là chữ để giữ số 0 đầu; ngày nhập tay có khi là chữ "20/03/2015" chứ không phải ngày).
2. Viết **một** công thức cho dòng đầu rồi điền cả cột bằng `et.fillRange` (range = cả cột kết quả, ví dụ `D2:D500`),
   không ghi từng ô. Tham chiếu cố định (bảng tra, ngày mốc) dùng `$`.
3. Đọc lại vài ô kết quả (`et.readRange`): không còn `#VALUE!`, `#N/A`, `#NAME?`; kết quả đúng với một hai dòng tự tính
   nhẩm. Lỗi thì sửa công thức, không ghi đè bằng giá trị.
4. Báo người dùng công thức đã dùng và ý nghĩa ngắn gọn.

Giải thích công thức có sẵn: `et.readRange` với `formulas: true` để đọc công thức, `trace_formula` để lần ô nguồn;
giải thích theo từng phần, bằng lời thường, kèm ví dụ với số thật trong bảng.

## Hàm có trên WPS Spreadsheets 12.1.2 (đã thử)

`XLOOKUP`, `FILTER`, `UNIQUE`, `SORT`, `SEQUENCE`, `LET`, `LAMBDA`, `MAP`, `REDUCE`, `TEXTJOIN`, `TEXTSPLIT`, `HSTACK`,
`IFS`, `SWITCH`, `CONCAT`, `DATEDIF`, `YEARFRAC`, `EDATE`, `NETWORKDAYS.INTL`, `TEXT`. Tệp sẽ mở bằng Excel cũ (2016/2019)
thì tránh nhóm hàm mảng mới (`XLOOKUP`, `FILTER`, `UNIQUE`, `LET`, `LAMBDA`…): dùng `INDEX`/`MATCH`, `SUMPRODUCT`.

## Mẫu đã kiểm trên WPS thật

| Việc | Công thức (dòng 2) | Ghi chú |
|---|---|---|
| Đọc số tiền thành chữ | xem `read_skill_file {name: "cong-thuc-bang-tinh", path: "references/doc-so-thanh-chu.md"}` | không có hàm sẵn; công thức `LET`+`LAMBDA` dài, chép nguyên |
| Năm sinh từ CCCD (A2) | `=LET(s,TEXT(A2,"000000000000"),c,VALUE(MID(s,4,1)),IF(c>=8,1800,1900+100*INT(c/2))+VALUE(MID(s,5,2)))` | chữ số thứ 4 là mã thế kỷ + giới tính (0/1: 1900–1999, 2/3: 2000–2099, …); `TEXT` bù số 0 đầu nếu CCCD bị lưu thành số |
| Giới tính từ CCCD | `=IF(ISEVEN(VALUE(MID(TEXT(A2,"000000000000"),4,1))),"Nam","Nữ")` | mã chẵn = Nam, lẻ = Nữ |
| Tuổi (ngày sinh B2) | `=DATEDIF(B2,TODAY(),"Y")` | |
| Thâm niên "x năm y tháng" (ngày vào C2) | `=DATEDIF(C2,TODAY(),"Y")&" năm "&DATEDIF(C2,TODAY(),"YM")&" tháng"` | mốc khác hôm nay thì thay `TODAY()` bằng ô mốc `$H$1` |
| Ngày công, nghỉ thứ Bảy + Chủ nhật | `=NETWORKDAYS.INTL(B2,C2,"0000011",$K$2:$K$20)` | `$K$2:$K$20` = danh sách ngày lễ; chỉ nghỉ Chủ nhật: `"0000001"` |
| Đếm không trùng | `=COUNTA(UNIQUE(A2:A500))` | Excel cũ: `=SUMPRODUCT((A2:A500<>"")/COUNTIF(A2:A500,A2:A500&""))` |
| Tra cứu theo mã | `=XLOOKUP(A2,Ma!$A:$A,Ma!$B:$B,"không có")` | Excel cũ: `=IFERROR(INDEX(Ma!$B:$B,MATCH(A2,Ma!$A:$A,0)),"không có")` |
| Ngày dạng chữ "20/03/2015" → ngày | `=DATE(RIGHT(A2,4),MID(A2,4,2),LEFT(A2,2))` | chỉ khi `ISTEXT(A2)`; đặt định dạng `dd/mm/yyyy` cho cột kết quả |
| Hiển thị ngày kiểu Việt Nam | `=TEXT(B2,"dd/mm/yyyy")` | ra chữ; muốn giữ là ngày thì đổi định dạng ô thay vì dùng `TEXT` |
| Ghép họ và tên | `=TRIM(A2&" "&B2)` | |
| Tách tên (chữ cuối) | `=TRIM(RIGHT(SUBSTITUTE(TRIM(A2)," ",REPT(" ",100)),100))` | |

## Lưu ý

- Mã tỉnh trong CCCD (3 chữ số đầu) là mã cũ trước sáp nhập 2025; muốn ra tên tỉnh phải có bảng tra riêng, không đoán.
- Số tiền: tiếng Việt dùng `.` phân cách hàng nghìn — đổi bằng **định dạng ô** (`#,##0` theo vùng của máy), không đổi
  giá trị thành chữ.
- Không thay công thức có sẵn của người dùng khi chỉ được hỏi giải thích.
