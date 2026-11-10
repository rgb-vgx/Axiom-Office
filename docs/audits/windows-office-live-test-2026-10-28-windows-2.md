# Báo cáo kiểm thử Windows: Office thật — Track Changes, Pivot, other_document (windows-2)

Ngày test: **28/10/2026** · nhánh **`windows-2`**, base `origin/main` @ `ccb8820` · HEAD mã nguồn khi đóng
báo cáo: `181b988` (5 commit sửa lỗi/test) + commit báo cáo `71c9506` · phạm vi: bridge C# cho Microsoft
Office + Agent Core Go, _kiểm thử trên Microsoft 365 thật trên Windows_ (không phải e2e giả lập).

Mục tiêu phiên: (1) build + cài bản `main` mới (mã C# của đợt Track Changes/Pivot/other_document chưa từng được
biên dịch); (2) chạy 4 suite theo yêu cầu; (3) kiểm tay các lệnh mới; (4) sửa lỗi tối thiểu, mỗi lỗi một commit;
(5) bổ sung test cho 15 lệnh mới để gỡ coverage gate.

---

## 1. Môi trường

| Thành phần | Giá trị |
|---|---|
| OS | Microsoft Windows 11 Home Single Language, build 26200, 64-bit |
| Office | Microsoft 365 (O365HomePremRetail) **16.0.20527.20088, x64** — Word / Excel / PowerPoint cùng build |
| Go | go1.26.10 windows/amd64 |
| Python | 3.15.1 |
| Bridge (Office) | cổng **47831** (Word), **47832** (Excel), **47833** (PowerPoint); Agent Core: **47840** |
| Build tool | `scripts\build.ps1` (csc của VS 18 Build Tools), cài bằng `scripts\install.ps1` |
| Log bridge | `%LOCALAPPDATA%\AxiomOffice\bridge.log` |
| LLM | `ocg/beam-free` @ `https://ai.hnspl.vn/v1`, key `dpapi:` (không ghi key) — `llm-test` OK 2.0 s |

Ghi chú môi trường: `MyDocuments` bị chuyển hướng sang OneDrive, nên thư mục lưu mặc định là
`<Documents>\Axiom` trên OneDrive; Word trả `fullName` dạng URL `https://d.docs.live.net/...` sau khi lưu.
Báo cáo dùng `<Documents>` thay cho đường dẫn thật.

---

## 2. Build & cài đặt

- `scripts\build.ps1 -Kill`: **không lỗi C#** — cả add-in, Host và Core Go biên dịch sạch ngay lần đầu.
  (Không gặp lỗi biên dịch nào để sửa; các lỗi trong mục 4 là lỗi hành vi khi chạy Office thật.)
- `scripts\install.ps1`: cài thành công; add-in nạp trên Word/Excel/PowerPoint.
- Trước mỗi lần build: kill `WINWORD/EXCEL/POWERPNT/et/wpp/wps` nhiều vòng; cuối phiên xác nhận không còn
  tiến trình Office/WPS nào đang chạy.

---

## 3. Kết quả test tổng hợp

| Suite | Kết quả nguyên văn | Ghi chú |
|---|---|---|
| `python -m unittest discover -s tests\catalog -v` | `Ran 3 tests in 0.139s` / `OK` | **3/3 pass** |
| `cd core-go; go test -race ./...` | `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` | **chưa chạy được** — máy không có gcc |
| `CGO_ENABLED=1 go test -race ./...` | `cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%` · tất cả package `[build failed]` | xác nhận nguyên nhân là thiếu compiler C, không phải lỗi mã |
| `cd core-go; go test ./...` (thay thế) | tất cả package `ok` (agent, api, office, tools, …) | chạy không `-race` |
| `python tests\live\test_live_commands.py --ai` (cuối phiên) | **376 passed, 0 failed** | gồm coverage gate + 15 lệnh mới |
| `python tests\live\test_live_commands.py --apps ppt --ai` (sau vá PPT) | **100 passed, 0 failed** | xác nhận flake `wpp.open` đã hết |
| `python tests\live\test_live_commands.py` (không `--ai`, giữa phiên) | **349 passed, 0 failed** | trước thay đổi `app.info` cuối cùng; full `--ai` sau đó phủ lại toàn bộ |
| `python tests\live\test_office_structure.py` | **17 passed, 0 failed** | |

