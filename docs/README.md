# Tài liệu Axiom Office

Mục lục mọi tài liệu của dự án, chia theo người đọc. Sắp xếp lại ngày 20/10/2026: các tài liệu kiến trúc trước đây
nằm ở gốc repo (`ARCHITECTURE.MD`, `New_arch.md`, `LibreOffice_arch.md`) nay ở `docs/architecture/`.

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
| [architecture/libreoffice.md](architecture/libreoffice.md) | Tích hợp LibreOffice trên Linux: extension Python UNO, sidebar, Core đa nền tảng, giai đoạn L0–L4 |
| [architecture/wps-bridge.md](architecture/wps-bridge.md) | WPS Office for Linux: add-in JS do Core phục vụ, kênh `/wps/v1/*`, khung chat, menu chuột phải, khung chat mini |
| [architecture/ribbon-ui.md](architecture/ribbon-ui.md) | Ribbon / menu trên ba nền tảng: bộ nút, đường thật đứng sau mỗi nút, nghiệm thu, nhật ký triển khai |
| [architecture/template-design.md](architecture/template-design.md) | Thiết kế lại slide theo template đính kèm |
| [architecture/hld.html](architecture/hld.html) · [architecture/for-interns.html](architecture/for-interns.html) | Bản trình bày dạng trang web: thiết kế tổng thể, và kiến trúc cho người mới (thực tập sinh). Mở bằng trình duyệt |

## Nghiên cứu và đánh giá

| Tài liệu | Nội dung |
|---|---|
| [research/wps-spike.md](research/wps-spike.md) | Spike: add-in JS trên WPS Office for Linux có làm được không (cơ sở của `architecture/wps-bridge.md`) |
| [research/skills-review.md](research/skills-review.md) | Review 8 skill dựng sẵn và khoảng trống so với công việc văn phòng |
| [../tests/wps/REFERENCE.md](../tests/wps/REFERENCE.md) | Các phép đo trên WPS thật (object model, bridge, ribbon, menu chuột phải, hộp thoại) — đặt cạnh test của add-in WPS |

## Lộ trình

| Tài liệu | Nội dung |
|---|---|
| [plans/00-readme.md](plans/00-readme.md) | Lộ trình agent văn phòng: các phase P0–P8, M1–M5, R0 — mỗi tệp là danh sách phần còn thiếu |

## Quy ước

- Tài liệu viết tiếng Việt có dấu. Chú thích trong mã viết không dấu (ASCII).
- Tài liệu **người dùng** ở `user/`; tài liệu **thiết kế** của một phần hệ thống ở `architecture/`; điều tra, spike,
  review ở `research/`; kế hoạch ở `plans/`. Số đo gắn với một bộ test thì để cạnh bộ test đó (vd
  `tests/wps/REFERENCE.md`).
- Trỏ tới tài liệu bằng đường dẫn tính từ gốc repo trong chữ thường và chú thích mã (`docs/architecture/core.md` §7),
  bằng đường dẫn tương đối trong liên kết Markdown.
- Thêm tài liệu mới thì thêm một dòng vào mục lục này.
