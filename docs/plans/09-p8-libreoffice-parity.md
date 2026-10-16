# 09 / P8 - LibreOffice và WPS Linux: lấp chỗ trống

> Cùng hợp đồng bridge, cùng tên lệnh. Core, skill, test dùng lại.

## 1. Đã có (đối chiếu code 16/10/2026, STATUS.md)

- Extension Python UNO (`src/AxiomOffice.LibreOffice/`): bridge Writer/Calc/Impress trên cổng 47851/47852/47853,
  sidebar Ask AI, wizard thiết lập (awt); 281/281 bài thử trên LibreOffice thật, wizard 20/20 với Core Go.
- WPS Linux: add-in JS (`src/AxiomOffice.WPS/`) nối long-poll về Core, Core mở bridge `:47861`
  (bản kế hoạch cũ ghi "ngoài phạm vi" - thực tế đã có).
- Core Go chạy Linux (mutex bằng `flock`, cấu hình `~/.config/axiom-office/`), MCP server là `AxiomOffice.Core mcp`.
- Danh mục lệnh dùng chung, sinh từ registry Python (`scripts/generate_mcp_commands.py`).

## 2. Còn thiếu - việc cần làm

- **Lệnh còn để trống phía LibreOffice** (báo "not available on LibreOffice yet"):
  - Trình bày lại Word: `writer.outline`, `writer.restyle`, `writer.insertBlocks` (`_design_unsupported` trong
    `axiom/writer.py`) → Core không mở `doc_design` trên LibreOffice.
  - Dựng slide theo mẫu: các lệnh `_template_unsupported` trong `axiom/impress.py` (`wpp.addGrid`,
    `wpp.deleteTagged`, `wpp.copyPicture` vùng…).
- **Hoàn tác**: kiểm mọi lệnh ghi bọc `XUndoManager.enterUndoContext` (hiện chỉ thấy 1 chỗ trong `axiom/*.py`).
- **Khung chat WPS**: thẻ nhiều lựa chọn có ô tự gõ đã có trong code (16/10/2026) nhưng chưa thử trên WPS Linux
  thật; chưa dịch giao diện.
- Ưu tiên theo nhu cầu người dùng: liệt kê lệnh agent hay gọi nhất trong `tool_calls` mà LibreOffice còn thiếu.

## 3. Tệp sẽ sửa

- `src/AxiomOffice.LibreOffice/python/pythonpath/axiom/{writer,impress,calc}.py`
- `src/AxiomOffice.WPS/*`

## 4. Tiêu chí xong

- `doc_design` chạy trên LibreOffice Writer với giáo án mẫu: kịch bản trong bộ `17` đạt như bản Word.
- Ctrl+Z sau mỗi thao tác AI trên Writer/Calc/Impress hoàn tác đúng một bước.
- `tests/live/test_live_commands.py` xanh trên Linux.

## 5. Ngoài phạm vi

- macOS, Online/Collabora. Không chạy macro/script trong skill.
