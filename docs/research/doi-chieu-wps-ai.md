# Đối chiếu WPS AI với Axiom trên WPS Linux: khoảng cách và thiết kế

Ngày: **31/10/2026**. Đối chiếu bộ "WPS AI" cài sẵn trong WPS Office for Linux 12.1.2.29182
(`/opt/kingsoft/wps-office/office6/addons/*ai*`, 33 addon) với những gì Axiom làm được **trên WPS Linux** hôm nay:
lệnh bridge có handler ở add-in WPS (`src/AxiomOffice.WPS/commands.js`), tool của Core (`core-go/internal/tools`),
skill (`skills/`), nút ribbon và menu chuột phải (`src/AxiomOffice.WPS/ribbon.xml`), lệnh nhanh *Sửa bằng AI*
(`core-go/internal/agent/quickactions.go`). Tài liệu này là **nghiên cứu và thiết kế (giai đoạn 1)**, chưa có code.
Phạm vi chỉ WPS Linux; không xét LibreOffice (đóng băng) và Windows.

**Cách đọc tính năng WPS AI.** Không có tài liệu công khai nên đọc thẳng chuỗi giao diện tiếng Anh trong
`mui/en_US/*.qm` của từng addon (bộ đọc định dạng Qt `.qm`, chỉ đọc, 1.476 chuỗi từ 12 addon có `.qm`). Addon không có
`.qm` (`ketaidataanalysis`, `ketaitoolbox`, `kwebaipdfinsight`, `kpdfaisearch`, `kwebsuwellaidocument`,
`kwebaigenerateimage`, `kwebaireport`) là trang web nhúng; mô tả theo tên addon và nhiệm vụ được giao, ghi rõ ở bảng.

**Bằng chứng phía Axiom** ghi `file:dòng`. "Có trên WPS" nghĩa là add-in WPS có handler; lệnh nằm trong
`CHUA_CO_HANDLER` (`tests/wps/test_addin.mjs`) thì **chưa** có trên WPS, và Core ẩn tool cần lệnh đó
(`core-go/internal/tools/needs.go`; bảng sinh tự động ở `docs/architecture/wps-bridge.md` §6).

**Ghi chú spike:** mục ghi *spike* phụ thuộc API WPS JS chưa đo; phải chạy thử trên WPS thật (Xvfb + HOME tạm, cách đo ở
`tests/wps/REFERENCE.md`) trước khi quyết định. Lượt spike 28/10 đã chứng minh có: `Document.TrackRevisions`,
`Document.Comments.Add`, `Document.Revisions`, `Range.Sort`, `Range.AutoFilter`, `Workbook.PivotCaches()`, `Chart.Export`
(`docs/audits/wps-linux-live-test-2026-10-28-run2.md` §5). **Chưa đo:** API bảng của Writer (`Table.Style`,
`AutoFitBehavior`, `Rows.HeadingFormat`), style đoạn (`Paragraph.Style`, tên style dựng sẵn), mục lục (`TablesOfContents`).

## Phát hiện đáng chú ý

1. **WPS AI không soát chính tả/ngữ pháp tiếng Việt.** `kwpsaiwritingsuggest` ghi: *"AI capabilities are only supported in
   English, Spanish, Chinese, French, Portuguese, Malay, and Indonesian. The language of the current Document only supports
   dictionary checks."* — tiếng Việt chỉ được soát bằng từ điển. Phần dịch (`kpubaigcbox`) thì có "Vietnamese".
