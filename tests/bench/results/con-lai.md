# Việc còn lại sau đợt đối chiếu bốn bài

Ghi lại để lần sau tiếp, xếp theo mức chặn việc. Mỗi mục nói rõ **đã đo được gì** và **cần làm gì**,
để không phải dò lại từ đầu.

## 1. Định dạng điều kiện — ĐÃ TÌM RA ĐƯỜNG, probe cũ kết luận SAI (02/10/2026)

**Kết luận cũ** ("không có đường nào tạo được entry") là **sai**, và sai ở chỗ đo: hai probe cũ chỉ
thử tạo entry từ `doc.createInstance`, `ServiceManager`, `uno.createUnoStruct`, và
`sheet.ConditionalFormats.createInstance()` — tức là từ **vật chứa**, chưa bao giờ hỏi **đối tượng
theo vùng**. Chúng cũng chỉ kiểm "lời gọi có ném lỗi không", chứ không kiểm cái thật sự vào file.

**Đã chạy được thật** — `tests/bench/probe_cf3.py`, LibreOffice 26.8.0.3, kết quả nguyên văn ở
[probe-conditional-format-3.txt](probe-conditional-format-3.txt) (10/10 OK):

1. `doc.createInstance("com.sun.star.sheet.SheetCellRanges")` + `addRangeAddress` → tập vùng.
   (`ServiceManager` trả `None`; `sheet.getCellRangesByName` **không có** trong pyuno;
   `sheet.Ranges`/`getRanges()` chỉ là tuple Python.)
2. `formats.createByRange(tập vùng)` → **trả về số ID**, không phải đối tượng. Đây là chỗ dễ tưởng
   "hỏng" nhất.
3. `formats.getConditionalFormats()` → dãy đối tượng theo vùng. Mỗi đối tượng **chính là dãy entry**
   (`getByIndex`/`getCount`/`removeByIndex`) và có `createEntry(nIndex, nType)`.
4. `createEntry(0, CONDITION)` **tạo entry thật** (Count 0 → 1) nhưng **pyuno trả về `None`** — phải
   lấy lại bằng `getByIndex(0)`. Đây là lý do thứ hai khiến probe cũ tưởng không làm được.
5. Đặt `Operator`/`Formula1`/`StyleName` trên entry đó.

**Kiểm chứng bằng hai định dạng độc lập** (không tin vào việc API không ném lỗi):
lưu ra `.ods` → `<style:map>=1`, `apply-style-name=1` (probe cũ đo được `0`);
xuất sang `.xlsx` → `<conditionalFormatting>=1`, `<dxf>=1`.

**Hệ quả**: không cần đi đường OOXML (saveAs → sửa gói → mở lại) như Claude Code. Làm được ngay
**trên tài liệu đang mở**.

**Cần làm**: đưa vào bridge thành `et.setConditionalFormat` / `et.listConditionalFormats` (và bản C#
cho Excel/WPS nếu muốn parity), thêm vào `catalog/live-commands.json` để model thấy.

## 2. Chart — ĐÃ TRẢ LỜI XONG (02/10/2026)

`et.addChart` / `et.listCharts` đã có cho cả hai bridge và xanh trên LibreOffice thật. Bốn lượt Axiom ở
ngân sách 400k gọi nó 0–3 lần; lượt chạy lại Test 4 với ngân sách 1.000.000 gọi **16 lần** và ra
**5 chart**, hoàn thành cả bài.

**Kết luận**: đây không phải khoảng cách năng lực. Model vẫn luôn được chào `et.addChart`
(`catalog/live-commands.json` có nó) — chỉ là bốn lượt kia hết ngân sách trước khi tới Dashboard.

**Việc còn lại** (nếu muốn): chạy lại Test 1 và Test 3 với ngân sách rộng để xác nhận chúng cũng tới
được Dashboard, chứ mới có một bài làm chứng.

## 3. Gom nhiều lần ghi trong một vòng

Model hay gọi 5–10 `writeRange` liên tiếp trong **cùng một phản hồi**; mỗi lời gọi là một lần
LibreOffice tính lại cả sổ, và trên sổ 50.000 dòng mỗi lần tính lại mất hàng chục giây.

**Đã thử và ĐO ĐƯỢC LÀ SAI**: tạm tắt tính tự động rồi `calculateAll()` một lần — chậm hơn 2,3–4,0 lần
(12,4s → 28,0s; 32,6s → 130,6s). Đã hoàn tác, số đo ghi trong `write_range`. Chi tiết ở
[test2-gaps.md](test2-gaps.md).

**Cần làm**: một lệnh ghi được **nhiều vùng** trong một lời gọi, ví dụ
`et.writeRanges {writes: [{range, values, sheet}, ...]}` — một lần tính lại cho cả lô.

## 4. Dữ liệu thô vẫn phải phát từng giá trị

`et.fillRange` giải quyết phần công thức (chiếm 37–53% lời gọi ở Test 2/3/4), nhưng dữ liệu giả lập
(Raw_Data 1.044 dòng × 26 cột ở Test 1, 50.000 SKU ở Test 2) vẫn do model phát ra từng khối giá trị.

Claude Code đi đường khác hẳn: `gen_data.py` sinh CSV, rồi nạp vào. Bridge chưa có đường nạp từ file
vào tài liệu đang mở.

**Cần làm**: cân nhắc `et.importCsv {path, range, sheet?}`. Lưu ý đây **mở thêm một bề mặt**: đường dẫn
file do model chọn, nên cần luật rõ về phạm vi đường dẫn được phép.

## 5. Chưa đối chiếu nội dung, mới so được cấu trúc

Cả bốn báo cáo hiện so: tên sheet, số ô công thức, số chart, số khối định dạng điều kiện, thời gian,
token, số tool call. **Chưa so giá trị**: KPI có đúng không, Checks có PASS/FAIL đúng không, đường găng
có đúng không, kết quả thống kê Monte Carlo có khớp không.

**Cần làm**: mở song song file của hai bên bằng openpyxl/pandas, đối chiếu từng ô ở các sheet kết quả.

## 6. Ngân sách 400k cắt oan — ĐÃ ĐO ĐƯỢC

Cả bốn lượt Axiom ở 400k đều dừng vì ngân sách. Đó là con số **tôi tự đặt** trong `run_prompt.py`,
không phải giới hạn sản phẩm (`MaxMaxTokens` là 1.000.000). Chạy lại Test 4 với 1.000.000:

- **Hoàn thành cả bài**: 8/8 sheet, 5 chart, `verified=true`, 24 phút.
- Tốn **375.806** token phải trả — **ít hơn** 404.021 của lượt bị cắt ở 400k, dù làm 3,7 lần số tool call.

Lý do: kích thước phản hồi của model dao động rất mạnh giữa các lần chạy (lượt bị cắt: 306.816 token ra
/ 79 vòng; lượt rộng: 141.269 / 293 vòng). Một ngân sách vừa khít sẽ cắt oan những lượt lẽ ra đã xong.

**Cần làm**: đặt lại ngân sách mặc định cho các lần đo tiếp theo (1.000.000, hoặc bỏ hẳn trần khi đo
"có xong được không"), và tách hai câu hỏi ra: *có xong không* và *xong trong bao nhiêu token*.
