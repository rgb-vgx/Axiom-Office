> Ghi chú: phản hồi tham khảo từ GPT (11/11/2026), dùng để thiết kế hạ tầng benchmark trong
> `tests/bench/campaign/`. Không phải quy trình đã chốt; các ngưỡng và con số cần kiểm lại với repo.

# User ask:
Bạn là chuyên gia thiết kế benchmark cho AI agent làm việc với tài liệu văn phòng.
Hãy giúp tôi thiết kế một kế hoạch benchmark hợp lý, có thể chạy thật, cho sản phẩm dưới đây.
Trả lời bằng tiếng Việt; càng cụ thể càng tốt (bảng, thứ tự chạy, cỡ mẫu, metric, ngưỡng giữ/bỏ).

## 1. Sản phẩm

Axiom Office là agent sửa trực tiếp tài liệu đang mở trong Word/Excel/PowerPoint (Windows COM)
và WPS/LibreOffice (Linux). Người dùng chính là người không chuyên, chạy model free/yếu
(ví dụ mimo-v2.6-flash-free, deepseek-v4.1-flash, beam-free), thường có cửa sổ ngữ cảnh 8k–128k,
đôi khi bị timeout 120 giây/request hoặc trả lời rỗng.

Kiến trúc:
- Agent Core viết bằng Go (`core-go/`), một tool duy nhất `office_action {action, params}`;
  `action` là lệnh bridge như `writer.replaceMany`, `et.readRange`, `wpp.checkLayout`.
- Catalog 115 lệnh; mỗi app chỉ thấy lệnh của app mình (Word ~37, Excel ~31, PowerPoint ~17).
- Có guard: PlanRule, StopRule, docStateGuard (chặn đọc lặp khi tài liệu không đổi),
  verifyGuard (bắt đọc lại sau khi sửa), sameTaskGuard/heavyRunGuard (chặn loop/đốt token).
- Có memory dài hạn, skills 3 tầng (SKILL.md), MCP, web search, DebugTrace từng vòng.
- Kết quả tool lớn bị cắt đầu+đuôi; `et.readRange` có `nextRange` để đọc tiếp.

## 2. Benchmark hiện có

- `tests/bench/` (LibreOffice Calc): 4 bài; có A/B trước đây n=3/cặp, model mimo free:
  variant rẻ hơn 3/3, median −11,4% billable token, −24,2% tool call, wall −9,9%;
  điểm yếu là chart (control 3/3 lượt có chart, variant 1/3).
- `tests/bench/office`: 22 ca (10 Word, 10 Excel) + 4 ca P3 → 26 ca.
  Baseline `ocg/beam-free`: đạt 13/22 (Word 6/11, Excel 7/11).
  Sau P0–P3 mới đo từng nhóm (6/9, 6/6, 6/7, 9/17…), CHƯA đo trọn bộ 26 ca × 3 lần.
- `tests/bench/slides`: 24 brief + 2 ca "cài sẵn lỗi" (s25, s26); có score.py chấm cấu trúc.
  Bài 10 slide thật: 11–45 vòng, 156k–1,3M token vào, 128–973 giây/lượt.
- `tests/live/`: chạy task thật trên app thật; `tests/core/test_core_e2e.py`: 180 ca e2e giả lập.
- Metric đã có: `METRIC` (billable, by_tool, dup_reads, wall), `run_rounds` trong core.db
  (input/cached/output token, số tool call, loại vòng), `Verified`, DebugTrace JSONL.
- Chấm điểm: XML/COM cho cấu trúc tài liệu; một số bài dùng score.py; chưa có LLM judge thống nhất.

## 3. Bốn thay đổi vừa làm, cần benchmark

1) HarnessProfile theo model
   - Model cửa sổ ≤ 32k tự nhận ngân sách lượt trước nhỏ hơn (8k → 2.000; 32k → 4.000 token)
     và mô tả lệnh gọn; có thể đăng ký profile riêng theo `provider:model`.
   - Giả thuyết: giảm token vào, giảm tràn ngữ cảnh, không giảm tỉ lệ thành công.
   - Rủi ro: ngân sách nhỏ làm mất ngữ cảnh cần thiết; mô tả gọn làm model gọi sai tham số.

2) Mô tả lệnh gọn (compact tool descriptions)
   - Bỏ hint dài trong chữ ký lệnh, giữ tên tham số.
   - Đo trên catalog: Word ~12,7k → ~4,1k ký tự; Excel ~10,7k → ~3,7k; PowerPoint ~6,2k → ~2,1k.
   - Giả thuyết: giảm ~1k token tiền tố mỗi vòng; rủi ro: tăng lỗi tham số/thiếu optional param.

3) Tách system prompt ổn định/động
   - Phần ổn định (Base + quy tắc + skill) đứng trước và được đánh dấu cache (Anthropic 2 block,
     chỉ block đầu có cache_control); phần động (tài liệu/memory/hội thoại) đứng sau.
   - Giả thuyết: tăng cache hit, giảm billable token; rủi ro: khác hành vi giữa provider,
     thay đổi thứ tự prompt ảnh hưởng chất lượng.

4) Chèn lại checklist mỗi vòng
   - Khi `write_todos` đổi, chèn một câu `[state] Checklist 1/3 done; doing: ...` vào trước vòng kế.
   - Giả thuyết: model yếu quên bước ít hơn, giảm vòng lặp; rủi ro: tốn token, model coi đó là yêu cầu mới.

5) Giữ kết quả bị cắt + tool `read_result`
   - Kết quả bị truncate được giữ nguyên văn trong bộ nhớ của lượt (4 MB/kết quả, 16 MB/lượt);
     kết quả trả về có `resultId` + `readMore`; model gọi `read_result {id, offset, limit}` để đọc tiếp.
   - Giả thuyết: giảm gọi lại bridge/đọc trùng, tăng chất lượng task tài liệu lớn; rủi ro:
     thêm một tool, model lạm dụng hoặc không dùng, tốn bộ nhớ.

## 4. Điều kiện thực tế

- Có 1 máy Windows 10 (Office 2019 x64) và 1 máy Ubuntu 24.04 (WPS/LibreOffice).
- Model free: hay timeout, empty reply, có thể đổi hành vi theo thời gian/gateway.
- Không thể chạy quá nhiều lượt; mỗi lượt task thật có thể 2–15 phút.
- Kết quả cần đủ tin để quyết định giữ hay bỏ từng thay đổi, không cần bài báo khoa học.

## 5. Câu hỏi cần trả lời

1. Thiết kế tổng thể: nên A/B từng thay đổi một (one-factor-at-a-time), hay factorial 2^k,
   hay chạy tuần tự từng nhóm? Với 5 thay đổi và model free nhiễu, cách nào cân bằng công sức/độ tin?
2. Bộ đề: nên dùng những ca nào trong 3 bộ hiện có (Calc 4 bài, Office 26 ca, Slides 24 brief)?
   Có cần thêm ca chuyên biệt cho từng thay đổi không (ví dụ ca cần optional param cho compact,
   ca đọc bảng 50k dòng cho read_result, ca 10+ bước cho checklist, ca context 8k cho budget)?
3. Cỡ mẫu: với tỉ lệ thành công nhị phân và metric token lệch mạnh, cần bao nhiêu lượt mỗi nhánh?
   Nên dùng paired design (cùng task, cùng seed) hay chạy độc lập? Có nên bootstrap CI / Wilcoxon /
   mixed-effects không, và báo cáo thế nào cho người không chuyên?
4. Chống nhiễu: cách xử lý cache của provider giữa các lượt (cache nóng làm sai metric token),
   thứ tự chạy, warm-up, reset `core.db`/app, model drift, timeout/empty reply.
   Lỗi hạ tầng nên loại khỏi mẫu hay tính riêng?
5. Metric: đâu là primary (thành công nhị phân, token/thành công, tool call/thành công, wall)?
   Secondary gồm gì (cached token, invalid tool call, context overflow, số lần đọc lại, số vòng)?
   Cách chấm "thành công" cho task mở (báo cáo, slide) mà không phụ thuộc LLM judge quá nhiều?
6. Tiêu chí giữ/bỏ: đặt ngưỡng cụ thể cho từng thay đổi (ví dụ giữ compact nếu token giảm ≥15%
   mà success không giảm quá X điểm; bỏ nếu invalid param tăng >Y%). Đề xuất ngưỡng và cách kiểm.
7. Lịch chạy: chia pha thế nào để có kết quả sớm (pilot 1 ngày, rồi mở rộng), ước lượng số lượt
   và thời gian cho từng pha.
8. Cạm bẫy: những lỗi thiết kế benchmark thường gặp với agent tài liệu + model free mà tôi nên tránh.

## 6. Định dạng trả lời mong muốn

