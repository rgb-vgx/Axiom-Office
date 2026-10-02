# Test 1 — Enterprise FP&A: đối chiếu Claude Code với Axiom Core

Đề: `C:\Users\ThuyetMT\test\benchmark\test1.txt` (11 sheet, 1000+ giao dịch, P&L, Budget vs Actual,
Forecast, Scenarios, KPI, Dashboard có chart, Checks, và bài change test 5% → 10%).
Cả hai bên chạy trên **LibreOffice thật**, không mô phỏng bridge.

| | Claude Code | Axiom Core (lượt tốt nhất, xem bảng ba lần chạy bên dưới) |
|---|---|---|
| Cách nói với LibreOffice | tự mở instance headless riêng, nói qua **UNO socket** (`--accept=socket,...,port=2002;urp;`) | nối vào **tài liệu đang mở** qua HTTP bridge của extension |
| Model | `ocg/deepseek-v4.1-flash` (qua `settings.proxy.json`) | `ocg/deepseek-v4.1-flash` |
| Thời gian | 02:19:02 → 03:32:52 (chết vì gateway 503), chạy tiếp, xong 04:42:01 | 06:29:45 → 06:52:57 (23 phút) |
| Tool call | 88 (Bash 50, Write 17, Edit 12, Read 9) | 315 (writeRange 100+, fillRange 106, readRange …, addSheet 10) |
| Token | vào 372.156 / ra **599.719** / cache đọc 36.012.544 | thô 4.685.423 vào / 233.531 ra; **4.578.176 đọc từ cache (98%)** |
| Kết thúc | hoàn thành | `agent stopped after 100 rounds` |
| Kết quả | 11/11 sheet, 15.349 ô công thức, 5 chart, 16 khối CF, change test | 9/11 sheet có nội dung thật; **Dashboard và Checks trống** |

## Claude Code đã làm gì

Nguồn: `tests/bench/results/test1-claude-code.summary.txt`, transcript
`~/.claude/projects/C--Users-ThuyetMT-test-bench-test1/2cff3e4c-….jsonl`.

1. Viết 5 script `probe*.py` để **dò API trước khi làm** — cách ly dấu phân cách tham số công thức,
   cách tạo sheet, cách đặt định dạng. Đọc `.log` sau mỗi lần chạy.
2. `gen_data.py` sinh `raw_data.csv` (88,6 KB) — dữ liệu ra từ **script**, không phải từ token của model.
3. `build_model.py` (75,5 KB, **sửa 11 lần**) dựng cả 11 sheet qua UNO.
4. `postprocess.py` (sửa 4 lần) — vì bản LibreOffice này từ chối `XSheetConditionalEntries.addNew`,
   nó **viết lại OOXML** của file .xlsx để gắn conditional formatting và chart, rồi mở lại kiểm tra.
5. `verify.py`, `validate.py`, `diag.py`, `final_check.py` — tự soát; chính các bước này bắt được 3 lỗi
   của chính nó (công thức budget tự tham chiếu, lệch một cột ở tổng scenario, KPI chia sai sheet).
6. `change_test.py` — đổi `Assumptions!B18` 0,05 → 0,10 trên file đã lưu, mở lại, đọc lại toàn bộ:
   23/35 chỉ số dịch chuyển, 37/37 check vẫn PASS. Xuất `Enterprise_FPA_Model_Growth_10pct.xlsx` +
   `CHANGE_TEST_REPORT.md`.

Kết quả kiểm chứng bằng openpyxl/zipfile trên `Enterprise_FPA_Model.xlsx`:

```
sheets: Raw_Data, Assumptions, Revenue, COGS, PnL, Budget_vs_Actual,
        Forecast, Scenarios, KPI, Dashboard, Checks        (11/11 đúng tên)
tổng ô công thức: 15.349
chart parts: 5      conditionalFormatting blocks: 16      dataValidation: 2
```

## Axiom Core đã làm gì

Nguồn: `tests/bench/results/test1-axiom-calls.txt`, SSE log `%TEMP%\axiom-test1.log`,
run `r_e82a67a129d65ae1b98dec8ef70ced45`.

- **Tạo đủ 11 sheet đúng tên** ngay từ vòng 2–14, bằng `et.renameSheet` + 10 × `et.addSheet`. Đây là
  các lệnh vừa được thêm trong phiên này; không có chúng thì bài này bất khả thi (xem
  `test1-axiom-first-attempt.txt` nếu cần bản chạy trước đó, model đoán ~30 tên lệnh rồi bỏ cuộc).
