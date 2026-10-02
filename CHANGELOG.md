# Changelog

Mọi thay đổi đáng chú ý của project được ghi ở đây.
Format tham khảo [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added — `tests/bench/compare_values.py`: đối chiếu GIÁ TRỊ, không chỉ cấu trúc
- Bốn báo cáo trước chỉ so tên sheet / số ô công thức / số chart, mà đề chấm bằng *"kết quả phải đúng về
  dữ liệu, công thức, logic nghiệp vụ"*. Công cụ này đọc **giá trị đã lưu** của cả hai file (`.ods` đổi
  sang `.xlsx` trước để dùng cùng một đường đọc): ô lỗi, công thức chưa có kết quả, và sheet kiểm tra
  đọc ra bao nhiêu PASS/FAIL.
- **Kết quả (Test 1–4, số nguyên văn trong `tests/bench/results/`)**: bài Test 2 và Test 3 của Axiom
  **ra khỏi tay agent với ô lỗi nằm trong file** — 8 ô ngay trên sheet `Checks` (Test 2:
  `#NAME?`/`#DIV/0!`/`#VALUE!`/`#N/A`) và 106 ô `#VALUE!` (Test 3). Đây là loại lỗi mà `et.checkRange`
  bắt được (`error-values`), nên đây là lỗi **quy trình** chứ không phải thiếu công cụ; đã đưa vào
  checklist của skill `mo-hinh-tai-chinh`.
- File do thư viện sinh (phía Claude Code) **không có kết quả lưu sẵn**, nên "0 ô lỗi" ở phía đó nghĩa
  là *không đọc được*, không phải *không có* — ghi rõ trong báo cáo để không kết luận oan.
- **Một lỗi đo của chính tôi, do dữ liệu thật bắt được**: lần chạy đầu chỉ đếm ô công thức khi ô đó có
  giá trị lưu sẵn, nên báo phía Claude Code "**0 công thức**" trên file có gần 400.000 công thức. Sai ở
  chỗ đọc, không phải ở file.

### Added — `et.importCsv`: nạp dữ liệu từ file vào sổ đang mở
- `path` (bắt buộc), `range?`, `sheet?`, `delimiter?`, `encoding?`. Không có `range` thì tạo sheet mới
  đặt tên theo tên file (trùng thì thêm số), có `range` thì ghi vào ô góc đó của sheet chỉ định.
- Ô trông như số thì thành **số**, còn lại là chuỗi; **ngày để nguyên chuỗi** — đoán sai kiểu ngày còn
  tệ hơn để người dùng tự chọn định dạng. Khuôn dạng số được nhận bằng regex chứ không bằng `float()`
  (`float()` nhận cả `nan`/`inf`, đó là chữ không phải số).
- Ghi bằng `setDataArray` theo lô (một lời gọi cho cả khối) thay vì từng ô; trần 500.000 ô để một file
  lỗi không treo LibreOffice.
- Có ở cả hai bridge, vào `catalog/live-commands.json` (64 lệnh).
- **Điều nó KHÔNG làm**: không lấp được khoảng cách token của benchmark — dữ liệu giả lập không nằm sẵn
  trong file nào, mà agent của Axiom không có đường chạy mã để tự sinh (Claude Code có shell và
  `gen_data.py`). Ghi rõ ở `tests/bench/results/con-lai.md` để lần sau khỏi tưởng nhầm.
- Kiểm chứng: `tests/live/test_live_libreoffice.py` phần Calc **162/162** (gồm chữ có dấu, mã dạng
  chuỗi, ô trống, số âm, và ca `2025-01-15` phải ở lại là CHUỖI), `tests/lo/test_extension.py` 19/19.

### Added — `et.writeRanges`: ghi nhiều vùng trong một lời gọi
- Đo được ở Test 2/3/4: model hay gọi 5–10 `et.writeRange` liên tiếp trong **cùng một phản hồi**; mỗi
  lời gọi là một vòng qua bridge (HTTP + gate + một lượt model trả lời). `writes` là mảng
  `{range, values, sheet?}`, ghi tuần tự, trả `ranges`/`written`/`sheets` và gom `formulaErrors`.
- **Kiểm hết tham số trước khi ghi**: vùng thứ ba sai thì hai vùng đầu **không** được vào file — có
  bài live kiểm đúng tính chất đó, không chỉ kiểm đường thành công.
- **Không hứa nhanh hơn về tính toán**: đường tắt (tắt tính tự động rồi `calculateAll()` một lần) đã
  được đo là **chậm hơn 2,3–4,0 lần** và đã hoàn tác; lệnh này chỉ gộp lời gọi.
- Có ở cả hai bridge, vào `catalog/live-commands.json` (63 lệnh). Kiểm chứng:
  `tests/live/test_live_libreoffice.py` phần Calc **148/148** (trước 137), `tests/lo/test_extension.py`
  19/19.

### Removed — bản MCP server C# (10 tệp, ~5.300 dòng)
- Xoá hẳn bản cài đặt thứ hai của cùng hợp đồng 50 tool: `McpServer`, `WordFiles`, `ExcelFiles`,
  `PptFiles`, `LiveTools`, `Cells`, `OoxmlPackage`, `XlsxBook`, `XlsxStyles`, `BridgeClient` và hai
  template docx/pptx nhúng trong exe (cùng hai cờ `/resource:` ở `build.ps1` và CI).
- **Lý do**: từ khi Core tự làm MCP server, bản C# **không chạy ở đâu nữa** (Windows chuyển tiếp sang
  Core), và nó **không được CI kiểm ở đâu cả** — hai bản cùng một hợp đồng mà chỉ một bản có người
  canh là cách chắc chắn nhất để chúng lệch nhau.
- **Giữ lại cửa vào cũ**: `AxiomOffice.Host.exe mcp …` vẫn chạy (232 dòng chuyển tiếp byte trong
  `McpHost.cs`), nên người đã cấu hình MCP client trỏ vào Host.exe **không phải đổi gì**. Gói cài thiếu
  `AxiomOffice.Core.exe` thì Host.exe báo rõ và thoát mã 2, không còn bản dự phòng để lệch.
- **Kiểm chứng**: `AxiomOffice.Host.exe` 559.104 → **355.328 byte**; `test_mcp_portable.py` nhắm vào
  `Host.exe mcp` **61/61** (tức cửa chuyển tiếp còn nguyên); Host.exe đặt một mình trong thư mục trống
  → thoát **mã 2** kèm hướng dẫn, không treo; `tests/lo/test_extension.py` (so registry Python với
  `Host.exe commands --json`) 19/19.

### Added — skill `mo-hinh-tai-chinh`: mô hình nhiều sheet (FP&A)
- Bộ skill cũ chỉ nói về **trình bày** (bảng điểm, number format, hàng tổng). Đo được: mỗi lượt chạy
  chỉ nạp **một** skill, và với app `et` chỉ có 3 skill được chào — không skill nào nói về **mô hình**.
  Nay có skill thứ tư, và mỗi quy tắc trong đó gắn với một lỗi **đã đo trên chính mô hình do agent
  dựng**, không phải lời khuyên chung:
  - **9/13 sheet đề yêu cầu TRỐNG HOÀN TOÀN** trong một lượt Test 3 (làm hết sheet dữ liệu rồi hết ngân
    sách trước khi tới phần phân tích) → dựng xong một nhánh dọc trước khi nhân ra.
  - **60.006 công thức đọc một ô trống** (`Inputs.C25`) trong khi ô seed được dán nhãn "Seed Value" ở
    `B22` **không công thức nào đọc** → ô giả định phải có giá trị trước, và `et.checkRange` phải sạch
    `empty-reference`.
  - Một check so ô chứa **chữ** `"integer"` với một con số nên **PASS oan** → check phải so giá trị ô
    điều khiển.
  - Đổi giả định rồi đọc lại là phép thử **duy nhất** phân biệt ô điều khiển sống với ô chết.
  - Điều kiện màu phải do `et.setConditionalFormat` sinh ra, không tô tay (tô tay đứng yên khi dữ liệu
    đổi).
- `skills/mo-hinh-tai-chinh/SKILL.md`; Core nạp **8 skill, 0 lỗi** (kiểm bằng cách chạy Core trên thư
  mục skill sạch và đọc `core.log`).

### Added — định dạng điều kiện: `et.setConditionalFormat` / `et.listConditionalFormats`
- Bốn bài benchmark đều cần tô màu theo ngưỡng, và đây là thứ **duy nhất** trong danh sách khoảng cách
  mà Axiom hoàn toàn không làm được. Hai lần đo trước kết luận "không có đường nào" — kết luận đó **sai**.
- **Sai ở chỗ đo, không phải ở chỗ API**: hai probe cũ chỉ thao tác trên **vật chứa**
  (`doc.createInstance`, `ServiceManager`, `uno.createUnoStruct`, `sheet.ConditionalFormats.createInstance()`)
  và chỉ kiểm "lời gọi có ném lỗi không". Đường đúng đi qua **đối tượng theo vùng**, và có hai cái bẫy im
  lặng: `createByRange()` trả về **số ID** chứ không phải đối tượng, còn `createEntry()` **tạo thật nhưng
  pyuno trả về `None`** — phải lấy lại bằng `getByIndex`. Đo được bằng probe chạy thật trên LibreOffice
  26.8.0.3, kết quả nguyên văn ở `tests/bench/results/probe-conditional-format-3.txt`.
- **Không phải đi đường vòng qua gói OOXML** (lưu ra file → sửa `xl/worksheets/sheetN.xml` → mở lại) như
  cách đã quan sát được ở phía Claude Code: lệnh này chạy **trên tài liệu đang mở**.
- Rule nhận `operator` (less/lessEqual/greater/greaterEqual/equal/notEqual/between/notBetween/formula),
  `formula1`, `formula2`, và `styleName` có sẵn (Good/Bad/Neutral/Warning/…) hoặc `bold`/`italic`/
  `fontColor`/`fillColor`/`numFmt` — kiểu sau tự sinh một cell style và **dùng lại** nếu đã có, không để
  rác lại trong danh sách style. Gọi lại trên cùng một vùng là **THAY** rule cũ của vùng đó.
- **Kiểm chứng chạy thật** (`tests/live/test_live_libreoffice.py`, phần Calc **137/137**):
  - vòng `tên → ghi → đọc lại → tên` cho **mọi** toán tử; đây là thứ giữ cho bảng số int trong
    `calc.py` không thể lệch âm thầm;
  - gọi lại trên cùng vùng thì còn **đúng một** bộ rule;
  - 5 ca tham số sai (toán tử lạ, thiếu `formula1`, `between` thiếu `formula2`, style không tồn tại,
    thiếu `rules`) đều báo lỗi đọc được;
  - `et.saveAs` ra `.xlsx` rồi tìm `<conditionalFormatting>` trong gói — **định dạng khác hẳn ODF**, nên
    vào được cả hai nghĩa là nó thật sự vào file, không phải trạng thái tạm trong phiên.
  - `tests/lo/test_extension.py` 19/19: `catalog/live-commands.json` (62 lệnh) khớp registry và khớp bản C#.
- Bản C# (Excel/WPS) có lệnh **cùng tên, cùng tham số** (`Range.FormatConditions`) để agent thấy một hợp
  đồng thống nhất, nhưng **chưa chạy thử trên Excel/WPS thật** — máy này chỉ có LibreOffice. Khác biệt có
  chủ ý: LibreOffice gắn style bằng **tên cell style** nên `styleName` chỉ có tác dụng bên đó; Excel gắn
  màu trực tiếp.

### Added — đọc `.xls` thẳng bằng Go, không cần LibreOffice lẫn Excel
- Ứng dụng `.xls` (BIFF8) được đọc **trực tiếp**: `internal/cfb` mở thùng Compound File Binary (OLE2) và
  `internal/xlsx/biff.go` đọc bản ghi BIFF8. Trước đây bản Linux phải nhờ
  `soffice --convert-to xlsx`, còn bản Windows đọc qua COM Excel/WPS — cả hai đều buộc người dùng cài
  thêm một thứ mà họ có thể không có.
- **Vì sao đáng làm**: sau khi MCP gom về một bản Go, người dùng Windows **có Office/WPS mà không có
  LibreOffice** là **đa số**, và chính họ sẽ mất `.xls`. Không còn ai để nương vào.
- Đọc thẳng cũng khớp với cách repo này vốn làm: chỉ **2 phụ thuộc trực tiếp** (`x/sys`,
  `modernc.org/sqlite` — bản pure-Go thay vì CGO) và cả engine OOXML đều tự viết. Không thêm thư viện nào.
- Giá trị trả về **khớp đúng** hai đường cũ: ô có định dạng ngày ra chuỗi ISO kèm giờ
  (`sheet.SerialToIso`, cùng hằng số với đường `.xlsx`), số nguyên ra `int64`, còn lại `float64`.
- Chỉ BIFF8 (Excel 97-2003). Gặp `.xls` cũ hơn thì nhờ LibreOffice nếu máy có, không thì báo rõ cả hai
  đường đã thử — không đoán bừa.
- **Kiểm chứng**:
  - `internal/cfb`: đối chiếu **từng byte** với một bản tự lần chuỗi viết trong test, trên **cả hai**
    đường chứa stream — mini stream (`sample.xls`, stream 2.281 byte) và sector thường (`large.xls`,
    72.272 byte). Cắt file ở mọi kích thước để chắc không panic/treo.
  - `internal/xlsx`: **đối chiếu hai đường đọc độc lập** — cùng một bảng ở hai định dạng
    (`sample.xlsx` gốc và `sample.xls` do LibreOffice đổi ra), đọc bằng reader OOXML và reader BIFF rồi
    so từng ô.
  - **`TestReadXLSWithoutLibreOffice`**: đặt `AXIOM_SOFFICE=none` để coi như máy không có LibreOffice rồi
    đọc `.xls` — đúng ca người dùng gặp, và là ca mà trước đây không đọc được.
  - `test_mcp_portable.py` nay chạy được mục `.xls` **cả khi máy không có LibreOffice** (dùng file mẫu
    đã commit thay vì tự sinh); trước đây nó **tự bỏ qua**, tức là đúng ca hỏng lại không ai kiểm.
- Hai lỗi tự gây ra, đều âm thầm, đều do test bắt: hàm nhận diện chữ ký CFB so sánh little-endian với
  hằng số viết theo thứ tự byte (mọi file `.xls` bị từ chối); và tên sheet/định dạng đọc `cch` theo
  **byte** trong khi chuỗi UTF-16 chiếm gấp đôi (tên "Trống" ra "Tr", ngày ra số thay vì chuỗi ISO).

### Changed — `AxiomOffice.Host.exe mcp` chuyển tiếp sang Agent Core (bản Go)
- Bước đầu để chỉ còn **một** bản MCP server. Trên Windows, MCP server nay là
  `AxiomOffice.Core.exe mcp …`; `AxiomOffice.Host.exe mcp …` **giữ nguyên giao diện** nhưng chuyển tiếp
  sang Core, nên người dùng đã cấu hình MCP client trỏ vào `Host.exe` **không phải đổi gì**.
- Chuyển tiếp ở mức **byte** trên stdin/stdout/stderr: giao thức là JSON theo dòng trên stdout, giải mã
  rồi mã hoá lại chỉ thêm một chỗ để sai.
- **Thiếu Core thì chạy bản C# như cũ** — gói cài có thể không kèm Core (máy build không có Go thì
  `scripts/build.ps1` bỏ qua), không để ai bị kẹt. `--in-process` ép chạy bản C# khi cần so sánh.
- **Vì sao**: hai bản MCP 50 tool là hai chỗ để lệch nhau. Ngày 02/10/2026 phải sửa **cùng một lỗi hai
  lần** (`empty-reference` ở `checks.py` và `Checks.cs`; tên chart ở `calc.py` và `Spreadsheet.cs`), và
  một lỗi chỉ có ở C# (`(?P<…>)` của Python không phải cú pháp nhóm có tên của .NET) đã làm
  `Host.exe` chết ngay khi gọi — biên dịch vẫn xanh.
- Bản Go **đã chạy được MCP trên Windows**: `test_mcp_portable.py` **70/70** nhắm vào
  `AxiomOffice.Core.exe` (đo trước khi đổi, không phải suy đoán).
- **Nợ `.xls` đã trả** (xem mục "đọc `.xls` thẳng bằng Go" ở trên): bản Go nay đọc BIFF8 trực tiếp, nên
  chỗ dùng COM **duy nhất** của bản C# chỉ còn chạy khi gói cài không kèm Core. Với bước này, bỏ hẳn
  `Mcp/` khỏi bản C# (bước 4 của kế hoạch) không còn làm mất tính năng nào.
- `AxiomOffice.Host.exe` **vẫn còn** ba vai trò khác: cầu COM companion (`… wps|et|wpp|word|excel|ppt`),
  `commands --json` (bài parity đang dùng), và `llm-test`.

### Fixed — et.addChart hỏng trên MỌI sheet chưa có biểu đồ (tên chart duy nhất theo tài liệu)
- **Tên chart trong LibreOffice là duy nhất theo TÀI LIỆU, không theo sheet.** Guard cũ chỉ hỏi
  `sheet.Charts.hasByName(name)` — mà sheet chưa có chart thì luôn trả về "chưa có" — nên nó cho qua rồi
  `addNewByName` mới đụng tên và ném ra `RuntimeException: Couldn't convert <traceback object ...>`.
- Vì tên tự sinh là `"Chart%d" % (Count+1)`, **mọi lần thêm chart trên sheet chưa có chart đều sinh ra
  `Chart1`** — và nếu `Chart1` đã bị một sheet khác giữ thì hỏng hết.
- **Đo được**: trên file Test 4, sheet `Sensitivity` có 0 chart và 0 hình, `Statistics` giữ `Chart1..Chart5`.
  Thử tại chỗ: `Chart2` → hỏng, `Chart5` → hỏng, `Chart6` → được, `Chart20` → được. Trong một lượt chạy
  thật, `et.addChart` hỏng **11 lần liên tiếp** vì lý do này, và thông báo lỗi không cho agent biết gì để sửa.
- Nay tên được kiểm trên **cả tài liệu** (`_taken_chart_names`, gồm cả tên hình trên trang vẽ của mọi
  sheet), và tên tự sinh nhảy qua mọi tên đã dùng. Đặt tên trùng thì báo lỗi đọc được:
  *"a chart named 'Chart2' already exists in this workbook; chart names are unique per DOCUMENT, not per
  sheet - pick another name"*.
