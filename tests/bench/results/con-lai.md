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

**Đã làm xong (02/10/2026)**: `et.setConditionalFormat` + `et.listConditionalFormats` cho bridge
LibreOffice, đã vào `catalog/live-commands.json` (62 lệnh) và `livecommands_gen.go`. Bằng chứng chạy
thật trên LibreOffice: `python tests/live/test_live_libreoffice.py` phần Calc **137/137**, trong đó có
vòng `tên -> ghi -> đọc lại -> tên` cho **mọi** toán tử, ca gọi lại trên cùng vùng (phải THAY chứ không
cộng dồn), 5 ca sai tham số, và một lần `et.saveAs` ra `.xlsx` rồi tìm `<conditionalFormatting>` trong
gói để chắc chắn nó vào file thật chứ không chỉ nằm trong phiên.

**Còn lại**: bản C# (Excel/WPS) đã có lệnh cùng tên và cùng tham số (`Range.FormatConditions`) nhưng
**chưa chạy thử trên Excel/WPS thật** — máy này chỉ có LibreOffice. Khác biệt có chủ ý: LibreOffice gắn
style bằng TÊN CELL STYLE nên `styleName` có tác dụng, Excel gắn màu trực tiếp nên `styleName` chỉ có
tác dụng bên LibreOffice.

## 2. Chart — ĐÃ TRẢ LỜI XONG (02/10/2026)

`et.addChart` / `et.listCharts` đã có cho cả hai bridge và xanh trên LibreOffice thật. Bốn lượt Axiom ở
ngân sách 400k gọi nó 0–3 lần; lượt chạy lại Test 4 với ngân sách 1.000.000 gọi **16 lần** và ra
**5 chart**, hoàn thành cả bài.

**Kết luận**: đây không phải khoảng cách năng lực. Model vẫn luôn được chào `et.addChart`
(`catalog/live-commands.json` có nó) — chỉ là bốn lượt kia hết ngân sách trước khi tới Dashboard.

**Việc còn lại** (nếu muốn): chạy lại Test 1 và Test 3 với ngân sách rộng để xác nhận chúng cũng tới
được Dashboard, chứ mới có một bài làm chứng.

## 3. Gom nhiều lần ghi trong một vòng — ĐÃ LÀM XONG (02/10/2026)

Model hay gọi 5–10 `writeRange` liên tiếp trong **cùng một phản hồi**; mỗi lời gọi là một lần
LibreOffice tính lại cả sổ, và trên sổ 50.000 dòng mỗi lần tính lại mất hàng chục giây.

**Đã thử và ĐO ĐƯỢC LÀ SAI**: tạm tắt tính tự động rồi `calculateAll()` một lần — chậm hơn 2,3–4,0 lần
(12,4s → 28,0s; 32,6s → 130,6s). Đã hoàn tác, số đo ghi trong `write_range`. Chi tiết ở
[test2-gaps.md](test2-gaps.md).

**Đã làm xong**: `et.writeRanges {writes: [{range, values, sheet?}, ...]}` cho cả hai bridge, đã vào
catalog (63 lệnh). Kiểm hết tham số **trước khi ghi** — một vùng sai thì không vùng nào vào file (có
bài live kiểm đúng tính chất đó). Bằng chứng: `tests/live/test_live_libreoffice.py` phần Calc
**148/148**.

**Nói cho đúng điều được lợi**: lệnh này bớt **số vòng qua bridge** (mỗi vòng là HTTP + gate + một
lượt model trả lời), **không** hứa nhanh hơn về tính toán — đường tắt tính toán đã bị đo là chậm hơn
2,3–4,0 lần và đã hoàn tác.

## 4. Dữ liệu thô vẫn phải phát từng giá trị — ĐÃ CÓ LỆNH, NHƯNG KHÔNG LẤP ĐƯỢC BÀI (02/10/2026)

`et.fillRange` giải quyết phần công thức (chiếm 37–53% lời gọi ở Test 2/3/4), nhưng dữ liệu giả lập
(Raw_Data 1.044 dòng × 26 cột ở Test 1, 50.000 SKU ở Test 2) vẫn do model phát ra từng khối giá trị.

