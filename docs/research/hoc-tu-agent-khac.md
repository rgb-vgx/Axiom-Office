# Học gì từ Claude Code, Codex, Grok Build, Hermes Agent

Ngày: **27/10/2026**. Mục tiêu: tìm cơ chế đã được kiểm chứng ở các agent khác mà Axiom có thể áp dụng cho đúng
những lỗ hổng trong `docs/audits/axiom-agent-runtime-deep-audit.md` (kết quả mơ hồ, lượt bị gián đoạn, hoàn tác đúng
phạm vi, người dùng sửa xen giữa, kiểm chứng). Không nhằm sao chép kiến trúc.

## Nguồn đã đọc

| Dự án | Nguồn | Phiên bản | Ghi chú |
|---|---|---|---|
| Codex | `openai/codex` (Rust, `codex-rs/`) | `6574bcd`, 27/10/2026 | Mã nguồn đầy đủ |
| Hermes Agent | `NousResearch/hermes-agent` (Python) | `e5ea5fd9`, 27/10/2026 | Mã nguồn đầy đủ |
| Grok Build | `xai-org/grok-build` (Rust, đồng bộ từ monorepo xAI) | `2bdd1d6`, 29/09/2026 | Mã nguồn CLI + runtime |
| Claude Code | `anthropics/claude-code` | `08c6a4f`, 26/10/2026 | **Không có mã nguồn agent**: repo chỉ có plugin, ví dụ hook, `CHANGELOG.md`. Các điểm về Claude Code dưới đây lấy từ changelog và tài liệu, không phải từ code |

Chỉ đọc các phần liên quan đến runtime (ngắt lượt, lưu phiên, checkpoint, phê duyệt, guard, kiểm chứng). Không build
hay chạy dự án nào.

## Tóm tắt

Cả bốn agent đều chạy một vòng lặp model ↔ tool giống Axiom, và không agent nào dùng workflow engine dạng đồ thị.
Điều làm chúng bền hơn nằm ở bốn thói quen mà Axiom chưa có hoặc mới có một phần:

1. **Lượt chưa kết thúc là một trạng thái có tên, được ghi bền và báo cho model** (Codex, Grok Build).
2. **Phân biệt "chưa chạy" với "có thể đã chạy một phần"** khi hủy hoặc lỗi (Codex).
3. **Khôi phục không phụ thuộc ngăn undo của ứng dụng**: tự chụp checkpoint trước lần ghi đầu tiên của lượt (Hermes,
   Claude Code `/rewind`), và gắn mỗi thay đổi với tác giả để duyệt hoặc từ chối riêng (Grok Build hunk tracker).
4. **Không ghi đè thứ mình chưa thấy hoặc đã đổi kể từ lần đọc** (Hermes stale-write guard).

Axiom đã ngang hoặc hơn ở: guard chặn lặp sinh từ sự cố thật, khoá vùng bôi đen, hỏi trước khi ghi đè ô Excel, giới
hạn undo theo lượt, và (từ commit `d9a3daa`) chặn gửi lại lệnh ghi có kết quả không rõ.

## 1. Lượt bị gián đoạn là trạng thái có tên

**Codex.** Khi người dùng ngắt, Codex chèn vào lịch sử hội thoại một mục cố định: *"The user interrupted the previous
turn on purpose... If any tools/commands were aborted, they may have partially executed."*
(`codex-rs/core/src/context/turn_aborted.rs:12-13`). Mục này được **flush xuống rollout trước khi** phát sự kiện
`TurnAborted`, vì client có thể đọc lại rollout ngay khi nhận sự kiện (`core/src/tasks/mod.rs:1015-1033`). Khi khôi
phục hoặc fork một phiên mà sau tin nhắn người dùng cuối cùng không có `TurnComplete`/`TurnAborted`, Codex coi là đang
dở và nối thêm đúng ranh giới "interrupted" đó (`core/src/thread_manager.rs:2755-2850`).

**Grok Build.** Khi nạp phiên, `interrupted_turn.rs` quét đuôi `events.jsonl`; một `turn_started` không có `turn_ended`
được đóng bằng `stop_reason = "interrupted"`, khác hẳn `error` và `cancelled`, kèm câu nhắc model *"The previous turn
was interrupted... Work before this message may have been lost."*
(`crates/codegen/xai-grok-shell/src/session/interrupted_turn.rs:8-30`). Sự kiện kết thúc lượt cũng được lưu để client
kết nối lại giữa lượt vẫn thấy lượt kết thúc từ bản phát lại (`session/turn_completion.rs:1-5`).

