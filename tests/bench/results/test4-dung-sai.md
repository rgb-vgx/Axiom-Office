# Kiểm tra tính ĐÚNG ĐẮN của workbook (không phải cấu trúc)

Bốn báo cáo trước so **cấu trúc**: tên sheet, số ô công thức, số chart. Đề benchmark chấm bằng
"Kết quả phải đúng về dữ liệu, công thức, logic nghiệp vụ", nên đợt này đọc **giá trị và công thức**, và
chạy **Phase 4 của đề** (đổi một giả định rồi xem chuỗi phụ thuộc có cập nhật không).

Cách làm: mở file thật trong LibreOffice, đọc công thức từ gói ODF (`table:formula`), rồi **đổi ô rồi đọc
lại** — vì đọc không phân biệt được một ô điều khiển còn sống với một ô đã chết.

## Axiom — Test 4 wide (bài đã hoàn thành, tự báo 18/18 PASS)

### Mô hình là thật

`Statistics` toàn công thức: `AVERAGE(Simulation.$H$3:.$H$10002)`, `PERCENTILE(...)`, `SKEW(...)`,
`COUNTIF(...)`, và cột Assessment dùng `IF` lồng. `Simulation` có 10.000 dòng với đủ chuỗi tính:
`H=B*(1+E)^duration`, `I=H*C`, `J=H-I-D`, `K=J-Investment/duration`, `M=(duration*J-Investment)/Investment`.

Kiểm số học bằng tay trên dòng 3, **khớp chính xác**: `I = 576821*0.672202 = 387740` ✓,
`J = 576821-387740-132132 = 56948` ✓, `K = 56948.1-500000/5 = -43051.9` ✓,
`M = (5*56948.1-500000)/500000 = -0.430519` ✓.

**Không tìm thấy kết quả ghi cứng.** Cột Status của `Checks` cũng là công thức
(`=IF(D5=C5;"PASS";"FAIL")`), không phải chữ.

### Nhưng có một ô điều khiển CHẾT

Trang `Inputs` có dòng:

```
A22: Seed Value (1 - 9,999,999) [change to re-run]   B22: 20250101   C22: integer
```

Người dùng được bảo đổi `B22` để chạy lại mô phỏng. Sự thật:

| Ô | Ai đọc nó |
|---|---|
| `B22` (giá trị seed được dán nhãn) | **không công thức nào** (quét toàn gói: 0) |
| `C25` (trống) | **60.006 công thức** — `MOD(SIN([Inputs.$C$25]+…)*10000;1)` |
| `C22` = chữ `"integer"` | check #15 của sheet Checks |

Thí nghiệm có đối chứng (đổi ô, đọc `Simulation!H3`, rồi trả lại):

```
H3 ban dau (seed that = trong, tuc 0)      576820.751433658
doi C25 := 999                             1750908.845488366   DOI        <- phep do co tac dung
doi B22 := 777                             1750908.845488366   KHONG DOI  <- o duoc dan nhan khong noi gi
tra C25 ve trong, B22 ve 20250101          576820.7514336582   (dung lai chinh xac)
```

Và check #15 tự khai là PASS bằng cách **đem so chữ với số**:

```
D19: =IF([Inputs.$C$22]>=1;"OK";"MISSING")
```

`C22` là chữ `"integer"`, so `>= 1` ra TRUE, nên PASS. Kiểm tra tính tái lập đang so **sai cột**, và không
ai biết vì phép so im lặng nhận giá trị chữ.

### Hệ quả

Sheet `Checks` báo **"ALL CHECKS PASSED", 18/18** — trong khi mô hình đang chạy với seed = 0 và ô điều
khiển mà tài liệu chỉ cho người dùng **không nối vào đâu cả**. Đây đúng loại lỗi mà đề gọi là nghiêm
trọng, và là lý do không thể tin lời kể của agent: chính agent đã chạy `verified=true`.

## Axiom — Test 1 wide: chuỗi phụ thuộc CHẠY ĐÚNG

