# Review bộ skill dựng sẵn (8 skill) và khoảng trống so với công việc văn phòng

Trạng thái: **P0 (§3) đã sửa xong 11/10/2026** — và chính lúc sửa §3.3 sinh thêm hai phát hiện P1 mới
(§4.7, §4.8); P1–P4 còn nguyên (xem §8). Cách làm: 4 lượt review độc lập qua
ChatGPT, rồi **đối chiếu máy móc lại với mã** — registry lệnh trong
[livecommands_gen.go](../core-go/internal/mcpserver/livecommands_gen.go) (65 mục: 60 lệnh tài liệu — `writer`
22, `et` 23, `wpp` 15 — cộng 5 lệnh app/UI), mã bridge LibreOffice
trong [src/AxiomOffice.LibreOffice/python/pythonpath/axiom/](../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/),
và tầng skill trong [core-go/internal/skills/](../core-go/internal/skills/).

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
[New_arch.md:71](../New_arch.md#L71) ghi rõ giai đoạn này gói NĐ30 "chỉ làm skill mẫu", và
[New_arch.md:566-579](../New_arch.md#L566-L579) §8.4.6 thiết kế đúng 4 skill tầng thiết kế — tức 4/8 skill
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

- `thiet-ke-van-phong` **không có `apps:`**, và [index.go:38-48](../core-go/internal/skills/index.go#L38-L48)
  cho `AppliesTo` trả `true` khi `Apps` rỗng → skill này có mặt trong cả ba app. **Đây là hành vi đúng**,
  không phải lỗi. Chỉ nên nói rõ trong `description` rằng nó dùng chung cho cả Word/Excel/PowerPoint.
- `mo-hinh-nhieu-sheet` là skill duy nhất viết từ **số đo thật** ([mo-hinh-nhieu-sheet/SKILL.md:16-17](../skills/mo-hinh-nhieu-sheet/SKILL.md#L16-L17)
  trích `tests/bench/results/`). Đây là mẫu đúng cho mọi skill nghiệp vụ sau này.

Soát tự động: **23/23 lệnh được nhắc trong skill đều tồn tại trong registry** — không có tên lệnh bịa.
Vấn đề nằm ở **ngữ nghĩa**, không ở tên lệnh.

## 3. P0 — skill đang dạy thao tác sai hoặc phá dữ liệu — **đã sửa 11/10/2026**

### 3.1 `the-thuc-van-ban` bảo sửa một ô bằng `writer.replaceAll` — lệnh không làm được việc đó

[the-thuc-van-ban/SKILL.md:33-34](../skills/the-thuc-van-ban/SKILL.md#L33-L34):

> `long-header-cell` (đoạn văn bị lọt vào ô tiêu đề: **sửa lại ô đó bằng `writer.replaceAll`**)

Nhưng [writer.py:541-567](../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/writer.py#L541-L567):
`replace_all` tạo `document.createReplaceDescriptor()` rồi gọi `document.replaceAll(descriptor)` —
**toàn tài liệu**, không nhắm được bảng/ô, và còn đặt `SearchCaseSensitive = False`.

Hệ quả: model làm đúng theo skill sẽ thay **mọi** lần xuất hiện của chuỗi trong cả văn bản, không phân biệt
hoa/thường. Với văn bản hành chính — nơi tên cơ quan, "CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM", chức vụ lặp lại
nhiều nơi — đây là lệnh phá tài liệu.

**Đã sửa:** bỏ hẳn `writer.replaceAll` khỏi skill ([the-thuc-van-ban/SKILL.md](../skills/the-thuc-van-ban/SKILL.md)).
Thay bằng hai đường **làm được thật**: bảng do **chính lượt này** chèn và sau đó chưa làm gì khác thì
`writer.undo` lùi về trước `writer.insertTable` rồi chèn lại bằng `values` đúng; bảng có sẵn trong tài liệu,
hoặc đã làm việc khác sau đó, thì **báo người dùng**, đừng tự vá. Kèm theo, `UndoRule`
([office_action.go:23-32](../core-go/internal/tools/office_action.go#L23-L32)) được nới đúng một ngoại lệ —
được `undo` thứ **chính lượt này** vừa làm khi không có cách sửa tại chỗ — nếu không thì prompt cấm đúng cái
đường duy nhất còn lại và dồn agent về `writer.replaceAll`. Lệnh `writer.setCell` vẫn nằm ở §5 (P2).

### 3.2 `bao-cao-du-lieu` bảo xoá `outside-table` bằng cách ghi `""` vào ô

[bao-cao-du-lieu/SKILL.md:39-41](../skills/bao-cao-du-lieu/SKILL.md#L39-L41):

> `outside-table` (dữ liệu ghi lạc ô — **xoá bằng cách ghi `""` vào đúng các ô đó**)

Câu này mâu thuẫn trực tiếp với chính system prompt:
[prompt.go:48-57](../core-go/internal/agent/prompt.go#L48-L57) — `CheckRule` nói *"Never change a cell
holding data the user already had, not even temporarily"*.

Vấn đề thật: `outside-table` là ô có dữ liệu **nằm ngoài vùng bảng** — mà ô đó rất thường là **dữ liệu
người dùng đã có sẵn** cạnh bảng. Skill đang dạy model xoá dữ liệu người dùng để làm `issueCount` về 0, tức
là **tối ưu chỉ số thay vì làm đúng việc**.

**Đã sửa:** [bao-cao-du-lieu/SKILL.md](../skills/bao-cao-du-lieu/SKILL.md) nay tách `outside-table` ra khỏi
danh sách "sửa mọi `issues`": chỉ xoá ô mà **chính lượt này** đã ghi nhầm, không chắc thì báo người dùng, và
muốn soát đúng bảng vừa dựng thì truyền `range` (đã kiểm trong
[checks.py](../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py): `outside-table` chỉ bắn khi
**không** truyền `range`). Yêu cầu tuyệt đối *"lặp đến khi `issueCount` = 0"* cũng bỏ — nó chính là chỗ biến
việc soát thành tối ưu chỉ số. Mô tả lệnh `et.checkRange` cũng nói rõ `outside-table` có thể là dữ liệu người
dùng đã có và **không** được xoá ô có sẵn từ trước.

### 3.3 Hai skill thiết kế để dùng cùng nhau lại mâu thuẫn về `empty-cells`

- [van-ban-hanh-chinh/SKILL.md:25-26](../skills/van-ban-hanh-chinh/SKILL.md#L25-L26) dựng bảng dàn trang
  2 cột **có ô trống cố ý** (khoảng trống ký, ô chờ điền).
- [the-thuc-van-ban/SKILL.md:33](../skills/the-thuc-van-ban/SKILL.md#L33) lại bảo:
  *"sửa `empty-cells` (điền hoặc ghi `—`)"*.
- [checks.py:66-67](../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py#L66-L67) đếm ô trống
  cho **mọi** bảng, không có ngoại lệ cho bảng dàn trang.

Mà [van-ban-hanh-chinh/SKILL.md:15](../skills/van-ban-hanh-chinh/SKILL.md#L15) nói rõ *"Dùng cùng skill
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
([writer.py](../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/writer.py),
[checks.py](../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py)). Số ô trống **vẫn hiện** trong
report để agent thấy, chỉ không thành `issue`. Ba chi tiết đo được khi làm:

- tên bảng Writer **không được chứa dấu cách hay dấu chấm** (`unotbl.cxx:2958`) và **không được trùng**
  (`:2971`), nên `_rename_table` thử lần lượt `AxiomLayout`, `AxiomLayout-2`…; `formatTable` ghi `layout-table`
  vào `applied` để tem đặt hụt **hiện ra ở `skipped`** thay vì im lặng;
- tem theo `.odt` sang lần mở sau (đo: đặt `Name` → lưu → mở lại, tên còn nguyên);
- `long-header-cell` **vẫn báo cho mọi bảng**, kể cả bảng dàn trang — nó bắt chữ lọt vào ô tiêu đề, mà với
  bảng dàn trang thì đó cũng là lỗi thật, và `van-ban-hanh-chinh` vốn đòi soát nó.

## 4. P1 — mâu thuẫn, lệch token, description hứa quá năng lực

### 4.1 `references/bo-cuc.md` chép cứng cỡ chữ và màu, lệch với `tokens.json`

[bao-cao-thang/references/bo-cuc.md:7](../skills/bao-cao-thang/references/bo-cuc.md#L7) dùng
`fontSize: 40` cho slide tiêu đề, [dòng 12](../skills/bao-cao-thang/references/bo-cuc.md#L12) dùng `32` cho
tiêu đề slide nội dung — nhưng
[skills/_design/tokens.json](../skills/_design/tokens.json) đặt `slide.type.title = 36`, và
[trinh-bay-chuyen-nghiep/SKILL.md:33](../skills/trinh-bay-chuyen-nghiep/SKILL.md#L33) bảo dùng đúng
`type.title`. File này cũng chép cứng `#1F4E79`, `#2E75B6`, `#595959`, `#262626` thay vì đọc token.

Hệ quả: hai skill dùng cùng nhau cho ra hai cỡ tiêu đề khác nhau, và **sửa `tokens.json` sẽ không lan tới
`bo-cuc.md`** — đúng cái mà §8.4.6 muốn tránh ("token thiết kế là dữ liệu").

**Sửa:** `bo-cuc.md` chỉ giữ **toạ độ và bố cục**, mọi cỡ chữ/màu ghi theo tên token (`type.title`,
`colors.primary`). Thêm test: mọi file `references/*.md` không được chứa `#RRGGBB` hay số cỡ chữ trần.

### 4.2 Hai skill lệch nhau về số lần gọi `wpp.checkLayout`

[bao-cao-thang/SKILL.md:32](../skills/bao-cao-thang/SKILL.md#L32): *"`wpp.checkLayout` cho **từng** slide mới"*.
[trinh-bay-chuyen-nghiep/SKILL.md:16](../skills/trinh-bay-chuyen-nghiep/SKILL.md#L16): *"Gọi `wpp.checkLayout`
**một lần**"*.

`checkLayout` mặc định kiểm **mọi** slide, nên cách của `bao-cao-thang` là thừa lượt gọi — mà chính
`CheckRule`/`PlanRule` trong [prompt.go](../core-go/internal/agent/prompt.go) đang cấm đọc thừa. Chốt một
cách: gọi một lần, không truyền `slide`.

### 4.3 `description` của `bao-cao-du-lieu` hứa "độ rộng bảng" — không lệnh nào làm được

[bao-cao-du-lieu/SKILL.md:4-5](../skills/bao-cao-du-lieu/SKILL.md#L4-L5) hứa *"mật độ và **độ rộng bảng**"*.
`et.formatRange` chỉ nhận `range, bold, italic, fontSize, fontColor, fillColor, numFmt, horizontal, wrap,
sheet` — **không có width/height**, và quét cả registry không có lệnh nào đặt độ rộng cột.

Đây là lỗi nặng hơn nó trông: `description` **chính là router** (tầng 1 — model chỉ thấy dòng này để quyết
định nạp skill). Một description hứa năng lực không tồn tại sẽ khiến model nạp skill rồi bị kẹt.

**Sửa:** bỏ "độ rộng bảng" khỏi description cho tới khi có lệnh đặt width.

### 4.4 Cho hai lựa chọn thay vì một mặc định

[van-ban-hanh-chinh/references/the-thuc.md:18](../skills/van-ban-hanh-chinh/references/the-thuc.md#L18) ghi
cỡ nội dung `13–14`, [SKILL.md:32](../skills/van-ban-hanh-chinh/SKILL.md#L32) dùng `size: 14`, còn
`tokens.json` đặt `document.type.body = 13`. [New_arch.md:563-564](../New_arch.md#L563-L564) §8.4.5 đã đặt
luật: *"nêu một cách mặc định thay vì liệt kê nhiều lựa chọn"*. Chốt một số.

### 4.5 `thiet-ke-van-phong` dạy "viết spec ngắn **trong đầu**"

[thiet-ke-van-phong/SKILL.md:34](../skills/thiet-ke-van-phong/SKILL.md#L34). Chỉ dẫn này không kiểm chứng
được và không ràng buộc hành vi. Cách `mo-hinh-nhieu-sheet` làm mới đúng: bắt **viết plan ra** rồi bám theo
(quy tắc 9 của skill đó, và `PlanRule` trong `prompt.go`).

### 4.6 NĐ30: một quy tắc chưa đối chiếu được bản gốc **(chưa xác minh)**

[the-thuc.md:26](../skills/van-ban-hanh-chinh/references/the-thuc.md#L26): *"Ngày dưới 10 và tháng 1, 2 thêm
số 0"*. Lượt review qua ChatGPT cho rằng quy tắc này **không có** trong Phụ lục I NĐ 30/2020. Tôi **không
xác minh được**: WebSearch trả kết quả rỗng và thuvienphapluat.vn chặn `403 Forbidden`.

Đây có thể là **thông lệ** soạn thảo chứ không phải quy định của nghị định. **Cần đối chiếu bản gốc Phụ lục I
trước khi giữ câu này.** Ngược lại, mục *"Nơi nhận:"* cỡ 12 nghiêng đậm và danh sách nơi nhận cỡ 11
([the-thuc.md:21-22](../skills/van-ban-hanh-chinh/references/the-thuc.md#L21-L22)) thì tôi cho là **đúng** —
ChatGPT phân vân chỗ này nhưng không đưa được căn cứ.

### 4.7 `long-header-cell` **không thể** bắn với bảng 2 cột — mà `van-ban-hanh-chinh` chỉ dựng bảng 2 cột

Phát hiện khi sửa §3.3. [checks.py:74-81](../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/checks.py#L74-L81):
`typical = lengths[len(lengths) // 2]` lấy **trung vị của chính các ô tiêu đề**, rồi đòi
`len(text) > 40` **và** `len(text) > 3 * typical`.

Với bảng **đúng 2 cột**, `sorted([a, b])[1]` chính là giá trị **lớn hơn** — nên ô dài nhất có
`len(text) == typical`, và `typical > 3 * typical` luôn **sai**. Đã kiểm bằng mã (không cần LibreOffice):
bảng 2 cột với một đoạn văn 57 ký tự gõ lọt vào ô tiêu đề → `long-header-cell` **không bắn**. Đây là
**bất khả thi về số học**, không phải ngưỡng đặt cao.

Hệ quả trực tiếp: [van-ban-hanh-chinh/SKILL.md:44-45](../skills/van-ban-hanh-chinh/SKILL.md#L44-L45) đòi
*"không được có `long-header-cell` do gõ lọt chữ"*, nhưng cả hai bảng của skill này đều **2 cột**
([SKILL.md:19-20, 25-26](../skills/van-ban-hanh-chinh/SKILL.md#L19-L26)). Một điều kiện **không bao giờ sai**
thì không phải là kiểm tra — nó chỉ làm checklist trông chặt hơn thực tế, và đúng cái lỗi nó sinh ra để bắt
(chữ lọt vào ô tiêu đề) thì nó không bắt được.

**Chưa chốt cách sửa** — và đừng sửa bằng cách nới con số:

- đổi mốc so sánh từ trung vị **ô tiêu đề** sang trung vị **mọi ô trong bảng** thì bắt thêm được vài ca
  (đã kiểm bằng mã: bảng "Nơi nhận:" 2 cột bắt được), nhưng **vẫn trượt** bảng phần đầu NĐ30 — vì ô
  `"CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM"` (34 ký tự) là chữ **hợp lệ** mà lại kéo trung vị lên;
- nên hướng đúng nhiều khả năng là **bỏ hẳn tỉ lệ**, thay bằng luật theo số cột, hoặc **bỏ yêu cầu này khỏi
  checklist** của `van-ban-hanh-chinh` và dựa vào `writer.getText` đọc lại (việc skill vốn đã yêu cầu).

Cần người quyết trước khi sửa, vì cả ba hướng đều đổi hành vi soát của `the-thuc-van-ban` cho **mọi** bảng.

### 4.8 `tokens.json` có một token không ai đọc: `plainTableStyle`

[skills/_design/tokens.json:32](../skills/_design/tokens.json#L32) đặt `document.plainTableStyle = "Table Grid"`.
Quét cả kho: **không file `.md`, `.go`, `.py` nào nhắc tới `plainTableStyle`**. Token chết.

Khác `document.tableStyle` (dòng 31, được
[the-thuc-van-ban/SKILL.md:24](../skills/the-thuc-van-ban/SKILL.md#L24) dùng thật, và
[writer.py:28-31](../src/AxiomOffice.LibreOffice/python/pythonpath/axiom/writer.py#L28-L31) dịch tên style Word
sang autoformat LibreOffice), `plainTableStyle` không có đường nào tới tài liệu.

Đây là cùng một loại lỗi với §4.1 — **token là dữ liệu, nhưng skill không đọc hết dữ liệu** — chỉ khác là
§4.1 đọc sai (chép cứng), còn §4.8 là không đọc (chết hẳn). Lưu ý thêm: giá trị `"Table Grid"` **sẽ** chạy
được nếu có ai gọi (`"table grid"` khớp nhánh `"grid" in lower` → `"Default Table Style"`), nên đây là token
**thừa**, không phải token **sai**. Sửa: hoặc dùng nó cho `writer.formatTable {style}` của bảng dữ liệu, hoặc
xoá khỏi `tokens.json`. Xoá là lựa chọn rẻ hơn cho tới khi có nhu cầu thật.

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

[New_arch.md:557-558](../New_arch.md#L557-L558) §8.4.5 đã tự đặt luật: *"Viết eval trước, viết hướng dẫn
sau: mỗi skill có ≥ 3 kịch bản test dựng từ chỗ model làm sai khi **chưa** có skill."*

Hiện trạng: test duy nhất chạm tới skill là
[TestRepoSkillsLoadAndReferencesResolve](../core-go/internal/tools/repo_skills_test.go) — nó kiểm skill
**nạp được** và mọi `read_skill_file` trỏ tới file **có thật**. Đó là test **cấu trúc**, không phải eval:
nó không nói gì về việc chỉ dẫn có đúng, có gọi được bằng lệnh thật, hay model có làm tốt hơn khi có skill.
`tests/bench/` là benchmark Calc 4 bài, không phải eval skill.

Vì vậy **"đủ" phải định nghĩa bằng tiêu chí phủ, không bằng con số**:

1. Mỗi **task type** ở §6 hoặc có skill, hoặc có quyết định ghi rõ là "không cần skill".
2. Mỗi skill có **≥ 3 kịch bản eval** dựng từ lỗi thật khi chưa có skill.
3. Mỗi eval có **ngưỡng đạt** viết trước khi chạy.
4. Đo thêm **tỉ lệ chọn đúng skill** (Recall@1): model có nạp đúng skill cho tình huống đó không. Tầng 1
   chỉ có `name` + `description`, nên đây là chỉ số đo chất lượng `description`.

Hạ tầng đã có: harness `POST /v1/runs` + SSE trong [tests/bench/](../tests/bench/) tái dùng được; số đo
`by_tool`, `dup_ratio`, `failures` đã đủ để chấm cả chất lượng lẫn chi phí.

## 8. Roadmap

| Ưu tiên | Việc | Vì sao trước |
|---|---|---|
| **P0** ✅ xong 11/10/2026 | §3.1 bỏ `writer.replaceAll` khỏi `the-thuc-van-ban`; §3.2 sửa quy tắc xoá `outside-table`; §3.3 thống nhất `empty-cells` cho bảng dàn trang | Sửa được **ngay**, không cần lệnh mới, và đang chủ động phá tài liệu người dùng |
| **P1** | §4.1 `bo-cuc.md` bám token + test chặn hex/cỡ chữ trần; §4.2 chốt một cách gọi `checkLayout`; §4.3 bỏ "độ rộng bảng" khỏi description; §4.4 chốt một cỡ chữ; §4.5 bắt viết plan ra; §4.7 **quyết** luật `long-header-cell` cho bảng 2 cột (cần người chốt); §4.8 xử lý token chết `plainTableStyle` | Rẻ, và §4.1/§4.3/§4.7 là loại lỗi sẽ nhân lên khi thêm skill mới |
| **P2** | Bù lệnh bridge theo §5, thứ tự: `writer.setCell` (mở khoá sửa bảng/tài liệu có sẵn) → `et` width/border → `writer` page setup + header/footer → `wpp` sửa shape có sẵn | Đòn bẩy lớn nhất; mở khoá 5/7 dòng ở §6 |
| **P3** | Viết eval theo §7, **rồi** mới thêm skill nghiệp vụ cho §6 | Tránh viết thêm 10 skill không đo được |
| **P4** | §4.6 đối chiếu Phụ lục I NĐ30 bản gốc | Nhỏ, nhưng là căn cứ pháp lý — không nên đoán |

## 9. Không nên làm lúc này

- **Viết skill cho header/footer, số trang, mục lục, lề, sort, filter, pivot.** Chưa có lệnh nào làm được
  (§5). Skill sẽ dạy model gọi lệnh không tồn tại — đúng loại lỗi tệ nhất, vì `office_action` chỉ trả về
  danh sách lệnh hợp lệ sau khi model đã gọi sai.
- **Cho skill chạy script.** [New_arch.md:543-546](../New_arch.md#L543-L546) §8.4.4 ghi rõ đây là **lệch
  chuẩn có chủ đích**; giữ nguyên.
- **Thêm field frontmatter mới** (`keywords`, `requires`, `when_not_to_use`…). Loader
  ([loader.go:32-79](../core-go/internal/skills/loader.go#L32-L79)) chỉ đọc `name`, `description`, `apps` —
  field mới sẽ **bị bỏ qua im lặng**, không có tác dụng gì cho tới khi sửa loader và `SkillsSection`.
- **Nhồi thêm chỉ dẫn vào `description`.** Nó bị cắt ở 300 ký tự khi vào system prompt
  ([prompt.go:102](../core-go/internal/agent/prompt.go#L102)) — phần vượt sẽ không tới model.