**Axiom hiện có.** Commit `d9a3daa` đã đổi dòng `runs` còn `running` thành `interrupted` khi khởi động. Nhưng lượt
sau trong cùng hội thoại **không biết** lượt trước bị cắt ngang. Khi hủy, hội thoại chỉ có `tool_summary` của các lệnh
đã xong, model không được báo lệnh cuối có thể đã chạy dở.

**Đề xuất (nhỏ, P0).**
- Khi lượt kết thúc `cancelled`/`timedout` hoặc bị đánh dấu `interrupted` lúc khởi động: thêm một tin nhắn vào
  `messages` của hội thoại, kiểu: "Lượt trước bị dừng giữa chừng; lệnh `writer.appendText` cuối cùng có thể đã chạy một
  phần — đọc lại tài liệu trước khi sửa tiếp." `ContextAssembler` đã đưa tin nhắn trước đó vào lượt sau, nên không cần cơ
  chế mới.
- Cần thêm `conversation_id` vào truy vấn `MarkInterrupted` (cột đã có trong bảng `runs`).

## 2. "Hủy trước khi chạy" khác "hủy khi đang chạy"

**Codex.** Kết quả của tool bị hủy có hai dạng: `"aborted before dispatch"` khi chưa bắt đầu, và `"aborted by user
after {secs}s"` khi đã chạy được một lúc (`core/src/tools/parallel.rs:459-478`). Kết quả này vẫn được ghi vào lịch sử
như một output của đúng `call_id`, nên lịch sử không bao giờ có tool call thiếu kết quả.

**Axiom hiện có.** Commit `d9a3daa` xử lý trường hợp bridge báo lỗi khi lệnh có thể đã chạy (`tools/uncertain.go`).
Còn trường hợp **hủy lượt khi lệnh đang ở trong Office**: `BridgeClient.send` trả `"cancelled"`, vòng lặp dừng, và lệnh
đó không được đánh dấu gì (`office/bridge.go:221-224`, `model/agent.go:316`).

**Đề xuất (nhỏ, P0).** Khi `ctx` bị hủy trong lúc `/cmd` của một lệnh ghi đang chờ, coi kết quả là "không rõ" như
`uncertain.go`, và đưa tên lệnh đó vào tin nhắn "lượt bị dừng" ở mục 1.

## 3. Khôi phục không phụ thuộc ngăn undo của ứng dụng

**Hermes.** `tools/checkpoint_manager.py` tự chụp thư mục làm việc vào một git store riêng **trước thao tác ghi đầu
tiên của mỗi lượt** (`write_file`, `patch`, lệnh shell phá huỷ). Model không thấy và không gọi được cơ chế này; nó là hạ
tầng bật bằng cấu hình. Người dùng quay lại bất kỳ checkpoint nào.

**Claude Code** (theo changelog và tài liệu, không phải code). `/rewind` khôi phục cả hội thoại lẫn file đã sửa bằng tool
chỉnh sửa của nó. Changelog nói rõ phạm vi: sửa qua Bash, kể cả `sed -i`, **không** được khôi phục, và màn hình rewind
phải báo đúng điều đó (`CHANGELOG.md:51, 151, 180`).

**Grok Build.** `xai-hunk-tracker` gắn mỗi hunk thay đổi với nguồn: agent (kèm lượt nào) hay bên ngoài. Người dùng chấp
nhận hoặc từ chối **từng hunk**, và có tóm tắt "thay đổi đang chờ theo từng lượt"
(`crates/codegen/xai-hunk-tracker/src/types.rs:68-70, 317-333`).

**Vì sao quan trọng với Axiom.** Undo của Axiom dựa vào ngăn undo của ứng dụng. Ngăn đó dùng chung với người dùng
(F03), không có với thay đổi qua COM trong Excel và không gộp được thành một bước trong PowerPoint (F10).

**Đề xuất, hai hướng bổ sung cho nhau:**

a. **Checkpoint tài liệu trước lần ghi đầu tiên của lượt** (P1, cần spike trên Office thật):
   - Excel: `Workbook.SaveCopyAs(path)` lưu bản sao mà không đổi sổ đang mở.
   - PowerPoint: `Presentation.SaveCopyAs(path)`.
   - Word không có `SaveCopyAs`. Ứng viên là đọc `Document.Content.WordOpenXML` rồi khôi phục bằng
     `Range.InsertXML`; cần đo độ trung thực (header/footer, section, style) và kích thước.
   - Core giữ đường dẫn checkpoint theo `runId`. "Khôi phục trước lượt" là một lệnh bridge riêng, có hỏi xác nhận, và
     **báo rõ phạm vi** như Claude Code: thay đổi người dùng làm sau lượt cũng sẽ mất.
   - Đây cũng là đường phục hồi sau crash (F04): checkpoint nằm trên đĩa, không phụ thuộc process Core.

