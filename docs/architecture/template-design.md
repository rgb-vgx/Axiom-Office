# Thiết kế lại slide theo template đính kèm — ghi chép & kế hoạch

Cập nhật: 13/10/2026 (chiều) · trạng thái: **làm cả bài theo lô bằng `slides_from_template` (§6.5): 86/87 slide tự dựng
trong ~91 s trên PowerPoint 16, chưa chạy lượt thật với model** — chống làm thừa ở §6.6, việc còn lại ở §6.7.

Tài liệu này gom mọi thứ của phiên 13/10/2026 để làm tiếp: lượt chạy thật đã quan sát, những gì đã sửa,
tư vấn của ChatGPT và Gemini, quy trình đề xuất, và việc còn dang dở.

## 1. Nghiệp vụ

Giảng viên có bài cũ **`Y4.4-GAN.14.ppt`** (87 slide, khổ **4:3** = 720×540 pt, file `.ppt` Compatibility Mode)
và mẫu của trường **`mau bai giang.pptx`** (3 slide, khổ **16:9** = 960×540 pt). Yêu cầu điển hình:

> "Sử dụng template trong file đính kèm, kết hợp với việc sử dụng tavily, hãy design và cập nhật lại slide hiện tại nhé"

Mẫu (**đã kiểm bằng OOXML + COM**, §6.1 — bảng cũ theo ảnh Gemini có "nội dung kiểu 1" là đọc nhầm ảnh bìa):

| Slide mẫu | Kiểu | Thiết kế nằm ở đâu | Các khung (tên shape) |
|---|---|---|---|
| 1 | Bìa | `Picture 1` = ảnh nền JPEG 1890×1063 phủ kín slide (logo + tên trường ở trên, dải xanh – trắng – đỏ đô chiếm ~30% dưới) | `TextBox 3` tên bài (Arial 40 đậm `#C00000`), `TextBox 6` giảng viên/đơn vị 4 đoạn (16 đậm trắng, trên dải đỏ), `TextBox 7` "Hà Nội, 2026" |
| 2 | Nội dung **sidebar trái** | `Picture 1` = ảnh nền phủ kín: khối logo + dải đỏ đô rộng ~209 pt bên trái, phần còn lại trắng | `TextBox 2` tiêu đề (28 đậm `#1F4E79`, căn giữa, 594 rộng); 3 mục lặp I/II/III, mỗi mục = 2 `Rectangle` (bóng xám + nền `#CC1254`) + `Oval` + `TextBox` số La Mã + `TextBox` nội dung (17 đậm trắng, 529 rộng) |
| 3 | Kết thúc | Không ảnh nền; 3 logo (trường + 2 BV đối tác) là ảnh trên slide | `Title 3` "XIN CẢM ƠN…" (36 đậm `#C51553`) — là textbox dù tên "Title" |


Nhận diện bất di bất dịch: logo + tên trường, dải màu trang trí; màu chủ đạo **đỏ đô + xanh dương đậm** trên nền
trắng. Mọi chữ/ảnh phải nằm trong **vùng an toàn** (vùng trắng).

## 2. Lượt chạy thật đã quan sát (13/10/2026, model `oc/muse-spark-1.3-contributor-free`)

**Lập kế hoạch** (plan mode, 26,9 s, 3 vòng, ~31,5k token): chỉ đọc — `wpp.listSlides`, `ppt_profile` +
`ppt_get_text` của tệp đính kèm (đọc được nhờ đường dẫn đầy đủ), `wpp.checkLayout slide 1`. Kế hoạch 5 bước + 3 câu
hỏi (chỉ slide đang mở hay cả 87? giữ ảnh gốc? có cập nhật kiến thức?).

**Thực hiện** (người dùng bấm *Thực hiện kế hoạch* **khi chưa trả lời 3 câu hỏi**; dừng tay sau 278,9 s):

| | |
|---|---|
| Vòng / lần gọi tool | 30 vòng, 37 lần |
| Token đầu vào | **1.271.221** (1.159.629 từ cache) — đầu ra 19.304 |
| Tavily | 3 lần `web_search` thật, đều ok (LI-RADS/HCC; u máu/FNH/adenoma; áp xe/di căn) → thành nội dung + dòng **nguồn** dưới slide |
| Kết quả | **Thêm 6 slide MỚI ở cuối bài** (88–93: bìa, mục tiêu, 3 slide nội dung, cảm ơn — sau đó tự xoá slide cảm ơn trùng); slide người dùng đang mở **không đổi** |
| Thiết kế | Tự vẽ textbox, màu **xanh `#1F4E79`** cỡ cố định — không giống mẫu; chưa xử lý 4:3 vs 16:9 |
| Soát | slide 89: 1 lỗi, slide 90: 2 lỗi (chưa sửa thì bị dừng) |

## 3. Đã sửa trong phiên (chưa commit — xem §8)

