# 18 / R0 - Task chi tiết (Model yếu và bộ đo)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `17-r0` trước.
> R0-00 (chặn loop + hỏi lượt nặng) đã có 16/10/2026. R0-01…R0-02 là hạ tầng để chấm mọi phase sau.

## R0-00. Chặn đốt token - ĐÃ CÓ

- [x] `sameTaskGuard` (`core-go/internal/model/guards.go`, `agent.go`): `MaxSameTaskCalls` mặc định 25, đếm theo
      nhóm việc, miễn lệnh ghi dữ liệu theo khối.
- [x] `heavyRunGuard` + `AgentCallbacks.AskContinue` (`agent.go`, `guards.go`): vượt 30 vòng hoặc 500k token
      billable thì hỏi qua thẻ `Choose`; `Interactive=false`/hết giờ thì chạy tiếp.
- [x] Test `internal/model/sametask_test.go`; đã tắt guard trong `agent_deadline_test.go` khi thử đồng hồ.

## R0-01. Định dạng kịch bản + 12 kịch bản phủ

- [ ] Thư mục mới `tests/regress/` (tách khỏi `tests/bench/` - bench là so với Claude Code, regress là hồi quy).
- [ ] `tests/regress/scenarios/<mã>.json`: `id`, `app` (`wps`/`et`/`wpp`), `source` (đường dẫn mẫu trong repo),
      `prompt`, `interactiveAnswers` (trả lời sẵn cho thẻ `Request`/`Choose`), `checks` (xem R0-02), `runs` (mặc
      định 3).
- [ ] Mẫu nguồn: đặt bản sao tĩnh dưới `tests/regress/fixtures/`; **không** dùng tệp thật của người dùng.
- [ ] Phủ tối thiểu 12 kịch bản: giáo án Word (trình bày lại, sửa nội dung, tóm tắt), bảng điểm Excel (thêm cột
      công thức, định dạng số), slide (dựng theo mẫu, sửa bố cục), memory (hai tệp không lẫn), 3 sự cố mục 1 của
      `17-r0` (137 lần `replaceAll`, "làm toàn bộ" xóa dấu ngày tháng, `replace` bị từ chối rồi chèn `before`).
- [ ] Test: `python tests/regress/test_scenarios.py` (kiểm mọi tệp kịch bản đọc được, `checks` có ít nhất một mục).

## R0-02. Chấm bằng lệnh đọc (không dùng LLM khác)

- [ ] Mỗi `check` là một lệnh bridge/đọc tài liệu, ví dụ:
      `et.readRange formulas=true` so mốc, `writer.getText` không mất số liệu, `writer.outline` đủ mục lớn,
      `wpp.checkLayout` không lỗi, `wpp.listSlides` giữ layout, không chữ lặp hai lần.
- [ ] Viết `checks.py`: hàm nhận kết quả lệnh + mốc, trả `passed`/`failed` + lý do; mốc lưu cạnh kịch bản
      (`<mã>.expected.json`), so với lần chạy trước khi chưa có mốc cứng.
- [ ] Không chấm bằng "model khác thấy ổn": mọi tiêu chí phải đọc được bằng lệnh (đúng `17-r0` mục 5).

## R0-03. Bộ chạy một kịch bản

- [ ] Script mở **bản sao** trong app (theo mẫu các smoke script sẵn có), chờ bridge lên, rồi
      `POST /v1/runs` (`interactive=false`, thẻ trả lời sẵn), đọc SSE tới `run.completed`/`run.failed`, đóng tài
      liệu **không lưu**.
- [ ] Tham chiếu cách chạy Core + bridge: `tests/core/test_core_e2e.py`, `tests/live/test_live_commands.py`,
      `tests/lo/`, `tests/wps/`. Dùng biến `AXIOM_*` như các bộ này.
- [ ] Timeout cứng mỗi lượt (ví dụ 10 phút) để bộ không treo khi model free lỗi mạng.

## R0-04. Đọc số đo từ `core.db`

- [ ] Đọc `runs` (model, số vòng, token vào/ra, cache, lỗi) và `tool_calls` (từng lệnh, params, `ok`, `ms`) theo
      `run_id` vừa chạy; đường dẫn `%LOCALAPPDATA%\AxiomOffice\core\core.db`.
- [ ] Tính thêm: số lệnh, lệnh lỗi, lệnh bị chặn (guard), `wall` time, số vòng.
- [ ] Ghi vào `<mã>.<model>.<lần>.json` cạnh kết quả.

## R0-05. Chạy N lần × danh sách model + xuất bảng

- [ ] Một lệnh chạy cả bộ: `mimo`, `deepseek flash`, một model mạnh làm mốc; mặc định 3 lần/kịch bản.
- [ ] Xuất `results/regress-<ngày>.md` (bảng: kịch bản × model, đạt/tổng, vòng, token, lệnh, thời gian) và
      `.csv` để so lần trước; không cần dashboard.
- [ ] Loại khỏi số liệu lượt `provider returned an empty reply` (lỗi hạ tầng) và **ghi rõ** số lượt bị loại.

## R0-06. Tiêu chí xong của R0 (chốt khi làm)

- [ ] Ba sự cố ở `17-r0` mục 1 có kịch bản tái hiện; trên code hiện tại cả ba đạt.
- [ ] Không kịch bản nào với mimo/deepseek vượt 30 vòng hoặc 500k token mà không hỏi người dùng.
- [ ] Chạy lại một lệnh ra bảng số đo cho mọi kịch bản × model.

## R0-07. Hồi quy + changelog

- [ ] `cd core-go && go test ./...` (guard đổi thì cập nhật `sametask_test.go`).
- [ ] Ghi `CHANGELOG.md [Unreleased]` (mục Added - Bộ đo R0) và cập nhật `17-r0` khi có số đo đầu tiên.

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/model/... ./internal/agent/...
cd ..; python tests/regress/test_scenarios.py
python tests/regress/run_all.py --models mimo,deepseek --runs 3   # cần Word/Excel/PowerPoint thật, đóng trước
```