- `et.listCharts` trả thêm khoá `shapes`: tên mọi hình trên trang vẽ, để thấy được hình còn sót.
- Bản C# (Excel/WPS) sửa một chỗ nhỏ tương ứng: tên tự sinh giờ tránh **mọi hình trên sheet**, không chỉ
  hình là chart (`ChartObjects().Count + 1` có thể sinh đúng tên một hình không phải chart). Ở Excel tên
  chart là duy nhất **theo sheet**, nên không cần kiểm toàn tài liệu như bản LibreOffice — khác nhau có
  chủ ý, ghi rõ trong mã.
- Kiểm chứng: `test_live_libreoffice.py --apps calc` **84/84** (thêm hai mục: sheet khác tự đặt tên chưa
  dùng, và đặt tên đã dùng phải ra lỗi đọc được chứ không phải lỗi pyuno).

### Added — et.checkRange báo công thức trỏ vào ô TRỐNG
- Thêm loại lỗi `empty-reference` cho **cả hai bridge**: một công thức trỏ vào MỘT ô đang trống thì không
  có gì báo lỗi — phép tính coi ô trống là 0 và chạy tiếp. Đây là cách một ô điều khiển "chết" mà không
  ai biết.
- **Vì sao**: đo ngày 02/10/2026 trên đề Monte Carlo. Một workbook 10.000 đường, tự dựng sheet `Checks`
  báo **18/18 PASS**, nhưng ô "Seed Value [change to re-run]" ở `B22` **không công thức nào đọc** — 60.006
  công thức đọc `C25`, một ô **trống**. Đổi ô được dán nhãn không làm gì cả; đổi `C25` thì mô phỏng đổi
  (1.750.908 so với 576.821). Chính bản kiểm tra của nó cũng so sai cột (`IF(Inputs.$C$22>=1;…)` với C22
  là **chữ** "integer").
- Lần chạy lại sau đó mắc lại đúng lỗi đó ở một dòng khác (`B35` được dán nhãn, công thức đọc `B34` là
  tiêu đề mục). Nên đây không phải chuyện cá biệt.
- **Đọc lại không phát hiện được**: `et.readRange` chỉ trả giá trị, mà một ô ghi cứng và một ô công thức
  ra cùng kết quả trông giống hệt nhau. Phải soi xem công thức trỏ vào đâu — nên phép soi này nằm trong
  `checkRange`, thứ agent **đã gọi sẵn** (17 lần trong một lượt chạy), tức là nó chạy **trong lúc dựng
  bài**, không phải chỉ khi model tự tuyên bố xong. Ở bài lớn model không bao giờ tuyên bố xong, nên bước
  kiểm chứng cuối lượt không bao giờ chạy tới.
- Bỏ qua vùng (`A1:B2` — cả hai đầu đều là tham chiếu hợp lệ) và tên sheet không tồn tại; giới hạn 4000
  ô công thức và 60 ô đích để không làm chậm báo cáo. Đo A/B trên sổ 10.007 dòng: **32,4s → 34,6s**
  (+2,2s; phần chậm còn lại là vòng phân loại sẵn có chứ không phải phép soi mới).
- Trên file có lỗi thật, nó báo đúng: `Simulation!B3 -> Inputs!C25 (trống)`.
- Kiểm chứng: `test_live_libreoffice.py` **197/197** trên LibreOffice thật (thêm ba mục: bắt được tham
  chiếu rỗng, chỉ đúng ô trống chứ không báo ô có nội dung, và chiều chống kêu oan).

### Added — biểu đồ trên tài liệu đang mở (et.addChart / et.listCharts)
- Hai lệnh mới cho CẢ HAI bridge (extension LibreOffice và add-in Windows). Bộ benchmark Calc
  (`libre_calc_agent_extreme_benchmark`) yêu cầu "Use charts where appropriate" ở Test 1 và
  "Create charts for:" ở Test 4 — trước đây không có cách nào tạo biểu đồ trên tài liệu đang mở, nên
  Dashboard chỉ có thể là bảng số.
- `et.addChart {range, type?, title?, name?, anchor?, width?, height?, sheet?}`: `type` là
  column/bar/line/pie/area/scatter, `anchor` là ô neo góc trên-trái, `width`/`height` tính bằng cm.
- `et.listCharts {sheet?}` trả về tên + **kiểu thật** của từng biểu đồ.
- Kết quả `addChart` nói **kiểu đọc lại được** (`diagram`) và `typeApplied`, không lặp lại điều vừa xin:
  đo bằng probe UNO ngày 02/10/2026 cho thấy thuộc tính đặt tiêu đề là `HasMainTitle` chứ **không phải**
  `HasTitle` (bản LibreOffice này không có `HasTitle`), và gán `chart.Diagram` thì chạy được và đọc lại
  được bằng `getDiagramType()`. `column` và `bar` cùng là `BarDiagram` của LibreOffice nên phải đọc
  thêm thuộc tính `Vertical` mới phân biệt được — hai bridge quy về cùng bộ tên nên so sánh được với nhau.

### Fixed — lượt chạy bị cắt vì hết ngân sách token của MỘT phản hồi
- Trước: `finish_reason=length` với content rỗng làm **hỏng cả lượt chạy**. Hai lượt chạy thật ngày
  02/10/2026 (cùng prompt FP&A trên LibreOffice) chết hẳn ở đây sau khi đã làm được nhiều việc.
- Nay: nhắc model viết nhỏ lại rồi làm tiếp (`TruncatedReplyNudge`), tối đa một lần, rồi mới bỏ cuộc.
  Trần token mỗi phản hồi cũng nâng 16384 → 32768 (model đang dùng khai `maxOutput` 384000).
- Thêm luật vào system prompt: khảo sát chỉ một-hai lời gọi rồi bắt tay dựng bài, không chạy một loạt
  thí nghiệm nhỏ để dò xem ứng dụng hỗ trợ gì — danh sách lệnh đã nằm trong mô tả `office_action`, và
  gọi một lệnh không tồn tại thì lỗi trả về kèm danh sách lệnh có thật.

### Added — tests/bench: đọc transcript Claude Code để lấy CÁCH làm, không chỉ kết quả
- `tests/bench/summarize.py`: thống kê tool call, lệnh Bash, file ghi/sửa, token, và trả lời cuối của
  một phiên `.jsonl`. Dùng cho việc đối chiếu benchmark (xem `tests/bench/README.md`).

### Added — agent tự kiểm chứng, hiện suy luận, đo cache
- **Agent tự kiểm chứng trước khi trả lời** (`VerifyWorkEnabled`, mặc định **BẬT**). Luot nào có lệnh
  SỬA tài liệu thì sau khi model định trả lời, harness chen một lượt nữa yêu cầu nó đọc lại bằng chính
  các lệnh có sẵn (`et.readRange`/`et.checkRange`, `writer.getText`/`writer.checkTables`,
  `wpp.listSlides`/`wpp.checkLayout`) rồi sửa nếu thiếu, xong mới trả lời người dùng. Các lệnh chẩn đoán
  này vốn đã có từ trước nhưng không ai buộc model dùng.
  - **Vì sao**: đo trên ba bộ khung chạy cùng một prompt và cùng một model (02/10/2026). Bộ khung tự
    kiểm chứng rồi sửa thì xong việc một mình (36 lần sửa sau khi đọc lại, không cần ai nhắc); bộ khung
    ghi một lần rồi trả lời thì phải có người nhắc mới chạy tiếp.
  - Luot CHỈ ĐỌC không tốn thêm vòng nào: `tool.ChangesDocument` quyết định theo tên lệnh. Lệnh lạ
    (kể cả lệnh thêm sau này) mặc định tính là "có sửa" — đoán sai theo hướng đó chỉ tốn một vòng đọc
    lại, còn đoán sai hướng ngược lại thì bỏ qua bước kiểm chứng của một lần ghi thật.
  - Kiểm chứng chạy đúng MỘT lần mỗi lượt, không quay vòng vô tận. Tắt được bằng `VerifyWorkEnabled=0`
    (Cài đặt của pane, hoặc `AXIOM_VERIFY_WORK=0`).
- **Hiện phần suy luận của model cho người dùng** (`LlmShowReasoning`, mặc định tắt). Bật thì Core phát
  thêm sự kiện `run.reasoning` (`round`, `text`, `model`); pane vẽ một khối mờ "Suy luận · vòng N", suy
  luận ngắn thì mở sẵn, dài thì thu thành một dòng bấm "Hiện" để xem đủ. Mỗi lượt một khối, mỗi vòng ghi
  đè bằng phần mới nhất nên khung chat không bị lấp.
  - Vẫn TẮT mặc định vì bật lên thì model dồn `max_tokens` vào phần nghĩ trước khi trả lời (đã đo: có
    lần ra câu trả lời rỗng với `finish_reason=length` trên prompt lớn) — đổi lại người dùng thấy được
    quá trình suy nghĩ.
- **Đo được prompt caching**: `Turn.CachedTokens` đọc cả ba kiểu tên (`prompt_cache_hit_tokens` của
  DeepSeek, `prompt_tokens_details.cached_tokens` của OpenAI, `cache_read_input_tokens` của Anthropic),
  cộng dồn vào `AgentResult.CachedTokens`, ghi ra log lượt chạy và trường `cachedTokens` của
  `GET /v1/runs/{id}`. Không có gì phải gửi lên để bật cache (cả ba nhà cung cấp đều cache ngầm theo
  tiền tố ổn định) — số này để BIẾT tiền tố của mình có ổn định thật không, thay vì đoán.
- `Run.Verified` + trường `verified` trong `GET /v1/runs/{id}`: lượt này có sửa tài liệu và đã được yêu
  cầu tự kiểm chứng.
- Hướng dẫn khảo sát trước và đọc lại sau khi sửa vào system prompt (`agent.ReconRule`, `agent.CheckRule`).

### Fixed — pane LibreOffice trên Windows không tự khởi động được Agent Core
- `scripts/libreoffice.ps1 -Install` nay điền `CoreExe` vào `HKCU\Software\AxiomOffice` (chỉ khi đang ở
  trong cây mã nguồn đã build, và không ghi đè lựa chọn sẵn có của người dùng). Trước đây trên Windows
  không bước nào điền giá trị này — khác Linux, nơi `install.sh` đặt Core vào `<data_dir>/core` — nên
  `core.core_exe()` trả rỗng và pane không tự khởi động được Core; phải tự gõ đường dẫn vào Cài đặt.

### Changed — bỏ trần thời gian của một lượt agent
- **Không còn trần đồng hồ cho một lượt chạy.** Đợt trước trần là 300s mặc định (tối đa 900s), và nó
  cắt ngang task dài trước khi agent kịp làm gì. Cả hai dự án tham khảo (opencode, goclaw) đều không
  có wall-clock cho một lượt — họ kiểm soát bằng số vòng, số tool call và ngân sách ngữ cảnh.
- Thay bằng `options.maxRounds` (mặc định **100**, tối đa 1000) + ngân sách token + nút **Dừng**.
  `options.maxSeconds` vẫn dùng được khi bên gọi tự muốn đặt (tối đa 86400s).
- Ba client đổi theo, có chủ ý khác nhau: **pane** (add-in Windows `CoreClient.cs`, extension
  `panel.py`) **không gửi trần** vì đi theo SSE và có nút Dừng; **bước "Thử ngay"** của wizard tự đặt
  120s vì nó chờ đồng bộ; **`ai.ask`** (`general.py`) vẫn giữ 300s vì là RPC đồng bộ cho agent bên
  ngoài — không thể chờ vô hạn.
- `DefaultDeadline` bỏ khỏi `internal/model` (không còn ai dùng).


### Changed (BREAKING nội bộ) — MCP server bản Linux chuyển sang Go, bỏ dự án .NET 10
- **Bản Windows không đổi**: MCP server vẫn là `AxiomOffice.Host.exe mcp` (net48, `src/AxiomOffice.Host/Mcp/`).
- **Bản Linux nay là subcommand của chính Core**: `AxiomOffice.Core mcp [all|word|excel|ppt] [--list]`
  (`core-go/internal/mcpserver/`) — gói Linux không còn file MCP riêng, máy đích vẫn không cần runtime nào.
  Subcommand được chặn **trước** mutex một-phiên-bản (Core sinh tiến trình con bằng chính binary của nó).
- Xoá `src/AxiomOffice.Mcp` (dự án .NET 10 + `Compat/BridgeCompat.cs` + `Compat/PortableJson.cs`) và dọn các
  nhánh `#if PORTABLE` trong mã C# dùng chung; bản C# còn đúng một cấu hình net48.
- **Hợp đồng tool giữ nguyên**: cùng 50 tool (20 file docx/xlsx/pptx/csv + 30 live + `office_sessions`), cùng
  tên, cùng tham số, cùng câu chữ mô tả, cùng cách trả lỗi (`isError` cho lỗi trong tool, `-32602` cho tool
  lạ). `mcp.LoadConfigs` không đổi: server built-in `office` vẫn là `{command, args: ["mcp"]}`, chỉ khác
  `command` theo nền tảng (hàm `officeMcpHost`).
- Engine OOXML viết lại bằng Go, port trung thành từ `Host/Mcp/*.cs`: `internal/ooxml` (zip/part/rels/
  content-types, chỉ ghi lại part bị sửa nên chart/ảnh/pivot/macro giữ nguyên), `internal/xlsx`,
  `internal/docx`, `internal/pptx`, `internal/sheet` (A1, CSV/TSV), `internal/filesafe`, `internal/textutil`.
  Hai regex lookbehind của bản C# (`FormulaShift`, `ReplaceSheetReference`) được thay bằng quét chỉ số và tự
  kiểm tra ký tự kề, vì RE2 của Go không hỗ trợ lookbehind.
- Giữ nguyên **có chủ ý** hai hành vi trông như lỗi của bản C# vì chúng nằm trong hợp đồng parity:
  `ReplaceSheetReference` thay dạng có nháy bằng tên mới không nháy khi tên đơn giản, và `IsDateFormat` chỉ
  xét phần trước dấu `;` đầu tiên. Riêng thông báo `excel_convert to='parquet'` đổi từ "in the C# server"
  sang "in this server" cho đúng sự thật (bộ test chỉ kiểm chữ "parquet").
- `catalog/live-commands.json` là nguồn duy nhất cho danh sách lệnh bridge (trước ở
  `src/AxiomOffice.Mcp/live-commands.json`); `scripts/generate_mcp_commands.py` sinh thêm
  `core-go/internal/mcpserver/livecommands_gen.go`. Template `default.docx`/`default.pptx` vẫn một nguồn ở
  `src/AxiomOffice.Host/Mcp/Templates/`, bản Go giữ bản sao trong `core-go/internal/templates/` và
  `scripts/generate_templates.py --check` (CI chạy) báo lỗi nếu lệch.
- CI: job **linux** bỏ hẳn .NET (không còn `dotnet build`), kiểm ba file sinh không lệch nguồn, chạy e2e
  **14 phần** (đã gồm `mcp`, vì Core tự làm MCP server) và bộ MCP portable nhắm vào binary Go; job **windows**
  bỏ .NET SDK, chạy bộ MCP portable nhắm vào `AxiomOffice.Host.exe mcp`.
- `scripts/linux/package.sh` bỏ bước `dotnet publish`; `scripts/linux/install.sh` in cấu hình `mcpServers`
  trỏ vào `core/AxiomOffice.Core` với `args: ["mcp", "all"]` và dọn thư mục `mcp/` của bản cài cũ.
- Kiểm chứng đợt này: `test_mcp_portable.py` **61/61** cho cả binary Go và `AxiomOffice.Host.exe mcp`;
  `test_mcp_host.py` (parity sâu với python-docx/openpyxl/python-pptx) **103/103** cho binary Go;
  `tests/mcp-host/oracle_diff.py` (công cụ mới: chạy cùng kịch bản trên bản Go **và** bản C#, so từng bước)
  **61/62**, một bước khác chỉ vì câu chữ thông báo `parquet`.

### Docs — cài đặt trên máy mới tinh (Windows + Ubuntu)
- README mục **Cài đặt cho người dùng** nay nói rõ máy đích không phải cài gì thêm, và điều kiện dễ bỏ sót
  nhất là **bitness**: add-in đóng gói x64 nên Office/WPS 32-bit cài xong vẫn không thấy tab — kèm lệnh
  kiểm tra `(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Office\ClickToRun\Configuration').Platform`. Phần
  đóng gói bổ sung: Windows không có artifact CI nên phải chạy `scripts\package.ps1`; máy build thiếu Go 1.26+
  thì gói vẫn ra nhưng không kèm `AxiomOffice.Core.exe`; đang mở Word/Excel/WPS thì build dừng lại và liệt kê
  tiến trình đang giữ DLL (chỉ dùng `-Kill` khi chấp nhận để script tự tắt app đó)
- README mục **Linux** thêm đường đi cho **máy Ubuntu mới tinh**: hai lệnh `apt` (LibreOffice + `python3-uno`,
  tuỳ chọn `libsecret-tools` cho keyring), lấy gói từ artifact CI `axiom-office-linux-x64` hoặc
  `scripts/linux/package.sh`, rồi `./install.sh` **không cần tham số** (cấu hình AI bằng wizard). Thêm cảnh
  báo **LibreOffice bản snap/flatpak không được hỗ trợ** (sandbox chặn `127.0.0.1` tới Agent Core và quyền
  chạy binary ngoài) kèm cách kiểm tra `readlink -f "$(command -v soffice)"`, và ghi chú endpoint nội bộ phải
  có đường tới trước khi bấm "Kiểm tra kết nối"
- `scripts/dist/HUONG-DAN-CAI-DAT.txt` (phát cho người nhận gói): thêm mục "Kiểm tra Office 64-bit" và hai
  dòng sự cố — không thấy tab vì app 32-bit, và "Kiểm tra kết nối" báo lỗi mạng khi máy chủ AI ở trong mạng
  công ty mà máy chưa vào được
- Bỏ phần trùng lặp trong README (cách đóng gói Linux và "bỏ qua tham số LLM" được nhắc hai lần sau khi thêm
  mục máy mới tinh)

### Changed — Wizard thiết lập (Windows): soi lại giao diện từ ảnh render thật
- **Có ảnh để soi**: thêm `tests/ui/shots.ps1` + `tests/ui/SetupWizardShots.cs` — biên dịch harness chung với
  mã nguồn add-in (không đụng `bin\Release`, không cần Office/WPS), mở `SetupWizardForm` thật, nhảy qua 5
  bước rồi lưu PNG vào `tests/ui/out/` (không commit). Mọi thay đổi bố cục/màu/chữ từ nay so được trước–sau
- **Bảng điều khiển WinForms trộn hai ngôn ngữ thị giác**: TextBox/CheckBox mặc định vẽ viền vuông + tick 3D
  trong khi nút của Axiom vốn phẳng bo góc, nhìn như hai sản phẩm ghép lại. Thêm `Ai/SetupWizardControls.cs`
  (tách khỏi `PaneControls.cs` đang dùng chung với pane): `FieldBox` (ô nhập bo góc 1px, viền đổi màu khi
  focus), `ToggleBox` (checkbox phẳng, bấm được cả nhãn, Space để bật/tắt), `StatusBadge` (đĩa tròn + ✓/!/✗
  thay cho ký tự trần trông như lỗi gõ), `CardBox` (thẻ bo góc theo sắc thái trung tính/thành công/cảnh
  báo/lỗi) và `StepBar` (5 đoạn indigo thay cho `●○○○○` ở cỡ 8.25pt — khó thấy và trùng nghĩa với chữ
  "bước 2/5" bên cạnh)
- Bước 1–5 chỉnh theo: tiêu đề/phụ đề dùng cỡ chữ riêng (`PaneTheme.DialogTitle/DialogSubtitle/FieldLabel`),
  đường kẻ 1px trên hàng nút, ô nhập có nhãn đậm phía trên (bỏ nhãn nằm lẫn trong placeholder), 4 nhà cung
  cấp xếp lưới 2×2 đều nhau (tính theo số nhà cung cấp, không hardcode), Enter ở bất kỳ ô nào = "Kiểm tra kết
  nối", bước kiểm tra máy đổi thành danh sách thẻ (huy hiệu + kết luận + chi tiết + nút sửa trong thẻ, nút
  "Kiểm tra lại" xuống dưới danh sách), bước 5 tóm tắt dạng cặp nhãn/giá trị (4 dòng: Model · Máy chủ AI ·
  Ghi nhớ dài hạn · Agent Core — trước đây dồn thành một đoạn văn có dấu "·" nên khó quét) và nút "Thử ngay
  trên tài liệu đang mở" giãn hết chiều ngang
