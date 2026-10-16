# 08 / P7 - UX trong app

> Người không rành kỹ thuật cài và dùng được. Lỗi hiện tiếng Việt (hoặc ngôn ngữ đã chọn) dễ hiểu.

## 1. Đã có (đối chiếu code 16/10/2026)

- Khung chat (`src/AxiomOffice/Ai/AskAiPane.cs`, `PaneControls.cs`, `MarkdownView.cs`): stream qua SSE, suy luận
  hiện trực tiếp, dòng trạng thái theo bước, nút Dừng hủy thật, "Trò chuyện mới" xóa sạch, đính kèm tệp, lập kế
  hoạch trước, danh sách việc (`write_todos`).
- Thẻ xác nhận và thẻ nhiều lựa chọn có ô tự gõ (`confirm.go`); hết giờ/hủy báo rõ.
- Chế độ cơ bản khi Core không chạy (agent trong add-in), dòng trạng thái ghi "chế độ cơ bản".
- Dừng lượt khi Word/Office đã đóng hoặc sập (`AppGone`).
- Wizard thiết lập 5 bước (`SetupWizardForm.cs`), nút Trạng thái, Mở nhật ký, Kiểm tra mô hình, Tắt Agent Core;
  hộp thoại Công cụ ngoài (MCP), Bộ nhớ AI, Kỹ năng.
- Ngôn ngữ giao diện (`Localization/Ui.cs`, `lang/en.json`): ribbon + khung chat Windows; menu "Ngôn ngữ · Language"
  và ô chọn ở bước 1 của wizard.
- Doctor: `core-go/internal/doctor/doctor.go`.

## 2. Còn thiếu - việc cần làm

- **Dịch nốt giao diện**: wizard (gồm chữ từ `catalog/setup.json`, dịch lúc hiển thị), Cài đặt nâng cao, Bộ nhớ AI,
  Công cụ ngoài, khung chat WPS (`pane.html`) và LibreOffice; câu Core gửi lên khung chat ("Bước 1/3…",
  "Dùng kỹ năng", thẻ hỏi kiểu trình bày) theo ngôn ngữ giao diện gửi kèm request.
- **Ba lỗi hay gặp có hướng dẫn sửa ngay trong khung chat**: sai khóa, sai endpoint, Core không chạy (đã có một
  phần trong thẻ lỗi; kiểm lại câu chữ với người dùng thật).
- **Trạng thái Core trên ribbon** (xanh/vàng/đỏ) thay cho phải bấm "Trạng thái".
- **WPS Windows**: kiểm ribbon có gọi `getLabel` không (hiện dùng nhãn tĩnh, đổi ngôn ngữ phải mở lại WPS).
- Bỏ lỗi mã hóa trong bản kế hoạch cũ ("Desdang dung che do co banDs").

## 3. Tệp sẽ sửa

- `src/AxiomOffice/Ai/*.cs`, `src/AxiomOffice/Ribbon/RibbonUi.cs`, `src/AxiomOffice/lang/*.json`
- `src/AxiomOffice.WPS/pane.html`, `pane-core.js`
- `core-go/internal/agent/orchestrator.go` (ngôn ngữ giao diện cho câu trạng thái)

## 4. Tiêu chí xong

- `python scripts/check_i18n.py` báo thiếu 0 sau khi thêm các form.
- Người mới (chưa từng dùng) cài xong và hỏi được AI lần đầu trong 5 phút - thử với 3 người thật, ghi lại chỗ vấp.
- Hủy lượt giữa chừng không treo khung chat, không rác session (test tay + `tests/core/test_core_e2e.py`).

## 5. Không làm

- Không giao diện dòng lệnh đầy đủ, không giọng nói, không web dashboard. Khung chat trong app là chính.
