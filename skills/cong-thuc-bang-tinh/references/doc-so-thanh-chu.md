# Đọc số tiền thành chữ tiếng Việt bằng công thức

WPS Spreadsheets và Excel đều **không có hàm** đọc số thành chữ tiếng Việt. Công thức dưới đây dùng `LET` + `LAMBDA`
(có trên WPS Spreadsheets 12.1.2 và Excel 365/2021) và đã thử trên WPS 12.1.2 thật với 20 số, gồm các ca khó: 0,
15 ("mười lăm"), 21 ("hai mươi mốt"), 24 ("hai mươi tư"), 105 ("một trăm linh năm"), 1.001 ("một nghìn không trăm linh
một"), 1.000.500 ("một triệu năm trăm"), 2.024.000.000 ("hai tỷ không trăm hai mươi tư triệu"), 999.999.999.999.

Số cần đọc ở `A2` (đổi `A2` thành ô của bạn; điền xuống cả cột bằng `et.fillRange` thì tham chiếu tự dịch):

```
=LET(n,INT(A2),d,{"không","một","hai","ba","bốn","năm","sáu","bảy","tám","chín"},g,LAMBDA(x,lead,LET(h,INT(x/100),t,INT(MOD(x,100)/10),u,MOD(x,10),hs,IF(AND(lead,h=0),"",INDEX(d,h+1)&" trăm"),ts,IF(t=0,IF(AND(u>0,OR(NOT(lead),h>0)),"linh",""),IF(t=1,"mười",INDEX(d,t+1)&" mươi")),us,IF(u=0,"",IF(AND(u=1,t>=2),"mốt",IF(AND(u=4,t>=2),"tư",IF(AND(u=5,t>=1),"lăm",INDEX(d,u+1))))),TRIM(hs&" "&ts&" "&us))),b,INT(n/1000000000),m,INT(MOD(n,1000000000)/1000000),k,INT(MOD(n,1000000)/1000),r,MOD(n,1000),s,TRIM(IF(b>0,g(b,TRUE)&" tỷ","")&" "&IF(m>0,g(m,b=0)&" triệu","")&" "&IF(k>0,g(k,AND(b=0,m=0))&" nghìn","")&" "&IF(r>0,g(r,n<1000),"")),IF(n=0,"Không",UPPER(LEFT(s))&MID(s,2,999))&" đồng")
```

- Đọc phần nguyên (`INT`), tối đa 999 tỷ; số âm, số lẻ không đọc.
- Đổi đơn vị: thay `" đồng"` ở cuối (ví dụ `" đồng chẵn"`, `" USD"`). Miền Nam đọc "ngàn": thay `" nghìn"`.
- Cần giữ cách đọc "bốn" thay cho "tư" (24 → "hai mươi bốn"): bỏ nhánh `IF(AND(u=4,t>=2),"tư", …)`.
- Bản Excel cũ (2016/2019) không có `LET`/`LAMBDA`: không dùng được công thức này. Khi đó ghi **giá trị** chữ (tự đọc số
  rồi `et.writeRange`) và nói rõ với người dùng là sửa số tiền thì phải đọc lại.