- Sửa lỗi nhìn thấy được khi render: thẻ kết quả thử kết nối bị hàng nút cắt mất chân, lưới chip nhà cung
  cấp tràn chữ khi có 4 mục, và chữ trong thẻ kết quả dính lên mép trên (nay canh giữa)
- Nút "← Quay lại" trên Windows đã có sẵn và vẫn bấm được (lỗi nút này là của bản Linux, xem mục ở dưới)

### Removed — bỏ Agent Core bản .NET (C#) khỏi repo
- Xoá `src/AxiomOffice.Core` (53 file / 8.393 dòng) và `tests/core/AxiomOffice.Core.Tests` (22 file / 3.624
  dòng, 228 test xUnit) sau khi bản Go đã qua **cả hai** bộ e2e (155/155 mỗi bản) và 159 test trên LibreOffice
  thật. `scripts/install-dotnet-sdk.ps1` cũng bỏ (chỉ tồn tại để build bản .NET)
- **Port trước, xoá sau**: bốn vùng chỉ bản .NET có test đã được chuyển sang Go trước khi xoá — `corelog`
  (định dạng dòng, ghi lỗi kèm tên kiểu qua `corelog.ErrorDetail`, xoay vòng file), `corefile` (`core.json`
  ghi atomic, file thiếu/hỏng, xoá lại lần hai), `policy` (luật xác nhận lưu/xuất, ghi đè, `replaceAll` trên
  tài liệu dài) và chọn port + khoá một-phiên-bản (`cmd/axiom-core`, `internal/instance`)
- Cắt các dây nối: `build.ps1` bỏ tham số `-Core` và nhánh `dotnet publish`; `package.sh` bỏ `AXIOM_CORE=dotnet`;
  CI bỏ hai bước `dotnet test tests/core/AxiomOffice.Core.Tests` và bước so sánh `prompts/extract.txt`
  (nay chỉ còn một bản, nhúng bằng `go:embed`); `catalog/setup.json` giờ chỉ sinh catalog cho add-in + extension + Go
- Tài liệu cập nhật theo: README (cây thư mục, lệnh test, yêu cầu build), `STATUS.md`, `core-go/README.md`
  và một ghi chú ở `New_arch.md` (phần .NET còn lại là thiết kế ban đầu)

### Fixed — Wizard thiết lập (Linux): nút "Quay lại" bấm không được
- **Gốc bệnh**: ở hàng nút dưới cùng, nhãn trạng thái (`FixedText` nền trắng, rộng `WIDTH - 2*PAD -
  2*BUTTON_W - 24` = 284 px, đặt tại `PAD`) trùm trọn nút "Quay lại" (rộng 110 px, cùng hàng). Điều khiển
  đè lên điều khiển bấm được thì nuốt hết cú bấm ở vùng đó — VCL còn nâng nó lên mỗi lần đổi `Label` — nên
  nút "Quay lại" chết hẳn. Đo trên LibreOffice thật bằng xdotool: bấm "Tiếp tục →" ăn (đổi bước), bấm "Quay
  lại" ở 5 điểm khác nhau đều không, và `mouseEntered` của nút không hề chạy (không có hiệu ứng hover).
  `ui.setup` không bao giờ lộ ra lỗi này vì nó đổi bước bằng API chứ không bằng chuột
- Hàng nút nay **chỉ còn điều hướng** (`← Quay lại` · `Để sau` · `Tiếp tục →`); nhãn trạng thái được đưa ra
  khỏi hàng nút, xuống đáy vùng nội dung — xem mục "chỗ của câu trạng thái" bên dưới. Nhờ vậy không còn điều
  khiển nào chồng lên nút bấm được trong hàng nút nữa
- **Vì sao mở wizard lại thấy bước 2/5**: máy đã có `LlmEndpoint` + `LlmModel` thì `_apply_collect` cố ý
  nhảy `welcome → checks` để vào thẳng màn kiểm tra / sửa lỗi — nên "Quay lại" không bấm được là mắc kẹt
  hẳn ở đó. Cái nhảy bước này nay đã bỏ (xem mục "bước mở đầu" bên dưới)
- Kiểm thử: `tests/live/test_setup_wizard.py` thêm `click_in_window` (bấm chuột THẬT theo toạ độ model, tự
  trừ lề cửa sổ) và 2 kiểm tra: về lại bước kiểm tra máy, rồi bấm "Quay lại" phải lui về bước chào mừng

### Changed — Wizard thiết lập: luôn mở ở bước 1/5 (Chào mừng)
- Trước đây máy đã có `LlmEndpoint` + `LlmModel` thì `_apply_collect` (Linux) và `ShowStep(Configured() ? 1 : 0)`
  (Windows) tự nhảy sang bước 2/5 để vào thẳng màn kiểm tra / sửa lỗi. Người dùng mở wizard ra là thấy mình ở
  bước 2/5 mà không rõ vì sao
- Nay wizard **luôn mở ở bước 1/5**; kết quả kiểm tra vẫn chạy nền và điền sẵn vào các dòng của bước 2, nên
  bấm "Tiếp tục →" là thấy ngay. Hai bản (LibreOffice + Windows) sửa cùng lúc cho khớp nhau
- `SetupState.configured` (chỉ dùng cho việc nhảy bước này) đã bỏ; bên C# `Configured()` vẫn giữ vì còn dùng
  để đổi tiêu đề bước kiểm tra khi máy đã thiết lập
- Kiểm thử: `tests/live/test_setup_wizard.py` chặt lại — mở wizard phải đúng bước `welcome`, và mở lại sau khi
  bấm X cũng phải về bước 1/5 (trước đây chấp nhận cả `checks`)

### Changed — Wizard thiết lập (Linux): chỗ của câu trạng thái ("Mọi thứ đều ổn.")
- **Câu đó là gì**: `_status_message()` — câu kết luận của màn "Kiểm tra máy", liệt kê những mục chưa đạt.
  Trước đây nó nằm ở hàng nút, nên (a) chỉ được ~250 px giữa nút "Quay lại" và link "Để sau" nên câu dài bị
  cắt bằng "…", (b) chen vào chỗ của điều hướng, và (c) chính nó là thứ nuốt cú bấm vào nút "Quay lại"
- **Chỗ mới**: đáy **vùng nội dung** (`STATUS_Y/STATUS_H = 420, 40`, rộng hết `WIDTH - 2*PAD`), ngay dưới
  danh sách kiểm tra mà nó nói về — đúng nguyên tắc "câu kết luận đặt cạnh thứ nó kết luận", và đủ hai dòng
  nên câu dài nhất không còn bị cắt. Vùng 420..462 không bước nào dùng đến; hàng nút chỉ còn điều hướng
- Bước "Kết nối máy chủ AI" **không dùng** dòng này: nó đã có dòng kết quả riêng (`test_line`, 414..456) và
  gợi ý cạnh nút "Tải danh sách model" (`models_hint`) — `_show_step` ẩn dòng trạng thái ở bước đó
- **Không còn nói "Mọi thứ đều ổn." khi chưa kiểm tra gì**: dòng nào chưa có kết quả (`ok is None`) thì
  chưa kết luận, để trống. Trước đây mở wizard là thấy ngay câu đó dù máy chưa được kiểm tra
- Lỗi Core vẫn được ưu tiên hiện trước danh sách (nó là nguyên nhân của phần lớn các dòng còn lại), nhưng
  nay hiện đủ hai dòng thay vì bị cắt
- Phần quyết định câu chữ tách thành hàm thuần `setup.checks_verdict()` (có test trong `tests/lo/test_setup.py`)
- **Sửa kèm**: câu kết quả sau khi bấm "Sửa" (`_after_fix`) và câu "Đã lưu. …" trước đây ghi thẳng vào nhãn
  rồi bị lần vẽ lại ngay sau đó xoá mất, nên người dùng không bao giờ đọc được; nay đi qua `_set_note()`
  (thông báo ngắn hạn, tự tắt khi đổi bước). Câu "Điền địa chỉ máy chủ AI trước." của nút "Tải danh sách
  model" cũng chuyển về `models_hint` — cạnh đúng thứ nó nói tới
- Kiểm thử: `tests/live/test_setup_wizard.py` đọc thêm `statusLine` từ `ui.setup` và kiểm tra dòng trạng thái
  ở bước kiểm tra máy là câu kết luận (không phải câu tiến độ), còn ở bước kết nối thì để trống

### Fixed — Calc (LibreOffice): công thức nhiều tham số bị Err:508, và lỗi công thức bị báo sai
- **Gốc bệnh**: bản LibreOffice 24.2 trên máy này đòi `;` làm dấu phân cách tham số, còn agent viết công
  thức theo cú pháp en-US (dấu `,`) → mọi công thức từ hai tham số trở lên (`=ROUND(AVERAGE(C2:E2),1)`,
  `=IF(...)`) đều ra `Err:508` "pair missing". Mục `Tools ▸ Options ▸ Calc ▸ Formula ▸ Separators` để đổi
  đã bị bỏ ở bản này, nên không sửa được từ phía cấu hình. Excel qua COM thì luôn nhận `,` bất kể locale —
  bản Windows không dính lỗi này, nên chỉ sửa phía LibreOffice
- `et.writeRange` nay tự kiểm tra: sau khi ghi công thức, đọc `cell.getError()`; gặp đúng lỗi cú pháp 508
  thì thử lại với dấu phân cách còn lại (bộ đổi dấu bỏ qua phần trong chuỗi `"..."`, kể cả ngoặc kép đôi
  `""`, để dấu phẩy trong `"Có, dữ liệu"` không bị đổi). Dấu phân cách dò được nhớ lại cho các ô sau trong
  cùng một lần ghi. Chỉ đổi dấu khi gặp 508 — lỗi lúc tính (`#DIV/0!` = 532) nghĩa là dấu đã đúng, đổi sang
  kiểu kia chỉ làm công thức hỏng thêm; đổi mà không cứu được thì trả lại đúng bản agent đã viết
- **`et.writeRange` không còn im lặng**: công thức nào còn lỗi sau khi thử thì trả về ở trường
  `formulaErrors` (`{cell, formula, error, text}`) thay vì chỉ `{"written": n}`. Trước đây agent ghi 11 ô
  `Err:508` mà kết quả vẫn báo thành công
- `et.checkRange` in **mã lỗi thật** của từng cột (`column E has 1 error value(s): #DIV/0!`) thay vì chuỗi
  ví dụ cứng `(#DIV/0!, #REF!...)` — chuỗi đó khiến người đọc (và agent) đi tìm sai bệnh. Bản C# đổi tương
  ứng: thêm `ExcelErrorName` (mã lỗi Excel → `#DIV/0!`, `#REF!`...), giữ hai bản khớp nhau
- Skill `bao-cao-du-lieu`: nói rõ viết công thức theo cú pháp en-US và **đọc `formulaErrors`** sau khi ghi,
  đừng coi `"written": n` là xong
- Kiểm thử: `tests/lo/test_calc.py` mới (**12** ca: đổi dấu phân cách, dấu phẩy trong chuỗi, lỗi lúc tính
  không bị đổi dấu, đổi dấu vô ích thì trả lại bản gốc, `formulaErrors` trong kết quả) — cả bộ `tests/lo`
  nay **101** test; `tests/live/test_live_commands.py` thêm **3** kiểm tra trên LibreOffice thật: công thức
  nhiều tham số ghi bằng dấu phẩy không lỗi, tính đúng giá trị, `checkRange` không báo `error-values`

### Fixed — Wizard thiết lập (Linux): chọn model trong danh sách, bấm X, và kết quả thử cũ
- **Chọn model trong danh sách không có tác dụng**: `Wizard._selected_model` đọc thuộc tính
  `SelectedItemPos` của *model*, nhưng LibreOffice trả về `None` ở đó dù người dùng đã bấm chọn — lựa chọn
  thật nằm ở control (`XListBox.getSelectedItemPos`, trả về -1 khi chưa chọn). Nay đọc từ control, nên bấm
  một model trong danh sách thật sự đổi model đang dùng
- **Kết quả "Kiểm tra kết nối" cũ không hết hiệu lực khi đổi giá trị**: `_read_fields` gán thẳng
  `state.endpoint/model/api_key` thay vì đi qua `SetupState.update_fields`, và danh sách model không có
  listener nào. Hậu quả: thử kết nối với model A thành công → chọn model B (hoặc gõ địa chỉ khác) → nút chính
  vẫn ghi "Tiếp tục →" → bấm là **lưu luôn giá trị chưa hề được thử**. Nay mọi đường đổi giá trị (gõ tay, đổi
  nhà cung cấp, chọn trong danh sách) đều đi qua `update_fields` và làm kết quả thử cũ hết hiệu lực; thêm
  `awt.ItemListener` cho ListBox model
- Gõ tay tên model thì **bỏ mục đang chọn** trong danh sách, để danh sách không ghi đè ngược lại lúc bấm
  "Tiếp tục"
- **Bấm X trên thanh tiêu đề không đóng được wizard**: cửa sổ tạo bằng toolkit không tự đóng khi bấm X. Thêm
  `awt.TopWindowListener` (`Dialog.on_close`) nên X đóng thật, và wizard được dọn khỏi `CURRENT` để lần sau
  mở ra là một wizard mới
- `Dialog.is_alive()` / `Dialog.to_front()`: mở lại wizard đang nằm sau cửa sổ LibreOffice thì đưa lên trước
  thay vì mở chồng
- Dialog **Tuỳ chọn nâng cao** mở từ wizard nay là cửa sổ rời (`floating=True`): cửa sổ con nằm dưới wizard
  nên bị che, và trên Linux cửa sổ con không nhận chuột
- Kiểm thử: `tests/lo/test_setup.py` **89** unit test (thêm 4 ca cho `next_label`/`update_fields`);
  `tests/live/test_setup_wizard.py` **20** kiểm tra trên LibreOffice thật (thêm 8: nhãn nút chính, kết quả thử
  cũ hết hiệu lực khi đổi địa chỉ / chọn model trong danh sách / gõ tay, dialog nâng cao là cửa sổ rời, bấm X
  đóng hẳn wizard)

### Fixed — Core dừng khi đang chạy lượt: agent phải dừng sửa tài liệu (`core-go/`)
- Bản Go trước đây không hủy các lượt đang chạy lúc Core dừng (bản .NET làm trong `ApplicationStopped`),
  nên agent có thể còn gửi lệnh xuống bridge sau khi Core đã tắt → tài liệu bị sửa dở. Nay Core hủy mọi
  lượt đang chạy **trước khi** đóng HTTP server (pane còn nhận được `run.cancelled`) rồi chờ tối đa 2s cho
  các lượt dừng hẳn
- Thêm phần e2e `shutdown` (dùng được cho cả hai bản): dừng Core giữa lúc agent đang ghi tài liệu → bridge
  không nhận thêm lệnh nào và tiến trình Core thoát

### Added — Hai vùng chức năng trước đây chưa có e2e: codec Anthropic và embedding
- Phần e2e **`anthropic`**: chạy một lượt thật qua `/messages` với máy chủ giả trong tiến trình — kiểm tra
  `x-api-key` + `anthropic-version`, `system` là trường riêng (không có message role system), tool theo dạng
  `input_schema`, `max_tokens`, vòng `tool_use` → `tool_result` giữ đúng `tool_use_id`, và ảnh chụp màn hình
  đi trong `tool_result` dạng khối `image` base64. Trước đây codec Anthropic chỉ được test đơn vị, chưa
  chạy qua lượt thật
- Phần e2e **`embeddings`** (`EmbeddingModel`): máy chủ `/embeddings` giả trả vector điều khiển được →
  kiểm tra chống trùng bằng cosine ≥ 0.96 (khác hash vẫn bị coi là trùng), vector khác thì thêm mới, và tìm
  được theo **nghĩa** dù truy vấn không chung từ khoá nào; mỗi lần ghi gửi cả lô trong một request
- Phần e2e **`summarize`**: 21 lượt trong cùng một hội thoại → Core tóm tắt ở hàng đợi nền, **chỉ khi vượt
  mốc 20 tin nhắn** (không tốn một lần gọi model mỗi lượt), và bản tóm tắt được đưa vào ngữ cảnh lượt sau
  thay cho các lượt cũ
- Phần `confirm` thêm ca **`interactive=false`** (đường `ai.ask` của agent bên ngoài, MCP, script): lệnh rủi
  ro bị từ chối **ngay** và không xuống bridge, thay vì chờ hết hạn xác nhận 8 giây
- Phần `memory` thêm các ca **PATCH** (đường dialog "Quản lý ghi nhớ" trên cả hai nền tảng): sửa nội dung,
  ghim/bỏ ghim, đặt và bỏ hạn dùng, hạn dùng sai định dạng → 400, id không tồn tại → 404, lịch sử ghi
  UPDATE/PIN/EXPIRES
