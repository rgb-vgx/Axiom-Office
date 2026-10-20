# Review bộ skill dựng sẵn (8 skill) và khoảng trống so với công việc văn phòng

Trạng thái: **P0 (§3) xong 11/10/2026, P1 (§4.1–§4.9) xong 12/10/2026** — P0 sửa §3.3 sinh thêm hai phát
hiện P1 (§4.7, §4.8), và P1 sửa §4.3 sinh thêm **§4.9** (`description` dài quá 300 ký tự bị cắt đuôi),
đã sửa trong cùng ngày sau khi chốt hướng. P2–P4 còn nguyên (xem §8). Cách làm: 4 lượt review độc lập qua
ChatGPT, rồi **đối chiếu máy móc lại với mã** — registry lệnh trong
[livecommands_gen.go](../../core-go/internal/mcpserver/livecommands_gen.go) (65 mục: 60 lệnh tài liệu — `writer`
22, `et` 23, `wpp` 15 — cộng 5 lệnh app/UI), mã bridge LibreOffice
trong [src/AxiomOffice.LibreOffice/python/pythonpath/axiom/](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom),
và tầng skill trong [core-go/internal/skills/](../../core-go/internal/skills).

Mọi phát hiện dưới đây đã kiểm lại trên mã. Chỗ nào chưa xác minh được thì ghi rõ **(chưa xác minh)** —
đừng đọc thành đã kiểm.

## 1. Kết luận ngắn

- Bộ 8 skill **đủ để chứng minh cơ chế nạp 3 tầng chạy đúng**; **chưa đủ** để phục vụ nhân viên văn phòng.
- Nút thắt không nằm ở số lượng skill, mà ở ba chỗ:
  1. vài skill đang dạy **thao tác sai hoặc phá dữ liệu** (§3);
  2. **trần của `office_action`** chặn nguyên những lớp việc phổ biến nhất (§5);
  3. **chưa có eval cho skill**, nên không có cách nào biết một skill làm model tốt lên hay không (§7).
- Thứ tự đúng: sửa P0 → bù lệnh bridge → viết eval → **rồi mới** viết skill mới.

