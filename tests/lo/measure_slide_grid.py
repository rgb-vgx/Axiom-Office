"""Đo lại lưới slide trong skills/trinh-bay-chuyen-nghiep/SKILL.md trên LibreOffice THẬT.

Không phải unittest (không có tiền tố `test_` nên `make test-lo` không chạy file này): đây là phép đo
để tái lập các con số trong skills/trinh-bay-chuyen-nghiep/references/co-so-do-duoc.md — 793.7x446.5pt,
dòng cao 1.12 × cỡ chữ, lề trong 7.1pt, bề ngang ký tự 0.55 × cỡ chữ, và sức chứa từng hộp của lưới.

    xvfb-run -a python3 -I tests/lo/measure_slide_grid.py

Phải có màn hình (thật hoặc xvfb): chạy `--headless` thì số đo chiều cao chữ sai. Script mở một bài
trống, đo, rồi đóng KHÔNG lưu; chỉ tắt tiến trình soffice do chính nó mở (profile riêng trong /tmp), nên
không đụng tới LibreOffice người dùng đang mở.

In "VUA"/"TRAN" cho từng hộp, rồi dựng thật một slide đầy nhất theo lưới và hỏi `checkLayout`. Kiểm luôn
giới hạn layout ở mục 4 của SKILL.md (cả bốn layout đều cho đúng hai hộp rỗng, 1 và 11 là cùng một kiểu).
Lệch một con số nào thì in "LECH: ..." và trả mã 1.
"""
from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
SOFFICE = "/usr/bin/soffice"
PORT = "2031"
PROFILE = os.path.join(tempfile.gettempdir(), "axiom-measure-slide-grid")

LINE_FACTOR = 1.12   # chiều cao một dòng / cỡ chữ (đo 13/10/2026: 1.11-1.12 qua 12-36pt)
PAD = 7.1            # lề trong trên + dưới của hộp chữ, không đổi theo cỡ chữ
CHAR_FACTOR = 0.55   # bề ngang một ký tự / cỡ chữ (đo 0.49-0.64 tuỳ chữ)
DENSE_CHARS = 600    # ngưỡng `dense` của checks.py
MARGIN = 40.0
TITLE = "Kênh trực tuyến tăng 32%, bù được phần giảm của kênh truyền thống"
BULLET = "%d. Kênh trực tuyến đóng góp 1,8 tỷ đồng, chiếm 41%% tổng doanh thu của tháng"
SOURCE = "Nguồn: sổ bán hàng tháng 09/2026, đối chiếu báo cáo tài chính"
SAMPLE = ("Kênh bán hàng trực tuyến tăng 32% so với cùng kỳ năm ngoái, bù được phần giảm của "
          "kênh truyền thống trong tháng")