- Rồi **tiêu 18 vòng tiếp theo cho thí nghiệm công thức nhỏ trong sheet `Assumptions`**: thử `=DATE`,
  `=EOMONTH`, dấu phân cách `,`/`;`, tham chiếu tương đối/tuyệt đối, rồi đuổi theo một chênh lệch địa
  chỉ mà chính nó tạo ra (`readRange("A16:C27")` trả về hàng đầu của khối 80 ô nó vừa ghi ở A1 — khối
  đó chỉ cao 16 hàng, nên "lệch 2 hàng" là do nó hình dùng sai kích thước khối, không phải bridge sai;
  đã kiểm chứng lại bằng `test_live_libreoffice.py`, 71/71 xanh).
- Vòng 33–37 mới bắt đầu ghi `Raw_Data`: A1 (header), A42, A103, A167, A229 — **ghi theo khối** đúng
  như thiết kế, mỗi khối 60–70 hàng.
- Hết ngân sách token ở đây. Chưa có công thức P&L, chưa KPI, chưa Dashboard, chưa Checks, chưa change test.

## Một lượt chạy lại KHÔNG dùng được làm bằng chứng

Sau khi thêm `et.fillRange`, tôi chạy lại Axiom trên cùng đề (05:1x–05:2x). Kết quả **không dùng được**:
LibreOffice mất tài liệu giữa chừng, 90 trong 94 tool call trả về cùng một lỗi
`InvalidOperationException: no active spreadsheet`.

Nguyên nhân là **va chạm môi trường**, không phải lỗi agent: phiên Claude Code cho Test 2 đang chạy song
song và tự mở/đóng instance LibreOffice của nó. Từ đây về sau hai bên phải chạy **tuần tự**.