| # | Vấn đề | Sửa |
|---|---|---|
| 1 | AI không biết slide đang mở | Pane gửi `document.view` (trình chiếu: `slide`/`slideCount`; bảng tính: `sheet`/`range`; văn bản: đoạn chọn). Core `ViewSection` ở **cuối** system prompt: "slide hiện tại = slide N, sửa tại chỗ trừ khi được bảo thêm slide" |
| 2 | Bấm thực hiện khi câu hỏi còn bỏ ngỏ | `PlanModeRule`: mỗi câu hỏi phải ghi **mặc định nhỏ nhất, an toàn nhất**; nút gửi "…câu hỏi chưa trả lời thì dùng phương án mặc định" |
| 3 | Không thấy phong cách mẫu | `ppt_profile` thêm khổ (pt + `16:9`/`4:3`), theme, **`usedColors`/`usedFonts`**. Đo trên mẫu: theme là Office mặc định (accent xanh — sai), `usedColors` = `#FFFFFF #CC1254 #000000 #BE4B4B #D2D2D2 #1F4E79 #C00000 #C51553`, font Arial |
| 5 | 1,27 triệu token | `wpp.listSlides` / `wpp.checkLayout` cả bài được cắt gọn (`office/truncate_slides.go`): giữ đủ số, slide đang mở + kề giữ nguyên, bỏ slide không lỗi |
| 6 | Mỗi request bị 400 vì `thinking` rồi gửi lại | Nhớ trên `model.Client` khi gửi lại không `thinking` thành công |

Kiểm: `go test ./...`, e2e Core 169/169, e2e MCP 63/63, add-in biên dịch; **đã cài** (build 02:59). Chưa chạy
lại lượt thật sau khi sửa.

**Vẫn còn** (ngoài template): mục 4 — **không hoàn tác được trên PowerPoint** (nút "Hoàn tác lượt này" chỉ có cho
Word; AI thêm 6 slide thì phải xoá tay).

## 4. Axiom hiện đọc được gì từ tệp mẫu

| Đọc được | Chưa đọc được |
|---|---|
| Chữ từng slide + ghi chú (`ppt_get_text`) | **Vị trí/kích thước từng khung** |
| Tên layout, **số** khung mỗi slide | **Màu nào tô phần nào** (tiêu đề hay dải nền) |
| Khổ slide + tỉ lệ | Hình/nền/gradient/đường cong/logo trong **slide master/layout** |
| Theme (màu, font) + màu/font thực dùng | Ảnh nhìn của mẫu (*Nhìn tài liệu* chỉ chụp được bài đang mở) |

→ AI chỉ *bắt chước* (đoán màu, tự vẽ textbox) nên không giống mẫu. *(Đã bù ở §6.3: `ppt_profile.kinds` đọc vị trí,
cỡ, màu từng khung, ảnh nền và vùng an toàn của mẫu.)*

## 5. Tư vấn

### 5.1 ChatGPT (qua MCP `chatgpt-lookup`, 13/10/2026)

Nguyên tắc: **model quyết định nội dung và chọn kiểu slide; Core/bridge lo bố cục, định dạng, kiểm tra.** Đừng để
model tái dựng template bằng lệnh vẽ textbox.

Ưu tiên:
1. **Dùng thẳng slide/layout của mẫu** rồi đổ nội dung vào placeholder — tất định nhất.
2. `Presentation.ApplyTemplate` — có thật, nhưng **không** tự ánh xạ 87 slide cũ vào bố cục mẫu; đổi khổ và
   thiết kế là hai việc khác nhau; phải làm trên bản sao, render kiểm, rollback được. Cạm bẫy: nội dung đè lên
   trang trí của master, placeholder không khớp, font bị thay, hình nền là ảnh trên slide chứ không ở master.
3. **Design spec** trích từ OOXML theo chuỗi slide → layout → master → theme: `p:sldSz`, `p:ph` type/idx (nối kế
   thừa), `a:xfrm`, `solidFill`/`gradFill`, `rPr`/`defRPr`, `p:pic`, `p:bg`, `showMasterSp`, `schemeClr` (phải giải
   qua color map + tint/shade). Vai trò khung suy **tất định**: placeholder type → kế thừa → vị trí/hình học →
   đặc trưng (logo nhỏ ở góc, dải lớn, ảnh phủ) → tên shape (tín hiệu phụ). Định dạng gọn ~100–300 token/layout:
   ```json
   {"page":"16:9","layout":"content_2col","fixed":["maroon_curve","logo"],
    "slots":[{"id":"title","role":"title","x":0.07,"y":0.08,"w":0.84,"h":0.12,"size":28,"color":"#A6192E"},
             {"id":"body1","role":"body","x":0.07,"y":0.25,"w":0.40,"h":0.62,"size":18}]}
   ```
   (schema minh hoạ, không phải số đo của mẫu thật)
4. Ảnh preview chỉ dùng **một lần** khi nhập mẫu hoặc khi duyệt kết quả.
5. Lệnh cho model nhỏ, có schema: `template.listLayouts`, `template.createSlide {layoutId}`,
   `slide.setPlaceholder {slideId, slot, text}`, `slide.setTable`, `slide.validate`; slot sai → trả danh sách
   đúng; `slideId` ổn định; idempotent; `addText` tự do chỉ là dự phòng.
6. Bridge thiết kế theo **năng lực** (`template.inspect/applyTheme/insertSlide/bindContent/validate`), mỗi backend
   trả `supported/partial/unsupported` — WPS COM và LibreOffice UNO (master page) phải kiểm theo phiên bản.
7. Copilot (Slide Master), Gamma 5 (nhập PPTX giữ layout), python-pptx (mở mẫu làm gốc, thêm slide theo layout,
   điền placeholder) — đều dựa vào layout/master.

### 5.2 Gemini (quy trình làm tay, người dùng chuyển lại)

