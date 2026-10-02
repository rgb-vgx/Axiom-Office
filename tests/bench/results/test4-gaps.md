# Test 4 — Monte Carlo Risk Simulation: đối chiếu Claude Code với Axiom Core

Đề: `C:\Users\ThuyetMT\test\benchmark\test4.txt` (8 sheet, mô phỏng Monte Carlo, phân phối, thống kê,
độ nhạy, dashboard, checks).

| | Claude Code | Axiom Core |
|---|---|---|
| Thời gian | 09:09:01 → 09:39:16 (**30 phút**) | 11:20:33 → 11:52:01 (**31 phút**) |
| Tool call | 78 (Bash 33, Edit 31, Read 8, Write 6) | 175 (171 ok / 4 lỗi) |
| Token | ra 295.068 / cache đọc 19.321.984 | ra 306.816 / cache đọc 3.913.600; **phải trả 404.021** |
| Kết thúc | hoàn thành | `token budget exceeded (billable 404021 > 400000)` |
| Kết quả | 10 sheet, 390.949 ô công thức, 4 chart, 2 khối CF, 2 data validation | **8 sheet (đủ 8/8 tên đề yêu cầu), 330.132 ô công thức, 0 chart** |

Đây là bài **sát nhất** trong bốn bài: cùng thời gian, cùng bậc độ lớn token, và số ô công thức chỉ kém
15%.

## Claude Code đã làm gì

Nguồn: `tests/bench/results/test4-claude-code.summary.txt`, `test4-claude-code-output.txt`.

`build_model.py` sửa 26 lần; thêm `stress_test.py` (5 lần), `negative_test.py` (2 lần),
`recalc_util.py`, `repro_test.py`. Đáng chú ý: trong thư mục còn `_base_recalc`, `_neg_recalc`,
`_stress` — nó lưu lại từng trạng thái workbook để **so kết quả sau mỗi lần đổi tham số**, tức là làm
đúng bài "thay đổi giả định rồi kiểm chứng chuỗi phụ thuộc" mà đề yêu cầu.

## Axiom Core đã làm gì

Nguồn: `tests/bench/results/test4-axiom-calls.txt`, `test4-axiom-output.txt`,
log `axiom-test4.log`, file lưu `result/axiom-test4.ods`.

```
Inputs | Assumptions | Distributions | Simulation | Statistics | Sensitivity | Dashboard | Checks
                                                                        (đủ 8/8 tên đề yêu cầu)
o cong thuc: 330.132        chart: 0        conditional formatting: 0        named expressions: 1
```

Phân bố tool call: `et.formatRange` **78**, `et.fillRange` 37, `et.writeRange` 30, `et.readRange` 14,
`et.addSheet` 7. Đây là bài duy nhất mà định dạng chiếm nhiều lời gọi nhất — model dành công sức làm
Dashboard và các bảng số cho dễ đọc.

## Khoảng cách

| # | Khoảng cách | Bằng chứng | Vì sao |
|---|---|---|---|
| 1 | ~~Không có chart~~ **đã giải thích được, và không phải khoảng cách năng lực** | Lượt 400k: `chart: 0`. Lượt 1M: **5 chart, 16 lời gọi `et.addChart`** | Không phải model không biết dùng — bốn lượt trước đều hết ngân sách trước khi tới Dashboard. Xem mục "Chạy lại với ngân sách rộng" |
| 2 | **Không có định dạng điều kiện** | `conditional formatting: 0`; Claude Code có 2 khối | Bridge chưa có lệnh nào làm được việc này |
| 3 | **Model phát ra tên tool hỏng** | Có lời gọi với tên `et.formatRange">\n<｜｜DSML｜｜ parameter name="params"...` — model trộn cả markup nội bộ của nó vào tên tool | Không phải lỗi bridge (lệnh vẫn bị từ chối sạch), nhưng đốt một vòng và cho thấy chưa có gì bắt sớm kiểu hỏng này |
| 4 | **Dừng vì ngân sách token** | `billable 404021 > 400000` sau 175 tool call | Sát ngưỡng (1%) — đây là bài gần xong nhất trong bốn bài |

## Cải tiến đã làm để thu hẹp

Không có cải tiến mới riêng cho Test 4. Bài này hưởng lợi trực tiếp từ những thứ làm ở các bài trước —
và con số cho thấy chúng cộng lại có tác dụng:

- `et.addSheet`/`et.renameSheet` (commit `ddf1c31`): dựng đủ 8 sheet đúng tên.
- `et.fillRange` (commit `bd8b94e`): 37 lời gọi — dựng bảng mô phỏng bằng công thức thay vì phát từng ô.
- Ngân sách token chỉ tính phần phải trả (commit `f243789`): 4,3 triệu token thô (3,9 triệu từ cache)
  lọt trong ngân sách 400k.
- Trần token mỗi phản hồi 32768 → 65536 (commit `074afee`).
- Lưu tài liệu trước khi đóng app (commit `074afee`): nhờ nó bài này mới còn file để kiểm chứng.

## Chạy lại với ngân sách rộng: Axiom HOÀN THÀNH

Bốn lượt trước đều dừng vì ngân sách 400k — con số **tôi tự đặt**, không phải giới hạn sản phẩm
(`MaxMaxTokens` là 1.000.000). Câu hỏi chưa trả lời được là: Axiom **có thể** xong một bài không, tách
khỏi câu hỏi "xong trong bao nhiêu token". Đã chạy lại Test 4 với ngân sách 1.000.000:

| | Axiom @ 400k | Axiom @ 1M | Claude Code |
|---|---|---|---|
| Kết thúc | `token budget exceeded` | **hoàn thành** | hoàn thành |
| Vòng / tool call | 79 / 175 | **293 / 656** | — / 78 |
| Thời gian | 31 phút | 24 phút | 30 phút |
| Token phải trả | 404.021 | **375.806** | — |
| Sheet | 8/8 tên đề | 8/8 tên đề | 8/8 + StressTest + README |
| Ô công thức | 330.132 | 150.479 | 390.949 |
| Chart | 0 | **5** | 4 |
| Tự kiểm chứng | — | **`verified=true`** | có |

Ba điều đáng chú ý:

1. **Chart không phải "không biết dùng" mà là "chưa kịp tới".** Lượt rộng gọi `et.addChart` **16 lần** và
   `et.listCharts` 4 lần, ra 5 chart. `et.addChart` vốn đã nằm trong danh sách lệnh model được chào
   (đã kiểm: `catalog/live-commands.json` có nó) — trước đây chỉ là bốn lượt đều hết ngân sách trước khi
   tới Dashboard.
2. **Ngân sách 400k cắt ngang một lượt lẽ ra đã xong.** Lượt rộng làm **3,7 lần số tool call** mà chỉ
   tốn **375.806** token phải trả, ít hơn 404.021 của lượt bị cắt. Lý do: lượt bị cắt có phản hồi model
   dài gấp ~8 lần mỗi vòng (306.816 token ra / 79 vòng so với 141.269 / 293 vòng). Kích thước phản hồi
   dao động mạnh giữa các lần chạy, nên một ngân sách vừa khít sẽ cắt oan.
3. **Tự kiểm chứng đã chạy** (`verified=true`) — tính năng thêm ở commit `ddf1c31` hoạt động trong một
   lượt thật: 17 lời gọi `et.checkRange` và 174 `et.readRange` để đọc lại.

Phân bố lời gọi lượt rộng: `writeRange` 226, `readRange` 174, `formatRange` 149, `fillRange` 40,
`checkRange` 17, `addChart` 16, `undo` 11, `addSheet` 7, `listCharts` 4.

## Còn lại, chưa làm

- **Định dạng điều kiện** — khoảng cách năng lực chung của cả bốn bài, và là thứ Claude Code cũng phải
  lách. Bản LibreOffice này từ chối `XSheetConditionalEntries.addNew` (Claude Code ghi rõ trong báo cáo
  của nó; tôi chưa tự đo lại), nên cần quyết định trước khi làm: đi đường UNO hay viết lại gói OOXML.
- **Vì sao model không dùng `et.addChart`** dù nó có sẵn — câu hỏi này lặp lại ở Test 3 và Test 4, và cả
  hai lần đều có thể giải thích bằng "hết ngân sách trước khi tới Dashboard". Cần một lần chạy không bị
  cắt để phân biệt "không biết dùng" với "chưa kịp dùng".
- **Chưa đối chiếu nội dung thống kê** (trung bình, độ lệch chuẩn, phân vị, kết luận rủi ro) với Claude
  Code — hiện mới so được tên sheet và số ô công thức.
