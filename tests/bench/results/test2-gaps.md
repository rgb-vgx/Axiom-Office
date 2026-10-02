# Test 2 — Inventory Optimization: đối chiếu Claude Code với Axiom Core

Đề: `C:\Users\ThuyetMT\test\benchmark\test2.txt` (12 sheet, tối thiểu **50.000 SKU-location**, 12 tháng
lịch sử nhu cầu, tồn kho an toàn, điểm đặt hàng lại, EOQ, rủi ro, tối ưu hoá, dashboard, checks).
Cả hai bên chạy trên LibreOffice thật.

| | Claude Code | Axiom Core |
|---|---|---|
| Cách nói với LibreOffice | UNO socket trên instance headless riêng | HTTP bridge của extension, trên tài liệu đang mở |
| Thời gian | 05:15:45 → 06:00:54 (**45 phút**) | 06:59:20 → 08:25:33 (**86 phút**) |
| Tool call | 141 (Bash 102, Edit 23, Write 9, Read 6, TaskStop 1) | 353 |
| Token | ra 618.800 / cache đọc 60.698.240 | ra 255.260 / cache đọc 3.702.912; **phải trả 444.559** |
| Kết thúc | hoàn thành | `token budget exceeded (billable 444559 > 400000)` |
| File | `Inventory_Optimization_Workbook.ods` (64,9 MB) | tài liệu đang mở |

## Claude Code đã làm gì

Nguồn: `tests/bench/results/test2-claude-code.summary.txt`,
`tests/bench/results/test2-claude-code-output.txt`.

Cùng lối như Test 1 nhưng nặng hơn: `genmodel.py` sinh mô hình dữ liệu, `ods_core.py` là lớp ghi ODS
trực tiếp (tự dựng gói ODS thay vì qua UNO cho phần lớn dữ liệu — nhanh hơn nhiều khi phải ghi 455.813
dòng), `build_workbook.py` sửa 10 lần, `verify.py` + `checks_sheet.py` soát lại, `postprocess.py` chỉnh
sau. Cuối bài nó tự nói: "the last two need LibreOffice's bundled Python for UNO".

Kết quả kiểm chứng trên `Inventory_Optimization_Workbook.ods`:

```
sheets (12): Raw_SKU_Data | Product_Master | Demand_History | Inventory | Forecast |
             Safety_Stock | Reorder_Point | EOQ | Risk | Optimization | Dashboard | Checks
ô công thức: 2.156.652        chart objects: 4
conditional formatting: 34    named expressions: 1
```

## Axiom Core đã làm gì

Nguồn: `tests/bench/results/test2-axiom-calls.txt`, `test2-axiom-output.txt`,
SSE log `C:\Users\ThuyetMT\test\bench\axiom-test2.log`.

Phân bố tool call — `et.fillRange` chiếm **169/353 (48%)**:

```
et.fillRange 169 | et.formatRange 66 | et.writeRange 56 | et.readRange 30
et.listSheets 14 | et.addSheet 11 | et.addChart 3 | et.renameSheet 1 | load_skill 3
27 lời gọi lỗi (tất cả là "Busy: LibreOffice did not answer within 60s")
```

Tài liệu để lại (`test2-axiom-output.txt`):

```
Raw_SKU_Data .. Checks           12/12 tên sheet đúng đề
Demand_History    6001 x 8       SKU | Category | Month_Index | Monthly_Demand
Safety_Stock     50001 x 11      SKU | Location | Service_Level | Z_Score
Reorder_Point    50001 x 11      SKU | Location | Avg_Daily_Demand | Lead_Time
EOQ              50001 x 11      SKU | Location | Annual_Demand | Order_Cost
Dashboard          78 x 18       "INVENTORY OPTIMIZATION DASHBOARD"
Checks             26 x 9        Check_ID | Check | Scope | Value  ("DQ-01 Total SKU-location
                                 records loaded", Raw_SKU_Data, 50000)
```

Sáu sheet còn lại không soát được vì `et.checkRange` trả đúng lỗi hết giờ 60s ở trên.

Điểm đáng chú ý: tới vòng 42 model tự nhận ra giá trị của lệnh mới —
*"Relative rows adjust correctly (E2,E3,E4). So I can fill a whole block with one call! This is a huge
efficiency win. Now let me plan the sheets with block fills."*

## Khoảng cách