Cùng phép thử, kết quả ngược lại — nên đây không phải "Axiom luôn sai":

| Phép thử | Kết quả |
|---|---|
| `Assumptions!C5` Revenue Growth 0.1 → 0.2 | `KPI!C4` 60.789.554 → **66.315.877** (= 55.263.231 × 1,2, chính xác); `Forecast!B7` 4.508.559 → 4.918.428 (= ×1,2/1,1, chính xác) |
| `Assumptions!C19` Scenario Selector 1 → 2 | `Forecast!B4` "Base Case" → **"Upside Case"**; `Forecast!B7` → 5.635.698 |
| Trả cả hai về giá trị cũ | về **đúng y giá trị ban đầu** |
| `PnL` (sheet ghi rõ "FY2024 Actual") | **không đổi** — và đó là đúng: số lịch sử không phụ thuộc giả định FY2025 |

## Claude Code — Test 4: cả hai ô điều khiển đều sống

| Phép thử | Kết quả |
|---|---|
| `Inputs!B17` Random seed 20261002 → 777 | `Simulation!B4:E4` 12.710.028 → **9.554.987** (ĐỔI) |
| Trả về 20261002 | về **đúng y** 12.710.028,825274462 |
| `Inputs!B19` Market growth stress shift 0 → 0.03 | `Simulation!G4` 0,0702 → **0,1002** (đúng +0,03); `K4` 82.572.496 → **89.063.587** |
| Trả về 0 | về **đúng y** |

Cả hai ô đều có công thức đọc chúng, có nhãn nói rõ tác dụng, và hoàn tác chính xác từng chữ số.

## Cải tiến đã làm

**Luật kiểm chứng phải THỬ, không chỉ ĐỌC** (`model.VerifyWorkNudge` + `agent.CheckRule`, commit
`f0e9dbe`). Đọc lại bắt được ô trống; nó **không** bắt được một ô điều khiển không nối vào đâu. Luật mới:
với thứ có đầu vào cho người dùng chỉnh (giả định, driver, seed) thì **đổi một cái, đọc lại những ô lẽ ra
phụ thuộc, xác nhận chúng thật sự nhúc nhích, rồi trả lại** — chỉ cách đó mới phân biệt được đầu vào còn
sống với đầu vào đã chết.

Đây là bài học đắt nhất của cả đợt đối chiếu: agent tự kiểm chứng (`verified=true`) vẫn ra một workbook
18/18 PASS với một ô điều khiển chết, vì bước kiểm chứng chỉ đọc.

## Chạy lại với luật "phải thử": lỗi CŨ, chỗ MỚI

Chạy lại Test 4 (16:00, ngân sách 1M, lưu `result/axiom-test4try.ods`) để xem luật mới có bắt được ô
điều khiển chết không. **Chưa kết luận được**, vì lượt chạy dừng ở trần 300 vòng ngay giữa lúc dựng bài
(`run.stopped`, không có `verified`) — **bước kiểm chứng chưa hề chạy**, nên luật mới chưa được dịp làm
việc.

Nhưng nó cho một dữ liệu mới: Inputs sheet lần này **thẳng cột** (nhãn A, giá trị B, đơn vị C, ghi chú E —
không còn lệch như lần trước), và vẫn mắc **đúng lỗi cũ ở một dòng khác**:

```
A34: REPRODUCIBILITY — FIXED RANDOM SEED     (tiêu đề mục, B34 TRỐNG)
A35: Random Seed   B35: 20250101             (giá trị seed)
E35: "Change the seed and press F9 / Ctrl+Shift+F9"

quét: Inputs.$B$35 ->     0 công thức
      Inputs.$B$34 -> 80.008 công thức        <- tiêu đề mục
      Inputs.$B$36 ->     0 công thức        (Simulation Count = 10.000 cũng không ai đọc)
```