- Một kế hoạch theo pha, có bảng: pha | câu hỏi | bộ đề | số lượt/nhánh | metric chính | thời gian ước tính.
- Chỉ rõ mỗi thay đổi nên đo bằng ca nào, so với baseline nào.
- Công thức/ngưỡng quyết định giữ-bỏ từng thay đổi.
- Danh sách việc cần chuẩn bị trước khi chạy (script, log, reset môi trường).
- Cảnh báo những kết luận sẽ KHÔNG đáng tin nếu chỉ chạy n nhỏ.
Đừng trả lời chung chung; hãy đưa ra lựa chọn cụ thể và nói rõ vì sao.

## GPT reply:
# Kế hoạch benchmark Axiom Office: đánh giá 5 thay đổi ở cấp production

## 1. Kết luận và lựa chọn thiết kế

Tôi đề xuất Sequential Paired Ablation – benchmark tuần tự theo từng thay đổi, kết hợp paired design và một vòng kiểm tra tương tác có chọn lọc.

Không nên chạy factorial \\(2^5 = 32\\) cấu hình ngay từ đầu. Cũng không nên chỉ chạy một bộ benchmark tổng hợp rồi so sánh điểm trung bình.

Lý do:

1. Model free có biến động lớn giữa các thời điểm, gateway và provider. Chạy 32 cấu hình sẽ khiến nhiễu theo thời gian lấn át hiệu ứng của thay đổi.
2. Năm thay đổi có bản chất khác nhau: một số tối ưu chi phí, một số tác động đến khả năng hoàn thành task, một số thay đổi cơ chế truyền dữ liệu.
3. Có thể đo tương đối độc lập bốn thay đổi đầu, nhưng `compact tool descriptions` và `HarnessProfile` có khả năng tương tác.
4. Với chi phí mỗi lượt 2–15 phút, cần ưu tiên các phép thử có khả năng bác bỏ giả thuyết nhanh.



Khuyến nghị

&#x20;Chạy theo 3 tầng:



- Tầng 1 – Screening: 3 cặp/task cho các thay đổi, tìm regression lớn.
- Tầng 2 – Confirmation: 8–12 cặp/task cho các thay đổi vượt qua screening.
- Tầng 3 – Integration: chạy cấu hình kết hợp trên bộ đề đại diện để xác nhận không có tương tác xấu.

Một nguyên tắc quan trọng: không dùng số token giảm để kết luận hệ thống tốt hơn nếu tỷ lệ hoàn thành task giảm.

Thứ tự ưu tiên đánh giá:

1. Correctness – tài liệu sau khi sửa có đúng không?
2. Completion – agent có hoàn thành toàn bộ yêu cầu không?
3. Efficiency – cần bao nhiêu token, tool call và thời gian?
4. Reliability – có ổn định qua nhiều lần chạy và nhiều điều kiện không?

Đơn vị benchmark phải là một lần thực thi task trên một trạng thái tài liệu xác định, không phải một vòng agent hoặc một tool call.

## 2. Thiết kế thực nghiệm tổng thể

### 2.1. Vì sao không chọn factorial 2⁵?

| Phương pháp                      | Ưu điểm                                     | Nhược điểm                                | Đánh giá   |
| -------------------------------- | ------------------------------------------- | ----------------------------------------- | ---------- |
| One-factor-at-a-time             | Dễ giải thích, dễ tìm regression            | Không phát hiện tương tác                 | Chưa đủ    |
| Full factorial 2⁵                | Phát hiện tương tác giữa các thay đổi       | 32 cấu hình, quá tốn lượt                 | Không chọn |
| Sequential paired ablation       | Ít lượt, dễ quy kết nguyên nhân             | Cần thiết kế thứ tự và kiểm tra tương tác | Chọn       |
| Sequential + selective factorial | Kiểm tra tương tác quan trọng sau screening | Tốn thêm một ít lượt                      | Tốt nhất   |

Tôi đề xuất không chạy 5 thay đổi độc lập hoàn toàn. Cần chú ý các quan hệ:

Ma trận tương tác cần kiểm tra

| Cặp thay đổi                           | Mức độ tương tác | Lý do                                                 |
| -------------------------------------- | ---------------- | ----------------------------------------------------- |
| HarnessProfile × Compact descriptions  | Cao              | Cùng làm giảm lượng thông tin model nhận được.        |
| Stable/dynamic prompt × HarnessProfile | Trung bình       | Thay đổi ngân sách và cấu trúc ngữ cảnh.              |
| Checklist × HarnessProfile             | Trung bình       | Checklist tiêu thụ ngân sách lượt trước.              |
| read_result × Compact descriptions     | Cao              | Model cần hiểu đúng tool mới và tham số offset/limit. |
| Checklist × Stable/dynamic prompt      | Thấp             | Có thể ảnh hưởng cache nhưng ít thay đổi semantics.   |

Đặc biệt, nếu compact descriptions và HarnessProfile đều có hiệu quả riêng nhưng kết hợp khiến model gọi sai tham số, benchmark từng thay đổi riêng sẽ không phát hiện được.

### 2.2. Định nghĩa các nhánh thử nghiệm

Dùng một baseline cố định và mỗi lần chỉ thay đổi một yếu tố.

| Nhánh | HarnessProfile | Compact | Prompt cache | Checklist | read_result |
| ----- | -------------- | ------- | ------------ | --------- | ----------- |
| B0    | Cũ             | Không   | Cũ           | Không     | Không       |
| B1    | Mới            | Không   | Cũ           | Không     | Không       |
| B2    | Cũ             | Có      | Cũ           | Không     | Không       |
| B3    | Cũ             | Không   | Mới          | Không     | Không       |
| B4    | Cũ             | Không   | Cũ           | Có        | Không       |
| B5    | Cũ             | Không   | Cũ           | Không     | Có          |
| B6    | Mới            | Có      | Mới          | Có        | Có          |

B0–B5 phục vụ ablation. B6 là cấu hình tích hợp, không được dùng để quy kết hiệu quả cho một thay đổi riêng lẻ.

Điều kiện bắt buộc: đóng băng cùng một commit của agent, model ID, tool schema, skill, memory, task prompt và tài liệu đầu vào giữa hai nhánh. Nếu thay đổi khác nhau ngoài biến đang thử, phép so sánh không còn hợp lệ.

### 2.3. Paired design

Mỗi task được chạy trên hai bản sao tài liệu giống nhau:

- Control: code baseline.
- Treatment: code có đúng một thay đổi.

Ghép cặp theo `task_id`, model, máy, thời điểm và điều kiện chạy.

Không nên dựa vào cùng seed nếu API model không hỗ trợ seed deterministic. Hãy dùng cùng task và cùng prompt; seed chỉ là một biến kiểm soát bổ sung nếu thực sự được hỗ trợ.

Ví dụ:

| Pair | Task | Control | Treatment |
| ---- | ---- | ------- | --------- |
| P001 | W03  | B0      | B1        |
| P002 | W03  | B1      | B0        |
| P003 | E05  | B0      | B1        |
| P004 | E05  | B1      | B0        |

Đảo thứ tự giữa các cặp để giảm ảnh hưởng của provider cache, tải hệ thống và model drift.

Không chạy toàn bộ control trước rồi mới chạy treatment.

## 3. Bộ đề benchmark

Tôi không khuyến nghị dùng toàn bộ 52 ca hiện tại cho mọi thay đổi. Cách đó lãng phí lượt và làm loãng tín hiệu.

Nên chia bộ đề thành ba nhóm:

- Core regression: phát hiện task thông thường bị hỏng.
- Targeted stress: kiểm tra trực tiếp giả thuyết của từng thay đổi.
- Integration suite: kiểm tra toàn bộ cấu hình sau khi kết hợp.

### 3.1. Phân bổ bộ đề hiện tại

| Bộ đề           | Hiện trạng                             | Vai trò                                | Có chạy đầy đủ?                             |
| --------------- | -------------------------------------- | -------------------------------------- | ------------------------------------------- |
| Calc 4 bài      | Đã có A/B                              | Regression nhanh, đo hiệu suất Excel   | Có, ưu tiên cao                             |
| Office 26 ca    | 10 Word + 10 Excel + 4 P3 (theo mô tả) | Core regression                        | Chọn 12–16 ca trước                         |
| Slides 24 brief | Có score.py                            | Kiểm tra chất lượng tạo slide          | Chọn 4–6 brief                              |
| Slides s25–s26  | Cài sẵn lỗi                            | Kiểm tra khả năng phát hiện và sửa lỗi | Có                                          |
| E2E 180 ca      | Giả lập                                | Kiểm tra logic guard/core              | Chạy toàn bộ, không thay thế live benchmark |
| Live tests      | App thật                               | Xác nhận production behavior           | Chọn lọc                                    |

Lưu ý: số liệu baseline 13/22 và 6/9, 6/6, 6/7, 9/17 hiện chưa tạo thành một baseline đủ chuẩn để so sánh treatment. Cần xác nhận chính xác tập task và phiên bản code trong từng báo cáo. Không được ghép các mẫu có cấu hình hoặc tập đề khác nhau để tạo một tỷ lệ tổng hợp.

### 3.2. Core regression suite đề xuất

Tôi đề xuất xây dựng một tập cố định gồm 16 task:

| ID  | Ứng dụng   | Loại task           | Đặc điểm cần kiểm tra          |
| --- | ---------- | ------------------- | ------------------------------ |
| W01 | Word       | Replace text        | Sửa chính xác nhiều vị trí     |
| W02 | Word       | Formatting          | Định dạng đoạn và heading      |
| W03 | Word       | Table               | Chỉnh sửa bảng                 |
| W04 | Word       | Multi-step          | Sửa nội dung + format + verify |
| W05 | Word       | Long document       | Đọc tài liệu nhiều trang       |
| W06 | Word       | Optional parameters | Dùng lệnh có optional param    |
| E01 | Excel      | Read/write range    | Đọc và ghi vùng dữ liệu        |
| E02 | Excel      | Formula             | Sửa công thức                  |
| E03 | Excel      | Formatting          | Định dạng bảng                 |
| E04 | Excel      | Chart               | Tạo/chỉnh chart                |
| E05 | Excel      | Large range         | Đọc dữ liệu lớn                |
| E06 | Excel      | Multi-step          | Đọc, tính toán, sửa, xác minh  |
| P01 | PowerPoint | Basic edit          | Sửa nội dung slide             |
| P02 | PowerPoint | Layout              | Sửa layout                     |
| P03 | PowerPoint | Multi-slide         | Sửa nhiều slide                |
| P04 | PowerPoint | Repair              | Sửa deck có lỗi cài sẵn        |

Các task này nên được chọn từ bộ hiện có khi có thể, không cần viết lại toàn bộ. Đặc biệt, giữ lại bài chart vì benchmark trước đã phát hiện regression nghiêm trọng.

Không được thay thế task chart bằng task formatting đơn giản chỉ vì task chart khó chạy.

### 3.3. Các ca chuyên biệt cần bổ sung



Nên bổ sung

&#x20;Năm thay đổi có những failure mode mà bộ đề hiện tại chưa chắc đã bao phủ.



| Thay đổi              | Ca chuyên biệt                                                       | Quy mô |
| --------------------- | -------------------------------------------------------------------- | ------ |
| HarnessProfile        | Task cần giữ ngữ cảnh qua nhiều vòng, context budget 8k/32k          | 2      |
| Compact descriptions  | Lệnh có nhiều optional parameter, kiểu dữ liệu và tham số dễ nhầm    | 2      |
| Stable/dynamic prompt | Task lặp cùng system prefix nhưng dynamic context thay đổi           | 2      |
| Checklist             | Task có 8–12 bước, yêu cầu thực hiện tuần tự                         | 2      |
| read_result           | Kết quả đọc lớn bị truncate, phải tiếp tục đọc để lấy thông tin cuối | 2      |

Tổng cộng: 10 ca targeted stress.

Có một lưu ý quan trọng: không nên tạo task chỉ để chứng minh tính năng hoạt động. Ví dụ với `read_result`, cần có ít nhất hai bài:

1. Model phải thực sự cần phần bị truncate để hoàn thành task.
2. Model không cần đọc thêm nhưng kết quả vẫn bị truncate, nhằm phát hiện model lạm dụng `read_result`.

Với `compact descriptions`, cần có bài sử dụng optional parameter thực sự ảnh hưởng đến kết quả, không chỉ bài mà bỏ optional param vẫn hoàn thành được.

Với `HarnessProfile`, cần phân biệt:

- Model có cửa sổ 8k nhưng task chỉ cần 2k context.
- Model có cửa sổ 8k nhưng task cần 4–6k context.
- Model có cửa sổ 32k nhưng task cần giữ thông tin qua nhiều lượt.

Nếu chỉ thử task nhỏ, bạn sẽ không phát hiện regression do ngân sách bị giảm quá mạnh.

## 4. Cỡ mẫu và phương pháp thống kê

Đây là phần quan trọng nhất để tránh đưa ra quyết định sai từ vài lượt chạy.

### 4.1. Tỷ lệ thành công: không dùng n=3 để kết luận

Giả sử baseline có success rate 60%.

Nếu chạy 3 lần và đạt 3/3, điều đó không chứng minh thay đổi mới có success rate 100%.

Với mẫu nhỏ, khoảng tin cậy rất rộng. Chẳng hạn, 3/3 thành công vẫn tương thích với một tỷ lệ thành công thực tế thấp hơn nhiều.

Tôi đề xuất cỡ mẫu theo ba cấp:

| Cấp               | Số cặp/task | Tổng lượt/task (2 nhánh) | Mục đích                              |
| ----------------- | ----------- | ------------------------ | ------------------------------------- |
| Screening         | 3           | 6                        | Phát hiện regression rõ rệt           |
| Confirmation      | 8           | 16                       | Kiểm tra hiệu ứng có lặp lại          |
| Stronger evidence | 12          | 24                       | Ra quyết định có độ tin cậy tốt hơn   |
| Extended          | 20          | 40                       | Task quan trọng hoặc kết quả bất đồng |

Với một thay đổi chạy trên 8 task, confirmation tương đương 128 lần thực thi.

Không nhất thiết mọi task đều cần 12 cặp. Có thể phân bổ:

- 4 task trọng yếu: 12 cặp.
- 4 task thông thường: 8 cặp.
- 2 task targeted stress: 8 cặp.

Tuy nhiên, phải phân biệt rõ kết luận ở cấp task và cấp toàn bộ sản phẩm.

### 4.2. Vì sao không thể chỉ tính số lượt tổng?

Giả sử bạn có 100 lần chạy:

- 80 lần từ Excel.
- 20 lần từ Word.

Nếu Excel chiếm phần lớn mẫu thì kết quả tổng hợp phản ánh Excel nhiều hơn, dù người dùng thực tế sử dụng Word và PowerPoint đáng kể.

Do đó cần hai cách báo cáo:

Micro-average: gộp tất cả lượt thực thi.

Macro-average: tính trung bình theo task hoặc nhóm ứng dụng, mỗi nhóm có trọng số được xác định trước.

Tôi khuyến nghị dùng macro-average làm kết quả chính và micro-average làm kết quả phụ.

### 4.3. Paired design cho từng metric

A. Success rate

Với mỗi cặp, ghi nhận:

- Both pass.
- Control pass, treatment fail.
- Control fail, treatment pass.
- Both fail.

Dùng McNemar test cho dữ liệu nhị phân paired khi có đủ số cặp bất đồng. Với mẫu nhỏ, dùng exact McNemar.

Ví dụ:

| Kết quả                      | Số cặp |
| ---------------------------- | ------ |
| Cả hai thành công            | 7      |
| Cả hai thất bại              | 1      |
| Control pass, treatment fail | 1      |
| Control fail, treatment pass | 3      |

Treatment có 10/12 success, control có 8/12 success.

Điều cần quan tâm không chỉ là chênh lệch 16,7 điểm phần trăm mà còn là số cặp treatment cứu được và số cặp treatment làm hỏng.

B. Token và tool call

Không nên dùng trung bình số học đơn thuần vì token có phân phối lệch mạnh.

Với từng cặp thành công:

\\[ R_i = \frac{Metric\_{treatment,i}}{Metric\_{control,i}} \\]

Sau đó tính:

\\[ Reduction = 1 - Median(R_i) \\]

Ví dụ:

| Pair | Control | Treatment | Reduction |
| ---- | ------- | --------- | --------- |
| P1   | 10.000  | 8.000     | 20%       |
| P2   | 12.000  | 10.000    | 16,7%     |
| P3   | 20.000  | 15.000    | 25%       |
| P4   | 8.000   | 7.500     | 6,25%     |

Dùng median của paired reduction và bootstrap CI.

C. Wall time

Wall time cũng lệch mạnh. Báo cáo:

- Median.
- P90.
- Paired median ratio.
- Timeout rate.

Không lấy average wall time làm metric duy nhất.

### 4.4. Có nên dùng Wilcoxon, bootstrap, mixed-effects?

| Phương pháp          | Có dùng?                 | Mục đích                                  |
| -------------------- | ------------------------ | ----------------------------------------- |
| Exact McNemar        | Có                       | So sánh success rate paired               |
| Bootstrap CI         | Có                       | Khoảng tin cậy của token/time reduction   |
| Wilcoxon signed-rank | Có điều kiện             | So sánh metric paired liên tục            |
| Mann–Whitney U       | Không ưu tiên            | Không khai thác cấu trúc paired           |
| Mixed-effects        | Chưa cần ở giai đoạn đầu | Phân tích nhiều task, model và môi trường |
| Permutation test     | Có thể                   | Kiểm định chênh lệch paired khi mẫu nhỏ   |

Với bootstrap, nên resample theo `task_id` hoặc cặp task, không resample từng tool call độc lập.

Một lưu ý: nếu chỉ có 3 task và mỗi task chạy 20 lần, bạn vẫn chỉ có 3 task độc lập. 60 lượt chạy không tương đương 60 loại tình huống khác nhau.

Mixed-effects model có thể hữu ích về sau:

\\[ Y\_{ij} = \beta_0 + \beta_1 Treatment\_{ij} + u\_{task_i} + u\_{model_j} + \epsilon\_{ij} \\]