1. Phân tích mẫu: các layout (View > Slide Master), nhận diện thương hiệu cố định, font, **vùng an toàn**.
2. **Save As mẫu thành file mới**; bài cũ chỉ là nguồn.
3. Tạo slide mới theo layout phù hợp; dán chữ bằng **Keep Text Only** (bỏ font/màu cũ, nhận định dạng mẫu).
4. Chép ảnh/biểu đồ, co vào vùng an toàn; **ảnh y khoa dùng bố cục sidebar** để phóng to tối đa.
5. Viết lại: gạch đầu dòng ≤ 2 dòng, cắt chữ thừa, SmartArt/khối màu đỏ đô–xanh của trường.
6. Logo đối tác gom về slide mở đầu/kết thúc.

### 5.3 Nhận xét (Claude) — đã kiểm ở §6.1

- **Mẫu không dùng placeholder** — đúng, và còn hơn thế: master/layout là Office mặc định (layout 4:3 cũ, nền
  trắng), **toàn bộ nhận diện nằm trong ảnh nền trên từng slide**. Hướng "đổ vào placeholder"/`ApplyTemplate`/design
  spec từ master của ChatGPT (§5.1 mục 1–3) **không áp dụng được cho mẫu này**.
- `InsertFromFile` mang theo ảnh nền vì ảnh nằm trên slide, không phụ thuộc bài đích.
- Mẫu có **3 slide**; "ảnh thứ 4" Gemini thấy là `docProps/thumbnail.jpeg` hoặc đọc nhầm.

## 6. Quy trình cho Axiom

### 6.1 Kết quả thí nghiệm (13/10/2026, bản sao trong thư mục tạm, phiên COM riêng không cửa sổ)

*Chạy qua `PowerPoint.Application` nên thực tế là **WPS Presentation** (xem cảnh báo §6.4); kết quả của 5 lệnh
đã kiểm lại trên PowerPoint 16 thật.*

Đọc OOXML của mẫu + chạy COM trên bản sao `mau.pptx`, `old.ppt` (script tạm, không giữ trong repo — nếu cần làm lại
thì dựng theo kiểu `tests/win/measure_slide_grid_ppt.ps1`: từ chối khi PowerPoint đang chạy, cuối cùng `Quit`).

| Câu hỏi | Kết quả |
|---|---|
| Mẫu dùng gì? | Khổ 960×540, master + 11 layout **Office mặc định** (layout vẫn toạ độ 4:3, nền trắng). Slide 1–2 dùng layout `Blank`, slide 3 `Title and Content` nhưng **không có placeholder nào** (`Type` 13 ảnh, 17 textbox, 1 hình). Thiết kế = ảnh nền 1890×1063 phủ kín trên slide (`ppt/media/image1.jpg` bìa, `image2.jpg` sidebar) |
| `Slides.AddSlide(i, CustomLayout của slide mẫu)` | Ra slide **trắng, 0 shape** — loại |
| `Slide.Duplicate()` trong bài tạo từ mẫu | Giữ đủ 17 shape, ảnh nền, vị trí — **dùng được** |
| `Slides.InsertFromFile(mau, i, k, k)` vào **bài trắng bất kỳ** | Giữ đủ shape + ảnh nền + màu/cỡ chữ (vì thiết kế nằm trên slide) — **dùng được**, không cần chép slide mẫu trong bài |
| Thay `TextFrame.TextRange.Text` | Giữ font/cỡ/đậm/màu của khung (Arial 28 đậm `#1F4E79` còn nguyên). Khung nhiều đoạn (bìa `TextBox 6`, 4 đoạn) thay bằng chuỗi có `\r` → mọi đoạn nhận định dạng **đoạn đầu** |
| Tiêu đề dài ở slide sidebar (594 rộng, 28 pt, căn giữa) | 57 ký tự → 2 dòng, đẹp; dòng thứ 3 sẽ đè mục I (y=144) → tối đa **2 dòng ≈ 70 ký tự**. Phiên không cửa sổ: `AutoSize=1` nhưng `Height` **không tự giãn** (46,8 dù 2 dòng) → `checkLayout` báo overflow; trong phiên có cửa sổ của người dùng cần đo lại |
| Ô mục tiêu (529 rộng, 17 pt đậm) | ~57 ký tự/dòng, vừa **2 dòng ≈ 110 ký tự** trong khối 59 pt |
| Xoá một mục lặp (2 `Rectangle` + `Oval` + 2 `TextBox`) | Xoá theo tên, phần còn lại không xê dịch — slide 2 mục vẫn đẹp (khoảng trống phía dưới) |
| Bài cũ `old.ppt` | 87 slide 720×540, 1 master; **79 slide có ảnh** (217 ảnh `Type 13` + 17 placeholder chứa ảnh); nền xanh đậm, tiêu đề vàng `#FFFF00` |
| Chép ảnh: `Shape.Export(path, 2)` (hàm ẩn) | Chạy được, **không đụng clipboard**, ra PNG đúng độ phân giải gốc (611×422 = ảnh gốc trong OOXML; 510×378 = gốc) |
| Chép ảnh: `Shape.Copy` + `Shapes.Paste` | Giữ ảnh gốc, giữ nguyên toạ độ cũ — nhưng **ghi đè clipboard người dùng** → không dùng |
| `InsertFromFile` slide cũ (4:3) vào bài 16:9 | Không co giãn, mọi shape **dịch phải 120 pt** (căn giữa); màu theo theme bị ánh xạ lại (chữ trắng → đen) nhưng màu RGB giữ (tiêu đề vàng trên nền trắng — không đọc được) → **không mang chữ cũ sang**, chỉ lấy ảnh |
| `ApplyTemplate(mau)` lên bản sao bài cũ | Khổ **vẫn 720×540**, 402 → 402 shape, nền thành trắng (master Office), chữ vàng mất tương phản, không có logo/dải màu — **loại hẳn** |

