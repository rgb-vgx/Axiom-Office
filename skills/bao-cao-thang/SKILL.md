---
name: bao-cao-thang
description: >-
  Dựng bộ slide báo cáo tháng/quý trong PowerPoint: slide tiêu đề, kết quả chính, số liệu so sánh, vấn
  đề và kế hoạch tháng tới. Dùng khi người dùng muốn làm "slide báo cáo", "báo cáo tháng", "báo cáo
  quý", "slide tổng kết", "trình bày kết quả công việc".
apps: [wpp]
---

# Slide báo cáo tháng

Dùng cùng skill `trinh-bay-chuyen-nghiep` (lưới, cỡ chữ, soát bố cục). Bố cục chi tiết từng slide:
`read_skill_file {name: "bao-cao-thang", path: "references/bo-cuc.md"}`.

## Cấu trúc mặc định (5 slide)

1. **Tiêu đề**: "Báo cáo tháng MM/YYYY" + đơn vị/người trình bày.
2. **Kết quả chính**: 3 con số quan trọng nhất, mỗi số một dòng ngắn.
3. **Số liệu so sánh**: bảng `wpp.addTable` (kỳ này / kỳ trước / % thay đổi), ≤ 6 dòng.
4. **Vấn đề và nguyên nhân**: ≤ 4 gạch đầu dòng.
5. **Kế hoạch tháng tới**: ≤ 4 việc, mỗi việc có người phụ trách hoặc mốc thời gian.

Người dùng đưa số liệu thì dùng đúng số đó; thiếu số thì ghi chỗ trống rõ ràng ("[doanh thu]") và nói
trong câu trả lời — **không bịa số liệu**.

## Checklist

- [ ] `wpp.listSlides` — bài đang có gì (thêm vào cuối, không xoá slide cũ trừ khi được yêu cầu).
- [ ] Mỗi slide: `wpp.addSlide {layout: 12}` → tiêu đề (`wpp.addText`, cỡ title, màu primary, đậm) →
      nội dung theo `references/bo-cuc.md`.
- [ ] Chi tiết/diễn giải → `wpp.setNotes`.
- [ ] **Soát** `wpp.checkLayout` cho từng slide mới, sửa đến khi không còn `issues`.