2. Mọi tính năng WPS AI cần **tài khoản Kingsoft và mạng tới máy chủ Kingsoft** (chuỗi "Subscription expired", "AI capabilities
   are not available in your current region", "Limited-Time Free", "Unlock to use…"). Axiom chạy với máy chủ AI do người dùng
   hoặc công ty chọn (kể cả máy chủ nội bộ).
3. Mẫu dàn trang của WPS AI (`kwpsaitypeset`) là mẫu Trung Quốc: "National standard GB7713", "Party and government
   administrative documents", luận văn các trường. Văn bản hành chính Việt Nam theo **Nghị định 30/2020/NĐ-CP** đã có skill
   riêng ở Axiom (`skills/van-ban-hanh-chinh/SKILL.md`, `skills/the-thuc-van-ban/SKILL.md`), nhưng thiếu lệnh WPS để áp style.

## 1. Bảng khoảng cách

Phân loại: **Đã có** · **Gần đủ** (thiếu skill hoặc prompt) · **Thiếu lệnh bridge** · **Ngoài phạm vi** · *spike*.

| # | Tính năng WPS AI (chuỗi giao diện tiêu biểu) | Axiom trên WPS Linux hôm nay (bằng chứng) | Phân loại | Ưu tiên |
|---|---|---|---|---|
| W-A | `kwpsaigc`: "Generate full text based on an outline", "Rewrite", giọng "Make the tone more formal / Express in concise language / Increase expression of positivity / Use a more relaxed language style", "Expand", "Simplify", "Correct typos and grammatical errors"; "WPS AI directory recognition" (nhận diện tiêu đề làm mục lục) | Menu chuột phải Writer *Sửa bằng AI*: Trau chuốt, Viết lại, Rút gọn, Mở rộng, Sửa lỗi chính tả, Trang trọng hơn (`quickactions.go:35-45`, `ribbon.xml:121-150`). Viết từ dàn ý: agent viết bằng `writer.heading`/`writer.insertStyledText` (`commands.js:1850-1851`); tool `doc_from_brief` **bị ẩn** trên WPS (thiếu `writer.insertLayoutTable`, `writer.writeContent` — bảng §6 của `wps-bridge.md`) | **Gần đủ** — chỉ có một giọng "trang trọng", thiếu "ngắn gọn / dễ hiểu / tích cực"; nhận diện tiêu đề → xem W-D | **P1** (phần giọng văn) |
| W-B | `kaiaccompanywrite`: "AI generates follow-up writing suggestions. Press Tab to accept", nút nổi theo con trỏ | Không có. Add-in JS không vẽ được chữ mờ ngay trong trang và không bắt phím Tab của trang soạn thảo | Gợi ý **tại chỗ khi gõ**: **Ngoài phạm vi** (*spike* nếu muốn: sự kiện thay đổi tài liệu của WPS JS). **"Viết tiếp" theo yêu cầu**: **Gần đủ** (một lệnh nhanh) | Viết tiếp: **P1** (kèm W-A); tại chỗ: P3 |
| W-C | `kwpsaiwritingsuggest`: "Check Whole Document", "AI Spell Check", "Refine", gạch chân lỗi, "Explain the Reasons for Suggested Rewrites" | Lệnh nhanh *Sửa lỗi chính tả* chỉ cho đoạn **bôi đen** (`quickactions.go:43`). Không soát cả tài liệu; không có cách hiện "đề xuất" để người dùng duyệt từng chỗ: `writer.addComment` (`test_addin.mjs:286`), `writer.setTracking` (`:284`), `writer.replaceMany` (`:308`) **chưa có trên WPS** dù API đã được chứng minh (spike 28/10) | **Thiếu lệnh bridge** (comment, Track Changes, sửa hàng loạt) | **P1** — WPS AI không làm tiếng Việt |
| W-D | `kwpsaitypeset`: "One-click formatting for the entire Document", mẫu "Academic Paper / Bidding document / Contract / Party and government administrative documents", nhận diện "Author Name / Abstract / Appendix Title / References Title" | Skill thể thức NĐ 30 và trình bày (`skills/van-ban-hanh-chinh/SKILL.md`, `skills/the-thuc-van-ban/SKILL.md`, `apps: [wps]` dòng 7); đọc cấu trúc bằng `writer.outline` (`commands.js:1822`); `writer.heading`, `writer.formatSelection`, `writer.setParagraphAlignment` có trên WPS. **Thiếu:** `writer.setStyle` (`test_addin.mjs:314`), `writer.insertToc` (`:315`), `writer.restyle`/`writer.insertBlocks` (`:311-312`) → tool `doc_design` bị ẩn | **Thiếu lệnh bridge** + *spike* (tên style dựng sẵn trên WPS) | **P1** (`setStyle` + `insertToc`); `doc_design` đầy đủ: P2 |
| W-E | `kwpsaitablestyle`: "Enhance your table in one click. Smartly adjust width, align content, unify fonts", "Repeat Header Rows", "Three-Line Table", "Fit Table to Margins", "Smart Table Generator" | `writer.insertTable` (`commands.js:4029`), `writer.checkTables` + nút *Soát bảng* (`commands.js:4031`, `ribbon.xml`). **Thiếu:** `writer.formatTable` (`test_addin.mjs:305`; catalog `live-commands.json:373`) | **Thiếu lệnh bridge** + *spike* (API bảng Writer) | **P1** |
| W-F | `kwpsaifindreplace`: "Fuzzy Match", "AI-suggested similar words", "Try describing what you're looking for", "View results from text in pictures" | Model đọc `writer.getText`/`writer.outline`, trả trích dẫn `[§n]` bấm được → `writer.goto` (`commands.js:4063`); thay bằng `writer.replaceAll` (`commands.js:4032`) | Tìm theo nghĩa: **Gần đủ** (chỉ cần câu dẫn); chữ trong ảnh: **Ngoài phạm vi** (cần OCR) | P2 / P3 |
| W-G | `kwpsaidoc2ppt`: "Generate PPT after confirming the outline", "Loading PPT outline" | Tool `other_document` đọc tài liệu đang mở khác (`tools/other_document.go:47`, `orchestrator_execution.go:514`) — trên WPS mỗi app là một phiên riêng từ 31/10 nên về nguyên tắc dùng được (*spike*). Tạo slide: `wpp.addSlide`/`addText`/`addTextBox`/`addTable` có trên WPS (`commands.js:4071-4075`); `slides_from_brief` **bị ẩn** (thiếu `wpp.addParagraphs`, `wpp.addShape`… `test_addin.mjs:340-347`) | **Thiếu lệnh bridge** (trình chiếu) + *spike* xuyên app | P2 |
| E-A | `ketaiformula`: "AI Formula", "Formula Explanation", "Argument Explanation", ví dụ "Extract the date of birth from the ID number in A2", "Convert the lowercase amount in A2 to uppercase", "Count unique values" | Ghi công thức: `et.fillRange`/`et.writeRange` (`commands.js:4051`); đọc công thức: `et.readRange {formulas:true}` (catalog `live-commands.json:1109-1130`; handler WPS đọc tham số ở `commands.js:2396`); truy vết gốc lỗi: tool `trace_formula` (`tools/trace_formula.go:18`, có trên WPS). Không có skill hướng dẫn mẫu công thức Việt Nam (CCCD, tiền bằng chữ, ngày công, thâm niên) | **Gần đủ** (skill) + *spike* nhỏ (hàm ET hỗ trợ: `DATEDIF`, `TEXTJOIN`, `XLOOKUP`, `NETWORKDAYS.INTL`) | **P1** |
| E-B | `ketaiconditionalformat`: "AI Conditional Formatting", "Please ask questions related to conditional formatting" | `et.setConditionalFormat` (`commands.js:4053`, catalog `:1415`), `et.listConditionalFormats` (`commands.js:4054`) có trên WPS | **Đã có** | — |
| E-C | `ketaidataanalysis` + `ketaitoolbox` (trang web, không có `.qm`): phân tích, phân loại, làm bảng, dự báo, biểu đồ | `dashboard_from_data` (`tools/dashboard.go:22`, có trên WPS), `et.addChart` (`commands.js:4055`), `et.checkRange` + *Soát vùng dữ liệu* (`commands.js:4062`), `et.sort`/`et.filter`; skill `bao-cao-du-lieu`, `mo-hinh-nhieu-sheet` (`apps: [et]`). **Thiếu:** `et.pivot` (`test_addin.mjs:282`), `et.createTable` (`:323`), `et.setValidation` (`:325`) | **Gần đủ** — thiếu pivot / bảng có tên / ràng buộc dữ liệu trên WPS | P2 |
| P-A | `kaiwpp`: "AI Slide Layout", "Convert to Card", "AI Generate Colors", "Change Card Layout", "Converted to a hierarchy / step-by-step diagram", làm đẹp toàn bài, đổi font cả bài | Skill `trinh-bay-chuyen-nghiep`, `bao-cao-thang` (`apps: [wpp]`); `wpp.checkLayout` (`commands.js:4082`). `slides_from_brief`, `slides_from_template` **bị ẩn** trên WPS (thiếu `wpp.addShape`, `wpp.addParagraphs`, `wpp.addGrid`, `wpp.addChart`, `wpp.addConnector`) | **Thiếu lệnh bridge** (nhiều lệnh) + *spike* (Shapes/TextFrame của WPP) | P2 |
| X-A | `kpubaigcbox`: thanh AI khi bôi đen ("Translate", "Explain", "Condense", "Polish"), "Parallel Translate", "Glossaries" (bảng thuật ngữ theo ngành) | Menu chuột phải Writer: Tóm tắt, Giải thích, Dịch (`quickactions.go:47-51`) — **chỉ Writer** (`pane-core.js:22`). Không có dịch song ngữ, không có bảng thuật ngữ. Kho skill nay sửa/nhập được trong pane (31/10) → bảng thuật ngữ có thể là tệp tham chiếu của một skill | **Gần đủ** (lệnh nhanh + skill) | **P1** (dịch song ngữ + thuật ngữ, kèm W-A) |
| X-B | `kwebaipdfinsight`, `kpdfaireference` ("AI Cross-Reference"), `kpdfaisearch`: hỏi đáp, tham chiếu chéo, tìm trong PDF | Add-in khai `type="wps,et,wpp"`; không có bridge cho trình đọc PDF của WPS; làn file MCP không đọc PDF | **Ngoài phạm vi** (*spike*: WPS PDF có nạp JS add-in không) | P3 |
| X-C | `kwebsuwellaidocument`: so sánh, trích xuất, kiểm chứng thông tin văn bản | `other_document` (đọc tài liệu đang mở khác), `writer.getText`; tool web (`tools/websearch.go:37`) khi bật Tavily; làn file MCP `doc_get_text` cho tệp đóng | **Gần đủ** (skill so sánh/trích xuất) + *spike* `other_document` trên WPS | P2 |
| X-D | `kwebaigenerateimage`, `kwebaireport`: tạo ảnh, viết báo cáo | Báo cáo: skill `bao-cao-thang`, tool web; `doc_from_brief` bị ẩn trên WPS. Tạo ảnh: không có (Axiom không gọi mô hình tạo ảnh) | Báo cáo: **Gần đủ**; tạo ảnh: **Ngoài phạm vi** | P2 / P3 |

Không đề xuất sao chép mọi thứ: E-B đã có; W-B dạng gợi ý tại chỗ, OCR ảnh (W-F), PDF (X-B), tạo ảnh (X-D) nằm ngoài
những gì add-in JS + Core làm được hôm nay.

### Cách xếp ưu tiên

Giá trị với người dùng văn phòng Việt Nam × chi phí × rủi ro:

| Mục | Giá trị VN | Chi phí | Rủi ro | Kết luận |
|---|---|---|---|---|
| W-C soát chính tả cả tài liệu, người dùng duyệt từng chỗ | Rất cao (WPS AI không làm tiếng Việt) | Vừa: 3 handler WPS đã có API | Thấp (API đã đo) | **P1** |
| W-D thể thức NĐ 30 / dàn trang theo style | Cao (văn bản hành chính) | Vừa: 2 handler + spike tên style | Vừa | **P1** |
| W-E làm đẹp bảng | Cao (bảng dán từ nơi khác là lỗi hay gặp) | Thấp–vừa: 1 handler + spike | Vừa | **P1** |
| W-A + W-B + X-A giọng văn, viết tiếp, dịch song ngữ, thuật ngữ | Cao | Thấp (lệnh nhanh + skill, không lệnh mới) | Thấp | **P1** |
| E-A công thức bằng lời + giải thích, mẫu Việt Nam | Cao (CCCD, tiền bằng chữ, ngày công) | Thấp (skill) | Thấp | **P1** |
| W-G, P-A, E-C, X-C | Vừa–cao | Cao (nhiều lệnh trình chiếu, pivot, xuyên app) | Vừa–cao | P2 |
| W-B tại chỗ, W-F OCR, X-B, X-D ảnh | Vừa | Rất cao hoặc không làm được | Cao | P3 |

## 2. P1 — đề xuất (đã duyệt 31/10/2026)

### P1-1 (W-C). Soát chính tả, ngữ pháp tiếng Việt cả tài liệu; người dùng duyệt từng chỗ

**Hành vi mục tiêu.** Nút *Soát chính tả* (nhóm *Kiểm tra tài liệu* trên ribbon Writer) hoặc câu "soát chính tả cả bài": AI
đọc cả tài liệu theo khối, sửa lỗi chính tả, dấu thanh, dấu câu, lặp từ dưới dạng **Track Changes** (người dùng *Chấp
nhận/Từ chối* bằng chính công cụ Review của WPS); chỗ cần cân nhắc (diễn đạt, thuật ngữ) để **comment** kèm lý do, không sửa.

**Lệnh bridge mới trên WPS** (catalog đã có, chỉ thiếu handler):
- `writer.setTracking {on}` → `Document.TrackRevisions` (+ `Application.UserName` = "Axiom AI" trong lượt, trả lại cuối
  lượt — giống bản Windows `tracking.go`). Đã đo có (28/10).
- `writer.addComment {paragraph, text, anchor?}` → `Document.Comments.Add(range, text)`. Đã đo `Comments.Add` là function;
  *spike nhỏ*: neo vào một đoạn chữ con (`Range.Find`) và tác giả comment.
- `writer.replaceMany {replacements:[{find, replace}]}` → lặp `Range.Find.Execute` trong **một** `UndoRecord`; bỏ khỏi
  `CHUA_CO_HANDLER`. Giảm số vòng (quy tắc `BatchEditRule` đã nhắc model dùng lệnh này).
- (kèm) `writer.listComments`, `writer.listRevisions` — để lượt sau đọc lại được.

**Core.** Không tool mới. Lệnh nhanh mới `proofread` ("Soát chính tả cả tài liệu", `Edits: true`, **không** đòi bôi đen) trong
`agent/quickactions.go` với câu lệnh: đọc theo khối `writer.outline` → `writer.getText`, bật tracking, sửa bằng
`replaceMany`, comment cho chỗ không chắc, cuối lượt báo số chỗ sửa / số comment. Lượt có `options.trackChanges=true`
(đã có ở Core, `orchestrator.go` `trackChanges`). Không đụng `needs.go` (không phải tool lớn).

**Skill.** Thêm `skills/chinh-ta-tieng-viet/` (`apps: [wps]`): lỗi hay gặp (dấu hỏi/ngã, `d/gi/r`, `s/x`, `ch/tr`, dấu thanh
đặt sai vị trí kiểu cũ/mới — chọn một kiểu và giữ nhất quán với tài liệu), không sửa tên riêng/thuật ngữ, không "trau chuốt"
khi được yêu cầu soát chính tả.

**Pane / ribbon.** Nút `axiomProofread` trong nhóm *Kiểm tra tài liệu* (`ribbon.xml`) → mở khung chat và chạy lệnh nhanh.

**Kiểm thử.** Node: handler `setTracking`/`addComment`/`replaceMany` trên `addin_fake.mjs` (một `UndoRecord`, lỗi trả
`{ok:false}`); `test_addin.mjs` bỏ 3–5 mục khỏi `CHUA_CO_HANDLER`. Go: `quickactions_test.go` (mục mới, không đòi bôi đen).
`make test-catalog`. **WPS thật:** bật tracking → `replaceMany` → `Revisions.Count` tăng, hiện trong Review; comment neo đúng
chỗ; *Hoàn tác lượt này* gỡ đúng.

**Rủi ro.** Chất lượng soát tiếng Việt phụ thuộc model; tài liệu dài phải chia khối (giới hạn đọc của `writer.getText`).

### P1-2 (W-D). Dàn trang theo thể thức: style đoạn và mục lục trên WPS

**Hành vi mục tiêu.** "Trình bày lại tờ trình này theo NĐ 30" / "đánh lại cấp tiêu đề rồi chèn mục lục": AI nhận diện tiêu
đề, phụ lục, chữ ký… (đọc `writer.outline`), gán **style dựng sẵn** (Heading 1–3, Normal, Title) thay vì tô định dạng từng
đoạn, rồi chèn mục lục.

**Lệnh bridge mới trên WPS:**
- `writer.setStyle {paragraphs:[n…] | from,to, style}` → `Paragraphs(n).Style = …`. *Spike bắt buộc:* WPS nhận tên style
  tiếng Anh ("Heading 1"), tên bản địa hoá, hay hằng số `wdStyleHeading1 = -2`; style của tài liệu `.docx` tạo bằng Word.
- `writer.insertToc {levels?}` → `Document.TablesOfContents.Add(range, …)`. *Spike:* có collection này trên WPS không.

**Core.** `doc_design` vẫn ẩn (cần `writer.restyle`, `writer.insertBlocks` — để P2). Cập nhật câu dẫn trong
`skills/the-thuc-van-ban` / `skills/van-ban-hanh-chinh`: "đổi cấp bằng `writer.setStyle`, không tô tay". `needs.go`:
`doc_from_brief` có `writer.insertToc` là **Optional** — không đổi.

**Kiểm thử.** Node: handler trên fake (gán style, đoạn ngoài phạm vi → lỗi rõ). Live WPS: tệp mẫu có 3 cấp tiêu đề →
`setStyle` → `writer.outline` đọc lại đúng cấp; `insertToc` tạo mục lục cập nhật được.

**Rủi ro.** Tên style khác nhau giữa WPS và Word → đo trước; nếu chỉ hằng số dùng được thì bridge dịch tên → hằng số.

### P1-3 (W-E). Làm đẹp bảng một lệnh

**Hành vi mục tiêu.** Nút *Làm đẹp bảng* (cạnh *Soát bảng*) hoặc câu lệnh: bảng vừa lề trang, độ rộng cột theo nội dung,
hàng tiêu đề đậm + **lặp lại khi sang trang**, thống nhất font/cỡ chữ, căn số sang phải; tuỳ chọn "bảng ba đường".

**Lệnh bridge mới trên WPS:** `writer.formatTable {table, style?, autofit?: "content"|"window", headerRepeat?, headerBold?,
font?, size?, align?}` (catalog đã có ở `live-commands.json:373`, tham số theo bản Windows). *Spike:* `Tables(n).Style`,
`AutoFitBehavior`, `Rows(1).HeadingFormat`, `Range.Font` trên Writer WPS; tham số WPS bỏ qua thì trả về `skipped` (như bản
LibreOffice đã làm).

**Core.** Lệnh nhanh `tableTidy` ("Làm đẹp bảng", chạy trên bảng chứa con trỏ/vùng chọn). Dùng kết quả `writer.checkTables`
(đã có) làm bằng chứng trước/sau.

**Kiểm thử.** Node fake + `test_addin.mjs`; live WPS: bảng dán lệch → `formatTable` → `checkTables` không còn lỗi, xuất PDF
hai trang thấy hàng tiêu đề lặp.

### P1-4 (W-A + W-B + X-A). Giọng văn, viết tiếp, dịch song ngữ có thuật ngữ — chỉ lệnh nhanh và skill

**Hành vi mục tiêu.** Menu chuột phải *Sửa bằng AI* thêm: **Ngắn gọn hơn**, **Dễ hiểu hơn**, **Tích cực hơn**, **Viết tiếp**
(viết tiếp sau đoạn bôi đen theo mạch văn), **Dịch song ngữ** (chèn bản dịch ngay dưới từng đoạn, giữ bản gốc). Dịch dùng
**bảng thuật ngữ** của người dùng nếu có.

**Core.** Thêm mục vào `agent/quickactions.go` (câu lệnh + `Edits`); `ribbon.xml` thêm mục cho cả `_t` và `_l`. Không lệnh
bridge mới: dùng `writer.insertStyledText`/`writer.replaceAll` sẵn có.

**Skill.** `skills/dich-thuat/` (`apps` để trống — mọi app): cách dịch hành chính/kỹ thuật, quy tắc giữ số liệu và tên riêng,
**`references/thuat-ngu.md`** (bảng Việt–Anh mẫu); người dùng sửa bảng này ngay trong màn hình *Kỹ năng* (tính năng sửa skill
31/10) hoặc nhập bảng của ngành mình.

**Kiểm thử.** Go `quickactions_test.go` (mục mới, nhãn, `Edits`); `tests/wps/test_pane.mjs` (id `axiomQuick_<mục>_<t|l>`
khớp danh sách Core); live WPS: bôi đen → *Dịch song ngữ* → đoạn dịch nằm ngay dưới, Ctrl+Z gỡ một bước.

**Rủi ro.** Menu chuột phải dài thêm; cân nhắc gom giọng văn vào một menu con *Đổi giọng ▸*.

### P1-5 (E-A). Công thức bằng lời và giải thích công thức — skill

**Hành vi mục tiêu.** "Tách ngày sinh từ số CCCD ở cột B", "đọc số tiền ở C2 thành chữ", "tính thâm niên đến hôm nay",
"giải thích công thức ở F10": AI viết công thức đúng phiên bản ET (ưu tiên hàm có trên cả Excel và WPS), ghi bằng
`et.fillRange` cho cả cột, đọc lại kết quả; giải thích bằng `et.readRange {formulas:true}` + `trace_formula`.

**Skill.** `skills/cong-thuc-bang-tinh/` (`apps: [et]`): mẫu Việt Nam — CCCD 12 số (mã tỉnh 3 số, chữ số giới tính/thế kỷ,
năm sinh 2 số), đọc số tiền thành chữ tiếng Việt (không có hàm sẵn: dùng bảng phụ hoặc ghi giá trị, nói rõ hạn chế), ngày
công (`NETWORKDAYS.INTL` với thứ Bảy), thâm niên (`DATEDIF`), đếm không trùng, tra cứu (`INDEX/MATCH` khi không có
`XLOOKUP`). *Spike nhỏ:* ET 12.1.2 có `DATEDIF`, `TEXTJOIN`, `XLOOKUP`, `NETWORKDAYS.INTL`, `UNIQUE` không (ghi một ô, đọc
lại giá trị/lỗi).

**Kiểm thử.** `internal/tools/repo_skills_test.go` (skill nạp được, đúng `apps`); live WPS: mỗi mẫu công thức chạy trên dữ
liệu giả, đọc lại kết quả đúng.

## 3. P2 — giá trị lớn, chi phí hoặc rủi ro cao

- **P-A + W-G.** Lệnh trình chiếu cho `slides_from_brief` trên WPS: `wpp.addParagraphs`, `wpp.addShape`, `wpp.addGrid`,
  `wpp.addChart`, `wpp.addConnector` (đúng các mục `test_addin.mjs:340-347`); khi đủ `Required` trong `needs.go`
  (`addSlide`, `addParagraphs`, `addShape`, `checkLayout`) tool tự hiện. Rồi "chuyển văn bản thành bài trình chiếu" =
  `other_document` (đọc Writer) + `slides_from_brief` (*spike* xuyên app trên WPS — mỗi app một phiên từ 31/10).
- **E-C.** `et.pivot` (đã đo `PivotCaches()` có), `et.createTable`, `et.setValidation` trên WPS.
- **W-D đầy đủ.** `writer.restyle` + `writer.insertBlocks` → `doc_design` hiện trên WPS.
- **X-C.** Skill so sánh/trích xuất văn bản dùng `other_document` + làn file MCP.
- **W-F.** Câu dẫn "tìm theo nghĩa → trả trích dẫn `[§n]`" trong prompt Writer.

## 4. P3 / ngoài phạm vi

- **W-B gợi ý tại chỗ khi gõ (Tab để nhận):** add-in JS không vẽ chữ mờ trong trang soạn thảo; cần sự kiện gõ phím và chèn
  tạm — *spike* nếu thật sự cần, nhiều khả năng không đạt độ mượt như bản gốc.
- **W-F chữ trong ảnh, X-B PDF, X-D tạo ảnh:** cần OCR / trình đọc PDF / mô hình tạo ảnh — ngoài kiến trúc hiện tại.

## 5. Câu hỏi cần quyết

1. **Duyệt danh sách P1** (P1-1 … P1-5) và thứ tự làm. Đề xuất: P1-4 và P1-5 trước (không cần spike, ra giá trị ngay), rồi
   P1-1, P1-3, P1-2 (mỗi mục mở đầu bằng spike API trên WPS thật).
2. **P1-1:** kết quả soát hiện bằng **Track Changes + comment** (người dùng duyệt trong Review của WPS) hay **danh sách đề
   xuất trong pane** (bấm từng mục để áp)? Đề xuất Track Changes vì dùng công cụ người dùng đã quen và hoàn tác được.
3. **Ribbon:** thêm nút *Soát chính tả*, *Làm đẹp bảng* vào nhóm *Kiểm tra tài liệu*, hay chỉ để trong menu chuột phải và
   khung chat?
4. **P1-4:** gom giọng văn vào menu con *Đổi giọng ▸* (gọn) hay để phẳng như hiện nay (bấm ít hơn)?
5. **Model dùng để đo chất lượng** soát chính tả tiếng Việt (P1-1): model trên máy chủ nội bộ đang dùng, hay model khác?

## 6. Trạng thái

| Mục | Trạng thái |
|---|---|
| Giai đoạn 1 (tài liệu này) | Xong 31/10/2026; P1 đã duyệt, làm P1-4 trước |
| P1-4 | **Xong trên nhánh, chờ merge** (31/10/2026): phần Claude (câu lệnh 5 mục mới + `Group: "tone"` trong `quickactions.go`, skill `dich-thuat` + bảng thuật ngữ, test) ở `spec/p1-4-giong-van`; phần opencode (`ribbon.xml`: menu con *Đổi giọng*, Viết tiếp, Dịch song ngữ) ở `aidev/TASK-000142` qua aidev. Đã thử trên WPS thật: menu đủ 14 mục, *Viết tiếp* chèn đúng chỗ trong vùng khoá, *Hoàn tác* gỡ một bước. **Chưa thử với model thật** (máy chủ AI nội bộ tắt lúc đo) |
| P1-1 | **Đang làm** (31/10/2026). Phần opencode — handler add-in WPS — qua aidev, test đỏ viết trước ở `spec/p1-1-review-wps` (fake `addin_fake.mjs` theo số đo WPS thật, `tests/wps/test_addin_review.mjs`): `writer.setTracking` + `writer.listRevisions` (`aidev/TASK-000143`, đã review; `spec/p1-1-t2` gỡ `data/research.db` lỡ commit), `writer.addComment` + `writer.listComments` (`aidev/TASK-000144`, đã review), `writer.replaceMany` (TASK-000145, đang chạy). Chưa làm: phần Claude (tool soát cả tài liệu, skill chính tả tiếng Việt, nút/lệnh nhanh, bộ câu đánh giá) và kiểm cả chuỗi trên WPS thật |
| P1-2, P1-3, P1-5 | Chưa làm |

## 7. Phân công phát triển: Claude và opencode

Nguyên tắc (31/10/2026): việc **cần suy luận** — spike API trên WPS thật, thiết kế tool và hợp đồng, prompt/skill tiếng Việt,
đánh giá chất lượng, review — do **Claude** làm. Việc **chân tay theo đặc tả đã rõ** — handler theo mẫu, nối ribbon/catalog,
chạy generator, cập nhật tài liệu — do **opencode** làm.

### 7.1 Phần nào của tính năng cần model suy luận khi chạy

Cùng một tính năng thường có phần cần model và phần Core làm tất định được; phần tất định nên nằm trong Core (test được,
không tốn token, model yếu cũng chạy đúng) — như `slides_from_brief` đã làm.

| P1 | Model làm (lúc chạy) | Core / bridge làm tất định |
|---|---|---|
| P1-1 soát chính tả | Trả danh sách `{find, replace, reason}` cho **từng khối** Core đưa vào; chỗ phụ thuộc ngữ cảnh (hỏi/ngã, `d/gi/r`, `s/x`) là phần cần suy luận | Chia khối, bật Track Changes, `replaceMany`, gắn comment, tổng kết — nên là **tool lớn** `proofread_document`, không để agent tự lặp đọc–sửa |
| P1-2 dàn trang | Gắn **nhãn vai trò** cho từng đoạn (quốc hiệu, trích yếu, tiêu đề cấp n, nơi nhận, chữ ký…) | Ánh xạ nhãn → style NĐ 30 (số cố định), áp style, chèn mục lục — tool lớn `apply_document_format` |
| P1-3 làm đẹp bảng | Không cần | Toàn bộ; nút gọi thẳng `/wps/v1/exec` như *Soát bảng* (không qua agent, chạy được cả khi chưa cấu hình máy chủ AI) |
| P1-4 giọng văn, dịch | Biến đổi ngôn ngữ (model nhẹ được) | Chèn đúng chỗ; tra và kiểm bảng thuật ngữ sau khi dịch |
| P1-5 công thức | Hiểu câu hỏi → hàm, vùng, kiểu dữ liệu | Ghi cả cột, đọc lại, truy vết (`trace_formula`) |

### 7.2 Việc phát triển theo người làm

| P1 | Claude | opencode |
|---|---|---|
| P1-1 | Spike trên WPS thật: neo comment vào đoạn chữ con, tác giả comment/revision, `Find.Execute` trong một `UndoRecord` khi đang Track Changes · thiết kế tool `proofread_document` (chia khối, schema kết quả, câu lệnh) · nội dung skill `chinh-ta-tieng-viet` · **bộ bài đo** đoạn văn có lỗi gài sẵn + chọn model · viết test đỏ cho handler và tool | Handler WPS `writer.setTracking`, `writer.addComment`, `writer.replaceMany`, `writer.listComments`, `writer.listRevisions` theo đặc tả + test fake · bỏ khỏi `CHUA_CO_HANDLER` · khai `needs.go` theo đặc tả · nút ribbon `axiomProofread` · `generate_wps_gaps.py` · CHANGELOG |
| P1-2 | Spike tên style dựng sẵn trên WPS (tên tiếng Anh / bản địa hoá / hằng số) và `TablesOfContents` · bảng nhãn vai trò + ánh xạ NĐ 30 (đối chiếu văn bản Nghị định) · câu lệnh gắn nhãn · test đỏ | Handler `writer.setStyle`, `writer.insertToc` · bảng ánh xạ thành tệp dữ liệu theo đặc tả · tool `apply_document_format` phần tất định theo đặc tả · cập nhật skill `the-thuc-van-ban`/`van-ban-hanh-chinh` theo câu chữ Claude đưa |
| P1-3 | Spike API bảng Writer (`Style`, `AutoFitBehavior`, `HeadingFormat`, `Font`) · viết quy tắc làm đẹp (khi nào vừa lề, nhận cột số để căn phải) thành đặc tả + test đỏ | Handler `writer.formatTable` · nút *Làm đẹp bảng* gọi `/wps/v1/exec` · test pane |
| P1-4 | Câu lệnh các mục mới (Ngắn gọn hơn, Dễ hiểu hơn, Tích cực hơn, Viết tiếp, Dịch song ngữ) · nội dung skill `dich-thuat` + `references/thuat-ngu.md` mẫu | Thêm mục vào `quickactions.go` và `ribbon.xml` (`_t`, `_l`) · test id menu khớp danh sách Core · test Go |
| P1-5 | Spike hàm ET (`DATEDIF`, `TEXTJOIN`, `XLOOKUP`, `NETWORKDAYS.INTL`, `UNIQUE`) · nội dung skill `cong-thuc-bang-tinh` (cấu trúc CCCD, đọc tiền bằng chữ…) | Kiểm skill nạp được (`repo_skills_test.go`) · kịch bản kiểm từng mẫu công thức trên WPS thật theo danh sách Claude đưa |
| Chung | Review diff của opencode · xác nhận trên WPS thật (xem ảnh chụp, đánh giá hành vi) · cập nhật §6 trạng thái | `make lint`, `make check-gen`, chạy lại generator; cập nhật `docs/architecture/wps-bridge.md`, hướng dẫn sử dụng theo nội dung Claude soạn |

### 7.3 Bàn giao cho opencode

Mỗi việc bàn giao là một đặc tả viết sẵn, gồm:

1. **Tệp được sửa** và tệp không được đụng (file sinh như `livecommands_gen.go`, khối `SINH TU DONG` trong `commands.js`).
2. **Test đỏ** do Claude viết trước và đã kiểm là đỏ **đúng lý do** (thiếu tính năng, không phải lỗi dựng test).
3. **Lệnh kiểm** phải xanh: `make test-go`, `make test-wps`, `make test-catalog`, `make lint`, `make check-gen` tuỳ việc.
4. **Ràng buộc**: không commit/push; add-in WPS viết ES5; không sửa tay tài liệu sinh tự động; không nới lỏng test để qua.
5. Hành vi WPS thật đã đo (trích từ spike của Claude) để handler không đoán API.

Claude nhận lại diff, review, rồi chạy thử trên WPS thật trước khi đánh dấu xong ở §6.

## Nguồn

- Chuỗi giao diện WPS AI: `/opt/kingsoft/wps-office/office6/addons/<addon>/mui/en_US/*.qm` (WPS 12.1.2.29182), đọc bằng bộ
  đọc định dạng Qt `.qm` (chỉ đọc). Addon không có `.qm`: mô tả theo tên và nhiệm vụ.
- Spike API WPS JS 28/10/2026: `docs/audits/wps-linux-live-test-2026-10-28-run2.md` §5; phép đo khác: `tests/wps/REFERENCE.md`.
- Khuôn trình bày: `docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md`.