Hệ quả: với mẫu kiểu này, "mẫu" = **bộ slide mẫu để nhân bản**, không phải master/layout. Mẫu khác của trường khác
có thể dùng placeholder thật — lệnh phải chạy được cả hai (§6.3).

### 6.2 Quy trình

1. **Phân tích mẫu một lần** (Core, tất định, từ OOXML — `pptx` đã đọc được): mỗi slide mẫu → một **kiểu**:
   - nền: `p:pic` phủ ≥ 90% slide → `fixed` (không bao giờ sửa/xoá);
   - **ô chữ** = mọi textbox/placeholder có chữ, kèm tên shape, toạ độ, cỡ chữ, màu, số ký tự hiện có, **sức chứa
     ước tính** (số dòng × ký tự/dòng từ bảng đo `co-so-do-duoc.md`);
   - **mục lặp**: các nhóm shape có cùng mẫu hình học lệch nhau theo trục y (mục I/II/III) → `items: [[tên…], …]` để
     model nói "giữ 2 mục" thay vì kể tên 5 shape;
   - **vùng an toàn** = khung bao các shape không phải nền (slide sidebar ≈ x 230–940) — ảnh y khoa đặt vào đây;
   - công dụng gợi ý theo kiểu: bìa / nội dung (danh sách) / nội dung + ảnh lớn / kết thúc.
   Đưa vào `ppt_profile` (đã có khổ, màu, font — thêm phần `kinds`). Ví dụ mẫu này:
   ```json
   {"page":"16:9","kinds":[
     {"kind":2,"use":"noi dung, danh sach muc","safe":[230,20,940,520],
      "slots":[{"shape":"TextBox 2","role":"title","size":28,"max":"2 dong ~70 ky tu"}],
      "items":[["Rectangle 3","Rectangle 4","Oval 5","TextBox 6","TextBox 7"], ...],
      "itemText":{"number":"TextBox 6","body":"TextBox 7","max":"2 dong ~110 ky tu"}}]}
   ```
2. **Tạo bài MỚI từ mẫu** cạnh bài cũ (`Y4.4-GAN.14 - mau moi.pptx`): chép tệp mẫu, xoá hết slide (giữ khổ +
   master), mở. Bài cũ giữ nguyên → không phải đổi 4:3 → 16:9.
3. Mỗi slide cũ: **model chỉ** chọn kiểu + viết lại chữ (+ chọn ảnh cần mang sang).
4. **Bridge làm tất định**: `InsertFromFile` slide mẫu → thay chữ theo tên shape → xoá mục lặp thừa → xuất ảnh
   cũ bằng `Shape.Export` rồi `AddPicture` co vừa vùng an toàn, giữ tỉ lệ.
5. `wpp.checkLayout` từng slide vừa tạo; ảnh preview chỉ để duyệt.

### 6.3 Lệnh đã viết (13/10/2026) — `CommandDispatcher.Template.cs`

Registry chung 65 → 70 lệnh (`impress.py`, sinh lại catalog/`livecommands_gen.go`/`commands.js`); LibreOffice báo
"not available", WPS chưa có handler (nằm trong `CHUA_CO_HANDLER` của `tests/wps/test_addin.mjs`, bị lọc khỏi catalog).

| Lệnh | Cài đặt trên PowerPoint (khác kế hoạch ở chỗ in nghiêng) |
|---|---|
| `wpp.newFromTemplate {template, path}` | *`Presentations.Open(template, Untitled)`* (mẫu không bị sửa, `.potx` cũng được) → xoá hết slide → `SaveAs(path, pptx)`. Từ chối nếu `path` đã có / không phải `.pptx`. *Không trả `kinds`* — model lấy từ `ppt_profile` |
| `wpp.addSlideFromTemplate {template, kind, slide?}` | `Slides.InsertFromFile(template, slide, kind, kind)`; trả `textShapes` (tên, chữ mẫu, cỡ) |
| `wpp.setShapeText {shape, text, slide?}` | Giữ định dạng khung; `\n` → `\r`; sai tên → danh sách tên khung chữ. *Khung `AutoSize = 1` được đặt lại chiều cao = `BoundHeight` + lề* (đo: PowerPoint qua COM **không** tự giãn, kể cả phiên có cửa sổ); trả `lines`, `height`, `overflow` |
| `wpp.deleteShapes {shapes, slide?}` | `shapes` là mảng hoặc chuỗi JSON; trả `deleted`/`notFound`/`refused`; ảnh phủ ≥ 90% slide bị từ chối |
| `wpp.copyPicture {from, fromSlide, shape?, slide?, left, top, width, height}` | Bài nguồn tìm theo `FullName` rồi tên tệp; *chưa mở thì mở chỉ đọc CÓ cửa sổ rồi `Activate` lại bài đang làm* (không mở ngầm vì `Close` bài không cửa sổ làm sập). `Shape.Export(png)` → `AddPicture` → co vừa khung, căn giữa; khung mặc định = slide trừ lề 36 |

