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

## 5.1 Kết quả P0 (26/10/2026, đo lại 8 ca, `results/p0-beam-free-2026-10-26/`)

Đã làm cả 4 việc P0 + một lỗi lộ ra khi đo (mỗi việc có test hồi quy, test hỏng khi gỡ bản sửa):

- **Undo trong phạm vi lượt** (`core-go/internal/tools/undo.go`): Core đếm số bước hoàn tác do lượt tạo (lệnh có `undo`
  trong catalog - `TestMakesUndoStepMatchesCatalog` đối chiếu), `*.undo` không lùi quá số đó, lượt chưa sửa gì thì
  từ chối. Excel cần nhất: `app.Undo()` chỉ lùi thao tác giao diện của người dùng.
- **Theo dõi tài liệu khi lưu đổi tên** (`savedDocument`, `target.go`): `save`/`saveAs` trả `fullName` khác → tài liệu
  đích của lượt đổi theo (trừ khi lưu một tài liệu khác).
- **Chặn lặp chỉ khi lặp liền** (`identicalCallGuard.writeAt`, `guards.go`): lệnh ghi y hệt được đếm lại khi giữa hai
  lần có lệnh ghi khác.
- **Miễn xác nhận ô do lượt ghi** (`RunContext.WrittenCells`, `overwrite.go`): ô đã ghi bằng `writeRange`/`writeRanges`/
  `fillRange` trong lượt thì ghi đè không hỏi; ô có từ trước vẫn hỏi.
- **`doc_design` từ chối tài liệu gần như trống** (< 400 ký tự): đo lại đề A lần đầu, model gọi nó sau một bảng, kiểu
  "basic" kết thúc lượt bằng câu trả lời sẵn - báo cáo dừng ở 1 trang.

| Ca | Baseline | Sau P0 |
|---|---|---|
| w08 chuẩn hóa tiêu đề | **mất toàn bộ nội dung** (critical), 60+ vòng | không mất chữ (undo bị giới hạn); vẫn trượt vì thiếu lệnh gán style (P1) |
| w10 đầu/chân trang | dừng vì chặn lệnh lặp | chạy hết lượt (14 vòng); vẫn trượt vì thiếu lệnh header/footer (P1) |
| a báo cáo 6–8 trang | 5 lần ngắt trang bị chặn, 60+ vòng | 7 lần ngắt trang không bị chặn, 0 lệnh lỗi; vẫn hết 60 vòng (P2: mỗi đoạn một lệnh) |
| x01 bảng mới + tổng | ✓, 19 vòng, 1 xác nhận | ✓, **7 vòng, 0 xác nhận** |
| x04 thêm cột | ✓, 11 vòng, 1 xác nhận | ✓, **6 vòng, 0 xác nhận** |
| x06 danh sách chọn | `DOCUMENT_CLOSED` sau `et.save`, lượt hỏng | chạy hết (8 vòng); trượt vì thiếu validation (P1) |
| x10 nhiều bước | ✓, 2 xác nhận | ✓, 1 xác nhận (ô tham số do **lượt trước** ghi - lượt mới vẫn hỏi, đúng thiết kế) |
| b workbook 4 sheet | 60+ vòng | 60+ vòng; thiếu Table/freeze/validation (P1) |

Không còn lỗi critical nào trong 8 ca. Ca trượt còn lại đều do thiếu lệnh (P1) hoặc số vòng (P2).

## 5.2 Kết quả P1 (26/10/2026, đo lại 9 ca, `results/p1-beam-free-2026-10-26/`)

