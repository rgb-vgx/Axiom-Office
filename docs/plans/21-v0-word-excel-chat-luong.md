# 21 / V0 - Chất lượng Word và Excel: audit, baseline, kế hoạch

> Làm theo đề xuất "Document / Spreadsheet Design Engine" (26/10/2026): đo trước, sửa theo lỗi đo được, chưa refactor.
> Phạm vi: **Microsoft Office trên Windows** (Word/Excel 2019 x64). LibreOffice đã dừng; WPS để sau (khi chuyển sang
> máy Linux). Model: `ocg/beam-free` (model yếu đã cấu hình) - không dùng model mạnh để đo.

## 1. Bộ đo (chạy lại được)

| Tệp | Vai trò |
|---|---|
| `tests/bench/office/briefs.json` | 22 ca: 10 Word (`w01`–`w10`), 10 Excel (`x01`–`x10`), 2 đề lớn (`a-word-…`, `b-excel-…`) |
| `tests/live/tasks/word-bao-cao-ai-agent.md`, `excel-ngan-sach-du-an.md` | Đề A (báo cáo 6–8 trang) và đề B (workbook 4 sheet) |
| `tests/bench/office/run.py` | Chạy ca trên Office thật qua `POST /v1/runs` (tương tác, **tự bấm đồng ý** như người dùng), tài liệu dựng sẵn (`seed`), nhiều lượt liên tiếp (`prompts`), lưu `result.docx/.xlsx`, đóng app sau mỗi ca |
| `tests/bench/office/check_docx.py` | Chấm `.docx` bằng XML: Heading thật / tiêu đề giả, mục lục (trường TOC), header/footer + trường PAGE, caption, bảng lặp tiêu đề, đoạn trống liên tiếp, font/cỡ trực tiếp; `--layout` mở Word (chỉ đọc) đo số trang, trang trắng, tiêu đề cuối trang, bảng nhỏ bị cắt |
| `tests/bench/office/check_xlsx.py` | Chấm `.xlsx` bằng XML: ô lỗi, công thức/số cứng theo sheet, Table/bộ lọc, freeze, validation, định dạng có điều kiện, định dạng số theo cột, biểu đồ và vùng nguồn; `--recalc` mở Excel, đổi một ô nhập liệu, xem ô tổng hợp có đổi theo |
| `tests/bench/office/score.py` | Chấm cả lượt theo `expect` của từng ca + `mustContain`/`mustKeep`/`mustRemove`; mất chữ có sẵn = critical |

```bash
# Core khong bat bo nho agent (lan lap phai doc lap):  AXIOM_MEMORY_ENABLED=false
python tests/bench/office/run.py --app word  --label <nhan>
python tests/bench/office/run.py --app excel --label <nhan>
python tests/bench/office/score.py tests/bench/office/results/<nhan>      # --no-app: bo phan mo Word/Excel
```

Mức độ (theo đề xuất): **critical** = sai dữ liệu / sai công thức / mất nội dung / hỏng tệp; **major** = bố cục lỗi rõ
hoặc chức năng quan trọng không có; **minor** = căn chỉnh, khoảng cách, định dạng nhỏ. Một ca **đạt** khi lượt kết thúc
bình thường, không có lỗi critical, đủ mọi `expect` và giữ đúng nội dung.

## 2. Capability matrix (Microsoft Office, đối chiếu code 26/10/2026)

Lệnh live: `catalog/live-commands.json` (bridge C# `src/AxiomOffice/Bridge/CommandDispatcher.*.cs`). Lệnh tệp (không
cần mở app, `core-go/internal/mcpserver/filetools_*.go`) làm trên tệp đóng, **không** sửa tài liệu đang mở.

### Word (`writer.*`: 28 lệnh, 21 cho agent)