Nhưng lượt hỏng đó phơi ra một lỗi thật của vòng lặp agent: model gọi **cùng một lệnh với cùng một lỗi
90 lần liên tiếp** và đốt hết 1.000.088 token vào đó, không tự biết dừng. Đã sửa (xem #7 bên dưới).

## Chạy lại với ngân sách rộng: Axiom dựng XONG cả 11 sheet

Ngân sách 400k là con số tôi tự đặt. Chạy lại Test 1 với 1.000.000 (12:27:06 → 13:13:56, 47 phút):

| | Axiom @ 400k | Axiom @ 1M | Claude Code |
|---|---|---|---|
| Vòng / tool call | 100 / 315 | **158 / 615** | — / 88 |
| Sheet có nội dung | 9/11 | **11/11** | 11/11 |
| Ô công thức | (chưa đo) | **24.130** | 15.349 |
| Chart | 0 | **5** | 5 |
| Định dạng điều kiện | 0 | 0 | 16 khối |
| Bài change test | không | **không** | có |
| Kết thúc | hết vòng | hết ngân sách (1.176.802 > 1.000.000) | hoàn thành |

Từng sheet trong file lưu (`result/axiom-test1wide.ods`):

```
Raw_Data        23.128 cong thuc, 1007 dong     KPI              74 cong thuc,  44 dong
Assumptions          3 cong thuc,   21 dong     Dashboard       138 cong thuc,  96 dong
Revenue             87 cong thuc,   32 dong     Checks          114 cong thuc,  43 dong
COGS                65 cong thuc,   16 dong     Budget_vs_Actual 104 cong thuc, 26 dong
PnL                179 cong thuc,   16 dong     Forecast        169 cong thuc,  16 dong
                                                Scenarios        69 cong thuc,  19 dong
```

Điều này lật lại kết luận ở bảng ba lần chạy bên dưới: **Dashboard và Checks trống ở lượt 400k không
phải vì Axiom không làm được, mà vì hết ngân sách trước khi tới.** Với ngân sách đủ, Axiom dựng cả 11
sheet, ra 5 chart, và số ô công thức còn nhiều hơn Claude Code (24.130 so với 15.349).

Vẫn còn hai thứ Claude Code làm được mà Axiom chưa: **định dạng điều kiện** (năng lực bridge còn thiếu,
xem `con-lai.md`) và **bài change test** (lượt rộng vẫn hết ngân sách trước khi tới đó).

## Ba lần chạy Axiom, cùng một đề

Cùng model, cùng LibreOffice thật, cùng đề. Chỉ khác cách tính ngân sách token và các lệnh đã có.

| Lần | Ngân sách | Token tính thế nào | Vòng | Tool call | Token thô | Đọc từ cache | Kết quả |
|---|---|---:|---:|---:|---:|---:|---|
| 1 | 400k | cả token cache | 25 | 37 | 400.863 | 253.440 (63%) | 11 tên sheet + Raw_Data dở |
| 2 | 1M | cả token cache | 64 | 67 | 1.005.301 | 806.400 (80%) | + Raw_Data 1225 dòng, Assumptions dở |
| **3** | **400k** | **chỉ token phải trả** | **100** | **315** | **4.918.954** | **4.578.176 (93%)** | **9/11 sheet dựng xong** |

Lần 3 dừng vì chạm trần **số vòng** (100), không phải vì tiền. Cùng ngân sách 400k, số tool call gấp
**4,7 lần** lần 1. Nguyên nhân duy nhất khác nhau: token đọc từ cache (98% ở lần 3) không bị tính vào
ngân sách nữa.

Tài liệu lần 3 để lại (đọc lại bằng `et.checkRange` + `et.readRange` trên chính tài liệu đang mở,
`tests/bench/results/test1-axiom-output.txt`):

```
Raw_Data          1044 x 26   Trans_ID, Date, Region, Product, Customer_Segment, Revenue, COGS, ...
Assumptions         53 x 4    "Enterprise FP&A — Assumptions & Drivers …"
Revenue             31 x 5    Month | Month_Start | Revenue_$   (Jan-24 …)
COGS                31 x 8
PnL                 17 x 27   "Profit & Loss — monthly, actual FY2024 and forecast FY2025"
Budget_vs_Actual    42 x 19   "Budget vs Actual — monthly and YTD comparison…"
Forecast            27 x 13   "12-Month Forecast (FY2025) — driven entirely by Assumptions"
Scenarios           23 x 13   "Scenario Analysis — FY2025 outcomes…"
KPI                 58 x 5    "KPI Dashboard — financial, transaction and dimensional metrics"
Dashboard            1 x 1    TRỐNG
Checks               1 x 1    TRỐNG
```

So với Claude Code (11/11 sheet, 15.349 ô công thức, 5 chart, change test): Axiom đã dựng được 9/11
sheet có tiêu đề và cấu trúc thật, còn thiếu đúng **Dashboard** (chưa gọi `et.addChart` lần nào) và
**Checks**, và chưa tới bài change test.

## Khoảng cách
|---|---|---|---|
| 1 | **Chi phí token tính theo từng ô** | Claude Code: 599.719 token ra cho cả bài và xong. Axiom: 110.200 token ra mới được ~1/5 `Raw_Data`, dừng ở 25 vòng | Axiom phải phát ra **từng giá trị ô** trong tham số tool; Claude Code phát ra **từng dòng code** rồi để script sinh dữ liệu. Cùng một bảng, chênh nhau hàng trăm lần |
| 2 | **Không có đường điền/khai báo, chỉ có ghi đủ** | Axiom ghi Raw_Data bằng 5 lần `writeRange` với mảng giá trị đầy đủ (`A42`, `A103`, …) | Bridge chưa có lệnh "viết một công thức rồi điền ra cả vùng", cũng chưa có nạp từ file. Cả hai bên đều cần khối lượng lớn; chỉ một bên có cách rẻ |
| 3 | **Trượt sang thí nghiệm nhỏ thay vì dựng bài** | 18/25 vòng ở `Assumptions` chỉ để thử công thức (`=DATE`, `=EOMONTH`, dấu `,`/`;`, đuổi chênh địa chỉ) | System prompt chưa giới hạn bước khảo sát, và các câu hỏi đó đáng lẽ trả lời được bằng một lần đọc mô tả tool |
| 4 | **Không có Dashboard/chart** | Đề yêu cầu "Use charts where appropriate"; trước phiên này bridge không có lệnh chart nào | Thiếu năng lực, không phải thiếu kế hoạch |
| 5 | **Không có change test** | Đề mục J yêu cầu đổi Revenue Growth 5% → 10% rồi **kiểm chứng lại**; Axiom dừng trước khi tới đó | Hệ quả của #1 và #2 |
| 6 | **Bị cắt vì ngân sách token của MỘT phản hồi thì mất cả lượt** | Hai lượt chạy trước cùng chết ở `finish_reason=length`, content rỗng | Vòng lặp agent chưa biết nhắc model viết nhỏ lại |
| 7 | **Không có phanh khi tool hỏng lặp lại** | Lượt chạy lại: 90/94 tool call là **cùng một lệnh với cùng một lỗi** (`no active spreadsheet`), đốt 1.000.088 token | Vòng lặp agent chỉ đếm vòng và token, không nhận ra "lặp y hệt" |

Điểm **không** phải khoảng cách: `et.addSheet`/`et.renameSheet` đã đủ dùng — model tạo đúng 11 sheet
một lần, không đoán tên lệnh lần nào.

## Cải tiến đã làm để thu hẹp

Làm trong phiên này, mỗi cái đều có test chạy thật:

1. **`et.addSheet` / `et.renameSheet`** cho cả hai bridge (commit `ddf1c31`) — điều kiện cần của bài
   nhiều sheet. Kiểm chứng: `test_live_libreoffice.py` trên LibreOffice thật.
2. **Lỗi "lệnh không có" liệt kê ngay các lệnh hợp lệ** (commit `ddf1c31`) — lượt chạy trước đoán ~30
   tên lệnh tự nghĩ ra; lượt này chỉ 3 lần sai rồi thôi.
3. **`et.addChart` / `et.listCharts`** cho cả hai bridge (commit `94208c9`) — khoảng cách #4. Kèm phát
   hiện đo được: thuộc tính đặt tiêu đề là `HasMainTitle` chứ không phải `HasTitle` (bản LibreOffice
   này không có `HasTitle`, đó là lý do lệnh chart đầu tiên lỗi khi chạy thật); `column`/`bar` cùng là
   `BarDiagram` nên phải đọc `Vertical` mới phân biệt được.
4. **`et.fillRange`** cho cả hai bridge (commit `bd8b94e`) — khoảng cách #1 và #2, cái đắt nhất: viết
   một công thức vào ô góc rồi điền ra cả vùng, tham chiếu tương đối tự dịch. Đo bằng probe UNO riêng:
   `=$D$1+ROW()*B1 → *B2 → *B3`, `=F2*2 → =G2*2 → =H2*2`, và điền được cả khối 2D.
5. **Cứu lượt chạy bị cắt vì hết ngân sách token của một phản hồi** (commit `94208c9`) — khoảng cách #6.
6. **Luật khảo sát có mức trong system prompt** (commit `94208c9`) — khoảng cách #3.
7. **Phanh khi tool hỏng lặp lại** (`AgentOptions.MaxIdenticalFailures`, mặc định 3 vòng) — khoảng
   cách #7. Một vòng chỉ toàn lỗi mà chu ký (tên tool + đối số + kết quả) y hệt vòng trước, lặp 3 lần
   thì lượt chạy dừng với thông báo nói rõ, thay vì để model đốt hết ngân sách. Lỗi **đổi** mỗi vòng
   thì không cắt — model có thể đang thử cách khác (có test riêng cho cả hai chiều).

8. **Ngân sách token không tính token đọc từ cache** (`AgentResult.BillableTokens`, commit `f243789`)
   — khoảng cách #1, và là cái đắt nhất. Đo được: cache đã chạy tốt sẵn (93–98% trúng), nhưng ngân sách
   đếm cả phần gần như miễn phí đó, nên lượt chạy dừng đúng lúc nhà cung cấp đang phục vụ từ cache.
   Cùng ngân sách 400k: **67 → 315 tool call**, 9/11 sheet thay vì 5/11.

## Còn lại, chưa làm

- **Dashboard và Checks trống.** Axiom chưa gọi `et.addChart` lần nào trong lượt 3 — năng lực đã có
  (`et.addChart` xanh trên LibreOffice thật), nhưng nó không tự tới đó. Nghi vấn: mô tả tool chưa làm
  nổi bật việc Dashboard cần chart, hoặc model lo phần số trước rồi hết vòng. Cần thử lại khi trần
  vòng không còn là nút thắt.
- **Trần 100 vòng là nút thắt mới.** Lượt 3 dừng vì `MaxRounds`, không vì tiền. `MaxMaxRounds` đang là
  1000 nên nới được, nhưng nới trần là chữa triệu chứng: câu hỏi thật là vì sao 315 tool call vẫn chưa
  xong một workbook 11 sheet — mỗi sheet đang tốn ~30 lời gọi.
- **`Raw_Data` vẫn phải phát từng giá trị.** Lượt 3 có 1044 dòng × 26 cột. Điền bằng công thức
  (`RAND`, `CHOOSE`, `INDEX`) chưa được dùng cho phần dữ liệu thô — model chọn phát từng khối giá trị.
- **Chưa có đường nạp dữ liệu từ file vào tài liệu đang mở** (`et.importCsv`). Đây là cách Claude Code
  đi (sinh CSV bằng script rồi nạp). Cần cân nhắc vì mở thêm bề mặt: đường dẫn file do model chọn.
- **Chưa làm bài change test** (đổi Revenue Growth 5% → 10% rồi kiểm chứng) — hệ quả trực tiếp của việc
  Checks còn trống.