b. **Chế độ "AI sửa bằng Track Changes" cho Word** (P2, cần spike). Đây là bản Office của hunk tracker:
   - Trong lượt, bật `Document.TrackRevisions` và đặt tên tác giả "Axiom AI". Mỗi thay đổi của AI thành một revision có
     tác giả và thời điểm.
   - "Hoàn tác lượt này" = từ chối các revision của tác giả "Axiom AI" tạo sau thời điểm bắt đầu lượt. Không đụng tới
     chữ người dùng gõ xen giữa, nên giải quyết F03 mà không cần đọc ngăn undo.
   - Người dùng duyệt từng thay đổi bằng giao diện Review sẵn có của Word.
   - Rủi ro cần đo: `writer.restyle`/`doc_design` tạo rất nhiều revision định dạng, bảng chèn mới, hiệu năng trên tài
     liệu dài, và việc đổi `UserName` tạm thời phải được khôi phục ở mọi đường thoát.

## 4. Không ghi đè thứ mình chưa thấy hoặc đã đổi

**Hermes.** `_stale_overwrite_blocker` từ chối **trước khi** ghi trong ba trường hợp: tệp đã đổi kể từ lần đọc
(mtime lệch, hoặc agent khác ghi), tệp có sẵn mà lượt chưa đọc đủ nội dung, hoặc lượt chỉ vá mù mà chưa đọc toàn bộ
(`tools/file_tools_write_guards.py:557-594`). Thông báo lỗi nói rõ phải làm gì: đọc lại toàn bộ rồi mới ghi đè.

**Axiom hiện có.** `overwrite.go` đọc trước vùng ô sắp ghi và hỏi khi ghi đè dữ liệu có sẵn; đây đã là một guard "nhìn
trước khi ghi". Chưa có:
- phát hiện người dùng sửa tài liệu **giữa hai lệnh** của agent (revision của Core chỉ đếm việc của lượt,
  `model/guards.go:248-249`);
- kiểm tra trước khi `writer.undo` (F03).

**Đề xuất (P1).** Mỗi lệnh ghi Word trả thêm một dấu vân tay rẻ của tài liệu sau khi ghi, ví dụ `Content.End` cộng
CRC của đoạn cuối vùng vừa ghi. Đây đúng là trường `post.fingerprint` đề xuất trong audit G.2. Trước `writer.undo` và
trước lệnh ghi kế tiếp, so với vân tay hiện tại: lệch thì báo model "tài liệu đã bị sửa ngoài lượt" và từ chối undo.

## 5. Ghi bền trước khi báo

**Codex.** Rollout là JSONL chỉ ghi thêm, có khoá người ghi (`codex-rs/rollout/src/writer_lock.rs`). Mục quan trọng
được flush **trước** sự kiện báo client (mục 1).

**Grok Build.** Sự kiện kết thúc lượt được lưu và phát lại để client gắn lại giữa chừng không bị kẹt ở "Waiting…"
(`session/turn_completion.rs:3-5`). Có hẳn crate chọn chế độ journal SQLite theo loại hệ thống tệp vì WAL không an toàn
trên NFS (`crates/codegen/xai-sqlite-journal/src/lib.rs:1-13`).

**Axiom.** Sự kiện SSE chỉ ở RAM (`agent/events.go`). Audit tool call được ghi sau khi tool xong.

**Đề xuất (P2).** Lưu ít nhất sự kiện kết thúc lượt vào `runs` (đã có cột `status`, `error`). Khi pane hỏi
`GET /v1/runs/{id}` cho một lượt không còn trong RAM, trả từ DB thay vì 404. Không cần lưu mọi sự kiện. Lưu ý thêm:
`core.db` nằm dưới `%LOCALAPPDATA%`, có thể bị chuyển hướng ra ổ mạng ở doanh nghiệp; nên ghi một dòng cảnh báo khi
phát hiện đường dẫn UNC.

## 6. Chính sách phê duyệt dạng khai báo, có ví dụ tự kiểm

**Codex.** `execpolicy` dùng luật `prefix_rule(pattern, decision = allow|prompt|forbidden, justification, match,
not_match)`. `match`/`not_match` là ví dụ được **kiểm khi nạp luật**, như unit test nằm trong chính file chính sách
(`codex-rs/execpolicy/README.md`).

**Claude Code** (changelog). Hook `PreToolUse`/`PermissionRequest` **fail-closed**: hook lỗi thì chặn, không cho qua
(`CHANGELOG.md:106, 160, 734`). Plan mode chặn lệnh ghi kể cả khi hook cho phép (`CHANGELOG.md:722`).

**Axiom.** Luật hỏi xác nhận nằm cứng trong `policy/policy.go:115-156`. Catalog lệnh (`catalog/live-commands.json`)
đã là nguồn chuẩn cho tên lệnh, tham số và `undo`.

