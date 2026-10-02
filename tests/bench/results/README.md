# Đối chiếu bốn bài benchmark: Claude Code vs Axiom Core

Đề: `C:\Users\ThuyetMT\Downloads\libre_calc_agent_extreme_benchmark(1).md`, tách thành
`tests/../benchmark/test1..4.txt`. Cả hai bên chạy trên **LibreOffice thật**, cùng model
`ocg/deepseek-v4.1-flash`, cùng máy.

- Claude Code: `claude --settings ...settings.proxy.json`, tự mở instance headless riêng và nói qua
  **UNO socket**.
- Axiom Core: nối vào **tài liệu đang mở** qua HTTP bridge của extension, chạy bằng
  `tests/bench/run_prompt.py`.

Hai bên **phải chạy tuần tự**, không chồng nhau: chúng dùng chung profile LibreOffice, và một lần chạy
song song đã làm mất tài liệu giữa lượt (xem `test1-gaps.md`, mục "Một lượt chạy lại KHÔNG dùng được").

## Kết quả

| Bài | Claude Code | Axiom Core | Báo cáo |
|---|---|---|---|
| 1 — FP&A (11 sheet) | 11/11 sheet, 15.349 ô công thức, 5 chart, 16 khối CF, **có bài change test** | @400k: 9/11 sheet. **@1M: 11/11 sheet, 24.130 ô công thức, 5 chart** — còn thiếu CF và change test | [test1-gaps.md](test1-gaps.md) |
| 2 — Inventory (50.000 SKU) | **12/12 sheet đủ nội dung**, mọi sheet phân tích 50.001 dòng, 2.156.652 ô công thức, 4 chart, Checks có PASS/FAIL (PASS 247 / FAIL 88) | 12/12 sheet CÓ nội dung, nhưng phân tích chỉ phủ **400/52.001 dòng**; sheet `Checks` chỉ là chỗ thử hàm — **PASS=0, FAIL=0**; Dashboard tự mâu thuẫn (phụ đề "50,000", chú thích "400 SKUs") | [test2-gaps.md](test2-gaps.md) |
| 3 — PPM (5.000 task) | **17/17 sheet đều có nội dung**, 494.340 ô công thức, 4 chart, 15 khối CF | **4/14 sheet có nội dung**; 9 sheet đề yêu cầu TRỐNG; 181.603 ô công thức dồn hết vào 4 sheet dữ liệu | [test3-gaps.md](test3-gaps.md) |
| 4 — Monte Carlo | 10 sheet, 390.949 ô công thức, 4 chart, 2 khối CF | 8 sheet (đủ 8/8 tên đề), 330.132 ô công thức, 0 chart | [test4-gaps.md](test4-gaps.md) |

Cả bốn lượt Axiom ở ngân sách 400k đều dừng vì hết ngân sách (Test 1 lượt tốt nhất dừng vì trần 100
vòng). Không lượt nào dừng vì lỗi bridge.

**Nhưng 400k là con số tôi tự đặt, không phải giới hạn sản phẩm** (`MaxMaxTokens` = 1.000.000). Chạy
lại hai bài nặng nhất với ngân sách 1.000.000:

- **Test 4 hoàn thành cả bài**: 293 vòng, 656 tool call, 8/8 sheet, 5 chart, `verified=true`, 24 phút,
  chỉ tốn **375.806** token phải trả — *ít hơn* 404.021 của lượt bị cắt ở 400k.
- **Test 1 dựng xong cả 11/11 sheet** (24.130 ô công thức, 5 chart, kể cả Dashboard và Checks), nhưng
  vẫn hết ngân sách ở 1.176.802 nên chưa làm bài change test. Lượt 400k chỉ được 9/11 sheet.

Nói cách khác: **phần lớn "khoảng cách" ở bảng trên là do ngân sách đo, không phải do năng lực.** Chi
tiết ở [test1-gaps.md](test1-gaps.md) và [test4-gaps.md](test4-gaps.md).

## Cải tiến đã làm, và đo được gì

Xếp theo mức tác động đo được, không theo thứ tự thời gian.