`ppt_profile.kinds` (`core-go/internal/pptx/kinds.go`, tệp ≤ 40 slide) — trên mẫu thật ra đúng 3 kiểu (`cover`,
`list` có 3 mục × 5 shape, `closing`), vùng an toàn bìa `[12,110,936,226]`, sidebar `[222,12,726,516]`,
`pictureBox` sidebar `[222,122.6,726,405.4]`; ~2,3 KB JSON.

`wpp.checkLayout` (C#) bỏ lỗi giả của slide mẫu (ảnh nền lệch mép, khung rộng quá mép nhưng chữ trong slide, nhãn
đặt trên ô màu) và báo `overflow` khi chữ làm khung giãn ra khỏi ô màu. **Bản LibreOffice (`checks.py`) chưa port**.

**Chạy thật** (harness gọi `CommandDispatcher.Execute` trong DLL vừa biên dịch, PowerPoint riêng, bản sao tệp): bài
mới 4 slide — bìa, mục tiêu 2 mục (xoá mục III), ảnh giải phẫu chép từ `.ppt` 4:3 vào `pictureBox` (547×405), cảm ơn;
tiêu đề 57 ký tự giãn thành 2 dòng đúng chỗ; `checkLayout` 20 lỗi giả → 1 lỗi đúng (ô mục II cố tình 172 ký tự).
Tệp 862 KB. Lỗi nhập đều trả thông báo sửa được (ghi đè tệp, sai tên khung, `kind` 7, shape nguồn không có).

### 6.4 Lượt thật với model (13/10/2026 trưa, PowerPoint 16, `ocg/mimo-v2.6-flash`)

> **Cảnh báo đo đạc**: WPS đã ghi đè `HKCU\Software\Classes\CLSID\{91493441-…}` (CLSID của
> `PowerPoint.Application`) sang `wpp.exe`. Thí nghiệm §6.1 và lần chạy harness đầu **thực ra chạy trên WPS
> Presentation 12.1**. Đã chạy lại harness trên PowerPoint 16 thật (mở thẳng `POWERPNT.EXE /automation` rồi
> `GetActiveObject`): 5 lệnh cho kết quả giống hệt; khác duy nhất là PowerPoint **tự giãn** khung `AutoSize`
> (WPS thì không — phần đặt lại chiều cao trong `setShapeText` dành cho WPS). Script cũ dùng
> `New-Object -ComObject PowerPoint.Application` (vd `tests/win/measure_slide_grid_ppt.ps1`) có thể cũng đã đo
> trên WPS — cần kiểm lại.

Model cấu hình của người dùng (`oc/muse-spark-1.3-contributor-free`) nay bị gateway từ chối (403 FreeTierError);
`oc/*` free không còn, dùng `ocg/mimo-v2.6-flash` qua `AXIOM_LLM_MODEL` cho riêng Core chạy thử. Prompt giống pane:
"Sử dụng template trong file đính kèm, hãy design lại 6 slide đầu … (giữ nguyên ảnh y khoa)" + khối tệp đính kèm.

| Lượt | Diễn biến | Sửa sau lượt |
|---|---|---|
| 1 | Đọc skill + `mau-dinh-kem.md` đúng, nhưng **không biết đường dẫn bài cũ** (prompt chỉ có tên) → `recall`, đoán đường dẫn; bỏ qua `newFromTemplate`, **chèn slide mẫu 16:9 vào bài cũ 4:3**. Dừng tay ở 415 s | (1) `DocumentSection` thêm *"Its full path: …"*; (2) `addSlideFromTemplate` **từ chối** khi khổ bài đang mở ≠ khổ mẫu (đọc `p:sldSz` trong tệp mẫu), lỗi chỉ "gọi `wpp.newFromTemplate` trước"; (3) skill: lệnh sửa đầu tiên luôn là `newFromTemplate` |
| 2 | **Hoàn thành**: 27 vòng, 889 s, 2 lỗi (đọc `.ppt` bằng tool tệp), token phải trả **67.804** (886k đầu vào, 841k cache). `newFromTemplate` đúng tên gợi ý → bìa (kind 1) + 5 slide ảnh (kind 2, xoá đủ 15 shape mục lặp, `copyPicture` vào `pictureBox`, slide 3 hai ảnh chia đôi), ghi chú từng slide, `checkLayout` 0 lỗi, tự xoá/đặt lại nhãn khi bị báo chồng. Bài cũ không đổi | (4) lỗi `.ppt/.doc/.xls` của tool tệp chỉ sang `office_action` + `copyPicture`; (5) skill: **không gắn nhãn lên ảnh y khoa** — model đặt "GAN TRÁI / GAN PHẢI" ngược hình ở slide phân khu |

Kết quả lượt 2 (bản lưu: thư mục tạm của phiên, `live\Y4.4-GAN.14 - mau moi.pptx`, 3,2 MB / 6 slide).

### 6.5 Cả bài nhiều slide — `slides_from_template` (13/10/2026 chiều)

**Vấn đề**: làm từng slide qua `office_action` tốn ~4 lần gọi tool + ~2,5 phút mỗi slide; trần 200k token của pane
chỉ đủ ~15–18 slide; 87 slide ~3,5 giờ, model yếu dễ lạc.

