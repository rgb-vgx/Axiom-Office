# 03 / P2 - Skill chuẩn văn phòng

> Skill tĩnh do người kiểm duyệt. Không tự sinh skill.

## 1. Mục tiêu

- Nội dung 8 skill đúng chuẩn văn phòng Việt Nam, ngắn đủ để model yếu đọc hết.
- Kiểm được skill có tác dụng hay không, bằng tiêu chí đo được thay vì so tệp golden.

## 2. Đã có (đối chiếu code 16/10/2026)

- `skills/`: `bang-diem`, `bao-cao-du-lieu`, `bao-cao-thang`, `mo-hinh-nhieu-sheet`, `the-thuc-van-ban`,
  `thiet-ke-van-phong`, `trinh-bay-chuyen-nghiep`, `van-ban-hanh-chinh` + `_design` (token màu/cỡ chữ).
- Nạp skill 3 tầng: `core-go/internal/skills/` (`loader.go`, `index.go`, `files.go`); tool `load_skill`,
  `read_skill_file` (`internal/tools/skill_tools.go`). **Model tự chọn skill** từ danh sách trong system prompt.
- Test: `repo_skills_test.go`, `repo_skills_tokens_test.go` (giới hạn token của skill).
- Lệnh kiểm cấu trúc có sẵn để chấm: `writer.checkTables`, `et.checkRange`, `wpp.checkLayout`, nút Soát trên ribbon.
- Một số tool "làm hộ" thay cho skill dài: `doc_design` (Word), `slides_from_template` (PowerPoint).

## 3. Thay đổi so với bản cũ

- **Bỏ golden chéo C#/Go**: MCP bản C# đã xóa (STATUS.md), `oracle_diff.py` không còn đối tượng so cho skill.
- **Bỏ so tệp golden từng byte**: skill chạy bằng LLM nên đầu ra không cố định, so byte luôn trượt.
- **Không làm bộ chọn skill bằng từ khóa**: đi ngược thiết kế hiện tại (model chọn qua `load_skill`). Nếu model
  chọn sai thì sửa mô tả skill.
- Câu "không skill nào ghi file trực tiếp" sửa thành: skill không chạy macro/script; tool như
  `slides_from_template` được lưu tệp mới cạnh tệp nguồn, không ghi đè tệp người dùng.

## 4. Còn thiếu - việc cần làm

### 4.1 Nội dung

- Front-matter thống nhất: tên, mô tả (khi nào dùng), không dùng cho gì.
- Rà từng skill với chuẩn thật: Nghị định 30/2020 (văn bản hành chính), mẫu báo cáo, định dạng số tiền Việt Nam.
- Cắt skill dài: mỗi skill dưới ngưỡng `repo_skills_tokens_test.go`; chi tiết đưa vào `references/` đọc khi cần.

### 4.2 Chấm bằng tiêu chí cấu trúc

- Mỗi skill có 2–3 kịch bản trong bộ `17`, kèm **tiêu chí đọc được bằng lệnh**: font/cỡ chữ heading, lề trang,
  số dòng tiêu đề bảng lặp, định dạng số, không ô lỗi (`et.checkRange`), không chữ tràn (`wpp.checkLayout`).
- Đo tỉ lệ model **gọi đúng skill** theo kịch bản (đọc từ `tool_calls`).

## 5. Tệp sẽ sửa

- `skills/*/SKILL.md`, `skills/*/references/*`
- `core-go/internal/tools/repo_skills_test.go` (nếu đổi front-matter)
- Kịch bản và tiêu chí trong bộ đo `17`

## 6. Tiêu chí xong

- Mỗi skill: kịch bản của nó đạt tiêu chí cấu trúc ở ≥ 4/5 lần với deepseek, ≥ 3/5 với mimo.
- Model gọi đúng skill ở ≥ 90% lần chạy của bộ kịch bản (đếm từ `tool_calls`).
- `go test ./internal/skills/... ./internal/tools/...` xanh.

## 7. Không làm

- Không tự sinh skill từ memory, không skill chạy macro/script, không gọi MCP ngoài trong skill mặc định.
