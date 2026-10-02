# Việc còn lại sau đợt đối chiếu bốn bài

Ghi lại để lần sau tiếp, xếp theo mức chặn việc. Mỗi mục nói rõ **đã đo được gì** và **cần làm gì**,
để không phải dò lại từ đầu.

## 1. Định dạng điều kiện — bridge không có đường nào làm được

Đây là khoảng cách năng lực chung của cả bốn bài.

**Đã đo** (probe UNO riêng, `test/probe_cf.py` và `test/probe_cf2.py`, 02/10/2026):

- `range.ConditionalFormat` và `sheet.ConditionalFormats` **đều tồn tại**.
- `sheet.ConditionalFormats` có `createByRange(RangeAddress)`, `removeByID`, `getConditionalFormats`.
- `range.ConditionalFormat` là `XSheetConditionalEntries` với `addNew`, `getByIndex`, `clear`.
- Tạo đối tượng entry thì **không có đường nào chạy**:
  - `doc.createInstance("com.sun.star.sheet.TableConditionalEntry")` → trả `None`.
  - `ctx.ServiceManager.createInstanceWithContext(...)` → trả `None`.
  - `uno.createUnoStruct("com.sun.star.sheet.TableConditionalEntry")` → tạo được, nhưng `addNew((entry,))`
    ném `RuntimeException: Couldn't convert ... 'getTypes'`.
  - `sheet.ConditionalFormats.createInstance()` → `AttributeError` (không có hàm này).
  - Liệt kê service có chữ "Conditional" → **rỗng**.
- Kết quả trong file lưu: `<style:map>=0 | table:condition=0`.

**Khớp với Claude Code**: nó ghi trong báo cáo của mình rằng bản LibreOffice này từ chối
`XSheetConditionalEntries.addNew`, và nó **viết lại gói OOXML** của file .xlsx để gắn CF (16 khối ở
Test 1, 34 khối ở Test 2). Đó là hai lần đo độc lập cùng ra một kết luận.

**Cần làm**: chọn một trong hai đường **trước khi viết code**:
- (a) Đi đường OOXML: ghi thẳng `<conditionalFormatting>` vào part `xl/worksheets/sheetN.xml` của gói
  (giống Claude Code). Dùng được engine OOXML đã có trong `core-go/internal/ooxml` — nhưng bridge
  extension đang chạy trong LibreOffice trên **tài liệu đang mở**, không phải trên file; phải
  `et.saveAs` ra file rồi sửa gói rồi mở lại, tức là đổi hẳn luồng làm việc.
- (b) Dùng **dispatch** (`com.sun.star.frame.DispatchHelper` với URL `.uno:ConditionalFormatDialog`),
  nhưng đường này mở hộp thoại — không hợp với agent.

Đường (a) là đường Claude Code chọn và nó đã chạy được thật, nên đây là hướng đáng thử trước.

## 2. Chart có sẵn nhưng không lượt Axiom nào dùng

`et.addChart` / `et.listCharts` đã có cho cả hai bridge và **xanh trên LibreOffice thật**
(`test_live_libreoffice.py --apps calc` 71/71, gồm cả mục đặt đúng kiểu và đọc lại được kiểu). Nhưng:

| Bài | Claude Code | Axiom |
|---|---|---|
| 1 | 5 chart | 0 (`addChart` không được gọi lần nào) |
| 2 | 4 chart | 3 lời gọi (ở sát cuối lượt) |
| 3 | 4 chart | 0 |
| 4 | 4 chart | 0 |

**Chưa phân biệt được** hai khả năng: model không biết dùng, hay nó hết ngân sách trước khi tới
Dashboard. Cả bốn lượt đều dừng vì ngân sách.

**Cần làm**: một lần chạy **không bị cắt** (ngân sách lớn hơn hẳn, hoặc chỉ giao riêng phần Dashboard)
để trả lời. Nếu ra "không biết dùng" thì sửa mô tả tool hoặc thêm luật vào system prompt.

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

## 6. Ngân sách 400k là nút thắt của MỌI lượt

Cả bốn lượt Axiom đều dừng vì ngân sách (Test 1 lượt tốt nhất dừng vì trần 100 vòng). Đây là con số
tôi tự đặt trong `run_prompt.py`, không phải giới hạn của sản phẩm (`MaxMaxTokens` là 1.000.000).

**Cần làm**: chạy lại ít nhất một bài với ngân sách rộng để biết Axiom **có thể** xong hay không, tách
khỏi câu hỏi "xong trong bao nhiêu token".
