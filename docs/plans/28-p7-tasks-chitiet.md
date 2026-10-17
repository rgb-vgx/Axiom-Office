# 28 / P7 - Task chi tiết (UX trong app)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `08-p7` trước.
> Đích: người không rành kỹ thuật cài và dùng được; lỗi hiện tiếng Việt (hoặc ngôn ngữ đã chọn) dễ hiểu.

## P7-00. Đã có

- [x] Khung chat Windows: stream SSE, suy luận trực tiếp, nút Dừng hủy thật, đính kèm, lập kế hoạch, danh sách việc.
- [x] Thẻ xác nhận và thẻ nhiều lựa chọn (`core-go/internal/agent/confirm.go`), hết giờ/hủy báo rõ.
- [x] Chế độ cơ bản khi Core không chạy; dừng lượt khi Office đóng/sập (`AppGone`).
- [x] Wizard 5 bước, nút Trạng thái, Mở nhật ký, Kiểm tra mô hình, Tắt Agent Core; MCP, Bộ nhớ AI, Kỹ năng.
- [x] Dịch giao diện: `Localization/Ui.cs`, `lang/en.json` (ribbon + khung chat Windows); menu "Ngôn ngữ ·
      Language". `python scripts/check_i18n.py` = **0 thiếu** (17/10/2026: 418 khoá).

## P7-01. Dịch nốt giao diện Windows

- [ ] Quét lại các chuỗi **chưa** vào `Ui.T`: wizard (`SetupWizardForm.cs`, `SetupWizardControls.cs`), Cài đặt nâng
      cao (`SettingsForm.cs`), Bộ nhớ AI (`MemoryForm.cs`), Công cụ ngoài (`McpForm.cs`), quản lý (`AdminForms.cs`).
- [ ] Chữ từ `catalog/setup.json` đã được `check_i18n.py` gom sẵn (0 thiếu) - chỉ kiểm phần dịch **lúc hiển thị**
      trong wizard dùng đúng `Ui.T` chứ không đọc thẳng chuỗi tiếng Việt.
- [ ] `src/AxiomOffice/lang/en.json`: thêm khoá mới; `python scripts/check_i18n.py` vẫn 0 thiếu.

## P7-02. Dịch khung chat WPS và LibreOffice

- [ ] `src/AxiomOffice.WPS/pane.html`, `pane-core.js`: 17/10/2026 kiểm **không có** cơ chế i18n; thêm bảng chuỗi
      tiếng Anh tối thiểu (nút, trạng thái, thẻ hỏi) và đọc ngôn ngữ gửi kèm.
- [ ] LibreOffice (`axiom/chat.py`, `chatview.py`): chuỗi hiển thị theo ngôn ngữ đã chọn ở wizard.
- [ ] Ghi chú: `check_i18n.py` chỉ quét `src/AxiomOffice`, nên hai khung này cần bộ kiểm riêng (đơn giản: đếm khoá).

## P7-03. Câu Core gửi lên khung chat theo ngôn ngữ - XONG

- [x] `agent.Request.Language` (`ParseRequest` đọc `body.language`, `NormalizeLanguage`: `en*` → `en`, còn lại →
      `vi`); giữ `Protocol = 1`, thiếu trường thì như cũ.
- [x] Câu trạng thái (`agent/live.go` `liveProgress`) dịch theo ngôn ngữ: "Đang suy nghĩ…"/"Thinking…",
      "Đang viết câu trả lời…"/"Writing the answer…", tiến độ dựng slide, "Đang viết lệnh …".
- [x] Thẻ hỏi kiểu trình bày (`tools/doc_design.go` `docStyleQuestion`/`docStyleChoices(language)`) có bản tiếng Anh;
      ID lựa chọn (`full`/`basic`/`keep`/`custom`) không đổi nên lựa chọn cũ vẫn đọc được.
- [x] Test `TestLiveProgressUsesRequestLanguage`, `TestParseRequestLanguage` (thiếu trường → không vỡ request cũ),
      `TestDocDesignStyleCardUsesLanguage`.
- [ ] Còn: chuỗi lỗi trong `Result.JSON` của các tool khác vẫn theo ngôn ngữ model trả - chỉ đổi khi có nhu cầu thật.

## P7-04. Ba lỗi hay gặp có hướng dẫn sửa ngay

- [ ] Sai khóa API, sai endpoint, Core không chạy: câu chữ trong thẻ lỗi đọc lại với người dùng thật, ghi lại chỗ
      vấp.
- [ ] `core-go/internal/doctor/doctor.go`: câu chẩn đoán tiếng Việt dễ hiểu.

## P7-05. Trạng thái Core trên ribbon

- [ ] `src/AxiomOffice/Ribbon/RibbonUi.cs`: hiện xanh/vàng/đỏ theo `getLabel` thay cho phải bấm "Trạng thái"; cập
      nhật khi Core bật/tắt.

## P7-06. WPS Windows: nhãn động

- [ ] Kiểm ribbon WPS có gọi `getLabel` không (17/10/2026: không thấy); nếu không thì nhãn tĩnh, đổi ngôn ngữ phải
      mở lại WPS - ghi rõ hoặc thêm `getLabel`.

## P7-07. Kiểm thử + changelog

- [ ] `python scripts/check_i18n.py` 0 thiếu.
- [ ] Hủy lượt giữa chừng không treo khung chat, không rác session (`tests/core/test_core_e2e.py --only cancel`).
- [ ] Thử với 3 người thật: cài xong và hỏi được AI lần đầu trong 5 phút, ghi lại chỗ vấp.
- [ ] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `08-p7`.

## Thứ tự chạy lệnh

```powershell
python scripts/check_i18n.py
cd core-go; go test ./internal/agent/... ./internal/doctor/...
cd ..; AXIOM_E2E_CORE_EXE=<binary> python tests/core/test_core_e2e.py --only cancel
```
