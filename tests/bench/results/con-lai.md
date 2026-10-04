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
`mo-hinh-nhieu-sheet` ("`et.checkRange`: `issueCount` = 0").

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

## 7. Roadmap hiệu năng — tư vấn ngoài + chỉnh cho repo (02/10/2026)

Đã xin tư vấn từ AI ngoài về "hướng cải tiến hiệu năng" (toàn văn câu hỏi + trả lời lưu ở ngoài repo —
hỏi gì thì xem transcript phiên 02/10/2026). AI đề xuất 3 ưu tiên theo thứ tự: (1) plan-then-fill,
(2) vertical slice + checkpoint, (3) sinh dữ liệu tất định trong bridge. Đánh giá của tôi:

- Thứ tự đúng, danh sách NOT-do đúng hết (nhất là 3 cái repo đã đổ máu: tắt auto-calc chậm 2,3–4×,
  chạy song song mất tài liệu, phá prompt ổn định mất cache 93–98%).
- Nhưng bê nguyên thì over-engineering: manifest + state machine Go (`PlannedOperation`,
  PENDING→VALIDATED) là vội — model flash viết plan JSON dài dễ stale, plan nằm trong context tự
  thành gánh token. Làm kỷ luật plan bằng chữ (skill + prompt) trước, chưa cần struct Go.
- Response-shape limit bị AI xếp thấp oan: `turns` trong `model/agent.go` append nguyên xi mọi tool
  result suốt lượt, Test 4 rộng có 174 `readRange`. Đây mới là món rẻ nhất sau plan.
