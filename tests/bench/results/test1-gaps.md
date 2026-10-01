# Test 1 — Enterprise FP&A: đối chiếu Claude Code với Axiom Core

Đề: `C:\Users\ThuyetMT\test\benchmark\test1.txt` (11 sheet, 1000+ giao dịch, P&L, Budget vs Actual,
Forecast, Scenarios, KPI, Dashboard có chart, Checks, và bài change test 5% → 10%).
Cả hai bên chạy trên **LibreOffice thật**, không mô phỏng bridge.

| | Claude Code | Axiom Core |
|---|---|---|
| Cách nói với LibreOffice | tự mở instance headless riêng, nói qua **UNO socket** (`--accept=socket,...,port=2002;urp;`) | nối vào **tài liệu đang mở** qua HTTP bridge của extension |
| Model | `ocg/deepseek-v4.1-flash` (qua `settings.proxy.json`) | `ocg/deepseek-v4.1-flash` |
| Thời gian | 02:19:02 → 03:32:52 (chết vì gateway 503), chạy tiếp, xong 04:42:01 | 04:51:56 → 05:02:44 |
| Tool call | 88 (Bash 50, Write 17, Edit 12, Read 9) | 37 |
| Token | vào 372.156 / ra **599.719** / cache đọc 36.012.544 | vào 290.663 / ra 110.200 |
| Kết thúc | hoàn thành | `token budget exceeded (400863 > 400000)` |

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

## Khoảng cách

| # | Khoảng cách | Bằng chứng | Vì sao |
|---|---|---|---|
| 1 | **Chi phí token tính theo từng ô** | Claude Code: 599.719 token ra cho cả bài và xong. Axiom: 110.200 token ra mới được ~1/5 `Raw_Data`, dừng ở 25 vòng | Axiom phải phát ra **từng giá trị ô** trong tham số tool; Claude Code phát ra **từng dòng code** rồi để script sinh dữ liệu. Cùng một bảng, chênh nhau hàng trăm lần |
| 2 | **Không có đường điền/khai báo, chỉ có ghi đủ** | Axiom ghi Raw_Data bằng 5 lần `writeRange` với mảng giá trị đầy đủ (`A42`, `A103`, …) | Bridge chưa có lệnh "viết một công thức rồi điền ra cả vùng", cũng chưa có nạp từ file. Cả hai bên đều cần khối lượng lớn; chỉ một bên có cách rẻ |
| 3 | **Trượt sang thí nghiệm nhỏ thay vì dựng bài** | 18/25 vòng ở `Assumptions` chỉ để thử công thức (`=DATE`, `=EOMONTH`, dấu `,`/`;`, đuổi chênh địa chỉ) | System prompt chưa giới hạn bước khảo sát, và các câu hỏi đó đáng lẽ trả lời được bằng một lần đọc mô tả tool |
| 4 | **Không có Dashboard/chart** | Đề yêu cầu "Use charts where appropriate"; trước phiên này bridge không có lệnh chart nào | Thiếu năng lực, không phải thiếu kế hoạch |
| 5 | **Không có change test** | Đề mục J yêu cầu đổi Revenue Growth 5% → 10% rồi **kiểm chứng lại**; Axiom dừng trước khi tới đó | Hệ quả của #1 và #2 |
| 6 | **Bị cắt vì ngân sách token của MỘT phản hồi thì mất cả lượt** | Hai lượt chạy trước cùng chết ở `finish_reason=length`, content rỗng | Vòng lặp agent chưa biết nhắc model viết nhỏ lại |

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

## Còn lại, chưa làm

- **#1 chưa được giải quyết triệt để.** `et.fillRange` gỡ được phần lớn chi phí cho các sheet
  công thức (P&L, Forecast, KPI, Checks đều là cùng một công thức lặp qua 12 tháng), nhưng `Raw_Data`
  cần **dữ liệu giả lập ngẫu nhiên** — điền công thức thì được (`RAND`, `CHOOSE`, `INDEX`), còn muốn
  đúng phân bố như đề tả thì vẫn phải phát từng giá trị hoặc nạp từ file.
- **Chưa có đường nạp dữ liệu từ file vào tài liệu đang mở** (`et.importCsv`). Đây là cách Claude Code
  đi (sinh CSV rồi nạp). Cần cân nhắc vì nó mở thêm một bề mặt: đường dẫn file do model chọn.
- **Chưa chạy lại Axiom trên Test 1 với `et.fillRange`** để đo mức cải thiện thật. Việc này phải làm
  trước khi nói bất cứ điều gì về hiệu quả của #4 — hiện chỉ mới là năng lực đã có và đã kiểm chứng
  riêng lẻ, chưa phải là kết quả trên đề.
