# 18 / S0 - Slide: bộ đo chất lượng và thiết kế spec dùng chung

> Mục tiêu: agent làm slide **đẹp một cách ổn định**, kể cả với model yếu. Không dựng thêm một hệ thống làm slide
> song song. Ta nâng cấp đường đang có để mọi cách tạo slide (từ brief, theo tệp mẫu) đi chung một spec, một
> layout solver và một bộ đo.

Rà soát code ngày **25/10/2026**. Mục 1 phân biệt rõ **đã chạy và kiểm chứng** với **mới là đề xuất**.

## 1. Trạng thái của tài liệu này

| Hạng mục | Trạng thái |
|---|---|
| Test characterization cho solver: `core-go/internal/tools/template_layout_test.go` | **Đã chạy, xanh** (`go test ./internal/tools/ ./internal/pptx/`) |
| Bộ 24 brief: `tests/bench/slides/briefs.json` | Đã viết; cấu trúc và việc "số liệu chấm được" được test kiểm |
| Bộ chấm: `tests/bench/slides/score.py` + `test_score.py` | **Đã chạy, 5/5 xanh** (dữ liệu giả, không cần PowerPoint) |
| Bộ chạy: `tests/bench/slides/run.py` | **Đã chạy thật** trên PowerPoint 2019 x64 (25/10/2026): smoke × 3 = 21 lượt, 21 lượt chạy hết |
| Baseline | **Có, cho một model yếu** (`ocg/deepseek-v4.1-flash`): mục 9. Còn thiếu model mạnh làm mốc (mục 8) |
| Thiết kế spec, solver, migration (mục 5–7) | Đề xuất, chưa triển khai |

## 2. Đã có (đối chiếu code)

**Hai đường tạo slide, hai bộ quy tắc khác nhau:**

| | Đường tự do (không có mẫu) | Đường theo mẫu: `slides_from_template` |
|---|---|---|
| Ai tính toạ độ | **Model**: đọc bảng lưới trong skill `trinh-bay-chuyen-nghiep` rồi gọi `wpp.addText {left, top, width, height}` | **Go**: `contentRegions`, `cardBoxes`, `fitCards`, `pictureBoxes` (`template_compose.go`, `template_build.go`) |
| Spec | Không có | `composeRow`: `layout ∈ {cover, agenda, section, content, cards, table, compare, update}`, kèm `points`, `cards`, `table`, `left/right`, `callout`, `caption`, `imgs`, `notes`, `sources` |
| Kiểm trước khi dựng | Không có; chỉ soát sau bằng `wpp.checkLayout` | `checkCompose` trả mã lỗi cho model (`NEED_2_TO_4_CARDS`, `TOO_LONG`, `TABLE_SIZE`…) |
| Lệnh dựng | `wpp.addText` (không chọn phông, không tô nền), `wpp.addTable` (luôn 18pt) | `wpp.addParagraphs` (có phông, nền, viền, hộp bo góc, tự co chữ), `wpp.addGrid`, `wpp.copyPicture` |
| Cỡ chữ | `_design/tokens.json`: body 20, title 28; skill cấm hạ cỡ | Hằng số trong Go: 24/22/16/15/14/12, **cho tự co** đến `minFontSize` 12–14; không đọc `tokens.json` |
| Nền tảng | Windows, LibreOffice, WPS Linux | **Chỉ Windows**: LibreOffice trả `_template_unsupported` cho 9 lệnh |

Các lệnh dựng tốt nhất (`addParagraphs`, `addGrid`) đều có `agent: false` trong `catalog/live-commands.json`. Model
không gọi được chúng; chỉ tool Go gọi.

