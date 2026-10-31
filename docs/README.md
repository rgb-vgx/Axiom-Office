# Tài liệu Axiom Office

Mục lục mọi tài liệu của dự án, chia theo người đọc. Sắp xếp lại ngày 20/10/2026: các tài liệu kiến trúc trước đây
nằm ở gốc repo (`ARCHITECTURE.MD`, `New_arch.md`, `LibreOffice_arch.md`) nay ở `docs/architecture/`.

## Người mới (thực tập sinh) — đọc trước tiên

| Tài liệu | Nội dung |
|---|---|
| [onboarding/intern-guide.md](onboarding/intern-guide.md) | **Axiom Office từ đầu tới cuối** cho người chưa biết gì: kiến thức nền (add-in, LLM agent, tool calling, MCP), bức tranh tổng, bản đồ repo, theo chân một yêu cầu qua từng file, bên trong Core và add-in WPS, dựng môi trường, test, thay đổi đầu tiên, quy ước, gỡ lỗi, bẫy đã gặp, lộ trình tuần đầu, từ điển |

## Người dùng

| Tài liệu | Nội dung |
|---|---|
| [user/user-manual.md](user/user-manual.md) | **Hướng dẫn sử dụng**: cài, thiết lập máy chủ AI, khung chat, sửa nhanh bằng chuột phải, từng nút ribbon/menu, Soát tài liệu, Bộ nhớ AI, dữ liệu, xử lý sự cố, gỡ cài đặt |

Gói cài Windows còn kèm `scripts/dist/HUONG-DAN-CAI-DAT.txt` (hướng dẫn cài ngắn, đặt cạnh `install.cmd`).

## Bắt đầu với mã nguồn

| Tài liệu | Nội dung |
|---|---|
| [../README.md](../README.md) | Giới thiệu, cài đặt, Ask AI, MCP server, HTTP API, cấu hình, phát triển, kiểm thử, từng nền tảng |
| [../STATUS.md](../STATUS.md) | Ảnh chụp tình trạng: đã làm được gì, chưa làm được gì, việc nên làm tiếp |
| [../CHANGELOG.md](../CHANGELOG.md) | Lịch sử thay đổi |

## Kiến trúc và thiết kế

| Tài liệu | Nội dung |
|---|---|
| [architecture/README.md](architecture/README.md) | **Kiến trúc tổng thể** — Phần I: thiết kế tổng thể (HLD: mục tiêu, khối lớn, kịch bản, triển khai, quyết định); Phần II: thiết kế chi tiết (LLD: tổ chức mã, add-in, Core, bridge…) |
| [architecture/core.md](architecture/core.md) | Agent Core: yêu cầu triển khai — hợp đồng giao tiếp (§7: Core API v1, sự kiện SSE), các module (§8: orchestrator, memory, skill, MCP), giai đoạn triển khai (§12) |
| [../core-go/README.md](../core-go/README.md) | Agent Core bản Go: bố cục gói, các giai đoạn G1–G12, khác biệt so với bản .NET đã bỏ |
| [architecture/wps-bridge.md](architecture/wps-bridge.md) | **WPS Office for Linux (trọng tâm)**: add-in JS do Core phục vụ, kênh `/wps/v1/*`, mỗi tiến trình WPS một phiên, giới hạn đã biết + bảng lệnh/tool còn thiếu (sinh tự động), khung chat, menu chuột phải, khung chat mini |
| [architecture/libreoffice.md](architecture/libreoffice.md) | Tích hợp LibreOffice (**đóng băng** từ 25/10/2026): extension Python UNO, sidebar, Core đa nền tảng, giai đoạn L0–L4 |
| [architecture/ribbon-ui.md](architecture/ribbon-ui.md) | Ribbon / menu trên ba nền tảng: bộ nút, đường thật đứng sau mỗi nút, nghiệm thu, nhật ký triển khai |
| [architecture/template-design.md](architecture/template-design.md) | Thiết kế lại slide theo template đính kèm |
| [architecture/hld.html](architecture/hld.html) · [architecture/for-interns.html](architecture/for-interns.html) | Bản trình bày dạng trang web: thiết kế tổng thể, và kiến trúc cho người mới. **Ảnh chụp 12/10/2026** (đầu trang ghi các điểm đã lỗi thời) — bản hiện hành cho người mới là `onboarding/intern-guide.md`. Mở bằng trình duyệt |

## Nghiên cứu và đánh giá