### 3.1. Diễn biến các lần chạy `--ai` (ghi nguyên văn)

1. **Lần đầu: `320 passed, 1 failed`** — fail duy nhất là coverage gate:
   `['et.exportChart', 'et.filter', 'et.goto', 'et.pivot', 'et.sort', 'wpp.goto', 'writer.addComment',
   'writer.editCommentScope', 'writer.goto', 'writer.listComments', 'writer.listRevisions',
   'writer.replyComment', 'writer.resolveComment', 'writer.reviewRevisions', 'writer.setTracking']`
2. **Sau khi thêm test + vá Track Changes/exportChart/other_document: `375 passed, 1 failed`** — fail:
   `FAIL PowerPoint ai.ask thêm slide 'Kết luận'` với `slideCount: 2` (slide được thêm vào
   `from-template.pptx`, không phải `wpp.pptx`).
3. **Sau khi vá PowerPoint `CurrentPresentation`/`app.info`: `376 passed, 0 failed`** (lần chạy cuối).
4. Chạy riêng `--apps ppt --ai`: **100 passed, 0 failed** — xác nhận không còn flake `wpp.open`.

> `go test -race` **chưa chạy** vì thiếu gcc/cgo; không ghi nhận là pass. `go test ./...` (không race) pass
> toàn bộ. Đây là giới hạn môi trường, không phải flake.

---

## 4. Lỗi phát hiện & bản vá (5 commit)

### 4.1. Track Changes: `replaceAll` lặp 200 lần, `getText` lẫn chữ đã xoá, chữ người dùng bị từ chối — `766cbf7`

**Triệu chứng (đo trên Word 16.0.20527):**
- Bật `writer.setTracking {on:true, author:"Axiom AI"}`, gọi
  `writer.replaceAll {"find":"gốc một","replace":"đã sửa một"}` trên câu ngắn → trả `count: 200`; tài liệu
  phình từ 30 lên 2.028 ký tự, chữ `đã sửa một` lặp 200 lần.
- `writer.getText` trả cả bản cũ lẫn bản mới (`Đoạn đã sửa mộtgốc một...`), agent tưởng lệnh chưa thay được.
- Lượt `options.trackChanges=true`, người dùng gõ `X` xen giữa: revision `X` mang tác giả `Axiom AI`, nên
  `POST /v1/runs/{id}/undo` (từ chối revision theo tác giả) **xoá luôn X**.

**Nguyên nhân:**
- Word giữ chữ bị xoá trong `Content.Text` (nằm sau chữ vừa chèn) → `Find` thấy lại đúng chuỗi cũ và vòng lặp
  thay tiếp tới giới hạn 200.
- `UserName` được đặt `Axiom AI` **suốt lượt**, nên mọi phím người dùng gõ cũng thành revision của AI.

**Vá:**
- `ReplaceLong` ẩn revision (`doc.ShowRevisions = false`) trong lúc Find/thay, khôi phục ngay sau; `writer.getText`
  cũng đọc trong trạng thái ẩn revision nếu đang bật tracking hoặc còn revision → trả chữ CUỐI.
- Chỉ đặt `UserName = "Axiom AI"` quanh từng lệnh `writer.*` **có thể tạo revision**
  (`RunWithTrackingAuthor`), trả lại tên người dùng ngay sau lệnh; bỏ qua lệnh chỉ đọc (`getText`, `outline`,
  `listComments`, `listRevisions`, `goto`, save, undo…).

**Kiểm chứng:**
- `writer.replaceAll` cùng câu: `count: 1` (trước là 200); `writer.getText` trả
  `Đoạn đã sửa một. Đoạn gốc hai.` (không còn bản cũ).
- Lượt thật với `options.trackChanges=true`: AI sửa 2 chỗ (4 revision insert/delete, tác giả `Axiom AI`);
  người dùng gõ `X` bằng Word UI (`Selection.TypeText`) giữa lượt → revision `X` mang tài khoản đăng nhập.
