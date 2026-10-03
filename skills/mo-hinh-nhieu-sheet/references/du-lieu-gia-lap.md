# Sinh dữ liệu giả lập bằng công thức tất định

Dùng khi đề cần **dữ liệu giả lập lớn** (hàng nghìn → hàng chục nghìn dòng) mà không có file nguồn.
Không phát từng giá trị (50.000 dòng × 11 cột là hàng triệu token), không dùng `RAND`/`RANDBETWEEN`
(hàm bay hơi: mỗi lần tính lại là dữ liệu đổi, trong khi đề hay yêu cầu "giữ nguyên dữ liệu thô").

Mỗi cột = **một** lời gọi `et.fillRange` với một công thức chỉ phụ thuộc `ROW()` và một hằng salt.
Đo trên LibreOffice thật (04/10/2026): sheet 11 cột × 50.000 dòng của bài tồn kho dựng trong
**9,7 giây**, 0 ô lỗi, mọi tỷ lệ bất thường ra đúng đích, điền lại cùng công thức ra **y hệt** từng ô.

## Công thức gốc: số u trong (0,1)

```
((MOD(MOD((ROW()+S)*(2*(ROW()+S)+1),4194304)*(2*MOD((ROW()+S)*(2*(ROW()+S)+1),4194304)+1),4194304)+0.5)/4194304)
```

Thay **S** bằng một số trong bảng dưới — **mỗi chỗ cần một u độc lập dùng một S khác nhau**. Số học
nguyên, mọi trung gian < 2^46 nên ra giống nhau trên Excel lẫn LibreOffice; `+0.5` để u không bao giờ
bằng 0 (`NORMSINV(0)` là lỗi).

| k | S | k | S | k | S | k | S |
|---|---|---|---|---|---|---|---|
| 1 | 100003 | 6 | 600018 | 11 | 1100033 | 16 | 1600048 |
| 2 | 200006 | 7 | 700021 | 12 | 1200036 | 17 | 1700051 |
| 3 | 300009 | 8 | 800024 | 13 | 1300039 | 18 | 1800054 |
| 4 | 400012 | 9 | 900027 | 14 | 1400042 | 19 | 1900057 |
| 5 | 500015 | 10 | 1000030 | 15 | 1500045 | 20 | 2000060 |

**Không tự đặt salt nhỏ liền nhau (1, 2, 3…).** Đo được: hai cột salt 1 và 2 có CORREL ≈ 0 và rải đều
hoàn hảo — nhưng **49.999/50.000 ô** của cột này chính là cột kia dịch một dòng. Salt phải cách nhau
nhiều hơn số dòng; bảng trên cách nhau 100.003, dùng được tới ~100.000 dòng mỗi bảng.

## Biến u thành giá trị

Viết `u(k)` là công thức gốc với S của dòng k.

| Cần | Công thức |
|---|---|
| số nguyên trong [a, b] | `a+INT(u(k)*(b-a+1))` |
| chọn trong danh sách | `CHOOSE(1+INT(u(k)*n),"A","B",…)` (n mục) hoặc `INDEX(Lists!$A$2:$A$9,1+INT(u(k)*8))` |
| lệch phải kiểu giá/nhu cầu (lognormal) | `ROUND(EXP(LN(trung_vị)+độ_lệch*NORMSINV(u(k))),2)` — độ_lệch 0,5–1 |
| ô trống cố định ~p | `IF(u(k)<p,"",giá_trị)` — u dùng làm cờ phải khác u của giá trị |
| bản ghi trùng dòng trên ~p | ở cột A: `IF(AND(ROW()>2,u(k)<p),A1,"SKU-"&TEXT(ROW()-1,"000000"))` |
| giá trị bằng 0 ~p | `IF(u(k)<p,0,giá_trị)` |
| giá trị cực đại ~p | `giá_trị*IF(u(k)>1-p,25,1)` |
| giá trị âm ~p | `IF(u(k)<p,-1,1)*giá_trị` |
| nhánh hiếm (vd lead time dài ~p) | `IF(u(k)>1-p,60+INT(u(m)*61),3+INT(u(m)*28))` |

Mã định danh (SKU, mã dự án…) không cần u: `"SKU-"&TEXT(ROW()-1,"000000")`.

## Ví dụ đã kiểm: cột Daily Demand

0 với ~5%, cực đại ×25 với ~0,5%, còn lại lognormal trung vị 12 — điền `F2:F50001`:

```
=IF(((MOD(MOD((ROW()+800024)*(2*(ROW()+800024)+1),4194304)*(2*MOD((ROW()+800024)*(2*(ROW()+800024)+1),4194304)+1),4194304)+0.5)/4194304)<0.05,0,ROUND(EXP(LN(12)+0.7*NORMSINV(((MOD(MOD((ROW()+900027)*(2*(ROW()+900027)+1),4194304)*(2*MOD((ROW()+900027)*(2*(ROW()+900027)+1),4194304)+1),4194304)+0.5)/4194304)))*IF(((MOD(MOD((ROW()+1000030)*(2*(ROW()+1000030)+1),4194304)*(2*MOD((ROW()+1000030)*(2*(ROW()+1000030)+1),4194304)+1),4194304)+0.5)/4194304)>0.995,25,1),2))
```

Kết quả đo trên 50.000 dòng: 2.496 ô bằng 0 (đích 2.500), trung vị 11,52.

## Đi kèm

- **Không sort, không chèn/xoá dòng trong sheet dữ liệu thô**: giá trị tính từ `ROW()`, đổi chỗ dòng là
  sinh lại dữ liệu khác. Cần bảng đã sắp xếp/lọc thì dựng ở sheet khác bằng công thức đọc sang.
- Các cột phân tích (tồn an toàn, điểm đặt hàng…) vẫn là công thức đọc từ sheet thô — như mọi sheet.
- Ghi tỷ lệ bất thường (2% thiếu, 1% trùng…) vào sheet giả định nếu đề muốn chỉnh được; khi đó thay
  hằng `0.02` bằng ô giả định.
- Soát xong bằng `et.checkRange` và đếm thật: `COUNTBLANK` (thiếu), `COUNTIF(…,0)` (bằng 0),
  `SUMPRODUCT(--(A3:A50001=A2:A50000))` (trùng dòng trên) — so với tỷ lệ đã đặt.
