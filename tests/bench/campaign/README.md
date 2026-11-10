# Benchmark paired cho Axiom Office

Bộ công cụ nhỏ để trả lời câu hỏi: **cải tiến X có thật sự tốt hơn không?**
Thay vì so điểm trung bình của hai đợt chạy khác nhau, mỗi task được chạy **có/không có** cải tiến
trên cùng tài liệu gốc, rồi ghép cặp và so.

Thiết kế bám theo `docs/research/gpt-advice.md`: paired design, success nhị phân, cost per successful
task, median tỷ lệ theo cặp, bootstrap CI, và không kết luận từ n=3.

## 1. Công tắc benchmark trong Core

Core đọc biến môi trường `AXIOM_BENCH_VARIANT` (chỉ dùng khi benchmark; không đặt = production):

| Nhánh | Bật |
|---|---|
| `B0` | không bật gì — hành vi cũ |
| `B1` | HarnessProfile (ngân sách/chế độ theo model) |
| `B2` | mô tả lệnh gọn |
| `B3` | tách system prompt ổn định/động |
| `B4` | chèn checklist mỗi vòng |
| `B5` | giữ kết quả bị cắt + `read_result` |
| `B6` | tất cả (cấu hình tích hợp) |

Ví dụ chạy control và treatment:

```bash
AXIOM_BENCH_VARIANT=B0 AxiomOffice.Core.exe
AXIOM_BENCH_VARIANT=B2 AxiomOffice.Core.exe
```

Mỗi lần chạy nên dùng **cùng commit, cùng model, cùng fixture, cùng prompt**. Variant hiện được ghi
vào `DebugTrace` (`benchVariant`) để đối chiếu.

## 2. Ghi một lần chạy

Mỗi dòng `runs.jsonl` là một lần thực thi một task:

```bash
python tests/bench/campaign/record.py --out runs.jsonl \
  --run-id run-001 --pair p001 --task W03 \
  --variant B0 --role control --success S3 \
  --billable 12000 --input-tokens 15000 --cached-tokens 9000 \
  --tool-calls 8 --rounds 5 --wall 95 \
  --app word --model "provider:model" --commit "$(git rev-parse --short HEAD)"
```

Cặp `p001` cần thêm một bản ghi treatment cùng `pair_id` và `task_id`:

```bash
python tests/bench/campaign/record.py --out runs.jsonl \
  --run-id run-002 --pair p001 --task W03 \
  --variant B2 --role treatment --success S3 \
  --billable 9000 --input-tokens 12000 --cached-tokens 9000 \
  --tool-calls 7 --rounds 4 --wall 85
```

Các trường chính:

| Trường | Ý nghĩa |
|---|---|
| `pair_id` | cặp control/treatment; bắt buộc đúng 1 control (B0) + 1 treatment |
| `task_id` | mã task, ví dụ `W03`, `E05`, `P03` |
| `success` | `S0` fail, `S1` partial, `S2` đủ nhưng có lỗi nhỏ, `S3` đúng |
| `failure` | `model`, `agent`, `document`, `infra`… để tách loại lỗi |
| `billable` | token phải trả thật (input − cached + output) |
| `tool_calls`, `invalid_tool_calls`, `duplicate_reads`, `rounds` | hiệu quả/thao tác |
| `context_overflow` | lượt vượt cửa sổ ngữ cảnh |
| `read_result_calls`, `unnecessary_read_result` | dùng/ lạm dụng `read_result` |

Có thể truyền nguyên một JSON bằng `--json '{"run_id":...}'` nếu script khác sinh ra.

## 3. Báo cáo

```bash
python tests/bench/campaign/report.py runs.jsonl --out report.md
python tests/bench/campaign/report.py runs.jsonl --success-level S2 --bootstrap 3000
```

Báo cáo gồm:

- Bảng theo nhánh: số cặp/task, success control vs treatment, Δ success, McNemar exact, cost/success,
  median giảm billable kèm CI bootstrap, gợi ý KEEP/REJECT/HOLD.
- Bảng metric phụ: input/cached/tool call/vòng/đọc trùng/`read_result`.
- Bảng theo task để thấy task nào bị regression.

Quy ước:

- Success chính chỉ tính **S3** (đổi bằng `--success-level`).
- Token/tool call chỉ tính trên cặp **cả hai thành công**, để so lượng token cho cùng một việc đã xong.
- Cost per successful task lấy tổng billable / số task thành công; task thất bại vẫn vào tử số.
- Bootstrap resample theo **cặp**, không theo từng tool call.
- CI quá rộng → HOLD, không phải KEEP.

## 4. Cách chạy một đợt nhỏ

Gợi ý tối thiểu (theo khuyến nghị trong `gpt-advice.md`):

1. **P0**: đóng băng commit baseline, chọn 8–12 task, xác nhận reset fixture + validator chấm đúng.
2. **P1 screening**: mỗi task chạy 3 cặp (B0 + một nhánh), đảo thứ tự giữa các cặp.
3. **P2 targeted**: thêm ca chuyên biệt (optional param, context 8k/32k, 10+ bước, kết quả bị cắt).
4. **P3 confirmation**: nhánh có tín hiệu tốt chạy 8–12 cặp/task trước khi bật mặc định.

Không chạy toàn bộ control trước rồi mới chạy treatment: cache của provider và model drift sẽ làm
treatment hưởng lợi (hoặc chịu thiệt) không do cải tiến.

## 5. Chống nhiễu

- Mỗi lượt khôi phục **golden fixture**; xác nhận hash trước khi mở app.
- Control và treatment cùng máy, cùng model, cùng ngày; đảo thứ tự theo block.
- Với thay đổi prompt cache: tách **cold cache** và **warm cache** thành hai thí nghiệm.
- Timeout/empty reply: tính là thất bại của agent; lỗi hạ tầng độc lập báo riêng, không âm thầm loại.
- Không chạy lại đến khi thành công rồi chỉ ghi lần cuối.

## 6. Kiểm thử bộ công cụ

```bash
python -m pytest tests/bench/campaign/test_campaign.py -q
```
