# Tính năng còn thiếu so với Claude for Microsoft 365: nghiên cứu và thiết kế

Ngày: **27/10/2026**. Đối chiếu tài liệu chính thức của Claude for Excel/Word/PowerPoint/Outlook
([Word](https://claude.com/docs/office-agents/word.md), [Excel](https://claude.com/docs/office-agents/excel.md),
[Work across apps](https://claude.com/docs/office-agents/work-across-apps)) với catalog lệnh hiện tại của Axiom
(`catalog/live-commands.json`, 100 lệnh) và code Core/bridge. Tài liệu này là **nghiên cứu và thiết kế**, chưa có code.

Giới hạn khi nghiên cứu: môi trường không truy cập được `learn.microsoft.com` và không có Office/WPS. API COM dưới đây
được đối chiếu qua kết quả tìm kiếm trỏ về tài liệu Microsoft (dẫn ở cuối). Mục nào ghi **spike** nghĩa là phải chạy
thử trên Office/WPS thật trước khi quyết định.

## 1. Bảng khoảng cách

| # | Tính năng của Claude for M365 | Axiom hiện có | Khoảng cách | Ưu tiên |
|---|---|---|---|---|
| W1 | Trích dẫn bấm được (tới đoạn trong Word, ô trong Excel) | `writer.outline` có số đoạn; khung chat vẽ link Markdown nhưng **không bấm được** (`Ai/MarkdownView.cs`) | Lệnh nhảy tới đoạn/ô + link bấm được trong pane | **P1** |
| W2 | Đọc và xử lý comment (sửa phần được neo, trả lời trong luồng) | Không có lệnh comment | `listComments`, `replyComment`, `resolveComment`, `addComment` | **P1** |
| W3 | Tóm tắt redline của đối tác | Không đọc được revision | `listRevisions` (chỉ đọc) | **P1** |
| W4 | Chế độ Track Changes: mọi sửa của AI thành revision | Không | `TrackRevisions` + gắn tác giả + duyệt/từ chối; cũng là lời giải cho F03 (undo lùi việc người dùng) | **P2** |
| W5 | Điều hướng theo chủ đề | Model đọc `outline`/`getText` được; thiếu W1 để chỉ chỗ | Chỉ cần W1 + quy tắc prompt | P1 (kèm W1) |
| W6 | Điền mẫu theo style tài liệu | `writer.setStyle`, `writeContent`, `insertBlocks`, `doc_design` | Gần đủ; đo bằng bench | — |
| X1 | Sort, filter | `et.createTable` có nút lọc, nhưng không có lệnh sắp xếp/lọc | `et.sort`, `et.filter` | **P1** |
| X2 | Truy vết công thức, tìm gốc lỗi | `et.readRange {formulas:true}`, `et.checkRange` soát ô lỗi | Lần ngược tiền lệ/hậu duệ | **P1** (làm ở Core) |
| X3 | Pivot table | Không | `et.pivot` | P3 |
| X4 | Data validation, định dạng có điều kiện | Có (`setValidation`, `setConditionalFormat`) | — | — |
| X5 | Trích dẫn ô bấm được | Không | Như W1 (`et.goto`) | P1 (kèm W1) |
| P1 | Theo template/master | Có (`slides_from_template`, `wpp.newFromTemplate`) | — | — |
| P2 | Biểu đồ trong slide, cập nhật từ Excel | Không có lệnh biểu đồ PowerPoint | `wpp.addChart` hoặc chép biểu đồ từ Excel | P3 (spike) |
| C1 | Làm việc xuyên ứng dụng trong một hội thoại | Mỗi lượt gắn một cổng (một app) | Định tuyến lệnh sang phiên khác | **P2** |
| C2 | Connector dữ liệu ngoài | MCP client (người dùng tự cấu hình server) | Đã tương đương về cơ chế | — |
| C3 | Outlook | Không có bridge Outlook | Ngoài phạm vi hiện tại | — |
| C4 | Mac, Office trên web | COM (Windows) + add-in JS WPS (Linux) | Cần một frontend Office.js | P3 (chiến lược) |

Không đề xuất sao chép mọi thứ: X4, P1, W6, C2 đã có tương đương; Outlook ngoài phạm vi sản phẩm hiện tại.

## 2. P1 — làm được ngay, rủi ro thấp

### W1 + X5 + W5. Trích dẫn bấm được và điều hướng

**Hành vi mục tiêu.** Câu trả lời có trích dẫn kiểu `[§12]` (đoạn 12 theo `writer.outline`) hoặc
`[Data!C42]`; bấm vào thì tài liệu cuộn tới và chọn đúng chỗ.

**Lệnh bridge mới (chỉ đọc, không tạo bước undo):**
- `writer.goto {paragraph}`: `doc.Paragraphs(n).Range.Select()` rồi `ActiveWindow.ScrollIntoView(range)`.
- `et.goto {range}`: `Application.Goto(Range("Data!C42"), Scroll:=true)`.
- `wpp.goto {slide}`: `ActiveWindow.View.GotoSlide(n)`.

**Core.**
- Thêm quy tắc prompt: khi trả lời về nội dung tài liệu, chèn trích dẫn theo đúng hai mẫu trên.
- Số đoạn phải lấy từ `writer.outline`, vốn đã đánh số toàn tài liệu.

**Pane.**
- `MarkdownView` nhận dạng `[§n]` / `[Sheet!A1]` (hoặc link `axiom://goto?...`) và vẽ thành link bấm được.
- Bấm thì gọi thẳng `OfficeActionTool.Execute` như `UndoLastRun`, không qua Core.
- Lệnh `goto` là lệnh đọc nên không cần policy hay xác nhận.

**Rủi ro.**
- Số đoạn lệch khi tài liệu đổi sau câu trả lời. Chấp nhận được: bấm vẫn tới gần đúng chỗ. Có thể kèm 30 ký tự đầu
  của đoạn để bridge dò lại.
- `Select()` đổi vùng chọn của người dùng; cần ghi chú trong UX.

**Test.**
- Go: quy tắc prompt có mặt.
- C# live: `goto` chọn đúng đoạn/ô.
- WPS Linux: `commands.js` cài cùng tên lệnh (API `Paragraphs`, `Range.Select` có trong `commands.js` hiện tại).

**Không cần** embedding hay tìm kiếm ngữ nghĩa riêng: model đọc `outline`/`getText` rồi trích dẫn, như Claude for Word
mô tả cho "semantic navigation".

### W2. Comment

**API Word.**
- `Document.Comments`. Mỗi `Comment` có `Range` (vùng được neo), `Author`, `Date`, `Done`
  ([Comment.Done](https://learn.microsoft.com/en-us/office/vba/api/word.comment.done)) và `Replies`
  ([Comment](https://learn.microsoft.com/en-us/office/vba/api/word.comment)).
- `Replies` có từ Word 2013. Microsoft lưu ý: đặt `Done` trên một *reply* không hiện gì trong giao diện comment mới, nên
  chỉ đặt trên comment gốc.

**Lệnh bridge:**
- `writer.listComments {includeDone?}` (đọc): `id` (chỉ số), `author`, `date`, `text`, `anchorText` (cắt 200 ký tự),
  `paragraph` (số đoạn theo `outline`), `done`, `replies[]`.
- `writer.replyComment {comment, text}`: `Comment.Replies.Add(range, text)` (*spike* chữ ký trên Word 2019 x64). Tác giả
  là người dùng Office hiện tại. Nội dung trả lời mở đầu bằng "AI: " để phân biệt.
- `writer.resolveComment {comment}`: `Comment.Done = true`.
- `writer.addComment {paragraph | find, text}`.
- `writer.editCommentScope {comment, text}`: thay chữ trong vùng được neo, giữ định dạng ký tự đầu (giống
  `replaceAll` nhưng đúng vùng).

**Core.**
- Thêm `writer.listComments` vào `noDocumentChange`/`readOnly`.
- `replyComment`/`resolveComment`/`addComment` có tạo bước undo (`UndoRecordScope`).
- Policy: `resolveComment` trên comment của **người khác** thì hỏi xác nhận, theo tinh thần `owned.go`.

**Quy trình "xử lý comment".** Đây là việc của model, không phải code: `listComments` → với từng comment chưa xong thì
`editCommentScope` → `replyComment` ("AI: đã …") → để người dùng tự `resolve`, hoặc resolve khi họ yêu cầu.

**Test.**
- Go: phân loại đọc/ghi, policy.
- Live Word: tài liệu mẫu có 3 comment (một có reply), kiểm danh sách, trả lời, resolve.
- WPS: **spike**. Chưa tìm thấy tài liệu API JS WPS cho `Comments`/`Replies`; thêm bước dò vào `probe.js` trước.

### W3. Đọc redline

**API.**
- `Document.Revisions`; mỗi `Revision` có `Author`, `Date`, `Type`, `Range`, `Accept`, `Reject`
  ([Revision](https://learn.microsoft.com/en-us/office/vba/api/word.revision),
  [Revisions](https://learn.microsoft.com/en-us/office/vba/api/word.revisions)).
- Lưu ý: `Revisions.Count` của tài liệu chỉ đếm *main story*; header, footer và chú thích phải duyệt qua `StoryRanges`.

**Lệnh.**
- `writer.listRevisions {author?, max?}` (đọc): `id`, `type` (chèn/xoá/định dạng/…), `author`, `date`, `text` (cắt),
  `paragraph`, `story`.
- Model tóm tắt và phân loại theo mức độ.

**Ghi (để sau, đi cùng W4):** `writer.reviewRevisions {action: accept|reject, ids | author}`. Luôn hỏi xác nhận khi
đụng revision của người khác.

**Test.** Live Word với tài liệu mẫu có revision của hai tác giả, gồm cả một revision trong footer.

### X1. Sort và filter

**API Excel.**
- `Range.Sort(Key1, Order1, Key2, …, Header:=xlYes)` hoặc `Worksheet.Sort` (`SortFields.Add`, `SetRange`, `Header`,
  `Apply`).
- `Range.AutoFilter(Field, Criteria1, Operator, Criteria2)`, hoặc `ListObject.Range.AutoFilter` với Table do
  `et.createTable` tạo.

**Lệnh.**
- `et.sort {range | table, keys:[{column, order}], header?}`: một bước, mặc định có hàng tiêu đề, nhận `column` theo
  tên tiêu đề hoặc chữ cột.
- `et.filter {range | table, column, values? | criteria?, clear?}`.

**Core / an toàn.**
- Sort **đổi vị trí dữ liệu** của người dùng và không hoàn tác được qua COM ở Excel (F10). Vì vậy:
  - hỏi xác nhận khi vùng có dữ liệu không do lượt ghi (`WrittenCells`);
  - báo trước khi sort một vùng mà có công thức tham chiếu *ra ngoài vùng* (tham chiếu tương đối sẽ trỏ sai sau khi
    đổi hàng). Phát hiện bằng `readRange {formulas:true}` ở Core, cùng bộ phân tích công thức của X2.
- Filter chỉ ẩn hàng nên không hỏi.

**Test.** Go: policy + phát hiện công thức tham chiếu ra ngoài. Live Excel: sort 2 khoá, filter theo giá trị, xoá filter.

### X2. Truy vết công thức — làm ở Core, không cần COM mới

**Lý do làm ở Core.** `Range.DirectPrecedents`/`Dependents` của Excel chỉ theo được trong **cùng sheet**; sang sheet
khác phải dùng `NavigateArrow`, chậm và không chạy trên WPS/LibreOffice. Trong khi đó Core đã đọc được công thức
(`et.readRange {formulas:true}`, `et.readRanges`).

**Thiết kế: tool Core `trace_formula {cell, direction: precedents|dependents, depth?}`.**
1. Đọc công thức của ô.
2. Tách tham chiếu khỏi công thức (A1, `$A$1`, `A1:B5`, `Sheet!A1`, `'Tên có dấu cách'!A1:B5`). `core-go/internal/sheet`
   đã có `ParseCell`/`ParseRange` cho địa chỉ trần; phần tách tham chiếu khỏi công thức (bỏ qua chuỗi trong ngoặc kép,
   tên hàm) và tách tên sheet phải viết thêm.
3. Đọc tiếp các ô được tham chiếu tới độ sâu `depth`.
4. Trả về cây có giá trị và công thức từng nút, đánh dấu nút lỗi (`#REF!`, `#DIV/0!`…) là "gốc lỗi" khi các nút con của
   nó không lỗi.
5. Chiều `dependents` thì quét `usedRange` của các sheet (dùng `et.sheetInfo`), có trần số ô.

**Lợi ích.**
- Chạy trên mọi backend.
- Kiểm thử thuần Go với bridge giả.
- Dùng lại được cho cảnh báo sort ở X1.

**Giới hạn.** Không hiểu tên vùng (named range), `INDIRECT`/`OFFSET` và tham chiếu sang sổ khác. Kết quả phải ghi rõ
"không lần được".

## 3. P2 — giá trị lớn, cần spike trên Office thật

### W4. Chế độ Track Changes cho lượt AI

**Hành vi mục tiêu.**
- Người dùng bật "AI sửa bằng Track Changes" trong pane, hoặc lệnh nhanh tự bật.
- Mọi thay đổi của AI hiện trong ngăn Review với tác giả "Axiom AI"; người dùng chấp nhận hoặc từ chối từng cái.
- "Hoàn tác lượt này" = từ chối các revision của lượt.

**Cơ chế.**
1. Đầu lượt: nhớ trạng thái `Document.TrackRevisions` cũ, bật lên.
2. Gắn tác giả: đặt `Application.UserName = "Axiom AI"` trong lúc chạy lệnh ghi, khôi phục trong `finally`.
   - **Rủi ro lớn nhất:** Microsoft 365 có thể ghi tác giả theo tài khoản đăng nhập thay vì `UserName`, trừ khi bật
     "Always use these values regardless of sign in to Office" (`Options.UseLocalUserInfo`). **Spike bắt buộc.**
3. Phương án dự phòng không cần tác giả:
   - Mỗi lệnh ghi trả về danh sách revision **mới** (so `Revisions` trước/sau lệnh theo vị trí + loại + `Date`).
   - Core lưu tập id này theo lượt.
   - Hoàn tác lượt = `Reject` đúng các revision đó, duyệt ngược.
4. Cuối lượt: trả `TrackRevisions` về trạng thái cũ.

**Vì sao đáng làm.**
- Hoàn tác theo revision không phụ thuộc ngăn undo, nên không lùi vào chữ người dùng gõ xen giữa. Đây là lời giải cho
  F03 trong audit.
- Người dùng duyệt được từng thay đổi bằng giao diện Review quen thuộc.

**Rủi ro cần đo.**
- `writer.restyle`/`doc_design` sinh hàng trăm revision định dạng: cân nhắc tắt Track Changes cho riêng các lệnh trình
  bày lại.
- Tốc độ duyệt `Revisions` trên tài liệu dài.
- Người dùng gõ trong lúc lượt đang chạy cũng thành revision (với tác giả của họ, hoặc "Axiom AI" nếu `UserName` đang
  bị đổi). `ComGate` tuần tự hoá lệnh, nhưng người dùng gõ phím không đi qua `ComGate`. Phải đo.
- WPS: **spike** (`TrackRevisions` có trong giao diện WPS; chưa có tài liệu API JS).

**Test live (Word).**
- Seed 3 đoạn → lượt sửa 2 chỗ ở chế độ Track Changes → người dùng gõ "X" → "Hoàn tác lượt này".
- Kỳ vọng: 3 đoạn seed nguyên vẹn, 2 revision bị từ chối, "X" còn.

### C1. Làm việc xuyên ứng dụng

**Hiện trạng.** Một lượt gắn một `office.port` (`Request.Port`). `Manager.TryStart` khoá theo port. `office.Directory`
đã biết mọi phiên đang mở (app, cổng, tài liệu).

**Thiết kế tối thiểu.**
1. System prompt có thêm một dòng cho mỗi tài liệu khác đang mở, ví dụ: "Also open: Excel 'Bao cao Q3.xlsx'
   (target: et:Bao cao Q3.xlsx)".
2. Progressive disclosure: một tool `use_document {target}` trả về chữ ký lệnh của app đó trong **kết quả tool**,
   không thêm vào danh sách tool, để không phá cache tiền tố.
3. `office_action` nhận thêm `target` (tuỳ chọn). Core định tuyến tới cổng của phiên đó, kiểm lệnh theo catalog của app
   đó, và gắn tài liệu đích như `target.go`.
4. Khoá: lần đầu đụng một cổng khác, Core xin khoá cổng đó (cùng cơ chế `TryStart`). Cổng đang có lượt khác thì báo bận
   cho model, không chờ.
5. Đếm undo, `WrittenCells`, `owned` đã theo app; cần thêm theo cổng nếu hai cửa sổ cùng app.

**Rủi ro.**
- Prompt và danh sách lệnh dài thêm (giảm bằng mục 2).
- Hủy lượt phải mở khoá mọi cổng đã đụng.
- Pane của app thứ hai cần biết đang bị một lượt khác dùng (sự kiện SSE `run.started` theo cổng).

**Test.** e2e Python với hai bridge giả (et + wps): lượt lấy số từ Excel ghi vào Word; lượt thứ hai trên cổng Excel
trong lúc đó nhận 409.

## 4. P3 — lớn hoặc chưa rõ khả thi

- **X3 Pivot:** `PivotCaches.Create(xlDatabase, SourceData)` → `CreatePivotTable(TableDestination, TableName)` →
  `PivotFields(...).Orientation`/`AddDataField`. Nên làm một lệnh khai báo
  `et.pivot {source, destination, rows, columns, values:[{field, func}]}`. Spike WPS ET. Ưu tiên thấp vì bench hiện chưa
  có ca nào cần.
- **P2 Biểu đồ PowerPoint:** `Shapes.AddChart2` mở một sổ dữ liệu Excel nhúng (`ChartData.Activate`), chậm và dễ treo
  qua COM. Phương án nên thử trước: chép biểu đồ từ Excel sang (`ChartObject.Copy` → `Slide.Shapes.Paste`), đi cùng C1.
  Spike cả hai.
- **C4 Mac / Office trên web:** cần một add-in Office.js mới. API Office.js của Word có track changes và comments; của
  Excel có pivot, sort, filter. Add-in gọi Core qua `http://127.0.0.1`, nhưng trang add-in bắt buộc HTTPS, nên việc
  gọi về localhost tuỳ webview (Edge WebView2 / WKWebView trên Mac): **spike**. Đây là việc chiến lược (một frontend thứ
  tư), chỉ nên làm khi có người dùng Mac hoặc Microsoft 365 web thực sự cần.

## 5. Lộ trình đề xuất

| Bước | Việc | Phụ thuộc | Kiểm chứng được trên Linux? |
|---|---|---|---|
| 1 | X2 `trace_formula` ở Core | — | **Có** (Go + bridge giả) |
| 2 | Quy tắc trích dẫn trong prompt + `writer.goto`/`et.goto`/`wpp.goto` + link bấm được trong pane | — | Core: có; C#/WPS: cần build Windows + WPS |
| 3 | X1 `et.sort`/`et.filter` + cảnh báo sort làm lệch công thức (dùng bộ phân tích của bước 1) | 1 | Core: có; bridge: Excel thật |
| 4 | W2 comment + W3 đọc revision | Spike `Replies.Add` trên Word 2019 | Không (Word thật) |
| 5 | Spike W4 (tác giả revision trên Microsoft 365 và Office 2019) → quyết định làm W4 | 4 | Không |
| 6 | C1 xuyên ứng dụng (định tuyến + `use_document` + khoá nhiều cổng) | — | **Có** (e2e hai bridge giả) |
| 7 | P3 tuỳ nhu cầu | 6 | — |

Mỗi bước thêm ca vào bộ đo `tests/bench/office` trước khi làm, theo đúng nguyên tắc của plan 21, và đo lại sau khi làm.

## 6. Trạng thái triển khai (27/10/2026)

Nhánh `claude/inspiring-davinci-1fxpeg`. **Mã C# chưa được biên dịch** (môi trường làm việc không có .NET SDK): phải
qua CI Windows và chạy trên Office thật trước khi phát hành. Mã Go, JS và e2e Python đã chạy thật trên Linux.

| # | Đã làm | Kiểm chứng | Còn lại |
|---|---|---|---|
| X2 | Tool `trace_formula` ở Core (`tools/trace_formula.go`, `formula_refs.go`) | Go test với bridge giả | — |
| W1/X5/W5 | `writer.goto`/`et.goto`/`wpp.goto`; quy tắc trích dẫn trong prompt; link bấm được trong pane Windows và WPS | Go + test WPS (`test_addin_goto.mjs`, `test_pane.mjs`) | Build C#; thử trên Office thật |
| X1 | `et.sort`/`et.filter` (C#); Core hỏi trước khi sắp xếp dữ liệu có sẵn hoặc khi công thức tham chiếu ra ngoài vùng theo hàng tương đối (`tools/sort_guard.go`) | Go test | Build C#; WPS chưa có handler |
| W2/W3 | `writer.listComments/addComment/replyComment/resolveComment/editCommentScope`, `writer.listRevisions/reviewRevisions` (C#); policy hỏi trước khi duyệt revision, đóng comment khi người dùng không yêu cầu | Go test policy | Spike `Replies.Add` trên Word 2019; WPS chưa có handler |
| W4 | `writer.setTracking` (C#, tác giả "Axiom AI", trả lại trạng thái cũ cuối lượt); `TrackChangesEnabled` / `options.trackChanges`; `POST /v1/runs/{id}/undo` từ chối revision của lượt (theo tác giả + thời điểm), lượt không theo dõi thì lùi `undoSteps`; nút "Hoàn tác lượt này" của pane Windows đi qua Core | Go test | **Spike tác giả** trên Microsoft 365 (`UseLocalUserInfo`); test live mục W4; WPS chưa có handler; chưa có công tắc trong pane (chỉ qua cấu hình) |
| C1 | Tool `other_document` (`tools/other_document.go`) thay cho `office_action {target}` + `use_document` của thiết kế: mô tả tool liệt kê tài liệu khác đang mở, `action: "commands"` trả lệnh của app đó (tiết lộ dần), lệnh chạy qua `office_action` + `RunContext` riêng nên mọi guard áp dụng cho tài liệu đó | Go test + e2e `cross_app` (hai bridge giả) | Không khoá cổng thứ hai: sửa tài liệu đang có lượt khác chạy thì bị từ chối, và sửa tài liệu khác luôn hỏi người dùng một lần mỗi tài liệu mỗi lượt |
| X3 | `et.pivot` (C#): PivotTable trên sheet mới, fallback `PivotTableWizard` | — | Build C#; thử trên Excel 2019; WPS chưa có handler |
| P2 | `et.exportChart` (C#): xuất PNG vào thư mục tạm; agent chèn bằng `wpp.addImage`/`writer.insertImage` qua `other_document` (ảnh, không phải biểu đồ sửa được) | — | Build C#; biểu đồ gốc sửa được trong PowerPoint vẫn là spike |
| C3, C4 | Không làm (ngoài phạm vi / chiến lược) | — | — |

## Nguồn

- [Claude for Word — tài liệu chính thức](https://claude.com/docs/office-agents/word.md)
- [Claude for Excel — tài liệu chính thức](https://claude.com/docs/office-agents/excel.md)
- [Work across M365 apps](https://claude.com/docs/office-agents/work-across-apps)
- [Comment.Done property (Word)](https://learn.microsoft.com/en-us/office/vba/api/word.comment.done)
- [Comment object (Word)](https://learn.microsoft.com/en-us/office/vba/api/word.comment)
- [Revision object (Word)](https://learn.microsoft.com/en-us/office/vba/api/word.revision)
- [Revisions object (Word)](https://learn.microsoft.com/en-us/office/vba/api/word.revisions)
- [Document.TrackRevisions (Word)](https://learn.microsoft.com/en-ca/office/vba/api/word.document.trackrevisions)
- [Selection.NextRevision — ví dụ duyệt revision theo tác giả](https://msdn.microsoft.com/en-us/library/ff834850)
- [Add and reply to comments in Word 2013](https://www.bettercloud.com/monitor/the-academy/add-and-reply-to-comments-in-word-2013/)
- [WPS Office — Track Changes](https://www.wps.com/academy/application-of-track-changes/1862164/)