def measure(proc) -> int:
    import uno

    ctx = None
    local = uno.getComponentContext()
    resolver = local.ServiceManager.createInstanceWithContext("com.sun.star.bridge.UnoUrlResolver", local)
    for _ in range(80):
        try:
            ctx = resolver.resolve("uno:socket,host=127.0.0.1,port=%s;urp;StarOffice.ComponentContext" % PORT)
            break
        except Exception:  # noqa: BLE001
            if proc.poll() is not None:
                print("soffice tắt ngay khi khởi động (thiếu DISPLAY? thử chạy qua xvfb-run)")
                return 1
            time.sleep(0.5)
    if ctx is None:
        print("không kết nối được LibreOffice")
        return 1

    sys.path.insert(0, PYTHONPATH)
    from axiom import bridge, checks, commands, impress  # noqa: E402

    commands.load_all()
    desktop = ctx.ServiceManager.createInstanceWithContext("com.sun.star.frame.Desktop", ctx)
    doc = desktop.loadComponentFromURL("private:factory/simpress", "_blank", 0, ())
    env = bridge.Env(ctx, "wpp", None)
    env._document = doc
    pages = doc.getDrawPages()
    W = pages.getByIndex(0).Width * checks.HMM_TO_POINT
    H = pages.getByIndex(0).Height * checks.HMM_TO_POINT
    bad = []

    def put(params, new_slide=True):
        if new_slide:
            impress.add_slide(env, {"layout": 12})
        impress.add_text(env, params)
        page = pages.getByIndex(pages.getCount() - 1)
        return page.getByIndex(page.getCount() - 1)

    def true_height(shape):
        """Chiều cao chữ THẬT: bật autogrow rồi ép chiều cao về 1/100mm, đọc lại số LibreOffice tự tính."""
        shape.TextAutoGrowHeight = True
        size = uno.createUnoStruct("com.sun.star.awt.Size")
        size.Width, size.Height = shape.Size.Width, 1
        shape.Size = size
        height = shape.Size.Height * checks.HMM_TO_POINT
        shape.TextAutoGrowHeight = False
        return height

    def one_and_two(size):
        args = {"left": MARGIN, "top": 130, "width": W - 80, "height": 40, "fontSize": size}
        h1 = true_height(put(dict(args, text="Xin chào")))
        h2 = true_height(put(dict(args, text="Xin chào\nXin chào")))
        return h1, h2

    print("slide %.1f x %.1f pt" % (W, H))

    # 1. Ba hằng số dàn chữ: đo lại từ đầu thay vì tin vào bài viết.
    print("\n== 1. Hằng số dàn chữ ==")
    line_h = {}
    for size in (18, 20, 28):
        h1, h2 = one_and_two(size)
        line_h[size], pad = h2 - h1, 2 * h1 - h2
        print("   %2dpt: một dòng %5.1fpt, hai dòng %5.1fpt -> dòng cao %5.2fpt (%.3f × cỡ chữ),"
              " lề trong %.1fpt" % (size, h1, h2, line_h[size], line_h[size] / size, pad))
        if abs(line_h[size] / size - LINE_FACTOR) > 0.02:
            bad.append("dòng cao ở %dpt là %.3f × cỡ chữ, không phải %.2f" % (size, line_h[size] / size, LINE_FACTOR))
        if abs(pad - PAD) > 0.5:
            bad.append("lề trong ở %dpt là %.1fpt, không phải %.1f" % (size, pad, PAD))

    # 2. Sức chứa một dòng: dò nhị phân chuỗi dài nhất còn vừa MỘT dòng.
    #    (Chia độ dài mẫu cho số dòng chỉ ra cận dưới - chỗ ngắt từ làm sai lệch - nên phải dò.)
    print("\n== 2. Sức chứa một dòng (dò nhị phân, mẫu tiếng Việt) ==")
    unit = "doanh thu tăng "

    def one_line_limit(size):
        low, high = 1, 400
        while low < high:
            mid = (low + high + 1) // 2
            text = (unit * (mid // len(unit) + 1))[:mid]
            shape = put({"text": text, "left": MARGIN, "top": 130, "width": W - 80,
                         "height": 40, "fontSize": size}, new_slide=False)
            if true_height(shape) <= PAD + line_h[size] + 1.5:
                low = mid
            else:
                high = mid - 1
        return low

    for size in (20, 28):
        limit = one_line_limit(size)
        factor = (W - 80) / limit / size
        estimate = int((W - 80) / (CHAR_FACTOR * size))
        print("   %2dpt: dài nhất %d ký tự một dòng -> 1 ký tự = %.2f × cỡ chữ;"
              " ước lượng %.2f cho ra %d ký tự" % (size, limit, factor, CHAR_FACTOR, estimate))
        if estimate > limit:
            bad.append("ước lượng %.2f ở %dpt cho %d ký tự, nhiều hơn sức chứa thật %d - đang hứa quá"
                       % (CHAR_FACTOR, size, estimate, limit))

    # Văn xuôi thật (có dấu, nhiều chữ ngắn) để biết ước lượng lệch bao nhiêu.
    shape = put({"text": SAMPLE, "left": MARGIN, "top": 130, "width": W - 80,
                 "height": 40, "fontSize": 20})
    lines = round((true_height(shape) - PAD) / line_h[20])
    print("   văn xuôi thật 20pt: %d ký tự thành %d dòng (%.1f ký tự/dòng) — ngắt từ ăn mất phần dư"
          % (len(SAMPLE), lines, len(SAMPLE) / lines))

    # 3. Lưới trong SKILL.md: từng hộp có đủ cho nội dung tối đa không (tính từ công thức).
    #    Số dòng ở đây là ĐÚNG con số SKILL.md hứa (tiêu đề 2, nội dung 12, nguồn 1) - hứa bao nhiêu
    #    thì đo bấy nhiêu, vì "12 dòng" chỉ vừa hộp 276.5pt với 0.6pt dư.
    print("\n== 3. Sức chứa từng hộp của lưới (tính từ công thức) ==")
    for label, box_h, size, lines in (("tiêu đề  28pt 2 dòng", 80.0, 28, 2),
                                      ("nội dung 20pt 12 dòng", H - 170.0, 20, 12),
                                      ("nguồn    14pt 1 dòng", 24.0, 14, 1)):
        need = PAD + lines * LINE_FACTOR * size
        verdict = "VỪA" if need <= box_h else "TRÀN"
        print("   %-22s hộp %6.1fpt, cần %6.1fpt -> %s" % (label, box_h, need, verdict))
        if verdict != "VỪA":
            bad.append("%s: cần %.1fpt nhưng hộp chỉ %.1fpt" % (label, need, box_h))

    # 4. Dựng thật một slide đầy nhất theo lưới rồi hỏi checkLayout.
    print("\n== 4. Dựng thật theo lưới + checkLayout ==")
    body = "\n".join(BULLET % i for i in (1, 2, 3, 4, 5))
    put({"text": TITLE, "left": MARGIN, "top": 30, "width": W - 80, "height": 80,
         "fontSize": 28, "bold": True})
    put({"text": body, "left": MARGIN, "top": 130, "width": W - 80, "height": H - 170,
         "fontSize": 20}, new_slide=False)
    put({"text": SOURCE, "left": MARGIN, "top": H - 36, "width": W - 80, "height": 24,
         "fontSize": 14}, new_slide=False)
    chars = len(TITLE) + len(body) - body.count("\n") + len(SOURCE)
    report = checks.layout_report(env, {"slide": pages.getCount()})
    issues = [i["type"] for i in report["slides"][0]["issues"]]
    print("   tiêu đề %d ký tự, nội dung %d ký tự (5 gạch đầu dòng), nguồn %d ký tự -> %d ký tự"
          % (len(TITLE), len(body), len(SOURCE), chars))
    print("   checkLayout: issueCount=%d %s" % (report["issueCount"], issues))
    if issues:
        bad.append("slide dựng đúng theo lưới bị checkLayout bắt: %s" % issues)
    if chars > DENSE_CHARS:
        bad.append("slide đầy nhất có %d ký tự, vượt ngưỡng dense %d" % (chars, DENSE_CHARS))

    # 5. Gioi han layout (SKILL.md muc 4): moi layout cho dung HAI hop RONG, khong hop nao co chu -
    #    do la ly do luon dung layout 12 + hop chu tu dat. Doc lai page.Layout cho biet layout nao la gi.
    print("\n== 5. Layout 1/2/11/12 cho gì ==")
    for layout in (1, 2, 11, 12):
        impress.add_slide(env, {"layout": layout})
        page = pages.getByIndex(pages.getCount() - 1)
        texts = []
        for j in range(page.getCount()):
            shape = page.getByIndex(j)
            try:
                texts.append(shape.getText().getString())
            except Exception:  # noqa: BLE001
                texts.append("<khong phai hop chu>")
        print("   layout %-2d (DrawPageLayout %d) -> %d hộp, chữ: %s"
              % (layout, page.Layout, page.getCount(), texts))
        if page.getCount() != 2 or any(t != "" for t in texts):
            bad.append("layout %d cho %d hộp / có chữ (%s) — SKILL.md mục 4 nói là hai hộp RỖNG"
                       % (layout, page.getCount(), texts))

    doc.close(False)
    try:
        desktop.terminate()
    except Exception:  # noqa: BLE001
        pass

    print()
    if bad:
        for line in bad:
            print("LỆCH: " + line)
        return 1
    print("OK: mọi hộp vừa, checkLayout sạch, hằng số khớp bài viết trong SKILL.md")
    return 0


def main() -> int:
    proc = subprocess.Popen(
        [SOFFICE, "-env:UserInstallation=file://" + PROFILE, "--norestore", "--nologo",
         "--nofirststartwizard", "--accept=socket,host=127.0.0.1,port=%s;urp;" % PORT],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        return measure(proc)
    except Exception as exc:  # noqa: BLE001
        print("lỗi khi đo: %r" % (exc,))
        return 1
    finally:
        proc.kill()


if __name__ == "__main__":
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    sys.exit(main())
