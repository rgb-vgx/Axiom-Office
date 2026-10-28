# Báo cáo kiểm thử Windows: Office thật — lần 3 (kiểm 3 bản sửa của `7380bb5`)

Ngày test: **28/10/2026** · nhánh **`main`**, HEAD lúc bắt đầu `7380bb5` · HEAD mã nguồn khi đóng báo cáo:
`4c0999e` (2 commit sửa lỗi + 1 commit test) · phạm vi: bridge C# cho Microsoft Office + Agent Core Go,
_kiểm thử trên Microsoft Office thật trên Windows_ (không phải e2e giả lập).

Mục tiêu phiên: (1) build + cài `main` (C# của `7380bb5` chưa từng được biên dịch); (2) chạy 4 suite;
(3) kiểm tay 3 bản sửa của `7380bb5` (A: Track Changes + ai.ask không treo; B: PowerPoint đi theo bài người
dùng đang xem; C: replaceAll khi còn revision nhưng đã tắt theo dõi); (4) sửa lỗi tối thiểu, mỗi lỗi một commit.

---

## 1. Môi trường

| Thành phần | Giá trị |
|---|---|
| OS | Windows 10 Pro 10.0.19043, 64-bit |
| Office | Office Professional Plus 2019 (ProPlus2019Volume, Click-to-Run) **16.0.10373.20050, x64** — Word / Excel / PowerPoint |
| Tài khoản Office | tài khoản Windows cục bộ, `UserName = "Administrator"`, `UseLocalUserInfo = False` (không đăng nhập M365) |
| Go | go1.27.3 windows/amd64 |
| Python | 3.15.1 (không có pywin32 — COM ngoài bridge đi qua Windows PowerShell 5.1) |
| gcc | **không có** → **chưa chạy `go test -race`** |
| Build tool | `scripts\build.ps1 -Kill` (Roslyn 4.11.0 qua `AXIOM_CSC`), cài bằng `scripts\install.ps1` |
| Bridge (Office) | cổng **47831** (Word), **47832** (Excel), **47833** (PowerPoint); Agent Core **47840** |
| LLM | `ocg/beam-free` (model yếu đã cấu hình, provider openai-compatible) |
| Log bridge | `%LOCALAPPDATA%\AxiomOffice\bridge.log` |

Ghi chú: file thử (A.pptx, B.pptx, script kiểm tay) nằm trong thư mục tạm của phiên, ghi là `<tmp>` trong báo cáo.

---

## 2. Build

- `scripts\build.ps1 -Kill` trên `7380bb5`: **biên dịch thành công, không có lỗi** (Core, add-in, host).
  Không cần sửa lỗi biên dịch.
- `scripts\install.ps1`: `Installed Axiom Office.` (ProgID `AxiomOffice.Connect`, token giữ nguyên).

---

## 3. Bảng test (số nguyên văn)

| # | Suite | Trên `7380bb5` (trước sửa) | Sau sửa (`4c0999e`) |
|---|---|---|---|
| 1 | `python -m unittest discover -s tests\catalog -v` | `Ran 3 tests` — `OK` | `Ran 3 tests` — `OK` (chạy lại trước mỗi commit) |
| 2 | `cd core-go; go test ./...` | 24 gói `ok`, 4 gói `[no test files]`, 0 FAIL | không đổi mã Go → không chạy lại |
| 2b | `go test -race ./...` | **chưa chạy** (không có gcc; không tự cài) | chưa chạy |
| 3 | `python tests\live\test_live_commands.py --ai` | **`376 passed, 0 failed`** (2 phút 33 giây) | **`386 passed, 0 failed`** (thêm kiểm hồi quy mục 5) |
| 4 | `python tests\live\test_office_structure.py` | **`17 passed, 0 failed`** | **`17 passed, 0 failed`** |
| B6 | `python tests\live\test_live_commands.py --apps ppt --ai` | **`100 passed, 0 failed`** | — |

Không có test flake trong phiên (không phải chạy lại để xác nhận).

---

## 4. Kiểm tay 3 bản sửa của `7380bb5`

Bằng chứng là JSON thô của bridge/Core (`>>> lệnh` / `<<< HTTP ...`). Thao tác "người dùng" làm ngoài bridge:
gõ phím bằng `WScript.Shell.SendKeys` vào cửa sổ Word (sau `SetForegroundWindow`); chuyển bài PowerPoint bằng
`SetForegroundWindow` + click chuột thật (`mouse_event`) vào cửa sổ B; đọc `UserName` bằng COM
`GetActiveObject('Word.Application')` từ process khác.

### A. Track Changes còn sót + `ai.ask` không treo — ✅ ĐẠT

**A1.** `writer.setTracking {on:true, author:"Axiom AI"}`, không `restoreAuthor`:
```
<<< {"ok": true, "result": {"tracking": true, "previous": false, "author": "Axiom AI"}}
UserName trước: Administrator|A|False   sau setTracking: Administrator|A|True
```
(`UseLocalUserInfo` bật; `UserName` chưa đổi — chỉ đổi quanh từng lệnh `writer.*`.)

**A2.** `ai.ask "Thêm câu 'Kết thúc.' vào cuối văn bản"` — xong bình thường, không chờ tới timeout:

| Lần | Thời gian (đo ngoài) | `seconds` / `rounds` | Lệnh |
|---|---|---|---|
| 1 | 9,5 s | 8,56 / 4 | outline, appendText, outline |
| 2 | 8,1 s | 6,83 / 4 | appendText, getText |
| 3 | 21,1 s | 20,28 / 8 | appendText, getText, outline, replaceAll, outline, getText |

bridge.log trong lượt chỉ có các dòng `ai.ask (core) tool.finished: ...` cách nhau 1–7 s (thời gian gọi LLM);
không có lỗi, không có `COM call rejected`, không có lệnh nào chờ khoá. (Bridge không log thời gian từng lệnh;
khoảng cách giữa các `tool.finished` là bằng chứng gián tiếp.)

**A3.** Gõ `Y` bằng bàn phím trong lúc `ai.ask` chạy (lần 3: gõ ở t = 4,9 s, `ai.ask` còn chạy: `True`):
```
{"id": "1:1", "type": "insert", "author": "Axiom AI", "text": "Kết thúc."}
{"id": "1:3", "type": "delete", "author": "Axiom AI", "text": "Y"}
{"id": "1:4", "type": "insert", "author": "Administrator", "text": "Y"}
"authors": {"Axiom AI": 3, "Administrator": 1}
```
→ revision `Y` mang tên người dùng (`Administrator`), không phải `Axiom AI`.

Ghi chú trung thực:
- Lần 1 và 2, `Y` **không vào tài liệu nào** (đã đọc text cả Document1/Document2 qua COM). Nguyên nhân ở harness:
  mẹo bấm-nhả Alt để giành foreground bật KeyTips của ribbon khi Word đã ở foreground, nên `Y` bị ribbon nuốt.
  Lần 3 gửi `{ESC}` trước phím → `Y` vào đúng. Không phải lỗi sản phẩm.
- Lần 3, model (`ocg/beam-free`) thấy `Kết thúc.Y` và **tự xoá chữ `Y` của người dùng** bằng `replaceAll`
  (thành revision delete của `Axiom AI`, nên "Hoàn tác lượt này" sẽ trả lại). Đây là hành vi model, không phải
  bridge; nhưng chính lượt này làm lộ lỗi 5.1 (outline vẫn thấy `Kết thúc.Y` sau khi xoá).

**A4.** Dọn: `writer.setTracking {on:false, restoreAuthor:true}` →
`{"tracking": false, "previous": true, "author": "Administrator"}`; COM ngoài bridge: `Administrator|A|False`
— trùng giá trị trước A1 (kể cả `UseLocalUserInfo`).

**A5. Không hồi quy** — lượt Core `POST /v1/runs` với `options.trackChanges=true` (sửa 2 chỗ), gõ ` X` ở t = 4 s
khi run `status=running`:
```
run r_c72e… status=completed sau 36.8s  {"tracking": true, "undoMode": "revisions", "rounds": 4, "verified": true}
revisions: 4 × Axiom AI (insert/delete "đã sửa một"/"gốc một", "đã sửa hai"/"gốc hai") + insert "X" (Administrator)
getText: "Đoạn đã sửa một. Đoạn đã sửa hai. X"
POST /v1/runs/{id}/undo → {"mode": "revisions", "result": {"action": "reject", "count": 4}}
còn lại: [{"type": "insert", "author": "Administrator", "text": "X"}]
getText: "Đoạn gốc một. Đoạn gốc hai. X"
UserName cuối: Administrator|A|False
```
→ undo chỉ từ chối 4 revision của AI, `X` còn.

### B. PowerPoint đi theo bài người dùng đang xem — ✅ ĐẠT (B1–B6)

A.pptx 2 slide, B.pptx 4 slide, cùng mở trong một PowerPoint.

**B2.** `wpp.open {path: A}` (`alreadyOpen: true`) →
`app.info.activePresentation = {"name": "A.pptx", "slides": 2}`, `wpp.listSlides = {"presentation": "A.pptx", "slideCount": 2}`.

**B3.** Bấm sang cửa sổ B (ngoài bridge): `hwnd=4130086 click=1251,882 COM ActivePresentation=B.pptx`.

**B4.** `app.info.activePresentation = {"name": "B.pptx", "slides": 4}`;
`wpp.listSlides = {"presentation": "B.pptx", "slideCount": 4}`. bridge.log:
```
13:01:57.372 [INFO] PowerPoint: active presentation changed to '<tmp>\b\B.pptx' - no longer targeting '<tmp>\b\A.pptx'
```

**B5.** `ai.ask "bài này có bao nhiêu slide?"` (cùng đường `CoreClient.Run` với pane; không bấm được pane bằng
tay trong phiên tự động) → 3,8 s, `wpp.listSlides` trả `B.pptx`, trả lời: `"Bài này có **4 slide**."`

**B6.** `test_live_commands.py --apps ppt --ai`: **`100 passed, 0 failed`**.

Ghi chú: bước phụ ngoài yêu cầu (click quay lại cửa sổ A) **không chuyển được** — COM vẫn báo
`ActivePresentation=B.pptx`, nên bridge đúng khi vẫn trả B. Nhiều khả năng `SetForegroundWindow` bị Windows
chặn (foreground lock) và click rơi vào cửa sổ B đang đè lên; **chưa xác nhận** được bằng thao tác tay thật.

### C. `replaceAll` khi còn revision nhưng đã tắt theo dõi — ✅ ĐẠT

**C1–C2.** `"Đoạn gốc một. Đoạn gốc hai."` → bật theo dõi → `replaceAll gốc một → đã sửa một` → tắt theo dõi,
không chấp nhận: `chars=38 revisions=2 track=False`.

**C3.** `writer.replaceAll {find:"gốc một", replace:"khác"}`:
```
<<< {"ok": true, "result": {"replaced": false, "count": 0}}
stats: chars=38 end=38 revisions=2      (không phình: trước và sau đều 38)
writer.getText: "Đoạn đã sửa một. Đoạn gốc hai.\r"
```

**C4. Thời gian trên tài liệu dài** — 1.100 đoạn, 123.194 ký tự, **79 trang** (dài hơn ~50 trang yêu cầu):

| Tình huống | Revisions | `replaceAll` |
|---|---|---|
| không khớp, chưa có revision | 0 | 0,35 s |
| khớp 1 chỗ, chưa có revision | 0 | 0,38 s |
| không khớp, đã tắt theo dõi | 74 (37 lần thay) | 0,39 s |
| tìm chữ đã bị xoá (revision delete) | 74 | 0,36 s — `count: 0` |
| khớp 1 chỗ | 74 | 0,44 s |
| `writer.getText {maxChars:200}` | 74 | 0,42 s |

→ đếm `Revisions.Count` + ẩn revision không làm chậm đáng kể (chênh < 0,1 s).

---

## 5. Lỗi phát hiện & bản vá

### 5.1. `writer.outline` trả cả chữ đã xoá khi còn revision — `a9360a2`

**Triệu chứng:** sau `replaceAll` có theo dõi, `writer.outline` trả
`"Đoạn đã sửa mộtgốc một. Đoạn gốc hai."` và vẫn liệt kê đoạn đã xoá `"Đoạn sẽ bị xoá."`, trong khi
`writer.getText` trả `"Đoạn đã sửa một. Đoạn gốc hai.\r\rĐoạn cuối.\r"`. Trong lượt A3, model xoá `Y` bằng
`replaceAll` (`count: 1`) nhưng outline ngay sau đó vẫn trả `Kết thúc.Y` — agent tưởng lệnh chưa ăn.

**Nguyên nhân:** outline đọc `Range.Text` khi `ShowRevisions` đang bật; chữ bị xoá có theo dõi vẫn nằm trong
`Range.Text`. `7380bb5` đã đồng bộ điều kiện ẩn revision cho `getText` và `replaceAll` nhưng bỏ sót outline.

**Sửa:** ẩn revision (cùng điều kiện `TrackRevisions || Revisions.Count > 0`) trong lúc duyệt đoạn, trả lại
`ShowRevisions` trong `finally`.

**Kiểm chứng:** cùng script tái hiện sau build:
`{"paragraphs": 3, "items": [{"n": 1, "t": "Đoạn đã sửa một. Đoạn gốc hai."}, {"n": 3, "t": "Đoạn cuối."}]}` —
số đoạn `n` không đổi (`writer.goto {paragraph:3}` vẫn trả `"Đoạn cuối."`); đoạn bị xoá hết bị bỏ như đoạn rỗng.
Thêm kiểm `Word outline ẩn chữ đã xoá khi còn revision` vào `test_live_commands.py` (`4c0999e`) — PASS.

### 5.2. Số đoạn của comment/revision lệch 1 khi neo ở đầu đoạn — `d8b22e6`

**Triệu chứng** (không cần revision): tài liệu 3 đoạn,
`writer.addComment {paragraph: 3}` → `{"comment": 1, "paragraph": 2}`; `{paragraph: 2}` → `"paragraph": 1`;
`writer.listComments` cũng báo `"anchor": "Ba có chữ neo.", "paragraph": 2`. Neo giữa đoạn (`find: "chữ neo"`)
thì đúng `3`, nên test cũ (chỉ kiểm đoạn 1) không bắt được.

**Nguyên nhân:** `ParagraphNumberAt` đếm `doc.Range(0, start).Paragraphs.Count`; khi `start` là đầu đoạn k,
range dừng ngay trước đoạn k nên chỉ đếm k−1.

**Sửa:** lấy range tới `start + 1` (chặn bởi `Content.End`) để range chạm vào đoạn k.

**Kiểm chứng:** sau build: `addComment {paragraph:3}` → `paragraph: 3`; `{paragraph:2}` → `2`; `{paragraph:1}` → `1`;
`find` giữa đoạn 3 → `3`; `listRevisions` cho revision ở đầu đoạn 2/3 → `paragraph: 2`/`3`.
Thêm kiểm `Word addComment theo đoạn trả đúng số đoạn (đầu đoạn 2)` (`4c0999e`) — PASS.

Sau cả hai bản vá: catalog `OK`, `test_live_commands.py --ai` **`386 passed, 0 failed`**,
`test_office_structure.py` **`17 passed, 0 failed`**.

---

## 6. Commit trong phiên

| Commit | Nội dung |
|---|---|
| `a9360a2` | `fix(bridge): writer.outline tra ca chu da xoa khi con revision` |
| `d8b22e6` | `fix(bridge): so doan cua comment/revision lech 1 khi neo o dau doan` |
| `4c0999e` | `test(live): kiem so doan cua addComment o dau doan 2 va outline an chu da xoa` |

Đã `git pull --rebase origin main` (không có thay đổi mới) và push `7380bb5..4c0999e` lên `main`, không force-push.

---

## 7. Việc còn lại

1. **`go test -race` chưa chạy** — máy không có gcc (không tự cài theo yêu cầu).
2. **Model xoá chữ người dùng gõ giữa lượt** (A3): với Track Changes thì "Hoàn tác lượt này" cứu được, nhưng
   không có Track Changes thì chữ người dùng mất. Cân nhắc báo cho agent biết đoạn nào người dùng vừa sửa
   (hoặc chỉ dẫn: không "dọn" chữ lạ mà người dùng có thể vừa gõ).
3. Các lệnh đọc khác của Word chưa soát điều kiện ẩn revision như `getText`/`outline`/`replaceAll`
   (`writer.selection`, chữ trả về của `writer.goto`, `writer.checkLayout`, `RangeText` của `listComments` anchor).
   Nên gom thành một helper dùng chung thay vì lặp mẫu `ShowRevisions` ở mỗi lệnh.
4. B: chưa xác nhận bằng thao tác tay thật việc người dùng bấm **quay lại** A sau khi bridge đã bỏ theo dõi A
   (harness tự động không giành được foreground lần hai).
5. B5 kiểm qua `ai.ask` (cùng đường `CoreClient.Run`), chưa bấm câu hỏi trong pane PowerPoint bằng tay.
6. Máy này là Office 2019 Volume, tài khoản cục bộ: chưa kiểm đường `UseLocalUserInfo` với tài khoản M365 đăng
   nhập trong phiên này (lần 2 đã kiểm trên Microsoft 365).
7. WPS không kiểm trong phiên này.