Nhưng hiện tại tôi không khuyến nghị đưa mixed-effects vào điều kiện bắt buộc để ra quyết định. Thiết kế paired, bảng chênh lệch và bootstrap là đủ thực dụng.

### 4.5. Quy tắc báo cáo thống kê

Mỗi thay đổi cần có một bảng:

| Metric         | Baseline | Treatment | Chênh lệch | 95% CI | Kết luận      |
| -------------- | -------- | --------- | ---------- | ------ | ------------- |
| Success        | 75%      | 79%       | +4 pp      | ...    | Chưa kết luận |
| Input token    | 12k      | 9k        | -25%       | ...    | Có lợi        |
| Tool calls     | 8        | 7         | -12,5%     | ...    | Có lợi        |
| Wall time      | 75s      | 70s       | -6,7%      | ...    | Có lợi        |
| Invalid params | 1%       | 3%        | +2 pp      | ...    | Cần điều tra  |

Không được ghi `PASS` chỉ vì p-value lớn hơn 0,05. Không có ý nghĩa thống kê không đồng nghĩa với hai cấu hình tương đương.

## 5. Metric: định nghĩa chính xác

Tôi đề xuất chia thành bốn lớp.

### 5.1. Primary metrics

| Metric                   | Định nghĩa                                               | Vai trò        |
| ------------------------ | -------------------------------------------------------- | -------------- |
| Task Success Rate        | Số task đạt toàn bộ acceptance criteria / số task hợp lệ | Primary        |
| Correctness Score        | Mức độ chính xác của kết quả tài liệu                    | Primary        |
| Cost per Successful Task | Billable token của tất cả lượt / số task thành công      | Primary        |
| Reliability              | Timeout, empty reply, crash, partial completion          | Primary safety |

Tại sao chọn Cost per Successful Task thay vì token trung bình?

Vì một agent tiêu thụ 5.000 token nhưng thất bại không tốt hơn agent tiêu thụ 10.000 token nhưng hoàn thành chính xác.

Công thức:

\\[ CPS = \frac{\sum\_{i=1}^{N} BillableToken_i} {\sum\_{i=1}^{N} Success_i} \\]

Trong đó task thất bại vẫn đóng góp token vào tử số.

Nếu không có task nào thành công thì CPS là vô hạn hoặc `N/A`, không được loại các task thất bại ra khỏi phép tính.

### 5.2. Secondary metrics

| Metric              | Định nghĩa                          | Đặc biệt hữu ích cho    |
| ------------------- | ----------------------------------- | ----------------------- |
| Input tokens        | Tổng input token                    | HarnessProfile, compact |
| Output tokens       | Tổng output token                   | Checklist               |
| Cached input tokens | Token được provider tính cache      | Prompt separation       |
| Cache hit ratio     | Cached input / eligible input       | Prompt separation       |
| Tool calls/task     | Tổng số lần gọi tool                | Tất cả                  |
| Invalid tool calls  | Gọi sai action/params               | Compact, read_result    |
| Duplicate reads     | Đọc lại dữ liệu không cần thiết     | read_result             |
| Verify compliance   | Tỷ lệ task có verify sau write      | Tất cả                  |
| Context overflow    | Lượt vượt giới hạn context          | HarnessProfile          |
| Agent rounds        | Số vòng agent                       | Checklist               |
| Empty reply         | Model trả rỗng                      | Tất cả                  |
| Truncation recovery | Tỷ lệ khôi phục được kết quả bị cắt | read_result             |
| Peak memory         | Bộ nhớ giữ kết quả                  | read_result             |
| P90 wall time       | Thời gian ở percentile 90           | Tất cả                  |

Tôi đặc biệt khuyến nghị thêm metric:

Unnecessary read_result rate

\\[ URR = \frac{\text{Số lần gọi read\\\_result không cần thiết}} {\text{Tổng số lần gọi read\\\_result}} \\]

Và:

Truncation recovery rate

\\[ TRR = \frac{\text{Task cần dữ liệu bị cắt và lấy đủ dữ liệu}} {\text{Task cần dữ liệu bị cắt}} \\]

### 5.3. Phân biệt token thực tế và token ước tính

Đối với model free, gateway có thể trả usage không đầy đủ.

Cần có ba trường riêng:

- `provider_reported_input_tokens`
- `provider_reported_cached_tokens`
- `estimated_input_tokens`

Không được cộng token ước tính và token provider báo cáo thành một con số.

Nếu gateway không trả usage, metric token phải được đánh dấu `unavailable`, không được coi bằng 0.

## 6. Chấm điểm tài liệu: không phụ thuộc LLM judge

Đây là điểm tôi muốn Axiom Office đầu tư nhiều nhất.

Axiom không chỉ trả lời câu hỏi. Nó thực sự sửa trạng thái của một tài liệu. Vì vậy, ground truth nên là trạng thái tài liệu sau khi thực thi.

### 6.1. Word

Dùng DOCX XML hoặc COM để kiểm tra:

| Nhóm         | Điều kiện                                 |
| ------------ | ----------------------------------------- |
| Content      | Văn bản đúng, không mất đoạn              |
| Structure    | Đúng số paragraph, heading, table         |
| Formatting   | Font, size, bold, alignment               |
| Table        | Đúng số hàng/cột, nội dung                |
| Preservation | Không làm hỏng nội dung ngoài phạm vi sửa |
| Verification | Đọc lại và xác nhận kết quả               |

Ví dụ task yêu cầu thay thế tên công ty trong 5 vị trí:

- Đúng 5 vị trí: pass.
- Đúng 4 vị trí: fail.
- Thay thế cả nội dung ngoài yêu cầu: fail.
- Tài liệu mở được nhưng format hỏng: fail.

### 6.2. Excel

Dùng LibreOffice UNO hoặc COM tùy môi trường:

- Cell values.
- Formula.
- Number format.
- Font/fill/border.
- Merged cells.
- Sheet count.
- Named ranges.
- Chart series.
- Chart type.
- Chart placement.
- Formula calculation result.

Không chỉ so sánh cell value. Đặc biệt với chart, cần kiểm tra cấu trúc chart object và series, không chỉ nhìn thấy biểu đồ trên màn hình.

### 6.3. PowerPoint

Dùng `python-pptx` hoặc OOXML để kiểm tra:

| Tiêu chí           | Phương pháp                      |
| ------------------ | -------------------------------- |
| Slide count        | XML                              |
| Text content       | XML                              |
| Shape count        | XML                              |
| Shape positions    | Bounding boxes                   |
| Font size          | XML                              |
| Image presence     | XML/media                        |
| Layout overlap     | Geometry                         |
| Text overflow      | Render + kiểm tra bounding boxes |
| Visual consistency | Screenshot comparison            |

Với slide, structural score không đủ.

Một slide có đủ 5 shape nhưng tất cả nằm chồng lên nhau vẫn phải thất bại.

Tôi đề xuất mỗi bài có hai score:

\\[ Score = 0.6 \times Structural + 0.4 \times Visual \\]

Trong đó:

- Structural: 60%.
- Visual: 40%.

Có hard-fail criteria:

- Thiếu slide bắt buộc.
- Mất nội dung quan trọng.
- Shape nằm ngoài canvas nghiêm trọng.
- Text bị cắt mất phần quan trọng.
- File không mở được.

Các bài slide nên có rubric chấm thủ công 2 người đối với chất lượng nội dung và thẩm mỹ, chỉ cần áp dụng cho một số bài đại diện. Không nên dùng LLM judge làm nguồn quyết định duy nhất.

### 6.4. Phân loại success

Tôi đề xuất bốn mức:

| Mức | Tên                   | Ý nghĩa                                    |
| --- | --------------------- | ------------------------------------------ |
| S0  | Failed                | Không hoàn thành hoặc tài liệu lỗi         |
| S1  | Partial               | Có sửa nhưng thiếu yêu cầu                 |
| S2  | Complete with defects | Đủ nội dung nhưng có lỗi nhỏ               |
| S3  | Correct               | Hoàn thành đúng, không có lỗi nghiêm trọng |

Primary success chỉ tính S3.

S2 được báo cáo riêng. Không gộp S2 vào success chỉ để tăng tỷ lệ hoàn thành.

Đối với task mở, cần có rubric được xác định trước khi chạy, gồm:

1. Required content.
2. Required structure.
3. Visual quality.
4. Preservation of existing content.
5. File validity.

Không được thay đổi acceptance criteria sau khi xem kết quả treatment.

## 7. Chống nhiễu: quy trình kiểm soát thực nghiệm

### 7.1. Provider cache

Đây là vấn đề lớn với thay đổi số 3.

Nếu control chạy trước treatment, treatment có thể hưởng lợi từ cache nóng.

Cần tách hai mục tiêu:

Experiment A – Cold cache

- Cùng một task.
- Cùng một model.
- Mỗi nhánh sử dụng prefix riêng biệt hoặc điều kiện cache không dùng chung.
- Đo input token, cached token và latency.

Experiment B – Warm cache