Claude Code đi đường khác hẳn: `gen_data.py` sinh CSV, rồi nạp vào. Bridge chưa có đường nạp từ file
vào tài liệu đang mở.

**Đã làm**: `et.importCsv {path, range?, sheet?, delimiter?, encoding?}` cho cả hai bridge, đã vào
catalog (64 lệnh). Không có `range` thì tạo sheet đặt tên theo tên file (trùng thì thêm số), có `range`
thì ghi vào ô góc đó; ô trông như số thành số, **ngày để nguyên chuỗi** (không đoán — đoán sai kiểu ngày
còn tệ hơn để người dùng tự chọn). Bằng chứng: `tests/live/test_live_libreoffice.py` phần Calc
**162/162**, gồm chữ có dấu, mã dạng chuỗi, ô trống, số âm, và ca `2025-01-15` phải ở lại là CHUỖI.

**Về bề mặt đường dẫn**: không phải bề mặt mới. Bridge **đã có** `et.open`, `et.saveAs`, `et.exportPdf`
và bộ tool file MCP — đều nhận đường dẫn do model chọn, và model vốn đã đọc được nội dung bất kỳ file
nào bằng `et.open` + `et.readRange`. Lệnh này gộp việc đó thành một lời gọi, không mở thêm quyền.

**Và nó KHÔNG lấp được khoảng cách token của bài benchmark** — nói rõ để lần sau khỏi tưởng nhầm: dữ liệu
giả lập (1.044 dòng × 26 cột) không nằm sẵn trong file nào. Claude Code tự **sinh** nó bằng `gen_data.py`
(agent đó có shell); agent của Axiom không có đường chạy mã, nên vẫn phải phát từng giá trị. Muốn lấp thì
phải làm một đường **sinh dữ liệu trong tài liệu** (ví dụ `RANDBETWEEN` + `INDEX` từ sheet `Lists`), và
đường đó có giá riêng: `RANDBETWEEN` là hàm **bay hơi**, mỗi lần tính lại là dữ liệu đổi, mà đề lại yêu cầu
"preserve the original raw data".

## 5. Chưa đối chiếu nội dung, mới so được cấu trúc — ĐÃ LÀM CHO TEST 4 (02/10/2026)

Cả bốn báo cáo hiện so: tên sheet, số ô công thức, số chart, số khối định dạng điều kiện, thời gian,
token, số tool call. **Chưa so giá trị**: KPI có đúng không, Checks có PASS/FAIL đúng không, đường găng
có đúng không, kết quả thống kê Monte Carlo có khớp không.

**Đã làm cho Test 4**: `tests/bench/compare_values.py` đọc **giá trị đã lưu** của cả hai file (`.ods`
đổi sang `.xlsx` trước để dùng cùng một đường đọc). Kết quả nguyên văn ở
[test4-doi-chieu-gia-tri.txt](test4-doi-chieu-gia-tri.txt). So **từng ô** giữa hai bên là vô nghĩa ở
bài này (Monte Carlo khác seed, hai bên còn thiết kế mô hình khác nhau) — nên so cái quyết định file có
dùng được không:

| | Axiom (`axiom-test4final.ods`) | Claude Code (`MonteCarlo_Risk_Model.xlsx`) |
|---|---|---|
| Công thức | 310.746 | 390.949 |
| Ô lỗi (`#REF!`, `#DIV/0!`, `Err:xxx`…) | **0** | **0** (không đọc được — xem dưới) |
| Công thức **có kết quả lưu trong file** | **310.746 / 310.746** | **0 / 390.949** |
| Sheet `Checks` đọc ra | **PASS=21, FAIL=0** | PASS=0, FAIL=0 (chưa có kết quả nào) |

Hai điều đáng chú ý, cả hai đều là **phát hiện mới**:

- File của Claude Code sinh bằng thư viện (openpyxl/xlsxwriter) nên **không có kết quả tính sẵn**: mở
  bằng Excel/Calc thì máy tự tính ra, nhưng đọc bằng thư viện (không có engine) thì ra rỗng. Hệ quả
  thật: **sheet `Checks` của nó không chứa một chữ PASS/FAIL nào** — ai chấm bằng cách đọc giá trị sẽ
  thấy 0/0. File của Axiom thì mang sẵn kết quả (LibreOffice lưu kèm giá trị).