**Soát:**
- `wpp.checkLayout` (C# và LibreOffice) báo `overflow`, `offslide`, `overlap`, `dense`, `small-font`, kèm
  toạ độ từng shape.
- `look_at_document`: tuỳ chọn bật thêm, **chỉ chụp slide đang hiện**, khuyên gọi 1–2 lần mỗi lượt. Phía C# đã có
  `Slide.Export`, nhưng chưa có lệnh "render slide số n".

**Đo:** `17-r0` đặt khung đo (chạy theo kịch bản, N lần × model, chấm bằng lệnh đọc, **không chấm bằng LLM**).
`tests/bench/` mới có bài Calc. Bộ slide ở tài liệu này là phần slide của khung đó.

**Không có:** engine OOXML dựng slide. Gói Go `pptx` chỉ **đọc** mẫu (`ReadTemplate`, `kinds`); tool file `ppt_*`
có 4 lệnh (`ppt_profile`, `ppt_get_text`, `ppt_create`, `ppt_add_slide_file`). Mọi đường dựng đều qua bridge live.

## 3. Bảng: năng lực có sẵn → khoảng trống → thay đổi → test chấp nhận

| # | Có sẵn | Khoảng trống | Thay đổi | Test chấp nhận |
|---|---|---|---|---|
| 1 | `composeRow` + `checkCompose` | Chỉ dùng được khi có `template` **và** `source`; đường tự do bắt model tính toạ độ | Spec dùng chung (mục 5); thêm chế độ dựng từ brief, không cần mẫu hay nguồn | Cùng một spec dựng được cả khi có mẫu lẫn khi không; brief smoke không còn lệnh `wpp.addText` nào do model gọi để dựng slide mới |
| 2 | Hàm hình học thuần trong `tools` | Gắn với `pptx.Kind`; không có phương án dự phòng | Gói `slidelayout` (mục 6) với `Frame` độc lập với mẫu; chuỗi fallback | Test characterization giữ nguyên số; test fallback: nội dung dài → tách slide, không hạ cỡ dưới cỡ tối thiểu của theme |
| 3 | `tokens.json` (đường tự do) và hằng số Go (đường mẫu) | Hai bộ typography | Hằng số Go lấy từ một `Theme`; chế độ brief đọc `tokens.json`; chế độ mẫu giữ hằng số cũ cho tới khi benchmark chứng minh nên đổi | Test: chế độ brief dùng đúng cỡ trong `tokens.json`; bridge-call golden của chế độ mẫu không đổi |
| 4 | Ước lượng chữ: `0.53 × cỡ` cho mỗi ký tự, dòng `1.21 × cỡ`, đo với Arial | Mẫu có phông khác vẫn dùng hệ số Arial. Phép `int()` trên số thực làm giá trị đúng biên bị tính thiếu (`106 / (0.53 × 20)` ra 9, không phải 10) | Giao diện `TextMeasurer`: mặc định giữ đúng công thức cũ; bảng hệ số theo phông đo bằng cách trong `co-so-do-duoc.md` | Hệ số mới chỉ vào khi có số đo PowerPoint; test riêng cho giá trị ở biên |
| 5 | `checkLayout` 5 loại lỗi | Chưa phát hiện lệch lưới giữa các slide, sát lề, tương phản | Bộ chấm benchmark đã có `titleDrift`, `marginViolations` (mềm); khi số đo ổn định mới đưa vào `checkLayout` | `test_score.py` (đã có); về sau thêm ca cố ý lệch trong `test_live_commands.py` |
| 6 | `look_at_document`: chỉ slide đang hiện | Không render được slide tuỳ ý; chưa có vòng sửa | P3: lệnh `wpp.renderSlide {slide}` dùng `Slide.Export`; ảnh lưu cùng benchmark | Ngoài phạm vi S0 |
| 7 | Primitive: text, bảng, ảnh, hộp có nền | Chưa có shape, connector, biểu đồ | P2: shape và connector **trong spec** (node, edge có ID); biểu đồ để sau | Ngoài phạm vi S0; brief `needs` đánh dấu sẵn 8 brief cần chúng |
| 8 | Không có bộ đo slide | Chưa có baseline | `tests/bench/slides/` (mục 4) | Một lệnh chạy cả bộ, ra bảng; baseline ghi vào mục 9 |

## 4. P0 - Bộ đo slide

### 4.1 Bộ brief (`tests/bench/slides/briefs.json`)

24 brief, phủ: bìa, chia mục, nội dung, so sánh, vấn đề–giải pháp, thẻ và ba con số, bảng (thường, rộng, dài),
biểu đồ, dòng thời gian, quy trình, chu trình, kiến trúc, sơ đồ tổ chức, nội dung dài cần tách slide, tiêu đề quá
dài, ảnh kèm chữ, trích dẫn, tóm tắt cho lãnh đạo, và một bài 6 slide.

- Mỗi brief **mang sẵn số liệu**. `mustContain` là những cụm phải xuất hiện trên slide; test bắt buộc mọi cụm
  đều có trong đề, để chấm bằng lệnh đọc chứ không cần LLM.
- `needs` đánh dấu khả năng bản hiện tại chưa có (`chart`, `shape`, `connector`). Baseline ghi lại agent xử lý
  thế nào (làm bảng thay biểu đồ, có nói rõ không). Đây không phải lỗi của brief.
- `smoke: true` đánh dấu 7 brief, là tập con chạy trước mỗi thay đổi lớn.

### 4.2 Chấm điểm (`score.py`)

| Loại | Chỉ số | Nguồn |
|---|---|---|
| **Cứng** (quyết định đạt) | Lượt kết thúc `run.completed`; số slide trong khoảng `slides`; `checkLayout` không còn lỗi; mọi cụm `mustContain` có trên slide | `run.json`, `check.json`, `slides.json` |
| Mềm (để so giữa các lần chạy) | `titleDrift` (tiêu đề lệch so với trung vị, pt), `marginViolations` (shape ra ngoài lưới 40 pt), `cleanSlideRatio`, `maxChars` | `check.json` |
| Chi phí | Số vòng, số tool call, số lệnh lỗi, token phải trả, thời gian | SSE của lượt |

Theo `17-r0` mục 5, **không dùng model thị giác để chấm đạt/không**. Ảnh và PDF mỗi lần chạy được lưu để người
xem lại. Điểm thị giác nếu có (P3) chỉ để tham khảo, có ngưỡng riêng và không bao giờ thay chỉ số cứng.

### 4.3 Chạy (`run.py`)

Mỗi lần chạy: `wpp.newPresentation` → `POST /v1/runs` (`interactive: true`, `maxSeconds` 900) → đọc SSE, **tự bấm
đồng ý** mọi `confirm.required` như người dùng bấm "Có" → `wpp.checkLayout` + `wpp.listSlides {detail}` → lưu
`deck.pptx` và `deck.pdf` (bỏ qua khi bài 0 slide). Kết quả ghi vào `results/<label>/<brief>/<lần>/` (thư mục này bị
`.gitignore`; bảng tổng hợp thì chép vào mục 9). Chữ trên slide được chấm từ `deck.pptx`, vì `wpp.listSlides` không
đọc chữ trong bảng (`19-h0` H3).

Bài học từ lượt chạy đầu (25/10/2026), đã đưa vào `run.py`:
- `interactive: false` làm Core **từ chối** mọi lệnh xoá, nên agent không sửa được lỗi của mình; đo như vậy là sai
  chất lượng. Giờ runner tự đồng ý và ghi các xác nhận vào `run.json` (`confirmations`).
- Máy ngủ (connected standby) giữa lượt làm mất request model và làm session bridge quá hạn: 21/21 lượt hỏng. Giờ
  runner giữ máy thức bằng `SetThreadExecutionState` (không đổi cấu hình nguồn).
- Không đóng bài tự động: `wpp.closeAll` thử thêm vào C# làm PowerPoint 2019 crash (`0xc0000409`) nên đã gỡ. Các bài
  đã lưu vẫn mở; chạy xong thì thoát PowerPoint.

Phần **chưa tự động** được:
- Mở PowerPoint, cài add-in, lấy port/pid bridge.
- Đánh giá khả năng chỉnh sửa: mở `deck.pptx`, xem shape là đối tượng gốc hay ảnh. Sau này đếm bằng `ppt_profile`
  hoặc đọc OOXML.
- Đếm "số lần sửa tay": người duyệt ghi vào `review.md` trong thư mục lần chạy.

Không đưa vào CI: cần PowerPoint thật và model thật. CI chạy `test_score.py` (job Linux) và test Go.

### 4.4 Cách chạy baseline

1. Build và cài: `scripts/build.ps1`, sau đó cài add-in như README. Cấu hình model qua wizard.
2. Mở PowerPoint (bridge PowerPoint ở cổng 47833). Lấy `pid` từ `GET http://127.0.0.1:47833/health`. Core ở cổng
   47840; nếu Core chưa chạy thì chạy `AxiomOffice.Core.exe` cạnh DLL. Token runner tự đọc từ HKCU.
3. `python tests/bench/slides/run.py --core-port 47840 --bridge-port 47833 --pid <pid> --label baseline-<model>-<ngày> --only smoke --repeat 3`. Chạy với ít nhất
   một model yếu (mimo hoặc deepseek flash) và một model mạnh làm mốc. Xong smoke thì chạy cả bộ.
4. `python tests/bench/slides/score.py tests/bench/slides/results/<label>` → chép bảng vào mục 9.

## 5. Thiết kế spec dùng chung

Ba tầng, mỗi tầng một kiểu dữ liệu:

```
SlideSpec (ý đồ: model viết)  →  Layout (hình học: solver tính)  →  []Op (lệnh bridge: renderer theo nền tảng)
```

### 5.1 `SlideSpec`: tầng ngữ nghĩa

**Giữ đúng tên trường của `composeRow`**, để lời gọi chế độ `compose` hiện có vẫn hợp lệ nguyên văn. Trường mới chỉ
được thêm, không đổi tên:

```jsonc
{
  "layout": "cards",              // từ vựng hiện có + mới: "figures" (ba con số), "quote"; P2: "process", "timeline", "diagram"
  "title": "Bốn nhiệm vụ trọng tâm quý IV",   // câu kết luận
  "subtitle": "",
  "points": ["…"],                // như composeRow: "**nhấn**", "- " cấp 2
  "cards": [{"title": "…", "points": ["…"]}],
  "table": [["…"]],
  "left": {"title": "…", "points": ["…"], "imgs": []}, "right": {…},
  "figures": [{"value": "128 tỷ", "label": "doanh thu, +12%"}],   // mới
  "imgs": ["7:i1" | "C:\\…\\anh.png"],   // tham chiếu nguồn (chế độ mẫu) hoặc đường dẫn (chế độ brief)
  "caption": "…", "callout": "…", "notes": "…", "sources": ["…"],
  "from": ["7:p2-4"],             // chỉ chế độ mẫu (kiểm phủ nguồn)
  // P2: "nodes": [{"id": "router", "text": "Bộ định tuyến", "group": "core"}], "edges": [{"from": "gateway", "to": "router", "label": "Yêu cầu"}]
}
```

- Spec **không có toạ độ**. Model không đặt `left/top/width/height` hay `fontSize`.
- Kiểm spec là `checkCompose` tách làm hai phần: kiểm từng slide (dùng chung) và kiểm phủ nguồn (chỉ chế độ mẫu).
  **Mã lỗi giữ nguyên**, vì model đọc chúng; chúng đã được ghim trong `TestCheckComposeErrorCodes`.
- Lý do chọn hướng này cho model yếu: viết JSON ý đồ với 8–10 kiểu cố định dễ hơn nhiều so với tính toạ độ cho
  từng hộp. Mã lỗi cụ thể (`NEED_2_TO_4_CARDS`) cũng dễ sửa hơn một `overflow` báo sau khi đã dựng.

### 5.2 `Layout`: tầng hình học

```go
// package slidelayout
type Frame struct {
    Width, Height float64   // cỡ slide (checkLayout hoặc tệp mẫu)
    Title   Rect            // ô tiêu đề
    Content Rect            // vùng nội dung (chế độ mẫu: contentArea(kind); chế độ brief: lưới tokens.json)
    Source  Rect            // dòng nguồn (rỗng = không có)
    Theme   Theme
}
type Theme struct {
    Font, Accent, Text, Muted string
    Title, Body, Card, Callout, Caption, Figure float64  // cỡ thiết kế
    MinBody float64          // sàn: không co chữ dưới mức này, quá thì fallback
    Shrink  bool             // chế độ mẫu: true (giữ hành vi cũ); chế độ brief: false
}
type Box struct {
    Role  string             // "title", "points", "card", "cell-grid", "picture", "caption", "callout", "figure", "source"
    Rect  Rect
    Size  float64
    Paras []Para
    Style Style              // box: "rounded"|"", fill, line, anchor
    Ref   string             // ảnh: tham chiếu nguồn hoặc đường dẫn
}
type Result struct {
    Slides   [][]Box         // > 1 slide khi fallback tách
    Fallback string          // "", "cards->table", "split"
    Findings []Finding       // mã lỗi như checkCompose
}
func Solve(spec SlideSpec, frame Frame, measure TextMeasurer) Result
```

- **Không phụ thuộc** `pptx.Kind`, bridge hay nền tảng. Chế độ mẫu tạo `Frame` từ `Kind`, chế độ brief tạo từ
  `tokens.json` và `slideWidth/slideHeight`.
- `TextMeasurer` mặc định giữ đúng `estimateLines`/`textFits` hiện tại, kể cả cách làm tròn. Đổi công thức là một
  thay đổi riêng, có số đo.
- **Chuỗi fallback** (chỉ chế độ brief, ở P1): `cards` quá dài → `table` nếu các thẻ cùng cấu trúc, không thì tách;
  `content` quá dài → hai slide, slide sau thêm "(tiếp theo)" (đúng quy tắc skill mục 3); `table` quá 8 dòng →
  tách theo dòng, lặp hàng tiêu đề. Không bao giờ co chữ dưới `MinBody`.

### 5.3 `[]Op`: tầng dựng

Renderer biến `Box` thành lệnh bridge **đã có**:

| Box | Windows (PowerPoint/WPS) | LibreOffice / WPS Linux |
|---|---|---|
| title, points, card, callout, figure | `wpp.addParagraphs` | `wpp.addText` (mất nền và viền thẻ; có báo giảm cấp) |
| cell-grid | `wpp.addGrid` | `wpp.addTable` |
| picture | `wpp.copyPicture` (ảnh nguồn) / `wpp.addImage` (đường dẫn) | `wpp.addImage` |

Năng lực của từng nền tảng đọc từ danh sách lệnh bridge trả về (`/commands`), không ghi cứng. Box không dựng được
thì renderer dùng bản đơn giản hơn và ghi vào kết quả cho model báo người dùng (skill mục 7: "việc không làm được").

## 6. Phụ thuộc của đường mẫu vào bridge (kiểm trước khi tách)

| Phụ thuộc | Hiện tại | Rủi ro khi tách solver |
|---|---|---|
| Cỡ slide, vùng nội dung | Đọc từ **tệp mẫu** qua `pptx.ReadTemplate` (`Kind.PictureBox`, `contentPad`), không hỏi app | Thấp: `Frame` lấy từ cùng nguồn |
| Đo chữ | Hằng số Go (`0.53`, `1.21`, `7.2`); phông lấy từ ô tiêu đề của mẫu (`accentOf`) nhưng hệ số vẫn theo Arial | Thấp nếu giữ nguyên công thức; đổi công thức thì phải đo lại |
| Danh tính shape | Tên shape của mẫu (`slot.Shape`) + thẻ slide (`tag`, `wpp.deleteTagged`) | Trung bình: renderer phải giữ đúng thứ tự `addSlideFromTemplate` → `setShapeText` → `deleteShapes` → thêm hộp |
| Xử lý lỗi | Bridge trả chuỗi `problem`; `buildComposed` dừng ở lỗi đầu, trả `index` slide dở | Trung bình: giữ đúng hợp đồng `(index, issues, problem)` |
| Trạng thái app | `ensureTarget`: `app.info` → bài đang mở là đích? → `wpp.open` hoặc `wpp.newFromTemplate` | Không đổi: nằm ngoài solver |
| Co chữ | `addParagraphs` (C#) co từ `fontSize` xuống `minFontSize` | Giữ `Theme.Shrink = true` cho chế độ mẫu |
| Soát sau dựng | `wpp.checkLayout {slide}` sau mỗi slide | Không đổi |

Tóm lại: phần hình học đã thuần (chỉ đọc `Kind` và hằng số), nên tách được mà không đụng bridge. Rủi ro nằm ở
**thứ tự và tham số lệnh bridge** do `buildComposed` phát ra, nên cần test golden cho chuỗi lệnh đó (M2 bên dưới).

## 7. Kế hoạch migration

| Bước | Việc | Không được đổi | Test giữ |
|---|---|---|---|
| **M0** (xong) | Test characterization: vùng, thẻ, ảnh, mã lỗi | - | `template_layout_test.go` |
| M1 | Bridge giả ghi lệnh; golden **chuỗi lệnh + tham số** của `buildComposed` cho 6 kiểu (cover, content có ảnh và lưu ý, cards 3 và 4, table, compare) | Mọi thứ | Golden mới + M0 |
| M2 | Chuyển hàm thuần sang `internal/slidelayout` nguyên văn; `tools` gọi qua | Số đo, mã lỗi, chuỗi lệnh | M0 + M1 chuyển theo, không sửa số |
| M3 | Thêm `Frame`/`Theme`/`Box`/`Solve`; `buildComposed` = `Solve` + renderer | Như M2 | M1 golden giống từng byte |
| M4 | Tool chế độ brief (mục 5): spec → `Frame` từ `tokens.json` → dựng; cập nhật skill: slide mới đi qua tool, `wpp.addText` chỉ để sửa lẻ | Chế độ mẫu | Test Go với bridge giả; benchmark smoke |
| M5 | Fallback chế độ brief; `figures`, `quote` | Chế độ mẫu | Test fallback; benchmark cả bộ |

**Rủi ro hồi quy chính:**
1. Đổi thứ tự lệnh bridge làm `deleteShapes` xoá nhầm hộp vừa thêm. M1 golden chặn rủi ro này.
2. Chế độ brief vô tình dùng `Theme` của chế độ mẫu (có co chữ). Cần test riêng cho từng chế độ.
3. Mô tả tool dài thêm làm model yếu đọc sai. Cần đo bằng benchmark (số vòng, lệnh lỗi), không đoán.

**Tệp dự kiến sửa (P1):** `core-go/internal/slidelayout/` (mới: `layout.go`, `fallback.go`, `measure.go` + test),
`core-go/internal/tools/template_compose.go`, `template_design.go`, `template_build.go` (gọi qua solver), một tool
mới cho chế độ brief trong `core-go/internal/tools/` và chỗ đăng ký tool, `skills/trinh-bay-chuyen-nghiep/SKILL.md`
và `references/bo-cuc-slide.md` (bỏ bảng toạ độ cho model), `docs/architecture/core.md` mục 8.4.6. C# và LibreOffice
**không đổi** ở P1.

## 8. Điều kiện bắt đầu P1 và tiêu chí xong P1

Bắt đầu P1 khi:
- Có **baseline thật**: smoke × 3 lần × (một model yếu + một model mạnh), bảng ghi ở mục 9.
- M0 và `test_score.py` xanh trong CI (đã xanh trên máy; CI chạy `go test ./...` và đã thêm `tests/bench/slides`).
- Thiết kế mục 5–7 được duyệt.

P1 xong khi:
- M1 golden không đổi qua M2–M3 (chế độ mẫu y như cũ).
- Benchmark smoke với model yếu: tỷ lệ đạt (chỉ số cứng) **không thấp hơn baseline** ở mọi brief, và tổng số lỗi
  `checkLayout` giảm. Mức cụ thể đặt sau khi có baseline, không đặt trước.
- Không lượt smoke nào vượt ngưỡng vòng/token của `17-r0`.

## 9. Kết quả đo

### 9.1 Baseline 25/10/2026: `ocg/deepseek-v4.1-flash`, smoke × 3

PowerPoint 2019 x64 (16.0.10373), commit gốc `209cf9a` cộng các thay đổi chưa commit của S0; skill
`trinh-bay-chuyen-nghiep` như trong repo. Đường **tự do** (model gọi `wpp.addText`/`wpp.addTable` theo skill).

| brief | đạt | lỗi `checkLayout` | thiếu số liệu | lệch tiêu đề (pt) | vòng TB | token phải trả TB | giây TB |
|---|---|---|---|---|---|---|---|
| s01-bia | 3/3 | 0 | 0 | 0 | 12,0 | 63 247 | 54 |
| s03-noi-dung-ngan | 3/3 | 0 | 0 | 0 | 14,3 | 86 229 | 51 |
| s05-so-sanh-hai-phuong-an | 3/3 | 0 | 0 | 0 | 13,7 | 69 848 | 56 |
| s07-ba-con-so | 3/3 | 0 | 0 | 0 | 13,0 | 67 954 | 37 |
| s09-bang-so-lieu | 2/2 (+1 lượt nhiễm, loại) | 0 | 0 | 0 | 13,0 | 66 548 | 57 |
| s13-dong-thoi-gian | 2/3 | 0 | 0 | 0 | 13,0 | 74 965 | 60 |
| s18-dai-tach-slide | 1/3 | 0 | 0 | 0 | 22,3 | 114 833 | 119 |
| **Tổng** | **17/20 lượt hợp lệ** | 0 | 0 | 0 | | | |

Đọc kết quả:
- **Một lượt bị nhiễm, đã loại** (`s09` lần 1). Bài mới tạo cho lượt không trở thành bài active, nên các lệnh không
  chọn bài (`wpp.addSlide`, rồi `wpp.saveAs` của runner) chạy trên bài của lượt trước (`s07` lần 1): file lưu có
  2 slide của hai brief, trong khi `listSlides` (có chọn bài) đọc bài đích 1 slide. Bộ chấm giờ tự loại lượt có số
  slide trong `.pptx` khác `listSlides` (`valid`). Bài đích còn lại được lưu thành `target-presentation33.pptx`.
  Đây là bằng chứng trực tiếp cho `19-h0` H1: chạy **tuần tự** cũng đã lẫn bài.
- **Chỉ số cứng gần như bão hoà** ở smoke: mọi lượt đều có `checkLayout` sạch, đủ số liệu, đúng lưới (tiêu đề cùng
  vị trí, không shape nào ra ngoài lề 40 pt). Skill hiện tại giữ được hình học cho slide đơn giản, nên smoke **không
  đủ để phân biệt** P1 tốt hơn hay kém hơn. Cần chạy cả bộ 24 brief (biểu đồ, quy trình, kiến trúc, nội dung dày),
  và cần thêm chỉ số chất lượng ngoài hình học.
- **Cả 3 lần trượt đều cùng một lỗi: thêm slide không được yêu cầu.** Agent tự thêm slide bìa khi brief xin "một
  slide" (`s13` lần 2) hoặc 2–4 slide (`s18` lần 1, 2 ra 5 slide). `s13` lần 2 còn thêm một slide nội dung **không có
  trong brief** ("năm việc cần chuẩn bị"): bịa nội dung. Đây là việc của skill hoặc spec (số slide là ràng buộc), không
  phải của layout.
- `s13` (dòng thời gian, cần `shape`): cả 3 lần agent làm **bảng** thay cho dòng thời gian. Hợp lệ theo skill mục 5,
  nhưng là lý do có P2.
- Chi phí: 9–26 vòng, 60k–149k token phải trả mỗi lượt. `s18` (tách nội dung dài) đắt gấp đôi và có lệnh lỗi
  (0, 1 và 4 lệnh lỗi ở ba lần).
- Xác nhận: 3/21 lượt cần xoá (`deleteShapes`, `deleteSlide`), tức agent sửa lại chính thứ mình vừa dựng (`19-h0` H4).

### 9.2 Lượt `interactive: false` (để đối chiếu, không phải baseline)

Cùng model, cùng ngày, 15 lượt chạy hết (`s01`–`s09`, chấm lại bằng chữ đọc từ `.pptx`, cả 15 hợp lệ): đạt 14/15. Lần trượt là `s03`
lần 3: **hai bảng giống hệt nhau chồng lên nhau** (`overlap`). Agent không thấy bảng của chính mình qua
`wpp.listSlides` nên chèn lại (`19-h0` H3). Lượt thử `s01` đầu tiên trượt vì slide trống thừa mà agent không được
phép xoá. `s13` hỏng do máy ngủ, không tính.

### 9.3 Việc tiếp theo cho bộ đo

1. Chạy một **model mạnh** làm mốc (smoke × 3).
2. Chạy **cả bộ** 24 brief × 3 với model yếu.
3. Thêm chỉ số **nội dung thừa**: chữ trên slide không truy được về brief (vd tỷ lệ số liệu hoặc cụm danh từ không có
   trong đề), để bắt kiểu `s13` lần 2.

## 10. Không làm trong S0

- Biểu đồ gốc, vòng render → sửa → render lại, lệnh render slide tuỳ ý (P2/P3).
- Engine OOXML dựng slide không cần app.
- Đổi hằng số typography của chế độ mẫu khi chưa có số đo.