- Chạy nhiều lần với stable prefix giống nhau.
- Dynamic context thay đổi có kiểm soát.
- Đo cache hit ratio, billable token và thời gian.

Không nên giả định thay đổi prompt chắc chắn tạo cache hit. Cache behavior phải được xác nhận bằng dữ liệu usage từ provider.

Đặc biệt, Anthropic prompt caching và OpenAI-compatible gateway có thể có semantics khác nhau. Không dùng kết quả từ một gateway để khẳng định hiệu quả của tất cả provider.

### 7.2. Thứ tự chạy

Sử dụng block randomization.

Ví dụ:

| Block | Thứ tự              |
| ----- | ------------------- |
| 1     | Control → Treatment |
| 2     | Treatment → Control |
| 3     | Treatment → Control |
| 4     | Control → Treatment |

Trong mỗi block:

- Cùng máy.
- Cùng model.
- Cùng task.
- Cùng trạng thái tài liệu.
- Cùng cấu hình mạng nếu có thể.

Không chạy Word trên Windows vào buổi sáng và treatment trên Ubuntu vào buổi tối rồi kết luận thay đổi tốt hơn.

### 7.3. Reset môi trường

Mỗi lượt phải có trạng thái khởi đầu độc lập.

Checklist reset:

Reset trước mỗi lần thực thi

0/10 hoàn thành

Reset checklist

Khôi phục file tài liệu từ golden fixture.

Đóng tài liệu và mở lại bằng app mục tiêu.

Xác nhận đúng phiên bản Office/WPS/LibreOffice.

Xóa state của agent thuộc lượt trước.

Tạo run_id và pair_id mới.

Reset conversation/history của lượt chạy.

Đảm bảo memory fixture giống nhau.

Xác nhận catalog/tool schema đúng nhánh.

Kiểm tra core.db không chứa state gây ảnh hưởng.

Ghi nhận model ID, provider, timestamp và cấu hình timeout.

Một lưu ý về `core.db`: không nhất thiết xóa toàn bộ database mỗi lượt. Tốt hơn là dùng database hoặc namespace riêng theo run, giữ nguyên schema và migration. Điều cần reset là conversation state, run state, cache nội bộ và dữ liệu phụ thuộc lượt trước.

### 7.4. Timeout và empty reply

Không được âm thầm loại các lượt này khỏi mẫu.

Phân loại:

| Loại lỗi                         | Cách xử lý             |
| -------------------------------- | ---------------------- |
| Model timeout                    | Tính là unsuccessful   |
| Empty reply                      | Tính là unsuccessful   |
| Invalid tool call                | Tính là agent failure  |
| Agent loop                       | Tính là agent failure  |
| Office app crash do agent        | Tính là agent failure  |
| Mất điện, OS crash độc lập       | Infrastructure failure |
| API gateway outage xác nhận được | Infrastructure failure |
| Lỗi benchmark script             | Harness failure        |

Infrastructure failure phải báo cáo riêng và chạy lại theo quy tắc được xác định trước.

Không được chỉ chạy lại những lượt treatment thất bại nhưng giữ nguyên control.

Đề xuất quy tắc:

- Một cặp bị hạ tầng lỗi độc lập: loại cả cặp, chạy lại cả cặp.
- Model timeout hoặc empty reply: giữ nguyên kết quả thất bại.
- Nếu không phân biệt được gateway outage và model timeout: ghi `ambiguous_infrastructure`, không tự ý loại.
- Báo cáo tỷ lệ infrastructure failure riêng theo từng nhánh.

### 7.5. Model drift

Model free có thể thay đổi backend mà không đổi tên model.

Tôi khuyến nghị:

- Chạy control và treatment trong cùng ngày.
- Chia paired runs thành block ngắn.
- Lưu raw response và provider metadata.
- Chạy một task sentinel cố định đầu mỗi block.
- Nếu sentinel thay đổi mạnh, đánh dấu block có khả năng drift.

Sentinel không được dùng để tự động loại các lượt xấu. Nó chỉ giúp xác định biến động theo thời gian.

# 8. Tiêu chí giữ/bỏ cho từng thay đổi

Tôi đề xuất phân biệt ba quyết định:

- KEEP: đủ bằng chứng giữ.
- REJECT: regression rõ ràng hoặc không đạt mục tiêu.
- HOLD: cần thêm mẫu.

Không ép mọi thay đổi phải ra quyết định nhị phân ngay sau 3 lần.

## 8.1. HarnessProfile

Mục tiêu chính: giảm input token

Giả thuyết:

- Giảm token đầu vào.
- Giảm context overflow.
- Không làm giảm success rate.
- Không làm tăng lỗi gọi tool.

| Metric            | Ngưỡng đề xuất      |
| ----------------- | ------------------- |
| Input token       | Giảm ≥15%           |
| Success rate      | Không giảm quá 5 pp |
| Context overflow  | Không tăng          |
| Invalid tool call | Không tăng quá 2 pp |
| Wall time         | Không tăng quá 10%  |
| Correctness       | Không giảm          |

Điều kiện KEEP:

\\[ TokenReduction \geq 15\\% \\]

và

\\[ \Delta Success \geq -5pp \\]

và không có regression nghiêm trọng về correctness.

Điều kiện REJECT:

- Success giảm từ 10 pp trở lên.
- Task cần context dài bị fail có tính hệ thống.
- Invalid tool call tăng mạnh.
- Context bị mất làm sai nội dung tài liệu.

Ca bắt buộc: context 8k, 32k và task multi-round cần giữ thông tin từ các vòng trước.

Một đề xuất kỹ thuật: đừng chỉ đặt ngân sách theo cửa sổ context danh nghĩa. Hãy kiểm tra cả ngân sách còn lại thực tế:

\\[ AvailableBudget = ContextWindow - System - History - Tools - OutputReserve \\]

Nếu profile 8k dành 2.000 token cho lượt trước nhưng tổng tool descriptions và system prompt đã chiếm phần lớn context, hiệu quả có thể khác với kỳ vọng.

## 8.2. Compact tool descriptions

Mục tiêu chính: giảm chi phí schema

| Metric                      | Ngưỡng              |
| --------------------------- | ------------------- |
| Tool description characters | Giảm ≥50%           |
| Input token                 | Giảm ≥15%           |
| Success rate                | Không giảm quá 3 pp |
| Invalid parameter           | Không tăng quá 2 pp |
| Missing optional param      | Không tăng          |
| Wrong action selection      | Không tăng quá 2 pp |

KEEP nếu giảm token đủ lớn và không tăng lỗi gọi lệnh.

REJECT nếu:

- Invalid param tăng ≥5 pp.
- Task có optional parameter bắt đầu thất bại.
- Model gọi nhầm action có tính hệ thống.

Tôi đặc biệt muốn có một metric riêng:

`optional_param_omission_rate`

\\[ OPOR = \frac{\text{Số lần thiếu optional param bắt buộc theo task}} {\text{Tổng số lần gọi tool có yêu cầu optional param}} \\]

Tên metric có thể gây nhầm: đây là tham số optional về mặt schema nhưng bắt buộc theo ngữ cảnh task.

Không nên chỉ đếm invalid schema. Một request có thể hợp lệ về JSON nhưng sai về semantics.

## 8.3. Stable/dynamic prompt separation

Mục tiêu chính: cache efficiency

Đây là thay đổi cần đo riêng trên từng provider.

| Metric               | Ngưỡng                |
| -------------------- | --------------------- |
| Cached token ratio   | Tăng ≥20 pp           |
| Billable input token | Giảm ≥15%             |
| Success rate         | Không giảm quá 3 pp   |
| Output quality       | Không giảm            |
| Cache miss           | Không tăng bất thường |
| Wall time            | Không tăng quá 10%    |

KEEP nếu cache hit tăng và billable cost giảm mà correctness không giảm.

REJECT nếu:

- Prompt reorder làm model quên instruction.
- Dynamic data bị đặt sai vị trí.
- Provider không hỗ trợ semantics cache như kỳ vọng.
- Cache tăng nhưng tổng billable token không giảm.

Điểm quan trọng: nên có metric `eligible_prefix_tokens`, tức số token thực sự có thể được cache. Nếu không, cache hit ratio giữa hai prompt khác kích thước có thể gây hiểu nhầm.

Không đánh đồng cached token với token miễn phí. Cần tính chi phí theo billing thực tế của từng provider.

## 8.4. Checklist mỗi vòng

Mục tiêu chính: tăng multi-step completion

| Metric             | Ngưỡng              |
| ------------------ | ------------------- |
| Multi-step success | Tăng ≥10 pp         |
| Overall success    | Không giảm quá 3 pp |
| Tool calls         | Không tăng quá 15%  |
| Input tokens       | Không tăng quá 10%  |
| Repeated actions   | Không tăng          |
| Loop rate          | Giảm ≥20%           |

KEEP nếu tăng success ở task nhiều bước và giảm loop.

REJECT nếu:

- Model hiểu checklist như yêu cầu mới.
- Gọi lại action đã hoàn thành.
- Token tăng nhiều nhưng success không cải thiện.
- Checklist làm gián đoạn task đơn giản.