- Phần e2e **`cancel`** (nút Dừng của pane): hủy giữa lúc agent đang ghi tài liệu → SSE kết thúc bằng
  `run.cancelled`, agent không sửa tài liệu nữa, `GET /v1/runs/{id}` báo `cancelled`, hủy lại → 409, và Core
  vẫn khỏe để chạy lượt mới. Phần `memory` thêm ca xóa cứng toàn bộ (`?scope=all&confirm=true`, thiếu
  `confirm` → 400) — nút "Xóa toàn bộ ghi nhớ" trong dialog
- Phần e2e **`mcp_http`**: MCP qua **Streamable HTTP** (`mcp.json` dạng `url`) — bắt tay `initialize`, giữ
  `Mcp-Session-Id` cho các request sau, gửi kèm `MCP-Protocol-Version`, đọc được cả response dạng JSON lẫn
  dạng SSE, tool đặt tên `mcp__<server>__<tool>` và vào audit. Trước đây chỉ transport stdio có test
- Bộ e2e giờ **155 kiểm tra** (14 phần chạy riêng được), tất cả đều xanh trên **cả hai bản** (.NET và Go);
  CI (Linux) chạy 13 phần (143 kiểm tra)

### Added — Wizard thiết lập cho người dùng không chuyên (Windows + Linux)
- **Một nội dung, hai bộ vẽ**: câu chữ + preset nhà cung cấp nằm ở `catalog/setup.json`, sinh ra
  `Setup/SetupCatalog.cs` (dùng chung add-in Windows và Agent Core) cùng `axiom/setup_catalog.py`;
  `scripts/generate_setup_catalog.py --check` chạy trong CI nên ba bản không thể lệch
- **Core API cho wizard**: `GET /v1/setup` (preset + bước + tính năng + việc cần làm + cấu hình đang chạy),
  `POST /v1/llm/test` (thử kết nối với giá trị *chưa lưu*), `GET /v1/llm/models` (danh sách model của máy chủ
  để chọn thay vì gõ tay); `Setup/LlmErrors.cs` dịch lỗi (401/403/404/429/5xx, DNS, mạng, hết giờ…) thành
  câu tiếng Việt kèm gợi ý sửa. Khoá API không bao giờ vào log hay response
- **Windows** (`Ai/SetupWizardForm.cs` + `Ai/CoreSetup.cs`): 5 bước Chào mừng → Kiểm tra máy (mỗi dòng có
  nút **Sửa**: khởi động lại Core, tạo khoá mới, sang bước kết nối) → **Kết nối máy chủ AI** (chọn máy chủ
  công ty/OpenAI/Anthropic/Gemini, tải danh sách model, *Kiểm tra kết nối* báo lỗi tiếng Việt, có nút xem
  khoá và mở trang lấy khoá) → Tính năng (lời thường + link **Tuỳ chọn nâng cao…** mở đúng `SettingsForm`
  cũ) → Hoàn tất (tóm tắt + **Thử ngay** chạy một lượt thật qua Core trên tài liệu đang mở). Điểm vào:
  ribbon `Settings` → **Thiết lập…** (thêm **Cài đặt nâng cao…** cho ngang bản Linux), link header pane
  `Cài đặt` → **Thiết lập**, và tự mở **một lần** khi
  pane mở mà chưa có endpoint/model (giống bản Linux, không làm phiền lần sau)
- **Linux**: `axiom/setup.py` (máy trạng thái + câu chữ, thuần Python nên test không cần LibreOffice) và
  `axiom/setupwizard.py` (dialog awt, cùng bố cục 5 bước); mục menu **Thiết lập…**, link header, thẻ mời tự mở
  một lần; tự sửa: khởi động lại Core, sinh `Token`, `chmod 0600`, gợi ý khi thiếu `python3-uno`
- Kiểm thử: `tests/lo/test_setup.py`, `SetupTests.cs` (xUnit) và `test_setup` trong e2e cho Core API;
  `tests/live/test_setup_wizard.py` bấm thật trong phiên X ảo (12 kiểm tra); CI chạy cả ba
- Ghi chú: wizard chỉ lo **cấu hình + tự sửa lỗi**, không thay installer — người dùng Linux vẫn chạy
  `install.sh` một lần như trước

### Changed — Agent Core phát hành bằng bản Go (`core-go/`, giai đoạn G6)
- `scripts/build.ps1` build Agent Core từ `core-go/` thành `AxiomOffice.Core.exe` (~11 MB, không cần .NET
  runtime trên máy người dùng; trước đây ~48 MB); `-Core dotnet` giữ đường cũ để đối chiếu
- `scripts/linux/package.sh` đóng gói `core/AxiomOffice.Core` từ bản Go (Go 1.26+, build chéo theo `--rid`);
  `AXIOM_CORE=dotnet` để đóng gói bản .NET
- Bản .NET vẫn nằm nguyên trong `src/AxiomOffice.Core` và bộ test xUnit của nó vẫn chạy: hai bản dùng chung
  `core.json`, `core.db` (schema 2) và Core API v1 nên đổi qua lại không mất dữ liệu hay phải cấu hình lại
- Kiểm chứng: e2e **94/94** với binary Go đặt đúng chỗ phát hành (`src\AxiomOfficein\Release\AxiomOffice.Core.exe`),
  82/82 trên Linux (trừ phần `mcp` cần `AxiomOffice.Host.exe`), và đóng gói Linux chạy được với `install.sh`

### Added — Agent Core bản Go, giai đoạn G4+G5: memory dài hạn, MCP client, QA thị giác (`core-go/`)
- **G4 — memory**: chuẩn hoá/bỏ dấu tiếng Việt (bảng tường minh như bản .NET), hash chống trùng, kho SQLite
  (thêm có chống trùng theo hash và theo cosine ≥ 0.96, liên kết chỉ tới memory còn sống, lịch sử ADD/UPDATE/
  PIN/UNPIN/EXPIRES/DELETE/RESTORE, ghim, hạn dùng, xoá mềm + dọn sau 30 ngày), truy hồi theo bm25 (FTS5)
  + entity boost + sigmoid như mem0 2.2.1, trích xuất sau mỗi lượt bằng LLM (bỏ qua prompt quá ngắn/lệnh thao
  tác thuần, lọc confidence/nhạy cảm/id bịa), tool `remember`/`recall`, và toàn bộ `/v1/memory` (tìm không
  dấu, sửa, xoá, khôi phục, lịch sử, xoá cứng có xác nhận)
- **G5 — MCP client**: transport stdio (JSON-RPC một dòng mỗi thông điệp, tự trả lời -32601 khi server hỏi
  ngược) + Streamable HTTP, `initialize`/`tools/list` có phân trang, tên tool `mcp__<server>__<tool>`, mcp.json
  nạp lại khi đổi, server lỗi bị ẩn tool kèm lý do; **server built-in `office`** = `AxiomOffice.Host.exe mcp`
  cạnh binary nhưng chỉ lấy tool đọc/ghi file (tool live trùng `office_action` bị loại); xác nhận trước khi
  gọi server ngoài (không tin cậy) và trước khi ghi đè file đã có; `/v1/mcp` liệt kê server + tool + lỗi
- **G5 — QA thị giác**: `look_at_document` chụp cửa sổ app qua bridge rồi gửi ảnh thật cho model (chỉ khi
  bật `VisualQaEnabled`), ảnh không vào audit
- Bản Go qua **94/94** kiểm tra e2e (bằng đúng con số của bản .NET, chạy trên Windows); trên Linux qua 82/82
  phần chạy được không cần `AxiomOffice.Host.exe`; `go test ./...` phủ thêm memory, mcp, office
- CI: job `windows` chạy trọn bộ e2e của bản Go (kèm Host.exe như gói phát hành), job `linux` chạy 7 phần và
  so sánh `prompts/extract.txt` giữa hai bản

### Added — Agent Core bản Go, giai đoạn G2+G3: lượt chạy agent và skills (`core-go/`)
- **G2 — lượt chạy**: `POST /v1/runs` (+ `GET /v1/runs/{id}`, `/cancel`, `/confirm`, SSE `events` với `?after=`),
  `GET /v1/audit`, `GET/DELETE /v1/conversations…`; orchestrator đầy đủ (kiểm tra session + `/health` của
  bridge, hội thoại theo tài liệu, ngữ cảnh 8000 token, tóm tắt khi vượt mốc 20 tin nhắn, allowlist lệnh qua
  `GET /commands`, ghi audit + transcript, `run.started|tool.*|run.completed|failed|cancelled|timedout|stopped`),
  `office_action` (chuẩn hoá tên lệnh viết sai nhẹ, `params` dạng chuỗi JSON), policy xác nhận
  (tự lưu/tự xuất, ghi đè `saveAs`, `wpp.deleteSlide`, `writer.replaceAll` trên tài liệu dài;
  `interactive:false` = từ chối ngay) và session registry (`flock`/mở handle để biết tiến trình còn sống)
- **G3 — skills**: `GET /v1/skills?app=…`, `POST /v1/skills/reload`, `load_skill`, `read_skill_file`
  (chặn `..`, đường dẫn tuyệt đối, symlink ra ngoài, giới hạn 64KB), frontmatter YAML (danh sách `- item`
  và `[a, b]`, khối `>`/`|`, nháy đơn/kép), nguồn `builtin` → `org` → `user` ghi đè theo thứ tự, gói tài
  nguyên `_design`, quét lại thư mục khi có thay đổi (Go không có FileSystemWatcher nên so dấu vết file mỗi 2s)
- Bản Go qua **60/60** kiểm tra e2e của các phần `fake_bridge`, `guards`, `skills`, `confirm`, `setup`
  (Windows và Linux); `go test ./...` phủ config, model/codec, dịch lỗi, guard + `/v1/setup`, skills
  (frontmatter, ghi đè nguồn, chặn thoát thư mục), tools (allowlist, chuẩn hoá tên lệnh, tham số) và
  agent (SSE, hàng đợi lượt chạy, xác nhận, prompt, ngân sách ngữ cảnh, mốc tóm tắt)
- Khác bản .NET có chủ ý: `conversationId` gửi kèm khoảng trắng được cắt bỏ (bản .NET coi là hội thoại mới);
  đóng kết nối keep-alive tới bridge khi Core dừng (bản .NET làm qua `HttpClient.Dispose()`) để bridge
  không nhận RST

### Added — Agent Core bản Go, giai đoạn G1 (`core-go/`, song song với bản .NET)
- Viết lại Agent Core bằng Go để phát hành **một binary ~11 MB không cần runtime** (Windows + Linux, build
  chéo không cần cgo nhờ `modernc.org/sqlite`). Bản .NET vẫn là bản phát hành cho tới khi bản Go qua toàn bộ
  e2e; kế hoạch G1–G6 trong `core-go/README.md`
- G1: cấu hình (`AXIOM_*` → HKCU + DPAPI / `config.json` + libsecret), `core.log` (cùng định dạng, xoay 10 MB),
  `core.json`, một-phiên-bản (cùng mutex với bản .NET; `flock` trên Linux), chọn port 47840–47849, guard
  (Origin 403 / token 401 / Content-Type 415), `/health`, `/v1/admin/shutdown`, migrate `core.db` schema 2
  giống hệt, model client (OpenAI-compatible + Anthropic, thử lại lỗi tạm thời, nhắc khi trả lời rỗng, bỏ
  `<think>`), vòng lặp agent, và API wizard `/v1/setup`, `/v1/llm/test`, `/v1/llm/models`
- `scripts/generate_setup_catalog.py` sinh thêm `core-go/internal/setup/catalog_gen.go` (vẫn một nguồn
  `catalog/setup.json`)
- `tests/core/test_core_e2e.py --only <phần,...>` chạy riêng từng phần (dùng để đối chiếu bản Go theo giai
  đoạn); `test_setup` dùng LLM giả riêng nên chạy độc lập được. Bản Go qua `/health` + 19/20 kiểm tra
  `setup` trên cả Windows và Linux (còn thiếu đếm skill, thuộc G3); `go test ./...` cho config, model/codec,
  dịch lỗi, guard + `/v1/setup`

### Added — Nốt phần L3 cho Linux: skill trong gói, libsecret, systemd, CI
- **Skill dựng sẵn trong gói**: `scripts/linux/package.sh` chép `skills/` (8 skill + `_design/tokens.json`)
  vào `core/skills` — Core nạp sẵn nguồn "builtin" cạnh binary, máy mới cài không cần cấu hình `SkillDirs`
- **libsecret cho khoá API trên Linux** (`LibreOffice_arch.md` mục 9): pane **Cài đặt** lưu khoá vào keyring
  qua `secret-tool` khi máy có, `config.json` chỉ giữ `libsecret:LlmApiKey`; Core (`Secrets.Unprotect`) đọc
  lại từ keyring. Máy không có `secret-tool`, keyring khoá, hoặc khoá `dpapi:` của máy Windows khác → coi
  như chưa cấu hình (không làm Core dừng), và vẫn giữ đường lưu thường trong file `0600`
- **systemd --user (tuỳ chọn)**: `install.sh --systemd` sinh unit từ `axiom-office-core.service.in`, trỏ
  thẳng tới Core vừa cài rồi `enable --now`; `--uninstall` gỡ unit. Phiên không có systemd --user thì bỏ qua
  và báo rõ. Mặc định Core vẫn chỉ chạy khi cần (pane tự khởi động)
- **CI GitHub Actions** (`.github/workflows/ci.yml`): job `linux` (Ubuntu 24.04) chạy unit test extension,
  xUnit của Core, build MCP, cài LibreOffice + `python3-uno` rồi chạy toàn bộ test lệnh bridge trên
  LibreOffice thật, test MCP kèm tool live, cuối cùng đóng gói tarball làm artifact; job `windows` chạy unit
  test, Core, build net48 (add-in + Host) và test MCP (không có Office/WPS trong runner nên không chạy làn live)
- Script shell trong repo (`scripts/libreoffice.sh`, `scripts/linux/*.sh`) được đánh dấu thực thi trong git
- Test: `tests/lo` 56 test (thêm `SecretStoreTests` cho `protect_secret`); `tests/core` 201 test (thêm
  `Api_key_libsecret_doc_tu_keyring` dùng `secret-tool` giả trong PATH, không đụng keyring thật)

### Added — MCP server đa nền tảng `axiom-office-mcp` (LibreOffice_arch.md mục 11)
- **`src/AxiomOffice.Mcp`** (`net10.0`, `axiom-office-mcp`): MCP stdio 50 tool — 20 tool file (docx/xlsx/
  pptx/csv: đọc, tạo, sửa, format, export) + tool live gọi bridge qua HTTP + `office_sessions`. **Dùng
  chung mã nguồn** với `AxiomOffice.Host` (compile lại `src/AxiomOffice.Host/Mcp/*.cs` với ký hiệu
  `PORTABLE`), nên hai bản không thể lệch nhau về hành vi tool
- Lớp `Compat/` cho bản .NET 10: `JavaScriptSerializer` trên `System.Text.Json` (cùng API mà `McpServer.cs`
  dùng, `DeserializeObject` trả `Dictionary<string, object>`/`object[]` như bản cũ), `Config`/`Logger`/
  `SessionRegistry` đọc HKCU + `%LOCALAPPDATA%` trên Windows và `~/.config/axiom-office/config.json` +
  `$XDG_RUNTIME_DIR/axiom-office` trên Linux, danh sách lệnh cho mô tả tool `*_command` nhúng sẵn
  (`live-commands.json`, sinh từ registry extension bằng `scripts/generate_mcp_commands.py`)
- `.xls` trên Linux: nhờ `soffice --headless --convert-to xlsx` với **profile riêng** (không đụng phiên
  LibreOffice đang mở) rồi đọc như file xlsx; không có LibreOffice thì báo lỗi rõ. Bản Windows vẫn dùng COM.
  `FindSoffice` dò `AXIOM_SOFFICE` → PATH → các vị trí cài quen thuộc (Linux + Windows)
- Gói Linux: `scripts/linux/package.sh` publish thêm `mcp/` (self-contained, máy đích không cần .NET);
  `install.sh` cài vào `~/.local/share/axiom-office/mcp`, gỡ bằng `--uninstall`, và in sẵn đoạn cấu hình
  `mcpServers` cho Claude Code/Desktop
- Test: `tests/mcp-host/test_mcp_portable.py` (không cần thư viện ngoài, chạy cả Windows lẫn Linux: giao
  thức, 50 tool, các tool file trên file thật, `.xls` qua LibreOffice, tool live khi có app mở, `--live`);
  `tests/mcp-host/test_mcp_host.py` chạy được cho bản .NET 10 qua `AXIOM_MCP_CMD` + `AXIOM_MCP_NO_CATALOG`;
  `tests/lo/test_extension.py` thêm test chống lệch giữa `live-commands.json` và registry
- Đã kiểm chứng: parity đầy đủ với bản Python (python-docx/openpyxl/python-pptx đọc file do bản .NET 10 ghi
  và ngược lại) **103/103 trên cả Windows và Linux**; `test_mcp_portable.py` 61/61 (không có app) và
  70/70 (LibreOffice Calc đang mở, có ghi/đọc ô thật qua bridge); bản net48 (`AxiomOffice.Host.exe`) vẫn
  biên dịch và chạy nguyên như trước

