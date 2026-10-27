# Báo cáo kiểm thử Windows: Office thật, 4 lỗi đã vá, model mới

Ngày test: **28/10/2026** · nhánh **`windows`** (fork từ `claude/inspiring-davinci-1fxpeg` @ `332f2d2`, HEAD khi
đóng báo cáo: `cdf59da`, đã push lên `origin/windows`) · phạm vi: bridge C# cho Microsoft Office + Agent Core Go,
_kiểm thử trên Office thật trên máy Windows_ (không phải e2e giả lập).

Mục tiêu của phiên: (1) build + cài add-in vào Office, chạy toàn bộ test catalog/Go/live; (2) kiểm tay các lệnh mới
(`goto`, comment, redline, sort/filter) không có trong harness; (3) vá các lỗi phát hiện được lên nhánh riêng và
push; (4) kiểm chứng lại với LLM mới do người dùng cấu hình.

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

LLM:
- Đầu phiên: `oc/beam-free` @ `http://localhost:20128/v1` — `llm-test` OK 3.5 s, nhưng hết hạn mức free giữa phiên
  (HTTP 429 `FreeUsageLimitError`).
- Cuối phiên (người dùng cấu hình mới): `ocg/beam-free` @ `https://ai.hnspl.vn/v1`, key lưu dạng `dpapi:` (add-in tự
  giải mã). Báo cáo **không** ghi key.

---

## 2. Build & cài đặt

- `scripts\build.ps1 -Kill`: **không lỗi C#**. Build lại 3 lần trong phiên (sau từng bản vá) đều sạch.
- `scripts\install.ps1`: cài thành công, add-in nạp đúng trên cả 3 app.
- Quy trình trước mỗi lần build: kill `WINWORD/EXCEL/POWERPNT/et/wpp/wps` (WPS có tự respawn — phải kill nhiều vòng,
  cuối phiên xác nhận sạch).

---

## 3. Kết quả test tổng hợp

| Suite | Kết quả | Ghi chú |
|---|---|---|
| `python -m unittest discover -s tests/catalog -v` | **3/3 OK** | chạy lại sau mỗi lần build lại |
| `cd core-go; go test ./...` | **tất cả OK** | không đổi trong phiên |
| `python tests\live\test_live_commands.py` (không `--ai`, trước vá) | **294 passed / 1 failed** | fail duy nhất: `saveAs` (đã vá) |
| `python tests\live\test_live_commands.py --ai` (trước vá SSE) | **311 passed / 10 failed** | 9 fail do LLM 429, 1 coverage gate |
| `python tests\live\test_live_commands.py --ai` (**sau vá SSE, model mới**) | **317 passed / 4 failed** | xem mục 4 |
| `python tests\live\test_live_commands.py --apps ppt --ai` (chạy lại PPT) | **98 passed / 0 failed** | xác nhận flake ở mục 5 |
| `python tests\live\test_office_structure.py` | **17/17 OK** | |

### 3.1. FAIL nguyên văn của run `--ai` cuối (317/4)

1. `FAIL PowerPoint wpp.open bài đã mở đưa nó lên làm bài đang hoạt động`
   `{'open': {'name': 'wpp.pptx', 'fullName': 'C:\\Users\\PhuongLy\\AppData\\Local\\Temp\\axiom-live-test\\wpp.pptx', 'alreadyOpen': True}, 'active': 'from-template.pptx'}`
2. `FAIL office-ppt tài liệu vẫn chưa lưu sau ai.ask`
   `{'name': 'wpp.pptx', 'fullName': '...\\wpp.pptx', 'saved': True, 'slides': 2}`
3. `FAIL PowerPoint ai.ask thêm slide 'Kết luận'`
   `{'presentation': 'wpp.pptx', 'slideCount': 2, 'slides': [...]}` (nội dung cũ, không có slide mới)
4. `FAIL mọi lệnh trong registry đều có test`
   `['et.filter', 'et.goto', 'et.sort', 'wpp.goto', 'writer.addComment', 'writer.editCommentScope', 'writer.goto', 'writer.listComments', 'writer.listRevisions', 'writer.replyComment', 'writer.resolveComment', 'writer.reviewRevisions']`