| Tài liệu | Nội dung |
|---|---|
| [research/wps-spike.md](research/wps-spike.md) | Spike: add-in JS trên WPS Office for Linux có làm được không (cơ sở của `architecture/wps-bridge.md`) |
| [research/doi-chieu-wps-ai.md](research/doi-chieu-wps-ai.md) | Đối chiếu bộ WPS AI (33 addon) với Axiom trên WPS Linux: bảng khoảng cách có bằng chứng `file:dòng`, ưu tiên, thiết kế P1, trạng thái từng mục (§6), phân công Claude/opencode (§7) |
| [research/tinh-nang-con-thieu-so-voi-claude-m365.md](research/tinh-nang-con-thieu-so-voi-claude-m365.md) | Đối chiếu với Claude for Microsoft 365: tính năng còn thiếu, thiết kế, trạng thái từng mục (§6) |
| [research/hoc-tu-agent-khac.md](research/hoc-tu-agent-khac.md) | Học gì từ Claude Code, Codex, Grok Build, Hermes Agent cho các lỗ hổng của agent runtime |
| [research/skills-review.md](research/skills-review.md) | Review 8 skill dựng sẵn và khoảng trống so với công việc văn phòng |
| [research/giam-ngu-canh-gui-lai-agent.md](research/giam-ngu-canh-gui-lai-agent.md) | Giảm ngữ cảnh agent gửi lại mỗi vòng: kỹ thuật trên thế giới (đến 10/2026), số đo trên Axiom, lộ trình ưu tiên |
| [audits/axiom-agent-runtime-deep-audit.md](audits/axiom-agent-runtime-deep-audit.md) | Audit chuyên sâu Agent Runtime: vòng lặp, guard, ranh giới side effect với Office |
| [audits/wps-linux-live-test-2026-10-28-run2.md](audits/wps-linux-live-test-2026-10-28-run2.md) | Kiểm thử WPS 12.1.2 lượt 2: `ribbon.xml` hỏng XML làm WPS bỏ add-in, `et.sort`/`et.filter`, Writer + Spreadsheets cùng mở làm nhầm app (đã sửa 31/10), spike API comment/revision/pivot |
| [audits/windows-office-live-test-2026-10-28.md](audits/windows-office-live-test-2026-10-28.md) · [lượt 2](audits/windows-office-live-test-2026-10-28-windows-2.md) · [lượt 3](audits/windows-office-live-test-2026-10-28-run3.md) | Kiểm thử Windows Office 28/10: build, live suite, Track Changes, pivot, `other_document`, các bản vá |
| [../tests/bench/nang-luc.md](../tests/bench/nang-luc.md) | Bộ bài kiểm chứng năng lực (trích dẫn, phân tích bảng tính, redline, xuyên ứng dụng, an toàn, phục hồi) |
| [audits/wps-linux-live-test-2026-10-28.md](audits/wps-linux-live-test-2026-10-28.md) | Kiểm thử thực tế WPS 12.1.2 trên Linux: kết quả các lệnh make, ba lỗi đã sửa (`appKind`, `et.readRanges`, cache `/commands` theo port), chẩn đoán nạp add-in, live test Excel `trace_formula` và Writer `[§n]` |
| [../tests/wps/REFERENCE.md](../tests/wps/REFERENCE.md) | Các phép đo trên WPS thật (object model, bridge, ribbon, menu chuột phải, hộp thoại) — đặt cạnh test của add-in WPS |

## Lộ trình

| Tài liệu | Nội dung |
|---|---|
| [plans/00-readme.md](plans/00-readme.md) | Lộ trình agent văn phòng: các phase P0–P8, M1–M5, R0 — mỗi tệp là danh sách phần còn thiếu |

## Quy ước

- Tài liệu viết tiếng Việt có dấu. Chú thích trong mã viết không dấu (ASCII).
- Tài liệu cho **người mới** ở `onboarding/`; tài liệu **người dùng** ở `user/`; tài liệu **thiết kế** của một phần hệ thống ở `architecture/`; điều tra, spike,
  review ở `research/`; kế hoạch ở `plans/`. Số đo gắn với một bộ test thì để cạnh bộ test đó (vd
  `tests/wps/REFERENCE.md`).
- Trỏ tới tài liệu bằng đường dẫn tính từ gốc repo trong chữ thường và chú thích mã (`docs/architecture/core.md` §7),
  bằng đường dẫn tương đối trong liên kết Markdown.
- Thêm tài liệu mới thì thêm một dòng vào mục lục này.
- **Tài liệu sống** (`../README.md`, `../STATUS.md`, `architecture/`, `user/`, `onboarding/`) phải khớp code; **ảnh chụp
  có ngày** (`audits/`, `research/`) không sửa nội dung cũ — viết bản mới. Con số sinh được từ code (số lệnh, lệnh còn
  thiếu) thì sinh bằng script (`scripts/generate_wps_gaps.py`), không chép tay.