**Tư vấn ChatGPT** (MCP `chatgpt-lookup`): chọn "Core chia lô + plan JSON + dựng tất định", và quan trọng hơn: **đừng
dùng LLM cho slide máy quyết được**. Plan ngắn chỉ tham chiếu mã đã cấp (`src`, số ảnh), kiểm 3 lớp (cấu trúc → ngữ
nghĩa/`maxChars` → `checkLayout`), lỗi trả theo từng dòng để model sửa đúng dòng; tag idempotent trên slide đích để
chạy lại không trùng; mỗi lô chỉ gửi dữ liệu của lô; JPEG cho ảnh chụp. Không theo: "bridge tự giảm cỡ chữ khi tràn"
(trái skill: cắt chữ, không hạ cỡ), OCR (chữ `.ppt` đọc thẳng qua COM).

**Cài đặt**
- Bridge: `wpp.sourceManifest {from, first, last}` — mỗi slide nguồn: tiêu đề (khung chữ cao nhất, bỏ số trang/chân
  trang), chữ, ảnh đánh số theo thứ tự đọc (kèm `w`,`h`, `overlays` = số nhãn/đường kẻ vẽ đè), `other` (nhóm, bảng,
  biểu đồ). OLE nhúng (sơ đồ) tính là ảnh. `wpp.addSlideFromTemplate {tag}` — slide mang `Tags("AXIOM_SRC")`; chạy lại
  cùng tag thì thay tại chỗ, slide mới chèn sau tag nhỏ hơn. `wpp.listSlides` trả `tag`. `wpp.copyPicture {overlays,
  format}` — có nhãn đè thì xuất **vùng slide** (Slide.Export rồi cắt, không sửa bài nguồn); `jpg` chất lượng 90.
  `wpp.open` tệp đang mở thì chỉ kích hoạt cửa sổ.
- Core: tool `slides_from_template {template, source, target?, first?, last?, plan?, closing?}`
  (`tools/template_build.go`, chỉ PowerPoint, không có trong plan mode). Tự tạo/mở bài đích (`<tên> - mau moi.pptx`),
  lô 25 slide, quy tắc tự quyết:
  - slide 1 không ảnh → bìa (tiêu đề + ô giảng viên);
  - tiêu đề: khung cao nhất nếu "giống tiêu đề" (≤ 3 dòng ngắn, không mở bằng "SA:/CLVT:/GPB:", không câu dài có
    phẩy); nhiều cấp quá dài thì lấy cấp sâu nhất; slide không có tiêu đề → **mang tiêu đề mục đang trình bày**;
  - một nhãn ngắn ("Đại thể") nối vào tiêu đề; đoạn mô tả giữ nguyên **cột trái 38%** (16 pt), ảnh cột phải;
  - ảnh chia lưới theo tỉ lệ thật (tổng diện tích lớn nhất); > 4 ảnh tách slide (tag `9`, `9.2`);
  - slide chữ ≤ 3 dòng vừa ô → mục lặp của mẫu; dài hơn → giữ nguyên văn trong vùng nội dung (18 pt);
  - bảng/nhóm/biểu đồ, chữ quá dài, không tìm được tiêu đề → `needsPlan` (kèm chữ gốc + lý do) cho model viết.
  Plan row `{src, kind?, title, items?, text?, imgs?|"all", notes?, texts?}`; lỗi `{src, field, error, max, actual,
  allowed}`.

**Đo trên Y4.4-GAN.14 (87 slide, PowerPoint 16, không model)**: **86/87 slide tự dựng**, 4 lô 21+24+28+18 s ≈ **91 s**,
0 lỗi bố cục, 90 slide đích (3 slide nhiều ảnh được tách). Chỉ slide 52 (có nhóm) cần model. Dung lượng: PNG +
vùng 3 px/pt = **97,8 MB** → JPEG 90 + 2,5 px/pt = **9,2 MB**. Lần đo đầu chỉ tự quyết 27/87 (thiếu tiêu đề mang
sang, coi đoạn mô tả là tiêu đề) — đã sửa như trên.

**Còn lại**: chưa chạy lượt thật với model trên công cụ này (người dùng tự chạy, prompt ở dưới); slide "chữ + ảnh"
chữ 16 pt trong cột 38% — với đoạn dài gần trần sẽ dày; nhãn đè dạng chữ nằm ≥ 50% trong ảnh mới tính là nhãn (khối
chữ nằm cạnh ảnh thành chữ cột trái).