| Nhu cầu | Có | Lệnh / bằng chứng |
|---|---|---|
| Đoạn văn có định dạng | Có | `writer.appendText`, `insertStyledText` (định dạng trực tiếp: bold/size/font/màu) |
| Heading 1–9 | Một phần | `writer.heading` **chỉ gõ chữ mới tại con trỏ** (`CommandDispatcher.Writer.cs` `WriterHeading`): không gán style cho đoạn đã có |
| Gán style / đổi cấp cho đoạn có sẵn | **Không** | Không lệnh nào (ca `w08`) |
| Danh sách đánh số / gạch đầu dòng thật | **Không** | `ListFormat` chỉ dùng nội bộ trong `writer.restyle`; agent gõ "1." "-" bằng chữ |
| Bảng | Có | `insertTable`, `formatTable`, `checkTables`; hàng tiêu đề lặp chỉ qua `restyle` (`HeadingFormat`) |
| Caption bảng/hình (trường SEQ) | **Không** | Không có `InsertCaption`; caption là đoạn chữ thường |
| Mục lục tự động | **Không** | Không có `TablesOfContents` (ca `w02`, `a`) |
| Header/footer, số trang | **Không** (riêng lẻ) | Chỉ `writer.restyle` tự thêm "Trang N" ở footer (`RestylePage`), không có lệnh header/footer cho agent (ca `w10`) |
| Lề, khổ giấy, section | **Không** (riêng lẻ) | Chỉ `restyle` đặt lề cố định |
| Ngắt trang | Có | `insertPageBreak` |
| Giữ tiêu đề với đoạn sau | Gián tiếp | Style Heading của Word đã có keep-with-next; `restyle` đặt thêm |
| Soát cấu trúc / phân trang | Một phần | `writer.outline`, `checkTables`; **không** có lệnh soát trang trắng, tiêu đề cuối trang, bảng bị cắt |
| Hoàn tác | Có, **không giới hạn** | `writer.undo` gọi `doc.Undo(1)` `count` lần (`WriterUndo`) - lùi được quá phần việc của lượt |
| Trình bày lại cả bài | Có | `doc_design` → `writer.restyle` + `insertBlocks` |

### Excel (`et.*`: 23 lệnh, 20 cho agent)

| Nhu cầu | Có | Lệnh / bằng chứng |
|---|---|---|
| Ghi giá trị / công thức | Có | `writeRange`, `writeRanges`, `fillRange` (tham chiếu tương đối) |
| Đọc công thức | Có | `readRange {formulas:true}` |
| Định dạng số / ô | Có | `formatRange` (`numFmt`, màu, căn) |
| Định dạng có điều kiện (theo công thức) | Có | `setConditionalFormat` (operator `formula`), `listConditionalFormats` |
| Biểu đồ | Một phần | `addChart` lấy dữ liệu **cùng sheet**; không lấy vùng nguồn ở sheet khác (ca `x09`) |
| Excel Table / bộ lọc | **Không** | Không lệnh `ListObjects.Add` / `AutoFilter` (ca `x05`, `b`); `excel_create_table` chỉ làm trên tệp đóng |
| Freeze panes | **Không** | Không lệnh (ca `x05`, `b`) |
| Data validation | **Không** | Không lệnh (ca `x06`, `b`) |
| Độ rộng cột / autofit | **Không** | Không lệnh |
| Tên vùng (named range), vùng in | **Không** | Không lệnh |
| Soát | Một phần | `checkRange` (tiêu đề trống, kiểu lẫn, số dạng chữ); **không** dò ô lỗi công thức toàn sổ, không thử "đổi đầu vào" |

## 3. Baseline 26/10/2026 (`ocg/beam-free`, 1 lần mỗi ca)

Kết quả: `tests/bench/office/results/baseline-beam-free-2026-10-26/` (`score.md`, `score.json`, từng ca có
`run.json`, `events-N.log`, `result.*`, `check.json`). **Đạt 13/22** (Word 6/11, Excel 7/11).

| Ca | Đạt | Vòng | Billable | Không đạt vì |
|---|---|---|---|---|
| w01 tờ trình ngắn | ✓ | 33 | 35k | (minor: 4 đoạn trống liền để chừa chỗ ký) |
| w02 báo cáo dài | | 29 | 80k | không mục lục, không số trang |
| w03 bảng nhỏ | | 10 | 8k | caption là đoạn chữ thường |
| w04 bảng 30 dòng | ✓ | 45 | 90k | |
| w05 sửa một mục | ✓ | 5 | 9k | |
| w06 chèn mục | ✓ | 9 | 13k | |
| w07 thiếu số liệu | ✓ | 5 | 7k | |
| w08 chuẩn hóa tiêu đề | | 60+ | - | **mất toàn bộ nội dung** (critical) |
| w09 nhiều bước (3 lượt) | ✓ | 81 | 221k | |
| w10 đầu/chân trang | | - | - | không có lệnh; dừng vì chặn lệnh lặp |
| a báo cáo 6–8 trang | | 60+ | - | hết 60 vòng; không TOC/số trang/caption; 1 trang trắng |
| x01 bảng mới + tổng | ✓ | 19 | 16k | |
| x02 tổng hợp chéo sheet | ✓ | 10 | 15k | (tổng hợp đổi theo khi đổi Data) |
| x03 sửa công thức lỗi | ✓ | 7 | 15k | |
| x04 thêm cột giữ dữ liệu | ✓ | 11 | 11k | |
| x05 bảng 200 dòng | | 9 | 15k | không freeze, không bộ lọc (model nói rõ là không có lệnh) |
| x06 danh sách chọn | | - | - | không validation (dùng CF thay); `et.save` đổi tên sổ → `DOCUMENT_CLOSED` |
| x07 cảnh báo ngưỡng | ✓ | 11 | 14k | |
| x08 dữ liệu thiếu | ✓ | 6 | 4k | |
| x09 biểu đồ sheet khác | | 47 | 46k | biểu đồ lấy bảng chép trên Dashboard; 4 biểu đồ (3 rác) + 1 lạc ở Data |
| x10 nhiều bước (3 lượt) | ✓ | 37 | 40k | |
| b workbook 4 sheet | | 60+ | - | hết 60 vòng; không Table/freeze/validation |