### Added — LibreOffice trên Linux (LibreOffice_arch.md giai đoạn L2–L3)
- **Agent Core chạy được trên Linux**: `TargetFramework` `net10.0` (bỏ `-windows`); cấu hình đọc từ
  `~/.config/axiom-office/config.json` (`JsonConfigSource`, cùng tên khoá với HKCU — giá trị bool ghi
  `1/0`, `SkillDirs` là mảng), khoá API để plaintext trong file `0600` (không có DPAPI), dữ liệu theo XDG
  (`~/.local/share/axiom-office`), session ở `$XDG_RUNTIME_DIR/axiom-office/sessions`. `DocumentKey` chỉ
  hạ chữ thường/đổi `\` trên Windows (Linux phân biệt hoa thường), memory tài liệu nhận đường dẫn `/`
- **Extension**: dùng chung một mã nguồn cho hai hệ điều hành; pane tự khởi động Core từ vị trí cài
  (`<data_dir>/core/AxiomOffice.Core`) khi chưa có `CoreExe`, và trên Linux tách hẳn session khỏi `soffice`
  (`start_new_session`) để Core không chết theo LibreOffice; `ai.ask` dùng chung đường khởi động với pane
  (trước đây có bản sao riêng, không biết vị trí cài)
- **Đóng gói & cài đặt cho Linux** (`scripts/linux/`): `package.sh` tạo
  `dist/axiom-office-linux-x64-<ver>.tar.gz` (Core self-contained + `.oxt` + `install.sh`, máy đích không
  cần .NET); `install.sh` cài không cần root (Core vào `~/.local/share/axiom-office/core`, `unopkg add
  --force`, ghi `config.json` `0600`, giữ cấu hình cũ khi nâng cấp, `--api-key -` đọc key từ stdin),
  `--uninstall [--purge]`. Bổ sung `scripts/libreoffice.sh` (Linux) và `scripts/package_oxt.py` (dùng chung
  hai hệ điều hành) bên cạnh bản PowerShell
- **Sửa lỗi chỉ thấy trên Linux**: footer pane bị thanh trạng thái che (sidebar VCL cấp chiều cao lớn hơn
  vùng vẽ thật — trừ 12px khi ở trong sidebar); mã lệnh bị cắt ở mép phải do font mono rộng hơn Consolas;
  dòng lỗi rút gọn tên kiểu lỗi (`ArgumentException: …`) để xuống dòng được trong cột nhãn hẹp
- **Dọn session của tiến trình đã chết**: `sweep()` xoá file `{pid}-{kind}.json` ngay khi khởi động nếu
  pid không còn sống (`kill 0` trên Linux, `OpenProcess` trên Windows), không phải chờ hết 10 phút theo
  heartbeat — trên Linux `soffice` hay bị tắt bằng SIGTERM/đăng xuất nên file cũ tồn đọng thành session ma
  trong `office_sessions`/Core
- Test: bộ live test chạy được trên Linux (`soffice` theo PATH, tắt app bằng SIGTERM, token đọc từ
  `config.json`, `winreg` import mềm); `tests/lo` 51 unit test (thêm `theme.error_summary`, khởi động Core
  theo nền tảng, dọn session cũ)
- Đã kiểm chứng trên Ubuntu 24.04 + LibreOffice 24.2 (KDE Plasma X11): 159/159 test lệnh headless và
  159/159 có cửa sổ, Agent Core 200/200 (Linux và Windows), lượt `ai.ask` thật trên Writer/Calc/Impress,
  pane chạy ở cả deck sidebar lẫn pane neo bên phải (ảnh chụp trong phiên X ảo và phiên KDE thật)

### Changed — pane Ask AI của LibreOffice làm lại theo thiết kế pane Office
- Giao diện giống pane Word/Excel/PowerPoint: `axiom/theme.py` (bản LibreOffice của `PaneTheme`: cùng mã
  màu, font Segoe UI, cỡ chữ, khoảng cách, nhãn tiếng Việt cho từng lệnh, gợi ý theo app), `axiom/widgets.py`
  (hộp bo góc = nền + 4 ảnh góc PNG vẽ sẵn bằng zlib + viền 1px; nhãn/nút/chip bấm được có hover),
  `axiom/chatview.py` (danh sách cuộn được: bong bóng người dùng/AI, dòng thao tác có icon ✓/✗ và mã lệnh,
  thẻ xác nhận / ghi nhớ / lỗi, "• • •" đang chờ, màn hình đầu có chip gợi ý). `chat.py` sinh thêm danh sách
  mục có cấu trúc (`items` + `rev`) để chỉ vẽ lại mục mới/đổi
- Pane mở trong **sidebar thật của LibreOffice** khi sidebar đang hiện (Calc/Impress): sửa factory đọc tham số
  `ParentWindow`/`Frame` theo tên, panel implement `XSidebarPanel`, ẩn thanh tiêu đề panel thừa; trạng thái
  sidebar đọc qua status của `.uno:Sidebar`
- Cửa sổ không có sidebar (Writer): pane **neo bên phải và co vùng tài liệu lại** như task pane Office (không
  còn đè lên thanh công cụ/tài liệu); ✕ đóng và trả lại bề rộng; mở lại giữ hội thoại
- Ô soạn: Enter gửi (sửa lỗi Edit chèn "\n" vào giữa tin nhắn khi con trỏ không ở cuối), Shift+Enter xuống
  dòng, PageUp/PageDown cuộn hội thoại, viền đổi màu khi có focus, placeholder
- Cài đặt/Ghi nhớ: nền trắng, viền mảnh, font như pane, đặt giữa vùng tài liệu (không đè pane)
- Bấm trên pane kích hoạt khi **nhấn** chuột: panel trong sidebar không nhận được `mouseReleased`
- `writer.heading`: tiêu đề luôn thành đoạn riêng khi con trỏ đang ở đoạn có chữ (trước đây nối vào đoạn cũ
  và biến cả đoạn thành Heading)
- Test: `tests/lo/test_chat.py` 23 test (thêm mục có cấu trúc, vòng đời thẻ xác nhận, nhãn, PNG góc/icon);
  live 159/159 (thêm ca heading), cả headless lẫn có cửa sổ; đã chạy thật bằng chuột/bàn phím: chip → Gửi,
  Enter giữa câu, Hoàn tác lượt này, ✕ đóng, mở lại qua menu, phóng to cửa sổ, lượt thật trong sidebar Calc

### Added — pane Ask AI trong LibreOffice (LibreOffice_arch.md mục 10, giai đoạn L2)
- **Pane nói chuyện với Agent Core bằng Python** (`axiom/core.py`, `axiom/chat.py`): POST `/v1/runs` (kèm
  `office.port` của bridge), đọc SSE `/v1/runs/{id}/events`, `cancel`/`confirm`, `GET /v1/memory`; Core
  chưa chạy thì tự khởi động bằng `CoreExe` trong cấu hình. Logic hội thoại (`chat.py`) thuần Python:
  transcript, số thao tác sửa để "Hoàn tác lượt này", thẻ xác nhận, ghi nhớ vừa ghi, dòng trạng thái
- **Giao diện pane** (`axiom/awt.py` + `axiom/panel.py`): transcript (dòng `✓ writer.…`, `✓ Dùng kỹ năng`,
  `✓ Đã ghi nhớ`), ô nhập (Enter = Gửi), **Gửi/Dừng**, thẻ **Đồng ý/Từ chối** khi policy cần xác nhận,
  **Trò chuyện mới**, **Hoàn tác lượt này** (Writer/Calc/Impress — UNO hoàn tác được, khác Excel qua COM),
  **Cài đặt**, **Ghi nhớ**, **Đóng**; mọi cập nhật UI đi qua `UnoGate` (awt chỉ chạy trên main thread, và
  không gọi lại gate khi đã ở main thread để tránh tự treo)
- **Cài đặt** (`axiom/dialogs.py`): provider/endpoint/model/API key/CoreExe + bật ghi nhớ, tự trích xuất,
  QA thị giác; ghi vào HKCU như add-in, API key mã hoá **DPAPI** (`dpapi:<base64>`, đúng định dạng
  `Secrets.Unprotect` của Core); có "Kiểm tra Core" và "Tắt Core". **Ghi nhớ**: liệt kê/tìm/xoá qua
  `/v1/memory`
- **Hai đường mở pane**: deck sidebar "Axiom Office" (`Sidebar.xcu` + `Factory.xcu` + `axiom_panel.py`:
  `XUIElementFactory`/`XUIElement`/`XToolPanel`) và menu **Axiom Office → Ask AI / Settings…**
  (`Addons.xcu` + `ProtocolHandler.xcu` + `axiom_dispatch.py`); lệnh `ui.askpane` mở deck nếu bản
  LibreOffice có sidebar, không thì mở pane dạng cửa sổ con neo bên phải cửa sổ tài liệu
- Test: `tests/lo/test_chat.py` (12 unit test: đếm thao tác, thẻ xác nhận, ghi nhớ, dòng trạng thái, đọc
  SSE bằng server giả); đã chạy thật trên LibreOffice: gửi yêu cầu → Core sửa tài liệu → hiện phản hồi
  (Writer 7–10s), **Hoàn tác lượt này** trả tài liệu về nguyên trạng, **Dừng** hủy giữa lượt, thẻ xác nhận
  `wpp.deleteSlide` trên Impress (đồng ý → xoá slide thật), dialog Cài đặt/Ghi nhớ hiện đúng cấu hình
- `LibreOffice_arch.md` mục 14.2: các khác biệt API phải xử lý khi làm pane (model điều khiển awt không
  nhận toạ độ → đặt lên view; dialog rời không hiện được → cửa sổ con; sidebar của LO 26.8 không có phần
  tử layout; `XInitialization` nằm ở `com.sun.star.lang`; tên node ProtocolHandler = implementation name;
  MRO khi vừa `XDispatchProvider` vừa `XDispatchProviderInterceptor`)

### Added — làn LibreOffice: extension Python UNO (LibreOffice_arch.md, giai đoạn L1)
- `src\AxiomOffice.LibreOffice`: extension `.oxt` (`org.axiomoffice.bridge`) chạy trong `soffice` — job
  `OnStartApp` bật bridge HTTP trên 47851/47852/47853 (Writer/Calc/Impress, kind `wps`/`et`/`wpp`),
  **giữ nguyên giao thức và tên lệnh** với add-in nên Agent Core, MCP (`office_sessions` + `*_command`
  kèm `port`), skill, memory và policy dùng lại không đổi. Port đổi qua `PortLibreOffice`, token dùng
  chung `Token`; bận cổng thì thử +10 (5 lần)
- `UnoGate`: mọi lệnh UNO chạy trên main thread qua `com.sun.star.awt.AsyncCallback`, khoá tuần tự như
  `ComGate`; `/health` thêm `stuck` khi main thread không trả lời (hộp thoại đang mở) và lỗi `Busy` nêu
  rõ phải đóng hộp thoại. Mỗi lệnh AI = **một bước Undo** (`XUndoManager`) — Calc/Impress cũng hoàn tác
  được, khác Excel qua COM
- Đủ bộ lệnh của bản C# (test so trực tiếp với `AxiomOffice.Host.exe commands --json`: cùng tên, kind,
  cờ `ForAgent`, tham số; chỉ thêm `et.closeAll`/`wpp.closeAll` cho test): `writer.*` (bảng, style theo
  autoformat LibreOffice, replaceAll biết giới hạn không tìm xuyên đoạn), `et.*` (ô `null` để trống
  thật, công thức tính đúng, `numFmt` theo locale en-US), `wpp.*` (layout Office 1/2/11/12 →
  `AUTOLAYOUT_*`, đo tràn chữ bằng `TextAutoGrowHeight`), `writer.checkTables`/`et.checkRange`/
  `wpp.checkLayout`, `app.info`, `app.screenshot` (xuất trang/slide ra PNG), `ai.ask` (chạy qua Agent Core)
- Session: một tiến trình `soffice` ghi ba file `{pid}-{kind}.json` (heartbeat 25s, xoá khi thoát, dọn
  file cũ > 10 phút); `writer.open`/`et.open`/`wpp.open` trên file **đang mở** thì kích hoạt cửa sổ đó
  (`alreadyOpen`) thay vì load lại — tránh hộp thoại "đã mở" chặn main thread
- `scripts\libreoffice.ps1`: `-Package` (đóng gói `.oxt` vào `dist`), `-Install`/`-Uninstall`
  (`unopkg add --force`, từ chối chạy khi LibreOffice đang mở), `-Status`, `-Log`
- Test: `tests\lo\test_extension.py` (18 unit test, không cần LibreOffice: uno giả — giải mã tham số
  `{"item":…}`/chuỗi JSON/số dạng chuỗi + registry khớp bản C#); `tests\live\test_live_libreoffice.py`
  (158 kiểm tra trên Writer/Calc/Impress thật, headless lẫn có cửa sổ, tự mở/đóng LibreOffice)
- Đã kiểm chứng `ai.ask` end-to-end: Core chạy agent đầy đủ (nạp skill, đọc tài liệu, chèn heading +
  bảng, tự soát bằng `writer.checkTables`) trên tài liệu LibreOffice thật

### Docs — `LibreOffice_arch.md`: thiết kế tích hợp LibreOffice trên Linux
- Bridge là extension Python UNO (`axiom-office.oxt`) chạy trong `soffice`, **giữ nguyên giao thức bridge và tên
  lệnh** (`writer.*`/`et.*`/`wpp.*`) để Agent Core, skill, memory, test dùng lại; ba session/port như WPS
  (47851–47853); `UnoGate` đưa lệnh về main thread qua `AsyncCallback`; mỗi thao tác AI = 1 bước Undo
  (`XUndoManager`, cả Calc/Impress); bảng ánh xạ từng lệnh sang UNO
- Agent Core đa nền tảng (`net10.0`, cấu hình `config.json` + XDG, libsecret, file lock), sidebar Ask AI,
  làn file `axiom-office-mcp`, cài không cần root (`unopkg` + tarball), kiểm thử headless + CI Ubuntu,
  giai đoạn L0–L4, rủi ro và câu hỏi mở

### Added — Agent Core giai đoạn 4: Mở rộng và an toàn (New_arch.md mục 7.7, 8.6, 8.7, 8.4.6)
- **Policy xác nhận** (`PolicyEngine` + `ConfirmationBroker`): hỏi trước khi model lưu/xuất file mà yêu cầu
  không nhắc tới lưu/xuất, `saveAs`/`exportPdf` ghi đè file đã có, `wpp.deleteSlide`, `writer.replaceAll` trên
  tài liệu > 20.000 ký tự, tool MCP ngoài; SSE `confirm.required` / `confirm.resolved`, `POST
  /v1/runs/{id}/confirm`, chờ tối đa `ConfirmTimeoutSeconds` (120s), hết giờ/hủy = từ chối, model nhận
  `user declined`. Pane: thẻ **Đồng ý / Từ chối**
- `GET /v1/audit?runId=&limit=`: nhật ký tool call (params ≤ 2KB; tool không phải `office_action` ghi đủ đối số)
- **MCP client**: stdio + Streamable HTTP, `mcp.json` (`command/args/env` hoặc `url/headers`, `trusted`,
  `disabled`), khởi động lười, lỗi server → ẩn tool; server built-in `office` = `Host.exe mcp` **chỉ mở tool
  làn file** (tool live đi vòng allowlist/policy bị lọc), ghi vào file đã có thì hỏi; `GET /v1/mcp`
- **`ai.ask` chạy qua Agent Core** (giữ hình dạng response, thêm `viaCore`; chế độ không tương tác từ chối
  xác nhận ngay); bridge xử lý `ai.ask` trên thread riêng để Core gọi ngược `/cmd` không bị kẹt
- **QA thị giác tuỳ chọn** (`VisualQaEnabled`, tắt mặc định): lệnh bridge `app.screenshot` (PrintWindow, thu
  nhỏ), tool `look_at_document` gửi ảnh cho model (OpenAI `image_url` / Anthropic khối `image`); Cài đặt có ô bật
- Pane (New_arch.md mục 9.2): nút **Hoàn tác lượt này** trong Word (đếm thao tác `writer.*` có sửa tài liệu
  của lượt, gọi `writer.undo {count: N}`; hiện cả sau khi bấm Dừng); link **Trò chuyện mới** chuyển lên header
  cạnh Cài đặt và luôn hiện khi dùng Agent Core (trước ở footer, khó thấy trong Word)
- `ARCHITECTURE.MD`: HLD cập nhật cho Agent Core (container, khối chức năng, kịch bản K1/K6/K7, triển khai,
  NFR, AD-11…AD-17, rủi ro) + LLD mục 22 Agent Core
- Test: 194 unit test Core; e2e 76 kiểm tra (xác nhận đồng ý/từ chối/hết giờ, MCP server mẫu Python stdlib,
  làn file office, QA thị giác, audit); `ai.ask` qua Core trên Excel thật; `app.screenshot` trên Excel thật

### Added — Agent Core giai đoạn 3: Memory dài hạn (New_arch.md mục 8.5, học từ mem0 2.2.1)
- Schema 2: `memories`, `memories_fts` (FTS5, text chuẩn hoá bỏ dấu kể cả `đ`), `memory_links`,
  `memory_history`, `memory_embeddings`; `runs.memory_status`
- `SqliteMemoryStore`: ADD theo lô trong một transaction (chống trùng hash SHA-256 trong lô + với memory cũ,
  gần-trùng cosine ≥ 0,96 nếu có embedding, link chỉ tới memory tồn tại, history); sửa/ghim/hạn dùng/xoá
  mềm/khôi phục/xoá cứng **chỉ do người dùng**; dọn xoá mềm sau 30 ngày
- `MemoryRetriever`: chấm điểm cộng dồn như `score_and_rank` của mem0 (keyword sigmoid theo độ dài truy
  vấn, entity boost giảm dần, semantic nếu có embedding; ngưỡng 0,1 chặn tín hiệu chính trước khi cộng);
  ngữ cảnh = ghim + ≤ 20 memory tài liệu + ≤ 8 memory user, ≤ ~1.500 token, bản chuyển đổi mới đứng trước
- `MemoryExtractor` **chỉ-ADD** (prompt tiếng Việt nhúng trong exe) chạy ở **hàng đợi nền**: id tạm chống
  bịa id, lọc confidence < 0,6 / > 300 ký tự / thông tin nhạy cảm (regex CCCD, số thẻ, mật khẩu, API key),
  bỏ qua lệnh thao tác thuần; JSON lỗi thì thử lại 1 lần. Tóm tắt hội thoại chuyển sang cùng hàng đợi
- Tool `remember` / `recall`; SSE `memory.written`; `/v1/memory` (list/tìm/thêm/sửa/xoá/khôi phục/lịch
  sử/xoá toàn bộ); embedding tuỳ chọn (`EmbeddingModel`, lỗi thì tự chạy chỉ keyword)
- Pane: dòng **Đã ghi nhớ: …** + **Xoá** (cả memory trích xuất nền của lượt vừa xong); Cài đặt thêm "Dùng
  Agent Core", "Ghi nhớ dài hạn", "Tự ghi nhớ sau mỗi lượt", form **Quản lý ghi nhớ…**
- Test: 33 unit test memory; e2e 18 kiểm tra (3 phiên + khởi động lại Core, chuyển đổi chức vụ có liên kết,
  trùng hash, id bịa, lệnh thao tác thuần, xoá/khôi phục/lịch sử, `MemoryEnabled=0`); 14 kiểm tra add-in
  `CoreClient` ↔ Core thật (thư mục dữ liệu tạm)

### Fixed
- Chuẩn hoá bỏ dấu tiếng Việt trong Core dùng bảng tường minh: Core chạy `InvariantGlobalization` nên
  `string.Normalize(FormD)` không tách dấu ("in đậm" không thành "in dam"); project test cũng chạy invariant

### Added — Agent Core giai đoạn 2: Skills (New_arch.md mục 8.4)
- **Skill theo chuẩn Agent Skills**: `Skills/SkillLoader` (frontmatter `name`/`description` + `apps` tuỳ
  chọn; validate tên ≤ 64 ký tự `a-z0-9-`, từ cấm `anthropic`/`claude`, mô tả ≤ 1024 không thẻ XML; field
  lạ bỏ qua), `SkillIndex` (3 nguồn: `skills\` cạnh exe → `SkillDirs` → `%LOCALAPPDATA%\AxiomOffice\skills`,
  nguồn sau thắng; thư mục `_*` là gói tài nguyên; FileSystemWatcher debounce 2s). Skill lỗi hiện ở
  `GET /v1/skills`, không làm hỏng Core
- Tool `load_skill` (tầng 2, phát SSE `skill.loaded`) và `read_skill_file` (tầng 3: text ≤ 64KB, file nhị
  phân → đường dẫn tuyệt đối; chặn `..`, đường dẫn tuyệt đối, symlink/junction ra ngoài; chỉ `.md .txt
  .json .csv .docx .xlsx .pptx .png .jpg`). Chỉ mục skill hợp app vào system prompt (tầng 1)
- `GET /v1/skills?app=`, `POST /v1/skills/reload`
- **Tầng thiết kế**: `skills/_design/tokens.json` + `thiet-ke-van-phong`, `trinh-bay-chuyen-nghiep`,
  `the-thuc-van-ban`, `bao-cao-du-lieu`; **tầng triển khai**: `bao-cao-thang` (PowerPoint), `bang-diem`
  (Excel), `van-ban-hanh-chinh` (Word, kèm `references/the-thuc.md` theo NĐ 30/2020)
- **QA cấu trúc** (lệnh bridge chỉ đọc, agent dùng được): `wpp.checkLayout` (chữ tràn khung, ra ngoài
  slide, shape chồng, chữ < 12pt, slide quá nhiều chữ), `et.checkRange` (tiêu đề trống, kiểu lẫn lộn, số
  dạng chữ, số lẻ chưa number format, ô lỗi, dữ liệu lạc ngoài bảng), `writer.checkTables` (ô trống, ô
  tiêu đề lẫn đoạn văn). `writer.formatTable` thêm `borders: false` (bảng dàn trang)
- Pane hiện dòng **Dùng kỹ năng: …**; thao tác `load_skill`/`read_skill_file` không hiện như thao tác tài liệu
- Test: 19 unit test skill; e2e 13 kiểm tra luồng `load_skill` với LLM giả; `test_core_e2e.py --real-llm`
  (LLM thật): muse-spark-1.3 chọn đúng `bao-cao-thang` / `bang-diem` / `van-ban-hanh-chinh` cho 3 yêu cầu
  mẫu và **không nạp skill** cho "in đậm dòng đầu"; test live dựng sẵn ca tràn chữ/chồng shape/dữ liệu lạc ô

### Changed
- Hết giờ mỗi request tới model: 60s → **120s**, chỉnh được (`LlmRequestTimeoutSeconds` /
  `AXIOM_LLM_REQUEST_TIMEOUT`): model free (oc/muse-spark) có lượt sinh công văn dài hơn 60s
- Model trả lời rỗng giữa chừng: Core nhắc **một lần** để làm tiếp/tóm tắt thay vì hỏng cả lượt

### Fixed — Excel hỏi lưu một sổ lạ / chạy ngầm sau khi đóng
- Agent không còn gọi được `et.newWorkbook` (giống `writer.newDocument`/`wpp.newPresentation`): log
  30/09 01:15 model tạo thêm Book2 dù Book1 đang mở, người dùng đóng Excel thì bị hỏi lưu một sổ họ
  không biết. Excel trống (chưa có sổ) thì `et.listSheets`/`et.writeRange`/`et.formatRange` tự tạo sổ
- Add-in nhả hẳn các tham chiếu COM (`Application`, CTP factory, task pane) và ép GC khi
  `OnDisconnection`: trước đây chỉ gán null nên RCW chờ finalizer
- Tên lệnh agent viết sai nhẹ (`et_writeRange`, khác hoa/thường — gặp ở `oc/mimo-v2.6-flash-free`)
  được quy về tên đúng thay vì bị từ chối và mất một vòng (Core + in-process)

### Added — độ bền khi gọi model
- **Tự thử lại khi nhà cung cấp lỗi tạm thời** (HTTP 429/500/502/503/504, rớt mạng) ở cả Agent Core
  (`ModelClient`) và agent in-process (`LlmClient`): tối đa 3 lần, chờ 1s → 2s → 4s, theo `Retry-After`
  nếu có (tối đa 10s); không thử lại khi hết giờ một request hay người dùng bấm Dừng. Trước đây một lỗi
  503 "high demand" (gemini-3.8-flash) hay 500 ngẫu nhiên (Gemma 4) làm hỏng cả lượt chạy
- Lọc `<thought>…</thought>` / `<think>…</think>` khỏi câu trả lời hiện cho người dùng (Gemma 4 qua
  endpoint của Google, DeepSeek/Qwen trả kèm phần suy nghĩ); tin nhắn gửi lại model vẫn giữ nguyên

### Added
- **Google Gemini** trong Cài đặt: chọn "Google Gemini" điền sẵn endpoint OpenAI-compatible của Google
  (`https://generativelanguage.googleapis.com/v1beta/openai`) và model `gemini-2.5-flash`; lưu dạng
  provider `openai` nên Core, agent in-process và bản cài cũ đều dùng được (không cần SDK
  `Google.GenAI` — SDK chỉ chạy được trong Core .NET 10, còn add-in là .NET Framework 4.8)