- Trước native generator (Python UNO + C# COM, 2 nền tảng) còn một nấc rẻ hơn: sinh dữ liệu bằng
  công thức tất định (`fillRange` + `ROW()`/`MOD()`/`INDEX`), không sửa bridge nào.

Thứ tự làm đã chỉnh cho repo (rẻ trước, đắt sau; mỗi bước đo A/B rồi mới sang bước sau):

**Bước 0 — metric A/B trong harness (ĐÃ LÀM, 02/10/2026)**: `run_prompt.py` in thêm dòng `METRIC`:
`billable_in`/`billable_out`/`billable` riêng (input − cache + output), `cache_hit`, `by_tool`
(histogram từng action), `reads`/`dup_reads`/`dup_ratio` (phát hiện đọc lại vùng chưa đổi —
paramsPreview bị cắt 200 ký tự nên chỉ là giới hạn dưới, dùng để soi chứ không để kết án),
`failures`/`repeat_fail_max`, `time_to_first_check_ok`, `wall`. Chỉ đếm từ SSE, không đụng Core.
Quy ước A/B: cùng model/skill/prompt, chạy tuần tự, mỗi biến thể ≥ 3 lần, báo median + min–max;
Test 2 iterate ở 1k/10k dòng, chỉ confirm full 50k khi đã xong. Tiêu chí pass: billable ↓, wall ↓,
coverage + `checkRange` giữ nguyên; cache-hit tụt mạnh → điều tra; nhanh hơn mà lỗi tăng → loại.

**Bước 1 — kỷ luật plan + vertical slice bằng chữ (ĐÃ LÀM, 02/10/2026)**: `PlanRule` mới trong
`core-go/internal/agent/prompt.go` (plan MỘT lần trước lệnh đầu, chỉ re-plan khi tiền đề sai hoặc
check hỏng; đọc để làm tiếp chứ không đọc để yên tâm; vùng > vài trăm dòng chỉ đọc mẫu đầu/cuối +
kích thước) + quy tắc 9–10 và 2 gạch checklist trong `skills/mo-hinh-nhieu-sheet/SKILL.md`.
Không đụng vòng lặp provider, không đụng bridge, thứ tự `Build` giữ nguyên nên cache còn nguyên.
A/B đầu tiên nên chạy Test 3 (bài nát nhất: 4/14 sheet, 9 sheet TRỐNG): metric là rounds,
billable out, số sheet có nội dung, `issueCount`.

**Bước 2 — chặn response phình (CHƯA LÀM)**: cắt `readRange`/`checkRange` trả về ở wrapper Go
(mẫu giới hạn + `truncated`/`totalRows`/`nextRange), **nhưng luôn trả đủ `issueCount` và tổng lỗi** —
cắt chi tiết được, giấu số lỗi thì không. Làm ở Go trước (một nền tảng, rẻ), chỉ đụng `calc.py`/
`gate.py` nếu wrapper không đủ. Đo billable input, số lần đọc lại, coverage có giữ không.

**Bước 3 — sinh dữ liệu bằng công thức tất định (ĐÃ LÀM phần skill + đo trên LO, 04/10/2026 — chưa A/B với LLM)**: recipe trong skill — seed cố định +
`MOD(ROW()*…,…)` + `INDEX(Lists)`, recalc ra số y hệt (không bay hơi như `RANDBETWEEN`, giữ được
"preserve raw data"). Đo 1k → 10k → 50k: output token, giờ sinh/ghi/tính riêng, hash 2 lần chạy có
khớp không. Chỉ khi cách này không đạt phân phối đề yêu cầu mới prototype native generator bên
LibreOffice trước, Windows sau.

**Không làm**: tăng budget/round-cap để "xong cho rồi", concurrency bridge, tắt auto-calc (đã đo
là sai), prompt động mất cache, thêm LLM planner thứ hai.

### Kết quả A/B Test 3 — n=1 mỗi bên (03/10/2026)

Model: `oc/muse-spark-1.3-contributor-free` (bạn chọn; `ocg/deepseek-v4.1-flash` đang dính weekly limit
của proxy — probe qua `POST /v1/llm/test` trước mỗi lượt). Cùng đề `test3.txt`, cùng ngân sách
1.000.000/300 vòng, cùng bridge **64 lệnh** (bản extension mới CỐ Ý chưa nạp vào LO lúc chạy — stash
`calc.py` trước khi LO khởi động, pop ngay sau đó, để hai bên cùng contract). Bằng chứng:
`C:\Users\ThuyetMT\test\bench\axiom-test3ab-{base,head}.log` + `result\axiom-test3ab-{base,head}.ods`.

| | CONTROL (`da699e1`, trước PlanRule) | VARIANT (HEAD: PlanRule + skill 9–10) |
|---|---|---|
| Kết thúc | `stopped` @1.000.056 | `stopped` @1.026.952 — **cả hai hết ngân sách, chưa verify** |
| Vòng / tool call | 168 / 183 | 216 / 227 |
| **Wall** | 92,7 phút | **47,7 phút (−49%)** |
| Token ra / cache_hit | 497.160 / 0,900 | **368.822 (−26%)** / 0,930 |
| Sheet | **14/14** (có README) | 13 (thiếu README) |
| Chart | 0 | **6** (Dashboard) |
| Schedule / Dependencies | 5.001 / 4.901 dòng | **101 / 2.001 dòng — nông hơn hẳn** |
| Tasks | 70.000 công thức | 110.000 công thức |
| by_tool khác biệt | writeRange=34, fillRange=98 | **writeRanges=33** (lệnh batch), fillRange=122, readRange=52, addChart=3 |

Đọc số (n=1 — model free dao động mạnh, quy ước vẫn là ≥3 lần/bên nên CHƯA KẾT LUẬN ai hơn):
- Variant **nhanh gấp đôi, tiết kiệm 26% token ra**, dùng `et.writeRanges` đúng như thiết kế, nạp đúng
  skill `mo-hinh-nhieu-sheet`.
- Nhưng hai bên "tiêu tiền" vào chỗ KHÁC nhau: variant làm Dashboard/chart đầy hơn thì Schedule/
  Dependencies nông hơn — và **cả hai đều chưa tới bước tự soát** (`verified=None`). Bài Test 3 với
  model này @1M chưa bài nào xong: phải tách "xong được không" khỏi "tốn bao nhiêu" (đúng bài học mục 6).

### Bài học vận hành A/B — ghi để lần sau khỏi mất lượt chạy (03/10/2026)

1. **Cô lập document là bắt buộc.** Lượt variant #1 hỏng vì `et.newWorkbook` không cô lập: extension
   chọn document qua `documents.active()` có fallback *"không có active thì lấy tài liệu cùng loại cuối
   cùng"*, nên giữa chừng mọi lệnh rơi vào workbook của CONTROL (bằng chứng `listSheets` trả
   `workbook=test3-base.ods`), saveAs cuối lưu nhầm tài liệu đối thủ. Bằng chứng giữ ở
   `result\axiom-test3ab-head-CONTAMINATED.ods`. Một lượt clean phải có **đúng một document**:
   restart LO → `et.newWorkbook` → kiểm `GET /session` trước khi chạy.
2. **Restart LO sau `taskkill /F` có thể mở ra không có document nào** (mọi lệnh trả
   "no active spreadsheet") — tạo lại qua `et.newWorkbook`, kiểm `GET /session`.
3. **Sửa extension phải `scripts/libreoffice.ps1 -Install`** (đóng gói `.oxt` + `unopkg add`): LO chạy
   từ gói đã cài, không đọc source. Bằng chứng: 4 case live mới fail 100% khi chưa cài lại → **286/286
   pass** ngay sau khi cài.
4. **Giữ contract bridge cũ cho A/B prompt/skill**: stash file extension trước khi LO khởi động, pop
   ngay sau — bộ lệnh trong bộ nhớ LO là bản cũ, đĩa trở lại bản mới.
5. Trước mỗi lượt: probe model qua `POST /v1/llm/test` (model free đổi vận cả ngày).

### Kết quả A/B Test 3 — đợt mimo, pilot n=3/cặp (03/10/2026 tối)

Điều kiện: model `oc/mimo-v2.6-flash-free` (user chọn), đề `test3.txt`, 1.000.000 token / 300 vòng,
CONTROL `da699e1` vs VARIANT `4169475` (PlanRule + skill 9–10 — khác đúng 2 file), thứ tự trong
cặp random **C V | V C | C V** (tư vấn ChatGPT: đừng cố định ABAB). Hạ tầng + driver + bài học
vận hành: `C:\Users\ThuyetMT\test\bench\ab\README.md`; báo cáo máy: `ab\report-pilot.md`
(paired-delta, độ sâu từng sheet, phân loại lỗi agent/infra).

| Cặp | Wall C → V | Billable C → V | Sheet | Chart C → V | Lỗi C → V |
|---|---|---|---|---|---|
| 1 | 84,1′ → 57,6′ (**−31,5%**) | 798.002 → 570.077 (**−28,6%**) | 13/13 = 13/13 | 4 → 0 | 3 → 5 |
| 2 | 76,9′ → 77,5′ (+0,8%) | 774.133 → 686.052 (**−11,4%**) | 13/13 = 13/13 | 2 → 0 | 1 → 3 |
| 3 | 99,9′ → 90,0′ (**−9,9%**) | 829.643 → 748.317 (**−9,8%**) | 13/13 = 13/13 | 6 → 6 | 8 → 0 |

**Cả 6 lượt `run.completed` + `verified=True`** — primary endpoint "chất lượng @ 1M" bằng nhau
trọn vẹn (13/13 sheet bắt buộc ×6), nên tác dụng PlanRule nổi lên ở **chi phí**:

- **Billable:variant rẻ hơn cả 3 cặp** — paired-delta median **−11,4%** (−28,6 / −11,4 / −9,8).
- **Tool call: −24,2% median** (534→382, 466→353, 489→420) — đọc/ghi lại ít hơn đúng như mục 10.
- **Wall: nhanh 2/3 cặp, median −9,9%** (cặp 2 hòa).
- Công thức tổng: median −7,6% (cặp 3 variant thấp hơn rõ: 261k vs 405k) — depth không bắt buộc
  nhưng nên theo dõi.
- **Chart là điểm yếu duy nhất**: control có chart 3/3 lượt (4/2/6), variant 1/3 (0/0/6). Đề không
  yêu cầu chart bắt buộc cho Dashboard, nhưng "GANTT visual" nên có biến thể hình.
- Lỗi agent: tổng 12 (C) vs 8 (V); không có lượt nào lỗi hạ tầng (1 lượt `empty reply` đã bị loại
  trước khi chạy — ghi `ab/infra-failures.log`, không tính vào bảng).

**Kết luận pilot (chưa chốt)**: hướng nhất quán 3/3 ở chi phí, chất lượng mốc bằng nhau — PlanRule
**đáng giữ**. Nhưng n=3 theo tư vấn ngoài chỉ là pilot (nên 5–10 cặp), và đây chỉ là lượt 1 của
đợt mimo (lượt n=1 hôm qua chạy `muse-spark` không trộn vào được). Còn nhiễu: wall cặp 1 lẹ tới
31,5% rồi cặp 2 bằng nhau — chưa đủ để chốt "nhanh hơn", nhưng "rẻ hơn" thì cả 3/3 cùng hướng.
Caveat ghi rõ: đang đo **gói prompt+skill gộp** (PlanRule + quy tắc 9–10), không tách được từng phần.

**Đề xuất bước tiếp**: (a) chạy thêm 2–4 cặp nữa nếu muốn chốt chắc (mỗi cặp ~2,5–3 giờ, cùng
driver, không sửa script giữa chừng — bài học #9); hoặc (b) coi đây là đủ để **giữ PlanRule và
sang Bước 2** (cắt response `readRange` ở wrapper Go), kèm 1 hành động nhỏ: bổ sung checklist
"Dashboard có chart/Gantt" vào skill để xử lý điểm yếu chart.

### Bước 2 + Bước 3 — đã làm khi proxy LLM tắt (04/10/2026 rạng sáng)

**Bước 2 (cắt response, `core-go/internal/tools/truncate.go`)**: `readRange` giữ 200 dòng đầu (≤ 6.000
ô) + `tailValues` 10 dòng cuối + `truncated/totalRows/totalCols/nextRange`; `checkRange` giữ nguyên
`issueCount`, cắt list ở 100 + `issuesOmitted`; lỗi trả nguyên văn; mô tả tool có `TruncationContract`
(mẫu bị cắt không chứng minh gì về phần còn lại). Tail + contract theo review của ChatGPT. Smoke test
live trên LibreOffice thật (`truncate_live_test.go`, tag `live`) **10/10**: đọc 50.001 dòng
**1.000.101 byte → 2.836 byte** vào history (~350×). Còn lại: mục hành vi (anomaly chỉ ở đuôi → agent
không được kết luận "sạch") cần LLM.

**Bước 3 (`skills/mo-hinh-nhieu-sheet/references/du-lieu-gia-lap.md` + quy tắc 11)**: hàm băm nguyên
`f(x)=x(2x+1) mod 2^22` hai vòng (ChatGPT đề xuất), u=(f+0,5)/2^22, salt = k×100003. Đo trên LO thật
từng ứng viên (50.000 dòng × 4 cột): χ² 10 bin < 16,9, CORREL < 0,006, tự tương quan < 0,005, điền lại
y hệt. **Hai bẫy chỉ đo mới thấy**: salt liền nhau 1,2,3 → CORREL ≈ 0 nhưng **49.999/50.000 ô** là bản sao
lệch dòng; cách vá bằng khối 65536·k → CORREL 0,166 (tệ hơn). Sheet `Raw_SKU_Data` đủ 11 cột × 50.000
dòng của Test 2: **9,7 giây**, 0 ô lỗi, tỷ lệ thật thiếu 1.014/1.056 (đích ~1.000), trùng 508 (~500),
demand 0 2.496 (~2.500), tồn âm 244 (~250), lead ≥ 60 1.548 (~1.500). Thêm quy tắc 12 (Dashboard có
biểu đồ) từ điểm yếu chart của A/B. Test cố định `repo_skills_test.go`: mọi `read_skill_file` nhắc
trong SKILL.md phải trỏ tới file có thật.

**Chưa đo**: model có thật sự theo quy tắc 11 khi làm Test 2 không, và token/wall giảm bao nhiêu — cần
proxy LLM. A/B mở rộng n=5 cũng dừng: variant4 chết HTTP 429 `FreeUsageLimitError` (đã loại, ghi
`ab/infra-failures.log`), sau đó proxy `localhost:20128` tắt hẳn.

### Lỗi tham số của agent — soi từ log A/B, đã sửa (04/10/2026, `c204fc5`)

Quét 3.328 tool call của 8 lượt A/B Test 3: phần lớn lỗi agent là `'range' is required` / `'writes' is
required` **trong khi model đã gửi tham số đó** — thông báo lỗi đánh lừa nên model gửi lại y hệt:
- 10 lần `et.formatRange` gửi `cell_range` (tên tham số của tool file MCP `excel_*` mà agent cũng thấy)
  → nay quy về `range` khi lệnh có tham số `range` và model chưa gửi tên đúng.
- 10 lần `params` là **chuỗi** JSON hỏng (công thức có `\"` lồng, xuống dòng thô…) → trước đây bị bỏ
  im thành `nil`. Nay lấy object đầu tiên nếu chỉ có rác phía sau; còn hỏng thì trả
  `params ... is not valid JSON (... near: ...)` và không gọi bridge.
Chưa đo lại được trên LLM (proxy 429); kỳ vọng: hết chuỗi lặp lỗi kiểu base3 (8 lần liền).

### Test 2 với bản HEAD (`1e267b4`, Bước 2 + Bước 3) — model `oc/mimo-v2.6-flash-free` (04/10/2026)

Một lượt, 1M/300 vòng. Bằng chứng: `C:\Users\ThuyetMT\test\bench\axiom-test2h-head1.{out,log}` +
`result\axiom-test2h-head1.ods`.

| | Lần đo cũ (02/10, deepseek, trước Bước 2/3) | HEAD (04/10, mimo) |
|---|---|---|
| Kết thúc | hết ngân sách | **`completed`, `verified=True`**, 76,8′, billable 654.771 |
| Phủ dòng của sheet phân tích | **400 / 52.001** | **50.001 / 50.001** ở cả 7 sheet phân tích |
| Checks | chỗ thử hàm, PASS=0 FAIL=0 | **37 kiểm tra công thức, 37 PASS** |
| Biểu đồ Dashboard | — | 4 (`et.addChart`) |
| Lỗi tool | — | 3, không lặp (`repeat_fail_max=1`) |

- **Quy tắc 11 có tác dụng**: lệnh thứ 3 là `read_skill_file references/du-lieu-gia-lap.md`; dữ liệu 50.000
  dòng sinh bằng 130 `fillRange` công thức tất định, model còn tự biến tấu đúng (10 location chung một
  SKU dùng chung u để thuộc tính sản phẩm nhất quán). Tỷ lệ thật: trùng 0,91%, thiếu 2,85%, demand 0 =
  2.589, cực đại 0,40%, tồn âm 486.
- **Bước 2**: tổng kết quả tool vào ngữ cảnh cả lượt 162.754 byte, lần lớn nhất 9.465 byte.
- **Lỗi params mới (`c204fc5`) phát huy**: một lần `writeRanges` chuỗi JSON hỏng nhận "not valid JSON
  (unexpected EOF)" và model không gửi lại y hệt.
- **Kiểm độc lập**: đổi `Dashboard!B4` 0,95→0,99 → Z 1,645→2,326, Safety Stock dòng đầu 7,44→10,53
  (tính tay `Z·σ·√LT` khớp cả hai), trả lại 0,95.
- **Chưa phải A/B sạch**: lần cũ khác model. Bước nhảy phủ dòng 400→50.001 gắn trực tiếp với việc sinh
  dữ liệu bằng công thức (quan sát được trong log), nhưng muốn có số về token/wall phải chạy baseline
  `run-variant` (4169475) trên Test 2 với mimo.
- Ứng viên tối ưu cho model yếu thấy trong lượt này: model mất ~4 lệnh dò cú pháp tham chiếu sheet
  (`Sheet1.A1` vs `Sheet1!A1`) — mô tả tool/bridge chưa nói; lượt suy nghĩ đầu ~18 phút; `et.deleteSheet`
  không có (lần 2).