- `undo` trả `{"mode":"revisions","result":{"action":"reject","count":4}}`; còn lại đúng 1 revision `X`;
  văn bản cuối `Đoạn gốc một. Đoạn gốc hai. X`.
- Sau lượt: `TrackRevisions=false` và `UserName` trở về giá trị trước lượt (đo lại bằng COM ngoài bridge).

### 4.2. `et.exportChart` ghi PNG 0 byte — `0f65d63`

**Triệu chứng:** `et.exportChart` trả `ok:true`, `path` tồn tại nhưng `FileInfo.Length = 0` (đo trên Excel
16.0.20527 với chart tạo bằng `Shapes.AddChart2`). Test `exportChart ghi PNG thật` fail; nếu không kiểm size
thì lỗi bị che.

**Nguyên nhân:** `Chart.Export` trả `true` nhưng chỉ ghi ảnh khi chart object đang được chọn/activate; chart
tạo qua COM không tự thành chart active.

**Vá:** `ExportChartPng` gọi `chartObject.Activate()` trước `Chart.Export`; sau đó kiểm file tồn tại và
`Length > 0`, nếu rỗng thì xoá và thử lại một lần; vẫn rỗng thì báo lỗi thay vì trả `path` rỗng.

**Kiểm chứng:** cùng chart, file PNG **4.013 byte**; `wpp.addImage {path}` chèn được (`shape: "Picture 4"`).

### 4.3. `wpp.open`/`ai.ask` làm trên bài PowerPoint khác — `159b789`

**Triệu chứng:** `wpp.open` trả `alreadyOpen:true` cho `wpp.pptx`, nhưng `wpp.listSlides` và lượt `ai.ask`
kế tiếp vẫn làm trên `from-template.pptx`; `app.info` báo `activePresentation` là `from-template.pptx`
(`saved:true`) dù bài đích chưa lưu.

**Nguyên nhân:** PowerPoint 16 **không đổi `ActivePresentation`** sau `Window.Activate()` (và cả
`Application.ActiveWindow = window`) khi cửa sổ bài không phải cửa sổ nổi — đo trực tiếp bằng PowerShell COM,
`ActivePresentation` vẫn là bài cũ. Bridge tin `ActivePresentation` ở `wpp.listSlides`, `ai.ask`, `app.info`.

**Vá:** ghi nhớ bài vừa mở/tạo bằng `TrackPresentation`; `TargetPresentation`, `HostProbe.ActiveDocument/View`,
`app.info` và `SaveDocument` dùng `CurrentPresentation` (bài đã ghi nhớ nếu còn mở, fallback
`ActivePresentation`). Khi mở bài đã mở, kích hoạt mọi cửa sổ tài liệu (không chỉ `Windows[1]`) rồi mới trả.

**Kiểm chứng:** sau `wpp.open`, `wpp.listSlides` trả `presentation: "wpp.pptx", slideCount: 1`; chạy
`--apps ppt --ai` **100/100**, gồm `ai.ask thêm slide 'Kết luận'` và `tài liệu vẫn chưa lưu sau ai.ask`.

### 4.4. `other_document` bỏ sót tài liệu vừa mở — `d49a689`

**Triệu chứng:** mở Excel (`ban-hang.xlsx`, đã lưu) và Word cùng lúc; lượt Word gọi `other_document` thì Core
trả `unknown tool 'other_document'`, danh sách tool không có tool xuyên app dù Excel đang mở.

**Nguyên nhân:** `otherSessions` chỉ đọc file session registry; file này được heartbeat làm mới **25 giây/lần**,
nên tài liệu vừa mở/lưu xong vẫn còn `document:null` hoặc tên cũ → Core coi như không có tài liệu khác.

**Vá:** thêm `BridgeClient.LiveSession` gọi `GET /session` (live=true, add-in đọc lại `ActiveDocument` và ghi
file registry); `Orchestrator.otherSessions` hỏi trực tiếp từng bridge trước khi quyết định có tool.