- Core đọc lại cấu hình LLM (provider/endpoint/key/model) **mỗi lượt chạy** (`Models/ModelSource.cs`):
  trước đây chỉ đọc lúc khởi động nên đổi model trong Cài đặt không có tác dụng tới khi Core khởi động lại
- `writer.formatTable` (agent dùng được): định dạng bảng **có sẵn** — kiểu, font, cỡ, màu chữ, màu
  hàng tiêu đề, màu sọc, viền, căn lề, co giãn. Trước đây không có lệnh này nên với "tô màu bảng cho
  đẹp" agent phải `undo` (xoá cả bảng và ghi chú) rồi dựng lại, mất 20 vòng/112s
- Mô tả tool `office_action`: sửa tại chỗ, không dùng `undo` để làm lại trừ khi người dùng yêu cầu

### Fixed — lỗi Ask AI thấy khi test Word qua Agent Core
- **Style bảng Word chưa bao giờ được áp**: `table.set_Style(...)` không tồn tại khi gọi late binding
  (`dynamic`) và lỗi bị nuốt, nên `style` của `writer.insertTable` luôn bị bỏ qua mà vẫn trả ok. Nay
  gán `Style` trực tiếp; lỗi (nếu có) trả về trong `styleError`
- Sau `writer.insertTable` con trỏ nằm ở ô (1,1) nên chữ chèn tiếp lọt vào ô tiêu đề (file thật:
  dòng ghi chú nằm trong ô "Thứ"). Nay con trỏ ra ngay sau bảng
- `writer.insertTable` lỗi COM "The requested member of the collection does not exist" khi chèn bảng
  rộng hơn ngay sau/trước một bảng khác: Word gộp hai bảng liền nhau. Nay tự chèn đoạn ngăn cách
- Dòng tiến trình của pane khi chạy qua Core chỉ hiện `office_action {}`: sự kiện `tool.finished`
  kèm `paramsPreview`, pane dựng lại `{"action": ..., "params": ...}` để hiện nhãn tiếng Việt; log
  "AskAiPane: ok ... N tool calls" đếm đúng số thao tác của Core
- **Mọi yêu cầu bị làm HAI lần** (Core và in-process chạy song song): `CoreClient` đọc `runId` ở cấp
  ngoài trong khi Core trả `{"ok":true,"result":{"runId":...}}`, nên luôn coi là "Core không trả
  runId" và chạy thêm in-process. Hệ quả: dữ liệu bị ghi hai lần (vd thừa một hàng "Trung bình"),
  lỗi COM `0x800A01A8` do hai agent sửa cùng lúc, luôn hiện "chế độ cơ bản", không có link "Cuộc trò
  chuyện mới"
- **Pane chạy một yêu cầu hai lần**: Core đã nhận lượt chạy (có `runId`) nhưng đọc SSE lỗi thì
  `CoreClient.Run` trả null và pane chạy lại in-process — tài liệu bị sửa hai lần, dòng trạng thái
  hiện cả "Xong" lẫn "chế độ cơ bản". Nay chỉ quay về in-process khi Core chưa nhận lượt chạy; mọi lý
  do quay về đều ghi `bridge.log` (`CoreClient: fallback to in-process - ...`)
- `writer.insertTable`/`et.writeRange`/... báo "nested array/object at row 1, column 1" khi model bọc
  mỗi dòng thêm một lớp (`{"item":[{"item":{"item":[...]}}]}`): gỡ lớp thừa của từng dòng
- "`rows` must be a whole number, got an object with keys [item]": tham số số nhận `{"item": 6}` và
  `[6]`; `writer.insertTable` lỡ nhận dữ liệu bảng trong `rows` thì dùng như `values`
- `writer.insertTable` lỗi COM "The range cannot be deleted": chèn ở cuối vùng chọn (trong bảng thì
  sau bảng) thay vì thay nội dung đang chọn
- `writer.replaceAll` không bao giờ khớp văn bản nhiều dòng (model gửi `\n`, Word dùng `\r`/`\v`) nên
  agent lặp tới hết ngân sách 200k token: đổi `\n` → `^p`, `\v` → `^l`, `\t` → `^t`, thoát `^`
- Core: `params` gửi dạng chuỗi JSON được parse thành object; tóm tắt hội thoại chỉ khi vượt mốc mới
  mỗi 20 tin nhắn (trước đây tóm tắt lại sau **mọi** lượt khi hội thoại > 20 tin, tốn thêm một lần
  gọi model mỗi lượt)

### Added — Agent Core giai đoạn 1 (phần 2): pane chạy qua Core
- Add-in: `Ai/CoreClient.cs` — tìm Core qua `core.json`, khởi động `AxiomOffice.Core.exe` khi cần,
  `POST /v1/runs` rồi đọc SSE; hủy lượt chạy cả hai phía (abort request + `POST .../cancel`)
- `Ai/AskAiPane.cs`: lượt chạy đi qua Agent Core khi có, **tự chạy agent in-process khi Core không
  dùng được** và ghi chú "chế độ cơ bản" ở dòng trạng thái; hội thoại **nhớ theo tài liệu** (mở lại
  tài liệu vẫn tiếp tục mạch cũ, hỏi Core qua `/v1/conversations?documentKey=`); link **Cuộc trò
  chuyện mới** ở footer; dòng tiến trình lấy từ sự kiện `tool.finished`; thông báo riêng khi dừng vì
  hết ngân sách token; đọc tên tài liệu qua `ComGate` (không gọi COM song song với thread bridge)
- `Bridge/Config.cs`: `CoreEnabled`; `LlmResult` thêm `ViaCore`, `Stopped`, `ConversationId`
- Sự kiện `tool.finished` của Core kèm `resultPreview` để pane hiện dòng giống chế độ in-process
- `build.ps1`: thêm `IncludeNativeLibrariesForSelfExtract` — **sửa lỗi đóng gói**: bản single-file
  trước đó thiếu `e_sqlite3.dll` nên Core báo "SqliteConnection type initializer threw" (49,2MB)
- Test e2e mới (`tests/core/fake_llm.py` + `tests/core/test_core_e2e.py`, 22 kiểm tra không cần
  Office + 6 kiểm tra `--office` trên Excel thật): thứ tự sự kiện SSE, hội thoại, audit, allowlist
  (lệnh bị chặn không xuống bridge), hủy, trần token, lỗi office, và tài liệu thật sự đổi

### Docs — `New_arch.md`: tầng thiết kế cho skills (design intelligence)
- Tách design intelligence khỏi skill triển khai theo app: **token thiết kế là dữ liệu**
  (`skills/_design/tokens.json`), skill thiết kế nạp khi cần (`thiet-ke-van-phong`,
  `trinh-bay-chuyen-nghiep`, `the-thuc-van-ban`, `bao-cao-du-lieu`), skill theo app dùng giá trị cụ
  thể trong lệnh. Không xây router riêng (model chọn theo `description`), không nhét design system
  vào `ppt/SKILL.md`, không nạp skill thiết kế cho sửa nhỏ
- **QA hai mức** cho điểm yếu "agent mù": giai đoạn 2 kiểm tra cấu trúc bằng số (tràn chữ, shape
  chồng, số dòng/cột, number format — không cần thị giác); giai đoạn 4 mới thêm `app.screenshot` +
  model thị giác, kèm xác nhận của người dùng vì tốn token

### Added — Agent Core (giai đoạn 0 của New_arch.md)
- `src/AxiomOffice.Core/` (.NET 10, `AxiomOffice.Core.exe`): process riêng của agent, một bản cho
  mỗi người dùng Windows (mutex `Local\AxiomOffice.Core`, instance thứ hai thoát ngay), chỉ nghe
  `127.0.0.1`. Giai đoạn 0 gồm: `GET /health` (không cần token), `POST /v1/admin/shutdown`, quy tắc
  bảo vệ giống bridge (chặn `Origin` → 403, thiếu token → 401, body không phải JSON → 415), ghi
  `%LOCALAPPDATA%\AxiomOffice\core.json` khi sẵn sàng và xoá khi thoát, log `core.log` (xoay 10MB × 3)
- Cấu hình Core: `HKCU\Software\AxiomOffice` (`CorePort` 47840, `CoreEnabled`, `MemoryEnabled`,
  `MemoryAutoExtract`, `LlmProvider/Endpoint/Model`, API key DPAPI) + override `AXIOM_*` để test
  không đụng cấu hình thật; port bận thì tự thử 47840–47849
- Bridge: `GET /commands` trả bộ lệnh của DLL đang chạy (tên, loại app, cờ agent, tham số, `version`)
  — Agent Core dùng để dựng tool cho agent đúng phiên bản