Lượt `ai.ask` (không tương tác) chạy trước đó cho đề A và B nằm ở `results/ai-ask-khong-tuong-tac-2026-10-26/`: đề B
dừng sau 57 vòng / 935k token vì mọi lần ghi đè ô **do chính lượt vừa ghi** đều cần xác nhận, mà `ai.ask` tự từ chối.

## 4. Lỗi theo mức độ (bằng chứng)

### Critical

1. **`writer.undo` lùi quá phần việc của lượt → mất nội dung** (`w08`). Model gọi `writer.undo {count:30}` rồi
   `{count:7}`; tài liệu còn 1 ký tự (`events-1.log`). `WriterUndo` không biết bước nào của lượt
   (`CommandDispatcher.Writer.cs:916`). Với người dùng thật: lùi cả phần họ vừa tự gõ.
2. **Lưu sổ mới làm Core mất tài liệu đích** (`x06`). `et.save` trên sổ chưa lưu ghi vào `Documents\Book1.xlsx`, tên
   sổ đổi `Book1` → `Book1.xlsx`; mọi lệnh sau báo `DOCUMENT_CLOSED: the workbook 'Book1' is not open`. Hộp xác nhận
   "AI muốn lưu dù yêu cầu không nhắc" có hiện nhưng runner (như người dùng) bấm đồng ý.

### Major

3. **Chặn lệnh lặp chặn nhầm lệnh ghi hợp lệ** (`identicalCallGuard`, `core-go/internal/model/guards.go`): lệnh ghi y
   hệt bị đếm suốt lượt dù giữa các lần tài liệu đã đổi. `writer.insertPageBreak` (không tham số) bị chặn từ lần 5 khi
   viết báo cáo nhiều chương (đề A: 5 lần); `w10` và lượt `ai.ask` đề B dừng hẳn vì "repeated the same failing call".
4. **Thiếu lệnh Word nền tảng**: gán style cho đoạn có sẵn, mục lục, header/footer + số trang, caption, danh sách thật
   (§2). Đề A, `w02`, `w03`, `w08`, `w10` trượt vì vậy; model xoay xở bằng thay chữ tạm `@@H1@@`, gõ "MỤC LỤC" tay.
5. **Thiếu lệnh Excel nền tảng**: Table/bộ lọc, freeze panes, data validation, biểu đồ lấy vùng sheet khác (§2).
   `x05`, `x06`, `x09`, đề B trượt vì vậy.
6. **Đốt vòng / token**: Word mỗi đoạn một lệnh - tờ trình nửa trang 33 vòng (`w01`), biên bản 3 lượt 81 vòng / 221k
   billable (`w09`), đề A hết 60 vòng; 7/22 ca chạm mốc hỏi "đã chạy 30 vòng, cho chạy tiếp?" (`agent.heavy_run`).
7. **Xác nhận ghi đè ô do chính lượt vừa ghi** (`x01`, `x04`, `x10`, đề B): slide đã miễn cho hình do lượt tạo
   (`owned.go`), ô Excel thì chưa. Trong khung chat thật là hộp thoại thừa; ở `ai.ask` thì làm hỏng cả lượt.

### Minor

8. Văn bản dài hơn yêu cầu: đề A 14 trang (đề 6–8). Bảng nhỏ bị cắt sang trang (đề A: 4 bảng ≤15 dòng).
9. Đoạn trống liên tiếp để dàn trang (`w01`); số tiêu đề gõ tay ("1.1.") thay vì đánh số tự động.
10. Model đôi khi gọi tool tên `assistant` (lỗi sinh của model yếu) - Core trả "unknown tool" (đề A, đề B).

## 5. Kế hoạch cải tiến

