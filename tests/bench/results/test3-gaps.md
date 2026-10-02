# Test 3 — Project Portfolio Management: đối chiếu Claude Code với Axiom Core

Đề: `C:\Users\ThuyetMT\test\benchmark\test3.txt` (13 sheet, ≥100 dự án, ≥5.000 task, ≥500 nguồn lực,
phụ thuộc công việc, đường găng, tải nguồn lực, rủi ro, dashboard, checks).

| | Claude Code | Axiom Core |
|---|---|---|
| Thời gian | 08:44:07 → 09:09:01 (**25 phút**) | 10:38:51 → 11:20:26 (**42 phút**) |
| Tool call | 136 (Edit 63, Bash 43, Read 25, Write 5) | 287 (264 ok / 23 lỗi) |
| Token | ra 536.688 / cache đọc 43.548.544 | ra 338.166 / cache đọc 1.615.104; **phải trả 406.385** |
| Kết thúc | hoàn thành | `token budget exceeded (billable 406385 > 400000)` |
| Kết quả | **17/17 sheet đều có nội dung**, 494.340 ô công thức, 4 chart, 15 khối CF, 2 data validation | **4/14 sheet có nội dung**, 181.603 ô công thức, 0 chart, 0 CF |

> **Đính chính (02/10/2026).** Bản đầu của báo cáo này ghi "14 sheet (đủ 13/13 tên đề yêu cầu), 181.603 ô
> công thức" mà chỉ đếm **tổng** số công thức. Đếm theo từng sheet thì con số đó nằm hết trong 4 sheet
> đầu; **9 sheet đề yêu cầu — `Budget`, `Actuals`, `Schedule`, `Critical_Path`, `Resource_Load`, `Risks`,
> `Portfolio`, `Dashboard`, `Checks` — TRỐNG HOÀN TOÀN**, không cả tiêu đề. Tổng số công thức là một
> thước đo tồi vì nó che mất chuyện đó.

## Claude Code đã làm gì

Nguồn: `tests/bench/results/test3-claude-code.summary.txt`,
`test3-claude-code-output.txt`.

`ppm_build.py` bị **sửa 41 lần** — đây là bài có vòng lặp tinh chỉnh dày nhất trong bốn bài, và lần này
nó sửa script chứ không sửa tài liệu. `ppm_data.py` (sinh dữ liệu) sửa 17 lần, `verify_ods.py` 4 lần.
17 sheet gồm đủ 13 sheet đề yêu cầu cộng README, Gantt, StressTest, Lists.

## Axiom Core đã làm gì

Nguồn: `tests/bench/results/test3-axiom-calls.txt`, `test3-axiom-output.txt`,
log `axiom-test3.log`, file lưu `result/axiom-test3.ods`.

```
sheet                 dong   cong thuc          sheet              dong  cong thuc
Projects               101        2.800         Budget                1         0
Tasks                5.001      129.999         Actuals               1         0
Dependencies         5.101       40.800         Schedule              1         0
Resources              515        8.004         Critical_Path         1         0
Gantt                    1            0         Resource_Load         1         0
Dashboard                1            0         Risks                 1         0
Checks                   1            0         Portfolio             1         0
```

**Axiom dựng xong 4 sheet DỮ LIỆU, còn 9 sheet PHÂN TÍCH mà đề chấm điểm thì trống.** Không phải
"gần xong": đây là phần lớn giá trị của bài (đường găng, tải nguồn lực, rủi ro, portfolio, dashboard,
checks) chưa hề được làm.

So sánh từng sheet với Claude Code (`ods_summary.py` trên file của hai bên):

```
sheet              Axiom     Claude Code          sheet            Axiom   Claude Code
Projects           2.800          1.500           Schedule             0       155.000
Tasks            129.999         40.000           Critical_Path        0        80.000
Dependencies      40.800         84.117           Actuals              0       110.000
Resources          8.004          6.000           Resource_Load        0         9.144
Dashboard              0            180 (8 chart) Budget               0         1.700
Checks                 0             87           Risks                0         2.124
Portfolio              0          2.814           Gantt                0         1.083
```

Đây là **lần chạy lại**: lần đầu (10:24) chết sau **16 tool call** với `finish_reason=length`. Đọc phần
suy luận cuối trong log thì thấy model đang lập kế hoạch 5.000 task × 10 cột công thức và **phần suy luận
một mình đã ăn hết trần 32.768 token** trước khi kịp viết lệnh nào — nó còn đang cân nhắc `CHOOSE`,
`MOD`, `INDEX` cho từng cột.