Tôi đề xuất chỉ chèn checklist khi `write_todos` thực sự thay đổi trạng thái, không chèn mỗi vòng nếu checklist không đổi.

Cần có một thử nghiệm riêng với task 10+ bước và một task ngắn 2–3 bước để tránh tối ưu multi-step nhưng làm chậm toàn bộ workload.

## 8.5. read_result

Mục tiêu chính: phục hồi dữ liệu bị truncate

| Metric                             | Ngưỡng              |
| ---------------------------------- | ------------------- |
| Truncation recovery rate           | ≥90%                |
| Duplicate bridge reads             | Giảm ≥50%           |
| Task success trên bài large result | Không giảm          |
| Unnecessary read_result            | ≤10%                |
| Memory cap violation               | 0                   |
| Tool errors                        | Không tăng quá 2 pp |

KEEP nếu:

- Có bằng chứng giảm đọc lại bridge.
- Task cần dữ liệu bị truncate có thể phục hồi.
- Không có vấn đề memory leak hoặc vượt quota.
- Model sử dụng offset/limit chính xác.

REJECT nếu:

- Model không biết dùng `read_result`.
- Gọi lặp nhiều lần cùng offset.
- Đọc tiếp không đúng offset.
- Memory retention gây tăng RAM không kiểm soát.

Cần kiểm tra thêm semantics của API:

- `offset` tính theo byte hay ký tự?
- UTF-8 tiếng Việt có thể bị cắt giữa một code point không?
- `limit` tính theo byte hay ký tự?
- Có đảm bảo kết quả trả về đúng thứ tự không?
- `resultId` có scope theo run/session không?
- TTL và cleanup hoạt động thế nào?
- Model có thể đọc nhầm `resultId` của lượt khác không?

Đây là các failure mode quan trọng hơn việc giảm vài tool call.

### 8.6. Bảng quyết định cuối cùng

| Thay đổi          | Điều kiện KEEP chính                             | Hard reject                              |
| ----------------- | ------------------------------------------------ | ---------------------------------------- |
| HarnessProfile    | Token -15%, success non-inferior trong biên 5 pp | Success -10 pp hoặc context regression   |
| Compact           | Token -15%, invalid param không tăng đáng kể     | Invalid param +5 pp                      |
| Prompt separation | Billable -15%, cache tăng                        | Correctness regression                   |
| Checklist         | Multi-step success +10 pp                        | Loop hoặc overall success giảm           |
| read_result       | Recovery ≥90%, duplicate reads -50%              | Data corruption, offset lỗi, memory leak |

Các ngưỡng trên là ngưỡng kỹ thuật đề xuất, không phải kết luận thống kê tự động.

Để tránh trường hợp may mắn với n nhỏ, tôi đề xuất thêm điều kiện: với các metric có thể bootstrap, phải xem cả khoảng tin cậy. Nếu CI quá rộng, kết quả là HOLD chứ không phải KEEP.

# 9. Lịch chạy benchmark theo pha

Tôi đề xuất kế hoạch 5 pha, có thể hoàn thành screening trong một ngày và có kết quả đủ tốt để quyết định mở rộng trong 3–5 ngày tiếp theo.

## 9.1. Kế hoạch tổng thể

| Pha               | Câu hỏi                                      | Bộ đề                         | Số lượt/nhánh | Thời gian |
| ----------------- | -------------------------------------------- | ----------------------------- | ------------- | --------- |
| P0 – Chuẩn bị     | Harness và ground truth đã đáng tin chưa?    | E2E 180 + smoke test          | 1 lần         | 2–3 giờ   |
| P1 – Screening    | Thay đổi nào có regression lớn?              | Calc 4 + Office 8 + Slides 4  | 3 cặp/task    | 1 ngày    |
| P2 – Targeted     | Giả thuyết của từng thay đổi có đúng không?  | 10 targeted cases             | 3–5 cặp/task  | 1 ngày    |
| P3 – Confirmation | Những thay đổi tốt có thực sự ổn định không? | Core 16 + targeted quan trọng | 8–12 cặp/task | 2–3 ngày  |
| P4 – Integration  | Kết hợp 5 thay đổi có gây regression không?  | 16 core + Slides 4 + Calc 4   | 3–5 cặp/task  | 1–2 ngày  |

Lưu ý: các số lượt ở P1 là số cặp, mỗi cặp có hai nhánh control và treatment.

### P0 – Chuẩn bị và kiểm tra harness

Không được bỏ qua

Mục tiêu:

- Xác nhận reset môi trường hoạt động.
- Xác nhận log token đầy đủ.
- Xác nhận ground truth chấm đúng.
- Xác nhận baseline có thể chạy lặp lại.

Việc cần làm:

1. Chạy 180 E2E tests.
2. Chạy một task Word, một task Excel, một task PowerPoint.
3. Chạy task có tool result bị truncate.
4. Kiểm tra `read_result` bằng unit/integration test.
5. Chạy một task trên từng máy hai lần để kiểm tra tính tái lập của harness.
6. Xác nhận artifact đầu ra được lưu riêng cho mỗi run.

Không cần đợi hết 180 live tasks. E2E 180 ca là kiểm tra tính đúng đắn của core, không phải bằng chứng chất lượng model.

### P1 – Screening trong một ngày

Tôi chọn 16 task:

- Calc: 4.
- Word/Excel: 8.
- Slides: 4.

Trong đó Slides phải có ít nhất một bài layout khó và một bài có lỗi cài sẵn.

Với mỗi thay đổi:

\\[ 16 \text{ tasks} \times 3 \text{ pairs} \times 2 = 96 \text{ runs} \\]

Nếu chạy riêng cả 5 thay đổi:

\\[ 96 \times 5 = 480 \text{ runs} \\]

Đây là quá nhiều cho một ngày, vì vậy không chạy đủ 16 task cho cả năm thay đổi.

Tôi đề xuất phân bổ:

| Thay đổi             | Task | Cặp/task | Tổng lượt |
| -------------------- | ---- | -------- | --------- |
| HarnessProfile       | 4    | 3        | 24        |
| Compact descriptions | 4    | 3        | 24        |
| Prompt separation    | 3    | 3        | 18        |
| Checklist            | 4    | 3        | 24        |
| read_result          | 4    | 3        | 24        |
| Tổng                 |      |          | 114       |

Các task có thể trùng nhau giữa những thay đổi, nhưng mỗi phép so sánh phải có baseline B0 tương ứng.

Với thời gian trung bình 2–5 phút/lượt, 114 lượt tương đương 4–9,5 giờ chạy tuần tự, chưa tính reset và lỗi hạ tầng.

Do đó, nếu chỉ có một máy chạy live:

- Ưu tiên 3 thay đổi có thể đo nhanh: compact, HarnessProfile, prompt separation.
- Chạy checklist và read_result trên targeted suite riêng.
- Chuyển phần còn lại sang ngày thứ hai.

Đừng cố ép 114 lượt vào một ngày bằng cách chạy nhiều session Office đồng thời trên cùng một máy. Điều đó làm hỏng tính độc lập của benchmark.

### P2 – Targeted stress test

Đây là pha quan trọng để phân biệt tối ưu thật với tối ưu giả.

| Thay đổi          | Task                                                       | Số cặp |
| ----------------- | ---------------------------------------------------------- | ------ |
| HarnessProfile    | 8k context, 32k context, long-history                      | 5/task |
| Compact           | Optional params, complex action schema                     | 5/task |
| Prompt separation | Cold cache, warm cache                                     | 5/task |
| Checklist         | 10+ steps, short task                                      | 5/task |
| read_result       | Large result, UTF-8 continuation, unnecessary continuation | 5/task |

Tổng cộng khoảng 50 cặp, tương đương 100 lượt.

P2 cần kiểm tra những tình huống mà P1 không có đủ sức phân biệt.

Ví dụ:

- Nếu compact giảm 20% token trên task đơn giản, chưa đủ chứng minh nó không làm mất optional parameter.
- Nếu checklist giảm vòng lặp trên task 10 bước nhưng làm tăng 30% token ở task 2 bước, cần cân nhắc chỉ bật có điều kiện.
- Nếu `read_result` giảm duplicate read nhưng model không dùng nó khi thực sự cần, thay đổi vẫn chưa đạt mục tiêu.

### P3 – Confirmation

Chỉ đưa thay đổi qua P1 và P2 vào pha này.

Tôi khuyến nghị:

| Nhóm              | Số task | Cặp/task |
| ----------------- | ------- | -------- |
| Word              | 4       | 8        |
| Excel             | 6       | 8        |
| PowerPoint        | 4       | 8        |
| Targeted critical | 4       | 12       |

Tương đương:

\\[ (14 \times 8 + 4 \times 12) \times 2 = 320 \text{ runs} \\]

Đây là mức xác nhận khá tốn kém. Không cần thực hiện toàn bộ 320 lượt cho từng thay đổi.

Phương án tiết kiệm:

- Mỗi thay đổi chỉ chạy trên nhóm task có liên quan.
- Với thay đổi có phạm vi rộng như compact và prompt separation, chạy 12 task đại diện.
- Với read_result, chỉ chạy những task large-result và một số core regression.
- Với checklist, ưu tiên multi-step và một số task ngắn.
- Với HarnessProfile, phân tầng theo context window.

### P4 – Integration

Đây là bước bắt buộc trước khi đưa cấu hình vào production.

Chạy:

- B0: toàn bộ cấu hình baseline.
- B6: toàn bộ thay đổi được chọn.

Không cần chạy tất cả 5 thay đổi nếu một số đã bị loại.

Tôi khuyến nghị ít nhất:

| Nhóm            | Task | Cặp/task |
| --------------- | ---- | -------- |
| Word            | 4    | 3        |
| Excel           | 6    | 3        |
| PowerPoint      | 4    | 3        |
| Calc regression | 4    | 3        |

Tổng:

\\[ 18 \times 3 \times 2 = 108 \text{ runs} \\]

Sau đó chạy một lượt smoke test trên Windows và Ubuntu cho mỗi ứng dụng được hỗ trợ.

Đây là bước kiểm tra tương tác, không phải phép đo hiệu quả độc lập của từng thay đổi.

## 9.2. Ước lượng chi phí thời gian

Với 114 lượt ở P1, giả định:

- 70% task mất 2–5 phút.
- 20% task mất 5–10 phút.
- 10% task mất 10–15 phút.

Thời gian trung bình giả định khoảng 5 phút/lượt:

\\[ 114 \times 5 = 570 \text{ phút} \\]

Ước lượng thời gian P1

# 9,5 giờ

Chưa tính thời gian điều tra lỗi và reset môi trường

| Công việc     | Thời gian |
| ------------- | --------- |
| Reset + setup | 45 phút   |
| 114 lượt chạy | 9,5 giờ   |
| Chấm kết quả  | 1–2 giờ   |
| Phân tích     | 1 giờ     |
| Tổng          | 12–13 giờ |

Nếu có thể giảm xuống 60 lượt bằng cách chọn 2–3 task đại diện cho mỗi thay đổi, screening có thể hoàn thành trong khoảng 1 ngày làm việc.

Tôi sẽ không đánh đổi độ phủ task để lấy số lượt lặp quá lớn. Với agent tài liệu, độ đa dạng của task quan trọng hơn việc chạy 30 lần cùng một bài replace text.

# 10. Những thay đổi nên đo trên task nào?

Đây là bảng thực thi tôi khuyến nghị dùng trực tiếp.

| Thay đổi          | Bộ đề ưu tiên                                     | Control | Treatment | Metric quan trọng nhất                |
| ----------------- | ------------------------------------------------- | ------- | --------- | ------------------------------------- |
| HarnessProfile    | W05, E05, E06, P03 + 8k/32k stress                | B0      | B1        | Success, input token, overflow        |
| Compact           | W06, E06, các bài Calc có tool phức tạp           | B0      | B2        | Input token, invalid params, OPOR     |
| Prompt separation | W03, E02, P03 + cache stress                      | B0      | B3        | Cached token, billable token, success |
| Checklist         | W04, E06, P03 + 10-step stress                    | B0      | B4        | Multi-step success, loop, rounds      |
| read_result       | E05, E06, bài đọc range lớn + Word large document | B0      | B5        | Recovery, duplicate reads, success    |

Một điểm cần lưu ý: task nào không thực sự kích hoạt một thay đổi thì không nên đưa vào phân tích hiệu quả chính của thay đổi đó.

Ví dụ:

- Task không có truncation không giúp chứng minh `read_result` hiệu quả.
- Task không có cache reuse không giúp chứng minh prompt caching hiệu quả.
- Task không có nhiều bước không giúp chứng minh checklist hiệu quả.

Những task này vẫn hữu ích để kiểm tra regression toàn cục, nhưng phải tách khỏi nhóm task mục tiêu.

# 11. Chuẩn bị hệ thống benchmark trước khi chạy

Tôi đề xuất bổ sung một lớp benchmark metadata riêng, không sửa logic chính của agent để phục vụ benchmark.

## 11.1. Run manifest

Mỗi lần chạy phải có manifest:

```
{
  "run_id": "run-20261111-001",
  "pair_id": "pair-001",
  "task_id": "E05",
  "variant": "treatment",
  "changes": {
    "harness_profile": false,
    "compact_descriptions": false,
    "prompt_separation": false,
    "checklist": false,
    "read_result": true
  },
  "model": "provider:model",
  "provider": "gateway",
  "app": "libreoffice",
  "os": "ubuntu-24.04",
  "commit": "git-sha",
  "fixture_hash": "sha256",
  "started_at": "2026-11-11T08:00:00Z",
  "timeout_seconds": 120
}
```

Không cần dùng đúng định dạng JSON này nếu đã có hệ thống config, nhưng các trường cần được lưu.

Đặc biệt, `changes` phải ghi chính xác trạng thái của từng flag, không chỉ ghi tên nhánh.

## 11.2. Schema benchmark database

Tôi đề xuất có bảng riêng:

| Table                | Nội dung                       |
| -------------------- | ------------------------------ |
| `bench_runs`         | Thông tin mỗi lần chạy         |
| `bench_pairs`        | Ghép cặp control/treatment     |
| `bench_task_results` | Success, correctness, score    |
| `bench_metrics`      | Token, time, tool calls        |
| `bench_tool_events`  | Action, params, result, errors |
| `bench_artifacts`    | File đầu vào/đầu ra và hash    |
| `bench_infra_events` | Timeout, crash, gateway outage |

Không nên chỉ dựa vào `run_rounds` để xây dựng báo cáo. Nó là dữ liệu vận hành của agent, còn benchmark cần thêm lớp ground truth và paired metadata.

## 11.3. Những trường cần log thêm

Từ `DebugTrace JSONL`, `METRIC` và `core.db`, tôi muốn lấy được:

Model/API

- Provider.
- Exact model ID.
- Request ID.
- HTTP status.
- Timeout.
- Empty response.
- Usage metadata.
- Cached tokens.
- Retry count.

Agent

- Prompt hash.
- Tool schema hash.
- Harness profile ID.
- System prompt token estimate.
- Dynamic prompt token estimate.
- Checklist version/hash.
- Number of agent rounds.
- Stop reason.

Tools

- Action name.
- Params.
- Validation result.
- Execution result.
- Truncation metadata.
- `resultId`.
- `read_result` offset/limit.
- Duplicate read classification.

Office

- Application/version.
- Document fixture hash.
- Output document hash.
- Document open/save status.
- Ground truth score.
- Verification status.

## 11.4. Các script nên có

| Script                | Trách nhiệm                  |
| --------------------- | ---------------------------- |
| `bench/run.py`        | Orchestrate task và variant  |
| `bench/reset.py`      | Reset app và fixture         |
| `bench/manifest.py`   | Ghi cấu hình run             |
| `bench/validate.py`   | Chấm kết quả tài liệu        |
| `bench/aggregate.py`  | Tổng hợp metric              |
| `bench/paired.py`     | Ghép cặp control/treatment   |
| `bench/report.py`     | Xuất Markdown/HTML           |
| `bench/statistics.py` | Bootstrap, McNemar, Wilcoxon |
| `bench/sentinel.py`   | Kiểm tra model drift         |

Nếu phần lớn hệ thống benchmark đang viết bằng Go, không nhất thiết phải chuyển sang Python. Python chỉ nên được sử dụng nếu nó giúp tận dụng các thư viện thống kê và đọc tài liệu hiện có.

# 12. Những cạm bẫy cần tránh

Đây là những vấn đề tôi cho rằng dễ làm benchmark Axiom Office đưa ra kết luận sai nhất.

### 12.1. Chạy cùng task nhưng không reset tài liệu

Ví dụ:

- Control sửa tài liệu thành công.
- Treatment chạy trên chính tài liệu đã sửa.
- Treatment không còn phải thực hiện đầy đủ công việc.

Kết quả treatment sẽ tốt giả tạo.

Cách tránh: mỗi lượt phải khôi phục golden fixture, xác minh hash trước khi mở app.

### 12.2. So sánh token giữa hai lần chạy không cùng điều kiện cache

Đây là lỗi đặc biệt nguy hiểm với prompt separation.

Một lần chạy có cache nóng, lần kia cache lạnh, chênh lệch billable token không còn phản ánh riêng thay đổi.

Cách tránh: cold/warm cache phải là hai thí nghiệm riêng.

### 12.3. Chỉ chấm file có mở được hay không

Một file Word mở được không có nghĩa là task thành công.

Một file PowerPoint có đủ 10 slide không có nghĩa là slide đẹp.

Một file Excel có đủ dữ liệu không có nghĩa là công thức đúng.

Cách tránh: golden fixture + structural validator + visual validator.

### 12.4. Gộp mọi loại failure thành một nhóm

Nếu model timeout, agent gọi sai tool và Office crash đều được ghi là `failed`, bạn sẽ không biết cần sửa phần nào.

