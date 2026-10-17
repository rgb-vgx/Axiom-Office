# 19 / P2 - Task chi tiết (Skill chuẩn văn phòng)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `03-p2` trước.
> Skill là tệp tĩnh do người kiểm duyệt; **không** tự sinh, không chạy macro/script.

## P2-01. Front-matter thống nhất - XONG

- [x] Mẫu chung: `name`, `description` (khi nào dùng), thêm khóa `not_for` (không dùng cho gì).
- [x] `core-go/internal/skills/loader.go` (`Parse`, `ValidateNotFor`), `index.go` (`Definition.NotFor`); kiểm độ dài +
      không XML như `description` (rỗng được phép ở tầng loader, skill trong repo thì bắt buộc).
- [x] `core-go/internal/skills/skills_test.go`: `TestParseNotFor` (chuỗi khối `>-`, rỗng, và XML bị từ chối).
- [x] Hiện `not_for` cho model ở danh sách skill (`agent/prompt.go` `SkillsSection`): dòng riêng
      `  (not for: …)` ngay dưới mỗi skill - **không** tính vào ngân sách 300 ký tự của `description`, giữ nguyên
      luật "model tự chọn skill qua `load_skill`".
- [x] `core-go/internal/tools/repo_skills_test.go`: kiểm mọi skill trong `skills/` có đủ 4 khóa (name, description,
      apps, not_for); đã thêm `not_for` cho cả 8 skill và `apps` cho `thiet-ke-van-phong`.

## P2-02. Rà nội dung 8 skill với chuẩn thật

- [ ] `skills/van-ban-hanh-chinh/`: Nghị định 30/2020/NĐ-CP (quốc hiệu, số/ký hiệu, trích yếu, nơi nhận, người ký,
      phông chữ, lề, số trang); đối chiếu từng mục, ghi nguồn trong `references/`.
- [ ] `skills/bang-diem/`, `bao-cao-du-lieu/`, `bao-cao-thang/`: mẫu báo cáo, định dạng số tiền Việt Nam
      (`1.234.567`, đơn vị tiền, phần trăm), công thức Excel.
- [ ] `skills/mo-hinh-nhieu-sheet/`, `the-thuc-van-ban/`, `thiet-ke-van-phong/`, `trinh-bay-chuyen-nghiep/`: rà lại
      mô tả "khi nào dùng", tránh hai skill chồng việc.
- [ ] Đổi mục nào thì ghi lý do vào `references/` (không để quy tắc mồ côi).

## P2-03. Cắt skill dài, chi tiết vào `references/`

- [ ] Ngưỡng token: `core-go/internal/tools/repo_skills_tokens_test.go` và `repo_skills_test.go`; skill dài quá
      ngưỡng thì chuyển bảng dài sang `skills/<name>/references/*.md` (đọc khi cần qua `read_skill_file`).
- [ ] Giữ `skills/_design/tokens.json` là **dữ liệu**, skill chỉ ghi tên token (đúng chú thích trong
      `repo_skills_tokens_test.go`).
- [ ] Test: `go test ./internal/skills/... ./internal/tools/...` xanh sau mỗi lần cắt.

## P2-04. Kịch bản + tiêu chí cấu trúc trong bộ `17`

- [ ] Mỗi skill 2–3 kịch bản trong `tests/regress/scenarios/` (xem `18-r0`), tiêu chí **đọc được bằng lệnh**:
      font/cỡ chữ heading, lề trang, số dòng tiêu đề bảng lặp, định dạng số, không ô lỗi (`et.checkRange`), không
      chữ tràn (`wpp.checkLayout`).
- [ ] Dùng lệnh kiểm có sẵn: `writer.checkTables`, `et.checkRange`, `wpp.checkLayout`, nút Soát trên ribbon.

## P2-05. Đo tỉ lệ gọi đúng skill

- [ ] Đọc `tool_calls` trong `core.db`: lọc `load_skill`, so tên skill mong đợi của kịch bản với thực tế.
- [ ] Ghi vào bảng R0 (`18-r0` R0-05): tỉ lệ gọi đúng theo model.
- [ ] Nếu model chọn sai nhiều: sửa `description`/`not_for`, **không** thêm bộ chọn bằng từ khóa.

## P2-06. Hồi quy + changelog

- [x] `cd core-go && go test ./internal/skills/... ./internal/tools/...`.
- [x] Ghi `CHANGELOG.md [Unreleased]` (mục Changed - skill) khi đổi front-matter hoặc nội dung.

## Tiêu chí xong (đo bằng bộ `17`)

- [ ] Mỗi skill: kịch bản của nó đạt tiêu chí cấu trúc ở ≥ 4/5 lần với deepseek, ≥ 3/5 với mimo.
- [ ] Model gọi đúng skill ở ≥ 90% lần chạy (đếm từ `tool_calls`).

## Thứ tự chạy lệnh

```powershell
cd core-go; go test ./internal/skills/... ./internal/tools/...
cd ..; python tests/regress/test_scenarios.py
```