- `scripts/install-dotnet-sdk.ps1` (cài .NET 10 SDK không cần admin), `scripts/core.ps1` (tắt Core
  êm qua API), `build.ps1` publish Core self-contained single-file với version của DLL (bật
  `EnableCompressionInSingleFile`: 103MB → 47,8MB đo trên máy này), `package.ps1` đóng gói kèm Core
  + thư mục `skills\`, `install.ps1` tạo khoá cấu hình mới, `uninstall.ps1` tắt Core
- `tests/core/AxiomOffice.Core.Tests` (xUnit, 35 test): cấu hình (ưu tiên env > HKCU > mặc định,
  DPAPI, port không hợp lệ), core.json, log (xoay file), chọn port, và test vòng đời trên **tiến
  trình thật** (core.json ↔ tiến trình, /health, 401/403/415/404, instance thứ hai, shutdown)
- `tests/live/test_live_commands.py`: kiểm tra `/commands` khớp registry và version Core ↔ bridge khi
  Core đang chạy

### Docs — `New_arch.md`: yêu cầu triển khai Agent Core
- Bản yêu cầu cho phiên làm việc mới: tách agent ra process riêng `AxiomOffice.Core.exe` (.NET 10,
  một bản mỗi người dùng) với hội thoại liên tục, skills (`SKILL.md`), memory SQLite, policy/xác
  nhận, MCP client; add-in mỏng lại, giữ agent in-process làm dự phòng
- Hợp đồng Core API v1 + SSE, `GET /commands` trên bridge, schema DB, build/đóng gói, chiến lược
  test (fake LLM theo kịch bản), 4 giai đoạn có tiêu chí hoàn thành, quy tắc bắt buộc của dự án
- Memory tự làm bằng C#, học từ mem0 2.2.1 đã đọc repo (không dùng thẳng: chỉ có SDK Python/TS, tự
  host cần Docker, bản cloud gửi dữ liệu ra ngoài): trích xuất **chỉ-ADD** sau run (nền, bộ lọc rẻ,
  lọc nhạy cảm, ghi rõ sự chuyển đổi kèm liên kết memory cũ), chống trùng bằng hash, tìm kiếm FTS5
  bỏ dấu + vector tuỳ chọn với scoring cộng dồn và sigmoid theo độ dài truy vấn, hạn dùng memory,
  `IMemoryStore` để chỗ cho backend mem0 sau này
- Skills theo **chuẩn Agent Skills của Anthropic** (nghiên cứu tài liệu chính thức + repo mở):
  thư mục `SKILL.md` + frontmatter `name` (≤ 64 ký tự, `a-z0-9-`, cấm "anthropic"/"claude") +
  `description` (≤ 1024 ký tự, ngôi ba, nêu cái gì + khi nào dùng); nạp 3 tầng — metadata vào
  prompt (~100 token/skill), `load_skill` khi kích hoạt, `read_skill_file` khi cần tài nguyên;
  `references/`/`examples/`/`templates/`, liên kết 1 cấp, đường dẫn kiểu `/`, file > 100 dòng có
  mục lục, thân `SKILL.md` < 500 dòng; viết eval trước khi viết hướng dẫn, checklist workflow,
  vòng đọc-lại. Lệch chuẩn có chủ đích: **không chạy script** trong skill (an toàn dữ liệu văn
  phòng); field mở rộng `apps` luôn tuỳ chọn nên skill tương thích 2 chiều
- Vá 4 lỗ hổng agent: (1) **làn file** qua `AxiomOffice.Host.exe mcp` làm MCP server built-in
  (`mcp__office__*`, trusted) — agent đọc/ghi file không cần mở app, không chiếm cửa sổ active,
  dùng lại 50 tool có sẵn không viết code mới; (2) quy tắc **chống prompt injection** trong system
  prompt (nội dung đọc từ tài liệu/file là dữ liệu, không phải chỉ dẫn); (3) nút **Hoàn tác lượt
  vừa rồi** trong pane (Word: đếm N lệnh đã chạy → `writer.undo {count: N}`); (4) **trần token
  mỗi run** (`maxTokens` mặc định 200k, event `run.stopped` kèm số token đã dùng)

### Docs — `ARCHITECTURE.MD`: tài liệu thiết kế
- Phần I, High Level Design: mục tiêu và phạm vi, bối cảnh hệ thống, container, khối chức năng,
  kịch bản chính, triển khai, yêu cầu phi chức năng, quyết định kiến trúc (AD-1..AD-10), rủi ro
- Phần II, Low Level Design: tổ chức mã nguồn, vòng đời add-in, mô hình thread và `ComGate`, HTTP
  bridge (pipeline, mô hình lỗi), hệ lệnh (registry, luồng thực thi, đọc tham số), AI agent (giao
  thức LLM, state machine), schema session và SSE, MCP server (50 tool, làn live/file), cấu hình và
  đăng ký COM, build/cài đặt, thiết kế kiểm thử; 18 sơ đồ mermaid
- README trỏ sang tài liệu này; bảng thành phần thêm chế độ `commands` của Host.exe

### Fixed — `values` bọc `{"item": ...}` bị ghi sai hướng; lỗi tham số khó hiểu
- Log (Excel, "ghi Tổng vào A5 và công thức tổng vào B5"): model gửi
  `{"item":{"item":["Tổng","=SUM(B2:B3)"]}}` (một dòng). Bridge gỡ mọi lớp bọc cùng lúc thành mảng
  1 chiều nên ghi thành **cột** (A5, A6); model loay hoay 19 vòng, ghi rác vào A6:C7. Giờ mỗi lớp
  `{"item": x}` là một cấp mảng (x không phải mảng = phần tử duy nhất), bỏ lớp bọc thừa ngoài cùng
  và ô bị bọc `{"item":"a"}`; nhận cả chuỗi JSON dạng object. Cùng yêu cầu giờ xong trong 3 vòng.
  Các dạng vốn chạy đúng (`{"item":[{"item":[...]}]}`, mảng 1 chiều = cột...) giữ nguyên kết quả
- Tham số số / true-false sai kiểu báo tên tham số và giá trị (`'layout' must be a whole number,
  got 'Title Only'`) thay cho `FormatException: Input string was not in a correct format.`; số
  dạng chuỗi có phần thập phân (`"12.0"`) được nhận
- `wpp.addSlide`: mô tả tool ghi rõ `layout` là số (1 tiêu đề, 2 tiêu đề + nội dung, 11 chỉ tiêu
  đề, 12 trống) — trước đó model gửi `"Title Only"`

### Changed — Ask AI: giới hạn lệnh của agent, không tự lưu file
- Tool `office_action` từ chối lệnh không có trong mô tả tool (không gắn `ForAgent()`), vd
  `writer.closeAll` (đóng mọi tài liệu, không lưu) hay `ai.ask` lồng nhau; model nhận lỗi bảo dùng
  lệnh trong danh sách, log `office_action refused: <action>`. HTTP API / MCP vẫn gọi được mọi lệnh
- Agent có thêm `writer.typeText` và `writer.appendText` (trước đó phải "nối dòng" bằng
  `writer.replaceAll` chèn `\n`)
- System prompt: không `save` / `saveAs` / `exportPdf` nếu người dùng không yêu cầu (trước đó model
  tự lưu, vd tạo `Documents\Presentation1.pptx`)
- Bảng lệnh README thêm cột **Ask AI**; `test_live_commands.py --ai` chạy `ai.ask` trên cả 3 app,
  kiểm tra không tự lưu/xuất và tài liệu còn ở trạng thái chưa lưu; kiểm tra offline (reflection
  vào DLL) rằng `office_action` từ chối lệnh ngoài danh sách

### Changed — Dọn nợ kỹ thuật: registry lệnh bridge
- Mỗi lệnh `POST /cmd` khai báo một lần (`Command(...)`: tên, loại app, handler, mô tả, tham số)
  thay cho 3 nơi phải sửa tay: `switch` của dispatcher, chuỗi lệnh của tool `office_action`, mô tả
  tool MCP. Tool `office_action` (lệnh gắn `ForAgent()`), danh sách lệnh trong mô tả MCP
  `word_command` / `ppt_command` / `wps_live_command` và bảng lệnh README đều sinh từ registry.
  Tên + tham số của 50 tool MCP không đổi; model thấy cùng bộ lệnh như trước (`writer.heading` hiện
  thêm tham số `break` vốn đã có)
- `CommandDispatcher.cs` (1721 dòng) tách theo app: `CommandDispatcher.Writer/Spreadsheet/Presentation.cs`
  + `Params.cs` (đọc tham số, chuyển giá trị COM); kiểm tra loại app (`RequireKind`) làm một lần ở
  dispatcher thay vì đầu mỗi handler. Thân handler giữ nguyên
- `AxiomOffice.Host.exe commands [--json|--markdown]`: in danh sách lệnh bridge
- `AxiomOffice.csproj` gom `**\*.cs` như `build.ps1` thay vì liệt kê từng file
- Bỏ code chết: `Probe.cs` (class COM chẩn đoán không còn đăng ký), `Ai/DocumentContext.cs` (không
  còn được gọi)

### Added — Test tích hợp lệnh bridge
- `tests/live/test_live_commands.py`: gọi mọi lệnh (và các lỗi tham số `values`, thiếu token, sai
  Content-Type, có Origin) trên Word/Excel/PowerPoint thật (`--wps`: WPS); tự mở app riêng, dừng nếu
  port đã có app của người dùng, xong tự đóng. `--record`/`--compare` so kết quả từng lệnh trước-sau
  refactor: bản registry cho kết quả trùng khớp cả 67 lần gọi với bản cũ trên Office 2024
- `tests/mcp-host/test_mcp_host.py` kiểm tra README khớp `commands --markdown` và mô tả tool MCP
  `*_command` liệt kê đủ lệnh

### Docs — README viết lại cho Axiom Office
- Giới thiệu 3 cách dùng (Ask AI, MCP server, HTTP API), sơ đồ kiến trúc Office + WPS, cài đặt
  cho người dùng (gói zip + install.cmd) tách khỏi phần phát triển, bảng tool MCP theo nhóm
- API: bảng endpoint, bảng lệnh đầy đủ (thêm `ui.askpane`, `writer.closeAll`, quy ước `values`
  mảng 2 chiều, `slide` trống = slide cuối), ví dụ PowerShell/Python có token và gửi UTF-8
- Bỏ phần lỗi thời: `build-native.ps1` (chỉ có ở branch `cpp-native-addin`), `docs/office-integration.md`
  (không tồn tại), CLSID cũ; MCP Python gọn lại thành mục legacy; troubleshooting dạng bảng


### Changed — Đổi tên dự án: WPS AI Bridge → **Axiom Office**
- Tên hiển thị (ribbon, task pane, hộp thoại, hướng dẫn cài) và toàn bộ định danh kỹ thuật:
  namespace `AxiomOffice.*`, `AxiomOffice.dll` / `AxiomOffice.Host.exe`, ProgID
  `AxiomOffice.Connect` / `AxiomOffice.AskAiPane`, CLSID mới, khóa `HKCU\Software\AxiomOffice`,
  log `%LOCALAPPDATA%\AxiomOffice`, thư mục `src/AxiomOffice*`, gói `AxiomOffice-<version>-...zip`
- `scripts/legacy.ps1`: `install.ps1` gỡ ProgID/CLSID/Office Addins/whitelist WPS của bản
  `WpsAiBridge` (tránh nạp add-in 2 lần tranh port) và chuyển cấu hình sang khóa mới — chỉ xoá
  khóa cũ sau khi đọc lại thấy khớp từng giá trị (API key DPAPI không dùng entropy nên chép
  nguyên được); `uninstall.ps1` cũng gỡ đăng ký cũ, `-Purge` xoá cả cấu hình/log cũ
- `install.ps1` đọc version từ DLL thay vì ghi cứng

### Fixed — Agent lặp tới "(da dat gioi han 8 buoc)" mà không làm được gì
- Nguyên nhân (log 20:13, "Tạo bảng điểm 5 học sinh"): model gửi `values` dạng
  `{"item":[{"item":[...]}]}`; `et.writeRange` không ghi gì nhưng vẫn trả `ok:true`, model đọc lại
  thấy ô trống nên ghi lại mãi tới khi hết 8 vòng
- `values` của `et.writeRange` / `writer.insertTable` / `wpp.addTable` đọc qua `ParamMatrix`: gỡ lớp
  bọc `{"item": ...}`, nhận cả mảng dạng chuỗi JSON; sai dạng, thiếu `values`/`range` thì trả lỗi kèm
  ví dụ `[["Họ tên","Điểm"],["An",9.5]]` để model tự sửa. Bảng tự suy ra/nới `rows`/`cols` theo
  `values` thay vì cắt bớt dữ liệu. Mô tả tool ghi rõ định dạng mảng 2 chiều
- Bỏ giới hạn số vòng của agent (trước là 8): lượt chạy dừng khi AI trả lời xong, khi bấm Dừng
  hoặc chạm trần 5 phút. Nếu bên gọi tự đặt giới hạn thì kết quả báo "chưa xong" (không còn chuỗi
  `(da dat gioi han n buoc)` giả làm câu trả lời). Log/`ai.ask` trả thêm số vòng (`rounds`)
- Đã chạy lại đúng prompt trên Excel thật: 5 vòng, 16s, bảng 5 học sinh + cột Trung bình


### Added — Đóng gói cài đặt cho người khác
- `scripts/package.ps1`: build (hoặc `-NoBuild`) rồi tạo `dist/AxiomOffice-<version>-<ngày>-<commit>.zip`
  (~245 KB; tên có `-dirty` khi code chưa commit): DLL, `AxiomOffice.Host.exe`, `install.ps1`/
  `uninstall.ps1`, `install.cmd`/`uninstall.cmd` nhấp đúp, `HUONG-DAN-CAI-DAT.txt`, NOTICE template.
  Entry zip dùng `/` (tự ghi từng entry; `CreateFromDirectory` của PowerShell 5.1 ghi `\`)
- `install.ps1` gỡ nhãn Zone.Identifier của DLL/EXE (file từ zip tải về) và in cấu hình MCP với
  đường dẫn exe đúng; add-in log đường dẫn DLL đang nạp (`Connect constructor ... from <path>`)
- Đã test như người nhận: giải nén + gắn Zone.Identifier → `install.cmd` gỡ nhãn, đăng ký trỏ vào
  thư mục gói, Word nạp add-in từ đó, MCP trong gói chạy (office_sessions, doc_create)

### Fixed
- `uninstall.ps1` sót đăng ký COM của Ask AI pane (`AxiomOffice.AskAiPane`, CLSID `{D99F8693-...}`);
  thêm `-Purge` xoá cả `HKCU\Software\AxiomOffice` (token, cấu hình AI) và `%LOCALAPPDATA%\AxiomOffice`


### Added — MCP server C# trong `AxiomOffice.Host.exe mcp` (thay 3 server Python)
- Người dùng không còn phải cài Python/venv/pip: `AxiomOffice.Host.exe mcp` là MCP server stdio
  (JSON-RPC 2.0, protocol 2024-11-05 → 2025-11-25) với cả 50 tool của word/excel/ppt-mcp, cùng
  tên và tham số; `mcp word|excel|ppt` để nạp từng nhóm, `mcp --list` in danh sách
- Làn file đọc/ghi OOXML trực tiếp (ZipArchive + XLinq, không thư viện ngoài): docx (paragraph,
  bảng có ô gộp, section, style, core props), pptx (clone placeholder từ layout như python-pptx,
  ghi chú), xlsx/xlsm (đọc streaming, shared strings, ngày tháng theo numFmt, dịch shared formula
  cho `show_formula`, ghi shared strings, styles font/fill/border/alignment/numFmt, table,
  thêm/chép/đổi tên/xoá sheet), csv/tsv; `.xls` đọc qua Excel/WPS (COM)
- Sửa file chỉ ghi lại phần XML liên quan: chart/ảnh/pivot/macro của file gốc được giữ (openpyxl
  làm mất); ghi công thức thì bỏ calcChain + `fullCalcOnLoad` để Excel tính lại khi mở
- Làn live + `office_sessions()` gọi bridge như bản Python; lỗi kết nối trả `{"ok":false}`
- Template `default.docx`/`default.pptx` của python-docx/python-pptx (MIT) nhúng trong exe
  (`src/AxiomOffice.Host/Mcp/Templates/NOTICE.md`)
- Bỏ: `excel_query` (DuckDB SQL) và xuất parquet của `excel_convert` (chỉ còn csv)
- `tests/mcp-host/test_mcp_host.py`: MCP client Python chính thức + so parity từng tool file
  với `file_tools.py` bản Python trên file do C#, python-docx/openpyxl/python-pptx và Office thật
  tạo (152/152); `office_roundtrip.ps1` mở file C# ghi ra bằng Word/Excel/PowerPoint 16 thật.
  Làn live đã chạy qua Word thật (styled text, heading, undo)


### Added — Event stream SSE `GET /events` (Phase 2)
- Mỗi bridge phát Server-Sent Events: `hello`, `document` (tài liệu active đổi),
  `selection` (Writer: text ≤ 200 ký tự + start/end; ET: sheet + address + giá trị
  ô trên-trái; WPP: slide/type/shape/text), `ping` mỗi 15s
- V1 poll-diff 500ms (không COM event sink — chạy giống nhau Office/WPS), poller chỉ
  chạy khi có subscriber, tối đa 5 subscriber (thứ 6 nhận 503); mỗi kết nối có thread
  ghi riêng nên `/cmd` không bị chặn (đo: 12 ms trong lúc stream); lỗi COM log tối đa
  1 lần/phút mỗi loại
- Python: `bridge.es_hint()` trong cả 3 MCP (URL `/events` + ví dụ curl)
- Đã verify: Word 47831 (selection khi bôi đen, document khi mở file khác, ping),
  Excel 47832 (selection theo địa chỉ `$B$2`, `$C$3:$D$5`), WPS Writer 47821

### Added — Session registry + `GET /session` (Phase 1)
- `Bridge/SessionRegistry.cs`: mỗi bridge ghi `%LOCALAPPDATA%\AxiomOffice\sessions\{pid}.json`
  (`pid`, `app`, `family` office|wps, `port`, `host`, `started`, `lastSeen`, `document`),
  heartbeat 25s, xoá file khi Stop/OnDisconnection; đọc tên tài liệu chạy nền, chờ tối
  đa 2s, host bận thì giữ giá trị cũ (heartbeat không bao giờ trễ vì Word bận)
- `GET /session` (cần token): thông tin bridge + tài liệu đọc live lúc gọi
- Python `bridge.sessions()` trong cả 3 MCP: đọc registry, gọi `/health` (1.5s) để đánh
  dấu `healthy`, prune khi process chết hoặc heartbeat > 90s mà health cũng fail
  (bridge đang bận lệnh dài vẫn được giữ, `healthy: false`) + unittest 5 case
- MCP tool `office_sessions()` cho cả 3 server; docstring `wps_live_command` /
  `word_command` / `ppt_command` hướng dẫn truyền `port` lấy từ danh sách này
- Đã verify: Word + Excel cùng lúc → 2 session đúng port/family; WPS Writer → family `wps`;
  kill Word/WPS → session bị prune; regression `/health` 200, `/cmd` 401/415/403, `ai.ask` ok

### Changed — Ask AI pane thiết kế lại
- Design tokens `PaneTheme` (màu theo vai trò, font Segoe UI/Semibold tạo một lần, spacing,
  radius, scale theo DPI); nút Ask/link đổi sang `#4F46E5` vì trắng trên `#6366F1` chỉ đạt
  4.47:1; viền ô nhập `#8C93A0` (3.09:1)
- Empty state có tiêu đề theo loại tài liệu + 3 chip gợi ý xếp dọc (theo host Writer/ET/WPP);
  bubble có "đuôi", nhận Tab/Ctrl+C và menu Sao chép; dòng tool activity (tick xanh / x đỏ +
  nhãn tiếng Việt + mã action, tooltip là dòng log gốc); typing 3 chấm tôn trọng reduced motion
- Composer tự cao 2–5 dòng, Ask chỉ bật khi có chữ, khoá kèm hướng dẫn khi chưa cấu hình AI;
  footer có trạng thái + chấm "đang chạy" + link **Dừng** / **Chèn trả lời**; thẻ lỗi có
  **Thử lại** / **Mở Cài đặt**; trạng thái xong/lỗi được báo cho trình đọc màn hình
- `PromptBox`: edit multiline không phát `EN_CHANGE` khi text đặt bằng `WM_SETTEXT` (UIA,
  automation) — tự báo `TextChanged` để Ask bật

### Fixed
- **Word crash (AV `wwlib.dll`) khi nhiều thread cùng gọi COM**: heartbeat đọc
  `ActiveDocument` đúng lúc `ai.ask` đang ghi → cuộc gọi thứ hai được Word dispatch
  reentrant. `Bridge/ComGate.cs` tuần tự hoá mọi truy cập object model của bridge (Pump,
  agent của pane, heartbeat, poller); heartbeat/poller bỏ qua lượt khi cổng bận
- **Agent chạy mãi không kết thúc / kết thúc âm thầm** (BUG-1): trần 5 phút cho cả lượt
  (`LlmClient.AgentTimeoutMs`), hủy qua `CancellationToken` (abort request LLM đang chờ,
  không chạy thêm tool); log kết quả ghi ngay trên worker; `BeginInvoke` thất bại và lỗi
  trong callback UI đều được log thay vì `catch {}`; `ai.ask` log từng tool (`ai.ask progress:`).
  Không tái hiện được treo trên build hiện tại (cùng prompt bảng điểm: 16.9s, 2 tool calls)
- **Tài liệu ẩn** (BUG-2): cửa sổ `OpusApp` ẩn không title là của Word, có từ lúc khởi động
  (trước khi tạo pane). Log trạng thái tài liệu/cửa sổ ở `OnConnection`, `OnStartupComplete`,
  lệnh bridge đầu tiên và trong `app.info.state`; `writer.*` kích hoạt tài liệu đang hiển thị
  nếu `ActiveDocument` không có cửa sổ visible
- **Chip gợi ý không được layout lúc mở pane** (BUG-3): empty state đi qua `ChatList.AddBlock`
- **Task pane hẹp trên WPS** (BUG-6): pane đo lại độ rộng sau khi host layout, quy đổi đơn vị
  CTP theo tỉ lệ đo được và nới về 360px; WPS 12.1.0.28485: 250 → 360px
- `ui.askpane` tạo CTP trên UI thread chính (trước đây chạy trên thread HTTP nên RCW của pane
  thuộc apartment khác)
- Danh sách chat không cuộn hết xuống cuối (AutoScroll bỏ qua Padding đáy; scrollbar xuất hiện
  làm bubble cao thêm): `AutoScrollMargin` + giữ vị trí cuối khi đo lại