**Đề xuất (P2).** Thêm trường `confirm` (`never | always | largeDocument | outsidePrompt`) và `justification` vào từng
lệnh trong catalog, thêm `examples` cho các trường hợp biên. Một test duyệt catalog để kiểm ví dụ. Tổ chức đổi được
chính sách mà không phải sửa Go, và nguyên tắc fail-closed giữ nguyên: lệnh mới chưa khai báo thì hỏi.

## 7. Các ý nhỏ hơn

| Ý | Nguồn | Áp dụng cho Axiom | Ưu tiên |
|---|---|---|---|
| Bộ phát hiện "bỏ cuộc sớm" theo mẫu câu ở đoạn cuối câu trả lời, có nhãn để đo precision/recall | Grok `session/goal_stop_detector.rs:1-20` | Mẫu câu tiếng Việt ("bạn hãy tự…", "tôi không thể…") sau một lượt có sửa → nhắc một lần. Chỉ làm khi bench cho thấy có vấn đề | P3 |
| Circuit breaker cửa sổ trượt cho nhà cung cấp model | Grok `crates/common/xai-circuit-breaker` | Model đang chết thì lượt sau chuyển ngay sang model dự phòng, không thử lại từ đầu mỗi lượt | P3 |
| Sổ bằng chứng kiểm chứng thụ động + chặn kết thúc khi sửa mà chưa có bằng chứng mới | Hermes `agent/verification_evidence.py`, `verification_stop.py` | Ghi lệnh kiểm nào chạy ở revision nào; `verified` chỉ đúng khi có bằng chứng ở revision cuối (F06). `verifyGuard` đã có nửa sau | P2 |
| Guard là bộ điều khiển không side effect, trả quyết định; runtime quyết định cảnh báo, trả kết quả giả hay dừng | Hermes `agent/tool_guardrails.py:1-6` | Guard của Axiom sửa `agentLoop` trực tiếp. Chỉ nên tách khi thêm guard mới khó test | P3 |
| Phát hiện lặp nội dung trong output trước khi nhắc "viết tiếp" | Hermes `agent/repetition_guard.py` | `emptyReplyGuard` nhắc viết tiếp khi `finish_reason=length`; nên kiểm lặp trước khi nhắc | P3 |
| Báo rõ phạm vi khôi phục | Claude Code `CHANGELOG.md:51, 180` | Nút "Hoàn tác lượt này" và câu trả lời cuối nói rõ: Excel/PowerPoint không hoàn tác được thay đổi qua COM | P1 (UX, rẻ) |

## Không nên học

- **Đa agent, sandbox, chạy tool song song** (Codex, Grok, Hermes): Axiom thao tác qua COM tuần tự (`ComGate`), không
  chạy shell; song song hoá chỉ làm hỏng tài liệu.
- **Chia nhỏ quá mức**: Hermes có 60 file `hermes_state_*.py` ở thư mục gốc cho phần lưu trạng thái. Axiom ở quy mô một
  `core.db` nhỏ thì vài bảng là đủ.
- **Workflow đồ thị**: không agent nào trong bốn dự án dùng. Kết luận của audit (Phần F) giữ nguyên.

## Lộ trình gợi ý (nối tiếp audit)

| # | Việc | Nguồn ý tưởng | Phạm vi | Phụ thuộc |
|---|---|---|---|---|
| 1 | Tin nhắn "lượt bị dừng/gián đoạn" vào hội thoại, kèm lệnh có thể đã chạy dở | Codex, Grok | Core Go | — |
| 2 | Hủy khi lệnh ghi đang chạy → kết quả "không rõ" | Codex | Core Go | — |
| 3 | Báo phạm vi hoàn tác trong pane/câu trả lời | Claude Code | Pane + prompt | — |
| 4 | Vân tay tài liệu sau lệnh ghi; từ chối undo khi tài liệu đổi ngoài lượt | Hermes | C# + Core | — |
| 5 | Spike checkpoint `SaveCopyAs` / `WordOpenXML` | Hermes, Claude Code | C# | Office thật |
| 6 | Lưu sự kiện kết thúc lượt; `GET /v1/runs/{id}` đọc DB | Grok, Codex | Core Go | — |
| 7 | Chính sách xác nhận khai báo trong catalog + ví dụ tự kiểm | Codex | Catalog + Go | — |
| 8 | Spike Track Changes cho lượt AI trong Word | Grok hunk tracker | C# | Office thật |

Mục 1, 2, 3 và 6 làm và test được hoàn toàn trên Linux, như các sửa trong `d9a3daa`. Mục 4, 5, 8 cần Word/Excel thật
để đo trước khi quyết định.
