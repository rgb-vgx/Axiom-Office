# Đối chiếu benchmark LibreOffice Calc với Claude Code

Bộ đề: `libre_calc_agent_extreme_benchmark` (4 bài, mỗi bài 100 điểm). Mục đích không phải chấm điểm
Axiom mà là **tìm khoảng cách**: cùng một đề, Claude Code làm được gì, Axiom Core làm được gì, và vì sao
khác nhau — kèm bằng chứng lấy từ log/transcript chứ không phải suy đoán.

## Chạy Claude Code trên một bài

Đặt mỗi bài vào thư mục riêng để transcript và file kết quả không lẫn nhau:

```bash
mkdir -p "C:/Users/ThuyetMT/test/bench/test1"
cd "C:/Users/ThuyetMT/test/bench/test1"
cat "C:/Users/ThuyetMT/test/benchmark/test1.txt" | claude \
    --settings "C:\Users\ThuyetMT\.claude\settings.proxy.json" \
    --allowedTools Bash Write Edit Read Glob Grep -p
```

- `-p` đọc đề từ stdin; truyền thẳng đề bằng tham số dòng lệnh hay bị rỗng.
- Đường dẫn file đề và file settings phải để trong nháy kép: tên có dấu ngoặc.
- Phiên chết giữa đường vì lỗi gateway (503) thì chạy tiếp bằng `--continue` với một câu ngắn, **đừng**
  giao lại cả đề — nó sẽ làm lại từ đầu.

## Đọc lại transcript

Transcript nằm ở `~/.claude/projects/<cwd-đổi-thành-gạch-ngang>/<session>.jsonl`.

```bash
python tests/bench/summarize.py "C:/Users/ThuyetMT/.claude/projects/C--Users-ThuyetMT-test-bench-test1/<session>.jsonl"
```

Cần ghi lại **cách làm**, không chỉ kết quả: gọi tool gì, dựng công thức ra sao, kiểm tra lại thế nào,
chỗ nào phải quay lại sửa. Ví dụ Test 1 (02/10/2026): 88 tool call (Bash 50, Write 17, Edit 12, Read 9),
5 script `probe*.py` để dò API trước, rồi `build_model.py` (sửa 11 lần) dựng cả workbook qua UNO, sau đó
`postprocess.py` viết lại OOXML cho phần API từ chối, rồi `verify.py`/`final_check.py` tự soát và sửa.

## Chạy Axiom Core trên cùng bài

Axiom làm việc trên tài liệu **đang mở**, nên phải mở LibreOffice thật (không mô phỏng bridge):

```bash
"/c/Program Files/LibreOffice/program/soffice.exe" --calc &      # cửa sổ mới, sheet trống
src/AxiomOffice/bin/Release/AxiomOffice.Core.exe &
python tests/bench/run_prompt.py <core_port> <bridge_port> <pid> <prompt_file> <log_file> <token>
```

`run_prompt.py` gửi đề qua đúng `POST /v1/runs` mà pane dùng và ghi lại toàn bộ SSE.

Cuối mỗi lượt, harness in sẵn các dòng `METRIC` để so A/B mà không phải mở log SSE ra đếm tay:
`billable_in` / `billable_out` / `billable` (input − cache + output), `cache_hit`, `by_tool`
(histogram từng lệnh), `reads` / `dup_reads` / `dup_ratio` (đọc lặp — chỉ là dưới chuẩn vì
`paramsPreview` cắt 200 ký tự), `failures` / `repeat_fail_max`, `time_to_first_check_ok`, `wall`.
Quy ước chạy A/B và thứ tự roadmap: `results/con-lai.md` mục 7.

## Kiểm chứng file kết quả

Không tin lời kể của agent. Mở file bằng openpyxl/zipfile và đếm.

**Đếm theo TỪNG SHEET, đừng chỉ đếm tổng.** Tổng số công thức là một thước đo tồi: báo cáo Test 3 bản
đầu ghi "181.603 ô công thức" trong khi con số đó nằm hết trong 4 sheet còn **9 sheet đề yêu cầu trống
hoàn toàn** (xem `results/test3-gaps.md`, mục Đính chính). Con số tổng che mất đúng thứ cần thấy.

```bash
python <repo>/tests/bench/ods_summary.py <file.ods>     # dong | cong thuc | o chu | chart, theo tung sheet
```

Kiểm ba tầng:

1. **Cấu trúc** — `ods_summary.py`: sheet nào có nội dung, sheet nào chỉ có tên.
2. **Công thức hay số cứng** — `formulas.py` đọc `table:formula` trong gói ODF. Đề liệt "hardcode kết quả"
   vào lỗi nghiêm trọng, mà đọc **giá trị** thì không phân biệt được.
3. **Đầu vào còn sống hay đã chết** — **đổi ô rồi đọc lại**. Đọc không bắt được ô điều khiển không nối
   vào đâu; quét công thức thì thấy (`scan.py` đếm xem có bao nhiêu công thức thật sự đọc ô đó).