Cách tránh: phân loại failure theo layer:

```
Infrastructure
  ├── Gateway outage
  ├── Network failure
  └── OS / App crash

Model
  ├── Empty response
  ├── Timeout
  ├── Invalid tool selection
  └── Hallucinated parameters

Agent Core
  ├── Guard regression
  ├── State management
  ├── Infinite loop
  └── Incorrect truncation handling

Document
  ├── Content incorrect
  ├── Structure incorrect
  ├── Formatting incorrect
  └── Visual defects
```

### 12.5. Tối ưu trên một model rồi áp dụng cho mọi model

HarnessProfile được thiết kế theo model là hợp lý. Nhưng không nên suy ra profile tốt cho `mimo-v2.6-flash-free` cũng tốt cho `deepseek-v4.1-flash`.

Cần lưu kết quả theo:

```
provider:model:profile_version
```

Không chỉ theo tên model.

### 12.6. Chỉ báo cáo average

Ví dụ:

- 9 task giảm 20% token.
- 1 task tăng 150% token.

Average có thể che giấu task bị regression nghiêm trọng.

Bắt buộc báo cáo median, P90, min/max và task-level results.

### 12.7. Chạy lại đến khi thành công

Nếu một task thất bại ba lần rồi lần thứ tư thành công, không được chỉ ghi lần thứ tư.

Đó là cherry-picking.

Phải ghi toàn bộ lượt và áp dụng retry policy thống nhất.

### 12.8. Đánh giá thay đổi bằng tổng token mà bỏ qua chất lượng

Ví dụ trước đây bạn đã có một kết quả đáng chú ý:

- Variant giảm median billable token 11,4%.
- Tool call giảm 24,2%.
- Wall time giảm 9,9%.
- Nhưng chart giảm từ 3/3 xuống 1/3.

Đây chính là ví dụ điển hình cho thấy một optimization có thể tốt về efficiency nhưng không đạt yêu cầu production.

Tôi sẽ coi chart regression là một blocker, không phải một trade-off chấp nhận được chỉ vì tiết kiệm token.

### 12.9. Coi các lần chạy lặp cùng task là nhiều task độc lập

Đây là lỗi thống kê thường gặp.

Chạy task `E01` 30 lần không tương đương chạy 30 task Excel khác nhau.

Cần có đủ task diversity và repeated runs.

### 12.10. Không kiểm tra interaction giữa các thay đổi

Đặc biệt:

- Compact + HarnessProfile.
- read_result + Compact.
- Checklist + HarnessProfile.

Một thay đổi có thể tốt độc lập nhưng xấu khi kết hợp.

Do đó P4 không phải phần tùy chọn.

# 13. Dashboard báo cáo cuối cùng

Tôi đề xuất một báo cáo duy nhất cho mỗi benchmark campaign.

## 13.1. Executive summary

AXIOM OFFICE — BENCHMARK CAMPAIGN

Ví dụ cấu trúc báo cáo, chưa phải kết quả thực nghiệm

Task success

# --%

Token reduction

# --%

Cost per success

# --

P90 wall time

# --s

Decision summary

| Change               | Decision |
| -------------------- | -------- |
| HarnessProfile       | Pending  |
| Compact descriptions | Pending  |
| Prompt separation    | Pending  |
| Checklist            | Pending  |
| read_result          | Pending  |

## 13.2. Bảng kết quả theo task

| Task | App   | Control | Treatment | Token Δ | Success Δ |
| ---- | ----- | ------- | --------- | ------- | --------- |
| W01  | Word  | S3      | S3        | -18%    | 0         |
| W02  | Word  | S3      | S3        | -12%    | 0         |
| E04  | Excel | S3      | S1        | -25%    | -1        |
| E05  | Excel | S2      | S3        | -10%    | +1        |
| P03  | PPT   | S3      | S3        | -16%    | 0         |

Đây là dữ liệu minh họa cách trình bày, không phải kết quả Axiom thực tế.

## 13.3. Bảng quyết định cuối cùng

Mỗi thay đổi cần có:

- Hypothesis.
- Sample size.
- Effect size.
- 95% CI.
- Success difference.
- Failure breakdown.
- Regression task.
- Decision.
- Lý do giữ/bỏ.

Tôi khuyến nghị thêm `confidence_level`:

- High: nhiều task, nhiều paired runs, CI đủ hẹp, không có regression đáng kể.
- Medium: xu hướng tốt nhưng mẫu còn hạn chế.
- Low: n nhỏ, model drift hoặc dữ liệu thiếu.

Đừng dùng confidence level để thay thế thống kê; nó là nhãn giúp người đọc hiểu mức độ chắc chắn của kết luận.

# 14. Kế hoạch thực hiện tôi khuyến nghị

Nếu trực tiếp phụ trách benchmark Axiom Office, tôi sẽ triển khai theo thứ tự sau:

1. Ngày 1 – Chuẩn hóa benchmark
   - Đóng băng baseline commit.
   - Chuẩn hóa 16 core tasks.
   - Chuẩn hóa acceptance criteria.
   - Bổ sung run manifest và paired ID.
   - Kiểm tra reset fixture và log token.
   - Chạy E2E 180 tests.
2. Ngày 2 – Screening
   - Chạy compact descriptions.
   - Chạy HarnessProfile.
   - Chạy prompt separation.
   - Chạy checklist.
   - Chạy read_result.
   - Phân loại regression.
3. Ngày 3 – Targeted tests
   - Context 8k/32k.
   - Optional parameters.
   - Cold/warm cache.
   - Task 10+ bước.
   - Large truncated results.
4. Ngày 4–5 – Confirmation
   - Mở rộng mẫu những thay đổi có triển vọng.
   - Phân tích paired differences.
   - Bootstrap confidence intervals.
   - Kiểm tra từng ứng dụng.
5. Ngày 6 – Integration
   - Chạy B0 so với cấu hình cuối.
   - Kiểm tra tương tác.
   - Chạy Windows Office 2019.
   - Chạy Ubuntu WPS/LibreOffice.
   - Ra quyết định production.

## 15. Kết luận: tôi sẽ ưu tiên điều gì?

Nếu phải chọn thứ tự đầu tư benchmark và triển khai, tôi chọn:

| Ưu tiên | Thay đổi              | Lý do                                                |
| ------- | --------------------- | ---------------------------------------------------- |
| 1       | Compact descriptions  | Tiềm năng giảm đáng kể prompt overhead, dễ đo        |
| 2       | HarnessProfile        | Đúng vấn đề model yếu và context nhỏ                 |
| 3       | read_result           | Giải quyết giới hạn dữ liệu lớn và duplicate reads   |
| 4       | Checklist             | Có thể cải thiện khả năng hoàn thành multi-step task |
| 5       | Stable/dynamic prompt | Tiềm năng tiết kiệm chi phí nhưng phụ thuộc provider |

Thứ tự này là thứ tự ưu tiên benchmark, không phải khẳng định mức độ hiệu quả đã được chứng minh.

Ba quyết định quan trọng nhất:

1. Không dùng n=3 để tuyên bố một thay đổi đã tốt hơn. n=3 chỉ để phát hiện lỗi rõ rệt.
2. Không dùng token reduction làm tiêu chí độc lập. Phải đánh giá cost per successful task và correctness.
3. Không đưa cấu hình kết hợp vào production chỉ dựa trên kết quả ablation riêng lẻ. Phải có integration benchmark.

### Một điều tôi muốn bổ sung riêng cho Axiom Office

Tôi nghĩ nên xây dựng một Axiom Office Benchmark Corpus độc lập với codebase:

```
tests/benchmark-corpus/
├── manifest/
│   ├── tasks.yaml
│   ├── models.yaml
│   └── profiles.yaml
├── fixtures/
│   ├── word/
│   ├── excel/
│   └── powerpoint/
├── expected/
│   ├── word/
│   ├── excel/
│   └── powerpoint/
├── validators/
│   ├── word/
│   ├── excel/
│   └── powerpoint/
├── stress/
│   ├── context-budget/
│   ├── optional-parameters/
│   ├── checklist/
│   ├── truncation/
│   └── prompt-cache/
└── reports/
```

Corpus này sẽ trở thành tài sản kiểm thử lâu dài của Axiom, không phụ thuộc vào việc bạn đang dùng model nào, Go Core phiên bản nào hay bridge nào.

Nó cũng giúp sau này benchmark được các thay đổi lớn hơn như:

- Một tool `office_action` so với nhiều tool riêng biệt.
- Một agent so với multi-agent.
- Thay đổi orchestration strategy.
- Thay đổi memory retrieval.
- Thay đổi model routing.
- Thay đổi StopRule và verifyGuard.

Quan điểm cuối cùng: Với Axiom Office, benchmark phải chứng minh được một điều: Agent tạo ra tài liệu đúng hơn hoặc ít nhất không kém, với chi phí và thời gian thấp hơn, trên những model yếu mà người dùng thực sự sử dụng. Đây mới là tiêu chuẩn production có ý nghĩa, không phải số token tiết kiệm được trong một bài demo.