Lệnh mới (C# `CommandDispatcher.WriterStructure.cs`, `CommandDispatcher.SpreadsheetStructure.cs`; catalog + bản sinh):

- **Word**: `writer.setStyle {style, paragraphs}` (style dựng sẵn theo hằng `WdBuiltinStyle`, không phụ thuộc ngôn ngữ
  giao diện), `writer.insertToc {levels, before}` (gọi lại = cập nhật), `writer.setHeaderFooter {header, footer,
  pageNumber}`, `writer.insertCaption {text, label, before}` (style Caption + trường SEQ; mặc định cuối bài, `before`
  = chú thích bảng có sẵn). `insertTable` luôn lặp hàng tiêu đề; **từ chối ô công thức** (`=C2/B2*100` - bảng Word
  không tính, ca `w03` từng in nguyên chuỗi).
- **Thứ tự viết nối tiếp**: `appendText` đưa con trỏ về cuối chữ vừa nối; `heading`/`insertTable` sang đoạn mới khi
  con trỏ ở cuối một đoạn có chữ - trước đó bảng gọi sau `appendText` nằm trên đoạn vừa nối.
- **Excel**: `et.createTable` (ListObject, có lọc), `et.freezePanes {rows, cols}`, `et.setValidation` (danh sách gõ
  thẳng / `source` vùng ở sheet khác / số, ngày trong khoảng), `et.addChart` nhận vùng sheet khác (`Data!A1:C7`).
- Skill `the-thuc-van-ban`, `bao-cao-du-lieu` chỉ cách dùng; khoá vùng bôi đen từ chối `insertToc`,
  `setHeaderFooter`, `insertCaption`, `setStyle` có `paragraphs`.
- Test live `tests/live/test_office_structure.py` (14 kiểm tra, Word + Excel thật, gọi thẳng bridge): **14/14**;
  `test_live_commands.py --apps word,excel` **211/211**.

| Ca | Baseline | Sau P0 | Sau P1 |
|---|---|---|---|
| w02 báo cáo dài | không TOC, không số trang | - | TOC + số trang ✓; còn 1 trang trắng cuối (P2) |
| w03 bảng nhỏ | caption chữ thường | - | số % đúng (công thức bị từ chối); model không gọi `insertCaption` |
| w08 chuẩn hóa tiêu đề | **mất nội dung**, 60+ vòng | không mất chữ, 60+ vòng | **✓ 5 vòng** |
| w10 đầu/chân trang | dừng vì chặn lặp | 14 vòng, trượt | **✓ 4 vòng** |
| a báo cáo 6–8 trang | 60+ vòng, không TOC/số trang/caption | 60+ vòng | 0 lỗi major; còn 11 trang (đề 6–8) và 60 vòng (P2) |
| x05 bảng 200 dòng | không freeze/lọc | - | **✓ 7 vòng** |
| x06 danh sách chọn | `DOCUMENT_CLOSED` | thiếu validation | **✓ 8 vòng** |
| x09 biểu đồ sheet khác | bảng chép trung gian, biểu đồ rác | - | **✓ 10 vòng** (biểu đồ đọc thẳng `Data!`) |
| b workbook 4 sheet | 60+ vòng, thiếu Table/freeze/validation | 60+ vòng | **✓ 51 vòng** |

**Đạt 6/9 ca P1** (trước P1: 0/9). Toàn bộ 22 ca chưa đo lại một lượt.

Lỗi có sẵn phát hiện thêm: `tests/live/test_writer_format.py` trượt "thân bài căn đều" (18/19) cả trên code trước P1.
Đã rõ ở P2 (§5.3): lỗi của **test**, không phải của lệnh - style Normal của tài liệu đã căn đều nên Word không ghi `jc`
vào đoạn; test giờ đọc căn lề thực tế (đoạn → style → basedOn), 19/19.

## 5.3 Kết quả P2 (27/10/2026, `results/p2-beam-free-2026-10-27/`, `results/p2b-beam-free-2026-10-27/`)

Lệnh mới (C# `CommandDispatcher.WriterLayout.cs`; catalog + bản sinh), chỉ trên Office Windows (WPS để sau):

- **`writer.writeContent {blocks}`**: viết nhiều khối nối tiếp vào cuối bài trong **một lệnh, một bước hoàn tác** -
  `{heading, level}`, `{text}` (căn đều mặc định), `{bullets}` / `{numbers}` (danh sách thật của Word, đoạn sau không
  bị kéo vào danh sách), `{table, caption}`, `{pageBreak}`. Soát hết khối (ô công thức, mảng rỗng) **trước** khi ghi.
- **`writer.checkLayout {maxPages}`** (chỉ đọc): số trang/chữ, trang của từng tiêu đề, và lỗi kèm cách sửa:
  `blank-page` (đoạn trống / dấu ngắt nằm trên trang trắng), `heading-at-page-bottom`, `split-table` (bảng ≤ 15 hàng bị
  cắt), `empty-paragraphs` (đoạn trống liên tiếp), `too-long` (dài hơn `maxPages`, kèm số chữ cần cắt).
- **`writer.deleteParagraphs {paragraphs}`**: xoá đoạn theo số (đoạn cuối bài: giữ style + định dạng đoạn trước); từ
  chối đoạn trong bảng / mục lục **trước** khi xoá; lỗi giữa chừng thì lùi phần đã xoá (tất cả hoặc không).
- **`writer.addTableRows {table, rows, before}`**: thêm hàng vào bảng có sẵn (lộ ra khi đo `w09`: model thay chuỗi có
  dấu ô, xoá đoạn trong bảng... hết 60 vòng mà bảng vẫn 4 hàng).
- `insertPageBreak {before}` (đặt "ngắt trang trước" cho đoạn có sẵn), `formatTable {keepTogether}`; `insertTable` tự
  giữ bảng ≤ 15 hàng trên một trang; chú thích (`insertCaption`) đi liền bảng; `insertStyledText` dạng khối sang đoạn
  mới khi con trỏ ở cuối một đoạn có chữ (trước đó dính vào đoạn của `appendText`).
- **Excel**: `et.checkRange` không truyền `range`/`sheet` soát thêm **ô lỗi ở mọi sheet** (`error-cells`:
  `Tinh!A2 #DIV/0!`).
- Core: `writer.checkLayout` là lệnh đọc/kiểm chứng (plan mode, `Verifies`), tối đa 10 lần/lượt; `writeContent` miễn
  đếm "cùng một việc"; lời nhắc tự kiểm nhắc `writer.checkLayout`. Skill `the-thuc-van-ban`: viết cả mục bằng
  `writeContent`, **ngân sách chữ theo số trang** (~280 chữ/trang A4 Times 13), `checkLayout {maxPages}` trước khi trả
  lời, số thập phân kiểu Việt Nam; `bao-cao-du-lieu`: `et.checkRange` không range cho sổ nhiều sheet.
- Test live `tests/live/test_office_layout.py` (22 kiểm tra, Word + Excel thật): **22/22**; `test_office_structure.py`
  14/14; `test_writer_format.py` 19/19; `test_live_commands.py --apps word,excel` 210/210.

| Ca | Baseline | Sau P1 | Lần đo P2 đầu | Sau sửa (P2b) | Tiêu chí P2 |
|---|---|---|---|---|---|
| w01 tờ trình | ✓ 33 vòng | - | **✓ 15 vòng** | - | ≤ 10 vòng: chưa đạt |
| w02 báo cáo dài | thiếu TOC/số trang | trang trắng cuối | **✓** (`checkLayout` → `deleteParagraphs`) | - | 0 trang trắng ✓ |
| w03 bảng nhỏ | không caption | không caption | caption ✓, ghi "98.4" | **✓ 4 vòng** | |
| w09 biên bản 3 lượt | ✓ 81 vòng / 221k | - | ✗ không có lệnh thêm hàng | **✓ 15 vòng / 51k** | ≤ 30 vòng ✓ |
| x03 sửa công thức | ✓ 7 vòng | - | **✓ 5 vòng** | - | |
| a báo cáo 6–8 trang | 60+ vòng | 60 vòng, 11 trang | 30 vòng, 11 trang (model thấy số trang, cho là "chấp nhận được") | **✓ 18 vòng, 8 trang**, 0 trang trắng / tiêu đề cuối trang / bảng bị cắt | ≤ 30 vòng ✓ |

Sau P2 cả 6 ca đều đạt (mỗi ca 1 lần đo). Toàn bộ 22 ca chưa đo lại một lượt.

## 5.4 Kết quả P3 (27/10/2026, `results/p3*-beam-free-2026-10-27/`)

Chỉ Office Windows (WPS để sau). Bộ đo thêm 4 ca P3: `w11` tờ trình, `w12` công văn, `x11` dashboard, `x12` tô màu theo
mức (26 ca). `check_docx` tách bảng dàn trang (tem `AxiomLayout`) khỏi bảng dữ liệu, nhận phần đầu NĐ 30 và "Nơi nhận";
`check_xlsx` đếm công thức đọc sheet khác (`readsSheets`); `score.py` thêm `adminHeader`, `recipients`, `cfTypes` (cho phép
"một trong các kiểu"), `chartsOn`, `readsSheet`, `maxCharts`.

- **Mẫu văn bản theo loại** - tool Core `doc_from_brief` (`core-go/internal/tools/doc_brief.go`): tờ trình, công văn,
  thông báo, biên bản, báo cáo, kế hoạch đúng thể thức NĐ 30/2020. Model khai báo thông tin + nội dung (khối như
  `writeContent`); tool dựng phần đầu (cơ quan chủ quản thường, cơ quan ban hành IN HOA đậm, số ký hiệu | quốc hiệu, tiêu
  ngữ đậm, địa danh ngày tháng nghiêng - "05/02" theo NĐ 30), tên loại + trích yếu, Kính gửi, thông tin cuộc họp (biên
  bản), nội dung, phần ký (nơi nhận | chức vụ, họ tên; biên bản: thư ký | chủ trì), số trang + mục lục (báo cáo, kế
  hoạch), rồi `checkLayout`. Chỗ người dùng chưa cho → "…" + `placeholders`, không bịa. Từ chối tài liệu đã có chữ;
  hỏng giữa chừng thì chỉ số bước `writer.undo`. Lệnh nền mới `writer.insertLayoutTable` (bảng không viền, định dạng
  từng dòng trong ô).
- **Dashboard dựng sẵn** - tool `dashboard_from_data` (`dashboard.go`): ô KPI là **công thức** đọc sheet dữ liệu
  (`=SUM(Data!$B$2:$B$13)`, `{label, column, agg, format}`), 1-4 biểu đồ đọc thẳng vùng dữ liệu (cột x + cột y, vùng
  rời), xếp lưới; sheet đã có chữ thì từ chối, `replace: true` dựng lại dashboard chính lượt đã dựng.
  `et.formatRange {columnWidth}`, `et.deleteSheet` (sheet do lượt tạo hoặc trống thì xoá thẳng, sheet có dữ liệu của
  người dùng thì hỏi), `et.addChart` nhận vùng rời mỗi phần có tên sheet (`Data!A1:A13,Data!D1:D13`).
- **Định dạng có điều kiện nâng cao** - `et.setConditionalFormat {type}`: `colorScale` (`colors` hoặc
  `minColor/maxColor`), `dataBar`, `iconSet`, `top`/`bottom {rank, percent}`, `aboveAverage`/`belowAverage`,
  `duplicate`/`unique`; gọi lại chỉ thay rule **đúng vùng đó** (trước đây xoá cả rule của vùng chỉ chồng một phần).
- **Core**: lệnh gọi có đối số không phải JSON (model cắt giữa chừng) không chạy, model nhận lỗi rõ, lịch sử gửi lại
  mang `{}` - trước đó nhà cung cấp từ chối CẢ request (HTTP 400, ca `x11` lần đầu hỏng lượt).
- Skill: `van-ban-hanh-chinh` (có `doc_from_brief` thì gọi ngay - lần đầu `w11` model kẹt gọi `writer.getText` 7 lần theo
  checklist cũ), `bao-cao-du-lieu`, `mo-hinh-nhieu-sheet`.
- Test: Go (bridge giả) `doc_brief_test.go`, `dashboard_test.go`, `TestInvalidArgumentsNotExecutedNorResent`; live
  `tests/live/test_office_templates.py` (chạy cả test Go `-tags live` trên Office thật) **32/32**;
  `test_live_commands.py --apps word,excel` 211/211; `test_office_layout.py` 22/22; `test_office_structure.py` 14/14.

| Ca | Trước P3 | Lần đo P3 đầu | Sau sửa |
|---|---|---|---|
| w01 tờ trình ngắn | ✓ 15 vòng (P2) | **✓ 2 vòng** (`doc_from_brief`) | |
| w09 biên bản 3 lượt | ✓ 15 vòng (P2) | ✓ 30 vòng | |
| w11 tờ trình đúng thể thức | - | ✗ kẹt `getText` theo checklist skill cũ | **✓ 3 vòng** |
| w12 công văn | - | **✓ 11 vòng** | |
| x11 dashboard | - | ✗ đối số JSON bị cắt → HTTP 400 | ✗ dựng tay (không xoá/dựng lại được) → **✓ 17 vòng** (`deleteSheet`, `replace`) |
| x12 tô màu theo mức | - | ✗ `top` trên cả hàng xoá thang màu B, thanh C | **✓ 22 vòng** |
| b workbook 4 sheet | ✓ 51 vòng (P1) | ✗ đổi tên `Sheet1` qua lại (không có lệnh xoá sheet) | đủ mọi tiêu chí nhưng hết 60 vòng (20 `readRange`, 19 `formatRange` từng ô) |

**Sau P3: 6/7 ca đạt** (mỗi ca 1 lần đo). Còn lại: đề B tốn vòng vì định dạng từng ô (gợi ý tiếp: định dạng nhiều vùng
trong một lệnh).

### 5.5 Định dạng nhiều vùng một lệnh (27/10/2026)

- **`et.formatRanges {formats: [{range, sheet?, bold, italic, fontSize, fontColor, fillColor, numFmt, horizontal, wrap,
  columnWidth}], sheet?}`**: cùng thuộc tính như `et.formatRange`, nhiều vùng / nhiều sheet trong một lời gọi; kiểm hết
  trước (sheet có, địa chỉ đúng, màu `#RRGGBB`, mục có ít nhất một thuộc tính) - một mục sai thì không vùng nào đổi.
- Core: từ lần `et.formatRange` thành công thứ 3 trong lượt, kết quả có `hint` nhắc gom phần còn lại vào `formatRanges`.
- Skill `bao-cao-du-lieu`, `mo-hinh-nhieu-sheet`, `bang-diem`: định dạng tĩnh của mọi sheet gom một lần gọi.
- Sửa thêm hai lỗi lộ ra khi đo lại đề B:
  - `et.createTable` từ chối khi hàng tiêu đề có ô trống (Excel tự đặt `Column1` và ô tiêu đề bảng không xoá trắng được -
    model ghi `""` vào `Config!B1` hơn 10 lần).
  - `et.checkRange` lấy định dạng số của **ô số lẻ đầu tiên** trong cột, không luôn lấy hàng 2 (sheet cấu hình có hàng 2
    trống: ô 0.9 đã để 0% vẫn bị báo `no-number-format`).
- Test: Go `TestFormatRangeHintsBatchFromThirdCall`; live `test_office_templates.py` (+6: 4 vùng 2 sheet, màu sai,
  sheet sai, mục rỗng, đọc lại màu / định dạng số trong xlsx) 18/18 Excel, `test_office_structure.py` (+3) Excel 8/8,
  `test_live_commands.py` 294/294 (kiểm "mọi lệnh có test" nay quét cả `test_office_*.py`).

| Lần đo đề B | `formatRange(s)` | Kết thúc | Lỗi chấm (min) |
|---|---|---|---|
| P3 (trước) | 19 `formatRange` | hết 60 vòng | - |
| `formatRanges` | 1 `formatRanges` + 2 | hết 60 vòng: ghi `""` vào `B1` của bảng (Column1) ~10 lần | 4 |
| + `createTable`/`checkRange` | 6 `formatRanges` + 4 | hết 60 vòng (KPI từng ô bằng `fillRange`, đổi giả định để thử) | 1 |

Đề B vẫn chưa đạt: phần định dạng đã gọn, số vòng còn lại là các bước khác của một đề lớn với model gần như một lệnh
mỗi vòng. Lần gọi `formatRanges` đầu gửi vùng của nhiều sheet mà không ghi `sheet` từng mục nên phải làm lại theo sheet.

## 6. Cách đo sau mỗi thay đổi

1. Chạy lại đúng ca liên quan (`--only`), 3 lần với `ocg/beam-free`; ca P0 chạy cả bộ.
2. `score.py` → so với `baseline-beam-free-2026-10-26/score.json`: số ca đạt, critical, vòng, billable, xác nhận.
3. Ghi kết quả vào `STATUS.md` và mục §3 của tệp này (thêm cột lần đo mới, không xóa baseline).