Diễn giải:
- **Fail 1 là flake timing** của `wpp.open` khi bài đã mở: reply `alreadyOpen: True` nhưng PowerPoint không chuyển cửa
  sổ hoạt động. Chạy riêng PPT (98/98 PASS) thì chính test này **PASS**, kể cả `ai.ask` 6 vòng thêm đúng slide
  "Kết luận". Không phải lỗi của bản vá nào trong phiên.
- **Fail 2–3 là hệ quả của fail 1**: test tự in `LƯU Ý: sau ai.ask cửa sổ hoạt động là ...from-template.pptx, không
  phải bài đích` — agent đã làm việc đúng nhưng trên bài đang active sai.
- **Fail 4 là coverage gate của script**, không phải lỗi sản phẩm: 12 lệnh mới chưa có test trong harness. Cả 12 lệnh
  đã được kiểm tay trên máy thật (mục 7).

### 3.2. Run `--ai` trước khi vá SSE (để đối chiếu)

**311 passed / 10 failed** — 9 fail đều in lỗi HTTP 429 từ endpoint
(`[429]: {"type":"error","error":{"type":"FreeUsageLimitError","message":"Free usage limit reached..."}}`), gồm:
3× `ai.ask trả lời xong`, 3× `tài liệu vẫn chưa lưu sau ai.ask` (doc mở lại sạch → `saved:True`), 3× kiểm nội dung
do ai.ask ghi (dòng mới của Word, `Tổng = 17.5` của Excel, slide "Kết luận" của PPT). Toàn bộ là hệ quả hết hạn mức
LLM, không phải lỗi add-in; sau khi đổi model, tất cả đều PASS.

---

## 4. Lỗi phát hiện & bản vá (4 commit trên `windows`)

### 4.1. `saveAs`/`save` — `a82788a`