Lần trước lệch **một cột** (`B22` được dán nhãn, công thức đọc `C25`); lần này lệch **một dòng**
(`B35` được dán nhãn, công thức đọc `B34`). Cùng một kiểu sai: ô điều khiển đặt cạnh nhãn nhưng công
thức trỏ vào ô bên cạnh hoặc ô trên. Và vì `B34` chứa **chữ**, phép tính coi nó là 0 — seed lại bằng 0,
không báo lỗi gì.

Đây là dữ liệu ủng hộ luật "phải thử": đọc lại không phân biệt được `B34` với `B35`, chỉ đổi thử mới
biết. Nhưng phải chạy một lượt tới được bước kiểm chứng mới nói được luật có bắt hay không — lượt tiếp
theo đặt trần 1.000 vòng vì lý do đó.

## Ba lần chạy Test 4, và luật "phải thử" chưa được chứng minh

| Lần | Thời điểm | Seed nối đúng? | Đạt bước kiểm chứng? |
|---|---|---|---|
| `axiom-test4wide` | 11:57 (TRƯỚC khi thêm luật) | **KHÔNG** — nhãn ở `B22`, công thức đọc `C25` (trống) | có (`verified=true`) |
| `axiom-test4try` | 15:49 (sau luật) | **KHÔNG** — nhãn ở `B35`, công thức đọc `B34` (tiêu đề mục) | **không** — dừng ở trần 300 vòng |
| `axiom-test4verify` | 16:19 (sau luật) | **CÓ** — công thức đọc `Inputs.$B$17`, kèm cả công thức kiểm tra seed hợp lệ | **không** — hết ngân sách ở vòng 136 |

Lượt thứ ba kiểm chứng bằng phép thử thật: đổi `Inputs!B17` 20.250.101 → 777 làm market growth của dòng
8 đi từ `0,003087` sang `-0,026322` và doanh thu từ `64.841.754,59` sang `44.671.196,33`; trả về thì
khớp lại **chính xác từng chữ số**. Đây là workbook 8/8 sheet, 301.440 ô công thức, có chart, và ô điều
khiển **sống** — ngang chất lượng file của Claude Code ở khía cạnh này.

**Nhưng không được phép nói luật mới là nguyên nhân.** Hai lý do:

1. **Bước kiểm chứng chưa hề chạy ở cả hai lượt sau.** `Verify` chỉ bật khi model tự quyết là đã xong
   (không còn tool call). Ở bài này model gọi tool liên tục rồi hết ngân sách/vòng trước khi "xong", nên
   lời nhắc "phải thử" **chưa bao giờ được gửi**. Thứ duy nhất khác đi là câu trong system prompt
   (`CheckRule`) — có mặt ở mọi vòng của cả hai lượt.
2. **Một lượt sai, một lượt đúng — với cùng câu đó trong prompt.** Không thể quy cho luật; đây là dao
   động giữa các lần chạy. Đúng/sai của ô seed là **chập chờn**, không phải đã sửa.

Kết luận có bằng chứng: **một câu trong system prompt không đủ.** Cơ chế bắt lỗi phải chạy **trong lúc
dựng bài**, không phải chỉ khi model tự tuyên bố xong — vì ở bài lớn nó không bao giờ tuyên bố xong.

## Còn lại, chưa làm

- **Chưa soát giá trị của Test 2 và Test 3** (Inventory 50.000 SKU, PPM 5.000 task) — mới làm Test 1 và
  Test 4. Cùng cách làm, chỉ tốn thêm thời gian.
- **Chưa chạy lại Axiom sau khi thêm luật "phải thử"** để đo xem nó có bắt được lỗi kiểu này không. Cần
  một lượt chạy mới; luật vừa thêm chưa có bằng chứng chạy thật.
- **`et.readRange` chỉ trả giá trị, không trả công thức.** Muốn xem công thức phải mở gói ODF bằng script
  ngoài. Một action đọc công thức (`et.readRange` thêm tham số `formulas: true`) sẽ giúp agent tự soát
  công thức ngay trên tài liệu đang mở — và giúp cả người viết test.