Bối cảnh cần nhớ: bộ skill hiện tại là **skill mẫu**, không phải danh mục sản phẩm.
[docs/architecture/core.md:71](../architecture/core.md#L71) ghi rõ giai đoạn này gói NĐ30 "chỉ làm skill mẫu", và
[docs/architecture/core.md:566-579](../architecture/core.md#L566-L579) §8.4.6 thiết kế đúng 4 skill tầng thiết kế — tức 4/8 skill
hiện có là **đúng thiết kế**, không phải nghiệp vụ. Vì vậy đừng đọc bảng kiểm kê ở §2 như một danh mục
sản phẩm bị thiếu.

## 2. Kiểm kê

| Skill | apps | Vai trò | Sinh ra từ | Lệnh dùng |
|---|---|---|---|---|
| `thiet-ke-van-phong` | *(không có)* | Tầng thiết kế dùng chung | §8.4.6 | `read_skill_file`, `look_at_document` |
| `trinh-bay-chuyen-nghiep` | wpp | Tầng thiết kế slide | §8.4.6 | `wpp.checkLayout`, `wpp.addSlide`, `wpp.addText`, `wpp.addTable`, `wpp.setNotes` |
| `the-thuc-van-ban` | wps | Tầng thiết kế Word | §8.4.6 | `writer.heading`, `writer.insertTable`, `writer.formatTable`, `writer.setParagraphAlignment`, `writer.checkTables`, `writer.undo` |
| `bao-cao-du-lieu` | et | Tầng thiết kế Excel | §8.4.6 | `et.writeRange`, `et.formatRange`, `et.checkRange` |
| `bang-diem` | et | Nghiệp vụ: bảng điểm | yêu cầu mẫu | `et.writeRange`, `et.formatRange`, `et.checkRange` |
| `bao-cao-thang` | wpp | Nghiệp vụ: slide báo cáo tháng | yêu cầu mẫu | `wpp.addSlide`, `wpp.addText`, `wpp.addTable`, `wpp.setNotes`, `wpp.checkLayout` |
| `van-ban-hanh-chinh` | wps | Nghiệp vụ: NĐ 30/2020 | yêu cầu mẫu | `writer.insertTable`, `writer.formatTable`, `writer.insertStyledText`, `writer.getText`, `writer.checkTables` |
| `mo-hinh-nhieu-sheet` | et | Nghiệp vụ: mô hình nhiều sheet | **lỗi đo được trên benchmark** | `et.listSheets`, `et.readRange`, `et.writeRange`, `et.fillRange`, `et.setConditionalFormat`, `et.addChart`, `et.checkRange` |

Hai ghi chú kiểm chứng được:

- `thiet-ke-van-phong` **không có `apps:`**, và [index.go:38-48](../../core-go/internal/skills/index.go#L38-L48)
  cho `AppliesTo` trả `true` khi `Apps` rỗng → skill này có mặt trong cả ba app. **Đây là hành vi đúng**,
  không phải lỗi. Chỉ nên nói rõ trong `description` rằng nó dùng chung cho cả Word/Excel/PowerPoint.
- `mo-hinh-nhieu-sheet` là skill duy nhất viết từ **số đo thật** ([mo-hinh-nhieu-sheet/SKILL.md:16-17](../../skills/mo-hinh-nhieu-sheet/SKILL.md#L16-L17)
  trích `tests/bench/results/`). Đây là mẫu đúng cho mọi skill nghiệp vụ sau này.

Soát tự động: **23/23 lệnh được nhắc trong skill đều tồn tại trong registry** — không có tên lệnh bịa.
Vấn đề nằm ở **ngữ nghĩa**, không ở tên lệnh.

## 3. P0 — skill đang dạy thao tác sai hoặc phá dữ liệu — **đã sửa 11/10/2026**

### 3.1 `the-thuc-van-ban` bảo sửa một ô bằng `writer.replaceAll` — lệnh không làm được việc đó

[the-thuc-van-ban/SKILL.md:33-34](../../skills/the-thuc-van-ban/SKILL.md#L33-L34):

> `long-header-cell` (đoạn văn bị lọt vào ô tiêu đề: **sửa lại ô đó bằng `writer.replaceAll`**)

Nhưng [writer.py:541-567](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/writer.py#L541-L567):
`replace_all` tạo `document.createReplaceDescriptor()` rồi gọi `document.replaceAll(descriptor)` —
**toàn tài liệu**, không nhắm được bảng/ô, và còn đặt `SearchCaseSensitive = False`.

Hệ quả: model làm đúng theo skill sẽ thay **mọi** lần xuất hiện của chuỗi trong cả văn bản, không phân biệt
hoa/thường. Với văn bản hành chính — nơi tên cơ quan, "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM", chức vụ lặp lại
nhiều nơi — đây là lệnh phá tài liệu.

**Đã sửa:** bỏ hẳn `writer.replaceAll` khỏi skill ([the-thuc-van-ban/SKILL.md](../../skills/the-thuc-van-ban/SKILL.md)).
Thay bằng hai đường **làm được thật**: bảng do **chính lượt này** chèn và sau đó chưa làm gì khác thì
`writer.undo` lùi về trước `writer.insertTable` rồi chèn lại bằng `values` đúng; bảng có sẵn trong tài liệu,
hoặc đã làm việc khác sau đó, thì **báo người dùng**, đừng tự vá. Kèm theo, `UndoRule`
([office_action.go:23-32](../../core-go/internal/tools/office_action.go#L23-L32)) được nới đúng một ngoại lệ —
được `undo` thứ **chính lượt này** vừa làm khi không có cách sửa tại chỗ — nếu không thì prompt cấm đúng cái
đường duy nhất còn lại và dồn agent về `writer.replaceAll`. Lệnh `writer.setCell` vẫn nằm ở §5 (P2).

### 3.2 `bao-cao-du-lieu` bảo xoá `outside-table` bằng cách ghi `""` vào ô

[bao-cao-du-lieu/SKILL.md:39-41](../../skills/bao-cao-du-lieu/SKILL.md#L39-L41):

> `outside-table` (dữ liệu ghi lạc ô — **xoá bằng cách ghi `""` vào đúng các ô đó**)

Câu này mâu thuẫn trực tiếp với chính system prompt:
[prompt.go:48-57](../../core-go/internal/agent/prompt.go#L48-L57) — `CheckRule` nói *"Never change a cell
holding data the user already had, not even temporarily"*.

Vấn đề thật: `outside-table` là ô có dữ liệu **nằm ngoài vùng bảng** — mà ô đó rất thường là **dữ liệu
người dùng đã có sẵn** cạnh bảng. Skill đang dạy model xoá dữ liệu người dùng để làm `issueCount` về 0, tức
là **tối ưu chỉ số thay vì làm đúng việc**.

**Đã sửa:** [bao-cao-du-lieu/SKILL.md](../../skills/bao-cao-du-lieu/SKILL.md) nay tách `outside-table` ra khỏi
danh sách "sửa mọi `issues`": chỉ xoá ô mà **chính lượt này** đã ghi nhầm, không chắc thì báo người dùng, và
muốn soát đúng bảng vừa dựng thì truyền `range` (đã kiểm trong
[checks.py](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py): `outside-table` chỉ bắn khi
**không** truyền `range`). Yêu cầu tuyệt đối *"lặp đến khi `issueCount` = 0"* cũng bỏ — nó chính là chỗ biến
việc soát thành tối ưu chỉ số. Mô tả lệnh `et.checkRange` cũng nói rõ `outside-table` có thể là dữ liệu người
dùng đã có và **không** được xoá ô có sẵn từ trước.

### 3.3 Hai skill thiết kế để dùng cùng nhau lại mâu thuẫn về `empty-cells`

- [van-ban-hanh-chinh/SKILL.md:25-26](../../skills/van-ban-hanh-chinh/SKILL.md#L25-L26) dựng bảng dàn trang
  2 cột **có ô trống cố ý** (khoảng trống ký, ô chờ điền).
- [the-thuc-van-ban/SKILL.md:33](../../skills/the-thuc-van-ban/SKILL.md#L33) lại bảo:
  *"sửa `empty-cells` (điền hoặc ghi `—`)"*.
- [checks.py:66-67](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py#L66-L67) đếm ô trống
  cho **mọi** bảng, không có ngoại lệ cho bảng dàn trang.

Mà [van-ban-hanh-chinh/SKILL.md:15](../../skills/van-ban-hanh-chinh/SKILL.md#L15) nói rõ *"Dùng cùng skill
`the-thuc-van-ban`"*. Nên model làm xong văn bản NĐ30 sẽ tự nhét `—` vào ô ký. Hai skill đang chống nhau.

**Đã sửa — nhưng không theo cách đề xuất ở trên.** Cách "bỏ qua bảng không viền" **sai**, và kiểm trên
LibreOffice thật thì thấy sai ở **cả hai chiều**:

- bảng **dữ liệu** chưa định dạng cũng **không có nét kẻ nào** — đúng trường hợp hai live test đang kiểm
  (`writer.insertTable {values}` trần rồi đòi `checkTables` bắt `empty-cells`), nên "không nét kẻ" không suy ra
  được "bảng dàn trang"; bản vá thử theo hướng này làm **hỏng cả hai live test**;
- bảng dàn trang kiểu **biểu mẫu điền** thì **có** kẻ khung, nên chiều ngược lại cũng trượt.

Đoán theo cấu trúc cũng trượt: quy tắc "ô trống mà cả dòng lẫn cột đều trống thì là bố cục" trượt đúng ca cần
nhất — khoảng trống ký của NĐ30 nằm trong dòng **có** chữ (`"- Như trên;"`).

Nên dấu hiệu được chọn là **tem tường minh**: `formatTable {borders:false}` đặt tên bảng thành
`writer.LAYOUT_TABLE_NAME`, `checkTables` đọc tem đó và trả cờ `layout` cho từng bảng
([writer.py](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/writer.py),
[checks.py](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py)). Số ô trống **vẫn hiện** trong
report để agent thấy, chỉ không thành `issue`. Ba chi tiết đo được khi làm:

- tên bảng Writer **không được chứa dấu cách hay dấu chấm** (`unotbl.cxx:2958`) và **không được trùng**
  (`:2971`), nên `_rename_table` thử lần lượt `AxiomLayout`, `AxiomLayout-2`…; `formatTable` ghi `layout-table`
  vào `applied` để tem đặt hụt **hiện ra ở `skipped`** thay vì im lặng;
- tem theo `.odt` sang lần mở sau (đo: đặt `Name` → lưu → mở lại, tên còn nguyên);
- `long-header-cell` **vẫn báo cho mọi bảng**, kể cả bảng dàn trang — nó bắt chữ lọt vào ô tiêu đề, mà với
  bảng dàn trang thì đó cũng là lỗi thật, và `van-ban-hanh-chinh` vốn đòi soát nó.

## 4. P1 — mâu thuẫn, lệch token, description hứa quá năng lực — **đã sửa 12/10/2026**

Năm điểm cần người quyết đã chốt 12/10/2026 (§4.1 cách ánh xạ cỡ chữ, §4.2, §4.4, §4.7, §4.9 hướng
viết lại); phần còn lại của §4 sửa theo. Ghi chú từng mục ở dưới.

### 4.1 `references/bo-cuc.md` chép cứng cỡ chữ và màu, lệch với `tokens.json`

> **Cập nhật 13/10/2026:** file này **đã bị xoá** ở §4.10 — nó vẫn giữ bộ **toạ độ** thứ hai (960×540),
> nên bám token thôi chưa đủ. Các liên kết tới `bo-cuc.md` dưới đây nay là liên kết chết, giữ lại để đọc
> lịch sử; bố cục giờ ở `trinh-bay-chuyen-nghiep/references/bo-cuc-slide.md`.

[bao-cao-thang/references/bo-cuc.md:7](../../skills/bao-cao-thang/references/bo-cuc.md#L7) dùng
`fontSize: 40` cho slide tiêu đề, [dòng 12](../../skills/bao-cao-thang/references/bo-cuc.md#L12) dùng `32` cho
tiêu đề slide nội dung — nhưng
[skills/_design/tokens.json](../../skills/_design/tokens.json) đặt `slide.type.title = 36`, và
[trinh-bay-chuyen-nghiep/SKILL.md:33](../../skills/trinh-bay-chuyen-nghiep/SKILL.md#L33) bảo dùng đúng
`type.title`. File này cũng chép cứng `#1F4E79`, `#2E75B6`, `#595959`, `#262626` thay vì đọc token.

Hệ quả: hai skill dùng cùng nhau cho ra hai cỡ tiêu đề khác nhau, và **sửa `tokens.json` sẽ không lan tới
`bo-cuc.md`** — đúng cái mà §8.4.6 muốn tránh ("token thiết kế là dữ liệu").

**Sửa:** `bo-cuc.md` chỉ giữ **toạ độ và bố cục**, mọi cỡ chữ/màu ghi theo tên token (`type.title`,
`colors.primary`). Thêm test: mọi file `references/*.md` không được chứa `#RRGGBB` hay số cỡ chữ trần.

**Đã sửa 12/10/2026.**

- `bo-cuc.md` giờ chỉ còn toạ độ; cỡ chữ/màu ghi bằng tên token **đường dẫn đầy đủ từ gốc**
  (`slide.type.hero`, `colors.primary`) — ghi đầy đủ để test tra được thẳng vào `tokens.json`, không phải
  đoán khối nào.
- Ba cỡ chữ không có token đã được người dùng chốt: **thêm** `slide.type.hero` = 40 (chữ bìa) và
  `slide.type.figure` = 40 (con số lớn) vào `tokens.json`, giữ đúng kích thước đang thấy. Hai chỗ còn lại
  **không** thêm token mà gán về token có sẵn, vì đó mới đúng là chỗ hai skill lệch nhau:
  tiêu đề slide 32 → `slide.type.title` (36, khớp `trinh-bay-chuyen-nghiep`), nhãn dưới số 18 →
  `slide.type.body` (20). Đây là **thay đổi nhìn thấy được** của slide (tiêu đề 32→36, nhãn 18→20).
- Test mới [repo_skills_tokens_test.go](../../core-go/internal/tools/repo_skills_tokens_test.go) (chạy trong
  `go test ./...`): `references/*.md` không được có `#RRGGBB` hay `fontSize: <số>`, và mọi tên token được
  nhắc trong đó phải **có thật** trong `tokens.json`. Đã kiểm ngược: với `bo-cuc.md` cũ test **đỏ** đúng
  ba lý do (màu chép cứng, cỡ chữ chép cứng, không có tên token nào).
- Luật "cấm cỡ chữ trần" bắt **đúng dạng lệnh** `fontSize: <số>`, nên nó không đụng tới bảng cỡ chữ của
  `van-ban-hanh-chinh/references/the-thuc.md`: ở đó cỡ chữ là **quy định của NĐ30** (12–13, 13–14…), không
  phải token thiết kế. Luật `#RRGGBB` thì áp cho mọi file `references/*.md`.

### 4.2 Hai skill lệch nhau về số lần gọi `wpp.checkLayout`

[bao-cao-thang/SKILL.md:32](../../skills/bao-cao-thang/SKILL.md#L32): *"`wpp.checkLayout` cho **từng** slide mới"*.
[trinh-bay-chuyen-nghiep/SKILL.md:16](../../skills/trinh-bay-chuyen-nghiep/SKILL.md#L16): *"Gọi `wpp.checkLayout`
**một lần**"*.

`checkLayout` mặc định kiểm **mọi** slide, nên cách của `bao-cao-thang` là thừa lượt gọi — mà chính
`CheckRule`/`PlanRule` trong [prompt.go](../../core-go/internal/agent/prompt.go) đang cấm đọc thừa. Chốt một
cách: gọi một lần, không truyền `slide`.

**Chốt 12/10/2026 — gọi một lần, không truyền `slide`.** Đã sửa `bao-cao-thang/SKILL.md` theo
`trinh-bay-chuyen-nghiep`.

### 4.3 `description` của `bao-cao-du-lieu` hứa "độ rộng bảng" — không lệnh nào làm được

[bao-cao-du-lieu/SKILL.md:4-5](../../skills/bao-cao-du-lieu/SKILL.md#L4-L5) hứa *"mật độ và **độ rộng bảng**"*.
`et.formatRange` chỉ nhận `range, bold, italic, fontSize, fontColor, fillColor, numFmt, horizontal, wrap,
sheet` — **không có width/height**, và quét cả registry không có lệnh nào đặt độ rộng cột.

Đây là lỗi nặng hơn nó trông: `description` **chính là router** (tầng 1 — model chỉ thấy dòng này để quyết
định nạp skill). Một description hứa năng lực không tồn tại sẽ khiến model nạp skill rồi bị kẹt.

**Sửa:** bỏ "độ rộng bảng" khỏi description cho tới khi có lệnh đặt width.

**Đã sửa 12/10/2026:** description còn "…công thức thay cho số tính tay, **mật độ trình bày**". Chưa có
lệnh nào đặt độ rộng cột, nên lời hứa đó chưa được phép quay lại (§5, dòng `et`).

### 4.4 Cho hai lựa chọn thay vì một mặc định

[van-ban-hanh-chinh/references/the-thuc.md:18](../../skills/van-ban-hanh-chinh/references/the-thuc.md#L18) ghi
cỡ nội dung `13–14`, [SKILL.md:32](../../skills/van-ban-hanh-chinh/SKILL.md#L32) dùng `size: 14`, còn
`tokens.json` đặt `document.type.body = 13`. [docs/architecture/core.md:563-564](../architecture/core.md#L563-L564) §8.4.5 đã đặt
luật: *"nêu một cách mặc định thay vì liệt kê nhiều lựa chọn"*. Chốt một số.

**Chốt 12/10/2026 — 13, theo token.** `document.type.body` giữ nguyên 13 (không đổi token, vì
`the-thuc-van-ban` đã dùng `type.body` cho mọi tài liệu Word); đã sửa `van-ban-hanh-chinh/SKILL.md`
(`size: 14` → `13`, "13–14" → "`type.body` (13)") và `references/the-thuc.md` (Nội dung: 13–14 → 13).
Các dòng khác của `the-thuc.md` **vẫn để khoảng** (quốc hiệu 12–13, tiêu ngữ 13–14…): đó là khoảng của
NĐ30, không phải token, và chốt chúng sẽ là bịa số.

### 4.5 `thiet-ke-van-phong` dạy "viết spec ngắn **trong đầu**"

[thiet-ke-van-phong/SKILL.md:34](../../skills/thiet-ke-van-phong/SKILL.md#L34). Chỉ dẫn này không kiểm chứng
được và không ràng buộc hành vi. Cách `mo-hinh-nhieu-sheet` làm mới đúng: bắt **viết plan ra** rồi bám theo
(quy tắc 9 của skill đó, và `PlanRule` trong `prompt.go`).

**Đã sửa 12/10/2026:** quy trình giờ đòi **viết spec ra** trước lệnh đầu tiên, kèm lý do ("nghĩ thầm thì
không có gì ràng buộc các lệnh sau").

### 4.6 NĐ30: một quy tắc chưa đối chiếu được bản gốc **(chưa xác minh)**

[the-thuc.md:26](../../skills/van-ban-hanh-chinh/references/the-thuc.md#L26): *"Ngày dưới 10 và tháng 1, 2 thêm
số 0"*. Lượt review qua ChatGPT cho rằng quy tắc này **không có** trong Phụ lục I NĐ 30/2020. Tôi **không
xác minh được**: WebSearch trả kết quả rỗng và thuvienphapluat.vn chặn `403 Forbidden`.

Đây có thể là **thông lệ** soạn thảo chứ không phải quy định của nghị định. **Cần đối chiếu bản gốc Phụ lục I
trước khi giữ câu này.** Ngược lại, mục *"Nơi nhận:"* cỡ 12 nghiêng đậm và danh sách nơi nhận cỡ 11
([the-thuc.md:21-22](../../skills/van-ban-hanh-chinh/references/the-thuc.md#L21-L22)) thì tôi cho là **đúng** —
ChatGPT phân vân chỗ này nhưng không đưa được căn cứ.

### 4.7 `long-header-cell` **không thể** bắn với bảng 2 cột — mà `van-ban-hanh-chinh` chỉ dựng bảng 2 cột

Phát hiện khi sửa §3.3. [checks.py:74-81](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py#L74-L81):
`typical = lengths[len(lengths) // 2]` lấy **trung vị của chính các ô tiêu đề**, rồi đòi
`len(text) > 40` **và** `len(text) > 3 * typical`.

Với bảng **đúng 2 cột**, `sorted([a, b])[1]` chính là giá trị **lớn hơn** — nên ô dài nhất có
`len(text) == typical`, và `typical > 3 * typical` luôn **sai**. Đã kiểm bằng mã (không cần LibreOffice):
bảng 2 cột với một đoạn văn 57 ký tự gõ lọt vào ô tiêu đề → `long-header-cell` **không bắn**. Đây là
**bất khả thi về số học**, không phải ngưỡng đặt cao.

Hệ quả trực tiếp: [van-ban-hanh-chinh/SKILL.md:44-45](../../skills/van-ban-hanh-chinh/SKILL.md#L44-L45) đòi
*"không được có `long-header-cell` do gõ lọt chữ"*, nhưng cả hai bảng của skill này đều **2 cột**
([SKILL.md:19-20, 25-26](../../skills/van-ban-hanh-chinh/SKILL.md#L19-L26)). Một điều kiện **không bao giờ sai**
thì không phải là kiểm tra — nó chỉ làm checklist trông chặt hơn thực tế, và đúng cái lỗi nó sinh ra để bắt
(chữ lọt vào ô tiêu đề) thì nó không bắt được.

**Đã chốt 12/10/2026 — so với trung vị các ô tiêu đề CÒN LẠI (leave-one-out).** Chỗ hỏng là **nhóm so
sánh có chứa chính ô đang xét**: bỏ nó ra khỏi nhóm thì phép toán hết suy biến mà không phải nới con số
nào (`LONG_HEADER_CHARS` vẫn 40, tỉ lệ vẫn 3×):

| ô tiêu đề | trung vị còn lại | kết quả |
|---|---|---|
| 2 cột `[10, 57]` | `[10]` → 10 | 57 > 40 và 57 > 30 → **báo** |
| 2 cột `[12, 34]` (quốc hiệu NĐ30) | `[12]` → 12 | 34 > 40 **sai** → không báo oan |
| 3 cột, mọi tiêu đề đều dài | ~50 | 55 > 150 sai → không báo oan |

Bảng **1 cột** vẫn không bao giờ báo (không có nhóm để so — ghi rõ trong mã, không phải im lặng).
Đã sửa [checks.py](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py) và thêm 4 test vào
`tests/lo/test_checks.py`; đã **kiểm ngược**: test "bảng 2 cột có đoạn văn lọt vào ô tiêu đề" **đỏ** với
luật cũ (`'long-header-cell' not found`), xanh với luật mới — tức lời hứa "không được có
`long-header-cell`" của `van-ban-hanh-chinh` (SKILL.md:44-45) nay mới thật sự kiểm được với hai bảng 2 cột
của chính nó.

**Ba hướng đã cân và loại** (ghi lại để đừng thử lại):

- **trung vị mọi ô trong bảng** thay vì ô tiêu đề: bắt thêm được vài ca (bảng "Nơi nhận:" 2 cột) nhưng
  **vẫn trượt** bảng phần đầu NĐ30 — ô `"CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM"` (34 ký tự) là chữ **hợp lệ**
  mà lại kéo trung vị lên;
- **nới ngưỡng** (bỏ tỉ lệ khi ≤2 cột, hoặc hạ `LONG_HEADER_CHARS`): đổi hành vi soát cho **mọi** bảng và
  bắt đầu báo oan những bảng 2 cột có tiêu đề dài hợp lệ;
- **bỏ yêu cầu khỏi checklist** `van-ban-hanh-chinh`, dựa vào `writer.getText` đọc lại: rẻ và không sai,
  nhưng vứt mất một kiểm tra thật. Leave-one-out giữ được cả hai.

### 4.8 `tokens.json` có một token không ai đọc: `plainTableStyle`

[skills/_design/tokens.json:32](../../skills/_design/tokens.json#L32) đặt `document.plainTableStyle = "Table Grid"`.
Quét cả kho: **không file `.md`, `.go`, `.py` nào nhắc tới `plainTableStyle`**. Token chết.

Khác `document.tableStyle` (dòng 31, được
[the-thuc-van-ban/SKILL.md:24](../../skills/the-thuc-van-ban/SKILL.md#L24) dùng thật, và
[writer.py:28-31](../../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/writer.py#L28-L31) dịch tên style Word
sang autoformat LibreOffice), `plainTableStyle` không có đường nào tới tài liệu.

Đây là cùng một loại lỗi với §4.1 — **token là dữ liệu, nhưng skill không đọc hết dữ liệu** — chỉ khác là
§4.1 đọc sai (chép cứng), còn §4.8 là không đọc (chết hẳn). Lưu ý thêm: giá trị `"Table Grid"` **sẽ** chạy
được nếu có ai gọi (`"table grid"` khớp nhánh `"grid" in lower` → `"Default Table Style"`), nên đây là token
**thừa**, không phải token **sai**. Sửa: hoặc dùng nó cho `writer.formatTable {style}` của bảng dữ liệu, hoặc
xoá khỏi `tokens.json`. Xoá là lựa chọn rẻ hơn cho tới khi có nhu cầu thật.

**Đã sửa 12/10/2026 — xoá.** `document.plainTableStyle` không còn trong `tokens.json`. `document.tableStyle`
giữ nguyên (có người dùng thật). Khi nào có lệnh đặt style cho bảng dữ liệu thì thêm lại kèm chỗ đọc.

### 4.9 **MỚI 12/10/2026** — 5/8 `description` dài hơn 300 ký tự nên phần đuôi **không tới model** — **đã sửa 12/10/2026**

Phát hiện khi sửa §4.3. Tầng 1 chỉ đưa `name` + `description` vào system prompt, và
[prompt.go:107](../../core-go/internal/agent/prompt.go#L107) cắt ở **300 rune**:

```go
lines = append(lines, "- "+skill.Name+": "+flatten(skill.Description, skillDescriptionLimit))
```

Chỗ đo là **chuỗi một dòng mà router thật sự thấy** — `flatten` gộp mọi xuống dòng thành dấu cách rồi mới
đếm, nên đo khối `description:` trong YAML sẽ lệch vài chục ký tự. Trước khi sửa:

| skill | dài | phần bị cắt |
|---|---|---|
| `mo-hinh-nhieu-sheet` | **665** | **toàn bộ 12 cụm "Dùng cho …"** — "mô hình tài chính", "FP&A", "ngân sách", "P&L", "dự báo", "dashboard KPI"… **không cụm nào tới model** |
| `van-ban-hanh-chinh` | 339 | "thông báo", "văn bản hành chính", "giấy mời" |
| `thiet-ke-van-phong` | 348 | "Không dùng cho sửa nhỏ như in đậm một dòng hay sửa một con số" |
| `the-thuc-van-ban` | 338 | "Không dùng cho sửa một câu hay in đậm một từ" |
| `trinh-bay-chuyen-nghiep` | 305 | "chữ" (cụt) |
| `bang-diem` / `bao-cao-du-lieu` / `bao-cao-thang` | 265 / 282 / 253 | không bị cắt |

Đây **cùng loại lỗi với §4.3** nhưng nặng hơn: `mo-hinh-nhieu-sheet` mất **cả** danh sách từ khoá dùng để
được nạp — nó gần như không thể định tuyến tới, trừ khi câu người dùng chứa đúng "mô hình tài" ở ngay mốc
300. Hai skill bị cắt phần **"Không dùng khi…"** thì rủi ro ngược lại: model nạp skill thiết kế cho một yêu
cầu sửa nhỏ, đúng ca mà [test_core_e2e.py:1241](../../tests/core/test_core_e2e.py#L1241) đang canh (`forbidden`).

**Đã sửa 12/10/2026 — từ khoá lên trước.** Mỗi description viết lại theo thứ tự: một câu định vị ngắn → danh
sách "Dùng khi …" **nằm gọn trong 300** → "Không dùng khi …" ngắn. Phần mô tả năng lực bị bỏ khỏi
description đều **đã có ở thân SKILL.md** (tầng 2, model đọc ngay sau `load_skill`); chỗ duy nhất thiếu thì
bù vào thân — `the-thuc-van-ban` thêm quy tắc 5 (`writer.insertPageBreak`) vì description còn nhắc "ngắt
trang".

| skill | trước | sau | bỏ/rút gì khỏi description |
|---|---|---|---|
| `mo-hinh-nhieu-sheet` | 665 | 291 | danh sách 5 loại sheet ở câu đầu (thân có ở quy tắc 1); các cụm từ khoá dài rút lại: "phân tích độ nhạy" → "độ nhạy", "quản lý tiến độ - nguồn lực - chi phí" → "tiến độ dự án" |
| `van-ban-hanh-chinh` | 339 | 284 | 4 thành phần thể thức khỏi câu đầu (thân vẫn liệt kê đủ ở mục Bố cục) |
| `thiet-ke-van-phong` | 348 | 287 | "Không dùng cho sửa nhỏ như in đậm một dòng hay sửa một con số" → "Không dùng cho sửa nhỏ" |
| `the-thuc-van-ban` | 338 | 286 | "Không dùng cho sửa một câu hay in đậm một từ" → "Không sửa một câu" |
| `trinh-bay-chuyen-nghiep` | 305 | 295 | viết gọn lại, không mất ý ("khi nào dùng bảng" → "bảng khi cần so sánh") |
| `bang-diem` / `bao-cao-du-lieu` / `bao-cao-thang` | 265 / 282 / 253 | không đổi | |

**Test chặn** (giờ mới thêm được): trần 300 có tên — `skillDescriptionLimit`
([prompt.go:98](../../core-go/internal/agent/prompt.go#L98)) — và
[TestRepoSkillDescriptionsFitRouterLimit](../../core-go/internal/agent/skills_router_test.go) đọc **skill thật
trong repo**, đo bằng **chính `flatten` của router** (không đo lại bằng tay), in bảng `dài/300` khi chạy
`-v`. Test nằm ở package `agent` chứ không phải `tools` vì phải gọi `flatten` + hằng số của chính router. Đã
chạy khi **chưa** sửa: đỏ đúng 5 skill (665/348/339/338/305) — test bắt đúng lỗi, rồi xanh sau khi sửa.

Hướng đã chọn: **giữ từ khoá, hạ văn xuôi** (hướng 1), ghép một phần hướng 2 — rút chữ trong cụm cho gọn
chứ **không bỏ cụm nào** chỉ vì ít gặp, vì bỏ cụm là bỏ đường định tuyến. Hướng 3 (nâng trần 300) không
chọn: tốn token mỗi lượt cho **mọi** skill, và bỏ mất con số đang canh.

### 4.10 **MỚI 13/10/2026** — Skill làm slide dựng lưới trên một slide **không tồn tại**, và lưới bị chép hai nơi — **đã sửa 13/10/2026**

Phát hiện khi làm đợt "cải tạo skill làm slide" (yêu cầu người dùng). Đây là cùng **loại** lỗi với §4.1 —
dữ liệu thiết kế nằm ở hai chỗ và một chỗ sai — nhưng lần này cái sai **tự sinh lỗi**, không chỉ lệch nhau.

**Bằng chứng.** `trinh-bay-chuyen-nghiep/SKILL.md` viết *"16:9 thường 960×540"*. 960×540 là mặc định của
**PowerPoint**; Impress mặc định là **793.7 × 446.5pt** (28 × 15.75cm) — nhỏ hơn **0.827 lần** theo chiều
dài. Đo trên LibreOffice thật (màn hình ảo `xvfb-run`; `--headless` cho số đo chiều cao chữ sai).

**Vì sao đây là lỗi chứ không phải chuyện thẩm mỹ.** Ba hằng số đo được: mỗi dòng chữ cao **1.12 × cỡ chữ**,
hộp chữ có **7.1pt lề trong**, và lưới cũ đặt `slide.type.title` = 36 trong hộp cao **60pt**. Một tiêu đề
2 dòng cần `7.1 + 2 × 1.12 × 36` = **87.7pt** — gấp rưỡi chiều cao hộp. Mà skill lại **bắt buộc** tiêu đề là
câu kết luận ≤ 2 dòng ("Doanh thu tăng 12% nhờ kênh trực tuyến"). Tức **làm đúng hướng dẫn là chắc chắn
`overflow`**, và lưới cũ còn không có chỗ cho dòng nguồn. Lỗi nằm ở **cỡ chữ lấy từ deck 960×540 rồi bê
nguyên sang slide nhỏ hơn 0.83 lần**: cùng số point, chữ to hơn 0.83 lần so với dự kiến.

**Lưới chép hai nơi.** `bao-cao-thang/references/bo-cuc.md` giữ bộ toạ độ **thứ hai** (960×540, 880, 920…)
cho cùng loại slide mà `trinh-bay-chuyen-nghiep` dựng — hai skill được thiết kế để **dùng cùng nhau**. Đây
là biến thể tệ hơn §4.1: chép cứng thì lệch khi `tokens.json` đổi, còn chép cứng **toạ độ** thì hai skill
nói hai chuyện khác nhau ngay từ đầu. Sửa: **xoá** `bo-cuc.md`; toạ độ chỉ còn **một** nơi; `bao-cao-thang`
chỉ nói *viết gì* (`references/noi-dung-slide.md`), vị trí thì trỏ về skill chung.

**Sửa:**
- `slide.type.title` **36 → 28** (= 36 × 0.827, tương đương thị giác với 36pt trên deck PowerPoint); hộp tiêu
  đề 60 → **80pt** (top 30), vùng nội dung top 110 → **130** cao `H − 170`, thêm **dòng nguồn** ở `H − 36`.
  Đây là **thay đổi nhìn thấy được** của slide do skill dựng ra
- Skill nay **đọc `slideWidth`/`slideHeight` từ `wpp.checkLayout` rồi mới tính lưới**, thay vì tin vào một
  con số viết sẵn — sửa tận gốc, không chỉ sửa 960 thành 793.7
- **Sức chứa tính trước khi viết chữ**: `chiều cao = 7.1 + số_dòng × 1.12 × cỡ_chữ`. Trần thật của một slide
  là **600 ký tự** (`dense` của `checks.py`), không phải sức chứa hình học (~780) — và 600 này **khớp** với
  `slide.maxBulletsPerSlide` × `slide.maxWordsPerBullet` trong `tokens.json`
- **Ghi rõ giới hạn thật của wpp** thay vì hứa suông: `addText` không có phông chữ / chữ nghiêng / gạch đầu
  dòng / giãn dòng, và cỡ chữ–đậm–màu áp cho **cả hộp** (không in đậm riêng một con số trong câu); không có
  lệnh biểu đồ, hình khối, sơ đồ, tô nền; `addTable` không nhận cỡ chữ hay màu ô; và **`checkLayout` không
  đọc được chữ trong bảng** — nên trạng thái "Đạt/Chậm" **phải ghi bằng chữ**, không thể trông vào màu ô.
  Đây là lỗi cùng họ với §4.3 ("hứa năng lực không tồn tại")
- **Ba ngưỡng tương phản tính từ `tokens.json`** (công thức WCAG 2.1, ghi lại để tính lại được):
  `colors.warning` **4.33:1** không đạt AA cho chữ thường; `colors.accent` 4.84:1 nhưng chỉ **4.33:1** trên
  `colors.surface`, còn `colors.success` **4.06:1** trên `surface` — đều tụt xuống mức "chỉ chữ lớn";
  `colors.muted` 7.00:1 vừa AAA trên nền trắng, trên `surface` còn AA. Ràng buộc kèm theo: **màu không được
  là dấu hiệu duy nhất**

**Đo lại được, không phải niềm tin.** Thêm `tests/lo/measure_slide_grid.py` — **không** phải unittest (không
có tiền tố `test_` nên `make test-lo` không gom): dựng đúng lưới trong SKILL.md lên slide thật, in từng hộp
*cần bao nhiêu / có bao nhiêu*, rồi hỏi `checkLayout`. Lệch một con số là in `LỆCH:` + trả mã 1, nên lần sau
LibreOffice đổi cách dàn chữ thì script đỏ **trước** khi người dùng thấy slide xấu.

| Đo | Kết quả |
|---|---|
| Slide đầy nhất dựng đúng lưới (65 + 379 + 61 = **501** ký tự) | `checkLayout` **issueCount=0**, không lỗi nào |
| Tiêu đề 2 dòng 28pt cần / hộp có | 69.8 / 80pt ✅ |
| Nội dung 10 dòng 20pt cần / hộp có | 231.1 / 276.5pt ✅ |
| Nguồn 1 dòng 14pt cần / hộp có | 22.8 / 24pt ✅ |

**Một chỗ số đo buộc phải nói "không biết".** Bề ngang ký tự **không có hằng số**: dò nhị phân chuỗi dài
nhất còn vừa một dòng ra 75 ký tự ở 20pt và 53 ở 28pt (0.48 × cỡ chữ) với chữ viết liền, nhưng một mẫu văn
xuôi thật có dấu chỉ được **55.5 ký tự/dòng** ở 20pt (0.64) vì chỗ ngắt từ ăn mất phần dư. Nên 0.55 là ước
lượng **ở giữa**, sai số **±20%** (bản cũ ghi ±15%) — và bài viết nay nói rõ: **chừa một dòng dư**, đừng
thiết kế sát trần. Ba thứ **chưa đo được** thì ghi thẳng vào mục "Cái chưa đo được — đừng bịa" của
`references/co-so-do-duoc.md` (chiều cao hàng bảng; bảng vô hình với `checkLayout`; trần 900pt của
`_shape_text_height`) thay vì đoán thành quy tắc.

**Còn treo.** `colors.warning` không đạt AA cho chữ thường là chuyện của **bảng màu**, không phải của skill:
sửa được thì phải sửa `tokens.json`, mà màu đó dùng chung với Word/Excel và có thể là màu nhận diện của tổ
chức — **cần người quyết**, không tự đổi. Hiện xử lý bằng cách ràng buộc cách dùng (chỉ cỡ lớn/in đậm).

### 4.11 **MỚI 13/10/2026** — Skill làm slide trích **"chuẩn" không tồn tại**: 13 khẳng định không có nguồn — **đã sửa 13/10/2026**

Cùng họ với §4.3 ("hứa năng lực không tồn tại") và §4.10, nhưng lần này cái không tồn tại là **nguồn**:
`references/cau-chuyen.md` ghi nhiều con số với giọng "theo chuẩn", mà đợt kiểm chứng không mở được trang
nào nói đúng chúng.

**Cách kiểm (quan trọng, vì nó quyết định kết quả).** `WebSearch` **trả về rỗng với mọi truy vấn** trong môi
trường này, và các máy tìm kiếm thay thế bị CAPTCHA. Nên đợt kiểm chứng chạy bằng **`WebFetch` trực tiếp**:
đoán URL đã biết rồi mở. Hệ quả phải nói thẳng: tìm được trang **đã biết đường dẫn**, **không** tìm được
trang chưa biết. Vì vậy "không tìm thấy nguồn" **không đồng nghĩa với "khẳng định sai"** — nó nghĩa là
"chưa kiểm chứng được", và file ghi đúng như vậy.

**Kết quả: 9 khẳng định mở được trang thật, 13 khẳng định phải hạ xuống "chưa có nguồn".**

| Kiểm chứng được (có URL) | Hạ xuống "chưa kiểm chứng" |
|---|---|
| Microsoft: chữ thường **≥ 18pt** · Guy Kawasaki: **không nhỏ hơn 30pt** · Bảng khoảng cách Paradi (24pt/32" → 15 ft) | Indezine 24pt — *và một bài của chính Indezine nói 30pt* · ngưỡng Kosslyn (> 4 gạch / > 2 dòng / < 20pt) |
| PresentationLoad **6–10 từ** · Slideworks/McKinsey **< 15 từ, ≤ 2 dòng** · định nghĩa assertion–evidence | quy tắc "6x6" · số liệu thí nghiệm Mayer · chỉ số t/d của Garner & Alley 2013 (trang nguồn chỉ có "p < .01") |
| WCAG 2.1 SC 1.4.3 (**4.5:1** / **3:1**, chữ lớn = **≥ 18pt hoặc ≥ 14pt đậm**) · SC 1.4.1 (màu không là dấu hiệu duy nhất) | "action safe 90%", lề 1/20, lưới 12 cột, nhịp 8pt/4pt · tỉ lệ màu 60-30-10 · cam/vàng trượt tương phản |
| Okabe & Ito: bảng màu + tỉ lệ mù màu (1/12 gốc Âu = 8%, 1/20 gốc Á = 5%, 1/25 gốc Phi = 4%) | halation (trang Wikipedia nói về **ảnh phim**, không nói về chữ trên slide) · biểu đồ tròn ≤ 5 phần · Tufte phản đối PowerPoint |
| Stephen Few về **hai trục tung** | "máy chiếu làm màu nhạt đi 20–30%" · mọi con số "số slide" |

**Vài chỗ đáng nhớ vì nguồn được gán còn phản lại khẳng định**: "Indezine nói tối thiểu 24pt" — không có
trang Indezine nào nói 24pt, và một bài của họ nói 30pt; "halation" bị trích từ một trang nói về hiện tượng
trong **phim**, không phải màn hình; chỉ số thống kê hay được trích của Garner & Alley 2013
(t(93.43) = 3.67, d = 0.81, 21,2 vs 41,5 từ/slide) **không có** trên trang của chính nhóm tác giả đó.

**Vì sao đây là lỗi thật, không phải chuyện câu chữ.** Skill là thứ agent đọc rồi **nói lại với người
dùng**. Một con số được gán cho "chuẩn X" mà X không nói thế thì dạy agent **nói giọng chắc chắn về điều
mình không biết** — đúng thứ mà cả tài liệu này đang chống. Cái sai không nằm ở con số (nhiều con số vẫn
hợp lý) mà ở **thẩm quyền được gán**: 60–30–10 nghe như định luật, thực ra chỉ là thực hành phổ biến.

**Sửa:**
- `references/cau-chuyen.md` viết lại: mỗi khẳng định có URL **chỉ khi trang đó đã mở thật**, và mục 10 liệt
  kê đủ 13 mục "chưa kiểm chứng" kèm lý do từng mục (không mở được / mở được nhưng không khớp / paywall)
- `references/co-so-do-duoc.md`: bỏ con số **"máy chiếu làm màu nhạt đi 20–30%"** (giữ lời khuyên "chừa dư
  tương phản", bỏ con số); "**≈ 8% nam giới**" → **5–8%** kèm tỉ lệ theo nhóm gốc
- `SKILL.md` mục 2 nay phân biệt **sàn kỹ thuật** (`slide.type.minimum` = 12, ngưỡng `small-font` của
  `checkLayout`) với **sàn trợ năng** (**18pt**, theo Microsoft): chữ để đọc dùng `slide.type.title` /
  `slide.type.body` (28/20 — trên cả hai sàn), **muốn vừa thì cắt chữ đừng hạ cỡ**; chỉ dòng phụ
  (`slide.type.caption` 14pt) được dưới 18pt, và khi đó nó là **chữ thường** theo WCAG nên phải đủ 4.5:1 →
  chỉ `colors.text` / `colors.muted`. Hai sự kiện rời nhau (cỡ chữ, tương phản) ghép thành một quy tắc dùng được
- `SKILL.md` mục 3 nêu nguồn cho trần tiêu đề ≤ 15 từ / ≤ 2 dòng, và nói rõ nó **trùng** trần kỹ thuật của
  lưới (2 dòng ở 28pt ≈ 92 ký tự ≈ 15 từ) — trước đó con số này không có xuất xứ

**Còn treo.** Đợt này **không kết luận được cái gì là sai**, chỉ kết luận được cái gì **chưa có nguồn**.
Nếu sau này có công cụ tìm kiếm chạy được, việc nên làm là **đi tìm nguồn cho 13 mục ở mục 10 và thăng cấp
mục nào tìm được** — nhất là Kosslyn (nhiều khả năng có trong sách, chỉ không có trên web) và 60-30-10
(nghe như bắt nguồn từ thiết kế nội thất, chưa xác minh). Trước khi tìm được thì mục 10 giữ nguyên là "chưa
kiểm chứng", không nâng thành "sai".

## 5. Trần công cụ — phần chặn nhiều việc nhất

Quét registry: `header|footer|toc|width|sort|pivot|merge|freeze|margin` **không khớp lệnh nào**. Danh sách
dưới đây đã đối chiếu từng tham số, không phải phỏng đoán.

| App | Thiếu | Chặn việc gì |
|---|---|---|
| **writer** | header/footer, số trang, mục lục, ngắt section, page setup (lề/khổ/chiều), chèn–xoá dòng/cột bảng, gộp ô, sửa **một ô** trong bảng, tìm-không-thay, footnote, comment/track changes | Báo cáo dài, hợp đồng, mọi tài liệu cần lề chuẩn và số trang; sửa bảng có sẵn |
| **et** | độ rộng cột / chiều cao dòng, border, sort, filter, pivot, chèn–xoá dòng/cột, freeze panes, page setup, xoá/đổi tên sheet có kiểm soát, gộp ô, copy/clear vùng | Bảng in được, làm sạch–sắp xếp dữ liệu, mọi báo cáo có kẻ khung |
| **wpp** | sửa chữ của shape **có sẵn**, chart, shape, đổi layout slide đã có, xoá shape, di chuyển/resize | Sửa bài thuyết trình có sẵn — hiện chỉ **thêm** được, không **sửa** được |

Điểm chung đáng chú ý: **gần như mọi lệnh hiện có đều là "thêm mới"**. Đó là lý do gốc khiến cả 8 skill đều
là skill *tạo từ đầu* — không phải vì người viết skill chọn vậy, mà vì **công cụ không cho làm khác**. Bù
lệnh bridge là việc có đòn bẩy lớn hơn viết thêm skill.

## 6. Khoảng trống nghiệp vụ — loại việc chưa có skill nào phủ

| Loại việc (theo tần suất thực tế) | Hiện có | Ghi chú |
|---|---|---|
| **Sửa/hoàn thiện tài liệu có sẵn** (thêm mục, cập nhật số, làm lại báo cáo tháng trước) | ✗ | Bị chặn bởi §5; cũng chưa có skill |
| **Làm sạch dữ liệu** (trùng, thiếu, sai định dạng, tách/gộp cột, so khớp 2 danh sách) | ✗ | Bị chặn bởi thiếu sort/filter |
| **Nối 3 app** (bảng trong Word/PDF → Excel → slide) | ✗ | Mỗi skill khoá đúng 1 app |
| **Trộn thư / mail merge** (1 mẫu + 1 danh sách → N file) | ✗ | Cần lệnh lặp + field |
| **Rà soát trước khi gửi** (số liệu lệch giữa các bảng, lỗi định dạng) | ✗ | Chỉ có `checkRange`/`checkTables`/`checkLayout` rời rạc |
| Báo cáo/slide theo mẫu có sẵn | ~ | `bao-cao-thang` tạo mới, không theo mẫu người dùng |
| Tài liệu hành chính khác (hợp đồng, biên bản, thông báo, giấy mời) | ~ | `van-ban-hanh-chinh` chỉ phủ NĐ30 |

**Kết luận §6:** đây là lý do chính khiến câu trả lời cho *"đã đủ chưa"* là **chưa** — và nó **không sửa
được bằng cách đếm skill**, vì 5/7 dòng đầu đang bị chặn bởi §5 chứ không bởi thiếu hướng dẫn.

## 7. Định nghĩa "đủ" và eval

[docs/architecture/core.md:557-558](../architecture/core.md#L557-L558) §8.4.5 đã tự đặt luật: *"Viết eval trước, viết hướng dẫn
sau: mỗi skill có ≥ 3 kịch bản test dựng từ chỗ model làm sai khi **chưa** có skill."*

Hiện trạng: ba test chạm tới skill, **cả ba đều là test cấu trúc, không phải eval**:

- [TestRepoSkillsLoadAndReferencesResolve](../../core-go/internal/tools/repo_skills_test.go) — skill **nạp
  được** và mọi `read_skill_file` trỏ tới file **có thật**;
- [TestSkillReferencesDoNotHardcodeDesignValues](../../core-go/internal/tools/repo_skills_tokens_test.go)
  (thêm 12/10/2026, §4.1) — `references/*.md` không chép cứng màu/cỡ chữ, và mọi tên token nhắc tới đều
  **có thật** trong `tokens.json`;
- [TestRepoSkillDescriptionsFitRouterLimit](../../core-go/internal/agent/skills_router_test.go) (thêm
  12/10/2026, §4.9) — `description` không vượt trần 300 ký tự của router, nên không skill nào bị cắt mất
  phần từ khoá.

Chúng không nói gì về việc chỉ dẫn có đúng, có gọi được bằng lệnh thật, hay model có làm tốt hơn khi có
skill. `tests/bench/` là benchmark Calc 4 bài, không phải eval skill.

Vì vậy **"đủ" phải định nghĩa bằng tiêu chí phủ, không bằng con số**:

1. Mỗi **task type** ở §6 hoặc có skill, hoặc có quyết định ghi rõ là "không cần skill".
2. Mỗi skill có **≥ 3 kịch bản eval** dựng từ lỗi thật khi chưa có skill.
3. Mỗi eval có **ngưỡng đạt** viết trước khi chạy.
4. Đo thêm **tỉ lệ chọn đúng skill** (Recall@1): model có nạp đúng skill cho tình huống đó không. Tầng 1
   chỉ có `name` + `description`, nên đây là chỉ số đo chất lượng `description`.

Hạ tầng đã có: harness `POST /v1/runs` + SSE trong [tests/bench/](../../tests/bench) tái dùng được; số đo
`by_tool`, `dup_ratio`, `failures` đã đủ để chấm cả chất lượng lẫn chi phí.

## 8. Roadmap

| Ưu tiên | Việc | Vì sao trước |
|---|---|---|
| **P0** ✅ xong 11/10/2026 | §3.1 bỏ `writer.replaceAll` khỏi `the-thuc-van-ban`; §3.2 sửa quy tắc xoá `outside-table`; §3.3 thống nhất `empty-cells` cho bảng dàn trang | Sửa được **ngay**, không cần lệnh mới, và đang chủ động phá tài liệu người dùng |
| **P1** ✅ xong 12/10/2026 | §4.1 `bo-cuc.md` bám token + test chặn hex/cỡ chữ trần; §4.2 chốt gọi `checkLayout` một lần; §4.3 bỏ "độ rộng bảng" khỏi description; §4.4 chốt cỡ nội dung 13; §4.5 bắt viết spec ra; §4.7 sửa luật `long-header-cell` cho bảng 2 cột (leave-one-out); §4.8 xoá token chết `plainTableStyle` | Rẻ, và §4.1/§4.3/§4.7 là loại lỗi sẽ nhân lên khi thêm skill mới |
| **P1.5** ✅ xong 12/10/2026 | §4.9 `description` dài quá 300 ký tự bị cắt đuôi (5/8 skill; `mo-hinh-nhieu-sheet` mất cả danh sách từ khoá) — viết lại theo thứ tự từ khoá lên trước, thêm test chặn ở tầng router | Cùng loại với §4.3 nhưng nặng hơn, và là **điều kiện tiên quyết** của P3 (Recall@1 đo chính `description`) |
| **P1.6** ✅ xong 13/10/2026 | §4.10 skill làm slide dựng lưới theo mặc định PowerPoint (960×540) nên **tự sinh `overflow`**; lưới bị chép hai nơi (`bao-cao-thang/references/bo-cuc.md` xoá); thêm `tests/lo/measure_slide_grid.py` để mọi con số đo lại được | Sửa được **ngay**, không cần lệnh mới, mà lại đang là skill dạy sai về mặt **số học** — cùng họ với §4.1 |
| **P1.7** ✅ xong 13/10/2026 | §4.11 nguồn của skill làm slide: mở từng trang để kiểm — 9 khẳng định có nguồn thật, 13 khẳng định hạ xuống "chưa kiểm chứng" (Indezine 24pt, Kosslyn, 6x6, 60-30-10, "action safe 90%"…); thay bằng nguồn mở được (Microsoft 18pt, WCAG 1.4.3/1.4.1, Kawasaki, Slideworks, Okabe & Ito, Few) | Cùng họ §4.3/§4.10 nhưng ở tầng **thẩm quyền**: skill dạy agent nói "theo chuẩn X" cho X không tồn tại. Con số có thể vẫn đúng, nhưng **căn cứ thì không** — và đây là thứ đọc lại là thấy ngay |
| **P2** | Bù lệnh bridge theo §5, thứ tự: `writer.setCell` (mở khoá sửa bảng/tài liệu có sẵn) → `et` width/border → `writer` page setup + header/footer → `wpp` sửa shape có sẵn | Đòn bẩy lớn nhất; mở khoá 5/7 dòng ở §6 |
| **P3** | Viết eval theo §7, **rồi** mới thêm skill nghiệp vụ cho §6 | Tránh viết thêm 10 skill không đo được |
| **P4** | §4.6 đối chiếu Phụ lục I NĐ30 bản gốc | Nhỏ, nhưng là căn cứ pháp lý — không nên đoán |

## 9. Không nên làm lúc này

- **Viết skill cho header/footer, số trang, mục lục, lề, sort, filter, pivot.** Chưa có lệnh nào làm được
  (§5). Skill sẽ dạy model gọi lệnh không tồn tại — đúng loại lỗi tệ nhất, vì `office_action` chỉ trả về
  danh sách lệnh hợp lệ sau khi model đã gọi sai.
- **Cho skill chạy script.** [docs/architecture/core.md:543-546](../architecture/core.md#L543-L546) §8.4.4 ghi rõ đây là **lệch
  chuẩn có chủ đích**; giữ nguyên.
- **Thêm field frontmatter mới** (`keywords`, `requires`, `when_not_to_use`…). Loader
  ([loader.go:32-79](../../core-go/internal/skills/loader.go#L32-L79)) chỉ đọc `name`, `description`, `apps` —
  field mới sẽ **bị bỏ qua im lặng**, không có tác dụng gì cho tới khi sửa loader và `SkillsSection`.
- **Nhồi thêm chỉ dẫn vào `description`.** Nó bị cắt ở 300 ký tự khi vào system prompt
  ([prompt.go:107](../../core-go/internal/agent/prompt.go#L107)) — phần vượt sẽ **không tới model**. Cả 8 skill
  nay đã trong trần (§4.9), và `description` chỉ nên chứa thứ để **định tuyến**: một câu định vị + danh sách
  "Dùng khi …" + "Không dùng khi …". Chỉ dẫn thao tác thuộc về thân SKILL.md (tầng 2) — thêm vào
  description thì vừa bị cắt, vừa đẩy từ khoá ra ngoài trần.
