# 29 / P8 - Task chi tiết (LibreOffice và WPS Linux)

> Chia nhỏ tới từng hàm. Làm xong task nào đánh dấu [x]. Đối chiếu code 17/10/2026; đọc `09-p8` trước.
> Cùng hợp đồng bridge, cùng tên lệnh; Core, skill, test dùng lại.

## P8-00. Đã có

- [x] Extension Python UNO (`src/AxiomOffice.LibreOffice/`): bridge Writer/Calc/Impress, sidebar Ask AI, wizard
      (awt); 281/281 bài thử trên LibreOffice thật, wizard 20/20 với Core Go.
- [x] WPS Linux: add-in JS nối long-poll về Core, Core mở bridge `:47861`.
- [x] Core Go chạy Linux (mutex `flock`, `~/.config/axiom-office/`); danh mục lệnh sinh từ registry Python.

## P8-01. Trình bày lại Word trên LibreOffice

- [ ] `axiom/writer.py` `_design_unsupported` → làm thật cho `writer.restyle` (tiêu đề mục, gạch đầu dòng, bảng,
      lề trang, số trang) và `writer.insertBlocks` (khối bo tròn nền nhạt).
- [ ] Tham chiếu hành vi C#: `src/AxiomOffice/Bridge/CommandDispatcher.WriterDesign.cs`.
- [ ] Bỏ stub khỏi `writer.py`; cập nhật mô tả lệnh; `python scripts/generate_mcp_commands.py --check` khớp.
- [ ] Khi xong: Core mở `doc_design` trên LibreOffice (`core-go/internal/tools/doc_design.go` không còn chặn theo
      nền tảng).

## P8-02. Dựng slide theo mẫu trên LibreOffice

- [ ] `axiom/impress.py`: 9 lệnh đang `_template_unsupported` - `wpp.newFromTemplate`, `wpp.addSlideFromTemplate`,
      `wpp.sourceManifest`, `wpp.setShapeText`, `wpp.deleteShapes`, `wpp.copyPicture`, `wpp.addParagraphs`,
      `wpp.addGrid`, `wpp.deleteTagged`.
- [ ] Ưu tiên theo nhu cầu: đếm lệnh agent hay gọi nhất trong `tool_calls` (`core.db`), làm trước nhóm đó.
- [ ] Mỗi lệnh làm xong: cập nhật mô tả, bỏ khỏi `_template_unsupported`, `generate_mcp_commands.py --check`.

## P8-03. Hoàn tác trên LibreOffice

- [ ] Kiểm mọi lệnh ghi bọc `XUndoManager.enterUndoContext` (17/10/2026: chỉ thấy **1 chỗ** -
      `axiom/documents.py:66`); rà `writer.py`, `calc.py`, `impress.py` theo lệnh `undo=True`.
- [ ] Mục tiêu: Ctrl+Z sau mỗi thao tác AI hoàn tác đúng một bước.

## P8-04. Khung chat WPS Linux

- [ ] Thẻ nhiều lựa chọn có ô tự gõ (đã có trong code Windows) - thử trên WPS Linux thật; sửa nếu lệch.
- [ ] Dịch giao diện khung chat (chung việc `28-p7` P7-02).

## P8-05. Kiểm thử + changelog

- [ ] `tests/live/test_live_commands.py` xanh trên Linux; bộ LibreOffice (`tests/lo/`) và WPS (`tests/wps/`).
- [ ] `doc_design` chạy trên LibreOffice Writer với giáo án mẫu: kịch bản trong bộ `17` đạt như bản Word.
- [ ] Ghi `CHANGELOG.md [Unreleased]`; cập nhật `09-p8`.

## Ngoài phạm vi

- [ ] macOS, Online/Collabora. Không chạy macro/script trong skill.

## Thứ tự chạy lệnh

```bash
python scripts/generate_mcp_commands.py --check
python tests/live/test_live_commands.py
```