| # | Khoảng cách | Bằng chứng | Vì sao |
|---|---|---|---|
| 1 | **Hết giờ 60s khi LibreOffice tính lại bảng lớn** | 27/353 lời gọi trả `Busy: LibreOffice did not answer within 60s`; agent tự chẩn đoán: *"the Checks write timed out (60s) because the full-column SUMPRODUCT over 50k rows × 10 checks recalculated"* | Ba tầng thời gian lệch nhau: gate extension 60s, Core 90s, MCP live 330s. Tầng hẹp nhất cắt trước, nên thông báo rõ ràng của bridge bị thay bằng lỗi hết giờ mơ hồ ở tầng trên |
| 2 | **Ghi vào sheet lớn là kích hoạt tính lại cả sổ** | `et.writeRange` trên `Risk!A1:Q1` lỗi Busy trong khi `et.fillRange` cùng lúc lại chạy được | Mỗi lần ghi một ô/khối là một lần LibreOffice tính lại toàn sổ. Ghi nhiều lần nhỏ đắt hơn hẳn ghi một lần lớn — chưa có gì gom các lần ghi trong một vòng lại |
| 3 | **Vẫn hết ngân sách trước khi xong** | `billable 444559 > 400000` sau 353 tool call; 6 sheet chưa soát được | 50.000 dòng là bài lớn nhất trong bộ đề. So với Claude Code: 618.800 token ra cho cả bài, nhưng đổi lại 60,7 triệu token đọc từ cache — nó trả giá bằng số vòng, không bằng số ô |
| 4 | **Chưa kiểm chứng được phần lớn thành quả** | `et.checkRange` trên 6/12 sheet trả lỗi hết giờ | Cùng gốc với #1: bước tự soát của agent cũng bị chặn bởi chính trần 60s đó |

Không còn là khoảng cách: **chi phí token theo từng ô**. `et.fillRange` chiếm 48% lời gọi và model tự
nói ra lợi ích — bảng 50.000 dòng giờ dựng được bằng công thức thay vì phát từng giá trị.

## Cải tiến đã làm để thu hẹp

1. **Nâng trần và xếp ba tầng thời gian cho đúng thứ tự** (commit `41e056b`) — khoảng cách #1 và #4:

   ```
   gate cua extension LibreOffice  300s   (axiom/gate.py, truoc la 60s)
       <  Core DefaultCommandTimeout  330s  (office/bridge.go, truoc la 90s)
       =  MCP live commandTimeout     330s  (mcpserver/livetools.go, khong doi)
   ```

   Bên gọi luôn phải chờ lâu hơn giới hạn của chính bridge, không thì thông báo rõ ràng của bridge bị
   thay bằng một lỗi hết giờ mơ hồ ở tầng trên. Con số 330 không mới: đường MCP live đã dùng nó cho
   cùng loại việc (ai.ask chạy tới 5 phút).

2. `et.fillRange` (commit `bd8b94e`) và ngân sách token chỉ tính phần phải trả (commit `f243789`) —
   làm từ Test 1, và chính là thứ cho phép lượt này dựng được 50.000 dòng.

3. **Thử tạm tắt tính lại tự động — ĐO ĐƯỢC LÀ CHẬM HƠN, đã hoàn tác** (commit `da6aaf3` rồi `66de08d`,
   `f2545de`). Giả thuyết: ghi từng ô mà để chế độ tính tự động thì LibreOffice tính lại cả sổ sau mỗi ô,
   nên gom lại thành một lần tính ở cuối sẽ nhanh hơn. Đo A/B thật, cùng máy, cùng khối ghi:

   | Khối ghi | Để nguyên | Tắt tính + `calculateAll()` |
   |---|---|---|
   | 24.008 ô | **12,4s** | 28,0s |
   | 48.008 ô | **32,6s** | 130,6s |

   Chậm hơn 2,3 và 4,0 lần. LibreOffice đã gộp việc tính lại sẵn; `calculateAll()` ở cuối kéo theo một
   lượt tính toàn bộ tài liệu, đắt hơn phần tính tăng dần mà nó thay thế. Đã hoàn tác, và ghi số đo ngay
   trong `write_range` để lần sau không ai thêm lại.

## Còn lại, chưa làm

- **Gom nhiều lần ghi trong một vòng** (khoảng cách #2). Hướng đúng là một lệnh ghi được NHIỀU vùng
  (`et.writeRanges {writes: [...]}`) — model hay gọi 5–10 `writeRange` liên tiếp trong cùng một phản hồi,
  mỗi lần là một lần tính lại cả sổ. Đã thử hướng rẻ hơn (đổi chế độ tính) và đo được là sai, xem mục 3
  trên. Chưa làm hướng gom lệnh.
- **Chưa chạy lại Test 2 sau khi nâng trần thời gian** để đo mức cải thiện thật — 27 lời gọi lỗi và 6
  sheet không soát được đều là hệ quả của trần cũ, nhưng đó là suy luận từ nguyên nhân, chưa phải số đo.
- **Chưa đối chiếu nội dung Dashboard và Checks với Claude Code** (số KPI, số check, PASS/FAIL) — hiện
  chỉ biết cả hai đều có mặt và có tiêu đề.