| Cải tiến | Commit | Đo được |
|---|---|---|
| Ngân sách token chỉ tính phần phải trả (bỏ token đọc từ cache) | `f243789` | Cùng ngân sách 400k: Test 1 đi từ **67 → 315 tool call**, **5/11 → 9/11 sheet** |
| `et.fillRange` — một công thức thay cho cả nghìn ô | `bd8b94e` | Chiếm 48–53% lời gọi ở Test 2/3/4; model tự nói: *"I can fill a whole block with one call! This is a huge efficiency win."* |
| `et.addSheet` / `et.renameSheet` cho cả hai bridge | `ddf1c31` | Trước: model đoán ~30 tên lệnh tự nghĩ ra rồi bỏ cuộc. Sau: dựng đủ 11–14 sheet đúng tên, không lần nào đoán lại |
| Lỗi "lệnh không có" liệt kê ngay các lệnh hợp lệ | `ddf1c31` | Vòng đoán tên lệnh từ ~30 xuống 2–3 |
| Trần token mỗi phản hồi 32768 → 65536 | `074afee` | Test 3: **16 → 287 tool call** cùng một bài, cùng ngân sách |
| Cứu lượt chạy bị cắt vì hết ngân sách của một phản hồi | `94208c9` | Trước: chết hẳn cả lượt. Sau: nhắc viết nhỏ lại rồi làm tiếp |
| Phanh khi tool hỏng lặp lại y hệt | `2a0fef3` | Một lượt chạy đã gọi cùng một lệnh cùng một lỗi **90 lần** và đốt 1.000.088 token |
| Nâng trần thời gian lệnh, xếp ba tầng cho đúng thứ tự | `41e056b` | Test 2 có 27 lời gọi chết vì trần 60s cũ |
| `et.addChart` / `et.listCharts` cho cả hai bridge | `94208c9` | Năng lực đã có và xanh trên LibreOffice thật — **nhưng chưa lượt nào của Axiom dùng tới** |
| Luật khảo sát có mức trong system prompt | `94208c9` | Test 1: 18/25 vòng đổ vào thí nghiệm nhỏ ở lượt đầu; lượt sau không còn |
| Lưu tài liệu trước khi đóng app | `074afee` | Một lượt chạy 27 phút đã mất sạch kết quả vì tài liệu chưa lưu |

### Một cải tiến ĐO ĐƯỢC LÀ SAI, đã hoàn tác

Tạm tắt tính lại tự động trong lúc ghi rồi `calculateAll()` một lần — nghe rất hợp lý, unit test xanh,
nhưng đo A/B thật thì **chậm hơn 2,3–4,0 lần** (12,4s → 28,0s; 32,6s → 130,6s). Hoàn tác ở `66de08d` /
`f2545de`, và ghi số đo ngay trong `write_range` để lần sau không ai thêm lại.
Chi tiết: [test2-gaps.md](test2-gaps.md).

## Khoảng cách còn lại, xếp theo mức chặn việc

1. **Định dạng điều kiện** — bridge không có lệnh nào làm được; Claude Code cũng phải lách bằng cách
   viết lại gói OOXML vì bản LibreOffice này từ chối `XSheetConditionalEntries.addNew`. Cần quyết định
   đi đường UNO hay đường OOXML trước khi làm.
2. **Chart có sẵn nhưng không lượt nào dùng** — lặp lại ở Test 1, 3, 4. Mọi lần đều có thể giải thích
   bằng "hết ngân sách trước khi tới Dashboard", nên cần **một lần chạy không bị cắt** để phân biệt
   "không biết dùng" với "chưa kịp dùng".
3. **Gom nhiều lần ghi trong một vòng** — model hay gọi 5–10 `writeRange` liên tiếp, mỗi lần là một lần
   LibreOffice tính lại cả sổ. Hướng đúng là một lệnh ghi được nhiều vùng, không phải đổi chế độ tính
   (đã thử và đo được là sai).
4. **Chi phí token theo từng ô cho dữ liệu thô** — `fillRange` giải quyết phần công thức, nhưng dữ liệu
   giả lập vẫn phải phát từng giá trị. Chưa có đường nạp từ file vào tài liệu đang mở.
5. **Chưa đối chiếu nội dung** (giá trị KPI, PASS/FAIL của Checks, kết quả thống kê) — hiện mới so được
   cấu trúc: tên sheet, số ô công thức, số chart, số khối định dạng điều kiện.

## Bằng chứng nằm ở đâu

Mỗi bài có bốn nhóm file, tên theo `testN-`:

- `testN-claude-code.summary.txt` — thống kê transcript `.jsonl`: tool call, token, file ghi/sửa.
- `testN-claude-code-output.txt` — kiểm chứng file kết quả bằng openpyxl/zipfile.
- `testN-axiom-calls.txt` / `testN-axiom-output.txt` — lệnh đã gọi và tài liệu để lại.
- `testN-gaps.md` — bảng khoảng cách kèm dẫn chứng và việc đã làm.

Transcript gốc: `~/.claude/projects/C--Users-ThuyetMT-test-bench-testN/<session>.jsonl`.
Log Axiom: `C:\Users\ThuyetMT\test\bench\axiom-testN.log` (SSE nguyên văn).
File kết quả Axiom: `C:\Users\ThuyetMT\test\bench\result\axiom-testN.ods`.
