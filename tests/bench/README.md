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

## Kiểm chứng file kết quả

Không tin lời kể của agent. Mở file bằng openpyxl/zipfile và đếm: tên sheet, số ô công thức, số chart
part, số khối `conditionalFormatting`, số `dataValidation`, ô lỗi.