- Kết luận này chỉ có được sau khi **sửa một lỗi đo của chính tôi**: lần chạy đầu, tôi chỉ đếm ô công
  thức khi ô đó có giá trị lưu sẵn, nên báo bên B "0 công thức" — nghe như bên B không có công thức
  nào. Sai ở chỗ đọc, không phải ở file.

**Đã làm nốt cho Test 1–3** (cùng lệnh, kết quả nguyên văn ở
[test1-3-doi-chieu-gia-tri.txt](test1-3-doi-chieu-gia-tri.txt)):

| Bài | Axiom: công thức / ô lỗi / Checks | Claude Code: công thức / ô lỗi / Checks |
|---|---|---|
| Test 1 | 24.130 / **0** / PASS=37 | 15.349 / 0 / PASS=40 |
| Test 2 | 852.207 / **8** / PASS=0 | 1.227.976 / 0\* / PASS=79 |
| Test 3 | 181.603 / **106** / PASS=0 | 494.340 / 0\* / PASS=0\* |
| Test 4 | 310.746 / **0** / PASS=21 | 390.949 / 0\* / PASS=0\* |

\* File của Claude Code sinh bằng thư viện nên **không có kết quả lưu sẵn**: ô lỗi (nếu có) và PASS/FAIL
chỉ hiện ra sau khi Excel/Calc tính lại. Nói "0 lỗi" ở cột đó là "không đọc được lỗi", không phải "không
có lỗi".

**Phát hiện mới, và là khoảng cách thật cần xử lý**: bài Test 2 và Test 3 của Axiom **ra khỏi tay agent
với ô lỗi nằm trong file** — Test 2 có 8 ô ngay trên sheet `Checks` (`#NAME?`, `#DIV/0!`, `#VALUE!`,
`#N/A`), Test 3 có 106 ô `#VALUE!` ở `Projects` và `Tasks`. Đây là loại lỗi mà **chính Axiom có lệnh bắt
được** (`et.checkRange` báo `error-values`), tức đây không phải thiếu công cụ mà là **lượt chạy đã không
soát trước khi trả bài** — và cả hai lượt đó đều dừng vì hết ngân sách. Đã đưa vào checklist của skill
`mo-hinh-tai-chinh` ("`et.checkRange`: `issueCount` = 0").

**Còn lại**: đối chiếu **từng ô** trên các sheet tất định (Inputs, Assumptions) — chỗ đó so được thật vì
không phụ thuộc ngẫu nhiên.

## 6. Ngân sách 400k cắt oan — ĐÃ ĐO ĐƯỢC VÀ ĐÃ SỬA MẶC ĐỊNH

Cả bốn lượt Axiom ở 400k đều dừng vì ngân sách. Đó là con số **tôi tự đặt** trong `run_prompt.py`,
không phải giới hạn sản phẩm (`MaxMaxTokens` là 1.000.000). Chạy lại Test 4 với 1.000.000:

- **Hoàn thành cả bài**: 8/8 sheet, 5 chart, `verified=true`, 24 phút.
- Tốn **375.806** token phải trả — **ít hơn** 404.021 của lượt bị cắt ở 400k, dù làm 3,7 lần số tool call.

Lý do: kích thước phản hồi của model dao động rất mạnh giữa các lần chạy (lượt bị cắt: 306.816 token ra
/ 79 vòng; lượt rộng: 141.269 / 293 vòng). Một ngân sách vừa khít sẽ cắt oan những lượt lẽ ra đã xong.

**Đã sửa**: `run_prompt.py` nay mặc định **1.000.000** (trước 400.000), kèm lý do và số đo ngay trong
chú thích để lần sau không ai hạ xuống lại. Vẫn truyền được số khác qua tham số thứ 8 khi muốn đo
"xong trong bao nhiêu token" — tách hai câu hỏi *có xong không* và *tốn bao nhiêu*.