**Kiểm chứng:** agent gọi `other_document` 2 lần (commands + `et.readRange C1:C5`), đọc đúng
`[["Doanh thu"],[100],[50],[200],[25]]`, ghi vào Word `Tổng doanh thu: 375`; audit có `tool: other_document`.
Khi yêu cầu **sửa** Excel, Core phát `confirm.required` (`et.writeRange`, lý do "Sửa tài liệu khác đang mở:
ban-hang.xlsx (Excel, port 47832)", timeout 120 s); duyệt xong A10 = 999.

### 4.5. Coverage gate: 15 lệnh mới chưa có test — `181b988`

**Triệu chứng:** `FAIL mọi lệnh trong registry đều có test` với 15 tên lệnh (mục 3.1 lần 1).

**Nguyên nhân:** gate đối chiếu registry với các lệnh đã gọi trong harness; các lệnh comment/Track Changes/
pivot/sort/filter/goto/exportChart mới chỉ được kiểm tay.

**Vá:** thêm test vào `tests/live/test_live_commands.py` (không xoá/nới gate):
`test_writer_review` (9 lệnh Word), `test_spreadsheet_pivot` (`et.pivot/sort/filter/goto`), `et.exportChart`
trong `test_spreadsheet_more`, `wpp.goto` trong `test_presentation`.

**Kiểm chứng:** lần chạy cuối `376 passed, 0 failed`, gate **PASS**.

---

## 5. Kiểm tay các lệnh mới

Mỗi mục chạy qua bridge thật/Core thật; đối chứng bằng raw JSON hoặc `ConvertTo-Json -Depth 8` (không dùng
`ConvertTo-Json` mặc định Depth=2).

| Lệnh / kịch bản | Kết quả | Bằng chứng chính |
|---|---|---|
| `options.trackChanges=true`, AI sửa 2 chỗ, người dùng gõ `X` giữa lượt | ✅ | 4 revision AI (`Axiom AI`) + 1 revision `X` (tài khoản đăng nhập); `undo` 200 `mode:revisions`, reject 4; X còn nguyên |
| `writer.setTracking {on:true, author:"Axiom AI"}` | ✅ | `{"tracking":true,"previous":false,"author":"Axiom AI"}` |
| `writer.setTracking {on:false, restoreAuthor:true}` | ✅ | `{"tracking":false,"previous":true,"author":"<tài khoản trước lượt>"}` |
| `writer.reviewRevisions {action:"reject", author:"Axiom AI", since}` | ✅ | `{"action":"reject","count":1}`; chữ `Phần AI.` biến mất, còn `Bản gốc.` |
| `writer.addComment` + `replyComment` + `editCommentScope` | ✅ | comment/reply còn; `before:"Mục tiêu ban đầu"`, `after:"Mục tiêu đã rõ"`; `listComments` anchor đúng chữ mới, 1 reply |
| `et.pivot` (Vùng, Sản phẩm, Doanh thu; rows `["Vùng"]`, sum Doanh thu) | ✅ | sheet `Pivot`, table `AxiomPivot`, caption `Tổng Doanh thu`, range `A3:B6`; đọc lại Bắc **150**, Nam **225**, Grand Total **375** |
| `et.exportChart` → file PNG → `wpp.addImage` | ✅ | PNG **4.013 byte**; `wpp.addImage` trả `{"slide":1,"shape":"Picture 4"}` |
| `other_document`: Word đọc tổng từ Excel đang mở | ✅ | audit `other_document` ×2; đọc `C1:C5` = header + `100,50,200,25`; Word ghi `Tổng doanh thu: 375` |
| `other_document`: sửa Excel | ✅ | `confirm.required` hiện trước khi ghi; duyệt → `et.writeRange`; A10 = 999 |
| `writer.save` tài liệu mới trùng tên đã có | ✅ | tạo sẵn `Document1.docx`, lưu tài liệu mới `Document1` → **`Document1 (2).docx`**, không ghi đè |
| `writer.save` hai tài liệu mới khác nhau | ✅ | Word đặt tên `Document2`/`Document3` trong phiên này; lưu lần lượt ra file riêng, không ghi đè |
| `writer.saveAs {path:"trung.docx"}` lần 2 | ✅ lỗi mong đợi | `ArgumentException: file already exists: <Documents>\Axiom\trung.docx - to overwrite it, call again with this full path ...` |
| Đóng tài liệu chưa lưu | ✅ | `DisplayAlerts=-1`; WM_CLOSE hiện `NUIDialog`; UIA đọc: **"Save your changes to this file?"** + `Save / Don't Save / Cancel` |

### 5.1. Tác giả revision trên Microsoft 365

- Revision do AI sửa: **`Axiom AI`** — không phải tên tài khoản Microsoft 365. (Word nhận `UserName` do
  bridge đặt trong lúc chạy lệnh.)
- Revision do người dùng gõ `X` giữa lượt: **tên tài khoản đăng nhập Windows/M365** (khác `Axiom AI`), nên
  `undo` theo tác giả + `since` chỉ từ chối 4 revision của AI, giữ `X`.
- Sau lượt, `UserName` và `TrackRevisions` trở về đúng giá trị trước lượt; `setTracking {restoreAuthor:true}`
  trả lại cả `UserInitials`/`UseLocalUserInfo`.

---

## 6. Flake & giới hạn đã ghi nhận

- **`wpp.open` với bài đã mở** (flake của báo cáo 28/10 trước): tái hiện 2 lần trong phiên này
  (`alreadyOpen:true` nhưng `active:'from-template.pptx'`), đã vá ở `159b789`; sau vá chạy PPT riêng
  **100/100** và full **376/0**.
- **`go test -race`**: không chạy được vì máy không có gcc/cgo; đã chạy `go test ./...` thay thế, tất cả pass.
  Không ghi nhận là pass cho `-race`.
- `et.exportChart` phải `Activate()` chart trước khi Export — đây là hành vi của Excel, đã xử lý trong mã.

---

## 7. Commit trên `windows-2`

| Commit | Nội dung |
|---|---|
| `766cbf7` | `fix(bridge): Track Changes - replaceAll lap 200 lan, getText lan chu da xoa, chu nguoi dung bi tu choi` |
| `0f65d63` | `fix(bridge): et.exportChart ghi PNG 0 byte khi chart chua duoc kich hoat` |
| `159b789` | `fix(bridge): giu bai PowerPoint vua mo/tao lam bai dich (PowerPoint 16 khong doi ActivePresentation)` |
| `d49a689` | `fix(core): other_document bo sot tai lieu vua mo do session registry chi heartbeat 25 giay` |
| `181b988` | `test(live): phu 15 lenh moi de go coverage gate ...` |

Base: `ccb8820` (`origin/main`). Không push lên `main`; push nhánh `windows-2` bằng
`git push -u origin windows-2`.

---

## 8. Việc còn lại / khuyến nghị

1. **Chạy `go test -race ./...`** trên máy có gcc/cgo (MinGW-w64/MSYS2) — hiện chưa chạy được, không thay thế
   bằng `go test` thường khi báo cáo race.
2. **WPS chưa test trong phiên này** (chỉ Microsoft Office thật). Add-in dùng chung nhưng nên chạy một lượt
   `--wps` trước phát hành, nhất là các lệnh Track Changes/Pivot mới.
3. `CurrentPresentation` đang là "bài bridge đang làm"; `app.info.state` vẫn phản ánh `ActivePresentation` thật
   (có thể khác `activePresentation` khi PowerPoint từ chối chuyển cửa sổ). Nếu cần nhất quán, hợp nhất
   `HostProbe.Describe` với `CurrentPresentation`.
4. `et.exportChart` gọi `chartObject.Activate()` — có thể đổi selection của người dùng sang chart; chấp nhận
   được cho lệnh xuất ảnh, nhưng nên ghi chú trong mô tả tool.
5. `writer.getText` ẩn revision cả khi `TrackRevisions=false` nhưng còn revision chưa chấp nhận (trả chữ cuối);
   nếu người dùng muốn đọc bản gốc, dùng `writer.listRevisions`/giao diện Word.

---

## 9. Lệnh tái lập

```powershell
# Build + cài (kill Office/WPS trước)
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1 -Kill
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\install.ps1

# Test
python -m unittest discover -s tests\catalog -v
cd core-go; go test ./...; cd ..
python tests\live\test_live_commands.py --ai
python tests\live\test_office_structure.py

# Kiểm tra LLM
src\AxiomOffice\bin\Release\AxiomOffice.Host.exe llm-test
```