Nguyên tắc: không refactor rộng, không thêm hàng loạt lệnh; mỗi việc có test hồi quy trước khi đổi hành vi; đo lại
bằng bộ ở §1 (cùng model, cùng ca) và so với baseline §3.

### P0 - Độ tin cậy (không mất dữ liệu, lượt không tự hỏng)

| Việc | Tiêu chí nghiệm thu |
|---|---|
| **Undo chỉ trong phạm vi lượt**: bridge đếm số bước undo do lượt ghi (mỗi lệnh ghi = 1 bước `UndoRecord`), `writer.undo`/`et.undo` không lùi quá số đó | Test: seed 3 đoạn → lượt ghi 2 lệnh → `undo {count:30}` chỉ lùi 2; `w08` không còn `CONTENT_LOST` |
| **Theo dõi tài liệu qua lưu đổi tên**: `et.save`/`writer.save` trên tài liệu chưa lưu trả tên mới, Core cập nhật tài liệu đích | Test Go: kết quả save có `fullName` mới → lệnh sau nhắm đúng; `x06` không còn `DOCUMENT_CLOSED` |
| **Chặn lệnh lặp chỉ khi lặp liền không đổi gì**: chữ ký lệnh ghi được đếm lại khi giữa hai lần có lệnh ghi khác (như lệnh đọc đã làm) | Test: 6 × `insertPageBreak` xen lệnh ghi khác không bị chặn; 5 × cùng lệnh liền nhau vẫn bị chặn |
| **Miễn xác nhận ghi đè ô do lượt vừa ghi** (mở rộng `owned.go` cho vùng ô `et.*`) | Test: ghi A1 rồi ghi đè A1 trong cùng lượt không hỏi; ô có từ trước vẫn hỏi; `x01`/`x04`/`x10` 0 xác nhận ghi đè |

### P1 - Cấu trúc có ngữ nghĩa (lệnh nền thiếu, mỗi lệnh có ca đo)

| Việc | Ca đo | Tiêu chí |
|---|---|---|
| `writer.setStyle {paragraphs:[n…], style:"Heading 2"}` - gán style cho đoạn có sẵn theo số đoạn của `writer.outline` | `w08` | 3/3 lần: H1=1, H2=3, giữ nguyên chữ |
| `writer.insertToc`, `writer.setHeaderFooter {header, footer, pageNumber}` | `w02`, `w10`, đề A | `toc`, `pageNumber`, `headerText` đạt |
| `writer.insertCaption {label:"Bảng"}` (trường SEQ) hoặc tham số `caption` của `insertTable` | `w03`, đề A | `captions` ≥ số bảng |
| `et.setupTable {range, table:true, freeze:true}` (ListObject + freeze hàng tiêu đề), `et.setValidation {range, list}` | `x05`, `x06`, đề B | `frozen`, `autoFilter`, `listValidation` đạt |
| `et.addChart` nhận vùng nguồn ở sheet khác (`Data!A1:C7`) | `x09` | 1 biểu đồ trên Dashboard, nguồn `Data!` |

### P2 - Tự kiểm và bớt vòng

| Việc | Tiêu chí |
|---|---|
| `writer.checkLayout` (chỉ đọc): số trang, trang trắng, tiêu đề cuối trang, bảng nhỏ bị cắt - cùng phép đo `check_docx --layout` | Đề A: 0 trang trắng, 0 tiêu đề cuối trang trước khi trả lời |
| Soát ô lỗi toàn sổ trong `et.checkRange` (không truyền range) | Ca có lỗi cài sẵn (`x03`) tự thấy hết ô lỗi |
| Viết nhiều đoạn có cấu trúc trong một lệnh (heading + đoạn + danh sách) thay vì mỗi đoạn một lệnh - cùng hướng `slides_from_brief` | `w01` ≤ 10 vòng, `w09` ≤ 30 vòng, đề A ≤ 30 vòng không chạm `heavy_run` |

### P3 - Nâng cao (sau khi P0–P2 đo đạt)

Mẫu theo loại tài liệu (báo cáo, biên bản, tờ trình), dashboard Excel dựng sẵn, định dạng có điều kiện nâng cao; WPS
khi có máy Linux.

## 6. Cách đo sau mỗi thay đổi

1. Chạy lại đúng ca liên quan (`--only`), 3 lần với `ocg/beam-free`; ca P0 chạy cả bộ.
2. `score.py` → so với `baseline-beam-free-2026-10-26/score.json`: số ca đạt, critical, vòng, billable, xác nhận.
3. Ghi kết quả vào `STATUS.md` và mục §3 của tệp này (thêm cột lần đo mới, không xóa baseline).