**Triệu chứng:** `writer.saveAs` nhận đường dẫn `.docx` nhưng Word 16.0.20527 ném
`Incompatible file type and file extension` — `SaveAs2(path)` không tự suy ra định dạng. Người dùng còn yêu cầu
mặc định lưu về `Documents\Axiom\`.

**Vá** (`CommandDispatcher.Params.cs`): `SaveFormatFor()` map đuôi → `WdSaveFormat`/`XlFileFormat`/`PpSaveAsFileType`
(docx=16, xlsx=51, pptx=24, …); đường dẫn tương đối quy về `Documents\Axiom\` (tạo thư mục nếu chưa có);
`Save()` trên tài liệu chưa từng lưu thì tự lưu vào `Documents\Axiom`; tắt `DisplayAlerts` trong lúc lưu để không bật
hộp thoại.

**Kiểm chứng:** live suite `writer.saveAs` + `writer.open` đọc lại nội dung OK; `et.saveAs`, `wpp.saveAs` OK.

### 4.2. Bão `RPC_E_CALL_REJECTED` khi Word mới mở — `3edc98e`

**Triệu chứng:** run đầu tiên fail dây chuyền 37 lệnh với `RPC_E_CALL_REJECTED`; replay thì 37/37 OK. Nguyên nhân:
add-in COM bên thứ ba (WPS PDF, Nitro PDF — thấy trên ribbon) giữ Word busy lúc khởi động, bridge gọi COM quá sớm.

**Vá** (`CommandDispatcher.cs`): `MaxComRetries` 10 → **30** (~25 giây cửa sổ chờ).

**Kiểm chứng:** các run sau không còn bão lỗi; log bridge chỉ còn retry lẻ tẻ.

### 4.3. `editCommentScope` xoá mất comment — `0ba64eb`

**Triệu chứng:** `scope.Text = …` (và cả Delete+Insert, `Selection.TypeText`, Find/Replace) làm Word 16.0.20527 **xoá
comment** khi thay toàn bộ chữ neo; lỗi nổi ở lệnh sau dưới dạng `COMException: Object has been deleted`. Microsoft
xác nhận hành vi: comment chỉ sống nếu còn **ít nhất 1 ký tự neo gốc**; `Comment.SetRange` không tồn tại trong Word OM
(đã thử và xác nhận); Find/Replace giữ comment nhưng bóp neo còn ký tự dôi.

**Vá** (`CommandDispatcher.WriterReview.cs`): chụp metadata luồng (text gốc, replies, cờ `Done`) →
`Comment.DeleteRecursively()` (Word 2013+; chính thông báo COM của Word chỉ ra API này) → thay chữ trên `Range` đã nắm
`[start..end]` → `Comments.Add` neo lại trên chữ mới → `Replies.Add` từng trả lời → khôi phục `Done`. Trả về id của
comment gốc mới (chỉ số có thể đổi sau khi thêm lại).

**Kiểm chứng:** A/B test (có/không reply) đều `ok:true`, comment còn neo đúng chữ mới; kiểm tra COM **ngoài tiến
trình** xác nhận `Comments.Count=2` với root + reply nguyên vẹn (`ancestor=reply` giữ đúng).

### 4.4. LLM proxy mặc định trả SSE, `LlmClient` C# hỏng parse — `cdf59da`

**Triệu chứng:** `AxiomOffice.Host.exe llm-test` báo `ERROR: Could not find text in provider response` với endpoint
mới `https://ai.hnspl.vn/v1`. Dò trực tiếp: request có `max_tokens` lớn bị proxy trả **SSE** (`data: {...}\n\ndata:
[DONE]`) → `Invalid JSON primitive: ata: [DONE]`; đôi lúc trả JSON thường → `content:null` vì hết token cho
`reasoning_content`.

**Nguyên nhân gốc:** proxy OpenAI-compatible (kiểu 9router/OpenCode) **mặc định stream** nếu client không gửi
`stream:false`; nhánh Core Go đã có `ParseStream` xử lý SSE đúng (kể cả `reasoning_content`, tool-call ghép theo
`index`), còn `LlmClient` C# (đường `llm-test` + nút test trong Settings) đọc cả body và parse JSON một lần.

**Vá** (`src/AxiomOffice/Ai/LlmClient.cs`): gửi tường minh `"stream": false` trong `CallOpenAi`. Không đổi Core.

**Kiểm chứng:** `llm-test` → `OK (3.4s): OK`; full suite sau vá lên **317 passed**, cả 3 app đều `ai.ask` PASS (Word 4
vòng, Excel 5 vòng, PPT 5 vòng; chạy lại PPT riêng 6 vòng).

> Ghi chú: `ocg/beam-free` là **reasoning model** (`reasoning_content`). Core đã tách reasoning khỏi `Raw` khi gửi lại
> lịch sử (tránh 400 của DeepSeek và tốn token); riêng `LlmClient` chỉ dùng cho test kết nối nên chỉ cần ép
> non-streaming.

---

## 5. Flake ghi nhận (không vá)

`wpp.open` với bài đã mở: trong một full run, reply trả `alreadyOpen: True` nhưng `ActivePresentation` không đổi →
`ai.ask` kế tiếp thao tác trên `from-template.pptx`. Chạy riêng PPT: **98/98 PASS**, gồm đúng test này và `ai.ask`
thêm slide "Kết luận". Kết luận: flake timing khi PowerPoint vừa trải qua nhiều thao tác liên tiếp, không phải lỗi
logic. Nếu tái diễn thường xuyên, hướng xử lý: sau khi activate bài đã mở, chờ/kiểm `ActivePresentation.FullName`
bằng vòng lặp ngắn (tương tự cách bridge đã xử lý các thao tác khác).

---

## 6. Một bẫy khi test (để người sau không mất thời gian)

Khi in kết quả bridge bằng PowerShell 5.1, `ConvertTo-Json` **mặc định Depth=2** sẽ bóp mảng lồng sâu thành chuỗi
`""`/`"System.Object[]"`. Lần đầu thấy `listComments` trả `"replies": ""` tưởng là bug, nhưng in **raw** `$r.Content`
thì JSON đúng: `replies: []` khi rỗng, `[{id:2,...}]` khi có trả lời. Quy ước từ nay: đối chứng bridge bằng raw
content hoặc `ConvertTo-Json -Depth 8`.

Kèm theo: PowerShell 5.1 đọc file `.ps1` không BOM theo ANSI → script test phải **ASCII thuần** (hoặc lưu UTF-8 BOM).
`& WINWORD.EXE ... | Out-Null` sẽ **block** tới khi Word thoát → dùng `Start-Process`.

---

## 7. Kiểm tay 12 lệnh ngoài harness

Mỗi lệnh chạy trên tài liệu mới, gọi qua bridge thật; đối chứng bằng reply + đọc lại trạng thái.

| Lệnh | Kết quả | Bằng chứng |
|---|---|---|
| `writer.goto {paragraph:3}` | ✅ | reply `{paragraph:3, text:"Doan ba (muc tieu goto)."}`; `writer.selection` xác nhận đoạn 3 được chọn; ảnh `m1_word_goto.png` |
| `et.goto {range:"Sheet1!C5"}` | ✅ | reply `{sheet:"Sheet1", range:"C5"}` |
| `wpp.goto {slide:2}` | ✅ | reply `{slide:2}` |
| `et.sort` (`keys` 1 cột) | ✅ | sau sort đọc lại `2, 9, 20` |
| `et.filter` (`criteria: ">9"`) | ✅ | `visibleRows: 1` |
| `et.filter` (`clear:true`) | ✅ | `{cleared:true}` |
| `writer.addComment` | ✅ | `{comment:1}` neo đúng đoạn |
| `writer.replyComment` | ✅ | `{replied:true}`; `listComments` thấy reply trong mảng `replies` |
| `writer.editCommentScope` | ✅ (sau vá) | A/B: comment còn sống, neo đúng chữ mới; out-of-proc: root + reply nguyên vẹn |
| `writer.listComments` | ✅ | raw JSON đúng; `includeDone` lọc đúng; `open`/`total` đúng |
| `writer.resolveComment` | ✅ | `done:true` ẩn khỏi list mặc định, hiện lại với `includeDone`; `done:false` mở lại đúng |
| `writer.listRevisions` / `writer.reviewRevisions` | ✅ | trả duyên dáng khi chưa bật track: `{revisions:[], tracking:false}` / `{count:0}` |

Ảnh chụp thủ công (Word goto) nằm ở `%LOCALAPPDATA%\Temp\opencode\shots\` — **không** commit vào repo.

---

## 8. Commit trên `windows`

| Commit | Nội dung |
|---|---|
| `a82788a` | `fix(bridge): saveAs dat dinh dang luu dung theo duoi file, duong dan tuong doi dua ve Documents\Axiom` |
| `3edc98e` | `fix(bridge): nang so lan thu lai COM khi oj ban dau tu 10 len 30` |
| `0ba64eb` | `fix(bridge): editCommentScope xoa luong roi them lai de giu comment khi thay toan bo chu neo` |
| `cdf59da` | `fix(ai): gui stream=false khi goi LLM tu C# - proxy OpenAI-compatible mac dinh tra SSE nen parse JSON bi hong` |

Base: `332f2d2` (nhánh `claude/inspiring-davinci-1fxpeg`). Đã push `origin/windows`; PR có thể mở tại
`https://github.com/rgb-vgx/Axiom-Office/pull/new/windows`.

---

## 9. Việc còn lại / khuyến nghị

1. **Bổ sung test harness cho 12 lệnh mới** để gỡ coverage gate (mục 3.1 fail 4).
2. **WPS chưa test trong phiên này** (cổng 47821–47823); add-in dùng chung nhưng nên chạy một lượt `--wps` trước khi
   phát hành.
3. **Theo dõi flake `wpp.open`** (mục 5) — nếu tái diễn, thêm chờ `ActivePresentation` sau khi activate.
4. `llm-test` hiện không gửi `max_tokens`; với model reasoning, nếu proxy siết mặc định thì có thể lại `content:null`.
   Cân nhắc đặt trần nhỏ (vd 512) riêng cho test kết nối — không ảnh hưởng đường Core.

---

## 10. Lệnh tái lập

```powershell
# Build + cài (kill Office trước)
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