**Lượt thật đầu tiên của người dùng** (13/10 13:44, `ocg/deepseek-v4.1-flash`, Y4.5-ĐM.14.ppt 95 slide, "xây dựng
lại slide hiện tại"): xong, 18 vòng, 2 phút 44 s, 304k token đầu vào. Dựng chỉ **1** lần gọi `slides_from_template`
(2,6 s); nhưng ~10 vòng sau đó model đọc bài mới bằng `ppt_profile`/`ppt_get_text` — tệp trên đĩa còn rỗng (chưa lưu)
nên nó tưởng hỏng, dựng lại, đọc đi đọc lại. Bìa giữ khối giảng viên **của tệp mẫu** vì dòng kế hoạch chỉ có title +
notes. Sửa: tool **lưu bài mới sau mỗi lô** và báo "kiểm bằng wpp.listSlides/checkLayout"; dòng kế hoạch chỉ ghi đè
trường model viết, trường bỏ trống lấy từ quy tắc tự động; `.ppt/.doc/.xls` báo lỗi rõ ngay ở tool tệp.

**Lượt thật cả bài** (13/10 14:06, `ocg/deepseek-v4.1-flash`, Y4.5-ĐM.14.ppt 95 slide): 4 lần `slides_from_template`
≈ 90 s, **95/95 slide tự dựng** (0 cần model) + slide cảm ơn, lưu sau mỗi lô. Nhưng lượt **dừng vì vượt ngân sách**
(202k > 200k token phải trả, 29 vòng, 8,5 phút): sau khi xong model đọc lại bài nguồn 4 lần, `checkLayout` từng slide,
rồi **xoá ghi chú 97 slide** — `InsertFromFile` mang theo ghi chú của slide mẫu ("Phân bổ thời gian… cấp cứu bụng").
Sửa: tool luôn đặt lại ghi chú mỗi slide (cả slide cảm ơn); thông báo kết thúc nói rõ "FINISHED… đừng kiểm lại từng
slide, trả lời người dùng ngay".

**Prompt chạy thử** (mở bài cũ trong PowerPoint, đính kèm tệp mẫu):
> Dựng lại toàn bộ bài giảng đang mở theo template trong file đính kèm. Giữ nguyên ảnh y khoa và nội dung chuyên môn
> của bài cũ, không sửa bài cũ.

### 6.6 Chống model làm thừa sau khi xong (13/10/2026 chiều)

Lượt cả bài Y4.5-ĐM.14 hết ngân sách dù việc xong ở vòng 5 (§6.5). Tư vấn ChatGPT (có tra internet: OpenAI Agents
SDK `StopAtTools`, Anthropic context editing / `stop_reason`, LangChain `ContextEditingMiddleware` /
`ToolCallLimitMiddleware`, Claude Code hooks): **"xong" phải là trạng thái của Core, không phải câu nhắc cho model**.
Đã cài cả năm khuyến nghị trong vòng lặp agent (`model/agent.go`, áp dụng cho mọi tác vụ):

| # | Cơ chế | Cài đặt |
|---|---|---|
| 1 | Tool báo xong → kết thúc lượt | `ToolResult.Final/FinalText`. `slides_from_template` đặt `Final` khi **mọi** slide nguồn đã có slide đích (tag), không còn `needsPlan`/lỗi, đã lưu, đã có slide cảm ơn → Core trả câu tiếng Việt do nó viết, không gọi model thêm |
| 2 | Ngữ cảnh gọn | Đầu vào một vòng > 40.000 token → thay kết quả tool cũ bằng `{"cleared":true,…}`, giữ 3 kết quả gần nhất, chỉ khi thu hồi ≥ 40.000 ký tự (≈ 10k token, bù được cache hỏng). Hiểu cả dạng OpenAI (`role: tool`) và Anthropic (`tool_result`). `wpp.sourceManifest` đưa cho model được cắt chữ còn 160 ký tự/đoạn khi vượt 8.000 ký tự |
| 3 | Ngân sách 3 mức | ≥ 80% phải trả: nhắc một lần "chỉ làm phần còn thiếu"; ≥ 90%: **tắt tool**, nhắc trả lời (việc đã làm, còn thiếu gì, cách tiếp tục); > 100%: dừng như cũ |
| 4 | Hạn mức theo tool/lệnh mỗi lượt | `tools.CallLimits`: `wpp.sourceManifest` 3, `wpp.listSlides` 6, `wpp.checkLayout` 20, `wpp.setNotes` 20, `slides_from_template` 20, `web_search` 5, `ppt_get_text`/`ppt_profile`/`doc_get_text` 4. Lệnh ghi theo khối (`et.writeRange`…) không giới hạn |
| 5 | Chặn lệnh y hệt | Cùng tool + cùng đối số lần thứ 5 trong lượt → không chạy, trả lỗi "blocked … use the earlier result" |

Test: `model/agent_guard_test.go` (kết thúc ở vòng 1 khi tool `Final`; chặn lần gọi y hệt thứ 5; vòng 3 không còn
`tools` và có lời nhắc khi đã dùng 95%; dọn kết quả cũ cả hai dạng, không dọn khi thu hồi ít),
`TestCallLimitsStopRepeatedReads`, `TestSlidesFromTemplateBuildsAndReportsNeedsPlan` (gửi dòng cho slide cuối → `Final`).

**Lượt thật sau khi cài** (13/10 17:00, deepseek-v4.1-flash, Y4.5-ĐM.14): **6 vòng, 79 s**, 123k token đầu vào, 4,5k đầu
ra — lượt tự kết thúc sau lô cuối bằng câu trả lời của Core (trước: 29 vòng, 8,5 phút, hết ngân sách). Tệp đích của
lượt 14:06 vẫn còn nên cả 95 slide được bỏ qua (`alreadyBuilt`); câu cuối khi đó nói sai "đã dựng lại xong" và điều
kiện `Final` đòi "vừa lưu" → sửa: câu cuối nói rõ dựng mới bao nhiêu / có sẵn bao nhiêu, không thay đổi gì thì không cần
lưu vẫn kết thúc.

**Lượt làm lại từ đầu** (13/10 17:04): xong, **8 vòng, 2 phút 39 s**, 113k token đầu vào, 5k đầu ra, tự kết thúc. Nhưng
giữa lô đầu (sau slide 14) "bài đang hoạt động" chuyển sang **bài cũ** — khung Axiom gắn vào cửa sổ bài cũ, bấm vào đó
là đổi bài; các lệnh sau trúng bài cũ (slide 15 "không có TextBox 2", slide 16–25 bị `RequireSameSize` chặn — bài cũ
không bị sửa). Model gửi lại lô với dòng kế hoạch, dựng xong. Sửa: 9 lệnh (`addSlideFromTemplate`, `setShapeText`,
`deleteShapes`, `copyPicture`, `setNotes`, `checkLayout`, `listSlides`, `addText`, `save`) nhận `presentation` (đường dẫn;
mặc định bài đang hoạt động) và tool **luôn chỉ đích danh bài mới**. Câu cuối đếm sai "dựng 20, 75 có từ trước" (đếm
riêng lô cuối) → cộng dồn số slide dựng trong cả lượt.

Chưa đo: ảnh hưởng của dọn ngữ cảnh lên cache của deepseek/mimo (ngưỡng 40k là điểm khởi đầu theo tư vấn).

### 6.7 Còn lại

- Lượt thật trên **cả bài 87 slide** (theo từng phần), và với slide chữ dùng `kind` danh sách (lượt 2 chỉ gặp slide ảnh).
- `Shape.Export` với ảnh bị **crop** và placeholder chứa ảnh (17 cái trong bài cũ) — chưa thử. Dung lượng: 6 slide /
  6 ảnh = 3,2 MB → ước ~40 MB cho 80 ảnh; cân nhắc xuất JPG.
- Kiểm lại số đo skill slide (`co-so-do-duoc.md`) trên PowerPoint thật (xem cảnh báo §6.4).
- Mẫu có placeholder thật — `kinds` chưa lấy toạ độ kế thừa từ layout.
- Ảnh nền bìa của mẫu lệch `top = -22` → đáy bìa hở dải trắng 22 pt (lỗi của mẫu).
- Phiên Linux: lệnh cho LibreOffice/WPS, port luật `checkLayout` mới sang `checks.py`.

## 7. Việc khác còn mở

- **Học từ LangChain 1.4.5** (`C:\Users\ThuyetMT\Documents\work\langchain`, `libs/langchain_v1/langchain/agents/middleware`):
  đã cài ContextEditing, ToolCallLimit (§6.6) và — 13/10 tối — guard (`model/guards.go`), model dự phòng, cứu tràn ngữ
  cảnh, cửa sổ theo model, `write_todos` (CHANGELOG). Không áp dụng: `LLMToolSelector` (thêm một lần gọi),
  `ProviderToolSearch` (chỉ Anthropic/OpenAI), `PII` (che một chiều, hỏng việc sửa tài liệu), prompt caching Anthropic,
  `ToolEmulator`. Lưu ý: **không chạy Python trong thư mục đó** — `agents/middleware/types.py` che module chuẩn `types`.
- **Hoàn tác trên PowerPoint** (mục 4 §3).
- **Phiên Linux**: pane LibreOffice/WPS thêm công tắc lập kế hoạch (`options.mode`), nút đính kèm, ô khoá Tavily
  (`TavilyApiKey` trong `~/.config/axiom-office/config.json`), gửi `document.view`. Core đã sẵn sàng. Ghi ở
  `docs/architecture/ribbon-ui.md` §8.
- **Sub-agent**: tạm hoãn theo yêu cầu người dùng.

## 8. Trạng thái mã (cuối phiên 13/10/2026)

Commit đã có trên `main` (chưa push): `0e219db` (ribbon đợt 1), `76936eb` (ribbon Quản lý/Hệ thống, một tab),
`3ac8f1d` (ảnh slide thật cho Nhìn tài liệu, pane hiện đủ bước, markdown).

**Chưa commit** — plan mode, đính kèm tệp, Tavily, và năm sửa ở §3:
- Core: `agent/orchestrator.go`, `agent/prompt.go`, `agent/agent_test.go`, `config/config.go`, `api/setup.go`,
  `model/client.go`, `model/model_test.go`, `office/truncate.go`, `office/truncate_slides.go`(+test),
  `pptx/pptx.go`, `pptx/style.go`(+test), `mcpserver/filetools_ppt.go`, `tools/{tools,office_action,mcp_tools,truncate}.go`,
  `tools/{plan,websearch}_test.go`, `tools/websearch.go`, `tools/truncate_test.go`
- Add-in: `Ai/AskAiPane.cs`, `Ai/CoreClient.cs`, `Ai/PaneControls.cs`, `Ai/SetupWizardForm.cs`,
  `Bridge/Config.cs`, `Bridge/HostProbe.cs`
- Tài liệu: `CHANGELOG.md`, `docs/architecture/ribbon-ui.md`, tài liệu này
- Làm theo mẫu (§6.3): `core-go/internal/pptx/kinds.go`(+test), `src/AxiomOffice/Bridge/CommandDispatcher.Template.cs`,
  `CommandDispatcher.cs`, `CommandDispatcher.Checks.cs`, `impress.py`, catalog sinh lại (`catalog/live-commands.json`,
  `mcpserver/livecommands_gen.go`, `AxiomOffice.WPS/commands.js`), `tests/lo/test_extension.py`,
  `tests/wps/test_addin{,_wpp}.mjs`, skill `trinh-bay-chuyen-nghiep` (`SKILL.md`, `references/mau-dinh-kem.md`)

Không thuộc phiên này, đừng commit lẫn: `tests/bench/run_prompt.py` (thay đổi riêng của người dùng).