## Khoảng cách

| # | Khoảng cách | Bằng chứng | Vì sao |
|---|---|---|---|
| 0 | **9/13 sheet đề yêu cầu TRỐNG** | `ods_summary.py` trên `axiom-test3.ods`: Budget, Actuals, Schedule, Critical_Path, Resource_Load, Risks, Portfolio, Dashboard, Checks — mỗi sheet 1 dòng, 0 công thức, 0 ô chữ. Claude Code: cả 17 sheet đều có nội dung | Đây là khoảng cách lớn nhất và trước đây bị **che mất** vì tôi chỉ đếm tổng số công thức. Lượt chạy dừng vì ngân sách khi vừa xong 4 sheet dữ liệu — model làm theo thứ tự, phần phân tích nằm cuối |
| 1 | **Trần token của MỘT phản hồi quá thấp cho bài suy luận nặng** | Lần đầu: chết ở 16 tool call với `finish_reason=length`; suy luận cuối cùng cho thấy model đang chọn công thức cho 10 cột × 5.000 dòng | Trần 32.768 tính cả phần suy luận. Model khai `maxOutput` 384.000 nên trần của mình thấp hơn cả chục lần so với cái model chịu được |
| 2 | **Không có chart, không có định dạng điều kiện** | `chart: 0`, `conditional formatting: 0` trong file lưu, trong khi Claude Code có 4 chart và 15 khối CF | `et.addChart` có sẵn và đã kiểm chứng, nhưng model không tới đó; `et.formatRange` không làm được định dạng điều kiện (chưa có lệnh nào làm được) |
| 3 | **Vẫn dừng vì ngân sách trước khi xong bài** | `billable 406385 > 400000` sau 287 tool call và 42 phút | Bài lớn nhất về số sheet. Claude Code trả bằng số vòng (136 tool call, 536.688 token ra) chứ không phải bằng số lệnh bridge |
| 4 | **23 lời gọi lỗi, phần lớn do model tự viết sai JSON** | model tự nhận: *"Some calls failed because of malformed JSON (I wrote `"action": {"formula"...` incorrectly - missing action param)"* | Không phải lỗi bridge — nhưng mỗi lần sai là một vòng bị đốt, và trước đây không có gì bắt được kiểu sai lặp lại này nếu nó lặp y hệt |

## Cải tiến đã làm để thu hẹp

1. **Nâng trần token mỗi phản hồi 32768 → 65536** (commit `074afee`) — khoảng cách #1. Cùng bài, cùng
   ngân sách: **16 → 287 tool call**. Đây là lần thứ ba phải nâng trần này (4096 → 16384 → 32768 →
   65536), mỗi lần đều vì một lượt chạy thật chết ở `finish_reason=length`.
2. **Lưu tài liệu trước khi đóng app** (commit `074afee`) — không phải khoảng cách của bài này mà là lỗi
   quy trình vừa phát hiện ở Test 4: chạy 27 phút rồi đóng LibreOffice mà tài liệu chưa lưu thì mất sạch,
   không còn gì để kiểm chứng.
3. `et.addSheet`/`et.renameSheet`, `et.fillRange`, ngân sách token chỉ tính phần phải trả — làm từ trước,
   và là điều kiện để lượt này dựng được 14 sheet trong 287 lời gọi.

## Còn lại, chưa làm

- **Không có lệnh nào làm định dạng điều kiện** (khoảng cách #2). Đây là khoảng cách năng lực thật, không
  phải model quên. Claude Code cũng gặp: nó ghi rõ bản LibreOffice này từ chối
  `XSheetConditionalEntries.addNew`, và nó phải viết lại OOXML để gắn CF. Trước khi làm lệnh này cần
  quyết định: đi đường UNO (nếu bản LibreOffice chấp nhận) hay đường viết lại gói OOXML như Claude Code.
- **Chart có sẵn nhưng model không dùng** (khoảng cách #2). Chưa rõ vì sao — có thể nó hết vòng trước khi
  tới Dashboard, có thể mô tả tool chưa đủ nổi bật. Cần một lần chạy không bị cắt để trả lời.
- **Chưa đối chiếu nội dung từng sheet với Claude Code** (số dòng task, đường găng có đúng không, Checks
  PASS/FAIL ra sao) — hiện mới so được số sheet và số ô công thức.
- **Chưa chạy lại Test 3 sau khi nâng trần** để biết nó có tới được Dashboard/Checks không: lượt vừa rồi
  vẫn dừng vì ngân sách, nên câu hỏi đó chưa có câu trả lời.
