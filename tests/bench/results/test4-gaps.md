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
| 1 | **Không có chart** | `chart: 0` trong file lưu, dù `et.addChart` có sẵn và đã kiểm chứng trên LibreOffice thật; Claude Code có 4 chart | Giống Test 3. Chưa rõ vì sao model không gọi: có thể hết ngân sách trước khi tới Dashboard, có thể mô tả tool chưa đủ nổi bật |
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

## Còn lại, chưa làm

- **Định dạng điều kiện** — khoảng cách năng lực chung của cả bốn bài, và là thứ Claude Code cũng phải
  lách. Bản LibreOffice này từ chối `XSheetConditionalEntries.addNew` (Claude Code ghi rõ trong báo cáo
  của nó; tôi chưa tự đo lại), nên cần quyết định trước khi làm: đi đường UNO hay viết lại gói OOXML.
- **Vì sao model không dùng `et.addChart`** dù nó có sẵn — câu hỏi này lặp lại ở Test 3 và Test 4, và cả
  hai lần đều có thể giải thích bằng "hết ngân sách trước khi tới Dashboard". Cần một lần chạy không bị
  cắt để phân biệt "không biết dùng" với "chưa kịp dùng".
- **Chưa đối chiếu nội dung thống kê** (trung bình, độ lệch chuẩn, phân vị, kết luận rủi ro) với Claude
  Code — hiện mới so được tên sheet và số ô công thức.