- **Build làm mất DLL** (BUG-8): `build.ps1` hỏi Restart Manager tiến trình nào đang lock DLL/EXE
  (assembly .NET không hiện trong `Process.Modules`), dừng kèm tên + pid hoặc `-Kill`; biên dịch
  vào `.stage` rồi mới chép đè; dọn `<guid>_AxiomOffice.dll` còn sót

### Ideas (chưa làm)
- Event stream là nền cho timeline/transaction sau này: thêm event `change` (hash nội dung
  theo đoạn) để agent biết user vừa sửa gì mà không cần đọc lại cả tài liệu
- Poll-diff đủ cho selection/document; khi cần độ trễ thấp hơn có thể chuyển Writer sang
  COM event sink (`WindowSelectionChange`) nhưng phải giữ đường poll cho WPS


### Added — Ask AI dạng Task Pane + AI agent thao tác live
- **Ask AI = task pane dock trong app** (`ICustomTaskPaneConsumer`/`ICTPFactory`
  — cùng API cho Microsoft Office và WPS), fallback cửa sổ nổi nếu host không
  hỗ trợ; pane chat có Ask / Insert reply / Settings, transcript từng bước
- **Agent mode**: LLM **tool-calling** (OpenAI + Anthropic) gọi tool
  `office_action` → thực thi `writer.*/et.*/wpp.*` **trực tiếp lên tài liệu đang
  mở** (real-time trước mặt user); mỗi action Word = 1 Ctrl+Z (UndoRecord);
  tự fallback chat thường nếu provider không hỗ trợ tools
- Bridge command mới **`ai.ask {prompt}`** — gọi agent từ bên ngoài qua HTTP
- Debug notes (ghi lại để đời):
  - `CTPFactoryAvailable` tham số phải khai báo `object` (marshal trực tiếp
    `ICTPFactory` fail âm thầm — host vẫn "gọi" nhưng QI lỗi)
  - `ICustomTaskPaneConsumer` là **dispinterface**: khai báo `IUnknown` →
    host gọi vtable slot IDispatch → **crash WINWORD** (0xc0000005) ngay sau
    `OnAddInsUpdate`, trước `GetCustomUI`
- Đã verify trên Word 2024: prompt *"Soạn cho tôi một mẫu đơn xin việc"* →
  4 tool calls (getText → insertStyledText header → heading → insertStyledText
  thân đơn), 41s, docx 1326 ký tự tiếng Việt chuẩn Unicode

### Added — MCP server Word (`tools/word-mcp`) và PowerPoint (`tools/ppt-mcp`)
- `word-mcp` (19 tools): làn file python-docx (profile/get_text/find_text/
  extract_table/create — atomic save) + làn live qua bridge (health, command,
  read/type/styled text, format_selection, heading, insert_table/image/hyperlink,
  replace_all, export_pdf, undo, save) — mặc định Microsoft Word, `app="wps"` cho WPS
- `ppt-mcp` (15 tools): làn file python-pptx (profile/get_text/create/add_slide_file)
  + làn live (health, command, list/add slide, add_text, add_image, add_table,
  set_notes, delete_slide, export_pdf, save)
- Mỗi server: venv riêng, run.cmd, unittest (word 5/5, ppt 4/4), MCP protocol
  roundtrip pass trên Word/PowerPoint thật
- **fix**: `writer.undo` dùng `Document.Undo` (Word object model không có
  `Application.Undo`); undo theo UndoRecord hoạt động end-to-end — `undo x2`
  hoàn tác heading + table đúng 2 bước về text gốc

### Added — Live command pack v2 (tham khảo ppt-mcp / word-mcp-live)
- **Word** (10 lệnh mới): `insertStyledText`, `formatSelection`,
  `setParagraphAlignment`, `insertTable` (kèm dữ liệu + style), `insertPageBreak`,
  `insertImage`, `insertHyperlink`, `heading` (Heading 1-9, tự xuống dòng),
  `undo`, `exportPdf` — mọi thao tác ghi được bọc trong **Word UndoRecord**
  (mỗi action của AI = 1 bước Ctrl+Z, pattern từ word-mcp-live)
- **Excel** (4): `formatRange` (bold/italic/fontSize/fontColor/fillColor/numFmt/
  horizontal/wrap), `activateSheet`, `exportPdf`, `undo`
- **PowerPoint** (7): `addSlide` có `layout`, `addText` (fontSize/bold/color/
  align), `addImage`, `addTable` (kèm dữ liệu), `setNotes` (speaker notes),
  `deleteSlide`, `exportPdf`
- Đã kiểm chứng trên **Microsoft Office 2024 thật** (3 app chạy song song):
  Word chèn bảng/hyperlink/ảnh + PDF (48KB); Excel format + PDF (25KB) +
  saveAs đúng; PowerPoint slide/text/table/notes + PDF (27KB) + pptx (40KB);
  heading có paragraph break (`Tieu de\rThan bai.`)

### Fixed/Added — vòng 2 theo code review (#6 + minors)
- **Retry khi app bận**: request tự retry khi gặp `RPC_E_CALL_REJECTED` /
  `RPC_E_SERVERCALL_RETRYLATER` / `RPC_E_CALL_CANCELED` / `VBA_E_IGNORE`
  (0x800AC472 — Excel đang gõ ô) — tối đa 10 lần, backoff luỹ tiến ~5.5s,
  log mỗi lần retry; an toàn vì lỗi "call rejected" nghĩa là call chưa thực thi
- **Giá trị lỗi Excel**: `#DIV/0!` `#VALUE!` `#NAME?` `#REF!` `#NUM!` `#NULL!`
  trả về tên chuỗi thay vì số âm (probe thực tế trên Excel 16.0); `#N/A` trùng
  mã 0x800A07FA với ô trống qua `Value2` nên vẫn về `null` — giới hạn đã biết
- **Version đồng bộ**: `BridgeVersion` lấy từ assembly version (health trả
  `1.0.0` thay vì `0.1.0`)
- **DPAPI**: `LlmApiKey` mã hóa bằng `ProtectedData` (CurrentUser); key
  plaintext cũ vẫn đọc được (tương thích ngược); `SettingsForm` lưu qua
  `WriteSecret`; đã migrate key thật sang dạng mã hóa + `llm-test` gọi LLM
  qua key DPAPI thành công
- **Tests**: `tools/excel-mcp/tests/test_file_tools.py` — 9 unit test
  (roundtrip, show_formula, ragged write, format/table, sheet ops, DuckDB
  sandbox, locked-file không sót temp, csv) — chạy bằng
  `python -m unittest discover -s tests`

### Fixed — bảo mật + đúng đắn (theo code review)
- **CSRF**: bridge từ chối request có header `Origin`; `POST /cmd` bắt buộc
  `Content-Type: application/json` (chặn browser "simple request"); token
  ngẫu nhiên 32 hex tự sinh khi chạy `install.ps1`; `bridge.py` tự đọc token
  từ registry và gửi `X-Auth-Token`
- **et.writeRange**: dùng `Range.Resize[rowCount, colCount]` — ghi đúng ma trận
  khi truyền anchor 1 ô ("A1", trước đây Excel thật chỉ ghi được ô đầu); colCount
  lấy max qua các dòng (ragged rows được pad null thay vì cắt âm thầm)
- **Ribbon port**: nút Status / Copy API URL hiển thị đúng port theo host
  (truyền `IsOfficeHost` — Office hiện 4783x thay vì 4782x)
- `bridge.py`: `APP_PORTS` thêm `word/excel/ppt` (47831-47833); `wps_health`
  nhận tham số `port`
- **An toàn ghi file (excel-mcp)**: mọi thao tác ghi đi qua file tạm cùng thư
  mục + `os.replace` (atomic — lỗi giữa chừng không làm hỏng file gốc, vẫn giữ
  hint khi file bị khóa); docstring cảnh báo openpyxl có thể mất chart/image/pivot
- **Sandbox `excel_query`**: `SET enable_external_access=false` — SQL không thể
  `read_csv`/`COPY TO`/`INSTALL`/`ATTACH` (chống prompt injection đọc/ghi file
  tùy ý); đã test chặn cả 3 đường

### Added — Office ports + companion Office + excel-mcp nâng cấp (branch `main`)
- **Port tách theo host**: WPS giữ `Port` 47821-47823; Microsoft Office dùng
  `PortOffice` mới (mặc định 47831-47833) — mở song song WPS + Office thật
  không còn đụng port (bỏ workaround cũ)
- **Companion hỗ trợ Microsoft Office**: `word|excel|ppt` →
  `Word/Excel/PowerPoint.Application`; log `Companion resolved application: ...`
  (cảnh báo khi ProgID bị WPS đăng ký đè ở HKCU — trên máy dev này WPS chiếm
  cả 3 ProgID, companion sẽ tạo WPS compat component version 12.0; Office thật
  dùng in-proc add-in khi mở app)
- **excel-mcp nâng cấp** (tham khảo negokaz/excel-mcp-server): thêm
  `excel_create_sheet`, `excel_copy_sheet`, `excel_rename_sheet`,
  `excel_delete_sheet`, `excel_format_range` (font/fill/border/alignment/
  numFmt/decimalPlaces), `excel_create_table`; `excel_read` thêm
  `show_formula`; tổng 16 tools; migrate SDK `mcp` 2.x (`MCPServer`)
- Kiểm chứng trên file thật `bank-additional-full.xlsx` (41k dòng): profile
  0.01s; query trực tiếp ~5s; convert parquet + query 0.023s; MCP protocol
  roundtrip 16 tools pass

### Verified — Microsoft Office 2024 ProPlus x64 (branch `main`)
- Word (`WINWORD.EXE`), Excel (`EXCEL.EXE`), PowerPoint (`POWERPNT.EXE`): add-in
  nạp đầy đủ lifecycle (OnConnection → OnAddInsUpdate → GetCustomUI →
  OnStartupComplete), ribbon tab hiển thị, bridge hoạt động — không crash,
  không cần sửa signature nào thêm
- Excel/PowerPoint đã E2E trên app thật: ghi/đọc range (Excel), tạo slide +
  textbox + đọc lại (PowerPoint)
- Fix chuẩn hoá ô trống Excel: VT_ERROR `0x800A07FA` / `0x80020004` giờ trả
  `null` trong `readRange` (trước đó trả số `-2146826246`)
- Ghi chú: WPS ET và Excel thật cùng map port 47822 — cần đổi `Port` base khi
  chạy song song hai hệ

### Added — Ribbon UI + AI in-app + MCP server (branch `main`)
- Ribbon tab **"Axiom Office"** (`IRibbonExtensibility.GetCustomUI`, XML chuẩn
  2006/01): nhóm Local bridge (Status / Copy API URL / Open Log) + nhóm AI
  (Ask AI..., Settings); callback qua IDispatch (`ClassInterfaceType.AutoDispatch`)
- **AI Settings**: dialog cấu hình provider (OpenAI-compatible | Anthropic),
  endpoint, API key, model; nút Test gọi thử; lưu vào
  `HKCU\Software\AxiomOffice` (LlmProvider/LlmEndpoint/LlmApiKey/LlmModel)
- **Ask AI**: dialog prompt + context (None / Selection / Whole document
  ≤50k chars), gọi LLM async không treo UI, chèn kết quả vào tài liệu
  (Insert at cursor); `DocumentContext` đọc toàn văn Writer, used range ET
  (TSV, xử lý mảng 2D COM lower-bound=1), text các slide WPP
- `GET /config`: trả provider/endpoint/model + API key đã che (dạng `sk-a...7e5f`)
- Companion: chế độ `llm-test` kiểm tra cấu hình LLM từ CLI
- `tools/excel-mcp`: MCP server Python (venv riêng: mcp, openpyxl, xlrd,
  duckdb, pandas)
  - Tools file: `excel_profile`, `excel_read` (paging), `excel_query` (DuckDB
    SQL trên bảng "data"), `excel_write`, `excel_create`, `excel_convert`
    (parquet/csv)
  - Tools live: `wps_health`, `wps_live_command`, `wps_live_read_range`,
    `wps_live_write_range` — gọi HTTP bridge 47821-47823
  - Hỗ trợ `.xlsx/.xlsm/.xls/.csv/.tsv`; nhận dạng định dạng bằng magic bytes;
    xử lý file mislabeled (nội dung xlsx nhưng đuôi .xls) qua stream-based
    openpyxl; ghi .xlsm giữ VBA; PermissionError có hint "file đang mở"

### Branch `cpp-native-addin` (f2f0f00) — WIP port C++ native

### Added
- Port add-in in-proc sang C++ native thuần (không CLR):
  - `Json.h/cpp` — JSON parser + serializer UTF-8, không dependency
  - `Http.cpp` — HTTP server Winsock bound 127.0.0.1 (`/health`, `/cmd`)
  - `Com.h/cpp` — IDispatch late-binding, VARIANT ↔ JSON, SAFEARRAY 2D cho `Range.Value2`
  - `Commands.cpp` — đầy đủ parity command set với bản C#
  - `Addin.cpp` — COM object với raw vtable `[IUnknown + IDispatch + IDTExtensibility2]`,
    IClassFactory, `DllGetClassObject`/`DllCanUnloadNow`, marshaling con trỏ
    `Application` qua thread bằng `CoMarshalInterThreadInterfaceInStream`
- `scripts/build-native.ps1` — build bằng `cl.exe` VS2022 BuildTools, `/MT`
  (static CRT — WPS không cần VC runtime), x64, DLL ~320KB
- `scripts/install.ps1` — tự ưu tiên native DLL nếu có: `InprocServer32` trỏ
  thẳng DLL (bỏ mscoree/CodeBase/versioned subkey), thêm `ProgId` + `TypeLib` subkey

### Known issues
- WPS 12 nạp DLL native, tạo instance, QI `IDTExtensibility2` +
  `IRibbonExtensibility` + `ICustomTaskPaneConsumer` nhưng **không invoke
  `OnConnection`** → ghi `AddinsCL` (disable). Gate của WPS cho native add-in
  chưa xác định (nghi: WPS chỉ chạy đầy đủ flow `IDTExtensibility2` cho add-in
  managed qua mscoree). Bản C# trên `main` vẫn là đường chạy chính thức.

## [0.1.0] — 2026-09-29

Commit `a7b1132` (branch `main`). Bản đầu tiên chạy hoàn chỉnh trên WPS 12.1 x64.

### Added
- **In-proc add-in** `WpsAiBridge.dll` (C#, .NET Framework 4.8, x64):
  COM add-in `IDTExtensibility2`, mở HTTP server (HttpListener) trên localhost,
  dùng chung command dispatcher với companion.
- **Companion** `WpsAiBridge.Host.exe`: process riêng dùng COM automation
  (`KWPS/KET/KWPP.Application`) cho app chưa mở hoặc khi add-in không nạp;
  tự chuyển idle nếu port đã do add-in in-proc phục vụ.
- HTTP API: `GET /health`, `POST /cmd` với `{"action", "params"}`; token tuỳ
  chọn qua header `X-Auth-Token`.
- Command set: `app.info`; writer (`newDocument/open/getText/typeText/appendText/
  replaceAll/selection/save/saveAs`); et (`newWorkbook/open/listSheets/readRange/
  writeRange/save/saveAs`); wpp (`newPresentation/open/listSlides/addSlide/
  addTextBox/save/saveAs`).
- Port riêng từng app: 47821 Writer / 47822 Spreadsheets / 47823 Presentation
  (base +0/+1/+2, đổi qua `HKCU\Software\WpsAiBridge`).
- `scripts/build.ps1` (Roslyn csc, không cần dotnet SDK), `install.ps1`,
  `uninstall.ps1` — toàn bộ đăng ký trong HKCU, không cần admin:
  `Classes\CLSID` (mscoree + CodeBase), `Microsoft\Office\*\Addins`
  (`LoadBehavior=3`), `Kingsoft\Office\{WPS,ET,WPP}\AddinsWL` (whitelist).
- Logger dùng chung tại `%LOCALAPPDATA%\WpsAiBridge\bridge.log`.

### Fixed — root cause vụ crash `clr.dll` khi WPS nạp add-in
Triệu chứng: `wps.exe` crash `0xc0000005` trong `clr.dll` (exit code `80131506`)
ngay khi nạp add-in; WPS auto-restart, tự hạ `LoadBehavior 3→2`, ghi
`AddinsCL\WpsAiBridge.Connect=1` (disable) và hiện popup "running into problems".
Không có log nào từ `OnConnection`.

Quá trình khoanh vùng và fix:

1. **Cô lập host**: class `Probe` (không implement `IDTExtensibility2`) tạo được
   instance, không crash → lỗi nằm ở đường invoke `IDTExtensibility2`, không
   phải CLR-hosting nói chung.
2. **Loại trừ nghi phạm**: bỏ dần dynamic COM probe (`DetectAppKind`) khỏi
   `OnConnection`, tạm bỏ HttpBridge, thêm log từng bước → `OnConnection` chạy
   xong hoàn toàn rồi mới crash ở bước kế tiếp.
3. **Xác định điểm chết**: WPS invoke callback kế tiếp (`OnStartupComplete`) và
   marshal tham số `custom` khác kiểu so với `OnConnection` (VARIANT không phải
   SAFEARRAY) → CLR dispatch marshaler AV.
4. **Fix chính**: giữ `IDTExtensibility2` `[InterfaceIsIDispatch]` + `[DispId(1..5)]`,
   nhưng:
   - `OnConnection` dùng `[MarshalAs(UnmanagedType.SafeArray)] ref Array custom`
     (đúng shape WPS yêu cầu — đổi sang `ref object` thì WPS bỏ qua không gọi);
   - 4 callback còn lại (`OnDisconnection/OnAddInsUpdate/OnStartupComplete/
     OnBeginShutdown`) dùng `ref object custom` (VARIANT passthrough).
5. **Resiliency của WPS**: sau mỗi crash WPS tự hạ `LoadBehavior=2` khiến các
   lần sau add-in không được nạp — khi debug phải set lại `LoadBehavior=3` và
   xoá `AddinsCL` mỗi lần.
6. **Kiến trúc WPS 12**: mọi component chạy chung binary `wps.exe` với flag
   `/wps` / `/et` / `/wpp` → nhận diện app bằng COM probe (`Documents` /
   `Workbooks` / `Presentations`) trước, fallback tên process.

### Verified end-to-end
- Writer: `newDocument → typeText → getText → saveAs` (.docx ~10KB) OK
- Spreadsheets: `newWorkbook → writeRange A1:B2 → readRange → saveAs` (.xlsx) OK
- Presentation: `newPresentation → addSlide → addTextBox → listSlides → saveAs`
  (.pptx ~57KB) OK
- `/health` đúng cho cả 3 app chạy song song trên 3 port
