# 19 / H0 - Nền tảng bền: nhắm đúng tài liệu, bridge không treo, đọc đủ những gì đã ghi

> Các điểm yếu dưới đây lộ ra khi chạy benchmark slide thật ngày **25/10/2026** (`18-s0`), không phải suy đoán. Chúng
> nằm ở ranh giới giữa Core và Office, nên ảnh hưởng **mọi** tính năng chứ không riêng slide. Mỗi mục có bằng chứng,
> phần đã có, việc cần làm, và tiêu chí xong đo được.

Rà soát code ngày 25/10/2026. Trước khi làm một mục, đọc lại mục "Đã có" rồi kiểm tra lại trong code: trạng thái có thể đã đổi.

**Phạm vi (quyết định 25/10/2026):** chỉ làm cho **Microsoft Office** (add-in C#) trên host Windows. **LibreOffice
chính thức dừng hỗ trợ**: không sửa, không đưa vào tiêu chí. Phần **WPS** gom vào mục "To-do WPS (host Linux)" ở
cuối, làm khi chuyển sang host Linux.

## Tổng quan và thứ tự

| # | Mục | Bằng chứng (25/10/2026) | Cỡ việc |
|---|---|---|---|
| H1 | Lệnh nhắm tài liệu tường minh | 50/75 lệnh không có tham số chọn tài liệu; một lượt benchmark đã ghi vào nhầm bài | Lớn (C# + Core) |
| H2 | Bridge không treo vì hộp thoại hay máy ngủ | Một hộp thoại modal chặn cả bộ chạy; máy ngủ làm 21 lượt thất bại | Vừa |
| H3 | Lệnh đọc phủ mọi thứ lệnh ghi tạo ra | `wpp.listSlides` không đọc bảng, agent chèn bảng hai lần | Nhỏ–vừa |
| H4 | Duyệt trước thao tác trên đối tượng do chính lượt tạo | `interactive=false` từ chối mọi lệnh xoá, agent không sửa được lỗi của mình | Vừa |
| H5 | Năng lực từng backend đi vào mô tả tool | Lệnh chỉ báo "không hỗ trợ" lúc gọi | **Hoãn**: chỉ còn một backend (C#); làm cùng WPS |
| H6 | Lệnh mới: primitive ở backend, ghép ở Core | Hai bộ typography (xem `18-s0`) | Quy tắc, áp dần |

Thứ tự làm: **H3 → H2 → H4 → H1** (H5 hoãn).

### Trạng thái (25/10/2026)

| # | Đã làm (code + test tự động) | Còn lại |
|---|---|---|
| H1 | Catalog chuyển nguồn chuẩn sang `catalog/live-commands.json`; 53 lệnh C# có thêm `presentation`/`workbook`/`document` (`CommandDispatcher.Target.cs`: kích hoạt đúng tài liệu, báo `DOCUMENT_CLOSED`); Core tự gắn tài liệu của lượt và đổi theo khi tạo/mở tài liệu (`tools/target.go`), ẩn tham số khỏi mô tả tool; `tests/catalog` | Live test hai tài liệu cùng mở trên Word/Excel/PowerPoint; benchmark chạy song song |
| H2 | `CommandWorker` (lệnh trên thread STA riêng, `HOST_BUSY` sau 120 s, lệnh xếp hàng bị bỏ không chạy muộn); `HostDialogs` báo nội dung hộp thoại; Core dùng lại session quá hạn khi bridge trả đúng `pid` (`FindWoken`); model client phát hiện máy vừa thức (đồng hồ thực nhảy) và thử lại (`model/resume.go`) | Live test hộp thoại modal thật; benchmark có cho máy ngủ giữa chừng |
| H3 | `wpp.listSlides` (C#) đọc bảng `[table RxC]`, nhóm, tiêu đề biểu đồ; bộ chấm có `readGap` (baseline: khác 0 ở `s05`, `s09`, `s13`) | Live test khứ hồi ghi → đọc; đo lại `readGap` sau khi build |
| H4 | Sổ đối tượng của lượt (`tools/owned.go`): xoá slide/shape do lượt tạo không phải hỏi, số slide dịch theo khi xoá; `wpp.addTable` trả tên shape; `slides_from_brief` cũng ghi sổ | Benchmark `interactive=false` không còn slide trống thừa |
| H5 | Hoãn (một backend) | Làm cùng WPS |
| H6 | Quy tắc ghi ở `docs/architecture/core.md` mục 9.1 | — | H3 nhỏ và sửa ngay một lỗi chất lượng đã thấy. H2 và H4 gỡ những chỗ làm
hỏng phép đo. H1 lớn nhất nên làm sau, khi bộ đo đã đáng tin. H6 là quy tắc áp cho mọi lệnh mới, kể cả lệnh của S0.

Mọi mục đo bằng bộ đo slide của `18-s0` (`tests/bench/slides/`) và khung đo của `17-r0`.

---

## H1 - Lệnh nhắm tài liệu tường minh

### Bằng chứng

Đếm trong `catalog/live-commands.json` (25/10/2026) số lệnh **không** có tham số chọn tài liệu (`presentation` /
`workbook` / `document` / `path`):

| Loại | Tổng | Không chọn được tài liệu | Trong đó agent dùng |
|---|---|---|---|
| `wpp.*` | 24 | 7 | `addSlide`, `addTable`, `deleteSlide`, `sourceManifest` |
| `et.*` | 23 | 19 | 17 lệnh, gồm `readRange`, `writeRange`, `formatRange`, `addChart`, `save` |
| `writer.*` | 28 | 24 | 18 lệnh, gồm `getText`, `insertStyledText`, `replaceMany`, `save` |

Các lệnh này làm trên **tài liệu đang active**. Hệ quả:
- Người dùng bấm sang cửa sổ khác giữa lượt chạy thì agent ghi vào nhầm tài liệu.
- Không chạy hai lượt song song trên cùng app được: benchmark slide phải chạy tuần tự, 21 lượt mất khoảng 40 phút.
- `wpp.exportPdf` và `wpp.saveAs` cũng không có tham số chọn bài.
- **Đã xảy ra khi chạy tuần tự** (benchmark 25/10/2026, `s09` lần 1): bài mới tạo không thành bài active, slide của
  lượt bị ghi nối vào bài của lượt trước và file được lưu là bài đó. `checkLayout`/`listSlides` (có chọn bài) lại
  đọc bài đích, nên kết quả đọc và kết quả lưu không khớp. Không cần người dùng bấm gì, chỉ cần cửa sổ
  active chưa chuyển kịp.

### Đã có

- C#: `TargetPresentation(app, p)` (`CommandDispatcher.Template.cs`) nhận `presentation` (đường dẫn đầy đủ hoặc tên),
  mặc định là bài active. 17 lệnh `wpp.*` đã dùng hàm này.
- Word: `EnsureVisibleWordDocument` kích hoạt tài liệu đang hiển thị trước lệnh `writer.*`, nhưng vẫn theo
  "tài liệu người dùng đang thấy" chứ không theo tài liệu của lượt chạy.
- Core biết tài liệu của lượt: `request.Document` (`name`, `fullName`) được gửi từ pane.

### Việc cần làm

1. **Catalog**: mỗi lệnh loại `wpp`/`et`/`wps` nhận tham số chọn tài liệu tuỳ chọn, tên thống nhất theo loại:
   `presentation`, `workbook`, `document`. Thêm test CI quét catalog: lệnh nào thiếu thì fail (ngoại lệ ghi tường
   minh: `*.newPresentation`, `*.open`, `app.*`).
2. **C#**: hàm `TargetWorkbook` và `TargetDocument` theo đúng mẫu `TargetPresentation`; mọi lệnh gọi qua chúng.
   Không đổi hành vi khi không truyền tham số.
3. WPS Linux (`commands.js`): xem "To-do WPS". LibreOffice: không làm (dừng hỗ trợ).
4. **Core tự gắn tài liệu**: khi lượt bắt đầu, Core chốt tài liệu đích (`request.Document.FullName`, nếu không có
   thì tên tài liệu đang active lúc bắt đầu) và **tự thêm** tham số chọn tài liệu vào mọi `office_action` mà model
   không truyền. Model không phải nhớ tham số này: mô tả tool không dài thêm, nên không làm khó model yếu.
5. Tài liệu đích bị đóng giữa lượt → bridge trả lỗi rõ ràng (`DOCUMENT_CLOSED`), không lặng lẽ rơi về tài liệu active.

### Tiêu chí xong

- Test CI quét catalog: 0 lệnh thiếu tham số chọn tài liệu (ngoài danh sách ngoại lệ).
- Live test (`test_live_commands.py`): mở hai bài A và B, B active, chạy chuỗi lệnh với `presentation: A`, thì mọi
  thay đổi nằm trong A và B không đổi. Làm tương tự cho Word và Excel.
- Test Go: `office_action` không có tham số chọn tài liệu thì Core gắn tài liệu đích của lượt.
- Bộ chạy benchmark chạy **2 lượt song song** trên cùng PowerPoint, điểm smoke không khác chạy tuần tự (cùng model,
  cùng số lần chạy, chênh lệch nằm trong dao động giữa các lần).

---

## H2 - Bridge không treo vì hộp thoại hay máy ngủ

### Bằng chứng

- **Hộp thoại modal chặn tất cả.** PowerPoint hiện "An error occurred while PowerPoint was saving the file" (do lưu
  hoặc xuất PDF một bài 0 slide ngay sau khi máy thức dậy). Từ đó mọi lệnh `/cmd`, kể cả `/health`, đều treo cho tới
  khi có người bấm OK. Lý do: `ComGate.Run` là `lock` không có thời hạn, và HTTP bridge không đặt timeout.
- **Máy ngủ (connected standby).** Event Log: máy ngủ lúc 05:40–06:27 và 06:43–08:16.
  - Một lượt đang chờ model bị treo 1 giờ 43 phút: watchdog "300 giây không nhận byte nào" (`model/client.go`)
    dùng đồng hồ đơn điệu, đồng hồ này dừng khi máy ngủ, và kết nối TCP chết không được phát hiện.
  - Heartbeat session (`SessionRegistry.cs`, 25 giây) dừng, nên khi thức dậy Core báo "office session not found"
    với app vẫn đang chạy, và 21/21 lượt thất bại.
- **Crash trong process.** `wpp.closeAll` (thêm thử rồi đã gỡ ngày 25/10/2026) đóng hai bài cùng lúc qua COM làm
  PowerPoint 2019 (16.0.10373) crash `0xc0000409` hai lần. Add-in chạy trong process Office, nên một lệnh COM lỗi
  là mất cả app.

### Đã có

- `ComGate.TryRun(timeoutMs, …)` cho heartbeat và poller: host bận thì bỏ qua lượt đó.
- Core so `pid` của `/health` với `pid` của request, nên báo đúng khi app đã khởi động lại.
- Watchdog idle 300 giây cho lượt agent.

### Việc cần làm

1. **Thời hạn cho lệnh COM**: `/cmd` chờ `ComGate` tối đa N giây (mặc định 60, cấu hình được). Quá hạn thì trả lỗi
   `HOST_BUSY` thay vì treo HTTP. Không huỷ được lời gọi COM đang chạy, nhưng bridge vẫn trả lời và Core vẫn báo được
   cho người dùng.
2. **Phát hiện hộp thoại**: khi `HOST_BUSY`, bridge liệt kê cửa sổ `#32770` hiển thị của process host (tiêu đề và
   chữ, như cách đã làm tay ngày 25/10/2026) và trả về trong lỗi (`dialog: {title, text}`). Core báo người dùng:
   "PowerPoint đang hiện hộp thoại '…' — bấm OK rồi thử lại". **Không tự bấm**: hộp thoại có thể là câu hỏi lưu file
   của người dùng.
3. **Thức dậy sau khi ngủ**:
   - Core: session quá hạn nhưng `/health` trên đúng cổng trả lời với đúng `pid` thì coi session còn sống và dùng
     tiếp, không fail.
   - Model client: phát hiện đồng hồ treo nhảy (khoảng cách thời gian thực giữa hai lần kiểm tra lớn hơn nhiều so với
     chu kỳ kiểm tra) thì huỷ request đang chờ và thử lại một lần, không đợi watchdog.
4. **Quy tắc cho lệnh chạy trong process** (ghi vào `docs/architecture/core.md`): lệnh thay đổi vòng đời tài liệu
   (đóng, mở, đóng hàng loạt) phải có live test trên phiên bản Office thấp nhất được hỗ trợ trước khi vào catalog;
   không đóng nhiều tài liệu trong một lời gọi COM.

### Tiêu chí xong

- Test C# (hoặc live test) có hook giữ `ComGate`: `/cmd` trả `HOST_BUSY` trong N giây ± 1 và `/health` vẫn trả lời.
- Live test: mở một hộp thoại modal trong host (ví dụ `MsgBox` qua macro trong tài liệu test), rồi `/cmd` trả lỗi có
  `dialog.text` đúng nội dung.
- Test Go: session có `lastSeenEpoch` cũ 2 giờ nhưng `/health` đúng `pid` thì lượt vẫn chạy. Đồng hồ giả nhảy 1 giờ
  thì request model bị huỷ và thử lại.
- Bộ chạy benchmark chạy smoke với máy được cho ngủ 5 phút giữa chừng: không lượt nào thất bại vì session hay treo
  quá thời hạn lượt.

---

## H3 - Lệnh đọc phủ mọi thứ lệnh ghi tạo ra

### Bằng chứng

- `wpp.listSlides` (C#) chỉ đọc shape có `TextFrame`. **Chữ trong bảng không được đọc.** Benchmark chấm sai: `s09`
  được đọc lại 0/3 trong khi bảng đúng đủ số liệu; đọc thẳng `.pptx` thì 3/3.
- Agent cũng dùng `wpp.listSlides` để tự soát, nên không thấy bảng của chính mình: `s03` lần 3 có **hai bảng giống
  hệt nhau chồng lên nhau** (`checkLayout` báo `overlap`).
- Nguyên tắc cần giữ: **thứ gì lệnh ghi tạo ra được thì lệnh đọc phải thấy được.**

### Đã có

- `checkLayout` liệt kê mọi shape kèm toạ độ, nhưng không có nội dung.
- Bộ chấm `score.py` đọc chữ từ `.pptx` (`deck_texts`), đây là nguồn đối chiếu.

### Việc cần làm

1. `wpp.listSlides`: đọc bảng (chữ từng ô theo hàng, đánh dấu `[bảng r×c]`), nhóm shape (đệ quy `GroupItems`),
   tiêu đề biểu đồ. Làm ở C#; WPS Linux xem "To-do WPS".
2. Kiểm tra tương tự cho `writer.getText` (bảng, hộp văn bản, header/footer) và `et.readRange` (ô gộp, công thức
   lỗi). Mỗi chỗ thiếu thành một việc riêng, có ca live test.
3. Live test khứ hồi: với mỗi lệnh ghi trong `test_live_commands.py`, đọc lại bằng lệnh đọc mà agent dùng và kiểm tra
   nội dung vừa ghi có mặt.
4. Bộ chấm benchmark thêm chỉ số `readGap`: số cụm `mustContain` có trong `.pptx` mà `wpp.listSlides` không thấy.

### Tiêu chí xong

- `readGap = 0` trên toàn bộ smoke.
- Live test khứ hồi: mọi lệnh ghi `wpp.*` đều đọc lại được.
- Benchmark smoke: không còn lượt nào có hai bảng hoặc hai khối trùng nội dung trên cùng slide.

---

## H4 - Duyệt trước thao tác trên đối tượng do chính lượt tạo

### Bằng chứng

- `interactive=false` (MCP `ai.ask`, agent bên ngoài, benchmark) thì `ConfirmOrDecline` **từ chối mọi lệnh cần xác
  nhận**. `s01` lần thử: agent thêm thừa một slide trống, gọi `wpp.deleteSlide` hai lần đều bị từ chối, và bài nộp có
  một slide trống. `s13`: ba lần `wpp.deleteShapes` bị từ chối, agent chuyển sang ghi đè chữ cho tới khi bị chặn
  lặp lệnh.
- Ở chế độ có người dùng, cũng chính các lệnh này hỏi người dùng về những thứ **agent vừa tự tạo trong lượt**:
  câu hỏi thừa, làm người dùng mỏi tay.

### Đã có

- `policy.Evaluate` (`core-go/internal/policy/policy.go`): `wpp.deleteSlide`, `wpp.deleteShapes`, `wpp.deleteTagged`,
  `wpp.newFromTemplate`, `writer.replaceAll`/`replaceMany` cần xác nhận.
- `overwrite.go` đã có tiền lệ "nhớ trong lượt": ghi đè lên sheet vừa tạo trong lượt thì không hỏi
  (`rememberOverwriteApproval`).
- Lệnh thêm trả về định danh: `wpp.addText` trả `{slide, shape}`.

### Việc cần làm

1. **Sổ đối tượng của lượt**: Core ghi lại mọi slide và shape do lệnh thêm của lượt này tạo ra (từ kết quả bridge).
   Bổ sung định danh vào kết quả ở lệnh thêm nào chưa trả (`wpp.addSlide`, `wpp.addTable`, `wpp.addImage`, …) trong
   bản C#; WPS xem "To-do WPS".
2. **Policy**: xoá hoặc sửa đối tượng **chỉ** nằm trong sổ của lượt thì không cần xác nhận, ở cả hai chế độ. Đối
   tượng có sẵn trong tài liệu thì giữ nguyên quy tắc hiện tại.
3. Ghi vào audit mỗi lần được miễn xác nhận theo sổ, để soát lại được.

### Tiêu chí xong

- Test Go: lượt thêm slide 3 rồi xoá slide 3 thì không hỏi; xoá slide 1 (có sẵn) thì vẫn hỏi. Chỉ số slide dịch đi
  sau một lần xoá thì sổ phải cập nhật theo (test riêng).
- Benchmark smoke với `interactive=false`: không còn lượt nào nộp kèm slide trống do agent tự tạo.
- Bộ chạy benchmark bỏ được phần tự bấm đồng ý (`run.py`, sự kiện `confirm.required`) mà điểm không giảm.

---

## H5 - Năng lực từng backend đi vào mô tả tool (hoãn)

Với phạm vi mới, chỉ còn một backend được hỗ trợ (C#, Microsoft Office), nên chưa có gì để lọc. Mục này chuyển vào
"To-do WPS (host Linux)" bên dưới: làm khi WPS Linux thành backend thứ hai.

Bằng chứng gốc (giữ để tham khảo): LibreOffice trả `_template_unsupported` cho 9 lệnh `wpp.*` **lúc gọi**, nên agent
chỉ biết sau khi đã lên kế hoạch và gọi xong.

---

## H6 - Lệnh mới: primitive ở backend, ghép ở Core

Không phải một việc riêng mà là **quy tắc** cho mọi lệnh thêm sau ngày 25/10/2026, ghi vào
`docs/architecture/core.md`:

- Backend (C#; sau này JS của WPS) chỉ làm **primitive**: tạo hoặc sửa một đối tượng, đọc lại đúng thứ đã tạo. Không có bố
  cục, không có ngưỡng thiết kế, không có hằng số typography.
- Bố cục, fallback, quy tắc thiết kế (cỡ chữ, lưới, màu) nằm ở **Core (Go)**, viết một lần và đọc từ
  `_design/tokens.json`. `18-s0` áp quy tắc này cho slide.
- Lệnh mới phải có cùng lúc: mục catalog, trường `backends` (H5), tham số chọn tài liệu (H1), cách đọc lại (H3), và
  live test trên Office thấp nhất được hỗ trợ (H2.4).

Tiêu chí: checklist PR trong `docs/architecture/core.md`; test quét catalog của H1 cũng kiểm các trường bắt buộc.

---

## To-do WPS (host Linux)

Làm khi chuyển sang host Linux (WPS Office for Linux, add-in `src/AxiomOffice.WPS/commands.js`). Mỗi mục bám đúng phần
C# đã làm ở trên:

- [ ] **H1**: tham số `presentation`/`workbook`/`document` cho mọi lệnh `wpp.*`/`et.*`/`writer.*` trong `commands.js`
      (hàm chọn tài liệu theo mẫu `wppPres`); live test hai tài liệu mở cùng lúc trên WPS thật.
- [ ] **H2**: thời hạn cho lệnh và phát hiện hộp thoại trong bridge WPS (cổng 47861, `core-go/internal/wpsbridge`);
      session sau khi máy ngủ.
- [ ] **H3**: `wpp.listSlides` đọc bảng (`Type` 19, `Table.Cell(r,c)`), nhóm (`Type` 6, `GroupItems`), tiêu đề biểu
      đồ, cùng định dạng `[table RxC]` với bản C#; test trong `tests/wps/test_addin_wpp.mjs` (fake đã có bảng:
      `addin_fake.mjs` `makeTable`).
- [ ] **H4**: lệnh thêm trả định danh (`slide`, `shape`) để Core ghi sổ đối tượng của lượt.
- [ ] **H5**: catalog thêm trường `backends` (`office`, `wps-linux`) sinh từ registry thật; Core lọc tool theo backend
      của phiên; kịch bản hành vi chung chạy trên C# live và WPS.
- [ ] **18-s0**: renderer của chế độ brief cho WPS (bảng Box → lệnh ở mục 5.3 của `18-s0`), benchmark slide chạy trên WPS.

## Không làm

- **LibreOffice**: dừng hỗ trợ từ 25/10/2026. Không sửa `src/AxiomOffice.LibreOffice`, không thêm test `tests/lo`. Gỡ
  hẳn code và CI LibreOffice là quyết định riêng, chưa làm.
- Không đổi tên `wpp`/`et`/`writer`: tốn công (catalog, các backend, cấu hình MCP của người dùng) mà không thêm năng lực.
- Không tự bấm hộp thoại của Office (H2): không phân biệt được hộp thoại lỗi với câu hỏi cần người dùng trả lời.
- Không chuyển add-in ra ngoài process trong đợt này: H2 giảm rủi ro treo trước, rồi đo xem có còn cần không.
- Không cho chạy song song nhiều lượt trên cùng app khi H1 chưa xong.
